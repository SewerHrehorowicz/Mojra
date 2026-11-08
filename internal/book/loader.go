package book

import (
	"archive/zip"
	"bytes"
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LoadBooksFromDirectory scans booksDir for unpacked book folders and packed .stbook/.zip archives.
// If a directory exists alongside a .stbook archive with the same ID, the unpacked directory takes precedence
// so the user can edit it live in the Node Studio.
func LoadBooksFromDirectory(booksDir string) ([]*Book, error) {
	if err := os.MkdirAll(booksDir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(booksDir)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*Book)
	var order []string

	// First pass: unpacked directories
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dirPath := filepath.Join(booksDir, e.Name())
		b, err := LoadBookFromDir(dirPath)
		if err != nil {
			continue
		}
		byID[b.ID] = b
		order = append(order, b.ID)
	}

	// Second pass: packed .playbook, .stbook, or .zip files (only if not already loaded as an unpacked folder)
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		lower := strings.ToLower(e.Name())
		if !isPackedBookArchiveName(lower) {
			continue
		}
		zipPath := filepath.Join(booksDir, e.Name())
		b, err := LoadBookFromZip(zipPath)
		if err != nil {
			continue
		}
		if _, exists := byID[b.ID]; !exists {
			byID[b.ID] = b
			order = append(order, b.ID)
		}
	}

	sort.Strings(order)
	out := make([]*Book, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, nil
}

func canonicalPathKey(p string) string {
	cleaned := filepath.Clean(strings.TrimSpace(p))
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil && resolved != "" {
		return filepath.Clean(resolved)
	}
	return cleaned
}

func isPathDirectlyInDir(targetPath, dirPath string) bool {
	return canonicalPathKey(filepath.Dir(targetPath)) == canonicalPathKey(dirPath)
}

func shortPathHash(p string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(canonicalPathKey(p)))
	return fmt.Sprintf("%04x", h.Sum32()&0xffff)
}

// RebindBookID updates the book's ID and re-encodes all media URLs (cover, scene audio, and scene images).
func RebindBookID(b *Book, newID string) {
	if b == nil || newID == "" {
		return
	}
	b.ID = SanitizeBookID(newID)
	if b.CoverFile != "" {
		b.CoverURL = encodeMediaURL(b.ID, b.CoverFile)
	}
	for _, sc := range b.Scenes {
		if sc.AudioFile != "" {
			sc.AudioURL = encodeMediaURL(b.ID, sc.AudioFile)
		}
		for i := range sc.VoiceSections {
			if sc.VoiceSections[i].AudioFile != "" {
				sc.VoiceSections[i].AudioURL = encodeMediaURL(b.ID, sc.VoiceSections[i].AudioFile)
			}
		}
		if sc.ImageFile != "" {
			sc.ImageURL = encodeMediaURL(b.ID, sc.ImageFile)
		}
	}
}

func isPackedBookArchiveName(lower string) bool {
	return strings.HasSuffix(lower, ".playbook") || strings.HasSuffix(lower, ".stbook") || strings.HasSuffix(lower, ".zip")
}

// LoadBookFromPath loads either an unpacked book directory or a packed .playbook/.stbook/.zip archive from any path on disk.
func LoadBookFromPath(bookPath string) (*Book, error) {
	info, err := os.Stat(bookPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return LoadBookFromDir(bookPath)
	}
	lower := strings.ToLower(info.Name())
	if isPackedBookArchiveName(lower) {
		return LoadBookFromZip(bookPath)
	}
	return nil, fmt.Errorf("nieobsługiwany format książki: %s", bookPath)
}

// LoadLibraryBooks combines books in booksDir with external books referenced on disk (without copying them).
func LoadLibraryBooks(booksDir string, externalPaths []string) ([]*Book, error) {
	localBooks, err := LoadBooksFromDirectory(booksDir)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*Book, len(localBooks)+len(externalPaths))
	seenRoots := make(map[string]bool, len(localBooks)*2+len(externalPaths))
	out := make([]*Book, 0, len(localBooks)+len(externalPaths))

	for _, b := range localBooks {
		byID[b.ID] = b
		seenRoots[canonicalPathKey(b.RootPath)] = true
		if !b.IsPacked {
			seenRoots[canonicalPathKey(filepath.Join(booksDir, b.ID+".playbook"))] = true
			seenRoots[canonicalPathKey(filepath.Join(booksDir, b.ID+".stbook"))] = true
			seenRoots[canonicalPathKey(filepath.Join(booksDir, b.ID+".zip"))] = true
		}
		out = append(out, b)
	}

	for _, rawExt := range externalPaths {
		absPath, err := ExpandLocalPath(rawExt)
		if err != nil {
			continue
		}
		rootKey := canonicalPathKey(absPath)
		if seenRoots[rootKey] {
			continue
		}
		b, err := LoadBookFromPath(absPath)
		if err != nil || b == nil {
			continue
		}
		b.IsExternal = !isPathDirectlyInDir(absPath, booksDir)
		if existing, exists := byID[b.ID]; exists {
			if existing.IsPacked && !b.IsPacked {
				byID[b.ID] = b
				seenRoots[rootKey] = true
				for idx := range out {
					if out[idx] == existing {
						out[idx] = b
						break
					}
				}
				continue
			}
			candidateID := fmt.Sprintf("%s-%s", b.ID, shortPathHash(absPath))
			for i := 2; byID[candidateID] != nil; i++ {
				candidateID = fmt.Sprintf("%s-%s-%d", b.ID, shortPathHash(absPath), i)
			}
			RebindBookID(b, candidateID)
		}
		byID[b.ID] = b
		seenRoots[rootKey] = true
		out = append(out, b)
	}

	return out, nil
}

