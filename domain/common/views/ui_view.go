package views

import (
	book "mojra/domain/book/actions"
	"mojra/domain/common/state"
	"mojra/domain/i18n"
	"mojra/engine"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

const (
	defaultPreviewMode = "full"
)

type uiTemplate struct {
	HasToast bool
}

type uiScript struct {
	theme           string
	uiScale         string
	imageless       bool
	imagelessLocked bool
	zenMode         bool
	sensitivity     string
	deviceMode      string
	lang            string
	toast           string
	nativeCmd       string
}

// LangOf resolves the active engine.Lang for ui.
func LangOf(ui *state.UIState) engine.Lang {
	if ui == nil || ui.Settings == nil {
		return engine.LangPL
	}
	return engine.ParseLang(ui.Settings.Language)
}

// Tr translates key using the active language of ui.
func Tr(ui *state.UIState, key engine.TrKey) string {
	return engine.T(LangOf(ui), key)
}

// Trf translates and formats key with args using the active language of ui.
func Trf(ui *state.UIState, key engine.TrKey, args ...any) string {
	return engine.Tf(LangOf(ui), key, args...)
}

// BuildUIScript constructs the uiScript payload for UIController hydration.
func BuildUIScript(ui *state.UIState) uiScript {
	langCode := book.NormalizeLanguage(ui.Settings.Language)
	devModeStr := ""
	if ui.Config.DevMode {
		devModeStr = ui.Settings.PreviewMode
		if devModeStr == "" {
			devModeStr = defaultPreviewMode
		}
	}
	return uiScript{
		theme:           ui.Settings.Theme,
		uiScale:         ui.Settings.UIScale,
		imageless:       ui.EffectiveImageless(),
		imagelessLocked: ui.BookLockedImageless(),
		zenMode:         ui.ZenModeActive(),
		sensitivity:     book.NormalizeScrollSensitivity(ui.Settings.ScrollSensitivity),
		deviceMode:      devModeStr,
		lang:            langCode,
		toast:           ui.Overlays.Toast,
		nativeCmd:       ui.Overlays.NativeCmd,
	}
}

// UIView returns the TemplateChunk for ui_view.tpl and UIController.
func UIView(ui *state.UIState) templates.TemplateChunk {
	scriptData := BuildUIScript(ui)
	return templates.TemplateChunk{
		TemplateData: uiTemplate{HasToast: scriptData.toast != ""},
		ScriptData:   &scriptData,
	}
}

// UIScriptView returns a script-only TemplateChunk for UIController hydration without DOM replacement.
func UIScriptView(ui *state.UIState) templates.TemplateChunk {
	scriptData := BuildUIScript(ui)
	return templates.TemplateChunk{
		ScriptData: &scriptData,
	}
}

// ToastFrames builds a single script-only ActionFrame carrying toast feedback to UIController.
func ToastFrames(ui *state.UIState, toast string) []transport.ActionFrame {
	if toast != "" {
		ui.Overlays.Toast = toast
	}
	return []transport.ActionFrame{
		{Target: state.TargetNone, Chunks: []templates.TemplateChunk{UIScriptView(ui)}},
	}
}

// BuildBookModeOptions builds the select options and active label for book vs screenplay mode.
func BuildBookModeOptions(activeMode string, lang engine.Lang) (string, []book.SelectOption) {
	norm := book.NormalizeBookMode(activeMode)
	bookLbl := engine.T(lang, i18n.TrModeInteractiveBook)
	spLbl := engine.T(lang, i18n.TrModeScreenplay)
	label := bookLbl
	if norm == book.BookModeScreenplay {
		label = spLbl
	}
	return label, []book.SelectOption{
		{Value: book.BookModeBook, Label: bookLbl, IsActive: norm == book.BookModeBook},
		{Value: book.BookModeScreenplay, Label: spLbl, IsActive: norm == book.BookModeScreenplay},
	}
}

// BuildScriptStandardOptions builds the select options and active label for PL/EU/US screenplay standards.
func BuildScriptStandardOptions(activeStd string, lang engine.Lang) (string, []book.SelectOption) {
	norm := book.NormalizeScriptStandard(activeStd)
	plLbl := engine.T(lang, i18n.TrStdLabelPL)
	euLbl := engine.T(lang, i18n.TrStdLabelEU)
	usLbl := engine.T(lang, i18n.TrStdLabelUS)
	label := plLbl
	switch norm {
	case book.ScriptStandardEU:
		label = euLbl
	case book.ScriptStandardUS:
		label = usLbl
	}
	return label, []book.SelectOption{
		{Value: book.ScriptStandardPL, Label: plLbl, IsActive: norm == book.ScriptStandardPL},
		{Value: book.ScriptStandardEU, Label: euLbl, IsActive: norm == book.ScriptStandardEU},
		{Value: book.ScriptStandardUS, Label: usLbl, IsActive: norm == book.ScriptStandardUS},
	}
}
