"use strict";

let toastTimer = null;
let uiTooltipTimer = null;
let activeHintEl = null;
let lastAudioSaveAt = 0;
let autoPlayChainActive = false;
let isWindowFullscreen = false;

function qs(sel, root) {
  return (root || document).querySelector(sel);
}

function qsa(sel, root) {
  return Array.from((root || document).querySelectorAll(sel));
}

function getOrCreateUITooltip() {
  return qs("#ui-tooltip");
}

function hideUITooltip() {
  if (uiTooltipTimer) {
    clearTimeout(uiTooltipTimer);
    uiTooltipTimer = null;
  }
  activeHintEl = null;
  const tip = qs("#ui-tooltip");
  if (tip) {
    tip.classList.add("hidden");
  }
}

function positionAndShowUITooltip(anchorEl) {
  if (!anchorEl || !anchorEl.isConnected) {
    hideUITooltip();
    return;
  }
  const parentSelect = anchorEl.closest(".custom-select");
  if (parentSelect && parentSelect.classList.contains("open")) {
    hideUITooltip();
    return;
  }
  const text = (anchorEl.getAttribute("data-hint") || "").trim();
  if (!text) {
    hideUITooltip();
    return;
  }
  const tip = getOrCreateUITooltip();
  if (!tip) return;
  tip.textContent = text;
  tip.classList.remove("hidden");

  const rect = anchorEl.getBoundingClientRect();
  const tipW = tip.offsetWidth || 120;
  const tipH = tip.offsetHeight || 26;
  const vw = window.innerWidth || 800;
  const vh = window.innerHeight || 600;

  let left = rect.left + rect.width / 2 - tipW / 2;
  left = Math.max(8, Math.min(vw - tipW - 8, left));

  let top = rect.bottom + 6;
  if (top + tipH > vh - 8 && rect.top - tipH - 6 >= 8) {
    top = rect.top - tipH - 6;
  } else {
    top = Math.max(8, Math.min(vh - tipH - 8, top));
  }

  tip.style.left = Math.round(left) + "px";
  tip.style.top = Math.round(top) + "px";
}

function setElementHint(el, text) {
  if (!el) return;
  el.removeAttribute("title");
  if (text) {
    el.setAttribute("data-hint", text);
  } else {
    el.removeAttribute("data-hint");
  }
  if (activeHintEl === el) {
    const tip = qs("#ui-tooltip");
    if (tip && !tip.classList.contains("hidden")) {
      positionAndShowUITooltip(el);
    }
  }
}

function currentLangIndex() {
  const code = (typeof document !== "undefined" && document.body && document.body.getAttribute("data-lang")) || "pl";
  return typeof I18N_LANGS !== "undefined" && I18N_LANGS[code] !== undefined ? I18N_LANGS[code] : 0;
}

function tr(key) {
  const row = typeof I18N_TABLE !== "undefined" ? I18N_TABLE[key] : null;
  if (!row) return "";
  const idx = currentLangIndex();
  return row[idx] || row[1] || row[0] || "";
}

function trStd(isPL, key) {
  const row = typeof I18N_TABLE !== "undefined" ? I18N_TABLE[key] : null;
  if (!row) return "";
  return row[isPL ? 0 : 1] || row[0] || "";
}

function trf(key) {
  let s = tr(key);
  for (let i = 1; i < arguments.length; i++) {
    s = s.replace("%s", String(arguments[i]));
  }
  return s;
}

function syncFullscreenButtonState() {
  const enterIcon = qs("#icon-fullscreen-enter");
  const exitIcon = qs("#icon-fullscreen-exit");
  const btn = qs("#btn-toggle-fullscreen");
  if (enterIcon) enterIcon.classList.toggle("hidden", isWindowFullscreen);
  if (exitIcon) exitIcon.classList.toggle("hidden", !isWindowFullscreen);
  if (btn) {
    setElementHint(
      btn,
      isWindowFullscreen ? tr("fullscreen_exit_hint") : tr("fullscreen_enter_hint")
    );
  }
}

function setFullscreenUIState(active) {
  isWindowFullscreen = Boolean(active);
  syncFullscreenButtonState();
}

document.addEventListener("fullscreenchange", () => {
  setFullscreenUIState(Boolean(document.fullscreenElement || document.webkitFullscreenElement));
});

document.addEventListener("webkitfullscreenchange", () => {
  setFullscreenUIState(Boolean(document.fullscreenElement || document.webkitFullscreenElement));
});

function showToast(message) {
  const banner = qs("#toast-banner");
  if (!banner || !message) return;
  banner.textContent = message;
  banner.classList.remove("hidden");
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    banner.classList.add("hidden");
  }, 2800);
}

function syncTopbarToggleIcons() {
  const isLarge = document.body.getAttribute("data-ui-scale") === "large";
  qsa('button[action="SetUIScale"]').forEach((btn) => {
    const sc = btn.getAttribute("data-scale");
    btn.classList.toggle("active", sc === "large" ? isLarge : !isLarge);
  });

  const isZen = document.body.getAttribute("data-zen") === "true";
  const zenBtn = qs("#btn-toggle-zen");
  if (zenBtn) {
    zenBtn.classList.toggle("active", isZen);
    setElementHint(
      zenBtn,
      isZen ? tr("scale_exit_zen_hint") : tr("scale_zen_hint")
    );
  }

  const isImagelessLocked = document.body.getAttribute("data-imageless-locked") === "true";
  const isImageless = isImagelessLocked || document.body.getAttribute("data-imageless") === "true";
  const imgIconOn = qs("#icon-imageless-off");
  const imgIconOff = qs("#icon-imageless-on");
  const imgBtn = qs("#btn-toggle-imageless");
  if (imgIconOn) imgIconOn.classList.toggle("hidden", isImageless);
  if (imgIconOff) imgIconOff.classList.toggle("hidden", !isImageless);
  if (imgBtn) {
    imgBtn.disabled = isImagelessLocked;
    imgBtn.classList.toggle("is-disabled", isImagelessLocked);
    imgBtn.classList.toggle("active", imgBtn.classList.contains("form-toggle-btn") ? !isImageless : isImageless);
    setElementHint(
      imgBtn,
      isImagelessLocked
        ? tr("imageless_locked_hint")
        : isImageless
          ? tr("imageless_enable_hint")
          : tr("imageless_disable_hint")
    );
  }

  const isLightTheme = document.body.getAttribute("data-theme") === "light";
  const sunIcon = qs("#icon-theme-sun");
  const moonIcon = qs("#icon-theme-moon");
  if (sunIcon) sunIcon.classList.toggle("hidden", isLightTheme);
  if (moonIcon) moonIcon.classList.toggle("hidden", !isLightTheme);
}

function toggleUIScaleClient(forcedScale) {
  const currentScale = document.body.getAttribute("data-ui-scale") === "large" ? "large" : "small";
  const nextScale =
    forcedScale === "large" || forcedScale === "small"
      ? forcedScale
      : currentScale === "large"
        ? "small"
        : "large";
  document.body.setAttribute("data-ui-scale", nextScale);
  syncTopbarToggleIcons();
  if (typeof restorePlayerScrollPosition === "function") {
    restorePlayerScrollPosition();
  }
  if (typeof initSceneTextModalFeed === "function") {
    initSceneTextModalFeed(false);
  }
  sendServerAction("ToggleUIScale", { scale: nextScale });
}

function decodeHeaderUTF8(raw) {
  if (!raw) return "";
  let allLatin1 = true;
  let hasHighByte = false;
  for (let i = 0; i < raw.length; i++) {
    const code = raw.charCodeAt(i);
    if (code > 0xff) {
      allLatin1 = false;
      break;
    }
    if (code >= 0x80) {
      hasHighByte = true;
    }
  }
  if (allLatin1 && hasHighByte && typeof TextDecoder !== "undefined") {
    try {
      const bytes = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) {
        bytes[i] = raw.charCodeAt(i) & 0xff;
      }
      return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
    } catch (_) {}
  }
  return raw;
}

function syncEditorHistoryHeaders(resp) {
  if (!resp || !resp.headers) return;
  const histPreviewHeader = resp.headers.get("X-History-Preview");
  if (histPreviewHeader === "true" || histPreviewHeader === "false") {
    document.body.setAttribute("data-history-preview", histPreviewHeader);
    const edView = qs("#view-editor");
    if (edView) {
      edView.setAttribute("data-history-preview", histPreviewHeader);
      edView.classList.toggle("is-history-preview", histPreviewHeader === "true");
    }
  }
}

