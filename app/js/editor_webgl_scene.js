"use strict";

const EditorWebglScene = {
  pushChapterPortsQuads(ch, theme) {
    if (!ch || ch.hidden) return;
    if (!ch.isRoot) {
      EditorWebgl.pushSmallInputSocketQuads(ch.inX, ch.inY, false, theme, false, ch.inColorIdx);
    }
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      const opColor = op.connected ? EditorWebgl.resolveWireColor(op.colorIdx !== undefined ? op.colorIdx : op.inColorIdx, theme) : false;
      EditorWebgl.pushSmallPortQuads(op.x, op.y, Boolean(op.connected), opColor, theme, false);
    }
  },

  pushNodePortsQuads(nd, theme) {
    if (nd.hasInputPort) {
      EditorWebgl.pushInputSocketQuads(nd.inX, nd.inY, false, theme, nd.unreachable, nd.inColorIdx);
    }
    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        const ptColor = pt.connected && !nd.unreachable ? EditorWebgl.resolveWireColor(pt.colorIdx, theme) : false;
        EditorWebgl.pushPortQuads(pt.x, pt.y, Boolean(pt.connected), ptColor, theme, nd.unreachable, Boolean(pt.isConditional));
      }
    } else if (nd.implicitPort) {
      const imp = nd.implicitPort;
      const impColor = imp.connected && !nd.unreachable ? EditorWebgl.resolveWireColor(imp.colorIdx, theme) : false;
      EditorWebgl.pushPortQuads(imp.x, imp.y, Boolean(imp.connected), impColor, theme, nd.unreachable, false);
    }
  },

  pushNodeChoiceRowQuads(nd, j, isHistoryPreview, isDraggingRow, theme) {
    const CARD = CanvasModel.CARD, UV = CanvasModel.UV, gState = CanvasModel.studioGraphState;
    const pt = nd.ports[j];
    const rowY = nd.y + CanvasModel.choiceRowY(j);
    const gripSp = EditorWebgl.studioGL.uv.grip;
    const trashSp = EditorWebgl.studioGL.uv.trashIcon;
    const textSec = nd.unreachable ? theme.unreachText : theme.textSecondary;

    if (isDraggingRow) {
      EditorWebgl.pushBorderedBoxQuads(nd.x + 1, rowY, nd.w - 2, CARD.choiceH, 1, theme.textPrimary, theme.bgElevated);
    } else {
      EditorWebgl.pushSolidQuad(nd.x + 1, rowY + 1, nd.w - 2, CARD.choiceH - 1, theme.bgObsidian);
    }

    if (!isHistoryPreview) {
      EditorWebgl.pushQuadInstance(nd.x + 3, rowY + (CARD.choiceH - 18) * 0.5, 16, 18, gripSp.u0 + UV.solid, gripSp.v0, gripSp.u1 + UV.solid, gripSp.v1, isDraggingRow ? theme.textPrimary : theme.textMuted);
    }

    const isEditingThisChoice = !isDraggingRow && gState.editingChoice && gState.editingChoice.node === nd && gState.editingChoice.idx === j;
    if (!isEditingThisChoice) {
      const rawLabel = pt.label || AppShellController.tr("choice_prefix") + (j + 1);
      const choiceMaxW = nd.w - (isHistoryPreview ? 36 : 47);
      if (pt._shapedLabelTxt !== rawLabel || pt._shapedChoiceMaxW !== choiceMaxW || !pt._shapedChoice) {
        pt._shapedLabelTxt = rawLabel;
        pt._shapedChoiceMaxW = choiceMaxW;
        pt._shapedChoice = EditorWebgl.shapeTextGlyphs(CanvasModel.FONT_CHOICE, rawLabel, choiceMaxW);
      }
      EditorWebgl.pushShapedTextQuads(pt._shapedChoice, nd.x + (isHistoryPreview ? 10 : 21), rowY + CARD.choiceH * 0.5, isDraggingRow ? theme.textPrimary : textSec);
    }

    if (!isHistoryPreview) {
      const tx = nd.x + nd.w - 24;
      const ty = rowY + (CARD.choiceH - trashSp.size) * 0.5;
      EditorWebgl.pushQuadInstance(tx, ty, trashSp.size, trashSp.size, trashSp.u0, trashSp.v0, trashSp.u1, trashSp.v1, isDraggingRow ? theme.textSecondary : textSec);
    }
  },

  pushCenteredFooterSprite(btnX, btnY, sprite, color, bgMaskColor) {
    const btnSize = CanvasModel.CARD.btnSize;
    const ix = btnX + (btnSize - sprite.size) * 0.5;
    const iy = btnY + (btnSize - sprite.size) * 0.5;
    if (bgMaskColor !== undefined) {
      EditorWebgl.pushSolidQuad(ix - 1, iy - 1, sprite.size + 2, sprite.size + 2, bgMaskColor);
    }
    EditorWebgl.pushQuadInstance(ix, iy, sprite.size, sprite.size, sprite.u0, sprite.v0, sprite.u1, sprite.v1, color);
  },

  compileStaticSceneQuads(theme) {
    const studioGL = EditorWebgl.studioGL;
    const gState = CanvasModel.studioGraphState;
    const state = CanvasModel.editorCanvasState;
    const CARD = CanvasModel.CARD, UV = CanvasModel.UV;
    studioGL.instCursor = 0;

    EditorWebgl.pushQuadInstance(0, 0, 1, 1, UV.grid, UV.grid, UV.grid, UV.grid, theme.bgObsidian);

    const vw = gState.viewW || 1200, vh = gState.viewH || 700;
    const z = state.zoom || 1.0, px = state.panX || 0, py = state.panY || 0;
    const visMinX = -px / z, visMinY = -py / z, visMaxX = (vw - px) / z, visMaxY = (vh - py) / z;
    const guardX = Math.max(120, (visMaxX - visMinX) * 0.25);
    const guardY = Math.max(120, (visMaxY - visMinY) * 0.25);
    const fMinX = visMinX - guardX, fMinY = visMinY - guardY, fMaxX = visMaxX + guardX, fMaxY = visMaxY + guardY;
    studioGL.cullMinX = fMinX; studioGL.cullMinY = fMinY; studioGL.cullMaxX = fMaxX; studioGL.cullMaxY = fMaxY;
    studioGL.hasCullBounds = true;

    const chapters = gState.chapters, wires = gState.wires, collectors = gState.collectorBars || [], nodes = gState.nodes;
    const gripSp = studioGL.uv.grip;

    for (let i = 0; i < chapters.length; i++) {
      const ch = chapters[i];
      if (ch.hidden || ch.right < fMinX || ch.left > fMaxX || ch.bottom < fMinY || ch.top > fMaxY) continue;
      EditorWebgl.pushSolidQuad(ch.x, ch.y, ch.w, ch.h, theme.chapterFill);
      EditorWebgl.pushSolidQuad(ch.x + 1, ch.y + 1, ch.w - 2, 27, theme.bgElevated);
      EditorWebgl.pushDashedRectQuads(ch.x, ch.y, ch.w, ch.h, 1, 9.0, theme.borderStrong);
      EditorWebgl.pushDashedHeaderDividerQuad(ch.x, ch.y + 28, ch.w, 1, 9.0, theme.borderStrong);
      EditorWebgl.pushQuadInstance(ch.x + 4, ch.y + 3, gripSp.w, gripSp.h, gripSp.u0 + UV.solid, gripSp.v0, gripSp.u1 + UV.solid, gripSp.v1, theme.textMuted);
      if (gState.editingChapter !== ch && ch.y + 30 >= fMinY && ch.y <= fMaxY) {
        const chMaxW = ch.w - 38;
        if (ch._shapedName !== ch.name || ch._shapedMaxW !== chMaxW || !ch._shapedTitle) {
          ch._shapedName = ch.name;
          ch._shapedMaxW = chMaxW;
          ch._shapedTitle = EditorWebgl.shapeTextGlyphs(CanvasModel.FONT_CHAPTER, (ch.name || "").toUpperCase(), chMaxW);
        }
        EditorWebgl.pushShapedTextQuads(ch._shapedTitle, ch.x + 28, ch.y + 15, theme.textPrimary);
      }
    }

    for (let i = 0; i < wires.length; i++) {
      const w = wires[i];
      if (w.maxX < fMinX || w.minX > fMaxX || w.maxY < fMinY || w.minY > fMaxY) continue;
      const wColor = EditorWebgl.resolveWireColor(w.colorIdx, theme);
      if (w.unreachable) EditorWebgl.pushWirePolylineQuads(w.pts, 2, true, theme.unreachWire, null);
      else EditorWebgl.pushWirePolylineQuads(w.pts, 2, false, wColor, w.bridges);
      if (w.pins && w.pins.length > 0) {
        for (let p = 0; p < w.pins.length; p++) {
          EditorWebgl.pushWirePinArrowQuads(w, p, w.unreachable ? false : wColor, theme, w.unreachable);
        }
      }
    }

    for (let i = 0; i < collectors.length; i++) {
      const c = collectors[i];
      if (c.maxX + 12 < fMinX || c.minX - 12 > fMaxX || c.portY + 12 < fMinY || c.barY - 12 > fMaxY) continue;
      const colColor = c.unreachable ? theme.unreachWire : theme.wireDefault;
      EditorWebgl.pushSolidQuad(c.minX - 1, c.barY - 1, c.maxX - c.minX + 2, 2, colColor);
      EditorWebgl.pushSolidQuad(c.x - 1, c.barY, 2, c.portY - c.barY, colColor);
    }

    const branchPlusSp = studioGL.uv.branchPlus, editPenSp = studioGL.uv.editPen, trashSp = studioGL.uv.trashIcon;
    const isHistoryPreview = CanvasModel.isHistoryPreview();

    for (let i = 0; i < nodes.length; i++) {
      const nd = nodes[i];
      if (nd.right + 12 < fMinX || nd.left - 12 > fMaxX || nd.bottom + 12 < fMinY || nd.top - 12 > fMaxY) continue;
      const isUnreach = Boolean(nd.unreachable);
      const cardBg = isUnreach ? theme.unreachCardBg : theme.bgObsidian;
      const textPri = isUnreach ? theme.unreachText : theme.textPrimary;
      const textSec = isUnreach ? theme.unreachText : theme.textSecondary;

      if (isUnreach) {
        EditorWebgl.pushSolidQuad(nd.x, nd.y, nd.w, nd.h, cardBg);
        EditorWebgl.pushDashedRectQuads(nd.x, nd.y, nd.w, nd.h, 1, 8.0, theme.unreachBorder);
      } else {
        EditorWebgl.pushBorderedBoxQuads(nd.x, nd.y, nd.w, nd.h, 1, theme.borderStrong, cardBg);
      }

      EditorWebgl.pushQuadInstance(nd.x + 4, nd.y + 6, gripSp.w, gripSp.h, gripSp.u0 + UV.solid, gripSp.v0, gripSp.u1 + UV.solid, gripSp.v1, theme.textMuted);

      let badgeW = 0;
      if (nd.isStart || nd.isEntry) {
        const badgeTxt = nd.isStart ? "START" : AppShellController.tr("entry_badge_upper");
        if (nd._shapedBadgeTxt !== badgeTxt || !nd._shapedBadge) {
          nd._shapedBadgeTxt = badgeTxt;
          nd._shapedBadge = EditorWebgl.shapeTextGlyphs(CanvasModel.FONT_BADGE, badgeTxt, 64);
        }
        badgeW = Math.ceil(nd._shapedBadge.totalW) + 10;
        const bx = nd.x + nd.w - badgeW - CARD.pad, by = nd.y + CARD.pad;
        EditorWebgl.pushBorderedBoxQuads(bx, by, badgeW, 17, 1, nd.isStart ? theme.textPrimary : theme.borderStrong, theme.bgElevated);
        EditorWebgl.pushShapedTextQuads(nd._shapedBadge, bx + 5, by + 8.5, nd.isStart ? theme.textPrimary : theme.textSecondary);
      }

      let radioReserveW = 0;
      if (nd.branchRole === "primary" || nd.branchRole === "secondary") {
        const rx = nd.x + nd.w - (badgeW > 0 ? badgeW + 8 + 14 : 16), ry = nd.y + 13;
        nd.branchRadioX = rx; nd.branchRadioY = ry; radioReserveW = 16;
        EditorWebgl.pushSolidQuad(rx, ry, 8, 8, nd.branchRole === "primary" ? theme.textPrimary : theme.borderStrong);
      }

      if (gState.editingNodeTitle !== nd) {
        const maxTitleW = nd.w - 36 - (badgeW > 0 ? badgeW + 6 : 0) - radioReserveW;
        const rawTitle = nd.name || nd.id;
        if (nd._shapedTitleTxt !== rawTitle || nd._shapedTitleMaxW !== maxTitleW || !nd._shapedTitle) {
          nd._shapedTitleTxt = rawTitle;
          nd._shapedTitleMaxW = maxTitleW;
          nd._shapedTitle = EditorWebgl.shapeTextGlyphs(CanvasModel.FONT_TITLE, rawTitle, maxTitleW);
        }
        EditorWebgl.pushShapedTextQuads(nd._shapedTitle, nd.x + 28, nd.y + 17, textPri);
      }

      const choicesTop = nd.y + CARD.headerH;
      EditorWebgl.pushSolidQuad(nd.x + 1, choicesTop, nd.w - 2, nd.h - CARD.headerH - 1, theme.borderSubtle);

      if (nd.hasChoices && nd.ports.length > 0) {
        for (let j = 0; j < nd.ports.length; j++) {
          this.pushNodeChoiceRowQuads(nd, j, isHistoryPreview, false, theme);
        }
      }

      const footerTop = nd.y + nd.h - CARD.footerH;
      EditorWebgl.pushSolidQuad(nd.x + 1, footerTop + 1, nd.w - 2, CARD.footerH - 2, cardBg);
      const btnY = footerTop + CARD.pad;
      if (!isHistoryPreview) {
        this.pushCenteredFooterSprite(CanvasModel.getNodeFooterBtnX(nd, "add", false), btnY, branchPlusSp, textSec);
        this.pushCenteredFooterSprite(CanvasModel.getNodeFooterBtnX(nd, "delete", false), btnY, trashSp, textSec);
      }
      this.pushCenteredFooterSprite(CanvasModel.getNodeFooterBtnX(nd, "edit", isHistoryPreview), btnY, editPenSp, textSec);
    }

    for (let i = 0; i < chapters.length; i++) {
      const ch = chapters[i];
      if (!ch.hidden && ch.right + 12 >= fMinX && ch.left - 12 <= fMaxX && ch.bottom + 12 >= fMinY && ch.top - 12 <= fMaxY) {
        this.pushChapterPortsQuads(ch, theme);
      }
    }

    for (let i = 0; i < nodes.length; i++) {
      const nd = nodes[i];
      if (nd.right + 12 >= fMinX && nd.left - 12 <= fMaxX && nd.bottom + 12 >= fMinY && nd.top - 12 <= fMaxY) {
        this.pushNodePortsQuads(nd, theme);
      }
    }

    studioGL.staticInstCount = studioGL.instCursor;
  },

  appendDynamicOverlayQuads(theme) {
    const studioGL = EditorWebgl.studioGL;
    const gState = CanvasModel.studioGraphState;
    const CARD = CanvasModel.CARD, UV = CanvasModel.UV, GRID = CanvasModel.EDITOR_GRID_SIZE;
    studioGL.instCursor = studioGL.staticInstCount;

    const hovered = gState.hoveredItem;
    const gesture = CanvasModel.activeCanvasGesture;
    const gripSp = studioGL.uv.grip;

    if (gesture && gesture.type === "chapter-drag" && gesture.chapter && gesture.dragConfirmed) {
      const ch = gesture.chapter;
      EditorWebgl.pushSolidBorderQuads(ch.x, ch.y, ch.w, ch.h, 1, theme.textPrimary);
      this.pushChapterPortsQuads(ch, theme);
    }

    const isNodeDragging = Boolean(gesture && gesture.type === "node-drag");
    const nodeHighlightColor = isNodeDragging ? theme.textPrimary : theme.textSecondary;
    CanvasModel.selectedNodeIds.forEach((selId) => {
      const nd = gState.nodeById.get(selId);
      if (!nd) return;
      EditorWebgl.pushSolidBorderQuads(nd.x, nd.y, nd.w, nd.h, 1, nodeHighlightColor);
      this.pushNodePortsQuads(nd, theme);
    });

    if (gesture && (gesture.type === "wire-connect" || gesture.type === "chapter-wire-connect")) {
      const ptsTemp = EditorWires.simplifyOrthogonalPoints(
        EditorWires.buildWireWaypoints(gesture.startX, gesture.startY, gesture.curX, gesture.curY, null, 0, null, null, Boolean(gesture.isBottomPort))
      );
      EditorWebgl.pushWirePolylineQuads(ptsTemp, 2, true, theme.textPrimary);
    }

    if (hovered) {
      if (hovered.type === "scene-out-port") {
        EditorWebgl.pushPortQuads(hovered.x, hovered.y, EditorWebgl.isHoveredPortItemConnected(hovered), true, theme, false, Boolean(hovered.port && hovered.port.isConditional));
      } else if (hovered.type === "chapter-out-port") {
        EditorWebgl.pushSmallPortQuads(hovered.x, hovered.y, EditorWebgl.isHoveredPortItemConnected(hovered), true, theme, false);
      } else if (hovered.type === "wire-pin" && hovered.wire && hovered.pinIdx !== undefined) {
        EditorWebgl.pushWirePinArrowQuads(hovered.wire, hovered.pinIdx, true, theme, false);
      } else if (hovered.type === "node-grip" && hovered.node) {
        const nd = hovered.node;
        EditorWebgl.pushQuadInstance(nd.x + 4, nd.y + 6, gripSp.w, gripSp.h, gripSp.u0 + UV.solid, gripSp.v0, gripSp.u1 + UV.solid, gripSp.v1, theme.textPrimary);
      }
    }

    if (gesture && gesture.type === "node-drag" && gesture.moved && gesture.ghostState) {
      const gs = gesture.ghostState;
      const cells = gs.collisionCells || [];
      for (let i = 0; i < cells.length; i++) {
        const cell = cells[i];
        EditorWebgl.pushSolidQuad(cell.x + 1, cell.y + 1, GRID - 2, GRID - 2, theme.collisionCellFill);
        EditorWebgl.pushSolidBorderQuads(cell.x, cell.y, GRID, GRID, 1, theme.collisionCellBorder);
      }
      const rects = gs.rects || [];
      const borderColor = gs.valid ? theme.ghostValidBorder : theme.ghostInvalidBorder;
      for (let i = 0; i < rects.length; i++) {
        const gr = rects[i];
        EditorWebgl.pushSolidQuad(gr.x, gr.y, gr.w, gr.h, theme.ghostFill);
        EditorWebgl.pushDashedRectQuads(gr.x, gr.y, gr.w, gr.h, 1.5, 8.0, borderColor);
      }
    }

    if (gesture && gesture.type === "marquee-select" && gesture.moved && gesture.marqueeRect) {
      const mr = gesture.marqueeRect;
      EditorWebgl.pushQuadInstance(mr.x, mr.y, mr.w, mr.h, UV.screen, 0.0, UV.screen, 0.0, theme.marqueeFill);
      const t = 1;
      EditorWebgl.pushQuadInstance(mr.x, mr.y, mr.w, t, UV.screen, 0.0, UV.screen, 0.0, theme.textPrimary);
      EditorWebgl.pushQuadInstance(mr.x, mr.y + mr.h - t, mr.w, t, UV.screen, 0.0, UV.screen, 0.0, theme.textPrimary);
      EditorWebgl.pushQuadInstance(mr.x, mr.y, t, mr.h, UV.screen, 0.0, UV.screen, 0.0, theme.textPrimary);
      EditorWebgl.pushQuadInstance(mr.x + mr.w - t, mr.y, t, mr.h, UV.screen, 0.0, UV.screen, 0.0, theme.textPrimary);
    }

    studioGL.totalInstCount = studioGL.instCursor;
  },

  render() {
    const sceneModal = document.querySelector("#modal-scene-text");
    if (sceneModal && (sceneModal.classList.contains("mode-full") || sceneModal.classList.contains("has-right-panel"))) {
      return;
    }
    const studioGL = EditorWebgl.studioGL;
    const gState = CanvasModel.studioGraphState;
    const state = CanvasModel.editorCanvasState;
    const canvas = document.querySelector("#studio-canvas");
    if (!canvas) return;
    const viewport = document.querySelector("#timeline-scroll-viewport");
    if (viewport) {
      const vRect = viewport.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      const vw = Math.max(320, Math.round(vRect.width || 1200));
      const vh = Math.max(240, Math.round(vRect.height || 700));
      gState.viewW = vw;
      gState.viewH = vh;
      gState.canvasDpr = dpr;
      const targetW = Math.round(vw * dpr), targetH = Math.round(vh * dpr);
      if (canvas.width !== targetW || canvas.height !== targetH) {
        canvas.width = targetW;
        canvas.height = targetH;
      }
    }
    const gl = EditorWebgl.initContext(canvas);
    if (!gl) return;

    if (studioGL.uiAtlasImgLoaded && !studioGL.uiAtlasUploaded && studioGL.uiAtlasImg) {
      gl.activeTexture(gl.TEXTURE0);
      gl.bindTexture(gl.TEXTURE_2D, studioGL.uiTex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, studioGL.uiAtlasImg);
      studioGL.uiAtlasUploaded = true;
    }
    if (studioGL.atlasImgLoaded && !studioGL.atlasUploaded && studioGL.atlasImg) {
      gl.activeTexture(gl.TEXTURE1);
      gl.bindTexture(gl.TEXTURE_2D, studioGL.fontTex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, studioGL.atlasImg);
      studioGL.atlasUploaded = true;
    }

    const theme = EditorWebgl.resolveThemeColors();
    const vw = gState.viewW || 1200, vh = gState.viewH || 700, dpr = gState.canvasDpr || 2.0;
    const z = state.zoom || 1.0, px = state.panX || 0, py = state.panY || 0;
    const visMinX = -px / z, visMinY = -py / z, visMaxX = (vw - px) / z, visMaxY = (vh - py) / z;

    if (!gState.staticBatchDirty && (!studioGL.hasCullBounds || visMinX < studioGL.cullMinX || visMaxX > studioGL.cullMaxX || visMinY < studioGL.cullMinY || visMaxY > studioGL.cullMaxY)) {
      gState.staticBatchDirty = true;
    }

    const fpi = EditorWebgl.FLOATS_PER_INSTANCE, bpi = EditorWebgl.BYTES_PER_INSTANCE;
    if (gState.staticBatchDirty) {
      this.compileStaticSceneQuads(theme);
      this.appendDynamicOverlayQuads(theme);
      const ringIdx = ((studioGL.vboRingIdx || 0) + 1) % 3;
      studioGL.vboRingIdx = ringIdx;
      const vbo = studioGL.instVbos[ringIdx];
      studioGL.bindInstVboAttribs(vbo);
      const activeFloats = studioGL.totalInstCount * fpi;
      const requiredBytes = (studioGL.totalInstCount + 64) * bpi;
      if (studioGL.instVboCapacities[ringIdx] < requiredBytes) {
        const nextCapBytes = Math.max(requiredBytes * 2, 4096 * bpi);
        gl.bufferData(gl.ARRAY_BUFFER, nextCapBytes, gl.STREAM_DRAW);
        studioGL.instVboCapacities[ringIdx] = nextCapBytes;
      }
      gl.bufferSubData(gl.ARRAY_BUFFER, 0, EditorWebgl.instanceFloatView.subarray(0, activeFloats));
      gState.staticBatchDirty = false;
      gState.overlayBatchDirty = false;
    } else if (gState.overlayBatchDirty) {
      this.appendDynamicOverlayQuads(theme);
      const dynCount = studioGL.totalInstCount - studioGL.staticInstCount;
      if (dynCount > 0) {
        const ringIdx = studioGL.vboRingIdx || 0;
        const vbo = studioGL.instVbos[ringIdx];
        const startFloat = studioGL.staticInstCount * fpi;
        const endFloat = studioGL.totalInstCount * fpi;
        const requiredBytes = studioGL.totalInstCount * bpi;
        gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
        if (studioGL.instVboCapacities[ringIdx] < requiredBytes) {
          const nextCapBytes = Math.max(requiredBytes * 2, 4096 * bpi);
          gl.bufferData(gl.ARRAY_BUFFER, nextCapBytes, gl.STREAM_DRAW);
          studioGL.instVboCapacities[ringIdx] = nextCapBytes;
          gl.bufferSubData(gl.ARRAY_BUFFER, 0, EditorWebgl.instanceFloatView.subarray(0, endFloat));
        } else {
          gl.bufferSubData(gl.ARRAY_BUFFER, studioGL.staticInstCount * bpi, EditorWebgl.instanceFloatView.subarray(startFloat, endFloat));
        }
      }
      gState.overlayBatchDirty = false;
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
    const gridStep = z < 0.45 ? CanvasModel.EDITOR_GRID_SIZE * 2 : CanvasModel.EDITOR_GRID_SIZE;
    if (studioGL.lastUniformGridStep !== gridStep) {
      gl.uniform1f(studioGL.uGridStep, gridStep);
      studioGL.lastUniformGridStep = gridStep;
    }

    if (studioGL.instExt) {
      studioGL.instExt.drawArraysInstancedANGLE(gl.TRIANGLES, 0, 6, studioGL.totalInstCount);
    }
  }
};
Dispatcher.register("EditorWebglScene", EditorWebglScene);
