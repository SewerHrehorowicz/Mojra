"use strict";

const WIRE_GRID = 24;
const WIRE_LANE_STEP = 6;

function snapToWireGrid(v) {
  return Math.round(v / WIRE_GRID) * WIRE_GRID;
}

function simplifyOrthogonalPoints(points) {
  if (!points || points.length === 0) return [];
  const pts = [{ x: points[0].x, y: points[0].y }];
  for (let i = 1; i < points.length; i++) {
    const prev = pts[pts.length - 1];
    const cur = { x: points[i].x, y: points[i].y };
    if (Math.abs(cur.x - prev.x) < 0.5 && Math.abs(cur.y - prev.y) < 0.5) {
      continue;
    }
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

function linecastVertical(x, yA, yB, obstacles, ignore1, ignore2, pad) {
  if (!obstacles || obstacles.length === 0) return null;
  const minY = yA < yB ? yA : yB;
  const maxY = yA > yB ? yA : yB;
  if (maxY - minY < 2) return null;
  const p = pad !== undefined ? pad : 10;

  const grid = obstacles._spatialGrid;
  if (grid && obstacles.length > 12) {
    const gx0 = (x - p) >> 8;
    const gx1 = (x + p) >> 8;
    const gy0 = (minY - p) >> 8;
    const gy1 = (maxY + p) >> 8;
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
            if (b.hidden) continue;
            const isEndpoint =
              b === ignore1 ||
              b === ignore2 ||
              Boolean(b.id && (b.id === ignore1 || b.id === ignore2));
            if (isEndpoint) {
              if (x > b.left - 6 && x < b.right + 6 && maxY > b.top + 3 && minY < b.bottom - 3) {
                return b;
              }
              continue;
            }
            if (x > b.left - p && x < b.right + p && maxY > b.top - p && minY < b.bottom + p) {
              const dist = Math.abs(b.top - yA);
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
    if (b.hidden) continue;
    const isEndpoint =
      b === ignore1 ||
      b === ignore2 ||
      Boolean(b.id && (b.id === ignore1 || b.id === ignore2));
    if (isEndpoint) {
      if (x > b.left - 6 && x < b.right + 6 && maxY > b.top + 3 && minY < b.bottom - 3) {
        return b;
      }
      continue;
    }
    if (x > b.left - p && x < b.right + p && maxY > b.top - p && minY < b.bottom + p) {
      return b;
    }
  }
  return null;
}

function linecastHorizontal(y, xA, xB, obstacles, ignore1, ignore2, pad) {
  if (!obstacles || obstacles.length === 0) return null;
  const minX = xA < xB ? xA : xB;
  const maxX = xA > xB ? xA : xB;
  if (maxX - minX < 2) return null;
  const p = pad !== undefined ? pad : 10;

  const grid = obstacles._spatialGrid;
  if (grid && obstacles.length > 12) {
    const gx0 = (minX - p) >> 8;
    const gx1 = (maxX + p) >> 8;
    const gy0 = (y - p) >> 8;
    const gy1 = (y + p) >> 8;
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
            if (b.hidden) continue;
            const isEndpoint =
              b === ignore1 ||
              b === ignore2 ||
              Boolean(b.id && (b.id === ignore1 || b.id === ignore2));
            if (isEndpoint) {
              if (y > b.top - 7 && y < b.bottom + 7 && maxX > b.left + 3 && minX < b.right - 3) {
                return b;
              }
              continue;
            }
            if (y > b.top - p && y < b.bottom + p && maxX > b.left - p && minX < b.right + p) {
              const dist = Math.abs(b.left - xA);
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
    if (b.hidden) continue;
    const isEndpoint =
      b === ignore1 ||
      b === ignore2 ||
      Boolean(b.id && (b.id === ignore1 || b.id === ignore2));
    if (isEndpoint) {
      if (y > b.top - 7 && y < b.bottom + 7 && maxX > b.left + 3 && minX < b.right - 3) {
        return b;
      }
      continue;
    }
    if (y > b.top - p && y < b.bottom + p && maxX > b.left - p && minX < b.right + p) {
      return b;
    }
  }
  return null;
}

function resolveClearXInDir(x, yA, yB, obstacles, ignoreId1, ignoreId2, dir, p) {
  let curX = x;
  for (let iter = 0; iter < 12; iter++) {
    const hit = linecastVertical(curX, yA, yB, obstacles, ignoreId1, ignoreId2, p - 2);
    if (!hit) return { x: curX, clear: true };
    curX = dir < 0 ? hit.left - p : hit.right + p;
  }
  return {
    x: curX,
    clear: !linecastVertical(curX, yA, yB, obstacles, ignoreId1, ignoreId2, p - 2)
  };
}

function resolveClearX(x, yA, yB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
  const p = pad !== undefined ? pad : 14;
  if (!linecastVertical(x, yA, yB, obstacles, ignoreId1, ignoreId2, p - 2)) {
    return x;
  }
  const dir = preferDir || 0;
  if (dir !== 0) {
    const primary = resolveClearXInDir(x, yA, yB, obstacles, ignoreId1, ignoreId2, dir, p);
    if (primary.clear) return primary.x;
    const fallback = resolveClearXInDir(x, yA, yB, obstacles, ignoreId1, ignoreId2, -dir, p);
    if (fallback.clear) return fallback.x;
    return primary.x;
  }
  const leftRes = resolveClearXInDir(x, yA, yB, obstacles, ignoreId1, ignoreId2, -1, p);
  const rightRes = resolveClearXInDir(x, yA, yB, obstacles, ignoreId1, ignoreId2, 1, p);
  if (leftRes.clear && rightRes.clear) {
    return Math.abs(leftRes.x - x) <= Math.abs(rightRes.x - x) ? leftRes.x : rightRes.x;
  }
  if (leftRes.clear) return leftRes.x;
  if (rightRes.clear) return rightRes.x;
  return Math.abs(leftRes.x - x) <= Math.abs(rightRes.x - x) ? leftRes.x : rightRes.x;
}

function resolveClearYInDir(y, xA, xB, obstacles, ignoreId1, ignoreId2, dir, p) {
  let curY = y;
  for (let iter = 0; iter < 12; iter++) {
    const hit = linecastHorizontal(curY, xA, xB, obstacles, ignoreId1, ignoreId2, p - 2);
    if (!hit) return { y: curY, clear: true };
    curY = dir < 0 ? hit.top - p : hit.bottom + p;
  }
  return {
    y: curY,
    clear: !linecastHorizontal(curY, xA, xB, obstacles, ignoreId1, ignoreId2, p - 2)
  };
}

function resolveClearY(y, xA, xB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
  const p = pad !== undefined ? pad : 14;
  if (!linecastHorizontal(y, xA, xB, obstacles, ignoreId1, ignoreId2, p - 2)) {
    return y;
  }
  const dir = preferDir || 0;
  if (dir !== 0) {
    const primary = resolveClearYInDir(y, xA, xB, obstacles, ignoreId1, ignoreId2, dir, p);
    if (primary.clear) return primary.y;
    const fallback = resolveClearYInDir(y, xA, xB, obstacles, ignoreId1, ignoreId2, -dir, p);
    if (fallback.clear) return fallback.y;
    return primary.y;
  }
  const topRes = resolveClearYInDir(y, xA, xB, obstacles, ignoreId1, ignoreId2, -1, p);
  const botRes = resolveClearYInDir(y, xA, xB, obstacles, ignoreId1, ignoreId2, 1, p);
  if (topRes.clear && botRes.clear) {
    return Math.abs(topRes.y - y) <= Math.abs(botRes.y - y) ? topRes.y : botRes.y;
  }
  if (topRes.clear) return topRes.y;
  if (botRes.clear) return botRes.y;
  return Math.abs(topRes.y - y) <= Math.abs(botRes.y - y) ? topRes.y : botRes.y;
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
        const hit = linecastVertical(x1, y1 + 4, y2 - 4, obs, fromId, toId, 10);
        if (!hit) {
          return [
            { x: x1, y: y1 },
            { x: x2, y: y2 }
          ];
        }
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
        const topY = Math.max(y1 + 12, resolveClearY(hit.top - 16, x1, detourX, obs, fromId, toId, -1, 14));
        const botY = Math.min(y2 - 12, resolveClearY(hit.bottom + 16, detourX, x2, obs, fromId, toId, 1, 14));
        return [
          { x: x1, y: y1 },
          { x: x1, y: topY },
          { x: detourX, y: topY },
          { x: detourX, y: botY },
          { x: x2, y: botY },
          { x: x2, y: y2 }
        ];
      }

      const rawMidY =
        fromBottom !== null && fromBottom !== undefined && fromBottom > y1 && fromBottom < y2 - 24
          ? (fromBottom + y2) * 0.5
          : (y1 + y2) * 0.5;
      const snappedMidY = Math.max(y1 + 16, Math.min(y2 - 16, snapToWireGrid(rawMidY)));
      let midY = resolveClearY(snappedMidY, x1, x2, obs, fromId, toId, 1, 14);
      midY = Math.max(y1 + 12, Math.min(y2 - 12, midY));
      const hitV1 = linecastVertical(x1, y1 + 4, midY, obs, fromId, toId, 10);
      const hitV2 = linecastVertical(x2, midY, y2 - 4, obs, fromId, toId, 10);
      if (!hitV1 && !hitV2) {
        return [
          { x: x1, y: y1 },
          { x: x1, y: midY },
          { x: x2, y: midY },
          { x: x2, y: y2 }
        ];
      }
      const stepOutY = resolveClearY(Math.max(y1 + 18, snapToWireGrid(y1 + 24)), x1, x2, obs, fromId, toId, 1, 14);
      const stepInY = resolveClearY(Math.min(y2 - 18, snapToWireGrid(y2 - 24)), x1, x2, obs, fromId, toId, -1, 14);
      const midChannelX = resolveClearX(snapToWireGrid((x1 + x2) * 0.5), stepOutY, stepInY, obs, fromId, toId, 0, 16);
      return [
        { x: x1, y: y1 },
        { x: x1, y: stepOutY },
        { x: midChannelX, y: stepOutY },
        { x: midChannelX, y: stepInY },
        { x: x2, y: stepInY },
        { x: x2, y: y2 }
      ];
    }

    let channelX = snapToWireGrid((x1 + x2) * 0.5);
    if (toLeft !== undefined && toLeft !== null && toRight !== undefined && toRight !== null) {
      const fromRight = x1 + 96;
      const fromLeft = x1 - 96;
      if (toLeft > fromRight + 20) {
        channelX = snapToWireGrid((fromRight + toLeft) * 0.5);
      } else if (toRight < fromLeft - 20) {
        channelX = snapToWireGrid((fromLeft + toRight) * 0.5);
      } else {
        channelX = snapToWireGrid(Math.max(fromRight, toRight) + 24);
      }
    }
    const baseOutY = Math.max(y1 + 18, snapToWireGrid(y1 + 24));
    const outY = resolveClearY(baseOutY, x1, channelX, obs, fromId, toId, 1, 14);
    const baseInY = Math.min(y2 - 18, snapToWireGrid(y2 - 24));
    const hitInH = linecastHorizontal(baseInY, channelX, x2, obs, fromId, toId, 12);
    const inPreferDir = hitInH && hitInH.bottom + 16 <= y2 - 12 ? 1 : -1;
    const inYBottom = resolveClearY(baseInY, channelX, x2, obs, fromId, toId, inPreferDir, 14);
    if (linecastVertical(channelX, outY, inYBottom, obs, fromId, toId, 14)) {
      channelX = resolveClearX(x1, outY, inYBottom, obs, fromId, toId, 0, 18);
      const snappedChX = snapToWireGrid(channelX);
      if (!linecastVertical(snappedChX, outY, inYBottom, obs, fromId, toId, 14)) {
        channelX = snappedChX;
      }
    }
    return [
      { x: x1, y: y1 },
      { x: x1, y: outY },
      { x: channelX, y: outY },
      { x: channelX, y: inYBottom },
      { x: x2, y: inYBottom },
      { x: x2, y: y2 }
    ];
  }

  // Side choice port: nest exitStub by turn direction so same-direction ports never cross
  const goesDown = fromBottom !== null && fromBottom !== undefined ? y2 > fromBottom + 4 : y2 > y1 + 16;
  const stubRank = goesDown ? Math.max(0, pCount - 1 - idx) : idx;
  const exitStub = WIRE_GRID + stubRank * WIRE_LANE_STEP;
  const entryStub = WIRE_GRID;
  let outX = typeof preferredOutX === "number" && !isNaN(preferredOutX) ? preferredOutX : x1 + exitStub;
  let inY = y2 - entryStub;

  if (fromBottom === null || fromBottom === undefined) {
    if (x2 >= outX && y2 >= y1) {
      return [
        { x: x1, y: y1 },
        { x: x2, y: y1 },
        { x: x2, y: y2 }
      ];
    }
    const midY = y2 >= y1 + 32 ? snapToWireGrid((y1 + y2) * 0.5) : inY;
    return [
      { x: x1, y: y1 },
      { x: outX, y: y1 },
      { x: outX, y: midY },
      { x: x2, y: midY },
      { x: x2, y: y2 }
    ];
  }

  // Target input port is below source node's bottom edge
  if (y2 >= fromBottom + 6) {
    if (
      Math.abs(outX - x2) <= 1.5 &&
      !linecastHorizontal(y1, x1 + 4, x2, obs, fromId, toId, 10) &&
      !linecastVertical(x2, y1, y2 - 4, obs, fromId, toId, 10)
    ) {
      return [
        { x: x1, y: y1 },
        { x: x2, y: y1 },
        { x: x2, y: y2 }
      ];
    }
    const gap = y2 - fromBottom;
    const minClear = Math.min(16, Math.max(8, Math.floor(gap * 0.35)));
    const minMidY = fromBottom + minClear;
    const maxMidY = Math.max(minMidY, y2 - minClear);
    const turnRank = x2 > outX ? pCount - 1 - stubRank : stubRank;
    const baseCorridorY = Math.max(minMidY, Math.min(maxMidY, snapToWireGrid((fromBottom + y2) * 0.5)));
    const laneShift = pCount > 1 && maxMidY - minMidY >= 6 ? Math.round((turnRank - (pCount - 1) * 0.5) * WIRE_LANE_STEP) : 0;
    let midY = Math.max(minMidY, Math.min(maxMidY, baseCorridorY + laneShift));
    outX = resolveClearX(outX, y1, midY, obs, fromId, toId, 1, 14);
    if (
      Math.abs(outX - x2) <= 1.5 &&
      !linecastHorizontal(y1, x1 + 4, x2, obs, fromId, toId, 10) &&
      !linecastVertical(x2, y1, y2 - 4, obs, fromId, toId, 10)
    ) {
      return [
        { x: x1, y: y1 },
        { x: x2, y: y1 },
        { x: x2, y: y2 }
      ];
    }
    midY = resolveClearY(midY, outX, x2, obs, fromId, toId, 1, 12);
    if (x2 < x1 + 4 && midY < fromBottom + 8) {
      midY = Math.max(fromBottom + 10, snapToWireGrid((fromBottom + y2) * 0.5));
    }

    const hitDrop = linecastVertical(x2, midY, y2 - 4, obs, fromId, toId, 10);
    if (!hitDrop) {
      return [
        { x: x1, y: y1 },
        { x: outX, y: y1 },
        { x: outX, y: midY },
        { x: x2, y: midY },
        { x: x2, y: y2 }
      ];
    }
    const detourX = resolveClearX(x2, midY, y2 - 14, obs, fromId, toId, x2 >= outX ? -1 : 1, 16);
    const entryY = resolveClearY(Math.min(y2 - 12, hitDrop.bottom + 14), detourX, x2, obs, fromId, toId, -1, 12);
    return [
      { x: x1, y: y1 },
      { x: outX, y: y1 },
      { x: outX, y: midY },
      { x: detourX, y: midY },
      { x: detourX, y: entryY },
      { x: x2, y: entryY },
      { x: x2, y: y2 }
    ];
  }

  if (toLeft !== undefined && toLeft !== null && toLeft > outX + 10) {
    if (
      inY >= y1 &&
      !linecastHorizontal(y1, x1 + 4, x2, obs, fromId, toId, 10) &&
      !linecastVertical(x2, y1, y2 - 4, obs, fromId, toId, 10)
    ) {
      return [
        { x: x1, y: y1 },
        { x: x2, y: y1 },
        { x: x2, y: y2 }
      ];
    }
    let channelX = Math.max(outX, Math.min(toLeft - 14, (x1 + toLeft) * 0.5 + stubRank * 6));
    inY = resolveClearY(inY, channelX, x2, obs, fromId, toId, -1, 14);
    channelX = resolveClearX(channelX, y1, inY, obs, fromId, toId, 0, 14);
    if (!linecastHorizontal(y1, x1 + 4, channelX, obs, fromId, toId, 10)) {
      return [
        { x: x1, y: y1 },
        { x: channelX, y: y1 },
        { x: channelX, y: inY },
        { x: x2, y: inY },
        { x: x2, y: y2 }
      ];
    }
    const dipY = resolveClearY(fromBottom + 16 + stubRank * 6, outX, channelX, obs, fromId, toId, 1, 14);
    outX = resolveClearX(outX, y1, dipY, obs, fromId, toId, 1, 14);
    channelX = resolveClearX(channelX, dipY, inY, obs, fromId, toId, 1, 14);
    return [
      { x: x1, y: y1 },
      { x: outX, y: y1 },
      { x: outX, y: dipY },
      { x: channelX, y: dipY },
      { x: channelX, y: inY },
      { x: x2, y: inY },
      { x: x2, y: y2 }
    ];
  }

  inY = resolveClearY(inY, outX, x2, obs, fromId, toId, -1, 14);
  let bypassX = Math.max(outX, (toRight !== undefined && toRight !== null ? toRight : x2) + 24 + stubRank * 6);
  bypassX = resolveClearX(bypassX, y1, inY, obs, fromId, toId, 1, 16);
  if (!linecastHorizontal(y1, x1 + 4, bypassX, obs, fromId, toId, 10)) {
    return [
      { x: x1, y: y1 },
      { x: bypassX, y: y1 },
      { x: bypassX, y: inY },
      { x: x2, y: inY },
      { x: x2, y: y2 }
    ];
  }
  const underY = resolveClearY(fromBottom + 16 + stubRank * 6, outX, bypassX, obs, fromId, toId, 1, 14);
  outX = resolveClearX(outX, y1, underY, obs, fromId, toId, 1, 14);
  bypassX = resolveClearX(bypassX, underY, inY, obs, fromId, toId, 1, 16);
  return [
    { x: x1, y: y1 },
    { x: outX, y: y1 },
    { x: outX, y: underY },
    { x: bypassX, y: underY },
    { x: bypassX, y: inY },
    { x: x2, y: inY },
    { x: x2, y: y2 }
  ];
}

function buildPinnedWireWaypoints(w, autoPtsInput) {
  const pins = w.pins;
  let pts =
    autoPtsInput && autoPtsInput.length >= 2
      ? autoPtsInput.map((pt) => ({ x: pt.x, y: pt.y }))
      : [
          { x: w.x1, y: w.y1 },
          { x: w.x2, y: w.y2 }
        ];
  if (!pins || pins.length === 0) {
    return simplifyOrthogonalPoints(pts);
  }

  for (let i = 0; i < pins.length; i++) {
    const p = pins[i];
    if (!p || pts.length < 2) continue;

    let s = -1;
    if (typeof p.segIdx === "number" && p.segIdx >= 0 && p.segIdx < pts.length - 1) {
      const a0 = pts[p.segIdx];
      const b0 = pts[p.segIdx + 1];
      const segAxis = Math.abs(a0.y - b0.y) <= Math.abs(a0.x - b0.x) ? "H" : "V";
      if (!p.axis || p.axis === segAxis) {
        s = p.segIdx;
      }
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
    const isHoriz = Math.abs(a.y - b.y) <= Math.abs(a.x - b.x);

    if (isHoriz) {
      const targetY = p.y;
      if (s > 0 && s < pts.length - 2) {
        pts[s].y = targetY;
        pts[s + 1].y = targetY;
        p.x = Math.round((pts[s].x + pts[s + 1].x) * 0.5);
        p.y = targetY;
        p.axis = "H";
        p.segIdx = s;
      } else if (s === 0 && s === pts.length - 2) {
        const dirX = w.x2 >= w.x1 ? 1 : -1;
        const stubLen = Math.min(24, Math.max(8, Math.abs(w.x2 - w.x1) * 0.25));
        const xA = w.x1 + dirX * stubLen;
        const xB = w.x2 - dirX * stubLen;
        pts = [
          { x: w.x1, y: w.y1 },
          { x: xA, y: w.y1 },
          { x: xA, y: targetY },
          { x: xB, y: targetY },
          { x: xB, y: w.y2 },
          { x: w.x2, y: w.y2 }
        ];
        p.x = Math.round((xA + xB) * 0.5);
        p.y = targetY;
        p.axis = "H";
        p.segIdx = 2;
      } else if (s === 0) {
        if (pts.length === 3) {
          const stubX = w.x1 + (w.x2 >= w.x1 ? 24 : -24);
          pts = [
            { x: w.x1, y: w.y1 },
            { x: stubX, y: w.y1 },
            { x: stubX, y: targetY },
            { x: w.x2, y: targetY },
            { x: w.x2, y: w.y2 }
          ];
          p.x = Math.round((stubX + w.x2) * 0.5);
          p.y = targetY;
          p.axis = "H";
          p.segIdx = 2;
        } else if (pts.length >= 4) {
          pts[2].y = targetY;
          pts[3].y = targetY;
          p.x = Math.round((pts[2].x + pts[3].x) * 0.5);
          p.y = targetY;
          p.axis = "H";
          p.segIdx = 2;
        }
      } else {
        const stubX = w.x2 + (pts[s].x >= w.x2 ? 18 : -18);
        pts[s].y = targetY;
        pts.splice(s + 1, 1, { x: stubX, y: targetY }, { x: stubX, y: w.y2 }, { x: w.x2, y: w.y2 });
        p.x = Math.round((pts[s].x + stubX) * 0.5);
        p.y = targetY;
        p.axis = "H";
        p.segIdx = s;
      }
    } else {
      let targetX = p.x;
      if (!w.isBottomPort && s <= 1) {
        targetX = Math.max(w.x1 + 12, targetX);
      }
      if (s > 0 && s < pts.length - 2) {
        pts[s].x = targetX;
        pts[s + 1].x = targetX;
        p.x = targetX;
        p.y = Math.round((pts[s].y + pts[s + 1].y) * 0.5);
        p.axis = "V";
        p.segIdx = s;
      } else if (s === 0 && s === pts.length - 2) {
        const dirY = w.y2 >= w.y1 ? 1 : -1;
        const stubLen = Math.min(18, Math.max(8, Math.abs(w.y2 - w.y1) * 0.25));
        const yA = w.y1 + dirY * stubLen;
        const yB = w.y2 - dirY * stubLen;
        pts = [
          { x: w.x1, y: w.y1 },
          { x: w.x1, y: yA },
          { x: targetX, y: yA },
          { x: targetX, y: yB },
          { x: w.x2, y: yB },
          { x: w.x2, y: w.y2 }
        ];
        p.x = targetX;
        p.y = Math.round((yA + yB) * 0.5);
        p.axis = "V";
        p.segIdx = 2;
      } else if (s === 0) {
        const nextY = pts[1].y;
        const yA = w.y1 + (nextY >= w.y1 ? 18 : -18);
        pts.splice(
          0,
          2,
          { x: w.x1, y: w.y1 },
          { x: w.x1, y: yA },
          { x: targetX, y: yA },
          { x: targetX, y: nextY }
        );
        p.x = targetX;
        p.y = Math.round((yA + nextY) * 0.5);
        p.axis = "V";
        p.segIdx = 2;
      } else {
        const topY = pts[s].y;
        const yB = w.y2 - (w.y2 >= topY ? 18 : -18);
        pts[s].x = targetX;
        pts.splice(s + 1, 1, { x: targetX, y: yB }, { x: w.x2, y: yB }, { x: w.x2, y: w.y2 });
        p.x = targetX;
        p.y = Math.round((topY + yB) * 0.5);
        p.axis = "V";
        p.segIdx = s;
      }
    }
  }

  return pts;
}

function syncUnreachableNodesMemory() {
  if (!studioGraphState.topologyDirty) return;
  studioGraphState.topologyDirty = false;
  const nodes = studioGraphState.nodes;
  if (!nodes || nodes.length === 0) return;

  let startNode = null;
  for (let i = 0; i < nodes.length; i++) {
    if (nodes[i].isStart) {
      startNode = nodes[i];
      break;
    }
  }
  if (!startNode) startNode = nodes[0];
  const startId = startNode.id;

  const adj = new Map();
  const addEdge = (fromId, toId) => {
    if (!fromId || !toId || toId === "END") return;
    let list = adj.get(fromId);
    if (!list) {
      list = [];
      adj.set(fromId, list);
    }
    list.push(toId);
  };

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        if (pt.targetChapter) {
          const ch = studioGraphState.chapterByKey.get(pt.targetChapter.toLowerCase());
          if (ch && ch.startSceneId) {
            addEdge(nd.id, ch.startSceneId);
          } else if (pt.targetId) {
            addEdge(nd.id, pt.targetId);
          }
        } else if (pt.targetId) {
          addEdge(nd.id, pt.targetId);
        }
      }
    } else if (nd.implicitPort) {
      const ip = nd.implicitPort;
      if (ip.targetChapter) {
        const ch = studioGraphState.chapterByKey.get(ip.targetChapter.toLowerCase());
        if (ch && ch.startSceneId) {
          addEdge(nd.id, ch.startSceneId);
        } else if (ip.targetId) {
          addEdge(nd.id, ip.targetId);
        }
      } else if (ip.targetId) {
        addEdge(nd.id, ip.targetId);
      }
    }
  }

  const reachable = new Set([startId]);
  const queue = [startId];
  let qHead = 0;
  while (qHead < queue.length) {
    const cur = queue[qHead++];
    const nexts = adj.get(cur);
    if (!nexts) continue;
    for (let i = 0; i < nexts.length; i++) {
      const nxt = nexts[i];
      if (!reachable.has(nxt)) {
        reachable.add(nxt);
        queue.push(nxt);
      }
    }
  }

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    nd.unreachable = Boolean(nd.id && !reachable.has(nd.id));
  }
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
  w.minX = minX - 18;
  w.minY = minY - 18;
  w.maxX = maxX + 18;
  w.maxY = maxY + 18;
  if (!w.bbox) {
    w.bbox = { minX: w.minX, minY: w.minY, maxX: w.maxX, maxY: w.maxY };
  } else {
    w.bbox.minX = w.minX;
    w.bbox.minY = w.minY;
    w.bbox.maxX = w.maxX;
    w.bbox.maxY = w.maxY;
  }
}

