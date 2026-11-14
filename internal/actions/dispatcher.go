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
	targetSelInspector = "#editor-context-panel"
	targetSelNone      = "none"

	modalIDConfirm        = "modal-confirm"
	modalIDQR             = "modal-qr"
	modalIDNewBook        = "modal-new-book"
	modalIDExport         = "modal-export"
	modalIDAddBook        = "modal-add-book"
	modalIDStudioProjects = "modal-studio-projects"
	modalIDSceneText      = "modal-scene-text"

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

func activeModalID(s *state.AppState) string {
	switch {
	case s.ModalConfirm.IsOpen():
		return modalIDConfirm
	case s.Modals.QR:
		return modalIDQR
	case s.ModalNewBook.Open:
		return modalIDNewBook
	case s.ModalExport.Open:
		return modalIDExport
	case s.Modals.AddBook:
		return modalIDAddBook
	case s.Modals.StudioProjects:
		return modalIDStudioProjects
	case s.Editor.SceneText.IsOpen():
		return modalIDSceneText
	default:
		return ""
	}
}

func resolveActionChunks(s *state.AppState, act string, spec ActionSpec, r *http.Request, before stateSnapshot) (string, string, []tpl.TemplateChunk) {
	if r.FormValue(formFieldSilent) == formValTrue {
		return targetSelNone, "", nil
	}
	if !before.confirmOpen && s.ModalConfirm.IsOpen() {
		return targetSelModals, modalIDConfirm, views.ViewActiveModalChunks(s)
	}
	if before.confirmOpen && !s.ModalConfirm.IsOpen() && act == ConfirmModalAccept {
		if before.confirmAction == RemoveChoice && s.Editor.SceneText.IsOpen() {
			return targetSelModals, modalIDSceneText, views.ViewActiveModalChunks(s)
		}
		return targetSelApp, "", []tpl.TemplateChunk{views.ViewAppShell(s)}
	}
	var targetSelector, modalID string
	var chunks []tpl.TemplateChunk
	switch spec.Target {
	case targetNone:
		if s.Modals.QR && spec.KeepScroll {
			return targetSelModals, modalIDQR, views.ViewActiveModalChunks(s)
		}
		if act == ToggleImageless && state.ActiveWorkspaceView(s) == state.ViewPlayer {
			return targetSelWorkspace, "", []tpl.TemplateChunk{views.ViewPlayer(s)}
		}
		return targetSelNone, "", nil
	case targetInspector:
		if s.Config.StandaloneMode {
			targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.ViewPlayer(s)}
		} else if s.Editor.SceneText.IsOpen() {
			targetSelector, chunks = targetSelApp, []tpl.TemplateChunk{views.ViewAppShell(s)}
		} else if before.hasFormScene {
			targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.ViewStudio(s)}
		} else {
			targetSelector, chunks = targetSelInspector, []tpl.TemplateChunk{views.ViewEditorContextPanel(s)}
		}
	case targetModals:
		if act == OpenSceneTextModal && before.hasFormScene {
			return targetSelApp, "", []tpl.TemplateChunk{views.ViewAppShell(s)}
		}
		return targetSelModals, activeModalID(s), views.ViewActiveModalChunks(s)
	case targetSceneForm:
		if s.Editor.SceneText.IsOpen() {
			reqSc := strings.TrimSpace(r.FormValue(formKeyScene))
			if (act == AddBookVariable && s.Editor.RightPanel == state.PanelVariables) ||
				(act == AddChoice && !before.hasFormScene && reqSc != "" && !strings.EqualFold(reqSc, before.sceneTextID)) {
				return targetSelApp, "", []tpl.TemplateChunk{views.ViewAppShell(s)}
			}
			return targetSelModals, modalIDSceneText, views.ViewActiveModalChunks(s)
		}
		if spec.InspectorWithoutFormID && !before.hasFormScene && state.ActiveWorkspaceView(s) == state.ViewEditor && !s.Config.StandaloneMode && !s.Editor.StudioHome {
			return targetSelInspector, "", []tpl.TemplateChunk{views.ViewEditorContextPanel(s)}
		}
		targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.ViewActiveWorkspaceChunk(s)}
	case targetUndoRedo:
		if s.Editor.SceneText.IsOpen() {
			return targetSelApp, "", []tpl.TemplateChunk{views.ViewAppShell(s)}
		}
		targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.ViewActiveWorkspaceChunk(s)}
	case targetWorkspace:
		if act == CloseSceneTextModal {
			if state.ActiveWorkspaceView(s) != state.ViewEditor || s.Config.StandaloneMode {
				return targetSelModals, modalIDSceneText, views.ViewActiveModalChunks(s)
			}
			return targetSelApp, "", []tpl.TemplateChunk{views.ViewAppShell(s)}
		}
		targetSelector, chunks = targetSelWorkspace, []tpl.TemplateChunk{views.ViewActiveWorkspaceChunk(s)}
	default:
		return targetSelApp, "", []tpl.TemplateChunk{views.ViewAppShell(s)}
	}
	if ((!before.hadPlayerBook && s.Player.Book != nil) || (!before.hadEditorBook && s.Editor.ActiveBook != nil)) && targetSelector == targetSelWorkspace {
		return targetSelApp, "", []tpl.TemplateChunk{views.ViewAppShell(s)}
	}
	return targetSelector, modalID, chunks
}

