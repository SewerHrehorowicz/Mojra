package book

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	MaxUnnamedHistoryRevisions = 100
	HistoryCoalesceWindowSec   = 3
)

var validHexHashRegexp = regexp.MustCompile(`^[a-f0-9]{64}$`)

// HistoryRevision represents a single snapshot in books/<id>/.history/history.txt.
type HistoryRevision struct {
	ID              string
	TimeUnix        int64
	Kind            string
	Named           bool
	Label           string
	SelectedSceneID string
	CoalesceKey     string
	Files           map[string]string
}

// BookHistory holds the parsed state of books/<id>/.history/history.txt.
type BookHistory struct {
	Cursor    string
	NextSeq   int
	Revisions []HistoryRevision
}

func bookHistoryDir(bookRoot string) string {
	return filepath.Join(bookRoot, ".history")
}

func bookHistoryIndexPath(bookRoot string) string {
	return filepath.Join(bookHistoryDir(bookRoot), "history.txt")
}

func bookHistoryBlobsDir(bookRoot string) string {
	return filepath.Join(bookHistoryDir(bookRoot), "blobs")
}

func bookHistoryBlobPath(bookRoot, hash string) string {
	return filepath.Join(bookHistoryBlobsDir(bookRoot), strings.ToLower(strings.TrimSpace(hash))+".gz")
}

// CursorIndex returns the index of the current cursor revision, or -1 if empty.
func (h *BookHistory) CursorIndex() int {
	if h == nil || len(h.Revisions) == 0 {
		return -1
	}
	for i := range h.Revisions {
		if h.Revisions[i].ID == h.Cursor {
			return i
		}
	}
	return len(h.Revisions) - 1
}

// LastNamedIndex returns the index of the latest Named Version (named: true), or -1 if none.
func (h *BookHistory) LastNamedIndex() int {
	if h == nil {
		return -1
	}
	for i := len(h.Revisions) - 1; i >= 0; i-- {
		if h.Revisions[i].Named {
			return i
		}
	}
	return -1
}

// CanUndo returns true if the cursor can step back without crossing the latest Named Version floor.
func (h *BookHistory) CanUndo() bool {
	if h == nil || len(h.Revisions) == 0 {
		return false
	}
	curIdx := h.CursorIndex()
	floorIdx := h.LastNamedIndex()
	if floorIdx < 0 {
		floorIdx = 0
	}
	return curIdx > floorIdx
}

// CanRedo returns true if there is a newer revision after the current cursor.
func (h *BookHistory) CanRedo() bool {
	if h == nil || len(h.Revisions) == 0 {
		return false
	}
	curIdx := h.CursorIndex()
	return curIdx >= 0 && curIdx < len(h.Revisions)-1
}

// UnnamedEditsCount returns the number of active unnamed edits applied after the latest Named Version up to the cursor.
func (h *BookHistory) UnnamedEditsCount() int {
	if h == nil || len(h.Revisions) == 0 {
		return 0
	}
	curIdx := h.CursorIndex()
	floorIdx := h.LastNamedIndex()
	if floorIdx < 0 {
		floorIdx = 0
	}
	if curIdx <= floorIdx {
		return 0
	}
	count := 0
	for i := floorIdx + 1; i <= curIdx && i < len(h.Revisions); i++ {
		if !h.Revisions[i].Named {
			count++
		}
	}
	return count
}

