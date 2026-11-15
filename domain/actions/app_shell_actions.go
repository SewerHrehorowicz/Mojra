//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"mojra/domain/book"
	_ "mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/domain/views"
	"mojra/engine"
	"mojra/engine/osbridge"
	"mojra/engine/templates"
)

const (
	targetSelApp       = "#app"
	targetSelWorkspace = "#workspace"
	targetSelModals    = "#modals-root"
	targetSelInspector = "#editor-context-view"
	targetSelNone      = "none"

	nativeCmdToggleFullscreen = "toggle-fullscreen"
	nativeCmdQuit             = "quit"

	formFieldAction    = "action"
	formFieldAudioTime = "audio_time"
	formFieldSilent    = "silent"

	errMethodNotAllowed = "method not allowed"
	errForbiddenOrigin  = "forbidden cross-origin action"
	errForbiddenLAN     = "forbidden remote LAN action"
	errForbiddenStand   = "forbidden in standalone book mode"

	actSaveAudioTime      = "SaveAudioTime"
	actStartWindowDrag    = "StartWindowDrag"
	actConfirmModalAccept = "ConfirmModalAccept"
	actAutoSaveScene      = "AutoSaveScene"
	actUndoEditor         = "UndoEditor"

	formKeyView     = "view"
	viewStudioAlias = "studio"
	modePhone       = "phone"
	modePortrait    = "portrait"
	modeIPad        = "ipad"
	modeTablet      = "tablet"
	modeDesktop     = "desktop"
	modeFull        = "full"
	modeLandscape   = "landscape"
)

// ActionHandler processes an action request against AppState and returns the DOM frames to render.
type ActionHandler func(appState *state.AppState, request *http.Request) []engine.ActionFrame

var (
	Handlers                 = map[string]ActionHandler{}
	lanAllowedActions        = map[string]bool{}
	standaloneAllowedActions = map[string]bool{}
)

var (
	_ = registerActions(map[string]ActionHandler{
		"SwitchView":    switchView,
		"SetDeviceMode": setDeviceMode,
	})
	_ = registerStandaloneActions(map[string]ActionHandler{
		"ToggleFullscreen": toggleFullscreen,
		"StartWindowDrag":  startWindowDrag,
		"QuitApp":          quitApp,
		"PromptQuitApp":    quitApp,
	})
)

func registerActions(entries map[string]ActionHandler) bool {
	for name, handler := range entries {
		Handlers[name] = handler
	}
	return true
}

func registerStandaloneActions(entries map[string]ActionHandler) bool {
	for name, handler := range entries {
		Handlers[name] = handler
		standaloneAllowedActions[name] = true
	}
	return true
}

func registerLANActions(entries map[string]ActionHandler) bool {
	for name, handler := range entries {
		Handlers[name] = handler
		lanAllowedActions[name] = true
		standaloneAllowedActions[name] = true
	}
	return true
}

func tr(s *state.AppState, k engine.TrKey) string {
	return engine.T(views.LangOf(s), k)
}

func trf(s *state.AppState, k engine.TrKey, args ...any) string {
	return engine.Tf(views.LangOf(s), k, args...)
}

func hasFormScene(r *http.Request) bool {
	return r != nil && strings.TrimSpace(r.FormValue(formKeySceneID)) != ""
}

func flushFormSceneIfPresent(s *state.AppState, r *http.Request) bool {
	if !hasFormScene(r) || isHistoryPreview(s) {
		return hasFormScene(r)
	}
	_ = saveSceneFromForm(s, r)
	return true
}

func frameNone(s *state.AppState, toast string, chunks ...templates.TemplateChunk) []engine.ActionFrame {
	s.Overlays.Toast = toast
	if len(chunks) == 0 {
		return nil
	}
	return []engine.ActionFrame{{Target: targetSelNone, Chunks: chunks}}
}

func frameSettingsKeepScroll(s *state.AppState, toast string) []engine.ActionFrame {
	s.Overlays.Toast = toast
	if s.Overlays.QR {
		return []engine.ActionFrame{{Target: targetSelModals, Chunks: views.ActiveOverlayViews(s)}}
	}
	return nil
}

func frameApp(s *state.AppState, toast string) []engine.ActionFrame {
	s.Overlays.Toast = toast
	if s.Confirm.IsOpen() {
		return frameModals(s, toast)
	}
	return []engine.ActionFrame{{Target: targetSelApp, Chunks: []templates.TemplateChunk{views.AppShellView(s)}}}
}

func frameAppResetCanvas(s *state.AppState, toast string) []engine.ActionFrame {
	s.Overlays.ResetCanvas = true
	return frameApp(s, toast)
}

func frameWorkspace(s *state.AppState, toast string) []engine.ActionFrame {
	s.Overlays.Toast = toast
	if s.Confirm.IsOpen() {
		return frameModals(s, toast)
	}
	return []engine.ActionFrame{{Target: targetSelWorkspace, Chunks: []templates.TemplateChunk{views.ActiveWorkspaceView(s)}}}
}

