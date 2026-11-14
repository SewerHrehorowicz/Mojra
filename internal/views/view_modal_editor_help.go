package views

import (
	"mojra/internal/state"
	"mojra/internal/tpl"
)

type modalEditorHelpTemplate struct {
	ShowEditorHelpModal bool
}

func ViewModalEditorHelp(s *state.AppState) tpl.TemplateChunk {
	return tpl.TemplateChunk{
		TemplateData: &modalEditorHelpTemplate{
			ShowEditorHelpModal: s.Editor.RightPanel == state.PanelHelp,
		},
	}
}
