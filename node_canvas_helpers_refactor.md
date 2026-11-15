# Plan podziału i uproszczenia `node_canvas_helpers.js` zgodnie z `@ARCH.md` i `@AGENTS.md`

## 1. Co tam jest i jakie są problemy

1. **Brak semantycznych typów geometrycznych — „zupa parametrów” skalarnych (`(px, py, x, y, w, h)`, `(px, py, ax, ay, bx, by)`, 14 argumentów w `buildWireWaypoints`)**:
   - Zamiast operować na strukturach geometrycznych (`Vec2`, `Rect`, `OrthogonalRay`, `Wire`), funkcje przyjmują od 6 do 14 luźnych liczb (`inRect(px, py, x, y, w, h)`, `projectPointToSegment(px, py, ax, ay, bx, by)`, `testObstacleHit(b, isVert, fixedCoord, minSpan, maxSpan, p, ignore1, ignore2)`, `linecastOrthogonal` — 8 parametrów, `resolveClearCoord` — 9 parametrów, `buildWireWaypoints` — 14 parametrów).
   - Nawet tam, gdzie punkty już istnieją jako obiekty `{ x, y }` (`w.pts[j]`, `w.pts[j + 1]`), są rozpakowywane do 6 skalarów `projectPointToSegment(wx, wy, w.pts[j].x, w.pts[j].y, w.pts[j + 1].x, w.pts[j + 1].y)`.
   - Test kolizji dwóch prostokątów z marginesem (`a.left < b.right + gap && a.right + gap > b.left && a.top < b.bottom + gap && a.bottom + gap > b.top`) jest pisany ręcznie 5 razy (`L1089`, `L1098`, `L1127`, `L1157`, `L1178`) zamiast jednej operacji `rectsOverlap(rectA, rectB, gap)`.
2. **Monolityczny plik `app/js/node_canvas/node_canvas_helpers.js` (1332 LOC > limit 600 LOC z `ARCH_MONOLITH_AND_NESTING`)**:
   - Jeden obiekt `NodeCanvasHelpers` miesza 4 niezależne podsystemy: geometrię/siatkę przestrzenną, model grafu/parser, ortogonalny router połączeń oraz interakcję/hit-testing/inputy DOM.
3. **Przerośnięty i nieczytelny `hitTest(wx, wy)` (`L949–1054`, 106 LOC)**:
   - Jedna metoda miesza 6 warstw Z-indexu, wielokrotnie przelicza globalne `nd.x + ...` i `nd.y + ...` zamiast użyć współrzędnych lokalnych karty (`localX = point.x - node.x`, `localY = point.y - node.y`) i opiera się na magicznych liczbach (`20`, `26`, `28`, `49`, `64`, `100`).
4. **Powielona praca i niespójności w routerze połączeń (`L604–929`)**:
   - W `routeSingleWireBasePts` (`L791–826`) 4 gałęzie `if/else` powtarzają wywołanie `simplifyOrthogonalPoints(buildWireWaypoints(...))` + `buildPinnedWireWaypoints`, a następnie `applyParallelOffsetsAndBundles` (`L911–919`) klonuje `w.basePts` do `w.pts` i drugi raz liczy `updateWireBounds`.
   - W `linecastOrthogonal` (`L604–637`) ścieżka ze `SpatialGrid` szuka **najbliższej** przeszkody (`bestDist`) i priorytetyzuje `hitKind === 2`, a pętla fallbackowa zwraca **pierwszą** przeszkodę z tablicy.
5. **Zduplikowane pętle i alokacje stringów w `evaluateNodeDragGhost` (`L1137–1196`)**:
   - Dwa razy powtarza podwójną pętlę po komórkach siatki (`L1161–1165` i `L1183–1188`) i alokuje stringi `cx + "," + cy` w `seenCellKeys` podczas ruchu myszą.
6. **Pomieszanie odpowiedzialności i duplikacja w `updateChapterGroupFrames` (`L390–452`), `applyBookGraph` (`L495–567`), `recomputeChapterPortPositions` (`L305–357`) i `buildWireCatalogFromMemory` (`L844–909`)**:
   - `updateChapterGroupFrames` skleja aktualizację inkrementalną podczas dragu (`if (onlyChaptersSet)`) z pełną przebudową po parsowaniu.
   - `updateChapterGroupFrames` i `applyBookGraph` duplikują konstruktor obiektu rozdziału.
   - `recomputeChapterPortPositions` powtarza 3 razy pętlę rozsuwania portów wzdłuż krawędzi (`bottomPorts`, `leftPorts`, `rightPorts`).
