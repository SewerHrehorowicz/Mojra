//go:build !(js && wasm)

package views

import (
	book "mojra/domain/book/actions"
	commonviews "mojra/domain/common/views"
	"mojra/domain/studio/state"
	"mojra/engine/templates"
)

type newBookTemplate struct {
	Open            bool
	Mode            string
	Standard        string
	Title           string
	Author          string
	Description     string
	Narrated        bool
	Imageless       bool
	Dir             string
	ModeLabel       string
	ModeOptions     []book.SelectOption
	StandardOptions []book.SelectOption
}

func NewBookView(studioState *state.StudioState) templates.TemplateChunk {
	draft := studioState.NewBook
	if draft == nil {
		draft = &state.NewBookState{}
	}
	draft.Mode = book.NormalizeBookMode(draft.Mode)
	draft.Standard = book.NormalizeScriptStandard(draft.Standard)
	if draft.Dir == "" {
		draft.Dir = book.ResolveNewBookDir(studioState.Settings)
	}
	lang := commonviews.LangOf(studioState.UIState)
	modeLbl, modeOpts := commonviews.BuildBookModeOptions(draft.Mode, lang)
	_, stdOpts := commonviews.BuildScriptStandardOptions(draft.Standard, lang)

	return templates.TemplateChunk{
		TemplateData: &newBookTemplate{
			Open:            draft.Open,
			Mode:            draft.Mode,
			Standard:        draft.Standard,
			Title:           draft.Title,
			Author:          draft.Author,
			Description:     draft.Description,
			Narrated:        draft.Narrated,
			Imageless:       draft.Imageless,
			Dir:             draft.Dir,
			ModeLabel:       modeLbl,
			ModeOptions:     modeOpts,
			StandardOptions: stdOpts,
		},
	}
}
