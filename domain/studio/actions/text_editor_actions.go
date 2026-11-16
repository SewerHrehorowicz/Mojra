//go:build !(js && wasm)

package actions

import (
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	"mojra/domain/studio/state"
	"mojra/engine/transport"
)

const (
	formKeyScene             = "scene"
	formKeySceneID           = "scene_id"
	formKeyTargetScene       = "target_scene"
	formKeySceneSettingsOpen = "scene_settings_open"
	formKeyMode              = "mode"
	formKeyWidth             = "width"
	formKeyVarName           = "var_name"
	formKeyVarValue          = "var_value"
	formKeyVarVal            = "var_val"
	formKeyDelta             = "delta"
	formKeyToggle            = "toggle"
	formKeyStep              = "step"
	formValTrue              = "true"

	defaultVoiceSectionKey = "01"
	histKindText           = "text"
	coalesceTextModalPref  = "textmodal:"
)

var (
	_ = registerActions(map[string]ActionHandler{
		"OpenSceneTextModal":         openSceneTextModal,
		"CloseSceneTextModal":        closeSceneTextModal,
		"ToggleSceneTextSettings":    toggleSceneTextSettings,
		"NavigateSceneTextModal":     navigateSceneTextModal,
		"ToggleChoiceRule":           toggleChoiceRuleAction,
		"ToggleSyntaxHelp":           toggleSyntaxHelpAction,
		"SetSceneTextPreviewMode":    setSceneTextPreviewModeAction,
		"SetSceneTextWidthMode":      setSceneTextWidthModeAction,
		"ToggleSceneTestVarsPopover": toggleSceneTestVarsPopoverAction,
		"UpdateSceneTextTestVar":     updateSceneTextTestVarAction,
	})
	_ = registerStandaloneActions(map[string]ActionHandler{
		"SaveSceneTextModal": saveSceneTextModal,
	})
)

func syncSceneTextSettingsOpen(s *state.StudioState, r *transport.ActionRequest) {
	if r != nil {
		if _, ok := r.Form[formKeySceneSettingsOpen]; ok {
			s.Editor.SceneText.SettingsOpen = r.FormValue(formKeySceneSettingsOpen) == formValTrue
		}
	}
}

func resolveSceneTextTarget(s *state.StudioState, r *transport.ActionRequest) *book.Scene {
	b := s.Editor.ActiveBook
	if b == nil {
		return nil
	}
	for _, cand := range []string{
		r.FormValue(formKeyScene),
		r.FormValue(formKeySceneID),
		s.Editor.SceneText.SceneID,
		s.Editor.SelectedSceneID,
		b.StartSceneID,
	} {
		if id := strings.TrimSpace(cand); id != "" {
			if sc := b.FindScene(id); sc != nil {
				return sc
			}
		}
	}
	if len(b.Scenes) > 0 {
		return b.Scenes[0]
	}
	return nil
}

func flushSceneTextChanges(s *state.StudioState, r *transport.ActionRequest) (saved bool, errToast string) {
	if !s.Editor.SceneText.IsOpen() {
		return false, ""
	}
	return commitSceneForm(s, r, sceneFlushTextModal)
}

func openSceneTextModal(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	hadFormScene := flushFormSceneIfPresent(s, r)
	sc := resolveSceneTextTarget(s, r)
	if sc != nil {
		if s.Editor.SelectedSceneID != sc.ID || s.Editor.SceneText.SceneID != sc.ID {
			s.Editor.SelectedVoiceSectionKey = defaultVoiceSectionKey
		}
		if s.Editor.SelectedSceneID != sc.ID {
			s.Editor.SceneText.SettingsOpen = false
		}
		s.Editor.SelectedSceneID = sc.ID
		s.Editor.SceneText.Open(sc.ID)
	}
	if hadFormScene {
		return frameApp(s, "")
	}
	return frameModals(s, "")
}

func setSceneTextWidthModeAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	raw := r.FormValue(formKeyMode)
	if raw == "" {
		raw = r.FormValue(formKeyWidth)
	}
	s.Editor.SceneText.WidthMode = state.ParseSceneWidthMode(raw)
	return frameModals(s, "")
}

func closeSceneTextModal(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_, errToast := flushSceneTextChanges(s, r)
	if r != nil && r.FormValue(formKeyStep) == formValTrue && !s.Editor.SceneText.StepCollapse() {
		s.Overlays.Toast = errToast
		return frameModals(s, errToast)
	}
	s.Editor.SceneText.Close()
	s.Editor.SelectedVoiceSectionKey = defaultVoiceSectionKey
	s.Overlays.Toast = errToast
	if s.Settings.ActiveView != string(commonstate.ViewEditor) || s.Config.StandaloneMode {
		return frameModals(s, errToast)
	}
	return frameApp(s, errToast)
}

func toggleSceneTextSettings(s *state.StudioState, _ *transport.ActionRequest) []transport.ActionFrame {
	s.Editor.SceneText.SettingsOpen = !s.Editor.SceneText.SettingsOpen
	return frameModals(s, "")
}

