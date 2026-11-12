package views

import "mojra/internal/state"

type modalConfirmView struct {
	ShowConfirmModal bool   `tpl:"show_confirm_modal"`
	ConfirmTitle     string `tpl:"confirm_title"`
	ConfirmMessage   string `tpl:"confirm_message"`
	ConfirmIsDanger  bool   `tpl:"confirm_is_danger"`
}

func ViewModalConfirm(s *state.AppState) *modalConfirmView {
	return &modalConfirmView{
		ShowConfirmModal: s.ModalConfirm.IsOpen(),
		ConfirmTitle:     s.ModalConfirm.Title,
		ConfirmMessage:   s.ModalConfirm.Message,
		ConfirmIsDanger:  s.ModalConfirm.Danger,
	}
}
