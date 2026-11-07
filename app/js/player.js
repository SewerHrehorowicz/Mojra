"use strict";

let manualFocusCard = null;
let playerResizeObserver = null;

const playerScrollState = {
  cardKey: "present",
  offsetRatio: 0,
  lastWidth: 0,
  lastHeight: 0,
  lastWinW: 0,
  lastWinH: 0,
  lastSetScrollTop: null
};

function getSceneCardKey(card) {
  if (!card) return "present";
  if (card.getAttribute("data-present-scene") === "true") {
    return "present";
  }
  const hIdx = card.getAttribute("data-history-idx");
  if (hIdx !== null && hIdx !== "") {
    return "history:" + hIdx;
  }
  return "present";
}

function findSceneCardByKey(feed, cardKey) {
  if (!feed) return null;
  if (cardKey && cardKey.indexOf("history:") === 0) {
    const idx = cardKey.slice("history:".length);
    const found = qs('.scene-card[data-history-idx="' + idx + '"]', feed);
    if (found) return found;
  }
  return qs('.scene-card[data-present-scene="true"]', feed) || qs(".scene-card", feed);
}

function getSceneCardStartScrollTop(feed, card) {
  if (!feed || !card) return 0;
  const globalMeta = qs("#reader-global-meta", feed);
  const metaH = globalMeta ? globalMeta.offsetHeight : 0;
  let layoutTop = 0;
  let el = card;
  while (el && el !== feed && feed.contains(el)) {
    layoutTop += el.offsetTop;
    el = el.offsetParent;
  }
  const rawTop = layoutTop - Math.max(0, metaH - 1);
  return Math.max(0, Math.round(rawTop));
}

function recordPlayerViewportDimensions(feed) {
  if (!feed) return;
  playerScrollState.lastWidth = feed.clientWidth;
  playerScrollState.lastHeight = feed.clientHeight;
  playerScrollState.lastWinW = window.innerWidth;
  playerScrollState.lastWinH = window.innerHeight;
}

function hasPlayerViewportResized(feed) {
  if (!feed || playerScrollState.lastWidth <= 0 || playerScrollState.lastHeight <= 0) {
    return false;
  }
  return (
    feed.clientWidth !== playerScrollState.lastWidth ||
    feed.clientHeight !== playerScrollState.lastHeight ||
    window.innerWidth !== playerScrollState.lastWinW ||
    window.innerHeight !== playerScrollState.lastWinH
  );
}

function capturePlayerScrollState(feed) {
  if (!feed) return;
  const focusedCard =
    (manualFocusCard && feed.contains(manualFocusCard) ? manualFocusCard : null) ||
    qs(".scene-card.scene-focused", feed) ||
    qs('.scene-card[data-present-scene="true"]', feed) ||
    qs(".scene-card", feed);
  if (!focusedCard) return;

  playerScrollState.cardKey = getSceneCardKey(focusedCard);
  if (manualFocusCard && focusedCard === manualFocusCard) {
    playerScrollState.offsetRatio = 0;
  } else {
    const startTop = getSceneCardStartScrollTop(feed, focusedCard);
    const offsetPx = feed.scrollTop - startTop;
    if (Math.abs(offsetPx) <= 2) {
      playerScrollState.offsetRatio = 0;
    } else {
      const cardH = Math.max(1, focusedCard.offsetHeight);
      playerScrollState.offsetRatio = offsetPx / cardH;
    }
  }
  recordPlayerViewportDimensions(feed);
}

function restorePlayerScrollPosition(feed) {
  if (!feed) {
    feed = qs("#story-scroll-feed");
  }
  if (!feed) return;

  const targetCard = findSceneCardByKey(feed, playerScrollState.cardKey);
  if (!targetCard) return;

  if (manualFocusCard && feed.contains(manualFocusCard)) {
    manualFocusCard = null;
  }

  const startTop = getSceneCardStartScrollTop(feed, targetCard);
  const cardH = Math.max(1, targetCard.offsetHeight);
  const sampleHeader = qs(".scene-header-block", targetCard) || qs(".scene-header-block", feed);
  const headerH = sampleHeader ? sampleHeader.offsetHeight : 44;

  let offsetPx = Math.round(playerScrollState.offsetRatio * cardH);
  if (offsetPx > 0) {
    offsetPx = Math.min(offsetPx, Math.max(0, cardH - headerH - 2));
  } else if (offsetPx < 0) {
    offsetPx = Math.max(offsetPx, -headerH);
  }

  const targetScrollTop = Math.max(0, startTop + offsetPx);
  playerScrollState.lastSetScrollTop = targetScrollTop;
  feed.scrollTop = targetScrollTop;
  recordPlayerViewportDimensions(feed);

  updateFocusedSceneByViewport();
}

