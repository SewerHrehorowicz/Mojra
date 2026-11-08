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
let nextAudioPreload = null;
const prefetchedMediaCache = new Set();

function parseSceneAudioQueue(queueAttr, fallbackUrl, fallbackTrimStart, fallbackTrimEnd) {
  const raw = (queueAttr || "").trim();
  const out = [];
  if (raw) {
    const parts = raw.split(";");
    for (let i = 0; i < parts.length; i++) {
      const seg = parts[i].trim();
      if (!seg) continue;
      const fields = seg.split("|");
      const segUrl = (fields[0] || "").trim();
      if (!segUrl) continue;
      let ts = parseFloat(fields[1] || "0");
      let te = parseFloat(fields[2] || "1");
      let dur = parseFloat(fields[3] || "0");
      if (!Number.isFinite(ts) || ts < 0 || ts >= 1) ts = 0;
      if (!Number.isFinite(te) || te <= 0 || te > 1) te = 1;
      if (te <= ts + 0.002) {
        ts = 0;
        te = 1;
      }
      if (!Number.isFinite(dur) || dur < 0) dur = 0;
      const span = Math.max(0.001, te - ts);
      out.push({
        url: segUrl,
        trimStart: ts,
        trimEnd: te,
        durationSec: dur,
        rawDurationSec: dur > 0 ? dur / span : 0
      });
    }
  }
  const cleanFallback = (fallbackUrl || "").trim();
  if (out.length === 0 && cleanFallback) {
    let ts = parseFloat(fallbackTrimStart || "0");
    let te = parseFloat(fallbackTrimEnd || "1");
    if (!Number.isFinite(ts) || ts < 0 || ts >= 1) ts = 0;
    if (!Number.isFinite(te) || te <= 0 || te > 1) te = 1;
    if (te <= ts + 0.002) {
      ts = 0;
      te = 1;
    }
    out.push({
      url: cleanFallback,
      trimStart: ts,
      trimEnd: te,
      durationSec: 0,
      rawDurationSec: 0
    });
  }
  return out;
}

function getQueueSegmentEffectiveBounds(segments, currentIdx, audio, idx) {
  if (!segments || idx < 0 || idx >= segments.length) {
    return getEffectiveAudioBounds(audio);
  }
  const seg = segments[idx];
  let rawDur = seg.rawDurationSec || 0;
  if (rawDur <= 0 && seg.durationSec > 0) {
    const span = Math.max(0.001, seg.trimEnd - seg.trimStart);
    rawDur = seg.durationSec / span;
    seg.rawDurationSec = rawDur;
  }
  if (rawDur <= 0 && idx === currentIdx && audio && Number.isFinite(audio.duration) && audio.duration > 0) {
    rawDur = audio.duration;
    seg.rawDurationSec = rawDur;
    seg.durationSec = rawDur * (seg.trimEnd - seg.trimStart);
  }
  const startSec = rawDur * seg.trimStart;
  const endSec = rawDur * seg.trimEnd;
  const effectiveDur = rawDur > 0 ? Math.max(0, endSec - startSec) : Math.max(0, seg.durationSec || 0);
  return {
    startRatio: seg.trimStart,
    endRatio: seg.trimEnd,
    startSec: startSec,
    endSec: endSec,
    effectiveDur: effectiveDur,
    rawDur: rawDur
  };
}

function getQueueTotalDuration(segments, currentIdx, audio) {
  if (!segments || segments.length === 0) {
    return getEffectiveAudioBounds(audio).effectiveDur;
  }
  let total = 0;
  for (let i = 0; i < segments.length; i++) {
    total += getQueueSegmentEffectiveBounds(segments, currentIdx, audio, i).effectiveDur;
  }
  return total;
}

function getQueueCurrentElapsed(segments, currentIdx, audio) {
  if (!segments || segments.length === 0) {
    const b = getEffectiveAudioBounds(audio);
    return Math.max(0, Math.min(b.effectiveDur, ((audio && audio.currentTime) || 0) - b.startSec));
  }
  let elapsed = 0;
  for (let i = 0; i < currentIdx && i < segments.length; i++) {
    elapsed += getQueueSegmentEffectiveBounds(segments, currentIdx, audio, i).effectiveDur;
  }
  const curBounds = getQueueSegmentEffectiveBounds(segments, currentIdx, audio, currentIdx);
  const rawCur = (audio && audio.currentTime) || 0;
  const localCur =
    curBounds.effectiveDur > 0
      ? Math.max(0, Math.min(curBounds.effectiveDur, rawCur - curBounds.startSec))
      : Math.max(0, rawCur - curBounds.startSec);
  return elapsed + localCur;
}

