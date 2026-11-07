"use strict";

const WIRE_GRID = 24;
const WIRE_LANE_STEP = 6;

const WIRE = {
  grid: 24,
  laneStep: 6,
  pad: 14,
  tightPad: 10,
  collectorOffset: 12,
  bboxPad: 18
};

function snapToWireGrid(v) {
  return Math.round(v / WIRE_GRID) * WIRE_GRID;
}

function ptsList() {
  const out = [];
  for (let i = 0; i < arguments.length; i += 2) {
    out.push({ x: arguments[i], y: arguments[i + 1] });
  }
  return out;
}

function simplifyOrthogonalPoints(points) {
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
}

function testObstacleHit(b, isVert, fixedCoord, minSpan, maxSpan, p, ignore1, ignore2) {
  if (b.hidden) return 0;
  const isEndpoint =
    b === ignore1 ||
    b === ignore2 ||
    Boolean(b.id && (b.id === ignore1 || b.id === ignore2));
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
}

function linecastOrthogonal(isVert, fixedCoord, spanA, spanB, obstacles, ignore1, ignore2, pad) {
  if (!obstacles || obstacles.length === 0) return null;
  const minSpan = spanA < spanB ? spanA : spanB;
  const maxSpan = spanA > spanB ? spanA : spanB;
  if (maxSpan - minSpan < 2) return null;
  const p = pad !== undefined ? pad : WIRE.tightPad;

  const grid = obstacles._spatialGrid;
  if (grid && obstacles.length > SPATIAL.minObstacles) {
    const gx0 = toSpatialCell(isVert ? fixedCoord - p : minSpan - p);
    const gx1 = toSpatialCell(isVert ? fixedCoord + p : maxSpan + p);
    const gy0 = toSpatialCell(isVert ? minSpan - p : fixedCoord - p);
    const gy1 = toSpatialCell(isVert ? maxSpan + p : fixedCoord + p);
    if ((gx1 - gx0 + 1) * (gy1 - gy0 + 1) < obstacles.length) {
      let bestHit = null;
      let bestDist = Infinity;
      for (let gy = gy0; gy <= gy1; gy++) {
        const rowMask = (gy & 0xffff) << 16;
        for (let gx = gx0; gx <= gx1; gx++) {
          const cell = grid.get(rowMask | (gx & 0xffff));
          if (!cell) continue;
          for (let i = 0; i < cell.length; i++) {
            const b = cell[i];
            const hitKind = testObstacleHit(b, isVert, fixedCoord, minSpan, maxSpan, p, ignore1, ignore2);
            if (hitKind === 2) return b;
            if (hitKind === 1) {
              const dist = Math.abs((isVert ? b.top : b.left) - spanA);
              if (dist < bestDist) {
                bestDist = dist;
                bestHit = b;
              }
            }
          }
        }
      }
      return bestHit;
    }
  }

  for (let i = 0; i < obstacles.length; i++) {
    const b = obstacles[i];
    if (testObstacleHit(b, isVert, fixedCoord, minSpan, maxSpan, p, ignore1, ignore2) > 0) return b;
  }
  return null;
}

function linecastVertical(x, yA, yB, obstacles, ignore1, ignore2, pad) {
  return linecastOrthogonal(true, x, yA, yB, obstacles, ignore1, ignore2, pad);
}

function linecastHorizontal(y, xA, xB, obstacles, ignore1, ignore2, pad) {
  return linecastOrthogonal(false, y, xA, xB, obstacles, ignore1, ignore2, pad);
}

function resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, dir, p) {
  let cur = coord;
  for (let iter = 0; iter < 12; iter++) {
    const hit = linecastOrthogonal(isVert, cur, spanA, spanB, obstacles, ignoreId1, ignoreId2, p - 2);
    if (!hit) return { coord: cur, clear: true };
    cur = dir < 0 ? (isVert ? hit.left : hit.top) - p : (isVert ? hit.right : hit.bottom) + p;
  }
  return {
    coord: cur,
    clear: !linecastOrthogonal(isVert, cur, spanA, spanB, obstacles, ignoreId1, ignoreId2, p - 2)
  };
}

function resolveClearCoord(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
  const p = pad !== undefined ? pad : WIRE.pad;
  if (!linecastOrthogonal(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, p - 2)) {
    return coord;
  }
  const dir = preferDir || 0;
  if (dir !== 0) {
    const primary = resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, dir, p);
    if (primary.clear) return primary.coord;
    const fallback = resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, -dir, p);
    return fallback.clear ? fallback.coord : primary.coord;
  }
  const negRes = resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, -1, p);
  const posRes = resolveClearCoordInDir(isVert, coord, spanA, spanB, obstacles, ignoreId1, ignoreId2, 1, p);
  if (negRes.clear && !posRes.clear) return negRes.coord;
  if (posRes.clear && !negRes.clear) return posRes.coord;
  return Math.abs(negRes.coord - coord) <= Math.abs(posRes.coord - coord) ? negRes.coord : posRes.coord;
}

function resolveClearX(x, yA, yB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
  return resolveClearCoord(true, x, yA, yB, obstacles, ignoreId1, ignoreId2, preferDir, pad);
}

function resolveClearY(y, xA, xB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
  return resolveClearCoord(false, y, xA, xB, obstacles, ignoreId1, ignoreId2, preferDir, pad);
}

