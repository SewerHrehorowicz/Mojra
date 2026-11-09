//go:build !(js && wasm)

package gui

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"mojra/internal/book"
	"mojra/internal/qr"
	"mojra/internal/tpl"
)

// ModalState groups all active modal/overlay state for the Server while keeping promoted fields accessible.
type ModalState struct {
	SceneTextModalID           string
	SceneTextModalInitialID    string
	LastRenamedFromSceneID     string
	SceneTextModalWidthMode    string
	SceneTextModalSettingsOpen bool
	VoiceRecorderOpen          bool
	QRModalOpen                bool
	NewBookModalOpen           bool
	NewBookDraftTitle          string
	NewBookDraftAuthor         string
	NewBookDraftMode           string
	NewBookDraftStandard       string
	NewBookDraftNarrated       bool
	NewBookDraftImageless      bool
	NewBookDraftDir            string
	ExportModalOpen            bool
	ExportModalFormat          string
	ExportModalFormats         map[string]bool
	ExportModalAdvancedOpen    bool
	ExportModalStandard        string
	ExportModalDir             string
	AddBookModalOpen           bool
	StudioProjectModalOpen     bool
	EditorHelpModalOpen        bool
	EditorHistoryPanelOpen     bool
	EditorBreakdownPanelOpen   bool
	EditorVariablesPanelOpen   bool
	BookSettingsOpen           bool
	ChoiceRulesOpen            map[string]bool
	SyntaxHelpPopoverOpen      bool
	SceneTextPreviewMode       string
	SceneTestVarsPopoverOpen   bool
	SceneTextTestVars          map[string]string
	ConfirmTitle               string
	ConfirmMessage             string
	ConfirmPendingAction       string
	ConfirmPendingParam        string
}

type historyCacheState struct {
	cachedHistory        *book.BookHistory
	cachedHistoryBookID  string
	cachedHistoryModNano int64
	cachedHistorySize    int64
	PreviewRevisionID    string
	PreviewRevisionLabel string
	LiveBook             *book.Book
	LivePlayerBook       *book.Book
	LiveSave             *book.SaveState
	LiveSelectedSceneID  string
}

type assetCacheState struct {
	templatesStamp uint64
	assetMu        sync.RWMutex
	cssStamp       uint64
	cachedCSS      []byte
	jsStamp        uint64
	cachedJS       []byte
	staticAssets   map[string][]byte
}

type Server struct {
	mu               sync.RWMutex
	RootDir          string
	BooksDir         string
	SavesDir         string
	AppDir           string
	EmbeddedFS       fs.FS
	Templates        map[string]*tpl.ParsedTemplate
	Port             int
	DevMode          bool
	StandaloneMode   bool
	StandaloneBookID string
	ShutdownCh       chan struct{}
	once             sync.Once

	// Server-side application state (persisted in saves/settings.ini and saves/<book>.sav)
	Settings          *book.AppSettings
	PlayerBook        *book.Book
	ActiveBook        *book.Book
	StudioHome        bool
	Save              *book.SaveState
	ZenMode           bool
	HistoryRevealed   bool
	FocusedHistoryIdx int
	SelectedSceneID         string
	SelectedChapter         string
	SelectedVoiceSectionKey string
	ModalState
	historyCacheState
	assetCacheState
}

func (s *Server) resetNewBookModalState() {
	s.NewBookModalOpen = false
	s.NewBookDraftTitle = ""
	s.NewBookDraftAuthor = ""
	s.NewBookDraftMode = ""
	s.NewBookDraftStandard = ""
	s.NewBookDraftNarrated = false
	s.NewBookDraftImageless = false
	s.NewBookDraftDir = ""
}

func (s *Server) resetExportModalState() {
	s.ExportModalOpen = false
	s.ExportModalFormat = ""
	s.ExportModalFormats = nil
	s.ExportModalAdvancedOpen = false
	s.ExportModalStandard = ""
	s.ExportModalDir = ""
}

func (s *Server) closeAllModals() {
	s.QRModalOpen = false
	s.SceneTextModalID = ""
	s.SceneTextModalSettingsOpen = false
	s.EditorHelpModalOpen = false
	s.resetNewBookModalState()
	s.resetExportModalState()
	s.AddBookModalOpen = false
	s.StudioProjectModalOpen = false
	s.ConfirmPendingAction = ""
	s.ConfirmPendingParam = ""
}

func (s *Server) withUnlocked(fn func()) {
	mu := &s.mu
	mu.Unlock()
	defer mu.Lock()
	fn()
}

func (s *Server) pickNativeBookPathUnlocked(chooseDir bool) string {
	var picked string
	s.withUnlocked(func() {
		picked = PickNativeBookPath(chooseDir)
	})
	return picked
}

func (s *Server) loadBooks() ([]*book.Book, error) {
	if s.StandaloneMode {
		if s.PlayerBook != nil {
			return []*book.Book{s.PlayerBook}, nil
		}
		if s.ActiveBook != nil {
			return []*book.Book{s.ActiveBook}, nil
		}
		return nil, nil
	}
	var extPaths []string
	if s.Settings != nil {
		extPaths = s.Settings.ExternalBooks
	}
	return book.LoadLibraryBooks(s.BooksDir, extPaths)
}

