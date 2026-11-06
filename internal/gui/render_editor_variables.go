package gui

import (
	"strings"

	"storyteller/internal/book"
)

func (s *Server) buildBookTargetSuggestions(sc *book.Scene) []map[string]any {
	if s.ActiveBook == nil {
		return nil
	}
	curCh := ""
	if sc != nil {
		curCh = strings.TrimSpace(sc.Chapter)
	}
	if curCh == "" {
		curCh = "Rozdział 1"
	}
	out := make([]map[string]any, 0, len(s.ActiveBook.Scenes)+len(s.ActiveBook.Chapters)+1)
	for _, cand := range s.ActiveBook.Scenes {
		if sc != nil && cand.ID == sc.ID {
			continue
		}
		candCh := strings.TrimSpace(cand.Chapter)
		if candCh == "" {
			candCh = "Rozdział 1"
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
	for _, chItem := range s.ActiveBook.Chapters {
		chRef := chItem.ReferenceKey()
		if ok, _ := s.ActiveBook.CanLinkChapters(curCh, chRef); !ok {
			continue
		}
		out = append(out, map[string]any{
			"value": "chapter:" + chRef,
			"meta":  chItem.DisplayName(),
		})
	}
	out = append(out, map[string]any{
		"value": "END",
		"meta":  "koniec ścieżki",
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
		sampleCond := v.Name
		sampleEff := v.Name + " = true"
		switch kind {
		case book.VarKindInt:
			sampleCond = v.Name + " >= 1"
			sampleEff = v.Name + " += 1"
		case book.VarKindEnum:
			firstOpt := v.EnumVal
			if firstOpt == "" && len(v.Options) > 0 {
				firstOpt = v.Options[0]
			}
			if len(v.Options) > 0 {
				meta = "enum: " + strings.Join(v.Options, ", ")
			}
			if firstOpt != "" {
				sampleCond = v.Name + " == " + firstOpt
				sampleEff = v.Name + " = " + firstOpt
			}
		}
		out = append(out, map[string]any{
			"name":        v.Name,
			"kind":        kind,
			"meta":        meta,
			"sample_cond": sampleCond,
			"sample_eff":  sampleEff,
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

func (s *Server) populateEditorVariablesContext(vars map[string]any) {
	vars["show_editor_variables_panel"] = s.EditorVariablesPanelOpen
	vars["has_book_variables"] = s.ActiveBook != nil && len(s.ActiveBook.Variables) > 0
	vars["book_vars_spec"] = book.EncodeBookVariablesSpec(s.ActiveBook)
	vars["book_var_suggestions"] = buildBookVarSuggestions(s.ActiveBook)
	vars["book_enum_suggestions"] = buildBookEnumValueSuggestions(s.ActiveBook)
	vars["book_target_suggestions"] = s.buildBookTargetSuggestions(s.findScene(s.SelectedSceneID))
	vars["syn_open"] = "{{"
	vars["syn_close"] = "}}"
	vars["syntax_help_title"] = "Składnia exit: / if: / set:"
	vars["syntax_help_cond_rows"] = []map[string]any{
		{"code": "if: odkryta_prawda && sumienie >= 2", "desc": "bool / int / enum (&&, ||, !, ==, !=, >=, <=)"},
		{"code": "set: odkryta_prawda = true; sumienie++", "desc": "efekty rozdzielone średnikiem ; (=, +=, -=, ++, --)"},
		{"code": "exit: ogien_w_kaplicy", "desc": "wyjście do sceny, chapter:<id> lub END"},
	}
	vars["syntax_help_eff_rows"] = []map[string]any{
		{"code": "set: !klucz; klucz = false", "desc": "bool toggle (!zmienna) lub jawnie (= true / false)"},
		{"code": "set: naboje -= 2; frakcja = rzeczni", "desc": "int (+=, -=, ++, --) lub enum (=)"},
	}
	if s.ActiveBook == nil {
		vars["var_flags"] = nil
		vars["var_counters"] = nil
		vars["var_states"] = nil
		vars["has_var_flags"] = false
		vars["has_var_counters"] = false
		vars["has_var_states"] = false
		return
	}
	book.ResolveBookVariableTypes(s.ActiveBook)
	var flags, counters, states []map[string]any
	for idx, v := range s.ActiveBook.Variables {
		switch v.Kind {
		case book.VarKindInt:
			counters = append(counters, map[string]any{
				"idx":     idx,
				"name":    v.Name,
				"int_val": v.IntVal,
			})
		case book.VarKindEnum:
			optsCSV := strings.Join(v.Options, ", ")
			if optsCSV == "" {
				optsCSV = v.EnumVal
			}
			states = append(states, map[string]any{
				"idx":         idx,
				"name":        v.Name,
				"options_csv": optsCSV,
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
