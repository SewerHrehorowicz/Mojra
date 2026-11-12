package views

import (
	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

type modalExportView struct {
	ShowExportModal          bool             `tpl:"show_export_modal"`
	ExportPrimaryFormats     []map[string]any `tpl:"export_primary_formats"`
	ExportAdvancedFormats    []map[string]any `tpl:"export_advanced_formats"`
	ExportSelectedCount      int              `tpl:"export_selected_count"`
	ExportSelectedCountLabel string           `tpl:"export_selected_count_label"`
	ExportHasSelected        bool             `tpl:"export_has_selected"`
	ExportAdvancedOpen       bool             `tpl:"export_advanced_open"`
	ExportModalStandard      string           `tpl:"export_modal_standard"`
	ExportModalStandardLabel string           `tpl:"export_modal_standard_label"`
	ExportModalStandardOpts  []map[string]any `tpl:"export_modal_standard_options"`
	ExportModalDir           string           `tpl:"export_modal_dir"`
	DefaultExportDir         string           `tpl:"default_export_dir"`
}

func formatExportSelectionCountLabel(count int, lang i18n.Lang) string {
	mod10 := count % 10
	mod100 := count % 100
	switch {
	case count == 1:
		return i18n.T(lang, i18n.TrExportSelectedOne)
	case mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14):
		return i18n.Tf(lang, i18n.TrExportSelectedFewFmt, count)
	default:
		return i18n.Tf(lang, i18n.TrExportSelectedManyFmt, count)
	}
}

func buildExportModalFormatLists(selected map[string]bool, lang i18n.Lang) ([]map[string]any, []map[string]any, int) {
	webLabel := i18n.T(lang, i18n.TrExportFmtWebApp)
	pdfLabel := i18n.T(lang, i18n.TrExportFmtPDFScreenplay)
	fountainLabel := i18n.T(lang, i18n.TrExportFmtFountain)
	playtxtLabel := i18n.T(lang, i18n.TrExportFmtPlaytxt)
	primaryDefs := []struct {
		id    string
		label string
		ext   string
	}{
		{"playbook", "Playbook", ".playbook"},
		{"wasm", webLabel, ".wasm"},
		{"pdf", pdfLabel, ".pdf"},
	}
	advancedDefs := []struct {
		id    string
		label string
		ext   string
	}{
		{"fountain", fountainLabel, ".fountain"},
		{"fdx", i18n.T(lang, i18n.TrExportFmtFDX), ".fdx"},
		{"treatment", i18n.T(lang, i18n.TrExportFmtTreatmentPDF), "-treatment.pdf"},
		{"book_pdf", i18n.T(lang, i18n.TrExportFmtBookPDF), "-book.pdf"},
		{"vo_csv", i18n.T(lang, i18n.TrExportFmtVOCSV), "-vo.csv"},
		{"actor_pdf", i18n.T(lang, i18n.TrExportFmtActorPDF), "-actors.pdf"},
		{"playtxt", playtxtLabel, ".playtxt"},
	}

	var primary []map[string]any
	var advanced []map[string]any
	totalSelected := 0

	for _, d := range primaryDefs {
		sel := selected == nil || selected[d.id]
		if sel {
			totalSelected++
		}
		primary = append(primary, map[string]any{
			"id":          d.id,
			"label":       d.label,
			"ext":         d.ext,
			"is_selected": sel,
		})
	}
	for _, d := range advancedDefs {
		sel := selected != nil && selected[d.id]
		if sel {
			totalSelected++
		}
		advanced = append(advanced, map[string]any{
			"id":          d.id,
			"label":       d.label,
			"ext":         d.ext,
			"is_selected": sel,
		})
	}
	return primary, advanced, totalSelected
}

func ViewModalExport(s *state.AppState) *modalExportView {
	lang := LangOf(s)
	b := s.Editor.ActiveBook
	primaryFmts, advancedFmts, selCount := buildExportModalFormatLists(s.ModalExport.Formats, lang)
	expStd := book.NormalizeScriptStandard(s.ModalExport.Standard)
	if s.ModalExport.Standard == "" && b != nil {
		expStd = b.NormalizedStandard()
	}
	expStdLabel, expStdOpts := buildScriptStandardOptions(expStd)
	defaultExportDir := b.ResolveExportDir(s.Settings)
	exportDir := s.ModalExport.Dir
	if exportDir == "" {
		exportDir = defaultExportDir
	}

	return &modalExportView{
		ShowExportModal:          s.ModalExport.Open,
		ExportPrimaryFormats:     primaryFmts,
		ExportAdvancedFormats:    advancedFmts,
		ExportSelectedCount:      selCount,
		ExportSelectedCountLabel: formatExportSelectionCountLabel(selCount, lang),
		ExportHasSelected:        selCount > 0,
		ExportAdvancedOpen:       s.ModalExport.AdvancedOpen,
		ExportModalStandard:      expStd,
		ExportModalStandardLabel: expStdLabel,
		ExportModalStandardOpts:  expStdOpts,
		ExportModalDir:           exportDir,
		DefaultExportDir:         defaultExportDir,
	}
}
