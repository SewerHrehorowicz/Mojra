//go:build !(js && wasm)

package gui

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"mojra/internal/book"
	"mojra/internal/tpl"
)

var wfpBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

// AcquireWFPBuffer obtains a pooled bytes.Buffer for zero-allocation WFP request decoding.
func AcquireWFPBuffer() *bytes.Buffer {
	b := wfpBufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

// ReleaseWFPBuffer returns a bytes.Buffer to the WFP pool if within size limits.
func ReleaseWFPBuffer(b *bytes.Buffer) {
	if b != nil && b.Cap() <= 1<<20 {
		b.Reset()
		wfpBufPool.Put(b)
	}
}

func isWFPRequestPayload(contentType string, raw []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "wfp") || strings.HasPrefix(ct, "text/plain") {
		return true
	}
	if bytes.IndexByte(raw, '\n') >= 0 {
		return true
	}
	if bytes.IndexByte(raw, '&') < 0 && bytes.IndexByte(raw, '%') < 0 && bytes.IndexByte(raw, '+') < 0 {
		return true
	}
	return false
}

// ParseActionRequest populates r.Form and r.PostForm from a WFP wire frame (or URL-encoded fallback) using pooled buffers.
func ParseActionRequest(r *http.Request) {
	if r == nil {
		return
	}
	if r.Form == nil {
		r.Form = make(url.Values, 8)
	}
	if r.URL != nil && r.URL.RawQuery != "" {
		if qVals, err := url.ParseQuery(r.URL.RawQuery); err == nil {
			for k, v := range qVals {
				r.Form[k] = v
			}
		}
	}
	if r.Body == nil {
		r.PostForm = r.Form
		return
	}
	buf := AcquireWFPBuffer()
	defer ReleaseWFPBuffer(buf)
	_, _ = buf.ReadFrom(io.LimitReader(r.Body, 4<<20))
	raw := buf.Bytes()
	if len(raw) == 0 {
		r.PostForm = r.Form
		return
	}
	if isWFPRequestPayload(r.Header.Get("Content-Type"), raw) {
		if DecodeWFPFormBytes(raw, r.Form) {
			r.PostForm = r.Form
			return
		}
	}
	if formVals, err := url.ParseQuery(string(raw)); err == nil {
		for k, v := range formVals {
			r.Form[k] = v
		}
	}
	r.PostForm = r.Form
}

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

const (
	targetSelApp           = "#app"
	targetSelWorkspace     = "#workspace"
	targetSelModals        = "#modals-root"
	targetSelEditorContext = "#editor-context-panel"
	targetSelNone          = "none"
)

// ActionSpec centralizes metadata, DOM chunk target rules, and the 1:1 handler for a POST /api/action command.
type ActionSpec struct {
	StandaloneForbidden bool
	KeepScroll          bool
	ResetCanvas         bool
	FlushFormScene      bool
	Target              actionTargetKind
	NativeCmd           string
	Handler             func(*Server, *http.Request) string
}

