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

// Texture 0: Dedicated 256x128 UI Sprite Atlas (solid white, SDF circle/diamond ports, 90° wire corners, 2x3 grip)
const UI_ATLAS_W = 256;
const UI_ATLAS_H = 128;

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
  collectorBars: [],
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
    diamondPortFill: { u0: 16 / UI_ATLAS_W, v0: 66 / UI_ATLAS_H, u1: 48 / UI_ATLAS_W, v1: 98 / UI_ATLAS_H, size: 24 },
    diamondPortRing: { u0: 52 / UI_ATLAS_W, v0: 66 / UI_ATLAS_H, u1: 84 / UI_ATLAS_W, v1: 98 / UI_ATLAS_H, size: 24 },
    diamondPortHover: { u0: 88 / UI_ATLAS_W, v0: 66 / UI_ATLAS_H, u1: 120 / UI_ATLAS_W, v1: 98 / UI_ATLAS_H, size: 24 },
    smallPortFill: { u0: 16 / UI_ATLAS_W, v0: 36 / UI_ATLAS_H, u1: 40 / UI_ATLAS_W, v1: 60 / UI_ATLAS_H, size: 16 },
    smallPortRing: { u0: 52 / UI_ATLAS_W, v0: 36 / UI_ATLAS_H, u1: 76 / UI_ATLAS_W, v1: 60 / UI_ATLAS_H, size: 16 },
    smallPortHover: { u0: 88 / UI_ATLAS_W, v0: 36 / UI_ATLAS_H, u1: 112 / UI_ATLAS_W, v1: 60 / UI_ATLAS_H, size: 16 },
    chevronH: { u0: 188 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 204 / UI_ATLAS_W, v1: 18 / UI_ATLAS_H, size: 10 },
    chevronV: { u0: 204 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 220 / UI_ATLAS_W, v1: 18 / UI_ATLAS_H, size: 10 },
    grip: { u0: 124 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 144 / UI_ATLAS_W, v1: 24 / UI_ATLAS_H, w: 20, h: 22 },
    branchPlus: { u0: 120 / UI_ATLAS_W, v0: 34 / UI_ATLAS_H, u1: 148 / UI_ATLAS_W, v1: 62 / UI_ATLAS_H, size: 15 },
    editPen: { u0: 152 / UI_ATLAS_W, v0: 34 / UI_ATLAS_H, u1: 180 / UI_ATLAS_W, v1: 62 / UI_ATLAS_H, size: 15 },
    trashIcon: { u0: 184 / UI_ATLAS_W, v0: 34 / UI_ATLAS_H, u1: 212 / UI_ATLAS_W, v1: 62 / UI_ATLAS_H, size: 15 },
    cornerTL: { u0: 152 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 168 / UI_ATLAS_W, v1: 18 / UI_ATLAS_H },
    cornerTR: { u0: 168 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 184 / UI_ATLAS_W, v1: 18 / UI_ATLAS_H },
    cornerBL: { u0: 152 / UI_ATLAS_W, v0: 18 / UI_ATLAS_H, u1: 168 / UI_ATLAS_W, v1: 34 / UI_ATLAS_H },
    cornerBR: { u0: 168 / UI_ATLAS_W, v0: 18 / UI_ATLAS_H, u1: 184 / UI_ATLAS_W, v1: 34 / UI_ATLAS_H },
    pinArrowH: { u0: 224 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 252 / UI_ATLAS_W, v1: 30 / UI_ATLAS_H, size: 16 },
    pinArrowV: { u0: 224 / UI_ATLAS_W, v0: 34 / UI_ATLAS_H, u1: 252 / UI_ATLAS_W, v1: 62 / UI_ATLAS_H, size: 16 }
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

      const dr = (Math.abs(wx) + Math.abs(wy)) * 0.78;
      const bDFill = distToByte(PORT_RADIUS - dr);
      setPixel(16 + x, 66 + y, bDFill, bDFill, bDFill, bDFill);

      const bDRing = distToByte(1.0 - Math.abs(dr - PORT_RADIUS));
      setPixel(52 + x, 66 + y, bDRing, bDRing, bDRing, bDRing);

      const bDHov = distToByte(PORT_RADIUS + 1.0 - dr);
      setPixel(88 + x, 66 + y, bDHov, bDHov, bDHov, bDHov);

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

  // 4. Analytic SDF Vector Icons (branchPlus at 120,34; editPen at 152,34; trashIcon at 184,34) & Chevrons
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
  // Open 4px directional chevrons: chevronH at (188,2,16x16), chevronV at (204,2,16x16)
  const chScale = 16.0 / 10.0;
  for (let y = 0; y < 16; y++) {
    for (let x = 0; x < 16; x++) {
      const wx = (x + 0.5 - 8.0) / chScale;
      const wy = (y + 0.5 - 8.0) / chScale;
      const dH = Math.min(
        ptSegDist(wx, wy, -2.0, -3.0, 1.5, 0.0),
        ptSegDist(wx, wy, -2.0, 3.0, 1.5, 0.0)
      );
      const bH = distToByte(0.85 - dH);
      setPixel(188 + x, 2 + y, bH, bH, bH, bH);
      const dV = Math.min(
        ptSegDist(wx, wy, -3.0, -2.0, 0.0, 1.5),
        ptSegDist(wx, wy, 3.0, -2.0, 0.0, 1.5)
      );
      const bV = distToByte(0.85 - dV);
      setPixel(204 + x, 2 + y, bV, bV, bV, bV);
    }
  }
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
      [5, 6, 6, 19],
      [6, 19, 7.5, 20.5],
      [7.5, 20.5, 16.5, 20.5],
      [16.5, 20.5, 18, 19],
      [18, 19, 19, 6],
      [10, 11, 10, 17],
      [14, 11, 14, 17]
    ],
    0.95
  );

  // 5. Analytic SDF Directional Arrowheads for User Wire Control Pins (pinArrowH at 224,2; pinArrowV at 224,34)
  const pinScale = 28.0 / 16.0;
  const xBase = -3.85;
  const xTip = 4.65;
  const yHalf = 4.25;
  const dxEdge = xTip - xBase;
  const edgeLen = Math.hypot(dxEdge, yHalf);
  const nx = yHalf / edgeLen;
  const ny = dxEdge / edgeLen;
  const sdPinArrow = (along, perp) => {
    const ay = Math.abs(perp);
    const dBase = along - xBase;
    const dSlant = -((along - xTip) * nx + ay * ny);
    const dCore =
      dBase >= 0.0 && dSlant >= 0.0
        ? Math.min(dBase, dSlant)
        : -Math.min(
            ptSegDist(along, ay, xBase, 0.0, xBase, yHalf),
            ptSegDist(along, ay, xBase, yHalf, xTip, 0.0)
          );
    return dCore + 0.35;
  };
  for (let y = 0; y < 28; y++) {
    for (let x = 0; x < 28; x++) {
      const wx = (x + 0.5 - 14.0) / pinScale;
      const wy = (y + 0.5 - 14.0) / pinScale;
      const bH = distToByte(sdPinArrow(wx, wy));
      setPixel(224 + x, 2 + y, bH, bH, bH, bH);
      const bV = distToByte(sdPinArrow(wy, wx));
      setPixel(224 + x, 34 + y, bV, bV, bV, bV);
    }
  }

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
        gridDotRgb: [0.875, 0.875, 0.865],
        bgObsidian: packRGBA(1.0, 1.0, 1.0, 1.0),
        bgElevated: packRGBA(0.957, 0.957, 0.957, 1.0),
        borderSubtle: packRGBA(0.886, 0.886, 0.886, 1.0),
        borderStrong: packRGBA(0.549, 0.549, 0.549, 1.0),
        wireDefault: packRGBA(0.549, 0.549, 0.549, 1.0),
        wirePalette: [
          packRGBA(0.31, 0.447, 0.588, 1.0),
          packRGBA(0.239, 0.51, 0.451, 1.0),
          packRGBA(0.612, 0.463, 0.22, 1.0),
          packRGBA(0.62, 0.314, 0.282, 1.0),
          packRGBA(0.451, 0.345, 0.565, 1.0),
          packRGBA(0.392, 0.49, 0.235, 1.0),
          packRGBA(0.569, 0.314, 0.439, 1.0),
          packRGBA(0.235, 0.471, 0.557, 1.0)
        ],
        textPrimary: packRGBA(0.0, 0.0, 0.0, 1.0),
        textSecondary: packRGBA(0.2, 0.2, 0.2, 1.0),
        textMuted: packRGBA(0.4, 0.4, 0.4, 1.0),
        accentDanger: packRGBA(0.788, 0.188, 0.173, 1.0),
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
        gridDotRgb: [0.118, 0.118, 0.118],
        bgObsidian: packRGBA(0.0, 0.0, 0.0, 1.0),
        bgElevated: packRGBA(0.047, 0.047, 0.047, 1.0),
        borderSubtle: packRGBA(0.141, 0.141, 0.141, 1.0),
        borderStrong: packRGBA(0.431, 0.431, 0.431, 1.0),
        wireDefault: packRGBA(0.431, 0.431, 0.431, 1.0),
        wirePalette: [
          packRGBA(0.42, 0.549, 0.682, 1.0),
          packRGBA(0.361, 0.62, 0.561, 1.0),
          packRGBA(0.71, 0.573, 0.341, 1.0),
          packRGBA(0.702, 0.42, 0.392, 1.0),
          packRGBA(0.553, 0.455, 0.659, 1.0),
          packRGBA(0.518, 0.612, 0.361, 1.0),
          packRGBA(0.678, 0.431, 0.549, 1.0),
          packRGBA(0.361, 0.596, 0.678, 1.0)
        ],
        textPrimary: packRGBA(1.0, 1.0, 1.0, 1.0),
        textSecondary: packRGBA(0.784, 0.784, 0.784, 1.0),
        textMuted: packRGBA(0.58, 0.58, 0.58, 1.0),
        accentDanger: packRGBA(0.878, 0.322, 0.322, 1.0),
        chapterFill: packRGBA(0.5, 0.5, 0.5, 0.03),
        marqueeFill: packRGBA(0.62, 0.62, 0.62, 0.09),
        unreachCardBg: packRGBA(0.047, 0.047, 0.047, 0.55),
        unreachBorder: packRGBA(0.18, 0.18, 0.18, 0.55),
        unreachText: packRGBA(0.5, 0.5, 0.5, 0.55),
        unreachWire: packRGBA(0.22, 0.22, 0.22, 0.55)
      };
  return studioGraphState.themeColors;
}

