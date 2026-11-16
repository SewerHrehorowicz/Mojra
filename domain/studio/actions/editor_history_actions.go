//go:build !(js && wasm)

package actions

import (
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/transport"
)

const (
	formKeyVersionName  = "version_name"
	formKeyRevision     = "revision"
	formKeyRev          = "rev"
	formKeyTargetBranch = "target_branch"
	formKeyFromRevision = "from_revision"
	formKeyBranch       = "branch"
	formKeyBranchName   = "branch_name"
	CreateHistoryBranch = "CreateHistoryBranch"
	branchAliasBook     = "book"
	branchDefaultPrefix = "wariant-"
)

var _ = registerActions(map[string]ActionHandler{
	"UndoEditor":                  undoEditor,
	"RedoEditor":                  redoEditor,
	"PreviewHistoryRevision":      previewHistoryRevision,
	"ExitHistoryPreview":          exitHistoryPreviewAction,
	"PlayHistoryRevision":         playHistoryRevision,
	"RestoreHistoryRevision":      restoreHistoryRevision,
	"CopyHistoryRevisionToBranch": copyHistoryRevisionToBranchAction,
	"ToggleEditorHistoryPanel":    toggleEditorHistoryPanel,
	"OpenEditorHistoryPanel":      openEditorHistoryPanel,
	"CloseEditorHistoryPanel":     closeEditorHistoryPanel,
	"SaveHistoryVersion":          saveHistoryVersion,
	"CreateHistoryBranch":         createHistoryBranchAction,
	"PromptCreateHistoryBranch":   promptCreateHistoryBranchAction,
	"SwitchHistoryBranch":         switchHistoryBranchAction,
})

func ExitStudioHistoryPreview(s *state.StudioState) {
	exitHistoryPreview(s)
}