func computeFilesStamp(paths []string) uint64 {
	var h uint64 = 14695981039346656037
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return 0
		}
		h ^= uint64(st.ModTime().UnixNano())
		h *= 1099511628211
		h ^= uint64(st.Size())
		h *= 1099511628211
	}
	return h
}

func computeDirsStamp(dirs ...string) uint64 {
	var h uint64 = 14695981039346656037
	found := false
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			found = true
			h ^= uint64(info.ModTime().UnixNano())
			h *= 1099511628211
			h ^= uint64(info.Size())
			h *= 1099511628211
		}
	}
	if !found {
		return 0
	}
	return h
}

func (s *Server) ensureTemplatesLoaded() {
	if !s.StandaloneMode {
		tplDir := filepath.Join(s.AppDir, "templates")
		iconsDir := filepath.Join(s.AppDir, "icons")
		stamp := computeDirsStamp(tplDir, iconsDir)
		if s.Templates == nil || (stamp != 0 && stamp != s.templatesStamp) {
			if loaded, err := tpl.LoadTemplatesFromDir(tplDir, s.EmbeddedFS); err == nil && len(loaded) > 0 {
				if !s.DevMode {
					tpl.StripDevNodesFromTemplates(loaded)
				}
				s.Templates = loaded
				s.templatesStamp = stamp
			}
		}
	}
	if s.Templates == nil {
		tplDir := filepath.Join(s.AppDir, "templates")
		if s.StandaloneMode {
			tplDir = ""
		}
		s.Templates, _ = tpl.LoadTemplatesFromDir(tplDir, s.EmbeddedFS)
		if s.StandaloneMode && s.Templates != nil {
			delete(s.Templates, "library_view")
			delete(s.Templates, "editor_view")
			delete(s.Templates, "editor_context_panel")
			delete(s.Templates, "editor_history_panel")
			delete(s.Templates, "editor_breakdown_panel")
			delete(s.Templates, "editor_variables_panel")
		}
		if (!s.DevMode || s.StandaloneMode) && s.Templates != nil {
			tpl.StripDevNodesFromTemplates(s.Templates)
		}
	}
}

func NewServerWithOptions(rootDir string, embeddedFS fs.FS, devMode bool) *Server {
	booksDir := filepath.Join(rootDir, "books")
	savesDir := book.ResolveSavesDir(rootDir)
	appDir := filepath.Join(rootDir, "app")
	if !book.IsInsideRepoDir(rootDir) || book.IsTempDir(rootDir) {
		_ = os.MkdirAll(booksDir, 0755)
	}
	_ = os.MkdirAll(savesDir, 0755)

	settings := book.LoadAppSettings(savesDir)
	if !devMode {
		settings.PreviewMode = "full"
	}

	s := &Server{
		RootDir:           rootDir,
		BooksDir:          booksDir,
		SavesDir:          savesDir,
		AppDir:            appDir,
		EmbeddedFS:        embeddedFS,
		DevMode:           devMode,
		ShutdownCh:        make(chan struct{}),
		Settings:          settings,
		FocusedHistoryIdx: -1,
	}
	s.ensureTemplatesLoaded()

	if settings.ActiveBookID != "" {
		if b, err := s.findBookByID(settings.ActiveBookID); err == nil && b != nil {
			s.setPlayerBook(b)
		}
	}
	if s.PlayerBook == nil {
		if books, err := s.loadBooks(); err == nil && len(books) > 0 {
			s.setPlayerBook(books[0])
		}
	}
	if settings.EditorBookID != "" {
		if b, err := s.findBookByID(settings.EditorBookID); err == nil && b != nil && !b.IsPacked {
			s.setActiveBook(b)
			if settings.EditorStudioHome {
				s.StudioHome = true
				s.SceneTextModalID = ""
			} else {
				s.restoreBookEditorUIState(settings.ActiveView == "editor")
			}
		}
	}

	return s
}

func isRunningUnderGoTestBinary() bool {
	return os.Getenv("ST_NO_WINDOW") == "1" ||
		strings.HasSuffix(os.Args[0], ".test") ||
		strings.HasSuffix(os.Args[0], ".test.exe") ||
		strings.Contains(os.Args[0], "/_test/")
}

func (s *Server) defaultNewBookDir() string {
	if s.NewBookDraftDir != "" {
		return s.NewBookDraftDir
	}
	if s.Settings != nil && strings.TrimSpace(s.Settings.NewBookDir) != "" {
		if expanded, err := book.ExpandLocalPath(s.Settings.NewBookDir); err == nil && expanded != "" {
			return expanded
		}
	}
	if isRunningUnderGoTestBinary() && book.IsTempDir(s.RootDir) && !book.IsInsideRepoDir(s.RootDir) {
		return s.BooksDir
	}
	return book.DefaultProjectsDir()
}

