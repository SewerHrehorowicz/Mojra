"use strict";

function compileStaticSceneQuads(theme) {
  studioGL.instCursor = 0;

  // Slot 0: Fullscreen procedural GPU dot grid quad
  pushQuadInstance(0, 0, 1, 1, -100000.0, -100000.0, -100000.0, -100000.0, theme.bgObsidian);

  const vw = studioGraphState.viewW || 1200;
  const vh = studioGraphState.viewH || 700;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;
  const visMinX = -px / z;
  const visMinY = -py / z;
  const visMaxX = (vw - px) / z;
  const visMaxY = (vh - py) / z;
  const guardX = Math.max(120, (visMaxX - visMinX) * 0.25);
  const guardY = Math.max(120, (visMaxY - visMinY) * 0.25);
  const fMinX = visMinX - guardX;
  const fMinY = visMinY - guardY;
  const fMaxX = visMaxX + guardX;
  const fMaxY = visMaxY + guardY;
  studioGL.cullMinX = fMinX;
  studioGL.cullMinY = fMinY;
  studioGL.cullMaxX = fMaxX;
  studioGL.cullMaxY = fMaxY;
  studioGL.hasCullBounds = true;

  const chapters = studioGraphState.chapters;
  const wires = studioGraphState.wires;
  const collectors = studioGraphState.collectorBars || [];
  const nodes = studioGraphState.nodes;
  const gripSp = studioGL.uv.grip;

  // 1. Chapter Group Frames (frustum-culled)
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    if (ch.hidden) continue;
    if (ch.right < fMinX || ch.left > fMaxX || ch.bottom < fMinY || ch.top > fMaxY) {
      continue;
    }
    pushSolidQuad(ch.x, ch.y, ch.w, ch.h, theme.chapterFill);
    pushSolidQuad(ch.x + 1, ch.y + 1, ch.w - 2, 27, theme.bgElevated);
    pushDashedRectQuads(ch.x, ch.y, ch.w, ch.h, 1, 9.0, theme.borderStrong);
    pushDashedHeaderDividerQuad(ch.x, ch.y + 28, ch.w, 1, 9.0, theme.borderStrong);
    pushQuadInstance(
      ch.x + 4,
      ch.y + 3,
      gripSp.w,
      gripSp.h,
      gripSp.u0 + 30.0,
      gripSp.v0,
      gripSp.u1 + 30.0,
      gripSp.v1,
      theme.textMuted
    );
    if (studioGraphState.editingChapter !== ch && ch.y + 30 >= fMinY && ch.y <= fMaxY) {
      const chMaxW = ch.w - 38;
      if (ch._shapedName !== ch.name || ch._shapedMaxW !== chMaxW || !ch._shapedTitle) {
        ch._shapedName = ch.name;
        ch._shapedMaxW = chMaxW;
        ch._shapedTitle = shapeTextGlyphs(FONT_CHAPTER, (ch.name || "").toUpperCase(), chMaxW);
      }
      pushShapedTextQuads(ch._shapedTitle, ch.x + 28, ch.y + 15, theme.textPrimary);
    }
  }

  // 2. Static Wires + Multi-Input Collector Bars (frustum-culled)
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.maxX < fMinX || w.minX > fMaxX || w.maxY < fMinY || w.minY > fMaxY) {
      continue;
    }
    const wColor = resolveWireColor(w.colorIdx, theme);
    if (w.unreachable) {
      pushWirePolylineQuads(w.pts, 2, true, theme.unreachWire, null);
    } else {
      pushWirePolylineQuads(w.pts, 2, false, wColor, w.bridges);
    }
    if (w.pins && w.pins.length > 0) {
      for (let p = 0; p < w.pins.length; p++) {
        pushWirePinArrowQuads(w, p, w.unreachable ? false : wColor, theme, w.unreachable);
      }
    }
  }

  for (let i = 0; i < collectors.length; i++) {
    const c = collectors[i];
    if (c.maxX + 12 < fMinX || c.minX - 12 > fMaxX || c.portY + 12 < fMinY || c.barY - 12 > fMaxY) {
      continue;
    }
    const colColor = c.unreachable ? theme.unreachWire : theme.wireDefault;
    pushSolidQuad(c.minX - 1, c.barY - 1, c.maxX - c.minX + 2, 2, colColor);
    pushSolidQuad(c.x - 1, c.barY, 2, c.portY - c.barY, colColor);
  }

  // 3. Scene Node Cards (frustum-culled)
  const branchPlusSp = studioGL.uv.branchPlus;
  const editPenSp = studioGL.uv.editPen;
  const trashSp = studioGL.uv.trashIcon;
  const footerIconBtnSize = 24;
  const isHistoryPreview = typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview();

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    if (
      nd.right + 12 < fMinX ||
      nd.left - 12 > fMaxX ||
      nd.bottom + 12 < fMinY ||
      nd.top - 12 > fMaxY
    ) {
      continue;
    }
    const isUnreach = Boolean(nd.unreachable);
    const cardBg = isUnreach ? theme.unreachCardBg : theme.bgObsidian;
    const textPri = isUnreach ? theme.unreachText : theme.textPrimary;
    const textSec = isUnreach ? theme.unreachText : theme.textSecondary;

    if (isUnreach) {
      pushSolidQuad(nd.x, nd.y, nd.w, nd.h, cardBg);
      pushDashedRectQuads(nd.x, nd.y, nd.w, nd.h, 1, 8.0, theme.unreachBorder);
    } else {
      pushBorderedBoxQuads(nd.x, nd.y, nd.w, nd.h, 1, theme.borderStrong, cardBg);
    }

    pushQuadInstance(
      nd.x + 4,
      nd.y + 6,
      gripSp.w,
      gripSp.h,
      gripSp.u0 + 30.0,
      gripSp.v0,
      gripSp.u1 + 30.0,
      gripSp.v1,
      theme.textMuted
    );

    let badgeW = 0;
    if (nd.isStart || nd.isEntry) {
      const badgeTxt = nd.isStart ? "START" : "WEJŚCIE";
      if (nd._shapedBadgeTxt !== badgeTxt || !nd._shapedBadge) {
        nd._shapedBadgeTxt = badgeTxt;
        nd._shapedBadge = shapeTextGlyphs(FONT_BADGE, badgeTxt, 64);
      }
      const shapedBadge = nd._shapedBadge;
      badgeW = Math.ceil(shapedBadge.totalW) + 10;
      const bx = nd.x + nd.w - badgeW - 8;
      const by = nd.y + 8;
      pushBorderedBoxQuads(bx, by, badgeW, 17, 1, nd.isStart ? theme.textPrimary : theme.borderStrong, theme.bgElevated);
      pushShapedTextQuads(shapedBadge, bx + 5, by + 8.5, nd.isStart ? theme.textPrimary : theme.textSecondary);
    }

    let radioReserveW = 0;
    if (nd.branchRole === "primary" || nd.branchRole === "secondary") {
      const rx = nd.x + nd.w - (badgeW > 0 ? badgeW + 8 + 14 : 16);
      const ry = nd.y + 13;
      nd.branchRadioX = rx;
      nd.branchRadioY = ry;
      radioReserveW = 16;
      const sqColor = nd.branchRole === "primary" ? theme.textPrimary : theme.borderStrong;
      pushSolidQuad(rx, ry, 8, 8, sqColor);
    }

    if (studioGraphState.editingNodeTitle !== nd) {
      const maxTitleW = nd.w - 36 - (badgeW > 0 ? badgeW + 6 : 0) - radioReserveW;
      const rawTitle = nd.name || nd.id;
      if (nd._shapedTitleTxt !== rawTitle || nd._shapedTitleMaxW !== maxTitleW || !nd._shapedTitle) {
        nd._shapedTitleTxt = rawTitle;
        nd._shapedTitleMaxW = maxTitleW;
        nd._shapedTitle = shapeTextGlyphs(FONT_TITLE, rawTitle, maxTitleW);
      }
      pushShapedTextQuads(nd._shapedTitle, nd.x + 28, nd.y + 17, textPri);
    }

    const choicesTop = nd.y + NODE_HEADER_HEIGHT;
    pushSolidQuad(nd.x + 1, choicesTop, nd.w - 2, nd.h - NODE_HEADER_HEIGHT - 1, theme.borderSubtle);

    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        const rowY = choicesTop + j * NODE_CHOICE_ROW_HEIGHT;
        pushSolidQuad(nd.x + 1, rowY + 1, nd.w - 2, NODE_CHOICE_ROW_HEIGHT - 1, theme.bgObsidian);
        if (!isHistoryPreview) {
          const cgW = 16;
          const cgH = 18;
          const cgX = nd.x + 3;
          const cgY = rowY + (NODE_CHOICE_ROW_HEIGHT - cgH) * 0.5;
          pushQuadInstance(
            cgX,
            cgY,
            cgW,
            cgH,
            gripSp.u0 + 30.0,
            gripSp.v0,
            gripSp.u1 + 30.0,
            gripSp.v1,
            theme.textMuted
          );
        }
        const isEditingThisChoice =
          studioGraphState.editingChoice &&
          studioGraphState.editingChoice.node === nd &&
          studioGraphState.editingChoice.idx === j;
        if (!isEditingThisChoice) {
          const rawLabel = pt.label || "Wybór " + (j + 1);
          const choiceMaxW = nd.w - (isHistoryPreview ? 36 : 47);
          if (pt._shapedLabelTxt !== rawLabel || pt._shapedChoiceMaxW !== choiceMaxW || !pt._shapedChoice) {
            pt._shapedLabelTxt = rawLabel;
            pt._shapedChoiceMaxW = choiceMaxW;
            pt._shapedChoice = shapeTextGlyphs(FONT_CHOICE, rawLabel, choiceMaxW);
          }
          pushShapedTextQuads(
            pt._shapedChoice,
            nd.x + (isHistoryPreview ? 10 : 21),
            rowY + NODE_CHOICE_ROW_HEIGHT * 0.5,
            textSec
          );
        }
        if (!isHistoryPreview) {
          const tx = nd.x + nd.w - 24;
          const ty = rowY + (NODE_CHOICE_ROW_HEIGHT - trashSp.size) * 0.5;
          pushQuadInstance(
            tx,
            ty,
            trashSp.size,
            trashSp.size,
            trashSp.u0,
            trashSp.v0,
            trashSp.u1,
            trashSp.v1,
            textSec
          );
        }
      }
    }

    const footerTop = nd.y + nd.h - NODE_FOOTER_HEIGHT;
    pushSolidQuad(nd.x + 1, footerTop + 1, nd.w - 2, NODE_FOOTER_HEIGHT - 2, cardBg);
    const btnY = footerTop + 8;
    if (!isHistoryPreview) {
      const btnX = nd.x + 8;
      pushQuadInstance(
        btnX + (footerIconBtnSize - branchPlusSp.size) * 0.5,
        btnY + (footerIconBtnSize - branchPlusSp.size) * 0.5,
        branchPlusSp.size,
        branchPlusSp.size,
        branchPlusSp.u0,
        branchPlusSp.v0,
        branchPlusSp.u1,
        branchPlusSp.v1,
        textSec
      );
      const delX = nd.x + nd.w - 8 - footerIconBtnSize;
      pushQuadInstance(
        delX + (footerIconBtnSize - trashSp.size) * 0.5,
        btnY + (footerIconBtnSize - trashSp.size) * 0.5,
        trashSp.size,
        trashSp.size,
        trashSp.u0,
        trashSp.v0,
        trashSp.u1,
        trashSp.v1,
        textSec
      );
    }
    const editX = isHistoryPreview
      ? nd.x + nd.w - 8 - footerIconBtnSize
      : nd.x + nd.w - 8 - footerIconBtnSize * 2 - 6;
    pushQuadInstance(
      editX + (footerIconBtnSize - editPenSp.size) * 0.5,
      btnY + (footerIconBtnSize - editPenSp.size) * 0.5,
      editPenSp.size,
      editPenSp.size,
      editPenSp.u0,
      editPenSp.v0,
      editPenSp.u1,
      editPenSp.v1,
      textSec
    );
  }

  // 4. Differentiated Input Sockets (○) & Output Plugs (●) (frustum-culled)
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    if (ch.hidden) continue;
    if (
      ch.right + 12 < fMinX ||
      ch.left - 12 > fMaxX ||
      ch.bottom + 12 < fMinY ||
      ch.top - 12 > fMaxY
    ) {
      continue;
    }
    if (!ch.isRoot) {
      pushSmallInputSocketQuads(ch.inX, ch.inY, false, theme, false, ch.inColorIdx);
    }
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      const opColor = op.connected ? resolveWireColor(op.colorIdx !== undefined ? op.colorIdx : op.inColorIdx, theme) : false;
      pushSmallPortQuads(op.x, op.y, Boolean(op.connected), opColor, theme, false);
    }
  }

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    if (
      nd.right + 12 < fMinX ||
      nd.left - 12 > fMaxX ||
      nd.bottom + 12 < fMinY ||
      nd.top - 12 > fMaxY
    ) {
      continue;
    }
    if (nd.hasInputPort) {
      pushInputSocketQuads(nd.x + nd.inOx, nd.y + nd.inOy, false, theme, nd.unreachable, nd.inColorIdx);
    }
    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        const ptColor = pt.connected && !nd.unreachable ? resolveWireColor(pt.colorIdx, theme) : false;
        pushPortQuads(nd.x + pt.ox, nd.y + pt.oy, Boolean(pt.connected), ptColor, theme, nd.unreachable, Boolean(pt.isConditional));
      }
    } else if (nd.implicitPort) {
      const impColor =
        nd.implicitPort.connected && !nd.unreachable ? resolveWireColor(nd.implicitPort.colorIdx, theme) : false;
      pushPortQuads(
        nd.x + nd.implicitPort.ox,
        nd.y + nd.implicitPort.oy,
        Boolean(nd.implicitPort.connected),
        impColor,
        theme,
        nd.unreachable
      );
    }
  }

  studioGL.staticInstCount = studioGL.instCursor;
}

