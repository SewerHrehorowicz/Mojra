"use strict";

const AudioDeckController = {
  data: {
    audioQueue: "",
    audioTotalDuration: 0,
    resumeIndex: 0,
    resumeOffsetSec: 0,
    audioSpeed: 1,
    autoChoiceEnabled: false,
    autoChoiceLabel: "",
    autoChoiceDelayMs: 2000,
    canAutoAdvance: false,
    noAudioToast: "",
    currentSegmentIndex: 0,
    preloadUrl: "",
    nextAudioPreload: null,
    autoAdvanceTimer: null,
    pendingAutoPlay: false,
    endedHandled: false,
    lastSaveAt: 0,
    queueSegments: []
  },

  bindings() {
    return {
      "#story-audio": { init: this.initController, timeupdate: this.onTimeUpdate, ended: this.onEnded },
      "#btn-audio-play": { click: this.togglePlay },
      "#audio-seek-slider": { input: this.onSeekInput, change: this.onSeekChange },
      "#btn-audio-speed": { click: this.cycleSpeed },
      "#btn-auto-choice-toggle": { click: this.toggleAutoChoice }
    };
  },

  settings: {
    zeroTime: "0:00",
    seekPctVar: "--seek-pct",
    saveThrottleMs: 2000,
    sliderMax: 1000,
    sliderPctDiv: 10,
    endMarginSec: 0.05,
    selectors: {
      storyAudio: "#story-audio",
      playIcon: "#icon-audio-play",
      pauseIcon: "#icon-audio-pause",
      timeCurrent: "#audio-time-current",
      seekSlider: "#audio-seek-slider",
      speedBtn: "#btn-audio-speed",
      autoChoiceBtn: "#btn-auto-choice-toggle"
    },
    classes: {
      hidden: "hidden",
      active: "active"
    },
    events: {
      canPlayThrough: "canplaythrough"
    },
    actions: {
      audioEnded: "AudioEnded",
      saveAudioTime: "SaveAudioTime",
      cycleSpeed: "CycleAudioSpeed",
      toggleAutoChoice: "ToggleAutoChoice"
    }
  },

  formatSeconds(sec) {
    if (!Number.isFinite(sec) || sec < 0) return this.settings.zeroTime;
    const total = Math.floor(sec);
    const s = total % 60;
    return Math.floor(total / 60) + ":" + (s < 10 ? "0" + s : s);
  },

  setPlayButtonVisual(isPlaying) {
    const s = this.settings;
    const playIcon = document.querySelector(s.selectors.playIcon);
    const pauseIcon = document.querySelector(s.selectors.pauseIcon);
    if (playIcon) playIcon.classList.toggle(s.classes.hidden, isPlaying);
    if (pauseIcon) pauseIcon.classList.toggle(s.classes.hidden, !isPlaying);
  },

  clearAutoTimer() {
    if (this.data.autoAdvanceTimer) {
      clearTimeout(this.data.autoAdvanceTimer);
      this.data.autoAdvanceTimer = null;
    }
  },

  syncControls() {
    const s = this.settings;
    const audio = document.querySelector(s.selectors.storyAudio);
    if (audio) audio.playbackRate = this.data.audioSpeed || 1;
    const speedBtn = document.querySelector(s.selectors.speedBtn);
    if (speedBtn) speedBtn.textContent = this.data.audioSpeed + "x";
    const autoBtn = document.querySelector(s.selectors.autoChoiceBtn);
    if (autoBtn) {
      autoBtn.classList.toggle(s.classes.active, this.data.autoChoiceEnabled);
      AppShellController.setHint(autoBtn, this.data.autoChoiceLabel);
    }
    if (!this.data.autoChoiceEnabled) this.clearAutoTimer();
  },

  async cycleSpeed() {
    await Dispatcher.post(this.settings.actions.cycleSpeed, {});
    this.syncControls();
  },

  async toggleAutoChoice() {
    await Dispatcher.post(this.settings.actions.toggleAutoChoice, {});
    this.syncControls();
  },

  parseSceneAudioQueue(queueSpec) {
    const raw = queueSpec !== undefined ? queueSpec : this.data.audioQueue;
    if (!raw) return [];
    const out = [];
    const parts = raw.split(";");
    for (let i = 0; i < parts.length; i++) {
      const fields = parts[i].split("|");
      if (!fields[0]) continue;
      out.push({
        url: fields[0],
        startSec: Number(fields[1]) || 0,
        endSec: Number(fields[2]) || 0,
        durationSec: Number(fields[3]) || 0
      });
    }
    return out;
  },

  currentElapsed(audio) {
    const segs = this.data.queueSegments;
    if (!segs.length) return 0;
    const curIdx = this.data.currentSegmentIndex;
    let elapsed = 0;
    for (let i = 0; i < curIdx && i < segs.length; i++) elapsed += segs[i].durationSec;
    const seg = segs[curIdx];
    if (!seg) return elapsed;
    const local = Math.max(0, ((audio && audio.currentTime) || 0) - seg.startSec);
    return elapsed + (seg.durationSec > 0 ? Math.min(seg.durationSec, local) : local);
  },

  mapGlobalTime(globalTimeSec) {
    const segs = this.data.queueSegments;
    const clamped = Math.max(0, Number(globalTimeSec) || 0);
    let accum = 0;
    for (let i = 0; i < segs.length; i++) {
      const seg = segs[i];
      if (clamped <= accum + seg.durationSec || i === segs.length - 1) {
        const local = seg.durationSec > 0 ? Math.max(0, Math.min(seg.durationSec, clamped - accum)) : Math.max(0, clamped - accum);
        return { index: i, localOffset: local, rawAudioTime: seg.startSec + local };
      }
      accum += seg.durationSec;
    }
    return { index: 0, localOffset: 0, rawAudioTime: 0 };
  },

  preloadNextSegment(nextIdx) {
    const seg = this.data.queueSegments[nextIdx];
    if (!seg || !seg.url) return;
    if (!this.data.nextAudioPreload && window.Audio) {
      try { this.data.nextAudioPreload = new window.Audio(); } catch (_) { this.data.nextAudioPreload = null; }
    }
    const pre = this.data.nextAudioPreload;
    if (!pre || this.data.preloadUrl === seg.url) return;
    this.data.preloadUrl = seg.url;
    pre.src = seg.url;
  },

  scheduleAutoChoiceAdvance() {
    this.clearAutoTimer();
    const s = this.settings;
    if (!this.data.autoChoiceEnabled || !this.data.canAutoAdvance) return;
    this.data.autoAdvanceTimer = setTimeout(() => {
      this.data.autoAdvanceTimer = null;
      if (this.data.autoChoiceEnabled && document.querySelector(s.selectors.storyAudio)) {
        Dispatcher.post(s.actions.audioEnded, {});
      }
    }, Math.max(0, this.data.autoChoiceDelayMs));
  },

  updateProgressDeckUI(audio, elapsedOverride) {
    const s = this.settings;
    const el = audio || document.querySelector(s.selectors.storyAudio);
    if (!el) return;
    const totalDur = this.data.audioTotalDuration;
    const globalCur = elapsedOverride !== undefined
      ? Math.max(0, Math.min(totalDur || elapsedOverride, elapsedOverride))
      : this.currentElapsed(el);
    const curEl = document.querySelector(s.selectors.timeCurrent);
    if (curEl) curEl.textContent = this.formatSeconds(globalCur);
    const slider = document.querySelector(s.selectors.seekSlider);
    if (slider && totalDur > 0) {
      const val = Math.min(s.sliderMax, Math.max(0, Math.round((globalCur / totalDur) * s.sliderMax)));
      slider.value = String(val);
      slider.style.setProperty(s.seekPctVar, (val / s.sliderPctDiv).toFixed(1) + "%");
    }
  },

  startPlayback(audio, retryOnCanPlay) {
    const s = this.settings;
    this.data.pendingAutoPlay = true;
    this.setPlayButtonVisual(true);
    let started = false;
    const run = () => {
      if (started || !this.data.pendingAutoPlay) return;
      started = true;
      const seg = this.data.queueSegments[this.data.currentSegmentIndex];
      if (seg && seg.startSec > 0 && audio.currentTime < seg.startSec) {
        try { audio.currentTime = seg.startSec; } catch (_) {}
      }
      const p = audio.play();
      if (p && p.then) {
        p.then(() => {
          this.data.pendingAutoPlay = false;
          this.setPlayButtonVisual(true);
        }).catch(() => {
          if (!retryOnCanPlay || audio.readyState >= 2) {
            this.data.pendingAutoPlay = false;
            this.setPlayButtonVisual(false);
          } else {
            started = false;
          }
        });
      } else {
        this.data.pendingAutoPlay = false;
        this.setPlayButtonVisual(!audio.paused);
      }
    };
    run();
    if (retryOnCanPlay && audio.paused && this.data.pendingAutoPlay && audio.addEventListener) {
      audio.addEventListener(s.events.canPlayThrough, run, { once: true });
    }
  },

  switchQueueSegment(audio, targetIdx, localOffsetSec, autoPlay) {
    const seg = this.data.queueSegments[targetIdx];
    if (!audio || !seg) return false;
    this.data.currentSegmentIndex = targetIdx;
    audio.playbackRate = this.data.audioSpeed || 1;
    if (audio.src !== seg.url) {
      audio.src = seg.url;
      if (audio.load) audio.load();
    }
    try { audio.currentTime = seg.startSec + (Number(localOffsetSec) || 0); } catch (_) {}
    this.updateProgressDeckUI(audio);
    this.preloadNextSegment(targetIdx + 1);
    if (autoPlay) this.startPlayback(audio, false);
    return true;
  },

  togglePlay() {
    const s = this.settings;
    const audio = document.querySelector(s.selectors.storyAudio);
    if (!audio) return;
    const segs = this.data.queueSegments;
    const curIdx = this.data.currentSegmentIndex;
    const seg = segs[curIdx];
    if (!seg || !seg.url) {
      AppShellController.showToast(this.data.noAudioToast);
      return;
    }
    this.clearAutoTimer();
    if (!audio.paused || this.data.pendingAutoPlay) {
      this.data.pendingAutoPlay = false;
      audio.pause();
      this.setPlayButtonVisual(false);
      return;
    }
    const wasAtEnd = this.data.endedHandled;
    this.data.endedHandled = false;
    if (wasAtEnd && segs.length > 1) {
      this.switchQueueSegment(audio, 0, 0, true);
      return;
    }
    const restart = wasAtEnd || audio.ended || (seg.endSec > 0 && audio.currentTime >= seg.endSec - s.endMarginSec) || audio.currentTime < seg.startSec;
    this.switchQueueSegment(audio, curIdx, restart ? 0 : Math.max(0, audio.currentTime - seg.startSec), true);
  },

  onTimeUpdate(params, el) {
    const s = this.settings;
    const audio = el || document.querySelector(s.selectors.storyAudio);
    if (!audio) return;
    const seg = this.data.queueSegments[this.data.currentSegmentIndex];
    if (seg && seg.endSec > 0 && audio.currentTime >= seg.endSec) {
      this.onEnded(params, audio);
      return;
    }
    const globalElapsed = this.currentElapsed(audio);
    this.updateProgressDeckUI(audio, globalElapsed);
    const now = Date.now();
    if (now - this.data.lastSaveAt > s.saveThrottleMs && !this.data.endedHandled) {
      this.data.lastSaveAt = now;
      Dispatcher.post(s.actions.saveAudioTime, { time: globalElapsed });
    }
  },

  onEnded(_params, el) {
    const s = this.settings;
    const audio = el || document.querySelector(s.selectors.storyAudio);
    if (!audio) return;
    const curIdx = this.data.currentSegmentIndex;
    if (curIdx + 1 < this.data.queueSegments.length) {
      this.data.endedHandled = false;
      this.switchQueueSegment(audio, curIdx + 1, 0, true);
      return;
    }
    if (this.data.endedHandled) return;
    this.data.endedHandled = true;
    this.data.pendingAutoPlay = false;
    if (!audio.paused) audio.pause();
    this.setPlayButtonVisual(false);
    this.updateProgressDeckUI(audio, this.data.audioTotalDuration);
    Dispatcher.post(s.actions.saveAudioTime, { time: 0 });
    this.scheduleAutoChoiceAdvance();
  },

  onSeekInput(_params, slider) {
    const s = this.settings;
    const audio = document.querySelector(s.selectors.storyAudio);
    if (!audio || !slider) return;
    this.clearAutoTimer();
    this.data.endedHandled = false;
    const totalDur = this.data.audioTotalDuration;
    if (totalDur <= 0) return;
    const targetSec = ((Number(slider.value) || 0) / s.sliderMax) * totalDur;
    const mapped = this.mapGlobalTime(targetSec);
    if (mapped.index !== this.data.currentSegmentIndex) {
      this.switchQueueSegment(audio, mapped.index, mapped.localOffset, !audio.paused || this.data.pendingAutoPlay);
    } else {
      try { audio.currentTime = mapped.rawAudioTime; } catch (_) {}
      this.updateProgressDeckUI(audio, targetSec);
    }
  },

  onSeekChange() {
    const s = this.settings;
    const audio = document.querySelector(s.selectors.storyAudio);
    if (audio && this.data.audioTotalDuration > 0) {
      Dispatcher.post(s.actions.saveAudioTime, { time: this.currentElapsed(audio) });
    }
  },

  initController(_params, el) {
    this.clearAutoTimer();
    this.data.pendingAutoPlay = false;
    this.data.endedHandled = false;
    const audio = (el && el.tagName ? el : null) || document.querySelector(this.settings.selectors.storyAudio);
    if (!audio) return;
    const segs = this.parseSceneAudioQueue(this.data.audioQueue);
    this.data.queueSegments = segs;
    this.data.currentSegmentIndex = 0;
    if (!segs.length) {
      if (this.data.autoChoiceEnabled) this.scheduleAutoChoiceAdvance();
      return;
    }
    this.switchQueueSegment(audio, this.data.resumeIndex, this.data.resumeOffsetSec, false);
    this.startPlayback(audio, true);
  }
};
Dispatcher.register("AudioDeckController", AudioDeckController);
