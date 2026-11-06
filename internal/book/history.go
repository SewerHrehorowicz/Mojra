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
	"unicode/utf8"
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
	sb.WriteString("# StoryTeller — Historia edycji książki (history.txt)\n")
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

func truncateAfterCursorPreservingNamed(h *BookHistory) {
	curIdx := h.CursorIndex()
	if curIdx < 0 || curIdx >= len(h.Revisions)-1 {
		return
	}
	activeBr := h.NormalizedActiveBranch()
	referencedByBranches := make(map[string]bool)
	for _, br := range h.Branches {
		if br.Name != activeBr {
			if br.HeadRevID != "" {
				referencedByBranches[br.HeadRevID] = true
			}
			if br.BaseRevID != "" {
				referencedByBranches[br.BaseRevID] = true
			}
		}
	}
	kept := make([]HistoryRevision, 0, len(h.Revisions))
	for i, rev := range h.Revisions {
		revBr := normalizeRevisionBranch(rev.Branch)
		if revBr != activeBr || i <= curIdx || rev.Named || referencedByBranches[rev.ID] {
			kept = append(kept, rev)
		}
	}
	h.Revisions = kept
}

func isCompactableHistoryKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "scene", "text", "layout", "meta", "edit", "":
		return true
	default:
		return false
	}
}

func isFiveMinBucketCompactableKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "restore", "version", "baseline":
		return false
	default:
		return true
	}
}

func isSceneOrTextKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "scene", "text", "edit", "":
		return true
	default:
		return false
	}
}

func isSameSceneRenameLabel(labelA, labelB string) bool {
	const prefix = "Edycja sceny: "
	if !strings.HasPrefix(labelA, prefix) || !strings.HasPrefix(labelB, prefix) {
		return false
	}
	nameA := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(labelA, prefix)))
	nameB := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(labelB, prefix)))
	if nameA == "" || nameB == "" {
		return false
	}
	return strings.HasPrefix(nameA, nameB) || strings.HasPrefix(nameB, nameA)
}

func canLiveCoalesceRevisions(prev, curr HistoryRevision) bool {
	if !isCompactableHistoryKind(prev.Kind) || !isCompactableHistoryKind(curr.Kind) {
		return false
	}
	if prev.CoalesceKey == "major" || curr.CoalesceKey == "major" {
		return false
	}
	prevKind := strings.ToLower(strings.TrimSpace(prev.Kind))
	currKind := strings.ToLower(strings.TrimSpace(curr.Kind))
	if prevKind == "layout" && currKind == "layout" {
		return true
	}
	if prevKind == "layout" && isSceneOrTextKind(currKind) {
		return true
	}
	if isSceneOrTextKind(prevKind) && isSceneOrTextKind(currKind) {
		if prev.SelectedSceneID != "" && prev.SelectedSceneID == curr.SelectedSceneID {
			return true
		}
		if isSameSceneRenameLabel(prev.Label, curr.Label) {
			return true
		}
	}
	prevGroup := normalizeCoalesceGroup(prev.Kind, prev.CoalesceKey, prev.SelectedSceneID)
	currGroup := normalizeCoalesceGroup(curr.Kind, curr.CoalesceKey, curr.SelectedSceneID)
	return prevGroup != "" && prevGroup == currGroup
}

func normalizeCoalesceGroup(kind, coalesceKey, selectedSceneID string) string {
	cleanKind := strings.ToLower(strings.TrimSpace(kind))
	cleanKey := strings.TrimSpace(coalesceKey)
	if cleanKey == "major" {
		return ""
	}
	if cleanKind == "layout" {
		if strings.HasPrefix(cleanKey, "nodedrag:") || cleanKey == "" {
			return "layout:drag"
		}
		return cleanKey
	}
	if isSceneOrTextKind(cleanKind) {
		for _, prefix := range []string{"autosave:", "textmodal:", "scene:"} {
			if strings.HasPrefix(cleanKey, prefix) {
				target := strings.TrimSpace(strings.TrimPrefix(cleanKey, prefix))
				if target != "" {
					return "scene:" + target
				}
			}
		}
		if strings.TrimSpace(selectedSceneID) != "" {
			return "scene:" + strings.TrimSpace(selectedSceneID)
		}
	}
	return cleanKey
}

func isProseStructuralLine(trimmed string) bool {
	if trimmed == "" || trimmed == "---" || strings.EqualFold(trimmed, ":end") {
		return true
	}
	if strings.Contains(trimmed, "->") && !strings.Contains(trimmed, `\->`) {
		return true
	}
	colonIdx := strings.IndexByte(trimmed, ':')
	if colonIdx > 0 && colonIdx <= 16 {
		key := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
		switch key {
		case "chapter", "scene", "id", "exit", "next", "label", "if", "if?", "else", "endif", "set", "then", "end", "image", "voice",
			"location", "characters", "slugline", "synopsis", "text",
			"title", "author", "description", "narrated", "imageless",
			"start", "cover", "export_dir", "mode", "language", "standard":
			return true
		}
	}
	return false
}

