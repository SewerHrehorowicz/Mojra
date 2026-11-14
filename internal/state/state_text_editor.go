package state

import (
	"strings"

	"mojra/internal/book"
)

// SceneWidthMode controls whether the scene text editor panel occupies half or full workspace width.
type SceneWidthMode uint8

const (
	WidthModeHalf SceneWidthMode = iota
	WidthModeFull
)

var widthModeNames = [...]string{
	WidthModeHalf: "half",
	WidthModeFull: "full",
}

// ParseSceneWidthMode parses a raw width mode string into SceneWidthMode (defaulting to WidthModeHalf).
func ParseSceneWidthMode(raw string) SceneWidthMode {
	if strings.EqualFold(strings.TrimSpace(raw), widthModeNames[WidthModeFull]) {
		return WidthModeFull
	}
	return WidthModeHalf
}

// String returns the canonical wire/INI identifier for the width mode.
func (m SceneWidthMode) String() string {
	if int(m) < len(widthModeNames) {
		return widthModeNames[m]
	}
	return widthModeNames[WidthModeHalf]
}

// ScenePreviewMode controls the live reader preview inside the scene text editor panel.
type ScenePreviewMode uint8

const (
	PreviewModeNormal ScenePreviewMode = iota
	PreviewModeHighlight
	PreviewModeOutput
)

var previewModeNames = [...]string{
	PreviewModeNormal:    "normal",
	PreviewModeHighlight: "highlight",
	PreviewModeOutput:    "output",
}

// ParseScenePreviewMode parses a raw preview mode string into ScenePreviewMode.
func ParseScenePreviewMode(raw string) ScenePreviewMode {
	clean := strings.ToLower(strings.TrimSpace(raw))
	for i, name := range previewModeNames {
		if clean == name {
			return ScenePreviewMode(i)
		}
	}
	return PreviewModeNormal
}

// String returns the canonical wire identifier for the scene preview mode.
func (m ScenePreviewMode) String() string {
	if int(m) < len(previewModeNames) {
		return previewModeNames[m]
	}
	return previewModeNames[PreviewModeNormal]
}

// TextEditorState holds the state of the Playbook Studio scene text editor panel (#modal-scene-text).
type TextEditorState struct {
	SceneID         string
	InitialID       string
	WidthMode       SceneWidthMode
	SettingsOpen    bool
	PreviewMode     ScenePreviewMode
	TestVars        map[string]string
	ChoiceRulesOpen map[string]bool
	SyntaxHelpOpen  bool
	TestVarsOpen    bool
}

// IsOpen reports whether the scene text editor panel is currently open.
func (m *TextEditorState) IsOpen() bool {
	return m.SceneID != ""
}

// Open opens the scene text editor panel for the given scene ID and resets transient popovers.
func (m *TextEditorState) Open(sceneID string) {
	m.SceneID = sceneID
	m.InitialID = sceneID
	m.ChoiceRulesOpen = nil
	m.SyntaxHelpOpen = false
	m.TestVarsOpen = false
}

// SyncSceneID updates the active and initial scene IDs of the open scene text panel.
func (m *TextEditorState) SyncSceneID(sceneID string) {
	m.SceneID = sceneID
	m.InitialID = sceneID
}

// Close closes the scene text editor panel while preserving sticky layout preferences (WidthMode, SettingsOpen).
func (m *TextEditorState) Close() {
	m.SceneID = ""
	m.InitialID = ""
	m.PreviewMode = PreviewModeNormal
	m.TestVars = nil
	m.ChoiceRulesOpen = nil
	m.SyntaxHelpOpen = false
	m.TestVarsOpen = false
}

// ApplyNavigationEffects applies choice and destination scene variable effects to the preview test state.
func (m *TextEditorState) ApplyNavigationEffects(b *book.Book, chosenEffects, nextSceneEffects string) {
	if b == nil || len(b.Variables) == 0 {
		return
	}
	m.TestVars = book.EnsureSaveVariables(b, m.TestVars)
	if chosenEffects != "" {
		book.ApplyEffects(chosenEffects, m.TestVars, b)
	}
	if nextSceneEffects != "" {
		book.ApplyEffects(nextSceneEffects, m.TestVars, b)
	}
}

// ResetRulesUI clears open choice rule rows and syntax help popover state.
func (m *TextEditorState) ResetRulesUI() {
	m.ChoiceRulesOpen = nil
	m.SyntaxHelpOpen = false
}

// ToggleChoiceRule toggles the visibility of the condition/effect rule fields for a specific choice.
func (m *TextEditorState) ToggleChoiceRule(sceneID, idxStr string, hasRule bool) {
	if m.ChoiceRulesOpen == nil {
		m.ChoiceRulesOpen = make(map[string]bool)
	}
	key := sceneID + ":" + idxStr
	curOpen, exists := m.ChoiceRulesOpen[key]
	if !exists && hasRule {
		curOpen = true
	}
	m.ChoiceRulesOpen[key] = !curOpen
	if !m.ChoiceRulesOpen[key] {
		m.SyntaxHelpOpen = false
	}
}

// TogglePreviewMode toggles the live prose preview mode between normal and output.
func (m *TextEditorState) TogglePreviewMode() {
	if m.PreviewMode == PreviewModeOutput || m.PreviewMode == PreviewModeHighlight {
		m.PreviewMode = PreviewModeNormal
	} else {
		m.PreviewMode = PreviewModeOutput
	}
}
