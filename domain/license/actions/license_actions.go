//go:build !(js && wasm)

package actions

import (
	commonactions "mojra/domain/common/actions"
	commonstate "mojra/domain/common/state"
	commonviews "mojra/domain/common/views"
	"mojra/domain/i18n"
	"mojra/domain/license/state"
	"mojra/domain/license/views"
	"mojra/engine/osbridge"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

const formKeyLicenseKey = "key"

type ActionHandler func(licState *state.LicenseState, req *transport.ActionRequest) []transport.ActionFrame

func registerActions(actionMap map[string]ActionHandler) bool {
	for actionName, handler := range actionMap {
		boundHandler := handler
		commonactions.RegisterAction(actionName, false, false, func(ui *commonstate.UIState, req *transport.ActionRequest) []transport.ActionFrame {
			if ui == nil || ui.Hooks.LicenseState == nil {
				return nil
			}
			licState, ok := ui.Hooks.LicenseState.(*state.LicenseState)
			if !ok || licState == nil {
				return nil
			}
			return boundHandler(licState, req)
		})
	}
	return true
}

var _ = registerActions(map[string]ActionHandler{
	"ActivateLicense":     activateLicense,
	"BrowseLicenseFile":   browseLicenseFile,
	"DeactivateLicense":   deactivateLicense,
	"CloseLicenseSummary": closeLicenseSummary,
})

func activateLicense(licState *state.LicenseState, req *transport.ActionRequest) []transport.ActionFrame {
	rawKey := ""
	if req != nil {
		rawKey = req.FormValue(formKeyLicenseKey)
	}
	if licState.VerifyKey(rawKey) {
		licState.ShowActivatedSummary = true
		_ = licState.SaveKeyToDisk(licState.RawKey)
		licState.Overlays.Toast = commonviews.Tr(licState.UIState, i18n.TrLicenseActivatedToast)
		if licState.Hooks.RenderAppFrames != nil {
			return licState.Hooks.RenderAppFrames()
		}
	}
	return []transport.ActionFrame{
		{
			Target: commonstate.TargetWorkspace,
			Chunks: []templates.TemplateChunk{views.LicenseView(licState)},
		},
	}
}

func browseLicenseFile(licState *state.LicenseState, _ *transport.ActionRequest) []transport.ActionFrame {
	var pickedPath string
	licState.WithUnlocked(func() {
		pickedPath = osbridge.PickNativeBookPath(false)
	})
	if pickedPath == "" {
		return []transport.ActionFrame{
			{
				Target: commonstate.TargetWorkspace,
				Chunks: []templates.TemplateChunk{views.LicenseView(licState)},
			},
		}
	}
	if licState.LoadKeyFromFile(pickedPath) {
		licState.ShowActivatedSummary = true
		_ = licState.SaveKeyToDisk(licState.RawKey)
		licState.Overlays.Toast = commonviews.Tr(licState.UIState, i18n.TrLicenseActivatedToast)
		if licState.Hooks.RenderAppFrames != nil {
			return licState.Hooks.RenderAppFrames()
		}
	}
	return []transport.ActionFrame{
		{
			Target: commonstate.TargetWorkspace,
			Chunks: []templates.TemplateChunk{views.LicenseView(licState)},
		},
	}
}

func closeLicenseSummary(licState *state.LicenseState, _ *transport.ActionRequest) []transport.ActionFrame {
	licState.ShowActivatedSummary = false
	if licState.Hooks.RenderAppFrames != nil {
		return licState.Hooks.RenderAppFrames()
	}
	return []transport.ActionFrame{
		{
			Target: commonstate.TargetWorkspace,
			Chunks: []templates.TemplateChunk{views.LicenseView(licState)},
		},
	}
}

func deactivateLicense(licState *state.LicenseState, _ *transport.ActionRequest) []transport.ActionFrame {
	licState.VerifyKey("")
	licState.ErrorKey = ""
	licState.ShowActivatedSummary = false
	licState.Overlays.Settings = false
	_ = licState.SaveKeyToDisk("")
	licState.Overlays.Toast = commonviews.Tr(licState.UIState, i18n.TrLicenseDeactivatedToast)
	if licState.Hooks.RenderAppFrames != nil {
		return licState.Hooks.RenderAppFrames()
	}
	return []transport.ActionFrame{
		{
			Target: commonstate.TargetWorkspace,
			Chunks: []templates.TemplateChunk{views.LicenseView(licState)},
		},
	}
}
