"use strict";

const EditorVoiceWaveform = {
  moduleName: "EditorVoiceWaveform",
  peakCache: new Map(),

  selectors: {
    canvas: ".voice-rec-waveform-canvas",
    playhead: ".voice-rec-playhead",
    trimStart: ".voice-rec-trim-handle.trim-start",
    trimEnd: ".voice-rec-trim-handle.trim-end",
    timer: ".voice-rec-timer"
  },

  classes: {
    visible: "is-visible"
  },

  attrs: {
    theme: "data-theme"
  },

  values: {
    undefType: "undefined"
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
    defaultBars: 160,
    peakDigitPairLen: 2,
    peakMaxScale: 99,
    radixDec: 10,
    minAudioAmpFloor: 0.12,
    minPeakVal: 0.005,
    minBarHeight: 2,
    barPadTotal: 2,
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

  parseCompactPeaks(peaksSpec) {
    const metrics = this.metrics;
    if (!peaksSpec || peaksSpec.length < metrics.peakDigitPairLen) return [];
    const parsedPeaks = [];
    for (let charIdx = 0; charIdx + 1 < peaksSpec.length; charIdx += metrics.peakDigitPairLen) {
      const rawVal = parseInt(peaksSpec.slice(charIdx, charIdx + metrics.peakDigitPairLen), metrics.radixDec);
      parsedPeaks.push(
        isNaN(rawVal)
          ? metrics.minRatio
          : Math.max(metrics.minRatio, Math.min(metrics.maxRatio, rawVal / metrics.peakMaxScale))
      );
    }
    return parsedPeaks;
  },

  resolvePeaks(stripElement, trimState, captureState) {
    if (trimState.stripEl === stripElement && captureState.peaks.length > 0) {
      return captureState.peaks;
    }
    const meta = EditorVoice.resolveStripMeta();
    const cacheKey = meta.audioUrl || meta.voicePeaks;
    if (cacheKey && this.peakCache.has(cacheKey)) {
      const cachedPeaks = this.peakCache.get(cacheKey);
      if (cachedPeaks && cachedPeaks.length > 0) {
        return cachedPeaks;
      }
    }
    if (meta.voicePeaks) {
      const parsedPeaks = this.parseCompactPeaks(meta.voicePeaks);
      if (parsedPeaks.length > 0) {
        if (cacheKey) this.peakCache.set(cacheKey, parsedPeaks);
        return parsedPeaks;
      }
    }
    return [];
  },

  syncPlayhead(stripElement, playheadRatio) {
    if (!stripElement) return;
    const metrics = this.metrics;
    const playheadElement = stripElement.querySelector(this.selectors.playhead);
    if (!playheadElement) return;
    if (playheadRatio >= metrics.minRatio && playheadRatio <= metrics.maxRatio) {
      playheadElement.style.left = (playheadRatio * metrics.fullPct).toFixed(metrics.pctDecimals) + this.theme.pctSuffix;
      playheadElement.classList.add(this.classes.visible);
    } else {
      playheadElement.classList.remove(this.classes.visible);
    }
  },

  syncHandles(stripElement, trimState, playbackState, captureState) {
    if (!stripElement) return;
    const metrics = this.metrics;
    const startPct = Math.max(metrics.minStartPct, Math.min(metrics.maxStartPct, trimState.trimStart * metrics.fullPct));
    const endPct = Math.max(startPct + metrics.minSpanPct, Math.min(metrics.fullPct, trimState.trimEnd * metrics.fullPct));
    const startHandle = stripElement.querySelector(this.selectors.trimStart);
    const endHandle = stripElement.querySelector(this.selectors.trimEnd);
    if (startHandle) startHandle.style.left = startPct.toFixed(metrics.pctDecimals) + this.theme.pctSuffix;
    if (endHandle) endHandle.style.left = endPct.toFixed(metrics.pctDecimals) + this.theme.pctSuffix;
    const timerElement = stripElement.querySelector(this.selectors.timer);
    if (timerElement && playbackState.durationSec > metrics.minRatio && !captureState.recording) {
      timerElement.textContent = AudioDeckController.formatSeconds(
        playbackState.durationSec * ((endPct - startPct) / metrics.fullPct)
      );
    }
    EditorVoice.syncStripState(stripElement);
  },

  draw(stripElement, trimState, captureState, playheadRatio) {
    if (!stripElement) return;
    const metrics = this.metrics;
    const themeSpec = this.theme;
    const canvasElement = stripElement.querySelector(this.selectors.canvas);
    if (!canvasElement || !canvasElement.getContext) return;
    const canvasRect = canvasElement.getBoundingClientRect();
    const pixelRatio = window.devicePixelRatio || metrics.defaultDpr;
    const cssWidth = Math.max(metrics.minWidth, Math.round(canvasRect.width || metrics.defaultWidth));
    const cssHeight = Math.max(metrics.minHeight, Math.round(canvasRect.height || metrics.defaultHeight));
    const targetWidth = Math.round(cssWidth * pixelRatio);
    const targetHeight = Math.round(cssHeight * pixelRatio);
    if (canvasElement.width !== targetWidth || canvasElement.height !== targetHeight) {
      canvasElement.width = targetWidth;
      canvasElement.height = targetHeight;
    }
    const context2d = canvasElement.getContext(themeSpec.ctx2d);
    if (!context2d) return;
    context2d.save();
    context2d.scale(pixelRatio, pixelRatio);
    context2d.clearRect(metrics.minRatio, metrics.minRatio, cssWidth, cssHeight);

    const isDarkTheme = !document.body || document.body.getAttribute(this.attrs.theme) !== themeSpec.light;
    const activeColor = isDarkTheme ? themeSpec.darkActive : themeSpec.lightActive;
    const mutedColor = isDarkTheme ? themeSpec.darkMuted : themeSpec.lightMuted;
    const recordColor = themeSpec.record;
    const midY = cssHeight * metrics.half;
    context2d.fillStyle = mutedColor;
    context2d.fillRect(metrics.minRatio, Math.round(midY), cssWidth, metrics.centerLineH);

    const peaks = this.resolvePeaks(stripElement, trimState, captureState);
    if (peaks.length > 0) {
      const maxBars = Math.max(metrics.minBars, Math.floor(cssWidth / metrics.barStep));
      const isLiveRecording = trimState.stripEl === stripElement && captureState.recording;
      for (let barIndex = 0; barIndex < maxBars; barIndex++) {
        let barValue = 0;
        if (isLiveRecording) {
          const peakIndex = peaks.length - maxBars + barIndex;
          if (peakIndex >= 0 && peakIndex < peaks.length) barValue = peaks[peakIndex];
        } else {
          const startIndex = Math.floor((barIndex / maxBars) * peaks.length);
          const endIndex = Math.max(
            startIndex + metrics.minBars,
            Math.floor(((barIndex + metrics.minBars) / maxBars) * peaks.length)
          );
          for (let sampleIndex = startIndex; sampleIndex < endIndex && sampleIndex < peaks.length; sampleIndex++) {
            if (peaks[sampleIndex] > barValue) barValue = peaks[sampleIndex];
          }
        }
        if (barValue <= metrics.minPeakVal) continue;
        const xPosition = barIndex * metrics.barStep;
        const ratio = xPosition / cssWidth;
        const inTrimRange = isLiveRecording || (ratio >= trimState.trimStart && ratio <= trimState.trimEnd);
        context2d.fillStyle = isLiveRecording ? recordColor : inTrimRange ? activeColor : mutedColor;
        const maxBarHeight = cssHeight - metrics.barPadTotal;
        const barHeight = Math.max(metrics.minBarHeight, Math.min(maxBarHeight, Math.round(barValue * maxBarHeight)));
        context2d.fillRect(xPosition, Math.round(midY - barHeight * metrics.half), metrics.barWidth, barHeight);
      }
    }
    context2d.restore();
    this.syncPlayhead(stripElement, playheadRatio);
  },

  loadPeaks(stripElement, trimState, captureState) {
    if (!stripElement) return;
    const metrics = this.metrics;
    const peaks = this.resolvePeaks(stripElement, trimState, captureState);
    this.draw(stripElement, trimState, captureState, metrics.playheadOff);
    if (peaks.length > 0) return;
    const meta = EditorVoice.resolveStripMeta();
    if (
      !meta.audioUrl ||
      this.peakCache.has(meta.audioUrl) ||
      typeof window === this.values.undefType ||
      !window.fetch
    ) {
      return;
    }
    const AudioContextClass = window.AudioContext || window.webkitAudioContext;
    if (!AudioContextClass) return;
    this.peakCache.set(meta.audioUrl, []);
    window
      .fetch(meta.audioUrl)
      .then((fetchResponse) => (fetchResponse && fetchResponse.ok ? fetchResponse.arrayBuffer() : null))
      .then((arrayBuffer) => {
        if (!arrayBuffer) return null;
        const audioContext = new AudioContextClass();
        return audioContext.decodeAudioData(arrayBuffer).finally(() => {
          if (audioContext.close) audioContext.close().catch(() => {});
        });
      })
      .then((audioBuffer) => {
        if (!audioBuffer || !audioBuffer.length) return;
        const channelData = audioBuffer.getChannelData(0);
        const barCount = metrics.defaultBars;
        const rawPeaks = new Array(barCount);
        let maxAmplitude = metrics.minAudioAmpFloor;
        for (let barIndex = 0; barIndex < barCount; barIndex++) {
          const startIndex = Math.floor((barIndex * channelData.length) / barCount);
          const endIndex = Math.max(startIndex + 1, Math.floor(((barIndex + 1) * channelData.length) / barCount));
          let peakValue = 0;
          for (let sampleIndex = startIndex; sampleIndex < endIndex && sampleIndex < channelData.length; sampleIndex++) {
            const sampleAmplitude = Math.abs(channelData[sampleIndex]);
            if (sampleAmplitude > peakValue) peakValue = sampleAmplitude;
          }
          rawPeaks[barIndex] = peakValue;
          if (peakValue > maxAmplitude) maxAmplitude = peakValue;
        }
        const normalizedPeaks = rawPeaks.map((peakValue) => Math.min(metrics.maxRatio, peakValue / maxAmplitude));
        this.peakCache.set(meta.audioUrl, normalizedPeaks);
        this.draw(stripElement, trimState, captureState, metrics.playheadOff);
      })
      .catch(() => {});
  }
};
Dispatcher.register(EditorVoiceWaveform.moduleName, EditorVoiceWaveform);
