//go:build !(js && wasm)

package views

import (
	book "mojra/domain/book/actions"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	libraryviews "mojra/domain/library/views"
	licenseviews "mojra/domain/license/views"
	playerviews "mojra/domain/player/views"
	"mojra/domain/shell/studio_app/state"
	studioviews "mojra/domain/studio/views"
	"mojra/engine"
	"mojra/engine/templates"
)

const (
	themeLight   = "light"
	themeDark    = "dark"
	uiScaleSmall = "small"
	uiScaleLarge = "large"
	previewFull  = "full"
	sensHigh     = "high"
	sensMedium   = "medium"
	sensLow      = "low"
	sensOff      = "off"
)

type shellTemplate struct {
	IsDocumentRoot       bool
	Theme                string
	IsDark               bool
	ThemeToggleLabel     string
	ImagelessMode        bool
	Imageless            bool
	ImagelessLocked      bool
	ImagelessLockedAttr  string
	ImagelessAttr        string
	ImagelessToggleLabel string
	UIScale              string
	IsSmallUI            bool
	IsLargeUI            bool
	AutoChoiceDelay      float64
	IsAutoDelay1         bool
	IsAutoDelay2         bool
	IsAutoDelay3         bool
	IsAutoDelay5         bool
	ScrollSensitivity    string
	IsScrollSensHigh     bool
	IsScrollSensMedium   bool
	IsScrollSensLow      bool
	IsScrollSensOff      bool
	StandaloneMode       bool
	DevMode              bool
	PreviewMode          string
	ActiveBookTitle      string
	ZenModeAttr          string
	Topbar               templates.TemplateChunk
	WorkspaceView        templates.TemplateChunk
	ActiveModals         []templates.TemplateChunk
	UI                   templates.TemplateChunk
}

// LangOf returns the active language for s.
func LangOf(s *state.AppState) engine.Lang {
	if s == nil {
		return engine.LangPL
	}
	return commonviews.LangOf(s.UIState)
}

func tr(s *state.AppState, key engine.TrKey) string {
	return commonviews.Tr(s.UIState, key)
}

func trf(s *state.AppState, key engine.TrKey, args ...any) string {
	return commonviews.Trf(s.UIState, key, args...)
}

// ActiveWorkspaceView returns the TemplateChunk for the active workspace view.
func ActiveWorkspaceView(s *state.AppState) templates.TemplateChunk {
	switch state.ActiveWorkspaceView(s) {
	case state.ViewEditor:
		if !s.License.IsActive() || s.License.ShowActivatedSummary {
			return licenseviews.LicenseView(&s.License)
		}
		return studioviews.StudioView(&s.Editor)
	case state.ViewPlayer:
		return playerviews.PlayerView(&s.Player)
	default:
		return libraryviews.LibraryView(&s.Library)
	}
}

// ActiveOverlayViews returns the ordered list of active overlay TemplateChunks for #modals-root.
func ActiveOverlayViews(s *state.AppState) []templates.TemplateChunk {
	var modals []templates.TemplateChunk
	if s.Overlays.Settings {
		modals = append(modals, commonviews.SettingsView(s.UIState))
	}
	if s.License.IsActive() && s.NewBook.Open {
		modals = append(modals, studioviews.NewBookView(&s.Editor))
	}
	if s.License.IsActive() && s.Export.Open {
		modals = append(modals, studioviews.ExportView(&s.Editor))
	}
	if s.Overlays.AddBook {
		modals = append(modals, libraryviews.AddBookView(&s.Library))
	}
	if s.License.IsActive() && s.Overlays.StudioProjects {
		modals = append(modals, studioviews.StudioProjectsView(&s.Editor))
	}
	if s.License.IsActive() && s.Editor.SceneText.IsOpen() {
		if m := studioviews.TextEditorView(&s.Editor); m.TemplateData != nil {
			modals = append(modals, m)
		}
	}
	if s.Confirm.IsOpen() {
		modals = append(modals, commonviews.ConfirmView(s.UIState))
	}
	return modals
}

