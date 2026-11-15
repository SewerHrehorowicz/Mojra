//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strings"

	book "mojra/domain/book/actions"
	"mojra/domain/common/state"
	"mojra/domain/common/views"
	"mojra/engine"
	"mojra/engine/osbridge"
	"mojra/engine/transport"
)

const (
	themeLight         = "light"
	themeDark          = "dark"
	scaleSmall         = "small"
	scaleLarge         = "large"
	dirValUp           = "up"
	dirValPlus         = "plus"
	dirValOut          = "out"
	dirValDown         = "down"
	dirValMinus        = "minus"
	modePhone          = "phone"
	modeIPad           = "ipad"
	modeDesktop        = "desktop"
	previewPortrait    = "portrait"
	previewLandscape   = "landscape"
	previewTablet      = "tablet"
	previewFull        = "full"
	nativeCmdFullscr   = "toggle_fullscreen"
	nativeCmdQuit      = "quit"
	paramKeyLang       = "lang"
	paramKeyLevel      = "level"
	paramKeyScale      = "scale"
	paramKeyMode       = "mode"
	paramKeyDir        = "dir"
)

// ActionHandler defines the signature for a UIState action handler.
type ActionHandler func(ui *state.UIState, req *http.Request) []transport.ActionFrame

var (
	// Handlers maps every registered action name to its handler across all domain modules.
	Handlers = map[string]ActionHandler{}
	// LANAllowedActions lists actions permitted from non-loopback LAN clients.
	LANAllowedActions = map[string]bool{}
	// StandaloneAllowedActions lists actions permitted in standalone single-book mode.
	StandaloneAllowedActions = map[string]bool{}
)

// RegisterAction registers a single action handler in the global registry.
func RegisterAction(name string, allowLAN, allowStandalone bool, handler ActionHandler) {
	Handlers[name] = handler
	if allowLAN {
		LANAllowedActions[name] = true
	}
	if allowStandalone {
		StandaloneAllowedActions[name] = true
	}
}

func registerActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		RegisterAction(actionName, false, false, handler)
	}
	return true
}

func registerLANActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		RegisterAction(actionName, true, false, handler)
	}
	return true
}

func registerStandaloneActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		RegisterAction(actionName, true, true, handler)
	}
	return true
}

var (
	_ = registerStandaloneActions(map[string]ActionHandler{
		"ToggleTheme":          toggleTheme,
		"ToggleLanguage":       setLanguage,
		"SetLanguage":          setLanguage,
		"SetUIScale":           toggleUIScale,
		"ToggleUIScale":        toggleUIScale,
		"ToggleImageless":      toggleImageless,
		"SetScrollSensitivity": setScrollSensitivity,
		"SetDeviceMode":        setDeviceMode,
	})
	_ = registerActions(map[string]ActionHandler{
		"ToggleFullscreen": toggleFullscreen,
		"StartWindowDrag":  startWindowDrag,
		"QuitApp":          quitApp,
	})
)

func toggleTheme(ui *state.UIState, _ *http.Request) []transport.ActionFrame {
	if ui.Settings.Theme == themeLight {
		ui.Settings.Theme = themeDark
	} else {
		ui.Settings.Theme = themeLight
	}
	ui.SaveSettings()
	if ui.Hooks.RenderToastFrames != nil {
		return ui.Hooks.RenderToastFrames("")
	}
	return views.ToastFrames(ui, "")
}

func toggleUIScale(ui *state.UIState, req *http.Request) []transport.ActionFrame {
	reqScale := strings.ToLower(strings.TrimSpace(req.FormValue(paramKeyScale)))
	if reqScale == "" {
		reqScale = strings.ToLower(strings.TrimSpace(req.FormValue(paramKeyMode)))
	}
	if reqScale == "" {
		switch strings.ToLower(strings.TrimSpace(req.FormValue(paramKeyDir))) {
		case "in", dirValUp, dirValPlus, "+":
			reqScale = scaleLarge
		case dirValOut, dirValDown, dirValMinus, "-":
			reqScale = scaleSmall
		}
	}
	if reqScale == scaleLarge || reqScale == scaleSmall {
		ui.Settings.UIScale = reqScale
	} else if ui.Settings.UIScale == scaleLarge {
		ui.Settings.UIScale = scaleSmall
	} else {
		ui.Settings.UIScale = scaleLarge
	}
	ui.SaveSettings()
	if ui.Hooks.RenderToastFrames != nil {
		return ui.Hooks.RenderToastFrames("")
	}
	return views.ToastFrames(ui, "")
}

