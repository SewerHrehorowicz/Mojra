package views

import (
	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

type studioProjectModalItem struct {
	Book      *book.Book `tpl:"*"`
	KindBadge string     `tpl:"kind_badge"`
	IsCurrent bool       `tpl:"is_current"`
}

type modalStudioProjectsView struct {
	ShowStudioProjectModal bool                     `tpl:"show_studio_project_modal"`
	HasStudioProjects      bool                     `tpl:"has_studio_projects"`
	StudioProjects         []studioProjectModalItem `tpl:"studio_projects"`
}

func ViewModalStudioProjects(s *state.AppState) *modalStudioProjectsView {
	var items []studioProjectModalItem
	if s.Modals.StudioProjects && !s.Config.StandaloneMode {
		allBooks := s.Library.Books
		items = make([]studioProjectModalItem, 0, len(allBooks))
		for _, b := range allBooks {
			kindBadge := tr(s, i18n.TrProjectKindBadge)
			if b.IsPacked {
				kindBadge = tr(s, i18n.TrPlaybookReadonlyBadge)
			}
			isCurrent := s.Editor.ActiveBook != nil && s.Editor.ActiveBook.ID == b.ID && !b.IsPacked
			items = append(items, studioProjectModalItem{
				Book:      b,
				KindBadge: kindBadge,
				IsCurrent: isCurrent,
			})
		}
	}
	return &modalStudioProjectsView{
		ShowStudioProjectModal: s.Modals.StudioProjects,
		HasStudioProjects:      len(items) > 0,
		StudioProjects:         items,
	}
}
