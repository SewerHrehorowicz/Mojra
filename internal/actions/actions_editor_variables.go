//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func toggleEditorVariablesPanel(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelVariables {
		s.Editor.ActivateRightPanel(state.PanelNone)
	} else {
		activateRightPanel(s, state.PanelVariables)
	}
	return ""
}

func closeEditorVariablesPanel(s *state.AppState, _ *http.Request) string {
	if s.Editor.RightPanel == state.PanelVariables {
		s.Editor.ActivateRightPanel(state.PanelNone)
	}
	return ""
}

func nextUniqueVariableName(b *book.Book, prefix string) string {
	if b == nil {
		return prefix + "_1"
	}
	for i := 1; i <= 999; i++ {
		cand := fmt.Sprintf("%s_%d", prefix, i)
		if b.FindVariable(cand) == nil {
			return cand
		}
	}
	return prefix
}

func addBookVariableAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	if !s.Editor.SceneText.IsOpen() && r.FormValue("from_dropdown") != "true" {
		activateRightPanel(s, state.PanelVariables)
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	customName := book.SanitizeVarName(r.FormValue("new_var_name"))
	if customName != "" && b.FindVariable(customName) != nil {
		customName = ""
	}
	var newVar book.VariableDef
	var label string
	switch kind {
	case book.VarKindInt:
		varName := customName
		if varName == "" {
			varName = nextUniqueVariableName(b, "int")
		}
		newVar = book.VariableDef{
			Name:   varName,
			Kind:   book.VarKindInt,
			IntVal: 0,
		}
		label = fmt.Sprintf("Dodano int: %s", newVar.Name)
	case book.VarKindEnum:
		varName := customName
		if varName == "" {
			varName = nextUniqueVariableName(b, "enum")
		}
		newVar = book.VariableDef{
			Name:    varName,
			Kind:    book.VarKindEnum,
			EnumVal: "wariant_1",
			Options: []string{"wariant_1", "wariant_2"},
		}
		label = fmt.Sprintf("Dodano enum: %s", newVar.Name)
	default:
		varName := customName
		if varName == "" {
			varName = nextUniqueVariableName(b, "bool")
		}
		newVar = book.VariableDef{
			Name:    varName,
			Kind:    book.VarKindBool,
			BoolVal: false,
		}
		label = fmt.Sprintf("Dodano bool: %s", newVar.Name)
	}
	b.Variables = append(b.Variables, newVar)
	book.ResolveBookVariableTypes(b)
	return commitBookVariableMutation(s, label, "addvariable")
}

func commitBookVariableMutation(s *state.AppState, label, coalesceKey string) string {
	b := s.Editor.ActiveBook
	if err := book.SaveBookMetadataToDisk(b); err != nil {
		return err.Error()
	}
	s.Editor.History.RecordStep(b, label, "meta", coalesceKey, s.Editor.SelectedSceneID)
	return ""
}

func mutateVariableAtFormIndex(s *state.AppState, r *http.Request, fn func(idx int, v *book.VariableDef) string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("var_idx")))
	if err != nil || idx < 0 || idx >= len(b.Variables) {
		return ""
	}
	return fn(idx, &b.Variables[idx])
}

