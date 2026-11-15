//go:build !(js && wasm)

package actions

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine"
	"mojra/engine/osbridge"
)

const (
	modeValFile       = "file"
	extFountain       = ".fountain"
	extFDX            = ".fdx"
	extPlaybook       = ".playbook"
	formKeyTargetView = "target_view"
	formKeyMediaKind  = "media_kind"
	mediaKindAudio    = "audio"
	mediaKindVoice    = "voice"
	mediaKindImage    = "image"
	mediaKindImg      = "img"
	booksEmbedDir     = "books"
	booksSlashPrefix  = "books/"
)

var _ = registerActions(map[string]ActionHandler{
	"OpenBook":             openBook,
	"BrowseAddBookMedia":   browseAddBookMedia,
	"AddBookMediaFromDisk": addBookMediaFromDisk,
	"RemoveBook":           removeBookFromForm,
	"ExtractEmbeddedBook":  extractEmbeddedBookAction,
})

func pickNativeBookPathUnlocked(s *state.AppState, chooseDir bool) string {
	var picked string
	s.WithUnlocked(func() {
		picked = osbridge.PickNativeBookPath(chooseDir)
	})
	return picked
}

func pickDiskBookPath(s *state.AppState, r *http.Request) string {
	mode := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyMode)))
	return pickNativeBookPathUnlocked(s, mode != modeValFile)
}

func isScreenplayImportFile(rawPath string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(rawPath)))
	return ext == extFountain || ext == extFDX
}

func openBook(s *state.AppState, r *http.Request) []engine.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	bookID := book.SanitizeBookID(r.FormValue(formKeyBook))
	targetView := strings.TrimSpace(r.FormValue(formKeyView))
	if targetView == "" {
		targetView = strings.TrimSpace(r.FormValue(formKeyTargetView))
	}
	if targetView != state.ViewEditor.String() {
		targetView = state.ViewPlayer.String()
	}
	toast := ""
	if b := s.Library.Get(bookID); b != nil {
		if targetView == state.ViewEditor.String() {
			toast = openBookInStudio(s, b)
			return frameAppResetCanvas(s, toast)
		}
		if s.Settings != nil && s.Settings.ActiveView == state.ViewEditor.String() && s.Editor.HasActiveBook() {
			s.Editor.SaveBookUIState(s.Settings)
		}
		if isHistoryPreview(s) {
			exitHistoryPreview(s)
		}
		s.Player.SetBook(s.Config.SavesDir, s.Settings, b)
		s.Settings.ActiveView = state.ViewPlayer.String()
		s.CloseAllModals()
		s.Editor.SyncToSettings(s.Settings)
		_ = s.SaveSettings()
	}
	return frameAppResetCanvas(s, toast)
}

func openBookInStudio(s *state.AppState, b *book.Book) string {
	if b == nil {
		return ""
	}
	if b.IsPacked {
		return promptUnpackPlaybookForStudio(s, b)
	}
	s.Editor.SetBook(b, s.Settings)
	if s.Player.Book == nil {
		s.Player.SetBook(s.Config.SavesDir, s.Settings, b)
	}
	s.Settings.ActiveView = state.ViewEditor.String()
	s.CloseAllModals()
	s.Editor.RestoreBookUIState(s.Settings, true)
	s.Editor.SyncToSettings(s.Settings)
	_ = s.SaveSettings()
	if strings.TrimSpace(b.License) != "" {
		return trf(s, i18n.TrOpenedInStudioLicenseFmt, b.Title, b.License)
	}
	return tr(s, i18n.TrOpenedInStudioPrefix) + b.Title
}

func importScreenplayFromDiskFile(s *state.AppState, rawPath string, openInStudio bool) string {
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
		return openBookInStudio(s, imported)
	}
	s.Player.SetBook(s.Config.SavesDir, s.Settings, imported)
	_ = s.SaveSettings()
	return tr(s, i18n.TrImportedScreenplayPrefix) + imported.Title
}

func prepareBookMediaAction(s *state.AppState, r *http.Request) (string, string, string, bool) {
	if isHistoryPreview(s) {
		return "", "", tr(s, i18n.TrHistoryPreviewLocked), false
	}
	syncSceneTextSettingsOpen(s, r)
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return "", "", "", false
	}
	kind := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyKind)))
	if kind == "" {
		kind = strings.ToLower(strings.TrimSpace(r.FormValue(formKeyMediaKind)))
	}
	if !s.Editor.ActiveBook.Narrated && (kind == mediaKindAudio || kind == mediaKindVoice) {
		return "", "", tr(s, i18n.TrBookNoNarrationMedia), false
	}
	scID := strings.TrimSpace(r.FormValue(formKeyScene))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue(formKeySceneID))
	}
	return kind, scID, "", true
}

