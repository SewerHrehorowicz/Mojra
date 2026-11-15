"use strict";

// NodeCanvasRouter: Orthogonal wire pathfinder, obstacle linecasting via SpatialGrid,
// user pin constraints, and wire catalog generation.
const NodeCanvasRouter = {
  activeWire: null,
  _ray: { isVert: false, coord: 0, spanStart: 0, spanEnd: 0, dir: 0, pad: 0 },

  tokens: {
    axisH: "H",
    axisV: "V",
    sideBottom: "bottom",
    endTarget: "END",
    chInPrefix: "ch_in:",
    chOutPrefix: "ch_out:",
    arrowSep: "->",
    kindSceneToChOut: "scene-to-chapter-out",
    kindSceneToScene: "scene-to-scene",
    kindChToCh: "chapter-to-chapter",
    kindChInToEntry: "chapter-in-to-entry",
    numType: "number"
  },

  makeRay(isVert, coord, spanStart, spanEnd, dir, pad) {
    const ray = this._ray;
    ray.isVert = Boolean(isVert);
    ray.coord = coord;
    ray.spanStart = spanStart;
    ray.spanEnd = spanEnd;
    ray.dir = dir || 0;
    ray.pad = pad !== undefined ? pad : NodeCanvasStyle.wire.pad;
    return ray;
  },

  ptsList() {
    const out = [];
    for (let idx = 0; idx < arguments.length; idx += 2) {
      out.push({ x: arguments[idx], y: arguments[idx + 1] });
    }
    return out;
  },

  simplifyOrthogonalPoints(points) {
    if (!points || points.length === 0) return [];
    const simplified = [{ x: points[0].x, y: points[0].y }];
    for (let idx = 1; idx < points.length; idx++) {
      const prev = simplified[simplified.length - 1];
      const cur = { x: points[idx].x, y: points[idx].y };
      if (Math.abs(cur.x - prev.x) < 0.5 && Math.abs(cur.y - prev.y) < 0.5) continue;
      if (simplified.length >= 2) {
        const prev2 = simplified[simplified.length - 2];
        const collinearX = Math.abs(prev2.x - prev.x) < 0.5 && Math.abs(prev.x - cur.x) < 0.5;
        const collinearY = Math.abs(prev2.y - prev.y) < 0.5 && Math.abs(prev.y - cur.y) < 0.5;
        if (collinearX || collinearY) {
          simplified[simplified.length - 1] = cur;
          continue;
        }
      }
      simplified.push(cur);
    }
    return simplified;
  },

  testObstacleHit(obstacleRect, ray) {
    if (!obstacleRect || obstacleRect.hidden) return 0;
    const wire = this.activeWire;
    const ignore1 = wire ? wire.fromEntity : null;
    const ignore2 = wire ? wire.toEntity : null;
    const isEndpoint =
      obstacleRect === ignore1 ||
      obstacleRect === ignore2 ||
      Boolean(obstacleRect.id && (obstacleRect.id === ignore1 || obstacleRect.id === ignore2));
    const minSpan = ray.spanStart < ray.spanEnd ? ray.spanStart : ray.spanEnd;
    const maxSpan = ray.spanStart > ray.spanEnd ? ray.spanStart : ray.spanEnd;
    const fixedCoord = ray.coord;
    const pad = ray.pad;
    if (ray.isVert) {
      if (isEndpoint) {
        return fixedCoord > obstacleRect.left - 6 && fixedCoord < obstacleRect.right + 6 && maxSpan > obstacleRect.top + 3 && minSpan < obstacleRect.bottom - 3 ? 2 : 0;
      }
      return fixedCoord > obstacleRect.left - pad && fixedCoord < obstacleRect.right + pad && maxSpan > obstacleRect.top - pad && minSpan < obstacleRect.bottom + pad ? 1 : 0;
    }
    if (isEndpoint) {
      return fixedCoord > obstacleRect.top - 7 && fixedCoord < obstacleRect.bottom + 7 && maxSpan > obstacleRect.left + 3 && minSpan < obstacleRect.right - 3 ? 2 : 0;
    }
    return fixedCoord > obstacleRect.top - pad && fixedCoord < obstacleRect.bottom + pad && maxSpan > obstacleRect.left - pad && minSpan < obstacleRect.right + pad ? 1 : 0;
  },

  linecast(ray) {
    const wire = this.activeWire;
    const obstacles = wire && wire.obstacles ? wire.obstacles : null;
    if (!obstacles || obstacles.length === 0) return null;
    const minSpan = ray.spanStart < ray.spanEnd ? ray.spanStart : ray.spanEnd;
    const maxSpan = ray.spanStart > ray.spanEnd ? ray.spanStart : ray.spanEnd;
    if (maxSpan - minSpan < 2) return null;
    const pad = ray.pad;
    let bestHit = null;
    let bestDist = Infinity;

    const grid = obstacles._spatialGrid;
    if (grid && obstacles.length > NodeCanvasStyle.grid.minObstacles) {
      const minCellX = NodeCanvasGeometry.toSpatialCell(ray.isVert ? ray.coord - pad : minSpan - pad);
      const maxCellX = NodeCanvasGeometry.toSpatialCell(ray.isVert ? ray.coord + pad : maxSpan + pad);
      const minCellY = NodeCanvasGeometry.toSpatialCell(ray.isVert ? minSpan - pad : ray.coord - pad);
      const maxCellY = NodeCanvasGeometry.toSpatialCell(ray.isVert ? maxSpan + pad : ray.coord + pad);
      if ((maxCellX - minCellX + 1) * (maxCellY - minCellY + 1) < obstacles.length) {
        for (let cellY = minCellY; cellY <= maxCellY; cellY++) {
          for (let cellX = minCellX; cellX <= maxCellX; cellX++) {
            const cell = grid.get(NodeCanvasGeometry.packGridKey(cellX, cellY));
            if (!cell) continue;
            for (let idx = 0; idx < cell.length; idx++) {
              const cand = cell[idx];
              const hitKind = this.testObstacleHit(cand, ray);
              if (hitKind === 2) return cand;
              if (hitKind === 1) {
                const dist = Math.abs((ray.isVert ? cand.top : cand.left) - ray.spanStart);
                if (dist < bestDist) {
                  bestDist = dist;
                  bestHit = cand;
                }
              }
            }
          }
        }
        return bestHit;
      }
    }

    for (let idx = 0; idx < obstacles.length; idx++) {
      const cand = obstacles[idx];
      const hitKind = this.testObstacleHit(cand, ray);
      if (hitKind === 2) return cand;
      if (hitKind === 1) {
        const dist = Math.abs((ray.isVert ? cand.top : cand.left) - ray.spanStart);
        if (dist < bestDist) {
          bestDist = dist;
          bestHit = cand;
        }
      }
    }
    return bestHit;
  },

  resolveClearCoordInDir(ray, dir) {
    const basePad = ray.pad;
    let curCoord = ray.coord;
    for (let iter = 0; iter < 12; iter++) {
      ray.coord = curCoord;
      ray.pad = basePad - 2;
      const hit = this.linecast(ray);
      if (!hit) {
        ray.pad = basePad;
        return { coord: curCoord, clear: true };
      }
      curCoord = dir < 0 ? (ray.isVert ? hit.left : hit.top) - basePad : (ray.isVert ? hit.right : hit.bottom) + basePad;
    }
    ray.coord = curCoord;
    ray.pad = basePad - 2;
    const isClear = !this.linecast(ray);
    ray.pad = basePad;
    return { coord: curCoord, clear: isClear };
  },

  resolveClearCoord(ray) {
    const origCoord = ray.coord;
    const basePad = ray.pad;
    ray.pad = basePad - 2;
    if (!this.linecast(ray)) {
      ray.pad = basePad;
      return origCoord;
    }
    ray.pad = basePad;
    const dir = ray.dir || 0;
    if (dir !== 0) {
      const primary = this.resolveClearCoordInDir(ray, dir);
      if (primary.clear) return primary.coord;
      ray.coord = origCoord;
      const fallback = this.resolveClearCoordInDir(ray, -dir);
      return fallback.clear ? fallback.coord : primary.coord;
    }
    const negRes = this.resolveClearCoordInDir(ray, -1);
    ray.coord = origCoord;
    const posRes = this.resolveClearCoordInDir(ray, 1);
    if (negRes.clear && !posRes.clear) return negRes.coord;
    if (posRes.clear && !negRes.clear) return posRes.coord;
    return Math.abs(negRes.coord - origCoord) <= Math.abs(posRes.coord - origCoord) ? negRes.coord : posRes.coord;
  },

  routeBottomPort(wire) {
    const wireStyle = NodeCanvasStyle.wire;
    const wireGrid = wireStyle.grid;
    const card = NodeCanvasStyle.card;
    const startX = wire.x1;
    const startY = wire.y1;
    const endX = wire.x2;
    const endY = wire.y2;
    const fromBottom = wire.fromBottom;
    const toLeft = wire.targetLeft;
    const toRight = wire.targetRight;

    if (endY >= startY + 12) {
      if (Math.abs(endX - startX) < 2) {
        const hit = this.linecast(this.makeRay(true, startX, startY + 4, endY - 4, 0, wireStyle.tightPad));
        if (!hit) return this.ptsList(startX, startY, endX, endY);
        const sideDir = startX <= (hit.left + hit.right) * 0.5 ? -1 : 1;
        const detourX = this.resolveClearCoord(this.makeRay(true, sideDir < 0 ? hit.left - 18 : hit.right + 18, hit.top - 16, hit.bottom + 16, sideDir, 16));
        const topY = Math.max(startY + 12, this.resolveClearCoord(this.makeRay(false, hit.top - 16, startX, detourX, -1, wireStyle.pad)));
        const botY = Math.min(endY - 12, this.resolveClearCoord(this.makeRay(false, hit.bottom + 16, detourX, endX, 1, wireStyle.pad)));
        return this.ptsList(startX, startY, startX, topY, detourX, topY, detourX, botY, endX, botY, endX, endY);
      }
      const rawMidY = fromBottom !== null && fromBottom !== undefined && fromBottom > startY && fromBottom < endY - wireGrid
        ? (fromBottom + endY) * 0.5
        : (startY + endY) * 0.5;
      let midY = this.resolveClearCoord(this.makeRay(false, Math.max(startY + 16, Math.min(endY - 16, NodeCanvasGeometry.snapToWireGrid(rawMidY))), startX, endX, 1, wireStyle.pad));
      midY = Math.max(startY + 12, Math.min(endY - 12, midY));
      const clearStartLeg = !this.linecast(this.makeRay(true, startX, startY + 4, midY, 0, wireStyle.tightPad));
      const clearEndLeg = !this.linecast(this.makeRay(true, endX, midY, endY - 4, 0, wireStyle.tightPad));
      if (clearStartLeg && clearEndLeg) {
        return this.ptsList(startX, startY, startX, midY, endX, midY, endX, endY);
      }
      const stepOutY = this.resolveClearCoord(this.makeRay(false, Math.max(startY + 18, NodeCanvasGeometry.snapToWireGrid(startY + wireGrid)), startX, endX, 1, wireStyle.pad));
      const stepInY = this.resolveClearCoord(this.makeRay(false, Math.min(endY - 18, NodeCanvasGeometry.snapToWireGrid(endY - wireGrid)), startX, endX, -1, wireStyle.pad));
      const midChannelX = this.resolveClearCoord(this.makeRay(true, NodeCanvasGeometry.snapToWireGrid((startX + endX) * 0.5), stepOutY, stepInY, 0, 16));
      return this.ptsList(startX, startY, startX, stepOutY, midChannelX, stepOutY, midChannelX, stepInY, endX, stepInY, endX, endY);
    }

    let channelX = NodeCanvasGeometry.snapToWireGrid((startX + endX) * 0.5);
    if (toLeft !== undefined && toLeft !== null && toRight !== undefined && toRight !== null) {
      const fromRight = startX + card.halfW;
      const fromLeft = startX - card.halfW;
      if (toLeft > fromRight + 20) channelX = NodeCanvasGeometry.snapToWireGrid((fromRight + toLeft) * 0.5);
      else if (toRight < fromLeft - 20) channelX = NodeCanvasGeometry.snapToWireGrid((fromLeft + toRight) * 0.5);
      else channelX = NodeCanvasGeometry.snapToWireGrid(Math.max(fromRight, toRight) + wireGrid);
    }
    const outY = this.resolveClearCoord(this.makeRay(false, Math.max(startY + 18, NodeCanvasGeometry.snapToWireGrid(startY + wireGrid)), startX, channelX, 1, wireStyle.pad));
    const baseInY = Math.min(endY - 18, NodeCanvasGeometry.snapToWireGrid(endY - wireGrid));
    const hitInH = this.linecast(this.makeRay(false, baseInY, channelX, endX, 0, 12));
    const inDir = hitInH && hitInH.bottom + 16 <= endY - 12 ? 1 : -1;
    const inYBottom = this.resolveClearCoord(this.makeRay(false, baseInY, channelX, endX, inDir, wireStyle.pad));
    if (this.linecast(this.makeRay(true, channelX, outY, inYBottom, 0, wireStyle.pad))) {
      channelX = this.resolveClearCoord(this.makeRay(true, startX, outY, inYBottom, 0, 18));
    }
    return this.ptsList(startX, startY, startX, outY, channelX, outY, channelX, inYBottom, endX, inYBottom, endX, endY);
  },

  routeSidePort(wire) {
    const wireStyle = NodeCanvasStyle.wire;
    const wireGrid = wireStyle.grid;
    const wireLaneStep = wireStyle.laneStep;
    const startX = wire.x1;
    const startY = wire.y1;
    const endX = wire.x2;
    const endY = wire.y2;
    const fromBottom = wire.fromBottom;
    const toRight = wire.targetRight;
    const portIdx = wire.portIdx || 0;
    const portCount = Math.max(1, wire.totalPorts || portIdx + 1);

    const goesDown = fromBottom !== null && fromBottom !== undefined ? endY > fromBottom + 4 : endY > startY + 16;
    const stubRank = goesDown ? Math.max(0, portCount - 1 - portIdx) : portIdx;
    const exitStub = wireGrid + stubRank * wireLaneStep;
    let outX = startX + exitStub;
    let inY = endY - wireGrid;

    if (fromBottom === null || fromBottom === undefined) {
      if (endX >= outX && endY >= startY) return this.ptsList(startX, startY, endX, startY, endX, endY);
      const midY = endY >= startY + 32 ? NodeCanvasGeometry.snapToWireGrid((startY + endY) * 0.5) : inY;
      return this.ptsList(startX, startY, outX, startY, outX, midY, endX, midY, endX, endY);
    }

    if (endY >= fromBottom + 6) {
      const clearHoriz = !this.linecast(this.makeRay(false, startY, startX + 4, endX, 0, wireStyle.tightPad));
      const clearVert = !this.linecast(this.makeRay(true, endX, startY, endY - 4, 0, wireStyle.tightPad));
      if (endX >= outX - 1.5 && clearHoriz && clearVert) {
        return this.ptsList(startX, startY, endX, startY, endX, endY);
      }
      const gap = endY - fromBottom;
      const minClear = Math.min(16, Math.max(8, Math.floor(gap * 0.35)));
      const minMidY = fromBottom + minClear;
      const maxMidY = Math.max(minMidY, endY - minClear);
      const turnRank = endX > outX ? portCount - 1 - stubRank : stubRank;
      const baseCorridorY = Math.max(minMidY, Math.min(maxMidY, NodeCanvasGeometry.snapToWireGrid((fromBottom + endY) * 0.5)));
      const laneShift = portCount > 1 && maxMidY - minMidY >= 6 ? Math.round((turnRank - (portCount - 1) * 0.5) * wireLaneStep) : 0;
      let midY = Math.max(minMidY, Math.min(maxMidY, baseCorridorY + laneShift));
      outX = this.resolveClearCoord(this.makeRay(true, outX, startY, midY, 1, wireStyle.pad));
      midY = this.resolveClearCoord(this.makeRay(false, midY, outX, endX, 1, 12));
      const hitDrop = this.linecast(this.makeRay(true, endX, midY, endY - 4, 0, wireStyle.tightPad));
      if (!hitDrop) return this.ptsList(startX, startY, outX, startY, outX, midY, endX, midY, endX, endY);
      const detourX = this.resolveClearCoord(this.makeRay(true, endX, midY, endY - 14, endX >= outX ? -1 : 1, 16));
      const entryY = this.resolveClearCoord(this.makeRay(false, Math.min(endY - 12, hitDrop.bottom + 14), detourX, endX, -1, 12));
      return this.ptsList(startX, startY, outX, startY, outX, midY, detourX, midY, detourX, entryY, endX, entryY, endX, endY);
    }

    inY = this.resolveClearCoord(this.makeRay(false, inY, outX, endX, -1, wireStyle.pad));
    let bypassX = Math.max(outX, (toRight !== undefined && toRight !== null ? toRight : endX) + wireGrid + stubRank * wireLaneStep);
    bypassX = this.resolveClearCoord(this.makeRay(true, bypassX, startY, inY, 1, 16));
    if (!this.linecast(this.makeRay(false, startY, startX + 4, bypassX, 0, wireStyle.tightPad))) {
      return this.ptsList(startX, startY, bypassX, startY, bypassX, inY, endX, inY, endX, endY);
    }
    const underY = this.resolveClearCoord(this.makeRay(false, fromBottom + 16 + stubRank * wireLaneStep, outX, bypassX, 1, wireStyle.pad));
    outX = this.resolveClearCoord(this.makeRay(true, outX, startY, underY, 1, wireStyle.pad));
    bypassX = this.resolveClearCoord(this.makeRay(true, bypassX, underY, inY, 1, 16));
    return this.ptsList(startX, startY, outX, startY, outX, underY, bypassX, underY, bypassX, inY, endX, inY, endX, endY);
  },

  buildWireWaypoints(wire) {
    this.activeWire = wire;
    const rawPts = wire.isBottomPort ? this.routeBottomPort(wire) : this.routeSidePort(wire);
    this.activeWire = null;
    return rawPts;
  },

  buildPinnedWireWaypoints(wire, autoPtsInput) {
    const tok = this.tokens;
    const pins = wire.pins;
    const points = autoPtsInput && autoPtsInput.length >= 2
      ? autoPtsInput.map((pt) => ({ x: pt.x, y: pt.y }))
      : this.ptsList(wire.x1, wire.y1, wire.x2, wire.y2);
    if (!pins || pins.length === 0) return this.simplifyOrthogonalPoints(points);
    for (let idx = 0; idx < pins.length; idx++) {
      const pin = pins[idx];
      if (!pin || points.length < 2) continue;
      const segIdx = typeof pin.segIdx === tok.numType && pin.segIdx >= 0 && pin.segIdx < points.length - 1 ? pin.segIdx : 0;
      const segStart = points[segIdx];
      const segEnd = points[segIdx + 1];
      if (Math.abs(segStart.y - segEnd.y) <= Math.abs(segStart.x - segEnd.x)) {
        pin.axis = tok.axisH;
        segStart.y = pin.y;
        segEnd.y = pin.y;
        pin.x = Math.round((segStart.x + segEnd.x) * 0.5);
      } else {
        pin.axis = tok.axisV;
        segStart.x = pin.x;
        segEnd.x = pin.x;
        pin.y = Math.round((segStart.y + segEnd.y) * 0.5);
      }
    }
    return points;
  },

  getChapterOutputPortSlot(chapter, targetChapterLower) {
    const outPorts = chapter.outPorts || [];
    let matchIdx = 0;
    if (targetChapterLower) {
      for (let idx = 0; idx < outPorts.length; idx++) {
        if (outPorts[idx].targetLower === targetChapterLower) {
          matchIdx = idx;
          break;
        }
      }
    }
    return outPorts[matchIdx] || {
      x: chapter.x + chapter.w * 0.5,
      y: chapter.y + chapter.h,
      side: this.tokens.sideBottom,
      idx: 0
    };
  },

  updateWireBounds(wire, points) {
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (let idx = 0; idx < points.length; idx++) {
      const pt = points[idx];
      minX = Math.min(minX, pt.x);
      minY = Math.min(minY, pt.y);
      maxX = Math.max(maxX, pt.x);
      maxY = Math.max(maxY, pt.y);
    }
    const pad = NodeCanvasStyle.wire.bboxPad;
    wire.minX = minX - pad;
    wire.minY = minY - pad;
    wire.maxX = maxX + pad;
    wire.maxY = maxY + pad;
  },

  resolveWireEndpoints(wire, nodeObstacles, chapterObstacles) {
    const tok = this.tokens;
    if (wire.kind === tok.kindSceneToChOut) {
      const fromNode = wire.fromNode;
      const fromCh = wire.fromChapterObj;
      const outPort = this.getChapterOutputPortSlot(fromCh, wire.targetChapterLower);
      wire.x1 = wire.portRef.x;
      wire.y1 = wire.portRef.y;
      wire.x2 = outPort.x;
      wire.y2 = outPort.y;
      wire.fromBottom = fromNode.bottom;
      wire.targetLeft = outPort.x - 8;
      wire.targetRight = outPort.x + 8;
      wire.totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
      wire.fromEntity = fromNode;
      wire.toEntity = null;
      wire.obstacles = fromCh && fromCh.nodes ? fromCh.nodes : nodeObstacles;
      wire.unreachable = fromNode.unreachable;
      return;
    }
    if (wire.kind === tok.kindSceneToScene) {
      const fromNode = wire.fromNode;
      const toNode = wire.toNode;
      const chObj = fromNode.chapterObj || toNode.chapterObj;
      wire.x1 = wire.portRef.x;
      wire.y1 = wire.portRef.y;
      wire.x2 = toNode.inX;
      wire.y2 = toNode.inY;
      wire.fromBottom = fromNode.bottom;
      wire.targetLeft = toNode.left;
      wire.targetRight = toNode.right;
      wire.totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
      wire.fromEntity = fromNode;
      wire.toEntity = toNode;
      wire.obstacles = chObj && chObj.nodes ? chObj.nodes : nodeObstacles;
      wire.unreachable = fromNode.unreachable;
      return;
    }
    if (wire.kind === tok.kindChToCh) {
      const fromCh = wire.fromChapterObj;
      const toCh = wire.toChapterObj;
      const outPort = wire.fromChapterOutPort;
      wire.isBottomPort = true;
      wire.x1 = outPort.x;
      wire.y1 = outPort.y;
      wire.x2 = toCh.inX;
      wire.y2 = toCh.inY;
      wire.fromBottom = Math.max(wire.y1, wire.y2 - NodeCanvasStyle.card.minChapterH);
      wire.targetLeft = toCh.x;
      wire.targetRight = toCh.x + toCh.w;
      wire.totalPorts = fromCh.outPorts ? fromCh.outPorts.length : 1;
      wire.fromEntity = fromCh;
      wire.toEntity = toCh;
      wire.obstacles = chapterObstacles;
      wire.unreachable = false;
      return;
    }
    if (wire.kind === tok.kindChInToEntry) {
      const chapter = wire.toChapterObj;
      const toNode = wire.toNode;
      wire.isBottomPort = true;
      wire.x1 = chapter.inX;
      wire.y1 = chapter.inY;
      wire.x2 = toNode.inX;
      wire.y2 = toNode.inY;
      wire.fromBottom = chapter.y + NodeCanvasStyle.hit.chapterHeaderH;
      wire.targetLeft = toNode.left;
      wire.targetRight = toNode.right;
      wire.portIdx = 0;
      wire.totalPorts = 1;
      wire.fromEntity = null;
      wire.toEntity = toNode;
      wire.obstacles = chapter && chapter.nodes ? chapter.nodes : nodeObstacles;
      wire.unreachable = toNode.unreachable;
    }
  },

  routeSingleWire(wire, nodeObstacles, chapterObstacles) {
    this.resolveWireEndpoints(wire, nodeObstacles, chapterObstacles);
    const autoPts = this.simplifyOrthogonalPoints(this.buildWireWaypoints(wire));
    wire.basePts = wire.pins && wire.pins.length > 0 ? this.buildPinnedWireWaypoints(wire, autoPts) : autoPts;
    wire.pts = wire.basePts;
    wire.colorIdx = (wire.portIdx || 0) % 8;
    if (wire.portRef) wire.portRef.colorIdx = wire.colorIdx;
    this.updateWireBounds(wire, wire.pts);
  },

  createWireRecord(spec) {
    const savedPins = NodeCanvasLayout.studioGraphState.wirePins.get(spec.wireKey);
    return {
      kind: spec.kind,
      wireKey: spec.wireKey,
      pins: savedPins ? savedPins.map((pin) => ({ x: pin.x, y: pin.y, axis: pin.axis, segIdx: pin.segIdx })) : [],
      fromNode: spec.fromNode || null,
      toNode: spec.toNode || null,
      fromChapterObj: spec.fromChapterObj || null,
      toChapterObj: spec.toChapterObj || null,
      fromChapterOutPort: spec.fromChapterOutPort || null,
      portRef: spec.portRef || null,
      portIdx: spec.portIdx || 0,
      totalPorts: 1,
      isBottomPort: Boolean(spec.isBottomPort),
      targetChapterLower: spec.targetChapterLower || "",
      unreachable: Boolean(spec.unreachable),
      fromEntity: null,
      toEntity: null,
      obstacles: null,
      fromBottom: null,
      targetLeft: null,
      targetRight: null,
      x1: 0,
      y1: 0,
      x2: 0,
      y2: 0,
      minX: 0,
      minY: 0,
      maxX: 0,
      maxY: 0,
      colorIdx: 0,
      basePts: [],
      pts: []
    };
  },

  buildWireCatalogFromMemory() {
    const tok = this.tokens;
    const graphState = NodeCanvasLayout.studioGraphState;
    const nodes = graphState.nodes;
    const chapters = graphState.chapters;
    const wires = [];

    for (let nodeIdx = 0; nodeIdx < nodes.length; nodeIdx++) {
      const fromNode = nodes[nodeIdx];
      const portList = NodeCanvasModel.getActiveNodePorts(fromNode);
      for (let portIdx = 0; portIdx < portList.length; portIdx++) {
        const port = portList[portIdx];
        const targetId = port.targetId || "";
        const targetChapter = (port.targetChapter || "").trim();
        if ((!targetId || targetId === tok.endTarget) && !targetChapter) continue;
        if (targetChapter) {
          const fromCh = fromNode.chapterObj || graphState.chapterByKey.get(fromNode.chapterLower);
          if (!fromCh || fromCh.hidden) continue;
          const targetChLower = targetChapter.toLowerCase();
          port.connected = true;
          wires.push(this.createWireRecord({
            kind: tok.kindSceneToChOut,
            wireKey: (fromNode.id + ":" + (port.idx || 0)).toLowerCase(),
            fromNode: fromNode,
            fromChapterObj: fromCh,
            portRef: port,
            portIdx: port.idx || 0,
            isBottomPort: port.isBottom,
            targetChapterLower: targetChLower,
            unreachable: fromNode.unreachable
          }));
          continue;
        }
        const toNode = graphState.nodeById.get(targetId);
        if (!toNode || (fromNode.chapterLower && toNode.chapterLower && fromNode.chapterLower !== toNode.chapterLower)) continue;
        port.connected = true;
        wires.push(this.createWireRecord({
          kind: tok.kindSceneToScene,
          wireKey: (fromNode.id + ":" + (port.idx || 0)).toLowerCase(),
          fromNode: fromNode,
          toNode: toNode,
          portRef: port,
          portIdx: port.idx || 0,
          isBottomPort: port.isBottom,
          unreachable: fromNode.unreachable
        }));
      }
    }

    for (let chIdx = 0; chIdx < chapters.length; chIdx++) {
      const fromCh = chapters[chIdx];
      if (fromCh.hidden) continue;
      if (!fromCh.isRoot) {
        const entryNode = NodeCanvasModel.resolveChapterEntryNode(fromCh);
        if (entryNode) {
          wires.push(this.createWireRecord({
            kind: tok.kindChInToEntry,
            wireKey: (tok.chInPrefix + fromCh.nameLower).toLowerCase(),
            toNode: entryNode,
            fromChapterObj: fromCh,
            toChapterObj: fromCh,
            isBottomPort: true,
            targetChapterLower: fromCh.nameLower,
            unreachable: entryNode.unreachable
          }));
        }
      }
      for (let outIdx = 0; outIdx < fromCh.outPorts.length; outIdx++) {
        const outPort = fromCh.outPorts[outIdx];
        if (!outPort.targetLower) continue;
        const toCh = graphState.chapterByKey.get(outPort.targetLower);
        if (!toCh || toCh.hidden || toCh === fromCh) continue;
        outPort.connected = true;
        wires.push(this.createWireRecord({
          kind: tok.kindChToCh,
          wireKey: (tok.chOutPrefix + fromCh.nameLower + tok.arrowSep + outPort.targetLower).toLowerCase(),
          fromChapterObj: fromCh,
          toChapterObj: toCh,
          fromChapterOutPort: outPort,
          portRef: outPort,
          portIdx: outIdx,
          isBottomPort: (outPort.side || tok.sideBottom) === tok.sideBottom,
          targetChapterLower: outPort.targetLower,
          unreachable: false
        }));
      }
    }
    return wires;
  },

  route() {
    const graphState = NodeCanvasLayout.studioGraphState;
    const wires = this.buildWireCatalogFromMemory();
    for (let idx = 0; idx < wires.length; idx++) {
      this.routeSingleWire(wires[idx], graphState.nodes, graphState.chapters);
    }
    graphState.wires = wires;
    graphState.staticBatchDirty = true;
  },

  rerouteAfterWirePinMutation(wire) {
    if (!wire) return;
    const graphState = NodeCanvasLayout.studioGraphState;
    if (wire.wireKey) {
      if (wire.pins && wire.pins.length > 0) {
        graphState.wirePins.set(wire.wireKey, wire.pins.map((pin) => ({ x: pin.x, y: pin.y, axis: pin.axis, segIdx: pin.segIdx })));
      } else {
        graphState.wirePins.delete(wire.wireKey);
      }
    }
    this.routeSingleWire(wire, graphState.nodes, graphState.chapters);
    graphState.staticBatchDirty = true;
    graphState.overlayBatchDirty = true;
    NodeCanvasRenderer.render();
  }
};
