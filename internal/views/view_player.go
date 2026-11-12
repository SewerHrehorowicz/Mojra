//go:build !(js && wasm)

package views

import (
	"fmt"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

type playerScreenplayBlockItem struct {
	Kind            string `tpl:"kind"`
	Text            string `tpl:"text"`
	SceneNum        int    `tpl:"scene_num"`
	ShowSceneNum    bool   `tpl:"show_scene_num"`
	IsSlugline      bool   `tpl:"is_slugline"`
	IsCharacter     bool   `tpl:"is_character"`
	IsParenthetical bool   `tpl:"is_parenthetical"`
	IsDialogue      bool   `tpl:"is_dialogue"`
	IsTransition    bool   `tpl:"is_transition"`
}

type playerChoiceView struct {
	Idx        int    `tpl:"idx"`
	Label      string `tpl:"label"`
	Arrow      string `tpl:"arrow"`
	IsBranch   bool   `tpl:"is_branch"`
	IsImplicit bool   `tpl:"is_implicit"`
	IsLocked   bool   `tpl:"is_locked"`
}

type sceneData struct {
	Idx                int                         `tpl:"idx"`
	HeaderMeta         string                      `tpl:"header_meta"`
	Name               string                      `tpl:"name"`
	Slugline           string                      `tpl:"slugline"`
	RailTitle          string                      `tpl:"rail_title"`
	HasTitle           bool                        `tpl:"has_title"`
	Prose              string                      `tpl:"prose"`
	ScreenplayBlocks   []playerScreenplayBlockItem `tpl:"screenplay_blocks"`
	HasImage           bool                        `tpl:"has_image"`
	ImageURL           string                      `tpl:"image_url"`
	HasBgImage         bool                        `tpl:"has_bg_image"`
	BgImageURL         string                      `tpl:"bg_image_url"`
	AudioURL           string                      `tpl:"audio_url"`
	AudioQueue         string                      `tpl:"audio_queue"`
	AudioDurationLabel string                      `tpl:"audio_duration_label"`
	AudioTrimStart     string                      `tpl:"audio_trim_start"`
	AudioTrimEnd       string                      `tpl:"audio_trim_end"`
	IsTerminal         bool                        `tpl:"is_terminal"`
	Choices            []playerChoiceView          `tpl:"choices"`
}

type SceneData = sceneData

type playerView struct {
	ImagelessMode       bool        `tpl:"imageless_mode"`
	IsScreenplayMode    bool        `tpl:"is_screenplay_mode"`
	IsHistoryPreview    bool        `tpl:"is_history_preview"`
	HistoryPreviewID    string      `tpl:"history_preview_id"`
	HistoryPreviewLabel string      `tpl:"history_preview_label"`
	HasHistory          bool        `tpl:"has_history"`
	HistoryItems        []sceneData `tpl:"history_items"`
	CurrentScene        sceneData   `tpl:"current_scene"`
	ShowAudioDeck       bool        `tpl:"show_audio_deck"`
	PrefetchMediaURLs   string      `tpl:"prefetch_media_urls"`
	BookProgressPct     int         `tpl:"book_progress_pct"`
	AudioResumeTime     string      `tpl:"audio_resume_time"`
	AudioSpeed          string      `tpl:"audio_speed"`
	AutoChoiceEnabled   bool        `tpl:"auto_choice_enabled"`
	AutoChoiceLabel     string      `tpl:"auto_choice_label"`
	AutoChoiceDelay     string      `tpl:"auto_choice_delay"`
}

type sceneAudioResolved struct {
	url       string
	duration  string
	trimStart string
	trimEnd   string
	queueSpec string
}

func buildPlayerScreenplayBlocks(sc *book.Scene, vars map[string]string, b *book.Book, sceneNum int, standard string) []playerScreenplayBlockItem {
	if sc == nil {
		return nil
	}
	evalSc := sc
	if book.HasDynamicProse(sc.Text) {
		clone := *sc
		clone.Text = book.EvaluateProseText(sc.Text, vars, b)
		evalSc = &clone
	}
	normStd := book.NormalizeScriptStandard(standard)
	blocks := book.ParseScreenplaySceneBlocks(evalSc, normStd)
	if len(blocks) == 0 {
		return nil
	}
	numbered := book.IsNumberedStandard(normStd)
	renderedPrimarySlug := false
	out := make([]playerScreenplayBlockItem, 0, len(blocks))
	for _, blk := range blocks {
		showNum := false
		if blk.Kind == book.BlockSlugline && !renderedPrimarySlug {
			renderedPrimarySlug = true
			showNum = numbered && sceneNum > 0
		}
		out = append(out, playerScreenplayBlockItem{
			Kind:            string(blk.Kind),
			Text:            blk.Text,
			SceneNum:        sceneNum,
			ShowSceneNum:    showNum,
			IsSlugline:      blk.Kind == book.BlockSlugline,
			IsCharacter:     blk.Kind == book.BlockCharacter,
			IsParenthetical: blk.Kind == book.BlockParenthetical,
			IsDialogue:      blk.Kind == book.BlockDialogue,
			IsTransition:    blk.Kind == book.BlockTransition,
		})
	}
	return out
}

func resolvePlayerSceneAudio(b *book.Book, sc *book.Scene, vars map[string]string) sceneAudioResolved {
	if b == nil || !b.Narrated || sc == nil {
		return sceneAudioResolved{duration: "0:00", trimStart: "0.0000", trimEnd: "1.0000"}
	}
	queue := book.EvaluateSceneAudioQueue(sc, vars, b)
	queueSpec, _, totalDurLbl := book.EncodeSceneAudioQueue(queue)
	if len(queue) > 0 {
		ts, te := book.NormalizedVoiceTrimValues(queue[0].TrimStart, queue[0].TrimEnd)
		return sceneAudioResolved{
			url:       queue[0].AudioURL,
			duration:  totalDurLbl,
			trimStart: fmt.Sprintf("%.4f", ts),
			trimEnd:   fmt.Sprintf("%.4f", te),
			queueSpec: queueSpec,
		}
	}
	if sc.AudioURL == "" {
		return sceneAudioResolved{duration: "0:00", trimStart: "0.0000", trimEnd: "1.0000"}
	}
	durLbl, stamp := book.ReadSceneAudioDurationAndStamp(b, sc)
	if durLbl == "" {
		durLbl = "0:00"
	}
	urlStr := sc.AudioURL
	if stamp > 0 && !strings.Contains(urlStr, "?v=") {
		urlStr = fmt.Sprintf("%s?v=%d", urlStr, stamp)
	}
	ts, te := sc.NormalizedVoiceTrim()
	return sceneAudioResolved{
		url:       urlStr,
		duration:  durLbl,
		trimStart: fmt.Sprintf("%.4f", ts),
		trimEnd:   fmt.Sprintf("%.4f", te),
	}
}

func buildPlayerSceneData(s *state.AppState, sc *book.Scene, idx int, vars map[string]string, choices []playerChoiceView) sceneData {
	b := s.Player.Book
	imageless := b.IsEffectiveImageless(s.Settings)
	imgURL := ""
	if !imageless && sc.ImageURL != "" {
		imgURL = sc.ImageURL
	}
	headerMeta := b.Title
	if sc.Chapter != "" {
		headerMeta = b.Title + " • " + sc.Chapter
	}
	var blocks []playerScreenplayBlockItem
	if b.IsScreenplay() {
		blocks = buildPlayerScreenplayBlocks(sc, vars, b, sc.OrderIndex+1, b.NormalizedStandard())
	}
	prose := book.EvaluateSceneProseText(sc, vars, b)
	aud := resolvePlayerSceneAudio(b, sc, vars)
	return sceneData{
		Idx:                idx,
		HeaderMeta:         headerMeta,
		Name:               sc.Name,
		Slugline:           sc.Slugline,
		RailTitle:          sc.DisplayName(),
		HasTitle:           strings.TrimSpace(sc.Name) != "",
		Prose:              prose,
		ScreenplayBlocks:   blocks,
		HasImage:           imgURL != "",
		ImageURL:           imgURL,
		AudioURL:           aud.url,
		AudioQueue:         aud.queueSpec,
		AudioDurationLabel: aud.duration,
		AudioTrimStart:     aud.trimStart,
		AudioTrimEnd:       aud.trimEnd,
		IsTerminal:         len(sc.Choices) == 0 || sc.ExplicitEnd,
		Choices:            choices,
	}
}

func ViewPlayer(s *state.AppState) *playerView {
	v := &playerView{}
	populatePlayerContext(s, v)
	return v
}

func populatePlayerContext(s *state.AppState, v *playerView) {
	b := s.Player.Book
	imageless := b.IsEffectiveImageless(s.Settings)
	v.ImagelessMode = imageless
	isHistPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(s.Editor.ActiveBook)
	v.IsHistoryPreview = isHistPreview
	if isHistPreview && s.Editor.History.Preview != nil {
		v.HistoryPreviewID = s.Editor.History.Preview.RevisionID
		v.HistoryPreviewLabel = s.Editor.History.Preview.RevisionLabel
	}

	if b == nil || len(b.Scenes) == 0 {
		return
	}
	curScene := b.CurrentScene(s.Player.Save)
	if curScene == nil {
		curScene = b.Scenes[0]
	}

	v.IsScreenplayMode = b.IsScreenplay()

	histCount := 0
	if s.Player.Save != nil {
		for i, h := range s.Player.Save.History {
			if strings.EqualFold(h.SceneID, curScene.ID) {
				s.Player.Save.History = s.Player.Save.History[:i]
				s.Player.PersistSave(s.Config.SavesDir)
				break
			}
		}
		histCount = len(s.Player.Save.History)
		if len(s.Player.Save.Variables) == 0 && len(b.Variables) > 0 {
			s.Player.Save.Variables = book.EnsureSaveVariables(b, s.Player.Save.Variables)
			if histCount == 0 && curScene.Effects != "" {
				book.ApplyEffects(curScene.Effects, s.Player.Save.Variables, b)
			}
		} else {
			s.Player.Save.Variables = book.EnsureSaveVariables(b, s.Player.Save.Variables)
		}
	}
	var curVars map[string]string
	if s.Player.Save != nil {
		curVars = s.Player.Save.Variables
	} else {
		curVars = book.DefaultVariablesMap(b)
	}
	v.HasHistory = histCount > 0

	lastImageURL := ""
	if histCount > 0 {
		histList := make([]sceneData, 0, histCount)
		for idx, entry := range s.Player.Save.History {
			pastScene := b.FindScene(entry.SceneID)
			if pastScene == nil {
				continue
			}
			if !imageless && pastScene.ImageURL != "" {
				lastImageURL = pastScene.ImageURL
			}
			pastVars := entry.Variables
			if len(pastVars) == 0 {
				pastVars = curVars
			}
			sd := buildPlayerSceneData(s, pastScene, idx, pastVars, nil)
			sd.BgImageURL = lastImageURL
			sd.HasBgImage = !imageless && lastImageURL != ""
			histList = append(histList, sd)
		}
		v.HistoryItems = histList
	}

	hasMultipleChoices := len(curScene.Choices) > 1
	curChoices := make([]playerChoiceView, 0, len(curScene.Choices))
	for idx, ch := range curScene.Choices {
		condMet := book.EvaluateCondition(ch.Condition, curVars, b)
		if !condMet && !ch.ShowWhenLocked {
			continue
		}
		label := ch.Label
		if !hasMultipleChoices {
			label = tr(s, i18n.TrNext)
		}
		arrow := "»"
		if ch.IsEnding {
			arrow = tr(s, i18n.TrEndUpper)
		}
		isImplicitNext := !hasMultipleChoices && !ch.IsEnding && ch.TargetSceneID != "END"
		curChoices = append(curChoices, playerChoiceView{
			Idx:        idx,
			Label:      label,
			Arrow:      arrow,
			IsBranch:   hasMultipleChoices && !ch.IsEnding,
			IsImplicit: isImplicitNext,
			IsLocked:   !condMet && ch.ShowWhenLocked,
		})
	}

	if !imageless && curScene.ImageURL != "" {
		lastImageURL = curScene.ImageURL
	}
	curSD := buildPlayerSceneData(s, curScene, 0, curVars, curChoices)
	curSD.BgImageURL = lastImageURL
	curSD.HasBgImage = !imageless && lastImageURL != ""
	v.CurrentScene = curSD
	v.ShowAudioDeck = b.Narrated

	seenURLs := make(map[string]bool, 16)
	if curSD.ImageURL != "" {
		seenURLs[curSD.ImageURL] = true
	}
	if lastImageURL != "" {
		seenURLs[lastImageURL] = true
	}
	if curScene.AudioURL != "" {
		seenURLs[curScene.AudioURL] = true
	}

	seenScenes := make(map[string]bool, 16)
	seenScenes[curScene.ID] = true

	var hop1Scenes []*book.Scene
	for _, ch := range curScene.Choices {
		if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == "END" || seenScenes[ch.TargetSceneID] {
			continue
		}
		target := b.FindScene(ch.TargetSceneID)
		if target == nil {
			continue
		}
		seenScenes[target.ID] = true
		hop1Scenes = append(hop1Scenes, target)
	}

	var hop2Scenes []*book.Scene
	for _, h1 := range hop1Scenes {
		for _, ch := range h1.Choices {
			if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == "END" || seenScenes[ch.TargetSceneID] {
				continue
			}
			target := b.FindScene(ch.TargetSceneID)
			if target == nil {
				continue
			}
			seenScenes[target.ID] = true
			hop2Scenes = append(hop2Scenes, target)
		}
	}

	var prefetchURLs []string
	appendSceneMedia := func(sc *book.Scene) {
		if !imageless && sc.ImageURL != "" && !seenURLs[sc.ImageURL] {
			seenURLs[sc.ImageURL] = true
			prefetchURLs = append(prefetchURLs, sc.ImageURL)
		}
		if b.Narrated && sc.AudioURL != "" && !seenURLs[sc.AudioURL] {
			seenURLs[sc.AudioURL] = true
			prefetchURLs = append(prefetchURLs, sc.AudioURL)
		}
	}
	for _, sc := range hop1Scenes {
		appendSceneMedia(sc)
	}
	for _, sc := range hop2Scenes {
		appendSceneMedia(sc)
	}
	v.PrefetchMediaURLs = strings.Join(prefetchURLs, ";")

	progressPct := 0
	if len(b.Scenes) > 0 {
		visited := make(map[string]bool)
		if s.Player.Save != nil {
			for _, h := range s.Player.Save.History {
				if h.SceneID != "" {
					visited[h.SceneID] = true
				}
			}
		}
		if curScene.ID != "" {
			visited[curScene.ID] = true
		}
		progressPct = (len(visited) * 100) / len(b.Scenes)
		if progressPct < 1 {
			progressPct = 1
		}
		if progressPct > 100 {
			progressPct = 100
		}
	}
	v.BookProgressPct = progressPct

	var audioTime float64
	if s.Player.Save != nil {
		audioTime = s.Player.Save.AudioTime
	}
	v.AudioResumeTime = fmt.Sprintf("%.1f", audioTime)
	v.AudioSpeed = fmt.Sprintf("%.2g", s.Settings.PlaybackSpeed)
	v.AutoChoiceEnabled = s.Settings.AutoChoice
	if s.Settings.AutoChoice {
		v.AutoChoiceLabel = tr(s, i18n.TrAutoChoiceOnLabel)
	} else {
		v.AutoChoiceLabel = tr(s, i18n.TrAutoChoiceOffLabel)
	}
	v.AutoChoiceDelay = fmt.Sprintf("%.2g", book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay))
}
