//go:build !(js && wasm)

package server

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mojra/internal/actions"
	"mojra/internal/book"
	"mojra/internal/logging"
	"mojra/internal/qr"
	"mojra/internal/state"
	"mojra/internal/tpl"
	"mojra/internal/views"
)

type assetCacheState struct {
	templatesStamp uint64
	Templates      map[string]*tpl.ParsedTemplate
	assetMu        sync.RWMutex
	cssStamp       uint64
	cachedCSS      []byte
	jsStamp        uint64
	cachedJS       []byte
	staticAssets   map[string][]byte
}

// Server holds the explicit AppState and template/bundle cache for the Mojra HTTP server.
type Server struct {
	State  *state.AppState
	assets assetCacheState
}

const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211
)

func computeFilesStamp(paths []string) uint64 {
	h := fnvOffset64
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return 0
		}
		h ^= uint64(st.ModTime().UnixNano())
		h *= fnvPrime64
		h ^= uint64(st.Size())
		h *= fnvPrime64
	}
	return h
}

func computeDirsStamp(dirs ...string) uint64 {
	h := fnvOffset64
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
			h *= fnvPrime64
			h ^= uint64(info.Size())
			h *= fnvPrime64
		}
	}
	if !found {
		return 0
	}
	return h
}

func (s *Server) ensureTemplatesLoaded() map[string]*tpl.ParsedTemplate {
	cfg := &s.State.Config
	if !cfg.StandaloneMode {
		tplDir := filepath.Join(cfg.AppDir, "templates")
		iconsDir := filepath.Join(cfg.AppDir, "icons")
		stamp := computeDirsStamp(tplDir, iconsDir)
		if s.assets.Templates == nil || (stamp != 0 && stamp != s.assets.templatesStamp) {
			if loaded, err := tpl.LoadTemplatesFromDir(tplDir, cfg.EmbeddedFS); err == nil && len(loaded) > 0 {
				if !cfg.DevMode {
					tpl.StripDevNodesFromTemplates(loaded)
				}
				s.assets.Templates = loaded
				s.assets.templatesStamp = stamp
			}
		}
	}
	if s.assets.Templates == nil {
		tplDir := filepath.Join(cfg.AppDir, "templates")
		if cfg.StandaloneMode {
			tplDir = ""
		}
		s.assets.Templates, _ = tpl.LoadTemplatesFromDir(tplDir, cfg.EmbeddedFS)
		if cfg.StandaloneMode && s.assets.Templates != nil {
			delete(s.assets.Templates, "library_view")
			delete(s.assets.Templates, "editor_view")
			delete(s.assets.Templates, "editor_context_panel")
			delete(s.assets.Templates, "editor_history_panel")
			delete(s.assets.Templates, "editor_breakdown_panel")
			delete(s.assets.Templates, "editor_variables_panel")
		}
		if (!cfg.DevMode || cfg.StandaloneMode) && s.assets.Templates != nil {
			tpl.StripDevNodesFromTemplates(s.assets.Templates)
		}
	}
	return s.assets.Templates
}

