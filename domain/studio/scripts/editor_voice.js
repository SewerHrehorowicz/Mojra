"use strict";

const EditorVoice = {
  moduleName: "EditorVoice",

  bindings() {
    return {
      ".voice-rec-strip": { init: this.initStrips },
      ".voice-rec-strip .voice-rec-play-btn": { click: this.togglePreview },
      ".voice-rec-strip .voice-rec-record-btn": { click: this.toggleRecord },
      ".voice-rec-strip .voice-rec-waveform-wrap": {
        pointerdown: this.startScrubDrag,
        pointermove: this.moveTrimDrag,
        pointerup: this.endTrimDrag
      },
      ".voice-rec-strip .voice-rec-trim-handle": {
        pointerdown: this.startTrimDrag,
        pointermove: this.moveTrimDrag,
        pointerup: this.endTrimDrag
      },
      ".voice-rec-strip .voice-rec-cut-btn": { click: this.cutTrim },
      ".voice-rec-strip .voice-rec-delete-btn": { click: Dispatcher.action("DeleteSceneVoice") }
    };
  },

  dom: {
    selectors: {
      strip: ".voice-rec-strip",
      wrap: ".voice-rec-waveform-wrap",
      timer: ".voice-rec-timer",
      playBtn: ".voice-rec-play-btn",
      recordBtn: ".voice-rec-record-btn",
      cutBtn: ".voice-rec-cut-btn",
      deleteBtn: ".voice-rec-delete-btn"
    }
  },

  classes: {
    idle: "is-idle",
    recording: "is-recording",
    ready: "is-ready",
    playing: "is-playing",
    disabled: "is-disabled",
    trimEnd: "trim-end"
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
    eventLoadedMeta: "loadedmetadata",
    eventCanPlay: "canplay",
    preloadAuto: "auto",
    audioClosed: "closed",
    audioSuspended: "suspended",
    undefType: "undefined",
    numType: "number",
    fnType: "function"
  },

  actions: {
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
    readyStateHaveCurrentData: 2,
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
    seekEpsilonSec: 0.015,
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
    startingPlay: false,
    durationSec: 0,
    scrubRatio: 0,
    animFrame: 0
  },

  capture: {
    sessionSeq: 0,
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
    gainNode: null,
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

  resolveStripControllerData() {
    if (typeof TextEditorController !== this.values.undefType) {
      return TextEditorController.data;
    }
    return null;
  },

  resolveStripMeta(params) {
    const vals = this.values;
    const spec = this.audioSpec;
    const ctrlData = this.resolveStripControllerData();
    const sceneId =
      (params && (params.scene || params.scene_id)) ||
      (ctrlData && ctrlData.sceneId) ||
      "";
    const sectionKey =
      (params && params.voice_section) ||
      (ctrlData && ctrlData.voiceSection) ||
      vals.defaultSection;
    const audioUrl = (
      (params && params.voice_url) ||
      (ctrlData && ctrlData.voiceUrl) ||
      ""
    ).trim();
    const voicePeaks = (ctrlData && ctrlData.voicePeaks) || "";
    const savedStart =
      ctrlData && typeof ctrlData.voiceTrimStart === vals.numType
        ? ctrlData.voiceTrimStart
        : spec.minRatio;
    const savedEnd =
      ctrlData && typeof ctrlData.voiceTrimEnd === vals.numType && ctrlData.voiceTrimEnd > spec.minRatio
        ? ctrlData.voiceTrimEnd
        : spec.maxRatio;
    const trimStart = savedStart >= spec.minRatio && savedStart < spec.maxRatio ? savedStart : spec.minRatio;
    const trimEnd = savedEnd > trimStart && savedEnd <= spec.maxRatio ? savedEnd : spec.maxRatio;
    return { sceneId, sectionKey, audioUrl, voicePeaks, trimStart, trimEnd };
  },

  syncStripState(stripElement) {
    if (!stripElement) return;
    const selectors = this.dom.selectors;
    const classes = this.classes;
    const captureState = this.capture;
    const playbackState = this.playback;
    const trimState = this.trimState;
    const meta = this.resolveStripMeta(null);
    const isThisStrip = trimState.stripEl === stripElement;
    const isRecording = isThisStrip && (captureState.recording || captureState.starting);
    const isPlaying = isThisStrip && (playbackState.playing || playbackState.startingPlay);
    const hasFile = Boolean(
      meta.audioUrl ||
      (isThisStrip && captureState.rawFloatSamples && captureState.rawFloatSamples.length > 0) ||
      (isThisStrip && captureState.peaks && captureState.peaks.length > 0 && !isRecording)
    );

    stripElement.classList.toggle(classes.recording, isRecording);
    stripElement.classList.toggle(classes.playing, !isRecording && isPlaying);
    stripElement.classList.toggle(classes.ready, !isRecording && !isPlaying && hasFile);
    stripElement.classList.toggle(classes.idle, !isRecording && !isPlaying && !hasFile);

    const startRatio = isThisStrip ? trimState.trimStart : meta.trimStart;
    const endRatio = isThisStrip ? trimState.trimEnd : meta.trimEnd;
    const isTrimmed = !isRecording && hasFile && EditorVoiceWaveform.isTrimActive(startRatio, endRatio);

    const playButton = stripElement.querySelector(selectors.playBtn);
    if (playButton) {
      const playDisabled = isRecording || !hasFile;
      playButton.disabled = playDisabled;
      playButton.classList.toggle(classes.disabled, playDisabled);
      UIController.setHint(
        playButton,
        UIController.tr(isPlaying ? this.i18n.pauseHint : this.i18n.listenHint)
      );
    }

    const recordButton = stripElement.querySelector(selectors.recordBtn);
    if (recordButton) {
      UIController.setHint(
        recordButton,
        UIController.tr(isRecording ? this.i18n.stopSaveHint : this.i18n.startRecHint)
      );
    }

    const cutButton = stripElement.querySelector(selectors.cutBtn);
    if (cutButton) {
      const cutDisabled = isRecording || !isTrimmed;
      cutButton.disabled = cutDisabled;
      cutButton.classList.toggle(classes.disabled, cutDisabled);
    }

    const deleteButton = stripElement.querySelector(selectors.deleteBtn);
    if (deleteButton) {
      const deleteDisabled = isRecording || !hasFile;
      deleteButton.disabled = deleteDisabled;
      deleteButton.classList.toggle(classes.disabled, deleteDisabled);
    }
  },

  resetPlaybackAudio() {
    const playbackState = this.playback;
    if (playbackState.animFrame && !this.capture.recording) {
      cancelAnimationFrame(playbackState.animFrame);
      playbackState.animFrame = 0;
    }
    if (playbackState.audio) {
      try { playbackState.audio.pause(); } catch (_) {}
    }
    playbackState.audio = null;
    playbackState.sourceUrl = "";
    playbackState.playing = false;
    playbackState.startingPlay = false;
    playbackState.durationSec = 0;
    playbackState.scrubRatio = 0;
  },

  stopPreview(stripElement) {
    const playbackState = this.playback;
    if (playbackState.audio) {
      try { playbackState.audio.pause(); } catch (_) {}
    }
    playbackState.playing = false;
    playbackState.startingPlay = false;
    if (playbackState.animFrame && !this.capture.recording) {
      cancelAnimationFrame(playbackState.animFrame);
      playbackState.animFrame = 0;
    }
    this.syncStripState(stripElement || this.trimState.stripEl);
  },

  togglePreview(_params, targetElement) {
    const stripElement =
      targetElement && targetElement.closest
        ? targetElement.closest(this.dom.selectors.strip) || targetElement
        : this.trimState.stripEl;
    if (!stripElement) return;
    const playbackState = this.playback;
    const trimState = this.trimState;
    const captureState = this.capture;
    const spec = this.audioSpec;
    if (playbackState.playing || playbackState.startingPlay) {
      if (playbackState.audio && playbackState.durationSec > spec.minRatio) {
        playbackState.scrubRatio = Math.max(
          trimState.trimStart,
          Math.min(trimState.trimEnd, playbackState.audio.currentTime / playbackState.durationSec)
        );
      }
      this.stopPreview(stripElement);
      EditorVoiceWaveform.syncPlayhead(stripElement, playbackState.scrubRatio);
      return;
    }
    const meta = this.resolveStripMeta(null);
    if (!meta.audioUrl || captureState.recording || captureState.starting) return;
    trimState.stripEl = stripElement;
    trimState.sceneId = meta.sceneId;
    trimState.sectionKey = meta.sectionKey;
    if (!playbackState.audio || playbackState.sourceUrl !== meta.audioUrl) {
      if (playbackState.audio) {
        try { playbackState.audio.pause(); } catch (_) {}
      }
      playbackState.audio = new window.Audio(meta.audioUrl);
      playbackState.audio.preload = this.values.preloadAuto;
      playbackState.sourceUrl = meta.audioUrl;
    }
    const audioElement = playbackState.audio;
    const startPlay = () => {
      if (playbackState.audio !== audioElement) return;
      const durationSec =
        audioElement.duration && isFinite(audioElement.duration)
          ? audioElement.duration
          : playbackState.durationSec || spec.fallbackDurationSec;
      playbackState.durationSec = durationSec;
      const startRatio =
        playbackState.scrubRatio >= trimState.trimStart &&
        playbackState.scrubRatio < trimState.trimEnd - spec.minTrimGap
          ? playbackState.scrubRatio
          : trimState.trimStart;
      const startSec = startRatio * durationSec;
      const endSec = Math.max(startSec + spec.minPlaySpanSec, trimState.trimEnd * durationSec);
      if (
        audioElement.currentTime < startSec - spec.seekEpsilonSec ||
        audioElement.currentTime >= endSec - spec.minPlaySpanSec ||
        Math.abs((audioElement.currentTime || 0) - startSec) > spec.seekEpsilonSec
      ) {
        audioElement.currentTime = startSec;
      }
      playbackState.startingPlay = true;
      playbackState.playing = true;
      this.syncStripState(stripElement);
      EditorVoiceWaveform.syncPlayhead(stripElement, startRatio);

      const tick = () => {
        if (!playbackState.playing || playbackState.audio !== audioElement) {
          return;
        }
        if (audioElement.paused && !playbackState.startingPlay) {
          playbackState.playing = false;
          this.syncStripState(stripElement);
          return;
        }
        if (audioElement.currentTime >= endSec) {
          audioElement.pause();
          audioElement.currentTime = trimState.trimStart * durationSec;
          playbackState.scrubRatio = trimState.trimStart;
          playbackState.playing = false;
          playbackState.startingPlay = false;
          this.syncStripState(stripElement);
          EditorVoiceWaveform.syncPlayhead(stripElement, spec.playheadOff);
          const timerElement = stripElement.querySelector(this.dom.selectors.timer);
          if (timerElement) {
            timerElement.textContent = AudioDeckController.formatSeconds(
              durationSec * (trimState.trimEnd - trimState.trimStart)
            );
          }
          return;
        }
        const currentRatio = durationSec > spec.minRatio ? audioElement.currentTime / durationSec : spec.minRatio;
        playbackState.scrubRatio = currentRatio;
        EditorVoiceWaveform.syncPlayhead(stripElement, currentRatio);
        const timerElement = stripElement.querySelector(this.dom.selectors.timer);
        if (timerElement) {
          timerElement.textContent = AudioDeckController.formatSeconds(audioElement.currentTime);
        }
        playbackState.animFrame = requestAnimationFrame(tick);
      };

      const onPlayStarted = () => {
        if (!playbackState.playing || playbackState.audio !== audioElement) return;
        playbackState.startingPlay = false;
        if (playbackState.animFrame) cancelAnimationFrame(playbackState.animFrame);
        playbackState.animFrame = requestAnimationFrame(tick);
      };

      const playPromise = audioElement.play();
      if (playPromise && typeof playPromise.then === this.values.fnType) {
        playPromise.then(onPlayStarted).catch(() => {
          playbackState.startingPlay = false;
          playbackState.playing = false;
          this.syncStripState(stripElement);
        });
      } else {
        onPlayStarted();
      }
    };

    if (
      audioElement.readyState >= spec.readyStateHaveCurrentData ||
      typeof audioElement.addEventListener !== this.values.fnType
    ) {
      startPlay();
    } else {
      let started = false;
      const runOnce = () => {
        if (started) return;
        started = true;
        startPlay();
      };
      audioElement.addEventListener(this.values.eventCanPlay, runOnce, { once: true });
      audioElement.addEventListener(this.values.eventLoadedMeta, runOnce, { once: true });
    }
  },

  stopCaptureTracks() {
    const captureState = this.capture;
    captureState.sessionSeq += 1;
    if (captureState.nativePollTimer) {
      clearInterval(captureState.nativePollTimer);
      captureState.nativePollTimer = null;
    }
    if (captureState.androidPollTimer) {
      clearInterval(captureState.androidPollTimer);
      captureState.androidPollTimer = null;
    }
    if (captureState.processorNode) {
      try { captureState.processorNode.onaudioprocess = null; } catch (_) {}
      try { captureState.processorNode.disconnect(); } catch (_) {}
      captureState.processorNode = null;
    }
    if (captureState.gainNode) {
      try { captureState.gainNode.disconnect(); } catch (_) {}
      captureState.gainNode = null;
    }
    if (captureState.sourceNode) {
      try { captureState.sourceNode.disconnect(); } catch (_) {}
      captureState.sourceNode = null;
    }
    if (captureState.mediaStream) {
      try { captureState.mediaStream.getTracks().forEach((mediaTrack) => mediaTrack.stop()); } catch (_) {}
      captureState.mediaStream = null;
    }
  },

  encodeInt16Base64(floatSamples, startRatio, endRatio) {
    if (!floatSamples || !floatSamples.length) return "";
    const spec = this.audioSpec;
    const totalSamples = floatSamples.length;
    const startIndex = Math.max(0, Math.min(totalSamples - 1, Math.floor((startRatio || spec.minRatio) * totalSamples)));
    const endIndex = Math.max(startIndex + 1, Math.min(totalSamples, Math.ceil((endRatio || spec.maxRatio) * totalSamples)));
    const sampleCount = endIndex - startIndex;
    const int16Buffer = new Int16Array(sampleCount);
    for (let sampleIdx = 0; sampleIdx < sampleCount; sampleIdx++) {
      let sampleVal = floatSamples[startIndex + sampleIdx];
      if (sampleVal > spec.maxRatio) sampleVal = spec.maxRatio;
      else if (sampleVal < -spec.maxRatio) sampleVal = -spec.maxRatio;
      int16Buffer[sampleIdx] =
        sampleVal < spec.minRatio
          ? Math.round(sampleVal * spec.pcmNegScale)
          : Math.round(sampleVal * spec.pcmPosScale);
    }
    const byteView = new Uint8Array(int16Buffer.buffer);
    let binaryStr = "";
    for (let byteOffset = 0; byteOffset < byteView.length; byteOffset += spec.b64ChunkSize) {
      binaryStr += String.fromCharCode.apply(
        null,
        byteView.subarray(byteOffset, Math.min(byteView.length, byteOffset + spec.b64ChunkSize))
      );
    }
    return window.btoa ? window.btoa(binaryStr) : "";
  },

  startAndroidCapture(stripElement) {
    const captureState = this.capture;
    const playbackState = this.playback;
    const trimState = this.trimState;
    const spec = this.audioSpec;
    captureState.starting = true;
    captureState.recording = false;
    this.syncStripState(stripElement);
    let status = 0;
    try { status = Number(window.mojraAndroid.startAudioCapture()); } catch (_) { status = 0; }
    if (status !== spec.androidStatusOk && status !== spec.androidStatusPerm) {
      captureState.starting = false;
      this.syncStripState(stripElement);
      UIController.showToast(UIController.tr(this.i18n.errUnavailable));
      return;
    }
    captureState.starting = false;
    captureState.recording = true;
    captureState.androidMode = true;
    captureState.androidPendingPerm = status === spec.androidStatusPerm;
    if (window.mojraAndroid.getAudioSampleRate) {
      const reportedRate = Number(window.mojraAndroid.getAudioSampleRate());
      if (reportedRate >= spec.minSampleRate && reportedRate <= spec.maxSampleRate) {
        captureState.sampleRate = reportedRate;
      }
    }
    this.syncStripState(stripElement);
    let startWallMs = Date.now();
    captureState.androidPollTimer = setInterval(() => {
      if (!captureState.recording || !captureState.androidMode || !window.mojraAndroid) return;
      const rawPeak = String(window.mojraAndroid.pollAudioPeak() || "");
      if (rawPeak === this.peakStatus.pending) {
        startWallMs = Date.now();
        return;
      }
      if (captureState.androidPendingPerm) {
        captureState.androidPendingPerm = false;
        startWallMs = Date.now();
      }
      if (rawPeak === this.peakStatus.denied || rawPeak === this.peakStatus.error) {
        this.stopCaptureTracks();
        captureState.recording = false;
        captureState.androidMode = false;
        this.syncStripState(stripElement);
        UIController.showToast(
          rawPeak === this.peakStatus.denied
            ? UIController.tr(this.i18n.errDenied)
            : UIController.tr(this.i18n.errStartFailed)
        );
        return;
      }
      if (rawPeak === this.peakStatus.stopped) {
        this.stopAndSaveCapture(stripElement);
        return;
      }
      const peakVal = parseFloat(rawPeak);
      if (!isNaN(peakVal) && peakVal >= spec.minRatio) {
        captureState.peaks.push(Math.min(spec.maxRatio, peakVal * spec.nativePeakGain));
        if (captureState.peaks.length > spec.maxPeaks) captureState.peaks.shift();
      }
    }, spec.androidPollMs);
    const tickAndroid = () => {
      if (!captureState.recording || !captureState.androidMode) return;
      playbackState.durationSec = captureState.androidPendingPerm
        ? spec.minRatio
        : (Date.now() - startWallMs) / spec.msPerSec;
      const timerElement = stripElement.querySelector(this.dom.selectors.timer);
      if (timerElement) timerElement.textContent = AudioDeckController.formatSeconds(playbackState.durationSec);
      EditorVoiceWaveform.draw(stripElement, trimState, captureState, spec.playheadOff);
      playbackState.animFrame = requestAnimationFrame(tickAndroid);
    };
    playbackState.animFrame = requestAnimationFrame(tickAndroid);
  },

  async startNativeCapture(stripElement, sessionId) {
    const captureState = this.capture;
    const playbackState = this.playback;
    const trimState = this.trimState;
    const spec = this.audioSpec;
    const vals = this.values;
    captureState.starting = true;
    captureState.recording = false;
    captureState.nativeMode = true;
    this.syncStripState(stripElement);
    const decoded = await Dispatcher.post(this.actions.startCapture, {
      scene: trimState.sceneId,
      voice_section: trimState.sectionKey,
      silent: vals.trueStr
    });
    if (!captureState.starting || sessionId !== captureState.sessionSeq) {
      Dispatcher.post(this.actions.stopCapture, {
        scene: trimState.sceneId,
        voice_section: trimState.sectionKey,
        silent: vals.trueStr
      });
      return;
    }
    const peakHeader =
      (typeof StudioController !== vals.undefType && StudioController.data && StudioController.data.voicePeak) || "";
    if (!decoded || peakHeader === this.peakStatus.stopped) {
      captureState.starting = false;
      captureState.recording = false;
      captureState.nativeMode = false;
      this.syncStripState(stripElement);
      return;
    }
    captureState.starting = false;
    captureState.recording = true;
    this.syncStripState(stripElement);
    const startWallMs = Date.now();
    captureState.nativePollTimer = setInterval(() => {
      if (!captureState.recording || !captureState.nativeMode) return;
      Dispatcher.post(this.actions.pollCapture, { silent: vals.trueStr });
    }, spec.nativePollMs);
    const tickNative = () => {
      if (!captureState.recording || !captureState.nativeMode) return;
      playbackState.durationSec = (Date.now() - startWallMs) / spec.msPerSec;
      const timerElement = stripElement.querySelector(this.dom.selectors.timer);
      if (timerElement) timerElement.textContent = AudioDeckController.formatSeconds(playbackState.durationSec);
      EditorVoiceWaveform.draw(stripElement, trimState, captureState, spec.playheadOff);
      playbackState.animFrame = requestAnimationFrame(tickNative);
    };
    playbackState.animFrame = requestAnimationFrame(tickNative);
  },

  async startWebAudioCapture(stripElement, sessionId) {
    const captureState = this.capture;
    const playbackState = this.playback;
    const trimState = this.trimState;
    const spec = this.audioSpec;
    captureState.starting = true;
    captureState.recording = false;
    this.syncStripState(stripElement);
    try {
      const mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: false,
          noiseSuppression: false,
          autoGainControl: false,
          channelCount: spec.channels
        }
      });
      if (!captureState.starting || sessionId !== captureState.sessionSeq) {
        mediaStream.getTracks().forEach((mediaTrack) => mediaTrack.stop());
        return;
      }
      const AudioContextClass = window.AudioContext || window.webkitAudioContext;
      if (!AudioContextClass) {
        captureState.starting = false;
        mediaStream.getTracks().forEach((mediaTrack) => mediaTrack.stop());
        this.syncStripState(stripElement);
        UIController.showToast(UIController.tr(this.i18n.errWebAudio));
        return;
      }
      if (!captureState.audioCtx || captureState.audioCtx.state === this.values.audioClosed) {
        captureState.audioCtx = new AudioContextClass({ sampleRate: spec.defaultSampleRate });
      }
      if (captureState.audioCtx.state === this.values.audioSuspended) {
        await captureState.audioCtx.resume();
        if (!captureState.starting || sessionId !== captureState.sessionSeq) {
          mediaStream.getTracks().forEach((mediaTrack) => mediaTrack.stop());
          return;
        }
      }
      captureState.sampleRate = captureState.audioCtx.sampleRate || spec.defaultSampleRate;
      captureState.mediaStream = mediaStream;
      captureState.sourceNode = captureState.audioCtx.createMediaStreamSource(mediaStream);
      const processorNode = captureState.audioCtx.createScriptProcessor(
        spec.scriptBufferSize,
        spec.channels,
        spec.channels
      );
      captureState.processorNode = processorNode;
      captureState.starting = false;
      captureState.recording = true;
      this.syncStripState(stripElement);
      const samplesPerPeak = Math.max(
        spec.minPeakSamples,
        Math.floor(captureState.sampleRate / spec.peakRateDivisor)
      );
      processorNode.onaudioprocess = (audioEvent) => {
        if (!captureState.recording || sessionId !== captureState.sessionSeq) return;
        if (audioEvent.outputBuffer && typeof audioEvent.outputBuffer.getChannelData === this.values.fnType) {
          const outChannel = audioEvent.outputBuffer.getChannelData(0);
          if (outChannel && typeof outChannel.fill === this.values.fnType) outChannel.fill(0);
        }
        const inputChannel = audioEvent.inputBuffer.getChannelData(0);
        const sampleCopy = new Float32Array(inputChannel.length);
        sampleCopy.set(inputChannel);
        captureState.pcmChunks.push(sampleCopy);
        captureState.totalSamples += sampleCopy.length;
        playbackState.durationSec = captureState.totalSamples / captureState.sampleRate;
        for (let sampleIdx = 0; sampleIdx < sampleCopy.length; sampleIdx++) {
          const sampleAmp = Math.abs(sampleCopy[sampleIdx]);
          if (sampleAmp > captureState.peakWindowMax) captureState.peakWindowMax = sampleAmp;
          captureState.peakWindowSamples++;
          if (captureState.peakWindowSamples >= samplesPerPeak) {
            captureState.peaks.push(Math.min(spec.maxRatio, captureState.peakWindowMax * spec.webPeakGain));
            if (captureState.peaks.length > spec.maxPeaks) captureState.peaks.shift();
            captureState.peakWindowMax = 0;
            captureState.peakWindowSamples = 0;
          }
        }
      };
      captureState.sourceNode.connect(processorNode);
      if (typeof captureState.audioCtx.createGain === this.values.fnType) {
        const silentGain = captureState.audioCtx.createGain();
        silentGain.gain.value = 0;
        captureState.gainNode = silentGain;
        processorNode.connect(silentGain);
        silentGain.connect(captureState.audioCtx.destination);
      } else {
        processorNode.connect(captureState.audioCtx.destination);
      }
      const tickRecording = () => {
        if (!captureState.recording || sessionId !== captureState.sessionSeq) return;
        const timerElement = stripElement.querySelector(this.dom.selectors.timer);
        if (timerElement) timerElement.textContent = AudioDeckController.formatSeconds(playbackState.durationSec);
        EditorVoiceWaveform.draw(stripElement, trimState, captureState, spec.playheadOff);
        playbackState.animFrame = requestAnimationFrame(tickRecording);
      };
      playbackState.animFrame = requestAnimationFrame(tickRecording);
    } catch (_) {
      captureState.starting = false;
      this.syncStripState(stripElement);
      UIController.showToast(UIController.tr(this.i18n.errAccess));
    }
  },

  isNativeHostCaptureAvailable() {
    if (typeof window === this.values.undefType) return false;
    if (window.mojraNative) return true;
    return Boolean(window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.nativeHost);
  },

  async startCapture(stripElement, params) {
    if (!stripElement) return;
    const captureState = this.capture;
    const playbackState = this.playback;
    const trimState = this.trimState;
    const spec = this.audioSpec;
    if (captureState.recording || captureState.starting) return;
    this.resetPlaybackAudio();
    this.stopCaptureTracks();
    const sessionId = captureState.sessionSeq;

    const meta = this.resolveStripMeta(params);
    trimState.stripEl = stripElement;
    trimState.sceneId = meta.sceneId;
    trimState.sectionKey = meta.sectionKey;
    trimState.trimStart = spec.minRatio;
    trimState.trimEnd = spec.maxRatio;
    captureState.pcmChunks = [];
    captureState.totalSamples = 0;
    captureState.peaks = [];
    captureState.peakWindowMax = 0;
    captureState.peakWindowSamples = 0;
    captureState.rawFloatSamples = null;
    captureState.nativeMode = false;
    captureState.androidMode = false;
    captureState.androidPendingPerm = false;
    EditorVoiceWaveform.syncHandles(stripElement, trimState, playbackState, captureState);
    EditorVoiceWaveform.draw(stripElement, trimState, captureState, spec.playheadOff);

    if (typeof window !== this.values.undefType && window.mojraAndroid && window.mojraAndroid.startAudioCapture) {
      this.startAndroidCapture(stripElement);
      return;
    }
    const canUseWebMic = Boolean(
      typeof navigator !== this.values.undefType &&
      navigator.mediaDevices &&
      navigator.mediaDevices.getUserMedia &&
      !this.isNativeHostCaptureAvailable()
    );
    if (!canUseWebMic) {
      await this.startNativeCapture(stripElement, sessionId);
      return;
    }
    await this.startWebAudioCapture(stripElement, sessionId);
  },

  async stopAndSaveCapture(stripElement) {
    const captureState = this.capture;
    const playbackState = this.playback;
    const trimState = this.trimState;
    const vals = this.values;
    const spec = this.audioSpec;
    if (!captureState.recording && !captureState.starting) return;
    const wasNative = captureState.nativeMode;
    const wasAndroid = captureState.androidMode;
    captureState.starting = false;
    captureState.recording = false;
    captureState.nativeMode = false;
    captureState.androidMode = false;
    captureState.androidPendingPerm = false;
    if (playbackState.animFrame) {
      cancelAnimationFrame(playbackState.animFrame);
      playbackState.animFrame = 0;
    }
    this.stopCaptureTracks();
    this.resetPlaybackAudio();

    if (wasAndroid && window.mojraAndroid && window.mojraAndroid.stopAudioCaptureBase64) {
      let base64Pcm = "";
      try { base64Pcm = String(window.mojraAndroid.stopAudioCaptureBase64() || ""); } catch (_) { base64Pcm = ""; }
      this.syncStripState(stripElement);
      if (!base64Pcm) return;
      await Dispatcher.post(this.actions.saveRecording, {
        scene: trimState.sceneId,
        voice_section: trimState.sectionKey,
        sample_rate: String(Math.round(captureState.sampleRate || spec.defaultSampleRate)),
        start_ratio: vals.defaultRatioStart,
        end_ratio: vals.defaultRatioEnd,
        pcm_b64: base64Pcm
      });
      return;
    }

    if (wasNative) {
      this.syncStripState(stripElement);
      await Dispatcher.post(this.actions.stopCapture, {
        scene: trimState.sceneId,
        voice_section: trimState.sectionKey
      });
      return;
    }

    if (captureState.totalSamples > 0) {
      const mergedSamples = new Float32Array(captureState.totalSamples);
      let sampleOffset = 0;
      for (let chunkIdx = 0; chunkIdx < captureState.pcmChunks.length; chunkIdx++) {
        mergedSamples.set(captureState.pcmChunks[chunkIdx], sampleOffset);
        sampleOffset += captureState.pcmChunks[chunkIdx].length;
      }
      captureState.rawFloatSamples = mergedSamples;
      captureState.pcmChunks = [];
    }
    EditorVoiceWaveform.syncHandles(stripElement, trimState, playbackState, captureState);
    EditorVoiceWaveform.draw(stripElement, trimState, captureState, spec.playheadOff);
    if (!captureState.rawFloatSamples || captureState.rawFloatSamples.length < spec.minSaveSamples) {
      this.syncStripState(stripElement);
      return;
    }
    const base64Pcm = this.encodeInt16Base64(captureState.rawFloatSamples, spec.minRatio, spec.maxRatio);
    if (!base64Pcm) return;
    await Dispatcher.post(this.actions.saveRecording, {
      scene: trimState.sceneId,
      voice_section: trimState.sectionKey,
      sample_rate: String(Math.round(captureState.sampleRate || spec.defaultSampleRate)),
      start_ratio: vals.defaultRatioStart,
      end_ratio: vals.defaultRatioEnd,
      pcm_b64: base64Pcm
    });
  },

  handlePeakHeader(peakVal) {
    const captureState = this.capture;
    const spec = this.audioSpec;
    if (!captureState.nativeMode || (!captureState.recording && !captureState.starting)) return;
    if (peakVal === this.peakStatus.stopped) {
      if (captureState.starting) return;
      this.stopAndSaveCapture(this.trimState.stripEl);
      return;
    }
    const parsedPeak = parseFloat(peakVal);
    if (!isNaN(parsedPeak) && parsedPeak >= spec.minRatio) {
      captureState.peaks.push(Math.min(spec.maxRatio, parsedPeak * spec.nativePeakGain));
      if (captureState.peaks.length > spec.maxPeaks) captureState.peaks.shift();
    }
  },

  toggleRecord(params, buttonElement) {
    const stripElement = buttonElement.closest(this.dom.selectors.strip) || buttonElement;
    const captureState = this.capture;
    if (captureState.recording || captureState.starting) {
      this.stopAndSaveCapture(stripElement);
      return;
    }
    this.startCapture(stripElement, params);
  },

  async cutTrim(params, buttonElement) {
    const stripElement = buttonElement.closest(this.dom.selectors.strip) || buttonElement;
    const trimState = this.trimState;
    const captureState = this.capture;
    const spec = this.audioSpec;
    const meta = this.resolveStripMeta(params);
    const startRatio = trimState.stripEl === stripElement ? trimState.trimStart : meta.trimStart;
    const endRatio = trimState.stripEl === stripElement ? trimState.trimEnd : meta.trimEnd;
    if (!EditorVoiceWaveform.isTrimActive(startRatio, endRatio)) return;
    trimState.stripEl = stripElement;
    trimState.trimStart = startRatio;
    trimState.trimEnd = endRatio > startRatio ? endRatio : spec.maxRatio;
    const postParams = {
      scene: meta.sceneId,
      voice_section: meta.sectionKey,
      start_ratio: trimState.trimStart.toFixed(spec.ratioDecimals),
      end_ratio: trimState.trimEnd.toFixed(spec.ratioDecimals),
      destructive: this.values.trueStr
    };
    if (captureState.rawFloatSamples && captureState.rawFloatSamples.length > spec.minSaveSamples) {
      postParams.sample_rate = String(Math.round(captureState.sampleRate || spec.defaultSampleRate));
      postParams.pcm_b64 = this.encodeInt16Base64(captureState.rawFloatSamples, spec.minRatio, spec.maxRatio);
    }
    this.resetPlaybackAudio();
    trimState.trimStart = spec.minRatio;
    trimState.trimEnd = spec.maxRatio;
    await Dispatcher.post(this.actions.trimVoice, postParams);
  },

  startTrimDrag(params, handleElement) {
    const stripElement = handleElement.closest(this.dom.selectors.strip);
    if (!stripElement) return;
    const trimState = this.trimState;
    const meta = this.resolveStripMeta(params);
    if (trimState.stripEl !== stripElement) {
      trimState.stripEl = stripElement;
      trimState.sceneId = meta.sceneId;
      trimState.sectionKey = meta.sectionKey;
      trimState.trimStart = meta.trimStart;
      trimState.trimEnd = meta.trimEnd;
    }
    trimState.draggingHandle = handleElement.classList.contains(this.classes.trimEnd)
      ? this.handles.end
      : this.handles.start;
  },

  moveTrimDrag(params) {
    const trimState = this.trimState;
    const playbackState = this.playback;
    const captureState = this.capture;
    const spec = this.audioSpec;
    if (!trimState.draggingHandle || !trimState.stripEl) return;
    const wrapElement = trimState.stripEl.querySelector(this.dom.selectors.wrap);
    if (!wrapElement) return;
    const wrapRect = wrapElement.getBoundingClientRect();
    if (!wrapRect.width) return;
    const ratio = Math.max(
      spec.minRatio,
      Math.min(spec.maxRatio, ((params.client_x || 0) - wrapRect.left) / wrapRect.width)
    );
    if (trimState.draggingHandle === this.handles.start) {
      trimState.trimStart = Math.min(ratio, trimState.trimEnd - spec.minTrimGap);
    } else if (trimState.draggingHandle === this.handles.end) {
      trimState.trimEnd = Math.max(ratio, trimState.trimStart + spec.minTrimGap);
    } else if (trimState.draggingHandle === this.handles.scrub) {
      const clampedRatio = Math.max(trimState.trimStart, Math.min(trimState.trimEnd, ratio));
      playbackState.scrubRatio = clampedRatio;
      if (playbackState.audio && playbackState.durationSec > spec.minRatio) {
        playbackState.audio.currentTime = clampedRatio * playbackState.durationSec;
      }
      if (playbackState.durationSec > spec.minRatio) {
        const timerElement = trimState.stripEl.querySelector(this.dom.selectors.timer);
        if (timerElement) {
          timerElement.textContent = AudioDeckController.formatSeconds(clampedRatio * playbackState.durationSec);
        }
      }
      EditorVoiceWaveform.syncPlayhead(trimState.stripEl, clampedRatio);
      return;
    }
    EditorVoiceWaveform.syncHandles(trimState.stripEl, trimState, playbackState, captureState);
    EditorVoiceWaveform.draw(
      trimState.stripEl,
      trimState,
      captureState,
      playbackState.scrubRatio > spec.minRatio ? playbackState.scrubRatio : spec.playheadOff
    );
  },

  async endTrimDrag() {
    const trimState = this.trimState;
    const spec = this.audioSpec;
    if (!trimState.draggingHandle) return;
    const wasTrim =
      trimState.draggingHandle === this.handles.start || trimState.draggingHandle === this.handles.end;
    const stripElement = trimState.stripEl;
    trimState.draggingHandle = "";
    if (wasTrim && stripElement) {
      const startStr = trimState.trimStart.toFixed(spec.ratioDecimals);
      const endStr = trimState.trimEnd.toFixed(spec.ratioDecimals);
      const ctrlData = this.resolveStripControllerData();
      if (ctrlData) {
        ctrlData.voiceTrimStart = trimState.trimStart;
        ctrlData.voiceTrimEnd = trimState.trimEnd;
      }
      this.syncStripState(stripElement);
      const meta = this.resolveStripMeta(null);
      await Dispatcher.post(this.actions.trimVoice, {
        scene: trimState.sceneId || meta.sceneId,
        voice_section: trimState.sectionKey || meta.sectionKey,
        start_ratio: startStr,
        end_ratio: endStr,
        destructive: this.values.falseStr,
        silent: this.values.trueStr
      });
    }
  },

  startScrubDrag(params, wrapElement) {
    const stripElement = wrapElement.closest(this.dom.selectors.strip);
    if (!stripElement) return;
    const trimState = this.trimState;
    const playbackState = this.playback;
    const captureState = this.capture;
    const spec = this.audioSpec;
    const meta = this.resolveStripMeta(params);
    if (!meta.audioUrl || captureState.recording || captureState.starting) return;
    const wrapRect = wrapElement.getBoundingClientRect();
    if (!wrapRect.width) return;
    trimState.stripEl = stripElement;
    trimState.sceneId = meta.sceneId;
    trimState.sectionKey = meta.sectionKey;
    trimState.draggingHandle = this.handles.scrub;
    const clampedRatio = Math.max(
      trimState.trimStart,
      Math.min(trimState.trimEnd, ((params.client_x || 0) - wrapRect.left) / wrapRect.width)
    );
    playbackState.scrubRatio = clampedRatio;
    if (playbackState.audio && playbackState.durationSec > spec.minRatio) {
      playbackState.audio.currentTime = clampedRatio * playbackState.durationSec;
    }
    if (playbackState.durationSec > spec.minRatio) {
      const timerElement = stripElement.querySelector(this.dom.selectors.timer);
      if (timerElement) {
        timerElement.textContent = AudioDeckController.formatSeconds(clampedRatio * playbackState.durationSec);
      }
    }
    EditorVoiceWaveform.syncPlayhead(stripElement, clampedRatio);
  },

  initStrips(_params, mountedElement) {
    const strips = mountedElement
      ? [mountedElement]
      : Array.from(document.querySelectorAll(this.dom.selectors.strip));
    if (!strips.length) return;
    const trimState = this.trimState;
    const playbackState = this.playback;
    const captureState = this.capture;
    strips.forEach((stripElement) => {
      const meta = this.resolveStripMeta(null);
      if (captureState.recording || captureState.starting) {
        trimState.stripEl = stripElement;
      } else {
        trimState.stripEl = stripElement;
        trimState.sceneId = meta.sceneId;
        trimState.sectionKey = meta.sectionKey;
        trimState.trimStart = meta.trimStart;
        trimState.trimEnd = meta.trimEnd;
        captureState.rawFloatSamples = null;
        captureState.peaks = [];
        if (playbackState.sourceUrl !== meta.audioUrl) {
          this.resetPlaybackAudio();
          if (meta.audioUrl && typeof window !== this.values.undefType && window.Audio) {
            const preloadAudio = new window.Audio(meta.audioUrl);
            preloadAudio.preload = this.values.preloadAuto;
            playbackState.audio = preloadAudio;
            playbackState.sourceUrl = meta.audioUrl;
            if (typeof preloadAudio.addEventListener === this.values.fnType) {
              preloadAudio.addEventListener(
                this.values.eventLoadedMeta,
                () => {
                  if (
                    playbackState.audio === preloadAudio &&
                    preloadAudio.duration &&
                    isFinite(preloadAudio.duration)
                  ) {
                    playbackState.durationSec = preloadAudio.duration;
                  }
                },
                { once: true }
              );
            }
          }
        }
      }
      EditorVoiceWaveform.syncHandles(stripElement, trimState, playbackState, captureState);
      EditorVoiceWaveform.loadPeaks(stripElement, trimState, captureState);
    });
  }
};
Dispatcher.register(EditorVoice.moduleName, EditorVoice);
