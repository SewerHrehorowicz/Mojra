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

function recomputeChapterInputPortPosition(ch) {
  ch.left = ch.x;
  ch.right = ch.x + ch.w;
  ch.top = ch.y;
  ch.bottom = ch.y + ch.h;
  ch.inY = ch.y;
  let desiredInX = ch.x + ch.w * 0.5;
  const chNodes = ch.nodes || studioGraphState.nodes;
  if (chNodes && chNodes.length > 0) {
    let entryNode = ch.startSceneId ? studioGraphState.nodeById.get(ch.startSceneId) : null;
    if (
      !entryNode ||
      (entryNode.chapterLower !== ch.nameLower && entryNode.chapterLower !== ch.refLower)
    ) {
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
    if (entryNode) {
      desiredInX = entryNode.x + (entryNode.inOx || NODE_CARD_WIDTH * 0.5);
    }
  }
  const minEdgeX = ch.x + 24;
  const maxEdgeX = Math.max(minEdgeX, ch.x + ch.w - 24);
  ch.inX = Math.max(minEdgeX, Math.min(maxEdgeX, desiredInX));
}

function recomputeChapterPortPositions(ch) {
  recomputeChapterInputPortPosition(ch);
  const minEdgeX = ch.x + 24;
  const maxEdgeX = Math.max(minEdgeX, ch.x + ch.w - 16);
  const minEdgeY = ch.y + 48;
  const maxEdgeY = Math.max(minEdgeY, ch.y + ch.h - 24);

  if (!ch.outPorts || ch.outPorts.length === 0) {
    ch.outPorts = [
      {
        idx: 0,
        target: "",
        targetLower: "",
        connected: false,
        side: "bottom",
        x: ch.x + ch.w * 0.5,
        y: ch.y + ch.h
      }
    ];
  }

  let chNodes = ch.nodes;
  if (!chNodes) {
    const nodes = studioGraphState.nodes || [];
    chNodes = [];
    for (let i = 0; i < nodes.length; i++) {
      const nd = nodes[i];
      if (nd.chapterLower === ch.nameLower || nd.chapterLower === ch.refLower) {
        chNodes.push(nd);
      }
    }
    ch.nodes = chNodes;
  }

  const bottomPorts = [];
  const leftPorts = [];
  const rightPorts = [];

  const m = ch.outPorts.length;
  for (let i = 0; i < m; i++) {
    const op = ch.outPorts[i];
    op.connected = Boolean(op.targetLower);
    const toCh = op.targetLower ? studioGraphState.chapterByKey.get(op.targetLower) : null;
    const targetX = toCh ? (toCh.inX || toCh.x + toCh.w * 0.5) : ch.x + ch.w * 0.5;
    const targetY = toCh ? (toCh.inY || toCh.y) : ch.y + ch.h + 120;

    let sumX = 0;
    let sumY = 0;
    let maxSrcBottom = ch.y + 96;
    let srcCount = 0;
    let lastSrcNode = null;
    let hasBlockedDropToBottom = false;

    if (op.targetLower && chNodes.length > 0) {
      for (let j = 0; j < chNodes.length; j++) {
        const nd = chNodes[j];
        const portList =
          nd.hasChoices && nd.ports && nd.ports.length > 0
            ? nd.ports
            : nd.implicitPort
              ? [nd.implicitPort]
              : [];
        for (let k = 0; k < portList.length; k++) {
          const pt = portList[k];
          const tLower = (pt.targetChapter || "").trim().toLowerCase();
          if (!tLower) continue;
          if (
            tLower === op.targetLower ||
            (toCh && (tLower === toCh.nameLower || tLower === toCh.refLower))
          ) {
            const sy = nd.y + pt.oy;
            let candX;
            if (pt.isBottom) {
              candX = nd.x + (nd.inOx || NODE_CARD_WIDTH * 0.5);
              if (typeof linecastVertical === "function") {
                if (linecastVertical(candX, sy + 6, ch.y + ch.h, chNodes, nd, null, 10)) {
                  hasBlockedDropToBottom = true;
                }
              }
              if (typeof resolveClearX === "function") {
                candX = resolveClearX(candX, sy + 4, ch.y + ch.h, chNodes, nd, null, 0, 14);
              }
            } else {
              const stubRank = Math.max(0, portList.length - 1 - k);
              const rightLaneX = nd.x + pt.ox + 16 + stubRank * 14;
              const nodeCenterX = nd.x + (nd.inOx || NODE_CARD_WIDTH * 0.5);
              if (toCh && toCh.inX <= nd.x + nd.w && portList.length > 1) {
                const desiredUnderNodeX = Math.max(nd.x + 24, Math.min(nd.x + nd.w - 24, toCh.inX || nodeCenterX));
                if (
                  typeof linecastVertical !== "function" ||
                  !linecastVertical(desiredUnderNodeX, nd.y + nd.h + 4, ch.y + ch.h, chNodes, nd, null, 10)
                ) {
                  candX = desiredUnderNodeX;
                } else {
                  candX = rightLaneX;
                }
              } else {
                candX = rightLaneX;
              }
              if (typeof linecastVertical === "function") {
                if (linecastVertical(candX, (candX <= nd.x + nd.w ? nd.y + nd.h : sy) + 6, ch.y + ch.h, chNodes, nd, null, 10)) {
                  hasBlockedDropToBottom = true;
                }
              }
              if (typeof resolveClearX === "function") {
                candX = resolveClearX(
                  candX,
                  candX <= nd.x + nd.w ? nd.y + nd.h + 4 : sy,
                  ch.y + ch.h,
                  chNodes,
                  nd,
                  null,
                  1,
                  14
                );
              }
            }
            sumX += candX;
            sumY += sy;
            if (nd.bottom > maxSrcBottom) {
              maxSrcBottom = nd.bottom;
            }
            lastSrcNode = nd;
            srcCount++;
          }
        }
      }
    }

    let side = "bottom";
    if (toCh && !toCh.hidden) {
      const isLeftAdjacent = toCh.right <= ch.left + 24;
      const isRightAdjacent = toCh.left >= ch.right - 24;
      const isSideVerticalBand = toCh.top < ch.bottom - 24;
      if (isLeftAdjacent && (isSideVerticalBand || (hasBlockedDropToBottom && toCh.top <= ch.bottom + 24))) {
        side = "left";
      } else if (isRightAdjacent && (isSideVerticalBand || (hasBlockedDropToBottom && toCh.top <= ch.bottom + 24))) {
        side = "right";
      }
    }
    op.side = side;

    if (side === "left" || side === "right") {
      op.x = side === "left" ? ch.x : ch.x + ch.w;
      let idealY = srcCount > 0 ? maxSrcBottom + 24 : (ch.y + ch.h) * 0.5;
      if (side === "right" && srcCount === 1 && lastSrcNode && !lastSrcNode.implicitPort) {
        const directY = sumY / srcCount;
        if (
          typeof linecastHorizontal !== "function" ||
          !linecastHorizontal(directY, lastSrcNode.right + 4, ch.right, chNodes, lastSrcNode, null, 10)
        ) {
          idealY = directY;
        }
      }
      if (typeof resolveClearY === "function") {
        const spanX1 = side === "left" ? ch.left : (lastSrcNode ? lastSrcNode.right + 4 : ch.left + ch.w * 0.5);
        const spanX2 = side === "left" ? (lastSrcNode ? lastSrcNode.right + 20 : ch.left + ch.w * 0.5) : ch.right;
        idealY = resolveClearY(idealY, spanX1, spanX2, chNodes, lastSrcNode, null, 1, 12);
      }
      op._idealY = Math.max(minEdgeY, Math.min(maxEdgeY, Math.round(idealY)));
      op._targetY = targetY;
      if (side === "left") {
        leftPorts.push(op);
      } else {
        rightPorts.push(op);
      }
    } else {
      let idealX = ch.x + ch.w * 0.5;
      if (srcCount > 0) {
        idealX = sumX / srcCount;
      } else if (op.targetLower && toCh) {
        idealX = targetX;
      } else if (chNodes.length === 1) {
        idealX = chNodes[0].x + (chNodes[0].inOx || NODE_CARD_WIDTH * 0.5);
      }
      op.y = ch.y + ch.h;
      op._idealX = Math.max(minEdgeX, Math.min(maxEdgeX, idealX));
      op._targetX = targetX;
      bottomPorts.push(op);
    }
  }

  const minGap = 24;
  const minBottomPortGap = 24;
  if (bottomPorts.length > 1) {
    const sortedIdealSlots = bottomPorts.map((p) => p._idealX).sort((a, b) => a - b);
    bottomPorts.sort((a, b) => {
      if (Math.abs(a._targetX - b._targetX) > 12) return a._targetX - b._targetX;
      if (Math.abs(a._idealX - b._idealX) > 6) return a._idealX - b._idealX;
      return (a.idx || 0) - (b.idx || 0);
    });
    for (let i = 0; i < bottomPorts.length; i++) {
      bottomPorts[i]._idealX = sortedIdealSlots[i];
    }
  }
  for (let i = 0; i < bottomPorts.length; i++) {
    bottomPorts[i].x = bottomPorts[i]._idealX;
    bottomPorts[i].y = ch.y + ch.h;
  }
  if (bottomPorts.length > 1) {
    let firstX = bottomPorts[0]._idealX;
    let lastX = bottomPorts[bottomPorts.length - 1]._idealX;
    if (Math.abs(lastX - firstX) <= 6) {
      const center = (firstX + lastX) * 0.5;
      const totalSpan = (bottomPorts.length - 1) * minGap;
      const startX = center - totalSpan * 0.5;
      for (let i = 0; i < bottomPorts.length; i++) {
        bottomPorts[i].x = Math.max(minEdgeX, Math.min(maxEdgeX, Math.round(startX + i * minGap)));
      }
    }
  }
  for (let i = 1; i < bottomPorts.length; i++) {
    if (bottomPorts[i].x < bottomPorts[i - 1].x + minBottomPortGap) {
      bottomPorts[i].x = bottomPorts[i - 1].x + minBottomPortGap;
    }
  }
  if (bottomPorts.length > 0 && bottomPorts[bottomPorts.length - 1].x > maxEdgeX) {
    bottomPorts[bottomPorts.length - 1].x = maxEdgeX;
    for (let i = bottomPorts.length - 2; i >= 0; i--) {
      if (bottomPorts[i].x > bottomPorts[i + 1].x - minBottomPortGap) {
        bottomPorts[i].x = Math.max(minEdgeX, bottomPorts[i + 1].x - minBottomPortGap);
      }
    }
  }

  const layoutSidePorts = (list, edgeX) => {
    if (list.length === 0) return;
    if (list.length > 1) {
      list.sort((a, b) => {
        if (Math.abs(a._idealY - b._idealY) > 6) return a._idealY - b._idealY;
        return a._targetY - b._targetY;
      });
    }
    for (let i = 0; i < list.length; i++) {
      list[i].x = edgeX;
      list[i].y = list[i]._idealY;
    }
    for (let i = 1; i < list.length; i++) {
      if (list[i].y < list[i - 1].y + minGap) {
        list[i].y = list[i - 1].y + minGap;
      }
    }
    if (list[list.length - 1].y > maxEdgeY) {
      list[list.length - 1].y = maxEdgeY;
      for (let i = list.length - 2; i >= 0; i--) {
        if (list[i].y > list[i + 1].y - minGap) {
          list[i].y = Math.max(minEdgeY, list[i + 1].y - minGap);
        }
      }
    }
  };

  layoutSidePorts(leftPorts, ch.x);
  layoutSidePorts(rightPorts, ch.x + ch.w);

  for (let i = 0; i < m; i++) {
    ch.outPorts[i].idx = i;
  }
}

function parseStudioGraphPayload(rawText) {
  studioGraphState.chapters = [];
  studioGraphState.chapterByKey.clear();
  studioGraphState.nodes = [];
  studioGraphState.nodeById.clear();
  studioGraphState.wires = [];
  studioGraphState.wirePins.clear();
  studioGraphState.bundleOverlays = [];
  studioGraphState.collectorBars = [];
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
        nodes: [],
        hidden: false
      };
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
        branchRole: kv.branch_role || "",
        branchParentId: kv.branch_parent || "",
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
    } else if (kind === "wire_pins") {
      const wKey = (kv.key || "").trim().toLowerCase();
      const pinsRaw = (kv.pins || "").trim();
      if (wKey && pinsRaw) {
        const pins = [];
        const segs = pinsRaw.split(";");
        for (let p = 0; p < segs.length; p++) {
          const xy = segs[p].split(",");
          if (xy.length === 2) {
            const px = parseInt(xy[0].trim(), 10);
            const py = parseInt(xy[1].trim(), 10);
            if (!isNaN(px) && !isNaN(py)) {
              pins.push({ x: px, y: py });
            }
          }
        }
        if (pins.length > 0) {
          studioGraphState.wirePins.set(wKey, pins);
        }
      }
    }
  }

  studioGraphState.topologyDirty = true;
  return vpInfo;
}