func (s *Server) defaultExportDir() string {
	if s.ActiveBook != nil {
		if strings.TrimSpace(s.ActiveBook.ExportDir) != "" {
			if expanded, err := book.ExpandLocalPath(s.ActiveBook.ExportDir); err == nil && expanded != "" {
				return expanded
			}
		}
		if s.Settings != nil {
			if saved := strings.TrimSpace(s.Settings.GetBookExportDir(s.ActiveBook.ID)); saved != "" {
				if expanded, err := book.ExpandLocalPath(saved); err == nil && expanded != "" {
					return expanded
				}
			}
		}
	}
	if s.Settings != nil && strings.TrimSpace(s.Settings.ExportDir) != "" {
		if expanded, err := book.ExpandLocalPath(s.Settings.ExportDir); err == nil && expanded != "" {
			return expanded
		}
	}
	if isRunningUnderGoTestBinary() && book.IsTempDir(s.RootDir) && !book.IsInsideRepoDir(s.RootDir) {
		return filepath.Join(s.SavesDir, "exports")
	}
	return book.DefaultExportDir()
}

func (s *Server) persistActiveBookExportDir(dir string) {
	if s.ActiveBook != nil {
		s.ActiveBook.ExportDir = dir
	}
	if s.Settings != nil {
		if s.ActiveBook != nil {
			s.Settings.SetBookExportDir(s.ActiveBook.ID, dir)
		}
		if dir != "" {
			s.Settings.ExportDir = dir
		}
		s.persistSettings()
	}
}

func (s *Server) resolveOrPickBookExportDir(r *http.Request) (string, string, bool) {
	if s.ActiveBook == nil {
		return "", "Brak aktywnego projektu", false
	}
	if r != nil {
		if rawDir := strings.TrimSpace(r.FormValue("export_dir")); rawDir != "" {
			expanded, err := book.ExpandLocalPath(rawDir)
			if err != nil {
				return "", err.Error(), false
			}
			s.persistActiveBookExportDir(expanded)
			return expanded, "", true
		}
	}
	forcePrompt := r != nil && r.FormValue("prompt_dir") == "true"
	if !forcePrompt && strings.TrimSpace(s.ActiveBook.ExportDir) != "" {
		expanded, err := book.ExpandLocalPath(s.ActiveBook.ExportDir)
		if err != nil {
			return "", err.Error(), false
		}
		return expanded, "", true
	}
	if !isRunningUnderGoTestBinary() {
		picked := s.pickNativeBookPathUnlocked(true)
		if picked == "" {
			if r != nil {
				r.Form.Set("silent", "true")
			}
			return "", "", false
		}
		if expanded, err := book.ExpandLocalPath(picked); err == nil && expanded != "" {
			picked = expanded
		}
		s.persistActiveBookExportDir(picked)
		return picked, "", true
	}
	defDir := s.defaultExportDir()
	s.persistActiveBookExportDir(defDir)
	return defDir, "", true
}

func (s *Server) revealExportedFileInOS(filePath string) {
	if isRunningUnderGoTestBinary() || os.Getenv("ST_NO_WINDOW") == "1" {
		return
	}
	cleanPath := filepath.Clean(strings.TrimSpace(filePath))
	if cleanPath == "" || cleanPath == "." {
		return
	}
	dirPath := filepath.Dir(cleanPath)
	go func() {
		switch runtime.GOOS {
		case "darwin":
			if err := exec.Command("open", "-R", cleanPath).Start(); err != nil {
				_ = exec.Command("open", dirPath).Start()
			}
		case "windows":
			if err := exec.Command("explorer.exe", "/select,", cleanPath).Start(); err != nil {
				_ = exec.Command("explorer.exe", dirPath).Start()
			}
		case "linux":
			_ = exec.Command("xdg-open", dirPath).Start()
		}
	}()
}

func (s *Server) saveAppSettings() error {
	if s.Settings == nil {
		return nil
	}
	return book.SaveAppSettings(s.SavesDir, s.Settings)
}

func (s *Server) persistSettings() {
	if err := s.saveAppSettings(); err != nil {
		return
	}
}

func (s *Server) persistSaveState() {
	if s.Save == nil || s.isHistoryPreview() {
		return
	}
	if err := book.WriteSaveState(s.SavesDir, s.Save); err != nil {
		return
	}
}

func (s *Server) persistAudioTimeInPlace(t float64) {
	if s.PlayerBook == nil || s.Save == nil || s.isHistoryPreview() {
		return
	}
	if err := book.UpdateSaveAudioTimeInPlace(s.SavesDir, s.PlayerBook.ID, t, s.PlayerBook.StartSceneID); err != nil {
		return
	}
}

func (s *Server) setPlayerBook(b *book.Book) {
	if s.isHistoryPreview() {
		s.exitHistoryPreview()
	}
	s.PlayerBook = b
	if b == nil {
		if s.Settings != nil {
			s.Settings.ActiveBookID = ""
		}
		s.Save = nil
		s.HistoryRevealed = false
		s.FocusedHistoryIdx = -1
		return
	}
	if s.Settings != nil {
		s.Settings.ActiveBookID = b.ID
	}
	s.Save = book.LoadSaveState(s.SavesDir, b.ID, b.StartSceneID)
	if s.findPlayerScene(s.Save.CurrentSceneID) == nil && len(b.Scenes) > 0 {
		s.Save.CurrentSceneID = b.StartSceneID
		if s.findPlayerScene(s.Save.CurrentSceneID) == nil {
			s.Save.CurrentSceneID = b.Scenes[0].ID
		}
	}
	s.HistoryRevealed = false
	s.FocusedHistoryIdx = -1
}

