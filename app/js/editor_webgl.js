"use strict";

const EditorWebgl = {
  FLOATS_PER_INSTANCE: 9,
  BYTES_PER_INSTANCE: 36,
  maxInstancesCapacity: 32768,
  instanceArrayBuffer: new ArrayBuffer(32768 * 36),
  instanceFloatView: null,
  instanceUint32View: null,

  studioGL: {
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
      whiteU: 4 / 256,
      whiteV: 4 / 128,
      portFill: { u0: 16 / 256, v0: 2 / 128, u1: 48 / 256, v1: 34 / 128, size: 24 },
      portRing: { u0: 52 / 256, v0: 2 / 128, u1: 84 / 256, v1: 34 / 128, size: 24 },
      portHover: { u0: 88 / 256, v0: 2 / 128, u1: 120 / 256, v1: 34 / 128, size: 24 },
      diamondPortFill: { u0: 16 / 256, v0: 66 / 128, u1: 48 / 256, v1: 98 / 128, size: 24 },
      diamondPortRing: { u0: 52 / 256, v0: 66 / 128, u1: 84 / 256, v1: 98 / 128, size: 24 },
      diamondPortHover: { u0: 88 / 256, v0: 66 / 128, u1: 120 / 256, v1: 98 / 128, size: 24 },
      smallPortFill: { u0: 16 / 256, v0: 36 / 128, u1: 40 / 256, v1: 60 / 128, size: 16 },
      smallPortRing: { u0: 52 / 256, v0: 36 / 128, u1: 76 / 256, v1: 60 / 128, size: 16 },
      smallPortHover: { u0: 88 / 256, v0: 36 / 128, u1: 112 / 256, v1: 60 / 128, size: 16 },
      chevronH: { u0: 188 / 256, v0: 2 / 128, u1: 204 / 256, v1: 18 / 128, size: 10 },
      chevronV: { u0: 204 / 256, v0: 2 / 128, u1: 220 / 256, v1: 18 / 128, size: 10 },
      grip: { u0: 124 / 256, v0: 2 / 128, u1: 144 / 256, v1: 24 / 128, w: 20, h: 22 },
      branchPlus: { u0: 120 / 256, v0: 34 / 128, u1: 148 / 256, v1: 62 / 128, size: 15 },
      editPen: { u0: 152 / 256, v0: 34 / 128, u1: 180 / 256, v1: 62 / 128, size: 15 },
      trashIcon: { u0: 184 / 256, v0: 34 / 128, u1: 212 / 256, v1: 62 / 128, size: 15 },
      cornerTL: { u0: 152 / 256, v0: 2 / 128, u1: 168 / 256, v1: 18 / 128 },
      cornerTR: { u0: 168 / 256, v0: 2 / 128, u1: 184 / 256, v1: 18 / 128 },
      cornerBL: { u0: 152 / 256, v0: 18 / 128, u1: 168 / 256, v1: 34 / 128 },
      cornerBR: { u0: 168 / 256, v0: 18 / 128, u1: 184 / 256, v1: 34 / 128 },
      pinArrowH: { u0: 224 / 256, v0: 2 / 128, u1: 252 / 256, v1: 30 / 128, size: 16 },
      pinArrowV: { u0: 224 / 256, v0: 34 / 128, u1: 252 / 256, v1: 62 / 128, size: 16 }
    }
  },

  ensureViews() {
    if (!this.instanceFloatView) {
      this.instanceFloatView = new Float32Array(this.instanceArrayBuffer);
      this.instanceUint32View = new Uint32Array(this.instanceArrayBuffer);
    }
  },

  growInstanceBuffer() {
    this.ensureViews();
    const nextCap = this.maxInstancesCapacity * 2;
    const nextBuf = new ArrayBuffer(nextCap * this.BYTES_PER_INSTANCE);
    const nextFloat = new Float32Array(nextBuf);
    const nextUint = new Uint32Array(nextBuf);
    nextFloat.set(this.instanceFloatView);
    this.maxInstancesCapacity = nextCap;
    this.instanceArrayBuffer = nextBuf;
    this.instanceFloatView = nextFloat;
    this.instanceUint32View = nextUint;
    if (this.studioGL.instVboCapacities) {
      this.studioGL.instVboCapacities.fill(0);
    }
  },

  ensureMetricsParsed() {
    if (this.studioGL.metricsParsed) return;
    this.studioGL.metricsParsed = true;
    const raw = typeof MsdfMetrics !== "undefined" ? MsdfMetrics.data || "" : "";
    if (!raw) return;
    const lines = raw.split("\n");
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      if (!line) continue;
      const p = line.split("|");
      if (p.length < 11) continue;
      const fontId = parseInt(p[0], 10) || 0;
      const ch = p[1];
      const map = this.studioGL.glyphMaps[fontId];
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
  },

  startPreloadingAtlas() {
    if (typeof Image === "undefined") return;
    const glState = this.studioGL;
    if (!glState.uiAtlasImg) {
      const uiImg = new Image();
      glState.uiAtlasImg = uiImg;
      uiImg.onload = () => {
        glState.uiAtlasImgLoaded = true;
        glState.uiAtlasUploaded = false;
        if (document.querySelector("#studio-canvas")) EditorWebglScene.render();
      };
      uiImg.src = "/studio-ui.png";
    }
    if (!glState.atlasImg) {
      const img = new Image();
      glState.atlasImg = img;
      img.onload = () => {
        glState.atlasImgLoaded = true;
        glState.atlasUploaded = false;
        if (document.querySelector("#studio-canvas")) EditorWebglScene.render();
      };
      img.src = "/studio-atlas.png";
    }
  },

  packRGBA(r, g, b, a) {
    return (
      ((Math.round(a * 255) & 255) << 24) |
      ((Math.round(b * 255) & 255) << 16) |
      ((Math.round(g * 255) & 255) << 8) |
      (Math.round(r * 255) & 255)
    ) >>> 0;
  },

  resolveThemeColors() {
    const gState = CanvasModel.studioGraphState;
    if (gState.themeColors) return gState.themeColors;
    const isLight = document.body && document.body.getAttribute("data-theme") === "light";
    const p = this.packRGBA;
    gState.themeColors = isLight
      ? {
          isLight: true,
          bgObsidianRgb: [1.0, 1.0, 1.0],
          gridDotRgb: [0.875, 0.875, 0.865],
          bgObsidian: p(1.0, 1.0, 1.0, 1.0),
          bgElevated: p(0.957, 0.957, 0.957, 1.0),
          borderSubtle: p(0.886, 0.886, 0.886, 1.0),
          borderStrong: p(0.549, 0.549, 0.549, 1.0),
          wireDefault: p(0.549, 0.549, 0.549, 1.0),
          wirePalette: [
            p(0.31, 0.447, 0.588, 1.0), p(0.239, 0.51, 0.451, 1.0), p(0.612, 0.463, 0.22, 1.0), p(0.62, 0.314, 0.282, 1.0),
            p(0.451, 0.345, 0.565, 1.0), p(0.392, 0.49, 0.235, 1.0), p(0.569, 0.314, 0.439, 1.0), p(0.235, 0.471, 0.557, 1.0)
          ],
          textPrimary: p(0.0, 0.0, 0.0, 1.0),
          textSecondary: p(0.2, 0.2, 0.2, 1.0),
          textMuted: p(0.4, 0.4, 0.4, 1.0),
          accentDanger: p(0.788, 0.188, 0.173, 1.0),
          chapterFill: p(0.0, 0.0, 0.0, 0.018),
          marqueeFill: p(0.0, 0.0, 0.0, 0.07),
          ghostFill: p(0.0, 0.0, 0.0, 0.045),
          ghostValidBorder: p(0.0, 0.0, 0.0, 0.85),
          ghostInvalidBorder: p(0.788, 0.188, 0.173, 0.95),
          collisionCellFill: p(0.86, 0.18, 0.18, 0.26),
          collisionCellBorder: p(0.788, 0.188, 0.173, 0.62),
          unreachCardBg: p(0.957, 0.957, 0.957, 0.55),
          unreachBorder: p(0.85, 0.85, 0.85, 0.55),
          unreachText: p(0.45, 0.45, 0.45, 0.55),
          unreachWire: p(0.78, 0.78, 0.78, 0.55)
        }
      : {
          isLight: false,
          bgObsidianRgb: [0.0, 0.0, 0.0],
          gridDotRgb: [0.118, 0.118, 0.118],
          bgObsidian: p(0.0, 0.0, 0.0, 1.0),
          bgElevated: p(0.047, 0.047, 0.047, 1.0),
          borderSubtle: p(0.141, 0.141, 0.141, 1.0),
          borderStrong: p(0.431, 0.431, 0.431, 1.0),
          wireDefault: p(0.431, 0.431, 0.431, 1.0),
          wirePalette: [
            p(0.42, 0.549, 0.682, 1.0), p(0.361, 0.62, 0.561, 1.0), p(0.71, 0.573, 0.341, 1.0), p(0.702, 0.42, 0.392, 1.0),
            p(0.553, 0.455, 0.659, 1.0), p(0.518, 0.612, 0.361, 1.0), p(0.678, 0.431, 0.549, 1.0), p(0.361, 0.596, 0.678, 1.0)
          ],
          textPrimary: p(1.0, 1.0, 1.0, 1.0),
          textSecondary: p(0.784, 0.784, 0.784, 1.0),
          textMuted: p(0.58, 0.58, 0.58, 1.0),
          accentDanger: p(0.878, 0.322, 0.322, 1.0),
          chapterFill: p(0.5, 0.5, 0.5, 0.03),
          marqueeFill: p(0.62, 0.62, 0.62, 0.09),
          ghostFill: p(1.0, 1.0, 1.0, 0.055),
          ghostValidBorder: p(1.0, 1.0, 1.0, 0.85),
          ghostInvalidBorder: p(0.92, 0.32, 0.32, 0.95),
          collisionCellFill: p(0.92, 0.24, 0.24, 0.28),
          collisionCellBorder: p(0.92, 0.32, 0.32, 0.62),
          unreachCardBg: p(0.047, 0.047, 0.047, 0.55),
          unreachBorder: p(0.18, 0.18, 0.18, 0.55),
          unreachText: p(0.5, 0.5, 0.5, 0.55),
          unreachWire: p(0.22, 0.22, 0.22, 0.55)
        };
    return gState.themeColors;
  },

  resolveWireColor(colorIdx, theme) {
    if (typeof colorIdx === "number" && colorIdx >= 0 && theme.wirePalette) {
      return theme.wirePalette[colorIdx % theme.wirePalette.length];
    }
    return theme.wireDefault || theme.borderStrong;
  },

  lookupGlyph(ttfId, ch) {
    this.ensureMetricsParsed();
    const map = this.studioGL.glyphMaps[ttfId];
    if (!map) return null;
    return map.get(ch) || map.get("?") || null;
  },

  shapeTextGlyphs(styleId, text, maxWidth) {
    if (!text || maxWidth <= 4) return { glyphs: [], totalW: 0, pxRange: 3.4 };
    const cache = CanvasModel.studioGraphState.shapedTextCache;
    const key = styleId + "|" + Math.round(maxWidth) + "|" + text;
    const cached = cache.get(key);
    if (cached) return cached;

    const style = CanvasModel.FONT_STYLES[styleId];
    const baseEm = CanvasModel.MSDF_BASE_EM_PX;
    const scale = style.size / baseEm;
    const extraTrack = style.track;
    const pxRange = scale * (2.0 * CanvasModel.MSDF_PIXEL_RANGE);
    const baselineOffsetY = style.size * 0.36;

    const chars = Array.from(text);
    const fullGlyphs = new Array(chars.length);
    let totalW = 0;
    for (let i = 0; i < chars.length; i++) {
      const g = this.lookupGlyph(style.ttfId, chars[i]);
      fullGlyphs[i] = g;
      totalW += (g ? g.advanceWidth : baseEm * 0.3) * scale + extraTrack;
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
        if (g && g.w > 0 && chars[i] !== " ") pushPlacedGlyph(g, curX);
        curX += (g ? g.advanceWidth : baseEm * 0.3) * scale + extraTrack;
      }
      const res = { glyphs: glyphs, totalW: totalW, pxRange: pxRange };
      if (cache.size > 4000) cache.clear();
      cache.set(key, res);
      return res;
    }

    const ellipsisGlyph = this.lookupGlyph(style.ttfId, "…");
    const ellipsisWidth = (ellipsisGlyph ? ellipsisGlyph.advanceWidth : baseEm * 0.6) * scale;
    const limitW = Math.max(0, maxWidth - ellipsisWidth);
    let curX = 0;
    for (let i = 0; i < fullGlyphs.length; i++) {
      const g = fullGlyphs[i];
      const advW = (g ? g.advanceWidth : baseEm * 0.3) * scale + extraTrack;
      if (curX + advW > limitW && i > 0) break;
      if (g && g.w > 0 && chars[i] !== " ") pushPlacedGlyph(g, curX);
      curX += advW;
    }
    if (ellipsisGlyph && ellipsisGlyph.w > 0) pushPlacedGlyph(ellipsisGlyph, curX);
    curX += ellipsisWidth;

    const res = { glyphs: glyphs, totalW: curX, pxRange: pxRange };
    cache.set(key, res);
    return res;
  },

  initContext(canvas) {
    this.ensureViews();
    const studioGL = this.studioGL;
    if (studioGL.gl && studioGL.canvas === canvas) return studioGL.gl;
    const gl =
      canvas.getContext("webgl", {
        alpha: false, antialias: true, depth: false, stencil: false, preserveDrawingBuffer: false, powerPreference: "high-performance"
      }) || canvas.getContext("experimental-webgl");
    if (!gl) return null;

    const UV = CanvasModel.UV;
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
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1]), gl.STATIC_DRAW);
    const aUnit = gl.getAttribLocation(prog, "aUnit");
    gl.enableVertexAttribArray(aUnit);
    gl.vertexAttribPointer(aUnit, 2, gl.FLOAT, false, 8, 0);
    if (instExt) instExt.vertexAttribDivisorANGLE(aUnit, 0);

    const aRect = gl.getAttribLocation(prog, "aRect");
    const aUV = gl.getAttribLocation(prog, "aUV");
    const aColor = gl.getAttribLocation(prog, "aColor");

    const instVbos = [gl.createBuffer(), gl.createBuffer(), gl.createBuffer()];
    const instVboCapacities = [0, 0, 0];
    const initCapBytes = 4096 * this.BYTES_PER_INSTANCE;
    for (let b = 0; b < 3; b++) {
      gl.bindBuffer(gl.ARRAY_BUFFER, instVbos[b]);
      gl.bufferData(gl.ARRAY_BUFFER, initCapBytes, gl.STREAM_DRAW);
      instVboCapacities[b] = initCapBytes;
    }

    const bpi = this.BYTES_PER_INSTANCE;
    const bindInstVboAttribs = (vbo) => {
      gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
      gl.vertexAttribPointer(aRect, 4, gl.FLOAT, false, bpi, 0);
      gl.vertexAttribPointer(aUV, 4, gl.FLOAT, false, bpi, 16);
      gl.vertexAttribPointer(aColor, 4, gl.UNSIGNED_BYTE, true, bpi, 32);
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

    studioGL.uiTex = createPlaceholderTexture(gl.TEXTURE0, new Uint8Array([255, 255, 255, 255]));
    studioGL.fontTex = createPlaceholderTexture(gl.TEXTURE1, new Uint8Array([0, 0, 0, 0]));
    studioGL.uiAtlasUploaded = false;
    studioGL.atlasUploaded = false;
    this.startPreloadingAtlas();

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

    CanvasModel.studioGraphState.staticBatchDirty = true;
    CanvasModel.studioGraphState.overlayBatchDirty = true;
    return gl;
  },

  pushQuadInstance(x, y, w, h, u0, v0, u1, v1, packedColor) {
    this.ensureViews();
    const idx = this.studioGL.instCursor;
    if (idx >= this.maxInstancesCapacity) this.growInstanceBuffer();
    const base = idx * this.FLOATS_PER_INSTANCE;
    const fv = this.instanceFloatView;
    fv[base] = x;
    fv[base + 1] = y;
    fv[base + 2] = w;
    fv[base + 3] = h;
    fv[base + 4] = u0;
    fv[base + 5] = v0;
    fv[base + 6] = u1;
    fv[base + 7] = v1;
    this.instanceUint32View[base + 8] = packedColor;
    this.studioGL.instCursor = idx + 1;
  },

  pushSolidQuad(x, y, w, h, packedColor) {
    const u = this.studioGL.uv.whiteU + CanvasModel.UV.solid;
    const v = this.studioGL.uv.whiteV;
    this.pushQuadInstance(x, y, w, h, u, v, u, v, packedColor);
  },

  pushBorderedBoxQuads(x, y, w, h, borderThickness, borderColor, fillColor) {
    this.pushSolidQuad(x, y, w, h, borderColor);
    this.pushSolidQuad(x + borderThickness, y + borderThickness, w - borderThickness * 2, h - borderThickness * 2, fillColor);
  },

  pushSolidBorderQuads(x, y, w, h, thickness, packedColor) {
    const t = thickness;
    this.pushSolidQuad(x, y, w, t, packedColor);
    this.pushSolidQuad(x, y + h - t, w, t, packedColor);
    this.pushSolidQuad(x, y + t, t, h - t * 2, packedColor);
    this.pushSolidQuad(x + w - t, y + t, t, h - t * 2, packedColor);
  },

  computeAlignedDashSpan(len, period) {
    const duty = CanvasModel.UV.dashDuty;
    const n = Math.max(1, Math.round(len / period - duty));
    return (n + duty) * period;
  },

  pushDashedSegmentQuad(x1, y1, x2, y2, thickness, period, packedColor) {
    const dx = x2 - x1, dy = y2 - y1, len = Math.hypot(dx, dy);
    if (len < 0.5) return;
    const halfT = thickness * 0.5, negPeriod = -period, span = this.computeAlignedDashSpan(len, period);
    if (Math.abs(dx) >= Math.abs(dy)) {
      this.pushQuadInstance(Math.min(x1, x2), y1 - halfT, len, thickness, 0, negPeriod, span, negPeriod, packedColor);
    } else {
      this.pushQuadInstance(x1 - halfT, Math.min(y1, y2), thickness, len, 0, negPeriod, span, negPeriod, packedColor);
    }
  },

  pushDashedHeaderDividerQuad(x, y, w, thickness, period, packedColor) {
    const duty = CanvasModel.UV.dashDuty;
    const nW = Math.max(1, Math.round(w / period - duty));
    if (nW <= 2) return;
    const worldPeriodW = w / (nW + duty);
    const startX = x + worldPeriodW;
    const innerW = w - worldPeriodW * 2.0;
    const span = (nW - 2 + duty) * period;
    this.pushQuadInstance(startX, y, innerW, thickness, 0, -period, span, -period, packedColor);
  },

  pushDashedRectQuads(x, y, w, h, thickness, period, packedColor) {
    const t = thickness, negPeriod = -period;
    const spanW = this.computeAlignedDashSpan(w, period);
    const innerH = Math.max(1, h - t * 2);
    const spanH = this.computeAlignedDashSpan(innerH, period);
    this.pushQuadInstance(x, y, w, t, 0, negPeriod, spanW, negPeriod, packedColor);
    this.pushQuadInstance(x, y + h - t, w, t, 0, negPeriod, spanW, negPeriod, packedColor);
    this.pushQuadInstance(x, y + t, t, innerH, 0, negPeriod, spanH, negPeriod, packedColor);
    this.pushQuadInstance(x + w - t, y + t, t, innerH, 0, negPeriod, spanH, negPeriod, packedColor);
  },

  pushWireChevronQuad(x, y, dir, packedColor) {
    const isHoriz = dir === "right" || dir === "left";
    const sp = isHoriz ? this.studioGL.uv.chevronH : this.studioGL.uv.chevronV;
    const hs = sp.size * 0.5;
    const u0 = dir === "left" ? sp.u1 : sp.u0;
    const u1 = dir === "left" ? sp.u0 : sp.u1;
    const v0 = dir === "up" ? sp.v1 : sp.v0;
    const v1 = dir === "up" ? sp.v0 : sp.v1;
    this.pushQuadInstance(x - hs, y - hs, sp.size, sp.size, u0, v0, u1, v1, packedColor);
  },

  pushHorizontalWireSegmentWithBridges(xA, xB, y, thickness, packedColor, segBridges) {
    const halfT = thickness * 0.5;
    const minX = xA < xB ? xA : xB;
    const maxX = xA > xB ? xA : xB;
    if (maxX - minX < 0.5) return;
    if (!segBridges || segBridges.length === 0) {
      this.pushSolidQuad(minX, y - halfT, maxX - minX, thickness, packedColor);
      return;
    }
    const gapR = 2.7;
    let curX = minX;
    for (let k = 0; k < segBridges.length; k++) {
      const bx = segBridges[k];
      if (bx - gapR < minX + 1.0 || bx + gapR > maxX - 1.0) continue;
      const leftLen = bx - gapR - curX;
      if (leftLen > 0.5) this.pushSolidQuad(curX, y - halfT, leftLen, thickness, packedColor);
      if (bx + gapR > curX) curX = bx + gapR;
    }
    if (maxX - curX > 0.5) this.pushSolidQuad(curX, y - halfT, maxX - curX, thickness, packedColor);
  },

  pushWirePolylineQuads(pts, thickness, isDashed, packedColor, bridgesBySeg) {
    if (!pts || pts.length < 2) return;
    const halfT = thickness * 0.5;
    const lastSeg = pts.length - 2;
    const padW = CanvasModel.PAD.w;
    for (let i = 0; i <= lastSeg; i++) {
      const p0 = pts[i], p1 = pts[i + 1];
      if (isDashed) {
        this.pushDashedSegmentQuad(p0.x, p0.y, p1.x, p1.y, thickness, 8.0, packedColor);
        continue;
      }
      const dx = p1.x - p0.x, dy = p1.y - p0.y;
      if (Math.abs(dx) >= Math.abs(dy)) {
        let xA = p0.x, xB = p1.x;
        if (i > 0) xA += dx >= 0 ? -halfT : halfT;
        if (i < lastSeg) xB += dx >= 0 ? halfT : -halfT;
        const segBridges = bridgesBySeg ? bridgesBySeg[i] : null;
        this.pushHorizontalWireSegmentWithBridges(xA, xB, p0.y, thickness, packedColor, segBridges);
        if (Math.abs(dx) >= padW) {
          const mx = (p0.x + p1.x) * 0.5;
          let nearGap = false;
          if (segBridges) {
            for (let g = 0; g < segBridges.length; g++) {
              if (Math.abs(segBridges[g] - mx) < 10) { nearGap = true; break; }
            }
          }
          if (!nearGap) this.pushWireChevronQuad(mx, p0.y, dx >= 0 ? "right" : "left", packedColor);
        }
      } else {
        const minY = p0.y < p1.y ? p0.y : p1.y;
        const maxY = p0.y > p1.y ? p0.y : p1.y;
        const lenY = maxY - minY;
        if (lenY >= 0.5) this.pushSolidQuad(p0.x - halfT, minY, thickness, lenY, packedColor);
        if ((i === lastSeg && lenY >= 18) || lenY >= padW) {
          this.pushWireChevronQuad(p0.x, (p0.y + p1.y) * 0.5, dy >= 0 ? "down" : "up", packedColor);
        }
      }
    }
  },

  pushShapedTextQuads(shaped, originX, centerY, packedColor) {
    const glyphs = shaped.glyphs;
    const uBias = shaped.pxRange >= 3.2 ? CanvasModel.UV.fontLg : CanvasModel.UV.fontSm;
    for (let i = 0; i < glyphs.length; i++) {
      const g = glyphs[i];
      this.pushQuadInstance(originX + g.dx, centerY + g.dy, g.w, g.h, g.u0 + uBias, g.v0, g.u1 + uBias, g.v1, packedColor);
    }
  },

  resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, fallbackColor) {
    if (typeof isHoveredOrActive === "number") return isHoveredOrActive;
    if (isHoveredOrActive) return theme.textPrimary;
    if (isUnreachable) return theme.unreachWire;
    return fallbackColor;
  },

  pushTwoLayerSpritePort(x, y, fillSp, ringSp, bgColor, portColor) {
    const hs = fillSp.size * 0.5;
    this.pushQuadInstance(x - hs, y - hs, fillSp.size, fillSp.size, fillSp.u0, fillSp.v0, fillSp.u1, fillSp.v1, bgColor);
    this.pushQuadInstance(x - hs, y - hs, ringSp.size, ringSp.size, ringSp.u0, ringSp.v0, ringSp.u1, ringSp.v1, portColor);
  },

  pushPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable, isConditional) {
    const uv = this.studioGL.uv;
    const portColor = this.resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, theme.wireDefault);
    if (isConditional) {
      const ringSp = isConnected ? uv.diamondPortHover : uv.diamondPortRing;
      this.pushTwoLayerSpritePort(x, y, uv.diamondPortFill, ringSp, theme.bgObsidian, portColor);
      return;
    }
    const sp = isConnected ? uv.portHover : uv.portRing;
    this.pushTwoLayerSpritePort(x, y, uv.portFill, sp, theme.bgObsidian, portColor);
  },

  pushInputSocketQuads(x, y, isHoveredOrActive, theme, isUnreachable, colorIdx) {
    const uv = this.studioGL.uv;
    const portColor = this.resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, this.resolveWireColor(colorIdx, theme));
    this.pushTwoLayerSpritePort(x, y, uv.portFill, uv.portRing, theme.bgObsidian, portColor);
  },

  pushSmallPortQuads(x, y, isConnected, isHoveredOrActive, theme, isUnreachable) {
    const uv = this.studioGL.uv;
    const portColor = this.resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, theme.wireDefault);
    const ringSp = isConnected ? uv.smallPortHover : uv.smallPortRing;
    this.pushTwoLayerSpritePort(x, y, uv.smallPortFill, ringSp, theme.bgObsidian, portColor);
  },

  pushSmallInputSocketQuads(x, y, isHoveredOrActive, theme, isUnreachable, colorIdx) {
    const uv = this.studioGL.uv;
    const portColor = this.resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, this.resolveWireColor(colorIdx, theme));
    this.pushTwoLayerSpritePort(x, y, uv.smallPortFill, uv.smallPortRing, theme.bgObsidian, portColor);
  },

  findClosestPolylineSegment(pts, px, py) {
    let bestSeg = 0, bestDistSq = Infinity, bestLenSq = 0;
    for (let s = 0; s < pts.length - 1; s++) {
      const p0 = pts[s], p1 = pts[s + 1], dx = p1.x - p0.x, dy = p1.y - p0.y, lenSq = dx * dx + dy * dy;
      if (lenSq < 1.0) continue;
      const t = Math.max(0.0, Math.min(1.0, ((px - p0.x) * dx + (py - p0.y) * dy) / lenSq));
      const ex = px - (p0.x + t * dx), ey = py - (p0.y + t * dy), dSq = ex * ex + ey * ey;
      if (dSq < bestDistSq - 0.5 || (dSq <= bestDistSq + 0.5 && (lenSq >= 64.0 || lenSq > bestLenSq))) {
        bestDistSq = dSq; bestSeg = s; bestLenSq = lenSq;
      }
    }
    return bestSeg;
  },

  computeWirePinSegmentBar(wire, pin) {
    const pts = wire && (wire.pts && wire.pts.length >= 2 ? wire.pts : wire.basePts);
    if (!pts || pts.length < 2 || !pin) return null;
    const bestSeg =
      typeof pin.segIdx === "number" && pin.segIdx >= 0 && pin.segIdx < pts.length - 1
        ? pin.segIdx
        : this.findClosestPolylineSegment(pts, pin.x, pin.y);
    if (bestSeg < 0 || bestSeg >= pts.length - 1) return null;
    const a = pts[bestSeg], b = pts[bestSeg + 1], dx = b.x - a.x, dy = b.y - a.y;
    const isHoriz = Math.abs(dx) >= Math.abs(dy);
    const segLen = isHoriz ? Math.abs(dx) : Math.abs(dy);
    if (segLen < 4) return null;
    const maxBarLen = CanvasModel.PAD.w;
    const availLen = segLen > 20 ? segLen - 16 : Math.max(6, segLen - 4);
    const barLen = Math.min(maxBarLen, availLen);
    const cx = (a.x + b.x) * 0.5, cy = (a.y + b.y) * 0.5;
    if (isHoriz) {
      return { isHoriz: true, segIdx: bestSeg, x1: cx - barLen * 0.5, y1: a.y, x2: cx + barLen * 0.5, y2: a.y, x: cx - barLen * 0.5, y: a.y, w: barLen, cx: cx, cy: a.y };
    }
    return { isHoriz: false, segIdx: bestSeg, x1: a.x, y1: cy - barLen * 0.5, x2: a.x, y2: cy + barLen * 0.5, x: a.x, y: cy - barLen * 0.5, h: barLen, cx: a.x, cy: cy };
  },

  pushWirePinArrowQuads(wire, pinIdx, isHoveredOrActive, theme, isUnreachable) {
    if (!wire || !wire.pins || !wire.pins[pinIdx]) return;
    const bar = this.computeWirePinSegmentBar(wire, wire.pins[pinIdx]);
    if (!bar) return;
    const thick = isHoveredOrActive === true ? 6 : 5;
    const halfT = thick * 0.5;
    const pinColor = this.resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, this.resolveWireColor(wire.colorIdx, theme));
    if (bar.isHoriz) this.pushSolidQuad(bar.x, bar.y - halfT, bar.w, thick, pinColor);
    else this.pushSolidQuad(bar.x - halfT, bar.y, thick, bar.h, pinColor);
  },

  isHoveredPortItemConnected(hovered) {
    if (!hovered) return false;
    if (hovered.type === "scene-out-port") return Boolean(hovered.port && hovered.port.connected);
    if (hovered.type === "chapter-out-port") return Boolean(hovered.outPort && hovered.outPort.connected);
    if (hovered.type === "wire-pin") return true;
    return false;
  }
};
Dispatcher.register("EditorWebgl", EditorWebgl);
