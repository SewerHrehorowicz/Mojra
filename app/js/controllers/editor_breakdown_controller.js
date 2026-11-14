"use strict";

const EditorBreakdownController = {
  data: {},

  bindings() {
    return {
      "#editor-breakdown-panel .btn-close-breakdown": {
        click: Dispatcher.action("CloseEditorBreakdownPanel")
      },
      "#editor-breakdown-panel .breakdown-rename-char-form": {
        submit: this.submitCharRename
      },
      "#editor-breakdown-panel .breakdown-rename-char-form .breakdown-name-input": {
        change: this.renameCharacter,
        blur: this.renameCharacter
      },
      "#editor-breakdown-panel .breakdown-rename-loc-form": {
        submit: this.submitLocRename
      },
      "#editor-breakdown-panel .breakdown-rename-loc-form .breakdown-name-input": {
        change: this.renameLocation,
        blur: this.renameLocation
      },
      "#editor-breakdown-panel .breakdown-jump-btn": {
        click: Dispatcher.action("SelectStudioScene")
      }
    };
  },

  settings: {
    nameInputSelector: ".breakdown-name-input",
    oldValAttr: "data-old-value",
    actions: {
      renameChar: "RenameCharacter",
      renameLoc: "RenameLocation"
    }
  },

  submitCharRename(p, formEl) {
    const inp = formEl && formEl.querySelector ? formEl.querySelector(this.settings.nameInputSelector) : null;
    const oldVal = inp ? inp.getAttribute(this.settings.oldValAttr) || "" : "";
    Dispatcher.post(this.settings.actions.renameChar, { old_name: oldVal, new_name: p.new_name || "" });
  },

  renameCharacter(p) {
    Dispatcher.post(this.settings.actions.renameChar, { old_name: p.old_value || "", new_name: p.new_name || "" });
  },

  submitLocRename(p, formEl) {
    const inp = formEl && formEl.querySelector ? formEl.querySelector(this.settings.nameInputSelector) : null;
    const oldVal = inp ? inp.getAttribute(this.settings.oldValAttr) || "" : "";
    Dispatcher.post(this.settings.actions.renameLoc, { old_location: oldVal, new_location: p.new_location || p.new_name || "" });
  },

  renameLocation(p) {
    Dispatcher.post(this.settings.actions.renameLoc, { old_location: p.old_value || "", new_location: p.new_location || p.new_name || "" });
  }
};
Dispatcher.register("EditorBreakdownController", EditorBreakdownController);
