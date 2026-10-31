"use strict";

function isEditorTouchClickSuppressed(e) {
  return (
    Date.now() < suppressNextTouchClickUntil &&
    e.target &&
    e.target.closest &&
    Boolean(e.target.closest("#timeline-scroll-viewport"))
  );
}

function clientToStudioWorld(viewport, clientX, clientY) {
  const rect = viewport.getBoundingClientRect();
  const z = editorCanvasState.zoom || 1.0;
  const vx = clientX - rect.left;
  const vy = clientY - rect.top;
  return {
    vx: vx,
    vy: vy,
    wx: (vx - editorCanvasState.panX) / z,
    wy: (vy - editorCanvasState.panY) / z
  };
}

function distToSegmentSq(px, py, x1, y1, x2, y2) {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const lenSq = dx * dx + dy * dy;
  if (lenSq < 0.001) {
    const ex = px - x1;
    const ey = py - y1;
    return ex * ex + ey * ey;
  }
  let t = ((px - x1) * dx + (py - y1) * dy) / lenSq;
  if (t < 0) t = 0;
  else if (t > 1) t = 1;
  const projX = x1 + t * dx;
  const projY = y1 + t * dy;
  const ox = px - projX;
  const oy = py - projY;
  return ox * ox + oy * oy;
}

function hitTestStudioCanvas(wx, wy) {
  const nodes = studioGraphState.nodes;
  const chapters = studioGraphState.chapters;
  const portHitR = 10;
  const portHitRSq = portHitR * portHitR;

  // 1. Scene output ports
  for (let i = nodes.length - 1; i >= 0; i--) {
    const nd = nodes[i];
    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        const px = nd.x + pt.ox;
        const py = nd.y + pt.oy;
        const dx = wx - px;
        const dy = wy - py;
        if (dx * dx + dy * dy <= portHitRSq) {
          return {
            type: "scene-out-port",
            node: nd,
            port: pt,
            portIdx: j,
            isBottom: false,
            x: px,
            y: py
          };
        }
      }
    } else if (nd.implicitPort) {
      const ip = nd.implicitPort;
      const px = nd.x + ip.ox;
      const py = nd.y + ip.oy;
      const dx = wx - px;
      const dy = wy - py;
      if (dx * dx + dy * dy <= portHitRSq) {
        return {
          type: "scene-out-port",
          node: nd,
          port: ip,
          portIdx: 0,
          isBottom: true,
          x: px,
          y: py
        };
      }
    }
  }

  // 2. Chapter output ports
  for (let i = chapters.length - 1; i >= 0; i--) {
    const ch = chapters[i];
    if (ch.hidden) continue;
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      const dx = wx - op.x;
      const dy = wy - op.y;
      if (dx * dx + dy * dy <= portHitRSq) {
        return {
          type: "chapter-out-port",
          chapter: ch,
          outPort: op,
          portIdx: j,
          x: op.x,
          y: op.y
        };
      }
    }
  }

  // 3. Scene input ports
  for (let i = nodes.length - 1; i >= 0; i--) {
    const nd = nodes[i];
    if (!nd.hasInputPort) continue;
    const px = nd.x + nd.inOx;
    const py = nd.y + nd.inOy;
    const dx = wx - px;
    const dy = wy - py;
    if (dx * dx + dy * dy <= portHitRSq) {
      return {
        type: "scene-in-port",
        node: nd,
        x: px,
        y: py
      };
    }
  }

  // 4. Chapter input ports
  for (let i = chapters.length - 1; i >= 0; i--) {
    const ch = chapters[i];
    if (ch.hidden || ch.isRoot) continue;
    const dx = wx - ch.inX;
    const dy = wy - ch.inY;
    if (dx * dx + dy * dy <= portHitRSq) {
      return {
        type: "chapter-in-port",
        chapter: ch,
        x: ch.inX,
        y: ch.inY
      };
    }
  }

  // 5. Scene node cards (+ Wybór button, grip ⋮⋮, or card body)
  for (let i = nodes.length - 1; i >= 0; i--) {
    const nd = nodes[i];
    if (wx >= nd.x && wx <= nd.x + nd.w && wy >= nd.y && wy <= nd.y + nd.h) {
      const footerTop = nd.y + nd.h - NODE_FOOTER_HEIGHT;
      if (wx >= nd.x + 8 && wx <= nd.x + nd.w - 8 && wy >= footerTop + 6 && wy <= footerTop + 34) {
        return { type: "add-choice-btn", node: nd };
      }
      if (wx >= nd.x + 2 && wx <= nd.x + 26 && wy >= nd.y + 4 && wy <= nd.y + 30) {
        return { type: "node-grip", node: nd };
      }
      return { type: "node-card", node: nd };
    }
  }

  // 6. Chapter group headers (grip ⋮⋮ or title)
  for (let i = chapters.length - 1; i >= 0; i--) {
    const ch = chapters[i];
    if (ch.hidden) continue;
    if (wx >= ch.x && wx <= ch.x + ch.w && wy >= ch.y && wy <= ch.y + 28) {
      if (wx <= ch.x + 26) {
        return { type: "chapter-grip", chapter: ch };
      }
      return { type: "chapter-title", chapter: ch };
    }
  }

  // 7. Static wires
  const wires = studioGraphState.wires;
  const wireTolSq = 49; // 7px world radius
  for (let i = wires.length - 1; i >= 0; i--) {
    const w = wires[i];
    if (w.bbox && (wx < w.bbox.minX || wx > w.bbox.maxX || wy < w.bbox.minY || wy > w.bbox.maxY)) {
      continue;
    }
    const pts = w.pts;
    for (let j = 0; j < pts.length - 1; j++) {
      if (distToSegmentSq(wx, wy, pts[j].x, pts[j].y, pts[j + 1].x, pts[j + 1].y) <= wireTolSq) {
        return { type: "wire", wire: w };
      }
    }
  }

  return { type: "empty" };
}

function isSameHoveredCanvasItem(a, b) {
  if (a === b) return true;
  if (!a || !b) return false;
  if (a.type !== b.type) return false;
  return (
    a.node === b.node &&
    a.chapter === b.chapter &&
    a.portIdx === b.portIdx &&
    a.wire === b.wire
  );
}

function updateCanvasHoverCursor(viewport, hit) {
  if (!viewport) return;
  if (editorCanvasState.spacePressed) {
    viewport.style.cursor = "grab";
    return;
  }
  if (!hit || hit.type === "empty") {
    viewport.style.cursor = "default";
    return;
  }
  switch (hit.type) {
    case "scene-out-port":
    case "scene-in-port":
    case "chapter-out-port":
    case "chapter-in-port":
      viewport.style.cursor = "crosshair";
      break;
    case "node-grip":
    case "chapter-grip":
      viewport.style.cursor = "grab";
      break;
    case "chapter-title":
      viewport.style.cursor = "text";
      break;
    case "add-choice-btn":
    case "node-card":
    case "wire":
      viewport.style.cursor = "pointer";
      break;
    default:
      viewport.style.cursor = "default";
      break;
  }
}

function triggerCanvasAddChoiceForNode(nd) {
  if (!nd || !nd.id) return;
  const params = { scene: nd.id };
  const contextForm = qs("#editor-context-form");
  if (contextForm && contextForm.getAttribute("data-context-scene-id") === nd.id) {
    if (typeof dirtyInspectorForms !== "undefined") {
      dirtyInspectorForms.delete(contextForm);
    }
    if (typeof liveSaveTimers !== "undefined" && liveSaveTimers.has(contextForm)) {
      clearTimeout(liveSaveTimers.get(contextForm));
      liveSaveTimers.delete(contextForm);
    }
    const fd = new FormData(contextForm);
    fd.forEach((val, key) => {
      params[key] = val;
    });
  }
  sendServerAction("AddChoice", params);
}

