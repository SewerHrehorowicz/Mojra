"use strict";

const voiceAudioState = {
  sceneId: "",
  voiceUrl: "",
  sampleRate: 24000,
  durationSec: 0,
  peaks: null,
  pcmSamplesInt16: null,
  startRatio: 0,
  endRatio: 1,
  playheadRatio: 0,
  isRecording: false,
  isNativeCapture: false,
  nativePollTimer: null,
  isPlaying: false,
  recStartTime: 0,
  recChunks: [],
  recTotalSamples: 0,
  recLivePeaks: [],
  mediaStream: null,
  audioCtx: null,
  scriptNode: null,
  sourceNode: null,
  playbackAudio: null,
  playbackCtx: null,
  playbackBufSource: null,
  playbackStartCtxTime: 0,
  playbackStartRatio: 0,
  playbackEndRatio: 1,
  autoRecordOnNextOpenScene: "",
  rafId: 0,
  dragMode: "", // "start" | "end" | "playhead"
  dragStripEl: null,
  dragMoved: false
};

const voiceDecodedCache = new Map();

function formatVoiceSeconds(sec) {
  const s = Math.max(0, Math.round(Number(sec) || 0));
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return m + ":" + (rem < 10 ? "0" + rem : String(rem));
}

function uint8ToBase64(bytes) {
  let binary = "";
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    const sub = bytes.subarray(i, Math.min(i + chunk, bytes.length));
    binary += String.fromCharCode.apply(null, sub);
  }
  return btoa(binary);
}

function int16ArrayToLEBase64(samples, startRatio, endRatio) {
  if (!samples || samples.length === 0) return "";
  const total = samples.length;
  const sIdx = Math.max(0, Math.min(total - 1, Math.floor(total * (startRatio || 0))));
  const eIdx = Math.max(sIdx + 1, Math.min(total, Math.ceil(total * (endRatio || 1))));
  const count = eIdx - sIdx;
  const bytes = new Uint8Array(count * 2);
  for (let i = 0; i < count; i++) {
    const v = samples[sIdx + i];
    bytes[i * 2] = v & 0xff;
    bytes[i * 2 + 1] = (v >> 8) & 0xff;
  }
  return uint8ToBase64(bytes);
}

function computePeaksFromInt16(samples, numBars) {
  const bars = Math.max(16, numBars || 160);
  const out = new Float32Array(bars);
  if (!samples || samples.length === 0) return out;
  const step = samples.length / bars;
  let globalPeak = 0.01;
  for (let b = 0; b < bars; b++) {
    const i0 = Math.floor(b * step);
    const i1 = Math.max(i0 + 1, Math.min(samples.length, Math.floor((b + 1) * step)));
    let maxAbs = 0;
    for (let i = i0; i < i1; i++) {
      const a = Math.abs(samples[i]) / 32768;
      if (a > maxAbs) maxAbs = a;
    }
    out[b] = maxAbs;
    if (maxAbs > globalPeak) globalPeak = maxAbs;
  }
  const norm = globalPeak > 0.05 ? 1 / globalPeak : 1;
  for (let b = 0; b < bars; b++) {
    out[b] = Math.min(1, out[b] * norm);
  }
  return out;
}

function readFLACUTF8Len(view, offset) {
  if (offset >= view.byteLength) return 0;
  const b0 = view.getUint8(offset);
  if ((b0 & 0x80) === 0) return 1;
  if ((b0 & 0xe0) === 0xc0) return 2;
  if ((b0 & 0xf0) === 0xe0) return 3;
  if ((b0 & 0xf8) === 0xf0) return 4;
  return 0;
}

