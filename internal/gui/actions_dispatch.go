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

// ActionSpec centralizes metadata, DOM chunk target rules, and the 1:1 handler for a POST /api/action command.
type ActionSpec struct {
	StandaloneForbidden bool
	KeepScroll          bool
	ResetCanvas         bool
	Target              actionTargetKind
	NativeCmd           string
	Handler             func(*Server, *http.Request) string
}

var actionSpecs = map[string]ActionSpec{
	// Navigation & Global UI
	"SwitchView":       {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).switchView},
	"OpenBook":         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).openBook},
	"ToggleTheme":      {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).toggleTheme},
	"SetUIScale":       {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).toggleUIScale},
	"ToggleUIScale":    {KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).toggleUIScale},
	"SetDeviceMode":    {StandaloneForbidden: true, KeepScroll: true, Target: targetThemeOrScale, Handler: (*Server).setDeviceMode},
	"ToggleImageless":  {KeepScroll: true, Target: targetImageless, Handler: (*Server).toggleImageless},
	"ToggleFullscreen": {KeepScroll: true, Target: targetNone, NativeCmd: "toggle-fullscreen", Handler: (*Server).toggleFullscreen},
	"StartWindowDrag":  {Target: targetNone, Handler: (*Server).startWindowDrag},
	"QuitApp":          {KeepScroll: true, Target: targetNone, NativeCmd: "quit", Handler: (*Server).quitApp},
	"PromptQuitApp":    {KeepScroll: true, Target: targetNone, NativeCmd: "quit", Handler: (*Server).quitApp},

	// Modals & Confirmations
	"OpenQRModal":             {KeepScroll: true, Target: targetModals, Handler: (*Server).openQRModal},
	"CloseQRModal":            {KeepScroll: true, Target: targetModals, Handler: (*Server).closeQRModal},
	"OpenSettingsModal":       {KeepScroll: true, Target: targetModals, Handler: (*Server).openQRModal},
	"CloseSettingsModal":      {KeepScroll: true, Target: targetModals, Handler: (*Server).closeQRModal},
	"OpenEditorHelpModal":     {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).openEditorHelpModal},
	"CloseEditorHelpModal":    {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).closeEditorHelpModal},
	"OpenNewBookModal":        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).openNewBookModal},
	"CloseNewBookModal":       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).closeNewBookModal},
	"BrowseNewBookFolder":     {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).browseNewBookFolder},
	"OpenExportModal":         {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).openExportModal},
	"CloseExportModal":        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).closeExportModal},
	"BrowseExportModalFolder": {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).browseExportModalFolder},
	"ExportProject":           {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).exportProjectAction},
	"OpenAddBookModal":        {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).openAddBookModal},
	"CloseAddBookModal":       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).closeAddBookModal},
	"OpenStudioProjectModal":    {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).openStudioProjectModal},
	"CloseStudioProjectModal":   {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).closeStudioProjectModal},
	"OpenStudioProject":         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).openStudioProjectAction},
	"BrowseOpenStudioProject":   {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).browseOpenStudioProject},
	"OpenStudioProjectFromDisk": {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).openStudioProjectFromDisk},
	"PromptRewindStep":        {KeepScroll: true, Target: targetModals, Handler: (*Server).promptRewindStep},
	"PromptResetProgress":     {KeepScroll: true, Target: targetModals, Handler: (*Server).promptResetProgress},
	"PromptRemoveBook":        {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).promptRemoveBook},
	"PromptDeleteScene":       {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).promptDeleteScene},
	"PromptDeleteScenes":      {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).promptDeleteScenesFromForm},
	"PromptRemoveChoice":      {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).promptRemoveChoiceFromScene},
	"ConfirmModalClose":       {KeepScroll: true, Target: targetModals, Handler: (*Server).closeConfirmModal},
	"ConfirmModalAccept":      {Target: targetConfirmModalAccept, Handler: (*Server).acceptConfirmModalAction},
	"OpenSceneTextModal":      {StandaloneForbidden: true, Target: targetOpenSceneTextModal, Handler: (*Server).openSceneTextModal},
	"CloseSceneTextModal":     {StandaloneForbidden: true, Target: targetCloseSceneTextModal, Handler: (*Server).closeSceneTextModal},
	"ToggleSceneTextSettings": {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).toggleSceneTextSettings},
	"NavigateSceneTextModal":  {StandaloneForbidden: true, Target: targetModals, Handler: (*Server).navigateSceneTextModal},
	"SaveSceneTextModal":      {Target: targetModals, Handler: (*Server).saveSceneTextModal},

	// Library & Book Packaging
	"CreateNewBook":         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).createNewBook},
	"BrowseAddBookFromDisk": {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).browseAddBookFromDisk},
	"AddBookFromDisk":       {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).addBookFromDisk},
	"AddBookFromURL":        {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).addBookFromURLAction},
	"RemoveBook":            {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).removeBookFromForm},
	"PackBook":              {StandaloneForbidden: true, Target: targetNone, Handler: (*Server).packActiveBook},
	"ExtractEmbeddedBook":   {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).extractEmbeddedBookAction},

	// Player & Audio
	"SaveAudioTime":       {KeepScroll: true, Target: targetNone, Handler: (*Server).saveAudioTime},
	"ChooseBranch":        {Target: targetBranchOrAudioEnded, Handler: (*Server).chooseBranch},
	"AutoAdvanceImplicit": {Target: targetBranchOrAudioEnded, Handler: (*Server).autoAdvanceImplicit},
	"AudioEnded":          {Target: targetBranchOrAudioEnded, Handler: (*Server).audioEnded},
	"RewindStepNow":       {Target: targetPlayerView, Handler: (*Server).rewindStepNow},
	"RevealHistory":       {Target: targetPlayerView, Handler: (*Server).revealHistory},
	"FocusHistoryStep":    {Target: targetPlayerView, Handler: (*Server).focusHistoryStep},
	"FocusPresent":        {Target: targetPlayerView, Handler: (*Server).focusPresent},
	"CycleAudioSpeed":     {KeepScroll: true, Target: targetNone, Handler: (*Server).cycleAudioSpeed},
	"ToggleAutoChoice":    {KeepScroll: true, Target: targetNone, Handler: (*Server).toggleAutoChoice},
	"ToggleAutoStrategy":  {KeepScroll: true, Target: targetNone, Handler: (*Server).toggleAutoStrategy},
	"PlayFromScene":       {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).playFromScene},

	// Node Studio & Editor
	"SelectStudioScene":        {StandaloneForbidden: true, Target: targetSelectStudioScene, Handler: (*Server).selectStudioScene},
	"SelectStudioChapter":      {StandaloneForbidden: true, Target: targetSelectStudioScene, Handler: (*Server).selectStudioChapter},
	"OpenBookSettings":         {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).openBookSettings},
	"CloseBookSettings":        {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).closeBookSettings},
	"ToggleBookSettings":       {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).toggleBookSettings},
	"SaveBookSettings":         {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).saveBookSettings},
	"SaveScene":                {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).saveSceneAction},
	"AutoSaveScene":            {StandaloneForbidden: true, Target: targetNone, Handler: (*Server).autoSaveSceneAction},
	"SaveEditorState":          {StandaloneForbidden: true, Target: targetNone, Handler: (*Server).saveEditorViewportAndNodePositions},
	"RenameBook":               {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).renameBook},
	"ToggleBookNarrated":       {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).toggleBookNarrated},
	"ToggleBookImageless":      {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).toggleBookImageless},
	"SetStartScene":            {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).setStartScene},
	"SetBookStartChapter":      {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).setBookStartChapter},
	"SetChapterStartScene":     {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).setChapterStartScene},
	"SaveChapterSettings":      {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).saveChapterSettings},
	"BrowseAddBookMedia":       {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).browseAddBookMedia},
	"AddBookMediaFromDisk":     {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).addBookMediaFromDisk},
	"ConnectChoice":            {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).connectChoiceOnScene},
	"ConnectChapter":           {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).connectChapterTransition},
	"AddChoice":                {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).addChoiceToScene},
	"RemoveChoice":             {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).removeChoiceFromScene},
	"ReorderChoice":            {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).reorderChoiceOnScene},
	"BranchChoiceScene":        {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).branchSceneFromChoice},
	"InsertScene":              {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).insertScene},
	"InsertSceneBefore":        {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).insertSceneBefore},
	"CreateSceneAt":            {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).createSceneAt},
	"DeleteScene":              {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).deleteSceneFromForm},
	"DeleteScenes":             {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).deleteScenesFromForm},
	"AutoLayoutNodes":          {StandaloneForbidden: true, ResetCanvas: true, Target: targetActiveWorkspace, Handler: (*Server).autoLayoutNodes},
	"RenameChapter":            {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).renameActiveBookChapter},
	"AssignChapterToScenes":    {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).assignChapterToScenes},
	"CreateChapterGroup":       {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).createChapterGroup},
	"UndoEditor":               {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).undoEditor},
	"RedoEditor":               {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).redoEditor},
	"PreviewHistoryRevision":   {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).previewHistoryRevision},
	"ExitHistoryPreview":       {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).exitHistoryPreviewAction},
	"PlayHistoryRevision":         {StandaloneForbidden: true, Target: targetAppShell, Handler: (*Server).playHistoryRevision},
	"RestoreHistoryRevision":      {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).restoreHistoryRevision},
	"CopyHistoryRevisionToBranch": {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).copyHistoryRevisionToBranchAction},
	"ToggleEditorHistoryPanel":    {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).toggleEditorHistoryPanel},
	"OpenEditorHistoryPanel":     {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).openEditorHistoryPanel},
	"CloseEditorHistoryPanel":    {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).closeEditorHistoryPanel},
	"SaveHistoryVersion":          {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).saveHistoryVersion},
	"PromptCreateHistoryBranch":   {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).promptCreateHistoryBranchAction},
	"CreateHistoryBranch":         {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).createHistoryBranchAction},
	"SwitchHistoryBranch":         {StandaloneForbidden: true, ResetCanvas: true, Target: targetAppShell, Handler: (*Server).switchHistoryBranchAction},
	"DeleteHistoryBranch":        {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).deleteHistoryBranchAction},
	"ToggleEditorBreakdownPanel": {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).toggleEditorBreakdownPanel},
	"OpenEditorBreakdownPanel":   {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).openEditorBreakdownPanel},
	"CloseEditorBreakdownPanel":  {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).closeEditorBreakdownPanel},
	"RenameCharacter":            {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).renameCharacterAction},
	"RenameLocation":             {StandaloneForbidden: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).renameLocationAction},
	"ExportScreenplayPDF":        {StandaloneForbidden: true, Target: targetNone, Handler: (*Server).exportScreenplayPDFAction},
	"ExportScreenplayFountain":   {StandaloneForbidden: true, Target: targetNone, Handler: (*Server).exportScreenplayFountainAction},
	"BrowseBookExportFolder":     {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).browseBookExportFolder},
	"BrowseBookSaveFolder":       {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).browseBookSaveFolder},
	"SelectPrimaryBranchScene":   {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspace, Handler: (*Server).selectPrimaryBranchScene},
	"ApplySluglineToken":         {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).applySluglineTokenAction},
	"InsertScreenplayBlock":      {StandaloneForbidden: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).insertScreenplayBlockAction},
	"ToggleEditorVariablesPanel": {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).toggleEditorVariablesPanel},
	"OpenEditorVariablesPanel":   {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).openEditorVariablesPanel},
	"CloseEditorVariablesPanel":  {StandaloneForbidden: true, Target: targetActiveWorkspace, Handler: (*Server).closeEditorVariablesPanel},
	"AddBookVariable":            {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).addBookVariableAction},
	"ToggleBookVariableBool":     {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).toggleBookVariableBoolAction},
	"UpdateBookVariable":         {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).updateBookVariableAction},
	"RemoveBookVariable":         {StandaloneForbidden: true, KeepScroll: true, Target: targetUndoRedoWorkspaceOrAppShell, Handler: (*Server).removeBookVariableAction},
	"ToggleChoiceRule":           {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).toggleChoiceRuleAction},
	"ToggleSyntaxHelp":           {StandaloneForbidden: true, KeepScroll: true, Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen, Handler: (*Server).toggleSyntaxHelpAction},
	"SetSceneTextPreviewMode":     {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).setSceneTextPreviewModeAction},
	"SetSceneTextWidthMode":       {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).setSceneTextWidthModeAction},
	"ToggleSceneTestVarsPopover":  {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).toggleSceneTestVarsPopoverAction},
	"UpdateSceneTextTestVar":      {StandaloneForbidden: true, KeepScroll: true, Target: targetModals, Handler: (*Server).updateSceneTextTestVarAction},
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
			if action == "AddBookVariable" && s.EditorVariablesPanelOpen {
				return "app_shell", "#app"
			}
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
			if s.SceneTextModalID != "" {
				return "app_shell", "#app"
			}
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

