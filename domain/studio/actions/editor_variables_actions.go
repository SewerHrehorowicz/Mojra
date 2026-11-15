//go:build !(js && wasm)

package actions

import (
	"fmt"
	"strconv"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/transport"
)

const (
	formKeyFromDropdown       = "from_dropdown"
	formKeyNewVarName         = "new_var_name"
	formKeyVarIdx             = "var_idx"
	formKeyOptCount           = "opt_count"
	formKeyOptIdx             = "opt_idx"
	formKeyEnumOptPref        = "enum_opt_"
	formKeyEnumOptFmt         = "enum_opt_%d"
	varNameFallbackSuff       = "_1"
	varNameSeqFmt             = "%s_%d"
	defaultEnumOpt1           = "wariant_1"
	defaultEnumOpt2           = "wariant_2"
	defaultEnumOptBase        = "wariant"
	defaultEnumOptSeqFmt      = "wariant_%d"
	boolValTak                = "tak"
	boolValYes                = "yes"
	coalesceAddVariable       = "addvariable"
	coalesceToggleVariable    = "togglevariable"
	coalesceUpdateVariable    = "updatevariable"
	coalesceAddEnumOpt     = "addenumopt"
	coalesceRemoveEnumOpt  = "removeenumopt"
	coalesceRemoveVariable = "removevariable"
)

var _ = registerActions(map[string]ActionHandler{
	"ToggleEditorVariablesPanel": toggleEditorVariablesPanel,
	"CloseEditorVariablesPanel":  closeEditorVariablesPanel,
	"AddBookVariable":            addBookVariableAction,
	"ToggleBookVariableBool":     toggleBookVariableBoolAction,
	"UpdateBookVariable":         updateBookVariableAction,
	"RemoveBookVariable":         removeBookVariableAction,
	"AddBookEnumOption":          addBookEnumOptionAction,
	"RemoveBookEnumOption":       removeBookEnumOptionAction,
})

func toggleEditorVariablesPanel(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	if s.Editor.RightPanel == state.PanelVariables {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		activateRightPanel(s, state.PanelVariables)
	}
	return frameWorkspace(s, "")
}

func closeEditorVariablesPanel(s *state.StudioState, _ *transport.ActionRequest) []transport.ActionFrame {
	if s.Editor.RightPanel == state.PanelVariables {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return frameWorkspace(s, "")
}

func nextUniqueVariableName(b *book.Book, prefix string) string {
	if b == nil {
		return prefix + varNameFallbackSuff
	}
	for i := 1; i <= 999; i++ {
		cand := fmt.Sprintf(varNameSeqFmt, prefix, i)
		if b.FindVariable(cand) == nil {
			return cand
		}
	}
	return prefix
}

func addBookVariableAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	hadFormScene := flushFormSceneIfPresent(s, r)
	if isHistoryPreview(s) {
		return frameSceneForm(s, tr(s, i18n.TrHistoryPreviewLocked), hadFormScene, false)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameSceneForm(s, "", hadFormScene, false)
	}
	if !s.Editor.SceneText.IsOpen() && r.FormValue(formKeyFromDropdown) != formValTrue {
		activateRightPanel(s, state.PanelVariables)
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	kind := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyKind)))
	customName := book.SanitizeVarName(r.FormValue(formKeyNewVarName))
	if customName != "" && b.FindVariable(customName) != nil {
		customName = ""
	}
	var newVar book.VariableDef
	var label string
	switch kind {
	case book.VarKindInt:
		varName := customName
		if varName == "" {
			varName = nextUniqueVariableName(b, book.VarKindInt)
		}
		newVar = book.VariableDef{
			Name:   varName,
			Kind:   book.VarKindInt,
			IntVal: 0,
		}
		label = trf(s, i18n.TrAddedIntVarFmt, newVar.Name)
	case book.VarKindEnum:
		varName := customName
		if varName == "" {
			varName = nextUniqueVariableName(b, book.VarKindEnum)
		}
		newVar = book.VariableDef{
			Name:    varName,
			Kind:    book.VarKindEnum,
			EnumVal: defaultEnumOpt1,
			Options: []string{defaultEnumOpt1, defaultEnumOpt2},
		}
		label = trf(s, i18n.TrAddedEnumVarFmt, newVar.Name)
	default:
		varName := customName
		if varName == "" {
			varName = nextUniqueVariableName(b, book.VarKindBool)
		}
		newVar = book.VariableDef{
			Name:    varName,
			Kind:    book.VarKindBool,
			BoolVal: false,
		}
		label = trf(s, i18n.TrAddedBoolVarFmt, newVar.Name)
	}
	b.Variables = append(b.Variables, newVar)
	book.ResolveBookVariableTypes(b)
	toast := commitBookVariableMutation(s, label, coalesceAddVariable)
	if s.Editor.SceneText.IsOpen() && s.Editor.RightPanel == state.PanelVariables {
		return frameApp(s, toast)
	}
	return frameSceneForm(s, toast, hadFormScene, false)
}

