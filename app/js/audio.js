"use strict";

function formatSeconds(sec) {
  if (!Number.isFinite(sec) || sec < 0) return "0:00";
  const total = Math.floor(sec);
  const m = Math.floor(total / 60);
  const s = total % 60;
  return m + ":" + (s < 10 ? "0" + s : s);
}

function setPlayButtonVisual(isPlaying) {
  const playIcon = qs("#icon-audio-play");
  const pauseIcon = qs("#icon-audio-pause");
  if (playIcon) playIcon.classList.toggle("hidden", isPlaying);
  if (pauseIcon) pauseIcon.classList.toggle("hidden", !isPlaying);
}

let autoAdvanceFallbackTimer = null;
let pendingSceneAutoPlay = false;
let sceneAudioEndedHandled = false;
let activePrefetchController = null;
const prefetchedMediaCache = new Set();

function parseAudioTrimBounds(audio) {
  if (!audio) return { startRatio: 0, endRatio: 1 };
  const s = parseFloat(audio.getAttribute("data-trim-start") || "0");
  const e = parseFloat(audio.getAttribute("data-trim-end") || "1");
  let startRatio = Number.isFinite(s) && s >= 0 && s < 1 ? s : 0;
  let endRatio = Number.isFinite(e) && e > 0 && e <= 1 ? e : 1;
  if (endRatio <= startRatio + 0.002) {
    startRatio = 0;
    endRatio = 1;
  }
  return { startRatio: startRatio, endRatio: endRatio };
}

function getEffectiveAudioBounds(audio) {
  const rawDur =
    audio && audio.duration > 0 && Number.isFinite(audio.duration) ? audio.duration : 0;
  const trim = parseAudioTrimBounds(audio);
  const startSec = rawDur * trim.startRatio;
  const endSec = rawDur * trim.endRatio;
  const effectiveDur = Math.max(0, endSec - startSec);
  return {
    startRatio: trim.startRatio,
    endRatio: trim.endRatio,
    startSec: startSec,
    endSec: endSec,
    effectiveDur: effectiveDur
  };
}

function scheduleAutoChoiceAdvance(audio) {
  if (autoAdvanceFallbackTimer) {
    clearTimeout(autoAdvanceFallbackTimer);
    autoAdvanceFallbackTimer = null;
  }
  if (!audio || audio.getAttribute("data-auto-choice") !== "true") {
    return;
  }
  const hasNextChoices =
    qsa('#active-scene-stage .choice-btn[action="ChooseBranch"]:not(.choice-locked)').length > 0;
  if (!hasNextChoices) {
    return;
  }
  const delaySec = parseFloat(audio.getAttribute("data-auto-delay") || "2");
  const delayMs = Math.max(0, (Number.isFinite(delaySec) && delaySec > 0 ? delaySec : 2.0) * 1000);
  autoAdvanceFallbackTimer = setTimeout(() => {
    autoAdvanceFallbackTimer = null;
    const curAudio = qs("#story-audio");
    if (curAudio && curAudio.getAttribute("data-auto-choice") === "true") {
      sendServerAction("AudioEnded", {});
    }
  }, delayMs);
}

function cancelMediaPrefetchQueue() {
  if (activePrefetchController) {
    activePrefetchController.abort();
    activePrefetchController = null;
  }
}

function waitForImageReady(imgEl, signal) {
  return new Promise((resolve) => {
    if (!imgEl || !imgEl.getAttribute("src") || imgEl.complete) {
      resolve();
      return;
    }
    const done = () => {
      imgEl.removeEventListener("load", done);
      imgEl.removeEventListener("error", done);
      resolve();
    };
    imgEl.addEventListener("load", done, { once: true });
    imgEl.addEventListener("error", done, { once: true });
    if (signal) {
      signal.addEventListener("abort", done, { once: true });
    }
  });
}

