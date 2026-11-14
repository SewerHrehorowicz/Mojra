"use strict";

const CanvasDrag = {
  moduleName: "CanvasDrag",

  actions: {
    saveEditorState: "SaveEditorState",
    selectChapter: "SelectStudioChapter",
    selectScene: "SelectStudioScene",
    reorderChoice: "ReorderChoice",
    connectChoice: "ConnectChoice",
    connectChapter: "ConnectChapter"
  },

  gestures: {
    pan: "pan",
    touchPan: "touch-pan",
    rightPan: "right-pan",
    choiceDrag: "choice-drag",
    wirePinDrag: "wire-pin-drag",
    nodeDrag: "node-drag",
    chapterDrag: "chapter-drag",
    marqueeSelect: "marquee-select",
    wireConnect: "wire-connect",
    chapterWireConnect: "chapter-wire-connect"
  },

  hits: {
    chapterOutPort: "chapter-out-port",
    sceneOutPort: "scene-out-port",
    chapterGrip: "chapter-grip",
    chapterTitle: "chapter-title",
    nodeGrip: "node-grip",
    choiceGrip: "choice-grip",
    wirePin: "wire-pin",
    wire: "wire"
  },

  selectors: {
    viewport: "#timeline-scroll-viewport"
  },

  classes: {
    panning: "is-panning"
  },

  targets: {
    end: "END",
    chapterPrefix: "chapter:",
    firstChoiceIdx: "0",
    bottomSide: "bottom"
  },

  axis: {
    horizontal: "H",
    vertical: "V"
  },

  metrics: {
    viewportSaveDelayMs: 240,
    zoomDecimals: 3,
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
    chapterHitPadBottom: 6
  },

  scheduleViewportSave() {
    if (CanvasModel.viewportSaveTimer) clearTimeout(CanvasModel.viewportSaveTimer);
    CanvasModel.viewportSaveTimer = setTimeout(() => {
      CanvasModel.viewportSaveTimer = null;
      const state = CanvasModel.editorCanvasState;
      Dispatcher.post(this.actions.saveEditorState, {
        pan_x: Math.round(state.panX || 0),
        pan_y: Math.round(state.panY || 0),
        zoom: (state.zoom || this.metrics.defaultZoom).toFixed(this.metrics.zoomDecimals)
      });
    }, this.metrics.viewportSaveDelayMs);
  },

  saveBatchNodePositions(items) {
    if (!items || !items.length) return;
    const state = CanvasModel.editorCanvasState;
    const posStr = items.map((it) => it.sceneId + ":" + Math.round(it.x) + "," + Math.round(it.y)).join(";");
    Dispatcher.post(this.actions.saveEditorState, {
      pan_x: Math.round(state.panX || 0),
      pan_y: Math.round(state.panY || 0),
      zoom: (state.zoom || this.metrics.defaultZoom).toFixed(this.metrics.zoomDecimals),
      positions: posStr
    });
  },

  saveWirePins(wireKey, pins) {
    if (!wireKey) return;
    const pinsStr = (pins || []).map((p) => Math.round(p.x) + "," + Math.round(p.y)).join(";");
    Dispatcher.post(this.actions.saveEditorState, { wire_key: wireKey, wire_pins: pinsStr });
  },

  removeWirePinAt(wire, pinIdx) {
    if (!wire || !wire.pins || pinIdx < 0 || pinIdx >= wire.pins.length) return false;
    wire.pins.splice(pinIdx, 1);
    CanvasModel.rerouteAfterWirePinMutation(wire);
    if (wire.wireKey) this.saveWirePins(wire.wireKey, wire.pins);
    return true;
  },

  beginChoiceDragGesture(nd, choiceIdx, ptrCtx) {
    if (!nd || !nd.ports || nd.ports.length < 2 || CanvasModel.isHistoryPreview()) return false;
    CanvasModel.editorSelectionCleared = false;
    CanvasModel.selectedNodeIds.clear();
    if (nd.id) CanvasModel.selectedNodeIds.add(nd.id);
    if (ptrCtx.viewport) ptrCtx.viewport.classList.add(this.classes.panning);
    CanvasModel.activeCanvasGesture = {
      type: this.gestures.choiceDrag, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
      node: nd, sceneId: nd.id, startIdx: choiceIdx, currentIdx: choiceIdx,
      startClientX: ptrCtx.clientX, startClientY: ptrCtx.clientY, moved: false
    };
    CanvasModel.studioGraphState.overlayBatchDirty = true;
    EditorWebglScene.render();
    return true;
  },

  beginWirePinDragGesture(hit, ptrCtx) {
    if (CanvasModel.isHistoryPreview() || !hit.wire) return false;
    const wire = hit.wire;
    if (!wire.pins) wire.pins = [];
    const pts = wire.pts && wire.pts.length >= 2 ? wire.pts : wire.basePts || [];
    let segIdx = Number.isFinite(hit.segIdx) ? hit.segIdx : 0;
    if (hit.type === this.hits.wirePin && hit.pin && Number.isFinite(hit.pin.segIdx)) segIdx = hit.pin.segIdx;
    if (segIdx < 0 || segIdx >= pts.length - 1) segIdx = Math.max(0, pts.length - 2);
    const p0 = pts[segIdx] || { x: wire.x1, y: wire.y1 };
    const p1 = pts[segIdx + 1] || { x: wire.x2, y: wire.y2 };
    const segAxis = Math.abs(p0.y - p1.y) <= Math.abs(p0.x - p1.x) ? this.axis.horizontal : this.axis.vertical;
    let existingIdx = hit.type === this.hits.wirePin && Number.isFinite(hit.pinIdx) ? hit.pinIdx : -1;
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
      const newArc = CanvasModel.computePointArcLengthAlongPolyline(pts, startWx, startWy);
      pinIdx = wire.pins.length;
      for (let i = 0; i < wire.pins.length; i++) {
        if (newArc <= CanvasModel.computePointArcLengthAlongPolyline(pts, wire.pins[i].x, wire.pins[i].y)) {
          pinIdx = i;
          break;
        }
      }
    }
    CanvasModel.activeCanvasGesture = {
      type: this.gestures.wirePinDrag, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
      wire: wire, pinIdx: pinIdx, segIdx: segIdx, axis: segAxis,
      fixedX: Math.round((p0.x + p1.x) * this.metrics.half), fixedY: Math.round((p0.y + p1.y) * this.metrics.half),
      isNewPin: isNewPin, inserted: !isNewPin,
      startClientX: ptrCtx.clientX, startClientY: ptrCtx.clientY, startWx: startWx, startWy: startWy, moved: false
    };
    CanvasModel.studioGraphState.overlayBatchDirty = true;
    EditorWebglScene.render();
    return true;
  },

  beginWireConnectGesture(hit, ptrCtx) {
    const nd = hit.node, pt = hit.port;
    CanvasModel.activeCanvasGesture = {
      type: this.gestures.wireConnect, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
      fromNode: nd, sourcePort: pt, fromSceneId: nd.id, fromChapter: nd.chapter,
      choiceIdx: String(hit.portIdx), isBottomPort: Boolean(hit.isBottom),
      hasExistingTarget: Boolean((pt.targetId && pt.targetId !== this.targets.end) || pt.targetChapter),
      startX: hit.x, startY: hit.y, curX: hit.x, curY: hit.y,
      hoveredTargetId: "", hoveredSceneNode: null, hoveredChapterIn: null, hoveredChapterOutPort: null
    };
    EditorWires.updateTempPreview();
    return true;
  },

  beginChapterWireConnectGesture(hit, ptrCtx) {
    const ch = hit.chapter, op = hit.outPort, opSide = op.side || this.targets.bottomSide;
    CanvasModel.activeCanvasGesture = {
      type: this.gestures.chapterWireConnect, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
      fromChapterObj: ch, activeChapterOutPort: op, outPortSide: opSide, isBottomPort: opSide === this.targets.bottomSide,
      fromChapter: ch.ref || ch.name, oldTargetChapter: op.target || "",
      startX: op.x, startY: op.y, curX: op.x, curY: op.y, hoveredTargetChapter: "", hoveredChapterIn: null
    };
    EditorWires.updateTempPreview();
    return true;
  },

  beginChapterDragGesture(ch, ptrCtx, clickedTitleArea) {
    if (!ch) return false;
    if (CanvasModel.isHistoryPreview()) {
      Dispatcher.post(this.actions.selectChapter, { chapter: ch.name });
      return true;
    }
    const chNodes = ch.nodes && ch.nodes.length > 0
      ? ch.nodes
      : CanvasModel.studioGraphState.nodes.filter((nd) => nd.chapterLower === ch.nameLower || nd.chapterLower === ch.refLower);
    if (chNodes.length === 0) return false;
    CanvasModel.activeCanvasGesture = {
      type: this.gestures.chapterDrag, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
      chapter: ch, chapterName: ch.name, clickedTitleArea: Boolean(clickedTitleArea), dragConfirmed: !clickedTitleArea,
      items: chNodes.map((nd) => ({ node: nd, sceneId: nd.id, startX: nd.x, startY: nd.y })),
      startClientX: ptrCtx.clientX, startClientY: ptrCtx.clientY, lastValidDx: 0, lastValidDy: 0, moved: false
    };
    EditorWebglScene.render();
    return true;
  },

  beginNodeDragGesture(nd, ptrCtx) {
    const sceneId = nd.id;
    const selected = CanvasModel.selectedNodeIds;
    if (ptrCtx.isMultiModifier) {
      if (selected.has(sceneId)) {
        selected.delete(sceneId);
        CanvasModel.editorSelectionCleared = selected.size === 0;
        EditorWebglScene.render();
        if (selected.size === 1) Dispatcher.post(this.actions.selectScene, { scene: Array.from(selected)[0] });
        return true;
      }
      if (sceneId) selected.add(sceneId);
      CanvasModel.editorSelectionCleared = false;
    } else if (!selected.has(sceneId)) {
      CanvasModel.editorSelectionCleared = false;
      selected.clear();
      if (sceneId) selected.add(sceneId);
    }
    if (CanvasModel.isHistoryPreview()) {
      EditorWebglScene.render();
      if (selected.size === 1) Dispatcher.post(this.actions.selectScene, { scene: Array.from(selected)[0] });
      return true;
    }
    const dragNodes = CanvasModel.studioGraphState.nodes.filter((n) => selected.has(n.id));
    if (dragNodes.length === 0) dragNodes.push(nd);
    CanvasModel.activeCanvasGesture = {
      type: this.gestures.nodeDrag, isTouch: ptrCtx.isTouch, viewport: ptrCtx.viewport,
      node: nd, sceneId: sceneId, isMultiModifier: ptrCtx.isMultiModifier,
      items: dragNodes.map((n) => ({ node: n, sceneId: n.id, startX: n.x, startY: n.y })),
      startClientX: ptrCtx.clientX, startClientY: ptrCtx.clientY,
      lastValidDx: 0, lastValidDy: 0, ghostDx: 0, ghostDy: 0, ghostState: null, moved: false
    };
    CanvasModel.studioGraphState.overlayBatchDirty = true;
    EditorWebglScene.render();
    return true;
  },

  beginSharedHitDragGesture(hit, ptrCtx) {
    if (hit.type === this.hits.chapterOutPort) return this.beginChapterWireConnectGesture(hit, ptrCtx);
    if (hit.type === this.hits.sceneOutPort) return this.beginWireConnectGesture(hit, ptrCtx);
    if (hit.type === this.hits.chapterGrip || hit.type === this.hits.chapterTitle) {
      return this.beginChapterDragGesture(hit.chapter, ptrCtx, hit.type === this.hits.chapterTitle);
    }
    if (hit.type === this.hits.nodeGrip) return this.beginNodeDragGesture(hit.node, ptrCtx);
    if (hit.type === this.hits.choiceGrip) return this.beginChoiceDragGesture(hit.node, hit.choiceIdx, ptrCtx);
    if (hit.type === this.hits.wirePin || hit.type === this.hits.wire) return this.beginWirePinDragGesture(hit, ptrCtx);
    return false;
  },

  updateMove(clientX, clientY) {
    const g = CanvasModel.activeCanvasGesture;
    if (!g) return;
    const state = CanvasModel.editorCanvasState;
    const gState = CanvasModel.studioGraphState;
    const m = this.metrics;
    const gridStep = CanvasModel.GAP.grid;

    if (g.type === this.gestures.pan) {
      state.panX = g.startPanX + (clientX - g.startClientX);
      state.panY = g.startPanY + (clientY - g.startClientY);
      StudioController.applyTransform();
      return;
    }
    if (g.type === this.gestures.touchPan || g.type === this.gestures.rightPan) {
      const dx = clientX - g.startClientX, dy = clientY - g.startClientY;
      if (!g.moved && Math.hypot(dx, dy) > m.panMoveMinPx) {
        g.moved = true;
        if (g.type === this.gestures.rightPan) state.suppressNextContextMenu = true;
        if (g.viewport) g.viewport.classList.add(this.classes.panning);
      }
      if (g.moved) {
        state.panX = g.startPanX + dx;
        state.panY = g.startPanY + dy;
        StudioController.applyTransform();
      }
      return;
    }
    if (g.type === this.gestures.choiceDrag) {
      const nd = g.node;
      if (!nd || !nd.ports || nd.ports.length < 2) return;
      const ptWorld = CanvasHittest.clientToStudioWorld(g.viewport, clientX, clientY);
      const targetIdx = Math.max(0, Math.min(nd.ports.length - 1, Math.floor((ptWorld.wy - (nd.y + CanvasModel.CARD.headerH)) / CanvasModel.CARD.choiceH)));
      if (targetIdx !== g.currentIdx && CanvasModel.moveNodePortInMemory(nd, g.currentIdx, targetIdx)) {
        g.currentIdx = targetIdx;
        g.moved = true;
      }
      return;
    }
    if (g.type === this.gestures.wirePinDrag) {
      if (!g.moved && Math.hypot(clientX - g.startClientX, clientY - g.startClientY) < m.pinMoveMinPx) return;
      const wire = g.wire;
      if (!wire) return;
      const ptWorld = CanvasHittest.clientToStudioWorld(g.viewport, clientX, clientY);
      const isHoriz = g.axis === this.axis.horizontal;
      const snappedX = isHoriz ? g.fixedX : CanvasModel.snapToWirePinGrid(ptWorld.wx);
      const snappedY = isHoriz ? CanvasModel.snapToWirePinGrid(ptWorld.wy) : g.fixedY;
      if (!g.inserted) {
        wire.pins.splice(g.pinIdx, 0, { x: snappedX, y: snappedY, axis: g.axis, segIdx: g.segIdx });
        g.inserted = true;
      } else if (wire.pins[g.pinIdx]) {
        const curPin = wire.pins[g.pinIdx];
        if (curPin.x === snappedX && curPin.y === snappedY && g.moved) return;
        curPin.x = snappedX;
        curPin.y = snappedY;
        curPin.axis = g.axis;
      }
      g.moved = true;
      CanvasModel.rerouteAfterWirePinMutation(wire);
      return;
    }
    if (g.type === this.gestures.nodeDrag) {
      const z = state.zoom || m.defaultZoom;
      const rawDx = CanvasModel.snapToEditorGrid((clientX - g.startClientX) / z);
      const rawDy = CanvasModel.snapToEditorGrid((clientY - g.startClientY) / z);
      if (rawDx === g.lastSnappedDx && rawDy === g.lastSnappedDy) return;
      g.lastSnappedDx = rawDx;
      g.lastSnappedDy = rawDy;
      g.ghostDx = rawDx;
      g.ghostDy = rawDy;
      g.ghostState = CanvasLayout.evaluateNodeDragGhost(g.items || [], rawDx, rawDy);
      if (Math.abs(rawDx) >= gridStep || Math.abs(rawDy) >= gridStep) g.moved = true;
      gState.overlayBatchDirty = true;
      EditorWebglScene.render();
      return;
    }
    if (g.type === this.gestures.chapterDrag) {
      if (!g.dragConfirmed) {
        if (Math.hypot(clientX - g.startClientX, clientY - g.startClientY) < m.chapterDragMinPx) return;
        g.dragConfirmed = true;
        CanvasOverlays.hideFloatingChapterInput(false);
      }
      const z = state.zoom || m.defaultZoom;
      const rawDx = CanvasModel.snapToEditorGrid((clientX - g.startClientX) / z);
      const rawDy = CanvasModel.snapToEditorGrid((clientY - g.startClientY) / z);
      if (rawDx === g.lastSnappedDx && rawDy === g.lastSnappedDy) return;
      g.lastSnappedDx = rawDx;
      g.lastSnappedDy = rawDy;
      const valid = CanvasLayout.resolveValidChapterDragDelta(g.chapter, g.items || [], rawDx, rawDy, g.lastValidDx || 0, g.lastValidDy || 0);
      if (valid.dx === g.lastValidDx && valid.dy === g.lastValidDy && g.moved) return;
      g.lastValidDx = valid.dx;
      g.lastValidDy = valid.dy;
      if (Math.abs(valid.dx) >= gridStep || Math.abs(valid.dy) >= gridStep) g.moved = true;
      CanvasLayout.applyIncrementalNodeMove(g.items || [], valid.dx, valid.dy);
      return;
    }
    if (g.type === this.gestures.marqueeSelect) {
      if (!g.moved && Math.hypot(clientX - g.startClientX, clientY - g.startClientY) > m.marqueeMoveMinPx) g.moved = true;
      if (!g.moved) return;
      const vpRect = g.viewport.getBoundingClientRect();
      const x1 = Math.min(g.startClientX, clientX) - vpRect.left;
      const y1 = Math.min(g.startClientY, clientY) - vpRect.top;
      const w = Math.abs(clientX - g.startClientX), h = Math.abs(clientY - g.startClientY);
      g.marqueeRect = { x: x1, y: y1, w: w, h: h };
      const z = state.zoom || m.defaultZoom, px = state.panX || 0, py = state.panY || 0;
      const worldLeft = (x1 - px) / z, worldRight = (x1 + w - px) / z, worldTop = (y1 - py) / z, worldBottom = (y1 + h - py) / z;
      CanvasModel.selectedNodeIds.clear();
      g.baseSelection.forEach((id) => CanvasModel.selectedNodeIds.add(id));
      for (let i = 0; i < gState.nodes.length; i++) {
        const nd = gState.nodes[i];
        if (nd.right >= worldLeft && nd.left <= worldRight && nd.bottom >= worldTop && nd.top <= worldBottom) {
          CanvasModel.selectedNodeIds.add(nd.id);
        }
      }
      gState.overlayBatchDirty = true;
      EditorWebglScene.render();
      return;
    }
    if (g.type === this.gestures.wireConnect) {
      const ptWorld = CanvasHittest.clientToStudioWorld(g.viewport, clientX, clientY);
      let cx = ptWorld.wx, cy = ptWorld.wy, hoveredId = "";
      const fromChLower = (g.fromChapter || "").trim().toLowerCase();
      for (let i = 0; i < gState.nodes.length; i++) {
        const nd = gState.nodes[i];
        if (nd.id === g.fromSceneId) continue;
        if (CanvasModel.inRect(cx, cy, nd.x - m.nodeHitPadX, nd.y - m.nodeHitPadTop, nd.w + m.nodeHitPadW, nd.h + m.nodeHitPadH)) {
          if (!fromChLower || !nd.chapterLower || nd.chapterLower === fromChLower) {
            hoveredId = nd.id; cx = nd.inX; cy = nd.inY;
          } else {
            const tf = gState.chapterByKey.get(nd.chapterLower);
            if (tf && !tf.hidden && !tf.isRoot) {
              hoveredId = this.targets.chapterPrefix + tf.ref; cx = tf.inX; cy = tf.inY;
            }
          }
          break;
        }
      }
      g.curX = cx; g.curY = cy; g.hoveredTargetId = hoveredId;
      EditorWires.updateTempPreview();
      return;
    }
    if (g.type === this.gestures.chapterWireConnect) {
      const ptWorld = CanvasHittest.clientToStudioWorld(g.viewport, clientX, clientY);
      let cx = ptWorld.wx, cy = ptWorld.wy, hoveredCh = "";
      const fromChLower = (g.fromChapter || "").trim().toLowerCase();
      for (let i = 0; i < gState.chapters.length; i++) {
        const fr = gState.chapters[i];
        if (fr.hidden || fr.isRoot || fr.nameLower === fromChLower || fr.refLower === fromChLower) continue;
        if (
          Math.hypot(cx - fr.inX, cy - fr.inY) <= m.chapterInSnapDist ||
          (cx >= fr.x - m.chapterHitPadX && cx <= fr.x + fr.w + m.chapterHitPadX && cy >= fr.y - m.chapterHitPadTop && cy <= fr.y + fr.h + m.chapterHitPadBottom)
        ) {
          hoveredCh = fr.ref; cx = fr.inX; cy = fr.inY; break;
        }
      }
      g.curX = cx; g.curY = cy; g.hoveredTargetChapter = hoveredCh;
      EditorWires.updateTempPreview();
    }
  },

  finishGesture() {
    const g = CanvasModel.activeCanvasGesture;
    if (!g) return;
    CanvasModel.activeCanvasGesture = null;
    const vp = g.viewport || document.querySelector(this.selectors.viewport);
    if (vp && !CanvasModel.editorCanvasState.spacePressed) vp.classList.remove(this.classes.panning);
    CanvasModel.studioGraphState.overlayBatchDirty = true;

    if (g.type === this.gestures.pan || (g.type === this.gestures.rightPan && g.moved)) {
      this.scheduleViewportSave();
      return;
    }
    if (g.type === this.gestures.choiceDrag) {
      EditorWebglScene.render();
      if (g.node && g.startIdx !== g.currentIdx) {
        Dispatcher.post(this.actions.reorderChoice, { scene: g.sceneId, from_idx: String(g.startIdx), to_idx: String(g.currentIdx) });
      }
      return;
    }
    if (g.type === this.gestures.wirePinDrag) {
      const wire = g.wire;
      if (wire && g.inserted && g.moved && wire.pins && wire.pins[g.pinIdx]) {
        CanvasModel.rerouteAfterWirePinMutation(wire);
        if (wire.wireKey) this.saveWirePins(wire.wireKey, wire.pins);
      } else {
        EditorWebglScene.render();
      }
      return;
    }
    if (g.type === this.gestures.nodeDrag) {
      const items = g.items || [];
      if (g.moved && items.length > 0) {
        const ghost = g.ghostState || CanvasLayout.evaluateNodeDragGhost(items, g.ghostDx || 0, g.ghostDy || 0);
        if (ghost.valid && (ghost.dx !== 0 || ghost.dy !== 0)) {
          CanvasLayout.applyIncrementalNodeMove(items, ghost.dx, ghost.dy);
          this.saveBatchNodePositions(items.map((it) => ({ sceneId: it.sceneId, x: it.node.x, y: it.node.y })));
        }
      } else if (!g.moved && g.sceneId) {
        Dispatcher.post(this.actions.selectScene, { scene: g.sceneId });
      }
      EditorWebglScene.render();
      return;
    }
    if (g.type === this.gestures.chapterDrag) {
      const items = g.items || [];
      EditorWebglScene.render();
      if (g.moved && items.length > 0) {
        this.saveBatchNodePositions(items.map((it) => ({ sceneId: it.sceneId, x: it.node.x, y: it.node.y })));
      } else if (!g.moved && g.chapter) {
        Dispatcher.post(this.actions.selectChapter, { chapter: g.chapter.name });
      }
      return;
    }
    if (g.type === this.gestures.wireConnect) {
      let toScene = "";
      let choiceIdx = g.choiceIdx;
      if (g.hoveredTargetId && g.fromSceneId && g.hoveredTargetId !== g.fromSceneId) {
        toScene = g.hoveredTargetId;
      } else if (g.isBottomPort && g.fromSceneId && !g.hoveredTargetId && g.hasExistingTarget && Math.hypot(g.curX - g.startX, g.curY - g.startY) > this.metrics.disconnectMinDist) {
        toScene = this.targets.end;
        choiceIdx = this.targets.firstChoiceIdx;
      }
      if (toScene) {
        Dispatcher.post(this.actions.connectChoice, { from_scene: g.fromSceneId, choice_idx: choiceIdx, to_scene: toScene });
      } else {
        EditorWires.updateTempPreview();
      }
      return;
    }
    if (g.type === this.gestures.chapterWireConnect) {
      if (g.hoveredTargetChapter && g.fromChapter) {
        Dispatcher.post(this.actions.connectChapter, { from_chapter: g.fromChapter, old_target_chapter: g.oldTargetChapter || "", to_chapter: g.hoveredTargetChapter });
      } else if (g.fromChapter && !g.hoveredTargetChapter && g.oldTargetChapter && Math.hypot(g.curX - g.startX, g.curY - g.startY) > this.metrics.disconnectMinDist) {
        Dispatcher.post(this.actions.connectChapter, { from_chapter: g.fromChapter, old_target_chapter: g.oldTargetChapter, to_chapter: this.targets.end });
      } else {
        EditorWires.updateTempPreview();
      }
      return;
    }
    EditorWebglScene.render();
  }
};
Dispatcher.register(CanvasDrag.moduleName, CanvasDrag);
