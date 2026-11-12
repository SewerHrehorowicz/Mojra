//go:build !(js && wasm)

package views

import (
	"fmt"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

type sceneMediaOptionsView struct {
	curImg        string
	curImgLabel   string
	imgOptions    []map[string]any
	curVoice      string
	curVoiceLabel string
	voiceOptions  []map[string]any
}

func ViewEditorContextPanel(s *state.AppState) map[string]any {
	vars := make(map[string]any, 64)
	populateEditorCommonVars(s, vars, false)
	if s.Editor.HasActiveBook() && len(s.Editor.ActiveBook.Scenes) > 0 {
		populateEditorSelectedNodeOnly(s, vars)
	}
	return vars
}

func buildChoiceTargetOptions(s *state.AppState, sc *book.Scene, ch book.Choice) string {
	selectedVal := "END"
	b := s.Editor.ActiveBook
	if b == nil {
		return selectedVal
	}
	curCh := ""
	if sc != nil {
		curCh = strings.TrimSpace(sc.Chapter)
	}
	if curCh == "" {
		curCh = b.DefaultChapterName()
	}

	matchedOption := false

	// 1. Same-chapter scenes
	for _, cand := range b.Scenes {
		candCh := strings.TrimSpace(cand.Chapter)
		if candCh == "" {
			candCh = b.DefaultChapterName()
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
	for _, chItem := range b.Chapters {
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

func buildSceneMediaOptions(s *state.AppState, sc *book.Scene, buildMenus bool) sceneMediaOptionsView {
	b := s.Editor.ActiveBook
	curImg := sc.ExplicitImage
	if curImg == "" && sc.ImageFile != "" {
		curImg = sc.ImageFile
	}
	noneLabel := tr(s, i18n.TrMediaNoneLabel)
	curImgLabel := noneLabel
	if curImg != "" {
		curImgLabel = curImg
	}
	var imgOptions []map[string]any
	if buildMenus && b != nil {
		imgOptions = make([]map[string]any, 0, len(b.ImageFiles)+2)
		imgOptions = append(imgOptions, map[string]any{
			"value":     "",
			"label":     noneLabel,
			"is_active": curImg == "",
		})
		matchedImg := curImg == ""
		for _, imgFile := range b.ImageFiles {
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
			"label":     tr(s, i18n.TrAddImageFromDiskOpt),
			"is_active": false,
		})
	}

	curVoice := ""
	curVoiceLabel := noneLabel
	var voiceOptions []map[string]any
	if b != nil && b.Narrated {
		curVoice = sc.ExplicitVoice
		if curVoice == "" && sc.AudioFile != "" {
			curVoice = sc.AudioFile
		}
		if curVoice != "" {
			curVoiceLabel = curVoice
		}
		if buildMenus {
			voiceOptions = make([]map[string]any, 0, len(b.AudioFiles)+3)
			voiceOptions = append(voiceOptions, map[string]any{
				"value":     "",
				"label":     noneLabel,
				"is_active": curVoice == "",
			})
			matchedVoice := curVoice == ""
			for _, audioFile := range b.AudioFiles {
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
				"label":     tr(s, i18n.TrAddAudioFromDiskOpt),
				"is_active": false,
			})
		}
	}
	return sceneMediaOptionsView{
		curImg:        curImg,
		curImgLabel:   curImgLabel,
		imgOptions:    imgOptions,
		curVoice:      curVoice,
		curVoiceLabel: curVoiceLabel,
		voiceOptions:  voiceOptions,
	}
}

func buildSceneVoiceSectionsView(s *state.AppState, sc *book.Scene) (hasMultiple bool, activeSec book.VoiceSection, sectionItems []map[string]any) {
	if sc == nil {
		return false, book.VoiceSection{Key: "01", TrimStart: 0, TrimEnd: 1, DurationLabel: "0:00"}, nil
	}
	sections := book.ExtractSceneVoiceSections(sc, s.Editor.ActiveBook)
	if len(sections) == 0 {
		return false, book.VoiceSection{Key: "01", TrimStart: 0, TrimEnd: 1, DurationLabel: "0:00"}, nil
	}
	hasMultiple = len(sections) > 1
	activeIdx := 0
	selKey := strings.TrimSpace(s.Editor.SelectedVoiceSectionKey)
	if selKey != "" {
		for i := range sections {
			if strings.EqualFold(sections[i].Key, selKey) {
				activeIdx = i
				break
			}
		}
	}
	activeSec = sections[activeIdx]
	s.Editor.SelectedVoiceSectionKey = activeSec.Key

	sectionItems = make([]map[string]any, len(sections))
	for i, sec := range sections {
		isActive := i == activeIdx
		inspLabel := sec.ShortLabel
		if isActive {
			inspLabel = sec.Label
		}
		sectionItems[i] = map[string]any{
			"key":             sec.Key,
			"kind":            sec.Kind,
			"label":           sec.Label,
			"inspector_label": inspLabel,
			"audio_url":       sec.AudioURL,
			"duration_label":  sec.DurationLabel,
			"has_audio":       sec.HasAudio,
			"is_active":       isActive,
		}
	}
	return hasMultiple, activeSec, sectionItems
}

func buildEditorNodeMap(s *state.AppState, sc *book.Scene, isSelected bool) map[string]any {
	b := s.Editor.ActiveBook
	isStart := false
	isChapterEntry := false
	if b != nil {
		if b.StartSceneID != "" {
			isStart = strings.EqualFold(sc.ID, b.StartSceneID)
		} else if len(b.Scenes) > 0 {
			isStart = sc.ID == b.Scenes[0].ID
		}
		isChapterEntry = isStart
		if !isChapterEntry {
			if chMeta := b.FindChapter(sc.Chapter); chMeta != nil && chMeta.StartSceneID != "" {
				isChapterEntry = strings.EqualFold(sc.ID, chMeta.StartSceneID)
			}
		}
	}

	hasChoices := len(sc.Choices) >= 2
	inspectorChoices := sc.Choices
	if len(inspectorChoices) == 0 || sc.ExplicitEnd {
		inspectorChoices = []book.Choice{
			{
				Label:      tr(s, i18n.TrNext),
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
			lbl = tr(s, i18n.TrNext)
		}
		selectedVal := "END"
		if isSelected {
			selectedVal = buildChoiceTargetOptions(s, sc, ch)
		}

		portMap, _ := buildChoiceRuleViewMap(s, sc, chIdx, ch, selectedVal)
		portMap["raw_label"] = lbl
		inspectorPorts = append(inspectorPorts, portMap)
	}

	media := buildSceneMediaOptions(s, sc, isSelected)
	curImg := media.curImg
	curImgLabel := media.curImgLabel
	imgOptions := media.imgOptions
	curVoice := media.curVoice
	curVoiceLabel := media.curVoiceLabel
	voiceOptions := media.voiceOptions

	nodeWarns := make([]map[string]any, 0, len(sc.Warnings))
	for _, w := range sc.Warnings {
		nodeWarns = append(nodeWarns, map[string]any{"text": w})
	}

	curChName := strings.TrimSpace(sc.Chapter)
	if curChName == "" && b != nil {
		curChName = b.DefaultChapterName()
	}
	var chapterOptions []map[string]any
	if isSelected && b != nil {
		chapterOptions = make([]map[string]any, 0, len(b.Chapters)+1)
		matchedCh := false
		for _, chItem := range b.Chapters {
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
	if b != nil && b.IsScreenplay() {
		metrics := book.ComputeSceneScriptMetrics(sc, sc.OrderIndex+1, b.NormalizedStandard())
		scriptLengthLabel = metrics.FormattedLength
	}

	scEffectsVal := book.CleanEffectsExpr(sc.Effects)
	scEffectsErr := ""
	if scEffectsVal != "" {
		scEffectsErr = book.ExplainEffectsExprErrorLang(sc.Effects, b, s.Settings.Language)
	}
	voiceURL := sc.AudioURL
	voiceDurLabel := "0:00"
	if isSelected && curVoice != "" && b != nil {
		durLbl, stamp := book.ReadSceneAudioDurationAndStamp(b, sc)
		voiceDurLabel = durLbl
		if voiceURL != "" && stamp > 0 {
			voiceURL = fmt.Sprintf("%s?v=%d", voiceURL, stamp)
		}
	}
	trimStart, trimEnd := sc.NormalizedVoiceTrim()
	hasVoiceFile := curVoice != "" && sc.AudioURL != ""
	hasVoiceTrim := hasVoiceFile && sc.HasVoiceTrim()
	hasVoiceSections := false
	activeVoiceSecKey := "01"
	var voiceSectionItems []map[string]any

	if isSelected {
		hasMultiSec, activeSec, secItems := buildSceneVoiceSectionsView(s, sc)
		hasVoiceSections = hasMultiSec
		activeVoiceSecKey = activeSec.Key
		voiceSectionItems = secItems
		if hasMultiSec {
			curVoice = activeSec.AudioFile
			if curVoice == "" {
				curVoice = activeSec.ExplicitVoice
			}
			if curVoice != "" {
				curVoiceLabel = curVoice
			} else if s.Modals.VoiceRecorder && activeSec.TargetFile != "" {
				curVoiceLabel = activeSec.TargetFile
			} else {
				curVoiceLabel = tr(s, i18n.TrMediaNoneLabel)
			}
			voiceURL = activeSec.AudioURL
			if activeSec.HasAudio {
				voiceDurLabel = activeSec.DurationLabel
			} else {
				voiceDurLabel = "0:00"
			}
			trimStart, trimEnd = activeSec.TrimStart, activeSec.TrimEnd
			hasVoiceFile = activeSec.HasAudio
			hasVoiceTrim = activeSec.HasTrim
			for i := range voiceOptions {
				val, _ := voiceOptions[i]["value"].(string)
				voiceOptions[i]["is_active"] = strings.EqualFold(val, curVoice)
			}
		}
	}

	portCount := 0
	if hasChoices {
		portCount = len(inspectorChoices)
	}

	return map[string]any{
		"id":                        sc.ID,
		"explicit_id":               sc.ExplicitID,
		"current_image":             curImg,
		"current_image_label":       curImgLabel,
		"image_options":             imgOptions,
		"current_voice":             curVoice,
		"current_voice_label":       curVoiceLabel,
		"current_voice_url":         voiceURL,
		"voice_duration_label":      voiceDurLabel,
		"voice_trim_start":          fmt.Sprintf("%.4f", trimStart),
		"voice_trim_end":            fmt.Sprintf("%.4f", trimEnd),
		"has_voice_file":            hasVoiceFile,
		"has_voice_trim":            hasVoiceTrim,
		"voice_options":             voiceOptions,
		"has_voice_sections":        hasVoiceSections,
		"active_voice_section_key":  activeVoiceSecKey,
		"voice_sections":            voiceSectionItems,
		"name":                      sc.Name,
		"slugline":                  sc.Slugline,
		"synopsis":                  sc.Synopsis,
		"effects":                   scEffectsVal,
		"has_effects":               scEffectsVal != "",
		"is_effects_invalid":        scEffectsErr != "",
		"effects_error_hint":        scEffectsErr,
		"script_length_label":       scriptLengthLabel,
		"chapter":                   curChName,
		"chapter_options":           chapterOptions,
		"source_file":               sc.SourceFile,
		"is_start_scene":            isStart,
		"is_chapter_entry":          isChapterEntry,
		"has_node_warnings":         len(sc.Warnings) > 0,
		"node_warnings":             nodeWarns,
		"inspector_ports":           inspectorPorts,
		"port_count":                portCount,
	}
}

func populateEditorBookAndChapterSettings(s *state.AppState, vars map[string]any) {
	b := s.Editor.ActiveBook
	if b == nil {
		vars["book_settings_open"] = false
		vars["has_selected_chapter"] = false
		vars["is_node_inspector_hidden"] = false
		vars["multi_scene_items"] = nil
		return
	}

	isScreenplay := b.IsScreenplay()
	normStd := b.NormalizedStandard()
	_, stdOpts := buildScriptStandardOptions(normStd)
	prefixPills, timePills := buildSluglineHelperPills(normStd)

	vars["is_screenplay_mode"] = isScreenplay
	vars["voice_recorder_open"] = s.Modals.VoiceRecorder
	vars["script_standard"] = normStd
	vars["script_standard_options"] = stdOpts
	vars["slugline_prefix_pills"] = prefixPills
	vars["slugline_time_pills"] = timePills

	multiSceneItems := make([]map[string]any, 0, len(b.Scenes))
	for _, sc := range b.Scenes {
		scCh := strings.TrimSpace(sc.Chapter)
		if scCh == "" {
			scCh = b.DefaultChapterName()
		}
		multiSceneItems = append(multiSceneItems, map[string]any{
			"id":      sc.ID,
			"title":   sc.DisplayName(),
			"chapter": scCh,
		})
	}
	vars["multi_scene_items"] = multiSceneItems

	bookSettingsOpen := s.Editor.RightPanel == state.PanelBook
	vars["book_settings_open"] = bookSettingsOpen
	vars["book"] = b

	startSceneID := b.StartSceneID
	if startSceneID == "" && len(b.Scenes) > 0 {
		startSceneID = b.Scenes[0].ID
	}
	startScene := b.FindScene(startSceneID)
	if startScene == nil && len(b.Scenes) > 0 {
		startScene = b.Scenes[0]
		startSceneID = startScene.ID
	}
	startChapterName := b.DefaultChapterName()
	if startScene != nil && strings.TrimSpace(startScene.Chapter) != "" {
		startChapterName = strings.TrimSpace(startScene.Chapter)
	} else if len(b.Chapters) > 0 && strings.TrimSpace(b.Chapters[0].Name) != "" {
		startChapterName = strings.TrimSpace(b.Chapters[0].Name)
	}

	startChapterOptions := make([]map[string]any, 0, len(b.Chapters))
	for _, ch := range b.Chapters {
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
	startSceneOptions := make([]map[string]any, 0, len(b.Scenes))
	for _, sc := range b.Scenes {
		scCh := strings.TrimSpace(sc.Chapter)
		if scCh == "" {
			scCh = b.DefaultChapterName()
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
	if s.Editor.SelectedChapter != "" {
		selCh = b.FindChapter(s.Editor.SelectedChapter)
	}
	hasSelectedChapter := selCh != nil && s.Editor.SelectedChapter != "" && !bookSettingsOpen
	if selCh == nil && len(b.Chapters) > 0 {
		selCh = &b.Chapters[0]
	}

	if selCh != nil {
		chName := strings.TrimSpace(selCh.Name)
		if chName == "" {
			chName = b.DefaultChapterName()
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
			if b.IsScreenplay() {
				metrics := book.ComputeSceneScriptMetrics(sc, sc.OrderIndex+1, b.NormalizedStandard())
				totalEighths += metrics.Eighths
			}
			if len(sc.Choices) >= 2 {
				branchCount += len(sc.Choices)
			}
			for _, ch := range sc.Choices {
				if ch.IsEnding || ch.IsChapterLink {
					exitCount++
				} else if ch.TargetSceneID != "" {
					if targetSc := b.FindScene(ch.TargetSceneID); targetSc != nil && !strings.EqualFold(strings.TrimSpace(targetSc.Chapter), chName) {
						exitCount++
					}
				}
			}
		}
		lengthStat := trf(s, i18n.TrWordsCountFmt, totalWords)
		if b.IsScreenplay() {
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
	isContextPanelHidden := !bookSettingsOpen && !hasSelectedChapter
	vars["is_context_panel_hidden"] = isContextPanelHidden
	vars["show_editor_help_panel"] = s.Editor.RightPanel == state.PanelHelp
	vars["show_syntax_help_popover"] = s.Editor.SceneText.SyntaxHelpOpen
	populateEditorVariablesContext(s, vars)
}

func buildChoiceRuleViewMap(s *state.AppState, sc *book.Scene, idx int, ch book.Choice, selectedVal string) (map[string]any, bool) {
	b := s.Editor.ActiveBook
	sceneID := ""
	if sc != nil {
		sceneID = sc.ID
	}
	condInputVal := book.CleanConditionExpr(ch.Condition)
	if ch.ShowWhenLocked && condInputVal != "" && !strings.HasPrefix(condInputVal, "?") {
		condInputVal = "? " + condInputVal
	}
	effInputVal := book.CleanEffectsExpr(ch.Effects)
	hasCond := condInputVal != ""
	hasEff := effInputVal != ""
	hasRule := hasCond || hasEff
	hasBookVars := b != nil && len(b.Variables) > 0
	ruleKey := fmt.Sprintf("%s:%d", sceneID, idx)
	isRuleOpen := hasBookVars
	if hasBookVars && s.Editor.SceneText.ChoiceRulesOpen != nil {
		if expOpen, exists := s.Editor.SceneText.ChoiceRulesOpen[ruleKey]; exists {
			isRuleOpen = expOpen
		}
	}
	exitErrHint := ""
	if sc != nil {
		exitErrHint = book.ExplainExitTargetErrorLang(sc, selectedVal, b, s.Settings.Language)
	}
	condErrHint := ""
	if hasCond {
		condErrHint = book.ExplainConditionExprErrorLang(ch.Condition, b, s.Settings.Language)
	}
	effErrHint := ""
	if hasEff {
		effErrHint = book.ExplainEffectsExprErrorLang(ch.Effects, b, s.Settings.Language)
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
		"is_exit_invalid": exitErrHint != "",
		"exit_error_hint": exitErrHint,
		"is_cond_invalid": condErrHint != "",
		"is_eff_invalid":  effErrHint != "",
		"cond_error_hint": condErrHint,
		"eff_error_hint":  effErrHint,
	}, hasCond
}

func populateEditorSelectedNodeOnly(s *state.AppState, vars map[string]any) {
	b := s.Editor.ActiveBook
	selected := b.FindScene(s.Editor.SelectedSceneID)
	if selected == nil {
		selected = b.Scenes[0]
		s.Editor.SelectedSceneID = selected.ID
	}
	vars["selected_scene_id"] = selected.ID
	vars["selected_scene_title"] = selected.DisplayName()
	selectedNodeMap := buildEditorNodeMap(s, selected, true)
	vars["selected_node"] = selectedNodeMap
	vars["has_selected_node"] = selectedNodeMap != nil
	populateEditorBookAndChapterSettings(s, vars)
}