func (s *Server) hasActiveStudioEditor() bool {
	return s.ActiveBook != nil && !s.StudioHome
}

func (s *Server) currentStudioRightPanel() string {
	switch {
	case s.BookSettingsOpen:
		return "book"
	case s.SelectedChapter != "":
		return "chapter"
	case s.EditorVariablesPanelOpen:
		return "variables"
	case s.EditorBreakdownPanelOpen:
		return "breakdown"
	case s.EditorHistoryPanelOpen:
		return "history"
	case s.EditorHelpModalOpen:
		return "help"
	default:
		return ""
	}
}

func (s *Server) saveCurrentBookEditorUIState() {
	if s.Settings == nil || s.ActiveBook == nil || s.StandaloneMode || s.isHistoryPreview() {
		return
	}
	st := book.EditorBookUIState{
		SelectedSceneID:       s.SelectedSceneID,
		SceneTextModalID:      s.SceneTextModalID,
		SceneTextWidthMode:    s.SceneTextModalWidthMode,
		SceneTextSettingsOpen: s.SceneTextModalSettingsOpen,
		RightPanel:            s.currentStudioRightPanel(),
		SelectedChapter:       s.SelectedChapter,
	}
	s.Settings.SetEditorBookUIState(s.ActiveBook.ID, st)
}

func (s *Server) restoreBookEditorUIState(openModalIfSaved bool) {
	if s.Settings == nil || s.ActiveBook == nil {
		return
	}
	st, ok := s.Settings.GetEditorBookUIState(s.ActiveBook.ID)
	if !ok && strings.EqualFold(s.Settings.EditorBookID, s.ActiveBook.ID) {
		st = book.EditorBookUIState{
			SelectedSceneID:       s.Settings.EditorSelectedSceneID,
			SceneTextModalID:      s.Settings.EditorSceneTextModalID,
			SceneTextWidthMode:    s.Settings.EditorSceneTextWidth,
			SceneTextSettingsOpen: s.Settings.EditorSceneSettings,
			RightPanel:            s.Settings.EditorRightPanel,
			SelectedChapter:       s.Settings.EditorSelectedChapter,
		}
		ok = st.SelectedSceneID != "" || st.SceneTextModalID != "" || st.SceneTextWidthMode != "" || st.RightPanel != ""
	}
	if !ok {
		return
	}
	if st.SelectedSceneID != "" {
		if sc := s.findScene(st.SelectedSceneID); sc != nil {
			s.SelectedSceneID = sc.ID
		}
	}
	if st.SceneTextWidthMode == "full" || st.SceneTextWidthMode == "half" {
		s.SceneTextModalWidthMode = st.SceneTextWidthMode
	}
	s.SceneTextModalSettingsOpen = st.SceneTextSettingsOpen

	s.BookSettingsOpen = false
	s.SelectedChapter = ""
	s.EditorVariablesPanelOpen = false
	s.EditorBreakdownPanelOpen = false
	s.EditorHistoryPanelOpen = false
	s.EditorHelpModalOpen = false
	switch st.RightPanel {
	case "book":
		s.BookSettingsOpen = true
	case "chapter":
		if st.SelectedChapter != "" && s.ActiveBook.FindChapter(st.SelectedChapter) != nil {
			s.SelectedChapter = st.SelectedChapter
		}
	case "variables":
		s.EditorVariablesPanelOpen = true
	case "breakdown":
		s.EditorBreakdownPanelOpen = true
	case "history":
		s.EditorHistoryPanelOpen = true
	case "help":
		s.EditorHelpModalOpen = true
	}

	if openModalIfSaved && st.SceneTextModalID != "" && !s.StudioHome {
		if modalSc := s.findScene(st.SceneTextModalID); modalSc != nil {
			s.SceneTextModalID = modalSc.ID
			s.SelectedSceneID = modalSc.ID
		}
	}
}

