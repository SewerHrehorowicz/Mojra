//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	commonactions "mojra/domain/common/actions"
	commonstate "mojra/domain/common/state"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/player/state"
	"mojra/domain/player/views"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

const (
	paramKeyZen  = "zen"
	boolValTrue  = "true"
	boolValFalse = "false"
	boolValOn    = "on"
	boolValOff   = "off"
)

type ActionHandler func(s *state.PlayerState, req *http.Request) []transport.ActionFrame

func playerFromUI(ui *commonstate.UIState) *state.PlayerState {
	if ui != nil && ui.Hooks.PlayerState != nil {
		if ps, ok := ui.Hooks.PlayerState.(*state.PlayerState); ok {
			return ps
		}
	}
	return nil
}

func registerStandaloneActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		fn := handler
		commonactions.RegisterAction(actionName, true, true, func(ui *commonstate.UIState, req *http.Request) []transport.ActionFrame {
			ps := playerFromUI(ui)
			if ps == nil {
				return nil
			}
			return fn(ps, req)
		})
	}
	return true
}

func registerLANActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		fn := handler
		commonactions.RegisterAction(actionName, true, false, func(ui *commonstate.UIState, req *http.Request) []transport.ActionFrame {
			ps := playerFromUI(ui)
			if ps == nil {
				return nil
			}
			return fn(ps, req)
		})
	}
	return true
}

var (
	_ = registerStandaloneActions(map[string]ActionHandler{
		"SaveAudioTime":       saveAudioTime,
		"ChooseBranch":        chooseBranch,
		"AutoAdvanceImplicit": autoAdvanceImplicit,
		"AudioEnded":          audioEnded,
		"RevealHistory":       revealHistory,
		"FocusHistoryStep":    focusHistoryStep,
		"FocusPresent":        focusPresent,
		"CycleAudioSpeed":     cycleAudioSpeed,
		"ToggleAutoChoice":    toggleAutoChoice,
		"SetAutoChoiceDelay":  setAutoChoiceDelay,
		"PromptRewindStep":    promptRewindStep,
		"PromptResetProgress": promptResetProgress,
	})
	_ = registerLANActions(map[string]ActionHandler{
		"ToggleZenMode": toggleZenMode,
	})
)

func renderPlayerWorkspace(s *state.PlayerState, toast string) []transport.ActionFrame {
	s.Overlays.Toast = toast
	if s.Confirm.IsOpen() {
		return renderPlayerModals(s, toast)
	}
	if s.Hooks.RenderWorkspaceFrames != nil {
		return s.Hooks.RenderWorkspaceFrames()
	}
	return []transport.ActionFrame{
		{Target: commonstate.TargetWorkspace, Chunks: []templates.TemplateChunk{views.PlayerView(s)}},
	}
}

func renderPlayerModals(s *state.PlayerState, toast string) []transport.ActionFrame {
	s.Overlays.Toast = toast
	if s.Hooks.RenderModalsFrame != nil {
		return []transport.ActionFrame{s.Hooks.RenderModalsFrame()}
	}
	return []transport.ActionFrame{
		{Target: commonstate.TargetModalsRoot, Chunks: []templates.TemplateChunk{commonviews.ConfirmView(s.UIState)}},
	}
}

func saveAudioTime(s *state.PlayerState, req *http.Request) []transport.ActionFrame {
	if s.Book != nil && s.Save != nil {
		if timeVal, err := strconv.ParseFloat(req.FormValue(formKeyTime), 64); err == nil && timeVal >= 0 {
			s.Save.AudioTime = timeVal
			if !isHistoryPreview(s) {
				if err := book.UpdateSaveAudioTimeInPlace(s.Config.SavesDir, s.Book.ID, timeVal, s.Book.StartSceneID); err != nil {
					s.PersistSave(s.Config.SavesDir)
				}
			}
		}
	}
	return nil
}

func chooseBranch(s *state.PlayerState, req *http.Request) []transport.ActionFrame {
	idx, err := strconv.Atoi(req.FormValue(formKeyChoiceIdx))
	cur := s.Book.CurrentScene(s.Save)
	toast := ""
	if err == nil && cur != nil && idx >= 0 && idx < len(cur.Choices) {
		toast = ExecutePlayerChoiceTransition(s, cur, cur.Choices[idx])
	}
	return renderPlayerWorkspace(s, toast)
}

func autoAdvanceImplicit(s *state.PlayerState, _ *http.Request) []transport.ActionFrame {
	cur := s.Book.CurrentScene(s.Save)
	toast := ""
	if cur != nil && len(cur.Choices) == 1 && !cur.Choices[0].IsEnding && cur.Choices[0].TargetSceneID != formValEND {
		toast = ExecutePlayerChoiceTransition(s, cur, cur.Choices[0])
	}
	return renderPlayerWorkspace(s, toast)
}

