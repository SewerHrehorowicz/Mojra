//go:build !(js && wasm)

package actions

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/logging"
	"mojra/internal/osbridge"
	"mojra/internal/state"
	"mojra/internal/tpl"
	"mojra/internal/views"
)

// Router dispatches POST /api/action commands against the explicit AppState.
type Router struct {
	State *state.AppState
}

func langOf(s *state.AppState) i18n.Lang {
	if s == nil || s.Settings == nil {
		return i18n.LangPL
	}
	return i18n.ParseLang(s.Settings.Language)
}

func tr(s *state.AppState, k i18n.TrKey) string {
	return i18n.T(langOf(s), k)
}

func trf(s *state.AppState, k i18n.TrKey, args ...any) string {
	return i18n.Tf(langOf(s), k, args...)
}

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
	_, _ = buf.ReadFrom(io.LimitReader(r.Body, 64<<20))
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
	targetModalQR
	targetModalNewBook
	targetModalExport
	targetModalAddBook
	targetModalStudioProjects
	targetModalConfirm
	targetModalSceneText
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

	modalIDQR             = "modal-backdrop-qr"
	modalIDNewBook        = "modal-backdrop-new-book"
	modalIDExport         = "modal-backdrop-export"
	modalIDAddBook        = "modal-backdrop-add-book"
	modalIDStudioProjects = "modal-backdrop-studio-projects"
	modalIDSceneText      = "modal-scene-text"
	modalIDConfirm        = "modal-confirm"
)

// ActionSpec centralizes metadata, DOM chunk target rules, and the 1:1 handler for a POST /api/action command.
type ActionSpec struct {
	StandaloneForbidden bool
	KeepScroll          bool
	ResetCanvas         bool
	FlushFormScene      bool
	Target              actionTargetKind
	NativeCmd           string
	Handler             func(*state.AppState, *http.Request) string
}

