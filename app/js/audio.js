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
let activePrefetchController = null;
const prefetchedMediaCache = new Set();

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
  const hasNextChoices = qsa('#active-scene-stage .choice-btn[action="ChooseBranch"]').length > 0;

  const setBufferingState = (buffering) => {
    if (playBtn) {
      playBtn.classList.toggle("is-buffering", Boolean(buffering));
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
    if (resumeTime > 0.2) {
      audio.addEventListener(
        "loadedmetadata",
        () => {
          if (resumeTime < (audio.duration || 0) - 0.2) {
            audio.currentTime = resumeTime;
          }
        },
        { once: true }
      );
    }
    audio.load();
    if (shouldAutoPlay) {
      let autoPlayStarted = false;
      const startPlay = () => {
        if (autoPlayStarted) return;
        autoPlayStarted = true;
        setBufferingState(false);
        audio.playbackRate = speed;
        audio.play().catch(() => {});
      };
      if (audio.readyState >= 4) {
        startPlay();
      } else {
        setBufferingState(true);
        audio.addEventListener("canplaythrough", startPlay, { once: true });
      }
    }
  } else if (shouldAutoPlay && autoChoiceOn && hasNextChoices) {
    autoAdvanceFallbackTimer = setTimeout(() => {
      sendServerAction("AudioEnded", {});
    }, 1500);
  }

  scheduleCurrentSceneMediaPrefetch(audio);

  if (playBtn) {
    playBtn.addEventListener("click", () => {
      if (!url) {
        showToast("Ta scena nie posiada przypisanego pliku audio.");
        return;
      }
      if (audio.paused) {
        if (audio.getAttribute("data-auto-choice") === "true") {
          autoPlayChainActive = true;
        }
        if (audio.readyState >= 4) {
          setBufferingState(false);
          audio.play().catch(() => {});
        } else {
          setBufferingState(true);
          audio.addEventListener(
            "canplaythrough",
            () => {
              setBufferingState(false);
              audio.play().catch(() => {});
            },
            { once: true }
          );
          audio.load();
        }
      } else {
        autoPlayChainActive = false;
        audio.pause();
      }
    });
  }

  audio.addEventListener("play", () => setPlayButtonVisual(true));
  audio.addEventListener("pause", () => {
    setPlayButtonVisual(false);
    if (!audio.ended && audio.currentTime > 0) {
      sendServerAction("SaveAudioTime", { time: Math.floor(audio.currentTime * 10) / 10 });
    }
  });
  audio.addEventListener("loadedmetadata", () => {
    const tot = qs("#audio-time-total");
    if (tot) tot.textContent = formatSeconds(audio.duration);
    if (slider && audio.duration > 0) {
      const val = Math.round((audio.currentTime / audio.duration) * 1000);
      slider.value = String(val);
      slider.style.setProperty("--seek-pct", (val / 10).toFixed(1) + "%");
    }
  });

  audio.addEventListener("timeupdate", () => {
    const cur = qs("#audio-time-current");
    if (cur) cur.textContent = formatSeconds(audio.currentTime);
    if (slider && audio.duration > 0) {
      const val = Math.round((audio.currentTime / audio.duration) * 1000);
      slider.value = String(val);
      slider.style.setProperty("--seek-pct", (val / 10).toFixed(1) + "%");
    }
    const now = Date.now();
    if (now - lastAudioSaveAt > 2000) {
      lastAudioSaveAt = now;
      sendServerAction("SaveAudioTime", { time: Math.floor(audio.currentTime * 10) / 10 });
    }
  });

  audio.addEventListener("ended", () => {
    setPlayButtonVisual(false);
    if (slider) {
      slider.value = "1000";
      slider.style.setProperty("--seek-pct", "100%");
    }
    sendServerAction("SaveAudioTime", { time: 0 });
    if (audio.getAttribute("data-auto-choice") === "true") {
      sendServerAction("AudioEnded", {});
    }
  });

  if (slider) {
    slider.addEventListener("input", () => {
      const val = parseFloat(slider.value) || 0;
      slider.style.setProperty("--seek-pct", (val / 10).toFixed(1) + "%");
      if (audio.duration > 0) {
        audio.currentTime = (val / 1000) * audio.duration;
      }
    });
    slider.addEventListener("change", () => {
      if (audio.duration > 0) {
        sendServerAction("SaveAudioTime", { time: Math.floor(audio.currentTime * 10) / 10 });
      }
    });
  }
}