function buildWireWaypoints(
  x1,
  y1,
  x2,
  y2,
  fromBottom,
  portIdx,
  toLeft,
  toRight,
  isBottomPort,
  obstacles,
  fromId,
  toId,
  totalPorts,
  preferredOutX
) {
  const idx = portIdx || 0;
  const pCount = Math.max(1, totalPorts || idx + 1);
  const obs = obstacles || [];

  if (isBottomPort) {
    if (y2 >= y1 + 12) {
      if (Math.abs(x2 - x1) < 2) {
        const hit = linecastVertical(x1, y1 + 4, y2 - 4, obs, fromId, toId, WIRE.tightPad);
        if (!hit) return ptsList(x1, y1, x2, y2);
        const sideDir = x1 <= (hit.left + hit.right) * 0.5 ? -1 : 1;
        const detourX = resolveClearX(
          sideDir < 0 ? hit.left - 18 : hit.right + 18,
          hit.top - 16,
          hit.bottom + 16,
          obs,
          fromId,
          toId,
          sideDir,
          16
        );
        const topY = Math.max(y1 + 12, resolveClearY(hit.top - 16, x1, detourX, obs, fromId, toId, -1, WIRE.pad));
        const botY = Math.min(y2 - 12, resolveClearY(hit.bottom + 16, detourX, x2, obs, fromId, toId, 1, WIRE.pad));
        return ptsList(x1, y1, x1, topY, detourX, topY, detourX, botY, x2, botY, x2, y2);
      }

      const rawMidY =
        fromBottom !== null && fromBottom !== undefined && fromBottom > y1 && fromBottom < y2 - WIRE_GRID
          ? (fromBottom + y2) * 0.5
          : (y1 + y2) * 0.5;
      const snappedMidY = Math.max(y1 + 16, Math.min(y2 - 16, snapToWireGrid(rawMidY)));
      let midY = resolveClearY(snappedMidY, x1, x2, obs, fromId, toId, 1, WIRE.pad);
      midY = Math.max(y1 + 12, Math.min(y2 - 12, midY));
      if (
        !linecastVertical(x1, y1 + 4, midY, obs, fromId, toId, WIRE.tightPad) &&
        !linecastVertical(x2, midY, y2 - 4, obs, fromId, toId, WIRE.tightPad)
      ) {
        return ptsList(x1, y1, x1, midY, x2, midY, x2, y2);
      }
      const stepOutY = resolveClearY(Math.max(y1 + 18, snapToWireGrid(y1 + WIRE_GRID)), x1, x2, obs, fromId, toId, 1, WIRE.pad);
      const stepInY = resolveClearY(Math.min(y2 - 18, snapToWireGrid(y2 - WIRE_GRID)), x1, x2, obs, fromId, toId, -1, WIRE.pad);
      const midChannelX = resolveClearX(snapToWireGrid((x1 + x2) * 0.5), stepOutY, stepInY, obs, fromId, toId, 0, 16);
      return ptsList(x1, y1, x1, stepOutY, midChannelX, stepOutY, midChannelX, stepInY, x2, stepInY, x2, y2);
    }

    let channelX = snapToWireGrid((x1 + x2) * 0.5);
    if (toLeft !== undefined && toLeft !== null && toRight !== undefined && toRight !== null) {
      const fromRight = x1 + CARD.halfW;
      const fromLeft = x1 - CARD.halfW;
      if (toLeft > fromRight + 20) channelX = snapToWireGrid((fromRight + toLeft) * 0.5);
      else if (toRight < fromLeft - 20) channelX = snapToWireGrid((fromLeft + toRight) * 0.5);
      else channelX = snapToWireGrid(Math.max(fromRight, toRight) + WIRE_GRID);
    }
    const outY = resolveClearY(Math.max(y1 + 18, snapToWireGrid(y1 + WIRE_GRID)), x1, channelX, obs, fromId, toId, 1, WIRE.pad);
    const baseInY = Math.min(y2 - 18, snapToWireGrid(y2 - WIRE_GRID));
    const hitInH = linecastHorizontal(baseInY, channelX, x2, obs, fromId, toId, 12);
    const inYBottom = resolveClearY(baseInY, channelX, x2, obs, fromId, toId, hitInH && hitInH.bottom + 16 <= y2 - 12 ? 1 : -1, WIRE.pad);
    if (linecastVertical(channelX, outY, inYBottom, obs, fromId, toId, WIRE.pad)) {
      channelX = resolveClearX(x1, outY, inYBottom, obs, fromId, toId, 0, 18);
      const snappedChX = snapToWireGrid(channelX);
      if (!linecastVertical(snappedChX, outY, inYBottom, obs, fromId, toId, WIRE.pad)) channelX = snappedChX;
    }
    return ptsList(x1, y1, x1, outY, channelX, outY, channelX, inYBottom, x2, inYBottom, x2, y2);
  }

  const goesDown = fromBottom !== null && fromBottom !== undefined ? y2 > fromBottom + 4 : y2 > y1 + 16;
  const stubRank = goesDown ? Math.max(0, pCount - 1 - idx) : idx;
  const exitStub = WIRE_GRID + stubRank * WIRE_LANE_STEP;
  let outX = typeof preferredOutX === "number" && !isNaN(preferredOutX) ? preferredOutX : x1 + exitStub;
  let inY = y2 - WIRE_GRID;

  if (fromBottom === null || fromBottom === undefined) {
    if (x2 >= outX && y2 >= y1) return ptsList(x1, y1, x2, y1, x2, y2);
    const midY = y2 >= y1 + 32 ? snapToWireGrid((y1 + y2) * 0.5) : inY;
    return ptsList(x1, y1, outX, y1, outX, midY, x2, midY, x2, y2);
  }

  if (y2 >= fromBottom + 6) {
    if (
      Math.abs(outX - x2) <= 1.5 &&
      !linecastHorizontal(y1, x1 + 4, x2, obs, fromId, toId, WIRE.tightPad) &&
      !linecastVertical(x2, y1, y2 - 4, obs, fromId, toId, WIRE.tightPad)
    ) {
      return ptsList(x1, y1, x2, y1, x2, y2);
    }
    const gap = y2 - fromBottom;
    const minClear = Math.min(16, Math.max(8, Math.floor(gap * 0.35)));
    const minMidY = fromBottom + minClear;
    const maxMidY = Math.max(minMidY, y2 - minClear);
    const turnRank = x2 > outX ? pCount - 1 - stubRank : stubRank;
    const baseCorridorY = Math.max(minMidY, Math.min(maxMidY, snapToWireGrid((fromBottom + y2) * 0.5)));
    const laneShift = pCount > 1 && maxMidY - minMidY >= 6 ? Math.round((turnRank - (pCount - 1) * 0.5) * WIRE_LANE_STEP) : 0;
    let midY = Math.max(minMidY, Math.min(maxMidY, baseCorridorY + laneShift));
    outX = resolveClearX(outX, y1, midY, obs, fromId, toId, 1, WIRE.pad);
    if (
      Math.abs(outX - x2) <= 1.5 &&
      !linecastHorizontal(y1, x1 + 4, x2, obs, fromId, toId, WIRE.tightPad) &&
      !linecastVertical(x2, y1, y2 - 4, obs, fromId, toId, WIRE.tightPad)
    ) {
      return ptsList(x1, y1, x2, y1, x2, y2);
    }
    midY = resolveClearY(midY, outX, x2, obs, fromId, toId, 1, 12);
    if (x2 < x1 + 4 && midY < fromBottom + 8) {
      midY = Math.max(fromBottom + 10, snapToWireGrid((fromBottom + y2) * 0.5));
    }

    const hitDrop = linecastVertical(x2, midY, y2 - 4, obs, fromId, toId, WIRE.tightPad);
    if (!hitDrop) return ptsList(x1, y1, outX, y1, outX, midY, x2, midY, x2, y2);
    const detourX = resolveClearX(x2, midY, y2 - 14, obs, fromId, toId, x2 >= outX ? -1 : 1, 16);
    const entryY = resolveClearY(Math.min(y2 - 12, hitDrop.bottom + 14), detourX, x2, obs, fromId, toId, -1, 12);
    return ptsList(x1, y1, outX, y1, outX, midY, detourX, midY, detourX, entryY, x2, entryY, x2, y2);
  }

  if (toLeft !== undefined && toLeft !== null && toLeft > outX + 10) {
    if (
      inY >= y1 &&
      !linecastHorizontal(y1, x1 + 4, x2, obs, fromId, toId, WIRE.tightPad) &&
      !linecastVertical(x2, y1, y2 - 4, obs, fromId, toId, WIRE.tightPad)
    ) {
      return ptsList(x1, y1, x2, y1, x2, y2);
    }
    let channelX = Math.max(outX, Math.min(toLeft - 14, (x1 + toLeft) * 0.5 + stubRank * WIRE_LANE_STEP));
    inY = resolveClearY(inY, channelX, x2, obs, fromId, toId, -1, WIRE.pad);
    channelX = resolveClearX(channelX, y1, inY, obs, fromId, toId, 0, WIRE.pad);
    if (!linecastHorizontal(y1, x1 + 4, channelX, obs, fromId, toId, WIRE.tightPad)) {
      return ptsList(x1, y1, channelX, y1, channelX, inY, x2, inY, x2, y2);
    }
    const dipY = resolveClearY(fromBottom + 16 + stubRank * WIRE_LANE_STEP, outX, channelX, obs, fromId, toId, 1, WIRE.pad);
    outX = resolveClearX(outX, y1, dipY, obs, fromId, toId, 1, WIRE.pad);
    channelX = resolveClearX(channelX, dipY, inY, obs, fromId, toId, 1, WIRE.pad);
    return ptsList(x1, y1, outX, y1, outX, dipY, channelX, dipY, channelX, inY, x2, inY, x2, y2);
  }

  inY = resolveClearY(inY, outX, x2, obs, fromId, toId, -1, WIRE.pad);
  let bypassX = Math.max(outX, (toRight !== undefined && toRight !== null ? toRight : x2) + WIRE_GRID + stubRank * WIRE_LANE_STEP);
  bypassX = resolveClearX(bypassX, y1, inY, obs, fromId, toId, 1, 16);
  if (!linecastHorizontal(y1, x1 + 4, bypassX, obs, fromId, toId, WIRE.tightPad)) {
    return ptsList(x1, y1, bypassX, y1, bypassX, inY, x2, inY, x2, y2);
  }
  const underY = resolveClearY(fromBottom + 16 + stubRank * WIRE_LANE_STEP, outX, bypassX, obs, fromId, toId, 1, WIRE.pad);
  outX = resolveClearX(outX, y1, underY, obs, fromId, toId, 1, WIRE.pad);
  bypassX = resolveClearX(bypassX, underY, inY, obs, fromId, toId, 1, 16);
  return ptsList(x1, y1, outX, y1, outX, underY, bypassX, underY, bypassX, inY, x2, inY, x2, y2);
}

