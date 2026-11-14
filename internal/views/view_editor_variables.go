//go:build !(js && wasm)

package views

import (
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type editorVariablesTemplate struct {
	ShowEditorVariablesPanel bool
	IsHistoryPreview         bool
	HasBookVariables         bool
	BookVarsSpec             string
	BookVarSuggestions       []book.VariableDef
	BookEnumSuggestions      []book.SelectOption
	BookTargetSuggestions    []book.SelectOption
	SyntaxHelpCondRows       []book.SelectOption
	VarFlags                 []book.VariableDef
	VarCounters              []book.VariableDef
	VarStates                []book.VariableDef
	HasVarFlags              bool
	HasVarCounters           bool
	HasVarStates             bool
}

func buildBookTargetSuggestions(s *state.AppState, sc *book.Scene) []book.SelectOption {
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
	out := make([]book.SelectOption, 0, len(b.Scenes)+len(b.Chapters)+1)
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
		out = append(out, book.SelectOption{
			Value: val,
			Meta:  meta,
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
		out = append(out, book.SelectOption{
			Value: "chapter:" + chRef,
			Meta:  chItem.DisplayName(),
		})
	}
	out = append(out, book.SelectOption{
		Value: "END",
		Meta:  tr(s, i18n.TrEndOfPathMeta),
	})
	return out
}

func buildBookVarSuggestions(b *book.Book) []book.VariableDef {
	if b == nil || len(b.Variables) == 0 {
		return nil
	}
	book.ResolveBookVariableTypes(b)
	out := make([]book.VariableDef, 0, len(b.Variables))
	for _, v := range b.Variables {
		kind := v.Kind
		if kind == "" {
			kind = book.VarKindBool
		}
		meta := kind
		if kind == book.VarKindEnum && len(v.Options) > 0 {
			meta = "enum: " + strings.Join(v.Options, ", ")
		}
		out = append(out, book.VariableDef{
			Name: v.Name,
			Kind: kind,
			Meta: meta,
		})
	}
	return out
}

func buildBookEnumValueSuggestions(b *book.Book) []book.SelectOption {
	if b == nil || len(b.Variables) == 0 {
		return nil
	}
	book.ResolveBookVariableTypes(b)
	var out []book.SelectOption
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
			out = append(out, book.SelectOption{
				VarName: varLower,
				Value:   cleanOpt,
				Meta:    v.Name,
			})
		}
	}
	return out
}

func ViewEditorVariables(s *state.AppState) tpl.TemplateChunk {
	b := s.Editor.ActiveBook
	var selScene *book.Scene
	if b != nil {
		selScene = b.FindScene(s.Editor.SelectedSceneID)
	}
	v := &editorVariablesTemplate{
		ShowEditorVariablesPanel: s.Editor.RightPanel == state.PanelVariables,
		IsHistoryPreview:         !s.Config.StandaloneMode && s.Editor.History.IsPreview(b),
		HasBookVariables:         b != nil && len(b.Variables) > 0,
		BookVarsSpec:             book.EncodeBookVariablesSpec(b),
		BookVarSuggestions:       buildBookVarSuggestions(b),
		BookEnumSuggestions:      buildBookEnumValueSuggestions(b),
		BookTargetSuggestions:    buildBookTargetSuggestions(s, selScene),
		SyntaxHelpCondRows: []book.SelectOption{
			{Value: "if: odkryta_prawda && sumienie >= 2", Label: "bool / int / enum (&&, ||, !, ==, !=, >=, <=)"},
			{Value: "set: odkryta_prawda = true; sumienie++", Label: tr(s, i18n.TrSyntaxHelpSetDesc)},
			{Value: "exit: ogien_w_kaplicy", Label: tr(s, i18n.TrSyntaxHelpSetDesc)},
		},
	}
	if b == nil {
		return tpl.TemplateChunk{TemplateData: v}
	}
	book.ResolveBookVariableTypes(b)
	for idx, bv := range b.Variables {
		switch bv.Kind {
		case book.VarKindInt:
			v.VarCounters = append(v.VarCounters, book.VariableDef{
				Idx:    idx,
				Name:   bv.Name,
				IntVal: bv.IntVal,
			})
		case book.VarKindEnum:
			rawOpts := bv.Options
			if len(rawOpts) == 0 && bv.EnumVal != "" {
				rawOpts = []string{bv.EnumVal}
			}
			optItems := make([]book.SelectOption, 0, len(rawOpts))
			for optIdx, optVal := range rawOpts {
				optItems = append(optItems, book.SelectOption{
					OptIdx: optIdx,
					Value:  optVal,
				})
			}
			v.VarStates = append(v.VarStates, book.VariableDef{
				Idx:          idx,
				Name:         bv.Name,
				OptionItems:  optItems,
				OptCount:     len(optItems),
				CanRemoveOpt: len(optItems) > 1,
			})
		default:
			lbl := "false"
			if bv.BoolVal {
				lbl = "true"
			}
			v.VarFlags = append(v.VarFlags, book.VariableDef{
				Idx:       idx,
				Name:      bv.Name,
				BoolVal:   bv.BoolVal,
				BoolLabel: lbl,
			})
		}
	}
	v.HasVarFlags = len(v.VarFlags) > 0
	v.HasVarCounters = len(v.VarCounters) > 0
	v.HasVarStates = len(v.VarStates) > 0
	return tpl.TemplateChunk{TemplateData: v}
}
