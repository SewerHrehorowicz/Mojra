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

	// Second pass: packed .stbook or .zip files (only if not already loaded as an unpacked folder)
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		lower := strings.ToLower(e.Name())
		if !strings.HasSuffix(lower, ".stbook") && !strings.HasSuffix(lower, ".zip") {
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
		if sc.ImageFile != "" {
			sc.ImageURL = encodeMediaURL(b.ID, sc.ImageFile)
		}
	}
}

// LoadBookFromPath loads either an unpacked book directory or a packed .stbook/.zip archive from any path on disk.
func LoadBookFromPath(bookPath string) (*Book, error) {
	info, err := os.Stat(bookPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return LoadBookFromDir(bookPath)
	}
	lower := strings.ToLower(info.Name())
	if strings.HasSuffix(lower, ".stbook") || strings.HasSuffix(lower, ".zip") {
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
		if _, exists := byID[b.ID]; exists {
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
	return filepath.Abs(filepath.Clean(trimmed))
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

// HasBookFilesInDir checks whether a directory contains at least one .txt book file (such as story.txt, data.txt, or a .txt file with StoryTeller headers).
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
		if lower == "data.txt" || lower == "story.txt" {
			return true
		}
		if strings.HasSuffix(lower, ".txt") && lower != "editor.txt" && lower != "nodes.txt" {
			if fileLooksLikeBookStory(filepath.Join(dir, e.Name())) {
				return true
			}
		}
	}
	return false
}

// ValidateBookZip checks whether a .stbook or .zip archive is a valid zip containing StoryTeller .txt files.
func ValidateBookZip(zipPath string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("nieprawidłowe archiwum ZIP / .stbook: %w", err)
	}
	defer zr.Close()

	commonPrefix := detectZipCommonPrefix(zr.File)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		relSlash := filepath.ToSlash(f.Name)
		if isIgnoredZipEntry(relSlash) {
			continue
		}
		if commonPrefix != "" && strings.HasPrefix(relSlash, commonPrefix) {
			relSlash = strings.TrimPrefix(relSlash, commonPrefix)
		}
		relSlash = SanitizeRelativeMediaPath(relSlash)
		if relSlash == "" || strings.HasPrefix(filepath.Base(relSlash), ".") {
			continue
		}
		lower := strings.ToLower(relSlash)
		if lower == "data.txt" || lower == "story.txt" || (!strings.Contains(relSlash, "/") && strings.HasSuffix(lower, ".txt") && lower != "editor.txt" && lower != "nodes.txt") {
			return nil
		}
	}
	return fmt.Errorf("archiwum nie zawiera plików opowieści (.txt / story.txt / data.txt)")
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
		if strings.HasSuffix(lower, ".stbook") || strings.HasSuffix(lower, ".zip") {
			if err := ValidateBookZip(absPath); err != nil {
				return nil, err
			}
			return []string{absPath}, nil
		}
		if strings.HasSuffix(lower, ".txt") {
			parentDir := filepath.Dir(absPath)
			if HasBookFilesInDir(parentDir) {
				return []string{parentDir}, nil
			}
		}
		return nil, fmt.Errorf("nieobsługiwany plik „%s” (wskaż folder książki lub archiwum .stbook / .zip)", info.Name())
	}

	if HasBookFilesInDir(absPath) {
		return []string{absPath}, nil
	}

	// Check if the chosen folder is a parent directory containing multiple book folders or .stbook/.zip files
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
		if strings.HasSuffix(lower, ".stbook") || strings.HasSuffix(lower, ".zip") {
			if ValidateBookZip(childPath) == nil {
				found = append(found, childPath)
			}
		}
	}
	if len(found) > 0 {
		sort.Strings(found)
		return found, nil
	}
	return nil, fmt.Errorf("w katalogu „%s” nie znaleziono książki StoryTeller (brak story.txt / data.txt ani archiwów .stbook / .zip)", filepath.Base(absPath))
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
	req.Header.Set("User-Agent", "StoryTeller/1.0")

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

	finalZipPath := filepath.Join(booksDir, bookID+".stbook")
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
	}
	if preview != nil && preview.Title != "" {
		titleID := SanitizeBookID(preview.Title)
		if genericStems[cleanStem] && titleID != "" && titleID != "book" {
			return titleID
		}
		if urlExt != ".stbook" && urlExt != ".zip" && titleID != "" && titleID != "book" {
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
	dw, err := zw.Create("data.txt")
	if err != nil {
		return err
	}
	if _, err := dw.Write([]byte(dataContent)); err != nil {
		return err
	}
	sw, err := zw.Create("rozdzial_1.txt")
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
		ID:          bookID,
		Title:       folderName,
		Author:      "Nieznany Autor",
		Description: "Interaktywna opowieść.",
		IsPacked:    false,
		RootPath:    bookDir,
		Layout:      make(map[string]NodePosition),
	}

	var storyFiles []string
	var audioFiles []string
	var imageFiles []string

	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)

		switch {
		case lower == "data.txt":
			raw, err := os.ReadFile(filepath.Join(bookDir, name))
			if err == nil {
				ParseDataFile(string(raw), b)
			}
		case lower == "editor.txt" || lower == "nodes.txt":
			raw, err := os.ReadFile(filepath.Join(bookDir, name))
			if err == nil {
				ParseEditorFile(string(raw), b)
			}
		case strings.HasSuffix(lower, ".txt"):
			if lower == "story.txt" {
				storyFiles = append(storyFiles, name)
			} else if raw, err := os.ReadFile(filepath.Join(bookDir, name)); err == nil && looksLikePlainTextStory(string(raw)) {
				storyFiles = append(storyFiles, name)
			}
		}
	}

	// Walk bookDir recursively to discover all raster images and audio files (including subdirectories)
	_ = filepath.WalkDir(bookDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != bookDir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
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

	for _, sf := range storyFiles {
		raw, err := os.ReadFile(filepath.Join(bookDir, sf))
		if err != nil {
			b.Warnings = append(b.Warnings, fmt.Sprintf("Nie można odczytać pliku %s: %v", sf, err))
			continue
		}
		ParseStoryFile(string(raw), sf, b)
	}

	if len(b.Scenes) == 0 {
		b.Warnings = append(b.Warnings, "Błąd parsowania książki: nie znaleziono żadnych scen w plikach .txt (dodaj nagłówek scene: Nazwa sceny).")
		b.Scenes = []*Scene{
			{
				ID:          "s1",
				Name:        "Pusta scena",
				Chapter:     "Rozdział 1",
				ChapterID:   "rozdzial_1",
				Text:        "Ta książka nie zawierała żadnych poprawnych scen podczas wczytywania.",
				HTML:        RenderSafeProseHTML("Ta książka nie zawierała żadnych poprawnych scen podczas wczytywania."),
				SourceFile:  "rozdzial_1.txt",
				ExplicitEnd: true,
			},
		}
	}

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
	bookID := SanitizeBookID(stem)

	b := &Book{
		ID:          bookID,
		Title:       stem,
		Author:      "Nieznany Autor",
		Description: "Interaktywna opowieść (pakiet .stbook).",
		IsPacked:    true,
		RootPath:    zipPath,
		Layout:      make(map[string]NodePosition),
	}

	textMap := make(map[string]string)
	var storyFiles []string
	var audioFiles []string
	var imageFiles []string

	// Detect optional common top-level prefix inside zip if every entry shares one folder
	commonPrefix := detectZipCommonPrefix(zr.File)

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		relSlash := filepath.ToSlash(f.Name)
		if isIgnoredZipEntry(relSlash) {
			continue
		}
		if commonPrefix != "" && strings.HasPrefix(relSlash, commonPrefix) {
			relSlash = strings.TrimPrefix(relSlash, commonPrefix)
		}
		relSlash = SanitizeRelativeMediaPath(relSlash)
		if relSlash == "" || strings.HasPrefix(filepath.Base(relSlash), ".") {
			continue
		}
		lower := strings.ToLower(relSlash)

		switch {
		case lower == "data.txt":
			if content, err := readZipEntryText(f); err == nil {
				ParseDataFile(content, b)
			}
		case lower == "editor.txt" || lower == "nodes.txt":
			if content, err := readZipEntryText(f); err == nil {
				ParseEditorFile(content, b)
			}
		case !strings.Contains(relSlash, "/") && strings.HasSuffix(lower, ".txt"):
			if content, err := readZipEntryText(f); err == nil {
				textMap[relSlash] = content
				storyFiles = append(storyFiles, relSlash)
			}
		case isAudioFilename(relSlash):
			audioFiles = append(audioFiles, relSlash)
		case isRasterImageFilename(relSlash):
			imageFiles = append(imageFiles, relSlash)
		}
	}

	sortStoryFiles(storyFiles)
	b.TextFiles = storyFiles
	for _, sf := range storyFiles {
		ParseStoryFile(textMap[sf], sf, b)
	}

	if len(b.Scenes) == 0 {
		b.Warnings = append(b.Warnings, "Błąd parsowania archiwum .stbook: nie znaleziono żadnych scen w plikach .txt.")
		b.Scenes = []*Scene{
			{
				ID:          "s1",
				Name:        "Pusta scena",
				Chapter:     "Rozdział 1",
				ChapterID:   "rozdzial_1",
				Text:        "Archiwum książki nie zawierało żadnych poprawnych scen.",
				HTML:        RenderSafeProseHTML("Archiwum książki nie zawierało żadnych poprawnych scen."),
				SourceFile:  "rozdzial_1.txt",
				ExplicitEnd: true,
			},
		}
	}

	FinalizeBook(b)
	matchBookMedia(b, audioFiles, imageFiles)
	return b, nil
}

func isIgnoredZipEntry(slashed string) bool {
	if strings.HasPrefix(slashed, "__MACOSX/") || strings.Contains(slashed, "/__MACOSX/") {
		return true
	}
	base := filepath.Base(slashed)
	return strings.HasPrefix(base, ".")
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
	// Strip top-level folder prefix if a book .txt file lives inside that top folder
	for _, f := range files {
		slashed := filepath.ToSlash(f.Name)
		if isIgnoredZipEntry(slashed) {
			continue
		}
		trimmed := strings.TrimPrefix(slashed, prefix)
		lower := strings.ToLower(trimmed)
		if lower == "data.txt" || lower == "story.txt" || (!strings.Contains(trimmed, "/") && strings.HasSuffix(lower, ".txt")) {
			return prefix
		}
	}
	return ""
}

func readZipEntryText(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 4*1024*1024))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func sortStoryFiles(files []string) {
	sort.Slice(files, func(i, j int) bool {
		li := strings.ToLower(files[i])
		lj := strings.ToLower(files[j])
		if li == "story.txt" {
			return true
		}
		if lj == "story.txt" {
			return false
		}
		return li < lj
	})
}
