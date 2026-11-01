package gui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"storyteller/internal/book"
)

func (s *Server) terminalChoiceDescriptor() book.Choice {
	return book.Choice{
		Target:        "END",
		TargetSceneID: "END",
		IsEnding:      true,
	}
}

func (s *Server) buildChoiceTargetOptions(sc *book.Scene, ch book.Choice) (string, string, []map[string]any) {
	selectedVal := "END"
	selectedLabel := "KONIEC (END)"
	if s.ActiveBook == nil {
		return selectedVal, selectedLabel, nil
	}
	curCh := ""
	if sc != nil {
		curCh = strings.TrimSpace(sc.Chapter)
	}
	if curCh == "" {
		curCh = "Rozdział 1"
	}

	targetOptions := make([]map[string]any, 0, len(s.ActiveBook.Scenes)+len(s.ActiveBook.Chapters)+2)
	matchedOption := false

	// 1. Same-chapter scenes
	for _, cand := range s.ActiveBook.Scenes {
		candCh := strings.TrimSpace(cand.Chapter)
		if candCh == "" {
			candCh = "Rozdział 1"
		}
		if !strings.EqualFold(candCh, curCh) {
			continue
		}
		if sc != nil && cand.ID == sc.ID && (ch.IsEnding || ch.IsChapterLink || ch.TargetSceneID != cand.ID) {
			continue
		}
		isSel := !ch.IsEnding && !ch.IsChapterLink && ch.TargetSceneID == cand.ID
		if isSel {
			matchedOption = true
		}
		optVal := cand.ReferenceKey()
		optLabel := cand.DisplayName()
		if cand.ExplicitID != "" {
			if strings.EqualFold(ch.Target, cand.ExplicitID) || isSel {
				optVal = cand.ExplicitID
			}
			if cand.Name != "" {
				optLabel = fmt.Sprintf("%s [id: %s]", cand.Name, cand.ExplicitID)
			} else {
				optLabel = fmt.Sprintf("[id: %s]", cand.ExplicitID)
			}
		}
		if isSel {
			selectedVal = optVal
			selectedLabel = cand.DisplayName()
		}
		targetOptions = append(targetOptions, map[string]any{
			"value":       optVal,
			"short_label": cand.DisplayName(),
			"label":       optLabel,
			"is_active":   isSel,
		})
	}

	// 2. Forward chapters (one-way chapter transitions: chapter:<id>)
	for _, chItem := range s.ActiveBook.Chapters {
		chRef := chItem.ReferenceKey()
		optVal := "chapter:" + chRef
		isCurrentSel := !ch.IsEnding && ch.IsChapterLink &&
			(strings.EqualFold(ch.TargetChapterID, chRef) ||
				strings.EqualFold(ch.TargetChapterID, chItem.ID) ||
				strings.EqualFold(ch.Target, optVal))
		canLink, _ := s.ActiveBook.CanLinkChapters(curCh, chRef)
		if !canLink && !isCurrentSel {
			continue
		}
		if isCurrentSel {
			matchedOption = true
		}
		shortLbl := "↪ Rozdział: " + chItem.DisplayName()
		optLabel := shortLbl
		if chItem.ExplicitID != "" && !strings.EqualFold(chItem.DisplayName(), chItem.ExplicitID) {
			optLabel = fmt.Sprintf("↪ Rozdział: %s [chapter:%s]", chItem.DisplayName(), chItem.ExplicitID)
		}
		if isCurrentSel {
			selectedVal = optVal
			selectedLabel = shortLbl
		}
		targetOptions = append(targetOptions, map[string]any{
			"value":       optVal,
			"short_label": shortLbl,
			"label":       optLabel,
			"is_active":   isCurrentSel,
		})
	}

	// 3. END
	isEndSel := ch.IsEnding || strings.EqualFold(ch.Target, "END")
	if isEndSel {
		matchedOption = true
		selectedVal = "END"
		selectedLabel = "KONIEC (END)"
	}
	targetOptions = append(targetOptions, map[string]any{
		"value":       "END",
		"short_label": "KONIEC (END)",
		"label":       "KONIEC (END)",
		"is_active":   isEndSel,
	})
	if !matchedOption && ch.Target != "" {
		selectedVal = ch.Target
		selectedLabel = "⚠ " + ch.Target
		targetOptions = append(targetOptions, map[string]any{
			"value":       ch.Target,
			"short_label": "⚠ " + ch.Target,
			"label":       "⚠ " + ch.Target,
			"is_active":   true,
		})
	}
	return selectedVal, selectedLabel, targetOptions
}

func (s *Server) buildSceneMediaOptions(sc *book.Scene, buildMenus bool) (curImg, curImgLabel string, imgOptions []map[string]any, curVoice, curVoiceLabel string, voiceOptions []map[string]any) {
	curImg = sc.ExplicitImage
	if curImg == "" && sc.ImageFile != "" {
		curImg = sc.ImageFile
	}
	curImgLabel = "— brak —"
	if curImg != "" {
		curImgLabel = curImg
	}
	if buildMenus && s.ActiveBook != nil {
		imgOptions = make([]map[string]any, 0, len(s.ActiveBook.ImageFiles)+2)
		imgOptions = append(imgOptions, map[string]any{
			"value":     "",
			"label":     "— brak —",
			"is_active": curImg == "",
		})
		matchedImg := curImg == ""
		for _, imgFile := range s.ActiveBook.ImageFiles {
			isAct := strings.EqualFold(curImg, imgFile)
			if isAct {
				matchedImg = true
				curImgLabel = imgFile
			}
			imgOptions = append(imgOptions, map[string]any{
				"value":     imgFile,
				"label":     imgFile,
				"is_active": isAct,
			})
		}
		if !matchedImg && curImg != "" {
			curImgLabel = "⚠ " + curImg
			imgOptions = append(imgOptions, map[string]any{
				"value":     curImg,
				"label":     "⚠ " + curImg,
				"is_active": true,
			})
		}
		imgOptions = append(imgOptions, map[string]any{
			"value":     "__browse_disk__",
			"label":     "+ Dodaj obraz z dysku...",
			"is_active": false,
		})
	}

	curVoice = ""
	curVoiceLabel = "— brak —"
	if s.ActiveBook != nil && s.ActiveBook.Narrated {
		curVoice = sc.ExplicitVoice
		if curVoice == "" && sc.AudioFile != "" {
			curVoice = sc.AudioFile
		}
		if curVoice != "" {
			curVoiceLabel = curVoice
		}
		if buildMenus {
			voiceOptions = make([]map[string]any, 0, len(s.ActiveBook.AudioFiles)+3)
			voiceOptions = append(voiceOptions, map[string]any{
				"value":     "",
				"label":     "— brak —",
				"is_active": curVoice == "",
			})
			matchedVoice := curVoice == ""
			for _, audioFile := range s.ActiveBook.AudioFiles {
				isAct := strings.EqualFold(curVoice, audioFile)
				if isAct {
					matchedVoice = true
					curVoiceLabel = audioFile
				}
				voiceOptions = append(voiceOptions, map[string]any{
					"value":     audioFile,
					"label":     audioFile,
					"is_active": isAct,
				})
			}
			if !matchedVoice && curVoice != "" {
				curVoiceLabel = "⚠ " + curVoice
				voiceOptions = append(voiceOptions, map[string]any{
					"value":     curVoice,
					"label":     "⚠ " + curVoice,
					"is_active": true,
				})
			}
			voiceOptions = append(voiceOptions, map[string]any{
				"value":     "__browse_disk__",
				"label":     "+ Dodaj nagranie z dysku...",
				"is_active": false,
			})
		}
	}
	return
}