function computeWireBBox(pts) {
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
  return { minX: minX - 18, minY: minY - 18, maxX: maxX + 18, maxY: maxY + 18 };
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
  const exitStub = 16 + stubRank * 14;
  const baseOutX = typeof preferredOutX === "number" && !isNaN(preferredOutX) ? preferredOutX : x1 + exitStub;

  if (toSide === "right") {
    if (!isBottomPort && Math.abs(y1 - y2) <= 2 && !linecastHorizontal(y2, x1 + 4, x2, obs, fromObj, toObj, 10)) {
      return [
        { x: x1, y: y1 },
        { x: x2, y: y2 }
      ];
    }
    const sx = isBottomPort ? x1 : resolveClearX(baseOutX, y1, y2, obs, fromObj, toObj, 1, 14);
    const sy = isBottomPort ? resolveClearY(y1 + 16, sx, x2, obs, fromObj, toObj, 1, 12) : y1;
    if (!linecastVertical(sx, sy, y2, obs, fromObj, toObj, 10) && !linecastHorizontal(y2, sx, x2, obs, fromObj, toObj, 10)) {
      return [
        { x: x1, y: y1 },
        { x: sx, y: sy },
        { x: sx, y: y2 },
        { x: x2, y: y2 }
      ];
    }
    const clearY = resolveClearY(Math.max(sy, y2), sx, x2 - 12, obs, fromObj, toObj, 1, 12);
    return [
      { x: x1, y: y1 },
      { x: sx, y: sy },
      { x: sx, y: clearY },
      { x: x2 - 16, y: clearY },
      { x: x2 - 16, y: y2 },
      { x: x2, y: y2 }
    ];
  }

  // toSide === "left" (x2 is on the left border of the chapter)
  const sx = isBottomPort ? x1 : resolveClearX(baseOutX, y1, y2, obs, fromObj, toObj, 1, 14);
  const startY = isBottomPort ? y1 + 4 : y1;
  if (
    !linecastVertical(sx, startY, y2, obs, fromObj, toObj, 10) &&
    !linecastHorizontal(y2, x2, sx, obs, fromObj, toObj, 10)
  ) {
    if (isBottomPort) {
      return [
        { x: x1, y: y1 },
        { x: x1, y: y2 },
        { x: x2, y: y2 }
      ];
    }
    return [
      { x: x1, y: y1 },
      { x: sx, y: y1 },
      { x: sx, y: y2 },
      { x: x2, y: y2 }
    ];
  }
  const clearY = resolveClearY(y2, x2 + 12, sx, obs, fromObj, toObj, 1, 12);
  return [
    { x: x1, y: y1 },
    { x: sx, y: y1 },
    { x: sx, y: clearY },
    { x: x2 + 16, y: clearY },
    { x: x2 + 16, y: y2 },
    { x: x2, y: y2 }
  ];
}

