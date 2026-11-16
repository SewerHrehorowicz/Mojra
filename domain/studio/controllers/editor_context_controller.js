"use strict";

const EditorContextController = {
  data: {
    selectedSceneId: "",
    isScreenplay: false,
    scriptStandard: "pl"
  },

  bindings() {
    return {
      "#editor-context-view": {
        init: this.initContextPanel
      },
      "#editor-book-form textarea.node-meta-textarea": {
        input: this.autoResize
      },
      "#editor-chapter-form": {
        submit: Dispatcher.action("SaveChapterSettings")
      },
      "#editor-chapter-form .chapter-group-title-input": {
        change: Dispatcher.action("RenameChapter")
      },
      "#editor-chapter-form .chapter-start-scene-select .custom-select-option": {
        click: this.selectChapterStartScene
      },
      ".btn-close-book-settings": {
        click: Dispatcher.action("CloseBookSettings")
      },
      "#editor-book-form": {
        submit: Dispatcher.action("SaveBookSettings")
      },
      "#book-settings-title-input": {
        change: Dispatcher.action("RenameBook")
      },
      "#editor-book-form .form-radio-option": {
        click: Dispatcher.action("SaveBookSettings")
      },
      "#editor-book-form input.form-radio-input, #editor-book-form input[name='author'], #book-settings-license-input, #editor-book-form textarea[name='description'], #editor-book-form input[name='export_dir']": {
        change: Dispatcher.action("SaveBookSettings")
      },
      "#book-start-chapter-select .custom-select-option": {
        click: this.selectBookStartChapter
      },
      "#book-start-scene-select .custom-select-option": {
        click: this.selectBookStartScene
      },
      "#editor-book-form .btn-toggle-book-narrated": {
        click: Dispatcher.action("ToggleBookNarrated")
      },
      "#editor-book-form .btn-toggle-book-imageless": {
        click: Dispatcher.action("ToggleBookImageless")
      },
      "#editor-book-form .btn-browse-book-save-folder": {
        click: Dispatcher.action("BrowseBookSaveFolder")
      },
      "#editor-book-form .btn-browse-book-export-folder": {
        click: Dispatcher.action("BrowseBookExportFolder")
      },
      "#btn-delete-selected-scenes": {
        click: Dispatcher.action("PromptDeleteScenes")
      }
    };
  },

  settings: {
    minTextareaHeightPx: 24,
    styleHeightAuto: "auto",
    styleUnitPx: "px",
    selectors: {
      viewport: "#timeline-scroll-viewport"
    },
    selectSceneAction: "SelectStudioScene",
    actions: {
      setChapterStartScene: "SetChapterStartScene",
      setBookStartChapter: "SetBookStartChapter",
      setStartScene: "SetStartScene"
    }
  },

  selectChapterStartScene(params) {
    Dispatcher.post(this.settings.actions.setChapterStartScene, Object.assign({}, params, { start_scene: params.select_value || "" }));
  },

  selectBookStartChapter(params) {
    Dispatcher.post(this.settings.actions.setBookStartChapter, Object.assign({}, params, { start_chapter: params.select_value || "" }));
  },

  selectBookStartScene(params) {
    const val = params.select_value || "";
    Dispatcher.post(this.settings.actions.setStartScene, Object.assign({}, params, { scene: val, start_scene: val }));
  },

  initContextPanel() {
    const cfg = this.settings;
    if (Dispatcher.lastAction === cfg.selectSceneAction && Dispatcher.lastParams && Dispatcher.lastParams.scene) {
      NodeCanvasController.selectSceneNode(Dispatcher.lastParams.scene);
    } else if (document.querySelector(cfg.selectors.viewport)) {
      NodeCanvasController.applyTransform();
    }
  },

  autoResize(_params, element) {
    if (!element) return;
    const cfg = this.settings;
    element.style.height = cfg.styleHeightAuto;
    element.style.height = Math.max(cfg.minTextareaHeightPx, element.scrollHeight || 0) + cfg.styleUnitPx;
  }
};
Dispatcher.register("EditorContextController", EditorContextController);
