package state

import (
	"io/fs"
	"sync"

	book "mojra/domain/book/actions"
	"mojra/domain/book/model"
	"mojra/engine/transport"
)

// ViewMode identifies the active workspace tab in the studio_app shell.
type ViewMode string

const (
	ViewLibrary ViewMode = "library"
	ViewPlayer  ViewMode = "player"
	ViewEditor  ViewMode = "editor"

	TargetNone          = "none"
	TargetApp           = "#app"
	TargetWorkspace     = "#workspace"
	TargetModalsRoot    = "#modals-root"
	TargetEditorContext = "#editor-context-view"
)

// RuntimeConfig holds the immutable startup configuration of the application.
type RuntimeConfig struct {
	RootDir          string
	BooksDir         string
	SavesDir         string
	AppDir           string
	EmbeddedFS       fs.FS
	DevMode          bool
	StandaloneMode   bool
	StandaloneBookID string
	Port             int
	ShutdownCh       chan struct{}
	ShutdownOnce     sync.Once
}

// TriggerShutdown signals the application host to shut down cleanly.
func (cfg *RuntimeConfig) TriggerShutdown() {
	cfg.ShutdownOnce.Do(func() {
		if cfg.ShutdownCh != nil {
			close(cfg.ShutdownCh)
		}
	})
}

// OverlayState tracks active view flags and transient UI feedback notifications.
type OverlayState struct {
	QR             bool
	AddBook        bool
	StudioProjects bool
	VoiceRecorder  bool
	Toast          string
	NativeCmd      string
	VoicePeak      string
	ResetCanvas    bool
}

// ShellHooks provides cross-module callbacks wired by the active shell without creating import cycles.
type ShellHooks struct {
	PlayerState           any
	LibraryState          any
	StudioState           any
	ShellState            any
	ActiveBook            func() *model.Book
	ActiveContextBook     func() *model.Book
	IsHistoryPreview      func() bool
	ActivePlayerSave      func(bookID string) *model.SaveState
	IsEffectiveImageless  func() bool
	IsBookLockedImageless func() bool
	IsZenMode             func() bool
	RenderAppFrames       func() []transport.ActionFrame
	RenderWorkspaceFrames func() []transport.ActionFrame
	RenderModalsFrame     func() transport.ActionFrame
	RenderToastFrames     func(toast string) []transport.ActionFrame
	ReloadAllBooks        func()
	ListAllBooks          func() []*model.Book
	OpenBookInView        func(bookID string, view ViewMode) string
	RemoveBookByID        func(bookID string) (string, bool)
	OnBookCreated         func(createdBook *model.Book, openTarget string)
	OnBookMutated         func(mutatedBook *model.Book)
	PlayBookWithSave      func(targetBook *model.Book, save *model.SaveState, isHistPreview bool, histLabel string)
	ExitHistoryPreview    func() string
	UnpackPlaybookForEdit func(bookID string) (string, bool)
	ExecutePendingConfirm func(pendingAction, pendingParam string) ([]transport.ActionFrame, bool)
}

// UIState holds the shared UI state, settings, overlays, and confirmation state across modules.
type UIState struct {
	Mu       sync.Mutex
	Config   RuntimeConfig
	Settings *model.AppSettings
	Overlays OverlayState
	Confirm  ConfirmState
	Hooks    ShellHooks
}

// NewUIState constructs a new shared UIState and loads settings.ini from cfg.SavesDir.
func NewUIState(cfg RuntimeConfig) *UIState {
	if cfg.ShutdownCh == nil {
		cfg.ShutdownCh = make(chan struct{})
	}
	settings := book.LoadAppSettings(cfg.SavesDir)
	return &UIState{
		Config:   cfg,
		Settings: settings,
	}
}

// SaveSettings persists the current AppSettings to disk in SavesDir.
func (ui *UIState) SaveSettings() {
	if ui == nil || ui.Settings == nil {
		return
	}
	if err := book.SaveAppSettings(ui.Config.SavesDir, ui.Settings); err != nil {
		return
	}
}

// WithUnlocked temporarily releases ui.Mu while executing blockingOSOp (such as a native OS file dialog) and re-locks afterward.
func (ui *UIState) WithUnlocked(blockingOSOp func()) {
	ui.Mu.Unlock()
	defer ui.Mu.Lock()
	blockingOSOp()
}

// CloseTransientViews closes open dialog views and clears any pending confirmation.
func (ui *UIState) CloseTransientViews() {
	if ui == nil {
		return
	}
	ui.Overlays.QR = false
	ui.Overlays.AddBook = false
	ui.Overlays.StudioProjects = false
	ui.Overlays.VoiceRecorder = false
	ui.Confirm.Clear()
}

// ConsumeTransientFeedback clears and returns the current Toast and NativeCmd values.
func (ui *UIState) ConsumeTransientFeedback() (string, string) {
	if ui == nil {
		return "", ""
	}
	toast := ui.Overlays.Toast
	nativeCmd := ui.Overlays.NativeCmd
	ui.Overlays.Toast = ""
	ui.Overlays.NativeCmd = ""
	return toast, nativeCmd
}

// EffectiveImageless reports whether imageless mode is active for the current book or global settings.
func (ui *UIState) EffectiveImageless() bool {
	if ui != nil && ui.Hooks.IsEffectiveImageless != nil {
		return ui.Hooks.IsEffectiveImageless()
	}
	return ui != nil && ui.Settings != nil && ui.Settings.Imageless
}

// BookLockedImageless reports whether the active book forces imageless mode.
func (ui *UIState) BookLockedImageless() bool {
	if ui != nil && ui.Hooks.IsBookLockedImageless != nil {
		return ui.Hooks.IsBookLockedImageless()
	}
	return false
}

// ZenModeActive reports whether Zen mode is active in the player.
func (ui *UIState) ZenModeActive() bool {
	if ui != nil && ui.Hooks.IsZenMode != nil {
		return ui.Hooks.IsZenMode()
	}
	return false
}
