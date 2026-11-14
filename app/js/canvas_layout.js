"use strict";

const CanvasLayout = {
  buildSpatialGridForNodes(nodeList) {
    const grid = new Map();
    if (!nodeList) return grid;
    for (let i = 0; i < nodeList.length; i++) {
      this.insertNodeIntoSpatialGrid(grid, nodeList[i]);
    }
    nodeList._spatialGrid = grid;
    return grid;
  },

  insertNodeIntoSpatialGrid(grid, nd) {
    if (!grid || !nd || nd.hidden) return;
    const GAP = CanvasModel.GAP;
    const gx0 = CanvasModel.toSpatialCell(nd.left - GAP.node);
    const gx1 = CanvasModel.toSpatialCell(nd.right + GAP.node);
    const gy0 = CanvasModel.toSpatialCell(nd.top - GAP.node);
    const gy1 = CanvasModel.toSpatialCell(nd.bottom + GAP.node);
    nd._gridMinX = gx0;
    nd._gridMaxX = gx1;
    nd._gridMinY = gy0;
    nd._gridMaxY = gy1;
    for (let gy = gy0; gy <= gy1; gy++) {
      const rowMask = (gy & 0xffff) << 16;
      for (let gx = gx0; gx <= gx1; gx++) {
        const key = rowMask | (gx & 0xffff);
        const cell = grid.get(key);
        if (!cell) grid.set(key, [nd]);
        else cell.push(nd);
      }
    }
  },

  removeNodeFromSpatialGrid(grid, nd) {
    if (!grid || !nd || nd._gridMinX === undefined) return;
    for (let gy = nd._gridMinY; gy <= nd._gridMaxY; gy++) {
      const rowMask = (gy & 0xffff) << 16;
      for (let gx = nd._gridMinX; gx <= nd._gridMaxX; gx++) {
        const cell = grid.get(rowMask | (gx & 0xffff));
        if (cell) {
          const idx = cell.indexOf(nd);
          if (idx >= 0) {
            cell[idx] = cell[cell.length - 1];
            cell.pop();
          }
        }
      }
    }
  },

  updateNodeInSpatialGrids(nd) {
    const gState = CanvasModel.studioGraphState;
    if (gState.nodeSpatialGrid) {
      this.removeNodeFromSpatialGrid(gState.nodeSpatialGrid, nd);
      this.insertNodeIntoSpatialGrid(gState.nodeSpatialGrid, nd);
    }
    const ch = nd.chapterObj;
    if (ch && ch.nodes && ch.nodes._spatialGrid) {
      this.removeNodeFromSpatialGrid(ch.nodes._spatialGrid, nd);
      this.insertNodeIntoSpatialGrid(ch.nodes._spatialGrid, nd);
    }
  },

  findClosestChapterAtWorldCoords(worldX, worldY) {
    const px = worldX + CanvasModel.CARD.halfW;
    const py = worldY + CanvasModel.GAP.node;
    const chapters = CanvasModel.studioGraphState.chapters;
    let bestCh = "";
    let bestDistSq = Infinity;
    for (let i = 0; i < chapters.length; i++) {
      const ch = chapters[i];
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
  },

  resolveNonOverlappingNodeCoords(baseX, baseY, chapterName, nodeW, nodeH) {
    const CARD = CanvasModel.CARD, GAP = CanvasModel.GAP, PAD = CanvasModel.PAD;
    const w = nodeW || CARD.w;
    const h = nodeH || CARD.baseH;
    const nodes = CanvasModel.studioGraphState.nodes || [];
    const chapters = CanvasModel.studioGraphState.chapters || [];
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
        if (cx < other.right + GAP.node && cx + w + GAP.node > other.left && cy < other.bottom + GAP.node && cy + h + GAP.node > other.top) {
          return true;
        }
      }
      let candLeft = cx - PAD.x, candTop = cy - PAD.top;
      let candRight = cx + w + PAD.x, candBottom = cy + Math.max(h, CARD.minChapterH) + PAD.bottom;
      if (checkChapterUnion && targetCh) {
        if (targetCh.left < candLeft) candLeft = targetCh.left;
        if (targetCh.top < candTop) candTop = targetCh.top;
        if (targetCh.right > candRight) candRight = targetCh.right;
        if (targetCh.bottom > candBottom) candBottom = targetCh.bottom;
      }
      for (let i = 0; i < chapters.length; i++) {
        const ch = chapters[i];
        if (ch.hidden || ch === targetCh || ch.nameLower === chKey || ch.refLower === chKey) continue;
        if (candLeft < ch.right + GAP.chapter && candRight + GAP.chapter > ch.left && candTop < ch.bottom + GAP.chapter && candBottom + GAP.chapter > ch.top) {
          return true;
        }
      }
      return false;
    };

    if (!collidesAt(baseX, baseY, true)) return { x: baseX, y: baseY };
    const unionPasses = [true, false];
    for (let p = 0; p < unionPasses.length; p++) {
      const checkUnion = unionPasses[p];
      for (let step = 1; step <= 48; step++) {
        const stepPx = step * CanvasModel.EDITOR_GRID_SIZE;
        const candidates = [
          [baseX, baseY + stepPx], [baseX + stepPx, baseY], [baseX - stepPx, baseY],
          [baseX, baseY - stepPx], [baseX + stepPx, baseY + stepPx], [baseX - stepPx, baseY + stepPx]
        ];
        for (let c = 0; c < candidates.length; c++) {
          const cx = CanvasModel.snapToEditorGrid(candidates[c][0]);
          const cy = CanvasModel.snapToEditorGrid(candidates[c][1]);
          if (!collidesAt(cx, cy, checkUnion)) return { x: cx, y: cy };
        }
      }
    }
    return { x: baseX, y: baseY };
  },

  resolveValidChapterDragDelta(ch, items, desiredDx, desiredDy, lastValidDx, lastValidDy) {
    const CARD = CanvasModel.CARD, GAP = CanvasModel.GAP, PAD = CanvasModel.PAD, GRID = CanvasModel.EDITOR_GRID_SIZE;
    const chapters = CanvasModel.studioGraphState.chapters || [];
    if (chapters.length <= 1 || !items || items.length === 0) {
      return { dx: desiredDx, dy: desiredDy };
    }
    let baseMinX = Infinity, baseMinY = Infinity, baseMaxX = -Infinity, baseMaxY = -Infinity;
    for (let i = 0; i < items.length; i++) {
      const it = items[i], nd = it.node;
      const nh = Math.max(nd.h || CARD.baseH, CARD.minChapterH);
      if (it.startX < baseMinX) baseMinX = it.startX;
      if (it.startY < baseMinY) baseMinY = it.startY;
      if (it.startX + nd.w > baseMaxX) baseMaxX = it.startX + nd.w;
      if (it.startY + nh > baseMaxY) baseMaxY = it.startY + nh;
    }
    const baseLeft = baseMinX - PAD.x, baseTop = baseMinY - PAD.top;
    const baseRight = baseMaxX + PAD.x, baseBottom = baseMaxY + PAD.bottom;

    const isDeltaValid = (dx, dy) => {
      const l = baseLeft + dx, t = baseTop + dy, r = baseRight + dx, b = baseBottom + dy;
      for (let i = 0; i < chapters.length; i++) {
        const other = chapters[i];
        if (other === ch || other.hidden) continue;
        if (l < other.right + GAP.chapter && r + GAP.chapter > other.left && t < other.bottom + GAP.chapter && b + GAP.chapter > other.top) {
          return false;
        }
      }
      return true;
    };

    if (isDeltaValid(desiredDx, desiredDy)) return { dx: desiredDx, dy: desiredDy };
    let bestDx = lastValidDx, bestDy = lastValidDy, bestDistSq = Infinity;
    const tryCandidate = (cx, cy) => {
      if (!isDeltaValid(cx, cy)) return;
      const dSq = (cx - desiredDx) ** 2 + (cy - desiredDy) ** 2;
      if (dSq < bestDistSq) { bestDistSq = dSq; bestDx = cx; bestDy = cy; }
    };
    tryCandidate(desiredDx, lastValidDy);
    tryCandidate(lastValidDx, desiredDy);
    const stepX = desiredDx >= lastValidDx ? -GRID : GRID;
    for (let cx = desiredDx; cx !== lastValidDx; cx += stepX) {
      tryCandidate(cx, desiredDy);
      tryCandidate(cx, lastValidDy);
      if (Math.abs(cx - desiredDx) > 960) break;
    }
    const stepY = desiredDy >= lastValidDy ? -GRID : GRID;
    for (let cy = desiredDy; cy !== lastValidDy; cy += stepY) {
      tryCandidate(desiredDx, cy);
      tryCandidate(lastValidDx, cy);
      if (Math.abs(cy - desiredDy) > 960) break;
    }
    return { dx: bestDx, dy: bestDy };
  },

  evaluateNodeDragGhost(items, desiredDx, desiredDy) {
    const CARD = CanvasModel.CARD, GAP = CanvasModel.GAP, PAD = CanvasModel.PAD, GRID = CanvasModel.EDITOR_GRID_SIZE;
    const dx = CanvasModel.snapToEditorGrid(desiredDx || 0);
    const dy = CanvasModel.snapToEditorGrid(desiredDy || 0);
    const nodes = CanvasModel.studioGraphState.nodes || [];
    const chapters = CanvasModel.studioGraphState.chapters || [];
    if (!items || items.length === 0) {
      return { dx: dx, dy: dy, valid: true, rects: [], collisionCells: [] };
    }

    for (let i = 0; i < items.length; i++) items[i].node._isDragging = true;

    const affectedChapters = [];
    for (let i = 0; i < items.length; i++) {
      const ch = items[i].node.chapterObj;
      if (ch && affectedChapters.indexOf(ch) === -1) {
        let sMinX = Infinity, sMinY = Infinity, sMaxX = -Infinity, sMaxY = -Infinity;
        let iMinX = Infinity, iMinY = Infinity, iMaxX = -Infinity, iMaxY = -Infinity;
        const chNodes = ch.nodes || [];
        for (let j = 0; j < chNodes.length; j++) {
          const nd = chNodes[j], nh = Math.max(nd.h || CARD.baseH, CARD.minChapterH);
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
        ch._staticMinX = sMinX; ch._staticMinY = sMinY; ch._staticMaxX = sMaxX; ch._staticMaxY = sMaxY;
        ch._initialLeft = iMinX - PAD.x; ch._initialTop = iMinY - PAD.top;
        ch._initialRight = iMaxX + PAD.x; ch._initialBottom = iMaxY + PAD.bottom;
        affectedChapters.push(ch);
      }
    }

    const rects = [], collisionCells = [], seenCellKeys = new Set();
    let hasCollision = false;
    const addCollisionCell = (cx, cy) => {
      const key = cx + "," + cy;
      if (!seenCellKeys.has(key)) { seenCellKeys.add(key); collisionCells.push({ x: cx, y: cy }); }
    };

    for (let i = 0; i < items.length; i++) {
      const it = items[i], nd = it.node;
      const gx = CanvasModel.snapToEditorGrid(it.startX + dx), gy = CanvasModel.snapToEditorGrid(it.startY + dy);
      const gw = nd.w || CARD.w, gh = Math.ceil((nd.h || CARD.baseH) / GRID) * GRID;
      const r = { node: nd, x: gx, y: gy, w: gw, h: gh, right: gx + gw, bottom: gy + gh };
      rects.push(r);

      for (let j = 0; j < nodes.length; j++) {
        const other = nodes[j];
        if (other._isDragging || other.hidden) continue;
        const fLeft = other.left - GAP.node, fRight = other.right + GAP.node;
        const fTop = other.top - GAP.node, fBottom = other.bottom + GAP.node;
        if (r.x < fRight && r.right > fLeft && r.y < fBottom && r.bottom > fTop) {
          hasCollision = true;
          const ix0 = Math.max(r.x, Math.floor(fLeft / GRID) * GRID);
          const ix1 = Math.min(r.right, Math.ceil(fRight / GRID) * GRID);
          const iy0 = Math.max(r.y, Math.floor(fTop / GRID) * GRID);
          const iy1 = Math.min(r.bottom, Math.ceil(fBottom / GRID) * GRID);
          for (let cy = iy0; cy < iy1; cy += GRID) {
            for (let cx = ix0; cx < ix1; cx += GRID) addCollisionCell(cx, cy);
          }
        }
      }
    }

    for (let c = 0; c < affectedChapters.length; c++) {
      const ch = affectedChapters[c];
      let minX = ch._staticMinX, minY = ch._staticMinY, maxX = ch._staticMaxX, maxY = ch._staticMaxY;
      for (let k = 0; k < rects.length; k++) {
        const r = rects[k];
        if (r.node.chapterObj !== ch) continue;
        const nh = Math.max(r.h, CARD.minChapterH);
        if (r.x < minX) minX = r.x;
        if (r.y < minY) minY = r.y;
        if (r.right > maxX) maxX = r.right;
        if (r.y + nh > maxY) maxY = r.y + nh;
      }
      const cLeft = minX - PAD.x, cTop = minY - PAD.top, cRight = maxX + PAD.x, cBottom = maxY + PAD.bottom;

      for (let o = 0; o < chapters.length; o++) {
        const otherCh = chapters[o];
        if (otherCh === ch || otherCh.hidden) continue;
        if (cLeft < otherCh.right + GAP.chapter && cRight + GAP.chapter > otherCh.left && cTop < otherCh.bottom + GAP.chapter && cBottom + GAP.chapter > otherCh.top) {
          const wasOverlapping = ch._initialLeft < otherCh.right + GAP.chapter && ch._initialRight + GAP.chapter > otherCh.left && ch._initialTop < otherCh.bottom + GAP.chapter && ch._initialBottom + GAP.chapter > otherCh.top;
          const expandedOutward = cLeft < ch._initialLeft || cRight > ch._initialRight || cTop < ch._initialTop || cBottom > ch._initialBottom;
          if (!wasOverlapping || expandedOutward) {
            hasCollision = true;
            for (let k = 0; k < rects.length; k++) {
              const r = rects[k];
              if (r.node.chapterObj !== ch) continue;
              for (let cy = r.y; cy < r.bottom; cy += GRID) {
                for (let cx = r.x; cx < r.right; cx += GRID) {
                  if (cx - PAD.x < otherCh.right + GAP.chapter && cx + GRID + PAD.x > otherCh.left && cy - PAD.top < otherCh.bottom + GAP.chapter && cy + GRID + PAD.bottom > otherCh.top) {
                    addCollisionCell(cx, cy);
                  }
                }
              }
            }
          }
        }
      }
    }

    for (let i = 0; i < items.length; i++) items[i].node._isDragging = false;
    return { dx: dx, dy: dy, valid: !hasCollision && collisionCells.length === 0, rects: rects, collisionCells: collisionCells };
  },

  computeChapterFrameRect(matching) {
    const CARD = CanvasModel.CARD, PAD = CanvasModel.PAD;
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
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
    return { x: minX - PAD.x, y: minY - PAD.top, w: maxX - minX + PAD.w, h: maxY - minY + PAD.h };
  },

  updateChapterGroupFrames(onlyChaptersSet, hasInterChapterPortMove) {
    const gState = CanvasModel.studioGraphState;
    const nodes = gState.nodes;
    const chapters = gState.chapters;
    if (!nodes || nodes.length === 0) return null;

    if (onlyChaptersSet && onlyChaptersSet.size > 0) {
      const changedChapters = new Set();
      onlyChaptersSet.forEach((ch) => {
        const matching = ch.nodes;
        if (!matching || matching.length === 0) return;
        const rect = this.computeChapterFrameRect(matching);
        const frameChanged = ch.x !== rect.x || ch.y !== rect.y || ch.w !== rect.w || ch.h !== rect.h;
        const prevInX = ch.inX, prevInY = ch.inY;
        ch.x = rect.x; ch.y = rect.y; ch.w = rect.w; ch.h = rect.h;
        CanvasModel.recomputeChapterInputPortPosition(ch);
        if (frameChanged || ch.inX !== prevInX || ch.inY !== prevInY || hasInterChapterPortMove) {
          CanvasModel.recomputeChapterPortPositions(ch);
          changedChapters.add(ch);
        }
      });
      gState.staticBatchDirty = true;
      return changedChapters;
    }

    gState.nodeSpatialGrid = this.buildSpatialGridForNodes(nodes);
    const byChapter = new Map();
    for (let i = 0; i < nodes.length; i++) {
      const nd = nodes[i];
      const key = (nd.chapter || AppShellController.tr("default_chapter")).trim().toLowerCase();
      let list = byChapter.get(key);
      if (!list) { list = []; byChapter.set(key, list); }
      list.push(nd);
    }

    byChapter.forEach((memberNodes, key) => {
      let ch = gState.chapterByKey.get(key);
      if (!ch) {
        const displayCh = memberNodes[0].chapter || AppShellController.tr("default_chapter");
        ch = {
          id: "ch:" + key, rawId: key, ref: displayCh, refLower: key, name: displayCh, nameLower: key,
          isRoot: chapters.length === 0, startSceneId: memberNodes[0].id,
          x: 0, y: 0, w: 240, h: 140, outPorts: [{ idx: 0, target: "", targetLower: "", x: 0, y: 0 }], hidden: false
        };
        chapters.push(ch);
        gState.chapterByKey.set(key, ch);
      }
    });

    for (let i = 0; i < chapters.length; i++) {
      const ch = chapters[i];
      const matching = byChapter.get(ch.nameLower) || byChapter.get(ch.refLower) || [];
      ch.nodes = matching;
      this.buildSpatialGridForNodes(matching);
      if (matching.length === 0) { ch.hidden = true; continue; }
      ch.hidden = false;
      for (let j = 0; j < matching.length; j++) matching[j].chapterObj = ch;
      const rect = this.computeChapterFrameRect(matching);
      ch.x = rect.x; ch.y = rect.y; ch.w = rect.w; ch.h = rect.h;
      CanvasModel.recomputeChapterInputPortPosition(ch);
    }
    for (let i = 0; i < chapters.length; i++) {
      if (!chapters[i].hidden) CanvasModel.recomputeChapterPortPositions(chapters[i]);
    }
    gState.staticBatchDirty = true;
    return null;
  },

  applyIncrementalNodeMove(items, snappedDx, snappedDy) {
    const dirtyNodeIds = new Set(), affectedChaptersSet = new Set(), dirtyAabbs = [];
    let hasInterChapterPortMove = false;
    for (let i = 0; i < items.length; i++) {
      const nd = items[i].node;
      dirtyNodeIds.add(nd.id);
      dirtyAabbs.push({ minX: nd.left - 24, minY: nd.top - 24, maxX: nd.right + 24, maxY: nd.bottom + 24 });
      const ch = nd.chapterObj || CanvasModel.studioGraphState.chapterByKey.get(nd.chapterLower);
      if (ch) affectedChaptersSet.add(ch);
      if (!hasInterChapterPortMove) {
        if (nd.implicitPort && nd.implicitPort.targetChapter) hasInterChapterPortMove = true;
        else if (nd.ports) {
          for (let k = 0; k < nd.ports.length; k++) {
            if (nd.ports[k].targetChapter) { hasInterChapterPortMove = true; break; }
          }
        }
      }
      nd.x = CanvasModel.snapToEditorGrid(items[i].startX + snappedDx);
      nd.y = CanvasModel.snapToEditorGrid(items[i].startY + snappedDy);
      CanvasModel.recomputeNodeMetrics(nd);
      this.updateNodeInSpatialGrids(nd);
      dirtyAabbs.push({ minX: nd.left - 24, minY: nd.top - 24, maxX: nd.right + 24, maxY: nd.bottom + 24 });
    }
    const changedChapterFramesSet = this.updateChapterGroupFrames(affectedChaptersSet, hasInterChapterPortMove);
    EditorWires.draw(dirtyNodeIds, dirtyAabbs, changedChapterFramesSet);
  }
};
Dispatcher.register("CanvasLayout", CanvasLayout);
