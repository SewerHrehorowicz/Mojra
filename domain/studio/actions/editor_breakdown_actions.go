//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine"
	"mojra/engine/transport"
)

const (
	formKeyOldName     = "old_name"
	formKeyNewName     = "new_name"
	formKeyOldLocation = "old_location"
	formKeyLocation    = "location"
	formKeyNewLocation = "new_location"
)

var _ = registerActions(map[string]ActionHandler{
	"ToggleEditorBreakdownPanel": toggleEditorBreakdownPanel,
	"CloseEditorBreakdownPanel":  closeEditorBreakdownPanel,
	"RenameCharacter":            renameCharacterAction,
	"RenameLocation":             renameLocationAction,
})

func toggleEditorBreakdownPanel(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if s.Editor.RightPanel == state.PanelBreakdown {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		activateRightPanel(s, state.PanelBreakdown)
	}
	return frameWorkspace(s, "")
}

func closeEditorBreakdownPanel(s *state.StudioState, _ *http.Request) []transport.ActionFrame {
	if s.Editor.RightPanel == state.PanelBreakdown {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return frameWorkspace(s, "")
}

func renameScreenplayEntityAction(s *state.StudioState, oldVal, newVal string, emptyKey, okFmtKey engine.TrKey, fn func(*book.Book, string, string) (int, error)) string {
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
		return tr(s, emptyKey)
	}
	return trf(s, okFmtKey, strings.ToUpper(oldVal), strings.ToUpper(newVal), count)
}

func renameCharacterAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	oldName := strings.TrimSpace(r.FormValue(formKeyOldName))
	if oldName == "" {
		oldName = strings.TrimSpace(r.FormValue(formKeyCharacter))
	}
	newName := strings.TrimSpace(r.FormValue(formKeyNewName))
	return frameUndoRedo(s, renameScreenplayEntityAction(s, oldName, newName, i18n.TrNoCuesToRename, i18n.TrRenamedCharacterFmt, book.RenameScreenplayCharacter))
}

func renameLocationAction(s *state.StudioState, r *http.Request) []transport.ActionFrame {
	oldLoc := strings.TrimSpace(r.FormValue(formKeyOldLocation))
	if oldLoc == "" {
		oldLoc = strings.TrimSpace(r.FormValue(formKeyLocation))
	}
	newLoc := strings.TrimSpace(r.FormValue(formKeyNewLocation))
	return frameUndoRedo(s, renameScreenplayEntityAction(s, oldLoc, newLoc, i18n.TrNoLocationsToRename, i18n.TrRenamedLocationFmt, book.RenameScreenplayLocation))
}
