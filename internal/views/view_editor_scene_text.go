//go:build !(js && wasm)

package views

import (
	"fmt"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

type screenplayInsertPillItem struct {
	Kind  string
	Label string
	Hint  string
}

type screenplayCharPillItem struct {
	Name  string
	Label string
	Hint  string
}

type incomingSceneChoiceItem struct {
	SceneID        string
	SceneName      string
	ChoiceLabel    string
	HasChoiceLabel bool
}

type sceneTestVarItem struct {
	Name        string
	Kind        string
	IsBool      bool
	IsInt       bool
	IsEnum      bool
	BoolVal     bool
	Value       string
	EnumOptions []map[string]any
}

type editorSceneTextView struct {
	ShowSceneTextModal                   bool
	SceneTextModalID                     string
	SceneTextModalName                   string
	SceneTextModalRawName                string
	SceneTextModalExplicitID             string
	SceneTextModalSlugline               string
	SceneTextModalSynopsis               string
	SceneTextModalText                   string
	SceneTextModalLengthLabel            string
	SluglinePrefixPills                  []map[string]any
	SluglineTimePills                    []map[string]any
	ScreenplayInsertPills                []screenplayInsertPillItem
	ScreenplayCharacterPills             []screenplayCharPillItem
	HasScreenplayCharacterPills          bool
	SceneTextModalIsStart                bool
	SceneTextModalChapter                string
	SceneTextModalChapterKey             string
	SceneTextModalChapterOptions         []map[string]any
	SceneTextModalHasVoiceSections       bool
	SceneTextModalVoiceSections          []map[string]any
	SceneTextModalActiveVoiceSectionKey  string
	SceneTextModalActiveVoiceSectionFile string
	IsVoiceSectionIsolated               bool
	IsolatedVoiceSectionKey              string
	IsolatedVoiceSectionHeader           string
	SceneTextModalCurrentImage           string
	SceneTextModalCurrentImageLabel      string
	SceneTextModalImageOptions           []map[string]any
	SceneTextModalBookNarrated           bool
	SceneTextModalCurrentVoice           string
	SceneTextModalCurrentVoiceLabel      string
	SceneTextModalVoiceURL               string
	SceneTextModalVoiceDurationLabel     string
	SceneTextModalVoiceTrimStart         string
	SceneTextModalVoiceTrimEnd           string
	SceneTextModalHasVoice               bool
	SceneTextModalHasVoiceTrim           bool
	SceneTextModalVoiceOptions           []map[string]any
	VoiceRecorderOpen                    bool
	HasBookVariables                     bool
	SceneTextHasRightPanel               bool
	SceneTextIsFullWidth                 bool
	SceneTextIsHalfWidth                 bool
	SceneTextModalSettingsOpen           bool
	SceneTextModalSettingsOpenVal        string
	SceneTextModalEffects                string
	SceneTextModalHasEffects             bool
	SceneTextModalEffectsInvalid         bool
	SceneTextModalEffectsErrorHint       string
	BookVarsSpec                         string
	BookVarSuggestions                   []map[string]any
	BookEnumSuggestions                  []map[string]any
	BookTargetSuggestions                []map[string]any
	ShowSyntaxHelpPopover                bool
	SyntaxHelpCondRows                   []map[string]any
	IsScenePreviewNormal                 bool
	IsScenePreviewHighlight              bool
	IsScenePreviewOutput                 bool
	SceneTestVars                        []sceneTestVarItem
	HasSceneTestVars                     bool
	ShowSceneTestVarsBar                 bool
	SceneTestVarsSpec                    string
	SceneTextModalHlSegments             []map[string]any
	SceneTextModalPreviewParagraphs      []map[string]any
	SceneTextModalOutputProse            string
	SceneTextModalHeaderMeta             string
	SceneTextModalHasImage               bool
	SceneTextModalImageURL               string
	SceneTextModalIsImageless            bool
	SceneTextModalIncoming               []incomingSceneChoiceItem
	SceneTextModalHasIncoming            bool
	SceneTextModalChoices                []map[string]any
	IsHistoryPreview                     bool
	IsScreenplayMode                     bool
	ScriptStandard                       string
}

func buildScreenplayFormatInsertPills(standard string, b *book.Book, lang i18n.Lang) ([]screenplayInsertPillItem, []screenplayCharPillItem) {
	isPL := book.NormalizeScriptStandard(standard) == "pl"
	pick := func(plVal, intlVal string) string {
		if isPL {
			return plVal
		}
		return intlVal
	}
	trK := func(k i18n.TrKey) string {
		return i18n.T(lang, k)
	}
	pickTr := func(plStdKey, intlStdKey i18n.TrKey) string {
		if isPL {
			return trK(plStdKey)
		}
		return trK(intlStdKey)
	}
	insertPills := []screenplayInsertPillItem{
		{Kind: "dialogue", Label: trK(i18n.TrSpPillDialogueLabel), Hint: trK(i18n.TrSpPillDialogueHint)},
		{Kind: "parenthetical", Label: pickTr(i18n.TrSpPillParenLabelPLStd, i18n.TrSpPillParenLabelIntlStd), Hint: pickTr(i18n.TrSpPillParenHintPLStd, i18n.TrSpPillParenHintIntlStd)},
		{Kind: "action", Label: trK(i18n.TrSpPillActionLabel), Hint: pickTr(i18n.TrSpPillActionHintPLStd, i18n.TrSpPillActionHintIntlStd)},
		{Kind: "dialogue_off", Label: pickTr(i18n.TrSpPillOffLabelPLStd, i18n.TrSpPillOffLabelIntlStd), Hint: pickTr(i18n.TrSpPillOffHintPLStd, i18n.TrSpPillOffHintIntlStd)},
		{Kind: "dialogue_vo", Label: trK(i18n.TrSpPillVOLabel), Hint: pickTr(i18n.TrSpPillVOHintPLStd, i18n.TrSpPillVOHintIntlStd)},
		{Kind: "transition_cut", Label: pick("+ CIĘCIE DO:", "+ CUT TO:"), Hint: pickTr(i18n.TrSpPillCutHintPLStd, i18n.TrSpPillCutHintIntlStd)},
		{Kind: "transition_fade", Label: pick("+ ŚCIEMNIENIE.", "+ FADE OUT."), Hint: pickTr(i18n.TrSpPillFadeHintPLStd, i18n.TrSpPillFadeHintIntlStd)},
		{Kind: "subslug", Label: pick("+ Podscena (WN./PL.)", trK(i18n.TrSpPillSubslugLabelIntlStd)), Hint: pickTr(i18n.TrSpPillSubslugHintPLStd, i18n.TrSpPillSubslugHintIntlStd)},
	}

	var charPills []screenplayCharPillItem
	if b != nil {
		bd := b.ScreenplayBreakdown()
		limit := len(bd.Characters)
		if limit > 8 {
			limit = 8
		}
		charPills = make([]screenplayCharPillItem, 0, limit)
		for i := 0; i < limit; i++ {
			chName := bd.Characters[i].Name
			charPills = append(charPills, screenplayCharPillItem{
				Name:  chName,
				Label: "@ " + chName,
				Hint:  i18n.Tf(lang, i18n.TrSpPillCharDialogueHintFmt, chName),
			})
		}
	}
	return insertPills, charPills
}

func ViewEditorSceneText(s *state.AppState) *editorSceneTextView {
	if !s.Editor.SceneText.IsOpen() {
		return nil
	}
	b := s.Editor.ActiveBook
	modalSc := b.FindScene(s.Editor.SceneText.SceneID)
	if modalSc == nil {
		s.Editor.SceneText.Close()
		return nil
	}

	lang := LangOf(s)
	v := &editorSceneTextView{
		ShowSceneTextModal:       true,
		SceneTextModalID:         modalSc.ID,
		SceneTextModalName:       modalSc.DisplayName(),
		SceneTextModalRawName:    modalSc.Name,
		SceneTextModalExplicitID: modalSc.ExplicitID,
		SceneTextModalSlugline:   modalSc.Slugline,
		SceneTextModalSynopsis:   modalSc.Synopsis,
		SceneTextModalText:       modalSc.Text,
		IsHistoryPreview:         !s.Config.StandaloneMode && s.Editor.History.IsPreview(b),
		ScriptStandard:           "pl",
	}

	if b != nil {
		v.IsScreenplayMode = b.IsScreenplay()
		v.ScriptStandard = b.NormalizedStandard()
	}

	if v.IsScreenplayMode {
		normStd := v.ScriptStandard
		metrics := book.ComputeSceneScriptMetrics(modalSc, modalSc.OrderIndex+1, normStd)
		v.SceneTextModalLengthLabel = metrics.FormattedLength
		v.SluglinePrefixPills, v.SluglineTimePills = buildSluglineHelperPills(normStd)
		v.ScreenplayInsertPills, v.ScreenplayCharacterPills = buildScreenplayFormatInsertPills(normStd, b, lang)
		v.HasScreenplayCharacterPills = len(v.ScreenplayCharacterPills) > 0
	}

	if b != nil {
		if b.StartSceneID != "" {
			v.SceneTextModalIsStart = strings.EqualFold(modalSc.ID, b.StartSceneID)
		} else if len(b.Scenes) > 0 {
			v.SceneTextModalIsStart = modalSc.ID == b.Scenes[0].ID
		}
	}

	curChName := strings.TrimSpace(modalSc.Chapter)
	if curChName == "" && b != nil {
		curChName = b.DefaultChapterName()
	}
	chKey := tr(s, i18n.TrChapterKey)
	if b != nil && b.IsScreenplay() {
		chKey = tr(s, i18n.TrActKey)
	}
	var modalChOptions []map[string]any
	if b != nil {
		modalChOptions = make([]map[string]any, 0, len(b.Chapters)+1)
		matchedCh := false
		for _, chItem := range b.Chapters {
			isAct := strings.EqualFold(curChName, strings.TrimSpace(chItem.Name))
			if isAct {
				matchedCh = true
			}
			modalChOptions = append(modalChOptions, map[string]any{
				"value":     chItem.Name,
				"label":     chItem.Name,
				"is_active": isAct,
			})
		}
		if !matchedCh && curChName != "" {
			modalChOptions = append(modalChOptions, map[string]any{
				"value":     curChName,
				"label":     curChName,
				"is_active": true,
			})
		}
	}
	v.SceneTextModalChapter = curChName
	v.SceneTextModalChapterKey = chKey
	v.SceneTextModalChapterOptions = modalChOptions

	media := buildSceneMediaOptions(s, modalSc, true)
	curImg := media.curImg
	curImgLabel := media.curImgLabel
	imgOptions := media.imgOptions
	curVoice := media.curVoice
	curVoiceLabel := media.curVoiceLabel
	voiceOptions := media.voiceOptions
	modalVoiceURL := modalSc.AudioURL
	modalVoiceDur := "0:00"
	if curVoice != "" && b != nil {
		durLbl, stamp := book.ReadSceneAudioDurationAndStamp(b, modalSc)
		modalVoiceDur = durLbl
		if modalVoiceURL != "" && stamp > 0 {
			modalVoiceURL = fmt.Sprintf("%s?v=%d", modalVoiceURL, stamp)
		}
	}
	modalTrimStart, modalTrimEnd := modalSc.NormalizedVoiceTrim()
	hasModalVoice := curVoice != "" && modalSc.AudioURL != ""
	hasModalVoiceTrim := hasModalVoice && modalSc.HasVoiceTrim()
	bookNarrated := b != nil && !b.IsScreenplay() && b.Narrated

	hasModalVoiceSections, activeModalSec, modalVoiceSectionItems := buildSceneVoiceSectionsView(s, modalSc)
	if hasModalVoiceSections {
		curVoice = activeModalSec.AudioFile
		if curVoice == "" {
			curVoice = activeModalSec.ExplicitVoice
		}
		if curVoice != "" {
			curVoiceLabel = curVoice
		} else if s.Modals.VoiceRecorder && activeModalSec.TargetFile != "" {
			curVoiceLabel = activeModalSec.TargetFile
		} else {
			curVoiceLabel = tr(s, i18n.TrMediaNoneLabel)
		}
		modalVoiceURL = activeModalSec.AudioURL
		if activeModalSec.HasAudio {
			modalVoiceDur = activeModalSec.DurationLabel
		} else {
			modalVoiceDur = "0:00"
		}
		modalTrimStart, modalTrimEnd = activeModalSec.TrimStart, activeModalSec.TrimEnd
		hasModalVoice = activeModalSec.HasAudio
		hasModalVoiceTrim = activeModalSec.HasTrim
		for i := range voiceOptions {
			val, _ := voiceOptions[i]["value"].(string)
			voiceOptions[i]["is_active"] = strings.EqualFold(val, curVoice)
		}
	}

	isVoiceSectionIsolated := bookNarrated && s.Modals.VoiceRecorder && hasModalVoiceSections
	displayedModalText := modalSc.Text
	if isVoiceSectionIsolated {
		displayedModalText = activeModalSec.Text
	}
	v.SceneTextModalText = displayedModalText
	v.SceneTextModalHasVoiceSections = hasModalVoiceSections
	v.SceneTextModalVoiceSections = modalVoiceSectionItems
	v.SceneTextModalActiveVoiceSectionKey = activeModalSec.Key
	v.SceneTextModalActiveVoiceSectionFile = activeModalSec.TargetFile
	v.IsVoiceSectionIsolated = isVoiceSectionIsolated
	v.IsolatedVoiceSectionKey = activeModalSec.Key
	v.IsolatedVoiceSectionHeader = activeModalSec.HeaderTitle

	v.SceneTextModalCurrentImage = curImg
	v.SceneTextModalCurrentImageLabel = curImgLabel
	v.SceneTextModalImageOptions = imgOptions
	v.SceneTextModalBookNarrated = bookNarrated
	v.SceneTextModalCurrentVoice = curVoice
	v.SceneTextModalCurrentVoiceLabel = curVoiceLabel
	v.SceneTextModalVoiceURL = modalVoiceURL
	v.SceneTextModalVoiceDurationLabel = modalVoiceDur
	v.SceneTextModalVoiceTrimStart = fmt.Sprintf("%.4f", modalTrimStart)
	v.SceneTextModalVoiceTrimEnd = fmt.Sprintf("%.4f", modalTrimEnd)
	v.SceneTextModalHasVoice = hasModalVoice
	v.SceneTextModalHasVoiceTrim = hasModalVoiceTrim
	v.SceneTextModalVoiceOptions = voiceOptions
	v.VoiceRecorderOpen = s.Modals.VoiceRecorder

	settingsOpenVal := "false"
	if s.Editor.SceneText.SettingsOpen {
		settingsOpenVal = "true"
	}
	hasBookVars := b != nil && len(b.Variables) > 0
	v.HasBookVariables = hasBookVars
	v.SceneTextHasRightPanel = s.Editor.RightPanel != state.PanelNone
	isFullWidth := s.Editor.SceneText.WidthMode == state.WidthModeFull
	v.SceneTextIsFullWidth = isFullWidth
	v.SceneTextIsHalfWidth = !isFullWidth
	v.SceneTextModalSettingsOpen = s.Editor.SceneText.SettingsOpen
	v.SceneTextModalSettingsOpenVal = settingsOpenVal

	effModalStr := book.CleanEffectsExpr(modalSc.Effects)
	effModalErr := ""
	if effModalStr != "" {
		effModalErr = book.ExplainEffectsExprErrorLang(modalSc.Effects, b, s.Settings.Language)
	}
	v.SceneTextModalEffects = effModalStr
	v.SceneTextModalHasEffects = effModalStr != ""
	v.SceneTextModalEffectsInvalid = effModalErr != ""
	v.SceneTextModalEffectsErrorHint = effModalErr
	v.BookVarsSpec = book.EncodeBookVariablesSpec(b)
	v.BookVarSuggestions = buildBookVarSuggestions(b)
	v.BookEnumSuggestions = buildBookEnumValueSuggestions(b)
	v.BookTargetSuggestions = buildBookTargetSuggestions(s, modalSc)
	v.ShowSyntaxHelpPopover = hasBookVars && s.Editor.SceneText.SyntaxHelpOpen
	v.SyntaxHelpCondRows = []map[string]any{
		{"code": "if: odkryta_prawda && sumienie >= 2", "desc": "bool / int / enum (&&, ||, !, ==, !=, >=, <=)"},
		{"code": "set: odkryta_prawda = true; sumienie++", "desc": tr(s, i18n.TrSyntaxHelpSetDesc)},
		{"code": "exit: ogien_w_kaplicy", "desc": tr(s, i18n.TrSyntaxHelpExitDesc)},
	}

	previewMode := s.Editor.SceneText.PreviewMode
	if !hasBookVars {
		previewMode = state.PreviewModeNormal
	}
	previewModeStr := previewMode.String()
	v.IsScenePreviewNormal = previewMode == state.PreviewModeNormal
	v.IsScenePreviewHighlight = previewMode == state.PreviewModeHighlight
	v.IsScenePreviewOutput = previewMode == state.PreviewModeOutput

	testState := book.EnsureSaveVariables(b, s.Editor.SceneText.TestVars)
	s.Editor.SceneText.TestVars = testState
	var testVarItems []sceneTestVarItem
	if b != nil {
		book.ResolveBookVariableTypes(b)
		testVarItems = make([]sceneTestVarItem, 0, len(b.Variables))
		for _, vd := range b.Variables {
			curVal := testState[vd.Name]
			isTrue := curVal == "true"
			var enumOpts []map[string]any
			if vd.Kind == book.VarKindEnum {
				enumOpts = make([]map[string]any, 0, len(vd.Options))
				for _, opt := range vd.Options {
					enumOpts = append(enumOpts, map[string]any{
						"value":     opt,
						"label":     opt,
						"is_active": strings.EqualFold(opt, curVal),
					})
				}
			}
			testVarItems = append(testVarItems, sceneTestVarItem{
				Name:        vd.Name,
				Kind:        vd.Kind,
				IsBool:      vd.Kind == book.VarKindBool,
				IsInt:       vd.Kind == book.VarKindInt,
				IsEnum:      vd.Kind == book.VarKindEnum,
				BoolVal:     isTrue,
				Value:       curVal,
				EnumOptions: enumOpts,
			})
		}
	}
	v.SceneTestVars = testVarItems
	v.HasSceneTestVars = len(testVarItems) > 0
	v.ShowSceneTestVarsBar = s.Editor.SceneText.TestVarsOpen || (previewMode == state.PreviewModeHighlight && len(testVarItems) > 0)
	v.SceneTestVarsSpec = book.EncodeVariablesStateSpec(testState, b)
	v.SceneTextModalHlSegments = book.BuildSceneProseHighlightSegments(displayedModalText, testState, b)
	v.SceneTextModalPreviewParagraphs = book.BuildSceneProsePreviewParagraphs(modalSc.Text, previewModeStr, testState, b)
	v.SceneTextModalOutputProse = book.EvaluateSceneProseText(modalSc, testState, b)

	modalHeaderMeta := ""
	if b != nil {
		modalHeaderMeta = b.Title
	}
	if modalSc.Chapter != "" {
		if modalHeaderMeta != "" {
			modalHeaderMeta += " • " + modalSc.Chapter
		} else {
			modalHeaderMeta = modalSc.Chapter
		}
	}
	v.SceneTextModalHeaderMeta = modalHeaderMeta

	imageless := b.IsEffectiveImageless(s.Settings)
	modalImgURL := ""
	if !imageless {
		modalImgURL = modalSc.ImageURL
		if modalImgURL == "" {
			modalImgURL = resolveInheritedSceneImageURL(s, modalSc)
		}
	}
	v.SceneTextModalHasImage = modalImgURL != ""
	v.SceneTextModalImageURL = modalImgURL
	v.SceneTextModalIsImageless = imageless || modalImgURL == ""

	var incomingScenes []incomingSceneChoiceItem
	if b != nil {
		seenIncoming := make(map[string]bool)
		for _, sc := range b.Scenes {
			if sc == nil || sc.ID == modalSc.ID {
				continue
			}
			scHasMulti := len(sc.Choices) > 1
			for _, ch := range sc.Choices {
				if ch.IsEnding || !strings.EqualFold(ch.TargetSceneID, modalSc.ID) {
					continue
				}
				choiceLbl := strings.TrimSpace(ch.Label)
				if !scHasMulti && (ch.IsImplicit || strings.EqualFold(choiceLbl, "Dalej")) {
					choiceLbl = ""
				}
				dedupKey := sc.ID + "\x00" + choiceLbl
				if seenIncoming[dedupKey] {
					continue
				}
				seenIncoming[dedupKey] = true
				incomingScenes = append(incomingScenes, incomingSceneChoiceItem{
					SceneID:        sc.ID,
					SceneName:      sc.DisplayName(),
					ChoiceLabel:    choiceLbl,
					HasChoiceLabel: choiceLbl != "",
				})
			}
		}
	}
	v.SceneTextModalIncoming = incomingScenes
	v.SceneTextModalHasIncoming = len(incomingScenes) > 0

	startID := ""
	if b != nil {
		startID = b.StartSceneID
		if startID == "" && len(b.Scenes) > 0 {
			startID = b.Scenes[0].ID
		}
	}
	isTerminal := len(modalSc.Choices) == 0 || modalSc.ExplicitEnd

	choicesSource := modalSc.Choices
	if isTerminal {
		choicesSource = []book.Choice{
			{
				Label:      tr(s, i18n.TrNext),
				Target:     "END",
				IsEnding:   true,
				IsImplicit: true,
			},
		}
	}
	hasMulti := len(choicesSource) > 1
	modalChoices := make([]map[string]any, 0, len(choicesSource))
	for idx, ch := range choicesSource {
		lbl := ch.Label
		if !hasMulti && lbl == "" {
			lbl = tr(s, i18n.TrNext)
		}
		arr := "»"
		if ch.IsEnding {
			arr = "↺"
		}
		selectedVal := buildChoiceTargetOptions(s, modalSc, ch)
		choiceMap, hasCond := buildChoiceRuleViewMap(s, modalSc, idx, ch, selectedVal)
		condMet := book.EvaluateCondition(ch.Condition, testState, b)
		if previewMode == state.PreviewModeOutput && hasCond && !condMet && !ch.ShowWhenLocked {
			continue
		}
		targetScID := ch.TargetSceneID
		if ch.IsEnding && targetScID == "" {
			targetScID = startID
		}
		choiceMap["label"] = lbl
		choiceMap["arrow"] = arr
		choiceMap["is_branch"] = !ch.IsEnding
		choiceMap["is_editable"] = true
		choiceMap["target_scene_id"] = targetScID
		choiceMap["is_cond_unmet"] = previewMode == state.PreviewModeHighlight && hasCond && !condMet
		choiceMap["is_locked_preview"] = previewMode == state.PreviewModeOutput && hasCond && !condMet && ch.ShowWhenLocked
		modalChoices = append(modalChoices, choiceMap)
	}
	v.SceneTextModalChoices = modalChoices
	return v
}

func resolveInheritedSceneImageURL(s *state.AppState, target *book.Scene) string {
	b := s.Editor.ActiveBook
	if target == nil || b == nil || len(b.Scenes) == 0 {
		return ""
	}
	if target.ImageURL != "" {
		return target.ImageURL
	}
	startSc := b.FindScene(b.StartSceneID)
	if startSc == nil {
		startSc = b.Scenes[0]
	}
	type bfsItem struct {
		sc  *book.Scene
		img string
	}
	visited := make(map[string]bool, len(b.Scenes))
	queue := []bfsItem{{sc: startSc, img: startSc.ImageURL}}
	visited[startSc.ID] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		curImg := cur.img
		if cur.sc.ImageURL != "" {
			curImg = cur.sc.ImageURL
		}
		if cur.sc.ID == target.ID {
			if curImg != "" {
				return curImg
			}
			break
		}
		for _, ch := range cur.sc.Choices {
			if ch.IsEnding || ch.TargetSceneID == "" || strings.EqualFold(ch.TargetSceneID, "END") {
				continue
			}
			nextSc := b.FindScene(ch.TargetSceneID)
			if nextSc != nil && !visited[nextSc.ID] {
				visited[nextSc.ID] = true
				nextImg := curImg
				if nextSc.ImageURL != "" {
					nextImg = nextSc.ImageURL
				}
				queue = append(queue, bfsItem{sc: nextSc, img: nextImg})
			}
		}
	}
	lastImg := ""
	for _, sc := range b.Scenes {
		if sc == nil {
			continue
		}
		if sc.ImageURL != "" {
			lastImg = sc.ImageURL
		}
		if sc.ID == target.ID && lastImg != "" {
			return lastImg
		}
	}
	return lastImg
}
