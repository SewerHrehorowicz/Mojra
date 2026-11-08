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

const CARD = {
  w: 192,
  halfW: 96,
  headerH: 34,
  choiceH: 26,
  footerH: 40,
  baseH: 74,
  minChapterH: 120,
  btnSize: 24,
  pad: 8
};

const PAD = {
  x: 36,
  top: 52,
  bottom: 44,
  w: 72,
  h: 96
};

const GAP = {
  grid: 24,
  chapter: 24,
  node: 48
};

const SPATIAL = {
  shift: 8,
  minObstacles: 12
};

const UV = {
  grid: -100000.0,
  gridTest: -80000.0,
  gridOffset: 200000.0,
  screen: -50000.0,
  screenTest: -30000.0,
  solid: 30.0,
  fontLg: 20.0,
  fontSm: 10.0,
  dashDuty: 0.54
};

function inRect(px, py, x, y, w, h) {
  return px >= x && px <= x + w && py >= y && py <= y + h;
}

function choiceRowY(idx) {
  return CARD.headerH + idx * CARD.choiceH;
}

function toSpatialCell(coord) {
  return coord >> SPATIAL.shift;
}

// Texture 0: Dedicated 256x128 UI Sprite Atlas (/studio-ui.png)
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
  collectorBars: [],
  hoveredItem: null,
  editingChapter: null,
  editingNodeTitle: null,
  editingChoice: null,
  selectedChapterKey: "",
  searchQuery: "",
  searchMatchSet: null,
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
  if (studioGL.instVboCapacities) {
    studioGL.instVboCapacities.fill(0);
  }
}

function uiAtlasBox(x0, y0, x1, y1, extra) {
  return Object.assign(
    {
      u0: x0 / UI_ATLAS_W,
      v0: y0 / UI_ATLAS_H,
      u1: x1 / UI_ATLAS_W,
      v1: y1 / UI_ATLAS_H
    },
    extra
  );
}

const studioGL = {
  canvas: null,
  gl: null,
  instExt: null,
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
  uiAtlasImg: null,
  uiAtlasImgLoaded: false,
  uiAtlasUploaded: false,
  atlasImg: null,
  atlasImgLoaded: false,
  atlasUploaded: false,
  metricsParsed: false,
  glyphMaps: [new Map(), new Map(), new Map()],
  uv: {
    whiteU: 4 / UI_ATLAS_W,
    whiteV: 4 / UI_ATLAS_H,
    portFill: uiAtlasBox(16, 2, 48, 34, { size: 24 }),
    portRing: uiAtlasBox(52, 2, 84, 34, { size: 24 }),
    portHover: uiAtlasBox(88, 2, 120, 34, { size: 24 }),
    diamondPortFill: uiAtlasBox(16, 66, 48, 98, { size: 24 }),
    diamondPortRing: uiAtlasBox(52, 66, 84, 98, { size: 24 }),
    diamondPortHover: uiAtlasBox(88, 66, 120, 98, { size: 24 }),
    smallPortFill: uiAtlasBox(16, 36, 40, 60, { size: 16 }),
    smallPortRing: uiAtlasBox(52, 36, 76, 60, { size: 16 }),
    smallPortHover: uiAtlasBox(88, 36, 112, 60, { size: 16 }),
    chevronH: uiAtlasBox(188, 2, 204, 18, { size: 10 }),
    chevronV: uiAtlasBox(204, 2, 220, 18, { size: 10 }),
    grip: uiAtlasBox(124, 2, 144, 24, { w: 20, h: 22 }),
    branchPlus: uiAtlasBox(120, 34, 148, 62, { size: 15 }),
    editPen: uiAtlasBox(152, 34, 180, 62, { size: 15 }),
    trashIcon: uiAtlasBox(184, 34, 212, 62, { size: 15 }),
    cornerTL: uiAtlasBox(152, 2, 168, 18),
    cornerTR: uiAtlasBox(168, 2, 184, 18),
    cornerBL: uiAtlasBox(152, 18, 168, 34),
    cornerBR: uiAtlasBox(168, 18, 184, 34),
    pinArrowH: uiAtlasBox(224, 2, 252, 30, { size: 16 }),
    pinArrowV: uiAtlasBox(224, 34, 252, 62, { size: 16 })
  }
};

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
    const adv = parseFloat(p[10]);
    map.set(ch, {
      u0: parseFloat(p[2]),
      v0: parseFloat(p[3]),
      u1: parseFloat(p[4]),
      v1: parseFloat(p[5]),
      w: parseFloat(p[6]),
      h: parseFloat(p[7]),
      ox: parseFloat(p[8]),
      oy: parseFloat(p[9]),
      adv: adv,
      advanceWidth: adv
    });
  }
}

