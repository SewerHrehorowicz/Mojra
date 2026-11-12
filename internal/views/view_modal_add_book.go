package views

import "mojra/internal/state"

type modalAddBookView struct {
	ShowAddBookModal bool `tpl:"show_add_book_modal"`
}

func ViewModalAddBook(s *state.AppState) *modalAddBookView {
	return &modalAddBookView{
		ShowAddBookModal: s.Modals.AddBook,
	}
}
