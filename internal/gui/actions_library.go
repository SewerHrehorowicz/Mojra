package gui

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"storyteller/internal/book"
)

func (s *Server) openBook(r *http.Request) string {
	bookID := book.SanitizeBookID(r.FormValue("book"))
	targetView := strings.TrimSpace(r.FormValue("view"))
	if targetView != "editor" {
		targetView = "player"
	}
	if b, err := s.findBookByID(bookID); err == nil && b != nil {
		s.setActiveBook(b)
		s.Settings.ActiveView = targetView
		s.closeAllModals()
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	}
	return ""
}

func (s *Server) browseAddBookFromDisk(r *http.Request) string {
	mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
	chooseDir := mode != "file"
	s.mu.Unlock()
	picked := PickNativeBookPath(chooseDir)
	s.mu.Lock()
	if picked != "" {
		return s.addBookFromDiskPath(picked)
	}
	return ""
}

func (s *Server) addBookFromDisk(r *http.Request) string {
	rawPath := strings.TrimSpace(r.FormValue("path"))
	lowerPath := strings.ToLower(rawPath)
	if strings.HasPrefix(lowerPath, "http://") || strings.HasPrefix(lowerPath, "https://") {
		return s.addBookFromURL(rawPath)
	}
	return s.addBookFromDiskPath(rawPath)
}

func (s *Server) addBookFromURLAction(r *http.Request) string {
	rawURL := strings.TrimSpace(r.FormValue("url"))
	return s.addBookFromURL(rawURL)
}

func (s *Server) browseAddBookMedia(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	if r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
		if kind == "" {
			kind = strings.ToLower(strings.TrimSpace(r.FormValue("media_kind")))
		}
		if !s.ActiveBook.Narrated && (kind == "audio" || kind == "voice") {
			return "Książka nie posiada narracji (narrated: false) — nagrania głosowe są wyłączone"
		}
		scID := strings.TrimSpace(r.FormValue("scene"))
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue("scene_id"))
		}
		if rawPath := strings.TrimSpace(r.FormValue("path")); rawPath != "" {
			return s.addBookMediaFromDiskPaths([]string{rawPath}, kind, scID)
		}
		s.mu.Unlock()
		picked := PickNativeMediaPaths(kind)
		s.mu.Lock()
		if len(picked) == 0 {
			r.Form.Set("silent", "true")
			return ""
		}
		return s.addBookMediaFromDiskPaths(picked, kind, scID)
	}
	return ""
}

func (s *Server) addBookMediaFromDisk(r *http.Request) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	if r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
		if kind == "" {
			kind = strings.ToLower(strings.TrimSpace(r.FormValue("media_kind")))
		}
		if !s.ActiveBook.Narrated && (kind == "audio" || kind == "voice") {
			return "Książka nie posiada narracji (narrated: false) — nagrania głosowe są wyłączone"
		}
		scID := strings.TrimSpace(r.FormValue("scene"))
		if scID == "" {
			scID = strings.TrimSpace(r.FormValue("scene_id"))
		}
		rawPath := strings.TrimSpace(r.FormValue("path"))
		return s.addBookMediaFromDiskPaths([]string{rawPath}, kind, scID)
	}
	return ""
}

func (s *Server) browseNewBookFolder(r *http.Request) string {
	s.NewBookDraftTitle = r.FormValue("title")
	s.NewBookDraftAuthor = r.FormValue("author")
	if m := strings.TrimSpace(r.FormValue("mode")); m != "" {
		s.NewBookDraftMode = book.NormalizeBookMode(m)
	}
	if std := strings.TrimSpace(r.FormValue("standard")); std != "" {
		s.NewBookDraftStandard = book.NormalizeScriptStandard(std)
	}
	rawNarr := strings.ToLower(strings.TrimSpace(r.FormValue("narrated")))
	s.NewBookDraftNarrated = rawNarr == "true" || rawNarr == "on" || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(r.FormValue("imageless")))
	s.NewBookDraftImageless = rawImg == "true" || rawImg == "on" || rawImg == "1"
	if curPath := strings.TrimSpace(r.FormValue("save_path")); curPath != "" {
		if expanded, err := book.ExpandLocalPath(curPath); err == nil && expanded != "" {
			s.NewBookDraftDir = expanded
		}
	}

	s.mu.Unlock()
	picked := PickNativeBookPath(true)
	s.mu.Lock()

	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.NewBookDraftDir = picked
		if s.Settings != nil {
			s.Settings.NewBookDir = picked
			_ = s.saveAppSettings()
		}
	}
	return ""
}

