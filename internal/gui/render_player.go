//go:build !(js && wasm)

package gui

import (
	"fmt"
	"strings"

	"mojra/internal/book"
)

func buildPlayerScreenplayBlocks(sc *book.Scene, state map[string]string, b *book.Book, sceneNum int, standard string) []map[string]any {
	if sc == nil {
		return nil
	}
	evalSc := sc
	if book.HasDynamicProse(sc.Text) {
		clone := *sc
		clone.Text = book.EvaluateProseText(sc.Text, state, b)
		evalSc = &clone
	}
	normStd := book.NormalizeScriptStandard(standard)
	blocks := book.ParseScreenplaySceneBlocks(evalSc, normStd)
	if len(blocks) == 0 {
		return nil
	}
	numbered := normStd == "pl" || normStd == "eu"
	renderedPrimarySlug := false
	out := make([]map[string]any, 0, len(blocks))
	for _, blk := range blocks {
		showNum := false
		if blk.Kind == book.BlockSlugline && !renderedPrimarySlug {
			renderedPrimarySlug = true
			showNum = numbered && sceneNum > 0
		}
		out = append(out, map[string]any{
			"kind":             string(blk.Kind),
			"text":             blk.Text,
			"scene_num":        sceneNum,
			"show_scene_num":   showNum,
			"is_slugline":      blk.Kind == book.BlockSlugline,
			"is_character":     blk.Kind == book.BlockCharacter,
			"is_parenthetical": blk.Kind == book.BlockParenthetical,
			"is_dialogue":      blk.Kind == book.BlockDialogue,
			"is_transition":    blk.Kind == book.BlockTransition,
		})
	}
	return out
}

func resolvePlayerSceneAudio(b *book.Book, sc *book.Scene, state map[string]string) (string, string, string, string, string) {
	if b == nil || !b.Narrated || sc == nil {
		return "", "0:00", "0.0000", "1.0000", ""
	}
	queue := book.EvaluateSceneAudioQueue(sc, state, b)
	queueSpec, _, totalDurLbl := book.EncodeSceneAudioQueue(queue)
	if len(queue) > 0 {
		ts, te := book.NormalizedVoiceTrimValues(queue[0].TrimStart, queue[0].TrimEnd)
		return queue[0].AudioURL, totalDurLbl, fmt.Sprintf("%.4f", ts), fmt.Sprintf("%.4f", te), queueSpec
	}
	if sc.AudioURL == "" {
		return "", "0:00", "0.0000", "1.0000", ""
	}
	durLbl, stamp := book.ReadSceneAudioDurationAndStamp(b, sc)
	if durLbl == "" {
		durLbl = "0:00"
	}
	urlStr := sc.AudioURL
	if stamp > 0 && !strings.Contains(urlStr, "?v=") {
		urlStr = fmt.Sprintf("%s?v=%d", urlStr, stamp)
	}
	ts, te := sc.NormalizedVoiceTrim()
	return urlStr, durLbl, fmt.Sprintf("%.4f", ts), fmt.Sprintf("%.4f", te), ""
}

