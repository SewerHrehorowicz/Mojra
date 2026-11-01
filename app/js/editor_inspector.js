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
  ["#editor-book-title-input", "#book-settings-title-input"].forEach((sel) => {
    const inp = qs(sel);
    if (inp && document.activeElement !== inp) {
      inp.value = clean;
      inp.setAttribute("data-old-title", clean);
    }
  });
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
  /* @dev-only-start */
  const stage = typeof qs === "function" ? qs("#device-stage") : null;
  if (stage) {
    const mode = stage.getAttribute("data-preview");
    if (mode === "portrait" || mode === "landscape") return true;
  }
  /* @dev-only-end */
  return false;
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

function selectNodeForContextPanel(sceneId) {
  if (!sceneId) return;
  editorSelectionCleared = false;
  if (typeof studioGraphState !== "undefined") {
    studioGraphState.selectedChapterKey = "";
    studioGraphState.overlayBatchDirty = true;
  }
  const bookSettingsBtn = qs("#btn-book-settings");
  if (bookSettingsBtn) {
    bookSettingsBtn.classList.remove("active");
  }
  const panel = qs("#editor-context-panel");
  if (isAndroidStudioClient()) {
    if (panel) panel.classList.add("hidden");
    return;
  }
  const form = qs("#editor-context-form");
  const chapterForm = qs("#editor-chapter-form");
  const bookForm = qs("#editor-book-form");
  const multiPanel = qs("#editor-multi-panel");
  if (panel) {
    panel.classList.remove("hidden");
  }
  if (chapterForm) {
    chapterForm.classList.add("hidden");
  }
  if (bookForm) {
    bookForm.classList.add("hidden");
  }
  if (multiPanel) {
    multiPanel.classList.add("hidden");
  }
  const currentPanelSceneId = form ? form.getAttribute("data-context-scene-id") || "" : "";
  if (panel && form && !form.classList.contains("hidden") && currentPanelSceneId === sceneId) {
    return;
  }
  const params = { scene: sceneId };
  if (form && dirtyInspectorForms.has(form)) {
    const existingTimer = liveSaveTimers.get(form);
    if (existingTimer) {
      clearTimeout(existingTimer);
      liveSaveTimers.delete(form);
    }
    dirtyInspectorForms.delete(form);
    const fd = new FormData(form);
    fd.forEach((val, key) => {
      params[key] = val;
    });
  }
  sendServerAction("SelectStudioScene", params);
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
  syncNodeSelectionDOM();
  renderStudioCanvas();
  const panel = qs("#editor-context-panel");
  if (isAndroidStudioClient()) {
    if (panel) panel.classList.add("hidden");
    return;
  }
  const form = qs("#editor-context-form");
  const chapterForm = qs("#editor-chapter-form");
  const bookForm = qs("#editor-book-form");
  const multiPanel = qs("#editor-multi-panel");
  if (panel) {
    panel.classList.remove("hidden");
  }
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
    return;
  }
  if (chapterForm && currentChapter.toLowerCase() === cleanName.toLowerCase()) {
    chapterForm.classList.remove("hidden");
  }
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
  const panel = qs("#editor-context-panel");
  if (panel) {
    if (isAndroidStudioClient()) {
      panel.classList.add("hidden");
    } else {
      panel.classList.remove("hidden");
    }
  }
  const form = qs("#editor-context-form");
  if (form) {
    form.classList.add("hidden");
  }
  const chapterForm = qs("#editor-chapter-form");
  if (chapterForm) {
    chapterForm.classList.add("hidden");
  }
  const bookForm = qs("#editor-book-form");
  if (bookForm) {
    bookForm.classList.add("hidden");
  }
  const multiPanel = qs("#editor-multi-panel");
  if (multiPanel) {
    multiPanel.classList.add("hidden");
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
  if (panel) {
    if (isAndroidStudioClient()) {
      panel.classList.add("hidden");
    } else {
      panel.classList.remove("hidden");
    }
  }
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
    if (typeof studioGraphState !== "undefined") {
      studioGraphState.selectedChapterKey = "";
    }
    if (chapterForm) chapterForm.classList.add("hidden");
    if (bookForm) bookForm.classList.add("hidden");
    const bookSettingsBtn = qs("#btn-book-settings");
    if (bookSettingsBtn) bookSettingsBtn.classList.remove("active");
    if (multiPanel) multiPanel.classList.add("hidden");
    const onlyId = Array.from(selectedNodeIds)[0];
    const currentFormId = form ? form.getAttribute("data-context-scene-id") || "" : "";
    if (form) {
      if (currentFormId === onlyId) {
        form.classList.remove("hidden");
      } else {
        form.classList.add("hidden");
      }
    }
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
  const nextH = Math.max(44, (ta.scrollHeight || 0) + 4);
  ta.style.height = nextH + "px";
  ta.style.minHeight = nextH + "px";
  ta.scrollTop = 0;
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
