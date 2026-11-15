//go:build !(js && wasm)

package transport

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"mojra/engine"
	"mojra/engine/templates"
)

const (
	maxActionBodyBytes    = 32 << 20
	fdpMimePrefix         = "text/x-fdp"
	headerContentType     = "Content-Type"
	headerCacheControl    = "Cache-Control"
	headerSecFetchSite    = "Sec-Fetch-Site"
	headerOrigin          = "Origin"
	headerReferer         = "Referer"
	secFetchSameOrigin    = "same-origin"
	secFetchNone          = "none"
	originNull            = "null"
	hostLocalhost         = "localhost"
	cacheNoStore          = "no-store"
	cacheImmutableAsset   = "public, max-age=31536000, immutable"
	mimeHTMLUTF8          = "text/html; charset=utf-8"
	mimeCSSUTF8           = "text/css; charset=utf-8"
	mimeJSUTF8            = "application/javascript; charset=utf-8"
	mimePNG               = "image/png"
	mimeSVG               = "image/svg+xml"
	mimeTTF               = "font/ttf"
	routeStyleCSS         = "/style.css"
	routeAppJS            = "/app.js"
	routeLANQR            = "/api/lan-qr.png"
	routeIconPNG          = "/icon.png"
	routeFaviconICO       = "/favicon.ico"
	routeIconsPrefix      = "/icons/"
	routeFontsPrefix      = "/fonts/"
	routeAPIAction        = "/api/action"
	routeRoot             = "/"
	extSVG                = ".svg"
	extPNG                = ".png"
	extTTF                = ".ttf"
	errMethodNotAllowed   = "Method not allowed"
	errTemplateCompileFmt = "Template compile error: %v"
	errQRGenFailed        = "qr generation failed"
	lanQRURLFmt           = "http://%s:%d"
	hdrXContentTypeOpts   = "X-Content-Type-Options"
	valNoSniff            = "nosniff"
	hdrXFrameOpts         = "X-Frame-Options"
	valDeny               = "DENY"
	hdrReferrerPolicy     = "Referrer-Policy"
	valNoReferrer         = "no-referrer"
	hdrCOOP               = "Cross-Origin-Opener-Policy"
	valSameOrigin         = "same-origin"
	hdrCORP               = "Cross-Origin-Resource-Policy"
	hdrCOEP               = "Cross-Origin-Embedder-Policy"
	valRequireCorp        = "require-corp"
	hdrCSP                = "Content-Security-Policy"
	valStrictCSP          = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; media-src 'self' blob:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'"
)

// GeneratedAssetBuilders maps virtual asset names (e.g. "studio-ui.png") to build-on-demand generator functions.
var GeneratedAssetBuilders = map[string]func() ([]byte, error){}