function buildPinnedWireWaypoints(w, autoPtsInput) {
  const pins = w.pins;
  let pts =
    autoPtsInput && autoPtsInput.length >= 2
      ? autoPtsInput.map((pt) => ({ x: pt.x, y: pt.y }))
      : ptsList(w.x1, w.y1, w.x2, w.y2);
  if (!pins || pins.length === 0) return simplifyOrthogonalPoints(pts);

  for (let i = 0; i < pins.length; i++) {
    const p = pins[i];
    if (!p || pts.length < 2) continue;

    let s = -1;
    if (typeof p.segIdx === "number" && p.segIdx >= 0 && p.segIdx < pts.length - 1) {
      const a0 = pts[p.segIdx];
      const b0 = pts[p.segIdx + 1];
      const segAxis = Math.abs(a0.y - b0.y) <= Math.abs(a0.x - b0.x) ? "H" : "V";
      if (!p.axis || p.axis === segAxis) s = p.segIdx;
    }

    if (s < 0) {
      let bestScore = Infinity;
      for (let k = 0; k < pts.length - 1; k++) {
        const a0 = pts[k];
        const b0 = pts[k + 1];
        const dx = b0.x - a0.x;
        const dy = b0.y - a0.y;
        const len = Math.hypot(dx, dy);
        if (len < 2) continue;
        const isH = Math.abs(dy) <= Math.abs(dx);
        if (p.axis && ((p.axis === "H" && !isH) || (p.axis === "V" && isH))) continue;
        let t = ((p.x - a0.x) * dx + (p.y - a0.y) * dy) / (len * len);
        const inSpan = t >= -0.05 && t <= 1.05;
        t = Math.max(0, Math.min(1, t));
        const d = Math.hypot(p.x - (a0.x + t * dx), p.y - (a0.y + t * dy));
        const isStub = (k === 0 || k === pts.length - 2) && pts.length > 3;
        const score = d + (inSpan ? 0 : 80) + (isStub ? 40 : 0);
        if (score < bestScore) {
          bestScore = score;
          s = k;
        }
      }
    }
    if (s < 0) s = 0;

    const a = pts[s];
    const b = pts[s + 1];
    if (Math.abs(a.y - b.y) <= Math.abs(a.x - b.x)) {
      const targetY = p.y;
      p.axis = "H";
      if (s > 0 && s < pts.length - 2) {
        pts[s].y = targetY;
        pts[s + 1].y = targetY;
        p.x = Math.round((pts[s].x + pts[s + 1].x) * 0.5);
        p.y = targetY;
        p.segIdx = s;
      } else if (s === 0 && s === pts.length - 2) {
        const dirX = w.x2 >= w.x1 ? 1 : -1;
        const stubLen = Math.min(WIRE_GRID, Math.max(8, Math.abs(w.x2 - w.x1) * 0.25));
        const xA = w.x1 + dirX * stubLen;
        const xB = w.x2 - dirX * stubLen;
        pts = ptsList(w.x1, w.y1, xA, w.y1, xA, targetY, xB, targetY, xB, w.y2, w.x2, w.y2);
        p.x = Math.round((xA + xB) * 0.5);
        p.y = targetY;
        p.segIdx = 2;
      } else if (s === 0) {
        if (pts.length === 3) {
          const stubX = w.x1 + (w.x2 >= w.x1 ? WIRE_GRID : -WIRE_GRID);
          pts = ptsList(w.x1, w.y1, stubX, w.y1, stubX, targetY, w.x2, targetY, w.x2, w.y2);
          p.x = Math.round((stubX + w.x2) * 0.5);
          p.y = targetY;
          p.segIdx = 2;
        } else if (pts.length >= 4) {
          pts[2].y = targetY;
          pts[3].y = targetY;
          p.x = Math.round((pts[2].x + pts[3].x) * 0.5);
          p.y = targetY;
          p.segIdx = 2;
        }
      } else {
        const stubX = w.x2 + (pts[s].x >= w.x2 ? 18 : -18);
        pts[s].y = targetY;
        pts.splice(s + 1, 1, { x: stubX, y: targetY }, { x: stubX, y: w.y2 }, { x: w.x2, y: w.y2 });
        p.x = Math.round((pts[s].x + stubX) * 0.5);
        p.y = targetY;
        p.segIdx = s;
      }
    } else {
      let targetX = !w.isBottomPort && s <= 1 ? Math.max(w.x1 + 12, p.x) : p.x;
      p.axis = "V";
      p.x = targetX;
      if (s > 0 && s < pts.length - 2) {
        pts[s].x = targetX;
        pts[s + 1].x = targetX;
        p.y = Math.round((pts[s].y + pts[s + 1].y) * 0.5);
        p.segIdx = s;
      } else if (s === 0 && s === pts.length - 2) {
        const dirY = w.y2 >= w.y1 ? 1 : -1;
        const stubLen = Math.min(18, Math.max(8, Math.abs(w.y2 - w.y1) * 0.25));
        const yA = w.y1 + dirY * stubLen;
        const yB = w.y2 - dirY * stubLen;
        pts = ptsList(w.x1, w.y1, w.x1, yA, targetX, yA, targetX, yB, w.x2, yB, w.x2, w.y2);
        p.y = Math.round((yA + yB) * 0.5);
        p.segIdx = 2;
      } else if (s === 0) {
        const nextY = pts[1].y;
        const yA = w.y1 + (nextY >= w.y1 ? 18 : -18);
        pts.splice(0, 2, { x: w.x1, y: w.y1 }, { x: w.x1, y: yA }, { x: targetX, y: yA }, { x: targetX, y: nextY });
        p.y = Math.round((yA + nextY) * 0.5);
        p.segIdx = 2;
      } else {
        const topY = pts[s].y;
        const yB = w.y2 - (w.y2 >= topY ? 18 : -18);
        pts[s].x = targetX;
        pts.splice(s + 1, 1, { x: targetX, y: yB }, { x: w.x2, y: yB }, { x: w.x2, y: w.y2 });
        p.y = Math.round((topY + yB) * 0.5);
        p.segIdx = s;
      }
    }
  }
  return pts;
}

function getChapterOutputPortSlot(ch, targetChapterLower) {
  const outPorts = ch.outPorts || [];
  let matchIdx = 0;
  if (targetChapterLower) {
    for (let i = 0; i < outPorts.length; i++) {
      if (outPorts[i].targetLower === targetChapterLower) {
        matchIdx = i;
        break;
      }
    }
  }
  return outPorts[matchIdx] || { x: ch.x + ch.w * 0.5, y: ch.y + ch.h, side: "bottom", idx: 0 };
}

function updateWireBounds(w, pts) {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (let i = 0; i < pts.length; i++) {
    const p = pts[i];
    if (p.x < minX) minX = p.x;
    if (p.y < minY) minY = p.y;
    if (p.x > maxX) maxX = p.x;
    if (p.y > maxY) maxY = p.y;
  }
  w.minX = minX - WIRE.bboxPad;
  w.minY = minY - WIRE.bboxPad;
  w.maxX = maxX + WIRE.bboxPad;
  w.maxY = maxY + WIRE.bboxPad;
  if (!w.bbox) {
    w.bbox = { minX: w.minX, minY: w.minY, maxX: w.maxX, maxY: w.maxY };
  } else {
    w.bbox.minX = w.minX;
    w.bbox.minY = w.minY;
    w.bbox.maxX = w.maxX;
    w.bbox.maxY = w.maxY;
  }
}

function aabbIntersects(a, b) {
  return a.maxX >= b.minX && a.minX <= b.maxX && a.maxY >= b.minY && a.minY <= b.maxY;
}

