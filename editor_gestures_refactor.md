# Plan refaktoryzacji `app/js/editor_*.js` (`editor_gestures_refactor.md`)

## 1. Co tam jest i jakie są problemy

1. **Pomieszanie silnika Canvas WebGL z DOM formularzy i siecią (`editor_canvas.js` 1908 LOC, `editor_gestures.js` 3047 LOC, `editor_inspector.js` 1252 LOC)**:
   - `editor_gestures.js` miesza fizykę/hit-test/drag kanwy z obsługą formularzy inspektora (`syncChoiceTargetToCanvas`, `.custom-select`, `[data-form-toggle]`, `.form-radio-option`, `Enter`/`change`/`blur`), ręczną mutacją `#form-new-book` (`L944–978`) i wstawianiem bloków scenariusza (`L670–770`).
   - `editor_canvas.js` miesza czysty model grafu (metryki węzłów, spatial grid, kolizje) z pływającymi inputami DOM (`#canvas-*-input`, `L1123–1308`), paletą szukania (`#studio-search-palette`, `L1636–1908`) i wywołaniami `sendServerAction` / `postServerActionSilent`.
   - `editor_inspector.js` miesza koordynację workspace (`#view-editor`), formularz `#editor-context-panel`, synchronizację `#modal-scene-text` i drzewo `#editor-history-panel` (`L1087–1250`).
2. **Duplikacja logiki między JS a Go**:
   - `syncChoiceTargetToCanvas` (`editor_gestures.js:20–125`) duplikuje `resolveCanvasSceneTargetId` (`editor_inspector.js:148`) i 48-liniowy BFS osiągalności z `book.FinalizeBook`.
   - `handleEditorCanvasClick` (`L670–770`) i `.new-book-mode-select` (`L944–978`) duplikują szablony z Go/`.tpl` oraz zostawiają 20 nadmiarowych kluczy w `jsI18nKeys` (`internal/i18n/dict.go:92–111`) i `trStd` w `core.js:102`.

---

## 2. Rozwiązania (4 czyste warstwy bez bloatu słowników stringów)

1. **Silnik Canvas (`canvas_model.js`, `canvas_webgl.js`, `canvas_webgl_scene.js`, `canvas_wires.js`, `canvas_hittest.js`) — zero `qs()`, zero sieci**:
   - `canvas_model.js`: stałe geometryczne (`EDITOR_GRID_SIZE`, `CARD`, `PAD`, `GAP`, `WIRE`, `SPATIAL`, `UV`, `FONT_STYLES`), stan grafu i kamery (`editorCanvasState`, `studioGraphState`, `selectedNodeIds`, `activeCanvasGesture`) oraz czysta matematyka modelu (`recomputeNodeMetrics`, `updateChapterGroupFrames`, spatial grid, `evaluateNodeDragGhost`, `moveNodePortInMemory`, `applyIncrementalNodeMove`, piny kabli).
   - `canvas_hittest.js`: `clientToStudioWorld`, `buildPointerContext(viewport, e, isTouch)`, `distToSegmentSq(px, py, segA, segB)`, `hitTestStudioCanvas(wx, wy)`, `isSameHoveredCanvasItem`.
2. **Kontroler wejścia kanwy (`canvas_input.js`) — tłumaczy pointer/wheel/touch/key na komendy modelu i emituje callbacki `canvasCallbacks.*`**:
   - Maszyna stanów gestów 120fps (`begin*Gesture(hit, ptrCtx)`, `DRAG_MOVE_HANDLERS`, `DRAG_FINISH_HANDLERS`) oraz zdarzenia `wheel`/`gesture*`/`touch*`/`mouse*`/`keydown` na `#timeline-scroll-viewport`.
   - Zamiast grzebać w formularzach DOM i wołać `sendServerAction`, woła `canvasCallbacks` (`onNodeSelect`, `onChapterSelect`, `onSelectionClear`, `onOpenSceneText`, `onDeleteScenes`, `onAddChoice`, `onRemoveChoice`, `onReorderChoice`, `onWireConnect`, `onChapterConnect`, `onCommitViewport`, `onCommitNodePositions`, `onCommitWirePins`, `onCreateNodeAt`).
3. **Nakładki kanwy (`canvas_overlays.js`)**:
   - Floating inputy (`#canvas-node-title-input`, `#canvas-choice-input`, `#canvas-chapter-input`), `#canvas-context-menu` oraz `#studio-search-palette` (`open/close/toggle/step/activateStudioSearchRow`).
4. **Widoki 1:1 z `.tpl` (`editor_view.js`, `editor_context_panel.js`, `editor_history_panel.js`, `modal_scene_text.js`)**:
   - `editor_view.js`: inicjalizacja `#view-editor` (`initNodeEditorCanvas`, ResizeObserver, `syncEditorViewportVisibility`), rejestracja `canvasCallbacks` do `POST /api/action` oraz deklaratywna obsługa kontrolek formularzy (`[data-form-toggle]`, `.form-radio-option`, `.custom-select` z `data-select-action`).
   - `editor_context_panel.js`: live-save i synchronizacja `#editor-context-panel` (`#editor-context-form`, `#editor-book-form`, `#editor-chapter-form`, `#editor-multi-panel`, `#editor-breakdown-panel`, `#editor-variables-panel`), `syncChoiceTargetToCanvas` (bez BFS, z `resolveCanvasSceneTargetId`) oraz `commitEditorField`.
   - `editor_history_panel.js`: `#editor-history-panel` (`updateHistoryScrollCue`, `clearHistoryDragHighlights`, `bindHistoryGraphInteractions`).
   - `modal_scene_text.js`: `#modal-scene-text` (`syncCanvasNodeFromSceneTextModal`, `[data-snippet]` wstawiane przez `insertSnippetIntoSceneTextarea`, test vars, `applyPendingSceneTextSearchHighlight`).

