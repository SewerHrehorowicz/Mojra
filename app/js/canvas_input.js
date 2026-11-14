"use strict";

const CanvasInput = {
  bindings() {
    return {
      "body": { init: this.bindEvents },
      "#btn-context-add-node": { click: this.contextAddNode },
      "#btn-context-add-chapter": { click: this.contextAddChapter }
    };
  },

  createSceneAtWorldCoords(wx, wy) {
    CanvasOverlays.closeContextMenu();
    const ch = CanvasLayout.findClosestChapterAtWorldCoords(wx, wy);
    const pos = CanvasLayout.resolveNonOverlappingNodeCoords(
      CanvasModel.snapToEditorGrid(wx),
      CanvasModel.snapToEditorGrid(wy),
      ch
    );
    Dispatcher.post("CreateSceneAt", { x: pos.x, y: pos.y, chapter: ch });
  },

  contextAddNode() {
    this.createSceneAtWorldCoords(CanvasModel.pendingContextMenuWorldPos.x, CanvasModel.pendingContextMenuWorldPos.y);
  },

  contextAddChapter() {
    CanvasOverlays.closeContextMenu();
    Dispatcher.post("CreateChapterGroup", {
      x: CanvasModel.snapToEditorGrid(CanvasModel.pendingContextMenuWorldPos.x),
      y: CanvasModel.snapToEditorGrid(CanvasModel.pendingContextMenuWorldPos.y)
    });
  },

  bindEvents() {
    if (this._bound || typeof document === "undefined") return;
    this._bound = true;

    document.addEventListener("keydown", (e) => {
      const state = CanvasModel.editorCanvasState;
      if (e.code === "Space" && !e.target.closest("input, textarea, select")) {
        state.spacePressed = true;
        const vp = document.querySelector("#timeline-scroll-viewport");
        if (vp) vp.classList.add("is-panning");
      }
      if ((e.ctrlKey || e.metaKey) && !e.altKey && document.querySelector("#timeline-scroll-viewport")) {
        const keyLower = (e.key || "").toLowerCase();
        const isUndo = (keyLower === "z" || e.code === "KeyZ") && !e.shiftKey;
        const isRedo = ((keyLower === "z" || e.code === "KeyZ") && e.shiftKey) || ((keyLower === "y" || e.code === "KeyY") && !e.shiftKey);
        if (isUndo || isRedo) {
          e.preventDefault();
          Dispatcher.post(isRedo ? "RedoEditor" : "UndoEditor", {});
          return;
        }
      }
      if ((e.key === "Delete" || e.key === "Backspace") && !e.target.closest("input, textarea, select") && !document.querySelector(".modal-backdrop") && document.querySelector("#timeline-scroll-viewport") && !CanvasModel.isHistoryPreview()) {
        if (CanvasModel.selectedNodeIds.size === 1) {
          e.preventDefault();
          Dispatcher.post("PromptDeleteScene", { scene: Array.from(CanvasModel.selectedNodeIds)[0] });
        } else if (CanvasModel.selectedNodeIds.size > 1) {
          e.preventDefault();
          Dispatcher.post("PromptDeleteScenes", { scenes: Array.from(CanvasModel.selectedNodeIds).join(",") });
        }
      }
    });

    document.addEventListener("keyup", (e) => {
      if (e.code === "Space") {
        CanvasModel.editorCanvasState.spacePressed = false;
        if (!CanvasModel.activeCanvasGesture) {
          const vp = document.querySelector("#timeline-scroll-viewport");
          if (vp) vp.classList.remove("is-panning");
        }
      }
    });

    document.addEventListener("wheel", (e) => {
      const viewport = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
      if (!viewport || e.target.closest(".custom-select-menu")) return;
      e.preventDefault();
      const sensitivity = e.ctrlKey || e.metaKey ? 0.0045 : 0.0018;
      StudioController.zoomAtPoint((CanvasModel.editorCanvasState.zoom || 1.0) * Math.exp(-e.deltaY * sensitivity), e.clientX, e.clientY);
    }, { passive: false });

    document.addEventListener("mousedown", (e) => {
      const viewport = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
      if (!viewport) {
        CanvasOverlays.closeContextMenu();
        CanvasOverlays.hideFloatingChapterInput(true);
        CanvasOverlays.hideFloatingNodeTitleInput(true);
        CanvasOverlays.hideFloatingChoiceInput(true);
        return;
      }
      if (e.target.closest("#canvas-context-menu, .custom-select-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #editor-empty-state, #btn-expand-scene-text, #studio-search-palette, #editor-status-bar")) {
        return;
      }
      CanvasOverlays.closeContextMenu();
      CanvasOverlays.hideFloatingChapterInput(true);
      CanvasOverlays.hideFloatingNodeTitleInput(true);
      CanvasOverlays.hideFloatingChoiceInput(true);

      const state = CanvasModel.editorCanvasState;
      if (e.button === 1 || (e.button === 0 && state.spacePressed)) {
        e.preventDefault();
        viewport.classList.add("is-panning");
        CanvasModel.activeCanvasGesture = {
          type: "pan", viewport: viewport, startClientX: e.clientX, startClientY: e.clientY,
          startPanX: state.panX, startPanY: state.panY
        };
        return;
      }
      if (e.button === 2 && !e.target.closest("input, textarea")) {
        CanvasModel.activeCanvasGesture = {
          type: "right-pan", viewport: viewport, startClientX: e.clientX, startClientY: e.clientY,
          startPanX: state.panX, startPanY: state.panY, moved: false
        };
        return;
      }
      if (e.button !== 0) return;

      const ptrCtx = CanvasHittest.buildPointerContext(viewport, e, false);
      const hit = CanvasHittest.hitTest(ptrCtx.wx, ptrCtx.wy);
      if (CanvasDrag.beginSharedHitDragGesture(hit, ptrCtx)) {
        e.preventDefault();
        return;
      }
      if (hit.type === "add-choice-btn") {
        e.preventDefault();
        Dispatcher.post("AddChoice", { scene: hit.node.id });
        return;
      }
      if (hit.type === "branch-radio" && hit.node && hit.node.branchRole === "secondary") {
        e.preventDefault();
        Dispatcher.post("SelectPrimaryBranchScene", { scene: hit.node.id, parent: hit.node.branchParentId || "" });
        return;
      }
      if (hit.type === "edit-scene-btn") {
        e.preventDefault();
        CanvasModel.selectedNodeIds.clear();
        if (hit.node && hit.node.id) CanvasModel.selectedNodeIds.add(hit.node.id);
        EditorWebglScene.render();
        Dispatcher.post("OpenSceneTextModal", { scene: hit.node.id });
        return;
      }
      if (hit.type === "delete-scene-btn") {
        e.preventDefault();
        Dispatcher.post("PromptDeleteScene", { scene: hit.node.id });
        return;
      }
      if (hit.type === "choice-trash-btn") {
        e.preventDefault();
        Dispatcher.post("PromptRemoveChoice", { scene: hit.node.id, choice_idx: String(hit.choiceIdx) });
        return;
      }
      if (hit.type === "choice-label" || hit.type === "node-title") {
        e.preventDefault();
        CanvasModel.selectedNodeIds.clear();
        if (hit.node && hit.node.id) CanvasModel.selectedNodeIds.add(hit.node.id);
        if (hit.type === "choice-label") CanvasOverlays.openFloatingChoiceInput(hit.node, hit.choiceIdx);
        else CanvasOverlays.openFloatingNodeTitleInput(hit.node);
        return;
      }
      if (hit.type === "node-card") {
        e.preventDefault();
        const sceneId = hit.node.id;
        CanvasModel.selectedNodeIds.clear();
        if (sceneId) CanvasModel.selectedNodeIds.add(sceneId);
        EditorWebglScene.render();
        Dispatcher.post("SelectStudioScene", { scene: sceneId });
        return;
      }

      e.preventDefault();
      const baseSelection = ptrCtx.isMultiModifier ? new Set(CanvasModel.selectedNodeIds) : new Set();
      if (!ptrCtx.isMultiModifier) {
        CanvasModel.editorSelectionCleared = true;
        CanvasModel.selectedNodeIds.clear();
        EditorWebglScene.render();
      }
      CanvasModel.activeCanvasGesture = {
        type: "marquee-select", viewport: viewport,
        startClientX: e.clientX, startClientY: e.clientY, baseSelection: baseSelection, marqueeRect: null, moved: false
      };
    });

    document.addEventListener("mousemove", (e) => {
      if (CanvasModel.activeCanvasGesture) {
        if (!CanvasModel.activeCanvasGesture.isTouch) CanvasDrag.updateMove(e.clientX, e.clientY);
        return;
      }
      const viewport = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
      const gState = CanvasModel.studioGraphState;
      if (!viewport || !viewport.contains(e.target) || e.target.closest("#canvas-context-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #btn-expand-scene-text")) {
        if (gState.hoveredItem) {
          gState.hoveredItem = null;
          gState.overlayBatchDirty = true;
          EditorWebglScene.render();
        }
        return;
      }
      const ptWorld = CanvasHittest.clientToStudioWorld(viewport, e.clientX, e.clientY);
      const hit = CanvasHittest.hitTest(ptWorld.wx, ptWorld.wy);
      viewport.style.cursor = CanvasModel.editorCanvasState.spacePressed
        ? "grab"
        : (hit && CanvasHittest.HIT_CURSOR_MAP[hit.type]) || "default";
      const nextHover = hit.type === "empty" ? null : hit;
      if (!CanvasHittest.isSameHoveredItem(gState.hoveredItem, nextHover)) {
        gState.hoveredItem = nextHover;
        gState.overlayBatchDirty = true;
        EditorWebglScene.render();
      }
    });

    document.addEventListener("mouseup", () => {
      if (!CanvasModel.activeCanvasGesture || CanvasModel.activeCanvasGesture.isTouch) return;
      CanvasDrag.finishGesture();
    });

    document.addEventListener("contextmenu", (e) => {
      const vp = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
      if (!vp || e.target.closest("input, textarea")) return;
      e.preventDefault();
      if (CanvasModel.editorCanvasState.suppressNextContextMenu) {
        CanvasModel.editorCanvasState.suppressNextContextMenu = false;
        return;
      }
      const ptWorld = CanvasHittest.clientToStudioWorld(vp, e.clientX, e.clientY);
      const hit = CanvasHittest.hitTest(ptWorld.wx, ptWorld.wy);
      if (hit.type === "wire-pin") {
        CanvasDrag.removeWirePinAt(hit.wire, hit.pinIdx);
        return;
      }
      const rect = vp.getBoundingClientRect();
      CanvasModel.pendingContextMenuWorldPos = { x: ptWorld.wx - 96, y: ptWorld.wy - 20 };
      const menu = vp.querySelector("#canvas-context-menu");
      if (menu) {
        menu.style.left = Math.max(8, Math.min(rect.width - 230, ptWorld.vx)) + "px";
        menu.style.top = Math.max(8, Math.min(rect.height - 150, ptWorld.vy)) + "px";
        menu.classList.remove("hidden");
      }
    });

    document.addEventListener("dblclick", (e) => {
      const vp = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
      if (!vp || e.target.closest("#canvas-context-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #editor-empty-state, #btn-expand-scene-text")) return;
      const ptWorld = CanvasHittest.clientToStudioWorld(vp, e.clientX, e.clientY);
      const hit = CanvasHittest.hitTest(ptWorld.wx, ptWorld.wy);
      if (hit.type === "wire-pin") {
        e.preventDefault();
        CanvasDrag.removeWirePinAt(hit.wire, hit.pinIdx);
        return;
      }
      if (hit.type === "chapter-title" || hit.type === "chapter-grip") {
        e.preventDefault();
        if (!CanvasModel.isHistoryPreview()) CanvasOverlays.openFloatingChapterInput(hit.chapter);
        else Dispatcher.post("SelectStudioChapter", { chapter: hit.chapter.name });
        return;
      }
      if (hit.type === "node-card" || hit.type === "node-title" || hit.type === "choice-label" || hit.type === "edit-scene-btn") {
        e.preventDefault();
        CanvasOverlays.hideFloatingNodeTitleInput(true);
        CanvasOverlays.hideFloatingChoiceInput(true);
        Dispatcher.post("OpenSceneTextModal", { scene: hit.node.id });
        return;
      }
      if (hit.type !== "empty" || CanvasModel.isHistoryPreview()) return;
      e.preventDefault();
      this.createSceneAtWorldCoords(ptWorld.wx - 96, ptWorld.wy - 20);
    });
  }
};
Dispatcher.register("CanvasInput", CanvasInput);
