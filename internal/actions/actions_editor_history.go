//go:build !(js && wasm)

package actions

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func isHistoryPreview(s *state.AppState) bool {
	return s != nil && !s.Config.StandaloneMode && s.Editor.History.IsPreview(s.Editor.ActiveBook)
}

func exitHistoryPreview(s *state.AppState) {
	if !isHistoryPreview(s) {
		return
	}
	savedSceneID := s.Editor.History.ExitPreview(s.Config.BooksDir, s.Settings, &s.Editor.ActiveBook)
	s.Player.ExitHistoryPreview(s.Config.SavesDir, s.Editor.ActiveBook)
	b := s.Editor.ActiveBook
	if b != nil {
		s.Library.Put(b)
		if savedSceneID != "" && b.FindScene(savedSceneID) != nil {
			s.Editor.SelectedSceneID = savedSceneID
		} else if b.FindScene(s.Editor.SelectedSceneID) == nil && len(b.Scenes) > 0 {
			s.Editor.SelectedSceneID = b.Scenes[0].ID
		}
		if s.Editor.SceneText.IsOpen() && b.FindScene(s.Editor.SceneText.SceneID) == nil {
			s.Editor.SceneText.SceneID = s.Editor.SelectedSceneID
		}
	}
}

func reloadActiveBook(s *state.AppState) {
	if s.Editor.ActiveBook == nil {
		return
	}
	reloaded := s.Editor.ActiveBook.ReloadFromDisk(s.Config.BooksDir, s.Settings)
	s.Editor.ActiveBook = reloaded
	s.Library.Put(reloaded)
	if !isHistoryPreview(s) && s.Player.Book != nil && s.Player.Book.ID == reloaded.ID {
		s.Player.Book = reloaded
	}
}

// WriteEditorHistoryHeaders writes the X-Scene-* and X-History-* HTTP response headers for the Studio editor.
func WriteEditorHistoryHeaders(w http.ResponseWriter, s *state.AppState) {
	b := s.Editor.ActiveBook
	if b != nil {
		if s.Editor.SelectedSceneID != "" {
			w.Header().Set("X-Scene-ID", s.Editor.SelectedSceneID)
			if s.Editor.LastRenamedFromSceneID != "" && !strings.EqualFold(s.Editor.LastRenamedFromSceneID, s.Editor.SelectedSceneID) {
				w.Header().Set("X-Scene-Prev-ID", s.Editor.LastRenamedFromSceneID)
			}
			s.Editor.LastRenamedFromSceneID = ""
			if sc := b.FindScene(s.Editor.SelectedSceneID); sc != nil {
				w.Header().Set("X-Scene-Explicit-ID", sc.ExplicitID)
				w.Header().Set("X-Scene-Ref", url.QueryEscape(sc.ReferenceKey()))
				w.Header().Set("X-Scene-Title", url.QueryEscape(sc.DisplayName()))
				w.Header().Set("X-Scene-Chapter", url.QueryEscape(sc.Chapter))
			}
		}
		if b.StartSceneID != "" {
			w.Header().Set("X-Start-Scene-ID", b.StartSceneID)
		}
	}
	histPreview := isHistoryPreview(s)
	if histPreview {
		w.Header().Set("X-History-Preview", "true")
	} else {
		w.Header().Set("X-History-Preview", "false")
	}
	if s.Config.StandaloneMode {
		return
	}
	h := s.Editor.History.RefreshFromDisk(b)
	if h == nil {
		return
	}
	count := h.UnnamedEditsCount()
	canUndo := !histPreview && h.CanUndo()
	canRedo := !histPreview && h.CanRedo()
	w.Header().Set("X-History-Count", strconv.Itoa(count))
	if canUndo {
		w.Header().Set("X-Can-Undo", "true")
		curIdx := h.CursorIndex()
		if curIdx >= 0 && curIdx < len(h.Revisions) {
			w.Header().Set("X-Undo-Title", "Cofnij: "+h.Revisions[curIdx].Label+" (Ctrl+Z)")
		}
	} else {
		w.Header().Set("X-Can-Undo", "false")
		if histPreview {
			w.Header().Set("X-Undo-Title", tr(s, i18n.TrHistoryPreviewLocked))
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
		if histPreview {
			w.Header().Set("X-Redo-Title", tr(s, i18n.TrHistoryPreviewLocked))
		} else {
			w.Header().Set("X-Redo-Title", "Brak zmian do ponowienia (Ctrl+Shift+Z)")
		}
	}
}

func commitScenesMutation(s *state.AppState, toast, kind, coalesceKey string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return toast
	}
	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	if toast != "" {
		s.Editor.History.RecordStep(s.Editor.ActiveBook, toast, kind, coalesceKey, s.Editor.SelectedSceneID)
	}
	return toast
}

