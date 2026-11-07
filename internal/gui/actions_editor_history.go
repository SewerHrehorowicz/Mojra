package gui

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mojra/internal/book"
)

const historyPreviewLockedToast = "Podgląd wersji historycznej jest zablokowany do edycji"

func (s *Server) isHistoryPreview() bool {
	return s.PreviewRevisionID != "" && s.ActiveBook != nil && !s.StandaloneMode
}

func (s *Server) exitHistoryPreview() {
	if s.PreviewRevisionID == "" {
		return
	}
	s.PreviewRevisionID = ""
	s.PreviewRevisionLabel = ""
	if s.LiveBook != nil {
		s.ActiveBook = s.LiveBook
		s.LiveBook = nil
	}
	if s.LivePlayerBook != nil {
		s.PlayerBook = s.LivePlayerBook
		s.LivePlayerBook = nil
	}
	s.reloadActiveBook()
	if s.LiveSave != nil {
		s.Save = s.LiveSave
		s.LiveSave = nil
	} else if s.PlayerBook != nil {
		s.Save = book.LoadSaveState(s.SavesDir, s.PlayerBook.ID, s.PlayerBook.StartSceneID)
	}
	if s.PlayerBook != nil && s.Save != nil {
		if s.findPlayerScene(s.Save.CurrentSceneID) == nil && len(s.PlayerBook.Scenes) > 0 {
			s.Save.CurrentSceneID = s.PlayerBook.StartSceneID
			if s.findPlayerScene(s.Save.CurrentSceneID) == nil {
				s.Save.CurrentSceneID = s.PlayerBook.Scenes[0].ID
			}
		}
	}
	if s.LiveSelectedSceneID != "" {
		if sc := s.findScene(s.LiveSelectedSceneID); sc != nil {
			s.SelectedSceneID = sc.ID
		}
		s.LiveSelectedSceneID = ""
	}
	if s.ActiveBook != nil && s.findScene(s.SelectedSceneID) == nil && len(s.ActiveBook.Scenes) > 0 {
		s.SelectedSceneID = s.ActiveBook.Scenes[0].ID
	}
	if s.SceneTextModalID != "" {
		if sc := s.findScene(s.SceneTextModalID); sc != nil {
			s.SceneTextModalID = sc.ID
		} else {
			s.SceneTextModalID = ""
		}
	}
	s.HistoryRevealed = false
	s.FocusedHistoryIdx = -1
}

func (s *Server) invalidateHistoryCache() {
	s.cachedHistory = nil
	s.cachedHistoryBookID = ""
	s.cachedHistoryModNano = 0
	s.cachedHistorySize = 0
}

func (s *Server) loadActiveBookHistory() *book.BookHistory {
	if s.ActiveBook == nil || s.ActiveBook.IsPacked || s.StandaloneMode || strings.TrimSpace(s.ActiveBook.RootPath) == "" {
		return nil
	}
	indexPath := filepath.Join(s.ActiveBook.RootPath, ".history", "history.txt")
	info, statErr := os.Stat(indexPath)
	var modNano, size int64
	if statErr == nil {
		modNano = info.ModTime().UnixNano()
		size = info.Size()
	}
	if s.cachedHistory != nil &&
		s.cachedHistoryBookID == s.ActiveBook.ID &&
		s.cachedHistoryModNano == modNano &&
		s.cachedHistorySize == size {
		return s.cachedHistory
	}
	h, err := book.LoadBookHistory(s.ActiveBook)
	if err != nil || h == nil {
		return nil
	}
	s.cachedHistory = h
	s.cachedHistoryBookID = s.ActiveBook.ID
	s.cachedHistoryModNano = modNano
	s.cachedHistorySize = size
	return h
}

