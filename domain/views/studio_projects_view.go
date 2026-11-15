package views

import (
	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine/templates"
)

type studioProjectsTemplate struct {
	ShowStudioProjectModal bool
	HasStudioProjects      bool
	StudioProjects         []book.Book
}

func StudioProjectsView(s *state.AppState) templates.TemplateChunk {
	var items []book.Book
	if s.Overlays.StudioProjects && !s.Config.StandaloneMode {
		allBooks := s.Library.Books
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