func (s *Server) syncAndPersistEditorUIState() {
	if s.Settings == nil || s.StandaloneMode || s.isHistoryPreview() {
		return
	}
	changed := false
	if s.Settings.EditorStudioHome != s.StudioHome {
		s.Settings.EditorStudioHome = s.StudioHome
		changed = true
	}
	if s.ActiveBook == nil {
		if s.Settings.EditorBookID != "" {
			s.Settings.EditorBookID = ""
			changed = true
		}
		if changed {
			s.persistSettings()
		}
		return
	}

	bookID := book.SanitizeBookID(s.ActiveBook.ID)
	if s.Settings.EditorBookID != bookID {
		s.Settings.EditorBookID = bookID
		changed = true
	}

	inActiveEditor := s.Settings.ActiveView == "editor" && !s.StudioHome
	prevSt, _ := s.Settings.GetEditorBookUIState(bookID)
	modalID := s.SceneTextModalID
	if !inActiveEditor && modalID == "" {
		modalID = prevSt.SceneTextModalID
	}
	widthMode := s.SceneTextModalWidthMode
	if widthMode != "full" && widthMode != "half" {
		widthMode = prevSt.SceneTextWidthMode
	}
	rightPanel := s.currentStudioRightPanel()
	selChapter := s.SelectedChapter

	nextSt := book.EditorBookUIState{
		SelectedSceneID:       s.SelectedSceneID,
		SceneTextModalID:      modalID,
		SceneTextWidthMode:    widthMode,
		SceneTextSettingsOpen: s.SceneTextModalSettingsOpen,
		RightPanel:            rightPanel,
		SelectedChapter:       selChapter,
	}
	if prevSt != nextSt {
		s.Settings.SetEditorBookUIState(bookID, nextSt)
		changed = true
	}
	if s.Settings.EditorSelectedSceneID != nextSt.SelectedSceneID {
		s.Settings.EditorSelectedSceneID = nextSt.SelectedSceneID
		changed = true
	}
	if s.Settings.EditorSceneTextModalID != nextSt.SceneTextModalID {
		s.Settings.EditorSceneTextModalID = nextSt.SceneTextModalID
		changed = true
	}
	if s.Settings.EditorSceneTextWidth != nextSt.SceneTextWidthMode {
		s.Settings.EditorSceneTextWidth = nextSt.SceneTextWidthMode
		changed = true
	}
	if s.Settings.EditorSceneSettings != nextSt.SceneTextSettingsOpen {
		s.Settings.EditorSceneSettings = nextSt.SceneTextSettingsOpen
		changed = true
	}
	if s.Settings.EditorRightPanel != nextSt.RightPanel {
		s.Settings.EditorRightPanel = nextSt.RightPanel
		changed = true
	}
	if s.Settings.EditorSelectedChapter != nextSt.SelectedChapter {
		s.Settings.EditorSelectedChapter = nextSt.SelectedChapter
		changed = true
	}
	if changed {
		s.persistSettings()
	}
}

func (s *Server) setActiveBook(b *book.Book) {
	if s.ActiveBook != nil && (b == nil || s.ActiveBook.ID != b.ID) && s.Settings != nil && s.Settings.ActiveView == "editor" && !s.StudioHome {
		s.saveCurrentBookEditorUIState()
	}
	s.PreviewRevisionID = ""
	s.PreviewRevisionLabel = ""
	s.LiveBook = nil
	s.LivePlayerBook = nil
	s.LiveSave = nil
	s.LiveSelectedSceneID = ""
	if b != nil && s.Settings != nil && strings.TrimSpace(b.ExportDir) == "" {
		b.ExportDir = s.Settings.GetBookExportDir(b.ID)
	}
	s.ActiveBook = b
	s.StudioHome = b == nil
	s.invalidateHistoryCache()
	if b == nil {
		if s.Settings != nil {
			s.Settings.EditorBookID = ""
		}
		s.EditorHistoryPanelOpen = false
		s.EditorBreakdownPanelOpen = false
		s.EditorVariablesPanelOpen = false
		s.EditorHelpModalOpen = false
		s.BookSettingsOpen = false
		s.SelectedChapter = ""
		s.SelectedSceneID = ""
		return
	}
	if s.Settings != nil {
		s.Settings.EditorBookID = b.ID
	}
	s.EditorHistoryPanelOpen = false
	s.EditorBreakdownPanelOpen = false
	s.EditorVariablesPanelOpen = false
	s.EditorHelpModalOpen = false
	s.BookSettingsOpen = false
	s.SelectedChapter = ""
	if len(b.Scenes) > 0 {
		s.SelectedSceneID = b.StartSceneID
		if s.findScene(s.SelectedSceneID) == nil {
			s.SelectedSceneID = b.Scenes[0].ID
		}
	} else {
		s.SelectedSceneID = ""
	}
	s.restoreBookEditorUIState(false)
	if s.StandaloneMode || s.PlayerBook == nil {
		s.setPlayerBook(b)
	}
}

func (s *Server) currentScene() *book.Scene {
	if s.PlayerBook == nil || len(s.PlayerBook.Scenes) == 0 {
		return nil
	}
	if s.Save != nil {
		if sc := s.findPlayerScene(s.Save.CurrentSceneID); sc != nil {
			return sc
		}
	}
	if s.PlayerBook.StartSceneID != "" {
		if sc := s.findPlayerScene(s.PlayerBook.StartSceneID); sc != nil {
			return sc
		}
	}
	return s.PlayerBook.Scenes[0]
}

func (s *Server) currentAudioTime() float64 {
	if s.Save != nil {
		return s.Save.AudioTime
	}
	return 0
}

func (s *Server) contextBookForTopbar() *book.Book {
	if !s.StandaloneMode && s.Settings != nil && s.Settings.ActiveView == "editor" && s.ActiveBook != nil {
		return s.ActiveBook
	}
	if s.PlayerBook != nil {
		return s.PlayerBook
	}
	return s.ActiveBook
}

func (s *Server) isBookImageless() bool {
	b := s.contextBookForTopbar()
	return b != nil && (b.Imageless || b.IsScreenplay())
}

func (s *Server) isEffectiveImageless() bool {
	if s.isBookImageless() {
		return true
	}
	return s.Settings != nil && s.Settings.Imageless
}

