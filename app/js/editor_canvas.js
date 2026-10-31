"use strict";

function snapToEditorGrid(val) {
  return Math.round(val / EDITOR_GRID_SIZE) * EDITOR_GRID_SIZE;
}

function closeCanvasContextMenu() {
  const menu = qs("#canvas-context-menu");
  if (menu) {
    menu.classList.add("hidden");
  }
}

function recomputeNodeMetrics(node) {
  node.w = NODE_CARD_WIDTH;
  const portCount = node.hasChoices && node.ports ? node.ports.length : 0;
  node.h = portCount > 0 ? NODE_BASE_HEIGHT + portCount * NODE_CHOICE_ROW_HEIGHT : NODE_BASE_HEIGHT;
  node.left = node.x;
  node.right = node.x + node.w;
  node.top = node.y;
  node.bottom = node.y + node.h;
  node.inOx = NODE_CARD_WIDTH * 0.5;
  node.inOy = 0;
  node.hasInputPort = !node.isStart;

  if (portCount > 0) {
    for (let i = 0; i < node.ports.length; i++) {
      const p = node.ports[i];
      p.isBottom = false;
      p.ox = NODE_CARD_WIDTH;
      p.oy = NODE_HEADER_HEIGHT + i * NODE_CHOICE_ROW_HEIGHT + NODE_CHOICE_ROW_HEIGHT * 0.5;
      p.connected = Boolean((p.targetId && p.targetId.toUpperCase() !== "END") || (p.targetChapter && p.targetChapter.trim()));
    }
    node.implicitPort = null;
  } else {
    const impTarget = node.impTarget || "";
    const impChapter = node.impChapter || "";
    const impConnected = Boolean((impTarget && impTarget.toUpperCase() !== "END") || (impChapter && impChapter.trim()));
    if (!node.implicitPort) {
      node.implicitPort = {
        idx: 0,
        label: "Dalej",
        targetId: impTarget,
        targetChapter: impChapter,
        connected: impConnected,
        isBottom: true,
        ox: NODE_CARD_WIDTH * 0.5,
        oy: node.h
      };
    } else {
      node.implicitPort.targetId = impTarget;
      node.implicitPort.targetChapter = impChapter;
      node.implicitPort.connected = impConnected;
      node.implicitPort.isBottom = true;
      node.implicitPort.ox = NODE_CARD_WIDTH * 0.5;
      node.implicitPort.oy = node.h;
    }
  }
}

function recomputeChapterPortPositions(ch) {
  ch.left = ch.x;
  ch.right = ch.x + ch.w;
  ch.top = ch.y;
  ch.bottom = ch.y + ch.h;
  ch.inX = ch.x + ch.w * 0.5;
  ch.inY = ch.y;
  if (!ch.outPorts || ch.outPorts.length === 0) {
    ch.outPorts = [{ idx: 0, target: "", targetLower: "", connected: false, x: ch.x + ch.w * 0.5, y: ch.y + ch.h }];
    return;
  }
  const m = ch.outPorts.length;
  for (let i = 0; i < m; i++) {
    const op = ch.outPorts[i];
    op.idx = i;
    op.connected = Boolean(op.targetLower);
    op.x = ch.x + ch.w * 0.5 + (i - (m - 1) * 0.5) * CHAPTER_OUT_PORT_PITCH;
    op.y = ch.y + ch.h;
  }
}

