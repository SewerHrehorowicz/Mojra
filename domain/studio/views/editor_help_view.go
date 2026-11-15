//go:build !(js && wasm)

package views

import (
	"mojra/domain/studio/state"
	"mojra/engine/templates"
)

type editorHelpTemplate struct {
	ShowEditorHelpPanel bool
}

func EditorHelpView(s *state.StudioState) templates.TemplateChunk {
	return templates.TemplateChunk{
		TemplateData: &editorHelpTemplate{
			ShowEditorHelpPanel: s.Editor.RightPanel == state.PanelHelp,
		},
	}
}