func withBookMutation(s *state.AppState, r *http.Request, flushFormScene bool, fn func(b *book.Book) (toast, kind, coalesceKey string, saveScenes bool)) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if flushFormScene {
		flushFormSceneIfPresent(s, r)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	toast, kind, coalesceKey, saveScenes := fn(b)
	if saveScenes {
		return commitScenesMutation(s, toast, kind, coalesceKey)
	}
	return toast
}

func reloadActiveBookAfterHistoryRestore(s *state.AppState, selSceneID string) {
	if s.Editor.ActiveBook == nil {
		return
	}
	s.Editor.History.InvalidateCache()
	reloadActiveBook(s)
	b := s.Editor.ActiveBook
	if selSceneID != "" {
		if sc := b.FindScene(selSceneID); sc != nil {
			s.Editor.SelectedSceneID = sc.ID
		}
	}
	if b.FindScene(s.Editor.SelectedSceneID) == nil && len(b.Scenes) > 0 {
		s.Editor.SelectedSceneID = b.Scenes[0].ID
	}
	if s.Editor.SceneText.IsOpen() {
		if sc := b.FindScene(s.Editor.SceneText.SceneID); sc != nil {
			s.Editor.SceneText.SceneID = sc.ID
		} else {
			s.Editor.SceneText.Close()
		}
	}
}