const SPATIAL_CELL_SHIFT = 8; // 256x256 px spatial hash cells

function buildSpatialGridForNodes(nodeList) {
  const grid = new Map();
  if (!nodeList) return grid;
  for (let i = 0; i < nodeList.length; i++) {
    insertNodeIntoSpatialGrid(grid, nodeList[i]);
  }
  nodeList._spatialGrid = grid;
  return grid;
}

function insertNodeIntoSpatialGrid(grid, nd) {
  if (!grid || !nd || nd.hidden) return;
  const gx0 = (nd.left - 48) >> SPATIAL_CELL_SHIFT;
  const gx1 = (nd.right + 48) >> SPATIAL_CELL_SHIFT;
  const gy0 = (nd.top - 48) >> SPATIAL_CELL_SHIFT;
  const gy1 = (nd.bottom + 48) >> SPATIAL_CELL_SHIFT;
  nd._gx0 = gx0;
  nd._gx1 = gx1;
  nd._gy0 = gy0;
  nd._gy1 = gy1;
  for (let gy = gy0; gy <= gy1; gy++) {
    const rowMask = (gy & 0xffff) << 16;
    for (let gx = gx0; gx <= gx1; gx++) {
      const key = rowMask | (gx & 0xffff);
      let cell = grid.get(key);
      if (!cell) {
        cell = [nd];
        grid.set(key, cell);
      } else {
        cell.push(nd);
      }
    }
  }
}