function buildSideChapterToChapterWaypoints(x1, y1, x2, y2, fromSide, obs, fromObj, toObj, portIdx) {
  const dir = fromSide === "left" ? -1 : 1;
  const stubOffset = 24 + (portIdx || 0) * 6;
  const approachY = resolveClearY(y2 - 24, x1, x2, obs, fromObj, toObj, -1, 14);
  const corridorX = resolveClearX(x1 + dir * stubOffset, y1, approachY, obs, fromObj, toObj, dir, 16);
  if (
    !linecastVertical(corridorX, y1, approachY, obs, fromObj, toObj, 10) &&
    !linecastHorizontal(approachY, corridorX, x2, obs, fromObj, toObj, 10)
  ) {
    return [
      { x: x1, y: y1 },
      { x: corridorX, y: y1 },
      { x: corridorX, y: approachY },
      { x: x2, y: approachY },
      { x: x2, y: y2 }
    ];
  }
  const midX = resolveClearX((corridorX + x2) * 0.5, y1, approachY, obs, fromObj, toObj, dir, 16);
  return [
    { x: x1, y: y1 },
    { x: corridorX, y: y1 },
    { x: corridorX, y: (y1 + approachY) * 0.5 },
    { x: midX, y: (y1 + approachY) * 0.5 },
    { x: midX, y: approachY },
    { x: x2, y: approachY },
    { x: x2, y: y2 }
  ];
}