func undoEditor(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	flushFormSceneIfPresent(s, r)
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		if selID, lbl, err := book.UndoBookHistory(s.Editor.ActiveBook); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			return "Cofnięto: " + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func redoEditor(s *state.AppState, _ *http.Request) string {
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		if selID, lbl, err := book.RedoBookHistory(s.Editor.ActiveBook); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			return "Ponowiono: " + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func toggleEditorHistoryPanel(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelHistory {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		activateRightPanel(s, state.PanelHistory)
		s.Editor.History.EnsureBaseline(s.Editor.ActiveBook, s.Editor.SelectedSceneID)
	}
	return ""
}

func openEditorHistoryPanel(s *state.AppState, _ *http.Request) string {
	activateRightPanel(s, state.PanelHistory)
	s.Editor.History.EnsureBaseline(s.Editor.ActiveBook, s.Editor.SelectedSceneID)
	return ""
}

func closeEditorHistoryPanel(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelHistory {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return ""
}

func saveHistoryVersion(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		verName := strings.TrimSpace(r.FormValue("version_name"))
		if verName == "" {
			verName = strings.TrimSpace(r.FormValue("name"))
		}
		if ver, err := book.SaveNamedBookVersion(b, verName, s.Editor.SelectedSceneID); err == nil && ver != nil {
			s.Editor.History.InvalidateCache()
			activateRightPanel(s, state.PanelHistory)
			return "Zapisano wersję: " + ver.Label
		} else if err != nil {
			return err.Error()
		}
	}
	return ""
}

func previewHistoryRevision(s *state.AppState, r *http.Request) string {
	flushFormSceneIfPresent(s, r)
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked || s.Config.StandaloneMode {
		return ""
	}
	if !isHistoryPreview(s) {
		s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	}
	revID := ""
	if r != nil {
		revID = strings.TrimSpace(r.FormValue("revision"))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue("rev"))
		}
	}
	h := s.Editor.History.RefreshFromDisk(b)
	if h == nil || len(h.Revisions) == 0 {
		return ""
	}
	if revID == "" || strings.EqualFold(revID, h.Cursor) {
		if isHistoryPreview(s) {
			exitHistoryPreview(s)
			curIdx := h.CursorIndex()
			if curIdx >= 0 && curIdx < len(h.Revisions) {
				return "Wrócono do aktualnej wersji: " + h.Revisions[curIdx].Label
			}
			return "Wrócono do aktualnej wersji"
		}
		return ""
	}

	baseBook := b
	if h.Preview != nil && h.Preview.LiveBook != nil {
		baseBook = h.Preview.LiveBook
	}
	previewBook, targetRev, err := book.LoadBookAtHistoryRevision(baseBook, revID)
	if err != nil {
		return err.Error()
	}
	if previewBook == nil || targetRev == nil {
		return ""
	}
	if !targetRev.Named && strings.EqualFold(targetRev.Branch, h.NormalizedActiveBranch()) {
		return restoreHistoryRevision(s, r)
	}

	if !isHistoryPreview(s) {
		h.Preview = &book.HistoryPreview{
			LiveBook:        b,
			SelectedSceneID: s.Editor.SelectedSceneID,
		}
		s.Player.LiveBook = s.Player.Book
		s.Player.LiveSave = s.Player.Save
	}
	h.Preview.RevisionID = targetRev.ID
	h.Preview.RevisionLabel = targetRev.Label
	s.Editor.ActiveBook = previewBook
	s.Player.Book = previewBook

	if targetRev.SelectedSceneID != "" {
		if sc := previewBook.FindScene(targetRev.SelectedSceneID); sc != nil {
			s.Editor.SelectedSceneID = sc.ID
		}
	}
	if previewBook.FindScene(s.Editor.SelectedSceneID) == nil {
		if previewBook.StartSceneID != "" {
			if sc := previewBook.FindScene(previewBook.StartSceneID); sc != nil {
				s.Editor.SelectedSceneID = sc.ID
			}
		}
	}
	if previewBook.FindScene(s.Editor.SelectedSceneID) == nil && len(previewBook.Scenes) > 0 {
		s.Editor.SelectedSceneID = previewBook.Scenes[0].ID
	}
	if s.Editor.SceneText.IsOpen() {
		if sc := previewBook.FindScene(s.Editor.SceneText.SceneID); sc != nil {
			s.Editor.SceneText.SceneID = sc.ID
		} else {
			s.Editor.SceneText.SceneID = s.Editor.SelectedSceneID
		}
	}

	startSceneID := previewBook.StartSceneID
	if previewBook.FindScene(startSceneID) == nil && len(previewBook.Scenes) > 0 {
		startSceneID = previewBook.Scenes[0].ID
	}
	s.Player.Save = &book.SaveState{
		BookID:             previewBook.ID,
		CurrentSceneID:     startSceneID,
		PlaybackSpeed:      s.Settings.PlaybackSpeed,
		AutoChoice:         s.Settings.AutoChoice,
		AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
		AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
	}
	s.Player.HistoryRevealed = false
	s.Player.FocusedHistoryIdx = -1
	return "Podgląd wersji: " + targetRev.Label + " (tylko do odczytu)"
}

func exitHistoryPreviewAction(s *state.AppState, _ *http.Request) string {
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
		return "Wrócono do aktualnej wersji"
	}
	return ""
}

