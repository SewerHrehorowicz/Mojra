"use strict";

const AppShellController = {
  data: {
    theme: "dark",
    uiScale: "small",
    imageless: false,
    imagelessLocked: false,
    zenMode: false,
    sensitivity: "medium",
    deviceMode: "full",
    lang: "pl",
    toast: "",
    nativeCmd: "",
    keepScroll: false,
    resetCanvas: false,
    voicePeak: "",
    saveHex: "",
    sceneId: "",
    toastTimer: null,
    activeTooltipTarget: null
  },

  bindings() {
    return {
      "body": {
        init: this.initShell,
        keydown: this.onKeydown,
        mouseover: this.onTooltipOver,
        mouseout: this.onTooltipOut,
        mousemove: this.onTooltipMove,
        mousedown: this.hideTooltip,
        wheel: this.hideTooltip
      },
      // @dev-only-start
      ".preview-mode-btn": {
        click: Dispatcher.action("SetDeviceMode")
      }
      // @dev-only-end
    };
  },

  settings: {
    toastDurationMs: 2600,
    tooltipGapPx: 6,
    tooltipEdgePadPx: 8,
    half: 0.5,
    fallbackVw: 1280,
    fallbackVh: 800,
    pxSuffix: "px",
    editableSelector: "input, textarea, select, [contenteditable='true']",
    selectors: {
      toast: "#toast-banner",
      tooltip: "#ui-tooltip",
      hintTarget: "[data-hint]",
      imagelessBtn: "#btn-toggle-imageless",
      zenBtn: "#btn-toggle-zen",
      modalBackdrops: "#modals-root .modal-backdrop",
      themeMoon: "#icon-theme-moon",
      themeSun: "#icon-theme-sun",
      imgOn: "#icon-imageless-on",
      imgOff: "#icon-imageless-off",
      fsEnter: "#icon-fullscreen-enter",
      fsExit: "#icon-fullscreen-exit"
      // @dev-only-start
      , deviceStage: "#device-stage"
      // @dev-only-end
    },
    attrs: {
      theme: "data-theme",
      uiScale: "data-ui-scale",
      imageless: "data-imageless",
      imagelessLocked: "data-imageless-locked",
      zen: "data-zen",
      scrollSens: "data-scroll-sensitivity",
      lang: "data-lang",
      hint: "data-hint"
      // @dev-only-start
      , preview: "data-preview"
      // @dev-only-end
    },
    classes: {
      hidden: "hidden",
      visible: "is-visible",
      active: "active"
    },
    vals: {
      light: "light",
      trueStr: "true",
      falseStr: "false",
      none: "none",
      defaultLang: "pl",
      toggleFs: "toggle_fullscreen",
      quit: "quit",
      clickEv: "click",
      fnType: "function",
      undefType: "undefined"
    },
    i18n: {
      imagelessLockedHint: "imageless_locked_hint",
      imagelessOffHint: "imageless_enable_hint"
    },
    keys: {
      escape: "Escape",
      z: "z",
      y: "y",
      f12: "F12"
    },
    devToolsAllowed: false
    // @dev-only-start
    || true
    // @dev-only-end
    ,
    historyActions: {
      undo: "UndoEditor",
      redo: "RedoEditor"
    }
  },

  initShell() {
    const b = document.body;
    if (!b) return;
    const a = this.settings.attrs;
    const v = this.settings.vals;
    this.data.theme = b.getAttribute(a.theme) || this.data.theme;
    this.data.uiScale = b.getAttribute(a.uiScale) || this.data.uiScale;
    this.data.imageless = b.getAttribute(a.imageless) === v.trueStr;
    this.data.imagelessLocked = b.getAttribute(a.imagelessLocked) === v.trueStr;
    this.data.zenMode = b.getAttribute(a.zen) === v.trueStr;
    this.data.lang = b.getAttribute(a.lang) || v.defaultLang;
  },

  activeLang() {
    const b = document.body;
    return (b && b.getAttribute(this.settings.attrs.lang)) || this.data.lang || this.settings.vals.defaultLang;
  },

  tr(key) {
    if (!window.mojraI18n) return key;
    const lang = this.activeLang();
    const dict = window.mojraI18n[lang] || window.mojraI18n[this.settings.vals.defaultLang];
    return (dict && dict[key]) || key;
  },

  showToast(msg) {
    const s = this.settings;
    const toast = document.querySelector(s.selectors.toast);
    if (!toast || !msg) return;
    let decoded = msg;
    try {
      decoded = decodeURIComponent(msg);
    } catch (_) {}
    toast.textContent = decoded;
    toast.classList.remove(s.classes.hidden);
    toast.classList.add(s.classes.visible);
    if (this.data.toastTimer) clearTimeout(this.data.toastTimer);
    this.data.toastTimer = setTimeout(() => {
      toast.classList.remove(s.classes.visible);
      toast.classList.add(s.classes.hidden);
    }, s.toastDurationMs);
  },

  hideTooltip() {
    this.data.activeTooltipTarget = null;
    const tip = document.querySelector(this.settings.selectors.tooltip);
    if (!tip) return;
    tip.classList.remove(this.settings.classes.visible);
  },

  setHint(el, text) {
    if (!el) return;
    const s = this.settings;
    const val = String(text || "").trim();
    if (val) {
      el.setAttribute(s.attrs.hint, val);
      if (this.data.activeTooltipTarget === el) {
        const tip = document.querySelector(s.selectors.tooltip);
        if (tip) tip.textContent = val;
      }
    } else {
      el.removeAttribute(s.attrs.hint);
      if (this.data.activeTooltipTarget === el) this.hideTooltip();
    }
  },

  showTooltip(targetEl) {
    const s = this.settings;
    const tip = document.querySelector(s.selectors.tooltip);
    if (!tip || !targetEl) return;
    const text = (targetEl.getAttribute(s.attrs.hint) || "").trim();
    if (!text) {
      this.hideTooltip();
      return;
    }
    this.data.activeTooltipTarget = targetEl;
    tip.textContent = text;
    tip.classList.add(s.classes.visible);

    const rect = targetEl.getBoundingClientRect();
    const tipRect = tip.getBoundingClientRect();
    const vw = window.innerWidth || document.documentElement.clientWidth || s.fallbackVw;
    const vh = window.innerHeight || document.documentElement.clientHeight || s.fallbackVh;
    const gap = s.tooltipGapPx;
    const edgePad = s.tooltipEdgePadPx;

    let top = rect.bottom + gap;
    if (top + tipRect.height > vh - edgePad && rect.top - gap - tipRect.height >= edgePad) {
      top = rect.top - gap - tipRect.height;
    }
    let left = rect.left + rect.width * s.half - tipRect.width * s.half;
    if (left < edgePad) left = edgePad;
    if (left + tipRect.width > vw - edgePad) {
      left = Math.max(edgePad, vw - edgePad - tipRect.width);
    }
    tip.style.left = Math.round(left) + s.pxSuffix;
    tip.style.top = Math.round(top) + s.pxSuffix;
  },

  onTooltipOver(_params, _el, e) {
    const target = e && e.target && e.target.closest ? e.target.closest(this.settings.selectors.hintTarget) : null;
    if (target) this.showTooltip(target);
    else this.hideTooltip();
  },

  onTooltipOut(_params, _el, e) {
    if (!this.data.activeTooltipTarget) return;
    const rel = e && e.relatedTarget;
    if (rel && this.data.activeTooltipTarget.contains && this.data.activeTooltipTarget.contains(rel)) return;
    this.hideTooltip();
  },

  onTooltipMove(_params, _el, e) {
    if (!this.data.activeTooltipTarget) return;
    const t = e && e.target;
    if (t && this.data.activeTooltipTarget.contains && !this.data.activeTooltipTarget.contains(t)) {
      this.hideTooltip();
    }
  },

  onKeydown(_params, _el, e) {
    if (!e) return;
    const s = this.settings;
    if (!s.devToolsAllowed && e.key === s.keys.f12) {
      e.preventDefault();
      return;
    }
    const target = e.target;
    const isEditable = Boolean(target && target.closest && target.closest(s.editableSelector));
    if (!isEditable && (e.metaKey || e.ctrlKey)) {
      const k = (e.key || "").toLowerCase();
      if (k === s.keys.z) {
        e.preventDefault();
        Dispatcher.post(e.shiftKey ? s.historyActions.redo : s.historyActions.undo, {});
        return;
      }
      if (k === s.keys.y) {
        e.preventDefault();
        Dispatcher.post(s.historyActions.redo, {});
        return;
      }
    }
    if (e.key === s.keys.escape) {
      const backdrops = document.querySelectorAll(s.selectors.modalBackdrops);
      if (backdrops.length > 0) {
        const bd = backdrops[backdrops.length - 1];
        if (typeof bd.click === s.vals.fnType) bd.click();
        else if (bd.dispatchEvent) bd.dispatchEvent(new Event(s.vals.clickEv, { bubbles: true }));
      }
    }
  },

  execNativeCommand(cmd) {
    const s = this.settings;
    const v = s.vals;
    if (cmd === v.toggleFs) {
      const docEl = document.documentElement;
      const enterIcon = document.querySelector(s.selectors.fsEnter);
      const exitIcon = document.querySelector(s.selectors.fsExit);
      if (!document.fullscreenElement && docEl && docEl.requestFullscreen) {
        docEl.requestFullscreen().catch(() => {});
        if (enterIcon) enterIcon.classList.add(s.classes.hidden);
        if (exitIcon) exitIcon.classList.remove(s.classes.hidden);
      } else if (document.exitFullscreen) {
        document.exitFullscreen().catch(() => {});
        if (enterIcon) enterIcon.classList.remove(s.classes.hidden);
        if (exitIcon) exitIcon.classList.add(s.classes.hidden);
      }
    } else if (cmd === v.quit && window.close) {
      window.close();
    }
  },

  onHydrate(decoded, _action, params) {
    if (!decoded) return;
    const s = this.settings;
    const a = s.attrs;
    const v = s.vals;
    const b = typeof document !== v.undefType ? document.body : null;

    if (decoded.target && decoded.target !== v.none) {
      this.hideTooltip();
    }
    if (decoded.resetCanvas === v.trueStr && typeof CanvasModel !== v.undefType) {
      CanvasModel.editorCanvasState.initialized = false;
    }

    const newSceneId = decoded.sceneId || "";
    const oldSceneId = (params && (params.scene_id || params.scene)) || "";
    if (newSceneId && oldSceneId && newSceneId !== oldSceneId && typeof CanvasModel !== v.undefType) {
      const gState = CanvasModel.studioGraphState;
      const nd = gState.nodeById.get(oldSceneId);
      if (nd) {
        gState.nodeById.delete(oldSceneId);
        nd.id = newSceneId;
        gState.nodeById.set(newSceneId, nd);
      }
      if (CanvasModel.selectedNodeIds.has(oldSceneId)) {
        CanvasModel.selectedNodeIds.delete(oldSceneId);
        CanvasModel.selectedNodeIds.add(newSceneId);
      }
    }

    if (decoded.theme && b) {
      b.setAttribute(a.theme, this.data.theme);
      const isLight = this.data.theme === v.light;
      const moon = document.querySelector(s.selectors.themeMoon);
      const sun = document.querySelector(s.selectors.themeSun);
      if (moon) moon.classList.toggle(s.classes.hidden, isLight);
      if (sun) sun.classList.toggle(s.classes.hidden, !isLight);
    }

    if (decoded.lang && b) {
      b.setAttribute(a.lang, this.data.lang);
    }

    if (decoded.uiScale && b) {
      b.setAttribute(a.uiScale, this.data.uiScale);
    }

    if (decoded.imageless !== undefined && b) {
      if (typeof PlayerController !== v.undefType && PlayerController.data) {
        PlayerController.data.imagelessMode = this.data.imageless;
      }
      b.setAttribute(a.imageless, this.data.imageless ? v.trueStr : v.falseStr);
      const imgBtn = document.querySelector(s.selectors.imagelessBtn);
      if (imgBtn) imgBtn.classList.toggle(s.classes.active, !this.data.imageless);
      const onIcon = document.querySelector(s.selectors.imgOn);
      const offIcon = document.querySelector(s.selectors.imgOff);
      if (onIcon) onIcon.classList.toggle(s.classes.hidden, !this.data.imageless);
      if (offIcon) offIcon.classList.toggle(s.classes.hidden, this.data.imageless);
    }

    if (decoded.imagelessLocked !== undefined && b) {
      b.setAttribute(a.imagelessLocked, this.data.imagelessLocked ? v.trueStr : v.falseStr);
      const imgBtn = document.querySelector(s.selectors.imagelessBtn);
      if (imgBtn) {
        imgBtn.disabled = this.data.imagelessLocked;
      }
    }

    if ((decoded.imageless !== undefined || decoded.imagelessLocked !== undefined) && b) {
      const imgBtn = document.querySelector(s.selectors.imagelessBtn);
      if (imgBtn) {
        const hintKey = this.data.imagelessLocked
          ? s.i18n.imagelessLockedHint
          : s.i18n.imagelessOffHint;
        this.setHint(imgBtn, this.tr(hintKey));
      }
    }

    if (decoded.zenMode !== undefined && b) {
      b.setAttribute(a.zen, this.data.zenMode ? v.trueStr : v.falseStr);
      const zenBtn = document.querySelector(s.selectors.zenBtn);
      if (zenBtn) zenBtn.classList.toggle(s.classes.active, this.data.zenMode);
    }

    if (decoded.sensitivity && b) {
      if (typeof PlayerController !== v.undefType && PlayerController.data) {
        PlayerController.data.sensitivity = this.data.sensitivity;
      }
      b.setAttribute(a.scrollSens, this.data.sensitivity);
    }

    // @dev-only-start
    if (decoded.deviceMode) {
      const stage = document.querySelector(s.selectors.deviceStage);
      if (stage) stage.setAttribute(a.preview, this.data.deviceMode);
    }
    // @dev-only-end

    if (decoded.toast) this.showToast(decoded.toast);

    if (decoded.nativeCmd) {
      if (window.mojraNative && typeof window.mojraNative.postMessage === v.fnType) {
        window.mojraNative.postMessage(decoded.nativeCmd);
      }
      this.execNativeCommand(decoded.nativeCmd);
    }

    if (decoded.voicePeak !== undefined && typeof EditorVoice !== v.undefType) {
      EditorVoice.handlePeakHeader(decoded.voicePeak);
    }

    if (decoded.saveHex && typeof PlayerController !== v.undefType) {
      PlayerController.saveWasmProgress(decoded.saveHex);
    }
  }
};
Dispatcher.register("AppShellController", AppShellController);
