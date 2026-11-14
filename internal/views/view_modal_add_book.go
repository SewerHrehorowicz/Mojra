package views

import (
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type modalAddBookTemplate struct {
	ShowAddBookModal bool
}

func ViewModalAddBook(s *state.AppState) tpl.TemplateChunk {
	return tpl.TemplateChunk{
		TemplateData: &modalAddBookTemplate{
			ShowAddBookModal: s.Modals.AddBook,
		},
	}
}