let activeScrollAnimFrame = null;

function cancelActiveScrollAnim() {
  if (activeScrollAnimFrame !== null) {
    cancelAnimationFrame(activeScrollAnimFrame);
    activeScrollAnimFrame = null;
  }
}

function animateFeedScrollTo(feed, targetOrCard, durationMs) {
  if (!feed) return;
  cancelActiveScrollAnim();
  const targetCard =
    targetOrCard && typeof targetOrCard === "object" ? targetOrCard : null;
  const resolveTarget = () => {
    const raw = targetCard
      ? getSceneCardStartScrollTop(feed, targetCard)
      : Number(targetOrCard) || 0;
    const maxScroll = Math.max(0, feed.scrollHeight - feed.clientHeight);
    return Math.min(Math.max(0, raw), maxScroll);
  };
  const startScrollTop = feed.scrollTop;
  const initialTarget = resolveTarget();
  if (Math.abs(initialTarget - startScrollTop) < 2 || durationMs <= 0) {
    playerScrollState.lastSetScrollTop = initialTarget;
    feed.scrollTop = initialTarget;
    updateFocusedSceneByViewport();
    return;
  }
  const startTime = performance.now();
  const step = (now) => {
    const elapsed = now - startTime;
    const t = Math.min(1, elapsed / durationMs);
    const eased = t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
    const currentTarget = resolveTarget();
    const delta = currentTarget - startScrollTop;
    const nextScrollTop = Math.round(startScrollTop + delta * eased);
    playerScrollState.lastSetScrollTop = nextScrollTop;
    feed.scrollTop = nextScrollTop;
    updateFocusedSceneByViewport();
    if (t < 1) {
      activeScrollAnimFrame = requestAnimationFrame(step);
    } else {
      const finalTarget = resolveTarget();
      playerScrollState.lastSetScrollTop = finalTarget;
      feed.scrollTop = finalTarget;
      activeScrollAnimFrame = null;
      if (targetCard) {
        playerScrollState.cardKey = getSceneCardKey(targetCard);
        playerScrollState.offsetRatio = 0;
        recordPlayerViewportDimensions(feed);
      } else {
        capturePlayerScrollState(feed);
      }
      updateFocusedSceneByViewport();
    }
  };
  activeScrollAnimFrame = requestAnimationFrame(step);
}

function scrollSceneCardToStart(feed, card, durationMs) {
  if (!feed || !card) return;
  animateFeedScrollTo(feed, card, durationMs || 1400);
}

let isAutoAdvancingImplicit = false;
let implicitOverscrollAccum = 0;
let implicitBottomReachedAt = 0;
let implicitTouchStartY = null;
let implicitSpringResetTimer = null;
const IMPLICIT_OVERSCROLL_THRESHOLD = 150;
const IMPLICIT_MAX_PULL_PX = 100;

function applyOverscrollSpringPull(feed, accum, activePull) {
  if (!feed) return;
  const clamped = Math.max(0, Math.min(IMPLICIT_OVERSCROLL_THRESHOLD, accum));
  const ratio = clamped / IMPLICIT_OVERSCROLL_THRESHOLD;
  const pullPx = Math.round(IMPLICIT_MAX_PULL_PX * (1 - Math.pow(1 - ratio, 1.45)));
  feed.classList.toggle("is-overscrolling", Boolean(activePull && pullPx > 0));
  const transformVal = pullPx > 0 ? "translateY(-" + pullPx + "px)" : "";
  const histStack = qs("#history-stack", feed);
  const activeStage = qs("#active-scene-stage", feed);
  if (histStack) histStack.style.transform = transformVal;
  if (activeStage) activeStage.style.transform = transformVal;
}

function resetOverscrollSpringPull(feed) {
  if (implicitSpringResetTimer) {
    clearTimeout(implicitSpringResetTimer);
    implicitSpringResetTimer = null;
  }
  implicitOverscrollAccum = 0;
  if (!feed) {
    feed = qs("#story-scroll-feed");
  }
  applyOverscrollSpringPull(feed, 0, false);
}

function scheduleOverscrollSpringBack(feed) {
  if (implicitSpringResetTimer) {
    clearTimeout(implicitSpringResetTimer);
  }
  implicitSpringResetTimer = setTimeout(() => {
    implicitSpringResetTimer = null;
    if (!isAutoAdvancingImplicit) {
      resetOverscrollSpringPull(feed);
    }
  }, 150);
}

