//go:build !(js && wasm)

package actions

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	book "mojra/domain/book/actions"
	"mojra/domain/common"
	commonactions "mojra/domain/common/actions"
	commonstate "mojra/domain/common/state"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/library"
	libraryactions "mojra/domain/library/actions"
	"mojra/domain/license"
	_ "mojra/domain/license/actions"
	licenseviews "mojra/domain/license/views"
	"mojra/domain/player"
	playeractions "mojra/domain/player/actions"
	bookplayeractions "mojra/domain/shell/book_player/actions"
	"mojra/domain/shell/studio_app"
	"mojra/domain/shell/studio_app/state"
	"mojra/domain/shell/studio_app/views"
	"mojra/domain/studio"
	studioactions "mojra/domain/studio/actions"
	studioviews "mojra/domain/studio/views"
	"mojra/engine"
	"mojra/engine/osbridge"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

const (
	appSubDir             = "app"
	iconsSubDir           = "icons"
	iconPNGFile           = "icon.png"
	routeBooksPrefix      = "/api/books/"
	mediaRouteSep         = "/media/"
	entryShellTpl         = "shell_view"
	errMethodNotAllowed   = "Method not allowed"
	errForbiddenOrigin    = "Forbidden: cross-origin action request rejected"
	errForbiddenMode      = "Forbidden in current mode"
	errForbiddenLAN       = "Forbidden on LAN"
	errMediaLocked        = "media locked until scene is reached"
	formFieldAction       = "action"
	formFieldAudioTime    = "audio_time"
	formKeyView           = "view"
	viewStudioAlias       = "studio"
	formKeyScene          = "scene"
	formFieldSilent       = "silent"
	formValTrue           = "true"
	pendingQuitApp        = "QuitApp"
	nativeCmdQuit         = "quit"
	assetStudioAtlas      = "studio-atlas.png"
	assetStudioUI         = "studio-ui.png"
	stageLoadLibrary      = engine.Stage("boot.load_library")
	stageInitPlayerState  = engine.Stage("boot.init_player_state")
)

func init() {
	if rawIcon, ok := templates.ReadModuleAsset([]fs.FS{common.FS}, filepath.ToSlash(filepath.Join(iconsSubDir, iconPNGFile))); ok {
		osbridge.AppIconPNG = rawIcon
	}
	transport.GeneratedAssetBuilders[assetStudioUI] = book.BuildStudioUIAtlas
	book.BookPlayerModules = bookplayeractions.ModuleFSList()
}

// StudioAppModules returns the ordered []fs.FS and module subpath list declared in studio_app/modules.cfg plus the shell itself.
func StudioAppModules() ([]fs.FS, []string) {
	registry := map[string]fs.FS{
		"common":  common.FS,
		"player":  player.FS,
		"library": library.FS,
		"studio":  studio.FS,
		"license": license.FS,
	}
	resolved, names, err := templates.ResolveModuleFSList(studio_app.FS, registry)
	if err != nil || len(resolved) == 0 {
		return []fs.FS{common.FS, player.FS, library.FS, studio.FS, license.FS, studio_app.FS},
			[]string{"common", "player", "library", "studio", "license", "shell/studio_app"}
	}
	return resolved, append(names, "shell/studio_app")
}

// Server wraps EngineServer with the Mojra studio_app application state.
type Server struct {
	State  *state.AppState
	engine *transport.EngineServer
}

// NewServerWithOptions initializes a full studio_app Server instance from rootDir.
func NewServerWithOptions(rootDir string, embeddedFS fs.FS, devMode bool) *Server {
	booksDir := filepath.Join(rootDir, "books")
	standaloneRootBook := !devMode && book.HasBookFilesInDir(rootDir)
	if standaloneRootBook {
		booksDir = rootDir
	}
	savesDir := book.ResolveSavesDir(rootDir)
	s, _ := newServerWithMode(rootDir, booksDir, savesDir, devMode, embeddedFS, standaloneRootBook, "")
	return s
}

