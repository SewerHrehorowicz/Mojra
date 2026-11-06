"use strict";

function isEditorTouchClickSuppressed(e) {
  return (
    Date.now() < suppressNextTouchClickUntil &&
    e.target &&
    e.target.closest &&
    Boolean(e.target.closest("#timeline-scroll-viewport"))
  );
}

function clientToStudioWorld(viewport, clientX, clientY) {
  const rect = viewport.getBoundingClientRect();
  const z = editorCanvasState.zoom || 1.0;
  const vx = clientX - rect.left;
  const vy = clientY - rect.top;
  return {
    vx: vx,
    vy: vy,
    wx: (vx - editorCanvasState.panX) / z,
    wy: (vy - editorCanvasState.panY) / z
  };
}

function distToSegmentSq(px, py, x1, y1, x2, y2) {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const lenSq = dx * dx + dy * dy;
  if (lenSq < 0.001) {
    const ex = px - x1;
    const ey = py - y1;
    return ex * ex + ey * ey;
  }
  let t = ((px - x1) * dx + (py - y1) * dy) / lenSq;
  if (t < 0) t = 0;
  else if (t > 1) t = 1;
  const projX = x1 + t * dx;
  const projY = y1 + t * dy;
  const ox = px - projX;
  const oy = py - projY;
  return ox * ox + oy * oy;
}

function hitTestStudioCanvas(wx, wy) {
  const nodes = studioGraphState.nodes;
  const chapters = studioGraphState.chapters;
  const isPreview = typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview();
  const portHitR = 10;
  const portHitRSq = portHitR * portHitR;

  if (!isPreview) {
    // 1. Scene output ports
    for (let i = nodes.length - 1; i >= 0; i--) {
      const nd = nodes[i];
      if (nd.hasChoices && nd.ports.length > 0) {
        for (let j = 0; j < nd.ports.length; j++) {
          const pt = nd.ports[j];
          const px = nd.x + pt.ox;
          const py = nd.y + pt.oy;
          const dx = wx - px;
          const dy = wy - py;
          if (dx * dx + dy * dy <= portHitRSq) {
            return {
              type: "scene-out-port",
              node: nd,
              port: pt,
              portIdx: j,
              isBottom: false,
              x: px,
              y: py
            };
          }
        }
      } else if (nd.implicitPort) {
        const ip = nd.implicitPort;
        const px = nd.x + ip.ox;
        const py = nd.y + ip.oy;
        const dx = wx - px;
        const dy = wy - py;
        if (dx * dx + dy * dy <= portHitRSq) {
          return {
            type: "scene-out-port",
            node: nd,
            port: ip,
            portIdx: 0,
            isBottom: true,
            x: px,
            y: py
          };
        }
      }
    }

    // 2. Chapter output ports
    for (let i = chapters.length - 1; i >= 0; i--) {
      const ch = chapters[i];
      if (ch.hidden) continue;
      for (let j = 0; j < ch.outPorts.length; j++) {
        const op = ch.outPorts[j];
        const dx = wx - op.x;
        const dy = wy - op.y;
        if (dx * dx + dy * dy <= portHitRSq) {
          return {
            type: "chapter-out-port",
            chapter: ch,
            outPort: op,
            portIdx: j,
            x: op.x,
            y: op.y
          };
        }
      }
    }

    // 2b. Wire control pins (thickened line segment handles)
    const allWires = studioGraphState.wires || [];
    const pinHitRSq = 64; // 8px perpendicular radius along thickened bar
    for (let i = allWires.length - 1; i >= 0; i--) {
      const w = allWires[i];
      if (!w.pins || w.pins.length === 0) continue;
      for (let pIdx = w.pins.length - 1; pIdx >= 0; pIdx--) {
        const p = w.pins[pIdx];
        const bar = typeof computeWirePinSegmentBar === "function" ? computeWirePinSegmentBar(w, p) : null;
        if (bar) {
          if (distToSegmentSq(wx, wy, bar.x1, bar.y1, bar.x2, bar.y2) <= pinHitRSq) {
            return {
              type: "wire-pin",
              wire: w,
              pinIdx: pIdx,
              pin: p,
              segIdx: bar.segIdx,
              x: bar.cx,
              y: bar.cy
            };
          }
        } else {
          const dx = wx - p.x;
          const dy = wy - p.y;
          if (dx * dx + dy * dy <= 100) {
            return {
              type: "wire-pin",
              wire: w,
              pinIdx: pIdx,
              pin: p,
              x: p.x,
              y: p.y
            };
          }
        }
      }
    }
  }

  // 3. Scene node cards (footer icons, choice rows, header title, grip ⋮⋮, or card body)
  const footerBtnSize = 24;
  for (let i = nodes.length - 1; i >= 0; i--) {
    const nd = nodes[i];
    if (wx >= nd.x && wx <= nd.x + nd.w && wy >= nd.y && wy <= nd.y + nd.h) {
      const footerTop = nd.y + nd.h - NODE_FOOTER_HEIGHT;
      const editX = isPreview
        ? nd.x + nd.w - 8 - footerBtnSize
        : nd.x + nd.w - 8 - footerBtnSize * 2 - 6;
      if (wx >= editX && wx <= editX + footerBtnSize && wy >= footerTop + 8 && wy <= footerTop + 8 + footerBtnSize) {
        return { type: "edit-scene-btn", node: nd };
      }
      if (isPreview) {
        return { type: "node-card", node: nd };
      }
      const delX = nd.x + nd.w - 8 - footerBtnSize;
      if (wx >= delX && wx <= delX + footerBtnSize && wy >= footerTop + 8 && wy <= footerTop + 8 + footerBtnSize) {
        return { type: "delete-scene-btn", node: nd };
      }
      if (wx >= nd.x + 8 && wx <= nd.x + 8 + footerBtnSize && wy >= footerTop + 8 && wy <= footerTop + 8 + footerBtnSize) {
        return { type: "add-choice-btn", node: nd };
      }
      if (wx >= nd.x + 2 && wx <= nd.x + 26 && wy >= nd.y + 4 && wy <= nd.y + 30) {
        return { type: "node-grip", node: nd };
      }
      if (
        (nd.branchRole === "primary" || nd.branchRole === "secondary") &&
        wx >= nd.branchRadioX - 4 &&
        wx <= nd.branchRadioX + 12 &&
        wy >= nd.branchRadioY - 4 &&
        wy <= nd.branchRadioY + 12
      ) {
        return { type: "branch-radio", node: nd };
      }
      if (wx > nd.x + 26 && wx <= nd.x + nd.w - 8 && wy >= nd.y + 4 && wy < nd.y + NODE_HEADER_HEIGHT) {
        return { type: "node-title", node: nd };
      }
      if (nd.hasChoices && nd.ports && nd.ports.length > 0 && wy >= nd.y + NODE_HEADER_HEIGHT && wy < footerTop) {
        const choiceIdx = Math.floor((wy - (nd.y + NODE_HEADER_HEIGHT)) / NODE_CHOICE_ROW_HEIGHT);
        if (choiceIdx >= 0 && choiceIdx < nd.ports.length) {
          if (wx >= nd.x + 2 && wx <= nd.x + 20) {
            return { type: "choice-grip", node: nd, choiceIdx: choiceIdx };
          }
          if (wx >= nd.x + nd.w - 26 && wx <= nd.x + nd.w - 8) {
            return { type: "choice-trash-btn", node: nd, choiceIdx: choiceIdx };
          }
          if (wx > nd.x + 20 && wx < nd.x + nd.w - 26) {
            return { type: "choice-label", node: nd, choiceIdx: choiceIdx };
          }
        }
      }
      return { type: "node-card", node: nd };
    }
  }

  // 4. Chapter group headers (grip ⋮⋮ or title)
  for (let i = chapters.length - 1; i >= 0; i--) {
    const ch = chapters[i];
    if (ch.hidden) continue;
    if (wx >= ch.x && wx <= ch.x + ch.w && wy >= ch.y && wy <= ch.y + 28) {
      if (isPreview) {
        return { type: "chapter-title", chapter: ch };
      }
      if (wx <= ch.x + 26) {
        return { type: "chapter-grip", chapter: ch };
      }
      return { type: "chapter-title", chapter: ch };
    }
  }

  // 5. Static wires
  const wires = studioGraphState.wires;
  const wireTolSq = 49; // 7px world radius
  for (let i = wires.length - 1; i >= 0; i--) {
    const w = wires[i];
    if (
      w.maxX !== undefined &&
      (wx < w.minX - 8 || wx > w.maxX + 8 || wy < w.minY - 8 || wy > w.maxY + 8)
    ) {
      continue;
    }
    const pts = w.pts;
    for (let j = 0; j < pts.length - 1; j++) {
      if (distToSegmentSq(wx, wy, pts[j].x, pts[j].y, pts[j + 1].x, pts[j + 1].y) <= wireTolSq) {
        return { type: "wire", wire: w, segIdx: j, wx: wx, wy: wy };
      }
    }
  }

  return { type: "empty" };
}

function isSameHoveredCanvasItem(a, b) {
  if (a === b) return true;
  if (!a || !b) return false;
  if (a.type !== b.type) return false;
  return (
    a.node === b.node &&
    a.chapter === b.chapter &&
    a.portIdx === b.portIdx &&
    a.choiceIdx === b.choiceIdx &&
    a.pinIdx === b.pinIdx &&
    a.wire === b.wire
  );
}

function updateCanvasHoverCursor(viewport, hit) {
  if (!viewport) return;
  if (editorCanvasState.spacePressed) {
    viewport.style.cursor = "grab";
    return;
  }
  if (!hit || hit.type === "empty") {
    viewport.style.cursor = "default";
    return;
  }
  switch (hit.type) {
    case "scene-out-port":
    case "chapter-out-port":
      viewport.style.cursor = "crosshair";
      break;
    case "node-grip":
    case "chapter-grip":
    case "choice-grip":
    case "wire-pin":
    case "wire":
      viewport.style.cursor = "grab";
      break;
    case "node-title":
    case "choice-label":
      viewport.style.cursor = "text";
      break;
    case "chapter-title":
    case "add-choice-btn":
    case "edit-scene-btn":
    case "delete-scene-btn":
    case "choice-trash-btn":
    case "branch-radio":
    case "node-card":
      viewport.style.cursor = "pointer";
      break;
    default:
      viewport.style.cursor = "default";
      break;
  }
}

function triggerCanvasAddChoiceForNode(nd) {
  if (!nd || !nd.id || (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview())) return;
  const params = { scene: nd.id };
  const contextForm = qs("#editor-context-form");
  if (contextForm && contextForm.getAttribute("data-context-scene-id") === nd.id) {
    if (typeof dirtyInspectorForms !== "undefined") {
      dirtyInspectorForms.delete(contextForm);
    }
    if (typeof liveSaveTimers !== "undefined" && liveSaveTimers.has(contextForm)) {
      clearTimeout(liveSaveTimers.get(contextForm));
      liveSaveTimers.delete(contextForm);
    }
    const fd = new FormData(contextForm);
    fd.forEach((val, key) => {
      params[key] = val;
    });
  }
  sendServerAction("AddChoice", params);
}

function triggerCanvasReorderChoice(nd, fromIdx, toIdx, sourceForm) {
  if (!nd || !nd.id || fromIdx === toIdx || (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview())) {
    return;
  }
  const params = {
    scene: nd.id,
    from_idx: String(fromIdx),
    to_idx: String(toIdx)
  };
  const form =
    sourceForm ||
    (() => {
      const cf = qs("#editor-context-form");
      return cf && cf.getAttribute("data-context-scene-id") === nd.id ? cf : null;
    })();
  if (form) {
    if (typeof dirtyInspectorForms !== "undefined") {
      dirtyInspectorForms.delete(form);
    }
    if (typeof liveSaveTimers !== "undefined" && liveSaveTimers.has(form)) {
      clearTimeout(liveSaveTimers.get(form));
      liveSaveTimers.delete(form);
    }
    const fd = new FormData(form);
    fd.forEach((val, key) => {
      params[key] = val;
    });
  }
  params.scene = nd.id;
  params.from_idx = String(fromIdx);
  params.to_idx = String(toIdx);
  sendServerAction("ReorderChoice", params);
}

function moveNodePortInMemory(nd, fromIdx, toIdx) {
  if (!nd || !nd.ports || fromIdx === toIdx) return false;
  if (fromIdx < 0 || fromIdx >= nd.ports.length || toIdx < 0 || toIdx >= nd.ports.length) return false;
  const movedPort = nd.ports.splice(fromIdx, 1)[0];
  nd.ports.splice(toIdx, 0, movedPort);
  for (let k = 0; k < nd.ports.length; k++) {
    nd.ports[k].idx = k;
  }
  recomputeNodeMetrics(nd);
  const ch = nd.chapterObj || studioGraphState.chapterByKey.get(nd.chapterLower);
  if (ch) {
    recomputeChapterPortPositions(ch);
  }
  routeStudioWires();
  studioGraphState.staticBatchDirty = true;
  studioGraphState.overlayBatchDirty = true;
  renderStudioCanvas();
  return true;
}