// ExpandLocalPath normalizes a user-supplied disk path (stripping quotes, file:// scheme, and expanding ~).
func ExpandLocalPath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	trimmed = strings.Trim(trimmed, `"'`)
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", fmt.Errorf("pusta ścieżka do książki")
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		if u, err := url.Parse(trimmed); err == nil && u.Path != "" {
			if unescaped, uErr := url.PathUnescape(u.Path); uErr == nil {
				trimmed = unescaped
			} else {
				trimmed = u.Path
			}
		} else {
			trimmed = strings.TrimPrefix(trimmed, "file://")
		}
	}
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, `~\`) {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if trimmed == "~" {
				trimmed = home
			} else {
				trimmed = filepath.Join(home, trimmed[2:])
			}
		}
	}
	absPath, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", err
	}
	if err := validateSafeUserDiskPath(absPath); err != nil {
		return "", err
	}
	return absPath, nil
}

var blockedSensitiveDotDirs = map[string]bool{
	".ssh":    true,
	".gnupg":  true,
	".aws":    true,
	".kube":   true,
	".git":    true,
	".hg":     true,
	".svn":    true,
	".docker": true,
}

func validateSafeUserDiskPath(absPath string) error {
	clean := filepath.Clean(strings.TrimSpace(absPath))
	if clean == "" || clean == "." {
		return fmt.Errorf("pusta ścieżka na dysku")
	}
	base := filepath.Base(clean)
	if strings.HasPrefix(base, ".") && base != "." {
		return fmt.Errorf("niedozwolona ścieżka ukryta lub systemowa: %s", absPath)
	}
	for _, seg := range strings.Split(filepath.ToSlash(clean), "/") {
		if blockedSensitiveDotDirs[strings.ToLower(seg)] {
			return fmt.Errorf("niedozwolony katalog systemowy w ścieżce: %s", seg)
		}
	}
	return nil
}

// IsSafePathInsideRoot verifies that targetPath is lexically and (after symlink resolution) strictly inside rootDir,
// and that targetPath itself is not a symbolic link.
func IsSafePathInsideRoot(rootDir, targetPath string) bool {
	cleanRoot := filepath.Clean(strings.TrimSpace(rootDir))
	cleanTarget := filepath.Clean(strings.TrimSpace(targetPath))
	if cleanRoot == "" || cleanTarget == "" || cleanRoot == "." || cleanTarget == "." {
		return false
	}
	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	if linfo, err := os.Lstat(cleanTarget); err == nil {
		if linfo.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	realRoot := canonicalPathKey(cleanRoot)
	if realTarget, err := filepath.EvalSymlinks(cleanTarget); err == nil && realTarget != "" {
		cleanRealTarget := filepath.Clean(realTarget)
		return strings.HasPrefix(cleanRealTarget, realRoot+string(os.PathSeparator))
	}
	realParent := canonicalPathKey(filepath.Dir(cleanTarget))
	return realParent == realRoot || strings.HasPrefix(realParent, realRoot+string(os.PathSeparator))
}

const (
	MaxZipEntries                = 4096
	MaxZipTotalUncompressedBytes = 500 * 1024 * 1024
	MaxZipSingleTextBytes        = 4 * 1024 * 1024
	MaxZipTotalTextBytes         = 32 * 1024 * 1024
)

func validateZipArchiveLimits(files []*zip.File) error {
	if len(files) > MaxZipEntries {
		return fmt.Errorf("archiwum ZIP przekracza dopuszczalny limit liczby plików (%d)", MaxZipEntries)
	}
	var totalUncompressed uint64
	for _, f := range files {
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archiwum ZIP zawiera niedozwolone dowiązanie symboliczne (symlink): %s", f.Name)
		}
		if f.UncompressedSize64 > uint64(MaxZipTotalUncompressedBytes) || totalUncompressed > uint64(MaxZipTotalUncompressedBytes)-f.UncompressedSize64 {
			return fmt.Errorf("archiwum ZIP przekracza dopuszczalny limit rozmiaru po rozpakowaniu (500 MB)")
		}
		totalUncompressed += f.UncompressedSize64
	}
	return nil
}

func readLimitedTextReader(r io.Reader, limitErrMsg string) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxZipSingleTextBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxZipSingleTextBytes {
		return "", fmt.Errorf("%s", limitErrMsg)
	}
	return string(data), nil
}

func readSafeBookTextFile(bookDir, relName string) (string, error) {
	fullPath := filepath.Join(bookDir, relName)
	if !IsSafePathInsideRoot(bookDir, fullPath) {
		return "", fmt.Errorf("niedozwolona ścieżka pliku poza katalogiem książki: %s", relName)
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return readLimitedTextReader(f, fmt.Sprintf("plik tekstowy %s przekracza limit rozmiaru (4 MB)", relName))
}

func fileLooksLikeBookStory(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, 64*1024))
	if err != nil || len(buf) == 0 {
		return false
	}
	return looksLikePlainTextStory(string(buf))
}

func isHiddenNonEditorFile(name string) bool {
	base := filepath.Base(name)
	if !strings.HasPrefix(base, ".") {
		return false
	}
	return !strings.EqualFold(base, EditorStateFileName)
}

// HasBookFilesInDir checks whether a directory contains at least one .playtxt book file (such as story.playtxt, book.playtxt, or a .playtxt file with Mojra headers).
func HasBookFilesInDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		lower := strings.ToLower(e.Name())
		if lower == BookDataFileName || lower == DefaultStoryFile {
			return true
		}
		if strings.HasSuffix(lower, PlaytxtExt) {
			if fileLooksLikeBookStory(filepath.Join(dir, e.Name())) {
				return true
			}
		}
	}
	return false
}

func normalizeZipEntryRelPath(f *zip.File, commonPrefix string) (string, string, bool) {
	relSlash := filepath.ToSlash(f.Name)
	if isIgnoredZipEntry(relSlash) {
		return "", "", false
	}
	if commonPrefix != "" && strings.HasPrefix(relSlash, commonPrefix) {
		relSlash = strings.TrimPrefix(relSlash, commonPrefix)
	}
	relSlash = SanitizeRelativeMediaPath(relSlash)
	if relSlash == "" || isHiddenNonEditorFile(relSlash) {
		return "", "", false
	}
	return relSlash, strings.ToLower(relSlash), true
}

func findZipEntryByRelPath(files []*zip.File, cleanRel string) *zip.File {
	commonPrefix := detectZipCommonPrefix(files)
	for _, f := range files {
		if f.FileInfo().IsDir() {
			continue
		}
		entryRel, _, ok := normalizeZipEntryRelPath(f, commonPrefix)
		if ok && strings.EqualFold(entryRel, cleanRel) {
			return f
		}
	}
	return nil
}

func readZipTextWithTotalCap(f *zip.File, totalTextBytes *int) (string, error) {
	content, err := readZipEntryText(f)
	if err != nil {
		return "", err
	}
	*totalTextBytes += len(content)
	if *totalTextBytes > MaxZipTotalTextBytes {
		return "", fmt.Errorf("archiwum ZIP przekracza łączny limit rozmiaru plików tekstowych (32 MB)")
	}
	return content, nil
}

func ensureFallbackEmptyScene(b *Book, warnMsg, fallbackText string) {
	if len(b.Scenes) > 0 {
		return
	}
	b.Warnings = append(b.Warnings, warnMsg)
	b.Scenes = []*Scene{
		{
			ID:          "s1",
			Name:        "Pusta scena",
			Chapter:     DefaultBookChapterName,
			ChapterID:   "rozdzial_1",
			Text:        fallbackText,
			HTML:        RenderSafeProseHTML(fallbackText),
			SourceFile:  DefaultChapterFile,
			ExplicitEnd: true,
		},
	}
}

// ValidateBookZip checks whether a .stbook or .zip archive is a valid zip containing Mojra .playtxt files.
func ValidateBookZip(zipPath string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("nieprawidłowe archiwum ZIP / .stbook: %w", err)
	}
	defer zr.Close()

	if err := validateZipArchiveLimits(zr.File); err != nil {
		return err
	}

	commonPrefix := detectZipCommonPrefix(zr.File)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		relSlash, lower, ok := normalizeZipEntryRelPath(f, commonPrefix)
		if !ok {
			continue
		}
		if lower == BookDataFileName || lower == DefaultStoryFile || (!strings.Contains(relSlash, "/") && strings.HasSuffix(lower, PlaytxtExt)) {
			return nil
		}
	}
	return fmt.Errorf("archiwum nie zawiera plików opowieści (.playtxt / story.playtxt / book.playtxt)")
}

// ResolveDiskBookPaths validates and resolves a disk path into one or more book paths (folders or .stbook/.zip files) without copying them.
func ResolveDiskBookPaths(rawPath string) ([]string, error) {
	absPath, err := ExpandLocalPath(rawPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("nie znaleziono ścieżki na dysku: %s", absPath)
	}

	if !info.IsDir() {
		lower := strings.ToLower(info.Name())
		if isPackedBookArchiveName(lower) {
			if err := ValidateBookZip(absPath); err != nil {
				return nil, err
			}
			return []string{absPath}, nil
		}
		if strings.HasSuffix(lower, PlaytxtExt) {
			parentDir := filepath.Dir(absPath)
			if HasBookFilesInDir(parentDir) {
				return []string{parentDir}, nil
			}
		}
		return nil, fmt.Errorf("nieobsługiwany plik „%s” (wskaż folder książki lub archiwum .playbook / .zip)", info.Name())
	}

	if HasBookFilesInDir(absPath) {
		return []string{absPath}, nil
	}

	// Check if the chosen folder is a parent directory containing multiple book folders or .playbook/.stbook/.zip files
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		childPath := filepath.Join(absPath, e.Name())
		if e.IsDir() {
			if HasBookFilesInDir(childPath) {
				found = append(found, childPath)
			}
			continue
		}
		lower := strings.ToLower(e.Name())
		if isPackedBookArchiveName(lower) {
			if ValidateBookZip(childPath) == nil {
				found = append(found, childPath)
			}
		}
	}
	if len(found) > 0 {
		sort.Strings(found)
		return found, nil
	}
	return nil, fmt.Errorf("w katalogu „%s” nie znaleziono książki (brak story.playtxt / book.playtxt ani archiwów .playbook / .zip)", filepath.Base(absPath))
}

const maxBookDownloadBytes = 250 * 1024 * 1024

func isRunningUnderGoTest() bool {
	if flag.Lookup("test.v") != nil {
		return true
	}
	if len(os.Args) > 0 && (strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], ".test.exe")) {
		return true
	}
	return false
}

func isBlockedSSRFIp(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}
	if ip.IsLoopback() {
		return !isRunningUnderGoTest()
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && (ip4[1]&0xc0) == 64 {
			return true
		}
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}

// DownloadBookFromURL downloads a .stbook or .zip archive (or plain .txt story) from an HTTP/HTTPS URL and stores it as a .stbook package in booksDir.
func DownloadBookFromURL(booksDir, rawURL string) (*Book, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return nil, fmt.Errorf("podaj adres URL do książki (.stbook lub .zip)")
	}
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, fmt.Errorf("nieprawidłowy adres URL (wymagany protokół http:// lub https://)")
	}

	if err := os.MkdirAll(booksDir, 0755); err != nil {
		return nil, err
	}

	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 15 * time.Second,
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if parsedIP := net.ParseIP(host); parsedIP != nil {
				if isBlockedSSRFIp(parsedIP) {
					return nil, fmt.Errorf("zablokowano połączenie z adresem wewnętrznym/prywatnym (SSRF): %s", parsedIP.String())
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(parsedIP.String(), port))
			}
			ipAddrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ipAddrs) == 0 {
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("nie można rozwiązać nazwy hosta: %s", host)
			}
			for _, ia := range ipAddrs {
				if isBlockedSSRFIp(ia.IP) {
					return nil, fmt.Errorf("zablokowano połączenie z adresem wewnętrznym/prywatnym (SSRF): %s", ia.IP.String())
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ipAddrs[0].IP.String(), port))
		},
	}

	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("zbyt wiele przekierowań HTTP")
			}
			if req.URL == nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
				return fmt.Errorf("niedozwolony protokół przekierowania")
			}
			return nil
		},
	}
	req, err := http.NewRequest(http.MethodGet, trimmedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("błąd tworzenia żądania HTTP: %w", err)
	}
	req.Header.Set("User-Agent", "Mojra/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("błąd pobierania z URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serwer zwrócił błąd HTTP %d (%s)", resp.StatusCode, resp.Status)
	}

	tmpFile, err := os.CreateTemp(booksDir, ".stbook-download-*.tmp")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	written, err := io.Copy(tmpFile, io.LimitReader(resp.Body, maxBookDownloadBytes+1))
	_ = tmpFile.Close()
	if err != nil {
		return nil, fmt.Errorf("błąd zapisu pobieranego pliku: %w", err)
	}
	if written == 0 {
		return nil, fmt.Errorf("pobrany plik z adresu URL jest pusty")
	}
	if written > maxBookDownloadBytes {
		return nil, fmt.Errorf("pobrany plik przekracza dopuszczalny limit rozmiaru (250 MB)")
	}

	urlBase := extractFilenameFromURLOrHeader(parsedURL, resp.Header.Get("Content-Disposition"))
	urlExt := strings.ToLower(filepath.Ext(urlBase))
	urlStem := strings.TrimSuffix(urlBase, filepath.Ext(urlBase))

	if err := ValidateBookZip(tmpPath); err != nil {
		// Check if the downloaded payload is a plain-text story file (.txt) with scenes
		rawBytes, readErr := os.ReadFile(tmpPath)
		if readErr == nil && looksLikePlainTextStory(string(rawBytes)) {
			if err := wrapPlainStoryIntoZip(tmpPath, string(rawBytes), urlStem); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("pobrany plik nie jest prawidłowym archiwum książki (.stbook / .zip): %v", err)
		}
	}

	previewBook, err := LoadBookFromZip(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("nie udało się wczytać pobranej książki: %w", err)
	}

	bookID := chooseDownloadedBookID(urlStem, urlExt, previewBook)
	// Avoid shadowing if an unpacked directory with the same ID already exists in booksDir
	if info, err := os.Stat(filepath.Join(booksDir, bookID)); err == nil && info.IsDir() {
		bookID = fmt.Sprintf("%s-%s", bookID, shortPathHash(trimmedURL))
	}

	outExt := ".stbook"
	if urlExt == ".playbook" {
		outExt = ".playbook"
	}
	finalZipPath := filepath.Join(booksDir, bookID+outExt)
	_ = os.Remove(finalZipPath)
	if err := os.Rename(tmpPath, finalZipPath); err != nil {
		return nil, fmt.Errorf("błąd zapisu książki w bibliotece: %w", err)
	}

	loaded, err := LoadBookFromZip(finalZipPath)
	if err != nil {
		return nil, err
	}
	loaded.SourceURL = trimmedURL
	return loaded, nil
}

func extractFilenameFromURLOrHeader(u *url.URL, contentDisposition string) string {
	if contentDisposition != "" {
		for _, part := range strings.Split(contentDisposition, ";") {
			trimmed := strings.TrimSpace(part)
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(lower, "filename=") {
				val := strings.Trim(trimmed[len("filename="):], `"' `)
				if val != "" {
					return filepath.Base(val)
				}
			}
		}
	}
	if u != nil && u.Path != "" {
		base := filepath.Base(u.Path)
		if base != "." && base != "/" {
			if unescaped, err := url.PathUnescape(base); err == nil && unescaped != "" {
				return unescaped
			}
			return base
		}
	}
	return ""
}

