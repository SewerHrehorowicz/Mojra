//go:build !(js && wasm)

package engine

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

	"mojra/engine/templates"
)

const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211

	routeLANQR    = "/api/lan-qr.png"
	routeAction   = "/api/action"
	routeRoot     = "/"
	routeIndex    = "/index.html"
	routeStyleCSS = "/style.css"
	routeAppJS    = "/app.js"
	prefixIcons   = "/icons/"
	prefixFonts   = "/fonts/"

	subDirTemplates = "templates"
	subDirComponents = "components"
	subDirStyles    = "styles"
	subDirJS        = "js"
	subDirIcons     = "icons"
	subDirFonts     = "fonts"
	subDirBuildApp  = "build/app"

	extSVG  = ".svg"
	extPNG  = ".png"
	extTTF  = ".ttf"
	extCSS  = ".css"
	extJS   = ".js"
	extHTML = ".html"

	mimeHTML = "text/html; charset=utf-8"
	mimeCSS  = "text/css; charset=utf-8"
	mimeJS   = "application/javascript; charset=utf-8"
	mimeSVG  = "image/svg+xml"
	mimePNG  = "image/png"
	mimeTTF  = "font/ttf"

	hdrXContentTypeOpts = "X-Content-Type-Options"
	valNoSniff          = "nosniff"
	hdrXFrameOpts       = "X-Frame-Options"
	valDeny             = "DENY"
	hdrReferrerPolicy   = "Referrer-Policy"
	valNoReferrer       = "no-referrer"
	hdrCOOP             = "Cross-Origin-Opener-Policy"
	valSameOrigin       = "same-origin"
	hdrCORP             = "Cross-Origin-Resource-Policy"
	hdrCOEP             = "Cross-Origin-Embedder-Policy"
	valRequireCorp      = "require-corp"
	hdrCSP              = "Content-Security-Policy"
	valStrictCSP        = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; media-src 'self' blob:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'"
	hdrCacheControl     = "Cache-Control"
	valNoStore          = "no-store"
	valCacheImmutable   = "public, max-age=86400"

	errQRGenFailed  = "qr generation failed"
	lanQRURLFmt     = "http://%s:%d"
	defaultLoopback = "127.0.0.1"
	appSubPathPref  = "app/"
)

// EngineConfig holds runtime configuration required by the generic HTTP engine.
type EngineConfig struct {
	RootDir        string
	AppDir         string
	EmbeddedFS     fs.FS
	DevMode        bool
	StandaloneMode bool
	Port           int
}

type assetCacheState struct {
	mu                sync.RWMutex
	Templates         map[string]*templates.ParsedTemplate
	templatesStamp    uint64
	cssBundleCache    []byte
	cssBundleStamp    uint64
	jsBundleCache     []byte
	jsBundleStamp     uint64
	embeddedIndexOnce sync.Once
	embeddedIndexHTML []byte
	embeddedCSSOnce   sync.Once
	embeddedCSSBytes  []byte
	embeddedJSOnce    sync.Once
	embeddedJSBytes   []byte
}

// EngineServer serves compiled .tpl views, bundled .tss/.js assets, security headers, LAN QR, and POST /api/action.
type EngineServer struct {
	Config                      func() EngineConfig
	RenderRootView              func(writer http.ResponseWriter, loadedTemplates map[string]*templates.ParsedTemplate)
	HandleAction                func(writer http.ResponseWriter, request *http.Request, ensureTemplates func() map[string]*templates.ParsedTemplate)
	CustomRoutes                map[string]http.HandlerFunc
	ExcludedStandaloneTemplates []string
	CSPExemptPrefix             string
	assets                      assetCacheState
}

func computeFilesStamp(files []string) uint64 {
	var h uint64 = fnvOffset64
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		h ^= uint64(info.ModTime().UnixNano())
		h *= fnvPrime64
		h ^= uint64(info.Size())
		h *= fnvPrime64
	}
	return h
}