func buildShellChunk(s *state.AppState, isDocumentRoot bool) templates.TemplateChunk {
	uiChunk := commonviews.UIView(s.UIState)
	theme := s.Settings.Theme
	if theme != themeLight {
		theme = themeDark
	}
	themeToggleLabel := tr(s, i18n.TrThemeLightLabel)
	if theme == themeLight {
		themeToggleLabel = tr(s, i18n.TrThemeDarkLabel)
	}

	ctxBook := state.ContextBook(s)
	imagelessLocked := ctxBook.IsImageless()
	effectiveImageless := ctxBook.IsEffectiveImageless(s.Settings)
	imagelessLockedAttr := attrFalse
	if imagelessLocked {
		imagelessLockedAttr = attrTrue
	}
	imagelessAttr := attrFalse
	imagelessToggleLabel := tr(s, i18n.TrImagelessOnLabel)
	if effectiveImageless {
		imagelessAttr = attrTrue
		if imagelessLocked {
			imagelessToggleLabel = tr(s, i18n.TrImagelessLockedLabel)
		} else {
			imagelessToggleLabel = tr(s, i18n.TrImagelessOffLabel)
		}
	}

	uiScale := s.Settings.UIScale
	if uiScale != uiScaleLarge {
		uiScale = uiScaleSmall
	}
	autoDelay := book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay)
	scrollSens := book.NormalizeScrollSensitivity(s.Settings.ScrollSensitivity)

	previewMode := previewFull
	if s.Config.DevMode && s.Settings.PreviewMode != "" {
		previewMode = s.Settings.PreviewMode
	}

	topbarChunk := TopbarView(s)
	topbarData, _ := topbarChunk.TemplateData.(*topbarTemplate)
	activeBookTitle := tr(s, i18n.TrNavLibrary)
	zenModeAttr := attrFalse
	if topbarData != nil {
		activeBookTitle = topbarData.ActiveBookTitle
		zenModeAttr = topbarData.ZenModeAttr
	}

	view := &shellTemplate{
		IsDocumentRoot:       isDocumentRoot,
		Theme:                theme,
		IsDark:               theme != themeLight,
		ThemeToggleLabel:     themeToggleLabel,
		ImagelessMode:        effectiveImageless,
		Imageless:            effectiveImageless,
		ImagelessLocked:      imagelessLocked,
		ImagelessLockedAttr:  imagelessLockedAttr,
		ImagelessAttr:        imagelessAttr,
		ImagelessToggleLabel: imagelessToggleLabel,
		UIScale:              uiScale,
		IsSmallUI:            uiScale == uiScaleSmall,
		IsLargeUI:            uiScale == uiScaleLarge,
		AutoChoiceDelay:      autoDelay,
		IsAutoDelay1:         autoDelay <= 1.25,
		IsAutoDelay2:         autoDelay > 1.25 && autoDelay <= 2.5,
		IsAutoDelay3:         autoDelay > 2.5 && autoDelay <= 4.0,
		IsAutoDelay5:         autoDelay > 4.0,
		ScrollSensitivity:    scrollSens,
		IsScrollSensHigh:     scrollSens == sensHigh,
		IsScrollSensMedium:   scrollSens == sensMedium,
		IsScrollSensLow:      scrollSens == sensLow,
		IsScrollSensOff:      scrollSens == sensOff,
		StandaloneMode:       s.Config.StandaloneMode,
		DevMode:              s.Config.DevMode,
		PreviewMode:          previewMode,
		ActiveBookTitle:      activeBookTitle,
		ZenModeAttr:          zenModeAttr,
		Topbar:               topbarChunk,
		WorkspaceView:        ActiveWorkspaceView(s),
		ActiveModals:         ActiveOverlayViews(s),
		UI:                   uiChunk,
	}
	return templates.TemplateChunk{
		TemplateData: view,
		ScriptData:   uiChunk.ScriptData,
	}
}

// ShellView renders the inner #app layout chunk (IsDocumentRoot=false).
func ShellView(s *state.AppState) templates.TemplateChunk {
	return buildShellChunk(s, false)
}

// ShellDocumentView renders the complete HTML document chunk (IsDocumentRoot=true) for GET /.
func ShellDocumentView(s *state.AppState) templates.TemplateChunk {
	return buildShellChunk(s, true)
}

// AppShellScriptView returns the script-only UIController chunk for global state hydration.
func AppShellScriptView(s *state.AppState) templates.TemplateChunk {
	return commonviews.UIScriptView(s.UIState)
}
