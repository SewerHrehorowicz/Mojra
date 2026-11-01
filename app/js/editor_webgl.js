"use strict";

const EDITOR_GRID_SIZE = 24;
const NODE_CARD_WIDTH = 192;
const NODE_HEADER_HEIGHT = 34;
const NODE_CHOICE_ROW_HEIGHT = 26;
const NODE_FOOTER_HEIGHT = 40;
const NODE_BASE_HEIGHT = 74;
const PORT_RADIUS = 6.5;
const CHAPTER_PORT_RADIUS = 3.5;
const CHAPTER_OUT_PORT_PITCH = 37;

// Texture 0: Dedicated 256x64 UI Sprite Atlas (solid white, SDF ports, 90° wire corners, 2x3 grip)
const UI_ATLAS_W = 256;
const UI_ATLAS_H = 64;

// Texture 1: Dedicated 512x512 Font MSDF Texture (/studio-atlas.png)
const FONT_ATLAS_SIZE = 512;
const MSDF_BASE_EM_PX = 23.0;
const MSDF_PIXEL_RANGE = 3.0;

// 3 real TrueType fonts baked into /studio-atlas.png at build time:
// 0 = Aleo-Regular.ttf, 1 = Lato-Regular.ttf, 2 = Lato-Bold.ttf
const MSDF_FONT_ALEO_REG = 0;
const MSDF_FONT_LATO_REG = 1;
const MSDF_FONT_LATO_BOLD = 2;

const FONT_TITLE = 0;   // Aleo-Regular 13px
const FONT_CHOICE = 1;  // Lato-Regular 11px
const FONT_CHAPTER = 2; // Lato-Bold 11px
const FONT_BADGE = 3;   // Lato-Bold 9px
const FONT_BTN = 4;     // Lato-Regular 10px

const FONT_STYLES = [
  { ttfId: MSDF_FONT_ALEO_REG, size: 13, track: 0.0 },
  { ttfId: MSDF_FONT_LATO_REG, size: 11, track: 0.0 },
  { ttfId: MSDF_FONT_LATO_BOLD, size: 11, track: 0.65 },
  { ttfId: MSDF_FONT_LATO_BOLD, size: 9, track: 0.45 },
  { ttfId: MSDF_FONT_LATO_REG, size: 10, track: 0.6 }
];

const editorCanvasState = {
  panX: 0,
  panY: 0,
  zoom: 1.0,
  initialized: false,
  spacePressed: false,
  gestureBaseZoom: 1.0,
  suppressNextContextMenu: false
};

const studioGraphState = {
  chapters: [],
  chapterByKey: new Map(),
  nodes: [],
  nodeById: new Map(),
  wires: [],
  wirePins: new Map(),
  bundleOverlays: [],
  hoveredItem: null,
  editingChapter: null,
  editingNodeTitle: null,
  editingChoice: null,
  selectedChapterKey: "",
  themeColors: null,
  shapedTextCache: new Map(),
  viewW: 1200,
  viewH: 700,
  canvasDpr: 2,
  staticBatchDirty: true,
  overlayBatchDirty: true,
  lastRouteMs: 0,
  lastDrawCalls: 1
};

let studioViewportResizeObserver = null;
let observedViewportEl = null;

const selectedNodeIds = new Set();
let editorSelectionCleared = false;

let activeCanvasGesture = null;
let viewportSaveTimer = null;
let pendingContextMenuWorldPos = { x: 768, y: 120 };
let suppressNextTouchClickUntil = 0;
let lastPointerClientPos = { x: null, y: null };
let lastDisplayedZoomPct = -1;

// Instanced GPU Quad Batcher: 9 floats (36 bytes) per instance [x, y, w, h, u0, v0, u1, v1, packedRGBA]
const FLOATS_PER_INSTANCE = 9;
const BYTES_PER_INSTANCE = 36;
let maxInstancesCapacity = 32768;

let instanceArrayBuffer = new ArrayBuffer(maxInstancesCapacity * BYTES_PER_INSTANCE);
let instanceFloatView = new Float32Array(instanceArrayBuffer);
let instanceUint32View = new Uint32Array(instanceArrayBuffer);

function growInstanceBuffer() {
  const nextCap = maxInstancesCapacity * 2;
  const nextBuf = new ArrayBuffer(nextCap * BYTES_PER_INSTANCE);
  const nextFloat = new Float32Array(nextBuf);
  const nextUint = new Uint32Array(nextBuf);
  nextFloat.set(instanceFloatView);
  maxInstancesCapacity = nextCap;
  instanceArrayBuffer = nextBuf;
  instanceFloatView = nextFloat;
  instanceUint32View = nextUint;
  studioGL.vboCapacityBytes = 0;
}

const studioGL = {
  canvas: null,
  gl: null,
  instExt: null,
  prog: null,
  quadVbo: null,
  instVbo: null,
  uiTex: null,
  fontTex: null,
  uViewSize: null,
  uPanZoom: null,
  uDpr: null,
  uBgColor: null,
  uDotColor: null,
  uGridStep: null,
  uUIAtlas: null,
  uFontAtlas: null,
  instCursor: 0,
  staticInstCount: 0,
  totalInstCount: 0,
  atlasImg: null,
  atlasImgLoaded: false,
  atlasUploaded: false,
  metricsParsed: false,
  glyphMaps: [new Map(), new Map(), new Map()],
  uv: {
    whiteU: 4 / UI_ATLAS_W,
    whiteV: 4 / UI_ATLAS_H,
    portFill: { u0: 16 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 48 / UI_ATLAS_W, v1: 34 / UI_ATLAS_H, size: 24 },
    portRing: { u0: 52 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 84 / UI_ATLAS_W, v1: 34 / UI_ATLAS_H, size: 24 },
    portHover: { u0: 88 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 120 / UI_ATLAS_W, v1: 34 / UI_ATLAS_H, size: 24 },
    smallPortFill: { u0: 16 / UI_ATLAS_W, v0: 36 / UI_ATLAS_H, u1: 40 / UI_ATLAS_W, v1: 60 / UI_ATLAS_H, size: 16 },
    smallPortRing: { u0: 52 / UI_ATLAS_W, v0: 36 / UI_ATLAS_H, u1: 76 / UI_ATLAS_W, v1: 60 / UI_ATLAS_H, size: 16 },
    smallPortHover: { u0: 88 / UI_ATLAS_W, v0: 36 / UI_ATLAS_H, u1: 112 / UI_ATLAS_W, v1: 60 / UI_ATLAS_H, size: 16 },
    bridgeArc: { u0: 188 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 220 / UI_ATLAS_W, v1: 22 / UI_ATLAS_H, w: 16, h: 10 },
    grip: { u0: 124 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 144 / UI_ATLAS_W, v1: 24 / UI_ATLAS_H, w: 20, h: 22 },
    branchPlus: { u0: 120 / UI_ATLAS_W, v0: 34 / UI_ATLAS_H, u1: 148 / UI_ATLAS_W, v1: 62 / UI_ATLAS_H, size: 15 },
    editPen: { u0: 152 / UI_ATLAS_W, v0: 34 / UI_ATLAS_H, u1: 180 / UI_ATLAS_W, v1: 62 / UI_ATLAS_H, size: 15 },
    trashIcon: { u0: 184 / UI_ATLAS_W, v0: 34 / UI_ATLAS_H, u1: 212 / UI_ATLAS_W, v1: 62 / UI_ATLAS_H, size: 12 },
    cornerTL: { u0: 152 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 168 / UI_ATLAS_W, v1: 18 / UI_ATLAS_H },
    cornerTR: { u0: 168 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 184 / UI_ATLAS_W, v1: 18 / UI_ATLAS_H },
    cornerBL: { u0: 152 / UI_ATLAS_W, v0: 18 / UI_ATLAS_H, u1: 168 / UI_ATLAS_W, v1: 34 / UI_ATLAS_H },
    cornerBR: { u0: 168 / UI_ATLAS_W, v0: 18 / UI_ATLAS_H, u1: 184 / UI_ATLAS_W, v1: 34 / UI_ATLAS_H }
  }
};

