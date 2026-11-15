package state

import (
	"mojra/domain/book/model"
	commonstate "mojra/domain/common/state"
)

// LibraryState holds the in-memory list of loaded library books.
type LibraryState struct {
	*commonstate.UIState
	Library *LibraryState
	NewBook *NewBookState
	Books   []*model.Book
	Loaded  bool
}

// BindUI links the library state to the shared UIState and NewBookState.
func (lib *LibraryState) BindUI(ui *commonstate.UIState, newBook *NewBookState) {
	lib.UIState = ui
	lib.Library = lib
	lib.NewBook = newBook
}

// Set replaces the in-memory book list and marks the library state as loaded.
func (lib *LibraryState) Set(books []*model.Book) {
	lib.Books = books
	lib.Loaded = true
}

// Get returns the book with the given ID from the in-memory list, or nil if not found.
func (lib *LibraryState) Get(bookID string) *model.Book {
	if !lib.Loaded || bookID == "" {
		return nil
	}
	for _, item := range lib.Books {
		if item != nil && item.ID == bookID {
			return item
		}
	}
	return nil
}

// Put replaces an existing book with the same ID in-place or appends a new book to the in-memory list.
func (lib *LibraryState) Put(targetBook *model.Book) {
	if targetBook == nil || !lib.Loaded {
		return
	}
	for idx, existing := range lib.Books {
		if existing != nil && existing.ID == targetBook.ID {
			lib.Books[idx] = targetBook
			return
		}
	}
	lib.Books = append(lib.Books, targetBook)
}

// Remove deletes the book with the given ID from the in-memory list in-place.
func (lib *LibraryState) Remove(bookID string) {
	if !lib.Loaded || bookID == "" {
		return
	}
	for idx, existing := range lib.Books {
		if existing != nil && existing.ID == bookID {
			lib.Books = append(lib.Books[:idx], lib.Books[idx+1:]...)
			return
		}
	}
}
