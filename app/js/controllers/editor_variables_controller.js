"use strict";

const EditorVariablesController = {
  data: {},

  bindings() {
    return {
      "#editor-variables-panel .btn-close-variables": {
        click: Dispatcher.action("CloseEditorVariablesPanel")
      },
      "#editor-variables-panel .var-section-add-btn": {
        click: Dispatcher.action("AddBookVariable")
      },
      "#editor-variables-panel .var-panel-inline-form": {
        submit: Dispatcher.action("UpdateBookVariable")
      },
      "#editor-variables-panel .var-name-input, #editor-variables-panel .var-val-input, #editor-variables-panel .var-options-input": {
        change: Dispatcher.action("UpdateBookVariable")
      },
      "#editor-variables-panel .var-bool-toggle-btn": {
        click: Dispatcher.action("ToggleBookVariableBool")
      },
      "#editor-variables-panel .var-remove-btn": {
        click: Dispatcher.action("RemoveBookVariable")
      },
      "#editor-variables-panel .var-enum-add-opt-btn": {
        click: Dispatcher.action("AddBookEnumOption")
      },
      "#editor-variables-panel .var-enum-opt-remove-btn": {
        click: Dispatcher.action("RemoveBookEnumOption")
      }
    };
  },

  settings: {}
};
Dispatcher.register("EditorVariablesController", EditorVariablesController);