function buildUISpriteTextureBytes() {
  const data = new Uint8Array(UI_ATLAS_W * UI_ATLAS_H * 4);
  const setPixel = (x, y, r, g, b, a) => {
    const idx = (y * UI_ATLAS_W + x) << 2;
    data[idx] = r;
    data[idx + 1] = g;
    data[idx + 2] = b;
    data[idx + 3] = a;
  };
  const distToByte = (sd) => {
    const norm = Math.max(0.0, Math.min(1.0, 0.5 + sd / (2.0 * MSDF_PIXEL_RANGE)));
    return Math.round(norm * 255);
  };

  // 1. Solid white block at (2..8, 2..8)
  for (let y = 2; y < 8; y++) {
    for (let x = 2; x < 8; x++) {
      setPixel(x, y, 255, 255, 255, 255);
    }
  }

  // 2. Analytic SDF Ports & 90° Wire Corners
  const spScale = 32.0 / 24.0;
  for (let y = 0; y < 32; y++) {
    for (let x = 0; x < 32; x++) {
      const wx = (x + 0.5 - 16.0) / spScale;
      const wy = (y + 0.5 - 16.0) / spScale;
      const r = Math.hypot(wx, wy);

      const bFill = distToByte(PORT_RADIUS - r);
      setPixel(16 + x, 2 + y, bFill, bFill, bFill, bFill);

      const bRing = distToByte(1.0 - Math.abs(r - PORT_RADIUS));
      setPixel(52 + x, 2 + y, bRing, bRing, bRing, bRing);

      const bHov = distToByte(PORT_RADIUS + 1.0 - r);
      setPixel(88 + x, 2 + y, bHov, bHov, bHov, bHov);

      const cwx = (x + 0.5 - 16.0) * (18.0 / 32.0);
      const cwy = (y + 0.5 - 16.0) * (18.0 / 32.0);
      const cr = Math.hypot(cwx, cwy);
      const bCorner = distToByte(1.0 - Math.abs(cr - 8.0));
      setPixel(152 + x, 2 + y, bCorner, bCorner, bCorner, bCorner);
    }
  }

  // 2b. Analytic 2x Smaller SDF Ports (r = 3.5px) for Chapter Ports & Wire Control Pins
  const smScale = 24.0 / 16.0;
  for (let y = 0; y < 24; y++) {
    for (let x = 0; x < 24; x++) {
      const wx = (x + 0.5 - 12.0) / smScale;
      const wy = (y + 0.5 - 12.0) / smScale;
      const r = Math.hypot(wx, wy);

      const bFill = distToByte(CHAPTER_PORT_RADIUS - r);
      setPixel(16 + x, 36 + y, bFill, bFill, bFill, bFill);

      const bRing = distToByte(1.0 - Math.abs(r - CHAPTER_PORT_RADIUS));
      setPixel(52 + x, 36 + y, bRing, bRing, bRing, bRing);

      const bHov = distToByte(CHAPTER_PORT_RADIUS + 0.8 - r);
      setPixel(88 + x, 36 + y, bHov, bHov, bHov, bHov);
    }
  }

  // 2c. Analytic Semicircular Wire Crossing Bridge (R = 5.5px) at (188..220, 2..22)
  const bridgeR = 5.5;
  for (let y = 0; y < 20; y++) {
    for (let x = 0; x < 32; x++) {
      const wx = (x + 0.5 - 16.0) * (16.0 / 32.0);
      const wy = (y + 0.5 - 16.0) * (10.0 / 20.0);
      const d = wy <= 0.0 ? Math.abs(Math.hypot(wx, wy) - bridgeR) : Math.hypot(Math.abs(wx) - bridgeR, wy);
      const bArc = distToByte(1.0 - d);
      setPixel(188 + x, 2 + y, bArc, bArc, bArc, bArc);
    }
  }

  // 3. Grip 2x3 dots at (124, 2, 20, 22)
  for (let row = 0; row < 3; row++) {
    const cy = 2 + 5 + row * 5;
    for (let dy = 0; dy < 2; dy++) {
      for (let dx = 0; dx < 2; dx++) {
        setPixel(124 + 6 + dx, cy + dy, 255, 255, 255, 255);
        setPixel(124 + 11 + dx, cy + dy, 255, 255, 255, 255);
      }
    }
  }

  // 4. Analytic SDF Vector Icons (branchPlus at 120,34; editPen at 152,34; trashIcon at 184,34)
  const ptSegDist = (px, py, x1, y1, x2, y2) => {
    const dx = x2 - x1;
    const dy = y2 - y1;
    const l2 = dx * dx + dy * dy;
    if (l2 < 0.0001) return Math.hypot(px - x1, py - y1);
    let t = ((px - x1) * dx + (py - y1) * dy) / l2;
    if (t < 0) t = 0;
    else if (t > 1) t = 1;
    return Math.hypot(px - (x1 + t * dx), py - (y1 + t * dy));
  };
  const rasterizeSDFIcon = (ox, oy, segs, halfStroke) => {
    const sc = 28.0 / 24.0;
    for (let y = 0; y < 28; y++) {
      for (let x = 0; x < 28; x++) {
        const ix = (x + 0.5) / sc;
        const iy = (y + 0.5) / sc;
        let minD = 999.0;
        for (let s = 0; s < segs.length; s++) {
          const sg = segs[s];
          const d = ptSegDist(ix, iy, sg[0], sg[1], sg[2], sg[3]);
          if (d < minD) minD = d;
        }
        const b = distToByte(halfStroke - minD);
        setPixel(ox + x, oy + y, b, b, b, b);
      }
    }
  };
  rasterizeSDFIcon(
    120,
    34,
    [
      [3, 12, 8, 12],
      [8, 12, 12, 7],
      [12, 7, 15, 7],
      [8, 12, 12, 17],
      [12, 17, 18, 17],
      [19, 4, 19, 10],
      [16, 7, 22, 7]
    ],
    0.95
  );
  rasterizeSDFIcon(
    152,
    34,
    [
      [12, 20, 21, 20],
      [16.5, 3.5, 20.5, 7.5],
      [20.5, 7.5, 7, 21],
      [7, 21, 3, 21],
      [3, 21, 3, 17],
      [3, 17, 16.5, 3.5]
    ],
    0.95
  );
  rasterizeSDFIcon(
    184,
    34,
    [
      [3, 6, 21, 6],
      [8, 6, 8, 4],
      [8, 4, 16, 4],
      [16, 4, 16, 6],
      [6, 6, 7, 20],
      [7, 20, 17, 20],
      [17, 20, 18, 6]
    ],
    0.95
  );
  return data;
}

function ensureBuildTimeMSDFMetricsParsed() {
  if (studioGL.metricsParsed) return;
  studioGL.metricsParsed = true;
  const raw = typeof window !== "undefined" ? window.__STUDIO_MSDF_DATA || "" : "";
  if (!raw) return;
  const lines = raw.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!line) continue;
    const p = line.split("|");
    if (p.length < 11) continue;
    const fontId = parseInt(p[0], 10) || 0;
    const ch = p[1];
    const map = studioGL.glyphMaps[fontId];
    if (!map) continue;
    map.set(ch, {
      u0: parseFloat(p[2]),
      v0: parseFloat(p[3]),
      u1: parseFloat(p[4]),
      v1: parseFloat(p[5]),
      w: parseFloat(p[6]),
      h: parseFloat(p[7]),
      ox: parseFloat(p[8]),
      oy: parseFloat(p[9]),
      adv: parseFloat(p[10])
    });
  }
}

function startPreloadingMSDFAtlasImage() {
  if (studioGL.atlasImg || typeof Image === "undefined") return;
  const img = new Image();
  studioGL.atlasImg = img;
  img.onload = () => {
    studioGL.atlasImgLoaded = true;
    studioGL.atlasUploaded = false;
    if (qs("#studio-canvas")) {
      renderStudioCanvas();
    }
  };
  img.src = "/studio-atlas.png";
}

if (typeof window !== "undefined") {
  ensureBuildTimeMSDFMetricsParsed();
  startPreloadingMSDFAtlasImage();
}

function packRGBA(r, g, b, a) {
  return (
    ((Math.round(a * 255) & 255) << 24) |
    ((Math.round(b * 255) & 255) << 16) |
    ((Math.round(g * 255) & 255) << 8) |
    (Math.round(r * 255) & 255)
  ) >>> 0;
}

function invalidateStudioThemeCache() {
  studioGraphState.themeColors = null;
  studioGraphState.staticBatchDirty = true;
  studioGraphState.overlayBatchDirty = true;
}

function resolveStudioThemeColors() {
  if (studioGraphState.themeColors) {
    return studioGraphState.themeColors;
  }
  const isLight = document.body && document.body.getAttribute("data-theme") === "light";
  studioGraphState.themeColors = isLight
    ? {
        isLight: true,
        bgObsidianRgb: [1.0, 1.0, 1.0],
        gridDotRgb: [0.74, 0.74, 0.74],
        bgObsidian: packRGBA(1.0, 1.0, 1.0, 1.0),
        bgElevated: packRGBA(0.957, 0.957, 0.957, 1.0),
        borderSubtle: packRGBA(0.886, 0.886, 0.886, 1.0),
        borderStrong: packRGBA(0.6, 0.6, 0.6, 1.0),
        textPrimary: packRGBA(0.0, 0.0, 0.0, 1.0),
        textSecondary: packRGBA(0.2, 0.2, 0.2, 1.0),
        textMuted: packRGBA(0.4, 0.4, 0.4, 1.0),
        chapterFill: packRGBA(0.0, 0.0, 0.0, 0.018),
        marqueeFill: packRGBA(0.0, 0.0, 0.0, 0.07),
        unreachCardBg: packRGBA(0.957, 0.957, 0.957, 0.55),
        unreachBorder: packRGBA(0.85, 0.85, 0.85, 0.55),
        unreachText: packRGBA(0.45, 0.45, 0.45, 0.55),
        unreachWire: packRGBA(0.78, 0.78, 0.78, 0.55)
      }
    : {
        isLight: false,
        bgObsidianRgb: [0.0, 0.0, 0.0],
        gridDotRgb: [0.16, 0.16, 0.16],
        bgObsidian: packRGBA(0.0, 0.0, 0.0, 1.0),
        bgElevated: packRGBA(0.047, 0.047, 0.047, 1.0),
        borderSubtle: packRGBA(0.141, 0.141, 0.141, 1.0),
        borderStrong: packRGBA(0.322, 0.322, 0.322, 1.0),
        textPrimary: packRGBA(1.0, 1.0, 1.0, 1.0),
        textSecondary: packRGBA(0.784, 0.784, 0.784, 1.0),
        textMuted: packRGBA(0.58, 0.58, 0.58, 1.0),
        chapterFill: packRGBA(0.5, 0.5, 0.5, 0.03),
        marqueeFill: packRGBA(0.62, 0.62, 0.62, 0.09),
        unreachCardBg: packRGBA(0.047, 0.047, 0.047, 0.55),
        unreachBorder: packRGBA(0.18, 0.18, 0.18, 0.55),
        unreachText: packRGBA(0.5, 0.5, 0.5, 0.55),
        unreachWire: packRGBA(0.22, 0.22, 0.22, 0.55)
      };
  return studioGraphState.themeColors;
}

function lookupMSDFGlyph(ttfId, ch) {
  ensureBuildTimeMSDFMetricsParsed();
  const map = studioGL.glyphMaps[ttfId];
  if (!map) return null;
  return map.get(ch) || map.get("?") || null;
}

