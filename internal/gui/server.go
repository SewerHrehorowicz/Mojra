package gui

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"storyteller/internal/book"
	"storyteller/internal/qr"
	"storyteller/internal/tpl"
)

// ModalState groups all active modal/overlay state for the Server while keeping promoted fields accessible.
type ModalState struct {
	SceneTextModalID           string
	SceneTextModalSettingsOpen bool
	QRModalOpen                bool
	NewBookModalOpen           bool
	NewBookDraftTitle          string
	NewBookDraftAuthor         string
	NewBookDraftMode           string
	NewBookDraftStandard       string
	NewBookDraftNarrated       bool
	NewBookDraftImageless      bool
	NewBookDraftDir            string
	AddBookModalOpen           bool
	EditorHelpModalOpen        bool
	EditorHistoryPanelOpen     bool
	EditorBreakdownPanelOpen   bool
	BookSettingsOpen           bool
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
	ActiveBook        *book.Book
	Save              *book.SaveState
	HistoryRevealed   bool
	FocusedHistoryIdx int
	SelectedSceneID   string
	SelectedChapter   string
	ModalState
	historyCacheState
	assetCacheState
}

func (s *Server) closeAllModals() {
	s.QRModalOpen = false
	s.SceneTextModalID = ""
	s.SceneTextModalSettingsOpen = false
	s.EditorHelpModalOpen = false
	s.NewBookModalOpen = false
	s.NewBookDraftTitle = ""
	s.NewBookDraftAuthor = ""
	s.NewBookDraftMode = ""
	s.NewBookDraftStandard = ""
	s.NewBookDraftNarrated = false
	s.NewBookDraftImageless = false
	s.NewBookDraftDir = ""
	s.AddBookModalOpen = false
	s.ConfirmPendingAction = ""
	s.ConfirmPendingParam = ""
}

func (s *Server) loadBooks() ([]*book.Book, error) {
	if s.StandaloneMode {
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
		}
		if (!s.DevMode || s.StandaloneMode) && s.Templates != nil {
			tpl.StripDevNodesFromTemplates(s.Templates)
		}
	}
}

func NewServer(rootDir string, embeddedFS fs.FS) *Server {
	return NewServerWithOptions(rootDir, embeddedFS, false)
}

func NewStandaloneServer(rootDir string, embeddedFS fs.FS, standaloneBook *book.Book) *Server {
	savesDir := filepath.Join(rootDir, "saves")
	_ = os.MkdirAll(savesDir, 0755)

	settings := book.LoadAppSettings(savesDir)
	settings.PreviewMode = "full"
	settings.ActiveView = "player"

	s := &Server{
		RootDir:           rootDir,
		BooksDir:          filepath.Join(rootDir, "books"),
		SavesDir:          savesDir,
		AppDir:            filepath.Join(rootDir, "app"),
		EmbeddedFS:        embeddedFS,
		DevMode:           false,
		StandaloneMode:    true,
		ShutdownCh:        make(chan struct{}),
		Settings:          settings,
		FocusedHistoryIdx: -1,
	}
	s.ensureTemplatesLoaded()

	if standaloneBook != nil {
		s.StandaloneBookID = standaloneBook.ID
		s.setActiveBook(standaloneBook)
	}
	return s
}

func NewServerWithOptions(rootDir string, embeddedFS fs.FS, devMode bool) *Server {
	booksDir := filepath.Join(rootDir, "books")
	savesDir := filepath.Join(rootDir, "saves")
	if strings.Contains(filepath.ToSlash(rootDir), ".app/Contents/Resources") {
		if cfgDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(cfgDir) != "" {
			savesDir = filepath.Join(cfgDir, "StoryTeller", "saves")
		}
	}
	appDir := filepath.Join(rootDir, "app")
	_ = os.MkdirAll(booksDir, 0755)
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
			s.setActiveBook(b)
		}
	}
	if s.ActiveBook == nil {
		if books, err := s.loadBooks(); err == nil && len(books) > 0 {
			s.setActiveBook(books[0])
		}
	}

	return s
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
	cleanRoot := filepath.Clean(s.RootDir)
	if filepath.Base(cleanRoot) == "dist" {
		parent := filepath.Dir(cleanRoot)
		if info, err := os.Stat(filepath.Join(parent, "go.mod")); err == nil && !info.IsDir() {
			return filepath.Join(parent, "books")
		}
	}
	if strings.Contains(filepath.ToSlash(s.BooksDir), ".app/Contents/Resources") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, "Documents", "StoryTeller")
		}
	}
	return s.BooksDir
}

func (s *Server) saveAppSettings() error {
	if s.Settings == nil {
		return nil
	}
	err := book.SaveAppSettings(s.SavesDir, s.Settings)
	cleanRoot := filepath.Clean(s.RootDir)
	if filepath.Base(cleanRoot) == "dist" {
		parent := filepath.Dir(cleanRoot)
		if info, statErr := os.Stat(filepath.Join(parent, "go.mod")); statErr == nil && !info.IsDir() {
			_ = book.SaveAppSettings(filepath.Join(parent, "saves"), s.Settings)
		}
	}
	return err
}