7. **Martwy kod i pomijanie `app/js/node_canvas/` w bundlerze**:
   - Nieużywana funkcja pass-through `distToSegmentSq` (`L138–140`), nieużywany parametr `preferredOutX` (`L666`) oraz martwe pola `chapterScope`, `targetGroupKey`, `bridges` (`L835–840`).
   - Potrójna duplikacja `positionFloating*Input` (`L1295–1322`).
   - `engine/bundle.go` pomija podkatalog `app/js/node_canvas/*.js`.

---

## 2. Rozwiązania: Semantyczne Struktury Danych i Podział na 4 Moduły

### 2.1. Kanoniczne struktury geometryczne i domenowe (max 1–3 parametry na metodę)

1. **`Vec2` (`{ x, y }`)**:
   - Punkt lub wektor 2D w przestrzeni świata/widoku (`point`, `delta`, wierzchołki `wire.pts[i]`, pozycje portów `port`).
   - Eliminuje pary argumentów `(px, py)`, `(ax, ay, bx, by)` na rzecz `(point, segStart, segEnd)`.
2. **`Rect` (`{ x, y, w, h, left, top, right, bottom }`)**:
   - Jednolity prostokąt obwiedni używany przez `Node`, `Chapter`, `dragRect` oraz `FloatingInput`.
   - Wspólne operacje:
     - `updateRect(rect)` — 1 parametr; przelicza `left = x`, `top = y`, `right = x + w`, `bottom = y + h` i zwraca `rect`,
     - `copyRect(targetRect, sourceRect)` — 2 parametry; kopiuje `x, y, w, h` z `sourceRect` do `targetRect` i wywołuje `updateRect(targetRect)`,
     - `inRect(point, rect)` — 2 parametry zamiast 6,
     - `rectsOverlap(rectA, rectB, gap)` — 3 parametry zastępujące 5 ręcznych warunków AABB,
     - `distSq(pointA, pointB)` — kwadrat odległości między dwoma `Vec2`.
3. **`OrthogonalRay` (`{ isVert, coord, spanStart, spanEnd, dir, pad }`)**:
   - Semantyczny promień ortogonalny używany przez router podczas sprawdzania kolizji z węzłami/rozdziałami.
   - `NodeCanvasRouter` trzyma referencję `this.activeWire` ustawianą na czas routingu pojedynczego połączenia (`fromEntity`, `toEntity`, `obstacles`), dzięki czemu `linecast(ray)`, `resolveClearCoord(ray)` i `testObstacleHit(obstacleRect, ray)` przyjmują wyłącznie obiekt `ray` (1–2 parametry zamiast 8–9).
4. **`Wire` (`{ kind, wireKey, fromEntity, toEntity, obstacles, start: Vec2, end: Vec2, fromBottom, targetLeft, targetRight, portIdx, totalPorts, isBottomPort, pins, pts, bounds: Rect }`)**:
   - Obiekt połączenia przechowuje własny kontekst geometryczny (`start`, `end`, obwiednię źródła/celu i listę `obstacles`), więc metody routera `resolveWireEndpoints(wire)`, `routeBottomPort(wire)`, `routeSidePort(wire)`, `buildWireWaypoints(wire)` i `routeSingleWire(wire)` przyjmują **wyłącznie 1 parametr (`wire`)**.

### 2.2. Podział na 4 moduły w `app/js/node_canvas/` (każdy < 350 LOC)

- **`app/js/node_canvas/node_canvas_geometry.js` (`NodeCanvasGeometry` — ~160 LOC)**:
  - Operacje na `Vec2`, `Rect`, `Segment2D` (`updateRect`, `copyRect`, `inRect`, `rectsOverlap`, `distSq`, `projectPointToSegment`, `findClosestPolylineSegment`, `computePointArcLengthAlongPolyline`, `computeWirePinSegmentBar`), snapping (`snapToEditorGrid`, `snapToWireGrid`, `snapToWirePinGrid`) oraz `SpatialGrid` (`toSpatialCell`, `packGridKey`, `buildSpatialGridForNodes`, `insertNodeIntoSpatialGrid`, `removeNodeFromSpatialGrid`, `updateNodeInSpatialGrids`).
