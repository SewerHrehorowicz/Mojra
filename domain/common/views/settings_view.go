//go:build !(js && wasm)

package views

import (
	"fmt"
	"net"

	book "mojra/domain/book/actions"
	"mojra/domain/common/state"
	"mojra/domain/i18n"
	"mojra/engine/templates"
)

const (
	loopbackShareURLFmt = "http://127.0.0.1:%d"
	lanShareURLFmt      = "http://%s:%d"
	themeLight          = "light"
	themeDark           = "dark"
	uiScaleSmall        = "small"
	uiScaleLarge        = "large"
	sensHigh            = "high"
	sensMedium          = "medium"
	sensLow             = "low"
	sensOff             = "off"
)

func resolveLANShareURL(port int) string {
	if port <= 0 {
		port = 8484
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return fmt.Sprintf(loopbackShareURLFmt, port)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil || ip4.IsLinkLocalUnicast() {
			continue
		}
		return fmt.Sprintf(lanShareURLFmt, ip4.String(), port)
	}
	return fmt.Sprintf(loopbackShareURLFmt, port)
}

type settingsTemplate struct {
	ShowSettingsModal    bool
	IsDark               bool
	ThemeToggleLabel     string
	Imageless            bool
	ImagelessLocked      bool
	ImagelessToggleLabel string
	IsSmallUI            bool
	IsLargeUI            bool
	IsScrollSensHigh     bool
	IsScrollSensMedium   bool
	IsScrollSensLow      bool
	IsScrollSensOff      bool
	AutoChoiceDelay      float64
	IsAutoDelay1         bool
	IsAutoDelay2         bool
	IsAutoDelay3         bool
	IsAutoDelay5         bool
	DevMode              bool
	LANURL               string
	ExtraSections        []templates.TemplateChunk
}

type settingsScript struct {
	theme           string
	uiScale         string
	imageless       bool
	imagelessLocked bool
	sensitivity     string
	autoChoiceDelay float64
}

// SettingsView builds the TemplateChunk for the global application settings modal (#settings-view).
func SettingsView(ui *state.UIState) templates.TemplateChunk {
	theme := ui.Settings.Theme
	if theme != themeLight {
		theme = themeDark
	}
	themeLbl := Tr(ui, i18n.TrThemeLightLabel)
	if theme == themeLight {
		themeLbl = Tr(ui, i18n.TrThemeDarkLabel)
	}

	var ctxBook *book.Book
	if ui.Hooks.ActiveContextBook != nil {
		ctxBook = ui.Hooks.ActiveContextBook()
	}
	locked := ctxBook.IsImageless()
	eff := ctxBook.IsEffectiveImageless(ui.Settings)

	var imagelessLbl string
	if locked {
		imagelessLbl = Tr(ui, i18n.TrImagelessLockedLabel)
	} else if eff {
		imagelessLbl = Tr(ui, i18n.TrImagelessOffLabel)
	} else {
		imagelessLbl = Tr(ui, i18n.TrImagelessOnLabel)
	}

	uiScale := ui.Settings.UIScale
	if uiScale != uiScaleLarge {
		uiScale = uiScaleSmall
	}
	scrollSens := book.NormalizeScrollSensitivity(ui.Settings.ScrollSensitivity)
	autoDelay := book.NormalizeAutoChoiceDelay(ui.Settings.AutoChoiceDelay)

	lanURL := ""
	if ui.Overlays.Settings && ui.Config.DevMode && !ui.Config.StandaloneMode {
		lanURL = resolveLANShareURL(ui.Config.Port)
	}

	var extraSections []templates.TemplateChunk
	if ui.Hooks.SettingsExtraSections != nil {
		extraSections = ui.Hooks.SettingsExtraSections()
	}

	return templates.TemplateChunk{
		TemplateData: &settingsTemplate{
			ShowSettingsModal:    ui.Overlays.Settings,
			IsDark:               theme != themeLight,
			ThemeToggleLabel:     themeLbl,
			Imageless:            eff,
			ImagelessLocked:      locked,
			ImagelessToggleLabel: imagelessLbl,
			IsSmallUI:            uiScale == uiScaleSmall,
			IsLargeUI:            uiScale == uiScaleLarge,
			IsScrollSensHigh:     scrollSens == sensHigh,
			IsScrollSensMedium:   scrollSens == sensMedium,
			IsScrollSensLow:      scrollSens == sensLow,
			IsScrollSensOff:      scrollSens == sensOff,
			AutoChoiceDelay:      autoDelay,
			IsAutoDelay1:         autoDelay <= 1.25,
			IsAutoDelay2:         autoDelay > 1.25 && autoDelay <= 2.5,
			IsAutoDelay3:         autoDelay > 2.5 && autoDelay <= 4.0,
			IsAutoDelay5:         autoDelay > 4.0,
			DevMode:              ui.Config.DevMode,
			LANURL:               lanURL,
			ExtraSections:        extraSections,
		},
		ScriptData: &settingsScript{
			theme:           theme,
			uiScale:         uiScale,
			imageless:       eff,
			imagelessLocked: locked,
			sensitivity:     scrollSens,
			autoChoiceDelay: autoDelay,
		},
	}
}
