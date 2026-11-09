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

function getNodeFooterBtnX(node, kind, isPreview) {
  if (kind === "add") {
    return node.x + CARD.pad;
  }
  if (kind === "delete") {
    return node.x + node.w - CARD.pad - CARD.btnSize;
  }
  const offset = isPreview ? CARD.btnSize : CARD.btnSize * 2 + 6;
  return node.x + node.w - CARD.pad - offset;
}

function recomputeNodeMetrics(node) {
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
      const oy = choiceRowY(i) + CARD.choiceH * 0.5;
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
        idx: 0,
        label: tr("next"),
        targetId: implicitTarget,
        targetChapter: implicitChapter,
        connected: impConnected,
        isBottom: true,
        ox: CARD.halfW,
        oy: node.h,
        x: node.x + CARD.halfW,
        y: node.y + node.h
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
      desiredInX = entryNode.inX;
    }
  }
  const minEdgeX = ch.x + GAP.grid;
  const maxEdgeX = Math.max(minEdgeX, ch.x + ch.w - GAP.grid);
  ch.inX = Math.max(minEdgeX, Math.min(maxEdgeX, desiredInX));
}

function recomputeChapterPortPositions(ch) {
  recomputeChapterInputPortPosition(ch);
  const minEdgeX = ch.x + GAP.grid;
  const maxEdgeX = Math.max(minEdgeX, ch.x + ch.w - 16);
  const minEdgeY = ch.y + GAP.node;
  const maxEdgeY = Math.max(minEdgeY, ch.y + ch.h - GAP.grid);

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
    const targetY = toCh ? (toCh.inY || toCh.y) : ch.y + ch.h + CARD.minChapterH;

    let sumX = 0;
    let sumY = 0;
    let maxSrcBottom = ch.y + PAD.h;
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
            const sy = pt.y;
            let candX;
            if (pt.isBottom) {
              candX = nd.inX;
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
              const rightLaneX = pt.x + 16 + stubRank * 14;
              const nodeCenterX = nd.inX;
              if (toCh && toCh.inX <= nd.right && portList.length > 1) {
                const desiredUnderNodeX = Math.max(nd.x + GAP.grid, Math.min(nd.right - GAP.grid, toCh.inX || nodeCenterX));
                if (
                  typeof linecastVertical !== "function" ||
                  !linecastVertical(desiredUnderNodeX, nd.bottom + 4, ch.y + ch.h, chNodes, nd, null, 10)
                ) {
                  candX = desiredUnderNodeX;
                } else {
                  candX = rightLaneX;
                }
              } else {
                candX = rightLaneX;
              }
              if (typeof linecastVertical === "function") {
                if (linecastVertical(candX, (candX <= nd.right ? nd.bottom : sy) + 6, ch.y + ch.h, chNodes, nd, null, 10)) {
                  hasBlockedDropToBottom = true;
                }
              }
              if (typeof resolveClearX === "function") {
                candX = resolveClearX(
                  candX,
                  candX <= nd.right ? nd.bottom + 4 : sy,
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
      const isLeftAdjacent = toCh.right <= ch.left + GAP.chapter;
      const isRightAdjacent = toCh.left >= ch.right - GAP.chapter;
      const isSideVerticalBand = toCh.top < ch.bottom - GAP.chapter;
      if (isLeftAdjacent && (isSideVerticalBand || (hasBlockedDropToBottom && toCh.top <= ch.bottom + GAP.chapter))) {
        side = "left";
      } else if (isRightAdjacent && (isSideVerticalBand || (hasBlockedDropToBottom && toCh.top <= ch.bottom + GAP.chapter))) {
        side = "right";
      }
    }
    op.side = side;

    if (side === "left" || side === "right") {
      op.x = side === "left" ? ch.x : ch.x + ch.w;
      let idealY = srcCount > 0 ? maxSrcBottom + GAP.grid : (ch.y + ch.h) * 0.5;
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
        idealX = chNodes[0].inX;
      }
      op.y = ch.y + ch.h;
      op._idealX = Math.max(minEdgeX, Math.min(maxEdgeX, idealX));
      op._targetX = targetX;
      bottomPorts.push(op);
    }
  }

  const minGap = GAP.grid;
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
    const firstX = bottomPorts[0]._idealX;
    const lastX = bottomPorts[bottomPorts.length - 1]._idealX;
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
    if (bottomPorts[i].x < bottomPorts[i - 1].x + minGap) {
      bottomPorts[i].x = bottomPorts[i - 1].x + minGap;
    }
  }
  if (bottomPorts.length > 0 && bottomPorts[bottomPorts.length - 1].x > maxEdgeX) {
    bottomPorts[bottomPorts.length - 1].x = maxEdgeX;
    for (let i = bottomPorts.length - 2; i >= 0; i--) {
      if (bottomPorts[i].x > bottomPorts[i + 1].x - minGap) {
        bottomPorts[i].x = Math.max(minEdgeX, bottomPorts[i + 1].x - minGap);
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
  studioGraphState.collectorBars = [];
  studioGraphState.hoveredItem = null;
  studioGraphState.staticBatchDirty = true;
  studioGraphState.overlayBatchDirty = true;

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
      if (colonIdx > 0) {
        kv[seg.slice(0, colonIdx).trim()] = seg.slice(colonIdx + 1).trim();
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

      const chName = kv.name || kv.ref || tr("default_chapter");
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
      const chName = kv.chapter || tr("default_chapter");
      const impTarget = kv.imp_target || "";
      const impChapter = kv.imp_chapter || "";
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
        implicitTarget: impTarget,
        implicitChapter: impChapter,
        impTarget: impTarget,
        impChapter: impChapter,
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
      const oy = choiceRowY(idx) + CARD.choiceH * 0.5;
      node.ports.push({
        idx: idx,
        label: kv.label || tr("choice_prefix") + (idx + 1),
        targetId: kv.target_id || "",
        targetChapter: kv.target_chapter || "",
        isConditional: kv.cond === "1",
        isBottom: false,
        ox: CARD.w,
        oy: oy,
        x: node.x + CARD.w,
        y: node.y + oy
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
  const gx0 = toSpatialCell(nd.left - GAP.node);
  const gx1 = toSpatialCell(nd.right + GAP.node);
  const gy0 = toSpatialCell(nd.top - GAP.node);
  const gy1 = toSpatialCell(nd.bottom + GAP.node);
  nd._gridMinX = gx0;
  nd._gridMaxX = gx1;
  nd._gridMinY = gy0;
  nd._gridMaxY = gy1;
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
  if (!grid || !nd || nd._gridMinX === undefined) return;
  for (let gy = nd._gridMinY; gy <= nd._gridMaxY; gy++) {
    const rowMask = (gy & 0xffff) << 16;
    for (let gx = nd._gridMinX; gx <= nd._gridMaxX; gx++) {
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
    const gx0 = toSpatialCell(nx - GAP.node);
    const gx1 = toSpatialCell(nr + GAP.node);
    const gy0 = toSpatialCell(ny - GAP.node);
    const gy1 = toSpatialCell(nb + GAP.node);
    for (let gy = gy0; gy <= gy1; gy++) {
      const rowMask = (gy & 0xffff) << 16;
      for (let gx = gx0; gx <= gx1; gx++) {
        const cell = grid.get(rowMask | (gx & 0xffff));
        if (!cell) continue;
        for (let i = 0; i < cell.length; i++) {
          const other = cell[i];
          if (other._isDragging) continue;
          if (
            nx < other.right + GAP.node &&
            nr + GAP.node > other.left &&
            ny < other.bottom + GAP.node &&
            nb + GAP.node > other.top
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
      nx < other.right + GAP.node &&
      nr + GAP.node > other.left &&
      ny < other.bottom + GAP.node &&
      nb + GAP.node > other.top
    ) {
      return true;
    }
  }
  return false;
}

function findClosestChapterAtWorldCoords(worldX, worldY) {
  const px = worldX + CARD.halfW;
  const py = worldY + GAP.node;
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
  const w = nodeW || CARD.w;
  const h = nodeH || CARD.baseH;
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
        cx < other.right + GAP.node &&
        cx + w + GAP.node > other.left &&
        cy < other.bottom + GAP.node &&
        cy + h + GAP.node > other.top
      ) {
        return true;
      }
    }
    let candLeft = cx - PAD.x;
    let candTop = cy - PAD.top;
    let candRight = cx + w + PAD.x;
    let candBottom = cy + Math.max(h, CARD.minChapterH) + PAD.bottom;
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
        candLeft < ch.right + GAP.chapter &&
        candRight + GAP.chapter > ch.left &&
        candTop < ch.bottom + GAP.chapter &&
        candBottom + GAP.chapter > ch.top
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

function findClosestValidDragDelta(desiredDx, desiredDy, lastValidDx, lastValidDy, maxSearchPx, isDeltaValid) {
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
    if (Math.abs(cx - desiredDx) > maxSearchPx) break;
  }
  const stepY = desiredDy >= lastValidDy ? -EDITOR_GRID_SIZE : EDITOR_GRID_SIZE;
  for (let cy = desiredDy; cy !== lastValidDy; cy += stepY) {
    tryCandidate(desiredDx, cy);
    tryCandidate(lastValidDx, cy);
    if (Math.abs(cy - desiredDy) > maxSearchPx) break;
  }
  return { dx: bestDx, dy: bestDy };
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
    const nh = Math.max(nd.h || CARD.baseH, CARD.minChapterH);
    if (it.startX < baseMinX) baseMinX = it.startX;
    if (it.startY < baseMinY) baseMinY = it.startY;
    if (it.startX + nd.w > baseMaxX) baseMaxX = it.startX + nd.w;
    if (it.startY + nh > baseMaxY) baseMaxY = it.startY + nh;
  }
  const baseLeft = baseMinX - PAD.x;
  const baseTop = baseMinY - PAD.top;
  const baseRight = baseMaxX + PAD.x;
  const baseBottom = baseMaxY + PAD.bottom;

  return findClosestValidDragDelta(desiredDx, desiredDy, lastValidDx, lastValidDy, 960, (dx, dy) => {
    const l = baseLeft + dx;
    const t = baseTop + dy;
    const r = baseRight + dx;
    const b = baseBottom + dy;
    for (let i = 0; i < chapters.length; i++) {
      const other = chapters[i];
      if (other === ch || other.hidden) continue;
      if (
        l < other.right + GAP.chapter &&
        r + GAP.chapter > other.left &&
        t < other.bottom + GAP.chapter &&
        b + GAP.chapter > other.top
      ) {
        return false;
      }
    }
    return true;
  });
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
        const nh = Math.max(nd.h || CARD.baseH, CARD.minChapterH);
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
      ch._staticMinX = sMinX;
      ch._staticMinY = sMinY;
      ch._staticMaxX = sMaxX;
      ch._staticMaxY = sMaxY;
      ch._initialLeft = iMinX - PAD.x;
      ch._initialTop = iMinY - PAD.top;
      ch._initialRight = iMaxX + PAD.x;
      ch._initialBottom = iMaxY + PAD.bottom;
      affectedChapters.push(ch);
    }
  }

  const globalGrid = studioGraphState.nodeSpatialGrid;

  const res = findClosestValidDragDelta(desiredDx, desiredDy, lastValidDx, lastValidDy, 720, (dx, dy) => {
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
      let minX = ch._staticMinX;
      let minY = ch._staticMinY;
      let maxX = ch._staticMaxX;
      let maxY = ch._staticMaxY;
      for (let k = 0; k < items.length; k++) {
        const it = items[k];
        const nd = it.node;
        if (nd.chapterObj !== ch) continue;
        const nx = snapToEditorGrid(it.startX + dx);
        const ny = snapToEditorGrid(it.startY + dy);
        const nh = Math.max(nd.h || CARD.baseH, CARD.minChapterH);
        if (nx < minX) minX = nx;
        if (ny < minY) minY = ny;
        if (nx + nd.w > maxX) maxX = nx + nd.w;
        if (ny + nh > maxY) maxY = ny + nh;
      }
      const cLeft = minX - PAD.x;
      const cTop = minY - PAD.top;
      const cRight = maxX + PAD.x;
      const cBottom = maxY + PAD.bottom;

      for (let o = 0; o < chapters.length; o++) {
        const otherCh = chapters[o];
        if (otherCh === ch || otherCh.hidden) continue;
        if (
          cLeft < otherCh.right + GAP.chapter &&
          cRight + GAP.chapter > otherCh.left &&
          cTop < otherCh.bottom + GAP.chapter &&
          cBottom + GAP.chapter > otherCh.top
        ) {
          const wasOverlapping =
            ch._initialLeft < otherCh.right + GAP.chapter &&
            ch._initialRight + GAP.chapter > otherCh.left &&
            ch._initialTop < otherCh.bottom + GAP.chapter &&
            ch._initialBottom + GAP.chapter > otherCh.top;
          if (!wasOverlapping) {
            return false;
          }
          if (
            cLeft < ch._initialLeft ||
            cRight > ch._initialRight ||
            cTop < ch._initialTop ||
            cBottom > ch._initialBottom
          ) {
            return false;
          }
        }
      }
    }
    return true;
  });

  for (let i = 0; i < items.length; i++) {
    items[i].node._isDragging = false;
  }
  return res;
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
  const cleanCoords = resolveNonOverlappingNodeCoords(baseX, baseY, resolvedChapter, CARD.w, CARD.baseH);
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
    if (sx2 > GAP.grid && sx1 < vw - GAP.grid && sy2 > GAP.grid && sy1 < vh - GAP.grid) {
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
    if (nd.x - GAP.grid < minX) minX = nd.x - GAP.grid;
    if (nd.y - PAD.top < minY) minY = nd.y - PAD.top;
    if (nd.x + nd.w + GAP.grid > maxX) maxX = nd.x + nd.w + GAP.grid;
    if (nd.y + nd.h + GAP.grid > maxY) maxY = nd.y + nd.h + GAP.grid;
  }

  const vw = studioGraphState.viewW || (viewport && viewport.clientWidth) || window.innerWidth || 1200;
  const vh = studioGraphState.viewH || (viewport && viewport.clientHeight) || window.innerHeight || 700;
  const padX = GAP.node;
  const padY = GAP.node;
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
  const editorBody = qs("#editor-workspace-body");
  const fullW =
    editorBody &&
    editorBody.clientWidth > 0 &&
    (typeof isPortraitPhoneStudioLayout !== "function" || !isPortraitPhoneStudioLayout())
      ? editorBody.clientWidth
      : 0;
  const vw = fullW || studioGraphState.viewW || (viewport && viewport.clientWidth) || window.innerWidth || 1200;
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
  const fx = startNode ? startNode.inX : 960;
  const fy = startNode ? startNode.y : 96;
  editorCanvasState.zoom = 1.0;
  editorCanvasState.panX = Math.round(vw * 0.5 - fx);
  editorCanvasState.panY = Math.round(PAD.w - fy);
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

function applyFloatingInputBoxStyle(inp, worldX, worldY, worldW, worldH, baseFontPx, baseLetterSpacing) {
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;
  inp.style.left = (worldX * z + px).toFixed(2) + "px";
  inp.style.top = (worldY * z + 0.5 + py).toFixed(2) + "px";
  inp.style.width = Math.max(24, worldW * z).toFixed(2) + "px";
  inp.style.height = Math.max(4, worldH * z).toFixed(2) + "px";
  inp.style.fontSize = (baseFontPx * z).toFixed(2) + "px";
  if (baseLetterSpacing !== undefined) {
    inp.style.letterSpacing = (baseLetterSpacing * z).toFixed(2) + "px";
  }
}

function showFloatingCanvasInputEl(inp, positionFn) {
  studioGraphState.staticBatchDirty = true;
  inp.classList.remove("hidden");
  positionFn();
  inp.focus();
  inp.select();
  renderStudioCanvas();
}

function positionFloatingChapterInput() {
  const inp = qs("#canvas-chapter-input");
  const ch = studioGraphState.editingChapter;
  if (!inp || !ch || inp.classList.contains("hidden")) return;
  applyFloatingInputBoxStyle(inp, ch.x + 28, ch.y + 4.703, ch.w - 38, 20, 11, 0.65);
}

function openFloatingChapterInput(ch) {
  const inp = qs("#canvas-chapter-input");
  if (!inp || !ch) return;
  hideFloatingNodeTitleInput(true);
  hideFloatingChoiceInput(true);
  studioGraphState.editingChapter = ch;
  inp.value = ch.name;
  inp.setAttribute("data-old-chapter", ch.name);
  showFloatingCanvasInputEl(inp, positionFloatingChapterInput);
}

function hideFloatingChapterInput(shouldCommit) {
  const inp = qs("#canvas-chapter-input");
  if (!inp || inp.classList.contains("hidden")) return;
  if (shouldCommit && typeof commitChapterTitleInput === "function") commitChapterTitleInput(inp);
  studioGraphState.editingChapter = null;
  studioGraphState.staticBatchDirty = true;
  inp.classList.add("hidden");
  renderStudioCanvas();
}

function positionFloatingNodeTitleInput() {
  const inp = qs("#canvas-node-title-input");
  const nd = studioGraphState.editingNodeTitle;
  if (!inp || !nd || inp.classList.contains("hidden")) return;
  const radioReserve = nd.branchRole === "primary" || nd.branchRole === "secondary" ? 16 : 0;
  const badgeReserve = (nd.isStart || nd.isEntry ? 50 : 8) + radioReserve;
  applyFloatingInputBoxStyle(inp, nd.x + 28, nd.y + 6.715, nd.w - 28 - badgeReserve, 22, 13);
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
  if (typeof syncSceneTitleAcrossForms === "function") {
    syncSceneTitleAcrossForms(nd.id, newName, inp);
  }
  postServerActionSilent("AutoSaveScene", { scene_id: nd.id, name: newName });
}

function openFloatingNodeTitleInput(nd) {
  const inp = qs("#canvas-node-title-input");
  if (!inp || !nd) return;
  hideFloatingChapterInput(true);
  hideFloatingChoiceInput(true);
  studioGraphState.editingNodeTitle = nd;
  inp.value = nd.name || nd.id || "";
  inp.setAttribute("data-scene-id", nd.id);
  showFloatingCanvasInputEl(inp, positionFloatingNodeTitleInput);
}

function hideFloatingNodeTitleInput(shouldCommit) {
  const inp = qs("#canvas-node-title-input");
  if (!inp || inp.classList.contains("hidden")) return;
  if (shouldCommit) commitNodeTitleInput(inp);
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
  const rowY = nd.y + CARD.headerH + ec.idx * CARD.choiceH;
  applyFloatingInputBoxStyle(inp, nd.x + 21, rowY + 2.703, nd.w - 47, CARD.choiceH - 6, 11);
}

function commitChoiceInput(inp) {
  const ec = studioGraphState.editingChoice;
  if (!inp || !ec || !ec.node) return;
  const nd = ec.node;
  const idx = ec.idx;
  const pt = nd.ports && nd.ports[idx];
  if (!pt) return;
  const newLabel = (inp.value || "").trim() || tr("choice_prefix") + (idx + 1);
  if (newLabel === (pt.label || "")) return;
  pt.label = newLabel;
  studioGraphState.staticBatchDirty = true;
  renderStudioCanvas();
  ["#form-scene-text", "#editor-context-form"].forEach((sel) => {
    const f = qs(sel);
    if (!f) return;
    const fScId = f.getAttribute("data-context-scene-id") || (qs('input[name="scene_id"]', f) && qs('input[name="scene_id"]', f).value) || "";
    if (fScId === nd.id) {
      const chInp = qs('[name="choice_label_' + idx + '"]', f);
      if (chInp && chInp !== inp) chInp.value = newLabel;
    }
  });
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
  inp.value = nd.ports[choiceIdx].label || tr("choice_prefix") + (choiceIdx + 1);
  inp.setAttribute("data-scene-id", nd.id);
  inp.setAttribute("data-choice-idx", String(choiceIdx));
  showFloatingCanvasInputEl(inp, positionFloatingChoiceInput);
}

function hideFloatingChoiceInput(shouldCommit) {
  const inp = qs("#canvas-choice-input");
  if (!inp || inp.classList.contains("hidden")) return;
  if (shouldCommit) commitChoiceInput(inp);
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

function computeChapterFrameRect(matching) {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (let j = 0; j < matching.length; j++) {
    const nd = matching[j];
    const h = Math.max(nd.h || CARD.baseH, CARD.minChapterH);
    const laneSpread = nd.hasChoices && nd.ports && nd.ports.length > 0 ? 10 + nd.ports.length * 14 : 0;
    const rightEdge = nd.x + nd.w + laneSpread;
    if (nd.x < minX) minX = nd.x;
    if (nd.y < minY) minY = nd.y;
    if (rightEdge > maxX) maxX = rightEdge;
    if (nd.y + h > maxY) maxY = nd.y + h;
  }
  return {
    x: minX - PAD.x,
    y: minY - PAD.top,
    w: maxX - minX + PAD.w,
    h: maxY - minY + PAD.h
  };
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
      const rect = computeChapterFrameRect(matching);
      const frameChanged =
        ch.x !== rect.x || ch.y !== rect.y || ch.w !== rect.w || ch.h !== rect.h;
      const prevInX = ch.inX;
      const prevInY = ch.inY;
      ch.x = rect.x;
      ch.y = rect.y;
      ch.w = rect.w;
      ch.h = rect.h;
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
    const key = (nd.chapter || tr("default_chapter")).trim().toLowerCase();
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
      const displayCh = memberNodes[0].chapter || tr("default_chapter");
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
    for (let j = 0; j < matching.length; j++) {
      matching[j].chapterObj = ch;
    }
    const rect = computeChapterFrameRect(matching);
    ch.x = rect.x;
    ch.y = rect.y;
    ch.w = rect.w;
    ch.h = rect.h;
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
  const isChapterOpen = Boolean(chapterForm && !chapterForm.classList.contains("hidden"));
  const isBookOpen = Boolean(bookForm && !bookForm.classList.contains("hidden"));
  if (isChapterOpen || isBookOpen) {
    selectedNodeIds.clear();
    studioGraphState.selectedChapterKey = isChapterOpen
      ? (chapterForm.getAttribute("data-context-chapter") || "").trim().toLowerCase()
      : "";
  } else {
    studioGraphState.selectedChapterKey = "";
    const modalSceneInput = qs('#modal-scene-text input[name="scene_id"]');
    const modalSceneId = modalSceneInput ? (modalSceneInput.value || "").trim() : "";
    const serverSelectedId =
      modalSceneId ||
      (contextForm ? (contextForm.getAttribute("data-context-scene-id") || "").trim() : "");
    if (
      !editorSelectionCleared &&
      serverSelectedId &&
      selectedNodeIds.size <= 1
    ) {
      selectedNodeIds.clear();
      selectedNodeIds.add(serverSelectedId);
    }
  }

  updateChapterGroupFrames();
  syncNodeSelectionDOM();
  routeStudioWires();
  restoreStudioSearchUIState();
  applyEditorCanvasTransform();
}

let studioSearchOpen = false;
let studioSearchQuery = "";
let pendingSceneSearchHighlight = "";

function centerCanvasCameraOnScene(sceneId) {
  if (!sceneId || !studioGraphState.nodeById) return;
  const nd = studioGraphState.nodeById.get(sceneId);
  if (!nd) return;
  const viewport = qs("#timeline-scroll-viewport");
  const vw = studioGraphState.viewW || (viewport && viewport.clientWidth) || 1200;
  const vh = studioGraphState.viewH || (viewport && viewport.clientHeight) || 700;
  const z = editorCanvasState.zoom || 1.0;
  const cx = nd.x + (nd.w || CARD.w) * 0.5;
  const cy = nd.y + (nd.h || CARD.baseH) * 0.5;
  editorCanvasState.panX = Math.round(vw * 0.5 - cx * z);
  editorCanvasState.panY = Math.round(vh * 0.5 - cy * z);
  selectedNodeIds.clear();
  selectedNodeIds.add(sceneId);
  editorSelectionCleared = false;
  syncNodeSelectionDOM();
  applyEditorCanvasTransform();
  scheduleSaveEditorViewport();
}

function buildSearchSnippetParts(sourceText, matchIdx, matchLen) {
  const flat = (sourceText || "").replace(/\s+/g, " ").trim();
  if (!flat || matchIdx < 0) {
    return { pre: flat.slice(0, 84), hit: "", post: "" };
  }
  const start = Math.max(0, matchIdx - 28);
  const end = Math.min(flat.length, matchIdx + matchLen + 36);
  const prefix = start > 0 ? "..." : "";
  const suffix = end < flat.length ? "..." : "";
  return {
    pre: prefix + flat.slice(start, matchIdx),
    hit: flat.slice(matchIdx, matchIdx + matchLen),
    post: flat.slice(matchIdx + matchLen, end) + suffix
  };
}

function decodeSearchBase64UTF8(b64) {
  if (!b64) return "";
  try {
    const bin = atob(b64);
    if (typeof TextDecoder !== "undefined") {
      const bytes = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) {
        bytes[i] = bin.charCodeAt(i);
      }
      return new TextDecoder("utf-8").decode(bytes);
    }
    return decodeURIComponent(escape(bin));
  } catch (_e) {
    return "";
  }
}

function detectTextMatchBadge(rawText, queryLower) {
  const lines = (rawText || "").split(/\r?\n/);
  let prevWasCue = false;
  for (let i = 0; i < lines.length; i++) {
    const ln = lines[i].trim();
    const isCue = ln.length >= 2 && ln.length <= 42 && ln === ln.toUpperCase() && ln !== ln.toLowerCase() && !/^(WN\.|PL\.|INT\.|EXT\.)/.test(ln);
    if (ln.toLowerCase().indexOf(queryLower) >= 0) {
      if (isCue || prevWasCue || ln.indexOf(":") > 0) {
        return tr("search_badge_dialogue");
      }
      break;
    }
    prevWasCue = isCue;
  }
  return tr("search_badge_prose");
}

function applyStudioSearchFilter(rawQuery) {
  studioSearchQuery = (rawQuery || "").trim();
  const qLower = studioSearchQuery.toLowerCase();
  const resultsRoot = qs("#studio-search-results");
  const countEl = qs("#studio-search-count");
  if (!resultsRoot) return;

  const rows = qsa(".studio-search-row", resultsRoot);
  const matchedSceneIds = new Set();
  let visibleCount = 0;
  let firstVisibleRow = null;
  let hasActiveVisible = false;

  for (let i = 0; i < rows.length; i++) {
    const row = rows[i];
    const sceneId = row.getAttribute("data-search-scene") || "";
    const title = row.getAttribute("data-search-title") || "";
    const slug = row.getAttribute("data-search-slug") || "";
    const chapter = row.getAttribute("data-search-chapter") || "";
    const synopsis = row.getAttribute("data-search-synopsis") || "";
    const text = decodeSearchBase64UTF8(row.getAttribute("data-search-b64") || "");
    const choices = row.getAttribute("data-search-choices") || "";
    const defBadge = row.getAttribute("data-default-badge") || "";
    const defSnip = row.getAttribute("data-default-snippet") || "";

    const badgeEl = qs(".studio-search-row-badge", row);
    const preEl = qs(".studio-search-snip-pre", row);
    const hitEl = qs(".studio-search-snip-hit", row);
    const postEl = qs(".studio-search-snip-post", row);

    if (!qLower) {
      row.classList.remove("hidden");
      if (badgeEl) badgeEl.textContent = defBadge;
      if (preEl) preEl.textContent = defSnip;
      if (hitEl) hitEl.textContent = "";
      if (postEl) postEl.textContent = "";
      visibleCount++;
      if (!firstVisibleRow) firstVisibleRow = row;
      if (row.classList.contains("is-active")) hasActiveVisible = true;
      continue;
    }

    const flatText = (text + " " + synopsis).replace(/\s+/g, " ").trim();
    const flatChoices = choices.replace(/\s+/g, " ").trim();
    const flatSlug = slug.replace(/\s+/g, " ").trim();
    const flatTitle = (title + " " + chapter).replace(/\s+/g, " ").trim();

    let badge = "";
    let snipSource = defSnip;
    let hitPos = -1;

    if (sceneId.toLowerCase().indexOf(qLower) >= 0) {
      badge = "ID";
      snipSource = sceneId + (defSnip ? " — " + defSnip : "");
      hitPos = snipSource.toLowerCase().indexOf(qLower);
    } else if (flatSlug.toLowerCase().indexOf(qLower) >= 0) {
      badge = tr("search_badge_location");
      snipSource = flatSlug;
      hitPos = flatSlug.toLowerCase().indexOf(qLower);
    } else if (flatTitle.toLowerCase().indexOf(qLower) >= 0) {
      badge = tr("search_badge_scene") || defBadge;
      snipSource = flatTitle;
      hitPos = flatTitle.toLowerCase().indexOf(qLower);
    } else if (flatChoices.toLowerCase().indexOf(qLower) >= 0) {
      badge = tr("search_badge_choice");
      snipSource = flatChoices;
      hitPos = flatChoices.toLowerCase().indexOf(qLower);
    } else if (flatText.toLowerCase().indexOf(qLower) >= 0) {
      badge = detectTextMatchBadge(text, qLower);
      snipSource = flatText;
      hitPos = flatText.toLowerCase().indexOf(qLower);
    }

    if (hitPos < 0) {
      row.classList.add("hidden");
      row.classList.remove("is-active");
      continue;
    }

    row.classList.remove("hidden");
    matchedSceneIds.add(sceneId);
    visibleCount++;
    if (!firstVisibleRow) firstVisibleRow = row;
    if (row.classList.contains("is-active")) hasActiveVisible = true;

    const parts = buildSearchSnippetParts(snipSource, hitPos, studioSearchQuery.length);
    if (badgeEl) badgeEl.textContent = badge;
    if (preEl) preEl.textContent = parts.pre;
    if (hitEl) hitEl.textContent = parts.hit;
    if (postEl) postEl.textContent = parts.post;
  }

  if (!hasActiveVisible && firstVisibleRow) {
    firstVisibleRow.classList.add("is-active");
  }
  if (countEl) {
    countEl.textContent = String(visibleCount);
  }

  if (studioSearchOpen && qLower) {
    studioGraphState.searchQuery = qLower;
    studioGraphState.searchMatchSet = matchedSceneIds;
  } else {
    studioGraphState.searchQuery = "";
    studioGraphState.searchMatchSet = null;
  }
  studioGraphState.staticBatchDirty = true;
  renderStudioCanvas();
}

function restoreStudioSearchUIState() {
  const palette = qs("#studio-search-palette");
  const btn = qs("#btn-studio-search");
  const inp = qs("#studio-search-input");
  if (!palette) return;
  palette.classList.toggle("hidden", !studioSearchOpen);
  if (btn) btn.classList.toggle("active", studioSearchOpen);
  if (studioSearchOpen) {
    if (inp && inp.value !== studioSearchQuery) {
      inp.value = studioSearchQuery;
    }
    applyStudioSearchFilter(studioSearchQuery);
  } else if (studioGraphState.searchQuery) {
    studioGraphState.searchQuery = "";
    studioGraphState.searchMatchSet = null;
    studioGraphState.staticBatchDirty = true;
  }
}

function openStudioSearchPalette() {
  const palette = qs("#studio-search-palette");
  const btn = qs("#btn-studio-search");
  const inp = qs("#studio-search-input");
  if (!palette) return;
  studioSearchOpen = true;
  palette.classList.remove("hidden");
  if (btn) btn.classList.add("active");
  applyStudioSearchFilter(inp ? inp.value : studioSearchQuery);
  if (inp) {
    inp.focus();
    inp.select();
  }
}

function closeStudioSearchPalette() {
  const palette = qs("#studio-search-palette");
  const btn = qs("#btn-studio-search");
  studioSearchOpen = false;
  if (palette) palette.classList.add("hidden");
  if (btn) btn.classList.remove("active");
  if (studioGraphState.searchQuery) {
    studioGraphState.searchQuery = "";
    studioGraphState.searchMatchSet = null;
    studioGraphState.staticBatchDirty = true;
    renderStudioCanvas();
  }
}

function toggleStudioSearchPalette() {
  if (studioSearchOpen) {
    closeStudioSearchPalette();
  } else {
    openStudioSearchPalette();
  }
}

function stepStudioSearchActiveRow(delta) {
  const resultsRoot = qs("#studio-search-results");
  if (!resultsRoot) return;
  const visible = qsa(".studio-search-row:not(.hidden)", resultsRoot);
  if (visible.length === 0) return;
  let curIdx = 0;
  for (let i = 0; i < visible.length; i++) {
    if (visible[i].classList.contains("is-active")) {
      curIdx = i;
      visible[i].classList.remove("is-active");
    }
  }
  const nextIdx = (curIdx + delta + visible.length) % visible.length;
  const nextRow = visible[nextIdx];
  nextRow.classList.add("is-active");
  if (typeof nextRow.scrollIntoView === "function") {
    nextRow.scrollIntoView({ block: "nearest" });
  }
  const sceneId = nextRow.getAttribute("data-search-scene") || "";
  if (sceneId) {
    centerCanvasCameraOnScene(sceneId);
  }
}

function activateStudioSearchRow(sceneId, openTextModal) {
  if (!sceneId) return;
  const resultsRoot = qs("#studio-search-results");
  if (resultsRoot) {
    qsa(".studio-search-row", resultsRoot).forEach((r) => {
      r.classList.toggle("is-active", r.getAttribute("data-search-scene") === sceneId);
    });
  }
  centerCanvasCameraOnScene(sceneId);
  if (openTextModal) {
    pendingSceneSearchHighlight = studioSearchQuery;
    closeStudioSearchPalette();
    sendServerAction("OpenSceneTextModal", { scene: sceneId });
  } else {
    sendServerAction("SelectStudioScene", { scene: sceneId });
  }
}

function applyPendingSceneTextSearchHighlight() {
  if (!pendingSceneSearchHighlight) return;
  const q = pendingSceneSearchHighlight;
  pendingSceneSearchHighlight = "";
  const ta = qs("#modal-scene-textarea");
  if (!ta || !ta.value) return;
  const idx = ta.value.toLowerCase().indexOf(q.toLowerCase());
  if (idx >= 0) {
    ta.focus();
    if (typeof ta.setSelectionRange === "function") {
      ta.setSelectionRange(idx, idx + q.length);
    }
  }
}
