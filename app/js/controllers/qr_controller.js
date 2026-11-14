"use strict";

const QRController = {
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
      "#qr-view": {
        click: this.onBackdrop
      },
      "#qr-view .btn-close-qr": {
        click: Dispatcher.action("CloseQRModal")
      },
      "#qr-view .lang-radio-btn": {
        click: Dispatcher.action("SetLanguage")
      },
      "#btn-toggle-theme": {
        click: Dispatcher.action("ToggleTheme")
      },
      "#btn-toggle-imageless": {
        click: Dispatcher.action("ToggleImageless")
      },
      "#qr-view .ui-scale-radio-btn": {
        click: Dispatcher.action("SetUIScale")
      },
      "#qr-view .scroll-sens-radio-btn": {
        click: Dispatcher.action("SetScrollSensitivity")
      },
      "#qr-view .auto-delay-radio-btn": {
        click: Dispatcher.action("SetAutoChoiceDelay")
      },
      // @dev-only-start
      "#qr-view .btn-extract-embedded-book": {
        click: Dispatcher.action("ExtractEmbeddedBook")
      }
      // @dev-only-end
    };
  },

  settings: {
    closeAction: "CloseQRModal"
  },

  onBackdrop(_params, el, e) {
    if (e && e.target === el) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  }
};
Dispatcher.register("QRController", QRController);
