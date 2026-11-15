//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/domain/views"
	"mojra/engine"
	"mojra/engine/osbridge"
	"mojra/engine/templates"
)

const (
	UnpackPlaybookForStudio = "UnpackPlaybookForStudio"
	pendingRewindStep       = "RewindStep"
	pendingResetProgress    = "ResetProgress"
	pendingRemoveBook       = "RemoveBook"
	pendingDeleteScene      = "DeleteScene"
	pendingDeleteScenes     = "DeleteScenes"
	pendingCreateBranch     = "CreateHistoryBranch"
	pendingRemoveChoice     = "RemoveChoice"
	pendingQuitApp          = "QuitApp"

	formKeyBranchName   = "branch_name"
	formKeyBranch       = "branch"
	branchAliasBook     = "book"
	branchDefaultPrefix = "wariant-"
	quotedNameFmt       = "„%s”"
	pendingParamFmt     = "%s:%d"
)

var (
	_ = registerLANActions(map[string]ActionHandler{
		"PromptRewindStep":    promptRewindStep,
		"PromptResetProgress": promptResetProgress,
		"ConfirmModalClose":   closeConfirmModal,
	})
	_ = registerStandaloneActions(map[string]ActionHandler{
		"ConfirmModalAccept": acceptConfirmModalAction,
	})
	_ = registerActions(map[string]ActionHandler{
		"PromptDeleteScene":         promptDeleteScene,
		"PromptDeleteScenes":        promptDeleteScenesFromForm,
		"PromptRemoveChoice":        promptRemoveChoiceFromScene,
		"PromptCreateHistoryBranch": promptCreateHistoryBranchAction,
	})
)

func closeConfirmModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Confirm.Reset()
	return frameModals(s, "")
}

func acceptConfirmModalAction(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	wasOpen := s.Confirm.IsOpen()
	pending := s.Confirm.PendingAction
	toast, nativeCmd := acceptConfirmModal(s)
	s.Overlays.Toast = toast
	s.Overlays.NativeCmd = nativeCmd
	if wasOpen && !s.Confirm.IsOpen() && pending == pendingRemoveChoice && s.Editor.SceneText.IsOpen() {
		return []engine.ActionFrame{
			{Target: targetSelModals, Chunks: views.ActiveOverlayViews(s)},
			{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}},
		}
	}
	return frameApp(s, toast)
}

func acceptConfirmModal(s *state.AppState) (toast, nativeCmd string) {
	pending := s.Confirm.PendingAction
	param := s.Confirm.PendingParam
	s.Confirm.Reset()

	switch pending {
	case pendingRewindStep:
		if stepIdx, err := strconv.Atoi(param); err == nil && s.Player.Save != nil && stepIdx >= 0 && stepIdx < len(s.Player.Save.History) {
			targetSceneID := s.Player.Save.History[stepIdx].SceneID
			if len(s.Player.Save.History[stepIdx].Variables) > 0 {
				s.Player.Save.Variables = book.CloneVariablesMap(s.Player.Save.History[stepIdx].Variables)
			} else if s.Player.Book != nil {
				s.Player.Save.Variables = book.DefaultVariablesMap(s.Player.Book)
				if targetSc := s.Player.Book.FindScene(targetSceneID); targetSc != nil && targetSc.Effects != "" {
					book.ApplyEffects(targetSc.Effects, s.Player.Save.Variables, s.Player.Book)
				}
			}
			s.Player.Save.History = s.Player.Save.History[:stepIdx]
			s.Player.Save.CurrentSceneID = targetSceneID
			s.Player.Save.AudioTime = 0
			s.Player.HistoryRevealed = false
			s.Player.FocusedHistoryIdx = -1
			if !isHistoryPreview(s) {
				s.Player.PersistSave(s.Config.SavesDir)
			}
			toast = tr(s, i18n.TrRewoundStoryToast)
		}
	case pendingResetProgress:
		targetID := param
		if targetID == "" && s.Player.Book != nil {
			targetID = s.Player.Book.ID
		}
		if targetID != "" {
			if isHistoryPreview(s) && s.Player.Book != nil && s.Player.Book.ID == targetID {
				startSceneID := s.Player.Book.StartSceneID
				if s.Player.Book.FindScene(startSceneID) == nil && len(s.Player.Book.Scenes) > 0 {
					startSceneID = s.Player.Book.Scenes[0].ID
				}
				s.Player.Save = &book.SaveState{
					BookID:             s.Player.Book.ID,
					CurrentSceneID:     startSceneID,
					PlaybackSpeed:      s.Settings.PlaybackSpeed,
					AutoChoice:         s.Settings.AutoChoice,
					AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
					AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
				}
				s.Player.HistoryRevealed = false
				s.Player.FocusedHistoryIdx = -1
			} else {
				_ = book.DeleteSaveState(s.Config.SavesDir, targetID)
				if s.Player.Book != nil && s.Player.Book.ID == targetID {
					s.Player.Save = book.LoadSaveState(s.Config.SavesDir, s.Player.Book.ID, s.Player.Book.StartSceneID)
					s.Player.HistoryRevealed = false
					s.Player.FocusedHistoryIdx = -1
				}
			}
			toast = tr(s, i18n.TrResetProgressToast)
		}
	case UnpackPlaybookForStudio:
		toast = unpackPlaybookWorkingCopyForStudio(s, param)
	case pendingRemoveBook:
		toast = removeBookByID(s, param)
	case pendingDeleteScene:
		toast = deleteSceneByID(s, param)
	case pendingDeleteScenes:
		toast = deleteScenesByIDs(s, param)
	case pendingCreateBranch:
		toast = createHistoryBranchByName(s, param, "")
	case pendingRemoveChoice:
		if sep := strings.LastIndex(param, ":"); sep > 0 {
			scID := param[:sep]
			if choiceIdx, err := strconv.Atoi(param[sep+1:]); err == nil {
				toast = removeChoiceBySceneAndIndex(s, scID, choiceIdx)
			}
		}
	case pendingQuitApp:
		nativeCmd = nativeCmdQuit
		go func() {
			s.Config.TriggerShutdown()
			osbridge.TerminateNativeWindow()
		}()
	}
	return toast, nativeCmd
}

