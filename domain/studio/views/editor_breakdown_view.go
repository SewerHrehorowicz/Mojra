//go:build !(js && wasm)

package views

import (
	"fmt"

	book "mojra/domain/book/actions"
	"mojra/domain/studio/state"
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

func EditorBreakdownView(s *state.StudioState) templates.TemplateChunk {
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
