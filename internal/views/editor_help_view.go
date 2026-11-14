package views

import (
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type editorHelpTemplate struct {
	ShowEditorHelpModal bool
}

func EditorHelpView(s *state.AppState) tpl.TemplateChunk {
	return tpl.TemplateChunk{
		TemplateData: &editorHelpTemplate{
			ShowEditorHelpModal: s.Editor.RightPanel == state.PanelHelp,
		},
	}
}
