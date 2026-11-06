"use strict";

let liveSaveTimers = new WeakMap();
let dirtyInspectorForms = new WeakSet();
let chapterLiveSaveTimers = new Map();
let dirtyChapterInputs = new Set();
let bookTitleSaveTimer = null;
let dirtyBookTitleInput = null;

function isEditorHistoryPreview() {
  const ed = typeof qs === "function" ? qs("#view-editor") : null;
  if (ed) {
    return ed.getAttribute("data-history-preview") === "true" || ed.classList.contains("is-history-preview");
  }
  if (typeof document !== "undefined" && document.body) {
    return document.body.getAttribute("data-history-preview") === "true";
  }
  return false;
}

function syncBookTitleDOM(newTitle) {
  const clean = (newTitle || "").trim();
  if (!clean) return;
  const playerTab = qs("#tab-player");
  if (playerTab) {
    const lbl = qs(".tab-label", playerTab);
    if (lbl) {
      lbl.textContent = clean;
    }
  }
  const toolbarTitle = qs("#editor-book-title");
  if (toolbarTitle) {
    toolbarTitle.textContent = clean;
  }
  const inp = qs("#book-settings-title-input");
  if (inp && document.activeElement !== inp) {
    inp.value = clean;
    inp.setAttribute("data-old-title", clean);
  }
}

function commitBookTitleInput(inputEl) {
  if (!inputEl || isEditorHistoryPreview()) return;
  if (bookTitleSaveTimer) {
    clearTimeout(bookTitleSaveTimer);
    bookTitleSaveTimer = null;
  }
  if (dirtyBookTitleInput === inputEl) {
    dirtyBookTitleInput = null;
  }
  const oldTitle = (inputEl.getAttribute("data-old-title") || "").trim();
  const newTitle = (inputEl.value || "").trim();
  if (!newTitle && oldTitle) {
    inputEl.value = oldTitle;
    syncBookTitleDOM(oldTitle);
    return;
  }
  if (newTitle && newTitle !== oldTitle) {
    syncBookTitleDOM(newTitle);
    inputEl.setAttribute("data-old-title", newTitle);
    postServerActionSilent("RenameBook", {
      title: newTitle,
      silent: "true"
    });
  }
}

function scheduleBookTitleLiveSave(inputEl) {
  if (!inputEl || isEditorHistoryPreview()) return;
  const newTitle = (inputEl.value || "").trim();
  if (newTitle) {
    syncBookTitleDOM(newTitle);
  }
  dirtyBookTitleInput = inputEl;
  if (bookTitleSaveTimer) {
    clearTimeout(bookTitleSaveTimer);
  }
  bookTitleSaveTimer = setTimeout(() => {
    bookTitleSaveTimer = null;
    commitBookTitleInput(inputEl);
  }, 250);
}

function syncChapterGroupRenameDOM(inputEl, targetChapterName) {
  if (!inputEl) return;
  const prevGroup = (inputEl.getAttribute("data-old-chapter") || "").trim();
  const nextGroup = (
    targetChapterName !== undefined ? targetChapterName : inputEl.value || ""
  ).trim();
  if (!prevGroup || !nextGroup) return;

  const prevLower = prevGroup.toLowerCase();
  const nextLower = nextGroup.toLowerCase();

  if (typeof studioGraphState !== "undefined") {
    for (let i = 0; i < studioGraphState.chapters.length; i++) {
      const ch = studioGraphState.chapters[i];
      if (ch.nameLower === prevLower) {
        studioGraphState.chapterByKey.delete(ch.nameLower);
        ch.name = nextGroup;
        ch.nameLower = nextLower;
        studioGraphState.chapterByKey.set(nextLower, ch);
      }
    }
    if (studioGraphState.selectedChapterKey === prevLower) {
      studioGraphState.selectedChapterKey = nextLower;
    }
    for (let i = 0; i < studioGraphState.nodes.length; i++) {
      const nd = studioGraphState.nodes[i];
      if (nd.chapterLower === prevLower) {
        nd.chapter = nextGroup;
        nd.chapterLower = nextLower;
      }
    }
    studioGraphState.staticBatchDirty = true;
    renderStudioCanvas();
  }

  const chapterForm = qs("#editor-chapter-form");
  if (chapterForm && (chapterForm.getAttribute("data-context-chapter") || "").trim().toLowerCase() === prevLower) {
    chapterForm.setAttribute("data-context-chapter", nextGroup);
    const oldInp = qs('input[name="old_chapter"]', chapterForm);
    if (oldInp) oldInp.value = nextGroup;
    const chInp = qs('input[name="chapter"]', chapterForm);
    if (chInp) chInp.value = nextGroup;
  }

  const contextForm = qs("#editor-context-form");
  if (contextForm) {
    const chInput = qs('input[name="chapter"]', contextForm);
    if (chInput && (chInput.value || "").trim().toLowerCase() === prevLower) {
      chInput.value = nextGroup;
    }
    qsa(".node-chapter-select .custom-select-option", contextForm).forEach((opt) => {
      const optVal = (opt.getAttribute("data-select-value") || "").trim();
      if (optVal.toLowerCase() === prevLower) {
        opt.setAttribute("data-select-value", nextGroup);
        opt.setAttribute("data-select-label", nextGroup);
        opt.textContent = nextGroup;
      }
    });
  }

  const multiPanel = qs("#editor-multi-panel");
  if (multiPanel) {
    qsa(".context-multi-scene-meta", multiPanel).forEach((meta) => {
      if ((meta.textContent || "").trim().toLowerCase() === prevLower) {
        meta.textContent = nextGroup;
      }
    });
  }
}

function commitChapterTitleInput(inputEl) {
  if (!inputEl || isEditorHistoryPreview()) return;
  const existingTimer = chapterLiveSaveTimers.get(inputEl);
  if (existingTimer) {
    clearTimeout(existingTimer);
    chapterLiveSaveTimers.delete(inputEl);
  }
  dirtyChapterInputs.delete(inputEl);

  const oldCh = (inputEl.getAttribute("data-old-chapter") || "").trim();
  const newCh = (inputEl.value || "").trim();
  if (!newCh && oldCh) {
    inputEl.value = oldCh;
    syncChapterGroupRenameDOM(inputEl, oldCh);
    return;
  }
  if (oldCh && newCh && oldCh !== newCh) {
    syncChapterGroupRenameDOM(inputEl, newCh);
    inputEl.setAttribute("data-old-chapter", newCh);
    postServerActionSilent("RenameChapter", {
      old_chapter: oldCh,
      new_chapter: newCh,
      silent: "true"
    });
  }
}

function scheduleChapterLiveSave(inputEl) {
  if (!inputEl || isEditorHistoryPreview()) return;
  const newCh = (inputEl.value || "").trim();
  if (newCh) {
    syncChapterGroupRenameDOM(inputEl, newCh);
  }
  dirtyChapterInputs.add(inputEl);
  const existing = chapterLiveSaveTimers.get(inputEl);
  if (existing) {
    clearTimeout(existing);
  }
  const timer = setTimeout(() => {
    chapterLiveSaveTimers.delete(inputEl);
    commitChapterTitleInput(inputEl);
  }, 250);
  chapterLiveSaveTimers.set(inputEl, timer);
}

function cancelPendingInspectorTimers() {
  if (bookTitleSaveTimer) {
    clearTimeout(bookTitleSaveTimer);
    bookTitleSaveTimer = null;
  }
  dirtyBookTitleInput = null;
  if (chapterLiveSaveTimers.size > 0) {
    chapterLiveSaveTimers.forEach((timer) => clearTimeout(timer));
    chapterLiveSaveTimers.clear();
  }
  dirtyChapterInputs.clear();
  ["#form-scene-text", "#editor-context-form"].forEach((sel) => {
    const form = qs(sel);
    if (form) {
      const existingTimer = liveSaveTimers.get(form);
      if (existingTimer) {
        clearTimeout(existingTimer);
        liveSaveTimers.delete(form);
      }
      dirtyInspectorForms.delete(form);
    }
  });
}