function resolveWireColor(colorIdx, theme) {
  if (typeof colorIdx === "number" && colorIdx >= 0 && theme.wirePalette) {
    return theme.wirePalette[colorIdx % theme.wirePalette.length];
  }
  return theme.wireDefault || theme.borderStrong;
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

function pushWireChevronQuad(x, y, dir, packedColor) {
  const isHoriz = dir === "right" || dir === "left";
  const sp = isHoriz ? studioGL.uv.chevronH : studioGL.uv.chevronV;
  const hs = sp.size * 0.5;
  const u0 = dir === "left" ? sp.u1 : sp.u0;
  const u1 = dir === "left" ? sp.u0 : sp.u1;
  const v0 = dir === "up" ? sp.v1 : sp.v0;
  const v1 = dir === "up" ? sp.v0 : sp.v1;
  pushQuadInstance(x - hs, y - hs, sp.size, sp.size, u0, v0, u1, v1, packedColor);
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
  const gapR = 2.7;
  let curX = minX;
  for (let k = 0; k < segBridges.length; k++) {
    const bx = segBridges[k];
    if (bx - gapR < minX + 1.0 || bx + gapR > maxX - 1.0) continue;
    const leftLen = bx - gapR - curX;
    if (leftLen > 0.5) {
      pushSolidQuad(curX, y - halfT, leftLen, thickness, packedColor);
    }
    if (bx + gapR > curX) {
      curX = bx + gapR;
    }
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
      const segBridges = bridgesBySeg ? bridgesBySeg[i] : null;
      pushHorizontalWireSegmentWithBridges(
        xA,
        xB,
        p0.y,
        thickness,
        packedColor,
        segBridges
      );
      const lenX = Math.abs(dx);
      if (lenX >= 72) {
        const mx = (p0.x + p1.x) * 0.5;
        let nearGap = false;
        if (segBridges) {
          for (let g = 0; g < segBridges.length; g++) {
            if (Math.abs(segBridges[g] - mx) < 10) {
              nearGap = true;
              break;
            }
          }
        }
        if (!nearGap) {
          pushWireChevronQuad(mx, p0.y, dx >= 0 ? "right" : "left", packedColor);
        }
      }
    } else {
      const minY = p0.y < p1.y ? p0.y : p1.y;
      const maxY = p0.y > p1.y ? p0.y : p1.y;
      const lenY = maxY - minY;
      if (lenY >= 0.5) {
        pushSolidQuad(p0.x - halfT, minY, thickness, lenY, packedColor);
      }
      if ((i === lastSeg && lenY >= 18) || lenY >= 72) {
        pushWireChevronQuad(p0.x, (p0.y + p1.y) * 0.5, dy >= 0 ? "down" : "up", packedColor);
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

function pushDiamondPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable) {
  const fillSp = studioGL.uv.diamondPortFill;
  const hs = fillSp.size * 0.5;
  const portColor =
    typeof isHoveredOrActive === "number"
      ? isHoveredOrActive
      : isHoveredOrActive
        ? theme.textPrimary
        : isUnreachable
          ? theme.unreachWire
          : theme.wireDefault;
  pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, theme.bgObsidian);
  const sp = isConnected ? studioGL.uv.diamondPortHover : studioGL.uv.diamondPortRing;
  pushQuadInstance(x - hs, y - hs, sp.size, sp.size, sp.u0, sp.v0, sp.u1, sp.v1, portColor);
}

function pushPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable) {
  if (arguments[6]) {
    pushDiamondPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable);
    return;
  }
  const fillSp = studioGL.uv.portFill;
  const hs = fillSp.size * 0.5;
  const portColor =
    typeof isHoveredOrActive === "number"
      ? isHoveredOrActive
      : isHoveredOrActive
        ? theme.textPrimary
        : isUnreachable
          ? theme.unreachWire
          : theme.wireDefault;
  pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, theme.bgObsidian);
  const sp = isConnected ? studioGL.uv.portHover : studioGL.uv.portRing;
  pushQuadInstance(x - hs, y - hs, sp.size, sp.size, sp.u0, sp.v0, sp.u1, sp.v1, portColor);
}