function shapeTextGlyphs(styleId, text, maxWidth) {
  if (!text || maxWidth <= 4) return { glyphs: [], totalW: 0, pxRange: 3.4 };
  const key = styleId + "|" + Math.round(maxWidth) + "|" + text;
  const cached = studioGraphState.shapedTextCache.get(key);
  if (cached) return cached;

  const style = FONT_STYLES[styleId];
  const scale = style.size / MSDF_BASE_EM_PX;
  const extraTrack = style.track;
  const pxRange = scale * (2.0 * MSDF_PIXEL_RANGE);
  const baselineOffsetY = style.size * 0.36;

  const chars = Array.from(text);
  const fullGlyphs = new Array(chars.length);
  let totalW = 0;
  for (let i = 0; i < chars.length; i++) {
    const g = lookupMSDFGlyph(style.ttfId, chars[i]);
    fullGlyphs[i] = g;
    totalW += (g ? g.adv : MSDF_BASE_EM_PX * 0.3) * scale + extraTrack;
  }

  const glyphs = [];

  if (totalW <= maxWidth) {
    let curX = 0;
    for (let i = 0; i < fullGlyphs.length; i++) {
      const g = fullGlyphs[i];
      if (g && g.w > 0 && chars[i] !== " ") {
        glyphs.push({
          dx: curX + g.ox * scale,
          dy: baselineOffsetY + g.oy * scale,
          w: g.w * scale,
          h: g.h * scale,
          u0: g.u0,
          v0: g.v0,
          u1: g.u1,
          v1: g.v1
        });
      }
      curX += (g ? g.adv : MSDF_BASE_EM_PX * 0.3) * scale + extraTrack;
    }
    const res = { glyphs: glyphs, totalW: totalW, pxRange: pxRange };
    if (studioGraphState.shapedTextCache.size > 4000) {
      studioGraphState.shapedTextCache.clear();
    }
    studioGraphState.shapedTextCache.set(key, res);
    return res;
  }

  const ellG = lookupMSDFGlyph(style.ttfId, "…");
  const ellW = (ellG ? ellG.adv : MSDF_BASE_EM_PX * 0.6) * scale;
  const limitW = Math.max(0, maxWidth - ellW);
  let curX = 0;
  for (let i = 0; i < fullGlyphs.length; i++) {
    const g = fullGlyphs[i];
    const advW = (g ? g.adv : MSDF_BASE_EM_PX * 0.3) * scale + extraTrack;
    if (curX + advW > limitW && i > 0) break;
    if (g && g.w > 0 && chars[i] !== " ") {
      glyphs.push({
        dx: curX + g.ox * scale,
        dy: baselineOffsetY + g.oy * scale,
        w: g.w * scale,
        h: g.h * scale,
        u0: g.u0,
        v0: g.v0,
        u1: g.u1,
        v1: g.v1
      });
    }
    curX += advW;
  }
  if (ellG && ellG.w > 0) {
    glyphs.push({
      dx: curX + ellG.ox * scale,
      dy: baselineOffsetY + ellG.oy * scale,
      w: ellG.w * scale,
      h: ellG.h * scale,
      u0: ellG.u0,
      v0: ellG.v0,
      u1: ellG.u1,
      v1: ellG.v1
    });
  }
  curX += ellW;

  const res = { glyphs: glyphs, totalW: curX, pxRange: pxRange };
  studioGraphState.shapedTextCache.set(key, res);
  return res;
}

function initStudioWebGLContext(canvas) {
  if (studioGL.gl && studioGL.canvas === canvas) {
    return studioGL.gl;
  }
  const gl =
    canvas.getContext("webgl", {
      alpha: false,
      antialias: true,
      depth: false,
      stencil: false,
      preserveDrawingBuffer: false,
      powerPreference: "high-performance"
    }) || canvas.getContext("experimental-webgl");
  if (!gl) return null;

  const instExt = gl.getExtension("ANGLE_instanced_arrays");
  studioGL.instExt = instExt;

  const vsSource = `
    attribute vec2 aUnit;
    attribute vec4 aRect;
    attribute vec4 aUV;
    attribute vec4 aColor;
    uniform vec2 uViewSize;
    uniform vec3 uPanZoom;
    varying vec2 vUV;
    varying vec4 vColor;
    varying float vMode;
    varying float vPxRange;
    void main() {
      vColor = aColor;
      vMode = 0.0;
      vPxRange = 4.5;
      vec2 uvs = mix(aUV.xy, aUV.zw, aUnit);
      if (aUV.y < -0.5 && aRect.w > aRect.z) {
        uvs.x = mix(aUV.x, aUV.z, aUnit.y);
      }
      vUV = uvs;
      vec2 screenPos;
      if (aUV.x < -80000.0) {
        screenPos = aUnit * uViewSize;
        vec2 worldPos = (screenPos - uPanZoom.xy) / uPanZoom.z;
        vUV = -worldPos - vec2(200000.0);
      } else if (aUV.x < -30000.0) {
        screenPos = aRect.xy + aUnit * aRect.zw;
      } else {
        if (aUV.x >= 25.0) {
          vUV.x = uvs.x - 30.0;
          vMode = -1.0;
        } else if (aUV.x >= 15.0) {
          vUV.x = uvs.x - 20.0;
          vMode = 1.0;
          vPxRange = 3.39;
        } else if (aUV.x >= 5.0) {
          vUV.x = uvs.x - 10.0;
          vMode = 1.0;
          vPxRange = 2.80;
        }
        vec2 worldPos = aRect.xy + aUnit * aRect.zw;
        screenPos = worldPos * uPanZoom.z + uPanZoom.xy;
      }
      vec2 clip = (screenPos / uViewSize) * 2.0 - 1.0;
      gl_Position = vec4(clip.x, -clip.y, 0.0, 1.0);
    }
  `;

  const fsSource = `
    precision highp float;
    varying vec2 vUV;
    varying vec4 vColor;
    varying float vMode;
    varying float vPxRange;
    uniform sampler2D uUIAtlas;
    uniform sampler2D uFontAtlas;
    uniform vec3 uBgColor;
    uniform vec3 uDotColor;
    uniform float uGridStep;
    uniform vec3 uPanZoom;
    uniform float uDpr;

    float median(float r, float g, float b) {
      return max(min(r, g), min(max(r, g), b));
    }

    void main() {
      if (vUV.x < -80000.0) {
        vec2 worldPos = -vUV - vec2(200000.0);
        vec2 cell = fract((worldPos + vec2(0.8)) / uGridStep) * uGridStep;
        float dotMask = step(cell.x, 1.7) * step(cell.y, 1.7);
        gl_FragColor = vec4(mix(uBgColor, uDotColor, dotMask), 1.0);
        return;
      }
      if (vUV.y < -0.5) {
        float dist = vUV.x;
        float period = -vUV.y;
        if (fract(dist / period) > 0.54) discard;
        gl_FragColor = vColor;
        return;
      }
      if (vMode < -0.5) {
        gl_FragColor = texture2D(uUIAtlas, vUV) * vColor;
        return;
      }
      vec4 tex = (vMode > 0.5) ? texture2D(uFontAtlas, vUV) : texture2D(uUIAtlas, vUV);
      float sd = median(tex.r, tex.g, tex.b) - 0.5;
      float screenPxRange = max(1.5, vPxRange * uPanZoom.z * uDpr);
      float alpha = clamp(sd * screenPxRange + 0.5, 0.0, 1.0);
      if (alpha <= 0.01) discard;
      gl_FragColor = vec4(vColor.rgb, vColor.a * alpha);
    }
  `;

  const compileShader = (type, src) => {
    const sh = gl.createShader(type);
    gl.shaderSource(sh, src);
    gl.compileShader(sh);
    return sh;
  };

  const vs = compileShader(gl.VERTEX_SHADER, vsSource);
  const fs = compileShader(gl.FRAGMENT_SHADER, fsSource);
  const prog = gl.createProgram();
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.linkProgram(prog);
  gl.useProgram(prog);

  const quadVbo = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, quadVbo);
  gl.bufferData(
    gl.ARRAY_BUFFER,
    new Float32Array([0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1]),
    gl.STATIC_DRAW
  );
  const aUnit = gl.getAttribLocation(prog, "aUnit");
  gl.enableVertexAttribArray(aUnit);
  gl.vertexAttribPointer(aUnit, 2, gl.FLOAT, false, 8, 0);
  if (instExt) instExt.vertexAttribDivisorANGLE(aUnit, 0);

  const aRect = gl.getAttribLocation(prog, "aRect");
  const aUV = gl.getAttribLocation(prog, "aUV");
  const aColor = gl.getAttribLocation(prog, "aColor");

  const instVbos = [gl.createBuffer(), gl.createBuffer(), gl.createBuffer()];
  const instVboCapacities = [0, 0, 0];
  const initCapBytes = 4096 * BYTES_PER_INSTANCE;
  for (let b = 0; b < 3; b++) {
    gl.bindBuffer(gl.ARRAY_BUFFER, instVbos[b]);
    gl.bufferData(gl.ARRAY_BUFFER, initCapBytes, gl.STREAM_DRAW);
    instVboCapacities[b] = initCapBytes;
  }

  const bindInstVboAttribs = (vbo) => {
    gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
    gl.vertexAttribPointer(aRect, 4, gl.FLOAT, false, BYTES_PER_INSTANCE, 0);
    gl.vertexAttribPointer(aUV, 4, gl.FLOAT, false, BYTES_PER_INSTANCE, 16);
    gl.vertexAttribPointer(aColor, 4, gl.UNSIGNED_BYTE, true, BYTES_PER_INSTANCE, 32);
  };

  gl.enableVertexAttribArray(aRect);
  gl.enableVertexAttribArray(aUV);
  gl.enableVertexAttribArray(aColor);
  bindInstVboAttribs(instVbos[0]);

  if (instExt) {
    instExt.vertexAttribDivisorANGLE(aRect, 1);
    instExt.vertexAttribDivisorANGLE(aUV, 1);
    instExt.vertexAttribDivisorANGLE(aColor, 1);
  }

  const uiTex = gl.createTexture();
  gl.activeTexture(gl.TEXTURE0);
  gl.bindTexture(gl.TEXTURE_2D, uiTex);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  gl.texImage2D(
    gl.TEXTURE_2D,
    0,
    gl.RGBA,
    UI_ATLAS_W,
    UI_ATLAS_H,
    0,
    gl.RGBA,
    gl.UNSIGNED_BYTE,
    buildUISpriteTextureBytes()
  );

  const fontTex = gl.createTexture();
  gl.activeTexture(gl.TEXTURE1);
  gl.bindTexture(gl.TEXTURE_2D, fontTex);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  gl.texImage2D(
    gl.TEXTURE_2D,
    0,
    gl.RGBA,
    1,
    1,
    0,
    gl.RGBA,
    gl.UNSIGNED_BYTE,
    new Uint8Array([0, 0, 0, 0])
  );
  studioGL.atlasUploaded = false;
  startPreloadingMSDFAtlasImage();

  gl.enable(gl.BLEND);
  gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

  studioGL.canvas = canvas;
  studioGL.gl = gl;
  studioGL.prog = prog;
  studioGL.quadVbo = quadVbo;
  studioGL.instVbos = instVbos;
  studioGL.instVboCapacities = instVboCapacities;
  studioGL.vboRingIdx = 0;
  studioGL.instVbo = instVbos[0];
  studioGL.bindInstVboAttribs = bindInstVboAttribs;
  studioGL.hasCullBounds = false;
  studioGL.lastVpW = 0;
  studioGL.lastVpH = 0;
  studioGL.lastUniformVw = -1;
  studioGL.lastUniformVh = -1;
  studioGL.lastUniformPx = NaN;
  studioGL.lastUniformPy = NaN;
  studioGL.lastUniformZ = -1;
  studioGL.lastUniformDpr = -1;
  studioGL.lastUniformLight = null;
  studioGL.lastUniformGridStep = -1;
  studioGL.uiTex = uiTex;
  studioGL.fontTex = fontTex;
  studioGL.uViewSize = gl.getUniformLocation(prog, "uViewSize");
  studioGL.uPanZoom = gl.getUniformLocation(prog, "uPanZoom");
  studioGL.uDpr = gl.getUniformLocation(prog, "uDpr");
  studioGL.uBgColor = gl.getUniformLocation(prog, "uBgColor");
  studioGL.uDotColor = gl.getUniformLocation(prog, "uDotColor");
  studioGL.uGridStep = gl.getUniformLocation(prog, "uGridStep");
  studioGL.uUIAtlas = gl.getUniformLocation(prog, "uUIAtlas");
  studioGL.uFontAtlas = gl.getUniformLocation(prog, "uFontAtlas");
  gl.uniform1i(studioGL.uUIAtlas, 0);
  gl.uniform1i(studioGL.uFontAtlas, 1);

  studioGraphState.staticBatchDirty = true;
  studioGraphState.overlayBatchDirty = true;
  return gl;
}

