package state

import (
	"strings"

	"mojra/internal/book"
)

// EditorRightPanel identifies the active right-hand inspector/drawer panel in Playbook Studio.
type EditorRightPanel uint8

const (
	PanelNone EditorRightPanel = iota
	PanelBook
	PanelChapter
	PanelVariables
	PanelBreakdown
	PanelHistory
	PanelHelp
)

var rightPanelNames = [...]string{
	PanelNone:      "",
	PanelBook:      "book",
	PanelChapter:   "chapter",
	PanelVariables: "variables",
	PanelBreakdown: "breakdown",
	PanelHistory:   "history",
	PanelHelp:      "help",
}

// ParseEditorRightPanel converts a persisted right-panel key into an EditorRightPanel enum.
func ParseEditorRightPanel(raw string) EditorRightPanel {
	clean := strings.ToLower(strings.TrimSpace(raw))
	for i, name := range rightPanelNames {
		if clean == name {
			return EditorRightPanel(i)
		}
	}
	return PanelNone
}

// String returns the canonical wire/INI identifier for the right panel.
func (p EditorRightPanel) String() string {
	if int(p) < len(rightPanelNames) {
		return rightPanelNames[p]
	}
	return ""
}

// EditorState holds the active Playbook Studio project, selected entities, right panel, scene text panel, and revision history.
type EditorState struct {
	ActiveBook              *book.Book
	StudioHome              bool
	SelectedSceneID         string
	SelectedChapter         string
	SelectedVoiceSectionKey string
	RightPanel              EditorRightPanel
	LastRenamedFromSceneID  string
	SceneText               EditorSceneTextState
	History                 *book.BookHistory
}

// HasActiveBook reports whether Playbook Studio has an open project on the canvas.
func (e *EditorState) HasActiveBook() bool {
	return e.ActiveBook != nil && !e.StudioHome
}

// ActivateRightPanel switches the active right-side drawer in Playbook Studio, closing any mutually exclusive drawer.
func (e *EditorState) ActivateRightPanel(panel EditorRightPanel) {
	e.RightPanel = panel
	if panel != PanelChapter {
		e.SelectedChapter = ""
	}
}

// SaveBookUIState records the current Studio UI state for ActiveBook into settings.EditorBookStates.
func (e *EditorState) SaveBookUIState(settings *book.AppSettings) {
	if e.ActiveBook == nil || e.StudioHome || settings == nil {
		return
	}
	if e.History != nil && e.History.IsPreview(e.ActiveBook) {
		return
	}
	settings.SetEditorBookUIState(e.ActiveBook.ID, book.EditorBookUIState{
		SelectedSceneID:       e.SelectedSceneID,
		SceneTextModalID:      e.SceneText.SceneID,
		SceneTextWidthMode:    e.SceneText.WidthMode.String(),
		SceneTextSettingsOpen: e.SceneText.SettingsOpen,
		RightPanel:            e.RightPanel.String(),
		SelectedChapter:       e.SelectedChapter,
	})
}

// RestoreBookUIState restores the Studio UI state for ActiveBook from settings.EditorBookStates.
func (e *EditorState) RestoreBookUIState(settings *book.AppSettings, openIfSaved bool) {
	e.SelectedSceneID = ""
	e.SelectedVoiceSectionKey = ""
	e.RightPanel = PanelNone
	e.SelectedChapter = ""
	e.SceneText.Close()
	e.SceneText.WidthMode = WidthModeHalf
	e.SceneText.SettingsOpen = false

	b := e.ActiveBook
	if b == nil {
		return
	}
	defaultSceneID := b.StartSceneID
	if b.FindScene(defaultSceneID) == nil && len(b.Scenes) > 0 {
		defaultSceneID = b.Scenes[0].ID
	}
	e.SelectedSceneID = defaultSceneID

	if settings == nil {
		return
	}
	st, ok := settings.GetEditorBookUIState(b.ID)
	if !ok && settings.EditorBookID != "" && book.SanitizeBookID(settings.EditorBookID) == book.SanitizeBookID(b.ID) {
		st = book.EditorBookUIState{
			SelectedSceneID:       settings.EditorSelectedSceneID,
			SceneTextModalID:      settings.EditorSceneTextModalID,
			SceneTextWidthMode:    settings.EditorSceneTextWidth,
			SceneTextSettingsOpen: settings.EditorSceneSettings,
			RightPanel:            settings.EditorRightPanel,
			SelectedChapter:       settings.EditorSelectedChapter,
		}
		ok = true
	}
	if !ok {
		return
	}
	if st.SelectedSceneID != "" && b.FindScene(st.SelectedSceneID) != nil {
		e.SelectedSceneID = st.SelectedSceneID
	}
	e.SceneText.WidthMode = ParseSceneWidthMode(st.SceneTextWidthMode)
	e.SceneText.SettingsOpen = st.SceneTextSettingsOpen
	if openIfSaved && st.SceneTextModalID != "" && b.FindScene(st.SceneTextModalID) != nil {
		e.SceneText.Open(st.SceneTextModalID)
		e.SelectedSceneID = st.SceneTextModalID
	}
	panel := ParseEditorRightPanel(st.RightPanel)
	if panel == PanelChapter {
		if st.SelectedChapter != "" {
			for _, ch := range b.Chapters {
				if ch.Name == st.SelectedChapter {
					e.SelectedChapter = st.SelectedChapter
					e.RightPanel = PanelChapter
					break
				}
			}
		}
	} else {
		e.ActivateRightPanel(panel)
	}
}

