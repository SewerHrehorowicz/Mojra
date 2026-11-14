package views

import (
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type modalConfirmTemplate struct {
	ShowConfirmModal bool
	ConfirmTitle     string
	ConfirmMessage   string
	ConfirmIsDanger  bool
}

func ViewModalConfirm(s *state.AppState) tpl.TemplateChunk {
	return tpl.TemplateChunk{
		TemplateData: &modalConfirmTemplate{
			ShowConfirmModal: s.ModalConfirm.IsOpen(),
			ConfirmTitle:     s.ModalConfirm.Title,
			ConfirmMessage:   s.ModalConfirm.Message,
			ConfirmIsDanger:  s.ModalConfirm.Danger,
		},
	}
}