function handleEditorCanvasClick(e) {
  const multiDeselectBtn = e.target.closest("[data-multi-deselect-scene]");
  if (multiDeselectBtn) {
    e.preventDefault();
    e.stopPropagation();
    const id = multiDeselectBtn.getAttribute("data-multi-deselect-scene") || "";
    if (id && selectedNodeIds.has(id)) {
      selectedNodeIds.delete(id);
      editorSelectionCleared = selectedNodeIds.size === 0;
      syncNodeSelectionDOM();
      renderStudioCanvas();
      if (selectedNodeIds.size === 1) {
        selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
      }
    }
    return true;
  }

  const multiSelectBtn = e.target.closest("[data-multi-select-scene]");
  if (multiSelectBtn) {
    e.preventDefault();
    e.stopPropagation();
    const id = multiSelectBtn.getAttribute("data-multi-select-scene") || "";
    if (id) {
      editorSelectionCleared = false;
      selectedNodeIds.clear();
      selectedNodeIds.add(id);
      syncNodeSelectionDOM();
      renderStudioCanvas();
      selectNodeForContextPanel(id);
    }
    return true;
  }

  const selectChBtn = e.target.closest("[data-select-chapter]");
  if (selectChBtn) {
    e.preventDefault();
    e.stopPropagation();
    const chName = (selectChBtn.getAttribute("data-select-chapter") || "").trim().toLowerCase();
    if (!chName) return true;
    if (!e.shiftKey && !e.metaKey && !e.ctrlKey) {
      selectedNodeIds.clear();
    }
    for (let i = 0; i < studioGraphState.nodes.length; i++) {
      const nd = studioGraphState.nodes[i];
      if (nd.chapterLower === chName) {
        selectedNodeIds.add(nd.id);
      }
    }
    editorSelectionCleared = selectedNodeIds.size === 0;
    syncNodeSelectionDOM();
    renderStudioCanvas();
    if (selectedNodeIds.size === 1) {
      selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
    }
    return true;
  }

  const addNodeBtn = e.target.closest("[data-canvas-add-node]");
  if (addNodeBtn) {
    e.preventDefault();
    e.stopPropagation();
    const vp = qs("#timeline-scroll-viewport");
    if (!vp) return true;
    const rect = vp.getBoundingClientRect();
    const z = editorCanvasState.zoom || 1.0;
    const worldCenterX = (rect.width * 0.5 - editorCanvasState.panX) / z - 96;
    const worldCenterY = (rect.height * 0.35 - editorCanvasState.panY) / z;
    createSceneNodeAtWorldCoords(worldCenterX, worldCenterY);
    return true;
  }

  const ctxAddBtn = e.target.closest("#btn-context-add-node");
  if (ctxAddBtn) {
    e.preventDefault();
    e.stopPropagation();
    createSceneNodeAtWorldCoords(pendingContextMenuWorldPos.x, pendingContextMenuWorldPos.y);
    return true;
  }

  const ctxAddChBtn = e.target.closest("#btn-context-add-chapter");
  if (ctxAddChBtn) {
    e.preventDefault();
    e.stopPropagation();
    closeCanvasContextMenu();
    sendServerAction("CreateChapterGroup", {
      x: snapToEditorGrid(pendingContextMenuWorldPos.x),
      y: snapToEditorGrid(pendingContextMenuWorldPos.y)
    });
    return true;
  }

  const ctxResetBtn = e.target.closest("#btn-context-reset-view");
  if (ctxResetBtn) {
    e.preventDefault();
    e.stopPropagation();
    closeCanvasContextMenu();
    const vp = qs("#timeline-scroll-viewport");
    if (!vp) return true;
    fitCanvasToAllNodes(vp);
    applyEditorCanvasTransform();
    scheduleSaveEditorViewport();
    return true;
  }

  const zoomBtn = e.target.closest("[data-canvas-zoom]");
  if (zoomBtn) {
    e.preventDefault();
    e.stopPropagation();
    const mode = zoomBtn.getAttribute("data-canvas-zoom");
    const vp = qs("#timeline-scroll-viewport");
    if (!vp) return true;
    const rect = vp.getBoundingClientRect();
    const cx = rect.left + rect.width * 0.5;
    const cy = rect.top + rect.height * 0.5;
    if (mode === "in") {
      zoomEditorAtPoint((editorCanvasState.zoom || 1.0) * 1.2, cx, cy);
    } else if (mode === "out") {
      zoomEditorAtPoint((editorCanvasState.zoom || 1.0) / 1.2, cx, cy);
    } else if (mode === "fit") {
      fitCanvasToAllNodes(vp);
      applyEditorCanvasTransform();
      scheduleSaveEditorViewport();
    } else if (mode === "reset") {
      if (areAllNodesOutOfViewport(vp)) {
        fitCanvasToAllNodes(vp);
      } else {
        resetCanvasToFirstNode(vp);
      }
      applyEditorCanvasTransform();
      scheduleSaveEditorViewport();
    }
    return true;
  }

  const selectToggle = e.target.closest("[data-select-toggle]");
  if (selectToggle) {
    e.preventDefault();
    e.stopPropagation();
    const selectEl = selectToggle.closest(".custom-select");
    if (!selectEl) return true;
    const menu = qs(".custom-select-menu", selectEl);
    const isOpen = selectEl.classList.contains("open");
    closeAllCustomSelects(selectEl);
    if (isOpen) {
      selectEl.classList.remove("open");
      if (menu) menu.classList.add("hidden");
    } else {
      selectEl.classList.add("open");
      if (menu) menu.classList.remove("hidden");
    }
    return true;
  }

  const selectOpt = e.target.closest(".custom-select-option");
  if (selectOpt) {
    e.preventDefault();
    e.stopPropagation();
    const selectEl = selectOpt.closest(".custom-select");
    if (!selectEl) return true;
    const val = selectOpt.getAttribute("data-select-value") || "";
    if (val === "__browse_disk__") {
      closeAllCustomSelects(null);
      const hiddenInput = qs(".custom-select-input", selectEl);
      const inputName = hiddenInput ? hiddenInput.getAttribute("name") || "" : "";
      const mediaKind = inputName === "explicit_image" ? "image" : "audio";
      const form = selectEl.closest("form.editor-context-form, form#form-scene-text");
      const params = { kind: mediaKind };
      if (form) {
        const fd = new FormData(form);
        fd.forEach((v, k) => {
          params[k] = v;
        });
        if (!params.scene && params.scene_id) {
          params.scene = params.scene_id;
        }
      }
      sendServerAction("BrowseAddBookMedia", params);
      return true;
    }
    const lbl = selectOpt.getAttribute("data-select-label") || val || "— brak —";
    const hiddenInput = qs(".custom-select-input", selectEl);
    const labelSpan = qs(".custom-select-label", selectEl);
    const triggerBtn = qs(".custom-select-trigger", selectEl);
    const portIcon = qs(".choice-port-icon", selectEl);
    if (hiddenInput) hiddenInput.value = val;
    if (labelSpan) labelSpan.textContent = lbl;
    if (triggerBtn && portIcon) {
      triggerBtn.title = lbl;
      const isFilled = Boolean(val && val.toUpperCase() !== "END");
      portIcon.classList.toggle("port-filled", isFilled);
    }
    qsa(".custom-select-option", selectEl).forEach((btn) => {
      btn.classList.toggle("active", btn === selectOpt);
    });
    closeAllCustomSelects(null);

    if (selectEl.classList.contains("node-chapter-select")) {
      const parentForm = selectEl.closest("form.editor-context-form");
      const nodeId = parentForm ? parentForm.getAttribute("data-context-scene-id") || "" : "";
      if (selectedNodeIds.size > 1 && selectedNodeIds.has(nodeId) && val) {
        sendServerAction("AssignChapterToScenes", {
          scene_ids: Array.from(selectedNodeIds).join(","),
          chapter: val
        });
        return true;
      }
    }

    const modalForm = selectEl.closest("form#form-scene-text");
    if (modalForm) {
      const params = {};
      const fd = new FormData(modalForm);
      fd.forEach((v, k) => {
        params[k] = v;
      });
      sendServerAction("SaveSceneTextModal", params);
      return true;
    }

    const form = selectEl.closest("form.editor-context-form");
    if (form) {
      const params = {};
      const fd = new FormData(form);
      fd.forEach((v, k) => {
        params[k] = v;
      });
      sendServerAction("SaveScene", params);
    }
    return true;
  }

  if (!e.target.closest(".custom-select")) {
    closeAllCustomSelects(null);
  }

  return false;
}

