package gui

import (
	"fmt"
	"sort"
	"strings"

	"storyteller/internal/book"
)

func (s *Server) buildChoiceTargetOptions(sc *book.Scene, ch book.Choice) string {
	selectedVal := "END"
	if s.ActiveBook == nil {
		return selectedVal
	}
	curCh := ""
	if sc != nil {
		curCh = strings.TrimSpace(sc.Chapter)
	}
	if curCh == "" {
		curCh = s.ActiveBook.DefaultChapterName()
	}

	matchedOption := false

	// 1. Same-chapter scenes
	for _, cand := range s.ActiveBook.Scenes {
		candCh := strings.TrimSpace(cand.Chapter)
		if candCh == "" {
			candCh = s.ActiveBook.DefaultChapterName()
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
			optVal := cand.ReferenceKey()
			if cand.ExplicitID != "" && (strings.EqualFold(ch.Target, cand.ExplicitID) || isSel) {
				optVal = cand.ExplicitID
			}
			selectedVal = optVal
		}
	}

	// 2. Forward chapters (one-way chapter transitions: chapter:<id>)
	for _, chItem := range s.ActiveBook.Chapters {
		chRef := chItem.ReferenceKey()
		optVal := "chapter:" + chRef
		isCurrentSel := !ch.IsEnding && ch.IsChapterLink &&
			(strings.EqualFold(ch.TargetChapterID, chRef) ||
				strings.EqualFold(ch.TargetChapterID, chItem.ID) ||
				strings.EqualFold(ch.Target, optVal))
		if isCurrentSel {
			matchedOption = true
			selectedVal = optVal
		}
	}

	// 3. END
	if ch.IsEnding || strings.EqualFold(ch.Target, "END") {
		matchedOption = true
		selectedVal = "END"
	}
	if !matchedOption && ch.Target != "" {
		selectedVal = ch.Target
	}
	return selectedVal
}

