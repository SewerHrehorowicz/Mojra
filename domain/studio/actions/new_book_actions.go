//go:build !(js && wasm)

package actions

import (
	"path/filepath"
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/transport"
)

var _ = registerActions(map[string]ActionHandler{
	"OpenNewBookModal":    openNewBookModal,
	"CloseNewBookModal":   closeNewBookModal,
	"SetNewBookModalMode": setNewBookModalModeAction,
	"BrowseNewBookFolder": browseNewBookFolder,
	"CreateNewBook":       createNewBook,
})

func openNewBookModal(studioState *state.StudioState, req *transport.ActionRequest) []transport.ActionFrame {
	studioState.Overlays.Settings = false
	studioState.Overlays.AddBook = false
	studioState.Overlays.StudioProjects = false
	studioState.Export.Open = false
	studioState.NewBook.Reset()
	studioState.NewBook.Open = true
	if req != nil && strings.EqualFold(strings.TrimSpace(req.FormValue(newBookFormMode)), book.BookModeScreenplay) {
		studioState.NewBook.Mode = book.BookModeScreenplay
	} else {
		studioState.NewBook.Mode = book.BookModeBook
	}
	studioState.NewBook.Standard = book.ScriptStandardPL
	studioState.NewBook.Dir = book.ResolveNewBookDir(studioState.Settings)
	return frameModals(studioState, "")
}

func closeNewBookModal(studioState *state.StudioState, _ *transport.ActionRequest) []transport.ActionFrame {
	studioState.NewBook.Reset()
	return frameModals(studioState, "")
}

func syncNewBookModalForm(studioState *state.StudioState, req *transport.ActionRequest) {
	if req == nil {
		return
	}
	studioState.NewBook.Title = req.FormValue(newBookFormTitle)
	studioState.NewBook.Author = req.FormValue(newBookFormAuthor)
	if rawMode := strings.TrimSpace(req.FormValue(newBookFormMode)); rawMode != "" {
		studioState.NewBook.Mode = book.NormalizeBookMode(rawMode)
	}
	if rawStandard := strings.TrimSpace(req.FormValue(newBookFormStandard)); rawStandard != "" {
		studioState.NewBook.Standard = book.NormalizeScriptStandard(rawStandard)
	}
	rawNarr := strings.ToLower(strings.TrimSpace(req.FormValue(newBookFormNarrated)))
	studioState.NewBook.Narrated = rawNarr == boolValTrue || rawNarr == boolValOn || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(req.FormValue(newBookFormImageless)))
	studioState.NewBook.Imageless = rawImg == boolValTrue || rawImg == boolValOn || rawImg == "1"
	if curPath := strings.TrimSpace(req.FormValue(newBookFormSavePath)); curPath != "" {
		if expanded, err := book.ExpandLocalPath(curPath); err == nil && expanded != "" {
			studioState.NewBook.Dir = expanded
		}
	}
}

func setNewBookModalModeAction(studioState *state.StudioState, req *transport.ActionRequest) []transport.ActionFrame {
	syncNewBookModalForm(studioState, req)
	return frameModals(studioState, "")
}

func browseNewBookFolder(studioState *state.StudioState, req *transport.ActionRequest) []transport.ActionFrame {
	syncNewBookModalForm(studioState, req)
	picked := pickNativeBookPathUnlocked(studioState, true)
	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		studioState.NewBook.Dir = picked
		if studioState.Settings != nil {
			studioState.Settings.NewBookDir = picked
			studioState.SaveSettings()
		}
	}
	return frameModals(studioState, "")
}

func createNewBook(studioState *state.StudioState, req *transport.ActionRequest) []transport.ActionFrame {
	title := req.FormValue(newBookFormTitle)
	author := req.FormValue(newBookFormAuthor)
	mode := req.FormValue(newBookFormMode)
	standard := req.FormValue(newBookFormStandard)
	rawNarr := strings.ToLower(strings.TrimSpace(req.FormValue(newBookFormNarrated)))
	narrated := rawNarr == boolValTrue || rawNarr == boolValOn || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(req.FormValue(newBookFormImageless)))
	imageless := rawImg == boolValTrue || rawImg == boolValOn || rawImg == "1"

	targetDir := studioState.NewBook.Dir
	if targetDir == "" {
		targetDir = book.ResolveNewBookDir(studioState.Settings)
	}
	rawSavePath := strings.TrimSpace(req.FormValue(newBookFormSavePath))
	if rawSavePath != "" {
		expanded, err := book.ExpandLocalPath(rawSavePath)
		if err != nil {
			return frameAppResetCanvas(studioState, err.Error())
		}
		targetDir = expanded
	}
	if targetDir == "" {
		targetDir = studioState.Config.BooksDir
	}

	if created, err := book.CreateNewBookWithMode(targetDir, title, author, mode, standard, narrated, imageless); err == nil && created != nil {
		studioState.NewBook.Reset()
		if rawSavePath != "" && studioState.Settings != nil {
			studioState.Settings.NewBookDir = filepath.Dir(created.RootPath)
		}
		if filepath.Clean(filepath.Dir(created.RootPath)) != filepath.Clean(studioState.Config.BooksDir) {
			studioState.Settings.AddExternalBook(created.RootPath)
			created.IsExternal = true
		}
		if studioState.Hooks.OnBookCreated != nil {
			studioState.Hooks.OnBookCreated(created, string(commonstate.ViewEditor))
		}
		studioState.Settings.ActiveView = string(commonstate.ViewEditor)
		studioState.SaveSettings()
		if created.IsScreenplay() {
			return frameAppResetCanvas(studioState, tr(studioState, i18n.TrCreatedScreenplayPrefix)+created.Title)
		}
		return frameAppResetCanvas(studioState, tr(studioState, i18n.TrCreatedBookPrefix)+created.Title)
	} else if err != nil {
		return frameAppResetCanvas(studioState, err.Error())
	}
	return frameAppResetCanvas(studioState, "")
}
