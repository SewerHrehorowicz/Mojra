"use strict";

// NodeCanvasGeometry: Semantic 2D geometry primitives (Vec2, Rect, Segment2D),
// grid snapping, polyline projection, and spatial hash grid indexing.
const NodeCanvasGeometry = {
  _segProj: { t: 0, distSq: 0, lenSq: 0 },

  tokens: {
    numType: "number"
  },

  updateRect(rect) {
    rect.left = rect.x;
    rect.top = rect.y;
    rect.right = rect.x + rect.w;
    rect.bottom = rect.y + rect.h;
    return rect;
  },

  copyRect(targetRect, sourceRect) {
    targetRect.x = sourceRect.x;
    targetRect.y = sourceRect.y;
    targetRect.w = sourceRect.w;
    targetRect.h = sourceRect.h;
    return this.updateRect(targetRect);
  },

  inRect(point, rect) {
    const left = rect.left !== undefined ? rect.left : rect.x;
    const top = rect.top !== undefined ? rect.top : rect.y;
    const right = rect.right !== undefined ? rect.right : rect.x + rect.w;
    const bottom = rect.bottom !== undefined ? rect.bottom : rect.y + rect.h;
    return point.x >= left && point.x <= right && point.y >= top && point.y <= bottom;
  },

  rectsOverlap(rectA, rectB, margin) {
    const gap = margin || 0;
    return (
      rectA.left < rectB.right + gap &&
      rectA.right + gap > rectB.left &&
      rectA.top < rectB.bottom + gap &&
      rectA.bottom + gap > rectB.top
    );
  },

  distSq(pointA, pointB) {
    const deltaX = pointA.x - pointB.x;
    const deltaY = pointA.y - pointB.y;
    return deltaX * deltaX + deltaY * deltaY;
  },

  toSpatialCell(coord) {
    return coord >> NodeCanvasStyle.grid.spatialShift;
  },

  packGridKey(cellX, cellY) {
    return ((cellY & 0xffff) << 16) | (cellX & 0xffff);
  },

  snapToEditorGrid(value) {
    const gridSize = NodeCanvasStyle.grid.size;
    return Math.round(value / gridSize) * gridSize;
  },

  snapToWireGrid(value) {
    const wireGrid = NodeCanvasStyle.wire.grid;
    return Math.round(value / wireGrid) * wireGrid;
  },

  snapToWirePinGrid(value) {
    const laneStep = NodeCanvasStyle.wire.laneStep;
    return Math.round(value / laneStep) * laneStep;
  },

  projectPointToSegment(point, segStart, segEnd) {
    const deltaX = segEnd.x - segStart.x;
    const deltaY = segEnd.y - segStart.y;
    const lenSq = deltaX * deltaX + deltaY * deltaY;
    const ratio = lenSq > 0.001
      ? Math.max(0, Math.min(1, ((point.x - segStart.x) * deltaX + (point.y - segStart.y) * deltaY) / lenSq))
      : 0;
    const errX = point.x - (segStart.x + ratio * deltaX);
    const errY = point.y - (segStart.y + ratio * deltaY);
    const out = this._segProj;
    out.t = ratio;
    out.distSq = errX * errX + errY * errY;
    out.lenSq = lenSq;
    return out;
  },

  findClosestPolylineSegment(points, point) {
    let bestSeg = 0;
    let bestDistSq = Infinity;
    let bestLenSq = 0;
    for (let segIdx = 0; segIdx < points.length - 1; segIdx++) {
      const proj = this.projectPointToSegment(point, points[segIdx], points[segIdx + 1]);
      if (proj.lenSq < 1.0) continue;
      if (proj.distSq < bestDistSq - 0.5 || (proj.distSq <= bestDistSq + 0.5 && (proj.lenSq >= 64.0 || proj.lenSq > bestLenSq))) {
        bestDistSq = proj.distSq;
        bestSeg = segIdx;
        bestLenSq = proj.lenSq;
      }
    }
    return bestSeg;
  },

  computePointArcLengthAlongPolyline(points, point) {
    if (!points || points.length < 2) return 0;
    let bestDistSq = Infinity;
    let bestArc = 0;
    let accum = 0;
    for (let segIdx = 0; segIdx < points.length - 1; segIdx++) {
      const proj = this.projectPointToSegment(point, points[segIdx], points[segIdx + 1]);
      const segLen = Math.sqrt(proj.lenSq);
      if (proj.distSq < bestDistSq) {
        bestDistSq = proj.distSq;
        bestArc = accum + proj.t * segLen;
      }
      accum += segLen;
    }
    return bestArc;
  },

  computeWirePinSegmentBar(wire, pin) {
    const points = wire && (wire.pts && wire.pts.length >= 2 ? wire.pts : wire.basePts);
    if (!points || points.length < 2 || !pin) return null;
    const bestSeg =
      typeof pin.segIdx === this.tokens.numType && pin.segIdx >= 0 && pin.segIdx < points.length - 1
        ? pin.segIdx
        : this.findClosestPolylineSegment(points, pin);
    if (bestSeg < 0 || bestSeg >= points.length - 1) return null;
    const segStart = points[bestSeg];
    const segEnd = points[bestSeg + 1];
    const deltaX = segEnd.x - segStart.x;
    const deltaY = segEnd.y - segStart.y;
    const isHoriz = Math.abs(deltaX) >= Math.abs(deltaY);
    const segLen = isHoriz ? Math.abs(deltaX) : Math.abs(deltaY);
    if (segLen < 4) return null;
    const maxBarLen = NodeCanvasStyle.pad.w;
    const availLen = segLen > 20 ? segLen - 16 : Math.max(6, segLen - 4);
    const barLen = Math.min(maxBarLen, availLen);
    const centerX = (segStart.x + segEnd.x) * 0.5;
    const centerY = (segStart.y + segEnd.y) * 0.5;
    if (isHoriz) {
      const startX = centerX - barLen * 0.5;
      const endX = centerX + barLen * 0.5;
      return {
        isHoriz: true,
        segIdx: bestSeg,
        start: { x: startX, y: segStart.y },
        end: { x: endX, y: segStart.y },
        center: { x: centerX, y: segStart.y },
        x: startX,
        y: segStart.y,
        w: barLen,
        h: 0
      };
    }
    const startY = centerY - barLen * 0.5;
    const endY = centerY + barLen * 0.5;
    return {
      isHoriz: false,
      segIdx: bestSeg,
      start: { x: segStart.x, y: startY },
      end: { x: segStart.x, y: endY },
      center: { x: segStart.x, y: centerY },
      x: segStart.x,
      y: startY,
      w: 0,
      h: barLen
    };
  },

  buildSpatialGridForNodes(nodeList) {
    const grid = new Map();
    if (!nodeList) return grid;
    for (let idx = 0; idx < nodeList.length; idx++) {
      this.insertNodeIntoSpatialGrid(grid, nodeList[idx]);
    }
    nodeList._spatialGrid = grid;
    return grid;
  },

  insertNodeIntoSpatialGrid(grid, node) {
    if (!grid || !node || node.hidden) return;
    const nodeGap = NodeCanvasStyle.gap.node;
    const minCellX = this.toSpatialCell(node.left - nodeGap);
    const maxCellX = this.toSpatialCell(node.right + nodeGap);
    const minCellY = this.toSpatialCell(node.top - nodeGap);
    const maxCellY = this.toSpatialCell(node.bottom + nodeGap);
    node._gridMinX = minCellX;
    node._gridMaxX = maxCellX;
    node._gridMinY = minCellY;
    node._gridMaxY = maxCellY;
    for (let cellY = minCellY; cellY <= maxCellY; cellY++) {
      for (let cellX = minCellX; cellX <= maxCellX; cellX++) {
        const key = this.packGridKey(cellX, cellY);
        const cell = grid.get(key);
        if (!cell) grid.set(key, [node]);
        else cell.push(node);
      }
    }
  },

  removeNodeFromSpatialGrid(grid, node) {
    if (!grid || !node || node._gridMinX === undefined) return;
    for (let cellY = node._gridMinY; cellY <= node._gridMaxY; cellY++) {
      for (let cellX = node._gridMinX; cellX <= node._gridMaxX; cellX++) {
        const cell = grid.get(this.packGridKey(cellX, cellY));
        if (!cell) continue;
        const idx = cell.indexOf(node);
        if (idx >= 0) {
          cell[idx] = cell[cell.length - 1];
          cell.pop();
        }
      }
    }
  },

  updateNodeInSpatialGrids(node) {
    const graphState = NodeCanvasLayout.studioGraphState;
    if (graphState.nodeSpatialGrid) {
      this.removeNodeFromSpatialGrid(graphState.nodeSpatialGrid, node);
      this.insertNodeIntoSpatialGrid(graphState.nodeSpatialGrid, node);
    }
    const chapter = node.chapterObj;
    if (chapter && chapter.nodes && chapter.nodes._spatialGrid) {
      this.removeNodeFromSpatialGrid(chapter.nodes._spatialGrid, node);
      this.insertNodeIntoSpatialGrid(chapter.nodes._spatialGrid, node);
    }
  }
};
