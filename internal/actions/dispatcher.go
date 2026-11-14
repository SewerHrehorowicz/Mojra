//go:build !(js && wasm)

package actions

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mojra/internal/book"
	"mojra/internal/logging"
	"mojra/internal/state"
	"mojra/internal/tpl"
	"mojra/internal/views"
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

	headerContentType  = "Content-Type"
	headerSecFetchSite = "Sec-Fetch-Site"
	headerOrigin       = "Origin"
	headerReferer      = "Referer"
	fdpMimePrefix      = "text/x-fdp"
	wfpMimePrefix      = "text/x-wfp"
	secFetchSameOrigin = "same-origin"
	secFetchNone       = "none"
	originNull         = "null"
	hostLocalhost      = "localhost"

	errMethodNotAllowed = "method not allowed"
	errForbiddenOrigin  = "forbidden cross-origin action"
	errForbiddenLAN     = "forbidden remote LAN action"
	errForbiddenStand   = "forbidden in standalone book mode"
)

type stateSnapshot struct {
	hadPlayerBook bool
	hadEditorBook bool
	confirmOpen   bool
	confirmAction string
	sceneTextID   string
	hasFormScene  bool
}

type actionFrame struct {
	target string
	chunks []tpl.TemplateChunk
}

func isFDPRequestPayload(ct string, raw []byte) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), fdpMimePrefix) ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), wfpMimePrefix) {
		return true
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	return bytes.IndexByte(trimmed, '\n') >= 0 || !bytes.Contains(trimmed, []byte("&"))
}