func findSceneInBook(b *book.Book, sceneID string) *book.Scene {
	if b == nil {
		return nil
	}
	for _, sc := range b.Scenes {
		if sc.ID == sceneID {
			return sc
		}
	}
	for _, sc := range b.Scenes {
		if strings.EqualFold(sc.ID, sceneID) || (sc.ExplicitID != "" && strings.EqualFold(sc.ExplicitID, sceneID)) {
			return sc
		}
	}
	normKey := book.NormalizeLookupKey(sceneID)
	for _, sc := range b.Scenes {
		if sc.Name != "" && (strings.EqualFold(sc.Name, sceneID) || (normKey != "" && book.NormalizeLookupKey(sc.Name) == normKey)) {
			return sc
		}
	}
	return nil
}

func (s *Server) findScene(sceneID string) *book.Scene {
	return findSceneInBook(s.ActiveBook, sceneID)
}

func (s *Server) findPlayerScene(sceneID string) *book.Scene {
	return findSceneInBook(s.PlayerBook, sceneID)
}

func (s *Server) TriggerShutdown() {
	s.once.Do(func() {
		close(s.ShutdownCh)
	})
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/action", s.handleAction)
	mux.HandleFunc("/api/lan-qr.png", s.handleLANQR)
	mux.HandleFunc("/api/export/screenplay.pdf", s.handleExportScreenplayPDF)
	mux.HandleFunc("/api/export/screenplay.fountain", s.handleExportScreenplayFountain)
	mux.HandleFunc("/api/books/", s.handleBookMediaRoute)
	mux.HandleFunc("/", s.handleRootAndAssets)

	return withSecurityHeaders(mux)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		if !strings.HasPrefix(r.URL.Path, "/api/books/") || !strings.Contains(r.URL.Path, "/media/") {
			w.Header().Set(
				"Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; media-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'",
			)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) activeWorkspaceView() string {
	view := s.Settings.ActiveView
	if s.StandaloneMode {
		return "player"
	}
	if view == "player" && s.PlayerBook == nil {
		return "library"
	}
	if view != "player" && view != "editor" {
		return "library"
	}
	return view
}

func (s *Server) activeWorkspaceTemplateName() string {
	switch s.activeWorkspaceView() {
	case "player":
		return "player_view"
	case "editor":
		return "editor_view"
	default:
		return "library_view"
	}
}

func (s *Server) handleExportScreenplayPDF(w http.ResponseWriter, r *http.Request) {
	if s.StandaloneMode {
		http.Error(w, "forbidden in standalone mode", http.StatusForbidden)
		return
	}
	s.mu.RLock()
	active := s.ActiveBook
	s.mu.RUnlock()
	if active == nil {
		http.Error(w, "brak aktywnego projektu", http.StatusNotFound)
		return
	}
	pdfBytes, err := book.ExportScreenplayPDF(active)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fileName := active.ID + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdfBytes)
}

func (s *Server) handleExportScreenplayFountain(w http.ResponseWriter, r *http.Request) {
	if s.StandaloneMode {
		http.Error(w, "forbidden in standalone mode", http.StatusForbidden)
		return
	}
	s.mu.RLock()
	active := s.ActiveBook
	s.mu.RUnlock()
	if active == nil {
		http.Error(w, "brak aktywnego projektu", http.StatusNotFound)
		return
	}
	fountainText := book.ExportScreenplayFountain(active)
	fileName := active.ID + ".fountain"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(fountainText))
}

