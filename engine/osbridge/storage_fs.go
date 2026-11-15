//go:build !(js && wasm)

package osbridge

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	atomicTempSuffix          = ".tmp"
	schemeHTTP                = "http"
	schemeHTTPS               = "https"
	networkTCP                = "tcp"
	headerContentDisposition  = "Content-Disposition"
	errRefuseSymlinkFmt       = "refusing to access symlink: %s"
	errInvalidURLFmt          = "invalid URL: %w"
	errUnsupportedScheme      = "unsupported URL scheme (only http:// and https:// are allowed)"
	errMissingHost            = "missing host in URL"
	errBlockedLocalAddressFmt = "blocked connection to local or private address (%s)"
	errDNSErrorFmt            = "DNS lookup failed for %s: %w"
	errNoPublicAddressFmt     = "no public IP address found for %s"
	errDialFailedFmt          = "connection failed: %w"
	errTooManyRedirects       = "too many HTTP redirects"
	errFetchFailedFmt         = "download failed: %w"
	errHTTPStatusFmt          = "server returned HTTP %d (%s)"
	errPayloadTooLargeFmt     = "downloaded file exceeds size limit (%d MB)"
	errArchiveEntryLimitFmt   = "archive entry %s exceeds size limit"
	bytesPerMegabyte          = 1 << 20
	testFlagName              = "test.v"
)

// DefaultStorage is the default OS filesystem storage instance.
var DefaultStorage Storage = NewFSStorage()

// FSStorage implements Storage backed by the operating system filesystem and network stack.
type FSStorage struct {
	AllowLoopbackInTests bool
}

// NewFSStorage constructs a filesystem-backed Storage implementation.
func NewFSStorage() *FSStorage {
	return &FSStorage{
		AllowLoopbackInTests: true,
	}
}

// ReadBytes reads raw bytes from a file path on disk, rejecting symlinks.
func (storage *FSStorage) ReadBytes(location string) ([]byte, error) {
	linkInfo, statErr := os.Lstat(location)
	if statErr != nil {
		return nil, statErr
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf(errRefuseSymlinkFmt, location)
	}
	return os.ReadFile(location)
}

// WriteAtomic writes payload to location atomically via a temporary file and rename.
func (storage *FSStorage) WriteAtomic(location string, payload []byte) error {
	if linkInfo, statErr := os.Lstat(location); statErr == nil && linkInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf(errRefuseSymlinkFmt, location)
	}
	parentDir := filepath.Dir(location)
	if mkdirErr := os.MkdirAll(parentDir, 0755); mkdirErr != nil {
		return mkdirErr
	}
	tempPath := location + atomicTempSuffix
	if writeErr := os.WriteFile(tempPath, payload, 0644); writeErr != nil {
		return writeErr
	}
	return os.Rename(tempPath, location)
}

// WriteAt patches payload bytes in-place at offset inside location without rewriting the whole file.
func (storage *FSStorage) WriteAt(location string, payload []byte, offset int64) error {
	linkInfo, statErr := os.Lstat(location)
	if statErr != nil {
		return statErr
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf(errRefuseSymlinkFmt, location)
	}
	handle, openErr := os.OpenFile(location, os.O_WRONLY, 0644)
	if openErr != nil {
		return openErr
	}
	defer handle.Close()
	if _, seekErr := handle.Seek(offset, io.SeekStart); seekErr != nil {
		return seekErr
	}
	_, writeErr := handle.Write(payload)
	return writeErr
}

// ReadArchive reads all regular entries from a zip archive on disk into memory.
func (storage *FSStorage) ReadArchive(location string) (map[string][]byte, error) {
	rawZip, readErr := storage.ReadBytes(location)
	if readErr != nil {
		return nil, readErr
	}
	zipReader, zipErr := zip.NewReader(bytes.NewReader(rawZip), int64(len(rawZip)))
	if zipErr != nil {
		return nil, zipErr
	}
	entries := make(map[string][]byte, len(zipReader.File))
	for _, zipEntry := range zipReader.File {
		if zipEntry.FileInfo().IsDir() || zipEntry.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if zipEntry.UncompressedSize64 > maxArchiveEntryBytes {
			return nil, fmt.Errorf(errArchiveEntryLimitFmt, zipEntry.Name)
		}
		rc, openErr := zipEntry.Open()
		if openErr != nil {
			return nil, openErr
		}
		entryBytes, copyErr := io.ReadAll(io.LimitReader(rc, maxArchiveEntryBytes+1))
		_ = rc.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if len(entryBytes) > maxArchiveEntryBytes {
			return nil, fmt.Errorf(errArchiveEntryLimitFmt, zipEntry.Name)
		}
		entries[zipEntry.Name] = entryBytes
	}
	return entries, nil
}