function triggerImplicitNextAdvance() {
  if (isAutoAdvancingImplicit) return;
  if (qs("#modal-confirm")) return;
  const feed = qs("#story-scroll-feed");
  if (!feed) return;
  const implicitBtn = qs('#active-scene-stage button.choice-btn[data-implicit-next="true"]', feed);
  if (!implicitBtn) return;

  if (implicitSpringResetTimer) {
    clearTimeout(implicitSpringResetTimer);
    implicitSpringResetTimer = null;
  }
  isAutoAdvancingImplicit = true;
  implicitOverscrollAccum = 0;
  sendServerAction("AutoAdvanceImplicit", {}).finally(() => {
    setTimeout(() => {
      isAutoAdvancingImplicit = false;
      implicitOverscrollAccum = 0;
      implicitBottomReachedAt = 0;
    }, 520);
  });
}

function getSceneCardFeedTop(feed, card) {
  let layoutTop = 0;
  let el = card;
  while (el && el !== feed && feed.contains(el)) {
    layoutTop += el.offsetTop;
    el = el.offsetParent;
  }
  return layoutTop;
}

function updateFeedScrollCue(feed) {
  if (!feed) return 0;
  const remaining = feed.scrollHeight - feed.scrollTop - feed.clientHeight;
  const scrollCue = qs(".scroll-more-cue", feed);
  if (scrollCue) {
    scrollCue.classList.toggle("hidden", !(remaining > 12));
  }
  return remaining;
}

let modalFeedResizeObserver = null;

function parseClientProseBlockDirective(trimmed) {
  if (!trimmed) return null;
  if (trimmed.toLowerCase() === ":end") {
    return { kind: "endif", cond: "" };
  }
  const colonIdx = trimmed.indexOf(":");
  if (colonIdx <= 0) return null;
  const key = trimmed.slice(0, colonIdx).trim().toLowerCase();
  const val = trimmed.slice(colonIdx + 1).trim();
  if (key === "else" && !val) return { kind: "else", cond: "" };
  if (key === "endif" && !val) return { kind: "endif", cond: "" };
  if ((key === "elif" || key === "else if") && val) return { kind: "elif", cond: val };
  if ((key === "if" || key === "if?" || key === "gdy" || key === "warunek") && val) {
    return { kind: "if", cond: val };
  }
  if (key === "set" || key === "then") return { kind: "set", cond: val };
  return null;
}

function parseClientTestVarsMap(ta) {
  const map = new Map();
  if (!ta) return map;
  const form = (ta.closest && ta.closest("#form-scene-text")) || qs("#form-scene-text");
  const rawSpec = (form && form.getAttribute && form.getAttribute("data-test-vars")) || "";
  if (!rawSpec) return map;
  const parts = rawSpec.split(";");
  for (let i = 0; i < parts.length; i++) {
    const p = parts[i].trim();
    const eq = p.indexOf("=");
    if (eq > 0) {
      map.set(p.slice(0, eq).trim().toLowerCase(), p.slice(eq + 1).trim());
    }
  }
  return map;
}

function trimClientOuterParens(s) {
  let cur = (s || "").trim();
  while (cur.length >= 2 && cur[0] === "(" && cur[cur.length - 1] === ")") {
    let depth = 0;
    let valid = true;
    for (let i = 0; i < cur.length; i++) {
      if (cur[i] === "(") depth++;
      else if (cur[i] === ")") {
        depth--;
        if (depth === 0 && i < cur.length - 1) {
          valid = false;
          break;
        }
      }
    }
    if (!valid || depth !== 0) break;
    cur = cur.slice(1, -1).trim();
  }
  return cur;
}

function splitClientTopLevelLogical(s, op) {
  const parts = [];
  let depth = 0;
  let last = 0;
  const opLen = op.length;
  for (let i = 0; i <= s.length - opLen; i++) {
    if (s[i] === "(") depth++;
    else if (s[i] === ")" && depth > 0) depth--;
    else if (depth === 0 && s.slice(i, i + opLen) === op) {
      parts.push(s.slice(last, i));
      last = i + opLen;
      i += opLen - 1;
    }
  }
  if (parts.length === 0) return null;
  parts.push(s.slice(last));
  return parts;
}

const PLAYTXT_CMP_OPS = ["==", "!=", ">=", "\x3c=", ">", "\x3c", "="];
const PLAYTXT_TRUE_SET = new Set(["true", "tak", "yes", "on"]);
const PLAYTXT_FALSE_SET = new Set(["false", "nie", "no", "off"]);

function isPlaytxtBoolTrue(s) {
  return PLAYTXT_TRUE_SET.has(String(s || "").trim().toLowerCase());
}

function isPlaytxtBoolFalse(s) {
  return PLAYTXT_FALSE_SET.has(String(s || "").trim().toLowerCase());
}

