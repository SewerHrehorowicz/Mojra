package views

import (
	"mojra/internal/book"
	"mojra/internal/state"
)

type modalNewBookView struct {
	Open            bool             `tpl:"open"`
	Mode            string           `tpl:"mode"`
	Standard        string           `tpl:"standard"`
	Title           string           `tpl:"title"`
	Author          string           `tpl:"author"`
	Description     string           `tpl:"description"`
	Narrated        bool             `tpl:"narrated"`
	Imageless       bool             `tpl:"imageless"`
	Dir             string           `tpl:"dir"`
	ModeLabel       string           `tpl:"mode_label"`
	ModeOptions     []map[string]any `tpl:"mode_options"`
	StandardOptions []map[string]any `tpl:"standard_options"`
}

func ViewModalNewBook(s *state.AppState) *modalNewBookView {
	d := s.ModalNewBook
	d.Mode = book.NormalizeBookMode(d.Mode)
	d.Standard = book.NormalizeScriptStandard(d.Standard)
	d.Dir = book.ResolveNewBookDir(s.Settings)
	modeLbl, modeOpts := buildBookModeOptions(d.Mode, LangOf(s))
	_, stdOpts := buildScriptStandardOptions(d.Standard)

	return &modalNewBookView{
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
	}
}
