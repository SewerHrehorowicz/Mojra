# Plan naprawczy modułu `node_canvas` zgodnie z `@ARCH.md` i `@AGENTS.md`

## 1. Co tam jest i jakie są problemy

Moduł `node_canvas` składa się z publicznego kontrolera FDP `app/js/controllers/node_canvas_controller.js` (`NodeCanvasController`) oraz czterech wewnętrznych modułów pomocniczych w `app/js/node_canvas/`:
- `node_canvas_style.js` (`NodeCanvasStyle` — 199 LOC)
- `node_canvas_layout.js` (`NodeCanvasLayout` — 863 LOC)
- `node_canvas_helpers.js` (`NodeCanvasHelpers` — 481 LOC)
- `node_canvas_renderer.js` (`NodeCanvasRenderer` — 1134 LOC)

Audyt wykazał następujące naruszenia separacji odpowiedzialności (`@ARCH.md` §1, §5) oraz reguł architektonicznych (`@AGENTS.md` §0, §5):

1. **`NodeCanvasLayout` miesza stan grafu książki ze stanem interakcji UI, WebGL i zapytaniami DOM (`app/js/node_canvas/node_canvas_layout.js`)**:
   - **L55–68 (`studioGraphState`)**: Obok czystych struktur grafu książki (`chapters`, `chapterByKey`, `nodes`, `nodeById`, `posById`, `wires`, `wirePins`) przechowuje stan sesji UI (`hoveredItem`, `editingChapter`, `editingNodeTitle`, `editingChoice`), cache glifów i buforów WebGL (`shapedTextCache`, `viewW`, `viewH`, `canvasDpr`, `staticBatchDirty`, `overlayBatchDirty`) oraz kopię stanu kamery (`hasSavedView`, `savedPanX`, `savedPanY`, `savedZoom`).
   - **L71–96, L105–112 (`inRect`, `choiceRowY`, `toSpatialCell`, `snapToEditorGrid`, `snapToWireGrid`, `snapToWirePinGrid`, `getNodeFooterBtnX`)**: Czyste funkcje matematyczne/geometryczne siedzą w `NodeCanvasLayout`, przez co `NodeCanvasHelpers` i `NodeCanvasRenderer` muszą ciągle odwoływać się do `NodeCanvasLayout` po elementarne operacje geometryczne.
   - **L98–103 (`isHistoryPreview`)**: Moduł layoutu odpytuje DOM (`document.querySelector("#studio-view")`) oraz stan `NodeCanvasController.data.isHistoryPreview`, mimo że `isHistoryPreview` jest natywnym polem kontraktu `NodeCanvasController.data` hydrowanym przez FDP (`@ARCH.md` §3.4, §5).

2. **Zdublowany stan kamery i `layoutHash` między `NodeCanvasController.data` a `NodeCanvasLayout` (`node_canvas_controller.js` + `node_canvas_layout.js`)**:
   - `NodeCanvasController.data` (L18–22) posiada już pola `layoutHash`, `savedPanX`, `savedPanY`, `savedZoom`, `hasView` hydrowane bezpośrednio z `nodeCanvasScript` przez FDP.
   - `NodeCanvasController.buildCameraMeta` (L1176–1185) buduje przejściowy obiekt tylko po to, by `NodeCanvasLayout.applyCanvasLayout` (L369–379) przepisał te same pola do `NodeCanvasLayout.layoutHash` i `studioGraphState.savedPanX/savedPanY/savedZoom/hasSavedView`, skąd `NodeCanvasController.resetToFirstNode` (L284–288) czyta je z powrotem.