function resolveClientOperandString(tok, stateMap) {
  let clean = (tok || "").trim();
  if ((clean.startsWith('"') && clean.endsWith('"')) || (clean.startsWith("'") && clean.endsWith("'"))) {
    return clean.slice(1, -1);
  }
  if (/^-?\d+$/.test(clean)) return clean;
  const lower = clean.toLowerCase();
  if (isPlaytxtBoolTrue(lower)) return "true";
  if (isPlaytxtBoolFalse(lower)) return "false";
  if (stateMap && stateMap.has(lower)) return stateMap.get(lower);
  return clean;
}

function evalClientProseAtom(raw, stateMap) {
  let s = (raw || "").trim();
  if (!s) return true;
  const unwrapped = trimClientOuterParens(s);
  if (unwrapped !== s) return evalClientProseCondition(unwrapped, stateMap);
  if (s.startsWith("?")) return evalClientProseAtom(s.slice(1), stateMap);
  if (s.startsWith("!") && !s.startsWith("!=")) return !evalClientProseAtom(s.slice(1), stateMap);
  for (let i = 0; i < PLAYTXT_CMP_OPS.length; i++) {
    const op = PLAYTXT_CMP_OPS[i];
    const idx = s.indexOf(op);
    if (idx > 0) {
      const lhs = s.slice(0, idx).trim();
      const rhs = s.slice(idx + op.length).trim();
      const lVal = resolveClientOperandString(lhs, stateMap);
      const rVal = resolveClientOperandString(rhs, stateMap);
      if (/^-?\d+$/.test(lVal) && /^-?\d+$/.test(rVal)) {
        const lInt = parseInt(lVal, 10);
        const rInt = parseInt(rVal, 10);
        if (op === "==" || op === "=") return lInt === rInt;
        if (op === "!=") return lInt !== rInt;
        if (op === ">=") return lInt >= rInt;
        if (op === "\x3c=") return lInt <= rInt;
        if (op === ">") return lInt > rInt;
        if (op === "\x3c") return lInt < rInt;
      }
      if (op === "==" || op === "=") return lVal.toLowerCase() === rVal.toLowerCase();
      if (op === "!=") return lVal.toLowerCase() !== rVal.toLowerCase();
      return false;
    }
  }
  const key = s.toLowerCase();
  const val = stateMap && stateMap.has(key) ? stateMap.get(key) : "";
  const lower = String(val).toLowerCase();
  if (isPlaytxtBoolTrue(lower)) return true;
  if (isPlaytxtBoolFalse(lower) || lower === "") return false;
  if (/^-?\d+$/.test(lower)) return parseInt(lower, 10) !== 0;
  return true;
}

function evalClientProseAnd(raw, stateMap) {
  const s = trimClientOuterParens(raw);
  const parts = splitClientTopLevelLogical(s, "&&");
  if (parts && parts.length > 1) {
    for (let i = 0; i < parts.length; i++) {
      if (!evalClientProseAtom(parts[i], stateMap)) return false;
    }
    return true;
  }
  return evalClientProseAtom(s, stateMap);
}

function evalClientProseCondition(raw, stateMap) {
  const s = trimClientOuterParens(raw);
  const parts = splitClientTopLevelLogical(s, "||");
  if (parts && parts.length > 1) {
    for (let i = 0; i < parts.length; i++) {
      if (evalClientProseAnd(parts[i], stateMap)) return true;
    }
    return false;
  }
  return evalClientProseAnd(s, stateMap);
}