function getNodePortTargetCoords(fromNode, pt) {
  if (!fromNode || !pt) return null;
  const targetChapter = (pt.targetChapter || "").trim();
  if (targetChapter) {
    const fromCh = fromNode.chapterObj || studioGraphState.chapterByKey.get(fromNode.chapterLower);
    if (!fromCh || fromCh.hidden) return null;
    const outPort = getChapterOutputPortSlot(fromCh, targetChapter.toLowerCase());
    return {
      x2: outPort.x,
      y2: outPort.y,
      toObj: null,
      outSide: outPort.side || "bottom"
    };
  }
  const targetId = pt.targetId || "";
  if (!targetId || targetId.toUpperCase() === "END") return null;
  const toNode = studioGraphState.nodeById.get(targetId);
  if (!toNode) return null;
  return {
    x2: toNode.x + toNode.inOx,
    y2: toNode.y + toNode.inOy,
    toObj: toNode,
    outSide: "bottom"
  };
}

function computeNodeChoiceExitLanes(fromNode, obs) {
  if (!fromNode || !fromNode.hasChoices || !fromNode.ports || fromNode.ports.length === 0) {
    return null;
  }
  const ports = fromNode.ports;
  const n = ports.length;
  const x1 = fromNode.x + fromNode.w;
  const fromBottom = fromNode.y + fromNode.h;
  const lanes = new Array(n);
  const targets = new Array(n);
  for (let i = 0; i < n; i++) {
    targets[i] = getNodePortTargetCoords(fromNode, ports[i]);
  }

  let prevDownOutX = x1;
  for (let i = n - 1; i >= 0; i--) {
    const pt = ports[i];
    const y1 = fromNode.y + pt.oy;
    const t = targets[i];
    if (!t) {
      const rank = n - 1 - i;
      lanes[i] = x1 + WIRE_GRID + rank * WIRE_LANE_STEP;
      continue;
    }
    const goesDown = t.y2 > fromBottom + 4;
    if (!goesDown) {
      lanes[i] = x1 + WIRE_GRID + i * WIRE_LANE_STEP;
      continue;
    }

    const minGap = prevDownOutX === x1 ? WIRE_GRID : WIRE_LANE_STEP;
    const minLaneX = prevDownOutX + minGap;

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

    const canDirectDrop =
      t.outSide === "bottom" &&
      t.x2 >= minLaneX &&
      !blocksUpperDirectDrop &&
      !linecastHorizontal(y1, x1 + 4, t.x2, obs, fromNode, t.toObj, 10) &&
      !linecastVertical(t.x2, y1, t.y2 - 4, obs, fromNode, t.toObj, 10);

    if (canDirectDrop) {
      lanes[i] = t.x2;
      if (t.x2 <= x1 + 36) {
        prevDownOutX = t.x2;
      }
    } else {
      const checkToY = Math.max(fromBottom + 16, Math.min(t.y2, fromBottom + 32));
      let candLane = resolveClearX(minLaneX, y1, checkToY, obs, fromNode, t.toObj, 1, 14);
      if (candLane < minLaneX) {
        candLane = minLaneX;
      }
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
    const preferredOutX = choiceLanes && choiceLanes[w.portIdx] !== undefined ? choiceLanes[w.portIdx] : undefined;
    w.toChapterOutPort = outPort;
    w.toSide = outSide;
    w.x1 = fromNode.x + w.portRef.ox;
    w.y1 = fromNode.y + w.portRef.oy;
    w.x2 = outPort.x;
    w.y2 = outPort.y;
    w.unreachable = fromNode.unreachable;
    const totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
    let rawPts;
    if (outSide === "left" || outSide === "right") {
      rawPts = buildSideArrivalWaypoints(
        w.x1,
        w.y1,
        w.x2,
        w.y2,
        w.isBottomPort,
        outSide,
        localNodes,
        fromNode,
        null,
        w.portIdx,
        totalPorts,
        preferredOutX
      );
    } else {
      rawPts = buildWireWaypoints(
        w.x1,
        w.y1,
        w.x2,
        w.y2,
        fromNode.y + fromNode.h,
        w.portIdx,
        outPort.x - 8,
        outPort.x + 8,
        w.isBottomPort,
        localNodes,
        fromNode,
        null,
        totalPorts,
        preferredOutX
      );
    }
    const autoPts = simplifyOrthogonalPoints(rawPts);
    w.basePts = w.pins && w.pins.length > 0 ? buildPinnedWireWaypoints(w, autoPts) : autoPts;
  } else if (w.kind === "scene-to-scene") {
    const fromNode = w.fromNode;
    const toNode = w.toNode;
    const chObj = fromNode.chapterObj || toNode.chapterObj;
    const localNodes = chObj && chObj.nodes ? chObj.nodes : nodeObstacles;
    const choiceLanes = !w.isBottomPort ? computeNodeChoiceExitLanes(fromNode, localNodes) : null;
    const preferredOutX = choiceLanes && choiceLanes[w.portIdx] !== undefined ? choiceLanes[w.portIdx] : undefined;
    w.x1 = fromNode.x + w.portRef.ox;
    w.y1 = fromNode.y + w.portRef.oy;
    w.x2 = toNode.x + toNode.inOx;
    w.y2 = toNode.y + toNode.inOy;
    w.unreachable = fromNode.unreachable;
    const totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
    const rawPts = buildWireWaypoints(
      w.x1,
      w.y1,
      w.x2,
      w.y2,
      fromNode.y + fromNode.h,
      w.portIdx,
      toNode.x,
      toNode.x + toNode.w,
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
    w.fromSide = fromSide;
    w.isBottomPort = fromSide === "bottom";
    w.x1 = outPort.x;
    w.y1 = outPort.y;
    w.x2 = toCh.inX;
    w.y2 = toCh.inY;
    w.unreachable = false;
    let rawPts;
    if (fromSide === "left" || fromSide === "right") {
      rawPts = buildSideChapterToChapterWaypoints(
        w.x1,
        w.y1,
        w.x2,
        w.y2,
        fromSide,
        chapterObstacles,
        fromCh,
        toCh,
        w.portIdx
      );
    } else {
      const corridorTop = Math.max(w.y1, w.y2 - 120);
      rawPts = buildWireWaypoints(
        w.x1,
        w.y1,
        w.x2,
        w.y2,
        corridorTop,
        w.portIdx,
        toCh.x,
        toCh.x + toCh.w,
        true,
        chapterObstacles,
        fromCh,
        toCh,
        fromCh.outPorts ? fromCh.outPorts.length : 1
      );
    }
    const autoPts = simplifyOrthogonalPoints(rawPts);
    w.basePts = w.pins && w.pins.length > 0 ? buildPinnedWireWaypoints(w, autoPts) : autoPts;
  } else if (w.kind === "chapter-in-to-entry") {
    const ch = w.toChapterObj;
    const toNode = w.toNode;
    const localNodes = ch && ch.nodes ? ch.nodes : nodeObstacles;
    w.x1 = ch.inX;
    w.y1 = ch.inY;
    w.x2 = toNode.x + toNode.inOx;
    w.y2 = toNode.y + toNode.inOy;
    w.unreachable = toNode.unreachable;
    const rawPts = buildWireWaypoints(
      w.x1,
      w.y1,
      w.x2,
      w.y2,
      ch.y + 28,
      0,
      toNode.x,
      toNode.x + toNode.w,
      true,
      localNodes,
      null,
      toNode,
      1
    );
    const autoPts = simplifyOrthogonalPoints(rawPts);
    w.basePts = w.pins && w.pins.length > 0 ? buildPinnedWireWaypoints(w, autoPts) : autoPts;
  }
  updateWireBounds(w, w.basePts);
}

function buildWireCatalogFromMemory() {
  const nodes = studioGraphState.nodes;
  const chapters = studioGraphState.chapters;
  const wires = [];

  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    ch.inConnected = false;
    for (let j = 0; j < ch.outPorts.length; j++) {
      ch.outPorts[j].connected = Boolean(ch.outPorts[j].targetLower);
    }
  }

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    nd.inConnected = false;
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
    const portList = fromNode.hasChoices && fromNode.ports.length > 0
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
        const wKey = (fromNode.id + ":" + (pt.idx || 0)).toLowerCase();
        const savedPins = studioGraphState.wirePins.get(wKey);
        pt.connected = true;
        wires.push({
          kind: "scene-to-chapter-out",
          wireKey: wKey,
          pins: savedPins ? savedPins.map((p) => ({ x: p.x, y: p.y, axis: p.axis, segIdx: p.segIdx })) : [],
          fromNode: fromNode,
          toNode: null,
          fromChapterObj: fromCh,
          toChapterObj: null,
          chapterScope: fromCh,
          fromChapterOutPort: null,
          toChapterOutPort: null,
          portRef: pt,
          portIdx: pt.idx || 0,
          isBottomPort: Boolean(pt.isBottom),
          targetChapterLower: targetChLower,
          targetGroupKey: "ch_out:" + fromNode.chapterLower + "->" + targetChLower,
          unreachable: fromNode.unreachable,
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
        });
        continue;
      }

      const toNode = studioGraphState.nodeById.get(targetId);
      if (!toNode) continue;
      if (fromNode.chapterLower && toNode.chapterLower && fromNode.chapterLower !== toNode.chapterLower) {
        continue;
      }
      const wKey = (fromNode.id + ":" + (pt.idx || 0)).toLowerCase();
      const savedPins = studioGraphState.wirePins.get(wKey);
      pt.connected = true;
      toNode.inConnected = true;
      wires.push({
        kind: "scene-to-scene",
        wireKey: wKey,
        pins: savedPins ? savedPins.map((p) => ({ x: p.x, y: p.y })) : [],
        fromNode: fromNode,
        toNode: toNode,
        fromChapterObj: null,
        toChapterObj: null,
        chapterScope: fromNode.chapterObj || toNode.chapterObj || null,
        fromChapterOutPort: null,
        toChapterOutPort: null,
        portRef: pt,
        portIdx: pt.idx || 0,
        isBottomPort: Boolean(pt.isBottom),
        targetChapterLower: "",
        targetGroupKey: "scene:" + targetId,
        unreachable: fromNode.unreachable,
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
      });
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
        const wKey = ("ch_in:" + fromCh.nameLower).toLowerCase();
        const savedPins = studioGraphState.wirePins.get(wKey);
        entryNode.inConnected = true;
        wires.push({
          kind: "chapter-in-to-entry",
          wireKey: wKey,
          pins: savedPins ? savedPins.map((p) => ({ x: p.x, y: p.y, axis: p.axis, segIdx: p.segIdx })) : [],
          fromNode: null,
          toNode: entryNode,
          fromChapterObj: fromCh,
          toChapterObj: fromCh,
          chapterScope: fromCh,
          fromChapterOutPort: null,
          toChapterOutPort: null,
          portRef: null,
          portIdx: 0,
          isBottomPort: true,
          targetChapterLower: fromCh.nameLower,
          targetGroupKey: "scene:" + entryNode.id,
          unreachable: entryNode.unreachable,
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
        });
      }
    }
    for (let j = 0; j < fromCh.outPorts.length; j++) {
      const op = fromCh.outPorts[j];
      if (!op.targetLower) continue;
      const toCh = studioGraphState.chapterByKey.get(op.targetLower);
      if (!toCh || toCh.hidden || toCh === fromCh) continue;
      const wKey = ("ch_out:" + fromCh.nameLower + "->" + op.targetLower).toLowerCase();
      const savedPins = studioGraphState.wirePins.get(wKey);
      op.connected = true;
      toCh.inConnected = true;
      wires.push({
        kind: "chapter-to-chapter",
        wireKey: wKey,
        pins: savedPins ? savedPins.map((p) => ({ x: p.x, y: p.y })) : [],
        fromNode: null,
        toNode: null,
        fromChapterObj: fromCh,
        toChapterObj: toCh,
        chapterScope: null,
        fromChapterOutPort: op,
        toChapterOutPort: null,
        portRef: op,
        portIdx: j,
        isBottomPort: (op.side || "bottom") === "bottom",
        targetChapterLower: op.targetLower,
        targetGroupKey: "ch_in:" + op.targetLower,
        unreachable: false,
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
      });
    }
  }

  return wires;
}

