package gui

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mojra/internal/book"
)

func (s *Server) openBook(r *http.Request) string {
	bookID := book.SanitizeBookID(r.FormValue("book"))
	targetView := strings.TrimSpace(r.FormValue("view"))
	if targetView == "" {
		targetView = strings.TrimSpace(r.FormValue("target_view"))
	}
	if targetView != "editor" {
		targetView = "player"
	}
	if b, err := s.findBookByID(bookID); err == nil && b != nil {
		if targetView == "editor" {
			return s.openBookInStudio(b)
		}
		s.setPlayerBook(b)
		s.Settings.ActiveView = "player"
		s.closeAllModals()
		s.persistSettings()
	}
	return ""
}

func (s *Server) openBookInStudio(b *book.Book) string {
	if b == nil {
		return ""
	}
	if b.IsPacked {
		return s.promptUnpackPlaybookForStudio(b)
	}
	s.setActiveBook(b)
	s.Settings.ActiveView = "editor"
	s.closeAllModals()
	s.persistSettings()
	if strings.TrimSpace(b.License) != "" {
		return s.trf(TrOpenedInStudioLicenseFmt, b.Title, b.License)
	}
	return s.tr(TrOpenedInStudioPrefix) + b.Title
}

func (s *Server) promptUnpackPlaybookForStudio(b *book.Book) string {
	if b == nil {
		return ""
	}
	s.closeAllModals()
	s.Settings.ActiveView = "editor"
	s.persistSettings()
	author := strings.TrimSpace(b.Author)
	if author == "" {
		author = s.tr(TrUnknownAuthor)
	}
	s.ConfirmTitle = s.tr(TrConfirmPlaybookReadonlyTitle)
	s.ConfirmMessage = s.trf(
		TrConfirmPlaybookUnpackMsgFmt,
		b.Title,
		author,
		b.EffectiveLicense(),
	)
	s.ConfirmPendingAction = "UnpackPlaybookForStudio"
	s.ConfirmPendingParam = b.RootPath
	return ""
}

func (s *Server) unpackPlaybookWorkingCopyForStudio(archivePath string) string {
	cleanArchive := filepath.Clean(strings.TrimSpace(archivePath))
	if cleanArchive == "" || cleanArchive == "." {
		return "Brak ścieżki archiwum .playbook"
	}
	packedBook, err := book.LoadBookFromPath(cleanArchive)
	if err != nil || packedBook == nil {
		return "Nie udało się odczytać archiwum .playbook"
	}
	projectsDir := s.defaultNewBookDir()
	if projectsDir == "" {
		projectsDir = s.BooksDir
	}
	_ = os.MkdirAll(projectsDir, 0755)
	baseID := book.SanitizeBookID(packedBook.ID)
	if baseID == "" {
		baseID = "projekt"
	}
	targetDir := filepath.Join(projectsDir, baseID)
	if _, statErr := os.Stat(targetDir); statErr == nil {
		for seq := 1; seq <= 99; seq++ {
			candID := fmt.Sprintf("%s-kopia", baseID)
			if seq > 1 {
				candID = fmt.Sprintf("%s-kopia-%d", baseID, seq)
			}
			candDir := filepath.Join(projectsDir, candID)
			if _, cErr := os.Stat(candDir); os.IsNotExist(cErr) {
				baseID = candID
				targetDir = candDir
				break
			}
		}
	}
	if err := book.UnpackBookZip(cleanArchive, targetDir); err != nil {
		return "Błąd wypakowywania kopii roboczej: " + err.Error()
	}
	unpacked, err := book.LoadBookFromPath(targetDir)
	if err != nil || unpacked == nil {
		return "Błąd wczytywania wypakowanego projektu"
	}
	if filepath.Clean(filepath.Dir(unpacked.RootPath)) != filepath.Clean(s.BooksDir) {
		s.Settings.AddExternalBook(unpacked.RootPath)
		unpacked.IsExternal = true
	}
	s.setActiveBook(unpacked)
	s.Settings.ActiveView = "editor"
	s.closeAllModals()
	s.persistSettings()
	return fmt.Sprintf("Wypakowano kopię roboczą do Studio: %s (%s)", unpacked.Title, unpacked.EffectiveLicense())
}

func (s *Server) openStudioProjectAction(r *http.Request) string {
	bookID := book.SanitizeBookID(r.FormValue("book"))
	if b, err := s.findBookByID(bookID); err == nil && b != nil {
		return s.openBookInStudio(b)
	}
	return "Nie znaleziono projektu"
}