// ParseHistoryFile parses a human-readable key: value history.txt file.
func ParseHistoryFile(content string) *BookHistory {
	h := &BookHistory{
		NextSeq: 1,
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")

	var current *HistoryRevision
	flushCurrent := func() {
		if current != nil && current.ID != "" {
			if current.Files == nil {
				current.Files = make(map[string]string)
			}
			h.Revisions = append(h.Revisions, *current)
			if strings.HasPrefix(current.ID, "r") {
				if seq, err := strconv.Atoi(strings.TrimPrefix(current.ID, "r")); err == nil && seq >= h.NextSeq {
					h.NextSeq = seq + 1
				}
			}
		}
		current = nil
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		val := strings.TrimSpace(line[colonIdx+1:])

		if key == "revision" {
			flushCurrent()
			revID := SanitizeSceneID(val)
			if revID == "" {
				continue
			}
			current = &HistoryRevision{
				ID:    revID,
				Files: make(map[string]string),
			}
			continue
		}

		if current == nil {
			switch key {
			case "cursor":
				h.Cursor = SanitizeSceneID(val)
			case "next_seq":
				if seq, err := strconv.Atoi(val); err == nil && seq > h.NextSeq {
					h.NextSeq = seq
				}
			}
			continue
		}

		switch key {
		case "time":
			if ts, err := strconv.ParseInt(val, 10, 64); err == nil {
				current.TimeUnix = ts
			}
		case "kind":
			current.Kind = SanitizePlainText(val)
		case "named":
			lowerVal := strings.ToLower(val)
			current.Named = lowerVal == "true" || lowerVal == "1" || lowerVal == "yes"
		case "label":
			current.Label = SanitizePlainText(val)
		case "scene":
			current.SelectedSceneID = SanitizePlainText(val)
		case "coalesce":
			current.CoalesceKey = SanitizePlainText(val)
		case "file":
			arrowIdx := strings.Index(val, "->")
			if arrowIdx <= 0 {
				continue
			}
			fileName := filepath.Base(strings.TrimSpace(val[:arrowIdx]))
			hashHex := strings.ToLower(strings.TrimSpace(val[arrowIdx+2:]))
			if fileName != "" && fileName != "." && validHexHashRegexp.MatchString(hashHex) {
				current.Files[fileName] = hashHex
			}
		}
	}
	flushCurrent()

	if h.Cursor == "" && len(h.Revisions) > 0 {
		h.Cursor = h.Revisions[len(h.Revisions)-1].ID
	}
	return h
}

// SerializeHistoryFile serializes BookHistory into the human-readable key: value format.
func SerializeHistoryFile(h *BookHistory) string {
	if h == nil {
		h = &BookHistory{NextSeq: 1}
	}
	var sb strings.Builder
	sb.WriteString("# StoryTeller — Historia edycji książki (history.txt)\n")
	sb.WriteString(fmt.Sprintf("cursor: %s\n", SanitizeSceneID(h.Cursor)))
	nextSeq := h.NextSeq
	if nextSeq < len(h.Revisions)+1 {
		nextSeq = len(h.Revisions) + 1
	}
	sb.WriteString(fmt.Sprintf("next_seq: %d\n", nextSeq))

	for _, rev := range h.Revisions {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("revision: %s\n", SanitizeSceneID(rev.ID)))
		sb.WriteString(fmt.Sprintf("time: %d\n", rev.TimeUnix))
		kind := SanitizePlainText(rev.Kind)
		if kind == "" {
			kind = "edit"
		}
		sb.WriteString(fmt.Sprintf("kind: %s\n", kind))
		if rev.Named {
			sb.WriteString("named: true\n")
		} else {
			sb.WriteString("named: false\n")
		}
		sb.WriteString(fmt.Sprintf("label: %s\n", SanitizePlainText(rev.Label)))
		sb.WriteString(fmt.Sprintf("scene: %s\n", SanitizePlainText(rev.SelectedSceneID)))
		if !rev.Named && rev.CoalesceKey != "" {
			sb.WriteString(fmt.Sprintf("coalesce: %s\n", SanitizePlainText(rev.CoalesceKey)))
		}
		fileNames := make([]string, 0, len(rev.Files))
		for k := range rev.Files {
			fileNames = append(fileNames, k)
		}
		sort.Strings(fileNames)
		for _, fn := range fileNames {
			sb.WriteString(fmt.Sprintf("file: %s -> %s\n", fn, rev.Files[fn]))
		}
	}
	return sb.String()
}

// LoadBookHistory reads books/<id>/.history/history.txt or returns an empty history if none exists yet.
func LoadBookHistory(b *Book) (*BookHistory, error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return &BookHistory{NextSeq: 1}, nil
	}
	raw, err := os.ReadFile(bookHistoryIndexPath(b.RootPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &BookHistory{NextSeq: 1}, nil
		}
		return &BookHistory{NextSeq: 1}, err
	}
	return ParseHistoryFile(string(raw)), nil
}

