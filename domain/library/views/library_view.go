//go:build !(js && wasm)

package views

import (
	"strings"
	"time"

	book "mojra/domain/book/actions"
	"mojra/domain/library/state"
	"mojra/engine"
	"mojra/engine/templates"
)

type libraryTemplate struct {
	DevMode bool
	Books   []book.Book
}

type libraryScript struct {
	bookIds string
}

func LibraryView(s *state.LibraryState) templates.TemplateChunk {
	if s.Library == nil {
		s.Library = s
	}
	tLib := time.Now()
	defer engine.Active.MarkStep(engine.StageViewLibrary, tLib, s.Config.BooksDir)
	books := s.Library.Books
	isHistPreview := false
	if s.Hooks.IsHistoryPreview != nil {
		isHistPreview = s.Hooks.IsHistoryPreview()
	}

	cards := make([]book.Book, 0, len(books))
	bookIDs := make([]string, 0, len(books))
	seenTitleKeys := make(map[string]bool, len(books))
	for _, b := range books {
		titleKey := book.SanitizeBookID(b.Title)
		if titleKey != "" {
			if seenTitleKeys[titleKey] {
				continue
			}
			seenTitleKeys[titleKey] = true
		}
		sav := book.LoadSaveState(s.Config.SavesDir, b.ID, b.StartSceneID)
		if !isHistPreview && s.Hooks.ActivePlayerSave != nil {
			if activeSav := s.Hooks.ActivePlayerSave(b.ID); activeSav != nil && activeSav.UpdatedAtUnix >= sav.UpdatedAtUnix {
				sav = activeSav
			}
		}
		hasProgress := len(sav.History) > 0 || (sav.CurrentSceneID != "" && sav.CurrentSceneID != b.StartSceneID) || sav.AudioTime > 0.5
		progressPct := 0
		if hasProgress && len(b.Scenes) > 0 {
			visited := make(map[string]bool)
			for _, h := range sav.History {
				if h.SceneID != "" {
					visited[h.SceneID] = true
				}
			}
			if sav.CurrentSceneID != "" {
				visited[sav.CurrentSceneID] = true
			}
			progressPct = (len(visited) * 100) / len(b.Scenes)
			if progressPct < 1 {
				progressPct = 1
			}
			if progressPct > 100 {
				progressPct = 100
			}
		}
		item := *b
		item.ProgressPct = progressPct
		cards = append(cards, item)
		bookIDs = append(bookIDs, b.ID)
	}
	return templates.TemplateChunk{
		TemplateData: &libraryTemplate{
			DevMode: s.Config.DevMode,
			Books:   cards,
		},
		ScriptData: &libraryScript{
			bookIds: strings.Join(bookIDs, ","),
		},
	}
}
