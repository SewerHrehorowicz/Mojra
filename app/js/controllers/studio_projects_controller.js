"use strict";

const StudioProjectsController = {
  data: {},

  bindings() {
    return {
      "#studio-projects-view": {
        click: this.onBackdrop
      },
      ".btn-close-studio-projects": {
        click: Dispatcher.action("CloseStudioProjectModal")
      },
      "#studio-projects-view .studio-project-row": {
        click: Dispatcher.action("OpenStudioProject")
      },
      "#studio-projects-view .btn-browse-studio-project": {
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
Dispatcher.register("StudioProjectsController", StudioProjectsController);
