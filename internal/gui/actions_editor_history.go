package gui

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"storyteller/internal/book"
)

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
	if s.ActiveBook.RootPath != "" {
		if reloaded, err := book.LoadBookFromPath(s.ActiveBook.RootPath); err == nil && reloaded != nil {
			reloaded.IsExternal = prevExt
			if prevID != "" && reloaded.ID != prevID {
				book.RebindBookID(reloaded, prevID)
			}
			s.ActiveBook = reloaded
			return
		}
	}
	if reloaded, err := s.findBookByID(prevID); err == nil && reloaded != nil {
		s.ActiveBook = reloaded
	}
}

func (s *Server) ensureHistoryBaselineBeforeMutation() {
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked && !s.StandaloneMode {
		_ = book.EnsureBookHistoryBaseline(s.ActiveBook, s.SelectedSceneID)
		s.invalidateHistoryCache()
	}
}

func (s *Server) recordHistoryAfterMutation(label, kind, coalesceKey string) {
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked && !s.StandaloneMode {
		_ = book.RecordBookHistoryStep(s.ActiveBook, label, kind, coalesceKey, s.SelectedSceneID)
		s.invalidateHistoryCache()
	}
}

// commitScenesMutation finalizes the active book, persists scenes to disk, reloads the book, and records a history step.
func (s *Server) commitScenesMutation(toast, kind, coalesceKey string) string {
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
	if flushFormScene && r != nil && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
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
	h := s.loadActiveBookHistory()
	if h == nil {
		return
	}
	count := h.UnnamedEditsCount()
	canUndo := h.CanUndo()
	canRedo := h.CanRedo()
	w.Header().Set("X-History-Count", strconv.Itoa(count))
	if canUndo {
		w.Header().Set("X-Can-Undo", "true")
		curIdx := h.CursorIndex()
		if curIdx >= 0 && curIdx < len(h.Revisions) {
			w.Header().Set("X-Undo-Title", "Cofnij: "+h.Revisions[curIdx].Label+" (Ctrl+Z)")
		}
	} else {
		w.Header().Set("X-Can-Undo", "false")
		w.Header().Set("X-Undo-Title", "Brak zmian do cofnięcia od ostatniej wersji (Ctrl+Z)")
	}
	if canRedo {
		w.Header().Set("X-Can-Redo", "true")
		curIdx := h.CursorIndex()
		if curIdx >= 0 && curIdx+1 < len(h.Revisions) {
			w.Header().Set("X-Redo-Title", "Ponów: "+h.Revisions[curIdx+1].Label+" (Ctrl+Shift+Z)")
		}
	} else {
		w.Header().Set("X-Can-Redo", "false")
		w.Header().Set("X-Redo-Title", "Brak zmian do ponowienia (Ctrl+Shift+Z)")
	}
}

func (s *Server) handleEditorHistoryAction(action string, r *http.Request) (toast string, handled bool) {
	switch action {
	case "UndoEditor":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
			if selID, lbl, err := book.UndoBookHistory(s.ActiveBook); err == nil {
				s.reloadActiveBookAfterHistoryRestore(selID)
				toast = "Cofnięto: " + lbl
			} else {
				toast = err.Error()
			}
		}
		return toast, true

	case "RedoEditor":
		if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
			if selID, lbl, err := book.RedoBookHistory(s.ActiveBook); err == nil {
				s.reloadActiveBookAfterHistoryRestore(selID)
				toast = "Ponowiono: " + lbl
			} else {
				toast = err.Error()
			}
		}
		return toast, true

	case "ToggleEditorHistoryPanel":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		s.EditorHistoryPanelOpen = !s.EditorHistoryPanelOpen
		if s.EditorHistoryPanelOpen {
			s.ensureHistoryBaselineBeforeMutation()
		}
		return "", true

	case "OpenEditorHistoryPanel":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		s.EditorHistoryPanelOpen = true
		s.ensureHistoryBaselineBeforeMutation()
		return "", true

	case "CloseEditorHistoryPanel":
		s.EditorHistoryPanelOpen = false
		return "", true

	case "SaveHistoryVersion":
		if r.FormValue("scene_id") != "" {
			_ = s.saveSceneFromForm(r)
		}
		if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
			verName := strings.TrimSpace(r.FormValue("version_name"))
			if verName == "" {
				verName = strings.TrimSpace(r.FormValue("name"))
			}
			if ver, err := book.SaveNamedBookVersion(s.ActiveBook, verName, s.SelectedSceneID); err == nil && ver != nil {
				s.invalidateHistoryCache()
				s.EditorHistoryPanelOpen = true
				toast = "Zapisano wersję: " + ver.Label
			} else if err != nil {
				toast = err.Error()
			}
		}
		return toast, true

	case "RestoreHistoryRevision":
		if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
			revID := strings.TrimSpace(r.FormValue("revision"))
			if revID == "" {
				revID = strings.TrimSpace(r.FormValue("rev"))
			}
			if selID, lbl, err := book.RestoreBookHistoryRevision(s.ActiveBook, revID); err == nil {
				s.reloadActiveBookAfterHistoryRestore(selID)
				toast = "Przywrócono: " + lbl
			} else {
				toast = err.Error()
			}
		}
		return toast, true
	}
	return "", false
}