// SaveBookHistory writes books/<id>/.history/history.txt atomically.
func SaveBookHistory(b *Book, h *BookHistory) error {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" || h == nil {
		return nil
	}
	histDir := bookHistoryDir(b.RootPath)
	if err := os.MkdirAll(histDir, 0755); err != nil {
		return err
	}
	indexPath := bookHistoryIndexPath(b.RootPath)
	tmpPath := indexPath + ".tmp"
	serialized := SerializeHistoryFile(h)
	if err := os.WriteFile(tmpPath, []byte(serialized), 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, indexPath)
}

func writeHistoryBlob(bookRoot string, data []byte) (string, error) {
	return writeHistoryBlobWithSet(bookRoot, data, nil)
}

func writeHistoryBlobWithSet(bookRoot string, data []byte, existingBlobs map[string]bool) (string, error) {
	sum := sha256.Sum256(data)
	hashHex := hex.EncodeToString(sum[:])
	blobFileName := hashHex + ".gz"
	if existingBlobs != nil && existingBlobs[blobFileName] {
		return hashHex, nil
	}
	blobPath := bookHistoryBlobPath(bookRoot, hashHex)
	if _, err := os.Stat(blobPath); err == nil {
		if existingBlobs != nil {
			existingBlobs[blobFileName] = true
		}
		return hashHex, nil
	}
	if err := os.MkdirAll(bookHistoryBlobsDir(bookRoot), 0755); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return "", err
	}
	if _, err := gw.Write(data); err != nil {
		_ = gw.Close()
		return "", err
	}
	if err := gw.Close(); err != nil {
		return "", err
	}
	tmpPath := blobPath + ".tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, blobPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if existingBlobs != nil {
		existingBlobs[blobFileName] = true
	}
	return hashHex, nil
}

func readHistoryBlob(bookRoot, hashHex string) ([]byte, error) {
	if !validHexHashRegexp.MatchString(strings.ToLower(strings.TrimSpace(hashHex))) {
		return nil, fmt.Errorf("nieprawidłowy skrót pliku historii: %s", hashHex)
	}
	blobPath := bookHistoryBlobPath(bookRoot, hashHex)
	rawGz, err := os.ReadFile(blobPath)
	if err != nil {
		return nil, err
	}
	gr, err := gzip.NewReader(bytes.NewReader(rawGz))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	return io.ReadAll(io.LimitReader(gr, 32*1024*1024))
}

func captureBookTextFiles(b *Book) (map[string]string, error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return nil, fmt.Errorf("brak rozpakowanej książki na dysku")
	}
	entries, err := os.ReadDir(b.RootPath)
	if err != nil {
		return nil, err
	}
	existingBlobs := make(map[string]bool)
	if blobEntries, bErr := os.ReadDir(bookHistoryBlobsDir(b.RootPath)); bErr == nil {
		for _, be := range blobEntries {
			if !be.IsDir() {
				existingBlobs[strings.ToLower(be.Name())] = true
			}
		}
	}
	filesMap := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".txt") {
			continue
		}
		raw, rErr := os.ReadFile(filepath.Join(b.RootPath, name))
		if rErr != nil {
			continue
		}
		hashHex, wErr := writeHistoryBlobWithSet(b.RootPath, raw, existingBlobs)
		if wErr != nil {
			return nil, wErr
		}
		filesMap[name] = hashHex
	}
	if _, hasData := filesMap["data.txt"]; !hasData {
		hashHex, wErr := writeHistoryBlobWithSet(b.RootPath, []byte(SerializeDataFile(b)), existingBlobs)
		if wErr == nil {
			filesMap["data.txt"] = hashHex
		}
	}
	if _, hasEditor := filesMap["editor.txt"]; !hasEditor {
		hashHex, wErr := writeHistoryBlobWithSet(b.RootPath, []byte(SerializeEditorFile(b)), existingBlobs)
		if wErr == nil {
			filesMap["editor.txt"] = hashHex
		}
	}
	return filesMap, nil
}