function parseStudioGraphPayload(rawText) {
  studioGraphState.chapters = [];
  studioGraphState.chapterByKey.clear();
  studioGraphState.nodes = [];
  studioGraphState.nodeById.clear();
  studioGraphState.wires = [];
  studioGraphState.bundleOverlays = [];
  studioGraphState.hoveredItem = null;
  studioGraphState.staticBatchDirty = true;
  studioGraphState.overlayBatchDirty = true;

  let vpInfo = { panX: 0, panY: 0, zoom: 1.0, hasView: false };
  if (!rawText) return vpInfo;

  const lines = rawText.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line) continue;
    const parts = line.split(" | ");
    const kind = parts[0].trim();
    const kv = {};
    for (let j = 1; j < parts.length; j++) {
      const seg = parts[j];
      const colonIdx = seg.indexOf(":");
      if (colonIdx > 0) {
        const k = seg.slice(0, colonIdx).trim();
        const v = seg.slice(colonIdx + 1).trim();
        kv[k] = v;
      }
    }

    if (kind === "viewport") {
      vpInfo.panX = parseFloat(kv.pan_x || "0") || 0;
      vpInfo.panY = parseFloat(kv.pan_y || "0") || 0;
      vpInfo.zoom = parseFloat(kv.zoom || "1") || 1.0;
      vpInfo.hasView = kv.has_view === "1";
    } else if (kind === "chapter") {
      const outsRaw = (kv.outs || "").trim();
      const outRefs = outsRaw ? outsRaw.split(",").map((s) => s.trim()).filter(Boolean) : [];
      const outPorts =
        outRefs.length > 0
          ? outRefs.map((ref, idx) => ({
              idx: idx,
              target: ref,
              targetLower: ref.toLowerCase(),
              x: 0,
              y: 0
            }))
          : [{ idx: 0, target: "", targetLower: "", x: 0, y: 0 }];

      const chName = kv.name || kv.ref || "Rozdział 1";
      const chRef = kv.ref || chName;
      const ch = {
        id: "ch:" + (chRef || chName).toLowerCase(),
        rawId: kv.id || chRef,
        ref: chRef,
        refLower: chRef.toLowerCase(),
        name: chName,
        nameLower: chName.toLowerCase(),
        isRoot: kv.root === "1",
        startSceneId: kv.start || "",
        x: parseInt(kv.x || "0", 10) || 0,
        y: parseInt(kv.y || "0", 10) || 0,
        w: parseInt(kv.w || "240", 10) || 240,
        h: parseInt(kv.h || "140", 10) || 140,
        outPorts: outPorts,
        hidden: false
      };
      recomputeChapterPortPositions(ch);
      studioGraphState.chapters.push(ch);
      if (ch.nameLower) studioGraphState.chapterByKey.set(ch.nameLower, ch);
      if (ch.refLower) studioGraphState.chapterByKey.set(ch.refLower, ch);
    } else if (kind === "node") {
      const id = kv.id || "";
      if (!id) continue;
      const chName = kv.chapter || "Rozdział 1";
      const node = {
        id: id,
        name: kv.name || id,
        chapter: chName,
        chapterLower: chName.toLowerCase(),
        x: parseInt(kv.x || "0", 10) || 0,
        y: parseInt(kv.y || "0", 10) || 0,
        selected: kv.selected === "1",
        isStart: kv.start === "1",
        isEntry: kv.entry === "1",
        unreachable: kv.unreachable === "1",
        hasChoices: kv.has_choices === "1",
        impTarget: kv.imp_target || "",
        impChapter: kv.imp_chapter || "",
        ports: []
      };
      recomputeNodeMetrics(node);
      studioGraphState.nodes.push(node);
      studioGraphState.nodeById.set(id, node);
    } else if (kind === "port") {
      const sceneId = kv.scene || "";
      const node = studioGraphState.nodeById.get(sceneId);
      if (!node) continue;
      const idx = parseInt(kv.idx || String(node.ports.length), 10) || 0;
      node.ports.push({
        idx: idx,
        label: kv.label || "Wybór " + (idx + 1),
        targetId: kv.target_id || "",
        targetChapter: kv.target_chapter || "",
        isBottom: false,
        ox: NODE_CARD_WIDTH,
        oy: NODE_HEADER_HEIGHT + idx * NODE_CHOICE_ROW_HEIGHT + NODE_CHOICE_ROW_HEIGHT * 0.5
      });
      node.hasChoices = node.ports.length >= 2;
      recomputeNodeMetrics(node);
    }
  }

  return vpInfo;
}

