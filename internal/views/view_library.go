package views

import (
	"strings"
	"time"

	"mojra/internal/book"
	"mojra/internal/logging"
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type libraryTemplate struct {
	DevMode bool
	Books   []book.Book
}

type libraryScript struct {
	bookIds string
}

func ViewLibrary(s *state.AppState) tpl.TemplateChunk {
	tLib := time.Now()
	defer logging.Active.MarkStep(logging.StageViewLibrary, tLib, s.Config.BooksDir)
	books := s.Library.Books
	isHistPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(s.Editor.ActiveBook)

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
		if !isHistPreview && s.Player.Book != nil && s.Player.Book.ID == b.ID && s.Player.Save != nil && s.Player.Save.UpdatedAtUnix >= sav.UpdatedAtUnix {
			sav = s.Player.Save
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
	return tpl.TemplateChunk{
		TemplateData: &libraryTemplate{
			DevMode: s.Config.DevMode,
			Books:   cards,
		},
		ScriptData: &libraryScript{
			bookIds: strings.Join(bookIDs, ","),
		},
	}
}