- **`app/js/node_canvas/node_canvas_model.js` (`NodeCanvasModel` — ~300 LOC)**:
  - Model grafu książki: `createChapterRecord(spec)`, `getActiveNodePorts(node)`, parser (`parsePipeKVLine`, `applyCanvasLayout`, `applyBookGraph`), metryki oparte na `Rect` (`recomputeNodeMetrics`, `resolveChapterEntryNode`, `recomputeChapterInputPortPosition`, `spreadPortsAlongEdge`, `recomputeChapterPortPositions`, `computeChapterFrameRect`, `updateChaptersIncremental`, `rebuildChapterFrames`) i mutacje (`applyIncrementalNodeMove`, `renameNode`, `moveNodePortInMemory`).
- **`app/js/node_canvas/node_canvas_router.js` (`NodeCanvasRouter` — ~280 LOC)**:
  - Ortogonalny router połączeń oparty na `this.activeWire` i `OrthogonalRay`: `testObstacleHit(obstacleRect, ray)`, `linecast(ray)`, `resolveClearCoordInDir(ray, dir)`, `resolveClearCoord(ray)`, `routeBottomPort(wire)`, `routeSidePort(wire)`, `buildWireWaypoints(wire)`, `buildPinnedWireWaypoints(wire, autoPts)`, `resolveWireEndpoints(wire)`, `routeSingleWire(wire)`, `createWireRecord(spec)`, `buildWireCatalogFromMemory()`, `route()`, `rerouteAfterWirePinMutation(wire)`.
- **`app/js/node_canvas/node_canvas_helpers.js` (`NodeCanvasHelpers` — ~320 LOC)**:
  - Stan sesji interakcji (`selectedNodeIds`, `activeCanvasGesture`, `pendingContextMenuWorldPos`, `hitTypes`, `gestures`, `tokens`), warstwowy `hitTest(worldPoint)` (`hitTestPortsAndPins`, `hitTestNodeCard`, `hitTestChaptersAndWires`), detekcja kolizji przeciągania oparta na `Rect` (`collectCollisionCells(boundsRect, seenCellKeys, collisionCells)`, `evaluateNodeDragGhost(items, rawDelta)`, `resolveValidChapterDragDelta(dragGesture, desiredDelta)`, `resolveNonOverlappingNodeCoords(basePoint, chapterName)`) oraz pływające inputy (`applyBoxStyle(inputEl, rect, fontPx, letterSpacing)`, `updateFloatingInputTransform()`, `openFloatingInput(inputEl)`).

---

## 3. Szczegółowy plan zmian plik po pliku, metoda po metodzie

### 1. `app/js/node_canvas/node_canvas_style.js` (`NodeCanvasStyle`)
- Dodać sekcję `hit`:
  `portRadiusSq: 100`, `pinBarDistSq: 64`, `wireDistSq: 49`, `wireBoundsPad: 8`, `chapterHeaderH: 28`, `chapterGripW: 26`, `choiceGripRight: 20`, `choiceTrashOffset: 26`.

### 2. `app/js/node_canvas/node_canvas_geometry.js` (nowy plik — `NodeCanvasGeometry`)
Wszystkie metody przyjmują maksymalnie 3 semantyczne parametry (`Vec2` / `Rect` / skalar):
- `_segProj: { t: 0, distSq: 0, lenSq: 0 }`
- `updateRect(rect)`: 1 parametr; ustawia `rect.left = rect.x, rect.top = rect.y, rect.right = rect.x + rect.w, rect.bottom = rect.y + rect.h` i zwraca `rect`
- `copyRect(targetRect, sourceRect)`: 2 parametry; kopiuje `{ x, y, w, h }` z `sourceRect` do `targetRect` i wywołuje `this.updateRect(targetRect)`
- `inRect(point, rect)`: `point.x >= rect.left && point.x <= rect.right && point.y >= rect.top && point.y <= rect.bottom` (2 parametry zamiast 6)
- `rectsOverlap(rectA, rectB, margin)`: sprawdza przecięcie dwóch `Rect` z opcjonalnym odstępem `margin || 0` (zastępuje 5 ręcznych warunków AABB)
- `distSq(pointA, pointB)`: `(pointA.x - pointB.x) ** 2 + (pointA.y - pointB.y) ** 2`
- `toSpatialCell(coord)` / `packGridKey(cellX, cellY)`: `((cellY & 0xffff) << 16) | (cellX & 0xffff)`
- `snapToEditorGrid(value)` / `snapToWireGrid(value)` / `snapToWirePinGrid(value)`
- `projectPointToSegment(point, segStart, segEnd)`: przyjmuje 3 obiekty `Vec2` (zamiast 6 skalarów `px, py, ax, ay, bx, by`)
- `findClosestPolylineSegment(points, point)` (2 parametry)
- `computePointArcLengthAlongPolyline(points, point)` (2 parametry)
- `computeWirePinSegmentBar(wire, pin)` (2 parametry; zwraca obiekt `Rect` z polami `isHoriz, segIdx, start: Vec2, end: Vec2, x, y, w, h, center: Vec2`)
- `buildSpatialGridForNodes(nodeList)` / `insertNodeIntoSpatialGrid(grid, node)` / `removeNodeFromSpatialGrid(grid, node)` / `updateNodeInSpatialGrids(node)`