function chapterMatchesTargetLower(ch, targetLower) {
  if (!ch || !targetLower) return false;
  return ch.nameLower === targetLower || ch.refLower === targetLower;
}

function collectHoveredConnectionWires(hoveredWire) {
  if (!hoveredWire) return [];
  if (hoveredWire.kind === "scene-to-scene") {
    return [hoveredWire];
  }
  const allWires = studioGraphState.wires || [];
  const result = [];
  const seen = new Set();
  const pushUnique = (w) => {
    if (w && !seen.has(w)) {
      seen.add(w);
      result.push(w);
    }
  };

  pushUnique(hoveredWire);

  if (hoveredWire.kind === "scene-to-chapter-out") {
    const fromCh = hoveredWire.fromChapterObj;
    const targetLower = hoveredWire.targetChapterLower;
    const toCh = studioGraphState.chapterByKey.get(targetLower);
    for (let i = 0; i < allWires.length; i++) {
      const w = allWires[i];
      if (
        w.kind === "chapter-to-chapter" &&
        w.fromChapterObj === fromCh &&
        (w.toChapterObj === toCh || chapterMatchesTargetLower(w.toChapterObj, targetLower))
      ) {
        pushUnique(w);
      } else if (
        w.kind === "chapter-in-to-entry" &&
        (w.toChapterObj === toCh || chapterMatchesTargetLower(w.toChapterObj, targetLower))
      ) {
        pushUnique(w);
      }
    }
  } else if (hoveredWire.kind === "chapter-to-chapter") {
    const fromCh = hoveredWire.fromChapterObj;
    const toCh = hoveredWire.toChapterObj;
    for (let i = 0; i < allWires.length; i++) {
      const w = allWires[i];
      if (
        w.kind === "scene-to-chapter-out" &&
        w.fromChapterObj === fromCh &&
        chapterMatchesTargetLower(toCh, w.targetChapterLower)
      ) {
        pushUnique(w);
      } else if (w.kind === "chapter-in-to-entry" && w.toChapterObj === toCh) {
        pushUnique(w);
      }
    }
  } else if (hoveredWire.kind === "chapter-in-to-entry") {
    const toCh = hoveredWire.toChapterObj;
    const upstreamFromChapters = new Set();
    for (let i = 0; i < allWires.length; i++) {
      const w = allWires[i];
      if (w.kind === "chapter-to-chapter" && w.toChapterObj === toCh) {
        pushUnique(w);
        if (w.fromChapterObj) upstreamFromChapters.add(w.fromChapterObj);
      }
    }
    for (let i = 0; i < allWires.length; i++) {
      const w = allWires[i];
      if (
        w.kind === "scene-to-chapter-out" &&
        upstreamFromChapters.has(w.fromChapterObj) &&
        chapterMatchesTargetLower(toCh, w.targetChapterLower)
      ) {
        pushUnique(w);
      }
    }
  }

  return result;
}

