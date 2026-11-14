package views

import (
	"mojra/internal/book"
	"mojra/internal/state"
	"mojra/internal/tpl"
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

func NewBookView(s *state.AppState) tpl.TemplateChunk {
	d := s.NewBook
	d.Mode = book.NormalizeBookMode(d.Mode)
	d.Standard = book.NormalizeScriptStandard(d.Standard)
	d.Dir = book.ResolveNewBookDir(s.Settings)
	modeLbl, modeOpts := buildBookModeOptions(d.Mode, LangOf(s))
	_, stdOpts := buildScriptStandardOptions(d.Standard)

	return tpl.TemplateChunk{
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
