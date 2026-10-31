"use strict";

const EDITOR_GRID_SIZE = 24;
const NODE_CARD_WIDTH = 192;
const NODE_HEADER_HEIGHT = 34;
const NODE_CHOICE_ROW_HEIGHT = 26;
const NODE_FOOTER_HEIGHT = 40;
const NODE_BASE_HEIGHT = 74;
const PORT_RADIUS = 6.5;
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
  bundleOverlays: [],
  hoveredItem: null,
  editingChapter: null,
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
const MAX_INSTANCES = 16384;

const instanceArrayBuffer = new ArrayBuffer(MAX_INSTANCES * BYTES_PER_INSTANCE);
const instanceFloatView = new Float32Array(instanceArrayBuffer);
const instanceUint32View = new Uint32Array(instanceArrayBuffer);

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
    grip: { u0: 124 / UI_ATLAS_W, v0: 2 / UI_ATLAS_H, u1: 144 / UI_ATLAS_W, v1: 24 / UI_ATLAS_H, w: 20, h: 22 },
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

  const instVbo = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, instVbo);
  gl.bufferData(gl.ARRAY_BUFFER, instanceArrayBuffer.byteLength, gl.DYNAMIC_DRAW);

  const aRect = gl.getAttribLocation(prog, "aRect");
  const aUV = gl.getAttribLocation(prog, "aUV");
  const aColor = gl.getAttribLocation(prog, "aColor");
  gl.enableVertexAttribArray(aRect);
  gl.vertexAttribPointer(aRect, 4, gl.FLOAT, false, BYTES_PER_INSTANCE, 0);
  gl.enableVertexAttribArray(aUV);
  gl.vertexAttribPointer(aUV, 4, gl.FLOAT, false, BYTES_PER_INSTANCE, 16);
  gl.enableVertexAttribArray(aColor);
  gl.vertexAttribPointer(aColor, 4, gl.UNSIGNED_BYTE, true, BYTES_PER_INSTANCE, 32);

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
  studioGL.instVbo = instVbo;
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
  if (idx >= MAX_INSTANCES) return;
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