function consumeDirtyInspectorFormIntoParams(params) {
  if (isEditorHistoryPreview()) {
    cancelPendingInspectorTimers();
    return;
  }
  if (dirtyBookTitleInput) {
    commitBookTitleInput(dirtyBookTitleInput);
  }
  if (dirtyChapterInputs.size > 0) {
    Array.from(dirtyChapterInputs).forEach((inp) => {
      commitChapterTitleInput(inp);
    });
  }
  ["#form-scene-text", "#editor-context-form"].forEach((sel) => {
    const form = qs(sel);
    if (!form || !dirtyInspectorForms.has(form)) return;
    const existingTimer = liveSaveTimers.get(form);
    if (existingTimer) {
      clearTimeout(existingTimer);
      liveSaveTimers.delete(form);
    }
    dirtyInspectorForms.delete(form);
    if (params && typeof params === "object") {
      const fd = new FormData(form);
      fd.forEach((val, key) => {
        if (params[key] === undefined) {
          params[key] = val;
        }
      });
    }
  });
}

function flushInspectorFormIfDirty() {
  if (isEditorHistoryPreview()) {
    cancelPendingInspectorTimers();
    return;
  }
  if (dirtyBookTitleInput) {
    commitBookTitleInput(dirtyBookTitleInput);
  }
  if (dirtyChapterInputs.size > 0) {
    Array.from(dirtyChapterInputs).forEach((inp) => {
      commitChapterTitleInput(inp);
    });
  }
  ["#form-scene-text", "#editor-context-form"].forEach((sel) => {
    const form = qs(sel);
    if (!form || !dirtyInspectorForms.has(form)) return;
    const existingTimer = liveSaveTimers.get(form);
    if (existingTimer) {
      clearTimeout(existingTimer);
      liveSaveTimers.delete(form);
    }
    dirtyInspectorForms.delete(form);
    postServerActionSilent("AutoSaveScene", new FormData(form));
  });
}

function isAndroidStudioClient() {
  if (typeof navigator !== "undefined" && /Android/i.test(navigator.userAgent || "")) {
    return true;
  }
  if (typeof document !== "undefined" && document.body && document.body.getAttribute("data-android") === "true") {
    return true;
  }
  return false;
}

function isAnyEditorPanelOpen() {
  const ids = [
    "#editor-context-panel",
    "#editor-variables-panel",
    "#editor-breakdown-panel",
    "#editor-history-panel",
    "#editor-help-panel"
  ];
  for (let i = 0; i < ids.length; i++) {
    const el = qs(ids[i]);
    if (el && !el.classList.contains("hidden")) {
      return true;
    }
  }
  return false;
}

function isPortraitPhoneStudioLayout() {
  /* @dev-only-start */
  const stage = typeof qs === "function" ? qs("#device-stage") : null;
  if (stage && stage.getAttribute("data-preview") === "portrait") {
    return true;
  }
  /* @dev-only-end */
  if (typeof window !== "undefined" && typeof window.matchMedia === "function") {
    return window.matchMedia("(max-width: 760px) and (orientation: portrait)").matches;
  }
  return false;
}

function syncEditorViewportVisibility() {
  const anyRightOpen = isAnyEditorPanelOpen();
  const sceneTextModal = qs("#modal-scene-text");
  const editorBody = qs("#editor-workspace-body");
  if (sceneTextModal) {
    sceneTextModal.classList.toggle("has-right-panel", anyRightOpen);
    if (editorBody && editorBody.offsetTop > 0) {
      sceneTextModal.style.top = editorBody.offsetTop + "px";
    } else if (sceneTextModal.style) {
      sceneTextModal.style.top = "";
    }
  }
  const vp = qs("#timeline-scroll-viewport");
  if (!vp) return;
  const shouldHide = isPortraitPhoneStudioLayout() && anyRightOpen;
  vp.classList.toggle("hidden", shouldHide);
  const canvasFullyCovered = Boolean(
    sceneTextModal &&
      (sceneTextModal.classList.contains("mode-full") || anyRightOpen)
  );
  if (!shouldHide && !canvasFullyCovered) {
    const cv = qs("#studio-canvas");
    if (cv && typeof syncCanvasBackingStoreSize === "function") {
      syncCanvasBackingStoreSize(cv, vp);
    }
    if (typeof renderStudioCanvas === "function") {
      renderStudioCanvas();
    }
  }
}

if (typeof document !== "undefined" && typeof navigator !== "undefined" && /Android/i.test(navigator.userAgent || "")) {
  if (document.body) {
    document.body.setAttribute("data-android", "true");
  } else {
    document.addEventListener("DOMContentLoaded", () => {
      if (document.body) document.body.setAttribute("data-android", "true");
    });
  }
}

function updateEditorContextPanelVisibility() {
  const panel = qs("#editor-context-panel");
  if (!panel) return;
  const chapterForm = qs("#editor-chapter-form", panel);
  const bookForm = qs("#editor-book-form", panel);
  const multiPanel = qs("#editor-multi-panel", panel);
  const hasVisible =
    (chapterForm && !chapterForm.classList.contains("hidden")) ||
    (bookForm && !bookForm.classList.contains("hidden")) ||
    (multiPanel && !multiPanel.classList.contains("hidden"));
  panel.classList.toggle("hidden", !hasVisible);
  syncEditorViewportVisibility();
}

function selectNodeForContextPanel(sceneId) {
  if (!sceneId) return;
  editorSelectionCleared = false;
  if (typeof studioGraphState !== "undefined") {
    studioGraphState.overlayBatchDirty = true;
  }
  const expandBtn = qs("#btn-expand-scene-text");
  if (expandBtn) {
    expandBtn.setAttribute("data-scene", sceneId);
  }
  const form = qs("#editor-context-form");
  if (form) {
    form.classList.add("hidden");
  }
  const multiPanel = qs("#editor-multi-panel");
  if (multiPanel) {
    multiPanel.classList.add("hidden");
  }
  updateEditorContextPanelVisibility();
  const openModal = qs("#modal-scene-text");
  if (openModal) {
    const curModalInp = qs('input[name="scene_id"]', openModal);
    const curModalId = curModalInp ? (curModalInp.value || "").trim() : "";
    if (curModalId !== sceneId) {
      sendServerAction("OpenSceneTextModal", { scene: sceneId });
    } else {
      postServerActionSilent("SelectStudioScene", { scene: sceneId });
    }
  } else {
    postServerActionSilent("SelectStudioScene", { scene: sceneId });
  }
}

function selectChapterForContextPanel(chapterName) {
  const cleanName = (chapterName || "").trim();
  if (!cleanName) return;
  flushInspectorFormIfDirty();
  editorSelectionCleared = true;
  selectedNodeIds.clear();
  if (typeof studioGraphState !== "undefined") {
    studioGraphState.selectedChapterKey = cleanName.toLowerCase();
    studioGraphState.overlayBatchDirty = true;
  }
  const bookSettingsBtn = qs("#btn-book-settings");
  if (bookSettingsBtn) {
    bookSettingsBtn.classList.remove("active");
  }
  const breakdownBtn = qs("#btn-editor-breakdown");
  if (breakdownBtn) {
    breakdownBtn.classList.remove("active");
  }
  const breakdownPanel = qs("#editor-breakdown-panel");
  if (breakdownPanel) {
    breakdownPanel.classList.add("hidden");
  }
  const variablesBtn = qs("#btn-editor-variables");
  if (variablesBtn) {
    variablesBtn.classList.remove("active");
  }
  const variablesPanel = qs("#editor-variables-panel");
  if (variablesPanel) {
    variablesPanel.classList.add("hidden");
  }
  const historyPanel = qs("#editor-history-panel");
  if (historyPanel) {
    historyPanel.classList.add("hidden");
  }
  const helpPanel = qs("#editor-help-panel");
  if (helpPanel) {
    helpPanel.classList.add("hidden");
  }
  const helpBtn = qs("#btn-editor-help");
  if (helpBtn) {
    helpBtn.classList.remove("active");
  }
  qsa('.canvas-history-controls .top-action-btn').forEach((btn) => {
    btn.classList.remove("active");
  });
  syncNodeSelectionDOM();
  renderStudioCanvas();
  const panel = qs("#editor-context-panel");
  const form = qs("#editor-context-form");
  const chapterForm = qs("#editor-chapter-form");
  const bookForm = qs("#editor-book-form");
  const multiPanel = qs("#editor-multi-panel");
  if (form) {
    form.classList.add("hidden");
  }
  if (bookForm) {
    bookForm.classList.add("hidden");
  }
  if (multiPanel) {
    multiPanel.classList.add("hidden");
  }
  const currentChapter = chapterForm ? (chapterForm.getAttribute("data-context-chapter") || "").trim() : "";
  if (
    panel &&
    chapterForm &&
    !chapterForm.classList.contains("hidden") &&
    currentChapter.toLowerCase() === cleanName.toLowerCase()
  ) {
    updateEditorContextPanelVisibility();
    return;
  }
  if (chapterForm && currentChapter.toLowerCase() === cleanName.toLowerCase()) {
    chapterForm.classList.remove("hidden");
  }
  updateEditorContextPanelVisibility();
  sendServerAction("SelectStudioChapter", { chapter: cleanName });
}