function removeNodeFromSpatialGrid(grid, nd) {
  if (!grid || !nd || nd._gx0 === undefined) return;
  for (let gy = nd._gy0; gy <= nd._gy1; gy++) {
    const rowMask = (gy & 0xffff) << 16;
    for (let gx = nd._gx0; gx <= nd._gx1; gx++) {
      const key = rowMask | (gx & 0xffff);
      const cell = grid.get(key);
      if (cell) {
        const idx = cell.indexOf(nd);
        if (idx >= 0) {
          cell[idx] = cell[cell.length - 1];
          cell.pop();
        }
      }
    }
  }
}

function updateNodeInSpatialGrids(nd) {
  if (studioGraphState.nodeSpatialGrid) {
    removeNodeFromSpatialGrid(studioGraphState.nodeSpatialGrid, nd);
    insertNodeIntoSpatialGrid(studioGraphState.nodeSpatialGrid, nd);
  }
  const ch = nd.chapterObj;
  if (ch && ch.nodes && ch.nodes._spatialGrid) {
    removeNodeFromSpatialGrid(ch.nodes._spatialGrid, nd);
    insertNodeIntoSpatialGrid(ch.nodes._spatialGrid, nd);
  }
}

function hasSpatialNodeOverlap(grid, nodesFallback, nx, ny, nw, nh) {
  const nr = nx + nw;
  const nb = ny + nh;
  if (grid && grid.size > 0) {
    const gx0 = (nx - 48) >> SPATIAL_CELL_SHIFT;
    const gx1 = (nr + 48) >> SPATIAL_CELL_SHIFT;
    const gy0 = (ny - 48) >> SPATIAL_CELL_SHIFT;
    const gy1 = (nb + 48) >> SPATIAL_CELL_SHIFT;
    for (let gy = gy0; gy <= gy1; gy++) {
      const rowMask = (gy & 0xffff) << 16;
      for (let gx = gx0; gx <= gx1; gx++) {
        const cell = grid.get(rowMask | (gx & 0xffff));
        if (!cell) continue;
        for (let i = 0; i < cell.length; i++) {
          const other = cell[i];
          if (other._isDragging) continue;
          if (
            nx < other.right + 48 &&
            nr + 48 > other.left &&
            ny < other.bottom + 48 &&
            nb + 48 > other.top
          ) {
            return true;
          }
        }
      }
    }
    return false;
  }
  for (let i = 0; i < nodesFallback.length; i++) {
    const other = nodesFallback[i];
    if (other._isDragging) continue;
    if (
      nx < other.right + 48 &&
      nr + 48 > other.left &&
      ny < other.bottom + 48 &&
      nb + 48 > other.top
    ) {
      return true;
    }
  }
  return false;
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

function resolveNonOverlappingNodeCoords(baseX, baseY, chapterName, nodeW, nodeH) {
  const w = nodeW || NODE_CARD_WIDTH;
  const h = nodeH || NODE_BASE_HEIGHT;
  const nodes = studioGraphState.nodes || [];
  const chapters = studioGraphState.chapters || [];
  const chKey = (chapterName || "").trim().toLowerCase();
  let targetCh = null;
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    if (!ch.hidden && (ch.nameLower === chKey || ch.refLower === chKey)) {
      targetCh = ch;
      break;
    }
  }

  const collidesAt = (cx, cy, checkChapterUnion) => {
    for (let i = 0; i < nodes.length; i++) {
      const other = nodes[i];
      if (
        cx < other.right + 48 &&
        cx + w + 48 > other.left &&
        cy < other.bottom + 48 &&
        cy + h + 48 > other.top
      ) {
        return true;
      }
    }
    let candLeft = cx - 36;
    let candTop = cy - 52;
    let candRight = cx + w + 36;
    let candBottom = cy + Math.max(h, 120) + 44;
    if (checkChapterUnion && targetCh) {
      if (targetCh.left < candLeft) candLeft = targetCh.left;
      if (targetCh.top < candTop) candTop = targetCh.top;
      if (targetCh.right > candRight) candRight = targetCh.right;
      if (targetCh.bottom > candBottom) candBottom = targetCh.bottom;
    }
    for (let i = 0; i < chapters.length; i++) {
      const ch = chapters[i];
      if (ch.hidden || ch === targetCh || ch.nameLower === chKey || ch.refLower === chKey) continue;
      if (
        candLeft < ch.right + 24 &&
        candRight + 24 > ch.left &&
        candTop < ch.bottom + 24 &&
        candBottom + 24 > ch.top
      ) {
        return true;
      }
    }
    return false;
  };

  if (!collidesAt(baseX, baseY, true)) {
    return { x: baseX, y: baseY };
  }
  const unionPasses = [true, false];
  for (let p = 0; p < unionPasses.length; p++) {
    const checkUnion = unionPasses[p];
    for (let step = 1; step <= 48; step++) {
      const stepPx = step * EDITOR_GRID_SIZE;
      const candidates = [
        [baseX, baseY + stepPx],
        [baseX + stepPx, baseY],
        [baseX - stepPx, baseY],
        [baseX, baseY - stepPx],
        [baseX + stepPx, baseY + stepPx],
        [baseX - stepPx, baseY + stepPx]
      ];
      for (let c = 0; c < candidates.length; c++) {
        const cx = snapToEditorGrid(candidates[c][0]);
        const cy = snapToEditorGrid(candidates[c][1]);
        if (!collidesAt(cx, cy, checkUnion)) {
          return { x: cx, y: cy };
        }
      }
    }
  }
  return { x: baseX, y: baseY };
}

