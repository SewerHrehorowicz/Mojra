package gui

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"storyteller/internal/book"
)

func (s *Server) toggleEditorVariablesPanel(r *http.Request) string {
	if r.FormValue("book_settings_open") == "false" {
		s.BookSettingsOpen = false
	}
	s.EditorVariablesPanelOpen = !s.EditorVariablesPanelOpen
	if s.EditorVariablesPanelOpen {
		s.activateStudioRightPanel("variables")
	}
	return ""
}

func (s *Server) closeEditorVariablesPanel(_ *http.Request) string {
	s.EditorVariablesPanelOpen = false
	return ""
}

func (s *Server) nextUniqueVariableName(prefix string) string {
	if s.ActiveBook == nil {
		return prefix + "_1"
	}
	for i := 1; i <= 999; i++ {
		cand := fmt.Sprintf("%s_%d", prefix, i)
		if s.ActiveBook.FindVariable(cand) == nil {
			return cand
		}
	}
	return prefix
}

func (s *Server) addBookVariableAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	if s.SceneTextModalID == "" && r.FormValue("from_dropdown") != "true" {
		s.EditorVariablesPanelOpen = true
	}
	s.ensureHistoryBaselineBeforeMutation()
	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	customName := book.SanitizeVarName(r.FormValue("new_var_name"))
	if customName != "" && s.ActiveBook.FindVariable(customName) != nil {
		customName = ""
	}
	var newVar book.VariableDef
	var label string
	switch kind {
	case book.VarKindInt:
		varName := customName
		if varName == "" {
			varName = s.nextUniqueVariableName("int")
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
			varName = s.nextUniqueVariableName("enum")
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
			varName = s.nextUniqueVariableName("bool")
		}
		newVar = book.VariableDef{
			Name:    varName,
			Kind:    book.VarKindBool,
			BoolVal: false,
		}
		label = fmt.Sprintf("Dodano bool: %s", newVar.Name)
	}
	s.ActiveBook.Variables = append(s.ActiveBook.Variables, newVar)
	book.ResolveBookVariableTypes(s.ActiveBook)
	return s.commitBookVariableMutation(label, "addvariable")
}

func (s *Server) commitBookVariableMutation(label, coalesceKey string) string {
	if err := book.SaveBookMetadataToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.recordHistoryAfterMutation(label, "meta", coalesceKey)
	return ""
}

func (s *Server) mutateVariableAtFormIndex(r *http.Request, fn func(idx int, v *book.VariableDef) string) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("var_idx")))
	if err != nil || idx < 0 || idx >= len(s.ActiveBook.Variables) {
		return ""
	}
	return fn(idx, &s.ActiveBook.Variables[idx])
}

