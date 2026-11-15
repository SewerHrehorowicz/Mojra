//go:build !(js && wasm)

package views

import (
	"fmt"

	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine"
	"mojra/engine/templates"
)

const breakdownRatioFmt = "%d (%d%%) / %d (%d%%)"

type editorBreakdownTemplate struct {
	ShowEditorBreakdownPanel bool
	IsHistoryPreview         bool
	BreakdownTotalScenes     int
	BreakdownTotalPages      string
	BreakdownRuntime         string
	BreakdownShootDays       int
	BreakdownIntExtLabel     string
	BreakdownDayNightLabel   string
	BreakdownActs            []book.ActStat
	HasBreakdownActs         bool
	BreakdownCharacters      []book.CharacterStat
	HasBreakdownCharacters   bool
	BreakdownLocations       []book.LocationStat
	HasBreakdownLocations    bool
}

func buildBookModeOptions(activeMode string, lang engine.Lang) (string, []book.SelectOption) {
	norm := book.NormalizeBookMode(activeMode)
	bookLbl := engine.T(lang, i18n.TrModeInteractiveBook)
	spLbl := engine.T(lang, i18n.TrModeScreenplay)
	label := bookLbl
	if norm == book.BookModeScreenplay {
		label = spLbl
	}
	return label, []book.SelectOption{
		{Value: book.BookModeBook, Label: bookLbl, IsActive: norm == book.BookModeBook},
		{Value: book.BookModeScreenplay, Label: spLbl, IsActive: norm == book.BookModeScreenplay},
	}
}

func buildScriptStandardOptions(activeStd string, lang engine.Lang) (string, []book.SelectOption) {
	norm := book.NormalizeScriptStandard(activeStd)
	plLbl := engine.T(lang, i18n.TrStdLabelPL)
	euLbl := engine.T(lang, i18n.TrStdLabelEU)
	usLbl := engine.T(lang, i18n.TrStdLabelUS)
	label := plLbl
	switch norm {
	case book.ScriptStandardEU:
		label = euLbl
	case book.ScriptStandardUS:
		label = usLbl
	}
	return label, []book.SelectOption{
		{Value: book.ScriptStandardPL, Label: plLbl, IsActive: norm == book.ScriptStandardPL},
		{Value: book.ScriptStandardEU, Label: euLbl, IsActive: norm == book.ScriptStandardEU},
		{Value: book.ScriptStandardUS, Label: usLbl, IsActive: norm == book.ScriptStandardUS},
	}
}

func EditorBreakdownView(s *state.AppState) templates.TemplateChunk {
	b := s.Editor.ActiveBook
	v := &editorBreakdownTemplate{
		ShowEditorBreakdownPanel: s.Editor.RightPanel == state.PanelBreakdown,
		IsHistoryPreview:         !s.Config.StandaloneMode && s.Editor.History.IsPreview(b),
	}
	if b == nil {
		return templates.TemplateChunk{TemplateData: v}
	}
	bd := b.ScreenplayBreakdown()
	v.BreakdownTotalScenes = bd.TotalScenes
	v.BreakdownTotalPages = bd.TotalPagesFormatted
	v.BreakdownRuntime = bd.EstimatedRuntimeLabel
	v.BreakdownShootDays = bd.EstimatedShootDays
	v.BreakdownIntExtLabel = fmt.Sprintf(breakdownRatioFmt, bd.IntScenes, bd.IntPct, bd.ExtScenes, bd.ExtPct)
	v.BreakdownDayNightLabel = fmt.Sprintf(breakdownRatioFmt, bd.DayScenes, bd.DayPct, bd.NightScenes, bd.NightPct)

	maxActPct := 0
	for _, a := range bd.Acts {
		if a.SharePct > maxActPct {
			maxActPct = a.SharePct
		}
	}
	v.BreakdownActs = make([]book.ActStat, 0, len(bd.Acts))
	for _, a := range bd.Acts {
		actItem := a
		actItem.IsDominant = maxActPct > 0 && a.SharePct == maxActPct
		v.BreakdownActs = append(v.BreakdownActs, actItem)
	}
	v.HasBreakdownActs = len(v.BreakdownActs) > 0

	v.BreakdownCharacters = make([]book.CharacterStat, 0, len(bd.Characters))
	for _, ch := range bd.Characters {
		firstSceneID := ""
		if len(ch.SpeakingSceneIDs) > 0 {
			firstSceneID = ch.SpeakingSceneIDs[0]
		} else if len(ch.SilentSceneIDs) > 0 {
			firstSceneID = ch.SilentSceneIDs[0]
		}
		chItem := ch
		chItem.SpeakingScenes = len(ch.SpeakingSceneIDs)
		chItem.SilentScenes = len(ch.SilentSceneIDs)
		chItem.FirstSceneID = firstSceneID
		v.BreakdownCharacters = append(v.BreakdownCharacters, chItem)
	}
	v.HasBreakdownCharacters = len(v.BreakdownCharacters) > 0

	v.BreakdownLocations = make([]book.LocationStat, 0, len(bd.Locations))
	for _, loc := range bd.Locations {
		firstSceneID := ""
		if len(loc.SceneIDs) > 0 {
			firstSceneID = loc.SceneIDs[0]
		}
		locItem := loc
		locItem.FirstSceneID = firstSceneID
		v.BreakdownLocations = append(v.BreakdownLocations, locItem)
	}
	v.HasBreakdownLocations = len(v.BreakdownLocations) > 0
	return templates.TemplateChunk{TemplateData: v}
}
