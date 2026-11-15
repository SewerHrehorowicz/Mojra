package state

import (
	"encoding/hex"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/book/model"
	commonstate "mojra/domain/common/state"
	playerstate "mojra/domain/player/state"
	"mojra/engine/osbridge"
	"mojra/engine/templates"
)

const (
	themeDark    = "dark"
	scaleSmall   = "small"
	viewPlayer   = "player"
	sensMedium   = "medium"
)

// ShellState holds the single-book runtime state for the book_player shell (both WASM and standalone executable).
type ShellState struct {
	UI        *commonstate.UIState
	Player    playerstate.PlayerState
	Media     map[string][]byte
	BlobURLs  map[string]string
	Templates map[string]*templates.ParsedTemplate
	Storage   *osbridge.WasmStorage
}

// NewShellState initializes a single-book ShellState from a decoded Book and optional hex-encoded save state.
func NewShellState(
	loadedBook *model.Book,
	media map[string][]byte,
	savedHex string,
	templatesMap map[string]*templates.ParsedTemplate,
	storage *osbridge.WasmStorage,
) *ShellState {
	settings := &model.AppSettings{
		Theme:             themeDark,
		UIScale:           scaleSmall,
		ActiveView:        viewPlayer,
		PlaybackSpeed:     1.0,
		AutoChoiceDelay:   book.DefaultAutoChoiceDelay,
		ScrollSensitivity: sensMedium,
	}
	ui := &commonstate.UIState{
		Config: commonstate.RuntimeConfig{
			StandaloneMode: true,
		},
		Settings: settings,
	}

	var save *model.SaveState
	if trimmedHex := strings.TrimSpace(savedHex); trimmedHex != "" {
		if rawSave, decErr := hex.DecodeString(trimmedHex); decErr == nil {
			save = book.DecodeSaveStateBytes(loadedBook.ID, rawSave, loadedBook.StartSceneID)
		}
	}
	if save == nil {
		save = &model.SaveState{
			BookID:         loadedBook.ID,
			CurrentSceneID: loadedBook.StartSceneID,
			PlaybackSpeed:  1.0,
			Variables:      book.DefaultVariablesMap(loadedBook),
		}
		if startSc := loadedBook.FindScene(loadedBook.StartSceneID); startSc != nil && startSc.Effects != "" {
			book.ApplyEffects(startSc.Effects, save.Variables, loadedBook)
		}
	}
	if resolvedID := loadedBook.ResolveValidSceneID(save.CurrentSceneID); resolvedID != "" {
		save.CurrentSceneID = resolvedID
	}
	save.Variables = book.EnsureSaveVariables(loadedBook, save.Variables)
	book.EnsureHistoryFromStart(loadedBook, save)

	shell := &ShellState{
		UI:        ui,
		Media:     media,
		BlobURLs:  make(map[string]string),
		Templates: templatesMap,
		Storage:   storage,
	}
	shell.Player.BindUI(ui)
	shell.Player.Book = loadedBook
	shell.Player.Save = save
	shell.Player.SyncSaveHex()

	ui.Hooks.PlayerState = &shell.Player
	ui.Hooks.ShellState = shell
	ui.Hooks.ActiveBook = func() *model.Book {
		return shell.Player.Book
	}
	ui.Hooks.ActiveContextBook = func() *model.Book {
		return shell.Player.Book
	}
	ui.Hooks.IsEffectiveImageless = func() bool {
		return shell.Player.Book.IsEffectiveImageless(shell.UI.Settings)
	}
	ui.Hooks.IsBookLockedImageless = func() bool {
		return shell.Player.Book.IsImageless()
	}
	return shell
}