function beginChoiceDragGesture(nd, choiceIdx, viewport, clientX, clientY, isTouch) {
  if (!nd || !nd.ports || nd.ports.length < 2) return false;
  if (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview()) return false;
  editorSelectionCleared = false;
  selectedNodeIds.clear();
  if (nd.id) selectedNodeIds.add(nd.id);
  syncNodeSelectionDOM();
  if (viewport) viewport.classList.add("is-panning");
  activeCanvasGesture = {
    type: "choice-drag",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    node: nd,
    sceneId: nd.id,
    startIdx: choiceIdx,
    currentIdx: choiceIdx,
    startClientX: clientX,
    startClientY: clientY,
    moved: false
  };
  studioGraphState.overlayBatchDirty = true;
  renderStudioCanvas();
  return true;
}

function handleEditorCanvasClick(e) {
  const multiDeselectBtn = e.target.closest("[data-multi-deselect-scene]");
  if (multiDeselectBtn) {
    e.preventDefault();
    e.stopPropagation();
    const id = multiDeselectBtn.getAttribute("data-multi-deselect-scene") || "";
    if (id && selectedNodeIds.has(id)) {
      selectedNodeIds.delete(id);
      editorSelectionCleared = selectedNodeIds.size === 0;
      syncNodeSelectionDOM();
      renderStudioCanvas();
      if (selectedNodeIds.size === 1) {
        selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
      }
    }
    return true;
  }

  const multiSelectBtn = e.target.closest("[data-multi-select-scene]");
  if (multiSelectBtn) {
    e.preventDefault();
    e.stopPropagation();
    const id = multiSelectBtn.getAttribute("data-multi-select-scene") || "";
    if (id) {
      editorSelectionCleared = false;
      selectedNodeIds.clear();
      selectedNodeIds.add(id);
      syncNodeSelectionDOM();
      renderStudioCanvas();
      selectNodeForContextPanel(id);
    }
    return true;
  }

  const selectChBtn = e.target.closest("[data-select-chapter]");
  if (selectChBtn) {
    e.preventDefault();
    e.stopPropagation();
    const chName = (selectChBtn.getAttribute("data-select-chapter") || "").trim().toLowerCase();
    if (!chName) return true;
    if (!e.shiftKey && !e.metaKey && !e.ctrlKey) {
      selectedNodeIds.clear();
    }
    for (let i = 0; i < studioGraphState.nodes.length; i++) {
      const nd = studioGraphState.nodes[i];
      if (nd.chapterLower === chName) {
        selectedNodeIds.add(nd.id);
      }
    }
    editorSelectionCleared = selectedNodeIds.size === 0;
    syncNodeSelectionDOM();
    renderStudioCanvas();
    if (selectedNodeIds.size === 1) {
      selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
    }
    return true;
  }

  const addNodeBtn = e.target.closest("[data-canvas-add-node]");
  if (addNodeBtn) {
    e.preventDefault();
    e.stopPropagation();
    const vp = qs("#timeline-scroll-viewport");
    if (!vp) return true;
    const rect = vp.getBoundingClientRect();
    const z = editorCanvasState.zoom || 1.0;
    const worldCenterX = (rect.width * 0.5 - editorCanvasState.panX) / z - 96;
    const worldCenterY = (rect.height * 0.35 - editorCanvasState.panY) / z;
    createSceneNodeAtWorldCoords(worldCenterX, worldCenterY);
    return true;
  }

  const ctxAddBtn = e.target.closest("#btn-context-add-node");
  if (ctxAddBtn) {
    e.preventDefault();
    e.stopPropagation();
    createSceneNodeAtWorldCoords(pendingContextMenuWorldPos.x, pendingContextMenuWorldPos.y);
    return true;
  }

  const ctxAddChBtn = e.target.closest("#btn-context-add-chapter");
  if (ctxAddChBtn) {
    e.preventDefault();
    e.stopPropagation();
    closeCanvasContextMenu();
    sendServerAction("CreateChapterGroup", {
      x: snapToEditorGrid(pendingContextMenuWorldPos.x),
      y: snapToEditorGrid(pendingContextMenuWorldPos.y)
    });
    return true;
  }

  const ctxResetBtn = e.target.closest("#btn-context-reset-view");
  if (ctxResetBtn) {
    e.preventDefault();
    e.stopPropagation();
    closeCanvasContextMenu();
    const vp = qs("#timeline-scroll-viewport");
    if (!vp) return true;
    fitCanvasToAllNodes(vp);
    applyEditorCanvasTransform();
    scheduleSaveEditorViewport();
    return true;
  }

  const zoomBtn = e.target.closest("[data-canvas-zoom]");
  if (zoomBtn) {
    e.preventDefault();
    e.stopPropagation();
    const mode = zoomBtn.getAttribute("data-canvas-zoom");
    const vp = qs("#timeline-scroll-viewport");
    if (!vp) return true;
    const rect = vp.getBoundingClientRect();
    const cx = rect.left + rect.width * 0.5;
    const cy = rect.top + rect.height * 0.5;
    if (mode === "in") {
      zoomEditorAtPoint((editorCanvasState.zoom || 1.0) * 1.2, cx, cy);
    } else if (mode === "out") {
      zoomEditorAtPoint((editorCanvasState.zoom || 1.0) / 1.2, cx, cy);
    } else if (mode === "fit") {
      fitCanvasToAllNodes(vp);
      applyEditorCanvasTransform();
      scheduleSaveEditorViewport();
    } else if (mode === "reset") {
      if (areAllNodesOutOfViewport(vp)) {
        fitCanvasToAllNodes(vp);
      } else {
        resetCanvasToFirstNode(vp);
      }
      applyEditorCanvasTransform();
      scheduleSaveEditorViewport();
    }
    return true;
  }

  const scriptInsertBtn = e.target.closest("[data-insert-script-block], [data-insert-script-char]");
  if (scriptInsertBtn) {
    const ta = qs("#modal-scene-textarea");
    if (ta) {
      e.preventDefault();
      e.stopPropagation();
      const kind = scriptInsertBtn.getAttribute("data-insert-script-block") || "";
      const charName = scriptInsertBtn.getAttribute("data-insert-script-char") || "";
      const std = scriptInsertBtn.getAttribute("data-script-standard") || "pl";
      const isPL = std === "pl";

      let blockText = "";
      let selectTarget = "";
      let needsDoubleNewline = true;

      if (charName) {
        blockText = charName + "\nKwestia dialogowa...";
        selectTarget = "Kwestia dialogowa...";
      } else if (kind === "dialogue") {
        blockText = "POSTAĆ\nKwestia dialogowa...";
        selectTarget = "POSTAĆ";
      } else if (kind === "parenthetical") {
        blockText = "(cicho)\n";
        selectTarget = "cicho";
        needsDoubleNewline = false;
      } else if (kind === "action") {
        blockText = "Opis akcji...";
        selectTarget = "Opis akcji...";
      } else if (kind === "dialogue_off") {
        blockText = (isPL ? "POSTAĆ (OFF)\n" : "POSTAĆ (O.S.)\n") + "Kwestia spoza kadru...";
        selectTarget = "POSTAĆ";
      } else if (kind === "dialogue_vo") {
        blockText = "POSTAĆ (V.O.)\nKwestia narratora...";
        selectTarget = "POSTAĆ";
      } else if (kind === "transition_cut") {
        blockText = isPL ? "> CIĘCIE DO:\n" : "> CUT TO:\n";
        selectTarget = "";
      } else if (kind === "transition_fade") {
        blockText = isPL ? "> ŚCIEMNIENIE.\n" : "> FADE OUT.\n";
        selectTarget = "";
      } else if (kind === "subslug") {
        blockText = isPL ? "WN. LOKACJA - CIĄG DALSZY\n" : "INT. LOCATION - CONTINUOUS\n";
        selectTarget = isPL ? "LOKACJA" : "LOCATION";
      }

      if (blockText) {
        const val = ta.value || "";
        const start = typeof ta.selectionStart === "number" ? ta.selectionStart : val.length;
        const end = typeof ta.selectionEnd === "number" ? ta.selectionEnd : val.length;
        const before = val.slice(0, start);
        const after = val.slice(end);

        let prefix = "";
        if (before.length > 0) {
          if (needsDoubleNewline) {
            if (before.endsWith("\n\n")) {
              prefix = "";
            } else if (before.endsWith("\n")) {
              prefix = "\n";
            } else {
              prefix = "\n\n";
            }
          } else if (!before.endsWith("\n")) {
            prefix = "\n";
          }
        }

        const inserted = prefix + blockText;
        ta.value = before + inserted + after;
        ta.focus();

        const insertStart = before.length + prefix.length;
        if (selectTarget) {
          const relIdx = blockText.indexOf(selectTarget);
          if (relIdx >= 0) {
            ta.setSelectionRange(insertStart + relIdx, insertStart + relIdx + selectTarget.length);
          } else {
            ta.setSelectionRange(insertStart + blockText.length, insertStart + blockText.length);
          }
        } else {
          ta.setSelectionRange(insertStart + blockText.length, insertStart + blockText.length);
        }

        if (typeof autoResizeSceneTextarea === "function") {
          autoResizeSceneTextarea(ta);
        } else if (typeof syncSceneTextareaHighlight === "function") {
          syncSceneTextareaHighlight(ta);
        }
        const modalForm = ta.closest("form#form-scene-text");
        if (modalForm && typeof scheduleNodeLiveSave === "function") {
          scheduleNodeLiveSave(modalForm);
        }
        return true;
      }
    }
  }

  const formToggleBtn = e.target.closest("[data-form-toggle]");
  if (formToggleBtn) {
    e.preventDefault();
    e.stopPropagation();
    if (formToggleBtn.disabled || formToggleBtn.classList.contains("is-disabled")) return true;
    const row = formToggleBtn.closest(".node-meta-row, .form-field") || formToggleBtn.parentElement;
    const fieldName = formToggleBtn.getAttribute("data-form-toggle") || "";
    const hiddenInput = row ? qs('input[type="hidden"][name="' + fieldName + '"]', row) : null;
    const curVal = hiddenInput ? hiddenInput.value === "true" : formToggleBtn.classList.contains("active");
    const nextVal = !curVal;
    if (hiddenInput) {
      hiddenInput.value = nextVal ? "true" : "false";
    }
    const activeWhen = formToggleBtn.getAttribute("data-active-when") || "true";
    const isVisualActive = activeWhen === "false" ? !nextVal : nextVal;
    formToggleBtn.classList.toggle("active", isVisualActive);
    const lblSpan = qs(".custom-select-label", formToggleBtn);
    if (lblSpan) {
      const onLbl = formToggleBtn.getAttribute("data-label-on") || "";
      const offLbl = formToggleBtn.getAttribute("data-label-off") || "";
      lblSpan.textContent = isVisualActive ? onLbl : offLbl;
    }
    const iconOn = qs(".toggle-icon-on", formToggleBtn);
    const iconOff = qs(".toggle-icon-off", formToggleBtn);
    if (iconOn) iconOn.classList.toggle("hidden", !isVisualActive);
    if (iconOff) iconOff.classList.toggle("hidden", isVisualActive);
    return true;
  }

  const radioOpt = e.target.closest("label.form-radio-option");
  if (radioOpt) {
    if (radioOpt.classList.contains("is-disabled")) {
      e.preventDefault();
      return true;
    }
    const radioInp = qs("input.form-radio-input", radioOpt);
    if (radioInp && !radioInp.disabled) {
      e.preventDefault();
      e.stopPropagation();
      const group = radioOpt.closest(".form-radio-group");
      if (group) {
        qsa("label.form-radio-option", group).forEach((opt) => {
          const inp = qs("input.form-radio-input", opt);
          const isCurrent = opt === radioOpt;
          opt.classList.toggle("active", isCurrent);
          if (inp) {
            inp.checked = isCurrent;
            if (isCurrent) {
              inp.setAttribute("checked", "checked");
            } else {
              inp.removeAttribute("checked");
            }
          }
        });
      }
      const bookForm = radioOpt.closest("form.editor-book-form");
      if (bookForm) {
        const params = {};
        const fd = new FormData(bookForm);
        fd.forEach((v, k) => {
          params[k] = v;
        });
        sendServerAction("SaveBookSettings", params);
      }
      return true;
    }
  }

  const selectToggle = e.target.closest("[data-select-toggle]");
  if (selectToggle) {
    e.preventDefault();
    e.stopPropagation();
    const selectEl = selectToggle.closest(".custom-select");
    if (!selectEl) return true;
    const menu = qs(".custom-select-menu", selectEl);
    const isOpen = selectEl.classList.contains("open");
    closeAllCustomSelects(selectEl);
    if (isOpen) {
      selectEl.classList.remove("open");
      if (menu) menu.classList.add("hidden");
    } else {
      selectEl.classList.add("open");
      if (menu) menu.classList.remove("hidden");
    }
    return true;
  }

  const selectOpt = e.target.closest(".custom-select-option");
  if (selectOpt) {
    e.preventDefault();
    e.stopPropagation();
    const selectEl = selectOpt.closest(".custom-select");
    if (!selectEl) return true;
    const val = selectOpt.getAttribute("data-select-value") || "";
    if (val === "__browse_disk__") {
      closeAllCustomSelects(null);
      const hiddenInput = qs(".custom-select-input", selectEl);
      const inputName = hiddenInput ? hiddenInput.getAttribute("name") || "" : "";
      const mediaKind = inputName === "explicit_image" ? "image" : "audio";
      const form = selectEl.closest("form.editor-context-form, form#form-scene-text");
      const params = { kind: mediaKind };
      if (form) {
        const fd = new FormData(form);
        fd.forEach((v, k) => {
          params[k] = v;
        });
        if (!params.scene && params.scene_id) {
          params.scene = params.scene_id;
        }
      }
      sendServerAction("BrowseAddBookMedia", params);
      return true;
    }
    const lbl = selectOpt.getAttribute("data-select-label") || val || "— brak —";
    const hiddenInput = qs(".custom-select-input", selectEl);
    const labelSpan = qs(".custom-select-label", selectEl);
    const triggerBtn = qs(".custom-select-trigger", selectEl);
    const portIcon = qs(".choice-port-icon", selectEl);
    if (hiddenInput) hiddenInput.value = val;
    if (labelSpan) labelSpan.textContent = lbl;
    if (triggerBtn && portIcon) {
      if (typeof setElementHint === "function") {
        setElementHint(triggerBtn, lbl);
      } else {
        triggerBtn.setAttribute("data-hint", lbl);
      }
      const isFilled = Boolean(val && val.toUpperCase() !== "END");
      portIcon.classList.toggle("port-filled", isFilled);
    }
    qsa(".custom-select-option", selectEl).forEach((btn) => {
      btn.classList.toggle("active", btn === selectOpt);
    });
    closeAllCustomSelects(null);

    if (selectEl.classList.contains("book-start-chapter-select")) {
      if (val) {
        sendServerAction("SetBookStartChapter", { chapter: val });
      }
      return true;
    }

    if (selectEl.classList.contains("book-start-scene-select")) {
      if (val) {
        sendServerAction("SetStartScene", { scene: val });
      }
      return true;
    }

    if (selectEl.classList.contains("chapter-start-scene-select")) {
      const chForm = selectEl.closest("form.editor-chapter-form");
      const chName = chForm ? chForm.getAttribute("data-context-chapter") || "" : "";
      if (chName && val) {
        sendServerAction("SetChapterStartScene", {
          chapter: chName,
          scene: val
        });
      }
      return true;
    }

    if (selectEl.classList.contains("node-chapter-select")) {
      const parentForm = selectEl.closest("form.editor-context-form");
      const nodeId = parentForm ? parentForm.getAttribute("data-context-scene-id") || "" : "";
      if (selectedNodeIds.size > 1 && selectedNodeIds.has(nodeId) && val) {
        sendServerAction("AssignChapterToScenes", {
          scene_ids: Array.from(selectedNodeIds).join(","),
          chapter: val
        });
        return true;
      }
    }

    if (selectEl.classList.contains("new-book-mode-select")) {
      const stdField = qs("#new-book-standard-field");
      const narrField = qs("#new-book-narrated-field");
      const imgField = qs("#new-book-imageless-field");
      const modalTitle = qs("#new-book-modal-title");
      const titleLabel = qs("#new-book-title-label");
      const titleInp = qs("#new-book-title");
      const submitBtn = qs("#new-book-submit-btn");
      const isScreenplay = val === "screenplay";
      if (stdField) stdField.classList.toggle("hidden", !isScreenplay);
      if (narrField) narrField.classList.toggle("hidden", isScreenplay);
      if (imgField) imgField.classList.toggle("hidden", isScreenplay);
      if (modalTitle) {
        modalTitle.textContent = isScreenplay ? "Utwórz Nowy Scenariusz" : "Utwórz Nową Książkę";
      }
      if (titleLabel) {
        titleLabel.textContent = "tytuł:";
      }
      if (titleInp) {
        titleInp.setAttribute("placeholder", isScreenplay ? "np. Ostatni Rejs" : "np. Tajemnica Starego Młyna");
      }
      if (submitBtn) {
        submitBtn.textContent = isScreenplay ? "Utwórz scenariusz" : "Utwórz książkę";
      }
      return true;
    }

    const bookForm = selectEl.closest("form.editor-book-form");
    if (bookForm) {
      const params = {};
      const fd = new FormData(bookForm);
      fd.forEach((v, k) => {
        params[k] = v;
      });
      sendServerAction("SaveBookSettings", params);
      return true;
    }

    if (selectEl.classList.contains("test-var-enum-select")) {
      const varName = selectEl.getAttribute("data-var-name") || "";
      if (varName) {
        const params = { var_name: varName, var_value: val };
        const modalForm = selectEl.closest("form#form-scene-text");
        if (modalForm) {
          const fd = new FormData(modalForm);
          fd.forEach((v, k) => {
            if (params[k] === undefined) params[k] = v;
          });
        }
        sendServerAction("UpdateSceneTextTestVar", params);
      }
      return true;
    }

    const modalForm = selectEl.closest("form#form-scene-text");
    if (modalForm) {
      const params = {};
      const fd = new FormData(modalForm);
      fd.forEach((v, k) => {
        params[k] = v;
      });
      sendServerAction("SaveSceneTextModal", params);
      return true;
    }

    const form = selectEl.closest("form.editor-context-form");
    if (form) {
      const params = {};
      const fd = new FormData(form);
      fd.forEach((v, k) => {
        params[k] = v;
      });
      sendServerAction("SaveScene", params);
    }
    return true;
  }

  if (!e.target.closest(".custom-select")) {
    closeAllCustomSelects(null);
  }

  return false;
}