function pushQuadInstance(x, y, w, h, u0, v0, u1, v1, packedColor) {
  const idx = studioGL.instCursor;
  if (idx >= maxInstancesCapacity) {
    growInstanceBuffer();
  }
  const base = idx * FLOATS_PER_INSTANCE;
  instanceFloatView[base] = x;
  instanceFloatView[base + 1] = y;
  instanceFloatView[base + 2] = w;
  instanceFloatView[base + 3] = h;
  instanceFloatView[base + 4] = u0;
  instanceFloatView[base + 5] = v0;
  instanceFloatView[base + 6] = u1;
  instanceFloatView[base + 7] = v1;
  instanceUint32View[base + 8] = packedColor;
  studioGL.instCursor = idx + 1;
}

function pushSolidQuad(x, y, w, h, packedColor) {
  const u = studioGL.uv.whiteU + 30.0;
  const v = studioGL.uv.whiteV;
  pushQuadInstance(x, y, w, h, u, v, u, v, packedColor);
}

function pushBorderedBoxQuads(x, y, w, h, borderThickness, borderColor, fillColor) {
  pushSolidQuad(x, y, w, h, borderColor);
  pushSolidQuad(
    x + borderThickness,
    y + borderThickness,
    w - borderThickness * 2,
    h - borderThickness * 2,
    fillColor
  );
}

function pushSolidBorderQuads(x, y, w, h, thickness, packedColor) {
  const t = thickness;
  pushSolidQuad(x, y, w, t, packedColor);
  pushSolidQuad(x, y + h - t, w, t, packedColor);
  pushSolidQuad(x, y + t, t, h - t * 2, packedColor);
  pushSolidQuad(x + w - t, y + t, t, h - t * 2, packedColor);
}

function computeAlignedDashSpan(len, period) {
  const n = Math.max(1, Math.round(len / period - 0.54));
  return (n + 0.54) * period;
}

function pushDashedSegmentQuad(x1, y1, x2, y2, thickness, period, packedColor) {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.hypot(dx, dy);
  if (len < 0.5) return;
  const halfT = thickness * 0.5;
  const negPeriod = -period;
  const span = computeAlignedDashSpan(len, period);
  if (Math.abs(dx) >= Math.abs(dy)) {
    const minX = Math.min(x1, x2);
    pushQuadInstance(minX, y1 - halfT, len, thickness, 0, negPeriod, span, negPeriod, packedColor);
  } else {
    const minY = Math.min(y1, y2);
    pushQuadInstance(x1 - halfT, minY, thickness, len, 0, negPeriod, span, negPeriod, packedColor);
  }
}

function pushDashedHeaderDividerQuad(x, y, w, thickness, period, packedColor) {
  const nW = Math.max(1, Math.round(w / period - 0.54));
  if (nW <= 2) {
    return;
  }
  const worldPeriodW = w / (nW + 0.54);
  const startX = x + worldPeriodW;
  const innerW = w - worldPeriodW * 2.0;
  const span = (nW - 2 + 0.54) * period;
  pushQuadInstance(startX, y, innerW, thickness, 0, -period, span, -period, packedColor);
}

function pushDashedRectQuads(x, y, w, h, thickness, period, packedColor) {
  const t = thickness;
  const negPeriod = -period;
  const spanW = computeAlignedDashSpan(w, period);
  const innerH = Math.max(1, h - t * 2);
  const spanH = computeAlignedDashSpan(innerH, period);
  pushQuadInstance(x, y, w, t, 0, negPeriod, spanW, negPeriod, packedColor);
  pushQuadInstance(x, y + h - t, w, t, 0, negPeriod, spanW, negPeriod, packedColor);
  pushQuadInstance(x, y + t, t, innerH, 0, negPeriod, spanH, negPeriod, packedColor);
  pushQuadInstance(x + w - t, y + t, t, innerH, 0, negPeriod, spanH, negPeriod, packedColor);
}

function pushHorizontalWireSegmentWithBridges(xA, xB, y, thickness, packedColor, segBridges) {
  const halfT = thickness * 0.5;
  const minX = xA < xB ? xA : xB;
  const maxX = xA > xB ? xA : xB;
  if (maxX - minX < 0.5) return;
  if (!segBridges || segBridges.length === 0) {
    pushSolidQuad(minX, y - halfT, maxX - minX, thickness, packedColor);
    return;
  }
  const gapR = 5.0;
  let curX = minX;
  for (let k = 0; k < segBridges.length; k++) {
    const bx = segBridges[k];
    if (bx - gapR <= curX + 1.0 || bx + gapR >= maxX - 1.0) continue;
    const leftLen = bx - gapR - curX;
    if (leftLen > 0.5) {
      pushSolidQuad(curX, y - halfT, leftLen, thickness, packedColor);
    }
    curX = bx + gapR;
  }
  if (maxX - curX > 0.5) {
    pushSolidQuad(curX, y - halfT, maxX - curX, thickness, packedColor);
  }
}

function pushWirePolylineQuads(pts, thickness, isDashed, packedColor, bridgesBySeg) {
  if (!pts || pts.length < 2) return;
  const halfT = thickness * 0.5;
  const lastSeg = pts.length - 2;

  for (let i = 0; i <= lastSeg; i++) {
    const p0 = pts[i];
    const p1 = pts[i + 1];
    if (isDashed) {
      pushDashedSegmentQuad(p0.x, p0.y, p1.x, p1.y, thickness, 8.0, packedColor);
      continue;
    }
    const dx = p1.x - p0.x;
    const dy = p1.y - p0.y;
    if (Math.abs(dx) >= Math.abs(dy)) {
      let xA = p0.x;
      let xB = p1.x;
      if (i > 0) {
        xA += dx >= 0 ? -halfT : halfT;
      }
      if (i < lastSeg) {
        xB += dx >= 0 ? halfT : -halfT;
      }
      pushHorizontalWireSegmentWithBridges(
        xA,
        xB,
        p0.y,
        thickness,
        packedColor,
        bridgesBySeg ? bridgesBySeg[i] : null
      );
    } else {
      const minY = p0.y < p1.y ? p0.y : p1.y;
      const maxY = p0.y > p1.y ? p0.y : p1.y;
      if (maxY - minY >= 0.5) {
        pushSolidQuad(p0.x - halfT, minY, thickness, maxY - minY, packedColor);
      }
    }
  }
}

function pushShapedTextQuads(shaped, originX, centerY, packedColor) {
  const glyphs = shaped.glyphs;
  const uBias = shaped.pxRange >= 3.2 ? 20.0 : 10.0;
  for (let i = 0; i < glyphs.length; i++) {
    const g = glyphs[i];
    pushQuadInstance(
      originX + g.dx,
      centerY + g.dy,
      g.w,
      g.h,
      g.u0 + uBias,
      g.v0,
      g.u1 + uBias,
      g.v1,
      packedColor
    );
  }
}

function pushPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable) {
  const fillSp = studioGL.uv.portFill;
  const hs = fillSp.size * 0.5;
  const portColor =
    typeof isHoveredOrActive === "number"
      ? isHoveredOrActive
      : isHoveredOrActive
        ? theme.textPrimary
        : isUnreachable
          ? theme.unreachWire
          : theme.borderStrong;
  pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, theme.bgObsidian);
  const sp = isConnected ? studioGL.uv.portHover : studioGL.uv.portRing;
  pushQuadInstance(x - hs, y - hs, sp.size, sp.size, sp.u0, sp.v0, sp.u1, sp.v1, portColor);
}

function pushSmallPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable) {
  const fillSp = studioGL.uv.smallPortFill;
  const hs = fillSp.size * 0.5;
  const portColor =
    typeof isHoveredOrActive === "number"
      ? isHoveredOrActive
      : isHoveredOrActive
        ? theme.textPrimary
        : isUnreachable
          ? theme.unreachWire
          : theme.borderStrong;
  pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, theme.bgObsidian);
  const sp = isConnected ? studioGL.uv.smallPortHover : studioGL.uv.smallPortRing;
  pushQuadInstance(x - hs, y - hs, sp.size, sp.size, sp.u0, sp.v0, sp.u1, sp.v1, portColor);
}

function isHoveredPortItemConnected(hovered) {
  if (!hovered) return false;
  if (hovered.type === "scene-out-port") {
    return Boolean(hovered.port && hovered.port.connected);
  }
  if (hovered.type === "chapter-out-port") {
    return Boolean(hovered.outPort && hovered.outPort.connected);
  }
  if (hovered.type === "wire-pin") {
    return true;
  }
  return false;
}

function compileStaticSceneQuads(theme) {
  studioGL.instCursor = 0;

  // Slot 0: Fullscreen procedural GPU dot grid quad
  pushQuadInstance(0, 0, 1, 1, -100000.0, -100000.0, -100000.0, -100000.0, theme.bgObsidian);

  const vw = studioGraphState.viewW || 1200;
  const vh = studioGraphState.viewH || 700;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;
  const visMinX = -px / z;
  const visMinY = -py / z;
  const visMaxX = (vw - px) / z;
  const visMaxY = (vh - py) / z;
  const guardX = Math.max(120, (visMaxX - visMinX) * 0.25);
  const guardY = Math.max(120, (visMaxY - visMinY) * 0.25);
  const fMinX = visMinX - guardX;
  const fMinY = visMinY - guardY;
  const fMaxX = visMaxX + guardX;
  const fMaxY = visMaxY + guardY;
  studioGL.cullMinX = fMinX;
  studioGL.cullMinY = fMinY;
  studioGL.cullMaxX = fMaxX;
  studioGL.cullMaxY = fMaxY;
  studioGL.hasCullBounds = true;

  const chapters = studioGraphState.chapters;
  const wires = studioGraphState.wires;
  const bundles = studioGraphState.bundleOverlays;
  const nodes = studioGraphState.nodes;
  const gripSp = studioGL.uv.grip;

  // 1. Chapter Group Frames (frustum-culled)
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    if (ch.hidden) continue;
    if (ch.right < fMinX || ch.left > fMaxX || ch.bottom < fMinY || ch.top > fMaxY) {
      continue;
    }
    pushSolidQuad(ch.x, ch.y, ch.w, ch.h, theme.chapterFill);
    pushSolidQuad(ch.x + 1, ch.y + 1, ch.w - 2, 27, theme.bgElevated);
    pushDashedRectQuads(ch.x, ch.y, ch.w, ch.h, 1, 9.0, theme.borderStrong);
    pushDashedHeaderDividerQuad(ch.x, ch.y + 28, ch.w, 1, 9.0, theme.borderStrong);
    pushQuadInstance(
      ch.x + 4,
      ch.y + 3,
      gripSp.w,
      gripSp.h,
      gripSp.u0 + 30.0,
      gripSp.v0,
      gripSp.u1 + 30.0,
      gripSp.v1,
      theme.textMuted
    );
    if (studioGraphState.editingChapter !== ch && ch.y + 30 >= fMinY && ch.y <= fMaxY) {
      const chMaxW = ch.w - 38;
      if (ch._shapedName !== ch.name || ch._shapedMaxW !== chMaxW || !ch._shapedTitle) {
        ch._shapedName = ch.name;
        ch._shapedMaxW = chMaxW;
        ch._shapedTitle = shapeTextGlyphs(FONT_CHAPTER, (ch.name || "").toUpperCase(), chMaxW);
      }
      pushShapedTextQuads(ch._shapedTitle, ch.x + 28, ch.y + 15, theme.textPrimary);
    }
  }

  // 2. Static Wires + Bundle Trunks + Bundle Count Badges (frustum-culled)
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.maxX < fMinX || w.minX > fMaxX || w.maxY < fMinY || w.minY > fMaxY) {
      continue;
    }
    if (w.unreachable) {
      pushWirePolylineQuads(w.pts, 2, true, theme.unreachWire, null);
    } else {
      pushWirePolylineQuads(w.pts, 2, false, theme.borderStrong, w.bridges);
    }
    if (w.pins && w.pins.length > 0) {
      for (let p = 0; p < w.pins.length; p++) {
        pushSmallPortQuads(w.pins[p].x, w.pins[p].y, true, false, theme, w.unreachable);
      }
    }
  }

  for (let i = 0; i < bundles.length; i++) {
    const b = bundles[i];
    const bMinY = b.y1 < b.y2 ? b.y1 : b.y2;
    const bMaxY = b.y1 > b.y2 ? b.y1 : b.y2;
    if (b.x2 + 12 < fMinX || b.x1 - 12 > fMaxX || bMaxY + 12 < fMinY || bMinY - 12 > fMaxY) {
      continue;
    }
    pushSolidQuad(b.x1 - 2.25, bMinY, 4.5, bMaxY - bMinY, theme.borderStrong);
    const mx = (b.x1 + b.x2) * 0.5;
    const my = (b.y1 + b.y2) * 0.5;
    pushBorderedBoxQuads(mx - 9, my - 8, 18, 16, 1.5, theme.textPrimary, theme.bgObsidian);
    const shapedNum = shapeTextGlyphs(FONT_BADGE, String(b.count), 16);
    pushShapedTextQuads(shapedNum, mx - shapedNum.totalW * 0.5, my, theme.textPrimary);
  }

  // 3. Scene Node Cards (frustum-culled)
  const branchPlusSp = studioGL.uv.branchPlus;
  const editPenSp = studioGL.uv.editPen;
  const trashSp = studioGL.uv.trashIcon;
  const footerIconBtnSize = 24;
  const isHistoryPreview = typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview();

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    if (
      nd.right + 12 < fMinX ||
      nd.left - 12 > fMaxX ||
      nd.bottom + 12 < fMinY ||
      nd.top - 12 > fMaxY
    ) {
      continue;
    }
    const isUnreach = Boolean(nd.unreachable);
    const cardBg = isUnreach ? theme.unreachCardBg : theme.bgObsidian;
    const textPri = isUnreach ? theme.unreachText : theme.textPrimary;
    const textSec = isUnreach ? theme.unreachText : theme.textSecondary;

    if (isUnreach) {
      pushSolidQuad(nd.x, nd.y, nd.w, nd.h, cardBg);
      pushDashedRectQuads(nd.x, nd.y, nd.w, nd.h, 1, 8.0, theme.unreachBorder);
    } else {
      pushBorderedBoxQuads(nd.x, nd.y, nd.w, nd.h, 1, theme.borderStrong, cardBg);
    }

    pushQuadInstance(
      nd.x + 4,
      nd.y + 6,
      gripSp.w,
      gripSp.h,
      gripSp.u0 + 30.0,
      gripSp.v0,
      gripSp.u1 + 30.0,
      gripSp.v1,
      theme.textMuted
    );

    let badgeW = 0;
    if (nd.isStart || nd.isEntry) {
      const badgeTxt = nd.isStart ? "START" : "WEJŚCIE";
      if (nd._shapedBadgeTxt !== badgeTxt || !nd._shapedBadge) {
        nd._shapedBadgeTxt = badgeTxt;
        nd._shapedBadge = shapeTextGlyphs(FONT_BADGE, badgeTxt, 64);
      }
      const shapedBadge = nd._shapedBadge;
      badgeW = Math.ceil(shapedBadge.totalW) + 10;
      const bx = nd.x + nd.w - badgeW - 8;
      const by = nd.y + 8;
      pushBorderedBoxQuads(bx, by, badgeW, 17, 1, nd.isStart ? theme.textPrimary : theme.borderStrong, theme.bgElevated);
      pushShapedTextQuads(shapedBadge, bx + 5, by + 8.5, nd.isStart ? theme.textPrimary : theme.textSecondary);
    }

    let radioReserveW = 0;
    if (nd.branchRole === "primary" || nd.branchRole === "secondary") {
      const rx = nd.x + nd.w - (badgeW > 0 ? badgeW + 8 + 14 : 16);
      const ry = nd.y + 13;
      nd.branchRadioX = rx;
      nd.branchRadioY = ry;
      radioReserveW = 16;
      const sqColor = nd.branchRole === "primary" ? theme.textPrimary : theme.borderStrong;
      pushSolidQuad(rx, ry, 8, 8, sqColor);
    }

    if (studioGraphState.editingNodeTitle !== nd) {
      const maxTitleW = nd.w - 36 - (badgeW > 0 ? badgeW + 6 : 0) - radioReserveW;
      const rawTitle = nd.name || nd.id;
      if (nd._shapedTitleTxt !== rawTitle || nd._shapedTitleMaxW !== maxTitleW || !nd._shapedTitle) {
        nd._shapedTitleTxt = rawTitle;
        nd._shapedTitleMaxW = maxTitleW;
        nd._shapedTitle = shapeTextGlyphs(FONT_TITLE, rawTitle, maxTitleW);
      }
      pushShapedTextQuads(nd._shapedTitle, nd.x + 28, nd.y + 17, textPri);
    }

    const choicesTop = nd.y + NODE_HEADER_HEIGHT;
    pushSolidQuad(nd.x + 1, choicesTop, nd.w - 2, nd.h - NODE_HEADER_HEIGHT - 1, theme.borderSubtle);

    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        const rowY = choicesTop + j * NODE_CHOICE_ROW_HEIGHT;
        pushSolidQuad(nd.x + 1, rowY + 1, nd.w - 2, NODE_CHOICE_ROW_HEIGHT - 1, theme.bgObsidian);
        if (!isHistoryPreview) {
          const cgW = 16;
          const cgH = 18;
          const cgX = nd.x + 3;
          const cgY = rowY + (NODE_CHOICE_ROW_HEIGHT - cgH) * 0.5;
          pushQuadInstance(
            cgX,
            cgY,
            cgW,
            cgH,
            gripSp.u0 + 30.0,
            gripSp.v0,
            gripSp.u1 + 30.0,
            gripSp.v1,
            theme.textMuted
          );
        }
        const isEditingThisChoice =
          studioGraphState.editingChoice &&
          studioGraphState.editingChoice.node === nd &&
          studioGraphState.editingChoice.idx === j;
        if (!isEditingThisChoice) {
          const rawLabel = pt.label || "Wybór " + (j + 1);
          const choiceMaxW = nd.w - (isHistoryPreview ? 36 : 47);
          if (pt._shapedLabelTxt !== rawLabel || pt._shapedChoiceMaxW !== choiceMaxW || !pt._shapedChoice) {
            pt._shapedLabelTxt = rawLabel;
            pt._shapedChoiceMaxW = choiceMaxW;
            pt._shapedChoice = shapeTextGlyphs(FONT_CHOICE, rawLabel, choiceMaxW);
          }
          pushShapedTextQuads(
            pt._shapedChoice,
            nd.x + (isHistoryPreview ? 10 : 21),
            rowY + NODE_CHOICE_ROW_HEIGHT * 0.5,
            textSec
          );
        }
        if (!isHistoryPreview) {
          const tx = nd.x + nd.w - 23;
          const ty = rowY + (NODE_CHOICE_ROW_HEIGHT - trashSp.size) * 0.5;
          pushQuadInstance(
            tx,
            ty,
            trashSp.size,
            trashSp.size,
            trashSp.u0,
            trashSp.v0,
            trashSp.u1,
            trashSp.v1,
            theme.textMuted
          );
        }
      }
    }

    const footerTop = nd.y + nd.h - NODE_FOOTER_HEIGHT;
    pushSolidQuad(nd.x + 1, footerTop + 1, nd.w - 2, NODE_FOOTER_HEIGHT - 2, cardBg);
    const btnY = footerTop + 8;
    if (!isHistoryPreview) {
      const btnX = nd.x + 8;
      pushBorderedBoxQuads(btnX, btnY, footerIconBtnSize, footerIconBtnSize, 1, theme.borderStrong, cardBg);
      pushQuadInstance(
        btnX + (footerIconBtnSize - branchPlusSp.size) * 0.5,
        btnY + (footerIconBtnSize - branchPlusSp.size) * 0.5,
        branchPlusSp.size,
        branchPlusSp.size,
        branchPlusSp.u0,
        branchPlusSp.v0,
        branchPlusSp.u1,
        branchPlusSp.v1,
        textPri
      );
    }
    const editX = nd.x + nd.w - 8 - footerIconBtnSize;
    pushBorderedBoxQuads(editX, btnY, footerIconBtnSize, footerIconBtnSize, 1, theme.borderStrong, cardBg);
    pushQuadInstance(
      editX + (footerIconBtnSize - editPenSp.size) * 0.5,
      btnY + (footerIconBtnSize - editPenSp.size) * 0.5,
      editPenSp.size,
      editPenSp.size,
      editPenSp.u0,
      editPenSp.v0,
      editPenSp.u1,
      editPenSp.v1,
      textPri
    );
  }

  // 4. Default Circular Ports (frustum-culled)
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    if (ch.hidden) continue;
    if (
      ch.right + 12 < fMinX ||
      ch.left - 12 > fMaxX ||
      ch.bottom + 12 < fMinY ||
      ch.top - 12 > fMaxY
    ) {
      continue;
    }
    if (!ch.isRoot) {
      pushSmallPortQuads(ch.inX, ch.inY, Boolean(ch.inConnected), false, theme, false);
    }
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      pushSmallPortQuads(op.x, op.y, Boolean(op.connected), false, theme, false);
    }
  }

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
    if (
      nd.right + 12 < fMinX ||
      nd.left - 12 > fMaxX ||
      nd.bottom + 12 < fMinY ||
      nd.top - 12 > fMaxY
    ) {
      continue;
    }
    if (nd.hasInputPort) {
      pushPortQuads(nd.x + nd.inOx, nd.y + nd.inOy, Boolean(nd.inConnected), false, theme, nd.unreachable);
    }
    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        pushPortQuads(nd.x + pt.ox, nd.y + pt.oy, Boolean(pt.connected), false, theme, nd.unreachable);
      }
    } else if (nd.implicitPort) {
      pushPortQuads(
        nd.x + nd.implicitPort.ox,
        nd.y + nd.implicitPort.oy,
        Boolean(nd.implicitPort.connected),
        false,
        theme,
        nd.unreachable
      );
    }
  }

  studioGL.staticInstCount = studioGL.instCursor;
}