func promptRewindStep(s *state.AppState, r *http.Request) []engine.ActionFrame {
	if stepIdx, err := strconv.Atoi(r.FormValue(formKeyStep)); err == nil && s.Player.Save != nil && stepIdx >= 0 && stepIdx < len(s.Player.Save.History) {
		entry := s.Player.Save.History[stepIdx]
		sceneName := entry.SceneName
		if sc := s.Player.Book.FindScene(entry.SceneID); sc != nil {
			sceneName = sc.DisplayName()
		}
		if sceneName == "" {
			sceneName = entry.SceneID
		}
		s.Confirm.Prompt(state.ConfirmState{
			Title:         tr(s, i18n.TrRewindSceneHint),
			Message:       trf(s, i18n.TrConfirmRewindMsgFmt, sceneName),
			PendingAction: pendingRewindStep,
			PendingParam:  strconv.Itoa(stepIdx),
			Danger:        false,
		})
	}
	return frameModals(s, "")
}

func promptResetProgress(s *state.AppState, r *http.Request) []engine.ActionFrame {
	targetBook := s.Player.Book
	if reqBookID := strings.TrimSpace(r.FormValue(formKeyBook)); reqBookID != "" {
		if found := s.Library.Get(book.SanitizeBookID(reqBookID)); found != nil {
			targetBook = found
		}
	}
	if targetBook != nil {
		s.Confirm.Prompt(state.ConfirmState{
			Title:         tr(s, i18n.TrConfirmResetTitle),
			Message:       trf(s, i18n.TrConfirmResetMsgFmt, targetBook.Title),
			PendingAction: pendingResetProgress,
			PendingParam:  targetBook.ID,
			Danger:        false,
		})
	}
	return frameModals(s, "")
}

func promptDeleteScene(s *state.AppState, r *http.Request) []engine.ActionFrame {
	if isHistoryPreview(s) {
		return frameModals(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return frameModals(s, "")
	}
	targetID := strings.TrimSpace(r.FormValue(formKeyScene))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue(formKeySceneID))
	}
	if targetID == "" {
		targetID = s.Editor.SelectedSceneID
	}
	if sc := b.FindScene(targetID); sc != nil {
		s.Confirm.Prompt(state.ConfirmState{
			Title:         tr(s, i18n.TrDeleteSceneBtn),
			Message:       trf(s, i18n.TrDeleteSceneConfirmFmt, sc.DisplayName(), sc.SourceFile),
			PendingAction: pendingDeleteScene,
			PendingParam:  sc.ID,
			Danger:        true,
		})
	}
	return frameModals(s, "")
}