function pushWirePolylineQuads(pts, thickness, isDashed, packedColor) {
  if (!pts || pts.length < 2) return;
  const halfT = thickness * 0.5;
  const maxR = 8;

  if (pts.length === 2 || isDashed) {
    for (let i = 0; i < pts.length - 1; i++) {
      const p0 = pts[i];
      const p1 = pts[i + 1];
      if (isDashed) {
        pushDashedSegmentQuad(p0.x, p0.y, p1.x, p1.y, thickness, 8.0, packedColor);
      } else if (Math.abs(p1.x - p0.x) >= Math.abs(p1.y - p0.y)) {
        const minX = Math.min(p0.x, p1.x);
        pushSolidQuad(minX, p0.y - halfT, Math.abs(p1.x - p0.x), thickness, packedColor);
      } else {
        const minY = Math.min(p0.y, p1.y);
        pushSolidQuad(p0.x - halfT, minY, thickness, Math.abs(p1.y - p0.y), packedColor);
      }
    }
    return;
  }

  let curStartX = pts[0].x;
  let curStartY = pts[0].y;

  for (let i = 1; i < pts.length - 1; i++) {
    const p0 = pts[i - 1];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const dx1 = p1.x - p0.x;
    const dy1 = p1.y - p0.y;
    const len1 = Math.hypot(dx1, dy1);
    const dx2 = p2.x - p1.x;
    const dy2 = p2.y - p1.y;
    const len2 = Math.hypot(dx2, dy2);
    const r = Math.min(maxR, len1 * 0.5, len2 * 0.5);

    if (r < 3) {
      if (Math.abs(dx1) >= Math.abs(dy1)) {
        pushSolidQuad(Math.min(curStartX, p1.x), p1.y - halfT, Math.abs(p1.x - curStartX), thickness, packedColor);
      } else {
        pushSolidQuad(p1.x - halfT, Math.min(curStartY, p1.y), thickness, Math.abs(p1.y - curStartY), packedColor);
      }
      curStartX = p1.x;
      curStartY = p1.y;
      continue;
    }

    const ux1 = Math.sign(dx1);
    const uy1 = Math.sign(dy1);
    const ux2 = Math.sign(dx2);
    const uy2 = Math.sign(dy2);
    const segEndX = p1.x - ux1 * r;
    const segEndY = p1.y - uy1 * r;

    if (Math.abs(dx1) >= Math.abs(dy1)) {
      pushSolidQuad(Math.min(curStartX, segEndX), p1.y - halfT, Math.abs(segEndX - curStartX), thickness, packedColor);
    } else {
      pushSolidQuad(p1.x - halfT, Math.min(curStartY, segEndY), thickness, Math.abs(segEndY - curStartY), packedColor);
    }

    const nextStartX = p1.x + ux2 * r;
    const nextStartY = p1.y + uy2 * r;
    const cx = ux1 !== 0 ? segEndX : nextStartX;
    const cy = uy1 !== 0 ? segEndY : nextStartY;
    const quadX = p1.x < cx ? p1.x - halfT : cx;
    const quadY = p1.y < cy ? p1.y - halfT : cy;
    const quadW = (p1.x < cx ? cx : p1.x + halfT) - quadX;
    const quadH = (p1.y < cy ? cy : p1.y + halfT) - quadY;

    let cornerUV = studioGL.uv.cornerTL;
    if (p1.x >= cx && p1.y < cy) cornerUV = studioGL.uv.cornerTR;
    else if (p1.x < cx && p1.y >= cy) cornerUV = studioGL.uv.cornerBL;
    else if (p1.x >= cx && p1.y >= cy) cornerUV = studioGL.uv.cornerBR;

    pushQuadInstance(quadX, quadY, quadW, quadH, cornerUV.u0, cornerUV.v0, cornerUV.u1, cornerUV.v1, packedColor);

    curStartX = nextStartX;
    curStartY = nextStartY;
  }

  const last = pts[pts.length - 1];
  if (Math.abs(last.x - curStartX) >= Math.abs(last.y - curStartY)) {
    pushSolidQuad(Math.min(curStartX, last.x), last.y - halfT, Math.abs(last.x - curStartX), thickness, packedColor);
  } else {
    pushSolidQuad(last.x - halfT, Math.min(curStartY, last.y), thickness, Math.abs(last.y - curStartY), packedColor);
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

function isHoveredPortItemConnected(hovered) {
  if (!hovered) return false;
  if (hovered.type === "scene-out-port") {
    return Boolean(hovered.port && hovered.port.connected);
  }
  if (hovered.type === "chapter-out-port") {
    return Boolean(hovered.outPort && hovered.outPort.connected);
  }
  return false;
}

function compileStaticSceneQuads(theme) {
  studioGL.instCursor = 0;

  // Slot 0: Fullscreen procedural GPU dot grid quad
  pushQuadInstance(0, 0, 1, 1, -100000.0, -100000.0, -100000.0, -100000.0, theme.bgObsidian);

  const chapters = studioGraphState.chapters;
  const wires = studioGraphState.wires;
  const bundles = studioGraphState.bundleOverlays;
  const nodes = studioGraphState.nodes;
  const gripSp = studioGL.uv.grip;

  // 1. Chapter Group Frames
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    if (ch.hidden) continue;
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
    if (studioGraphState.editingChapter !== ch) {
      const shapedCh = shapeTextGlyphs(FONT_CHAPTER, (ch.name || "").toUpperCase(), ch.w - 38);
      pushShapedTextQuads(shapedCh, ch.x + 28, ch.y + 15, theme.textPrimary);
    }
  }

  // 2. Static Wires + Bundle Trunks + Bundle Count Badges
  for (let i = 0; i < wires.length; i++) {
    const w = wires[i];
    if (w.unreachable) {
      pushWirePolylineQuads(w.pts, 2, true, theme.unreachWire);
    } else {
      pushWirePolylineQuads(w.pts, 2, false, theme.borderStrong);
    }
  }

  for (let i = 0; i < bundles.length; i++) {
    const b = bundles[i];
    pushSolidQuad(b.x1 - 2.25, Math.min(b.y1, b.y2), 4.5, Math.abs(b.y2 - b.y1), theme.borderStrong);
    const mx = (b.x1 + b.x2) * 0.5;
    const my = (b.y1 + b.y2) * 0.5;
    pushBorderedBoxQuads(mx - 9, my - 8, 18, 16, 1.5, theme.textPrimary, theme.bgObsidian);
    const shapedNum = shapeTextGlyphs(FONT_BADGE, String(b.count), 16);
    pushShapedTextQuads(shapedNum, mx - shapedNum.totalW * 0.5, my, theme.textPrimary);
  }

  // 3. Scene Node Cards
  const addChoiceShaped = shapeTextGlyphs(FONT_BTN, "+ WYBÓR", 80);
  const addChoiceBtnW = Math.ceil(addChoiceShaped.totalW) + 20;
  const addChoiceBtnH = 24;

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
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
      const shapedBadge = shapeTextGlyphs(FONT_BADGE, badgeTxt, 64);
      badgeW = Math.ceil(shapedBadge.totalW) + 10;
      const bx = nd.x + nd.w - badgeW - 8;
      const by = nd.y + 8;
      pushBorderedBoxQuads(bx, by, badgeW, 17, 1, nd.isStart ? theme.textPrimary : theme.borderStrong, theme.bgElevated);
      pushShapedTextQuads(shapedBadge, bx + 5, by + 8.5, nd.isStart ? theme.textPrimary : theme.textSecondary);
    }

    const maxTitleW = nd.w - 36 - (badgeW > 0 ? badgeW + 6 : 0);
    const shapedTitle = shapeTextGlyphs(FONT_TITLE, nd.name || nd.id, maxTitleW);
    pushShapedTextQuads(shapedTitle, nd.x + 28, nd.y + 17, textPri);

    const choicesTop = nd.y + NODE_HEADER_HEIGHT;
    pushSolidQuad(nd.x + 1, choicesTop, nd.w - 2, nd.h - NODE_HEADER_HEIGHT - 1, theme.borderSubtle);

    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        const rowY = choicesTop + j * NODE_CHOICE_ROW_HEIGHT;
        pushSolidQuad(nd.x + 1, rowY + 1, nd.w - 2, NODE_CHOICE_ROW_HEIGHT - 1, theme.bgObsidian);
        const shapedChoice = shapeTextGlyphs(FONT_CHOICE, pt.label || "Wybór " + (j + 1), nd.w - 24);
        pushShapedTextQuads(shapedChoice, nd.x + 10, rowY + NODE_CHOICE_ROW_HEIGHT * 0.5, textSec);
      }
    }

    const footerTop = nd.y + nd.h - NODE_FOOTER_HEIGHT;
    pushSolidQuad(nd.x + 1, footerTop + 1, nd.w - 2, NODE_FOOTER_HEIGHT - 2, cardBg);
    const btnX = nd.x + 8;
    const btnY = footerTop + 8;
    pushBorderedBoxQuads(btnX, btnY, addChoiceBtnW, addChoiceBtnH, 1, theme.borderStrong, cardBg);
    pushShapedTextQuads(addChoiceShaped, btnX + (addChoiceBtnW - addChoiceShaped.totalW) * 0.5, btnY + addChoiceBtnH * 0.5, textPri);
  }

  // 4. Default Circular Ports
  for (let i = 0; i < chapters.length; i++) {
    const ch = chapters[i];
    if (ch.hidden) continue;
    if (!ch.isRoot) {
      pushPortQuads(ch.inX, ch.inY, Boolean(ch.inConnected), false, theme, false);
    }
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      pushPortQuads(op.x, op.y, Boolean(op.connected), false, theme, false);
    }
  }

  for (let i = 0; i < nodes.length; i++) {
    const nd = nodes[i];
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
  const hoveredWire = hovered && hovered.type === "wire" ? hovered.wire : null;
  const gesture = activeCanvasGesture;
  const gripSp = studioGL.uv.grip;

  const renderChapterPortsOverlay = (ch) => {
    if (!ch || ch.hidden) return;
    if (!ch.isRoot) {
      pushPortQuads(ch.inX, ch.inY, Boolean(ch.inConnected), false, theme, false);
    }
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      pushPortQuads(op.x, op.y, Boolean(op.connected), false, theme, false);
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
      pushWirePolylineQuads(hw.pts, 2, false, theme.textSecondary);
    }
    for (let i = 0; i < chain.length; i++) {
      const hw = chain[i];
      if (hw.fromNode) {
        pushPortQuads(hw.x1, hw.y1, true, theme.textSecondary, theme, false);
      } else if (hw.fromChapterOutPort) {
        pushPortQuads(hw.fromChapterOutPort.x, hw.fromChapterOutPort.y, true, theme.textSecondary, theme, false);
      } else if (hw.kind === "chapter-in-to-entry" && hw.fromChapterObj) {
        pushPortQuads(hw.fromChapterObj.inX, hw.fromChapterObj.inY, Boolean(hw.fromChapterObj.inConnected), theme.textSecondary, theme, false);
      }
      if (hw.toNode) {
        pushPortQuads(hw.x2, hw.y2, true, theme.textSecondary, theme, false);
      } else if (hw.toChapterOutPort) {
        pushPortQuads(hw.toChapterOutPort.x, hw.toChapterOutPort.y, true, theme.textSecondary, theme, false);
      } else if (hw.toChapterObj) {
        pushPortQuads(hw.toChapterObj.inX, hw.toChapterObj.inY, true, theme.textSecondary, theme, false);
      }
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
    pushPortQuads(gesture.startX, gesture.startY, startConnected, true, theme, false);
    if (gesture.hoveredSceneNode) {
      const tnd = gesture.hoveredSceneNode;
      pushPortQuads(tnd.x + tnd.inOx, tnd.y + tnd.inOy, Boolean(tnd.inConnected), true, theme, false);
    }
    if (gesture.hoveredChapterIn) {
      pushPortQuads(
        gesture.hoveredChapterIn.inX,
        gesture.hoveredChapterIn.inY,
        Boolean(gesture.hoveredChapterIn.inConnected),
        true,
        theme,
        false
      );
    }
    if (gesture.hoveredChapterOutPort) {
      pushPortQuads(
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
    if (
      hovered.type === "scene-out-port" ||
      hovered.type === "chapter-out-port"
    ) {
      pushPortQuads(hovered.x, hovered.y, isHoveredPortItemConnected(hovered), true, theme, false);
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
      const addChoiceShaped = shapeTextGlyphs(FONT_BTN, "+ WYBÓR", 80);
      const btnX = nd.x + 8;
      const btnY = nd.y + nd.h - NODE_FOOTER_HEIGHT + 8;
      const btnW = Math.ceil(addChoiceShaped.totalW) + 20;
      const btnH = 24;
      pushBorderedBoxQuads(btnX, btnY, btnW, btnH, 1, theme.textPrimary, theme.bgElevated);
      pushShapedTextQuads(addChoiceShaped, btnX + (btnW - addChoiceShaped.totalW) * 0.5, btnY + btnH * 0.5, theme.textPrimary);
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

  if (studioGraphState.staticBatchDirty) {
    compileStaticSceneQuads(theme);
    appendDynamicOverlayQuads(theme);
    gl.bindBuffer(gl.ARRAY_BUFFER, studioGL.instVbo);
    gl.bufferSubData(
      gl.ARRAY_BUFFER,
      0,
      instanceFloatView.subarray(0, studioGL.totalInstCount * FLOATS_PER_INSTANCE)
    );
    studioGraphState.staticBatchDirty = false;
    studioGraphState.overlayBatchDirty = false;
  } else if (studioGraphState.overlayBatchDirty) {
    appendDynamicOverlayQuads(theme);
    const dynCount = studioGL.totalInstCount - studioGL.staticInstCount;
    if (dynCount > 0) {
      const startFloat = studioGL.staticInstCount * FLOATS_PER_INSTANCE;
      const endFloat = studioGL.totalInstCount * FLOATS_PER_INSTANCE;
      gl.bindBuffer(gl.ARRAY_BUFFER, studioGL.instVbo);
      gl.bufferSubData(
        gl.ARRAY_BUFFER,
        studioGL.staticInstCount * BYTES_PER_INSTANCE,
        instanceFloatView.subarray(startFloat, endFloat)
      );
    }
    studioGraphState.overlayBatchDirty = false;
  }

  gl.viewport(0, 0, canvas.width, canvas.height);
  gl.useProgram(studioGL.prog);
  gl.uniform2f(studioGL.uViewSize, vw, vh);
  gl.uniform3f(studioGL.uPanZoom, px, py, z);
  gl.uniform1f(studioGL.uDpr, dpr);
  gl.uniform3f(studioGL.uBgColor, theme.bgObsidianRgb[0], theme.bgObsidianRgb[1], theme.bgObsidianRgb[2]);
  gl.uniform3f(studioGL.uDotColor, theme.gridDotRgb[0], theme.gridDotRgb[1], theme.gridDotRgb[2]);
  gl.uniform1f(studioGL.uGridStep, z < 0.45 ? EDITOR_GRID_SIZE * 2 : EDITOR_GRID_SIZE);

  // 1 SINGLE INSTANCED GPU DRAW CALL (0 bytes uploaded on Pan/Zoom frames!)
  if (studioGL.instExt) {
    studioGL.instExt.drawArraysInstancedANGLE(gl.TRIANGLES, 0, 6, studioGL.totalInstCount);
  }
  studioGraphState.lastDrawCalls = 1;
}
