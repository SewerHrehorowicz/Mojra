"use strict";

const ModalEditorHelpController = {
  data: {},

  bindings() {
    return {
      "#modal-backdrop-editor-help": {
        click: this.onBackdrop
      },
      "#modal-backdrop-editor-help .btn-close-editor-help": {
        click: Dispatcher.action("CloseEditorHelpModal")
      }
    };
  },

  settings: {
    closeAction: "CloseEditorHelpModal"
  },

  onBackdrop(_params, el, e) {
    if (e && e.target === el) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  }
};
Dispatcher.register("ModalEditorHelpController", ModalEditorHelpController);