function buildProseHighlightSegmentsClient(raw, stateMap) {
  const normalized = String(raw || "").replace(/\r\n/g, "\n");
  if (!normalized) {
    return [{ text: "", syn: false }];
  }
  const lines = normalized.split("\n");
  const segs = [];
  const appendSeg = (txt, syn) => {
    if (!txt) return;
    if (segs.length > 0 && segs[segs.length - 1].syn === syn) {
      segs[segs.length - 1].text += txt;
      return;
    }
    segs.push({ text: txt, syn: Boolean(syn) });
  };

  const stack = [];
  const isCurrentlyActive = () => {
    for (let j = 0; j < stack.length; j++) {
      if (!stack[j].branchActive) return false;
    }
    return true;
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = (line || "").trim();
    const dir = parseClientProseBlockDirective(trimmed);
    if (dir) {
      if (dir.kind === "endif") {
        if (stack.length > 0) stack.pop();
      } else if (dir.kind === "else") {
        if (stack.length > 0) {
          const top = stack[stack.length - 1];
          top.branchActive = !top.branchTaken;
          top.branchTaken = true;
        }
      } else if (dir.kind === "elif") {
        if (stack.length > 0) {
          const top = stack[stack.length - 1];
          if (top.branchTaken) {
            top.branchActive = false;
          } else {
            const met = evalClientProseCondition(dir.cond, stateMap);
            top.branchActive = met;
            if (met) top.branchTaken = true;
          }
        }
      } else if (dir.kind === "if") {
        const met = evalClientProseCondition(dir.cond, stateMap);
        stack.push({ branchTaken: met, branchActive: met });
      }
      appendSeg(line, true);
    } else if (trimmed === "---" || trimmed === "***" || !isCurrentlyActive()) {
      appendSeg(line, true);
    } else {
      let rest = line;
      while (rest.length > 0) {
        const openIdx = rest.indexOf("{{");
        if (openIdx < 0) {
          appendSeg(rest, false);
          break;
        }
        appendSeg(rest.slice(0, openIdx), false);
        const afterOpen = rest.slice(openIdx + 2);
        const closeIdx = afterOpen.indexOf("}}");
        if (closeIdx < 0) {
          appendSeg(rest.slice(openIdx), false);
          break;
        }
        appendSeg(rest.slice(openIdx, openIdx + 2 + closeIdx + 2), true);
        rest = afterOpen.slice(closeIdx + 2);
      }
    }
    if (i < lines.length - 1) {
      const lastSyn = segs.length > 0 && segs[segs.length - 1].syn;
      appendSeg("\n", lastSyn);
    }
  }
  if (normalized.endsWith("\n")) {
    appendSeg("\n", false);
  }
  if (segs.length === 0) {
    return [{ text: "", syn: false }];
  }
  return segs;
}

function syncSceneTextareaHighlight(ta) {
  if (!ta || typeof document === "undefined") return;
  const wrap = ta.closest ? ta.closest(".scene-prose-edit-wrap") : null;
  const hlEl = (wrap && qs("#modal-scene-prose-hl", wrap)) || qs("#modal-scene-prose-hl");
  if (!hlEl) return;
  if (hlEl.childNodes) {
    for (let i = hlEl.childNodes.length - 1; i >= 0; i--) {
      const cn = hlEl.childNodes[i];
      if (cn && cn.nodeType === 3) {
        hlEl.removeChild(cn);
      }
    }
  }
  const stateMap = parseClientTestVarsMap(ta);
  const segs = buildProseHighlightSegmentsClient(ta.value || "", stateMap);
  const firstChild = hlEl.children && hlEl.children.length > 0 ? hlEl.children[0] : null;
  const proto = firstChild && typeof firstChild.cloneNode === "function" ? firstChild.cloneNode(false) : null;
  if (!proto) {
    hlEl.textContent = ta.value || "";
    return;
  }
  while (hlEl.children.length < segs.length) {
    hlEl.appendChild(proto.cloneNode(false));
  }
  for (let i = 0; i < hlEl.children.length; i++) {
    const child = hlEl.children[i];
    if (i < segs.length) {
      const seg = segs[i];
      if (child.textContent !== seg.text) {
        child.textContent = seg.text;
      }
      child.classList.toggle("is-syntax", seg.syn);
      child.classList.remove("hidden");
    } else {
      if (child.textContent !== "") {
        child.textContent = "";
      }
      child.classList.remove("is-syntax");
      child.classList.add("hidden");
    }
  }
}

function autoResizeSceneTextarea(ta) {
  if (!ta) return;
  syncSceneTextareaHighlight(ta);
  const feed = ta.closest(".story-scroll-feed");
  const prevScrollTop = feed ? feed.scrollTop : 0;
  ta.style.minHeight = "0px";
  ta.style.height = "0px";
  const borderExtra = (ta.offsetHeight - ta.clientHeight) || 0;
  const fullH = Math.max(320, ta.scrollHeight + borderExtra);
  ta.style.height = fullH + "px";
  ta.style.minHeight = fullH + "px";
  ta.scrollTop = 0;
  if (feed) {
    feed.scrollTop = prevScrollTop;
    updateFeedScrollCue(feed);
  }
}

