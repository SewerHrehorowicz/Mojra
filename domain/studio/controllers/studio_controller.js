"use strict";

const StudioController = {
  data: {
    isHistoryPreview: false,
    historyPreview: false,
    selectedSceneId: "",
    studioSearchOpen: false,
    sceneId: "",
    scenePrevId: "",
    sceneExplicitId: "",
    sceneRef: "",
    sceneTitle: "",
    sceneChapter: "",
    startSceneId: "",
    voicePeak: "",
    historyCount: 0,
    canUndo: false,
    undoTitle: "",
    canRedo: false,
    redoTitle: ""
  },

  bindings() {
    return {
      ".canvas-toolbar .btn-open-new-book": {
        click: Dispatcher.action("OpenNewBookModal")
      },
      "#btn-open-studio-project": {
        click: Dispatcher.action("OpenStudioProjectModal")
      },
      "#btn-open-export-modal": {
        click: Dispatcher.action("OpenExportModal")
      },
      '[data-canvas-zoom="out"]': {
        click: this.zoomOut
      },
      "#btn-zoom-reset": {
        click: this.resetToFirstNode
      },
      '[data-canvas-zoom="in"]': {
        click: this.zoomIn
      },
      '[data-canvas-zoom="fit"]': {
        click: this.fitAllNodes
      },
      "#btn-auto-layout-nodes, .btn-auto-layout-nodes": {
        click: Dispatcher.action("AutoLayoutNodes")
      },
      "#btn-studio-search": {
        click: Dispatcher.action("ToggleStudioSearch")
      },
      "#btn-book-settings": {
        click: Dispatcher.action("ToggleBookSettings")
      },
      "#btn-editor-variables": {
        click: Dispatcher.action("ToggleEditorVariablesPanel")
      },
      "#btn-editor-breakdown": {
        click: Dispatcher.action("ToggleEditorBreakdownPanel")
      },
      ".btn-toggle-history": {
        click: Dispatcher.action("ToggleEditorHistoryPanel")
      },
      "#btn-editor-help": {
        click: Dispatcher.action("OpenEditorHelpModal")
      },
      ".btn-play-history-revision": {
        click: Dispatcher.action("PlayHistoryRevision")
      },
      ".btn-restore-history-revision": {
        click: Dispatcher.action("RestoreHistoryRevision")
      },
      ".btn-exit-history-preview": {
        click: Dispatcher.action("ExitHistoryPreview")
      },
      "#btn-expand-scene-text": {
        click: Dispatcher.action("OpenSceneTextModal")
      },
      "#studio-search-input": {
        change: Dispatcher.action("FilterStudioSearch")
      },
      "#btn-close-studio-search": {
        click: Dispatcher.action("CloseStudioSearch")
      },
      "#studio-search-results .studio-search-edit-btn": {
        click: this.openSearchSceneText
      },
      "#studio-search-results .studio-search-row": {
        click: this.selectSearchScene
      },
      "#btn-toggle-status-warnings": {
        click: Dispatcher.action("ToggleBookWarnings")
      },
      "#btn-new-book, #btn-new-screenplay": {
        click: Dispatcher.action("OpenNewBookModal")
      },
      "#btn-open-studio-project-home": {
        click: Dispatcher.action("OpenStudioProjectModal")
      },
      "#studio-projects-grid .studio-project-cover-wrap, #studio-packed-books-grid .studio-project-cover-wrap": {
        click: Dispatcher.action("OpenStudioProject")
      },
      "#btn-switch-to-reader": {
        click: this.switchToReader
      },
      "[data-select-toggle]": {
        click: this.toggleSelect
      }
    };
  },

  settings: {
    selectors: {
      viewport: "#timeline-scroll-viewport",
      expandSceneBtn: "#btn-expand-scene-text",
      expandSceneTitle: "#scene-text-expand-title",
      selectRoot: "[data-select-root]",
      openSelects: '[data-select-root][data-select-open="true"]',
      selectMenu: "[data-select-menu]",
      selectLabel: "[data-select-label-target]",
      selectOption: "[data-select-value]",
      formInput: "input",
      radioInput: 'input[type="radio"]',
      radioOption: "[data-radio-option]"
    },
    classes: {
      hidden: "hidden",
      active: "active",
      open: "is-open",
      disabled: "is-disabled"
    },
    attrs: {
      dataScene: "data-scene",
      selectOpen: "data-select-open",
      activeWhen: "data-active-when",
      paramVal: "param-value",
      dataVal: "data-select-value",
      paramLbl: "param-label",
      dataLbl: "data-select-label"
    },
    events: {
      change: "change"
    },
    actions: {
      selectScene: "SelectStudioScene",
      openSceneText: "OpenSceneTextModal",
      switchView: "SwitchView"
    },
    i18n: {
      badgeTitle: "search_badge_title",
      badgeSlug: "search_badge_slug",
      badgeChapter: "search_badge_chapter",
      badgeSynopsis: "search_badge_synopsis",
      badgeChoices: "search_badge_choices",
      badgeProse: "search_badge_prose"
    },
    values: {
      trueStr: "true",
      falseStr: "false",
      undefType: "undefined",
      libraryView: "library"
    }
  },

  openSearchSceneText(p) {
    const sc = p.search_open_text || p.scene || "";
    NodeCanvasController.focusNode(sc);
    NodeCanvasController.flushSync();
    Dispatcher.post(this.settings.actions.openSceneText, { scene: sc });
  },

  selectSearchScene(p) {
    const sc = p.search_scene || p.scene || "";
    NodeCanvasController.focusNode(sc);
    NodeCanvasController.flushSync();
    Dispatcher.post(this.settings.actions.selectScene, { scene: sc });
  },

  switchToReader() {
    NodeCanvasController.flushSync();
    Dispatcher.post(this.settings.actions.switchView, {
      view: this.settings.values.libraryView
    });
  },

  zoomIn() {
    NodeCanvasController.zoomIn();
  },

  zoomOut() {
    NodeCanvasController.zoomOut();
  },

  resetToFirstNode(_params, _el, e) {
    NodeCanvasController.resetToFirstNode(Boolean(e), this.data.startSceneId || this.data.selectedSceneId);
  },

  fitAllNodes() {
    NodeCanvasController.fitAllNodes();
  },

  syncViewportVisibility() {
    const s = this.settings;
    const viewport = document.querySelector(s.selectors.viewport);
    if (!viewport) return;
    viewport.classList.remove(s.classes.hidden);
  },

  toggleSelect(_params, el) {
    const s = this.settings;
    const selectRoot = el && el.closest ? el.closest(s.selectors.selectRoot) : null;
    if (!selectRoot || selectRoot.classList.contains(s.classes.disabled)) return;
    const willOpen = selectRoot.getAttribute(s.attrs.selectOpen) !== s.values.trueStr;
    Array.from(document.querySelectorAll(s.selectors.openSelects)).forEach((other) => {
      if (other !== selectRoot) {
        other.removeAttribute(s.attrs.selectOpen);
        other.classList.remove(s.classes.open);
        const otherMenu = other.querySelector(s.selectors.selectMenu);
        if (otherMenu) otherMenu.classList.add(s.classes.hidden);
      }
    });
    if (willOpen) {
      selectRoot.setAttribute(s.attrs.selectOpen, s.values.trueStr);
    } else {
      selectRoot.removeAttribute(s.attrs.selectOpen);
    }
    selectRoot.classList.toggle(s.classes.open, willOpen);
    const menu = selectRoot.querySelector(s.selectors.selectMenu);
    if (menu) menu.classList.toggle(s.classes.hidden, !willOpen);
  },

  selectOption(params, el) {
    if (!el || el.disabled) return;
    const s = this.settings;
    const selectRoot = el.closest ? el.closest(s.selectors.selectRoot) : null;
    if (!selectRoot) return;
    const val = params && params.value !== undefined ? params.value : (el.getAttribute(s.attrs.paramVal) || el.getAttribute(s.attrs.dataVal) || "");
    const lbl = (params && params.label) || el.getAttribute(s.attrs.paramLbl) || el.getAttribute(s.attrs.dataLbl) || (el.textContent || "").trim();
    const inp = selectRoot.querySelector(s.selectors.formInput);
    const lblSpan = selectRoot.querySelector(s.selectors.selectLabel);
    if (lblSpan) lblSpan.textContent = lbl || val;
    Array.from(selectRoot.querySelectorAll(s.selectors.selectOption)).forEach((o) => o.classList.toggle(s.classes.active, o === el));
    selectRoot.removeAttribute(s.attrs.selectOpen);
    selectRoot.classList.remove(s.classes.open);
    const menu = selectRoot.querySelector(s.selectors.selectMenu);
    if (menu) menu.classList.add(s.classes.hidden);
    if (inp) {
      inp.value = val;
      inp.dispatchEvent(new Event(s.events.change, { bubbles: true }));
    }
  },

  toggleFormBtn(_params, el) {
    if (!el || el.disabled) return;
    const s = this.settings;
    const row = el.parentElement;
    const inp = row ? row.querySelector(s.selectors.formInput) : null;
    if (!inp) return;
    const nextVal = inp.value === s.values.trueStr ? s.values.falseStr : s.values.trueStr;
    inp.value = nextVal;
    const activeWhen = el.getAttribute(s.attrs.activeWhen) || s.values.trueStr;
    el.classList.toggle(s.classes.active, nextVal === activeWhen);
  },

  selectRadio(_params, el) {
    if (!el) return;
    const s = this.settings;
    const radio = el.querySelector(s.selectors.radioInput);
    if (!radio || radio.disabled) return;
    const group = el.parentElement;
    if (group) {
      Array.from(group.querySelectorAll(s.selectors.radioOption)).forEach((lbl) => {
        lbl.classList.toggle(s.classes.active, lbl === el);
        const r = lbl.querySelector(s.selectors.radioInput);
        if (r) r.checked = lbl === el;
      });
    }
  },

  onHydrate(frame) {
    const s = this.settings;
    if (frame && frame.voicePeak && typeof EditorVoice !== s.values.undefType) {
      EditorVoice.handlePeakHeader(frame.voicePeak);
    }
    const expandBtn = document.querySelector(s.selectors.expandSceneBtn);
    if (expandBtn && this.data.selectedSceneId) {
      expandBtn.setAttribute(s.attrs.dataScene, this.data.selectedSceneId);
    }
    const expandTitle = document.querySelector(s.selectors.expandSceneTitle);
    if (expandTitle && this.data.sceneTitle) {
      expandTitle.textContent = decodeURIComponent(this.data.sceneTitle.replace(/\+/g, " "));
    }
  }
};
Dispatcher.register("StudioController", StudioController);
