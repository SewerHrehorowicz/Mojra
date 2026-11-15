package views

import (
	"mojra/domain/state"
	"mojra/engine/templates"
)

type addBookTemplate struct {
	ShowAddBookModal bool
}

func AddBookView(s *state.AppState) templates.TemplateChunk {
	return templates.TemplateChunk{
		TemplateData: &addBookTemplate{
			ShowAddBookModal: s.Overlays.AddBook,
		},
	}
}
