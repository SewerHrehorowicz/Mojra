package book

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func CreateStandaloneScene(b *Book, newSceneName string, x, y int, hasCoords bool) (*Scene, error) {
	return CreateStandaloneSceneInChapter(b, newSceneName, "", x, y, hasCoords)
}

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

func formatSignedInt(delta int) string {
	if delta > 0 {
		return fmt.Sprintf("+%d", delta)
	}
	return fmt.Sprintf("%d", delta)
}

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

func TestBookHistoryLifecycleAndNamedVersions(t *testing.T) {
	booksDir := t.TempDir()
	b, err := CreateNewBook(booksDir, "Kroniki Mgły", "Autor Testowy", false)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}

	// 1. Baseline creation before first edit
	if err := EnsureBookHistoryBaseline(b, "poczatek"); err != nil {
		t.Fatalf("EnsureBookHistoryBaseline failed: %v", err)
	}

	h, err := LoadBookHistory(b)
	if err != nil {
		t.Fatalf("LoadBookHistory failed: %v", err)
	}
	if len(h.Revisions) != 1 || !h.Revisions[0].Named || h.Revisions[0].Label != "Stan początkowy" {
		t.Fatalf("expected single baseline named revision, got: %+v", h.Revisions)
	}
	if h.CanUndo() || h.CanRedo() {
		t.Fatalf("expected CanUndo=false and CanRedo=false at baseline")
	}

	// Verify history.txt uses key: value syntax and zero JSON
	rawHist, err := os.ReadFile(bookHistoryIndexPath(b.RootPath))
	if err != nil {
		t.Fatalf("failed reading history.txt: %v", err)
	}
	if strings.Contains(string(rawHist), "{") || !strings.Contains(string(rawHist), "revision: r1") || !strings.Contains(string(rawHist), "named: true") {
		t.Fatalf("expected key: value history.txt without JSON braces, got:\n%s", string(rawHist))
	}

	// 2. Mutate book (add a scene) and record step
	created1, err := CreateStandaloneScene(b, "Scena w lesie", 400, 300, true)
	if err != nil {
		t.Fatalf("CreateStandaloneScene 1 failed: %v", err)
	}
	if err := RecordBookHistoryStep(b, "Dodano nową scenę: Scena w lesie", "graph", "", created1.ID); err != nil {
		t.Fatalf("RecordBookHistoryStep 1 failed: %v", err)
	}

	h, _ = LoadBookHistory(b)
	if len(h.Revisions) != 2 || h.UnnamedEditsCount() != 1 || !h.CanUndo() || h.CanRedo() {
		t.Fatalf("expected 2 revisions (1 unnamed), CanUndo=true, got revisions=%d unnamed=%d", len(h.Revisions), h.UnnamedEditsCount())
	}

	// 3. Coalesced rapid auto-saves on the same scene
	b.Scenes[len(b.Scenes)-1].Text = "Pierwsze zdanie w lesie."
	FinalizeBook(b)
	_ = SaveBookScenesToDisk(b)
	if err := RecordBookHistoryStep(b, "Edycja sceny: Scena w lesie", "scene", "autosave:"+created1.ID, created1.ID); err != nil {
		t.Fatalf("RecordBookHistoryStep autosave 1 failed: %v", err)
	}

	b.Scenes[len(b.Scenes)-1].Text = "Pierwsze zdanie w lesie. Drugie zdanie po chwili."
	FinalizeBook(b)
	_ = SaveBookScenesToDisk(b)
	if err := RecordBookHistoryStep(b, "Edycja sceny: Scena w lesie", "scene", "autosave:"+created1.ID, created1.ID); err != nil {
		t.Fatalf("RecordBookHistoryStep autosave 2 failed: %v", err)
	}

	h, _ = LoadBookHistory(b)
	if len(h.Revisions) != 3 || h.UnnamedEditsCount() != 2 {
		t.Fatalf("expected coalesced autosaves to produce 1 revision (total 3 revisions), got %d", len(h.Revisions))
	}

	// 4. Undo steps back through unnamed edits and stops at baseline Named Version
	selScene, undoneLbl, err := UndoBookHistory(b)
	if err != nil {
		t.Fatalf("UndoBookHistory 1 failed: %v", err)
	}
	if undoneLbl != "Edycja sceny: Scena w lesie" || selScene != created1.ID {
		t.Fatalf("unexpected undo 1 result: sel=%q label=%q", selScene, undoneLbl)
	}
	reloaded, err := LoadBookFromDir(b.RootPath)
	if err != nil {
		t.Fatalf("LoadBookFromDir after undo 1 failed: %v", err)
	}
	if reloaded.Scenes[len(reloaded.Scenes)-1].Text == "Pierwsze zdanie w lesie. Drugie zdanie po chwili." {
		t.Fatalf("expected scene text to be reverted after undo 1")
	}

	// Undo second step back to baseline
	_, _, err = UndoBookHistory(b)
	if err != nil {
		t.Fatalf("UndoBookHistory 2 failed: %v", err)
	}
	reloaded, _ = LoadBookFromDir(b.RootPath)
	if len(reloaded.Scenes) != 2 {
		t.Fatalf("expected 2 initial scenes at baseline after undo 2, got %d", len(reloaded.Scenes))
	}

	// Cannot undo past baseline Named Version
	if _, _, err := UndoBookHistory(b); err == nil {
		t.Fatalf("expected UndoBookHistory at baseline floor to return an error")
	}

	// 5. Redo both steps back to latest state
	if _, _, err := RedoBookHistory(b); err != nil {
		t.Fatalf("RedoBookHistory 1 failed: %v", err)
	}
	if _, _, err := RedoBookHistory(b); err != nil {
		t.Fatalf("RedoBookHistory 2 failed: %v", err)
	}
	reloaded, _ = LoadBookFromDir(b.RootPath)
	if len(reloaded.Scenes) != 3 || !strings.Contains(reloaded.Scenes[2].Text, "Drugie zdanie po chwili.") {
		t.Fatalf("expected 3 scenes and restored text after redo, got len=%d", len(reloaded.Scenes))
	}

	// 6. Save a Named Version ("Wersja Alfa") -> collapses unnamed edits and prunes intermediate blobs
	verAlfa, err := SaveNamedBookVersion(reloaded, "Wersja Alfa", created1.ID)
	if err != nil {
		t.Fatalf("SaveNamedBookVersion failed: %v", err)
	}
	if !verAlfa.Named || verAlfa.Label != "Wersja Alfa" {
		t.Fatalf("unexpected saved version: %+v", verAlfa)
	}
	h, _ = LoadBookHistory(reloaded)
	if len(h.Revisions) != 2 || h.UnnamedEditsCount() != 0 || h.CanUndo() {
		t.Fatalf("expected unnamed edits to collapse into Wersja Alfa (total 2 named revisions, CanUndo=false), got len=%d unnamed=%d canUndo=%v",
			len(h.Revisions), h.UnnamedEditsCount(), h.CanUndo())
	}

	// 7. Make another edit after "Wersja Alfa", then restore older Named Version ("Stan początkowy") non-destructively
	b = reloaded
	created2, err := CreateStandaloneScene(b, "Scena na zamku", 600, 300, true)
	if err != nil {
		t.Fatalf("CreateStandaloneScene 2 failed: %v", err)
	}
	if err := RecordBookHistoryStep(b, "Dodano nową scenę: Scena na zamku", "graph", "", created2.ID); err != nil {
		t.Fatalf("RecordBookHistoryStep after Wersja Alfa failed: %v", err)
	}

	// Restore "r1" ("Stan początkowy") which is older than "Wersja Alfa"
	_, restoredLbl, err := RestoreBookHistoryRevision(b, "r1")
	if err != nil {
		t.Fatalf("RestoreBookHistoryRevision(r1) failed: %v", err)
	}
	if restoredLbl != "Stan początkowy" {
		t.Fatalf("expected restored label 'Stan początkowy', got %q", restoredLbl)
	}
	reloaded, _ = LoadBookFromDir(b.RootPath)
	if len(reloaded.Scenes) != 2 {
		t.Fatalf("expected 2 scenes after restoring r1, got %d", len(reloaded.Scenes))
	}

	h, _ = LoadBookHistory(reloaded)
	// Both "Stan początkowy" and "Wersja Alfa" must still exist!
	namedCount := 0
	for _, rev := range h.Revisions {
		if rev.Named {
			namedCount++
		}
	}
	if namedCount != 2 {
		t.Fatalf("expected both Named Versions to be preserved after restoring older version, got %d", namedCount)
	}
	if !h.CanUndo() {
		t.Fatalf("expected non-destructive restore of older Named Version to be undoable with Ctrl+Z")
	}

	// Undo the restore -> returns to the 4-scene state ("Scena na zamku")!
	if _, _, err := UndoBookHistory(reloaded); err != nil {
		t.Fatalf("UndoBookHistory after version restore failed: %v", err)
	}
	reloaded, _ = LoadBookFromDir(b.RootPath)
	if len(reloaded.Scenes) != 4 {
		t.Fatalf("expected 4 scenes after undoing version restore, got %d", len(reloaded.Scenes))
	}

	// 8. Verify PackBookToZip excludes .history/
	outZip := filepath.Join(booksDir, "exported.playbook")
	if err := PackBookToZip(b.RootPath, outZip); err != nil {
		t.Fatalf("PackBookToZip failed: %v", err)
	}
	packedBook, err := LoadBookFromZip(outZip)
	if err != nil {
		t.Fatalf("LoadBookFromZip failed: %v", err)
	}
	for _, tf := range packedBook.TextFiles {
		if strings.Contains(tf, "history") {
			t.Fatalf("expected .history to be excluded from .playbook archive, found %q", tf)
		}
	}
}