func restoreBookTextFiles(b *Book, rev HistoryRevision) error {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return fmt.Errorf("brak rozpakowanej książki na dysku")
	}
	if len(rev.Files) == 0 {
		return fmt.Errorf("rewizja %s nie zawiera plików", rev.ID)
	}

	// Decompress all blobs into memory first so restoration is atomic if any blob is missing.
	restoredPayloads := make(map[string][]byte, len(rev.Files))
	expectedLower := make(map[string]bool, len(rev.Files))
	for fileName, hashHex := range rev.Files {
		cleanName := filepath.Base(strings.TrimSpace(fileName))
		if cleanName == "" || cleanName == "." || !strings.HasSuffix(strings.ToLower(cleanName), ".txt") {
			continue
		}
		data, err := readHistoryBlob(b.RootPath, hashHex)
		if err != nil {
			return fmt.Errorf("nie można odczytać migawki pliku %s: %w", cleanName, err)
		}
		restoredPayloads[cleanName] = data
		expectedLower[strings.ToLower(cleanName)] = true
	}

	for cleanName, data := range restoredPayloads {
		outPath := filepath.Join(b.RootPath, cleanName)
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return err
		}
	}

	// Remove any top-level .txt files in the book directory that did not exist in this revision.
	if entries, err := os.ReadDir(b.RootPath); err == nil {
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".txt") && !expectedLower[lower] {
				_ = os.Remove(filepath.Join(b.RootPath, name))
			}
		}
	}
	return nil
}

func equalFileHashMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok || va != vb {
			return false
		}
	}
	return true
}

func cloneFileHashMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// PruneUnreferencedBlobs deletes any .gz files in .history/blobs/ not referenced by h.Revisions.
func PruneUnreferencedBlobs(b *Book, h *BookHistory) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" || h == nil {
		return
	}
	referenced := make(map[string]bool)
	for _, rev := range h.Revisions {
		for _, hashHex := range rev.Files {
			referenced[strings.ToLower(strings.TrimSpace(hashHex))+".gz"] = true
		}
	}
	blobsDir := bookHistoryBlobsDir(b.RootPath)
	entries, err := os.ReadDir(blobsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".gz") && !referenced[name] {
			_ = os.Remove(filepath.Join(blobsDir, e.Name()))
		}
	}
}

// EnsureBookHistoryBaseline captures the initial state ("Stan początkowy") before the first edit if no history exists yet.
func EnsureBookHistoryBaseline(b *Book, selectedSceneID string) error {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return nil
	}
	h, _ := LoadBookHistory(b)
	if len(h.Revisions) > 0 {
		return nil
	}
	filesMap, err := captureBookTextFiles(b)
	if err != nil {
		return err
	}
	if selectedSceneID == "" {
		selectedSceneID = b.StartSceneID
		if selectedSceneID == "" && len(b.Scenes) > 0 {
			selectedSceneID = b.Scenes[0].ID
		}
	}
	rev := HistoryRevision{
		ID:              "r1",
		TimeUnix:        time.Now().Unix(),
		Kind:            "baseline",
		Named:           true,
		Label:           "Stan początkowy",
		SelectedSceneID: selectedSceneID,
		Files:           filesMap,
	}
	h.Cursor = "r1"
	h.NextSeq = 2
	h.Revisions = []HistoryRevision{rev}
	return SaveBookHistory(b, h)
}

func truncateAfterCursorPreservingNamed(h *BookHistory) {
	curIdx := h.CursorIndex()
	if curIdx < 0 || curIdx >= len(h.Revisions)-1 {
		return
	}
	kept := make([]HistoryRevision, 0, len(h.Revisions))
	for i, rev := range h.Revisions {
		if i <= curIdx || rev.Named {
			kept = append(kept, rev)
		}
	}
	h.Revisions = kept
}