function copyBasePtsToPts(w) {
  const src = w.basePts;
  const n = src.length;
  let dst = w.pts;
  if (!dst) {
    dst = [];
    w.pts = dst;
  }
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

function restoreWirePtsWithParallelOffset(w) {
  copyBasePtsToPts(w);
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
          startY: yA,
          endY: yB,
          dirY: yB >= yA ? 1 : -1,
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
      if (visited[j] || sB.wire === sA.wire || sB.wire.chapterScope !== sA.wire.chapterScope) {
        continue;
      }
      const overlapY = (sA.maxY < sB.maxY ? sA.maxY : sB.maxY) - (sA.minY > sB.minY ? sA.minY : sB.minY);
      if (overlapY > 8) {
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
      const isLeftPerimeter = cluster.every((it) => baseX < it.wire.x1 - 24);
      cluster.sort((a, b) => {
        if (isLeftPerimeter) {
          if (Math.abs(a.wire.x1 - b.wire.x1) > 1) return a.wire.x1 - b.wire.x1;
        }
        if (Math.abs(a.wire.y1 - b.wire.y1) > 1) return b.wire.y1 - a.wire.y1;
        return (a.wire.portIdx || 0) - (b.wire.portIdx || 0);
      });
      for (let k = 0; k < cluster.length; k++) {
        const dx = isLeftPerimeter
          ? -(cluster.length - 1 - k) * WIRE_LANE_STEP
          : k * WIRE_LANE_STEP;
        const item = cluster[k];
        const pts = item.wire.pts;
        const nextX = baseX + dx;
        pts[item.segIdx].x = nextX;
        pts[item.segIdx + 1].x = nextX;
      }
    }
  }
}