func TestBookHistoryMultiChapterDeltaAndBlobPruning(t *testing.T) {
	booksDir := t.TempDir()
	b, err := CreateNewBook(booksDir, "Wielo-Rozdziałowa", "Autor", false)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	// Convert to multi-file book by adding a second chapter file on disk
	ch1Path := filepath.Join(b.RootPath, "rozdzial_1.playtxt")
	ch2Path := filepath.Join(b.RootPath, "rozdzial_2.playtxt")
	_ = os.Remove(filepath.Join(b.RootPath, "story.playtxt"))
	if err := os.WriteFile(ch1Path, []byte("chapter: Rozdział 1\nid: rozdzial_1\n\nscene: Start\nid: poczatek\nnext: s2\ntext:\nTekst rozdziału pierwszego.\n"), 0644); err != nil {
		t.Fatalf("write ch1 failed: %v", err)
	}
	if err := os.WriteFile(ch2Path, []byte("chapter: Rozdział 2\nid: rozdzial_2\n\nscene: Druga\nid: s2\nend: true\ntext:\nTekst rozdziału drugiego.\n"), 0644); err != nil {
		t.Fatalf("write ch2 failed: %v", err)
	}
	b, err = LoadBookFromDir(b.RootPath)
	if err != nil {
		t.Fatalf("LoadBookFromDir failed: %v", err)
	}

	if err := EnsureBookHistoryBaseline(b, "poczatek"); err != nil {
		t.Fatalf("EnsureBookHistoryBaseline failed: %v", err)
	}

	// Modify only Rozdział 2 and add a third chapter file
	ch3Path := filepath.Join(b.RootPath, "rozdzial_3.playtxt")
	if err := os.WriteFile(ch3Path, []byte("chapter: Rozdział 3\nid: rozdzial_3\n\nscene: Trzecia\nid: s3\nend: true\ntext:\nNowy rozdział trzeci.\n"), 0644); err != nil {
		t.Fatalf("write ch3 failed: %v", err)
	}
	b, _ = LoadBookFromDir(b.RootPath)
	if err := RecordBookHistoryStep(b, "Dodano rozdział 3", "chapter", "", "s3"); err != nil {
		t.Fatalf("RecordBookHistoryStep failed: %v", err)
	}

	h, _ := LoadBookHistory(b)
	if len(h.Revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(h.Revisions))
	}
	// Verify unchanged rozdzial_1.playtxt shares the exact same SHA-256 blob hash between r1 and r2
	if h.Revisions[0].Files["rozdzial_1.playtxt"] == "" || h.Revisions[0].Files["rozdzial_1.playtxt"] != h.Revisions[1].Files["rozdzial_1.playtxt"] {
		t.Fatalf("expected unchanged rozdzial_1.playtxt to reuse identical blob hash across revisions")
	}

	// Undo should remove rozdzial_3.playtxt from disk since it did not exist in r1
	if _, _, err := UndoBookHistory(b); err != nil {
		t.Fatalf("UndoBookHistory failed: %v", err)
	}
	if _, err := os.Stat(ch3Path); !os.IsNotExist(err) {
		t.Fatalf("expected rozdzial_3.playtxt to be removed after undo to baseline")
	}

	// Redo brings rozdzial_3.playtxt back
	if _, _, err := RedoBookHistory(b); err != nil {
		t.Fatalf("RedoBookHistory failed: %v", err)
	}
	if _, err := os.Stat(ch3Path); err != nil {
		t.Fatalf("expected rozdzial_3.playtxt to be restored after redo: %v", err)
	}

	// Modify ch3 text in a new unnamed edit, then collapse into a Named Version and verify old ch3 blob is pruned
	oldCh3Hash := h.Revisions[1].Files["rozdzial_3.playtxt"]
	if err := os.WriteFile(ch3Path, []byte("chapter: Rozdział 3\nid: rozdzial_3\n\nscene: Trzecia\nid: s3\nend: true\ntext:\nZmieniony tekst rozdziału trzeciego.\n"), 0644); err != nil {
		t.Fatalf("write updated ch3 failed: %v", err)
	}
	b, _ = LoadBookFromDir(b.RootPath)
	_ = RecordBookHistoryStep(b, "Zmieniono rozdział 3", "scene", "", "s3")

	if _, err := SaveNamedBookVersion(b, "Wydanie 1", "s3"); err != nil {
		t.Fatalf("SaveNamedBookVersion failed: %v", err)
	}
	// Intermediate blob oldCh3Hash is not in r1 (baseline) and not in Wydanie 1, so it must be pruned
	if _, err := os.Stat(bookHistoryBlobPath(b.RootPath, oldCh3Hash)); !os.IsNotExist(err) {
		t.Fatalf("expected unreferenced intermediate blob %s.gz to be pruned after SaveNamedBookVersion", oldCh3Hash)
	}
}

