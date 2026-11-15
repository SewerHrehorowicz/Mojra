package views

import (
	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine"
	"mojra/engine/templates"
)

const exportPlaybookLabel = "Playbook"

type exportFormatSpec struct {
	id         string
	staticLbl  string
	labelTrKey engine.TrKey
	ext        string
}

var (
	primaryExportFormatSpecs = [...]exportFormatSpec{
		{id: "playbook", staticLbl: exportPlaybookLabel, ext: ".playbook"},
		{id: "wasm", labelTrKey: i18n.TrExportFmtWebApp, ext: ".wasm"},
		{id: "pdf", labelTrKey: i18n.TrExportFmtPDFScreenplay, ext: ".pdf"},
	}
	advancedExportFormatSpecs = [...]exportFormatSpec{
		{id: "fountain", labelTrKey: i18n.TrExportFmtFountain, ext: ".fountain"},
		{id: "fdx", labelTrKey: i18n.TrExportFmtFDX, ext: ".fdx"},
		{id: "treatment", labelTrKey: i18n.TrExportFmtTreatmentPDF, ext: "-treatment.pdf"},
		{id: "book_pdf", labelTrKey: i18n.TrExportFmtBookPDF, ext: "-book.pdf"},
		{id: "vo_csv", labelTrKey: i18n.TrExportFmtVOCSV, ext: "-vo.csv"},
		{id: "actor_pdf", labelTrKey: i18n.TrExportFmtActorPDF, ext: "-actors.pdf"},
		{id: "playtxt", labelTrKey: i18n.TrExportFmtPlaytxt, ext: ".playtxt"},
	}
)

type exportTemplate struct {
	ShowExportModal            bool
	ExportPrimaryFormats       []book.ExportFormatItem
	ExportAdvancedFormats      []book.ExportFormatItem
	ExportSelectedCount        int
	ExportSelectedCountLabel   string
	ExportHasSelected          bool
	ExportAdvancedOpen         bool
	ExportModalStandard        string
	ExportModalStandardLabel   string
	ExportModalStandardOptions []book.SelectOption
	ExportModalDir             string
	DefaultExportDir           string
}

func buildExportModalFormatLists(selected map[string]bool, lang engine.Lang) ([]book.ExportFormatItem, []book.ExportFormatItem, int) {
	primary := make([]book.ExportFormatItem, 0, len(primaryExportFormatSpecs))
	advanced := make([]book.ExportFormatItem, 0, len(advancedExportFormatSpecs))
	totalSelected := 0

	for _, d := range primaryExportFormatSpecs {
		sel := selected == nil || selected[d.id]
		if sel {
			totalSelected++
		}
		lbl := d.staticLbl
		if lbl == "" {
			lbl = engine.T(lang, d.labelTrKey)
		}
		primary = append(primary, book.ExportFormatItem{
			ID:         d.id,
			Label:      lbl,
			Ext:        d.ext,
			IsSelected: sel,
		})
	}
	for _, d := range advancedExportFormatSpecs {
		sel := selected != nil && selected[d.id]
		if sel {
			totalSelected++
		}
		advanced = append(advanced, book.ExportFormatItem{
			ID:         d.id,
			Label:      engine.T(lang, d.labelTrKey),
			Ext:        d.ext,
			IsSelected: sel,
		})
	}
	return primary, advanced, totalSelected
}

func ExportView(s *state.AppState) templates.TemplateChunk {
	lang := LangOf(s)
	b := s.Editor.ActiveBook
	primaryFmts, advancedFmts, selCount := buildExportModalFormatLists(s.Export.Formats, lang)
	expStd := book.NormalizeScriptStandard(s.Export.Standard)
	if s.Export.Standard == "" && b != nil {
		expStd = b.NormalizedStandard()
	}
	expStdLabel, expStdOpts := buildScriptStandardOptions(expStd, lang)
	defaultExportDir := b.ResolveExportDir(s.Settings)
	exportDir := s.Export.Dir
	if exportDir == "" {
		exportDir = defaultExportDir
	}

	return templates.TemplateChunk{
		TemplateData: &exportTemplate{
			ShowExportModal:            s.Export.Open,
			ExportPrimaryFormats:       primaryFmts,
			ExportAdvancedFormats:      advancedFmts,
			ExportSelectedCount:        selCount,
			ExportSelectedCountLabel:   engine.Plural(lang, selCount, i18n.TrExportSelectedOne, i18n.TrExportSelectedFewFmt, i18n.TrExportSelectedManyFmt),
			ExportHasSelected:          selCount > 0,
			ExportAdvancedOpen:         s.Export.AdvancedOpen,
			ExportModalStandard:        expStd,
			ExportModalStandardLabel:   expStdLabel,
			ExportModalStandardOptions: expStdOpts,
			ExportModalDir:             exportDir,
			DefaultExportDir:           defaultExportDir,
		},
	}
}