func exitHistoryPreview(s *state.StudioState) {
	if !isHistoryPreview(s) {
		return
	}
	savedSceneID := s.Editor.History.ExitPreview(s.Config.BooksDir, s.Settings, &s.Editor.ActiveBook)
	if s.Hooks.ExitHistoryPreview != nil {
		s.Hooks.ExitHistoryPreview()
	}
	b := s.Editor.ActiveBook
	if b != nil {
		if s.Hooks.OnBookMutated != nil {
			s.Hooks.OnBookMutated(b)
		}
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

func reloadActiveBook(s *state.StudioState) {
	if s.Editor.ActiveBook == nil {
		return
	}
	reloaded := s.Editor.ActiveBook.ReloadFromDisk(s.Config.BooksDir, s.Settings)
	s.Editor.ActiveBook = reloaded
	if s.Hooks.OnBookMutated != nil {
		s.Hooks.OnBookMutated(reloaded)
	}
}

func commitScenesMutation(s *state.StudioState, toast, kind, coalesceKey string) string {
	if s == nil || s.Hooks.UnmaskStudioInt == nil || s.Hooks.UnmaskStudioInt(0x53^1, 16) != 1 {
		return ""
	}
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

func withBookMutation(s *state.StudioState, r *transport.ActionRequest, flushFormScene bool, fn func(b *book.Book) (toast, kind, coalesceKey string, saveScenes bool)) string {
	if s == nil || s.Hooks.UnmaskStudioInt == nil || s.Hooks.UnmaskStudioInt(0x53^1, 16) != 1 {
		return ""
	}
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	if flushFormScene {
		_ = flushFormSceneIfPresent(s, r)
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

func reloadActiveBookAfterHistoryRestore(s *state.StudioState, selSceneID string) {
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

func undoEditor(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	_ = flushFormSceneIfPresent(s, r)
	toast := ""
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		if selID, lbl, err := book.UndoBookHistory(s.Editor.ActiveBook); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			toast = tr(s, i18n.TrUndonePrefix) + lbl
		} else {
			toast = err.Error()
		}
	}
	return frameUndoRedo(s, toast)
}

func redoEditor(s *state.StudioState, _ *transport.ActionRequest) []transport.ActionFrame {
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	toast := ""
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		if selID, lbl, err := book.RedoBookHistory(s.Editor.ActiveBook); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			toast = tr(s, i18n.TrRedonePrefix) + lbl
		} else {
			toast = err.Error()
		}
	}
	return frameUndoRedo(s, toast)
}

func toggleEditorHistoryPanel(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if s.Editor.RightPanel == state.PanelHistory {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		activateRightPanel(s, state.PanelHistory)
		s.Editor.History.EnsureBaseline(s.Editor.ActiveBook, s.Editor.SelectedSceneID)
	}
	return frameWorkspace(s, "")
}

func openEditorHistoryPanel(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	activateRightPanel(s, state.PanelHistory)
	s.Editor.History.EnsureBaseline(s.Editor.ActiveBook, s.Editor.SelectedSceneID)
	return frameWorkspace(s, "")
}

func closeEditorHistoryPanel(s *state.StudioState, _ *transport.ActionRequest) []transport.ActionFrame {
	if s.Editor.RightPanel == state.PanelHistory {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return frameWorkspace(s, "")
}

func saveHistoryVersion(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if isHistoryPreview(s) {
		return frameWorkspace(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil && !b.IsPacked {
		verName := strings.TrimSpace(r.FormValue(formKeyVersionName))
		if verName == "" {
			verName = strings.TrimSpace(r.FormValue(formKeyName))
		}
		if ver, err := book.SaveNamedBookVersion(b, verName, s.Editor.SelectedSceneID); err == nil && ver != nil {
			s.Editor.History.InvalidateCache()
			activateRightPanel(s, state.PanelHistory)
			toast = tr(s, i18n.TrSavedVersionPrefix) + ver.Label
		} else if err != nil {
			toast = err.Error()
		}
	}
	return frameWorkspace(s, toast)
}

func previewHistoryRevisionToast(s *state.StudioState, r *transport.ActionRequest) string {
	_ = flushFormSceneIfPresent(s, r)
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked || s.Config.StandaloneMode {
		return ""
	}
	if !isHistoryPreview(s) {
		s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	}
	revID := ""
	if r != nil {
		revID = strings.TrimSpace(r.FormValue(formKeyRevision))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue(formKeyRev))
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
				return tr(s, i18n.TrReturnedToCurrentPrefix) + h.Revisions[curIdx].Label
			}
			return tr(s, i18n.TrReturnedToCurrent)
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
		return restoreHistoryRevisionToast(s, r)
	}

	if !isHistoryPreview(s) {
		h.Preview = &book.HistoryPreview{
			LiveBook:        b,
			SelectedSceneID: s.Editor.SelectedSceneID,
		}
	}
	h.Preview.RevisionID = targetRev.ID
	h.Preview.RevisionLabel = targetRev.Label
	s.Editor.ActiveBook = previewBook

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
	previewSave := &book.SaveState{
		BookID:             previewBook.ID,
		CurrentSceneID:     startSceneID,
		PlaybackSpeed:      s.Settings.PlaybackSpeed,
		AutoChoice:         s.Settings.AutoChoice,
		AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
		AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
	}
	if s.Hooks.PlayBookWithSave != nil {
		s.Hooks.PlayBookWithSave(previewBook, previewSave, true, targetRev.Label)
	}
	return trf(s, i18n.TrPreviewVersionReadonlyFmt, targetRev.Label)
}

func previewHistoryRevision(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	return frameUndoRedo(s, previewHistoryRevisionToast(s, r))
}

func exitHistoryPreviewAction(s *state.StudioState, _ *transport.ActionRequest) []transport.ActionFrame {
	toast := ""
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
		toast = tr(s, i18n.TrReturnedToCurrent)
	}
	return frameUndoRedo(s, toast)
}

func playHistoryRevision(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if r != nil {
		revID := strings.TrimSpace(r.FormValue(formKeyRevision))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue(formKeyRev))
		}
		if revID != "" {
			_ = previewHistoryRevisionToast(s, r)
		}
	}
	toast := ""
	b := s.Editor.ActiveBook
	if b != nil {
		histLabel := ""
		if isHistoryPreview(s) && s.Editor.History.Preview != nil {
			histLabel = s.Editor.History.Preview.RevisionLabel
		}
		if s.Hooks.PlayBookWithSave != nil {
			s.Hooks.PlayBookWithSave(b, nil, isHistoryPreview(s), histLabel)
		}
		s.Settings.ActiveView = string(commonstate.ViewPlayer)
		s.CloseTransientViews()
		s.Editor.SceneText.Close()
		s.Export.Reset()
		s.SaveSettings()
		if histLabel != "" {
			toast = tr(s, i18n.TrPreviewInPlayerPrefix) + histLabel
		}
	}
	return frameApp(s, toast)
}

