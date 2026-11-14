"use strict";

const EditorVariablesController = {
  data: {},

  bindings() {
    return {
      "#editor-variables-view .btn-close-variables": {
        click: Dispatcher.action("CloseEditorVariablesPanel")
      },
      "#editor-variables-view .var-section-add-btn": {
        click: Dispatcher.action("AddBookVariable")
      },
      "#editor-variables-view .var-panel-inline-form": {
        submit: Dispatcher.action("UpdateBookVariable")
      },
      "#editor-variables-view .var-name-input, #editor-variables-view .var-val-input, #editor-variables-view .var-options-input": {
        change: Dispatcher.action("UpdateBookVariable")
      },
      "#editor-variables-view .var-bool-toggle-btn": {
        click: Dispatcher.action("ToggleBookVariableBool")
      },
      "#editor-variables-view .var-remove-btn": {
        click: Dispatcher.action("RemoveBookVariable")
      },
      "#editor-variables-view .var-enum-add-opt-btn": {
        click: Dispatcher.action("AddBookEnumOption")
      },
      "#editor-variables-view .var-enum-opt-remove-btn": {
        click: Dispatcher.action("RemoveBookEnumOption")
      }
    };
  },

  settings: {}
};
Dispatcher.register("EditorVariablesController", EditorVariablesController);
