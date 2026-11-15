//go:build !(js && wasm)

package domain

import (
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"mojra/domain/actions"
	"mojra/domain/book"
	_ "mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/domain/views"
	"mojra/engine"
	"mojra/engine/templates"
)

const (
	studioUIFileName  = "studio-ui.png"
	subDirApp         = "app"
	subDirBooks       = "books"
	indexTemplateName = "index"
	previewModeFull   = "full"
	routeBooksPrefix  = "/api/books/"
	mediaRouteSep     = "/media/"
	errMediaLocked    = "media locked until scene is reached"
)

var (
	mojraNonPlayerModulePrefixes = []string{
		"editor_",
		"studio_",
		"canvas_",
		"node_canvas",
		"msdf_",
		"text_editor",
		"library",
		"add_book",
		"export",
		"new_book",
	}

	standaloneExcludedTemplates = []string{
		"library_view",
		"studio_view",
		"editor_context_view",
		"editor_history_view",
		"editor_breakdown_view",
		"editor_variables_view",
	}
)

func init() {
	engine.GeneratedAssetBuilders[studioUIFileName] = book.BuildStudioUIAtlas
	engine.NonPlayerModulePrefixes = mojraNonPlayerModulePrefixes
}

// Server wraps the application state and the generic HTTP EngineServer.
type Server struct {
	State *state.AppState
	engine.EngineServer
}

// NewServerWithOptions boots the application domain (settings, books, player, studio) and wires it to EngineServer.
func NewServerWithOptions(rootDir string, embeddedFS fs.FS, devMode bool) *Server {
	booksDir := filepath.Join(rootDir, subDirBooks)
	standaloneRootBook := !devMode && book.HasBookFilesInDir(rootDir)
	if standaloneRootBook {
		booksDir = rootDir
	}
	savesDir := book.ResolveSavesDir(rootDir)

	tCfg := time.Now()
	cfg := book.LoadAppSettings(savesDir)
	cfg.PreviewMode = previewModeFull
	if cfg.NewBookDir == "" && book.IsTempDir(rootDir) && !book.IsInsideRepoDir(rootDir) {
		cfg.NewBookDir = booksDir
	}
	if standaloneRootBook {
		cfg.ActiveView = state.ViewPlayer.String()
	}
	engine.Active.MarkStep(engine.StageLoadSettings, tCfg, savesDir)

	appState := &state.AppState{
		Config: state.RuntimeConfig{
			RootDir:        rootDir,
			AppDir:         filepath.Join(rootDir, subDirApp),
			BooksDir:       booksDir,
			SavesDir:       savesDir,
			EmbeddedFS:     embeddedFS,
			DevMode:        devMode,
			StandaloneMode: standaloneRootBook,
			Port:           8080,
			ShutdownCh:     make(chan struct{}),
		},
		Settings: cfg,
	}
	appState.Player.FocusedHistoryIdx = -1

	server := &Server{
		State: appState,
	}
	server.EngineServer = engine.EngineServer{
		Config:                      server.engineConfig,
		RenderRootView:              server.renderRootView,
		HandleAction:                server.handleAction,
		CustomRoutes:                map[string]http.HandlerFunc{routeBooksPrefix: server.handleBookMediaRoute},
		ExcludedStandaloneTemplates: standaloneExcludedTemplates,
		CSPExemptPrefix:             routeBooksPrefix,
	}

	tTpl := time.Now()
	server.EnsureTemplatesLoaded()
	engine.Active.MarkStep(engine.StageLoadTemplates, tTpl, appState.Config.AppDir)

	tBooks := time.Now()
	var loadedBooks []*book.Book
	if standaloneRootBook {
		if singleBook, err := book.LoadBookFromDir(rootDir); err == nil && singleBook != nil {
			loadedBooks = []*book.Book{singleBook}
		}
	} else {
		loadedBooks, _ = book.LoadLibraryBooks(booksDir, cfg.ExternalBooks)
	}
	appState.Library.Set(loadedBooks)
	engine.Active.MarkStep(engine.StageLoadLibrary, tBooks, booksDir)

	if len(appState.Library.Books) > 0 {
		if standaloneRootBook {
			appState.Config.StandaloneBookID = appState.Library.Books[0].ID
			appState.Settings.ActiveBookID = appState.Library.Books[0].ID
		}
		tInitBook := time.Now()
		initialBook := appState.Library.Books[0]
		if cfg.ActiveBookID != "" {
			if found := appState.Library.Get(cfg.ActiveBookID); found != nil {
				initialBook = found
			}
		}
		appState.Player.SetBook(savesDir, appState.Settings, initialBook)
		engine.Active.MarkStep(engine.StageInitPlayerState, tInitBook, initialBook.ID)
		if !standaloneRootBook && cfg.EditorBookID != "" {
			if edBook := appState.Library.Get(cfg.EditorBookID); edBook != nil && !edBook.IsPacked {
				tEditor := time.Now()
				appState.Editor.SetBook(edBook, appState.Settings)
				appState.Editor.RestoreBookUIState(appState.Settings, appState.Settings.ActiveView == state.ViewEditor.String())
				engine.Active.MarkStep(engine.StageInitEditorState, tEditor, edBook.ID)
			}
		} else if !standaloneRootBook {
			appState.Editor.StudioHome = true
		}
	} else {
		appState.Editor.StudioHome = true
	}

	return server
}