function mapQueueGlobalTimeToSegment(segments, currentIdx, audio, globalTimeSec) {
  const clampedGlobal = Math.max(0, Number(globalTimeSec) || 0);
  if (!segments || segments.length === 0) {
    const b = getEffectiveAudioBounds(audio);
    const localOff = Math.max(0, Math.min(b.effectiveDur, clampedGlobal));
    return {
      index: 0,
      localOffset: localOff,
      rawAudioTime: b.startSec + localOff,
      bounds: b
    };
  }
  let accum = 0;
  for (let i = 0; i < segments.length; i++) {
    const b = getQueueSegmentEffectiveBounds(segments, currentIdx, audio, i);
    if (clampedGlobal <= accum + b.effectiveDur || i === segments.length - 1) {
      const localOff =
        b.effectiveDur > 0
          ? Math.max(0, Math.min(b.effectiveDur, clampedGlobal - accum))
          : Math.max(0, clampedGlobal - accum);
      return {
        index: i,
        localOffset: localOff,
        rawAudioTime: b.startSec + localOff,
        bounds: b
      };
    }
    accum += b.effectiveDur;
  }
  const lastIdx = segments.length - 1;
  const lastB = getQueueSegmentEffectiveBounds(segments, currentIdx, audio, lastIdx);
  return {
    index: lastIdx,
    localOffset: lastB.effectiveDur,
    rawAudioTime: lastB.endSec,
    bounds: lastB
  };
}

