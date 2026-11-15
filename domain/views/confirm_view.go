package views

import (
	"mojra/domain/state"
	"mojra/engine/templates"
)

type confirmTemplate struct {
	ShowConfirmModal bool
	ConfirmTitle     string
	ConfirmMessage   string
	ConfirmIsDanger  bool
}

func ConfirmView(s *state.AppState) templates.TemplateChunk {
	return templates.TemplateChunk{
		TemplateData: &confirmTemplate{
			ShowConfirmModal: s.Confirm.IsOpen(),
			ConfirmTitle:     s.Confirm.Title,
			ConfirmMessage:   s.Confirm.Message,
			ConfirmIsDanger:  s.Confirm.Danger,
		},
	}
}
