//go:build !(js && wasm)

package views

import (
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
	"mojra/internal/tpl"
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

func ViewScene(s *state.AppState, sc *book.Scene, vars map[string]string, isPresent bool) tpl.TemplateChunk {
	if s == nil || s.Player.Book == nil || sc == nil {
		return tpl.TemplateChunk{}
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
	hasMultiple := len(sc.Choices) > 1
	hasTitle := strings.TrimSpace(sc.Name) != ""
	hasImage := imgURL != ""
	isTerminal := len(sc.Choices) == 0 || sc.ExplicitEnd
	var choices []book.Choice
	if isPresent {
		choices = make([]book.Choice, 0, len(sc.Choices))
		for _, ch := range sc.Choices {
			condMet := book.EvaluateCondition(ch.Condition, vars, b)
			if !condMet && !ch.ShowWhenLocked {
				continue
			}
			item := ch
			if !hasMultiple {
				item.Label = tr(s, i18n.TrNext)
			}
			item.ShowWhenLocked = !condMet && ch.ShowWhenLocked
			choices = append(choices, item)
		}
	}
	var scriptData any
	if isPresent {
		scriptData = &sceneScript{
			isPresent:        isPresent,
			isScreenplayMode: isScreenplay,
			hasTitle:         hasTitle,
			hasImage:         hasImage,
			imageUrl:         imgURL,
			isTerminal:       isTerminal,
			isBranching:      hasMultiple,
		}
	}
	return tpl.TemplateChunk{
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
			IsTerminal:       isTerminal,
			IsBranching:      hasMultiple,
			Choices:          choices,
		},
		ScriptData: scriptData,
	}
}
