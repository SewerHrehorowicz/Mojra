"use strict";

// NodeCanvasHelpers: Interactive session state, layered hit-testing,
// Rect-based collision/drag ghost evaluation, and floating DOM input positioning.
const NodeCanvasHelpers = {
  selectedNodeIds: new Set(),
  activeCanvasGesture: null,
  pendingContextMenuWorldPos: { x: 120, y: 120 },

  hitTypes: {
    sceneOutPort: "scene-out-port",
    chapterOutPort: "chapter-out-port",
    wirePin: "wire-pin",
    editSceneBtn: "edit-scene-btn",
    nodeCard: "node-card",
    deleteSceneBtn: "delete-scene-btn",
    addChoiceBtn: "add-choice-btn",
    nodeGrip: "node-grip",
    branchRadio: "branch-radio",
    nodeTitle: "node-title",
    choiceGrip: "choice-grip",
    choiceTrashBtn: "choice-trash-btn",
    choiceLabel: "choice-label",
    chapterGrip: "chapter-grip",
    chapterTitle: "chapter-title",
    wire: "wire",
    empty: "empty"
  },

  gestures: {
    pan: "pan",
    touchPan: "touch-pan",
    rightPan: "right-pan",
    pinch: "pinch",
    choiceDrag: "choice-drag",
    wirePinDrag: "wire-pin-drag",
    wireCreatePin: "wire-create-pin",
    wireConnect: "wire-connect",
    chapterWireConnect: "chapter-wire-connect",
    chapterDrag: "chapter-drag",
    nodeDrag: "node-drag",
    nodeClick: "node-click",
    marqueeSelect: "marquee-select"
  },

  tokens: {
    addBtn: "add",
    deleteBtn: "delete",
    editBtn: "edit",
    rolePrimary: "primary",
    roleSecondary: "secondary",
    axisH: "H",
    axisV: "V",
    undefType: "undefined",
    objType: "object",
    funcType: "function",
    chapterInput: "#canvas-chapter-input",
    nodeTitleInput: "#canvas-node-title-input",
    choiceInput: "#canvas-choice-input",
    hiddenClass: "hidden",
    pxSuffix: "px"
  },

  choiceRowY(idx) {
    return NodeCanvasStyle.card.headerH + idx * NodeCanvasStyle.card.choiceH;
  },

  isHistoryPreview() {
    const tok = this.tokens;
    return Boolean(typeof NodeCanvasController !== tok.undefType && NodeCanvasController.data && NodeCanvasController.data.isHistoryPreview);
  },

  getNodeFooterBtnX(node, kind, isPreview) {
    const card = NodeCanvasStyle.card;
    const tok = this.tokens;
    if (kind === tok.addBtn) return node.x + card.pad;
    if (kind === tok.deleteBtn) return node.x + node.w - card.pad - card.btnSize;
    const offset = isPreview ? card.btnSize : card.btnSize * 2 + 6;
    return node.x + node.w - card.pad - offset;
  },

  clientToStudioWorld(viewport, clientX, clientY) {
    const rect = viewport.getBoundingClientRect();
    const state = NodeCanvasController.data;
    const zoom = state.zoom || 1.0;
    const viewX = clientX - rect.left;
    const viewY = clientY - rect.top;
    return {
      vx: viewX,
      vy: viewY,
      wx: (viewX - state.panX) / zoom,
      wy: (viewY - state.panY) / zoom
    };
  },

  buildPointerContext(viewport, event, isTouch) {
    const ptWorld = this.clientToStudioWorld(viewport, event.clientX, event.clientY);
    return {
      viewport: viewport,
      clientX: event.clientX,
      clientY: event.clientY,
      wx: ptWorld.wx,
      wy: ptWorld.wy,
      isTouch: Boolean(isTouch),
      isMultiModifier: Boolean(!isTouch && (event.shiftKey || event.metaKey || event.ctrlKey))
    };
  },

  hitTestPortsAndPins(worldPoint) {
    const graphState = NodeCanvasLayout.studioGraphState;
    const hit = NodeCanvasStyle.hit;
    const hitTypes = this.hitTypes;
    const nodes = graphState.nodes;
    const chapters = graphState.chapters;

    for (let nodeIdx = nodes.length - 1; nodeIdx >= 0; nodeIdx--) {
      const node = nodes[nodeIdx];
      const portList = NodeCanvasModel.getActiveNodePorts(node);
      for (let portIdx = 0; portIdx < portList.length; portIdx++) {
        const port = portList[portIdx];
        if (NodeCanvasGeometry.distSq(worldPoint, port) <= hit.portRadiusSq) {
          return {
            type: hitTypes.sceneOutPort,
            node: node,
            port: port,
            portIdx: port.isBottom ? 0 : portIdx,
            isBottom: Boolean(port.isBottom),
            x: port.x,
            y: port.y
          };
        }
      }
    }

    for (let chIdx = chapters.length - 1; chIdx >= 0; chIdx--) {
      const chapter = chapters[chIdx];
      if (chapter.hidden) continue;
      for (let outIdx = 0; outIdx < chapter.outPorts.length; outIdx++) {
        const outPort = chapter.outPorts[outIdx];
        if (NodeCanvasGeometry.distSq(worldPoint, outPort) <= hit.portRadiusSq) {
          return {
            type: hitTypes.chapterOutPort,
            chapter: chapter,
            outPort: outPort,
            portIdx: outIdx,
            x: outPort.x,
            y: outPort.y
          };
        }
      }
    }

    const allWires = graphState.wires || [];
    for (let wireIdx = allWires.length - 1; wireIdx >= 0; wireIdx--) {
      const wire = allWires[wireIdx];
      if (!wire.pins || wire.pins.length === 0) continue;
      for (let pinIdx = wire.pins.length - 1; pinIdx >= 0; pinIdx--) {
        const pin = wire.pins[pinIdx];
        const bar = NodeCanvasGeometry.computeWirePinSegmentBar(wire, pin);
        if (bar) {
          if (NodeCanvasGeometry.projectPointToSegment(worldPoint, bar.start, bar.end).distSq <= hit.pinBarDistSq) {
            return {
              type: hitTypes.wirePin,
              wire: wire,
              pinIdx: pinIdx,
              pin: pin,
              segIdx: bar.segIdx,
              x: bar.center.x,
              y: bar.center.y
            };
          }
        } else if (NodeCanvasGeometry.distSq(worldPoint, pin) <= hit.portRadiusSq) {
          return {
            type: hitTypes.wirePin,
            wire: wire,
            pinIdx: pinIdx,
            pin: pin,
            x: pin.x,
            y: pin.y
          };
        }
      }
    }
    return null;
  },

  hitTestNodeCard(node, worldPoint, isPreview) {
    const card = NodeCanvasStyle.card;
    const hit = NodeCanvasStyle.hit;
    const tok = this.tokens;
    const hitTypes = this.hitTypes;
    const localX = worldPoint.x - node.x;
    const localY = worldPoint.y - node.y;
    const footerBtnTop = node.h - card.footerH + card.pad;

    if (localY >= footerBtnTop && localY <= footerBtnTop + card.btnSize) {
      const editLeft = this.getNodeFooterBtnX(node, tok.editBtn, isPreview) - node.x;
      if (localX >= editLeft && localX <= editLeft + card.btnSize) {
        return { type: hitTypes.editSceneBtn, node: node };
      }
      if (!isPreview) {
        const deleteLeft = this.getNodeFooterBtnX(node, tok.deleteBtn, false) - node.x;
        if (localX >= deleteLeft && localX <= deleteLeft + card.btnSize) {
          return { type: hitTypes.deleteSceneBtn, node: node };
        }
        const addLeft = this.getNodeFooterBtnX(node, tok.addBtn, false) - node.x;
        if (localX >= addLeft && localX <= addLeft + card.btnSize) {
          return { type: hitTypes.addChoiceBtn, node: node };
        }
      }
    }
    if (isPreview) return { type: hitTypes.nodeCard, node: node };

    if (localX >= 2 && localX <= 2 + card.gripW && localY >= 4 && localY <= 4 + hit.chapterGripW) {
      return { type: hitTypes.nodeGrip, node: node };
    }
    if (
      (node.branchRole === tok.rolePrimary || node.branchRole === tok.roleSecondary) &&
      worldPoint.x >= node.branchRadioX - 4 &&
      worldPoint.x <= node.branchRadioX - 4 + card.radioReserveW &&
      worldPoint.y >= node.branchRadioY - 4 &&
      worldPoint.y <= node.branchRadioY - 4 + card.radioReserveW
    ) {
      return { type: hitTypes.branchRadio, node: node };
    }
    if (localX > hit.chapterGripW && localX <= node.w - card.pad && localY >= 4 && localY < card.headerH) {
      return { type: hitTypes.nodeTitle, node: node };
    }
    const footerTop = node.h - card.footerH;
    if (node.hasChoices && node.ports && node.ports.length > 0 && localY >= card.headerH && localY < footerTop) {
      const choiceIdx = Math.floor((localY - card.headerH) / card.choiceH);
      if (choiceIdx >= 0 && choiceIdx < node.ports.length) {
        if (localX >= 2 && localX <= hit.choiceGripRight) return { type: hitTypes.choiceGrip, node: node, choiceIdx: choiceIdx };
        if (localX >= node.w - hit.choiceTrashOffset && localX <= node.w - card.pad) return { type: hitTypes.choiceTrashBtn, node: node, choiceIdx: choiceIdx };
        if (localX > hit.choiceGripRight && localX < node.w - hit.choiceTrashOffset) return { type: hitTypes.choiceLabel, node: node, choiceIdx: choiceIdx };
      }
    }
    return { type: hitTypes.nodeCard, node: node };
  },

  hitTestChaptersAndWires(worldPoint, isPreview) {
    const graphState = NodeCanvasLayout.studioGraphState;
    const hit = NodeCanvasStyle.hit;
    const hitTypes = this.hitTypes;
    const chapters = graphState.chapters;

    for (let chIdx = chapters.length - 1; chIdx >= 0; chIdx--) {
      const chapter = chapters[chIdx];
      if (!chapter.hidden && NodeCanvasGeometry.inRect(worldPoint, { x: chapter.x, y: chapter.y, w: chapter.w, h: hit.chapterHeaderH })) {
        return !isPreview && worldPoint.x <= chapter.x + hit.chapterGripW
          ? { type: hitTypes.chapterGrip, chapter: chapter }
          : { type: hitTypes.chapterTitle, chapter: chapter };
      }
    }

    const wires = graphState.wires || [];
    const pad = hit.wireBoundsPad;
    for (let wireIdx = wires.length - 1; wireIdx >= 0; wireIdx--) {
      const wire = wires[wireIdx];
      if (wire.maxX !== undefined && (worldPoint.x < wire.minX - pad || worldPoint.x > wire.maxX + pad || worldPoint.y < wire.minY - pad || worldPoint.y > wire.maxY + pad)) {
        continue;
      }
      for (let segIdx = 0; segIdx < wire.pts.length - 1; segIdx++) {
        if (NodeCanvasGeometry.projectPointToSegment(worldPoint, wire.pts[segIdx], wire.pts[segIdx + 1]).distSq <= hit.wireDistSq) {
          return { type: hitTypes.wire, wire: wire, segIdx: segIdx, wx: worldPoint.x, wy: worldPoint.y };
        }
      }
    }
    return { type: hitTypes.empty };
  },

  hitTest(worldX, worldY) {
    const worldPoint = worldX && typeof worldX === this.tokens.objType ? worldX : { x: worldX, y: worldY };
    const isPreview = this.isHistoryPreview();
    if (!isPreview) {
      const portOrPinHit = this.hitTestPortsAndPins(worldPoint);
      if (portOrPinHit) return portOrPinHit;
    }
    const nodes = NodeCanvasLayout.studioGraphState.nodes;
    for (let idx = nodes.length - 1; idx >= 0; idx--) {
      const node = nodes[idx];
      if (NodeCanvasGeometry.inRect(worldPoint, node)) {
        return this.hitTestNodeCard(node, worldPoint, isPreview);
      }
    }
    return this.hitTestChaptersAndWires(worldPoint, isPreview);
  },

  isSameHoveredItem(firstItem, secondItem) {
    if (firstItem === secondItem) return true;
    if (!firstItem || !secondItem || firstItem.type !== secondItem.type) return false;
    return (
      firstItem.node === secondItem.node &&
      firstItem.chapter === secondItem.chapter &&
      firstItem.portIdx === secondItem.portIdx &&
      firstItem.choiceIdx === secondItem.choiceIdx &&
      firstItem.pinIdx === secondItem.pinIdx &&
      firstItem.wire === secondItem.wire
    );
  },

  findClosestChapterAtWorldCoords(worldPoint) {
    const rawX = worldPoint && typeof worldPoint === this.tokens.objType ? worldPoint.x : worldPoint;
    const rawY = worldPoint && typeof worldPoint === this.tokens.objType ? worldPoint.y : arguments[1];
    const probeX = rawX + NodeCanvasStyle.card.halfW;
    const probeY = rawY + NodeCanvasStyle.gap.node;
    const chapters = NodeCanvasLayout.studioGraphState.chapters;
    let bestChapter = "";
    let bestDistSq = Infinity;
    for (let idx = 0; idx < chapters.length; idx++) {
      const chapter = chapters[idx];
      if (chapter.hidden || !chapter.name) continue;
      const distX = probeX < chapter.x ? chapter.x - probeX : probeX > chapter.x + chapter.w ? probeX - (chapter.x + chapter.w) : 0;
      const distY = probeY < chapter.y ? chapter.y - probeY : probeY > chapter.y + chapter.h ? probeY - (chapter.y + chapter.h) : 0;
      const distSq = distX * distX + distY * distY;
      if (distSq < bestDistSq) {
        bestDistSq = distSq;
        bestChapter = chapter.name;
      }
    }
    return bestChapter;
  },

  resolveNonOverlappingNodeCoords(basePoint, chapterName) {
    const isVec = Boolean(basePoint && typeof basePoint === this.tokens.objType);
    const baseX = isVec ? basePoint.x : basePoint;
    const baseY = isVec ? basePoint.y : arguments[1];
    const rawChapterName = isVec ? chapterName : arguments[2];
    const card = NodeCanvasStyle.card;
    const gap = NodeCanvasStyle.gap;
    const pad = NodeCanvasStyle.pad;
    const gridSize = NodeCanvasStyle.grid.size;
    const nodeW = card.w;
    const nodeH = card.baseH;
    const graphState = NodeCanvasLayout.studioGraphState;
    const nodes = graphState.nodes || [];
    const chapters = graphState.chapters || [];
    const chKey = (rawChapterName || "").trim().toLowerCase();
    const rawTargetCh = chKey ? graphState.chapterByKey.get(chKey) : null;
    const targetCh = rawTargetCh && !rawTargetCh.hidden ? rawTargetCh : null;
    const candRect = { x: 0, y: 0, w: nodeW, h: nodeH, left: 0, top: 0, right: 0, bottom: 0 };
    const chFrameRect = { x: 0, y: 0, w: 0, h: 0, left: 0, top: 0, right: 0, bottom: 0 };

    const collidesAt = (candX, candY, checkChapterUnion) => {
      candRect.x = candX;
      candRect.y = candY;
      NodeCanvasGeometry.updateRect(candRect);
      for (let idx = 0; idx < nodes.length; idx++) {
        if (NodeCanvasGeometry.rectsOverlap(candRect, nodes[idx], gap.node)) return true;
      }
      chFrameRect.left = candX - pad.x;
      chFrameRect.top = candY - pad.top;
      chFrameRect.right = candX + nodeW + pad.x;
      chFrameRect.bottom = candY + Math.max(nodeH, card.minChapterH) + pad.bottom;
      if (checkChapterUnion && targetCh) {
        chFrameRect.left = Math.min(chFrameRect.left, targetCh.left);
        chFrameRect.top = Math.min(chFrameRect.top, targetCh.top);
        chFrameRect.right = Math.max(chFrameRect.right, targetCh.right);
        chFrameRect.bottom = Math.max(chFrameRect.bottom, targetCh.bottom);
      }
      for (let idx = 0; idx < chapters.length; idx++) {
        const chapter = chapters[idx];
        if (chapter.hidden || chapter === targetCh || chapter.nameLower === chKey || chapter.refLower === chKey) continue;
        if (NodeCanvasGeometry.rectsOverlap(chFrameRect, chapter, gap.chapter)) return true;
      }
      return false;
    };

    if (!collidesAt(baseX, baseY, true)) return { x: baseX, y: baseY };
    const unionPasses = [true, false];
    for (let passIdx = 0; passIdx < unionPasses.length; passIdx++) {
      for (let step = 1; step <= 48; step++) {
        const stepPx = step * gridSize;
        const candidates = [
          [baseX, baseY + stepPx],
          [baseX + stepPx, baseY],
          [baseX - stepPx, baseY],
          [baseX, baseY - stepPx],
          [baseX + stepPx, baseY + stepPx],
          [baseX - stepPx, baseY + stepPx]
        ];
        for (let candIdx = 0; candIdx < candidates.length; candIdx++) {
          const candX = NodeCanvasGeometry.snapToEditorGrid(candidates[candIdx][0]);
          const candY = NodeCanvasGeometry.snapToEditorGrid(candidates[candIdx][1]);
          if (!collidesAt(candX, candY, unionPasses[passIdx])) return { x: candX, y: candY };
        }
      }
    }
    return { x: baseX, y: baseY };
  },

  resolveValidChapterDragDelta(dragGesture, desiredDelta) {
    const chapter = dragGesture.chapter;
    const items = dragGesture.items;
    const desiredDx = desiredDelta.x;
    const desiredDy = desiredDelta.y;
    const lastValidDx = dragGesture.lastValidDx || 0;
    const lastValidDy = dragGesture.lastValidDy || 0;
    const chapterGap = NodeCanvasStyle.gap.chapter;
    const chapters = NodeCanvasLayout.studioGraphState.chapters || [];
    if (chapters.length <= 1 || !items || items.length === 0) return { dx: desiredDx, dy: desiredDy };
    const baseRect = NodeCanvasModel.computeChapterFrameRect(items);
    const shiftedRect = { left: 0, top: 0, right: 0, bottom: 0 };
    const isDeltaValid = (testDx, testDy) => {
      shiftedRect.left = baseRect.left + testDx;
      shiftedRect.top = baseRect.top + testDy;
      shiftedRect.right = baseRect.right + testDx;
      shiftedRect.bottom = baseRect.bottom + testDy;
      for (let idx = 0; idx < chapters.length; idx++) {
        const other = chapters[idx];
        if (other !== chapter && !other.hidden && NodeCanvasGeometry.rectsOverlap(shiftedRect, other, chapterGap)) {
          return false;
        }
      }
      return true;
    };
    if (isDeltaValid(desiredDx, desiredDy)) return { dx: desiredDx, dy: desiredDy };
    if (isDeltaValid(desiredDx, lastValidDy)) return { dx: desiredDx, dy: lastValidDy };
    if (isDeltaValid(lastValidDx, desiredDy)) return { dx: lastValidDx, dy: desiredDy };
    return { dx: lastValidDx, dy: lastValidDy };
  },

  collectCollisionCells(boundsRect, seenCellKeys, collisionCells) {
    const gridSize = NodeCanvasStyle.grid.size;
    for (let cellY = boundsRect.top; cellY < boundsRect.bottom; cellY += gridSize) {
      for (let cellX = boundsRect.left; cellX < boundsRect.right; cellX += gridSize) {
        const key = NodeCanvasGeometry.packGridKey(cellX / gridSize, cellY / gridSize);
        if (!seenCellKeys.has(key)) {
          seenCellKeys.add(key);
          collisionCells.push({ x: cellX, y: cellY });
        }
      }
    }
  },

  evaluateNodeDragGhost(items, desiredDx, desiredDy) {
    const isVec = Boolean(desiredDx && typeof desiredDx === this.tokens.objType);
    const rawDx = isVec ? desiredDx.x : desiredDx;
    const rawDy = isVec ? desiredDx.y : desiredDy;
    const card = NodeCanvasStyle.card;
    const gap = NodeCanvasStyle.gap;
    const gridSize = NodeCanvasStyle.grid.size;
    const snappedDx = NodeCanvasGeometry.snapToEditorGrid(rawDx || 0);
    const snappedDy = NodeCanvasGeometry.snapToEditorGrid(rawDy || 0);
    const nodes = NodeCanvasLayout.studioGraphState.nodes || [];
    const chapters = NodeCanvasLayout.studioGraphState.chapters || [];
    if (!items || items.length === 0) {
      return { dx: snappedDx, dy: snappedDy, valid: true, rects: [], collisionCells: [] };
    }

    for (let idx = 0; idx < items.length; idx++) items[idx].node._isDragging = true;
    const rects = [];
    const collisionCells = [];
    const seenCellKeys = new Set();
    let hasCollision = false;

    for (let idx = 0; idx < items.length; idx++) {
      const item = items[idx];
      const node = item.node;
      const ghostX = NodeCanvasGeometry.snapToEditorGrid(item.startX + snappedDx);
      const ghostY = NodeCanvasGeometry.snapToEditorGrid(item.startY + snappedDy);
      const ghostW = node.w || card.w;
      const ghostH = Math.ceil((node.h || card.baseH) / gridSize) * gridSize;
      const dragRect = NodeCanvasGeometry.updateRect({
        node: node,
        x: ghostX,
        y: ghostY,
        w: ghostW,
        h: ghostH
      });
      rects.push(dragRect);
      for (let otherIdx = 0; otherIdx < nodes.length; otherIdx++) {
        const other = nodes[otherIdx];
        if (other._isDragging || other.hidden) continue;
        if (NodeCanvasGeometry.rectsOverlap(dragRect, other, gap.node)) {
          hasCollision = true;
          const forbLeft = other.left - gap.node;
          const forbRight = other.right + gap.node;
          const forbTop = other.top - gap.node;
          const forbBottom = other.bottom + gap.node;
          this.collectCollisionCells(
            {
              left: Math.max(dragRect.left, Math.floor(forbLeft / gridSize) * gridSize),
              right: Math.min(dragRect.right, Math.ceil(forbRight / gridSize) * gridSize),
              top: Math.max(dragRect.top, Math.floor(forbTop / gridSize) * gridSize),
              bottom: Math.min(dragRect.bottom, Math.ceil(forbBottom / gridSize) * gridSize)
            },
            seenCellKeys,
            collisionCells
          );
        }
      }
    }

    for (let chIdx = 0; chIdx < chapters.length; chIdx++) {
      const chapter = chapters[chIdx];
      if (!chapter || chapter.hidden || !chapter.nodes || !chapter.nodes.some((member) => member._isDragging)) continue;
      const chRect = NodeCanvasModel.computeChapterFrameRect(chapter.nodes, rects);
      for (let otherIdx = 0; otherIdx < chapters.length; otherIdx++) {
        const otherCh = chapters[otherIdx];
        if (otherCh === chapter || otherCh.hidden) continue;
        if (NodeCanvasGeometry.rectsOverlap(chRect, otherCh, gap.chapter)) {
          hasCollision = true;
          for (let rectIdx = 0; rectIdx < rects.length; rectIdx++) {
            if (rects[rectIdx].node.chapterObj === chapter) {
              this.collectCollisionCells(rects[rectIdx], seenCellKeys, collisionCells);
            }
          }
        }
      }
    }

    for (let idx = 0; idx < items.length; idx++) items[idx].node._isDragging = false;
    return {
      dx: snappedDx,
      dy: snappedDy,
      valid: !hasCollision && collisionCells.length === 0,
      rects: rects,
      collisionCells: collisionCells
    };
  },

  applyBoxStyle(inputEl, rect, fontPx, letterSpacing) {
    const state = NodeCanvasController.data;
    const zoom = state.zoom || 1.0;
    const panX = state.panX || 0;
    const panY = state.panY || 0;
    const sfx = this.tokens.pxSuffix;
    inputEl.style.left = (rect.x * zoom + panX).toFixed(2) + sfx;
    inputEl.style.top = (rect.y * zoom + 0.5 + panY).toFixed(2) + sfx;
    inputEl.style.width = Math.max(24, rect.w * zoom).toFixed(2) + sfx;
    inputEl.style.height = Math.max(4, rect.h * zoom).toFixed(2) + sfx;
    inputEl.style.fontSize = (fontPx * zoom).toFixed(2) + sfx;
    if (letterSpacing !== undefined) inputEl.style.letterSpacing = (letterSpacing * zoom).toFixed(2) + sfx;
  },

  updateFloatingInputTransform() {
    const tok = this.tokens;
    const graphState = NodeCanvasLayout.studioGraphState;
    const chapterEl = document.querySelector(tok.chapterInput);
    const chapter = graphState.editingChapter;
    if (chapterEl && chapter && !chapterEl.classList.contains(tok.hiddenClass)) {
      this.applyBoxStyle(chapterEl, { x: chapter.x + 28, y: chapter.y + 4.703, w: chapter.w - 38, h: 20 }, 11, 0.65);
    }
    const titleEl = document.querySelector(tok.nodeTitleInput);
    const titleNode = graphState.editingNodeTitle;
    if (titleEl && titleNode && !titleEl.classList.contains(tok.hiddenClass)) {
      this.applyBoxStyle(titleEl, { x: titleNode.x + NodeCanvasStyle.card.titleLeft, y: titleNode.y + 6.715, w: titleNode.titleMaxW, h: 22 }, 13);
    }
    const choiceEl = document.querySelector(tok.choiceInput);
    const editChoice = graphState.editingChoice;
    if (choiceEl && editChoice && editChoice.node && !choiceEl.classList.contains(tok.hiddenClass)) {
      const card = NodeCanvasStyle.card;
      const node = editChoice.node;
      const rowY = node.y + card.headerH + editChoice.idx * card.choiceH;
      this.applyBoxStyle(choiceEl, { x: node.x + 21, y: rowY + 2.703, w: node.w - 47, h: card.choiceH - 6 }, 11);
    }
  },

  openFloatingInput(inputEl) {
    NodeCanvasLayout.studioGraphState.staticBatchDirty = true;
    inputEl.classList.remove(this.tokens.hiddenClass);
    this.updateFloatingInputTransform();
    inputEl.focus();
    if (typeof inputEl.select === this.tokens.funcType) inputEl.select();
    NodeCanvasRenderer.render();
  }
};
