//go:build !(js && wasm)

package views

import (
	"fmt"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
	"mojra/internal/tpl"
)

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

func buildBookModeOptions(activeMode string, lang i18n.Lang) (string, []book.SelectOption) {
	norm := book.NormalizeBookMode(activeMode)
	bookLbl := i18n.T(lang, i18n.TrModeInteractiveBook)
	spLbl := i18n.T(lang, i18n.TrModeScreenplay)
	label := bookLbl
	if norm == "screenplay" {
		label = spLbl
	}
	return label, []book.SelectOption{
		{Value: "book", Label: bookLbl, IsActive: norm == "book"},
		{Value: "screenplay", Label: spLbl, IsActive: norm == "screenplay"},
	}
}

func buildScriptStandardOptions(activeStd string) (string, []book.SelectOption) {
	norm := book.NormalizeScriptStandard(activeStd)
	label := "Polska — PISF (A4, WN./PL.)"
	switch norm {
	case "eu":
		label = "Europa (A4, INT./EXT.)"
	case "us":
		label = "USA (Letter, INT./EXT.)"
	}
	return label, []book.SelectOption{
		{Value: "pl", Label: "Polska — PISF (A4, WN./PL.)", IsActive: norm == "pl"},
		{Value: "eu", Label: "Europa (A4, INT./EXT.)", IsActive: norm == "eu"},
		{Value: "us", Label: "USA (Letter, INT./EXT.)", IsActive: norm == "us"},
	}
}

func buildSluglineHelperPills(standard string) ([]string, []string) {
	if book.NormalizeScriptStandard(standard) == "pl" {
		return []string{"WN.", "PL.", "WN./PL."}, []string{"DZIEŃ", "NOC", "ŚWIT", "ZMIERZCH"}
	}
	return []string{"INT.", "EXT.", "INT./EXT."}, []string{"DAY", "NIGHT", "DAWN", "DUSK"}
}

func ViewEditorBreakdown(s *state.AppState) tpl.TemplateChunk {
	b := s.Editor.ActiveBook
	v := &editorBreakdownTemplate{
		ShowEditorBreakdownPanel: s.Editor.RightPanel == state.PanelBreakdown,
		IsHistoryPreview:         !s.Config.StandaloneMode && s.Editor.History.IsPreview(b),
	}
	if b == nil {
		return tpl.TemplateChunk{TemplateData: v}
	}
	bd := b.ScreenplayBreakdown()
	v.BreakdownTotalScenes = bd.TotalScenes
	v.BreakdownTotalPages = bd.TotalPagesFormatted
	v.BreakdownRuntime = bd.EstimatedRuntimeLabel
	v.BreakdownShootDays = bd.EstimatedShootDays
	v.BreakdownIntExtLabel = fmt.Sprintf("%d (%d%%) / %d (%d%%)", bd.IntScenes, bd.IntPct, bd.ExtScenes, bd.ExtPct)
	v.BreakdownDayNightLabel = fmt.Sprintf("%d (%d%%) / %d (%d%%)", bd.DayScenes, bd.DayPct, bd.NightScenes, bd.NightPct)

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
	return tpl.TemplateChunk{TemplateData: v}
}
