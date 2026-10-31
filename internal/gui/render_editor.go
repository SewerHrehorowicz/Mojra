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
		sel := "false"
		actCls := ""
		if !ch.IsEnding && !ch.IsChapterLink && ch.TargetSceneID == cand.ID {
			sel = "true"
			actCls = "active"
			matchedOption = true
		}
		optVal := cand.ReferenceKey()
		optLabel := cand.DisplayName()
		if cand.ExplicitID != "" {
			if strings.EqualFold(ch.Target, cand.ExplicitID) {
				optVal = cand.ExplicitID
			} else if sel == "true" {
				optVal = cand.ExplicitID
			}
			if cand.Name != "" {
				optLabel = fmt.Sprintf("%s [id: %s]", cand.Name, cand.ExplicitID)
			} else {
				optLabel = fmt.Sprintf("[id: %s]", cand.ExplicitID)
			}
		}
		if sel == "true" {
			selectedVal = optVal
			selectedLabel = cand.DisplayName()
		}
		targetOptions = append(targetOptions, map[string]any{
			"value":        optVal,
			"short_label":  cand.DisplayName(),
			"label":        optLabel,
			"selected":     sel,
			"active_class": actCls,
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
		sel := "false"
		actCls := ""
		if isCurrentSel {
			sel = "true"
			actCls = "active"
			matchedOption = true
		}
		shortLbl := "↪ Rozdział: " + chItem.DisplayName()
		optLabel := shortLbl
		if chItem.ExplicitID != "" && !strings.EqualFold(chItem.DisplayName(), chItem.ExplicitID) {
			optLabel = fmt.Sprintf("↪ Rozdział: %s [chapter:%s]", chItem.DisplayName(), chItem.ExplicitID)
		}
		if sel == "true" {
			selectedVal = optVal
			selectedLabel = shortLbl
		}
		targetOptions = append(targetOptions, map[string]any{
			"value":        optVal,
			"short_label":  shortLbl,
			"label":        optLabel,
			"selected":     sel,
			"active_class": actCls,
		})
	}

	// 3. END
	endSel := "false"
	endAct := ""
	if ch.IsEnding || strings.EqualFold(ch.Target, "END") {
		endSel = "true"
		endAct = "active"
		matchedOption = true
		selectedVal = "END"
		selectedLabel = "KONIEC (END)"
	}
	targetOptions = append(targetOptions, map[string]any{
		"value":        "END",
		"short_label":  "KONIEC (END)",
		"label":        "KONIEC (END)",
		"selected":     endSel,
		"active_class": endAct,
	})
	if !matchedOption && ch.Target != "" {
		selectedVal = ch.Target
		selectedLabel = "⚠ " + ch.Target
		targetOptions = append(targetOptions, map[string]any{
			"value":        ch.Target,
			"short_label":  "⚠ " + ch.Target,
			"label":        "⚠ " + ch.Target,
			"selected":     "true",
			"active_class": "active",
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
		noneImgAct := ""
		if curImg == "" {
			noneImgAct = "active"
		}
		imgOptions = append(imgOptions, map[string]any{
			"value":        "",
			"label":        "— brak —",
			"active_class": noneImgAct,
		})
		matchedImg := curImg == ""
		for _, imgFile := range s.ActiveBook.ImageFiles {
			act := ""
			if strings.EqualFold(curImg, imgFile) {
				act = "active"
				matchedImg = true
				curImgLabel = imgFile
			}
			imgOptions = append(imgOptions, map[string]any{
				"value":        imgFile,
				"label":        imgFile,
				"active_class": act,
			})
		}
		if !matchedImg && curImg != "" {
			curImgLabel = "⚠ " + curImg
			imgOptions = append(imgOptions, map[string]any{
				"value":        curImg,
				"label":        "⚠ " + curImg,
				"active_class": "active",
			})
		}
		imgOptions = append(imgOptions, map[string]any{
			"value":        "__browse_disk__",
			"label":        "+ Dodaj obraz z dysku...",
			"active_class": "",
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
			noneVoiceAct := ""
			if curVoice == "" {
				noneVoiceAct = "active"
			}
			voiceOptions = append(voiceOptions, map[string]any{
				"value":        "",
				"label":        "— brak —",
				"active_class": noneVoiceAct,
			})
			matchedVoice := curVoice == ""
			for _, audioFile := range s.ActiveBook.AudioFiles {
				act := ""
				if strings.EqualFold(curVoice, audioFile) {
					act = "active"
					matchedVoice = true
					curVoiceLabel = audioFile
				}
				voiceOptions = append(voiceOptions, map[string]any{
					"value":        audioFile,
					"label":        audioFile,
					"active_class": act,
				})
			}
			if !matchedVoice && curVoice != "" {
				curVoiceLabel = "⚠ " + curVoice
				voiceOptions = append(voiceOptions, map[string]any{
					"value":        curVoice,
					"label":        "⚠ " + curVoice,
					"active_class": "active",
				})
			}
			voiceOptions = append(voiceOptions, map[string]any{
				"value":        "__browse_disk__",
				"label":        "+ Dodaj nagranie z dysku...",
				"active_class": "",
			})
		}
	}
	return
}

func (s *Server) buildEditorNodeMap(sc *book.Scene, isSelected bool) map[string]any {
	selClass := ""
	if isSelected {
		selClass = "node-selected"
	}

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
	startClass := ""
	if isStart {
		startClass = "node-is-start"
	} else if isChapterEntry {
		startClass = "node-is-chapter-entry"
	}
	unreachableClass := ""
	if sc.Unreachable {
		unreachableClass = "node-unreachable"
	}

	audioClass := ""
	audioBadge := "voice: wyłączony"
	if s.ActiveBook != nil && s.ActiveBook.Narrated {
		if sc.AudioFile != "" {
			audioClass = "badge-ok"
			audioBadge = "voice: " + sc.AudioFile
		} else {
			audioClass = "badge-warn"
			audioBadge = "⚠ BRAK VOICE!"
		}
	}

	imgClass := ""
	imgBadge := "image: opcjonalny"
	if sc.ImageFile != "" {
		imgClass = "badge-ok"
		imgBadge = "image: " + sc.ImageFile
	} else if sc.ExplicitImage != "" {
		imgClass = "badge-warn"
		imgBadge = "⚠ BRAK IMAGE!"
	}

	hasChoices := len(sc.Choices) >= 2
	ports := make([]map[string]any, 0, len(sc.Choices))
	var nodeChoiceLines []string
	if hasChoices {
		for chIdx, ch := range sc.Choices {
			if !ch.IsImplicit && ch.Label != "" && ch.Target != "" {
				nodeChoiceLines = append(nodeChoiceLines, ch.Label+" -> "+ch.Target)
			}
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
			portClass := ""
			if !ch.IsEnding && ch.TargetSceneID != "" && !strings.EqualFold(ch.TargetSceneID, "END") && !strings.EqualFold(selectedVal, "END") {
				portClass = "port-filled"
			}

			ports = append(ports, map[string]any{
				"idx":            chIdx,
				"raw_label":      ch.Label,
				"label":          lbl,
				"target":         tgt,
				"target_id":      ch.TargetSceneID,
				"target_chapter": targetChapterRef,
				"is_implicit":    ch.IsImplicit,
				"selected_value": selectedVal,
				"selected_label": selectedLabel,
				"port_class":     portClass,
				"target_options": targetOptions,
			})
		}
	}
	primaryChild := ""
	if len(sc.Choices) > 0 && !sc.Choices[0].IsEnding {
		primaryChild = sc.Choices[0].TargetSceneID
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

	expectedStem := sc.Name
	if sc.ExplicitID != "" {
		expectedStem = sc.ExplicitID
	}

	curImg, curImgLabel, imgOptions, curVoice, curVoiceLabel, voiceOptions := s.buildSceneMediaOptions(sc, isSelected)

	nodeWarns := make([]map[string]any, 0, len(sc.Warnings))
	for _, w := range sc.Warnings {
		nodeWarns = append(nodeWarns, map[string]any{"text": w})
	}

	curChName := strings.TrimSpace(sc.Chapter)
	if curChName == "" {
		curChName = "Rozdział 1"
	}
	var chapterOptions []map[string]any
	if isSelected {
		chapterOptions = make([]map[string]any, 0, len(s.ActiveBook.Chapters)+1)
		matchedCh := false
		for _, chItem := range s.ActiveBook.Chapters {
			act := ""
			if strings.EqualFold(curChName, strings.TrimSpace(chItem.Name)) {
				act = "active"
				matchedCh = true
			}
			chapterOptions = append(chapterOptions, map[string]any{
				"value":        chItem.Name,
				"label":        chItem.Name,
				"active_class": act,
			})
		}
		if !matchedCh && curChName != "" {
			chapterOptions = append(chapterOptions, map[string]any{
				"value":        curChName,
				"label":        curChName,
				"active_class": "active",
			})
		}
	}

	return map[string]any{
		"id":                  sc.ID,
		"display_name":        sc.DisplayName(),
		"explicit_id":         sc.ExplicitID,
		"explicit_image":      sc.ExplicitImage,
		"explicit_voice":      sc.ExplicitVoice,
		"current_image":       curImg,
		"current_image_label": curImgLabel,
		"image_options":       imgOptions,
		"current_voice":       curVoice,
		"current_voice_label": curVoiceLabel,
		"voice_options":       voiceOptions,
		"book_narrated":       s.ActiveBook != nil && s.ActiveBook.Narrated,
		"image_placeholder":   fmt.Sprintf("image: np. images/%s.png (opcjonalny)", expectedStem),
		"voice_placeholder":   fmt.Sprintf("voice: np. voice/%s.ogg lub %s.mp3", expectedStem, expectedStem),
		"name":                sc.Name,
		"chapter":             curChName,
		"chapter_options":     chapterOptions,
		"row":                 sc.Row,
		"col":                 sc.Col,
		"x":                   sc.X,
		"y":                   sc.Y,
		"text":                sc.Text,
		"first_sentence":      extractFirstSentence(sc.Text),
		"choices_text":        strings.Join(nodeChoiceLines, "\n"),
		"source_file":         sc.SourceFile,
		"selected_class":          selClass,
		"is_start_scene":          isStart,
		"is_chapter_entry":        isChapterEntry,
		"is_non_root_entry":       isChapterEntry && !isStart,
		"start_node_class":        startClass,
		"is_unreachable":          sc.Unreachable,
		"unreachable_class":       unreachableClass,
		"audio_badge_class":       audioClass,
		"audio_badge_text":        audioBadge,
		"img_badge_class":         imgClass,
		"img_badge_text":          imgBadge,
		"has_node_warnings":       len(sc.Warnings) > 0,
		"node_warnings":           nodeWarns,
		"ports":                   ports,
		"has_choices":             hasChoices,
		"has_ports":               len(ports) > 0,
		"port_count":              len(ports),
		"primary_child_id":        primaryChild,
		"implicit_target_id":      implicitTargetID,
		"implicit_target_chapter": implicitTargetChapter,
	}
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
}

func (s *Server) populateEditorContext(vars map[string]any) {
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
		if chItem.ExplicitID != "" {
			chBadge = "ROZDZIAŁ • " + chItem.ExplicitID
		} else if chItem.ID != "" {
			chBadge = "ROZDZIAŁ • " + chItem.ID
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
	vars["editor_pan_x"] = s.ActiveBook.EditorPanX
	vars["editor_pan_y"] = s.ActiveBook.EditorPanY
	vars["editor_zoom"] = fmt.Sprintf("%.2f", edZoom)
	vars["has_editor_view"] = s.ActiveBook.HasEditorView
	vars["studio_graph_payload"] = buildStudioGraphPayload(s.ActiveBook, edZoom, chapterGroups, allNodes)

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
		selClass, _ := nd["selected_class"].(string)
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
			boolToPayloadBit(selClass != ""),
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

func (s *Server) populateEditorHistoryContext(vars map[string]any) {
	canUndo := false
	canRedo := false
	undoTitle := "Brak zmian do cofnięcia od ostatniej wersji (Ctrl+Z)"
	redoTitle := "Brak zmian do ponowienia (Ctrl+Shift+Z)"
	unnamedCount := 0
	var unnamedItems []map[string]any
	var namedVersions []map[string]any

	if s.ActiveBook != nil && !s.ActiveBook.IsPacked && !s.StandaloneMode {
		if s.EditorHistoryPanelOpen {
			s.ensureHistoryBaselineBeforeMutation()
		}
		if h := s.loadActiveBookHistory(); h != nil {
			canUndo = h.CanUndo()
			canRedo = h.CanRedo()
			unnamedCount = h.UnnamedEditsCount()
			curIdx := h.CursorIndex()
			lastNamedIdx := h.LastNamedIndex()

			if canUndo && curIdx >= 0 && curIdx < len(h.Revisions) {
				undoTitle = "Cofnij: " + h.Revisions[curIdx].Label + " (Ctrl+Z)"
			}
			if canRedo && curIdx >= 0 && curIdx+1 < len(h.Revisions) {
				redoTitle = "Ponów: " + h.Revisions[curIdx+1].Label + " (Ctrl+Shift+Z)"
			}

			floorIdx := lastNamedIdx
			if floorIdx < 0 {
				floorIdx = 0
			}
			hasAnyUnnamed := false
			for i := floorIdx + 1; i < len(h.Revisions); i++ {
				if !h.Revisions[i].Named {
					hasAnyUnnamed = true
					break
				}
			}
			if hasAnyUnnamed && len(h.Revisions) > 0 {
				for i := len(h.Revisions) - 1; i >= floorIdx; i-- {
					rev := h.Revisions[i]
					isCurrent := i == curIdx
					isUndone := i > curIdx
					actClass := ""
					statusLabel := ""
					if isCurrent {
						actClass = "history-item-current"
						statusLabel = "AKTYWNY"
					} else if isUndone {
						actClass = "history-item-undone"
						statusLabel = "COFNIĘTO"
					} else if rev.Named {
						statusLabel = "WERSJA BAZOWA"
					}
					unnamedItems = append(unnamedItems, map[string]any{
						"id":           rev.ID,
						"label":        rev.Label,
						"time_label":   formatHistoryTimestamp(rev.TimeUnix),
						"kind":         rev.Kind,
						"is_current":   isCurrent,
						"is_undone":    isUndone,
						"is_floor":     rev.Named,
						"active_class": actClass,
						"status_label": statusLabel,
					})
				}
			}

			for i := len(h.Revisions) - 1; i >= 0; i-- {
				rev := h.Revisions[i]
				if !rev.Named {
					continue
				}
				isCurrent := i == curIdx
				isLatestNamed := i == lastNamedIdx
				actClass := ""
				if isCurrent {
					actClass = "history-item-current"
				}
				namedVersions = append(namedVersions, map[string]any{
					"id":              rev.ID,
					"label":           rev.Label,
					"time_label":      formatHistoryTimestamp(rev.TimeUnix),
					"is_current":      isCurrent,
					"is_latest_named": isLatestNamed,
					"active_class":    actClass,
				})
			}
		}
		if len(namedVersions) == 0 {
			namedVersions = append(namedVersions, map[string]any{
				"id":              "r1",
				"label":           "Stan początkowy",
				"time_label":      formatHistoryTimestamp(time.Now().Unix()),
				"is_current":      true,
				"is_latest_named": true,
				"active_class":    "history-item-current",
			})
		}
	}

	undoDisabledClass := "is-disabled"
	undoDisabledAttr := "true"
	if canUndo {
		undoDisabledClass = ""
		undoDisabledAttr = "false"
	}
	redoDisabledClass := "is-disabled"
	redoDisabledAttr := "true"
	if canRedo {
		redoDisabledClass = ""
		redoDisabledAttr = "false"
	}
	hasUnnamed := unnamedCount > 0
	badgeHiddenClass := "hidden"
	if hasUnnamed {
		badgeHiddenClass = ""
	}
	panelActiveClass := ""
	panelHiddenClass := "hidden"
	if s.EditorHistoryPanelOpen {
		panelActiveClass = "active"
		panelHiddenClass = ""
	}

	vars["can_undo"] = canUndo
	vars["can_redo"] = canRedo
	vars["undo_disabled"] = undoDisabledAttr
	vars["redo_disabled"] = redoDisabledAttr
	vars["can_undo_disabled"] = undoDisabledAttr
	vars["can_redo_disabled"] = redoDisabledAttr
	vars["undo_disabled_class"] = undoDisabledClass
	vars["redo_disabled_class"] = redoDisabledClass
	vars["undo_title"] = undoTitle
	vars["redo_title"] = redoTitle
	vars["has_unnamed_edits"] = hasUnnamed
	vars["unnamed_edits_count"] = unnamedCount
	vars["history_badge_hidden_class"] = badgeHiddenClass
	vars["show_editor_history_panel"] = s.EditorHistoryPanelOpen
	vars["history_panel_active_class"] = panelActiveClass
	vars["history_panel_hidden_class"] = panelHiddenClass
	vars["history_unnamed_items"] = unnamedItems
	vars["has_history_unnamed_items"] = len(unnamedItems) > 0
	vars["history_named_versions"] = namedVersions
	vars["has_history_named_versions"] = len(namedVersions) > 0
	vars["default_next_version_name"] = fmt.Sprintf("Wersja %d", len(namedVersions)+1)
}
