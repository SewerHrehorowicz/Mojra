//go:build !(js && wasm)

package actions

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	book "mojra/domain/book/actions"
	commonactions "mojra/domain/common/actions"
	commonstate "mojra/domain/common/state"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/library/state"
	"mojra/domain/library/views"
	"mojra/engine"
	"mojra/engine/osbridge"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

const (
	PendingRemoveBook           = "RemoveBook"
	PendingUnpackPlaybookStudio = "UnpackPlaybookForStudio"
	modeValFile                 = "file"
	extFountain                 = ".fountain"
	extFDX                      = ".fdx"
	extPlaybook                 = ".playbook"
	formKeyBook                 = "book"
	formKeyView                 = "view"
	formKeyTargetView           = "target_view"
	formKeyMode                 = "mode"
	formKeyKind                 = "kind"
	formKeyMediaKind            = "media_kind"
	formKeyPath                 = "path"
	booksEmbedDir               = "books"
	booksSlashPrefix            = "books/"
)

type ActionHandler func(s *state.LibraryState, req *http.Request) []transport.ActionFrame

func libraryFromUI(ui *commonstate.UIState) *state.LibraryState {
	if ui != nil && ui.Hooks.LibraryState != nil {
		if ls, ok := ui.Hooks.LibraryState.(*state.LibraryState); ok {
			return ls
		}
	}
	return nil
}

func registerActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		fn := handler
		commonactions.RegisterAction(actionName, false, false, func(ui *commonstate.UIState, req *http.Request) []transport.ActionFrame {
			ls := libraryFromUI(ui)
			if ls == nil {
				return nil
			}
			return fn(ls, req)
		})
	}
	return true
}

func registerStandaloneActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		fn := handler
		commonactions.RegisterAction(actionName, true, true, func(ui *commonstate.UIState, req *http.Request) []transport.ActionFrame {
			ls := libraryFromUI(ui)
			if ls == nil {
				return nil
			}
			return fn(ls, req)
		})
	}
	return true
}

func tr(s *state.LibraryState, key engine.TrKey) string {
	return commonviews.Tr(s.UIState, key)
}

func trf(s *state.LibraryState, key engine.TrKey, args ...any) string {
	return commonviews.Trf(s.UIState, key, args...)
}

func frameLibraryApp(s *state.LibraryState, toast string) []transport.ActionFrame {
	s.Overlays.Toast = toast
	if s.Confirm.IsOpen() {
		return frameLibraryModals(s, toast)
	}
	if s.Hooks.RenderAppFrames != nil {
		return s.Hooks.RenderAppFrames()
	}
	return []transport.ActionFrame{
		{Target: commonstate.TargetWorkspace, Chunks: []templates.TemplateChunk{views.LibraryView(s)}},
	}
}

func frameLibraryAppResetCanvas(s *state.LibraryState, toast string) []transport.ActionFrame {
	s.Overlays.ResetCanvas = true
	return frameLibraryApp(s, toast)
}

func frameLibraryModals(s *state.LibraryState, toast string) []transport.ActionFrame {
	s.Overlays.Toast = toast
	if s.Hooks.RenderModalsFrame != nil {
		return []transport.ActionFrame{s.Hooks.RenderModalsFrame()}
	}
	return []transport.ActionFrame{
		{Target: commonstate.TargetModalsRoot, Chunks: []templates.TemplateChunk{commonviews.ConfirmView(s.UIState)}},
	}
}

var _ = registerActions(map[string]ActionHandler{
	"OpenBook":             openBook,
	"BrowseAddBookMedia":   browseAddBookMedia,
	"AddBookMediaFromDisk": addBookMediaFromDisk,
	"RemoveBook":           removeBookFromForm,
	"ExtractEmbeddedBook":  extractEmbeddedBookAction,
})

func pickNativeBookPathUnlocked(s *state.LibraryState, chooseDir bool) string {
	var picked string
	s.WithUnlocked(func() {
		picked = osbridge.PickNativeBookPath(chooseDir)
	})
	return picked
}

