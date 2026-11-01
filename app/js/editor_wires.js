"use strict";

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

function resolveClearX(x, yA, yB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
  const p = pad !== undefined ? pad : 14;
  let curX = x;
  let dir = preferDir || 0;
  for (let iter = 0; iter < 4; iter++) {
    const hit = linecastVertical(curX, yA, yB, obstacles, ignoreId1, ignoreId2, p - 2);
    if (!hit) return curX;
    const leftCand = hit.left - p;
    const rightCand = hit.right + p;
    if (dir < 0) {
      curX = leftCand;
    } else if (dir > 0) {
      curX = rightCand;
    } else if (Math.abs(curX - leftCand) <= Math.abs(rightCand - curX)) {
      curX = leftCand;
      dir = -1;
    } else {
      curX = rightCand;
      dir = 1;
    }
  }
  return curX;
}

function resolveClearY(y, xA, xB, obstacles, ignoreId1, ignoreId2, preferDir, pad) {
  const p = pad !== undefined ? pad : 14;
  let curY = y;
  let dir = preferDir || 0;
  for (let iter = 0; iter < 4; iter++) {
    const hit = linecastHorizontal(curY, xA, xB, obstacles, ignoreId1, ignoreId2, p - 2);
    if (!hit) return curY;
    const topCand = hit.top - p;
    const botCand = hit.bottom + p;
    if (dir < 0) {
      curY = topCand;
    } else if (dir > 0) {
      curY = botCand;
    } else if (Math.abs(curY - topCand) <= Math.abs(botCand - curY)) {
      curY = topCand;
      dir = -1;
    } else {
      curY = botCand;
      dir = 1;
    }
  }
  return curY;
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

      const baseMidY =
        fromBottom !== null && fromBottom !== undefined && fromBottom > y1 && fromBottom < y2 - 24
          ? (fromBottom + y2) * 0.5
          : (y1 + y2) * 0.5;
      let midY = resolveClearY(baseMidY, x1, x2, obs, fromId, toId, 1, 14);
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
      const stepOutY = resolveClearY(y1 + 16, x1, x2, obs, fromId, toId, 1, 14);
      const stepInY = resolveClearY(y2 - 16, x1, x2, obs, fromId, toId, -1, 14);
      const midChannelX = resolveClearX((x1 + x2) * 0.5, stepOutY, stepInY, obs, fromId, toId, 0, 16);
      return [
        { x: x1, y: y1 },
        { x: x1, y: stepOutY },
        { x: midChannelX, y: stepOutY },
        { x: midChannelX, y: stepInY },
        { x: x2, y: stepInY },
        { x: x2, y: y2 }
      ];
    }

    let channelX = (x1 + x2) * 0.5;
    if (toLeft !== undefined && toLeft !== null && toRight !== undefined && toRight !== null) {
      const fromRight = x1 + 96;
      const fromLeft = x1 - 96;
      if (toLeft > fromRight + 20) {
        channelX = (fromRight + toLeft) * 0.5;
      } else if (toRight < fromLeft - 20) {
        channelX = (fromLeft + toRight) * 0.5;
      } else {
        channelX = Math.max(fromRight, toRight) + 28;
      }
    }
    const outY = resolveClearY(y1 + 24, x1, channelX, obs, fromId, toId, 1, 14);
    const inYBottom = resolveClearY(y2 - 24, channelX, x2, obs, fromId, toId, -1, 14);
    channelX = resolveClearX(channelX, outY, inYBottom, obs, fromId, toId, 0, 16);
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
  const exitStub = 16 + stubRank * 14;
  const entryStub = 20;
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
    const midY = y2 >= y1 + 32 ? (y1 + y2) * 0.5 : inY;
    return [
      { x: x1, y: y1 },
      { x: outX, y: y1 },
      { x: outX, y: midY },
      { x: x2, y: midY },
      { x: x2, y: y2 }
    ];
  }

  // Target input port is below source node's bottom edge (including tight vertical gaps like 18px)
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
    const minClear = Math.min(14, Math.max(7, Math.floor(gap * 0.42)));
    const minMidY = fromBottom + minClear;
    const maxMidY = Math.max(minMidY, y2 - minClear);
    const laneShift = pCount > 1 && maxMidY - minMidY >= 8 ? (stubRank - (pCount - 1) * 0.5) * 8 : 0;
    let midY = Math.max(minMidY, Math.min(maxMidY, (fromBottom + y2) * 0.5 + laneShift));
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
    if (x2 < x1 + 4 && midY < fromBottom + 7) {
      midY = Math.max(fromBottom + 8, (fromBottom + y2) * 0.5);
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

function buildPinnedWireWaypoints(w) {
  const pins = w.pins;
  const pts = [{ x: w.x1, y: w.y1 }];
  let curX = w.x1;
  let curY = w.y1;
  let lastDir = w.isBottomPort ? "V" : "H";

  if (!w.isBottomPort && pins.length > 0 && pins[0].x < w.x1 + 12) {
    const outX = w.x1 + 20;
    pts.push({ x: outX, y: curY });
    curX = outX;
    lastDir = "H";
  } else if (w.isBottomPort && pins.length > 0 && pins[0].y < w.y1 + 10) {
    const outY = w.y1 + 16;
    pts.push({ x: curX, y: outY });
    curY = outY;
    lastDir = "V";
  }

  for (let i = 0; i < pins.length; i++) {
    const p = pins[i];
    const dx = Math.abs(p.x - curX);
    const dy = Math.abs(p.y - curY);
    if (dx < 1 && dy < 1) continue;
    if (dx < 1) {
      pts.push({ x: p.x, y: p.y });
      curX = p.x;
      curY = p.y;
      lastDir = "V";
    } else if (dy < 1) {
      pts.push({ x: p.x, y: p.y });
      curX = p.x;
      curY = p.y;
      lastDir = "H";
    } else if (lastDir === "H") {
      pts.push({ x: p.x, y: curY });
      pts.push({ x: p.x, y: p.y });
      curX = p.x;
      curY = p.y;
      lastDir = "V";
    } else {
      pts.push({ x: curX, y: p.y });
      pts.push({ x: p.x, y: p.y });
      curX = p.x;
      curY = p.y;
      lastDir = "H";
    }
  }

  if (Math.abs(w.x2 - curX) < 1) {
    pts.push({ x: w.x2, y: w.y2 });
  } else if (curY <= w.y2 - 8) {
    pts.push({ x: w.x2, y: curY });
    pts.push({ x: w.x2, y: w.y2 });
  } else {
    const entryY = w.y2 - 16;
    pts.push({ x: curX, y: entryY });
    pts.push({ x: w.x2, y: entryY });
    pts.push({ x: w.x2, y: w.y2 });
  }

  return simplifyOrthogonalPoints(pts);
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
  let prevWasLeftTurn = false;
  for (let i = n - 1; i >= 0; i--) {
    const pt = ports[i];
    const y1 = fromNode.y + pt.oy;
    const t = targets[i];
    if (!t) {
      const rank = n - 1 - i;
      lanes[i] = x1 + 16 + rank * 14;
      continue;
    }
    const goesDown = t.y2 > fromBottom + 4;
    if (!goesDown) {
      lanes[i] = x1 + 16 + i * 14;
      continue;
    }

    const minGap = prevDownOutX === x1 ? (t.x2 < x1 ? 12 : 16) : prevWasLeftTurn ? 12 : 14;
    const minLaneX = prevDownOutX + minGap;

    let blocksUpperDirectDrop = false;
    if (t.x2 > minLaneX + 14) {
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
      prevWasLeftTurn = false;
    } else {
      const checkToY = Math.max(fromBottom + 16, Math.min(t.y2, fromBottom + 32));
      let candLane = resolveClearX(minLaneX, y1, checkToY, obs, fromNode, t.toObj, 1, 14);
      if (candLane < minLaneX) {
        candLane = minLaneX;
      }
      lanes[i] = candLane;
      prevWasLeftTurn = t.x2 < x1;
    }
    prevDownOutX = lanes[i];
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
    if (w.pins && w.pins.length > 0) {
      w.basePts = buildPinnedWireWaypoints(w);
    } else if (outSide === "left" || outSide === "right") {
      const totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
      const rawPts = buildSideArrivalWaypoints(
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
      w.basePts = simplifyOrthogonalPoints(rawPts);
    } else {
      const totalPorts = fromNode.hasChoices && fromNode.ports ? fromNode.ports.length : 1;
      const rawPts = buildWireWaypoints(
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
      w.basePts = simplifyOrthogonalPoints(rawPts);
    }
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
    if (w.pins && w.pins.length > 0) {
      w.basePts = buildPinnedWireWaypoints(w);
    } else {
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
      w.basePts = simplifyOrthogonalPoints(rawPts);
    }
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
    if (w.pins && w.pins.length > 0) {
      w.basePts = buildPinnedWireWaypoints(w);
    } else if (fromSide === "left" || fromSide === "right") {
      const rawPts = buildSideChapterToChapterWaypoints(
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
      w.basePts = simplifyOrthogonalPoints(rawPts);
    } else {
      const corridorTop = Math.max(w.y1, w.y2 - 120);
      const rawPts = buildWireWaypoints(
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
      w.basePts = simplifyOrthogonalPoints(rawPts);
    }
  } else if (w.kind === "chapter-in-to-entry") {
    const ch = w.toChapterObj;
    const toNode = w.toNode;
    const localNodes = ch && ch.nodes ? ch.nodes : nodeObstacles;
    w.x1 = ch.inX;
    w.y1 = ch.inY;
    w.x2 = toNode.x + toNode.inOx;
    w.y2 = toNode.y + toNode.inOy;
    w.unreachable = toNode.unreachable;
    if (w.pins && w.pins.length > 0) {
      w.basePts = buildPinnedWireWaypoints(w);
    } else {
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
      w.basePts = simplifyOrthogonalPoints(rawPts);
    }
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
          pins: savedPins ? savedPins.map((p) => ({ x: p.x, y: p.y })) : [],
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
          pins: savedPins ? savedPins.map((p) => ({ x: p.x, y: p.y })) : [],
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
  const dx = w._parallelDx || 0;
  const n = w.pts.length;
  if (Math.abs(dx) >= 0.1 && n >= 3 && !(w.toSide && w.toSide !== "bottom")) {
    w.pts[n - 2].x = w.x2 + dx;
    w.pts[n - 1] = { x: w.x2 + dx, y: w.y2 - 3 };
    w.pts.push({ x: w.x2, y: w.y2 });
  }
}

function applyParallelOffsetsAndBundles(wires, dirtyGroupKeys, localRerouteBbox) {
  const byTarget = new Map();
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (dirtyGroupKeys && !dirtyGroupKeys.has(w.targetGroupKey)) continue;
    let g = byTarget.get(w.targetGroupKey);
    if (!g) {
      g = [];
      byTarget.set(w.targetGroupKey, g);
    }
    g.push(w);
  }

  if (!dirtyGroupKeys) {
    studioGraphState.bundleOverlays = [];
  } else {
    studioGraphState.bundleOverlays = studioGraphState.bundleOverlays.filter(
      (b) => !dirtyGroupKeys.has(b.targetGroupKey)
    );
  }

  byTarget.forEach((group, groupKey) => {
    for (let i = 0; i < group.length; i++) {
      group[i]._parallelDx = 0;
      copyBasePtsToPts(group[i]);
    }
    if (group.length < 2) {
      if (group.length === 1) {
        updateWireBounds(group[0], group[0].pts);
        if (localRerouteBbox) {
          if (group[0].minX < localRerouteBbox.minX) localRerouteBbox.minX = group[0].minX;
          if (group[0].minY < localRerouteBbox.minY) localRerouteBbox.minY = group[0].minY;
          if (group[0].maxX > localRerouteBbox.maxX) localRerouteBbox.maxX = group[0].maxX;
          if (group[0].maxY > localRerouteBbox.maxY) localRerouteBbox.maxY = group[0].maxY;
        }
      }
      return;
    }

    const getApproachX = (it) => (it.pts.length >= 3 ? it.pts[it.pts.length - 3].x : it.x1);
    group.sort((a, b) => {
      const ax = getApproachX(a);
      const bx = getApproachX(b);
      if (Math.abs(ax - bx) > 1) return ax - bx;
      if (Math.abs(a.y1 - b.y1) > 1) return a.y1 - b.y1;
      return (a.portIdx || 0) - (b.portIdx || 0);
    });

    if (group.length <= 3) {
      for (let idx = 0; idx < group.length; idx++) {
        const it = group[idx];
        it._parallelDx = 0;
        updateWireBounds(it, it.pts);
        if (localRerouteBbox) {
          if (it.minX < localRerouteBbox.minX) localRerouteBbox.minX = it.minX;
          if (it.minY < localRerouteBbox.minY) localRerouteBbox.minY = it.minY;
          if (it.maxX > localRerouteBbox.maxX) localRerouteBbox.maxX = it.maxX;
          if (it.maxY > localRerouteBbox.maxY) localRerouteBbox.maxY = it.maxY;
        }
      }
    } else {
      const tx = group[0].x2;
      const ty2 = group[0].y2;
      let ty1 = ty2 - 36;
      for (let idx = 0; idx < group.length; idx++) {
        const it = group[idx];
        it._parallelDx = 0;
        const n = it.pts.length;
        if (n >= 2) {
          const candY = it.pts[n - 2].y;
          if (candY < ty2 - 16 && candY > ty1) {
            ty1 = candY;
          }
        }
        updateWireBounds(it, it.pts);
        if (localRerouteBbox) {
          if (it.minX < localRerouteBbox.minX) localRerouteBbox.minX = it.minX;
          if (it.minY < localRerouteBbox.minY) localRerouteBbox.minY = it.minY;
          if (it.maxX > localRerouteBbox.maxX) localRerouteBbox.maxX = it.maxX;
          if (it.maxY > localRerouteBbox.maxY) localRerouteBbox.maxY = it.maxY;
        }
      }
      if (ty2 - ty1 < 24) {
        ty1 = ty2 - 28;
      }
      studioGraphState.bundleOverlays.push({
        targetGroupKey: groupKey,
        x1: tx,
        y1: ty1,
        x2: tx,
        y2: ty2,
        count: group.length
      });
    }
  });

  if (localRerouteBbox) {
    const padMinX = localRerouteBbox.minX - 16;
    const padMinY = localRerouteBbox.minY - 16;
    const padMaxX = localRerouteBbox.maxX + 16;
    const padMaxY = localRerouteBbox.maxY + 16;
    const scopedWires = [];
    for (let i = 0; i < wires.length; i++) {
      const w = wires[i];
      if (w.maxX >= padMinX && w.minX <= padMaxX && w.maxY >= padMinY && w.minY <= padMaxY) {
        if (dirtyGroupKeys && !dirtyGroupKeys.has(w.targetGroupKey)) {
          restoreWirePtsWithParallelOffset(w);
        }
        scopedWires.push(w);
      }
    }
    separateOverlappingWireSegments(scopedWires);
    computeWireCrossingBridges(scopedWires);
  } else {
    separateOverlappingWireSegments(wires);
    computeWireCrossingBridges(wires);
  }
}

function separateOverlappingWireSegments(wires) {
  const hSegs = [];
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.pins && w.pins.length > 0) continue;
    const pts = w.pts;
    if (!pts || pts.length < 4) continue;
    for (let j = 1; j < pts.length - 2; j++) {
      const p0 = pts[j];
      const p1 = pts[j + 1];
      if (Math.abs(p0.y - p1.y) < 0.5 && Math.abs(p1.x - p0.x) > 12) {
        const xA = p0.x;
        const xB = p1.x;
        hSegs.push({
          wire: w,
          segIdx: j,
          y: p0.y,
          minX: xA < xB ? xA : xB,
          maxX: xA > xB ? xA : xB
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
      if (sB.y - sA.y >= 5) break;
      if (
        visited[j] ||
        sB.wire === sA.wire ||
        sB.wire.chapterScope !== sA.wire.chapterScope ||
        sB.wire.targetGroupKey === sA.wire.targetGroupKey
      ) {
        continue;
      }
      const overlapX = (sA.maxX < sB.maxX ? sA.maxX : sB.maxX) - (sA.minX > sB.minX ? sA.minX : sB.minX);
      if (overlapX > 10) {
        if (!cluster) {
          cluster = [sA];
          visited[i] = 1;
        }
        cluster.push(sB);
        visited[j] = 1;
      }
    }
    if (cluster && cluster.length >= 2) {
      cluster.sort((a, b) => a.y - b.y || a.wire.x1 - b.wire.x1 || a.wire.y1 - b.wire.y1);
      for (let k = 0; k < cluster.length; k++) {
        const dy = (k - (cluster.length - 1) * 0.5) * 6;
        const item = cluster[k];
        const pts = item.wire.pts;
        pts[item.segIdx].y += dy;
        pts[item.segIdx + 1].y += dy;
      }
    }
  }
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
      if (Math.abs(p1.x - p0.x) < 0.5 && Math.abs(p1.y - p0.y) > 14) {
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
      if (Math.abs(p1.y - p0.y) >= 0.5 || Math.abs(p1.x - p0.x) <= 18) continue;
      const y = p0.y;
      const minX = (p0.x < p1.x ? p0.x : p1.x) + 9;
      const maxX = (p0.x > p1.x ? p0.x : p1.x) - 9;
      if (minX > maxX) continue;

      // Binary search first vertical segment with x >= minX
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
        if (vs.wire === w || vs.wire.chapterScope !== w.chapterScope) continue;
        if (vs.wire.targetGroupKey === w.targetGroupKey && Math.abs(vs.x - w.x2) < 8) continue;
        if (y > vs.minY + 9 && y < vs.maxY - 9) {
          if (vs.x - lastBridgeX >= 11) {
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