func (s *Server) buildEditorNodeMap(sc *book.Scene, isSelected bool) map[string]any {
	isStart := false
	isChapterEntry := false
	if s.ActiveBook != nil {
		if s.ActiveBook.StartSceneID != "" {
			isStart = strings.EqualFold(sc.ID, s.ActiveBook.StartSceneID)
		} else if len(s.ActiveBook.Scenes) > 0 {
			isStart = sc.ID == s.ActiveBook.Scenes[0].ID
		}
		isChapterEntry = isStart
		if !isChapterEntry {
			if chMeta := s.ActiveBook.FindChapter(sc.Chapter); chMeta != nil && chMeta.StartSceneID != "" {
				isChapterEntry = strings.EqualFold(sc.ID, chMeta.StartSceneID)
			}
		}
	}

	hasChoices := len(sc.Choices) >= 2
	ports := make([]map[string]any, 0, len(sc.Choices))
	if hasChoices {
		for chIdx, ch := range sc.Choices {
			lbl := ch.Label
			tgt := ch.Target
			targetChapterRef := ""
			if ch.IsEnding {
				tgt = "KONIEC"
			} else if ch.IsChapterLink && ch.TargetChapterID != "" {
				targetChapterRef = ch.TargetChapterID
				if s.ActiveBook != nil {
					if tCh := s.ActiveBook.FindChapter(ch.TargetChapterID); tCh != nil {
						tgt = "↪ " + tCh.DisplayName()
						targetChapterRef = tCh.ReferenceKey()
					}
				}
			} else if tSc := s.findScene(ch.TargetSceneID); tSc != nil {
				tgt = tSc.DisplayName()
			}

			var targetOptions []map[string]any
			selectedVal := "END"
			selectedLabel := "KONIEC (END)"
			if isSelected {
				selectedVal, selectedLabel, targetOptions = s.buildChoiceTargetOptions(sc, ch)
			}
			isFilled := !ch.IsEnding &&
				((ch.TargetSceneID != "" && !strings.EqualFold(ch.TargetSceneID, "END")) ||
					(ch.IsChapterLink && ch.TargetChapterID != "")) &&
				(!isSelected || !strings.EqualFold(selectedVal, "END"))

			ports = append(ports, map[string]any{
				"idx":            chIdx,
				"raw_label":      ch.Label,
				"label":          lbl,
				"target":         tgt,
				"target_id":      ch.TargetSceneID,
				"target_chapter": targetChapterRef,
				"selected_value": selectedVal,
				"selected_label": selectedLabel,
				"is_filled":      isFilled,
				"target_options": targetOptions,
			})
		}
	}
	implicitTargetID := ""
	implicitTargetChapter := ""
	if !hasChoices && len(sc.Choices) == 1 && !sc.Choices[0].IsEnding {
		implicitTargetID = sc.Choices[0].TargetSceneID
		if sc.Choices[0].IsChapterLink && sc.Choices[0].TargetChapterID != "" {
			implicitTargetChapter = sc.Choices[0].TargetChapterID
			if s.ActiveBook != nil {
				if tCh := s.ActiveBook.FindChapter(implicitTargetChapter); tCh != nil {
					implicitTargetChapter = tCh.ReferenceKey()
				}
			}
		}
	}

	curImg, curImgLabel, imgOptions, curVoice, curVoiceLabel, voiceOptions := s.buildSceneMediaOptions(sc, isSelected)

	nodeWarns := make([]map[string]any, 0, len(sc.Warnings))
	for _, w := range sc.Warnings {
		nodeWarns = append(nodeWarns, map[string]any{"text": w})
	}

	curChName := strings.TrimSpace(sc.Chapter)
	if curChName == "" {
		if s.ActiveBook != nil && s.ActiveBook.IsScreenplay() {
			curChName = "Akt I"
		} else {
			curChName = "Rozdział 1"
		}
	}
	var chapterOptions []map[string]any
	if isSelected {
		chapterOptions = make([]map[string]any, 0, len(s.ActiveBook.Chapters)+1)
		matchedCh := false
		for _, chItem := range s.ActiveBook.Chapters {
			isAct := strings.EqualFold(curChName, strings.TrimSpace(chItem.Name))
			if isAct {
				matchedCh = true
			}
			chapterOptions = append(chapterOptions, map[string]any{
				"value":     chItem.Name,
				"label":     chItem.Name,
				"is_active": isAct,
			})
		}
		if !matchedCh && curChName != "" {
			chapterOptions = append(chapterOptions, map[string]any{
				"value":     curChName,
				"label":     curChName,
				"is_active": true,
			})
		}
	}

	scriptLengthLabel := ""
	if s.ActiveBook != nil && s.ActiveBook.IsScreenplay() {
		metrics := book.ComputeSceneScriptMetrics(sc, sc.OrderIndex+1, s.ActiveBook.NormalizedStandard())
		scriptLengthLabel = metrics.FormattedLength
	}

	return map[string]any{
		"id":                      sc.ID,
		"explicit_id":             sc.ExplicitID,
		"current_image":           curImg,
		"current_image_label":     curImgLabel,
		"image_options":           imgOptions,
		"current_voice":           curVoice,
		"current_voice_label":     curVoiceLabel,
		"voice_options":           voiceOptions,
		"name":                    sc.Name,
		"slugline":                sc.Slugline,
		"has_slugline":            strings.TrimSpace(sc.Slugline) != "",
		"synopsis":                sc.Synopsis,
		"has_synopsis":            strings.TrimSpace(sc.Synopsis) != "",
		"script_length_label":     scriptLengthLabel,
		"chapter":                 curChName,
		"chapter_options":         chapterOptions,
		"x":                       sc.X,
		"y":                       sc.Y,
		"first_sentence":          extractFirstSentence(sc.Text),
		"source_file":             sc.SourceFile,
		"is_selected":             isSelected,
		"is_start_scene":          isStart,
		"is_chapter_entry":        isChapterEntry,
		"is_unreachable":          sc.Unreachable,
		"has_node_warnings":       len(sc.Warnings) > 0,
		"node_warnings":           nodeWarns,
		"ports":                   ports,
		"has_choices":             hasChoices,
		"port_count":              len(ports),
		"implicit_target_id":      implicitTargetID,
		"implicit_target_chapter": implicitTargetChapter,
	}
}

