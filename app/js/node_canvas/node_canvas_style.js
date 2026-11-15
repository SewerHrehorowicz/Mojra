"use strict";

// NodeCanvasStyle: Declarative visual specification ("CSS for WebGL canvas").
// Owns all dimensions, paddings, grid steps, MSDF typography metrics, UI atlas
// UV coordinates, and cursor mappings. Contains zero state and zero logic.
const NodeCanvasStyle = {
  grid: {
    size: 24,
    spatialShift: 8,
    minObstacles: 12
  },

  card: {
    w: 192,
    halfW: 96,
    headerH: 34,
    choiceH: 24,
    footerH: 38,
    baseH: 72,
    minChapterH: 120,
    btnSize: 24,
    pad: 8,
    gripW: 24,
    titleLeft: 28,
    badgeH: 17,
    badgePadX: 10,
    startBadgeW: 48,
    entryBadgeW: 54,
    radioSize: 8,
    radioReserveW: 16,
    radioYOffset: 13,
    laneSpreadBase: 10,
    laneSpreadStep: 14
  },

  pad: {
    x: 36,
    top: 52,
    bottom: 44,
    w: 72,
    h: 96
  },

  gap: {
    grid: 24,
    chapter: 24,
    node: 48
  },

  wire: {
    grid: 24,
    laneStep: 6,
    pad: 14,
    tightPad: 10,
    collectorOffset: 12,
    bboxPad: 18
  },

  uv: {
    grid: -100000.0,
    gridTest: -80000.0,
    gridOffset: 200000.0,
    screen: -50000.0,
    screenTest: -30000.0,
    solid: 30.0,
    fontLg: 20.0,
    fontSm: 10.0,
    dashDuty: 0.54
  },

  hit: {
    portRadiusSq: 100,
    pinBarDistSq: 64,
    wireDistSq: 49,
    wireBoundsPad: 8,
    chapterHeaderH: 28,
    chapterGripW: 26,
    choiceGripRight: 20,
    choiceTrashOffset: 26
  },

  typography: {
    emPx: 23.0,
    pixelRange: 3.0,
    styleIds: {
      title: 0,
      choice: 1,
      chapter: 2,
      badge: 3,
      btn: 4
    },
    styles: [
      { ttfId: 0, size: 13, track: 0.0 },
      { ttfId: 1, size: 11, track: 0.0 },
      { ttfId: 2, size: 11, track: 0.65 },
      { ttfId: 2, size: 9, track: 0.45 },
      { ttfId: 1, size: 10, track: 0.6 }
    ]
  },

  camera: {
    minZoom: 0.25,
    maxZoom: 2.2,
    defaultZoom: 1,
    zoomStep: 0.15,
    zoomEpsilon: 0.001,
    half: 0.5,
    startFocusXRatio: 0.25,
    fallbackWidth: 900,
    fallbackHeight: 600,
    minStartPan: 32,
    emptyPan: 80,
    fitPad: 96,
    fitPadSides: 2,
    minFitBoxW: 240,
    minFitBoxH: 160,
    maxFitZoom: 1.15,
    syncDebounceMs: 1200,
    zoomDecimals: 3
  },

  sprites: {
    whiteU: 4 / 256,
    whiteV: 4 / 128,
    portFill: { u0: 16 / 256, v0: 2 / 128, u1: 48 / 256, v1: 34 / 128, size: 24 },
    portRing: { u0: 52 / 256, v0: 2 / 128, u1: 84 / 256, v1: 34 / 128, size: 24 },
    portHover: { u0: 88 / 256, v0: 2 / 128, u1: 120 / 256, v1: 34 / 128, size: 24 },
    diamondPortFill: { u0: 16 / 256, v0: 66 / 128, u1: 48 / 256, v1: 98 / 128, size: 24 },
    diamondPortRing: { u0: 52 / 256, v0: 66 / 128, u1: 84 / 256, v1: 98 / 128, size: 24 },
    diamondPortHover: { u0: 88 / 256, v0: 66 / 128, u1: 120 / 256, v1: 98 / 128, size: 24 },
    smallPortFill: { u0: 16 / 256, v0: 36 / 128, u1: 40 / 256, v1: 60 / 128, size: 16 },
    smallPortRing: { u0: 52 / 256, v0: 36 / 128, u1: 76 / 256, v1: 60 / 128, size: 16 },
    smallPortHover: { u0: 88 / 256, v0: 36 / 128, u1: 112 / 256, v1: 60 / 128, size: 16 },
    chevronH: { u0: 188 / 256, v0: 2 / 128, u1: 204 / 256, v1: 18 / 128, size: 10 },
    chevronV: { u0: 204 / 256, v0: 2 / 128, u1: 220 / 256, v1: 18 / 128, size: 10 },
    grip: { u0: 124 / 256, v0: 2 / 128, u1: 144 / 256, v1: 24 / 128, w: 20, h: 22 },
    branchPlus: { u0: 120 / 256, v0: 34 / 128, u1: 148 / 256, v1: 62 / 128, size: 15 },
    editPen: { u0: 152 / 256, v0: 34 / 128, u1: 180 / 256, v1: 62 / 128, size: 15 },
    trashIcon: { u0: 184 / 256, v0: 34 / 128, u1: 212 / 256, v1: 62 / 128, size: 15 }
  },

  themes: {
    light: {
      isLight: true,
      bgObsidianRgb: [1.0, 1.0, 1.0],
      gridDotRgb: [0.875, 0.875, 0.865],
      bgObsidian: 0xffffffff,
      bgElevated: 0xfff4f4f4,
      borderSubtle: 0xffe2e2e2,
      borderStrong: 0xff8c8c8c,
      wireDefault: 0xff8c8c8c,
      wirePalette: [
        0xff96724f, 0xff73823d, 0xff38769c, 0xff48509e,
        0xff905873, 0xff3c7d64, 0xff705091, 0xff8e783c
      ],
      textPrimary: 0xff000000,
      textSecondary: 0xff333333,
      textMuted: 0xff666666,
      accentDanger: 0xff2c30c9,
      chapterFill: 0x05000000,
      marqueeFill: 0x12000000,
      ghostFill: 0x0b000000,
      ghostValidBorder: 0xd9000000,
      ghostInvalidBorder: 0xf22c30c9,
      collisionCellFill: 0x422e2edb,
      collisionCellBorder: 0x9e2c30c9,
      unreachCardBg: 0x8cf4f4f4,
      unreachBorder: 0x8cd9d9d9,
      unreachText: 0x8c737373,
      unreachWire: 0x8cc7c7c7
    },
    dark: {
      isLight: false,
      bgObsidianRgb: [0.0, 0.0, 0.0],
      gridDotRgb: [0.118, 0.118, 0.118],
      bgObsidian: 0xff000000,
      bgElevated: 0xff0c0c0c,
      borderSubtle: 0xff242424,
      borderStrong: 0xff6e6e6e,
      wireDefault: 0xff6e6e6e,
      wirePalette: [
        0xffae8c6b, 0xff8f9e5c, 0xff5792b5, 0xff646bb3,
        0xffa8748d, 0xff5c9c84, 0xff8c6ead, 0xffad985c
      ],
      textPrimary: 0xffffffff,
      textSecondary: 0xffc8c8c8,
      textMuted: 0xff949494,
      accentDanger: 0xff5252e0,
      chapterFill: 0x08808080,
      marqueeFill: 0x179e9e9e,
      ghostFill: 0x0effffff,
      ghostValidBorder: 0xd9ffffff,
      ghostInvalidBorder: 0xf25252eb,
      collisionCellFill: 0x473d3deb,
      collisionCellBorder: 0x9e5252eb,
      unreachCardBg: 0x8c0c0c0c,
      unreachBorder: 0x8c2e2e2e,
      unreachText: 0x8c808080,
      unreachWire: 0x8c383838
    }
  },

  cursors: {
    "scene-out-port": "crosshair",
    "chapter-out-port": "crosshair",
    "node-grip": "grab",
    "chapter-grip": "grab",
    "choice-grip": "grab",
    "wire-pin": "grab",
    wire: "grab",
    "node-title": "text",
    "choice-label": "text",
    "chapter-title": "pointer",
    "add-choice-btn": "pointer",
    "edit-scene-btn": "pointer",
    "delete-scene-btn": "pointer",
    "choice-trash-btn": "pointer",
    "branch-radio": "pointer",
    "node-card": "pointer"
  }
};