// ParseActionRequest decodes an incoming action payload (FDP or URL-encoded form) into r.Form and r.PostForm.
func ParseActionRequest(r *http.Request) {
	if r == nil || r.Body == nil {
		return
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 32<<20)
	ct := r.Header.Get(headerContentType)
	buf := tpl.AcquireBuffer()
	defer tpl.ReleaseBuffer(buf)
	if _, err := io.Copy(buf, r.Body); err != nil {
		return
	}
	raw := buf.Bytes()
	if isFDPRequestPayload(ct, raw) {
		vals := make(url.Values, 16)
		if tpl.DecodeFDPBytes(raw, vals) {
			r.Form = vals
			r.PostForm = vals
			return
		}
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	_ = r.ParseForm()
}

func validateSameOriginAction(r *http.Request) bool {
	if secSite := strings.ToLower(strings.TrimSpace(r.Header.Get(headerSecFetchSite))); secSite != "" {
		if secSite != secFetchSameOrigin && secSite != secFetchNone {
			return false
		}
	}
	if origin := strings.TrimSpace(r.Header.Get(headerOrigin)); origin != "" {
		if strings.EqualFold(origin, originNull) {
			return false
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	if referer := strings.TrimSpace(r.Header.Get(headerReferer)); referer != "" {
		u, err := url.Parse(referer)
		if err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	return true
}

func isLoopbackRequest(r *http.Request) bool {
	if r == nil {
		return true
	}
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" {
		return true
	}
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	} else {
		host = strings.Trim(remoteAddr, "[]")
	}
	if strings.EqualFold(host, hostLocalhost) {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func resolveActionFrames(s *state.AppState, act string, spec ActionSpec, r *http.Request, before stateSnapshot) []actionFrame {
	if act == AutoSaveScene && state.ActiveWorkspaceView(s) == state.ViewEditor && !s.Config.StandaloneMode && !s.Editor.StudioHome {
		if s.Editor.SceneText.IsOpen() {
			return []actionFrame{
				{target: targetSelInspector, chunks: []tpl.TemplateChunk{views.EditorContextView(s)}},
				{target: targetSelNone, chunks: []tpl.TemplateChunk{views.TextEditorScriptView(s)}},
			}
		}
		if r.FormValue(formFieldSilent) == formValTrue {
			return []actionFrame{
				{target: targetSelNone, chunks: []tpl.TemplateChunk{views.EditorContextScriptView(s)}},
			}
		}
		return []actionFrame{
			{target: targetSelInspector, chunks: []tpl.TemplateChunk{views.EditorContextView(s)}},
		}
	}
	if r.FormValue(formFieldSilent) == formValTrue {
		return nil
	}
	if !before.confirmOpen && s.Confirm.IsOpen() {
		return []actionFrame{{target: targetSelModals, chunks: views.ActiveOverlayViews(s)}}
	}
	if before.confirmOpen && !s.Confirm.IsOpen() && act == ConfirmModalAccept {
		if before.confirmAction == RemoveChoice && s.Editor.SceneText.IsOpen() {
			return []actionFrame{
				{target: targetSelModals, chunks: views.ActiveOverlayViews(s)},
				{target: targetSelInspector, chunks: []tpl.TemplateChunk{views.EditorContextView(s)}},
			}
		}
		return []actionFrame{{target: targetSelApp, chunks: []tpl.TemplateChunk{views.AppShellView(s)}}}
	}
	var targetSelector string
	var chunks []tpl.TemplateChunk
	switch spec.Target {
	case targetNone:
		if s.Overlays.QR && spec.KeepScroll {
			return []actionFrame{{target: targetSelModals, chunks: views.ActiveOverlayViews(s)}}
		}
		if act == ToggleImageless && state.ActiveWorkspaceView(s) == state.ViewPlayer {
			return []actionFrame{{target: targetSelWorkspace, chunks: []tpl.TemplateChunk{views.PlayerView(s)}}}
		}
		return nil
	case targetInspector:
		if s.Config.StandaloneMode {
			targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.PlayerView(s)}
		} else if s.Editor.SceneText.IsOpen() {
			return []actionFrame{
				{target: targetSelInspector, chunks: []tpl.TemplateChunk{views.EditorContextView(s)}},
				{target: targetSelModals, chunks: views.ActiveOverlayViews(s)},
			}
		} else if before.hasFormScene {
			targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.StudioView(s)}
		} else {
			targetSelector, chunks = targetSelInspector, []tpl.TemplateChunk{views.EditorContextView(s)}
		}
	case targetModals:
		if act == OpenSceneTextModal && before.hasFormScene {
			return []actionFrame{{target: targetSelApp, chunks: []tpl.TemplateChunk{views.AppShellView(s)}}}
		}
		if s.Editor.SceneText.IsOpen() && state.ActiveWorkspaceView(s) == state.ViewEditor && !s.Config.StandaloneMode && !s.Editor.StudioHome {
			return []actionFrame{
				{target: targetSelModals, chunks: views.ActiveOverlayViews(s)},
				{target: targetSelInspector, chunks: []tpl.TemplateChunk{views.EditorContextView(s)}},
			}
		}
		return []actionFrame{{target: targetSelModals, chunks: views.ActiveOverlayViews(s)}}
	case targetSceneForm:
		if s.Editor.SceneText.IsOpen() {
			reqSc := strings.TrimSpace(r.FormValue(formKeyScene))
			if (act == AddBookVariable && s.Editor.RightPanel == state.PanelVariables) ||
				(act == AddChoice && !before.hasFormScene && reqSc != "" && !strings.EqualFold(reqSc, before.sceneTextID)) {
				return []actionFrame{{target: targetSelApp, chunks: []tpl.TemplateChunk{views.AppShellView(s)}}}
			}
			return []actionFrame{
				{target: targetSelModals, chunks: views.ActiveOverlayViews(s)},
				{target: targetSelInspector, chunks: []tpl.TemplateChunk{views.EditorContextView(s)}},
			}
		}
		if spec.InspectorWithoutFormID && !before.hasFormScene && state.ActiveWorkspaceView(s) == state.ViewEditor && !s.Config.StandaloneMode && !s.Editor.StudioHome {
			return []actionFrame{{target: targetSelInspector, chunks: []tpl.TemplateChunk{views.EditorContextView(s)}}}
		}
		targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.ActiveWorkspaceView(s)}
	case targetUndoRedo:
		if s.Editor.SceneText.IsOpen() {
			return []actionFrame{{target: targetSelApp, chunks: []tpl.TemplateChunk{views.AppShellView(s)}}}
		}
		targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.ActiveWorkspaceView(s)}
	case targetWorkspace:
		if act == CloseSceneTextModal {
			if state.ActiveWorkspaceView(s) != state.ViewEditor || s.Config.StandaloneMode {
				return []actionFrame{{target: targetSelModals, chunks: views.ActiveOverlayViews(s)}}
			}
			return []actionFrame{{target: targetSelApp, chunks: []tpl.TemplateChunk{views.AppShellView(s)}}}
		}
		targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.ActiveWorkspaceView(s)}
	default:
		return []actionFrame{{target: targetSelApp, chunks: []tpl.TemplateChunk{views.AppShellView(s)}}}
	}
	if ((!before.hadPlayerBook && s.Player.Book != nil) || (!before.hadEditorBook && s.Editor.ActiveBook != nil)) && targetSelector == targetSelWorkspace {
		return []actionFrame{{target: targetSelApp, chunks: []tpl.TemplateChunk{views.AppShellView(s)}}}
	}
	return []actionFrame{{target: targetSelector, chunks: chunks}}
}