func buildBookModeOptions(activeMode string) (string, []map[string]any) {
	norm := book.NormalizeBookMode(activeMode)
	label := "Książka interaktywna"
	if norm == "screenplay" {
		label = "Scenariusz filmowy"
	}
	return label, []map[string]any{
		{"value": "book", "label": "Książka interaktywna", "is_active": norm == "book"},
		{"value": "screenplay", "label": "Scenariusz filmowy", "is_active": norm == "screenplay"},
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

func (s *Server) populateEditorBookAndChapterSettings(vars map[string]any) {
	if s.ActiveBook == nil {
		vars["book_settings_open"] = false
		vars["has_selected_chapter"] = false
		vars["is_node_inspector_hidden"] = false
		vars["multi_scene_items"] = nil
		vars["chapter_noun"] = "Rozdział"
		vars["add_chapter_label"] = "+ Rozdział"
		return
	}

	isScreenplay := s.ActiveBook.IsScreenplay()
	normMode := s.ActiveBook.NormalizedMode()
	normStd := s.ActiveBook.NormalizedStandard()
	modeLabel, modeOpts := buildBookModeOptions(normMode)
	stdLabel, stdOpts := buildScriptStandardOptions(normStd)
	prefixPills, timePills := buildSluglineHelperPills(normStd)

	vars["is_screenplay_mode"] = isScreenplay
	vars["book_mode"] = normMode
	vars["book_mode_label"] = modeLabel
	vars["book_mode_options"] = modeOpts
	vars["script_standard"] = normStd
	vars["script_standard_label"] = stdLabel
	vars["script_standard_options"] = stdOpts
	vars["slugline_prefix_pills"] = prefixPills
	vars["slugline_time_pills"] = timePills
	if isScreenplay {
		vars["chapter_noun"] = "Akt"
		vars["add_chapter_label"] = "+ Akt"
	} else {
		vars["chapter_noun"] = "Rozdział"
		vars["add_chapter_label"] = "+ Rozdział"
	}

	multiSceneItems := make([]map[string]any, 0, len(s.ActiveBook.Scenes))
	for _, sc := range s.ActiveBook.Scenes {
		scCh := strings.TrimSpace(sc.Chapter)
		if scCh == "" {
			scCh = "Rozdział 1"
		}
		multiSceneItems = append(multiSceneItems, map[string]any{
			"id":      sc.ID,
			"title":   sc.DisplayName(),
			"chapter": scCh,
		})
	}
	vars["multi_scene_items"] = multiSceneItems

	vars["book_settings_open"] = s.BookSettingsOpen
	vars["active_book_author"] = s.ActiveBook.Author
	vars["active_book_description"] = s.ActiveBook.Description

	startSceneID := s.ActiveBook.StartSceneID
	if startSceneID == "" && len(s.ActiveBook.Scenes) > 0 {
		startSceneID = s.ActiveBook.Scenes[0].ID
	}
	startScene := s.findScene(startSceneID)
	if startScene == nil && len(s.ActiveBook.Scenes) > 0 {
		startScene = s.ActiveBook.Scenes[0]
		startSceneID = startScene.ID
	}
	startChapterName := "Rozdział 1"
	if startScene != nil && strings.TrimSpace(startScene.Chapter) != "" {
		startChapterName = strings.TrimSpace(startScene.Chapter)
	} else if len(s.ActiveBook.Chapters) > 0 && strings.TrimSpace(s.ActiveBook.Chapters[0].Name) != "" {
		startChapterName = strings.TrimSpace(s.ActiveBook.Chapters[0].Name)
	}

	startChapterOptions := make([]map[string]any, 0, len(s.ActiveBook.Chapters))
	for _, ch := range s.ActiveBook.Chapters {
		chName := strings.TrimSpace(ch.Name)
		if chName == "" {
			continue
		}
		isActCh := strings.EqualFold(chName, startChapterName)
		startChapterOptions = append(startChapterOptions, map[string]any{
			"value":     chName,
			"label":     ch.DisplayName(),
			"is_active": isActCh,
		})
	}
	vars["book_start_chapter_name"] = startChapterName
	vars["book_start_chapter_options"] = startChapterOptions

	startSceneLabel := ""
	startSceneOptions := make([]map[string]any, 0, len(s.ActiveBook.Scenes))
	for _, sc := range s.ActiveBook.Scenes {
		scCh := strings.TrimSpace(sc.Chapter)
		if scCh == "" {
			scCh = "Rozdział 1"
		}
		if !strings.EqualFold(scCh, startChapterName) {
			continue
		}
		isStart := strings.EqualFold(sc.ID, startSceneID)
		optLabel := sc.DisplayName()
		if isStart {
			startSceneLabel = optLabel
		}
		startSceneOptions = append(startSceneOptions, map[string]any{
			"value":     sc.ID,
			"label":     optLabel,
			"is_active": isStart,
		})
	}
	if startSceneLabel == "" && len(startSceneOptions) > 0 {
		startSceneLabel, _ = startSceneOptions[0]["label"].(string)
	}
	vars["book_start_scene_id"] = startSceneID
	vars["book_start_scene_label"] = startSceneLabel
	vars["book_start_scene_options"] = startSceneOptions

	var selCh *book.Chapter
	if s.SelectedChapter != "" {
		selCh = s.ActiveBook.FindChapter(s.SelectedChapter)
	}
	hasSelectedChapter := selCh != nil && s.SelectedChapter != "" && !s.BookSettingsOpen
	if selCh == nil && len(s.ActiveBook.Chapters) > 0 {
		selCh = &s.ActiveBook.Chapters[0]
	}

	if selCh != nil {
		chName := strings.TrimSpace(selCh.Name)
		if chName == "" {
			chName = "Rozdział 1"
		}
		var chScenes []*book.Scene
		for _, sc := range s.ActiveBook.Scenes {
			scCh := strings.TrimSpace(sc.Chapter)
			if scCh == "" {
				scCh = "Rozdział 1"
			}
			if strings.EqualFold(scCh, chName) {
				chScenes = append(chScenes, sc)
			}
		}
		chStartID := selCh.StartSceneID
		if chStartID == "" && len(chScenes) > 0 {
			chStartID = chScenes[0].ID
		}
		chStartLabel := ""
		chStartOptions := make([]map[string]any, 0, len(chScenes))
		for _, sc := range chScenes {
			isChStart := strings.EqualFold(sc.ID, chStartID)
			optLabel := sc.DisplayName()
			if isChStart {
				chStartLabel = optLabel
			}
			chStartOptions = append(chStartOptions, map[string]any{
				"value":     sc.ID,
				"label":     optLabel,
				"is_active": isChStart,
			})
		}
		if chStartLabel == "" && len(chStartOptions) > 0 {
			chStartLabel, _ = chStartOptions[0]["label"].(string)
		}
		vars["selected_chapter"] = map[string]any{
			"name":                chName,
			"start_scene_id":      chStartID,
			"start_scene_label":   chStartLabel,
			"start_scene_options": chStartOptions,
		}
	}
	vars["has_selected_chapter"] = hasSelectedChapter
	vars["is_node_inspector_hidden"] = s.BookSettingsOpen || hasSelectedChapter
}

func (s *Server) populateEditorSelectedNodeOnly(vars map[string]any) {
	selected := s.findScene(s.SelectedSceneID)
	if selected == nil {
		selected = s.ActiveBook.Scenes[0]
		s.SelectedSceneID = selected.ID
	}
	selectedNodeMap := s.buildEditorNodeMap(selected, true)
	vars["selected_node"] = selectedNodeMap
	vars["has_selected_node"] = selectedNodeMap != nil
	s.populateEditorBookAndChapterSettings(vars)
}

func (s *Server) populateEditorContext(vars map[string]any) {
	s.populateEditorBookAndChapterSettings(vars)
	selected := s.findScene(s.SelectedSceneID)
	if selected == nil {
		selected = s.ActiveBook.Scenes[0]
		s.SelectedSceneID = selected.ID
	}

	orderedScenes := make([]*book.Scene, len(s.ActiveBook.Scenes))
	copy(orderedScenes, s.ActiveBook.Scenes)
	sort.SliceStable(orderedScenes, func(i, j int) bool {
		ri, rj := max(orderedScenes[i].Row, 1), max(orderedScenes[j].Row, 1)
		if ri != rj {
			return ri < rj
		}
		return orderedScenes[i].Col < orderedScenes[j].Col
	})

	allNodes := make([]map[string]any, 0, len(orderedScenes))
	var selectedNodeMap map[string]any
	for _, sc := range orderedScenes {
		isSel := sc.ID == selected.ID
		nodeMap := s.buildEditorNodeMap(sc, isSel)
		if isSel {
			selectedNodeMap = nodeMap
		}
		allNodes = append(allNodes, nodeMap)
	}
	vars["all_nodes"] = allNodes
	if selectedNodeMap == nil && len(allNodes) > 0 {
		selectedNodeMap = allNodes[0]
	}
	vars["selected_node"] = selectedNodeMap
	vars["has_selected_node"] = selectedNodeMap != nil

	chapterGroups := make([]map[string]any, 0, len(s.ActiveBook.Chapters))
	for chIdx, chItem := range s.ActiveBook.Chapters {
		chName := strings.TrimSpace(chItem.Name)
		if chName == "" {
			continue
		}
		var chScenes []*book.Scene
		for _, sc := range s.ActiveBook.Scenes {
			scCh := strings.TrimSpace(sc.Chapter)
			if scCh == "" {
				scCh = "Rozdział 1"
			}
			if strings.EqualFold(scCh, chName) {
				chScenes = append(chScenes, sc)
			}
		}
		if len(chScenes) == 0 {
			continue
		}
		minX, maxX := chScenes[0].X, chScenes[0].X+192
		minY, maxY := chScenes[0].Y, chScenes[0].Y+120
		for _, sc := range chScenes[1:] {
			if sc.X < minX {
				minX = sc.X
			}
			if sc.X+192 > maxX {
				maxX = sc.X + 192
			}
			if sc.Y < minY {
				minY = sc.Y
			}
			if sc.Y+120 > maxY {
				maxY = sc.Y + 120
			}
		}
		count := len(chScenes)
		sceneWord := "scen"
		if count == 1 {
			sceneWord = "scena"
		} else if count%10 >= 2 && count%10 <= 4 && (count%100 < 12 || count%100 > 14) {
			sceneWord = "sceny"
		}
		chBadge := "ROZDZIAŁ"
		if s.ActiveBook.IsScreenplay() {
			chBadge = "AKT"
		}
		if chItem.ExplicitID != "" {
			chBadge += " • " + chItem.ExplicitID
		} else if chItem.ID != "" {
			chBadge += " • " + chItem.ID
		}
		chRef := chItem.ReferenceKey()
		isRootChapter := chIdx == 0
		if s.ActiveBook.StartSceneID != "" {
			isRootChapter = false
			for _, sc := range chScenes {
				if strings.EqualFold(sc.ID, s.ActiveBook.StartSceneID) {
					isRootChapter = true
					break
				}
			}
		}
		var outputPorts []map[string]any
		seenTargets := make(map[string]bool)
		for _, sc := range chScenes {
			for _, ch := range sc.Choices {
				if ch.IsEnding || !ch.IsChapterLink || ch.TargetChapterID == "" {
					continue
				}
				tCh := s.ActiveBook.FindChapter(ch.TargetChapterID)
				if tCh == nil {
					continue
				}
				tRef := tCh.ReferenceKey()
				tKey := strings.ToLower(tRef)
				if seenTargets[tKey] {
					continue
				}
				seenTargets[tKey] = true
				outputPorts = append(outputPorts, map[string]any{
					"idx":         len(outputPorts),
					"target_ref":  tRef,
					"target_name": tCh.DisplayName(),
					"port_class":  "port-filled",
					"title":       fmt.Sprintf("Wyjście rozdziału do: %s (przeciągnij, aby zmienić lub rozłączyć)", tCh.DisplayName()),
				})
			}
		}
		if len(outputPorts) == 0 {
			outputPorts = append(outputPorts, map[string]any{
				"idx":         0,
				"target_ref":  "",
				"target_name": "",
				"port_class":  "",
				"title":       fmt.Sprintf("Port wyjściowy rozdziału „%s” — przeciągnij przewód ze sceny tutaj lub stąd do kolejnego rozdziału", chName),
			})
		}
		startSceneID := chItem.StartSceneID
		if startSceneID == "" && len(chScenes) > 0 {
			startSceneID = chScenes[0].ID
		}
		chapterGroups = append(chapterGroups, map[string]any{
			"idx":            chIdx + 1,
			"id":             chItem.ID,
			"ref":            chRef,
			"badge":          chBadge,
			"name":           chName,
			"is_root":        isRootChapter,
			"start_scene_id": startSceneID,
			"output_ports":   outputPorts,
			"scene_count":    count,
			"scene_word":     sceneWord,
			"x":              minX - 24,
			"y":              minY - 52,
			"w":              (maxX - minX) + 48,
			"h":              (maxY - minY) + 96,
			"next_x":         book.SnapToGrid(minX),
			"next_y":         book.SnapToGrid(maxY + 120),
		})
	}
	vars["chapter_groups"] = chapterGroups
	edZoom := s.ActiveBook.EditorZoom
	if edZoom < 0.2 || edZoom > 3.0 {
		edZoom = 1.0
	}
	vars["studio_graph_payload"] = buildStudioGraphPayload(s.ActiveBook, edZoom, chapterGroups, allNodes)

	s.populateEditorBreakdownContext(vars)
	s.populateEditorHistoryContext(vars)
}

func (s *Server) populateEditorBreakdownContext(vars map[string]any) {
	vars["show_editor_breakdown_panel"] = s.EditorBreakdownPanelOpen
	if s.ActiveBook == nil {
		vars["screenplay_toolbar_summary"] = ""
		vars["has_breakdown_characters"] = false
		vars["has_breakdown_locations"] = false
		return
	}
	bd := book.AnalyzeScreenplay(s.ActiveBook)
	vars["screenplay_toolbar_summary"] = bd.ToolbarSummary
	vars["breakdown_total_scenes"] = bd.TotalScenes
	vars["breakdown_total_pages"] = bd.TotalPagesFormatted
	vars["breakdown_runtime"] = bd.EstimatedRuntimeLabel
	vars["breakdown_shoot_days"] = bd.EstimatedShootDays
	vars["breakdown_int_ext_label"] = fmt.Sprintf("%d (%d%%) / %d (%d%%)", bd.IntScenes, bd.IntPct, bd.ExtScenes, bd.ExtPct)
	vars["breakdown_day_night_label"] = fmt.Sprintf("%d (%d%%) / %d (%d%%)", bd.DayScenes, bd.DayPct, bd.NightScenes, bd.NightPct)

	actItems := make([]map[string]any, 0, len(bd.Acts))
	for _, a := range bd.Acts {
		actItems = append(actItems, map[string]any{
			"id":               a.ID,
			"name":             a.Name,
			"scene_count":      a.SceneCount,
			"formatted_length": a.FormattedLength,
			"share_pct":        a.SharePct,
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
			"locations_label":     strings.Join(ch.LocationsVisited, ", "),
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

func sanitizeGraphPayloadField(v string) string {
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.ReplaceAll(v, "|", "¦")
	return strings.TrimSpace(v)
}

func boolToPayloadBit(b bool) int {
	if b {
		return 1
	}
	return 0
}

func buildStudioGraphPayload(b *book.Book, edZoom float64, chapterGroups, allNodes []map[string]any) string {
	if b == nil {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(allNodes)*160 + len(chapterGroups)*120 + 64)
	fmt.Fprintf(&sb, "viewport | pan_x:%d | pan_y:%d | zoom:%.2f | has_view:%d\n",
		b.EditorPanX, b.EditorPanY, edZoom, boolToPayloadBit(b.HasEditorView))

	for _, chg := range chapterGroups {
		chID, _ := chg["id"].(string)
		chRef, _ := chg["ref"].(string)
		chName, _ := chg["name"].(string)
		isRoot, _ := chg["is_root"].(bool)
		startSceneID, _ := chg["start_scene_id"].(string)
		x, _ := chg["x"].(int)
		y, _ := chg["y"].(int)
		w, _ := chg["w"].(int)
		h, _ := chg["h"].(int)

		var outRefs []string
		if outPorts, ok := chg["output_ports"].([]map[string]any); ok {
			for _, op := range outPorts {
				if tRef, _ := op["target_ref"].(string); tRef != "" {
					outRefs = append(outRefs, sanitizeGraphPayloadField(tRef))
				}
			}
		}
		fmt.Fprintf(&sb, "chapter | id:%s | ref:%s | name:%s | root:%d | start:%s | x:%d | y:%d | w:%d | h:%d | outs:%s\n",
			sanitizeGraphPayloadField(chID),
			sanitizeGraphPayloadField(chRef),
			sanitizeGraphPayloadField(chName),
			boolToPayloadBit(isRoot),
			sanitizeGraphPayloadField(startSceneID),
			x, y, w, h,
			strings.Join(outRefs, ","))
	}

	for _, nd := range allNodes {
		id, _ := nd["id"].(string)
		name, _ := nd["name"].(string)
		if name == "" {
			name = id
		}
		chapter, _ := nd["chapter"].(string)
		x, _ := nd["x"].(int)
		y, _ := nd["y"].(int)
		isSelected, _ := nd["is_selected"].(bool)
		isStart, _ := nd["is_start_scene"].(bool)
		isEntry, _ := nd["is_chapter_entry"].(bool)
		isUnreachable, _ := nd["is_unreachable"].(bool)
		hasChoices, _ := nd["has_choices"].(bool)
		impTarget, _ := nd["implicit_target_id"].(string)
		impChapter, _ := nd["implicit_target_chapter"].(string)

		fmt.Fprintf(&sb, "node | id:%s | name:%s | chapter:%s | x:%d | y:%d | selected:%d | start:%d | entry:%d | unreachable:%d | has_choices:%d | imp_target:%s | imp_chapter:%s\n",
			sanitizeGraphPayloadField(id),
			sanitizeGraphPayloadField(name),
			sanitizeGraphPayloadField(chapter),
			x, y,
			boolToPayloadBit(isSelected),
			boolToPayloadBit(isStart),
			boolToPayloadBit(isEntry),
			boolToPayloadBit(isUnreachable),
			boolToPayloadBit(hasChoices),
			sanitizeGraphPayloadField(impTarget),
			sanitizeGraphPayloadField(impChapter))

		if ports, ok := nd["ports"].([]map[string]any); ok {
			for _, pt := range ports {
				idx, _ := pt["idx"].(int)
				lbl, _ := pt["label"].(string)
				targetID, _ := pt["target_id"].(string)
				targetChapter, _ := pt["target_chapter"].(string)
				fmt.Fprintf(&sb, "port | scene:%s | idx:%d | label:%s | target_id:%s | target_chapter:%s\n",
					sanitizeGraphPayloadField(id),
					idx,
					sanitizeGraphPayloadField(lbl),
					sanitizeGraphPayloadField(targetID),
					sanitizeGraphPayloadField(targetChapter))
			}
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func formatHistoryTimestamp(ts int64) string {
	if ts <= 0 {
		return "—"
	}
	t := time.Unix(ts, 0)
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04:05")
	}
	return t.Format("2006-01-02 15:04")
}

func formatHistoryCompactTime(ts int64) string {
	if ts <= 0 {
		return "—"
	}
	t := time.Unix(ts, 0)
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("01-02 15:04")
}

func normalizeGraphBranchName(raw string) string {
	br := book.SanitizeBookID(strings.TrimSpace(raw))
	if br == "" || br == "book" {
		return "main"
	}
	return br
}

func buildHistoryGraphRows(
	b *book.Book,
	h *book.BookHistory,
	curIdx, viewedIdx int,
	isHistPreview bool,
) ([]map[string]any, map[string]string) {
	revHints, branchHints := book.BuildHistoryHoverSummaries(b, h)
	if h == nil || len(h.Revisions) == 0 {
		nowStr := formatHistoryCompactTime(time.Now().Unix())
		scenesCount := 0
		if b != nil {
			scenesCount = len(b.Scenes)
		}
		defHint := fmt.Sprintf("main • %d scen", scenesCount)
		branchHints["main"] = defHint
		row := map[string]any{
			"id":                "r1",
			"label":             "Stan początkowy",
			"time_label":        nowStr,
			"meta_label":        "r1 • " + nowStr,
			"branch":            "main",
			"hover_hint":        defHint,
			"node_action":       "PreviewHistoryRevision",
			"is_unnamed_step":   false,
			"is_current":        true,
			"is_active_head":    true,
			"is_preview":        false,
			"is_undone":         false,
			"is_active_lineage": true,
			"is_active_branch":  true,
			"is_branch_head":    true,
			"lanes": []map[string]any{
				{
					"branch":              "main",
					"hover_hint":          defHint,
					"is_branch_rail":      true,
					"is_other_branch":     false,
					"has_rail_up":         true,
					"has_rail_down":       true,
					"is_active_rail_up":   true,
					"is_active_rail_down": true,
					"is_dashed_rail_up":   true,
					"is_dashed_rail_down": false,
					"has_node":            true,
					"is_fork_corner":      false,
					"has_fork_horiz":      false,
					"is_active_fork":      false,
					"is_compact_in":       false,
					"is_compact_out":      false,
					"has_compact_horiz":   false,
					"is_active_compact":   false,
					"has_stem_start":      true,
					"has_stem_through":    false,
				},
			},
		}
		return []map[string]any{row}, branchHints
	}

	activeBranchName := h.NormalizedActiveBranch()

	// 1. Canonical branch list in fixed creation order from h.Branches
	var branches []book.HistoryBranch
	branchIdxByName := make(map[string]int)
	addBranch := func(br book.HistoryBranch) {
		name := normalizeGraphBranchName(br.Name)
		if _, exists := branchIdxByName[name]; exists {
			return
		}
		br.Name = name
		branchIdxByName[name] = len(branches)
		branches = append(branches, br)
	}
	for _, br := range h.Branches {
		addBranch(br)
	}
	if _, hasMain := branchIdxByName["main"]; !hasMain {
		addBranch(book.HistoryBranch{Name: "main", HeadRevID: h.Revisions[0].ID, BaseRevID: h.Revisions[0].ID})
	}
	for _, rev := range h.Revisions {
		brName := normalizeGraphBranchName(rev.Branch)
		if _, exists := branchIdxByName[brName]; !exists {
			addBranch(book.HistoryBranch{Name: brName, HeadRevID: rev.ID, BaseRevID: rev.ID})
		}
	}

	// Map revisions by ID
	revByID := make(map[string]book.HistoryRevision, len(h.Revisions))
	revPosByID := make(map[string]int, len(h.Revisions))
	for i, rev := range h.Revisions {
		revByID[rev.ID] = rev
		revPosByID[rev.ID] = i
	}

	// Identify branch head revisions and floor Named Version on active branch
	branchHeadRevID := make(map[string]string, len(branches))
	for _, br := range branches {
		if br.HeadRevID != "" {
			branchHeadRevID[br.Name] = br.HeadRevID
		}
	}
	for _, rev := range h.Revisions {
		brName := normalizeGraphBranchName(rev.Branch)
		if branchHeadRevID[brName] == "" || rev.Named {
			branchHeadRevID[brName] = rev.ID
		}
	}
	for _, br := range branches {
		if br.HeadRevID != "" {
			if _, ok := revByID[br.HeadRevID]; ok {
				branchHeadRevID[br.Name] = br.HeadRevID
			}
		}
	}

	branchIdxs := h.ActiveBranchRevisions()
	curBranchPos := len(branchIdxs) - 1
	floorBranchPos := 0
	for j, idx := range branchIdxs {
		if idx == curIdx {
			curBranchPos = j
		}
		if h.Revisions[idx].Named {
			floorBranchPos = j
		}
	}

	// 2. Build unified row list:
	//    Top: unnamed steps on active branch above floorBranchPos (newest to oldest)
	//    Main: named versions (plus any branch head/base) across all branches (newest to oldest)
	type graphRowEntry struct {
		revIdx        int
		rev           book.HistoryRevision
		branch        string
		isUnnamedStep bool
		isUndone      bool
	}
	var entries []graphRowEntry
	includedRevIdx := make(map[int]bool)

	for j := len(branchIdxs) - 1; j > floorBranchPos; j-- {
		idx := branchIdxs[j]
		rev := h.Revisions[idx]
		if !rev.Named {
			entries = append(entries, graphRowEntry{
				revIdx:        idx,
				rev:           rev,
				branch:        activeBranchName,
				isUnnamedStep: true,
				isUndone:      j > curBranchPos,
			})
			includedRevIdx[idx] = true
		}
	}

	mustIncludeRevID := make(map[string]bool)
	for _, br := range branches {
		if br.HeadRevID != "" {
			mustIncludeRevID[br.HeadRevID] = true
		}
		if br.BaseRevID != "" {
			mustIncludeRevID[br.BaseRevID] = true
		}
	}

	for i := len(h.Revisions) - 1; i >= 0; i-- {
		if includedRevIdx[i] {
			continue
		}
		rev := h.Revisions[i]
		if rev.Named || mustIncludeRevID[rev.ID] || i == curIdx || i == viewedIdx {
			entries = append(entries, graphRowEntry{
				revIdx:        i,
				rev:           rev,
				branch:        normalizeGraphBranchName(rev.Branch),
				isUnnamedStep: !rev.Named,
				isUndone:      false,
			})
			includedRevIdx[i] = true
		}
	}

	R := len(entries)
	rowIdxByRevID := make(map[string]int, R)
	for r, e := range entries {
		rowIdxByRevID[e.rev.ID] = r
	}

	// 3. Compute active lineage (all revisions on the ancestor path of the active branch down to r1)
	activeAncestorRevs := make(map[string]bool)
	for _, e := range entries {
		if e.branch == activeBranchName {
			activeAncestorRevs[e.rev.ID] = true
		}
	}
	walkID := h.Cursor
	if walkID == "" && len(branchIdxs) > 0 {
		walkID = h.Revisions[branchIdxs[len(branchIdxs)-1]].ID
	}
	visitedWalk := make(map[string]bool)
	for walkID != "" && !visitedWalk[walkID] {
		visitedWalk[walkID] = true
		activeAncestorRevs[walkID] = true
		rev, ok := revByID[walkID]
		if !ok {
			break
		}
		if rev.ParentID != "" {
			walkID = rev.ParentID
			continue
		}
		brName := normalizeGraphBranchName(rev.Branch)
		if bIdx, ok := branchIdxByName[brName]; ok && branches[bIdx].BaseRevID != "" && branches[bIdx].BaseRevID != walkID {
			walkID = branches[bIdx].BaseRevID
			continue
		}
		pos := revPosByID[walkID]
		nextWalk := ""
		for p := pos - 1; p >= 0; p-- {
			if normalizeGraphBranchName(h.Revisions[p].Branch) == brName {
				nextWalk = h.Revisions[p].ID
				break
			}
		}
		walkID = nextWalk
	}
	if len(h.Revisions) > 0 {
		activeAncestorRevs[h.Revisions[0].ID] = true
	}

	// 4. Determine topRow, botRow (fork base row), and activeTopRow for each branch
	B := len(branches)
	topRow := make([]int, B)
	botRow := make([]int, B)
	forkTargetRow := make([]int, B)
	forkTargetBranch := make([]int, B)
	activeTopRow := make([]int, B)
	for bIdx := 0; bIdx < B; bIdx++ {
		topRow[bIdx] = -1
		botRow[bIdx] = -1
		forkTargetRow[bIdx] = -1
		forkTargetBranch[bIdx] = -1
		activeTopRow[bIdx] = R + 1
	}

	for r, e := range entries {
		bIdx, ok := branchIdxByName[e.branch]
		if !ok {
			continue
		}
		if topRow[bIdx] < 0 || r < topRow[bIdx] {
			topRow[bIdx] = r
		}
		if r > botRow[bIdx] {
			botRow[bIdx] = r
		}
		if activeAncestorRevs[e.rev.ID] && r < activeTopRow[bIdx] {
			activeTopRow[bIdx] = r
		}
	}

	// Resolve fork base row for each non-root branch
	findDisplayedAncestorRow := func(startRevID string) int {
		curr := startRevID
		seen := make(map[string]bool)
		for curr != "" && !seen[curr] {
			seen[curr] = true
			if rIdx, ok := rowIdxByRevID[curr]; ok {
				return rIdx
			}
			rev, ok := revByID[curr]
			if !ok {
				break
			}
			curr = rev.ParentID
		}
		return -1
	}

	for bIdx := 0; bIdx < B; bIdx++ {
		br := branches[bIdx]
		if bIdx == 0 || br.Name == "main" {
			if topRow[bIdx] < 0 {
				topRow[bIdx] = 0
			}
			botRow[bIdx] = R - 1
			continue
		}
		baseCand := br.BaseRevID
		if botRow[bIdx] >= 0 {
			oldestOwnRev := entries[botRow[bIdx]].rev
			if oldestOwnRev.ParentID != "" {
				baseCand = oldestOwnRev.ParentID
			}
		}
		baseR := findDisplayedAncestorRow(baseCand)
		if baseR < 0 && R > 0 {
			baseR = R - 1
		}
		if topRow[bIdx] < 0 && baseR >= 0 {
			topRow[bIdx] = baseR
		}
		if baseR > botRow[bIdx] {
			botRow[bIdx] = baseR
			forkTargetRow[bIdx] = baseR
			if parentIdx, ok := branchIdxByName[entries[baseR].branch]; ok {
				forkTargetBranch[bIdx] = parentIdx
			} else {
				forkTargetBranch[bIdx] = 0
			}
		}
	}

	// Propagate activeTopRow from active child branches into their parent branch at forkTargetRow
	for bIdx := B - 1; bIdx >= 1; bIdx-- {
		if activeTopRow[bIdx] <= botRow[bIdx] && forkTargetRow[bIdx] >= 0 && forkTargetBranch[bIdx] >= 0 {
			pIdx := forkTargetBranch[bIdx]
			if forkTargetRow[bIdx] < activeTopRow[pIdx] {
				activeTopRow[pIdx] = forkTargetRow[bIdx]
			}
		}
	}

	// Find last unnamed row index on active branch so dashed rail connects down to the first named version
	firstNamedRowOnActive := -1
	hasUnnamedOnActive := false
	for r, e := range entries {
		if e.branch == activeBranchName {
			if e.isUnnamedStep {
				hasUnnamedOnActive = true
			} else if firstNamedRowOnActive < 0 {
				firstNamedRowOnActive = r
				break
			}
		}
	}

	// 5. Assign compacted columns per row:
	//    At row r, all branches bIdx with topRow[bIdx] >= 0 && r <= botRow[bIdx] occupy columns 0..K-1 in fixed creation order.
	colOfBranch := make([][]int, R)
	branchAtCol := make([][]int, R)
	for r := 0; r < R; r++ {
		colOfBranch[r] = make([]int, B)
		for bIdx := 0; bIdx < B; bIdx++ {
			colOfBranch[r][bIdx] = -1
		}
		var cols []int
		for bIdx := 0; bIdx < B; bIdx++ {
			if topRow[bIdx] < 0 {
				continue
			}
			if r <= botRow[bIdx] {
				colOfBranch[r][bIdx] = len(cols)
				cols = append(cols, bIdx)
			}
		}
		branchAtCol[r] = cols
	}

	// 6. Build output rows and lane cells
	outRows := make([]map[string]any, 0, R)
	for r, e := range entries {
		rev := e.rev
		rowBranchIdx := branchIdxByName[e.branch]
		nodeCol := colOfBranch[r][rowBranchIdx]
		if nodeCol < 0 {
			nodeCol = 0
		}

		numCols := len(branchAtCol[r])
		if r > 0 && len(branchAtCol[r-1]) > numCols {
			// Check if any branch compacted from r-1 to r; include its previous column so the ╯ turn renders cleanly
			for bIdx := 0; bIdx < B; bIdx++ {
				curC := colOfBranch[r][bIdx]
				prevC := colOfBranch[r-1][bIdx]
				if curC >= 0 && prevC > curC && prevC+1 > numCols {
					numCols = prevC + 1
				}
			}
		}
		if numCols < 1 {
			numCols = 1
		}

		// Check fork connectors on this row (branches that have forkTargetRow[bIdx] == r)
		maxForkCol := -1
		maxActiveForkCol := -1
		forkCornerAtCol := make(map[int]bool)
		activeForkCornerAtCol := make(map[int]bool)
		for bIdx := 1; bIdx < B; bIdx++ {
			if forkTargetRow[bIdx] == r {
				c := colOfBranch[r][bIdx]
				if c > nodeCol {
					forkCornerAtCol[c] = true
					if c > maxForkCol {
						maxForkCol = c
					}
					if activeTopRow[bIdx] <= r {
						activeForkCornerAtCol[c] = true
						if c > maxActiveForkCol {
							maxActiveForkCol = c
						}
					}
				}
			}
		}

		// Check compaction shifts on this row (branches where colOfBranch[r][bIdx] < colOfBranch[r-1][bIdx])
		compactInAtCol := make(map[int]bool)
		compactOutAtCol := make(map[int]bool)
		compactHorizAtCol := make(map[int]bool)
		activeCompactAtCol := make(map[int]bool)
		compactOutBranchAtCol := make(map[int]int)
		if r > 0 {
			for bIdx := 0; bIdx < B; bIdx++ {
				curC := colOfBranch[r][bIdx]
				prevC := colOfBranch[r-1][bIdx]
				if curC >= 0 && prevC > curC {
					isAct := r >= activeTopRow[bIdx]
					compactInAtCol[curC] = true
					compactOutAtCol[prevC] = true
					compactOutBranchAtCol[prevC] = bIdx
					if isAct {
						activeCompactAtCol[curC] = true
						activeCompactAtCol[prevC] = true
					}
					for c := curC + 1; c < prevC; c++ {
						compactHorizAtCol[c] = true
						if isAct {
							activeCompactAtCol[c] = true
						}
					}
				}
			}
		}

		lanes := make([]map[string]any, 0, numCols)
		for c := 0; c < numCols; c++ {
			bIdx := -1
			if c < len(branchAtCol[r]) {
				bIdx = branchAtCol[r][c]
			} else if outBIdx, ok := compactOutBranchAtCol[c]; ok {
				bIdx = outBIdx
			}

			cellBranch := ""
			cellHint := ""
			isOtherBranch := false
			hasRailUp := false
			hasRailDown := false
			isActiveRailUp := false
			isActiveRailDown := false
			isDashedRailUp := false
			isDashedRailDown := false

			if bIdx >= 0 {
				cellBranch = branches[bIdx].Name
				cellHint = branchHints[cellBranch]
				isOtherBranch = cellBranch != activeBranchName
				if c < len(branchAtCol[r]) {
					hasRailUp = (r > topRow[bIdx] || (r == topRow[bIdx] && cellBranch == activeBranchName)) && !compactInAtCol[c]
					hasRailDown = (r >= topRow[bIdx] && r < botRow[bIdx]) || (bIdx == 0 && r == R-1)
					isActiveRailUp = hasRailUp && r >= activeTopRow[bIdx]
					isActiveRailDown = hasRailDown && r >= activeTopRow[bIdx]
					if cellBranch == activeBranchName {
						if r == topRow[bIdx] && !hasUnnamedOnActive {
							isDashedRailUp = true
						} else if hasUnnamedOnActive && firstNamedRowOnActive > 0 {
							isDashedRailUp = hasRailUp && r <= firstNamedRowOnActive
							isDashedRailDown = hasRailDown && r < firstNamedRowOnActive
						}
					}
				}
			}

			isForkCorner := forkCornerAtCol[c]
			if isForkCorner {
				hasRailDown = false
			}
			hasForkHoriz := maxForkCol > nodeCol && c > nodeCol && c < maxForkCol
			isActiveFork := activeForkCornerAtCol[c] || (maxActiveForkCol > nodeCol && c > nodeCol && c < maxActiveForkCol)

			lanes = append(lanes, map[string]any{
				"branch":              cellBranch,
				"hover_hint":          cellHint,
				"is_branch_rail":      bIdx >= 0 && (hasRailUp || hasRailDown || isForkCorner || c == nodeCol),
				"is_other_branch":     isOtherBranch,
				"has_rail_up":         hasRailUp,
				"has_rail_down":       hasRailDown,
				"is_active_rail_up":   isActiveRailUp,
				"is_active_rail_down": isActiveRailDown,
				"is_dashed_rail_up":   isDashedRailUp,
				"is_dashed_rail_down": isDashedRailDown,
				"has_node":            c == nodeCol,
				"is_fork_corner":      isForkCorner,
				"has_fork_horiz":      hasForkHoriz,
				"is_active_fork":      isActiveFork,
				"is_compact_in":       compactInAtCol[c],
				"is_compact_out":      compactOutAtCol[c],
				"has_compact_horiz":   compactHorizAtCol[c],
				"is_active_compact":   activeCompactAtCol[c],
				"has_stem_start":      c == nodeCol,
				"has_stem_through":    c > nodeCol,
			})
		}

		isActiveHead := e.revIdx == curIdx
		isViewed := e.revIdx == viewedIdx
		isPreview := isHistPreview && isViewed
		isActiveLineage := activeAncestorRevs[rev.ID]
		isActiveBranch := e.branch == activeBranchName
		isBranchHead := branchHeadRevID[e.branch] == rev.ID

		timeStr := formatHistoryCompactTime(rev.TimeUnix)
		metaLabel := fmt.Sprintf("%s • %s", rev.ID, timeStr)
		if isPreview {
			metaLabel = "PODGLĄD • " + rev.ID
		}

		rowHint := revHints[rev.ID]
		if rowHint == "" {
			rowHint = branchHints[e.branch]
		}

		nodeAction := "PreviewHistoryRevision"
		if !isActiveBranch {
			nodeAction = "SwitchHistoryBranch"
		}

		outRows = append(outRows, map[string]any{
			"id":                rev.ID,
			"label":             rev.Label,
			"time_label":        timeStr,
			"meta_label":        metaLabel,
			"branch":            e.branch,
			"hover_hint":        rowHint,
			"node_action":       nodeAction,
			"is_unnamed_step":   e.isUnnamedStep,
			"is_current":        isViewed,
			"is_active_head":    isActiveHead,
			"is_preview":        isPreview,
			"is_undone":         e.isUndone,
			"is_active_lineage": isActiveLineage,
			"is_active_branch":  isActiveBranch,
			"is_branch_head":    isBranchHead,
			"lanes":             lanes,
		})
	}

	return outRows, branchHints
}

func (s *Server) populateEditorHistoryContext(vars map[string]any) {
	canUndo := false
	canRedo := false
	undoTitle := "Brak zmian do cofnięcia od ostatniej wersji (Ctrl+Z)"
	redoTitle := "Brak zmian do ponowienia (Ctrl+Shift+Z)"
	unnamedCount := 0
	var unnamedItems []map[string]any
	var namedVersions []map[string]any
	var graphRows []map[string]any
	activeBranchName := "main"
	var branchItems []map[string]any
	branchDiffSummary := ""
	hasBranchDiff := false

	if s.ActiveBook != nil && !s.StandaloneMode {
		s.ensureActiveBookUnpackedForStudio()
		if !s.ActiveBook.IsPacked {
			if s.EditorHistoryPanelOpen && !s.isHistoryPreview() {
				s.ensureHistoryBaselineBeforeMutation()
			}
			if h := s.loadActiveBookHistory(); h != nil {
				activeBranchName = h.NormalizedActiveBranch()
				unnamedCount = h.UnnamedEditsCount()
				curIdx := h.CursorIndex()
				previewIdx := -1
				if s.PreviewRevisionID != "" {
					for i := range h.Revisions {
						if h.Revisions[i].ID == s.PreviewRevisionID {
							previewIdx = i
							break
						}
					}
					if previewIdx == curIdx {
						s.PreviewRevisionID = ""
						s.PreviewRevisionLabel = ""
						previewIdx = -1
					} else if previewIdx >= 0 {
						s.PreviewRevisionLabel = h.Revisions[previewIdx].Label
					}
				}
				isHistPreview := previewIdx >= 0 && previewIdx != curIdx
				viewedIdx := curIdx
				if isHistPreview {
					viewedIdx = previewIdx
				}
				vars["is_history_preview"] = isHistPreview
				if isHistPreview {
					vars["history_preview_attr"] = "true"
				} else {
					vars["history_preview_attr"] = "false"
				}
				vars["history_preview_id"] = s.PreviewRevisionID
				vars["history_preview_label"] = s.PreviewRevisionLabel

				canUndo = !isHistPreview && h.CanUndo()
				canRedo = !isHistPreview && h.CanRedo()

				var branchHints map[string]string
				graphRows, branchHints = buildHistoryGraphRows(s.ActiveBook, h, curIdx, viewedIdx, isHistPreview)

				for _, br := range h.Branches {
					isAct := strings.EqualFold(br.Name, activeBranchName)
					branchItems = append(branchItems, map[string]any{
						"name":        br.Name,
						"head_rev_id": br.HeadRevID,
						"base_rev_id": br.BaseRevID,
						"time_label":  formatHistoryTimestamp(br.CreatedAtUnix),
						"hover_hint":  branchHints[normalizeGraphBranchName(br.Name)],
						"is_active":   isAct,
						"can_delete":  !isAct && !strings.EqualFold(br.Name, "main"),
					})
				}

				if activeBranchName != "main" {
					if cmp, err := book.CompareBookBranches(s.ActiveBook, "main", activeBranchName); err == nil && cmp != nil {
						branchDiffSummary = cmp.SummaryLabel
						hasBranchDiff = true
					}
				}

				branchIdxs := h.ActiveBranchRevisions()
				curBranchPos := len(branchIdxs) - 1
				floorBranchPos := 0
				for j, idx := range branchIdxs {
					if idx == curIdx {
						curBranchPos = j
					}
					if h.Revisions[idx].Named {
						floorBranchPos = j
					}
				}

				if canUndo && curIdx >= 0 && curIdx < len(h.Revisions) {
					undoTitle = "Cofnij: " + h.Revisions[curIdx].Label + " (Ctrl+Z)"
				} else if isHistPreview {
					undoTitle = historyPreviewLockedToast
				}
				if canRedo && curBranchPos >= 0 && curBranchPos+1 < len(branchIdxs) {
					redoTitle = "Ponów: " + h.Revisions[branchIdxs[curBranchPos+1]].Label + " (Ctrl+Shift+Z)"
				} else if isHistPreview {
					redoTitle = historyPreviewLockedToast
				}

				hasAnyUnnamed := false
				for j := floorBranchPos + 1; j < len(branchIdxs); j++ {
					if !h.Revisions[branchIdxs[j]].Named {
						hasAnyUnnamed = true
						break
					}
				}
				if hasAnyUnnamed && len(branchIdxs) > 0 {
					for j := len(branchIdxs) - 1; j >= floorBranchPos; j-- {
						idx := branchIdxs[j]
						rev := h.Revisions[idx]
						isActiveHead := idx == curIdx
						isViewed := idx == viewedIdx
						isPreview := isHistPreview && isViewed
						isUndone := j > curBranchPos
						statusLabel := ""
						if isPreview {
							statusLabel = "PODGLĄD"
						} else if isActiveHead {
							statusLabel = "AKTYWNY"
						} else if isUndone {
							statusLabel = "COFNIĘTO"
						} else if rev.Named {
							statusLabel = "WERSJA BAZOWA"
						}
						unnamedItems = append(unnamedItems, map[string]any{
							"id":             rev.ID,
							"label":          rev.Label,
							"time_label":     formatHistoryTimestamp(rev.TimeUnix),
							"is_current":     isViewed,
							"is_active_head": isActiveHead,
							"is_preview":     isPreview,
							"is_undone":      isUndone,
							"status_label":   statusLabel,
						})
					}
				}

				for j := len(branchIdxs) - 1; j >= 0; j-- {
					idx := branchIdxs[j]
					rev := h.Revisions[idx]
					if !rev.Named {
						continue
					}
					isActiveHead := idx == curIdx
					isViewed := idx == viewedIdx
					isPreview := isHistPreview && isViewed
					namedVersions = append(namedVersions, map[string]any{
						"id":             rev.ID,
						"label":          rev.Label,
						"time_label":     formatHistoryTimestamp(rev.TimeUnix),
						"is_current":     isViewed,
						"is_active_head": isActiveHead,
						"is_preview":     isPreview,
					})
				}
			}
		}
		if len(graphRows) == 0 {
			graphRows, _ = buildHistoryGraphRows(s.ActiveBook, nil, -1, -1, false)
		}
		if len(namedVersions) == 0 {
			namedVersions = append(namedVersions, map[string]any{
				"id":             "r1",
				"label":          "Stan początkowy",
				"time_label":     formatHistoryTimestamp(time.Now().Unix()),
				"is_current":     true,
				"is_active_head": true,
				"is_preview":     false,
			})
		}
		if len(branchItems) == 0 {
			branchItems = append(branchItems, map[string]any{
				"name":        "main",
				"head_rev_id": "r1",
				"base_rev_id": "r1",
				"time_label":  formatHistoryTimestamp(time.Now().Unix()),
				"hover_hint":  "main",
				"is_active":   true,
				"can_delete":  false,
			})
		}
	}

	vars["can_undo"] = canUndo
	vars["can_redo"] = canRedo
	vars["undo_title"] = undoTitle
	vars["redo_title"] = redoTitle
	vars["has_unnamed_edits"] = unnamedCount > 0
	vars["unnamed_edits_count"] = unnamedCount
	vars["show_editor_history_panel"] = s.EditorHistoryPanelOpen
	vars["active_branch_name"] = activeBranchName
	vars["history_branches"] = branchItems
	vars["has_multiple_branches"] = len(branchItems) > 1
	vars["branch_diff_summary"] = branchDiffSummary
	vars["has_branch_diff"] = hasBranchDiff
	vars["history_unnamed_items"] = unnamedItems
	vars["has_history_unnamed_items"] = len(unnamedItems) > 0
	vars["history_named_versions"] = namedVersions
	vars["has_history_named_versions"] = len(namedVersions) > 0
	vars["history_graph_rows"] = graphRows
	vars["has_history_graph_rows"] = len(graphRows) > 0
	vars["default_next_version_name"] = fmt.Sprintf("Wersja %d", len(namedVersions)+1)
	vars["default_next_branch_name"] = fmt.Sprintf("wariant-%d", len(branchItems)+1)
}
