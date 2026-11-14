"use strict";

const SceneController = {
  data: {
    isPresent: true,
    isScreenplayMode: false,
    hasTitle: false,
    hasImage: false,
    imageUrl: "",
    isTerminal: false,
    isBranching: false,
    focusedIdx: 0,
    manualCard: null
  },

  bindings() {
    return {
      "article.scene-card button.contextual-rewind-btn": { click: this.promptRewindStep },
      "article.scene-card button.choice-btn": { click: this.chooseBranch },
      "article.scene-card button.choice-reset-btn": { click: Dispatcher.action("PromptResetProgress") }
    };
  },

  settings: {
    srcAttr: "src",
    selectors: {
      card: "article.scene-card",
      portraitImg: ".scene-portrait-img",
      choiceBtns: ".choice-btn:not(.choice-locked)"
    },
    classes: {
      focused: "scene-focused",
      dimmed: "scene-dimmed"
    },
    actions: {
      promptRewind: "PromptRewindStep",
      chooseBranch: "ChooseBranch"
    }
  },

  promptRewindStep(p) {
    Dispatcher.post(this.settings.actions.promptRewind, { step: p.idx });
  },

  chooseBranch(p) {
    Dispatcher.post(this.settings.actions.chooseBranch, { choice_idx: p.idx });
  },

  cards(root) {
    const scope = root || document;
    return scope.querySelectorAll ? Array.from(scope.querySelectorAll(this.settings.selectors.card)) : [];
  },

  cardStartTop(feed, card, metaH) {
    if (!feed || !card) return 0;
    let layoutTop = 0;
    let el = card;
    while (el && el !== feed && feed.contains(el)) {
      layoutTop += el.offsetTop;
      el = el.offsetParent;
    }
    const rawTop = layoutTop - Math.max(0, (metaH || 0) - 1);
    return Math.max(0, Math.round(rawTop));
  },

  hasImplicitChoice(root) {
    const scope = root || document;
    const list = scope.querySelectorAll ? scope.querySelectorAll(this.settings.selectors.choiceBtns) : [];
    return list.length === 1;
  },

  syncFocus(feed, probeTop, atBottom, metaH) {
    const cards = this.cards(feed);
    if (cards.length === 0) {
      return { bestIdx: -1, total: 0, portraitUrl: "" };
    }
    let bestIdx = -1;
    if (this.data.manualCard && feed && feed.contains(this.data.manualCard)) {
      bestIdx = cards.indexOf(this.data.manualCard);
    }
    if (bestIdx < 0) {
      if (atBottom) {
        bestIdx = cards.length - 1;
      } else {
        bestIdx = 0;
        for (let i = 0; i < cards.length; i++) {
          if (this.cardStartTop(feed, cards[i], metaH) <= probeTop) {
            bestIdx = i;
          }
        }
      }
    }
    this.data.focusedIdx = bestIdx;
    for (let i = 0; i < cards.length; i++) {
      const isFocused = i === bestIdx;
      cards[i].classList.toggle(this.settings.classes.focused, isFocused);
      cards[i].classList.toggle(this.settings.classes.dimmed, !isFocused);
    }
    let portraitUrl = "";
    for (let i = 0; i <= bestIdx; i++) {
      const portrait = cards[i].querySelector(this.settings.selectors.portraitImg);
      if (portrait) {
        const src = portrait.getAttribute(this.settings.srcAttr);
        if (src) portraitUrl = src;
      }
    }
    return { bestIdx: this.data.focusedIdx, total: cards.length, portraitUrl: portraitUrl };
  }
};
Dispatcher.register("SceneController", SceneController);