function buildSideArrivalWaypoints(
  x1,
  y1,
  x2,
  y2,
  isBottomPort,
  toSide,
  obs,
  fromObj,
  toObj,
  portIdx,
  totalPorts,
  preferredOutX
) {
  const idx = portIdx || 0;
  const pCount = Math.max(1, totalPorts || idx + 1);
  const goesDown = fromObj && fromObj.bottom !== undefined ? y2 > fromObj.bottom + 4 : y2 > y1 + 16;
  const stubRank = goesDown ? Math.max(0, pCount - 1 - idx) : idx;
  const baseOutX = typeof preferredOutX === "number" && !isNaN(preferredOutX) ? preferredOutX : x1 + 16 + stubRank * 14;

  if (toSide === "right") {
    if (!isBottomPort && Math.abs(y1 - y2) <= 2 && !linecastHorizontal(y2, x1 + 4, x2, obs, fromObj, toObj, WIRE.tightPad)) {
      return ptsList(x1, y1, x2, y2);
    }
    const sx = isBottomPort ? x1 : resolveClearX(baseOutX, y1, y2, obs, fromObj, toObj, 1, WIRE.pad);
    const sy = isBottomPort ? resolveClearY(y1 + 16, sx, x2, obs, fromObj, toObj, 1, 12) : y1;
    if (!linecastVertical(sx, sy, y2, obs, fromObj, toObj, WIRE.tightPad) && !linecastHorizontal(y2, sx, x2, obs, fromObj, toObj, WIRE.tightPad)) {
      return ptsList(x1, y1, sx, sy, sx, y2, x2, y2);
    }
    const clearY = resolveClearY(Math.max(sy, y2), sx, x2 - 12, obs, fromObj, toObj, 1, 12);
    return ptsList(x1, y1, sx, sy, sx, clearY, x2 - 16, clearY, x2 - 16, y2, x2, y2);
  }

  const sx = isBottomPort ? x1 : resolveClearX(baseOutX, y1, y2, obs, fromObj, toObj, 1, WIRE.pad);
  if (
    !linecastVertical(sx, isBottomPort ? y1 + 4 : y1, y2, obs, fromObj, toObj, WIRE.tightPad) &&
    !linecastHorizontal(y2, x2, sx, obs, fromObj, toObj, WIRE.tightPad)
  ) {
    return isBottomPort ? ptsList(x1, y1, x1, y2, x2, y2) : ptsList(x1, y1, sx, y1, sx, y2, x2, y2);
  }
  const clearY = resolveClearY(y2, x2 + 12, sx, obs, fromObj, toObj, 1, 12);
  return ptsList(x1, y1, sx, y1, sx, clearY, x2 + 16, clearY, x2 + 16, y2, x2, y2);
}

function buildSideChapterToChapterWaypoints(x1, y1, x2, y2, fromSide, obs, fromObj, toObj, portIdx) {
  const dir = fromSide === "left" ? -1 : 1;
  const stubOffset = WIRE_GRID + (portIdx || 0) * WIRE_LANE_STEP;
  const approachY = resolveClearY(y2 - WIRE_GRID, x1, x2, obs, fromObj, toObj, -1, WIRE.pad);
  const corridorX = resolveClearX(x1 + dir * stubOffset, y1, approachY, obs, fromObj, toObj, dir, 16);
  if (
    !linecastVertical(corridorX, y1, approachY, obs, fromObj, toObj, WIRE.tightPad) &&
    !linecastHorizontal(approachY, corridorX, x2, obs, fromObj, toObj, WIRE.tightPad)
  ) {
    return ptsList(x1, y1, corridorX, y1, corridorX, approachY, x2, approachY, x2, y2);
  }
  const midX = resolveClearX((corridorX + x2) * 0.5, y1, approachY, obs, fromObj, toObj, dir, 16);
  const halfY = (y1 + approachY) * 0.5;
  return ptsList(x1, y1, corridorX, y1, corridorX, halfY, midX, halfY, midX, approachY, x2, approachY, x2, y2);
}

function getNodePortTargetCoords(fromNode, pt) {
  if (!fromNode || !pt) return null;
  const targetChapter = (pt.targetChapter || "").trim();
  if (targetChapter) {
    const fromCh = fromNode.chapterObj || studioGraphState.chapterByKey.get(fromNode.chapterLower);
    if (!fromCh || fromCh.hidden) return null;
    const outPort = getChapterOutputPortSlot(fromCh, targetChapter.toLowerCase());
    return { x2: outPort.x, y2: outPort.y, toObj: null, outSide: outPort.side || "bottom" };
  }
  const targetId = pt.targetId || "";
  if (!targetId || targetId.toUpperCase() === "END") return null;
  const toNode = studioGraphState.nodeById.get(targetId);
  if (!toNode) return null;
  return { x2: toNode.inX, y2: toNode.inY, toObj: toNode, outSide: "bottom" };
}

function computeNodeChoiceExitLanes(fromNode, obs) {
  if (!fromNode || !fromNode.hasChoices || !fromNode.ports || fromNode.ports.length === 0) return null;
  const ports = fromNode.ports;
  const n = ports.length;
  const x1 = fromNode.right;
  const fromBottom = fromNode.bottom;
  const lanes = new Array(n);
  const targets = ports.map((p) => getNodePortTargetCoords(fromNode, p));

  let prevDownOutX = x1;
  for (let i = n - 1; i >= 0; i--) {
    const pt = ports[i];
    const y1 = pt.y;
    const t = targets[i];
    if (!t) {
      lanes[i] = x1 + WIRE_GRID + (n - 1 - i) * WIRE_LANE_STEP;
      continue;
    }
    if (t.y2 <= fromBottom + 4) {
      lanes[i] = x1 + WIRE_GRID + i * WIRE_LANE_STEP;
      continue;
    }

    const minLaneX = prevDownOutX + (prevDownOutX === x1 ? WIRE_GRID : WIRE_LANE_STEP);
    let blocksUpperDirectDrop = false;
    if (t.x2 > minLaneX + 12) {
      for (let k = 0; k < i; k++) {
        const tk = targets[k];
        if (tk && tk.y2 > fromBottom + 4 && tk.x2 >= minLaneX && tk.x2 <= t.x2) {
          blocksUpperDirectDrop = true;
          break;
        }
      }
    }

    if (
      t.outSide === "bottom" &&
      t.x2 >= minLaneX &&
      !blocksUpperDirectDrop &&
      !linecastHorizontal(y1, x1 + 4, t.x2, obs, fromNode, t.toObj, WIRE.tightPad) &&
      !linecastVertical(t.x2, y1, t.y2 - 4, obs, fromNode, t.toObj, WIRE.tightPad)
    ) {
      lanes[i] = t.x2;
      if (t.x2 <= x1 + PAD.x) prevDownOutX = t.x2;
    } else {
      const checkToY = Math.max(fromBottom + 16, Math.min(t.y2, fromBottom + 32));
      const candLane = Math.max(minLaneX, resolveClearX(minLaneX, y1, checkToY, obs, fromNode, t.toObj, 1, WIRE.pad));
      lanes[i] = candLane;
      prevDownOutX = candLane;
    }
  }
  return lanes;
}