function chapterMatchesTargetLower(ch, targetLower) {
  if (!ch || !targetLower) return false;
  return ch.nameLower === targetLower || ch.refLower === targetLower;
}

function collectHoveredConnectionWires(hoveredWire) {
  if (!hoveredWire) return [];
  if (hoveredWire.kind === "scene-to-scene") {
    return [hoveredWire];
  }
  const allWires = studioGraphState.wires || [];
  const result = [];
  const seen = new Set();
  const pushUnique = (w) => {
    if (w && !seen.has(w)) {
      seen.add(w);
      result.push(w);
    }
  };

  pushUnique(hoveredWire);

  if (hoveredWire.kind === "scene-to-chapter-out") {
    const fromCh = hoveredWire.fromChapterObj;
    const targetLower = hoveredWire.targetChapterLower;
    const toCh = studioGraphState.chapterByKey.get(targetLower);
    for (let i = 0; i < allWires.length; i++) {
      const w = allWires[i];
      if (
        w.kind === "chapter-to-chapter" &&
        w.fromChapterObj === fromCh &&
        (w.toChapterObj === toCh || chapterMatchesTargetLower(w.toChapterObj, targetLower))
      ) {
        pushUnique(w);
      } else if (
        w.kind === "chapter-in-to-entry" &&
        (w.toChapterObj === toCh || chapterMatchesTargetLower(w.toChapterObj, targetLower))
      ) {
        pushUnique(w);
      }
    }
  } else if (hoveredWire.kind === "chapter-to-chapter") {
    const fromCh = hoveredWire.fromChapterObj;
    const toCh = hoveredWire.toChapterObj;
    for (let i = 0; i < allWires.length; i++) {
      const w = allWires[i];
      if (
        w.kind === "scene-to-chapter-out" &&
        w.fromChapterObj === fromCh &&
        chapterMatchesTargetLower(toCh, w.targetChapterLower)
      ) {
        pushUnique(w);
      } else if (w.kind === "chapter-in-to-entry" && w.toChapterObj === toCh) {
        pushUnique(w);
      }
    }
  } else if (hoveredWire.kind === "chapter-in-to-entry") {
    const toCh = hoveredWire.toChapterObj;
    const upstreamFromChapters = new Set();
    for (let i = 0; i < allWires.length; i++) {
      const w = allWires[i];
      if (w.kind === "chapter-to-chapter" && w.toChapterObj === toCh) {
        pushUnique(w);
        if (w.fromChapterObj) upstreamFromChapters.add(w.fromChapterObj);
      }
    }
    for (let i = 0; i < allWires.length; i++) {
      const w = allWires[i];
      if (
        w.kind === "scene-to-chapter-out" &&
        upstreamFromChapters.has(w.fromChapterObj) &&
        chapterMatchesTargetLower(toCh, w.targetChapterLower)
      ) {
        pushUnique(w);
      }
    }
  }

  return result;
}