function applyServerResponseHeaders(resp, actionName) {
  syncEditorHistoryHeaders(resp);
  const lang = resp.headers.get("X-Lang");
  if (lang) {
    document.body.setAttribute("data-lang", lang);
    if (document.documentElement) {
      document.documentElement.setAttribute("lang", lang);
    }
  }
  const theme = resp.headers.get("X-Theme");
  if (theme) {
    document.body.setAttribute("data-theme", theme);
    if (typeof invalidateStudioThemeCache === "function") {
      invalidateStudioThemeCache();
    }
    if (typeof renderStudioCanvas === "function") {
      renderStudioCanvas();
    }
    if (typeof syncAllVoiceStrips === "function") {
      syncAllVoiceStrips();
    }
  }

  const imagelessLocked = resp.headers.get("X-Imageless-Locked");
  if (imagelessLocked) {
    document.body.setAttribute("data-imageless-locked", imagelessLocked);
  }

  const imageless = resp.headers.get("X-Imageless");
  if (imageless) {
    const effectiveImageless = imageless === "true" || document.body.getAttribute("data-imageless-locked") === "true";
    document.body.setAttribute("data-imageless", effectiveImageless ? "true" : "false");
    const playerLayout = qs("#player-layout");
    if (playerLayout) {
      playerLayout.classList.toggle("is-imageless", effectiveImageless);
    }
    const sceneTextLayout = qs(".scene-text-book-layout");
    if (sceneTextLayout) {
      const hasSceneImg = Boolean(qs(".player-bg-img", sceneTextLayout));
      sceneTextLayout.classList.toggle("is-imageless", effectiveImageless || !hasSceneImg);
    }
  }

  const uiScale = resp.headers.get("X-UI-Scale");
  if (uiScale === "large" || uiScale === "small") {
    document.body.setAttribute("data-ui-scale", uiScale);
  }

  const zenMode = resp.headers.get("X-Zen-Mode");
  if (zenMode === "true" || zenMode === "false") {
    document.body.setAttribute("data-zen", zenMode);
  }

  const audioSpeed = resp.headers.get("X-Audio-Speed");
  if (audioSpeed) {
    const audioEl = qs("#story-audio");
    const speedNum = parseFloat(audioSpeed) || 1.0;
    if (audioEl) {
      audioEl.setAttribute("data-speed", audioSpeed);
      audioEl.playbackRate = speedNum;
    }
    const speedBtn = qs("#btn-audio-speed");
    if (speedBtn) {
      speedBtn.textContent = audioSpeed + "x";
    }
  }

  const autoDelay = resp.headers.get("X-Auto-Delay");
  if (autoDelay) {
    const audioEl = qs("#story-audio");
    if (audioEl) {
      audioEl.setAttribute("data-auto-delay", autoDelay);
    }
  }

  const scrollSens = resp.headers.get("X-Scroll-Sensitivity");
  if (scrollSens) {
    document.body.setAttribute("data-scroll-sensitivity", scrollSens);
  }

  const autoChoice = resp.headers.get("X-Auto-Choice");
  if (autoChoice) {
    const isAutoOn = autoChoice === "true";
    const audioEl = qs("#story-audio");
    if (audioEl) {
      audioEl.setAttribute("data-auto-choice", autoChoice);
    }
    const autoBtn = qs("#btn-auto-choice-toggle");
    if (autoBtn) {
      autoBtn.classList.toggle("active", isAutoOn);
      setElementHint(autoBtn, isAutoOn ? tr("auto_choice_on") : tr("auto_choice_off"));
    }
    if (actionName === "ToggleAutoChoice") {
      if (isAutoOn) {
        autoPlayChainActive = true;
        if (audioEl && audioEl.getAttribute("data-audio-url")) {
          if (audioEl.ended || (typeof sceneAudioEndedHandled !== "undefined" && sceneAudioEndedHandled)) {
            if (typeof scheduleAutoChoiceAdvance === "function") {
              scheduleAutoChoiceAdvance(audioEl);
            }
          } else if (audioEl.paused) {
            setPlayButtonVisual(true);
            const p = audioEl.play();
            if (p && typeof p.catch === "function") {
              p.catch(() => setPlayButtonVisual(false));
            }
          }
        } else if (audioEl && typeof scheduleAutoChoiceAdvance === "function") {
          scheduleAutoChoiceAdvance(audioEl);
        }
      } else {
        autoPlayChainActive = false;
        if (typeof autoAdvanceFallbackTimer !== "undefined" && autoAdvanceFallbackTimer) {
          clearTimeout(autoAdvanceFallbackTimer);
          autoAdvanceFallbackTimer = null;
        }
      }
    }
  }

  /* @dev-only-start */
  const deviceMode = resp.headers.get("X-Device-Mode");
  if (deviceMode) {
    const stage = qs("#device-stage");
    if (stage) {
      stage.setAttribute("data-preview", deviceMode);
    }
    qsa(".preview-mode-btn").forEach((btn) => {
      btn.classList.toggle("active", btn.getAttribute("data-mode") === deviceMode);
    });
  }
  /* @dev-only-end */

  const nativeCmd = resp.headers.get("X-Native-Cmd");
  if (nativeCmd === "toggle-fullscreen") {
    if (
      (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.nativeHost) ||
      (window.chrome && window.chrome.webview) ||
      window.__nativeHost
    ) {
      isWindowFullscreen = !isWindowFullscreen;
    } else if (document.documentElement.requestFullscreen) {
      if (!document.fullscreenElement) {
        isWindowFullscreen = true;
        document.documentElement.requestFullscreen().catch(() => {});
      } else {
        isWindowFullscreen = false;
        document.exitFullscreen().catch(() => {});
      }
    } else {
      isWindowFullscreen = !isWindowFullscreen;
    }
  } else if (nativeCmd === "quit" || actionName === "QuitApp" || actionName === "PromptQuitApp") {
    if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.nativeHost) {
      window.webkit.messageHandlers.nativeHost.postMessage("quit");
    } else if (window.chrome && window.chrome.webview && typeof window.chrome.webview.postMessage === "function") {
      window.chrome.webview.postMessage("quit");
    }
    window.close();
    return true;
  }

  syncFullscreenButtonState();
  syncTopbarToggleIcons();

  const toast = decodeHeaderUTF8(resp.headers.get("X-Toast"));
  if (toast) {
    showToast(toast);
  }
  const voicePeak = resp.headers.get("X-Voice-Peak");
  if (voicePeak && typeof handleNativeVoicePeakHeader === "function") {
    handleNativeVoicePeakHeader(voicePeak);
  }
  return false;
}

function escapeWFPHeaderVal(str) {
  return String(str)
    .replace(/\\/g, "\\\\")
    .replace(/\n/g, "\\n")
    .replace(/\r/g, "\\r");
}

function encodeWFP(params) {
  if (!params) return "";
  const entries = [];
  if (typeof FormData !== "undefined" && params instanceof FormData) {
    params.forEach((val, key) => {
      if (val !== undefined && val !== null) {
        entries.push([String(key), String(val)]);
      }
    });
  } else {
    Object.keys(params).forEach((k) => {
      const val = params[k];
      if (val !== undefined && val !== null) {
        entries.push([String(k), String(val)]);
      }
    });
  }
  let bodyKey = "";
  let bodyVal = "";
  for (let i = 0; i < entries.length; i++) {
    const k = entries[i][0];
    const v = entries[i][1];
    if (v.includes("\n") || v.includes("\r")) {
      if (!bodyKey || k === "text") {
        bodyKey = k;
        bodyVal = v;
      }
    }
  }
  const lines = [];
  for (let i = 0; i < entries.length; i++) {
    const k = entries[i][0];
    const v = entries[i][1];
    if (k === bodyKey) {
      lines.push("_body=" + k);
    } else {
      lines.push(k + "=" + escapeWFPHeaderVal(v));
    }
  }
  return lines.join("\n") + (bodyKey ? "\n\n" + bodyVal : "");
}

function decodeWFP(rawText) {
  const raw = String(rawText || "");
  const sep = raw.indexOf("\n\n");
  const head = sep < 0 ? raw : raw.slice(0, sep);
  const body = sep < 0 ? "" : raw.slice(sep + 2);
  const out = {};
  let bodyKey = "";
  head.split("\n").forEach((line) => {
    const clean = line.endsWith("\r") ? line.slice(0, -1) : line;
    const idx = clean.indexOf("=");
    if (idx <= 0) return;
    const k = clean.slice(0, idx).trim();
    const v = clean.slice(idx + 1);
    if (k === "_body") {
      bodyKey = v.trim();
      return;
    }
    out[k] = v.replace(/\\([nr\\])/g, (_, c) => (c === "n" ? "\n" : c === "r" ? "\r" : "\\"));
  });
  if (bodyKey) {
    out[bodyKey] = body;
  }
  return out;
}