function routeSingleWireBasePts(w, nodeObstacles, chapterObstacles) {
  if (w.kind === "scene-to-chapter-out") {
    const fromNode = w.fromNode;
    const fromCh = w.fromChapterObj;
    const localNodes = fromCh && fromCh.nodes ? fromCh.nodes : nodeObstacles;
    const outPort = getChapterOutputPortSlot(fromCh, w.targetChapterLower);
    const outSide = outPort.side || "bottom";
    const choiceLanes = !w.isBottomPort ? computeNodeChoiceExitLanes(fromNode, localNodes) : null;
    const preferredOutX = choiceLanes ? choiceLanes[w.portIdx] : undefined;
    w.toChapterOutPort = outPort;
    w.toSide = outSide;
    w.x1 = w.portRef.x;
    w.y1 = w.portRef.y;
    w.x2 = outPort.x;
    w.y2 = outPort.y;
    w.unreachable = fromNode.unreachable;
    const totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
    const rawPts =
      outSide === "left" || outSide === "right"
        ? buildSideArrivalWaypoints(w.x1, w.y1, w.x2, w.y2, w.isBottomPort, outSide, localNodes, fromNode, null, w.portIdx, totalPorts, preferredOutX)
        : buildWireWaypoints(w.x1, w.y1, w.x2, w.y2, fromNode.bottom, w.portIdx, outPort.x - 8, outPort.x + 8, w.isBottomPort, localNodes, fromNode, null, totalPorts, preferredOutX);
    const autoPts = simplifyOrthogonalPoints(rawPts);
    w.basePts = w.pins && w.pins.length > 0 ? buildPinnedWireWaypoints(w, autoPts) : autoPts;
  } else if (w.kind === "scene-to-scene") {
    const fromNode = w.fromNode;
    const toNode = w.toNode;
    const chObj = fromNode.chapterObj || toNode.chapterObj;
    const localNodes = chObj && chObj.nodes ? chObj.nodes : nodeObstacles;
    const choiceLanes = !w.isBottomPort ? computeNodeChoiceExitLanes(fromNode, localNodes) : null;
    const preferredOutX = choiceLanes ? choiceLanes[w.portIdx] : undefined;
    w.x1 = w.portRef.x;
    w.y1 = w.portRef.y;
    w.x2 = toNode.inX;
    w.y2 = toNode.inY;
    w.unreachable = fromNode.unreachable;
    const totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
    const rawPts = buildWireWaypoints(
      w.x1,
      w.y1,
      w.x2,
      w.y2,
      fromNode.bottom,
      w.portIdx,
      toNode.left,
      toNode.right,
      w.isBottomPort,
      localNodes,
      fromNode,
      toNode,
      totalPorts,
      preferredOutX
    );
    const autoPts = simplifyOrthogonalPoints(rawPts);
    w.basePts = w.pins && w.pins.length > 0 ? buildPinnedWireWaypoints(w, autoPts) : autoPts;
  } else if (w.kind === "chapter-to-chapter") {
    const fromCh = w.fromChapterObj;
    const toCh = w.toChapterObj;
    const outPort = w.fromChapterOutPort;
    const fromSide = outPort.side || "bottom";
    w.isBottomPort = fromSide === "bottom";
    w.x1 = outPort.x;
    w.y1 = outPort.y;
    w.x2 = toCh.inX;
    w.y2 = toCh.inY;
    w.unreachable = false;
    const rawPts =
      fromSide === "left" || fromSide === "right"
        ? buildSideChapterToChapterWaypoints(w.x1, w.y1, w.x2, w.y2, fromSide, chapterObstacles, fromCh, toCh, w.portIdx)
        : buildWireWaypoints(
            w.x1,
            w.y1,
            w.x2,
            w.y2,
            Math.max(w.y1, w.y2 - CARD.minChapterH),
            w.portIdx,
            toCh.x,
            toCh.x + toCh.w,
            true,
            chapterObstacles,
            fromCh,
            toCh,
            fromCh.outPorts ? fromCh.outPorts.length : 1
          );
    const autoPts = simplifyOrthogonalPoints(rawPts);
    w.basePts = w.pins && w.pins.length > 0 ? buildPinnedWireWaypoints(w, autoPts) : autoPts;
  } else if (w.kind === "chapter-in-to-entry") {
    const ch = w.toChapterObj;
    const toNode = w.toNode;
    const localNodes = ch && ch.nodes ? ch.nodes : nodeObstacles;
    w.x1 = ch.inX;
    w.y1 = ch.inY;
    w.x2 = toNode.inX;
    w.y2 = toNode.inY;
    w.unreachable = toNode.unreachable;
    const rawPts = buildWireWaypoints(w.x1, w.y1, w.x2, w.y2, ch.y + 28, 0, toNode.left, toNode.right, true, localNodes, null, toNode, 1);
    const autoPts = simplifyOrthogonalPoints(rawPts);
    w.basePts = w.pins && w.pins.length > 0 ? buildPinnedWireWaypoints(w, autoPts) : autoPts;
  }
  updateWireBounds(w, w.basePts);
}

function createWireRecord(spec) {
  const savedPins = studioGraphState.wirePins.get(spec.wireKey);
  return {
    kind: spec.kind,
    wireKey: spec.wireKey,
    pins: savedPins ? savedPins.map((p) => ({ x: p.x, y: p.y, axis: p.axis, segIdx: p.segIdx })) : [],
    fromNode: spec.fromNode || null,
    toNode: spec.toNode || null,
    fromChapterObj: spec.fromChapterObj || null,
    toChapterObj: spec.toChapterObj || null,
    chapterScope: spec.chapterScope || null,
    fromChapterOutPort: spec.fromChapterOutPort || null,
    toChapterOutPort: null,
    portRef: spec.portRef || null,
    portIdx: spec.portIdx || 0,
    isBottomPort: Boolean(spec.isBottomPort),
    targetChapterLower: spec.targetChapterLower || "",
    targetGroupKey: spec.targetGroupKey || "",
    unreachable: Boolean(spec.unreachable),
    x1: 0,
    y1: 0,
    x2: 0,
    y2: 0,
    minX: 0,
    minY: 0,
    maxX: 0,
    maxY: 0,
    basePts: [],
    pts: [],
    bridges: null,
    bbox: null
  };
}

function buildWireCatalogFromMemory() {
  const nodes = studioGraphState.nodes;
  const chapters = studioGraphState.chapters;
  const wires = [];

  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    for (let j = 0; j < ch.outPorts.length; j++) {
      ch.outPorts[j].connected = Boolean(ch.outPorts[j].targetLower);
    }
  }

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    for (let j = 0; j < nd.ports.length; j++) {
      const pt = nd.ports[j];
      pt.connected = Boolean((pt.targetId && pt.targetId.toUpperCase() !== "END") || (pt.targetChapter && pt.targetChapter.trim()));
    }
    if (nd.implicitPort) {
      const ip = nd.implicitPort;
      ip.connected = Boolean((ip.targetId && ip.targetId.toUpperCase() !== "END") || (ip.targetChapter && ip.targetChapter.trim()));
    }
  }

  for (let i = 0; i < nodes.length; i++) {
    const fromNode = nodes[i];
    const portList =
      fromNode.hasChoices && fromNode.ports.length > 0
        ? fromNode.ports
        : fromNode.implicitPort
          ? [fromNode.implicitPort]
          : [];

    for (let j = 0; j < portList.length; j++) {
      const pt = portList[j];
      const targetId = pt.targetId || "";
      const targetChapter = (pt.targetChapter || "").trim();
      if ((!targetId || targetId === "END") && !targetChapter) continue;

      if (targetChapter) {
        const fromCh = fromNode.chapterObj || studioGraphState.chapterByKey.get(fromNode.chapterLower);
        if (!fromCh || fromCh.hidden) continue;
        const targetChLower = targetChapter.toLowerCase();
        pt.connected = true;
        wires.push(
          createWireRecord({
            kind: "scene-to-chapter-out",
            wireKey: (fromNode.id + ":" + (pt.idx || 0)).toLowerCase(),
            fromNode,
            fromChapterObj: fromCh,
            chapterScope: fromCh,
            portRef: pt,
            portIdx: pt.idx || 0,
            isBottomPort: pt.isBottom,
            targetChapterLower: targetChLower,
            targetGroupKey: "ch_out:" + fromNode.chapterLower + "->" + targetChLower,
            unreachable: fromNode.unreachable
          })
        );
        continue;
      }

      const toNode = studioGraphState.nodeById.get(targetId);
      if (!toNode || (fromNode.chapterLower && toNode.chapterLower && fromNode.chapterLower !== toNode.chapterLower)) {
        continue;
      }
      pt.connected = true;
      wires.push(
        createWireRecord({
          kind: "scene-to-scene",
          wireKey: (fromNode.id + ":" + (pt.idx || 0)).toLowerCase(),
          fromNode,
          toNode,
          chapterScope: fromNode.chapterObj || toNode.chapterObj || null,
          portRef: pt,
          portIdx: pt.idx || 0,
          isBottomPort: pt.isBottom,
          targetGroupKey: "scene:" + targetId,
          unreachable: fromNode.unreachable
        })
      );
    }
  }

  for (let i = 0; i < chapters.length; i++) {
    const fromCh = chapters[i];
    if (fromCh.hidden) continue;
    if (!fromCh.isRoot) {
      let entryNode = fromCh.startSceneId ? studioGraphState.nodeById.get(fromCh.startSceneId) : null;
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
        wires.push(
          createWireRecord({
            kind: "chapter-in-to-entry",
            wireKey: ("ch_in:" + fromCh.nameLower).toLowerCase(),
            toNode: entryNode,
            fromChapterObj: fromCh,
            toChapterObj: fromCh,
            chapterScope: fromCh,
            isBottomPort: true,
            targetChapterLower: fromCh.nameLower,
            targetGroupKey: "scene:" + entryNode.id,
            unreachable: entryNode.unreachable
          })
        );
      }
    }
    for (let j = 0; j < fromCh.outPorts.length; j++) {
      const op = fromCh.outPorts[j];
      if (!op.targetLower) continue;
      const toCh = studioGraphState.chapterByKey.get(op.targetLower);
      if (!toCh || toCh.hidden || toCh === fromCh) continue;
      op.connected = true;
      wires.push(
        createWireRecord({
          kind: "chapter-to-chapter",
          wireKey: ("ch_out:" + fromCh.nameLower + "->" + op.targetLower).toLowerCase(),
          fromChapterObj: fromCh,
          toChapterObj: toCh,
          fromChapterOutPort: op,
          portRef: op,
          portIdx: j,
          isBottomPort: (op.side || "bottom") === "bottom",
          targetChapterLower: op.targetLower,
          targetGroupKey: "ch_in:" + op.targetLower,
          unreachable: false
        })
      );
    }
  }
  return wires;
}

