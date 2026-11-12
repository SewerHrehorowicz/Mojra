//go:build !(js && wasm)

package views

import (
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func buildBookTargetSuggestions(s *state.AppState, sc *book.Scene) []map[string]any {
	b := s.Editor.ActiveBook
	if b == nil {
		return nil
	}
	curCh := ""
	if sc != nil {
		curCh = strings.TrimSpace(sc.Chapter)
	}
	if curCh == "" {
		curCh = b.DefaultChapterName()
	}
	curChMeta := b.FindChapter(curCh)
	out := make([]map[string]any, 0, len(b.Scenes)+len(b.Chapters)+1)
	for _, cand := range b.Scenes {
		if sc != nil && strings.EqualFold(cand.ID, sc.ID) {
			continue
		}
		candCh := strings.TrimSpace(cand.Chapter)
		if candCh == "" {
			candCh = b.DefaultChapterName()
		}
		if !strings.EqualFold(candCh, curCh) {
			continue
		}
		val := cand.ReferenceKey()
		if cand.ExplicitID != "" {
			val = cand.ExplicitID
		}
		meta := cand.DisplayName()
		if strings.EqualFold(meta, val) {
			meta = candCh
		}
		out = append(out, map[string]any{
			"value": val,
			"meta":  meta,
		})
	}
	for _, chItem := range b.Chapters {
		chRef := chItem.ReferenceKey()
		if strings.EqualFold(chItem.Name, curCh) || strings.EqualFold(chRef, curCh) || strings.EqualFold(chItem.ID, curCh) {
			continue
		}
		if curChMeta != nil && (strings.EqualFold(chItem.ID, curChMeta.ID) || strings.EqualFold(chRef, curChMeta.ReferenceKey())) {
			continue
		}
		out = append(out, map[string]any{
			"value": "chapter:" + chRef,
			"meta":  chItem.DisplayName(),
		})
	}
	out = append(out, map[string]any{
		"value": "END",
		"meta":  tr(s, i18n.TrEndOfPathMeta),
	})
	return out
}

func buildBookVarSuggestions(b *book.Book) []map[string]any {
	if b == nil || len(b.Variables) == 0 {
		return nil
	}
	book.ResolveBookVariableTypes(b)
	out := make([]map[string]any, 0, len(b.Variables))
	for _, v := range b.Variables {
		kind := v.Kind
		if kind == "" {
			kind = book.VarKindBool
		}
		meta := kind
		if kind == book.VarKindEnum && len(v.Options) > 0 {
			meta = "enum: " + strings.Join(v.Options, ", ")
		}
		out = append(out, map[string]any{
			"name": v.Name,
			"kind": kind,
			"meta": meta,
		})
	}
	return out
}

func buildBookEnumValueSuggestions(b *book.Book) []map[string]any {
	if b == nil || len(b.Variables) == 0 {
		return nil
	}
	book.ResolveBookVariableTypes(b)
	var out []map[string]any
	for _, v := range b.Variables {
		if v.Kind != book.VarKindEnum || len(v.Options) == 0 {
			continue
		}
		varLower := strings.ToLower(strings.TrimSpace(v.Name))
		for _, opt := range v.Options {
			cleanOpt := strings.TrimSpace(opt)
			if cleanOpt == "" {
				continue
			}
			out = append(out, map[string]any{
				"var_name": varLower,
				"value":    cleanOpt,
				"meta":     v.Name,
			})
		}
	}
	return out
}

func populateEditorVariablesContext(s *state.AppState, vars map[string]any) {
	b := s.Editor.ActiveBook
	vars["show_editor_variables_panel"] = s.Editor.RightPanel == state.PanelVariables
	vars["has_book_variables"] = b != nil && len(b.Variables) > 0
	vars["book_vars_spec"] = book.EncodeBookVariablesSpec(b)
	vars["book_var_suggestions"] = buildBookVarSuggestions(b)
	vars["book_enum_suggestions"] = buildBookEnumValueSuggestions(b)
	vars["book_target_suggestions"] = buildBookTargetSuggestions(s, b.FindScene(s.Editor.SelectedSceneID))
	vars["syntax_help_cond_rows"] = []map[string]any{
		{"code": "if: odkryta_prawda && sumienie >= 2", "desc": "bool / int / enum (&&, ||, !, ==, !=, >=, <=)"},
		{"code": "set: odkryta_prawda = true; sumienie++", "desc": tr(s, i18n.TrSyntaxHelpSetDesc)},
		{"code": "exit: ogien_w_kaplicy", "desc": tr(s, i18n.TrSyntaxHelpSetDesc)},
	}
	if b == nil {
		vars["var_flags"] = nil
		vars["var_counters"] = nil
		vars["var_states"] = nil
		vars["has_var_flags"] = false
		vars["has_var_counters"] = false
		vars["has_var_states"] = false
		return
	}
	book.ResolveBookVariableTypes(b)
	var flags, counters, states []map[string]any
	for idx, v := range b.Variables {
		switch v.Kind {
		case book.VarKindInt:
			counters = append(counters, map[string]any{
				"idx":     idx,
				"name":    v.Name,
				"int_val": v.IntVal,
			})
		case book.VarKindEnum:
			rawOpts := v.Options
			if len(rawOpts) == 0 && v.EnumVal != "" {
				rawOpts = []string{v.EnumVal}
			}
			optItems := make([]map[string]any, 0, len(rawOpts))
			for optIdx, optVal := range rawOpts {
				optItems = append(optItems, map[string]any{
					"opt_idx": optIdx,
					"value":   optVal,
				})
			}
			states = append(states, map[string]any{
				"idx":            idx,
				"name":           v.Name,
				"options":        optItems,
				"opt_count":      len(optItems),
				"can_remove_opt": len(optItems) > 1,
			})
		default:
			lbl := "false"
			if v.BoolVal {
				lbl = "true"
			}
			flags = append(flags, map[string]any{
				"idx":        idx,
				"name":       v.Name,
				"bool_val":   v.BoolVal,
				"bool_label": lbl,
			})
		}
	}
	vars["var_flags"] = flags
	vars["var_counters"] = counters
	vars["var_states"] = states
	vars["has_var_flags"] = len(flags) > 0
	vars["has_var_counters"] = len(counters) > 0
	vars["has_var_states"] = len(states) > 0
}
