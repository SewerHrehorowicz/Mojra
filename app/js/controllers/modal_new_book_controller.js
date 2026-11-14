"use strict";

const ModalNewBookController = {
  data: {},

  bindings() {
    return {
      "#modal-backdrop-new-book": {
        click: this.onBackdrop
      },
      "#form-new-book": {
        submit: Dispatcher.action("CreateNewBook")
      },
      "#form-new-book .new-book-mode-select .custom-select-option": {
        click: this.selectMode
      },
      "#form-new-book [data-radio-option]": {
        click: this.selectRadio
      },
      "#form-new-book [data-form-toggle]": {
        click: this.toggleFormBtn
      },
      "#btn-browse-new-book-folder": {
        click: Dispatcher.action("BrowseNewBookFolder")
      },
      ".btn-close-new-book": {
        click: Dispatcher.action("CloseNewBookModal")
      }
    };
  },

  settings: {
    closeAction: "CloseNewBookModal",
    setModeAction: "SetNewBookModalMode"
  },

  onBackdrop(_params, el, e) {
    if (e && e.target === el) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  },

  selectMode(p) {
    Dispatcher.post(this.settings.setModeAction, Object.assign({}, p, { mode: p.select_value || "" }));
  },

  selectRadio(p, el) {
    if (!el) return;
    StudioController.selectRadio(p, el);
  },

  toggleFormBtn(p, el) {
    if (!el || el.disabled) return;
    StudioController.toggleFormBtn(p, el);
  }
};
Dispatcher.register("ModalNewBookController", ModalNewBookController);