func frameModals(s *state.AppState, toast string) []engine.ActionFrame {
	s.Overlays.Toast = toast
	if s.Editor.SceneText.IsOpen() && state.ActiveWorkspaceView(s) == state.ViewEditor && !s.Config.StandaloneMode && !s.Editor.StudioHome {
		return []engine.ActionFrame{
			{Target: targetSelModals, Chunks: views.ActiveOverlayViews(s)},
			{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}},
		}
	}
	return []engine.ActionFrame{{Target: targetSelModals, Chunks: views.ActiveOverlayViews(s)}}
}

func frameInspector(s *state.AppState, toast string, hadFormScene bool) []engine.ActionFrame {
	s.Overlays.Toast = toast
	if s.Config.StandaloneMode {
		return []engine.ActionFrame{{Target: targetSelWorkspace, Chunks: []templates.TemplateChunk{views.PlayerView(s)}}}
	}
	if s.Editor.SceneText.IsOpen() {
		return []engine.ActionFrame{
			{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}},
			{Target: targetSelModals, Chunks: views.ActiveOverlayViews(s)},
		}
	}
	if hadFormScene {
		return []engine.ActionFrame{{Target: targetSelWorkspace, Chunks: []templates.TemplateChunk{views.StudioView(s)}}}
	}
	return []engine.ActionFrame{{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}}}
}

func frameSceneForm(s *state.AppState, toast string, hadFormScene bool, inspectorWithoutFormID bool) []engine.ActionFrame {
	s.Overlays.Toast = toast
	if s.Confirm.IsOpen() {
		return frameModals(s, toast)
	}
	if s.Editor.SceneText.IsOpen() {
		return []engine.ActionFrame{
			{Target: targetSelModals, Chunks: views.ActiveOverlayViews(s)},
			{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}},
		}
	}
	if inspectorWithoutFormID && !hadFormScene && state.ActiveWorkspaceView(s) == state.ViewEditor && !s.Config.StandaloneMode && !s.Editor.StudioHome {
		return []engine.ActionFrame{{Target: targetSelInspector, Chunks: []templates.TemplateChunk{views.EditorContextView(s)}}}
	}
	return frameWorkspace(s, toast)
}

func frameUndoRedo(s *state.AppState, toast string) []engine.ActionFrame {
	s.Overlays.Toast = toast
	if s.Confirm.IsOpen() {
		return frameModals(s, toast)
	}
	if s.Editor.SceneText.IsOpen() {
		return frameApp(s, toast)
	}
	return frameWorkspace(s, toast)
}

