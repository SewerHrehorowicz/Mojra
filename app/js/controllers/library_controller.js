"use strict";

const LibraryController = {
  data: {
    bookIds: ""
  },

  bindings() {
    return {
      "#btn-open-add-book": {
        click: Dispatcher.action("OpenAddBookModal")
      },
      // @dev-only-start
      "#btn-extract-embedded-book": {
        click: Dispatcher.action("ExtractEmbeddedBook")
      },
      // @dev-only-end
      "#view-library article.book-card button.book-cover-wrap": {
        click: this.openBook
      },
      "#btn-switch-to-studio": {
        click: this.switchToStudio
      }
    };
  },

  settings: {
    actions: {
      openBook: "OpenBook",
      switchView: "SwitchView"
    },
    vals: {
      playerView: "player",
      studioView: "studio"
    }
  },

  bookIdAt(idx) {
    if (!this.data.bookIds) return "";
    const list = this.data.bookIds.split(",");
    return list[idx] || "";
  },

  openBook(p) {
    Dispatcher.post(this.settings.actions.openBook, {
      book: p.book || this.bookIdAt(p.idx),
      view: p.view || this.settings.vals.playerView
    });
  },

  switchToStudio() {
    Dispatcher.post(this.settings.actions.switchView, {
      view: this.settings.vals.studioView
    });
  }
};
Dispatcher.register("LibraryController", LibraryController);