func (s *Server) toggleBookVariableBoolAction(r *http.Request) string {
	return s.mutateVariableAtFormIndex(r, func(_ int, v *book.VariableDef) string {
		s.ensureHistoryBaselineBeforeMutation()
		v.BoolVal = !v.BoolVal
		return s.commitBookVariableMutation(fmt.Sprintf("Zmieniono wartość startową zmiennej „%s”", v.Name), "togglevariable")
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

func (s *Server) applyVariableFormEdits(idx int, v *book.VariableDef, r *http.Request) bool {
	changed := false
	if rawName, hasName := r.Form["var_name"]; hasName && len(rawName) > 0 {
		cleanName := book.SanitizeVarName(rawName[0])
		if cleanName != "" && cleanName != v.Name {
			duplicate := false
			for i := range s.ActiveBook.Variables {
				if i != idx && strings.EqualFold(s.ActiveBook.Variables[i].Name, cleanName) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				s.ensureHistoryBaselineBeforeMutation()
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
					s.ensureHistoryBaselineBeforeMutation()
				}
				v.BoolVal = nextBool
				changed = true
			}
		}
	case book.VarKindInt:
		if rawVal, hasVal := r.Form["var_value"]; hasVal && len(rawVal) > 0 {
			if iv, err := strconv.Atoi(strings.TrimSpace(rawVal[0])); err == nil && iv != v.IntVal {
				if !changed {
					s.ensureHistoryBaselineBeforeMutation()
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
					s.ensureHistoryBaselineBeforeMutation()
				}
				syncEnumVariableOptions(s.ActiveBook, v, opts)
				changed = true
			}
		}
	}
	return changed
}

func (s *Server) updateBookVariableAction(r *http.Request) string {
	return s.mutateVariableAtFormIndex(r, func(idx int, v *book.VariableDef) string {
		if s.applyVariableFormEdits(idx, v, r) {
			book.ResolveBookVariableTypes(s.ActiveBook)
			return s.commitBookVariableMutation(fmt.Sprintf("Zaktualizowano zmienną „%s”", v.Name), "updatevariable")
		}
		return ""
	})
}

func (s *Server) addBookEnumOptionAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("var_idx")))
	if err != nil || idx < 0 || idx >= len(s.ActiveBook.Variables) {
		return ""
	}
	v := &s.ActiveBook.Variables[idx]
	if v.Kind != book.VarKindEnum {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	_ = s.applyVariableFormEdits(idx, v, r)
	opts := append([]string(nil), v.Options...)
	if len(opts) == 0 && v.EnumVal != "" {
		opts = []string{v.EnumVal}
	}
	nextOpt := nextUniqueEnumOptionName(opts)
	opts = append(opts, nextOpt)
	syncEnumVariableOptions(s.ActiveBook, v, opts)
	book.ResolveBookVariableTypes(s.ActiveBook)
	return s.commitBookVariableMutation(fmt.Sprintf("Dodano wariant „%s” do zmiennej „%s”", nextOpt, v.Name), "addenumopt")
}

func (s *Server) removeBookEnumOptionAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("var_idx")))
	if err != nil || idx < 0 || idx >= len(s.ActiveBook.Variables) {
		return ""
	}
	optIdx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("opt_idx")))
	if err != nil || optIdx < 0 {
		return ""
	}
	v := &s.ActiveBook.Variables[idx]
	if v.Kind != book.VarKindEnum {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	_ = s.applyVariableFormEdits(idx, v, r)
	if len(v.Options) <= 1 || optIdx >= len(v.Options) {
		return ""
	}
	removedOpt := v.Options[optIdx]
	opts := append([]string(nil), v.Options[:optIdx]...)
	opts = append(opts, v.Options[optIdx+1:]...)
	syncEnumVariableOptions(s.ActiveBook, v, opts)
	book.ResolveBookVariableTypes(s.ActiveBook)
	return s.commitBookVariableMutation(fmt.Sprintf("Usunięto wariant „%s” ze zmiennej „%s”", removedOpt, v.Name), "removeenumopt")
}

func (s *Server) removeBookVariableAction(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("var_idx")))
	if err != nil || idx < 0 || idx >= len(s.ActiveBook.Variables) {
		return ""
	}
	s.ensureHistoryBaselineBeforeMutation()
	removedName := s.ActiveBook.Variables[idx].Name
	s.ActiveBook.Variables = append(s.ActiveBook.Variables[:idx], s.ActiveBook.Variables[idx+1:]...)
	if len(s.ActiveBook.Variables) == 0 {
		s.SceneTextPreviewMode = "normal"
		s.SceneTestVarsPopoverOpen = false
		s.SyntaxHelpPopoverOpen = false
		s.ChoiceRulesOpen = nil
	}
	return s.commitBookVariableMutation(fmt.Sprintf("Usunięto zmienną „%s”", removedName), "removevariable")
}