func (s *Server) createNewBook(r *http.Request) string {
	title := r.FormValue("title")
	author := r.FormValue("author")
	mode := r.FormValue("mode")
	standard := r.FormValue("standard")
	rawNarr := strings.ToLower(strings.TrimSpace(r.FormValue("narrated")))
	narrated := rawNarr == "true" || rawNarr == "on" || rawNarr == "1"
	rawImg := strings.ToLower(strings.TrimSpace(r.FormValue("imageless")))
	imageless := rawImg == "true" || rawImg == "on" || rawImg == "1"

	targetDir := s.defaultNewBookDir()
	rawSavePath := strings.TrimSpace(r.FormValue("save_path"))
	if rawSavePath != "" {
		expanded, err := book.ExpandLocalPath(rawSavePath)
		if err != nil {
			return err.Error()
		}
		targetDir = expanded
	}
	if targetDir == "" {
		targetDir = s.BooksDir
	}

	if created, err := book.CreateNewBookWithMode(targetDir, title, author, mode, standard, narrated, imageless); err == nil && created != nil {
		s.NewBookModalOpen = false
		s.NewBookDraftTitle = ""
		s.NewBookDraftAuthor = ""
		s.NewBookDraftMode = ""
		s.NewBookDraftStandard = ""
		s.NewBookDraftNarrated = false
		s.NewBookDraftImageless = false
		s.NewBookDraftDir = ""

		if rawSavePath != "" && s.Settings != nil {
			s.Settings.NewBookDir = filepath.Dir(created.RootPath)
		}
		if filepath.Clean(filepath.Dir(created.RootPath)) != filepath.Clean(s.BooksDir) {
			s.Settings.AddExternalBook(created.RootPath)
			created.IsExternal = true
		}
		s.setActiveBook(created)
		s.Settings.ActiveView = "editor"
		_ = s.saveAppSettings()
		if created.IsScreenplay() {
			return "Utworzono nowy scenariusz: " + created.Title
		}
		return "Utworzono nową książkę: " + created.Title
	} else if err != nil {
		return err.Error()
	}
	return ""
}

func (s *Server) promptRemoveBook(r *http.Request) string {
	reqBookID := book.SanitizeBookID(r.FormValue("book"))
	if b, err := s.findBookByID(reqBookID); err == nil && b != nil {
		if b.IsExternal {
			s.ConfirmTitle = "Odepnij książkę z biblioteki"
			s.ConfirmMessage = fmt.Sprintf("Czy chcesz usunąć książkę „%s” z listy w bibliotece? Oryginalne pliki na dysku (%s) nie zostaną usunięte.", b.Title, b.RootPath)
		} else {
			s.ConfirmTitle = "Usuń książkę z biblioteki"
			s.ConfirmMessage = fmt.Sprintf("Czy na pewno chcesz usunąć książkę „%s” z katalogu biblioteki?", b.Title)
		}
		s.ConfirmPendingAction = "RemoveBook"
		s.ConfirmPendingParam = b.ID
	}
	return ""
}

func (s *Server) removeBookFromForm(r *http.Request) string {
	return s.removeBookByID(book.SanitizeBookID(r.FormValue("book")))
}

func (s *Server) packActiveBook(r *http.Request) string {
	if s.ActiveBook != nil && !s.ActiveBook.IsPacked {
		exportDir, errMsg, ok := s.resolveOrPickBookExportDir(r)
		if !ok {
			return errMsg
		}
		if err := os.MkdirAll(exportDir, 0755); err != nil {
			return err.Error()
		}
		outZip := filepath.Join(exportDir, s.ActiveBook.ID+".stbook")
		if err := book.PackBookToZip(s.ActiveBook.RootPath, outZip); err == nil {
			s.revealExportedFileInOS(outZip)
			return "Spakowano archiwum: " + outZip
		} else {
			return err.Error()
		}
	}
	return ""
}

func (s *Server) extractEmbeddedBookAction(_ *http.Request) string {
	if s.DevMode && !s.StandaloneMode {
		return s.extractEmbeddedDefaultBook()
	}
	return ""
}

func (s *Server) addBookFromDiskPath(rawPath string) string {
	resolvedPaths, err := book.ResolveDiskBookPaths(rawPath)
	if err != nil {
		return err.Error()
	}

	cleanBooksDir := filepath.Clean(s.BooksDir)
	for _, p := range resolvedPaths {
		if filepath.Clean(filepath.Dir(p)) != cleanBooksDir {
			s.Settings.AddExternalBook(p)
		}
	}
	s.AddBookModalOpen = false

	books, _ := s.loadBooks()
	var firstAdded *book.Book
	for _, b := range books {
		if filepath.Clean(b.RootPath) == filepath.Clean(resolvedPaths[0]) {
			firstAdded = b
			break
		}
	}
	if firstAdded != nil {
		s.setActiveBook(firstAdded)
	}
	_ = book.SaveAppSettings(s.SavesDir, s.Settings)

	if len(resolvedPaths) > 1 {
		return fmt.Sprintf("Dodano %d książek z dysku (bez kopiowania)", len(resolvedPaths))
	}
	if firstAdded != nil {
		return "Dodano z dysku (bez kopiowania): " + firstAdded.Title
	}
	return "Dodano książkę z dysku"
}