function resolveValidChapterDragDelta(ch, items, desiredDx, desiredDy, lastValidDx, lastValidDy) {
  const chapters = studioGraphState.chapters || [];
  if (chapters.length <= 1 || !items || items.length === 0) {
    return { dx: desiredDx, dy: desiredDy };
  }
  let baseMinX = Infinity;
  let baseMinY = Infinity;
  let baseMaxX = -Infinity;
  let baseMaxY = -Infinity;
  for (let i = 0; i < items.length; i++) {
    const it = items[i];
    const nd = it.node;
    const nh = Math.max(nd.h || 74, 120);
    if (it.startX < baseMinX) baseMinX = it.startX;
    if (it.startY < baseMinY) baseMinY = it.startY;
    if (it.startX + nd.w > baseMaxX) baseMaxX = it.startX + nd.w;
    if (it.startY + nh > baseMaxY) baseMaxY = it.startY + nh;
  }
  const baseLeft = baseMinX - 36;
  const baseTop = baseMinY - 52;
  const baseRight = baseMaxX + 36;
  const baseBottom = baseMaxY + 44;

  const isDeltaValid = (dx, dy) => {
    const l = baseLeft + dx;
    const t = baseTop + dy;
    const r = baseRight + dx;
    const b = baseBottom + dy;
    for (let i = 0; i < chapters.length; i++) {
      const other = chapters[i];
      if (other === ch || other.hidden) continue;
      if (
        l < other.right + 24 &&
        r + 24 > other.left &&
        t < other.bottom + 24 &&
        b + 24 > other.top
      ) {
        return false;
      }
    }
    return true;
  };

  if (isDeltaValid(desiredDx, desiredDy)) {
    return { dx: desiredDx, dy: desiredDy };
  }

  let bestDx = lastValidDx;
  let bestDy = lastValidDy;
  let bestDistSq = Infinity;

  const tryCandidate = (cx, cy) => {
    if (!isDeltaValid(cx, cy)) return;
    const ddx = cx - desiredDx;
    const ddy = cy - desiredDy;
    const dSq = ddx * ddx + ddy * ddy;
    if (dSq < bestDistSq) {
      bestDistSq = dSq;
      bestDx = cx;
      bestDy = cy;
    }
  };

  tryCandidate(desiredDx, lastValidDy);
  tryCandidate(lastValidDx, desiredDy);

  const stepX = desiredDx >= lastValidDx ? -EDITOR_GRID_SIZE : EDITOR_GRID_SIZE;
  for (let cx = desiredDx; cx !== lastValidDx; cx += stepX) {
    tryCandidate(cx, desiredDy);
    tryCandidate(cx, lastValidDy);
    if (Math.abs(cx - desiredDx) > 960) break;
  }
  const stepY = desiredDy >= lastValidDy ? -EDITOR_GRID_SIZE : EDITOR_GRID_SIZE;
  for (let cy = desiredDy; cy !== lastValidDy; cy += stepY) {
    tryCandidate(desiredDx, cy);
    tryCandidate(lastValidDx, cy);
    if (Math.abs(cy - desiredDy) > 960) break;
  }

  return { dx: bestDx, dy: bestDy };
}