func audioEnded(s *state.PlayerState, _ *http.Request) []transport.ActionFrame {
	return renderPlayerWorkspace(s, ExecutePlayerAudioEnded(s))
}

func revealHistory(s *state.PlayerState, _ *http.Request) []transport.ActionFrame {
	if s.Save != nil && len(s.Save.History) > 0 {
		s.HistoryRevealed = true
		s.FocusedHistoryIdx = len(s.Save.History) - 1
	}
	return renderPlayerWorkspace(s, "")
}

func focusHistoryStep(s *state.PlayerState, req *http.Request) []transport.ActionFrame {
	if stepIdx, err := strconv.Atoi(req.FormValue(formKeyStep)); err == nil && s.Save != nil && stepIdx >= 0 && stepIdx < len(s.Save.History) {
		s.HistoryRevealed = true
		s.FocusedHistoryIdx = stepIdx
	}
	return renderPlayerWorkspace(s, "")
}

func focusPresent(s *state.PlayerState, _ *http.Request) []transport.ActionFrame {
	s.FocusedHistoryIdx = -1
	s.HistoryRevealed = false
	return renderPlayerWorkspace(s, "")
}

func cycleAudioSpeed(s *state.PlayerState, _ *http.Request) []transport.ActionFrame {
	CyclePlayerAudioSpeed(s)
	if s.Hooks.RenderToastFrames != nil {
		return s.Hooks.RenderToastFrames("")
	}
	return commonviews.ToastFrames(s.UIState, "")
}

func toggleAutoChoice(s *state.PlayerState, _ *http.Request) []transport.ActionFrame {
	TogglePlayerAutoChoice(s)
	if s.Hooks.RenderToastFrames != nil {
		return s.Hooks.RenderToastFrames("")
	}
	return commonviews.ToastFrames(s.UIState, "")
}

func setAutoChoiceDelay(s *state.PlayerState, req *http.Request) []transport.ActionFrame {
	SetPlayerAutoChoiceDelay(s, req.FormValue(formKeyDelay))
	if s.Hooks.RenderToastFrames != nil {
		return s.Hooks.RenderToastFrames("")
	}
	return commonviews.ToastFrames(s.UIState, "")
}

func promptRewindStep(s *state.PlayerState, req *http.Request) []transport.ActionFrame {
	PromptPlayerRewindStep(s, req.FormValue(formKeyStep))
	if s.Confirm.IsOpen() {
		param := s.Confirm.PendingParam
		s.Confirm.OnAccept = func() []transport.ActionFrame {
			toast := RewindPlayerToStep(s, param)
			return renderPlayerWorkspace(s, toast)
		}
	}
	return renderPlayerModals(s, "")
}

func promptResetProgress(s *state.PlayerState, req *http.Request) []transport.ActionFrame {
	targetID := ""
	if req != nil {
		targetID = book.SanitizeBookID(req.FormValue("book"))
	}
	targetTitle := targetID
	if s.Book != nil && (targetID == "" || targetID == s.Book.ID) {
		targetID = s.Book.ID
		targetTitle = s.Book.Title
	}
	if targetID != "" {
		var onAccept func() []transport.ActionFrame
		if s.Hooks.ExecutePendingConfirm == nil {
			onAccept = func() []transport.ActionFrame {
				toast := ResetPlayerBookProgress(s)
				return renderPlayerWorkspace(s, toast)
			}
		}
		s.Confirm.Prompt(commonstate.ConfirmState{
			Title:         tr(s, i18n.TrConfirmResetTitle),
			Message:       trf(s, i18n.TrConfirmResetMsgFmt, targetTitle),
			PendingAction: PendingResetProgress,
			PendingParam:  targetID,
			Danger:        false,
			AllowLAN:      true,
			OnAccept:      onAccept,
		})
	}
	return renderPlayerModals(s, "")
}

func toggleZenMode(s *state.PlayerState, req *http.Request) []transport.ActionFrame {
	if s.Settings.ActiveView != string(commonstate.ViewPlayer) || s.Book == nil || s.Book.IsScreenplay() {
		s.ZenMode = false
		return commonviews.ToastFrames(s.UIState, "")
	}
	switch strings.ToLower(strings.TrimSpace(req.FormValue(paramKeyZen))) {
	case boolValTrue, "1", boolValOn:
		s.ZenMode = true
	case boolValFalse, "0", boolValOff:
		s.ZenMode = false
	default:
		s.ZenMode = !s.ZenMode
	}
	return commonviews.ToastFrames(s.UIState, "")
}
