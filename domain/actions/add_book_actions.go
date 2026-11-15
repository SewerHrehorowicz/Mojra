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
	formKeyPath = "path"
	formKeyURL  = "url"
	httpPrefix  = "http://"
	httpsPrefix = "https://"
)

var _ = registerActions(map[string]ActionHandler{
	"OpenAddBookModal":      openAddBookModal,
	"CloseAddBookModal":     closeAddBookModal,
	"BrowseAddBookFromDisk": browseAddBookFromDisk,
	"AddBookFromDisk":       addBookFromDisk,
	"AddBookFromURL":        addBookFromURLAction,
})

func openAddBookModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Overlays.QR = false
	s.NewBook.Open = false
	s.Overlays.StudioProjects = false
	s.Overlays.AddBook = true
	return frameModals(s, "")
}

func closeAddBookModal(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Overlays.AddBook = false
	return frameModals(s, "")
}

func browseAddBookFromDisk(s *state.AppState, r *http.Request) []engine.ActionFrame {
	toast := ""
	if picked := pickDiskBookPath(s, r); picked != "" {
		toast = addBookFromDiskPath(s, picked)
	}
	return frameAppResetCanvas(s, toast)
}

func addBookFromDisk(s *state.AppState, r *http.Request) []engine.ActionFrame {
	rawPath := strings.TrimSpace(r.FormValue(formKeyPath))
	lowerPath := strings.ToLower(rawPath)
	if strings.HasPrefix(lowerPath, httpPrefix) || strings.HasPrefix(lowerPath, httpsPrefix) {
		return frameAppResetCanvas(s, addBookFromURL(s, rawPath))
	}
	return frameAppResetCanvas(s, addBookFromDiskPath(s, rawPath))
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
	s.Overlays.AddBook = false

	if firstAdded != nil {
		s.Player.SetBook(s.Config.SavesDir, s.Settings, firstAdded)
	}
	_ = s.SaveSettings()

	if len(resolvedPaths) > 1 {
		return trf(s, i18n.TrAddedMultiDiskFmt, len(resolvedPaths))
	}
	if firstAdded != nil {
		return tr(s, i18n.TrAddedSingleDiskPrefix) + firstAdded.Title
	}
	return tr(s, i18n.TrAddedDiskGeneric)
}

func addBookFromURLAction(s *state.AppState, r *http.Request) []engine.ActionFrame {
	rawURL := strings.TrimSpace(r.FormValue(formKeyURL))
	return frameAppResetCanvas(s, addBookFromURL(s, rawURL))
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
	s.Overlays.AddBook = false
	s.Library.Put(downloaded)
	s.Player.SetBook(s.Config.SavesDir, s.Settings, downloaded)
	_ = s.SaveSettings()
	return tr(s, i18n.TrDownloadedURLPrefix) + downloaded.Title
}
