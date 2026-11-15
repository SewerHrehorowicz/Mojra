//go:build !(js && wasm)

package views

import (
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine"
	"mojra/engine/templates"
)

const (
	charPillPrefix = "@ "
	arrowBranch    = "»"
	arrowRestart   = "↺"
)

type screenplayPillSpec struct {
	kind     string
	labelPL  engine.TrKey
	labelInt engine.TrKey
	hintPL   engine.TrKey
	hintInt  engine.TrKey
}

var screenplayInsertPillSpecs = [...]screenplayPillSpec{
	{book.ScreenplayInsertDialogue, i18n.TrSpPillDialogueLabel, i18n.TrSpPillDialogueLabel, i18n.TrSpPillDialogueHint, i18n.TrSpPillDialogueHint},
	{book.ScreenplayInsertParenthetical, i18n.TrSpPillParenLabelPLStd, i18n.TrSpPillParenLabelIntlStd, i18n.TrSpPillParenHintPLStd, i18n.TrSpPillParenHintIntlStd},
	{book.ScreenplayInsertAction, i18n.TrSpPillActionLabel, i18n.TrSpPillActionLabel, i18n.TrSpPillActionHintPLStd, i18n.TrSpPillActionHintIntlStd},
	{book.ScreenplayInsertDialogueOff, i18n.TrSpPillOffLabelPLStd, i18n.TrSpPillOffLabelIntlStd, i18n.TrSpPillOffHintPLStd, i18n.TrSpPillOffHintIntlStd},
	{book.ScreenplayInsertDialogueVO, i18n.TrSpPillVOLabel, i18n.TrSpPillVOLabel, i18n.TrSpPillVOHintPLStd, i18n.TrSpPillVOHintIntlStd},
	{book.ScreenplayInsertTransitionCut, i18n.TrSpPillCutLabelPLStd, i18n.TrSpPillCutLabelIntlStd, i18n.TrSpPillCutHintPLStd, i18n.TrSpPillCutHintIntlStd},
	{book.ScreenplayInsertTransitionFade, i18n.TrSpPillFadeLabelPLStd, i18n.TrSpPillFadeLabelIntlStd, i18n.TrSpPillFadeHintPLStd, i18n.TrSpPillFadeHintIntlStd},
	{book.ScreenplayInsertSubslug, i18n.TrSpPillSubslugLabelPLStd, i18n.TrSpPillSubslugLabelIntlStd, i18n.TrSpPillSubslugHintPLStd, i18n.TrSpPillSubslugHintIntlStd},
}

type textEditorTemplate struct {
	ShowSceneTextModal              bool
	SelectedNode                    *book.EditorNodeView
	BookNarrated                    bool
	SceneTextModalName              string
	SceneTextModalText              string
	SluglinePrefixPills             []string
	SluglineTimePills               []string
	ScreenplayInsertPills           []book.ScreenplayInsertPill
	ScreenplayCharacterPills        []book.ScreenplayInsertPill
	HasScreenplayCharacterPills     bool
	SceneTextModalChapterKey        string
	IsVoiceSectionIsolated          bool
	VoiceRecorderOpen               bool
	HasBookVariables                bool
	SceneTextHasRightPanel          bool
	SceneTextIsFullWidth            bool
	SceneTextIsHalfWidth            bool
	SceneTextModalSettingsOpen      bool
	SceneTextModalSettingsOpenVal   string
	BookVarsSpec                    string
	BookVarSuggestions              []book.VariableDef
	BookEnumSuggestions             []book.SelectOption
	BookTargetSuggestions           []book.SelectOption
	ShowSyntaxHelpPopover           bool
	SyntaxHelpCondRows              []book.SelectOption
	IsScenePreviewNormal            bool
	IsScenePreviewHighlight         bool
	IsScenePreviewOutput            bool
	SceneTestVars                   []book.VariableDef
	HasSceneTestVars                bool
	ShowSceneTestVarsBar            bool
	SceneTestVarsSpec               string
	SceneTextModalHlSegments        []map[string]any
	SceneTextModalPreviewParagraphs []map[string]any
	SceneTextModalOutputProse       string
	SceneTextModalHeaderMeta        string
	SceneTextModalHasImage          bool
	SceneTextModalImageURL          string
	SceneTextModalIsImageless       bool
	SceneTextModalIncoming          []book.SelectOption
	SceneTextModalHasIncoming       bool
	IsHistoryPreview                bool
	IsScreenplayMode                bool
	ScriptStandard                  string
}

type textEditorScript struct {
	sceneId        string
	bookVarsSpec   string
	isScreenplay   bool
	scriptStandard string
	voiceOpen      bool
	voiceSection   string
	voiceUrl       string
	voiceTrimStart float64
	voiceTrimEnd   float64
}

func buildScreenplayFormatInsertPills(standard string, b *book.Book, lang engine.Lang) ([]book.ScreenplayInsertPill, []book.ScreenplayInsertPill) {
	isPL := book.NormalizeScriptStandard(standard) == book.ScriptStandardPL
	pickTr := func(plStdKey, intlStdKey engine.TrKey) string {
		if isPL {
			return engine.T(lang, plStdKey)
		}
		return engine.T(lang, intlStdKey)
	}
	insertPills := make([]book.ScreenplayInsertPill, len(screenplayInsertPillSpecs))
	for i, spec := range screenplayInsertPillSpecs {
		snip, selTgt, dblNl := book.ScreenplayBlockSnippet(standard, spec.kind, "", lang)
		insertPills[i] = book.ScreenplayInsertPill{
			Kind:          spec.kind,
			Label:         pickTr(spec.labelPL, spec.labelInt),
			Hint:          pickTr(spec.hintPL, spec.hintInt),
			Snippet:       snip,
			SelectTarget:  selTgt,
			DoubleNewline: dblNl,
		}
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
				Label:         charPillPrefix + chName,
				Hint:          engine.Tf(lang, i18n.TrSpPillCharDialogueHintFmt, chName),
				Snippet:       snip,
				SelectTarget:  selTgt,
				DoubleNewline: dblNl,
			})
		}
	}
	return insertPills, charPills
}