func chooseDownloadedBookID(urlStem, urlExt string, preview *Book) string {
	cleanStem := SanitizeBookID(urlStem)
	genericStems := map[string]bool{
		"":         true,
		"book":     true,
		"story":    true,
		"download": true,
		"archive":  true,
		"main":     true,
		"master":   true,
		"head":     true,
		"latest":   true,
		"release":  true,
		"zip":      true,
		"stbook":   true,
		"playbook": true,
	}
	if preview != nil && preview.Title != "" {
		titleID := SanitizeBookID(preview.Title)
		if genericStems[cleanStem] && titleID != "" && titleID != "book" {
			return titleID
		}
		if urlExt != ".playbook" && urlExt != ".stbook" && urlExt != ".zip" && titleID != "" && titleID != "book" {
			return titleID
		}
	}
	if cleanStem != "" && cleanStem != "book" {
		return cleanStem
	}
	if preview != nil && preview.Title != "" {
		return SanitizeBookID(preview.Title)
	}
	return fmt.Sprintf("ksiazka-%d", time.Now().Unix()%10000)
}

func looksLikePlainTextStory(content string) bool {
	if strings.Contains(content, "\x00") {
		return false
	}
	lower := strings.ToLower(content)
	return strings.Contains(lower, "scene:") || strings.Contains(lower, "scena:") || strings.Contains(lower, "chapter:") || strings.Contains(lower, "rozdział:")
}