function resolveValidNodeDragDelta(items, desiredDx, desiredDy, lastValidDx, lastValidDy) {
  const nodes = studioGraphState.nodes || [];
  const chapters = studioGraphState.chapters || [];
  if (!items || items.length === 0) {
    return { dx: desiredDx, dy: desiredDy };
  }

  for (let i = 0; i < items.length; i++) {
    items[i].node._isDragging = true;
  }

  // Precompute stationary chapter bounds once so candidate testing is O(items.length), not O(chNodes.length)
  const affectedChapters = [];
  for (let i = 0; i < items.length; i++) {
    const ch = items[i].node.chapterObj;
    if (ch && affectedChapters.indexOf(ch) === -1) {
      let sMinX = Infinity;
      let sMinY = Infinity;
      let sMaxX = -Infinity;
      let sMaxY = -Infinity;
      let iMinX = Infinity;
      let iMinY = Infinity;
      let iMaxX = -Infinity;
      let iMaxY = -Infinity;
      const chNodes = ch.nodes || [];
      for (let j = 0; j < chNodes.length; j++) {
        const nd = chNodes[j];
        const nh = Math.max(nd.h || 74, 120);
        if (nd.x < iMinX) iMinX = nd.x;
        if (nd.y < iMinY) iMinY = nd.y;
        if (nd.x + nd.w > iMaxX) iMaxX = nd.x + nd.w;
        if (nd.y + nh > iMaxY) iMaxY = nd.y + nh;
        if (nd._isDragging) continue;
        if (nd.x < sMinX) sMinX = nd.x;
        if (nd.y < sMinY) sMinY = nd.y;
        if (nd.x + nd.w > sMaxX) sMaxX = nd.x + nd.w;
        if (nd.y + nh > sMaxY) sMaxY = nd.y + nh;
      }
      ch._statMinX = sMinX;
      ch._statMinY = sMinY;
      ch._statMaxX = sMaxX;
      ch._statMaxY = sMaxY;
      ch._initLeft = iMinX - 36;
      ch._initTop = iMinY - 52;
      ch._initRight = iMaxX + 36;
      ch._initBottom = iMaxY + 44;
      affectedChapters.push(ch);
    }
  }

  const globalGrid = studioGraphState.nodeSpatialGrid;

  const isDeltaValid = (dx, dy) => {
    for (let i = 0; i < items.length; i++) {
      const it = items[i];
      const nd = it.node;
      const nx = snapToEditorGrid(it.startX + dx);
      const ny = snapToEditorGrid(it.startY + dy);
      if (hasSpatialNodeOverlap(globalGrid, nodes, nx, ny, nd.w, nd.h)) {
        return false;
      }
    }

    for (let c = 0; c < affectedChapters.length; c++) {
      const ch = affectedChapters[c];
      let minX = ch._statMinX;
      let minY = ch._statMinY;
      let maxX = ch._statMaxX;
      let maxY = ch._statMaxY;
      for (let k = 0; k < items.length; k++) {
        const it = items[k];
        const nd = it.node;
        if (nd.chapterObj !== ch) continue;
        const nx = snapToEditorGrid(it.startX + dx);
        const ny = snapToEditorGrid(it.startY + dy);
        const nh = Math.max(nd.h || 74, 120);
        if (nx < minX) minX = nx;
        if (ny < minY) minY = ny;
        if (nx + nd.w > maxX) maxX = nx + nd.w;
        if (ny + nh > maxY) maxY = ny + nh;
      }
      const cLeft = minX - 36;
      const cTop = minY - 52;
      const cRight = maxX + 36;
      const cBottom = maxY + 44;

      for (let o = 0; o < chapters.length; o++) {
        const otherCh = chapters[o];
        if (otherCh === ch || otherCh.hidden) continue;
        if (
          cLeft < otherCh.right + 24 &&
          cRight + 24 > otherCh.left &&
          cTop < otherCh.bottom + 24 &&
          cBottom + 24 > otherCh.top
        ) {
          const wasOverlapping =
            ch._initLeft < otherCh.right + 24 &&
            ch._initRight + 24 > otherCh.left &&
            ch._initTop < otherCh.bottom + 24 &&
            ch._initBottom + 24 > otherCh.top;
          if (!wasOverlapping) {
            return false;
          }
          if (
            cLeft < ch._initLeft ||
            cRight > ch._initRight ||
            cTop < ch._initTop ||
            cBottom > ch._initBottom
          ) {
            return false;
          }
        }
      }
    }
    return true;
  };

  let resDx = desiredDx;
  let resDy = desiredDy;
  if (!isDeltaValid(desiredDx, desiredDy)) {
    resDx = lastValidDx;
    resDy = lastValidDy;
    let bestDistSq = Infinity;

    const tryCandidate = (cx, cy) => {
      if (!isDeltaValid(cx, cy)) return;
      const ddx = cx - desiredDx;
      const ddy = cy - desiredDy;
      const dSq = ddx * ddx + ddy * ddy;
      if (dSq < bestDistSq) {
        bestDistSq = dSq;
        resDx = cx;
        resDy = cy;
      }
    };

    tryCandidate(desiredDx, lastValidDy);
    tryCandidate(lastValidDx, desiredDy);

    const stepX = desiredDx >= lastValidDx ? -EDITOR_GRID_SIZE : EDITOR_GRID_SIZE;
    for (let cx = desiredDx; cx !== lastValidDx; cx += stepX) {
      tryCandidate(cx, desiredDy);
      tryCandidate(cx, lastValidDy);
      if (Math.abs(cx - desiredDx) > 720) break;
    }
    const stepY = desiredDy >= lastValidDy ? -EDITOR_GRID_SIZE : EDITOR_GRID_SIZE;
    for (let cy = desiredDy; cy !== lastValidDy; cy += stepY) {
      tryCandidate(desiredDx, cy);
      tryCandidate(lastValidDx, cy);
      if (Math.abs(cy - desiredDy) > 720) break;
    }
  }

  for (let i = 0; i < items.length; i++) {
    items[i].node._isDragging = false;
  }
  return { dx: resDx, dy: resDy };
}

