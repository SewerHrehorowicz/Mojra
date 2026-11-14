"use strict";

const ModalExportController = {
  data: {},

  bindings() {
    return {
      "#modal-backdrop-export": {
        click: this.onBackdrop
      },
      "#form-export-project": {
        submit: Dispatcher.action("ExportProject")
      },
      "#form-export-project .export-format-item": {
        click: Dispatcher.action("ToggleExportFormat")
      },
      "#btn-browse-export-folder": {
        click: Dispatcher.action("BrowseExportModalFolder")
      },
      "#form-export-project .export-advanced-toggle": {
        click: Dispatcher.action("ToggleExportModalAdvanced")
      },
      "#form-export-project .export-standard-select .custom-select-option": {
        click: this.selectStandard
      },
      ".btn-close-export": {
        click: Dispatcher.action("CloseExportModal")
      }
    };
  },

  settings: {
    closeAction: "CloseExportModal",
    setStandardAction: "SetExportModalStandard"
  },

  onBackdrop(_params, el, e) {
    if (e && e.target === el) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  },

  selectStandard(p) {
    Dispatcher.post(this.settings.setStandardAction, { standard: p.select_value || "" });
  }
};
Dispatcher.register("ModalExportController", ModalExportController);