function beginWireConnectGesture(hit, viewport, isTouch) {
  const nd = hit.node;
  const pt = hit.port;
  const hasExistingTarget = Boolean((pt.targetId && pt.targetId !== "END") || pt.targetChapter);
  activeCanvasGesture = {
    type: "wire-connect",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    fromNode: nd,
    fromSceneId: nd.id,
    fromChapter: nd.chapter,
    choiceIdx: String(hit.portIdx),
    isBottomPort: Boolean(hit.isBottom),
    hasExistingTarget: hasExistingTarget,
    startX: hit.x,
    startY: hit.y,
    curX: hit.x,
    curY: hit.y,
    hoveredTargetId: "",
    hoveredSceneNode: null,
    hoveredChapterIn: null,
    hoveredChapterOutPort: null
  };
  updateTempWirePreview();
  return true;
}

function beginChapterWireConnectGesture(hit, viewport, isTouch) {
  const ch = hit.chapter;
  const op = hit.outPort;
  activeCanvasGesture = {
    type: "chapter-wire-connect",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    fromChapterObj: ch,
    activeChapterOutPort: op,
    isBottomPort: true,
    fromChapter: ch.ref || ch.name,
    oldTargetChapter: op.target || "",
    startX: op.x,
    startY: op.y,
    curX: op.x,
    curY: op.y,
    hoveredTargetChapter: "",
    hoveredChapterIn: null
  };
  updateTempWirePreview();
  return true;
}

function beginChapterDragGesture(ch, viewport, clientX, clientY, isTouch, clickedTitleArea) {
  if (!ch) return false;
  const chNodes = studioGraphState.nodes.filter(
    (nd) => nd.chapterLower === ch.nameLower || nd.chapterLower === ch.refLower
  );
  if (chNodes.length === 0) return false;
  const items = chNodes.map((nd) => ({
    node: nd,
    sceneId: nd.id,
    startX: nd.x,
    startY: nd.y
  }));
  activeCanvasGesture = {
    type: "chapter-drag",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    chapter: ch,
    chapterName: ch.name,
    clickedTitleArea: Boolean(clickedTitleArea),
    dragConfirmed: !clickedTitleArea,
    items: items,
    startClientX: clientX,
    startClientY: clientY,
    moved: false
  };
  renderStudioCanvas();
  return true;
}

function beginNodeDragGesture(nd, viewport, clientX, clientY, isTouch, isMultiModifier) {
  const sceneId = nd.id;
  if (isMultiModifier) {
    if (selectedNodeIds.has(sceneId)) {
      selectedNodeIds.delete(sceneId);
      editorSelectionCleared = selectedNodeIds.size === 0;
      syncNodeSelectionDOM();
      renderStudioCanvas();
      if (selectedNodeIds.size === 1) {
        selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
      }
      return true;
    }
    if (sceneId) selectedNodeIds.add(sceneId);
    editorSelectionCleared = false;
  } else if (!selectedNodeIds.has(sceneId)) {
    editorSelectionCleared = false;
    selectedNodeIds.clear();
    if (sceneId) selectedNodeIds.add(sceneId);
  }
  syncNodeSelectionDOM();

  const dragNodes = studioGraphState.nodes.filter((n) => selectedNodeIds.has(n.id));
  if (dragNodes.length === 0) {
    dragNodes.push(nd);
  }

  const items = dragNodes.map((n) => ({
    node: n,
    sceneId: n.id,
    startX: n.x,
    startY: n.y
  }));

  activeCanvasGesture = {
    type: "node-drag",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    node: nd,
    sceneId: sceneId,
    isMultiModifier: Boolean(isMultiModifier),
    items: items,
    startClientX: clientX,
    startClientY: clientY,
    moved: false,
    hoveredChapterDrop: ""
  };
  renderStudioCanvas();
  return true;
}

