"use strict";

const LicenseController = {
  data: {},

  bindings() {
    return {
      "#form-license": {
        submit: Dispatcher.action("ActivateLicense")
      },
      "#btn-browse-license-file": {
        click: Dispatcher.action("BrowseLicenseFile")
      },
      "#btn-deactivate-license": {
        click: Dispatcher.action("DeactivateLicense")
      },
      "#btn-settings-deactivate-license": {
        click: Dispatcher.action("DeactivateLicense")
      },
      "#btn-close-license-summary": {
        click: Dispatcher.action("CloseLicenseSummary")
      },
      "#btn-license-to-reader": {
        click: this.switchToLibrary
      }
    };
  },

  settings: {
    actions: {
      switchView: "SwitchView"
    },
    vals: {
      libraryView: "library"
    }
  },

  switchToLibrary() {
    Dispatcher.post(this.settings.actions.switchView, {
      view: this.settings.vals.libraryView
    });
  }
};
Dispatcher.register("LicenseController", LicenseController);