function snapToWirePinGrid(val) {
  return Math.round(val / 6) * 6;
}

function computePointArcLengthAlongPolyline(pts, px, py) {
  if (!pts || pts.length < 2) return 0;
  let bestDistSq = Infinity;
  let bestArc = 0;
  let accum = 0;
  for (let i = 0; i < pts.length - 1; i++) {
    const ax = pts[i].x;
    const ay = pts[i].y;
    const bx = pts[i + 1].x;
    const by = pts[i + 1].y;
    const dx = bx - ax;
    const dy = by - ay;
    const lenSq = dx * dx + dy * dy;
    const segLen = Math.sqrt(lenSq);
    let t = 0;
    if (lenSq > 0.001) {
      t = Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / lenSq));
    }
    const projX = ax + t * dx;
    const projY = ay + t * dy;
    const dSq = (px - projX) * (px - projX) + (py - projY) * (py - projY);
    if (dSq < bestDistSq) {
      bestDistSq = dSq;
      bestArc = accum + t * segLen;
    }
    accum += segLen;
  }
  return bestArc;
}

function findInsertIndexForWirePin(wire, wx, wy) {
  const pins = wire.pins || [];
  if (pins.length === 0) return 0;
  const pts = wire.pts || wire.basePts || [];
  const newArc = computePointArcLengthAlongPolyline(pts, wx, wy);
  for (let i = 0; i < pins.length; i++) {
    const pArc = computePointArcLengthAlongPolyline(pts, pins[i].x, pins[i].y);
    if (newArc <= pArc) {
      return i;
    }
  }
  return pins.length;
}

function rerouteAfterWirePinMutation(wire) {
  if (!wire) return;
  if (wire.wireKey) {
    if (wire.pins && wire.pins.length > 0) {
      studioGraphState.wirePins.set(
        wire.wireKey,
        wire.pins.map((p) => ({ x: p.x, y: p.y, axis: p.axis, segIdx: p.segIdx }))
      );
    } else {
      studioGraphState.wirePins.delete(wire.wireKey);
    }
  }
  routeSingleWireBasePts(wire, studioGraphState.nodes, studioGraphState.chapters);
  applyParallelOffsetsAndBundles(studioGraphState.wires, null);
  studioGraphState.staticBatchDirty = true;
  studioGraphState.overlayBatchDirty = true;
  renderStudioCanvas();
}

function removeWirePinAt(wire, pinIdx) {
  if (!wire || !wire.pins || pinIdx < 0 || pinIdx >= wire.pins.length) return false;
  wire.pins.splice(pinIdx, 1);
  rerouteAfterWirePinMutation(wire);
  if (wire.wireKey) {
    saveWirePinsOnServer(wire.wireKey, wire.pins);
  }
  return true;
}

function beginWirePinDragGesture(hit, viewport, clientX, clientY, isTouch) {
  if (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview()) {
    return false;
  }
  const wire = hit.wire;
  if (!wire) return false;
  if (!wire.pins) wire.pins = [];
  const pts = wire.pts && wire.pts.length >= 2 ? wire.pts : wire.basePts || [];
  let segIdx = typeof hit.segIdx === "number" ? hit.segIdx : 0;
  if (hit.type === "wire-pin" && hit.pin && typeof hit.pin.segIdx === "number") {
    segIdx = hit.pin.segIdx;
  }
  if (segIdx < 0 || segIdx >= pts.length - 1) {
    segIdx = Math.max(0, pts.length - 2);
  }
  const p0 = pts[segIdx] || { x: wire.x1, y: wire.y1 };
  const p1 = pts[segIdx + 1] || { x: wire.x2, y: wire.y2 };
  const isHoriz = Math.abs(p0.y - p1.y) <= Math.abs(p0.x - p1.x);
  const axis = isHoriz ? "H" : "V";
  const segMidX = Math.round((p0.x + p1.x) * 0.5);
  const segMidY = Math.round((p0.y + p1.y) * 0.5);

  let existingIdx = -1;
  if (hit.type === "wire-pin" && typeof hit.pinIdx === "number") {
    existingIdx = hit.pinIdx;
  } else {
    for (let i = 0; i < wire.pins.length; i++) {
      if (wire.pins[i] && wire.pins[i].segIdx === segIdx) {
        existingIdx = i;
        break;
      }
    }
  }

  const isNewPin = existingIdx < 0;
  const ptWorld = clientToStudioWorld(viewport, clientX, clientY);
  const startWx = isNewPin ? ptWorld.wx : wire.pins[existingIdx].x;
  const startWy = isNewPin ? ptWorld.wy : wire.pins[existingIdx].y;
  const pinIdx = isNewPin ? findInsertIndexForWirePin(wire, startWx, startWy) : existingIdx;

  activeCanvasGesture = {
    type: "wire-pin-drag",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    wire: wire,
    pinIdx: pinIdx,
    segIdx: segIdx,
    axis: axis,
    fixedX: segMidX,
    fixedY: segMidY,
    isNewPin: isNewPin,
    inserted: !isNewPin,
    startClientX: clientX,
    startClientY: clientY,
    startWx: startWx,
    startWy: startWy,
    moved: false
  };
  studioGraphState.overlayBatchDirty = true;
  renderStudioCanvas();
  return true;
}

function beginWireConnectGesture(hit, viewport, isTouch) {
  const nd = hit.node;
  const pt = hit.port;
  const hasExistingTarget = Boolean((pt.targetId && pt.targetId !== "END") || pt.targetChapter);
  activeCanvasGesture = {
    type: "wire-connect",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    fromNode: nd,
    sourcePort: pt,
    fromSceneId: nd.id,
    fromChapter: nd.chapter,
    choiceIdx: String(hit.portIdx),
    isBottomPort: Boolean(hit.isBottom),
    hasExistingTarget: hasExistingTarget,
    startX: hit.x,
    startY: hit.y,
    curX: hit.x,
    curY: hit.y,
    hoveredTargetId: "",
    hoveredSceneNode: null,
    hoveredChapterIn: null,
    hoveredChapterOutPort: null
  };
  updateTempWirePreview();
  return true;
}

