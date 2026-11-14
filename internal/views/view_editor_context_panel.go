//go:build !(js && wasm)

package views

import (
	"fmt"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type editorContextPanelTemplate struct {
	HasEditorBook                 bool
	Book                          *book.Book
	DefaultNewBookDir             string
	DefaultExportDir              string
	IsScreenplayMode              bool
	ScriptStandard                string
	EditorTitlePlaceholder        string
	EditorAuthorPlaceholder       string
	EditorDescriptionPlaceholder  string
	EditorChapterTitlePlaceholder string
	EditorSettingsHint            string
	IsHistoryPreview              bool
	HistoryPreviewAttr            string
	HistoryPreviewID              string
	HistoryPreviewLabel           string
	BookNarrated                  bool
	BookNarratedLabel             string
	BookNarratedShortLabel        string
	BookImageless                 bool
	BookImagelessLabel            string
	BookImagelessShortLabel       string
	HasBookVariables              bool
	BookVarsSpec                  string
	BookVarSuggestions            []book.VariableDef
	BookEnumSuggestions           []book.SelectOption
	BookTargetSuggestions         []book.SelectOption
	SyntaxHelpCondRows            []book.SelectOption
	SelectedSceneID               string
	SelectedSceneTitle            string
	SelectedNode                  *book.EditorNodeView
	HasSelectedNode               bool
	VoiceRecorderOpen             bool
	ScriptStandardOptions         []book.SelectOption
	SluglinePrefixPills           []string
	SluglineTimePills             []string
	MultiSceneItems               []book.SelectOption
	BookSettingsOpen              bool
	BookStartChapterName          string
	BookStartChapterOptions       []book.SelectOption
	BookStartSceneID              string
	BookStartSceneLabel           string
	BookStartSceneOptions         []book.SelectOption
	SelectedChapter               *book.Chapter
	HasSelectedChapter            bool
	IsNodeInspectorHidden         bool
	IsContextPanelHidden          bool
	ShowEditorHelpPanel           bool
	ShowSyntaxHelpPopover         bool
}

type editorContextPanelScript struct {
	selectedSceneId string
	bookVarsSpec    string
	isScreenplay    bool
	scriptStandard  string
}

func populateEditorCommonFields(s *state.AppState, v *editorContextPanelTemplate) {
	b := s.Editor.ActiveBook
	hasEditorBook := s.Editor.HasActiveBook()
	v.HasEditorBook = hasEditorBook
	v.Book = b
	v.DefaultNewBookDir = book.ResolveNewBookDir(s.Settings)
	v.DefaultExportDir = b.ResolveExportDir(s.Settings)
	v.ScriptStandard = "pl"
	v.HistoryPreviewAttr = attrFalse
	if hasEditorBook {
		v.IsScreenplayMode = b.IsScreenplay()
		v.ScriptStandard = b.NormalizedStandard()
	}
	if v.IsScreenplayMode {
		v.EditorTitlePlaceholder = tr(s, i18n.TrEditorSpTitlePlaceholder)
		v.EditorAuthorPlaceholder = tr(s, i18n.TrEditorSpAuthorPlaceholder)
		v.EditorDescriptionPlaceholder = tr(s, i18n.TrEditorSpDescPlaceholder)
		v.EditorChapterTitlePlaceholder = tr(s, i18n.TrEditorActTitlePlaceholder)
		v.EditorSettingsHint = tr(s, i18n.TrEditorSpSettingsHint)
	} else {
		v.EditorTitlePlaceholder = tr(s, i18n.TrEditorBookTitlePlaceholder)
		v.EditorAuthorPlaceholder = tr(s, i18n.TrEditorBookAuthorPlaceholder)
		v.EditorDescriptionPlaceholder = tr(s, i18n.TrEditorBookDescPlaceholder)
		v.EditorChapterTitlePlaceholder = tr(s, i18n.TrEditorChapterTitlePlaceholder)
		v.EditorSettingsHint = tr(s, i18n.TrEditorBookSettingsHint)
	}

	v.IsHistoryPreview = !s.Config.StandaloneMode && s.Editor.History.IsPreview(b)
	if v.IsHistoryPreview {
		v.HistoryPreviewAttr = attrTrue
		if s.Editor.History.Preview != nil {
			v.HistoryPreviewID = s.Editor.History.Preview.RevisionID
			v.HistoryPreviewLabel = s.Editor.History.Preview.RevisionLabel
		}
	}

	if hasEditorBook && len(b.Scenes) > 0 {
		v.BookNarrated = !b.IsScreenplay() && b.Narrated
		if v.BookNarrated {
			v.BookNarratedLabel = tr(s, i18n.TrBookNarratedReqLabel)
			v.BookNarratedShortLabel = tr(s, i18n.TrRequiredLabel)
		} else {
			v.BookNarratedLabel = tr(s, i18n.TrBookNarratedOffLabel)
			v.BookNarratedShortLabel = tr(s, i18n.TrDisabledLabel)
		}
		v.BookImageless = b.Imageless
		if b.Imageless {
			v.BookImagelessLabel = tr(s, i18n.TrBookImagelessOnLabel)
			v.BookImagelessShortLabel = tr(s, i18n.TrNoImagesLabel)
		} else {
			v.BookImagelessLabel = tr(s, i18n.TrBookImagelessOffLabel)
			v.BookImagelessShortLabel = tr(s, i18n.TrWithImagesLabel)
		}
	} else {
		v.BookNarratedLabel = tr(s, i18n.TrBookNarratedOffLabel)
		v.BookNarratedShortLabel = tr(s, i18n.TrDisabledLabel)
		v.BookImagelessLabel = tr(s, i18n.TrBookImagelessOffLabel)
		v.BookImagelessShortLabel = tr(s, i18n.TrWithImagesLabel)
	}
}

func ViewEditorContextPanel(s *state.AppState) tpl.TemplateChunk {
	v := &editorContextPanelTemplate{}
	populateEditorCommonFields(s, v)
	if s.Editor.HasActiveBook() && len(s.Editor.ActiveBook.Scenes) > 0 {
		populateEditorSelectedNodeOnly(s, v)
	}
	return tpl.TemplateChunk{
		TemplateData: v,
		ScriptData: &editorContextPanelScript{
			selectedSceneId: v.SelectedSceneID,
			bookVarsSpec:    v.BookVarsSpec,
			isScreenplay:    v.IsScreenplayMode,
			scriptStandard:  v.ScriptStandard,
		},
	}
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

	if ch.IsEnding || strings.EqualFold(ch.Target, "END") {
		matchedOption = true
		selectedVal = "END"
	}
	if !matchedOption && ch.Target != "" {
		selectedVal = ch.Target
	}
	return selectedVal
}

func buildSceneMediaOptions(s *state.AppState, sc *book.Scene, buildMenus bool) (string, string, []book.SelectOption, string, string, []book.SelectOption) {
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
	var imgOptions []book.SelectOption
	if buildMenus && b != nil {
		imgOptions = make([]book.SelectOption, 0, len(b.ImageFiles)+2)
		imgOptions = append(imgOptions, book.SelectOption{
			Value:    "",
			Label:    noneLabel,
			IsActive: curImg == "",
		})
		matchedImg := curImg == ""
		for _, imgFile := range b.ImageFiles {
			isAct := strings.EqualFold(curImg, imgFile)
			if isAct {
				matchedImg = true
				curImgLabel = imgFile
			}
			imgOptions = append(imgOptions, book.SelectOption{
				Value:    imgFile,
				Label:    imgFile,
				IsActive: isAct,
			})
		}
		if !matchedImg && curImg != "" {
			curImgLabel = "⚠ " + curImg
			imgOptions = append(imgOptions, book.SelectOption{
				Value:    curImg,
				Label:    "⚠ " + curImg,
				IsActive: true,
			})
		}
		imgOptions = append(imgOptions, book.SelectOption{
			Value:    "__browse_disk__",
			Label:    tr(s, i18n.TrAddImageFromDiskOpt),
			IsActive: false,
		})
	}

	curVoice := ""
	curVoiceLabel := noneLabel
	var voiceOptions []book.SelectOption
	if b != nil && b.Narrated {
		curVoice = sc.ExplicitVoice
		if curVoice == "" && sc.AudioFile != "" {
			curVoice = sc.AudioFile
		}
		if curVoice != "" {
			curVoiceLabel = curVoice
		}
		if buildMenus {
			voiceOptions = make([]book.SelectOption, 0, len(b.AudioFiles)+3)
			voiceOptions = append(voiceOptions, book.SelectOption{
				Value:    "",
				Label:    noneLabel,
				IsActive: curVoice == "",
			})
			matchedVoice := curVoice == ""
			for _, audioFile := range b.AudioFiles {
				isAct := strings.EqualFold(curVoice, audioFile)
				if isAct {
					matchedVoice = true
					curVoiceLabel = audioFile
				}
				voiceOptions = append(voiceOptions, book.SelectOption{
					Value:    audioFile,
					Label:    audioFile,
					IsActive: isAct,
				})
			}
			if !matchedVoice && curVoice != "" {
				curVoiceLabel = "⚠ " + curVoice
				voiceOptions = append(voiceOptions, book.SelectOption{
					Value:    curVoice,
					Label:    "⚠ " + curVoice,
					IsActive: true,
				})
			}
			voiceOptions = append(voiceOptions, book.SelectOption{
				Value:    "__browse_disk__",
				Label:    tr(s, i18n.TrAddAudioFromDiskOpt),
				IsActive: false,
			})
		}
	}
	return curImg, curImgLabel, imgOptions, curVoice, curVoiceLabel, voiceOptions
}

func buildSceneVoiceSectionsView(s *state.AppState, sc *book.Scene) (hasMultiple bool, activeSec book.VoiceSection, sectionItems []book.VoiceSection) {
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

	sectionItems = make([]book.VoiceSection, len(sections))
	for i, sec := range sections {
		isActive := i == activeIdx
		inspLabel := sec.ShortLabel
		if isActive {
			inspLabel = sec.Label
		}
		secItem := sec
		secItem.InspectorLabel = inspLabel
		secItem.IsActive = isActive
		sectionItems[i] = secItem
	}
	return hasMultiple, activeSec, sectionItems
}

func buildEditorNodeView(s *state.AppState, sc *book.Scene, isSelected bool) *book.EditorNodeView {
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
	inspectorPorts := make([]book.Choice, 0, len(inspectorChoices))
	for chIdx, ch := range inspectorChoices {
		lbl := ch.Label
		if lbl == "" && !hasChoices {
			lbl = tr(s, i18n.TrNext)
		}
		selectedVal := "END"
		if isSelected {
			selectedVal = buildChoiceTargetOptions(s, sc, ch)
		}
		portView, _ := buildChoiceRuleView(s, sc, chIdx, ch, selectedVal)
		portView.RawLabel = lbl
		inspectorPorts = append(inspectorPorts, portView)
	}

	curImg, curImgLabel, imgOptions, curVoice, curVoiceLabel, voiceOptions := buildSceneMediaOptions(s, sc, isSelected)

	nodeWarns := append([]string(nil), sc.Warnings...)

	curChName := strings.TrimSpace(sc.Chapter)
	if curChName == "" && b != nil {
		curChName = b.DefaultChapterName()
	}
	var chapterOptions []book.SelectOption
	if isSelected && b != nil {
		chapterOptions = make([]book.SelectOption, 0, len(b.Chapters)+1)
		matchedCh := false
		for _, chItem := range b.Chapters {
			isAct := strings.EqualFold(curChName, strings.TrimSpace(chItem.Name))
			if isAct {
				matchedCh = true
			}
			chapterOptions = append(chapterOptions, book.SelectOption{
				Value:    chItem.Name,
				Label:    chItem.Name,
				IsActive: isAct,
			})
		}
		if !matchedCh && curChName != "" {
			chapterOptions = append(chapterOptions, book.SelectOption{
				Value:    curChName,
				Label:    curChName,
				IsActive: true,
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
	var activeVoiceSecFile, activeVoiceSecText, activeVoiceSecHeader string
	var voiceSectionItems []book.VoiceSection

	if isSelected {
		hasMultiSec, activeSec, secItems := buildSceneVoiceSectionsView(s, sc)
		hasVoiceSections = hasMultiSec
		activeVoiceSecKey = activeSec.Key
		activeVoiceSecFile = activeSec.TargetFile
		activeVoiceSecText = activeSec.Text
		activeVoiceSecHeader = activeSec.HeaderTitle
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
				voiceOptions[i].IsActive = strings.EqualFold(voiceOptions[i].Value, curVoice)
			}
		}
	}

	portCount := 0
	if hasChoices {
		portCount = len(inspectorChoices)
	}

	return &book.EditorNodeView{
		ID:                       sc.ID,
		ExplicitID:               sc.ExplicitID,
		CurrentImage:             curImg,
		CurrentImageLabel:        curImgLabel,
		ImageOptions:             imgOptions,
		CurrentVoice:             curVoice,
		CurrentVoiceLabel:        curVoiceLabel,
		CurrentVoiceURL:          voiceURL,
		VoiceDurationLabel:       voiceDurLabel,
		VoiceTrimStart:           fmt.Sprintf("%.4f", trimStart),
		VoiceTrimEnd:             fmt.Sprintf("%.4f", trimEnd),
		HasVoiceFile:             hasVoiceFile,
		HasVoiceTrim:             hasVoiceTrim,
		VoiceOptions:             voiceOptions,
		HasVoiceSections:         hasVoiceSections,
		ActiveVoiceSectionKey:    activeVoiceSecKey,
		ActiveVoiceSectionFile:   activeVoiceSecFile,
		ActiveVoiceSectionText:   activeVoiceSecText,
		ActiveVoiceSectionHeader: activeVoiceSecHeader,
		VoiceSections:            voiceSectionItems,
		Name:                     sc.Name,
		Slugline:                 sc.Slugline,
		Synopsis:                 sc.Synopsis,
		Effects:                  scEffectsVal,
		HasEffects:               scEffectsVal != "",
		IsEffectsInvalid:         scEffectsErr != "",
		EffectsErrorHint:         scEffectsErr,
		ScriptLengthLabel:        scriptLengthLabel,
		Chapter:                  curChName,
		ChapterOptions:           chapterOptions,
		SourceFile:               sc.SourceFile,
		IsStartScene:             isStart,
		IsChapterEntry:           isChapterEntry,
		HasNodeWarnings:          len(sc.Warnings) > 0,
		NodeWarnings:             nodeWarns,
		InspectorPorts:           inspectorPorts,
		PortCount:                portCount,
	}
}

func populateEditorBookAndChapterSettings(s *state.AppState, v *editorContextPanelTemplate) {
	b := s.Editor.ActiveBook
	if b == nil {
		return
	}

	normStd := b.NormalizedStandard()
	_, stdOpts := buildScriptStandardOptions(normStd)
	prefixPills, timePills := buildSluglineHelperPills(normStd)

	v.IsScreenplayMode = b.IsScreenplay()
	v.ScriptStandard = normStd
	v.VoiceRecorderOpen = s.Modals.VoiceRecorder
	v.ScriptStandardOptions = stdOpts
	v.SluglinePrefixPills = prefixPills
	v.SluglineTimePills = timePills

	v.MultiSceneItems = make([]book.SelectOption, 0, len(b.Scenes))
	for _, sc := range b.Scenes {
		scCh := strings.TrimSpace(sc.Chapter)
		if scCh == "" {
			scCh = b.DefaultChapterName()
		}
		v.MultiSceneItems = append(v.MultiSceneItems, book.SelectOption{
			Value: sc.ID,
			Label: sc.DisplayName(),
			Meta:  scCh,
		})
	}

	bookSettingsOpen := s.Editor.RightPanel == state.PanelBook
	v.BookSettingsOpen = bookSettingsOpen
	v.Book = b

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

	v.BookStartChapterOptions = make([]book.SelectOption, 0, len(b.Chapters))
	for _, ch := range b.Chapters {
		chName := strings.TrimSpace(ch.Name)
		if chName == "" {
			continue
		}
		v.BookStartChapterOptions = append(v.BookStartChapterOptions, book.SelectOption{
			Value:    chName,
			Label:    ch.DisplayName(),
			IsActive: strings.EqualFold(chName, startChapterName),
		})
	}
	v.BookStartChapterName = startChapterName

	startSceneLabel := ""
	v.BookStartSceneOptions = make([]book.SelectOption, 0, len(b.Scenes))
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
		v.BookStartSceneOptions = append(v.BookStartSceneOptions, book.SelectOption{
			Value:    sc.ID,
			Label:    optLabel,
			IsActive: isStart,
		})
	}
	if startSceneLabel == "" && len(v.BookStartSceneOptions) > 0 {
		startSceneLabel = v.BookStartSceneOptions[0].Label
	}
	v.BookStartSceneID = startSceneID
	v.BookStartSceneLabel = startSceneLabel

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
		chStartOptions := make([]book.SelectOption, 0, len(chScenes))
		for _, sc := range chScenes {
			isChStart := strings.EqualFold(sc.ID, chStartID)
			optLabel := sc.DisplayName()
			if isChStart {
				chStartLabel = optLabel
			}
			chStartOptions = append(chStartOptions, book.SelectOption{
				Value:    sc.ID,
				Label:    optLabel,
				IsActive: isChStart,
			})
		}
		if chStartLabel == "" && len(chStartOptions) > 0 {
			chStartLabel = chStartOptions[0].Label
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
		v.SelectedChapter = &book.Chapter{
			Name:              chName,
			StartSceneID:      chStartID,
			StartSceneLabel:   chStartLabel,
			StartSceneOptions: chStartOptions,
			SceneCount:        len(chScenes),
			LengthStat:        lengthStat,
			BranchCount:       branchCount,
			ExitCount:         exitCount,
		}
	}
	v.HasSelectedChapter = hasSelectedChapter
	v.IsNodeInspectorHidden = true
	v.IsContextPanelHidden = !bookSettingsOpen && !hasSelectedChapter
	v.ShowEditorHelpPanel = s.Editor.RightPanel == state.PanelHelp
	v.ShowSyntaxHelpPopover = s.Editor.SceneText.SyntaxHelpOpen

	var selScene *book.Scene
	if b != nil {
		selScene = b.FindScene(s.Editor.SelectedSceneID)
	}
	v.HasBookVariables = b != nil && len(b.Variables) > 0
	v.BookVarsSpec = book.EncodeBookVariablesSpec(b)
	v.BookVarSuggestions = buildBookVarSuggestions(b)
	v.BookEnumSuggestions = buildBookEnumValueSuggestions(b)
	v.BookTargetSuggestions = buildBookTargetSuggestions(s, selScene)
	v.SyntaxHelpCondRows = []book.SelectOption{
		{Value: "if: odkryta_prawda && sumienie >= 2", Label: "bool / int / enum (&&, ||, !, ==, !=, >=, <=)"},
		{Value: "set: odkryta_prawda = true; sumienie++", Label: tr(s, i18n.TrSyntaxHelpSetDesc)},
		{Value: "exit: ogien_w_kaplicy", Label: tr(s, i18n.TrSyntaxHelpSetDesc)},
	}
}

func buildChoiceRuleView(s *state.AppState, sc *book.Scene, idx int, ch book.Choice, selectedVal string) (book.Choice, bool) {
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
	out := ch
	out.Idx = idx
	out.SceneID = sceneID
	out.SelectedValue = selectedVal
	out.Condition = condInputVal
	out.Effects = effInputVal
	out.HasCondition = hasCond
	out.HasEffects = hasEff
	out.HasRule = hasRule
	out.IsRuleOpen = isRuleOpen
	out.IsExitInvalid = exitErrHint != ""
	out.ExitErrorHint = exitErrHint
	out.IsCondInvalid = condErrHint != ""
	out.IsEffInvalid = effErrHint != ""
	out.CondErrorHint = condErrHint
	out.EffErrorHint = effErrHint
	return out, hasCond
}

func populateEditorSelectedNodeOnly(s *state.AppState, v *editorContextPanelTemplate) {
	b := s.Editor.ActiveBook
	selected := b.FindScene(s.Editor.SelectedSceneID)
	if selected == nil {
		selected = b.Scenes[0]
		s.Editor.SelectedSceneID = selected.ID
	}
	v.SelectedSceneID = selected.ID
	v.SelectedSceneTitle = selected.DisplayName()
	v.SelectedNode = buildEditorNodeView(s, selected, true)
	v.HasSelectedNode = v.SelectedNode != nil
	populateEditorBookAndChapterSettings(s, v)
}