func isFDPRequestPayload(contentType string, raw []byte) bool {
	lowerCT := strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(lowerCT, fdpMimePrefix) {
		return true
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	return bytes.IndexByte(trimmed, '\n') >= 0 || !bytes.Contains(trimmed, []byte("&"))
}

// ParseActionRequest decodes an incoming action payload (FDP or URL-encoded form) into request.Form and request.PostForm.
func ParseActionRequest(request *http.Request) {
	if request == nil || request.Body == nil {
		return
	}
	request.Body = http.MaxBytesReader(nil, request.Body, maxActionBodyBytes)
	contentType := request.Header.Get(headerContentType)
	buf := templates.AcquireBuffer()
	defer templates.ReleaseBuffer(buf)
	if _, copyErr := io.Copy(buf, request.Body); copyErr != nil {
		return
	}
	raw := buf.Bytes()
	if isFDPRequestPayload(contentType, raw) {
		vals := make(url.Values, 16)
		if DecodeFDPBytes(raw, vals) {
			request.Form = vals
			request.PostForm = vals
			return
		}
	}
	request.Body = io.NopCloser(bytes.NewReader(raw))
	_ = request.ParseForm()
}

// ValidateSameOriginAction verifies Sec-Fetch-Site, Origin, and Referer headers against request.Host.
func ValidateSameOriginAction(request *http.Request) bool {
	if secSite := strings.ToLower(strings.TrimSpace(request.Header.Get(headerSecFetchSite))); secSite != "" {
		if secSite != secFetchSameOrigin && secSite != secFetchNone {
			return false
		}
	}
	if origin := strings.TrimSpace(request.Header.Get(headerOrigin)); origin != "" {
		if strings.EqualFold(origin, originNull) {
			return false
		}
		parsedOrigin, parseErr := url.Parse(origin)
		if parseErr != nil || parsedOrigin.Host == "" || !strings.EqualFold(parsedOrigin.Host, request.Host) {
			return false
		}
	}
	if referer := strings.TrimSpace(request.Header.Get(headerReferer)); referer != "" {
		parsedReferer, parseErr := url.Parse(referer)
		if parseErr != nil || parsedReferer.Host == "" || !strings.EqualFold(parsedReferer.Host, request.Host) {
			return false
		}
	}
	return true
}

// IsLoopbackRequest reports whether request originated from a loopback address.
func IsLoopbackRequest(request *http.Request) bool {
	if request == nil {
		return true
	}
	remoteAddr := strings.TrimSpace(request.RemoteAddr)
	if remoteAddr == "" {
		return true
	}
	host := remoteAddr
	if splitHost, _, splitErr := net.SplitHostPort(remoteAddr); splitErr == nil {
		host = splitHost
	} else {
		host = strings.Trim(remoteAddr, "[]")
	}
	if strings.EqualFold(host, hostLocalhost) {
		return true
	}
	parsedIP := net.ParseIP(host)
	return parsedIP != nil && parsedIP.IsLoopback()
}

// WriteRenderedActionFrames renders the given ActionFrames and global script chunks into a multi-frame FDP HTTP response.
func WriteRenderedActionFrames(
	writer http.ResponseWriter,
	lang engine.Lang,
	actionFrames []ActionFrame,
	globalScriptChunks []templates.TemplateChunk,
	loadedTemplates map[string]*templates.ParsedTemplate,
) {
	tRender := time.Now()
	fdpFrames, renderErr := RenderActionFrames(lang, actionFrames, globalScriptChunks, loadedTemplates)
	if renderErr != nil {
		http.Error(writer, renderErr.Error(), http.StatusInternalServerError)
		return
	}
	engine.Active.MarkStep(engine.StageActionRenderTpl, tRender, fmt.Sprintf("frames=%d", len(fdpFrames)))
	writer.Header().Set(headerContentType, ContentTypeFDP)
	writer.Header().Set(headerCacheControl, cacheNoStore)
	_ = WriteFDPFrames(writer, fdpFrames...)
}

// EngineConfig configures the generic EngineServer with application-provided module filesystems and callbacks.
type EngineConfig struct {
	RootDir          string
	EngineFS         fs.FS
	ModuleFSList     []fs.FS
	ModuleSubDirs    []string
	ModuleRootAssets []string
	DevMode          bool
	StandaloneMode   bool
	CSPExemptPrefix  string
	RenderRootView   func(writer io.Writer, parsed map[string]*templates.ParsedTemplate) error
	HandleAction     func(writer http.ResponseWriter, req *http.Request)
	RegisterRoutes   func(mux *http.ServeMux)
	ResolveIconBytes func() ([]byte, bool)
	ShutdownCh       <-chan struct{}
}

// EngineServer manages template/CSS/JS caching, static asset serving, and HTTP routing.
type EngineServer struct {
	cfg            EngineConfig
	tplMu          sync.RWMutex
	templates      map[string]*templates.ParsedTemplate
	templatesStamp int64
	templatesSize  int64
	cssCache       []byte
	cssStamp       int64
	cssSize        int64
	jsCache        []byte
	jsStamp        int64
	jsSize         int64
	embeddedLoaded bool
	httpHandler    http.Handler
}

// NewEngineServer creates and initializes an EngineServer with prewarmed templates and bundles.
func NewEngineServer(cfg EngineConfig) *EngineServer {
	srv := &EngineServer{cfg: cfg}
	srv.PrewarmServerTemplates()
	srv.httpHandler = srv.buildMux()
	return srv
}

// Handler returns the configured HTTP handler for this EngineServer.
func (srv *EngineServer) Handler() http.Handler {
	return srv.httpHandler
}

// PrewarmServerTemplates compiles templates and bundles CSS/JS ahead of time.
func (srv *EngineServer) PrewarmServerTemplates() {
	_, _ = srv.LoadServerTemplates()
	_, _, _, _ = srv.loadAndCacheAssets()
}

func (srv *EngineServer) activeModules() ([]fs.FS, fs.FS) {
	if !srv.cfg.StandaloneMode && srv.cfg.RootDir != "" && len(srv.cfg.ModuleSubDirs) > 0 {
		modList, diskEngineFS := templates.BuildDiskOrEmbeddedModules(srv.cfg.RootDir, srv.cfg.ModuleSubDirs, srv.cfg.ModuleFSList)
		if diskEngineFS == nil {
			diskEngineFS = srv.cfg.EngineFS
		}
		return modList, diskEngineFS
	}
	return srv.cfg.ModuleFSList, srv.cfg.EngineFS
}

// LoadServerTemplates returns cached compiled templates, reloading only when disk files change.
func (srv *EngineServer) LoadServerTemplates() (map[string]*templates.ParsedTemplate, error) {
	if !srv.cfg.DevMode {
		srv.tplMu.RLock()
		if srv.templates != nil && srv.embeddedLoaded {
			cached := srv.templates
			srv.tplMu.RUnlock()
			return cached, nil
		}
		srv.tplMu.RUnlock()
	}

	mtime, size := templates.ComputeModulesDiskStamp(srv.cfg.RootDir, srv.cfg.ModuleSubDirs)

	srv.tplMu.RLock()
	if srv.templates != nil && (srv.embeddedLoaded || (srv.templatesStamp == mtime && srv.templatesSize == size)) {
		cached := srv.templates
		srv.tplMu.RUnlock()
		return cached, nil
	}
	srv.tplMu.RUnlock()

	srv.tplMu.Lock()
	defer srv.tplMu.Unlock()
	if srv.templates != nil && (srv.embeddedLoaded || (srv.templatesStamp == mtime && srv.templatesSize == size)) {
		return srv.templates, nil
	}

	startLoad := time.Now()
	modList, _ := srv.activeModules()
	parsed, loadErr := templates.LoadTemplatesFromModules(modList)
	if loadErr != nil {
		return nil, loadErr
	}
	if !srv.cfg.DevMode {
		templates.StripDevNodesFromTemplates(parsed)
	}
	srv.templates = parsed
	srv.templatesStamp = mtime
	srv.templatesSize = size
	if !srv.cfg.DevMode {
		srv.embeddedLoaded = true
	}
	engine.Active.MarkStep(engine.StageLoadTemplates, startLoad, fmt.Sprintf("templates=%d", len(parsed)))
	return srv.templates, nil
}

func (srv *EngineServer) loadAndCacheAssets() (map[string]*templates.ParsedTemplate, []byte, []byte, error) {
	parsed, tplErr := srv.LoadServerTemplates()
	if tplErr != nil {
		return nil, nil, nil, tplErr
	}

	srv.tplMu.RLock()
	stamp := srv.templatesStamp
	size := srv.templatesSize
	if srv.cssCache != nil && srv.jsCache != nil && (srv.embeddedLoaded || (srv.cssStamp == stamp && srv.cssSize == size && srv.jsStamp == stamp && srv.jsSize == size)) {
		css := srv.cssCache
		js := srv.jsCache
		srv.tplMu.RUnlock()
		return parsed, css, js, nil
	}
	srv.tplMu.RUnlock()

	srv.tplMu.Lock()
	defer srv.tplMu.Unlock()
	if srv.cssCache != nil && srv.jsCache != nil && (srv.embeddedLoaded || (srv.cssStamp == stamp && srv.cssSize == size && srv.jsStamp == stamp && srv.jsSize == size)) {
		return parsed, srv.cssCache, srv.jsCache, nil
	}

	prodMode := !srv.cfg.DevMode
	modList, activeEngineFS := srv.activeModules()

	startCSS := time.Now()
	cssStr, cssErr := templates.BundleModulesCSS(modList, prodMode)
	if cssErr != nil {
		return nil, nil, nil, cssErr
	}
	srv.cssCache = []byte(cssStr)
	srv.cssStamp = stamp
	srv.cssSize = size
	engine.Active.MarkStep(engine.StageHTTPAssetServe, startCSS, fmt.Sprintf("bytes=%d", len(srv.cssCache)))

	startJS := time.Now()
	jsStr, jsErr := templates.BundleModulesJS(activeEngineFS, modList, prodMode)
	if jsErr != nil {
		return nil, nil, nil, jsErr
	}
	srv.jsCache = []byte(jsStr)
	srv.jsStamp = stamp
	srv.jsSize = size
	engine.Active.MarkStep(engine.StageHTTPAssetServe, startJS, fmt.Sprintf("bytes=%d", len(srv.jsCache)))

	return parsed, srv.cssCache, srv.jsCache, nil
}

func (srv *EngineServer) buildMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(routeStyleCSS, srv.serveUIAsset(routeStyleCSS, mimeCSSUTF8))
	mux.HandleFunc(routeAppJS, srv.serveUIAsset(routeAppJS, mimeJSUTF8))
	for _, assetName := range srv.cfg.ModuleRootAssets {
		cleanAsset := strings.TrimPrefix(assetName, "/")
		if cleanAsset != "" {
			mux.HandleFunc("/"+cleanAsset, srv.serveModuleRootAsset(cleanAsset))
		}
	}
	for genName := range GeneratedAssetBuilders {
		cleanGen := strings.TrimPrefix(genName, "/")
		if cleanGen != "" {
			mux.HandleFunc("/"+cleanGen, srv.serveGeneratedAsset(cleanGen))
		}
	}
	mux.HandleFunc(routeLANQR, srv.handleLANQR)
	mux.HandleFunc(routeIconPNG, srv.serveAppIcon)
	mux.HandleFunc(routeFaviconICO, srv.serveAppIcon)
	mux.HandleFunc(routeIconsPrefix, srv.serveStaticIcon)
	mux.HandleFunc(routeFontsPrefix, srv.serveStaticFont)
	if srv.cfg.HandleAction != nil {
		mux.HandleFunc(routeAPIAction, srv.cfg.HandleAction)
	}
	if srv.cfg.RegisterRoutes != nil {
		srv.cfg.RegisterRoutes(mux)
	}
	mux.HandleFunc(routeRoot, srv.serveIndex)
	return srv.withSecurityHeaders(mux)
}