func (s *Server) switchView(r *http.Request) string {
	if !s.isHistoryPreview() && r.FormValue("scene_id") != "" {
		_ = s.saveSceneFromForm(r)
	}
	view := strings.TrimSpace(r.FormValue("view"))
	if view == "library" && s.isHistoryPreview() {
		s.exitHistoryPreview()
	}
	if view == "player" && s.PlayerBook == nil {
		if s.ActiveBook != nil {
			s.setPlayerBook(s.ActiveBook)
		} else if books, err := s.loadBooks(); err == nil && len(books) > 0 {
			s.setPlayerBook(books[0])
		}
	}
	if view == "library" || view == "player" || view == "editor" {
		s.Settings.ActiveView = view
		s.closeAllModals()
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	}
	return ""
}

func (s *Server) toggleTheme(_ *http.Request) string {
	if s.Settings.Theme == "light" {
		s.Settings.Theme = "dark"
	} else {
		s.Settings.Theme = "light"
	}
	_ = book.SaveAppSettings(s.SavesDir, s.Settings)
	return ""
}

func (s *Server) toggleImageless(_ *http.Request) string {
	if !s.isBookImageless() {
		s.Settings.Imageless = !s.Settings.Imageless
		_ = book.SaveAppSettings(s.SavesDir, s.Settings)
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
	if reqScale == "large" || reqScale == "small" || reqScale == "zen" {
		s.Settings.UIScale = reqScale
	} else {
		switch s.Settings.UIScale {
		case "large":
			s.Settings.UIScale = "zen"
		case "zen":
			s.Settings.UIScale = "small"
		default:
			s.Settings.UIScale = "large"
		}
	}
	_ = book.SaveAppSettings(s.SavesDir, s.Settings)
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
			_ = book.SaveAppSettings(s.SavesDir, s.Settings)
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
		s.BookSettingsOpen = false
		s.SelectedChapter = ""
		s.EditorBreakdownPanelOpen = false
		s.EditorVariablesPanelOpen = false
		s.EditorHistoryPanelOpen = false
	}
	return ""
}