function parseFLACMono16FromArrayBuffer(buf) {
  if (!buf || buf.byteLength < 42) return null;
  const view = new DataView(buf);
  if (
    view.getUint8(0) !== 0x66 ||
    view.getUint8(1) !== 0x4c ||
    view.getUint8(2) !== 0x61 ||
    view.getUint8(3) !== 0x43
  ) {
    return null;
  }
  let pos = 4;
  let sampleRate = 24000;
  let totalSamples = 0;
  while (pos + 4 <= view.byteLength) {
    const hdr = view.getUint8(pos);
    const isLast = (hdr & 0x80) !== 0;
    const blockType = hdr & 0x7f;
    const blockLen =
      (view.getUint8(pos + 1) << 16) |
      (view.getUint8(pos + 2) << 8) |
      view.getUint8(pos + 3);
    pos += 4;
    if (pos + blockLen > view.byteLength) return null;
    if (blockType === 0 && blockLen >= 34) {
      const srHi = view.getUint16(pos + 10, false);
      const srLo = view.getUint8(pos + 12) >> 4;
      const sr = (srHi << 4) | srLo;
      if (sr > 0) sampleRate = sr;
      const totHi = view.getUint8(pos + 13) & 0x0f;
      const totLo = view.getUint32(pos + 14, false);
      totalSamples = totHi * 0x100000000 + totLo;
    }
    pos += blockLen;
    if (isLast) break;
  }

  const out = [];
  while (pos + 6 < view.byteLength) {
    if (view.getUint8(pos) !== 0xff || (view.getUint8(pos + 1) & 0xfe) !== 0xf8) {
      break;
    }
    const bsCode = (view.getUint8(pos + 2) >> 4) & 0x0f;
    const srCode = view.getUint8(pos + 2) & 0x0f;
    const utf8Len = readFLACUTF8Len(view, pos + 4);
    if (utf8Len <= 0) break;
    let cur = pos + 4 + utf8Len;
    let blockLen = 4096;
    if (bsCode === 0x06) {
      if (cur + 1 > view.byteLength) break;
      blockLen = view.getUint8(cur) + 1;
      cur += 1;
    } else if (bsCode === 0x07) {
      if (cur + 2 > view.byteLength) break;
      blockLen = view.getUint16(cur, false) + 1;
      cur += 2;
    }
    if (srCode === 0x0c) {
      cur += 1;
    } else if (srCode === 0x0d || srCode === 0x0e) {
      if (cur + 2 <= view.byteLength && srCode === 0x0d) {
        const sr = view.getUint16(cur, false);
        if (sr > 0) sampleRate = sr;
      }
      cur += 2;
    }
    cur += 1; // CRC-8
    if (cur >= view.byteLength) break;
    const subHdr = view.getUint8(cur);
    cur += 1;
    const subType = (subHdr >> 1) & 0x3f;
    if (subType === 0x00) {
      if (cur + 2 > view.byteLength) break;
      const val = view.getInt16(cur, false);
      cur += 2;
      for (let i = 0; i < blockLen; i++) out.push(val);
    } else if (subType === 0x01) {
      if (cur + blockLen * 2 > view.byteLength) break;
      for (let i = 0; i < blockLen; i++) {
        out.push(view.getInt16(cur, false));
        cur += 2;
      }
    } else {
      return null;
    }
    cur += 2; // CRC-16
    pos = cur;
  }
  if (out.length === 0) return null;
  const int16 = new Int16Array(out);
  const dur = sampleRate > 0 ? int16.length / sampleRate : 0;
  return {
    sampleRate: sampleRate,
    durationSec: dur,
    samples: int16,
    peaks: computePeaksFromInt16(int16, 170)
  };
}

function getAllActiveVoiceStrips() {
  return qsa(".voice-rec-strip").filter((el) => !el.classList.contains("hidden"));
}

function syncVoiceStripDOM(stripEl) {
  if (!stripEl) return;
  stripEl.classList.toggle("is-recording", voiceAudioState.isRecording);

  const playIcon = qs(".voice-rec-icon-play", stripEl);
  const micIcon = qs(".voice-rec-icon-mic", stripEl);
  const pauseIcon = qs(".voice-rec-icon-pause", stripEl);
  const stopIcon = qs(".voice-rec-icon-stop", stripEl);
  const hasAudio = Boolean(
    (stripEl.getAttribute("data-voice-url") || "").trim() ||
      (voiceAudioState.pcmSamplesInt16 && voiceAudioState.pcmSamplesInt16.length > 0)
  );
  if (voiceAudioState.isRecording) {
    if (playIcon) playIcon.classList.add("hidden");
    if (micIcon) micIcon.classList.add("hidden");
    if (pauseIcon) pauseIcon.classList.add("hidden");
    if (stopIcon) stopIcon.classList.remove("hidden");
  } else if (voiceAudioState.isPlaying) {
    if (playIcon) playIcon.classList.add("hidden");
    if (micIcon) micIcon.classList.add("hidden");
    if (pauseIcon) pauseIcon.classList.remove("hidden");
    if (stopIcon) stopIcon.classList.add("hidden");
  } else {
    if (playIcon) playIcon.classList.toggle("hidden", !hasAudio);
    if (micIcon) micIcon.classList.toggle("hidden", hasAudio);
    if (pauseIcon) pauseIcon.classList.add("hidden");
    if (stopIcon) stopIcon.classList.add("hidden");
  }

  const startHandle = qs('.voice-rec-trim-handle[data-voice-trim="start"]', stripEl);
  const endHandle = qs('.voice-rec-trim-handle[data-voice-trim="end"]', stripEl);
  const playheadEl = qs(".voice-rec-playhead", stripEl);

  if (startHandle) {
    startHandle.style.left = (voiceAudioState.startRatio * 100).toFixed(2) + "%";
  }
  if (endHandle) {
    endHandle.style.left = (voiceAudioState.endRatio * 100).toFixed(2) + "%";
  }
  if (playheadEl) {
    playheadEl.style.left = (voiceAudioState.playheadRatio * 100).toFixed(2) + "%";
  }

  const timerEl = qs(".voice-rec-timer", stripEl);
  if (timerEl) {
    if (voiceAudioState.isRecording) {
      const elapsed = (Date.now() - voiceAudioState.recStartTime) / 1000;
      timerEl.textContent = formatVoiceSeconds(elapsed);
    } else if (voiceAudioState.durationSec > 0) {
      const trimmedDur =
        (voiceAudioState.endRatio - voiceAudioState.startRatio) * voiceAudioState.durationSec;
      timerEl.textContent = formatVoiceSeconds(trimmedDur);
    }
  }

  drawVoiceRecorderCanvas(stripEl);
}