func promptDeleteScenesFromForm(s *state.AppState, r *http.Request) []engine.ActionFrame {
	if isHistoryPreview(s) {
		return frameModals(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return frameModals(s, "")
	}
	rawIDs := strings.TrimSpace(r.FormValue(formKeyScenes))
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeySceneIDs))
	}
	if rawIDs == "" {
		rawIDs = strings.TrimSpace(r.FormValue(formKeyScene))
	}
	var matched []*book.Scene
	seen := make(map[string]bool)
	for _, part := range strings.Split(rawIDs, ",") {
		idPart := strings.TrimSpace(part)
		if idPart == "" {
			continue
		}
		if sc := b.FindScene(idPart); sc != nil && !seen[sc.ID] {
			seen[sc.ID] = true
			matched = append(matched, sc)
		}
	}
	if len(matched) == 1 {
		sc := matched[0]
		s.Confirm.Prompt(state.ConfirmState{
			Title:         tr(s, i18n.TrDeleteSceneBtn),
			Message:       trf(s, i18n.TrDeleteSceneConfirmFmt, sc.DisplayName(), sc.SourceFile),
			PendingAction: pendingDeleteScene,
			PendingParam:  sc.ID,
			Danger:        true,
		})
	} else if len(matched) > 1 {
		var ids []string
		var names []string
		for _, sc := range matched {
			ids = append(ids, sc.ID)
			names = append(names, fmt.Sprintf(quotedNameFmt, sc.DisplayName()))
		}
		s.Confirm.Prompt(state.ConfirmState{
			Title:         trf(s, i18n.TrDeleteScenesTitleFmt, len(matched)),
			Message:       trf(s, i18n.TrDeleteScenesConfirmFmt, len(matched), strings.Join(names, ", ")),
			PendingAction: pendingDeleteScenes,
			PendingParam:  strings.Join(ids, ","),
			Danger:        true,
		})
	}
	return frameModals(s, "")
}

func promptRemoveChoiceFromScene(s *state.AppState, r *http.Request) []engine.ActionFrame {
	if isHistoryPreview(s) {
		return frameModals(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	syncSceneTextSettingsOpen(s, r)
	b := s.Editor.ActiveBook
	if b == nil {
		return frameModals(s, "")
	}
	scID, target := resolveEditorSceneFromRequest(s, r, true)
	if target == nil {
		return frameModals(s, "")
	}
	choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyChoiceIdx)))
	if err != nil {
		return frameModals(s, "")
	}

	if s.Editor.SceneText.IsOpen() {
		if saved, _ := flushSceneTextChanges(s, r); saved {
			target = s.Editor.ActiveBook.FindScene(s.Editor.SceneText.SceneID)
		}
	} else if !b.IsPacked {
		_ = flushFormSceneIfPresent(s, r)
		target = s.Editor.ActiveBook.FindScene(scID)
		if target == nil {
			target = s.Editor.ActiveBook.FindScene(s.Editor.SelectedSceneID)
		}
	}
	if target == nil || choiceIdx < 0 || choiceIdx >= len(target.Choices) {
		return frameModals(s, "")
	}
	chLabel := strings.TrimSpace(target.Choices[choiceIdx].Label)
	if chLabel == "" {
		chLabel = trf(s, i18n.TrChoiceDefaultFmt, choiceIdx+1)
	}
	s.Confirm.Prompt(state.ConfirmState{
		Title:         tr(s, i18n.TrDeleteChoiceTitle),
		Message:       trf(s, i18n.TrDeleteChoiceConfirmFmt, chLabel, target.DisplayName()),
		PendingAction: pendingRemoveChoice,
		PendingParam:  fmt.Sprintf(pendingParamFmt, target.ID, choiceIdx),
		Danger:        true,
	})
	return frameModals(s, "")
}

func promptCreateHistoryBranchAction(s *state.AppState, r *http.Request) []engine.ActionFrame {
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
	s.Confirm.Prompt(state.ConfirmState{
		Title:         tr(s, i18n.TrCreateBranchTitle),
		Message:       trf(s, i18n.TrCreateBranchConfirmFmt, cleanBranch),
		PendingAction: pendingCreateBranch,
		PendingParam:  cleanBranch,
		Danger:        false,
	})
	return frameModals(s, "")
}
