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

function linecastVertical(x, yA, yB, obstacles, ignoreId1, ignoreId2, pad) {
  if (!obstacles || obstacles.length === 0) return null;
  const minY = Math.min(yA, yB);
  const maxY = Math.max(yA, yB);
  if (maxY - minY < 2) return null;
  const p = pad !== undefined ? pad : 10;
  for (let i = 0; i < obstacles.length; i++) {
    const b = obstacles[i];
    if (b.hidden) continue;
    if (b.id && (b.id === ignoreId1 || b.id === ignoreId2)) continue;
    if (x > b.left - p && x < b.right + p && maxY > b.top - p && minY < b.bottom + p) {
      return b;
    }
  }
  return null;
}

function linecastHorizontal(y, xA, xB, obstacles, ignoreId1, ignoreId2, pad) {
  if (!obstacles || obstacles.length === 0) return null;
  const minX = Math.min(xA, xB);
  const maxX = Math.max(xA, xB);
  if (maxX - minX < 2) return null;
  const p = pad !== undefined ? pad : 10;
  for (let i = 0; i < obstacles.length; i++) {
    const b = obstacles[i];
    if (b.hidden) continue;
    if (b.id && (b.id === ignoreId1 || b.id === ignoreId2)) continue;
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

function buildWireWaypoints(x1, y1, x2, y2, fromBottom, portIdx, toLeft, toRight, isBottomPort, obstacles, fromId, toId) {
  const idx = portIdx || 0;
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

      let midY = resolveClearY((y1 + y2) * 0.5, x1, x2, obs, fromId, toId, 0, 14);
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

  // Side choice port
  const exitStub = 24 + idx * 4;
  const entryStub = 24;
  let outX = x1 + exitStub;
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

  if (inY >= fromBottom + 10) {
    const minMidY = fromBottom + 18 + idx * 4;
    const maxMidY = inY;
    let midY = Math.max(minMidY, Math.min(maxMidY, (fromBottom + y2) * 0.5 + idx * 4));
    outX = resolveClearX(outX, y1, midY, obs, fromId, toId, 1, 14);
    midY = resolveClearY(midY, outX, x2, obs, fromId, toId, 0, 14);

    const hitDrop = linecastVertical(x2, midY, y2 - 6, obs, fromId, toId, 10);
    if (!hitDrop) {
      return [
        { x: x1, y: y1 },
        { x: outX, y: y1 },
        { x: outX, y: midY },
        { x: x2, y: midY },
        { x: x2, y: y2 }
      ];
    }
    const detourX = resolveClearX(x2, midY, y2 - 16, obs, fromId, toId, x2 >= outX ? -1 : 1, 16);
    const entryY = resolveClearY(Math.min(y2 - 16, hitDrop.bottom + 16), detourX, x2, obs, fromId, toId, -1, 14);
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
    let channelX = Math.max(outX, Math.min(toLeft - 14, (x1 + toLeft) * 0.5 + idx * 4));
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
    const dipY = resolveClearY(fromBottom + 18 + idx * 4, outX, channelX, obs, fromId, toId, 1, 14);
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
  let bypassX = Math.max(outX, (toRight !== undefined && toRight !== null ? toRight : x2) + 28 + idx * 4);
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
  const underY = resolveClearY(fromBottom + 18 + idx * 4, outX, bypassX, obs, fromId, toId, 1, 14);
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

function syncUnreachableNodesMemory() {
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
  while (queue.length > 0) {
    const cur = queue.shift();
    const nexts = adj.get(cur) || [];
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
  return outPorts[matchIdx] || { x: ch.x + ch.w * 0.5, y: ch.y + ch.h, idx: 0 };
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

function routeSingleWireBasePts(w, nodeObstacles, chapterObstacles) {
  if (w.kind === "scene-to-chapter-out") {
    const fromNode = w.fromNode;
    const fromCh = w.fromChapterObj;
    const outPort = getChapterOutputPortSlot(fromCh, w.targetChapterLower);
    w.toChapterOutPort = outPort;
    w.x1 = fromNode.x + w.portRef.ox;
    w.y1 = fromNode.y + w.portRef.oy;
    w.x2 = outPort.x;
    w.y2 = outPort.y;
    w.unreachable = fromNode.unreachable;
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
      nodeObstacles,
      fromNode.id,
      ""
    );
    w.basePts = simplifyOrthogonalPoints(rawPts);
  } else if (w.kind === "scene-to-scene") {
    const fromNode = w.fromNode;
    const toNode = w.toNode;
    w.x1 = fromNode.x + w.portRef.ox;
    w.y1 = fromNode.y + w.portRef.oy;
    w.x2 = toNode.x + toNode.inOx;
    w.y2 = toNode.y + toNode.inOy;
    w.unreachable = fromNode.unreachable;
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
      nodeObstacles,
      fromNode.id,
      toNode.id
    );
    w.basePts = simplifyOrthogonalPoints(rawPts);
  } else if (w.kind === "chapter-to-chapter") {
    const fromCh = w.fromChapterObj;
    const toCh = w.toChapterObj;
    const outPort = w.fromChapterOutPort;
    w.x1 = outPort.x;
    w.y1 = outPort.y;
    w.x2 = toCh.inX;
    w.y2 = toCh.inY;
    w.unreachable = false;
    const rawPts = buildWireWaypoints(
      w.x1,
      w.y1,
      w.x2,
      w.y2,
      w.y1,
      w.portIdx,
      toCh.x,
      toCh.x + toCh.w,
      true,
      chapterObstacles,
      fromCh.id,
      toCh.id
    );
    w.basePts = simplifyOrthogonalPoints(rawPts);
  } else if (w.kind === "chapter-in-to-entry") {
    const ch = w.toChapterObj;
    const toNode = w.toNode;
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
      nodeObstacles,
      "",
      toNode.id
    );
    w.basePts = simplifyOrthogonalPoints(rawPts);
  }
  w.bbox = computeWireBBox(w.basePts);
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
        const fromCh = studioGraphState.chapterByKey.get(fromNode.chapterLower);
        if (!fromCh || fromCh.hidden) continue;
        const targetChLower = targetChapter.toLowerCase();
        pt.connected = true;
        wires.push({
          kind: "scene-to-chapter-out",
          fromNode: fromNode,
          toNode: null,
          fromChapterObj: fromCh,
          toChapterObj: null,
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
          basePts: [],
          pts: [],
          bbox: null
        });
        continue;
      }

      const toNode = studioGraphState.nodeById.get(targetId);
      if (!toNode) continue;
      if (fromNode.chapterLower && toNode.chapterLower && fromNode.chapterLower !== toNode.chapterLower) {
        continue;
      }
      pt.connected = true;
      toNode.inConnected = true;
      wires.push({
        kind: "scene-to-scene",
        fromNode: fromNode,
        toNode: toNode,
        fromChapterObj: null,
        toChapterObj: null,
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
        basePts: [],
        pts: [],
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
        for (let k = 0; k < nodes.length; k++) {
          if (nodes[k].chapterLower === fromCh.nameLower && (nodes[k].isEntry || !entryNode)) {
            entryNode = nodes[k];
            if (nodes[k].isEntry) break;
          }
        }
      }
      if (entryNode) {
        entryNode.inConnected = true;
        wires.push({
          kind: "chapter-in-to-entry",
          fromNode: null,
          toNode: entryNode,
          fromChapterObj: fromCh,
          toChapterObj: fromCh,
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
          basePts: [],
          pts: [],
          bbox: null
        });
      }
    }
    for (let j = 0; j < fromCh.outPorts.length; j++) {
      const op = fromCh.outPorts[j];
      if (!op.targetLower) continue;
      const toCh = studioGraphState.chapterByKey.get(op.targetLower);
      if (!toCh || toCh.hidden || toCh === fromCh) continue;
      op.connected = true;
      toCh.inConnected = true;
      wires.push({
        kind: "chapter-to-chapter",
        fromNode: null,
        toNode: null,
        fromChapterObj: fromCh,
        toChapterObj: toCh,
        fromChapterOutPort: op,
        toChapterOutPort: null,
        portRef: op,
        portIdx: j,
        isBottomPort: true,
        targetChapterLower: op.targetLower,
        targetGroupKey: "ch_in:" + op.targetLower,
        unreachable: false,
        x1: 0,
        y1: 0,
        x2: 0,
        y2: 0,
        basePts: [],
        pts: [],
        bbox: null
      });
    }
  }

  return wires;
}

function applyParallelOffsetsAndBundles(wires, dirtyGroupKeys) {
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
      const w = group[i];
      w.pts = w.basePts.map((p) => ({ x: p.x, y: p.y }));
    }
    if (group.length < 2) return;

    const getApproachX = (it) => (it.pts.length >= 3 ? it.pts[it.pts.length - 3].x : it.x1);
    group.sort((a, b) => {
      const ax = getApproachX(a);
      const bx = getApproachX(b);
      if (Math.abs(ax - bx) > 1) return ax - bx;
      if (Math.abs(a.y1 - b.y1) > 1) return a.y1 - b.y1;
      return (a.portIdx || 0) - (b.portIdx || 0);
    });

    if (group.length <= 3) {
      const hasCenterVertical = group.some((it) => it.pts.length === 2);
      for (let idx = 0; idx < group.length; idx++) {
        const it = group[idx];
        const n = it.pts.length;
        if (n === 2) continue;
        let dx = (idx - (group.length - 1) * 0.5) * 3.5;
        if (hasCenterVertical && group.length === 2) {
          dx = getApproachX(it) < it.x2 ? -3.5 : 3.5;
        }
        if (Math.abs(dx) < 0.1) continue;
        if (n >= 3) {
          it.pts[n - 2].x = it.x2 + dx;
          it.pts[n - 1] = { x: it.x2 + dx, y: it.y2 - 3 };
          it.pts.push({ x: it.x2, y: it.y2 });
        }
      }
    } else {
      const tx = group[0].x2;
      const ty2 = group[0].y2;
      let ty1 = ty2 - 36;
      for (let idx = 0; idx < group.length; idx++) {
        const it = group[idx];
        const n = it.pts.length;
        if (n >= 2) {
          const candY = it.pts[n - 2].y;
          if (candY < ty2 - 16 && candY > ty1) {
            ty1 = candY;
          }
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
}

function routeStudioWires(dirtyNodeIds, dirtyAabbs, dirtyChaptersSet) {
  const nodeObstacles = studioGraphState.nodes;
  const chapterObstacles = studioGraphState.chapters;

  if (!dirtyNodeIds || studioGraphState.wires.length === 0) {
    syncUnreachableNodesMemory();
    const wires = buildWireCatalogFromMemory();
    for (let i = 0; i < wires.length; i++) {
      routeSingleWireBasePts(wires[i], nodeObstacles, chapterObstacles);
    }
    studioGraphState.wires = wires;
    applyParallelOffsetsAndBundles(wires, null);
    studioGraphState.staticBatchDirty = true;
    return;
  }

  const wires = studioGraphState.wires;
  const dirtyGroupKeys = new Set();

  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    let isDirty = false;
    if (w.fromNode && dirtyNodeIds.has(w.fromNode.id)) {
      isDirty = true;
    } else if (w.toNode && dirtyNodeIds.has(w.toNode.id)) {
      isDirty = true;
    } else if (dirtyChaptersSet && w.fromChapterObj && dirtyChaptersSet.has(w.fromChapterObj)) {
      isDirty = true;
    } else if (dirtyChaptersSet && w.toChapterObj && dirtyChaptersSet.has(w.toChapterObj)) {
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
      routeSingleWireBasePts(w, nodeObstacles, chapterObstacles);
      dirtyGroupKeys.add(w.targetGroupKey);
    }
  }

  if (dirtyGroupKeys.size > 0) {
    applyParallelOffsetsAndBundles(wires, dirtyGroupKeys);
  }
  studioGraphState.staticBatchDirty = true;
}

function updateTempWirePreview() {
  studioGraphState.overlayBatchDirty = true;
  renderStudioCanvas();
}

function drawStudioWires(dirtyNodeIds, dirtyAabbs, dirtyChaptersSet) {
  routeStudioWires(dirtyNodeIds, dirtyAabbs, dirtyChaptersSet);
  renderStudioCanvas();
}