function findClosestChapterAtWorldCoords(worldX, worldY) {
  const px = worldX + 96;
  const py = worldY + 48;
  let bestCh = "";
  let bestDistSq = Infinity;
  for (let i = 0; i < studioGraphState.chapters.length; i++) {
    const ch = studioGraphState.chapters[i];
    if (ch.hidden || !ch.name) continue;
    const dx = px < ch.x ? ch.x - px : px > ch.x + ch.w ? px - (ch.x + ch.w) : 0;
    const dy = py < ch.y ? ch.y - py : py > ch.y + ch.h ? py - (ch.y + ch.h) : 0;
    const dSq = dx * dx + dy * dy;
    if (dSq < bestDistSq) {
      bestDistSq = dSq;
      bestCh = ch.name;
    }
  }
  return bestCh;
}

function createSceneNodeAtWorldCoords(worldX, worldY, chapterName) {
  closeCanvasContextMenu();
  hideFloatingChapterInput(false);
  editorSelectionCleared = false;
  selectedNodeIds.clear();
  const nx = snapToEditorGrid(worldX);
  const ny = snapToEditorGrid(worldY);
  const resolvedChapter = chapterName || findClosestChapterAtWorldCoords(nx, ny);
  const params = {
    x: nx,
    y: ny
  };
  if (resolvedChapter) {
    params.chapter = resolvedChapter;
  }
  sendServerAction("CreateSceneAt", params);
}

function resetEditorCanvasViewState() {
  editorCanvasState.initialized = false;
  editorSelectionCleared = false;
  selectedNodeIds.clear();
}

function areAllNodesOutOfViewport(viewport) {
  const nodes = studioGraphState.nodes;
  if (nodes.length === 0) return false;
  const vw = studioGraphState.viewW || (viewport && viewport.clientWidth) || window.innerWidth || 1200;
  const vh = studioGraphState.viewH || (viewport && viewport.clientHeight) || window.innerHeight || 700;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    const sx1 = nd.x * z + px;
    const sx2 = (nd.x + nd.w) * z + px;
    const sy1 = nd.y * z + py;
    const sy2 = (nd.y + nd.h) * z + py;
    if (sx2 > 24 && sx1 < vw - 24 && sy2 > 24 && sy1 < vh - 24) {
      return false;
    }
  }
  return true;
}

function fitCanvasToAllNodes(viewport) {
  const nodes = studioGraphState.nodes;
  if (nodes.length === 0) {
    editorCanvasState.zoom = 1.0;
    editorCanvasState.panX = 0;
    editorCanvasState.panY = 0;
    return;
  }

  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    if (nd.x - 24 < minX) minX = nd.x - 24;
    if (nd.y - 52 < minY) minY = nd.y - 52;
    if (nd.x + nd.w + 24 > maxX) maxX = nd.x + nd.w + 24;
    if (nd.y + nd.h + 24 > maxY) maxY = nd.y + nd.h + 24;
  }

  const vw = studioGraphState.viewW || (viewport && viewport.clientWidth) || window.innerWidth || 1200;
  const vh = studioGraphState.viewH || (viewport && viewport.clientHeight) || window.innerHeight || 700;
  const padX = 48;
  const padY = 48;
  const contentW = Math.max(240, maxX - minX);
  const contentH = Math.max(160, maxY - minY);
  const availW = Math.max(200, vw - padX * 2);
  const availH = Math.max(160, vh - padY * 2);

  const idealZoom = Math.min(1.0, availW / contentW, availH / contentH);
  const z = Math.max(0.25, Math.min(1.0, idealZoom));
  const cx = (minX + maxX) * 0.5;
  const cy = (minY + maxY) * 0.5;

  editorCanvasState.zoom = z;
  editorCanvasState.panX = Math.round(vw * 0.5 - cx * z);
  editorCanvasState.panY = Math.round(vh * 0.5 - cy * z);
}