function startPreloadingMSDFAtlasImage() {
  if (typeof Image === "undefined") return;
  if (!studioGL.uiAtlasImg) {
    const uiImg = new Image();
    studioGL.uiAtlasImg = uiImg;
    uiImg.onload = () => {
      studioGL.uiAtlasImgLoaded = true;
      studioGL.uiAtlasUploaded = false;
      if (qs("#studio-canvas")) {
        renderStudioCanvas();
      }
    };
    uiImg.src = "/studio-ui.png";
  }
  if (!studioGL.atlasImg) {
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
    totalW += (g ? g.advanceWidth : MSDF_BASE_EM_PX * 0.3) * scale + extraTrack;
  }

  const glyphs = [];
  const pushPlacedGlyph = (g, curX) => {
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
  };

  if (totalW <= maxWidth) {
    let curX = 0;
    for (let i = 0; i < fullGlyphs.length; i++) {
      const g = fullGlyphs[i];
      if (g && g.w > 0 && chars[i] !== " ") {
        pushPlacedGlyph(g, curX);
      }
      curX += (g ? g.advanceWidth : MSDF_BASE_EM_PX * 0.3) * scale + extraTrack;
    }
    const res = { glyphs: glyphs, totalW: totalW, pxRange: pxRange };
    if (studioGraphState.shapedTextCache.size > 4000) {
      studioGraphState.shapedTextCache.clear();
    }
    studioGraphState.shapedTextCache.set(key, res);
    return res;
  }

  const ellipsisGlyph = lookupMSDFGlyph(style.ttfId, "…");
  const ellipsisWidth = (ellipsisGlyph ? ellipsisGlyph.advanceWidth : MSDF_BASE_EM_PX * 0.6) * scale;
  const limitW = Math.max(0, maxWidth - ellipsisWidth);
  let curX = 0;
  for (let i = 0; i < fullGlyphs.length; i++) {
    const g = fullGlyphs[i];
    const advW = (g ? g.advanceWidth : MSDF_BASE_EM_PX * 0.3) * scale + extraTrack;
    if (curX + advW > limitW && i > 0) break;
    if (g && g.w > 0 && chars[i] !== " ") {
      pushPlacedGlyph(g, curX);
    }
    curX += advW;
  }
  if (ellipsisGlyph && ellipsisGlyph.w > 0) {
    pushPlacedGlyph(ellipsisGlyph, curX);
  }
  curX += ellipsisWidth;

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
      if (aUV.x < ${UV.gridTest.toFixed(1)}) {
        screenPos = aUnit * uViewSize;
        vec2 worldPos = (screenPos - uPanZoom.xy) / uPanZoom.z;
        vUV = -worldPos - vec2(${UV.gridOffset.toFixed(1)});
      } else if (aUV.x < ${UV.screenTest.toFixed(1)}) {
        screenPos = aRect.xy + aUnit * aRect.zw;
      } else {
        if (aUV.x >= 25.0) {
          vUV.x = uvs.x - ${UV.solid.toFixed(1)};
          vMode = -1.0;
        } else if (aUV.x >= 15.0) {
          vUV.x = uvs.x - ${UV.fontLg.toFixed(1)};
          vMode = 1.0;
          vPxRange = 3.39;
        } else if (aUV.x >= 5.0) {
          vUV.x = uvs.x - ${UV.fontSm.toFixed(1)};
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
      if (vUV.x < ${UV.gridTest.toFixed(1)}) {
        vec2 worldPos = -vUV - vec2(${UV.gridOffset.toFixed(1)});
        vec2 cell = fract((worldPos + vec2(0.8)) / uGridStep) * uGridStep;
        float dotMask = step(cell.x, 1.7) * step(cell.y, 1.7);
        gl_FragColor = vec4(mix(uBgColor, uDotColor, dotMask), 1.0);
        return;
      }
      if (vUV.y < -0.5) {
        float dist = vUV.x;
        float period = -vUV.y;
        if (fract(dist / period) > ${UV.dashDuty}) discard;
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

  const createPlaceholderTexture = (unit, pixelBytes) => {
    const tex = gl.createTexture();
    gl.activeTexture(unit);
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, pixelBytes);
    return tex;
  };

  const uiTex = createPlaceholderTexture(gl.TEXTURE0, new Uint8Array([255, 255, 255, 255]));
  const fontTex = createPlaceholderTexture(gl.TEXTURE1, new Uint8Array([0, 0, 0, 0]));
  studioGL.uiAtlasUploaded = false;
  studioGL.atlasUploaded = false;
  startPreloadingMSDFAtlasImage();

  gl.enable(gl.BLEND);
  gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

  studioGL.canvas = canvas;
  studioGL.gl = gl;
  studioGL.instVbos = instVbos;
  studioGL.instVboCapacities = instVboCapacities;
  studioGL.vboRingIdx = 0;
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
  const u = studioGL.uv.whiteU + UV.solid;
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
  const n = Math.max(1, Math.round(len / period - UV.dashDuty));
  return (n + UV.dashDuty) * period;
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
  const nW = Math.max(1, Math.round(w / period - UV.dashDuty));
  if (nW <= 2) {
    return;
  }
  const worldPeriodW = w / (nW + UV.dashDuty);
  const startX = x + worldPeriodW;
  const innerW = w - worldPeriodW * 2.0;
  const span = (nW - 2 + UV.dashDuty) * period;
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
      if (lenX >= PAD.w) {
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
      if ((i === lastSeg && lenY >= 18) || lenY >= PAD.w) {
        pushWireChevronQuad(p0.x, (p0.y + p1.y) * 0.5, dy >= 0 ? "down" : "up", packedColor);
      }
    }
  }
}

function pushShapedTextQuads(shaped, originX, centerY, packedColor) {
  const glyphs = shaped.glyphs;
  const uBias = shaped.pxRange >= 3.2 ? UV.fontLg : UV.fontSm;
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

function resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, fallbackColor) {
  if (typeof isHoveredOrActive === "number") return isHoveredOrActive;
  if (isHoveredOrActive) return theme.textPrimary;
  if (isUnreachable) return theme.unreachWire;
  return fallbackColor;
}

function pushTwoLayerSpritePort(x, y, fillSp, ringSp, bgColor, portColor) {
  const hs = fillSp.size * 0.5;
  pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, bgColor);
  pushQuadInstance(x - hs, y - hs, ringSp.size, ringSp.size, ringSp.u0, ringSp.v0, ringSp.u1, ringSp.v1, portColor);
}

function pushDiamondPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable) {
  const portColor = resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, theme.wireDefault);
  const ringSp = isConnected ? studioGL.uv.diamondPortHover : studioGL.uv.diamondPortRing;
  pushTwoLayerSpritePort(x, y, studioGL.uv.diamondPortFill, ringSp, theme.bgObsidian, portColor);
}

function pushPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable) {
  if (arguments[6]) {
    pushDiamondPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable);
    return;
  }
  const portColor = resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, theme.wireDefault);
  const sp = isConnected ? studioGL.uv.portHover : studioGL.uv.portRing;
  pushTwoLayerSpritePort(x, y, studioGL.uv.portFill, sp, theme.bgObsidian, portColor);
}

function pushInputSocketQuads(x, y, isHoveredOrActive, theme, isUnreachable, colorIdx) {
  const portColor = resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, resolveWireColor(colorIdx, theme));
  pushTwoLayerSpritePort(x, y, studioGL.uv.portFill, studioGL.uv.portRing, theme.bgObsidian, portColor);
}

function pushSmallPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable) {
  const portColor = resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, theme.wireDefault);
  const ringSp = isConnected ? studioGL.uv.smallPortHover : studioGL.uv.smallPortRing;
  pushTwoLayerSpritePort(x, y, studioGL.uv.smallPortFill, ringSp, theme.bgObsidian, portColor);
}

function pushSmallInputSocketQuads(x, y, isHoveredOrActive, theme, isUnreachable, colorIdx) {
  const portColor = resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, resolveWireColor(colorIdx, theme));
  pushTwoLayerSpritePort(x, y, studioGL.uv.smallPortFill, studioGL.uv.smallPortRing, theme.bgObsidian, portColor);
}

function findClosestPolylineSegment(pts, px, py) {
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
    const t = Math.max(0.0, Math.min(1.0, ((px - p0.x) * dx + (py - p0.y) * dy) / lenSq));
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
  return bestSeg;
}

function computeWirePinSegmentBar(wire, pin) {
  const pts = wire && (wire.pts && wire.pts.length >= 2 ? wire.pts : wire.basePts);
  if (!pts || pts.length < 2 || !pin) return null;
  const bestSeg =
    typeof pin.segIdx === "number" && pin.segIdx >= 0 && pin.segIdx < pts.length - 1
      ? pin.segIdx
      : findClosestPolylineSegment(pts, pin.x, pin.y);
  if (bestSeg < 0 || bestSeg >= pts.length - 1) return null;
  const a = pts[bestSeg];
  const b = pts[bestSeg + 1];
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const isHoriz = Math.abs(dx) >= Math.abs(dy);
  const segLen = isHoriz ? Math.abs(dx) : Math.abs(dy);
  if (segLen < 4) return null;
  const maxBarLen = PAD.w;
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
  const pinColor = resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, resolveWireColor(wire.colorIdx, theme));
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