func computeDirsStamp(dirs ...string) uint64 {
	var h uint64 = fnvOffset64
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				subEntries, subErr := os.ReadDir(filepath.Join(dir, e.Name()))
				if subErr != nil {
					continue
				}
				for _, se := range subEntries {
					if info, iErr := se.Info(); iErr == nil {
						h ^= uint64(info.ModTime().UnixNano())
						h *= fnvPrime64
						h ^= uint64(info.Size())
						h *= fnvPrime64
					}
				}
				continue
			}
			if info, iErr := e.Info(); iErr == nil {
				h ^= uint64(info.ModTime().UnixNano())
				h *= fnvPrime64
				h ^= uint64(info.Size())
				h *= fnvPrime64
			}
		}
	}
	return h
}

// EnsureTemplatesLoaded parses and caches .tpl templates, hot-reloading in DevMode only when mtimes/sizes change.
func (server *EngineServer) EnsureTemplatesLoaded() map[string]*templates.ParsedTemplate {
	cfg := server.Config()
	tplDir := filepath.Join(cfg.AppDir, subDirTemplates)
	compDir := filepath.Join(cfg.AppDir, subDirComponents)

	server.assets.mu.RLock()
	curTpl := server.assets.Templates
	curStamp := server.assets.templatesStamp
	server.assets.mu.RUnlock()

	if !cfg.DevMode && curTpl != nil {
		return curTpl
	}

	newStamp := computeDirsStamp(tplDir, compDir)
	if curTpl != nil && newStamp != 0 && newStamp == curStamp {
		return curTpl
	}

	loaded, err := templates.LoadTemplatesFromDir(tplDir, cfg.EmbeddedFS)
	if err != nil || loaded == nil {
		return curTpl
	}
	if cfg.StandaloneMode {
		for _, ex := range server.ExcludedStandaloneTemplates {
			delete(loaded, ex)
		}
	}
	if !cfg.DevMode {
		StripDevNodesFromTemplates(loaded)
	}

	server.assets.mu.Lock()
	server.assets.Templates = loaded
	server.assets.templatesStamp = newStamp
	server.assets.mu.Unlock()
	return loaded
}

// Handler builds the HTTP mux with security headers, custom routes, QR endpoint, action dispatcher, and UI asset server.
func (server *EngineServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(routeLANQR, server.handleLANQR)
	for pattern, routeHandler := range server.CustomRoutes {
		mux.HandleFunc(pattern, routeHandler)
	}
	if server.HandleAction != nil {
		mux.HandleFunc(routeAction, func(writer http.ResponseWriter, request *http.Request) {
			server.HandleAction(writer, request, server.EnsureTemplatesLoaded)
		})
	}
	mux.HandleFunc(routeRoot, server.handleRootAndAssets)
	return server.withSecurityHeaders(mux)
}

func (server *EngineServer) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		hdr := writer.Header()
		hdr.Set(hdrXContentTypeOpts, valNoSniff)
		hdr.Set(hdrXFrameOpts, valDeny)
		hdr.Set(hdrReferrerPolicy, valNoReferrer)
		hdr.Set(hdrCOOP, valSameOrigin)
		hdr.Set(hdrCORP, valSameOrigin)
		hdr.Set(hdrCOEP, valRequireCorp)
		if server.CSPExemptPrefix == "" || !strings.HasPrefix(request.URL.Path, server.CSPExemptPrefix) {
			hdr.Set(hdrCSP, valStrictCSP)
		}
		next.ServeHTTP(writer, request)
	})
}

func (server *EngineServer) handleLANQR(writer http.ResponseWriter, request *http.Request) {
	cfg := server.Config()
	if !cfg.DevMode {
		http.NotFound(writer, request)
		return
	}
	tQR := time.Now()
	defer Active.MarkStep(StageHTTPAssetServe, tQR, routeLANQR)
	lanURL := fmt.Sprintf(lanQRURLFmt, DetectLANIP(), cfg.Port)
	pngBytes, err := EncodePNG(lanURL, 6)
	if err != nil {
		http.Error(writer, errQRGenFailed, http.StatusInternalServerError)
		return
	}
	writer.Header().Set(headerContentType, mimePNG)
	writer.Header().Set(hdrCacheControl, valNoStore)
	_, _ = writer.Write(pngBytes)
}

