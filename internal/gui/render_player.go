package gui

import (
	"fmt"
	"strings"

	"storyteller/internal/book"
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
				"html":         pastScene.EnsureHTML(),
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
		"html":         curScene.EnsureHTML(),
		"has_image":    curImgURL != "",
		"image_url":    curImgURL,
		"has_bg_image": !imageless && lastImageURL != "",
		"bg_image_url": lastImageURL,
		"audio_url":    curScene.AudioURL,
		"is_terminal":  len(curScene.Choices) == 0 || curScene.ExplicitEnd,
		"choices":      curChoices,
	}

	narrated := s.ActiveBook != nil && s.ActiveBook.Narrated
	vars["show_audio_deck"] = narrated

	seenURLs := make(map[string]bool, 16)
	if curImgURL != "" {
		seenURLs[curImgURL] = true
	}
	if lastImageURL != "" {
		seenURLs[lastImageURL] = true
	}
	if curScene.AudioURL != "" {
		seenURLs[curScene.AudioURL] = true
	}

	seenScenes := make(map[string]bool, 16)
	seenScenes[curScene.ID] = true

	var hop1Scenes []*book.Scene
	for _, ch := range curScene.Choices {
		if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == "END" || seenScenes[ch.TargetSceneID] {
			continue
		}
		target := s.findScene(ch.TargetSceneID)
		if target == nil {
			continue
		}
		seenScenes[target.ID] = true
		hop1Scenes = append(hop1Scenes, target)
	}

	var hop2Scenes []*book.Scene
	for _, h1 := range hop1Scenes {
		for _, ch := range h1.Choices {
			if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == "END" || seenScenes[ch.TargetSceneID] {
				continue
			}
			target := s.findScene(ch.TargetSceneID)
			if target == nil {
				continue
			}
			seenScenes[target.ID] = true
			hop2Scenes = append(hop2Scenes, target)
		}
	}

	var prefetchURLs []string
	appendSceneMedia := func(sc *book.Scene) {
		if !imageless && sc.ImageURL != "" && !seenURLs[sc.ImageURL] {
			seenURLs[sc.ImageURL] = true
			prefetchURLs = append(prefetchURLs, sc.ImageURL)
		}
		if narrated && sc.AudioURL != "" && !seenURLs[sc.AudioURL] {
			seenURLs[sc.AudioURL] = true
			prefetchURLs = append(prefetchURLs, sc.AudioURL)
		}
	}
	for _, sc := range hop1Scenes {
		appendSceneMedia(sc)
	}
	for _, sc := range hop2Scenes {
		appendSceneMedia(sc)
	}
	vars["prefetch_media_urls"] = strings.Join(prefetchURLs, ";")

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