func pickDiskBookPath(s *state.LibraryState, req *http.Request) string {
	mode := strings.ToLower(strings.TrimSpace(req.FormValue(formKeyMode)))
	return pickNativeBookPathUnlocked(s, mode != modeValFile)
}

func isScreenplayImportFile(rawPath string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(rawPath)))
	return ext == extFountain || ext == extFDX
}

func openBook(s *state.LibraryState, req *http.Request) []transport.ActionFrame {
	bookID := book.SanitizeBookID(req.FormValue(formKeyBook))
	targetView := strings.TrimSpace(req.FormValue(formKeyView))
	if targetView == "" {
		targetView = strings.TrimSpace(req.FormValue(formKeyTargetView))
	}
	if targetView != string(commonstate.ViewEditor) {
		targetView = string(commonstate.ViewPlayer)
	}
	toast := ""
	if s.Hooks.OpenBookInView != nil {
		toast = s.Hooks.OpenBookInView(bookID, commonstate.ViewMode(targetView))
	}
	return frameLibraryAppResetCanvas(s, toast)
}

func ImportScreenplayFromDiskFile(s *state.LibraryState, rawPath string, openInStudio bool) string {
	expanded, err := book.ExpandLocalPath(rawPath)
	if err != nil {
		return err.Error()
	}
	destDir := book.ResolveNewBookDir(s.Settings)
	if destDir == "" {
		destDir = s.Config.BooksDir
	}
	imported, err := book.ImportScreenplayFileToDir(expanded, destDir)
	if err != nil {
		return err.Error()
	}
	if filepath.Clean(filepath.Dir(imported.RootPath)) != filepath.Clean(s.Config.BooksDir) {
		s.Settings.AddExternalBook(imported.RootPath)
		imported.IsExternal = true
	}
	s.Library.Put(imported)
	s.Overlays.AddBook = false
	s.Overlays.StudioProjects = false
	if openInStudio {
		if s.Hooks.OpenBookInView != nil {
			return s.Hooks.OpenBookInView(imported.ID, commonstate.ViewEditor)
		}
		return ""
	}
	if s.Hooks.OnBookCreated != nil {
		s.Hooks.OnBookCreated(imported, string(commonstate.ViewLibrary))
	}
	s.SaveSettings()
	return tr(s, i18n.TrImportedScreenplayPrefix) + imported.Title
}

func browseAddBookMedia(s *state.LibraryState, req *http.Request) []transport.ActionFrame {
	if s.Hooks.BrowseAddBookMedia != nil {
		if frames, ok := s.Hooks.BrowseAddBookMedia(req.URL.RawQuery); ok {
			_ = req
			return frames
		}
	}
	return frameLibraryModals(s, "")
}

func addBookMediaFromDisk(s *state.LibraryState, req *http.Request) []transport.ActionFrame {
	if s.Hooks.AddBookMediaFromDisk != nil {
		kind := strings.ToLower(strings.TrimSpace(req.FormValue(formKeyKind)))
		if kind == "" {
			kind = strings.ToLower(strings.TrimSpace(req.FormValue(formKeyMediaKind)))
		}
		if frames, ok := s.Hooks.AddBookMediaFromDisk(kind, strings.TrimSpace(req.FormValue(formKeyPath))); ok {
			return frames
		}
	}
	return frameLibraryModals(s, "")
}

func removeBookFromForm(s *state.LibraryState, req *http.Request) []transport.ActionFrame {
	bookID := book.SanitizeBookID(req.FormValue(formKeyBook))
	toast := ""
	if s.Hooks.RemoveBookByID != nil {
		toast, _ = s.Hooks.RemoveBookByID(bookID)
	}
	return frameLibraryApp(s, toast)
}