function parseWasmActionResponse(rawWfp) {
  const parsed = decodeWFP(rawWfp);
  const saveHex = parsed["X-Save-Hex"] || "";
  if (saveHex && window.__wasmBookId && typeof localStorage !== "undefined") {
    try {
      localStorage.setItem("mojra_save_" + window.__wasmBookId, saveHex);
    } catch (_) {}
  }
  const status = parseInt(parsed.status || "200", 10) || 200;
  const htmlBody = parsed.html || "";
  return {
    status: status,
    headers: {
      get: function (name) {
        if (!name) return null;
        if (parsed[name] !== undefined) return parsed[name];
        const lower = name.toLowerCase();
        const keys = Object.keys(parsed);
        for (let i = 0; i < keys.length; i++) {
          if (keys[i].toLowerCase() === lower) {
            return parsed[keys[i]];
          }
        }
        return null;
      }
    },
    text: async function () {
      return htmlBody;
    }
  };
}

async function bootWasmPlayer(playBtn) {
  if (!playBtn || playBtn.classList.contains("is-loading")) return;
  playBtn.classList.add("is-loading");

  if (typeof Audio !== "undefined") {
    try {
      const unlockAudio = new Audio();
      unlockAudio.muted = true;
      const p = unlockAudio.play();
      if (p && typeof p.catch === "function") p.catch(() => {});
    } catch (_) {}
  }

  try {
    const mountEl =
      playBtn.closest("#wasm-mount") ||
      playBtn.closest("#app") ||
      qs("#wasm-mount") ||
      qs("#app");
    const wasmSrc =
      (playBtn.getAttribute("data-wasm-src") ||
        (mountEl && mountEl.getAttribute("data-wasm-src")) ||
        "mojra.wasm").trim();

    if (typeof Go !== "function" || typeof WebAssembly === "undefined") {
      playBtn.classList.remove("is-loading");
      return;
    }

    let wasmBuf = null;
    let wasmBytes = null;
    if (typeof fetch !== "undefined") {
      try {
        const resp = await fetch(wasmSrc);
        if (resp && resp.ok) {
          wasmBuf = await resp.arrayBuffer();
          wasmBytes = new Uint8Array(wasmBuf);
        }
      } catch (_) {}
    }
    if (!wasmBytes || wasmBytes.length === 0) {
      playBtn.classList.remove("is-loading");
      return;
    }

    if (typeof window.__wasmBoot !== "function") {
      const go = new Go();
      const instantiated = await WebAssembly.instantiate(wasmBuf, go.importObject);
      go.run(instantiated.instance);
    }

    if (typeof window.__wasmBoot !== "function") {
      playBtn.classList.remove("is-loading");
      return;
    }

    const bookHint =
      (playBtn.getAttribute("data-book-id") ||
        (mountEl && mountEl.getAttribute("data-book-id")) ||
        "").trim();
    let savedHex = "";
    if (bookHint && typeof localStorage !== "undefined") {
      try {
        savedHex = localStorage.getItem("mojra_save_" + bookHint) || "";
      } catch (_) {}
    }

    const rawBootWfp = window.__wasmBoot(wasmBytes, savedHex);
    const bootParsed = decodeWFP(rawBootWfp);
    const bookId = bootParsed["X-Book-ID"] || bookHint || "book";
    window.__wasmBookId = bookId;

    if (!savedHex && typeof localStorage !== "undefined") {
      try {
        const existingSave = localStorage.getItem("mojra_save_" + bookId) || "";
        if (existingSave) {
          const reloaded = decodeWFP(window.__wasmBoot(wasmBytes, existingSave));
          if (reloaded.html) {
            Object.assign(bootParsed, reloaded);
          }
        }
      } catch (_) {}
    }

    if (bootParsed["X-Save-Hex"] && typeof localStorage !== "undefined") {
      try {
        localStorage.setItem("mojra_save_" + bookId, bootParsed["X-Save-Hex"]);
      } catch (_) {}
    }

    const imageless = bootParsed["X-Imageless"] === "true";
    document.body.setAttribute("data-imageless", imageless ? "true" : "false");
    document.body.setAttribute("data-imageless-locked", imageless ? "true" : "false");

    if (mountEl && bootParsed.html) {
      mountEl.classList.add("wasm-player-root");
      mountEl.innerHTML = bootParsed.html;
      if (typeof initPlayerScrollAndAudio === "function") {
        initPlayerScrollAndAudio(null, true, null, null);
      }
    }
  } catch (_) {
    playBtn.classList.remove("is-loading");
  }
}

const ACTIONS_FLUSH_INSPECTOR = new Set([
  "ToggleEditorHistoryPanel",
  "OpenEditorHistoryPanel",
  "PreviewHistoryRevision",
  "PlayHistoryRevision",
  "SaveHistoryVersion",
  "PromptCreateHistoryBranch",
  "CreateHistoryBranch",
  "SwitchHistoryBranch",
  "CopyHistoryRevisionToBranch",
  "CloseSceneTextModal",
  "NavigateSceneTextModal",
  "AddBookVariable",
  "SetSceneTextPreviewMode",
  "ToggleSceneTestVarsPopover",
  "UpdateSceneTextTestVar",
  "ToggleChoiceRule",
  "ToggleSyntaxHelp",
  "ToggleEditorVariablesPanel",
  "ToggleBookSettings",
  "ToggleEditorBreakdownPanel",
  "OpenEditorHelpModal",
  "CloseBookSettings",
  "CloseEditorVariablesPanel",
  "CloseEditorBreakdownPanel",
  "CloseEditorHistoryPanel",
  "CloseEditorHelpModal",
  "ToggleBookVariableBool",
  "UpdateBookVariable",
  "RemoveBookVariable",
  "AddBookEnumOption",
  "RemoveBookEnumOption",
  "ToggleSceneVoiceRecorder",
  "SelectSceneVoiceSection",
  "StartSceneVoiceCapture",
  "StopSceneVoiceCapture",
  "SaveSceneVoiceRecording",
  "TrimSceneVoice",
  "DeleteSceneVoice",
  "OpenSceneTextModal",
  "CreateSceneAt",
  "InsertScene",
  "InsertSceneBefore",
  "CreateChapterGroup",
  "SelectStudioScene",
  "SelectStudioChapter",
  "UndoEditor",
  "SwitchView",
  "OpenBook"
]);

const ACTIONS_CLEAR_SELECTION = new Set([
  "PreviewHistoryRevision",
  "ExitHistoryPreview",
  "PlayHistoryRevision",
  "RestoreHistoryRevision",
  "CopyHistoryRevisionToBranch",
  "SwitchHistoryBranch",
  "UndoEditor",
  "RedoEditor"
]);

const ACTIONS_KEEP_MODAL_SCROLL = new Set([
  "SaveSceneTextModal",
  "ToggleSceneTextSettings",
  "SetStartScene",
  "BrowseAddBookMedia",
  "AddBookMediaFromDisk",
  "AddChoice",
  "PromptRemoveChoice",
  "RemoveChoice",
  "ConfirmModalClose",
  "ConfirmModalAccept",
  "BranchChoiceScene",
  "ConnectChoice",
  "AddBookVariable",
  "SetSceneTextPreviewMode",
  "ToggleSceneTestVarsPopover",
  "UpdateSceneTextTestVar",
  "ToggleChoiceRule",
  "ToggleSyntaxHelp",
  "ToggleSceneVoiceRecorder",
  "SelectSceneVoiceSection",
  "StopSceneVoiceCapture",
  "SaveSceneVoiceRecording",
  "TrimSceneVoice",
  "DeleteSceneVoice",
  "ToggleLanguage",
  "SetLanguage"
]);

function restoreSynDropdownFocus(rootEl) {
  if (typeof lastSynRuleInputName !== "string" || !lastSynRuleInputName) return;
  const restoredInput = qs('input[name="' + lastSynRuleInputName + '"]', rootEl);
  if (restoredInput && typeof updateSynAutocompleteOnInput === "function") {
    restoredInput.focus();
    updateSynAutocompleteOnInput(restoredInput, true);
  }
}

function setSceneTextModalWidthMode(sceneModal, toFull) {
  const expandFullBtn = qs("#btn-expand-scene-text-full");
  sceneModal.classList.remove("is-entering");
  sceneModal.classList.toggle("mode-full", toFull);
  sceneModal.classList.toggle("mode-half", !toFull);
  if (expandFullBtn) expandFullBtn.classList.toggle("hidden", toFull);
  if (typeof syncEditorViewportVisibility === "function") syncEditorViewportVisibility();
  const modalTa = qs("#modal-scene-textarea", sceneModal);
  if (modalTa && typeof autoResizeSceneTextarea === "function") {
    autoResizeSceneTextarea(modalTa);
    setTimeout(() => autoResizeSceneTextarea(modalTa), 240);
  }
  postServerActionSilent("SetSceneTextWidthMode", { mode: toFull ? "full" : "half" });
}