// CreateBlobURL returns the normalized file location for OS filesystem storage.
func (storage *FSStorage) CreateBlobURL(location string, _ []byte, _ string) string {
	return location
}

// FetchURLBytes downloads up to maxBytes from an HTTP/HTTPS URL with SSRF protection against private/local IPs.
func (storage *FSStorage) FetchURLBytes(rawURL string, maxBytes int64) ([]byte, string, error) {
	parsedURL, parseErr := url.Parse(strings.TrimSpace(rawURL))
	if parseErr != nil {
		return nil, "", fmt.Errorf(errInvalidURLFmt, parseErr)
	}
	if parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS {
		return nil, "", errors.New(errUnsupportedScheme)
	}
	if parsedURL.Hostname() == "" {
		return nil, "", errors.New(errMissingHost)
	}
	allowLoopback := storage.AllowLoopbackInTests && flag.Lookup(testFlagName) != nil
	if checkErr := validateSSRFSafeHost(parsedURL.Hostname(), allowLoopback); checkErr != nil {
		return nil, "", checkErr
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	client := &http.Client{
		Timeout: 45 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, splitErr := net.SplitHostPort(addr)
				if splitErr != nil {
					return nil, splitErr
				}
				ips, lookupErr := net.DefaultResolver.LookupIPAddr(ctx, host)
				if lookupErr != nil {
					return nil, fmt.Errorf(errDNSErrorFmt, host, lookupErr)
				}
				var safeIP string
				for _, ipAddr := range ips {
					if isBlockedIP(ipAddr.IP, allowLoopback) {
						return nil, fmt.Errorf(errBlockedLocalAddressFmt, ipAddr.IP.String())
					}
					if safeIP == "" {
						safeIP = ipAddr.IP.String()
					}
				}
				if safeIP == "" {
					return nil, fmt.Errorf(errNoPublicAddressFmt, host)
				}
				return dialer.DialContext(ctx, networkTCP, net.JoinHostPort(safeIP, port))
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New(errTooManyRedirects)
			}
			if req.URL.Scheme != schemeHTTP && req.URL.Scheme != schemeHTTPS {
				return errors.New(errUnsupportedScheme)
			}
			return validateSSRFSafeHost(req.URL.Hostname(), allowLoopback)
		},
	}

	resp, reqErr := client.Get(parsedURL.String())
	if reqErr != nil {
		return nil, "", fmt.Errorf(errFetchFailedFmt, reqErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf(errHTTPStatusFmt, resp.StatusCode, resp.Status)
	}
	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if readErr != nil {
		return nil, "", readErr
	}
	if int64(len(payload)) > maxBytes {
		return nil, "", fmt.Errorf(errPayloadTooLargeFmt, maxBytes/bytesPerMegabyte)
	}
	return payload, resp.Header.Get(headerContentDisposition), nil
}

func validateSSRFSafeHost(host string, allowLoopback bool) error {
	cleanHost := strings.TrimSpace(host)
	if cleanHost == "" {
		return errors.New(errMissingHost)
	}
	if parsedIP := net.ParseIP(cleanHost); parsedIP != nil {
		if isBlockedIP(parsedIP, allowLoopback) {
			return fmt.Errorf(errBlockedLocalAddressFmt, parsedIP.String())
		}
		return nil
	}
	ips, lookupErr := net.LookupIP(cleanHost)
	if lookupErr != nil {
		return fmt.Errorf(errDNSErrorFmt, cleanHost, lookupErr)
	}
	for _, ipAddr := range ips {
		if isBlockedIP(ipAddr, allowLoopback) {
			return fmt.Errorf(errBlockedLocalAddressFmt, ipAddr.String())
		}
	}
	return nil
}

func isBlockedIP(ipAddr net.IP, allowLoopback bool) bool {
	if ipAddr == nil {
		return true
	}
	if ipAddr.IsLoopback() {
		return !allowLoopback
	}
	if ipAddr.IsPrivate() || ipAddr.IsUnspecified() || ipAddr.IsLinkLocalUnicast() || ipAddr.IsLinkLocalMulticast() || ipAddr.IsMulticast() {
		return true
	}
	if ip4 := ipAddr.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		if ip4[0] == 0 {
			return true
		}
	}
	return false
}
