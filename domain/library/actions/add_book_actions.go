//go:build !(js && wasm)

package actions

import (
	"path/filepath"
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	"mojra/domain/i18n"
	"mojra/domain/library/state"
	"mojra/engine/transport"
)

const (
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

func openAddBookModal(s *state.LibraryState, _ *transport.ActionRequest) []transport.ActionFrame {
	s.Overlays.QR = false
	s.NewBook.Open = false
	s.Overlays.StudioProjects = false
	s.Overlays.AddBook = true
	return frameLibraryModals(s, "")
}

func closeAddBookModal(s *state.LibraryState, _ *transport.ActionRequest) []transport.ActionFrame {
	s.Overlays.AddBook = false
	return frameLibraryModals(s, "")
}

func browseAddBookFromDisk(s *state.LibraryState, req *transport.ActionRequest) []transport.ActionFrame {
	toast := ""
	if picked := pickDiskBookPath(s, req); picked != "" {
		toast = AddBookFromDiskPath(s, picked)
	}
	return frameLibraryAppResetCanvas(s, toast)
}

func addBookFromDisk(s *state.LibraryState, req *transport.ActionRequest) []transport.ActionFrame {
	rawPath := strings.TrimSpace(req.FormValue(formKeyPath))
	lowerPath := strings.ToLower(rawPath)
	if strings.HasPrefix(lowerPath, httpPrefix) || strings.HasPrefix(lowerPath, httpsPrefix) {
		return frameLibraryAppResetCanvas(s, addBookFromURL(s, rawPath))
	}
	return frameLibraryAppResetCanvas(s, AddBookFromDiskPath(s, rawPath))
}

// AddBookFromDiskPath resolves and loads one or more books from rawPath into LibraryState.
func AddBookFromDiskPath(s *state.LibraryState, rawPath string) string {
	if isScreenplayImportFile(rawPath) {
		return ImportScreenplayFromDiskFile(s, rawPath, false)
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

	if firstAdded != nil && s.Hooks.OnBookCreated != nil {
		s.Hooks.OnBookCreated(firstAdded, string(commonstate.ViewLibrary))
	}
	s.SaveSettings()

	if len(resolvedPaths) > 1 {
		return trf(s, i18n.TrAddedMultiDiskFmt, len(resolvedPaths))
	}
	if firstAdded != nil {
		return tr(s, i18n.TrAddedSingleDiskPrefix) + firstAdded.Title
	}
	return tr(s, i18n.TrAddedDiskGeneric)
}

func addBookFromURLAction(s *state.LibraryState, req *transport.ActionRequest) []transport.ActionFrame {
	rawURL := strings.TrimSpace(req.FormValue(formKeyURL))
	return frameLibraryAppResetCanvas(s, addBookFromURL(s, rawURL))
}

func addBookFromURL(s *state.LibraryState, rawURL string) string {
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
	if s.Hooks.OnBookCreated != nil {
		s.Hooks.OnBookCreated(downloaded, string(commonstate.ViewLibrary))
	}
	s.SaveSettings()
	return tr(s, i18n.TrDownloadedURLPrefix) + downloaded.Title
}