func wrapPlainStoryIntoZip(zipOutPath, storyContent, titleHint string) error {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	title := SanitizePlainText(titleHint)
	if title == "" {
		title = "Pobrana Opowieść"
	}
	dataContent := fmt.Sprintf("title: %s\nauthor: Autor\nnarrated: false\n", title)
	dw, err := zw.Create(BookDataFileName)
	if err != nil {
		return err
	}
	if _, err := dw.Write([]byte(dataContent)); err != nil {
		return err
	}
	sw, err := zw.Create(DefaultChapterFile)
	if err != nil {
		return err
	}
	if _, err := sw.Write([]byte(storyContent)); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(zipOutPath, buf.Bytes(), 0644)
}

// LoadBookFromDir loads an unpacked book folder from disk, supporting media files in subdirectories.
func LoadBookFromDir(bookDir string) (*Book, error) {
	entries, err := os.ReadDir(bookDir)
	if err != nil {
		return nil, err
	}

	folderName := filepath.Base(bookDir)
	bookID := SanitizeBookID(folderName)
	b := &Book{
		ID:             bookID,
		Title:          folderName,
		Author:         "Nieznany Autor",
		Description:    "Interaktywna opowieść.",
		IsPacked:       false,
		RootPath:       bookDir,
		Layout:         make(map[string]NodePosition),
		deferProseHTML: true,
	}

	var storyFiles []string
	var audioFiles []string
	var imageFiles []string

	for _, e := range entries {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 || isHiddenNonEditorFile(e.Name()) {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)

		switch {
		case lower == BookDataFileName:
			raw, err := readSafeBookTextFile(bookDir, name)
			if err == nil {
				ParseDataFile(raw, b)
			}
		case lower == EditorStateFileName || lower == "editor.cfg":
			raw, err := readSafeBookTextFile(bookDir, name)
			if err == nil {
				ParseEditorFile(raw, b)
			}
		case strings.HasSuffix(lower, PlaytxtExt):
			if lower == DefaultStoryFile {
				storyFiles = append(storyFiles, name)
			} else if raw, err := readSafeBookTextFile(bookDir, name); err == nil && looksLikePlainTextStory(raw) {
				storyFiles = append(storyFiles, name)
			}
		}
	}

	// Walk bookDir recursively to discover all raster images and audio files (including subdirectories)
	_ = filepath.WalkDir(bookDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != bookDir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || !IsSafePathInsideRoot(bookDir, path) {
			return nil
		}
		rel, err := filepath.Rel(bookDir, path)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		switch {
		case isAudioFilename(relSlash):
			audioFiles = append(audioFiles, relSlash)
		case isRasterImageFilename(relSlash):
			imageFiles = append(imageFiles, relSlash)
		}
		return nil
	})

	sortStoryFiles(storyFiles)
	b.TextFiles = storyFiles

	var totalDirTextBytes int
	for _, sf := range storyFiles {
		raw, err := readSafeBookTextFile(bookDir, sf)
		if err != nil {
			b.Warnings = append(b.Warnings, fmt.Sprintf("Nie można odczytać pliku %s: %v", sf, err))
			continue
		}
		if totalDirTextBytes+len(raw) > MaxZipTotalTextBytes {
			b.Warnings = append(b.Warnings, fmt.Sprintf("Pominięto plik %s: przekroczono łączny limit rozmiaru tekstu (32 MB)", sf))
			break
		}
		totalDirTextBytes += len(raw)
		ParseStoryFile(raw, sf, b)
	}

	ensureFallbackEmptyScene(b,
		"Błąd parsowania książki: nie znaleziono żadnych scen w plikach .playtxt (dodaj nagłówek scene: Nazwa sceny).",
		"Ta książka nie zawierała żadnych poprawnych scen podczas wczytywania.",
	)

	FinalizeBook(b)
	matchBookMedia(b, audioFiles, imageFiles)
	return b, nil
}

