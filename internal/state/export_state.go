package state

// ExportState holds the state of the project export view (#export-view).
type ExportState struct {
	Open         bool
	Format       string
	Formats      map[string]bool
	AdvancedOpen bool
	Standard     string
	Dir          string
}

// Reset clears the export view state.
func (m *ExportState) Reset() {
	*m = ExportState{}
}