// HandleAction processes POST /api/action requests and writes multi-frame FDP responses.
func HandleAction(appState *state.AppState, writer http.ResponseWriter, request *http.Request, ensureTemplates func() map[string]*templates.ParsedTemplate) {
	if request.Method != http.MethodPost {
		http.Error(writer, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	if !engine.ValidateSameOriginAction(request) {
		http.Error(writer, errForbiddenOrigin, http.StatusForbidden)
		return
	}
	tAction := time.Now()
	engine.ParseActionRequest(request)
	act := strings.TrimSpace(request.FormValue(formFieldAction))
	handler := Handlers[act]

	appState.Mu.Lock()
	defer appState.Mu.Unlock()

	if !engine.IsLoopbackRequest(request) {
		lanOK := lanAllowedActions[act] || (act == actConfirmModalAccept && (appState.Confirm.PendingAction == pendingRewindStep || appState.Confirm.PendingAction == pendingResetProgress))
		if !lanOK {
			http.Error(writer, errForbiddenLAN, http.StatusForbidden)
			return
		}
	}
	if appState.Config.StandaloneMode {
		appState.Settings.ActiveView = state.ViewPlayer.String()
		if handler != nil && !standaloneAllowedActions[act] {
			http.Error(writer, errForbiddenStand, http.StatusForbidden)
			return
		}
	}

	if act != actSaveAudioTime && appState.Player.Book != nil && appState.Player.Save != nil {
		if rawAudioTime := strings.TrimSpace(request.FormValue(formFieldAudioTime)); rawAudioTime != "" {
			if t, err := strconv.ParseFloat(rawAudioTime, 64); err == nil && t >= 0 {
				if t > 0.2 || appState.Player.Save.UpdatedAtUnix > 0 || len(appState.Player.Save.History) > 0 {
					appState.Player.Save.AudioTime = t
					if !isHistoryPreview(appState) {
						if err := book.UpdateSaveAudioTimeInPlace(appState.Config.SavesDir, appState.Player.Book.ID, t, appState.Player.Book.StartSceneID); err != nil {
							appState.Player.PersistSave(appState.Config.SavesDir)
						}
					}
				}
			}
		}
	}

	if act == actSaveAudioTime || act == actStartWindowDrag {
		if handler != nil {
			_ = handler(appState, request)
		}
		engine.WriteRenderedActionFrames(writer, views.LangOf(appState), nil, []templates.TemplateChunk{views.AppShellScriptView(appState)}, nil)
		return
	}

	var actionFrames []engine.ActionFrame
	if handler != nil {
		tStep := time.Now()
		actionFrames = handler(appState, request)
		engine.Active.MarkStep(engine.StageActionHandler, tStep, act)
	} else {
		actionFrames = frameApp(appState, "")
	}

	if request.FormValue(formFieldSilent) == formValTrue && act != actAutoSaveScene {
		actionFrames = nil
	}

	if appState.Editor.SyncToSettings(appState.Settings) {
		tSave := time.Now()
		_ = appState.SaveSettings()
		engine.Active.MarkStep(engine.StageActionSaveSettings, tSave, act)
	}

	var loadedTemplates map[string]*templates.ParsedTemplate
	if ensureTemplates != nil && len(actionFrames) > 0 {
		loadedTemplates = ensureTemplates()
	}

	hasAppShell := false
	hasStudio := false
	for _, af := range actionFrames {
		if af.Target == targetSelApp {
			hasAppShell = true
		}
		if (af.Target == targetSelApp || af.Target == targetSelWorkspace) && state.ActiveWorkspaceView(appState) == state.ViewEditor {
			hasStudio = true
		}
	}

	globalScriptChunks := make([]templates.TemplateChunk, 0, 3)
	if !hasAppShell {
		globalScriptChunks = append(globalScriptChunks, views.AppShellScriptView(appState))
	}
	if !appState.Config.StandaloneMode && !hasStudio {
		globalScriptChunks = append(globalScriptChunks,
			views.StudioScriptView(appState),
			views.NodeCanvasScriptView(appState),
		)
	}

	appState.Overlays.VoicePeak = ""
	appState.Overlays.Toast = ""
	appState.Overlays.NativeCmd = ""
	appState.Overlays.ResetCanvas = false

	engine.WriteRenderedActionFrames(writer, views.LangOf(appState), actionFrames, globalScriptChunks, loadedTemplates)
	engine.Active.MarkStep(engine.StageActionTotal, tAction, act)
}

func switchView(s *state.AppState, r *http.Request) []engine.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	view := strings.TrimSpace(r.FormValue(formKeyView))
	prevView := ""
	if s.Settings != nil {
		prevView = s.Settings.ActiveView
	}
	wasActiveStudioEditor := prevView == state.ViewEditor.String() && s.Editor.HasActiveBook()
	if (view == state.ViewLibrary.String() || view == viewStudioAlias) && isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	if view == viewStudioAlias {
		if wasActiveStudioEditor {
			s.Editor.SaveBookUIState(s.Settings)
		}
		s.Editor.StudioHome = true
		s.Player.ZenMode = false
		s.Settings.ActiveView = state.ViewEditor.String()
		s.CloseAllModals()
		_ = s.Editor.SyncToSettings(s.Settings)
		_ = s.SaveSettings()
		return frameApp(s, "")
	}
	if view == state.ViewEditor.String() {
		s.Editor.StudioHome = false
	}
	if view == state.ViewPlayer.String() && s.Player.Book == nil {
		if s.Editor.ActiveBook != nil {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, s.Editor.ActiveBook)
		} else if len(s.Library.Books) > 0 {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, s.Library.Books[0])
		}
	}
	if view == state.ViewLibrary.String() || view == state.ViewPlayer.String() || view == state.ViewEditor.String() {
		if view != state.ViewEditor.String() && wasActiveStudioEditor {
			s.Editor.SaveBookUIState(s.Settings)
		}
		if view != state.ViewPlayer.String() {
			s.Player.ZenMode = false
		}
		s.Settings.ActiveView = view
		s.CloseAllModals()
		if view == state.ViewEditor.String() && !wasActiveStudioEditor {
			s.Editor.RestoreBookUIState(s.Settings, true)
		}
		s.Editor.SyncToSettings(s.Settings)
		_ = s.SaveSettings()
	}
	return frameApp(s, "")
}

func setDeviceMode(s *state.AppState, r *http.Request) []engine.ActionFrame {
	if s.Config.DevMode {
		mode := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyMode)))
		switch mode {
		case modePhone:
			mode = modePortrait
		case modeIPad:
			mode = modeTablet
		case modeDesktop:
			mode = modeFull
		}
		if mode == modeFull || mode == modePortrait || mode == modeTablet || mode == modeLandscape {
			s.Settings.PreviewMode = mode
			_ = s.SaveSettings()
		}
	}
	return frameSettingsKeepScroll(s, "")
}

func toggleFullscreen(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Overlays.NativeCmd = nativeCmdToggleFullscreen
	osbridge.ToggleNativeWindowFullscreen()
	return frameNone(s, "")
}

func startWindowDrag(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	osbridge.StartNativeWindowDrag()
	return frameNone(s, "")
}

func quitApp(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	s.Confirm.Reset()
	s.Overlays.NativeCmd = nativeCmdQuit
	go func() {
		s.Config.TriggerShutdown()
		osbridge.TerminateNativeWindow()
	}()
	return frameNone(s, "")
}
