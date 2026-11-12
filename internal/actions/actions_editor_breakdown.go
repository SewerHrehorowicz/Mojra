//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func toggleEditorBreakdownPanel(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelBreakdown {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		activateRightPanel(s, state.PanelBreakdown)
	}
	return ""
}

func closeEditorBreakdownPanel(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelBreakdown {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return ""
}

func renameScreenplayEntityAction(s *state.AppState, oldVal, newVal, emptyMsg, okFmt string, fn func(*book.Book, string, string) (int, error)) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	count, err := fn(b, oldVal, newVal)
	if err != nil {
		return err.Error()
	}
	s.Editor.History.InvalidateCache()
	reloadActiveBook(s)
	if count == 0 {
		return emptyMsg
	}
	return fmt.Sprintf(okFmt, strings.ToUpper(oldVal), strings.ToUpper(newVal), count)
}

func renameCharacterAction(s *state.AppState, r *http.Request) string {
	oldName := strings.TrimSpace(r.FormValue("old_name"))
	if oldName == "" {
		oldName = strings.TrimSpace(r.FormValue("character"))
	}
	newName := strings.TrimSpace(r.FormValue("new_name"))
	return renameScreenplayEntityAction(s, oldName, newName, "Brak kwestii do zmiany", "Zmieniono postać %s → %s (%d kwestii)", book.RenameScreenplayCharacter)
}

func renameLocationAction(s *state.AppState, r *http.Request) string {
	oldLoc := strings.TrimSpace(r.FormValue("old_location"))
	if oldLoc == "" {
		oldLoc = strings.TrimSpace(r.FormValue("location"))
	}
	newLoc := strings.TrimSpace(r.FormValue("new_location"))
	return renameScreenplayEntityAction(s, oldLoc, newLoc, "Brak lokacji do zmiany", "Zmieniono lokację %s → %s (%d scen)", book.RenameScreenplayLocation)
}