func newServerWithMode(rootDir, booksDir, savesDir string, devMode bool, embeddedFS fs.FS, standalone bool, standaloneBookID string) (*Server, error) {
	if savesDir == "" {
		savesDir = book.ResolveSavesDir(rootDir)
	}
	tCfg := time.Now()
	cfg := state.RuntimeConfig{
		RootDir:          rootDir,
		BooksDir:         booksDir,
		SavesDir:         savesDir,
		AppDir:           filepath.Join(rootDir, appSubDir),
		EmbeddedFS:       embeddedFS,
		DevMode:          devMode,
		StandaloneMode:   standalone,
		StandaloneBookID: book.SanitizeBookID(standaloneBookID),
		ShutdownCh:       make(chan struct{}),
	}
	appState := state.NewAppState(cfg)
	if appState.Settings.NewBookDir == "" {
		appState.Settings.NewBookDir = booksDir
	}
	WireShellHooks(appState)
	engine.Active.MarkStep(engine.StageLoadSettings, tCfg, savesDir)

	s := &Server{State: appState}
	tBooks := time.Now()
	var books []*book.Book
	if standalone {
		if b, err := book.LoadBookFromDir(booksDir); err == nil && b != nil {
			books = []*book.Book{b}
		} else if b, err := book.FindBookByID(booksDir, appState.Settings.ExternalBooks, cfg.StandaloneBookID); err == nil && b != nil {
			books = []*book.Book{b}
		}
	} else {
		books, _ = book.LoadLibraryBooks(booksDir, appState.Settings.ExternalBooks)
	}
	appState.Library.Set(books)
	if standalone && len(books) > 0 && appState.Config.StandaloneBookID == "" {
		appState.Config.StandaloneBookID = books[0].ID
		appState.Settings.ActiveBookID = books[0].ID
	}
	engine.Active.MarkStep(stageLoadLibrary, tBooks, fmt.Sprintf("books=%d", len(books)))

	tInit := time.Now()
	s.initializeActiveBooks(books)
	engine.Active.MarkStep(stageInitPlayerState, tInit, appState.Settings.ActiveView)

	modFSList, modSubDirs := StudioAppModules()
	s.engine = transport.NewEngineServer(transport.EngineConfig{
		RootDir:          rootDir,
		EngineFS:         engine.FS,
		ModuleFSList:     modFSList,
		ModuleSubDirs:    modSubDirs,
		ModuleRootAssets: []string{assetStudioAtlas},
		DevMode:          devMode,
		StandaloneMode:   standalone,
		CSPExemptPrefix:  routeBooksPrefix,
		RenderRootView:   s.renderRootView,
		HandleAction:     s.handleAction,
		RegisterRoutes:   s.registerDomainRoutes,
		ResolveIconBytes: s.resolveAppOrBookIconBytes,
		ShutdownCh:       appState.Config.ShutdownCh,
	})
	return s, nil
}

// ShutdownCh returns the channel closed when the user triggers QuitApp.
func (s *Server) ShutdownCh() <-chan struct{} {
	return s.State.Config.ShutdownCh
}

// EnsureTemplatesLoaded returns the compiled template map.
func (s *Server) EnsureTemplatesLoaded() map[string]*templates.ParsedTemplate {
	if s.engine == nil {
		return nil
	}
	parsed, _ := s.engine.LoadServerTemplates()
	return parsed
}

// Handler returns the HTTP handler for this Server.
func (s *Server) Handler() http.Handler {
	return s
}

// ServeHTTP delegates HTTP requests to the underlying EngineServer handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.engine != nil {
		s.engine.Handler().ServeHTTP(w, r)
	}
}

func (s *Server) renderRootView(w io.Writer, parsed map[string]*templates.ParsedTemplate) error {
	s.State.Mu.Lock()
	shellChunk := views.ShellDocumentView(s.State)
	lang := views.LangOf(s.State)
	s.State.Mu.Unlock()

	ctx := engine.NewViewContext(lang, shellChunk.TemplateData)
	return templates.RenderTemplateTo(w, entryShellTpl, parsed, ctx)
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	HandleAction(s.State, w, r, s.EnsureTemplatesLoaded)
}

func (s *Server) registerDomainRoutes(mux *http.ServeMux) {
	mux.HandleFunc(routeBooksPrefix, s.serveMedia)
}

