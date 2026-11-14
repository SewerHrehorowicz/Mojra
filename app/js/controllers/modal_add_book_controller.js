"use strict";

const ModalAddBookController = {
  data: {},

  bindings() {
    return {
      "#modal-backdrop-add-book": {
        click: this.onBackdrop
      },
      "#modal-backdrop-add-book .btn-close-add-book": {
        click: Dispatcher.action("CloseAddBookModal")
      },
      "#modal-backdrop-add-book .btn-browse-add-book": {
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
Dispatcher.register("ModalAddBookController", ModalAddBookController);
