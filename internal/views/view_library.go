package views

import (
	"time"

	"mojra/internal/book"
	"mojra/internal/logging"
	"mojra/internal/state"
)

type libraryBookCard struct {
	Book        *book.Book `tpl:"*"`
	ProgressPct int        `tpl:"progress_pct"`
}

type libraryView struct {
	DevMode bool              `tpl:"dev_mode"`
	Books   []libraryBookCard `tpl:"books"`
}

func ViewLibrary(s *state.AppState) *libraryView {
	tLib := time.Now()
	defer logging.Active.MarkStep(logging.StageViewLibrary, tLib, s.Config.BooksDir)
	books := s.Library.Books
	isHistPreview := !s.Config.StandaloneMode && s.Editor.History.IsPreview(s.Editor.ActiveBook)

	cards := make([]libraryBookCard, 0, len(books))
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
		cards = append(cards, libraryBookCard{
			Book:        b,
			ProgressPct: progressPct,
		})
	}
	return &libraryView{
		DevMode: s.Config.DevMode,
		Books:   cards,
	}
}