func (s *Server) initializeActiveBooks(books []*book.Book) {
	st := s.State
	if st.Config.StandaloneMode {
		var chosen *book.Book
		for _, b := range books {
			if b.ID == st.Config.StandaloneBookID {
				chosen = b
				break
			}
		}
		if chosen == nil && len(books) > 0 {
			chosen = books[0]
		}
		if chosen != nil {
			st.Player.SetBook(st.Config.SavesDir, st.Settings, chosen)
		}
		st.Settings.ActiveView = string(state.ViewPlayer)
		return
	}

	var chosenPlayer *book.Book
	if st.Settings.ActiveBookID != "" {
		for _, b := range books {
			if b.ID == st.Settings.ActiveBookID {
				chosenPlayer = b
				break
			}
		}
	}
	if chosenPlayer == nil && len(books) > 0 {
		chosenPlayer = books[0]
	}
	if chosenPlayer != nil {
		st.Player.SetBook(st.Config.SavesDir, st.Settings, chosenPlayer)
	}

	if !st.License.IsActive() || st.Settings.EditorStudioHome {
		st.Editor.ActiveBook = nil
		st.Editor.StudioHome = true
	} else {
		var chosenEditor *book.Book
		if st.Settings.EditorBookID != "" {
			for _, b := range books {
				if b.ID == st.Settings.EditorBookID && !b.IsPacked {
					chosenEditor = b
					break
				}
			}
		}
		if chosenEditor != nil {
			st.Editor.SetBook(chosenEditor, st.Settings)
		} else {
			st.Editor.ActiveBook = nil
			st.Editor.StudioHome = true
		}
	}

	switch state.ParseWorkspaceView(st.Settings.ActiveView) {
	case state.ViewPlayer:
		if st.Player.Book == nil {
			st.Settings.ActiveView = string(state.ViewLibrary)
		}
	case state.ViewEditor:
		st.Settings.ActiveView = string(state.ViewEditor)
	default:
		st.Settings.ActiveView = string(state.ViewLibrary)
	}
}

func (s *Server) findBook(bookID string) *book.Book {
	s.State.Mu.Lock()
	if s.State.Editor.ActiveBook != nil && s.State.Editor.ActiveBook.ID == bookID {
		b := s.State.Editor.ActiveBook
		s.State.Mu.Unlock()
		return b
	}
	if s.State.Player.Book != nil && s.State.Player.Book.ID == bookID {
		b := s.State.Player.Book
		s.State.Mu.Unlock()
		return b
	}
	if b := s.State.Library.Get(bookID); b != nil {
		s.State.Mu.Unlock()
		return b
	}
	ext := append([]string(nil), s.State.Settings.ExternalBooks...)
	booksDir := s.State.Config.BooksDir
	s.State.Mu.Unlock()

	b, _ := book.FindBookByID(booksDir, ext, bookID)
	return b
}

func (s *Server) resolveAppOrBookIconBytes() ([]byte, bool) {
	s.State.Mu.Lock()
	var activeBook *book.Book
	if s.State.Config.StandaloneMode && s.State.Player.Book != nil {
		activeBook = s.State.Player.Book
	}
	s.State.Mu.Unlock()

	if activeBook != nil && activeBook.CoverFile != "" && !activeBook.IsPacked {
		coverPath := filepath.Join(activeBook.RootPath, filepath.FromSlash(activeBook.CoverFile))
		if book.IsSafePathInsideRoot(activeBook.RootPath, coverPath) {
			if data, err := os.ReadFile(coverPath); err == nil && len(data) > 0 {
				return data, true
			}
		}
	}
	modFSList, _ := StudioAppModules()
	return templates.ReadModuleAsset(modFSList, filepath.ToSlash(filepath.Join(iconsSubDir, iconPNGFile)))
}

func (s *Server) isStandaloneMediaUnlocked(activeBook *book.Book, rawRelPath string) bool {
	if !s.State.Config.StandaloneMode || s.State.Config.DevMode {
		return true
	}
	if activeBook == nil || (s.State.Config.StandaloneBookID != "" && activeBook.ID != s.State.Config.StandaloneBookID) {
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

	save := s.State.Player.Save
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

func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request) {
	tMedia := time.Now()
	defer engine.Active.MarkStep(engine.StageHTTPAssetServe, tMedia, r.URL.Path)
	trimmed := strings.TrimPrefix(r.URL.Path, routeBooksPrefix)
	parts := strings.SplitN(trimmed, mediaRouteSep, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(w, r)
		return
	}
	bookID := book.SanitizeBookID(parts[0])
	relPath := parts[1]

	s.State.Mu.Lock()
	var activeBook *book.Book
	if s.State.Hooks.IsHistoryPreview() && s.State.Editor.ActiveBook != nil && s.State.Editor.ActiveBook.ID == bookID {
		activeBook = s.State.Editor.ActiveBook
	} else {
		activeBook = s.State.Library.Get(bookID)
	}
	mediaUnlocked := s.isStandaloneMediaUnlocked(activeBook, relPath)
	s.State.Mu.Unlock()

	if activeBook == nil {
		activeBook = s.findBook(bookID)
	}
	if activeBook == nil {
		http.NotFound(w, r)
		return
	}
	if !mediaUnlocked {
		http.Error(w, errMediaLocked, http.StatusForbidden)
		return
	}
	book.ServeBookMedia(w, r, activeBook, relPath)
}

// ActionHandler defines the signature for a studio_app shell action handler.
type ActionHandler func(s *state.AppState, req *transport.ActionRequest) []transport.ActionFrame

var (
	// Handlers aliases the global action registry in domain/common/actions.
	Handlers = commonactions.Handlers
)

func shellFromUI(ui *commonstate.UIState) *state.AppState {
	if ui != nil && ui.Hooks.ShellState != nil {
		if app, ok := ui.Hooks.ShellState.(*state.AppState); ok {
			return app
		}
	}
	return nil
}

func registerActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		fn := handler
		commonactions.RegisterAction(actionName, false, false, func(ui *commonstate.UIState, req *transport.ActionRequest) []transport.ActionFrame {
			app := shellFromUI(ui)
			if app == nil {
				return nil
			}
			return fn(app, req)
		})
	}
	return true
}

func registerLANActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		fn := handler
		commonactions.RegisterAction(actionName, true, false, func(ui *commonstate.UIState, req *transport.ActionRequest) []transport.ActionFrame {
			app := shellFromUI(ui)
			if app == nil {
				return nil
			}
			return fn(app, req)
		})
	}
	return true
}

var (
	_ = registerLANActions(map[string]ActionHandler{
		"SwitchView": switchView,
	})
	_ = registerActions(map[string]ActionHandler{
		"PlayFromScene": playFromScene,
	})
)

// WireShellHooks connects all cross-module callbacks on s.Hooks.
func WireShellHooks(s *state.AppState) {
	s.BindSubstates()

	s.Hooks.RenderAppFrames = func() []transport.ActionFrame {
		if s.Confirm.IsOpen() {
			return []transport.ActionFrame{s.Hooks.RenderModalsFrame()}
		}
		return []transport.ActionFrame{
			{Target: commonstate.TargetApp, Chunks: []templates.TemplateChunk{views.ShellView(s)}},
		}
	}

	s.Hooks.RenderWorkspaceFrames = func() []transport.ActionFrame {
		if s.Confirm.IsOpen() {
			return []transport.ActionFrame{s.Hooks.RenderModalsFrame()}
		}
		return []transport.ActionFrame{
			{Target: commonstate.TargetWorkspace, Chunks: []templates.TemplateChunk{views.ActiveWorkspaceView(s)}},
		}
	}

	s.Hooks.SettingsExtraSections = func() []templates.TemplateChunk {
		return []templates.TemplateChunk{licenseviews.LicenseSummaryView(&s.License)}
	}

	s.Hooks.RenderModalsFrame = func() transport.ActionFrame {
		return transport.ActionFrame{
			Target: commonstate.TargetModalsRoot,
			Chunks: views.ActiveOverlayViews(s),
		}
	}

	s.Hooks.RenderToastFrames = func(toast string) []transport.ActionFrame {
		if toast != "" {
			s.Overlays.Toast = toast
		}
		if s.Overlays.Settings {
			return []transport.ActionFrame{s.Hooks.RenderModalsFrame()}
		}
		return nil
	}

	s.Hooks.ReloadAllBooks = func() {
		books, _ := book.LoadLibraryBooks(s.Config.BooksDir, s.Settings.ExternalBooks)
		s.Library.Set(books)
	}

	s.Hooks.OpenBookInView = func(bookID string, targetView commonstate.ViewMode) string {
		b := s.Library.Get(bookID)
		if b == nil {
			return ""
		}
		if targetView == commonstate.ViewEditor {
			if !s.License.IsActive() {
				s.Settings.ActiveView = string(commonstate.ViewEditor)
				s.CloseAllModals()
				s.SaveSettings()
				return ""
			}
			return studioactions.OpenBookInStudio(&s.Editor, b)
		}
		if s.Settings != nil && s.Settings.ActiveView == string(commonstate.ViewEditor) && s.Editor.HasActiveBook() {
			s.Editor.SaveBookUIState(s.Settings)
		}
		if s.Hooks.IsHistoryPreview() {
			s.Hooks.ExitHistoryPreview()
		}
		s.Player.SetBook(s.Config.SavesDir, s.Settings, b)
		s.Settings.ActiveView = string(commonstate.ViewPlayer)
		s.CloseAllModals()
		s.Editor.SyncToSettings(s.Settings)
		s.SaveSettings()
		return ""
	}

	s.Hooks.RemoveBookByID = func(bookID string) (string, bool) {
		b := s.Library.Get(bookID)
		msg := libraryactions.RemoveBookByID(&s.Library, bookID)
		if b != nil {
			if s.Player.Book != nil && s.Player.Book.ID == b.ID {
				s.Player.SetBook(s.Config.SavesDir, s.Settings, nil)
				if len(s.Library.Books) > 0 {
					s.Player.SetBook(s.Config.SavesDir, s.Settings, s.Library.Books[0])
				}
				s.SaveSettings()
			}
			if s.Editor.ActiveBook != nil && s.Editor.ActiveBook.ID == b.ID {
				s.Editor.SetBook(nil, s.Settings)
				s.SaveSettings()
			}
		}
		return msg, true
	}

	s.Hooks.OnBookCreated = func(created *book.Book, openTarget string) {
		if created == nil {
			return
		}
		s.Library.Put(created)
		if openTarget == string(commonstate.ViewEditor) {
			if s.License.IsActive() {
				s.Editor.SetBook(created, s.Settings)
			}
			if s.Player.Book == nil {
				s.Player.SetBook(s.Config.SavesDir, s.Settings, created)
			}
		} else {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, created)
			if s.License.IsActive() && s.Editor.ActiveBook == nil {
				s.Editor.SetBook(created, s.Settings)
			}
		}
		s.SaveSettings()
	}

	s.Hooks.OnBookMutated = func(mutated *book.Book) {
		if mutated == nil {
			return
		}
		s.Library.Put(mutated)
		if !s.Hooks.IsHistoryPreview() {
			if s.Player.Book == nil {
				s.Player.SetBook(s.Config.SavesDir, s.Settings, mutated)
			} else if s.Player.Book.ID == mutated.ID {
				s.Player.Book = mutated
				if s.Player.Save != nil && len(s.Player.Save.History) == 0 && s.Player.Save.AudioTime == 0 {
					s.Player.Save.CurrentSceneID = mutated.ResolveValidSceneID(mutated.StartSceneID)
				} else if s.Player.Save != nil {
					s.Player.Save.CurrentSceneID = mutated.ResolveValidSceneID(s.Player.Save.CurrentSceneID)
				}
			}
		}
	}

	s.Hooks.PlayBookWithSave = func(targetBook *book.Book, save *book.SaveState, isHistPreview bool, histLabel string) {
		if targetBook == nil {
			return
		}
		if isHistPreview {
			if !s.Player.IsHistoryPreview {
				s.Player.LiveBook = s.Player.Book
				s.Player.LiveSave = s.Player.Save
			}
			s.Player.IsHistoryPreview = true
			if s.Editor.History != nil && s.Editor.History.Preview != nil {
				s.Player.HistoryPreviewID = s.Editor.History.Preview.RevisionID
			}
			s.Player.HistoryPreviewLabel = histLabel
		}
		s.Player.Book = targetBook
		if save != nil {
			s.Player.Save = save
			s.Player.HistoryRevealed = false
			s.Player.FocusedHistoryIdx = -1
		} else if s.Player.Save == nil {
			startSceneID := targetBook.ResolveValidSceneID(targetBook.StartSceneID)
			s.Player.Save = &book.SaveState{
				BookID:             targetBook.ID,
				CurrentSceneID:     startSceneID,
				PlaybackSpeed:      s.Settings.PlaybackSpeed,
				AutoChoice:         s.Settings.AutoChoice,
				AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
				AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
			}
		} else if targetBook.FindScene(s.Player.Save.CurrentSceneID) == nil && len(targetBook.Scenes) > 0 {
			s.Player.Save.CurrentSceneID = targetBook.ResolveValidSceneID(targetBook.StartSceneID)
		}
	}

	s.Hooks.ExitHistoryPreview = func() string {
		s.Player.ExitHistoryPreview(s.Config.SavesDir, s.Editor.ActiveBook)
		return ""
	}

	s.Hooks.UnpackPlaybookForEdit = func(bookID string) (string, bool) {
		if !s.License.IsActive() {
			return "", false
		}
		b := s.Library.Get(bookID)
		if b == nil {
			return "", false
		}
		toast := studioactions.UnpackPlaybookWorkingCopyForStudio(&s.Editor, b.RootPath)
		return toast, true
	}

	s.Hooks.ExecutePendingConfirm = func(pendingAction, pendingParam string) ([]transport.ActionFrame, bool) {
		switch pendingAction {
		case playeractions.PendingRewindStep:
			toast := playeractions.RewindPlayerToStep(&s.Player, pendingParam)
			s.Overlays.Toast = toast
			return s.Hooks.RenderAppFrames(), true
		case playeractions.PendingResetProgress:
			targetID := pendingParam
			if targetID == "" && s.Player.Book != nil {
				targetID = s.Player.Book.ID
			}
			toast := ""
			if targetID != "" {
				if s.Hooks.IsHistoryPreview() && s.Player.Book != nil && s.Player.Book.ID == targetID {
					startSceneID := s.Player.Book.ResolveValidSceneID(s.Player.Book.StartSceneID)
					s.Player.Save = &book.SaveState{
						BookID:             s.Player.Book.ID,
						CurrentSceneID:     startSceneID,
						PlaybackSpeed:      s.Settings.PlaybackSpeed,
						AutoChoice:         s.Settings.AutoChoice,
						AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
						AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
					}
					s.Player.HistoryRevealed = false
					s.Player.FocusedHistoryIdx = -1
				} else {
					_ = book.DeleteSaveState(s.Config.SavesDir, targetID)
					if s.Player.Book != nil && s.Player.Book.ID == targetID {
						s.Player.Save = book.LoadSaveState(s.Config.SavesDir, s.Player.Book.ID, s.Player.Book.StartSceneID)
						s.Player.HistoryRevealed = false
						s.Player.FocusedHistoryIdx = -1
					}
				}
				toast = commonviews.Tr(s.UIState, i18n.TrResetProgressToast)
			}
			s.Overlays.Toast = toast
			return s.Hooks.RenderAppFrames(), true
		case libraryactions.PendingRemoveBook:
			toast, _ := s.Hooks.RemoveBookByID(pendingParam)
			s.Overlays.Toast = toast
			return s.Hooks.RenderAppFrames(), true
		case pendingQuitApp:
			s.Overlays.NativeCmd = nativeCmdQuit
			go func() {
				s.Config.TriggerShutdown()
				osbridge.TerminateNativeWindow()
			}()
			return nil, true
		default:
			if s.License.IsActive() {
				if frames, ok := studioactions.ExecutePendingStudioConfirm(&s.Editor, pendingAction, pendingParam); ok {
					return frames, true
				}
			}
		}
		return nil, false
	}
}