func (s *Server) closeEditorHelpModal(_ *http.Request) string {
	s.EditorHelpModalOpen = false
	return ""
}

func (s *Server) openExportModal(_ *http.Request) string {
	if s.ActiveBook == nil {
		return "Brak aktywnego projektu"
	}
	s.QRModalOpen = false
	s.NewBookModalOpen = false
	s.AddBookModalOpen = false
	s.ExportModalOpen = true
	if s.ActiveBook.IsScreenplay() {
		s.ExportModalFormat = "pdf"
	} else {
		s.ExportModalFormat = "playbook"
	}
	s.ExportModalDir = s.defaultExportDir()
	return ""
}

func (s *Server) closeExportModal(_ *http.Request) string {
	s.ExportModalOpen = false
	s.ExportModalFormat = ""
	s.ExportModalDir = ""
	return ""
}

func (s *Server) openNewBookModal(_ *http.Request) string {
	s.QRModalOpen = false
	s.AddBookModalOpen = false
	s.ExportModalOpen = false
	s.NewBookModalOpen = true
	s.NewBookDraftTitle = ""
	s.NewBookDraftAuthor = ""
	s.NewBookDraftMode = "book"
	s.NewBookDraftStandard = "pl"
	s.NewBookDraftNarrated = false
	s.NewBookDraftImageless = false
	s.NewBookDraftDir = s.defaultNewBookDir()
	return ""
}