func TestHistoryBranchesCreateSwitchCompareDelete(t *testing.T) {
	booksDir := t.TempDir()
	b, err := CreateNewBookWithMode(booksDir, "Wersje Scenariusza", "Autor", "screenplay", "pl", false, false)
	if err != nil {
		t.Fatalf("CreateNewBookWithMode failed: %v", err)
	}
	if err := EnsureBookHistoryBaseline(b, b.StartSceneID); err != nil {
		t.Fatalf("EnsureBookHistoryBaseline failed: %v", err)
	}

	// Fork a new branch "wersja-rezysera"
	br, err := CreateBookBranch(b, "wersja-rezysera", "")
	if err != nil || br == nil || br.Name != "wersja-rezysera" {
		t.Fatalf("CreateBookBranch failed: br=%+v err=%v", br, err)
	}

	// Add a new scene on "wersja-rezysera"
	inserted, err := InsertSceneBetween(b, b.Scenes[0].ID, b.Scenes[1].ID, "Dodatkowa scena reżysera")
	if err != nil || inserted == nil {
		t.Fatalf("InsertSceneBetween on branch failed: %v", err)
	}
	if err := RecordBookHistoryStep(b, "Dodano scenę na gałęzi reżysera", "graph", "", inserted.ID); err != nil {
		t.Fatalf("RecordBookHistoryStep on branch failed: %v", err)
	}
	if len(b.Scenes) != 4 {
		t.Fatalf("expected 4 scenes on wersja-rezysera, got %d", len(b.Scenes))
	}

	// Compare "main" vs "wersja-rezysera"
	cmp, err := CompareBookBranches(b, "main", "wersja-rezysera")
	if err != nil {
		t.Fatalf("CompareBookBranches failed: %v", err)
	}
	if cmp.SceneDelta != 1 || len(cmp.AddedScenes) != 1 {
		t.Fatalf("expected +1 scene delta between main and wersja-rezysera, got %+v", cmp)
	}

	// Switch back to "main"
	if _, err := SwitchBookBranch(b, "main"); err != nil {
		t.Fatalf("SwitchBookBranch(main) failed: %v", err)
	}
	if len(b.Scenes) != 3 {
		t.Fatalf("expected 3 scenes after switching back to main, got %d", len(b.Scenes))
	}

	// Switch to "wersja-rezysera" again and verify 4 scenes are restored
	if _, err := SwitchBookBranch(b, "wersja-rezysera"); err != nil {
		t.Fatalf("SwitchBookBranch(wersja-rezysera) failed: %v", err)
	}
	if len(b.Scenes) != 4 {
		t.Fatalf("expected 4 scenes after switching to wersja-rezysera, got %d", len(b.Scenes))
	}

	// Protecting "main" and active branch from deletion
	if err := DeleteBookBranch(b, "main"); err == nil {
		t.Fatalf("expected DeleteBookBranch(main) to be rejected")
	}
	if err := DeleteBookBranch(b, "wersja-rezysera"); err == nil {
		t.Fatalf("expected DeleteBookBranch on active branch to be rejected")
	}

	// Switch to "main" and delete "wersja-rezysera"
	if _, err := SwitchBookBranch(b, "main"); err != nil {
		t.Fatalf("SwitchBookBranch(main) failed: %v", err)
	}
	if err := DeleteBookBranch(b, "wersja-rezysera"); err != nil {
		t.Fatalf("DeleteBookBranch(wersja-rezysera) failed: %v", err)
	}
	h, _ := LoadBookHistory(b)
	if len(h.Branches) != 1 || h.Branches[0].Name != "main" {
		t.Fatalf("expected only main branch remaining, got %+v", h.Branches)
	}
}

