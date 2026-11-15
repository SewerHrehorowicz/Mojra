package state

// NewBookState holds the form draft and open state for the new book/screenplay view (#new-book-view).
type NewBookState struct {
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

// Reset clears the new book view state.
func (m *NewBookState) Reset() {
	*m = NewBookState{}
}
