package views

import (
	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
	"mojra/internal/tpl"
)

var prewarmedLangRoots [i18n.LangCount]*tpl.RenderContext

func init() {
	for l := i18n.Lang(0); l < i18n.LangCount; l++ {
		vars := make(map[string]any, len(i18n.I18nDicts[l])+4)
		i18n.PopulateI18nContext(vars, l)
		prewarmedLangRoots[l] = tpl.NewRenderContext(vars)
	}
}

// LangOf returns the active UI language from AppState.
func LangOf(s *state.AppState) i18n.Lang {
	if s == nil || s.Settings == nil {
		return i18n.LangPL
	}
	return i18n.ParseLang(s.Settings.Language)
}

func tr(s *state.AppState, k i18n.TrKey) string {
	return i18n.T(LangOf(s), k)
}

func trf(s *state.AppState, k i18n.TrKey, args ...any) string {
	return i18n.Tf(LangOf(s), k, args...)
}

// NewViewContext creates a RenderContext with pre-warmed i18n bindings as parent
// and the view data (either a *Struct or a map[string]any) as child overlay.
func NewViewContext(lang i18n.Lang, data any) *tpl.RenderContext {
	if lang >= i18n.LangCount {
		lang = i18n.LangPL
	}
	base := prewarmedLangRoots[lang]
	if m, ok := data.(map[string]any); ok {
		return base.WithOverlay(m)
	}
	return base.WithStruct(data)
}

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
	Topbar               tpl.TemplateChunk
	WorkspaceView        tpl.TemplateChunk
	ActiveModals         []tpl.TemplateChunk
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
	}
}

// ViewAppShellScript returns a script-only TemplateChunk hydrating AppShellController.data on partial updates.
func ViewAppShellScript(s *state.AppState) tpl.TemplateChunk {
	return tpl.TemplateChunk{
		ScriptData: buildAppShellScript(s),
	}
}

// ViewActiveWorkspaceChunk returns the TemplateChunk for the active workspace view.
func ViewActiveWorkspaceChunk(s *state.AppState) tpl.TemplateChunk {
	switch state.ActiveWorkspaceView(s) {
	case state.ViewEditor:
		return ViewStudio(s)
	case state.ViewPlayer:
		return ViewPlayer(s)
	default:
		return ViewLibrary(s)
	}
}

// ViewActiveModalChunks returns the ordered list of active modal TemplateChunks for #modals-root.
func ViewActiveModalChunks(s *state.AppState) []tpl.TemplateChunk {
	var modals []tpl.TemplateChunk
	if s.Editor.RightPanel == state.PanelHelp {
		modals = append(modals, ViewModalEditorHelp(s))
	}
	if s.Modals.QR {
		modals = append(modals, ViewModalQR(s))
	}
	if s.ModalNewBook.Open {
		modals = append(modals, ViewModalNewBook(s))
	}
	if s.ModalExport.Open {
		modals = append(modals, ViewModalExport(s))
	}
	if s.Modals.AddBook {
		modals = append(modals, ViewModalAddBook(s))
	}
	if s.Modals.StudioProjects {
		modals = append(modals, ViewModalStudioProjects(s))
	}
	if s.Editor.SceneText.IsOpen() {
		if m := ViewTextEditor(s); m.TemplateData != nil {
			modals = append(modals, m)
		}
	}
	if s.ModalConfirm.IsOpen() {
		modals = append(modals, ViewModalConfirm(s))
	}
	return modals
}

func ViewAppShell(s *state.AppState) tpl.TemplateChunk {
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

	topbarChunk := ViewTopbar(s)
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
		WorkspaceView:        ViewActiveWorkspaceChunk(s),
		ActiveModals:         ViewActiveModalChunks(s),
	}
	return tpl.TemplateChunk{
		TemplateData: view,
		ScriptData:   script,
	}
}