func playHistoryRevision(s *state.AppState, r *http.Request) string {
	if r != nil {
		revID := strings.TrimSpace(r.FormValue("revision"))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue("rev"))
		}
		if revID != "" {
			_ = previewHistoryRevision(s, r)
		}
	}
	b := s.Editor.ActiveBook
	if b != nil {
		s.Player.Book = b
		if s.Player.Save == nil {
			startSceneID := b.StartSceneID
			if b.FindScene(startSceneID) == nil && len(b.Scenes) > 0 {
				startSceneID = b.Scenes[0].ID
			}
			s.Player.Save = &book.SaveState{
				BookID:             b.ID,
				CurrentSceneID:     startSceneID,
				PlaybackSpeed:      s.Settings.PlaybackSpeed,
				AutoChoice:         s.Settings.AutoChoice,
				AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
				AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
			}
		} else if b.FindScene(s.Player.Save.CurrentSceneID) == nil && len(b.Scenes) > 0 {
			s.Player.Save.CurrentSceneID = b.StartSceneID
			if b.FindScene(s.Player.Save.CurrentSceneID) == nil {
				s.Player.Save.CurrentSceneID = b.Scenes[0].ID
			}
		}
		s.Settings.ActiveView = state.ViewPlayer.String()
		s.CloseAllModals()
		_ = s.SaveSettings()
		if isHistoryPreview(s) && s.Editor.History.Preview != nil && s.Editor.History.Preview.RevisionLabel != "" {
			return "Podgląd w odtwarzaczu: " + s.Editor.History.Preview.RevisionLabel
		}
	}
	return ""
}

func restoreHistoryRevision(s *state.AppState, r *http.Request) string {
	if r != nil && strings.TrimSpace(r.FormValue("target_branch")) != "" {
		return copyHistoryRevisionToBranchAction(s, r)
	}
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		revID := strings.TrimSpace(r.FormValue("revision"))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue("rev"))
		}
		if revID == "" && s.Editor.History.Preview != nil {
			revID = s.Editor.History.Preview.RevisionID
		}
		if isHistoryPreview(s) {
			exitHistoryPreview(s)
		}
		if selID, lbl, err := book.RestoreBookHistoryRevision(s.Editor.ActiveBook, revID); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			return "Przywrócono: " + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func copyHistoryRevisionToBranchAction(s *state.AppState, r *http.Request) string {
	flushFormSceneIfPresent(s, r)
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
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
		if revID == "" && s.Editor.History.Preview != nil {
			revID = s.Editor.History.Preview.RevisionID
		}
		if isHistoryPreview(s) {
			exitHistoryPreview(s)
		}
		if selID, lbl, err := book.CopyHistoryRevisionToBranch(s.Editor.ActiveBook, revID, targetBranch); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			activateRightPanel(s, state.PanelHistory)
			return "Nałożono wersję: " + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func createHistoryBranchByName(s *state.AppState, branchName, fromRev string) string {
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return ""
	}
	if fromRev == "" && s.Editor.History.Preview != nil {
		fromRev = s.Editor.History.Preview.RevisionID
	}
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	if br, err := book.CreateBookBranch(s.Editor.ActiveBook, branchName, fromRev); err == nil && br != nil {
		s.Editor.History.InvalidateCache()
		reloadActiveBook(s)
		activateRightPanel(s, state.PanelHistory)
		return "Utworzono i przełączono na gałąź: " + br.Name
	} else if err != nil {
		return err.Error()
	}
	return ""
}

func createHistoryBranchAction(s *state.AppState, r *http.Request) string {
	branchName := strings.TrimSpace(r.FormValue("branch_name"))
	if branchName == "" {
		branchName = strings.TrimSpace(r.FormValue("branch"))
	}
	fromRev := strings.TrimSpace(r.FormValue("from_revision"))
	return createHistoryBranchByName(s, branchName, fromRev)
}

func switchHistoryBranchAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		branchName := strings.TrimSpace(r.FormValue("branch"))
		if branchName == "" {
			branchName = strings.TrimSpace(r.FormValue("branch_name"))
		}
		if selID, err := book.SwitchBookBranch(s.Editor.ActiveBook, branchName); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			return "Przełączono na gałąź: " + book.SanitizeBookID(branchName)
		} else {
			return err.Error()
		}
	}
	return ""
}
