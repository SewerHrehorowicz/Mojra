//go:build !(js && wasm)

package actions

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"mojra/internal/book"
	"mojra/internal/state"
)

func openAddBookModal(s *state.AppState, _ *http.Request) string {
	s.Modals.QR = false
	s.ModalNewBook.Open = false
	s.Modals.StudioProjects = false
	s.Modals.AddBook = true
	return ""
}

func closeAddBookModal(s *state.AppState, _ *http.Request) string {
	s.Modals.AddBook = false
	return ""
}

func browseAddBookFromDisk(s *state.AppState, r *http.Request) string {
	if picked := pickDiskBookPath(s, r); picked != "" {
		return addBookFromDiskPath(s, picked)
	}
	return ""
}

func addBookFromDisk(s *state.AppState, r *http.Request) string {
	rawPath := strings.TrimSpace(r.FormValue("path"))
	lowerPath := strings.ToLower(rawPath)
	if strings.HasPrefix(lowerPath, "http://") || strings.HasPrefix(lowerPath, "https://") {
		return addBookFromURL(s, rawPath)
	}
	return addBookFromDiskPath(s, rawPath)
}

func addBookFromDiskPath(s *state.AppState, rawPath string) string {
	if isScreenplayImportFile(rawPath) {
		return importScreenplayFromDiskFile(s, rawPath, false)
	}
	resolvedPaths, err := book.ResolveDiskBookPaths(rawPath)
	if err != nil {
		return err.Error()
	}

	cleanBooksDir := filepath.Clean(s.Config.BooksDir)
	var firstAdded *book.Book
	for _, p := range resolvedPaths {
		isExt := filepath.Clean(filepath.Dir(p)) != cleanBooksDir
		if isExt {
			s.Settings.AddExternalBook(p)
		}
		if loaded, lErr := book.LoadBookFromPath(p); lErr == nil && loaded != nil {
			loaded.IsExternal = isExt
			s.Library.Put(loaded)
			if firstAdded == nil {
				firstAdded = loaded
			}
		}
	}
	s.Modals.AddBook = false

	if firstAdded != nil {
		s.Player.SetBook(s.Config.SavesDir, s.Settings, firstAdded)
	}
	_ = s.SaveSettings()

	if len(resolvedPaths) > 1 {
		return fmt.Sprintf("Dodano %d książek z dysku (bez kopiowania)", len(resolvedPaths))
	}
	if firstAdded != nil {
		return "Dodano z dysku (bez kopiowania): " + firstAdded.Title
	}
	return "Dodano książkę z dysku"
}

func addBookFromURLAction(s *state.AppState, r *http.Request) string {
	rawURL := strings.TrimSpace(r.FormValue("url"))
	return addBookFromURL(s, rawURL)
}

func addBookFromURL(s *state.AppState, rawURL string) string {
	targetBooksDir := s.Config.BooksDir
	if !book.IsTempDir(s.Config.RootDir) || book.IsInsideRepoDir(s.Config.RootDir) {
		targetBooksDir = book.DefaultProjectsDir()
	}
	var downloaded *book.Book
	var err error
	s.WithUnlocked(func() {
		downloaded, err = book.DownloadBookFromURL(targetBooksDir, rawURL)
	})
	if err != nil {
		return err.Error()
	}
	if filepath.Clean(filepath.Dir(downloaded.RootPath)) != filepath.Clean(s.Config.BooksDir) {
		s.Settings.AddExternalBook(downloaded.RootPath)
		downloaded.IsExternal = true
	}
	s.Modals.AddBook = false
	s.Library.Put(downloaded)
	s.Player.SetBook(s.Config.SavesDir, s.Settings, downloaded)
	_ = s.SaveSettings()
	return "Pobrano książkę z URL: " + downloaded.Title
}