func commitBookVariableMutation(s *state.StudioState, label, coalesceKey string) string {
	b := s.Editor.ActiveBook
	if err := book.SaveBookMetadataToDisk(b); err != nil {
		return err.Error()
	}
	s.Editor.History.RecordStep(b, label, historyScopeMeta, coalesceKey, s.Editor.SelectedSceneID)
	return ""
}

func mutateVariableAtFormIndex(s *state.StudioState, r *transport.ActionRequest, fn func(idx int, v *book.VariableDef) string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyVarIdx)))
	if err != nil || idx < 0 || idx >= len(b.Variables) {
		return ""
	}
	return fn(idx, &b.Variables[idx])
}

func toggleBookVariableBoolAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	toast := mutateVariableAtFormIndex(s, r, func(_ int, v *book.VariableDef) string {
		s.Editor.History.EnsureBaseline(s.Editor.ActiveBook, s.Editor.SelectedSceneID)
		v.BoolVal = !v.BoolVal
		return commitBookVariableMutation(s, trf(s, i18n.TrToggledVarStartValFmt, v.Name), coalesceToggleVariable)
	})
	return frameUndoRedo(s, toast)
}

func parseEnumOptionsSlice(rawItems []string) []string {
	var opts []string
	seen := make(map[string]bool)
	for _, raw := range rawItems {
		norm := strings.NewReplacer("\r\n", ",", "\n", ",", ";", ",").Replace(raw)
		for _, p := range strings.Split(norm, ",") {
			opt := book.SanitizeEnumOption(p)
			if opt != "" && !seen[opt] {
				seen[opt] = true
				opts = append(opts, opt)
			}
		}
	}
	return opts
}

func extractEnumOptionsFromForm(r *transport.ActionRequest) ([]string, bool) {
	if rawVal, hasVal := r.Form[formKeyVarValue]; hasVal && len(rawVal) > 0 {
		return parseEnumOptionsSlice(rawVal), true
	}
	maxIdx := -1
	if cntStr := strings.TrimSpace(r.FormValue(formKeyOptCount)); cntStr != "" {
		if cnt, err := strconv.Atoi(cntStr); err == nil && cnt > 0 && cnt <= 500 {
			maxIdx = cnt - 1
		}
	}
	for k := range r.Form {
		if strings.HasPrefix(k, formKeyEnumOptPref) {
			if idx, err := strconv.Atoi(strings.TrimPrefix(k, formKeyEnumOptPref)); err == nil && idx >= 0 && idx <= 500 {
				if idx > maxIdx {
					maxIdx = idx
				}
			}
		}
	}
	if maxIdx < 0 {
		return nil, false
	}
	rawItems := make([]string, 0, maxIdx+1)
	for i := 0; i <= maxIdx; i++ {
		key := fmt.Sprintf(formKeyEnumOptFmt, i)
		if vals, ok := r.Form[key]; ok && len(vals) > 0 {
			rawItems = append(rawItems, vals[0])
		}
	}
	return parseEnumOptionsSlice(rawItems), true
}

func syncEnumVariableOptions(b *book.Book, v *book.VariableDef, opts []string) {
	v.Options = opts
	if len(opts) > 0 {
		v.EnumVal = opts[0]
	}
	if v.EnumName != "" && b != nil {
		for i := range b.Enums {
			if strings.EqualFold(b.Enums[i].Name, v.EnumName) {
				b.Enums[i].Values = append([]string(nil), opts...)
			}
		}
	}
}

func nextUniqueEnumOptionName(opts []string) string {
	seen := make(map[string]bool, len(opts))
	for _, o := range opts {
		seen[strings.ToLower(strings.TrimSpace(o))] = true
	}
	for i := 1; i <= 999; i++ {
		cand := fmt.Sprintf(defaultEnumOptSeqFmt, i)
		if !seen[cand] {
			return cand
		}
	}
	return defaultEnumOptBase
}

