package views

import (
	book "mojra/domain/book/actions"
	commonviews "mojra/domain/common/views"
	playerviews "mojra/domain/player/views"
	"mojra/domain/shell/book_player/state"
	"mojra/engine/templates"
)

const (
	defaultTitle   = "Mojra"
	defaultBookID  = "book"
	defaultWasmSrc = "mojra.wasm"
	themeLight     = "light"
	themeDark      = "dark"
	scaleSmall     = "small"
	scaleLarge     = "large"
)

type shellTemplate struct {
	IsDocumentRoot    bool
	ShowSplash        bool
	Title             string
	Author            string
	Description       string
	CoverSrc          string
	BookID            string
	WasmSrc           string
	Theme             string
	UIScale           string
	ImagelessAttr     bool
	ScrollSensitivity string
	WorkspaceView     templates.TemplateChunk
	ActiveModals      []templates.TemplateChunk
	UI                templates.TemplateChunk
}

// ActiveOverlayViews returns the active overlay chunks for #modals-root in book_player.
func ActiveOverlayViews(s *state.ShellState) []templates.TemplateChunk {
	var modals []templates.TemplateChunk
	if s.UI.Confirm.IsOpen() {
		modals = append(modals, commonviews.ConfirmView(s.UI))
	}
	return modals
}

func buildBookPlayerChunk(s *state.ShellState, isDocumentRoot bool, showSplash bool) templates.TemplateChunk {
	uiChunk := commonviews.UIView(s.UI)
	title := defaultTitle
	author := ""
	desc := ""
	bookID := defaultBookID
	coverSrc := ""
	effImageless := s.UI.Settings.Imageless
	if s.Player.Book != nil {
		b := s.Player.Book
		effImageless = b.IsEffectiveImageless(s.UI.Settings)
		if b.Title != "" {
			title = b.Title
		}
		author = b.Author
		desc = b.Description
		if b.ID != "" {
			bookID = b.ID
		}
		if !effImageless {
			coverSrc = b.CoverURL
		}
	}
	theme := s.UI.Settings.Theme
	if theme != themeLight {
		theme = themeDark
	}
	uiScale := s.UI.Settings.UIScale
	if uiScale != scaleLarge {
		uiScale = scaleSmall
	}

	tpl := &shellTemplate{
		IsDocumentRoot:    isDocumentRoot,
		ShowSplash:        showSplash,
		Title:             title,
		Author:            author,
		Description:       desc,
		CoverSrc:          coverSrc,
		BookID:            bookID,
		WasmSrc:           defaultWasmSrc,
		Theme:             theme,
		UIScale:           uiScale,
		ImagelessAttr:     effImageless,
		ScrollSensitivity: book.NormalizeScrollSensitivity(s.UI.Settings.ScrollSensitivity),
		WorkspaceView:     playerviews.PlayerView(&s.Player),
		ActiveModals:      ActiveOverlayViews(s),
		UI:                uiChunk,
	}
	return templates.TemplateChunk{
		TemplateData: tpl,
		ScriptData:   uiChunk.ScriptData,
	}
}

// ShellView renders the inner #app layout chunk (without topbar) for book_player.
func ShellView(s *state.ShellState) templates.TemplateChunk {
	return buildBookPlayerChunk(s, false, false)
}

// ShellDocumentView renders the full HTML document chunk for book_player.
func ShellDocumentView(s *state.ShellState, showSplash bool) templates.TemplateChunk {
	return buildBookPlayerChunk(s, true, showSplash)
}