func switchView(s *state.AppState, req *transport.ActionRequest) []transport.ActionFrame {
	_ = studioactions.FlushFormSceneIfPresent(&s.Editor, req)
	s.License.ShowActivatedSummary = false
	view := strings.TrimSpace(req.FormValue(formKeyView))
	prevView := ""
	if s.Settings != nil {
		prevView = s.Settings.ActiveView
	}
	wasActiveStudioEditor := prevView == string(state.ViewEditor) && s.Editor.HasActiveBook()
	if (view == string(state.ViewLibrary) || view == viewStudioAlias) && s.Hooks.IsHistoryPreview() {
		studioactions.ExitStudioHistoryPreview(&s.Editor)
	}
	if view == viewStudioAlias {
		if wasActiveStudioEditor {
			s.Editor.SaveBookUIState(s.Settings)
		}
		s.Editor.StudioHome = true
		s.Player.ZenMode = false
		s.Settings.ActiveView = string(state.ViewEditor)
		s.CloseAllModals()
		if s.License.IsActive() {
			s.Editor.SyncToSettings(s.Settings)
		}
		s.SaveSettings()
		return s.Hooks.RenderAppFrames()
	}
	if view == string(state.ViewEditor) {
		s.Editor.StudioHome = false
	}
	if view == string(state.ViewPlayer) && s.Player.Book == nil {
		if s.Editor.ActiveBook != nil {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, s.Editor.ActiveBook)
		} else if len(s.Library.Books) > 0 {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, s.Library.Books[0])
		}
	}
	if view == string(state.ViewLibrary) || view == string(state.ViewPlayer) || view == string(state.ViewEditor) {
		if view != string(state.ViewEditor) && wasActiveStudioEditor {
			s.Editor.SaveBookUIState(s.Settings)
		}
		if view != string(state.ViewPlayer) {
			s.Player.ZenMode = false
		}
		s.Settings.ActiveView = view
		s.CloseAllModals()
		if view == string(state.ViewEditor) && !wasActiveStudioEditor && s.License.IsActive() {
			s.Editor.RestoreBookUIState(s.Settings, true)
		}
		if s.License.IsActive() {
			s.Editor.SyncToSettings(s.Settings)
		}
		s.SaveSettings()
	}
	return s.Hooks.RenderAppFrames()
}

