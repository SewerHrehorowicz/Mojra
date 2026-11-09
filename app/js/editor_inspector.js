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
  const playerTitle = qs("#topbar-book-title");
  if (playerTitle) {
    playerTitle.textContent = clean;
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

function syncSceneTitleAcrossForms(sceneId, newTitle, sourceEl) {
  if (!sceneId) return;
  const cleanTitle = (newTitle || "").trim();
  ["#form-scene-text", "#editor-context-form"].forEach((sel) => {
    const f = qs(sel);
    if (!f) return;
    const scInp = qs('input[name="scene_id"]', f);
    const fId = f.getAttribute("data-context-scene-id") || (scInp ? scInp.value : "") || "";
    if (fId === sceneId) {
      const nameInp = qs('input[name="name"]', f);
      if (nameInp && nameInp !== sourceEl && document.activeElement !== nameInp) {
        nameInp.value = cleanTitle;
      }
    }
  });
  const displayTitle = cleanTitle || sceneId;
  const bookForm = qs("#editor-book-form");
  if (bookForm) {
    qsa('[data-select-name="start_scene"] .custom-select-option', bookForm).forEach((opt) => {
      if ((opt.getAttribute("data-select-value") || "") === sceneId) {
        opt.setAttribute("data-select-label", displayTitle);
        opt.textContent = displayTitle;
      }
    });
    const startInp = qs('input[name="start_scene"]', bookForm);
    if (startInp && startInp.value === sceneId) {
      const lbl = qs('[data-select-name="start_scene"] .custom-select-label', bookForm);
      if (lbl) lbl.textContent = displayTitle;
    }
  }
  qsa('.studio-search-row[data-search-scene="' + CSS.escape(sceneId) + '"] .studio-search-row-title').forEach((el) => {
    el.textContent = displayTitle;
  });
}

function resolveCanvasSceneTargetId(rawTgt) {
  const clean = (typeof stripOuterBracesClient === "function" ? stripOuterBracesClient(rawTgt || "") : (rawTgt || ""))
    .replace(/\?/g, "")
    .trim();
  if (!clean || typeof studioGraphState === "undefined" || !studioGraphState.nodeById) return "";
  const direct = studioGraphState.nodeById.get(clean);
  if (direct) return direct.id;
  const lower = clean.toLowerCase();
  if (studioGraphState.nodes) {
    for (let i = 0; i < studioGraphState.nodes.length; i++) {
      const cand = studioGraphState.nodes[i];
      if (cand.id.toLowerCase() === lower || (cand.name || "").trim().toLowerCase() === lower) {
        return cand.id;
      }
    }
  }
  return clean;
}

let sceneIdRedirects = new Map();

function resolveCurrentClientSceneId(id) {
  let cur = (id || "").trim();
  let guard = 0;
  while (cur && sceneIdRedirects.has(cur) && guard < 16) {
    const next = sceneIdRedirects.get(cur);
    if (!next || next === cur) break;
    cur = next;
    guard++;
  }
  return cur;
}

function syncSceneIdentityAndMetaFromHeaders(oldId, newId, newExplicitId, newRef, newTitle, newChapter, startSceneId) {
  const curId = (newId || oldId || "").trim();
  if (!curId) return;
  const rawOldId = (oldId || curId).trim();
  let prevId = resolveCurrentClientSceneId(rawOldId);
  if (
    typeof studioGraphState !== "undefined" &&
    studioGraphState.nodeById &&
    !studioGraphState.nodeById.has(prevId)
  ) {
    const modalScInp = qs('#form-scene-text input[name="scene_id"]');
    const modalCurId = modalScInp ? (modalScInp.value || "").trim() : "";
    if (modalCurId && studioGraphState.nodeById.has(modalCurId)) {
      prevId = modalCurId;
    }
  }
  const idChanged = Boolean(prevId && curId && prevId !== curId);
  if (rawOldId && rawOldId !== curId) {
    sceneIdRedirects.set(rawOldId, curId);
  }
  if (prevId && prevId !== curId) {
    sceneIdRedirects.set(prevId, curId);
  }
  sceneIdRedirects.delete(curId);
  const displayTitle = (newTitle || newRef || curId).trim();
  const cleanChapter = (newChapter || "").trim();

  if (idChanged && typeof studioGraphState !== "undefined" && studioGraphState.nodeById) {
    const nd = studioGraphState.nodeById.get(prevId);
    if (nd) {
      studioGraphState.nodeById.delete(prevId);
      nd.id = curId;
      if (displayTitle) nd.name = displayTitle;
      if (cleanChapter) {
        nd.chapter = cleanChapter;
        nd.chapterLower = cleanChapter.toLowerCase();
      }
      studioGraphState.nodeById.set(curId, nd);
    }
    for (let i = 0; i < studioGraphState.nodes.length; i++) {
      const n = studioGraphState.nodes[i];
      if (n.ports) {
        for (let j = 0; j < n.ports.length; j++) {
          if (n.ports[j].targetId === prevId) {
            n.ports[j].targetId = curId;
            if (newRef) n.ports[j].target = newRef;
          }
        }
      }
      if (n.implicitPort && n.implicitPort.targetId === prevId) {
        n.implicitPort.targetId = curId;
        if (newRef) n.implicitPort.target = newRef;
      }
    }
    if (studioGraphState.chapters) {
      for (let i = 0; i < studioGraphState.chapters.length; i++) {
        if (studioGraphState.chapters[i].startSceneId === prevId) {
          studioGraphState.chapters[i].startSceneId = curId;
        }
      }
    }
    if (typeof selectedNodeIds !== "undefined" && selectedNodeIds.has(prevId)) {
      selectedNodeIds.delete(prevId);
      selectedNodeIds.add(curId);
    }
    if (typeof drawStudioWires === "function") drawStudioWires();
    studioGraphState.staticBatchDirty = true;
    studioGraphState.overlayBatchDirty = true;
    if (typeof renderStudioCanvas === "function") renderStudioCanvas();
  } else if (typeof studioGraphState !== "undefined" && studioGraphState.nodeById) {
    const nd = studioGraphState.nodeById.get(curId);
    if (nd) {
      let nodeChanged = false;
      if (displayTitle && nd.name !== displayTitle) {
        nd.name = displayTitle;
        nodeChanged = true;
      }
      if (cleanChapter && nd.chapter !== cleanChapter) {
        nd.chapter = cleanChapter;
        nd.chapterLower = cleanChapter.toLowerCase();
        nodeChanged = true;
      }
      if (nodeChanged) {
        studioGraphState.staticBatchDirty = true;
        if (typeof renderStudioCanvas === "function") renderStudioCanvas();
      }
    }
  }

  ["#form-scene-text", "#editor-context-form"].forEach((sel) => {
    const f = qs(sel);
    if (!f) return;
    const scInp = qs('input[name="scene_id"]', f);
    const fId = f.getAttribute("data-context-scene-id") || (scInp ? scInp.value : "") || "";
    if (fId === prevId || fId === curId) {
      if (f.hasAttribute("data-context-scene-id")) {
        f.setAttribute("data-context-scene-id", curId);
      }
      if (scInp) scInp.value = curId;
      const expInp = qs('input[name="explicit_id"]', f);
      if (expInp && document.activeElement !== expInp && newExplicitId !== undefined) {
        expInp.value = newExplicitId;
      }
      const chInp = qs('input[name="chapter"]', f);
      if (chInp && document.activeElement !== chInp && cleanChapter) {
        chInp.value = cleanChapter;
      }
      if (idChanged) {
        qsa('[data-scene="' + CSS.escape(prevId) + '"]', f).forEach((el) => {
          el.setAttribute("data-scene", curId);
        });
      }
    }
  });

  const expandBtn = qs("#btn-expand-scene-text");
  if (expandBtn && expandBtn.getAttribute("data-scene") === prevId) {
    expandBtn.setAttribute("data-scene", curId);
  }

  const bookForm = qs("#editor-book-form");
  if (bookForm) {
    qsa('[data-select-name="start_scene"] .custom-select-option', bookForm).forEach((opt) => {
      const v = opt.getAttribute("data-select-value") || "";
      if (v === prevId || v === curId) {
        opt.setAttribute("data-select-value", curId);
        if (displayTitle) {
          opt.setAttribute("data-select-label", displayTitle);
          opt.textContent = displayTitle;
        }
      }
    });
    const startInp = qs('input[name="start_scene"]', bookForm);
    if (startInp && (startInp.value === prevId || startInp.value === curId || (startSceneId && startSceneId === curId))) {
      startInp.value = curId;
      if (displayTitle) {
        const lbl = qs('[data-select-name="start_scene"] .custom-select-label', bookForm);
        if (lbl) lbl.textContent = displayTitle;
      }
    }
  }

  qsa('.syn-dropdown-item[data-syn-kind="target"]').forEach((item) => {
    const sId = item.getAttribute("data-syn-scene-id") || "";
    const vName = item.getAttribute("data-syn-var-name") || "";
    if (sId === prevId || sId === curId || vName === prevId) {
      item.setAttribute("data-syn-scene-id", curId);
      if (newRef) item.setAttribute("data-syn-var-name", newRef);
    }
  });
}

function syncCanvasNodeFromSceneTextModal(modalsRoot) {
  if (!modalsRoot || typeof studioGraphState === "undefined" || !studioGraphState.nodeById) return;
  const modalForm = qs("#form-scene-text", modalsRoot);
  if (!modalForm) return;
  const scInp = qs('input[name="scene_id"]', modalForm);
  const sceneId = scInp ? (scInp.value || "").trim() : "";
  if (!sceneId) return;
  const nd = studioGraphState.nodeById.get(sceneId);
  if (!nd) return;

  const nameInp = qs('input[name="name"]', modalForm);
  const expInp = qs('input[name="explicit_id"]', modalForm);
  const chInp = qs('input[name="chapter"]', modalForm);
  const displayTitle =
    (nameInp && (nameInp.value || "").trim()) ||
    (expInp && (expInp.value || "").trim()) ||
    sceneId;
  nd.name = displayTitle;
  if (chInp && (chInp.value || "").trim()) {
    nd.chapter = chInp.value.trim();
    nd.chapterLower = nd.chapter.toLowerCase();
  }
  syncSceneTitleAcrossForms(sceneId, nameInp ? nameInp.value : displayTitle, nameInp);

  const rows = qsa(".modal-choice-row", modalForm);
  if (rows.length >= 2) {
    nd.hasChoices = true;
    nd.implicitPort = null;
    nd.ports = rows.map((rowEl, idx) => {
      const lblEl = qs('[name="choice_label_' + idx + '"]', rowEl);
      const tgtEl = qs('[name="choice_target_' + idx + '"]', rowEl);
      const condEl = qs('[name="choice_cond_' + idx + '"]', rowEl);
      const pIcon = qs(".choice-port-icon", rowEl);
      const rawTgt = tgtEl ? (tgtEl.value || "").trim() : "";
      let resolvedId = "";
      let isCh = false;
      if (rawTgt && rawTgt.toUpperCase() !== "END") {
        if (rawTgt.toLowerCase().indexOf("chapter:") === 0) {
          isCh = true;
          resolvedId = rawTgt;
        } else {
          resolvedId = resolveCanvasSceneTargetId(rawTgt) || rawTgt;
        }
      }
      return {
        idx: idx,
        label: (lblEl && (lblEl.value || "").trim()) || tr("choice_prefix") + (idx + 1),
        target: rawTgt || "END",
        targetId: resolvedId,
        isChapterLink: isCh,
        isConditional: Boolean((condEl && (condEl.value || "").trim()) || (pIcon && pIcon.classList.contains("port-diamond"))),
        x: 0,
        y: 0
      };
    });
  } else if (rows.length === 1) {
    nd.hasChoices = false;
    nd.ports = [];
    const tgtEl = qs('[name="choice_target_0"]', rows[0]);
    const condEl = qs('[name="choice_cond_0"]', rows[0]);
    const pIcon = qs(".choice-port-icon", rows[0]);
    const rawTgt = tgtEl ? (tgtEl.value || "").trim() : "";
    let resolvedId = "";
    let isCh = false;
    if (rawTgt && rawTgt.toUpperCase() !== "END") {
      if (rawTgt.toLowerCase().indexOf("chapter:") === 0) {
        isCh = true;
        resolvedId = rawTgt;
      } else {
        resolvedId = resolveCanvasSceneTargetId(rawTgt) || rawTgt;
      }
    }
    nd.implicitPort = {
      idx: 0,
      label: "Dalej",
      target: rawTgt || "END",
      targetId: resolvedId,
      isChapterLink: isCh,
      isConditional: Boolean((condEl && (condEl.value || "").trim()) || (pIcon && pIcon.classList.contains("port-diamond"))),
      x: 0,
      y: 0
    };
  }
  if (typeof recomputeNodeMetrics === "function") recomputeNodeMetrics(nd);
  if (typeof updateChapterGroupFrames === "function") updateChapterGroupFrames();
  if (typeof drawStudioWires === "function") drawStudioWires();
  studioGraphState.staticBatchDirty = true;
  if (typeof renderStudioCanvas === "function") renderStudioCanvas();
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

  ["#editor-context-form", "#form-scene-text"].forEach((sel) => {
    const form = qs(sel);
    if (!form) return;
    const chInput = qs('input[name="chapter"]', form);
    if (chInput && chInput !== inputEl && (chInput.value || "").trim().toLowerCase() === prevLower) {
      chInput.value = nextGroup;
    }
    qsa(".node-chapter-select .custom-select-option", form).forEach((opt) => {
      const optVal = (opt.getAttribute("data-select-value") || "").trim();
      if (optVal.toLowerCase() === prevLower) {
        opt.setAttribute("data-select-value", nextGroup);
        opt.setAttribute("data-select-label", nextGroup);
        opt.textContent = nextGroup;
      }
    });
  });

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

function flushPendingDirtyFormsInternal(intoParamsObj) {
  if (isEditorHistoryPreview()) {
    cancelPendingInspectorTimers();
    return;
  }
  if (dirtyBookTitleInput) commitBookTitleInput(dirtyBookTitleInput);
  if (dirtyChapterInputs.size > 0) {
    Array.from(dirtyChapterInputs).forEach(commitChapterTitleInput);
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
    if (intoParamsObj && typeof intoParamsObj === "object") {
      new FormData(form).forEach((val, key) => {
        if (intoParamsObj[key] === undefined) intoParamsObj[key] = val;
      });
    } else {
      postServerActionSilent("AutoSaveScene", new FormData(form));
    }
  });
}

function consumeDirtyInspectorFormIntoParams(params) {
  flushPendingDirtyFormsInternal(params || {});
}

function flushInspectorFormIfDirty() {
  flushPendingDirtyFormsInternal(null);
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
  if (chapterForm) {
    chapterForm.classList.remove("hidden");
  }
  syncNodeSelectionDOM();
  renderStudioCanvas();
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
      badge.textContent = tr("selected_count_prefix") + selectedNodeIds.size;
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
    closeAuxiliaryStudioPanelsAndButtons();
    if (multiPanel) {
      multiPanel.classList.remove("hidden");
      const countLabel = qs("#multi-select-count-label", multiPanel);
      if (countLabel) {
        countLabel.textContent = tr("selected_scenes_prefix") + selectedItems.length + ")";
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
        delLabel.textContent = tr("delete_selected_scenes_prefix") + selectedItems.length + ")";
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
