//go:build !(js && wasm)

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
	HistoryCoalesceWindowSec   = 180
	HistoryLiveWindowSec       = 180
	HistoryCompactionBucketSec = 300
	HistoryLargeDeltaChars     = 300
	HistoryLargeDeltaLines     = 6
)

var validHexHashRegexp = regexp.MustCompile(`^[a-f0-9]{64}$`)

// HistoryBranch represents a named Git-style draft branch in books/<id>/.history/history.txt.
type HistoryBranch struct {
	Name          string
	HeadRevID     string
	BaseRevID     string
	CreatedAtUnix int64
}

// BranchComparison summarizes structural, page/runtime, cast, and location deltas between two branches.
type BranchComparison struct {
	BranchA             string
	BranchB             string
	ScenesA             int
	ScenesB             int
	SceneDelta          int
	AddedScenes         []string
	RemovedScenes       []string
	ModifiedScenes      []string
	EighthsA            int
	EighthsB            int
	EighthsDelta        int
	PagesDeltaFormatted string
	MinutesA            int
	MinutesB            int
	MinutesDelta        int
	LocationsA          int
	LocationsB          int
	LocationDelta       int
	CharactersA         int
	CharactersB         int
	CharacterDelta      int
	SummaryLabel        string
}

// HistoryRevision represents a single snapshot in books/<id>/.history/history.txt.
type HistoryRevision struct {
	ID              string
	Branch          string
	ParentID        string
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
	ActiveBranch string
	Cursor       string
	NextSeq      int
	Branches     []HistoryBranch
	Revisions    []HistoryRevision
}

func bookHistoryDir(bookRoot string) string {
	if IsTempDir(bookRoot) && !IsInsideRepoDir(bookRoot) {
		return filepath.Join(bookRoot, ".history")
	}
	base := SanitizeBookID(filepath.Base(bookRoot))
	if base == "" {
		base = "book"
	}
	return filepath.Join(DefaultAppDataDir(), "history", base+"-"+shortPathHash(bookRoot))
}

func bookHistoryIndexPath(bookRoot string) string {
	return filepath.Join(bookHistoryDir(bookRoot), "history.txt")
}

// BookHistoryIndexPath returns the resolved path to history.txt for the given book root directory.
func BookHistoryIndexPath(bookRoot string) string {
	return bookHistoryIndexPath(bookRoot)
}

func bookHistoryBlobsDir(bookRoot string) string {
	return filepath.Join(bookHistoryDir(bookRoot), "blobs")
}

func bookHistoryBlobPath(bookRoot, hash string) string {
	return filepath.Join(bookHistoryBlobsDir(bookRoot), strings.ToLower(strings.TrimSpace(hash))+".gz")
}

// NormalizedActiveBranch returns the sanitized active branch name, defaulting to "main".
func (h *BookHistory) NormalizedActiveBranch() string {
	if h == nil {
		return "main"
	}
	br := SanitizeBookID(strings.TrimSpace(h.ActiveBranch))
	if br == "" || br == "book" {
		return "main"
	}
	return br
}

func normalizeRevisionBranch(raw string) string {
	br := SanitizeBookID(strings.TrimSpace(raw))
	if br == "" || br == "book" {
		return "main"
	}
	return br
}

// ActiveBranchRevisions returns the slice indices in h.Revisions belonging to the active branch.
func (h *BookHistory) ActiveBranchRevisions() []int {
	if h == nil || len(h.Revisions) == 0 {
		return nil
	}
	activeBr := h.NormalizedActiveBranch()
	var idxs []int
	for i := range h.Revisions {
		if normalizeRevisionBranch(h.Revisions[i].Branch) == activeBr {
			idxs = append(idxs, i)
		}
	}
	return idxs
}

func (h *BookHistory) syncBranchesMetadata() {
	if h == nil {
		return
	}
	activeBr := h.NormalizedActiveBranch()
	h.ActiveBranch = activeBr

	hasMain := false
	for i := range h.Branches {
		h.Branches[i].Name = normalizeRevisionBranch(h.Branches[i].Name)
		if h.Branches[i].Name == "main" {
			hasMain = true
		}
	}
	if !hasMain && len(h.Revisions) > 0 {
		firstID := h.Revisions[0].ID
		headID := firstID
		for _, rev := range h.Revisions {
			if normalizeRevisionBranch(rev.Branch) == "main" {
				headID = rev.ID
			}
		}
		h.Branches = append([]HistoryBranch{{
			Name:          "main",
			HeadRevID:     headID,
			BaseRevID:     firstID,
			CreatedAtUnix: h.Revisions[0].TimeUnix,
		}}, h.Branches...)
	}
	if h.Cursor != "" {
		foundBranch := false
		for i := range h.Branches {
			if h.Branches[i].Name == activeBr {
				h.Branches[i].HeadRevID = h.Cursor
				foundBranch = true
				break
			}
		}
		if !foundBranch && len(h.Revisions) > 0 {
			h.Branches = append(h.Branches, HistoryBranch{
				Name:          activeBr,
				HeadRevID:     h.Cursor,
				BaseRevID:     h.Cursor,
				CreatedAtUnix: time.Now().Unix(),
			})
		}
	}
}