func saveSceneTextModal(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	syncSceneTextSettingsOpen(s, r)
	_, toast := commitSceneForm(s, r, sceneSaveTextModal)
	return frameModals(s, toast)
}

func navigateSceneTextModal(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	s.Editor.SceneText.SettingsOpen = false
	_, errToast := flushSceneTextChanges(s, r)
	b := s.Editor.ActiveBook
	if b == nil {
		return frameModals(s, errToast)
	}
	curSc := b.FindScene(s.Editor.SceneText.SceneID)
	targetID := strings.TrimSpace(r.FormValue(formKeyScene))
	if targetID == "" {
		targetID = strings.TrimSpace(r.FormValue(formKeyTargetScene))
	}
	var chosenEffects string
	if choiceIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyChoiceIdx))); err == nil && curSc != nil {
		targetID, chosenEffects = curSc.ResolveChoiceNavigation(choiceIdx, targetID, b.StartSceneID)
	}
	if strings.EqualFold(targetID, book.TargetEnd) {
		targetID = b.StartSceneID
	}
	nextSc := b.FindScene(targetID)
	if nextSc == nil && len(b.Scenes) > 0 && targetID == "" {
		nextSc = b.Scenes[0]
	}
	if nextSc != nil {
		s.Editor.SceneText.ApplyNavigationEffects(b, chosenEffects, nextSc.Effects)
		s.Editor.SelectedSceneID = nextSc.ID
		s.Editor.SceneText.SyncSceneID(nextSc.ID)
		s.Editor.SelectedVoiceSectionKey = defaultVoiceSectionKey
	}
	return frameModals(s, errToast)
}

func toggleChoiceRuleAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	hadFormScene := flushFormSceneIfPresent(s, r)
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.ResetRulesUI()
		return frameSceneForm(s, "", hadFormScene, false)
	}
	sc := resolveSceneTextTarget(s, r)
	idxStr := strings.TrimSpace(r.FormValue(formKeyChoiceIdx))
	if sc == nil || idxStr == "" {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	hasRule := false
	if chIdx, err := strconv.Atoi(idxStr); err == nil && chIdx >= 0 && chIdx < len(sc.Choices) {
		ch := sc.Choices[chIdx]
		hasRule = strings.TrimSpace(ch.Condition) != "" || strings.TrimSpace(ch.Effects) != ""
	}
	s.Editor.SceneText.ToggleChoiceRule(sc.ID, idxStr, hasRule)
	return frameSceneForm(s, "", hadFormScene, false)
}

func toggleSyntaxHelpAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	hadFormScene := flushFormSceneIfPresent(s, r)
	if s.Editor.ActiveBook == nil || len(s.Editor.ActiveBook.Variables) == 0 {
		s.Editor.SceneText.SyntaxHelpOpen = false
		return frameSceneForm(s, "", hadFormScene, false)
	}
	s.Editor.SceneText.SyntaxHelpOpen = !s.Editor.SceneText.SyntaxHelpOpen
	return frameSceneForm(s, "", hadFormScene, false)
}

func setSceneTextPreviewModeAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.PreviewMode = state.PreviewModeNormal
		return frameModals(s, "")
	}
	if raw := strings.TrimSpace(r.FormValue(formKeyMode)); raw != "" {
		s.Editor.SceneText.PreviewMode = state.ParseScenePreviewMode(raw)
	} else {
		s.Editor.SceneText.TogglePreviewMode()
	}
	if s.Editor.SceneText.PreviewMode != state.PreviewModeNormal {
		s.Editor.SceneText.TestVars = book.EnsureSaveVariables(b, s.Editor.SceneText.TestVars)
	}
	return frameModals(s, "")
}

func toggleSceneTestVarsPopoverAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.TestVarsOpen = false
		return frameModals(s, "")
	}
	s.Editor.SceneText.TestVarsOpen = !s.Editor.SceneText.TestVarsOpen
	if s.Editor.SceneText.TestVarsOpen {
		s.Editor.SceneText.TestVars = book.EnsureSaveVariables(b, s.Editor.SceneText.TestVars)
	}
	return frameModals(s, "")
}

func updateSceneTextTestVarAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	b := s.Editor.ActiveBook
	varName := book.SanitizeVarName(r.FormValue(formKeyVarName))
	if varName == "" || b == nil {
		return frameModals(s, "")
	}
	val := strings.TrimSpace(r.FormValue(formKeyVarValue))
	if val == "" {
		val = strings.TrimSpace(r.FormValue(formKeyVarVal))
	}
	s.Editor.SceneText.TestVars = book.UpdateTestVariable(
		s.Editor.SceneText.TestVars,
		b,
		varName,
		val,
		r.FormValue(formKeyDelta),
		r.FormValue(formKeyToggle) == formValTrue,
	)
	return frameModals(s, "")
}
