package views

import "mojra/internal/state"

type modalEditorHelpView struct {
	ShowEditorHelpModal bool `tpl:"show_editor_help_modal"`
}

func ViewModalEditorHelp(s *state.AppState) *modalEditorHelpView {
	return &modalEditorHelpView{
		ShowEditorHelpModal: s.Editor.RightPanel == state.PanelHelp,
	}
}