func toggleImageless(ui *state.UIState, _ *http.Request) []transport.ActionFrame {
	if !ui.BookLockedImageless() {
		ui.Settings.Imageless = !ui.Settings.Imageless
		ui.SaveSettings()
	}
	if ui.Settings.ActiveView == string(state.ViewPlayer) && ui.Hooks.RenderWorkspaceFrames != nil {
		return ui.Hooks.RenderWorkspaceFrames()
	}
	if ui.Hooks.RenderToastFrames != nil {
		return ui.Hooks.RenderToastFrames("")
	}
	return views.ToastFrames(ui, "")
}

func setLanguage(ui *state.UIState, req *http.Request) []transport.ActionFrame {
	reqLang := strings.TrimSpace(req.FormValue(paramKeyLang))
	if reqLang != "" {
		ui.Settings.Language = engine.NormalizeLangCode(reqLang)
	} else {
		next := engine.NextLang(views.LangOf(ui))
		ui.Settings.Language = engine.SupportedLanguages[next].Code
	}
	ui.SaveSettings()
	if ui.Hooks.RenderAppFrames != nil {
		return ui.Hooks.RenderAppFrames()
	}
	return views.ToastFrames(ui, "")
}

func setScrollSensitivity(ui *state.UIState, req *http.Request) []transport.ActionFrame {
	rawSens := strings.TrimSpace(req.FormValue(paramKeyLevel))
	if rawSens == "" {
		switch book.NormalizeScrollSensitivity(ui.Settings.ScrollSensitivity) {
		case "high":
			rawSens = "medium"
		case "medium":
			rawSens = "low"
		case "low":
			rawSens = "off"
		default:
			rawSens = "high"
		}
	}
	ui.Settings.ScrollSensitivity = book.NormalizeScrollSensitivity(rawSens)
	ui.SaveSettings()
	if ui.Hooks.RenderToastFrames != nil {
		return ui.Hooks.RenderToastFrames("")
	}
	return views.ToastFrames(ui, "")
}

func setDeviceMode(ui *state.UIState, req *http.Request) []transport.ActionFrame {
	if ui.Config.DevMode {
		mode := strings.ToLower(strings.TrimSpace(req.FormValue(paramKeyMode)))
		switch mode {
		case modePhone:
			mode = previewPortrait
		case modeIPad:
			mode = previewTablet
		case modeDesktop:
			mode = previewFull
		}
		switch mode {
		case previewPortrait, previewLandscape, previewTablet, previewFull:
			ui.Settings.PreviewMode = mode
			ui.SaveSettings()
		}
	}
	if ui.Hooks.RenderToastFrames != nil {
		return ui.Hooks.RenderToastFrames("")
	}
	return views.ToastFrames(ui, "")
}

func toggleFullscreen(ui *state.UIState, _ *http.Request) []transport.ActionFrame {
	ui.Overlays.NativeCmd = nativeCmdFullscr
	osbridge.ToggleNativeWindowFullscreen()
	return views.ToastFrames(ui, "")
}

func startWindowDrag(_ *state.UIState, _ *http.Request) []transport.ActionFrame {
	osbridge.StartNativeWindowDrag()
	return nil
}

func quitApp(ui *state.UIState, _ *http.Request) []transport.ActionFrame {
	ui.Confirm.Clear()
	ui.Overlays.NativeCmd = nativeCmdQuit
	go func() {
		ui.Config.TriggerShutdown()
		osbridge.TerminateNativeWindow()
	}()
	return views.ToastFrames(ui, "")
}
