//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strings"

	"mojra/domain/book"
	"mojra/domain/state"
	"mojra/domain/views"
	"mojra/engine"
)

const (
	themeValLight = "light"
	themeValDark  = "dark"
	formKeyLang   = "lang"
	formKeyScale  = "scale"
	formKeyDir    = "dir"
	formKeyZen    = "zen"
	scaleValLarge = "large"
	scaleValSmall = "small"
	dirValUp      = "up"
	dirValPlus    = "plus"
	dirValOut     = "out"
	dirValDown    = "down"
	dirValMinus   = "minus"
	boolValOff    = "off"
)

var _ = registerLANActions(map[string]ActionHandler{
	"ToggleTheme":     toggleTheme,
	"ToggleLanguage":  setLanguage,
	"SetLanguage":     setLanguage,
	"SetUIScale":      toggleUIScale,
	"ToggleUIScale":   toggleUIScale,
	"ToggleZenMode":   toggleZenMode,
	"ToggleImageless": toggleImageless,
})

func contextBookForTopbarAction(s *state.AppState) *book.Book {
	if !s.Config.StandaloneMode && s.Settings != nil && s.Settings.ActiveView == state.ViewEditor.String() && s.Editor.ActiveBook != nil {
		return s.Editor.ActiveBook
	}
	if s.Player.Book != nil {
		return s.Player.Book
	}
	return s.Editor.ActiveBook
}

func toggleTheme(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	if s.Settings.Theme == themeValLight {
		s.Settings.Theme = themeValDark
	} else {
		s.Settings.Theme = themeValLight
	}
	_ = s.SaveSettings()
	return frameSettingsKeepScroll(s, "")
}

func setLanguage(s *state.AppState, r *http.Request) []engine.ActionFrame {
	_ = flushFormSceneIfPresent(s, r)
	reqLang := strings.TrimSpace(r.FormValue(formKeyLang))
	if reqLang != "" {
		s.Settings.Language = engine.NormalizeLangCode(reqLang)
	} else {
		next := engine.NextLang(views.LangOf(s))
		s.Settings.Language = engine.SupportedLanguages[next].Code
	}
	_ = s.SaveSettings()
	return frameApp(s, "")
}

func toggleImageless(s *state.AppState, _ *http.Request) []engine.ActionFrame {
	ctxBook := contextBookForTopbarAction(s)
	if !ctxBook.IsImageless() {
		s.Settings.Imageless = !s.Settings.Imageless
		_ = s.SaveSettings()
	}
	if state.ActiveWorkspaceView(s) == state.ViewPlayer {
		return []engine.ActionFrame{{Target: targetSelWorkspace, Chunks: frameWorkspace(s, "")[0].Chunks}}
	}
	return frameSettingsKeepScroll(s, "")
}

func toggleUIScale(s *state.AppState, r *http.Request) []engine.ActionFrame {
	reqScale := strings.ToLower(strings.TrimSpace(r.FormValue(formKeyScale)))
	if reqScale == "" {
		reqScale = strings.ToLower(strings.TrimSpace(r.FormValue(formKeyMode)))
	}
	if reqScale == "" {
		switch strings.ToLower(strings.TrimSpace(r.FormValue(formKeyDir))) {
		case "in", dirValUp, dirValPlus, "+":
			reqScale = scaleValLarge
		case dirValOut, dirValDown, dirValMinus, "-":
			reqScale = scaleValSmall
		}
	}
	if reqScale == scaleValLarge || reqScale == scaleValSmall {
		s.Settings.UIScale = reqScale
	} else if s.Settings.UIScale == scaleValLarge {
		s.Settings.UIScale = scaleValSmall
	} else {
		s.Settings.UIScale = scaleValLarge
	}
	_ = s.SaveSettings()
	return frameSettingsKeepScroll(s, "")
}

func toggleZenMode(s *state.AppState, r *http.Request) []engine.ActionFrame {
	if state.ActiveWorkspaceView(s) != state.ViewPlayer || s.Player.Book == nil || s.Player.Book.IsScreenplay() {
		s.Player.ZenMode = false
		return frameSettingsKeepScroll(s, "")
	}
	switch strings.ToLower(strings.TrimSpace(r.FormValue(formKeyZen))) {
	case boolValTrue, "1", boolValOn:
		s.Player.ZenMode = true
	case formValFalse, "0", boolValOff:
		s.Player.ZenMode = false
	default:
		s.Player.ZenMode = !s.Player.ZenMode
	}
	return frameSettingsKeepScroll(s, "")
}