func (s *Server) closeNewBookModal(_ *http.Request) string {
	s.NewBookModalOpen = false
	s.NewBookDraftTitle = ""
	s.NewBookDraftAuthor = ""
	s.NewBookDraftMode = ""
	s.NewBookDraftStandard = ""
	s.NewBookDraftNarrated = false
	s.NewBookDraftImageless = false
	s.NewBookDraftDir = ""
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
			if !s.isHistoryPreview() {
				_ = book.WriteSaveState(s.SavesDir, s.Save)
			}
			toast = "Cofnięto opowieść do wybranej sceny."
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
			toast = "Postęp został zresetowany do początku."
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
					if !s.isHistoryPreview() {
						_ = book.UpdateSaveAudioTimeInPlace(s.SavesDir, s.PlayerBook.ID, t, s.PlayerBook.StartSceneID)
					}
				}
			}
		}
	}

	if spec, ok := actionSpecs[action]; ok && spec.Handler != nil {
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

	if r.FormValue("silent") == "true" {
		s.writeEditorHistoryHeaders(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	targetTpl, targetSelector := s.resolveActionChunkTarget(action, prevConfirmAction, strings.TrimSpace(r.FormValue("scene_id")) != "")
	if ((!hadPlayerBook && s.PlayerBook != nil) || (!hadEditorBook && s.ActiveBook != nil)) && targetSelector == "#workspace" {
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
	switch s.Settings.UIScale {
	case "large":
		w.Header().Set("X-UI-Scale", "large")
	case "zen":
		w.Header().Set("X-UI-Scale", "zen")
	default:
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
