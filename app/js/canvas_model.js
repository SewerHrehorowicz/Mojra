"use strict";

const CanvasModel = {
  EDITOR_GRID_SIZE: 24,
  NODE_CARD_WIDTH: 192,
  NODE_HEADER_HEIGHT: 34,
  NODE_CHOICE_ROW_HEIGHT: 24,
  NODE_FOOTER_HEIGHT: 38,
  NODE_BASE_HEIGHT: 72,
  PORT_RADIUS: 6.5,
  CHAPTER_PORT_RADIUS: 3.5,
  CHAPTER_OUT_PORT_PITCH: 37,
  WIRE_GRID: 24,
  WIRE_LANE_STEP: 6,
  UI_ATLAS_W: 256,
  UI_ATLAS_H: 128,
  FONT_ATLAS_SIZE: 512,
  MSDF_BASE_EM_PX: 23.0,
  MSDF_PIXEL_RANGE: 3.0,
  MSDF_FONT_ALEO_REG: 0,
  MSDF_FONT_LATO_REG: 1,
  MSDF_FONT_LATO_BOLD: 2,
  FONT_TITLE: 0,
  FONT_CHOICE: 1,
  FONT_CHAPTER: 2,
  FONT_BADGE: 3,
  FONT_BTN: 4,

  CARD: {
    w: 192,
    halfW: 96,
    headerH: 34,
    choiceH: 24,
    footerH: 38,
    baseH: 72,
    minChapterH: 120,
    btnSize: 24,
    pad: 8
  },

  PAD: {
    x: 36,
    top: 52,
    bottom: 44,
    w: 72,
    h: 96
  },

  GAP: {
    grid: 24,
    chapter: 24,
    node: 48
  },

  SPATIAL: {
    shift: 8,
    minObstacles: 12
  },

  UV: {
    grid: -100000.0,
    gridTest: -80000.0,
    gridOffset: 200000.0,
    screen: -50000.0,
    screenTest: -30000.0,
    solid: 30.0,
    fontLg: 20.0,
    fontSm: 10.0,
    dashDuty: 0.54
  },

  WIRE: {
    grid: 24,
    laneStep: 6,
    pad: 14,
    tightPad: 10,
    collectorOffset: 12,
    bboxPad: 18
  },

  FONT_STYLES: [
    { ttfId: 0, size: 13, track: 0.0 },
    { ttfId: 1, size: 11, track: 0.0 },
    { ttfId: 2, size: 11, track: 0.65 },
    { ttfId: 2, size: 9, track: 0.45 },
    { ttfId: 1, size: 10, track: 0.6 }
  ],

  editorCanvasState: {
    panX: 0,
    panY: 0,
    zoom: 1.0,
    bookId: "",
    initialized: false,
    spacePressed: false,
    gestureBaseZoom: 1.0,
    suppressNextContextMenu: false
  },

  studioGraphState: {
    chapters: [],
    chapterByKey: new Map(),
    nodes: [],
    nodeById: new Map(),
    wires: [],
    wirePins: new Map(),
    collectorBars: [],
    hoveredItem: null,
    editingChapter: null,
    editingNodeTitle: null,
    editingChoice: null,
    selectedChapterKey: "",
    searchQuery: "",
    searchMatchSet: null,
    themeColors: null,
    shapedTextCache: new Map(),
    viewW: 1200,
    viewH: 700,
    canvasDpr: 2,
    staticBatchDirty: true,
    overlayBatchDirty: true,
    lastRouteMs: 0,
    lastDrawCalls: 1,
    hasSavedView: false,
    savedPanX: 0,
    savedPanY: 0,
    savedZoom: 1.0
  },

  selectedNodeIds: new Set(),
  selectedChapterName: "",
  editorSelectionCleared: false,
  activeCanvasGesture: null,
  viewportSaveTimer: null,
  pendingContextMenuWorldPos: { x: 768, y: 120 },
  suppressNextTouchClickUntil: 0,
  lastPointerClientPos: { x: null, y: null },
  lastTouchGestureEndPos: { x: null, y: null },
  suppressAnyTouchClick: false,

  inRect(px, py, x, y, w, h) {
    return px >= x && px <= x + w && py >= y && py <= y + h;
  },

  choiceRowY(idx) {
    return this.CARD.headerH + idx * this.CARD.choiceH;
  },

  toSpatialCell(coord) {
    return coord >> this.SPATIAL.shift;
  },

  snapToEditorGrid(val) {
    return Math.round(val / this.EDITOR_GRID_SIZE) * this.EDITOR_GRID_SIZE;
  },

  snapToWireGrid(v) {
    return Math.round(v / this.WIRE_GRID) * this.WIRE_GRID;
  },

  snapToWirePinGrid(val) {
    return Math.round(val / this.WIRE_LANE_STEP) * this.WIRE_LANE_STEP;
  },

  isHistoryPreview() {
    const view = document.querySelector("#view-editor");
    return Boolean(view && view.getAttribute("data-history-preview") === "true");
  },

  getNodeFooterBtnX(node, kind, isPreview) {
    if (kind === "add") return node.x + this.CARD.pad;
    if (kind === "delete") return node.x + node.w - this.CARD.pad - this.CARD.btnSize;
    const offset = isPreview ? this.CARD.btnSize : this.CARD.btnSize * 2 + 6;
    return node.x + node.w - this.CARD.pad - offset;
  },

  findStudioNodeByRef(rawRef) {
    const key = String(rawRef || "").trim().toLowerCase();
    if (!key) return null;
    const nodes = this.studioGraphState.nodes;
    for (let i = 0; i < nodes.length; i++) {
      const nd = nodes[i];
      if ((nd.id && nd.id.toLowerCase() === key) || (nd.name && nd.name.toLowerCase() === key)) {
        return nd;
      }
    }
    return null;
  },

  recomputeNodeMetrics(node) {
    const CARD = this.CARD;
    node.w = CARD.w;
    const portCount = node.hasChoices && node.ports ? node.ports.length : 0;
    node.h = portCount > 0 ? CARD.baseH + portCount * CARD.choiceH : CARD.baseH;
    node.left = node.x;
    node.right = node.x + node.w;
    node.top = node.y;
    node.bottom = node.y + node.h;
    node.inX = node.x + CARD.halfW;
    node.inY = node.y;
    node.hasInputPort = !node.isStart;

    if (node.branchRole === "primary" || node.branchRole === "secondary") {
      const estBadgeOffset = node.isStart ? 54 : node.isEntry ? 64 : 0;
      node.branchRadioX = node.x + node.w - 16 - estBadgeOffset;
      node.branchRadioY = node.y + 13;
    } else {
      node.branchRadioX = 0;
      node.branchRadioY = 0;
    }

    if (portCount > 0) {
      for (let i = 0; i < node.ports.length; i++) {
        const p = node.ports[i];
        const oy = this.choiceRowY(i) + CARD.choiceH * 0.5;
        p.isBottom = false;
        p.ox = CARD.w;
        p.oy = oy;
        p.x = node.x + CARD.w;
        p.y = node.y + oy;
        p.connected = Boolean((p.targetId && p.targetId.toUpperCase() !== "END") || (p.targetChapter && p.targetChapter.trim()));
      }
      node.implicitPort = null;
    } else {
      const implicitTarget = node.implicitTarget || node.impTarget || "";
      const implicitChapter = node.implicitChapter || node.impChapter || "";
      const impConnected = Boolean((implicitTarget && implicitTarget.toUpperCase() !== "END") || (implicitChapter && implicitChapter.trim()));
      if (!node.implicitPort) {
        node.implicitPort = {
          idx: 0, label: AppShellController.tr("next"), targetId: implicitTarget, targetChapter: implicitChapter,
          connected: impConnected, isBottom: true, ox: CARD.halfW, oy: node.h,
          x: node.x + CARD.halfW, y: node.y + node.h
        };
      } else {
        const ip = node.implicitPort;
        ip.targetId = implicitTarget;
        ip.targetChapter = implicitChapter;
        ip.connected = impConnected;
        ip.isBottom = true;
        ip.ox = CARD.halfW;
        ip.oy = node.h;
        ip.x = node.x + CARD.halfW;
        ip.y = node.y + node.h;
      }
    }
  },

  recomputeChapterInputPortPosition(ch) {
    ch.left = ch.x;
    ch.right = ch.x + ch.w;
    ch.top = ch.y;
    ch.bottom = ch.y + ch.h;
    ch.inY = ch.y;
    let desiredInX = ch.x + ch.w * 0.5;
    const chNodes = ch.nodes || this.studioGraphState.nodes;
    if (chNodes && chNodes.length > 0) {
      let entryNode = ch.startSceneId ? this.studioGraphState.nodeById.get(ch.startSceneId) : null;
      if (!entryNode || (entryNode.chapterLower !== ch.nameLower && entryNode.chapterLower !== ch.refLower)) {
        entryNode = null;
        for (let i = 0; i < chNodes.length; i++) {
          const nd = chNodes[i];
          if (nd.chapterLower === ch.nameLower || nd.chapterLower === ch.refLower) {
            if (nd.isEntry || !entryNode) {
              entryNode = nd;
              if (nd.isEntry) break;
            }
          }
        }
      }
      if (entryNode) desiredInX = entryNode.inX;
    }
    const minEdgeX = ch.x + this.GAP.grid;
    const maxEdgeX = Math.max(minEdgeX, ch.x + ch.w - this.GAP.grid);
    ch.inX = Math.max(minEdgeX, Math.min(maxEdgeX, desiredInX));
  },

  recomputeChapterPortPositions(ch) {
    this.recomputeChapterInputPortPosition(ch);
    const minEdgeX = ch.x + this.GAP.grid;
    const maxEdgeX = Math.max(minEdgeX, ch.x + ch.w - 16);
    const minEdgeY = ch.y + this.GAP.node;
    const maxEdgeY = Math.max(minEdgeY, ch.y + ch.h - this.GAP.grid);

    if (!ch.outPorts || ch.outPorts.length === 0) {
      ch.outPorts = [{ idx: 0, target: "", targetLower: "", connected: false, side: "bottom", x: ch.x + ch.w * 0.5, y: ch.y + ch.h }];
    }

    let chNodes = ch.nodes;
    if (!chNodes) {
      const nodes = this.studioGraphState.nodes || [];
      chNodes = [];
      for (let i = 0; i < nodes.length; i++) {
        const nd = nodes[i];
        if (nd.chapterLower === ch.nameLower || nd.chapterLower === ch.refLower) chNodes.push(nd);
      }
      ch.nodes = chNodes;
    }

    const bottomPorts = [], leftPorts = [], rightPorts = [];
    const m = ch.outPorts.length;
    for (let i = 0; i < m; i++) {
      const op = ch.outPorts[i];
      op.connected = Boolean(op.targetLower);
      const toCh = op.targetLower ? this.studioGraphState.chapterByKey.get(op.targetLower) : null;
      const targetX = toCh ? (toCh.inX || toCh.x + toCh.w * 0.5) : ch.x + ch.w * 0.5;
      const targetY = toCh ? (toCh.inY || toCh.y) : ch.y + ch.h + this.CARD.minChapterH;
      let sumX = 0, sumY = 0, maxSrcBottom = ch.y + this.PAD.h, srcCount = 0, lastSrcNode = null, hasBlockedDropToBottom = false;

      if (op.targetLower && chNodes.length > 0) {
        for (let j = 0; j < chNodes.length; j++) {
          const nd = chNodes[j];
          const portList = nd.hasChoices && nd.ports && nd.ports.length > 0 ? nd.ports : nd.implicitPort ? [nd.implicitPort] : [];
          for (let k = 0; k < portList.length; k++) {
            const pt = portList[k];
            const tLower = (pt.targetChapter || "").trim().toLowerCase();
            if (!tLower) continue;
            if (tLower === op.targetLower || (toCh && (tLower === toCh.nameLower || tLower === toCh.refLower))) {
              const sy = pt.y;
              let candX;
              if (pt.isBottom) {
                candX = nd.inX;
                if (EditorWires.linecastVertical(candX, sy + 6, ch.y + ch.h, chNodes, nd, null, 10)) hasBlockedDropToBottom = true;
                candX = EditorWires.resolveClearX(candX, sy + 4, ch.y + ch.h, chNodes, nd, null, 0, 14);
              } else {
                const stubRank = Math.max(0, portList.length - 1 - k);
                const rightLaneX = pt.x + 16 + stubRank * 14;
                if (toCh && toCh.inX <= nd.right && portList.length > 1) {
                  const desiredUnderNodeX = Math.max(nd.x + this.GAP.grid, Math.min(nd.right - this.GAP.grid, toCh.inX || nd.inX));
                  candX = !EditorWires.linecastVertical(desiredUnderNodeX, nd.bottom + 4, ch.y + ch.h, chNodes, nd, null, 10) ? desiredUnderNodeX : rightLaneX;
                } else {
                  candX = rightLaneX;
                }
                if (EditorWires.linecastVertical(candX, (candX <= nd.right ? nd.bottom : sy) + 6, ch.y + ch.h, chNodes, nd, null, 10)) hasBlockedDropToBottom = true;
                candX = EditorWires.resolveClearX(candX, candX <= nd.right ? nd.bottom + 4 : sy, ch.y + ch.h, chNodes, nd, null, 1, 14);
              }
              sumX += candX;
              sumY += sy;
              if (nd.bottom > maxSrcBottom) maxSrcBottom = nd.bottom;
              lastSrcNode = nd;
              srcCount++;
            }
          }
        }
      }

      let side = "bottom";
      if (toCh && !toCh.hidden) {
        const isLeftAdjacent = toCh.right <= ch.left + this.GAP.chapter;
        const isRightAdjacent = toCh.left >= ch.right - this.GAP.chapter;
        const isSideVerticalBand = toCh.top < ch.bottom - this.GAP.chapter;
        if (isLeftAdjacent && (isSideVerticalBand || (hasBlockedDropToBottom && toCh.top <= ch.bottom + this.GAP.chapter))) side = "left";
        else if (isRightAdjacent && (isSideVerticalBand || (hasBlockedDropToBottom && toCh.top <= ch.bottom + this.GAP.chapter))) side = "right";
      }
      op.side = side;

      if (side === "left" || side === "right") {
        op.x = side === "left" ? ch.x : ch.x + ch.w;
        let idealY = srcCount > 0 ? maxSrcBottom + this.GAP.grid : (ch.y + ch.h) * 0.5;
        if (side === "right" && srcCount === 1 && lastSrcNode && !lastSrcNode.implicitPort) {
          const directY = sumY / srcCount;
          if (!EditorWires.linecastHorizontal(directY, lastSrcNode.right + 4, ch.right, chNodes, lastSrcNode, null, 10)) idealY = directY;
        }
        const spanX1 = side === "left" ? ch.left : (lastSrcNode ? lastSrcNode.right + 4 : ch.left + ch.w * 0.5);
        const spanX2 = side === "left" ? (lastSrcNode ? lastSrcNode.right + 20 : ch.left + ch.w * 0.5) : ch.right;
        idealY = EditorWires.resolveClearY(idealY, spanX1, spanX2, chNodes, lastSrcNode, null, 1, 12);
        op._idealY = Math.max(minEdgeY, Math.min(maxEdgeY, Math.round(idealY)));
        op._targetY = targetY;
        if (side === "left") leftPorts.push(op);
        else rightPorts.push(op);
      } else {
        const idealX = srcCount > 0 ? sumX / srcCount : (op.targetLower && toCh ? targetX : (chNodes.length === 1 ? chNodes[0].inX : ch.x + ch.w * 0.5));
        op.y = ch.y + ch.h;
        op._idealX = Math.max(minEdgeX, Math.min(maxEdgeX, idealX));
        op._targetX = targetX;
        bottomPorts.push(op);
      }
    }

    const minGap = this.GAP.grid;
    if (bottomPorts.length > 1) {
      const sortedIdealSlots = bottomPorts.map((p) => p._idealX).sort((a, b) => a - b);
      bottomPorts.sort((a, b) => {
        if (Math.abs(a._targetX - b._targetX) > 12) return a._targetX - b._targetX;
        if (Math.abs(a._idealX - b._idealX) > 6) return a._idealX - b._idealX;
        return (a.idx || 0) - (b.idx || 0);
      });
      for (let i = 0; i < bottomPorts.length; i++) bottomPorts[i]._idealX = sortedIdealSlots[i];
    }
    for (let i = 0; i < bottomPorts.length; i++) {
      bottomPorts[i].x = bottomPorts[i]._idealX;
      bottomPorts[i].y = ch.y + ch.h;
    }
    for (let i = 1; i < bottomPorts.length; i++) {
      if (bottomPorts[i].x < bottomPorts[i - 1].x + minGap) bottomPorts[i].x = bottomPorts[i - 1].x + minGap;
    }
    if (bottomPorts.length > 0 && bottomPorts[bottomPorts.length - 1].x > maxEdgeX) {
      bottomPorts[bottomPorts.length - 1].x = maxEdgeX;
      for (let i = bottomPorts.length - 2; i >= 0; i--) {
        if (bottomPorts[i].x > bottomPorts[i + 1].x - minGap) bottomPorts[i].x = Math.max(minEdgeX, bottomPorts[i + 1].x - minGap);
      }
    }

    const layoutSidePorts = (list, edgeX) => {
      if (list.length === 0) return;
      if (list.length > 1) {
        list.sort((a, b) => (Math.abs(a._idealY - b._idealY) > 6 ? a._idealY - b._idealY : a._targetY - b._targetY));
      }
      for (let i = 0; i < list.length; i++) { list[i].x = edgeX; list[i].y = list[i]._idealY; }
      for (let i = 1; i < list.length; i++) {
        if (list[i].y < list[i - 1].y + minGap) list[i].y = list[i - 1].y + minGap;
      }
      if (list[list.length - 1].y > maxEdgeY) {
        list[list.length - 1].y = maxEdgeY;
        for (let i = list.length - 2; i >= 0; i--) {
          if (list[i].y > list[i + 1].y - minGap) list[i].y = Math.max(minEdgeY, list[i + 1].y - minGap);
        }
      }
    };

    layoutSidePorts(leftPorts, ch.x);
    layoutSidePorts(rightPorts, ch.x + ch.w);
    for (let i = 0; i < m; i++) ch.outPorts[i].idx = i;
  },

  parseStudioGraphPayload(rawText) {
    if (rawText === undefined) {
      const pre = document.querySelector("#studio-graph-data");
      rawText = pre ? pre.textContent || "" : "";
    }
    const gState = this.studioGraphState;
    gState.chapters = [];
    gState.chapterByKey.clear();
    gState.nodes = [];
    gState.nodeById.clear();
    gState.wires = [];
    gState.wirePins.clear();
    gState.collectorBars = [];
    gState.hoveredItem = null;
    gState.staticBatchDirty = true;
    gState.overlayBatchDirty = true;

    const vpInfo = { panX: 0, panY: 0, zoom: 1.0, hasView: false };
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
        if (colonIdx > 0) kv[seg.slice(0, colonIdx).trim()] = seg.slice(colonIdx + 1).trim();
      }

      if (kind === "viewport") {
        vpInfo.panX = parseFloat(kv.pan_x || "0") || 0;
        vpInfo.panY = parseFloat(kv.pan_y || "0") || 0;
        vpInfo.zoom = parseFloat(kv.zoom || "1") || 1.0;
        vpInfo.hasView = kv.has_view === "1";
        gState.savedPanX = vpInfo.panX;
        gState.savedPanY = vpInfo.panY;
        gState.savedZoom = vpInfo.zoom;
        gState.hasSavedView = vpInfo.hasView;
      } else if (kind === "chapter") {
        const outsRaw = (kv.outs || "").trim();
        const outRefs = outsRaw ? outsRaw.split(",").map((s) => s.trim()).filter(Boolean) : [];
        const outPorts = outRefs.length > 0
          ? outRefs.map((ref, idx) => ({ idx: idx, target: ref, targetLower: ref.toLowerCase(), x: 0, y: 0 }))
          : [{ idx: 0, target: "", targetLower: "", x: 0, y: 0 }];
        const chName = kv.name || kv.ref || AppShellController.tr("default_chapter");
        const chRef = kv.ref || chName;
        const ch = {
          id: "ch:" + (chRef || chName).toLowerCase(), rawId: kv.id || chRef, ref: chRef, refLower: chRef.toLowerCase(),
          name: chName, nameLower: chName.toLowerCase(), isRoot: kv.root === "1", startSceneId: kv.start || "",
          x: parseInt(kv.x || "0", 10) || 0, y: parseInt(kv.y || "0", 10) || 0,
          w: parseInt(kv.w || "240", 10) || 240, h: parseInt(kv.h || "140", 10) || 140,
          outPorts: outPorts, nodes: [], hidden: false
        };
        gState.chapters.push(ch);
        if (ch.nameLower) gState.chapterByKey.set(ch.nameLower, ch);
        if (ch.refLower) gState.chapterByKey.set(ch.refLower, ch);
      } else if (kind === "node") {
        const id = kv.id || "";
        if (!id) continue;
        const chName = kv.chapter || AppShellController.tr("default_chapter");
        const impTarget = kv.imp_target || "", impChapter = kv.imp_chapter || "";
        const node = {
          id: id, name: kv.name || id, chapter: chName, chapterLower: chName.toLowerCase(),
          x: parseInt(kv.x || "0", 10) || 0, y: parseInt(kv.y || "0", 10) || 0,
          selected: kv.selected === "1", isStart: kv.start === "1", isEntry: kv.entry === "1",
          unreachable: kv.unreachable === "1", hasChoices: kv.has_choices === "1",
          implicitTarget: impTarget, implicitChapter: impChapter, impTarget: impTarget, impChapter: impChapter,
          branchRole: kv.branch_role || "", branchParentId: kv.branch_parent || "", ports: []
        };
        this.recomputeNodeMetrics(node);
        gState.nodes.push(node);
        gState.nodeById.set(id, node);
        if (node.selected && !this.editorSelectionCleared && this.selectedNodeIds.size === 0) {
          this.selectedNodeIds.add(id);
        }
      } else if (kind === "port") {
        const node = gState.nodeById.get(kv.scene || "");
        if (!node) continue;
        const idx = parseInt(kv.idx || String(node.ports.length), 10) || 0;
        const oy = this.choiceRowY(idx) + this.CARD.choiceH * 0.5;
        node.ports.push({
          idx: idx, label: kv.label || AppShellController.tr("choice_prefix") + (idx + 1),
          targetId: kv.target_id || "", targetChapter: kv.target_chapter || "",
          isConditional: kv.cond === "1", isBottom: false, ox: this.CARD.w, oy: oy,
          x: node.x + this.CARD.w, y: node.y + oy
        });
        node.hasChoices = node.ports.length >= 2;
        this.recomputeNodeMetrics(node);
      } else if (kind === "wire_pins") {
        const wKey = (kv.key || "").trim().toLowerCase();
        const pinsRaw = (kv.pins || "").trim();
        if (wKey && pinsRaw) {
          const pins = [];
          const segs = pinsRaw.split(";");
          for (let p = 0; p < segs.length; p++) {
            const xy = segs[p].split(",");
            if (xy.length === 2) {
              const px = parseInt(xy[0].trim(), 10), py = parseInt(xy[1].trim(), 10);
              if (!isNaN(px) && !isNaN(py)) pins.push({ x: px, y: py });
            }
          }
          if (pins.length > 0) gState.wirePins.set(wKey, pins);
        }
      }
    }
    CanvasLayout.updateChapterGroupFrames();
    return vpInfo;
  },

  moveNodePortInMemory(nd, fromIdx, toIdx) {
    if (!nd || !nd.ports || fromIdx === toIdx) return false;
    if (fromIdx < 0 || fromIdx >= nd.ports.length || toIdx < 0 || toIdx >= nd.ports.length) return false;
    const movedPort = nd.ports.splice(fromIdx, 1)[0];
    nd.ports.splice(toIdx, 0, movedPort);
    for (let k = 0; k < nd.ports.length; k++) nd.ports[k].idx = k;
    this.recomputeNodeMetrics(nd);
    const ch = nd.chapterObj || this.studioGraphState.chapterByKey.get(nd.chapterLower);
    if (ch) this.recomputeChapterPortPositions(ch);
    EditorWires.route();
    this.studioGraphState.staticBatchDirty = true;
    this.studioGraphState.overlayBatchDirty = true;
    EditorWebglScene.render();
    return true;
  },

  computePointArcLengthAlongPolyline(pts, px, py) {
    if (!pts || pts.length < 2) return 0;
    let bestDistSq = Infinity, bestArc = 0, accum = 0;
    for (let i = 0; i < pts.length - 1; i++) {
      const ax = pts[i].x, ay = pts[i].y, bx = pts[i + 1].x, by = pts[i + 1].y;
      const dx = bx - ax, dy = by - ay, lenSq = dx * dx + dy * dy, segLen = Math.sqrt(lenSq);
      const t = lenSq > 0.001 ? Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / lenSq)) : 0;
      const dSq = (px - (ax + t * dx)) ** 2 + (py - (ay + t * dy)) ** 2;
      if (dSq < bestDistSq) { bestDistSq = dSq; bestArc = accum + t * segLen; }
      accum += segLen;
    }
    return bestArc;
  },

  rerouteAfterWirePinMutation(wire) {
    if (!wire) return;
    if (wire.wireKey) {
      if (wire.pins && wire.pins.length > 0) {
        this.studioGraphState.wirePins.set(wire.wireKey, wire.pins.map((p) => ({ x: p.x, y: p.y, axis: p.axis, segIdx: p.segIdx })));
      } else {
        this.studioGraphState.wirePins.delete(wire.wireKey);
      }
    }
    EditorWires.routeSingleWireBasePts(wire, this.studioGraphState.nodes, this.studioGraphState.chapters);
    EditorWires.applyParallelOffsetsAndBundles(this.studioGraphState.wires);
    this.studioGraphState.staticBatchDirty = true;
    this.studioGraphState.overlayBatchDirty = true;
    EditorWebglScene.render();
  }
};
Dispatcher.register("CanvasModel", CanvasModel);
