"use strict";

const TextEditorController = {
  data: {
    sceneId: "",
    bookVarsSpec: "",
    isScreenplay: false,
    scriptStandard: "pl"
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
    heightAuto: "auto",
    pxUnit: "px",
    varNameAttr: "data-var-name",
    selectors: {
      textTextarea: "#modal-scene-textarea",
      sceneIdInput: '#form-scene-text input[name="scene_id"]',
      modalDropdown: "#modal-syn-autocomplete-dropdown",
      enumSelectRoot: ".test-var-enum-select"
    },
    actions: {
      closeSceneText: "CloseSceneTextModal",
      autoSaveScene: "AutoSaveScene",
      updateTestVar: "UpdateSceneTextTestVar",
      insertScriptBlock: "InsertScreenplayBlock"
    }
  },

  onModalBackdrop(_params, el, e) {
    if (e && e.target === el) {
      Dispatcher.post(this.settings.actions.closeSceneText, {});
    }
  },

  initTextarea(_params, ta) {
    this.autoResize(ta);
  },

  autoResize(ta) {
    const s = this.settings;
    const el = ta || document.querySelector(s.selectors.textTextarea);
    if (!el) return;
    el.style.height = s.heightAuto;
    el.style.height = Math.max(s.minHeightPx, (el.scrollHeight || s.minHeightPx) + s.heightPaddingPx) + s.pxUnit;
  },

  autoResizeMeta(_params, el) {
    if (!el) return;
    const s = this.settings;
    el.style.height = s.heightAuto;
    el.style.height = Math.max(s.minMetaHeightPx, el.scrollHeight || 0) + s.pxUnit;
  },

  onHydrate() {
    if (!this.data.sceneId) return;
    const inp = document.querySelector(this.settings.selectors.sceneIdInput);
    if (inp) inp.value = this.data.sceneId;
  },

  onSceneTitleInput(_params, el) {
    EditorContextController.syncCanvasNodeTitle(this.data.sceneId, el ? el.value : "");
  },

  onChoiceLabelInput(params, el) {
    this.autoResizeMeta(params, el);
    EditorContextController.syncCanvasChoiceLabel(this.data.sceneId, params ? params.idx : 0, el ? el.value : "");
  },

  onTextareaInput(p, el, e) {
    this.autoResize(el);
    EditorContextController.onRuleInput(p, el, e, document.querySelector(this.settings.selectors.modalDropdown));
  },

  onRuleInput(p, el, e) {
    EditorContextController.onRuleInput(p, el, e, document.querySelector(this.settings.selectors.modalDropdown));
  },

  onRuleKeydown(p, el, e) {
    if (!el) return;
    EditorContextController.onRuleKeydown(p, el, e);
  },

  onRuleBlur(p, el) {
    if (!el) return;
    EditorContextController.onRuleBlur(p, el);
  },

  chooseAutocompleteItem(_params, el, e) {
    if (e) e.preventDefault();
    EditorContextController.applyAutocompleteItem(el);
  },

  updateTestIntVar(p) {
    Dispatcher.post(this.settings.actions.updateTestVar, Object.assign({}, p, { var_name: p.test_var_input || "" }));
  },

  updateTestEnumVar(p, el) {
    const s = this.settings;
    const root = el && el.closest ? el.closest(s.selectors.enumSelectRoot) : null;
    const varName = root ? root.getAttribute(s.varNameAttr) || "" : "";
    Dispatcher.post(s.actions.updateTestVar, {
      scene: this.data.sceneId || p.scene_id || "",
      var_name: varName,
      var_value: p.select_value || ""
    });
  },

  selectModalChapter(p) {
    Dispatcher.post(this.settings.actions.autoSaveScene, Object.assign({}, p, { chapter: p.select_value || "" }));
  },

  selectModalImage(p) {
    Dispatcher.post(this.settings.actions.autoSaveScene, Object.assign({}, p, { explicit_image: p.select_value || "" }));
  },

  selectModalVoice(p) {
    Dispatcher.post(this.settings.actions.autoSaveScene, Object.assign({}, p, { explicit_voice: p.select_value || "" }));
  },

  insertScriptFormatBlock(p) {
    Dispatcher.post(this.settings.actions.insertScriptBlock, Object.assign({}, p, { kind: p.insert_script_block || "" }));
  },

  insertScriptCharBlock(p) {
    Dispatcher.post(this.settings.actions.insertScriptBlock, Object.assign({}, p, { character: p.insert_script_char || "" }));
  }
};
Dispatcher.register("TextEditorController", TextEditorController);