function hideEditorContextPanel() {
  flushInspectorFormIfDirty();
  if (typeof studioGraphState !== "undefined") {
    studioGraphState.selectedChapterKey = "";
    studioGraphState.overlayBatchDirty = true;
  }
  const bookSettingsBtn = qs("#btn-book-settings");
  if (bookSettingsBtn) {
    bookSettingsBtn.classList.remove("active");
  }
  const form = qs("#editor-context-form");
  if (form) {
    form.classList.add("hidden");
  }
  const chapterForm = qs("#editor-chapter-form");
  const bookForm = qs("#editor-book-form");
  const hadOpenContextForm = Boolean(
    (chapterForm && !chapterForm.classList.contains("hidden")) ||
      (bookForm && !bookForm.classList.contains("hidden"))
  );
  if (chapterForm) {
    chapterForm.classList.add("hidden");
  }
  if (bookForm) {
    bookForm.classList.add("hidden");
  }
  const multiPanel = qs("#editor-multi-panel");
  if (multiPanel) {
    multiPanel.classList.add("hidden");
  }
  updateEditorContextPanelVisibility();
  if (hadOpenContextForm && typeof postServerActionSilent === "function") {
    postServerActionSilent("CloseBookSettings");
  }
}

function syncNodeSelectionDOM() {
  if (typeof studioGraphState !== "undefined") {
    studioGraphState.overlayBatchDirty = true;
  }
  const selectedItems = [];
  const seenIds = new Set();

  if (typeof studioGraphState !== "undefined" && studioGraphState.nodes) {
    for (let i = 0; i < studioGraphState.nodes.length; i++) {
      const nd = studioGraphState.nodes[i];
      const isSel = selectedNodeIds.has(nd.id);
      nd.selected = isSel;
      if (isSel && nd.id) {
        seenIds.add(nd.id);
        selectedItems.push({
          id: nd.id,
          title: nd.name || nd.id,
          chapter: nd.chapter || ""
        });
      }
    }
  }

  selectedNodeIds.forEach((id) => {
    if (id && !seenIds.has(id)) {
      selectedItems.push({
        id: id,
        title: id,
        chapter: ""
      });
    }
  });

  const badge = qs("#canvas-selection-badge");
  if (badge) {
    if (selectedNodeIds.size > 1) {
      badge.textContent = "Zaznaczono: " + selectedNodeIds.size;
      badge.classList.remove("hidden");
    } else {
      badge.classList.add("hidden");
    }
  }

  const panel = qs("#editor-context-panel");
  const form = qs("#editor-context-form");
  const chapterForm = qs("#editor-chapter-form");
  const bookForm = qs("#editor-book-form");
  const multiPanel = qs("#editor-multi-panel");
  if (bookForm && !bookForm.classList.contains("hidden")) {
    qsa("textarea.node-meta-textarea", bookForm).forEach((ta) => {
      autoResizeInspectorTextarea(ta);
    });
  }

  if (selectedNodeIds.size === 0) {
    flushInspectorFormIfDirty();
    if (form) form.classList.add("hidden");
    if (multiPanel) multiPanel.classList.add("hidden");
  } else if (selectedNodeIds.size === 1) {
    if (form) form.classList.add("hidden");
    if (multiPanel) multiPanel.classList.add("hidden");
  } else {
    flushInspectorFormIfDirty();
    if (typeof studioGraphState !== "undefined") {
      studioGraphState.selectedChapterKey = "";
    }
    if (form) form.classList.add("hidden");
    if (chapterForm) chapterForm.classList.add("hidden");
    if (bookForm) bookForm.classList.add("hidden");
    const bookSettingsBtn = qs("#btn-book-settings");
    if (bookSettingsBtn) bookSettingsBtn.classList.remove("active");
    const breakdownBtn = qs("#btn-editor-breakdown");
    if (breakdownBtn) breakdownBtn.classList.remove("active");
    const breakdownPanel = qs("#editor-breakdown-panel");
    if (breakdownPanel) breakdownPanel.classList.add("hidden");
    const variablesBtn = qs("#btn-editor-variables");
    if (variablesBtn) variablesBtn.classList.remove("active");
    const variablesPanel = qs("#editor-variables-panel");
    if (variablesPanel) variablesPanel.classList.add("hidden");
    const historyPanel = qs("#editor-history-panel");
    if (historyPanel) historyPanel.classList.add("hidden");
    const helpPanel = qs("#editor-help-panel");
    if (helpPanel) helpPanel.classList.add("hidden");
    const helpBtn = qs("#btn-editor-help");
    if (helpBtn) helpBtn.classList.remove("active");
    qsa('.canvas-history-controls .top-action-btn').forEach((btn) => {
      btn.classList.remove("active");
    });
    if (multiPanel) {
      multiPanel.classList.remove("hidden");
      const countLabel = qs("#multi-select-count-label", multiPanel);
      if (countLabel) {
        countLabel.textContent = "Zaznaczone sceny (" + selectedItems.length + ")";
      }
      const listEl = qs("#context-multi-scene-list", multiPanel);
      if (listEl) {
        const itemById = new Map();
        selectedItems.forEach((it) => {
          itemById.set(it.id, it);
        });
        qsa(".context-multi-scene-item", listEl).forEach((row) => {
          const rowId = row.getAttribute("data-multi-scene-id") || "";
          const item = itemById.get(rowId);
          row.classList.toggle("hidden", !item);
          if (item) {
            const titleSpan = qs(".context-multi-scene-title", row);
            if (titleSpan && item.title) {
              titleSpan.textContent = item.title;
            }
            const metaSpan = qs(".context-multi-scene-meta", row);
            if (metaSpan && item.chapter) {
              metaSpan.textContent = item.chapter;
            }
          }
        });
      }
      const delBtn = qs("#btn-delete-selected-scenes", multiPanel);
      if (delBtn) {
        delBtn.setAttribute("data-scenes", selectedItems.map((it) => it.id).join(","));
      }
      const delLabel = qs("#btn-delete-selected-scenes-label", multiPanel);
      if (delLabel) {
        delLabel.textContent = "Usuń zaznaczone sceny (" + selectedItems.length + ")";
      }
    }
  }
  updateEditorContextPanelVisibility();
  autoResizeAllInspectorTextareas(panel || document);
}

function closeAllCustomSelects(exceptSelect) {
  qsa(".custom-select.open").forEach((sel) => {
    if (sel === exceptSelect) return;
    sel.classList.remove("open");
    const menu = qs(".custom-select-menu", sel);
    if (menu) menu.classList.add("hidden");
  });
}

function scheduleNodeLiveSave(form) {
  if (!form || isEditorHistoryPreview()) return;
  dirtyInspectorForms.add(form);
  const existing = liveSaveTimers.get(form);
  if (existing) {
    clearTimeout(existing);
  }
  const timer = setTimeout(() => {
    liveSaveTimers.delete(form);
    dirtyInspectorForms.delete(form);
    postServerActionSilent("AutoSaveScene", new FormData(form));
  }, 250);
  liveSaveTimers.set(form, timer);
}

function autoResizeInspectorTextarea(ta) {
  if (!ta) return;
  ta.style.minHeight = "0px";
  ta.style.height = "0px";
  const nextH = Math.max(24, (ta.scrollHeight || 0) + 2);
  ta.style.height = nextH + "px";
  ta.style.minHeight = nextH + "px";
  ta.scrollTop = 0;
}

function autoResizeAllInspectorTextareas(root) {
  qsa(".node-meta-textarea", root || document).forEach((ta) => {
    autoResizeInspectorTextarea(ta);
  });
}