function appendDynamicOverlayQuads(theme) {
  studioGL.instCursor = studioGL.staticInstCount;

  const hovered = studioGraphState.hoveredItem;
  const gesture = activeCanvasGesture;
  const hoveredWire =
    hovered && (hovered.type === "wire" || hovered.type === "wire-pin")
      ? hovered.wire
      : gesture && gesture.type === "wire-pin-drag" && gesture.wire
        ? gesture.wire
        : null;
  const gripSp = studioGL.uv.grip;

  const renderChapterPortsOverlay = (ch) => {
    if (!ch || ch.hidden) return;
    if (!ch.isRoot) {
      pushSmallInputSocketQuads(ch.inX, ch.inY, false, theme, false, ch.inColorIdx);
    }
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      const opColor = op.connected ? resolveWireColor(op.colorIdx !== undefined ? op.colorIdx : op.inColorIdx, theme) : false;
      pushSmallPortQuads(op.x, op.y, Boolean(op.connected), opColor, theme, false);
    }
  };

  if (gesture && gesture.type === "chapter-drag" && gesture.chapter && gesture.dragConfirmed) {
    const ch = gesture.chapter;
    pushSolidBorderQuads(ch.x, ch.y, ch.w, ch.h, 1, theme.textPrimary);
    renderChapterPortsOverlay(ch);
  } else if (studioGraphState.selectedChapterKey) {
    const selCh = studioGraphState.chapterByKey.get(studioGraphState.selectedChapterKey);
    if (selCh && !selCh.hidden) {
      pushSolidBorderQuads(selCh.x, selCh.y, selCh.w, selCh.h, 1, theme.textSecondary);
      renderChapterPortsOverlay(selCh);
    }
  }

  const isNodeDragging = Boolean(gesture && gesture.type === "node-drag");
  const nodeHighlightColor = isNodeDragging ? theme.textPrimary : theme.textSecondary;
  selectedNodeIds.forEach((selId) => {
    const nd = studioGraphState.nodeById.get(selId);
    if (!nd) return;
    pushSolidBorderQuads(nd.x, nd.y, nd.w, nd.h, 1, nodeHighlightColor);
    if (nd.hasInputPort) {
      pushInputSocketQuads(nd.x + nd.inOx, nd.y + nd.inOy, false, theme, nd.unreachable, nd.inColorIdx);
    }
    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        const ptColor = pt.connected && !nd.unreachable ? resolveWireColor(pt.colorIdx, theme) : false;
        pushPortQuads(nd.x + pt.ox, nd.y + pt.oy, Boolean(pt.connected), ptColor, theme, nd.unreachable, Boolean(pt.isConditional));
      }
    } else if (nd.implicitPort) {
      const impColor =
        nd.implicitPort.connected && !nd.unreachable ? resolveWireColor(nd.implicitPort.colorIdx, theme) : false;
      pushPortQuads(
        nd.x + nd.implicitPort.ox,
        nd.y + nd.implicitPort.oy,
        Boolean(nd.implicitPort.connected),
        impColor,
        theme,
        nd.unreachable
      );
    }
  });

  if (hoveredWire) {
    const chain = collectHoveredConnectionWires(hoveredWire);
    for (let i = 0; i < chain.length; i++) {
      const hw = chain[i];
      pushWirePolylineQuads(hw.pts, 2, false, theme.textSecondary, hw.bridges);
    }
    for (let i = 0; i < chain.length; i++) {
      const hw = chain[i];
      if (hw.pins && hw.pins.length > 0) {
        for (let p = 0; p < hw.pins.length; p++) {
          pushWirePinArrowQuads(hw, p, theme.textSecondary, theme, false);
        }
      }
      if (hw.fromNode) {
        pushPortQuads(hw.x1, hw.y1, true, theme.textSecondary, theme, false, Boolean(hw.fromPort && hw.fromPort.isConditional));
      } else if (hw.fromChapterOutPort) {
        pushSmallPortQuads(hw.fromChapterOutPort.x, hw.fromChapterOutPort.y, true, theme.textSecondary, theme, false);
      } else if (hw.kind === "chapter-in-to-entry" && hw.fromChapterObj) {
        pushSmallInputSocketQuads(hw.fromChapterObj.inX, hw.fromChapterObj.inY, theme.textSecondary, theme, false, -1);
      }
      if (hw.toNode) {
        pushInputSocketQuads(hw.x2, hw.y2, theme.textSecondary, theme, false, -1);
      } else if (hw.toChapterOutPort) {
        pushSmallPortQuads(hw.toChapterOutPort.x, hw.toChapterOutPort.y, true, theme.textSecondary, theme, false);
      } else if (hw.toChapterObj) {
        pushSmallInputSocketQuads(hw.toChapterObj.inX, hw.toChapterObj.inY, theme.textSecondary, theme, false, -1);
      }
    }
  }

  if (gesture && gesture.type === "wire-pin-drag" && gesture.wire && gesture.wire.pins && gesture.wire.pins[gesture.pinIdx]) {
    pushWirePinArrowQuads(gesture.wire, gesture.pinIdx, true, theme, false);
  }

  if (gesture && gesture.type === "choice-drag" && gesture.node && gesture.node.ports) {
    const nd = gesture.node;
    const j = gesture.currentIdx;
    if (j >= 0 && j < nd.ports.length) {
      const pt = nd.ports[j];
      const rowY = nd.y + NODE_HEADER_HEIGHT + j * NODE_CHOICE_ROW_HEIGHT;
      pushBorderedBoxQuads(nd.x + 1, rowY, nd.w - 2, NODE_CHOICE_ROW_HEIGHT, 1, theme.textPrimary, theme.bgElevated);
      const cgW = 16;
      const cgH = 18;
      pushQuadInstance(
        nd.x + 3,
        rowY + (NODE_CHOICE_ROW_HEIGHT - cgH) * 0.5,
        cgW,
        cgH,
        gripSp.u0 + 30.0,
        gripSp.v0,
        gripSp.u1 + 30.0,
        gripSp.v1,
        theme.textPrimary
      );
      const rawLabel = pt.label || "Wybór " + (j + 1);
      const choiceMaxW = nd.w - 47;
      if (pt._shapedLabelTxt !== rawLabel || pt._shapedChoiceMaxW !== choiceMaxW || !pt._shapedChoice) {
        pt._shapedLabelTxt = rawLabel;
        pt._shapedChoiceMaxW = choiceMaxW;
        pt._shapedChoice = shapeTextGlyphs(FONT_CHOICE, rawLabel, choiceMaxW);
      }
      pushShapedTextQuads(pt._shapedChoice, nd.x + 21, rowY + NODE_CHOICE_ROW_HEIGHT * 0.5, theme.textPrimary);
      const trashSp = studioGL.uv.trashIcon;
      const tx = nd.x + nd.w - 24;
      const ty = rowY + (NODE_CHOICE_ROW_HEIGHT - trashSp.size) * 0.5;
      pushQuadInstance(tx, ty, trashSp.size, trashSp.size, trashSp.u0, trashSp.v0, trashSp.u1, trashSp.v1, theme.textSecondary);
      pushPortQuads(nd.x + pt.ox, nd.y + pt.oy, Boolean(pt.connected), true, theme, nd.unreachable, Boolean(pt.isConditional));
    }
  }

  if (gesture && (gesture.type === "wire-connect" || gesture.type === "chapter-wire-connect")) {
    const hoveredTarget = gesture.hoveredTargetId || "";
    if (
      gesture.type === "wire-connect" &&
      hoveredTarget.indexOf("chapter:") === 0 &&
      hoveredTarget !== "chapter:__next__" &&
      gesture.midOutX !== undefined
    ) {
      const pts1 = simplifyOrthogonalPoints(
        buildWireWaypoints(
          gesture.startX,
          gesture.startY,
          gesture.midOutX,
          gesture.midOutY,
          null,
          0,
          gesture.midOutX - 8,
          gesture.midOutX + 8,
          Boolean(gesture.isBottomPort)
        )
      );
      const pts2 = simplifyOrthogonalPoints(
        buildWireWaypoints(
          gesture.midOutX,
          gesture.midOutY,
          gesture.curX,
          gesture.curY,
          gesture.midOutY,
          0,
          null,
          null,
          true
        )
      );
      pushWirePolylineQuads(pts1, 2, true, theme.textPrimary);
      pushWirePolylineQuads(pts2, 2, true, theme.textPrimary);
    } else {
      const ptsTemp = simplifyOrthogonalPoints(
        buildWireWaypoints(
          gesture.startX,
          gesture.startY,
          gesture.curX,
          gesture.curY,
          null,
          0,
          null,
          null,
          Boolean(gesture.isBottomPort)
        )
      );
      pushWirePolylineQuads(ptsTemp, 2, true, theme.textPrimary);
    }
    const startConnected =
      gesture.type === "wire-connect"
        ? Boolean(gesture.sourcePort ? gesture.sourcePort.connected : gesture.hasExistingTarget)
        : Boolean(gesture.activeChapterOutPort && gesture.activeChapterOutPort.connected);
    if (gesture.type === "chapter-wire-connect") {
      pushSmallPortQuads(gesture.startX, gesture.startY, startConnected, true, theme, false);
    } else {
      pushPortQuads(gesture.startX, gesture.startY, startConnected, true, theme, false, Boolean(gesture.sourcePort && gesture.sourcePort.isConditional));
    }
    if (gesture.hoveredSceneNode) {
      const tnd = gesture.hoveredSceneNode;
      pushInputSocketQuads(tnd.x + tnd.inOx, tnd.y + tnd.inOy, true, theme, false, -1);
    }
    if (gesture.hoveredChapterIn) {
      pushSmallInputSocketQuads(
        gesture.hoveredChapterIn.inX,
        gesture.hoveredChapterIn.inY,
        true,
        theme,
        false,
        -1
      );
    }
    if (gesture.hoveredChapterOutPort) {
      pushSmallPortQuads(
        gesture.hoveredChapterOutPort.x,
        gesture.hoveredChapterOutPort.y,
        Boolean(gesture.hoveredChapterOutPort.connected),
        true,
        theme,
        false
      );
    }
  }

  if (hovered) {
    if (hovered.type === "scene-out-port") {
      pushPortQuads(hovered.x, hovered.y, isHoveredPortItemConnected(hovered), true, theme, false, Boolean(hovered.port && hovered.port.isConditional));
    } else if (hovered.type === "chapter-out-port") {
      pushSmallPortQuads(hovered.x, hovered.y, isHoveredPortItemConnected(hovered), true, theme, false);
    } else if (hovered.type === "wire-pin" && hovered.wire && hovered.pinIdx !== undefined) {
      pushWirePinArrowQuads(hovered.wire, hovered.pinIdx, true, theme, false);
    } else if (hovered.type === "node-grip" && hovered.node) {
      const nd = hovered.node;
      pushQuadInstance(
        nd.x + 4,
        nd.y + 6,
        gripSp.w,
        gripSp.h,
        gripSp.u0 + 30.0,
        gripSp.v0,
        gripSp.u1 + 30.0,
        gripSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "choice-grip" && hovered.node && hovered.choiceIdx !== undefined) {
      const nd = hovered.node;
      const rowY = nd.y + NODE_HEADER_HEIGHT + hovered.choiceIdx * NODE_CHOICE_ROW_HEIGHT;
      const cgW = 16;
      const cgH = 18;
      pushQuadInstance(
        nd.x + 3,
        rowY + (NODE_CHOICE_ROW_HEIGHT - cgH) * 0.5,
        cgW,
        cgH,
        gripSp.u0 + 30.0,
        gripSp.v0,
        gripSp.u1 + 30.0,
        gripSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "chapter-grip" && hovered.chapter && !hovered.chapter.hidden) {
      const ch = hovered.chapter;
      pushQuadInstance(
        ch.x + 4,
        ch.y + 3,
        gripSp.w,
        gripSp.h,
        gripSp.u0 + 30.0,
        gripSp.v0,
        gripSp.u1 + 30.0,
        gripSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "add-choice-btn" && hovered.node) {
      const nd = hovered.node;
      const branchPlusSp = studioGL.uv.branchPlus;
      const btnX = nd.x + 8;
      const btnY = nd.y + nd.h - NODE_FOOTER_HEIGHT + 8;
      const btnS = 24;
      const cardBg = nd.unreachable ? theme.unreachCardBg : theme.bgObsidian;
      const ix = btnX + (btnS - branchPlusSp.size) * 0.5;
      const iy = btnY + (btnS - branchPlusSp.size) * 0.5;
      pushSolidQuad(ix - 1, iy - 1, branchPlusSp.size + 2, branchPlusSp.size + 2, cardBg);
      pushQuadInstance(
        ix,
        iy,
        branchPlusSp.size,
        branchPlusSp.size,
        branchPlusSp.u0,
        branchPlusSp.v0,
        branchPlusSp.u1,
        branchPlusSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "edit-scene-btn" && hovered.node) {
      const nd = hovered.node;
      const editPenSp = studioGL.uv.editPen;
      const btnS = 24;
      const isPreview = typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview();
      const btnX = isPreview ? nd.x + nd.w - 8 - btnS : nd.x + nd.w - 8 - btnS * 2 - 6;
      const btnY = nd.y + nd.h - NODE_FOOTER_HEIGHT + 8;
      const cardBg = nd.unreachable ? theme.unreachCardBg : theme.bgObsidian;
      const ix = btnX + (btnS - editPenSp.size) * 0.5;
      const iy = btnY + (btnS - editPenSp.size) * 0.5;
      pushSolidQuad(ix - 1, iy - 1, editPenSp.size + 2, editPenSp.size + 2, cardBg);
      pushQuadInstance(
        ix,
        iy,
        editPenSp.size,
        editPenSp.size,
        editPenSp.u0,
        editPenSp.v0,
        editPenSp.u1,
        editPenSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "delete-scene-btn" && hovered.node) {
      const nd = hovered.node;
      const trashSp = studioGL.uv.trashIcon;
      const btnS = 24;
      const btnX = nd.x + nd.w - 8 - btnS;
      const btnY = nd.y + nd.h - NODE_FOOTER_HEIGHT + 8;
      const cardBg = nd.unreachable ? theme.unreachCardBg : theme.bgObsidian;
      const ix = btnX + (btnS - trashSp.size) * 0.5;
      const iy = btnY + (btnS - trashSp.size) * 0.5;
      pushSolidQuad(ix - 1, iy - 1, trashSp.size + 2, trashSp.size + 2, cardBg);
      pushQuadInstance(
        ix,
        iy,
        trashSp.size,
        trashSp.size,
        trashSp.u0,
        trashSp.v0,
        trashSp.u1,
        trashSp.v1,
        theme.accentDanger
      );
    } else if (hovered.type === "choice-trash-btn" && hovered.node && typeof hovered.choiceIdx === "number") {
      const nd = hovered.node;
      const trashSp = studioGL.uv.trashIcon;
      const rowY = nd.y + NODE_HEADER_HEIGHT + hovered.choiceIdx * NODE_CHOICE_ROW_HEIGHT;
      const tx = nd.x + nd.w - 24;
      const ty = rowY + (NODE_CHOICE_ROW_HEIGHT - trashSp.size) * 0.5;
      pushSolidQuad(tx - 1, ty - 1, trashSp.size + 2, trashSp.size + 2, theme.bgObsidian);
      pushQuadInstance(
        tx,
        ty,
        trashSp.size,
        trashSp.size,
        trashSp.u0,
        trashSp.v0,
        trashSp.u1,
        trashSp.v1,
        theme.accentDanger
      );
    } else if (hovered.type === "branch-radio" && hovered.node) {
      const nd = hovered.node;
      const sqColor = nd.branchRole === "primary" ? theme.textPrimary : theme.textSecondary;
      pushSolidQuad(nd.branchRadioX, nd.branchRadioY, 8, 8, sqColor);
    }
  }

  if (gesture && gesture.type === "marquee-select" && gesture.moved && gesture.marqueeRect) {
    const mr = gesture.marqueeRect;
    pushQuadInstance(mr.x, mr.y, mr.w, mr.h, -50000.0, 0.0, -50000.0, 0.0, theme.marqueeFill);
    const t = 1;
    pushQuadInstance(mr.x, mr.y, mr.w, t, -50000.0, 0.0, -50000.0, 0.0, theme.textPrimary);
    pushQuadInstance(mr.x, mr.y + mr.h - t, mr.w, t, -50000.0, 0.0, -50000.0, 0.0, theme.textPrimary);
    pushQuadInstance(mr.x, mr.y, t, mr.h, -50000.0, 0.0, -50000.0, 0.0, theme.textPrimary);
    pushQuadInstance(mr.x + mr.w - t, mr.y, t, mr.h, -50000.0, 0.0, -50000.0, 0.0, theme.textPrimary);
  }

  studioGL.totalInstCount = studioGL.instCursor;
}

function renderStudioCanvas() {
  const sceneModal = qs("#modal-scene-text");
  if (sceneModal && (sceneModal.classList.contains("mode-full") || sceneModal.classList.contains("has-right-panel"))) {
    return;
  }
  const canvas = qs("#studio-canvas");
  if (!canvas) return;
  const gl = initStudioWebGLContext(canvas);
  if (!gl) return;

  if (studioGL.atlasImgLoaded && !studioGL.atlasUploaded && studioGL.atlasImg) {
    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D, studioGL.fontTex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, studioGL.atlasImg);
    studioGL.atlasUploaded = true;
  }

  const theme = resolveStudioThemeColors();
  const vw = studioGraphState.viewW || 1200;
  const vh = studioGraphState.viewH || 700;
  const dpr = studioGraphState.canvasDpr || 2.0;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;

  const visMinX = -px / z;
  const visMinY = -py / z;
  const visMaxX = (vw - px) / z;
  const visMaxY = (vh - py) / z;
  if (
    !studioGraphState.staticBatchDirty &&
    (!studioGL.hasCullBounds ||
      visMinX < studioGL.cullMinX ||
      visMaxX > studioGL.cullMaxX ||
      visMinY < studioGL.cullMinY ||
      visMaxY > studioGL.cullMaxY)
  ) {
    studioGraphState.staticBatchDirty = true;
  }

  if (studioGraphState.staticBatchDirty) {
    compileStaticSceneQuads(theme);
    appendDynamicOverlayQuads(theme);
    const ringIdx = ((studioGL.vboRingIdx || 0) + 1) % 3;
    studioGL.vboRingIdx = ringIdx;
    const vbo = studioGL.instVbos[ringIdx];
    studioGL.instVbo = vbo;
    studioGL.bindInstVboAttribs(vbo);
    const activeFloats = studioGL.totalInstCount * FLOATS_PER_INSTANCE;
    const requiredBytes = (studioGL.totalInstCount + 64) * BYTES_PER_INSTANCE;
    if (studioGL.instVboCapacities[ringIdx] < requiredBytes) {
      const nextCapBytes = Math.max(requiredBytes * 2, 4096 * BYTES_PER_INSTANCE);
      gl.bufferData(gl.ARRAY_BUFFER, nextCapBytes, gl.STREAM_DRAW);
      studioGL.instVboCapacities[ringIdx] = nextCapBytes;
    }
    gl.bufferSubData(
      gl.ARRAY_BUFFER,
      0,
      instanceFloatView.subarray(0, activeFloats)
    );
    studioGraphState.staticBatchDirty = false;
    studioGraphState.overlayBatchDirty = false;
  } else if (studioGraphState.overlayBatchDirty) {
    appendDynamicOverlayQuads(theme);
    const dynCount = studioGL.totalInstCount - studioGL.staticInstCount;
    if (dynCount > 0) {
      const ringIdx = studioGL.vboRingIdx || 0;
      const vbo = studioGL.instVbos[ringIdx];
      const startFloat = studioGL.staticInstCount * FLOATS_PER_INSTANCE;
      const endFloat = studioGL.totalInstCount * FLOATS_PER_INSTANCE;
      const requiredBytes = studioGL.totalInstCount * BYTES_PER_INSTANCE;
      gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
      if (studioGL.instVboCapacities[ringIdx] < requiredBytes) {
        const nextCapBytes = Math.max(requiredBytes * 2, 4096 * BYTES_PER_INSTANCE);
        gl.bufferData(gl.ARRAY_BUFFER, nextCapBytes, gl.STREAM_DRAW);
        studioGL.instVboCapacities[ringIdx] = nextCapBytes;
        gl.bufferSubData(
          gl.ARRAY_BUFFER,
          0,
          instanceFloatView.subarray(0, endFloat)
        );
      } else {
        gl.bufferSubData(
          gl.ARRAY_BUFFER,
          studioGL.staticInstCount * BYTES_PER_INSTANCE,
          instanceFloatView.subarray(startFloat, endFloat)
        );
      }
    }
    studioGraphState.overlayBatchDirty = false;
  }

  if (studioGL.lastVpW !== canvas.width || studioGL.lastVpH !== canvas.height) {
    gl.viewport(0, 0, canvas.width, canvas.height);
    studioGL.lastVpW = canvas.width;
    studioGL.lastVpH = canvas.height;
  }
  if (studioGL.lastUniformVw !== vw || studioGL.lastUniformVh !== vh) {
    gl.uniform2f(studioGL.uViewSize, vw, vh);
    studioGL.lastUniformVw = vw;
    studioGL.lastUniformVh = vh;
  }
  if (studioGL.lastUniformPx !== px || studioGL.lastUniformPy !== py || studioGL.lastUniformZ !== z) {
    gl.uniform3f(studioGL.uPanZoom, px, py, z);
    studioGL.lastUniformPx = px;
    studioGL.lastUniformPy = py;
    studioGL.lastUniformZ = z;
  }
  if (studioGL.lastUniformDpr !== dpr) {
    gl.uniform1f(studioGL.uDpr, dpr);
    studioGL.lastUniformDpr = dpr;
  }
  if (studioGL.lastUniformLight !== theme.isLight) {
    gl.uniform3f(studioGL.uBgColor, theme.bgObsidianRgb[0], theme.bgObsidianRgb[1], theme.bgObsidianRgb[2]);
    gl.uniform3f(studioGL.uDotColor, theme.gridDotRgb[0], theme.gridDotRgb[1], theme.gridDotRgb[2]);
    studioGL.lastUniformLight = theme.isLight;
  }
  const gridStep = z < 0.45 ? EDITOR_GRID_SIZE * 2 : EDITOR_GRID_SIZE;
  if (studioGL.lastUniformGridStep !== gridStep) {
    gl.uniform1f(studioGL.uGridStep, gridStep);
    studioGL.lastUniformGridStep = gridStep;
  }

  // 1 SINGLE INSTANCED GPU DRAW CALL
  if (studioGL.instExt) {
    studioGL.instExt.drawArraysInstancedANGLE(gl.TRIANGLES, 0, 6, studioGL.totalInstCount);
  }
  studioGraphState.lastDrawCalls = 1;
}
