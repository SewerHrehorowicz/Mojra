"use strict";

const ModalQRController = {
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
      "#modal-backdrop-qr": {
        click: this.onBackdrop
      },
      "#modal-backdrop-qr .btn-close-qr": {
        click: Dispatcher.action("CloseQRModal")
      },
      "#modal-backdrop-qr .lang-radio-btn": {
        click: Dispatcher.action("SetLanguage")
      },
      "#btn-toggle-theme": {
        click: Dispatcher.action("ToggleTheme")
      },
      "#btn-toggle-imageless": {
        click: Dispatcher.action("ToggleImageless")
      },
      "#modal-backdrop-qr .ui-scale-radio-btn": {
        click: Dispatcher.action("SetUIScale")
      },
      "#modal-backdrop-qr .scroll-sens-radio-btn": {
        click: Dispatcher.action("SetScrollSensitivity")
      },
      "#modal-backdrop-qr .auto-delay-radio-btn": {
        click: Dispatcher.action("SetAutoChoiceDelay")
      },
      // @dev-only-start
      "#modal-backdrop-qr .btn-extract-embedded-book": {
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
Dispatcher.register("ModalQRController", ModalQRController);