function scheduleBookSettingsLiveSave(form) {
  if (!form || isEditorHistoryPreview()) return;
  const existing = liveSaveTimers.get(form);
  if (existing) {
    clearTimeout(existing);
  }
  const timer = setTimeout(() => {
    liveSaveTimers.delete(form);
    const fd = new FormData(form);
    fd.set("silent", "true");
    postServerActionSilent("SaveBookSettings", fd);
  }, 250);
  liveSaveTimers.set(form, timer);
}

let historyDragState = null;
let suppressNextHistoryClickUntil = 0;

function updateHistoryScrollCue() {
  const scrollEl = typeof qs === "function" ? qs("#history-graph-scroll") : null;
  const cueEl = typeof qs === "function" ? qs("#history-scroll-more-cue") : null;
  if (!scrollEl || !cueEl) return;
  const canScrollMore = scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight > 6;
  cueEl.classList.toggle("hidden", !canScrollMore);
}

function clearHistoryDragHighlights() {
  qsa(".history-lane-cell.is-drop-target-rail").forEach((cell) => {
    cell.classList.remove("is-drop-target-rail");
  });
  qsa(".history-graph-row.is-dragging-source").forEach((row) => {
    row.classList.remove("is-dragging-source");
  });
  const previewRow = qs("#history-drop-preview-row");
  if (previewRow) {
    previewRow.classList.add("hidden");
  }
}

function bindHistoryGraphInteractions() {
  if (typeof document === "undefined") return;

  document.addEventListener(
    "scroll",
    (e) => {
      if (e.target && e.target.id === "history-graph-scroll") {
        updateHistoryScrollCue();
      }
    },
    true
  );

  if (typeof window !== "undefined") {
    window.addEventListener("resize", () => {
      updateHistoryScrollCue();
    });
  }

  document.addEventListener(
    "click",
    (e) => {
      if (Date.now() < suppressNextHistoryClickUntil) {
        e.preventDefault();
        e.stopPropagation();
        suppressNextHistoryClickUntil = 0;
      }
    },
    true
  );

  document.addEventListener("pointerdown", (e) => {
    if (e.button !== 0 || !e.target || !e.target.closest) return;
    const handle = e.target.closest(".history-row-grip, .history-rail-node");
    if (!handle) return;
    const rowEl = handle.closest(".history-graph-row");
    if (!rowEl) return;
    const revID = (rowEl.getAttribute("data-revision") || "").trim();
    const sourceBranch = (rowEl.getAttribute("data-branch") || "").trim();
    const label = (rowEl.getAttribute("data-label") || "").trim();
    if (!revID) return;

    historyDragState = {
      revID: revID,
      sourceBranch: sourceBranch,
      label: label || revID,
      isCurrentNode: Boolean(handle.classList.contains("is-current-node")),
      startX: e.clientX,
      startY: e.clientY,
      moved: false,
      targetBranch: "",
      targetRevID: "",
      sourceRowEl: rowEl
    };
  });

  document.addEventListener("pointermove", (e) => {
    if (!historyDragState) return;
    if (!historyDragState.moved) {
      const dist = Math.hypot(e.clientX - historyDragState.startX, e.clientY - historyDragState.startY);
      if (dist < 5) return;
      historyDragState.moved = true;
      if (historyDragState.sourceRowEl) {
        historyDragState.sourceRowEl.classList.add("is-dragging-source");
      }
      if (typeof hideUITooltip === "function") {
        hideUITooltip();
      }
    }

    const hitEl = document.elementFromPoint(e.clientX, e.clientY);
    let hoverBranch = "";
    let hoverRevID = "";
    if (hitEl && hitEl.closest) {
      const laneCell = hitEl.closest(".history-lane-cell[data-branch]");
      const rowHit = hitEl.closest(".history-graph-row[data-branch]");
      if (laneCell) {
        hoverBranch = (laneCell.getAttribute("data-branch") || "").trim();
      } else if (rowHit) {
        hoverBranch = (rowHit.getAttribute("data-branch") || "").trim();
      }
      if (rowHit) {
        hoverRevID = (rowHit.getAttribute("data-revision") || "").trim();
      }
    }

    historyDragState.targetBranch = hoverBranch;
    historyDragState.targetRevID = hoverRevID;

    qsa(".history-lane-cell.is-drop-target-rail").forEach((cell) => {
      cell.classList.remove("is-drop-target-rail");
    });
    if (hoverBranch) {
      qsa(".history-lane-cell[data-branch]").forEach((cell) => {
        if ((cell.getAttribute("data-branch") || "").trim() === hoverBranch) {
          cell.classList.add("is-drop-target-rail");
        }
      });
    }

    const previewRow = qs("#history-drop-preview-row");
    const previewTitle = qs("#history-drop-preview-title");
    const previewMeta = qs("#history-drop-preview-meta");
    if (previewRow) {
      const showPreview = Boolean(hoverBranch && (hoverBranch !== historyDragState.sourceBranch || hoverRevID !== historyDragState.revID));
      previewRow.classList.toggle("hidden", !showPreview);
      if (showPreview) {
        if (previewTitle) previewTitle.textContent = historyDragState.label;
        if (previewMeta) previewMeta.textContent = hoverBranch;
      }
    }
  });

  const finishHistoryDrag = () => {
    if (!historyDragState) return;
    const state = historyDragState;
    historyDragState = null;
    clearHistoryDragHighlights();
    if (!state.moved) return;

    suppressNextHistoryClickUntil = Date.now() + 260;
    if (state.isCurrentNode) {
      if (state.targetBranch && state.targetBranch !== state.sourceBranch) {
        sendServerAction("SwitchHistoryBranch", { branch: state.targetBranch });
      } else if (state.targetRevID && state.targetRevID !== state.revID) {
        sendServerAction("RestoreHistoryRevision", { revision: state.targetRevID });
      }
      return;
    }
    if (state.targetBranch) {
      sendServerAction("CopyHistoryRevisionToBranch", {
        revision: state.revID,
        target_branch: state.targetBranch
      });
    }
  };

  document.addEventListener("pointerup", finishHistoryDrag);
  document.addEventListener("pointercancel", finishHistoryDrag);
}

bindHistoryGraphInteractions();

function parseBookVarsSpec(specStr) {
  const map = new Map();
  if (!specStr) return map;
  specStr.split(";").forEach((entry) => {
    if (!entry) return;
    const parts = entry.split(":");
    const name = (parts[0] || "").trim().toLowerCase();
    if (!name) return;
    const kind = (parts[1] || "bool").trim().toLowerCase();
    const opts = (parts[2] || "")
      .split(",")
      .map((o) => o.trim().toLowerCase())
      .filter(Boolean);
    map.set(name, { kind, opts });
  });
  return map;
}

function isClientBoolLiteral(s) {
  const v = (s || "").trim().toLowerCase();
  return v === "true" || v === "false" || v === "tak" || v === "nie" || v === "yes" || v === "no" || v === "on" || v === "off";
}