let cachedVoiceStripEl = null;
let cachedVoiceCanvasEl = null;

function resolveVoiceWaveformCanvas(stripEl) {
  if (!stripEl) return null;
  if (cachedVoiceStripEl === stripEl && cachedVoiceCanvasEl && cachedVoiceCanvasEl.isConnected !== false) {
    return cachedVoiceCanvasEl;
  }
  cachedVoiceStripEl = stripEl;
  cachedVoiceCanvasEl = qs(".voice-rec-waveform-canvas", stripEl);
  return cachedVoiceCanvasEl;
}

function drawVoiceRecorderCanvas(stripEl) {
  const canvas = resolveVoiceWaveformCanvas(stripEl);
  if (!canvas || typeof canvas.getContext !== "function") return;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  const dpr = (typeof window !== "undefined" && window.devicePixelRatio) || 1;
  const cssW = canvas.clientWidth || Number(canvas.getAttribute("width")) || 216;
  const cssH = canvas.clientHeight || Number(canvas.getAttribute("height")) || 28;
  const targetW = Math.max(1, Math.round(cssW * dpr));
  const targetH = Math.max(1, Math.round(cssH * dpr));
  if (canvas.width !== targetW || canvas.height !== targetH) {
    canvas.width = targetW;
    canvas.height = targetH;
  }

  ctx.save();
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, cssW, cssH);

  const isLight =
    typeof document !== "undefined" &&
    document.body &&
    document.body.getAttribute("data-theme") === "light";
  const activeColor = isLight ? "#666666" : "#7a7a7a";
  const dimColor = isLight ? "#d4d4d4" : "#292929";
  const step = 3;
  const numBars = Math.max(1, Math.floor(cssW / step));
  const midY = cssH / 2;

  if (voiceAudioState.isRecording) {
    const livePeaks = voiceAudioState.recLivePeaks;
    const headRatio = Math.min(1, livePeaks.length / numBars);
    voiceAudioState.playheadRatio = headRatio;
    const headX = Math.round(headRatio * cssW);

    ctx.fillStyle = dimColor;
    ctx.fillRect(headX, Math.round(midY) - 0.5, Math.max(0, cssW - headX), 1);

    ctx.fillStyle = activeColor;
    const startIdx = Math.max(0, livePeaks.length - numBars);
    for (let i = 0; i < livePeaks.length - startIdx; i++) {
      const x = i * step + 1;
      const amp = Math.max(2, Math.round(livePeaks[startIdx + i] * (cssH - 4)));
      const half = Math.max(1, Math.floor(amp / 2));
      ctx.fillRect(x, Math.round(midY) - half, 1, half * 2);
    }
    ctx.restore();
    return;
  }

  const peaks = voiceAudioState.peaks;
  for (let b = 0; b < numBars; b++) {
    const ratio = numBars > 1 ? b / (numBars - 1) : 0;
    const x = b * step + 1;
    const inRange =
      ratio >= voiceAudioState.startRatio - 0.001 &&
      ratio <= voiceAudioState.endRatio + 0.001;
    ctx.fillStyle = inRange ? activeColor : dimColor;

    let normAmp = 0.08;
    if (peaks && peaks.length > 0) {
      const pIdx = Math.min(peaks.length - 1, Math.floor(ratio * peaks.length));
      normAmp = peaks[pIdx];
    }
    const barH = Math.max(2, Math.round(normAmp * (cssH - 4)));
    const half = Math.max(1, Math.floor(barH / 2));
    ctx.fillRect(x, Math.round(midY) - half, 1, half * 2);
  }

  ctx.restore();
}

function syncAllVoiceStrips() {
  getAllActiveVoiceStrips().forEach((stripEl) => {
    syncVoiceStripDOM(stripEl);
  });
}

