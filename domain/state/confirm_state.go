package state

// ConfirmState holds the state and pending server action for the confirmation view (#confirm-view).
type ConfirmState struct {
	Title         string
	Message       string
	PendingAction string
	PendingParam  string
	Danger        bool
}

// IsOpen reports whether a confirmation prompt is currently active.
func (m *ConfirmState) IsOpen() bool {
	return m.PendingAction != ""
}

// Prompt configures and opens the confirmation view with an explicit danger flag.
func (m *ConfirmState) Prompt(next ConfirmState) {
	*m = next
}

// Reset clears and closes the confirmation view.
func (m *ConfirmState) Reset() {
	*m = ConfirmState{}
}