### 3. `app/js/node_canvas/node_canvas_model.js` (nowy plik — `NodeCanvasModel`)
- `tokens`: słownik tokenów rekordu i ról
- `getActiveNodePorts(node)`: zwraca aktywną tablicę portów węzła (`ports` lub `[implicitPort]`)
- `createChapterRecord(spec)`: jednolity konstruktor obiektu rozdziału używający `NodeCanvasGeometry.updateRect`
- `recomputeNodeMetrics(node)`: ustawia `node.w`, `node.h`, wywołuje `NodeCanvasGeometry.updateRect(node)` oraz wylicza `badgeW`, `branchRadioX`, `branchRadioY`, `titleMaxW` i pozycje portów
- `resolveChapterEntryNode(chapter)` / `recomputeChapterInputPortPosition(chapter)`
- `spreadPortsAlongEdge(ports, axisProp, minGap, maxCoord)`: rozsuwa porty na krawędzi (`bottomPorts`, `leftPorts`, `rightPorts`)
- `recomputeChapterPortPositions(chapter)`: używa `this.getActiveNodePorts` i `this.spreadPortsAlongEdge`
- `computeChapterFrameRect(matchingNodes, draggedRects)`: zwraca `Rect` przez `NodeCanvasGeometry.updateRect`
- `updateChaptersIncremental(chaptersSet, hasInterChapterPortMove)` i `rebuildChapterFrames()` (rozdzielone z `updateChapterGroupFrames`, używają `NodeCanvasGeometry.copyRect(ch, rect)`)
- `parsePipeKVLine(rawLine)` / `applyCanvasLayout(rawLayout, resetCanvas)` / `applyBookGraph(rawText)`
- `applyIncrementalNodeMove(items, snappedDelta)` (przyjmuje `snappedDelta: Vec2 { x, y }`)
- `renameNode(oldId, newId)` / `moveNodePortInMemory(node, fromIdx, toIdx)`

### 4. `app/js/node_canvas/node_canvas_router.js` (nowy plik — `NodeCanvasRouter`)
Zero wieloparametrowych funkcji; routing operuje na `this.activeWire` oraz obiekcie `OrthogonalRay`:
- `activeWire: null`, `_ray: { isVert: false, coord: 0, spanStart: 0, spanEnd: 0, dir: 0, pad: 0 }`
- `makeRay(isVert, coord, spanStart, spanEnd, dir, pad)`: wypełnia i zwraca bezalokacyjny `this._ray`
- `ptsList()` / `simplifyOrthogonalPoints(points)`
- `testObstacleHit(obstacleRect, ray)`: 2 parametry; porównuje `obstacleRect` z `ray` oraz `this.activeWire.fromEntity` / `this.activeWire.toEntity`
- `linecast(ray)`: 1 parametr; przeszukuje `this.activeWire.obstacles` (przez `SpatialGrid` lub tablicę) z jednolitym wyborem najbliższej przeszkody (`bestDist`) i priorytetem `hitKind === 2`
- `resolveClearCoordInDir(ray, dir)` / `resolveClearCoord(ray)`: 1–2 parametry
- `routeBottomPort(wire)`: 1 parametr (`wire`); wyznacza punkty dla wyjścia dolnego
- `routeSidePort(wire)`: 1 parametr (`wire`); wyznacza punkty dla wyjścia bocznego
- `buildWireWaypoints(wire)`: 1 parametr (`wire`); deleguje do `routeBottomPort(wire)` lub `routeSidePort(wire)`
- `buildPinnedWireWaypoints(wire, autoPts)` / `getChapterOutputPortSlot(chapter, targetChapterLower)` / `updateWireBounds(wire, points)`
- `resolveWireEndpoints(wire, nodeObstacles, chapterObstacles)`: wypełnia na `wire` pola `x1, y1, x2, y2, fromBottom, targetLeft, targetRight, totalPorts, fromEntity, toEntity, obstacles`
- `routeSingleWire(wire, nodeObstacles, chapterObstacles)`: ustawia `this.activeWire = wire`, wywołuje `resolveWireEndpoints`, wylicza `wire.pts`, ustawia `wire.colorIdx` i `updateWireBounds(wire, wire.pts)`
- `createWireRecord(spec)` / `buildWireCatalogFromMemory()` / `route()` / `rerouteAfterWirePinMutation(wire)`

