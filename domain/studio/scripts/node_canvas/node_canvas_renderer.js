"use strict";

// NodeCanvasRenderer: Internal WebGL instanced renderer for the node canvas.
// Private to NodeCanvasController — contains zero business logic and zero actions.
// Manages WebGL shaders, texture atlases (/studio-ui.png, /studio-atlas.png),
// MSDF glyph shaping, static/dynamic VBO ring buffers, and 120fps draw calls.
const NodeCanvasRenderer = {
  floatsPerInstance: 9,
  bytesPerInstance: 36,
  maxInstancesCapacity: 32768,
  instanceArrayBuffer: new ArrayBuffer(32768 * 36),
  instanceFloatView: null,
  instanceUint32View: null,

  tokens: {
    canvasSelector: "#studio-canvas",
    viewportSelector: "#timeline-scroll-viewport",
    textEditorSelector: "#text-editor-view",
    modeFullClass: "mode-full",
    hasRightPanelClass: "has-right-panel",
    uiAtlasUrl: "/studio-ui.png",
    fontAtlasUrl: "/studio-atlas.png",
    bodyThemeAttr: "data-theme",
    lightTheme: "light",
    webglCtx: "webgl",
    expWebglCtx: "experimental-webgl",
    highPerf: "high-performance",
    extInstanced: "ANGLE_instanced_arrays",
    aUnit: "aUnit",
    aRect: "aRect",
    aUV: "aUV",
    aColor: "aColor",
    uViewSize: "uViewSize",
    uPanZoom: "uPanZoom",
    uDpr: "uDpr",
    uBgColor: "uBgColor",
    uDotColor: "uDotColor",
    uGridStep: "uGridStep",
    uUIAtlas: "uUIAtlas",
    uFontAtlas: "uFontAtlas",
    dirRight: "right",
    dirLeft: "left",
    dirUp: "up",
    dirDown: "down",
    startBadge: "START",
    entryBadgeKey: "entry_badge_upper",
    choicePrefixKey: "choice_prefix",
    addChoiceBtnKey: "add_choice_btn",
    ellipsisChar: "…",
    fallbackGlyph: "?",
    numType: "number",
    undefType: "undefined"
  },

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
    shapedTextCache: new Map(),
    viewW: 1200,
    viewH: 700,
    canvasDpr: 2,
    uv: null
  },

  ensureViews() {
    if (!this.studioGL.uv) {
      this.studioGL.uv = NodeCanvasStyle.sprites;
    }
    if (!this.instanceFloatView) {
      this.instanceFloatView = new Float32Array(this.instanceArrayBuffer);
      this.instanceUint32View = new Uint32Array(this.instanceArrayBuffer);
    }
  },

  growInstanceBuffer() {
    this.ensureViews();
    const nextCap = this.maxInstancesCapacity * 2;
    const nextBuf = new ArrayBuffer(nextCap * this.bytesPerInstance);
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
    const raw = typeof MsdfMetrics !== this.tokens.undefType ? MsdfMetrics.data || "" : "";
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
        u0: parseFloat(p[2]), v0: parseFloat(p[3]), u1: parseFloat(p[4]), v1: parseFloat(p[5]),
        w: parseFloat(p[6]), h: parseFloat(p[7]), ox: parseFloat(p[8]), oy: parseFloat(p[9]),
        adv: adv, advanceWidth: adv
      });
    }
  },

  startPreloadingAtlas() {
    const t = this.tokens;
    if (typeof Image === t.undefType) return;
    const glState = this.studioGL;
    if (!glState.uiAtlasImg) {
      const uiImg = new Image();
      glState.uiAtlasImg = uiImg;
      uiImg.onload = () => {
        glState.uiAtlasImgLoaded = true;
        glState.uiAtlasUploaded = false;
        if (document.querySelector(t.canvasSelector)) this.render();
      };
      uiImg.src = t.uiAtlasUrl;
    }
    if (!glState.atlasImg) {
      const img = new Image();
      glState.atlasImg = img;
      img.onload = () => {
        glState.atlasImgLoaded = true;
        glState.atlasUploaded = false;
        if (document.querySelector(t.canvasSelector)) this.render();
      };
      img.src = t.fontAtlasUrl;
    }
  },

  resolveThemeColors() {
    const t = this.tokens, themes = NodeCanvasStyle.themes;
    return document.body && document.body.getAttribute(t.bodyThemeAttr) === t.lightTheme ? themes.light : themes.dark;
  },

  resolveWireColor(colorIdx, theme) {
    if (typeof colorIdx === this.tokens.numType && colorIdx >= 0 && theme.wirePalette) {
      return theme.wirePalette[colorIdx % theme.wirePalette.length];
    }
    return theme.wireDefault || theme.borderStrong;
  },

  lookupGlyph(ttfId, ch) {
    this.ensureMetricsParsed();
    const map = this.studioGL.glyphMaps[ttfId];
    if (!map) return null;
    return map.get(ch) || map.get(this.tokens.fallbackGlyph) || null;
  },

  shapeTextGlyphs(styleId, text, maxWidth) {
    if (!text || maxWidth <= 4) return { glyphs: [], totalW: 0, pxRange: 3.4 };
    const cache = this.studioGL.shapedTextCache;
    const key = styleId + "|" + Math.round(maxWidth) + "|" + text;
    const cached = cache.get(key);
    if (cached) return cached;

    const typo = NodeCanvasStyle.typography;
    const style = typo.styles[styleId];
    const baseEm = typo.emPx;
    const scale = style.size / baseEm;
    const extraTrack = style.track;
    const pxRange = scale * (2.0 * typo.pixelRange);
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

    const ellipsisGlyph = this.lookupGlyph(style.ttfId, this.tokens.ellipsisChar);
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
    const studioGL = this.studioGL, t = this.tokens;
    if (studioGL.gl && studioGL.canvas === canvas) return studioGL.gl;
    const gl =
      canvas.getContext(t.webglCtx, {
        alpha: false, antialias: true, depth: false, stencil: false, preserveDrawingBuffer: false, powerPreference: t.highPerf
      }) || canvas.getContext(t.expWebglCtx);
    if (!gl) return null;

    const instExt = gl.getExtension(t.extInstanced);
    studioGL.instExt = instExt;

    const compileShader = (type, source) => {
      const shader = gl.createShader(type);
      gl.shaderSource(shader, source);
      gl.compileShader(shader);
      return shader;
    };

    const vertexShader = compileShader(gl.VERTEX_SHADER, NodeCanvasVertexShader.source);
    const fragmentShader = compileShader(gl.FRAGMENT_SHADER, NodeCanvasFragmentShader.source);
    const shaderProgram = gl.createProgram();
    gl.attachShader(shaderProgram, vertexShader);
    gl.attachShader(shaderProgram, fragmentShader);
    gl.linkProgram(shaderProgram);
    gl.useProgram(shaderProgram);

    const quadVbo = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, quadVbo);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1]), gl.STATIC_DRAW);
    const aUnit = gl.getAttribLocation(shaderProgram, t.aUnit);
    gl.enableVertexAttribArray(aUnit);
    gl.vertexAttribPointer(aUnit, 2, gl.FLOAT, false, 8, 0);
    if (instExt) instExt.vertexAttribDivisorANGLE(aUnit, 0);

    const aRect = gl.getAttribLocation(shaderProgram, t.aRect);
    const aUV = gl.getAttribLocation(shaderProgram, t.aUV);
    const aColor = gl.getAttribLocation(shaderProgram, t.aColor);

    const instVbos = [gl.createBuffer(), gl.createBuffer(), gl.createBuffer()];
    const instVboCapacities = [0, 0, 0];
    const initCapBytes = 4096 * this.bytesPerInstance;
    for (let b = 0; b < 3; b++) {
      gl.bindBuffer(gl.ARRAY_BUFFER, instVbos[b]);
      gl.bufferData(gl.ARRAY_BUFFER, initCapBytes, gl.STREAM_DRAW);
      instVboCapacities[b] = initCapBytes;
    }

    const bpi = this.bytesPerInstance;
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
    studioGL.uViewSize = gl.getUniformLocation(shaderProgram, t.uViewSize);
    studioGL.uPanZoom = gl.getUniformLocation(shaderProgram, t.uPanZoom);
    studioGL.uDpr = gl.getUniformLocation(shaderProgram, t.uDpr);
    studioGL.uBgColor = gl.getUniformLocation(shaderProgram, t.uBgColor);
    studioGL.uDotColor = gl.getUniformLocation(shaderProgram, t.uDotColor);
    studioGL.uGridStep = gl.getUniformLocation(shaderProgram, t.uGridStep);
    studioGL.uUIAtlas = gl.getUniformLocation(shaderProgram, t.uUIAtlas);
    studioGL.uFontAtlas = gl.getUniformLocation(shaderProgram, t.uFontAtlas);
    gl.uniform1i(studioGL.uUIAtlas, 0);
    gl.uniform1i(studioGL.uFontAtlas, 1);

    NodeCanvasLayout.studioGraphState.staticBatchDirty = true;
    NodeCanvasLayout.studioGraphState.overlayBatchDirty = true;
    return gl;
  },

  pushQuadInstance(x, y, w, h, u0, v0, u1, v1, packedColor) {
    this.ensureViews();
    const idx = this.studioGL.instCursor;
    if (idx >= this.maxInstancesCapacity) this.growInstanceBuffer();
    const base = idx * this.floatsPerInstance;
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
    const u = this.studioGL.uv.whiteU + NodeCanvasStyle.uv.solid;
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
    const duty = NodeCanvasStyle.uv.dashDuty;
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
    const duty = NodeCanvasStyle.uv.dashDuty;
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
    const t = this.tokens;
    const isHoriz = dir === t.dirRight || dir === t.dirLeft;
    const sp = isHoriz ? this.studioGL.uv.chevronH : this.studioGL.uv.chevronV;
    const hs = sp.size * 0.5;
    const u0 = dir === t.dirLeft ? sp.u1 : sp.u0;
    const u1 = dir === t.dirLeft ? sp.u0 : sp.u1;
    const v0 = dir === t.dirUp ? sp.v1 : sp.v0;
    const v1 = dir === t.dirUp ? sp.v0 : sp.v1;
    this.pushQuadInstance(x - hs, y - hs, sp.size, sp.size, u0, v0, u1, v1, packedColor);
  },

  pushWirePolylineQuads(pts, thickness, isDashed, packedColor) {
    if (!pts || pts.length < 2) return;
    const t = this.tokens, halfT = thickness * 0.5, lastSeg = pts.length - 2, padW = NodeCanvasStyle.pad.w;
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
        const minX = xA < xB ? xA : xB, maxX = xA > xB ? xA : xB;
        if (maxX - minX >= 0.5) {
          this.pushSolidQuad(minX, p0.y - halfT, maxX - minX, thickness, packedColor);
        }
        if (Math.abs(dx) >= padW) {
          this.pushWireChevronQuad((p0.x + p1.x) * 0.5, p0.y, dx >= 0 ? t.dirRight : t.dirLeft, packedColor);
        }
      } else {
        const minY = p0.y < p1.y ? p0.y : p1.y, maxY = p0.y > p1.y ? p0.y : p1.y, lenY = maxY - minY;
        if (lenY >= 0.5) this.pushSolidQuad(p0.x - halfT, minY, thickness, lenY, packedColor);
        if ((i === lastSeg && lenY >= 18) || lenY >= padW) {
          this.pushWireChevronQuad(p0.x, (p0.y + p1.y) * 0.5, dy >= 0 ? t.dirDown : t.dirUp, packedColor);
        }
      }
    }
  },

  pushShapedTextQuads(shaped, originX, centerY, packedColor) {
    const glyphs = shaped.glyphs;
    const uBias = shaped.pxRange >= 3.2 ? NodeCanvasStyle.uv.fontLg : NodeCanvasStyle.uv.fontSm;
    for (let i = 0; i < glyphs.length; i++) {
      const g = glyphs[i];
      this.pushQuadInstance(originX + g.dx, centerY + g.dy, g.w, g.h, g.u0 + uBias, g.v0, g.u1 + uBias, g.v1, packedColor);
    }
  },

  resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, fallbackColor) {
    if (typeof isHoveredOrActive === this.tokens.numType) return isHoveredOrActive;
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

  computeWirePinSegmentBar: NodeCanvasGeometry.computeWirePinSegmentBar.bind(NodeCanvasGeometry),

  pushWirePinArrowQuads(wire, pinIdx, isHoveredOrActive, theme, isUnreachable) {
    if (!wire || !wire.pins || !wire.pins[pinIdx]) return;
    const bar = NodeCanvasGeometry.computeWirePinSegmentBar(wire, wire.pins[pinIdx]);
    if (!bar) return;
    const thick = isHoveredOrActive === true ? 6 : 5;
    const halfT = thick * 0.5;
    const pinColor = this.resolvePortQuadColor(isHoveredOrActive, theme, isUnreachable, this.resolveWireColor(wire.colorIdx, theme));
    if (bar.isHoriz) this.pushSolidQuad(bar.x, bar.y - halfT, bar.w, thick, pinColor);
    else this.pushSolidQuad(bar.x - halfT, bar.y, thick, bar.h, pinColor);
  },

  isHoveredPortItemConnected(hovered) {
    if (!hovered) return false;
    const ht = NodeCanvasHelpers.hitTypes;
    if (hovered.type === ht.sceneOutPort) return Boolean(hovered.port && hovered.port.connected);
    if (hovered.type === ht.chapterOutPort) return Boolean(hovered.outPort && hovered.outPort.connected);
    if (hovered.type === ht.wirePin) return true;
    return false;
  },

  pushChapterPortsQuads(ch, theme) {
    if (!ch || ch.hidden) return;
    if (!ch.isRoot) {
      this.pushSmallInputSocketQuads(ch.inX, ch.inY, false, theme, false, ch.inColorIdx);
    }
    for (let j = 0; j < ch.outPorts.length; j++) {
      const op = ch.outPorts[j];
      const opColor = op.connected ? this.resolveWireColor(op.colorIdx !== undefined ? op.colorIdx : op.inColorIdx, theme) : false;
      this.pushSmallPortQuads(op.x, op.y, Boolean(op.connected), opColor, theme, false);
    }
  },

  pushNodePortsQuads(nd, theme) {
    if (nd.hasInputPort) {
      this.pushInputSocketQuads(nd.inX, nd.inY, false, theme, nd.unreachable, nd.inColorIdx);
    }
    if (nd.hasChoices && nd.ports.length > 0) {
      for (let j = 0; j < nd.ports.length; j++) {
        const pt = nd.ports[j];
        const ptColor = pt.connected && !nd.unreachable ? this.resolveWireColor(pt.colorIdx, theme) : false;
        this.pushPortQuads(pt.x, pt.y, Boolean(pt.connected), ptColor, theme, nd.unreachable, Boolean(pt.isConditional));
      }
    } else if (nd.implicitPort) {
      const imp = nd.implicitPort;
      const impColor = imp.connected && !nd.unreachable ? this.resolveWireColor(imp.colorIdx, theme) : false;
      this.pushPortQuads(imp.x, imp.y, Boolean(imp.connected), impColor, theme, nd.unreachable, false);
    }
  },

  pushNodeChoiceRowQuads(nd, j, isHistoryPreview, isDraggingRow, theme) {
    const card = NodeCanvasStyle.card, uv = NodeCanvasStyle.uv, styleIds = NodeCanvasStyle.typography.styleIds;
    const gState = NodeCanvasLayout.studioGraphState, pt = nd.ports[j];
    const rowY = nd.y + NodeCanvasHelpers.choiceRowY(j);
    const gripSp = this.studioGL.uv.grip, trashSp = this.studioGL.uv.trashIcon;
    const textSec = nd.unreachable ? theme.unreachText : theme.textSecondary;

    if (isDraggingRow) {
      this.pushBorderedBoxQuads(nd.x + 1, rowY, nd.w - 2, card.choiceH, 1, theme.textPrimary, theme.bgElevated);
    } else {
      this.pushSolidQuad(nd.x + 1, rowY + 1, nd.w - 2, card.choiceH - 1, theme.bgObsidian);
    }
    if (!isHistoryPreview) {
      this.pushQuadInstance(nd.x + 3, rowY + (card.choiceH - 18) * 0.5, 16, 18, gripSp.u0 + uv.solid, gripSp.v0, gripSp.u1 + uv.solid, gripSp.v1, isDraggingRow ? theme.textPrimary : theme.textMuted);
    }
    const isEditingThisChoice = !isDraggingRow && gState.editingChoice && gState.editingChoice.node === nd && gState.editingChoice.idx === j;
    if (!isEditingThisChoice) {
      const rawLabel = pt.label || UIController.tr(this.tokens.choicePrefixKey) + (j + 1);
      const choiceMaxW = nd.w - (isHistoryPreview ? 36 : 47);
      if (pt._shapedLabelTxt !== rawLabel || pt._shapedChoiceMaxW !== choiceMaxW || !pt._shapedChoice) {
        pt._shapedLabelTxt = rawLabel;
        pt._shapedChoiceMaxW = choiceMaxW;
        pt._shapedChoice = this.shapeTextGlyphs(styleIds.choice, rawLabel, choiceMaxW);
      }
      this.pushShapedTextQuads(pt._shapedChoice, nd.x + (isHistoryPreview ? 10 : 21), rowY + card.choiceH * 0.5, isDraggingRow ? theme.textPrimary : textSec);
    }
    if (!isHistoryPreview) {
      const tx = nd.x + nd.w - 24, ty = rowY + (card.choiceH - trashSp.size) * 0.5;
      this.pushQuadInstance(tx, ty, trashSp.size, trashSp.size, trashSp.u0, trashSp.v0, trashSp.u1, trashSp.v1, isDraggingRow ? theme.textSecondary : textSec);
    }
  },

  pushCenteredFooterSprite(btnX, btnY, sprite, color, bgMaskColor) {
    const btnSize = NodeCanvasStyle.card.btnSize;
    const ix = btnX + (btnSize - sprite.size) * 0.5, iy = btnY + (btnSize - sprite.size) * 0.5;
    if (bgMaskColor !== undefined) {
      this.pushSolidQuad(ix - 1, iy - 1, sprite.size + 2, sprite.size + 2, bgMaskColor);
    }
    this.pushQuadInstance(ix, iy, sprite.size, sprite.size, sprite.u0, sprite.v0, sprite.u1, sprite.v1, color);
  },

  compileStaticSceneQuads(theme) {
    const studioGL = this.studioGL, gState = NodeCanvasLayout.studioGraphState, state = NodeCanvasController.data;
    const card = NodeCanvasStyle.card, uv = NodeCanvasStyle.uv, styleIds = NodeCanvasStyle.typography.styleIds;
    const ht = NodeCanvasHelpers.tokens, t = this.tokens;
    studioGL.instCursor = 0;

    this.pushQuadInstance(0, 0, 1, 1, uv.grid, uv.grid, uv.grid, uv.grid, theme.bgObsidian);

    const vw = studioGL.viewW || 1200, vh = studioGL.viewH || 700;
    const z = state.zoom || 1.0, px = state.panX || 0, py = state.panY || 0;
    const visMinX = -px / z, visMinY = -py / z, visMaxX = (vw - px) / z, visMaxY = (vh - py) / z;
    const guardX = Math.max(120, (visMaxX - visMinX) * 0.25), guardY = Math.max(120, (visMaxY - visMinY) * 0.25);
    const fMinX = visMinX - guardX, fMinY = visMinY - guardY, fMaxX = visMaxX + guardX, fMaxY = visMaxY + guardY;
    studioGL.cullMinX = fMinX; studioGL.cullMinY = fMinY; studioGL.cullMaxX = fMaxX; studioGL.cullMaxY = fMaxY;
    studioGL.hasCullBounds = true;

    const chapters = gState.chapters, wires = gState.wires, nodes = gState.nodes;
    const gripSp = studioGL.uv.grip;

    for (let i = 0; i < chapters.length; i++) {
      const ch = chapters[i];
      if (ch.hidden || ch.right < fMinX || ch.left > fMaxX || ch.bottom < fMinY || ch.top > fMaxY) continue;
      this.pushSolidQuad(ch.x, ch.y, ch.w, ch.h, theme.chapterFill);
      this.pushSolidQuad(ch.x + 1, ch.y + 1, ch.w - 2, 27, theme.bgElevated);
      this.pushDashedRectQuads(ch.x, ch.y, ch.w, ch.h, 1, 9.0, theme.borderStrong);
      this.pushDashedHeaderDividerQuad(ch.x, ch.y + 28, ch.w, 1, 9.0, theme.borderStrong);
      this.pushQuadInstance(ch.x + 4, ch.y + 3, gripSp.w, gripSp.h, gripSp.u0 + uv.solid, gripSp.v0, gripSp.u1 + uv.solid, gripSp.v1, theme.textMuted);
      if (gState.editingChapter !== ch && ch.y + 30 >= fMinY && ch.y <= fMaxY) {
        const chMaxW = ch.w - 38;
        if (ch._shapedName !== ch.name || ch._shapedMaxW !== chMaxW || !ch._shapedTitle) {
          ch._shapedName = ch.name;
          ch._shapedMaxW = chMaxW;
          ch._shapedTitle = this.shapeTextGlyphs(styleIds.chapter, (ch.name || "").toUpperCase(), chMaxW);
        }
        this.pushShapedTextQuads(ch._shapedTitle, ch.x + 28, ch.y + 15, theme.textPrimary);
      }
    }

    for (let i = 0; i < wires.length; i++) {
      const w = wires[i];
      if (w.maxX < fMinX || w.minX > fMaxX || w.maxY < fMinY || w.minY > fMaxY) continue;
      const wColor = this.resolveWireColor(w.colorIdx, theme);
      if (w.unreachable) this.pushWirePolylineQuads(w.pts, 2, true, theme.unreachWire);
      else this.pushWirePolylineQuads(w.pts, 2, false, wColor);
      if (w.pins && w.pins.length > 0) {
        for (let p = 0; p < w.pins.length; p++) {
          this.pushWirePinArrowQuads(w, p, w.unreachable ? false : wColor, theme, w.unreachable);
        }
      }
    }

    const branchPlusSp = studioGL.uv.branchPlus, editPenSp = studioGL.uv.editPen, trashSp = studioGL.uv.trashIcon;
    const isHistoryPreview = NodeCanvasHelpers.isHistoryPreview();

    for (let i = 0; i < nodes.length; i++) {
      const nd = nodes[i];
      if (nd.right + 12 < fMinX || nd.left - 12 > fMaxX || nd.bottom + 12 < fMinY || nd.top - 12 > fMaxY) continue;
      const isUnreach = Boolean(nd.unreachable);
      const cardBg = isUnreach ? theme.unreachCardBg : theme.bgObsidian;
      const textPri = isUnreach ? theme.unreachText : theme.textPrimary;
      const textSec = isUnreach ? theme.unreachText : theme.textSecondary;

      if (isUnreach) {
        this.pushSolidQuad(nd.x, nd.y, nd.w, nd.h, cardBg);
        this.pushDashedRectQuads(nd.x, nd.y, nd.w, nd.h, 1, 8.0, theme.unreachBorder);
      } else {
        this.pushBorderedBoxQuads(nd.x, nd.y, nd.w, nd.h, 1, theme.borderStrong, cardBg);
      }

      this.pushQuadInstance(nd.x + 4, nd.y + 6, gripSp.w, gripSp.h, gripSp.u0 + uv.solid, gripSp.v0, gripSp.u1 + uv.solid, gripSp.v1, theme.textMuted);

      if (nd.isStart || nd.isEntry) {
        const badgeTxt = nd.isStart ? t.startBadge : UIController.tr(t.entryBadgeKey);
        if (nd._shapedBadgeTxt !== badgeTxt || !nd._shapedBadge) {
          nd._shapedBadgeTxt = badgeTxt;
          nd._shapedBadge = this.shapeTextGlyphs(styleIds.badge, badgeTxt, 64);
          NodeCanvasModel.recomputeNodeMetrics(nd);
        }
        const bx = nd.x + nd.w - nd.badgeW - card.pad, by = nd.y + card.pad;
        this.pushBorderedBoxQuads(bx, by, nd.badgeW, card.badgeH, 1, nd.isStart ? theme.textPrimary : theme.borderStrong, theme.bgElevated);
        this.pushShapedTextQuads(nd._shapedBadge, bx + 5, by + 8.5, nd.isStart ? theme.textPrimary : theme.textSecondary);
      }

      if (nd.branchRadioX > 0) {
        this.pushSolidQuad(nd.branchRadioX, nd.branchRadioY, card.radioSize, card.radioSize, nd.branchRole === ht.rolePrimary ? theme.textPrimary : theme.borderStrong);
      }

      if (gState.editingNodeTitle !== nd) {
        const rawTitle = nd.name || nd.id;
        if (nd._shapedTitleTxt !== rawTitle || nd._shapedTitleMaxW !== nd.titleMaxW || !nd._shapedTitle) {
          nd._shapedTitleTxt = rawTitle;
          nd._shapedTitleMaxW = nd.titleMaxW;
          nd._shapedTitle = this.shapeTextGlyphs(styleIds.title, rawTitle, nd.titleMaxW);
        }
        this.pushShapedTextQuads(nd._shapedTitle, nd.x + card.titleLeft, nd.y + 17, textPri);
      }

      const choicesTop = nd.y + card.headerH;
      this.pushSolidQuad(nd.x + 1, choicesTop, nd.w - 2, nd.h - card.headerH - 1, theme.borderSubtle);

      if (nd.hasChoices && nd.ports.length > 0) {
        for (let j = 0; j < nd.ports.length; j++) {
          this.pushNodeChoiceRowQuads(nd, j, isHistoryPreview, false, theme);
        }
      }

      const footerTop = nd.y + nd.h - card.footerH;
      this.pushSolidQuad(nd.x + 1, footerTop + 1, nd.w - 2, card.footerH - 2, cardBg);
      const btnY = footerTop + card.pad;
      if (!isHistoryPreview) {
        this.pushCenteredFooterSprite(NodeCanvasHelpers.getNodeFooterBtnX(nd, ht.addBtn, false), btnY, branchPlusSp, textSec);
        this.pushCenteredFooterSprite(NodeCanvasHelpers.getNodeFooterBtnX(nd, ht.deleteBtn, false), btnY, trashSp, textSec);
      }
      this.pushCenteredFooterSprite(NodeCanvasHelpers.getNodeFooterBtnX(nd, ht.editBtn, isHistoryPreview), btnY, editPenSp, textSec);
    }

    for (let i = 0; i < chapters.length; i++) {
      const ch = chapters[i];
      if (!ch.hidden && ch.right + 12 >= fMinX && ch.left - 12 <= fMaxX && ch.bottom + 12 >= fMinY && ch.top - 12 <= fMaxY) {
        this.pushChapterPortsQuads(ch, theme);
      }
    }

    for (let i = 0; i < nodes.length; i++) {
      const nd = nodes[i];
      if (nd.right + 12 >= fMinX && nd.left - 12 <= fMaxX && nd.bottom + 12 >= fMinY && nd.top - 12 <= fMaxY) {
        this.pushNodePortsQuads(nd, theme);
      }
    }

    studioGL.staticInstCount = studioGL.instCursor;
  },

  appendDynamicOverlayQuads(theme) {
    const studioGL = this.studioGL, gState = NodeCanvasLayout.studioGraphState;
    const uv = NodeCanvasStyle.uv, gridSize = NodeCanvasStyle.grid.size;
    const ht = NodeCanvasHelpers.hitTypes, gt = NodeCanvasHelpers.gestures;
    studioGL.instCursor = studioGL.staticInstCount;

    const hovered = gState.hoveredItem;
    const gesture = NodeCanvasHelpers.activeCanvasGesture;
    const gripSp = studioGL.uv.grip;

    if (gesture && gesture.type === gt.chapterDrag && gesture.chapter && gesture.dragConfirmed) {
      const ch = gesture.chapter;
      this.pushSolidBorderQuads(ch.x, ch.y, ch.w, ch.h, 1, theme.textPrimary);
      this.pushChapterPortsQuads(ch, theme);
    }

    const isNodeDragging = Boolean(gesture && gesture.type === gt.nodeDrag);
    const nodeHighlightColor = isNodeDragging ? theme.textPrimary : theme.textSecondary;
    NodeCanvasHelpers.selectedNodeIds.forEach((selId) => {
      const nd = gState.nodeById.get(selId);
      if (!nd) return;
      this.pushSolidBorderQuads(nd.x, nd.y, nd.w, nd.h, 1, nodeHighlightColor);
      this.pushNodePortsQuads(nd, theme);
    });

    if (gesture && (gesture.type === gt.wireConnect || gesture.type === gt.chapterWireConnect)) {
      const ptsTemp = NodeCanvasRouter.simplifyOrthogonalPoints(
        NodeCanvasRouter.buildWireWaypoints({
          x1: gesture.startX,
          y1: gesture.startY,
          x2: gesture.curX,
          y2: gesture.curY,
          isBottomPort: Boolean(gesture.isBottomPort)
        })
      );
      this.pushWirePolylineQuads(ptsTemp, 2, true, theme.textPrimary);
    }

    if (hovered) {
      if (hovered.type === ht.sceneOutPort) {
        this.pushPortQuads(hovered.x, hovered.y, this.isHoveredPortItemConnected(hovered), true, theme, false, Boolean(hovered.port && hovered.port.isConditional));
      } else if (hovered.type === ht.chapterOutPort) {
        this.pushSmallPortQuads(hovered.x, hovered.y, this.isHoveredPortItemConnected(hovered), true, theme, false);
      } else if (hovered.type === ht.wirePin && hovered.wire && hovered.pinIdx !== undefined) {
        this.pushWirePinArrowQuads(hovered.wire, hovered.pinIdx, true, theme, false);
      } else if (hovered.type === ht.nodeGrip && hovered.node) {
        const nd = hovered.node;
        this.pushQuadInstance(nd.x + 4, nd.y + 6, gripSp.w, gripSp.h, gripSp.u0 + uv.solid, gripSp.v0, gripSp.u1 + uv.solid, gripSp.v1, theme.textPrimary);
      }
    }

    if (gesture && gesture.type === gt.nodeDrag && gesture.moved && gesture.ghostState) {
      const gs = gesture.ghostState;
      const cells = gs.collisionCells || [];
      for (let i = 0; i < cells.length; i++) {
        const cell = cells[i];
        this.pushSolidQuad(cell.x + 1, cell.y + 1, gridSize - 2, gridSize - 2, theme.collisionCellFill);
        this.pushSolidBorderQuads(cell.x, cell.y, gridSize, gridSize, 1, theme.collisionCellBorder);
      }
      const rects = gs.rects || [];
      const borderColor = gs.valid ? theme.ghostValidBorder : theme.ghostInvalidBorder;
      for (let i = 0; i < rects.length; i++) {
        const gr = rects[i];
        this.pushSolidQuad(gr.x, gr.y, gr.w, gr.h, theme.ghostFill);
        this.pushDashedRectQuads(gr.x, gr.y, gr.w, gr.h, 1.5, 8.0, borderColor);
      }
    }

    if (gesture && gesture.type === gt.marqueeSelect && gesture.moved && gesture.marqueeRect) {
      const mr = gesture.marqueeRect;
      this.pushQuadInstance(mr.x, mr.y, mr.w, mr.h, uv.screen, 0.0, uv.screen, 0.0, theme.marqueeFill);
      const t = 1;
      this.pushQuadInstance(mr.x, mr.y, mr.w, t, uv.screen, 0.0, uv.screen, 0.0, theme.textPrimary);
      this.pushQuadInstance(mr.x, mr.y + mr.h - t, mr.w, t, uv.screen, 0.0, uv.screen, 0.0, theme.textPrimary);
      this.pushQuadInstance(mr.x, mr.y, t, mr.h, uv.screen, 0.0, uv.screen, 0.0, theme.textPrimary);
      this.pushQuadInstance(mr.x + mr.w - t, mr.y, t, mr.h, uv.screen, 0.0, uv.screen, 0.0, theme.textPrimary);
    }

    studioGL.totalInstCount = studioGL.instCursor;
  },

  render() {
    const t = this.tokens;
    const sceneModal = document.querySelector(t.textEditorSelector);
    if (sceneModal && (sceneModal.classList.contains(t.modeFullClass) || sceneModal.classList.contains(t.hasRightPanelClass))) {
      return;
    }
    const studioGL = this.studioGL, gState = NodeCanvasLayout.studioGraphState, state = NodeCanvasController.data;
    let canvas = document.querySelector(t.canvasSelector);
    if (!canvas) return;
    if (studioGL.canvas && canvas !== studioGL.canvas && canvas.parentElement) {
      canvas.parentElement.replaceChild(studioGL.canvas, canvas);
      canvas = studioGL.canvas;
    }
    const viewport = document.querySelector(t.viewportSelector);
    if (viewport) {
      const vRect = viewport.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      const vw = Math.max(320, Math.round(vRect.width || 1200));
      const vh = Math.max(240, Math.round(vRect.height || 700));
      studioGL.viewW = vw;
      studioGL.viewH = vh;
      studioGL.canvasDpr = dpr;
      const targetW = Math.round(vw * dpr), targetH = Math.round(vh * dpr);
      if (canvas.width !== targetW || canvas.height !== targetH) {
        canvas.width = targetW;
        canvas.height = targetH;
      }
    }
    const gl = this.initContext(canvas);
    if (!gl) return;

    if (studioGL.uiAtlasImgLoaded && !studioGL.uiAtlasUploaded && studioGL.uiAtlasImg) {
      gl.activeTexture(gl.TEXTURE0);
      gl.bindTexture(gl.TEXTURE_2D, studioGL.uiTex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, studioGL.uiAtlasImg);
      studioGL.uiAtlasUploaded = true;
    }
    if (studioGL.atlasImgLoaded && !studioGL.atlasUploaded && studioGL.atlasImg) {
      gl.activeTexture(gl.TEXTURE1);
      gl.bindTexture(gl.TEXTURE_2D, studioGL.fontTex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, studioGL.atlasImg);
      studioGL.atlasUploaded = true;
    }

    const theme = this.resolveThemeColors();
    const vw = studioGL.viewW || 1200, vh = studioGL.viewH || 700, dpr = studioGL.canvasDpr || 2.0;
    const z = state.zoom || 1.0, px = state.panX || 0, py = state.panY || 0;
    const visMinX = -px / z, visMinY = -py / z, visMaxX = (vw - px) / z, visMaxY = (vh - py) / z;

    if (!gState.staticBatchDirty && (!studioGL.hasCullBounds || visMinX < studioGL.cullMinX || visMaxX > studioGL.cullMaxX || visMinY < studioGL.cullMinY || visMaxY > studioGL.cullMaxY)) {
      gState.staticBatchDirty = true;
    }

    const fpi = this.floatsPerInstance, bpi = this.bytesPerInstance;
    if (gState.staticBatchDirty) {
      this.compileStaticSceneQuads(theme);
      this.appendDynamicOverlayQuads(theme);
      const ringIdx = ((studioGL.vboRingIdx || 0) + 1) % 3;
      studioGL.vboRingIdx = ringIdx;
      const vbo = studioGL.instVbos[ringIdx];
      studioGL.bindInstVboAttribs(vbo);
      const activeFloats = studioGL.totalInstCount * fpi;
      const requiredBytes = (studioGL.totalInstCount + 64) * bpi;
      if (studioGL.instVboCapacities[ringIdx] < requiredBytes) {
        const nextCapBytes = Math.max(requiredBytes * 2, 4096 * bpi);
        gl.bufferData(gl.ARRAY_BUFFER, nextCapBytes, gl.STREAM_DRAW);
        studioGL.instVboCapacities[ringIdx] = nextCapBytes;
      }
      gl.bufferSubData(gl.ARRAY_BUFFER, 0, this.instanceFloatView.subarray(0, activeFloats));
      gState.staticBatchDirty = false;
      gState.overlayBatchDirty = false;
    } else if (gState.overlayBatchDirty) {
      this.appendDynamicOverlayQuads(theme);
      const dynCount = studioGL.totalInstCount - studioGL.staticInstCount;
      if (dynCount > 0) {
        const ringIdx = studioGL.vboRingIdx || 0;
        const vbo = studioGL.instVbos[ringIdx];
        const startFloat = studioGL.staticInstCount * fpi;
        const endFloat = studioGL.totalInstCount * fpi;
        const requiredBytes = studioGL.totalInstCount * bpi;
        gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
        if (studioGL.instVboCapacities[ringIdx] < requiredBytes) {
          const nextCapBytes = Math.max(requiredBytes * 2, 4096 * bpi);
          gl.bufferData(gl.ARRAY_BUFFER, nextCapBytes, gl.STREAM_DRAW);
          studioGL.instVboCapacities[ringIdx] = nextCapBytes;
          gl.bufferSubData(gl.ARRAY_BUFFER, 0, this.instanceFloatView.subarray(0, endFloat));
        } else {
          gl.bufferSubData(gl.ARRAY_BUFFER, studioGL.staticInstCount * bpi, this.instanceFloatView.subarray(startFloat, endFloat));
        }
      }
      gState.overlayBatchDirty = false;
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
    const gridSize = NodeCanvasStyle.grid.size;
    const gridStep = z < 0.45 ? gridSize * 2 : gridSize;
    if (studioGL.lastUniformGridStep !== gridStep) {
      gl.uniform1f(studioGL.uGridStep, gridStep);
      studioGL.lastUniformGridStep = gridStep;
    }

    if (studioGL.instExt) {
      studioGL.instExt.drawArraysInstancedANGLE(gl.TRIANGLES, 0, 6, studioGL.totalInstCount);
    }
  }
};
