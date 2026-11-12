package state

// ModalExportState holds the state of the project export modal (#modal-export-project).
type ModalExportState struct {
	Open         bool
	Format       string
	Formats      map[string]bool
	AdvancedOpen bool
	Standard     string
	Dir          string
}

// Reset clears the export modal state.
func (m *ModalExportState) Reset() {
	*m = ModalExportState{}
}