func (s *Server) pickDiskBookPath(r *http.Request) string {
	mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
	return s.pickNativeBookPathUnlocked(mode != "file")
}

func (s *Server) browseOpenStudioProject(r *http.Request) string {
	if picked := s.pickDiskBookPath(r); picked != "" {
		return s.openStudioProjectFromDiskPath(picked)
	}
	return ""
}

func (s *Server) openStudioProjectFromDisk(r *http.Request) string {
	rawPath := strings.TrimSpace(r.FormValue("path"))
	return s.openStudioProjectFromDiskPath(rawPath)
}

func (s *Server) openStudioProjectFromDiskPath(rawPath string) string {
	resolvedPaths, err := book.ResolveDiskBookPaths(rawPath)
	if err != nil {
		return err.Error()
	}
	firstPath := filepath.Clean(resolvedPaths[0])
	loaded, err := book.LoadBookFromPath(firstPath)
	if err != nil || loaded == nil {
		if err != nil {
			return err.Error()
		}
		return "Nie udało się otworzyć projektu"
	}
	if loaded.IsPacked {
		return s.promptUnpackPlaybookForStudio(loaded)
	}
	if filepath.Clean(filepath.Dir(firstPath)) != filepath.Clean(s.BooksDir) {
		s.Settings.AddExternalBook(firstPath)
		loaded.IsExternal = true
	}
	if reloaded, rErr := s.findBookByID(loaded.ID); rErr == nil && reloaded != nil && !reloaded.IsPacked {
		loaded = reloaded
	}
	return s.openBookInStudio(loaded)
}