function waitForAudioReady(audioEl, signal) {
  return new Promise((resolve) => {
    if (!audioEl || !audioEl.getAttribute("data-audio-url") || audioEl.readyState >= 4) {
      resolve();
      return;
    }
    const done = () => {
      audioEl.removeEventListener("canplaythrough", done);
      audioEl.removeEventListener("error", done);
      resolve();
    };
    audioEl.addEventListener("canplaythrough", done, { once: true });
    audioEl.addEventListener("error", done, { once: true });
    if (signal) {
      signal.addEventListener("abort", done, { once: true });
    }
  });
}

async function scheduleCurrentSceneMediaPrefetch(audioEl) {
  cancelMediaPrefetchQueue();
  const stage = qs("#active-scene-stage");
  if (!stage || typeof AbortController === "undefined") return;

  const rawUrls = stage.getAttribute("data-prefetch-urls") || "";
  if (!rawUrls) return;

  const urls = rawUrls
    .split(";")
    .map((u) => u.trim())
    .filter(Boolean);
  if (urls.length === 0) return;

  const controller = new AbortController();
  activePrefetchController = controller;
  const signal = controller.signal;

  const bgImg = qs("#player-bg-img");
  const portraitImg = qs(".scene-portrait-img", stage);
  await Promise.all([
    waitForAudioReady(audioEl, signal),
    waitForImageReady(bgImg, signal),
    waitForImageReady(portraitImg, signal)
  ]);

  if (signal.aborted) return;

  for (let i = 0; i < urls.length; i++) {
    if (signal.aborted) return;
    const targetUrl = urls[i];
    if (!targetUrl || prefetchedMediaCache.has(targetUrl)) {
      continue;
    }
    try {
      const resp = await fetch(targetUrl, {
        method: "GET",
        credentials: "same-origin",
        signal: signal
      });
      if (resp && resp.ok) {
        await resp.arrayBuffer();
        prefetchedMediaCache.add(targetUrl);
      }
    } catch (_) {
      if (signal.aborted) return;
    }
  }
}

