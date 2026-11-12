package state

import "mojra/internal/book"

// PlayerState holds the active reader/player book, reading progress, and history preview backup.
type PlayerState struct {
	Book              *book.Book
	Save              *book.SaveState
	LiveBook          *book.Book
	LiveSave          *book.SaveState
	ZenMode           bool
	HistoryRevealed   bool
	FocusedHistoryIdx int
}

// PersistSave writes the current player SaveState to saves/<book-id>.sav.
func (p *PlayerState) PersistSave(savesDir string) {
	if p.Book == nil || p.Save == nil {
		return
	}
	p.Save.BookID = p.Book.ID
	if err := book.WriteSaveState(savesDir, p.Save); err != nil {
		return
	}
}

// SetBook sets the active book in the player and loads or initializes its .sav progress.
func (p *PlayerState) SetBook(savesDir string, settings *book.AppSettings, b *book.Book) {
	p.Book = b
	p.HistoryRevealed = false
	p.FocusedHistoryIdx = -1
	if b == nil {
		p.Save = nil
		return
	}
	if settings != nil {
		settings.ActiveBookID = b.ID
		if err := book.SaveAppSettings(savesDir, settings); err != nil {
			return
		}
	}
	p.Save = book.LoadSaveState(savesDir, b.ID, b.StartSceneID)
	if b.FindScene(p.Save.CurrentSceneID) == nil && len(b.Scenes) > 0 {
		p.Save.CurrentSceneID = b.StartSceneID
		if b.FindScene(p.Save.CurrentSceneID) == nil {
			p.Save.CurrentSceneID = b.Scenes[0].ID
		}
		p.PersistSave(savesDir)
	}
}

// ExitHistoryPreview restores the live player book and save state after exiting a Studio history preview.
func (p *PlayerState) ExitHistoryPreview(savesDir string, studioBook *book.Book) {
	if p.LiveBook != nil {
		p.Book = p.LiveBook
		p.LiveBook = nil
	}
	if studioBook != nil && p.Book != nil && p.Book.ID == studioBook.ID {
		p.Book = studioBook
	}
	if p.LiveSave != nil {
		p.Save = p.LiveSave
		p.LiveSave = nil
	} else if p.Book != nil {
		p.Save = book.LoadSaveState(savesDir, p.Book.ID, p.Book.StartSceneID)
	}
	if p.Book != nil && p.Save != nil {
		if p.Book.FindScene(p.Save.CurrentSceneID) == nil && len(p.Book.Scenes) > 0 {
			p.Save.CurrentSceneID = p.Book.StartSceneID
			if p.Book.FindScene(p.Save.CurrentSceneID) == nil {
				p.Save.CurrentSceneID = p.Book.Scenes[0].ID
			}
		}
	}
	p.HistoryRevealed = false
	p.FocusedHistoryIdx = -1
}

// IsEffectiveZenMode reports whether Zen mode is active in the player view.
func (p *PlayerState) IsEffectiveZenMode(activeView WorkspaceView) bool {
	return p.ZenMode && p.Book != nil && activeView == ViewPlayer
}