var ActionSpecs = map[string]ActionSpec{
	// Navigation & Global UI
	SwitchView:       {StandaloneForbidden: true, FlushFormScene: true, Target: targetAppShell, Handler: switchView},
	OpenBook:         {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetAppShell, Handler: openBook},
	ToggleTheme:      {KeepScroll: true, Target: targetThemeOrScale, Handler: toggleTheme},
	ToggleLanguage:   {KeepScroll: true, FlushFormScene: true, Target: targetAppShell, Handler: setLanguage},
	SetLanguage:      {KeepScroll: true, FlushFormScene: true, Target: targetAppShell, Handler: setLanguage},
	SetUIScale:       {KeepScroll: true, Target: targetThemeOrScale, Handler: toggleUIScale},
	ToggleUIScale:    {KeepScroll: true, Target: targetThemeOrScale, Handler: toggleUIScale},
	ToggleZenMode:    {KeepScroll: true, Target: targetThemeOrScale, Handler: toggleZenMode},
	SetDeviceMode:    {StandaloneForbidden: true, KeepScroll: true, Target: targetThemeOrScale, Handler: setDeviceMode},
	ToggleImageless:  {KeepScroll: true, Target: targetImageless, Handler: toggleImageless},
	ToggleFullscreen: {KeepScroll: true, Target: targetNone, NativeCmd: "toggle-fullscreen", Handler: toggleFullscreen},
	StartWindowDrag:  {Target: targetNone, Handler: startWindowDrag},
	QuitApp:          {KeepScroll: true, Target: targetNone, NativeCmd: "quit", Handler: quitApp},
	PromptQuitApp:    {KeepScroll: true, Target: targetNone, NativeCmd: "quit", Handler: quitApp},

	// Modals & Confirmations
	OpenQRModal:               {KeepScroll: true, Target: targetModalQR, Handler: openQRModal},
	CloseQRModal:              {KeepScroll: true, Target: targetModalQR, Handler: closeQRModal},
	OpenEditorHelpModal:       {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: openEditorHelpModal},
	CloseEditorHelpModal:      {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: closeEditorHelpModal},
	OpenNewBookModal:          {StandaloneForbidden: true, KeepScroll: true, Target: targetModalNewBook, Handler: openNewBookModal},
	CloseNewBookModal:         {StandaloneForbidden: true, KeepScroll: true, Target: targetModalNewBook, Handler: closeNewBookModal},
	BrowseNewBookFolder:       {StandaloneForbidden: true, KeepScroll: true, Target: targetModalNewBook, Handler: browseNewBookFolder},
	OpenExportModal:           {StandaloneForbidden: true, KeepScroll: true, Target: targetModalExport, Handler: openExportModal},
	CloseExportModal:          {StandaloneForbidden: true, KeepScroll: true, Target: targetModalExport, Handler: closeExportModal},
	ToggleExportFormat:        {StandaloneForbidden: true, KeepScroll: true, Target: targetModalExport, Handler: toggleExportFormat},
	ToggleExportModalAdvanced: {StandaloneForbidden: true, KeepScroll: true, Target: targetModalExport, Handler: toggleExportModalAdvanced},
	SetExportModalStandard:    {StandaloneForbidden: true, KeepScroll: true, Target: targetModalExport, Handler: setExportModalStandard},
	BrowseExportModalFolder:   {StandaloneForbidden: true, KeepScroll: true, Target: targetModalExport, Handler: browseExportModalFolder},
	ExportProject:             {StandaloneForbidden: true, KeepScroll: true, Target: targetModalExport, Handler: exportProjectAction},
	OpenAddBookModal:          {StandaloneForbidden: true, KeepScroll: true, Target: targetModalAddBook, Handler: openAddBookModal},
	CloseAddBookModal:         {StandaloneForbidden: true, KeepScroll: true, Target: targetModalAddBook, Handler: closeAddBookModal},
	OpenStudioProjectModal:    {StandaloneForbidden: true, KeepScroll: true, Target: targetModalStudioProjects, Handler: openStudioProjectModal},
	CloseStudioProjectModal:   {StandaloneForbidden: true, KeepScroll: true, Target: targetModalStudioProjects, Handler: closeStudioProjectModal},
	OpenStudioProject:         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: openStudioProjectAction},
	BrowseOpenStudioProject:   {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: browseOpenStudioProject},
	OpenStudioProjectFromDisk: {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: openStudioProjectFromDisk},
	PromptRewindStep:          {KeepScroll: true, Target: targetModalConfirm, Handler: promptRewindStep},
	PromptResetProgress:       {KeepScroll: true, Target: targetModalConfirm, Handler: promptResetProgress},
	PromptDeleteScene:         {StandaloneForbidden: true, Target: targetModalConfirm, Handler: promptDeleteScene},
	PromptDeleteScenes:        {StandaloneForbidden: true, Target: targetModalConfirm, Handler: promptDeleteScenesFromForm},
	PromptRemoveChoice:        {StandaloneForbidden: true, KeepScroll: true, Target: targetModalConfirm, Handler: promptRemoveChoiceFromScene},
	ConfirmModalClose:         {KeepScroll: true, Target: targetModalConfirm, Handler: closeConfirmModal},
	ConfirmModalAccept:        {Target: targetConfirmModalAccept, Handler: acceptConfirmModalAction},
	OpenSceneTextModal:        {StandaloneForbidden: true, FlushFormScene: true, Target: targetOpenSceneTextModal, Handler: openSceneTextModal},
	CloseSceneTextModal:       {StandaloneForbidden: true, Target: targetCloseSceneTextModal, Handler: closeSceneTextModal},
	ToggleSceneTextSettings:   {StandaloneForbidden: true, Target: targetModalSceneText, Handler: toggleSceneTextSettings},
	NavigateSceneTextModal:    {StandaloneForbidden: true, Target: targetModalSceneText, Handler: navigateSceneTextModal},
	SaveSceneTextModal:        {Target: targetModalSceneText, Handler: saveSceneTextModal},

	// Library & Book Packaging
	CreateNewBook:         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: createNewBook},
	BrowseAddBookFromDisk: {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: browseAddBookFromDisk},
	AddBookFromDisk:       {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: addBookFromDisk},
	AddBookFromURL:        {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: addBookFromURLAction},
	RemoveBook:            {StandaloneForbidden: true, Target: targetAppShell, Handler: removeBookFromForm},
	ExtractEmbeddedBook:   {StandaloneForbidden: true, Target: targetAppShell, Handler: extractEmbeddedBookAction},

	// Player & Audio
	SaveAudioTime:        {KeepScroll: true, Target: targetNone, Handler: saveAudioTime},
	ChooseBranch:         {Target: targetBranchOrAudioEnded, Handler: chooseBranch},
	AutoAdvanceImplicit:  {Target: targetBranchOrAudioEnded, Handler: autoAdvanceImplicit},
	AudioEnded:           {Target: targetBranchOrAudioEnded, Handler: audioEnded},
	RevealHistory:        {Target: targetPlayerView, Handler: revealHistory},
	FocusHistoryStep:     {Target: targetPlayerView, Handler: focusHistoryStep},
	FocusPresent:         {Target: targetPlayerView, Handler: focusPresent},
	CycleAudioSpeed:      {KeepScroll: true, Target: targetNone, Handler: cycleAudioSpeed},
	ToggleAutoChoice:     {KeepScroll: true, Target: targetNone, Handler: toggleAutoChoice},
	SetAutoChoiceDelay:   {KeepScroll: true, Target: targetThemeOrScale, Handler: setAutoChoiceDelay},
	SetScrollSensitivity: {KeepScroll: true, Target: targetThemeOrScale, Handler: setScrollSensitivity},
	PlayFromScene:        {StandaloneForbidden: true, FlushFormScene: true, Target: targetAppShell, Handler: playFromScene},

	// Node Studio & Editor
	SelectStudioScene:           {StandaloneForbidden: true, FlushFormScene: true, Target: targetSelectStudioScene, Handler: selectStudioScene},
	SelectStudioChapter:         {StandaloneForbidden: true, FlushFormScene: true, Target: targetSelectStudioScene, Handler: selectStudioChapter},
	CloseBookSettings:           {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: closeBookSettings},
	ToggleBookSettings:          {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: toggleBookSettings},
	SaveBookSettings:            {StandaloneForbidden: true, Target: targetAppShell, Handler: saveBookSettings},
	SaveScene:                   {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: saveSceneAction},
	AutoSaveScene:               {StandaloneForbidden: true, Target: targetNone, Handler: autoSaveSceneAction},
	SaveEditorState:             {StandaloneForbidden: true, Target: targetNone, Handler: saveEditorViewportAndNodePositions},
	RenameBook:                  {StandaloneForbidden: true, Target: targetAppShell, Handler: renameBook},
	ToggleBookNarrated:          {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: toggleBookNarrated},
	ToggleBookImageless:         {StandaloneForbidden: true, Target: targetAppShell, Handler: toggleBookImageless},
	SetStartScene:               {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: setStartScene},
	SetBookStartChapter:         {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: setBookStartChapter},
	SetChapterStartScene:        {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: setChapterStartScene},
	SaveChapterSettings:         {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: saveChapterSettings},
	BrowseAddBookMedia:          {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: browseAddBookMedia},
	AddBookMediaFromDisk:        {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: addBookMediaFromDisk},
	ConnectChoice:               {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: connectChoiceOnScene},
	ConnectChapter:              {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: connectChapterTransition},
	AddChoice:                   {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: addChoiceToScene},
	RemoveChoice:                {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: removeChoiceFromScene},
	ReorderChoice:               {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: reorderChoiceOnScene},
	BranchChoiceScene:           {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: branchSceneFromChoice},
	InsertScene:                 {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: insertScene},
	InsertSceneBefore:           {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: insertSceneBefore},
	CreateSceneAt:               {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: createSceneAt},
	DeleteScene:                 {StandaloneForbidden: true, Target: targetAppShell, Handler: deleteSceneFromForm},
	DeleteScenes:                {StandaloneForbidden: true, Target: targetAppShell, Handler: deleteScenesFromForm},
	AutoLayoutNodes:             {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: autoLayoutNodes},
	RenameChapter:               {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: renameActiveBookChapter},
	AssignChapterToScenes:       {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: assignChapterToScenes},
	CreateChapterGroup:          {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: createChapterGroup},
	UndoEditor:                  {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: undoEditor},
	RedoEditor:                  {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: redoEditor},
	PreviewHistoryRevision:      {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: previewHistoryRevision},
	ExitHistoryPreview:          {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: exitHistoryPreviewAction},
	PlayHistoryRevision:         {StandaloneForbidden: true, Target: targetAppShell, Handler: playHistoryRevision},
	RestoreHistoryRevision:      {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: restoreHistoryRevision},
	CopyHistoryRevisionToBranch: {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: copyHistoryRevisionToBranchAction},
	ToggleEditorHistoryPanel:    {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: toggleEditorHistoryPanel},
	OpenEditorHistoryPanel:      {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: openEditorHistoryPanel},
	CloseEditorHistoryPanel:     {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: closeEditorHistoryPanel},
	SaveHistoryVersion:          {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: saveHistoryVersion},
	PromptCreateHistoryBranch:   {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModalConfirm, Handler: promptCreateHistoryBranchAction},
	CreateHistoryBranch:         {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: createHistoryBranchAction},
	SwitchHistoryBranch:         {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetAppShell, Handler: switchHistoryBranchAction},
	ToggleEditorBreakdownPanel:  {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: toggleEditorBreakdownPanel},
	CloseEditorBreakdownPanel:   {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: closeEditorBreakdownPanel},
	RenameCharacter:             {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: renameCharacterAction},
	RenameLocation:              {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: renameLocationAction},
	BrowseBookExportFolder:      {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: browseBookExportFolder},
	BrowseBookSaveFolder:        {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: browseBookSaveFolder},
	SelectPrimaryBranchScene:    {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: selectPrimaryBranchScene},
	ApplySluglineToken:          {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: applySluglineTokenAction},
	InsertScreenplayBlock:       {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: insertScreenplayBlockAction},
	ToggleEditorVariablesPanel:  {StandaloneForbidden: true, FlushFormScene: true, Target: targetActiveWorkspace, Handler: toggleEditorVariablesPanel},
	CloseEditorVariablesPanel:   {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: closeEditorVariablesPanel},
	AddBookVariable:             {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: addBookVariableAction},
	ToggleBookVariableBool:      {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: toggleBookVariableBoolAction},
	UpdateBookVariable:          {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: updateBookVariableAction},
	RemoveBookVariable:          {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: removeBookVariableAction},
	AddBookEnumOption:           {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: addBookEnumOptionAction},
	RemoveBookEnumOption:        {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: removeBookEnumOptionAction},
	ToggleChoiceRule:            {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: toggleChoiceRuleAction},
	ToggleSyntaxHelp:            {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: toggleSyntaxHelpAction},
	SetSceneTextPreviewMode:     {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModalSceneText, Handler: setSceneTextPreviewModeAction},
	SetSceneTextWidthMode:       {StandaloneForbidden: true, KeepScroll: true, Target: targetModalSceneText, Handler: setSceneTextWidthModeAction},
	ToggleSceneTestVarsPopover:  {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModalSceneText, Handler: toggleSceneTestVarsPopoverAction},
	UpdateSceneTextTestVar:      {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModalSceneText, Handler: updateSceneTextTestVarAction},
	ToggleSceneVoiceRecorder:    {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: toggleSceneVoiceRecorderAction},
	SelectSceneVoiceSection:     {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: selectSceneVoiceSectionAction},
	StartSceneVoiceCapture:      {StandaloneForbidden: true, KeepScroll: true, Target: targetNone, Handler: startSceneVoiceCaptureAction},
	PollSceneVoiceCapture:       {StandaloneForbidden: true, KeepScroll: true, Target: targetNone, Handler: pollSceneVoiceCaptureAction},
	StopSceneVoiceCapture:       {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: stopSceneVoiceCaptureAction},
	SaveSceneVoiceRecording:     {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: saveSceneVoiceRecordingAction},
	TrimSceneVoice:              {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: trimSceneVoiceAction},
	DeleteSceneVoice:            {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: deleteSceneVoiceAction},
}

