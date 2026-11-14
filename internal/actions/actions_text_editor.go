//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

const (
	formKeyScene             = "scene"
	formKeySceneID           = "scene_id"
	formKeyTargetScene       = "target_scene"
	formKeySceneSettingsOpen = "scene_settings_open"
	formKeyChoiceIdx         = "choice_idx"
	formKeyMode              = "mode"
	formKeyWidth             = "width"
	formKeyVarName           = "var_name"
	formKeyVarValue          = "var_value"
	formKeyVarVal            = "var_val"
	formKeyDelta             = "delta"
	formKeyToggle            = "toggle"
	formValTrue              = "true"

	defaultVoiceSectionKey = "01"
	histKindText           = "text"
	coalesceTextModalPref  = "textmodal:"
)

func syncSceneTextSettingsOpen(s *state.AppState, r *http.Request) {
	if r != nil {
		if _, ok := r.Form[formKeySceneSettingsOpen]; ok {
			s.Editor.SceneText.SettingsOpen = r.FormValue(formKeySceneSettingsOpen) == formValTrue
		}
	}
}

func resolveSceneTextTarget(s *state.AppState, r *http.Request) *book.Scene {
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

func flushSceneTextChanges(s *state.AppState, r *http.Request) (saved bool, errToast string) {
	if isHistoryPreview(s) || !s.Editor.SceneText.IsOpen() {
		return false, ""
	}
	b := s.Editor.ActiveBook
	curID := strings.TrimSpace(r.FormValue(formKeySceneID))
	if curID == "" {
		curID = s.Editor.SceneText.SceneID
	}
	curSc := b.FindScene(curID)
	if curSc == nil || b == nil || b.IsPacked || !hasSceneTextModalChanges(s, curSc, r) {
		return false, ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, curSc, r)
	curScID := curSc.ID
	histLabel := trf(s, i18n.TrHistSceneTextEditedFmt, curSc.DisplayName())
	mutMsg := commitScenesMutation(s, histLabel, histKindText, coalesceTextModalPref+curScID)
	s.Editor.SelectedSceneID = curScID
	s.Editor.SceneText.SyncSceneID(curScID)
	if mutMsg != histLabel {
		return false, mutMsg
	}
	return true, ""
}

func openSceneTextModal(s *state.AppState, r *http.Request) string {
	sc := resolveSceneTextTarget(s, r)
	if sc == nil {
		return ""
	}
	if s.Editor.SelectedSceneID != sc.ID || s.Editor.SceneText.SceneID != sc.ID {
		s.Editor.SelectedVoiceSectionKey = defaultVoiceSectionKey
	}
	if s.Editor.SelectedSceneID != sc.ID {
		s.Editor.SceneText.SettingsOpen = false
	}
	s.Editor.SelectedSceneID = sc.ID
	s.Editor.SceneText.Open(sc.ID)
	return ""
}

func setSceneTextWidthModeAction(s *state.AppState, r *http.Request) string {
	raw := r.FormValue(formKeyMode)
	if raw == "" {
		raw = r.FormValue(formKeyWidth)
	}
	s.Editor.SceneText.WidthMode = state.ParseSceneWidthMode(raw)
	return ""
}

func closeSceneTextModal(s *state.AppState, r *http.Request) string {
	_, errToast := flushSceneTextChanges(s, r)
	s.Editor.SceneText.Close()
	s.Editor.SelectedVoiceSectionKey = defaultVoiceSectionKey
	return errToast
}

func toggleSceneTextSettings(s *state.AppState, _ *http.Request) string {
	s.Editor.SceneText.SettingsOpen = !s.Editor.SceneText.SettingsOpen
	return ""
}

func saveSceneTextModal(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	syncSceneTextSettingsOpen(s, r)
	b := s.Editor.ActiveBook
	target := resolveSceneTextTarget(s, r)
	if target == nil || b == nil || b.IsPacked {
		return ""
	}
	prevID := book.SanitizeSceneID(r.FormValue(formKeySceneID))
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	applySceneFormFields(s, target, r)
	if prevID != "" && !strings.EqualFold(prevID, target.ID) {
		s.Editor.LastRenamedFromSceneID = prevID
	}
	s.Editor.SelectedSceneID = target.ID
	s.Editor.SceneText.SyncSceneID(target.ID)
	toast := trf(s, i18n.TrSceneTextSavedFmt, target.DisplayName())
	return commitScenesMutation(s, toast, histKindText, coalesceTextModalPref+target.ID)
}

func navigateSceneTextModal(s *state.AppState, r *http.Request) string {
	s.Editor.SceneText.SettingsOpen = false
	_, errToast := flushSceneTextChanges(s, r)
	b := s.Editor.ActiveBook
	if b == nil {
		return errToast
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
	return errToast
}

func toggleChoiceRuleAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.ResetRulesUI()
		return ""
	}
	sc := resolveSceneTextTarget(s, r)
	idxStr := strings.TrimSpace(r.FormValue(formKeyChoiceIdx))
	if sc == nil || idxStr == "" {
		return ""
	}
	hasRule := false
	if chIdx, err := strconv.Atoi(idxStr); err == nil && chIdx >= 0 && chIdx < len(sc.Choices) {
		ch := sc.Choices[chIdx]
		hasRule = strings.TrimSpace(ch.Condition) != "" || strings.TrimSpace(ch.Effects) != ""
	}
	s.Editor.SceneText.ToggleChoiceRule(sc.ID, idxStr, hasRule)
	return ""
}

func toggleSyntaxHelpAction(s *state.AppState, _ *http.Request) string {
	if s.Editor.ActiveBook == nil || len(s.Editor.ActiveBook.Variables) == 0 {
		s.Editor.SceneText.SyntaxHelpOpen = false
		return ""
	}
	s.Editor.SceneText.SyntaxHelpOpen = !s.Editor.SceneText.SyntaxHelpOpen
	return ""
}

func setSceneTextPreviewModeAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.PreviewMode = state.PreviewModeNormal
		return ""
	}
	if raw := strings.TrimSpace(r.FormValue(formKeyMode)); raw != "" {
		s.Editor.SceneText.PreviewMode = state.ParseScenePreviewMode(raw)
	} else {
		s.Editor.SceneText.TogglePreviewMode()
	}
	if s.Editor.SceneText.PreviewMode != state.PreviewModeNormal {
		s.Editor.SceneText.TestVars = book.EnsureSaveVariables(b, s.Editor.SceneText.TestVars)
	}
	return ""
}

func toggleSceneTestVarsPopoverAction(s *state.AppState, _ *http.Request) string {
	b := s.Editor.ActiveBook
	if b == nil || len(b.Variables) == 0 {
		s.Editor.SceneText.TestVarsOpen = false
		return ""
	}
	s.Editor.SceneText.TestVarsOpen = !s.Editor.SceneText.TestVarsOpen
	if s.Editor.SceneText.TestVarsOpen {
		s.Editor.SceneText.TestVars = book.EnsureSaveVariables(b, s.Editor.SceneText.TestVars)
	}
	return ""
}

func updateSceneTextTestVarAction(s *state.AppState, r *http.Request) string {
	b := s.Editor.ActiveBook
	varName := book.SanitizeVarName(r.FormValue(formKeyVarName))
	if varName == "" || b == nil {
		return ""
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
	return ""
}
