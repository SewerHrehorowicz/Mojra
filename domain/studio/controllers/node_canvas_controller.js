"use strict";

// NodeCanvasController: Sole public entry point and FDP controller for the Studio node canvas.
// Paired 1:1 with internal/views/node_canvas_view.go and app/templates/node_canvas_view.tpl.
// Owns camera state (panX, panY, zoom), lazy-sync persistence (dirty -> SaveEditorState),
// all DOM event bindings/listeners, and backend action dispatching. Delegates internal
// styling, layout/routing, hit-testing/collision math, and WebGL rendering to private
// modules in app/js/node_canvas/ (NodeCanvasStyle, NodeCanvasLayout, NodeCanvasHelpers, NodeCanvasRenderer).
const NodeCanvasController = {
  data: {
    isHistoryPreview: false,
    selectedSceneId: "",
    startSceneId: "",
    sceneId: "",
    scenePrevId: "",
    bookGraph: "",
    canvasLayout: "",
    layoutHash: "",
    savedPanX: 0,
    savedPanY: 0,
    savedZoom: 1,
    hasView: false,
    resetCanvas: false,
    panX: 0,
    panY: 0,
    zoom: 1,
    dirty: false,
    syncTimer: null,
    bookId: "",
    initialized: false,
    spacePressed: false,
    suppressNextContextMenu: false
  },

  bindings() {
    return {
      "body": {
        keydown: this.onKeydown,
        keyup: this.onKeyup,
        mousedown: this.onMousedown,
        mousemove: this.onMousemove,
        mouseup: this.finishGesture
      },
      "#timeline-scroll-viewport": {
        wheel: this.onWheel,
        contextmenu: this.onContextMenu,
        dblclick: this.onDblclick
      },
      "#studio-canvas": {
        init: this.initCanvas
      },
      "#btn-context-add-node": {
        click: this.contextAddNode
      },
      "#btn-context-add-chapter": {
        click: this.contextAddChapter
      },
      "#btn-context-reset-view": {
        click: this.fitAllNodes
      },
      "#canvas-chapter-input": {
        input: this.onChapterInput,
        blur: this.commitChapterInput,
        keydown: this.onChapterKeydown
      },
      "#canvas-node-title-input": {
        input: this.onNodeTitleInput,
        blur: this.commitNodeTitle,
        keydown: this.onNodeTitleKeydown
      },
      "#canvas-choice-input": {
        input: this.onChoiceLabelInput,
        blur: this.commitChoiceLabel,
        keydown: this.onChoiceLabelKeydown
      }
    };
  },

  settings: {
    metrics: {
      defaultZoom: 1,
      half: 0.5,
      panMoveMinPx: 4,
      pinMoveMinPx: 3,
      chapterDragMinPx: 5,
      marqueeMoveMinPx: 3,
      chapterInSnapDist: 28,
      disconnectMinDist: 28,
      nodeHitPadX: 6,
      nodeHitPadTop: 12,
      nodeHitPadW: 12,
      nodeHitPadH: 18,
      chapterHitPadX: 10,
      chapterHitPadTop: 14,
      chapterHitPadBottom: 6,
      wheelZoomCtrlSens: 0.0045,
      wheelZoomSens: 0.0018,
      ctxMenuOffsetX: 96,
      ctxMenuOffsetY: 20,
      ctxMenuMinEdgePx: 8,
      ctxMenuMaxRightPadPx: 230,
      ctxMenuMaxBottomPadPx: 150
    },
    selectors: {
      viewport: "#timeline-scroll-viewport",
      activeBookTab: "#topbar .topbar-tab[data-book]",
      expandBtn: "#btn-expand-scene-text",
      expandTitle: "#scene-text-expand-title",
      menu: "#canvas-context-menu",
      customSelectMenu: ".custom-select-menu",
      modalBackdrop: ".modal-backdrop",
      editableInputs: "input, textarea, select",
      textInputs: "input, textarea",
      chapterInput: "#canvas-chapter-input",
      nodeTitleInput: "#canvas-node-title-input",
      choiceInput: "#canvas-choice-input",
      modalSceneName: '#form-scene-text input[name="name"]',
      textModal: "#text-editor-view",
      textModalCard: ".player-layout.scene-text-book-layout",
      ignoreMousedown: "#canvas-context-menu, .custom-select-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #editor-empty-state, #btn-expand-scene-text, #studio-search-palette, #editor-status-bar",
      ignoreMousemove: "#canvas-context-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #btn-expand-scene-text",
      ignoreDblclick: "#canvas-context-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #editor-empty-state, #btn-expand-scene-text"
    },
    attrs: {
      book: "data-book",
      scene: "data-scene",
      oldChapter: "data-old-chapter",
      sceneId: "data-scene-id",
      oldTitle: "data-old-title",
      choiceIdx: "data-choice-idx",
      oldLabel: "data-old-label"
    },
    classes: {
      panning: "is-panning",
      hidden: "hidden",
      modeFull: "mode-full",
      hasRightPanel: "has-right-panel"
    },
    actions: {
      saveEditorState: "SaveEditorState",
      selectScene: "SelectStudioScene",
      selectChapter: "SelectStudioChapter",
      createSceneAt: "CreateSceneAt",
      createChapterGroup: "CreateChapterGroup",
      redoEditor: "RedoEditor",
      undoEditor: "UndoEditor",
      promptDeleteScene: "PromptDeleteScene",
      promptDeleteScenes: "PromptDeleteScenes",
      addChoice: "AddChoice",
      selectPrimaryBranch: "SelectPrimaryBranchScene",
      openTextModal: "OpenSceneTextModal",
      promptRemoveChoice: "PromptRemoveChoice",
      reorderChoice: "ReorderChoice",
      connectChoice: "ConnectChoice",
      connectChapter: "ConnectChapter",
      renameChapter: "RenameChapter",
      autoSaveScene: "AutoSaveScene"
    },
    values: {
      defaultBook: "default",
      trueStr: "true",
      undefType: "undefined",
      paramPositions: "positions",
      paramWirePins: "wire_pins",
      emptyPinsVal: "-",
      spaceCode: "Space",
      keyZ: "z",
      codeZ: "KeyZ",
      keyY: "y",
      codeY: "KeyY",
      deleteKey: "Delete",
      backspaceKey: "Backspace",
      enterKey: "Enter",
      escapeKey: "Escape",
      grabCursor: "grab",
      defaultCursor: "default",
      pxSuffix: "px",
      endTarget: "END",
      chapterPrefix: "chapter:",
      firstChoiceIdx: "0",
      bottomSide: "bottom",
      axisH: "H",
      axisV: "V",
      choiceLabelPrefix: "choice_label_",
      choicePrefixKey: "choice_prefix"
    }
  },

  markDirty() {
    this.data.dirty = true;
    if (this.data.syncTimer) clearTimeout(this.data.syncTimer);
    this.data.syncTimer = setTimeout(() => {
      this.data.syncTimer = null;
      if (NodeCanvasHelpers.activeCanvasGesture) this.markDirty();
      else this.flushSync();
    }, NodeCanvasStyle.camera.syncDebounceMs);
  },

  flushSync() {
    if (this.data.syncTimer) {
      clearTimeout(this.data.syncTimer);
      this.data.syncTimer = null;
    }
    if (!this.data.dirty) return null;
    this.data.dirty = false;
    const gState = NodeCanvasLayout.studioGraphState;
    const v = this.settings.values;
    const payload = {
      pan_x: String(Math.round(this.data.panX || 0)),
      pan_y: String(Math.round(this.data.panY || 0)),
      zoom: Number(this.data.zoom || 1).toFixed(NodeCanvasStyle.camera.zoomDecimals)
    };
    if (gState.nodes.length > 0) {
      payload[v.paramPositions] = gState.nodes.map((n) => n.id + ":" + Math.round(n.x) + "," + Math.round(n.y)).join(";");
    }
    const pinParts = [];
    gState.wirePins.forEach((pts, key) => {
      if (pts && pts.length > 0) pinParts.push(key + "=" + pts.map((p) => Math.round(p.x) + "," + Math.round(p.y)).join(";"));
    });
    payload[v.paramWirePins] = pinParts.length > 0 ? pinParts.join("|") : v.emptyPinsVal;
    this.syncCanvasState(payload);
    return payload;
  },

  syncCanvasState(payload) {
    if (!payload || NodeCanvasHelpers.isHistoryPreview()) return;
    Dispatcher.post(this.settings.actions.saveEditorState, payload);
  },

  previewNodeEdit(sceneId, isName, choiceIdx, value) {
    const gState = NodeCanvasLayout.studioGraphState;
    const nd = gState.nodeById.get(sceneId);
    if (!nd) return;
    if (isName) {
      nd.name = value;
      nd._shapedTitle = null;
    } else if (nd.ports && nd.ports[choiceIdx]) {
      nd.ports[choiceIdx].label = value;
      nd.ports[choiceIdx]._shapedChoice = null;
    }
    gState.staticBatchDirty = true;
    NodeCanvasRenderer.render();
  },

  selectSceneNode(sceneId) {
    if (!sceneId) return;
    NodeCanvasHelpers.selectedNodeIds.clear();
    NodeCanvasHelpers.selectedNodeIds.add(sceneId);
    if (document.querySelector(this.settings.selectors.viewport)) {
      NodeCanvasRenderer.render();
    }
  },

  applyTransform() {
    NodeCanvasHelpers.updateFloatingInputTransform();
    NodeCanvasRenderer.render();
  },

  zoomAtPoint(newZoom, pivotX, pivotY) {
    const viewport = document.querySelector(this.settings.selectors.viewport);
    if (!viewport) return;
    const cam = NodeCanvasStyle.camera;
    const clamped = Math.max(cam.minZoom, Math.min(cam.maxZoom, newZoom));
    const oldZoom = this.data.zoom || cam.defaultZoom;
    if (Math.abs(clamped - oldZoom) < cam.zoomEpsilon) return;
    const rect = viewport.getBoundingClientRect();
    const px = pivotX !== undefined ? pivotX - rect.left : rect.width * cam.half;
    const py = pivotY !== undefined ? pivotY - rect.top : rect.height * cam.half;
    this.data.panX = Math.round(px - ((px - this.data.panX) / oldZoom) * clamped);
    this.data.panY = Math.round(py - ((py - this.data.panY) / oldZoom) * clamped);
    this.data.zoom = clamped;
    this.applyTransform();
    this.markDirty();
  },

  zoomIn() {
    this.zoomAtPoint((this.data.zoom || 1) + NodeCanvasStyle.camera.zoomStep);
  },

  zoomOut() {
    this.zoomAtPoint((this.data.zoom || 1) - NodeCanvasStyle.camera.zoomStep);
  },

  getVisibleViewportMetrics(viewport) {
    const cfg = this.settings;
    const cam = NodeCanvasStyle.camera;
    const rect = viewport.getBoundingClientRect();
    const vpW = rect.width || viewport.clientWidth || cam.fallbackWidth;
    const vpH = rect.height || viewport.clientHeight || cam.fallbackHeight;
    let visLeft = 0;
    const textModal = document.querySelector(cfg.selectors.textModal);
    if (textModal && !textModal.classList.contains(cfg.classes.modeFull) && !textModal.classList.contains(cfg.classes.hasRightPanel)) {
      const card = textModal.querySelector(cfg.selectors.textModalCard);
      const cardRight = card ? card.getBoundingClientRect().right - rect.left : 0;
      visLeft = cardRight > 0 ? Math.min(vpW * cam.half, cardRight) : vpW * cam.half;
    }
    return { vpW: vpW, vpH: vpH, visLeft: visLeft, visW: Math.max(cam.minFitBoxW, vpW - visLeft) };
  },

  hasNodeInVisibleViewport(viewport, zoom, panX, panY) {
    const cam = NodeCanvasStyle.camera;
    const nodes = NodeCanvasLayout.studioGraphState.nodes;
    if (!nodes || nodes.length === 0) return true;
    const metrics = this.getVisibleViewportMetrics(viewport);
    const margin = cam.minStartPan;
    for (let idx = 0; idx < nodes.length; idx++) {
      const node = nodes[idx];
      const screenX = node.x * zoom + panX;
      const screenY = node.y * zoom + panY;
      const screenW = (node.w || NodeCanvasStyle.card.w) * zoom;
      const screenH = (node.h || NodeCanvasStyle.card.baseH) * zoom;
      if (
        screenX + screenW > metrics.visLeft + margin &&
        screenX < metrics.vpW - margin &&
        screenY + screenH > margin &&
        screenY < metrics.vpH - margin
      ) {
        return true;
      }
    }
    return false;
  },

  resetToFirstNode(forceCenter, startSceneId) {
    const vp = document.querySelector(this.settings.selectors.viewport);
    if (!vp) return;
    const cam = NodeCanvasStyle.camera, gState = NodeCanvasLayout.studioGraphState;
    if (!forceCenter && this.data.hasView) {
      const savedZoom = Math.max(cam.minZoom, Math.min(cam.maxZoom, this.data.savedZoom || cam.defaultZoom));
      if (this.hasNodeInVisibleViewport(vp, savedZoom, this.data.savedPanX, this.data.savedPanY)) {
        this.data.zoom = savedZoom;
        this.data.panX = this.data.savedPanX;
        this.data.panY = this.data.savedPanY;
        this.applyTransform();
        return;
      }
    }
    this.data.zoom = cam.defaultZoom;
    const metrics = this.getVisibleViewportMetrics(vp);
    const preferredId = metrics.visLeft > 0 ? (this.data.selectedSceneId || startSceneId || this.data.startSceneId) : (startSceneId || this.data.startSceneId || this.data.selectedSceneId);
    const startNode = (preferredId && gState.nodeById.get(preferredId)) || gState.nodes.find((n) => n.isStart) || gState.nodes[0];
    if (startNode) {
      this.data.panX = Math.round(metrics.visLeft + Math.max(cam.minStartPan, metrics.visW * cam.startFocusXRatio) - startNode.x);
      this.data.panY = Math.round(Math.max(cam.minStartPan, metrics.vpH * cam.half - (startNode.y + startNode.h * cam.half)));
    } else {
      this.data.panX = Math.round(metrics.visLeft + cam.emptyPan);
      this.data.panY = cam.emptyPan;
    }
    this.applyTransform();
    if (forceCenter) this.markDirty();
  },

  fitAllNodes() {
    const vp = document.querySelector(this.settings.selectors.viewport);
    if (!vp) return;
    const cam = NodeCanvasStyle.camera, nodes = NodeCanvasLayout.studioGraphState.nodes;
    if (!nodes || nodes.length === 0) {
      this.resetToFirstNode(true, "");
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
    const metrics = this.getVisibleViewportMetrics(vp);
    const boxW = Math.max(cam.minFitBoxW, maxX - minX + cam.fitPad * cam.fitPadSides);
    const boxH = Math.max(cam.minFitBoxH, maxY - minY + cam.fitPad * cam.fitPadSides);
    const fitZoom = Math.max(cam.minZoom, Math.min(cam.maxFitZoom, Math.min(metrics.visW / boxW, metrics.vpH / boxH)));
    this.data.zoom = fitZoom;
    this.data.panX = Math.round(metrics.visLeft + metrics.visW * cam.half - (minX + (maxX - minX) * cam.half) * fitZoom);
    this.data.panY = Math.round(metrics.vpH * cam.half - (minY + (maxY - minY) * cam.half) * fitZoom);
    this.applyTransform();
    this.markDirty();
  },

  focusNode(sceneId) {
    const nd = sceneId && NodeCanvasLayout.studioGraphState.nodeById.get(sceneId);
    if (!nd) return;
    NodeCanvasHelpers.selectedNodeIds.clear();
    NodeCanvasHelpers.selectedNodeIds.add(sceneId);
    const vp = document.querySelector(this.settings.selectors.viewport);
    if (!vp) return;
    const cam = NodeCanvasStyle.camera;
    const metrics = this.getVisibleViewportMetrics(vp);
    const z = this.data.zoom || cam.defaultZoom;
    this.data.panX = Math.round(metrics.visLeft + metrics.visW * cam.half - (nd.x + nd.w * cam.half) * z);
    this.data.panY = Math.round(metrics.vpH * cam.half - (nd.y + nd.h * cam.half) * z);
    this.applyTransform();
    this.markDirty();
  },

  closeContextMenu() {
    const menu = document.querySelector(this.settings.selectors.menu);
    if (menu) menu.classList.add(this.settings.classes.hidden);
  },

  hideAllFloatingInputs(shouldCommit) {
    this.hideFloatingChapterInput(shouldCommit);
    this.hideFloatingNodeTitleInput(shouldCommit);
    this.hideFloatingChoiceInput(shouldCommit);
  },

  handleFloatingInputKeydown(e, hideFn) {
    const v = this.settings.values;
    if (e.key === v.enterKey) {
      e.preventDefault();
      hideFn.call(this, true);
    } else if (e.key === v.escapeKey) {
      e.preventDefault();
      hideFn.call(this, false);
    }
  },

  openFloatingChapterInput(ch) {
    const s = this.settings, inp = document.querySelector(s.selectors.chapterInput);
    if (!inp || !ch) return;
    this.hideFloatingNodeTitleInput(true);
    this.hideFloatingChoiceInput(true);
    NodeCanvasLayout.studioGraphState.editingChapter = ch;
    inp.value = ch.name;
    inp.setAttribute(s.attrs.oldChapter, ch.name);
    NodeCanvasHelpers.openFloatingInput(inp);
  },

  commitChapterInput() {
    const s = this.settings, inp = document.querySelector(s.selectors.chapterInput);
    const gState = NodeCanvasLayout.studioGraphState, ch = gState.editingChapter;
    if (!inp || !ch) return;
    const newName = (inp.value || "").trim(), oldName = (inp.getAttribute(s.attrs.oldChapter) || ch.name || "").trim();
    gState.editingChapter = null;
    gState.staticBatchDirty = true;
    inp.classList.add(s.classes.hidden);
    if (newName && newName !== oldName) {
      ch.name = newName;
      ch.nameLower = newName.toLowerCase();
      this.flushSync();
      Dispatcher.post(s.actions.renameChapter, { old_chapter: oldName, new_chapter: newName });
    }
    NodeCanvasRenderer.render();
  },

  hideFloatingChapterInput(shouldCommit) {
    const s = this.settings, inp = document.querySelector(s.selectors.chapterInput);
    if (!inp || inp.classList.contains(s.classes.hidden)) return;
    if (shouldCommit) { this.commitChapterInput(); return; }
    NodeCanvasLayout.studioGraphState.editingChapter = null;
    NodeCanvasLayout.studioGraphState.staticBatchDirty = true;
    inp.classList.add(s.classes.hidden);
    NodeCanvasRenderer.render();
  },

  onChapterInput(_params, inp) {
    const gState = NodeCanvasLayout.studioGraphState, ch = gState.editingChapter;
    if (!ch || !inp) return;
    ch.name = inp.value || "";
    gState.staticBatchDirty = true;
    NodeCanvasRenderer.render();
  },

  onChapterKeydown(_params, _inp, e) {
    this.handleFloatingInputKeydown(e, this.hideFloatingChapterInput);
  },

  openFloatingNodeTitleInput(nd) {
    const s = this.settings, inp = document.querySelector(s.selectors.nodeTitleInput);
    if (!inp || !nd) return;
    this.hideFloatingChapterInput(true);
    this.hideFloatingChoiceInput(true);
    NodeCanvasLayout.studioGraphState.editingNodeTitle = nd;
    const curTitle = nd.name || nd.id || "";
    inp.value = curTitle;
    inp.setAttribute(s.attrs.sceneId, nd.id);
    inp.setAttribute(s.attrs.oldTitle, curTitle);
    NodeCanvasHelpers.openFloatingInput(inp);
  },

  onNodeTitleInput(_params, inp) {
    const s = this.settings, gState = NodeCanvasLayout.studioGraphState, nd = gState.editingNodeTitle;
    if (!nd || !inp) return;
    nd.name = inp.value || "";
    const expandTitle = document.querySelector(s.selectors.expandTitle);
    if (expandTitle) expandTitle.textContent = nd.name;
    const modalInp = document.querySelector(s.selectors.modalSceneName);
    if (modalInp) modalInp.value = nd.name;
    gState.staticBatchDirty = true;
    NodeCanvasRenderer.render();
  },

  commitNodeTitle() {
    const s = this.settings, inp = document.querySelector(s.selectors.nodeTitleInput);
    const gState = NodeCanvasLayout.studioGraphState, nd = gState.editingNodeTitle;
    if (!inp || !nd) return;
    const newName = (inp.value || "").trim(), oldName = (inp.getAttribute(s.attrs.oldTitle) || nd.name || nd.id || "").trim();
    nd.name = newName || oldName || nd.id;
    gState.editingNodeTitle = null;
    gState.staticBatchDirty = true;
    inp.classList.add(s.classes.hidden);
    NodeCanvasRenderer.render();
    if (newName && newName !== oldName) {
      this.flushSync();
      Dispatcher.post(s.actions.autoSaveScene, { scene_id: nd.id, name: newName });
    }
  },

  hideFloatingNodeTitleInput(shouldCommit) {
    const s = this.settings, inp = document.querySelector(s.selectors.nodeTitleInput);
    if (!inp || inp.classList.contains(s.classes.hidden)) return;
    if (shouldCommit) { this.commitNodeTitle(); return; }
    const gState = NodeCanvasLayout.studioGraphState, nd = gState.editingNodeTitle;
    if (nd) nd.name = (inp.getAttribute(s.attrs.oldTitle) || nd.id || "").trim();
    gState.editingNodeTitle = null;
    gState.staticBatchDirty = true;
    inp.classList.add(s.classes.hidden);
    NodeCanvasRenderer.render();
  },

  onNodeTitleKeydown(_params, _inp, e) {
    this.handleFloatingInputKeydown(e, this.hideFloatingNodeTitleInput);
  },

  openFloatingChoiceInput(nd, choiceIdx) {
    const s = this.settings, inp = document.querySelector(s.selectors.choiceInput);
    if (!inp || !nd || !nd.ports || !nd.ports[choiceIdx]) return;
    this.hideFloatingChapterInput(true);
    this.hideFloatingNodeTitleInput(true);
    NodeCanvasLayout.studioGraphState.editingChoice = { node: nd, idx: choiceIdx };
    const curLabel = nd.ports[choiceIdx].label || UIController.tr(s.values.choicePrefixKey) + (choiceIdx + 1);
    inp.value = curLabel;
    inp.setAttribute(s.attrs.sceneId, nd.id);
    inp.setAttribute(s.attrs.choiceIdx, String(choiceIdx));
    inp.setAttribute(s.attrs.oldLabel, curLabel);
    NodeCanvasHelpers.openFloatingInput(inp);
  },

  onChoiceLabelInput(_params, inp) {
    const gState = NodeCanvasLayout.studioGraphState, ec = gState.editingChoice;
    if (!ec || !ec.node || !inp) return;
    const pt = ec.node.ports && ec.node.ports[ec.idx];
    if (pt) {
      pt.label = inp.value || "";
      gState.staticBatchDirty = true;
      NodeCanvasRenderer.render();
    }
  },

  commitChoiceLabel() {
    const s = this.settings, inp = document.querySelector(s.selectors.choiceInput);
    const gState = NodeCanvasLayout.studioGraphState, ec = gState.editingChoice;
    if (!inp || !ec || !ec.node) return;
    const nd = ec.node, idx = ec.idx, pt = nd.ports && nd.ports[idx];
    const newLabel = (inp.value || "").trim() || UIController.tr(s.values.choicePrefixKey) + (idx + 1);
    const oldLabel = (inp.getAttribute(s.attrs.oldLabel) || "").trim();
    if (pt) pt.label = newLabel;
    gState.editingChoice = null;
    gState.staticBatchDirty = true;
    inp.classList.add(s.classes.hidden);
    NodeCanvasRenderer.render();
    if (newLabel !== oldLabel) {
      this.flushSync();
      Dispatcher.post(s.actions.autoSaveScene, { scene_id: nd.id, [s.values.choiceLabelPrefix + idx]: newLabel });
    }
  },

  hideFloatingChoiceInput(shouldCommit) {
    const s = this.settings, inp = document.querySelector(s.selectors.choiceInput);
    if (!inp || inp.classList.contains(s.classes.hidden)) return;
    if (shouldCommit) { this.commitChoiceLabel(); return; }
    const gState = NodeCanvasLayout.studioGraphState, ec = gState.editingChoice;
    if (ec && ec.node && ec.node.ports && ec.node.ports[ec.idx]) {
      ec.node.ports[ec.idx].label = (inp.getAttribute(s.attrs.oldLabel) || UIController.tr(s.values.choicePrefixKey) + (ec.idx + 1)).trim();
    }
    gState.editingChoice = null;
    gState.staticBatchDirty = true;
    inp.classList.add(s.classes.hidden);
    NodeCanvasRenderer.render();
  },

  onChoiceLabelKeydown(_params, _inp, e) {
    this.handleFloatingInputKeydown(e, this.hideFloatingChoiceInput);
  },

  createSceneAtWorldCoords(wx, wy) {
    this.closeContextMenu();
    const ch = NodeCanvasHelpers.findClosestChapterAtWorldCoords({ x: wx, y: wy });
    const pos = NodeCanvasHelpers.resolveNonOverlappingNodeCoords({
      x: NodeCanvasGeometry.snapToEditorGrid(wx),
      y: NodeCanvasGeometry.snapToEditorGrid(wy)
    }, ch);
    this.flushSync();
    Dispatcher.post(this.settings.actions.createSceneAt, { x: pos.x, y: pos.y, chapter: ch });
  },

  contextAddNode() {
    this.createSceneAtWorldCoords(NodeCanvasHelpers.pendingContextMenuWorldPos.x, NodeCanvasHelpers.pendingContextMenuWorldPos.y);
  },

  contextAddChapter() {
    this.closeContextMenu();
    this.flushSync();
    Dispatcher.post(this.settings.actions.createChapterGroup, {
      x: NodeCanvasGeometry.snapToEditorGrid(NodeCanvasHelpers.pendingContextMenuWorldPos.x),
      y: NodeCanvasGeometry.snapToEditorGrid(NodeCanvasHelpers.pendingContextMenuWorldPos.y)
    });
  },

  removeWirePinAt(wire, pinIdx) {
    if (!wire || !wire.pins || pinIdx < 0 || pinIdx >= wire.pins.length) return false;
    wire.pins.splice(pinIdx, 1);
    NodeCanvasRouter.rerouteAfterWirePinMutation(wire);
    this.markDirty();
    return true;
  },

  beginSharedHitDragGesture(hit, ptrCtx) {
    const s = this.settings, ht = NodeCanvasHelpers.hitTypes, gt = NodeCanvasHelpers.gestures;
    if (hit.type === ht.chapterOutPort) {
      const ch = hit.chapter, op = hit.outPort, opSide = op.side || s.values.bottomSide;
      NodeCanvasHelpers.activeCanvasGesture = {
        type: gt.chapterWireConnect, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
        fromChapterObj: ch, activeChapterOutPort: op, outPortSide: opSide, isBottomPort: opSide === s.values.bottomSide,
        fromChapter: ch.ref || ch.name, oldTargetChapter: op.target || "",
        startX: op.x, startY: op.y, curX: op.x, curY: op.y, hoveredTargetChapter: ""
      };
      NodeCanvasLayout.studioGraphState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
      return true;
    }
    if (hit.type === ht.sceneOutPort) {
      const nd = hit.node, pt = hit.port;
      NodeCanvasHelpers.activeCanvasGesture = {
        type: gt.wireConnect, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
        fromNode: nd, sourcePort: pt, fromSceneId: nd.id, fromChapter: nd.chapter,
        choiceIdx: String(hit.portIdx), isBottomPort: Boolean(hit.isBottom),
        hasExistingTarget: Boolean((pt.targetId && pt.targetId !== s.values.endTarget) || pt.targetChapter),
        startX: hit.x, startY: hit.y, curX: hit.x, curY: hit.y, hoveredTargetId: ""
      };
      NodeCanvasLayout.studioGraphState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
      return true;
    }
    if (hit.type === ht.chapterGrip || hit.type === ht.chapterTitle) {
      const ch = hit.chapter;
      if (!ch) return false;
      if (NodeCanvasHelpers.isHistoryPreview()) {
        Dispatcher.post(s.actions.selectChapter, { chapter: ch.name });
        return true;
      }
      const chNodes = ch.nodes && ch.nodes.length > 0
        ? ch.nodes
        : NodeCanvasLayout.studioGraphState.nodes.filter((nd) => nd.chapterLower === ch.nameLower || nd.chapterLower === ch.refLower);
      if (chNodes.length === 0) return false;
      const clickedTitleArea = hit.type === ht.chapterTitle;
      NodeCanvasHelpers.activeCanvasGesture = {
        type: gt.chapterDrag, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
        chapter: ch, chapterName: ch.name, clickedTitleArea: clickedTitleArea, dragConfirmed: !clickedTitleArea,
        items: chNodes.map((nd) => ({ node: nd, sceneId: nd.id, startX: nd.x, startY: nd.y })),
        startClientX: ptrCtx.clientX, startClientY: ptrCtx.clientY, lastValidDx: 0, lastValidDy: 0, moved: false
      };
      NodeCanvasRenderer.render();
      return true;
    }
    if (hit.type === ht.nodeGrip) {
      const nd = hit.node, sceneId = nd.id, selected = NodeCanvasHelpers.selectedNodeIds;
      if (ptrCtx.isMultiModifier) {
        if (selected.has(sceneId)) {
          selected.delete(sceneId);
          NodeCanvasRenderer.render();
          if (selected.size === 1) {
            this.flushSync();
            Dispatcher.post(s.actions.selectScene, { scene: Array.from(selected)[0] });
          }
          return true;
        }
        if (sceneId) selected.add(sceneId);
      } else if (!selected.has(sceneId)) {
        selected.clear();
        if (sceneId) selected.add(sceneId);
      }
      if (NodeCanvasHelpers.isHistoryPreview()) {
        NodeCanvasRenderer.render();
        if (selected.size === 1) Dispatcher.post(s.actions.selectScene, { scene: Array.from(selected)[0] });
        return true;
      }
      const dragNodes = NodeCanvasLayout.studioGraphState.nodes.filter((n) => selected.has(n.id));
      if (dragNodes.length === 0) dragNodes.push(nd);
      NodeCanvasHelpers.activeCanvasGesture = {
        type: gt.nodeDrag, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
        node: nd, sceneId: sceneId, isMultiModifier: ptrCtx.isMultiModifier,
        items: dragNodes.map((n) => ({ node: n, sceneId: n.id, startX: n.x, startY: n.y })),
        startClientX: ptrCtx.clientX, startClientY: ptrCtx.clientY,
        lastValidDx: 0, lastValidDy: 0, ghostDx: 0, ghostDy: 0, ghostState: null, moved: false
      };
      NodeCanvasLayout.studioGraphState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
      return true;
    }
    if (hit.type === ht.choiceGrip) {
      const nd = hit.node;
      if (!nd || !nd.ports || nd.ports.length < 2 || NodeCanvasHelpers.isHistoryPreview()) return false;
      NodeCanvasHelpers.selectedNodeIds.clear();
      if (nd.id) NodeCanvasHelpers.selectedNodeIds.add(nd.id);
      if (ptrCtx.viewport) ptrCtx.viewport.classList.add(s.classes.panning);
      NodeCanvasHelpers.activeCanvasGesture = {
        type: gt.choiceDrag, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
        node: nd, sceneId: nd.id, startIdx: hit.choiceIdx, currentIdx: hit.choiceIdx,
        startClientX: ptrCtx.clientX, startClientY: ptrCtx.clientY, moved: false
      };
      NodeCanvasLayout.studioGraphState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
      return true;
    }
    if (hit.type === ht.wirePin || hit.type === ht.wire) {
      if (NodeCanvasHelpers.isHistoryPreview() || !hit.wire) return false;
      const wire = hit.wire;
      if (!wire.pins) wire.pins = [];
      const pts = wire.pts && wire.pts.length >= 2 ? wire.pts : wire.basePts || [];
      let segIdx = Number.isFinite(hit.segIdx) ? hit.segIdx : 0;
      if (hit.type === ht.wirePin && hit.pin && Number.isFinite(hit.pin.segIdx)) segIdx = hit.pin.segIdx;
      if (segIdx < 0 || segIdx >= pts.length - 1) segIdx = Math.max(0, pts.length - 2);
      const p0 = pts[segIdx] || { x: wire.x1, y: wire.y1 }, p1 = pts[segIdx + 1] || { x: wire.x2, y: wire.y2 };
      const segAxis = Math.abs(p0.y - p1.y) <= Math.abs(p0.x - p1.x) ? s.values.axisH : s.values.axisV;
      let existingIdx = hit.type === ht.wirePin && Number.isFinite(hit.pinIdx) ? hit.pinIdx : -1;
      if (existingIdx < 0) {
        for (let i = 0; i < wire.pins.length; i++) {
          if (wire.pins[i] && wire.pins[i].segIdx === segIdx) { existingIdx = i; break; }
        }
      }
      const isNewPin = existingIdx < 0;
      const startWx = isNewPin ? ptrCtx.wx : wire.pins[existingIdx].x;
      const startWy = isNewPin ? ptrCtx.wy : wire.pins[existingIdx].y;
      let pinIdx = existingIdx;
      if (isNewPin) {
        const newArc = NodeCanvasGeometry.computePointArcLengthAlongPolyline(pts, { x: startWx, y: startWy });
        pinIdx = wire.pins.length;
        for (let i = 0; i < wire.pins.length; i++) {
          if (newArc <= NodeCanvasGeometry.computePointArcLengthAlongPolyline(pts, wire.pins[i])) {
            pinIdx = i; break;
          }
        }
      }
      NodeCanvasHelpers.activeCanvasGesture = {
        type: gt.wirePinDrag, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
        wire: wire, pinIdx: pinIdx, segIdx: segIdx, axis: segAxis,
        fixedX: Math.round((p0.x + p1.x) * s.metrics.half), fixedY: Math.round((p0.y + p1.y) * s.metrics.half),
        isNewPin: isNewPin, inserted: !isNewPin,
        startClientX: ptrCtx.clientX, startClientY: ptrCtx.clientY, startWx: startWx, startWy: startWy, moved: false
      };
      NodeCanvasLayout.studioGraphState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
      return true;
    }
    return false;
  },

  updateMove(clientX, clientY) {
    const g = NodeCanvasHelpers.activeCanvasGesture;
    if (!g) return;
    const s = this.settings, m = s.metrics, gt = NodeCanvasHelpers.gestures;
    const gState = NodeCanvasLayout.studioGraphState, gridStep = NodeCanvasStyle.gap.grid, card = NodeCanvasStyle.card;

    if (g.type === gt.pan) {
      this.data.panX = g.startPanX + (clientX - g.startClientX);
      this.data.panY = g.startPanY + (clientY - g.startClientY);
      g.moved = true;
      this.applyTransform();
      return;
    }
    if (g.type === gt.touchPan || g.type === gt.rightPan) {
      const dx = clientX - g.startClientX, dy = clientY - g.startClientY;
      if (!g.moved && Math.hypot(dx, dy) > m.panMoveMinPx) {
        g.moved = true;
        if (g.type === gt.rightPan) this.data.suppressNextContextMenu = true;
        if (g.viewport) g.viewport.classList.add(s.classes.panning);
      }
      if (g.moved) {
        this.data.panX = g.startPanX + dx;
        this.data.panY = g.startPanY + dy;
        this.applyTransform();
      }
      return;
    }
    if (g.type === gt.choiceDrag) {
      const nd = g.node;
      if (!nd || !nd.ports || nd.ports.length < 2) return;
      const ptWorld = NodeCanvasHelpers.clientToStudioWorld(g.viewport, clientX, clientY);
      const targetIdx = Math.max(0, Math.min(nd.ports.length - 1, Math.floor((ptWorld.wy - (nd.y + card.headerH)) / card.choiceH)));
      if (targetIdx !== g.currentIdx && NodeCanvasModel.moveNodePortInMemory(nd, g.currentIdx, targetIdx)) {
        g.currentIdx = targetIdx;
        g.moved = true;
      }
      return;
    }
    if (g.type === gt.wirePinDrag) {
      if (!g.moved && Math.hypot(clientX - g.startClientX, clientY - g.startClientY) < m.pinMoveMinPx) return;
      const wire = g.wire;
      if (!wire) return;
      const ptWorld = NodeCanvasHelpers.clientToStudioWorld(g.viewport, clientX, clientY);
      const isHoriz = g.axis === s.values.axisH;
      const snappedX = isHoriz ? g.fixedX : NodeCanvasGeometry.snapToWirePinGrid(ptWorld.wx);
      const snappedY = isHoriz ? NodeCanvasGeometry.snapToWirePinGrid(ptWorld.wy) : g.fixedY;
      if (!g.inserted) {
        wire.pins.splice(g.pinIdx, 0, { x: snappedX, y: snappedY, axis: g.axis, segIdx: g.segIdx });
        g.inserted = true;
      } else if (wire.pins[g.pinIdx]) {
        const curPin = wire.pins[g.pinIdx];
        if (curPin.x === snappedX && curPin.y === snappedY && g.moved) return;
        curPin.x = snappedX; curPin.y = snappedY; curPin.axis = g.axis;
      }
      g.moved = true;
      NodeCanvasRouter.rerouteAfterWirePinMutation(wire);
      return;
    }
    if (g.type === gt.nodeDrag) {
      const z = this.data.zoom || m.defaultZoom;
      const rawDx = NodeCanvasGeometry.snapToEditorGrid((clientX - g.startClientX) / z);
      const rawDy = NodeCanvasGeometry.snapToEditorGrid((clientY - g.startClientY) / z);
      if (rawDx === g.lastSnappedDx && rawDy === g.lastSnappedDy) return;
      g.lastSnappedDx = rawDx; g.lastSnappedDy = rawDy; g.ghostDx = rawDx; g.ghostDy = rawDy;
      g.ghostState = NodeCanvasHelpers.evaluateNodeDragGhost(g.items || [], rawDx, rawDy);
      if (Math.abs(rawDx) >= gridStep || Math.abs(rawDy) >= gridStep) g.moved = true;
      gState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
      return;
    }
    if (g.type === gt.chapterDrag) {
      if (!g.dragConfirmed) {
        if (Math.hypot(clientX - g.startClientX, clientY - g.startClientY) < m.chapterDragMinPx) return;
        g.dragConfirmed = true;
        this.hideFloatingChapterInput(false);
      }
      const z = this.data.zoom || m.defaultZoom;
      const rawDx = NodeCanvasGeometry.snapToEditorGrid((clientX - g.startClientX) / z);
      const rawDy = NodeCanvasGeometry.snapToEditorGrid((clientY - g.startClientY) / z);
      if (rawDx === g.lastSnappedDx && rawDy === g.lastSnappedDy) return;
      g.lastSnappedDx = rawDx; g.lastSnappedDy = rawDy;
      const valid = NodeCanvasHelpers.resolveValidChapterDragDelta(g, { x: rawDx, y: rawDy });
      if (valid.dx === g.lastValidDx && valid.dy === g.lastValidDy && g.moved) return;
      g.lastValidDx = valid.dx; g.lastValidDy = valid.dy;
      if (Math.abs(valid.dx) >= gridStep || Math.abs(valid.dy) >= gridStep) g.moved = true;
      NodeCanvasModel.applyIncrementalNodeMove(g.items || [], { x: valid.dx, y: valid.dy });
      return;
    }
    if (g.type === gt.marqueeSelect) {
      if (!g.moved && Math.hypot(clientX - g.startClientX, clientY - g.startClientY) > m.marqueeMoveMinPx) g.moved = true;
      if (!g.moved) return;
      const vpRect = g.viewport.getBoundingClientRect();
      const x1 = Math.min(g.startClientX, clientX) - vpRect.left, y1 = Math.min(g.startClientY, clientY) - vpRect.top;
      const w = Math.abs(clientX - g.startClientX), h = Math.abs(clientY - g.startClientY);
      g.marqueeRect = { x: x1, y: y1, w: w, h: h };
      const z = this.data.zoom || m.defaultZoom, px = this.data.panX || 0, py = this.data.panY || 0;
      const wL = (x1 - px) / z, wR = (x1 + w - px) / z, wT = (y1 - py) / z, wB = (y1 + h - py) / z;
      NodeCanvasHelpers.selectedNodeIds.clear();
      g.baseSelection.forEach((id) => NodeCanvasHelpers.selectedNodeIds.add(id));
      for (let i = 0; i < gState.nodes.length; i++) {
        const nd = gState.nodes[i];
        if (nd.right >= wL && nd.left <= wR && nd.bottom >= wT && nd.top <= wB) NodeCanvasHelpers.selectedNodeIds.add(nd.id);
      }
      gState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
      return;
    }
    if (g.type === gt.wireConnect) {
      const ptWorld = NodeCanvasHelpers.clientToStudioWorld(g.viewport, clientX, clientY);
      let cx = ptWorld.wx, cy = ptWorld.wy, hoveredId = "";
      const fromChLower = (g.fromChapter || "").trim().toLowerCase();
      for (let i = 0; i < gState.nodes.length; i++) {
        const nd = gState.nodes[i];
        if (nd.id === g.fromSceneId) continue;
        if (NodeCanvasGeometry.inRect({ x: cx, y: cy }, { x: nd.x - m.nodeHitPadX, y: nd.y - m.nodeHitPadTop, w: nd.w + m.nodeHitPadW, h: nd.h + m.nodeHitPadH })) {
          if (!fromChLower || !nd.chapterLower || nd.chapterLower === fromChLower) {
            hoveredId = nd.id; cx = nd.inX; cy = nd.inY;
          } else {
            const tf = gState.chapterByKey.get(nd.chapterLower);
            if (tf && !tf.hidden && !tf.isRoot) {
              hoveredId = s.values.chapterPrefix + tf.ref; cx = tf.inX; cy = tf.inY;
            }
          }
          break;
        }
      }
      g.curX = cx; g.curY = cy; g.hoveredTargetId = hoveredId;
      gState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
      return;
    }
    if (g.type === gt.chapterWireConnect) {
      const ptWorld = NodeCanvasHelpers.clientToStudioWorld(g.viewport, clientX, clientY);
      let cx = ptWorld.wx, cy = ptWorld.wy, hoveredCh = "";
      const fromChLower = (g.fromChapter || "").trim().toLowerCase();
      for (let i = 0; i < gState.chapters.length; i++) {
        const fr = gState.chapters[i];
        if (fr.hidden || fr.isRoot || fr.nameLower === fromChLower || fr.refLower === fromChLower) continue;
        if (Math.hypot(cx - fr.inX, cy - fr.inY) <= m.chapterInSnapDist || (cx >= fr.x - m.chapterHitPadX && cx <= fr.x + fr.w + m.chapterHitPadX && cy >= fr.y - m.chapterHitPadTop && cy <= fr.y + fr.h + m.chapterHitPadBottom)) {
          hoveredCh = fr.ref; cx = fr.inX; cy = fr.inY; break;
        }
      }
      g.curX = cx; g.curY = cy; g.hoveredTargetChapter = hoveredCh;
      gState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
    }
  },

  finishGesture() {
    const g = NodeCanvasHelpers.activeCanvasGesture;
    if (!g || g.isTouch) return;
    NodeCanvasHelpers.activeCanvasGesture = null;
    const s = this.settings, gt = NodeCanvasHelpers.gestures;
    const vp = g.viewport || document.querySelector(s.selectors.viewport);
    if (vp && !this.data.spacePressed) vp.classList.remove(s.classes.panning);
    NodeCanvasLayout.studioGraphState.overlayBatchDirty = true;

    if (g.type === gt.pan || (g.type === gt.rightPan && g.moved)) {
      if (g.moved) this.markDirty();
      return;
    }
    if (g.type === gt.choiceDrag) {
      NodeCanvasRenderer.render();
      if (g.node && g.startIdx !== g.currentIdx) {
        this.flushSync();
        Dispatcher.post(s.actions.reorderChoice, { scene: g.sceneId, from_idx: String(g.startIdx), to_idx: String(g.currentIdx) });
      }
      return;
    }
    if (g.type === gt.wirePinDrag) {
      const wire = g.wire;
      if (wire && g.inserted && g.moved && wire.pins && wire.pins[g.pinIdx]) {
        NodeCanvasRouter.rerouteAfterWirePinMutation(wire);
        this.markDirty();
      } else {
        NodeCanvasRenderer.render();
      }
      return;
    }
    if (g.type === gt.nodeDrag) {
      const items = g.items || [];
      if (g.moved && items.length > 0) {
        const ghost = g.ghostState || NodeCanvasHelpers.evaluateNodeDragGhost(items, g.ghostDx || 0, g.ghostDy || 0);
        if (ghost.valid && (ghost.dx !== 0 || ghost.dy !== 0)) {
          NodeCanvasModel.applyIncrementalNodeMove(items, { x: ghost.dx, y: ghost.dy });
          this.markDirty();
        }
      } else if (!g.moved && g.sceneId) {
        this.flushSync();
        Dispatcher.post(s.actions.selectScene, { scene: g.sceneId });
      }
      NodeCanvasRenderer.render();
      return;
    }
    if (g.type === gt.chapterDrag) {
      const items = g.items || [];
      NodeCanvasRenderer.render();
      if (g.moved && items.length > 0) {
        this.markDirty();
      } else if (!g.moved && g.chapter) {
        this.flushSync();
        Dispatcher.post(s.actions.selectChapter, { chapter: g.chapter.name });
      }
      return;
    }
    if (g.type === gt.wireConnect) {
      let toScene = "", choiceIdx = g.choiceIdx;
      if (g.hoveredTargetId && g.fromSceneId && g.hoveredTargetId !== g.fromSceneId) {
        toScene = g.hoveredTargetId;
      } else if (g.isBottomPort && g.fromSceneId && !g.hoveredTargetId && g.hasExistingTarget && Math.hypot(g.curX - g.startX, g.curY - g.startY) > s.metrics.disconnectMinDist) {
        toScene = s.values.endTarget;
        choiceIdx = s.values.firstChoiceIdx;
      }
      if (toScene) {
        this.flushSync();
        Dispatcher.post(s.actions.connectChoice, { from_scene: g.fromSceneId, choice_idx: choiceIdx, to_scene: toScene });
      } else {
        NodeCanvasRenderer.render();
      }
      return;
    }
    if (g.type === gt.chapterWireConnect) {
      if (g.hoveredTargetChapter && g.fromChapter) {
        this.flushSync();
        Dispatcher.post(s.actions.connectChapter, { from_chapter: g.fromChapter, old_target_chapter: g.oldTargetChapter || "", to_chapter: g.hoveredTargetChapter });
      } else if (g.fromChapter && !g.hoveredTargetChapter && g.oldTargetChapter && Math.hypot(g.curX - g.startX, g.curY - g.startY) > s.metrics.disconnectMinDist) {
        this.flushSync();
        Dispatcher.post(s.actions.connectChapter, { from_chapter: g.fromChapter, old_target_chapter: g.oldTargetChapter, to_chapter: s.values.endTarget });
      } else {
        NodeCanvasRenderer.render();
      }
      return;
    }
    NodeCanvasRenderer.render();
  },

  onKeydown(_params, _el, e) {
    const s = this.settings, v = s.values, sel = s.selectors, act = s.actions;
    if (e.code === v.spaceCode && !e.target.closest(sel.editableInputs)) {
      this.data.spacePressed = true;
      const vp = document.querySelector(sel.viewport);
      if (vp) vp.classList.add(s.classes.panning);
    }
    if ((e.ctrlKey || e.metaKey) && !e.altKey && document.querySelector(sel.viewport)) {
      const keyLower = (e.key || "").toLowerCase();
      const isUndo = (keyLower === v.keyZ || e.code === v.codeZ) && !e.shiftKey;
      const isRedo = ((keyLower === v.keyZ || e.code === v.codeZ) && e.shiftKey) || ((keyLower === v.keyY || e.code === v.codeY) && !e.shiftKey);
      if (isUndo || isRedo) {
        e.preventDefault();
        this.flushSync();
        Dispatcher.post(isRedo ? act.redoEditor : act.undoEditor, {});
        return;
      }
    }
    if ((e.key === v.deleteKey || e.key === v.backspaceKey) && !e.target.closest(sel.editableInputs) && !document.querySelector(sel.modalBackdrop) && document.querySelector(sel.viewport) && !NodeCanvasHelpers.isHistoryPreview()) {
      if (NodeCanvasHelpers.selectedNodeIds.size === 1) {
        e.preventDefault();
        this.flushSync();
        Dispatcher.post(act.promptDeleteScene, { scene: Array.from(NodeCanvasHelpers.selectedNodeIds)[0] });
      } else if (NodeCanvasHelpers.selectedNodeIds.size > 1) {
        e.preventDefault();
        this.flushSync();
        Dispatcher.post(act.promptDeleteScenes, { scenes: Array.from(NodeCanvasHelpers.selectedNodeIds).join(",") });
      }
    }
  },

  onKeyup(_params, _el, e) {
    const s = this.settings;
    if (e.code === s.values.spaceCode) {
      this.data.spacePressed = false;
      if (!NodeCanvasHelpers.activeCanvasGesture) {
        const vp = document.querySelector(s.selectors.viewport);
        if (vp) vp.classList.remove(s.classes.panning);
      }
    }
  },

  onWheel(_params, viewport, e) {
    const s = this.settings, m = s.metrics;
    if (!viewport || e.target.closest(s.selectors.customSelectMenu)) return;
    e.preventDefault();
    const sensitivity = e.ctrlKey || e.metaKey ? m.wheelZoomCtrlSens : m.wheelZoomSens;
    this.zoomAtPoint((this.data.zoom || m.defaultZoom) * Math.exp(-e.deltaY * sensitivity), e.clientX, e.clientY);
  },

  onMousedown(_params, _el, e) {
    const s = this.settings, sel = s.selectors, act = s.actions;
    const ht = NodeCanvasHelpers.hitTypes, gt = NodeCanvasHelpers.gestures;
    const viewport = e.target && e.target.closest ? e.target.closest(sel.viewport) : null;
    if (!viewport) {
      this.closeContextMenu();
      this.hideAllFloatingInputs(true);
      return;
    }
    if (e.target.closest(sel.ignoreMousedown)) return;
    this.closeContextMenu();
    this.hideAllFloatingInputs(true);

    if (e.button === 1 || (e.button === 0 && this.data.spacePressed)) {
      e.preventDefault();
      viewport.classList.add(s.classes.panning);
      NodeCanvasHelpers.activeCanvasGesture = {
        type: gt.pan, viewport: viewport, startClientX: e.clientX, startClientY: e.clientY,
        startPanX: this.data.panX, startPanY: this.data.panY
      };
      return;
    }
    if (e.button === 2 && !e.target.closest(sel.textInputs)) {
      NodeCanvasHelpers.activeCanvasGesture = {
        type: gt.rightPan, viewport: viewport, startClientX: e.clientX, startClientY: e.clientY,
        startPanX: this.data.panX, startPanY: this.data.panY, moved: false
      };
      return;
    }
    if (e.button !== 0) return;

    const ptrCtx = NodeCanvasHelpers.buildPointerContext(viewport, e, false);
    const hit = NodeCanvasHelpers.hitTest(ptrCtx.wx, ptrCtx.wy);
    if (this.beginSharedHitDragGesture(hit, ptrCtx)) {
      e.preventDefault();
      return;
    }
    if (hit.type === ht.addChoiceBtn) {
      e.preventDefault();
      this.flushSync();
      Dispatcher.post(act.addChoice, { scene: hit.node.id });
      return;
    }
    if (hit.type === ht.branchRadio && hit.node && hit.node.branchRole === NodeCanvasHelpers.tokens.roleSecondary) {
      e.preventDefault();
      this.flushSync();
      Dispatcher.post(act.selectPrimaryBranch, { scene: hit.node.id, parent: hit.node.branchParentId || "" });
      return;
    }
    if (hit.type === ht.editSceneBtn) {
      e.preventDefault();
      NodeCanvasHelpers.selectedNodeIds.clear();
      if (hit.node && hit.node.id) NodeCanvasHelpers.selectedNodeIds.add(hit.node.id);
      NodeCanvasRenderer.render();
      this.flushSync();
      Dispatcher.post(act.openTextModal, { scene: hit.node.id });
      return;
    }
    if (hit.type === ht.deleteSceneBtn) {
      e.preventDefault();
      this.flushSync();
      Dispatcher.post(act.promptDeleteScene, { scene: hit.node.id });
      return;
    }
    if (hit.type === ht.choiceTrashBtn) {
      e.preventDefault();
      this.flushSync();
      Dispatcher.post(act.promptRemoveChoice, { scene: hit.node.id, choice_idx: String(hit.choiceIdx) });
      return;
    }
    if (hit.type === ht.choiceLabel || hit.type === ht.nodeTitle) {
      e.preventDefault();
      NodeCanvasHelpers.selectedNodeIds.clear();
      if (hit.node && hit.node.id) NodeCanvasHelpers.selectedNodeIds.add(hit.node.id);
      if (hit.type === ht.choiceLabel) this.openFloatingChoiceInput(hit.node, hit.choiceIdx);
      else this.openFloatingNodeTitleInput(hit.node);
      return;
    }
    if (hit.type === ht.nodeCard) {
      e.preventDefault();
      const sceneId = hit.node.id;
      NodeCanvasHelpers.selectedNodeIds.clear();
      if (sceneId) NodeCanvasHelpers.selectedNodeIds.add(sceneId);
      NodeCanvasRenderer.render();
      this.flushSync();
      Dispatcher.post(act.selectScene, { scene: sceneId });
      return;
    }

    e.preventDefault();
    const baseSelection = ptrCtx.isMultiModifier ? new Set(NodeCanvasHelpers.selectedNodeIds) : new Set();
    if (!ptrCtx.isMultiModifier) {
      NodeCanvasHelpers.selectedNodeIds.clear();
      NodeCanvasRenderer.render();
    }
    NodeCanvasHelpers.activeCanvasGesture = {
      type: gt.marqueeSelect, viewport: viewport,
      startClientX: e.clientX, startClientY: e.clientY, baseSelection: baseSelection, marqueeRect: null, moved: false
    };
  },

  onMousemove(_params, _el, e) {
    if (NodeCanvasHelpers.activeCanvasGesture) {
      if (!NodeCanvasHelpers.activeCanvasGesture.isTouch) this.updateMove(e.clientX, e.clientY);
      return;
    }
    const s = this.settings, v = s.values, sel = s.selectors, ht = NodeCanvasHelpers.hitTypes;
    const viewport = e.target && e.target.closest ? e.target.closest(sel.viewport) : null;
    const gState = NodeCanvasLayout.studioGraphState;
    if (!viewport || !viewport.contains(e.target) || e.target.closest(sel.ignoreMousemove)) {
      if (gState.hoveredItem) {
        gState.hoveredItem = null;
        gState.overlayBatchDirty = true;
        NodeCanvasRenderer.render();
      }
      return;
    }
    const ptWorld = NodeCanvasHelpers.clientToStudioWorld(viewport, e.clientX, e.clientY);
    const hit = NodeCanvasHelpers.hitTest(ptWorld.wx, ptWorld.wy);
    viewport.style.cursor = this.data.spacePressed
      ? v.grabCursor
      : (hit && NodeCanvasStyle.cursors[hit.type]) || v.defaultCursor;
    const nextHover = hit.type === ht.empty ? null : hit;
    if (!NodeCanvasHelpers.isSameHoveredItem(gState.hoveredItem, nextHover)) {
      gState.hoveredItem = nextHover;
      gState.overlayBatchDirty = true;
      NodeCanvasRenderer.render();
    }
  },

  onContextMenu(_params, vp, e) {
    const s = this.settings, m = s.metrics, v = s.values, sel = s.selectors;
    if (!vp || e.target.closest(sel.textInputs)) return;
    e.preventDefault();
    if (this.data.suppressNextContextMenu) {
      this.data.suppressNextContextMenu = false;
      return;
    }
    const ptWorld = NodeCanvasHelpers.clientToStudioWorld(vp, e.clientX, e.clientY);
    const hit = NodeCanvasHelpers.hitTest(ptWorld.wx, ptWorld.wy);
    if (hit.type === NodeCanvasHelpers.hitTypes.wirePin) {
      this.removeWirePinAt(hit.wire, hit.pinIdx);
      return;
    }
    const rect = vp.getBoundingClientRect();
    NodeCanvasHelpers.pendingContextMenuWorldPos = { x: ptWorld.wx - m.ctxMenuOffsetX, y: ptWorld.wy - m.ctxMenuOffsetY };
    const menu = vp.querySelector(sel.menu);
    if (menu) {
      menu.style.left = Math.max(m.ctxMenuMinEdgePx, Math.min(rect.width - m.ctxMenuMaxRightPadPx, ptWorld.vx)) + v.pxSuffix;
      menu.style.top = Math.max(m.ctxMenuMinEdgePx, Math.min(rect.height - m.ctxMenuMaxBottomPadPx, ptWorld.vy)) + v.pxSuffix;
      menu.classList.remove(s.classes.hidden);
    }
  },

  onDblclick(_params, vp, e) {
    const s = this.settings, m = s.metrics, sel = s.selectors, act = s.actions, ht = NodeCanvasHelpers.hitTypes;
    if (!vp || e.target.closest(sel.ignoreDblclick)) return;
    const ptWorld = NodeCanvasHelpers.clientToStudioWorld(vp, e.clientX, e.clientY);
    const hit = NodeCanvasHelpers.hitTest(ptWorld.wx, ptWorld.wy);
    if (hit.type === ht.wirePin) {
      e.preventDefault();
      this.removeWirePinAt(hit.wire, hit.pinIdx);
      return;
    }
    if (hit.type === ht.chapterTitle || hit.type === ht.chapterGrip) {
      e.preventDefault();
      if (!NodeCanvasHelpers.isHistoryPreview()) this.openFloatingChapterInput(hit.chapter);
      else Dispatcher.post(act.selectChapter, { chapter: hit.chapter.name });
      return;
    }
    if (hit.type === ht.nodeCard || hit.type === ht.nodeTitle || hit.type === ht.choiceLabel || hit.type === ht.editSceneBtn) {
      e.preventDefault();
      this.hideFloatingNodeTitleInput(true);
      this.hideFloatingChoiceInput(true);
      this.flushSync();
      Dispatcher.post(act.openTextModal, { scene: hit.node.id });
      return;
    }
    if (hit.type !== ht.empty || NodeCanvasHelpers.isHistoryPreview()) return;
    e.preventDefault();
    this.createSceneAtWorldCoords(ptWorld.wx - m.ctxMenuOffsetX, ptWorld.wy - m.ctxMenuOffsetY);
  },

  resolveBookId() {
    const s = this.settings;
    const activeBookTab = document.querySelector(s.selectors.activeBookTab);
    return (activeBookTab && activeBookTab.getAttribute(s.attrs.book)) || s.values.defaultBook;
  },

  initCanvas(_params, canvasEl) {
    const s = this.settings;
    if (
      typeof NodeCanvasRenderer !== s.values.undefType &&
      NodeCanvasRenderer.studioGL.canvas &&
      canvasEl &&
      canvasEl !== NodeCanvasRenderer.studioGL.canvas &&
      canvasEl.parentElement
    ) {
      canvasEl.parentElement.replaceChild(NodeCanvasRenderer.studioGL.canvas, canvasEl);
    }
    const viewport = document.querySelector(s.selectors.viewport);
    if (!viewport) return;

    const currentBookId = this.resolveBookId();
    const needReset = !this.data.initialized || this.data.bookId !== currentBookId || Boolean(this.data.resetCanvas);

    if (this.data.selectedSceneId) {
      NodeCanvasHelpers.selectedNodeIds.clear();
      NodeCanvasHelpers.selectedNodeIds.add(this.data.selectedSceneId);
    }

    if (!this.data.bookGraph) {
      const expandBtn = document.querySelector(s.selectors.expandBtn);
      const fallbackSc = (expandBtn && expandBtn.getAttribute(s.attrs.scene)) || this.data.selectedSceneId;
      Dispatcher.post(s.actions.selectScene, { scene: fallbackSc, silent: s.values.trueStr });
      return;
    }

    if (this.data.canvasLayout && (needReset || this.data.layoutHash !== NodeCanvasLayout.layoutHash)) {
      NodeCanvasLayout.layoutHash = String(this.data.layoutHash || "");
      NodeCanvasModel.applyCanvasLayout(this.data.canvasLayout, needReset);
    }
    NodeCanvasModel.applyBookGraph(this.data.bookGraph);
    NodeCanvasRouter.route();

    if (needReset) {
      this.data.initialized = true;
      this.data.bookId = currentBookId;
      this.data.resetCanvas = false;
      this.resetToFirstNode(false, this.data.startSceneId || this.data.selectedSceneId);
    } else {
      this.applyTransform();
    }
  },

  onHydrate(frame) {
    const s = this.settings;
    if (typeof NodeCanvasLayout === s.values.undefType) return;
    const frameData = (frame && frame.data) || frame;

    const resetFlag = Boolean(this.data.resetCanvas);
    if (resetFlag) {
      this.data.initialized = false;
      this.data.resetCanvas = false;
    }

    const newSceneId = this.data.sceneId || "";
    const oldSceneId = this.data.scenePrevId || "";
    if (newSceneId && oldSceneId && newSceneId !== oldSceneId) {
      NodeCanvasModel.renameNode(oldSceneId, newSceneId);
    }
    this.data.scenePrevId = "";

    if (this.data.selectedSceneId) {
      NodeCanvasHelpers.selectedNodeIds.clear();
      NodeCanvasHelpers.selectedNodeIds.add(this.data.selectedSceneId);
    }

    if (!document.querySelector(s.selectors.viewport)) return;

    const currentBookId = this.resolveBookId();
    const needCameraReset = resetFlag || !this.data.initialized || this.data.bookId !== currentBookId;

    if (this.data.canvasLayout && (needCameraReset || this.data.layoutHash !== NodeCanvasLayout.layoutHash)) {
      NodeCanvasLayout.layoutHash = String(this.data.layoutHash || "");
      NodeCanvasModel.applyCanvasLayout(this.data.canvasLayout, needCameraReset);
    } else if (frameData && frameData.layoutHash && !this.data.dirty) {
      NodeCanvasLayout.layoutHash = String(frameData.layoutHash);
    }

    if (frameData && frameData.bookGraph && this.data.bookGraph) {
      NodeCanvasModel.applyBookGraph(this.data.bookGraph);
      NodeCanvasRouter.route();
      if (needCameraReset) {
        this.data.initialized = true;
        this.data.bookId = currentBookId;
        this.resetToFirstNode(false, this.data.startSceneId || this.data.selectedSceneId);
      } else {
        this.applyTransform();
      }
    }
  }
};
Dispatcher.register("NodeCanvasController", NodeCanvasController);

