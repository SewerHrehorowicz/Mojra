//go:build !(js && wasm)

package actions

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/osbridge"
	"mojra/internal/state"
)

func pickNativeBookPathUnlocked(s *state.AppState, chooseDir bool) string {
	var picked string
	s.WithUnlocked(func() {
		picked = osbridge.PickNativeBookPath(chooseDir)
	})
	return picked
}

func pickDiskBookPath(s *state.AppState, r *http.Request) string {
	mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
	return pickNativeBookPathUnlocked(s, mode != "file")
}

func isScreenplayImportFile(rawPath string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(rawPath)))
	return ext == ".fountain" || ext == ".fdx"
}

func openBook(s *state.AppState, r *http.Request) string {
	bookID := book.SanitizeBookID(r.FormValue("book"))
	targetView := strings.TrimSpace(r.FormValue("view"))
	if targetView == "" {
		targetView = strings.TrimSpace(r.FormValue("target_view"))
	}
	if targetView != state.ViewEditor.String() {
		targetView = state.ViewPlayer.String()
	}
	if b := s.Library.Get(bookID); b != nil {
		if targetView == state.ViewEditor.String() {
			return openBookInStudio(s, b)
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
	return ""
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
	s.Modals.AddBook = false
	s.Modals.StudioProjects = false
	if openInStudio {
		return openBookInStudio(s, imported)
	}
	s.Player.SetBook(s.Config.SavesDir, s.Settings, imported)
	_ = s.SaveSettings()
	return "Zaimportowano scenariusz: " + imported.Title
}

func prepareBookMediaAction(s *state.AppState, r *http.Request) (string, string, string, bool) {
	if isHistoryPreview(s) {
		return "", "", tr(s, i18n.TrHistoryPreviewLocked), false
	}
	syncSceneTextSettingsOpen(s, r)
	if s.Editor.ActiveBook == nil || s.Editor.ActiveBook.IsPacked {
		return "", "", "", false
	}
	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	if kind == "" {
		kind = strings.ToLower(strings.TrimSpace(r.FormValue("media_kind")))
	}
	if !s.Editor.ActiveBook.Narrated && (kind == "audio" || kind == "voice") {
		return "", "", "Książka nie posiada narracji (narrated: false) — nagrania głosowe są wyłączone", false
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	return kind, scID, "", true
}

func browseAddBookMedia(s *state.AppState, r *http.Request) string {
	kind, scID, errToast, ok := prepareBookMediaAction(s, r)
	if !ok {
		return errToast
	}
	if rawPath := strings.TrimSpace(r.FormValue("path")); rawPath != "" {
		return addBookMediaFromDiskPaths(s, []string{rawPath}, kind, scID)
	}
	var picked []string
	s.WithUnlocked(func() {
		picked = osbridge.PickNativeMediaPaths(kind)
	})
	if len(picked) == 0 {
		r.Form.Set("silent", "true")
		return ""
	}
	return addBookMediaFromDiskPaths(s, picked, kind, scID)
}

func addBookMediaFromDisk(s *state.AppState, r *http.Request) string {
	kind, scID, errToast, ok := prepareBookMediaAction(s, r)
	if !ok {
		return errToast
	}
	rawPath := strings.TrimSpace(r.FormValue("path"))
	return addBookMediaFromDiskPaths(s, []string{rawPath}, kind, scID)
}

func addBookMediaFromDiskPaths(s *state.AppState, paths []string, kind, sceneID string) string {
	if isHistoryPreview(s) {
		return tr(s, i18n.TrHistoryPreviewLocked)
	}
	b := s.Editor.ActiveBook
	if b == nil {
		return "Brak aktywnej książki"
	}
	if b.IsPacked {
		return "Nie można edytować spakowanego archiwum .playbook bezpośrednio"
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
			if !assignedImg && (kind == "image" || kind == "img" || kind == "") && book.IsRasterImagePath(rel) {
				target.ExplicitImage = rel
				assignedImg = true
			}
			if !assignedAud && b.Narrated && (kind == "audio" || kind == "voice" || kind == "") && book.IsAudioPath(rel) {
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
		msg := "Dodano plik multimedialny: " + imported[0]
		s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, "scene", "", s.Editor.SelectedSceneID)
		return msg
	}
	msg := fmt.Sprintf("Dodano %d plików multimedialnych do książki", len(imported))
	s.Editor.History.RecordStep(s.Editor.ActiveBook, msg, "scene", "", s.Editor.SelectedSceneID)
	return msg
}

func removeBookFromForm(s *state.AppState, r *http.Request) string {
	return removeBookByID(s, book.SanitizeBookID(r.FormValue("book")))
}

func removeBookByID(s *state.AppState, bookID string) string {
	if bookID == "" {
		return ""
	}
	b := s.Library.Get(bookID)
	if b == nil {
		return "Nie znaleziono książki"
	}

	msg := ""
	if b.IsExternal {
		s.Settings.RemoveExternalBook(b.RootPath)
		_ = s.SaveSettings()
		msg = "Odpięto książkę z biblioteki (pliki na dysku zachowane): " + b.Title
	} else if b.IsPacked {
		_ = os.Remove(b.RootPath)
		msg = "Usunięto pakiet książki: " + b.Title
	} else {
		_ = os.RemoveAll(b.RootPath)
		_ = os.Remove(filepath.Join(s.Config.BooksDir, b.ID+".playbook"))
		msg = "Usunięto książkę: " + b.Title
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

func extractEmbeddedBookAction(s *state.AppState, _ *http.Request) string {
	if s.Config.DevMode && !s.Config.StandaloneMode {
		return extractEmbeddedDefaultBook(s)
	}
	return ""
}

func extractEmbeddedDefaultBook(s *state.AppState) string {
	if s.Config.EmbeddedFS == nil {
		return "Brak wbudowanego systemu plików z książką"
	}
	entries, err := fs.ReadDir(s.Config.EmbeddedFS, "books")
	if err != nil {
		return "Wbudowana książka nie została znaleziona w binarium"
	}
	embeddedID := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candPrefix := "books/" + e.Name()
		if _, statErr := fs.Stat(s.Config.EmbeddedFS, candPrefix+"/"+book.BookDataFileName); statErr == nil {
			embeddedID = e.Name()
			break
		}
	}
	if embeddedID == "" {
		return "Wbudowana książka nie została znaleziona w binarium"
	}
	embedPrefix := "books/" + embeddedID

	targetBooksDir := s.Config.BooksDir
	if !book.IsTempDir(s.Config.RootDir) || book.IsInsideRepoDir(s.Config.RootDir) {
		targetBooksDir = book.DefaultProjectsDir()
	}
	sampleDir := filepath.Join(targetBooksDir, embeddedID)
	_ = os.Remove(filepath.Join(targetBooksDir, embeddedID+".playbook"))
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
		return "Błąd wypakowywania książki: " + extractErr.Error()
	}

	isExt := filepath.Clean(targetBooksDir) != filepath.Clean(s.Config.BooksDir)
	if isExt {
		s.Settings.AddExternalBook(sampleDir)
	}
	s.Modals.QR = false
	if b, err := book.LoadBookFromPath(sampleDir); err == nil && b != nil {
		b.IsExternal = isExt
		s.Library.Put(b)
		s.Editor.SetBook(b, s.Settings)
		if s.Player.Book == nil {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, b)
		}
		_ = s.SaveSettings()
		return "Wypakowano wbudowaną książkę: " + b.Title
	}
	return "Wypakowano wbudowaną książkę do katalogu books/"
}