func (s *Server) reloadActiveBook() {
	if s.ActiveBook == nil {
		return
	}
	prevID := s.ActiveBook.ID
	prevExt := s.ActiveBook.IsExternal
	prevExportDir := s.ActiveBook.ExportDir
	if prevExportDir == "" && s.Settings != nil {
		prevExportDir = s.Settings.GetBookExportDir(prevID)
	}
	if s.ActiveBook.RootPath != "" {
		if reloaded, err := book.LoadBookFromPath(s.ActiveBook.RootPath); err == nil && reloaded != nil {
			reloaded.IsExternal = prevExt
			reloaded.ExportDir = prevExportDir
			if prevID != "" && reloaded.ID != prevID {
				book.RebindBookID(reloaded, prevID)
			}
			s.ActiveBook = reloaded
			if !s.isHistoryPreview() && s.PlayerBook != nil && s.PlayerBook.ID == reloaded.ID {
				s.PlayerBook = reloaded
			}
			return
		}
	}
	if reloaded, err := s.findBookByID(prevID); err == nil && reloaded != nil {
		reloaded.ExportDir = prevExportDir
		s.ActiveBook = reloaded
		if !s.isHistoryPreview() && s.PlayerBook != nil && s.PlayerBook.ID == reloaded.ID {
			s.PlayerBook = reloaded
		}
	}
}

func (s *Server) ensureHistoryBaselineBeforeMutation() {
	if s.isHistoryPreview() {
		return
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked && !s.StandaloneMode {
		if err := book.EnsureBookHistoryBaseline(s.ActiveBook, s.SelectedSceneID); err != nil {
			return
		}
		s.invalidateHistoryCache()
	}
}

func (s *Server) recordHistoryAfterMutation(label, kind, coalesceKey string) {
	if s.isHistoryPreview() {
		return
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked && !s.StandaloneMode {
		if err := book.RecordBookHistoryStep(s.ActiveBook, label, kind, coalesceKey, s.SelectedSceneID); err != nil {
			return
		}
		s.invalidateHistoryCache()
	}
}

// commitScenesMutation finalizes the active book, persists scenes to disk, reloads the book, and records a history step.
func (s *Server) commitScenesMutation(toast, kind, coalesceKey string) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return toast
	}
	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if toast != "" {
		s.recordHistoryAfterMutation(toast, kind, coalesceKey)
	}
	return toast
}

// withBookMutation wraps common editor preconditions (optional form scene flush, editable book check, history baseline)
// and commits scene changes when requested.
func (s *Server) withBookMutation(r *http.Request, flushFormScene bool, fn func(b *book.Book) (toast, kind, coalesceKey string, saveScenes bool)) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if flushFormScene {
		s.flushFormSceneIfPresent(r)
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	toast, kind, coalesceKey, saveScenes := fn(s.ActiveBook)
	if saveScenes {
		return s.commitScenesMutation(toast, kind, coalesceKey)
	}
	return toast
}

func (s *Server) reloadActiveBookAfterHistoryRestore(selSceneID string) {
	if s.ActiveBook == nil {
		return
	}
	s.invalidateHistoryCache()
	s.reloadActiveBook()
	if selSceneID != "" {
		if sc := s.findScene(selSceneID); sc != nil {
			s.SelectedSceneID = sc.ID
		}
	}
	if s.findScene(s.SelectedSceneID) == nil && len(s.ActiveBook.Scenes) > 0 {
		s.SelectedSceneID = s.ActiveBook.Scenes[0].ID
	}
	if s.SceneTextModalID != "" {
		if sc := s.findScene(s.SceneTextModalID); sc != nil {
			s.SceneTextModalID = sc.ID
		} else {
			s.SceneTextModalID = ""
		}
	}
}

func (s *Server) writeEditorHistoryHeaders(w http.ResponseWriter) {
	if s.isHistoryPreview() {
		w.Header().Set("X-History-Preview", "true")
	} else {
		w.Header().Set("X-History-Preview", "false")
	}
	h := s.loadActiveBookHistory()
	if h == nil {
		return
	}
	count := h.UnnamedEditsCount()
	canUndo := !s.isHistoryPreview() && h.CanUndo()
	canRedo := !s.isHistoryPreview() && h.CanRedo()
	w.Header().Set("X-History-Count", strconv.Itoa(count))
	if canUndo {
		w.Header().Set("X-Can-Undo", "true")
		curIdx := h.CursorIndex()
		if curIdx >= 0 && curIdx < len(h.Revisions) {
			w.Header().Set("X-Undo-Title", "Cofnij: "+h.Revisions[curIdx].Label+" (Ctrl+Z)")
		}
	} else {
		w.Header().Set("X-Can-Undo", "false")
		if s.isHistoryPreview() {
			w.Header().Set("X-Undo-Title", historyPreviewLockedToast)
		} else {
			w.Header().Set("X-Undo-Title", "Brak zmian do cofnięcia od ostatniej wersji (Ctrl+Z)")
		}
	}
	if canRedo {
		w.Header().Set("X-Can-Redo", "true")
		curIdx := h.CursorIndex()
		if curIdx >= 0 && curIdx+1 < len(h.Revisions) {
			w.Header().Set("X-Redo-Title", "Ponów: "+h.Revisions[curIdx+1].Label+" (Ctrl+Shift+Z)")
		}
	} else {
		w.Header().Set("X-Can-Redo", "false")
		if s.isHistoryPreview() {
			w.Header().Set("X-Redo-Title", historyPreviewLockedToast)
		} else {
			w.Header().Set("X-Redo-Title", "Brak zmian do ponowienia (Ctrl+Shift+Z)")
		}
	}
}