function classifyHorizontalSegmentRank(pts, segIdx, y) {
  const prevY = pts[segIdx - 1].y;
  const hasNext = segIdx + 2 < pts.length;
  const nextY = hasNext ? pts[segIdx + 2].y : y + 24;
  const goDownStart = prevY > y;
  const goDownEnd = nextY > y;
  if (!goDownStart && goDownEnd) return 0; // Top-to-bottom Z-step (or side-wall exit)
  if (goDownStart && goDownEnd) return 1; // Inverted U-arch into top input
  if (!goDownStart && !goDownEnd) return 2; // U-trough below bottom output
  return 3; // Bottom-to-top Z-step
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
      if (visited[j] || sB.wire === sA.wire || sB.wire.chapterScope !== sA.wire.chapterScope) {
        continue;
      }
      const overlapX = (sA.maxX < sB.maxX ? sA.maxX : sB.maxX) - (sA.minX > sB.minX ? sA.minX : sB.minX);
      if (overlapX > 6) {
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
        if (a.typeRank === 0) {
          const keyA = a.dirX < 0 ? a.startX : -a.startX;
          const keyB = b.dirX < 0 ? b.startX : -b.startX;
          if (Math.abs(keyA - keyB) > 1) return keyA - keyB;
        } else if (a.typeRank === 1) {
          const keyA = a.dirX > 0 ? a.startX : -a.startX;
          const keyB = b.dirX > 0 ? b.startX : -b.startX;
          if (Math.abs(keyA - keyB) > 1) return keyA - keyB;
        } else if (a.typeRank === 2) {
          const keyA = a.dirX < 0 ? -a.endX : a.endX;
          const keyB = b.dirX < 0 ? -b.endX : b.endX;
          if (Math.abs(keyA - keyB) > 1) return keyA - keyB;
        } else {
          const keyA = a.dirX < 0 ? -a.startX : a.startX;
          const keyB = b.dirX < 0 ? -b.startX : b.startX;
          if (Math.abs(keyA - keyB) > 1) return keyA - keyB;
        }
        return a.y - b.y || a.wire.y1 - b.wire.y1;
      });

      let baseClusterY = snapToWireGrid(cluster[0].y);
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
          const isFinalDrop = it.segIdx + 2 === pts.length - 1;
          const clearBot = isFinalDrop && it.wire.toNode ? nextY - 24 : nextY - 12;
          if (clearBot < maxAllowedY) maxAllowedY = clearBot;
        } else if (it.isTerminalSide && it.wire.fromChapterObj) {
          const chBot = it.wire.fromChapterObj.y + it.wire.fromChapterObj.h - 12;
          if (chBot < maxAllowedY) maxAllowedY = chBot;
        }
      }
      const totalSpan = (cluster.length - 1) * WIRE_LANE_STEP;
      let startY = Math.round(baseClusterY - totalSpan * 0.5);
      if (startY + totalSpan > maxAllowedY) {
        startY = maxAllowedY - totalSpan;
      }
      if (startY < minAllowedY) {
        startY = minAllowedY;
      }
      for (let k = 0; k < cluster.length; k++) {
        if (cluster[k].isTerminalSide) {
          startY = cluster[k].y - k * WIRE_LANE_STEP;
          break;
        }
      }

      for (let k = 0; k < cluster.length; k++) {
        const item = cluster[k];
        if (item.isTerminalSide) continue;
        const pts = item.wire.pts;
        const nextY = startY + k * WIRE_LANE_STEP;
        pts[item.segIdx].y = nextY;
        pts[item.segIdx + 1].y = nextY;
      }
    }
  }
}