func (s *Server) browseAddBookFromDisk(r *http.Request) string {
	if picked := s.pickDiskBookPath(r); picked != "" {
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

func (s *Server) prepareBookMediaAction(r *http.Request) (string, string, string, bool) {
	if s.isHistoryPreview() {
		return "", "", historyPreviewLockedToast, false
	}
	if _, hasSettingsOpen := r.Form["scene_settings_open"]; hasSettingsOpen {
		s.SceneTextModalSettingsOpen = r.FormValue("scene_settings_open") == "true"
	}
	if s.ActiveBook == nil || s.ActiveBook.IsPacked {
		return "", "", "", false
	}
	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	if kind == "" {
		kind = strings.ToLower(strings.TrimSpace(r.FormValue("media_kind")))
	}
	if !s.ActiveBook.Narrated && (kind == "audio" || kind == "voice") {
		return "", "", "Książka nie posiada narracji (narrated: false) — nagrania głosowe są wyłączone", false
	}
	scID := strings.TrimSpace(r.FormValue("scene"))
	if scID == "" {
		scID = strings.TrimSpace(r.FormValue("scene_id"))
	}
	return kind, scID, "", true
}

func (s *Server) browseAddBookMedia(r *http.Request) string {
	kind, scID, errToast, ok := s.prepareBookMediaAction(r)
	if !ok {
		return errToast
	}
	if rawPath := strings.TrimSpace(r.FormValue("path")); rawPath != "" {
		return s.addBookMediaFromDiskPaths([]string{rawPath}, kind, scID)
	}
	var picked []string
	s.withUnlocked(func() {
		picked = PickNativeMediaPaths(kind)
	})
	if len(picked) == 0 {
		r.Form.Set("silent", "true")
		return ""
	}
	return s.addBookMediaFromDiskPaths(picked, kind, scID)
}

func (s *Server) addBookMediaFromDisk(r *http.Request) string {
	kind, scID, errToast, ok := s.prepareBookMediaAction(r)
	if !ok {
		return errToast
	}
	rawPath := strings.TrimSpace(r.FormValue("path"))
	return s.addBookMediaFromDiskPaths([]string{rawPath}, kind, scID)
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

	picked := s.pickNativeBookPathUnlocked(true)

	if picked != "" {
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.NewBookDraftDir = picked
		if s.Settings != nil {
			s.Settings.NewBookDir = picked
			s.persistSettings()
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
		s.resetNewBookModalState()

		if rawSavePath != "" && s.Settings != nil {
			s.Settings.NewBookDir = filepath.Dir(created.RootPath)
		}
		if filepath.Clean(filepath.Dir(created.RootPath)) != filepath.Clean(s.BooksDir) {
			s.Settings.AddExternalBook(created.RootPath)
			created.IsExternal = true
		}
		s.setActiveBook(created)
		s.Settings.ActiveView = "editor"
		s.persistSettings()
		if created.IsScreenplay() {
			return "Utworzono nowy scenariusz: " + created.Title
		}
		return "Utworzono nową książkę: " + created.Title
	} else if err != nil {
		return err.Error()
	}
	return ""
}

func (s *Server) removeBookFromForm(r *http.Request) string {
	return s.removeBookByID(book.SanitizeBookID(r.FormValue("book")))
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
		s.setPlayerBook(firstAdded)
	}
	s.persistSettings()

	if len(resolvedPaths) > 1 {
		return fmt.Sprintf("Dodano %d książek z dysku (bez kopiowania)", len(resolvedPaths))
	}
	if firstAdded != nil {
		return "Dodano z dysku (bez kopiowania): " + firstAdded.Title
	}
	return "Dodano książkę z dysku"
}

func (s *Server) addBookFromURL(rawURL string) string {
	targetBooksDir := s.BooksDir
	if !book.IsTempDir(s.RootDir) || book.IsInsideRepoDir(s.RootDir) {
		targetBooksDir = book.DefaultProjectsDir()
	}
	var downloaded *book.Book
	var err error
	s.withUnlocked(func() {
		downloaded, err = book.DownloadBookFromURL(targetBooksDir, rawURL)
	})
	if err != nil {
		return err.Error()
	}
	if filepath.Clean(filepath.Dir(downloaded.RootPath)) != filepath.Clean(s.BooksDir) {
		s.Settings.AddExternalBook(downloaded.RootPath)
	}
	s.AddBookModalOpen = false
	if reloaded, rErr := s.findBookByID(downloaded.ID); rErr == nil && reloaded != nil {
		s.setPlayerBook(reloaded)
	} else {
		s.setPlayerBook(downloaded)
	}
	s.persistSettings()
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
		s.persistSettings()
		msg = "Odpięto książkę z biblioteki (pliki na dysku zachowane): " + b.Title
	} else if b.IsPacked {
		_ = os.Remove(b.RootPath)
		msg = "Usunięto pakiet książki: " + b.Title
	} else {
		_ = os.RemoveAll(b.RootPath)
		_ = os.Remove(filepath.Join(s.BooksDir, b.ID+".playbook"))
		_ = os.Remove(filepath.Join(s.BooksDir, b.ID+".stbook"))
		msg = "Usunięto książkę: " + b.Title
	}

	if s.PlayerBook != nil && s.PlayerBook.ID == b.ID {
		s.setPlayerBook(nil)
		if remaining, err := s.loadBooks(); err == nil && len(remaining) > 0 {
			s.setPlayerBook(remaining[0])
		}
		s.persistSettings()
	}
	if s.ActiveBook != nil && s.ActiveBook.ID == b.ID {
		s.setActiveBook(nil)
		s.persistSettings()
	}
	return msg
}

func (s *Server) extractEmbeddedDefaultBook() string {
	if s.EmbeddedFS == nil {
		return "Brak wbudowanego systemu plików z książką"
	}
	entries, err := fs.ReadDir(s.EmbeddedFS, "books")
	if err != nil {
		return "Wbudowana książka nie została znaleziona w binarium"
	}
	embeddedID := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candPrefix := "books/" + e.Name()
		if _, statErr := fs.Stat(s.EmbeddedFS, candPrefix+"/"+book.BookDataFileName); statErr == nil {
			embeddedID = e.Name()
			break
		}
	}
	if embeddedID == "" {
		return "Wbudowana książka nie została znaleziona w binarium"
	}
	embedPrefix := "books/" + embeddedID

	targetBooksDir := s.BooksDir
	if !book.IsTempDir(s.RootDir) || book.IsInsideRepoDir(s.RootDir) {
		targetBooksDir = book.DefaultProjectsDir()
	}
	sampleDir := filepath.Join(targetBooksDir, embeddedID)
	_ = os.Remove(filepath.Join(targetBooksDir, embeddedID+".stbook"))
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

	if filepath.Clean(targetBooksDir) != filepath.Clean(s.BooksDir) {
		s.Settings.AddExternalBook(sampleDir)
	}
	s.QRModalOpen = false
	if b, err := s.findBookByID(embeddedID); err == nil && b != nil {
		s.setActiveBook(b)
		s.persistSettings()
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