func TestCopyHistoryRevisionToBranchAndHoverSummaries(t *testing.T) {
	booksDir := t.TempDir()
	b, err := CreateNewBookWithMode(booksDir, "Drzewo Wariantów", "Autor", "screenplay", "pl", false, false)
	if err != nil {
		t.Fatalf("CreateNewBookWithMode failed: %v", err)
	}
	if err := EnsureBookHistoryBaseline(b, b.StartSceneID); err != nil {
		t.Fatalf("EnsureBookHistoryBaseline failed: %v", err)
	}

	// Create branch "mroczny-final" and add a scene on it
	brDark, err := CreateBookBranch(b, "mroczny-final", "")
	if err != nil || brDark == nil {
		t.Fatalf("CreateBookBranch(mroczny-final) failed: %v", err)
	}
	inserted, err := InsertSceneBetween(b, b.Scenes[0].ID, b.Scenes[1].ID, "Scena mrocznego finału")
	if err != nil || inserted == nil {
		t.Fatalf("InsertSceneBetween failed: %v", err)
	}
	if err := RecordBookHistoryStep(b, "Dodano mroczny finał", "graph", "", inserted.ID); err != nil {
		t.Fatalf("RecordBookHistoryStep failed: %v", err)
	}
	verDark, err := SaveNamedBookVersion(b, "Mroczny Finał v1", inserted.ID)
	if err != nil || verDark == nil {
		t.Fatalf("SaveNamedBookVersion on mroczny-final failed: %v", err)
	}

	// Switch back to "main" (3 scenes)
	if _, err := SwitchBookBranch(b, "main"); err != nil {
		t.Fatalf("SwitchBookBranch(main) failed: %v", err)
	}
	if len(b.Scenes) != 3 {
		t.Fatalf("expected 3 scenes on main before copy, got %d", len(b.Scenes))
	}

	// Copy verDark.ID from "mroczny-final" onto "main"
	selID, copiedLabel, err := CopyHistoryRevisionToBranch(b, verDark.ID, "main")
	if err != nil {
		t.Fatalf("CopyHistoryRevisionToBranch failed: %v", err)
	}
	if copiedLabel != "Mroczny Finał v1" || selID == "" {
		t.Fatalf("unexpected CopyHistoryRevisionToBranch result: sel=%q label=%q", selID, copiedLabel)
	}
	if len(b.Scenes) != 4 {
		t.Fatalf("expected 4 scenes on main after copying revision from mroczny-final, got %d", len(b.Scenes))
	}

	h, err := LoadBookHistory(b)
	if err != nil {
		t.Fatalf("LoadBookHistory failed: %v", err)
	}
	revHints, branchHints := BuildHistoryHoverSummaries(b, h)
	if !strings.Contains(branchHints["main"], "main • 4 scen") || !strings.Contains(revHints[verDark.ID], "mroczny-final • 4 scen") {
		t.Fatalf("unexpected hover summaries: branchHints=%+v revHints=%+v", branchHints, revHints)
	}
}