### 5. `app/js/node_canvas/node_canvas_helpers.js` (odchudzony plik — `NodeCanvasHelpers`)
- `selectedNodeIds`, `activeCanvasGesture`, `pendingContextMenuWorldPos`, `hitTypes`, `gestures`, `tokens`
- `choiceRowY(idx)` / `isHistoryPreview()` / `getNodeFooterBtnX(node, kind, isPreview)`
- `clientToStudioWorld(viewport, clientX, clientY)` / `buildPointerContext(viewport, event, isTouch)`
- **Warstwowy `hitTest(worldX, worldY)`**:
  - `hitTestPortsAndPins(worldPoint)` (używa `NodeCanvasGeometry.distSq` i `NodeCanvasGeometry.projectPointToSegment`)
  - `hitTestNodeCard(node, worldPoint, isPreview)` (używa współrzędnych lokalnych `localX = worldPoint.x - node.x`, `localY = worldPoint.y - node.y`)
  - `hitTestChaptersAndWires(worldPoint, isPreview)` (używa `NodeCanvasGeometry.inRect` oraz `NodeCanvasGeometry.projectPointToSegment(worldPoint, pts[j], pts[j + 1])`)
  - `hitTest(worldX, worldY)` (zachowuje sygnaturę `(wx, wy)` dla kompatybilności z testami runtime, tworząc wewnątrz `worldPoint = { x: worldX, y: worldY }`)
- `isSameHoveredItem(firstItem, secondItem)`
- `findClosestChapterAtWorldCoords(worldPoint)`
- `resolveNonOverlappingNodeCoords(basePoint, chapterName)` (używa `NodeCanvasGeometry.rectsOverlap`)
- `resolveValidChapterDragDelta(dragGesture, desiredDelta)` (przyjmuje obiekt gestu `dragGesture` i `desiredDelta: Vec2 { x, y }`, używa `NodeCanvasGeometry.rectsOverlap`)
- `collectCollisionCells(boundsRect, seenCellKeys, collisionCells)` (używa `NodeCanvasGeometry.packGridKey`)
- `evaluateNodeDragGhost(items, desiredDx, desiredDy)` (używa `NodeCanvasGeometry.rectsOverlap`, `NodeCanvasModel.computeChapterFrameRect` i `this.collectCollisionCells`)
- `applyBoxStyle(inputEl, rect, fontPx, letterSpacing)` / `updateFloatingInputTransform()` / `openFloatingInput(inputEl)`

### 6. Aktualizacja wywołań w `NodeCanvasController`, `NodeCanvasRenderer` i `engine/bundle.go`
1. **`app/js/controllers/node_canvas_controller.js`**:
   - Wywołania `openFloatingInput(inp)`, `NodeCanvasGeometry.snapToEditorGrid`, `NodeCanvasGeometry.snapToWirePinGrid`, `NodeCanvasGeometry.computePointArcLengthAlongPolyline`, `NodeCanvasGeometry.inRect`, `NodeCanvasModel.*` oraz `NodeCanvasRouter.*`.
2. **`app/js/node_canvas/node_canvas_renderer.js`**:
   - Usunięcie martwego parametru `bridgesBySeg` z `pushWirePolylineQuads`.
   - W podglądzie ciągniętego kabla (`L822–823`): wywołanie `NodeCanvasRouter.buildWireWaypoints({ x1: gesture.startX, y1: gesture.startY, x2: gesture.curX, y2: gesture.curY, isBottomPort: Boolean(gesture.isBottomPort) })`.
   - Odwołania do `NodeCanvasGeometry.computeWirePinSegmentBar` i `NodeCanvasModel.recomputeNodeMetrics`.
3. **`engine/bundle.go`**:
   - Skanowanie podkatalogów pomocniczych `app/js/<subdir>/*.js` (np. `app/js/node_canvas/*.js`) przed `app/js/controllers/*.js` w `discoverDirModuleFiles` i `discoverFSModuleFiles`.