function initSceneTextModalFeed(resetScroll) {
  const feed = qs(".scene-text-editor-feed");
  if (!feed) {
    if (modalFeedResizeObserver) {
      modalFeedResizeObserver.disconnect();
      modalFeedResizeObserver = null;
    }
    return;
  }
  const ta = qs("#modal-scene-textarea", feed);
  if (ta) {
    autoResizeSceneTextarea(ta);
    if (!ta.__stBoundInput) {
      ta.__stBoundInput = true;
      ta.addEventListener("input", () => {
        autoResizeSceneTextarea(ta);
      });
      ta.addEventListener("scroll", () => {
        if (ta.scrollTop !== 0) {
          autoResizeSceneTextarea(ta);
        }
      });
    }
  }
  if (resetScroll) {
    feed.scrollTop = 0;
  }
  updateFeedScrollCue(feed);
  if (!feed.__stBoundScroll) {
    feed.__stBoundScroll = true;
    feed.addEventListener(
      "scroll",
      () => {
        updateFeedScrollCue(feed);
      },
      { passive: true }
    );
  }
  if (typeof ResizeObserver !== "undefined") {
    if (modalFeedResizeObserver) {
      modalFeedResizeObserver.disconnect();
    }
    modalFeedResizeObserver = new ResizeObserver(() => {
      const curTa = qs("#modal-scene-textarea", feed);
      if (curTa) {
        autoResizeSceneTextarea(curTa);
      } else {
        updateFeedScrollCue(feed);
      }
    });
    modalFeedResizeObserver.observe(feed);
  }
  requestAnimationFrame(() => {
    if (ta) {
      autoResizeSceneTextarea(ta);
    }
    updateFeedScrollCue(feed);
  });
}

function updateFocusedSceneByViewport() {
  const feed = qs("#story-scroll-feed");
  if (!feed) return;
  const remaining = updateFeedScrollCue(feed);
  if (remaining > 8) {
    if (implicitOverscrollAccum > 0) {
      resetOverscrollSpringPull(feed);
    }
    implicitBottomReachedAt = 0;
  } else if (implicitBottomReachedAt === 0) {
    implicitBottomReachedAt = Date.now();
  }
  const cards = qsa(".scene-card", feed);
  if (cards.length === 0) return;

  const globalMeta = qs("#reader-global-meta", feed);
  const metaH = globalMeta ? globalMeta.offsetHeight : 0;
  const sampleHeader = qs(".scene-header-block", feed);
  const headerH = sampleHeader ? sampleHeader.offsetHeight : 44;
  const focalScrollY = feed.scrollTop + metaH + headerH;

  let bestCard = null;
  let bestIdx = -1;

  if (manualFocusCard && feed.contains(manualFocusCard)) {
    const targetStart = getSceneCardStartScrollTop(feed, manualFocusCard);
    const maxScroll = Math.max(0, feed.scrollHeight - feed.clientHeight);
    const clampedTarget = Math.min(targetStart, maxScroll);
    bestCard = manualFocusCard;
    bestIdx = cards.indexOf(bestCard);
    if (Math.abs(feed.scrollTop - clampedTarget) <= 3) {
      manualFocusCard = null;
    }
  }

  if (!bestCard) {
    let lo = 0;
    let hi = cards.length - 1;
    let candidateIdx = 0;
    while (lo <= hi) {
      const mid = (lo + hi) >> 1;
      const top = getSceneCardFeedTop(feed, cards[mid]);
      if (top <= focalScrollY) {
        candidateIdx = mid;
        lo = mid + 1;
      } else {
        hi = mid - 1;
      }
    }
    const candCard = cards[candidateIdx];
    const candTop = getSceneCardFeedTop(feed, candCard);
    const candBottom = candTop + candCard.offsetHeight;
    if (focalScrollY > candBottom && candidateIdx + 1 < cards.length) {
      const nextCard = cards[candidateIdx + 1];
      const nextTop = getSceneCardFeedTop(feed, nextCard);
      if (Math.abs(nextTop - focalScrollY) < Math.abs(candBottom - focalScrollY)) {
        candidateIdx = candidateIdx + 1;
      }
    }
    bestIdx = candidateIdx;
    bestCard = cards[bestIdx];
  }

  for (let i = 0; i < cards.length; i++) {
    const card = cards[i];
    const isFocused = i === bestIdx;
    if (card.classList.contains("scene-focused") !== isFocused) {
      card.classList.toggle("scene-focused", isFocused);
    }
    if (card.classList.contains("scene-dimmed") !== !isFocused) {
      card.classList.toggle("scene-dimmed", !isFocused);
    }
  }

  const rail = qs("#story-history-rail");
  const railTicks = qsa("#story-history-rail .history-rail-tick");
  if (railTicks.length === cards.length) {
    const activeIdx = bestIdx;
    for (let idx = 0; idx < railTicks.length; idx++) {
      const isAct = idx === activeIdx;
      if (railTicks[idx].classList.contains("active") !== isAct) {
        railTicks[idx].classList.toggle("active", isAct);
      }
    }
    const activeTick = activeIdx >= 0 ? railTicks[activeIdx] : null;
    if (rail && activeTick && rail.scrollHeight > rail.clientHeight) {
      const tickTop = activeTick.offsetTop;
      const tickBottom = tickTop + activeTick.offsetHeight;
      if (tickTop < rail.scrollTop + 16) {
        rail.scrollTop = Math.max(0, tickTop - 16);
      } else if (tickBottom > rail.scrollTop + rail.clientHeight - 16) {
        rail.scrollTop = Math.max(0, tickBottom - rail.clientHeight + 16);
      }
    }
  }

  if (bestCard) {
    const metaEl = qs("#reader-global-meta-text");
    const metaText = bestCard.getAttribute("data-scene-meta") || "";
    if (metaEl && metaText) {
      metaEl.textContent = metaText;
    }
    const audio = qs("#story-audio");
    if (audio && audio.paused) {
      const cardAudio = bestCard.getAttribute("data-scene-audio") || "";
      const cardDur = bestCard.getAttribute("data-scene-audio-duration") || "0:00";
      if ((audio.getAttribute("data-audio-url") || "") !== cardAudio) {
        audio.setAttribute("data-audio-url", cardAudio);
        const curEl = qs("#audio-time-current");
        const totEl = qs("#audio-time-total");
        const slider = qs("#audio-seek-slider");
        if (curEl) curEl.textContent = "0:00";
        if (totEl) totEl.textContent = cardDur;
        if (slider) {
          slider.value = "0";
          slider.style.setProperty("--seek-pct", "0%");
        }
        if (cardAudio) {
          audio.src = cardAudio;
          audio.load();
        } else {
          audio.removeAttribute("src");
        }
      }
    }
  }

  const bgStage = qs("#player-bg-stage");
  if (bgStage && bestCard && document.body.getAttribute("data-imageless") !== "true") {
    const imgUrl = bestCard.getAttribute("data-scene-image") || "";
    const bgImg = qs("#player-bg-img", bgStage);
    if (bgImg) {
      if (imgUrl) {
        if (bgImg.getAttribute("src") !== imgUrl) {
          bgImg.setAttribute("src", imgUrl);
        }
        bgImg.classList.remove("hidden");
      } else {
        bgImg.classList.add("hidden");
      }
    }
  }
}