async function sendServerAction(actionName, params) {
  hideUITooltip();
  params = params || {};
  if (ACTIONS_FLUSH_INSPECTOR.has(actionName)) {
    if (typeof consumeDirtyInspectorFormIntoParams === "function") {
      consumeDirtyInspectorFormIntoParams(params);
    }
  } else if (actionName === "UndoEditor" || actionName === "RedoEditor" || actionName === "RestoreHistoryRevision" || actionName === "ExitHistoryPreview") {
    if (typeof cancelPendingInspectorTimers === "function") {
      cancelPendingInspectorTimers();
    }
  }
  if (actionName === "ToggleBookSettings" && params.open === undefined) {
    const bsBtn = qs("#btn-book-settings");
    if (bsBtn) params.open = bsBtn.classList.contains("active") ? "false" : "true";
  } else if (
    (actionName === "ToggleEditorHistoryPanel" || actionName === "ToggleEditorBreakdownPanel") &&
    params.book_settings_open === undefined
  ) {
    const bsBtn = qs("#btn-book-settings");
    if (bsBtn) params.book_settings_open = bsBtn.classList.contains("active") ? "true" : "false";
  }
  if ((actionName === "SelectStudioScene" || actionName === "OpenSceneTextModal") && params.scene) {
    if (typeof editorSelectionCleared !== "undefined") editorSelectionCleared = false;
    if (typeof selectedNodeIds !== "undefined") {
      selectedNodeIds.clear();
      selectedNodeIds.add(String(params.scene));
    }
    if (typeof syncNodeSelectionDOM === "function") syncNodeSelectionDOM();
    if (typeof renderStudioCanvas === "function") renderStudioCanvas();
  } else if (ACTIONS_CLEAR_SELECTION.has(actionName)) {
    if (typeof editorSelectionCleared !== "undefined") editorSelectionCleared = false;
    if (typeof selectedNodeIds !== "undefined") selectedNodeIds.clear();
  }

  if (actionName !== "SaveAudioTime" && typeof cancelMediaPrefetchQueue === "function") {
    cancelMediaPrefetchQueue();
  }

  const prevAudio = qs("#story-audio");
  const prevWasPlaying = prevAudio ? !prevAudio.paused && !prevAudio.ended : false;
  const prevAudioTime = prevAudio ? prevAudio.currentTime || 0 : 0;
  const prevAudioUrl = prevAudio ? prevAudio.getAttribute("data-audio-url") || "" : "";

  const wfpPayload = { action: actionName };
  if (prevAudio && prevAudioTime > 0 && actionName !== "SaveAudioTime") {
    wfpPayload.audio_time = String(Math.floor(prevAudioTime * 10) / 10);
  }
  if (params) {
    Object.keys(params).forEach((k) => {
      if (params[k] !== undefined && params[k] !== null) {
        wfpPayload[k] = String(params[k]);
      }
    });
  }

  let resp;
  if (typeof window.__wasmAction === "function") {
    resp = parseWasmActionResponse(window.__wasmAction(encodeWFP(wfpPayload)));
  } else {
    resp = await fetch("/api/action", {
      method: "POST",
      headers: {
        "Content-Type": "text/x-wfp;charset=UTF-8",
        "X-Requested-With": "Mojra"
      },
      body: encodeWFP(wfpPayload)
    });
  }

  if (applyServerResponseHeaders(resp, actionName)) {
    return;
  }

  const targetSelector = resp.headers.get("X-Target") || "#app";
  const keepScroll = resp.headers.get("X-Keep-Scroll") === "true";

  if (resp.status === 204 || targetSelector === "none") {
    if (keepScroll && typeof updateFocusedSceneByViewport === "function") {
      requestAnimationFrame(() => {
        if (typeof restorePlayerScrollPosition === "function") {
          restorePlayerScrollPosition();
        } else {
          updateFocusedSceneByViewport();
        }
        if (typeof initSceneTextModalFeed === "function") {
          initSceneTextModalFeed(false);
        }
      });
    }
    return;
  }

  const html = await resp.text();
  if (!html && targetSelector !== "#modals-root") {
    return;
  }

  if (targetSelector === "#modals-root") {
    const modalsRoot = qs("#modals-root");
    const wasModalSceneTextOpen = Boolean(qs("#modal-scene-text", modalsRoot));
    const prevModalSceneInput = qs('#modal-scene-text input[name="scene_id"]', modalsRoot);
    const prevModalSceneId = prevModalSceneInput ? (prevModalSceneInput.value || "").trim() : "";
    const prevModalFeed = qs(".scene-text-editor-feed", modalsRoot);
    const prevModalScrollTop = prevModalFeed ? prevModalFeed.scrollTop : 0;
    if (modalsRoot) {
      modalsRoot.innerHTML = html;
    }
    const nextSceneTextModal = qs("#modal-scene-text", modalsRoot);
    if (nextSceneTextModal && !wasModalSceneTextOpen) {
      nextSceneTextModal.classList.add("is-entering");
    }
    const nextModalSceneInput = nextSceneTextModal ? qs('input[name="scene_id"]', nextSceneTextModal) : null;
    const nextModalSceneId = nextModalSceneInput ? (nextModalSceneInput.value || "").trim() : "";
    const syncedCanvasSceneId = nextModalSceneId || (actionName === "CloseSceneTextModal" ? prevModalSceneId : "");
    if (syncedCanvasSceneId && typeof selectedNodeIds !== "undefined") {
      if (typeof editorSelectionCleared !== "undefined") {
        editorSelectionCleared = false;
      }
      if (typeof studioGraphState !== "undefined") {
        studioGraphState.selectedChapterKey = "";
        studioGraphState.overlayBatchDirty = true;
      }
      selectedNodeIds.clear();
      selectedNodeIds.add(syncedCanvasSceneId);
      const contextForm = qs("#editor-context-form");
      if (contextForm) {
        contextForm.setAttribute("data-context-scene-id", syncedCanvasSceneId);
        const scInp = qs('input[name="scene_id"]', contextForm);
        if (scInp) scInp.value = syncedCanvasSceneId;
      }
      if (
        prevModalSceneId &&
        nextModalSceneId &&
        prevModalSceneId !== nextModalSceneId &&
        actionName === "SaveSceneTextModal" &&
        typeof syncSceneIdentityAndMetaFromHeaders === "function"
      ) {
        syncSceneIdentityAndMetaFromHeaders(prevModalSceneId, nextModalSceneId);
      }
      if (typeof syncNodeSelectionDOM === "function") {
        syncNodeSelectionDOM();
      }
      if (typeof syncEditorViewportVisibility === "function") {
        syncEditorViewportVisibility();
      }
      if (typeof syncCanvasNodeFromSceneTextModal === "function") {
        syncCanvasNodeFromSceneTextModal(modalsRoot);
      }
      if (typeof syncExpandSceneTextButton === "function") {
        syncExpandSceneTextButton(syncedCanvasSceneId);
      } else {
        const expandBtn = qs("#btn-expand-scene-text");
        if (expandBtn) {
          expandBtn.setAttribute("data-scene", syncedCanvasSceneId);
        }
      }
      if (typeof renderStudioCanvas === "function") {
        renderStudioCanvas();
      }
    } else if (typeof syncEditorViewportVisibility === "function") {
      syncEditorViewportVisibility();
    }
    if (modalsRoot && typeof autoResizeAllInspectorTextareas === "function") {
      autoResizeAllInspectorTextareas(modalsRoot);
    }
    if ((actionName === "NavigateSceneTextModal" || actionName === "SaveSceneTextModal") && params && params.scene_id) {
      const nd =
        typeof studioGraphState !== "undefined" && studioGraphState.nodeById
          ? studioGraphState.nodeById.get(String(params.scene_id))
          : null;
      const contextForm = qs("#editor-context-form");
      const matchesContext = Boolean(contextForm && contextForm.getAttribute("data-context-scene-id") === String(params.scene_id));
      if (params.text !== undefined) {
        const cleaned = String(params.text).trim().replace(/\s+/g, " ");
        let summaryText = tr("scene_text_placeholder");
        if (cleaned) {
          const m = cleaned.match(/^.*?[.!?…](?=\s|$)/);
          summaryText = m ? m[0] : cleaned;
        }
        if (matchesContext) {
          const previewBody = qs(".node-text-preview-body, .node-text-snippet", contextForm);
          if (previewBody) {
            previewBody.textContent = summaryText;
          }
        }
      }
      if (params.name !== undefined) {
        const trimmedName = String(params.name).trim();
        if (nd) {
          nd.name = trimmedName || String(params.scene_id);
        }
        if (matchesContext) {
          const nameInput = qs('input[name="name"]', contextForm);
          if (nameInput) {
            nameInput.value = trimmedName;
          }
        }
      }
      Object.keys(params).forEach((k) => {
        if (k.startsWith("choice_label_")) {
          const idx = k.slice("choice_label_".length);
          const lbl = String(params[k]).trim() || tr("choice_prefix") + (Number(idx) + 1);
          if (nd && nd.ports && nd.ports[Number(idx)]) {
            nd.ports[Number(idx)].label = lbl;
          }
          if (matchesContext) {
            const chInput = qs('input[name="' + CSS.escape(k) + '"]', contextForm);
            if (chInput) {
              chInput.value = lbl;
            }
          }
        }
      });
      if (nd && typeof renderStudioCanvas === "function") {
        studioGraphState.staticBatchDirty = true;
        renderStudioCanvas();
      }
    }
    if (actionName === "OpenSceneTextModal" || actionName === "NavigateSceneTextModal") {
      if (typeof initSceneTextModalFeed === "function") initSceneTextModalFeed(true);
      const modalTa = qs("#modal-scene-textarea");
      const floatingInputActive = Boolean(
        qs("#canvas-node-title-input:not(.hidden), #canvas-choice-input:not(.hidden), #canvas-chapter-input:not(.hidden)")
      );
      if (modalTa && actionName === "OpenSceneTextModal" && !floatingInputActive) {
        modalTa.focus({ preventScroll: true });
      }
    } else if (ACTIONS_KEEP_MODAL_SCROLL.has(actionName)) {
      const feed = qs(".scene-text-editor-feed", modalsRoot);
      if (feed && prevModalScrollTop !== null) feed.scrollTop = prevModalScrollTop;
      if (typeof initSceneTextModalFeed === "function") initSceneTextModalFeed(false);
      if (actionName === "ToggleChoiceRule" && typeof studioGraphState !== "undefined" && studioGraphState.nodeById) {
        const scInp = qs('input[name="scene_id"]', modalsRoot);
        const scId = scInp ? scInp.value : "";
        const nd = scId ? studioGraphState.nodeById.get(scId) : null;
        if (nd && nd.ports) {
          qsa(".modal-choice-row", modalsRoot).forEach((rowEl, idx) => {
            if (nd.ports[idx]) {
              const pIcon = qs(".choice-port-icon", rowEl);
              nd.ports[idx].isConditional = Boolean(pIcon && pIcon.classList.contains("port-diamond"));
            }
          });
          studioGraphState.staticBatchDirty = true;
          if (typeof renderStudioCanvas === "function") renderStudioCanvas();
        }
      }
      if (actionName === "AddBookVariable") restoreSynDropdownFocus(modalsRoot);
    } else if (actionName === "OpenNewBookModal") {
      const newBookInp = qs("#new-book-title", modalsRoot);
      if (newBookInp) setTimeout(() => newBookInp.focus(), 0);
    }
    if (typeof initVoiceRecorderStrips === "function") initVoiceRecorderStrips();
    return;
  }

  if (targetSelector === "#editor-context-panel") {
    const existingPanel = qs("#editor-context-panel");
    const editorBody = qs("#editor-workspace-body");
    if (existingPanel) {
      existingPanel.outerHTML = html;
    } else if (editorBody && html) {
      editorBody.insertAdjacentHTML("afterbegin", html);
    }
    if (actionName === "SaveSceneTextModal" || actionName === "CloseSceneTextModal") {
      const modalsRoot = qs("#modals-root");
      if (modalsRoot) {
        modalsRoot.innerHTML = "";
      }
      const updatedForm = qs("#editor-context-form");
      const selId = updatedForm ? updatedForm.getAttribute("data-context-scene-id") || "" : "";
      if (selId && typeof studioGraphState !== "undefined" && studioGraphState.nodeById) {
        const nd = studioGraphState.nodeById.get(selId);
        if (nd) {
          const nameInp = qs('input[name="name"]', updatedForm);
          if (nameInp) {
            nd.name = nameInp.value.trim() || selId;
          }
          qsa('input[name^="choice_label_"]', updatedForm).forEach((inp) => {
            const idx = inp.getAttribute("data-choice-idx");
            if (idx !== null && nd.ports && nd.ports[Number(idx)]) {
              nd.ports[Number(idx)].label = inp.value.trim() || tr("choice_prefix") + (Number(idx) + 1);
            }
          });
          studioGraphState.staticBatchDirty = true;
        }
      }
    }
    const updatedForm = qs("#editor-context-form");
    const isNodeFormVisible = Boolean(updatedForm && !updatedForm.classList.contains("hidden"));
    const selId = isNodeFormVisible
      ? updatedForm.getAttribute("data-context-scene-id") || ""
      : actionName === "SelectStudioScene" && params && params.scene
        ? String(params.scene)
        : "";
    if (selId && typeof selectedNodeIds !== "undefined" && typeof syncNodeSelectionDOM === "function") {
      if (actionName === "SelectStudioScene") {
        if (typeof editorSelectionCleared !== "undefined") {
          editorSelectionCleared = false;
        }
        selectedNodeIds.clear();
        selectedNodeIds.add(selId);
      } else if ((typeof editorSelectionCleared === "undefined" || !editorSelectionCleared) && selectedNodeIds.size <= 1) {
        selectedNodeIds.clear();
        selectedNodeIds.add(selId);
      }
      syncNodeSelectionDOM();
      if (typeof syncEditorViewportVisibility === "function") {
        syncEditorViewportVisibility();
      }
      if (typeof renderStudioCanvas === "function") {
        renderStudioCanvas();
      }
    } else {
      if (typeof syncEditorViewportVisibility === "function") {
        syncEditorViewportVisibility();
      }
      if (typeof autoResizeAllInspectorTextareas === "function") {
        autoResizeAllInspectorTextareas(qs("#editor-context-panel") || document);
      }
    }
    if (typeof initVoiceRecorderStrips === "function") {
      initVoiceRecorderStrips();
    }
    return;
  }

  const prevFeed = qs("#story-scroll-feed");
  const prevScrollTop = prevFeed ? prevFeed.scrollTop : null;
  const prevEditorViewport = qs("#timeline-scroll-viewport");
  const prevEditorScrollTop = prevEditorViewport ? prevEditorViewport.scrollTop : null;
  const prevEditorScrollLeft = prevEditorViewport ? prevEditorViewport.scrollLeft : null;
  const prevHistScroll = qs("#history-graph-scroll");
  const prevHistScrollTop = prevHistScroll ? prevHistScroll.scrollTop : null;
  const resetCanvas = resp.headers.get("X-Reset-Canvas") === "true";
  const isAnimatedSceneTransition =
    actionName === "ChooseBranch" ||
    actionName === "AutoAdvanceImplicit" ||
    actionName === "AudioEnded";

  if (resetCanvas && typeof resetEditorCanvasViewState === "function") {
    resetEditorCanvasViewState();
  }

  const wasSceneTextOpen = Boolean(qs("#modal-scene-text"));
  const wasModalTextareaFocused = Boolean(
    typeof document !== "undefined" && document.activeElement && document.activeElement.id === "modal-scene-textarea"
  );
  const prevOpenRightPanels = new Set();
  qsa("#editor-workspace-body > aside:not(.hidden)").forEach((el) => {
    if (el.id) prevOpenRightPanels.add(el.id);
  });
  const prevModalFeed = qs(".scene-text-editor-feed");
  const prevModalScrollTop = prevModalFeed ? prevModalFeed.scrollTop : null;
  const targetEl = qs(targetSelector) || qs("#app");
  if (targetEl) {
    targetEl.innerHTML = html;
    if (targetSelector === "#app" || targetEl.id === "app") {
      const restoredToast = decodeHeaderUTF8(resp.headers.get("X-Toast"));
      if (restoredToast) {
        showToast(restoredToast);
      }
    }
    if (targetSelector === "#workspace") {
      const modalsRoot = qs("#modals-root");
      if (modalsRoot && modalsRoot.innerHTML !== "") {
        const keepSceneText =
          actionName !== "CloseSceneTextModal" &&
          Boolean(qs("#view-editor", targetEl)) &&
          Boolean(qs("#modal-scene-text", modalsRoot));
        if (!keepSceneText) {
          modalsRoot.innerHTML = "";
        }
      }
    }
    qsa("#editor-workspace-body > aside:not(.hidden)").forEach((el) => {
      if (el.id && !prevOpenRightPanels.has(el.id)) {
        el.classList.add("is-entering");
      }
    });
    const nextSceneTextModal = qs("#modal-scene-text");
    if (nextSceneTextModal && !wasSceneTextOpen) {
      nextSceneTextModal.classList.add("is-entering");
    }
    if (
      nextSceneTextModal &&
      nextSceneTextModal.classList.contains("mode-full") &&
      (actionName === "ToggleBookSettings" ||
        actionName === "ToggleEditorVariablesPanel" ||
        actionName === "ToggleEditorBreakdownPanel" ||
        actionName === "ToggleEditorHistoryPanel" ||
        actionName === "OpenEditorHelpModal") &&
      typeof isAnyEditorPanelOpen === "function" &&
      isAnyEditorPanelOpen()
    ) {
      nextSceneTextModal.classList.remove("mode-full");
      nextSceneTextModal.classList.add("mode-half");
      const expandFullBtn = qs("#btn-expand-scene-text-full", nextSceneTextModal);
      if (expandFullBtn) expandFullBtn.classList.remove("hidden");
    }
    syncFullscreenButtonState();
    syncTopbarToggleIcons();
    syncEditorHistoryHeaders(resp);
    syncBreakdownActBar(targetEl);
    if (typeof autoResizeAllInspectorTextareas === "function") {
      autoResizeAllInspectorTextareas(targetEl);
    }
    if (typeof initVoiceRecorderStrips === "function") {
      initVoiceRecorderStrips();
    }
    const nextModalFeed = qs(".scene-text-editor-feed");
    if (nextModalFeed && prevModalScrollTop !== null && actionName !== "OpenSceneTextModal" && actionName !== "NavigateSceneTextModal") {
      nextModalFeed.scrollTop = prevModalScrollTop;
    }
    if (actionName === "AddBookVariable" && typeof lastSynRuleInputName === "string" && lastSynRuleInputName) {
      const restoredInput = qs('input[name="' + lastSynRuleInputName + '"]', targetEl);
      if (restoredInput && typeof updateSynAutocompleteOnInput === "function") {
        restoredInput.focus();
        updateSynAutocompleteOnInput(restoredInput, true);
      }
    }
    const nextEditorViewport = qs("#timeline-scroll-viewport");
    if (nextEditorViewport && prevEditorScrollTop !== null) {
      nextEditorViewport.scrollTop = prevEditorScrollTop;
      nextEditorViewport.scrollLeft = prevEditorScrollLeft || 0;
    }
    const nextHistScroll = qs("#history-graph-scroll");
    if (nextHistScroll && prevHistScrollTop !== null && actionName !== "SaveHistoryVersion" && actionName !== "CreateHistoryBranch") {
      nextHistScroll.scrollTop = prevHistScrollTop;
    }
    if (typeof updateHistoryScrollCue === "function") {
      requestAnimationFrame(() => {
        updateHistoryScrollCue();
      });
    }

    const nextAudio = qs("#story-audio");
    const nextAutoChoice = nextAudio ? nextAudio.getAttribute("data-auto-choice") === "true" : false;
    const nextAudioUrl = nextAudio ? nextAudio.getAttribute("data-audio-url") || "" : "";
    const isSameSceneKeepScroll = Boolean(keepScroll && prevAudioUrl && prevAudioUrl === nextAudioUrl);

    let shouldAutoPlay = false;
    if (nextAudio && nextAudioUrl) {
      if (isSameSceneKeepScroll) {
        shouldAutoPlay = prevWasPlaying;
      } else {
        shouldAutoPlay = true;
      }
    } else if (actionName === "ToggleAutoChoice" && nextAutoChoice) {
      shouldAutoPlay = true;
      autoPlayChainActive = true;
    } else if (actionName === "ToggleAutoChoice" && !nextAutoChoice) {
      autoPlayChainActive = false;
    } else if (nextAutoChoice && (!keepScroll || autoPlayChainActive)) {
      shouldAutoPlay = true;
      autoPlayChainActive = true;
    }

    const preserveTime = isSameSceneKeepScroll ? prevAudioTime : null;

    initPlayerScrollAndAudio(
      keepScroll ? prevScrollTop : null,
      shouldAutoPlay,
      preserveTime,
      isAnimatedSceneTransition ? prevScrollTop : null
    );

    if (actionName === "OpenSceneTextModal") {
      if (typeof initSceneTextModalFeed === "function") {
        initSceneTextModalFeed(true);
      }
      const modalTa = qs("#modal-scene-textarea");
      const floatingInputActive = Boolean(
        qs("#canvas-node-title-input:not(.hidden), #canvas-choice-input:not(.hidden), #canvas-chapter-input:not(.hidden)")
      );
      if (modalTa && !floatingInputActive) {
        modalTa.focus({ preventScroll: true });
      }
      if (typeof applyPendingSceneTextSearchHighlight === "function") {
        applyPendingSceneTextSearchHighlight();
      }
    } else if (qs("#modal-scene-text") && typeof initSceneTextModalFeed === "function") {
      initSceneTextModalFeed(false);
      if (wasModalTextareaFocused && (actionName === "UndoEditor" || actionName === "RedoEditor" || actionName === "RestoreHistoryRevision")) {
        const modalTa = qs("#modal-scene-textarea");
        if (modalTa) modalTa.focus({ preventScroll: true });
      }
    }
  }
}