function copyBasePtsToPts(w) {
  const src = w.basePts;
  const n = src.length;
  const dst = w.pts || (w.pts = []);
  dst.length = n;
  for (let i = 0; i < n; i++) {
    const sp = src[i];
    if (dst[i]) {
      dst[i].x = sp.x;
      dst[i].y = sp.y;
    } else {
      dst[i] = { x: sp.x, y: sp.y };
    }
  }
}

function separateOverlappingVerticalSegments(wires) {
  const vSegs = [];
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.pins && w.pins.length > 0) continue;
    const pts = w.pts;
    if (!pts || pts.length < 4) continue;
    for (let j = 1; j < pts.length - 2; j++) {
      const p0 = pts[j];
      const p1 = pts[j + 1];
      if (Math.abs(p0.x - p1.x) < 0.5 && Math.abs(p1.y - p0.y) > 12) {
        const yA = p0.y;
        const yB = p1.y;
        vSegs.push({
          wire: w,
          segIdx: j,
          x: p0.x,
          minY: yA < yB ? yA : yB,
          maxY: yA > yB ? yA : yB
        });
      }
    }
  }
  if (vSegs.length < 2) return;

  vSegs.sort((a, b) => a.x - b.x);
  const visited = new Uint8Array(vSegs.length);
  for (let i = 0; i < vSegs.length; i++) {
    if (visited[i]) continue;
    const sA = vSegs[i];
    let cluster = null;
    for (let j = i + 1; j < vSegs.length; j++) {
      const sB = vSegs[j];
      if (sB.x - sA.x >= 8) break;
      if (visited[j] || sB.wire === sA.wire || sB.wire.chapterScope !== sA.wire.chapterScope) continue;
      if (Math.min(sA.maxY, sB.maxY) - Math.max(sA.minY, sB.minY) > 8) {
        if (!cluster) {
          cluster = [sA];
          visited[i] = 1;
        }
        cluster.push(sB);
        visited[j] = 1;
      }
    }
    if (cluster && cluster.length >= 2) {
      const baseX = cluster[0].x;
      const isLeftPerimeter = cluster.every((it) => baseX < it.wire.x1 - WIRE_GRID);
      cluster.sort((a, b) => {
        if (isLeftPerimeter && Math.abs(a.wire.x1 - b.wire.x1) > 1) return a.wire.x1 - b.wire.x1;
        if (Math.abs(a.wire.y1 - b.wire.y1) > 1) return b.wire.y1 - a.wire.y1;
        return (a.wire.portIdx || 0) - (b.wire.portIdx || 0);
      });
      for (let k = 0; k < cluster.length; k++) {
        const dx = isLeftPerimeter ? -(cluster.length - 1 - k) * WIRE_LANE_STEP : k * WIRE_LANE_STEP;
        const item = cluster[k];
        const nextX = baseX + dx;
        item.wire.pts[item.segIdx].x = nextX;
        item.wire.pts[item.segIdx + 1].x = nextX;
      }
    }
  }
}

function classifyHorizontalSegmentRank(pts, segIdx, y) {
  const prevY = pts[segIdx - 1].y;
  const nextY = segIdx + 2 < pts.length ? pts[segIdx + 2].y : y + WIRE_GRID;
  const goDownStart = prevY > y;
  const goDownEnd = nextY > y;
  if (!goDownStart && goDownEnd) return 0;
  if (goDownStart && goDownEnd) return 1;
  if (!goDownStart && !goDownEnd) return 2;
  return 3;
}

function separateOverlappingWireSegments(wires) {
  const hSegs = [];
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.pins && w.pins.length > 0) continue;
    const pts = w.pts;
    if (!pts || pts.length < 3) continue;
    const isSideTerminal = w.toSide === "left" || w.toSide === "right";
    const maxSegIdx = isSideTerminal ? pts.length - 2 : pts.length - 3;
    for (let j = 1; j <= maxSegIdx; j++) {
      const p0 = pts[j];
      const p1 = pts[j + 1];
      if (Math.abs(p0.y - p1.y) < 0.5 && Math.abs(p1.x - p0.x) > 10) {
        const xA = p0.x;
        const xB = p1.x;
        hSegs.push({
          wire: w,
          segIdx: j,
          isTerminalSide: isSideTerminal && j === pts.length - 2,
          y: p0.y,
          startX: xA,
          endX: xB,
          dirX: xB >= xA ? 1 : -1,
          minX: xA < xB ? xA : xB,
          maxX: xA > xB ? xA : xB,
          typeRank: classifyHorizontalSegmentRank(pts, j, p0.y)
        });
      }
    }
  }
  if (hSegs.length < 2) return;

  hSegs.sort((a, b) => a.y - b.y);
  const visited = new Uint8Array(hSegs.length);
  for (let i = 0; i < hSegs.length; i++) {
    if (visited[i]) continue;
    const sA = hSegs[i];
    let cluster = null;
    for (let j = i + 1; j < hSegs.length; j++) {
      const sB = hSegs[j];
      if (sB.y - sA.y > 20) break;
      if (visited[j] || sB.wire === sA.wire || sB.wire.chapterScope !== sA.wire.chapterScope) continue;
      if (Math.min(sA.maxX, sB.maxX) - Math.max(sA.minX, sB.minX) > 6) {
        if (!cluster) {
          cluster = [sA];
          visited[i] = 1;
        }
        cluster.push(sB);
        visited[j] = 1;
      }
    }
    if (cluster && cluster.length >= 2) {
      cluster.sort((a, b) => {
        if (a.typeRank !== b.typeRank) return a.typeRank - b.typeRank;
        const keyFor = (s) => {
          if (s.typeRank === 0) return s.dirX < 0 ? s.startX : -s.startX;
          if (s.typeRank === 1) return s.dirX > 0 ? s.startX : -s.startX;
          if (s.typeRank === 2) return s.dirX < 0 ? -s.endX : s.endX;
          return s.dirX < 0 ? -s.startX : s.startX;
        };
        const diff = keyFor(a) - keyFor(b);
        if (Math.abs(diff) > 1) return diff;
        return a.y - b.y || a.wire.y1 - b.wire.y1;
      });

      const baseClusterY = snapToWireGrid(cluster[0].y);
      let minAllowedY = -Infinity;
      let maxAllowedY = Infinity;
      for (let k = 0; k < cluster.length; k++) {
        const it = cluster[k];
        const pts = it.wire.pts;
        const prevY = pts[it.segIdx - 1].y;
        const hasNext = it.segIdx + 2 < pts.length;
        const nextY = hasNext ? pts[it.segIdx + 2].y : Infinity;
        if (prevY < it.y && prevY + 12 > minAllowedY) minAllowedY = prevY + 12;
        if (hasNext && nextY < it.y && nextY + 12 > minAllowedY) minAllowedY = nextY + 12;
        if (prevY > it.y && prevY - 12 < maxAllowedY) maxAllowedY = prevY - 12;
        if (hasNext && nextY > it.y) {
          const clearBot = it.segIdx + 2 === pts.length - 1 && it.wire.toNode ? nextY - WIRE_GRID : nextY - 12;
          if (clearBot < maxAllowedY) maxAllowedY = clearBot;
        } else if (it.isTerminalSide && it.wire.fromChapterObj) {
          const chBot = it.wire.fromChapterObj.y + it.wire.fromChapterObj.h - 12;
          if (chBot < maxAllowedY) maxAllowedY = chBot;
        }
      }
      const totalSpan = (cluster.length - 1) * WIRE_LANE_STEP;
      let startY = Math.round(baseClusterY - totalSpan * 0.5);
      if (startY + totalSpan > maxAllowedY) startY = maxAllowedY - totalSpan;
      if (startY < minAllowedY) startY = minAllowedY;
      for (let k = 0; k < cluster.length; k++) {
        if (cluster[k].isTerminalSide) {
          startY = cluster[k].y - k * WIRE_LANE_STEP;
          break;
        }
      }
      for (let k = 0; k < cluster.length; k++) {
        const item = cluster[k];
        if (item.isTerminalSide) continue;
        const nextY = startY + k * WIRE_LANE_STEP;
        item.wire.pts[item.segIdx].y = nextY;
        item.wire.pts[item.segIdx + 1].y = nextY;
      }
    }
  }
}