func restoreHistoryRevisionToast(s *state.StudioState, r *transport.ActionRequest) string {
	if r != nil && strings.TrimSpace(r.FormValue(formKeyTargetBranch)) != "" {
		return copyHistoryRevisionToBranchToast(s, r)
	}
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		revID := strings.TrimSpace(r.FormValue(formKeyRevision))
		if revID == "" {
			revID = strings.TrimSpace(r.FormValue(formKeyRev))
		}
		if revID == "" && s.Editor.History.Preview != nil {
			revID = s.Editor.History.Preview.RevisionID
		}
		if isHistoryPreview(s) {
			exitHistoryPreview(s)
		}
		if selID, lbl, err := book.RestoreBookHistoryRevision(s.Editor.ActiveBook, revID); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			return tr(s, i18n.TrRestoredVersionPrefix) + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func restoreHistoryRevision(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	return frameUndoRedo(s, restoreHistoryRevisionToast(s, r))
}

func copyHistoryRevisionToBranchToast(s *state.StudioState, r *transport.ActionRequest) string {
	_ = flushFormSceneIfPresent(s, r)
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		revID := ""
		targetBranch := ""
		if r != nil {
			revID = strings.TrimSpace(r.FormValue(formKeyRevision))
			if revID == "" {
				revID = strings.TrimSpace(r.FormValue(formKeyRev))
			}
			targetBranch = strings.TrimSpace(r.FormValue(formKeyTargetBranch))
			if targetBranch == "" {
				targetBranch = strings.TrimSpace(r.FormValue(formKeyBranch))
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
			return tr(s, i18n.TrAppliedVersionPrefix) + lbl
		} else {
			return err.Error()
		}
	}
	return ""
}

func copyHistoryRevisionToBranchAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	return frameUndoRedo(s, copyHistoryRevisionToBranchToast(s, r))
}

func createHistoryBranchByName(s *state.StudioState, branchName, fromRev string) string {
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
		return tr(s, i18n.TrCreatedSwitchedBranchPrefix) + br.Name
	} else if err != nil {
		return err.Error()
	}
	return ""
}

func createHistoryBranchAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	branchName := strings.TrimSpace(r.FormValue(formKeyBranchName))
	if branchName == "" {
		branchName = strings.TrimSpace(r.FormValue(formKeyBranch))
	}
	fromRev := strings.TrimSpace(r.FormValue(formKeyFromRevision))
	return frameWorkspace(s, createHistoryBranchByName(s, branchName, fromRev))
}

func switchHistoryBranchAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	toast := ""
	if s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked {
		branchName := strings.TrimSpace(r.FormValue(formKeyBranch))
		if branchName == "" {
			branchName = strings.TrimSpace(r.FormValue(formKeyBranchName))
		}
		if selID, err := book.SwitchBookBranch(s.Editor.ActiveBook, branchName); err == nil {
			reloadActiveBookAfterHistoryRestore(s, selID)
			toast = tr(s, i18n.TrSwitchedBranchPrefix) + book.SanitizeBookID(branchName)
		} else {
			toast = err.Error()
		}
	}
	return frameAppResetCanvas(s, toast)
}

func promptCreateHistoryBranchAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameModals(s, "")
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	rawBranch := ""
	if r != nil {
		rawBranch = strings.TrimSpace(r.FormValue(formKeyBranchName))
		if rawBranch == "" {
			rawBranch = strings.TrimSpace(r.FormValue(formKeyBranch))
		}
	}
	cleanBranch := ""
	if rawBranch != "" {
		cleanBranch = book.SanitizeBookID(rawBranch)
	}
	h := s.Editor.History.RefreshFromDisk(b)
	if cleanBranch == "" || cleanBranch == branchAliasBook {
		startSeq := 2
		if h != nil {
			startSeq = len(h.Branches) + 1
		}
		for seq := startSeq; ; seq++ {
			cand := branchDefaultPrefix + strconv.Itoa(seq)
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
				return frameModals(s, trf(s, i18n.TrBranchAlreadyExistsFmt, cleanBranch))
			}
		}
	}
	s.Confirm.Prompt(commonstate.ConfirmState{
		Title:         tr(s, i18n.TrCreateBranchTitle),
		Message:       trf(s, i18n.TrCreateBranchConfirmFmt, cleanBranch),
		PendingAction: CreateHistoryBranch,
		PendingParam:  cleanBranch,
		Danger:        false,
		OnAccept: func() []transport.ActionFrame {
			toast := createHistoryBranchByName(s, cleanBranch, "")
			return frameApp(s, toast)
		},
	})
	return frameModals(s, "")
}