function resetCanvasToFirstNode(viewport) {
  const vw = studioGraphState.viewW || (viewport && viewport.clientWidth) || window.innerWidth || 1200;
  const nodes = studioGraphState.nodes;
  let startNode = null;
  for (let i = 0; i < nodes.length; i++) {
    if (nodes[i].isStart) {
      startNode = nodes[i];
      break;
    }
  }
  if (!startNode && nodes.length > 0) {
    startNode = nodes[0];
  }
  const fx = startNode ? startNode.x + 96 : 960;
  const fy = startNode ? startNode.y : 96;
  editorCanvasState.zoom = 1.0;
  editorCanvasState.panX = Math.round(vw * 0.5 - fx);
  editorCanvasState.panY = Math.round(72 - fy);
}

function scheduleSaveEditorViewport() {
  if (viewportSaveTimer) {
    clearTimeout(viewportSaveTimer);
  }
  viewportSaveTimer = setTimeout(() => {
    viewportSaveTimer = null;
    postServerActionSilent("SaveEditorState", {
      pan_x: Math.round(editorCanvasState.panX),
      pan_y: Math.round(editorCanvasState.panY),
      zoom: editorCanvasState.zoom.toFixed(2)
    });
  }, 220);
}

function saveNodePositionOnServer(sceneId, x, y) {
  saveBatchNodePositionsOnServer([{ sceneId: sceneId, x: x, y: y }]);
}

function saveBatchNodePositionsOnServer(items) {
  if (!items || items.length === 0) return;
  const encoded = items
    .map((it) => it.sceneId + ":" + Math.round(it.x) + "," + Math.round(it.y))
    .join(";");
  const params = {
    positions: encoded,
    pan_x: Math.round(editorCanvasState.panX),
    pan_y: Math.round(editorCanvasState.panY),
    zoom: editorCanvasState.zoom.toFixed(2)
  };
  if (items.length === 1) {
    params.scene_id = items[0].sceneId;
    params.x = Math.round(items[0].x);
    params.y = Math.round(items[0].y);
  }
  postServerActionSilent("SaveEditorState", params);
}

function positionFloatingChapterInput() {
  const inp = qs("#canvas-chapter-input");
  const ch = studioGraphState.editingChapter;
  if (!inp || !ch || inp.classList.contains("hidden")) return;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;
  const left = (ch.x + 26) * z + px;
  const top = (ch.y + 2) * z + py;
  const width = Math.max(80, (ch.w - 34) * z);
  const height = Math.max(20, 24 * z);
  inp.style.left = Math.round(left) + "px";
  inp.style.top = Math.round(top) + "px";
  inp.style.width = Math.round(width) + "px";
  inp.style.height = Math.round(height) + "px";
}

function openFloatingChapterInput(ch) {
  const inp = qs("#canvas-chapter-input");
  if (!inp || !ch) return;
  studioGraphState.editingChapter = ch;
  studioGraphState.staticBatchDirty = true;
  inp.value = ch.name;
  inp.setAttribute("data-old-chapter", ch.name);
  inp.classList.remove("hidden");
  positionFloatingChapterInput();
  inp.focus();
  inp.select();
  renderStudioCanvas();
}

function hideFloatingChapterInput(shouldCommit) {
  const inp = qs("#canvas-chapter-input");
  if (!inp || inp.classList.contains("hidden")) return;
  if (shouldCommit && typeof commitChapterTitleInput === "function") {
    commitChapterTitleInput(inp);
  }
  studioGraphState.editingChapter = null;
  studioGraphState.staticBatchDirty = true;
  inp.classList.add("hidden");
  renderStudioCanvas();
}

function applyEditorCanvasTransform() {
  if (isNaN(editorCanvasState.zoom) || editorCanvasState.zoom <= 0) {
    editorCanvasState.zoom = 1.0;
  }
  if (isNaN(editorCanvasState.panX)) {
    editorCanvasState.panX = 0;
  }
  if (isNaN(editorCanvasState.panY)) {
    editorCanvasState.panY = 0;
  }

  const z = editorCanvasState.zoom || 1.0;
  const pct = Math.round(z * 100);
  if (pct !== lastDisplayedZoomPct) {
    lastDisplayedZoomPct = pct;
    const resetBtn = qs("#btn-zoom-reset");
    if (resetBtn) {
      resetBtn.textContent = pct + "%";
    }
  }

  positionFloatingChapterInput();
  renderStudioCanvas();
}