function beginChapterWireConnectGesture(hit, viewport, isTouch) {
  const ch = hit.chapter;
  const op = hit.outPort;
  const opSide = op.side || "bottom";
  activeCanvasGesture = {
    type: "chapter-wire-connect",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    fromChapterObj: ch,
    activeChapterOutPort: op,
    outPortSide: opSide,
    isBottomPort: opSide === "bottom",
    fromChapter: ch.ref || ch.name,
    oldTargetChapter: op.target || "",
    startX: op.x,
    startY: op.y,
    curX: op.x,
    curY: op.y,
    hoveredTargetChapter: "",
    hoveredChapterIn: null
  };
  updateTempWirePreview();
  return true;
}

function beginChapterDragGesture(ch, viewport, clientX, clientY, isTouch, clickedTitleArea) {
  if (!ch) return false;
  if (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview()) {
    selectChapterForContextPanel(ch.name);
    return true;
  }
  const chNodes =
    ch.nodes && ch.nodes.length > 0
      ? ch.nodes
      : studioGraphState.nodes.filter(
          (nd) => nd.chapterLower === ch.nameLower || nd.chapterLower === ch.refLower
        );
  if (chNodes.length === 0) return false;
  const items = chNodes.map((nd) => ({
    node: nd,
    sceneId: nd.id,
    startX: nd.x,
    startY: nd.y
  }));
  activeCanvasGesture = {
    type: "chapter-drag",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    chapter: ch,
    chapterName: ch.name,
    clickedTitleArea: Boolean(clickedTitleArea),
    dragConfirmed: !clickedTitleArea,
    items: items,
    startClientX: clientX,
    startClientY: clientY,
    lastValidDx: 0,
    lastValidDy: 0,
    moved: false
  };
  renderStudioCanvas();
  return true;
}

function beginNodeDragGesture(nd, viewport, clientX, clientY, isTouch, isMultiModifier) {
  const sceneId = nd.id;
  if (isMultiModifier) {
    if (selectedNodeIds.has(sceneId)) {
      selectedNodeIds.delete(sceneId);
      editorSelectionCleared = selectedNodeIds.size === 0;
      syncNodeSelectionDOM();
      renderStudioCanvas();
      if (selectedNodeIds.size === 1) {
        selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
      }
      return true;
    }
    if (sceneId) selectedNodeIds.add(sceneId);
    editorSelectionCleared = false;
  } else if (!selectedNodeIds.has(sceneId)) {
    editorSelectionCleared = false;
    selectedNodeIds.clear();
    if (sceneId) selectedNodeIds.add(sceneId);
  }
  syncNodeSelectionDOM();

  if (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview()) {
    renderStudioCanvas();
    if (selectedNodeIds.size === 1) {
      selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
    }
    return true;
  }

  const dragNodes = studioGraphState.nodes.filter((n) => selectedNodeIds.has(n.id));
  if (dragNodes.length === 0) {
    dragNodes.push(nd);
  }

  const items = dragNodes.map((n) => ({
    node: n,
    sceneId: n.id,
    startX: n.x,
    startY: n.y
  }));

  activeCanvasGesture = {
    type: "node-drag",
    isTouch: Boolean(isTouch),
    viewport: viewport,
    node: nd,
    sceneId: sceneId,
    isMultiModifier: Boolean(isMultiModifier),
    items: items,
    startClientX: clientX,
    startClientY: clientY,
    lastValidDx: 0,
    lastValidDy: 0,
    moved: false,
    hoveredChapterDrop: ""
  };
  renderStudioCanvas();
  return true;
}

