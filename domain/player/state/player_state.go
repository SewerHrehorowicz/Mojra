package state

import (
	"encoding/hex"

	book "mojra/domain/book/actions"
	"mojra/domain/book/model"
	commonstate "mojra/domain/common/state"
)

// PlayerState holds the active reader/player book, reading progress, and history preview backup.
type PlayerState struct {
	*commonstate.UIState
	Player              *PlayerState
	Book                *model.Book
	Save                *model.SaveState
	LiveBook            *model.Book
	LiveSave            *model.SaveState
	ZenMode             bool
	HistoryRevealed     bool
	FocusedHistoryIdx   int
	SaveHex             string
	IsHistoryPreview    bool
	HistoryPreviewID    string
	HistoryPreviewLabel string
}

// BindUI links the player state to the shared UIState and initializes self-reference.
func (player *PlayerState) BindUI(ui *commonstate.UIState) {
	player.UIState = ui
	player.Player = player
}

// SyncSaveHex encodes Save into SaveHex for standalone/WASM persistence.
func (player *PlayerState) SyncSaveHex() {
	if player == nil || player.Save == nil {
		return
	}
	player.SaveHex = hex.EncodeToString(book.EncodeSaveStateBytes(player.Save))
}

// PersistSave writes the current player SaveState to saves/<book-id>.sav.
func (player *PlayerState) PersistSave(savesDir string) {
	if player.Book == nil || player.Save == nil || savesDir == "" || player.IsHistoryPreview {
		return
	}
	player.Save.BookID = player.Book.ID
	if err := book.WriteSaveState(savesDir, player.Save); err != nil {
		return
	}
}

// SetBook sets the active book in the player and loads or initializes its .sav progress.
func (player *PlayerState) SetBook(savesDir string, settings *model.AppSettings, targetBook *model.Book) {
	player.Book = targetBook
	player.HistoryRevealed = false
	player.FocusedHistoryIdx = -1
	if targetBook == nil {
		player.Save = nil
		return
	}
	if settings != nil {
		settings.ActiveBookID = targetBook.ID
		if err := book.SaveAppSettings(savesDir, settings); err != nil {
			return
		}
	}
	player.Save = book.LoadSaveState(savesDir, targetBook.ID, targetBook.StartSceneID)
	needsPersist := false
	if resolvedID := targetBook.ResolveValidSceneID(player.Save.CurrentSceneID); resolvedID != "" && resolvedID != player.Save.CurrentSceneID {
		player.Save.CurrentSceneID = resolvedID
		needsPersist = true
	}
	if book.EnsureHistoryFromStart(targetBook, player.Save) {
		needsPersist = true
	}
	if needsPersist {
		player.PersistSave(savesDir)
	}
}

// ExitHistoryPreview restores the live player book and save state after exiting a Studio history preview.
func (player *PlayerState) ExitHistoryPreview(savesDir string, studioBook *model.Book) {
	player.IsHistoryPreview = false
	player.HistoryPreviewID = ""
	player.HistoryPreviewLabel = ""
	if player.LiveBook != nil {
		player.Book = player.LiveBook
		player.LiveBook = nil
	}
	if studioBook != nil && player.Book != nil && player.Book.ID == studioBook.ID {
		player.Book = studioBook
	}
	if player.LiveSave != nil {
		player.Save = player.LiveSave
		player.LiveSave = nil
	} else if player.Book != nil {
		player.Save = book.LoadSaveState(savesDir, player.Book.ID, player.Book.StartSceneID)
	}
	if player.Book != nil && player.Save != nil {
		if resolvedID := player.Book.ResolveValidSceneID(player.Save.CurrentSceneID); resolvedID != "" {
			player.Save.CurrentSceneID = resolvedID
		}
	}
	player.HistoryRevealed = false
	player.FocusedHistoryIdx = -1
}

// IsEffectiveZenMode reports whether Zen mode is active in the player view.
func (player *PlayerState) IsEffectiveZenMode(activeView commonstate.ViewMode) bool {
	return player.ZenMode && player.Book != nil && activeView == commonstate.ViewPlayer
}