func (srv *EngineServer) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		hdr := writer.Header()
		hdr.Set(hdrXContentTypeOpts, valNoSniff)
		hdr.Set(hdrXFrameOpts, valDeny)
		hdr.Set(hdrReferrerPolicy, valNoReferrer)
		hdr.Set(hdrCOOP, valSameOrigin)
		hdr.Set(hdrCORP, valSameOrigin)
		hdr.Set(hdrCOEP, valRequireCorp)
		if srv.cfg.CSPExemptPrefix == "" || !strings.HasPrefix(req.URL.Path, srv.cfg.CSPExemptPrefix) {
			hdr.Set(hdrCSP, valStrictCSP)
		}
		next.ServeHTTP(writer, req)
	})
}

func (srv *EngineServer) serveGeneratedAsset(assetName string) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if buildFn, ok := GeneratedAssetBuilders[assetName]; ok && buildFn != nil {
			if pngBytes, err := buildFn(); err == nil && len(pngBytes) > 0 {
				writer.Header().Set(headerContentType, mimePNG)
				writer.Header().Set(headerCacheControl, cacheImmutableAsset)
				_, _ = writer.Write(pngBytes)
				return
			}
		}
		http.NotFound(writer, nil)
	}
}

func (srv *EngineServer) handleLANQR(writer http.ResponseWriter, req *http.Request) {
	if !srv.cfg.DevMode {
		http.NotFound(writer, req)
		return
	}
	tQR := time.Now()
	defer engine.Active.MarkStep(engine.StageHTTPAssetServe, tQR, routeLANQR)
	lanURL := fmt.Sprintf(lanQRURLFmt, req.Host, 8080)
	pngBytes, err := engine.EncodePNG(lanURL, 6)
	if err != nil {
		http.Error(writer, errQRGenFailed, http.StatusInternalServerError)
		return
	}
	writer.Header().Set(headerContentType, mimePNG)
	writer.Header().Set(headerCacheControl, cacheNoStore)
	_, _ = writer.Write(pngBytes)
}