// SyncToSettings reconciles the current Studio UI state with ActiveBook and updates settings if changed.
func (e *EditorState) SyncToSettings(settings *book.AppSettings) bool {
	if settings == nil {
		return false
	}
	if e.History != nil && e.History.IsPreview(e.ActiveBook) {
		return false
	}
	if e.StudioHome || e.ActiveBook == nil {
		changed := settings.EditorStudioHome != e.StudioHome
		settings.EditorStudioHome = e.StudioHome
		if e.StudioHome {
			if settings.EditorBookID != "" || settings.EditorSelectedSceneID != "" || settings.EditorSceneTextModalID != "" {
				changed = true
			}
			settings.EditorBookID = ""
			settings.EditorSelectedSceneID = ""
			settings.EditorSceneTextModalID = ""
			settings.EditorSceneTextWidth = ""
			settings.EditorSceneSettings = false
			settings.EditorRightPanel = ""
			settings.EditorSelectedChapter = ""
		}
		return changed
	}
	b := e.ActiveBook
	if e.SelectedSceneID == "" || b.FindScene(e.SelectedSceneID) == nil {
		e.SelectedSceneID = b.StartSceneID
		if b.FindScene(e.SelectedSceneID) == nil && len(b.Scenes) > 0 {
			e.SelectedSceneID = b.Scenes[0].ID
		}
	}
	if e.SceneText.SceneID != "" {
		if b.FindScene(e.SceneText.SceneID) == nil {
			e.SceneText.Close()
		} else {
			e.SelectedSceneID = e.SceneText.SceneID
		}
	}
	if e.RightPanel == PanelChapter {
		foundCh := false
		for _, ch := range b.Chapters {
			if ch.Name == e.SelectedChapter {
				foundCh = true
				break
			}
		}
		if !foundCh {
			e.SelectedChapter = ""
			e.RightPanel = PanelNone
		}
	}
	modalID := e.SceneText.SceneID
	widthMode := e.SceneText.WidthMode.String()
	settingsOpen := e.SceneText.SettingsOpen
	panelStr := e.RightPanel.String()
	next := book.EditorBookUIState{
		SelectedSceneID:       e.SelectedSceneID,
		SceneTextModalID:      modalID,
		SceneTextWidthMode:    widthMode,
		SceneTextSettingsOpen: settingsOpen,
		RightPanel:            panelStr,
		SelectedChapter:       e.SelectedChapter,
	}
	prev, ok := settings.GetEditorBookUIState(b.ID)
	changed := !ok || prev != next ||
		settings.EditorBookID != b.ID ||
		settings.EditorStudioHome ||
		settings.EditorSelectedSceneID != e.SelectedSceneID ||
		settings.EditorSceneTextModalID != modalID ||
		settings.EditorSceneTextWidth != widthMode ||
		settings.EditorSceneSettings != settingsOpen ||
		settings.EditorRightPanel != panelStr ||
		settings.EditorSelectedChapter != e.SelectedChapter
	if changed {
		settings.EditorBookID = b.ID
		settings.EditorStudioHome = false
		settings.EditorSelectedSceneID = e.SelectedSceneID
		settings.EditorSceneTextModalID = modalID
		settings.EditorSceneTextWidth = widthMode
		settings.EditorSceneSettings = settingsOpen
		settings.EditorRightPanel = panelStr
		settings.EditorSelectedChapter = e.SelectedChapter
		settings.SetEditorBookUIState(b.ID, next)
	}
	return changed
}

// SetBook sets the active Studio project, initializes its BookHistory, and restores its saved Studio UI state.
func (e *EditorState) SetBook(b *book.Book, settings *book.AppSettings) {
	if e.History != nil {
		e.History.Preview = nil
	}
	if e.ActiveBook != nil && (b == nil || b.ID != e.ActiveBook.ID) {
		e.SaveBookUIState(settings)
	}
	e.ActiveBook = b
	e.StudioHome = false
	if b == nil {
		e.History = nil
		e.SelectedSceneID = ""
		e.SelectedChapter = ""
		e.SelectedVoiceSectionKey = ""
		e.RightPanel = PanelNone
		e.SceneText.Close()
		return
	}
	e.History = &book.BookHistory{NextSeq: 1}
	if settings != nil && b.ExportDir == "" {
		if savedDir := settings.GetBookExportDir(b.ID); savedDir != "" {
			b.ExportDir = savedDir
		}
	}
	e.RestoreBookUIState(settings, true)
	if settings != nil {
		settings.ActiveBookID = b.ID
	}
	e.SaveBookUIState(settings)
}
