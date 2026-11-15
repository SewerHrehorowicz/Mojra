package views

import (
	"fmt"
	"net"

	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine/templates"
)

const (
	loopbackShareURLFmt = "http://127.0.0.1:%d"
	lanShareURLFmt      = "http://%s:%d"
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

type qrTemplate struct {
	ShowQRModal          bool
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
}

type qRScript struct {
	theme           string
	uiScale         string
	imageless       bool
	imagelessLocked bool
	sensitivity     string
	autoChoiceDelay float64
}

func QRView(s *state.AppState) templates.TemplateChunk {
	theme := s.Settings.Theme
	if theme != themeLight {
		theme = themeDark
	}
	themeLbl := tr(s, i18n.TrThemeLightLabel)
	if theme == themeLight {
		themeLbl = tr(s, i18n.TrThemeDarkLabel)
	}

	ctxBook := contextBookForTopbar(s)
	locked := ctxBook.IsImageless()
	eff := ctxBook.IsEffectiveImageless(s.Settings)

	var imagelessLbl string
	if locked {
		imagelessLbl = tr(s, i18n.TrImagelessLockedLabel)
	} else if eff {
		imagelessLbl = tr(s, i18n.TrImagelessOffLabel)
	} else {
		imagelessLbl = tr(s, i18n.TrImagelessOnLabel)
	}

	uiScale := s.Settings.UIScale
	if uiScale != uiScaleLarge {
		uiScale = uiScaleSmall
	}
	scrollSens := book.NormalizeScrollSensitivity(s.Settings.ScrollSensitivity)
	autoDelay := book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay)

	lanURL := ""
	if s.Overlays.QR && s.Config.DevMode && !s.Config.StandaloneMode {
		lanURL = resolveLANShareURL(s.Config.Port)
	}

	return templates.TemplateChunk{
		TemplateData: &qrTemplate{
			ShowQRModal:          s.Overlays.QR,
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
			DevMode:              s.Config.DevMode,
			LANURL:               lanURL,
		},
		ScriptData: &qRScript{
			theme:           theme,
			uiScale:         uiScale,
			imageless:       eff,
			imagelessLocked: locked,
			sensitivity:     scrollSens,
			autoChoiceDelay: autoDelay,
		},
	}
}