function pushInputSocketQuads(x, y, isHoveredOrActive, theme, isUnreachable, colorIdx) {
  const fillSp = studioGL.uv.portFill;
  const hs = fillSp.size * 0.5;
  const portColor =
    typeof isHoveredOrActive === "number"
      ? isHoveredOrActive
      : isHoveredOrActive
        ? theme.textPrimary
        : isUnreachable
          ? theme.unreachWire
          : resolveWireColor(colorIdx, theme);
  pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, theme.bgObsidian);
  const ringSp = studioGL.uv.portRing;
  pushQuadInstance(x - hs, y - hs, ringSp.size, ringSp.size, ringSp.u0, ringSp.v0, ringSp.u1, ringSp.v1, portColor);
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
          : theme.wireDefault;
  pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, theme.bgObsidian);
  const sp = isConnected ? studioGL.uv.smallPortHover : studioGL.uv.smallPortRing;
  pushQuadInstance(x - hs, y - hs, sp.size, sp.size, sp.u0, sp.v0, sp.u1, sp.v1, portColor);
}

function pushSmallInputSocketQuads(x, y, isHoveredOrActive, theme, isUnreachable, colorIdx) {
  const fillSp = studioGL.uv.smallPortFill;
  const hs = fillSp.size * 0.5;
  const portColor =
    typeof isHoveredOrActive === "number"
      ? isHoveredOrActive
      : isHoveredOrActive
        ? theme.textPrimary
        : isUnreachable
          ? theme.unreachWire
          : resolveWireColor(colorIdx, theme);
  pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, theme.bgObsidian);
  const ringSp = studioGL.uv.smallPortRing;
  pushQuadInstance(x - hs, y - hs, ringSp.size, ringSp.size, ringSp.u0, ringSp.v0, ringSp.u1, ringSp.v1, portColor);
}