func (server *EngineServer) handleRootAndAssets(writer http.ResponseWriter, request *http.Request) {
	tAsset := time.Now()
	cfg := server.Config()
	cleanPath := filepath.ToSlash(filepath.Clean(request.URL.Path))
	if cleanPath == "." {
		cleanPath = routeRoot
	}
	defer func() {
		if cleanPath != routeRoot && cleanPath != routeIndex {
			Active.MarkStep(StageHTTPAssetServe, tAsset, cleanPath)
		}
	}()

	if cleanPath == routeRoot || cleanPath == routeIndex {
		tEnsure := time.Now()
		loadedTemplates := server.EnsureTemplatesLoaded()
		Active.MarkStep(StageLoadTemplates, tEnsure, "")
		if loadedTemplates != nil && server.RenderRootView != nil {
			writer.Header().Set(headerContentType, mimeHTML)
			writer.Header().Set(hdrCacheControl, valNoStore)
			server.RenderRootView(writer, loadedTemplates)
			return
		}
		server.serveUIAsset(writer, request, routeIndex, mimeHTML)
		return
	}

	switch cleanPath {
	case routeStyleCSS:
		server.serveUIAsset(writer, request, routeStyleCSS, mimeCSS)
		return
	case routeAppJS:
		server.serveUIAsset(writer, request, routeAppJS, mimeJS)
		return
	}

	relAssetName := strings.TrimPrefix(cleanPath, routeRoot)
	if strings.HasSuffix(cleanPath, extPNG) && !strings.Contains(relAssetName, routeRoot) {
		server.serveUIAsset(writer, request, cleanPath, mimePNG)
		return
	}

	if strings.HasPrefix(cleanPath, prefixIcons) && (strings.HasSuffix(cleanPath, extSVG) || strings.HasSuffix(cleanPath, extPNG)) {
		iconName := filepath.Base(cleanPath)
		ct := mimeSVG
		if strings.HasSuffix(cleanPath, extPNG) {
			ct = mimePNG
		}
		diskIcon := filepath.Join(cfg.AppDir, subDirIcons, iconName)
		if info, err := os.Stat(diskIcon); err == nil && !info.IsDir() {
			writer.Header().Set(headerContentType, ct)
			http.ServeFile(writer, request, diskIcon)
			return
		}
		if cfg.EmbeddedFS != nil {
			if data, err := fs.ReadFile(cfg.EmbeddedFS, appSubPathPref+subDirIcons+"/"+iconName); err == nil {
				writer.Header().Set(headerContentType, ct)
				_, _ = writer.Write(data)
				return
			}
		}
	}

	if strings.HasPrefix(cleanPath, prefixFonts) && strings.HasSuffix(cleanPath, extTTF) {
		fontName := filepath.Base(cleanPath)
		diskFont := filepath.Join(cfg.AppDir, subDirFonts, fontName)
		if info, err := os.Stat(diskFont); err == nil && !info.IsDir() {
			writer.Header().Set(headerContentType, mimeTTF)
			writer.Header().Set(hdrCacheControl, valCacheImmutable)
			http.ServeFile(writer, request, diskFont)
			return
		}
		if cfg.EmbeddedFS != nil {
			if data, err := fs.ReadFile(cfg.EmbeddedFS, appSubPathPref+subDirFonts+"/"+fontName); err == nil {
				writer.Header().Set(headerContentType, mimeTTF)
				writer.Header().Set(hdrCacheControl, valCacheImmutable)
				_, _ = writer.Write(data)
				return
			}
		}
	}

	http.NotFound(writer, request)
}

