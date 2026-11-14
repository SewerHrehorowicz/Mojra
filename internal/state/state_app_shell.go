package state

import (
	"io/fs"
	"strings"
	"sync"

	"mojra/internal/book"
)

// WorkspaceView identifies the active top-level workspace tab.
type WorkspaceView uint8

const (
	ViewLibrary WorkspaceView = iota
	ViewPlayer
	ViewEditor
)

var workspaceViewNames = [...]string{
	ViewLibrary: "library",
	ViewPlayer:  "player",
	ViewEditor:  "editor",
}

// ParseWorkspaceView parses a persisted or form workspace view name into WorkspaceView.
func ParseWorkspaceView(raw string) WorkspaceView {
	clean := strings.ToLower(strings.TrimSpace(raw))
	for i, name := range workspaceViewNames {
		if clean == name {
			return WorkspaceView(i)
		}
	}
	return ViewLibrary
}

// String returns the canonical wire/INI identifier for the workspace view.
func (v WorkspaceView) String() string {
	if int(v) < len(workspaceViewNames) {
		return workspaceViewNames[v]
	}
	return workspaceViewNames[ViewLibrary]
}

// RuntimeConfig holds the immutable startup configuration and shutdown channel for the server process.
type RuntimeConfig struct {
	RootDir          string
	BooksDir         string
	SavesDir         string
	AppDir           string
	EmbeddedFS       fs.FS
	Port             int
	DevMode          bool
	StandaloneMode   bool
	StandaloneBookID string
	ShutdownCh       chan struct{}
	once             sync.Once
}

// TriggerShutdown safely signals the main loop to close the window and exit.
func (c *RuntimeConfig) TriggerShutdown() {
	if c.ShutdownCh == nil {
		return
	}
	c.once.Do(func() {
		close(c.ShutdownCh)
	})
}

// SimpleModalsState groups boolean open flags for stateless modals without dedicated form drafts.
type SimpleModalsState struct {
	QR             bool
	AddBook        bool
	StudioProjects bool
	VoiceRecorder  bool
	VoicePeak      string
}

// AppState holds the complete application state decomposed into cohesive domain sub-states.
type AppState struct {
	Mu           sync.RWMutex
	Config       RuntimeConfig
	Settings     *book.AppSettings
	Library      LibraryState
	Player       PlayerState
	Editor       EditorState
	Modals       SimpleModalsState
	ModalExport  ModalExportState
	ModalNewBook ModalNewBookState
	ModalConfirm ModalConfirmState
}

// CloseAllModals closes all open modals and popovers across the application.
func (s *AppState) CloseAllModals() {
	s.Modals = SimpleModalsState{}
	if s.Editor.RightPanel == PanelHelp {
		s.Editor.RightPanel = PanelNone
	}
	s.Editor.SceneText.Close()
	s.ModalExport.Reset()
	s.ModalNewBook.Reset()
	s.ModalConfirm.Reset()
}

// WithUnlocked temporarily releases s.Mu around a blocking OS operation (such as a native file picker)
// and re-acquires s.Mu before returning.
func (s *AppState) WithUnlocked(fn func()) {
	s.Mu.Unlock()
	defer s.Mu.Lock()
	fn()
}

// SaveSettings writes the current AppSettings to saves/settings.ini.
func (s *AppState) SaveSettings() error {
	if s.Settings == nil {
		return nil
	}
	return book.SaveAppSettings(s.Config.SavesDir, s.Settings)
}

// ActiveWorkspaceView resolves the currently active workspace tab from AppState.
func ActiveWorkspaceView(s *AppState) WorkspaceView {
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