3. **Potrójna, niespójna kalkulacja geometrii nagłówka węzła (`branchRadioX/Y`, szerokość badge'a i tytułu)**:
   - **`NodeCanvasLayout.recomputeNodeMetrics` (L184–185)**: Wylicza `node.branchRadioX` ze sztywnych stałych (`node.isStart ? 54 : node.isEntry ? 64 : 0`).
   - **`NodeCanvasRenderer.compileStaticSceneQuads` (L867–885)**: Podczas renderowania klatki WebGL liczy szerokość badge'a z MSDF (`Math.ceil(nd._shapedBadge.totalW) + 10`) i **nadpisuje** `nd.branchRadioX` oraz `nd.branchRadioY` inną wartością (`nd.x + nd.w - (badgeW > 0 ? badgeW + 8 + 14 : 16)`).
   - **`NodeCanvasHelpers.positionFloatingNodeTitleInput` (L452–455)**: Trzeci raz liczy rezerwę miejsca na badge i radio z jeszcze inną stałą (`(nd.isStart || nd.isEntry ? 50 : 8) + radioReserve`).

4. **Zdublowane wyliczanie ramki rozdziału (Chapter AABB) oraz wyszukiwanie węzłów rozdziału**:
   - **`NodeCanvasLayout.computeChapterFrameRect` (L282–293)** liczy AABB rozdziału z uwzględnieniem `card.minChapterH`, `pad` i `laneSpread`.
   - **`NodeCanvasHelpers.resolveValidChapterDragDelta` (L249–254)** i **`NodeCanvasHelpers.evaluateNodeDragGhost` (L306–319)** ręcznie powtarzają pętle `minX/minY/maxX/maxY` oraz odejmowanie/dodawanie `pad.x`, `pad.top`, `pad.bottom` (pomijając `laneSpread`, przez co walidacja kolizji podczas przeciągania rozdziału używa innej obwiedni niż właściwy layout).
   - Wyszukiwanie węzłów wejściowych rozdziału (`nd.chapterLower === ch.nameLower || nd.chapterLower === ch.refLower`) jest powtórzone 4 razy: `Layout.recomputeChapterInputPortPosition` (L220–224), `Layout.buildWireCatalogFromMemory` (L812–816), `Helpers.resolveNonOverlappingNodeCoords` (L211) i `Controller.beginSharedHitDragGesture` (L590–592).

5. **Potrójna implementacja rzutowania punktu na odcinek (Point-to-Segment Projection)**:
   - Ten sam wzór matematyczny (`dx = bx - ax`, `dy = by - ay`, `lenSq = dx*dx + dy*dy`, `t = clamp(((px - ax)*dx + (py - ay)*dy) / lenSq, 0, 1)`, `dSq = ...`) jest napisany niezależnie w 3 miejscach:
     1. `NodeCanvasHelpers.distToSegmentSq` (`node_canvas_helpers.js:71–76`),
     2. `NodeCanvasHelpers.computePointArcLengthAlongPolyline` (`node_canvas_helpers.js:399–411`),
     3. `NodeCanvasRenderer.findClosestPolylineSegment` (`node_canvas_renderer.js:670–682`),
   - Ponadto geometria uchwytu pina na segmencie (`computeWirePinSegmentBar`, `node_canvas_renderer.js:684–704`) siedzi w rendererze WebGL, choć jest używana przez `NodeCanvasHelpers.hitTest` (`node_canvas_helpers.js:120`).

6. **Funkcje pass-through w `NodeCanvasLayout` (naruszenie `@AGENTS.md` §0 i `@ARCH.md` §5.6)**:
   - `linecastVertical` (L551–553), `linecastHorizontal` (L555–557), `resolveClearX` (L586–588) i `resolveClearY` (L590–592) to czyste funkcje pass-through delegujące 1:1 do `linecastOrthogonal(true/false, ...)` oraz `resolveClearCoord(true/false, ...)`.

7. **Potrójna duplikacja cyklu życia pływających inputów (`open / onInput / commit / hide / onKeydown`) w `NodeCanvasController`**:
   - W `node_canvas_controller.js` (L353–525) niemal identyczny zestaw 5 metod jest powtórzony 3 razy dla `#canvas-chapter-input`, `#canvas-node-title-input` i `#canvas-choice-input` (173 linie zduplikowanego kodu sterowania inputem).

---

## 2. Rozwiązania docelowe

1. **Czysty podział odpowiedzialności między 4 modułami wewnętrznymi a `NodeCanvasController`**:
   - **`NodeCanvasController` (`app/js/controllers/node_canvas_controller.js`)**: Jedyny właściciel stanu FDP (`this.data`, w tym `isHistoryPreview`, `savedPanX`, `savedPanY`, `savedZoom`, `hasView`, `layoutHash`, `panX`, `panY`, `zoom`, `dirty`), bindowań DOM (`bindings()`) oraz wysyłania akcji `Dispatcher.post(...)`.
   - **`NodeCanvasStyle` (`app/js/node_canvas/node_canvas_style.js`)**: Wyłącznie deklaratywne stałe wizualne, wymiary kart/portów (`badgePadX`, `badgeH`, `radioSize`, `radioGap`), siatki, UV, typografia, palety motywów i mapa kursorów. Zero stanu i zero logiki.
   - **`NodeCanvasLayout` (`app/js/node_canvas/node_canvas_layout.js`)**: Wyłącznie model i geometria grafu bieżącej książki (`studioGraphState`: `chapters`, `chapterByKey`, `nodes`, `nodeById`, `posById`, `wires`, `wirePins`), indeks przestrzenny (`nodeSpatialGrid`), spójne metryki węzłów/rozdziałów (`recomputeNodeMetrics`, `computeChapterFrameRect`, `resolveChapterEntryNode`) oraz ortogonalny routing połączeń (`route`, `routeSingleWireBasePts`, `linecastOrthogonal`, `resolveClearCoord`). Zero zapytań DOM, zero pass-through, zero stanu WebGL.
   - **`NodeCanvasHelpers` (`app/js/node_canvas/node_canvas_helpers.js`)**: Geometryczne i interakcyjne funkcje pomocnicze: prymitywy (`inRect`, `snapTo*`, `projectPointToSegment`, `findClosestPolylineSegment`, `computeWirePinSegmentBar`), `hitTest`, detekcja kolizji/duchów (`evaluateNodeDragGhost`, `resolveValidChapterDragDelta`, `resolveNonOverlappingNodeCoords` oparte na wspólnym `NodeCanvasLayout.computeChapterFrameRect`), mutacje in-memory oraz ujednolicona obsługa pływających inputów.
   - **`NodeCanvasRenderer` (`app/js/node_canvas/node_canvas_renderer.js`)**: Wyłącznie WebGL, shadery, atlasy tekstur, cache glifów MSDF (`shapedTextCache` przeniesiony do `studioGL`), rozmiary viewportu (`viewW`, `viewH`, `canvasDpr` w `studioGL`), flagi brudnych buforów VBO (`invalidateStatic()`, `invalidateOverlay()`) oraz instancjonowane rysowanie 120fps.

2. **Eliminacja duplikacji i funkcji pass-through**:
   - Usunięcie 4 funkcji pass-through (`linecastVertical`, `linecastHorizontal`, `resolveClearX`, `resolveClearY`) i bezpośrednie wywołanie `linecastOrthogonal(true/false, ...)` oraz `resolveClearCoord(true/false, ...)`.
   - Zastąpienie 3 pętli rzutowania na odcinek jedną bezalokacyjną metodą `NodeCanvasHelpers.projectPointToSegment(px, py, ax, ay, bx, by, out)` i przeniesienie `computeWirePinSegmentBar` oraz `findClosestPolylineSegment` do `NodeCanvasHelpers` (z zachowaniem wywołania w `NodeCanvasRenderer`).
   - Ujednolicenie geometrii nagłówka węzła (`badgeW`, `branchRadioX`, `branchRadioY`, `titleMaxW`) w jednym miejscu (`NodeCanvasLayout.recomputeNodeMetrics`), z którego korzystają zarówno `NodeCanvasRenderer`, `NodeCanvasHelpers.hitTest`, jak i `NodeCanvasHelpers.positionFloatingNodeTitleInput`.
   - Użycie `NodeCanvasLayout.computeChapterFrameRect` we wszystkich testach kolizji rozdziałów (`resolveValidChapterDragDelta`, `evaluateNodeDragGhost`, `resolveNonOverlappingNodeCoords`).
   - Zastąpienie 3 zduplikowanych zestawów metod pływających inputów w `NodeCanvasController` jednym wspólnym mechanizmem opartym o specyfikację pola (`chapter`, `nodeTitle`, `choice`).

3. **Wydajność (P0 — 120fps na ścieżce gorącej)**:
   - `projectPointToSegment` używa współdzielonego obiektu zwrotnego `_segProj` (zero alokacji na ścieżce `mousemove` / `hitTest`).
   - `computeChapterFrameRect(nodes, overrideRects)` liczy obwiednię rozdziału w jednym przebiegu $O(N)$ bez alokacji tymczasowych tablic węzłów podczas przeciągania.
   - Zachowanie 32-bitowego pakowania kluczy siatki przestrzennej (`(gy & 0xffff) << 16 | (gx & 0xffff)`) oraz pierścienia 3 buforów VBO (`STREAM_DRAW`).

---

## 3. Szczegółowy plan zmian plik po pliku, metoda po metodzie

### 3.1. `app/js/node_canvas/node_canvas_style.js` (`NodeCanvasStyle`)
1. **`NodeCanvasStyle.card` (L13–23)**:
   - Dodać jawne stałe wymiarów nagłówka karty używane wspólnie przez layout, hit-test, floating input i renderer:
     - `gripW: 24`, `titleLeft: 28`, `badgeH: 17`, `badgePadX: 10`, `StartBadgeW: 48`, `entryBadgeW: 54`, `radioSize: 8`, `radioReserveW: 16`, `radioYOffset: 13`, `laneSpreadBase: 10`, `laneSpreadStep: 14`.

### 3.2. `app/js/node_canvas/node_canvas_layout.js` (`NodeCanvasLayout`)
1. **`studioGraphState` (L47–69)**:
   - Usunąć zduplikowane pola kamery (`hasSavedView`, `savedPanX`, `savedPanY`, `savedZoom`) — ich jedynym właścicielem jest `NodeCanvasController.data`.
   - Przenieść `shapedTextCache`, `viewW`, `viewH`, `canvasDpr` do `NodeCanvasRenderer.studioGL` (z zachowaniem flag `staticBatchDirty` i `overlayBatchDirty` aktualizowanych przez helper `markDirtyBatch(staticDirty, overlayDirty)`).
2. **Współdzielone prymitywy geometryczne (`inRect`, `choiceRowY`, `toSpatialCell`, `snapToEditorGrid`, `snapToWireGrid`, `snapToWirePinGrid`, `getNodeFooterBtnX` — L71–112)**:
   - Usunąć zapytanie DOM z `isHistoryPreview()` — czytać bezpośrednio `Boolean(NodeCanvasController.data.isHistoryPreview)`.
   - Utrzymać metody geometryczne siatki i kart (`inRect`, `choiceRowY`, `toSpatialCell`, `snapToEditorGrid`, `snapToWireGrid`, `snapToWirePinGrid`, `getNodeFooterBtnX`) jako czyste funkcje bez dostępu do DOM (wymagane m.in. przez testy runtime `NodeCanvasLayout.getNodeFooterBtnX`).
3. **Jednolite wyliczanie metryk nagłówka węzła w `recomputeNodeMetrics(node)` (L171–212)**:
   - Wyliczać w jednym miejscu:
     - `node.badgeW`: `node.isStart ? card.StartBadgeW : node.isEntry ? card.entryBadgeW : 0` (aktualizowane dokładnie przez MSDF przy kształtowaniu tekstu, jeśli dostępne),
     - `node.branchRadioX` i `node.branchRadioY`: `hasBranchRadio ? node.x + node.w - (node.badgeW > 0 ? node.badgeW + card.pad + 14 : card.radioReserveW) : 0`, `node.y + card.radioYOffset`,
     - `node.titleMaxW`: `node.w - 36 - (node.badgeW > 0 ? node.badgeW + 6 : 0) - (hasBranchRadio ? card.radioReserveW : 0)`.
   - Usunąć nadpisywanie `nd.branchRadioX/Y` z `NodeCanvasRenderer.compileStaticSceneQuads` (L882–883) oraz ręczne liczenie `badgeReserve`/`radioReserve` z `NodeCanvasHelpers.positionFloatingNodeTitleInput` (L453–455).
4. **Dedykowany selektor węzła wejściowego rozdziału `resolveChapterEntryNode(ch)` (zastępuje L220–224 i L812–816)**:
   - Nowa metoda w `NodeCanvasLayout`:
     - Sprawdza `ch.startSceneId` w `studioGraphState.nodeById` (weryfikując przynależność do `ch.nameLower` / `ch.refLower`),
     - W przeciwnym razie wybiera z `ch.nodes` węzeł z `nd.isEntry` lub pierwszy węzeł rozdziału (`ch.nodes[0]`).
   - Użyć w `recomputeChapterInputPortPosition` (L220) oraz `buildWireCatalogFromMemory` (L812).
5. **Ujednolicenie `computeChapterFrameRect(matching, draggedRectsByNode)` (L282–293)**:
   - Rozszerzyć `computeChapterFrameRect`, aby opcjonalnie przyjmował przesunięcia `(dx, dy)` lub mapę/tablicę prostokątów przeciąganych węzłów, licząc `minX, minY, maxX, maxY` (z `laneSpread` i `card.minChapterH`) oraz zwracając `{ x, y, w, h, left, top, right, bottom }`.
   - Użyć tej jednej metody w `updateChapterGroupFrames` (L305, L348), `resolveValidChapterDragDelta` (`node_canvas_helpers.js:249`) oraz `evaluateNodeDragGhost` (`node_canvas_helpers.js:306`).
6. **Uproszczenie `applyCanvasLayout(rawLayout, resetCanvas)` (L369–405)**:
   - Usunąć parametr `cameraMeta` oraz przepisywanie `savedPanX/Y/Zoom/hasView` do `studioGraphState`.
   - Przyjmować `(rawLayout, resetCanvas)` — gdy `resetCanvas` jest prawdziwe, czyścić `NodeCanvasController.data.dirty = false` i parsować pozycje (`pos`) oraz piny (`pin` / `wire_pins`).
7. **Usunięcie 4 funkcji pass-through (L551–557 i L586–592)**:
   - Usunąć `linecastVertical`, `linecastHorizontal`, `resolveClearX`, `resolveClearY`.
   - W `buildWireWaypoints` (L594–677) wywoływać bezpośrednio `this.linecastOrthogonal(true, ...)` / `this.linecastOrthogonal(false, ...)` oraz `this.resolveClearCoord(true, ...)` / `this.resolveClearCoord(false, ...)`.

### 3.3. `app/js/node_canvas/node_canvas_helpers.js` (`NodeCanvasHelpers`)
1. **Wspólna projekcja punktu na odcinek i geometria pinów (`projectPointToSegment`, `distToSegmentSq`, `findClosestPolylineSegment`, `computePointArcLengthAlongPolyline`, `computeWirePinSegmentBar`)**:
   - Dodać bezalokacyjny obiekt roboczy `_segProj: { t: 0, distSq: 0, lenSq: 0 }` oraz metodę `projectPointToSegment(px, py, ax, ay, bx, by)`.
   - Przebudować `distToSegmentSq` (L71–76) i `computePointArcLengthAlongPolyline` (L399–411), aby korzystały z `projectPointToSegment`.
   - Przenieść logikę `findClosestPolylineSegment` i `computeWirePinSegmentBar` do `NodeCanvasHelpers` (z użyciem `projectPointToSegment`), aby `hitTest` (L120) nie musiał wywoływać `NodeCanvasRenderer`. W `NodeCanvasRenderer.computeWirePinSegmentBar` zostawić delegację lub bezpośrednie odwołanie zgodne z testem `server_js_runtime_test.go:1366`.
2. **Deduplikacja obwiedni rozdziału w `resolveNonOverlappingNodeCoords`, `resolveValidChapterDragDelta` i `evaluateNodeDragGhost` (L206–341)**:
   - W `resolveNonOverlappingNodeCoords` (L211): pobierać rozdział w $O(1)$ przez `NodeCanvasLayout.studioGraphState.chapterByKey.get(chKey)` zamiast `chapters.find(...)`.
   - W `resolveValidChapterDragDelta` (L249–257): wyliczyć bazową obwiednię raz przez `NodeCanvasLayout.computeChapterFrameRect(ch.nodes)` i w `isDeltaValid(dx, dy)` przesuwać ją o `(dx, dy)`.
   - W `evaluateNodeDragGhost` (L303–337): wyliczać obwiednię rozdziału z przeciąganymi węzłami przez `NodeCanvasLayout.computeChapterFrameRect(ch.nodes, rects)` zamiast duplikować dwie pętle po `ch.nodes` i `rects`.
3. **Uproszczenie `positionFloatingNodeTitleInput` (L448–456)**:
   - Użyć `nd.titleMaxW` wyliczonego w `NodeCanvasLayout.recomputeNodeMetrics(nd)`:
     `this.applyBoxStyle(inp, { x: nd.x + card.titleLeft, y: nd.y + 6.715, w: nd.titleMaxW, h: 22 }, 13);`

### 3.4. `app/js/node_canvas/node_canvas_renderer.js` (`NodeCanvasRenderer`)
1. **Własność stanu WebGL i cache'u glifów (`studioGL` — L158–184)**:
   - Przenieść `shapedTextCache: new Map()`, `viewW: 1200`, `viewH: 700`, `canvasDpr: 2` z `NodeCanvasLayout.studioGraphState` do `NodeCanvasRenderer.studioGL`.
   - W `shapeTextGlyphs` (L283) i `render` (L804, L1024–1026, L1050) korzystać z `this.studioGL.shapedTextCache`, `this.studioGL.viewW`, `this.studioGL.viewH`, `this.studioGL.canvasDpr`.
2. **Brak mutacji metryk layoutu podczas renderowania (`compileStaticSceneQuads` — L867–896)**:
   - Nie nadpisywać `nd.branchRadioX` i `nd.branchRadioY` wewnątrz pętli renderującej WebGL; korzystać z gotowych współrzędnych `nd.branchRadioX`, `nd.branchRadioY`, `nd.titleMaxW` wyliczonych przez `NodeCanvasLayout.recomputeNodeMetrics`.

### 3.5. `app/js/controllers/node_canvas_controller.js` (`NodeCanvasController`)
1. **Usunięcie duplikacji stanu kamery i `buildCameraMeta` (L280–304, L1176–1275)**:
   - Usunąć metodę `buildCameraMeta`.
   - W `resetToFirstNode(forceCenter, startSceneId)` (L284–290) czytać `this.data.hasView`, `this.data.savedZoom`, `this.data.savedPanX`, `this.data.savedPanY` bezpośrednio z `this.data` (hydrowanego przez FDP z `nodeCanvasScript`).
   - W `initCanvas` (L1209–1211) i `onHydrate` (L1258–1262) porównywać `this.data.layoutHash !== NodeCanvasLayout.layoutHash`, ustawiać `NodeCanvasLayout.layoutHash = this.data.layoutHash` i wywoływać `NodeCanvasLayout.applyCanvasLayout(this.data.canvasLayout, needReset)`.
2. **Konsolidacja obsługi 3 pływających inputów (`#canvas-chapter-input`, `#canvas-node-title-input`, `#canvas-choice-input` — L353–525)**:
   - Wydzielić wspólną metodę `hideAllFloatingInputs(shouldCommit)` (zastępującą 4-krotne powtarzanie 3 linii `hideFloatingChapterInput(true); hideFloatingNodeTitleInput(true); hideFloatingChoiceInput(true);` w `openFloating*` i `onMousedown`) oraz wspólny handler klawiszy `handleFloatingInputKeydown(e, hideFn)` dla `Enter` / `Escape`.

---

## 4. Weryfikacja i testy po wdrożeniu

1. **Zgodność z istniejącymi bramkami testowymi**:
   - Wszystkie referencje używane w `internal/server/server_js_runtime_test.go` i `internal/server/server_chaos_fuzz_test.go` (`NodeCanvasLayout.studioGraphState.nodes`, `nodeById`, `wires`, `NodeCanvasLayout.getNodeFooterBtnX`, `NodeCanvasHelpers.selectedNodeIds`, `NodeCanvasHelpers.hitTest`, `NodeCanvasHelpers.activeCanvasGesture`, `NodeCanvasHelpers.evaluateNodeDragGhost`, `NodeCanvasRenderer.computeWirePinSegmentBar`, `NodeCanvasRenderer.studioGL.staticInstCount`) pozostają w 100% kompatybilne.
2. **Kroki weryfikacji**:
   - Uruchomić pełny zestaw testów: `go test ./...` (w tym `TestClientServerActionAndOverlayContracts`, `TestClientJSRuntimeAndInteractiveResponsiveness`, testy architektury JS/FDP).
   - Zbudować aplikację na macOS: `./build.sh --macos`.
