"use strict";

// NodeCanvasModel: In-memory book graph model, parser, node/chapter metrics,
// chapter frame bounds, and incremental node/port mutations.
const NodeCanvasModel = {
  tokens: {
    rolePrimary: "primary",
    roleSecondary: "secondary",
    endTarget: "END",
    nextKey: "next",
    defaultChapterKey: "default_chapter",
    choicePrefixKey: "choice_prefix",
    sideBottom: "bottom",
    sideLeft: "left",
    sideRight: "right",
    chIdPrefix: "ch_",
    recPos: "pos",
    recPin: "pin",
    recWirePins: "wire_pins",
    recChapter: "chapter",
    recNode: "node",
    recPort: "port",
    bitTrue: "1",
    axisX: "x",
    axisY: "y",
    zeroStr: "0",
    pipeSep: " | ",
    objType: "object",
    undefType: "undefined"
  },

  choiceRowY(idx) {
    return NodeCanvasStyle.card.headerH + idx * NodeCanvasStyle.card.choiceH;
  },

  getActiveNodePorts(node) {
    if (!node) return [];
    if (node.hasChoices && node.ports && node.ports.length > 0) return node.ports;
    return node.implicitPort ? [node.implicitPort] : [];
  },

  createChapterRecord(spec) {
    const outPorts = spec.outPorts && spec.outPorts.length > 0
      ? spec.outPorts
      : [{ idx: 0, target: "", targetLower: "", connected: false, side: this.tokens.sideBottom, x: 0, y: 0 }];
    return NodeCanvasGeometry.updateRect({
      id: this.tokens.chIdPrefix + spec.refLower,
      rawId: spec.rawId || spec.ref,
      ref: spec.ref,
      refLower: spec.refLower,
      name: spec.name,
      nameLower: spec.nameLower,
      isRoot: Boolean(spec.isRoot),
      startSceneId: spec.startSceneId || "",
      x: 0,
      y: 0,
      w: 240,
      h: 140,
      inX: 0,
      inY: 0,
      outPorts: outPorts,
      nodes: [],
      hidden: false
    });
  },

  recomputeNodeMetrics(node) {
    const card = NodeCanvasStyle.card;
    const tok = this.tokens;
    node.w = card.w;
    const portCount = node.hasChoices && node.ports ? node.ports.length : 0;
    node.h = portCount > 0 ? card.baseH + portCount * card.choiceH : card.baseH;
    NodeCanvasGeometry.updateRect(node);
    node.inX = node.x + card.halfW;
    node.inY = node.y;
    node.hasInputPort = !node.isStart;
    const hasBranchRadio = node.branchRole === tok.rolePrimary || node.branchRole === tok.roleSecondary;
    node.badgeW = node._shapedBadge
      ? Math.ceil(node._shapedBadge.totalW) + card.badgePadX
      : node.isStart
        ? card.startBadgeW
        : node.isEntry
          ? card.entryBadgeW
          : 0;
    node.branchRadioX = hasBranchRadio ? node.x + node.w - (node.badgeW > 0 ? node.badgeW + card.pad + 14 : card.radioReserveW) : 0;
    node.branchRadioY = hasBranchRadio ? node.y + card.radioYOffset : 0;
    node.titleMaxW = node.w - 36 - (node.badgeW > 0 ? node.badgeW + 6 : 0) - (hasBranchRadio ? card.radioReserveW : 0);

    if (portCount > 0) {
      for (let idx = 0; idx < portCount; idx++) {
        const port = node.ports[idx];
        port.isBottom = false;
        port.x = node.x + card.w;
        port.y = node.y + this.choiceRowY(idx) + card.choiceH * 0.5;
        port.connected = Boolean((port.targetId && port.targetId.toUpperCase() !== tok.endTarget) || (port.targetChapter && port.targetChapter.trim()));
      }
      node.implicitPort = null;
      return;
    }
    const implicitTarget = node.implicitTarget || "";
    const implicitChapter = node.implicitChapter || "";
    const impConnected = Boolean((implicitTarget && implicitTarget.toUpperCase() !== tok.endTarget) || (implicitChapter && implicitChapter.trim()));
    if (!node.implicitPort) {
      node.implicitPort = {
        idx: 0,
        label: UIController.tr(tok.nextKey),
        targetId: implicitTarget,
        targetChapter: implicitChapter,
        connected: impConnected,
        isBottom: true,
        x: node.x + card.halfW,
        y: node.y + node.h
      };
      return;
    }
    const impPort = node.implicitPort;
    impPort.targetId = implicitTarget;
    impPort.targetChapter = implicitChapter;
    impPort.connected = impConnected;
    impPort.isBottom = true;
    impPort.x = node.x + card.halfW;
    impPort.y = node.y + node.h;
  },

  resolveChapterEntryNode(chapter) {
    const graphState = NodeCanvasLayout.studioGraphState;
    let entryNode = chapter.startSceneId ? graphState.nodeById.get(chapter.startSceneId) : null;
    if (!entryNode || (entryNode.chapterLower !== chapter.nameLower && entryNode.chapterLower !== chapter.refLower)) {
      const chNodes = chapter.nodes || graphState.nodes;
      entryNode =
        (chNodes && chNodes.find((cand) => (cand.chapterLower === chapter.nameLower || cand.chapterLower === chapter.refLower) && cand.isEntry)) ||
        (chNodes && chNodes[0]) ||
        null;
    }
    return entryNode;
  },

  recomputeChapterInputPortPosition(chapter) {
    const gridGap = NodeCanvasStyle.gap.grid;
    NodeCanvasGeometry.updateRect(chapter);
    chapter.inY = chapter.y;
    const entryNode = this.resolveChapterEntryNode(chapter);
    const desiredInX = entryNode ? entryNode.inX : chapter.x + chapter.w * 0.5;
    const minInX = chapter.x + gridGap;
    const maxInX = Math.max(minInX, chapter.x + chapter.w - gridGap);
    chapter.inX = Math.max(minInX, Math.min(maxInX, desiredInX));
  },

  spreadPortsAlongEdge(ports, axisProp, minGap, maxCoord) {
    for (let idx = 1; idx < ports.length; idx++) {
      if (ports[idx][axisProp] < ports[idx - 1][axisProp] + minGap) {
        ports[idx][axisProp] = Math.min(maxCoord, ports[idx - 1][axisProp] + minGap);
      }
    }
  },

  recomputeChapterPortPositions(chapter) {
    const gap = NodeCanvasStyle.gap;
    const tok = this.tokens;
    const graphState = NodeCanvasLayout.studioGraphState;
    this.recomputeChapterInputPortPosition(chapter);
    const minEdgeX = chapter.x + gap.grid;
    const maxEdgeX = Math.max(minEdgeX, chapter.x + chapter.w - 16);
    const minEdgeY = chapter.y + gap.node;
    const maxEdgeY = Math.max(minEdgeY, chapter.y + chapter.h - gap.grid);
    if (!chapter.outPorts || chapter.outPorts.length === 0) {
      chapter.outPorts = [{ idx: 0, target: "", targetLower: "", connected: false, side: tok.sideBottom, x: chapter.x + chapter.w * 0.5, y: chapter.y + chapter.h }];
    }
    const chNodes = chapter.nodes || [];
    const bottomPorts = [];
    const leftPorts = [];
    const rightPorts = [];
    for (let portIdx = 0; portIdx < chapter.outPorts.length; portIdx++) {
      const outPort = chapter.outPorts[portIdx];
      outPort.idx = portIdx;
      outPort.connected = Boolean(outPort.targetLower);
      const targetChapter = outPort.targetLower ? graphState.chapterByKey.get(outPort.targetLower) : null;
      const targetX = targetChapter ? (targetChapter.inX || targetChapter.x + targetChapter.w * 0.5) : chapter.x + chapter.w * 0.5;
      let sumX = 0;
      let srcCount = 0;
      for (let nodeIdx = 0; outPort.targetLower && nodeIdx < chNodes.length; nodeIdx++) {
        const portList = this.getActiveNodePorts(chNodes[nodeIdx]);
        for (let choiceIdx = 0; choiceIdx < portList.length; choiceIdx++) {
          const targetLower = (portList[choiceIdx].targetChapter || "").trim().toLowerCase();
          if (targetLower && (targetLower === outPort.targetLower || (targetChapter && (targetLower === targetChapter.nameLower || targetLower === targetChapter.refLower)))) {
            sumX += portList[choiceIdx].x;
            srcCount++;
          }
        }
      }
      let side = tok.sideBottom;
      if (targetChapter && !targetChapter.hidden && targetChapter.top < chapter.bottom - gap.chapter) {
        if (targetChapter.right <= chapter.left + gap.chapter) side = tok.sideLeft;
        else if (targetChapter.left >= chapter.right - gap.chapter) side = tok.sideRight;
      }
      outPort.side = side;
      if (side === tok.sideLeft || side === tok.sideRight) {
        outPort.x = side === tok.sideLeft ? chapter.x : chapter.x + chapter.w;
        outPort.y = Math.max(minEdgeY, Math.min(maxEdgeY, Math.round((chapter.y + chapter.h) * 0.5)));
        if (side === tok.sideLeft) leftPorts.push(outPort);
        else rightPorts.push(outPort);
      } else {
        const idealX = srcCount > 0 ? sumX / srcCount : (outPort.targetLower && targetChapter ? targetX : chapter.x + chapter.w * 0.5);
        outPort.x = Math.max(minEdgeX, Math.min(maxEdgeX, idealX));
        outPort.y = chapter.y + chapter.h;
        bottomPorts.push(outPort);
      }
    }
    bottomPorts.sort((portA, portB) => portA.x - portB.x);
    this.spreadPortsAlongEdge(bottomPorts, tok.axisX, gap.grid, maxEdgeX);
    this.spreadPortsAlongEdge(leftPorts, tok.axisY, gap.grid, maxEdgeY);
    this.spreadPortsAlongEdge(rightPorts, tok.axisY, gap.grid, maxEdgeY);
  },

  computeChapterFrameRect(matchingNodes, draggedRects) {
    const card = NodeCanvasStyle.card;
    const pad = NodeCanvasStyle.pad;
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (let idx = 0; idx < matchingNodes.length; idx++) {
      const item = matchingNodes[idx];
      const isDragItem = Boolean(item.node);
      const node = isDragItem ? item.node : item;
      let nodeX = isDragItem ? item.startX : node.x;
      let nodeY = isDragItem ? item.startY : node.y;
      let nodeW = node.w || card.w;
      let nodeH = node.h || card.baseH;
      if (!isDragItem && draggedRects && node._isDragging) {
        for (let dragIdx = 0; dragIdx < draggedRects.length; dragIdx++) {
          if (draggedRects[dragIdx].node === node) {
            nodeX = draggedRects[dragIdx].x;
            nodeY = draggedRects[dragIdx].y;
            nodeW = draggedRects[dragIdx].w;
            nodeH = draggedRects[dragIdx].h;
            break;
          }
        }
      }
      const height = Math.max(nodeH, card.minChapterH);
      const laneSpread = node.hasChoices && node.ports && node.ports.length > 0
        ? card.laneSpreadBase + node.ports.length * card.laneSpreadStep
        : 0;
      minX = Math.min(minX, nodeX);
      minY = Math.min(minY, nodeY);
      maxX = Math.max(maxX, nodeX + nodeW + laneSpread);
      maxY = Math.max(maxY, nodeY + height);
    }
    return NodeCanvasGeometry.updateRect({
      x: minX - pad.x,
      y: minY - pad.top,
      w: maxX - minX + pad.w,
      h: maxY - minY + pad.h
    });
  },

  updateChaptersIncremental(chaptersSet, hasInterChapterPortMove) {
    const graphState = NodeCanvasLayout.studioGraphState;
    const changedChapters = new Set();
    chaptersSet.forEach((chapter) => {
      if (!chapter.nodes || chapter.nodes.length === 0) return;
      const rect = this.computeChapterFrameRect(chapter.nodes);
      const frameChanged = chapter.x !== rect.x || chapter.y !== rect.y || chapter.w !== rect.w || chapter.h !== rect.h;
      const prevInX = chapter.inX;
      const prevInY = chapter.inY;
      NodeCanvasGeometry.copyRect(chapter, rect);
      this.recomputeChapterInputPortPosition(chapter);
      if (frameChanged || chapter.inX !== prevInX || chapter.inY !== prevInY || hasInterChapterPortMove) {
        this.recomputeChapterPortPositions(chapter);
        changedChapters.add(chapter);
      }
    });
    graphState.staticBatchDirty = true;
    return changedChapters;
  },

  rebuildChapterFrames() {
    const tok = this.tokens;
    const graphState = NodeCanvasLayout.studioGraphState;
    const nodes = graphState.nodes;
    const chapters = graphState.chapters;
    if (!nodes || nodes.length === 0) return null;

    graphState.nodeSpatialGrid = NodeCanvasGeometry.buildSpatialGridForNodes(nodes);
    const byChapter = new Map();
    for (let idx = 0; idx < nodes.length; idx++) {
      const node = nodes[idx];
      const key = (node.chapter || UIController.tr(tok.defaultChapterKey)).trim().toLowerCase();
      let list = byChapter.get(key);
      if (!list) {
        list = [];
        byChapter.set(key, list);
      }
      list.push(node);
    }
    byChapter.forEach((memberNodes, key) => {
      if (!graphState.chapterByKey.has(key)) {
        const displayCh = memberNodes[0].chapter || UIController.tr(tok.defaultChapterKey);
        const chapter = this.createChapterRecord({
          rawId: key,
          ref: displayCh,
          refLower: key,
          name: displayCh,
          nameLower: key,
          isRoot: chapters.length === 0,
          startSceneId: memberNodes[0].id
        });
        chapters.push(chapter);
        graphState.chapterByKey.set(key, chapter);
      }
    });
    for (let idx = 0; idx < chapters.length; idx++) {
      const chapter = chapters[idx];
      const matching = byChapter.get(chapter.nameLower) || byChapter.get(chapter.refLower) || [];
      chapter.nodes = matching;
      NodeCanvasGeometry.buildSpatialGridForNodes(matching);
      if (matching.length === 0) {
        chapter.hidden = true;
        continue;
      }
      chapter.hidden = false;
      for (let nodeIdx = 0; nodeIdx < matching.length; nodeIdx++) {
        matching[nodeIdx].chapterObj = chapter;
      }
      NodeCanvasGeometry.copyRect(chapter, this.computeChapterFrameRect(matching));
      this.recomputeChapterInputPortPosition(chapter);
    }
    for (let idx = 0; idx < chapters.length; idx++) {
      if (!chapters[idx].hidden) this.recomputeChapterPortPositions(chapters[idx]);
    }
    graphState.staticBatchDirty = true;
    return null;
  },

  updateChapterGroupFrames(onlyChaptersSet, hasInterChapterPortMove) {
    const graphState = NodeCanvasLayout.studioGraphState;
    if (!graphState.nodes || graphState.nodes.length === 0) return null;
    if (onlyChaptersSet && onlyChaptersSet.size > 0) {
      return this.updateChaptersIncremental(onlyChaptersSet, hasInterChapterPortMove);
    }
    return this.rebuildChapterFrames();
  },

  parsePipeKVLine(rawLine) {
    const parts = rawLine.split(this.tokens.pipeSep);
    const fields = {};
    for (let idx = 1; idx < parts.length; idx++) {
      const seg = parts[idx];
      const colonIdx = seg.indexOf(":");
      if (colonIdx > 0) fields[seg.slice(0, colonIdx).trim()] = seg.slice(colonIdx + 1).trim();
    }
    return { kind: (parts[0] || "").trim(), kv: fields };
  },

  applyCanvasLayout(rawLayout, resetCanvas) {
    const tok = this.tokens;
    const graphState = NodeCanvasLayout.studioGraphState;
    if (resetCanvas) NodeCanvasController.data.dirty = false;
    if (NodeCanvasController.data.dirty && !resetCanvas) return;
    graphState.posById.clear();
    graphState.wirePins.clear();
    const lines = String(rawLayout || "").split("\n");
    for (let idx = 0; idx < lines.length; idx++) {
      const line = lines[idx].trim();
      if (!line) continue;
      const { kind, kv } = this.parsePipeKVLine(line);
      if (kind === tok.recPos && kv.id) {
        graphState.posById.set(kv.id, {
          x: parseInt(kv.x || tok.zeroStr, 10) || 0,
          y: parseInt(kv.y || tok.zeroStr, 10) || 0
        });
      } else if ((kind === tok.recPin || kind === tok.recWirePins) && kv.key) {
        const pinsRaw = (kv.pts || kv.pins || "").trim();
        if (!pinsRaw) continue;
        const pins = [];
        const segs = pinsRaw.split(";");
        for (let segIdx = 0; segIdx < segs.length; segIdx++) {
          const coords = segs[segIdx].split(",");
          if (coords.length === 2) {
            const pinX = parseInt(coords[0].trim(), 10);
            const pinY = parseInt(coords[1].trim(), 10);
            if (!isNaN(pinX) && !isNaN(pinY)) pins.push({ x: pinX, y: pinY });
          }
        }
        if (pins.length > 0) graphState.wirePins.set(kv.key.trim().toLowerCase(), pins);
      }
    }
  },

  applyBookGraph(rawText) {
    const tok = this.tokens;
    const card = NodeCanvasStyle.card;
    const pad = NodeCanvasStyle.pad;
    const gap = NodeCanvasStyle.gap;
    const graphState = NodeCanvasLayout.studioGraphState;
    for (let idx = 0; idx < graphState.nodes.length; idx++) {
      const existing = graphState.nodes[idx];
      if (!graphState.posById.has(existing.id)) {
        graphState.posById.set(existing.id, { x: existing.x, y: existing.y });
      }
    }
    graphState.chapters = [];
    graphState.chapterByKey.clear();
    graphState.nodes = [];
    graphState.nodeById.clear();
    graphState.wires = [];
    graphState.hoveredItem = null;
    graphState.staticBatchDirty = true;
    graphState.overlayBatchDirty = true;
    if (!rawText) return;

    const lines = String(rawText).split("\n");
    for (let idx = 0; idx < lines.length; idx++) {
      const line = lines[idx].trim();
      if (!line) continue;
      const { kind, kv } = this.parsePipeKVLine(line);
      if (kind === tok.recChapter) {
        const outRefs = kv.outs ? kv.outs.split(",").map((part) => part.trim()).filter(Boolean) : [];
        const outPorts = outRefs.length > 0
          ? outRefs.map((ref, outIdx) => ({ idx: outIdx, target: ref, targetLower: ref.toLowerCase(), x: 0, y: 0 }))
          : [{ idx: 0, target: "", targetLower: "", x: 0, y: 0 }];
        const chName = kv.name || kv.ref || UIController.tr(tok.defaultChapterKey);
        const chRef = kv.ref || chName;
        const chapter = this.createChapterRecord({
          rawId: kv.id || chRef,
          ref: chRef,
          refLower: chRef.toLowerCase(),
          name: chName,
          nameLower: chName.toLowerCase(),
          isRoot: kv.root === tok.bitTrue,
          startSceneId: kv.start || "",
          outPorts: outPorts
        });
        graphState.chapters.push(chapter);
        if (chapter.nameLower) graphState.chapterByKey.set(chapter.nameLower, chapter);
        if (chapter.refLower) graphState.chapterByKey.set(chapter.refLower, chapter);
      } else if (kind === tok.recNode && kv.id) {
        const nodeId = kv.id;
        let pos = graphState.posById.get(nodeId);
        if (!pos) {
          pos = kv.x !== undefined
            ? { x: parseInt(kv.x, 10) || 0, y: parseInt(kv.y, 10) || 0 }
            : { x: card.w + graphState.nodes.length * (card.w + gap.node), y: pad.top };
          graphState.posById.set(nodeId, pos);
        }
        const chName = kv.chapter || UIController.tr(tok.defaultChapterKey);
        const node = {
          id: nodeId,
          name: kv.name || nodeId,
          chapter: chName,
          chapterLower: chName.toLowerCase(),
          x: pos.x,
          y: pos.y,
          selected: kv.selected === tok.bitTrue,
          isStart: kv.start === tok.bitTrue,
          isEntry: kv.entry === tok.bitTrue,
          unreachable: kv.unreachable === tok.bitTrue,
          hasChoices: kv.has_choices === tok.bitTrue,
          implicitTarget: kv.imp_target || "",
          implicitChapter: kv.imp_chapter || "",
          branchRole: kv.branch_role || "",
          branchParentId: kv.branch_parent || "",
          ports: []
        };
        this.recomputeNodeMetrics(node);
        graphState.nodes.push(node);
        graphState.nodeById.set(nodeId, node);
      } else if (kind === tok.recPort) {
        const node = graphState.nodeById.get(kv.scene || "");
        if (!node) continue;
        const portIdx = parseInt(kv.idx || String(node.ports.length), 10) || 0;
        node.ports.push({
          idx: portIdx,
          label: kv.label || UIController.tr(tok.choicePrefixKey) + (portIdx + 1),
          targetId: kv.target_id || "",
          targetChapter: kv.target_chapter || "",
          isConditional: kv.cond === tok.bitTrue,
          isBottom: false,
          x: node.x + card.w,
          y: node.y + this.choiceRowY(portIdx) + card.choiceH * 0.5
        });
        node.hasChoices = node.ports.length >= 2;
        this.recomputeNodeMetrics(node);
      }
    }
    this.rebuildChapterFrames();
    if (typeof NodeCanvasHelpers !== tok.undefType && NodeCanvasHelpers.selectedNodeIds) {
      for (const selId of Array.from(NodeCanvasHelpers.selectedNodeIds)) {
        if (!graphState.nodeById.has(selId)) NodeCanvasHelpers.selectedNodeIds.delete(selId);
      }
    }
  },

  applyIncrementalNodeMove(items, snappedDelta) {
    const tok = this.tokens;
    const deltaX = snappedDelta && typeof snappedDelta === tok.objType ? snappedDelta.x : snappedDelta;
    const deltaY = snappedDelta && typeof snappedDelta === tok.objType ? snappedDelta.y : arguments[2];
    const graphState = NodeCanvasLayout.studioGraphState;
    const affectedChaptersSet = new Set();
    let hasInterChapterPortMove = false;
    for (let idx = 0; idx < items.length; idx++) {
      const node = items[idx].node;
      const chapter = node.chapterObj || graphState.chapterByKey.get(node.chapterLower);
      if (chapter) affectedChaptersSet.add(chapter);
      if (!hasInterChapterPortMove && ((node.implicitPort && node.implicitPort.targetChapter) || (node.ports && node.ports.some((port) => port.targetChapter)))) {
        hasInterChapterPortMove = true;
      }
      node.x = NodeCanvasGeometry.snapToEditorGrid(items[idx].startX + deltaX);
      node.y = NodeCanvasGeometry.snapToEditorGrid(items[idx].startY + deltaY);
      graphState.posById.set(node.id, { x: node.x, y: node.y });
      this.recomputeNodeMetrics(node);
      NodeCanvasGeometry.updateNodeInSpatialGrids(node);
    }
    this.updateChaptersIncremental(affectedChaptersSet, hasInterChapterPortMove);
    NodeCanvasRouter.route();
    NodeCanvasRenderer.render();
  },

  renameNode(oldId, newId) {
    if (!oldId || !newId || oldId === newId) return;
    const graphState = NodeCanvasLayout.studioGraphState;
    if (graphState.posById.has(oldId)) {
      graphState.posById.set(newId, graphState.posById.get(oldId));
      graphState.posById.delete(oldId);
    }
    if (graphState.nodeById.has(oldId)) {
      const node = graphState.nodeById.get(oldId);
      node.id = newId;
      graphState.nodeById.delete(oldId);
      graphState.nodeById.set(newId, node);
    }
    if (typeof NodeCanvasHelpers !== this.tokens.undefType && NodeCanvasHelpers.selectedNodeIds && NodeCanvasHelpers.selectedNodeIds.has(oldId)) {
      NodeCanvasHelpers.selectedNodeIds.delete(oldId);
      NodeCanvasHelpers.selectedNodeIds.add(newId);
    }
  },

  moveNodePortInMemory(node, fromIdx, toIdx) {
    if (!node || !node.ports || fromIdx === toIdx || fromIdx < 0 || fromIdx >= node.ports.length || toIdx < 0 || toIdx >= node.ports.length) {
      return false;
    }
    const movedPort = node.ports.splice(fromIdx, 1)[0];
    node.ports.splice(toIdx, 0, movedPort);
    for (let idx = 0; idx < node.ports.length; idx++) {
      node.ports[idx].idx = idx;
    }
    this.recomputeNodeMetrics(node);
    const chapter = node.chapterObj || NodeCanvasLayout.studioGraphState.chapterByKey.get(node.chapterLower);
    if (chapter) this.recomputeChapterPortPositions(chapter);
    NodeCanvasRouter.route();
    NodeCanvasLayout.studioGraphState.staticBatchDirty = true;
    NodeCanvasLayout.studioGraphState.overlayBatchDirty = true;
    NodeCanvasRenderer.render();
    return true;
  }
};