// HandleAction processes POST /api/action requests and writes multi-frame FDP responses.
func HandleAction(s *state.AppState, w http.ResponseWriter, r *http.Request, ensureTemplates func() map[string]*tpl.ParsedTemplate) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	if !validateSameOriginAction(r) {
		http.Error(w, errForbiddenOrigin, http.StatusForbidden)
		return
	}
	tAction := time.Now()
	ParseActionRequest(r)
	act := strings.TrimSpace(r.FormValue(formFieldAction))
	spec := ActionSpecs[act]

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !isLoopbackRequest(r) {
		lanOK := spec.LANAllowed || (act == ConfirmModalAccept && (s.Confirm.PendingAction == RewindStep || s.Confirm.PendingAction == ResetProgress))
		if !lanOK {
			http.Error(w, errForbiddenLAN, http.StatusForbidden)
			return
		}
	}
	if s.Config.StandaloneMode {
		s.Settings.ActiveView = state.ViewPlayer.String()
		if spec.StandaloneForbidden {
			http.Error(w, errForbiddenStand, http.StatusForbidden)
			return
		}
	}

	if act != SaveAudioTime && s.Player.Book != nil && s.Player.Save != nil {
		if rawAudioTime := strings.TrimSpace(r.FormValue(formFieldAudioTime)); rawAudioTime != "" {
			if t, err := strconv.ParseFloat(rawAudioTime, 64); err == nil && t >= 0 {
				if t > 0.2 || s.Player.Save.UpdatedAtUnix > 0 || len(s.Player.Save.History) > 0 {
					s.Player.Save.AudioTime = t
					if !isHistoryPreview(s) {
						if err := book.UpdateSaveAudioTimeInPlace(s.Config.SavesDir, s.Player.Book.ID, t, s.Player.Book.StartSceneID); err != nil {
							s.Player.PersistSave(s.Config.SavesDir)
						}
					}
				}
			}
		}
	}

	before := stateSnapshot{
		hadPlayerBook: s.Player.Book != nil,
		hadEditorBook: s.Editor.ActiveBook != nil,
		confirmOpen:   s.Confirm.IsOpen(),
		confirmAction: s.Confirm.PendingAction,
		sceneTextID:   s.Editor.SceneText.SceneID,
		hasFormScene:  strings.TrimSpace(r.FormValue(formKeySceneID)) != "",
	}
	toast := ""
	nativeCmd := spec.NativeCmd
	if spec.Handler != nil {
		tStep := time.Now()
		if spec.FlushFormScene {
			flushFormSceneIfPresent(s, r)
		}
		toast = spec.Handler(s, r)
		if act == ConfirmModalAccept && before.confirmAction == QuitApp {
			nativeCmd = nativeCmdQuit
		}
		logging.Active.MarkStep(logging.StageActionHandler, tStep, act)
	}

	if act == SaveAudioTime || act == StartWindowDrag {
		w.Header().Set(headerContentType, tpl.ContentTypeFDP)
		_ = tpl.WriteFDPFrames(w, tpl.FDPFrame{Target: targetSelNone, Chunk: views.AppShellScriptView(s)})
		return
	}

	if s.Editor.SyncToSettings(s.Settings) {
		tSave := time.Now()
		_ = s.SaveSettings()
		logging.Active.MarkStep(logging.StageActionSaveSettings, tSave, act)
	}

	s.Overlays.Toast = toast
	s.Overlays.NativeCmd = nativeCmd
	s.Overlays.ResetCanvas = spec.ResetCanvas || (act == AddChoice && !s.Editor.SceneText.IsOpen())

	tBuildView := time.Now()
	actionFrames := resolveActionFrames(s, act, spec, r, before)
	logging.Active.MarkStep(logging.StageActionBuildView, tBuildView, act)

	var templates map[string]*tpl.ParsedTemplate
	if ensureTemplates != nil && len(actionFrames) > 0 {
		templates = ensureTemplates()
	}
	lang := views.LangOf(s)

	fdpFrames := make([]tpl.FDPFrame, 0, len(actionFrames)+2)
	hasAppShell := false
	hasStudio := false

	for _, af := range actionFrames {
		if af.target == targetSelApp {
			hasAppShell = true
		}
		if (af.target == targetSelApp || af.target == targetSelWorkspace) && state.ActiveWorkspaceView(s) == state.ViewEditor {
			hasStudio = true
		}
		if af.target == targetSelNone {
			for _, ch := range af.chunks {
				fdpFrames = append(fdpFrames, tpl.FDPFrame{
					Target: targetSelNone,
					Chunk:  ch,
				})
			}
			continue
		}
		if len(af.chunks) == 0 {
			fdpFrames = append(fdpFrames, tpl.FDPFrame{
				Target: af.target,
			})
			continue
		}
		htmlBuf := tpl.AcquireBuffer()
		tRender := time.Now()
		for _, chunk := range af.chunks {
			tplName, tplData := chunk.Resolve()
			if tplName != "" && templates != nil {
				ctx := views.NewViewContext(lang, tplData)
				if err := tpl.RenderTemplateTo(htmlBuf, tplName, templates, ctx); err != nil {
					tpl.ReleaseBuffer(htmlBuf)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		logging.Active.MarkStep(logging.StageActionRenderTpl, tRender, af.target)
		bodyCopy := append([]byte(nil), htmlBuf.Bytes()...)
		tpl.ReleaseBuffer(htmlBuf)

		fdpFrames = append(fdpFrames, tpl.FDPFrame{
			Target:   af.target,
			HTMLBody: bodyCopy,
			Chunk:    af.chunks[0],
		})
		for i := 1; i < len(af.chunks); i++ {
			if af.chunks[i].ScriptData != nil {
				fdpFrames = append(fdpFrames, tpl.FDPFrame{
					Target: targetSelNone,
					Chunk:  tpl.TemplateChunk{ScriptData: af.chunks[i].ScriptData},
				})
			}
		}
	}

	if !hasAppShell {
		fdpFrames = append(fdpFrames, tpl.FDPFrame{
			Target: targetSelNone,
			Chunk:  views.AppShellScriptView(s),
		})
	}
	if !s.Config.StandaloneMode && !hasStudio {
		fdpFrames = append(fdpFrames, tpl.FDPFrame{
			Target: targetSelNone,
			Chunk:  views.StudioScriptView(s),
		})
	}

	s.Overlays.VoicePeak = ""
	s.Overlays.Toast = ""
	s.Overlays.NativeCmd = ""
	s.Overlays.ResetCanvas = false

	w.Header().Set(headerContentType, tpl.ContentTypeFDP)
	_ = tpl.WriteFDPFrames(w, fdpFrames...)
	logging.Active.MarkStep(logging.StageActionTotal, tAction, act)
}
