//go:build !(js && wasm)

package book

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

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
	if trimmed == "" || trimmed == "---" {
		return true
	}
	if strings.Contains(trimmed, "->") && !strings.Contains(trimmed, `\->`) {
		return true
	}
	colonIdx := strings.IndexByte(trimmed, ':')
	if colonIdx > 0 && colonIdx <= 16 {
		key := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
		switch key {
		case "chapter", "act", "scene", "id", "exit", "next", "label", "if", "elif", "else", "endif", "set", "end", "image", "voice", "voice_trim",
			"location", "characters", "slugline", "synopsis", "text",
			"title", "author", "license", "description", "narrated", "imageless",
			"start", "cover", "export_dir", "mode", "language", "standard", "enum", "var":
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
	for maxPass := len(h.Revisions); maxPass > 0; maxPass-- {
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
	for maxPass := len(h.Revisions); maxPass > 0; maxPass-- {
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

func stepBookHistory(b *Book, delta int, emptyErr string) (selectedSceneID string, label string, err error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return "", "", fmt.Errorf("brak aktywnej książki")
	}
	h, err := LoadBookHistory(b)
	if err != nil {
		return "", "", err
	}
	if (delta < 0 && !h.CanUndo()) || (delta > 0 && !h.CanRedo()) {
		return "", "", fmt.Errorf("%s", emptyErr)
	}
	branchIdxs := h.ActiveBranchRevisions()
	curPos := 0
	if delta < 0 {
		curPos = len(branchIdxs) - 1
	}
	for j, idx := range branchIdxs {
		if h.Revisions[idx].ID == h.Cursor {
			curPos = j
			break
		}
	}
	curRev := h.Revisions[branchIdxs[curPos]]
	targetRev := h.Revisions[branchIdxs[curPos+delta]]

	if err := restoreBookTextFiles(b, targetRev); err != nil {
		return "", "", err
	}
	h.Cursor = targetRev.ID
	h.syncBranchesMetadata()
	if err := SaveBookHistory(b, h); err != nil {
		return "", "", err
	}
	if delta < 0 {
		return targetRev.SelectedSceneID, curRev.Label, nil
	}
	return targetRev.SelectedSceneID, targetRev.Label, nil
}

// UndoBookHistory steps backward by one unnamed edit on the active branch (stopping at the latest Named Version floor) and restores files to disk.
func UndoBookHistory(b *Book) (selectedSceneID string, label string, err error) {
	return stepBookHistory(b, -1, "brak zmian do cofnięcia od ostatniej zapisanej wersji")
}

// RedoBookHistory steps forward by one unnamed edit on the active branch and restores files to disk.
func RedoBookHistory(b *Book) (selectedSceneID string, label string, err error) {
	return stepBookHistory(b, 1, "brak zmian do ponowienia")
}

func loadHistoryAndFindRevision(b *Book, revID string) (*BookHistory, string, int, error) {
	if b == nil || b.IsPacked || strings.TrimSpace(b.RootPath) == "" {
		return nil, "", -1, fmt.Errorf("brak aktywnej książki")
	}
	cleanID := SanitizeSceneID(revID)
	if cleanID == "" {
		return nil, "", -1, fmt.Errorf("nieprawidłowy identyfikator rewizji")
	}
	h, err := LoadBookHistory(b)
	if err != nil {
		return nil, "", -1, err
	}
	targetIdx := -1
	for i := range h.Revisions {
		if h.Revisions[i].ID == cleanID {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return nil, "", -1, fmt.Errorf("nie znaleziono rewizji %s", cleanID)
	}
	return h, cleanID, targetIdx, nil
}

// RestoreBookHistoryRevision restores any unnamed step or Named Version.
// Restoring an older Named Version (prior to the latest Named Version) is non-destructive:
// it copies that version's files onto the top of the current unnamed edit stack so later Named Versions are preserved.
func RestoreBookHistoryRevision(b *Book, revID string) (selectedSceneID string, label string, err error) {
	h, _, targetIdx, err := loadHistoryAndFindRevision(b, revID)
	if err != nil {
		return "", "", err
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