// RemoveBookByID removes or unlinks a book by ID from LibraryState and updates settings.
func RemoveBookByID(s *state.LibraryState, bookID string) string {
	if bookID == "" {
		return ""
	}
	b := s.Library.Get(bookID)
	if b == nil {
		return tr(s, i18n.TrBookNotFound)
	}
	msg := ""
	if b.IsExternal {
		s.Settings.RemoveExternalBook(b.RootPath)
		s.SaveSettings()
		msg = tr(s, i18n.TrUnlinkedBookPrefix) + b.Title
	} else if b.IsPacked {
		_ = os.Remove(b.RootPath)
		msg = tr(s, i18n.TrRemovedPackedBookPrefix) + b.Title
	} else {
		_ = os.RemoveAll(b.RootPath)
		_ = os.Remove(filepath.Join(s.Config.BooksDir, b.ID+extPlaybook))
		msg = tr(s, i18n.TrRemovedBookPrefix) + b.Title
	}
	s.Library.Remove(b.ID)
	return msg
}

func extractEmbeddedBookAction(s *state.LibraryState, _ *http.Request) []transport.ActionFrame {
	toast := ""
	if s.Config.DevMode && !s.Config.StandaloneMode {
		toast = extractEmbeddedDefaultBook(s)
	}
	return frameLibraryApp(s, toast)
}

func extractEmbeddedDefaultBook(s *state.LibraryState) string {
	if s.Config.EmbeddedFS == nil {
		return tr(s, i18n.TrNoEmbeddedBookFS)
	}
	entries, err := fs.ReadDir(s.Config.EmbeddedFS, booksEmbedDir)
	if err != nil {
		return tr(s, i18n.TrEmbeddedBookNotFound)
	}
	embeddedID := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candPrefix := booksSlashPrefix + e.Name()
		if _, statErr := fs.Stat(s.Config.EmbeddedFS, candPrefix+"/"+book.BookDataFileName); statErr == nil {
			embeddedID = e.Name()
			break
		}
	}
	if embeddedID == "" {
		return tr(s, i18n.TrEmbeddedBookNotFound)
	}
	embedPrefix := booksSlashPrefix + embeddedID

	targetBooksDir := s.Config.BooksDir
	if !book.IsTempDir(s.Config.RootDir) || book.IsInsideRepoDir(s.Config.RootDir) {
		targetBooksDir = book.DefaultProjectsDir()
	}
	sampleDir := filepath.Join(targetBooksDir, embeddedID)
	_ = os.Remove(filepath.Join(targetBooksDir, embeddedID+extPlaybook))
	_ = os.RemoveAll(sampleDir)

	var extractErr error
	_ = fs.WalkDir(s.Config.EmbeddedFS, embedPrefix, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			extractErr = walkErr
			return walkErr
		}
		rel := strings.TrimPrefix(path, embedPrefix)
		rel = strings.TrimPrefix(rel, "/")
		target := sampleDir
		if rel != "" {
			target = filepath.Join(sampleDir, filepath.FromSlash(rel))
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, readErr := fs.ReadFile(s.Config.EmbeddedFS, path)
		if readErr != nil {
			extractErr = readErr
			return readErr
		}
		if mkErr := os.MkdirAll(filepath.Dir(target), 0755); mkErr != nil {
			extractErr = mkErr
			return mkErr
		}
		if wrErr := os.WriteFile(target, data, 0644); wrErr != nil {
			extractErr = wrErr
			return wrErr
		}
		return nil
	})
	if extractErr != nil {
		return tr(s, i18n.TrExtractBookErrPrefix) + extractErr.Error()
	}

	isExt := filepath.Clean(targetBooksDir) != filepath.Clean(s.Config.BooksDir)
	if isExt {
		s.Settings.AddExternalBook(sampleDir)
	}
	s.Overlays.QR = false
	if b, err := book.LoadBookFromPath(sampleDir); err == nil && b != nil {
		b.IsExternal = isExt
		s.Library.Put(b)
		if s.Hooks.OnBookCreated != nil {
			s.Hooks.OnBookCreated(b, string(commonstate.ViewLibrary))
		}
		s.SaveSettings()
		return tr(s, i18n.TrExtractedBookPrefix) + b.Title
	}
	return tr(s, i18n.TrExtractedBookGeneric)
}