function appendDynamicOverlayQuads(theme) {
  studioGL.instCursor = studioGL.staticInstCount;

  const hovered = studioGraphState.hoveredItem;
  const gesture = activeCanvasGesture;
  const hoveredWire =
    hovered && (hovered.type === "wire" || hovered.type === "wire-pin")
      ? hovered.wire
      : gesture && gesture.type === "wire-pin-drag" && gesture.wire
        ? gesture.wire
        : null;
  const gripSp = studioGL.uv.grip;

  const renderChapterPortsOverlay = (ch) => {
    if (!ch || ch.hidden) return;
    if (!ch.isRoot) {
      pushSmallPortQuads(ch.inX, ch.inY, Boolean(ch.inConnected), false, theme, false);
    }
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      pushSmallPortQuads(op.x, op.y, Boolean(op.connected), false, theme, false);
    }
  };

  if (gesture && gesture.type === "chapter-drag" && gesture.chapter && gesture.dragConfirmed) {
    const ch = gesture.chapter;
    pushSolidBorderQuads(ch.x, ch.y, ch.w, ch.h, 1, theme.textPrimary);
    renderChapterPortsOverlay(ch);
  } else if (studioGraphState.selectedChapterKey) {
    const selCh = studioGraphState.chapterByKey.get(studioGraphState.selectedChapterKey);
    if (selCh && !selCh.hidden) {
      pushSolidBorderQuads(selCh.x, selCh.y, selCh.w, selCh.h, 1, theme.textSecondary);
      renderChapterPortsOverlay(selCh);
    }
  }

  const isNodeDragging = Boolean(gesture && gesture.type === "node-drag");
  const nodeHighlightColor = isNodeDragging ? theme.textPrimary : theme.textSecondary;
  selectedNodeIds.forEach((selId) => {
    const nd = studioGraphState.nodeById.get(selId);
    if (!nd) return;
    pushSolidBorderQuads(nd.x, nd.y, nd.w, nd.h, 1, nodeHighlightColor);
    if (nd.hasInputPort) {
      pushPortQuads(nd.x + nd.inOx, nd.y + nd.inOy, Boolean(nd.inConnected), false, theme, nd.unreachable);
    }
    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        pushPortQuads(nd.x + pt.ox, nd.y + pt.oy, Boolean(pt.connected), false, theme, nd.unreachable);
      }
    } else if (nd.implicitPort) {
      pushPortQuads(
        nd.x + nd.implicitPort.ox,
        nd.y + nd.implicitPort.oy,
        Boolean(nd.implicitPort.connected),
        false,
        theme,
        nd.unreachable
      );
    }
  });

  if (hoveredWire) {
    const chain = collectHoveredConnectionWires(hoveredWire);
    for (let i = 0; i < chain.length; i++) {
      const hw = chain[i];
      pushWirePolylineQuads(hw.pts, 2, false, theme.textSecondary, hw.bridges);
    }
    for (let i = 0; i < chain.length; i++) {
      const hw = chain[i];
      if (hw.pins && hw.pins.length > 0) {
        for (let p = 0; p < hw.pins.length; p++) {
          pushSmallPortQuads(hw.pins[p].x, hw.pins[p].y, true, theme.textSecondary, theme, false);
        }
      }
      if (hw.fromNode) {
        pushPortQuads(hw.x1, hw.y1, true, theme.textSecondary, theme, false);
      } else if (hw.fromChapterOutPort) {
        pushSmallPortQuads(hw.fromChapterOutPort.x, hw.fromChapterOutPort.y, true, theme.textSecondary, theme, false);
      } else if (hw.kind === "chapter-in-to-entry" && hw.fromChapterObj) {
        pushSmallPortQuads(hw.fromChapterObj.inX, hw.fromChapterObj.inY, Boolean(hw.fromChapterObj.inConnected), theme.textSecondary, theme, false);
      }
      if (hw.toNode) {
        pushPortQuads(hw.x2, hw.y2, true, theme.textSecondary, theme, false);
      } else if (hw.toChapterOutPort) {
        pushSmallPortQuads(hw.toChapterOutPort.x, hw.toChapterOutPort.y, true, theme.textSecondary, theme, false);
      } else if (hw.toChapterObj) {
        pushSmallPortQuads(hw.toChapterObj.inX, hw.toChapterObj.inY, true, theme.textSecondary, theme, false);
      }
    }
  }

  if (gesture && gesture.type === "wire-pin-drag" && gesture.wire && gesture.wire.pins && gesture.wire.pins[gesture.pinIdx]) {
    const activePin = gesture.wire.pins[gesture.pinIdx];
    pushSmallPortQuads(activePin.x, activePin.y, true, true, theme, false);
  }

  if (gesture && gesture.type === "choice-drag" && gesture.node && gesture.node.ports) {
    const nd = gesture.node;
    const j = gesture.currentIdx;
    if (j >= 0 && j < nd.ports.length) {
      const pt = nd.ports[j];
      const rowY = nd.y + NODE_HEADER_HEIGHT + j * NODE_CHOICE_ROW_HEIGHT;
      pushBorderedBoxQuads(nd.x + 1, rowY, nd.w - 2, NODE_CHOICE_ROW_HEIGHT, 1, theme.textPrimary, theme.bgElevated);
      const cgW = 16;
      const cgH = 18;
      pushQuadInstance(
        nd.x + 3,
        rowY + (NODE_CHOICE_ROW_HEIGHT - cgH) * 0.5,
        cgW,
        cgH,
        gripSp.u0 + 30.0,
        gripSp.v0,
        gripSp.u1 + 30.0,
        gripSp.v1,
        theme.textPrimary
      );
      const rawLabel = pt.label || "Wybór " + (j + 1);
      const choiceMaxW = nd.w - 47;
      if (pt._shapedLabelTxt !== rawLabel || pt._shapedChoiceMaxW !== choiceMaxW || !pt._shapedChoice) {
        pt._shapedLabelTxt = rawLabel;
        pt._shapedChoiceMaxW = choiceMaxW;
        pt._shapedChoice = shapeTextGlyphs(FONT_CHOICE, rawLabel, choiceMaxW);
      }
      pushShapedTextQuads(pt._shapedChoice, nd.x + 21, rowY + NODE_CHOICE_ROW_HEIGHT * 0.5, theme.textPrimary);
      const trashSp = studioGL.uv.trashIcon;
      const tx = nd.x + nd.w - 23;
      const ty = rowY + (NODE_CHOICE_ROW_HEIGHT - trashSp.size) * 0.5;
      pushQuadInstance(tx, ty, trashSp.size, trashSp.size, trashSp.u0, trashSp.v0, trashSp.u1, trashSp.v1, theme.textMuted);
      pushPortQuads(nd.x + pt.ox, nd.y + pt.oy, Boolean(pt.connected), true, theme, nd.unreachable);
    }
  }

  if (gesture && (gesture.type === "wire-connect" || gesture.type === "chapter-wire-connect")) {
    const hoveredTarget = gesture.hoveredTargetId || "";
    if (
      gesture.type === "wire-connect" &&
      hoveredTarget.indexOf("chapter:") === 0 &&
      hoveredTarget !== "chapter:__next__" &&
      gesture.midOutX !== undefined
    ) {
      const pts1 = simplifyOrthogonalPoints(
        buildWireWaypoints(
          gesture.startX,
          gesture.startY,
          gesture.midOutX,
          gesture.midOutY,
          null,
          0,
          gesture.midOutX - 8,
          gesture.midOutX + 8,
          Boolean(gesture.isBottomPort)
        )
      );
      const pts2 = simplifyOrthogonalPoints(
        buildWireWaypoints(
          gesture.midOutX,
          gesture.midOutY,
          gesture.curX,
          gesture.curY,
          gesture.midOutY,
          0,
          null,
          null,
          true
        )
      );
      pushWirePolylineQuads(pts1, 2, true, theme.textPrimary);
      pushWirePolylineQuads(pts2, 2, true, theme.textPrimary);
    } else {
      const ptsTemp = simplifyOrthogonalPoints(
        buildWireWaypoints(
          gesture.startX,
          gesture.startY,
          gesture.curX,
          gesture.curY,
          null,
          0,
          null,
          null,
          Boolean(gesture.isBottomPort)
        )
      );
      pushWirePolylineQuads(ptsTemp, 2, true, theme.textPrimary);
    }
    const startConnected =
      gesture.type === "wire-connect"
        ? Boolean(gesture.sourcePort ? gesture.sourcePort.connected : gesture.hasExistingTarget)
        : Boolean(gesture.activeChapterOutPort && gesture.activeChapterOutPort.connected);
    if (gesture.type === "chapter-wire-connect") {
      pushSmallPortQuads(gesture.startX, gesture.startY, startConnected, true, theme, false);
    } else {
      pushPortQuads(gesture.startX, gesture.startY, startConnected, true, theme, false);
    }
    if (gesture.hoveredSceneNode) {
      const tnd = gesture.hoveredSceneNode;
      pushPortQuads(tnd.x + tnd.inOx, tnd.y + tnd.inOy, Boolean(tnd.inConnected), true, theme, false);
    }
    if (gesture.hoveredChapterIn) {
      pushSmallPortQuads(
        gesture.hoveredChapterIn.inX,
        gesture.hoveredChapterIn.inY,
        Boolean(gesture.hoveredChapterIn.inConnected),
        true,
        theme,
        false
      );
    }
    if (gesture.hoveredChapterOutPort) {
      pushSmallPortQuads(
        gesture.hoveredChapterOutPort.x,
        gesture.hoveredChapterOutPort.y,
        Boolean(gesture.hoveredChapterOutPort.connected),
        true,
        theme,
        false
      );
    }
  }

  if (hovered) {
    if (hovered.type === "scene-out-port") {
      pushPortQuads(hovered.x, hovered.y, isHoveredPortItemConnected(hovered), true, theme, false);
    } else if (hovered.type === "chapter-out-port" || hovered.type === "wire-pin") {
      pushSmallPortQuads(hovered.x, hovered.y, isHoveredPortItemConnected(hovered), true, theme, false);
    } else if (hovered.type === "node-grip" && hovered.node) {
      const nd = hovered.node;
      pushQuadInstance(
        nd.x + 4,
        nd.y + 6,
        gripSp.w,
        gripSp.h,
        gripSp.u0 + 30.0,
        gripSp.v0,
        gripSp.u1 + 30.0,
        gripSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "choice-grip" && hovered.node && hovered.choiceIdx !== undefined) {
      const nd = hovered.node;
      const rowY = nd.y + NODE_HEADER_HEIGHT + hovered.choiceIdx * NODE_CHOICE_ROW_HEIGHT;
      const cgW = 16;
      const cgH = 18;
      pushQuadInstance(
        nd.x + 3,
        rowY + (NODE_CHOICE_ROW_HEIGHT - cgH) * 0.5,
        cgW,
        cgH,
        gripSp.u0 + 30.0,
        gripSp.v0,
        gripSp.u1 + 30.0,
        gripSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "chapter-grip" && hovered.chapter && !hovered.chapter.hidden) {
      const ch = hovered.chapter;
      pushQuadInstance(
        ch.x + 4,
        ch.y + 3,
        gripSp.w,
        gripSp.h,
        gripSp.u0 + 30.0,
        gripSp.v0,
        gripSp.u1 + 30.0,
        gripSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "add-choice-btn" && hovered.node) {
      const nd = hovered.node;
      const branchPlusSp = studioGL.uv.branchPlus;
      const btnX = nd.x + 8;
      const btnY = nd.y + nd.h - NODE_FOOTER_HEIGHT + 8;
      const btnS = 24;
      pushBorderedBoxQuads(btnX, btnY, btnS, btnS, 1, theme.textPrimary, theme.bgElevated);
      pushQuadInstance(
        btnX + (btnS - branchPlusSp.size) * 0.5,
        btnY + (btnS - branchPlusSp.size) * 0.5,
        branchPlusSp.size,
        branchPlusSp.size,
        branchPlusSp.u0,
        branchPlusSp.v0,
        branchPlusSp.u1,
        branchPlusSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "edit-scene-btn" && hovered.node) {
      const nd = hovered.node;
      const editPenSp = studioGL.uv.editPen;
      const btnS = 24;
      const btnX = nd.x + nd.w - 8 - btnS;
      const btnY = nd.y + nd.h - NODE_FOOTER_HEIGHT + 8;
      pushBorderedBoxQuads(btnX, btnY, btnS, btnS, 1, theme.textPrimary, theme.bgElevated);
      pushQuadInstance(
        btnX + (btnS - editPenSp.size) * 0.5,
        btnY + (btnS - editPenSp.size) * 0.5,
        editPenSp.size,
        editPenSp.size,
        editPenSp.u0,
        editPenSp.v0,
        editPenSp.u1,
        editPenSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "choice-trash-btn" && hovered.node && typeof hovered.choiceIdx === "number") {
      const nd = hovered.node;
      const trashSp = studioGL.uv.trashIcon;
      const rowY = nd.y + NODE_HEADER_HEIGHT + hovered.choiceIdx * NODE_CHOICE_ROW_HEIGHT;
      const tx = nd.x + nd.w - 23;
      const ty = rowY + (NODE_CHOICE_ROW_HEIGHT - trashSp.size) * 0.5;
      pushQuadInstance(
        tx,
        ty,
        trashSp.size,
        trashSp.size,
        trashSp.u0,
        trashSp.v0,
        trashSp.u1,
        trashSp.v1,
        theme.textPrimary
      );
    } else if (hovered.type === "branch-radio" && hovered.node) {
      const nd = hovered.node;
      const sqColor = nd.branchRole === "primary" ? theme.textPrimary : theme.textSecondary;
      pushSolidQuad(nd.branchRadioX, nd.branchRadioY, 8, 8, sqColor);
    }
  }

  if (gesture && gesture.type === "marquee-select" && gesture.moved && gesture.marqueeRect) {
    const mr = gesture.marqueeRect;
    pushQuadInstance(mr.x, mr.y, mr.w, mr.h, -50000.0, 0.0, -50000.0, 0.0, theme.marqueeFill);
    const t = 1;
    pushQuadInstance(mr.x, mr.y, mr.w, t, -50000.0, 0.0, -50000.0, 0.0, theme.textPrimary);
    pushQuadInstance(mr.x, mr.y + mr.h - t, mr.w, t, -50000.0, 0.0, -50000.0, 0.0, theme.textPrimary);
    pushQuadInstance(mr.x, mr.y, t, mr.h, -50000.0, 0.0, -50000.0, 0.0, theme.textPrimary);
    pushQuadInstance(mr.x + mr.w - t, mr.y, t, mr.h, -50000.0, 0.0, -50000.0, 0.0, theme.textPrimary);
  }

  studioGL.totalInstCount = studioGL.instCursor;
}

function renderStudioCanvas() {
  const canvas = qs("#studio-canvas");
  if (!canvas) return;
  const gl = initStudioWebGLContext(canvas);
  if (!gl) return;

  if (studioGL.atlasImgLoaded && !studioGL.atlasUploaded && studioGL.atlasImg) {
    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D, studioGL.fontTex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, studioGL.atlasImg);
    studioGL.atlasUploaded = true;
  }

  const theme = resolveStudioThemeColors();
  const vw = studioGraphState.viewW || 1200;
  const vh = studioGraphState.viewH || 700;
  const dpr = studioGraphState.canvasDpr || 2.0;
  const z = editorCanvasState.zoom || 1.0;
  const px = editorCanvasState.panX || 0;
  const py = editorCanvasState.panY || 0;

  const visMinX = -px / z;
  const visMinY = -py / z;
  const visMaxX = (vw - px) / z;
  const visMaxY = (vh - py) / z;
  if (
    !studioGraphState.staticBatchDirty &&
    (!studioGL.hasCullBounds ||
      visMinX < studioGL.cullMinX ||
      visMaxX > studioGL.cullMaxX ||
      visMinY < studioGL.cullMinY ||
      visMaxY > studioGL.cullMaxY)
  ) {
    studioGraphState.staticBatchDirty = true;
  }

  if (studioGraphState.staticBatchDirty) {
    compileStaticSceneQuads(theme);
    appendDynamicOverlayQuads(theme);
    const ringIdx = ((studioGL.vboRingIdx || 0) + 1) % 3;
    studioGL.vboRingIdx = ringIdx;
    const vbo = studioGL.instVbos[ringIdx];
    studioGL.instVbo = vbo;
    studioGL.bindInstVboAttribs(vbo);
    const activeFloats = studioGL.totalInstCount * FLOATS_PER_INSTANCE;
    const requiredBytes = (studioGL.totalInstCount + 64) * BYTES_PER_INSTANCE;
    if (studioGL.instVboCapacities[ringIdx] < requiredBytes) {
      const nextCapBytes = Math.max(requiredBytes * 2, 4096 * BYTES_PER_INSTANCE);
      gl.bufferData(gl.ARRAY_BUFFER, nextCapBytes, gl.STREAM_DRAW);
      studioGL.instVboCapacities[ringIdx] = nextCapBytes;
    }
    gl.bufferSubData(
      gl.ARRAY_BUFFER,
      0,
      instanceFloatView.subarray(0, activeFloats)
    );
    studioGraphState.staticBatchDirty = false;
    studioGraphState.overlayBatchDirty = false;
  } else if (studioGraphState.overlayBatchDirty) {
    appendDynamicOverlayQuads(theme);
    const dynCount = studioGL.totalInstCount - studioGL.staticInstCount;
    if (dynCount > 0) {
      const ringIdx = studioGL.vboRingIdx || 0;
      const vbo = studioGL.instVbos[ringIdx];
      const startFloat = studioGL.staticInstCount * FLOATS_PER_INSTANCE;
      const endFloat = studioGL.totalInstCount * FLOATS_PER_INSTANCE;
      const requiredBytes = studioGL.totalInstCount * BYTES_PER_INSTANCE;
      gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
      if (studioGL.instVboCapacities[ringIdx] < requiredBytes) {
        const nextCapBytes = Math.max(requiredBytes * 2, 4096 * BYTES_PER_INSTANCE);
        gl.bufferData(gl.ARRAY_BUFFER, nextCapBytes, gl.STREAM_DRAW);
        studioGL.instVboCapacities[ringIdx] = nextCapBytes;
        gl.bufferSubData(
          gl.ARRAY_BUFFER,
          0,
          instanceFloatView.subarray(0, endFloat)
        );
      } else {
        gl.bufferSubData(
          gl.ARRAY_BUFFER,
          studioGL.staticInstCount * BYTES_PER_INSTANCE,
          instanceFloatView.subarray(startFloat, endFloat)
        );
      }
    }
    studioGraphState.overlayBatchDirty = false;
  }

  if (studioGL.lastVpW !== canvas.width || studioGL.lastVpH !== canvas.height) {
    gl.viewport(0, 0, canvas.width, canvas.height);
    studioGL.lastVpW = canvas.width;
    studioGL.lastVpH = canvas.height;
  }
  if (studioGL.lastUniformVw !== vw || studioGL.lastUniformVh !== vh) {
    gl.uniform2f(studioGL.uViewSize, vw, vh);
    studioGL.lastUniformVw = vw;
    studioGL.lastUniformVh = vh;
  }
  if (studioGL.lastUniformPx !== px || studioGL.lastUniformPy !== py || studioGL.lastUniformZ !== z) {
    gl.uniform3f(studioGL.uPanZoom, px, py, z);
    studioGL.lastUniformPx = px;
    studioGL.lastUniformPy = py;
    studioGL.lastUniformZ = z;
  }
  if (studioGL.lastUniformDpr !== dpr) {
    gl.uniform1f(studioGL.uDpr, dpr);
    studioGL.lastUniformDpr = dpr;
  }
  if (studioGL.lastUniformLight !== theme.isLight) {
    gl.uniform3f(studioGL.uBgColor, theme.bgObsidianRgb[0], theme.bgObsidianRgb[1], theme.bgObsidianRgb[2]);
    gl.uniform3f(studioGL.uDotColor, theme.gridDotRgb[0], theme.gridDotRgb[1], theme.gridDotRgb[2]);
    studioGL.lastUniformLight = theme.isLight;
  }
  const gridStep = z < 0.45 ? EDITOR_GRID_SIZE * 2 : EDITOR_GRID_SIZE;
  if (studioGL.lastUniformGridStep !== gridStep) {
    gl.uniform1f(studioGL.uGridStep, gridStep);
    studioGL.lastUniformGridStep = gridStep;
  }

  // 1 SINGLE INSTANCED GPU DRAW CALL
  if (studioGL.instExt) {
    studioGL.instExt.drawArraysInstancedANGLE(gl.TRIANGLES, 0, 6, studioGL.totalInstCount);
  }
  studioGraphState.lastDrawCalls = 1;
}
