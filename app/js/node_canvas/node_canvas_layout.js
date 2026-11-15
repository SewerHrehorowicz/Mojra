"use strict";

// NodeCanvasLayout: Pure in-memory layout state of the currently loaded book.
// Holds chapters, nodes, ports, resting positions, routed wires, and wire pins.
// Contains zero methods and zero logic — all geometry, routing, hit-testing,
// and layout calculation helpers live in NodeCanvasHelpers.
const NodeCanvasLayout = {
  layoutHash: "",

  studioGraphState: {
    chapters: [],
    chapterByKey: new Map(),
    nodes: [],
    nodeById: new Map(),
    posById: new Map(),
    wires: [],
    wirePins: new Map(),
    nodeSpatialGrid: null,
    hoveredItem: null,
    editingChapter: null,
    editingNodeTitle: null,
    editingChoice: null,
    staticBatchDirty: true,
    overlayBatchDirty: true
  }
};