var actionSpecs = map[string]ActionSpec{
	// Navigation & Global UI
	"SwitchView":       {StandaloneForbidden: true, FlushFormScene: true, Target: targetAppShell, Handler: (*Server).switchView},
	"OpenBook":         {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetAppShell, Handler: (*Server).openBook},
	"ToggleTheme":      {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).toggleTheme},
	"ToggleLanguage":   {KeepScroll: true, FlushFormScene: true, Target: targetAppShell, Handler: (*Server).setLanguage},
	"SetLanguage":      {KeepScroll: true, FlushFormScene: true, Target: targetAppShell, Handler: (*Server).setLanguage},
	"SetUIScale":       {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).toggleUIScale},
	"ToggleUIScale":    {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).toggleUIScale},
	"ToggleZenMode":    {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).toggleZenMode},
	"SetDeviceMode":    {StandaloneForbidden: true, KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).setDeviceMode},
	"ToggleImageless":  {KeepScroll: true, Target: targetImageless, Handler: (*Server).toggleImageless},
	"ToggleFullscreen": {KeepScroll: true, Target: targetNone, NativeCmd: "toggle-fullscreen", Handler: (*Server).toggleFullscreen},
	"StartWindowDrag":  {Target: targetNone, Handler: (*Server).startWindowDrag},
	"QuitApp":          {KeepScroll: true, Target: targetNone, NativeCmd: "quit", Handler: (*Server).quitApp},
	"PromptQuitApp":    {KeepScroll: true, Target: targetNone, NativeCmd: "quit", Handler: (*Server).quitApp},

	// Modals & Confirmations
	"OpenQRModal":               {KeepScroll: true, Target: targetModals, Handler: (*Server).openQRModal},
	"CloseQRModal":              {KeepScroll: true, Target: targetModals, Handler: (*Server).closeQRModal},
	"OpenEditorHelpModal":       {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).openEditorHelpModal},
	"CloseEditorHelpModal":      {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).closeEditorHelpModal},
	"OpenNewBookModal":          {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).openNewBookModal},
	"CloseNewBookModal":         {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).closeNewBookModal},
	"BrowseNewBookFolder":       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).browseNewBookFolder},
	"OpenExportModal":           {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).openExportModal},
	"CloseExportModal":          {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).closeExportModal},
	"ToggleExportFormat":        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).toggleExportFormat},
	"ToggleExportModalAdvanced": {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).toggleExportModalAdvanced},
	"SetExportModalStandard":    {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).setExportModalStandard},
	"BrowseExportModalFolder":   {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).browseExportModalFolder},
	"ExportProject":             {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).exportProjectAction},
	"OpenAddBookModal":          {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).openAddBookModal},
	"CloseAddBookModal":         {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).closeAddBookModal},
	"OpenStudioProjectModal":    {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).openStudioProjectModal},
	"CloseStudioProjectModal":   {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).closeStudioProjectModal},
	"OpenStudioProject":         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).openStudioProjectAction},
	"BrowseOpenStudioProject":   {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).browseOpenStudioProject},
	"OpenStudioProjectFromDisk": {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).openStudioProjectFromDisk},
	"PromptRewindStep":          {KeepScroll: true, Target: targetModals, Handler: (*Server).promptRewindStep},
	"PromptResetProgress":       {KeepScroll: true, Target: targetModals, Handler: (*Server).promptResetProgress},
	"PromptDeleteScene":         {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).promptDeleteScene},
	"PromptDeleteScenes":        {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).promptDeleteScenesFromForm},
	"PromptRemoveChoice":        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).promptRemoveChoiceFromScene},
	"ConfirmModalClose":         {KeepScroll: true, Target: targetModals, Handler: (*Server).closeConfirmModal},
	"ConfirmModalAccept":        {Target: targetConfirmModalAccept, Handler: (*Server).acceptConfirmModalAction},
	"OpenSceneTextModal":        {StandaloneForbidden: true, FlushFormScene: true, Target: targetOpenSceneTextModal, Handler: (*Server).openSceneTextModal},
	"CloseSceneTextModal":       {StandaloneForbidden: true, Target: targetCloseSceneTextModal, Handler: (*Server).closeSceneTextModal},
	"ToggleSceneTextSettings":   {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).toggleSceneTextSettings},
	"NavigateSceneTextModal":    {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).navigateSceneTextModal},
	"SaveSceneTextModal":        {Target: targetModals, Handler: (*Server).saveSceneTextModal},

	// Library & Book Packaging
	"CreateNewBook":         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).createNewBook},
	"BrowseAddBookFromDisk": {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).browseAddBookFromDisk},
	"AddBookFromDisk":       {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).addBookFromDisk},
	"AddBookFromURL":        {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).addBookFromURLAction},
	"RemoveBook":            {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).removeBookFromForm},
	"ExtractEmbeddedBook":   {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).extractEmbeddedBookAction},

	// Player & Audio
	"SaveAudioTime":       {KeepScroll: true, Target: targetNone, Handler: (*Server).saveAudioTime},
	"ChooseBranch":        {Target: targetBranchOrAudioEnded, Handler: (*Server).chooseBranch},
	"AutoAdvanceImplicit": {Target: targetBranchOrAudioEnded, Handler: (*Server).autoAdvanceImplicit},
	"AudioEnded":          {Target: targetBranchOrAudioEnded, Handler: (*Server).audioEnded},
	"RevealHistory":       {Target: targetPlayerView, Handler: (*Server).revealHistory},
	"FocusHistoryStep":    {Target: targetPlayerView, Handler: (*Server).focusHistoryStep},
	"FocusPresent":        {Target: targetPlayerView, Handler: (*Server).focusPresent},
	"CycleAudioSpeed":      {KeepScroll: true, Target: targetNone, Handler: (*Server).cycleAudioSpeed},
	"ToggleAutoChoice":     {KeepScroll: true, Target: targetNone, Handler: (*Server).toggleAutoChoice},
	"SetAutoChoiceDelay":   {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).setAutoChoiceDelay},
	"SetScrollSensitivity": {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).setScrollSensitivity},
	"PlayFromScene":        {StandaloneForbidden: true, FlushFormScene: true, Target: targetAppShell, Handler: (*Server).playFromScene},

	// Node Studio & Editor
	"SelectStudioScene":          {StandaloneForbidden: true, FlushFormScene: true, Target: targetSelectStudioScene, Handler: (*Server).selectStudioScene},
	"SelectStudioChapter":        {StandaloneForbidden: true, FlushFormScene: true, Target: targetSelectStudioScene, Handler: (*Server).selectStudioChapter},
	"CloseBookSettings":          {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).closeBookSettings},
	"ToggleBookSettings":         {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: (*Server).toggleBookSettings},
	"SaveBookSettings":           {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).saveBookSettings},
	"SaveScene":                  {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).saveSceneAction},
	"AutoSaveScene":              {StandaloneForbidden: true, Target: targetNone, Handler: (*Server).autoSaveSceneAction},
	"SaveEditorState":            {StandaloneForbidden: true, Target: targetNone, Handler: (*Server).saveEditorViewportAndNodePositions},
	"RenameBook":                 {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).renameBook},
	"ToggleBookNarrated":         {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).toggleBookNarrated},
	"ToggleBookImageless":        {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).toggleBookImageless},
	"SetStartScene":              {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).setStartScene},
	"SetBookStartChapter":        {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).setBookStartChapter},
	"SetChapterStartScene":       {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).setChapterStartScene},
	"SaveChapterSettings":        {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).saveChapterSettings},
	"BrowseAddBookMedia":         {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).browseAddBookMedia},
	"AddBookMediaFromDisk":       {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).addBookMediaFromDisk},
	"ConnectChoice":              {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).connectChoiceOnScene},
	"ConnectChapter":             {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).connectChapterTransition},
	"AddChoice":                  {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).addChoiceToScene},
	"RemoveChoice":               {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).removeChoiceFromScene},
	"ReorderChoice":              {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).reorderChoiceOnScene},
	"BranchChoiceScene":          {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).branchSceneFromChoice},
	"InsertScene":                {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).insertScene},
	"InsertSceneBefore":          {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).insertSceneBefore},
	"CreateSceneAt":              {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).createSceneAt},
	"DeleteScene":                {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).deleteSceneFromForm},
	"DeleteScenes":               {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).deleteScenesFromForm},
	"AutoLayoutNodes":            {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).autoLayoutNodes},
	"RenameChapter":              {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).renameActiveBookChapter},
	"AssignChapterToScenes":      {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).assignChapterToScenes},
	"CreateChapterGroup":         {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).createChapterGroup},
	"UndoEditor":                 {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).undoEditor},
	"RedoEditor":                 {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).redoEditor},
	"PreviewHistoryRevision":     {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).previewHistoryRevision},
	"ExitHistoryPreview":         {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).exitHistoryPreviewAction},
	"PlayHistoryRevision":        {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).playHistoryRevision},
	"RestoreHistoryRevision":     {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).restoreHistoryRevision},
	"CopyHistoryRevisionToBranch": {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).copyHistoryRevisionToBranchAction},
	"ToggleEditorHistoryPanel":   {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: (*Server).toggleEditorHistoryPanel},
	"OpenEditorHistoryPanel":     {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: (*Server).openEditorHistoryPanel},
	"CloseEditorHistoryPanel":    {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).closeEditorHistoryPanel},
	"SaveHistoryVersion":         {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: (*Server).saveHistoryVersion},
	"PromptCreateHistoryBranch":  {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModals, Handler: (*Server).promptCreateHistoryBranchAction},
	"CreateHistoryBranch":        {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: (*Server).createHistoryBranchAction},
	"SwitchHistoryBranch":        {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetAppShell, Handler: (*Server).switchHistoryBranchAction},
	"ToggleEditorBreakdownPanel": {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: (*Server).toggleEditorBreakdownPanel},
	"CloseEditorBreakdownPanel":  {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).closeEditorBreakdownPanel},
	"RenameCharacter":            {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).renameCharacterAction},
	"RenameLocation":             {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).renameLocationAction},
	"BrowseBookExportFolder":     {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).browseBookExportFolder},
	"BrowseBookSaveFolder":       {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).browseBookSaveFolder},
	"SelectPrimaryBranchScene":   {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).selectPrimaryBranchScene},
	"ApplySluglineToken":         {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).applySluglineTokenAction},
	"InsertScreenplayBlock":      {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).insertScreenplayBlockAction},
	"ToggleEditorVariablesPanel": {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: (*Server).toggleEditorVariablesPanel},
	"CloseEditorVariablesPanel":  {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).closeEditorVariablesPanel},
	"AddBookVariable":            {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).addBookVariableAction},
	"ToggleBookVariableBool":     {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).toggleBookVariableBoolAction},
	"UpdateBookVariable":         {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).updateBookVariableAction},
	"RemoveBookVariable":         {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).removeBookVariableAction},
	"AddBookEnumOption":          {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).addBookEnumOptionAction},
	"RemoveBookEnumOption":       {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).removeBookEnumOptionAction},
	"ToggleChoiceRule":           {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).toggleChoiceRuleAction},
	"ToggleSyntaxHelp":           {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).toggleSyntaxHelpAction},
	"SetSceneTextPreviewMode":    {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModals, Handler: (*Server).setSceneTextPreviewModeAction},
	"SetSceneTextWidthMode":      {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).setSceneTextWidthModeAction},
	"ToggleSceneTestVarsPopover": {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModals, Handler: (*Server).toggleSceneTestVarsPopoverAction},
	"UpdateSceneTextTestVar":     {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModals, Handler: (*Server).updateSceneTextTestVarAction},
	"ToggleSceneVoiceRecorder":   {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).toggleSceneVoiceRecorderAction},
	"SelectSceneVoiceSection":    {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).selectSceneVoiceSectionAction},
	"StartSceneVoiceCapture":     {StandaloneForbidden: true, KeepScroll: true, Target: targetNone, Handler: (*Server).startSceneVoiceCaptureAction},
	"PollSceneVoiceCapture":      {StandaloneForbidden: true, KeepScroll: true, Target: targetNone, Handler: (*Server).pollSceneVoiceCaptureAction},
	"StopSceneVoiceCapture":      {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).stopSceneVoiceCaptureAction},
	"SaveSceneVoiceRecording":    {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).saveSceneVoiceRecordingAction},
	"TrimSceneVoice":             {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).trimSceneVoiceAction},
	"DeleteSceneVoice":           {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).deleteSceneVoiceAction},
}