func (server *Server) engineConfig() engine.EngineConfig {
	return engine.EngineConfig{
		RootDir:        server.State.Config.RootDir,
		AppDir:         server.State.Config.AppDir,
		EmbeddedFS:     server.State.Config.EmbeddedFS,
		DevMode:        server.State.Config.DevMode,
		StandaloneMode: server.State.Config.StandaloneMode,
		Port:           server.State.Config.Port,
	}
}

func (server *Server) renderRootView(writer http.ResponseWriter, loadedTemplates map[string]*templates.ParsedTemplate) {
	tRender := time.Now()
	server.State.Mu.Lock()
	_, appShellData := views.AppShellView(server.State).Resolve()
	ctx := engine.NewViewContext(views.LangOf(server.State), appShellData)
	server.State.Mu.Unlock()

	buf := templates.AcquireBuffer()
	defer templates.ReleaseBuffer(buf)
	_ = templates.RenderTemplateTo(buf, indexTemplateName, loadedTemplates, ctx)
	_, _ = writer.Write(buf.Bytes())
	engine.Active.MarkStep(engine.StageHTTPIndexRender, tRender, server.State.Settings.ActiveView)
}

func (server *Server) handleAction(writer http.ResponseWriter, request *http.Request, ensureTemplates func() map[string]*templates.ParsedTemplate) {
	actions.HandleAction(server.State, writer, request, ensureTemplates)
}

func (server *Server) isStandaloneMediaUnlocked(activeBook *book.Book, rawRelPath string) bool {
	if !server.State.Config.StandaloneMode || server.State.Config.DevMode {
		return true
	}
	if activeBook == nil || (server.State.Config.StandaloneBookID != "" && activeBook.ID != server.State.Config.StandaloneBookID) {
		return false
	}
	decoded, err := url.PathUnescape(rawRelPath)
	if err != nil {
		decoded = rawRelPath
	}
	cleanRel := book.SanitizeRelativeMediaPath(decoded)
	if cleanRel == "" {
		return false
	}
	if activeBook.CoverFile != "" && strings.EqualFold(cleanRel, activeBook.CoverFile) {
		return true
	}

	save := server.State.Player.Save
	var unlockedScenes []*book.Scene
	if save != nil {
		if currentScene := activeBook.FindScene(save.CurrentSceneID); currentScene != nil {
			unlockedScenes = append(unlockedScenes, currentScene)
		}
		for _, histEntry := range save.History {
			if histScene := activeBook.FindScene(histEntry.SceneID); histScene != nil {
				unlockedScenes = append(unlockedScenes, histScene)
			}
		}
	}
	if len(unlockedScenes) == 0 {
		if startScene := activeBook.FindScene(activeBook.StartSceneID); startScene != nil {
			unlockedScenes = append(unlockedScenes, startScene)
		} else if len(activeBook.Scenes) > 0 {
			unlockedScenes = append(unlockedScenes, activeBook.Scenes[0])
		}
	}

	for _, scene := range unlockedScenes {
		if scene == nil {
			continue
		}
		if (scene.ImageFile != "" && strings.EqualFold(cleanRel, scene.ImageFile)) ||
			(scene.AudioFile != "" && strings.EqualFold(cleanRel, scene.AudioFile)) {
			return true
		}
		for _, sec := range scene.VoiceSections {
			if sec.AudioFile != "" && strings.EqualFold(cleanRel, sec.AudioFile) {
				return true
			}
		}
	}
	return false
}

func (server *Server) handleBookMediaRoute(writer http.ResponseWriter, request *http.Request) {
	tMedia := time.Now()
	defer engine.Active.MarkStep(engine.StageHTTPAssetServe, tMedia, request.URL.Path)
	trimmed := strings.TrimPrefix(request.URL.Path, routeBooksPrefix)
	parts := strings.SplitN(trimmed, mediaRouteSep, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(writer, request)
		return
	}
	bookID := book.SanitizeBookID(parts[0])
	relPath := parts[1]

	server.State.Mu.RLock()
	var activeBook *book.Book
	if isHistoryPreview(server.State) && server.State.Editor.ActiveBook != nil && server.State.Editor.ActiveBook.ID == bookID {
		activeBook = server.State.Editor.ActiveBook
	} else {
		activeBook = server.State.Library.Get(bookID)
	}
	mediaUnlocked := server.isStandaloneMediaUnlocked(activeBook, relPath)
	server.State.Mu.RUnlock()

	if activeBook == nil {
		http.NotFound(writer, request)
		return
	}
	if !mediaUnlocked {
		http.Error(writer, errMediaLocked, http.StatusForbidden)
		return
	}
	book.ServeBookMedia(writer, request, activeBook, relPath)
}

func isHistoryPreview(s *state.AppState) bool {
	return s != nil && !s.Config.StandaloneMode && s.Editor.History.IsPreview(s.Editor.ActiveBook)
}
