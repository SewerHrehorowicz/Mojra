//go:build !(js && wasm)

package views

import (
	"strings"
	"time"

	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/license/state"
	"mojra/engine/templates"
)

const dateFormatISO = "2006-01-02"

type licenseTemplate struct {
	ErrorMsg    string
	HolderName  string
	Email       string
	Tier        string
	ExpiresText string
	IsActive    bool
	HasError    bool
}

// LicenseView builds the TemplateChunk for #license-view.
func LicenseView(licState *state.LicenseState) templates.TemplateChunk {
	if licState == nil {
		return templates.TemplateChunk{
			TemplateData: &licenseTemplate{},
		}
	}
	errMsg := ""
	if licState.ErrorKey != "" {
		errMsg = commonviews.Tr(licState.UIState, licState.ErrorKey)
	}
	expiresText := commonviews.Tr(licState.UIState, i18n.TrLicenseLifetime)
	if licState.ExpiresAt > 0 {
		expiresText = time.Unix(licState.ExpiresAt, 0).Format(dateFormatISO)
	}
	return templates.TemplateChunk{
		TemplateData: &licenseTemplate{
			ErrorMsg:    errMsg,
			HolderName:  licState.HolderName,
			Email:       licState.Email,
			Tier:        strings.ToUpper(licState.Tier),
			ExpiresText: expiresText,
			IsActive:    licState.IsActive(),
			HasError:    errMsg != "",
		},
	}
}