func toggleBookVariableBoolAction(s *state.AppState, r *http.Request) string {
	return mutateVariableAtFormIndex(s, r, func(_ int, v *book.VariableDef) string {
		s.Editor.History.EnsureBaseline(s.Editor.ActiveBook, s.Editor.SelectedSceneID)
		v.BoolVal = !v.BoolVal
		return commitBookVariableMutation(s, fmt.Sprintf("Zmieniono wartość startową zmiennej „%s”", v.Name), "togglevariable")
	})
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

func extractEnumOptionsFromForm(r *http.Request) ([]string, bool) {
	if rawVal, hasVal := r.Form["var_value"]; hasVal && len(rawVal) > 0 {
		return parseEnumOptionsSlice(rawVal), true
	}
	maxIdx := -1
	if cntStr := strings.TrimSpace(r.FormValue("opt_count")); cntStr != "" {
		if cnt, err := strconv.Atoi(cntStr); err == nil && cnt > 0 && cnt <= 500 {
			maxIdx = cnt - 1
		}
	}
	for k := range r.Form {
		if strings.HasPrefix(k, "enum_opt_") {
			if idx, err := strconv.Atoi(strings.TrimPrefix(k, "enum_opt_")); err == nil && idx >= 0 && idx <= 500 {
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
		key := fmt.Sprintf("enum_opt_%d", i)
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
		cand := fmt.Sprintf("wariant_%d", i)
		if !seen[cand] {
			return cand
		}
	}
	return "wariant"
}

func applyVariableFormEdits(s *state.AppState, idx int, v *book.VariableDef, r *http.Request) bool {
	b := s.Editor.ActiveBook
	changed := false
	if rawName, hasName := r.Form["var_name"]; hasName && len(rawName) > 0 {
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
		if rawVal, hasVal := r.Form["var_value"]; hasVal && len(rawVal) > 0 {
			lower := strings.ToLower(strings.TrimSpace(rawVal[0]))
			nextBool := lower == "true" || lower == "tak" || lower == "1" || lower == "yes"
			if nextBool != v.BoolVal {
				if !changed {
					s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
				}
				v.BoolVal = nextBool
				changed = true
			}
		}
	case book.VarKindInt:
		if rawVal, hasVal := r.Form["var_value"]; hasVal && len(rawVal) > 0 {
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

func updateBookVariableAction(s *state.AppState, r *http.Request) string {
	return mutateVariableAtFormIndex(s, r, func(idx int, v *book.VariableDef) string {
		if applyVariableFormEdits(s, idx, v, r) {
			book.ResolveBookVariableTypes(s.Editor.ActiveBook)
			return commitBookVariableMutation(s, fmt.Sprintf("Zaktualizowano zmienną „%s”", v.Name), "updatevariable")
		}
		return ""
	})
}

func addBookEnumOptionAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("var_idx")))
	if err != nil || idx < 0 || idx >= len(b.Variables) {
		return ""
	}
	v := &b.Variables[idx]
	if v.Kind != book.VarKindEnum {
		return ""
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
	return commitBookVariableMutation(s, fmt.Sprintf("Dodano wariant „%s” do zmiennej „%s”", nextOpt, v.Name), "addenumopt")
}

func removeBookEnumOptionAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("var_idx")))
	if err != nil || idx < 0 || idx >= len(b.Variables) {
		return ""
	}
	optIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("opt_idx")))
	if err != nil || optIdx < 0 {
		return ""
	}
	v := &b.Variables[idx]
	if v.Kind != book.VarKindEnum {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	_ = applyVariableFormEdits(s, idx, v, r)
	if len(v.Options) <= 1 || optIdx >= len(v.Options) {
		return ""
	}
	removedOpt := v.Options[optIdx]
	opts := append([]string(nil), v.Options[:optIdx]...)
	opts = append(opts, v.Options[optIdx+1:]...)
	syncEnumVariableOptions(b, v, opts)
	book.ResolveBookVariableTypes(b)
	return commitBookVariableMutation(s, fmt.Sprintf("Usunięto wariant „%s” ze zmiennej „%s”", removedOpt, v.Name), "removeenumopt")
}

func removeBookVariableAction(s *state.AppState, r *http.Request) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil || b.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("var_idx")))
	if err != nil || idx < 0 || idx >= len(b.Variables) {
		return ""
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	removedName := b.Variables[idx].Name
	b.Variables = append(b.Variables[:idx], b.Variables[idx+1:]...)
	if len(b.Variables) == 0 {
		s.Editor.SceneText.PreviewMode = state.PreviewModeNormal
		s.Editor.SceneText.TestVarsOpen = false
		s.Editor.SceneText.ResetRulesUI()
	}
	return commitBookVariableMutation(s, fmt.Sprintf("Usunięto zmienną „%s”", removedName), "removevariable")
}