func (s *Server) undoEditor(_ *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		if selID, lbl, err := book.UndoBookHistory(s.ActiveBook); err == nil {
			s.reloadActiveBookAfterHistoryRestore(selID)
			return "Cofnięto: " + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func (s *Server) redoEditor(_ *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		if selID, lbl, err := book.RedoBookHistory(s.ActiveBook); err == nil {
			s.reloadActiveBookAfterHistoryRestore(selID)
			return "Ponowiono: " + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func (s *Server) toggleEditorHistoryPanel(r *http.Request) string {
	if r.FormValue("book_settings_open") == "false" {
		s.BookSettingsOpen = false
	}
	s.EditorHistoryPanelOpen = !s.EditorHistoryPanelOpen
	if s.EditorHistoryPanelOpen {
		s.activateStudioRightPanel("history")
		s.ensureHistoryBaselineBeforeMutation()
	}
	return ""
}

func (s *Server) openEditorHistoryPanel(_ *http.Request) string {
	s.activateStudioRightPanel("history")
	s.ensureHistoryBaselineBeforeMutation()
	return ""
}

func (s *Server) closeEditorHistoryPanel(_ *http.Request) string {
	s.EditorHistoryPanelOpen = false
	return ""
}

func (s *Server) saveHistoryVersion(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		verName := strings.TrimSpace(r.FormValue("version_name"))
		if verName == "" {
			verName = strings.TrimSpace(r.FormValue("name"))
		}
		if ver, err := book.SaveNamedBookVersion(s.ActiveBook, verName, s.SelectedSceneID); err == nil && ver != nil {
			s.invalidateHistoryCache()
			s.EditorHistoryPanelOpen = true
			return "Zapisano wersję: " + ver.Label
		} else if err != nil {
			return err.Error()
		}
	}
	return ""
}

func (s *Server) previewHistoryRevision(r *http.Request) string {
	s.flushFormSceneIfPresent(r)
	if s.ActiveBook == nil || s.ActiveBook.IsPacked || s.StandaloneMode {
		return ""
	}
	if !s.isHistoryPreview() {
		s.ensureHistoryBaselineBeforeMutation()
	}
	revID := ""
	if r != nil {
		revID = strings.TrimSpace(r.FormValue("revision"))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue("rev"))
		}
	}
	h := s.loadActiveBookHistory()
	if h == nil || len(h.Revisions) == 0 {
		return ""
	}
	if revID == "" || strings.EqualFold(revID, h.Cursor) {
		if s.isHistoryPreview() {
			s.exitHistoryPreview()
			curIdx := h.CursorIndex()
			if curIdx >= 0 && curIdx < len(h.Revisions) {
				return "Wrócono do aktualnej wersji: " + h.Revisions[curIdx].Label
			}
			return "Wrócono do aktualnej wersji"
		}
		return ""
	}

	baseBook := s.ActiveBook
	if s.LiveBook != nil {
		baseBook = s.LiveBook
	}
	previewBook, targetRev, err := book.LoadBookAtHistoryRevision(baseBook, revID)
	if err != nil {
		return err.Error()
	}
	if previewBook == nil || targetRev == nil {
		return ""
	}

	if !s.isHistoryPreview() {
		s.LiveBook = s.ActiveBook
		s.LivePlayerBook = s.PlayerBook
		s.LiveSave = s.Save
		s.LiveSelectedSceneID = s.SelectedSceneID
	}
	s.PreviewRevisionID = targetRev.ID
	s.PreviewRevisionLabel = targetRev.Label
	s.ActiveBook = previewBook
	s.PlayerBook = previewBook

	if targetRev.SelectedSceneID != "" {
		if sc := s.findScene(targetRev.SelectedSceneID); sc != nil {
			s.SelectedSceneID = sc.ID
		}
	}
	if s.findScene(s.SelectedSceneID) == nil {
		if previewBook.StartSceneID != "" {
			if sc := s.findScene(previewBook.StartSceneID); sc != nil {
				s.SelectedSceneID = sc.ID
			}
		}
	}
	if s.findScene(s.SelectedSceneID) == nil && len(previewBook.Scenes) > 0 {
		s.SelectedSceneID = previewBook.Scenes[0].ID
	}
	if s.SceneTextModalID != "" {
		if sc := s.findScene(s.SceneTextModalID); sc != nil {
			s.SceneTextModalID = sc.ID
		} else {
			s.SceneTextModalID = s.SelectedSceneID
		}
	}

	startSceneID := previewBook.StartSceneID
	if s.findScene(startSceneID) == nil && len(previewBook.Scenes) > 0 {
		startSceneID = previewBook.Scenes[0].ID
	}
	s.Save = &book.SaveState{
		BookID:             previewBook.ID,
		CurrentSceneID:     startSceneID,
		PlaybackSpeed:      s.Settings.PlaybackSpeed,
		AutoChoice:         s.Settings.AutoChoice,
		AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
		AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
	}
	s.HistoryRevealed = false
	s.FocusedHistoryIdx = -1
	return "Podgląd wersji: " + targetRev.Label + " (tylko do odczytu)"
}

func (s *Server) exitHistoryPreviewAction(_ *http.Request) string {
	if s.isHistoryPreview() {
		s.exitHistoryPreview()
		return "Wrócono do aktualnej wersji"
	}
	return ""
}

func (s *Server) playHistoryRevision(r *http.Request) string {
	if r != nil {
		revID := strings.TrimSpace(r.FormValue("revision"))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue("rev"))
		}
		if revID != "" {
			_ = s.previewHistoryRevision(r)
		}
	}
	if s.ActiveBook != nil {
		s.PlayerBook = s.ActiveBook
		if s.Save == nil {
			startSceneID := s.ActiveBook.StartSceneID
			if s.findScene(startSceneID) == nil && len(s.ActiveBook.Scenes) > 0 {
				startSceneID = s.ActiveBook.Scenes[0].ID
			}
			s.Save = &book.SaveState{
				BookID:             s.ActiveBook.ID,
				CurrentSceneID:     startSceneID,
				PlaybackSpeed:      s.Settings.PlaybackSpeed,
				AutoChoice:         s.Settings.AutoChoice,
				AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
				AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
			}
		} else if s.findScene(s.Save.CurrentSceneID) == nil && len(s.ActiveBook.Scenes) > 0 {
			s.Save.CurrentSceneID = s.ActiveBook.StartSceneID
			if s.findScene(s.Save.CurrentSceneID) == nil {
				s.Save.CurrentSceneID = s.ActiveBook.Scenes[0].ID
			}
		}
		s.Settings.ActiveView = "player"
		s.closeAllModals()
		s.persistSettings()
		if s.isHistoryPreview() && s.PreviewRevisionLabel != "" {
			return "Podgląd w odtwarzaczu: " + s.PreviewRevisionLabel
		}
	}
	return ""
}