function zoomEditorAtPoint(newZoom, clientX, clientY) {
  const viewport = qs("#timeline-scroll-viewport");
  if (!viewport) return;

  const clamped = Math.max(0.25, Math.min(2.5, newZoom));
  const rect = viewport.getBoundingClientRect();
  const safeClientX =
    typeof clientX === "number" && !isNaN(clientX)
      ? clientX
      : typeof lastPointerClientPos.x === "number" && !isNaN(lastPointerClientPos.x)
        ? lastPointerClientPos.x
        : rect.left + rect.width * 0.5;
  const safeClientY =
    typeof clientY === "number" && !isNaN(clientY)
      ? clientY
      : typeof lastPointerClientPos.y === "number" && !isNaN(lastPointerClientPos.y)
        ? lastPointerClientPos.y
        : rect.top + rect.height * 0.5;
  const vx = safeClientX - rect.left;
  const vy = safeClientY - rect.top;

  const oldZoom = editorCanvasState.zoom || 1.0;
  const worldX = (vx - editorCanvasState.panX) / oldZoom;
  const worldY = (vy - editorCanvasState.panY) / oldZoom;

  editorCanvasState.zoom = clamped;
  editorCanvasState.panX = vx - worldX * clamped;
  editorCanvasState.panY = vy - worldY * clamped;

  applyEditorCanvasTransform();
  scheduleSaveEditorViewport();
}

function updateChapterGroupFrames() {
  const nodes = studioGraphState.nodes;
  const chapters = studioGraphState.chapters;
  if (!nodes || nodes.length === 0) return;

  const byChapter = new Map();
  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    const key = (nd.chapter || "Rozdział 1").trim().toLowerCase();
    let list = byChapter.get(key);
    if (!list) {
      list = [];
      byChapter.set(key, list);
    }
    list.push(nd);
  }

  byChapter.forEach((memberNodes, key) => {
    let ch = studioGraphState.chapterByKey.get(key);
    if (!ch) {
      const displayCh = memberNodes[0].chapter || "Rozdział 1";
      ch = {
        id: "ch:" + key,
        rawId: key,
        ref: displayCh,
        refLower: key,
        name: displayCh,
        nameLower: key,
        isRoot: chapters.length === 0,
        startSceneId: memberNodes[0].id,
        x: 0,
        y: 0,
        w: 240,
        h: 140,
        outPorts: [{ idx: 0, target: "", targetLower: "", x: 0, y: 0 }],
        hidden: false
      };
      chapters.push(ch);
      studioGraphState.chapterByKey.set(key, ch);
    }
  });

  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    const matching = byChapter.get(ch.nameLower) || byChapter.get(ch.refLower) || [];
    if (matching.length === 0) {
      ch.hidden = true;
      continue;
    }
    ch.hidden = false;

    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (let j = 0; j < matching.length; j++) {
      const nd = matching[j];
      const h = Math.max(nd.h || 74, 120);
      if (nd.x < minX) minX = nd.x;
      if (nd.y < minY) minY = nd.y;
      if (nd.x + nd.w > maxX) maxX = nd.x + nd.w;
      if (nd.y + h > maxY) maxY = nd.y + h;
    }
    ch.x = minX - 24;
    ch.y = minY - 52;
    ch.w = maxX - minX + 48;
    ch.h = maxY - minY + 96;
    recomputeChapterPortPositions(ch);
  }
  studioGraphState.staticBatchDirty = true;
}

function syncCanvasBackingStoreSize(canvas, viewport) {
  const vw = Math.max(1, viewport.clientWidth || 1200);
  const vh = Math.max(1, viewport.clientHeight || 700);
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  studioGraphState.viewW = vw;
  studioGraphState.viewH = vh;
  studioGraphState.canvasDpr = dpr;
  const targetW = Math.max(1, Math.round(vw * dpr));
  const targetH = Math.max(1, Math.round(vh * dpr));
  if (canvas.width !== targetW || canvas.height !== targetH) {
    canvas.width = targetW;
    canvas.height = targetH;
  }
}

