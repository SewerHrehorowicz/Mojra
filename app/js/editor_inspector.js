"use strict";

let liveSaveTimers = new WeakMap();
let dirtyInspectorForms = new WeakSet();
let chapterLiveSaveTimers = new Map();
let dirtyChapterInputs = new Set();
let bookTitleSaveTimer = null;
let dirtyBookTitleInput = null;

function syncBookTitleDOM(newTitle) {
  const clean = (newTitle || "").trim();
  if (!clean) return;
  const playerTab = qs("#tab-player");
  if (playerTab) {
    playerTab.setAttribute("title", clean);
    const lbl = qs(".tab-label", playerTab);
    if (lbl) {
      lbl.textContent = clean;
    }
  }
}

function commitBookTitleInput(inputEl) {
  if (!inputEl) return;
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
  if (!inputEl) return;
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
    for (let i = 0; i < studioGraphState.nodes.length; i++) {
      const nd = studioGraphState.nodes[i];
      if (nd.chapterLower === prevLower) {
        nd.chapter = nextGroup;
        nd.chapterLower = nextLower;
      }
    }
    studioGraphState.staticBatchDirty = true;
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
  if (!inputEl) return;
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
  if (!inputEl) return;
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
  const panel = qs("#editor-context-panel");
  if (isAndroidStudioClient()) {
    if (panel) panel.classList.add("hidden");
    return;
  }
  const form = qs("#editor-context-form");
  const multiPanel = qs("#editor-multi-panel");
  if (panel) {
    panel.classList.remove("hidden");
  }
  if (multiPanel) {
    multiPanel.classList.add("hidden");
  }
  const currentPanelSceneId = form ? form.getAttribute("data-context-scene-id") || "" : "";
  if (panel && form && currentPanelSceneId === sceneId) {
    form.classList.remove("hidden");
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

function hideEditorContextPanel() {
  flushInspectorFormIfDirty();
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
  const multiPanel = qs("#editor-multi-panel");

  if (selectedNodeIds.size === 0) {
    flushInspectorFormIfDirty();
    if (form) form.classList.add("hidden");
    if (multiPanel) multiPanel.classList.add("hidden");
  } else if (selectedNodeIds.size === 1) {
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
    if (form) form.classList.add("hidden");
    if (multiPanel) {
      multiPanel.classList.remove("hidden");
      const countLabel = qs("#multi-select-count-label", multiPanel);
      if (countLabel) {
        countLabel.textContent = "Zaznaczone sceny (" + selectedItems.length + ")";
      }
      const listEl = qs("#context-multi-scene-list", multiPanel);
      if (listEl) {
        listEl.textContent = "";
        selectedItems.forEach((item) => {
          const row = document.createElement("div");
          row.className = "context-multi-scene-item";
          row.setAttribute("data-multi-scene-id", item.id);

          const infoBtn = document.createElement("button");
          infoBtn.type = "button";
          infoBtn.className = "context-multi-scene-select-btn";
          infoBtn.setAttribute("data-multi-select-scene", item.id);
          infoBtn.title = "Edytuj scenę: " + item.title;

          const titleSpan = document.createElement("span");
          titleSpan.className = "context-multi-scene-title";
          titleSpan.textContent = item.title;
          infoBtn.appendChild(titleSpan);

          if (item.chapter) {
            const metaSpan = document.createElement("span");
            metaSpan.className = "context-multi-scene-meta";
            metaSpan.textContent = item.chapter;
            infoBtn.appendChild(metaSpan);
          }

          const deselectBtn = document.createElement("button");
          deselectBtn.type = "button";
          deselectBtn.className = "node-icon-btn context-multi-scene-deselect-btn";
          deselectBtn.setAttribute("data-multi-deselect-scene", item.id);
          deselectBtn.title = "Odznacz scenę";
          deselectBtn.textContent = "✕";

          row.appendChild(infoBtn);
          row.appendChild(deselectBtn);
          listEl.appendChild(row);
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
  if (!form) return;
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
