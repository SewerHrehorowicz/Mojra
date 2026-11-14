"use strict";

const EditorContextPanelController = {
  data: {
    selectedSceneId: "",
    bookVarsSpec: "",
    isScreenplay: false,
    scriptStandard: "pl",
    activeInput: null,
    activeDropdown: null,
    qIndex: -1,
    activeIdx: 0
  },

  bindings() {
    return {
      "#editor-context-panel": {
        init: this.initContextPanel
      },
      "#editor-context-form": {
        submit: Dispatcher.action("SaveScene")
      },
      "#editor-context-form input.node-inline-chapter, #editor-context-form input.node-inline-title, #context-scene-id-input, #editor-context-form input.node-slugline-input, #editor-context-form textarea[name='synopsis'], #editor-context-form textarea.node-choice-label": {
        change: Dispatcher.action("SaveScene")
      },
      "#editor-context-form textarea.node-meta-textarea, #editor-book-form textarea.node-meta-textarea": {
        input: this.autoResize
      },
      "#editor-context-form .node-chapter-select .custom-select-option": {
        click: this.selectNodeChapter
      },
      "#editor-context-form .node-image-select .custom-select-option": {
        click: this.selectNodeImage
      },
      "#editor-context-form .node-voice-select .custom-select-option": {
        click: this.selectNodeVoice
      },
      "#editor-context-form input.syn-inner-input, #editor-context-form input.choice-exit-input, #editor-context-form input.choice-rule-input": {
        change: Dispatcher.action("SaveScene"),
        input: this.onRuleInput,
        keydown: this.onRuleKeydown,
        blur: this.onRuleBlur
      },
      "#inspector-syn-autocomplete-dropdown .syn-dropdown-item": {
        pointerdown: this.chooseAutocompleteItem,
        click: this.chooseAutocompleteItem
      },
      "#editor-context-form .slugline-pill-btn": {
        click: Dispatcher.action("ApplySluglineToken")
      },
      "#editor-context-form .node-media-add-btn[data-kind]": {
        click: Dispatcher.action("BrowseAddBookMedia")
      },
      "#editor-context-form .voice-rec-section-btn": {
        click: Dispatcher.action("SelectSceneVoiceSection")
      },
      "#btn-inspector-voice-recorder": {
        click: Dispatcher.action("ToggleSceneVoiceRecorder")
      },
      "#editor-context-form .btn-open-scene-text": {
        click: Dispatcher.action("OpenSceneTextModal")
      },
      "#editor-context-form .btn-play-from-scene": {
        click: Dispatcher.action("PlayFromScene")
      },
      "#editor-context-form .node-choice-remove": {
        click: Dispatcher.action("PromptRemoveChoice")
      },
      "#editor-context-form .node-add-choice-btn": {
        click: Dispatcher.action("AddChoice")
      },
      "#editor-context-form .btn-prompt-delete-scene": {
        click: Dispatcher.action("PromptDeleteScene")
      },
      "#editor-chapter-form": {
        submit: Dispatcher.action("SaveChapterSettings")
      },
      "#editor-chapter-form .chapter-group-title-input": {
        change: Dispatcher.action("RenameChapter")
      },
      "#editor-chapter-form .chapter-start-scene-select .custom-select-option": {
        click: this.selectChapterStartScene
      },
      ".btn-close-book-settings": {
        click: Dispatcher.action("CloseBookSettings")
      },
      "#editor-book-form": {
        submit: Dispatcher.action("SaveBookSettings")
      },
      "#book-settings-title-input": {
        change: Dispatcher.action("RenameBook")
      },
      "#editor-book-form .form-radio-option": {
        click: Dispatcher.action("SaveBookSettings")
      },
      "#editor-book-form input.form-radio-input, #editor-book-form input[name='author'], #book-settings-license-input, #editor-book-form textarea[name='description'], #editor-book-form input[name='export_dir']": {
        change: Dispatcher.action("SaveBookSettings")
      },
      "#book-start-chapter-select .custom-select-option": {
        click: this.selectBookStartChapter
      },
      "#book-start-scene-select .custom-select-option": {
        click: this.selectBookStartScene
      },
      "#editor-book-form .btn-toggle-book-narrated": {
        click: Dispatcher.action("ToggleBookNarrated")
      },
      "#editor-book-form .btn-toggle-book-imageless": {
        click: Dispatcher.action("ToggleBookImageless")
      },
      "#editor-book-form .btn-browse-book-save-folder": {
        click: Dispatcher.action("BrowseBookSaveFolder")
      },
      "#editor-book-form .btn-browse-book-export-folder": {
        click: Dispatcher.action("BrowseBookExportFolder")
      },
      "#btn-delete-selected-scenes": {
        click: Dispatcher.action("PromptDeleteScenes")
      }
    };
  },

  settings: {
    minTextareaHeightPx: 24,
    blurCloseDelayMs: 120,
    dropdownOffsetTopPx: 2,
    dropdownMinLeftPx: 8,
    styleHeightAuto: "auto",
    styleUnitPx: "px",
    changeEvent: "change",
    targetKind: "target",
    varKind: "var",
    leadingSpaceBeforeRe: /[\s(;!]$/,
    invalidFilterCharRe: /[\s=<>!;)]/,
    wordTokenRe: /[a-zA-Z_\u0080-\uffff][a-zA-Z0-9_\-\u0080-\uffff]*/g,
    proseDirectiveRe: /^(?:if|elif|set):/i,
    reservedWords: new Set(["and", "or", "not", "true", "false"]),
    selectors: {
      viewport: "#timeline-scroll-viewport",
      inspectorDropdown: "#inspector-syn-autocomplete-dropdown",
      dropdownItem: ".syn-dropdown-item",
      visibleItem: ".syn-dropdown-item:not(.hidden)"
    },
    classes: {
      choiceExitInput: "choice-exit-input",
      hidden: "hidden",
      active: "is-active"
    },
    attrs: {
      synKind: "data-syn-kind",
      synVarName: "data-syn-var-name"
    },
    keys: {
      arrowDown: "ArrowDown",
      arrowUp: "ArrowUp",
      enter: "Enter",
      tab: "Tab",
      escape: "Escape"
    },
    selectSceneAction: "SelectStudioScene",
    actions: {
      saveScene: "SaveScene",
      setChapterStartScene: "SetChapterStartScene",
      setBookStartChapter: "SetBookStartChapter",
      setStartScene: "SetStartScene"
    }
  },

  selectNodeChapter(p) {
    Dispatcher.post(this.settings.actions.saveScene, Object.assign({}, p, { chapter: p.select_value || "" }));
  },

  selectNodeImage(p) {
    Dispatcher.post(this.settings.actions.saveScene, Object.assign({}, p, { explicit_image: p.select_value || "" }));
  },

  selectNodeVoice(p) {
    Dispatcher.post(this.settings.actions.saveScene, Object.assign({}, p, { explicit_voice: p.select_value || "" }));
  },

  selectChapterStartScene(p) {
    Dispatcher.post(this.settings.actions.setChapterStartScene, Object.assign({}, p, { start_scene: p.select_value || "" }));
  },

  selectBookStartChapter(p) {
    Dispatcher.post(this.settings.actions.setBookStartChapter, Object.assign({}, p, { start_chapter: p.select_value || "" }));
  },

  selectBookStartScene(p) {
    const val = p.select_value || "";
    Dispatcher.post(this.settings.actions.setStartScene, Object.assign({}, p, { scene: val, start_scene: val }));
  },

  initContextPanel() {
    const s = this.settings;
    if (Dispatcher.lastAction === s.selectSceneAction && Dispatcher.lastParams && Dispatcher.lastParams.scene) {
      CanvasModel.editorSelectionCleared = false;
      CanvasModel.selectedNodeIds.clear();
      CanvasModel.selectedNodeIds.add(Dispatcher.lastParams.scene);
    }
    if (document.querySelector(s.selectors.viewport)) {
      EditorWebglScene.render();
    }
  },

  autoResize(_params, element) {
    if (!element) return;
    const s = this.settings;
    element.style.height = s.styleHeightAuto;
    element.style.height = Math.max(s.minTextareaHeightPx, element.scrollHeight || 0) + s.styleUnitPx;
  },

  extractUsedVarNames(text, excludeStart, excludeEnd) {
    const s = this.settings;
    const stripped = String(text || "").slice(0, excludeStart) + " " + String(text || "").slice(excludeEnd);
    const used = new Set();
    const matches = stripped.match(s.wordTokenRe) || [];
    for (let i = 0; i < matches.length; i++) {
      const w = matches[i].toLowerCase();
      if (!s.reservedWords.has(w)) used.add(w);
    }
    return used;
  },

  visibleItems(drop) {
    return drop ? Array.from(drop.querySelectorAll(this.settings.selectors.visibleItem)) : [];
  },

  syncActiveItem(drop) {
    const items = this.visibleItems(drop);
    if (this.data.activeIdx >= items.length) this.data.activeIdx = 0;
    if (this.data.activeIdx < 0) this.data.activeIdx = Math.max(0, items.length - 1);
    items.forEach((btn, idx) => btn.classList.toggle(this.settings.classes.active, idx === this.data.activeIdx));
  },

  closeDropdown() {
    if (this.data.activeDropdown) this.data.activeDropdown.classList.add(this.settings.classes.hidden);
    this.data.activeInput = null;
    this.data.activeDropdown = null;
    this.data.qIndex = -1;
    this.data.activeIdx = 0;
  },

  applyAutocompleteItem(btn) {
    if (!btn || !this.data.activeInput || this.data.qIndex < 0) return;
    const s = this.settings;
    const varName = btn.getAttribute(s.attrs.synVarName) || "";
    if (!varName) return;
    const el = this.data.activeInput;
    const val = el.value || "";

    if (el.classList && el.classList.contains(s.classes.choiceExitInput)) {
      el.value = varName;
    } else {
      const caret = el.selectionStart != null ? el.selectionStart : this.data.qIndex + 1;
      const before = val.slice(0, this.data.qIndex);
      const after = val.slice(Math.max(this.data.qIndex + 1, caret));
      const prefix = before.length > 0 && !s.leadingSpaceBeforeRe.test(before) ? before + " " : before;
      el.value = prefix + varName + after;
    }
    const newPos = el.value.length;
    this.closeDropdown();
    el.focus();
    if (el.setSelectionRange) el.setSelectionRange(newPos, newPos);
    el.dispatchEvent(new Event(s.changeEvent, { bubbles: true }));
  },

  onRuleInput(_params, el, _e, dropdownOverride) {
    if (!el) return;
    const s = this.settings;
    const drop = dropdownOverride || document.querySelector(s.selectors.inspectorDropdown);
    if (!drop) return;
    const val = el.value || "";
    const caret = el.selectionStart != null ? el.selectionStart : val.length;
    const beforeCaret = val.slice(0, caret);
    const lastQ = beforeCaret.lastIndexOf("?");
    if (lastQ < 0) {
      this.closeDropdown();
      return;
    }
    const filterRaw = beforeCaret.slice(lastQ + 1);
    if (s.invalidFilterCharRe.test(filterRaw)) {
      this.closeDropdown();
      return;
    }
    if (el.rows !== undefined) {
      const lineStart = beforeCaret.lastIndexOf("\n") + 1;
      const lineBeforeQ = beforeCaret.slice(lineStart, lastQ).trim();
      if (!s.proseDirectiveRe.test(lineBeforeQ)) {
        this.closeDropdown();
        return;
      }
    }

    const filter = filterRaw.toLowerCase();
    const allItems = Array.from(drop.querySelectorAll(s.selectors.dropdownItem));
    const isTargetInput = Boolean(el.classList && el.classList.contains(s.classes.choiceExitInput));
    let visibleCount = 0;

    if (isTargetInput) {
      const existingVal = (val.slice(0, lastQ) + val.slice(caret)).trim().toLowerCase();
      allItems.forEach((btn) => {
        const vName = (btn.getAttribute(s.attrs.synVarName) || "").toLowerCase();
        const match = btn.getAttribute(s.attrs.synKind) === s.targetKind && vName !== existingVal && (!filter || vName.includes(filter));
        btn.classList.toggle(s.classes.hidden, !match);
        if (match) visibleCount++;
      });
    } else {
      const usedVars = this.extractUsedVarNames(val, lastQ, caret);
      allItems.forEach((btn) => {
        const synKind = btn.getAttribute(s.attrs.synKind) || s.varKind;
        const vName = (btn.getAttribute(s.attrs.synVarName) || "").toLowerCase();
        const match = synKind === s.varKind && !usedVars.has(vName) && (!filter || vName.includes(filter));
        btn.classList.toggle(s.classes.hidden, !match);
        if (match) visibleCount++;
      });
    }

    if (visibleCount === 0) {
      this.closeDropdown();
      return;
    }

    this.data.activeInput = el;
    this.data.activeDropdown = drop;
    this.data.qIndex = lastQ;
    this.data.activeIdx = 0;
    this.syncActiveItem(drop);

    const rect = el.getBoundingClientRect();
    const parent = drop.offsetParent || document.body;
    const pRect = parent.getBoundingClientRect();
    drop.style.top = Math.round(rect.bottom - pRect.top + s.dropdownOffsetTopPx) + s.styleUnitPx;
    drop.style.left = Math.round(Math.max(s.dropdownMinLeftPx, rect.left - pRect.left)) + s.styleUnitPx;
    drop.classList.remove(s.classes.hidden);
  },

  onRuleKeydown(_params, el, e) {
    const s = this.settings;
    if (!this.data.activeDropdown || this.data.activeDropdown.classList.contains(s.classes.hidden) || this.data.activeInput !== el) {
      if (e && e.key === s.keys.escape) {
        AppShellController.onKeydown(_params, el, e);
      }
      return;
    }
    const items = this.visibleItems(this.data.activeDropdown);
    if (!items.length) return;
    if (e.key === s.keys.arrowDown) {
      e.preventDefault();
      e.stopPropagation();
      this.data.activeIdx = (this.data.activeIdx + 1) % items.length;
      this.syncActiveItem(this.data.activeDropdown);
    } else if (e.key === s.keys.arrowUp) {
      e.preventDefault();
      e.stopPropagation();
      this.data.activeIdx = (this.data.activeIdx - 1 + items.length) % items.length;
      this.syncActiveItem(this.data.activeDropdown);
    } else if (e.key === s.keys.enter || e.key === s.keys.tab) {
      e.preventDefault();
      e.stopPropagation();
      this.applyAutocompleteItem(items[this.data.activeIdx] || items[0]);
    } else if (e.key === s.keys.escape) {
      e.preventDefault();
      e.stopPropagation();
      this.closeDropdown();
    }
  },

  onRuleBlur(_params, el) {
    setTimeout(() => {
      if (this.data.activeInput === el) this.closeDropdown();
    }, this.settings.blurCloseDelayMs);
  },

  chooseAutocompleteItem(_params, el, e) {
    if (e) e.preventDefault();
    this.applyAutocompleteItem(el);
  }
};
Dispatcher.register("EditorContextPanelController", EditorContextPanelController);
