//go:build !(js && wasm)

package actions

import (
	"net/http"

	"mojra/internal/i18n"
	"mojra/internal/state"
	"mojra/internal/views"
)

type actionTargetKind uint8

const (
	targetApp actionTargetKind = iota
	targetWorkspace
	targetModals
	targetInspector
	targetSceneForm
	targetUndoRedo
	targetNone
)

type ActionSpec struct {
	StandaloneForbidden    bool
	LANAllowed             bool
	KeepScroll             bool
	ResetCanvas            bool
	FlushFormScene         bool
	InspectorWithoutFormID bool
	Target                 actionTargetKind
	NativeCmd              string
	Handler                func(*state.AppState, *http.Request) string
}

func tr(s *state.AppState, k i18n.TrKey) string {
	return i18n.T(views.LangOf(s), k)
}

func trf(s *state.AppState, k i18n.TrKey, args ...any) string {
	return i18n.Tf(views.LangOf(s), k, args...)
}

func flushFormSceneIfPresent(s *state.AppState, r *http.Request) {
	if r == nil || isHistoryPreview(s) || r.FormValue(formKeySceneID) == "" {
		return
	}
	if msg := saveSceneFromForm(s, r); msg == "" {
		return
	}
}

var ActionSpecs = map[string]ActionSpec{
	// Navigation & Global UI
	SwitchView:       {StandaloneForbidden: true, FlushFormScene: true, Target: targetApp, Handler: switchView},
	OpenBook:         {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetApp, Handler: openBook},
	ToggleTheme:      {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: toggleTheme},
	ToggleLanguage:   {KeepScroll: true, LANAllowed: true, FlushFormScene: true, Target: targetApp, Handler: setLanguage},
	SetLanguage:      {KeepScroll: true, LANAllowed: true, FlushFormScene: true, Target: targetApp, Handler: setLanguage},
	SetUIScale:       {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: toggleUIScale},
	ToggleUIScale:    {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: toggleUIScale},
	ToggleZenMode:    {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: toggleZenMode},
	SetDeviceMode:    {StandaloneForbidden: true, KeepScroll: true, Target: targetNone, Handler: setDeviceMode},
	ToggleImageless:  {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: toggleImageless},
	ToggleFullscreen: {KeepScroll: true, Target: targetNone, NativeCmd: nativeCmdToggleFullscreen, Handler: toggleFullscreen},
	StartWindowDrag:  {Target: targetNone, Handler: startWindowDrag},
	QuitApp:          {KeepScroll: true, Target: targetNone, NativeCmd: nativeCmdQuit, Handler: quitApp},
	PromptQuitApp:    {KeepScroll: true, Target: targetNone, NativeCmd: nativeCmdQuit, Handler: quitApp},

	// Modals & Confirmations
	OpenQRModal:               {KeepScroll: true, Target: targetModals, Handler: openQRModal},
	CloseQRModal:              {KeepScroll: true, Target: targetModals, Handler: closeQRModal},
	OpenEditorHelpModal:       {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: openEditorHelpModal},
	CloseEditorHelpModal:      {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: closeEditorHelpModal},
	OpenNewBookModal:          {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: openNewBookModal},
	CloseNewBookModal:         {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: closeNewBookModal},
	SetNewBookModalMode:       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: setNewBookModalModeAction},
	BrowseNewBookFolder:       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: browseNewBookFolder},
	OpenExportModal:           {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: openExportModal},
	CloseExportModal:          {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: closeExportModal},
	ToggleExportFormat:        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: toggleExportFormat},
	ToggleExportModalAdvanced: {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: toggleExportModalAdvanced},
	SetExportModalStandard:    {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: setExportModalStandard},
	BrowseExportModalFolder:   {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: browseExportModalFolder},
	ExportProject:             {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: exportProjectAction},
	OpenAddBookModal:          {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: openAddBookModal},
	CloseAddBookModal:         {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: closeAddBookModal},
	OpenStudioProjectModal:    {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: openStudioProjectModal},
	CloseStudioProjectModal:   {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: closeStudioProjectModal},
	OpenStudioProject:         {StandaloneForbidden: true, ResetCanvas: true, Target: targetApp, Handler: openStudioProjectAction},
	BrowseOpenStudioProject:   {StandaloneForbidden: true, ResetCanvas: true, Target: targetApp, Handler: browseOpenStudioProject},
	OpenStudioProjectFromDisk: {StandaloneForbidden: true, ResetCanvas: true, Target: targetApp, Handler: openStudioProjectFromDisk},
	PromptRewindStep:          {KeepScroll: true, LANAllowed: true, Target: targetModals, Handler: promptRewindStep},
	PromptResetProgress:       {KeepScroll: true, LANAllowed: true, Target: targetModals, Handler: promptResetProgress},
	PromptDeleteScene:         {StandaloneForbidden: true, Target: targetModals, Handler: promptDeleteScene},
	PromptDeleteScenes:        {StandaloneForbidden: true, Target: targetModals, Handler: promptDeleteScenesFromForm},
	PromptRemoveChoice:        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: promptRemoveChoiceFromScene},
	ConfirmModalClose:         {KeepScroll: true, LANAllowed: true, Target: targetModals, Handler: closeConfirmModal},
	ConfirmModalAccept:        {Target: targetApp, Handler: acceptConfirmModalAction},
	OpenSceneTextModal:        {StandaloneForbidden: true, FlushFormScene: true, Target: targetModals, Handler: openSceneTextModal},
	CloseSceneTextModal:       {StandaloneForbidden: true, Target: targetWorkspace, Handler: closeSceneTextModal},
	ToggleSceneTextSettings:   {StandaloneForbidden: true, Target: targetModals, Handler: toggleSceneTextSettings},
	NavigateSceneTextModal:    {StandaloneForbidden: true, Target: targetModals, Handler: navigateSceneTextModal},
	SaveSceneTextModal:        {Target: targetModals, Handler: saveSceneTextModal},

	// Library & Book Packaging
	CreateNewBook:         {StandaloneForbidden: true, ResetCanvas: true, Target: targetApp, Handler: createNewBook},
	BrowseAddBookFromDisk: {StandaloneForbidden: true, ResetCanvas: true, Target: targetApp, Handler: browseAddBookFromDisk},
	AddBookFromDisk:       {StandaloneForbidden: true, ResetCanvas: true, Target: targetApp, Handler: addBookFromDisk},
	AddBookFromURL:        {StandaloneForbidden: true, ResetCanvas: true, Target: targetApp, Handler: addBookFromURLAction},
	RemoveBook:            {StandaloneForbidden: true, Target: targetApp, Handler: removeBookFromForm},
	ExtractEmbeddedBook:   {StandaloneForbidden: true, Target: targetApp, Handler: extractEmbeddedBookAction},

	// Player & Audio
	SaveAudioTime:        {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: saveAudioTime},
	ChooseBranch:         {LANAllowed: true, Target: targetWorkspace, Handler: chooseBranch},
	AutoAdvanceImplicit:  {LANAllowed: true, Target: targetWorkspace, Handler: autoAdvanceImplicit},
	AudioEnded:           {LANAllowed: true, Target: targetWorkspace, Handler: audioEnded},
	RevealHistory:        {LANAllowed: true, Target: targetWorkspace, Handler: revealHistory},
	FocusHistoryStep:     {LANAllowed: true, Target: targetWorkspace, Handler: focusHistoryStep},
	FocusPresent:         {LANAllowed: true, Target: targetWorkspace, Handler: focusPresent},
	CycleAudioSpeed:      {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: cycleAudioSpeed},
	ToggleAutoChoice:     {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: toggleAutoChoice},
	SetAutoChoiceDelay:   {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: setAutoChoiceDelay},
	SetScrollSensitivity: {KeepScroll: true, LANAllowed: true, Target: targetNone, Handler: setScrollSensitivity},
	PlayFromScene:        {StandaloneForbidden: true, FlushFormScene: true, Target: targetApp, Handler: playFromScene},

	// Node Studio & Editor
	SelectStudioScene:           {StandaloneForbidden: true, FlushFormScene: true, Target: targetInspector, Handler: selectStudioScene},
	SelectStudioChapter:         {StandaloneForbidden: true, FlushFormScene: true, Target: targetInspector, Handler: selectStudioChapter},
	CloseBookSettings:           {StandaloneForbidden: true, Target: targetWorkspace, Handler: closeBookSettings},
	ToggleBookSettings:          {StandaloneForbidden: true, FlushFormScene: true, Target: targetWorkspace, Handler: toggleBookSettings},
	SaveBookSettings:            {StandaloneForbidden: true, Target: targetApp, Handler: saveBookSettings},
	SaveScene:                   {StandaloneForbidden: true, Target: targetWorkspace, Handler: saveSceneAction},
	AutoSaveScene:               {StandaloneForbidden: true, Target: targetNone, Handler: autoSaveSceneAction},
	SaveEditorState:             {StandaloneForbidden: true, Target: targetNone, Handler: saveEditorViewportAndNodePositions},
	RenameBook:                  {StandaloneForbidden: true, Target: targetApp, Handler: renameBook},
	ToggleBookNarrated:          {StandaloneForbidden: true, Target: targetWorkspace, Handler: toggleBookNarrated},
	ToggleBookImageless:         {StandaloneForbidden: true, Target: targetApp, Handler: toggleBookImageless},
	SetStartScene:               {StandaloneForbidden: true, Target: targetUndoRedo, Handler: setStartScene},
	SetBookStartChapter:         {StandaloneForbidden: true, Target: targetUndoRedo, Handler: setBookStartChapter},
	SetChapterStartScene:        {StandaloneForbidden: true, Target: targetUndoRedo, Handler: setChapterStartScene},
	SaveChapterSettings:         {StandaloneForbidden: true, Target: targetUndoRedo, Handler: saveChapterSettings},
	BrowseAddBookMedia:          {StandaloneForbidden: true, FlushFormScene: true, Target: targetSceneForm, Handler: browseAddBookMedia},
	AddBookMediaFromDisk:        {StandaloneForbidden: true, FlushFormScene: true, Target: targetSceneForm, Handler: addBookMediaFromDisk},
	ConnectChoice:               {StandaloneForbidden: true, Target: targetUndoRedo, Handler: connectChoiceOnScene},
	ConnectChapter:              {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedo, Handler: connectChapterTransition},
	AddChoice:                   {StandaloneForbidden: true, Target: targetSceneForm, Handler: addChoiceToScene},
	RemoveChoice:                {StandaloneForbidden: true, Target: targetSceneForm, Handler: removeChoiceFromScene},
	ReorderChoice:               {StandaloneForbidden: true, KeepScroll: true, Target: targetSceneForm, Handler: reorderChoiceOnScene},
	BranchChoiceScene:           {StandaloneForbidden: true, Target: targetUndoRedo, Handler: branchSceneFromChoice},
	InsertScene:                 {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedo, Handler: insertScene},
	InsertSceneBefore:           {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedo, Handler: insertSceneBefore},
	CreateSceneAt:               {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedo, Handler: createSceneAt},
	DeleteScene:                 {StandaloneForbidden: true, Target: targetApp, Handler: deleteSceneFromForm},
	DeleteScenes:                {StandaloneForbidden: true, Target: targetApp, Handler: deleteScenesFromForm},
	AutoLayoutNodes:             {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetUndoRedo, Handler: autoLayoutNodes},
	RenameChapter:               {StandaloneForbidden: true, Target: targetUndoRedo, Handler: renameActiveBookChapter},
	AssignChapterToScenes:       {StandaloneForbidden: true, Target: targetUndoRedo, Handler: assignChapterToScenes},
	CreateChapterGroup:          {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedo, Handler: createChapterGroup},
	UndoEditor:                  {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedo, Handler: undoEditor},
	RedoEditor:                  {StandaloneForbidden: true, Target: targetUndoRedo, Handler: redoEditor},
	PreviewHistoryRevision:      {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedo, Handler: previewHistoryRevision},
	ExitHistoryPreview:          {StandaloneForbidden: true, Target: targetUndoRedo, Handler: exitHistoryPreviewAction},
	PlayHistoryRevision:         {StandaloneForbidden: true, Target: targetApp, Handler: playHistoryRevision},
	RestoreHistoryRevision:      {StandaloneForbidden: true, Target: targetUndoRedo, Handler: restoreHistoryRevision},
	CopyHistoryRevisionToBranch: {StandaloneForbidden: true, FlushFormScene: true, Target: targetUndoRedo, Handler: copyHistoryRevisionToBranchAction},
	ToggleEditorHistoryPanel:    {StandaloneForbidden: true, FlushFormScene: true, Target: targetWorkspace, Handler: toggleEditorHistoryPanel},
	OpenEditorHistoryPanel:      {StandaloneForbidden: true, FlushFormScene: true, Target: targetWorkspace, Handler: openEditorHistoryPanel},
	CloseEditorHistoryPanel:     {StandaloneForbidden: true, Target: targetWorkspace, Handler: closeEditorHistoryPanel},
	SaveHistoryVersion:          {StandaloneForbidden: true, FlushFormScene: true, Target: targetWorkspace, Handler: saveHistoryVersion},
	PromptCreateHistoryBranch:   {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModals, Handler: promptCreateHistoryBranchAction},
	CreateHistoryBranch:         {StandaloneForbidden: true, FlushFormScene: true, Target: targetWorkspace, Handler: createHistoryBranchAction},
	SwitchHistoryBranch:         {StandaloneForbidden: true, ResetCanvas: true, FlushFormScene: true, Target: targetApp, Handler: switchHistoryBranchAction},
	ToggleEditorBreakdownPanel:  {StandaloneForbidden: true, FlushFormScene: true, Target: targetWorkspace, Handler: toggleEditorBreakdownPanel},
	CloseEditorBreakdownPanel:   {StandaloneForbidden: true, Target: targetWorkspace, Handler: closeEditorBreakdownPanel},
	RenameCharacter:             {StandaloneForbidden: true, Target: targetUndoRedo, Handler: renameCharacterAction},
	RenameLocation:              {StandaloneForbidden: true, Target: targetUndoRedo, Handler: renameLocationAction},
	BrowseBookExportFolder:      {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: browseBookExportFolder},
	BrowseBookSaveFolder:        {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: browseBookSaveFolder},
	SelectPrimaryBranchScene:    {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: selectPrimaryBranchScene},
	ApplySluglineToken:          {StandaloneForbidden: true, Target: targetSceneForm, Handler: applySluglineTokenAction},
	InsertScreenplayBlock:       {StandaloneForbidden: true, Target: targetSceneForm, Handler: insertScreenplayBlockAction},
	ToggleEditorVariablesPanel:  {StandaloneForbidden: true, FlushFormScene: true, Target: targetWorkspace, Handler: toggleEditorVariablesPanel},
	CloseEditorVariablesPanel:   {StandaloneForbidden: true, Target: targetWorkspace, Handler: closeEditorVariablesPanel},
	AddBookVariable:             {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetSceneForm, Handler: addBookVariableAction},
	ToggleBookVariableBool:      {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedo, Handler: toggleBookVariableBoolAction},
	UpdateBookVariable:          {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedo, Handler: updateBookVariableAction},
	RemoveBookVariable:          {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedo, Handler: removeBookVariableAction},
	AddBookEnumOption:           {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedo, Handler: addBookEnumOptionAction},
	RemoveBookEnumOption:        {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedo, Handler: removeBookEnumOptionAction},
	ToggleChoiceRule:            {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetSceneForm, Handler: toggleChoiceRuleAction},
	ToggleSyntaxHelp:            {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetSceneForm, Handler: toggleSyntaxHelpAction},
	SetSceneTextPreviewMode:     {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModals, Handler: setSceneTextPreviewModeAction},
	SetSceneTextWidthMode:       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: setSceneTextWidthModeAction},
	ToggleSceneTestVarsPopover:  {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModals, Handler: toggleSceneTestVarsPopoverAction},
	UpdateSceneTextTestVar:      {StandaloneForbidden: true, KeepScroll: true, FlushFormScene: true, Target: targetModals, Handler: updateSceneTextTestVarAction},
	ToggleSceneVoiceRecorder:    {StandaloneForbidden: true, KeepScroll: true, InspectorWithoutFormID: true, Target: targetSceneForm, Handler: toggleSceneVoiceRecorderAction},
	SelectSceneVoiceSection:     {StandaloneForbidden: true, KeepScroll: true, InspectorWithoutFormID: true, Target: targetSceneForm, Handler: selectSceneVoiceSectionAction},
	StartSceneVoiceCapture:      {StandaloneForbidden: true, KeepScroll: true, Target: targetNone, Handler: startSceneVoiceCaptureAction},
	PollSceneVoiceCapture:       {StandaloneForbidden: true, KeepScroll: true, Target: targetNone, Handler: pollSceneVoiceCaptureAction},
	StopSceneVoiceCapture:       {StandaloneForbidden: true, KeepScroll: true, Target: targetSceneForm, Handler: stopSceneVoiceCaptureAction},
	SaveSceneVoiceRecording:     {StandaloneForbidden: true, KeepScroll: true, Target: targetSceneForm, Handler: saveSceneVoiceRecordingAction},
	TrimSceneVoice:              {StandaloneForbidden: true, KeepScroll: true, InspectorWithoutFormID: true, Target: targetSceneForm, Handler: trimSceneVoiceAction},
	DeleteSceneVoice:            {StandaloneForbidden: true, KeepScroll: true, Target: targetSceneForm, Handler: deleteSceneVoiceAction},
	ToggleStudioSearch:          {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: toggleStudioSearchAction},
	CloseStudioSearch:           {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: closeStudioSearchAction},
	FilterStudioSearch:          {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: filterStudioSearchAction},
	ToggleBookWarnings:          {StandaloneForbidden: true, KeepScroll: true, Target: targetWorkspace, Handler: toggleBookWarningsAction},
}
