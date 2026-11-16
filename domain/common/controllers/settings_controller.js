"use strict";

const SettingsController = {
  data: {
    theme: "dark",
    uiScale: "small",
    imageless: false,
    imagelessLocked: false,
    sensitivity: "medium",
    autoChoiceDelay: 3
  },

  bindings() {
    return {
      "#settings-view": {
        click: this.onBackdrop
      },
      "#settings-view .btn-close-settings": {
        click: Dispatcher.action("CloseSettingsModal")
      },
      "#settings-view .lang-radio-btn": {
        click: Dispatcher.action("SetLanguage")
      },
      "#btn-toggle-theme": {
        click: Dispatcher.action("ToggleTheme")
      },
      "#btn-toggle-imageless": {
        click: Dispatcher.action("ToggleImageless")
      },
      "#settings-view .ui-scale-radio-btn": {
        click: Dispatcher.action("SetUIScale")
      },
      "#settings-view .scroll-sens-radio-btn": {
        click: Dispatcher.action("SetScrollSensitivity")
      },
      "#settings-view .auto-delay-radio-btn": {
        click: Dispatcher.action("SetAutoChoiceDelay")
      },
      // @dev-only-start
      "#settings-view .btn-extract-embedded-book": {
        click: Dispatcher.action("ExtractEmbeddedBook")
      }
      // @dev-only-end
    };
  },

  settings: {
    closeAction: "CloseSettingsModal"
  },

  onBackdrop(_params, backdropEl, domEvent) {
    if (domEvent && domEvent.target === backdropEl) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  }
};
Dispatcher.register("SettingsController", SettingsController);