func (s *Server) buildSceneMediaOptions(sc *book.Scene, buildMenus bool) (curImg, curImgLabel string, imgOptions []map[string]any, curVoice, curVoiceLabel string, voiceOptions []map[string]any) {
	curImg = sc.ExplicitImage
	if curImg == "" && sc.ImageFile != "" {
		curImg = sc.ImageFile
	}
	noneLabel := s.tr(TrMediaNoneLabel)
	curImgLabel = noneLabel
	if curImg != "" {
		curImgLabel = curImg
	}
	if buildMenus && s.ActiveBook != nil {
		imgOptions = make([]map[string]any, 0, len(s.ActiveBook.ImageFiles)+2)
		imgOptions = append(imgOptions, map[string]any{
			"value":     "",
			"label":     noneLabel,
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
			"label":     s.tr(TrAddImageFromDiskOpt),
			"is_active": false,
		})
	}

	curVoice = ""
	curVoiceLabel = noneLabel
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
				"label":     noneLabel,
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
				"label":     s.tr(TrAddAudioFromDiskOpt),
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
	inspectorChoices := sc.Choices
	if len(inspectorChoices) == 0 || sc.ExplicitEnd {
		inspectorChoices = []book.Choice{
			{
				Label:      s.tr(TrNext),
				Target:     "END",
				IsEnding:   true,
				IsImplicit: true,
			},
		}
	}
	inspectorPorts := make([]map[string]any, 0, len(inspectorChoices))
	for chIdx, ch := range inspectorChoices {
		lbl := ch.Label
		if lbl == "" && !hasChoices {
			lbl = s.tr(TrNext)
		}
		selectedVal := "END"
		if isSelected {
			selectedVal = s.buildChoiceTargetOptions(sc, ch)
		}

		portMap, _ := s.buildChoiceRuleViewMap(sc.ID, chIdx, ch, selectedVal)
		portMap["raw_label"] = lbl
		inspectorPorts = append(inspectorPorts, portMap)
	}

	curImg, curImgLabel, imgOptions, curVoice, curVoiceLabel, voiceOptions := s.buildSceneMediaOptions(sc, isSelected)

	nodeWarns := make([]map[string]any, 0, len(sc.Warnings))
	for _, w := range sc.Warnings {
		nodeWarns = append(nodeWarns, map[string]any{"text": w})
	}

	curChName := strings.TrimSpace(sc.Chapter)
	if curChName == "" {
		curChName = s.ActiveBook.DefaultChapterName()
	}
	var chapterOptions []map[string]any
	if isSelected && s.ActiveBook != nil {
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

	scEffectsVal := book.CleanEffectsExpr(sc.Effects)
	scEffectsErr := ""
	if scEffectsVal != "" {
		scEffectsErr = book.ExplainEffectsExprErrorLang(sc.Effects, s.ActiveBook, s.Settings.Language)
	}
	voiceURL := sc.AudioURL
	voiceDurLabel := "0:00"
	if isSelected && curVoice != "" && s.ActiveBook != nil {
		durLbl, stamp := book.ReadSceneAudioDurationAndStamp(s.ActiveBook, sc)
		voiceDurLabel = durLbl
		if voiceURL != "" && stamp > 0 {
			voiceURL = fmt.Sprintf("%s?v=%d", voiceURL, stamp)
		}
	}
	trimStart, trimEnd := sc.NormalizedVoiceTrim()
	hasVoiceFile := curVoice != "" && sc.AudioURL != ""

	portCount := 0
	if hasChoices {
		portCount = len(inspectorChoices)
	}

	return map[string]any{
		"id":                   sc.ID,
		"explicit_id":          sc.ExplicitID,
		"current_image":        curImg,
		"current_image_label":  curImgLabel,
		"image_options":        imgOptions,
		"current_voice":        curVoice,
		"current_voice_label":  curVoiceLabel,
		"current_voice_url":    voiceURL,
		"voice_duration_label": voiceDurLabel,
		"voice_trim_start":     fmt.Sprintf("%.4f", trimStart),
		"voice_trim_end":       fmt.Sprintf("%.4f", trimEnd),
		"has_voice_file":       hasVoiceFile,
		"has_voice_trim":       hasVoiceFile && sc.HasVoiceTrim(),
		"voice_options":        voiceOptions,
		"name":                 sc.Name,
		"slugline":             sc.Slugline,
		"synopsis":             sc.Synopsis,
		"effects":              scEffectsVal,
		"has_effects":          scEffectsVal != "",
		"is_effects_invalid":   scEffectsErr != "",
		"effects_error_hint":   scEffectsErr,
		"script_length_label":  scriptLengthLabel,
		"chapter":              curChName,
		"chapter_options":      chapterOptions,
		"source_file":          sc.SourceFile,
		"is_start_scene":       isStart,
		"is_chapter_entry":     isChapterEntry,
		"has_node_warnings":    len(sc.Warnings) > 0,
		"node_warnings":        nodeWarns,
		"inspector_ports":      inspectorPorts,
		"port_count":           portCount,
	}
}

func (s *Server) populateEditorBookAndChapterSettings(vars map[string]any) {
	if s.ActiveBook == nil {
		vars["book_settings_open"] = false
		vars["has_selected_chapter"] = false
		vars["is_node_inspector_hidden"] = false
		vars["multi_scene_items"] = nil
		return
	}

	isScreenplay := s.ActiveBook.IsScreenplay()
	normMode := s.ActiveBook.NormalizedMode()
	normStd := s.ActiveBook.NormalizedStandard()
	modeLabel, modeOpts := buildBookModeOptions(normMode, s.lang())
	_, stdOpts := buildScriptStandardOptions(normStd)
	prefixPills, timePills := buildSluglineHelperPills(normStd)

	vars["is_screenplay_mode"] = isScreenplay
	vars["voice_recorder_open"] = s.VoiceRecorderOpen
	vars["book_mode"] = normMode
	vars["book_mode_label"] = modeLabel
	vars["book_mode_options"] = modeOpts
	vars["script_standard"] = normStd
	vars["script_standard_options"] = stdOpts
	vars["slugline_prefix_pills"] = prefixPills
	vars["slugline_time_pills"] = timePills

	multiSceneItems := make([]map[string]any, 0, len(s.ActiveBook.Scenes))
	for _, sc := range s.ActiveBook.Scenes {
		scCh := strings.TrimSpace(sc.Chapter)
		if scCh == "" {
			scCh = s.ActiveBook.DefaultChapterName()
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
	vars["active_book_license"] = s.ActiveBook.License
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
	startChapterName := s.ActiveBook.DefaultChapterName()
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
			scCh = s.ActiveBook.DefaultChapterName()
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
			chName = s.ActiveBook.DefaultChapterName()
		}
		var chScenes []*book.Scene
		for _, sc := range s.ActiveBook.Scenes {
			scCh := strings.TrimSpace(sc.Chapter)
			if scCh == "" {
				scCh = s.ActiveBook.DefaultChapterName()
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

		branchCount := 0
		exitCount := 0
		totalWords := 0
		totalEighths := 0
		for _, sc := range chScenes {
			totalWords += len(strings.Fields(sc.Text))
			if s.ActiveBook.IsScreenplay() {
				metrics := book.ComputeSceneScriptMetrics(sc, sc.OrderIndex+1, s.ActiveBook.NormalizedStandard())
				totalEighths += metrics.Eighths
			}
			if len(sc.Choices) >= 2 {
				branchCount += len(sc.Choices)
			}
			for _, ch := range sc.Choices {
				if ch.IsEnding || ch.IsChapterLink {
					exitCount++
				} else if ch.TargetSceneID != "" {
					if targetSc := s.findScene(ch.TargetSceneID); targetSc != nil && !strings.EqualFold(strings.TrimSpace(targetSc.Chapter), chName) {
						exitCount++
					}
				}
			}
		}
		lengthStat := s.trf(TrWordsCountFmt, totalWords)
		if s.ActiveBook.IsScreenplay() {
			lengthStat = book.FormatEighths(totalEighths)
		}
		vars["selected_chapter"] = map[string]any{
			"name":                chName,
			"start_scene_id":      chStartID,
			"start_scene_label":   chStartLabel,
			"start_scene_options": chStartOptions,
			"scene_count":         len(chScenes),
			"length_stat":         lengthStat,
			"branch_count":        branchCount,
			"exit_count":          exitCount,
		}
	}
	vars["has_selected_chapter"] = hasSelectedChapter
	vars["is_node_inspector_hidden"] = true
	isContextPanelHidden := !s.BookSettingsOpen && !hasSelectedChapter
	vars["is_context_panel_hidden"] = isContextPanelHidden
	vars["show_editor_help_panel"] = s.EditorHelpModalOpen
	vars["show_syntax_help_popover"] = s.SyntaxHelpPopoverOpen
	s.populateEditorVariablesContext(vars)
}

func (s *Server) buildChoiceRuleViewMap(sceneID string, idx int, ch book.Choice, selectedVal string) (map[string]any, bool) {
	condInputVal := book.CleanConditionExpr(ch.Condition)
	if ch.ShowWhenLocked && condInputVal != "" && !strings.HasPrefix(condInputVal, "?") {
		condInputVal = "? " + condInputVal
	}
	effInputVal := book.CleanEffectsExpr(ch.Effects)
	hasCond := condInputVal != ""
	hasEff := effInputVal != ""
	hasRule := hasCond || hasEff
	hasBookVars := s.ActiveBook != nil && len(s.ActiveBook.Variables) > 0
	ruleKey := fmt.Sprintf("%s:%d", sceneID, idx)
	isRuleOpen := hasBookVars
	if hasBookVars && s.ChoiceRulesOpen != nil {
		if expOpen, exists := s.ChoiceRulesOpen[ruleKey]; exists {
			isRuleOpen = expOpen
		}
	}
	condErrHint := ""
	if hasCond {
		condErrHint = book.ExplainConditionExprErrorLang(ch.Condition, s.ActiveBook, s.Settings.Language)
	}
	effErrHint := ""
	if hasEff {
		effErrHint = book.ExplainEffectsExprErrorLang(ch.Effects, s.ActiveBook, s.Settings.Language)
	}
	return map[string]any{
		"idx":             idx,
		"selected_value":  selectedVal,
		"condition":       condInputVal,
		"effects":         effInputVal,
		"has_condition":   hasCond,
		"has_effects":     hasEff,
		"has_rule":        hasRule,
		"is_rule_open":    isRuleOpen,
		"is_cond_invalid": condErrHint != "",
		"is_eff_invalid":  effErrHint != "",
		"cond_error_hint": condErrHint,
		"eff_error_hint":  effErrHint,
	}, hasCond
}

func (s *Server) populateEditorSelectedNodeOnly(vars map[string]any) {
	selected := s.findScene(s.SelectedSceneID)
	if selected == nil {
		selected = s.ActiveBook.Scenes[0]
		s.SelectedSceneID = selected.ID
	}
	vars["selected_scene_id"] = selected.ID
	selectedNodeMap := s.buildEditorNodeMap(selected, true)
	vars["selected_node"] = selectedNodeMap
	vars["has_selected_node"] = selectedNodeMap != nil
	s.populateEditorBookAndChapterSettings(vars)
}

func (s *Server) populateEditorContext(vars map[string]any) {
	s.populateEditorSelectedNodeOnly(vars)
	selected := s.findScene(s.SelectedSceneID)

	orderedScenes := make([]*book.Scene, len(s.ActiveBook.Scenes))
	copy(orderedScenes, s.ActiveBook.Scenes)
	sort.SliceStable(orderedScenes, func(i, j int) bool {
		ri, rj := max(orderedScenes[i].Row, 1), max(orderedScenes[j].Row, 1)
		if ri != rj {
			return ri < rj
		}
		return orderedScenes[i].Col < orderedScenes[j].Col
	})

	edZoom := s.ActiveBook.EditorZoom
	if edZoom < 0.2 || edZoom > 3.0 {
		edZoom = 1.0
	}
	vars["studio_graph_payload"] = buildStudioGraphPayload(s.ActiveBook, edZoom, selected.ID, orderedScenes)

	s.populateEditorBreakdownContext(vars)
	s.populateEditorHistoryContext(vars)
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

func buildStudioGraphPayload(b *book.Book, edZoom float64, selectedSceneID string, orderedScenes []*book.Scene) string {
	if b == nil {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(orderedScenes)*160 + len(b.Chapters)*120 + 64)
	fmt.Fprintf(&sb, "viewport | pan_x:%d | pan_y:%d | zoom:%.2f | has_view:%d\n",
		b.EditorPanX, b.EditorPanY, edZoom, boolToPayloadBit(b.HasEditorView))

	for chIdx, chItem := range b.Chapters {
		chName := strings.TrimSpace(chItem.Name)
		if chName == "" {
			continue
		}
		var chScenes []*book.Scene
		for _, sc := range b.Scenes {
			scCh := strings.TrimSpace(sc.Chapter)
			if scCh == "" {
				scCh = b.DefaultChapterName()
			}
			if strings.EqualFold(scCh, chName) {
				chScenes = append(chScenes, sc)
			}
		}
		if len(chScenes) == 0 {
			continue
		}
		minX, maxX := chScenes[0].X, chScenes[0].X+book.NodeCardWidth
		minY, maxY := chScenes[0].Y, chScenes[0].Y+book.NodeMinLogicalHeight
		for _, sc := range chScenes[1:] {
			if sc.X < minX {
				minX = sc.X
			}
			if sc.X+book.NodeCardWidth > maxX {
				maxX = sc.X + book.NodeCardWidth
			}
			if sc.Y < minY {
				minY = sc.Y
			}
			if sc.Y+book.NodeMinLogicalHeight > maxY {
				maxY = sc.Y + book.NodeMinLogicalHeight
			}
		}
		chRef := chItem.ReferenceKey()
		isRoot := chIdx == 0
		if b.StartSceneID != "" {
			isRoot = false
			for _, sc := range chScenes {
				if strings.EqualFold(sc.ID, b.StartSceneID) {
					isRoot = true
					break
				}
			}
		}
		var outRefs []string
		seenTargets := make(map[string]bool)
		for _, sc := range chScenes {
			for _, ch := range sc.Choices {
				if ch.IsEnding || !ch.IsChapterLink || ch.TargetChapterID == "" {
					continue
				}
				tCh := b.FindChapter(ch.TargetChapterID)
				if tCh == nil {
					continue
				}
				tRef := tCh.ReferenceKey()
				tKey := strings.ToLower(tRef)
				if seenTargets[tKey] {
					continue
				}
				seenTargets[tKey] = true
				outRefs = append(outRefs, sanitizeGraphPayloadField(tRef))
			}
		}
		startSceneID := chItem.StartSceneID
		if startSceneID == "" && len(chScenes) > 0 {
			startSceneID = chScenes[0].ID
		}
		x := minX - book.ChapterFramePadX
		y := minY - book.ChapterFramePadTop
		w := (maxX - minX) + book.ChapterFramePadX2
		h := (maxY - minY) + book.ChapterFramePadYTotal
		fmt.Fprintf(&sb, "chapter | id:%s | ref:%s | name:%s | root:%d | start:%s | x:%d | y:%d | w:%d | h:%d | outs:%s\n",
			sanitizeGraphPayloadField(chItem.ID),
			sanitizeGraphPayloadField(chRef),
			sanitizeGraphPayloadField(chName),
			boolToPayloadBit(isRoot),
			sanitizeGraphPayloadField(startSceneID),
			x, y, w, h,
			strings.Join(outRefs, ","))
	}

	branchRoles, branchParents := book.ComputeBranchRoles(b)

	for _, sc := range orderedScenes {
		id := sc.ID
		name := sc.Name
		if name == "" {
			name = id
		}
		chapter := strings.TrimSpace(sc.Chapter)
		if chapter == "" {
			chapter = b.DefaultChapterName()
		}
		isSelected := sc.ID == selectedSceneID
		isStart := false
		if b.StartSceneID != "" {
			isStart = strings.EqualFold(sc.ID, b.StartSceneID)
		} else if len(b.Scenes) > 0 {
			isStart = sc.ID == b.Scenes[0].ID
		}
		isEntry := isStart
		if !isEntry {
			if chMeta := b.FindChapter(sc.Chapter); chMeta != nil && chMeta.StartSceneID != "" {
				isEntry = strings.EqualFold(sc.ID, chMeta.StartSceneID)
			}
		}
		hasChoices := len(sc.Choices) >= 2
		impTarget := ""
		impChapter := ""
		if !hasChoices && len(sc.Choices) == 1 && !sc.Choices[0].IsEnding {
			impTarget = sc.Choices[0].TargetSceneID
			if sc.Choices[0].IsChapterLink && sc.Choices[0].TargetChapterID != "" {
				impChapter = sc.Choices[0].TargetChapterID
				if tCh := b.FindChapter(impChapter); tCh != nil {
					impChapter = tCh.ReferenceKey()
				}
			}
		}
		branchRole := branchRoles[id]
		branchParent := branchParents[id]

		fmt.Fprintf(&sb, "node | id:%s | name:%s | chapter:%s | x:%d | y:%d | selected:%d | start:%d | entry:%d | unreachable:%d | has_choices:%d | imp_target:%s | imp_chapter:%s | branch_role:%s | branch_parent:%s\n",
			sanitizeGraphPayloadField(id),
			sanitizeGraphPayloadField(name),
			sanitizeGraphPayloadField(chapter),
			sc.X, sc.Y,
			boolToPayloadBit(isSelected),
			boolToPayloadBit(isStart),
			boolToPayloadBit(isEntry),
			boolToPayloadBit(sc.Unreachable),
			boolToPayloadBit(hasChoices),
			sanitizeGraphPayloadField(impTarget),
			sanitizeGraphPayloadField(impChapter),
			sanitizeGraphPayloadField(branchRole),
			sanitizeGraphPayloadField(branchParent))

		if hasChoices {
			inspectorChoices := sc.Choices
			if len(inspectorChoices) == 0 || sc.ExplicitEnd {
				inspectorChoices = []book.Choice{
					{
						Label:      "Dalej",
						Target:     "END",
						IsEnding:   true,
						IsImplicit: true,
					},
				}
			}
			for chIdx, ch := range inspectorChoices {
				lbl := ch.Label
				targetChapterRef := ""
				if !ch.IsEnding && ch.IsChapterLink && ch.TargetChapterID != "" {
					targetChapterRef = ch.TargetChapterID
					if tCh := b.FindChapter(ch.TargetChapterID); tCh != nil {
						targetChapterRef = tCh.ReferenceKey()
					}
				}
				hasCond := book.CleanConditionExpr(ch.Condition) != ""
				fmt.Fprintf(&sb, "port | scene:%s | idx:%d | label:%s | target_id:%s | target_chapter:%s | cond:%d\n",
					sanitizeGraphPayloadField(id),
					chIdx,
					sanitizeGraphPayloadField(lbl),
					sanitizeGraphPayloadField(ch.TargetSceneID),
					sanitizeGraphPayloadField(targetChapterRef),
					boolToPayloadBit(hasCond))
			}
		}
	}

	if len(b.WirePins) > 0 {
		wireKeys := make([]string, 0, len(b.WirePins))
		for k, pins := range b.WirePins {
			if k != "" && len(pins) > 0 {
				wireKeys = append(wireKeys, k)
			}
		}
		sort.Strings(wireKeys)
		for _, k := range wireKeys {
			pins := b.WirePins[k]
			var parts []string
			for _, p := range pins {
				parts = append(parts, fmt.Sprintf("%d,%d", p.X, p.Y))
			}
			fmt.Fprintf(&sb, "wire_pins | key:%s | pins:%s\n",
				sanitizeGraphPayloadField(k),
				strings.Join(parts, ";"))
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}