func applyVariableFormEdits(s *state.StudioState, idx int, v *book.VariableDef, r *transport.ActionRequest) bool {
	b := s.Editor.ActiveBook
	changed := false
	if rawName, hasName := r.Form[formKeyVarName]; hasName && len(rawName) > 0 {
		cleanName := book.SanitizeVarName(rawName[0])
		if cleanName != "" && cleanName != v.Name {
			duplicate := false
			for i := range b.Variables {
				if i != idx && strings.EqualFold(b.Variables[i].Name, cleanName) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
				v.Name = cleanName
				changed = true
			}
		}
	}

	switch v.Kind {
	case book.VarKindBool:
		if rawVal, hasVal := r.Form[formKeyVarValue]; hasVal && len(rawVal) > 0 {
			lower := strings.ToLower(strings.TrimSpace(rawVal[0]))
			nextBool := lower == formValTrue || lower == boolValTak || lower == "1" || lower == boolValYes
			if nextBool != v.BoolVal {
				if !changed {
					s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
				}
				v.BoolVal = nextBool
				changed = true
			}
		}
	case book.VarKindInt:
		if rawVal, hasVal := r.Form[formKeyVarValue]; hasVal && len(rawVal) > 0 {
			if iv, err := strconv.Atoi(strings.TrimSpace(rawVal[0])); err == nil && iv != v.IntVal {
				if !changed {
					s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
				}
				v.IntVal = iv
				changed = true
			}
		}
	case book.VarKindEnum:
		if opts, hasOpts := extractEnumOptionsFromForm(r); hasOpts && len(opts) > 0 {
			oldJoined := strings.Join(v.Options, ",")
			newJoined := strings.Join(opts, ",")
			if oldJoined != newJoined || v.EnumVal != opts[0] {
				if !changed {
					s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
				}
				syncEnumVariableOptions(b, v, opts)
				changed = true
			}
		}
	}
	return changed
}

func updateBookVariableAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	toast := mutateVariableAtFormIndex(s, r, func(idx int, v *book.VariableDef) string {
		if applyVariableFormEdits(s, idx, v, r) {
			book.ResolveBookVariableTypes(s.Editor.ActiveBook)
			return commitBookVariableMutation(s, trf(s, i18n.TrUpdatedVarFmt, v.Name), coalesceUpdateVariable)
		}
		return ""
	})
	return frameUndoRedo(s, toast)
}

func addBookEnumOptionAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameUndoRedo(s, "")
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyVarIdx)))
	if err != nil || idx < 0 || idx >= len(b.Variables) {
		return frameUndoRedo(s, "")
	}
	v := &b.Variables[idx]
	if v.Kind != book.VarKindEnum {
		return frameUndoRedo(s, "")
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	_ = applyVariableFormEdits(s, idx, v, r)
	opts := append([]string(nil), v.Options...)
	if len(opts) == 0 && v.EnumVal != "" {
		opts = []string{v.EnumVal}
	}
	nextOpt := nextUniqueEnumOptionName(opts)
	opts = append(opts, nextOpt)
	syncEnumVariableOptions(b, v, opts)
	book.ResolveBookVariableTypes(b)
	return frameUndoRedo(s, commitBookVariableMutation(s, trf(s, i18n.TrAddedEnumOptFmt, nextOpt, v.Name), coalesceAddEnumOpt))
}

func removeBookEnumOptionAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameUndoRedo(s, "")
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyVarIdx)))
	if err != nil || idx < 0 || idx >= len(b.Variables) {
		return frameUndoRedo(s, "")
	}
	optIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyOptIdx)))
	if err != nil || optIdx < 0 {
		return frameUndoRedo(s, "")
	}
	v := &b.Variables[idx]
	if v.Kind != book.VarKindEnum {
		return frameUndoRedo(s, "")
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	_ = applyVariableFormEdits(s, idx, v, r)
	if len(v.Options) <= 1 || optIdx >= len(v.Options) {
		return frameUndoRedo(s, "")
	}
	removedOpt := v.Options[optIdx]
	opts := append([]string(nil), v.Options[:optIdx]...)
	opts = append(opts, v.Options[optIdx+1:]...)
	syncEnumVariableOptions(b, v, opts)
	book.ResolveBookVariableTypes(b)
	return frameUndoRedo(s, commitBookVariableMutation(s, trf(s, i18n.TrRemovedEnumOptFmt, removedOpt, v.Name), coalesceRemoveEnumOpt))
}

func removeBookVariableAction(s *state.StudioState, r *transport.ActionRequest) []transport.ActionFrame {
	if isHistoryPreview(s) {
		return frameUndoRedo(s, tr(s, i18n.TrHistoryPreviewLocked))
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return frameUndoRedo(s, "")
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue(formKeyVarIdx)))
	if err != nil || idx < 0 || idx >= len(b.Variables) {
		return frameUndoRedo(s, "")
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	removedName := b.Variables[idx].Name
	b.Variables = append(b.Variables[:idx], b.Variables[idx+1:]...)
	if len(b.Variables) == 0 {
		s.Editor.SceneText.PreviewMode = state.PreviewModeNormal
		s.Editor.SceneText.TestVarsOpen = false
		s.Editor.SceneText.ResetRulesUI()
	}
	return frameUndoRedo(s, commitBookVariableMutation(s, trf(s, i18n.TrRemovedVarFmt, removedName), coalesceRemoveVariable))
}
