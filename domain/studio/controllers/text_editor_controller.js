"use strict";

const TextEditorController = {
  data: {
    sceneId: "",
    bookVarsSpec: "",
    isScreenplay: false,
    scriptStandard: "pl",
    voiceOpen: false,
    voiceSection: "01_static",
    voiceUrl: "",
    voicePeaks: "",
    voiceTrimStart: 0,
    voiceTrimEnd: 1,
    activeInput: null,
    activeDropdown: null,
    qIndex: -1,
    activeIdx: 0
  },

  bindings() {
    return {
      "#modal-scene-textarea": {
        init: this.initTextarea,
        change: Dispatcher.action("AutoSaveScene"),
        input: this.onTextareaInput,
        keydown: this.onRuleKeydown,
        blur: this.onRuleBlur
      },
      "#text-editor-view": {
        click: this.onModalBackdrop
      },
      "#text-editor-view .btn-play-from-scene": {
        click: Dispatcher.action("PlayFromScene")
      },
      "#text-editor-view .btn-prompt-delete-scene": {
        click: Dispatcher.action("PromptDeleteScene")
      },
      "#btn-collapse-scene-text": {
        click: Dispatcher.action("CloseSceneTextModal")
      },
      "#btn-expand-scene-text-full": {
        click: Dispatcher.action("SetSceneTextWidthMode")
      },
      "#form-scene-text": {
        submit: Dispatcher.action("SaveSceneTextModal")
      },
      "#form-scene-text .incoming-choice-btn, #form-scene-text .modal-choice-nav-btn": {
        click: Dispatcher.action("NavigateSceneTextModal")
      },
      "#form-scene-text input.scene-title": {
        input: this.onSceneTitleInput,
        change: Dispatcher.action("AutoSaveScene")
      },
      "#form-scene-text textarea.choice-label-text": {
        input: this.onChoiceLabelInput,
        change: Dispatcher.action("AutoSaveScene")
      },
      "#form-scene-text input.node-inline-chapter, #form-scene-text input[name='explicit_id'], #modal-slugline-input, #form-scene-text textarea[name='synopsis']": {
        change: Dispatcher.action("AutoSaveScene")
      },
      "#form-scene-text textarea.node-meta-textarea": {
        input: this.autoResizeMeta
      },
      "#btn-scene-preview-output": {
        click: Dispatcher.action("SetSceneTextPreviewMode")
      },
      "#btn-scene-test-vars, #scene-test-vars-bar .btn-close-test-vars": {
        click: Dispatcher.action("ToggleSceneTestVarsPopover")
      },
      "#btn-modal-voice-recorder, #modal-scene-settings-block .voice-rec-toggle-btn": {
        click: Dispatcher.action("ToggleSceneVoiceRecorder")
      },
      "#form-scene-text .scene-settings-toggle-btn": {
        click: Dispatcher.action("ToggleSceneTextSettings")
      },
      "#scene-test-vars-bar .test-var-bool-btn": {
        click: Dispatcher.action("UpdateSceneTextTestVar")
      },
      "#scene-test-vars-bar .test-var-int-input": {
        change: this.updateTestIntVar
      },
      "#scene-test-vars-bar .test-var-enum-select .custom-select-option": {
        click: this.updateTestEnumVar
      },
      "#modal-scene-settings-block .node-chapter-select .custom-select-option": {
        click: this.selectModalChapter
      },
      "#modal-scene-settings-block .modal-image-select .custom-select-option": {
        click: this.selectModalImage
      },
      "#modal-scene-settings-block .modal-voice-select .custom-select-option": {
        click: this.selectModalVoice
      },
      "#modal-scene-settings-block .node-media-add-btn[data-kind]": {
        click: Dispatcher.action("BrowseAddBookMedia")
      },
      "#form-scene-text input.syn-inner-input, #form-scene-text input.choice-exit-input, #form-scene-text input.choice-rule-input": {
        change: Dispatcher.action("AutoSaveScene"),
        input: this.onRuleInput,
        keydown: this.onRuleKeydown,
        blur: this.onRuleBlur
      },
      "#modal-syn-autocomplete-dropdown .syn-dropdown-item": {
        pointerdown: this.chooseAutocompleteItem,
        click: this.chooseAutocompleteItem
      },
      "#modal-voice-rec-sections .voice-rec-section-btn": {
        click: Dispatcher.action("SelectSceneVoiceSection")
      },
      "#form-scene-text .slugline-pill-btn:not(.script-format-pill-btn):not(.script-char-pill-btn)": {
        click: Dispatcher.action("ApplySluglineToken")
      },
      "#form-scene-text .script-format-pill-btn": {
        click: this.insertScriptFormatBlock
      },
      "#form-scene-text .script-char-pill-btn": {
        click: this.insertScriptCharBlock
      },
      "#form-scene-text .node-choice-remove": {
        click: Dispatcher.action("PromptRemoveChoice")
      },
      "#form-scene-text .modal-add-choice-btn": {
        click: Dispatcher.action("AddChoice")
      }
    };
  },

  settings: {
    minHeightPx: 240,
    minMetaHeightPx: 24,
    heightPaddingPx: 12,
    blurCloseDelayMs: 120,
    dropdownOffsetTopPx: 2,
    dropdownMinLeftPx: 8,
    heightAuto: "auto",
    pxUnit: "px",
    changeEvent: "change",
    targetKind: "target",
    varKind: "var",
    varNameAttr: "data-var-name",
    leadingSpaceBeforeRe: /[\s(;!]$/,
    invalidFilterCharRe: /[\s=<>!;)]/,
    wordTokenRe: /[a-zA-Z_\u0080-\uffff][a-zA-Z0-9_\-\u0080-\uffff]*/g,
    proseDirectiveRe: /^(?:if|elif|set):/i,
    reservedWords: new Set(["and", "or", "not", "true", "false"]),
    selectors: {
      textTextarea: "#modal-scene-textarea",
      sceneIdInput: '#form-scene-text input[name="scene_id"]',
      modalDropdown: "#modal-syn-autocomplete-dropdown",
      dropdownItem: ".syn-dropdown-item",
      visibleItem: ".syn-dropdown-item:not(.hidden)",
      enumSelectRoot: ".test-var-enum-select"
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
    actions: {
      closeSceneText: "CloseSceneTextModal",
      autoSaveScene: "AutoSaveScene",
      updateTestVar: "UpdateSceneTextTestVar",
      insertScriptBlock: "InsertScreenplayBlock"
    }
  },

  onModalBackdrop(_params, element, event) {
    if (event && event.target === element) {
      Dispatcher.post(this.settings.actions.closeSceneText, {});
    }
  },

  initTextarea(_params, textareaEl) {
    this.autoResize(textareaEl);
  },

  autoResize(textareaEl) {
    const cfg = this.settings;
    const targetEl = textareaEl || document.querySelector(cfg.selectors.textTextarea);
    if (!targetEl) return;
    targetEl.style.height = cfg.heightAuto;
    targetEl.style.height = Math.max(cfg.minHeightPx, (targetEl.scrollHeight || cfg.minHeightPx) + cfg.heightPaddingPx) + cfg.pxUnit;
  },

  autoResizeMeta(_params, element) {
    if (!element) return;
    const cfg = this.settings;
    element.style.height = cfg.heightAuto;
    element.style.height = Math.max(cfg.minMetaHeightPx, element.scrollHeight || 0) + cfg.pxUnit;
  },

  onHydrate() {
    if (!this.data.sceneId) return;
    const inp = document.querySelector(this.settings.selectors.sceneIdInput);
    if (inp) inp.value = this.data.sceneId;
  },

  onSceneTitleInput(_params, element) {
    if (!this.data.sceneId) return;
    const rawTitle = element ? element.value : "";
    NodeCanvasController.previewNodeEdit(this.data.sceneId, true, -1, rawTitle || this.data.sceneId);
  },

  onChoiceLabelInput(params, element) {
    this.autoResizeMeta(params, element);
    if (!this.data.sceneId) return;
    const choiceIdx = params ? params.idx : 0;
    const rawLabel = element ? element.value : "";
    NodeCanvasController.previewNodeEdit(this.data.sceneId, false, choiceIdx, rawLabel || "");
  },

  extractUsedVarNames(text, excludeStart, excludeEnd) {
    const cfg = this.settings;
    const stripped = String(text || "").slice(0, excludeStart) + " " + String(text || "").slice(excludeEnd);
    const used = new Set();
    const matches = stripped.match(cfg.wordTokenRe) || [];
    for (let idx = 0; idx < matches.length; idx++) {
      const word = matches[idx].toLowerCase();
      if (!cfg.reservedWords.has(word)) used.add(word);
    }
    return used;
  },

  visibleItems(dropdownEl) {
    return dropdownEl ? Array.from(dropdownEl.querySelectorAll(this.settings.selectors.visibleItem)) : [];
  },

  syncActiveItem(dropdownEl) {
    const items = this.visibleItems(dropdownEl);
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

  applyAutocompleteItem(itemBtn) {
    if (!itemBtn || !this.data.activeInput || this.data.qIndex < 0) return;
    const cfg = this.settings;
    const varName = itemBtn.getAttribute(cfg.attrs.synVarName) || "";
    if (!varName) return;
    const inputEl = this.data.activeInput;
    const curVal = inputEl.value || "";

    if (inputEl.classList && inputEl.classList.contains(cfg.classes.choiceExitInput)) {
      inputEl.value = varName;
    } else {
      const caret = inputEl.selectionStart != null ? inputEl.selectionStart : this.data.qIndex + 1;
      const before = curVal.slice(0, this.data.qIndex);
      const after = curVal.slice(Math.max(this.data.qIndex + 1, caret));
      const prefix = before.length > 0 && !cfg.leadingSpaceBeforeRe.test(before) ? before + " " : before;
      inputEl.value = prefix + varName + after;
    }
    const newPos = inputEl.value.length;
    this.closeDropdown();
    inputEl.focus();
    if (inputEl.setSelectionRange) inputEl.setSelectionRange(newPos, newPos);
    inputEl.dispatchEvent(new Event(cfg.changeEvent, { bubbles: true }));
  },

  onTextareaInput(params, element, event) {
    this.autoResize(element);
    this.onRuleInput(params, element, event);
  },

  onRuleInput(_params, inputEl) {
    if (!inputEl) return;
    const cfg = this.settings;
    const drop = document.querySelector(cfg.selectors.modalDropdown);
    if (!drop) return;
    const curVal = inputEl.value || "";
    const caret = inputEl.selectionStart != null ? inputEl.selectionStart : curVal.length;
    const beforeCaret = curVal.slice(0, caret);
    const lastQ = beforeCaret.lastIndexOf("?");
    if (lastQ < 0) {
      this.closeDropdown();
      return;
    }
    const filterRaw = beforeCaret.slice(lastQ + 1);
    if (cfg.invalidFilterCharRe.test(filterRaw)) {
      this.closeDropdown();
      return;
    }
    if (inputEl.rows !== undefined) {
      const lineStart = beforeCaret.lastIndexOf("\n") + 1;
      const lineBeforeQ = beforeCaret.slice(lineStart, lastQ).trim();
      if (!cfg.proseDirectiveRe.test(lineBeforeQ)) {
        this.closeDropdown();
        return;
      }
    }

    const filter = filterRaw.toLowerCase();
    const allItems = Array.from(drop.querySelectorAll(cfg.selectors.dropdownItem));
    const isTargetInput = Boolean(inputEl.classList && inputEl.classList.contains(cfg.classes.choiceExitInput));
    let visibleCount = 0;

    if (isTargetInput) {
      const existingVal = (curVal.slice(0, lastQ) + curVal.slice(caret)).trim().toLowerCase();
      allItems.forEach((btn) => {
        const vName = (btn.getAttribute(cfg.attrs.synVarName) || "").toLowerCase();
        const match = btn.getAttribute(cfg.attrs.synKind) === cfg.targetKind && vName !== existingVal && (!filter || vName.includes(filter));
        btn.classList.toggle(cfg.classes.hidden, !match);
        if (match) visibleCount++;
      });
    } else {
      const usedVars = this.extractUsedVarNames(curVal, lastQ, caret);
      allItems.forEach((btn) => {
        const synKind = btn.getAttribute(cfg.attrs.synKind) || cfg.varKind;
        const vName = (btn.getAttribute(cfg.attrs.synVarName) || "").toLowerCase();
        const match = synKind === cfg.varKind && !usedVars.has(vName) && (!filter || vName.includes(filter));
        btn.classList.toggle(cfg.classes.hidden, !match);
        if (match) visibleCount++;
      });
    }

    if (visibleCount === 0) {
      this.closeDropdown();
      return;
    }

    this.data.activeInput = inputEl;
    this.data.activeDropdown = drop;
    this.data.qIndex = lastQ;
    this.data.activeIdx = 0;
    this.syncActiveItem(drop);

    const rect = inputEl.getBoundingClientRect();
    const parent = drop.offsetParent || document.body;
    const parentRect = parent.getBoundingClientRect();
    drop.style.top = Math.round(rect.bottom - parentRect.top + cfg.dropdownOffsetTopPx) + cfg.pxUnit;
    drop.style.left = Math.round(Math.max(cfg.dropdownMinLeftPx, rect.left - parentRect.left)) + cfg.pxUnit;
    drop.classList.remove(cfg.classes.hidden);
  },

  onRuleKeydown(params, inputEl, event) {
    if (!inputEl) return;
    const cfg = this.settings;
    if (!this.data.activeDropdown || this.data.activeDropdown.classList.contains(cfg.classes.hidden) || this.data.activeInput !== inputEl) {
      if (event && event.key === cfg.keys.escape) {
        UIController.onKeydown(params, inputEl, event);
      }
      return;
    }
    const items = this.visibleItems(this.data.activeDropdown);
    if (!items.length) return;
    if (event.key === cfg.keys.arrowDown) {
      event.preventDefault();
      event.stopPropagation();
      this.data.activeIdx = (this.data.activeIdx + 1) % items.length;
      this.syncActiveItem(this.data.activeDropdown);
    } else if (event.key === cfg.keys.arrowUp) {
      event.preventDefault();
      event.stopPropagation();
      this.data.activeIdx = (this.data.activeIdx - 1 + items.length) % items.length;
      this.syncActiveItem(this.data.activeDropdown);
    } else if (event.key === cfg.keys.enter || event.key === cfg.keys.tab) {
      event.preventDefault();
      event.stopPropagation();
      this.applyAutocompleteItem(items[this.data.activeIdx] || items[0]);
    } else if (event.key === cfg.keys.escape) {
      event.preventDefault();
      event.stopPropagation();
      this.closeDropdown();
    }
  },

  onRuleBlur(_params, inputEl) {
    if (!inputEl) return;
    setTimeout(() => {
      if (this.data.activeInput === inputEl) this.closeDropdown();
    }, this.settings.blurCloseDelayMs);
  },

  chooseAutocompleteItem(_params, itemBtn, event) {
    if (event) event.preventDefault();
    this.applyAutocompleteItem(itemBtn);
  },

  updateTestIntVar(params) {
    Dispatcher.post(this.settings.actions.updateTestVar, Object.assign({}, params, { var_name: params.test_var_input || "" }));
  },

  updateTestEnumVar(params, element) {
    const cfg = this.settings;
    const root = element && element.closest ? element.closest(cfg.selectors.enumSelectRoot) : null;
    const varName = root ? root.getAttribute(cfg.varNameAttr) || "" : "";
    Dispatcher.post(cfg.actions.updateTestVar, {
      scene: this.data.sceneId || params.scene_id || "",
      var_name: varName,
      var_value: params.select_value || ""
    });
  },

  selectModalChapter(params) {
    Dispatcher.post(this.settings.actions.autoSaveScene, Object.assign({}, params, { chapter: params.select_value || "" }));
  },

  selectModalImage(params) {
    Dispatcher.post(this.settings.actions.autoSaveScene, Object.assign({}, params, { explicit_image: params.select_value || "" }));
  },

  selectModalVoice(params) {
    Dispatcher.post(this.settings.actions.autoSaveScene, Object.assign({}, params, { explicit_voice: params.select_value || "" }));
  },

  insertScriptFormatBlock(params) {
    Dispatcher.post(this.settings.actions.insertScriptBlock, Object.assign({}, params, { kind: params.insert_script_block || "" }));
  },

  insertScriptCharBlock(params) {
    Dispatcher.post(this.settings.actions.insertScriptBlock, Object.assign({}, params, { character: params.insert_script_char || "" }));
  }
};
Dispatcher.register("TextEditorController", TextEditorController);
