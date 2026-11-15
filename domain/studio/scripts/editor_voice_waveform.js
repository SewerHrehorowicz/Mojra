"use strict";

const EditorVoiceWaveform = {
  moduleName: "EditorVoiceWaveform",
  peakCache: new Map(),

  selectors: {
    canvas: "[data-voice-canvas]",
    trimStart: '[data-voice-trim="start"]',
    trimEnd: '[data-voice-trim="end"]',
    timer: "[data-voice-timer]"
  },

  attrs: {
    url: "data-voice-url",
    paramUrl: "param-voice-url",
    theme: "data-theme"
  },

  theme: {
    light: "light",
    ctx2d: "2d",
    pctSuffix: "%",
    darkActive: "#e6e1d6",
    lightActive: "#1c1b18",
    darkMuted: "rgba(230,225,214,0.22)",
    lightMuted: "rgba(28,27,24,0.20)",
    record: "#d94a38"
  },

  metrics: {
    minWidth: 120,
    defaultWidth: 240,
    minHeight: 22,
    defaultHeight: 26,
    defaultDpr: 1,
    half: 0.5,
    centerLineH: 1,
    barWidth: 2,
    barStep: 3,
    minBars: 1,
    minPeakVal: 0.005,
    minBarHeight: 2,
    barPadTotal: 2,
    playheadWidth: 1.5,
    playheadOff: -1,
    minRatio: 0,
    maxRatio: 1,
    trimMinThreshold: 0.003,
    trimMaxThreshold: 0.997,
    minStartPct: 0,
    maxStartPct: 99,
    minSpanPct: 1,
    fullPct: 100,
    pctDecimals: 2
  },

  isTrimActive(startRatio, endRatio) {
    return (
      startRatio > this.metrics.trimMinThreshold ||
      (endRatio > this.metrics.minRatio && endRatio < this.metrics.trimMaxThreshold)
    );
  },

  syncHandles(stripEl, trimState, playback, capture) {
    if (!stripEl) return;
    const m = this.metrics;
    const startPct = Math.max(m.minStartPct, Math.min(m.maxStartPct, trimState.trimStart * m.fullPct));
    const endPct = Math.max(startPct + m.minSpanPct, Math.min(m.fullPct, trimState.trimEnd * m.fullPct));
    const startHandle = stripEl.querySelector(this.selectors.trimStart);
    const endHandle = stripEl.querySelector(this.selectors.trimEnd);
    if (startHandle) startHandle.style.left = startPct.toFixed(m.pctDecimals) + this.theme.pctSuffix;
    if (endHandle) endHandle.style.left = endPct.toFixed(m.pctDecimals) + this.theme.pctSuffix;
    const timeEl = stripEl.querySelector(this.selectors.timer);
    if (timeEl && playback.durationSec > m.minRatio && !capture.recording) {
      timeEl.textContent = AudioDeckController.formatSeconds(playback.durationSec * ((endPct - startPct) / m.fullPct));
    }
    EditorVoice.syncStripState(stripEl);
  },

  draw(stripEl, trimState, capture, playheadRatio) {
    if (!stripEl) return;
    const canvas = stripEl.querySelector(this.selectors.canvas);
    if (!canvas || !canvas.getContext) return;
    const m = this.metrics;
    const th = this.theme;
    const rect = canvas.getBoundingClientRect();
    const dpr = window.devicePixelRatio || m.defaultDpr;
    const cssW = Math.max(m.minWidth, Math.round(rect.width || m.defaultWidth));
    const cssH = Math.max(m.minHeight, Math.round(rect.height || m.defaultHeight));
    const targetW = Math.round(cssW * dpr);
    const targetH = Math.round(cssH * dpr);
    if (canvas.width !== targetW || canvas.height !== targetH) {
      canvas.width = targetW;
      canvas.height = targetH;
    }
    const ctx = canvas.getContext(th.ctx2d);
    if (!ctx) return;
    ctx.save();
    ctx.scale(dpr, dpr);
    ctx.clearRect(m.minRatio, m.minRatio, cssW, cssH);

    const isDark = !document.body || document.body.getAttribute(this.attrs.theme) !== th.light;
    const fgActive = isDark ? th.darkActive : th.lightActive;
    const fgMuted = isDark ? th.darkMuted : th.lightMuted;
    const fgRec = th.record;
    const midY = cssH * m.half;
    ctx.fillStyle = fgMuted;
    ctx.fillRect(m.minRatio, Math.round(midY), cssW, m.centerLineH);

    const audioUrl = (stripEl.getAttribute(this.attrs.url) || stripEl.getAttribute(this.attrs.paramUrl) || "").trim();
    const cached = audioUrl ? this.peakCache.get(audioUrl) : null;
    const peaks = trimState.stripEl === stripEl && capture.peaks.length > 0 ? capture.peaks : cached || [];
    if (peaks.length > 0) {
      const maxBars = Math.max(m.minBars, Math.floor(cssW / m.barStep));
      const isLiveRec = trimState.stripEl === stripEl && capture.recording;
      for (let b = 0; b < maxBars; b++) {
        let val = 0;
        if (isLiveRec) {
          const idx = peaks.length - maxBars + b;
          if (idx >= 0 && idx < peaks.length) val = peaks[idx];
        } else {
          const i0 = Math.floor((b / maxBars) * peaks.length);
          const i1 = Math.max(i0 + m.minBars, Math.floor(((b + m.minBars) / maxBars) * peaks.length));
          for (let j = i0; j < i1 && j < peaks.length; j++) {
            if (peaks[j] > val) val = peaks[j];
          }
        }
        if (val <= m.minPeakVal) continue;
        const x = b * m.barStep;
        const ratio = x / cssW;
        const inTrim = isLiveRec || (ratio >= trimState.trimStart && ratio <= trimState.trimEnd);
        ctx.fillStyle = isLiveRec ? fgRec : inTrim ? fgActive : fgMuted;
        const maxBarH = cssH - m.barPadTotal;
        const bh = Math.max(m.minBarHeight, Math.min(maxBarH, Math.round(val * maxBarH)));
        ctx.fillRect(x, Math.round(midY - bh * m.half), m.barWidth, bh);
      }
    }
    if (playheadRatio >= m.minRatio && playheadRatio <= m.maxRatio) {
      ctx.fillStyle = fgRec;
      ctx.fillRect(Math.round(playheadRatio * cssW), m.minRatio, m.playheadWidth, cssH);
    }
    ctx.restore();
  },

  loadPeaks(stripEl, trimState, capture) {
    if (!stripEl) return;
    this.draw(stripEl, trimState, capture, this.metrics.playheadOff);
  }
};
Dispatcher.register(EditorVoiceWaveform.moduleName, EditorVoiceWaveform);