// LoadBookFromZip loads a packed .stbook or .zip archive, supporting media files in subdirectories.
func LoadBookFromZip(zipPath string) (*Book, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	base := filepath.Base(zipPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	b, _, err := loadBookFromZipFiles(zr.File, stem, zipPath, false)
	return b, err
}

func loadBookFromZipFiles(files []*zip.File, stem, zipPath string, extractMediaBytes bool) (*Book, map[string][]byte, error) {
	if err := validateZipArchiveLimits(files); err != nil {
		return nil, nil, err
	}

	bookID := SanitizeBookID(stem)
	if bookID == "" {
		bookID = "book"
	}

	b := &Book{
		ID:             bookID,
		Title:          stem,
		Author:         "Nieznany Autor",
		Description:    "Interaktywna opowieść (pakiet .stbook).",
		IsPacked:       true,
		RootPath:       zipPath,
		PackedMedia:    make(map[string]PackedMediaEntry),
		Layout:         make(map[string]NodePosition),
		deferProseHTML: true,
	}

	textMap := make(map[string]string)
	var mediaBytes map[string][]byte
	if extractMediaBytes {
		mediaBytes = make(map[string][]byte)
	}
	var storyFiles []string
	var audioFiles []string
	var imageFiles []string
	var totalTextBytes int
	var totalMediaBytes int64

	// Detect optional common top-level prefix inside zip if every entry shares one folder
	commonPrefix := detectZipCommonPrefix(files)

	for _, f := range files {
		if f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		relSlash, lower, ok := normalizeZipEntryRelPath(f, commonPrefix)
		if !ok {
			continue
		}

		switch {
		case lower == BookDataFileName:
			content, err := readZipTextWithTotalCap(f, &totalTextBytes)
			if err != nil {
				return nil, nil, err
			}
			ParseDataFile(content, b)
		case lower == EditorStateFileName || lower == "editor.cfg":
			content, err := readZipTextWithTotalCap(f, &totalTextBytes)
			if err != nil {
				return nil, nil, err
			}
			ParseEditorFile(content, b)
		case !strings.Contains(relSlash, "/") && strings.HasSuffix(lower, PlaytxtExt):
			content, err := readZipTextWithTotalCap(f, &totalTextBytes)
			if err != nil {
				return nil, nil, err
			}
			textMap[relSlash] = content
			storyFiles = append(storyFiles, relSlash)
		case isAudioFilename(relSlash):
			audioFiles = append(audioFiles, relSlash)
			b.PackedMedia[lower] = indexPackedMediaEntry(f, relSlash)
			if extractMediaBytes {
				rawMedia, err := readValidatedZipMediaBytes(f, relSlash, &totalMediaBytes)
				if err != nil {
					return nil, nil, err
				}
				mediaBytes[relSlash] = rawMedia
			}
		case isRasterImageFilename(relSlash):
			imageFiles = append(imageFiles, relSlash)
			b.PackedMedia[lower] = indexPackedMediaEntry(f, relSlash)
			if extractMediaBytes {
				rawMedia, err := readValidatedZipMediaBytes(f, relSlash, &totalMediaBytes)
				if err != nil {
					return nil, nil, err
				}
				mediaBytes[relSlash] = rawMedia
			}
		}
	}

	sortStoryFiles(storyFiles)
	b.TextFiles = storyFiles
	for _, sf := range storyFiles {
		ParseStoryFile(textMap[sf], sf, b)
	}

	ensureFallbackEmptyScene(b,
		"Błąd parsowania archiwum .stbook: nie znaleziono żadnych scen w plikach .playtxt.",
		"Archiwum książki nie zawierało żadnych poprawnych scen.",
	)

	FinalizeBook(b)
	matchBookMedia(b, audioFiles, imageFiles)
	return b, mediaBytes, nil
}

func readValidatedZipMediaBytes(f *zip.File, relSlash string, totalBytes *int64) ([]byte, error) {
	if f.UncompressedSize64 > maxMediaEntryBytes {
		return nil, fmt.Errorf("plik multimedialny %s przekracza limit rozmiaru (100 MB)", relSlash)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxMediaEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxMediaEntryBytes {
		return nil, fmt.Errorf("plik multimedialny %s przekracza limit rozmiaru (100 MB)", relSlash)
	}
	if totalBytes != nil {
		*totalBytes += int64(len(data))
		if *totalBytes > MaxZipTotalUncompressedBytes {
			return nil, fmt.Errorf("archiwum ZIP przekracza łączny limit rozmiaru multimediów (500 MB)")
		}
	}
	if !validateMediaHeader(relSlash, data) {
		return nil, fmt.Errorf("nieprawidłowy lub zamaskowany nagłówek pliku multimedialnego: %s", relSlash)
	}
	return data, nil
}

// UnpackBookZip extracts a packed .stbook or .zip archive into targetDir so it can be edited in Node Studio.
func UnpackBookZip(zipPath, targetDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	if err := validateZipArchiveLimits(zr.File); err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	var totalExtractedBytes int64
	commonPrefix := detectZipCommonPrefix(zr.File)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		relSlash, lower, ok := normalizeZipEntryRelPath(f, commonPrefix)
		if !ok {
			continue
		}
		isRootTxt := !strings.Contains(relSlash, "/") && (strings.HasSuffix(lower, PlaytxtExt) || lower == EditorStateFileName || lower == "editor.cfg")
		if !isRootTxt && !isAudioFilename(relSlash) && !isRasterImageFilename(relSlash) {
			continue
		}
		dstPath := filepath.Join(targetDir, filepath.FromSlash(relSlash))
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}
		if !IsSafePathInsideRoot(targetDir, dstPath) {
			return fmt.Errorf("niedozwolona ścieżka wypakowania poza katalogiem docelowym: %s", relSlash)
		}
		maxEntryCap := int64(MaxSingleMediaBytes)
		if isRootTxt {
			maxEntryCap = int64(MaxZipSingleTextBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, maxEntryCap+1))
		_ = rc.Close()
		if readErr != nil {
			return readErr
		}
		if int64(len(data)) > maxEntryCap {
			return fmt.Errorf("plik %s w archiwum przekracza dopuszczalny limit rozmiaru", relSlash)
		}
		totalExtractedBytes += int64(len(data))
		if totalExtractedBytes > MaxZipTotalUncompressedBytes {
			return fmt.Errorf("archiwum ZIP przekracza dopuszczalny limit rozmiaru po rozpakowaniu (500 MB)")
		}
		if (isAudioFilename(relSlash) || isRasterImageFilename(relSlash)) && !validateMediaHeader(relSlash, data) {
			return fmt.Errorf("nieprawidłowy lub zamaskowany nagłówek pliku multimedialnego: %s", relSlash)
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func isIgnoredZipEntry(slashed string) bool {
	if strings.HasPrefix(slashed, "__MACOSX/") || strings.Contains(slashed, "/__MACOSX/") {
		return true
	}
	return isHiddenNonEditorFile(slashed)
}

func detectZipCommonPrefix(files []*zip.File) string {
	var prefix string
	for _, f := range files {
		if f.FileInfo().IsDir() {
			continue
		}
		slashed := filepath.ToSlash(f.Name)
		if isIgnoredZipEntry(slashed) {
			continue
		}
		idx := strings.IndexByte(slashed, '/')
		if idx <= 0 {
			return ""
		}
		cand := slashed[:idx+1]
		if prefix == "" {
			prefix = cand
		} else if prefix != cand {
			return ""
		}
	}
	if prefix == "" {
		return ""
	}
	// Strip top-level folder prefix if a book .playtxt file lives inside that top folder
	for _, f := range files {
		slashed := filepath.ToSlash(f.Name)
		if isIgnoredZipEntry(slashed) {
			continue
		}
		trimmed := strings.TrimPrefix(slashed, prefix)
		lower := strings.ToLower(trimmed)
		if lower == BookDataFileName || lower == DefaultStoryFile || (!strings.Contains(trimmed, "/") && strings.HasSuffix(lower, PlaytxtExt)) {
			return prefix
		}
	}
	return ""
}

func indexPackedMediaEntry(f *zip.File, relSlash string) PackedMediaEntry {
	entry := PackedMediaEntry{
		Size:    int64(f.UncompressedSize64),
		Method:  f.Method,
		ModTime: f.Modified,
	}
	if off, err := f.DataOffset(); err == nil {
		entry.DataOffset = off
	}
	if f.UncompressedSize64 > 0 && f.UncompressedSize64 <= maxMediaEntryBytes {
		if rc, err := f.Open(); err == nil {
			n, _ := io.ReadFull(rc, entry.Header512[:])
			_ = rc.Close()
			entry.HeaderLen = n
			entry.ValidHeader = validateMediaHeader(relSlash, entry.Header512[:n])
		}
	}
	return entry
}

func readZipEntryText(f *zip.File) (string, error) {
	limitMsg := fmt.Sprintf("plik tekstowy %s w archiwum przekracza limit rozmiaru (4 MB)", f.Name)
	if f.UncompressedSize64 > MaxZipSingleTextBytes {
		return "", fmt.Errorf("%s", limitMsg)
	}
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	return readLimitedTextReader(rc, limitMsg)
}

func sortStoryFiles(files []string) {
	sort.Slice(files, func(i, j int) bool {
		li := strings.ToLower(files[i])
		lj := strings.ToLower(files[j])
		if li == DefaultStoryFile {
			return true
		}
		if lj == DefaultStoryFile {
			return false
		}
		return li < lj
	})
}

var (
	defaultTestAppDataOnce sync.Once
	defaultTestAppDataDir  string
)

// IsTempDir reports whether p is inside the OS temporary directory.
func IsTempDir(p string) bool {
	if p == "" {
		return false
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	clean := filepath.ToSlash(filepath.Clean(abs))
	if strings.HasPrefix(clean, "/tmp/") || strings.HasPrefix(clean, "/private/tmp/") {
		return true
	}
	if resolvedP, err := filepath.EvalSymlinks(abs); err == nil && resolvedP != "" {
		resPClean := filepath.ToSlash(filepath.Clean(resolvedP))
		if strings.HasPrefix(resPClean, "/tmp/") || strings.HasPrefix(resPClean, "/private/tmp/") {
			return true
		}
	}
	if td := os.TempDir(); td != "" {
		tdClean := filepath.ToSlash(filepath.Clean(td))
		if strings.HasPrefix(clean, tdClean+"/") || clean == tdClean {
			return true
		}
		if resolved, err := filepath.EvalSymlinks(td); err == nil && resolved != "" {
			resClean := filepath.ToSlash(filepath.Clean(resolved))
			if strings.HasPrefix(clean, resClean+"/") || clean == resClean {
				return true
			}
			if resolvedP, errP := filepath.EvalSymlinks(abs); errP == nil && resolvedP != "" {
				resPClean := filepath.ToSlash(filepath.Clean(resolvedP))
				if strings.HasPrefix(resPClean, resClean+"/") || resPClean == resClean {
					return true
				}
			}
		}
	}
	return false
}

// IsInsideRepoDir reports whether p is located inside a Go source repository (a directory containing go.mod).
func IsInsideRepoDir(p string) bool {
	if p == "" {
		return false
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	cur := filepath.Clean(abs)
	for depth := 0; depth < 256; depth++ {
		if info, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil && !info.IsDir() {
			return true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return false
}

// DefaultAppDataDir returns the standard OS application data directory for Mojra
// (e.g. ~/Library/Application Support/Mojra on macOS, %AppData%\Mojra on Windows, ~/.config/Mojra on Linux).
// During `go test`, it returns an isolated temporary directory so tests never touch user or repository state.
func DefaultAppDataDir() string {
	if envDir := strings.TrimSpace(os.Getenv("MOJRA_DATA_DIR")); envDir != "" {
		return filepath.Clean(envDir)
	}
	if isRunningUnderGoTest() {
		defaultTestAppDataOnce.Do(func() {
			if td, err := os.MkdirTemp("", "mojra-test-appdata-*"); err == nil && td != "" {
				defaultTestAppDataDir = td
				return
			}
			defaultTestAppDataDir = filepath.Join(os.TempDir(), "mojra-test-appdata")
		})
		return defaultTestAppDataDir
	}
	if cfgDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(cfgDir) != "" {
		return filepath.Join(cfgDir, "Mojra")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".config", "Mojra")
	}
	return filepath.Join(os.TempDir(), "Mojra")
}

// DefaultSavesDir returns the standard application saves/state directory inside DefaultAppDataDir.
func DefaultSavesDir() string {
	return filepath.Join(DefaultAppDataDir(), "saves")
}

// ResolveSavesDir returns the saves directory for rootDir.
// It always uses DefaultSavesDir() in standard OS application data except when rootDir is an isolated non-repo test temp directory.
func ResolveSavesDir(rootDir string) string {
	if IsTempDir(rootDir) && !IsInsideRepoDir(rootDir) && filepath.Clean(rootDir) != filepath.Clean(DefaultAppDataDir()) {
		return filepath.Join(rootDir, "saves")
	}
	return DefaultSavesDir()
}

// DefaultExportDir returns the default export directory inside standard application data.
func DefaultExportDir() string {
	return filepath.Join(DefaultAppDataDir(), "exports")
}

// DefaultProjectsDir returns the default directory for user-created projects inside standard application data.
func DefaultProjectsDir() string {
	return filepath.Join(DefaultAppDataDir(), "projects")
}

