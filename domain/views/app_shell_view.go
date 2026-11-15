package views

import (
	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine"
	"mojra/engine/templates"
)

const (
	themeLight       = "light"
	themeDark        = "dark"
	uiScaleLarge     = "large"
	uiScaleSmall     = "small"
	sensHigh         = "high"
	sensMedium       = "medium"
	sensLow          = "low"
	sensOff          = "off"
	previewFull      = "full"
	previewPortrait  = "portrait"
	previewTablet    = "tablet"
	previewLandscape = "landscape"
)

// LangOf returns the active UI language from AppState.
func LangOf(s *state.AppState) engine.Lang {
	if s == nil || s.Settings == nil {
		return engine.DefaultLang
	}
	return engine.ParseLang(s.Settings.Language)
}

func tr(s *state.AppState, k engine.TrKey) string {
	return engine.T(LangOf(s), k)
}

func trf(s *state.AppState, k engine.TrKey, args ...any) string {
	return engine.Tf(LangOf(s), k, args...)
}

type appShellTemplate struct {
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
}

type appShellScript struct {
	theme           string
	lang            string
	uiScale         string
	imageless       bool
	imagelessLocked bool
	zenMode         bool
	sensitivity     string
	deviceMode      string
	toast           string
	nativeCmd       string
}

func buildAppShellScript(s *state.AppState) *appShellScript {
	theme := s.Settings.Theme
	if theme != themeLight {
		theme = themeDark
	}
	ctxBook := contextBookForTopbar(s)
	uiScale := s.Settings.UIScale
	if uiScale != uiScaleLarge {
		uiScale = uiScaleSmall
	}
	devPreview := ""
	if s.Config.DevMode {
		pm := s.Settings.PreviewMode
		if pm == previewPortrait || pm == previewTablet || pm == previewLandscape {
			devPreview = pm
		} else {
			devPreview = previewFull
		}
	}
	return &appShellScript{
		theme:           theme,
		lang:            book.NormalizeLanguage(s.Settings.Language),
		uiScale:         uiScale,
		imageless:       ctxBook.IsEffectiveImageless(s.Settings),
		imagelessLocked: ctxBook.IsImageless(),
		zenMode:         s.Player.IsEffectiveZenMode(state.ActiveWorkspaceView(s)),
		sensitivity:     book.NormalizeScrollSensitivity(s.Settings.ScrollSensitivity),
		deviceMode:      devPreview,
		toast:           s.Overlays.Toast,
		nativeCmd:       s.Overlays.NativeCmd,
	}
}

// AppShellScriptView returns a script-only TemplateChunk hydrating AppShellController.data on partial updates.
func AppShellScriptView(s *state.AppState) templates.TemplateChunk {
	return templates.TemplateChunk{
		ScriptData: buildAppShellScript(s),
	}
}

// ActiveWorkspaceView returns the TemplateChunk for the active workspace view.
func ActiveWorkspaceView(s *state.AppState) templates.TemplateChunk {
	switch state.ActiveWorkspaceView(s) {
	case state.ViewEditor:
		return StudioView(s)
	case state.ViewPlayer:
		return PlayerView(s)
	default:
		return LibraryView(s)
	}
}

// ActiveOverlayViews returns the ordered list of active overlay TemplateChunks for #modals-root.
func ActiveOverlayViews(s *state.AppState) []templates.TemplateChunk {
	var modals []templates.TemplateChunk
	if s.Editor.RightPanel == state.PanelHelp {
		modals = append(modals, EditorHelpView(s))
	}
	if s.Overlays.QR {
		modals = append(modals, QRView(s))
	}
	if s.NewBook.Open {
		modals = append(modals, NewBookView(s))
	}
	if s.Export.Open {
		modals = append(modals, ExportView(s))
	}
	if s.Overlays.AddBook {
		modals = append(modals, AddBookView(s))
	}
	if s.Overlays.StudioProjects {
		modals = append(modals, StudioProjectsView(s))
	}
	if s.Editor.SceneText.IsOpen() {
		if m := TextEditorView(s); m.TemplateData != nil {
			modals = append(modals, m)
		}
	}
	if s.Confirm.IsOpen() {
		modals = append(modals, ConfirmView(s))
	}
	return modals
}

func AppShellView(s *state.AppState) templates.TemplateChunk {
	script := buildAppShellScript(s)
	theme := script.theme
	themeToggleLabel := tr(s, i18n.TrThemeLightLabel)
	if theme == themeLight {
		themeToggleLabel = tr(s, i18n.TrThemeDarkLabel)
	}

	imagelessLocked := script.imagelessLocked
	effectiveImageless := script.imageless
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

	uiScale := script.uiScale
	autoDelay := book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay)
	scrollSens := script.sensitivity

	previewMode := previewFull
	if script.deviceMode != "" {
		previewMode = script.deviceMode
	}

	topbarChunk := TopbarView(s)
	topbarData, _ := topbarChunk.TemplateData.(*topbarTemplate)
	activeBookTitle := tr(s, i18n.TrNavLibrary)
	zenModeAttr := attrFalse
	if topbarData != nil {
		activeBookTitle = topbarData.ActiveBookTitle
		zenModeAttr = topbarData.ZenModeAttr
	}

	view := &appShellTemplate{
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
	}
	return templates.TemplateChunk{
		TemplateData: view,
		ScriptData:   script,
	}
}