func (server *EngineServer) serveUIAsset(writer http.ResponseWriter, request *http.Request, relPath string, contentType string) {
	cfg := server.Config()
	cleanRel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(relPath)), routeRoot)

	switch cleanRel {
	case strings.TrimPrefix(routeStyleCSS, routeRoot):
		var cssFiles []string
		if cfg.StandaloneMode {
			cssFiles = CSSPlayerModuleFiles(cfg.AppDir)
		} else {
			cssFiles = CSSModuleFiles(cfg.AppDir)
		}
		stamp := computeFilesStamp(cssFiles)
		if stamp != 0 {
			server.assets.mu.RLock()
			cached := server.assets.cssBundleCache
			cachedStamp := server.assets.cssBundleStamp
			server.assets.mu.RUnlock()
			if len(cached) > 0 && stamp == cachedStamp {
				writer.Header().Set(headerContentType, contentType)
				if cfg.DevMode {
					writer.Header().Set(hdrCacheControl, valNoStore)
				}
				_, _ = writer.Write(cached)
				return
			}
			if bundled, err := BundleFilesToString(cssFiles, !cfg.DevMode); err == nil && len(bundled) > 0 {
				rawBytes := []byte(bundled)
				server.assets.mu.Lock()
				server.assets.cssBundleCache = rawBytes
				server.assets.cssBundleStamp = stamp
				server.assets.mu.Unlock()
				writer.Header().Set(headerContentType, contentType)
				if cfg.DevMode {
					writer.Header().Set(hdrCacheControl, valNoStore)
				}
				_, _ = writer.Write(rawBytes)
				return
			}
		}
	case strings.TrimPrefix(routeAppJS, routeRoot):
		var jsFiles []string
		if cfg.StandaloneMode {
			jsFiles = JSPlayerModuleFiles(cfg.AppDir)
		} else {
			jsFiles = JSModuleFiles(cfg.AppDir)
		}
		stamp := computeFilesStamp(jsFiles)
		if stamp != 0 {
			server.assets.mu.RLock()
			cached := server.assets.jsBundleCache
			cachedStamp := server.assets.jsBundleStamp
			server.assets.mu.RUnlock()
			if len(cached) > 0 && stamp == cachedStamp {
				writer.Header().Set(headerContentType, contentType)
				if cfg.DevMode {
					writer.Header().Set(hdrCacheControl, valNoStore)
				}
				_, _ = writer.Write(cached)
				return
			}
			if bundled, err := BundleFilesToString(jsFiles, !cfg.DevMode); err == nil && len(bundled) > 0 {
				rawBytes := []byte(bundled)
				server.assets.mu.Lock()
				server.assets.jsBundleCache = rawBytes
				server.assets.jsBundleStamp = stamp
				server.assets.mu.Unlock()
				writer.Header().Set(headerContentType, contentType)
				if cfg.DevMode {
					writer.Header().Set(hdrCacheControl, valNoStore)
				}
				_, _ = writer.Write(rawBytes)
				return
			}
		}
	}

	if cfg.DevMode {
		if buildFn, ok := GeneratedAssetBuilders[cleanRel]; ok && buildFn != nil {
			if pngBytes, err := buildFn(); err == nil && len(pngBytes) > 0 {
				writer.Header().Set(headerContentType, contentType)
				writer.Header().Set(hdrCacheControl, valNoStore)
				_, _ = writer.Write(pngBytes)
				return
			}
		}
		if strings.HasSuffix(cleanRel, extPNG) {
			srcPNG := filepath.Join(cfg.AppDir, cleanRel)
			if info, err := os.Stat(srcPNG); err == nil && !info.IsDir() {
				writer.Header().Set(headerContentType, contentType)
				writer.Header().Set(hdrCacheControl, valNoStore)
				http.ServeFile(writer, request, srcPNG)
				return
			}
		}
	}

	if !cfg.StandaloneMode {
		builtCandidate := filepath.Join(cfg.RootDir, filepath.FromSlash(subDirBuildApp), filepath.FromSlash(cleanRel))
		if info, err := os.Stat(builtCandidate); err == nil && !info.IsDir() {
			if !cfg.DevMode && (strings.HasSuffix(cleanRel, extCSS) || strings.HasSuffix(cleanRel, extJS) || strings.HasSuffix(cleanRel, extHTML)) {
				if raw, rErr := os.ReadFile(builtCandidate); rErr == nil {
					cleaned := templates.StripDevOnlyBlocks(string(raw))
					writer.Header().Set(headerContentType, contentType)
					_, _ = writer.Write([]byte(cleaned))
					return
				}
			}
			writer.Header().Set(headerContentType, contentType)
			http.ServeFile(writer, request, builtCandidate)
			return
		}
	}

	if cfg.DevMode {
		srcCandidate := filepath.Join(cfg.AppDir, filepath.FromSlash(cleanRel))
		if info, err := os.Stat(srcCandidate); err == nil && !info.IsDir() {
			writer.Header().Set(headerContentType, contentType)
			http.ServeFile(writer, request, srcCandidate)
			return
		}
	}

	if buildFn, ok := GeneratedAssetBuilders[cleanRel]; ok && buildFn != nil {
		if pngBytes, err := buildFn(); err == nil && len(pngBytes) > 0 {
			writer.Header().Set(headerContentType, contentType)
			_, _ = writer.Write(pngBytes)
			return
		}
	}

	if cfg.EmbeddedFS != nil {
		switch cleanRel {
		case strings.TrimPrefix(routeIndex, routeRoot):
			server.assets.embeddedIndexOnce.Do(func() {
				if loadedTpl := server.EnsureTemplatesLoaded(); loadedTpl != nil {
					if idxTpl := loadedTpl[strings.TrimSuffix(cleanRel, extHTML)]; idxTpl != nil {
						server.assets.embeddedIndexHTML = []byte(templates.TranspileStaticHTML(idxTpl, loadedTpl))
					}
				}
			})
			if len(server.assets.embeddedIndexHTML) > 0 {
				writer.Header().Set(headerContentType, contentType)
				_, _ = writer.Write(server.assets.embeddedIndexHTML)
				return
			}
		case strings.TrimPrefix(routeStyleCSS, routeRoot):
			server.assets.embeddedCSSOnce.Do(func() {
				var bundled string
				var err error
				if cfg.StandaloneMode {
					bundled, err = BundlePlayerCSSFromFS(cfg.EmbeddedFS, !cfg.DevMode)
				} else {
					bundled, err = BundleCSSFromFS(cfg.EmbeddedFS, !cfg.DevMode)
				}
				if err == nil {
					server.assets.embeddedCSSBytes = []byte(bundled)
				}
			})
			if len(server.assets.embeddedCSSBytes) > 0 {
				writer.Header().Set(headerContentType, contentType)
				_, _ = writer.Write(server.assets.embeddedCSSBytes)
				return
			}
		case strings.TrimPrefix(routeAppJS, routeRoot):
			server.assets.embeddedJSOnce.Do(func() {
				var bundled string
				var err error
				if cfg.StandaloneMode {
					bundled, err = BundlePlayerJSFromFS(cfg.EmbeddedFS, !cfg.DevMode)
				} else {
					bundled, err = BundleJSFromFS(cfg.EmbeddedFS, !cfg.DevMode)
				}
				if err == nil {
					server.assets.embeddedJSBytes = []byte(bundled)
				}
			})
			if len(server.assets.embeddedJSBytes) > 0 {
				writer.Header().Set(headerContentType, contentType)
				_, _ = writer.Write(server.assets.embeddedJSBytes)
				return
			}
		}

		for _, embPath := range []string{
			subDirBuildApp + "/" + cleanRel,
			appSubPathPref + cleanRel,
		} {
			if data, err := fs.ReadFile(cfg.EmbeddedFS, embPath); err == nil {
				if !cfg.DevMode && (strings.HasSuffix(cleanRel, extCSS) || strings.HasSuffix(cleanRel, extJS) || strings.HasSuffix(cleanRel, extHTML)) {
					data = []byte(templates.StripDevOnlyBlocks(string(data)))
				}
				writer.Header().Set(headerContentType, contentType)
				_, _ = writer.Write(data)
				return
			}
		}
	}

	http.NotFound(writer, request)
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
	return defaultLoopback
}
