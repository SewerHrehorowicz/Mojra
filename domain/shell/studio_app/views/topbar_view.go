//go:build !(js && wasm)

package views

import (
	book "mojra/domain/book/actions"
	"mojra/domain/i18n"
	"mojra/domain/shell/studio_app/state"
	"mojra/engine/templates"
)

const (
	attrTrue  = "true"
	attrFalse = "false"
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

// TopbarView prepares the TemplateChunk for topbar_view.tpl.
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
