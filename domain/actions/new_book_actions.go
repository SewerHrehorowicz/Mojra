//go:build !(js && wasm)

package actions

import (
	"net/http"
	"path/filepath"
	"strings"

	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine"
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

func openNewBookModal(s *state.AppState, r *http.Request) []engine.ActionFrame {
	s.Overlays.QR = false
	s.Overlays.AddBook = false
	s.Export.Open = false
	s.NewBook.Reset()
	s.NewBook.Open = true
	if r != nil && strings.EqualFold(strings.TrimSpace(r.FormValue(newBookFormMode)), book.BookModeScreenplay) {
		s.NewBook.Mode = book.BookModeScreenplay
	} else {
		s.NewBook.Mode = book.BookModeBook
	}
	s.NewBook.Standard = book.ScriptStandardPL
	s.NewBook.Dir = book.ResolveNewBookDir(s.Settings)
	return frameModals(s, "")
}

func closeNewBookModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.NewBook.Reset()
	return frameModals(s, "")
}

func syncNewBookModalForm(s *state.AppState, r *http.Request) {
	if r == nil {
		return
	}
	s.NewBook.Title = r.FormValue(newBookFormTitle)
	s.NewBook.Author = r.FormValue(newBookFormAuthor)
	if m := strings.TrimSpace(r.FormValue(newBookFormMode)); m != "" {
		s.NewBook.Mode = book.NormalizeBookMode(m)
	}
	if std := strings.TrimSpace(r.FormValue(newBookFormStandard)); std != "" {
		s.NewBook.Standard = book.NormalizeScriptStandard(std)
	}
	rawNarr := strings.ToLower(strings.TrimSpace(r.FormValue(newBookFormNarrated)))
	s.NewBook.Narrated = rawNarr == boolValTrue || rawNarr == boolValOn || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(r.FormValue(newBookFormImageless)))
	s.NewBook.Imageless = rawImg == boolValTrue || rawImg == boolValOn || rawImg == "1"
	if curPath := strings.TrimSpace(r.FormValue(newBookFormSavePath)); curPath != "" {
		if expanded, err := book.ExpandLocalPath(curPath); err == nil && expanded != "" {
			s.NewBook.Dir = expanded
		}
	}
}

func setNewBookModalModeAction(s *state.AppState, r *http.Request) []engine.ActionFrame {
	syncNewBookModalForm(s, r)
	return frameModals(s, "")
}

func browseNewBookFolder(s *state.AppState, r *http.Request) []engine.ActionFrame {
	syncNewBookModalForm(s, r)

	picked := pickNativeBookPathUnlocked(s, true)
	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.NewBook.Dir = picked
		if s.Settings != nil {
			s.Settings.NewBookDir = picked
			_ = s.SaveSettings()
		}
	}
	return frameModals(s, "")
}

func createNewBook(s *state.AppState, r *http.Request) []engine.ActionFrame {
	title := r.FormValue(newBookFormTitle)
	author := r.FormValue(newBookFormAuthor)
	mode := r.FormValue(newBookFormMode)
	standard := r.FormValue(newBookFormStandard)
	rawNarr := strings.ToLower(strings.TrimSpace(r.FormValue(newBookFormNarrated)))
	narrated := rawNarr == boolValTrue || rawNarr == boolValOn || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(r.FormValue(newBookFormImageless)))
	imageless := rawImg == boolValTrue || rawImg == boolValOn || rawImg == "1"

	targetDir := s.NewBook.Dir
	if targetDir == "" {
		targetDir = book.ResolveNewBookDir(s.Settings)
	}
	rawSavePath := strings.TrimSpace(r.FormValue(newBookFormSavePath))
	if rawSavePath != "" {
		expanded, err := book.ExpandLocalPath(rawSavePath)
		if err != nil {
			return frameAppResetCanvas(s, err.Error())
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
		s.Editor.SetBook(created, s.Settings)
		if s.Player.Book == nil {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, created)
		}
		s.Settings.ActiveView = state.ViewEditor.String()
		_ = s.SaveSettings()
		if created.IsScreenplay() {
			return frameAppResetCanvas(s, tr(s, i18n.TrCreatedScreenplayPrefix)+created.Title)
		}
		return frameAppResetCanvas(s, tr(s, i18n.TrCreatedBookPrefix)+created.Title)
	} else if err != nil {
		return frameAppResetCanvas(s, err.Error())
	}
	return frameAppResetCanvas(s, "")
}