function syncBreakdownActBar(root) {
  qsa(".breakdown-act-segment[data-share-pct]", root || document).forEach((seg) => {
    const pct = Math.max(1, Number(seg.getAttribute("data-share-pct")) || 1);
    seg.style.flexGrow = String(pct);
  });
}

function decodeHeaderComponentSafe(val) {
  if (!val) return "";
  try {
    return decodeURIComponent(String(val).replace(/\+/g, " "));
  } catch (_e) {
    return String(val);
  }
}

function postServerActionSilent(actionName, paramsOrFormData) {
  const wfpPayload = { action: actionName };
  if (paramsOrFormData) {
    if (typeof FormData !== "undefined" && paramsOrFormData instanceof FormData) {
      paramsOrFormData.forEach((val, key) => {
        if (val !== undefined && val !== null) {
          wfpPayload[key] = String(val);
        }
      });
    } else {
      Object.keys(paramsOrFormData).forEach((k) => {
        if (paramsOrFormData[k] !== undefined && paramsOrFormData[k] !== null) {
          wfpPayload[k] = String(paramsOrFormData[k]);
        }
      });
    }
  }
  const sentSceneId = (wfpPayload.scene_id || wfpPayload.scene || "").trim();
  if (typeof window.__wasmAction === "function") {
    parseWasmActionResponse(window.__wasmAction(encodeWFP(wfpPayload)));
    return;
  }
  return fetch("/api/action", {
    method: "POST",
    headers: {
      "Content-Type": "text/x-wfp;charset=UTF-8",
      "X-Requested-With": "Mojra"
    },
    body: encodeWFP(wfpPayload)
  })
    .then((resp) => {
      if (resp) {
        syncEditorHistoryHeaders(resp);
        if (actionName === "AutoSaveScene" && typeof syncSceneIdentityAndMetaFromHeaders === "function") {
          const newId = (resp.headers.get("X-Scene-ID") || "").trim();
          const newExpRaw = resp.headers.get("X-Scene-Explicit-ID");
          const newExp = newExpRaw !== null ? newExpRaw.trim() : undefined;
          const newRef = decodeHeaderComponentSafe(resp.headers.get("X-Scene-Ref"));
          const newTitle = decodeHeaderComponentSafe(resp.headers.get("X-Scene-Title"));
          const newChapter = decodeHeaderComponentSafe(resp.headers.get("X-Scene-Chapter"));
          const startScId = (resp.headers.get("X-Start-Scene-ID") || "").trim();
          syncSceneIdentityAndMetaFromHeaders(sentSceneId, newId, newExp, newRef, newTitle, newChapter, startScId);
        }
        const voicePeak = resp.headers.get("X-Voice-Peak");
        if (voicePeak && typeof handleNativeVoicePeakHeader === "function") {
          handleNativeVoicePeakHeader(voicePeak);
        }
      }
    })
    .catch(() => {});
}

