//go:build !(js && wasm)

package views

import (
	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/studio/state"
	"mojra/engine/templates"
)

type studioProjectsTemplate struct {
	ShowStudioProjectModal bool
	HasStudioProjects      bool
	StudioProjects         []book.Book
}

func StudioProjectsView(s *state.StudioState) templates.TemplateChunk {
	var items []book.Book
	if s.Overlays.StudioProjects && !s.Config.StandaloneMode {
		var allBooks []*book.Book
		if s.Hooks.ListAllBooks != nil {
			allBooks = s.Hooks.ListAllBooks()
		}
		items = make([]book.Book, 0, len(allBooks))
		for _, b := range allBooks {
			kindBadge := tr(s, i18n.TrProjectKindBadge)
			if b.IsPacked {
				kindBadge = tr(s, i18n.TrPlaybookReadonlyBadge)
			}
			isCurrent := s.Editor.ActiveBook != nil && s.Editor.ActiveBook.ID == b.ID && !b.IsPacked
			item := *b
			item.KindBadge = kindBadge
			item.IsCurrent = isCurrent
			items = append(items, item)
		}
	}
	return templates.TemplateChunk{
		TemplateData: &studioProjectsTemplate{
			ShowStudioProjectModal: s.Overlays.StudioProjects,
			HasStudioProjects:      len(items) > 0,
			StudioProjects:         items,
		},
	}
}