func (s *Server) flushFormSceneIfPresent(r *http.Request) {
	if r == nil || s.isHistoryPreview() || r.FormValue("scene_id") == "" {
		return
	}
	if msg := s.saveSceneFromForm(r); msg == "" {
		return
	}
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
		return "app_shell", targetSelApp
	}
	switch spec.Target {
	case targetNone:
		return "", targetSelNone
	case targetModals:
		return "modals", targetSelModals
	case targetPlayerView:
		return "player_view", targetSelWorkspace
	case targetActiveWorkspace:
		return s.activeWorkspaceTemplateName(), targetSelWorkspace
	case targetActiveWorkspaceOrModalsWhenSceneTextOpen:
		if s.SceneTextModalID != "" {
			if action == "AddBookVariable" && s.EditorVariablesPanelOpen {
				return "app_shell", targetSelApp
			}
			return "modals", targetSelModals
		}
		return s.activeWorkspaceTemplateName(), targetSelWorkspace
	case targetUndoRedoWorkspaceOrAppShell:
		if s.SceneTextModalID != "" {
			return "app_shell", targetSelApp
		}
		return s.activeWorkspaceTemplateName(), targetSelWorkspace
	case targetThemeOrScale:
		if s.QRModalOpen {
			return "modals", targetSelModals
		}
		return "", targetSelNone
	case targetImageless:
		if s.QRModalOpen {
			return "modals", targetSelModals
		}
		if s.activeWorkspaceView() == "player" {
			return "player_view", targetSelWorkspace
		}
		return "", targetSelNone
	case targetOpenSceneTextModal:
		if hasFormSceneID {
			return "app_shell", targetSelApp
		}
		return "modals", targetSelModals
	case targetCloseSceneTextModal:
		if s.activeWorkspaceView() == "editor" && !s.StandaloneMode {
			return "editor_view", targetSelWorkspace
		}
		return "modals", targetSelModals
	case targetSelectStudioScene:
		if !s.StandaloneMode {
			if s.SceneTextModalID != "" {
				return "app_shell", targetSelApp
			}
			if hasFormSceneID {
				return "editor_view", targetSelWorkspace
			}
			return "editor_context_panel", targetSelEditorContext
		}
		return "player_view", targetSelWorkspace
	case targetBranchOrAudioEnded:
		if s.ConfirmPendingAction != "" {
			return "modals", targetSelModals
		}
		return "player_view", targetSelWorkspace
	case targetConfirmModalAccept:
		if prevConfirmAction == "RemoveChoice" && s.SceneTextModalID != "" {
			return "modals", targetSelModals
		}
		return "app_shell", targetSelApp
	default:
		return "app_shell", targetSelApp
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
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func isRemoteLANAllowedAction(action, pendingConfirm string) bool {
	switch action {
	case "ChooseBranch",
		"AutoAdvanceImplicit",
		"AudioEnded",
		"SaveAudioTime",
		"RevealHistory",
		"FocusHistoryStep",
		"FocusPresent",
		"CycleAudioSpeed",
		"ToggleAutoChoice",
		"SetAutoChoiceDelay",
		"SetScrollSensitivity",
		"ToggleTheme",
		"ToggleLanguage",
		"SetLanguage",
		"SetUIScale",
		"ToggleUIScale",
		"ToggleZenMode",
		"ToggleImageless",
		"PromptRewindStep",
		"PromptResetProgress",
		"ConfirmModalClose":
		return true
	case "ConfirmModalAccept":
		return pendingConfirm == "RewindStep" || pendingConfirm == "ResetProgress"
	default:
		return false
	}
}

func (s *Server) switchView(r *http.Request) string {
	view := strings.TrimSpace(r.FormValue("view"))
	prevView := ""
	if s.Settings != nil {
		prevView = s.Settings.ActiveView
	}
	wasActiveStudioEditor := prevView == "editor" && !s.StudioHome && s.ActiveBook != nil
	if (view == "library" || view == "studio") && s.isHistoryPreview() {
		s.exitHistoryPreview()
	}
	if view == "studio" {
		if wasActiveStudioEditor {
			s.saveCurrentBookEditorUIState()
		}
		s.StudioHome = true
		s.ZenMode = false
		s.Settings.ActiveView = "editor"
		s.closeAllModals()
		s.syncAndPersistEditorUIState()
		return ""
	}
	if view == "editor" {
		s.StudioHome = false
	}
	if view == "player" && s.PlayerBook == nil {
		if s.ActiveBook != nil {
			s.setPlayerBook(s.ActiveBook)
		} else if books, err := s.loadBooks(); err == nil && len(books) > 0 {
			s.setPlayerBook(books[0])
		}
	}
	if view == "library" || view == "player" || view == "editor" {
		if view != "editor" && wasActiveStudioEditor {
			s.saveCurrentBookEditorUIState()
		}
		if view != "player" {
			s.ZenMode = false
		}
		s.Settings.ActiveView = view
		s.closeAllModals()
		if view == "editor" && !wasActiveStudioEditor {
			s.restoreBookEditorUIState(true)
		}
		s.syncAndPersistEditorUIState()
	}
	return ""
}

func (s *Server) toggleTheme(_ *http.Request) string {
	if s.Settings.Theme == "light" {
		s.Settings.Theme = "dark"
	} else {
		s.Settings.Theme = "light"
	}
	s.persistSettings()
	return ""
}

func (s *Server) setLanguage(r *http.Request) string {
	reqLang := strings.TrimSpace(r.FormValue("lang"))
	if reqLang != "" {
		s.Settings.Language = tpl.NormalizeLangCode(reqLang)
	} else {
		next := tpl.NextLang(s.lang())
		s.Settings.Language = tpl.SupportedLanguages[next].Code
	}
	s.persistSettings()
	return ""
}

func (s *Server) toggleImageless(_ *http.Request) string {
	if !s.isBookImageless() {
		s.Settings.Imageless = !s.Settings.Imageless
		s.persistSettings()
	}
	return ""
}

func (s *Server) toggleUIScale(r *http.Request) string {
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
	s.persistSettings()
	return ""
}

func (s *Server) isEffectiveZenMode() bool {
	return s.ZenMode && s.activeWorkspaceView() == "player" && s.PlayerBook != nil && !s.PlayerBook.IsScreenplay()
}

func (s *Server) toggleZenMode(r *http.Request) string {
	if s.activeWorkspaceView() != "player" || s.PlayerBook == nil || s.PlayerBook.IsScreenplay() {
		s.ZenMode = false
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(r.FormValue("zen"))) {
	case "true", "1", "on":
		s.ZenMode = true
	case "false", "0", "off":
		s.ZenMode = false
	default:
		s.ZenMode = !s.ZenMode
	}
	return ""
}

func (s *Server) setDeviceMode(r *http.Request) string {
	if s.DevMode {
		mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
		switch mode {
		case "phone":
			mode = "portrait"
		case "ipad":
			mode = "tablet"
		case "desktop":
			mode = "full"
		}
		if mode == "full" || mode == "portrait" || mode == "tablet" || mode == "landscape" {
			s.Settings.PreviewMode = mode
			s.persistSettings()
		}
	}
	return ""
}

func (s *Server) toggleFullscreen(_ *http.Request) string {
	ToggleNativeWindowFullscreen()
	return ""
}

func (s *Server) startWindowDrag(_ *http.Request) string {
	StartNativeWindowDrag()
	return ""
}

func (s *Server) openQRModal(_ *http.Request) string {
	s.QRModalOpen = true
	return ""
}

func (s *Server) closeQRModal(_ *http.Request) string {
	s.QRModalOpen = false
	return ""
}

func (s *Server) openEditorHelpModal(_ *http.Request) string {
	s.EditorHelpModalOpen = !s.EditorHelpModalOpen
	if s.EditorHelpModalOpen {
		s.activateStudioRightPanel("help")
	}
	return ""
}

func (s *Server) closeEditorHelpModal(_ *http.Request) string {
	s.EditorHelpModalOpen = false
	return ""
}

func (s *Server) openExportModal(_ *http.Request) string {
	if s.ActiveBook == nil {
		return s.tr(TrNoActiveProject)
	}
	s.QRModalOpen = false
	s.NewBookModalOpen = false
	s.AddBookModalOpen = false
	s.ExportModalOpen = true
	s.ExportModalAdvancedOpen = false
	s.ExportModalStandard = s.ActiveBook.NormalizedStandard()
	s.ExportModalFormats = map[string]bool{
		"playbook": true,
		"wasm":     true,
		"pdf":      true,
	}
	if s.ActiveBook.IsScreenplay() {
		s.ExportModalFormat = "pdf"
	} else {
		s.ExportModalFormat = "playbook"
	}
	s.ExportModalDir = s.defaultExportDir()
	return ""
}

func (s *Server) closeExportModal(_ *http.Request) string {
	s.resetExportModalState()
	return ""
}

func (s *Server) toggleExportFormat(r *http.Request) string {
	if dir := strings.TrimSpace(r.FormValue("export_dir")); dir != "" {
		s.ExportModalDir = dir
	}
	if std := strings.TrimSpace(r.FormValue("standard")); std != "" {
		s.ExportModalStandard = book.NormalizeScriptStandard(std)
	}
	fmtID := strings.ToLower(strings.TrimSpace(r.FormValue("format")))
	if fmtID == "stbook" {
		fmtID = "playbook"
	}
	switch fmtID {
	case "playbook", "wasm", "pdf", "fountain", "fdx", "treatment", "book_pdf", "vo_csv", "actor_pdf", "playtxt":
		if s.ExportModalFormats == nil {
			s.ExportModalFormats = make(map[string]bool)
		}
		if s.ExportModalFormats[fmtID] {
			delete(s.ExportModalFormats, fmtID)
		} else {
			s.ExportModalFormats[fmtID] = true
		}
	}
	return ""
}

func (s *Server) toggleExportModalAdvanced(r *http.Request) string {
	if dir := strings.TrimSpace(r.FormValue("export_dir")); dir != "" {
		s.ExportModalDir = dir
	}
	if std := strings.TrimSpace(r.FormValue("standard")); std != "" {
		s.ExportModalStandard = book.NormalizeScriptStandard(std)
	}
	s.ExportModalAdvancedOpen = !s.ExportModalAdvancedOpen
	return ""
}

func (s *Server) setExportModalStandard(r *http.Request) string {
	if dir := strings.TrimSpace(r.FormValue("export_dir")); dir != "" {
		s.ExportModalDir = dir
	}
	if std := strings.TrimSpace(r.FormValue("standard")); std != "" {
		s.ExportModalStandard = book.NormalizeScriptStandard(std)
	}
	return ""
}

func (s *Server) openNewBookModal(r *http.Request) string {
	s.QRModalOpen = false
	s.AddBookModalOpen = false
	s.ExportModalOpen = false
	s.resetNewBookModalState()
	s.NewBookModalOpen = true
	if r != nil && strings.EqualFold(strings.TrimSpace(r.FormValue("mode")), "screenplay") {
		s.NewBookDraftMode = "screenplay"
	} else {
		s.NewBookDraftMode = "book"
	}
	s.NewBookDraftStandard = "pl"
	s.NewBookDraftDir = s.defaultNewBookDir()
	return ""
}

func (s *Server) closeNewBookModal(_ *http.Request) string {
	s.resetNewBookModalState()
	return ""
}

func (s *Server) openAddBookModal(_ *http.Request) string {
	s.QRModalOpen = false
	s.NewBookModalOpen = false
	s.StudioProjectModalOpen = false
	s.AddBookModalOpen = true
	return ""
}

func (s *Server) closeAddBookModal(_ *http.Request) string {
	s.AddBookModalOpen = false
	return ""
}

func (s *Server) openStudioProjectModal(_ *http.Request) string {
	s.QRModalOpen = false
	s.NewBookModalOpen = false
	s.AddBookModalOpen = false
	s.ExportModalOpen = false
	s.StudioProjectModalOpen = true
	return ""
}

func (s *Server) closeStudioProjectModal(_ *http.Request) string {
	s.StudioProjectModalOpen = false
	return ""
}

func (s *Server) quitApp(_ *http.Request) string {
	s.ConfirmPendingAction = ""
	s.ConfirmPendingParam = ""
	go func() {
		s.TriggerShutdown()
		TerminateNativeWindow()
	}()
	return ""
}

func (s *Server) closeConfirmModal(_ *http.Request) string {
	s.ConfirmPendingAction = ""
	s.ConfirmPendingParam = ""
	return ""
}

func (s *Server) acceptConfirmModalAction(_ *http.Request) string {
	toast, _ := s.acceptConfirmModal()
	return toast
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
			s.persistSaveState()
			toast = s.tr(TrRewoundStoryToast)
		}
	case "ResetProgress":
		targetID := param
		if targetID == "" && s.PlayerBook != nil {
			targetID = s.PlayerBook.ID
		}
		if targetID != "" {
			if s.isHistoryPreview() && s.PlayerBook != nil && s.PlayerBook.ID == targetID {
				startSceneID := s.PlayerBook.StartSceneID
				if s.findPlayerScene(startSceneID) == nil && len(s.PlayerBook.Scenes) > 0 {
					startSceneID = s.PlayerBook.Scenes[0].ID
				}
				s.Save = &book.SaveState{
					BookID:             s.PlayerBook.ID,
					CurrentSceneID:     startSceneID,
					PlaybackSpeed:      s.Settings.PlaybackSpeed,
					AutoChoice:         s.Settings.AutoChoice,
					AutoChoiceStrategy: s.Settings.AutoChoiceStrategy,
					AutoChoiceDelay:    s.Settings.AutoChoiceDelay,
				}
				s.HistoryRevealed = false
				s.FocusedHistoryIdx = -1
			} else {
				_ = book.DeleteSaveState(s.SavesDir, targetID)
				if s.PlayerBook != nil && s.PlayerBook.ID == targetID {
					s.Save = book.LoadSaveState(s.SavesDir, s.PlayerBook.ID, s.PlayerBook.StartSceneID)
					s.HistoryRevealed = false
					s.FocusedHistoryIdx = -1
				}
			}
			toast = s.tr(TrResetProgressToast)
		}
	case "UnpackPlaybookForStudio":
		toast = s.unpackPlaybookWorkingCopyForStudio(param)
	case "RemoveBook":
		toast = s.removeBookByID(param)
	case "DeleteScene":
		toast = s.deleteSceneByID(param)
	case "DeleteScenes":
		toast = s.deleteScenesByIDs(param)
	case "CreateHistoryBranch":
		toast = s.createHistoryBranchByName(param, "")
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
	ParseActionRequest(r)
	action := strings.TrimSpace(r.FormValue("action"))

	s.mu.Lock()
	defer s.mu.Unlock()

	if !isLoopbackRequest(r) && !isRemoteLANAllowedAction(action, s.ConfirmPendingAction) {
		http.Error(w, "forbidden remote LAN action", http.StatusForbidden)
		return
	}

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
	hadPlayerBook := s.PlayerBook != nil
	hadEditorBook := s.ActiveBook != nil

	// If the client sends its current audio_time along with a navigation/switch action, persist it for the active player book before switching.
	if action != "SaveAudioTime" && s.PlayerBook != nil && s.Save != nil {
		if rawAudioTime := strings.TrimSpace(r.FormValue("audio_time")); rawAudioTime != "" {
			if t, err := strconv.ParseFloat(rawAudioTime, 64); err == nil && t >= 0 {
				if t > 0.2 || s.Save.UpdatedAtUnix > 0 || len(s.Save.History) > 0 {
					s.Save.AudioTime = t
					s.persistAudioTimeInPlace(t)
				}
			}
		}
	}

	if spec, ok := actionSpecs[action]; ok && spec.Handler != nil {
		if spec.FlushFormScene {
			s.flushFormSceneIfPresent(r)
		}
		toast = spec.Handler(s, r)
		nativeCmd = spec.NativeCmd
		if action == "ConfirmModalAccept" && prevConfirmAction == "QuitApp" {
			nativeCmd = "quit"
		}
	}

	if action == "SaveAudioTime" || action == "StartWindowDrag" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.syncAndPersistEditorUIState()

	if r.FormValue("silent") == "true" {
		s.writeEditorHistoryHeaders(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	targetTpl, targetSelector := s.resolveActionChunkTarget(action, prevConfirmAction, strings.TrimSpace(r.FormValue("scene_id")) != "")
	if ((!hadPlayerBook && s.PlayerBook != nil) || (!hadEditorBook && s.ActiveBook != nil)) && targetSelector == targetSelWorkspace {
		targetTpl = "app_shell"
		targetSelector = targetSelApp
	}

	if targetSelector != targetSelNone {
		s.ensureTemplatesLoaded()
	}

	s.writeActionResponseHeaders(w, action, targetSelector, toast, nativeCmd)

	if targetSelector == targetSelNone || targetTpl == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var books []*book.Book
	view := s.activeWorkspaceView()
	if targetTpl == "library_view" || (targetTpl == "app_shell" && view == "library") || ((targetTpl == "editor_view" || (targetTpl == "app_shell" && view == "editor")) && !s.hasActiveStudioEditor()) {
		books, _ = s.loadBooks()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx := s.BuildTargetedRenderContext(targetTpl, books)
	if err := tpl.RenderTemplateTo(w, targetTpl, s.Templates, ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) writeActionResponseHeaders(w http.ResponseWriter, action, targetSelector, toast, nativeCmd string) {
	s.writeEditorHistoryHeaders(w)
	w.Header().Set("X-Target", targetSelector)
	w.Header().Set("X-Theme", s.Settings.Theme)
	w.Header().Set("X-Lang", book.NormalizeLanguage(s.Settings.Language))
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
	if s.isEffectiveZenMode() {
		w.Header().Set("X-Zen-Mode", "true")
	} else {
		w.Header().Set("X-Zen-Mode", "false")
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
	w.Header().Set("X-Auto-Delay", fmt.Sprintf("%.2g", book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay)))
	w.Header().Set("X-Scroll-Sensitivity", book.NormalizeScrollSensitivity(s.Settings.ScrollSensitivity))
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
	if action == "PollSceneVoiceCapture" {
		peak, active := PollNativeAudioCapturePeak()
		if active {
			w.Header().Set("X-Voice-Peak", fmt.Sprintf("%.3f", peak))
		} else {
			w.Header().Set("X-Voice-Peak", "stopped")
		}
	}
}
