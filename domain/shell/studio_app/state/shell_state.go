package state

import (
	"strings"

	book "mojra/domain/book/actions"
	commonstate "mojra/domain/common/state"
	librarystate "mojra/domain/library/state"
	playerstate "mojra/domain/player/state"
	studiostate "mojra/domain/studio/state"
)

type (
	ViewMode          = commonstate.ViewMode
	WorkspaceView     = commonstate.ViewMode
	RuntimeConfig     = commonstate.RuntimeConfig
	OverlayState      = commonstate.OverlayState
	ConfirmState      = commonstate.ConfirmState
	LibraryState      = librarystate.LibraryState
	NewBookState      = librarystate.NewBookState
	PlayerState       = playerstate.PlayerState
	StudioState       = studiostate.StudioState
	EditorState       = studiostate.StudioState
	EditorRightPanel  = studiostate.EditorRightPanel
	TextEditorState   = studiostate.TextEditorState
	SceneWidthMode    = studiostate.SceneWidthMode
	ScenePreviewMode  = studiostate.ScenePreviewMode
	ExportState       = studiostate.ExportState
)

const (
	ViewLibrary          = commonstate.ViewLibrary
	ViewPlayer           = commonstate.ViewPlayer
	ViewEditor           = commonstate.ViewEditor
	PanelNone            = studiostate.PanelNone
	PanelBook            = studiostate.PanelBook
	PanelChapter         = studiostate.PanelChapter
	PanelVariables       = studiostate.PanelVariables
	PanelBreakdown       = studiostate.PanelBreakdown
	PanelHistory         = studiostate.PanelHistory
	PanelHelp            = studiostate.PanelHelp
	WidthModeHalf        = studiostate.WidthModeHalf
	WidthModeFull        = studiostate.WidthModeFull
	PreviewModeNormal    = studiostate.PreviewModeNormal
	PreviewModeHighlight = studiostate.PreviewModeHighlight
	PreviewModeOutput    = studiostate.PreviewModeOutput
)

// ParseWorkspaceView normalizes a raw view name string to ViewLibrary, ViewPlayer, or ViewEditor.
func ParseWorkspaceView(raw string) ViewMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ViewPlayer):
		return ViewPlayer
	case string(ViewEditor):
		return ViewEditor
	default:
		return ViewLibrary
	}
}

// AppState aggregates the shared UIState and the Library, Player, and Studio module states for studio_app.
type AppState struct {
	*commonstate.UIState
	Library librarystate.LibraryState
	NewBook librarystate.NewBookState
	Player  playerstate.PlayerState
	Editor  studiostate.StudioState
	Export  studiostate.ExportState
}

// ShellState is an alias for AppState.
type ShellState = AppState

// NewAppState constructs and wires a new AppState for the studio_app shell.
func NewAppState(cfg RuntimeConfig) *AppState {
	ui := commonstate.NewUIState(cfg)
	app := &AppState{
		UIState: ui,
	}
	app.BindSubstates()
	return app
}

// BindSubstates links Library, Player, and Editor to the shared UIState.
func (s *AppState) BindSubstates() {
	if s.UIState == nil {
		s.UIState = &commonstate.UIState{
			Settings: &book.AppSettings{},
		}
	}
	s.Library.BindUI(s.UIState, &s.NewBook)
	s.Player.BindUI(s.UIState)
	s.Editor.BindUI(s.UIState, &s.Export)
	s.Hooks.PlayerState = &s.Player
	s.Hooks.LibraryState = &s.Library
	s.Hooks.StudioState = &s.Editor
	s.Hooks.ShellState = s
	s.Hooks.ActiveBook = func() *book.Book {
		return s.Player.Book
	}
	s.Hooks.ActiveContextBook = func() *book.Book {
		return ContextBook(s)
	}
	s.Hooks.IsHistoryPreview = func() bool {
		return !s.Config.StandaloneMode && s.Editor.History != nil && s.Editor.History.IsPreview(s.Editor.ActiveBook)
	}
	s.Hooks.ActivePlayerSave = func(bookID string) *book.SaveState {
		if s.Player.Book != nil && s.Player.Book.ID == bookID {
			return s.Player.Save
		}
		return nil
	}
	s.Hooks.IsEffectiveImageless = func() bool {
		return ContextBook(s).IsEffectiveImageless(s.Settings)
	}
	s.Hooks.IsBookLockedImageless = func() bool {
		return ContextBook(s).IsImageless()
	}
	s.Hooks.IsZenMode = func() bool {
		return s.Player.IsEffectiveZenMode(ActiveWorkspaceView(s))
	}
	s.Hooks.ListAllBooks = func() []*book.Book {
		return s.Library.Books
	}
}

// ContextBook returns the active book for the current view (Editor book in Studio, Player book otherwise).
func ContextBook(s *AppState) *book.Book {
	if s == nil {
		return nil
	}
	if ActiveWorkspaceView(s) == ViewEditor {
		if s.Editor.StudioHome {
			return nil
		}
		if s.Editor.ActiveBook != nil {
			return s.Editor.ActiveBook
		}
	}
	return s.Player.Book
}

// CloseAllModals closes all open dialog views and resets transient state.
func (s *AppState) CloseAllModals() {
	s.CloseTransientViews()
	if s.Editor.RightPanel == PanelHelp {
		s.Editor.RightPanel = PanelNone
	}
	s.Editor.SceneText.Close()
	s.Export.Reset()
	s.NewBook.Reset()
}

// ActiveWorkspaceView resolves the currently active workspace tab from AppState.
func ActiveWorkspaceView(s *AppState) ViewMode {
	if s == nil {
		return ViewLibrary
	}
	if s.Config.StandaloneMode {
		return ViewPlayer
	}
	if s.Settings != nil {
		switch ParseWorkspaceView(s.Settings.ActiveView) {
		case ViewEditor:
			return ViewEditor
		case ViewPlayer:
			if s.Player.Book != nil {
				return ViewPlayer
			}
		}
	}
	return ViewLibrary
}