func extractProseLines(content string) []string {
	rawLines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if isProseStructuralLine(trimmed) {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func computeProseDelta(oldStr, newStr string) (int, int) {
	if oldStr == newStr {
		return 0, 0
	}
	oldLines := extractProseLines(oldStr)
	newLines := extractProseLines(newStr)
	if len(oldLines) == 0 && len(newLines) == 0 {
		return 0, 0
	}

	lineCounts := make(map[string]int, len(oldLines))
	for _, l := range oldLines {
		lineCounts[l]++
	}
	var unmatchedNew []string
	for _, l := range newLines {
		if lineCounts[l] > 0 {
			lineCounts[l]--
		} else {
			unmatchedNew = append(unmatchedNew, l)
		}
	}
	var unmatchedOld []string
	for _, l := range oldLines {
		if lineCounts[l] > 0 {
			lineCounts[l]--
			unmatchedOld = append(unmatchedOld, l)
		}
	}

	deltaLines := len(unmatchedOld)
	if len(unmatchedNew) > deltaLines {
		deltaLines = len(unmatchedNew)
	}

	wordCounts := make(map[string]int)
	for _, l := range unmatchedOld {
		for _, w := range strings.Fields(l) {
			wordCounts[w]++
		}
	}
	newWordRunes := 0
	for _, l := range unmatchedNew {
		for _, w := range strings.Fields(l) {
			if wordCounts[w] > 0 {
				wordCounts[w]--
			} else {
				newWordRunes += utf8.RuneCountInString(w) + 1
			}
		}
	}
	oldWordRunes := 0
	for w, cnt := range wordCounts {
		if cnt > 0 {
			oldWordRunes += (utf8.RuneCountInString(w) + 1) * cnt
		}
	}
	deltaRunes := oldWordRunes
	if newWordRunes > deltaRunes {
		deltaRunes = newWordRunes
	}
	return deltaRunes, deltaLines
}

func readHistoryBlobCached(bookRoot, hashHex string, cache map[string][]byte) ([]byte, error) {
	key := strings.ToLower(strings.TrimSpace(hashHex))
	if cache != nil {
		if data, ok := cache[key]; ok {
			return data, nil
		}
	}
	data, err := readHistoryBlob(bookRoot, key)
	if err == nil && cache != nil {
		cache[key] = data
	}
	return data, err
}

func isMajorHistoryDeltaWithCache(bookRoot string, oldFiles, newFiles map[string]string, kind string, cache map[string][]byte) bool {
	cleanKind := strings.ToLower(strings.TrimSpace(kind))
	if cleanKind == "layout" || cleanKind == "meta" {
		return false
	}
	if cleanKind == "restore" || cleanKind == "version" || cleanKind == "baseline" {
		return true
	}
	totalRunes := 0
	totalLines := 0
	for fileName, newHash := range newFiles {
		if strings.EqualFold(fileName, EditorStateFileName) || strings.EqualFold(fileName, "editor.cfg") || strings.EqualFold(fileName, BookDataFileName) {
			continue
		}
		oldHash, exists := oldFiles[fileName]
		if !exists {
			newBytes, err := readHistoryBlobCached(bookRoot, newHash, cache)
			if err != nil {
				continue
			}
			dRunes, dLines := computeProseDelta("", string(newBytes))
			totalRunes += dRunes
			totalLines += dLines
			if totalRunes >= HistoryLargeDeltaChars || totalLines > HistoryLargeDeltaLines {
				return true
			}
			continue
		}
		if oldHash == newHash {
			continue
		}
		oldBytes, err1 := readHistoryBlobCached(bookRoot, oldHash, cache)
		newBytes, err2 := readHistoryBlobCached(bookRoot, newHash, cache)
		if err1 != nil || err2 != nil {
			continue
		}
		dRunes, dLines := computeProseDelta(string(oldBytes), string(newBytes))
		totalRunes += dRunes
		totalLines += dLines
		if totalRunes >= HistoryLargeDeltaChars || totalLines > HistoryLargeDeltaLines {
			return true
		}
	}
	for fileName, oldHash := range oldFiles {
		if strings.EqualFold(fileName, EditorStateFileName) || strings.EqualFold(fileName, "editor.cfg") || strings.EqualFold(fileName, BookDataFileName) {
			continue
		}
		if _, exists := newFiles[fileName]; !exists {
			oldBytes, err := readHistoryBlobCached(bookRoot, oldHash, cache)
			if err != nil {
				continue
			}
			dRunes, dLines := computeProseDelta(string(oldBytes), "")
			totalRunes += dRunes
			totalLines += dLines
			if totalRunes >= HistoryLargeDeltaChars || totalLines > HistoryLargeDeltaLines {
				return true
			}
		}
	}
	return false
}

// compactHistoryRevisions merges redundant unnamed small edits on the active branch:
// 1. Live window (0-3 minutes): same-scene edits (including title renames) and layout moves coalesce immediately.
// 2. Older than 3 minutes: all unnamed revisions in the same 5-minute time bucket compact into one entry per 5-minute block.
// Major text pastes (>= 300 prose chars or > 6 prose lines) and named versions are always preserved.
func compactHistoryRevisions(bookRoot string, h *BookHistory, now int64, cache map[string][]byte) bool {
	if h == nil || len(h.Revisions) < 2 || strings.TrimSpace(bookRoot) == "" {
		return false
	}
	if cache == nil {
		cache = make(map[string][]byte)
	}
	activeBr := h.NormalizedActiveBranch()
	curIdx := h.CursorIndex()
	if curIdx < 0 {
		return false
	}
	protectedIDs := make(map[string]bool)
	for _, br := range h.Branches {
		if br.Name != activeBr {
			if br.HeadRevID != "" {
				protectedIDs[br.HeadRevID] = true
			}
			if br.BaseRevID != "" {
				protectedIDs[br.BaseRevID] = true
			}
		} else if br.BaseRevID != "" {
			protectedIDs[br.BaseRevID] = true
		}
	}

	compacted := false
	for {
		mergedInPass := false
		branchIdxs := h.ActiveBranchRevisions()
		curBranchPos := -1
		for pos, idx := range branchIdxs {
			if idx == h.CursorIndex() {
				curBranchPos = pos
				break
			}
		}
		if curBranchPos < 1 {
			break
		}

		for pos := 1; pos <= curBranchPos; pos++ {
			prevIdx := branchIdxs[pos-1]
			currIdx := branchIdxs[pos]
			prev := &h.Revisions[prevIdx]
			curr := &h.Revisions[currIdx]

			if prev.Named || curr.Named || protectedIDs[prev.ID] || prev.ID == h.Cursor {
				continue
			}
			if !isFiveMinBucketCompactableKind(prev.Kind) || !isFiveMinBucketCompactableKind(curr.Kind) {
				continue
			}

			dt := curr.TimeUnix - prev.TimeUnix
			if dt < 0 {
				dt = 0
			}

			liveMerge := dt <= HistoryCoalesceWindowSec && canLiveCoalesceRevisions(*prev, *curr)
			bothOlderInSameFiveMinBucket := (now-prev.TimeUnix) > HistoryLiveWindowSec &&
				(now-curr.TimeUnix) > HistoryLiveWindowSec &&
				(prev.TimeUnix/HistoryCompactionBucketSec) == (curr.TimeUnix/HistoryCompactionBucketSec) &&
				prev.CoalesceKey != "major" && curr.CoalesceKey != "major"

			if !liveMerge && !bothOlderInSameFiveMinBucket {
				continue
			}

			if isMajorHistoryDeltaWithCache(bookRoot, prev.Files, curr.Files, curr.Kind, cache) {
				continue
			}
			if pos >= 2 {
				baseFiles := h.Revisions[branchIdxs[pos-2]].Files
				if isMajorHistoryDeltaWithCache(bookRoot, baseFiles, prev.Files, prev.Kind, cache) {
					continue
				}
			}

			removedID := prev.ID
			curr.ParentID = prev.ParentID
			if curr.Kind == "layout" && prev.Kind != "layout" {
				curr.Kind = prev.Kind
				curr.Label = prev.Label
				if prev.SelectedSceneID != "" {
					curr.SelectedSceneID = prev.SelectedSceneID
				}
			} else if curr.Kind == "layout" && prev.Kind == "layout" && prev.Label != curr.Label {
				curr.Label = "Przesunięto węzły"
				curr.CoalesceKey = "nodedrag:batch"
			}
			for k := range h.Revisions {
				if h.Revisions[k].ParentID == removedID && k != currIdx {
					h.Revisions[k].ParentID = curr.ID
				}
			}
			h.Revisions = append(h.Revisions[:prevIdx], h.Revisions[prevIdx+1:]...)
			compacted = true
			mergedInPass = true
			break
		}
		if !mergedInPass {
			break
		}
	}
	if compacted {
		h.syncBranchesMetadata()
	}
	return compacted
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
	now := time.Now().Unix()
	blobCache := make(map[string][]byte)
	compactHistoryRevisions(b.RootPath, h, now, blobCache)

	curIdx = h.CursorIndex()
	activeBr := h.NormalizedActiveBranch()

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

	branchIdxs := h.ActiveBranchRevisions()
	if curIdx >= 0 && len(branchIdxs) > 0 && curIdx == branchIdxs[len(branchIdxs)-1] {
		curRev := &h.Revisions[curIdx]
		stepMajor := isMajorHistoryDeltaWithCache(b.RootPath, curRev.Files, filesMap, cleanKind, blobCache)
		if stepMajor {
			cleanCoalesce = "major"
		} else if !curRev.Named {
			candRev := HistoryRevision{
				Kind:            cleanKind,
				Label:           cleanLabel,
				SelectedSceneID: selectedSceneID,
				CoalesceKey:     cleanCoalesce,
			}
			prevWasMajor := false
			if len(branchIdxs) >= 2 {
				baseFiles := h.Revisions[branchIdxs[len(branchIdxs)-2]].Files
				if isMajorHistoryDeltaWithCache(b.RootPath, baseFiles, curRev.Files, curRev.Kind, blobCache) {
					prevWasMajor = true
				}
			}
			if !prevWasMajor && canLiveCoalesceRevisions(*curRev, candRev) && (now-curRev.TimeUnix) <= HistoryCoalesceWindowSec {
				if cleanKind == "layout" && curRev.Kind == "layout" && curRev.Label != cleanLabel {
					cleanLabel = "Przesunięto węzły"
					cleanCoalesce = "nodedrag:batch"
				} else if cleanKind == "layout" && curRev.Kind != "layout" {
					cleanKind = curRev.Kind
					cleanLabel = curRev.Label
					if curRev.SelectedSceneID != "" {
						selectedSceneID = curRev.SelectedSceneID
					}
					cleanCoalesce = curRev.CoalesceKey
				}
				curRev.TimeUnix = now
				curRev.Label = cleanLabel
				curRev.Kind = cleanKind
				curRev.SelectedSceneID = selectedSceneID
				curRev.CoalesceKey = cleanCoalesce
				curRev.Files = filesMap
				h.syncBranchesMetadata()
				if err := SaveBookHistory(b, h); err != nil {
					return err
				}
				PruneUnreferencedBlobs(b, h)
				return nil
			}
		}
	}

	if h.NextSeq < 1 {
		h.NextSeq = len(h.Revisions) + 1
	}
	revID := fmt.Sprintf("r%d", h.NextSeq)
	h.NextSeq++

	parentID := ""
	if curIdx >= 0 && curIdx < len(h.Revisions) {
		parentID = h.Revisions[curIdx].ID
	}

	newRev := HistoryRevision{
		ID:              revID,
		Branch:          activeBr,
		ParentID:        parentID,
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
	h.syncBranchesMetadata()

	// Enforce rolling cap of MaxUnnamedHistoryRevisions after the latest Named Version on the active branch.
	for {
		lastNamedIdx := h.LastNamedIndex()
		unnamedCount := 0
		oldestUnnamedIdx := -1
		for i := lastNamedIdx + 1; i < len(h.Revisions); i++ {
			if normalizeRevisionBranch(h.Revisions[i].Branch) == activeBr && !h.Revisions[i].Named {
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

// SaveNamedBookVersion collapses all unnamed edits on the active branch into a permanent Named Version and prunes unreferenced blobs.
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

	activeBr := h.NormalizedActiveBranch()
	cleanName := SanitizePlainText(versionName)
	if cleanName == "" {
		cleanName = fmt.Sprintf("Wersja %d", len(h.Revisions)+1)
	}
	parentID := ""
	if curIdx := h.CursorIndex(); curIdx >= 0 {
		parentID = h.Revisions[curIdx].ID
		if selectedSceneID == "" {
			selectedSceneID = h.Revisions[curIdx].SelectedSceneID
		}
	}
	if selectedSceneID == "" {
		selectedSceneID = b.StartSceneID
	}

	referencedByOtherBranches := make(map[string]bool)
	for _, br := range h.Branches {
		if br.Name != activeBr {
			if br.HeadRevID != "" {
				referencedByOtherBranches[br.HeadRevID] = true
			}
			if br.BaseRevID != "" {
				referencedByOtherBranches[br.BaseRevID] = true
			}
		}
	}

	var keptRevs []HistoryRevision
	for _, rev := range h.Revisions {
		revBr := normalizeRevisionBranch(rev.Branch)
		if revBr != activeBr || rev.Named || referencedByOtherBranches[rev.ID] {
			keptRevs = append(keptRevs, rev)
		}
	}

	if h.NextSeq < 1 {
		h.NextSeq = len(keptRevs) + 1
	}
	revID := fmt.Sprintf("r%d", h.NextSeq)
	h.NextSeq++

	newVer := HistoryRevision{
		ID:              revID,
		Branch:          activeBr,
		ParentID:        parentID,
		TimeUnix:        time.Now().Unix(),
		Kind:            "version",
		Named:           true,
		Label:           cleanName,
		SelectedSceneID: selectedSceneID,
		Files:           filesMap,
	}
	keptRevs = append(keptRevs, newVer)
	h.Revisions = keptRevs
	h.Cursor = newVer.ID
	h.syncBranchesMetadata()

	if err := SaveBookHistory(b, h); err != nil {
		return nil, err
	}
	PruneUnreferencedBlobs(b, h)
	return &newVer, nil
}

// UndoBookHistory steps backward by one unnamed edit on the active branch (stopping at the latest Named Version floor) and restores files to disk.
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
	branchIdxs := h.ActiveBranchRevisions()
	curPos := len(branchIdxs) - 1
	for j, idx := range branchIdxs {
		if h.Revisions[idx].ID == h.Cursor {
			curPos = j
			break
		}
	}
	undoneRev := h.Revisions[branchIdxs[curPos]]
	targetRev := h.Revisions[branchIdxs[curPos-1]]

	if err := restoreBookTextFiles(b, targetRev); err != nil {
		return "", "", err
	}
	h.Cursor = targetRev.ID
	h.syncBranchesMetadata()
	if err := SaveBookHistory(b, h); err != nil {
		return "", "", err
	}
	return targetRev.SelectedSceneID, undoneRev.Label, nil
}

// RedoBookHistory steps forward by one unnamed edit on the active branch and restores files to disk.
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
	branchIdxs := h.ActiveBranchRevisions()
	curPos := 0
	for j, idx := range branchIdxs {
		if h.Revisions[idx].ID == h.Cursor {
			curPos = j
			break
		}
	}
	targetRev := h.Revisions[branchIdxs[curPos+1]]

	if err := restoreBookTextFiles(b, targetRev); err != nil {
		return "", "", err
	}
	h.Cursor = targetRev.ID
	h.syncBranchesMetadata()
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
	activeBr := h.NormalizedActiveBranch()

	if err := restoreBookTextFiles(b, targetRev); err != nil {
		return "", "", err
	}

	if (targetRev.Named && targetIdx < lastNamedIdx) || normalizeRevisionBranch(targetRev.Branch) != activeBr {
		truncateAfterCursorPreservingNamed(h)
		if h.NextSeq < 1 {
			h.NextSeq = len(h.Revisions) + 1
		}
		newID := fmt.Sprintf("r%d", h.NextSeq)
		h.NextSeq++
		restoredStep := HistoryRevision{
			ID:              newID,
			Branch:          activeBr,
			ParentID:        h.Cursor,
			TimeUnix:        time.Now().Unix(),
			Kind:            "restore",
			Named:           false,
			Label:           "Przywrócono wersję: " + targetRev.Label,
			SelectedSceneID: targetRev.SelectedSceneID,
			Files:           cloneFileHashMap(targetRev.Files),
		}
		h.Revisions = append(h.Revisions, restoredStep)
		h.Cursor = newID
		h.syncBranchesMetadata()
		if err := SaveBookHistory(b, h); err != nil {
			return "", "", err
		}
		PruneUnreferencedBlobs(b, h)
		return restoredStep.SelectedSceneID, targetRev.Label, nil
	}

	h.Cursor = targetRev.ID
	h.syncBranchesMetadata()
	if err := SaveBookHistory(b, h); err != nil {
		return "", "", err
	}
	return targetRev.SelectedSceneID, targetRev.Label, nil
}

// CopyHistoryRevisionToBranch applies the snapshot of revID onto the top of targetBranch
// as a new Named Version and switches ActiveBranch to targetBranch.
// If targetBranch is empty or already the active branch and revID belongs to it, it delegates to RestoreBookHistoryRevision.
func CopyHistoryRevisionToBranch(b *Book, revID, targetBranch string) (selectedSceneID string, label string, err error) {
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
	activeBr := h.NormalizedActiveBranch()
	cleanTarget := activeBr
	if strings.TrimSpace(targetBranch) != "" {
		cleanTarget = normalizeRevisionBranch(targetBranch)
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
	srcRev := h.Revisions[targetIdx]

	if cleanTarget == activeBr && normalizeRevisionBranch(srcRev.Branch) == activeBr {
		return RestoreBookHistoryRevision(b, cleanID)
	}

	if cleanTarget != activeBr {
		_ = RecordBookHistoryStep(b, "Auto-zapis przed nałożeniem wersji", "autosave", "", b.StartSceneID)
		h, err = LoadBookHistory(b)
		if err != nil {
			return "", "", err
		}
	}

	branchIdx := -1
	for i := range h.Branches {
		if strings.EqualFold(h.Branches[i].Name, cleanTarget) {
			branchIdx = i
			break
		}
	}
	if branchIdx < 0 {
		return "", "", fmt.Errorf("nie znaleziono gałęzi „%s”", cleanTarget)
	}

	parentID := h.Branches[branchIdx].HeadRevID
	if parentID == "" {
		for i := len(h.Revisions) - 1; i >= 0; i-- {
			if normalizeRevisionBranch(h.Revisions[i].Branch) == cleanTarget {
				parentID = h.Revisions[i].ID
				break
			}
		}
	}

	if err := restoreBookTextFiles(b, srcRev); err != nil {
		return "", "", err
	}

	if h.NextSeq < 1 {
		h.NextSeq = len(h.Revisions) + 1
	}
	newID := fmt.Sprintf("r%d", h.NextSeq)
	h.NextSeq++

	cleanLabel := SanitizePlainText(srcRev.Label)
	if cleanLabel == "" {
		cleanLabel = srcRev.ID
	}

	copiedRev := HistoryRevision{
		ID:              newID,
		Branch:          cleanTarget,
		ParentID:        parentID,
		TimeUnix:        time.Now().Unix(),
		Kind:            "copy",
		Named:           true,
		Label:           cleanLabel,
		SelectedSceneID: srcRev.SelectedSceneID,
		Files:           cloneFileHashMap(srcRev.Files),
	}
	h.Revisions = append(h.Revisions, copiedRev)
	h.ActiveBranch = cleanTarget
	h.Cursor = newID
	h.Branches[branchIdx].HeadRevID = newID
	h.syncBranchesMetadata()

	if err := SaveBookHistory(b, h); err != nil {
		return "", "", err
	}
	PruneUnreferencedBlobs(b, h)
	_ = reloadBookInPlace(b)
	return copiedRev.SelectedSceneID, copiedRev.Label, nil
}

// BuildHistoryHoverSummaries computes concise hover hints ("<branch> • <N> scen • <pages/delta>")
// for all revisions and branches in h, deduplicating snapshot parsing by file-hash signature.
func BuildHistoryHoverSummaries(b *Book, h *BookHistory) (revHints map[string]string, branchHints map[string]string) {
	revHints = make(map[string]string)
	branchHints = make(map[string]string)
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" || h == nil || len(h.Revisions) == 0 {
		return revHints, branchHints
	}

	type snapMetrics struct {
		scenes  int
		eighths int
	}
	metricsBySig := make(map[string]snapMetrics)
	computeMetrics := func(rev HistoryRevision) snapMetrics {
		if len(rev.Files) == 0 {
			return snapMetrics{scenes: len(b.Scenes), eighths: len(b.Scenes)}
		}
		keys := make([]string, 0, len(rev.Files))
		for k := range rev.Files {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var sig strings.Builder
		for _, k := range keys {
			sig.WriteString(k)
			sig.WriteByte('=')
			sig.WriteString(rev.Files[k])
			sig.WriteByte(';')
		}
		sigStr := sig.String()
		if m, ok := metricsBySig[sigStr]; ok {
			return m
		}
		snapBook, err := loadBookFromHistoryRevision(b.RootPath, rev)
		if err != nil || snapBook == nil {
			m := snapMetrics{scenes: len(b.Scenes), eighths: len(b.Scenes)}
			metricsBySig[sigStr] = m
			return m
		}
		bd := AnalyzeScreenplay(snapBook)
		m := snapMetrics{
			scenes:  len(snapBook.Scenes),
			eighths: bd.TotalEighths,
		}
		metricsBySig[sigStr] = m
		return m
	}

	baseMetrics := computeMetrics(h.Revisions[0])
	if curIdx := h.CursorIndex(); curIdx >= 0 && curIdx < len(h.Revisions) {
		baseMetrics = computeMetrics(h.Revisions[curIdx])
	}

	for _, rev := range h.Revisions {
		brName := normalizeRevisionBranch(rev.Branch)
		m := computeMetrics(rev)
		deltaEighths := m.eighths - baseMetrics.eighths
		var pagesPart string
		if deltaEighths > 0 {
			pagesPart = "+" + FormatEighths(deltaEighths)
		} else if deltaEighths < 0 {
			pagesPart = "-" + FormatEighths(-deltaEighths)
		} else {
			pagesPart = FormatEighths(m.eighths)
		}
		hint := fmt.Sprintf("%s • %d scen • %s", brName, m.scenes, pagesPart)
		revHints[rev.ID] = hint
		branchHints[brName] = hint
	}
	for _, br := range h.Branches {
		brName := normalizeRevisionBranch(br.Name)
		if br.HeadRevID != "" {
			if hint, ok := revHints[br.HeadRevID]; ok {
				branchHints[brName] = hint
				continue
			}
		}
		if _, ok := branchHints[brName]; !ok {
			branchHints[brName] = fmt.Sprintf("%s • %d scen", brName, len(b.Scenes))
		}
	}
	return revHints, branchHints
}

func reloadBookInPlace(b *Book) error {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return nil
	}
	reloaded, err := LoadBookFromDir(b.RootPath)
	if err != nil {
		return err
	}
	reloaded.ID = b.ID
	reloaded.IsExternal = b.IsExternal
	reloaded.SourceURL = b.SourceURL
	*b = *reloaded
	return nil
}

// CreateBookBranch forks a new Git-style draft branch from the current state (or specified revision ID) and switches ActiveBranch to it.
func CreateBookBranch(b *Book, branchName, fromRevID string) (*HistoryBranch, error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return nil, fmt.Errorf("brak rozpakowanej książki na dysku")
	}
	if err := EnsureBookHistoryBaseline(b, b.StartSceneID); err != nil {
		return nil, err
	}
	rawTrimmed := strings.TrimSpace(branchName)
	cleanBranch := ""
	if rawTrimmed != "" {
		cleanBranch = SanitizeBookID(rawTrimmed)
	}
	// Auto-save any pending disk modifications on the current branch before forking
	_ = RecordBookHistoryStep(b, "Auto-zapis przed utworzeniem gałęzi", "autosave", "", b.StartSceneID)

	h, err := LoadBookHistory(b)
	if err != nil {
		return nil, err
	}
	if cleanBranch == "" || cleanBranch == "book" {
		for seq := len(h.Branches) + 1; ; seq++ {
			cand := fmt.Sprintf("wariant-%d", seq)
			exists := false
			for _, br := range h.Branches {
				if strings.EqualFold(br.Name, cand) {
					exists = true
					break
				}
			}
			if !exists {
				cleanBranch = cand
				break
			}
		}
	}
	for _, br := range h.Branches {
		if strings.EqualFold(br.Name, cleanBranch) {
			return nil, fmt.Errorf("gałąź „%s” już istnieje", cleanBranch)
		}
	}

	baseIdx := h.CursorIndex()
	if trimmedFrom := SanitizeSceneID(fromRevID); trimmedFrom != "" {
		for i := range h.Revisions {
			if h.Revisions[i].ID == trimmedFrom {
				baseIdx = i
				break
			}
		}
	}
	if baseIdx < 0 || baseIdx >= len(h.Revisions) {
		return nil, fmt.Errorf("nie znaleziono rewizji bazowej do rozgałęzienia")
	}
	baseRev := h.Revisions[baseIdx]

	if h.NextSeq < 1 {
		h.NextSeq = len(h.Revisions) + 1
	}
	newRevID := fmt.Sprintf("r%d", h.NextSeq)
	h.NextSeq++
	now := time.Now().Unix()

	branchRev := HistoryRevision{
		ID:              newRevID,
		Branch:          cleanBranch,
		ParentID:        baseRev.ID,
		TimeUnix:        now,
		Kind:            "branch",
		Named:           true,
		Label:           fmt.Sprintf("Utworzono gałąź „%s” (z %s)", cleanBranch, baseRev.ID),
		SelectedSceneID: baseRev.SelectedSceneID,
		Files:           cloneFileHashMap(baseRev.Files),
	}
	newBr := HistoryBranch{
		Name:          cleanBranch,
		HeadRevID:     newRevID,
		BaseRevID:     baseRev.ID,
		CreatedAtUnix: now,
	}

	if err := restoreBookTextFiles(b, branchRev); err != nil {
		return nil, err
	}

	h.Revisions = append(h.Revisions, branchRev)
	h.Branches = append(h.Branches, newBr)
	h.ActiveBranch = cleanBranch
	h.Cursor = newRevID
	h.syncBranchesMetadata()

	if err := SaveBookHistory(b, h); err != nil {
		return nil, err
	}
	_ = reloadBookInPlace(b)
	return &newBr, nil
}

// SwitchBookBranch auto-saves current unnamed edits on the current branch, restores the target branch head's .txt blobs, and reloads the book.
func SwitchBookBranch(b *Book, branchName string) (selectedSceneID string, err error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return "", fmt.Errorf("brak rozpakowanej książki na dysku")
	}
	cleanBranch := normalizeRevisionBranch(branchName)
	if err := EnsureBookHistoryBaseline(b, b.StartSceneID); err != nil {
		return "", err
	}
	// Auto-save any unrecorded disk changes on the current branch before switching
	_ = RecordBookHistoryStep(b, "Auto-zapis przed zmianą gałęzi", "autosave", "", b.StartSceneID)

	h, err := LoadBookHistory(b)
	if err != nil {
		return "", err
	}
	if h.NormalizedActiveBranch() == cleanBranch {
		curIdx := h.CursorIndex()
		if curIdx >= 0 {
			return h.Revisions[curIdx].SelectedSceneID, nil
		}
		return b.StartSceneID, nil
	}

	var targetBranch *HistoryBranch
	for i := range h.Branches {
		if strings.EqualFold(h.Branches[i].Name, cleanBranch) {
			targetBranch = &h.Branches[i]
			break
		}
	}
	if targetBranch == nil {
		return "", fmt.Errorf("nie znaleziono gałęzi „%s”", cleanBranch)
	}

	targetIdx := -1
	if targetBranch.HeadRevID != "" {
		for i := range h.Revisions {
			if h.Revisions[i].ID == targetBranch.HeadRevID {
				targetIdx = i
				break
			}
		}
	}
	if targetIdx < 0 {
		for i := len(h.Revisions) - 1; i >= 0; i-- {
			if normalizeRevisionBranch(h.Revisions[i].Branch) == cleanBranch {
				targetIdx = i
				break
			}
		}
	}
	if targetIdx < 0 {
		return "", fmt.Errorf("gałąź „%s” nie posiada żadnej migawki", cleanBranch)
	}

	targetRev := h.Revisions[targetIdx]
	if err := restoreBookTextFiles(b, targetRev); err != nil {
		return "", err
	}

	h.ActiveBranch = cleanBranch
	h.Cursor = targetRev.ID
	targetBranch.HeadRevID = targetRev.ID
	h.syncBranchesMetadata()

	if err := SaveBookHistory(b, h); err != nil {
		return "", err
	}
	_ = reloadBookInPlace(b)
	return targetRev.SelectedSceneID, nil
}

// DeleteBookBranch deletes a non-active branch ("main" is protected) and prunes unreferenced .gz blobs.
func DeleteBookBranch(b *Book, branchName string) error {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return fmt.Errorf("brak rozpakowanej książki na dysku")
	}
	cleanBranch := normalizeRevisionBranch(branchName)
	if cleanBranch == "main" {
		return fmt.Errorf("nie można usunąć głównej gałęzi „main”")
	}
	h, err := LoadBookHistory(b)
	if err != nil {
		return err
	}
	if h.NormalizedActiveBranch() == cleanBranch {
		return fmt.Errorf("nie można usunąć aktualnie aktywnej gałęzi „%s” — najpierw przełącz na inną gałąź", cleanBranch)
	}

	found := false
	var keptBranches []HistoryBranch
	referencedBases := make(map[string]bool)
	for _, br := range h.Branches {
		if strings.EqualFold(br.Name, cleanBranch) {
			found = true
			continue
		}
		keptBranches = append(keptBranches, br)
		if br.BaseRevID != "" {
			referencedBases[br.BaseRevID] = true
		}
		if br.HeadRevID != "" {
			referencedBases[br.HeadRevID] = true
		}
	}
	if !found {
		return fmt.Errorf("nie znaleziono gałęzi „%s”", cleanBranch)
	}

	var keptRevs []HistoryRevision
	for _, rev := range h.Revisions {
		if normalizeRevisionBranch(rev.Branch) == cleanBranch && !referencedBases[rev.ID] {
			continue
		}
		keptRevs = append(keptRevs, rev)
	}

	h.Branches = keptBranches
	h.Revisions = keptRevs
	h.syncBranchesMetadata()
	if err := SaveBookHistory(b, h); err != nil {
		return err
	}
	PruneUnreferencedBlobs(b, h)
	return nil
}

func loadBookFromHistoryRevision(bookRoot string, rev HistoryRevision) (*Book, error) {
	b := &Book{
		ID:             SanitizeBookID(filepath.Base(bookRoot)),
		Title:          filepath.Base(bookRoot),
		Author:         "Autor",
		RootPath:       bookRoot,
		Layout:         make(map[string]NodePosition),
		deferProseHTML: true,
	}
	var storyFiles []string
	textMap := make(map[string]string)

	for fileName, hashHex := range rev.Files {
		cleanName := filepath.Base(strings.TrimSpace(fileName))
		lower := strings.ToLower(cleanName)
		if !isTrackedBookHistoryFile(cleanName) {
			continue
		}
		rawBytes, err := readHistoryBlob(bookRoot, hashHex)
		if err != nil {
			return nil, err
		}
		content := string(rawBytes)
		switch {
		case lower == BookDataFileName:
			ParseDataFile(content, b)
		case lower == EditorStateFileName || lower == "editor.cfg":
			ParseEditorFile(content, b)
		default:
			textMap[cleanName] = content
			storyFiles = append(storyFiles, cleanName)
		}
	}
	sortStoryFiles(storyFiles)
	b.TextFiles = storyFiles
	for _, sf := range storyFiles {
		ParseStoryFile(textMap[sf], sf, b)
	}
	FinalizeBook(b)
	return b, nil
}

// LoadBookAtHistoryRevision reconstructs an in-memory *Book snapshot for a specific history revision without modifying disk files.
func LoadBookAtHistoryRevision(baseBook *Book, revID string) (*Book, *HistoryRevision, error) {
	if baseBook == nil || baseBook.IsPacked || strings.TrimSpace(baseBook.RootPath) == "" {
		return nil, nil, fmt.Errorf("brak aktywnej książki")
	}
	cleanID := SanitizeSceneID(revID)
	if cleanID == "" {
		return nil, nil, fmt.Errorf("nieprawidłowy identyfikator rewizji")
	}
	h, err := LoadBookHistory(baseBook)
	if err != nil {
		return nil, nil, err
	}
	targetIdx := -1
	for i := range h.Revisions {
		if h.Revisions[i].ID == cleanID {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return nil, nil, fmt.Errorf("nie znaleziono rewizji %s", cleanID)
	}
	rev := h.Revisions[targetIdx]
	b, err := loadBookFromHistoryRevision(baseBook.RootPath, rev)
	if err != nil {
		return nil, nil, err
	}
	b.IsExternal = baseBook.IsExternal
	b.SourceURL = baseBook.SourceURL
	if baseBook.ID != "" {
		b.ID = baseBook.ID
	}

	var audioFiles []string
	var imageFiles []string
	_ = filepath.WalkDir(baseBook.RootPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != baseBook.RootPath && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, relErr := filepath.Rel(baseBook.RootPath, path)
		if relErr != nil {
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
	matchBookMedia(b, audioFiles, imageFiles)
	if baseBook.ID != "" {
		RebindBookID(b, baseBook.ID)
	}
	return b, &rev, nil
}

func formatSignedInt(delta int) string {
	if delta > 0 {
		return fmt.Sprintf("+%d", delta)
	}
	return fmt.Sprintf("%d", delta)
}

// CompareBookBranches loads both branch trees in memory and computes scene delta, page/runtime delta, and location/cast delta.
func CompareBookBranches(b *Book, branchA, branchB string) (*BranchComparison, error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return nil, fmt.Errorf("brak rozpakowanej książki na dysku")
	}
	h, err := LoadBookHistory(b)
	if err != nil {
		return nil, err
	}
	normA := normalizeRevisionBranch(branchA)
	normB := normalizeRevisionBranch(branchB)

	findHeadRev := func(brName string) (HistoryRevision, bool) {
		for _, br := range h.Branches {
			if strings.EqualFold(br.Name, brName) && br.HeadRevID != "" {
				for _, rev := range h.Revisions {
					if rev.ID == br.HeadRevID {
						return rev, true
					}
				}
			}
		}
		for i := len(h.Revisions) - 1; i >= 0; i-- {
			if normalizeRevisionBranch(h.Revisions[i].Branch) == brName {
				return h.Revisions[i], true
			}
		}
		return HistoryRevision{}, false
	}

	revA, okA := findHeadRev(normA)
	revB, okB := findHeadRev(normB)
	if !okA {
		return nil, fmt.Errorf("nie znaleziono gałęzi „%s”", normA)
	}
	if !okB {
		return nil, fmt.Errorf("nie znaleziono gałęzi „%s”", normB)
	}

	bookA, err := loadBookFromHistoryRevision(b.RootPath, revA)
	if err != nil {
		return nil, err
	}
	bookB, err := loadBookFromHistoryRevision(b.RootPath, revB)
	if err != nil {
		return nil, err
	}

	bdA := AnalyzeScreenplay(bookA)
	bdB := AnalyzeScreenplay(bookB)

	scenesMapA := make(map[string]*Scene, len(bookA.Scenes))
	for _, sc := range bookA.Scenes {
		scenesMapA[sc.ID] = sc
	}
	scenesMapB := make(map[string]*Scene, len(bookB.Scenes))
	for _, sc := range bookB.Scenes {
		scenesMapB[sc.ID] = sc
	}

	var added, removed, modified []string
	for _, scB := range bookB.Scenes {
		scA, exists := scenesMapA[scB.ID]
		if !exists {
			added = append(added, scB.DisplayName())
		} else if scA.Text != scB.Text || scA.Slugline != scB.Slugline || scA.Name != scB.Name {
			modified = append(modified, scB.DisplayName())
		}
	}
	for _, scA := range bookA.Scenes {
		if _, exists := scenesMapB[scA.ID]; !exists {
			removed = append(removed, scA.DisplayName())
		}
	}

	eighthsDelta := bdB.TotalEighths - bdA.TotalEighths
	pagesDeltaStr := "0 str."
	if eighthsDelta > 0 {
		pagesDeltaStr = "+" + FormatEighths(eighthsDelta)
	} else if eighthsDelta < 0 {
		pagesDeltaStr = "-" + FormatEighths(-eighthsDelta)
	}

	cmp := &BranchComparison{
		BranchA:             normA,
		BranchB:             normB,
		ScenesA:             len(bookA.Scenes),
		ScenesB:             len(bookB.Scenes),
		SceneDelta:          len(bookB.Scenes) - len(bookA.Scenes),
		AddedScenes:         added,
		RemovedScenes:       removed,
		ModifiedScenes:      modified,
		EighthsA:            bdA.TotalEighths,
		EighthsB:            bdB.TotalEighths,
		EighthsDelta:        eighthsDelta,
		PagesDeltaFormatted: pagesDeltaStr,
		MinutesA:            bdA.EstimatedMinutes,
		MinutesB:            bdB.EstimatedMinutes,
		MinutesDelta:        bdB.EstimatedMinutes - bdA.EstimatedMinutes,
		LocationsA:          len(bdA.Locations),
		LocationsB:          len(bdB.Locations),
		LocationDelta:       len(bdB.Locations) - len(bdA.Locations),
		CharactersA:         len(bdA.Characters),
		CharactersB:         len(bdB.Characters),
		CharacterDelta:      len(bdB.Characters) - len(bdA.Characters),
	}

	if cmp.SceneDelta == 0 && cmp.EighthsDelta == 0 && len(cmp.ModifiedScenes) == 0 {
		cmp.SummaryLabel = fmt.Sprintf("Zgodna z %s (brak różnic)", normA)
	} else {
		cmp.SummaryLabel = fmt.Sprintf("vs %s: sceny %s, objętość %s, zmodyfikowane: %d",
			normA, formatSignedInt(cmp.SceneDelta), cmp.PagesDeltaFormatted, len(cmp.ModifiedScenes))
	}
	return cmp, nil
}
