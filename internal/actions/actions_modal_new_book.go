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
	s.Modals.QR = false
	s.Modals.AddBook = false
	s.ModalExport.Open = false
	s.ModalNewBook.Reset()
	s.ModalNewBook.Open = true
	if r != nil && strings.EqualFold(strings.TrimSpace(r.FormValue("mode")), "screenplay") {
		s.ModalNewBook.Mode = "screenplay"
	} else {
		s.ModalNewBook.Mode = "book"
	}
	s.ModalNewBook.Standard = "pl"
	s.ModalNewBook.Dir = book.ResolveNewBookDir(s.Settings)
	return ""
}

func closeNewBookModal(s *state.AppState, _ *http.Request) string {
	s.ModalNewBook.Reset()
	return ""
}

func browseNewBookFolder(s *state.AppState, r *http.Request) string {
	s.ModalNewBook.Title = r.FormValue("title")
	s.ModalNewBook.Author = r.FormValue("author")
	if m := strings.TrimSpace(r.FormValue("mode")); m != "" {
		s.ModalNewBook.Mode = book.NormalizeBookMode(m)
	}
	if std := strings.TrimSpace(r.FormValue("standard")); std != "" {
		s.ModalNewBook.Standard = book.NormalizeScriptStandard(std)
	}
	rawNarr := strings.ToLower(strings.TrimSpace(r.FormValue("narrated")))
	s.ModalNewBook.Narrated = rawNarr == "true" || rawNarr == "on" || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(r.FormValue("imageless")))
	s.ModalNewBook.Imageless = rawImg == "true" || rawImg == "on" || rawImg == "1"
	if curPath := strings.TrimSpace(r.FormValue("save_path")); curPath != "" {
		if expanded, err := book.ExpandLocalPath(curPath); err == nil && expanded != "" {
			s.ModalNewBook.Dir = expanded
		}
	}

	picked := pickNativeBookPathUnlocked(s, true)
	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.ModalNewBook.Dir = picked
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

	targetDir := s.ModalNewBook.Dir
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
		s.ModalNewBook.Reset()

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
