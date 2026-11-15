package views

import (
	"mojra/domain/book"
	"mojra/domain/i18n"
	"mojra/domain/state"
	"mojra/engine/templates"
)

type topbarTemplate struct {
	IsViewLibrary   bool
	IsViewPlayer    bool
	IsViewEditor    bool
	HasEditorBook   bool
	EditorBook      *book.Book
	PlayerBook      *book.Book
	ActiveBookTitle string
	ShowZenMode     bool
	ZenMode         bool
	ZenModeAttr     string
	ZenToggleTitle  string
}

func contextBookForTopbar(s *state.AppState) *book.Book {
	if s == nil {
		return nil
	}
	switch state.ActiveWorkspaceView(s) {
	case state.ViewPlayer:
		return s.Player.Book
	case state.ViewEditor:
		return s.Editor.ActiveBook
	default:
		return nil
	}
}

func TopbarView(s *state.AppState) templates.TemplateChunk {
	activeView := state.ActiveWorkspaceView(s)
	hasEditorBook := s.Editor.HasActiveBook()
	var editorBook *book.Book
	if hasEditorBook {
		editorBook = s.Editor.ActiveBook
	}
	activeBookTitle := tr(s, i18n.TrNavLibrary)
	if s.Player.Book != nil {
		activeBookTitle = s.Player.Book.Title
	}

	showZenMode := activeView == state.ViewPlayer && s.Player.Book != nil && !s.Player.Book.IsScreenplay()
	zenMode := showZenMode && s.Player.ZenMode
	zenModeAttr := attrFalse
	zenTitle := tr(s, i18n.TrScaleZenHint)
	if zenMode {
		zenModeAttr = attrTrue
		zenTitle = tr(s, i18n.TrScaleExitZenHint)
	}

	return templates.TemplateChunk{
		TemplateData: &topbarTemplate{
			IsViewLibrary:   activeView == state.ViewLibrary,
			IsViewPlayer:    activeView == state.ViewPlayer,
			IsViewEditor:    activeView == state.ViewEditor,
			HasEditorBook:   hasEditorBook,
			EditorBook:      editorBook,
			PlayerBook:      s.Player.Book,
			ActiveBookTitle: activeBookTitle,
			ShowZenMode:     showZenMode,
			ZenMode:         zenMode,
			ZenModeAttr:     zenModeAttr,
			ZenToggleTitle:  zenTitle,
		},
	}
}
