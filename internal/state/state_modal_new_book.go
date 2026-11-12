package state

// ModalNewBookState holds the form draft and open state for the new book/screenplay modal (#modal-new-book).
type ModalNewBookState struct {
	Open        bool
	Mode        string
	Standard    string
	Title       string
	Author      string
	Description string
	Narrated    bool
	Imageless   bool
	Dir         string
}

// Reset clears the new book modal state.
func (m *ModalNewBookState) Reset() {
	*m = ModalNewBookState{}
}