func (s *Server) addBookFromURL(rawURL string) string {
	downloaded, err := book.DownloadBookFromURL(s.BooksDir, rawURL)
	if err != nil {
		return err.Error()
	}
	s.AddBookModalOpen = false
	if reloaded, rErr := s.findBookByID(downloaded.ID); rErr == nil && reloaded != nil {
		s.setActiveBook(reloaded)
	} else {
		s.setActiveBook(downloaded)
	}
	_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	return "Pobrano książkę z URL: " + downloaded.Title
}

func (s *Server) addBookMediaFromDiskPaths(paths []string, kind, sceneID string) string {
	if s.isHistoryPreview() {
		return historyPreviewLockedToast
	}
	if s.ActiveBook == nil {
		return "Brak aktywnej książki"
	}
	if s.ActiveBook.IsPacked {
		return "Nie można edytować spakowanego archiwum .stbook bezpośrednio"
	}
	imported, err := book.ImportBookMediaFromDisk(s.ActiveBook, paths, kind)
	if err != nil {
		return err.Error()
	}
	if len(imported) == 0 {
		return ""
	}

	if sceneID == "" {
		sceneID = s.SelectedSceneID
	}
	s.ensureHistoryBaselineBeforeMutation()
	if target := s.findScene(sceneID); target != nil {
		assignedImg := false
		assignedAud := false
		for _, rel := range imported {
			if !assignedImg && (kind == "image" || kind == "img" || kind == "") && book.IsRasterImagePath(rel) {
				target.ExplicitImage = rel
				assignedImg = true
			}
			if !assignedAud && s.ActiveBook.Narrated && (kind == "audio" || kind == "voice" || kind == "") && book.IsAudioPath(rel) {
				target.ExplicitVoice = rel
				assignedAud = true
			}
		}
		s.SelectedSceneID = target.ID
	}

	book.FinalizeBook(s.ActiveBook)
	if err := book.SaveBookScenesToDisk(s.ActiveBook); err != nil {
		return err.Error()
	}
	s.reloadActiveBook()
	if len(imported) == 1 {
		msg := "Dodano plik multimedialny: " + imported[0]
		s.recordHistoryAfterMutation(msg, "scene", "")
		return msg
	}
	msg := fmt.Sprintf("Dodano %d plików multimedialnych do książki", len(imported))
	s.recordHistoryAfterMutation(msg, "scene", "")
	return msg
}

func (s *Server) removeBookByID(bookID string) string {
	if bookID == "" {
		return ""
	}
	b, err := s.findBookByID(bookID)
	if err != nil || b == nil {
		return "Nie znaleziono książki"
	}

	msg := ""
	if b.IsExternal {
		s.Settings.RemoveExternalBook(b.RootPath)
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)
		msg = "Odpięto książkę z biblioteki (pliki na dysku zachowane): " + b.Title
	} else if b.IsPacked {
		_ = os.Remove(b.RootPath)
		msg = "Usunięto pakiet książki: " + b.Title
	} else {
		_ = os.RemoveAll(b.RootPath)
		_ = os.Remove(filepath.Join(s.BooksDir, b.ID+".stbook"))
		msg = "Usunięto książkę: " + b.Title
	}

	if s.ActiveBook != nil && s.ActiveBook.ID == b.ID {
		s.ActiveBook = nil
		s.Save = nil
		if remaining, err := s.loadBooks(); err == nil && len(remaining) > 0 {
			s.setActiveBook(remaining[0])
		}
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	}
	return msg
}

func (s *Server) extractEmbeddedDefaultBook() string {
	if s.EmbeddedFS == nil {
		return "Brak wbudowanego systemu plików z książką"
	}
	const embedPrefix = "books/nie-boj-sie-policji-rzecznej"
	if _, err := fs.Stat(s.EmbeddedFS, embedPrefix+"/data.txt"); err != nil {
		return "Wbudowana książka nie została znaleziona w binarium"
	}

	sampleDir := filepath.Join(s.BooksDir, "nie-boj-sie-policji-rzecznej")
	_ = os.Remove(filepath.Join(s.BooksDir, "nie-boj-sie-policji-rzecznej.stbook"))
	_ = os.RemoveAll(sampleDir)

	var extractErr error
	_ = fs.WalkDir(s.EmbeddedFS, embedPrefix, func(path string, d fs.DirEntry, walkErr error) error {
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
		data, readErr := fs.ReadFile(s.EmbeddedFS, path)
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

	s.QRModalOpen = false
	if b, err := s.findBookByID("nie-boj-sie-policji-rzecznej"); err == nil && b != nil {
		s.setActiveBook(b)
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)
		return "Wypakowano wbudowaną książkę: " + b.Title
	}
	return "Wypakowano wbudowaną książkę do katalogu books/"
}

func (s *Server) findBookByID(bookID string) (*book.Book, error) {
	books, err := s.loadBooks()
	if err != nil {
		return nil, err
	}
	for _, b := range books {
		if b.ID == bookID {
			return b, nil
		}
	}
	return nil, fmt.Errorf("book %q not found", bookID)
}