// HandleAction processes POST /api/action requests and writes an FDP response frame.
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
		lanOK := spec.LANAllowed || (act == ConfirmModalAccept && (s.ModalConfirm.PendingAction == RewindStep || s.ModalConfirm.PendingAction == ResetProgress))
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
		confirmOpen:   s.ModalConfirm.IsOpen(),
		confirmAction: s.ModalConfirm.PendingAction,
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
		_ = tpl.WriteFDPResponse(w, tpl.FDPFrameMeta{Target: targetSelNone, KeepScroll: true}, nil)
		return
	}

	if s.Editor.SyncToSettings(s.Settings) {
		tSave := time.Now()
		_ = s.SaveSettings()
		logging.Active.MarkStep(logging.StageActionSaveSettings, tSave, act)
	}

	tBuildView := time.Now()
	targetSelector, modalID, chunks := resolveActionChunks(s, act, spec, r, before)
	logging.Active.MarkStep(logging.StageActionBuildView, tBuildView, act)

	meta := tpl.FDPFrameMeta{
		Target:      targetSelector,
		KeepScroll:  spec.KeepScroll,
		ResetCanvas: spec.ResetCanvas || (act == AddChoice && !s.Editor.SceneText.IsOpen()),
		Toast:       toast,
		NativeCmd:   nativeCmd,
		VoicePeak:   s.Modals.VoicePeak,
		ModalID:     modalID,
	}
	s.Modals.VoicePeak = ""

	htmlBuf := tpl.AcquireBuffer()
	defer tpl.ReleaseBuffer(htmlBuf)
	if targetSelector != targetSelNone && len(chunks) > 0 && ensureTemplates != nil {
		templates := ensureTemplates()
		tRender := time.Now()
		lang := views.LangOf(s)
		for _, chunk := range chunks {
			tplName, tplData := chunk.Resolve()
			if tplName != "" {
				ctx := views.NewViewContext(lang, tplData)
				if err := tpl.RenderTemplateTo(htmlBuf, tplName, templates, ctx); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		logging.Active.MarkStep(logging.StageActionRenderTpl, tRender, targetSelector)
	}

	allChunks := make([]tpl.TemplateChunk, 0, len(chunks)+2)
	allChunks = append(allChunks, chunks...)
	if targetSelector != targetSelApp {
		allChunks = append(allChunks, views.ViewAppShellScript(s))
	}
	studioRendered := (targetSelector == targetSelApp || targetSelector == targetSelWorkspace) &&
		state.ActiveWorkspaceView(s) == state.ViewEditor
	if !s.Config.StandaloneMode && !studioRendered {
		allChunks = append(allChunks, views.ViewStudioScript(s))
	}

	w.Header().Set(headerContentType, tpl.ContentTypeFDP)
	_ = tpl.WriteFDPResponse(w, meta, htmlBuf.Bytes(), allChunks...)
	logging.Active.MarkStep(logging.StageActionTotal, tAction, act)
}
