"use strict";

const ModalConfirmController = {
  data: {},

  bindings() {
    return {
      "#modal-confirm": {
        click: this.onBackdrop
      },
      "#modal-confirm .btn-confirm-cancel": {
        click: Dispatcher.action("ConfirmModalClose")
      },
      "#modal-confirm .btn-confirm-accept": {
        click: Dispatcher.action("ConfirmModalAccept")
      }
    };
  },

  settings: {
    modalSelector: "#modal-confirm",
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
Dispatcher.register("ModalConfirmController", ModalConfirmController);
