package views

import (
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type addBookTemplate struct {
	ShowAddBookModal bool
}

func AddBookView(s *state.AppState) tpl.TemplateChunk {
	return tpl.TemplateChunk{
		TemplateData: &addBookTemplate{
			ShowAddBookModal: s.Overlays.AddBook,
		},
	}
}