function assignTargetCollectorBarsAndSubLanes(wires) {
  studioGraphState.collectorBars = [];
  const byTarget = new Map();
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    const key = w.targetGroupKey || "";
    if (!key) continue;
    let g = byTarget.get(key);
    if (!g) byTarget.set(key, (g = []));
    g.push(w);
  }

  byTarget.forEach((group, groupKey) => {
    if (group.length < 2) return;
    if (group[0].toSide && group[0].toSide !== "bottom") return;

    const tx = group[0].x2;
    const ty = group[0].y2;
    const barY = ty - WIRE.collectorOffset;
    const n = group.length;

    const meta = group.map((w) => {
      const pts = w.pts;
      const len = pts.length;
      const fromX = len >= 3 ? pts[len - 3].x : w.x1;
      const turnY = len >= 3 ? pts[len - 2].y : w.y1;
      const side = len >= 3 ? (fromX < tx - 2 ? -1 : fromX > tx + 2 ? 1 : 0) : 0;
      return { wire: w, side, fromX, turnY };
    });

    meta.sort((a, b) => {
      if (a.side !== b.side) return a.side - b.side;
      if (a.side === -1) return Math.abs(a.turnY - b.turnY) > 1 ? b.turnY - a.turnY : b.fromX - a.fromX;
      if (a.side === 1) return Math.abs(a.turnY - b.turnY) > 1 ? a.turnY - b.turnY : b.fromX - a.fromX;
      if (Math.abs(a.wire.x1 - b.wire.x1) > 1) return a.wire.x1 - b.wire.x1;
      return (a.wire.portIdx || 0) - (b.wire.portIdx || 0);
    });

    let minHorizontalTurnY = barY - WIRE.collectorOffset;
    for (let i = 0; i < meta.length; i++) {
      if (meta[i].side !== 0 && meta[i].turnY < minHorizontalTurnY) minHorizontalTurnY = meta[i].turnY;
    }

    for (let k = 0; k < n; k++) {
      const w = meta[k].wire;
      const dx = Math.round((k - (n - 1) * 0.5) * WIRE_LANE_STEP);
      const slotX = tx + dx;
      const pts = w.pts;

      if (pts.length >= 3) {
        const oldDropX = pts[pts.length - 2].x;
        pts[pts.length - 2].x = slotX;
        pts[pts.length - 1] = { x: slotX, y: barY };
        if (pts.length === 3 && w.fromNode && slotX > oldDropX) {
          for (let s = 0; s < wires.length; s++) {
            const sw = wires[s];
            if (sw !== w && sw.fromNode === w.fromNode && sw.pts && sw.pts.length >= 4) {
              if (Math.abs(sw.pts[1].x - sw.pts[2].x) < 0.5 && sw.pts[1].x > oldDropX - 1 && sw.pts[1].x < slotX + WIRE_LANE_STEP) {
                sw.pts[1].x = slotX + WIRE_LANE_STEP;
                sw.pts[2].x = slotX + WIRE_LANE_STEP;
              }
            }
          }
        }
      } else if (pts.length === 2) {
        if (Math.abs(dx) < 0.5) {
          pts[1] = { x: slotX, y: barY };
        } else {
          const stepY = Math.max(w.y1 + 12, minHorizontalTurnY - WIRE_LANE_STEP);
          w.pts = ptsList(w.x1, w.y1, w.x1, stepY, slotX, stepY, slotX, barY);
        }
      }
    }

    const halfSpan = (n - 1) * 0.5 * WIRE_LANE_STEP;
    studioGraphState.collectorBars.push({
      targetGroupKey: groupKey,
      x: tx,
      portY: ty,
      barY,
      minX: tx + Math.round(-halfSpan),
      maxX: tx + Math.round(halfSpan),
      count: n,
      unreachable: group.every((it) => it.unreachable)
    });
  });
}

function hashWireColorSeed(str) {
  let h = 2166136261 >>> 0;
  for (let i = 0; i < str.length; i++) {
    h = Math.imul((h ^ str.charCodeAt(i)) >>> 0, 16777619) >>> 0;
  }
  return h >>> 0;
}

function pickDistinctPaletteColor(seed, paletteSize, tgtSet, destChSet, srcSet, parentInColor) {
  for (let pass = 0; pass < 3; pass++) {
    for (let step = 0; step < paletteSize; step++) {
      const cand = (seed + step) % paletteSize;
      if (tgtSet.has(cand) || (destChSet && destChSet.has(cand))) continue;
      if (pass < 2 && srcSet.has(cand)) continue;
      if (pass === 0 && cand === parentInColor) continue;
      return cand;
    }
  }
  return seed % paletteSize;
}

