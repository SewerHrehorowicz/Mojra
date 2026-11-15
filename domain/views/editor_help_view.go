package views

import (
	"mojra/domain/state"
	"mojra/engine/templates"
)

type editorHelpTemplate struct {
	ShowEditorHelpModal bool
}

func EditorHelpView(s *state.AppState) templates.TemplateChunk {
	return templates.TemplateChunk{
		TemplateData: &editorHelpTemplate{
			ShowEditorHelpModal: s.Editor.RightPanel == state.PanelHelp,
		},
	}
}