function createSceneNodeAtWorldCoords(worldX, worldY, chapterName) {
  if (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview()) {
    return;
  }
  closeCanvasContextMenu();
  hideFloatingChapterInput(false);
  hideFloatingNodeTitleInput(false);
  hideFloatingChoiceInput(false);
  editorSelectionCleared = false;
  selectedNodeIds.clear();
  const baseX = snapToEditorGrid(worldX);
  const baseY = snapToEditorGrid(worldY);
  const resolvedChapter = chapterName || findClosestChapterAtWorldCoords(baseX, baseY);
  const cleanCoords = resolveNonOverlappingNodeCoords(baseX, baseY, resolvedChapter, NODE_CARD_WIDTH, NODE_BASE_HEIGHT);
  const params = {
    x: cleanCoords.x,
    y: cleanCoords.y
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

function saveWirePinsOnServer(wireKey, pins) {
  if (!wireKey) return;
  const cleanKey = wireKey.trim().toLowerCase();
  const encodedPins =
    pins && pins.length > 0
      ? pins.map((p) => Math.round(p.x) + "," + Math.round(p.y)).join(";")
      : "";
  postServerActionSilent("SaveEditorState", {
    wire_key: cleanKey,
    wire_pins: encodedPins,
    pan_x: Math.round(editorCanvasState.panX),
    pan_y: Math.round(editorCanvasState.panY),
    zoom: editorCanvasState.zoom.toFixed(2)
  });
}

function positionFloatingChapterInput() {
  const inp = qs("#canvas-chapter-input");
  const ch = studioGraphState.editingChapter;
  if (!inp || !ch || inp.classList.contains("hidden")) return;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;
  const left = (ch.x + 28) * z + px;
  const top = (ch.y + 4.703) * z + 0.5 + py;
  const width = Math.max(24, (ch.w - 38) * z);
  const height = Math.max(4, 20 * z);
  inp.style.left = left.toFixed(2) + "px";
  inp.style.top = top.toFixed(2) + "px";
  inp.style.width = width.toFixed(2) + "px";
  inp.style.height = height.toFixed(2) + "px";
  inp.style.fontSize = (11 * z).toFixed(2) + "px";
  inp.style.letterSpacing = (0.65 * z).toFixed(2) + "px";
}

function openFloatingChapterInput(ch) {
  const inp = qs("#canvas-chapter-input");
  if (!inp || !ch) return;
  hideFloatingNodeTitleInput(true);
  hideFloatingChoiceInput(true);
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

function positionFloatingNodeTitleInput() {
  const inp = qs("#canvas-node-title-input");
  const nd = studioGraphState.editingNodeTitle;
  if (!inp || !nd || inp.classList.contains("hidden")) return;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;
  const radioReserve = nd.branchRole === "primary" || nd.branchRole === "secondary" ? 16 : 0;
  const badgeReserve = (nd.isStart || nd.isEntry ? 50 : 8) + radioReserve;
  const left = (nd.x + 28) * z + px;
  const top = (nd.y + 6.715) * z + 0.5 + py;
  const width = Math.max(24, (nd.w - 28 - badgeReserve) * z);
  const height = Math.max(4, 22 * z);
  inp.style.left = left.toFixed(2) + "px";
  inp.style.top = top.toFixed(2) + "px";
  inp.style.width = width.toFixed(2) + "px";
  inp.style.height = height.toFixed(2) + "px";
  inp.style.fontSize = (13 * z).toFixed(2) + "px";
}

function commitNodeTitleInput(inp) {
  const nd = studioGraphState.editingNodeTitle;
  if (!inp || !nd) return;
  const newName = (inp.value || "").trim();
  const oldName = (nd.name || nd.id || "").trim();
  if (!newName || newName === oldName) return;
  nd.name = newName;
  studioGraphState.staticBatchDirty = true;
  renderStudioCanvas();
  postServerActionSilent("AutoSaveScene", {
    scene_id: nd.id,
    name: newName
  });
}

function openFloatingNodeTitleInput(nd) {
  const inp = qs("#canvas-node-title-input");
  if (!inp || !nd) return;
  hideFloatingChapterInput(true);
  hideFloatingChoiceInput(true);
  studioGraphState.editingNodeTitle = nd;
  studioGraphState.staticBatchDirty = true;
  inp.value = nd.name || nd.id || "";
  inp.setAttribute("data-scene-id", nd.id);
  inp.classList.remove("hidden");
  positionFloatingNodeTitleInput();
  inp.focus();
  inp.select();
  renderStudioCanvas();
}

function hideFloatingNodeTitleInput(shouldCommit) {
  const inp = qs("#canvas-node-title-input");
  if (!inp || inp.classList.contains("hidden")) return;
  if (shouldCommit) {
    commitNodeTitleInput(inp);
  }
  studioGraphState.editingNodeTitle = null;
  studioGraphState.staticBatchDirty = true;
  inp.classList.add("hidden");
  renderStudioCanvas();
}

function positionFloatingChoiceInput() {
  const inp = qs("#canvas-choice-input");
  const ec = studioGraphState.editingChoice;
  if (!inp || !ec || !ec.node || inp.classList.contains("hidden")) return;
  const nd = ec.node;
  const idx = ec.idx;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;
  const rowY = nd.y + NODE_HEADER_HEIGHT + idx * NODE_CHOICE_ROW_HEIGHT;
  const left = (nd.x + 21) * z + px;
  const top = (rowY + 2.703) * z + 0.5 + py;
  const width = Math.max(24, (nd.w - 47) * z);
  const height = Math.max(4, (NODE_CHOICE_ROW_HEIGHT - 6) * z);
  inp.style.left = left.toFixed(2) + "px";
  inp.style.top = top.toFixed(2) + "px";
  inp.style.width = width.toFixed(2) + "px";
  inp.style.height = height.toFixed(2) + "px";
  inp.style.fontSize = (11 * z).toFixed(2) + "px";
}

function commitChoiceInput(inp) {
  const ec = studioGraphState.editingChoice;
  if (!inp || !ec || !ec.node) return;
  const nd = ec.node;
  const idx = ec.idx;
  const pt = nd.ports && nd.ports[idx];
  if (!pt) return;
  const newLabel = (inp.value || "").trim() || "Wybór " + (idx + 1);
  if (newLabel === (pt.label || "")) return;
  pt.label = newLabel;
  studioGraphState.staticBatchDirty = true;
  renderStudioCanvas();
  const payload = { scene_id: nd.id };
  payload["choice_label_" + idx] = newLabel;
  postServerActionSilent("AutoSaveScene", payload);
}

function openFloatingChoiceInput(nd, choiceIdx) {
  const inp = qs("#canvas-choice-input");
  if (!inp || !nd || !nd.ports || !nd.ports[choiceIdx]) return;
  hideFloatingChapterInput(true);
  hideFloatingNodeTitleInput(true);
  studioGraphState.editingChoice = { node: nd, idx: choiceIdx };
  studioGraphState.staticBatchDirty = true;
  inp.value = nd.ports[choiceIdx].label || "Wybór " + (choiceIdx + 1);
  inp.setAttribute("data-scene-id", nd.id);
  inp.setAttribute("data-choice-idx", String(choiceIdx));
  inp.classList.remove("hidden");
  positionFloatingChoiceInput();
  inp.focus();
  inp.select();
  renderStudioCanvas();
}

function hideFloatingChoiceInput(shouldCommit) {
  const inp = qs("#canvas-choice-input");
  if (!inp || inp.classList.contains("hidden")) return;
  if (shouldCommit) {
    commitChoiceInput(inp);
  }
  studioGraphState.editingChoice = null;
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
  positionFloatingNodeTitleInput();
  positionFloatingChoiceInput();
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

function updateChapterGroupFrames(onlyChaptersSet, hasInterChapterPortMove) {
  const nodes = studioGraphState.nodes;
  const chapters = studioGraphState.chapters;
  if (!nodes || nodes.length === 0) return null;

  if (onlyChaptersSet && onlyChaptersSet.size > 0) {
    const changedChapters = new Set();
    onlyChaptersSet.forEach((ch) => {
      const matching = ch.nodes;
      if (!matching || matching.length === 0) return;
      let minX = Infinity;
      let minY = Infinity;
      let maxX = -Infinity;
      let maxY = -Infinity;
      for (let j = 0; j < matching.length; j++) {
        const nd = matching[j];
        const h = Math.max(nd.h || 74, 120);
        const laneSpread = nd.hasChoices && nd.ports && nd.ports.length > 0 ? 10 + nd.ports.length * 14 : 0;
        const rightEdge = nd.x + nd.w + laneSpread;
        if (nd.x < minX) minX = nd.x;
        if (nd.y < minY) minY = nd.y;
        if (rightEdge > maxX) maxX = rightEdge;
        if (nd.y + h > maxY) maxY = nd.y + h;
      }
      const nextX = minX - 36;
      const nextY = minY - 52;
      const nextW = maxX - minX + 72;
      const nextH = maxY - minY + 96;
      const frameChanged =
        ch.x !== nextX || ch.y !== nextY || ch.w !== nextW || ch.h !== nextH;
      const prevInX = ch.inX;
      const prevInY = ch.inY;
      ch.x = nextX;
      ch.y = nextY;
      ch.w = nextW;
      ch.h = nextH;
      recomputeChapterInputPortPosition(ch);
      const inPortChanged = ch.inX !== prevInX || ch.inY !== prevInY;
      if (frameChanged || inPortChanged || hasInterChapterPortMove) {
        recomputeChapterPortPositions(ch);
        changedChapters.add(ch);
      }
    });
    studioGraphState.staticBatchDirty = true;
    return changedChapters;
  }

  studioGraphState.nodeSpatialGrid = buildSpatialGridForNodes(nodes);

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
    ch.nodes = matching;
    buildSpatialGridForNodes(matching);
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
      nd.chapterObj = ch;
      const h = Math.max(nd.h || 74, 120);
      const laneSpread = nd.hasChoices && nd.ports && nd.ports.length > 0 ? 10 + nd.ports.length * 14 : 0;
      const rightEdge = nd.x + nd.w + laneSpread;
      if (nd.x < minX) minX = nd.x;
      if (nd.y < minY) minY = nd.y;
      if (rightEdge > maxX) maxX = rightEdge;
      if (nd.y + h > maxY) maxY = nd.y + h;
    }
    ch.x = minX - 36;
    ch.y = minY - 52;
    ch.w = maxX - minX + 72;
    ch.h = maxY - minY + 96;
    recomputeChapterInputPortPosition(ch);
  }
  for (let i = 0; i < chapters.length; i++) {
    if (!chapters[i].hidden) {
      recomputeChapterPortPositions(chapters[i]);
    }
  }
  studioGraphState.staticBatchDirty = true;
  return null;
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
  if (typeof initStudioWebGLContext === "function") {
    initStudioWebGLContext(canvas);
  }
  bindStudioViewportResizeObserver(viewport, canvas);
  invalidateStudioThemeCache();
  hideFloatingChapterInput(false);
  hideFloatingNodeTitleInput(false);
  hideFloatingChoiceInput(false);

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
  routeStudioWires();
  applyEditorCanvasTransform();
}
