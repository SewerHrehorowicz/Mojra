"use strict";

const EditorHistoryController = {
  data: {
    drag: null
  },

  bindings() {
    return {
      "#editor-history-panel .btn-close-history": {
        click: Dispatcher.action("CloseEditorHistoryPanel")
      },
      "#editor-history-panel .history-version-save-form": {
        submit: Dispatcher.action("SaveHistoryVersion")
      },
      "#history-graph-scroll": {
        scroll: this.updateScrollCue
      },
      "#editor-history-panel .history-rail-hit": {
        click: Dispatcher.action("SwitchHistoryBranch")
      },
      "#editor-history-panel .history-rail-node-active-branch": {
        pointerdown: this.startNodeDrag,
        pointermove: this.moveNodeDrag,
        pointerup: this.endNodeDrag,
        click: Dispatcher.action("PreviewHistoryRevision")
      },
      "#editor-history-panel .history-rail-node-other-branch": {
        pointerdown: this.startNodeDrag,
        pointermove: this.moveNodeDrag,
        pointerup: this.endNodeDrag,
        click: Dispatcher.action("SwitchHistoryBranch")
      },
      "#editor-history-panel .history-row-grip": {
        pointerdown: this.startNodeDrag,
        pointermove: this.moveNodeDrag,
        pointerup: this.endNodeDrag
      },
      "#editor-history-panel .history-row-box": {
        click: Dispatcher.action("PreviewHistoryRevision")
      },
      "#editor-history-panel .history-branch-create-form": {
        submit: Dispatcher.action("PromptCreateHistoryBranch")
      }
    };
  },

  settings: {
    minMovePx: 4,
    scrollCueMarginPx: 6,
    trueStr: "true",
    branchArrowPrefix: "→ ",
    restorePrefix: "↺ #",
    selectors: {
      dropHoverCells: ".history-lane-cell.is-drop-hover",
      previewRow: "#history-drop-preview-row",
      previewTitle: "#history-drop-preview-title",
      previewMeta: "#history-drop-preview-meta",
      scrollCue: "#history-scroll-more-cue"
    },
    classes: {
      laneCell: "history-lane-cell",
      dropHover: "is-drop-hover",
      hidden: "hidden"
    },
    actions: {
      switchBranch: "SwitchHistoryBranch",
      restoreRevision: "RestoreHistoryRevision",
      copyRevision: "CopyHistoryRevisionToBranch"
    }
  },

  startNodeDrag(params) {
    if (!params.revision) return;
    this.data.drag = {
      rev: params.revision,
      branch: params.branch || "",
      label: params.label || "",
      isCurrent: params.is_current === this.settings.trueStr,
      startX: params.client_x || 0,
      startY: params.client_y || 0,
      moved: false
    };
  },

  moveNodeDrag(params) {
    if (!this.data.drag) return;
    const s = this.settings;
    if (!this.data.drag.moved) {
      if (Math.hypot((params.client_x || 0) - this.data.drag.startX, (params.client_y || 0) - this.data.drag.startY) < s.minMovePx) return;
      this.data.drag.moved = true;
    }
    Array.from(document.querySelectorAll(s.selectors.dropHoverCells)).forEach((c) => c.classList.remove(s.classes.dropHover));
    const drop = params.drop_target || {};
    const targetBranch = drop.branch || "";
    const targetRev = drop.revision || "";
    if (params.drop_el && params.drop_el.classList && params.drop_el.classList.contains(s.classes.laneCell)) {
      params.drop_el.classList.add(s.classes.dropHover);
    }
    const previewRow = document.querySelector(s.selectors.previewRow);
    const previewTitle = document.querySelector(s.selectors.previewTitle);
    const previewMeta = document.querySelector(s.selectors.previewMeta);
    if (!previewRow) return;
    const isDiffBranch = Boolean(targetBranch && targetBranch !== this.data.drag.branch);
    const isDiffRev = Boolean(this.data.drag.isCurrent && targetRev && targetRev !== this.data.drag.rev && (!targetBranch || targetBranch === this.data.drag.branch));
    if (isDiffBranch || isDiffRev) {
      previewRow.classList.remove(s.classes.hidden);
      if (previewTitle) previewTitle.textContent = this.data.drag.label || "#" + this.data.drag.rev;
      if (previewMeta) previewMeta.textContent = isDiffBranch ? s.branchArrowPrefix + targetBranch : s.restorePrefix + targetRev;
    } else {
      previewRow.classList.add(s.classes.hidden);
    }
  },

  endNodeDrag(params) {
    const s = this.settings;
    const d = this.data.drag;
    this.data.drag = null;
    Array.from(document.querySelectorAll(s.selectors.dropHoverCells)).forEach((c) => c.classList.remove(s.classes.dropHover));
    const previewRow = document.querySelector(s.selectors.previewRow);
    if (previewRow) previewRow.classList.add(s.classes.hidden);
    if (!d || !d.moved || !params.drop_target) return;
    const targetBranch = params.drop_target.branch || "";
    const targetRev = params.drop_target.revision || "";
    if (d.isCurrent) {
      if (targetBranch && targetBranch !== d.branch) {
        Dispatcher.post(s.actions.switchBranch, { branch: targetBranch });
      } else if (targetRev && targetRev !== d.rev) {
        Dispatcher.post(s.actions.restoreRevision, { revision: targetRev });
      }
    } else if (targetBranch && targetBranch !== d.branch) {
      Dispatcher.post(s.actions.copyRevision, { revision: d.rev, target_branch: targetBranch });
    }
  },

  updateScrollCue(_params, el) {
    const s = this.settings;
    const cue = document.querySelector(s.selectors.scrollCue);
    if (!el || !cue) return;
    cue.classList.toggle(s.classes.hidden, el.scrollHeight - el.scrollTop - el.clientHeight <= s.scrollCueMarginPx);
  }
};
Dispatcher.register("EditorHistoryController", EditorHistoryController);
