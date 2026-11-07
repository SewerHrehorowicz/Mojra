package gui

import (
	"fmt"

	"storyteller/internal/book"
)

func buildBookModeOptions(activeMode string, lang Lang) (string, []map[string]any) {
	norm := book.NormalizeBookMode(activeMode)
	bookLbl := trLang(lang, TrModeInteractiveBook)
	spLbl := trLang(lang, TrModeScreenplay)
	label := bookLbl
	if norm == "screenplay" {
		label = spLbl
	}
	return label, []map[string]any{
		{"value": "book", "label": bookLbl, "is_active": norm == "book"},
		{"value": "screenplay", "label": spLbl, "is_active": norm == "screenplay"},
	}
}

func buildScriptStandardOptions(activeStd string) (string, []map[string]any) {
	norm := book.NormalizeScriptStandard(activeStd)
	label := "Polska — PISF (A4, WN./PL.)"
	switch norm {
	case "eu":
		label = "Europa (A4, INT./EXT.)"
	case "us":
		label = "USA (Letter, INT./EXT.)"
	}
	return label, []map[string]any{
		{"value": "pl", "label": "Polska — PISF (A4, WN./PL.)", "is_active": norm == "pl"},
		{"value": "eu", "label": "Europa (A4, INT./EXT.)", "is_active": norm == "eu"},
		{"value": "us", "label": "USA (Letter, INT./EXT.)", "is_active": norm == "us"},
	}
}

func buildSluglineHelperPills(standard string) ([]map[string]any, []map[string]any) {
	if book.NormalizeScriptStandard(standard) == "pl" {
		return []map[string]any{
				{"token": "WN."},
				{"token": "PL."},
				{"token": "WN./PL."},
			}, []map[string]any{
				{"token": "DZIEŃ"},
				{"token": "NOC"},
				{"token": "ŚWIT"},
				{"token": "ZMIERZCH"},
			}
	}
	return []map[string]any{
			{"token": "INT."},
			{"token": "EXT."},
			{"token": "INT./EXT."},
		}, []map[string]any{
			{"token": "DAY"},
			{"token": "NIGHT"},
			{"token": "DAWN"},
			{"token": "DUSK"},
		}
}

func (s *Server) populateEditorBreakdownContext(vars map[string]any) {
	vars["show_editor_breakdown_panel"] = s.EditorBreakdownPanelOpen
	if s.ActiveBook == nil {
		vars["has_breakdown_characters"] = false
		vars["has_breakdown_locations"] = false
		return
	}
	bd := s.ActiveBook.ScreenplayBreakdown()
	vars["breakdown_total_scenes"] = bd.TotalScenes
	vars["breakdown_total_pages"] = bd.TotalPagesFormatted
	vars["breakdown_runtime"] = bd.EstimatedRuntimeLabel
	vars["breakdown_shoot_days"] = bd.EstimatedShootDays
	vars["breakdown_int_ext_label"] = fmt.Sprintf("%d (%d%%) / %d (%d%%)", bd.IntScenes, bd.IntPct, bd.ExtScenes, bd.ExtPct)
	vars["breakdown_day_night_label"] = fmt.Sprintf("%d (%d%%) / %d (%d%%)", bd.DayScenes, bd.DayPct, bd.NightScenes, bd.NightPct)

	maxActPct := 0
	for _, a := range bd.Acts {
		if a.SharePct > maxActPct {
			maxActPct = a.SharePct
		}
	}
	actItems := make([]map[string]any, 0, len(bd.Acts))
	for _, a := range bd.Acts {
		actItems = append(actItems, map[string]any{
			"id":               a.ID,
			"name":             a.Name,
			"scene_count":      a.SceneCount,
			"formatted_length": a.FormattedLength,
			"share_pct":        a.SharePct,
			"is_dominant":      maxActPct > 0 && a.SharePct == maxActPct,
		})
	}
	vars["breakdown_acts"] = actItems
	vars["has_breakdown_acts"] = len(actItems) > 0

	charItems := make([]map[string]any, 0, len(bd.Characters))
	for _, ch := range bd.Characters {
		firstSceneID := ""
		if len(ch.SpeakingSceneIDs) > 0 {
			firstSceneID = ch.SpeakingSceneIDs[0]
		} else if len(ch.SilentSceneIDs) > 0 {
			firstSceneID = ch.SilentSceneIDs[0]
		}
		charItems = append(charItems, map[string]any{
			"name":                ch.Name,
			"dialogue_cues":       ch.DialogueCues,
			"word_count":          ch.WordCount,
			"total_scenes":        ch.TotalScenes,
			"speaking_scenes":     len(ch.SpeakingSceneIDs),
			"silent_scenes":       len(ch.SilentSceneIDs),
			"estimated_call_days": ch.EstimatedCallDays,
			"first_scene_id":      firstSceneID,
		})
	}
	vars["breakdown_characters"] = charItems
	vars["has_breakdown_characters"] = len(charItems) > 0

	locItems := make([]map[string]any, 0, len(bd.Locations))
	for _, loc := range bd.Locations {
		firstSceneID := ""
		if len(loc.SceneIDs) > 0 {
			firstSceneID = loc.SceneIDs[0]
		}
		locItems = append(locItems, map[string]any{
			"name":                 loc.Name,
			"int_ext":              loc.IntExt,
			"scene_count":          loc.SceneCount,
			"formatted_length":     loc.FormattedLength,
			"day_scenes":           loc.DayScenes,
			"night_scenes":         loc.NightScenes,
			"estimated_shoot_days": loc.EstimatedShootDays,
			"first_scene_id":       firstSceneID,
		})
	}
	vars["breakdown_locations"] = locItems
	vars["has_breakdown_locations"] = len(locItems) > 0
}
