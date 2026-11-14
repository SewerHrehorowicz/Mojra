"use strict";

const EditorWires = {
  ptsList() {
    const out = [];
    for (let i = 0; i < arguments.length; i += 2) {
      out.push({ x: arguments[i], y: arguments[i + 1] });
    }
    return out;
  },

  simplifyOrthogonalPoints(points) {
    if (!points || points.length === 0) return [];
    const pts = [{ x: points[0].x, y: points[0].y }];
    for (let i = 1; i < points.length; i++) {
      const prev = pts[pts.length - 1];
      const cur = { x: points[i].x, y: points[i].y };
      if (Math.abs(cur.x - prev.x) < 0.5 && Math.abs(cur.y - prev.y) < 0.5) continue;
      if (pts.length >= 2) {
        const prev2 = pts[pts.length - 2];
        const sameX = Math.abs(prev2.x - prev.x) < 0.5 && Math.abs(prev.x - cur.x) < 0.5;
        const sameY = Math.abs(prev2.y - prev.y) < 0.5 && Math.abs(prev.y - cur.y) < 0.5;
        if (sameX || sameY) {
          pts[pts.length - 1] = cur;
          continue;
        }
      }
      pts.push(cur);
    }
    return pts;
  },

  testObstacleHit(b, isVert, fixedCoord, minSpan, maxSpan, p, ignore1, ignore2) {
    if (b.hidden) return 0;
    const isEndpoint = b === ignore1 || b === ignore2 || Boolean(b.id && (b.id === ignore1 || b.id === ignore2));
    if (isVert) {
      if (isEndpoint) {
        return fixedCoord > b.left - 6 && fixedCoord < b.right + 6 && maxSpan > b.top + 3 && minSpan < b.bottom - 3 ? 2 : 0;
      }
      return fixedCoord > b.left - p && fixedCoord < b.right + p && maxSpan > b.top - p && minSpan < b.bottom + p ? 1 : 0;
    }
    if (isEndpoint) {
      return fixedCoord > b.top - 7 && fixedCoord < b.bottom + 7 && maxSpan > b.left + 3 && minSpan < b.right - 3 ? 2 : 0;
    }
    return fixedCoord > b.top - p && fixedCoord < b.bottom + p && maxSpan > b.left - p && minSpan < b.right + p ? 1 : 0;
  },

  linecastOrthogonal(isVert, fixedCoord, spanA, spanB, obstacles, ignore1, ignore2, pad) {
    if (!obstacles || obstacles.length === 0) return null;
    const minSpan = spanA < spanB ? spanA : spanB;
    const maxSpan = spanA > spanB ? spanA : spanB;
    if (maxSpan - minSpan < 2) return null;
    const p = pad !== undefined ? pad : CanvasModel.WIRE.tightPad;

    const grid = obstacles._spatialGrid;
    if (grid && obstacles.length > CanvasModel.SPATIAL.minObstacles) {
      const gx0 = CanvasModel.toSpatialCell(isVert ? fixedCoord - p : minSpan - p);
      const gx1 = CanvasModel.toSpatialCell(isVert ? fixedCoord + p : maxSpan + p);
      const gy0 = CanvasModel.toSpatialCell(isVert ? minSpan - p : fixedCoord - p);
      const gy1 = CanvasModel.toSpatialCell(isVert ? maxSpan + p : fixedCoord + p);
      if ((gx1 - gx0 + 1) * (gy1 - gy0 + 1) < obstacles.length) {
        let bestHit = null, bestDist = Infinity;
        for (let gy = gy0; gy <= gy1; gy++) {
          const rowMask = (gy & 0xffff) << 16;
          for (let gx = gx0; gx <= gx1; gx++) {
            const cell = grid.get(rowMask | (gx & 0xffff));
            if (!cell) continue;
            for (let i = 0; i < cell.length; i++) {
              const b = cell[i];
              const hitKind = this.testObstacleHit(b, isVert, fixedCoord, minSpan, maxSpan, p, ignore1, ignore2);
              if (hitKind === 2) return b;
              if (hitKind === 1) {
                const dist = Math.abs((isVert ? b.top : b.left) - spanA);
                if (dist < bestDist) { bestDist = dist; bestHit = b; }
              }
            }
          }
        }
        return bestHit;
      }
    }

    for (let i = 0; i < obstacles.length; i++) {
      const b = obstacles[i];
      if (this.testObstacleHit(b, isVert, fixedCoord, minSpan, maxSpan, p, ignore1, ignore2) > 0) return b;
    }
    return null;
  },

  linecastVertical(x, yA, yB, obstacles, ignore1, ignore2, pad) {
    return this.linecastOrthogonal(true, x, yA, yB, obstacles, ignore1, ignore2, pad);
  },

  linecastHorizontal(y, xA, xB, obstacles, ignore1, ignore2, pad) {
    return this.linecastOrthogonal(false, y, xA, xB, obstacles, ignore1, ignore2, pad);
  },

  resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, dir, p) {
    let cur = coord;
    for (let iter = 0; iter < 12; iter++) {
      const hit = this.linecastOrthogonal(isVert, cur, spanA, spanB, obstacles, ignoreId1, ignoreId2, p - 2);
      if (!hit) return { coord: cur, clear: true };
      cur = dir < 0 ? (isVert ? hit.left : hit.top) - p : (isVert ? hit.right : hit.bottom) + p;
    }
    return {
      coord: cur,
      clear: !this.linecastOrthogonal(isVert, cur, spanA, spanB, obstacles, ignoreId1, ignoreId2, p - 2)
    };
  },

  resolveClearCoord(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
    const p = pad !== undefined ? pad : CanvasModel.WIRE.pad;
    if (!this.linecastOrthogonal(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, p - 2)) {
      return coord;
    }
    const dir = preferDir || 0;
    if (dir !== 0) {
      const primary = this.resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, dir, p);
      if (primary.clear) return primary.coord;
      const fallback = this.resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, -dir, p);
      return fallback.clear ? fallback.coord : primary.coord;
    }
    const negRes = this.resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, -1, p);
    const posRes = this.resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, 1, p);
    if (negRes.clear && !posRes.clear) return negRes.coord;
    if (posRes.clear && !negRes.clear) return posRes.coord;
    return Math.abs(negRes.coord - coord) <= Math.abs(posRes.coord - coord) ? negRes.coord : posRes.coord;
  },

  resolveClearX(x, yA, yB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
    return this.resolveClearCoord(true, x, yA, yB, obstacles, ignoreId1, ignoreId2, preferDir, pad);
  },

  resolveClearY(y, xA, xB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
    return this.resolveClearCoord(false, y, xA, xB, obstacles, ignoreId1, ignoreId2, preferDir, pad);
  },

  buildWireWaypoints(x1, y1, x2, y2, fromBottom, portIdx, toLeft, toRight, isBottomPort, obstacles, fromId, toId, totalPorts, preferredOutX) {
    const WIRE = CanvasModel.WIRE, WIRE_GRID = CanvasModel.WIRE_GRID, WIRE_LANE_STEP = CanvasModel.WIRE_LANE_STEP, CARD = CanvasModel.CARD;
    const idx = portIdx || 0;
    const pCount = Math.max(1, totalPorts || idx + 1);
    const obs = obstacles || [];

    if (isBottomPort) {
      if (y2 >= y1 + 12) {
        if (Math.abs(x2 - x1) < 2) {
          const hit = this.linecastVertical(x1, y1 + 4, y2 - 4, obs, fromId, toId, WIRE.tightPad);
          if (!hit) return this.ptsList(x1, y1, x2, y2);
          const sideDir = x1 <= (hit.left + hit.right) * 0.5 ? -1 : 1;
          const detourX = this.resolveClearX(sideDir < 0 ? hit.left - 18 : hit.right + 18, hit.top - 16, hit.bottom + 16, obs, fromId, toId, sideDir, 16);
          const topY = Math.max(y1 + 12, this.resolveClearY(hit.top - 16, x1, detourX, obs, fromId, toId, -1, WIRE.pad));
          const botY = Math.min(y2 - 12, this.resolveClearY(hit.bottom + 16, detourX, x2, obs, fromId, toId, 1, WIRE.pad));
          return this.ptsList(x1, y1, x1, topY, detourX, topY, detourX, botY, x2, botY, x2, y2);
        }
        const rawMidY = fromBottom !== null && fromBottom !== undefined && fromBottom > y1 && fromBottom < y2 - WIRE_GRID ? (fromBottom + y2) * 0.5 : (y1 + y2) * 0.5;
        let midY = this.resolveClearY(Math.max(y1 + 16, Math.min(y2 - 16, CanvasModel.snapToWireGrid(rawMidY))), x1, x2, obs, fromId, toId, 1, WIRE.pad);
        midY = Math.max(y1 + 12, Math.min(y2 - 12, midY));
        if (!this.linecastVertical(x1, y1 + 4, midY, obs, fromId, toId, WIRE.tightPad) && !this.linecastVertical(x2, midY, y2 - 4, obs, fromId, toId, WIRE.tightPad)) {
          return this.ptsList(x1, y1, x1, midY, x2, midY, x2, y2);
        }
        const stepOutY = this.resolveClearY(Math.max(y1 + 18, CanvasModel.snapToWireGrid(y1 + WIRE_GRID)), x1, x2, obs, fromId, toId, 1, WIRE.pad);
        const stepInY = this.resolveClearY(Math.min(y2 - 18, CanvasModel.snapToWireGrid(y2 - WIRE_GRID)), x1, x2, obs, fromId, toId, -1, WIRE.pad);
        const midChannelX = this.resolveClearX(CanvasModel.snapToWireGrid((x1 + x2) * 0.5), stepOutY, stepInY, obs, fromId, toId, 0, 16);
        return this.ptsList(x1, y1, x1, stepOutY, midChannelX, stepOutY, midChannelX, stepInY, x2, stepInY, x2, y2);
      }

      let channelX = CanvasModel.snapToWireGrid((x1 + x2) * 0.5);
      if (toLeft !== undefined && toLeft !== null && toRight !== undefined && toRight !== null) {
        const fromRight = x1 + CARD.halfW, fromLeft = x1 - CARD.halfW;
        if (toLeft > fromRight + 20) channelX = CanvasModel.snapToWireGrid((fromRight + toLeft) * 0.5);
        else if (toRight < fromLeft - 20) channelX = CanvasModel.snapToWireGrid((fromLeft + toRight) * 0.5);
        else channelX = CanvasModel.snapToWireGrid(Math.max(fromRight, toRight) + WIRE_GRID);
      }
      const outY = this.resolveClearY(Math.max(y1 + 18, CanvasModel.snapToWireGrid(y1 + WIRE_GRID)), x1, channelX, obs, fromId, toId, 1, WIRE.pad);
      const baseInY = Math.min(y2 - 18, CanvasModel.snapToWireGrid(y2 - WIRE_GRID));
      const hitInH = this.linecastHorizontal(baseInY, channelX, x2, obs, fromId, toId, 12);
      const inYBottom = this.resolveClearY(baseInY, channelX, x2, obs, fromId, toId, hitInH && hitInH.bottom + 16 <= y2 - 12 ? 1 : -1, WIRE.pad);
      if (this.linecastVertical(channelX, outY, inYBottom, obs, fromId, toId, WIRE.pad)) {
        channelX = this.resolveClearX(x1, outY, inYBottom, obs, fromId, toId, 0, 18);
      }
      return this.ptsList(x1, y1, x1, outY, channelX, outY, channelX, inYBottom, x2, inYBottom, x2, y2);
    }

    const goesDown = fromBottom !== null && fromBottom !== undefined ? y2 > fromBottom + 4 : y2 > y1 + 16;
    const stubRank = goesDown ? Math.max(0, pCount - 1 - idx) : idx;
    const exitStub = WIRE_GRID + stubRank * WIRE_LANE_STEP;
    let outX = typeof preferredOutX === "number" && !isNaN(preferredOutX) ? preferredOutX : x1 + exitStub;
    let inY = y2 - WIRE_GRID;

    if (fromBottom === null || fromBottom === undefined) {
      if (x2 >= outX && y2 >= y1) return this.ptsList(x1, y1, x2, y1, x2, y2);
      const midY = y2 >= y1 + 32 ? CanvasModel.snapToWireGrid((y1 + y2) * 0.5) : inY;
      return this.ptsList(x1, y1, outX, y1, outX, midY, x2, midY, x2, y2);
    }

    if (y2 >= fromBottom + 6) {
      if (x2 >= outX - 1.5 && !this.linecastHorizontal(y1, x1 + 4, x2, obs, fromId, toId, WIRE.tightPad) && !this.linecastVertical(x2, y1, y2 - 4, obs, fromId, toId, WIRE.tightPad)) {
        return this.ptsList(x1, y1, x2, y1, x2, y2);
      }
      const gap = y2 - fromBottom;
      const minClear = Math.min(16, Math.max(8, Math.floor(gap * 0.35)));
      const minMidY = fromBottom + minClear, maxMidY = Math.max(minMidY, y2 - minClear);
      const turnRank = x2 > outX ? pCount - 1 - stubRank : stubRank;
      const baseCorridorY = Math.max(minMidY, Math.min(maxMidY, CanvasModel.snapToWireGrid((fromBottom + y2) * 0.5)));
      const laneShift = pCount > 1 && maxMidY - minMidY >= 6 ? Math.round((turnRank - (pCount - 1) * 0.5) * WIRE_LANE_STEP) : 0;
      let midY = Math.max(minMidY, Math.min(maxMidY, baseCorridorY + laneShift));
      outX = this.resolveClearX(outX, y1, midY, obs, fromId, toId, 1, WIRE.pad);
      midY = this.resolveClearY(midY, outX, x2, obs, fromId, toId, 1, 12);
      const hitDrop = this.linecastVertical(x2, midY, y2 - 4, obs, fromId, toId, WIRE.tightPad);
      if (!hitDrop) return this.ptsList(x1, y1, outX, y1, outX, midY, x2, midY, x2, y2);
      const detourX = this.resolveClearX(x2, midY, y2 - 14, obs, fromId, toId, x2 >= outX ? -1 : 1, 16);
      const entryY = this.resolveClearY(Math.min(y2 - 12, hitDrop.bottom + 14), detourX, x2, obs, fromId, toId, -1, 12);
      return this.ptsList(x1, y1, outX, y1, outX, midY, detourX, midY, detourX, entryY, x2, entryY, x2, y2);
    }

    inY = this.resolveClearY(inY, outX, x2, obs, fromId, toId, -1, WIRE.pad);
    let bypassX = Math.max(outX, (toRight !== undefined && toRight !== null ? toRight : x2) + WIRE_GRID + stubRank * WIRE_LANE_STEP);
    bypassX = this.resolveClearX(bypassX, y1, inY, obs, fromId, toId, 1, 16);
    if (!this.linecastHorizontal(y1, x1 + 4, bypassX, obs, fromId, toId, WIRE.tightPad)) {
      return this.ptsList(x1, y1, bypassX, y1, bypassX, inY, x2, inY, x2, y2);
    }
    const underY = this.resolveClearY(fromBottom + 16 + stubRank * WIRE_LANE_STEP, outX, bypassX, obs, fromId, toId, 1, WIRE.pad);
    outX = this.resolveClearX(outX, y1, underY, obs, fromId, toId, 1, WIRE.pad);
    bypassX = this.resolveClearX(bypassX, underY, inY, obs, fromId, toId, 1, 16);
    return this.ptsList(x1, y1, outX, y1, outX, underY, bypassX, underY, bypassX, inY, x2, inY, x2, y2);
  },

  buildPinnedWireWaypoints(w, autoPtsInput) {
    const pins = w.pins;
    let pts = autoPtsInput && autoPtsInput.length >= 2 ? autoPtsInput.map((pt) => ({ x: pt.x, y: pt.y })) : this.ptsList(w.x1, w.y1, w.x2, w.y2);
    if (!pins || pins.length === 0) return this.simplifyOrthogonalPoints(pts);

    for (let i = 0; i < pins.length; i++) {
      const p = pins[i];
      if (!p || pts.length < 2) continue;
      let s = typeof p.segIdx === "number" && p.segIdx >= 0 && p.segIdx < pts.length - 1 ? p.segIdx : 0;
      const a = pts[s], b = pts[s + 1];
      if (Math.abs(a.y - b.y) <= Math.abs(a.x - b.x)) {
        p.axis = "H";
        pts[s].y = p.y;
        pts[s + 1].y = p.y;
        p.x = Math.round((pts[s].x + pts[s + 1].x) * 0.5);
      } else {
        p.axis = "V";
        pts[s].x = p.x;
        pts[s + 1].x = p.x;
        p.y = Math.round((pts[s].y + pts[s + 1].y) * 0.5);
      }
    }
    return pts;
  },

  getChapterOutputPortSlot(ch, targetChapterLower) {
    const outPorts = ch.outPorts || [];
    let matchIdx = 0;
    if (targetChapterLower) {
      for (let i = 0; i < outPorts.length; i++) {
        if (outPorts[i].targetLower === targetChapterLower) { matchIdx = i; break; }
      }
    }
    return outPorts[matchIdx] || { x: ch.x + ch.w * 0.5, y: ch.y + ch.h, side: "bottom", idx: 0 };
  },

  updateWireBounds(w, pts) {
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (let i = 0; i < pts.length; i++) {
      const p = pts[i];
      if (p.x < minX) minX = p.x;
      if (p.y < minY) minY = p.y;
      if (p.x > maxX) maxX = p.x;
      if (p.y > maxY) maxY = p.y;
    }
    const pad = CanvasModel.WIRE.bboxPad;
    w.minX = minX - pad; w.minY = minY - pad; w.maxX = maxX + pad; w.maxY = maxY + pad;
  },

  routeSingleWireBasePts(w, nodeObstacles, chapterObstacles) {
    if (w.kind === "scene-to-chapter-out") {
      const fromNode = w.fromNode, fromCh = w.fromChapterObj;
      const localNodes = fromCh && fromCh.nodes ? fromCh.nodes : nodeObstacles;
      const outPort = this.getChapterOutputPortSlot(fromCh, w.targetChapterLower);
      w.x1 = w.portRef.x; w.y1 = w.portRef.y; w.x2 = outPort.x; w.y2 = outPort.y;
      w.unreachable = fromNode.unreachable;
      const totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
      const rawPts = this.buildWireWaypoints(w.x1, w.y1, w.x2, w.y2, fromNode.bottom, w.portIdx, outPort.x - 8, outPort.x + 8, w.isBottomPort, localNodes, fromNode, null, totalPorts);
      const autoPts = this.simplifyOrthogonalPoints(rawPts);
      w.basePts = w.pins && w.pins.length > 0 ? this.buildPinnedWireWaypoints(w, autoPts) : autoPts;
    } else if (w.kind === "scene-to-scene") {
      const fromNode = w.fromNode, toNode = w.toNode;
      const chObj = fromNode.chapterObj || toNode.chapterObj;
      const localNodes = chObj && chObj.nodes ? chObj.nodes : nodeObstacles;
      w.x1 = w.portRef.x; w.y1 = w.portRef.y; w.x2 = toNode.inX; w.y2 = toNode.inY;
      w.unreachable = fromNode.unreachable;
      const totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
      const rawPts = this.buildWireWaypoints(w.x1, w.y1, w.x2, w.y2, fromNode.bottom, w.portIdx, toNode.left, toNode.right, w.isBottomPort, localNodes, fromNode, toNode, totalPorts);
      const autoPts = this.simplifyOrthogonalPoints(rawPts);
      w.basePts = w.pins && w.pins.length > 0 ? this.buildPinnedWireWaypoints(w, autoPts) : autoPts;
    } else if (w.kind === "chapter-to-chapter") {
      const fromCh = w.fromChapterObj, toCh = w.toChapterObj, outPort = w.fromChapterOutPort;
      w.isBottomPort = (outPort.side || "bottom") === "bottom";
      w.x1 = outPort.x; w.y1 = outPort.y; w.x2 = toCh.inX; w.y2 = toCh.inY;
      w.unreachable = false;
      const rawPts = this.buildWireWaypoints(w.x1, w.y1, w.x2, w.y2, Math.max(w.y1, w.y2 - CanvasModel.CARD.minChapterH), w.portIdx, toCh.x, toCh.x + toCh.w, true, chapterObstacles, fromCh, toCh, fromCh.outPorts ? fromCh.outPorts.length : 1);
      const autoPts = this.simplifyOrthogonalPoints(rawPts);
      w.basePts = w.pins && w.pins.length > 0 ? this.buildPinnedWireWaypoints(w, autoPts) : autoPts;
    } else if (w.kind === "chapter-in-to-entry") {
      const ch = w.toChapterObj, toNode = w.toNode;
      const localNodes = ch && ch.nodes ? ch.nodes : nodeObstacles;
      w.x1 = ch.inX; w.y1 = ch.inY; w.x2 = toNode.inX; w.y2 = toNode.inY;
      w.unreachable = toNode.unreachable;
      const rawPts = this.buildWireWaypoints(w.x1, w.y1, w.x2, w.y2, ch.y + 28, 0, toNode.left, toNode.right, true, localNodes, null, toNode, 1);
      const autoPts = this.simplifyOrthogonalPoints(rawPts);
      w.basePts = w.pins && w.pins.length > 0 ? this.buildPinnedWireWaypoints(w, autoPts) : autoPts;
    }
    this.updateWireBounds(w, w.basePts);
  },

  createWireRecord(spec) {
    const savedPins = CanvasModel.studioGraphState.wirePins.get(spec.wireKey);
    return {
      kind: spec.kind, wireKey: spec.wireKey,
      pins: savedPins ? savedPins.map((p) => ({ x: p.x, y: p.y, axis: p.axis, segIdx: p.segIdx })) : [],
      fromNode: spec.fromNode || null, toNode: spec.toNode || null,
      fromChapterObj: spec.fromChapterObj || null, toChapterObj: spec.toChapterObj || null,
      chapterScope: spec.chapterScope || null, fromChapterOutPort: spec.fromChapterOutPort || null,
      toChapterOutPort: null, portRef: spec.portRef || null, portIdx: spec.portIdx || 0,
      isBottomPort: Boolean(spec.isBottomPort), targetChapterLower: spec.targetChapterLower || "",
      targetGroupKey: spec.targetGroupKey || "", unreachable: Boolean(spec.unreachable),
      x1: 0, y1: 0, x2: 0, y2: 0, minX: 0, minY: 0, maxX: 0, maxY: 0,
      basePts: [], pts: [], bridges: null, bbox: null
    };
  },

  buildWireCatalogFromMemory() {
    const gState = CanvasModel.studioGraphState;
    const nodes = gState.nodes, chapters = gState.chapters;
    const wires = [];

    for (let i = 0; i < nodes.length; i++) {
      const fromNode = nodes[i];
      const portList = fromNode.hasChoices && fromNode.ports.length > 0 ? fromNode.ports : fromNode.implicitPort ? [fromNode.implicitPort] : [];
      for (let j = 0; j < portList.length; j++) {
        const pt = portList[j];
        const targetId = pt.targetId || pt.target || "";
        const targetChapter = (pt.targetChapter || "").trim();
        if ((!targetId || targetId === "END") && !targetChapter) continue;
        if (targetChapter) {
          const fromCh = fromNode.chapterObj || gState.chapterByKey.get(fromNode.chapterLower);
          if (!fromCh || fromCh.hidden) continue;
          const targetChLower = targetChapter.toLowerCase();
          pt.connected = true;
          wires.push(this.createWireRecord({
            kind: "scene-to-chapter-out", wireKey: (fromNode.id + ":" + (pt.idx || 0)).toLowerCase(),
            fromNode, fromChapterObj: fromCh, chapterScope: fromCh, portRef: pt, portIdx: pt.idx || 0,
            isBottomPort: pt.isBottom, targetChapterLower: targetChLower,
            targetGroupKey: "ch_out:" + fromNode.chapterLower + "->" + targetChLower, unreachable: fromNode.unreachable
          }));
          continue;
        }
        const toNode = gState.nodeById.get(targetId);
        if (!toNode || (fromNode.chapterLower && toNode.chapterLower && fromNode.chapterLower !== toNode.chapterLower)) continue;
        pt.connected = true;
        wires.push(this.createWireRecord({
          kind: "scene-to-scene", wireKey: (fromNode.id + ":" + (pt.idx || 0)).toLowerCase(),
          fromNode, toNode, chapterScope: fromNode.chapterObj || toNode.chapterObj || null,
          portRef: pt, portIdx: pt.idx || 0, isBottomPort: pt.isBottom,
          targetGroupKey: "scene:" + targetId, unreachable: fromNode.unreachable
        }));
      }
    }

    for (let i = 0; i < chapters.length; i++) {
      const fromCh = chapters[i];
      if (fromCh.hidden) continue;
      if (!fromCh.isRoot) {
        let entryNode = fromCh.startSceneId ? gState.nodeById.get(fromCh.startSceneId) : null;
        if (!entryNode || entryNode.chapterLower !== fromCh.nameLower) {
          entryNode = null;
          const chNodes = fromCh.nodes || nodes;
          for (let k = 0; k < chNodes.length; k++) {
            if (chNodes[k].chapterLower === fromCh.nameLower && (chNodes[k].isEntry || !entryNode)) {
              entryNode = chNodes[k];
              if (chNodes[k].isEntry) break;
            }
          }
        }
        if (entryNode) {
          wires.push(this.createWireRecord({
            kind: "chapter-in-to-entry", wireKey: ("ch_in:" + fromCh.nameLower).toLowerCase(),
            toNode: entryNode, fromChapterObj: fromCh, toChapterObj: fromCh, chapterScope: fromCh,
            isBottomPort: true, targetChapterLower: fromCh.nameLower, targetGroupKey: "scene:" + entryNode.id,
            unreachable: entryNode.unreachable
          }));
        }
      }
      for (let j = 0; j < fromCh.outPorts.length; j++) {
        const op = fromCh.outPorts[j];
        if (!op.targetLower) continue;
        const toCh = gState.chapterByKey.get(op.targetLower);
        if (!toCh || toCh.hidden || toCh === fromCh) continue;
        op.connected = true;
        wires.push(this.createWireRecord({
          kind: "chapter-to-chapter", wireKey: ("ch_out:" + fromCh.nameLower + "->" + op.targetLower).toLowerCase(),
          fromChapterObj: fromCh, toChapterObj: toCh, fromChapterOutPort: op, portRef: op, portIdx: j,
          isBottomPort: (op.side || "bottom") === "bottom", targetChapterLower: op.targetLower,
          targetGroupKey: "ch_in:" + op.targetLower, unreachable: false
        }));
      }
    }
    return wires;
  },

  applyParallelOffsetsAndBundles(wires) {
    for (let i = 0; i < wires.length; i++) {
      const w = wires[i];
      w.pts = w.basePts.map((p) => ({ x: p.x, y: p.y }));
      w.colorIdx = (w.portIdx || 0) % 8;
      if (w.portRef) w.portRef.colorIdx = w.colorIdx;
      this.updateWireBounds(w, w.pts);
    }
  },

  rebuildWiresFromPorts() {
    this.route();
  },

  route() {
    const gState = CanvasModel.studioGraphState;
    const wires = this.buildWireCatalogFromMemory();
    for (let i = 0; i < wires.length; i++) {
      this.routeSingleWireBasePts(wires[i], gState.nodes, gState.chapters);
    }
    gState.wires = wires;
    this.applyParallelOffsetsAndBundles(wires);
    gState.staticBatchDirty = true;
  },

  updateTempPreview() {
    CanvasModel.studioGraphState.overlayBatchDirty = true;
    EditorWebglScene.render();
  },

  draw(dirtyNodeIds, dirtyAabbs, changedChapterFramesSet) {
    this.route(dirtyNodeIds, dirtyAabbs, changedChapterFramesSet);
    EditorWebglScene.render();
  }
};
Dispatcher.register("EditorWires", EditorWires);