function bindEditorCanvasEvents() {
  document.addEventListener("keydown", (e) => {
    if (e.code === "Space" && !e.target.closest("input, textarea, select")) {
      editorCanvasState.spacePressed = true;
      const vp = qs("#timeline-scroll-viewport");
      if (vp) vp.classList.add("is-panning");
    }
    if (e.key === "Escape") {
      if (e.target && e.target.id === "canvas-node-title-input") {
        e.preventDefault();
        hideFloatingNodeTitleInput(false);
        return;
      }
      if (e.target && e.target.id === "canvas-choice-input") {
        e.preventDefault();
        hideFloatingChoiceInput(false);
        return;
      }
      if (
        e.target &&
        e.target.classList &&
        (e.target.classList.contains("chapter-group-title-input") ||
          e.target.id === "book-settings-title-input")
      ) {
        e.preventDefault();
        if (e.target.id === "canvas-chapter-input") {
          hideFloatingChapterInput(false);
        } else {
          e.target.blur();
        }
        return;
      }
      if (!e.target.closest("input, textarea") && !qs(".modal-backdrop")) {
        editorSelectionCleared = true;
        if (selectedNodeIds.size > 0) {
          selectedNodeIds.clear();
        }
        syncNodeSelectionDOM();
        hideEditorContextPanel();
        renderStudioCanvas();
      }
    }
    if (e.key === "Enter" && e.target && e.target.classList) {
      if (e.target.id === "canvas-node-title-input") {
        e.preventDefault();
        hideFloatingNodeTitleInput(true);
        return;
      }
      if (e.target.id === "canvas-choice-input") {
        e.preventDefault();
        hideFloatingChoiceInput(true);
        return;
      }
      if (e.target.classList.contains("chapter-group-title-input")) {
        e.preventDefault();
        hideFloatingChapterInput(true);
        return;
      }
      if (e.target.id === "book-settings-title-input") {
        e.preventDefault();
        commitBookTitleInput(e.target);
        e.target.blur();
        return;
      }
      if (e.target.classList.contains("breakdown-name-input")) {
        e.preventDefault();
        commitBreakdownRenameInput(e.target);
        e.target.blur();
        return;
      }
    }
    if (
      (e.ctrlKey || e.metaKey) &&
      !e.altKey &&
      !e.target.closest("input, textarea, select, [contenteditable]") &&
      qs("#timeline-scroll-viewport") &&
      !(typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview())
    ) {
      const keyLower = (e.key || "").toLowerCase();
      if (keyLower === "z" && !e.shiftKey) {
        e.preventDefault();
        sendServerAction("UndoEditor", {});
        return;
      }
      if ((keyLower === "z" && e.shiftKey) || (keyLower === "y" && !e.shiftKey)) {
        e.preventDefault();
        sendServerAction("RedoEditor", {});
        return;
      }
    }
    if (
      (e.key === "f" || e.key === "F") &&
      !e.ctrlKey &&
      !e.metaKey &&
      !e.altKey &&
      !e.target.closest("input, textarea, select") &&
      !qs(".modal-backdrop") &&
      qs("#timeline-scroll-viewport")
    ) {
      e.preventDefault();
      const vp = qs("#timeline-scroll-viewport");
      fitCanvasToAllNodes(vp);
      applyEditorCanvasTransform();
      scheduleSaveEditorViewport();
      return;
    }
    if (
      (e.key === "Delete" || e.key === "Backspace") &&
      !e.target.closest("input, textarea, select") &&
      !qs(".modal-backdrop") &&
      qs("#timeline-scroll-viewport") &&
      !(typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview())
    ) {
      if (selectedNodeIds.size === 1) {
        e.preventDefault();
        sendServerAction("PromptDeleteScene", {
          scene: Array.from(selectedNodeIds)[0]
        });
      } else if (selectedNodeIds.size > 1) {
        e.preventDefault();
        sendServerAction("PromptDeleteScenes", {
          scenes: Array.from(selectedNodeIds).join(",")
        });
      }
    }
  });

  document.addEventListener("keyup", (e) => {
    if (e.code === "Space") {
      editorCanvasState.spacePressed = false;
      if (!activeCanvasGesture) {
        const vp = qs("#timeline-scroll-viewport");
        if (vp) vp.classList.remove("is-panning");
      }
    }
  });

  function startTouchPinchPanGesture(viewport, t0, t1) {
    const midX = (t0.clientX + t1.clientX) * 0.5;
    const midY = (t0.clientY + t1.clientY) * 0.5;
    const dist = Math.max(8, Math.hypot(t1.clientX - t0.clientX, t1.clientY - t0.clientY));
    const rect = viewport.getBoundingClientRect();
    const startZoom = editorCanvasState.zoom || 1.0;
    const startPanX = editorCanvasState.panX || 0;
    const startPanY = editorCanvasState.panY || 0;
    viewport.classList.add("is-panning");
    activeCanvasGesture = {
      type: "touch-pinch-pan",
      isTouch: true,
      viewport: viewport,
      startMidX: midX,
      startMidY: midY,
      startDist: dist,
      startZoom: startZoom,
      startPanX: startPanX,
      startPanY: startPanY,
      worldX: (midX - rect.left - startPanX) / startZoom,
      worldY: (midY - rect.top - startPanY) / startZoom,
      moved: true
    };
  }

  function applyIncrementalNodeMove(items, snappedDx, snappedDy) {
    const dirtyNodeIds = new Set();
    const affectedChaptersSet = new Set();
    const dirtyAabbs = [];
    let hasInterChapterPortMove = false;

    for (let i = 0; i < items.length; i++) {
      const nd = items[i].node;
      dirtyNodeIds.add(nd.id);
      dirtyAabbs.push({
        minX: nd.left - 24,
        minY: nd.top - 24,
        maxX: nd.right + 24,
        maxY: nd.bottom + 24
      });
      const ch = nd.chapterObj || studioGraphState.chapterByKey.get(nd.chapterLower);
      if (ch) {
        affectedChaptersSet.add(ch);
      }
      if (!hasInterChapterPortMove) {
        if (nd.implicitPort && nd.implicitPort.targetChapter) {
          hasInterChapterPortMove = true;
        } else if (nd.ports) {
          for (let k = 0; k < nd.ports.length; k++) {
            if (nd.ports[k].targetChapter) {
              hasInterChapterPortMove = true;
              break;
            }
          }
        }
      }
      nd.x = snapToEditorGrid(items[i].startX + snappedDx);
      nd.y = snapToEditorGrid(items[i].startY + snappedDy);
      recomputeNodeMetrics(nd);
      if (typeof updateNodeInSpatialGrids === "function") {
        updateNodeInSpatialGrids(nd);
      }
      dirtyAabbs.push({
        minX: nd.left - 24,
        minY: nd.top - 24,
        maxX: nd.right + 24,
        maxY: nd.bottom + 24
      });
    }

    const changedChapterFramesSet = updateChapterGroupFrames(
      affectedChaptersSet,
      hasInterChapterPortMove
    );

    drawStudioWires(dirtyNodeIds, dirtyAabbs, changedChapterFramesSet, affectedChaptersSet);
  }

  function updateCanvasDragMove(clientX, clientY) {
    if (!activeCanvasGesture) return;

    if (activeCanvasGesture.type === "pan") {
      editorCanvasState.panX = activeCanvasGesture.startPanX + (clientX - activeCanvasGesture.startClientX);
      editorCanvasState.panY = activeCanvasGesture.startPanY + (clientY - activeCanvasGesture.startClientY);
      applyEditorCanvasTransform();
      return;
    }

    if (activeCanvasGesture.type === "touch-pan") {
      const dx = clientX - activeCanvasGesture.startClientX;
      const dy = clientY - activeCanvasGesture.startClientY;
      const threshold = activeCanvasGesture.startedInteractive ? 7 : 3;
      if (!activeCanvasGesture.moved && Math.hypot(dx, dy) > threshold) {
        activeCanvasGesture.moved = true;
        if (activeCanvasGesture.viewport) {
          activeCanvasGesture.viewport.classList.add("is-panning");
        }
      }
      if (activeCanvasGesture.moved) {
        editorCanvasState.panX = activeCanvasGesture.startPanX + dx;
        editorCanvasState.panY = activeCanvasGesture.startPanY + dy;
        applyEditorCanvasTransform();
      }
      return;
    }

    if (activeCanvasGesture.type === "right-pan") {
      const dx = clientX - activeCanvasGesture.startClientX;
      const dy = clientY - activeCanvasGesture.startClientY;
      if (!activeCanvasGesture.moved && Math.hypot(dx, dy) > 4) {
        activeCanvasGesture.moved = true;
        editorCanvasState.suppressNextContextMenu = true;
        const vp = qs("#timeline-scroll-viewport");
        if (vp) vp.classList.add("is-panning");
      }
      if (activeCanvasGesture.moved) {
        editorCanvasState.panX = activeCanvasGesture.startPanX + dx;
        editorCanvasState.panY = activeCanvasGesture.startPanY + dy;
        applyEditorCanvasTransform();
      }
      return;
    }

    if (activeCanvasGesture.type === "choice-drag") {
      const nd = activeCanvasGesture.node;
      if (!nd || !nd.ports || nd.ports.length < 2) return;
      const ptWorld = clientToStudioWorld(activeCanvasGesture.viewport, clientX, clientY);
      const choicesTop = nd.y + NODE_HEADER_HEIGHT;
      let targetIdx = Math.floor((ptWorld.wy - choicesTop) / NODE_CHOICE_ROW_HEIGHT);
      if (targetIdx < 0) targetIdx = 0;
      if (targetIdx >= nd.ports.length) targetIdx = nd.ports.length - 1;
      if (targetIdx !== activeCanvasGesture.currentIdx) {
        if (moveNodePortInMemory(nd, activeCanvasGesture.currentIdx, targetIdx)) {
          activeCanvasGesture.currentIdx = targetIdx;
          activeCanvasGesture.moved = true;
        }
      }
      return;
    }

    if (activeCanvasGesture.type === "dom-choice-drag") {
      const list = activeCanvasGesture.list;
      const row = activeCanvasGesture.row;
      if (!list || !row) return;
      const children = Array.from(list.children);
      let targetIdx = activeCanvasGesture.currentIdx;
      for (let i = 0; i < children.length; i++) {
        const child = children[i];
        if (child === row) continue;
        const rect = child.getBoundingClientRect();
        const midY = rect.top + rect.height * 0.5;
        if (i < activeCanvasGesture.currentIdx && clientY < midY) {
          targetIdx = i;
          break;
        }
        if (i > activeCanvasGesture.currentIdx && clientY > midY) {
          targetIdx = i;
        }
      }
      if (targetIdx !== activeCanvasGesture.currentIdx) {
        const refNode =
          targetIdx < activeCanvasGesture.currentIdx
            ? children[targetIdx]
            : children[targetIdx].nextSibling;
        list.insertBefore(row, refNode);
        if (activeCanvasGesture.node) {
          moveNodePortInMemory(activeCanvasGesture.node, activeCanvasGesture.currentIdx, targetIdx);
        }
        activeCanvasGesture.currentIdx = targetIdx;
        activeCanvasGesture.moved = true;
      }
      return;
    }

    if (activeCanvasGesture.type === "wire-pin-drag") {
      const rawDist = Math.hypot(
        clientX - activeCanvasGesture.startClientX,
        clientY - activeCanvasGesture.startClientY
      );
      if (!activeCanvasGesture.moved && rawDist < 3) {
        return;
      }
      const wire = activeCanvasGesture.wire;
      if (!wire) return;
      const ptWorld = clientToStudioWorld(activeCanvasGesture.viewport, clientX, clientY);
      const isHoriz = activeCanvasGesture.axis === "H";
      const snappedX = isHoriz ? activeCanvasGesture.fixedX : snapToWirePinGrid(ptWorld.wx);
      const snappedY = isHoriz ? snapToWirePinGrid(ptWorld.wy) : activeCanvasGesture.fixedY;
      if (!activeCanvasGesture.inserted) {
        wire.pins.splice(activeCanvasGesture.pinIdx, 0, {
          x: snappedX,
          y: snappedY,
          axis: activeCanvasGesture.axis,
          segIdx: activeCanvasGesture.segIdx
        });
        activeCanvasGesture.inserted = true;
      } else if (wire.pins[activeCanvasGesture.pinIdx]) {
        const curPin = wire.pins[activeCanvasGesture.pinIdx];
        if (
          curPin.x === snappedX &&
          curPin.y === snappedY &&
          activeCanvasGesture.moved
        ) {
          return;
        }
        curPin.x = snappedX;
        curPin.y = snappedY;
        curPin.axis = activeCanvasGesture.axis;
      }
      activeCanvasGesture.moved = true;
      rerouteAfterWirePinMutation(wire);
      return;
    }

    if (activeCanvasGesture.type === "node-drag") {
      const z = editorCanvasState.zoom || 1.0;
      const dx = (clientX - activeCanvasGesture.startClientX) / z;
      const dy = (clientY - activeCanvasGesture.startClientY) / z;
      const rawDx = snapToEditorGrid(dx);
      const rawDy = snapToEditorGrid(dy);
      if (rawDx === activeCanvasGesture.lastSnappedDx && rawDy === activeCanvasGesture.lastSnappedDy) {
        return;
      }
      activeCanvasGesture.lastSnappedDx = rawDx;
      activeCanvasGesture.lastSnappedDy = rawDy;
      const valid = resolveValidNodeDragDelta(
        activeCanvasGesture.items || [],
        rawDx,
        rawDy,
        activeCanvasGesture.lastValidDx || 0,
        activeCanvasGesture.lastValidDy || 0
      );
      if (
        valid.dx === activeCanvasGesture.lastValidDx &&
        valid.dy === activeCanvasGesture.lastValidDy &&
        activeCanvasGesture.moved
      ) {
        return;
      }
      activeCanvasGesture.lastValidDx = valid.dx;
      activeCanvasGesture.lastValidDy = valid.dy;
      if (Math.abs(valid.dx) >= EDITOR_GRID_SIZE || Math.abs(valid.dy) >= EDITOR_GRID_SIZE) {
        activeCanvasGesture.moved = true;
      }
      applyIncrementalNodeMove(activeCanvasGesture.items || [], valid.dx, valid.dy);
      return;
    }

    if (activeCanvasGesture.type === "chapter-drag") {
      const rawDist = Math.hypot(
        clientX - activeCanvasGesture.startClientX,
        clientY - activeCanvasGesture.startClientY
      );
      if (!activeCanvasGesture.dragConfirmed) {
        if (rawDist < 5) {
          return;
        }
        activeCanvasGesture.dragConfirmed = true;
        hideFloatingChapterInput(false);
      }
      const z = editorCanvasState.zoom || 1.0;
      const dx = (clientX - activeCanvasGesture.startClientX) / z;
      const dy = (clientY - activeCanvasGesture.startClientY) / z;
      const rawDx = snapToEditorGrid(dx);
      const rawDy = snapToEditorGrid(dy);
      if (rawDx === activeCanvasGesture.lastSnappedDx && rawDy === activeCanvasGesture.lastSnappedDy) {
        return;
      }
      activeCanvasGesture.lastSnappedDx = rawDx;
      activeCanvasGesture.lastSnappedDy = rawDy;
      const valid = resolveValidChapterDragDelta(
        activeCanvasGesture.chapter,
        activeCanvasGesture.items || [],
        rawDx,
        rawDy,
        activeCanvasGesture.lastValidDx || 0,
        activeCanvasGesture.lastValidDy || 0
      );
      if (
        valid.dx === activeCanvasGesture.lastValidDx &&
        valid.dy === activeCanvasGesture.lastValidDy &&
        activeCanvasGesture.moved
      ) {
        return;
      }
      activeCanvasGesture.lastValidDx = valid.dx;
      activeCanvasGesture.lastValidDy = valid.dy;
      if (Math.abs(valid.dx) >= EDITOR_GRID_SIZE || Math.abs(valid.dy) >= EDITOR_GRID_SIZE) {
        activeCanvasGesture.moved = true;
      }
      applyIncrementalNodeMove(activeCanvasGesture.items || [], valid.dx, valid.dy);
      return;
    }

    if (activeCanvasGesture.type === "marquee-select") {
      const dx = clientX - activeCanvasGesture.startClientX;
      const dy = clientY - activeCanvasGesture.startClientY;
      if (!activeCanvasGesture.moved && Math.hypot(dx, dy) > 3) {
        activeCanvasGesture.moved = true;
      }
      if (!activeCanvasGesture.moved) return;

      const vpRect = activeCanvasGesture.viewport.getBoundingClientRect();
      const x1 = Math.min(activeCanvasGesture.startClientX, clientX) - vpRect.left;
      const y1 = Math.min(activeCanvasGesture.startClientY, clientY) - vpRect.top;
      const w = Math.abs(clientX - activeCanvasGesture.startClientX);
      const h = Math.abs(clientY - activeCanvasGesture.startClientY);
      activeCanvasGesture.marqueeRect = { x: x1, y: y1, w: w, h: h };

      const z = editorCanvasState.zoom || 1.0;
      const px = editorCanvasState.panX || 0;
      const py = editorCanvasState.panY || 0;
      const worldLeft = (x1 - px) / z;
      const worldRight = (x1 + w - px) / z;
      const worldTop = (y1 - py) / z;
      const worldBottom = (y1 + h - py) / z;

      selectedNodeIds.clear();
      activeCanvasGesture.baseSelection.forEach((id) => selectedNodeIds.add(id));

      for (let i = 0; i < studioGraphState.nodes.length; i++) {
        const nd = studioGraphState.nodes[i];
        if (nd.right >= worldLeft && nd.left <= worldRight && nd.bottom >= worldTop && nd.top <= worldBottom) {
          selectedNodeIds.add(nd.id);
        }
      }
      syncNodeSelectionDOM();
      renderStudioCanvas();
      return;
    }

    if (activeCanvasGesture.type === "wire-connect") {
      const ptWorld = clientToStudioWorld(activeCanvasGesture.viewport, clientX, clientY);
      let cx = ptWorld.wx;
      let cy = ptWorld.wy;
      let hoveredId = "";
      let midOutX;
      let midOutY;
      activeCanvasGesture.hoveredSceneNode = null;
      activeCanvasGesture.hoveredChapterIn = null;
      activeCanvasGesture.hoveredChapterOutPort = null;

      const fromChLower = (activeCanvasGesture.fromChapter || "").trim().toLowerCase();
      const chapters = studioGraphState.chapters;
      const nodes = studioGraphState.nodes;
      const fromFrame = studioGraphState.chapterByKey.get(fromChLower);

      const activateChapter = (tf) => {
        if (!tf || tf.hidden || tf.isRoot || !tf.ref || tf.nameLower === fromChLower || tf.refLower === fromChLower) {
          return false;
        }
        hoveredId = "chapter:" + tf.ref;
        cx = tf.inX;
        cy = tf.inY;
        activeCanvasGesture.hoveredChapterIn = tf;
        if (fromFrame) {
          const wantLower = tf.refLower;
          const matchedOut = fromFrame.outPorts.find(
            (op) => op.targetLower === wantLower || op.target === ""
          );
          if (matchedOut) {
            midOutX = matchedOut.x;
            midOutY = matchedOut.y;
            activeCanvasGesture.hoveredChapterOutPort = matchedOut;
          } else if (fromFrame.outPorts.length > 0) {
            const lastOp = fromFrame.outPorts[fromFrame.outPorts.length - 1];
            midOutX = lastOp.x + 24;
            midOutY = lastOp.y;
          } else {
            midOutX = fromFrame.x + fromFrame.w * 0.5;
            midOutY = fromFrame.y + fromFrame.h;
          }
        }
        return true;
      };

      let matched = false;
      if (fromFrame) {
        for (let j = 0; j < fromFrame.outPorts.length; j++) {
          const op = fromFrame.outPorts[j];
          if (Math.hypot(cx - op.x, cy - op.y) <= 24) {
            hoveredId = "chapter:" + (op.target || "__next__");
            cx = op.x;
            cy = op.y;
            activeCanvasGesture.hoveredChapterOutPort = op;
            matched = true;
            break;
          }
        }
      }
      if (!matched) {
        for (let i = 0; i < chapters.length; i++) {
          const fr = chapters[i];
          if (fr.hidden || fr.isRoot || fr.nameLower === fromChLower) continue;
          if (Math.hypot(cx - fr.inX, cy - fr.inY) <= 28) {
            if (activateChapter(fr)) {
              matched = true;
              break;
            }
          }
        }
      }

      if (!matched) {
        for (let i = 0; i < nodes.length; i++) {
          const nd = nodes[i];
          if (nd.id === activeCanvasGesture.fromSceneId) continue;
          if (cx >= nd.x - 6 && cx <= nd.x + nd.w + 6 && cy >= nd.y - 12 && cy <= nd.y + nd.h + 6) {
            if (!fromChLower || !nd.chapterLower || nd.chapterLower === fromChLower) {
              hoveredId = nd.id;
              cx = nd.x + nd.inOx;
              cy = nd.y + nd.inOy;
              activeCanvasGesture.hoveredSceneNode = nd;
              matched = true;
            } else {
              const tf = studioGraphState.chapterByKey.get(nd.chapterLower);
              if (activateChapter(tf)) {
                matched = true;
              }
            }
            break;
          }
        }
      }

      if (!matched) {
        for (let i = 0; i < chapters.length; i++) {
          const fr = chapters[i];
          if (fr.hidden || fr.isRoot || fr.nameLower === fromChLower) continue;
          if (cx >= fr.x - 10 && cx <= fr.x + fr.w + 10 && cy >= fr.y - 14 && cy <= fr.y + fr.h + 6) {
            if (activateChapter(fr)) break;
          }
        }
      }

      activeCanvasGesture.curX = cx;
      activeCanvasGesture.curY = cy;
      activeCanvasGesture.midOutX = midOutX;
      activeCanvasGesture.midOutY = midOutY;
      activeCanvasGesture.hoveredTargetId = hoveredId;
      updateTempWirePreview();
      return;
    }

    if (activeCanvasGesture.type === "chapter-wire-connect") {
      const ptWorld = clientToStudioWorld(activeCanvasGesture.viewport, clientX, clientY);
      let cx = ptWorld.wx;
      let cy = ptWorld.wy;
      let hoveredCh = "";
      activeCanvasGesture.hoveredChapterIn = null;

      const chapters = studioGraphState.chapters;
      const fromChLower = (activeCanvasGesture.fromChapter || "").trim().toLowerCase();

      for (let i = 0; i < chapters.length; i++) {
        const fr = chapters[i];
        if (fr.hidden || fr.isRoot || fr.nameLower === fromChLower || fr.refLower === fromChLower) continue;
        if (
          Math.hypot(cx - fr.inX, cy - fr.inY) <= 28 ||
          (cx >= fr.x - 10 && cx <= fr.x + fr.w + 10 && cy >= fr.y - 14 && cy <= fr.y + fr.h + 6)
        ) {
          hoveredCh = fr.ref;
          cx = fr.inX;
          cy = fr.inY;
          activeCanvasGesture.hoveredChapterIn = fr;
          break;
        }
      }

      activeCanvasGesture.curX = cx;
      activeCanvasGesture.curY = cy;
      activeCanvasGesture.hoveredTargetChapter = hoveredCh;
      updateTempWirePreview();
    }
  }

  function finishActiveCanvasGesture() {
    if (!activeCanvasGesture) return;
    const g = activeCanvasGesture;
    activeCanvasGesture = null;
    studioGraphState.overlayBatchDirty = true;

    const vp = qs("#timeline-scroll-viewport");
    if (vp && !editorCanvasState.spacePressed) {
      vp.classList.remove("is-panning");
    }

    if (g.type === "pan") {
      scheduleSaveEditorViewport();
      return;
    }

    if (g.type === "touch-pan" || g.type === "touch-pinch-pan") {
      if (g.moved) {
        suppressNextTouchClickUntil = Date.now() + 360;
        scheduleSaveEditorViewport();
      } else if (g.startedAddChoiceNode) {
        triggerCanvasAddChoiceForNode(g.startedAddChoiceNode);
      } else if (g.startedBranchRadioNode) {
        if (g.startedBranchRadioNode.branchRole === "secondary") {
          sendServerAction("SelectPrimaryBranchScene", {
            scene: g.startedBranchRadioNode.id,
            parent: g.startedBranchRadioNode.branchParentId || ""
          });
        }
      } else if (g.startedNodeId && !g.startedInteractive) {
        editorSelectionCleared = false;
        selectedNodeIds.clear();
        selectedNodeIds.add(g.startedNodeId);
        syncNodeSelectionDOM();
        renderStudioCanvas();
        selectNodeForContextPanel(g.startedNodeId);
      } else if (g.startedOnEmptyCanvas) {
        closeAllCustomSelects(null);
        editorSelectionCleared = true;
        if (selectedNodeIds.size > 0) {
          selectedNodeIds.clear();
        }
        syncNodeSelectionDOM();
        hideEditorContextPanel();
        renderStudioCanvas();
      }
      return;
    }

    if (g.type === "right-pan") {
      if (g.moved) {
        scheduleSaveEditorViewport();
      }
      return;
    }

    if (g.type === "choice-drag") {
      if (g.isTouch && g.moved) {
        suppressNextTouchClickUntil = Date.now() + 360;
      }
      renderStudioCanvas();
      if (g.node && g.startIdx !== g.currentIdx) {
        triggerCanvasReorderChoice(g.node, g.startIdx, g.currentIdx, null);
      } else if (g.sceneId) {
        selectNodeForContextPanel(g.sceneId);
      }
      return;
    }

    if (g.type === "dom-choice-drag") {
      if (g.row) {
        g.row.classList.remove("is-dragging-choice");
      }
      document.body.classList.remove("is-dragging-choice-row");
      if (g.startIdx !== g.currentIdx) {
        if (g.node) {
          triggerCanvasReorderChoice(g.node, g.startIdx, g.currentIdx, g.form);
        } else if (g.sceneId) {
          const params = {
            scene: g.sceneId,
            from_idx: String(g.startIdx),
            to_idx: String(g.currentIdx)
          };
          if (g.form) {
            const fd = new FormData(g.form);
            fd.forEach((val, key) => {
              params[key] = val;
            });
          }
          params.scene = g.sceneId;
          params.from_idx = String(g.startIdx);
          params.to_idx = String(g.currentIdx);
          sendServerAction("ReorderChoice", params);
        }
      }
      return;
    }

    if (g.type === "wire-pin-drag") {
      const wire = g.wire;
      if (wire && g.inserted && g.moved && wire.pins && wire.pins[g.pinIdx]) {
        if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
        let redundantCollinear = false;
        if (!g.isNewPin) {
          const savedPin = wire.pins.splice(g.pinIdx, 1)[0];
          routeSingleWireBasePts(wire, studioGraphState.nodes, studioGraphState.chapters);
          const testPts = wire.basePts || [];
          for (let j = 0; j < testPts.length - 1; j++) {
            if (distToSegmentSq(savedPin.x, savedPin.y, testPts[j].x, testPts[j].y, testPts[j + 1].x, testPts[j + 1].y) <= 16) {
              redundantCollinear = true;
              break;
            }
          }
          wire.pins.splice(g.pinIdx, 0, savedPin);
        }
        if (redundantCollinear) {
          wire.pins.splice(g.pinIdx, 1);
        }
        rerouteAfterWirePinMutation(wire);
        if (wire.wireKey) {
          saveWirePinsOnServer(wire.wireKey, wire.pins);
        }
      } else {
        renderStudioCanvas();
      }
      return;
    }

    if (g.type === "node-drag") {
      const items = g.items || [];
      renderStudioCanvas();
      if (g.moved && items.length > 0) {
        if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
        const payload = items.map((it) => ({
          sceneId: it.sceneId,
          x: it.node.x,
          y: it.node.y
        }));
        saveBatchNodePositionsOnServer(payload);
        if (selectedNodeIds.size === 1) {
          selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
        }
      } else if (!g.moved && g.sceneId) {
        if (!g.isMultiModifier && selectedNodeIds.size > 1) {
          editorSelectionCleared = false;
          selectedNodeIds.clear();
          selectedNodeIds.add(g.sceneId);
          syncNodeSelectionDOM();
          renderStudioCanvas();
        }
        selectNodeForContextPanel(g.sceneId);
      }
      return;
    }

    if (g.type === "chapter-drag") {
      const items = g.items || [];
      renderStudioCanvas();
      if (g.moved && items.length > 0) {
        if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
        const payload = items.map((it) => ({
          sceneId: it.sceneId,
          x: it.node.x,
          y: it.node.y
        }));
        saveBatchNodePositionsOnServer(payload);
      } else if (!g.moved && g.chapter) {
        selectChapterForContextPanel(g.chapter.name);
      }
      return;
    }

    if (g.type === "marquee-select") {
      editorSelectionCleared = selectedNodeIds.size === 0;
      syncNodeSelectionDOM();
      renderStudioCanvas();
      if (selectedNodeIds.size === 0) {
        hideEditorContextPanel();
      } else if (selectedNodeIds.size === 1) {
        selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
      }
      return;
    }

    if (g.type === "wire-connect") {
      if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
      if (g.hoveredTargetId && g.fromSceneId && g.hoveredTargetId !== g.fromSceneId) {
        const params = {
          from_scene: g.fromSceneId,
          choice_idx: g.choiceIdx,
          to_scene: g.hoveredTargetId
        };
        const contextForm = qs("#editor-context-form");
        if (contextForm && contextForm.getAttribute("data-context-scene-id") === g.fromSceneId) {
          const fd = new FormData(contextForm);
          fd.forEach((val, key) => {
            params[key] = val;
          });
        }
        sendServerAction("ConnectChoice", params);
      } else if (
        g.isBottomPort &&
        g.fromSceneId &&
        !g.hoveredTargetId &&
        g.hasExistingTarget &&
        Math.hypot(g.curX - g.startX, g.curY - g.startY) > 28
      ) {
        const params = {
          from_scene: g.fromSceneId,
          choice_idx: "0",
          to_scene: "END"
        };
        const contextForm = qs("#editor-context-form");
        if (contextForm && contextForm.getAttribute("data-context-scene-id") === g.fromSceneId) {
          const fd = new FormData(contextForm);
          fd.forEach((val, key) => {
            params[key] = val;
          });
        }
        sendServerAction("ConnectChoice", params);
      } else {
        updateTempWirePreview();
      }
      return;
    }

    if (g.type === "chapter-wire-connect") {
      if (g.isTouch) suppressNextTouchClickUntil = Date.now() + 360;
      if (g.hoveredTargetChapter && g.fromChapter) {
        sendServerAction("ConnectChapter", {
          from_chapter: g.fromChapter,
          old_target_chapter: g.oldTargetChapter || "",
          to_chapter: g.hoveredTargetChapter
        });
      } else if (
        g.fromChapter &&
        !g.hoveredTargetChapter &&
        g.oldTargetChapter &&
        Math.hypot(g.curX - g.startX, g.curY - g.startY) > 28
      ) {
        sendServerAction("ConnectChapter", {
          from_chapter: g.fromChapter,
          old_target_chapter: g.oldTargetChapter,
          to_chapter: "END"
        });
      } else {
        updateTempWirePreview();
      }
    }
  }

  document.addEventListener(
    "wheel",
    (e) => {
      const viewport = e.target.closest("#timeline-scroll-viewport");
      if (!viewport) return;
      if (e.target.closest(".custom-select-menu")) return;
      e.preventDefault();
      lastPointerClientPos.x = e.clientX;
      lastPointerClientPos.y = e.clientY;
      const sensitivity = e.ctrlKey || e.metaKey ? 0.0045 : 0.0018;
      const factor = Math.exp(-e.deltaY * sensitivity);
      zoomEditorAtPoint((editorCanvasState.zoom || 1.0) * factor, e.clientX, e.clientY);
    },
    { passive: false }
  );

  document.addEventListener(
    "gesturestart",
    (e) => {
      if (!e.target.closest("#timeline-scroll-viewport")) return;
      e.preventDefault();
      editorCanvasState.gestureBaseZoom = editorCanvasState.zoom || 1.0;
    },
    { passive: false }
  );

  document.addEventListener(
    "gesturechange",
    (e) => {
      if (!e.target.closest("#timeline-scroll-viewport")) return;
      e.preventDefault();
      if (activeCanvasGesture && activeCanvasGesture.isTouch) return;
      zoomEditorAtPoint(editorCanvasState.gestureBaseZoom * (e.scale || 1.0), e.clientX, e.clientY);
    },
    { passive: false }
  );

  document.addEventListener(
    "gestureend",
    (e) => {
      if (!e.target.closest("#timeline-scroll-viewport")) return;
      e.preventDefault();
    },
    { passive: false }
  );

  document.addEventListener(
    "touchstart",
    (e) => {
      const viewport =
        (e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null) ||
        (activeCanvasGesture && activeCanvasGesture.viewport ? activeCanvasGesture.viewport : null);
      if (!viewport) {
        closeCanvasContextMenu();
        return;
      }

      if (e.touches.length >= 2) {
        e.preventDefault();
        closeCanvasContextMenu();
        startTouchPinchPanGesture(viewport, e.touches[0], e.touches[1]);
        return;
      }

      if (e.touches.length !== 1) return;
      const t = e.touches[0];
      lastPointerClientPos.x = t.clientX;
      lastPointerClientPos.y = t.clientY;

      if (e.target.closest("#canvas-context-menu, .custom-select-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #editor-empty-state, #btn-expand-scene-text")) {
        return;
      }
      closeCanvasContextMenu();
      hideFloatingChapterInput(true);
      hideFloatingNodeTitleInput(true);
      hideFloatingChoiceInput(true);

      const ptWorld = clientToStudioWorld(viewport, t.clientX, t.clientY);
      const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);

      if (hit.type === "chapter-out-port") {
        e.preventDefault();
        e.stopPropagation();
        beginChapterWireConnectGesture(hit, viewport, true);
        return;
      }

      if (hit.type === "scene-out-port") {
        e.preventDefault();
        e.stopPropagation();
        beginWireConnectGesture(hit, viewport, true);
        return;
      }

      if (hit.type === "chapter-grip" || hit.type === "chapter-title") {
        e.preventDefault();
        beginChapterDragGesture(hit.chapter, viewport, t.clientX, t.clientY, true, hit.type === "chapter-title");
        return;
      }

      if (hit.type === "node-grip") {
        e.preventDefault();
        beginNodeDragGesture(hit.node, viewport, t.clientX, t.clientY, true, false);
        return;
      }

      if (hit.type === "choice-grip") {
        e.preventDefault();
        beginChoiceDragGesture(hit.node, hit.choiceIdx, viewport, t.clientX, t.clientY, true);
        return;
      }

      if (hit.type === "wire-pin" || hit.type === "wire") {
        e.preventDefault();
        beginWirePinDragGesture(hit, viewport, t.clientX, t.clientY, true);
        return;
      }

      activeCanvasGesture = {
        type: "touch-pan",
        isTouch: true,
        viewport: viewport,
        startClientX: t.clientX,
        startClientY: t.clientY,
        startPanX: editorCanvasState.panX,
        startPanY: editorCanvasState.panY,
        moved: false,
        startedInteractive: hit.type === "add-choice-btn" || hit.type === "branch-radio",
        startedAddChoiceNode: hit.type === "add-choice-btn" ? hit.node : null,
        startedBranchRadioNode: hit.type === "branch-radio" ? hit.node : null,
        startedNodeId: hit.type === "node-card" ? hit.node.id : "",
        startedOnEmptyCanvas: hit.type === "empty"
      };
    },
    { passive: false }
  );

  document.addEventListener(
    "touchmove",
    (e) => {
      if (!activeCanvasGesture && e.touches.length >= 2) {
        const vp = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
        if (vp) {
          e.preventDefault();
          startTouchPinchPanGesture(vp, e.touches[0], e.touches[1]);
          return;
        }
      }
      if (!activeCanvasGesture || !activeCanvasGesture.isTouch) return;

      if (e.touches.length >= 2) {
        e.preventDefault();
        if (activeCanvasGesture.type !== "touch-pinch-pan") {
          const vp = activeCanvasGesture.viewport || qs("#timeline-scroll-viewport");
          if (vp) {
            startTouchPinchPanGesture(vp, e.touches[0], e.touches[1]);
          }
          return;
        }
        const t0 = e.touches[0];
        const t1 = e.touches[1];
        const curMidX = (t0.clientX + t1.clientX) * 0.5;
        const curMidY = (t0.clientY + t1.clientY) * 0.5;
        const curDist = Math.max(8, Math.hypot(t1.clientX - t0.clientX, t1.clientY - t0.clientY));
        const scale = curDist / activeCanvasGesture.startDist;
        const newZoom = Math.max(0.25, Math.min(2.5, activeCanvasGesture.startZoom * scale));
        const rect = activeCanvasGesture.viewport.getBoundingClientRect();
        const vx = curMidX - rect.left;
        const vy = curMidY - rect.top;

        editorCanvasState.zoom = newZoom;
        editorCanvasState.panX = vx - activeCanvasGesture.worldX * newZoom;
        editorCanvasState.panY = vy - activeCanvasGesture.worldY * newZoom;

        applyEditorCanvasTransform();
        return;
      }

      if (e.touches.length === 1) {
        const t = e.touches[0];
        lastPointerClientPos.x = t.clientX;
        lastPointerClientPos.y = t.clientY;
        updateCanvasDragMove(t.clientX, t.clientY);
        if (activeCanvasGesture && (activeCanvasGesture.moved || activeCanvasGesture.type !== "touch-pan")) {
          e.preventDefault();
        }
      }
    },
    { passive: false }
  );

  function handleCanvasTouchEndOrCancel(e) {
    if (!activeCanvasGesture || !activeCanvasGesture.isTouch) return;

    if (activeCanvasGesture.type === "touch-pinch-pan" && e.touches && e.touches.length === 1) {
      const rem = e.touches[0];
      activeCanvasGesture = {
        type: "touch-pan",
        isTouch: true,
        viewport: activeCanvasGesture.viewport,
        startClientX: rem.clientX,
        startClientY: rem.clientY,
        startPanX: editorCanvasState.panX,
        startPanY: editorCanvasState.panY,
        moved: true,
        startedInteractive: false,
        startedNodeId: "",
        startedOnEmptyCanvas: false
      };
      return;
    }

    if (e.touches && e.touches.length > 0) return;

    const wasMoved = Boolean(activeCanvasGesture.moved);
    finishActiveCanvasGesture();
    if (wasMoved && e.cancelable) {
      e.preventDefault();
    }
  }

  document.addEventListener("touchend", handleCanvasTouchEndOrCancel, { passive: false });
  document.addEventListener("touchcancel", handleCanvasTouchEndOrCancel, { passive: false });

  document.addEventListener("mousedown", (e) => {
    if (e.button === 0 && e.target && typeof e.target.closest === "function") {
      const domChoiceGrip = e.target.closest("[data-choice-grip]");
      if (domChoiceGrip && !(typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview())) {
        const row = domChoiceGrip.closest(".context-choice-item, .modal-choice-row");
        const list = row ? row.parentElement : null;
        if (row && list && list.children.length >= 2) {
          e.preventDefault();
          e.stopPropagation();
          const children = Array.from(list.children);
          const startIdx = children.indexOf(row);
          if (startIdx >= 0) {
            const form = row.closest("form");
            const sceneIdInput = form ? qs('input[name="scene_id"], input[name="scene"]', form) : null;
            const sceneId =
              (form && form.getAttribute("data-context-scene-id")) ||
              (sceneIdInput ? sceneIdInput.value : "") ||
              "";
            const nd = sceneId ? studioGraphState.nodeById.get(sceneId) : null;
            row.classList.add("is-dragging-choice");
            document.body.classList.add("is-dragging-choice-row");
            activeCanvasGesture = {
              type: "dom-choice-drag",
              isTouch: false,
              row: row,
              list: list,
              form: form,
              node: nd,
              sceneId: sceneId,
              startIdx: startIdx,
              currentIdx: startIdx,
              startClientX: e.clientX,
              startClientY: e.clientY,
              moved: false
            };
            return;
          }
        }
      }
    }

    const viewport = e.target.closest("#timeline-scroll-viewport");
    if (!viewport) {
      closeCanvasContextMenu();
      hideFloatingChapterInput(true);
      hideFloatingNodeTitleInput(true);
      hideFloatingChoiceInput(true);
      return;
    }

    if (e.target.closest("#canvas-context-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #editor-empty-state, #btn-expand-scene-text")) {
      return;
    }
    closeCanvasContextMenu();
    hideFloatingChapterInput(true);
    hideFloatingNodeTitleInput(true);
    hideFloatingChoiceInput(true);

    if (e.button === 1 || (e.button === 0 && editorCanvasState.spacePressed)) {
      e.preventDefault();
      viewport.classList.add("is-panning");
      activeCanvasGesture = {
        type: "pan",
        startClientX: e.clientX,
        startClientY: e.clientY,
        startPanX: editorCanvasState.panX,
        startPanY: editorCanvasState.panY
      };
      return;
    }

    if (e.button === 2 && !e.target.closest("input, textarea")) {
      activeCanvasGesture = {
        type: "right-pan",
        startClientX: e.clientX,
        startClientY: e.clientY,
        startPanX: editorCanvasState.panX,
        startPanY: editorCanvasState.panY,
        moved: false
      };
      return;
    }

    if (e.button !== 0) return;

    if (
      document.activeElement &&
      document.activeElement !== e.target &&
      typeof document.activeElement.blur === "function" &&
      document.activeElement.matches &&
      document.activeElement.matches("input, textarea, select")
    ) {
      document.activeElement.blur();
    }

    const ptWorld = clientToStudioWorld(viewport, e.clientX, e.clientY);
    const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);

    if (hit.type === "chapter-out-port") {
      e.preventDefault();
      e.stopPropagation();
      beginChapterWireConnectGesture(hit, viewport, false);
      return;
    }

    if (hit.type === "scene-out-port") {
      e.preventDefault();
      e.stopPropagation();
      beginWireConnectGesture(hit, viewport, false);
      return;
    }

    if (hit.type === "add-choice-btn") {
      e.preventDefault();
      e.stopPropagation();
      triggerCanvasAddChoiceForNode(hit.node);
      return;
    }

    if (hit.type === "branch-radio") {
      e.preventDefault();
      e.stopPropagation();
      if (hit.node && hit.node.branchRole === "secondary") {
        sendServerAction("SelectPrimaryBranchScene", {
          scene: hit.node.id,
          parent: hit.node.branchParentId || ""
        });
      }
      return;
    }

    if (hit.type === "edit-scene-btn") {
      e.preventDefault();
      e.stopPropagation();
      editorSelectionCleared = false;
      selectedNodeIds.clear();
      if (hit.node && hit.node.id) selectedNodeIds.add(hit.node.id);
      syncNodeSelectionDOM();
      renderStudioCanvas();
      sendServerAction("OpenSceneTextModal", { scene: hit.node.id });
      return;
    }

    if (hit.type === "delete-scene-btn") {
      e.preventDefault();
      e.stopPropagation();
      sendServerAction("PromptDeleteScene", { scene: hit.node.id });
      return;
    }

    if (hit.type === "choice-trash-btn") {
      e.preventDefault();
      e.stopPropagation();
      sendServerAction("PromptRemoveChoice", {
        scene: hit.node.id,
        choice_idx: String(hit.choiceIdx)
      });
      return;
    }

    if (hit.type === "choice-label") {
      e.preventDefault();
      e.stopPropagation();
      editorSelectionCleared = false;
      selectedNodeIds.clear();
      if (hit.node && hit.node.id) selectedNodeIds.add(hit.node.id);
      syncNodeSelectionDOM();
      if (qs("#modal-scene-text") && hit.node && hit.node.id) {
        selectNodeForContextPanel(hit.node.id);
      }
      openFloatingChoiceInput(hit.node, hit.choiceIdx);
      return;
    }

    if (hit.type === "node-title") {
      e.preventDefault();
      e.stopPropagation();
      editorSelectionCleared = false;
      selectedNodeIds.clear();
      if (hit.node && hit.node.id) selectedNodeIds.add(hit.node.id);
      syncNodeSelectionDOM();
      if (qs("#modal-scene-text") && hit.node && hit.node.id) {
        selectNodeForContextPanel(hit.node.id);
      }
      openFloatingNodeTitleInput(hit.node);
      return;
    }

    if (hit.type === "chapter-grip" || hit.type === "chapter-title") {
      e.preventDefault();
      beginChapterDragGesture(hit.chapter, viewport, e.clientX, e.clientY, false, hit.type === "chapter-title");
      return;
    }

    const isMultiModifier = Boolean(e.shiftKey || e.metaKey || e.ctrlKey);

    if (hit.type === "node-grip") {
      e.preventDefault();
      beginNodeDragGesture(hit.node, viewport, e.clientX, e.clientY, false, isMultiModifier);
      return;
    }

    if (hit.type === "choice-grip") {
      e.preventDefault();
      beginChoiceDragGesture(hit.node, hit.choiceIdx, viewport, e.clientX, e.clientY, false);
      return;
    }

    if (hit.type === "wire-pin" || hit.type === "wire") {
      e.preventDefault();
      beginWirePinDragGesture(hit, viewport, e.clientX, e.clientY, false);
      return;
    }

    if (hit.type === "node-card") {
      e.preventDefault();
      const sceneId = hit.node.id;
      if (isMultiModifier) {
        if (selectedNodeIds.has(sceneId)) {
          selectedNodeIds.delete(sceneId);
        } else if (sceneId) {
          selectedNodeIds.add(sceneId);
        }
        editorSelectionCleared = selectedNodeIds.size === 0;
        syncNodeSelectionDOM();
        renderStudioCanvas();
        if (selectedNodeIds.size === 1) {
          selectNodeForContextPanel(Array.from(selectedNodeIds)[0]);
        }
        return;
      }
      editorSelectionCleared = false;
      selectedNodeIds.clear();
      if (sceneId) selectedNodeIds.add(sceneId);
      syncNodeSelectionDOM();
      renderStudioCanvas();
      selectNodeForContextPanel(sceneId);
      return;
    }

    // Empty canvas background: start rubber-band marquee multi-selection
    e.preventDefault();
    const isAdditive = Boolean(e.shiftKey || e.metaKey || e.ctrlKey);
    const baseSelection = isAdditive ? new Set(selectedNodeIds) : new Set();
    if (!isAdditive) {
      editorSelectionCleared = true;
      if (selectedNodeIds.size > 0) {
        selectedNodeIds.clear();
      }
      syncNodeSelectionDOM();
      hideEditorContextPanel();
      renderStudioCanvas();
    }
    const vpRect = viewport.getBoundingClientRect();
    activeCanvasGesture = {
      type: "marquee-select",
      viewport: viewport,
      vpRect: vpRect,
      startClientX: e.clientX,
      startClientY: e.clientY,
      baseSelection: baseSelection,
      marqueeRect: null,
      moved: false
    };
  });

  document.addEventListener("mousemove", (e) => {
    lastPointerClientPos.x = e.clientX;
    lastPointerClientPos.y = e.clientY;
    if (activeCanvasGesture) {
      if (!activeCanvasGesture.isTouch) {
        updateCanvasDragMove(e.clientX, e.clientY);
      }
      return;
    }
    const viewport = e.target && e.target.closest ? e.target.closest("#timeline-scroll-viewport") : null;
    if (viewport && !viewport.contains(e.target)) {
      if (studioGraphState.hoveredItem) {
        studioGraphState.hoveredItem = null;
        studioGraphState.overlayBatchDirty = true;
        renderStudioCanvas();
      }
      return;
    }
    if (!viewport || e.target.closest("#canvas-context-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #btn-expand-scene-text")) {
      if (studioGraphState.hoveredItem) {
        studioGraphState.hoveredItem = null;
        studioGraphState.overlayBatchDirty = true;
        renderStudioCanvas();
      }
      return;
    }
    const ptWorld = clientToStudioWorld(viewport, e.clientX, e.clientY);
    const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);
    updateCanvasHoverCursor(viewport, hit);
    const nextHover = hit.type === "empty" ? null : hit;
    if (!isSameHoveredCanvasItem(studioGraphState.hoveredItem, nextHover)) {
      studioGraphState.hoveredItem = nextHover;
      studioGraphState.overlayBatchDirty = true;
      renderStudioCanvas();
    }
  });

  document.addEventListener("mouseup", () => {
    if (!activeCanvasGesture || activeCanvasGesture.isTouch) return;
    finishActiveCanvasGesture();
  });

  document.addEventListener("contextmenu", (e) => {
    const vp = e.target.closest("#timeline-scroll-viewport");
    if (!vp) return;
    if (e.target.closest("input, textarea")) return;
    e.preventDefault();
    if (editorCanvasState.suppressNextContextMenu) {
      editorCanvasState.suppressNextContextMenu = false;
      return;
    }
    const ptWorld = clientToStudioWorld(vp, e.clientX, e.clientY);
    const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);
    if (hit.type === "wire-pin") {
      removeWirePinAt(hit.wire, hit.pinIdx);
      return;
    }
    closeAllCustomSelects(null);
    const rect = vp.getBoundingClientRect();
    const z = editorCanvasState.zoom || 1.0;
    const vx = e.clientX - rect.left;
    const vy = e.clientY - rect.top;
    pendingContextMenuWorldPos = {
      x: (vx - editorCanvasState.panX) / z - 96,
      y: (vy - editorCanvasState.panY) / z - 20
    };
    const menu = qs("#canvas-context-menu", vp);
    if (menu) {
      menu.style.left = Math.max(8, Math.min(rect.width - 230, vx)) + "px";
      menu.style.top = Math.max(8, Math.min(rect.height - 150, vy)) + "px";
      menu.classList.remove("hidden");
    }
  });

  document.addEventListener("dblclick", (e) => {
    const vp = e.target.closest("#timeline-scroll-viewport");
    if (!vp) return;
    if (e.target.closest("#canvas-context-menu, #canvas-chapter-input, #canvas-node-title-input, #canvas-choice-input, #editor-empty-state, #btn-expand-scene-text")) return;
    const ptWorld = clientToStudioWorld(vp, e.clientX, e.clientY);
    const hit = hitTestStudioCanvas(ptWorld.wx, ptWorld.wy);
    if (hit.type === "wire-pin") {
      e.preventDefault();
      removeWirePinAt(hit.wire, hit.pinIdx);
      return;
    }
    if (hit.type === "chapter-title" || hit.type === "chapter-grip") {
      e.preventDefault();
      if (!(typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview())) {
        openFloatingChapterInput(hit.chapter);
      } else {
        selectChapterForContextPanel(hit.chapter.name);
      }
      return;
    }
    if (
      hit.type === "node-card" ||
      hit.type === "node-title" ||
      hit.type === "choice-label" ||
      hit.type === "edit-scene-btn"
    ) {
      e.preventDefault();
      hideFloatingNodeTitleInput(true);
      hideFloatingChoiceInput(true);
      sendServerAction("OpenSceneTextModal", { scene: hit.node.id });
      return;
    }
    if (hit.type !== "empty") return;
    if (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview()) return;
    e.preventDefault();
    createSceneNodeAtWorldCoords(ptWorld.wx - 96, ptWorld.wy - 20);
  });

  document.addEventListener("selectstart", (e) => {
    if (!e.target || typeof e.target.closest !== "function") return;
    if (e.target.closest("input, textarea")) return;
    if (e.target.closest("#timeline-scroll-viewport")) {
      e.preventDefault();
    }
  });

  document.addEventListener("input", (e) => {
    if (e.target && e.target.classList && e.target.classList.contains("node-meta-textarea")) {
      autoResizeInspectorTextarea(e.target);
    }
    if (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview()) {
      return;
    }
    if (e.target && e.target.classList && e.target.classList.contains("chapter-group-title-input")) {
      scheduleChapterLiveSave(e.target);
      return;
    }
    if (e.target && e.target.id === "book-settings-title-input") {
      scheduleBookTitleLiveSave(e.target);
      return;
    }
    if (e.target && e.target.getAttribute && e.target.getAttribute("data-test-var-input")) {
      const varName = (e.target.getAttribute("data-test-var-input") || "").trim().toLowerCase();
      const modalForm = e.target.closest("form#form-scene-text");
      if (varName && modalForm) {
        const curSpec = modalForm.getAttribute("data-test-vars") || "";
        const map = new Map();
        curSpec.split(";").forEach((p) => {
          const eq = p.indexOf("=");
          if (eq > 0) map.set(p.slice(0, eq).trim().toLowerCase(), p.slice(eq + 1).trim());
        });
        map.set(varName, (e.target.value || "").trim());
        const nextParts = [];
        map.forEach((v, k) => nextParts.push(k + "=" + v));
        modalForm.setAttribute("data-test-vars", nextParts.join(";"));
        const modalTa = qs("#modal-scene-textarea", modalForm);
        if (modalTa && typeof syncSceneTextareaHighlight === "function") {
          syncSceneTextareaHighlight(modalTa);
        }
      }
      return;
    }
    const bookForm = e.target.closest("form.editor-book-form");
    if (bookForm) {
      scheduleBookSettingsLiveSave(bookForm);
      return;
    }
    const form = e.target.closest("form.editor-context-form, form#form-scene-text");
    if (!form) return;
    if (typeof validateRuleInputElement === "function") {
      validateRuleInputElement(e.target, form);
    }
    const sceneIdInput = qs('input[name="scene_id"]', form);
    const sceneId =
      form.getAttribute("data-context-scene-id") ||
      (sceneIdInput ? sceneIdInput.value : "") ||
      "";
    const nd = sceneId ? studioGraphState.nodeById.get(sceneId) : null;
    if (e.target && e.target.name === "chapter" && nd) {
      nd.chapter = e.target.value || "Rozdział 1";
      nd.chapterLower = nd.chapter.toLowerCase();
      updateChapterGroupFrames();
      drawStudioWires();
    }
    if (e.target && (e.target.name === "name" || e.target.name === "explicit_id") && nd) {
      const nameInput = qs('input[name="name"]', form);
      const expIdInput = qs('input[name="explicit_id"]', form);
      const displayTitle =
        (nameInput && nameInput.value.trim()) ||
        (expIdInput && expIdInput.value.trim()) ||
        sceneId;
      nd.name = displayTitle;
      studioGraphState.staticBatchDirty = true;
      renderStudioCanvas();
    }
    if (e.target && e.target.name && e.target.name.indexOf("choice_label_") === 0 && nd) {
      const idx = Number(e.target.getAttribute("data-choice-idx") || e.target.name.replace("choice_label_", ""));
      if (nd.ports && nd.ports[idx]) {
        nd.ports[idx].label = e.target.value || "Dalej";
        studioGraphState.staticBatchDirty = true;
        renderStudioCanvas();
      }
    }
    if (e.target && e.target.name && e.target.name.indexOf("choice_cond_") === 0) {
      const idx = Number(e.target.getAttribute("data-choice-idx") || e.target.name.replace("choice_cond_", ""));
      const hasCond = Boolean((e.target.value || "").trim());
      const rowEl = e.target.closest(".context-choice-item, .modal-choice-row");
      if (rowEl) {
        const portIcon = qs(".choice-port-icon", rowEl);
        if (portIcon) {
          portIcon.classList.toggle("port-diamond", hasCond);
        }
      }
      if (nd && nd.ports && nd.ports[idx]) {
        nd.ports[idx].isConditional = hasCond;
        studioGraphState.staticBatchDirty = true;
        renderStudioCanvas();
      }
    }
    scheduleNodeLiveSave(form);
  });

  function commitVariablePanelInput(inputEl) {
    if (!inputEl || (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview())) return;
    const form = inputEl.closest("form.var-panel-inline-form");
    if (!form) return;
    const params = {};
    const fd = new FormData(form);
    fd.forEach((val, key) => {
      params[key] = val;
    });
    sendServerAction("UpdateBookVariable", params);
  }

  function commitBreakdownRenameInput(inputEl) {
    if (!inputEl || (typeof isEditorHistoryPreview === "function" && isEditorHistoryPreview())) return;
    const form = inputEl.closest("form.breakdown-rename-form");
    if (!form) return;
    const actionName = (form.getAttribute("action") || "").trim();
    if (!actionName) return;
    const oldInp = qs('input[name="old_name"], input[name="old_location"]', form);
    const oldVal = ((inputEl.getAttribute("data-old-value") || (oldInp ? oldInp.value : "")) || "").trim();
    const newVal = (inputEl.value || "").trim();
    if (!newVal && oldVal) {
      inputEl.value = oldVal;
      return;
    }
    if (oldVal && newVal && oldVal !== newVal) {
      inputEl.setAttribute("data-old-value", newVal);
      if (oldInp) oldInp.value = oldVal;
      const params = {};
      const fd = new FormData(form);
      fd.forEach((val, key) => {
        params[key] = val;
      });
      sendServerAction(actionName, params);
    }
  }

  function commitTestVarInput(inputEl) {
    if (!inputEl) return;
    const varName = (inputEl.getAttribute("data-test-var-input") || "").trim();
    if (!varName) return;
    const params = {
      var_name: varName,
      var_value: (inputEl.value || "").trim()
    };
    const modalForm = inputEl.closest ? inputEl.closest("form#form-scene-text") : null;
    if (modalForm) {
      const fd = new FormData(modalForm);
      fd.forEach((v, k) => {
        if (params[k] === undefined) params[k] = v;
      });
    }
    sendServerAction("UpdateSceneTextTestVar", params);
  }

  document.addEventListener("change", (e) => {
    if (e.target && e.target.id === "canvas-node-title-input") {
      commitNodeTitleInput(e.target);
    } else if (e.target && e.target.id === "canvas-choice-input") {
      commitChoiceInput(e.target);
    } else if (e.target && e.target.classList && e.target.classList.contains("chapter-group-title-input")) {
      commitChapterTitleInput(e.target);
    } else if (e.target && e.target.id === "book-settings-title-input") {
      commitBookTitleInput(e.target);
    } else if (e.target && e.target.classList && e.target.classList.contains("breakdown-name-input")) {
      commitBreakdownRenameInput(e.target);
    } else if (e.target && e.target.closest && e.target.closest("form.var-panel-inline-form")) {
      commitVariablePanelInput(e.target);
    } else if (e.target && e.target.getAttribute && e.target.getAttribute("data-test-var-input")) {
      commitTestVarInput(e.target);
    }
  });

  document.addEventListener(
    "blur",
    (e) => {
      if (e.target && e.target.id === "canvas-node-title-input") {
        hideFloatingNodeTitleInput(true);
      } else if (e.target && e.target.id === "canvas-choice-input") {
        hideFloatingChoiceInput(true);
      } else if (e.target && e.target.classList && e.target.classList.contains("chapter-group-title-input")) {
        if (e.target.id === "canvas-chapter-input") {
          hideFloatingChapterInput(true);
        } else {
          commitChapterTitleInput(e.target);
        }
      } else if (e.target && e.target.id === "book-settings-title-input") {
        commitBookTitleInput(e.target);
      } else if (e.target && e.target.classList && e.target.classList.contains("breakdown-name-input")) {
        commitBreakdownRenameInput(e.target);
      }
    },
    true
  );
}