func TestSmartCoalescingDeltaAndFiveMinuteCompaction(t *testing.T) {
	booksDir := t.TempDir()
	b, err := CreateNewBook(booksDir, "Test Scalania", "Autor", false)
	if err != nil {
		t.Fatalf("CreateNewBook failed: %v", err)
	}
	scID := b.Scenes[0].ID
	if err := EnsureBookHistoryBaseline(b, scID); err != nil {
		t.Fatalf("EnsureBookHistoryBaseline failed: %v", err)
	}

	// 1. Small multi-line edits on the same scene (even across autosave and textmodal keys) coalesce into 1 step
	b.Scenes[0].Text = "Linia pierwsza.\nLinia druga."
	FinalizeBook(b)
	_ = SaveBookScenesToDisk(b)
	_ = RecordBookHistoryStep(b, "Edycja sceny: Start", "scene", "autosave:"+scID, scID)

	b.Scenes[0].Text = "Linia pierwsza.\nLinia druga.\nLinia trzecia."
	FinalizeBook(b)
	_ = SaveBookScenesToDisk(b)
	_ = RecordBookHistoryStep(b, "Edycja treści sceny: Start", "text", "textmodal:"+scID, scID)

	h, _ := LoadBookHistory(b)
	if len(h.Revisions) != 2 {
		t.Fatalf("expected small same-scene edits to coalesce into 1 unnamed revision (total 2), got %d", len(h.Revisions))
	}

	// 2. Large pasted block (>= 300 chars) must NOT coalesce into the small edit; must be preserved as its own step
	largePaste := strings.Repeat("Wklejony duży akapit tekstu fabularnego o bardzo długiej treści. ", 6)
	b.Scenes[0].Text = "Linia pierwsza.\nLinia druga.\nLinia trzecia.\n\n" + largePaste
	FinalizeBook(b)
	_ = SaveBookScenesToDisk(b)
	_ = RecordBookHistoryStep(b, "Edycja sceny: Start", "scene", "autosave:"+scID, scID)

	h, _ = LoadBookHistory(b)
	if len(h.Revisions) != 3 {
		t.Fatalf("expected large pasted block to create a separate revision (total 3), got %d", len(h.Revisions))
	}

	// 3. Subsequent small edits after the large paste must NOT overwrite the large paste revision, but coalesce together
	b.Scenes[0].Text += "\nKrótki dopisek."
	FinalizeBook(b)
	_ = SaveBookScenesToDisk(b)
	_ = RecordBookHistoryStep(b, "Edycja sceny: Start", "scene", "autosave:"+scID, scID)

	b.Scenes[0].Text += " I jeszcze słowo."
	FinalizeBook(b)
	_ = SaveBookScenesToDisk(b)
	_ = RecordBookHistoryStep(b, "Edycja sceny: Start", "scene", "autosave:"+scID, scID)

	h, _ = LoadBookHistory(b)
	if len(h.Revisions) != 4 {
		t.Fatalf("expected post-paste small edits to start 4th revision and coalesce together (total 4), got %d", len(h.Revisions))
	}

	// 4. Consecutive node drags on different scenes coalesce into a single layout revision
	b.Scenes[0].X = 120
	b.Scenes[0].Y = 120
	_ = SaveBookEditorStateToDisk(b)
	_ = RecordBookHistoryStep(b, "Przesunięto węzeł: Start", "layout", "nodedrag:"+b.Scenes[0].ID, b.Scenes[0].ID)

	b.Scenes[1].X = 360
	b.Scenes[1].Y = 240
	_ = SaveBookEditorStateToDisk(b)
	_ = RecordBookHistoryStep(b, "Przesunięto węzeł: Druga", "layout", "nodedrag:"+b.Scenes[1].ID, b.Scenes[1].ID)

	h, _ = LoadBookHistory(b)
	if len(h.Revisions) != 5 || h.Revisions[len(h.Revisions)-1].Label != "Przesunięto węzły" {
		t.Fatalf("expected consecutive node drags to coalesce into 5th revision 'Przesunięto węzły', got len=%d last=%+v",
			len(h.Revisions), h.Revisions[len(h.Revisions)-1])
	}

	// 5. Simulate revisions older than 3 minutes in the same 5-minute bucket:
	// Small edits and layout moves in that 5-minute bucket compact, while the large paste revision remains intact.
	bucketBase := int64(1700000100) // all within [1700000100..1700000299] (same 300s bucket)
	for i := 1; i < len(h.Revisions); i++ {
		h.Revisions[i].TimeUnix = bucketBase + int64(i*20)
	}
	compactHistoryRevisions(b.RootPath, h, bucketBase+600, nil)
	_ = SaveBookHistory(b, h)

	compactedH, err := LoadBookHistory(b)
	if err != nil {
		t.Fatalf("LoadBookHistory after aging failed: %v", err)
	}
	// Expected remaining revisions:
	// r1 (Named baseline), r2 (pre-paste small edit), r3 (large paste), and the merged post-paste+layout step!
	if len(compactedH.Revisions) != 4 {
		t.Fatalf("expected 5-minute bucket compaction to merge post-paste small edit and layout move while keeping pre-paste and large paste (total 4), got %d",
			len(compactedH.Revisions))
	}

	// 6. Incremental scene rename ("L" -> "Lu" -> "Luis" -> "Luis cz" -> "Luis czeka") after a node drag,
	// even when linked from scene[0] earlier in the file, must coalesce into a single "Edycja sceny: Luis czeka" revision.
	sc2ID := b.Scenes[1].ID
	b.Scenes[1].X = 480
	_ = SaveBookEditorStateToDisk(b)
	_ = RecordBookHistoryStep(b, "Przesunięto węzły", "layout", "nodedrag:batch", sc2ID)

	for _, partialTitle := range []string{"L", "Lu", "Luis", "Luis cz", "Luis czeka"} {
		b.Scenes[0].ExplicitNext = partialTitle
		b.Scenes[1].Name = partialTitle
		FinalizeBook(b)
		_ = SaveBookScenesToDisk(b)
		_ = RecordBookHistoryStep(b, "Edycja sceny: "+partialTitle, "scene", "autosave:"+sc2ID, sc2ID)
	}

	b.Scenes[1].Text = "Luis czeka przy oknie.\nPatrzy w deszcz."
	FinalizeBook(b)
	_ = SaveBookScenesToDisk(b)
	_ = RecordBookHistoryStep(b, "Edycja sceny: Luis czeka", "scene", "autosave:"+sc2ID, sc2ID)

	hAfterRename, _ := LoadBookHistory(b)
	lastRev := hAfterRename.Revisions[len(hAfterRename.Revisions)-1]
	if len(hAfterRename.Revisions) != 5 || lastRev.Label != "Edycja sceny: Luis czeka" {
		t.Fatalf("expected layout drag + 5 incremental title renames + text edit to coalesce into 1 revision 'Edycja sceny: Luis czeka' (total 5), got len=%d last=%+v",
			len(hAfterRename.Revisions), lastRev)
	}
}