// RecordBookHistoryStep records an unnamed edit step after a book mutation, coalescing rapid identical keys.
func RecordBookHistoryStep(b *Book, label, kind, coalesceKey, selectedSceneID string) error {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return nil
	}
	h, _ := LoadBookHistory(b)
	if len(h.Revisions) == 0 {
		if err := EnsureBookHistoryBaseline(b, selectedSceneID); err != nil {
			return err
		}
		h, _ = LoadBookHistory(b)
	}

	filesMap, err := captureBookTextFiles(b)
	if err != nil {
		return err
	}

	curIdx := h.CursorIndex()
	if curIdx >= 0 && curIdx < len(h.Revisions) {
		if equalFileHashMaps(filesMap, h.Revisions[curIdx].Files) {
			return nil
		}
	}

	truncateAfterCursorPreservingNamed(h)
	curIdx = h.CursorIndex()

	now := time.Now().Unix()
	cleanLabel := SanitizePlainText(label)
	if cleanLabel == "" {
		cleanLabel = "Edycja książki"
	}
	cleanKind := SanitizePlainText(kind)
	if cleanKind == "" {
		cleanKind = "edit"
	}
	cleanCoalesce := SanitizePlainText(coalesceKey)
	if selectedSceneID == "" {
		selectedSceneID = b.StartSceneID
		if selectedSceneID == "" && len(b.Scenes) > 0 {
			selectedSceneID = b.Scenes[0].ID
		}
	}

	if curIdx >= 0 && curIdx == len(h.Revisions)-1 {
		curRev := &h.Revisions[curIdx]
		if !curRev.Named && cleanCoalesce != "" && curRev.CoalesceKey == cleanCoalesce && (now-curRev.TimeUnix) <= HistoryCoalesceWindowSec {
			curRev.TimeUnix = now
			curRev.Label = cleanLabel
			curRev.Kind = cleanKind
			curRev.SelectedSceneID = selectedSceneID
			curRev.Files = filesMap
			if err := SaveBookHistory(b, h); err != nil {
				return err
			}
			PruneUnreferencedBlobs(b, h)
			return nil
		}
	}

	if h.NextSeq < 1 {
		h.NextSeq = len(h.Revisions) + 1
	}
	revID := fmt.Sprintf("r%d", h.NextSeq)
	h.NextSeq++

	newRev := HistoryRevision{
		ID:              revID,
		TimeUnix:        now,
		Kind:            cleanKind,
		Named:           false,
		Label:           cleanLabel,
		SelectedSceneID: selectedSceneID,
		CoalesceKey:     cleanCoalesce,
		Files:           filesMap,
	}
	h.Revisions = append(h.Revisions, newRev)
	h.Cursor = revID

	// Enforce rolling cap of MaxUnnamedHistoryRevisions after the latest Named Version.
	for {
		lastNamedIdx := h.LastNamedIndex()
		unnamedCount := 0
		oldestUnnamedIdx := -1
		for i := lastNamedIdx + 1; i < len(h.Revisions); i++ {
			if !h.Revisions[i].Named {
				unnamedCount++
				if oldestUnnamedIdx == -1 {
					oldestUnnamedIdx = i
				}
			}
		}
		if unnamedCount <= MaxUnnamedHistoryRevisions || oldestUnnamedIdx < 0 {
			break
		}
		h.Revisions = append(h.Revisions[:oldestUnnamedIdx], h.Revisions[oldestUnnamedIdx+1:]...)
	}

	if err := SaveBookHistory(b, h); err != nil {
		return err
	}
	PruneUnreferencedBlobs(b, h)
	return nil
}

// SaveNamedBookVersion collapses all unnamed edits into a permanent Named Version and prunes unreferenced blobs.
func SaveNamedBookVersion(b *Book, versionName, selectedSceneID string) (*HistoryRevision, error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return nil, fmt.Errorf("brak rozpakowanej książki na dysku")
	}
	if err := EnsureBookHistoryBaseline(b, selectedSceneID); err != nil {
		return nil, err
	}
	h, err := LoadBookHistory(b)
	if err != nil {
		return nil, err
	}
	filesMap, err := captureBookTextFiles(b)
	if err != nil {
		return nil, err
	}

	cleanName := SanitizePlainText(versionName)
	if cleanName == "" {
		cleanName = fmt.Sprintf("Wersja %d", len(h.Revisions)+1)
	}
	if selectedSceneID == "" {
		if curIdx := h.CursorIndex(); curIdx >= 0 {
			selectedSceneID = h.Revisions[curIdx].SelectedSceneID
		}
		if selectedSceneID == "" {
			selectedSceneID = b.StartSceneID
		}
	}

	var keptNamed []HistoryRevision
	for _, rev := range h.Revisions {
		if rev.Named {
			keptNamed = append(keptNamed, rev)
		}
	}

	if h.NextSeq < 1 {
		h.NextSeq = len(keptNamed) + 1
	}
	revID := fmt.Sprintf("r%d", h.NextSeq)
	h.NextSeq++

	newVer := HistoryRevision{
		ID:              revID,
		TimeUnix:        time.Now().Unix(),
		Kind:            "version",
		Named:           true,
		Label:           cleanName,
		SelectedSceneID: selectedSceneID,
		Files:           filesMap,
	}
	keptNamed = append(keptNamed, newVer)
	h.Revisions = keptNamed
	h.Cursor = newVer.ID

	if err := SaveBookHistory(b, h); err != nil {
		return nil, err
	}
	PruneUnreferencedBlobs(b, h)
	return &newVer, nil
}

