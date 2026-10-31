"use strict";

let toastTimer = null;
let lastAudioSaveAt = 0;
let autoPlayChainActive = false;
let isWindowFullscreen = false;

function qs(sel, root) {
  return (root || document).querySelector(sel);
}

function qsa(sel, root) {
  return Array.from((root || document).querySelectorAll(sel));
}

function syncFullscreenButtonState() {
  const enterIcon = qs("#icon-fullscreen-enter");
  const exitIcon = qs("#icon-fullscreen-exit");
  const btn = qs("#btn-toggle-fullscreen");
  if (enterIcon) enterIcon.classList.toggle("hidden", isWindowFullscreen);
  if (exitIcon) exitIcon.classList.toggle("hidden", !isWindowFullscreen);
  if (btn) {
    btn.setAttribute(
      "title",
      isWindowFullscreen ? "Wróć do trybu okienkowego" : "Przełącz pełny ekran"
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
  const zoomInIcon = qs("#icon-zoom-in");
  const zoomOutIcon = qs("#icon-zoom-out");
  const zoomBtn = qs("#btn-toggle-ui-scale");
  if (zoomInIcon) zoomInIcon.classList.toggle("hidden", isLarge);
  if (zoomOutIcon) zoomOutIcon.classList.toggle("hidden", !isLarge);
  if (zoomBtn) {
    zoomBtn.setAttribute(
      "title",
      isLarge ? "Pomniejsz przyciski i kontrolki (- / Ctrl+-)" : "Powiększ przyciski i kontrolki (+ / Ctrl++)"
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
    imgBtn.classList.toggle("active", isImageless);
    imgBtn.setAttribute("data-imageless-locked", isImagelessLocked ? "true" : "false");
    imgBtn.setAttribute(
      "title",
      isImagelessLocked
        ? "Obrazy wyłączone w ustawieniach książki (imageless: true)"
        : isImageless
          ? "Włącz obrazy (tryb bez obrazów aktywny)"
          : "Wyłącz obrazy (tryb tekstowy na środku)"
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
  const canUndoHeader = resp.headers.get("X-Can-Undo");
  const canRedoHeader = resp.headers.get("X-Can-Redo");
  const histCountHeader = resp.headers.get("X-History-Count");
  const undoTitleHeader = decodeHeaderUTF8(resp.headers.get("X-Undo-Title"));
  const redoTitleHeader = decodeHeaderUTF8(resp.headers.get("X-Redo-Title"));

  if (canUndoHeader !== null) {
    const canUndo = canUndoHeader === "true";
    const undoBtn = qs("#btn-editor-undo");
    if (undoBtn) {
      undoBtn.disabled = !canUndo;
      undoBtn.classList.toggle("is-disabled", !canUndo);
      if (undoTitleHeader) {
        undoBtn.setAttribute("title", undoTitleHeader);
      }
    }
  }

  if (canRedoHeader !== null) {
    const canRedo = canRedoHeader === "true";
    const redoBtn = qs("#btn-editor-redo");
    if (redoBtn) {
      redoBtn.disabled = !canRedo;
      redoBtn.classList.toggle("is-disabled", !canRedo);
      if (redoTitleHeader) {
        redoBtn.setAttribute("title", redoTitleHeader);
      }
    }
  }

  if (histCountHeader !== null) {
    const count = parseInt(histCountHeader, 10) || 0;
    const badge = qs("#history-unsaved-badge");
    if (badge) {
      badge.textContent = String(count);
      badge.classList.toggle("hidden", count <= 0);
    }
  }
}

function applyServerResponseHeaders(resp, actionName) {
  syncEditorHistoryHeaders(resp);
  const theme = resp.headers.get("X-Theme");
  if (theme) {
    document.body.setAttribute("data-theme", theme);
    if (typeof invalidateStudioThemeCache === "function") {
      invalidateStudioThemeCache();
    }
    if (typeof renderStudioCanvas === "function") {
      renderStudioCanvas();
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
  if (uiScale) {
    document.body.setAttribute("data-ui-scale", uiScale);
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

  const autoChoice = resp.headers.get("X-Auto-Choice");
  if (autoChoice) {
    const isAutoOn = autoChoice === "true";
    const audioEl = qs("#story-audio");
    if (audioEl) {
      audioEl.setAttribute("data-auto-choice", autoChoice);
    }
    const autoBtn = qs("#btn-auto-choice-toggle");
    if (autoBtn) {
      autoBtn.classList.toggle("chip-active", isAutoOn);
      autoBtn.setAttribute("title", isAutoOn ? "Auto-Wybór: WŁ" : "Auto-Wybór: WYŁ");
    }
    if (actionName === "ToggleAutoChoice") {
      if (isAutoOn) {
        autoPlayChainActive = true;
        if (audioEl && audioEl.paused && audioEl.getAttribute("data-audio-url")) {
          audioEl.play().catch(() => {});
        } else if (
          (!audioEl || !audioEl.getAttribute("data-audio-url")) &&
          qsa('#active-scene-stage .choice-btn[action="ChooseBranch"]').length > 0
        ) {
          if (typeof autoAdvanceFallbackTimer !== "undefined" && autoAdvanceFallbackTimer) {
            clearTimeout(autoAdvanceFallbackTimer);
          }
          autoAdvanceFallbackTimer = setTimeout(() => {
            sendServerAction("AudioEnded", {});
          }, 1500);
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
      (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.storytellerHost) ||
      (window.chrome && window.chrome.webview) ||
      window.__storytellerNativeHost
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
    if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.storytellerHost) {
      window.webkit.messageHandlers.storytellerHost.postMessage("quit");
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
  return false;
}

async function sendServerAction(actionName, params) {
  params = params || {};
  if (
    actionName === "UndoEditor" ||
    actionName === "ToggleEditorHistoryPanel" ||
    actionName === "OpenEditorHistoryPanel" ||
    actionName === "SaveHistoryVersion" ||
    actionName === "CloseSceneTextModal" ||
    actionName === "NavigateSceneTextModal" ||
    actionName === "SwitchView" ||
    actionName === "OpenBook"
  ) {
    if (typeof consumeDirtyInspectorFormIntoParams === "function") {
      consumeDirtyInspectorFormIntoParams(params);
    }
  } else if (actionName === "RedoEditor" || actionName === "RestoreHistoryRevision") {
    if (typeof cancelPendingInspectorTimers === "function") {
      cancelPendingInspectorTimers();
    }
  }

  const prevAudio = qs("#story-audio");
  const prevWasPlaying = prevAudio ? !prevAudio.paused && !prevAudio.ended : false;
  const prevAudioTime = prevAudio ? prevAudio.currentTime || 0 : 0;
  const prevAudioUrl = prevAudio ? prevAudio.getAttribute("data-audio-url") || "" : "";

  const form = new URLSearchParams();
  form.set("action", actionName);
  if (prevAudio && prevAudioTime > 0 && actionName !== "SaveAudioTime") {
    form.set("audio_time", String(Math.floor(prevAudioTime * 10) / 10));
  }
  if (params) {
    Object.keys(params).forEach((k) => {
      if (params[k] !== undefined && params[k] !== null) {
        form.set(k, String(params[k]));
      }
    });
  }

  const resp = await fetch("/api/action", {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      "X-Requested-With": "StoryTeller"
    },
    body: form.toString()
  });

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
    let modalsRoot = qs("#modals-root");
    if (!modalsRoot) {
      const appRoot = qs("#app");
      if (appRoot) {
        modalsRoot = document.createElement("div");
        modalsRoot.id = "modals-root";
        modalsRoot.className = "modals-root";
        appRoot.appendChild(modalsRoot);
      }
    }
    const prevModalFeed = qs(".scene-text-editor-feed", modalsRoot);
    const prevModalScrollTop = prevModalFeed ? prevModalFeed.scrollTop : 0;
    if (modalsRoot) {
      modalsRoot.innerHTML = html;
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
        let summaryText = "Kliknij, aby wpisać treść sceny...";
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
          const lbl = String(params[k]).trim() || "Wybór " + (Number(idx) + 1);
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
      if (typeof initSceneTextModalFeed === "function") {
        initSceneTextModalFeed(true);
      }
      const modalTa = qs("#modal-scene-textarea");
      if (modalTa && actionName === "OpenSceneTextModal") {
        modalTa.focus({ preventScroll: true });
      }
    } else if (
      actionName === "SaveSceneTextModal" ||
      actionName === "ToggleSceneTextSettings" ||
      actionName === "SetStartScene" ||
      actionName === "BrowseAddBookMedia" ||
      actionName === "AddBookMediaFromDisk" ||
      actionName === "AddChoice" ||
      actionName === "PromptRemoveChoice" ||
      actionName === "RemoveChoice" ||
      actionName === "ConfirmModalClose" ||
      actionName === "ConfirmModalAccept" ||
      actionName === "BranchChoiceScene" ||
      actionName === "ConnectChoice"
    ) {
      const feed = qs(".scene-text-editor-feed", modalsRoot);
      if (feed) {
        feed.scrollTop = prevModalScrollTop;
      }
      if (typeof initSceneTextModalFeed === "function") {
        initSceneTextModalFeed(false);
      }
    } else if (actionName === "OpenNewBookModal") {
      const newBookInp = qs("#new-book-title", modalsRoot);
      if (newBookInp) {
        setTimeout(() => {
          newBookInp.focus();
        }, 0);
      }
    }
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
              nd.ports[Number(idx)].label = inp.value.trim() || "Wybór " + (Number(idx) + 1);
            }
          });
          studioGraphState.staticBatchDirty = true;
        }
      }
    }
    const updatedForm = qs("#editor-context-form");
    const selId = updatedForm ? updatedForm.getAttribute("data-context-scene-id") || "" : "";
    if (selId && typeof selectedNodeIds !== "undefined" && typeof syncNodeSelectionDOM === "function") {
      if ((typeof editorSelectionCleared === "undefined" || !editorSelectionCleared) && selectedNodeIds.size <= 1) {
        selectedNodeIds.clear();
        selectedNodeIds.add(selId);
      }
      syncNodeSelectionDOM();
      if (typeof renderStudioCanvas === "function") {
        renderStudioCanvas();
      }
    }
    return;
  }

  const prevFeed = qs("#story-scroll-feed");
  const prevScrollTop = prevFeed ? prevFeed.scrollTop : null;
  const prevEditorViewport = qs("#timeline-scroll-viewport");
  const prevEditorScrollTop = prevEditorViewport ? prevEditorViewport.scrollTop : null;
  const prevEditorScrollLeft = prevEditorViewport ? prevEditorViewport.scrollLeft : null;
  const resetCanvas = resp.headers.get("X-Reset-Canvas") === "true";
  const isAnimatedSceneTransition =
    actionName === "ChooseBranch" ||
    actionName === "AutoAdvanceImplicit" ||
    actionName === "AudioEnded";

  if (resetCanvas && typeof resetEditorCanvasViewState === "function") {
    resetEditorCanvasViewState();
  }

  const targetEl = qs(targetSelector) || qs("#app");
  if (targetEl) {
    targetEl.innerHTML = html;
    if (targetSelector === "#workspace") {
      const modalsRoot = qs("#modals-root");
      if (modalsRoot && modalsRoot.innerHTML !== "") {
        modalsRoot.innerHTML = "";
      }
    }
    syncFullscreenButtonState();
    syncTopbarToggleIcons();
    syncEditorHistoryHeaders(resp);
    const nextEditorViewport = qs("#timeline-scroll-viewport");
    if (nextEditorViewport && prevEditorScrollTop !== null) {
      nextEditorViewport.scrollTop = prevEditorScrollTop;
      nextEditorViewport.scrollLeft = prevEditorScrollLeft || 0;
    }

    const nextAudio = qs("#story-audio");
    const nextAutoChoice = nextAudio ? nextAudio.getAttribute("data-auto-choice") === "true" : false;
    const nextAudioUrl = nextAudio ? nextAudio.getAttribute("data-audio-url") || "" : "";

    let shouldAutoPlay = prevWasPlaying || actionName === "AudioEnded";
    if (actionName === "ToggleAutoChoice" && nextAutoChoice) {
      shouldAutoPlay = true;
      autoPlayChainActive = true;
    } else if (actionName === "ToggleAutoChoice" && !nextAutoChoice) {
      autoPlayChainActive = false;
    } else if (nextAutoChoice && (actionName === "ChooseBranch" || actionName === "AudioEnded" || autoPlayChainActive)) {
      shouldAutoPlay = true;
      autoPlayChainActive = true;
    }

    const preserveTime =
      keepScroll && prevAudioUrl && prevAudioUrl === nextAudioUrl ? prevAudioTime : null;

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
      if (modalTa) {
        modalTa.focus({ preventScroll: true });
      }
    }
  }
}

function postServerActionSilent(actionName, paramsOrFormData) {
  const body = new URLSearchParams();
  body.set("action", actionName);
  if (paramsOrFormData) {
    if (typeof FormData !== "undefined" && paramsOrFormData instanceof FormData) {
      paramsOrFormData.forEach((val, key) => {
        body.set(key, String(val));
      });
    } else {
      Object.keys(paramsOrFormData).forEach((k) => {
        if (paramsOrFormData[k] !== undefined && paramsOrFormData[k] !== null) {
          body.set(k, String(paramsOrFormData[k]));
        }
      });
    }
  }
  fetch("/api/action", {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      "X-Requested-With": "StoryTeller"
    },
    body: body.toString()
  })
    .then((resp) => {
      if (resp) {
        syncEditorHistoryHeaders(resp);
      }
    })
    .catch(() => {});
}

function bindDeclarativeActionBridge() {
  document.addEventListener("keydown", (e) => {
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

    if (e.key === "Escape") {
      const topbarMenu = qs("#topbar-actions-menu");
      const topbarToggle = qs("#btn-topbar-menu");
      if (topbarMenu) topbarMenu.classList.remove("open");
      if (topbarToggle) topbarToggle.classList.remove("active");
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
        if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.storytellerHost) {
          window.webkit.messageHandlers.storytellerHost.postMessage("start-window-drag");
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
      const toggleBtn = qs("#btn-topbar-menu");
      if (menu) menu.classList.remove("open");
      if (toggleBtn) toggleBtn.classList.remove("active");
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
        if (typeof updateScrollMoreCue === "function") {
          updateScrollMoreCue(qs(".scene-text-editor-feed"), qs("#modal-scroll-more-cue"));
        }
      }
      return;
    }

    if (typeof handleEditorCanvasClick === "function" && handleEditorCanvasClick(e)) {
      return;
    }

    const submitBtn = e.target.closest('form[action] button[type="submit"], form[action] input[type="submit"]');
    if (submitBtn) {
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

    const params = {};
    const isModalOnlyAction =
      actionName === "OpenSceneTextModal" ||
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
    const parentCardForm = target.closest("form.editor-context-form, form.timeline-node-card, form#form-scene-text");
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
      const contextForm = qs("#editor-context-form");
      if (contextForm && contextForm.getAttribute("data-context-scene-id") === target.dataset.scene) {
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
    const topbarToggle = qs("#btn-topbar-menu");
    if (topbarMenu) topbarMenu.classList.remove("open");
    if (topbarToggle) topbarToggle.classList.remove("active");
    sendServerAction(actionName, params);
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
  });

  if (typeof bindEditorCanvasEvents === "function") {
    bindEditorCanvasEvents();
  }

  syncTopbarToggleIcons();
  if (typeof initPlayerScrollAndAudio === "function") {
    initPlayerScrollAndAudio(null);
  }
}

document.addEventListener("DOMContentLoaded", bindDeclarativeActionBridge);
