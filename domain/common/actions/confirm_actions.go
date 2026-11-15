package actions

import (
	"mojra/domain/common/state"
	"mojra/domain/common/views"
	"mojra/engine/templates"
	"mojra/engine/transport"
)

var _ = registerStandaloneActions(map[string]ActionHandler{
	"ConfirmModalAccept": confirmModalAccept,
	"ConfirmModalClose":  confirmModalClose,
})

func confirmModalClose(ui *state.UIState, _ *transport.ActionRequest) []transport.ActionFrame {
	ui.Confirm.Clear()
	if ui.Hooks.RenderModalsFrame != nil {
		return []transport.ActionFrame{
			ui.Hooks.RenderModalsFrame(),
		}
	}
	return []transport.ActionFrame{
		{Target: state.TargetModalsRoot, Chunks: []templates.TemplateChunk{views.ConfirmView(ui)}},
	}
}

func confirmModalAccept(ui *state.UIState, req *transport.ActionRequest) []transport.ActionFrame {
	isLAN := req != nil && !req.IsLoopback
	if isLAN && !ui.Confirm.AllowLAN {
		ui.Confirm.Clear()
		return confirmModalClose(ui, req)
	}
	acceptFn := ui.Confirm.OnAccept
	pendingAction := ui.Confirm.PendingAction
	pendingParam := ui.Confirm.PendingParam
	ui.Confirm.Clear()

	if acceptFn != nil {
		return acceptFn()
	}
	if pendingAction != "" && ui.Hooks.ExecutePendingConfirm != nil {
		if frames, handled := ui.Hooks.ExecutePendingConfirm(pendingAction, pendingParam); handled {
			return frames
		}
	}
	if ui.Hooks.RenderAppFrames != nil {
		return ui.Hooks.RenderAppFrames()
	}
	return confirmModalClose(ui, req)
}