func (s *Server) restoreHistoryRevision(r *http.Request) string {
	if r != nil && strings.TrimSpace(r.FormValue("target_branch")) != "" {
		return s.copyHistoryRevisionToBranchAction(r)
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		revID := strings.TrimSpace(r.FormValue("revision"))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue("rev"))
		}
		if revID == "" && s.PreviewRevisionID != "" {
			revID = s.PreviewRevisionID
		}
		if s.isHistoryPreview() {
			s.exitHistoryPreview()
		}
		if selID, lbl, err := book.RestoreBookHistoryRevision(s.ActiveBook, revID); err == nil {
			s.reloadActiveBookAfterHistoryRestore(selID)
			return "Przywrócono: " + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func (s *Server) copyHistoryRevisionToBranchAction(r *http.Request) string {
	s.flushFormSceneIfPresent(r)
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		revID := ""
		targetBranch := ""
		if r != nil {
			revID = strings.TrimSpace(r.FormValue("revision"))
			if revID == "" {
				revID = strings.TrimSpace(r.FormValue("rev"))
			}
			targetBranch = strings.TrimSpace(r.FormValue("target_branch"))
			if targetBranch == "" {
				targetBranch = strings.TrimSpace(r.FormValue("branch"))
			}
		}
		if revID == "" && s.PreviewRevisionID != "" {
			revID = s.PreviewRevisionID
		}
		if s.isHistoryPreview() {
			s.exitHistoryPreview()
		}
		if selID, lbl, err := book.CopyHistoryRevisionToBranch(s.ActiveBook, revID, targetBranch); err == nil {
			s.reloadActiveBookAfterHistoryRestore(selID)
			s.EditorHistoryPanelOpen = true
			return "Nałożono wersję: " + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func (s *Server) promptCreateHistoryBranchAction(r *http.Request) string {
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	rawBranch := ""
	if r != nil {
		rawBranch = strings.TrimSpace(r.FormValue("branch_name"))
		if rawBranch == "" {
			rawBranch = strings.TrimSpace(r.FormValue("branch"))
		}
	}
	cleanBranch := ""
	if rawBranch != "" {
		cleanBranch = book.SanitizeBookID(rawBranch)
	}
	h := s.loadActiveBookHistory()
	if cleanBranch == "" || cleanBranch == "book" {
		startSeq := 2
		if h != nil {
			startSeq = len(h.Branches) + 1
		}
		for seq := startSeq; ; seq++ {
			cand := "wariant-" + strconv.Itoa(seq)
			exists := false
			if h != nil {
				for _, br := range h.Branches {
					if strings.EqualFold(br.Name, cand) {
						exists = true
						break
					}
				}
			}
			if !exists {
				cleanBranch = cand
				break
			}
		}
	}
	if h != nil {
		for _, br := range h.Branches {
			if strings.EqualFold(br.Name, cleanBranch) {
				return "Gałąź „" + cleanBranch + "” już istnieje"
			}
		}
	}
	s.ConfirmTitle = "Utwórz nowy wariant"
	s.ConfirmMessage = "Czy na pewno chcesz utworzyć nowy wariant „" + cleanBranch + "”? Nowy tor jest trwałą częścią historii projektu."
	s.ConfirmPendingAction = "CreateHistoryBranch"
	s.ConfirmPendingParam = cleanBranch
	return ""
}

func (s *Server) createHistoryBranchByName(branchName, fromRev string) string {
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	if fromRev == "" && s.PreviewRevisionID != "" {
		fromRev = s.PreviewRevisionID
	}
	if s.isHistoryPreview() {
		s.exitHistoryPreview()
	}
	if br, err := book.CreateBookBranch(s.ActiveBook, branchName, fromRev); err == nil && br != nil {
		s.invalidateHistoryCache()
		s.reloadActiveBook()
		s.EditorHistoryPanelOpen = true
		return "Utworzono i przełączono na gałąź: " + br.Name
	} else if err != nil {
		return err.Error()
	}
	return ""
}

func (s *Server) createHistoryBranchAction(r *http.Request) string {
	branchName := strings.TrimSpace(r.FormValue("branch_name"))
	if branchName == "" {
		branchName = strings.TrimSpace(r.FormValue("branch"))
	}
	fromRev := strings.TrimSpace(r.FormValue("from_revision"))
	return s.createHistoryBranchByName(branchName, fromRev)
}

func (s *Server) switchHistoryBranchAction(r *http.Request) string {
	if s.isHistoryPreview() {
		s.exitHistoryPreview()
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		branchName := strings.TrimSpace(r.FormValue("branch"))
		if branchName == "" {
			branchName = strings.TrimSpace(r.FormValue("branch_name"))
		}
		if selID, err := book.SwitchBookBranch(s.ActiveBook, branchName); err == nil {
			s.reloadActiveBookAfterHistoryRestore(selID)
			return "Przełączono na gałąź: " + book.SanitizeBookID(branchName)
		} else {
			return err.Error()
		}
	}
	return ""
}