func playFromScene(s *state.AppState, req *transport.ActionRequest) []transport.ActionFrame {
	if !s.License.IsActive() {
		return s.Hooks.RenderAppFrames()
	}
	_ = studioactions.FlushFormSceneIfPresent(&s.Editor, req)
	if scID := strings.TrimSpace(req.FormValue(formKeyScene)); scID != "" {
		s.Editor.SelectedSceneID = scID
	}
	b := s.Editor.ActiveBook
	if b != nil && s.Editor.SelectedSceneID != "" {
		if !s.Hooks.IsHistoryPreview() && (s.Player.Book == nil || s.Player.Book.ID != b.ID || s.Player.Save == nil) {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, b)
		} else if s.Player.Book == nil {
			s.Player.Book = b
		}
		if s.Player.Save != nil {
			if !strings.EqualFold(s.Player.Save.CurrentSceneID, s.Editor.SelectedSceneID) {
				truncated := false
				for i, h := range s.Player.Save.History {
					if strings.EqualFold(h.SceneID, s.Editor.SelectedSceneID) {
						if len(h.Variables) > 0 {
							s.Player.Save.Variables = book.CloneVariablesMap(h.Variables)
						}
						s.Player.Save.History = s.Player.Save.History[:i]
						truncated = true
						break
					}
				}
				if !truncated {
					s.Player.Save.History = nil
					s.Player.Save.Variables = nil
				}
			}
			s.Player.Save.CurrentSceneID = s.Editor.SelectedSceneID
			s.Player.Save.AudioTime = 0
			book.EnsureHistoryFromStart(s.Player.Book, s.Player.Save)
			if len(s.Editor.SceneText.TestVars) > 0 {
				s.Player.Save.Variables = book.EnsureSaveVariables(s.Player.Book, s.Editor.SceneText.TestVars)
			} else {
				s.Player.Save.Variables = book.EnsureSaveVariables(s.Player.Book, s.Player.Save.Variables)
			}
			if !s.Hooks.IsHistoryPreview() {
				s.Player.PersistSave(s.Config.SavesDir)
			}
		}
		s.Editor.SaveBookUIState(s.Settings)
		s.Settings.ActiveView = string(state.ViewPlayer)
		s.Editor.SceneText.SceneID = ""
		s.Editor.SyncToSettings(s.Settings)
		s.SaveSettings()
	}
	return s.Hooks.RenderAppFrames()
}

