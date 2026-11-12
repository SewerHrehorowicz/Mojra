//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strings"

	"mojra/internal/osbridge"
	"mojra/internal/state"
)

func switchView(s *state.AppState, r *http.Request) string {
	view := strings.TrimSpace(r.FormValue("view"))
	prevView := ""
	if s.Settings != nil {
		prevView = s.Settings.ActiveView
	}
	wasActiveStudioEditor := prevView == state.ViewEditor.String() && s.Editor.HasActiveBook()
	if (view == state.ViewLibrary.String() || view == "studio") && isHistoryPreview(s) {
		exitHistoryPreview(s)
	}
	if view == "studio" {
		if wasActiveStudioEditor {
			s.Editor.SaveBookUIState(s.Settings)
		}
		s.Editor.StudioHome = true
		s.Player.ZenMode = false
		s.Settings.ActiveView = state.ViewEditor.String()
		s.CloseAllModals()
		if s.Editor.SyncToSettings(s.Settings) {
			_ = s.SaveSettings()
		} else {
			_ = s.SaveSettings()
		}
		return ""
	}
	if view == state.ViewEditor.String() {
		s.Editor.StudioHome = false
	}
	if view == state.ViewPlayer.String() && s.Player.Book == nil {
		if s.Editor.ActiveBook != nil {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, s.Editor.ActiveBook)
		} else if len(s.Library.Books) > 0 {
			s.Player.SetBook(s.Config.SavesDir, s.Settings, s.Library.Books[0])
		}
	}
	if view == state.ViewLibrary.String() || view == state.ViewPlayer.String() || view == state.ViewEditor.String() {
		if view != state.ViewEditor.String() && wasActiveStudioEditor {
			s.Editor.SaveBookUIState(s.Settings)
		}
		if view != state.ViewPlayer.String() {
			s.Player.ZenMode = false
		}
		s.Settings.ActiveView = view
		s.CloseAllModals()
		if view == state.ViewEditor.String() && !wasActiveStudioEditor {
			s.Editor.RestoreBookUIState(s.Settings, true)
		}
		s.Editor.SyncToSettings(s.Settings)
		_ = s.SaveSettings()
	}
	return ""
}

func setDeviceMode(s *state.AppState, r *http.Request) string {
	if s.Config.DevMode {
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
			_ = s.SaveSettings()
		}
	}
	return ""
}

func toggleFullscreen(_ *state.AppState, _ *http.Request) string {
	osbridge.ToggleNativeWindowFullscreen()
	return ""
}

func startWindowDrag(_ *state.AppState, _ *http.Request) string {
	osbridge.StartNativeWindowDrag()
	return ""
}

func quitApp(s *state.AppState, _ *http.Request) string {
	s.ModalConfirm.Reset()
	go func() {
		s.Config.TriggerShutdown()
		osbridge.TerminateNativeWindow()
	}()
	return ""
}