async function loadVoiceStripAudio(stripEl) {
  if (!stripEl || voiceAudioState.isRecording) return;
  const sceneId = (stripEl.getAttribute("data-scene") || "").trim();
  const voiceUrl = (stripEl.getAttribute("data-voice-url") || "").trim();

  if (sceneId !== voiceAudioState.sceneId || voiceUrl !== voiceAudioState.voiceUrl) {
    stopVoicePlayback();
    voiceAudioState.sceneId = sceneId;
    voiceAudioState.voiceUrl = voiceUrl;
    voiceAudioState.startRatio = 0;
    voiceAudioState.endRatio = 1;
    voiceAudioState.playheadRatio = 0;
    voiceAudioState.peaks = null;
    voiceAudioState.pcmSamplesInt16 = null;
    voiceAudioState.durationSec = 0;
  }

  if (!voiceUrl) {
    syncVoiceStripDOM(stripEl);
    return;
  }

  if (voiceDecodedCache.has(voiceUrl)) {
    const cached = voiceDecodedCache.get(voiceUrl);
    voiceAudioState.sampleRate = cached.sampleRate;
    voiceAudioState.durationSec = cached.durationSec;
    voiceAudioState.peaks = cached.peaks;
    voiceAudioState.pcmSamplesInt16 = cached.samples;
    syncAllVoiceStrips();
    return;
  }

  try {
    const resp = await fetch(voiceUrl, { credentials: "same-origin" });
    if (!resp.ok) {
      syncVoiceStripDOM(stripEl);
      return;
    }
    const arrayBuf = await resp.arrayBuffer();
    let parsed = parseFLACMono16FromArrayBuffer(arrayBuf);
    if (!parsed) {
      const AC =
        (typeof window !== "undefined" && (window.AudioContext || window.webkitAudioContext)) ||
        null;
      if (AC) {
        const ctx = new AC();
        const audioBuf = await ctx.decodeAudioData(arrayBuf.slice(0));
        const ch0 = audioBuf.getChannelData(0);
        const sr = Math.min(65535, Math.max(8000, Math.round(audioBuf.sampleRate || 24000)));
        const int16 = new Int16Array(ch0.length);
        for (let i = 0; i < ch0.length; i++) {
          const s = Math.max(-1, Math.min(1, ch0[i]));
          int16[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
        }
        parsed = {
          sampleRate: sr,
          durationSec: audioBuf.duration || int16.length / sr,
          samples: int16,
          peaks: computePeaksFromInt16(int16, 170)
        };
        if (typeof ctx.close === "function") {
          ctx.close().catch(() => {});
        }
      }
    }
    if (parsed && voiceAudioState.voiceUrl === voiceUrl) {
      voiceDecodedCache.set(voiceUrl, parsed);
      voiceAudioState.sampleRate = parsed.sampleRate;
      voiceAudioState.durationSec = parsed.durationSec;
      voiceAudioState.peaks = parsed.peaks;
      voiceAudioState.pcmSamplesInt16 = parsed.samples;
    }
  } catch (_) {
    // Ignore decode errors in headless environments
  }
  syncAllVoiceStrips();
}

function stopVoicePlayback() {
  if (voiceAudioState.rafId) {
    cancelAnimationFrame(voiceAudioState.rafId);
    voiceAudioState.rafId = 0;
  }
  if (voiceAudioState.playbackBufSource) {
    try {
      voiceAudioState.playbackBufSource.onended = null;
      voiceAudioState.playbackBufSource.stop();
      voiceAudioState.playbackBufSource.disconnect();
    } catch (_) {}
    voiceAudioState.playbackBufSource = null;
  }
  if (voiceAudioState.playbackAudio) {
    try {
      voiceAudioState.playbackAudio.pause();
    } catch (_) {}
  }
  voiceAudioState.isPlaying = false;
}

function startVoicePlaybackFromRatio(stripEl, ratio) {
  stopVoicePlayback();
  const voiceUrl = (stripEl && stripEl.getAttribute("data-voice-url")) || voiceAudioState.voiceUrl;
  if (!voiceUrl && (!voiceAudioState.pcmSamplesInt16 || voiceAudioState.pcmSamplesInt16.length === 0)) {
    return;
  }

  const startR = Math.max(
    voiceAudioState.startRatio,
    Math.min(voiceAudioState.endRatio, ratio !== undefined ? ratio : voiceAudioState.playheadRatio)
  );
  const clampedStart =
    startR >= voiceAudioState.endRatio - 0.01 ? voiceAudioState.startRatio : startR;
  voiceAudioState.playheadRatio = clampedStart;

  const AC =
    (typeof window !== "undefined" && (window.AudioContext || window.webkitAudioContext)) ||
    null;
  if (
    AC &&
    voiceAudioState.pcmSamplesInt16 &&
    voiceAudioState.pcmSamplesInt16.length > 0 &&
    typeof AC.prototype.createBuffer === "function"
  ) {
    try {
      if (!voiceAudioState.playbackCtx) {
        voiceAudioState.playbackCtx = new AC();
      }
      const ctx = voiceAudioState.playbackCtx;
      if (ctx.state === "suspended" && typeof ctx.resume === "function") {
        ctx.resume().catch(() => {});
      }
      const samples = voiceAudioState.pcmSamplesInt16;
      const sr = voiceAudioState.sampleRate || 24000;
      const totalDur = samples.length / sr;
      const sIdx = Math.max(0, Math.min(samples.length - 1, Math.floor(clampedStart * samples.length)));
      const eIdx = Math.max(sIdx + 1, Math.min(samples.length, Math.ceil(voiceAudioState.endRatio * samples.length)));
      const sliceLen = eIdx - sIdx;
      const audioBuf = ctx.createBuffer(1, sliceLen, sr);
      const ch0 = audioBuf.getChannelData(0);
      for (let i = 0; i < sliceLen; i++) {
        ch0[i] = samples[sIdx + i] / 32768;
      }
      const srcNode = ctx.createBufferSource();
      srcNode.buffer = audioBuf;
      srcNode.connect(ctx.destination);
      voiceAudioState.playbackBufSource = srcNode;
      voiceAudioState.playbackStartCtxTime = ctx.currentTime;
      voiceAudioState.playbackStartRatio = clampedStart;
      voiceAudioState.playbackEndRatio = voiceAudioState.endRatio;
      voiceAudioState.isPlaying = true;
      syncAllVoiceStrips();

      const tickWebAudioPlayhead = () => {
        if (!voiceAudioState.isPlaying || !voiceAudioState.playbackBufSource) return;
        const elapsed = Math.max(0, ctx.currentTime - voiceAudioState.playbackStartCtxTime);
        const curRatio =
          totalDur > 0 ? voiceAudioState.playbackStartRatio + elapsed / totalDur : voiceAudioState.playbackEndRatio;
        if (curRatio >= voiceAudioState.playbackEndRatio) {
          stopVoicePlayback();
          voiceAudioState.playheadRatio = voiceAudioState.startRatio;
          syncAllVoiceStrips();
          return;
        }
        voiceAudioState.playheadRatio = curRatio;
        syncAllVoiceStrips();
        if (typeof requestAnimationFrame === "function") {
          voiceAudioState.rafId = requestAnimationFrame(tickWebAudioPlayhead);
        }
      };

      srcNode.onended = () => {
        if (!voiceAudioState.isPlaying) return;
        stopVoicePlayback();
        voiceAudioState.playheadRatio = voiceAudioState.startRatio;
        syncAllVoiceStrips();
      };

      srcNode.start(0);
      if (typeof requestAnimationFrame === "function") {
        voiceAudioState.rafId = requestAnimationFrame(tickWebAudioPlayhead);
      }
      return;
    } catch (_) {
      // Fall back to HTMLAudioElement below
    }
  }

  if (!voiceAudioState.playbackAudio) {
    if (typeof Audio !== "undefined") {
      voiceAudioState.playbackAudio = new Audio();
    } else if (typeof document !== "undefined") {
      voiceAudioState.playbackAudio = qs("#story-audio");
    }
  }
  const audio = voiceAudioState.playbackAudio;
  if (!audio) return;

  if (audio.getAttribute && audio.getAttribute("data-src") !== voiceUrl) {
    audio.src = voiceUrl;
    if (audio.setAttribute) audio.setAttribute("data-src", voiceUrl);
  } else if (audio.src !== voiceUrl && !audio.src.endsWith(voiceUrl)) {
    audio.src = voiceUrl;
  }

  const dur = voiceAudioState.durationSec || audio.duration || 0;
  if (dur > 0) {
    try {
      audio.currentTime = clampedStart * dur;
    } catch (_) {}
  }

  voiceAudioState.isPlaying = true;
  syncAllVoiceStrips();

  const tickPlayhead = () => {
    if (!voiceAudioState.isPlaying) return;
    const d = voiceAudioState.durationSec || audio.duration || 0;
    if (d > 0 && !audio.paused) {
      const r = audio.currentTime / d;
      if (r >= voiceAudioState.endRatio || audio.ended) {
        stopVoicePlayback();
        voiceAudioState.playheadRatio = voiceAudioState.startRatio;
        syncAllVoiceStrips();
        return;
      }
      voiceAudioState.playheadRatio = Math.max(
        voiceAudioState.startRatio,
        Math.min(voiceAudioState.endRatio, r)
      );
      syncAllVoiceStrips();
    }
    if (typeof requestAnimationFrame === "function") {
      voiceAudioState.rafId = requestAnimationFrame(tickPlayhead);
    }
  };

  audio.onended = () => {
    stopVoicePlayback();
    voiceAudioState.playheadRatio = voiceAudioState.startRatio;
    syncAllVoiceStrips();
  };

  const playPromise = audio.play && audio.play();
  if (playPromise && typeof playPromise.catch === "function") {
    playPromise.catch(() => {});
  }
  if (typeof requestAnimationFrame === "function") {
    voiceAudioState.rafId = requestAnimationFrame(tickPlayhead);
  }
}

async function startVoiceRecording(stripEl) {
  if (voiceAudioState.isRecording) return;
  stopVoicePlayback();

  const sceneId =
    (stripEl && (stripEl.getAttribute("data-scene") || "").trim()) ||
    voiceAudioState.sceneId;
  if (!sceneId) return;

  voiceAudioState.sceneId = sceneId;
  voiceAudioState.isRecording = true;
  voiceAudioState.recStartTime = Date.now();
  voiceAudioState.recChunks = [];
  voiceAudioState.recTotalSamples = 0;
  voiceAudioState.recLivePeaks = [];
  voiceAudioState.startRatio = 0;
  voiceAudioState.endRatio = 1;
  voiceAudioState.playheadRatio = 0;
  syncAllVoiceStrips();

  const useNativeBackend =
    Boolean(
      typeof window !== "undefined" &&
        window.webkit &&
        window.webkit.messageHandlers &&
        window.webkit.messageHandlers.storytellerHost
    ) ||
    typeof navigator === "undefined" ||
    !navigator.mediaDevices ||
    typeof navigator.mediaDevices.getUserMedia !== "function";

  if (useNativeBackend) {
    await startNativeBackendVoiceCapture(sceneId);
    return;
  }

  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    voiceAudioState.mediaStream = stream;
    const AC =
      (typeof window !== "undefined" && (window.AudioContext || window.webkitAudioContext)) ||
      null;
    if (AC) {
      const ctx = new AC({ sampleRate: 24000 });
      voiceAudioState.audioCtx = ctx;
      voiceAudioState.sampleRate = Math.min(65535, Math.max(8000, Math.round(ctx.sampleRate || 24000)));
      const source = ctx.createMediaStreamSource(stream);
      const proc = ctx.createScriptProcessor(2048, 1, 1);
      voiceAudioState.sourceNode = source;
      voiceAudioState.scriptNode = proc;
      proc.onaudioprocess = (ev) => {
        if (!voiceAudioState.isRecording) return;
        const input = ev.inputBuffer.getChannelData(0);
        const chunk = new Int16Array(input.length);
        let peak = 0;
        for (let i = 0; i < input.length; i++) {
          const s = Math.max(-1, Math.min(1, input[i]));
          const abs = Math.abs(s);
          if (abs > peak) peak = abs;
          chunk[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
        }
        voiceAudioState.recChunks.push(chunk);
        voiceAudioState.recTotalSamples += chunk.length;
        voiceAudioState.recLivePeaks.push(Math.min(1, peak * 1.8));
        syncAllVoiceStrips();
      };
      source.connect(proc);
      proc.connect(ctx.destination);
    }
  } catch (_) {
    await startNativeBackendVoiceCapture(sceneId);
  }
}

async function startNativeBackendVoiceCapture(sceneId) {
  voiceAudioState.isNativeCapture = true;
  if (voiceAudioState.nativePollTimer) {
    clearInterval(voiceAudioState.nativePollTimer);
    voiceAudioState.nativePollTimer = null;
  }
  await sendServerAction("StartSceneVoiceCapture", { scene: sceneId });
  postServerActionSilent("PollSceneVoiceCapture", {});
  if (!voiceAudioState.isRecording) return;
  voiceAudioState.nativePollTimer = setInterval(() => {
    if (!voiceAudioState.isRecording || !voiceAudioState.isNativeCapture) {
      clearInterval(voiceAudioState.nativePollTimer);
      voiceAudioState.nativePollTimer = null;
      return;
    }
    postServerActionSilent("PollSceneVoiceCapture", {});
  }, 90);
}

function handleNativeVoicePeakHeader(peakVal) {
  if (!voiceAudioState.isRecording || !voiceAudioState.isNativeCapture) return;
  if (peakVal === "stopped") {
    voiceAudioState.isRecording = false;
    voiceAudioState.isNativeCapture = false;
    if (voiceAudioState.nativePollTimer) {
      clearInterval(voiceAudioState.nativePollTimer);
      voiceAudioState.nativePollTimer = null;
    }
    syncAllVoiceStrips();
    return;
  }
  const num = Math.max(0.03, Math.min(1, parseFloat(peakVal) || 0.04));
  voiceAudioState.recLivePeaks.push(num);
  syncAllVoiceStrips();
}

function stopVoiceRecordingAndSave() {
  if (!voiceAudioState.isRecording) return;
  const wasNative = voiceAudioState.isNativeCapture;
  voiceAudioState.isRecording = false;
  voiceAudioState.isNativeCapture = false;
  if (voiceAudioState.nativePollTimer) {
    clearInterval(voiceAudioState.nativePollTimer);
    voiceAudioState.nativePollTimer = null;
  }
  if (wasNative) {
    voiceDecodedCache.clear();
    syncAllVoiceStrips();
    if (voiceAudioState.sceneId) {
      sendServerAction("StopSceneVoiceCapture", { scene: voiceAudioState.sceneId });
    }
    return;
  }

  if (voiceAudioState.scriptNode) {
    try {
      voiceAudioState.scriptNode.disconnect();
    } catch (_) {}
    voiceAudioState.scriptNode = null;
  }
  if (voiceAudioState.sourceNode) {
    try {
      voiceAudioState.sourceNode.disconnect();
    } catch (_) {}
    voiceAudioState.sourceNode = null;
  }
  if (voiceAudioState.audioCtx && typeof voiceAudioState.audioCtx.close === "function") {
    voiceAudioState.audioCtx.close().catch(() => {});
    voiceAudioState.audioCtx = null;
  }
  if (voiceAudioState.mediaStream) {
    try {
      voiceAudioState.mediaStream.getTracks().forEach((t) => t.stop());
    } catch (_) {}
    voiceAudioState.mediaStream = null;
  }

  let total = voiceAudioState.recTotalSamples;
  const sr = voiceAudioState.sampleRate || 24000;
  let merged;
  if (total > 0) {
    merged = new Int16Array(total);
    let offset = 0;
    for (let i = 0; i < voiceAudioState.recChunks.length; i++) {
      merged.set(voiceAudioState.recChunks[i], offset);
      offset += voiceAudioState.recChunks[i].length;
    }
  } else {
    total = Math.max(2400, Math.round(sr * 0.25));
    merged = new Int16Array(total);
  }

  voiceAudioState.pcmSamplesInt16 = merged;
  voiceAudioState.durationSec = merged.length / sr;
  voiceAudioState.peaks = computePeaksFromInt16(merged, 170);
  voiceAudioState.startRatio = 0;
  voiceAudioState.endRatio = 1;
  voiceAudioState.playheadRatio = 0;
  syncAllVoiceStrips();

  const b64 = int16ArrayToLEBase64(merged, 0, 1);
  if (b64 && voiceAudioState.sceneId) {
    sendServerAction("SaveSceneVoiceRecording", {
      scene: voiceAudioState.sceneId,
      pcm_b64: b64,
      sample_rate: String(sr),
      start_ratio: "0",
      end_ratio: "1"
    });
  }
}

function commitVoiceTrimAutoSave(stripEl) {
  const sceneId =
    (stripEl && (stripEl.getAttribute("data-scene") || "").trim()) ||
    voiceAudioState.sceneId;
  if (!sceneId) return;
  if (voiceAudioState.startRatio <= 0.002 && voiceAudioState.endRatio >= 0.998) {
    return;
  }
  const params = {
    scene: sceneId,
    start_ratio: voiceAudioState.startRatio.toFixed(4),
    end_ratio: voiceAudioState.endRatio.toFixed(4)
  };
  const voiceFile = (stripEl && stripEl.getAttribute("data-voice-file")) || "";
  if (
    voiceAudioState.pcmSamplesInt16 &&
    voiceAudioState.pcmSamplesInt16.length > 0 &&
    !voiceFile.toLowerCase().endsWith(".flac")
  ) {
    params.pcm_b64 = int16ArrayToLEBase64(voiceAudioState.pcmSamplesInt16, 0, 1);
    params.sample_rate = String(voiceAudioState.sampleRate || 24000);
  }
  voiceDecodedCache.clear();
  sendServerAction("TrimSceneVoice", params);
}

function initVoiceRecorderStrips() {
  const strips = getAllActiveVoiceStrips();
  if (strips.length === 0) return;
  const pendingAutoScene = voiceAudioState.autoRecordOnNextOpenScene;
  voiceAudioState.autoRecordOnNextOpenScene = "";
  strips.forEach((stripEl) => {
    const scId = (stripEl.getAttribute("data-scene") || "").trim();
    const vUrl = (stripEl.getAttribute("data-voice-url") || "").trim();
    if (pendingAutoScene && scId === pendingAutoScene && !vUrl && !voiceAudioState.isRecording) {
      startVoiceRecording(stripEl);
      return;
    }
    loadVoiceStripAudio(stripEl);
  });
}

function bindVoiceRecorderEvents() {
  if (typeof document === "undefined") return;

  document.addEventListener("click", (e) => {
    if (!e.target || !e.target.closest) return;

    const toggleRecBtn = e.target.closest('.voice-rec-toggle-btn[action="ToggleSceneVoiceRecorder"]');
    if (toggleRecBtn) {
      if (!toggleRecBtn.classList.contains("active")) {
        voiceAudioState.autoRecordOnNextOpenScene =
          (toggleRecBtn.getAttribute("data-scene") || "").trim();
      } else {
        voiceAudioState.autoRecordOnNextOpenScene = "";
        if (voiceAudioState.isRecording) {
          stopVoiceRecordingAndSave();
        } else {
          stopVoicePlayback();
        }
      }
      return;
    }

    const rerecordBtn = e.target.closest('[data-voice-record-start="true"]');
    if (rerecordBtn) {
      e.preventDefault();
      e.stopPropagation();
      const stripEl = rerecordBtn.closest(".voice-rec-strip");
      if (stripEl) {
        startVoiceRecording(stripEl);
      }
      return;
    }

    const transportBtn = e.target.closest('[data-voice-transport="true"]');
    if (transportBtn) {
      e.preventDefault();
      e.stopPropagation();
      const stripEl = transportBtn.closest(".voice-rec-strip");
      if (!stripEl) return;

      if (voiceAudioState.isRecording) {
        stopVoiceRecordingAndSave();
        return;
      }
      const voiceUrl = (stripEl.getAttribute("data-voice-url") || "").trim();
      if (!voiceUrl && (!voiceAudioState.pcmSamplesInt16 || voiceAudioState.pcmSamplesInt16.length === 0)) {
        startVoiceRecording(stripEl);
        return;
      }
      if (voiceAudioState.isPlaying) {
        stopVoicePlayback();
        syncAllVoiceStrips();
      } else {
        startVoicePlaybackFromRatio(stripEl, voiceAudioState.playheadRatio);
      }
    }
  });

  document.addEventListener("pointerdown", (e) => {
    if (e.button !== 0 || !e.target || !e.target.closest) return;
    if (voiceAudioState.isRecording) return;

    const trimHandle = e.target.closest(".voice-rec-trim-handle[data-voice-trim]");
    if (trimHandle) {
      const stripEl = trimHandle.closest(".voice-rec-strip");
      if (!stripEl) return;
      e.preventDefault();
      e.stopPropagation();
      stopVoicePlayback();
      voiceAudioState.dragMode = trimHandle.getAttribute("data-voice-trim") || "";
      voiceAudioState.dragStripEl = stripEl;
      voiceAudioState.dragMoved = false;
      return;
    }

    const wrapEl = e.target.closest('[data-voice-waveform-wrap="true"]');
    if (wrapEl) {
      const stripEl = wrapEl.closest(".voice-rec-strip");
      if (!stripEl) return;
      const rect = wrapEl.getBoundingClientRect();
      if (rect.width <= 0) return;
      e.preventDefault();
      const rawRatio = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
      const clamped = Math.max(
        voiceAudioState.startRatio,
        Math.min(voiceAudioState.endRatio, rawRatio)
      );
      voiceAudioState.playheadRatio = clamped;
      voiceAudioState.dragMode = "playhead";
      voiceAudioState.dragStripEl = stripEl;
      voiceAudioState.dragMoved = false;
      startVoicePlaybackFromRatio(stripEl, clamped);
    }
  });

  document.addEventListener("pointermove", (e) => {
    if (!voiceAudioState.dragMode || !voiceAudioState.dragStripEl) return;
    const wrapEl = qs('[data-voice-waveform-wrap="true"]', voiceAudioState.dragStripEl);
    if (!wrapEl) return;
    const rect = wrapEl.getBoundingClientRect();
    if (rect.width <= 0) return;

    const ratio = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    const minSpan = 0.05;

    if (voiceAudioState.dragMode === "start") {
      voiceAudioState.startRatio = Math.max(0, Math.min(voiceAudioState.endRatio - minSpan, ratio));
      if (voiceAudioState.playheadRatio < voiceAudioState.startRatio) {
        voiceAudioState.playheadRatio = voiceAudioState.startRatio;
      }
      voiceAudioState.dragMoved = true;
      syncAllVoiceStrips();
    } else if (voiceAudioState.dragMode === "end") {
      voiceAudioState.endRatio = Math.min(1, Math.max(voiceAudioState.startRatio + minSpan, ratio));
      if (voiceAudioState.playheadRatio > voiceAudioState.endRatio) {
        voiceAudioState.playheadRatio = voiceAudioState.endRatio;
      }
      voiceAudioState.dragMoved = true;
      syncAllVoiceStrips();
    } else if (voiceAudioState.dragMode === "playhead") {
      const clamped = Math.max(
        voiceAudioState.startRatio,
        Math.min(voiceAudioState.endRatio, ratio)
      );
      voiceAudioState.playheadRatio = clamped;
      if (voiceAudioState.playbackAudio && voiceAudioState.durationSec > 0) {
        try {
          voiceAudioState.playbackAudio.currentTime = clamped * voiceAudioState.durationSec;
        } catch (_) {}
      }
      syncAllVoiceStrips();
    }
  });

  const endVoiceDrag = () => {
    if (!voiceAudioState.dragMode) return;
    const mode = voiceAudioState.dragMode;
    const stripEl = voiceAudioState.dragStripEl;
    const moved = voiceAudioState.dragMoved;
    voiceAudioState.dragMode = "";
    voiceAudioState.dragStripEl = null;
    voiceAudioState.dragMoved = false;

    if ((mode === "start" || mode === "end") && moved && stripEl) {
      commitVoiceTrimAutoSave(stripEl);
    }
  };

  document.addEventListener("pointerup", endVoiceDrag);
  document.addEventListener("pointercancel", endVoiceDrag);
}

bindVoiceRecorderEvents();