// CursorIndex returns the index of the current cursor revision in h.Revisions, or -1 if empty.
func (h *BookHistory) CursorIndex() int {
	if h == nil || len(h.Revisions) == 0 {
		return -1
	}
	for i := range h.Revisions {
		if h.Revisions[i].ID == h.Cursor {
			return i
		}
	}
	branchIdxs := h.ActiveBranchRevisions()
	if len(branchIdxs) > 0 {
		return branchIdxs[len(branchIdxs)-1]
	}
	return len(h.Revisions) - 1
}

// LastNamedIndex returns the index of the latest Named Version on the active branch, or -1 if none.
func (h *BookHistory) LastNamedIndex() int {
	if h == nil {
		return -1
	}
	branchIdxs := h.ActiveBranchRevisions()
	for j := len(branchIdxs) - 1; j >= 0; j-- {
		idx := branchIdxs[j]
		if h.Revisions[idx].Named {
			return idx
		}
	}
	return -1
}

// CanUndo returns true if the cursor can step back on the active branch without crossing the latest Named Version floor.
func (h *BookHistory) CanUndo() bool {
	if h == nil || len(h.Revisions) == 0 {
		return false
	}
	branchIdxs := h.ActiveBranchRevisions()
	if len(branchIdxs) <= 1 {
		return false
	}
	curPos := len(branchIdxs) - 1
	floorPos := 0
	for j, idx := range branchIdxs {
		if h.Revisions[idx].ID == h.Cursor {
			curPos = j
		}
		if h.Revisions[idx].Named {
			floorPos = j
		}
	}
	return curPos > floorPos
}

// CanRedo returns true if there is a newer revision after the current cursor on the active branch.
func (h *BookHistory) CanRedo() bool {
	if h == nil || len(h.Revisions) == 0 {
		return false
	}
	branchIdxs := h.ActiveBranchRevisions()
	if len(branchIdxs) <= 1 {
		return false
	}
	curPos := -1
	for j, idx := range branchIdxs {
		if h.Revisions[idx].ID == h.Cursor {
			curPos = j
			break
		}
	}
	return curPos >= 0 && curPos < len(branchIdxs)-1
}

// UnnamedEditsCount returns the number of active unnamed edits applied after the latest Named Version up to the cursor on the active branch.
func (h *BookHistory) UnnamedEditsCount() int {
	if h == nil || len(h.Revisions) == 0 {
		return 0
	}
	branchIdxs := h.ActiveBranchRevisions()
	if len(branchIdxs) == 0 {
		return 0
	}
	curPos := len(branchIdxs) - 1
	floorPos := 0
	for j, idx := range branchIdxs {
		if h.Revisions[idx].ID == h.Cursor {
			curPos = j
		}
		if h.Revisions[idx].Named {
			floorPos = j
		}
	}
	if curPos <= floorPos {
		return 0
	}
	count := 0
	for j := floorPos + 1; j <= curPos && j < len(branchIdxs); j++ {
		if !h.Revisions[branchIdxs[j]].Named {
			count++
		}
	}
	return count
}

func parseHistoryBranchLine(val string) (HistoryBranch, bool) {
	arrowIdx := strings.Index(val, "->")
	if arrowIdx <= 0 {
		name := normalizeRevisionBranch(val)
		if name == "" {
			return HistoryBranch{}, false
		}
		return HistoryBranch{Name: name}, true
	}
	name := normalizeRevisionBranch(val[:arrowIdx])
	rest := strings.TrimSpace(val[arrowIdx+2:])
	br := HistoryBranch{Name: name}
	for _, seg := range strings.Split(rest, "|") {
		seg = strings.TrimSpace(seg)
		cIdx := strings.IndexByte(seg, ':')
		if cIdx <= 0 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(seg[:cIdx]))
		v := strings.TrimSpace(seg[cIdx+1:])
		switch k {
		case "head":
			br.HeadRevID = SanitizeSceneID(v)
		case "base":
			br.BaseRevID = SanitizeSceneID(v)
		case "time":
			if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
				br.CreatedAtUnix = ts
			}
		}
	}
	return br, br.Name != ""
}