function assignTargetCollectorBarsAndSubLanes(wires) {
  studioGraphState.bundleOverlays = [];
  studioGraphState.collectorBars = [];

  const byTarget = new Map();
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    const key = w.targetGroupKey || "";
    if (!key) continue;
    let g = byTarget.get(key);
    if (!g) {
      g = [];
      byTarget.set(key, g);
    }
    g.push(w);
  }

  byTarget.forEach((group, groupKey) => {
    if (group.length < 2) return;
    if (group[0].toSide && group[0].toSide !== "bottom") return;

    const tx = group[0].x2;
    const ty = group[0].y2;
    const barY = ty - 12;
    const n = group.length;

    const meta = group.map((w) => {
      const pts = w.pts;
      const len = pts.length;
      const fromX = len >= 3 ? pts[len - 3].x : w.x1;
      const turnY = len >= 3 ? pts[len - 2].y : w.y1;
      let side = 0;
      if (len >= 3 && fromX < tx - 2) side = -1;
      else if (len >= 3 && fromX > tx + 2) side = 1;
      return { wire: w, side, fromX, turnY };
    });

    meta.sort((a, b) => {
      if (a.side !== b.side) return a.side - b.side;
      if (a.side === -1) {
        // Left-approaching: lowest horizontal track (largest turnY) gets leftmost slot -> 0 corner crossings
        if (Math.abs(a.turnY - b.turnY) > 1) return b.turnY - a.turnY;
        return b.fromX - a.fromX;
      }
      if (a.side === 1) {
        // Right-approaching: highest horizontal track (smallest turnY) gets leftmost of right slots -> 0 corner crossings
        if (Math.abs(a.turnY - b.turnY) > 1) return a.turnY - b.turnY;
        return b.fromX - a.fromX;
      }
      if (Math.abs(a.wire.x1 - b.wire.x1) > 1) return a.wire.x1 - b.wire.x1;
      return (a.wire.portIdx || 0) - (b.wire.portIdx || 0);
    });

    let minHorizontalTurnY = barY - 12;
    for (let i = 0; i < meta.length; i++) {
      if (meta[i].side !== 0 && meta[i].turnY < minHorizontalTurnY) {
        minHorizontalTurnY = meta[i].turnY;
      }
    }

    for (let k = 0; k < n; k++) {
      const item = meta[k];
      const w = item.wire;
      const dx = Math.round((k - (n - 1) * 0.5) * WIRE_LANE_STEP);
      const slotX = tx + dx;
      w._parallelDx = dx;
      const pts = w.pts;

      if (pts.length >= 3) {
        const oldDropX = pts[pts.length - 2].x;
        pts[pts.length - 2].x = slotX;
        pts[pts.length - 1] = { x: slotX, y: barY };
        // If this was a 3-point L-wire from a choice port, keep sibling choice exit lanes >= 6px clear
        if (pts.length === 3 && w.fromNode && slotX > oldDropX) {
          for (let s = 0; s < wires.length; s++) {
            const sw = wires[s];
            if (sw !== w && sw.fromNode === w.fromNode && sw.pts && sw.pts.length >= 4) {
              if (Math.abs(sw.pts[1].x - sw.pts[2].x) < 0.5 && sw.pts[1].x > oldDropX - 1 && sw.pts[1].x < slotX + WIRE_LANE_STEP) {
                const shiftedX = slotX + WIRE_LANE_STEP;
                sw.pts[1].x = shiftedX;
                sw.pts[2].x = shiftedX;
              }
            }
          }
        }
      } else if (pts.length === 2) {
        if (Math.abs(dx) < 0.5) {
          pts[1] = { x: slotX, y: barY };
        } else {
          const stepY = Math.max(w.y1 + 12, minHorizontalTurnY - WIRE_LANE_STEP);
          w.pts = [
            { x: w.x1, y: w.y1 },
            { x: w.x1, y: stepY },
            { x: slotX, y: stepY },
            { x: slotX, y: barY }
          ];
        }
      }
    }

    const minDx = Math.round((-(n - 1) * 0.5) * WIRE_LANE_STEP);
    const maxDx = Math.round(((n - 1) * 0.5) * WIRE_LANE_STEP);
    studioGraphState.collectorBars.push({
      targetGroupKey: groupKey,
      x: tx,
      portY: ty,
      barY: barY,
      minX: tx + minDx,
      maxX: tx + maxDx,
      count: n,
      unreachable: group.every((it) => it.unreachable)
    });
  });
}

function hashWireColorSeed(str) {
  let h = 2166136261 >>> 0;
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i);
    h = Math.imul(h, 16777619) >>> 0;
  }
  return h >>> 0;
}

function computeWireAndPortColors(wires) {
  const PALETTE_SIZE = 8;
  const nodes = studioGraphState.nodes || [];
  const chapters = studioGraphState.chapters || [];
  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    nd.inColorIdx = -1;
    if (nd.ports) {
      for (let j = 0; j < nd.ports.length; j++) {
        nd.ports[j].colorIdx = -1;
      }
    }
    if (nd.implicitPort) {
      nd.implicitPort.colorIdx = -1;
    }
  }
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    ch.inColorIdx = -1;
    if (ch.outPorts) {
      for (let j = 0; j < ch.outPorts.length; j++) {
        ch.outPorts[j].colorIdx = -1;
      }
    }
  }

  const byTarget = new Map();
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    w.colorIdx = -1;
    const key = w.targetGroupKey || "";
    if (!key) continue;
    let g = byTarget.get(key);
    if (!g) {
      g = [];
      byTarget.set(key, g);
    }
    g.push(w);
  }

  const usedAtSource = new Map();
  const usedAtTarget = new Map();

  // Pass A: Color scene-to-scene and scene-to-chapter-out wires first
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.kind === "chapter-to-chapter" || w.kind === "chapter-in-to-entry") {
      continue;
    }
    const fromMultiNode = Boolean(w.fromNode && w.fromNode.hasChoices && w.fromNode.ports && w.fromNode.ports.length >= 2);
    const targetGroup = w.targetGroupKey ? byTarget.get(w.targetGroupKey) : null;
    const toMultiTarget = Boolean(targetGroup && targetGroup.length >= 2);
    const destChGroup = w.kind === "scene-to-chapter-out" && w.targetChapterLower ? byTarget.get("ch_in:" + w.targetChapterLower) : null;
    const toMultiDestCh = Boolean(destChGroup && destChGroup.length >= 2);

    if (!fromMultiNode && !toMultiTarget && !toMultiDestCh) {
      continue;
    }

    const srcKey = w.fromNode ? "node:" + w.fromNode.id : "wire:" + i;
    const tgtKey = w.targetGroupKey || "tgt:" + i;
    const destChKey = w.kind === "scene-to-chapter-out" && w.targetChapterLower ? "ch_in:" + w.targetChapterLower : "";

    let srcSet = usedAtSource.get(srcKey);
    if (!srcSet) {
      srcSet = new Set();
      usedAtSource.set(srcKey, srcSet);
    }
    let tgtSet = usedAtTarget.get(tgtKey);
    if (!tgtSet) {
      tgtSet = new Set();
      usedAtTarget.set(tgtKey, tgtSet);
    }
    let destChSet = null;
    if (destChKey) {
      destChSet = usedAtTarget.get(destChKey);
      if (!destChSet) {
        destChSet = new Set();
        usedAtTarget.set(destChKey, destChSet);
      }
    }

    let parentInColor = -1;
    if (w.fromNode) {
      const parentInGroup = byTarget.get("scene:" + w.fromNode.id);
      if (parentInGroup && parentInGroup.length === 1 && parentInGroup[0].colorIdx >= 0) {
        parentInColor = parentInGroup[0].colorIdx;
      }
    }

    const seed = (hashWireColorSeed(srcKey) + (w.portIdx || 0) * 3) % PALETTE_SIZE;
    let chosen = -1;

    // Pass 1: distinct at target, destination chapter, source, and parent's incoming wire
    for (let step = 0; step < PALETTE_SIZE; step++) {
      const cand = (seed + step) % PALETTE_SIZE;
      if (!tgtSet.has(cand) && (!destChSet || !destChSet.has(cand)) && !srcSet.has(cand) && cand !== parentInColor) {
        chosen = cand;
        break;
      }
    }
    // Pass 2: distinct at target, destination chapter, and source
    if (chosen < 0) {
      for (let step = 0; step < PALETTE_SIZE; step++) {
        const cand = (seed + step) % PALETTE_SIZE;
        if (!tgtSet.has(cand) && (!destChSet || !destChSet.has(cand)) && !srcSet.has(cand)) {
          chosen = cand;
          break;
        }
      }
    }
    // Pass 3: distinct at target and destination chapter
    if (chosen < 0) {
      for (let step = 0; step < PALETTE_SIZE; step++) {
        const cand = (seed + step) % PALETTE_SIZE;
        if (!tgtSet.has(cand) && (!destChSet || !destChSet.has(cand))) {
          chosen = cand;
          break;
        }
      }
    }
    if (chosen < 0) {
      chosen = seed % PALETTE_SIZE;
    }

    w.colorIdx = chosen;
    srcSet.add(chosen);
    tgtSet.add(chosen);
    if (destChSet && (!targetGroup || targetGroup.length === 1)) {
      destChSet.add(chosen);
    }
  }

  // Propagate scene-to-chapter-out colors to chapter output ports
  byTarget.forEach((group) => {
    if (group.length === 1) {
      const w = group[0];
      if (w.toNode) {
        w.toNode.inColorIdx = w.colorIdx;
      } else if (w.toChapterOutPort) {
        w.toChapterOutPort.inColorIdx = w.colorIdx;
        w.toChapterOutPort.colorIdx = w.colorIdx;
      }
    } else {
      for (let k = 0; k < group.length; k++) {
        const w = group[k];
        if (w.toNode) w.toNode.inColorIdx = -1;
        if (w.toChapterOutPort) {
          w.toChapterOutPort.inColorIdx = -1;
          w.toChapterOutPort.colorIdx = -1;
        }
      }
    }
  });

  // Pass B: Chapter-to-chapter wires inherit single incoming scene-to-chapter-out wire color, or resolve conflict-free
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.kind !== "chapter-to-chapter") continue;
    const op = w.fromChapterOutPort;
    const tgtKey = w.targetGroupKey || "tgt:" + i;
    let tgtSet = usedAtTarget.get(tgtKey);
    if (!tgtSet) {
      tgtSet = new Set();
      usedAtTarget.set(tgtKey, tgtSet);
    }

    if (op && op.inColorIdx >= 0) {
      w.colorIdx = op.inColorIdx;
      op.colorIdx = w.colorIdx;
      tgtSet.add(w.colorIdx);
      continue;
    }

    const fromMultiCh = Boolean(w.fromChapterObj && w.fromChapterObj.outPorts && w.fromChapterObj.outPorts.length >= 2);
    const targetGroup = w.targetGroupKey ? byTarget.get(w.targetGroupKey) : null;
    const toMultiTarget = Boolean(targetGroup && targetGroup.length >= 2);
    if (!fromMultiCh && !toMultiTarget) {
      continue;
    }

    const srcKey = w.fromChapterObj ? "ch:" + w.fromChapterObj.nameLower : "wire:" + i;
    let srcSet = usedAtSource.get(srcKey);
    if (!srcSet) {
      srcSet = new Set();
      usedAtSource.set(srcKey, srcSet);
    }
    const seed = (hashWireColorSeed(srcKey) + (w.portIdx || 0) * 3) % PALETTE_SIZE;
    let chosen = -1;
    for (let step = 0; step < PALETTE_SIZE; step++) {
      const cand = (seed + step) % PALETTE_SIZE;
      if (!tgtSet.has(cand) && !srcSet.has(cand)) {
        chosen = cand;
        break;
      }
    }
    if (chosen < 0) {
      for (let step = 0; step < PALETTE_SIZE; step++) {
        const cand = (seed + step) % PALETTE_SIZE;
        if (!tgtSet.has(cand)) {
          chosen = cand;
          break;
        }
      }
    }
    if (chosen < 0) chosen = seed % PALETTE_SIZE;
    w.colorIdx = chosen;
    if (op) op.colorIdx = chosen;
    srcSet.add(chosen);
    tgtSet.add(chosen);
  }

  // Propagate chapter-to-chapter colors to destination chapter input socket and chapter-in-to-entry wire
  byTarget.forEach((group) => {
    if (group.length === 1 && group[0].kind === "chapter-to-chapter") {
      const w = group[0];
      if (w.toChapterObj) {
        w.toChapterObj.inColorIdx = w.colorIdx;
      }
    } else if (group.length >= 2 && group[0].kind === "chapter-to-chapter") {
      for (let k = 0; k < group.length; k++) {
        if (group[k].toChapterObj) group[k].toChapterObj.inColorIdx = -1;
      }
    }
  });

  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.kind === "chapter-in-to-entry" && w.fromChapterObj) {
      w.colorIdx = w.fromChapterObj.inColorIdx !== undefined ? w.fromChapterObj.inColorIdx : -1;
      const entryGroup = w.targetGroupKey ? byTarget.get(w.targetGroupKey) : null;
      if (entryGroup && entryGroup.length === 1 && w.toNode) {
        w.toNode.inColorIdx = w.colorIdx;
      }
    }
  }

  // Propagate wire colors to source output ports (●)
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.portRef) {
      w.portRef.colorIdx = w.colorIdx;
    }
  }
}