func (s *Server) populatePlayerContext(vars map[string]any) {
	if s.PlayerBook == nil || len(s.PlayerBook.Scenes) == 0 {
		return
	}
	curScene := s.currentScene()
	if curScene == nil {
		curScene = s.PlayerBook.Scenes[0]
	}

	isScreenplay := s.PlayerBook.IsScreenplay()
	normStd := s.PlayerBook.NormalizedStandard()
	vars["is_screenplay_mode"] = isScreenplay

	histCount := 0
	if s.Save != nil {
		for i, h := range s.Save.History {
			if strings.EqualFold(h.SceneID, curScene.ID) {
				s.Save.History = s.Save.History[:i]
				s.persistSaveState()
				break
			}
		}
		histCount = len(s.Save.History)
		if len(s.Save.Variables) == 0 && len(s.PlayerBook.Variables) > 0 {
			s.Save.Variables = book.EnsureSaveVariables(s.PlayerBook, s.Save.Variables)
			if histCount == 0 && curScene != nil && curScene.Effects != "" {
				book.ApplyEffects(curScene.Effects, s.Save.Variables, s.PlayerBook)
			}
		} else {
			s.Save.Variables = book.EnsureSaveVariables(s.PlayerBook, s.Save.Variables)
		}
	}
	var curVars map[string]string
	if s.Save != nil {
		curVars = s.Save.Variables
	} else {
		curVars = book.DefaultVariablesMap(s.PlayerBook)
	}
	vars["has_history"] = histCount > 0

	lastImageURL := ""
	imageless := s.isEffectiveImageless()
	if histCount > 0 {
		histList := make([]map[string]any, 0, histCount)
		for idx, entry := range s.Save.History {
			pastScene := s.findPlayerScene(entry.SceneID)
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
			pastHeaderMeta := s.PlayerBook.Title
			if pastScene.Chapter != "" {
				pastHeaderMeta = s.PlayerBook.Title + " • " + pastScene.Chapter
			}
			pastHasTitle := strings.TrimSpace(pastScene.Name) != ""
			pastVars := entry.Variables
			if len(pastVars) == 0 {
				pastVars = curVars
			}
			var pastBlocks []map[string]any
			if isScreenplay {
				pastBlocks = buildPlayerScreenplayBlocks(pastScene, pastVars, s.PlayerBook, pastScene.OrderIndex+1, normStd)
			}
			pastProse := book.EvaluateSceneProseText(pastScene, pastVars, s.PlayerBook)
			pastAudioURL, pastAudioDur, pastTrimStart, pastTrimEnd, pastAudioQueue := resolvePlayerSceneAudio(s.PlayerBook, pastScene, pastVars)
			histList = append(histList, map[string]any{
				"idx":                  idx,
				"header_meta":          pastHeaderMeta,
				"name":                 pastScene.Name,
				"slugline":             pastScene.Slugline,
				"rail_title":           pastScene.DisplayName(),
				"has_title":            pastHasTitle,
				"prose":                pastProse,
				"screenplay_blocks":    pastBlocks,
				"has_image":            pastImgURL != "",
				"image_url":            pastImgURL,
				"bg_image_url":         lastImageURL,
				"audio_url":            pastAudioURL,
				"audio_queue":          pastAudioQueue,
				"audio_duration_label": pastAudioDur,
				"audio_trim_start":     pastTrimStart,
				"audio_trim_end":       pastTrimEnd,
			})
		}
		vars["history_items"] = histList
	}

	hasMultipleChoices := len(curScene.Choices) > 1
	curChoices := make([]map[string]any, 0, len(curScene.Choices))
	for idx, ch := range curScene.Choices {
		condMet := book.EvaluateCondition(ch.Condition, curVars, s.PlayerBook)
		if !condMet && !ch.ShowWhenLocked {
			continue
		}
		label := ch.Label
		if !hasMultipleChoices {
			label = s.tr(TrNext)
		}
		arrow := "»"
		if ch.IsEnding {
			arrow = s.tr(TrEndUpper)
		}
		isImplicitNext := !hasMultipleChoices && !ch.IsEnding && ch.TargetSceneID != "END"
		curChoices = append(curChoices, map[string]any{
			"idx":         idx,
			"label":       label,
			"arrow":       arrow,
			"is_branch":   hasMultipleChoices && !ch.IsEnding,
			"is_implicit": isImplicitNext,
			"is_locked":   !condMet && ch.ShowWhenLocked,
		})
	}

	curImgURL := ""
	if !imageless && curScene.ImageURL != "" {
		curImgURL = curScene.ImageURL
		lastImageURL = curScene.ImageURL
	}
	curHeaderMeta := s.PlayerBook.Title
	if curScene.Chapter != "" {
		curHeaderMeta = s.PlayerBook.Title + " • " + curScene.Chapter
	}
	curHasTitle := strings.TrimSpace(curScene.Name) != ""
	var curBlocks []map[string]any
	if isScreenplay {
		curBlocks = buildPlayerScreenplayBlocks(curScene, curVars, s.PlayerBook, curScene.OrderIndex+1, normStd)
	}
	curProse := book.EvaluateSceneProseText(curScene, curVars, s.PlayerBook)
	curAudioURL, curAudioDur, curTrimStart, curTrimEnd, curAudioQueue := resolvePlayerSceneAudio(s.PlayerBook, curScene, curVars)

	vars["current_scene"] = map[string]any{
		"name":                 curScene.Name,
		"slugline":             curScene.Slugline,
		"rail_title":           curScene.DisplayName(),
		"has_title":            curHasTitle,
		"header_meta":          curHeaderMeta,
		"prose":                curProse,
		"screenplay_blocks":    curBlocks,
		"has_image":            curImgURL != "",
		"image_url":            curImgURL,
		"has_bg_image":         !imageless && lastImageURL != "",
		"bg_image_url":         lastImageURL,
		"audio_url":            curAudioURL,
		"audio_queue":          curAudioQueue,
		"audio_duration_label": curAudioDur,
		"audio_trim_start":     curTrimStart,
		"audio_trim_end":       curTrimEnd,
		"is_terminal":          len(curScene.Choices) == 0 || curScene.ExplicitEnd,
		"choices":              curChoices,
	}

	narrated := s.PlayerBook.Narrated
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
		target := s.findPlayerScene(ch.TargetSceneID)
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
			target := s.findPlayerScene(ch.TargetSceneID)
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
	if len(s.PlayerBook.Scenes) > 0 {
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
		progressPct = (len(visited) * 100) / len(s.PlayerBook.Scenes)
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
		vars["auto_choice_label"] = s.tr(TrAutoChoiceOnLabel)
	} else {
		vars["auto_choice_label"] = s.tr(TrAutoChoiceOffLabel)
	}
	vars["auto_choice_delay"] = fmt.Sprintf("%.2g", book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay))
}