func (s *Server) handleLANQR(w http.ResponseWriter, r *http.Request) {
	if !s.DevMode || s.StandaloneMode {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lanIP := DetectLANIP()
	port := s.Port
	if port == 0 {
		port = 8484
	}
	lanURL := fmt.Sprintf("http://%s:%d", lanIP, port)
	pngBytes, err := qr.EncodePNG(lanURL, 8)
	if err != nil {
		http.Error(w, "failed to encode QR", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pngBytes)
}

func (s *Server) isStandaloneMediaUnlocked(b *book.Book, reqURLPath, rawRel string) bool {
	if b == nil {
		return false
	}
	cleanRel := book.SanitizeRelativeMediaPath(rawRel)
	if cleanRel == "" {
		return false
	}
	matchMedia := func(fileRel, fullURL string) bool {
		if fileRel != "" && strings.EqualFold(book.SanitizeRelativeMediaPath(fileRel), cleanRel) {
			return true
		}
		if fullURL != "" && strings.EqualFold(fullURL, reqURLPath) {
			return true
		}
		return false
	}
	if matchMedia(b.CoverFile, b.CoverURL) {
		return true
	}
	matchScene := func(sc *book.Scene) bool {
		if sc == nil {
			return false
		}
		return matchMedia(sc.ImageFile, sc.ImageURL) ||
			matchMedia(sc.AudioFile, sc.AudioURL) ||
			matchMedia(sc.ExplicitImage, "") ||
			matchMedia(sc.ExplicitVoice, "")
	}
	if matchScene(s.currentScene()) {
		return true
	}
	if s.Save != nil {
		for _, h := range s.Save.History {
			if matchScene(findSceneInBook(b, h.SceneID)) {
				return true
			}
		}
	}
	return false
}

func (s *Server) handleBookMediaRoute(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/books/")
	parts := strings.SplitN(trimmed, "/media/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	bookID := book.SanitizeBookID(parts[0])

	if s.StandaloneMode {
		s.mu.RLock()
		active := s.PlayerBook
		if active == nil {
			active = s.ActiveBook
		}
		allowed := active != nil && active.ID == bookID && s.isStandaloneMediaUnlocked(active, r.URL.Path, parts[1])
		s.mu.RUnlock()
		if active == nil || active.ID != bookID {
			http.Error(w, "forbidden in standalone mode", http.StatusForbidden)
			return
		}
		if !allowed {
			http.Error(w, "media not unlocked yet", http.StatusForbidden)
			return
		}
		book.ServeBookMedia(w, r, active, parts[1])
		return
	}

	s.mu.RLock()
	activeEditor := s.ActiveBook
	activePlayer := s.PlayerBook
	s.mu.RUnlock()
	if activePlayer != nil && activePlayer.ID == bookID {
		book.ServeBookMedia(w, r, activePlayer, parts[1])
		return
	}
	if activeEditor != nil && activeEditor.ID == bookID {
		book.ServeBookMedia(w, r, activeEditor, parts[1])
		return
	}

	b, err := s.findBookByID(bookID)
	if err != nil || b == nil {
		http.NotFound(w, r)
		return
	}
	book.ServeBookMedia(w, r, b, parts[1])
}

func (s *Server) handleRootAndAssets(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Path
	if reqPath == "/" || reqPath == "/index.html" {
		s.mu.Lock()
		defer s.mu.Unlock()

		var books []*book.Book
		if s.activeWorkspaceView() == "library" || (s.activeWorkspaceView() == "editor" && !s.hasActiveStudioEditor()) {
			books, _ = s.loadBooks()
		}
		s.ensureTemplatesLoaded()
		ctx := s.BuildTargetedRenderContext("index", books)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if err := tpl.RenderTemplateTo(w, "index", s.Templates, ctx); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if reqPath == "/favicon.ico" {
		s.serveUIAsset(w, r, "icons/favicon.ico", "image/x-icon")
		return
	}
	if reqPath == "/icon.png" {
		s.serveUIAsset(w, r, "icons/icon.png", "image/png")
		return
	}
	if reqPath == "/app.js" {
		s.serveUIAsset(w, r, "app.js", "application/javascript; charset=utf-8")
		return
	}
	if reqPath == "/studio-atlas.png" {
		s.serveUIAsset(w, r, "studio-atlas.png", "image/png")
		return
	}
	if reqPath == "/studio-ui.png" {
		s.serveUIAsset(w, r, "studio-ui.png", "image/png")
		return
	}
	if reqPath == "/style.css" {
		s.serveUIAsset(w, r, "style.css", "text/css; charset=utf-8")
		return
	}
	if strings.HasPrefix(reqPath, "/styles/") && strings.HasSuffix(reqPath, ".css") {
		cleanRel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(reqPath)), "/")
		if !strings.Contains(cleanRel, "..") {
			s.serveUIAsset(w, r, cleanRel, "text/css; charset=utf-8")
			return
		}
	}
	if strings.HasPrefix(reqPath, "/icons/") && strings.HasSuffix(reqPath, ".svg") {
		cleanRel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(reqPath)), "/")
		if !strings.Contains(cleanRel, "..") {
			s.serveUIAsset(w, r, cleanRel, "image/svg+xml")
			return
		}
	}
	if strings.HasPrefix(reqPath, "/fonts/") {
		cleanRel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(reqPath)), "/")
		if !strings.Contains(cleanRel, "..") {
			var fontMime string
			switch strings.ToLower(filepath.Ext(cleanRel)) {
			case ".ttf":
				fontMime = "font/ttf"
			case ".otf":
				fontMime = "font/otf"
			case ".woff2":
				fontMime = "font/woff2"
			case ".woff":
				fontMime = "font/woff"
			}
			if fontMime != "" {
				s.serveUIAsset(w, r, cleanRel, fontMime)
				return
			}
		}
	}
	http.NotFound(w, r)
}

