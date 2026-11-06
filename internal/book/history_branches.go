package book

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