function resolveWirePinDirection(wire, pin) {
  const pts = wire && (wire.pts && wire.pts.length >= 2 ? wire.pts : wire.basePts);
  if (!pts || pts.length < 2 || !pin) return "down";
  const px = pin.x;
  const py = pin.y;
  let bestSeg = 0;
  let bestDistSq = Infinity;
  let bestLenSq = 0;
  for (let s = 0; s < pts.length - 1; s++) {
    const p0 = pts[s];
    const p1 = pts[s + 1];
    const dx = p1.x - p0.x;
    const dy = p1.y - p0.y;
    const lenSq = dx * dx + dy * dy;
    if (lenSq < 1.0) continue;
    let t = ((px - p0.x) * dx + (py - p0.y) * dy) / lenSq;
    if (t < 0.0) t = 0.0;
    else if (t > 1.0) t = 1.0;
    const ex = px - (p0.x + t * dx);
    const ey = py - (p0.y + t * dy);
    const dSq = ex * ex + ey * ey;
    if (
      dSq < bestDistSq - 0.5 ||
      (dSq <= bestDistSq + 0.5 && (lenSq >= 64.0 || lenSq > bestLenSq))
    ) {
      bestDistSq = dSq;
      bestSeg = s;
      bestLenSq = lenSq;
    }
  }
  const a = pts[bestSeg];
  const b = pts[bestSeg + 1];
  const sdx = b.x - a.x;
  const sdy = b.y - a.y;
  if (Math.abs(sdx) >= Math.abs(sdy)) {
    return sdx >= 0 ? "right" : "left";
  }
  return sdy >= 0 ? "down" : "up";
}

