package book

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	outZip := filepath.Join(booksDir, "exported.stbook")
	if err := PackBookToZip(b.RootPath, outZip); err != nil {
		t.Fatalf("PackBookToZip failed: %v", err)
	}
	packedBook, err := LoadBookFromZip(outZip)
	if err != nil {
		t.Fatalf("LoadBookFromZip failed: %v", err)
	}
	for _, tf := range packedBook.TextFiles {
		if strings.Contains(tf, "history") {
			t.Fatalf("expected .history to be excluded from .stbook archive, found %q", tf)
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
	ch1Path := filepath.Join(b.RootPath, "rozdzial_1.txt")
	ch2Path := filepath.Join(b.RootPath, "rozdzial_2.txt")
	_ = os.Remove(filepath.Join(b.RootPath, "story.txt"))
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
	ch3Path := filepath.Join(b.RootPath, "rozdzial_3.txt")
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
	// Verify unchanged rozdzial_1.txt shares the exact same SHA-256 blob hash between r1 and r2
	if h.Revisions[0].Files["rozdzial_1.txt"] == "" || h.Revisions[0].Files["rozdzial_1.txt"] != h.Revisions[1].Files["rozdzial_1.txt"] {
		t.Fatalf("expected unchanged rozdzial_1.txt to reuse identical blob hash across revisions")
	}

	// Undo should remove rozdzial_3.txt from disk since it did not exist in r1
	if _, _, err := UndoBookHistory(b); err != nil {
		t.Fatalf("UndoBookHistory failed: %v", err)
	}
	if _, err := os.Stat(ch3Path); !os.IsNotExist(err) {
		t.Fatalf("expected rozdzial_3.txt to be removed after undo to baseline")
	}

	// Redo brings rozdzial_3.txt back
	if _, _, err := RedoBookHistory(b); err != nil {
		t.Fatalf("RedoBookHistory failed: %v", err)
	}
	if _, err := os.Stat(ch3Path); err != nil {
		t.Fatalf("expected rozdzial_3.txt to be restored after redo: %v", err)
	}

	// Modify ch3 text in a new unnamed edit, then collapse into a Named Version and verify old ch3 blob is pruned
	oldCh3Hash := h.Revisions[1].Files["rozdzial_3.txt"]
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
