//go:build !(js && wasm)

package actions

import (
	"net/http"
	"path/filepath"
	"strings"

	"mojra/internal/book"
	"mojra/internal/state"
)

func openNewBookModal(s *state.AppState, r *http.Request) string {
	s.Overlays.QR = false
	s.Overlays.AddBook = false
	s.Export.Open = false
	s.NewBook.Reset()
	s.NewBook.Open = true
	if r != nil && strings.EqualFold(strings.TrimSpace(r.FormValue("mode")), "screenplay") {
		s.NewBook.Mode = "screenplay"
	} else {
		s.NewBook.Mode = "book"
	}
	s.NewBook.Standard = "pl"
	s.NewBook.Dir = book.ResolveNewBookDir(s.Settings)
	return ""
}

func closeNewBookModal(s *state.AppState, _ *http.Request) string {
	s.NewBook.Reset()
	return ""
}

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

func setNewBookModalModeAction(s *state.AppState, r *http.Request) string {
	syncNewBookModalForm(s, r)
	return ""
}

func browseNewBookFolder(s *state.AppState, r *http.Request) string {
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
	return ""
}

func createNewBook(s *state.AppState, r *http.Request) string {
	title := r.FormValue("title")
	author := r.FormValue("author")
	mode := r.FormValue("mode")
	standard := r.FormValue("standard")
	rawNarr := strings.ToLower(strings.TrimSpace(r.FormValue("narrated")))
	narrated := rawNarr == "true" || rawNarr == "on" || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(r.FormValue("imageless")))
	imageless := rawImg == "true" || rawImg == "on" || rawImg == "1"

	targetDir := s.NewBook.Dir
	if targetDir == "" {
		targetDir = book.ResolveNewBookDir(s.Settings)
	}
	rawSavePath := strings.TrimSpace(r.FormValue("save_path"))
	if rawSavePath != "" {
		expanded, err := book.ExpandLocalPath(rawSavePath)
		if err != nil {
			return err.Error()
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
			return "Utworzono nowy scenariusz: " + created.Title
		}
		return "Utworzono nową książkę: " + created.Title
	} else if err != nil {
		return err.Error()
	}
	return ""
}
