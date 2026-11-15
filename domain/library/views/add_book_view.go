//go:build !(js && wasm)

package views

import (
	"mojra/domain/library/state"
	"mojra/engine/templates"
)

type addBookTemplate struct {
	ShowAddBookModal bool
}

func AddBookView(s *state.LibraryState) templates.TemplateChunk {
	return templates.TemplateChunk{
		TemplateData: &addBookTemplate{
			ShowAddBookModal: s.Overlays.AddBook,
		},
	}
}