// ParseHistoryFile parses a human-readable key: value history.txt file.
func ParseHistoryFile(content string) *BookHistory {
	h := &BookHistory{
		ActiveBranch: "main",
		NextSeq:      1,
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")

	var current *HistoryRevision
	flushCurrent := func() {
		if current != nil && current.ID != "" {
			if current.Branch == "" {
				current.Branch = "main"
			}
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
				ID:     revID,
				Branch: "main",
				Files:  make(map[string]string),
			}
			continue
		}

		if current == nil {
			switch key {
			case "active_branch":
				h.ActiveBranch = normalizeRevisionBranch(val)
			case "cursor":
				h.Cursor = SanitizeSceneID(val)
			case "next_seq":
				if seq, err := strconv.Atoi(val); err == nil && seq > h.NextSeq {
					h.NextSeq = seq
				}
			case "branch":
				if br, ok := parseHistoryBranchLine(val); ok {
					h.Branches = append(h.Branches, br)
				}
			}
			continue
		}

		switch key {
		case "branch":
			current.Branch = normalizeRevisionBranch(val)
		case "parent":
			current.ParentID = SanitizeSceneID(val)
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
	h.syncBranchesMetadata()
	return h
}

// SerializeHistoryFile serializes BookHistory into the human-readable key: value format.
func SerializeHistoryFile(h *BookHistory) string {
	if h == nil {
		h = &BookHistory{ActiveBranch: "main", NextSeq: 1}
	}
	h.syncBranchesMetadata()
	var sb strings.Builder
	sb.WriteString("# Mojra — Historia edycji książki (history.txt)\n")
	sb.WriteString(fmt.Sprintf("active_branch: %s\n", h.NormalizedActiveBranch()))
	sb.WriteString(fmt.Sprintf("cursor: %s\n", SanitizeSceneID(h.Cursor)))
	nextSeq := h.NextSeq
	if nextSeq < len(h.Revisions)+1 {
		nextSeq = len(h.Revisions) + 1
	}
	sb.WriteString(fmt.Sprintf("next_seq: %d\n", nextSeq))
	for _, br := range h.Branches {
		sb.WriteString(fmt.Sprintf("branch: %s -> head:%s | base:%s | time:%d\n",
			normalizeRevisionBranch(br.Name),
			SanitizeSceneID(br.HeadRevID),
			SanitizeSceneID(br.BaseRevID),
			br.CreatedAtUnix,
		))
	}

	for _, rev := range h.Revisions {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("revision: %s\n", SanitizeSceneID(rev.ID)))
		sb.WriteString(fmt.Sprintf("branch: %s\n", normalizeRevisionBranch(rev.Branch)))
		if rev.ParentID != "" {
			sb.WriteString(fmt.Sprintf("parent: %s\n", SanitizeSceneID(rev.ParentID)))
		}
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
	h := ParseHistoryFile(string(raw))
	branchIdxs := h.ActiveBranchRevisions()
	if curIdx := h.CursorIndex(); curIdx >= 0 && len(branchIdxs) > 0 && curIdx == branchIdxs[len(branchIdxs)-1] {
		if compactHistoryRevisions(b.RootPath, h, time.Now().Unix(), nil) {
			_ = SaveBookHistory(b, h)
			PruneUnreferencedBlobs(b, h)
		}
	}
	return h, nil
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

func isTrackedBookHistoryFile(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if strings.HasSuffix(lower, PlaytxtExt) && !strings.HasPrefix(lower, ".") {
		return true
	}
	return lower == EditorStateFileName
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
		if e.IsDir() || !isTrackedBookHistoryFile(e.Name()) {
			continue
		}
		name := e.Name()
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
	if _, hasData := filesMap[BookDataFileName]; !hasData {
		hashHex, wErr := writeHistoryBlobWithSet(b.RootPath, []byte(SerializeDataFile(b)), existingBlobs)
		if wErr == nil {
			filesMap[BookDataFileName] = hashHex
		}
	}
	if _, hasEditor := filesMap[EditorStateFileName]; !hasEditor {
		hashHex, wErr := writeHistoryBlobWithSet(b.RootPath, []byte(SerializeEditorFile(b)), existingBlobs)
		if wErr == nil {
			filesMap[EditorStateFileName] = hashHex
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
		if cleanName == "" || cleanName == "." || !isTrackedBookHistoryFile(cleanName) {
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

	// Remove any top-level tracked book files in the book directory that did not exist in this revision.
	if entries, err := os.ReadDir(b.RootPath); err == nil {
		for _, e := range entries {
			if e.IsDir() || !isTrackedBookHistoryFile(e.Name()) {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if !expectedLower[lower] {
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
	now := time.Now().Unix()
	rev := HistoryRevision{
		ID:              "r1",
		Branch:          "main",
		TimeUnix:        now,
		Kind:            "baseline",
		Named:           true,
		Label:           "Stan początkowy",
		SelectedSceneID: selectedSceneID,
		Files:           filesMap,
	}
	h.ActiveBranch = "main"
	h.Cursor = "r1"
	h.NextSeq = 2
	h.Branches = []HistoryBranch{{
		Name:          "main",
		HeadRevID:     "r1",
		BaseRevID:     "r1",
		CreatedAtUnix: now,
	}}
	h.Revisions = []HistoryRevision{rev}
	return SaveBookHistory(b, h)
}
