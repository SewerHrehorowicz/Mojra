package views

import (
	"strings"

	book "mojra/domain/book/actions"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/player/state"
	"mojra/engine/templates"
)

const endTargetID = "END"

type sceneTemplate struct {
	IsPresent        bool
	IsScreenplayMode bool
	SceneNum         int
	ShowSceneNum     bool
	Name             string
	Slugline         string
	RailTitle        string
	HasTitle         bool
	Prose            string
	ScreenplayBlocks []book.ScreenplayBlock
	HasImage         bool
	ImageURL         string
	IsTerminal       bool
	IsBranching      bool
	Choices          []book.Choice
}

type sceneScript struct {
	isPresent        bool
	isScreenplayMode bool
	hasTitle         bool
	hasImage         bool
	imageUrl         string
	isTerminal       bool
	isBranching      bool
}

func SceneView(s *state.PlayerState, sc *book.Scene, vars map[string]string, isPresent bool) templates.TemplateChunk {
	if s == nil || s.Player == nil {
		if s != nil {
			s.Player = s
		}
	}
	if s == nil || s.Player.Book == nil || sc == nil {
		return templates.TemplateChunk{}
	}
	b := s.Player.Book
	imageless := b.IsEffectiveImageless(s.Settings)
	imgURL := ""
	if !imageless && sc.ImageURL != "" {
		imgURL = sc.ImageURL
	}
	isScreenplay := b.IsScreenplay()
	sceneNum := sc.OrderIndex + 1
	showSceneNum := false
	var blocks []book.ScreenplayBlock
	if isScreenplay {
		evalSc := sc
		if book.HasDynamicProse(sc.Text) {
			clone := *sc
			clone.Text = book.EvaluateProseText(sc.Text, vars, b)
			evalSc = &clone
		}
		normStd := book.NormalizeScriptStandard(b.NormalizedStandard())
		blocks = book.ParseScreenplaySceneBlocks(evalSc, normStd)
		showSceneNum = book.IsNumberedStandard(normStd) && sceneNum > 0
	}
	eval := b.EvaluateSceneChoices(sc, vars, commonviews.Tr(s.UIState, i18n.TrNext))
	hasTitle := strings.TrimSpace(sc.Name) != ""
	hasImage := imgURL != ""
	var choices []book.Choice
	if isPresent {
		choices = eval.Visible
	}
	var scriptData any
	if isPresent {
		scriptData = &sceneScript{
			isPresent:        isPresent,
			isScreenplayMode: isScreenplay,
			hasTitle:         hasTitle,
			hasImage:         hasImage,
			imageUrl:         imgURL,
			isTerminal:       eval.IsTerminal,
			isBranching:      eval.IsBranching,
		}
	}
	return templates.TemplateChunk{
		TemplateData: &sceneTemplate{
			IsPresent:        isPresent,
			IsScreenplayMode: isScreenplay,
			SceneNum:         sceneNum,
			ShowSceneNum:     showSceneNum,
			Name:             sc.Name,
			Slugline:         sc.Slugline,
			RailTitle:        sc.DisplayName(),
			HasTitle:         hasTitle,
			Prose:            book.EvaluateSceneProseText(sc, vars, b),
			ScreenplayBlocks: blocks,
			HasImage:         hasImage,
			ImageURL:         imgURL,
			IsTerminal:       eval.IsTerminal,
			IsBranching:      eval.IsBranching,
			Choices:          choices,
		},
		ScriptData: scriptData,
	}
}
