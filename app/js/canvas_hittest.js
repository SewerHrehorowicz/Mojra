"use strict";

const CanvasHittest = {
  HIT_CURSOR_MAP: {
    "scene-out-port": "crosshair",
    "chapter-out-port": "crosshair",
    "node-grip": "grab",
    "chapter-grip": "grab",
    "choice-grip": "grab",
    "wire-pin": "grab",
    wire: "grab",
    "node-title": "text",
    "choice-label": "text",
    "chapter-title": "pointer",
    "add-choice-btn": "pointer",
    "edit-scene-btn": "pointer",
    "delete-scene-btn": "pointer",
    "choice-trash-btn": "pointer",
    "branch-radio": "pointer",
    "node-card": "pointer"
  },

  clientToStudioWorld(viewport, clientX, clientY) {
    const rect = viewport.getBoundingClientRect();
    const state = CanvasModel.editorCanvasState;
    const z = state.zoom || 1.0;
    const vx = clientX - rect.left;
    const vy = clientY - rect.top;
    return {
      vx: vx,
      vy: vy,
      wx: (vx - state.panX) / z,
      wy: (vy - state.panY) / z
    };
  },

  buildPointerContext(viewport, e, isTouch) {
    const clientX = e.clientX;
    const clientY = e.clientY;
    const ptWorld = this.clientToStudioWorld(viewport, clientX, clientY);
    return {
      viewport: viewport,
      clientX: clientX,
      clientY: clientY,
      wx: ptWorld.wx,
      wy: ptWorld.wy,
      isTouch: Boolean(isTouch),
      isMultiModifier: Boolean(!isTouch && (e.shiftKey || e.metaKey || e.ctrlKey))
    };
  },

  distToSegmentSq(px, py, segA, segB) {
    const dx = segB.x - segA.x;
    const dy = segB.y - segA.y;
    const lenSq = dx * dx + dy * dy;
    if (lenSq < 0.001) {
      const ex = px - segA.x;
      const ey = py - segA.y;
      return ex * ex + ey * ey;
    }
    const t = Math.max(0, Math.min(1, ((px - segA.x) * dx + (py - segA.y) * dy) / lenSq));
    const ox = px - (segA.x + t * dx);
    const oy = py - (segA.y + t * dy);
    return ox * ox + oy * oy;
  },

  hitTest(wx, wy) {
    const gState = CanvasModel.studioGraphState;
    const CARD = CanvasModel.CARD;
    const nodes = gState.nodes;
    const chapters = gState.chapters;
    const isPreview = CanvasModel.isHistoryPreview();
    const portHitR = 10;
    const portHitRSq = portHitR * portHitR;

    if (!isPreview) {
      for (let i = nodes.length - 1; i >= 0; i--) {
        const nd = nodes[i];
        if (nd.hasChoices && nd.ports.length > 0) {
          for (let j = 0; j < nd.ports.length; j++) {
            const pt = nd.ports[j];
            const dx = wx - pt.x, dy = wy - pt.y;
            if (dx * dx + dy * dy <= portHitRSq) {
              return { type: "scene-out-port", node: nd, port: pt, portIdx: j, isBottom: false, x: pt.x, y: pt.y };
            }
          }
        } else if (nd.implicitPort) {
          const ip = nd.implicitPort;
          const dx = wx - ip.x, dy = wy - ip.y;
          if (dx * dx + dy * dy <= portHitRSq) {
            return { type: "scene-out-port", node: nd, port: ip, portIdx: 0, isBottom: true, x: ip.x, y: ip.y };
          }
        }
      }

      for (let i = chapters.length - 1; i >= 0; i--) {
        const ch = chapters[i];
        if (ch.hidden) continue;
        for (let j = 0; j < ch.outPorts.length; j++) {
          const op = ch.outPorts[j];
          const dx = wx - op.x, dy = wy - op.y;
          if (dx * dx + dy * dy <= portHitRSq) {
            return { type: "chapter-out-port", chapter: ch, outPort: op, portIdx: j, x: op.x, y: op.y };
          }
        }
      }

      const allWires = gState.wires || [];
      const pinHitRSq = 64;
      for (let i = allWires.length - 1; i >= 0; i--) {
        const w = allWires[i];
        if (!w.pins || w.pins.length === 0) continue;
        for (let pIdx = w.pins.length - 1; pIdx >= 0; pIdx--) {
          const p = w.pins[pIdx];
          const bar = EditorWebgl.computeWirePinSegmentBar(w, p);
          if (bar) {
            if (this.distToSegmentSq(wx, wy, { x: bar.x1, y: bar.y1 }, { x: bar.x2, y: bar.y2 }) <= pinHitRSq) {
              return { type: "wire-pin", wire: w, pinIdx: pIdx, pin: p, segIdx: bar.segIdx, x: bar.cx, y: bar.cy };
            }
          } else {
            const dx = wx - p.x, dy = wy - p.y;
            if (dx * dx + dy * dy <= 100) {
              return { type: "wire-pin", wire: w, pinIdx: pIdx, pin: p, x: p.x, y: p.y };
            }
          }
        }
      }
    }

    for (let i = nodes.length - 1; i >= 0; i--) {
      const nd = nodes[i];
      if (CanvasModel.inRect(wx, wy, nd.x, nd.y, nd.w, nd.h)) {
        const footerBtnY = nd.y + nd.h - CARD.footerH + CARD.pad;
        const editX = CanvasModel.getNodeFooterBtnX(nd, "edit", isPreview);
        if (CanvasModel.inRect(wx, wy, editX, footerBtnY, CARD.btnSize, CARD.btnSize)) {
          return { type: "edit-scene-btn", node: nd };
        }
        if (isPreview) return { type: "node-card", node: nd };
        const delX = CanvasModel.getNodeFooterBtnX(nd, "delete", false);
        if (CanvasModel.inRect(wx, wy, delX, footerBtnY, CARD.btnSize, CARD.btnSize)) {
          return { type: "delete-scene-btn", node: nd };
        }
        const addX = CanvasModel.getNodeFooterBtnX(nd, "add", false);
        if (CanvasModel.inRect(wx, wy, addX, footerBtnY, CARD.btnSize, CARD.btnSize)) {
          return { type: "add-choice-btn", node: nd };
        }
        if (CanvasModel.inRect(wx, wy, nd.x + 2, nd.y + 4, 24, 26)) {
          return { type: "node-grip", node: nd };
        }
        if ((nd.branchRole === "primary" || nd.branchRole === "secondary") && CanvasModel.inRect(wx, wy, nd.branchRadioX - 4, nd.branchRadioY - 4, 16, 16)) {
          return { type: "branch-radio", node: nd };
        }
        if (wx > nd.x + 26 && wx <= nd.x + nd.w - CARD.pad && wy >= nd.y + 4 && wy < nd.y + CARD.headerH) {
          return { type: "node-title", node: nd };
        }
        const footerTop = nd.y + nd.h - CARD.footerH;
        if (nd.hasChoices && nd.ports && nd.ports.length > 0 && wy >= nd.y + CARD.headerH && wy < footerTop) {
          const choiceIdx = Math.floor((wy - (nd.y + CARD.headerH)) / CARD.choiceH);
          if (choiceIdx >= 0 && choiceIdx < nd.ports.length) {
            if (wx >= nd.x + 2 && wx <= nd.x + 20) return { type: "choice-grip", node: nd, choiceIdx: choiceIdx };
            if (wx >= nd.x + nd.w - 26 && wx <= nd.x + nd.w - CARD.pad) return { type: "choice-trash-btn", node: nd, choiceIdx: choiceIdx };
            if (wx > nd.x + 20 && wx < nd.x + nd.w - 26) return { type: "choice-label", node: nd, choiceIdx: choiceIdx };
          }
        }
        return { type: "node-card", node: nd };
      }
    }

    for (let i = chapters.length - 1; i >= 0; i--) {
      const ch = chapters[i];
      if (ch.hidden) continue;
      if (CanvasModel.inRect(wx, wy, ch.x, ch.y, ch.w, 28)) {
        if (isPreview) return { type: "chapter-title", chapter: ch };
        if (wx <= ch.x + 26) return { type: "chapter-grip", chapter: ch };
        return { type: "chapter-title", chapter: ch };
      }
    }

    const wires = gState.wires;
    const wireTolSq = 49;
    for (let i = wires.length - 1; i >= 0; i--) {
      const w = wires[i];
      if (w.maxX !== undefined && (wx < w.minX - 8 || wx > w.maxX + 8 || wy < w.minY - 8 || wy > w.maxY + 8)) continue;
      const pts = w.pts;
      for (let j = 0; j < pts.length - 1; j++) {
        if (this.distToSegmentSq(wx, wy, pts[j], pts[j + 1]) <= wireTolSq) {
          return { type: "wire", wire: w, segIdx: j, wx: wx, wy: wy };
        }
      }
    }

    return { type: "empty" };
  },

  isSameHoveredItem(a, b) {
    if (a === b) return true;
    if (!a || !b || a.type !== b.type) return false;
    return a.node === b.node && a.chapter === b.chapter && a.portIdx === b.portIdx && a.choiceIdx === b.choiceIdx && a.pinIdx === b.pinIdx && a.wire === b.wire;
  }
};
Dispatcher.register("CanvasHittest", CanvasHittest);
