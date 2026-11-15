//go:build !(js && wasm)

package views

import (
	book "mojra/domain/book/actions"
	commonviews "mojra/domain/common/views"
	"mojra/domain/library/state"
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

func NewBookView(s *state.LibraryState) templates.TemplateChunk {
	d := s.NewBook
	if d == nil {
		d = &state.NewBookState{}
	}
	d.Mode = book.NormalizeBookMode(d.Mode)
	d.Standard = book.NormalizeScriptStandard(d.Standard)
	if d.Dir == "" {
		d.Dir = book.ResolveNewBookDir(s.Settings)
	}
	lang := commonviews.LangOf(s.UIState)
	modeLbl, modeOpts := commonviews.BuildBookModeOptions(d.Mode, lang)
	_, stdOpts := commonviews.BuildScriptStandardOptions(d.Standard, lang)

	return templates.TemplateChunk{
		TemplateData: &newBookTemplate{
			Open:            d.Open,
			Mode:            d.Mode,
			Standard:        d.Standard,
			Title:           d.Title,
			Author:          d.Author,
			Description:     d.Description,
			Narrated:        d.Narrated,
			Imageless:       d.Imageless,
			Dir:             d.Dir,
			ModeLabel:       modeLbl,
			ModeOptions:     modeOpts,
			StandardOptions: stdOpts,
		},
	}
}