// TextEditorScriptView returns a script-only TemplateChunk hydrating TextEditorController.data on partial updates.
func TextEditorScriptView(s *state.StudioState) templates.TemplateChunk {
	if !s.Editor.SceneText.IsOpen() {
		return templates.TemplateChunk{}
	}
	b := s.Editor.ActiveBook
	isSp := false
	std := book.ScriptStandardPL
	var nv *book.EditorNodeView
	if b != nil {
		isSp = b.IsScreenplay()
		std = b.NormalizedStandard()
		if modalSc := b.FindScene(s.Editor.SceneText.SceneID); modalSc != nil {
			nv = buildEditorNodeView(s, modalSc, true)
		}
	}
	vOpen, vSec, vURL, vStart, vEnd := resolveVoiceScriptFields(s, nv)
	return templates.TemplateChunk{
		ScriptData: &textEditorScript{
			sceneId:        s.Editor.SceneText.SceneID,
			bookVarsSpec:   book.EncodeBookVariablesSpec(b),
			isScreenplay:   isSp,
			scriptStandard: std,
			voiceOpen:      vOpen,
			voiceSection:   vSec,
			voiceUrl:       vURL,
			voiceTrimStart: vStart,
			voiceTrimEnd:   vEnd,
		},
	}
}

func TextEditorView(s *state.StudioState) templates.TemplateChunk {
	if !s.Editor.SceneText.IsOpen() {
		return templates.TemplateChunk{}
	}
	b := s.Editor.ActiveBook
	modalSc := b.FindScene(s.Editor.SceneText.SceneID)
	if modalSc == nil {
		s.Editor.SceneText.Close()
		return templates.TemplateChunk{}
	}

	lang := LangOf(s)
	nv := buildEditorNodeView(s, modalSc, true)
	bookNarrated := b != nil && !b.IsScreenplay() && b.Narrated
	isVoiceSectionIsolated := bookNarrated && s.Overlays.VoiceRecorder && nv.HasVoiceSections
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
		ShowSceneTextModal:            true,
		SelectedNode:                  nv,
		BookNarrated:                  bookNarrated,
		SceneTextModalName:            modalSc.DisplayName(),
		SceneTextModalText:            displayedModalText,
		SceneTextModalChapterKey:      chKey,
		IsVoiceSectionIsolated:        isVoiceSectionIsolated,
		VoiceRecorderOpen:             s.Overlays.VoiceRecorder,
		HasBookVariables:              hasBookVars,
		SceneTextHasRightPanel:        s.Editor.RightPanel != state.PanelNone,
		SceneTextIsFullWidth:          isFullWidth,
		SceneTextIsHalfWidth:          !isFullWidth,
		SceneTextModalSettingsOpen:    s.Editor.SceneText.SettingsOpen,
		SceneTextModalSettingsOpenVal: settingsOpenVal,
		BookVarsSpec:                  book.EncodeBookVariablesSpec(b),
		BookVarSuggestions:            buildBookVarSuggestions(b),
		BookEnumSuggestions:           buildBookEnumValueSuggestions(b),
		BookTargetSuggestions:         buildBookTargetSuggestions(s, modalSc),
		ShowSyntaxHelpPopover:         hasBookVars && s.Editor.SceneText.SyntaxHelpOpen,
		SyntaxHelpCondRows:            buildSyntaxHelpCondRows(s),
		IsHistoryPreview:              !s.Config.StandaloneMode && s.Editor.History.IsPreview(b),
		ScriptStandard:                book.ScriptStandardPL,
	}

	if b != nil {
		v.IsScreenplayMode = b.IsScreenplay()
		v.ScriptStandard = b.NormalizedStandard()
	}

	if v.IsScreenplayMode {
		normStd := v.ScriptStandard
		v.SluglinePrefixPills, v.SluglineTimePills = book.SluglineHelperPills(normStd)
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
			isTrue := curVal == attrTrue
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
			modalHeaderMeta += metaSep + modalSc.Chapter
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
		defaultNextLbl := tr(s, i18n.TrNext)
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
				if !scHasMulti && (ch.IsImplicit || strings.EqualFold(choiceLbl, defaultNextLbl)) {
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
	nv.InspectorPorts = buildSceneInspectorChoices(s, modalSc, previewMode, testState)
	vOpen, vSec, vURL, vStart, vEnd := resolveVoiceScriptFields(s, nv)
	return templates.TemplateChunk{
		TemplateData: v,
		ScriptData: &textEditorScript{
			sceneId:        nv.ID,
			bookVarsSpec:   v.BookVarsSpec,
			isScreenplay:   v.IsScreenplayMode,
			scriptStandard: v.ScriptStandard,
			voiceOpen:      vOpen,
			voiceSection:   vSec,
			voiceUrl:       vURL,
			voiceTrimStart: vStart,
			voiceTrimEnd:   vEnd,
		},
	}
}

func resolveInheritedSceneImageURL(s *state.StudioState, target *book.Scene) string {
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
			if ch.IsEnding || ch.TargetSceneID == "" || strings.EqualFold(ch.TargetSceneID, endTargetID) {
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
