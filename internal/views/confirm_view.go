package views

import (
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type confirmTemplate struct {
	ShowConfirmModal bool
	ConfirmTitle     string
	ConfirmMessage   string
	ConfirmIsDanger  bool
}

func ConfirmView(s *state.AppState) tpl.TemplateChunk {
	return tpl.TemplateChunk{
		TemplateData: &confirmTemplate{
			ShowConfirmModal: s.Confirm.IsOpen(),
			ConfirmTitle:     s.Confirm.Title,
			ConfirmMessage:   s.Confirm.Message,
			ConfirmIsDanger:  s.Confirm.Danger,
		},
	}
}
