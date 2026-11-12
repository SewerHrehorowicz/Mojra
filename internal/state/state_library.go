package state

import "mojra/internal/book"

// LibraryState holds the in-memory list of loaded library books.
type LibraryState struct {
	Books  []*book.Book
	Loaded bool
}

// Set replaces the in-memory book list and marks the library state as loaded.
func (l *LibraryState) Set(books []*book.Book) {
	l.Books = books
	l.Loaded = true
}

// Get returns the book with the given ID from the in-memory list, or nil if not found.
func (l *LibraryState) Get(bookID string) *book.Book {
	if !l.Loaded || bookID == "" {
		return nil
	}
	for _, b := range l.Books {
		if b != nil && b.ID == bookID {
			return b
		}
	}
	return nil
}

// Put replaces an existing book with the same ID in-place or appends a new book to the in-memory list.
func (l *LibraryState) Put(b *book.Book) {
	if b == nil || !l.Loaded {
		return
	}
	for i, existing := range l.Books {
		if existing != nil && existing.ID == b.ID {
			l.Books[i] = b
			return
		}
	}
	l.Books = append(l.Books, b)
}

// Remove deletes the book with the given ID from the in-memory list in-place.
func (l *LibraryState) Remove(bookID string) {
	if !l.Loaded || bookID == "" {
		return
	}
	for i, existing := range l.Books {
		if existing != nil && existing.ID == bookID {
			l.Books = append(l.Books[:i], l.Books[i+1:]...)
			return
		}
	}
}