function bindDeclarativeActionBridge() {
  document.addEventListener("pointerover", (e) => {
    if (e.pointerType && e.pointerType !== "mouse" && e.pointerType !== "pen") {
      return;
    }
    const t = e.target;
    if (!t || !t.closest) return;
    const rawTitleEl = t.closest("[title]");
    if (rawTitleEl) {
      const rawVal = rawTitleEl.getAttribute("title") || "";
      rawTitleEl.removeAttribute("title");
      if (rawVal && !rawTitleEl.getAttribute("data-hint")) {
        rawTitleEl.setAttribute("data-hint", rawVal);
      }
    }
    const hintEl = t.closest("[data-hint]");
    if (hintEl === activeHintEl) return;
    if (uiTooltipTimer) {
      clearTimeout(uiTooltipTimer);
      uiTooltipTimer = null;
    }
    if (!hintEl || !(hintEl.getAttribute("data-hint") || "").trim()) {
      hideUITooltip();
      return;
    }
    activeHintEl = hintEl;
    const tip = qs("#ui-tooltip");
    if (tip && !tip.classList.contains("hidden")) {
      positionAndShowUITooltip(hintEl);
    } else {
      uiTooltipTimer = setTimeout(() => {
        uiTooltipTimer = null;
        if (activeHintEl === hintEl) {
          positionAndShowUITooltip(hintEl);
        }
      }, 120);
    }
  });

  document.addEventListener("pointermove", (e) => {
    if (activeHintEl) return;
    if (e.pointerType && e.pointerType !== "mouse" && e.pointerType !== "pen") {
      return;
    }
    const t = e.target;
    if (!t || !t.closest) return;
    const hintEl = t.closest("[data-hint]");
    if (!hintEl || !(hintEl.getAttribute("data-hint") || "").trim()) return;
    activeHintEl = hintEl;
    if (uiTooltipTimer) {
      clearTimeout(uiTooltipTimer);
    }
    uiTooltipTimer = setTimeout(() => {
      uiTooltipTimer = null;
      if (activeHintEl === hintEl) {
        positionAndShowUITooltip(hintEl);
      }
    }, 120);
  });

  document.addEventListener("pointerout", (e) => {
    if (!activeHintEl) return;
    const rel = e.relatedTarget;
    if (rel && activeHintEl.contains(rel)) {
      return;
    }
    const nextHint = rel && rel.closest ? rel.closest("[data-hint]") : null;
    if (!nextHint) {
      hideUITooltip();
    }
  });

  document.addEventListener("pointerdown", () => {
    hideUITooltip();
  });

  document.addEventListener("wheel", () => {
    hideUITooltip();
  }, { passive: true });

  document.addEventListener("scroll", () => {
    hideUITooltip();
  }, true);

  document.addEventListener("keydown", (e) => {
    hideUITooltip();
    let allowDevTools = false;
    /* @dev-only-start */
    allowDevTools = true;
    /* @dev-only-end */
    if (!allowDevTools && (e.key === "F12" || e.code === "F12")) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    const inEditableField = Boolean(
      e.target &&
        e.target.closest &&
        e.target.closest("input, textarea, select, [contenteditable]")
    );
    const hasZoomMod = Boolean(e.ctrlKey || e.metaKey);
    const isPlusKey =
      e.key === "+" ||
      e.key === "=" ||
      e.code === "Equal" ||
      e.code === "NumpadAdd";
    const isMinusKey =
      e.key === "-" ||
      e.key === "_" ||
      e.code === "Minus" ||
      e.code === "NumpadSubtract";
    const isZeroKey =
      e.key === "0" ||
      e.code === "Digit0" ||
      e.code === "Numpad0";

    if (!e.altKey) {
      if (
        (hasZoomMod && isPlusKey) ||
        (!hasZoomMod && !inEditableField && (e.key === "+" || e.key === "=" || e.code === "NumpadAdd"))
      ) {
        e.preventDefault();
        toggleUIScaleClient("large");
        return;
      }
      if (
        (hasZoomMod && isMinusKey) ||
        (!hasZoomMod && !inEditableField && (e.key === "-" || e.code === "NumpadSubtract"))
      ) {
        e.preventDefault();
        toggleUIScaleClient("small");
        return;
      }
      if (hasZoomMod && isZeroKey) {
        e.preventDefault();
        toggleUIScaleClient("small");
        return;
      }
    }

    if (hasZoomMod && !e.altKey && (e.key === "f" || e.key === "F" || e.code === "KeyF")) {
      if (qs("#view-editor") && typeof openStudioSearchPalette === "function") {
        e.preventDefault();
        e.stopPropagation();
        openStudioSearchPalette();
        return;
      }
    }

    if (e.target && e.target.id === "studio-search-input") {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        if (typeof stepStudioSearchActiveRow === "function") stepStudioSearchActiveRow(1);
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        if (typeof stepStudioSearchActiveRow === "function") stepStudioSearchActiveRow(-1);
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        const activeRow = qs("#studio-search-results .studio-search-row.is-active:not(.hidden)");
        if (activeRow && typeof activateStudioSearchRow === "function") {
          activateStudioSearchRow(activeRow.getAttribute("data-search-scene") || "", Boolean(e.shiftKey));
        }
        return;
      }
    }

    if (e.key === "Escape") {
      const searchPalette = qs("#studio-search-palette");
      if (searchPalette && !searchPalette.classList.contains("hidden") && typeof closeStudioSearchPalette === "function") {
        e.preventDefault();
        closeStudioSearchPalette();
        return;
      }
      const topbarMenu = qs("#topbar-actions-menu");
      if (topbarMenu) topbarMenu.classList.remove("open");
      if (typeof closeCanvasContextMenu === "function") {
        closeCanvasContextMenu();
      }
      if (typeof closeAllCustomSelects === "function") {
        closeAllCustomSelects(null);
      }
      const openBackdrop = qs("#modal-confirm[data-backdrop-action]") || qs(".modal-backdrop[data-backdrop-action]");
      if (openBackdrop) {
        const closeAct = openBackdrop.getAttribute("data-backdrop-action");
        if (closeAct) {
          sendServerAction(closeAct, {});
          return;
        }
      }
      if (document.body.getAttribute("data-zen") === "true" && qs("#view-player")) {
        sendServerAction("ToggleZenMode", { zen: "false" });
        return;
      }
    }
  });

  document.addEventListener("mousedown", (e) => {
    if (e.button === 0) {
      let dragBarSelector = ".topbar";
      /* @dev-only-start */
      dragBarSelector = ".device-preview-bar, .topbar";
      /* @dev-only-end */
      const dragBar = e.target.closest(dragBarSelector);
      if (dragBar && !e.target.closest("button, a, input, select, textarea, label, [action], .brand-logo")) {
        if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.nativeHost) {
          window.webkit.messageHandlers.nativeHost.postMessage("start-window-drag");
        } else {
          postServerActionSilent("StartWindowDrag");
        }
      }
    }
  });

  document.addEventListener("click", (e) => {
    const topbarMenuToggle = e.target.closest("[data-topbar-menu-toggle]");
    if (topbarMenuToggle) {
      e.preventDefault();
      e.stopPropagation();
      const menu = qs("#topbar-actions-menu");
      if (menu) {
        const willOpen = !menu.classList.contains("open");
        menu.classList.toggle("open", willOpen);
        topbarMenuToggle.classList.toggle("active", willOpen);
      }
      return;
    }
    if (!e.target.closest("#topbar-right")) {
      const menu = qs("#topbar-actions-menu");
      if (menu) menu.classList.remove("open");
    }

    if (typeof isEditorTouchClickSuppressed === "function" && isEditorTouchClickSuppressed(e)) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }

    if (e.target && e.target.classList && e.target.classList.contains("modal-backdrop")) {
      const backdropAction = e.target.getAttribute("data-backdrop-action");
      if (backdropAction) {
        sendServerAction(backdropAction, {});
        return;
      }
    }

    const searchToggleBtn = e.target && e.target.closest ? e.target.closest("[data-studio-search-toggle]") : null;
    if (searchToggleBtn && typeof toggleStudioSearchPalette === "function") {
      e.preventDefault();
      e.stopPropagation();
      toggleStudioSearchPalette();
      return;
    }

    const searchCloseBtn = e.target && e.target.closest ? e.target.closest("[data-studio-search-close]") : null;
    if (searchCloseBtn && typeof closeStudioSearchPalette === "function") {
      e.preventDefault();
      e.stopPropagation();
      closeStudioSearchPalette();
      return;
    }

    const searchOpenTextBtn = e.target && e.target.closest ? e.target.closest("[data-search-open-text]") : null;
    if (searchOpenTextBtn && typeof activateStudioSearchRow === "function") {
      e.preventDefault();
      e.stopPropagation();
      activateStudioSearchRow(searchOpenTextBtn.getAttribute("data-search-open-text") || "", true);
      return;
    }

    const searchRowEl = e.target && e.target.closest ? e.target.closest(".studio-search-row[data-search-scene]") : null;
    if (searchRowEl && typeof activateStudioSearchRow === "function") {
      e.preventDefault();
      e.stopPropagation();
      activateStudioSearchRow(searchRowEl.getAttribute("data-search-scene") || "", false);
      return;
    }

    const wasmPlayBtn = e.target && e.target.closest ? e.target.closest("[data-wasm-play]") : null;
    if (wasmPlayBtn) {
      e.preventDefault();
      e.stopPropagation();
      bootWasmPlayer(wasmPlayBtn);
      return;
    }

    const warningsToggle = e.target.closest("[data-warnings-toggle]");
    if (warningsToggle) {
      e.preventDefault();
      e.stopPropagation();
      const listEl = qs("#editor-status-warnings-list");
      const expandIcon = qs("#icon-warnings-expand", warningsToggle);
      const collapseIcon = qs("#icon-warnings-collapse", warningsToggle);
      if (listEl) {
        const willOpen = listEl.classList.contains("hidden");
        listEl.classList.toggle("hidden", !willOpen);
        warningsToggle.classList.toggle("active", willOpen);
        if (expandIcon) expandIcon.classList.toggle("hidden", willOpen);
        if (collapseIcon) collapseIcon.classList.toggle("hidden", !willOpen);
      }
      return;
    }

    const settingsToggle = e.target.closest("[data-scene-settings-toggle]");
    if (settingsToggle) {
      e.preventDefault();
      e.stopPropagation();
      const settingsBlock = qs("#modal-scene-settings-block");
      const openInput = qs("#scene-settings-open-input");
      if (settingsBlock) {
        const willOpen = settingsBlock.classList.contains("hidden");
        settingsBlock.classList.toggle("hidden", !willOpen);
        settingsToggle.classList.toggle("active", willOpen);
        if (openInput) {
          openInput.value = willOpen ? "true" : "false";
        }
        if (typeof updateFeedScrollCue === "function") {
          updateFeedScrollCue(qs(".scene-text-editor-feed"));
        }
      }
      return;
    }

    if (typeof handleEditorCanvasClick === "function" && handleEditorCanvasClick(e)) {
      return;
    }

    const submitBtn = e.target.closest('form[action] button[type="submit"], form[action] input[type="submit"]');
    if (submitBtn) {
      if (submitBtn.disabled || submitBtn.classList.contains("is-disabled")) {
        e.preventDefault();
        e.stopPropagation();
        return;
      }
      const form = submitBtn.closest("form[action]");
      if (form) {
        e.preventDefault();
        e.stopPropagation();
        const actionName = (form.getAttribute("action") || "").trim();
        if (actionName) {
          const params = {};
          const fd = new FormData(form);
          fd.forEach((val, key) => {
            params[key] = val;
          });
          sendServerAction(actionName, params);
          return;
        }
      }
    }

    const target = e.target.closest("[action]");
    if (!target || target.tagName.toLowerCase() === "form" || target.disabled || target.classList.contains("is-disabled")) {
      return;
    }
    const actionName = (target.getAttribute("action") || "").trim();
    if (!actionName) {
      return;
    }
    if (actionName === "ToggleImageless" && document.body.getAttribute("data-imageless-locked") === "true") {
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    e.preventDefault();
    e.stopPropagation();

    if (actionName === "SetSceneTextWidthMode") {
      const sceneModal = qs("#modal-scene-text");
      if (sceneModal) {
        setSceneTextModalWidthMode(sceneModal, (target.getAttribute("data-mode") || "full").toLowerCase() === "full");
        return;
      }
    }
    if (actionName === "CloseSceneTextModal" && target.id === "btn-collapse-scene-text") {
      const sceneModal = qs("#modal-scene-text");
      if (sceneModal && sceneModal.classList.contains("mode-full")) {
        setSceneTextModalWidthMode(sceneModal, false);
        return;
      }
      if (sceneModal) {
        sceneModal.classList.remove("is-entering");
        sceneModal.classList.add("is-collapsing");
      }
    }

    const params = {};
    const isModalOnlyAction =
      actionName === "OpenSceneTextModal" ||
      actionName === "SelectStudioScene" ||
      actionName === "PromptDeleteScene" ||
      actionName === "PromptDeleteScenes";
    if (
      actionName === "CreateSceneAt" ||
      actionName === "InsertScene" ||
      actionName === "InsertSceneBefore" ||
      actionName === "BranchChoiceScene" ||
      actionName === "CreateChapterGroup"
    ) {
      if (typeof editorSelectionCleared !== "undefined") {
        editorSelectionCleared = false;
      }
      if (typeof selectedNodeIds !== "undefined") {
        selectedNodeIds.clear();
      }
    }
    const parentCardForm = target.closest(
      "form.editor-context-form, form.timeline-node-card, form#form-scene-text, form#form-new-book, form#form-export-project, form.var-panel-inline-form"
    );
    if (parentCardForm && (!isModalOnlyAction || (typeof dirtyInspectorForms !== "undefined" && dirtyInspectorForms.has(parentCardForm)))) {
      if (typeof dirtyInspectorForms !== "undefined") {
        dirtyInspectorForms.delete(parentCardForm);
      }
      if (typeof liveSaveTimers !== "undefined" && liveSaveTimers.has(parentCardForm)) {
        clearTimeout(liveSaveTimers.get(parentCardForm));
        liveSaveTimers.delete(parentCardForm);
      }
      const fd = new FormData(parentCardForm);
      fd.forEach((val, key) => {
        params[key] = val;
      });
    } else if (!isModalOnlyAction && target.dataset && target.dataset.scene) {
      const contextForm = typeof getMatchingContextForm === "function" ? getMatchingContextForm(target.dataset.scene) : null;
      if (contextForm) {
        if (typeof dirtyInspectorForms !== "undefined") dirtyInspectorForms.delete(contextForm);
        if (typeof liveSaveTimers !== "undefined" && liveSaveTimers.has(contextForm)) {
          clearTimeout(liveSaveTimers.get(contextForm));
          liveSaveTimers.delete(contextForm);
        }
        const fd = new FormData(contextForm);
        fd.forEach((val, key) => {
          params[key] = val;
        });
      }
    }
    Object.keys(target.dataset).forEach((k) => {
      const snakeKey = k.replace(/[A-Z]/g, (m) => "_" + m.toLowerCase());
      params[snakeKey] = target.dataset[k];
    });
    if (actionName === "PromptDeleteScenes" && !params.scenes && typeof selectedNodeIds !== "undefined" && selectedNodeIds.size > 0) {
      params.scenes = Array.from(selectedNodeIds).join(",");
    }
    const topbarMenu = qs("#topbar-actions-menu");
    if (topbarMenu) topbarMenu.classList.remove("open");
    sendServerAction(actionName, params);
  });

  document.addEventListener("input", (e) => {
    if (e.target && e.target.id === "studio-search-input" && typeof applyStudioSearchFilter === "function") {
      applyStudioSearchFilter(e.target.value || "");
    }
  });

  document.addEventListener("submit", (e) => {
    const form = e.target.closest("form[action]");
    if (!form) return;
    e.preventDefault();

    const actionName = (form.getAttribute("action") || "").trim();
    if (!actionName) return;
    const params = {};
    const fd = new FormData(form);
    fd.forEach((val, key) => {
      params[key] = val;
    });
    sendServerAction(actionName, params);
  });

  window.addEventListener("resize", () => {
    if (typeof restorePlayerScrollPosition === "function") {
      restorePlayerScrollPosition();
    }
    if (typeof initSceneTextModalFeed === "function") {
      initSceneTextModalFeed(false);
    }
    if (typeof updateChapterGroupFrames === "function") {
      updateChapterGroupFrames();
    }
    if (typeof drawStudioWires === "function") {
      drawStudioWires();
    }
    if (typeof autoResizeAllInspectorTextareas === "function") {
      autoResizeAllInspectorTextareas(document);
    }
    if (typeof syncEditorViewportVisibility === "function") {
      syncEditorViewportVisibility();
    }
  });

  if (typeof bindEditorCanvasEvents === "function") {
    bindEditorCanvasEvents();
  }

  syncTopbarToggleIcons();
  syncBreakdownActBar(document);
  if (typeof autoResizeAllInspectorTextareas === "function") {
    autoResizeAllInspectorTextareas(document);
  }
  if (typeof syncEditorViewportVisibility === "function") {
    syncEditorViewportVisibility();
  }
  if (typeof initPlayerScrollAndAudio === "function") {
    initPlayerScrollAndAudio(null, Boolean(qs("#story-audio")), null, null);
  }
  if (typeof initVoiceRecorderStrips === "function") {
    initVoiceRecorderStrips();
  }
}

document.addEventListener("DOMContentLoaded", bindDeclarativeActionBridge);
