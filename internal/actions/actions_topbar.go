//go:build !(js && wasm)

package actions

import (
	"net/http"
	"strings"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
	"mojra/internal/views"
)

func contextBookForTopbarAction(s *state.AppState) *book.Book {
	if !s.Config.StandaloneMode && s.Settings != nil && s.Settings.ActiveView == state.ViewEditor.String() && s.Editor.ActiveBook != nil {
		return s.Editor.ActiveBook
	}
	if s.Player.Book != nil {
		return s.Player.Book
	}
	return s.Editor.ActiveBook
}

func toggleTheme(s *state.AppState, _ *http.Request) string {
	if s.Settings.Theme == "light" {
		s.Settings.Theme = "dark"
	} else {
		s.Settings.Theme = "light"
	}
	_ = s.SaveSettings()
	return ""
}

func setLanguage(s *state.AppState, r *http.Request) string {
	reqLang := strings.TrimSpace(r.FormValue("lang"))
	if reqLang != "" {
		s.Settings.Language = i18n.NormalizeLangCode(reqLang)
	} else {
		next := i18n.NextLang(views.LangOf(s))
		s.Settings.Language = i18n.SupportedLanguages[next].Code
	}
	_ = s.SaveSettings()
	return ""
}

func toggleImageless(s *state.AppState, _ *http.Request) string {
	ctxBook := contextBookForTopbarAction(s)
	if !ctxBook.IsImageless() {
		s.Settings.Imageless = !s.Settings.Imageless
		_ = s.SaveSettings()
	}
	return ""
}

func toggleUIScale(s *state.AppState, r *http.Request) string {
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
	_ = s.SaveSettings()
	return ""
}

func toggleZenMode(s *state.AppState, r *http.Request) string {
	if state.ActiveWorkspaceView(s) != state.ViewPlayer || s.Player.Book == nil || s.Player.Book.IsScreenplay() {
		s.Player.ZenMode = false
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(r.FormValue("zen"))) {
	case "true", "1", "on":
		s.Player.ZenMode = true
	case "false", "0", "off":
		s.Player.ZenMode = false
	default:
		s.Player.ZenMode = !s.Player.ZenMode
	}
	return ""
}
