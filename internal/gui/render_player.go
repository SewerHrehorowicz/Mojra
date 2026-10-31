package gui

import (
	"fmt"
	"strings"
)

func extractFirstSentence(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "Kliknij, aby wpisać treść sceny..."
	}
	flat := strings.Join(strings.Fields(cleaned), " ")
	runes := []rune(flat)
	for i, r := range runes {
		if r == '.' || r == '!' || r == '?' || r == '…' {
			if i+1 == len(runes) || runes[i+1] == ' ' {
				return string(runes[:i+1])
			}
		}
	}
	return flat
}

func (s *Server) populatePlayerContext(vars map[string]any) {
	curScene := s.currentScene()
	if curScene == nil {
		curScene = s.ActiveBook.Scenes[0]
	}

	histCount := 0
	if s.Save != nil {
		histCount = len(s.Save.History)
	}
	vars["has_history"] = histCount > 0

	lastImageURL := ""
	imageless := s.isEffectiveImageless()
	if histCount > 0 {
		histList := make([]map[string]any, 0, histCount)
		for idx, entry := range s.Save.History {
			pastScene := s.findScene(entry.SceneID)
			if pastScene == nil {
				continue
			}
			if !imageless && pastScene.ImageURL != "" {
				lastImageURL = pastScene.ImageURL
			}
			pastImgURL := ""
			if !imageless && pastScene.ImageURL != "" {
				pastImgURL = pastScene.ImageURL
			}
			pastHeaderMeta := s.ActiveBook.Title
			if pastScene.Chapter != "" {
				pastHeaderMeta = s.ActiveBook.Title + " • " + pastScene.Chapter
			}
			pastHasTitle := strings.TrimSpace(pastScene.Name) != ""
			histList = append(histList, map[string]any{
				"idx":          idx,
				"header_meta":  pastHeaderMeta,
				"name":         pastScene.Name,
				"rail_title":   pastScene.DisplayName(),
				"has_title":    pastHasTitle,
				"html":         pastScene.HTML,
				"has_image":    pastImgURL != "",
				"image_url":    pastImgURL,
				"bg_image_url": lastImageURL,
			})
		}
		vars["history_items"] = histList
	}

	hasMultipleChoices := len(curScene.Choices) > 1
	curChoices := make([]map[string]any, 0, len(curScene.Choices))
	for idx, ch := range curScene.Choices {
		label := ch.Label
		if !hasMultipleChoices {
			label = "Dalej"
		}
		arrow := "»"
		if ch.IsEnding {
			arrow = "KONIEC"
		}
		isImplicitNext := !hasMultipleChoices && !ch.IsEnding && ch.TargetSceneID != "END"
		curChoices = append(curChoices, map[string]any{
			"idx":         idx,
			"label":       label,
			"arrow":       arrow,
			"is_branch":   hasMultipleChoices && !ch.IsEnding,
			"is_implicit": isImplicitNext,
		})
	}

	curImgURL := ""
	if !imageless && curScene.ImageURL != "" {
		curImgURL = curScene.ImageURL
		lastImageURL = curScene.ImageURL
	}
	curHeaderMeta := s.ActiveBook.Title
	if curScene.Chapter != "" {
		curHeaderMeta = s.ActiveBook.Title + " • " + curScene.Chapter
	}
	curHasTitle := strings.TrimSpace(curScene.Name) != ""

	vars["current_scene"] = map[string]any{
		"name":         curScene.Name,
		"rail_title":   curScene.DisplayName(),
		"has_title":    curHasTitle,
		"header_meta":  curHeaderMeta,
		"html":         curScene.HTML,
		"has_image":    curImgURL != "",
		"image_url":    curImgURL,
		"has_bg_image": !imageless && lastImageURL != "",
		"bg_image_url": lastImageURL,
		"audio_url":    curScene.AudioURL,
		"is_terminal":  len(curScene.Choices) == 0 || curScene.ExplicitEnd,
		"choices":      curChoices,
	}

	vars["show_audio_deck"] = s.ActiveBook.Narrated

	progressPct := 0
	if len(s.ActiveBook.Scenes) > 0 {
		visited := make(map[string]bool)
		if s.Save != nil {
			for _, h := range s.Save.History {
				if h.SceneID != "" {
					visited[h.SceneID] = true
				}
			}
		}
		if curScene != nil && curScene.ID != "" {
			visited[curScene.ID] = true
		}
		progressPct = (len(visited) * 100) / len(s.ActiveBook.Scenes)
		if progressPct < 1 {
			progressPct = 1
		}
		if progressPct > 100 {
			progressPct = 100
		}
	}
	vars["book_progress_pct"] = progressPct

	vars["audio_resume_time"] = fmt.Sprintf("%.1f", s.currentAudioTime())
	vars["audio_speed"] = fmt.Sprintf("%.2g", s.Settings.PlaybackSpeed)
	vars["auto_choice_enabled"] = s.Settings.AutoChoice
	if s.Settings.AutoChoice {
		vars["auto_choice_label"] = "Auto-Wybór: WŁ"
	} else {
		vars["auto_choice_label"] = "Auto-Wybór: WYŁ"
	}
}