// UndoBookHistory steps backward by one unnamed edit (stopping at the latest Named Version floor) and restores files to disk.
func UndoBookHistory(b *Book) (selectedSceneID string, label string, err error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return "", "", fmt.Errorf("brak aktywnej książki")
	}
	h, err := LoadBookHistory(b)
	if err != nil {
		return "", "", err
	}
	if !h.CanUndo() {
		return "", "", fmt.Errorf("brak zmian do cofnięcia od ostatniej zapisanej wersji")
	}
	curIdx := h.CursorIndex()
	undoneRev := h.Revisions[curIdx]
	targetRev := h.Revisions[curIdx-1]

	if err := restoreBookTextFiles(b, targetRev); err != nil {
		return "", "", err
	}
	h.Cursor = targetRev.ID
	if err := SaveBookHistory(b, h); err != nil {
		return "", "", err
	}
	return targetRev.SelectedSceneID, undoneRev.Label, nil
}

// RedoBookHistory steps forward by one unnamed edit and restores files to disk.
func RedoBookHistory(b *Book) (selectedSceneID string, label string, err error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return "", "", fmt.Errorf("brak aktywnej książki")
	}
	h, err := LoadBookHistory(b)
	if err != nil {
		return "", "", err
	}
	if !h.CanRedo() {
		return "", "", fmt.Errorf("brak zmian do ponowienia")
	}
	curIdx := h.CursorIndex()
	targetRev := h.Revisions[curIdx+1]

	if err := restoreBookTextFiles(b, targetRev); err != nil {
		return "", "", err
	}
	h.Cursor = targetRev.ID
	if err := SaveBookHistory(b, h); err != nil {
		return "", "", err
	}
	return targetRev.SelectedSceneID, targetRev.Label, nil
}

// RestoreBookHistoryRevision restores any unnamed step or Named Version.
// Restoring an older Named Version (prior to the latest Named Version) is non-destructive:
// it copies that version's files onto the top of the current unnamed edit stack so later Named Versions are preserved.
func RestoreBookHistoryRevision(b *Book, revID string) (selectedSceneID string, label string, err error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return "", "", fmt.Errorf("brak aktywnej książki")
	}
	cleanID := SanitizeSceneID(revID)
	if cleanID == "" {
		return "", "", fmt.Errorf("nieprawidłowy identyfikator rewizji")
	}
	h, err := LoadBookHistory(b)
	if err != nil {
		return "", "", err
	}
	targetIdx := -1
	for i := range h.Revisions {
		if h.Revisions[i].ID == cleanID {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return "", "", fmt.Errorf("nie znaleziono rewizji %s", cleanID)
	}

	targetRev := h.Revisions[targetIdx]
	lastNamedIdx := h.LastNamedIndex()

	if err := restoreBookTextFiles(b, targetRev); err != nil {
		return "", "", err
	}

	if targetRev.Named && targetIdx < lastNamedIdx {
		truncateAfterCursorPreservingNamed(h)
		if h.NextSeq < 1 {
			h.NextSeq = len(h.Revisions) + 1
		}
		newID := fmt.Sprintf("r%d", h.NextSeq)
		h.NextSeq++
		restoredStep := HistoryRevision{
			ID:              newID,
			TimeUnix:        time.Now().Unix(),
			Kind:            "restore",
			Named:           false,
			Label:           "Przywrócono wersję: " + targetRev.Label,
			SelectedSceneID: targetRev.SelectedSceneID,
			Files:           cloneFileHashMap(targetRev.Files),
		}
		h.Revisions = append(h.Revisions, restoredStep)
		h.Cursor = newID
		if err := SaveBookHistory(b, h); err != nil {
			return "", "", err
		}
		PruneUnreferencedBlobs(b, h)
		return restoredStep.SelectedSceneID, targetRev.Label, nil
	}

	h.Cursor = targetRev.ID
	if err := SaveBookHistory(b, h); err != nil {
		return "", "", err
	}
	return targetRev.SelectedSceneID, targetRev.Label, nil
}