function bindEditorCanvasEvents() {
  document.addEventListener("keydown", (e) => {
    if (e.code === "Space" && !e.target.closest("input, textarea, select")) {
      editorCanvasState.spacePressed = true;
      const vp = qs("#timeline-scroll-viewport");
      if (vp) vp.classList.add("is-panning");
    }
    if (e.key === "Escape") {
      if (
        e.target &&
        e.target.classList &&
        (e.target.classList.contains("chapter-group-title-input") ||
          e.target.classList.contains("canvas-book-title-input"))
      ) {
        e.preventDefault();
        if (e.target.id === "canvas-chapter-input") {
          hideFloatingChapterInput(false);
        } else {
          e.target.blur();
        }
        return;
      }
      if (!e.target.closest("input, textarea") && !qs(".modal-backdrop")) {
        editorSelectionCleared = true;
        if (selectedNodeIds.size > 0) {
          selectedNodeIds.clear();
        }
        syncNodeSelectionDOM();
        hideEditorContextPanel();
        renderStudioCanvas();
      }
    }
    if (e.key === "Enter" && e.target && e.target.classList) {
      if (e.target.classList.contains("chapter-group-title-input")) {
        e.preventDefault();
        hideFloatingChapterInput(true);
        return;
      }
      if (e.target.classList.contains("canvas-book-title-input")) {
        e.preventDefault();
        commitBookTitleInput(e.target);
        e.target.blur();
        return;
      }
    }
    if (
      (e.ctrlKey || e.metaKey) &&
      !e.altKey &&
      !e.target.closest("input, textarea, select, [contenteditable]") &&
      qs("#timeline-scroll-viewport")
    ) {
      const keyLower = (e.key || "").toLowerCase();
      if (keyLower === "z" && !e.shiftKey) {
        e.preventDefault();
        sendServerAction("UndoEditor", {});
        return;
      }
      if ((keyLower === "z" && e.shiftKey) || (keyLower === "y" && !e.shiftKey)) {
        e.preventDefault();
        sendServerAction("RedoEditor", {});
        return;
      }
    }
    if (
      (e.key === "f" || e.key === "F") &&
      !e.ctrlKey &&
      !e.metaKey &&
      !e.altKey &&
      !e.target.closest("input, textarea, select") &&
      !qs(".modal-backdrop") &&
      qs("#timeline-scroll-viewport")
    ) {
      e.preventDefault();
      const vp = qs("#timeline-scroll-viewport");
      fitCanvasToAllNodes(vp);
      applyEditorCanvasTransform();
      scheduleSaveEditorViewport();
      return;
    }
    if (
      (e.key === "Delete" || e.key === "Backspace") &&
      !e.target.closest("input, textarea, select") &&
      !qs(".modal-backdrop") &&
      qs("#timeline-scroll-viewport")
    ) {
      if (selectedNodeIds.size === 1) {
        e.preventDefault();
        sendServerAction("PromptDeleteScene", {
          scene: Array.from(selectedNodeIds)[0]
        });
      } else if (selectedNodeIds.size > 1) {
        e.preventDefault();
        sendServerAction("PromptDeleteScenes", {
          scenes: Array.from(selectedNodeIds).join(",")
        });
      }
    }
  });

  document.addEventListener("keyup", (e) => {
    if (e.code === "Space") {
      editorCanvasState.spacePressed = false;
      if (!activeCanvasGesture) {
        const vp = qs("#timeline-scroll-viewport");
        if (vp) vp.classList.remove("is-panning");
      }
    }
  });

  function startTouchPinchPanGesture(viewport, t0, t1) {
    const midX = (t0.clientX + t1.clientX) * 0.5;
    const midY = (t0.clientY + t1.clientY) * 0.5;
    const dist = Math.max(8, Math.hypot(t1.clientX - t0.clientX, t1.clientY - t0.clientY));
    const rect = viewport.getBoundingClientRect();
    const startZoom = editorCanvasState.zoom || 1.0;
    const startPanX = editorCanvasState.panX || 0;
    const startPanY = editorCanvasState.panY || 0;
    viewport.classList.add("is-panning");
    activeCanvasGesture = {
      type: "touch-pinch-pan",
      isTouch: true,
      viewport: viewport,
      startMidX: midX,
      startMidY: midY,
      startDist: dist,
      startZoom: startZoom,
      startPanX: startPanX,
      startPanY: startPanY,
      worldX: (midX - rect.left - startPanX) / startZoom,
      worldY: (midY - rect.top - startPanY) / startZoom,
      moved: true
    };
  }

  function applyIncrementalNodeMove(items, snappedDx, snappedDy) {
    const dirtyNodeIds = new Set();
    const dirtyChaptersSet = new Set();
    const dirtyAabbs = [];

    for (let i = 0; i < items.length; i++) {
      const nd = items[i].node;
      dirtyNodeIds.add(nd.id);
      dirtyAabbs.push({
        minX: nd.left - 24,
        minY: nd.top - 24,
        maxX: nd.right + 24,
        maxY: nd.bottom + 24
      });
      const ch = studioGraphState.chapterByKey.get(nd.chapterLower);
      if (ch && !dirtyChaptersSet.has(ch)) {
        dirtyChaptersSet.add(ch);
        dirtyAabbs.push({
          minX: ch.left - 24,
          minY: ch.top - 24,
          maxX: ch.right + 24,
          maxY: ch.bottom + 24
        });
      }
      nd.x = snapToEditorGrid(items[i].startX + snappedDx);
      nd.y = snapToEditorGrid(items[i].startY + snappedDy);
      recomputeNodeMetrics(nd);
      dirtyAabbs.push({
        minX: nd.left - 24,
        minY: nd.top - 24,
        maxX: nd.right + 24,
        maxY: nd.bottom + 24
      });
    }

    updateChapterGroupFrames();
    dirtyChaptersSet.forEach((ch) => {
      dirtyAabbs.push({
        minX: ch.left - 24,
        minY: ch.top - 24,
        maxX: ch.right + 24,
        maxY: ch.bottom + 24
      });
    });

    drawStudioWires(dirtyNodeIds, dirtyAabbs, dirtyChaptersSet);
  }

  function updateCanvasDragMove(clientX, clientY) {
    if (!activeCanvasGesture) return;

    if (activeCanvasGesture.type === "pan") {
      editorCanvasState.panX = activeCanvasGesture.startPanX + (clientX - activeCanvasGesture.startClientX);
      editorCanvasState.panY = activeCanvasGesture.startPanY + (clientY - activeCanvasGesture.startClientY);
      applyEditorCanvasTransform();
      return;
    }

    if (activeCanvasGesture.type === "touch-pan") {
      const dx = clientX - activeCanvasGesture.startClientX;
      const dy = clientY - activeCanvasGesture.startClientY;
      const threshold = activeCanvasGesture.startedInteractive ? 7 : 3;
      if (!activeCanvasGesture.moved && Math.hypot(dx, dy) > threshold) {
        activeCanvasGesture.moved = true;
        if (activeCanvasGesture.viewport) {
          activeCanvasGesture.viewport.classList.add("is-panning");
        }
      }
      if (activeCanvasGesture.moved) {
        editorCanvasState.panX = activeCanvasGesture.startPanX + dx;
        editorCanvasState.panY = activeCanvasGesture.startPanY + dy;
        applyEditorCanvasTransform();
      }
      return;
    }

    if (activeCanvasGesture.type === "right-pan") {
      const dx = clientX - activeCanvasGesture.startClientX;
      const dy = clientY - activeCanvasGesture.startClientY;
      if (!activeCanvasGesture.moved && Math.hypot(dx, dy) > 4) {
        activeCanvasGesture.moved = true;
        editorCanvasState.suppressNextContextMenu = true;
        const vp = qs("#timeline-scroll-viewport");
        if (vp) vp.classList.add("is-panning");
      }
      if (activeCanvasGesture.moved) {
        editorCanvasState.panX = activeCanvasGesture.startPanX + dx;
        editorCanvasState.panY = activeCanvasGesture.startPanY + dy;
        applyEditorCanvasTransform();
      }
      return;
    }

    if (activeCanvasGesture.type === "node-drag") {
      const z = editorCanvasState.zoom || 1.0;
      const dx = (clientX - activeCanvasGesture.startClientX) / z;
      const dy = (clientY - activeCanvasGesture.startClientY) / z;
      const snappedDx = snapToEditorGrid(dx);
      const snappedDy = snapToEditorGrid(dy);
      if (snappedDx === activeCanvasGesture.lastSnappedDx && snappedDy === activeCanvasGesture.lastSnappedDy) {
        return;
      }
      activeCanvasGesture.lastSnappedDx = snappedDx;
      activeCanvasGesture.lastSnappedDy = snappedDy;
      if (Math.abs(snappedDx) >= EDITOR_GRID_SIZE || Math.abs(snappedDy) >= EDITOR_GRID_SIZE) {
        activeCanvasGesture.moved = true;
      }
      applyIncrementalNodeMove(activeCanvasGesture.items || [], snappedDx, snappedDy);
      return;
    }

    if (activeCanvasGesture.type === "chapter-drag") {
      const rawDist = Math.hypot(
        clientX - activeCanvasGesture.startClientX,
        clientY - activeCanvasGesture.startClientY
      );
      if (!activeCanvasGesture.dragConfirmed) {
        if (rawDist < 5) {
          return;
        }
        activeCanvasGesture.dragConfirmed = true;
        hideFloatingChapterInput(false);
      }
      const z = editorCanvasState.zoom || 1.0;
      const dx = (clientX - activeCanvasGesture.startClientX) / z;
      const dy = (clientY - activeCanvasGesture.startClientY) / z;
      const snappedDx = snapToEditorGrid(dx);
      const snappedDy = snapToEditorGrid(dy);
      if (snappedDx === activeCanvasGesture.lastSnappedDx && snappedDy === activeCanvasGesture.lastSnappedDy) {
        return;
      }
      activeCanvasGesture.lastSnappedDx = snappedDx;
      activeCanvasGesture.lastSnappedDy = snappedDy;
      if (Math.abs(snappedDx) >= EDITOR_GRID_SIZE || Math.abs(snappedDy) >= EDITOR_GRID_SIZE) {
        activeCanvasGesture.moved = true;
      }
      applyIncrementalNodeMove(activeCanvasGesture.items || [], snappedDx, snappedDy);
      return;
    }

    if (activeCanvasGesture.type === "marquee-select") {
      const dx = clientX - activeCanvasGesture.startClientX;
      const dy = clientY - activeCanvasGesture.startClientY;
      if (!activeCanvasGesture.moved && Math.hypot(dx, dy) > 3) {
        activeCanvasGesture.moved = true;
      }
      if (!activeCanvasGesture.moved) return;

      const vpRect = activeCanvasGesture.viewport.getBoundingClientRect();
      const x1 = Math.min(activeCanvasGesture.startClientX, clientX) - vpRect.left;
      const y1 = Math.min(activeCanvasGesture.startClientY, clientY) - vpRect.top;
      const w = Math.abs(clientX - activeCanvasGesture.startClientX);
      const h = Math.abs(clientY - activeCanvasGesture.startClientY);
      activeCanvasGesture.marqueeRect = { x: x1, y: y1, w: w, h: h };

      const z = editorCanvasState.zoom || 1.0;
      const px = editorCanvasState.panX || 0;
      const py = editorCanvasState.panY || 0;
      const worldLeft = (x1 - px) / z;
      const worldRight = (x1 + w - px) / z;
      const worldTop = (y1 - py) / z;
      const worldBottom = (y1 + h - py) / z;

      selectedNodeIds.clear();
      activeCanvasGesture.baseSelection.forEach((id) => selectedNodeIds.add(id));

      for (let i = 0; i < studioGraphState.nodes.length; i++) {
        const nd = studioGraphState.nodes[i];
        if (nd.right >= worldLeft && nd.left <= worldRight && nd.bottom >= worldTop && nd.top <= worldBottom) {
          selectedNodeIds.add(nd.id);
        }
      }
      syncNodeSelectionDOM();
      renderStudioCanvas();
      return;
    }

    if (activeCanvasGesture.type === "wire-connect") {
      const ptWorld = clientToStudioWorld(activeCanvasGesture.viewport, clientX, clientY);
      let cx = ptWorld.wx;
      let cy = ptWorld.wy;
      let hoveredId = "";
      let midOutX;
      let midOutY;
      activeCanvasGesture.hoveredSceneNode = null;
      activeCanvasGesture.hoveredChapterIn = null;
      activeCanvasGesture.hoveredChapterOutPort = null;

      const fromChLower = (activeCanvasGesture.fromChapter || "").trim().toLowerCase();
      const chapters = studioGraphState.chapters;
      const nodes = studioGraphState.nodes;
      const fromFrame = studioGraphState.chapterByKey.get(fromChLower);

      const activateChapter = (tf) => {
        if (!tf || tf.hidden || tf.isRoot || !tf.ref || tf.nameLower === fromChLower || tf.refLower === fromChLower) {
          return false;
        }
        hoveredId = "chapter:" + tf.ref;
        cx = tf.inX;
        cy = tf.inY;
        activeCanvasGesture.hoveredChapterIn = tf;
        if (fromFrame) {
          const wantLower = tf.refLower;
          const matchedOut = fromFrame.outPorts.find(
            (op) => op.targetLower === wantLower || op.target === ""
          );
          if (matchedOut) {
            midOutX = matchedOut.x;
            midOutY = matchedOut.y;
            activeCanvasGesture.hoveredChapterOutPort = matchedOut;
          } else if (fromFrame.outPorts.length > 0) {
            const lastOp = fromFrame.outPorts[fromFrame.outPorts.length - 1];
            midOutX = lastOp.x + 24;
            midOutY = lastOp.y;
          } else {
            midOutX = fromFrame.x + fromFrame.w * 0.5;
            midOutY = fromFrame.y + fromFrame.h;
          }
        }
        return true;
      };

      let matched = false;
      if (fromFrame) {
        for (let j = 0; j < fromFrame.outPorts.length; j++) {
          const op = fromFrame.outPorts[j];
          if (Math.hypot(cx - op.x, cy - op.y) <= 24) {
            hoveredId = "chapter:" + (op.target || "__next__");
            cx = op.x;
            cy = op.y;
            activeCanvasGesture.hoveredChapterOutPort = op;
            matched = true;
            break;
          }
        }
      }
      if (!matched) {
        for (let i = 0; i < chapters.length; i++) {
          const fr = chapters[i];
          if (fr.hidden || fr.isRoot || fr.nameLower === fromChLower) continue;
          if (Math.hypot(cx - fr.inX, cy - fr.inY) <= 28) {
            if (activateChapter(fr)) {
              matched = true;
              break;
            }
          }
        }
      }

      if (!matched) {
        for (let i = 0; i < nodes.length; i++) {
          const nd = nodes[i];
          if (nd.id === activeCanvasGesture.fromSceneId) continue;
          if (cx >= nd.x - 6 && cx <= nd.x + nd.w + 6 && cy >= nd.y - 12 && cy <= nd.y + nd.h + 6) {
            if (!fromChLower || !nd.chapterLower || nd.chapterLower === fromChLower) {
              hoveredId = nd.id;
              cx = nd.x + nd.inOx;
              cy = nd.y + nd.inOy;
              activeCanvasGesture.hoveredSceneNode = nd;
              matched = true;
            } else {
              const tf = studioGraphState.chapterByKey.get(nd.chapterLower);
              if (activateChapter(tf)) {
                matched = true;
              }
            }
            break;
          }
        }
      }

      if (!matched) {
        for (let i = 0; i < chapters.length; i++) {
          const fr = chapters[i];
          if (fr.hidden || fr.isRoot || fr.nameLower === fromChLower) continue;
          if (cx >= fr.x - 10 && cx <= fr.x + fr.w + 10 && cy >= fr.y - 14 && cy <= fr.y + fr.h + 6) {
            if (activateChapter(fr)) break;
          }
        }
      }

      activeCanvasGesture.curX = cx;
      activeCanvasGesture.curY = cy;
      activeCanvasGesture.midOutX = midOutX;
      activeCanvasGesture.midOutY = midOutY;
      activeCanvasGesture.hoveredTargetId = hoveredId;
      updateTempWirePreview();
      return;
    }

    if (activeCanvasGesture.type === "chapter-wire-connect") {
      const ptWorld = clientToStudioWorld(activeCanvasGesture.viewport, clientX, clientY);
      let cx = ptWorld.wx;
      let cy = ptWorld.wy;
      let hoveredCh = "";
      activeCanvasGesture.hoveredChapterIn = null;

      const chapters = studioGraphState.chapters;
      const fromChLower = (activeCanvasGesture.fromChapter || "").trim().toLowerCase();

      for (let i = 0; i < chapters.length; i++) {
        const fr = chapters[i];
        if (fr.hidden || fr.isRoot || fr.nameLower === fromChLower || fr.refLower === fromChLower) continue;
        if (
          Math.hypot(cx - fr.inX, cy - fr.inY) <= 28 ||
          (cx >= fr.x - 10 && cx <= fr.x + fr.w + 10 && cy >= fr.y - 14 && cy <= fr.y + fr.h + 6)
        ) {
          hoveredCh = fr.ref;
          cx = fr.inX;
          cy = fr.inY;
          activeCanvasGesture.hoveredChapterIn = fr;
          break;
        }
      }

      activeCanvasGesture.curX = cx;
      activeCanvasGesture.curY = cy;
      activeCanvasGesture.hoveredTargetChapter = hoveredCh;
      updateTempWirePreview();
    }
  }

  function finishActiveCanvasGesture() {
    if (!activeCanvasGesture) return;
    const g = activeCanvasGesture;
    activeCanvasGesture = null;
    studioGraphState.overlayBatchDirty = true;

    const vp = qs("#timeline-scroll-viewport");
    if (vp && !editorCanvasState.spacePressed) {
      vp.classList.remove("is-panning");
    }

    if (g.type === "pan") {
      scheduleSaveEditorViewport();
      return;
    }

    if (g.type === "touch-pan" || g.type === "touch-pinch-pan") {
      if (g.moved) {
        suppressNextTouchClickUntil = Date.now() + 360;
        scheduleSaveEditorViewport();
      } else if (g.startedAddChoiceNode) {
        triggerCanvasAddChoiceForNode(g.startedAddChoiceNode);
      } else if (g.startedNodeId && !g.startedInteractive) {
        editorSelectionCleared = false;
        selectedNodeIds.clear();
        selectedNodeIds.add(g.startedNodeId);
        syncNodeSelectionDOM();
        renderStudioCanvas();
        selectNodeForContextPanel(g.startedNodeId);
      } else if (g.startedOnEmptyCanvas) {
        closeAllCustomSelects(null);
        editorSelectionCleared = true;
        if (selectedNodeIds.size > 0) {
          selectedNodeIds.clear();
        }
        syncNodeSelectionDOM();
        hideEditorContextPanel();
        renderStudioCanvas();
      }
      return;
    }

    if (g.type === "right-pan") {
      if (g.moved) {
        scheduleSaveEditorViewport();
      }
      return;
    }

    if (g.type === "node-drag") {
      const items = g.items || [];
      renderStudioCanvas();
      if (g.moved && items.length > 0) {
        if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
        const payload = items.map((it) => ({
          sceneId: it.sceneId,
          x: it.node.x,
          y: it.node.y
        }));
        saveBatchNodePositionsOnServer(payload);
        if (selectedNodeIds.size === 1) {
          selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
        }
      } else if (!g.moved && g.sceneId) {
        if (!g.isMultiModifier && selectedNodeIds.size > 1) {
          editorSelectionCleared = false;
          selectedNodeIds.clear();
          selectedNodeIds.add(g.sceneId);
          syncNodeSelectionDOM();
          renderStudioCanvas();
        }
        selectNodeForContextPanel(g.sceneId);
      }
      return;
    }

    if (g.type === "chapter-drag") {
      const items = g.items || [];
      renderStudioCanvas();
      if (g.moved && items.length > 0) {
        if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
        const payload = items.map((it) => ({
          sceneId: it.sceneId,
          x: it.node.x,
          y: it.node.y
        }));
        saveBatchNodePositionsOnServer(payload);
      } else if (!g.moved && g.clickedTitleArea && g.chapter) {
        openFloatingChapterInput(g.chapter);
      }
      return;
    }

    if (g.type === "marquee-select") {
      editorSelectionCleared = selectedNodeIds.size === 0;
      syncNodeSelectionDOM();
      renderStudioCanvas();
      if (selectedNodeIds.size === 0) {
        hideEditorContextPanel();
      } else if (selectedNodeIds.size === 1) {
        selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
      }
      return;
    }

    if (g.type === "wire-connect") {
      if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
      if (g.hoveredTargetId && g.fromSceneId && g.hoveredTargetId !== g.fromSceneId) {
        const params = {
          from_scene: g.fromSceneId,
          choice_idx: g.choiceIdx,
          to_scene: g.hoveredTargetId
        };
        const contextForm = qs("#editor-context-form");
        if (contextForm && contextForm.getAttribute("data-context-scene-id") === g.fromSceneId) {
          const fd = new FormData(contextForm);
          fd.forEach((val, key) => {
            params[key] = val;
          });
        }
        sendServerAction("ConnectChoice", params);
      } else if (
        g.isBottomPort &&
        g.fromSceneId &&
        !g.hoveredTargetId &&
        g.hasExistingTarget &&
        Math.hypot(g.curX - g.startX, g.curY - g.startY) > 28
      ) {
        const params = {
          from_scene: g.fromSceneId,
          choice_idx: "0",
          to_scene: "END"
        };
        const contextForm = qs("#editor-context-form");
        if (contextForm && contextForm.getAttribute("data-context-scene-id") === g.fromSceneId) {
          const fd = new FormData(contextForm);
          fd.forEach((val, key) => {
            params[key] = val;
          });
        }
        sendServerAction("ConnectChoice", params);
      } else {
        updateTempWirePreview();
      }
      return;
    }

    if (g.type === "chapter-wire-connect") {
      if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
      if (g.hoveredTargetChapter && g.fromChapter) {
        sendServerAction("ConnectChapter", {
          from_chapter: g.fromChapter,
          old_target_chapter: g.oldTargetChapter || "",
          to_chapter: g.hoveredTargetChapter
        });
      } else if (
        g.fromChapter &&
        !g.hoveredTargetChapter &&
        g.oldTargetChapter &&
        Math.hypot(g.curX - g.startX, g.curY - g.startY) > 28
      ) {
        sendServerAction("ConnectChapter", {
          from_chapter: g.fromChapter,
          old_target_chapter: g.oldTargetChapter,
          to_chapter: "END"
        });
      } else {
        updateTempWirePreview();
      }
    }
  }

  document.addEventListener(
    "wheel",
    (e) => {
      const viewport = e.target.closest("#timeline-scroll-viewport");
      if (!viewport) return;
      if (e.target.closest(".custom-select-menu")) return;
      e.preventDefault();
      lastPointerClientPos.x = e.clientX;
      lastPointerClientPos.y = e.clientY;
      const sensitivity = e.ctrlKey || e.metaKey ? 0.0045 : 0.0018;
      const factor = Math.exp(-e.deltaY * sensitivity);
      zoomEditorAtPoint((editorCanvasState.zoom || 1.0) * factor, e.clientX, e.clientY);
    },
    { passive: false }
  );

  document.addEventListener(
    "gesturestart",
    (e) => {
      if (!e.target.closest("#timeline-scroll-viewport")) return;
      e.preventDefault();
      editorCanvasState.gestureBaseZoom = editorCanvasState.zoom || 1.0;
    },
    { passive: false }
  );

  document.addEventListener(
    "gesturechange",
    (e) => {
      if (!e.target.closest("#timeline-scroll-viewport")) return;
      e.preventDefault();
      if (activeCanvasGesture && activeCanvasGesture.isTouch) return;
      zoomEditorAtPoint(editorCanvasState.gestureBaseZoom * (e.scale || 1.0), e.clientX, e.clientY);
    },
    { passive: false }
  );

  document.addEventListener(
    "gestureend",
    (e) => {
      if (!e.target.closest("#timeline-scroll-viewport")) return;
      e.preventDefault();
    },
    { passive: false }
  );

  document.addEventListener(
    "touchstart",
    (e) => {
      const viewport =
        (e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null) ||
        (activeCanvasGesture && activeCanvasGesture.viewport ? activeCanvasGesture.viewport : null);
      if (!viewport) {
        closeCanvasContextMenu();
        return;
      }

      if (e.touches.length >= 2) {
        e.preventDefault();
        closeCanvasContextMenu();
        startTouchPinchPanGesture(viewport, e.touches[0], e.touches[1]);
        return;
      }

      if (e.touches.length !== 1) return;
      const t = e.touches[0];
      lastPointerClientPos.x = t.clientX;
      lastPointerClientPos.y = t.clientY;

      if (e.target.closest("#canvas-context-menu, .custom-select-menu, #canvas-chapter-input")) {
        return;
      }
      closeCanvasContextMenu();
      hideFloatingChapterInput(true);

      const ptWorld = clientToStudioWorld(viewport, t.clientX, t.clientY);
      const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);

      if (hit.type === "chapter-out-port") {
        e.preventDefault();
        e.stopPropagation();
        beginChapterWireConnectGesture(hit, viewport, true);
        return;
      }

      if (hit.type === "scene-out-port") {
        e.preventDefault();
        e.stopPropagation();
        beginWireConnectGesture(hit, viewport, true);
        return;
      }

      if (hit.type === "chapter-grip" || hit.type === "chapter-title") {
        e.preventDefault();
        beginChapterDragGesture(hit.chapter, viewport, t.clientX, t.clientY, true, hit.type === "chapter-title");
        return;
      }

      if (hit.type === "node-grip") {
        e.preventDefault();
        beginNodeDragGesture(hit.node, viewport, t.clientX, t.clientY, true, false);
        return;
      }

      activeCanvasGesture = {
        type: "touch-pan",
        isTouch: true,
        viewport: viewport,
        startClientX: t.clientX,
        startClientY: t.clientY,
        startPanX: editorCanvasState.panX,
        startPanY: editorCanvasState.panY,
        moved: false,
        startedInteractive: hit.type === "add-choice-btn",
        startedAddChoiceNode: hit.type === "add-choice-btn" ? hit.node : null,
        startedNodeId: hit.type === "node-card" ? hit.node.id : "",
        startedOnEmptyCanvas: hit.type === "empty"
      };
    },
    { passive: false }
  );

  document.addEventListener(
    "touchmove",
    (e) => {
      if (!activeCanvasGesture && e.touches.length >= 2) {
        const vp = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
        if (vp) {
          e.preventDefault();
          startTouchPinchPanGesture(vp, e.touches[0], e.touches[1]);
          return;
        }
      }
      if (!activeCanvasGesture || !activeCanvasGesture.isTouch) return;

      if (e.touches.length >= 2) {
        e.preventDefault();
        if (activeCanvasGesture.type !== "touch-pinch-pan") {
          const vp = activeCanvasGesture.viewport || qs("#timeline-scroll-viewport");
          if (vp) {
            startTouchPinchPanGesture(vp, e.touches[0], e.touches[1]);
          }
          return;
        }
        const t0 = e.touches[0];
        const t1 = e.touches[1];
        const curMidX = (t0.clientX + t1.clientX) * 0.5;
        const curMidY = (t0.clientY + t1.clientY) * 0.5;
        const curDist = Math.max(8, Math.hypot(t1.clientX - t0.clientX, t1.clientY - t0.clientY));
        const scale = curDist / activeCanvasGesture.startDist;
        const newZoom = Math.max(0.25, Math.min(2.5, activeCanvasGesture.startZoom * scale));
        const rect = activeCanvasGesture.viewport.getBoundingClientRect();
        const vx = curMidX - rect.left;
        const vy = curMidY - rect.top;

        editorCanvasState.zoom = newZoom;
        editorCanvasState.panX = vx - activeCanvasGesture.worldX * newZoom;
        editorCanvasState.panY = vy - activeCanvasGesture.worldY * newZoom;

        applyEditorCanvasTransform();
        return;
      }

      if (e.touches.length === 1) {
        const t = e.touches[0];
        lastPointerClientPos.x = t.clientX;
        lastPointerClientPos.y = t.clientY;
        updateCanvasDragMove(t.clientX, t.clientY);
        if (activeCanvasGesture && (activeCanvasGesture.moved || activeCanvasGesture.type !== "touch-pan")) {
          e.preventDefault();
        }
      }
    },
    { passive: false }
  );

  function handleCanvasTouchEndOrCancel(e) {
    if (!activeCanvasGesture || !activeCanvasGesture.isTouch) return;

    if (activeCanvasGesture.type === "touch-pinch-pan" && e.touches && e.touches.length === 1) {
      const rem = e.touches[0];
      activeCanvasGesture = {
        type: "touch-pan",
        isTouch: true,
        viewport: activeCanvasGesture.viewport,
        startClientX: rem.clientX,
        startClientY: rem.clientY,
        startPanX: editorCanvasState.panX,
        startPanY: editorCanvasState.panY,
        moved: true,
        startedInteractive: false,
        startedNodeId: "",
        startedOnEmptyCanvas: false
      };
      return;
    }

    if (e.touches && e.touches.length > 0) return;

    const wasMoved = Boolean(activeCanvasGesture.moved);
    finishActiveCanvasGesture();
    if (wasMoved && e.cancelable) {
      e.preventDefault();
    }
  }

  document.addEventListener("touchend", handleCanvasTouchEndOrCancel, { passive: false });
  document.addEventListener("touchcancel", handleCanvasTouchEndOrCancel, { passive: false });

  document.addEventListener("mousedown", (e) => {
    const viewport = e.target.closest("#timeline-scroll-viewport");
    if (!viewport) {
      closeCanvasContextMenu();
      hideFloatingChapterInput(true);
      return;
    }

    if (e.target.closest("#canvas-context-menu, #canvas-chapter-input, #editor-empty-state")) {
      return;
    }
    closeCanvasContextMenu();
    hideFloatingChapterInput(true);

    if (e.button === 1 || (e.button === 0 && editorCanvasState.spacePressed)) {
      e.preventDefault();
      viewport.classList.add("is-panning");
      activeCanvasGesture = {
        type: "pan",
        startClientX: e.clientX,
        startClientY: e.clientY,
        startPanX: editorCanvasState.panX,
        startPanY: editorCanvasState.panY
      };
      return;
    }

    if (e.button === 2 && !e.target.closest("input, textarea")) {
      activeCanvasGesture = {
        type: "right-pan",
        startClientX: e.clientX,
        startClientY: e.clientY,
        startPanX: editorCanvasState.panX,
        startPanY: editorCanvasState.panY,
        moved: false
      };
      return;
    }

    if (e.button !== 0) return;

    if (
      document.activeElement &&
      document.activeElement !== e.target &&
      typeof document.activeElement.blur === "function" &&
      document.activeElement.matches &&
      document.activeElement.matches("input, textarea, select")
    ) {
      document.activeElement.blur();
    }

    const ptWorld = clientToStudioWorld(viewport, e.clientX, e.clientY);
    const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);

    if (hit.type === "chapter-out-port") {
      e.preventDefault();
      e.stopPropagation();
      beginChapterWireConnectGesture(hit, viewport, false);
      return;
    }

    if (hit.type === "scene-out-port") {
      e.preventDefault();
      e.stopPropagation();
      beginWireConnectGesture(hit, viewport, false);
      return;
    }

    if (hit.type === "add-choice-btn") {
      e.preventDefault();
      e.stopPropagation();
      triggerCanvasAddChoiceForNode(hit.node);
      return;
    }

    if (hit.type === "chapter-grip" || hit.type === "chapter-title") {
      e.preventDefault();
      beginChapterDragGesture(hit.chapter, viewport, e.clientX, e.clientY, false, hit.type === "chapter-title");
      return;
    }

    const isMultiModifier = Boolean(e.shiftKey || e.metaKey || e.ctrlKey);

    if (hit.type === "node-grip") {
      e.preventDefault();
      beginNodeDragGesture(hit.node, viewport, e.clientX, e.clientY, false, isMultiModifier);
      return;
    }

    if (hit.type === "node-card" || hit.type === "scene-in-port") {
      e.preventDefault();
      const sceneId = hit.node.id;
      if (isMultiModifier) {
        if (selectedNodeIds.has(sceneId)) {
          selectedNodeIds.delete(sceneId);
        } else if (sceneId) {
          selectedNodeIds.add(sceneId);
        }
        editorSelectionCleared = selectedNodeIds.size === 0;
        syncNodeSelectionDOM();
        renderStudioCanvas();
        if (selectedNodeIds.size === 1) {
          selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
        }
        return;
      }
      editorSelectionCleared = false;
      selectedNodeIds.clear();
      if (sceneId) selectedNodeIds.add(sceneId);
      syncNodeSelectionDOM();
      renderStudioCanvas();
      selectNodeForContextPanel(sceneId);
      return;
    }

    // Empty canvas background: start rubber-band marquee multi-selection
    e.preventDefault();
    const isAdditive = Boolean(e.shiftKey || e.metaKey || e.ctrlKey);
    const baseSelection = isAdditive ? new Set(selectedNodeIds) : new Set();
    if (!isAdditive) {
      editorSelectionCleared = true;
      if (selectedNodeIds.size > 0) {
        selectedNodeIds.clear();
      }
      syncNodeSelectionDOM();
      hideEditorContextPanel();
      renderStudioCanvas();
    }
    const vpRect = viewport.getBoundingClientRect();
    activeCanvasGesture = {
      type: "marquee-select",
      viewport: viewport,
      vpRect: vpRect,
      startClientX: e.clientX,
      startClientY: e.clientY,
      baseSelection: baseSelection,
      marqueeRect: null,
      moved: false
    };
  });

  document.addEventListener("mousemove", (e) => {
    lastPointerClientPos.x = e.clientX;
    lastPointerClientPos.y = e.clientY;
    if (activeCanvasGesture) {
      if (!activeCanvasGesture.isTouch) {
        updateCanvasDragMove(e.clientX, e.clientY);
      }
      return;
    }
    const viewport = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
    if (viewport && !viewport.contains(e.target)) {
      if (studioGraphState.hoveredItem) {
        studioGraphState.hoveredItem = null;
        studioGraphState.overlayBatchDirty = true;
        renderStudioCanvas();
      }
      return;
    }
    if (!viewport || e.target.closest("#canvas-context-menu, #canvas-chapter-input")) {
      if (studioGraphState.hoveredItem) {
        studioGraphState.hoveredItem = null;
        studioGraphState.overlayBatchDirty = true;
        renderStudioCanvas();
      }
      return;
    }
    const ptWorld = clientToStudioWorld(viewport, e.clientX, e.clientY);
    const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);
    updateCanvasHoverCursor(viewport, hit);
    const nextHover = hit.type === "empty" ? null : hit;
    if (!isSameHoveredCanvasItem(studioGraphState.hoveredItem, nextHover)) {
      studioGraphState.hoveredItem = nextHover;
      studioGraphState.overlayBatchDirty = true;
      renderStudioCanvas();
    }
  });

  document.addEventListener("mouseup", () => {
    if (!activeCanvasGesture || activeCanvasGesture.isTouch) return;
    finishActiveCanvasGesture();
  });

  document.addEventListener("contextmenu", (e) => {
    const vp = e.target.closest("#timeline-scroll-viewport");
    if (!vp) return;
    if (e.target.closest("input, textarea")) return;
    e.preventDefault();
    if (editorCanvasState.suppressNextContextMenu) {
      editorCanvasState.suppressNextContextMenu = false;
      return;
    }
    closeAllCustomSelects(null);
    const rect = vp.getBoundingClientRect();
    const z = editorCanvasState.zoom || 1.0;
    const vx = e.clientX - rect.left;
    const vy = e.clientY - rect.top;
    pendingContextMenuWorldPos = {
      x: (vx - editorCanvasState.panX) / z - 96,
      y: (vy - editorCanvasState.panY) / z - 20
    };
    const menu = qs("#canvas-context-menu", vp);
    if (menu) {
      menu.style.left = Math.max(8, Math.min(rect.width - 230, vx)) + "px";
      menu.style.top = Math.max(8, Math.min(rect.height - 150, vy)) + "px";
      menu.classList.remove("hidden");
    }
  });

  document.addEventListener("dblclick", (e) => {
    const vp = e.target.closest("#timeline-scroll-viewport");
    if (!vp) return;
    if (e.target.closest("#canvas-context-menu, #canvas-chapter-input")) return;
    const ptWorld = clientToStudioWorld(vp, e.clientX, e.clientY);
    const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);
    if (hit.type === "chapter-title" || hit.type === "chapter-grip") {
      e.preventDefault();
      openFloatingChapterInput(hit.chapter);
      return;
    }
    if (hit.type !== "empty") return;
    e.preventDefault();
    createSceneNodeAtWorldCoords(ptWorld.wx - 96, ptWorld.wy - 20);
  });

  document.addEventListener("selectstart", (e) => {
    if (!e.target || typeof e.target.closest !== "function") return;
    if (e.target.closest("input, textarea")) return;
    if (e.target.closest("#timeline-scroll-viewport")) {
      e.preventDefault();
    }
  });

  document.addEventListener("input", (e) => {
    if (e.target && e.target.classList && e.target.classList.contains("chapter-group-title-input")) {
      scheduleChapterLiveSave(e.target);
      return;
    }
    if (e.target && e.target.classList && e.target.classList.contains("canvas-book-title-input")) {
      scheduleBookTitleLiveSave(e.target);
      return;
    }
    const form = e.target.closest("form.editor-context-form, form#form-scene-text");
    if (!form) return;
    const sceneIdInput = qs('input[name="scene_id"]', form);
    const sceneId =
      form.getAttribute("data-context-scene-id") ||
      (sceneIdInput ? sceneIdInput.value : "") ||
      "";
    const nd = sceneId ? studioGraphState.nodeById.get(sceneId) : null;
    if (e.target && e.target.name === "chapter" && nd) {
      nd.chapter = e.target.value || "Rozdział 1";
      nd.chapterLower = nd.chapter.toLowerCase();
      updateChapterGroupFrames();
      drawStudioWires();
    }
    if (e.target && (e.target.name === "name" || e.target.name === "explicit_id") && nd) {
      const nameInput = qs('input[name="name"]', form);
      const expIdInput = qs('input[name="explicit_id"]', form);
      const displayTitle =
        (nameInput && nameInput.value.trim()) ||
        (expIdInput && expIdInput.value.trim()) ||
        sceneId;
      nd.name = displayTitle;
      studioGraphState.staticBatchDirty = true;
      renderStudioCanvas();
    }
    if (e.target && e.target.name && e.target.name.indexOf("choice_label_") === 0 && nd) {
      const idx = Number(e.target.getAttribute("data-choice-idx") || e.target.name.replace("choice_label_", ""));
      if (nd.ports && nd.ports[idx]) {
        nd.ports[idx].label = e.target.value || "Dalej";
        studioGraphState.staticBatchDirty = true;
        renderStudioCanvas();
      }
    }
    scheduleNodeLiveSave(form);
  });

  document.addEventListener("change", (e) => {
    if (e.target && e.target.classList && e.target.classList.contains("chapter-group-title-input")) {
      commitChapterTitleInput(e.target);
    } else if (e.target && e.target.classList && e.target.classList.contains("canvas-book-title-input")) {
      commitBookTitleInput(e.target);
    }
  });

  document.addEventListener(
    "blur",
    (e) => {
      if (e.target && e.target.classList && e.target.classList.contains("chapter-group-title-input")) {
        hideFloatingChapterInput(true);
      } else if (e.target && e.target.classList && e.target.classList.contains("canvas-book-title-input")) {
        commitBookTitleInput(e.target);
      }
    },
    true
  );
}
