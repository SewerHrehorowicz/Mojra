"use strict";

const AddBookController = {
  data: {},

  bindings() {
    return {
      "#add-book-view": {
        click: this.onBackdrop
      },
      "#add-book-view .btn-close-add-book": {
        click: Dispatcher.action("CloseAddBookModal")
      },
      "#add-book-view .btn-browse-add-book": {
        click: Dispatcher.action("BrowseAddBookFromDisk")
      },
      "#form-add-book-disk": {
        submit: Dispatcher.action("AddBookFromDisk")
      },
      "#form-add-book-url": {
        submit: Dispatcher.action("AddBookFromURL")
      }
    };
  },

  settings: {
    closeAction: "CloseAddBookModal"
  },

  onBackdrop(_params, el, e) {
    if (e && e.target === el) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  }
};
Dispatcher.register("AddBookController", AddBookController);