---

## 3. Zmiany plik po pliku, metoda po metodzie

### 1. Backend Go, `i18n` i `.tpl` (likwidacja duplikacji w JS)
- **`internal/book/screenplay.go` + `internal/views/view_editor_scene_text.go` + `internal/actions/actions_editor_scene.go`**:
  - Wspólny `book.ScreenplayBlockSnippet(standard, kind, charName, lang)` zasila `insertScreenplayBlockAction` oraz `buildScreenplayFormatInsertPills` (`data-snippet`, `data-select-target`, `data-double-newline` w `modal_scene_text.tpl:382,395`).
- **`internal/actions/actions_modal_new_book.go` + `internal/actions/actions_dispatch.go`**:
  - Wspólny `syncNewBookModalForm(s, r)` i akcja `SetNewBookModalMode` (`Target: targetModalNewBook`).
- **`internal/i18n/dict.go:92–111` + `app/js/core.js:102–106`**:
  - Usunięcie 20 kluczy (`new_*`, `sp_ins_*`) z `jsI18nKeys` oraz usunięcie `trStd`.
- **Szablony `.tpl` (`data-select-action`)**:
  - `modal_scene_text.tpl:134` (`UpdateSceneTextTestVar`), `editor_context_panel.tpl:440,514,534` (`SetChapterStartScene`, `SetBookStartChapter`, `SetStartScene`), `modal_new_book.tpl:36` (`SetNewBookModalMode`), `modal_export.tpl:57` (`SetExportModalStandard`, `data-select-silent: "true"`).

### 2. Warstwa Silnika Canvas (`canvas_*.js`)
- **`app/js/canvas_model.js`** (~360 LOC) — stałe (`EDITOR_GRID_SIZE`..`FONT_STYLES`, `CARD`, `PAD`, `GAP`, `WIRE`, `SPATIAL`, `UV`), stan grafu i kamery (`editorCanvasState`, `studioGraphState`, `selectedNodeIds`, `activeCanvasGesture`), metryki węzłów/rozdziałów, parser grafu i geometria pinów kabli.
- **`app/js/canvas_layout.js`** (~300 LOC) — spatial grid, unikanie kolizji, delty przeciągania rozdziałów/węzłów (`evaluateNodeDragGhost`), ramki rozdziałów i `applyIncrementalNodeMove`.
- **`app/js/canvas_hittest.js`** (~205 LOC) — `HIT_CURSOR_MAP`, `clientToStudioWorld`, `buildPointerContext(viewport, e, isTouch)`, `distToSegmentSq(px, py, segA, segB)`, `hitTestStudioCanvas(wx, wy)`, `isSameHoveredCanvasItem`.
- **`app/js/canvas_drag.js`** (~475 LOC) — maszyna stanów gestów 120fps (`begin*Gesture`, `DRAG_MOVE_HANDLERS`, `updateCanvasDragMove`, `DRAG_FINISH_HANDLERS`, `finishActiveCanvasGesture`).
- **`app/js/canvas_input.js`** (~380 LOC) — kontroler wejścia kanwy (`handleEditorCanvasClick`, `handleCanvasTouchEndOrCancel`, `bindEditorCanvasEvents`).
- **`app/js/canvas_overlays.js`** (~325 LOC) — floating inputy (`#canvas-*-input`, `applyFloatingInputBoxStyle(inp, rect, fontPx, letterSpacing)`), `#canvas-context-menu` oraz `#studio-search-palette`.

### 3. Warstwa Widoków 1:1 (`editor_view.js`, `editor_context_panel.js`, `editor_history_panel.js`, `modal_scene_text.js`)
- **`app/js/editor_view.js`** (~390 LOC) — kamera, inicjalizacja `#view-editor` (`initNodeEditorCanvas`, ResizeObserver, `syncEditorViewportVisibility`), zapisy stanu kanwy (`SaveEditorState`, `CreateSceneAt`) i deklaratywne kontrolki formularzy (`handleEditorFormClick`).
- **`app/js/editor_context_panel.js`** (~480 LOC) — stan i synchronizacja `#editor-context-panel`, `syncChoiceTargetToCanvas` (bez BFS, z `resolveCanvasSceneTargetId`), wybór węzła/rozdziału, live-save oraz `commitEditorField` + `bindEditorFormEvents`.
- **`app/js/editor_history_panel.js`** (~165 LOC) — `#editor-history-panel` (`updateHistoryScrollCue`, `clearHistoryDragHighlights`, `bindHistoryGraphInteractions`).
- **`app/js/modal_scene_text.js`** (~345 LOC) — `#modal-scene-text` (`syncSceneTitleAcrossForms`, `syncSceneChoiceLabelAcrossForms`, `resolveCanvasSceneTargetId`, `resolveCurrentClientSceneId`, `syncSceneIdentityAndMetaFromHeaders`, `syncCanvasNodeFromSceneTextModal`, `insertSnippetIntoSceneTextarea`, `applyPendingSceneTextSearchHighlight`).

### 4. `internal/tpl/bundle.go` (`JSModuleFiles`)
- Kolejność modułów:
  `core.js`, `audio.js`, `player.js`, `msdf_metrics.js`, `canvas_model.js`, `canvas_layout.js`, `editor_webgl.js`, `editor_webgl_scene.js`, `editor_wires.js`, `canvas_hittest.js`, `canvas_overlays.js`, `canvas_drag.js`, `canvas_input.js`, `editor_view.js`, `editor_context_panel.js`, `editor_history_panel.js`, `modal_scene_text.js`, `editor_voice.js`, `editor_autocomplete.js`, `editor_perf.js`.
