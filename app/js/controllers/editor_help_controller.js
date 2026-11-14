"use strict";

const EditorHelpController = {
  data: {},

  bindings() {
    return {
      "#editor-help-view": {
        click: this.onBackdrop
      },
      "#editor-help-view .btn-close-editor-help": {
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
Dispatcher.register("EditorHelpController", EditorHelpController);
