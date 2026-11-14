"use strict";

const ModalStudioProjectsController = {
  data: {},

  bindings() {
    return {
      "#modal-backdrop-studio-projects": {
        click: this.onBackdrop
      },
      ".btn-close-studio-projects": {
        click: Dispatcher.action("CloseStudioProjectModal")
      },
      "#modal-studio-project .studio-project-row": {
        click: Dispatcher.action("OpenStudioProject")
      },
      "#modal-studio-project .btn-browse-studio-project": {
        click: Dispatcher.action("BrowseOpenStudioProject")
      },
      "#form-open-studio-project-disk": {
        submit: Dispatcher.action("OpenStudioProjectFromDisk")
      }
    };
  },

  settings: {
    closeAction: "CloseStudioProjectModal"
  },

  onBackdrop(_params, el, e) {
    if (e && e.target === el) {
      Dispatcher.post(this.settings.closeAction, {});
    }
  }
};
Dispatcher.register("ModalStudioProjectsController", ModalStudioProjectsController);