function computeWirePinSegmentBar(wire, pin) {
  const pts = wire && (wire.pts && wire.pts.length >= 2 ? wire.pts : wire.basePts);
  if (!pts || pts.length < 2 || !pin) return null;
  let bestSeg = -1;
  if (typeof pin.segIdx === "number" && pin.segIdx >= 0 && pin.segIdx < pts.length - 1) {
    bestSeg = pin.segIdx;
  } else {
    const px = pin.x;
    const py = pin.y;
    let bestDistSq = Infinity;
    let bestLenSq = 0;
    for (let s = 0; s < pts.length - 1; s++) {
      const p0 = pts[s];
      const p1 = pts[s + 1];
      const dx = p1.x - p0.x;
      const dy = p1.y - p0.y;
      const lenSq = dx * dx + dy * dy;
      if (lenSq < 1.0) continue;
      let t = ((px - p0.x) * dx + (py - p0.y) * dy) / lenSq;
      if (t < 0.0) t = 0.0;
      else if (t > 1.0) t = 1.0;
      const ex = px - (p0.x + t * dx);
      const ey = py - (p0.y + t * dy);
      const dSq = ex * ex + ey * ey;
      if (
        dSq < bestDistSq - 0.5 ||
        (dSq <= bestDistSq + 0.5 && (lenSq >= 64.0 || lenSq > bestLenSq))
      ) {
        bestDistSq = dSq;
        bestSeg = s;
        bestLenSq = lenSq;
      }
    }
  }
  if (bestSeg < 0 || bestSeg >= pts.length - 1) return null;
  const a = pts[bestSeg];
  const b = pts[bestSeg + 1];
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const isHoriz = Math.abs(dx) >= Math.abs(dy);
  const segLen = isHoriz ? Math.abs(dx) : Math.abs(dy);
  if (segLen < 4) return null;
  // Max length = 3 grid segments (3 * 24px = 72px), leaving 8px corner clearance on each end when possible
  const maxBarLen = 72;
  const availLen = segLen > 20 ? segLen - 16 : Math.max(6, segLen - 4);
  const barLen = Math.min(maxBarLen, availLen);
  const cx = (a.x + b.x) * 0.5;
  const cy = (a.y + b.y) * 0.5;
  if (isHoriz) {
    return {
      isHoriz: true,
      segIdx: bestSeg,
      x1: cx - barLen * 0.5,
      y1: a.y,
      x2: cx + barLen * 0.5,
      y2: a.y,
      x: cx - barLen * 0.5,
      y: a.y,
      w: barLen,
      cx: cx,
      cy: a.y
    };
  }
  return {
    isHoriz: false,
    segIdx: bestSeg,
    x1: a.x,
    y1: cy - barLen * 0.5,
    x2: a.x,
    y2: cy + barLen * 0.5,
    x: a.x,
    y: cy - barLen * 0.5,
    h: barLen,
    cx: a.x,
    cy: cy
  };
}

function pushWirePinArrowQuads(wire, pinIdx, isHoveredOrActive, theme, isUnreachable) {
  if (!wire || !wire.pins || !wire.pins[pinIdx]) return;
  const pin = wire.pins[pinIdx];
  const bar = computeWirePinSegmentBar(wire, pin);
  if (!bar) return;
  const isHighlighted = isHoveredOrActive === true;
  const thick = isHighlighted ? 6 : 5;
  const halfT = thick * 0.5;
  const pinColor =
    typeof isHoveredOrActive === "number"
      ? isHoveredOrActive
      : isHighlighted
        ? theme.textPrimary
        : isUnreachable
          ? theme.unreachWire
          : resolveWireColor(wire.colorIdx, theme);
  if (bar.isHoriz) {
    pushSolidQuad(bar.x, bar.y - halfT, bar.w, thick, pinColor);
  } else {
    pushSolidQuad(bar.x - halfT, bar.y, thick, bar.h, pinColor);
  }
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
