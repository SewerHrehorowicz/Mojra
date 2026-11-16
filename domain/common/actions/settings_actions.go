//go:build !(js && wasm)

package actions

import (
	"mojra/domain/common/state"
	"mojra/domain/common/views"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

var _ = registerStandaloneActions(map[string]ActionHandler{
	"OpenSettingsModal":  openSettingsModal,
	"CloseSettingsModal": closeSettingsModal,
})

func openSettingsModal(ui *state.UIState, _ *transport.ActionRequest) []transport.ActionFrame {
	ui.Overlays.Settings = true
	if ui.Hooks.RenderModalsFrame != nil {
		return []transport.ActionFrame{ui.Hooks.RenderModalsFrame()}
	}
	return []transport.ActionFrame{
		{Target: state.TargetModalsRoot, Chunks: []templates.TemplateChunk{views.SettingsView(ui)}},
	}
}

func closeSettingsModal(ui *state.UIState, _ *transport.ActionRequest) []transport.ActionFrame {
	ui.Overlays.Settings = false
	if ui.Hooks.RenderModalsFrame != nil {
		return []transport.ActionFrame{ui.Hooks.RenderModalsFrame()}
	}
	return []transport.ActionFrame{
		{Target: state.TargetModalsRoot, Chunks: []templates.TemplateChunk{views.ConfirmView(ui)}},
	}
}