func (s *Server) serveUIAsset(w http.ResponseWriter, r *http.Request, relAppPath string, contentType string) {
	w.Header().Set("Content-Type", contentType)
	if strings.HasPrefix(relAppPath, "fonts/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	if !s.StandaloneMode {
		stripDev := !s.DevMode
		switch relAppPath {
		case "style.css":
			cssFiles := tpl.CSSModuleFiles(s.AppDir)
			stamp := computeFilesStamp(cssFiles)
			if stamp != 0 {
				s.assetMu.RLock()
				cached := s.cachedCSS
				cachedStamp := s.cssStamp
				s.assetMu.RUnlock()
				if len(cached) > 0 && stamp == cachedStamp {
					_, _ = w.Write(cached)
					return
				}
				if bundled, err := tpl.BundleFilesToString(cssFiles, stripDev); err == nil {
					raw := []byte(bundled)
					s.assetMu.Lock()
					s.cachedCSS = raw
					s.cssStamp = stamp
					s.assetMu.Unlock()
					_, _ = w.Write(raw)
					return
				}
			}
		case "app.js":
			jsFiles := tpl.JSModuleFiles(s.AppDir)
			stamp := computeFilesStamp(jsFiles)
			if stamp != 0 {
				s.assetMu.RLock()
				cached := s.cachedJS
				cachedStamp := s.jsStamp
				s.assetMu.RUnlock()
				if len(cached) > 0 && stamp == cachedStamp {
					_, _ = w.Write(cached)
					return
				}
				if bundled, err := tpl.BundleFilesToString(jsFiles, stripDev); err == nil {
					raw := []byte(bundled)
					s.assetMu.Lock()
					s.cachedJS = raw
					s.jsStamp = stamp
					s.assetMu.Unlock()
					_, _ = w.Write(raw)
					return
				}
			}
		case "studio-atlas.png":
			if s.DevMode {
				if pngBytes, _, err := tpl.BuildStudioMSDFAtlas(s.AppDir); err == nil && len(pngBytes) > 0 {
					_, _ = w.Write(pngBytes)
					return
				}
			}
		case "studio-ui.png":
			if pngBytes, err := tpl.BuildStudioUIAtlas(); err == nil && len(pngBytes) > 0 {
				_, _ = w.Write(pngBytes)
				return
			}
		}
	}

	s.assetMu.RLock()
	if s.staticAssets != nil {
		if cached, ok := s.staticAssets[relAppPath]; ok {
			s.assetMu.RUnlock()
			_, _ = w.Write(cached)
			return
		}
	}
	s.assetMu.RUnlock()

	writeAsset := func(data []byte) {
		if (!s.DevMode || s.StandaloneMode) && (strings.HasSuffix(relAppPath, ".css") || strings.HasSuffix(relAppPath, ".js")) {
			data = []byte(tpl.StripDevOnlyBlocks(string(data)))
		}
		s.assetMu.Lock()
		if s.staticAssets == nil {
			s.staticAssets = make(map[string][]byte)
		}
		s.staticAssets[relAppPath] = data
		s.assetMu.Unlock()
		_, _ = w.Write(data)
	}

	if !s.StandaloneMode {
		if !s.DevMode || (relAppPath != "style.css" && relAppPath != "app.js") {
			buildDiskPath := filepath.Join(s.RootDir, "build", "app", filepath.FromSlash(relAppPath))
			if data, err := os.ReadFile(buildDiskPath); err == nil {
				writeAsset(data)
				return
			}
		}
		diskPath := filepath.Join(s.AppDir, filepath.FromSlash(relAppPath))
		if data, err := os.ReadFile(diskPath); err == nil {
			writeAsset(data)
			return
		}
	}
	if s.EmbeddedFS != nil {
		cleanSlash := strings.TrimPrefix(filepath.ToSlash(relAppPath), "/")
		stripDev := !s.DevMode || s.StandaloneMode
		if !stripDev {
			switch cleanSlash {
			case "style.css":
				if bundled, err := tpl.BundleCSSFromFS(s.EmbeddedFS, false); err == nil {
					writeAsset([]byte(bundled))
					return
				}
			case "app.js":
				if bundled, err := tpl.BundleJSFromFS(s.EmbeddedFS, false); err == nil {
					writeAsset([]byte(bundled))
					return
				}
			}
		}
		for _, embedPath := range []string{"build/app/" + cleanSlash, "app/" + cleanSlash} {
			if data, err := fs.ReadFile(s.EmbeddedFS, embedPath); err == nil {
				writeAsset(data)
				return
			}
		}
		switch cleanSlash {
		case "style.css":
			if bundled, err := tpl.BundleCSSFromFS(s.EmbeddedFS, stripDev); err == nil {
				writeAsset([]byte(bundled))
				return
			}
		case "app.js":
			if bundled, err := tpl.BundleJSFromFS(s.EmbeddedFS, stripDev); err == nil {
				writeAsset([]byte(bundled))
				return
			}
		case "studio-atlas.png":
			if pngBytes, _, err := tpl.BuildStudioMSDFAtlasFromFS(s.EmbeddedFS); err == nil && len(pngBytes) > 0 {
				writeAsset(pngBytes)
				return
			}
		case "studio-ui.png":
			if pngBytes, err := tpl.BuildStudioUIAtlas(); err == nil && len(pngBytes) > 0 {
				writeAsset(pngBytes)
				return
			}
		}
	}
	http.NotFound(w, r)
}

// DetectLANIP finds the preferred local Wi-Fi/LAN IPv4 address for mobile QR preview.
func DetectLANIP() string {
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
					continue
				}
				if ipv4 := ip.To4(); ipv4 != nil {
					return ipv4.String()
				}
			}
		}
	}
	return "127.0.0.1"
}

func (s *Server) lang() Lang {
	if s == nil || s.Settings == nil {
		return LangPL
	}
	return tpl.ParseLang(s.Settings.Language)
}

func (s *Server) tr(k TrKey) string {
	return i18nTable[k][s.lang()]
}

func (s *Server) trf(k TrKey, args ...any) string {
	return fmt.Sprintf(s.tr(k), args...)
}