function bindStudioViewportResizeObserver(viewport, canvas) {
  if (observedViewportEl === viewport) return;
  observedViewportEl = viewport;
  if (studioViewportResizeObserver) {
    studioViewportResizeObserver.disconnect();
  }
  if (typeof ResizeObserver === "function") {
    studioViewportResizeObserver = new ResizeObserver(() => {
      const vp = qs("#timeline-scroll-viewport");
      const cv = qs("#studio-canvas");
      if (!vp || !cv) return;
      syncCanvasBackingStoreSize(cv, vp);
      renderStudioCanvas();
    });
    studioViewportResizeObserver.observe(viewport);
  }
}

function initNodeEditorCanvas() {
  const viewport = qs("#timeline-scroll-viewport");
  const canvas = qs("#studio-canvas");
  if (!viewport || !canvas) return;

  ensureBuildTimeMSDFMetricsParsed();
  startPreloadingMSDFAtlasImage();
  syncCanvasBackingStoreSize(canvas, viewport);
  bindStudioViewportResizeObserver(viewport, canvas);
  invalidateStudioThemeCache();
  hideFloatingChapterInput(false);

  const dataPre = qs("#studio-graph-data", viewport);
  const rawPayload = dataPre ? dataPre.textContent || "" : "";
  const vpInfo = parseStudioGraphPayload(rawPayload);

  const existingNodeIds = new Set();
  for (let i = 0; i < studioGraphState.nodes.length; i++) {
    existingNodeIds.add(studioGraphState.nodes[i].id);
  }

  const hadSelectionBeforePrune = selectedNodeIds.size > 0;
  Array.from(selectedNodeIds).forEach((id) => {
    if (!existingNodeIds.has(id)) {
      selectedNodeIds.delete(id);
    }
  });
  if (hadSelectionBeforePrune && selectedNodeIds.size === 0) {
    editorSelectionCleared = true;
  }

  if (!editorCanvasState.initialized) {
    if (vpInfo.hasView) {
      editorCanvasState.panX = isNaN(vpInfo.panX) ? 0 : vpInfo.panX;
      editorCanvasState.panY = isNaN(vpInfo.panY) ? 0 : vpInfo.panY;
      editorCanvasState.zoom = isNaN(vpInfo.zoom) ? 1.0 : Math.max(0.25, Math.min(2.5, vpInfo.zoom));
      if (areAllNodesOutOfViewport(viewport)) {
        fitCanvasToAllNodes(viewport);
      }
    } else {
      resetCanvasToFirstNode(viewport);
    }
    editorCanvasState.initialized = true;
    editorSelectionCleared = false;
  }

  const contextForm = qs("#editor-context-form");
  const chapterForm = qs("#editor-chapter-form");
  const bookForm = qs("#editor-book-form");
  const contextPanel = qs("#editor-context-panel");
  const isChapterOpen = Boolean(chapterForm && !chapterForm.classList.contains("hidden"));
  const isBookOpen = Boolean(bookForm && !bookForm.classList.contains("hidden"));
  if (isChapterOpen || isBookOpen) {
    selectedNodeIds.clear();
    studioGraphState.selectedChapterKey = isChapterOpen
      ? (chapterForm.getAttribute("data-context-chapter") || "").trim().toLowerCase()
      : "";
  } else {
    studioGraphState.selectedChapterKey = "";
    const serverSelectedId = contextForm ? contextForm.getAttribute("data-context-scene-id") || "" : "";
    if (
      !editorSelectionCleared &&
      serverSelectedId &&
      contextPanel &&
      !contextPanel.classList.contains("hidden") &&
      selectedNodeIds.size <= 1
    ) {
      selectedNodeIds.clear();
      selectedNodeIds.add(serverSelectedId);
    }
  }

  updateChapterGroupFrames();
  syncNodeSelectionDOM();
  drawStudioWires();
  applyEditorCanvasTransform();
}