func browseAddBookMedia(s *state.AppState, r *http.Request) []engine.ActionFrame {
	hadFormScene := flushFormSceneIfPresent(s, r)
	kind, scID, errToast, ok := prepareBookMediaAction(s, r)
	if !ok {
		return frameSceneForm(s, errToast, hadFormScene, false)
	}
	if rawPath := strings.TrimSpace(r.FormValue(formKeyPath)); rawPath != "" {
		return frameSceneForm(s, addBookMediaFromDiskPaths(s, []string{rawPath}, kind, scID), hadFormScene, false)
	}
	var picked []string
	s.WithUnlocked(func() {
		picked = osbridge.PickNativeMediaPaths(kind)
	})
	if len(picked) == 0 {
		r.Form.Set(formFieldSilent, boolValTrue)
		return frameSceneForm(s, "", hadFormScene, false)
	}
	return frameSceneForm(s, addBookMediaFromDiskPaths(s, picked, kind, scID), hadFormScene, false)
}

func addBookMediaFromDisk(s *state.AppState, r *http.Request) []engine.ActionFrame {
	hadFormScene := flushFormSceneIfPresent(s, r)
	kind, scID, errToast, ok := prepareBookMediaAction(s, r)
	if !ok {
		return frameSceneForm(s, errToast, hadFormScene, false)
	}
	rawPath := strings.TrimSpace(r.FormValue(formKeyPath))
	return frameSceneForm(s, addBookMediaFromDiskPaths(s, []string{rawPath}, kind, scID), hadFormScene, false)
}

func addBookMediaFromDiskPaths(s *state.AppState, paths []string, kind, sceneID string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return tr(s, i18n.TrVoiceNoActiveBook)
	}
	if b.IsPacked {
		return tr(s, i18n.TrVoicePackedBookReadOnly)
	}
	imported, err := book.ImportBookMediaFromDisk(b, paths, kind)
	if err != nil {
		return err.Error()
	}
	if len(imported) == 0 {
		return ""
	}

	if sceneID == "" {
		sceneID = s.Editor.SelectedSceneID
	}
	s.Editor.History.EnsureBaseline(b, s.Editor.SelectedSceneID)
	if target := b.FindScene(sceneID); target != nil {
		assignedImg := false
		assignedAud := false
		for _, rel := range imported {
			if !assignedImg && (kind == mediaKindImage || kind == mediaKindImg || kind == "") && book.IsRasterImagePath(rel) {
				target.ExplicitImage = rel
				assignedImg = true
			}
			if !assignedAud && b.Narrated && (kind == mediaKindAudio || kind == mediaKindVoice || kind == "") && book.IsAudioPath(rel) {
				target.ExplicitVoice = rel
				assignedAud = true
			}
		}
		s.Editor.SelectedSceneID = target.ID
	}

	book.FinalizeBook(b)
	if err := book.SaveBookScenesToDisk(b); err != nil {
		return err.Error()
	}
	reloadActiveBook(s)
	if len(imported) == 1 {
		msg := tr(s, i18n.TrAddedMediaSinglePrefix) + imported[0]
		s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeScene, "", s.Editor.SelectedSceneID)
		return msg
	}
	msg := trf(s, i18n.TrAddedMediaMultiFmt, len(imported))
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, historyScopeScene, "", s.Editor.SelectedSceneID)
	return msg
}

func removeBookFromForm(s *state.AppState, r *http.Request) []engine.ActionFrame {
	return frameApp(s, removeBookByID(s, book.SanitizeBookID(r.FormValue(formKeyBook))))
}

func removeBookByID(s *state.AppState, bookID string) string {
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
		_ = s.SaveSettings()
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

	if s.Player.Book != nil && s.Player.Book.ID == b.ID {
		s.Player.SetBook(s.Config.SavesDir, s.Settings, nil)
		if len(s.Library.Books) > 0 {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, s.Library.Books[0])
		}
		_ = s.SaveSettings()
	}
	if s.Editor.ActiveBook != nil && s.Editor.ActiveBook.ID == b.ID {
		s.Editor.SetBook(nil, s.Settings)
		_ = s.SaveSettings()
	}
	return msg
}

func extractEmbeddedBookAction(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	toast := ""
	if s.Config.DevMode && !s.Config.StandaloneMode {
		toast = extractEmbeddedDefaultBook(s)
	}
	return frameApp(s, toast)
}

func extractEmbeddedDefaultBook(s *state.AppState) string {
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
		s.Editor.SetBook(b, s.Settings)
		if s.Player.Book == nil {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, b)
		}
		_ = s.SaveSettings()
		return tr(s, i18n.TrExtractedBookPrefix) + b.Title
	}
	return tr(s, i18n.TrExtractedBookGeneric)
}
