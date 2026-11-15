"use strict";

const TopbarController = {
  data: {},

  bindings() {
    return {
      "#tab-studio-home": {
        click: this.switchToStudio
      },
      "#tab-library": {
        click: this.switchToLibrary
      },
      "#btn-toggle-zen": {
        click: Dispatcher.action("ToggleZenMode")
      },
      "#btn-open-settings": {
        click: Dispatcher.action("OpenQRModal")
      },
      "#btn-toggle-fullscreen": {
        click: Dispatcher.action("ToggleFullscreen")
      },
      "#topbar .btn-quit-app": {
        click: Dispatcher.action("QuitApp")
      }
    };
  },

  settings: {
    actions: {
      switchView: "SwitchView"
    },
    vals: {
      studioView: "studio",
      libraryView: "library"
    }
  },

  switchToStudio() {
    Dispatcher.post(this.settings.actions.switchView, {
      view: this.settings.vals.studioView
    });
  },

  switchToLibrary() {
    Dispatcher.post(this.settings.actions.switchView, {
      view: this.settings.vals.libraryView
    });
  }
};
Dispatcher.register("TopbarController", TopbarController);