// HandleAction processes POST /api/action requests for the studio_app shell and writes multi-frame FDP responses.
func HandleAction(appState *state.AppState, writer http.ResponseWriter, request *http.Request, ensureTemplates func() map[string]*templates.ParsedTemplate) {
	if request.Method != http.MethodPost {
		http.Error(writer, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	if !transport.ValidateSameOriginAction(request) {
		http.Error(writer, errForbiddenOrigin, http.StatusForbidden)
		return
	}
	tAction := time.Now()
	transport.ParseActionRequest(request)
	isLoopback := transport.IsLoopbackRequest(request)
	actionReq := &transport.ActionRequest{
		Form:       request.Form,
		IsLoopback: isLoopback,
	}
	act := strings.TrimSpace(actionReq.FormValue(formFieldAction))
	handler := commonactions.Handlers[act]

	appState.Mu.Lock()
	if !isLoopback {
		if !commonactions.LANAllowedActions[act] {
			appState.Mu.Unlock()
			http.Error(writer, errForbiddenLAN, http.StatusForbidden)
			return
		}
	}
	if appState.Config.StandaloneMode {
		appState.Settings.ActiveView = string(state.ViewPlayer)
		if handler != nil && !commonactions.StandaloneAllowedActions[act] {
			appState.Mu.Unlock()
			http.Error(writer, errForbiddenMode, http.StatusForbidden)
			return
		}
	}

	if !commonactions.ImmediateNoContentActions[act] && appState.Player.Book != nil && appState.Player.Save != nil {
		if rawAudioTime := strings.TrimSpace(actionReq.FormValue(formFieldAudioTime)); rawAudioTime != "" {
			if parsedTime, err := strconv.ParseFloat(rawAudioTime, 64); err == nil && parsedTime >= 0 {
				if parsedTime > 0.2 || appState.Player.Save.UpdatedAtUnix > 0 || len(appState.Player.Save.History) > 0 {
					appState.Player.Save.AudioTime = parsedTime
					if !appState.Hooks.IsHistoryPreview() {
						if err := book.UpdateSaveAudioTimeInPlace(appState.Config.SavesDir, appState.Player.Book.ID, parsedTime, appState.Player.Book.StartSceneID); err != nil {
							appState.Player.PersistSave(appState.Config.SavesDir)
						}
					}
				}
			}
		}
	}

	if commonactions.ImmediateNoContentActions[act] {
		if handler != nil {
			_ = handler(appState.UIState, actionReq)
		}
		scriptChunk := views.AppShellScriptView(appState)
		lang := views.LangOf(appState)
		appState.Mu.Unlock()
		transport.WriteRenderedActionFrames(writer, lang, nil, []templates.TemplateChunk{scriptChunk}, nil)
		return
	}

	var frames []transport.ActionFrame
	if handler != nil {
		frames = handler(appState.UIState, actionReq)
	} else {
		frames = appState.Hooks.RenderAppFrames()
	}

	if actionReq.FormValue(formFieldSilent) == formValTrue && !commonactions.IgnoreSilentActions[act] {
		frames = nil
	}

	actionFrames, globalScriptChunks := finalizeActionFrames(appState, act, frames)
	lang := views.LangOf(appState)
	_, _ = appState.ConsumeTransientFeedback()
	appState.Overlays.VoicePeak = ""
	appState.Overlays.ResetCanvas = false
	appState.Mu.Unlock()

	var loadedTemplates map[string]*templates.ParsedTemplate
	if ensureTemplates != nil && len(actionFrames) > 0 {
		loadedTemplates = ensureTemplates()
	}
	transport.WriteRenderedActionFrames(writer, lang, actionFrames, globalScriptChunks, loadedTemplates)
	engine.Active.MarkStep(engine.StageActionTotal, tAction, fmt.Sprintf("act=%s frames=%d", act, len(actionFrames)))
}

func finalizeActionFrames(appState *state.AppState, act string, actionFrames []transport.ActionFrame) ([]transport.ActionFrame, []templates.TemplateChunk) {
	if !commonactions.SkipSettingsSyncActions[act] && !appState.Config.StandaloneMode && appState.License.IsActive() && !appState.Hooks.IsHistoryPreview() {
		if appState.Editor.SyncToSettings(appState.Settings) {
			appState.SaveSettings()
		}
	}

	hasAppTarget := false
	hasModalsTarget := false
	hasStudio := false
	for _, af := range actionFrames {
		if af.Target == commonstate.TargetApp {
			hasAppTarget = true
		}
		if af.Target == commonstate.TargetModalsRoot {
			hasModalsTarget = true
		}
		if (af.Target == commonstate.TargetApp || af.Target == commonstate.TargetWorkspace) && state.ActiveWorkspaceView(appState) == state.ViewEditor && appState.License.IsActive() && !appState.Editor.StudioHome {
			hasStudio = true
		}
	}

	var globalScriptChunks []templates.TemplateChunk
	if !hasAppTarget {
		globalScriptChunks = append(globalScriptChunks, views.AppShellScriptView(appState))
		if !appState.Config.StandaloneMode {
			globalScriptChunks = append(globalScriptChunks, templates.TemplateChunk{ScriptData: views.TopbarView(appState).ScriptData})
		}
	}
	if !appState.Config.StandaloneMode && appState.License.IsActive() && !hasStudio {
		if !(commonactions.IgnoreSilentActions[act] && requestFormSilent(actionFrames)) {
			globalScriptChunks = append(globalScriptChunks,
				studioviews.StudioScriptView(&appState.Editor),
				studioviews.NodeCanvasScriptView(&appState.Editor),
			)
			if !hasAppTarget && !hasModalsTarget && appState.Editor.SceneText.IsOpen() {
				globalScriptChunks = append(globalScriptChunks, studioviews.TextEditorScriptView(&appState.Editor))
			}
		}
	}
	appState.Overlays.ResetCanvas = false
	return actionFrames, globalScriptChunks
}

func requestFormSilent(frames []transport.ActionFrame) bool {
	return len(frames) == 1 && frames[0].Target == commonstate.TargetNone
}
