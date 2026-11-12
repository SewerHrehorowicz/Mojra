package views

import (
	"mojra/internal/book"
	"mojra/internal/i18n"
	"mojra/internal/state"
)

type topbarView struct {
	IsViewLibrary   bool       `tpl:"is_view_library"`
	IsViewPlayer    bool       `tpl:"is_view_player"`
	IsViewEditor    bool       `tpl:"is_view_editor"`
	HasEditorBook   bool       `tpl:"has_editor_book"`
	EditorBook      *book.Book `tpl:"editor_book"`
	PlayerBook      *book.Book `tpl:"player_book"`
	ActiveBookTitle string     `tpl:"active_book_title"`
	ShowZenMode     bool       `tpl:"show_zen_mode"`
	ZenMode         bool       `tpl:"zen_mode"`
	ZenModeAttr     string     `tpl:"zen_mode_attr"`
	ZenToggleTitle  string     `tpl:"zen_toggle_title"`
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

func viewTopbar(s *state.AppState) topbarView {
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
	zenModeAttr := "false"
	zenTitle := tr(s, i18n.TrScaleZenHint)
	if zenMode {
		zenModeAttr = "true"
		zenTitle = tr(s, i18n.TrScaleExitZenHint)
	}

	return topbarView{
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
	}
}