func (srv *EngineServer) serveModuleRootAsset(assetName string) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		modList, _ := srv.activeModules()
		if data, found := templates.ReadModuleAsset(modList, assetName); found {
			writer.Header().Set(headerContentType, mimePNG)
			writer.Header().Set(headerCacheControl, cacheImmutableAsset)
			_, _ = writer.Write(data)
			return
		}
		http.NotFound(writer, nil)
	}
}

func (srv *EngineServer) serveAppIcon(writer http.ResponseWriter, _ *http.Request) {
	if srv.cfg.ResolveIconBytes != nil {
		if data, ok := srv.cfg.ResolveIconBytes(); ok && len(data) > 0 {
			writer.Header().Set(headerContentType, mimePNG)
			writer.Header().Set(headerCacheControl, cacheImmutableAsset)
			_, _ = writer.Write(data)
			return
		}
	}
	http.NotFound(writer, nil)
}

func (srv *EngineServer) serveStaticIcon(writer http.ResponseWriter, req *http.Request) {
	name := path.Base(req.URL.Path)
	isSVG := strings.HasSuffix(name, extSVG)
	isPNG := strings.HasSuffix(name, extPNG)
	if (!isSVG && !isPNG) || strings.Contains(name, "..") {
		http.NotFound(writer, req)
		return
	}
	modList, _ := srv.activeModules()
	if data, found := templates.ReadModuleAsset(modList, path.Join("icons", name)); found {
		if isPNG {
			writer.Header().Set(headerContentType, mimePNG)
		} else {
			writer.Header().Set(headerContentType, mimeSVG)
		}
		writer.Header().Set(headerCacheControl, cacheImmutableAsset)
		_, _ = writer.Write(data)
		return
	}
	http.NotFound(writer, req)
}

