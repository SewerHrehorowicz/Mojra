package views

import (
	"mojra/domain/book"
	"mojra/domain/state"
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

func NewBookView(s *state.AppState) templates.TemplateChunk {
	d := s.NewBook
	d.Mode = book.NormalizeBookMode(d.Mode)
	d.Standard = book.NormalizeScriptStandard(d.Standard)
	d.Dir = book.ResolveNewBookDir(s.Settings)
	lang := LangOf(s)
	modeLbl, modeOpts := buildBookModeOptions(d.Mode, lang)
	_, stdOpts := buildScriptStandardOptions(d.Standard, lang)

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