function preloadNextQueueSegment(segments, nextIdx) {
  if (!segments || nextIdx < 0 || nextIdx >= segments.length) {
    return;
  }
  const nextSeg = segments[nextIdx];
  if (!nextSeg || !nextSeg.url) return;
  if (!nextAudioPreload && typeof Audio !== "undefined") {
    try {
      nextAudioPreload = new Audio();
    } catch (_) {
      nextAudioPreload = null;
    }
  }
  if (!nextAudioPreload) return;
  nextAudioPreload.preload = "auto";
  const prevPreloaded =
    (nextAudioPreload.getAttribute && nextAudioPreload.getAttribute("data-preload-url")) || "";
  if (prevPreloaded === nextSeg.url) {
    return;
  }
  if (nextAudioPreload.setAttribute) {
    nextAudioPreload.setAttribute("data-preload-url", nextSeg.url);
  }
  nextAudioPreload.src = nextSeg.url;
  const applyPreloadTrim = () => {
    if (Number.isFinite(nextAudioPreload.duration) && nextAudioPreload.duration > 0) {
      nextSeg.rawDurationSec = nextAudioPreload.duration;
      nextSeg.durationSec = nextAudioPreload.duration * (nextSeg.trimEnd - nextSeg.trimStart);
      if (nextSeg.trimStart > 0) {
        try {
          nextAudioPreload.currentTime = nextAudioPreload.duration * nextSeg.trimStart;
        } catch (_) {}
      }
    }
  };
  if (typeof nextAudioPreload.addEventListener === "function") {
    nextAudioPreload.addEventListener("loadedmetadata", applyPreloadTrim, { once: true });
  }
  if (typeof nextAudioPreload.load === "function") {
    try {
      nextAudioPreload.load();
    } catch (_) {}
  }
}

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

  const initialUrl = audio.getAttribute("data-audio-url") || "";
  const resumeTime =
    preservedTime !== null && preservedTime !== undefined
      ? preservedTime
      : parseFloat(audio.getAttribute("data-resume-time") || "0") || 0;
  const speed = parseFloat(audio.getAttribute("data-speed") || "1") || 1.0;
  const autoChoiceOn = audio.getAttribute("data-auto-choice") === "true";
  const slider = qs("#audio-seek-slider");
  const playBtn = qs("#btn-audio-play");
  const hasNextChoices = qsa('#active-scene-stage .choice-btn[action="ChooseBranch"]:not(.choice-locked)').length > 0;

  let queueSegments = parseSceneAudioQueue(
    audio.getAttribute("data-scene-audio-queue"),
    initialUrl,
    audio.getAttribute("data-trim-start"),
    audio.getAttribute("data-trim-end")
  );
  let currentSegmentIdx = 0;
  let initialPositionApplied = false;

  const syncAudioQueueInspectionProps = () => {
    audio._queueSegments = queueSegments;
    audio._currentSegmentIndex = currentSegmentIdx;
    audio._nextAudioPreload = nextAudioPreload;
  };
  syncAudioQueueInspectionProps();

  const setBufferingState = (buffering) => {
    if (playBtn) {
      playBtn.classList.toggle("is-buffering", Boolean(buffering));
    }
  };

  const updateProgressDeckUI = (globalElapsedOverride) => {
    const totalDur = getQueueTotalDuration(queueSegments, currentSegmentIdx, audio);
    const tot = qs("#audio-time-total");
    if (tot && totalDur > 0) {
      tot.textContent = formatSeconds(totalDur);
    }
    const globalCur =
      globalElapsedOverride !== undefined
        ? Math.max(0, Math.min(totalDur || globalElapsedOverride, globalElapsedOverride))
        : getQueueCurrentElapsed(queueSegments, currentSegmentIdx, audio);
    const curEl = qs("#audio-time-current");
    if (curEl) {
      curEl.textContent = formatSeconds(globalCur);
    }
    if (slider && totalDur > 0) {
      const val = Math.min(1000, Math.max(0, Math.round((globalCur / totalDur) * 1000)));
      slider.value = String(val);
      slider.style.setProperty("--seek-pct", (val / 10).toFixed(1) + "%");
    }
  };

  const switchQueueSegment = (targetIdx, localOffsetSec, autoPlayAfterSwitch) => {
    if (!queueSegments || targetIdx < 0 || targetIdx >= queueSegments.length) {
      return false;
    }
    currentSegmentIdx = targetIdx;
    const seg = queueSegments[currentSegmentIdx];
    audio.setAttribute("data-audio-url", seg.url);
    audio.setAttribute("data-trim-start", seg.trimStart.toFixed(4));
    audio.setAttribute("data-trim-end", seg.trimEnd.toFixed(4));
    const curSpeed = parseFloat(audio.getAttribute("data-speed") || "1") || 1.0;
    audio.playbackRate = curSpeed;
    const prevSrc = audio.getAttribute("src") || String(audio.src || "");
    const srcChanged = prevSrc !== seg.url && !String(prevSrc).endsWith(seg.url);
    if (srcChanged) {
      audio.src = seg.url;
      if (typeof audio.load === "function") {
        audio.load();
      }
    }
    const applySegmentSeek = () => {
      const b = getQueueSegmentEffectiveBounds(queueSegments, currentSegmentIdx, audio, currentSegmentIdx);
      const targetRaw = b.startSec + (Number(localOffsetSec) || 0);
      try {
        audio.currentTime = targetRaw;
      } catch (_) {}
      updateProgressDeckUI();
    };
    applySegmentSeek();
    if (srcChanged && !(audio.duration > 0)) {
      audio.addEventListener("loadedmetadata", applySegmentSeek, { once: true });
    }
    preloadNextQueueSegment(queueSegments, currentSegmentIdx + 1);
    syncAudioQueueInspectionProps();
    if (autoPlayAfterSwitch) {
      pendingSceneAutoPlay = true;
      setPlayButtonVisual(true);
      const p = audio.play();
      if (p && typeof p.then === "function") {
        p.then(() => {
          pendingSceneAutoPlay = false;
          setBufferingState(false);
          setPlayButtonVisual(true);
        }).catch(() => {
          pendingSceneAutoPlay = false;
        });
      } else {
        pendingSceneAutoPlay = false;
        setPlayButtonVisual(!audio.paused);
      }
    }
    return true;
  };

  audio._reloadQueueFromAttributes = (fallbackDurLabel) => {
    queueSegments = parseSceneAudioQueue(
      audio.getAttribute("data-scene-audio-queue"),
      audio.getAttribute("data-audio-url"),
      audio.getAttribute("data-trim-start"),
      audio.getAttribute("data-trim-end")
    );
    currentSegmentIdx = 0;
    sceneAudioEndedHandled = false;
    const curEl = qs("#audio-time-current");
    const totEl = qs("#audio-time-total");
    if (curEl) curEl.textContent = "0:00";
    if (totEl && fallbackDurLabel) totEl.textContent = fallbackDurLabel;
    if (slider) {
      slider.value = "0";
      slider.style.setProperty("--seek-pct", "0%");
    }
    if (queueSegments.length > 0) {
      switchQueueSegment(0, 0, false);
    } else {
      audio.removeAttribute("src");
      syncAudioQueueInspectionProps();
    }
  };

  const applyInitialPositionAndTotal = () => {
    const totalDur = getQueueTotalDuration(queueSegments, currentSegmentIdx, audio);
    const tot = qs("#audio-time-total");
    if (tot && totalDur > 0) {
      tot.textContent = formatSeconds(totalDur);
    }
    if (!initialPositionApplied && totalDur > 0) {
      initialPositionApplied = true;
      const targetResume =
        preservedTime !== null && preservedTime !== undefined && preservedTime > 0.2
          ? preservedTime
          : resumeTime > 0.2
            ? resumeTime
            : 0;
      if (targetResume > 0.2 && targetResume < totalDur - 0.1) {
        const mapped = mapQueueGlobalTimeToSegment(queueSegments, currentSegmentIdx, audio, targetResume);
        if (mapped.index !== currentSegmentIdx) {
          switchQueueSegment(mapped.index, mapped.localOffset, shouldAutoPlay);
          return;
        }
        try {
          audio.currentTime = mapped.rawAudioTime;
        } catch (_) {}
      } else {
        const b = getQueueSegmentEffectiveBounds(queueSegments, currentSegmentIdx, audio, currentSegmentIdx);
        if (b.startSec > 0 && audio.currentTime < b.startSec) {
          try {
            audio.currentTime = b.startSec;
          } catch (_) {}
        }
      }
    }
    updateProgressDeckUI();
  };

  const url = queueSegments.length > 0 ? queueSegments[0].url : initialUrl;
  if (url) {
    if (queueSegments.length > 0) {
      audio.setAttribute("data-audio-url", queueSegments[0].url);
      audio.setAttribute("data-trim-start", queueSegments[0].trimStart.toFixed(4));
      audio.setAttribute("data-trim-end", queueSegments[0].trimEnd.toFixed(4));
    }
    audio.preload = "auto";
    audio.src = url;
    audio.playbackRate = speed;
    preloadNextQueueSegment(queueSegments, 1);
    syncAudioQueueInspectionProps();
    if (audio.readyState < 4) {
      setBufferingState(true);
      const clearBuffering = () => setBufferingState(false);
      audio.addEventListener("canplaythrough", clearBuffering, { once: true });
      audio.addEventListener("playing", clearBuffering, { once: true });
      audio.addEventListener("error", clearBuffering, { once: true });
    }
    audio.addEventListener("loadedmetadata", applyInitialPositionAndTotal, { once: true });
    audio.load();
    if (audio.duration > 0 || getQueueTotalDuration(queueSegments, currentSegmentIdx, audio) > 0) {
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
        const bounds = getQueueSegmentEffectiveBounds(queueSegments, currentSegmentIdx, audio, currentSegmentIdx);
        if (bounds.startSec > 0 && audio.currentTime < bounds.startSec) {
          try {
            audio.currentTime = bounds.startSec;
          } catch (_) {}
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

  const handleSegmentOrQueueReachedEnd = () => {
    if (currentSegmentIdx + 1 < queueSegments.length) {
      sceneAudioEndedHandled = false;
      switchQueueSegment(currentSegmentIdx + 1, 0, true);
      return;
    }
    if (sceneAudioEndedHandled) return;
    sceneAudioEndedHandled = true;
    pendingSceneAutoPlay = false;
    if (!audio.paused) {
      audio.pause();
    }
    setPlayButtonVisual(false);
    const totalDur = getQueueTotalDuration(queueSegments, currentSegmentIdx, audio);
    const cur = qs("#audio-time-current");
    if (cur && totalDur > 0) {
      cur.textContent = formatSeconds(totalDur);
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
      const currentUrl =
        (queueSegments.length > 0 && queueSegments[currentSegmentIdx] && queueSegments[currentSegmentIdx].url) ||
        audio.getAttribute("data-audio-url") ||
        "";
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
        const wasAtQueueEnd = sceneAudioEndedHandled;
        sceneAudioEndedHandled = false;
        if (audio.getAttribute("data-auto-choice") === "true") {
          autoPlayChainActive = true;
        }
        if (wasAtQueueEnd && queueSegments.length > 1) {
          switchQueueSegment(0, 0, true);
          return;
        }
        const curSpeed = parseFloat(audio.getAttribute("data-speed") || "1") || 1.0;
        audio.playbackRate = curSpeed;
        if (audio.getAttribute("src") !== currentUrl && !String(audio.src || "").endsWith(currentUrl)) {
          audio.src = currentUrl;
          audio.load();
        }
        const bounds = getQueueSegmentEffectiveBounds(queueSegments, currentSegmentIdx, audio, currentSegmentIdx);
        if (
          wasAtQueueEnd ||
          audio.ended ||
          (bounds.endSec > 0 && audio.currentTime >= bounds.endSec - 0.05) ||
          audio.currentTime < bounds.startSec
        ) {
          try {
            audio.currentTime = bounds.startSec;
          } catch (_) {}
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
      const globalElapsed = getQueueCurrentElapsed(queueSegments, currentSegmentIdx, audio);
      sendServerAction("SaveAudioTime", { time: Math.floor(globalElapsed * 10) / 10 });
    }
  });
  audio.addEventListener("loadedmetadata", applyInitialPositionAndTotal);

  audio.addEventListener("timeupdate", () => {
    const bounds = getQueueSegmentEffectiveBounds(queueSegments, currentSegmentIdx, audio, currentSegmentIdx);
    if (bounds.endRatio < 0.999 && bounds.endSec > 0 && audio.currentTime >= bounds.endSec) {
      handleSegmentOrQueueReachedEnd();
      return;
    }
    const globalElapsed = getQueueCurrentElapsed(queueSegments, currentSegmentIdx, audio);
    updateProgressDeckUI(globalElapsed);
    const now = Date.now();
    if (now - lastAudioSaveAt > 2000 && !sceneAudioEndedHandled) {
      lastAudioSaveAt = now;
      sendServerAction("SaveAudioTime", { time: Math.floor(globalElapsed * 10) / 10 });
    }
  });

  audio.addEventListener("ended", handleSegmentOrQueueReachedEnd);

  if (slider) {
    slider.addEventListener("input", () => {
      if (autoAdvanceFallbackTimer) {
        clearTimeout(autoAdvanceFallbackTimer);
        autoAdvanceFallbackTimer = null;
      }
      sceneAudioEndedHandled = false;
      const val = parseFloat(slider.value) || 0;
      slider.style.setProperty("--seek-pct", (val / 10).toFixed(1) + "%");
      const totalDur = getQueueTotalDuration(queueSegments, currentSegmentIdx, audio);
      if (totalDur > 0) {
        const targetGlobalSec = (val / 1000) * totalDur;
        const mapped = mapQueueGlobalTimeToSegment(queueSegments, currentSegmentIdx, audio, targetGlobalSec);
        if (mapped.index !== currentSegmentIdx) {
          const wasPlaying = !audio.paused || pendingSceneAutoPlay;
          switchQueueSegment(mapped.index, mapped.localOffset, wasPlaying);
        } else {
          try {
            audio.currentTime = mapped.rawAudioTime;
          } catch (_) {}
        }
        const curEl = qs("#audio-time-current");
        if (curEl) curEl.textContent = formatSeconds(targetGlobalSec);
      }
    });
    slider.addEventListener("change", () => {
      const totalDur = getQueueTotalDuration(queueSegments, currentSegmentIdx, audio);
      if (totalDur > 0) {
        const globalElapsed = getQueueCurrentElapsed(queueSegments, currentSegmentIdx, audio);
        sendServerAction("SaveAudioTime", { time: Math.floor(globalElapsed * 10) / 10 });
      }
    });
  }
}