function initAudioController(shouldAutoPlay, preservedTime) {
  if (autoAdvanceFallbackTimer) {
    clearTimeout(autoAdvanceFallbackTimer);
    autoAdvanceFallbackTimer = null;
  }
  pendingSceneAutoPlay = false;
  sceneAudioEndedHandled = false;
  cancelMediaPrefetchQueue();

  const audio = qs("#story-audio");
  if (!audio) {
    scheduleCurrentSceneMediaPrefetch(null);
    return;
  }

  const url = audio.getAttribute("data-audio-url") || "";
  const resumeTime =
    preservedTime !== null && preservedTime !== undefined
      ? preservedTime
      : parseFloat(audio.getAttribute("data-resume-time") || "0") || 0;
  const speed = parseFloat(audio.getAttribute("data-speed") || "1") || 1.0;
  const autoChoiceOn = audio.getAttribute("data-auto-choice") === "true";
  const slider = qs("#audio-seek-slider");
  const playBtn = qs("#btn-audio-play");
  const hasNextChoices = qsa('#active-scene-stage .choice-btn[action="ChooseBranch"]:not(.choice-locked)').length > 0;

  const setBufferingState = (buffering) => {
    if (playBtn) {
      playBtn.classList.toggle("is-buffering", Boolean(buffering));
    }
  };

  const applyInitialPositionAndTotal = () => {
    const bounds = getEffectiveAudioBounds(audio);
    const tot = qs("#audio-time-total");
    if (tot && bounds.effectiveDur > 0) {
      tot.textContent = formatSeconds(bounds.effectiveDur);
    }
    if (bounds.effectiveDur > 0) {
      if (preservedTime !== null && preservedTime !== undefined && preservedTime > 0.2 && preservedTime < bounds.endSec - 0.1) {
        audio.currentTime = Math.max(bounds.startSec, preservedTime);
      } else if (resumeTime > 0.2 && bounds.startSec + resumeTime < bounds.endSec - 0.2) {
        audio.currentTime = bounds.startSec + resumeTime;
      } else if (bounds.startSec > 0 && audio.currentTime < bounds.startSec) {
        audio.currentTime = bounds.startSec;
      }
      const relCur = Math.max(0, Math.min(bounds.effectiveDur, audio.currentTime - bounds.startSec));
      const curEl = qs("#audio-time-current");
      if (curEl) curEl.textContent = formatSeconds(relCur);
      if (slider) {
        const val = Math.round((relCur / bounds.effectiveDur) * 1000);
        slider.value = String(val);
        slider.style.setProperty("--seek-pct", (val / 10).toFixed(1) + "%");
      }
    }
  };

  if (url) {
    audio.preload = "auto";
    audio.src = url;
    audio.playbackRate = speed;
    if (audio.readyState < 4) {
      setBufferingState(true);
      const clearBuffering = () => setBufferingState(false);
      audio.addEventListener("canplaythrough", clearBuffering, { once: true });
      audio.addEventListener("playing", clearBuffering, { once: true });
      audio.addEventListener("error", clearBuffering, { once: true });
    }
    audio.addEventListener("loadedmetadata", applyInitialPositionAndTotal, { once: true });
    audio.load();
    if (audio.duration > 0) {
      applyInitialPositionAndTotal();
    }
    if (shouldAutoPlay) {
      pendingSceneAutoPlay = true;
      setPlayButtonVisual(true);
      let autoPlayStarted = false;
      const startPlay = () => {
        if (autoPlayStarted || !pendingSceneAutoPlay) return;
        autoPlayStarted = true;
        setBufferingState(false);
        audio.playbackRate = speed;
        if (audio.duration > 0) {
          const bounds = getEffectiveAudioBounds(audio);
          if (bounds.startSec > 0 && audio.currentTime < bounds.startSec) {
            try {
              audio.currentTime = bounds.startSec;
            } catch (_) {}
          }
        }
        const playPromise = audio.play();
        if (playPromise && typeof playPromise.then === "function") {
          playPromise
            .then(() => {
              pendingSceneAutoPlay = false;
              setBufferingState(false);
              setPlayButtonVisual(true);
            })
            .catch(() => {
              if (audio.readyState >= 2) {
                pendingSceneAutoPlay = false;
                setBufferingState(false);
                setPlayButtonVisual(false);
              } else {
                autoPlayStarted = false;
              }
            });
        } else {
          pendingSceneAutoPlay = false;
          setPlayButtonVisual(!audio.paused);
        }
      };
      startPlay();
      if (audio.paused && pendingSceneAutoPlay) {
        audio.addEventListener("canplaythrough", startPlay, { once: true });
      }
    }
  } else if (shouldAutoPlay && autoChoiceOn && hasNextChoices) {
    scheduleAutoChoiceAdvance(audio);
  }

  scheduleCurrentSceneMediaPrefetch(audio);

  const handleAudioReachedEnd = () => {
    if (sceneAudioEndedHandled) return;
    sceneAudioEndedHandled = true;
    pendingSceneAutoPlay = false;
    if (!audio.paused) {
      audio.pause();
    }
    setPlayButtonVisual(false);
    const bounds = getEffectiveAudioBounds(audio);
    const cur = qs("#audio-time-current");
    if (cur && bounds.effectiveDur > 0) {
      cur.textContent = formatSeconds(bounds.effectiveDur);
    }
    if (slider) {
      slider.value = "1000";
      slider.style.setProperty("--seek-pct", "100%");
    }
    sendServerAction("SaveAudioTime", { time: 0 });
    scheduleAutoChoiceAdvance(audio);
  };

  if (playBtn) {
    playBtn.addEventListener("click", () => {
      const currentUrl = audio.getAttribute("data-audio-url") || "";
      if (!currentUrl) {
        showToast(tr("no_scene_audio_toast"));
        return;
      }
      if (autoAdvanceFallbackTimer) {
        clearTimeout(autoAdvanceFallbackTimer);
        autoAdvanceFallbackTimer = null;
      }
      if (!audio.paused || pendingSceneAutoPlay) {
        pendingSceneAutoPlay = false;
        autoPlayChainActive = false;
        setBufferingState(false);
        audio.pause();
        setPlayButtonVisual(false);
      } else {
        sceneAudioEndedHandled = false;
        if (audio.getAttribute("data-auto-choice") === "true") {
          autoPlayChainActive = true;
        }
        const curSpeed = parseFloat(audio.getAttribute("data-speed") || "1") || 1.0;
        audio.playbackRate = curSpeed;
        if (audio.getAttribute("src") !== currentUrl && !String(audio.src || "").endsWith(currentUrl)) {
          audio.src = currentUrl;
          audio.load();
        }
        if (audio.duration > 0) {
          const bounds = getEffectiveAudioBounds(audio);
          if (audio.ended || audio.currentTime >= bounds.endSec - 0.05 || audio.currentTime < bounds.startSec) {
            try {
              audio.currentTime = bounds.startSec;
            } catch (_) {}
          }
        }
        setPlayButtonVisual(true);
        const playPromise = audio.play();
        if (playPromise && typeof playPromise.catch === "function") {
          playPromise.catch(() => {
            setBufferingState(false);
            setPlayButtonVisual(false);
          });
        }
      }
    });
  }

  audio.addEventListener("play", () => {
    sceneAudioEndedHandled = false;
    pendingSceneAutoPlay = false;
    setPlayButtonVisual(true);
  });
  audio.addEventListener("pause", () => {
    if (pendingSceneAutoPlay) return;
    setPlayButtonVisual(false);
    if (!audio.ended && !sceneAudioEndedHandled && audio.currentTime > 0) {
      const bounds = getEffectiveAudioBounds(audio);
      const relCur = Math.max(0, audio.currentTime - bounds.startSec);
      sendServerAction("SaveAudioTime", { time: Math.floor(relCur * 10) / 10 });
    }
  });
  audio.addEventListener("loadedmetadata", applyInitialPositionAndTotal);

  audio.addEventListener("timeupdate", () => {
    const bounds = getEffectiveAudioBounds(audio);
    if (bounds.endRatio < 0.999 && bounds.endSec > 0 && audio.currentTime >= bounds.endSec) {
      handleAudioReachedEnd();
      return;
    }
    const relCur = Math.max(0, audio.currentTime - bounds.startSec);
    const cur = qs("#audio-time-current");
    if (cur) cur.textContent = formatSeconds(relCur);
    if (slider && bounds.effectiveDur > 0) {
      const val = Math.min(1000, Math.max(0, Math.round((relCur / bounds.effectiveDur) * 1000)));
      slider.value = String(val);
      slider.style.setProperty("--seek-pct", (val / 10).toFixed(1) + "%");
    }
    const now = Date.now();
    if (now - lastAudioSaveAt > 2000 && !sceneAudioEndedHandled) {
      lastAudioSaveAt = now;
      sendServerAction("SaveAudioTime", { time: Math.floor(relCur * 10) / 10 });
    }
  });

  audio.addEventListener("ended", handleAudioReachedEnd);

  if (slider) {
    slider.addEventListener("input", () => {
      if (autoAdvanceFallbackTimer) {
        clearTimeout(autoAdvanceFallbackTimer);
        autoAdvanceFallbackTimer = null;
      }
      sceneAudioEndedHandled = false;
      const val = parseFloat(slider.value) || 0;
      slider.style.setProperty("--seek-pct", (val / 10).toFixed(1) + "%");
      const bounds = getEffectiveAudioBounds(audio);
      if (bounds.effectiveDur > 0) {
        audio.currentTime = bounds.startSec + (val / 1000) * bounds.effectiveDur;
      }
    });
    slider.addEventListener("change", () => {
      const bounds = getEffectiveAudioBounds(audio);
      if (bounds.effectiveDur > 0) {
        const relCur = Math.max(0, audio.currentTime - bounds.startSec);
        sendServerAction("SaveAudioTime", { time: Math.floor(relCur * 10) / 10 });
      }
    });
  }
}