func (s *Server) setActiveBook(b *book.Book) {
	s.PreviewRevisionID = ""
	s.PreviewRevisionLabel = ""
	s.LiveBook = nil
	s.LiveSave = nil
	s.LiveSelectedSceneID = ""
	s.ActiveBook = b
	s.invalidateHistoryCache()
	s.Settings.ActiveBookID = b.ID
	s.Save = book.LoadSaveState(s.SavesDir, b.ID, b.StartSceneID)
	if s.findScene(s.Save.CurrentSceneID) == nil && len(b.Scenes) > 0 {
		s.Save.CurrentSceneID = b.StartSceneID
		if s.findScene(s.Save.CurrentSceneID) == nil {
			s.Save.CurrentSceneID = b.Scenes[0].ID
		}
	}
	s.HistoryRevealed = false
	s.FocusedHistoryIdx = -1
	s.EditorHistoryPanelOpen = false
	s.EditorBreakdownPanelOpen = false
	s.BookSettingsOpen = false
	s.SelectedChapter = ""
	if len(b.Scenes) > 0 {
		s.SelectedSceneID = s.Save.CurrentSceneID
		if s.findScene(s.SelectedSceneID) == nil {
			s.SelectedSceneID = b.Scenes[0].ID
		}
	}
}

func (s *Server) currentScene() *book.Scene {
	if s.ActiveBook == nil || len(s.ActiveBook.Scenes) == 0 {
		return nil
	}
	if s.Save != nil {
		if sc := s.findScene(s.Save.CurrentSceneID); sc != nil {
			return sc
		}
	}
	if s.ActiveBook.StartSceneID != "" {
		if sc := s.findScene(s.ActiveBook.StartSceneID); sc != nil {
			return sc
		}
	}
	return s.ActiveBook.Scenes[0]
}

func (s *Server) currentAudioTime() float64 {
	if s.Save != nil {
		return s.Save.AudioTime
	}
	return 0
}

func (s *Server) isBookImageless() bool {
	return s.ActiveBook != nil && (s.ActiveBook.Imageless || s.ActiveBook.IsScreenplay())
}

func (s *Server) isEffectiveImageless() bool {
	if s.isBookImageless() {
		return true
	}
	return s.Settings != nil && s.Settings.Imageless
}

func (s *Server) findScene(sceneID string) *book.Scene {
	if s.ActiveBook == nil {
		return nil
	}
	for _, sc := range s.ActiveBook.Scenes {
		if sc.ID == sceneID {
			return sc
		}
	}
	for _, sc := range s.ActiveBook.Scenes {
		if strings.EqualFold(sc.ID, sceneID) || (sc.ExplicitID != "" && strings.EqualFold(sc.ExplicitID, sceneID)) {
			return sc
		}
	}
	for i, sc := range s.ActiveBook.Scenes {
		if fmt.Sprintf("s%d", i+1) == sceneID {
			return sc
		}
	}
	return nil
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
	if view == "player" && s.ActiveBook == nil {
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
			if matchScene(s.findScene(h.SceneID)) {
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
		active := s.ActiveBook
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
	active := s.ActiveBook
	s.mu.RUnlock()
	if active != nil && active.ID == bookID {
		book.ServeBookMedia(w, r, active, parts[1])
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
		if s.activeWorkspaceView() == "library" {
			books, _ = s.loadBooks()
		}
		s.ensureTemplatesLoaded()
		ctx := s.BuildTargetedRenderContext("index", books)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_ = tpl.RenderTemplateTo(w, "index", s.Templates, ctx)
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
		}
	}

	if !s.DevMode || s.StandaloneMode {
		s.assetMu.RLock()
		if s.staticAssets != nil {
			if cached, ok := s.staticAssets[relAppPath]; ok {
				s.assetMu.RUnlock()
				_, _ = w.Write(cached)
				return
			}
		}
		s.assetMu.RUnlock()
	}

	writeAsset := func(data []byte) {
		if (!s.DevMode || s.StandaloneMode) && (strings.HasSuffix(relAppPath, ".css") || strings.HasSuffix(relAppPath, ".js")) {
			data = []byte(tpl.StripDevOnlyBlocks(string(data)))
		}
		if !s.DevMode || s.StandaloneMode {
			s.assetMu.Lock()
			if s.staticAssets == nil {
				s.staticAssets = make(map[string][]byte)
			}
			s.staticAssets[relAppPath] = data
			s.assetMu.Unlock()
		}
		_, _ = w.Write(data)
	}

	if !s.StandaloneMode {
		buildDiskPath := filepath.Join(s.RootDir, "build", "app", filepath.FromSlash(relAppPath))
		if data, err := os.ReadFile(buildDiskPath); err == nil {
			writeAsset(data)
			return
		}
		diskPath := filepath.Join(s.AppDir, filepath.FromSlash(relAppPath))
		if data, err := os.ReadFile(diskPath); err == nil {
			writeAsset(data)
			return
		}
	}
	if s.EmbeddedFS != nil {
		cleanSlash := strings.TrimPrefix(filepath.ToSlash(relAppPath), "/")
		for _, embedPath := range []string{"build/app/" + cleanSlash, "app/" + cleanSlash} {
			if data, err := fs.ReadFile(s.EmbeddedFS, embedPath); err == nil {
				writeAsset(data)
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