function applyParallelOffsetsAndBundles(wires, dirtyGroupKeys, localRerouteBbox) {
  for (let i = 0; i < wires.length; i++) {
    wires[i]._parallelDx = 0;
    copyBasePtsToPts(wires[i]);
  }
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
        const yA = p0.y;
        const yB = p1.y;
        vSegs.push({
          wire: w,
          x: p0.x,
          minY: yA < yB ? yA : yB,
          maxY: yA > yB ? yA : yB
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
      const minX = (p0.x < p1.x ? p0.x : p1.x) + 3.5;
      const maxX = (p0.x > p1.x ? p0.x : p1.x) - 3.5;
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
        if (vs.wire === w) continue;
        if (y > vs.minY + 2.5 && y < vs.maxY - 2.5) {
          if (vs.x - lastBridgeX >= 1.5) {
            if (!deduped) deduped = [];
            deduped.push(vs.x);
            lastBridgeX = vs.x;
          }
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
    syncUnreachableNodesMemory();
    const wires = buildWireCatalogFromMemory();
    for (let i = 0; i < wires.length; i++) {
      routeSingleWireBasePts(wires[i], nodeObstacles, chapterObstacles);
    }
    studioGraphState.wires = wires;
    applyParallelOffsetsAndBundles(wires, null, null);
    studioGraphState.staticBatchDirty = true;
    return;
  }

  const wires = studioGraphState.wires;
  const dirtyGroupKeys = new Set();
  const localRerouteBbox = {
    minX: Infinity,
    minY: Infinity,
    maxX: -Infinity,
    maxY: -Infinity
  };
  if (dirtyAabbs) {
    for (let k = 0; k < dirtyAabbs.length; k++) {
      const a = dirtyAabbs[k];
      if (a.minX < localRerouteBbox.minX) localRerouteBbox.minX = a.minX;
      if (a.minY < localRerouteBbox.minY) localRerouteBbox.minY = a.minY;
      if (a.maxX > localRerouteBbox.maxX) localRerouteBbox.maxX = a.maxX;
      if (a.maxY > localRerouteBbox.maxY) localRerouteBbox.maxY = a.maxY;
    }
  }

  const hasChangedChapters = Boolean(changedChapterFramesSet && changedChapterFramesSet.size > 0);

  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    let isDirty = false;
    if (w.fromNode && dirtyNodeIds.has(w.fromNode.id)) {
      isDirty = true;
    } else if (w.toNode && dirtyNodeIds.has(w.toNode.id)) {
      isDirty = true;
    } else if (
      hasChangedChapters &&
      ((w.fromChapterObj && changedChapterFramesSet.has(w.fromChapterObj)) ||
        (w.toChapterObj && changedChapterFramesSet.has(w.toChapterObj)))
    ) {
      isDirty = true;
    } else if (dirtyAabbs && w.bbox) {
      for (let k = 0; k < dirtyAabbs.length; k++) {
        if (aabbIntersects(w.bbox, dirtyAabbs[k])) {
          isDirty = true;
          break;
        }
      }
    }
    if (isDirty) {
      if (w.minX < localRerouteBbox.minX) localRerouteBbox.minX = w.minX;
      if (w.minY < localRerouteBbox.minY) localRerouteBbox.minY = w.minY;
      if (w.maxX > localRerouteBbox.maxX) localRerouteBbox.maxX = w.maxX;
      if (w.maxY > localRerouteBbox.maxY) localRerouteBbox.maxY = w.maxY;
      routeSingleWireBasePts(w, nodeObstacles, chapterObstacles);
      if (w.minX < localRerouteBbox.minX) localRerouteBbox.minX = w.minX;
      if (w.minY < localRerouteBbox.minY) localRerouteBbox.minY = w.minY;
      if (w.maxX > localRerouteBbox.maxX) localRerouteBbox.maxX = w.maxX;
      if (w.maxY > localRerouteBbox.maxY) localRerouteBbox.maxY = w.maxY;
      dirtyGroupKeys.add(w.targetGroupKey);
    }
  }

  if (dirtyGroupKeys.size > 0) {
    applyParallelOffsetsAndBundles(wires, dirtyGroupKeys, localRerouteBbox);
  }
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
