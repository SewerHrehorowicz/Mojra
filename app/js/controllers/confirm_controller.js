"use strict";

const ConfirmController = {
  data: {},

  bindings() {
    return {
      "#confirm-view": {
        click: this.onBackdrop
      },
      "#confirm-view .btn-confirm-cancel": {
        click: Dispatcher.action("ConfirmModalClose")
      },
      "#confirm-view .btn-confirm-accept": {
        click: Dispatcher.action("ConfirmModalAccept")
      }
    };
  },

  settings: {
    modalSelector: "#confirm-view",
    closeAction: "ConfirmModalClose"
  },

  isOpen() {
    return Boolean(document.querySelector(this.settings.modalSelector));
  },

  onBackdrop(_params, el, e) {
    if (e && e.target === el) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  }
};
Dispatcher.register("ConfirmController", ConfirmController);
