"use strict";

const CanvasOverlays = {
  bindings() {
    return {
      "#canvas-chapter-input": {
        input: this.onChapterInput,
        blur: this.commitChapterInput,
        keydown: this.onChapterKeydown
      },
      "#canvas-node-title-input": {
        input: this.onNodeTitleInput,
        blur: this.commitNodeTitle,
        keydown: this.onNodeTitleKeydown
      },
      "#canvas-choice-input": {
        input: this.onChoiceLabelInput,
        blur: this.commitChoiceLabel,
        keydown: this.onChoiceLabelKeydown
      }
    };
  },

  closeContextMenu() {
    const menu = document.querySelector("#canvas-context-menu");
    if (menu) menu.classList.add("hidden");
  },

  applyBoxStyle(inp, rect, fontPx, letterSpacing) {
    const state = CanvasModel.editorCanvasState;
    const z = state.zoom || 1.0;
    const px = state.panX || 0;
    const py = state.panY || 0;
    inp.style.left = (rect.x * z + px).toFixed(2) + "px";
    inp.style.top = (rect.y * z + 0.5 + py).toFixed(2) + "px";
    inp.style.width = Math.max(24, rect.w * z).toFixed(2) + "px";
    inp.style.height = Math.max(4, rect.h * z).toFixed(2) + "px";
    inp.style.fontSize = (fontPx * z).toFixed(2) + "px";
    if (letterSpacing !== undefined) {
      inp.style.letterSpacing = (letterSpacing * z).toFixed(2) + "px";
    }
  },

  updateFloatingInputTransform() {
    this.positionFloatingChapterInput();
    this.positionFloatingNodeTitleInput();
    this.positionFloatingChoiceInput();
  },

  showInput(inp, positionFn) {
    CanvasModel.studioGraphState.staticBatchDirty = true;
    inp.classList.remove("hidden");
    positionFn.call(this);
    inp.focus();
    if (typeof inp.select === "function") inp.select();
    EditorWebglScene.render();
  },

  positionFloatingChapterInput() {
    const inp = document.querySelector("#canvas-chapter-input");
    const ch = CanvasModel.studioGraphState.editingChapter;
    if (!inp || !ch || inp.classList.contains("hidden")) return;
    this.applyBoxStyle(inp, { x: ch.x + 28, y: ch.y + 4.703, w: ch.w - 38, h: 20 }, 11, 0.65);
  },

  openFloatingChapterInput(ch) {
    const inp = document.querySelector("#canvas-chapter-input");
    if (!inp || !ch) return;
    this.hideFloatingNodeTitleInput(true);
    this.hideFloatingChoiceInput(true);
    CanvasModel.studioGraphState.editingChapter = ch;
    inp.value = ch.name;
    inp.setAttribute("data-old-chapter", ch.name);
    this.showInput(inp, this.positionFloatingChapterInput);
  },

  commitChapterInput() {
    const inp = document.querySelector("#canvas-chapter-input");
    const ch = CanvasModel.studioGraphState.editingChapter;
    if (!inp || !ch) return;
    const newName = (inp.value || "").trim();
    const oldName = (inp.getAttribute("data-old-chapter") || ch.name || "").trim();
    CanvasModel.studioGraphState.editingChapter = null;
    CanvasModel.studioGraphState.staticBatchDirty = true;
    inp.classList.add("hidden");
    if (newName && newName !== oldName) {
      ch.name = newName;
      ch.nameLower = newName.toLowerCase();
      Dispatcher.post("RenameChapter", { old_chapter: oldName, new_chapter: newName });
    }
    EditorWebglScene.render();
  },

  hideFloatingChapterInput(shouldCommit) {
    const inp = document.querySelector("#canvas-chapter-input");
    if (!inp || inp.classList.contains("hidden")) return;
    if (shouldCommit) {
      this.commitChapterInput();
      return;
    }
    CanvasModel.studioGraphState.editingChapter = null;
    CanvasModel.studioGraphState.staticBatchDirty = true;
    inp.classList.add("hidden");
    EditorWebglScene.render();
  },

  onChapterInput(params, inp) {
    const ch = CanvasModel.studioGraphState.editingChapter;
    if (!ch || !inp) return;
    ch.name = inp.value || "";
    CanvasModel.studioGraphState.staticBatchDirty = true;
    EditorWebglScene.render();
  },

  onChapterKeydown(params, inp, e) {
    if (e.key === "Enter") {
      e.preventDefault();
      this.hideFloatingChapterInput(true);
    } else if (e.key === "Escape") {
      e.preventDefault();
      this.hideFloatingChapterInput(false);
    }
  },

  positionFloatingNodeTitleInput() {
    const inp = document.querySelector("#canvas-node-title-input");
    const nd = CanvasModel.studioGraphState.editingNodeTitle;
    if (!inp || !nd || inp.classList.contains("hidden")) return;
    const radioReserve = nd.branchRole === "primary" || nd.branchRole === "secondary" ? 16 : 0;
    const badgeReserve = (nd.isStart || nd.isEntry ? 50 : 8) + radioReserve;
    this.applyBoxStyle(inp, { x: nd.x + 28, y: nd.y + 6.715, w: nd.w - 28 - badgeReserve, h: 22 }, 13);
  },

  openFloatingNodeTitleInput(nd) {
    const inp = document.querySelector("#canvas-node-title-input");
    if (!inp || !nd) return;
    this.hideFloatingChapterInput(true);
    this.hideFloatingChoiceInput(true);
    CanvasModel.studioGraphState.editingNodeTitle = nd;
    const curTitle = nd.name || nd.id || "";
    inp.value = curTitle;
    inp.setAttribute("data-scene-id", nd.id);
    inp.setAttribute("data-old-title", curTitle);
    this.showInput(inp, this.positionFloatingNodeTitleInput);
  },

  onNodeTitleInput(params, inp) {
    const nd = CanvasModel.studioGraphState.editingNodeTitle;
    if (!nd || !inp) return;
    nd.name = inp.value || "";
    const expandTitle = document.querySelector("#scene-text-expand-title");
    if (expandTitle) expandTitle.textContent = nd.name;
    const modalInp = document.querySelector('#form-scene-text input[name="name"]');
    if (modalInp) modalInp.value = nd.name;
    CanvasModel.studioGraphState.staticBatchDirty = true;
    EditorWebglScene.render();
  },

  commitNodeTitle() {
    const inp = document.querySelector("#canvas-node-title-input");
    const nd = CanvasModel.studioGraphState.editingNodeTitle;
    if (!inp || !nd) return;
    const newName = (inp.value || "").trim();
    const oldName = (inp.getAttribute("data-old-title") || nd.name || nd.id || "").trim();
    nd.name = newName || oldName || nd.id;
    CanvasModel.studioGraphState.editingNodeTitle = null;
    CanvasModel.studioGraphState.staticBatchDirty = true;
    inp.classList.add("hidden");
    EditorWebglScene.render();
    if (newName && newName !== oldName) {
      Dispatcher.post("AutoSaveScene", { scene_id: nd.id, name: newName });
    }
  },

  hideFloatingNodeTitleInput(shouldCommit) {
    const inp = document.querySelector("#canvas-node-title-input");
    if (!inp || inp.classList.contains("hidden")) return;
    if (shouldCommit) {
      this.commitNodeTitle();
      return;
    }
    const nd = CanvasModel.studioGraphState.editingNodeTitle;
    if (nd) {
      nd.name = (inp.getAttribute("data-old-title") || nd.id || "").trim();
    }
    CanvasModel.studioGraphState.editingNodeTitle = null;
    CanvasModel.studioGraphState.staticBatchDirty = true;
    inp.classList.add("hidden");
    EditorWebglScene.render();
  },

  onNodeTitleKeydown(params, inp, e) {
    if (e.key === "Enter") {
      e.preventDefault();
      this.hideFloatingNodeTitleInput(true);
    } else if (e.key === "Escape") {
      e.preventDefault();
      this.hideFloatingNodeTitleInput(false);
    }
  },

  positionFloatingChoiceInput() {
    const inp = document.querySelector("#canvas-choice-input");
    const ec = CanvasModel.studioGraphState.editingChoice;
    if (!inp || !ec || !ec.node || inp.classList.contains("hidden")) return;
    const nd = ec.node;
    const rowY = nd.y + CanvasModel.CARD.headerH + ec.idx * CanvasModel.CARD.choiceH;
    this.applyBoxStyle(inp, { x: nd.x + 21, y: rowY + 2.703, w: nd.w - 47, h: CanvasModel.CARD.choiceH - 6 }, 11);
  },

  openFloatingChoiceInput(nd, choiceIdx) {
    const inp = document.querySelector("#canvas-choice-input");
    if (!inp || !nd || !nd.ports || !nd.ports[choiceIdx]) return;
    this.hideFloatingChapterInput(true);
    this.hideFloatingNodeTitleInput(true);
    CanvasModel.studioGraphState.editingChoice = { node: nd, idx: choiceIdx };
    const curLabel = nd.ports[choiceIdx].label || AppShellController.tr("choice_prefix") + (choiceIdx + 1);
    inp.value = curLabel;
    inp.setAttribute("data-scene-id", nd.id);
    inp.setAttribute("data-choice-idx", String(choiceIdx));
    inp.setAttribute("data-old-label", curLabel);
    this.showInput(inp, this.positionFloatingChoiceInput);
  },

  onChoiceLabelInput(params, inp) {
    const ec = CanvasModel.studioGraphState.editingChoice;
    if (!ec || !ec.node || !inp) return;
    const pt = ec.node.ports && ec.node.ports[ec.idx];
    if (pt) {
      pt.label = inp.value || "";
      CanvasModel.studioGraphState.staticBatchDirty = true;
      EditorWebglScene.render();
    }
  },

  commitChoiceLabel() {
    const inp = document.querySelector("#canvas-choice-input");
    const ec = CanvasModel.studioGraphState.editingChoice;
    if (!inp || !ec || !ec.node) return;
    const nd = ec.node, idx = ec.idx;
    const pt = nd.ports && nd.ports[idx];
    const newLabel = (inp.value || "").trim() || AppShellController.tr("choice_prefix") + (idx + 1);
    const oldLabel = (inp.getAttribute("data-old-label") || "").trim();
    if (pt) pt.label = newLabel;
    CanvasModel.studioGraphState.editingChoice = null;
    CanvasModel.studioGraphState.staticBatchDirty = true;
    inp.classList.add("hidden");
    EditorWebglScene.render();
    if (newLabel !== oldLabel) {
      Dispatcher.post("AutoSaveScene", { scene_id: nd.id, ["choice_label_" + idx]: newLabel });
    }
  },

  hideFloatingChoiceInput(shouldCommit) {
    const inp = document.querySelector("#canvas-choice-input");
    if (!inp || inp.classList.contains("hidden")) return;
    if (shouldCommit) {
      this.commitChoiceLabel();
      return;
    }
    const ec = CanvasModel.studioGraphState.editingChoice;
    if (ec && ec.node && ec.node.ports && ec.node.ports[ec.idx]) {
      ec.node.ports[ec.idx].label = (inp.getAttribute("data-old-label") || AppShellController.tr("choice_prefix") + (ec.idx + 1)).trim();
    }
    CanvasModel.studioGraphState.editingChoice = null;
    CanvasModel.studioGraphState.staticBatchDirty = true;
    inp.classList.add("hidden");
    EditorWebglScene.render();
  },

  onChoiceLabelKeydown(params, inp, e) {
    if (e.key === "Enter") {
      e.preventDefault();
      this.hideFloatingChoiceInput(true);
    } else if (e.key === "Escape") {
      e.preventDefault();
      this.hideFloatingChoiceInput(false);
    }
  }
};
Dispatcher.register("CanvasOverlays", CanvasOverlays);
