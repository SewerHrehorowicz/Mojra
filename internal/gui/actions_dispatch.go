package gui

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"storyteller/internal/book"
	"storyteller/internal/tpl"
)

type actionTargetKind uint8

const (
	targetAppShell actionTargetKind = iota
	targetNone
	targetModals
	targetPlayerView
	targetActiveWorkspace
	targetActiveWorkspaceOrModalsWhenSceneTextOpen
	targetUndoRedoWorkspaceOrAppShell
	targetThemeOrScale
	targetImageless
	targetOpenSceneTextModal
	targetCloseSceneTextModal
	targetSelectStudioScene
	targetBranchOrAudioEnded
	targetConfirmModalAccept
)

// ActionSpec centralizes metadata and DOM chunk target rules for a POST /api/action command.
type ActionSpec struct {
	StandaloneForbidden bool
	KeepScroll          bool
	ResetCanvas         bool
	Target              actionTargetKind
}

var actionSpecs = map[string]ActionSpec{
	// Navigation & Global UI
	"SwitchView":       {StandaloneForbidden: true, Target: targetAppShell},
	"OpenBook":         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell},
	"ToggleTheme":      {KeepScroll: true, Target: targetThemeOrScale},
	"SetUIScale":       {KeepScroll: true, Target: targetThemeOrScale},
	"ToggleUIScale":    {KeepScroll: true, Target: targetThemeOrScale},
	"SetDeviceMode":    {StandaloneForbidden: true, KeepScroll: true, Target: targetThemeOrScale},
	"ToggleImageless":  {KeepScroll: true, Target: targetImageless},
	"ToggleFullscreen": {KeepScroll: true, Target: targetNone},
	"StartWindowDrag":  {Target: targetNone},
	"QuitApp":          {KeepScroll: true, Target: targetNone},
	"PromptQuitApp":    {KeepScroll: true, Target: targetNone},

	// Modals & Confirmations
	"OpenQRModal":             {KeepScroll: true, Target: targetModals},
	"CloseQRModal":            {KeepScroll: true, Target: targetModals},
	"OpenSettingsModal":       {KeepScroll: true, Target: targetModals},
	"CloseSettingsModal":      {KeepScroll: true, Target: targetModals},
	"OpenEditorHelpModal":     {StandaloneForbidden: true, KeepScroll: true, Target: targetModals},
	"CloseEditorHelpModal":    {StandaloneForbidden: true, KeepScroll: true, Target: targetModals},
	"OpenNewBookModal":        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals},
	"CloseNewBookModal":       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals},
	"OpenAddBookModal":        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals},
	"CloseAddBookModal":       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals},
	"PromptRewindStep":        {KeepScroll: true, Target: targetModals},
	"PromptResetProgress":     {KeepScroll: true, Target: targetModals},
	"PromptRemoveBook":        {StandaloneForbidden: true, Target: targetModals},
	"PromptDeleteScene":       {StandaloneForbidden: true, Target: targetModals},
	"PromptDeleteScenes":      {StandaloneForbidden: true, Target: targetModals},
	"PromptRemoveChoice":      {StandaloneForbidden: true, KeepScroll: true, Target: targetModals},
	"ConfirmModalClose":       {KeepScroll: true, Target: targetModals},
	"ConfirmModalAccept":      {Target: targetConfirmModalAccept},
	"OpenSceneTextModal":      {StandaloneForbidden: true, Target: targetOpenSceneTextModal},
	"CloseSceneTextModal":     {StandaloneForbidden: true, Target: targetCloseSceneTextModal},
	"ToggleSceneTextSettings": {StandaloneForbidden: true, Target: targetModals},
	"NavigateSceneTextModal":  {StandaloneForbidden: true, Target: targetModals},
	"SaveSceneTextModal":      {Target: targetModals},

	// Library & Book Packaging
	"CreateNewBook":         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell},
	"BrowseAddBookFromDisk": {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell},
	"AddBookFromDisk":       {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell},
	"AddBookFromURL":        {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell},
	"RemoveBook":            {StandaloneForbidden: true, Target: targetAppShell},
	"PackBook":              {StandaloneForbidden: true, Target: targetNone},
	"ExtractEmbeddedBook":   {StandaloneForbidden: true, Target: targetAppShell},

	// Player & Audio
	"ChooseBranch":        {Target: targetBranchOrAudioEnded},
	"AutoAdvanceImplicit": {Target: targetBranchOrAudioEnded},
	"AudioEnded":          {Target: targetBranchOrAudioEnded},
	"RewindStepNow":       {Target: targetPlayerView},
	"RevealHistory":       {Target: targetPlayerView},
	"FocusHistoryStep":    {Target: targetPlayerView},
	"FocusPresent":        {Target: targetPlayerView},
	"CycleAudioSpeed":     {KeepScroll: true, Target: targetNone},
	"ToggleAutoChoice":    {KeepScroll: true, Target: targetNone},
	"ToggleAutoStrategy":  {KeepScroll: true, Target: targetNone},
	"PlayFromScene":       {StandaloneForbidden: true, Target: targetAppShell},

	// Node Studio & Editor
	"SelectStudioScene":        {StandaloneForbidden: true, Target: targetSelectStudioScene},
	"SelectStudioChapter":      {StandaloneForbidden: true, Target: targetSelectStudioScene},
	"OpenBookSettings":         {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"CloseBookSettings":        {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"ToggleBookSettings":       {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"SaveBookSettings":         {StandaloneForbidden: true, Target: targetAppShell},
	"SaveScene":                {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"AutoSaveScene":            {StandaloneForbidden: true, Target: targetNone},
	"SaveEditorState":          {StandaloneForbidden: true, Target: targetNone},
	"RenameBook":               {StandaloneForbidden: true, Target: targetAppShell},
	"ToggleBookNarrated":       {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"ToggleBookImageless":      {StandaloneForbidden: true, Target: targetAppShell},
	"SetStartScene":            {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen},
	"SetBookStartChapter":      {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"SetChapterStartScene":     {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"SaveChapterSettings":      {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"BrowseAddBookMedia":       {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen},
	"AddBookMediaFromDisk":     {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen},
	"ConnectChoice":            {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen},
	"ConnectChapter":           {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen},
	"AddChoice":                {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen},
	"RemoveChoice":             {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen},
	"BranchChoiceScene":        {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen},
	"InsertScene":              {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"InsertSceneBefore":        {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"CreateSceneAt":            {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"DeleteScene":              {StandaloneForbidden: true, Target: targetAppShell},
	"DeleteScenes":             {StandaloneForbidden: true, Target: targetAppShell},
	"AutoLayoutNodes":          {StandaloneForbidden: true, ResetCanvas: true, Target: targetActiveWorkspace},
	"RenameChapter":            {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"AssignChapterToScenes":    {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"CreateChapterGroup":       {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"UndoEditor":               {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell},
	"RedoEditor":               {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell},
	"RestoreHistoryRevision":   {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell},
	"ToggleEditorHistoryPanel": {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"OpenEditorHistoryPanel":   {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"CloseEditorHistoryPanel":  {StandaloneForbidden: true, Target: targetActiveWorkspace},
	"SaveHistoryVersion":       {StandaloneForbidden: true, Target: targetActiveWorkspace},
}

func isStandaloneForbiddenAction(action string) bool {
	return actionSpecs[action].StandaloneForbidden
}

func isKeepScrollAction(action string) bool {
	return actionSpecs[action].KeepScroll
}

func isResetCanvasAction(action string) bool {
	return actionSpecs[action].ResetCanvas
}

// resolveActionChunkTarget returns the template name to render and the client DOM selector target (or "none" for 204).
func (s *Server) resolveActionChunkTarget(action, prevConfirmAction string, hasFormSceneID bool) (string, string) {
	spec, ok := actionSpecs[action]
	if !ok {
		return "app_shell", "#app"
	}
	switch spec.Target {
	case targetNone:
		return "", "none"
	case targetModals:
		return "modals", "#modals-root"
	case targetPlayerView:
		return "player_view", "#workspace"
	case targetActiveWorkspace:
		return s.activeWorkspaceTemplateName(), "#workspace"
	case targetActiveWorkspaceOrModalsWhenSceneTextOpen:
		if s.SceneTextModalID != "" {
			return "modals", "#modals-root"
		}
		return s.activeWorkspaceTemplateName(), "#workspace"
	case targetUndoRedoWorkspaceOrAppShell:
		if s.SceneTextModalID != "" {
			return "app_shell", "#app"
		}
		return s.activeWorkspaceTemplateName(), "#workspace"
	case targetThemeOrScale:
		if s.QRModalOpen {
			return "modals", "#modals-root"
		}
		return "", "none"
	case targetImageless:
		if s.QRModalOpen {
			return "modals", "#modals-root"
		}
		if s.activeWorkspaceView() == "player" {
			return "player_view", "#workspace"
		}
		return "", "none"
	case targetOpenSceneTextModal:
		if hasFormSceneID {
			return "app_shell", "#app"
		}
		return "modals", "#modals-root"
	case targetCloseSceneTextModal:
		if s.activeWorkspaceView() == "editor" && !s.StandaloneMode {
			return "editor_view", "#workspace"
		}
		return "modals", "#modals-root"
	case targetSelectStudioScene:
		if !s.StandaloneMode {
			if hasFormSceneID {
				return "editor_view", "#workspace"
			}
			return "editor_context_panel", "#editor-context-panel"
		}
		return "player_view", "#workspace"
	case targetBranchOrAudioEnded:
		if s.ConfirmPendingAction != "" {
			return "modals", "#modals-root"
		}
		return "player_view", "#workspace"
	case targetConfirmModalAccept:
		if prevConfirmAction == "RemoveChoice" && s.SceneTextModalID != "" {
			return "modals", "#modals-root"
		}
		return "app_shell", "#app"
	default:
		return "app_shell", "#app"
	}
}

func validateSameOriginAction(r *http.Request) bool {
	if secSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); secSite != "" {
		if secSite != "same-origin" && secSite != "none" {
			return false
		}
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		if strings.EqualFold(origin, "null") {
			return false
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	if referer := strings.TrimSpace(r.Header.Get("Referer")); referer != "" {
		u, err := url.Parse(referer)
		if err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	return true
}

func (s *Server) handleGlobalOrModalAction(action string, w http.ResponseWriter, r *http.Request) (toast, nativeCmd string, wroteResponse bool) {
	switch action {
	case "SwitchView":
		view := strings.TrimSpace(r.FormValue("view"))
		if (view == "player" || view == "editor") && s.ActiveBook == nil {
			if books, err := s.loadBooks(); err == nil && len(books) > 0 {
				s.setActiveBook(books[0])
			}
		}
		if view == "library" || view == "player" || view == "editor" {
			s.Settings.ActiveView = view
			s.closeAllModals()
			_ = book.SaveAppSettings(s.SavesDir, s.Settings)
		}

	case "ToggleTheme":
		if s.Settings.Theme == "light" {
			s.Settings.Theme = "dark"
		} else {
			s.Settings.Theme = "light"
		}
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)

	case "ToggleImageless":
		if !s.isBookImageless() {
			s.Settings.Imageless = !s.Settings.Imageless
			_ = book.SaveAppSettings(s.SavesDir, s.Settings)
		}

	case "ToggleUIScale", "SetUIScale":
		reqScale := strings.ToLower(strings.TrimSpace(r.FormValue("scale")))
		if reqScale == "" {
			reqScale = strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
		}
		if reqScale == "" {
			switch strings.ToLower(strings.TrimSpace(r.FormValue("dir"))) {
			case "in", "up", "plus", "+":
				reqScale = "large"
			case "out", "down", "minus", "-":
				reqScale = "small"
			}
		}
		if reqScale == "large" || reqScale == "small" {
			s.Settings.UIScale = reqScale
		} else if s.Settings.UIScale == "large" {
			s.Settings.UIScale = "small"
		} else {
			s.Settings.UIScale = "large"
		}
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)

	case "SetDeviceMode":
		if s.DevMode {
			mode := strings.TrimSpace(r.FormValue("mode"))
			if mode == "full" || mode == "portrait" || mode == "landscape" {
				s.Settings.PreviewMode = mode
				_ = book.SaveAppSettings(s.SavesDir, s.Settings)
			}
		}

	case "ToggleFullscreen":
		ToggleNativeWindowFullscreen()
		nativeCmd = "toggle-fullscreen"

	case "StartWindowDrag":
		StartNativeWindowDrag()
		w.WriteHeader(http.StatusNoContent)
		return "", "", true

	case "OpenQRModal", "OpenSettingsModal":
		s.QRModalOpen = true

	case "CloseQRModal", "CloseSettingsModal":
		s.QRModalOpen = false

	case "OpenEditorHelpModal":
		s.EditorHelpModalOpen = true

	case "CloseEditorHelpModal":
		s.EditorHelpModalOpen = false

	case "OpenNewBookModal":
		s.QRModalOpen = false
		s.AddBookModalOpen = false
		s.NewBookModalOpen = true

	case "CloseNewBookModal":
		s.NewBookModalOpen = false

	case "OpenAddBookModal":
		s.QRModalOpen = false
		s.NewBookModalOpen = false
		s.AddBookModalOpen = true

	case "CloseAddBookModal":
		s.AddBookModalOpen = false

	case "QuitApp", "PromptQuitApp":
		s.ConfirmPendingAction = ""
		s.ConfirmPendingParam = ""
		nativeCmd = "quit"
		go func() {
			s.TriggerShutdown()
			TerminateNativeWindow()
		}()

	case "ConfirmModalClose":
		s.ConfirmPendingAction = ""
		s.ConfirmPendingParam = ""

	case "ConfirmModalAccept":
		toast, nativeCmd = s.acceptConfirmModal()
	}
	return toast, nativeCmd, false
}

func (s *Server) acceptConfirmModal() (toast, nativeCmd string) {
	pending := s.ConfirmPendingAction
	param := s.ConfirmPendingParam
	s.ConfirmPendingAction = ""
	s.ConfirmPendingParam = ""

	switch pending {
	case "RewindStep":
		if stepIdx, err := strconv.Atoi(param); err == nil && s.Save != nil && stepIdx >= 0 && stepIdx < len(s.Save.History) {
			targetSceneID := s.Save.History[stepIdx].SceneID
			s.Save.History = s.Save.History[:stepIdx]
			s.Save.CurrentSceneID = targetSceneID
			s.Save.AudioTime = 0
			s.HistoryRevealed = false
			s.FocusedHistoryIdx = -1
			_ = book.WriteSaveState(s.SavesDir, s.Save)
			toast = "Cofnięto opowieść do wybranej sceny."
		}
	case "ResetProgress":
		targetID := param
		if targetID == "" && s.ActiveBook != nil {
			targetID = s.ActiveBook.ID
		}
		if targetID != "" {
			_ = book.DeleteSaveState(s.SavesDir, targetID)
			if s.ActiveBook != nil && s.ActiveBook.ID == targetID {
				s.Save = book.LoadSaveState(s.SavesDir, s.ActiveBook.ID, s.ActiveBook.StartSceneID)
				s.HistoryRevealed = false
				s.FocusedHistoryIdx = -1
			}
			toast = "Postęp został zresetowany do początku."
		}
	case "RemoveBook":
		toast = s.removeBookByID(param)
	case "DeleteScene":
		toast = s.deleteSceneByID(param)
	case "DeleteScenes":
		toast = s.deleteScenesByIDs(param)
	case "RemoveChoice":
		if sep := strings.LastIndex(param, ":"); sep > 0 {
			scID := param[:sep]
			if choiceIdx, err := strconv.Atoi(param[sep+1:]); err == nil {
				toast = s.removeChoiceBySceneAndIndex(scID, choiceIdx)
			}
		}
	case "QuitApp":
		nativeCmd = "quit"
		go func() {
			s.TriggerShutdown()
			TerminateNativeWindow()
		}()
	}
	return toast, nativeCmd
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !validateSameOriginAction(r) {
		http.Error(w, "forbidden cross-origin action", http.StatusForbidden)
		return
	}
	_ = r.ParseForm()
	action := strings.TrimSpace(r.FormValue("action"))

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.StandaloneMode {
		s.Settings.ActiveView = "player"
		if isStandaloneForbiddenAction(action) {
			http.Error(w, "forbidden in standalone book mode", http.StatusForbidden)
			return
		}
	}

	toast := ""
	nativeCmd := ""
	prevConfirmAction := s.ConfirmPendingAction
	hadActiveBook := s.ActiveBook != nil

	// If the client sends its current audio_time along with a navigation/switch action, persist it for the active book before switching.
	if action != "SaveAudioTime" && s.ActiveBook != nil && s.Save != nil {
		if rawAudioTime := strings.TrimSpace(r.FormValue("audio_time")); rawAudioTime != "" {
			if t, err := strconv.ParseFloat(rawAudioTime, 64); err == nil && t >= 0 {
				if t > 0.2 || s.Save.UpdatedAtUnix > 0 || len(s.Save.History) > 0 {
					s.Save.AudioTime = t
					_ = book.UpdateSaveAudioTimeInPlace(s.SavesDir, s.ActiveBook.ID, t, s.ActiveBook.StartSceneID)
				}
			}
		}
	}

	if t, wrote, ok := s.handlePlayerAction(action, w, r); ok {
		if wrote {
			return
		}
		toast = t
	} else if t, wrote, ok := s.handleEditorAction(action, w, r); ok {
		if wrote {
			return
		}
		toast = t
	} else if t, wrote, ok := s.handleLibraryAction(action, w, r); ok {
		if wrote {
			return
		}
		toast = t
	} else {
		var wrote bool
		toast, nativeCmd, wrote = s.handleGlobalOrModalAction(action, w, r)
		if wrote {
			return
		}
	}

	targetTpl, targetSelector := s.resolveActionChunkTarget(action, prevConfirmAction, strings.TrimSpace(r.FormValue("scene_id")) != "")
	if !hadActiveBook && s.ActiveBook != nil && targetSelector == "#workspace" {
		targetTpl = "app_shell"
		targetSelector = "#app"
	}

	if targetSelector != "none" {
		s.ensureTemplatesLoaded()
	}

	s.writeActionResponseHeaders(w, action, targetSelector, toast, nativeCmd)

	if targetSelector == "none" || targetTpl == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var books []*book.Book
	view := s.activeWorkspaceView()
	if targetTpl == "library_view" || (targetTpl == "app_shell" && view == "library") {
		books, _ = s.loadBooks()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := s.BuildTargetedRenderContext(targetTpl, books)
	_ = tpl.RenderTemplateTo(w, targetTpl, s.Templates, ctx)
}

func (s *Server) writeActionResponseHeaders(w http.ResponseWriter, action, targetSelector, toast, nativeCmd string) {
	s.writeEditorHistoryHeaders(w)
	w.Header().Set("X-Target", targetSelector)
	w.Header().Set("X-Theme", s.Settings.Theme)
	if s.isEffectiveImageless() {
		w.Header().Set("X-Imageless", "true")
	} else {
		w.Header().Set("X-Imageless", "false")
	}
	if s.isBookImageless() {
		w.Header().Set("X-Imageless-Locked", "true")
	} else {
		w.Header().Set("X-Imageless-Locked", "false")
	}
	if s.Settings.UIScale == "large" {
		w.Header().Set("X-UI-Scale", "large")
	} else {
		w.Header().Set("X-UI-Scale", "small")
	}
	if s.Settings.PlaybackSpeed > 0 {
		w.Header().Set("X-Audio-Speed", fmt.Sprintf("%.2g", s.Settings.PlaybackSpeed))
	}
	if s.Settings.AutoChoice {
		w.Header().Set("X-Auto-Choice", "true")
	} else {
		w.Header().Set("X-Auto-Choice", "false")
	}
	if s.Settings.AutoChoiceStrategy != "" {
		w.Header().Set("X-Auto-Strategy", s.Settings.AutoChoiceStrategy)
	}
	if isKeepScrollAction(action) {
		w.Header().Set("X-Keep-Scroll", "true")
	}
	if isResetCanvasAction(action) {
		w.Header().Set("X-Reset-Canvas", "true")
	}
	if s.DevMode {
		w.Header().Set("X-Device-Mode", s.Settings.PreviewMode)
	}
	if toast != "" {
		w.Header().Set("X-Toast", toast)
	}
	if nativeCmd != "" {
		w.Header().Set("X-Native-Cmd", nativeCmd)
	}
}
