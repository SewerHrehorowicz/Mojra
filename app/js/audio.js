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

function initAudioController(shouldAutoPlay, preservedTime) {
  if (autoAdvanceFallbackTimer) {
    clearTimeout(autoAdvanceFallbackTimer);
    autoAdvanceFallbackTimer = null;
  }

  const audio = qs("#story-audio");
  if (!audio) return;

  const url = audio.getAttribute("data-audio-url") || "";
  const resumeTime =
    preservedTime !== null && preservedTime !== undefined
      ? preservedTime
      : parseFloat(audio.getAttribute("data-resume-time") || "0") || 0;
  const speed = parseFloat(audio.getAttribute("data-speed") || "1") || 1.0;
  const autoChoiceOn = audio.getAttribute("data-auto-choice") === "true";
  const slider = qs("#audio-seek-slider");
  const hasNextChoices = qsa('#active-scene-stage .choice-btn[action="ChooseBranch"]').length > 0;

  if (url) {
    audio.src = url;
    audio.playbackRate = speed;
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
    if (shouldAutoPlay) {
      const startPlay = () => {
        audio.playbackRate = speed;
        audio.play().catch(() => {});
      };
      if (audio.readyState >= 1) {
        startPlay();
      } else {
        audio.addEventListener("loadedmetadata", startPlay, { once: true });
        startPlay();
      }
    }
  } else if (shouldAutoPlay && autoChoiceOn && hasNextChoices) {
    autoAdvanceFallbackTimer = setTimeout(() => {
      sendServerAction("AudioEnded", {});
    }, 1500);
  }

  const playBtn = qs("#btn-audio-play");

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
        audio.play().catch(() => {});
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
