"use strict";

const PlayerController = {
  data: {
    bookId: "",
    sensitivity: "medium",
    imagelessMode: false,
    isAutoAdvancing: false,
    overscrollAccum: 0,
    springResetTimer: null
  },

  bindings() {
    return {
      "#story-scroll-feed": { init: this.focusPresent, scroll: this.updateSceneView, wheel: this.handleSceneScroll },
      ".history-rail-tick": { click: this.focusTick },
      "#btn-preview-back-editor": { click: this.backToEditor },
      "#btn-preview-exit": { click: Dispatcher.action("ExitHistoryPreview") },
      "#btn-wasm-play, #btn-launch-wasm": { click: this.bootWasm }
    };
  },

  settings: {
    maxPullPx: 84,
    pullExp: 1.45,
    cueMarginPx: 12,
    springResetMs: 160,
    advanceLockMs: 520,
    probeRatio: 0.35,
    srcAttr: "src",
    wheelThresholds: { high: 180, medium: 320, low: 520, off: Infinity },
    selectors: {
      wasmMount: "#wasm-mount",
      appMount: "#app",
      feed: "#story-scroll-feed",
      meta: "#reader-global-meta",
      histStack: "#history-stack",
      activeStage: "#active-scene-stage",
      railTicks: "#story-history-rail .history-rail-tick",
      bgImg: "#player-bg-img",
      scrollCue: ".scroll-more-cue",
      confirmModal: "#modal-confirm"
    },
    classes: {
      overscroll: "is-overscrolling",
      active: "active",
      hidden: "hidden"
    },
    actions: {
      autoAdvance: "AutoAdvanceImplicit",
      switchView: "SwitchView"
    },
    vals: {
      editorView: "editor"
    },
    storage: {
      savePrefix: "mojra_save_",
      defaultBook: "default",
      defaultWasm: "mojra.wasm"
    },
    css: {
      translateYPrefix: "translateY(",
      pxSuffix: "px)"
    }
  },

  backToEditor() {
    Dispatcher.post(this.settings.actions.switchView, {
      view: this.settings.vals.editorView
    });
  },

  wasmSaveKey(bookId) {
    return this.settings.storage.savePrefix + (bookId || this.data.bookId || this.settings.storage.defaultBook);
  },

  saveWasmProgress(saveHex) {
    if (!saveHex || !window.localStorage) return;
    try {
      window.localStorage.setItem(this.wasmSaveKey(), saveHex);
    } catch (_) {}
  },

  async bootWasm(params) {
    const wasmSrc = (params && params.wasm_src) || this.settings.storage.defaultWasm;
    const bookId = (params && params.book_id) || this.settings.storage.defaultBook;
    this.data.bookId = bookId;
    const origFetch = window.fetch;
    const resp = await origFetch(wasmSrc);
    const buf = await resp.arrayBuffer();
    if (!window.__wasmBoot && window.Go) {
      const go = new window.Go();
      const res = await WebAssembly.instantiate(buf, go.importObject);
      go.run(res.instance);
    }
    if (window.__wasmBoot) {
      let savedHex = "";
      try {
        savedHex = (window.localStorage && window.localStorage.getItem(this.wasmSaveKey(bookId))) || "";
      } catch (_) {}
      const rawResp = window.__wasmBoot(new Uint8Array(buf), savedHex);
      window.fetch = async (url, opts) => {
        if (String(url).endsWith(Dispatcher.endpoint) && window.__wasmAction) {
          return window.__wasmAction((opts && opts.body) || "");
        }
        return origFetch(url, opts);
      };
      const mountSel = document.querySelector(this.settings.selectors.wasmMount)
        ? this.settings.selectors.wasmMount
        : this.settings.selectors.appMount;
      await Dispatcher.applyResponse(rawResp, "", params, mountSel);
    }
  },

  applySpringPull(feed, accum, activePull, thresholdOverride, direction) {
    if (!feed) return;
    const defaultThresh = this.settings.wheelThresholds.medium;
    const threshold = thresholdOverride && Number.isFinite(thresholdOverride) && thresholdOverride > 0 ? thresholdOverride : defaultThresh;
    const clamped = Math.max(0, Math.min(threshold, accum));
    const ratio = clamped / threshold;
    const pullPx = Math.round(this.settings.maxPullPx * (1 - Math.pow(1 - ratio, this.settings.pullExp)));
    feed.classList.toggle(this.settings.classes.overscroll, Boolean(activePull && pullPx > 0));
    const dir = direction < 0 ? 1 : -1;
    const transformVal = pullPx > 0 ? this.settings.css.translateYPrefix + dir * pullPx + this.settings.css.pxSuffix : "";
    const histStack = feed.querySelector(this.settings.selectors.histStack);
    const activeStage = feed.querySelector(this.settings.selectors.activeStage);
    if (histStack) histStack.style.transform = transformVal;
    if (activeStage) activeStage.style.transform = transformVal;
  },

  resetSpringPull(feed) {
    if (this.data.springResetTimer) {
      clearTimeout(this.data.springResetTimer);
      this.data.springResetTimer = null;
    }
    this.data.overscrollAccum = 0;
    this.applySpringPull(feed || document.querySelector(this.settings.selectors.feed), 0, false, this.settings.wheelThresholds.medium, 1);
  },

  scheduleSpringBack(feed) {
    if (this.data.springResetTimer) clearTimeout(this.data.springResetTimer);
    this.data.springResetTimer = setTimeout(() => {
      this.data.springResetTimer = null;
      if (!this.data.isAutoAdvancing) this.resetSpringPull(feed);
    }, this.settings.springResetMs);
  },

  stepOverscroll(feed, deltaY, thresh, dir, onReach) {
    this.data.overscrollAccum = Math.min(thresh, this.data.overscrollAccum + Math.abs(deltaY));
    this.applySpringPull(feed, this.data.overscrollAccum, true, thresh, dir);
    this.scheduleSpringBack(feed);
    if (this.data.overscrollAccum >= thresh) onReach();
  },

  triggerImplicitNextAdvance() {
    if (this.data.isAutoAdvancing || document.querySelector(this.settings.selectors.confirmModal)) return;
    const feed = document.querySelector(this.settings.selectors.feed);
    const activeStage = feed ? feed.querySelector(this.settings.selectors.activeStage) : null;
    if (!activeStage || !SceneController.hasImplicitChoice(activeStage)) return;
    this.data.isAutoAdvancing = true;
    this.data.overscrollAccum = 0;
    Dispatcher.post(this.settings.actions.autoAdvance, {}).finally(() => {
      setTimeout(() => {
        this.data.isAutoAdvancing = false;
        this.data.overscrollAccum = 0;
      }, this.settings.advanceLockMs);
    });
  },

  updateFeedScrollCue(feed) {
    if (!feed) return 0;
    const remaining = feed.scrollHeight - feed.scrollTop - feed.clientHeight;
    const scrollCue = feed.querySelector(this.settings.selectors.scrollCue);
    if (scrollCue) scrollCue.classList.toggle(this.settings.classes.hidden, !(remaining > this.settings.cueMarginPx));
    return remaining;
  },

  updateSceneView(_params, feed) {
    const targetFeed = feed || document.querySelector(this.settings.selectors.feed);
    if (!targetFeed) return;
    const remaining = this.updateFeedScrollCue(targetFeed);
    const globalMeta = targetFeed.querySelector(this.settings.selectors.meta);
    const metaH = globalMeta ? globalMeta.offsetHeight : 0;
    const atBottom = remaining <= this.settings.cueMarginPx || targetFeed.scrollHeight <= targetFeed.clientHeight;
    const probeTop = targetFeed.scrollTop + Math.round(targetFeed.clientHeight * this.settings.probeRatio);
    const focusInfo = SceneController.syncFocus(targetFeed, probeTop, atBottom, metaH);
    if (focusInfo.total === 0) return;

    const railTicks = Array.from(document.querySelectorAll(this.settings.selectors.railTicks));
    if (railTicks.length === focusInfo.total) {
      for (let idx = 0; idx < railTicks.length; idx++) {
        railTicks[idx].classList.toggle(this.settings.classes.active, idx === focusInfo.bestIdx);
      }
    }

    if (!this.data.imagelessMode && focusInfo.portraitUrl) {
      const bgImg = document.querySelector(this.settings.selectors.bgImg);
      if (bgImg) {
        if (bgImg.getAttribute(this.settings.srcAttr) !== focusInfo.portraitUrl) {
          bgImg.setAttribute(this.settings.srcAttr, focusInfo.portraitUrl);
        }
        bgImg.classList.remove(this.settings.classes.hidden);
      }
    }
  },

  focusAndScrollCard(feed, targetCard) {
    if (!feed || !targetCard) return;
    SceneController.data.manualCard = targetCard;
    this.resetSpringPull(feed);
    const globalMeta = feed.querySelector(this.settings.selectors.meta);
    const metaH = globalMeta ? globalMeta.offsetHeight : 0;
    feed.scrollTop = SceneController.cardStartTop(feed, targetCard, metaH);
    this.updateSceneView(null, feed);
  },

  focusPresent(_params, feed) {
    const targetFeed = feed || document.querySelector(this.settings.selectors.feed);
    if (!targetFeed) return;
    const cards = SceneController.cards(targetFeed);
    if (cards.length > 0) {
      this.focusAndScrollCard(targetFeed, cards[cards.length - 1]);
    }
  },

  focusTick(params) {
    const feed = document.querySelector(this.settings.selectors.feed);
    if (!feed) return;
    const cards = SceneController.cards(feed);
    if (!cards.length) return;
    const idx = params && params.idx !== undefined ? params.idx : cards.length - 1;
    const targetCard = cards[idx] || cards[cards.length - 1];
    this.focusAndScrollCard(feed, targetCard);
  },

  handleSceneScroll(_params, feed, e) {
    if (!feed || !e || e.deltaY === 0) return;
    SceneController.data.manualCard = null;
    if (e.deltaY < 0) {
      this.resetSpringPull(feed);
      return;
    }
    const maxScroll = Math.max(0, feed.scrollHeight - feed.clientHeight);
    if (feed.scrollTop < maxScroll - 1) return;
    const thresh = this.settings.wheelThresholds[this.data.sensitivity] || this.settings.wheelThresholds.medium;
    const activeStage = feed.querySelector(this.settings.selectors.activeStage);
    if (!activeStage || !SceneController.hasImplicitChoice(activeStage) || this.data.isAutoAdvancing || !Number.isFinite(thresh)) return;
    e.preventDefault();
    this.stepOverscroll(feed, e.deltaY, thresh, 1, () => this.triggerImplicitNextAdvance());
  }
};
Dispatcher.register("PlayerController", PlayerController);
