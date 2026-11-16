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

type licenseSummaryTemplate struct {
	HolderName  string
	Email       string
	Tier        string
	ExpiresText string
	IsActive    bool
}

// LicenseSummaryView builds the embeddable TemplateChunk (#license-summary-view) for the global Settings modal.
func LicenseSummaryView(licState *state.LicenseState) templates.TemplateChunk {
	if licState == nil {
		return templates.TemplateChunk{
			TemplateData: &licenseSummaryTemplate{},
		}
	}
	expiresText := commonviews.Tr(licState.UIState, i18n.TrLicenseLifetime)
	if licState.ExpiresAt > 0 {
		expiresText = time.Unix(licState.ExpiresAt, 0).Format(dateFormatISO)
	}
	return templates.TemplateChunk{
		TemplateData: &licenseSummaryTemplate{
			HolderName:  licState.HolderName,
			Email:       licState.Email,
			Tier:        strings.ToUpper(licState.Tier),
			ExpiresText: expiresText,
			IsActive:    licState.IsActive(),
		},
	}
}
