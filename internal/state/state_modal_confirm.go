package state

// ModalConfirmState holds the state and pending server action for the confirmation modal (#modal-confirm).
type ModalConfirmState struct {
	Title         string
	Message       string
	PendingAction string
	PendingParam  string
	Danger        bool
}

// IsOpen reports whether a confirmation prompt is currently active.
func (m *ModalConfirmState) IsOpen() bool {
	return m.PendingAction != ""
}

// Prompt configures and opens the confirmation modal with an explicit danger flag.
func (m *ModalConfirmState) Prompt(next ModalConfirmState) {
	*m = next
}

// Reset clears and closes the confirmation modal.
func (m *ModalConfirmState) Reset() {
	*m = ModalConfirmState{}
}
