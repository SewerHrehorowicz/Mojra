package state

import "mojra/engine/transport"

// ConfirmAcceptFunc executes the confirmed operation and returns the resulting FDP ActionFrames.
type ConfirmAcceptFunc func() []transport.ActionFrame

// ConfirmState holds the state of the confirmation view.
type ConfirmState struct {
	Title         string
	Message       string
	PendingAction string
	PendingParam  string
	Danger        bool
	AllowLAN      bool
	OnAccept      ConfirmAcceptFunc
}

// IsOpen reports whether a confirmation prompt is currently active.
func (confirm *ConfirmState) IsOpen() bool {
	return confirm != nil && confirm.PendingAction != ""
}

// Prompt configures and opens the confirmation view.
func (confirm *ConfirmState) Prompt(next ConfirmState) {
	if confirm == nil {
		return
	}
	*confirm = next
}

// Reset clears the confirmation view state.
func (confirm *ConfirmState) Reset() {
	if confirm == nil {
		return
	}
	*confirm = ConfirmState{}
}

// Clear resets the confirmation view state.
func (confirm *ConfirmState) Clear() {
	if confirm == nil {
		return
	}
	*confirm = ConfirmState{}
}
