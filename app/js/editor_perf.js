"use strict";

// Set to false (or remove editor_perf.js from internal/tpl/bundle.go) to disable the perf overlay completely.
const EDITOR_PERF_OVERLAY_ENABLED = true;

// Host: Apple M1 Max (10-core Firestorm/Icestorm CPU, 32-core Metal 3 GPU, 32 GB Unified Memory).
// Low-end Android reference: 8x Cortex-A53 / A55 @ 1.8GHz + Mali-G52/G57 (~6x slower WebGL quad batching).
const HOST_HW_LABEL = "M1 Max (10c/32g)";
const LOWEND_ANDROID_LABEL = "Est. Low-End Android (A53)";
const LOWEND_ANDROID_MULTIPLIER = 6.0;

(function initEditorPerfModule() {
  if (!EDITOR_PERF_OVERLAY_ENABLED) return;

  const stats = {
    avgRenderMs: 0,
    avgRouteMs: 0,
    frameRouteMs: 0,
    hasRenderSample: false,
    nodeCount: 0,
    wireCount: 0,
    drawCalls: 1,
    quadCount: 0,
    lastDomUpdate: 0,
    pendingTimer: null
  };

  function ensurePerfOverlayDOM() {
    const el = typeof qs === "function" ? qs("#editor-perf-overlay") : document.getElementById("editor-perf-overlay");
    if (el) {
      el.classList.remove("hidden");
    }
    return el;
  }

  function flushPerfOverlayDOM() {
    stats.pendingTimer = null;
    stats.lastDomUpdate = performance.now();
    const el = ensurePerfOverlayDOM();
    if (!el) return;
    const totalM1 = stats.avgRenderMs + stats.avgRouteMs;
    const totalAndroid = totalM1 * LOWEND_ANDROID_MULTIPLIER;

    el.textContent =
      HOST_HW_LABEL +
      ": " +
      totalM1.toFixed(2) +
      " ms (route: " +
      stats.avgRouteMs.toFixed(2) +
      " ms, draw: " +
      stats.avgRenderMs.toFixed(2) +
      " ms, " +
      stats.drawCalls +
      " drawcall, " +
      stats.quadCount +
      " quads)  •  " +
      LOWEND_ANDROID_LABEL +
      ": ~" +
      totalAndroid.toFixed(2) +
      " ms  •  nodes: " +
      stats.nodeCount +
      ", wires: " +
      stats.wireCount;
  }

  function schedulePerfOverlayUpdate() {
    const now = performance.now();
    if (now - stats.lastDomUpdate >= 120) {
      flushPerfOverlayDOM();
    } else if (!stats.pendingTimer) {
      stats.pendingTimer = setTimeout(flushPerfOverlayDOM, 120);
    }
  }

  if (typeof routeStudioWires === "function") {
    const origRouteStudioWires = routeStudioWires;
    routeStudioWires = function () {
      const t0 = performance.now();
      origRouteStudioWires.apply(this, arguments);
      stats.frameRouteMs += performance.now() - t0;
    };
  }

  if (typeof renderStudioCanvas === "function") {
    const origRenderStudioCanvas = renderStudioCanvas;
    renderStudioCanvas = function () {
      const t0 = performance.now();
      origRenderStudioCanvas.apply(this, arguments);
      const dt = performance.now() - t0;
      const rMs = stats.frameRouteMs;
      stats.frameRouteMs = 0;

      stats.avgRenderMs = stats.hasRenderSample ? stats.avgRenderMs * 0.8 + dt * 0.2 : dt;
      stats.avgRouteMs = stats.hasRenderSample ? stats.avgRouteMs * 0.8 + rMs * 0.2 : rMs;
      stats.hasRenderSample = true;

      if (typeof studioGraphState !== "undefined") {
        stats.nodeCount = studioGraphState.nodes ? studioGraphState.nodes.length : 0;
        stats.wireCount = studioGraphState.wires ? studioGraphState.wires.length : 0;
        stats.drawCalls = studioGraphState.lastDrawCalls || 1;
      }
      if (typeof studioGL !== "undefined") {
        stats.quadCount = studioGL.totalInstCount || 0;
      }
      schedulePerfOverlayUpdate();
    };
  }
})();
