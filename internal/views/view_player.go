//go:build !(js && wasm)

package views

import (
	"strings"

	"mojra/internal/book"
	"mojra/internal/state"
	"mojra/internal/tpl"
)

const metaSep = " • "

type playerTemplate struct {
	ImagelessMode       bool
	IsHistoryPreview    bool
	HistoryPreviewID    string
	HistoryPreviewLabel string
	HeaderMeta          string
	RailTitle           string
	HasBgImage          bool
	BgImageURL          string
	HasHistory          bool
	HistoryScenes       []tpl.TemplateChunk
	HistoryTicks        []string
	CurrentScene        tpl.TemplateChunk
	ShowAudioDeck       bool
	AudioDeck           tpl.TemplateChunk
	PrefetchMediaURLs   string
	BookProgressPct     int
}

type playerScript struct {
	bookId        string
	sensitivity   string
	imagelessMode bool
}

func ViewPlayer(s *state.AppState) tpl.TemplateChunk {
	v := &playerTemplate{}
	scData := &playerScript{}
	chunk := tpl.TemplateChunk{
		TemplateData: v,
		ScriptData:   scData,
	}

	b := s.Player.Book
	imageless := b.IsEffectiveImageless(s.Settings)
	v.ImagelessMode = imageless
	scData.imagelessMode = imageless
	if b != nil {
		scData.bookId = b.ID
	}
	if s.Settings != nil {
		scData.sensitivity = book.NormalizeScrollSensitivity(s.Settings.ScrollSensitivity)
	}
	isHistPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(s.Editor.ActiveBook)
	v.IsHistoryPreview = isHistPreview
	if isHistPreview && s.Editor.History.Preview != nil {
		v.HistoryPreviewID = s.Editor.History.Preview.RevisionID
		v.HistoryPreviewLabel = s.Editor.History.Preview.RevisionLabel
	}

	if b == nil || len(b.Scenes) == 0 {
		return chunk
	}
	curScene := b.CurrentScene(s.Player.Save)
	if curScene == nil {
		curScene = b.Scenes[0]
	}

	histCount := 0
	if s.Player.Save != nil {
		for i, h := range s.Player.Save.History {
			if strings.EqualFold(h.SceneID, curScene.ID) {
				s.Player.Save.History = s.Player.Save.History[:i]
				s.Player.PersistSave(s.Config.SavesDir)
				break
			}
		}
		if book.EnsureHistoryFromStart(b, s.Player.Save) && !isHistPreview {
			s.Player.PersistSave(s.Config.SavesDir)
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
		histChunks := make([]tpl.TemplateChunk, 0, histCount)
		histTicks := make([]string, 0, histCount)
		for _, entry := range s.Player.Save.History {
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
			histChunks = append(histChunks, ViewScene(s, pastScene, pastVars, false))
			histTicks = append(histTicks, pastScene.DisplayName())
		}
		v.HistoryScenes = histChunks
		v.HistoryTicks = histTicks
	}

	if !imageless && curScene.ImageURL != "" {
		lastImageURL = curScene.ImageURL
	}
	headerMeta := b.Title
	if curScene.Chapter != "" {
		headerMeta = b.Title + metaSep + curScene.Chapter
	}
	v.HeaderMeta = headerMeta
	v.RailTitle = curScene.DisplayName()
	v.BgImageURL = lastImageURL
	v.HasBgImage = !imageless && lastImageURL != ""
	v.CurrentScene = ViewScene(s, curScene, curVars, true)
	v.ShowAudioDeck = b.Narrated
	if b.Narrated {
		v.AudioDeck = ViewAudioDeck(s, b, curScene, curVars)
	}

	seenURLs := make(map[string]bool, 16)
	if !imageless && curScene.ImageURL != "" {
		seenURLs[curScene.ImageURL] = true
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
		if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == endTargetID || seenScenes[ch.TargetSceneID] {
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
			if ch.IsEnding || ch.TargetSceneID == "" || ch.TargetSceneID == endTargetID || seenScenes[ch.TargetSceneID] {
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
	return chunk
}