func (s *Server) toggleChoiceRuleAction(r *http.Request) string {
	if s.ActiveBook == nil || len(s.ActiveBook.Variables) == 0 {
		s.ChoiceRulesOpen = nil
		s.SyntaxHelpPopoverOpen = false
		return ""
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	if scID == "" {
		scID = s.SelectedSceneID
	}
	idxStr := strings.TrimSpace(r.FormValue("choice_idx"))
	if scID == "" || idxStr == "" {
		return ""
	}
	if s.ChoiceRulesOpen == nil {
		s.ChoiceRulesOpen = make(map[string]bool)
	}
	key := scID + ":" + idxStr
	curOpen := s.ChoiceRulesOpen[key]
	if sc := s.findScene(scID); sc != nil {
		if chIdx, err := strconv.Atoi(idxStr); err == nil && chIdx >= 0 && chIdx < len(sc.Choices) {
			ch := sc.Choices[chIdx]
			hasRule := strings.TrimSpace(ch.Condition) != "" || strings.TrimSpace(ch.Effects) != ""
			if _, exists := s.ChoiceRulesOpen[key]; !exists && hasRule {
				curOpen = true
			}
		}
	}
	s.ChoiceRulesOpen[key] = !curOpen
	if !s.ChoiceRulesOpen[key] {
		s.SyntaxHelpPopoverOpen = false
	}
	return ""
}

func (s *Server) toggleSyntaxHelpAction(_ *http.Request) string {
	if s.ActiveBook == nil || len(s.ActiveBook.Variables) == 0 {
		s.SyntaxHelpPopoverOpen = false
		return ""
	}
	s.SyntaxHelpPopoverOpen = !s.SyntaxHelpPopoverOpen
	return ""
}

func (s *Server) setSceneTextPreviewModeAction(r *http.Request) string {
	if s.ActiveBook == nil || len(s.ActiveBook.Variables) == 0 {
		s.SceneTextPreviewMode = "normal"
		return ""
	}
	mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
	switch mode {
	case "highlight", "output":
		s.SceneTextPreviewMode = mode
		s.SceneTextTestVars = book.EnsureSaveVariables(s.ActiveBook, s.SceneTextTestVars)
	case "normal":
		s.SceneTextPreviewMode = "normal"
	default:
		if s.SceneTextPreviewMode == "output" || s.SceneTextPreviewMode == "highlight" {
			s.SceneTextPreviewMode = "normal"
		} else {
			s.SceneTextPreviewMode = "output"
			s.SceneTextTestVars = book.EnsureSaveVariables(s.ActiveBook, s.SceneTextTestVars)
		}
	}
	return ""
}

func (s *Server) toggleSceneTestVarsPopoverAction(_ *http.Request) string {
	if s.ActiveBook == nil || len(s.ActiveBook.Variables) == 0 {
		s.SceneTestVarsPopoverOpen = false
		return ""
	}
	s.SceneTestVarsPopoverOpen = !s.SceneTestVarsPopoverOpen
	if s.SceneTestVarsPopoverOpen {
		s.SceneTextTestVars = book.EnsureSaveVariables(s.ActiveBook, s.SceneTextTestVars)
	}
	return ""
}

func (s *Server) updateSceneTextTestVarAction(r *http.Request) string {
	varName := book.SanitizeVarName(r.FormValue("var_name"))
	if varName == "" || s.ActiveBook == nil {
		return ""
	}
	s.SceneTextTestVars = book.EnsureSaveVariables(s.ActiveBook, s.SceneTextTestVars)
	if r.FormValue("toggle") == "true" {
		if s.SceneTextTestVars[varName] == "true" {
			s.SceneTextTestVars[varName] = "false"
		} else {
			s.SceneTextTestVars[varName] = "true"
		}
		return ""
	}
	if deltaStr := strings.TrimSpace(r.FormValue("delta")); deltaStr != "" {
		if delta, err := strconv.Atoi(deltaStr); err == nil {
			cur, _ := strconv.Atoi(s.SceneTextTestVars[varName])
			s.SceneTextTestVars[varName] = strconv.Itoa(cur + delta)
			return ""
		}
	}
	val := strings.TrimSpace(r.FormValue("var_value"))
	if vDef := s.ActiveBook.FindVariable(varName); vDef != nil {
		switch vDef.Kind {
		case book.VarKindBool:
			if strings.EqualFold(val, "true") || strings.EqualFold(val, "tak") || val == "1" {
				s.SceneTextTestVars[varName] = "true"
			} else {
				s.SceneTextTestVars[varName] = "false"
			}
		case book.VarKindInt:
			if iv, err := strconv.Atoi(val); err == nil {
				s.SceneTextTestVars[varName] = strconv.Itoa(iv)
			}
		case book.VarKindEnum:
			opt := book.SanitizeEnumOption(val)
			if opt != "" {
				s.SceneTextTestVars[varName] = opt
			}
		}
	} else if val != "" {
		s.SceneTextTestVars[varName] = val
	}
	return ""
}
