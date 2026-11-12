package views

import (
	"fmt"
	"net"

	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

func resolveLANShareURL(port int) string {
	if port <= 0 {
		port = 8484
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return fmt.Sprintf("http://127.0.0.1:%d", port)
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
		return fmt.Sprintf("http://%s:%d", ip4.String(), port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

type modalQRView struct {
	ShowQRModal          bool    `tpl:"show_qr_modal"`
	IsDark               bool    `tpl:"is_dark"`
	ThemeToggleLabel     string  `tpl:"theme_toggle_label"`
	Imageless            bool    `tpl:"imageless"`
	ImagelessLocked      bool    `tpl:"imageless_locked"`
	ImagelessToggleLabel string  `tpl:"imageless_toggle_label"`
	IsSmallUI            bool    `tpl:"is_small_ui"`
	IsLargeUI            bool    `tpl:"is_large_ui"`
	IsScrollSensHigh     bool    `tpl:"is_scroll_sens_high"`
	IsScrollSensMedium   bool    `tpl:"is_scroll_sens_medium"`
	IsScrollSensLow      bool    `tpl:"is_scroll_sens_low"`
	IsScrollSensOff      bool    `tpl:"is_scroll_sens_off"`
	AutoChoiceDelay      float64 `tpl:"auto_choice_delay"`
	IsAutoDelay1         bool    `tpl:"is_auto_delay_1"`
	IsAutoDelay2         bool    `tpl:"is_auto_delay_2"`
	IsAutoDelay3         bool    `tpl:"is_auto_delay_3"`
	IsAutoDelay5         bool    `tpl:"is_auto_delay_5"`
	DevMode              bool    `tpl:"dev_mode"`
	LANURL               string  `tpl:"lan_url"`
}

func ViewModalQR(s *state.AppState) *modalQRView {
	theme := s.Settings.Theme
	if theme != "light" {
		theme = "dark"
	}
	themeLbl := tr(s, i18n.TrThemeLightLabel)
	if theme == "light" {
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
	if uiScale != "large" {
		uiScale = "small"
	}
	scrollSens := book.NormalizeScrollSensitivity(s.Settings.ScrollSensitivity)
	autoDelay := book.NormalizeAutoChoiceDelay(s.Settings.AutoChoiceDelay)

	lanURL := ""
	if s.Modals.QR && s.Config.DevMode && !s.Config.StandaloneMode {
		lanURL = resolveLANShareURL(s.Config.Port)
	}

	return &modalQRView{
		ShowQRModal:          s.Modals.QR,
		IsDark:               theme != "light",
		ThemeToggleLabel:     themeLbl,
		Imageless:            eff,
		ImagelessLocked:      locked,
		ImagelessToggleLabel: imagelessLbl,
		IsSmallUI:            uiScale == "small",
		IsLargeUI:            uiScale == "large",
		IsScrollSensHigh:     scrollSens == "high",
		IsScrollSensMedium:   scrollSens == "medium",
		IsScrollSensLow:      scrollSens == "low",
		IsScrollSensOff:      scrollSens == "off",
		AutoChoiceDelay:      autoDelay,
		IsAutoDelay1:         autoDelay <= 1.25,
		IsAutoDelay2:         autoDelay > 1.25 && autoDelay <= 2.5,
		IsAutoDelay3:         autoDelay > 2.5 && autoDelay <= 4.0,
		IsAutoDelay5:         autoDelay > 4.0,
		DevMode:              s.Config.DevMode,
		LANURL:               lanURL,
	}
}