func NewServerWithOptions(rootDir string, embeddedFS fs.FS, devMode bool) *Server {
	booksDir := filepath.Join(rootDir, "books")
	savesDir := book.ResolveSavesDir(rootDir)
	appDir := filepath.Join(rootDir, "app")
	if !book.IsInsideRepoDir(rootDir) || book.IsTempDir(rootDir) {
		_ = os.MkdirAll(booksDir, 0755)
	}
	_ = os.MkdirAll(savesDir, 0755)

	tSettings := time.Now()
	settings := book.LoadAppSettings(savesDir)
	logging.Active.MarkStep(logging.StageLoadSettings, tSettings, savesDir)
	if !devMode {
		settings.PreviewMode = "full"
	}
	if book.IsTempDir(rootDir) && !book.IsInsideRepoDir(rootDir) && filepath.Clean(rootDir) != filepath.Clean(book.DefaultAppDataDir()) {
		if settings.NewBookDir == "" {
			settings.NewBookDir = booksDir
		}
		if settings.ExportDir == "" {
			settings.ExportDir = filepath.Join(savesDir, "exports")
		}
	}

	appState := &state.AppState{
		Config: state.RuntimeConfig{
			RootDir:    rootDir,
			BooksDir:   booksDir,
			SavesDir:   savesDir,
			AppDir:     appDir,
			EmbeddedFS: embeddedFS,
			DevMode:    devMode,
			ShutdownCh: make(chan struct{}),
		},
		Settings: settings,
	}
	appState.Player.FocusedHistoryIdx = -1

	s := &Server{
		State: appState,
	}
	tTpl := time.Now()
	s.ensureTemplatesLoaded()
	logging.Active.MarkStep(logging.StageLoadTemplates, tTpl, appDir)

	tLib := time.Now()
	books, _ := book.LoadLibraryBooks(booksDir, settings.ExternalBooks)
	appState.Library.Set(books)
	logging.Active.MarkStep(logging.StageLoadLibrary, tLib, booksDir)

	tPlayer := time.Now()
	if settings.ActiveBookID != "" {
		if b := appState.Library.Get(settings.ActiveBookID); b != nil {
			appState.Player.SetBook(savesDir, settings, b)
		}
	}
	if appState.Player.Book == nil && len(appState.Library.Books) > 0 {
		appState.Player.SetBook(savesDir, settings, appState.Library.Books[0])
	}
	logging.Active.MarkStep(logging.StageInitPlayerState, tPlayer, settings.ActiveBookID)

	tEditor := time.Now()
	if settings.EditorBookID != "" {
		if b := appState.Library.Get(settings.EditorBookID); b != nil && !b.IsPacked {
			appState.Editor.SetBook(b, settings)
			if settings.EditorStudioHome {
				appState.Editor.StudioHome = true
				appState.Editor.SceneText.Close()
			} else {
				appState.Editor.RestoreBookUIState(settings, settings.ActiveView == state.ViewEditor.String())
			}
		}
	}
	logging.Active.MarkStep(logging.StageInitEditorState, tEditor, settings.EditorBookID)

	return s
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

func (s *Server) handleExportScreenplayPDF(w http.ResponseWriter, _ *http.Request) {
	if s.State.Config.StandaloneMode {
		http.Error(w, "forbidden in standalone mode", http.StatusForbidden)
		return
	}
	s.State.Mu.RLock()
	active := s.State.Editor.ActiveBook
	s.State.Mu.RUnlock()
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

func (s *Server) handleExportScreenplayFountain(w http.ResponseWriter, _ *http.Request) {
	if s.State.Config.StandaloneMode {
		http.Error(w, "forbidden in standalone mode", http.StatusForbidden)
		return
	}
	s.State.Mu.RLock()
	active := s.State.Editor.ActiveBook
	s.State.Mu.RUnlock()
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
	if !s.State.Config.DevMode || s.State.Config.StandaloneMode {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lanIP := DetectLANIP()
	port := s.State.Config.Port
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
	if matchScene(b.CurrentScene(s.State.Player.Save)) {
		return true
	}
	if s.State.Player.Save != nil {
		for _, h := range s.State.Player.Save.History {
			if matchScene(b.FindScene(h.SceneID)) {
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

	if s.State.Config.StandaloneMode {
		s.State.Mu.RLock()
		active := s.State.Player.Book
		if active == nil {
			active = s.State.Editor.ActiveBook
		}
		allowed := active != nil && active.ID == bookID && s.isStandaloneMediaUnlocked(active, r.URL.Path, parts[1])
		s.State.Mu.RUnlock()
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

	s.State.Mu.RLock()
	activeEditor := s.State.Editor.ActiveBook
	activePlayer := s.State.Player.Book
	cachedBook := s.State.Library.Get(bookID)
	var extPaths []string
	if s.State.Settings != nil {
		extPaths = s.State.Settings.ExternalBooks
	}
	booksDir := s.State.Config.BooksDir
	s.State.Mu.RUnlock()
	if activePlayer != nil && activePlayer.ID == bookID {
		book.ServeBookMedia(w, r, activePlayer, parts[1])
		return
	}
	if activeEditor != nil && activeEditor.ID == bookID {
		book.ServeBookMedia(w, r, activeEditor, parts[1])
		return
	}
	if cachedBook != nil {
		book.ServeBookMedia(w, r, cachedBook, parts[1])
		return
	}

	b, err := book.FindBookByID(booksDir, extPaths, bookID)
	if err != nil || b == nil {
		http.NotFound(w, r)
		return
	}
	book.ServeBookMedia(w, r, b, parts[1])
}

func (s *Server) handleRootAndAssets(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Path
	if reqPath == "/" || reqPath == "/index.html" {
		tIndex := time.Now()
		s.State.Mu.Lock()
		defer s.State.Mu.Unlock()

		templates := s.ensureTemplatesLoaded()
		ctx := views.NewViewContext(views.LangOf(s.State), views.ViewAppShell(s.State))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if err := tpl.RenderTemplateTo(w, "index", templates, ctx); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		logging.Active.MarkStep(logging.StageHTTPIndexRender, tIndex, reqPath)
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

func (s *Server) serveUIAsset(w http.ResponseWriter, r *http.Request, relAppPath, contentType string) {
	tAsset := time.Now()
	defer logging.Active.MarkAssetOnce(relAppPath, tAsset)
	cfg := &s.State.Config
	w.Header().Set("Content-Type", contentType)
	if strings.HasPrefix(relAppPath, "fonts/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	if !cfg.StandaloneMode {
		stripDev := !cfg.DevMode
		switch relAppPath {
		case "style.css":
			cssFiles := tpl.CSSModuleFiles(cfg.AppDir)
			stamp := computeFilesStamp(cssFiles)
			if stamp != 0 {
				s.assets.assetMu.RLock()
				cached := s.assets.cachedCSS
				cachedStamp := s.assets.cssStamp
				s.assets.assetMu.RUnlock()
				if len(cached) > 0 && stamp == cachedStamp {
					_, _ = w.Write(cached)
					return
				}
				if bundled, err := tpl.BundleFilesToString(cssFiles, stripDev); err == nil {
					raw := []byte(bundled)
					s.assets.assetMu.Lock()
					s.assets.cachedCSS = raw
					s.assets.cssStamp = stamp
					s.assets.assetMu.Unlock()
					_, _ = w.Write(raw)
					return
				}
			}
		case "app.js":
			jsFiles := tpl.JSModuleFiles(cfg.AppDir)
			stamp := computeFilesStamp(jsFiles)
			if stamp != 0 {
				s.assets.assetMu.RLock()
				cached := s.assets.cachedJS
				cachedStamp := s.assets.jsStamp
				s.assets.assetMu.RUnlock()
				if len(cached) > 0 && stamp == cachedStamp {
					_, _ = w.Write(cached)
					return
				}
				if bundled, err := tpl.BundleFilesToString(jsFiles, stripDev); err == nil {
					raw := []byte(bundled)
					s.assets.assetMu.Lock()
					s.assets.cachedJS = raw
					s.assets.jsStamp = stamp
					s.assets.assetMu.Unlock()
					_, _ = w.Write(raw)
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

	s.assets.assetMu.RLock()
	if s.assets.staticAssets != nil {
		if cached, ok := s.assets.staticAssets[relAppPath]; ok {
			s.assets.assetMu.RUnlock()
			_, _ = w.Write(cached)
			return
		}
	}
	s.assets.assetMu.RUnlock()

	writeAsset := func(data []byte) {
		if (!cfg.DevMode || cfg.StandaloneMode) && (strings.HasSuffix(relAppPath, ".css") || strings.HasSuffix(relAppPath, ".js")) {
			data = []byte(tpl.StripDevOnlyBlocks(string(data)))
		}
		s.assets.assetMu.Lock()
		if s.assets.staticAssets == nil {
			s.assets.staticAssets = make(map[string][]byte)
		}
		s.assets.staticAssets[relAppPath] = data
		s.assets.assetMu.Unlock()
		_, _ = w.Write(data)
	}

	if !cfg.StandaloneMode {
		if !cfg.DevMode || (relAppPath != "style.css" && relAppPath != "app.js") {
			buildDiskPath := filepath.Join(cfg.RootDir, "build", "app", filepath.FromSlash(relAppPath))
			if data, err := os.ReadFile(buildDiskPath); err == nil {
				writeAsset(data)
				return
			}
		}
		diskPath := filepath.Join(cfg.AppDir, filepath.FromSlash(relAppPath))
		if data, err := os.ReadFile(diskPath); err == nil {
			writeAsset(data)
			return
		}
	}
	if cfg.EmbeddedFS != nil {
		cleanSlash := strings.TrimPrefix(filepath.ToSlash(relAppPath), "/")
		stripDev := !cfg.DevMode || cfg.StandaloneMode
		if !stripDev {
			switch cleanSlash {
			case "style.css":
				if bundled, err := tpl.BundleCSSFromFS(cfg.EmbeddedFS, false); err == nil {
					writeAsset([]byte(bundled))
					return
				}
			case "app.js":
				if bundled, err := tpl.BundleJSFromFS(cfg.EmbeddedFS, false); err == nil {
					writeAsset([]byte(bundled))
					return
				}
			}
		}
		for _, embedPath := range []string{"build/app/" + cleanSlash, "app/" + cleanSlash} {
			if data, err := fs.ReadFile(cfg.EmbeddedFS, embedPath); err == nil {
				writeAsset(data)
				return
			}
		}
		switch cleanSlash {
		case "style.css":
			if bundled, err := tpl.BundleCSSFromFS(cfg.EmbeddedFS, stripDev); err == nil {
				writeAsset([]byte(bundled))
				return
			}
		case "app.js":
			if bundled, err := tpl.BundleJSFromFS(cfg.EmbeddedFS, stripDev); err == nil {
				writeAsset([]byte(bundled))
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

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.State == nil {
		return
	}
	actions.HandleAction(s.State, w, r, s.ensureTemplatesLoaded)
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
