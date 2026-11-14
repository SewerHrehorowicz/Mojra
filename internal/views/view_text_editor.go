//go:build !(js && wasm)

package views

import (
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type textEditorTemplate struct {
	ShowSceneTextModal                   bool
	SceneTextModalID                     string
	SceneTextModalName                   string
	SceneTextModalRawName                string
	SceneTextModalExplicitID             string
	SceneTextModalSlugline               string
	SceneTextModalSynopsis               string
	SceneTextModalText                   string
	SceneTextModalLengthLabel            string
	SluglinePrefixPills                  []string
	SluglineTimePills                    []string
	ScreenplayInsertPills                []book.ScreenplayInsertPill
	ScreenplayCharacterPills             []book.ScreenplayInsertPill
	HasScreenplayCharacterPills          bool
	SceneTextModalIsStart                bool
	SceneTextModalChapter                string
	SceneTextModalChapterKey             string
	SceneTextModalChapterOptions         []book.SelectOption
	SceneTextModalHasVoiceSections       bool
	SceneTextModalVoiceSections          []book.VoiceSection
	SceneTextModalActiveVoiceSectionKey  string
	SceneTextModalActiveVoiceSectionFile string
	IsVoiceSectionIsolated               bool
	IsolatedVoiceSectionKey              string
	IsolatedVoiceSectionHeader           string
	SceneTextModalCurrentImage           string
	SceneTextModalCurrentImageLabel      string
	SceneTextModalImageOptions           []book.SelectOption
	SceneTextModalBookNarrated           bool
	SceneTextModalCurrentVoice           string
	SceneTextModalCurrentVoiceLabel      string
	SceneTextModalVoiceURL               string
	SceneTextModalVoiceDurationLabel     string
	SceneTextModalVoiceTrimStart         string
	SceneTextModalVoiceTrimEnd           string
	SceneTextModalHasVoice               bool
	SceneTextModalHasVoiceTrim           bool
	SceneTextModalVoiceOptions           []book.SelectOption
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
	BookVarSuggestions                   []book.VariableDef
	BookEnumSuggestions                  []book.SelectOption
	BookTargetSuggestions                []book.SelectOption
	ShowSyntaxHelpPopover                bool
	SyntaxHelpCondRows                   []book.SelectOption
	IsScenePreviewNormal                 bool
	IsScenePreviewHighlight              bool
	IsScenePreviewOutput                 bool
	SceneTestVars                        []book.VariableDef
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
	SceneTextModalIncoming               []book.SelectOption
	SceneTextModalHasIncoming            bool
	SceneTextModalChoices                []book.Choice
	IsHistoryPreview                     bool
	IsScreenplayMode                     bool
	ScriptStandard                       string
}

type textEditorScript struct {
	sceneId        string
	bookVarsSpec   string
	isScreenplay   bool
	scriptStandard string
}

func buildScreenplayFormatInsertPills(standard string, b *book.Book, lang i18n.Lang) ([]book.ScreenplayInsertPill, []book.ScreenplayInsertPill) {
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
	insertPills := []book.ScreenplayInsertPill{
		{Kind: "dialogue", Label: trK(i18n.TrSpPillDialogueLabel), Hint: trK(i18n.TrSpPillDialogueHint)},
		{Kind: "parenthetical", Label: pickTr(i18n.TrSpPillParenLabelPLStd, i18n.TrSpPillParenLabelIntlStd), Hint: pickTr(i18n.TrSpPillParenHintPLStd, i18n.TrSpPillParenHintIntlStd)},
		{Kind: "action", Label: trK(i18n.TrSpPillActionLabel), Hint: pickTr(i18n.TrSpPillActionHintPLStd, i18n.TrSpPillActionHintIntlStd)},
		{Kind: "dialogue_off", Label: pickTr(i18n.TrSpPillOffLabelPLStd, i18n.TrSpPillOffLabelIntlStd), Hint: pickTr(i18n.TrSpPillOffHintPLStd, i18n.TrSpPillOffHintIntlStd)},
		{Kind: "dialogue_vo", Label: trK(i18n.TrSpPillVOLabel), Hint: pickTr(i18n.TrSpPillVOHintPLStd, i18n.TrSpPillVOHintIntlStd)},
		{Kind: "transition_cut", Label: pick("+ CIĘCIE DO:", "+ CUT TO:"), Hint: pickTr(i18n.TrSpPillCutHintPLStd, i18n.TrSpPillCutHintIntlStd)},
		{Kind: "transition_fade", Label: pick("+ ŚCIEMNIENIE.", "+ FADE OUT."), Hint: pickTr(i18n.TrSpPillFadeHintPLStd, i18n.TrSpPillFadeHintIntlStd)},
		{Kind: "subslug", Label: pick("+ Podscena (WN./PL.)", trK(i18n.TrSpPillSubslugLabelIntlStd)), Hint: pickTr(i18n.TrSpPillSubslugHintPLStd, i18n.TrSpPillSubslugHintIntlStd)},
	}
	for i := range insertPills {
		insertPills[i].Snippet, insertPills[i].SelectTarget, insertPills[i].DoubleNewline = book.ScreenplayBlockSnippet(standard, insertPills[i].Kind, "", lang)
	}

	var charPills []book.ScreenplayInsertPill
	if b != nil {
		bd := b.ScreenplayBreakdown()
		limit := len(bd.Characters)
		if limit > 8 {
			limit = 8
		}
		charPills = make([]book.ScreenplayInsertPill, 0, limit)
		for i := 0; i < limit; i++ {
			chName := bd.Characters[i].Name
			snip, selTgt, dblNl := book.ScreenplayBlockSnippet(standard, "", chName, lang)
			charPills = append(charPills, book.ScreenplayInsertPill{
				Name:          chName,
				Label:         "@ " + chName,
				Hint:          i18n.Tf(lang, i18n.TrSpPillCharDialogueHintFmt, chName),
				Snippet:       snip,
				SelectTarget:  selTgt,
				DoubleNewline: dblNl,
			})
		}
	}
	return insertPills, charPills
}

func ViewTextEditor(s *state.AppState) tpl.TemplateChunk {
	if !s.Editor.SceneText.IsOpen() {
		return tpl.TemplateChunk{}
	}
	b := s.Editor.ActiveBook
	modalSc := b.FindScene(s.Editor.SceneText.SceneID)
	if modalSc == nil {
		s.Editor.SceneText.Close()
		return tpl.TemplateChunk{}
	}

	lang := LangOf(s)
	nv := buildEditorNodeView(s, modalSc, true)
	bookNarrated := b != nil && !b.IsScreenplay() && b.Narrated
	isVoiceSectionIsolated := bookNarrated && s.Modals.VoiceRecorder && nv.HasVoiceSections
	displayedModalText := modalSc.Text
	if isVoiceSectionIsolated {
		displayedModalText = nv.ActiveVoiceSectionText
	}
	chKey := tr(s, i18n.TrChapterKey)
	if b != nil && b.IsScreenplay() {
		chKey = tr(s, i18n.TrActKey)
	}
	settingsOpenVal := attrFalse
	if s.Editor.SceneText.SettingsOpen {
		settingsOpenVal = attrTrue
	}
	hasBookVars := b != nil && len(b.Variables) > 0
	isFullWidth := s.Editor.SceneText.WidthMode == state.WidthModeFull

	v := &textEditorTemplate{
		ShowSceneTextModal:                   true,
		SceneTextModalID:                     nv.ID,
		SceneTextModalName:                   modalSc.DisplayName(),
		SceneTextModalRawName:                nv.Name,
		SceneTextModalExplicitID:             nv.ExplicitID,
		SceneTextModalSlugline:               nv.Slugline,
		SceneTextModalSynopsis:               nv.Synopsis,
		SceneTextModalText:                   displayedModalText,
		SceneTextModalLengthLabel:            nv.ScriptLengthLabel,
		SceneTextModalIsStart:                nv.IsStartScene,
		SceneTextModalChapter:                nv.Chapter,
		SceneTextModalChapterKey:             chKey,
		SceneTextModalChapterOptions:         nv.ChapterOptions,
		SceneTextModalHasVoiceSections:       nv.HasVoiceSections,
		SceneTextModalVoiceSections:          nv.VoiceSections,
		SceneTextModalActiveVoiceSectionKey:  nv.ActiveVoiceSectionKey,
		SceneTextModalActiveVoiceSectionFile: nv.ActiveVoiceSectionFile,
		IsVoiceSectionIsolated:               isVoiceSectionIsolated,
		IsolatedVoiceSectionKey:              nv.ActiveVoiceSectionKey,
		IsolatedVoiceSectionHeader:           nv.ActiveVoiceSectionHeader,
		SceneTextModalCurrentImage:           nv.CurrentImage,
		SceneTextModalCurrentImageLabel:      nv.CurrentImageLabel,
		SceneTextModalImageOptions:           nv.ImageOptions,
		SceneTextModalBookNarrated:           bookNarrated,
		SceneTextModalCurrentVoice:           nv.CurrentVoice,
		SceneTextModalCurrentVoiceLabel:      nv.CurrentVoiceLabel,
		SceneTextModalVoiceURL:               nv.CurrentVoiceURL,
		SceneTextModalVoiceDurationLabel:     nv.VoiceDurationLabel,
		SceneTextModalVoiceTrimStart:         nv.VoiceTrimStart,
		SceneTextModalVoiceTrimEnd:           nv.VoiceTrimEnd,
		SceneTextModalHasVoice:               nv.HasVoiceFile,
		SceneTextModalHasVoiceTrim:           nv.HasVoiceTrim,
		SceneTextModalVoiceOptions:           nv.VoiceOptions,
		VoiceRecorderOpen:                    s.Modals.VoiceRecorder,
		HasBookVariables:                     hasBookVars,
		SceneTextHasRightPanel:               s.Editor.RightPanel != state.PanelNone,
		SceneTextIsFullWidth:                 isFullWidth,
		SceneTextIsHalfWidth:                 !isFullWidth,
		SceneTextModalSettingsOpen:           s.Editor.SceneText.SettingsOpen,
		SceneTextModalSettingsOpenVal:        settingsOpenVal,
		SceneTextModalEffects:                nv.Effects,
		SceneTextModalHasEffects:             nv.HasEffects,
		SceneTextModalEffectsInvalid:         nv.IsEffectsInvalid,
		SceneTextModalEffectsErrorHint:       nv.EffectsErrorHint,
		BookVarsSpec:                         book.EncodeBookVariablesSpec(b),
		BookVarSuggestions:                   buildBookVarSuggestions(b),
		BookEnumSuggestions:                  buildBookEnumValueSuggestions(b),
		BookTargetSuggestions:                buildBookTargetSuggestions(s, modalSc),
		ShowSyntaxHelpPopover:                hasBookVars && s.Editor.SceneText.SyntaxHelpOpen,
		SyntaxHelpCondRows: []book.SelectOption{
			{Value: "if: odkryta_prawda && sumienie >= 2", Label: "bool / int / enum (&&, ||, !, ==, !=, >=, <=)"},
			{Value: "set: odkryta_prawda = true; sumienie++", Label: tr(s, i18n.TrSyntaxHelpSetDesc)},
			{Value: "exit: ogien_w_kaplicy", Label: tr(s, i18n.TrSyntaxHelpExitDesc)},
		},
		IsHistoryPreview: !s.Config.StandaloneMode && s.Editor.History.IsPreview(b),
		ScriptStandard:   "pl",
	}

	if b != nil {
		v.IsScreenplayMode = b.IsScreenplay()
		v.ScriptStandard = b.NormalizedStandard()
	}

	if v.IsScreenplayMode {
		normStd := v.ScriptStandard
		v.SluglinePrefixPills, v.SluglineTimePills = buildSluglineHelperPills(normStd)
		v.ScreenplayInsertPills, v.ScreenplayCharacterPills = buildScreenplayFormatInsertPills(normStd, b, lang)
		v.HasScreenplayCharacterPills = len(v.ScreenplayCharacterPills) > 0
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
	var testVarItems []book.VariableDef
	if b != nil {
		book.ResolveBookVariableTypes(b)
		testVarItems = make([]book.VariableDef, 0, len(b.Variables))
		for _, vd := range b.Variables {
			curVal := testState[vd.Name]
			isTrue := curVal == "true"
			var enumOpts []book.SelectOption
			if vd.Kind == book.VarKindEnum {
				enumOpts = make([]book.SelectOption, 0, len(vd.Options))
				for _, opt := range vd.Options {
					enumOpts = append(enumOpts, book.SelectOption{
						Value:    opt,
						Label:    opt,
						IsActive: strings.EqualFold(opt, curVal),
					})
				}
			}
			testVarItems = append(testVarItems, book.VariableDef{
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

	var incomingScenes []book.SelectOption
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
				incomingScenes = append(incomingScenes, book.SelectOption{
					Value:    sc.ID,
					Label:    sc.DisplayName(),
					Meta:     choiceLbl,
					IsActive: choiceLbl != "",
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
	modalChoices := make([]book.Choice, 0, len(choicesSource))
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
		choiceView, hasCond := buildChoiceRuleView(s, modalSc, idx, ch, selectedVal)
		condMet := book.EvaluateCondition(ch.Condition, testState, b)
		if previewMode == state.PreviewModeOutput && hasCond && !condMet && !ch.ShowWhenLocked {
			continue
		}
		targetScID := ch.TargetSceneID
		if ch.IsEnding && targetScID == "" {
			targetScID = startID
		}
		choiceView.Label = lbl
		choiceView.Arrow = arr
		choiceView.IsBranch = !ch.IsEnding
		choiceView.IsEditable = true
		choiceView.TargetSceneID = targetScID
		choiceView.IsCondUnmet = previewMode == state.PreviewModeHighlight && hasCond && !condMet
		choiceView.IsLockedPreview = previewMode == state.PreviewModeOutput && hasCond && !condMet && ch.ShowWhenLocked
		modalChoices = append(modalChoices, choiceView)
	}
	v.SceneTextModalChoices = modalChoices
	return tpl.TemplateChunk{
		TemplateData: v,
		ScriptData: &textEditorScript{
			sceneId:        v.SceneTextModalID,
			bookVarsSpec:   v.BookVarsSpec,
			isScreenplay:   v.IsScreenplayMode,
			scriptStandard: v.ScriptStandard,
		},
	}
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