function explainClientVarValueError(varName, vDef, op, rhs, varsMap) {
  const cleanRhs = (rhs || "").replace(/^["']|["']$/g, "").trim().toLowerCase();
  if (!cleanRhs) {
    return "Błąd składni: brak wartości po operatorze '" + op + "' dla zmiennej '" + varName + "'";
  }
  if (vDef.kind === "int") {
    if (/^-?\d+$/.test(cleanRhs)) return "";
    const rhsVar = varsMap.get(cleanRhs);
    if (rhsVar && rhsVar.kind === "int") return "";
    return "Błąd typu: zmienna '" + varName + "' (int) wymaga liczby całkowitej, otrzymano '" + cleanRhs + "'";
  }
  if (vDef.kind === "enum") {
    if (op !== "==" && op !== "!=" && op !== "=") {
      return "Błąd operatora: zmienna '" + varName + "' (enum) obsługuje tylko '=' / '==' lub '!='";
    }
    if (vDef.opts.includes(cleanRhs)) return "";
    const rhsVar = varsMap.get(cleanRhs);
    if (rhsVar && rhsVar.kind === "enum") return "";
    return "Nieznana wartość '" + cleanRhs + "' dla zmiennej '" + varName + "' (dozwolone: " + vDef.opts.join(", ") + ")";
  }
  if (op !== "==" && op !== "!=" && op !== "=") {
    return "Błąd operatora: zmienna '" + varName + "' (bool) obsługuje tylko '=' / '==' lub '!='";
  }
  if (isClientBoolLiteral(cleanRhs)) return "";
  if (cleanRhs.startsWith("!")) {
    const negVar = varsMap.get(cleanRhs.slice(1).trim());
    if (negVar && negVar.kind === "bool") return "";
  }
  const rhsVar = varsMap.get(cleanRhs);
  if (rhsVar && rhsVar.kind === "bool") return "";
  return "Błąd typu: zmienna '" + varName + "' (bool) wymaga wartości true/false lub samej nazwy zmiennej";
}

function explainClientCondExprError(raw, varsMap) {
  let s = (raw || "").trim();
  if (s.startsWith("{{") && s.endsWith("}}") && s.length >= 4) {
    s = s.slice(2, -2).trim();
  }
  if (s.startsWith("?")) s = s.slice(1).trim();
  if (!s) return "";
  if (varsMap.size === 0) return "Brak zadeklarowanych zmiennych w książce";
  let depth = 0;
  for (let i = 0; i < s.length; i++) {
    if (s[i] === "(") depth++;
    else if (s[i] === ")") {
      depth--;
      if (depth < 0) return "Błąd składni: niedomknięty nawias w warunku";
    }
  }
  if (depth !== 0) return "Błąd składni: niedomknięty nawias w warunku";
  const clauses = s.split(/\|\||&&/);
  for (let i = 0; i < clauses.length; i++) {
    let c = clauses[i].replace(/^[()\s]+|[()\s]+$/g, "");
    while ((c.startsWith("!") && !c.startsWith("!=")) || c.startsWith("?")) {
      c = c.slice(1).replace(/^[()\s]+|[()\s]+$/g, "");
    }
    if (!c) return "Błąd składni: brak wyrażenia po operatorze logicznym";
    const mutOps = ["++", "--", "+=", "-="];
    for (let m = 0; m < mutOps.length; m++) {
      if (c.indexOf(mutOps[m]) >= 0) {
        return "Błąd w if: operator '" + mutOps[m] + "' służy do zmiany wartości w polu set:, a w if: podaj porównanie";
      }
    }
    if (c.indexOf(",") >= 0 || c.indexOf(";") >= 0) {
      return "Błąd składni w if: użyj '&&' (oraz) lub '||' (lub) zamiast przecinka/średnika";
    }
    const ops = ["==", "!=", ">=", "\x3c=", ">", "\x3c", "="];
    let matchedOp = "";
    let opIdx = -1;
    for (let j = 0; j < ops.length; j++) {
      const idx = c.indexOf(ops[j]);
      if (idx >= 0) {
        matchedOp = ops[j];
        opIdx = idx;
        break;
      }
    }
    if (opIdx >= 0) {
      const lhs = c.slice(0, opIdx).trim().toLowerCase();
      const rhs = c.slice(opIdx + matchedOp.length).trim();
      if (!lhs) return "Błąd składni: brak nazwy zmiennej przed operatorem '" + matchedOp + "'";
      if (!rhs) return "Błąd składni: brak wartości po operatorze '" + matchedOp + "' dla zmiennej '" + lhs + "'";
      const vDef = varsMap.get(lhs);
      if (!vDef) return "Nieznana zmienna '" + lhs + "' — zadeklaruj ją w panelu Zmienne {}";
      const errMsg = explainClientVarValueError(lhs, vDef, matchedOp, rhs, varsMap);
      if (errMsg) return errMsg;
    } else {
      const varName = c.trim().toLowerCase();
      if (/[\s=!<>+\-(),:|&]/.test(varName)) {
        return "Błąd składni w warunku: '" + c.trim() + "'";
      }
      if (!varsMap.has(varName)) {
        return "Nieznana zmienna '" + varName + "' — zadeklaruj ją w panelu Zmienne {}";
      }
    }
  }
  return "";
}

function validateClientCondExpr(raw, varsMap) {
  return explainClientCondExprError(raw, varsMap) === "";
}

function splitClientSpaceEffectChunk(chunk) {
  if (
    chunk.includes("==") ||
    chunk.includes("!=") ||
    chunk.includes(">=") ||
    chunk.includes("\x3c=") ||
    chunk.includes(">") ||
    chunk.includes("\x3c") ||
    chunk.includes("&&") ||
    chunk.includes("||")
  ) {
    return [chunk];
  }
  let norm = "";
  for (let i = 0; i < chunk.length; i++) {
    const ch = chunk.charAt(i);
    if ((ch === "+" || ch === "-") && i + 1 < chunk.length && chunk.charAt(i + 1) === "=") {
      norm += " " + ch + "= ";
      i++;
      continue;
    }
    if (ch === "=") {
      norm += " = ";
      continue;
    }
    norm += ch;
  }
  const tokens = norm.trim().split(/\s+/).filter(Boolean);
  if (tokens.length <= 1) return [chunk];
  const clauses = [];
  for (let i = 0; i < tokens.length; ) {
    const tok = tokens[i];
    if (i + 1 < tokens.length && (tokens[i + 1] === "=" || tokens[i + 1] === "+=" || tokens[i + 1] === "-=")) {
      const op = tokens[i + 1];
      if (i + 2 >= tokens.length) {
        clauses.push(tok + " " + op);
        break;
      }
      let clause = tok + " " + op + " " + tokens[i + 2];
      i += 3;
      while (i < tokens.length && /^-?\d+$/.test(tokens[i])) {
        clause += " " + tokens[i];
        i++;
      }
      clauses.push(clause);
      continue;
    }
    if (tok === "!" && i + 1 < tokens.length) {
      clauses.push("!" + tokens[i + 1]);
      i += 2;
      continue;
    }
    if ((tok.startsWith("+") || tok.startsWith("-")) && !tok.endsWith("++") && !tok.endsWith("--")) {
      let clause = tok;
      i++;
      if ((tok === "+" || tok === "-") && i < tokens.length) {
        clause += tokens[i];
        i++;
      }
      if (i < tokens.length && /^-?\d+$/.test(tokens[i])) {
        clause += " " + tokens[i];
        i++;
      }
      clauses.push(clause);
      continue;
    }
    clauses.push(tok);
    i++;
  }
  return clauses;
}

function explainClientEffectsExprError(raw, varsMap) {
  let s = (raw || "").trim();
  if (s.startsWith("{{") && s.endsWith("}}") && s.length >= 4) {
    s = s.slice(2, -2).trim();
  }
  if (!s) return "";
  if (varsMap.size === 0) return "Brak zadeklarowanych zmiennych w książce";
  const rawParts = s.split(/[;,]/).map((p) => p.trim());
  if (rawParts.length === 0) return "Błąd składni w polu set:";
  const parts = [];
  for (let r = 0; r < rawParts.length; r++) {
    const rp = rawParts[r];
    if (!rp) {
      if (r === rawParts.length - 1 && s.endsWith(";")) continue;
      return "Błąd składni w set: puste wyrażenie po przecinku";
    }
    const sub = splitClientSpaceEffectChunk(rp);
    for (let k = 0; k < sub.length; k++) {
      parts.push(sub[k]);
    }
  }
  for (let i = 0; i < parts.length; i++) {
    const c = parts[i];
    if (!c) {
      return "Błąd składni w set: puste wyrażenie po przecinku";
    }
    if ((c.endsWith("++") || c.endsWith("--")) && !c.includes("=")) {
      const op = c.slice(-2);
      const varName = c.slice(0, -2).trim().toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return "Nieznana zmienna '" + varName + "' — zadeklaruj ją w panelu Zmienne {}";
      if (vDef.kind !== "int") {
        return "Błąd typu: operator '" + op + "' wymaga zmiennej typu int (zmienna '" + varName + "' jest typu " + vDef.kind + ")";
      }
      continue;
    }
    if (c.startsWith("!") && !c.includes("=")) {
      const varName = c.slice(1).trim().toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return "Nieznana zmienna '" + varName + "' — zadeklaruj ją w panelu Zmienne {}";
      if (vDef.kind !== "bool") {
        return "Błąd typu: negacja '!" + varName + "' wymaga zmiennej typu bool (jest typu " + vDef.kind + ")";
      }
      continue;
    }
    if ((c.startsWith("+") || c.startsWith("-")) && !c.includes("=")) {
      const tokens = c.slice(1).trim().split(/\s+/).filter(Boolean);
      if (tokens.length === 0 || tokens.length > 2) return "Błąd składni w set: '" + c + "'";
      const varName = tokens[0].toLowerCase();
      const vDef = varsMap.get(varName);
      if (!vDef) return "Nieznana zmienna '" + varName + "' — zadeklaruj ją w panelu Zmienne {}";
      if (vDef.kind !== "bool" && vDef.kind !== "int") {
        return "Błąd typu: dla zmiennej '" + varName + "' (enum) użyj przypisania '='";
      }
      if (tokens.length === 2 && (vDef.kind !== "int" || !/^-?\d+$/.test(tokens[1]))) {
        return "Błąd typu: zmienna '" + varName + "' wymaga liczby całkowitej";
      }
      continue;
    }
    if (c.includes("==") || c.includes("!=") || c.includes(">=") || c.includes("\x3c=") || c.includes(">") || c.includes("\x3c") || c.includes("&&") || c.includes("||")) {
      return "Błąd w set: użyj przypisania ('=', '+=', '-=', '++', '--'), a nie operatora warunkowego";
    }
    const plusEq = c.indexOf("+=");
    const minusEq = c.indexOf("-=");
    if (plusEq > 0 || minusEq > 0) {
      const idx = plusEq > 0 ? plusEq : minusEq;
      const op = plusEq > 0 ? "+=" : "-=";
      const lhs = c.slice(0, idx).trim().toLowerCase();
      const rhs = c.slice(idx + 2).trim().toLowerCase();
      const vDef = varsMap.get(lhs);
      if (!vDef) return "Nieznana zmienna '" + lhs + "' — zadeklaruj ją w panelu Zmienne {}";
      if (vDef.kind !== "int") {
        return "Błąd typu: operator '" + op + "' wymaga zmiennej typu int (zmienna '" + lhs + "' jest typu " + vDef.kind + ")";
      }
      if (!rhs) return "Błąd składni: brak wartości po '" + op + "' dla zmiennej '" + lhs + "'";
      if (!/^-?\d+$/.test(rhs)) {
        const rhsVar = varsMap.get(rhs);
        if (!rhsVar || rhsVar.kind !== "int") {
          return "Błąd typu: zmienna '" + lhs + "' (int) wymaga liczby całkowitej po '" + op + "', otrzymano '" + rhs + "'";
        }
      }
      continue;
    }
    const eqIdx = c.indexOf("=");
    if (eqIdx > 0) {
      const lhs = c.slice(0, eqIdx).trim().toLowerCase();
      const rhs = c.slice(eqIdx + 1).trim();
      const vDef = varsMap.get(lhs);
      if (!vDef) return "Nieznana zmienna '" + lhs + "' — zadeklaruj ją w panelu Zmienne {}";
      if (!rhs) return "Błąd składni: brak przypisywanej wartości po '=' dla zmiennej '" + lhs + "'";
      const errMsg = explainClientVarValueError(lhs, vDef, "=", rhs, varsMap);
      if (errMsg) return errMsg;
      continue;
    }
    const bareName = c.toLowerCase();
    const bareDef = varsMap.get(bareName);
    if (bareDef) {
      if (bareDef.kind === "bool") continue;
      return "Błąd składni: dla zmiennej '" + bareName + "' (" + bareDef.kind + ") podaj przypisanie (np. " + bareName + " = wartość)";
    }
    if (!/[\s+\-*/=<>!&|,;:]/.test(bareName)) {
      return "Nieznana zmienna '" + bareName + "' — zadeklaruj ją w panelu Zmienne {}";
    }
    return "Błąd składni w set: '" + c + "'";
  }
  return "";
}

function validateClientEffectsExpr(raw, varsMap) {
  return explainClientEffectsExprError(raw, varsMap) === "";
}

function validateRuleInputElement(inputEl, form) {
  if (!inputEl || !form) return;
  const name = inputEl.name || "";
  const isCond = name.indexOf("choice_cond_") === 0;
  const isEff = name.indexOf("choice_effects_") === 0 || name === "scene_effects";
  if (!isCond && !isEff) return;
  const isEmpty = !(inputEl.value || "").trim();
  const emptyRow = inputEl.closest(".choice-triple-row, .node-meta-row, .scene-settings-row");
  if (emptyRow) {
    emptyRow.classList.toggle("is-empty-rule", isEmpty);
  }
  const varsMap = parseBookVarsSpec(form.getAttribute("data-book-vars") || "");
  const errMsg = isCond
    ? explainClientCondExprError(inputEl.value, varsMap)
    : explainClientEffectsExprError(inputEl.value, varsMap);
  const valid = !errMsg;
  const wrap = inputEl.closest(".choice-triple-row, .syn-input-wrap");
  if (wrap) {
    wrap.classList.toggle("is-invalid", !valid);
  }
  if (errMsg) {
    inputEl.setAttribute("data-hint", errMsg);
    if (wrap) wrap.setAttribute("data-hint", errMsg);
    if (typeof activeHintEl !== "undefined" && (activeHintEl === inputEl || activeHintEl === wrap || document.activeElement === inputEl)) {
      activeHintEl = inputEl;
      if (typeof positionAndShowUITooltip === "function") {
        positionAndShowUITooltip(inputEl);
      }
    }
  } else {
    inputEl.removeAttribute("data-hint");
    if (wrap) wrap.removeAttribute("data-hint");
    if (typeof activeHintEl !== "undefined" && (activeHintEl === inputEl || activeHintEl === wrap)) {
      if (typeof hideUITooltip === "function") {
        hideUITooltip();
      }
    }
  }
}

let activeSynRuleInput = null;
let lastSynRuleInputName = "";
let lastRulePointerWasTouch = false;

function stripOuterBracesClient(s) {
  let v = (s || "").trim();
  if (v.startsWith("{{") && v.endsWith("}}") && v.length >= 4) {
    v = v.slice(2, -2).trim();
  }
  return v;
}

function extractCurrentTypingToken(val, cursorPos) {
  const before = val.slice(0, cursorPos);
  const m = before.match(/([a-z0-9_ąćęłńóśźż]+)$/i);
  return m ? m[1].toLowerCase() : "";
}

function isAtFreshOperandSlot(val, cursorPos) {
  const before = val.slice(0, cursorPos);
  return /^\s*[!?+\-]?\s*$/.test(before) || /(?:&&|\|\||;|,)\s*[!?+\-]?\s*$/.test(before);
}

function replaceCurrentTypingToken(inputEl, varName) {
  let val = inputEl.value || "";
  if (!val.trim()) {
    inputEl.value = varName;
    const p = varName.length;
    if (typeof inputEl.setSelectionRange === "function") inputEl.setSelectionRange(p, p);
    return;
  }
  const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
  const before = val.slice(0, pos);
  const after = val.slice(pos).replace(/^[a-z0-9_ąćęłńóśźż]*/i, "");
  const prefix = before.replace(/\?\s*[a-z0-9_ąćęłńóśźż]*$|[a-z0-9_ąćęłńóśźż]+$/i, "");
  let sep = "";
  if (prefix.length > 0 && !/\s$/.test(prefix) && !/[(!]$/.test(prefix) && !/(?:^|[\s;(,])[-+]$/.test(prefix)) {
    sep = " ";
  }
  const replacedBefore = prefix + sep + varName;
  const needTrailingSpace = after.length > 0 && /^[a-z0-9_ąćęłńóśźż!?]/i.test(after);
  inputEl.value = replacedBefore + (needTrailingSpace ? " " : "") + after;
  const newPos = replacedBefore.length;
  if (typeof inputEl.setSelectionRange === "function") {
    inputEl.setSelectionRange(newPos, newPos);
  }
}

function getVisibleSynDropdownItems(dropdown) {
  if (!dropdown) return [];
  return qsa(".syn-dropdown-item", dropdown).filter((btn) => !btn.classList.contains("hidden"));
}

function setSynDropdownActiveIndex(dropdown, nextIdx) {
  if (!dropdown) return;
  const visibleItems = getVisibleSynDropdownItems(dropdown);
  qsa(".syn-dropdown-item", dropdown).forEach((btn) => btn.classList.remove("active"));
  if (visibleItems.length === 0) return;
  const clamped = ((nextIdx % visibleItems.length) + visibleItems.length) % visibleItems.length;
  const activeBtn = visibleItems[clamped];
  if (activeBtn) {
    activeBtn.classList.add("active");
    if (typeof activeBtn.scrollIntoView === "function") {
      activeBtn.scrollIntoView({ block: "nearest" });
    }
  }
}

function moveSynDropdownSelection(dropdown, delta) {
  if (!dropdown) return;
  const visibleItems = getVisibleSynDropdownItems(dropdown);
  if (visibleItems.length === 0) return;
  let curIdx = -1;
  for (let i = 0; i < visibleItems.length; i++) {
    if (visibleItems[i].classList.contains("active")) {
      curIdx = i;
      break;
    }
  }
  if (curIdx < 0) {
    setSynDropdownActiveIndex(dropdown, delta >= 0 ? 0 : visibleItems.length - 1);
  } else {
    setSynDropdownActiveIndex(dropdown, curIdx + delta);
  }
}

function syncChoiceInlineHighlight(inputEl) {
  if (!inputEl) return;
  const val = inputEl.value || "";
  const arrowIdx = val.lastIndexOf("->");
  const hasArrow = arrowIdx >= 0 && (arrowIdx === 0 || val.charAt(arrowIdx - 1) !== "\\");
  const wrap = inputEl.closest ? inputEl.closest(".choice-inline-wrap") : null;
  if (wrap) {
    const hlLabel = qs(".choice-hl-label", wrap);
    const hlArrow = qs(".choice-hl-arrow", wrap);
    if (hasArrow) {
      if (hlLabel) hlLabel.textContent = val.slice(0, arrowIdx);
      if (hlArrow) hlArrow.textContent = val.slice(arrowIdx);
    } else {
      if (hlLabel) hlLabel.textContent = val;
      if (hlArrow) hlArrow.textContent = "";
    }
  }
  const choiceIdx = inputEl.getAttribute ? inputEl.getAttribute("data-choice-idx") : null;
  const form = inputEl.closest ? inputEl.closest("form.editor-context-form, form#form-scene-text") : null;
  if (form && choiceIdx !== null && choiceIdx !== "" && hasArrow) {
    const tgtInput = qs('input[name="choice_target_' + choiceIdx + '"]', form);
    if (tgtInput) {
      const parsedTgt = val.slice(arrowIdx + 2).trim();
      tgtInput.value = parsedTgt || "END";
    }
  }
}

function replaceChoiceTargetToken(inputEl, targetVal) {
  if (!inputEl) return;
  if (inputEl.classList && inputEl.classList.contains("choice-exit-input")) {
    inputEl.value = targetVal;
    const newPos = targetVal.length;
    if (typeof inputEl.setSelectionRange === "function") {
      inputEl.setSelectionRange(newPos, newPos);
    }
    return;
  }
  const val = inputEl.value || "";
  const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
  const before = val.slice(0, pos);
  const after = val.slice(pos);
  const replacedBefore = before.replace(/->\s*([a-z0-9_ąćęłńóśźż:\-]*)$/i, "-> " + targetVal);
  inputEl.value = replacedBefore + after;
  const newPos = replacedBefore.length;
  if (typeof inputEl.setSelectionRange === "function") {
    inputEl.setSelectionRange(newPos, newPos);
  }
  syncChoiceInlineHighlight(inputEl);
}

function updateChoiceTargetAutocompleteOnInput(inputEl, forceAll) {
  if (!inputEl || typeof document === "undefined") return;
  const isExitInput = Boolean(inputEl.classList && inputEl.classList.contains("choice-exit-input"));
  if (!isExitInput) {
    syncChoiceInlineHighlight(inputEl);
  }
  const form = inputEl.closest("form.editor-context-form, form#form-scene-text");
  if (!form) return;
  const dropdown = qs(".syn-autocomplete-dropdown", form);
  if (!dropdown) return;
  activeSynRuleInput = inputEl;
  lastSynRuleInputName = inputEl.name || "";

  const val = inputEl.value || "";
  let prefix = "";
  if (isExitInput) {
    prefix = val.trim().toLowerCase();
    if (!forceAll && prefix === "") {
      dropdown.classList.add("hidden");
      return;
    }
  } else {
    const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
    const before = val.slice(0, pos);
    const m = before.match(/->\s*([a-z0-9_ąćęłńóśźż:\-]*)$/i);
    if (!m) {
      dropdown.classList.add("hidden");
      return;
    }
    prefix = (m[1] || "").toLowerCase();
  }
  const items = qsa(".syn-dropdown-item", dropdown);
  let matchCount = 0;
  items.forEach((btn) => {
    const kind = btn.getAttribute("data-syn-kind") || "var";
    if (kind !== "target") {
      btn.classList.add("hidden");
      return;
    }
    const tVal = (btn.getAttribute("data-syn-target-value") || "").toLowerCase();
    const show = prefix === "" || (tVal.startsWith(prefix) && tVal !== prefix);
    btn.classList.toggle("hidden", !show);
    if (show) matchCount++;
  });
  if (matchCount === 0) {
    dropdown.classList.add("hidden");
    return;
  }
  const anchor = inputEl.closest(".choice-triple-row, .choice-inline-wrap");
  if (anchor && dropdown.parentNode !== anchor) {
    anchor.appendChild(dropdown);
  }
  dropdown.classList.remove("hidden");
  setSynDropdownActiveIndex(dropdown, 0);
}

function updateSynAutocompleteOnInput(inputEl, forceAll) {
  if (!inputEl || typeof document === "undefined") return;
  if (inputEl.id === "modal-scene-textarea") return;
  const form = inputEl.closest("form.editor-context-form, form#form-scene-text");
  if (!form) return;
  const dropdown = qs(".syn-autocomplete-dropdown", form);
  if (!dropdown) return;
  activeSynRuleInput = inputEl;
  lastSynRuleInputName = inputEl.name || "";

  const val = inputEl.value || "";
  const pos = typeof inputEl.selectionStart === "number" ? inputEl.selectionStart : val.length;
  const beforeCursor = val.slice(0, pos);
  const lastNewline = beforeCursor.lastIndexOf("\n");
  const currentLineBefore = lastNewline >= 0 ? beforeCursor.slice(lastNewline + 1) : beforeCursor;

  const varsMap = parseBookVarsSpec(form.getAttribute("data-book-vars") || "");
  const enumSlotMatch = currentLineBefore.match(/([a-z0-9_ąćęłńóśźż]+)\s*(?:==|!=|=)\s*(\??\s*[a-z0-9_ąćęłńóśźż]*)$/i);
  const items = qsa(".syn-dropdown-item", dropdown);

  if (enumSlotMatch) {
    const enumVarName = (enumSlotMatch[1] || "").toLowerCase();
    const vDef = varsMap.get(enumVarName);
    if (vDef && vDef.kind === "enum") {
      const rhsRaw = (enumSlotMatch[2] || "").trim();
      const hasQuestion = rhsRaw.startsWith("?");
      const enumPrefix = rhsRaw.replace(/^\?\s*/, "").toLowerCase();
      if (!forceAll && !hasQuestion && !enumPrefix && !lastRulePointerWasTouch) {
        dropdown.classList.add("hidden");
        return;
      }
      let matchCount = 0;
      items.forEach((btn) => {
        const kind = btn.getAttribute("data-syn-kind") || "var";
        const btnEnumVar = (btn.getAttribute("data-syn-enum-var") || "").toLowerCase();
        const optVal = (btn.getAttribute("data-syn-var-name") || "").toLowerCase();
        if (kind !== "enum_val" || btnEnumVar !== enumVarName) {
          btn.classList.add("hidden");
          return;
        }
        const show = forceAll || hasQuestion || !enumPrefix || (optVal.startsWith(enumPrefix) && optVal !== enumPrefix);
        btn.classList.toggle("hidden", !show);
        if (show) matchCount++;
      });
      if (matchCount === 0) {
        dropdown.classList.add("hidden");
        return;
      }
      const anchor = inputEl.closest(".choice-triple-row, .syn-input-wrap, .scene-body-block");
      if (anchor && dropdown.parentNode !== anchor) {
        anchor.appendChild(dropdown);
      }
      dropdown.classList.remove("hidden");
      setSynDropdownActiveIndex(dropdown, 0);
      return;
    }
  }

  const triggeredByQuestionMark = /\?\s*$/.test(currentLineBefore);
  const token = extractCurrentTypingToken(currentLineBefore, currentLineBefore.length);
  const touchSlot = lastRulePointerWasTouch && isAtFreshOperandSlot(currentLineBefore, currentLineBefore.length);

  if (!forceAll && !triggeredByQuestionMark && !touchSlot && !token) {
    dropdown.classList.add("hidden");
    return;
  }

  const usedVars = new Set();
  const prefixBeforeToken = currentLineBefore.replace(/\?\s*[a-z0-9_ąćęłńóśźż]*$|[a-z0-9_ąćęłńóśźż]+$/i, "");
  const afterToken = val.slice(pos).replace(/^[a-z0-9_ąćęłńóśźż]*/i, "");
  const restTokens = (prefixBeforeToken + " " + afterToken).toLowerCase().match(/[a-z0-9_ąćęłńóśźż]+/gi) || [];
  restTokens.forEach((tok) => {
    if (varsMap.has(tok)) usedVars.add(tok);
  });

  let matchCount = 0;
  items.forEach((btn) => {
    const kind = btn.getAttribute("data-syn-kind") || "var";
    if (kind !== "var") {
      btn.classList.add("hidden");
      return;
    }
    const vName = (btn.getAttribute("data-syn-var-name") || "").toLowerCase();
    if (usedVars.has(vName)) {
      btn.classList.add("hidden");
      return;
    }
    const show = forceAll || triggeredByQuestionMark || touchSlot || (token.length > 0 && vName.startsWith(token) && vName !== token);
    btn.classList.toggle("hidden", !show);
    if (show) matchCount++;
  });

  if (matchCount === 0) {
    dropdown.classList.add("hidden");
    return;
  }

  const anchor = inputEl.closest(".choice-triple-row, .syn-input-wrap, .scene-body-block");
  if (anchor && dropdown.parentNode !== anchor) {
    anchor.appendChild(dropdown);
  }
  dropdown.classList.remove("hidden");
  setSynDropdownActiveIndex(dropdown, 0);
}

function bindSynAutocompleteDropdown() {
  if (typeof document === "undefined") return;

  document.addEventListener("focusin", (e) => {
    const t = e.target;
    if (!t || !t.matches) return;
    if (t.matches("input.choice-rule-input, input.syn-inner-input")) {
      if (lastRulePointerWasTouch) {
        updateSynAutocompleteOnInput(t, !(t.value || "").trim());
      }
    } else if (t.matches("input.choice-exit-input")) {
      if (lastRulePointerWasTouch) {
        updateChoiceTargetAutocompleteOnInput(t, true);
      }
    }
  });

  document.addEventListener("focusout", (e) => {
    const t = e.target;
    if (!t || !t.matches) return;
    if (t.matches("input.choice-rule-input, input.syn-inner-input")) {
      const inner = stripOuterBracesClient(t.value);
      t.value = inner;
      const emptyRow = t.closest(".choice-triple-row, .node-meta-row, .scene-settings-row");
      if (emptyRow) {
        emptyRow.classList.toggle("is-empty-rule", !inner);
      }
    } else if (t.matches("input.node-choice-label, textarea.node-choice-label, input.choice-label-text, textarea.choice-label-text")) {
      syncChoiceInlineHighlight(t);
    }
  });

  document.addEventListener("input", (e) => {
    const t = e.target;
    if (!t || !t.matches) return;
    if (t.matches("input.choice-rule-input, input.syn-inner-input")) {
      updateSynAutocompleteOnInput(t, false);
    } else if (t.matches("input.choice-exit-input, input.node-choice-label, textarea.node-choice-label, input.choice-label-text, textarea.choice-label-text")) {
      updateChoiceTargetAutocompleteOnInput(t, false);
    }
  });

  document.addEventListener("keydown", (e) => {
    const t = e.target;
    if (!t || !t.matches || !t.matches("input.choice-exit-input, input.choice-rule-input, input.syn-inner-input, input.node-choice-label, textarea.node-choice-label, input.choice-label-text, textarea.choice-label-text")) return;
    const form = t.closest("form.editor-context-form, form#form-scene-text");
    const dropdown = form ? qs(".syn-autocomplete-dropdown", form) : null;
    if (!dropdown || dropdown.classList.contains("hidden")) {
      if (e.key === "ArrowDown") {
        if (t.matches("input.choice-rule-input, input.syn-inner-input")) {
          e.preventDefault();
          e.stopPropagation();
          updateSynAutocompleteOnInput(t, true);
          return;
        }
        if (t.matches("input.choice-exit-input")) {
          e.preventDefault();
          e.stopPropagation();
          updateChoiceTargetAutocompleteOnInput(t, true);
          return;
        }
      }
      if (e.key === "Enter" && t.matches("textarea.node-choice-label, textarea.choice-label-text")) {
        e.preventDefault();
        t.blur();
      }
      return;
    }
    const visibleItems = getVisibleSynDropdownItems(dropdown);
    if (visibleItems.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      e.stopPropagation();
      moveSynDropdownSelection(dropdown, 1);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      e.stopPropagation();
      moveSynDropdownSelection(dropdown, -1);
      return;
    }
    if (e.key === "Tab" || e.key === "Enter") {
      e.preventDefault();
      e.stopPropagation();
      const chosenBtn = visibleItems.find((btn) => btn.classList.contains("active")) || visibleItems[0];
      const kind = chosenBtn.getAttribute("data-syn-kind") || "var";
      if (kind === "target") {
        const chosenTarget = chosenBtn.getAttribute("data-syn-target-value") || "";
        if (chosenTarget) {
          replaceChoiceTargetToken(t, chosenTarget);
          dropdown.classList.add("hidden");
          if (typeof scheduleNodeLiveSave === "function") scheduleNodeLiveSave(form);
        }
      } else {
        const chosen = chosenBtn.getAttribute("data-syn-var-name") || "";
        if (chosen) {
          replaceCurrentTypingToken(t, chosen);
          dropdown.classList.add("hidden");
          validateRuleInputElement(t, form);
          if (typeof scheduleNodeLiveSave === "function") scheduleNodeLiveSave(form);
        }
      }
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      dropdown.classList.add("hidden");
    }
  });

  document.addEventListener("pointerover", (e) => {
    const t = e.target;
    if (!t || !t.closest) return;
    const itemBtn = t.closest(".syn-dropdown-item");
    if (!itemBtn || itemBtn.classList.contains("hidden")) return;
    const dropdown = itemBtn.closest(".syn-autocomplete-dropdown");
    if (!dropdown || dropdown.classList.contains("hidden")) return;
    const visibleItems = getVisibleSynDropdownItems(dropdown);
    const idx = visibleItems.indexOf(itemBtn);
    if (idx >= 0) {
      setSynDropdownActiveIndex(dropdown, idx);
    }
  });

  document.addEventListener("pointerdown", (e) => {
    lastRulePointerWasTouch = e.pointerType === "touch";
    const t = e.target;
    if (!t || !t.closest) return;

    const itemBtn = t.closest(".syn-dropdown-item");
    if (itemBtn && activeSynRuleInput) {
      e.preventDefault();
      e.stopPropagation();
      const kind = itemBtn.getAttribute("data-syn-kind") || "var";
      if (kind === "target") {
        const targetVal = itemBtn.getAttribute("data-syn-target-value") || "";
        replaceChoiceTargetToken(activeSynRuleInput, targetVal);
      } else {
        const varName = itemBtn.getAttribute("data-syn-var-name") || "";
        replaceCurrentTypingToken(activeSynRuleInput, varName);
      }
      const dropdown = itemBtn.closest(".syn-autocomplete-dropdown");
      if (dropdown) dropdown.classList.add("hidden");
      const form = activeSynRuleInput.closest("form.editor-context-form, form#form-scene-text");
      if (form) {
        if (kind !== "target") {
          validateRuleInputElement(activeSynRuleInput, form);
        }
        if (typeof scheduleNodeLiveSave === "function") {
          scheduleNodeLiveSave(form);
        }
      }
      activeSynRuleInput.focus();
      return;
    }

    if (!t.closest(".syn-autocomplete-dropdown")) {
      qsa(".syn-autocomplete-dropdown").forEach((d) => d.classList.add("hidden"));
    }
  });
}

bindSynAutocompleteDropdown();
