"use strict";

const NewBookController = {
  data: {},

  bindings() {
    return {
      "#new-book-view": {
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

  onBackdrop(_params, backdropEl, domEvent) {
    if (domEvent && domEvent.target === backdropEl) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  },

  selectMode(params) {
    Dispatcher.post(this.settings.setModeAction, Object.assign({}, params, { mode: params.select_value || "" }));
  },

  selectRadio(params, optionEl) {
    if (!optionEl) return;
    StudioController.selectRadio(params, optionEl);
  },

  toggleFormBtn(params, toggleEl) {
    if (!toggleEl || toggleEl.disabled) return;
    StudioController.toggleFormBtn(params, toggleEl);
  }
};
Dispatcher.register("NewBookController", NewBookController);
