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
    historyCount: 0,
    canUndo: false,
    undoTitle: "",
    canRedo: false,
    redoTitle: ""
  },

  bindings() {
    return {
      "#studio-canvas": {
        init: this.initCanvas
      },
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
      "#btn-context-reset-view": {
        click: this.fitAllNodes
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
    camera: {
      minZoom: 0.25,
      maxZoom: 2.2,
      defaultZoom: 1,
      zoomStep: 0.15,
      zoomEpsilon: 0.001,
      half: 0.5,
      startFocusXRatio: 0.25,
      fallbackWidth: 900,
      fallbackHeight: 600,
      minStartPan: 32,
      emptyPan: 80,
      fitPad: 96,
      fitPadSides: 2,
      minFitBoxW: 240,
      minFitBoxH: 160,
      maxFitZoom: 1.15
    },
    selectors: {
      viewport: "#timeline-scroll-viewport",
      contextPanel: "#editor-context-panel",
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
      bookId: "data-book-id",
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
    values: {
      trueStr: "true",
      falseStr: "false",
      undefType: "undefined",
      libraryView: "library"
    },
    types: {
      number: "number",
      fn: "function",
      bool: "boolean"
    }
  },

  openSearchSceneText(p) {
    Dispatcher.post(this.settings.actions.openSceneText, {
      scene: p.search_open_text || p.scene || ""
    });
  },

  selectSearchScene(p) {
    Dispatcher.post(this.settings.actions.selectScene, {
      scene: p.search_scene || p.scene || ""
    });
  },

  switchToReader() {
    Dispatcher.post(this.settings.actions.switchView, {
      view: this.settings.values.libraryView
    });
  },

  resolveViewport(candidate) {
    const s = this.settings;
    return candidate && typeof candidate.getBoundingClientRect === s.types.fn
      ? candidate
      : document.querySelector(s.selectors.viewport);
  },

  applyTransform() {
    CanvasOverlays.updateFloatingInputTransform();
    EditorWebglScene.render();
  },

  clampZoom(val) {
    const c = this.settings.camera;
    return Math.min(c.maxZoom, Math.max(c.minZoom, val));
  },

  zoomAtPoint(newZoom, pivotX, pivotY) {
    const viewport = this.resolveViewport();
    if (!viewport) return;
    const c = this.settings.camera;
    const t = this.settings.types;
    const state = CanvasModel.editorCanvasState;
    const targetZoom = this.clampZoom(newZoom);
    const oldZoom = state.zoom || c.defaultZoom;
    if (Math.abs(targetZoom - oldZoom) < c.zoomEpsilon) return;

    const rect = viewport.getBoundingClientRect();
    const px = typeof pivotX === t.number ? pivotX : rect.width * c.half;
    const py = typeof pivotY === t.number ? pivotY : rect.height * c.half;
    const worldX = (px - state.panX) / oldZoom;
    const worldY = (py - state.panY) / oldZoom;

    state.zoom = targetZoom;
    state.panX = Math.round(px - worldX * targetZoom);
    state.panY = Math.round(py - worldY * targetZoom);
    this.applyTransform();
    CanvasDrag.scheduleViewportSave();
  },

  zoomIn() {
    const c = this.settings.camera;
    this.zoomAtPoint((CanvasModel.editorCanvasState.zoom || c.defaultZoom) + c.zoomStep);
  },

  zoomOut() {
    const c = this.settings.camera;
    this.zoomAtPoint((CanvasModel.editorCanvasState.zoom || c.defaultZoom) - c.zoomStep);
  },

  resetToFirstNode(viewport, forceCenter) {
    const vp = this.resolveViewport(viewport);
    if (!vp) return;
    const c = this.settings.camera;
    const shouldForce = typeof forceCenter === this.settings.types.bool ? forceCenter : false;
    const state = CanvasModel.editorCanvasState;
    const gState = CanvasModel.studioGraphState;
    if (!shouldForce && gState.hasSavedView) {
      state.panX = gState.savedPanX;
      state.panY = gState.savedPanY;
      state.zoom = this.clampZoom(gState.savedZoom || c.defaultZoom);
      state.initialized = true;
      this.applyTransform();
      return;
    }
    const vRect = vp.getBoundingClientRect();
    const vw = vRect.width || c.fallbackWidth;
    const vh = vRect.height || c.fallbackHeight;
    const nodes = gState.nodes;
    const firstNode = nodes.find((n) => n.isStart) || nodes[0] || null;
    state.zoom = c.defaultZoom;
    if (firstNode) {
      state.panX = Math.max(c.minStartPan, Math.round(vw * c.startFocusXRatio - firstNode.x));
      state.panY = Math.max(c.minStartPan, Math.round(vh * c.half - (firstNode.y + firstNode.h * c.half)));
    } else {
      state.panX = c.emptyPan;
      state.panY = c.emptyPan;
    }
    state.initialized = true;
    this.applyTransform();
    if (shouldForce) CanvasDrag.scheduleViewportSave();
  },

  fitAllNodes(viewport) {
    const vp = this.resolveViewport(viewport);
    if (!vp) return;
    const c = this.settings.camera;
    const nodes = CanvasModel.studioGraphState.nodes;
    if (!nodes.length) {
      this.resetToFirstNode(vp, true);
      return;
    }
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (let i = 0; i < nodes.length; i++) {
      const n = nodes[i];
      if (n.x < minX) minX = n.x;
      if (n.y < minY) minY = n.y;
      if (n.x + n.w > maxX) maxX = n.x + n.w;
      if (n.y + n.h > maxY) maxY = n.y + n.h;
    }
    const vRect = vp.getBoundingClientRect();
    const vw = vRect.width || c.fallbackWidth;
    const vh = vRect.height || c.fallbackHeight;
    const pad = c.fitPad;
    const boxW = Math.max(c.minFitBoxW, maxX - minX);
    const boxH = Math.max(c.minFitBoxH, maxY - minY);
    const state = CanvasModel.editorCanvasState;
    state.zoom = this.clampZoom(Math.min((vw - pad * c.fitPadSides) / boxW, (vh - pad * c.fitPadSides) / boxH, c.maxFitZoom));
    state.panX = Math.round((vw - boxW * state.zoom) * c.half - minX * state.zoom);
    state.panY = Math.round((vh - boxH * state.zoom) * c.half - minY * state.zoom);
    state.initialized = true;
    this.applyTransform();
    CanvasDrag.scheduleViewportSave();
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

  initCanvas(_params, canvasEl) {
    const s = this.settings;
    if (
      typeof EditorWebgl !== s.values.undefType &&
      EditorWebgl.studioGL.canvas &&
      canvasEl &&
      canvasEl !== EditorWebgl.studioGL.canvas &&
      canvasEl.parentElement
    ) {
      canvasEl.parentElement.replaceChild(EditorWebgl.studioGL.canvas, canvasEl);
    }
    const viewport = document.querySelector(s.selectors.viewport);
    if (!viewport) return;
    const bookId = viewport.getAttribute(s.attrs.bookId) || "";
    const state = CanvasModel.editorCanvasState;
    CanvasModel.parseStudioGraphPayload();
    EditorWires.route();
    if (!state.initialized || state.bookId !== bookId) {
      state.bookId = bookId;
      this.resetToFirstNode(viewport, false);
    } else {
      this.applyTransform();
    }
    this.syncViewportVisibility();
  }
};
Dispatcher.register("StudioController", StudioController);