func flushFormSceneIfPresent(s *state.AppState, r *http.Request) {
	if r == nil || isHistoryPreview(s) || r.FormValue("scene_id") == "" {
		return
	}
	if msg := saveSceneFromForm(s, r); msg == "" {
		return
	}
}

func isStandaloneForbiddenAction(act string) bool {
	return ActionSpecs[act].StandaloneForbidden
}

func isKeepScrollAction(act string) bool {
	return ActionSpecs[act].KeepScroll
}

func isResetCanvasAction(act string) bool {
	return ActionSpecs[act].ResetCanvas
}

func resolveActionView(s *state.AppState, act, prevConfirmAction string, hasFormSceneID bool) (tpl.TemplateChunk, string, string) {
	spec, ok := ActionSpecs[act]
	if !ok {
		return tpl.TemplateChunk{Template: "app_shell", Data: views.ViewAppShell(s)}, targetSelApp, ""
	}
	switch spec.Target {
	case targetNone:
		return tpl.TemplateChunk{}, targetSelNone, ""
	case targetModalQR:
		return tpl.TemplateChunk{Template: "modal_qr", Data: views.ViewModalQR(s)}, targetSelModals, modalIDQR
	case targetModalNewBook:
		return tpl.TemplateChunk{Template: "modal_new_book", Data: views.ViewModalNewBook(s)}, targetSelModals, modalIDNewBook
	case targetModalExport:
		return tpl.TemplateChunk{Template: "modal_export", Data: views.ViewModalExport(s)}, targetSelModals, modalIDExport
	case targetModalAddBook:
		return tpl.TemplateChunk{Template: "modal_add_book", Data: views.ViewModalAddBook(s)}, targetSelModals, modalIDAddBook
	case targetModalStudioProjects:
		return tpl.TemplateChunk{Template: "modal_studio_projects", Data: views.ViewModalStudioProjects(s)}, targetSelModals, modalIDStudioProjects
	case targetModalConfirm:
		return tpl.TemplateChunk{Template: "confirm_modal", Data: views.ViewModalConfirm(s)}, targetSelModals, modalIDConfirm
	case targetModalSceneText:
		return tpl.TemplateChunk{Template: "modal_scene_text", Data: views.ViewEditorSceneText(s)}, targetSelModals, modalIDSceneText
	case targetPlayerView:
		return tpl.TemplateChunk{Template: "player_view", Data: views.ViewPlayer(s)}, targetSelWorkspace, ""
	case targetActiveWorkspace:
		return views.ViewActiveWorkspaceChunk(s), targetSelWorkspace, ""
	case targetActiveWorkspaceOrModalsWhenSceneTextOpen:
		if s.Editor.SceneText.IsOpen() {
			if act == AddBookVariable && s.Editor.RightPanel == state.PanelVariables {
				return tpl.TemplateChunk{Template: "app_shell", Data: views.ViewAppShell(s)}, targetSelApp, ""
			}
			return tpl.TemplateChunk{Template: "modal_scene_text", Data: views.ViewEditorSceneText(s)}, targetSelModals, modalIDSceneText
		}
		if (act == ToggleSceneVoiceRecorder || act == SelectSceneVoiceSection || act == TrimSceneVoice) &&
			!hasFormSceneID && state.ActiveWorkspaceView(s) == state.ViewEditor && !s.Config.StandaloneMode && !s.Editor.StudioHome {
			return tpl.TemplateChunk{Template: "editor_context_panel", Data: views.ViewEditorContextPanel(s)}, targetSelEditorContext, ""
		}
		return views.ViewActiveWorkspaceChunk(s), targetSelWorkspace, ""
	case targetUndoRedoWorkspaceOrAppShell:
		if s.Editor.SceneText.IsOpen() {
			return tpl.TemplateChunk{Template: "app_shell", Data: views.ViewAppShell(s)}, targetSelApp, ""
		}
		return views.ViewActiveWorkspaceChunk(s), targetSelWorkspace, ""
	case targetThemeOrScale:
		if s.Modals.QR {
			return tpl.TemplateChunk{Template: "modal_qr", Data: views.ViewModalQR(s)}, targetSelModals, modalIDQR
		}
		return tpl.TemplateChunk{}, targetSelNone, ""
	case targetImageless:
		if s.Modals.QR {
			return tpl.TemplateChunk{Template: "modal_qr", Data: views.ViewModalQR(s)}, targetSelModals, modalIDQR
		}
		if state.ActiveWorkspaceView(s) == state.ViewPlayer {
			return tpl.TemplateChunk{Template: "player_view", Data: views.ViewPlayer(s)}, targetSelWorkspace, ""
		}
		return tpl.TemplateChunk{}, targetSelNone, ""
	case targetOpenSceneTextModal:
		if hasFormSceneID {
			return tpl.TemplateChunk{Template: "app_shell", Data: views.ViewAppShell(s)}, targetSelApp, ""
		}
		return tpl.TemplateChunk{Template: "modal_scene_text", Data: views.ViewEditorSceneText(s)}, targetSelModals, modalIDSceneText
	case targetCloseSceneTextModal:
		if state.ActiveWorkspaceView(s) == state.ViewEditor && !s.Config.StandaloneMode {
			return tpl.TemplateChunk{Template: "editor_view", Data: views.ViewEditor(s)}, targetSelWorkspace, ""
		}
		return tpl.TemplateChunk{Template: "modal_scene_text", Data: views.ViewEditorSceneText(s)}, targetSelModals, modalIDSceneText
	case targetSelectStudioScene:
		if !s.Config.StandaloneMode {
			if s.Editor.SceneText.IsOpen() {
				return tpl.TemplateChunk{Template: "app_shell", Data: views.ViewAppShell(s)}, targetSelApp, ""
			}
			if hasFormSceneID {
				return tpl.TemplateChunk{Template: "editor_view", Data: views.ViewEditor(s)}, targetSelWorkspace, ""
			}
			return tpl.TemplateChunk{Template: "editor_context_panel", Data: views.ViewEditorContextPanel(s)}, targetSelEditorContext, ""
		}
		return tpl.TemplateChunk{Template: "player_view", Data: views.ViewPlayer(s)}, targetSelWorkspace, ""
	case targetBranchOrAudioEnded:
		if s.ModalConfirm.IsOpen() {
			return tpl.TemplateChunk{Template: "confirm_modal", Data: views.ViewModalConfirm(s)}, targetSelModals, modalIDConfirm
		}
		return tpl.TemplateChunk{Template: "player_view", Data: views.ViewPlayer(s)}, targetSelWorkspace, ""
	case targetConfirmModalAccept:
		if prevConfirmAction == RemoveChoice && s.Editor.SceneText.IsOpen() {
			return tpl.TemplateChunk{Template: "modal_scene_text", Data: views.ViewEditorSceneText(s)}, targetSelModals, modalIDSceneText
		}
		return tpl.TemplateChunk{Template: "app_shell", Data: views.ViewAppShell(s)}, targetSelApp, ""
	default:
		return tpl.TemplateChunk{Template: "app_shell", Data: views.ViewAppShell(s)}, targetSelApp, ""
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

func isRemoteLANAllowedAction(act, pendingConfirm string) bool {
	switch act {
	case ChooseBranch,
		AutoAdvanceImplicit,
		AudioEnded,
		SaveAudioTime,
		RevealHistory,
		FocusHistoryStep,
		FocusPresent,
		CycleAudioSpeed,
		ToggleAutoChoice,
		SetAutoChoiceDelay,
		SetScrollSensitivity,
		ToggleTheme,
		ToggleLanguage,
		SetLanguage,
		SetUIScale,
		ToggleUIScale,
		ToggleZenMode,
		ToggleImageless,
		PromptRewindStep,
		PromptResetProgress,
		ConfirmModalClose:
		return true
	case ConfirmModalAccept:
		return pendingConfirm == RewindStep || pendingConfirm == ResetProgress
	default:
		return false
	}
}

func HandleAction(s *state.AppState, w http.ResponseWriter, r *http.Request, ensureTemplates func() map[string]*tpl.ParsedTemplate) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !validateSameOriginAction(r) {
		http.Error(w, "forbidden cross-origin action", http.StatusForbidden)
		return
	}
	ParseActionRequest(r)
	act := strings.TrimSpace(r.FormValue("action"))
	tAction := time.Now()

	if act == PollSceneVoiceCapture && !s.Config.StandaloneMode {
		if !isLoopbackRequest(r) {
			http.Error(w, "forbidden remote LAN action", http.StatusForbidden)
			return
		}
		peak, active := osbridge.PollNativeAudioCapturePeak()
		if active {
			w.Header().Set("X-Voice-Peak", fmt.Sprintf("%.3f", peak))
		} else {
			w.Header().Set("X-Voice-Peak", "stopped")
		}
		w.Header().Set("X-Target", targetSelNone)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if !s.Config.StandaloneMode && isLoopbackRequest(r) {
		switch act {
		case StartSceneVoiceCapture:
			s.Mu.Lock()
			canStart := !isHistoryPreview(s) && s.Editor.ActiveBook != nil && !s.Editor.ActiveBook.IsPacked && s.Editor.ActiveBook.Narrated
			s.Mu.Unlock()
			if canStart {
				err := osbridge.StartNativeAudioCapture()
				r = r.WithContext(context.WithValue(r.Context(), voiceCaptureCtxKey{}, voiceCaptureResult{err: err}))
			}
		case StopSceneVoiceCapture:
			pcm, sr, err := osbridge.StopNativeAudioCapture()
			r = r.WithContext(context.WithValue(r.Context(), voiceCaptureCtxKey{}, voiceCaptureResult{pcmLE: pcm, sampleRate: sr, err: err}))
		case SelectSceneVoiceSection:
			_, _, _ = osbridge.StopNativeAudioCapture()
		}
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !isLoopbackRequest(r) && !isRemoteLANAllowedAction(act, s.ModalConfirm.PendingAction) {
		http.Error(w, "forbidden remote LAN action", http.StatusForbidden)
		return
	}

	if s.Config.StandaloneMode {
		s.Settings.ActiveView = state.ViewPlayer.String()
		if isStandaloneForbiddenAction(act) {
			http.Error(w, "forbidden in standalone book mode", http.StatusForbidden)
			return
		}
	}

	toast := ""
	nativeCmd := ""
	prevConfirmAction := s.ModalConfirm.PendingAction
	hadPlayerBook := s.Player.Book != nil
	hadEditorBook := s.Editor.ActiveBook != nil

	if act != SaveAudioTime && s.Player.Book != nil && s.Player.Save != nil {
		if rawAudioTime := strings.TrimSpace(r.FormValue("audio_time")); rawAudioTime != "" {
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

	if spec, ok := ActionSpecs[act]; ok && spec.Handler != nil {
		tStep := time.Now()
		if spec.FlushFormScene {
			flushFormSceneIfPresent(s, r)
		}
		toast = spec.Handler(s, r)
		nativeCmd = spec.NativeCmd
		if act == ConfirmModalAccept && prevConfirmAction == QuitApp {
			nativeCmd = "quit"
		}
		logging.Active.MarkStep(logging.StageActionHandler, tStep, act)
	}

	if act == SaveAudioTime || act == StartWindowDrag {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if s.Editor.SyncToSettings(s.Settings) {
		tSave := time.Now()
		_ = s.SaveSettings()
		logging.Active.MarkStep(logging.StageActionSaveSettings, tSave, act)
	}

	if r.FormValue("silent") == "true" {
		WriteEditorHistoryHeaders(w, s)
		w.WriteHeader(http.StatusNoContent)
		logging.Active.MarkStep(logging.StageActionTotal, tAction, act)
		return
	}

	tBuildView := time.Now()
	chunk, targetSelector, modalID := resolveActionView(s, act, prevConfirmAction, strings.TrimSpace(r.FormValue("scene_id")) != "")
	if ((!hadPlayerBook && s.Player.Book != nil) || (!hadEditorBook && s.Editor.ActiveBook != nil)) && targetSelector == targetSelWorkspace {
		chunk = tpl.TemplateChunk{Template: "app_shell", Data: views.ViewAppShell(s)}
		targetSelector = targetSelApp
		modalID = ""
	}
	logging.Active.MarkStep(logging.StageActionBuildView, tBuildView, act)

	var templates map[string]*tpl.ParsedTemplate
	if targetSelector != targetSelNone && ensureTemplates != nil {
		templates = ensureTemplates()
	}

	if modalID != "" {
		w.Header().Set("X-Modal-ID", modalID)
	}
	writeActionResponseHeaders(w, s, act, targetSelector, toast, nativeCmd)

	if targetSelector == targetSelNone || chunk.Template == "" {
		w.WriteHeader(http.StatusNoContent)
		logging.Active.MarkStep(logging.StageActionTotal, tAction, act)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tRender := time.Now()
	ctx := views.NewViewContext(langOf(s), chunk.Data)
	if err := tpl.RenderTemplateTo(w, chunk.Template, templates, ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	logging.Active.MarkStep(logging.StageActionRenderTpl, tRender, chunk.Template)
	logging.Active.MarkStep(logging.StageActionTotal, tAction, act)
}

func contextBookForHeaders(s *state.AppState) *book.Book {
	if !s.Config.StandaloneMode && s.Settings != nil && s.Settings.ActiveView == state.ViewEditor.String() && s.Editor.ActiveBook != nil {
		return s.Editor.ActiveBook
	}
	if s.Player.Book != nil {
		return s.Player.Book
	}
	return s.Editor.ActiveBook
}

func writeActionResponseHeaders(w http.ResponseWriter, s *state.AppState, act, targetSelector, toast, nativeCmd string) {
	WriteEditorHistoryHeaders(w, s)
	ctxBook := contextBookForHeaders(s)
	activeView := state.ActiveWorkspaceView(s)
	w.Header().Set("X-Target", targetSelector)
	w.Header().Set("X-Theme", s.Settings.Theme)
	w.Header().Set("X-Lang", book.NormalizeLanguage(s.Settings.Language))
	if ctxBook.IsEffectiveImageless(s.Settings) {
		w.Header().Set("X-Imageless", "true")
	} else {
		w.Header().Set("X-Imageless", "false")
	}
	if ctxBook.IsImageless() {
		w.Header().Set("X-Imageless-Locked", "true")
	} else {
		w.Header().Set("X-Imageless-Locked", "false")
	}
	if s.Settings.UIScale == "large" {
		w.Header().Set("X-UI-Scale", "large")
	} else {
		w.Header().Set("X-UI-Scale", "small")
	}
	if s.Player.IsEffectiveZenMode(activeView) {
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
	if isKeepScrollAction(act) {
		w.Header().Set("X-Keep-Scroll", "true")
	}
	if isResetCanvasAction(act) {
		w.Header().Set("X-Reset-Canvas", "true")
	}
	if s.Config.DevMode {
		w.Header().Set("X-Device-Mode", s.Settings.PreviewMode)
	}
	if toast != "" {
		w.Header().Set("X-Toast", toast)
	}
	if nativeCmd != "" {
		w.Header().Set("X-Native-Cmd", nativeCmd)
	}
	if act == PollSceneVoiceCapture {
		peak, active := osbridge.PollNativeAudioCapturePeak()
		if active {
			w.Header().Set("X-Voice-Peak", fmt.Sprintf("%.3f", peak))
		} else {
			w.Header().Set("X-Voice-Peak", "stopped")
		}
	} else if act == StartSceneVoiceCapture && toast != "" {
		w.Header().Set("X-Voice-Peak", "stopped")
	}
}
