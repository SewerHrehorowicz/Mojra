package views

import (
	"mojra/domain/common/state"
	"mojra/engine/templates"
)

type confirmTemplate struct {
	ShowConfirmModal bool
	ConfirmIsDanger  bool
	ConfirmTitle     string
	ConfirmMessage   string
	IsOpen           bool
	Danger           bool
	Title            string
	Message          string
}

type confirmScript struct{}

// ConfirmView prepares the TemplateChunk for confirm_view.tpl.
func ConfirmView(ui *state.UIState) templates.TemplateChunk {
	isOpen := ui.Confirm.PendingAction != ""
	return templates.TemplateChunk{
		TemplateData: &confirmTemplate{
			ShowConfirmModal: isOpen,
			ConfirmIsDanger:  ui.Confirm.Danger,
			ConfirmTitle:     ui.Confirm.Title,
			ConfirmMessage:   ui.Confirm.Message,
			IsOpen:           isOpen,
			Danger:           ui.Confirm.Danger,
			Title:            ui.Confirm.Title,
			Message:          ui.Confirm.Message,
		},
		ScriptData: confirmScript{},
	}
}
