"use strict";

const EditorVoice = {
  moduleName: "EditorVoice",

  bindings() {
    return {
      "[data-voice-strip]": { init: this.initStrips },
      "[data-voice-strip] .voice-rec-transport-btn": { click: this.toggleTransport },
      "[data-voice-strip] .voice-rec-waveform-wrap": {
        pointerdown: this.startScrubDrag,
        pointermove: this.moveTrimDrag,
        pointerup: this.endTrimDrag
      },
      "[data-voice-strip] .voice-rec-trim-handle": {
        pointerdown: this.startTrimDrag,
        pointermove: this.moveTrimDrag,
        pointerup: this.endTrimDrag
      },
      "[data-voice-strip] .voice-rec-cut-btn": { click: this.cutTrim },
      "[data-voice-strip] .voice-rec-rerecord-btn": { click: this.startRecord },
      "[data-voice-strip] .voice-rec-delete-btn": { click: Dispatcher.action("DeleteSceneVoice") }
    };
  },

  dom: {
    selectors: {
      strip: "[data-voice-strip]",
      wrap: "[data-voice-waveform-wrap]",
      timer: "[data-voice-timer]",
      transport: "[data-voice-transport]",
      cut: "[data-voice-trim-destructive]"
    },
    attrs: {
      state: "data-voice-state",
      trimmed: "data-voice-trimmed",
      open: "data-voice-open",
      scene: "data-scene",
      paramScene: "param-scene",
      section: "data-voice-section",
      paramSection: "param-voice-section",
      url: "data-voice-url",
      paramUrl: "param-voice-url",
      trimStart: "data-trim-start",
      trimEnd: "data-trim-end",
      trimRole: "data-voice-trim"
    }
  },

  modes: {
    idle: "idle",
    recording: "recording",
    ready: "ready",
    playing: "playing"
  },

  handles: {
    start: "start",
    end: "end",
    scrub: "scrub"
  },

  peakStatus: {
    pending: "pending",
    denied: "denied",
    error: "error",
    stopped: "stopped"
  },

  values: {
    defaultSection: "01",
    defaultRatioStart: "0.0000",
    defaultRatioEnd: "1.0000",
    trueStr: "true",
    falseStr: "false",
    zeroStr: "0",
    oneStr: "1",
    querySep: "?",
    eventLoadedMeta: "loadedmetadata",
    audioClosed: "closed",
    audioSuspended: "suspended"
  },

  actions: {
    toggleRecorder: "ToggleSceneVoiceRecorder",
    startCapture: "StartSceneVoiceCapture",
    pollCapture: "PollSceneVoiceCapture",
    stopCapture: "StopSceneVoiceCapture",
    saveRecording: "SaveSceneVoiceRecording",
    trimVoice: "TrimSceneVoice"
  },

  i18n: {
    stopSaveHint: "voice_stop_save_hint",
    pauseHint: "player_pause_hint",
    listenHint: "voice_listen_take_hint",
    startRecHint: "voice_start_rec_hint",
    errUnavailable: "voice_err_mic_unavailable",
    errDenied: "voice_err_mic_permission_denied",
    errStartFailed: "voice_err_mic_start_failed",
    errWebAudio: "voice_err_webaudio_unsupported",
    errAccess: "voice_err_mic_access"
  },

  audioSpec: {
    defaultSampleRate: 24000,
    minSampleRate: 8000,
    maxSampleRate: 96000,
    scriptBufferSize: 2048,
    channels: 1,
    minPeakSamples: 256,
    peakRateDivisor: 25,
    maxPeaks: 240,
    webPeakGain: 1.35,
    nativePeakGain: 1.4,
    androidPollMs: 75,
    nativePollMs: 90,
    msPerSec: 1000,
    minSaveSamples: 64,
    pcmNegScale: 32768,
    pcmPosScale: 32767,
    b64ChunkSize: 8192,
    minTrimGap: 0.03,
    ratioDecimals: 4,
    fallbackDurationSec: 5,
    minPlaySpanSec: 0.05,
    androidStatusOk: 1,
    androidStatusPerm: 2,
    minRatio: 0,
    maxRatio: 1,
    playheadOff: -1
  },

  playback: {
    audio: null,
    sourceUrl: "",
    playing: false,
    durationSec: 0,
    animFrame: 0
  },

  capture: {
    recording: false,
    starting: false,
    nativeMode: false,
    nativePollTimer: null,
    androidMode: false,
    androidPendingPerm: false,
    androidPollTimer: null,
    audioCtx: null,
    mediaStream: null,
    sourceNode: null,
    processorNode: null,
    sampleRate: 24000,
    pcmChunks: [],
    totalSamples: 0,
    peaks: [],
    peakWindowMax: 0,
    peakWindowSamples: 0,
    rawFloatSamples: null
  },

  trimState: {
    stripEl: null,
    sceneId: "",
    sectionKey: "01",
    trimStart: 0.0,
    trimEnd: 1.0,
    draggingHandle: ""
  },

  resolveStripMeta(stripEl, params) {
    const attrs = this.dom.attrs;
    const vals = this.values;
    const spec = this.audioSpec;
    if (!stripEl) {
      return {
        sceneId: "",
        sectionKey: vals.defaultSection,
        audioUrl: "",
        trimStart: spec.minRatio,
        trimEnd: spec.maxRatio
      };
    }
    const sceneId =
      (params && (params.scene || params.scene_id)) ||
      stripEl.getAttribute(attrs.paramScene) ||
      stripEl.getAttribute(attrs.scene) ||
      "";
    const sectionKey =
      (params && params.voice_section) ||
      stripEl.getAttribute(attrs.paramSection) ||
      stripEl.getAttribute(attrs.section) ||
      vals.defaultSection;
    const audioUrl = (
      (params && params.voice_url) ||
      stripEl.getAttribute(attrs.url) ||
      stripEl.getAttribute(attrs.paramUrl) ||
      ""
    ).trim();
    const savedStart = parseFloat(stripEl.getAttribute(attrs.trimStart) || vals.zeroStr) || spec.minRatio;
    const savedEnd = parseFloat(stripEl.getAttribute(attrs.trimEnd) || vals.oneStr);
    const trimStart = savedStart >= spec.minRatio && savedStart < spec.maxRatio ? savedStart : spec.minRatio;
    const trimEnd = savedEnd > trimStart && savedEnd <= spec.maxRatio ? savedEnd : spec.maxRatio;
    return { sceneId, sectionKey, audioUrl, trimStart, trimEnd };
  },

  syncStripState(stripEl) {
    if (!stripEl) return;
    const attrs = this.dom.attrs;
    const sels = this.dom.selectors;
    const vals = this.values;
    const cap = this.capture;
    const pb = this.playback;
    const ts = this.trimState;
    const meta = this.resolveStripMeta(stripEl, null);
    const isThisStrip = ts.stripEl === stripEl;
    const isRec = isThisStrip && (cap.recording || cap.starting);
    const isPlaying = isThisStrip && pb.playing;
    const hasFile = Boolean(
      meta.audioUrl ||
      (isThisStrip && cap.rawFloatSamples && cap.rawFloatSamples.length > 0) ||
      (isThisStrip && cap.peaks && cap.peaks.length > 0 && !isRec)
    );

    const stateMode = isRec
      ? this.modes.recording
      : isPlaying
        ? this.modes.playing
        : hasFile
          ? this.modes.ready
          : this.modes.idle;
    stripEl.setAttribute(attrs.state, stateMode);

    const sRatio = isThisStrip ? ts.trimStart : meta.trimStart;
    const eRatio = isThisStrip ? ts.trimEnd : meta.trimEnd;
    const trimmed = !isRec && hasFile && EditorVoiceWaveform.isTrimActive(sRatio, eRatio);
    stripEl.setAttribute(attrs.trimmed, trimmed ? vals.trueStr : vals.falseStr);

    const cutBtn = stripEl.querySelector(sels.cut);
    if (cutBtn) cutBtn.disabled = !trimmed;

    const transportBtn = stripEl.querySelector(sels.transport);
    if (transportBtn) {
      const hintKey = isRec
        ? this.i18n.stopSaveHint
        : isPlaying
          ? this.i18n.pauseHint
          : hasFile
            ? this.i18n.listenHint
            : this.i18n.startRecHint;
      AppShellController.setHint(transportBtn, AppShellController.tr(hintKey));
    }
  },

  stopPreview(stripEl) {
    const pb = this.playback;
    if (pb.audio) {
      try { pb.audio.pause(); } catch (_) {}
    }
    pb.playing = false;
    if (pb.animFrame && !this.capture.recording) {
      cancelAnimationFrame(pb.animFrame);
      pb.animFrame = 0;
    }
    this.syncStripState(stripEl || this.trimState.stripEl);
  },

  togglePreview(stripEl) {
    if (!stripEl) return;
    const pb = this.playback;
    const ts = this.trimState;
    const cap = this.capture;
    const spec = this.audioSpec;
    if (pb.playing) {
      this.stopPreview(stripEl);
      EditorVoiceWaveform.draw(stripEl, ts, cap, spec.playheadOff);
      return;
    }
    const meta = this.resolveStripMeta(stripEl, null);
    if (!meta.audioUrl) return;
    const baseUrl = meta.audioUrl.split(this.values.querySep)[0];
    if (!pb.audio || pb.sourceUrl !== baseUrl) {
      if (pb.audio) {
        try { pb.audio.pause(); } catch (_) {}
      }
      pb.audio = new window.Audio(meta.audioUrl);
      pb.sourceUrl = baseUrl;
    }
    const aud = pb.audio;
    const startPlay = () => {
      const dur = aud.duration && isFinite(aud.duration) ? aud.duration : pb.durationSec || spec.fallbackDurationSec;
      pb.durationSec = dur;
      const t0 = ts.trimStart * dur;
      const t1 = Math.max(t0 + spec.minPlaySpanSec, ts.trimEnd * dur);
      if (aud.currentTime < t0 || aud.currentTime >= t1 - spec.minPlaySpanSec) aud.currentTime = t0;
      pb.playing = true;
      this.syncStripState(stripEl);
      aud.play().catch(() => {
        pb.playing = false;
        this.syncStripState(stripEl);
      });
      const tick = () => {
        if (!pb.playing || aud.paused) {
          pb.playing = false;
          this.syncStripState(stripEl);
          EditorVoiceWaveform.draw(stripEl, ts, cap, spec.playheadOff);
          return;
        }
        if (aud.currentTime >= t1) {
          aud.pause();
          aud.currentTime = t0;
          pb.playing = false;
          this.syncStripState(stripEl);
          EditorVoiceWaveform.draw(stripEl, ts, cap, spec.playheadOff);
          return;
        }
        EditorVoiceWaveform.draw(stripEl, ts, cap, dur > spec.minRatio ? aud.currentTime / dur : spec.minRatio);
        pb.animFrame = requestAnimationFrame(tick);
      };
      pb.animFrame = requestAnimationFrame(tick);
    };
    if (aud.readyState >= spec.channels) startPlay();
    else aud.addEventListener(this.values.eventLoadedMeta, startPlay, { once: true });
  },

  stopCaptureTracks() {
    const cap = this.capture;
    if (cap.nativePollTimer) { clearInterval(cap.nativePollTimer); cap.nativePollTimer = null; }
    if (cap.androidPollTimer) { clearInterval(cap.androidPollTimer); cap.androidPollTimer = null; }
    if (cap.processorNode) { try { cap.processorNode.disconnect(); } catch (_) {} cap.processorNode = null; }
    if (cap.sourceNode) { try { cap.sourceNode.disconnect(); } catch (_) {} cap.sourceNode = null; }
    if (cap.mediaStream) {
      try { cap.mediaStream.getTracks().forEach((t) => t.stop()); } catch (_) {}
      cap.mediaStream = null;
    }
  },

  encodeInt16Base64(floatSamples, startRatio, endRatio) {
    if (!floatSamples || !floatSamples.length) return "";
    const spec = this.audioSpec;
    const total = floatSamples.length;
    const i0 = Math.max(0, Math.min(total - 1, Math.floor((startRatio || spec.minRatio) * total)));
    const i1 = Math.max(i0 + 1, Math.min(total, Math.ceil((endRatio || spec.maxRatio) * total)));
    const len = i1 - i0;
    const int16 = new Int16Array(len);
    for (let i = 0; i < len; i++) {
      let s = floatSamples[i0 + i];
      if (s > spec.maxRatio) s = spec.maxRatio;
      else if (s < -spec.maxRatio) s = -spec.maxRatio;
      int16[i] = s < spec.minRatio ? Math.round(s * spec.pcmNegScale) : Math.round(s * spec.pcmPosScale);
    }
    const bytes = new Uint8Array(int16.buffer);
    let binary = "";
    for (let i = 0; i < bytes.length; i += spec.b64ChunkSize) {
      binary += String.fromCharCode.apply(null, bytes.subarray(i, Math.min(bytes.length, i + spec.b64ChunkSize)));
    }
    return window.btoa ? window.btoa(binary) : "";
  },

  startAndroidCapture(stripEl) {
    const cap = this.capture;
    const pb = this.playback;
    const ts = this.trimState;
    const spec = this.audioSpec;
    cap.starting = true;
    cap.recording = false;
    this.syncStripState(stripEl);
    let status = 0;
    try { status = Number(window.mojraAndroid.startAudioCapture()); } catch (_) { status = 0; }
    if (status !== spec.androidStatusOk && status !== spec.androidStatusPerm) {
      cap.starting = false;
      this.syncStripState(stripEl);
      AppShellController.showToast(AppShellController.tr(this.i18n.errUnavailable));
      return;
    }
    cap.starting = false;
    cap.recording = true;
    cap.androidMode = true;
    cap.androidPendingPerm = status === spec.androidStatusPerm;
    if (window.mojraAndroid.getAudioSampleRate) {
      const sr = Number(window.mojraAndroid.getAudioSampleRate());
      if (sr >= spec.minSampleRate && sr <= spec.maxSampleRate) cap.sampleRate = sr;
    }
    this.syncStripState(stripEl);
    let startWallMs = Date.now();
    cap.androidPollTimer = setInterval(() => {
      if (!cap.recording || !cap.androidMode || !window.mojraAndroid) return;
      const rawPeak = String(window.mojraAndroid.pollAudioPeak() || "");
      if (rawPeak === this.peakStatus.pending) {
        startWallMs = Date.now();
        return;
      }
      if (cap.androidPendingPerm) {
        cap.androidPendingPerm = false;
        startWallMs = Date.now();
      }
      if (rawPeak === this.peakStatus.denied || rawPeak === this.peakStatus.error) {
        this.stopCaptureTracks();
        cap.recording = false;
        cap.androidMode = false;
        this.syncStripState(stripEl);
        AppShellController.showToast(rawPeak === this.peakStatus.denied ? AppShellController.tr(this.i18n.errDenied) : AppShellController.tr(this.i18n.errStartFailed));
        return;
      }
      if (rawPeak === this.peakStatus.stopped) {
        this.stopAndSaveCapture(stripEl);
        return;
      }
      const p = parseFloat(rawPeak);
      if (!isNaN(p) && p >= spec.minRatio) {
        cap.peaks.push(Math.min(spec.maxRatio, p * spec.nativePeakGain));
        if (cap.peaks.length > spec.maxPeaks) cap.peaks.shift();
      }
    }, spec.androidPollMs);
    const tickAndroid = () => {
      if (!cap.recording || !cap.androidMode) return;
      pb.durationSec = cap.androidPendingPerm ? spec.minRatio : (Date.now() - startWallMs) / spec.msPerSec;
      const timeEl = stripEl.querySelector(this.dom.selectors.timer);
      if (timeEl) timeEl.textContent = AudioDeckController.formatSeconds(pb.durationSec);
      EditorVoiceWaveform.draw(stripEl, ts, cap, spec.playheadOff);
      pb.animFrame = requestAnimationFrame(tickAndroid);
    };
    pb.animFrame = requestAnimationFrame(tickAndroid);
  },

  async startNativeCapture(stripEl) {
    const cap = this.capture;
    const pb = this.playback;
    const ts = this.trimState;
    const spec = this.audioSpec;
    const vals = this.values;
    cap.starting = true;
    cap.recording = false;
    this.syncStripState(stripEl);
    const decoded = await Dispatcher.post(this.actions.startCapture, {
      scene: ts.sceneId,
      voice_section: ts.sectionKey,
      silent: vals.trueStr
    });
    if (!cap.starting) return;
    const peakHdr = (decoded && decoded.voicePeak) || "";
    if (!decoded || peakHdr === this.peakStatus.stopped) {
      cap.starting = false;
      cap.recording = false;
      this.syncStripState(stripEl);
      return;
    }
    cap.starting = false;
    cap.recording = true;
    cap.nativeMode = true;
    this.syncStripState(stripEl);
    const startWallMs = Date.now();
    cap.nativePollTimer = setInterval(() => {
      if (!cap.recording || !cap.nativeMode) return;
      Dispatcher.post(this.actions.pollCapture, { silent: vals.trueStr });
    }, spec.nativePollMs);
    const tickNative = () => {
      if (!cap.recording || !cap.nativeMode) return;
      pb.durationSec = (Date.now() - startWallMs) / spec.msPerSec;
      const timeEl = stripEl.querySelector(this.dom.selectors.timer);
      if (timeEl) timeEl.textContent = AudioDeckController.formatSeconds(pb.durationSec);
      EditorVoiceWaveform.draw(stripEl, ts, cap, spec.playheadOff);
      pb.animFrame = requestAnimationFrame(tickNative);
    };
    pb.animFrame = requestAnimationFrame(tickNative);
  },

  async startWebAudioCapture(stripEl) {
    const cap = this.capture;
    const pb = this.playback;
    const ts = this.trimState;
    const spec = this.audioSpec;
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, channelCount: spec.channels }
      });
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (!AudioCtx) {
        stream.getTracks().forEach((t) => t.stop());
        AppShellController.showToast(AppShellController.tr(this.i18n.errWebAudio));
        return;
      }
      if (!cap.audioCtx || cap.audioCtx.state === this.values.audioClosed) {
        cap.audioCtx = new AudioCtx({ sampleRate: spec.defaultSampleRate });
      }
      if (cap.audioCtx.state === this.values.audioSuspended) await cap.audioCtx.resume();
      cap.sampleRate = cap.audioCtx.sampleRate || spec.defaultSampleRate;
      cap.mediaStream = stream;
      cap.sourceNode = cap.audioCtx.createMediaStreamSource(stream);
      const proc = cap.audioCtx.createScriptProcessor(spec.scriptBufferSize, spec.channels, spec.channels);
      cap.processorNode = proc;
      cap.recording = true;
      this.syncStripState(stripEl);
      const samplesPerPeak = Math.max(spec.minPeakSamples, Math.floor(cap.sampleRate / spec.peakRateDivisor));
      proc.onaudioprocess = (ev) => {
        if (!cap.recording) return;
        const ch = ev.inputBuffer.getChannelData(0);
        const copy = new Float32Array(ch.length);
        copy.set(ch);
        cap.pcmChunks.push(copy);
        cap.totalSamples += copy.length;
        pb.durationSec = cap.totalSamples / cap.sampleRate;
        for (let i = 0; i < copy.length; i++) {
          const a = Math.abs(copy[i]);
          if (a > cap.peakWindowMax) cap.peakWindowMax = a;
          cap.peakWindowSamples++;
          if (cap.peakWindowSamples >= samplesPerPeak) {
            cap.peaks.push(Math.min(spec.maxRatio, cap.peakWindowMax * spec.webPeakGain));
            if (cap.peaks.length > spec.maxPeaks) cap.peaks.shift();
            cap.peakWindowMax = 0;
            cap.peakWindowSamples = 0;
          }
        }
      };
      cap.sourceNode.connect(proc);
      proc.connect(cap.audioCtx.destination);
      const tick = () => {
        if (!cap.recording) return;
        const timeEl = stripEl.querySelector(this.dom.selectors.timer);
        if (timeEl) timeEl.textContent = Audio.formatSeconds(pb.durationSec);
        EditorVoiceWaveform.draw(stripEl, ts, cap, spec.playheadOff);
        pb.animFrame = requestAnimationFrame(tick);
      };
      pb.animFrame = requestAnimationFrame(tick);
    } catch (_) {
      AppShellController.showToast(AppShellController.tr(this.i18n.errAccess));
    }
  },

  async startCapture(stripEl, params) {
    if (!stripEl) return;
    const cap = this.capture;
    const pb = this.playback;
    const ts = this.trimState;
    const spec = this.audioSpec;
    this.stopPreview(stripEl);
    this.stopCaptureTracks();

    const meta = this.resolveStripMeta(stripEl, params);
    ts.stripEl = stripEl;
    ts.sceneId = meta.sceneId;
    ts.sectionKey = meta.sectionKey;
    ts.trimStart = spec.minRatio;
    ts.trimEnd = spec.maxRatio;
    cap.pcmChunks = [];
    cap.totalSamples = 0;
    cap.peaks = [];
    cap.peakWindowMax = 0;
    cap.peakWindowSamples = 0;
    cap.rawFloatSamples = null;
    cap.nativeMode = false;
    cap.androidMode = false;
    cap.androidPendingPerm = false;
    pb.durationSec = 0;
    EditorVoiceWaveform.syncHandles(stripEl, ts, pb, cap);

    if (typeof window !== "undefined" && window.mojraAndroid && window.mojraAndroid.startAudioCapture) {
      this.startAndroidCapture(stripEl);
      return;
    }
    const canUseWebMic = Boolean(
      typeof navigator !== "undefined" &&
      navigator.mediaDevices &&
      navigator.mediaDevices.getUserMedia &&
      (typeof window === "undefined" || !window.mojraNative)
    );
    if (!canUseWebMic) {
      await this.startNativeCapture(stripEl);
      return;
    }
    await this.startWebAudioCapture(stripEl);
  },

  async stopAndSaveCapture(stripEl) {
    const cap = this.capture;
    const pb = this.playback;
    const ts = this.trimState;
    const vals = this.values;
    const spec = this.audioSpec;
    if (!cap.recording && !cap.starting) return;
    const wasNative = cap.nativeMode;
    const wasAndroid = cap.androidMode;
    cap.starting = false;
    cap.recording = false;
    cap.nativeMode = false;
    cap.androidMode = false;
    cap.androidPendingPerm = false;
    if (pb.animFrame) { cancelAnimationFrame(pb.animFrame); pb.animFrame = 0; }
    this.stopCaptureTracks();

    if (wasAndroid && window.mojraAndroid && window.mojraAndroid.stopAudioCaptureBase64) {
      let b64 = "";
      try { b64 = String(window.mojraAndroid.stopAudioCaptureBase64() || ""); } catch (_) { b64 = ""; }
      this.syncStripState(stripEl);
      if (!b64) return;
      await Dispatcher.post(this.actions.saveRecording, {
        scene: ts.sceneId,
        voice_section: ts.sectionKey,
        sample_rate: String(Math.round(cap.sampleRate || spec.defaultSampleRate)),
        start_ratio: vals.defaultRatioStart,
        end_ratio: vals.defaultRatioEnd,
        pcm_b64: b64
      });
      return;
    }

    if (wasNative) {
      this.syncStripState(stripEl);
      await Dispatcher.post(this.actions.stopCapture, {
        scene: ts.sceneId,
        voice_section: ts.sectionKey
      });
      return;
    }

    if (cap.totalSamples > 0) {
      const merged = new Float32Array(cap.totalSamples);
      let off = 0;
      for (let i = 0; i < cap.pcmChunks.length; i++) {
        merged.set(cap.pcmChunks[i], off);
        off += cap.pcmChunks[i].length;
      }
      cap.rawFloatSamples = merged;
      cap.pcmChunks = [];
    }
    EditorVoiceWaveform.syncHandles(stripEl, ts, pb, cap);
    EditorVoiceWaveform.draw(stripEl, ts, cap, spec.playheadOff);
    if (!cap.rawFloatSamples || cap.rawFloatSamples.length < spec.minSaveSamples) {
      this.syncStripState(stripEl);
      return;
    }
    const b64 = this.encodeInt16Base64(cap.rawFloatSamples, spec.minRatio, spec.maxRatio);
    if (!b64) return;
    await Dispatcher.post(this.actions.saveRecording, {
      scene: ts.sceneId,
      voice_section: ts.sectionKey,
      sample_rate: String(Math.round(cap.sampleRate || spec.defaultSampleRate)),
      start_ratio: vals.defaultRatioStart,
      end_ratio: vals.defaultRatioEnd,
      pcm_b64: b64
    });
  },

  handlePeakHeader(peakVal) {
    const cap = this.capture;
    const spec = this.audioSpec;
    if (!cap.nativeMode || (!cap.recording && !cap.starting)) return;
    if (peakVal === this.peakStatus.stopped) {
      if (cap.starting) return;
      this.stopAndSaveCapture(this.trimState.stripEl);
      return;
    }
    const p = parseFloat(peakVal);
    if (!isNaN(p) && p >= spec.minRatio) {
      cap.peaks.push(Math.min(spec.maxRatio, p * spec.nativePeakGain));
      if (cap.peaks.length > spec.maxPeaks) cap.peaks.shift();
    }
  },

  toggleTransport(params, el) {
    const stripEl = el.closest(this.dom.selectors.strip) || el;
    const cap = this.capture;
    const ts = this.trimState;
    if (ts.stripEl === stripEl && (cap.recording || cap.starting)) {
      this.stopAndSaveCapture(stripEl);
      return;
    }
    const meta = this.resolveStripMeta(stripEl, params);
    const hasExisting = Boolean(
      meta.audioUrl ||
      (ts.stripEl === stripEl && cap.rawFloatSamples && cap.rawFloatSamples.length > 0)
    );
    if (hasExisting) {
      ts.stripEl = stripEl;
      ts.sceneId = meta.sceneId;
      ts.sectionKey = meta.sectionKey;
      this.togglePreview(stripEl);
    } else {
      this.startCapture(stripEl, params);
    }
  },

  startRecord(params, el) {
    const stripEl = el.closest(this.dom.selectors.strip) || el;
    this.startCapture(stripEl, params);
  },

  async cutTrim(params, el) {
    const stripEl = el.closest(this.dom.selectors.strip) || el;
    const ts = this.trimState;
    const cap = this.capture;
    const spec = this.audioSpec;
    const meta = this.resolveStripMeta(stripEl, params);
    const sRatio = ts.stripEl === stripEl ? ts.trimStart : meta.trimStart;
    const eRatio = ts.stripEl === stripEl ? ts.trimEnd : meta.trimEnd;
    if (!EditorVoiceWaveform.isTrimActive(sRatio, eRatio)) return;
    ts.stripEl = stripEl;
    ts.trimStart = sRatio;
    ts.trimEnd = eRatio > sRatio ? eRatio : spec.maxRatio;
    const postParams = {
      scene: meta.sceneId,
      voice_section: meta.sectionKey,
      start_ratio: ts.trimStart.toFixed(spec.ratioDecimals),
      end_ratio: ts.trimEnd.toFixed(spec.ratioDecimals),
      destructive: this.values.trueStr
    };
    if (cap.rawFloatSamples && cap.rawFloatSamples.length > spec.minSaveSamples) {
      postParams.sample_rate = String(Math.round(cap.sampleRate || spec.defaultSampleRate));
      postParams.pcm_b64 = this.encodeInt16Base64(cap.rawFloatSamples, spec.minRatio, spec.maxRatio);
    }
    this.stopPreview(stripEl);
    ts.trimStart = spec.minRatio;
    ts.trimEnd = spec.maxRatio;
    await Dispatcher.post(this.actions.trimVoice, postParams);
  },

  startTrimDrag(params, el) {
    const stripEl = el.closest(this.dom.selectors.strip);
    if (!stripEl) return;
    const ts = this.trimState;
    const meta = this.resolveStripMeta(stripEl, params);
    if (ts.stripEl !== stripEl) {
      ts.stripEl = stripEl;
      ts.sceneId = meta.sceneId;
      ts.sectionKey = meta.sectionKey;
      ts.trimStart = meta.trimStart;
      ts.trimEnd = meta.trimEnd;
    }
    ts.draggingHandle = params.voice_trim || el.getAttribute(this.dom.attrs.trimRole) || this.handles.start;
  },

  moveTrimDrag(params) {
    const ts = this.trimState;
    const pb = this.playback;
    const cap = this.capture;
    const spec = this.audioSpec;
    if (!ts.draggingHandle || !ts.stripEl) return;
    const wrap = ts.stripEl.querySelector(this.dom.selectors.wrap);
    if (!wrap) return;
    const rect = wrap.getBoundingClientRect();
    if (!rect.width) return;
    const ratio = Math.max(spec.minRatio, Math.min(spec.maxRatio, ((params.client_x || 0) - rect.left) / rect.width));
    if (ts.draggingHandle === this.handles.start) {
      ts.trimStart = Math.min(ratio, ts.trimEnd - spec.minTrimGap);
    } else if (ts.draggingHandle === this.handles.end) {
      ts.trimEnd = Math.max(ratio, ts.trimStart + spec.minTrimGap);
    } else if (ts.draggingHandle === this.handles.scrub && pb.audio && pb.durationSec > spec.minRatio) {
      const clamped = Math.max(ts.trimStart, Math.min(ts.trimEnd, ratio));
      pb.audio.currentTime = clamped * pb.durationSec;
      EditorVoiceWaveform.draw(ts.stripEl, ts, cap, clamped);
      return;
    }
    EditorVoiceWaveform.syncHandles(ts.stripEl, ts, pb, cap);
    EditorVoiceWaveform.draw(ts.stripEl, ts, cap, spec.playheadOff);
  },

  async endTrimDrag() {
    const ts = this.trimState;
    const spec = this.audioSpec;
    const attrs = this.dom.attrs;
    if (!ts.draggingHandle) return;
    const wasTrim = ts.draggingHandle === this.handles.start || ts.draggingHandle === this.handles.end;
    const stripEl = ts.stripEl;
    ts.draggingHandle = "";
    if (wasTrim && stripEl) {
      const startStr = ts.trimStart.toFixed(spec.ratioDecimals);
      const endStr = ts.trimEnd.toFixed(spec.ratioDecimals);
      stripEl.setAttribute(attrs.trimStart, startStr);
      stripEl.setAttribute(attrs.trimEnd, endStr);
      this.syncStripState(stripEl);
      const meta = this.resolveStripMeta(stripEl, null);
      await Dispatcher.post(this.actions.trimVoice, {
        scene: ts.sceneId || meta.sceneId,
        voice_section: ts.sectionKey || meta.sectionKey,
        start_ratio: startStr,
        end_ratio: endStr,
        destructive: this.values.falseStr,
        silent: this.values.trueStr
      });
    }
  },

  startScrubDrag(params, el) {
    const stripEl = el.closest(this.dom.selectors.strip);
    if (!stripEl) return;
    const ts = this.trimState;
    const pb = this.playback;
    const cap = this.capture;
    const spec = this.audioSpec;
    const meta = this.resolveStripMeta(stripEl, params);
    if (!meta.audioUrl || cap.recording) return;
    const rect = el.getBoundingClientRect();
    if (!rect.width) return;
    ts.stripEl = stripEl;
    ts.draggingHandle = this.handles.scrub;
    const ratio = Math.max(ts.trimStart, Math.min(ts.trimEnd, ((params.client_x || 0) - rect.left) / rect.width));
    EditorVoiceWaveform.draw(stripEl, ts, cap, ratio);
    if (!pb.playing) this.togglePreview(stripEl);
    if (pb.audio && pb.durationSec > spec.minRatio) {
      pb.audio.currentTime = ratio * pb.durationSec;
    }
  },

  initStrips() {
    const triggerAction = Dispatcher.lastAction;
    const strips = Array.from(document.querySelectorAll(this.dom.selectors.strip));
    if (!strips.length) return;
    const ts = this.trimState;
    const pb = this.playback;
    const cap = this.capture;
    strips.forEach((stripEl) => {
      const meta = this.resolveStripMeta(stripEl, null);
      if (!cap.recording && (ts.sceneId !== meta.sceneId || ts.sectionKey !== meta.sectionKey || ts.stripEl !== stripEl)) {
        ts.stripEl = stripEl;
        ts.sceneId = meta.sceneId;
        ts.sectionKey = meta.sectionKey;
        ts.trimStart = meta.trimStart;
        ts.trimEnd = meta.trimEnd;
        cap.rawFloatSamples = null;
        cap.peaks = [];
        if (pb.sourceUrl && pb.sourceUrl !== meta.audioUrl.split(this.values.querySep)[0]) {
          this.stopPreview(stripEl);
          pb.audio = null;
          pb.sourceUrl = "";
        }
      }
      EditorVoiceWaveform.syncHandles(stripEl, ts, pb, cap);
      EditorVoiceWaveform.loadPeaks(stripEl, ts, cap);
      if (
        triggerAction === this.actions.toggleRecorder &&
        stripEl.getAttribute(this.dom.attrs.open) === this.values.trueStr &&
        !meta.audioUrl &&
        !cap.recording &&
        !cap.starting
      ) {
        this.startCapture(stripEl, { scene: meta.sceneId, voice_section: meta.sectionKey });
      }
    });
  }
};
Dispatcher.register(EditorVoice.moduleName, EditorVoice);