function initPlayerScrollAndAudio(savedScrollTop, shouldAutoPlay, preserveTime, animateFromScrollTop) {
  initAudioController(Boolean(shouldAutoPlay), preserveTime);
  if (typeof initNodeEditorCanvas === "function") {
    initNodeEditorCanvas();
  }
  initSceneTextModalFeed(false);

  const feed = qs("#story-scroll-feed");
  if (!feed) return;

  implicitOverscrollAccum = 0;
  implicitBottomReachedAt = 0;
  implicitTouchStartY = null;

  if (animateFromScrollTop !== null && animateFromScrollTop !== undefined) {
    manualFocusCard = null;
    playerScrollState.cardKey = "present";
    playerScrollState.offsetRatio = 0;
    const presentCard =
      qs('#active-scene-stage .scene-card[data-present-scene="true"]', feed) ||
      qs("#active-scene-stage .scene-card", feed);
    if (presentCard) {
      presentCard.classList.add("scene-entering");
    }
    feed.scrollTop = animateFromScrollTop;
    recordPlayerViewportDimensions(feed);
    updateFocusedSceneByViewport();
    requestAnimationFrame(() => {
      if (presentCard) {
        scrollSceneCardToStart(feed, presentCard, 1500);
      } else {
        restorePlayerScrollPosition(feed);
      }
    });
  } else if (savedScrollTop !== null && savedScrollTop !== undefined) {
    if (!hasPlayerViewportResized(feed) && playerScrollState.lastWidth > 0) {
      playerScrollState.lastSetScrollTop = savedScrollTop;
      feed.scrollTop = savedScrollTop;
      updateFocusedSceneByViewport();
      capturePlayerScrollState(feed);
    } else {
      restorePlayerScrollPosition(feed);
    }
    requestAnimationFrame(() => {
      if (hasPlayerViewportResized(feed)) {
        restorePlayerScrollPosition(feed);
      } else {
        updateFocusedSceneByViewport();
      }
    });
  } else {
    manualFocusCard = null;
    playerScrollState.cardKey = "present";
    playerScrollState.offsetRatio = 0;
    restorePlayerScrollPosition(feed);
    requestAnimationFrame(() => {
      restorePlayerScrollPosition(feed);
    });
  }

  feed.addEventListener(
    "wheel",
    (e) => {
      cancelActiveScrollAnim();
      manualFocusCard = null;
      const implicitBtn = qs('#active-scene-stage button.choice-btn[data-implicit-next="true"]', feed);
      if (!implicitBtn || isAutoAdvancingImplicit) {
        resetOverscrollSpringPull(feed);
        return;
      }
      const remaining = feed.scrollHeight - feed.scrollTop - feed.clientHeight;
      if (remaining > 6 || e.deltaY <= 0) {
        if (e.deltaY < 0 && implicitOverscrollAccum > 0) {
          resetOverscrollSpringPull(feed);
        }
        return;
      }
      if (implicitBottomReachedAt === 0) {
        implicitBottomReachedAt = Date.now();
      }
      implicitOverscrollAccum = Math.min(
        IMPLICIT_OVERSCROLL_THRESHOLD,
        implicitOverscrollAccum + Math.abs(e.deltaY)
      );
      applyOverscrollSpringPull(feed, implicitOverscrollAccum, true);
      scheduleOverscrollSpringBack(feed);
      if (implicitOverscrollAccum >= IMPLICIT_OVERSCROLL_THRESHOLD) {
        triggerImplicitNextAdvance();
      }
    },
    { passive: true }
  );

  feed.addEventListener(
    "touchstart",
    (e) => {
      cancelActiveScrollAnim();
      if (e.touches && e.touches.length === 1) {
        implicitTouchStartY = e.touches[0].clientY;
      }
    },
    { passive: true }
  );

  feed.addEventListener(
    "touchmove",
    (e) => {
      manualFocusCard = null;
      if (implicitTouchStartY === null || !e.touches || e.touches.length !== 1) return;
      const implicitBtn = qs('#active-scene-stage button.choice-btn[data-implicit-next="true"]', feed);
      if (!implicitBtn || isAutoAdvancingImplicit) return;
      const remaining = feed.scrollHeight - feed.scrollTop - feed.clientHeight;
      if (remaining > 6) {
        implicitTouchStartY = e.touches[0].clientY;
        if (implicitOverscrollAccum > 0) {
          resetOverscrollSpringPull(feed);
        }
        return;
      }
      const dragUp = implicitTouchStartY - e.touches[0].clientY;
      if (dragUp <= 0) {
        resetOverscrollSpringPull(feed);
        return;
      }
      const mappedAccum = Math.min(IMPLICIT_OVERSCROLL_THRESHOLD, (dragUp / 65) * IMPLICIT_OVERSCROLL_THRESHOLD);
      implicitOverscrollAccum = mappedAccum;
      applyOverscrollSpringPull(feed, implicitOverscrollAccum, true);
      if (dragUp >= 65) {
        implicitTouchStartY = null;
        triggerImplicitNextAdvance();
      }
    },
    { passive: true }
  );

  feed.addEventListener(
    "touchend",
    () => {
      implicitTouchStartY = null;
      if (!isAutoAdvancingImplicit && implicitOverscrollAccum > 0) {
        resetOverscrollSpringPull(feed);
      }
    },
    { passive: true }
  );

  feed.addEventListener(
    "touchcancel",
    () => {
      implicitTouchStartY = null;
      if (!isAutoAdvancingImplicit && implicitOverscrollAccum > 0) {
        resetOverscrollSpringPull(feed);
      }
    },
    { passive: true }
  );

  feed.addEventListener(
    "scroll",
    () => {
      if (hasPlayerViewportResized(feed)) {
        restorePlayerScrollPosition(feed);
        return;
      }
      if (
        playerScrollState.lastSetScrollTop !== null &&
        Math.abs(feed.scrollTop - playerScrollState.lastSetScrollTop) <= 1
      ) {
        playerScrollState.lastSetScrollTop = null;
        updateFocusedSceneByViewport();
        return;
      }
      playerScrollState.lastSetScrollTop = null;
      updateFocusedSceneByViewport();
      capturePlayerScrollState(feed);
    },
    { passive: true }
  );

  if (typeof ResizeObserver !== "undefined") {
    if (playerResizeObserver) {
      playerResizeObserver.disconnect();
    }
    playerResizeObserver = new ResizeObserver(() => {
      if (hasPlayerViewportResized(feed)) {
        restorePlayerScrollPosition(feed);
      }
    });
    playerResizeObserver.observe(feed);
  }

  const focusAndScrollCard = (targetCard) => {
    if (!targetCard) return;
    manualFocusCard = targetCard;
    playerScrollState.cardKey = getSceneCardKey(targetCard);
    playerScrollState.offsetRatio = 0;
    recordPlayerViewportDimensions(feed);
    updateFocusedSceneByViewport();
    scrollSceneCardToStart(feed, targetCard);
  };

  qsa("#story-history-rail .history-rail-tick").forEach((tick, idx) => {
    tick.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      focusAndScrollCard(qsa(".scene-card", feed)[idx]);
    });
  });

  qsa(".scene-card", feed).forEach((card) => {
    card.addEventListener("click", (e) => {
      if (e.target.closest("button") || e.target.closest("[action]")) return;
      if (card.classList.contains("scene-dimmed")) focusAndScrollCard(card);
    });
  });
}