func (srv *EngineServer) serveStaticFont(writer http.ResponseWriter, req *http.Request) {
	name := path.Base(req.URL.Path)
	if !strings.HasSuffix(name, extTTF) || strings.Contains(name, "..") {
		http.NotFound(writer, req)
		return
	}
	modList, _ := srv.activeModules()
	if data, found := templates.ReadModuleAsset(modList, path.Join("fonts", name)); found {
		writer.Header().Set(headerContentType, mimeTTF)
		writer.Header().Set(headerCacheControl, cacheImmutableAsset)
		_, _ = writer.Write(data)
		return
	}
	http.NotFound(writer, req)
}

func (srv *EngineServer) serveUIAsset(assetPath, contentType string) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		tStart := time.Now()
		_, cssBytes, jsBytes, loadErr := srv.loadAndCacheAssets()
		if loadErr != nil {
			http.NotFound(writer, req)
			return
		}
		engine.Active.MarkAssetOnce(strings.TrimPrefix(assetPath, "/"), tStart)
		writer.Header().Set(headerContentType, contentType)
		writer.Header().Set(headerCacheControl, cacheNoStore)
		if assetPath == routeStyleCSS {
			_, _ = writer.Write(cssBytes)
			return
		}
		_, _ = writer.Write(jsBytes)
	}
}

func (srv *EngineServer) serveIndex(writer http.ResponseWriter, req *http.Request) {
	if req.URL.Path != routeRoot {
		http.NotFound(writer, req)
		return
	}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(writer, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	startIdx := time.Now()
	parsed, _, _, loadErr := srv.loadAndCacheAssets()
	if loadErr != nil {
		http.Error(writer, fmt.Sprintf(errTemplateCompileFmt, loadErr), http.StatusInternalServerError)
		return
	}
	buf := templates.AcquireBuffer()
	defer templates.ReleaseBuffer(buf)
	if srv.cfg.RenderRootView != nil {
		if renderErr := srv.cfg.RenderRootView(buf, parsed); renderErr != nil {
			http.Error(writer, renderErr.Error(), http.StatusInternalServerError)
			return
		}
	}
	writer.Header().Set(headerContentType, mimeHTMLUTF8)
	writer.Header().Set(headerCacheControl, cacheNoStore)
	_, _ = writer.Write(buf.Bytes())
	engine.Active.MarkStep(engine.StageHTTPIndexRender, startIdx, fmt.Sprintf("bytes=%d", buf.Len()))
}