function computeWireAndPortColors(wires) {
  const PALETTE_SIZE = 8;
  const nodes = studioGraphState.nodes || [];
  const chapters = studioGraphState.chapters || [];
  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    nd.inColorIdx = -1;
    if (nd.ports) nd.ports.forEach((p) => (p.colorIdx = -1));
    if (nd.implicitPort) nd.implicitPort.colorIdx = -1;
  }
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    ch.inColorIdx = -1;
    if (ch.outPorts) ch.outPorts.forEach((p) => (p.colorIdx = -1));
  }

  const byTarget = new Map();
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    w.colorIdx = -1;
    const key = w.targetGroupKey || "";
    if (!key) continue;
    let g = byTarget.get(key);
    if (!g) byTarget.set(key, (g = []));
    g.push(w);
  }

  const usedAtSource = new Map();
  const usedAtTarget = new Map();
  const getOrCreateSet = (map, key) => {
    let s = map.get(key);
    if (!s) map.set(key, (s = new Set()));
    return s;
  };

  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.kind === "chapter-to-chapter" || w.kind === "chapter-in-to-entry") continue;
    const fromMultiNode = Boolean(w.fromNode && w.fromNode.hasChoices && w.fromNode.ports && w.fromNode.ports.length >= 2);
    const targetGroup = w.targetGroupKey ? byTarget.get(w.targetGroupKey) : null;
    const toMultiTarget = Boolean(targetGroup && targetGroup.length >= 2);
    const destChGroup = w.kind === "scene-to-chapter-out" && w.targetChapterLower ? byTarget.get("ch_in:" + w.targetChapterLower) : null;
    if (!fromMultiNode && !toMultiTarget && !(destChGroup && destChGroup.length >= 2)) continue;

    const srcKey = w.fromNode ? "node:" + w.fromNode.id : "wire:" + i;
    const tgtKey = w.targetGroupKey || "tgt:" + i;
    const destChKey = w.kind === "scene-to-chapter-out" && w.targetChapterLower ? "ch_in:" + w.targetChapterLower : "";
    const srcSet = getOrCreateSet(usedAtSource, srcKey);
    const tgtSet = getOrCreateSet(usedAtTarget, tgtKey);
    const destChSet = destChKey ? getOrCreateSet(usedAtTarget, destChKey) : null;

    let parentInColor = -1;
    if (w.fromNode) {
      const parentInGroup = byTarget.get("scene:" + w.fromNode.id);
      if (parentInGroup && parentInGroup.length === 1 && parentInGroup[0].colorIdx >= 0) {
        parentInColor = parentInGroup[0].colorIdx;
      }
    }

    const seed = (hashWireColorSeed(srcKey) + (w.portIdx || 0) * 3) % PALETTE_SIZE;
    const chosen = pickDistinctPaletteColor(seed, PALETTE_SIZE, tgtSet, destChSet, srcSet, parentInColor);
    w.colorIdx = chosen;
    srcSet.add(chosen);
    tgtSet.add(chosen);
    if (destChSet && (!targetGroup || targetGroup.length === 1)) destChSet.add(chosen);
  }

  byTarget.forEach((group) => {
    const singleColor = group.length === 1 ? group[0].colorIdx : -1;
    for (let k = 0; k < group.length; k++) {
      const w = group[k];
      if (w.toNode) w.toNode.inColorIdx = singleColor;
      else if (w.toChapterOutPort) {
        w.toChapterOutPort.inColorIdx = singleColor;
        w.toChapterOutPort.colorIdx = singleColor;
      }
    }
  });

  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.kind !== "chapter-to-chapter") continue;
    const op = w.fromChapterOutPort;
    const tgtSet = getOrCreateSet(usedAtTarget, w.targetGroupKey || "tgt:" + i);

    if (op && op.inColorIdx >= 0) {
      w.colorIdx = op.inColorIdx;
      op.colorIdx = w.colorIdx;
      tgtSet.add(w.colorIdx);
      continue;
    }

    const fromMultiCh = Boolean(w.fromChapterObj && w.fromChapterObj.outPorts && w.fromChapterObj.outPorts.length >= 2);
    const targetGroup = w.targetGroupKey ? byTarget.get(w.targetGroupKey) : null;
    if (!fromMultiCh && !(targetGroup && targetGroup.length >= 2)) continue;

    const srcKey = w.fromChapterObj ? "ch:" + w.fromChapterObj.nameLower : "wire:" + i;
    const srcSet = getOrCreateSet(usedAtSource, srcKey);
    const seed = (hashWireColorSeed(srcKey) + (w.portIdx || 0) * 3) % PALETTE_SIZE;
    const chosen = pickDistinctPaletteColor(seed, PALETTE_SIZE, tgtSet, null, srcSet, -1);
    w.colorIdx = chosen;
    if (op) op.colorIdx = chosen;
    srcSet.add(chosen);
    tgtSet.add(chosen);
  }

  byTarget.forEach((group) => {
    if (group[0].kind !== "chapter-to-chapter") return;
    const singleColor = group.length === 1 ? group[0].colorIdx : -1;
    for (let k = 0; k < group.length; k++) {
      if (group[k].toChapterObj) group[k].toChapterObj.inColorIdx = singleColor;
    }
  });

  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.kind === "chapter-in-to-entry" && w.fromChapterObj) {
      w.colorIdx = w.fromChapterObj.inColorIdx !== undefined ? w.fromChapterObj.inColorIdx : -1;
      const entryGroup = w.targetGroupKey ? byTarget.get(w.targetGroupKey) : null;
      if (entryGroup && entryGroup.length === 1 && w.toNode) w.toNode.inColorIdx = w.colorIdx;
    }
    if (w.portRef) w.portRef.colorIdx = w.colorIdx;
  }
}

function applyParallelOffsetsAndBundles(wires) {
  for (let i = 0; i < wires.length; i++) copyBasePtsToPts(wires[i]);
  separateOverlappingVerticalSegments(wires);
  separateOverlappingWireSegments(wires);
  assignTargetCollectorBarsAndSubLanes(wires);
  computeWireAndPortColors(wires);
  computeWireCrossingBridges(wires);
}

function computeWireCrossingBridges(wires) {
  const vSegs = [];
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    w.bridges = null;
    if (w.unreachable || !w.pts || w.pts.length < 2) continue;
    const pts = w.pts;
    for (let j = 0; j < pts.length - 1; j++) {
      const p0 = pts[j];
      const p1 = pts[j + 1];
      if (Math.abs(p1.x - p0.x) < 0.5 && Math.abs(p1.y - p0.y) > 4) {
        vSegs.push({
          wire: w,
          x: p0.x,
          minY: Math.min(p0.y, p1.y),
          maxY: Math.max(p0.y, p1.y)
        });
      }
    }
  }

  if (vSegs.length === 0) {
    for (let i = 0; i < wires.length; i++) {
      if (wires[i].pts) updateWireBounds(wires[i], wires[i].pts);
    }
    return;
  }

  vSegs.sort((a, b) => a.x - b.x);
  const vLen = vSegs.length;

  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (!w.pts || w.pts.length < 2) continue;
    updateWireBounds(w, w.pts);
    if (w.unreachable) continue;

    const pts = w.pts;
    let wireBridges = null;
    for (let j = 0; j < pts.length - 1; j++) {
      const p0 = pts[j];
      const p1 = pts[j + 1];
      if (Math.abs(p1.y - p0.y) >= 0.5 || Math.abs(p1.x - p0.x) <= 8) continue;
      const y = p0.y;
      const minX = Math.min(p0.x, p1.x) + 3.5;
      const maxX = Math.max(p0.x, p1.x) - 3.5;
      if (minX > maxX) continue;

      let lo = 0;
      let hi = vLen - 1;
      let startIdx = vLen;
      while (lo <= hi) {
        const mid = (lo + hi) >> 1;
        if (vSegs[mid].x >= minX) {
          startIdx = mid;
          hi = mid - 1;
        } else {
          lo = mid + 1;
        }
      }

      let deduped = null;
      let lastBridgeX = -Infinity;
      for (let k = startIdx; k < vLen; k++) {
        const vs = vSegs[k];
        if (vs.x > maxX) break;
        if (vs.wire !== w && y > vs.minY + 2.5 && y < vs.maxY - 2.5 && vs.x - lastBridgeX >= 1.5) {
          if (!deduped) deduped = [];
          deduped.push(vs.x);
          lastBridgeX = vs.x;
        }
      }
      if (deduped) {
        if (!wireBridges) wireBridges = {};
        wireBridges[j] = deduped;
      }
    }
    w.bridges = wireBridges;
  }
}

function routeStudioWires(dirtyNodeIds, dirtyAabbs, changedChapterFramesSet) {
  const nodeObstacles = studioGraphState.nodes;
  const chapterObstacles = studioGraphState.chapters;

  if (!dirtyNodeIds || studioGraphState.wires.length === 0) {
    const wires = buildWireCatalogFromMemory();
    for (let i = 0; i < wires.length; i++) {
      routeSingleWireBasePts(wires[i], nodeObstacles, chapterObstacles);
    }
    studioGraphState.wires = wires;
    applyParallelOffsetsAndBundles(wires);
    studioGraphState.staticBatchDirty = true;
    return;
  }

  const wires = studioGraphState.wires;
  const hasChangedChapters = Boolean(changedChapterFramesSet && changedChapterFramesSet.size > 0);
  let anyDirty = false;

  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    const isDirty =
      Boolean(w.fromNode && dirtyNodeIds.has(w.fromNode.id)) ||
      Boolean(w.toNode && dirtyNodeIds.has(w.toNode.id)) ||
      Boolean(
        hasChangedChapters &&
          ((w.fromChapterObj && changedChapterFramesSet.has(w.fromChapterObj)) ||
            (w.toChapterObj && changedChapterFramesSet.has(w.toChapterObj)))
      ) ||
      Boolean(dirtyAabbs && w.bbox && dirtyAabbs.some((box) => aabbIntersects(w.bbox, box)));
    if (isDirty) {
      routeSingleWireBasePts(w, nodeObstacles, chapterObstacles);
      anyDirty = true;
    }
  }

  if (anyDirty) applyParallelOffsetsAndBundles(wires);
  studioGraphState.staticBatchDirty = true;
}

function updateTempWirePreview() {
  studioGraphState.overlayBatchDirty = true;
  renderStudioCanvas();
}

function drawStudioWires(dirtyNodeIds, dirtyAabbs, changedChapterFramesSet) {
  routeStudioWires(dirtyNodeIds, dirtyAabbs, changedChapterFramesSet);
  renderStudioCanvas();
}
