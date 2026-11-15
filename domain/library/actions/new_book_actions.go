//go:build !(js && wasm)

package actions

import (
	"net/http"
	"path/filepath"
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	"mojra/domain/i18n"
	"mojra/domain/library/state"
	"mojra/engine/transport"
)

const (
	newBookFormTitle     = "title"
	newBookFormAuthor    = "author"
	newBookFormMode      = "mode"
	newBookFormStandard  = "standard"
	newBookFormNarrated  = "narrated"
	newBookFormImageless = "imageless"
	newBookFormSavePath  = "save_path"
	boolValTrue          = "true"
	boolValOn            = "on"
)

var _ = registerActions(map[string]ActionHandler{
	"OpenNewBookModal":    openNewBookModal,
	"CloseNewBookModal":   closeNewBookModal,
	"SetNewBookModalMode": setNewBookModalModeAction,
	"BrowseNewBookFolder": browseNewBookFolder,
	"CreateNewBook":       createNewBook,
})

func openNewBookModal(s *state.LibraryState, req *http.Request) []transport.ActionFrame {
	s.Overlays.QR = false
	s.Overlays.AddBook = false
	s.NewBook.Reset()
	s.NewBook.Open = true
	if req != nil && strings.EqualFold(strings.TrimSpace(req.FormValue(newBookFormMode)), book.BookModeScreenplay) {
		s.NewBook.Mode = book.BookModeScreenplay
	} else {
		s.NewBook.Mode = book.BookModeBook
	}
	s.NewBook.Standard = book.ScriptStandardPL
	s.NewBook.Dir = book.ResolveNewBookDir(s.Settings)
	return frameLibraryModals(s, "")
}

func closeNewBookModal(s *state.LibraryState, _ *http.Request) []transport.ActionFrame {
	s.NewBook.Reset()
	return frameLibraryModals(s, "")
}

func syncNewBookModalForm(s *state.LibraryState, req *http.Request) {
	if req == nil {
		return
	}
	s.NewBook.Title = req.FormValue(newBookFormTitle)
	s.NewBook.Author = req.FormValue(newBookFormAuthor)
	if m := strings.TrimSpace(req.FormValue(newBookFormMode)); m != "" {
		s.NewBook.Mode = book.NormalizeBookMode(m)
	}
	if std := strings.TrimSpace(req.FormValue(newBookFormStandard)); std != "" {
		s.NewBook.Standard = book.NormalizeScriptStandard(std)
	}
	rawNarr := strings.ToLower(strings.TrimSpace(req.FormValue(newBookFormNarrated)))
	s.NewBook.Narrated = rawNarr == boolValTrue || rawNarr == boolValOn || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(req.FormValue(newBookFormImageless)))
	s.NewBook.Imageless = rawImg == boolValTrue || rawImg == boolValOn || rawImg == "1"
	if curPath := strings.TrimSpace(req.FormValue(newBookFormSavePath)); curPath != "" {
		if expanded, err := book.ExpandLocalPath(curPath); err == nil && expanded != "" {
			s.NewBook.Dir = expanded
		}
	}
}

func setNewBookModalModeAction(s *state.LibraryState, req *http.Request) []transport.ActionFrame {
	syncNewBookModalForm(s, req)
	return frameLibraryModals(s, "")
}

func browseNewBookFolder(s *state.LibraryState, req *http.Request) []transport.ActionFrame {
	syncNewBookModalForm(s, req)
	picked := pickNativeBookPathUnlocked(s, true)
	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.NewBook.Dir = picked
		if s.Settings != nil {
			s.Settings.NewBookDir = picked
			s.SaveSettings()
		}
	}
	return frameLibraryModals(s, "")
}

func createNewBook(s *state.LibraryState, req *http.Request) []transport.ActionFrame {
	title := req.FormValue(newBookFormTitle)
	author := req.FormValue(newBookFormAuthor)
	mode := req.FormValue(newBookFormMode)
	standard := req.FormValue(newBookFormStandard)
	rawNarr := strings.ToLower(strings.TrimSpace(req.FormValue(newBookFormNarrated)))
	narrated := rawNarr == boolValTrue || rawNarr == boolValOn || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(req.FormValue(newBookFormImageless)))
	imageless := rawImg == boolValTrue || rawImg == boolValOn || rawImg == "1"

	targetDir := s.NewBook.Dir
	if targetDir == "" {
		targetDir = book.ResolveNewBookDir(s.Settings)
	}
	rawSavePath := strings.TrimSpace(req.FormValue(newBookFormSavePath))
	if rawSavePath != "" {
		expanded, err := book.ExpandLocalPath(rawSavePath)
		if err != nil {
			return frameLibraryAppResetCanvas(s, err.Error())
		}
		targetDir = expanded
	}
	if targetDir == "" {
		targetDir = s.Config.BooksDir
	}

	if created, err := book.CreateNewBookWithMode(targetDir, title, author, mode, standard, narrated, imageless); err == nil && created != nil {
		s.NewBook.Reset()
		if rawSavePath != "" && s.Settings != nil {
			s.Settings.NewBookDir = filepath.Dir(created.RootPath)
		}
		if filepath.Clean(filepath.Dir(created.RootPath)) != filepath.Clean(s.Config.BooksDir) {
			s.Settings.AddExternalBook(created.RootPath)
			created.IsExternal = true
		}
		s.Library.Put(created)
		if s.Hooks.OnBookCreated != nil {
			s.Hooks.OnBookCreated(created, string(commonstate.ViewEditor))
		}
		s.Settings.ActiveView = string(commonstate.ViewEditor)
		s.SaveSettings()
		if created.IsScreenplay() {
			return frameLibraryAppResetCanvas(s, tr(s, i18n.TrCreatedScreenplayPrefix)+created.Title)
		}
		return frameLibraryAppResetCanvas(s, tr(s, i18n.TrCreatedBookPrefix)+created.Title)
	} else if err != nil {
		return frameLibraryAppResetCanvas(s, err.Error())
	}
	return frameLibraryAppResetCanvas(s, "")
}
