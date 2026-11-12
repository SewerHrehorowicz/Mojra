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

type appShellView struct {
	Theme                string              `tpl:"theme"`
	IsDark               bool                `tpl:"is_dark"`
	ThemeToggleLabel     string              `tpl:"theme_toggle_label"`
	ImagelessMode        bool                `tpl:"imageless_mode"`
	Imageless            bool                `tpl:"imageless"`
	ImagelessLocked      bool                `tpl:"imageless_locked"`
	ImagelessLockedAttr  string              `tpl:"imageless_locked_attr"`
	ImagelessAttr        string              `tpl:"imageless_attr"`
	ImagelessToggleLabel string              `tpl:"imageless_toggle_label"`
	UIScale              string              `tpl:"ui_scale"`
	IsSmallUI            bool                `tpl:"is_small_ui"`
	IsLargeUI            bool                `tpl:"is_large_ui"`
	AutoChoiceDelay      float64             `tpl:"auto_choice_delay"`
	IsAutoDelay1         bool                `tpl:"is_auto_delay_1"`
	IsAutoDelay2         bool                `tpl:"is_auto_delay_2"`
	IsAutoDelay3         bool                `tpl:"is_auto_delay_3"`
	IsAutoDelay5         bool                `tpl:"is_auto_delay_5"`
	ScrollSensitivity    string              `tpl:"scroll_sensitivity"`
	IsScrollSensHigh     bool                `tpl:"is_scroll_sens_high"`
	IsScrollSensMedium   bool                `tpl:"is_scroll_sens_medium"`
	IsScrollSensLow      bool                `tpl:"is_scroll_sens_low"`
	IsScrollSensOff      bool                `tpl:"is_scroll_sens_off"`
	StandaloneMode       bool                `tpl:"standalone_mode"`
	DevMode              bool                `tpl:"dev_mode"`
	PreviewMode          string              `tpl:"preview_mode"`
	Topbar               topbarView          `tpl:"*"`
	WorkspaceView        tpl.TemplateChunk   `tpl:"workspace_view"`
	ActiveModals         []tpl.TemplateChunk `tpl:"active_modals"`
}

// ViewActiveWorkspaceChunk returns the TemplateChunk for the active workspace view.
func ViewActiveWorkspaceChunk(s *state.AppState) tpl.TemplateChunk {
	switch state.ActiveWorkspaceView(s) {
	case state.ViewEditor:
		return tpl.TemplateChunk{Template: "editor_view", Data: ViewEditor(s)}
	case state.ViewPlayer:
		return tpl.TemplateChunk{Template: "player_view", Data: ViewPlayer(s)}
	default:
		return tpl.TemplateChunk{Template: "library_view", Data: ViewLibrary(s)}
	}
}

// ViewActiveModalChunks returns the ordered list of active modal TemplateChunks for #modals-root.
func ViewActiveModalChunks(s *state.AppState) []tpl.TemplateChunk {
	var modals []tpl.TemplateChunk
	if s.Editor.RightPanel == state.PanelHelp {
		modals = append(modals, tpl.TemplateChunk{Template: "modal_editor_help", Data: ViewModalEditorHelp(s)})
	}
	if s.Modals.QR {
		modals = append(modals, tpl.TemplateChunk{Template: "modal_qr", Data: ViewModalQR(s)})
	}
	if s.ModalNewBook.Open {
		modals = append(modals, tpl.TemplateChunk{Template: "modal_new_book", Data: ViewModalNewBook(s)})
	}
	if s.ModalExport.Open {
		modals = append(modals, tpl.TemplateChunk{Template: "modal_export", Data: ViewModalExport(s)})
	}
	if s.Modals.AddBook {
		modals = append(modals, tpl.TemplateChunk{Template: "modal_add_book", Data: ViewModalAddBook(s)})
	}
	if s.Modals.StudioProjects {
		modals = append(modals, tpl.TemplateChunk{Template: "modal_studio_projects", Data: ViewModalStudioProjects(s)})
	}
	if s.Editor.SceneText.IsOpen() && state.ActiveWorkspaceView(s) == state.ViewEditor {
		if m := ViewEditorSceneText(s); m != nil {
			modals = append(modals, tpl.TemplateChunk{Template: "modal_scene_text", Data: m})
		}
	}
	if s.ModalConfirm.IsOpen() {
		modals = append(modals, tpl.TemplateChunk{Template: "confirm_modal", Data: ViewModalConfirm(s)})
	}
	return modals
}

func ViewAppShell(s *state.AppState) *appShellView {
	theme := s.Settings.Theme
	if theme != "light" {
		theme = "dark"
	}
	themeToggleLabel := tr(s, i18n.TrThemeLightLabel)
	if theme == "light" {
		themeToggleLabel = tr(s, i18n.TrThemeDarkLabel)
	}

	ctxBook := contextBookForTopbar(s)
	imagelessLocked := ctxBook.IsImageless()
	effectiveImageless := ctxBook.IsEffectiveImageless(s.Settings)
	imagelessLockedAttr := "false"
	if imagelessLocked {
		imagelessLockedAttr = "true"
	}
	imagelessAttr := "false"
	imagelessToggleLabel := tr(s, i18n.TrImagelessOnLabel)
	if effectiveImageless {
		imagelessAttr = "true"
		if imagelessLocked {
			imagelessToggleLabel = tr(s, i18n.TrImagelessLockedLabel)
		} else {
			imagelessToggleLabel = tr(s, i18n.TrImagelessOffLabel)
		}
	}

	uiScale := s.Settings.UIScale
	if uiScale != "large" {
		uiScale = "small"
	}

	autoDelay := book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay)
	scrollSens := book.NormalizeScrollSensitivity(s.Settings.ScrollSensitivity)

	previewMode := "full"
	if s.Config.DevMode {
		pm := s.Settings.PreviewMode
		if pm == "portrait" || pm == "tablet" || pm == "landscape" {
			previewMode = pm
		}
	}

	return &appShellView{
		Theme:                theme,
		IsDark:               theme != "light",
		ThemeToggleLabel:     themeToggleLabel,
		ImagelessMode:        effectiveImageless,
		Imageless:            effectiveImageless,
		ImagelessLocked:      imagelessLocked,
		ImagelessLockedAttr:  imagelessLockedAttr,
		ImagelessAttr:        imagelessAttr,
		ImagelessToggleLabel: imagelessToggleLabel,
		UIScale:              uiScale,
		IsSmallUI:            uiScale == "small",
		IsLargeUI:            uiScale == "large",
		AutoChoiceDelay:      autoDelay,
		IsAutoDelay1:         autoDelay <= 1.25,
		IsAutoDelay2:         autoDelay > 1.25 && autoDelay <= 2.5,
		IsAutoDelay3:         autoDelay > 2.5 && autoDelay <= 4.0,
		IsAutoDelay5:         autoDelay > 4.0,
		ScrollSensitivity:    scrollSens,
		IsScrollSensHigh:     scrollSens == "high",
		IsScrollSensMedium:   scrollSens == "medium",
		IsScrollSensLow:      scrollSens == "low",
		IsScrollSensOff:      scrollSens == "off",
		StandaloneMode:       s.Config.StandaloneMode,
		DevMode:              s.Config.DevMode,
		PreviewMode:          previewMode,
		Topbar:               viewTopbar(s),
		WorkspaceView:        ViewActiveWorkspaceChunk(s),
		ActiveModals:         ViewActiveModalChunks(s),
	}
}
