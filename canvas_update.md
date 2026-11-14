# Plan Refaktoryzacji: Autonomiczny `NodeCanvas` i Komunikacja `StudioController <-> NodeCanvas` (`canvas_update.md`)

Dokument definiuje architekturę i plan przebudowy podsystemu canvasa zgodnie z `@ARCH.md` i `@AGENTS.md`.

---

## 1. Co jest teraz i jakie są problemy (`What is there & Problems`)

1. **Wymieszanie domeny książki (`Book`) z geometrią UI (`Canvas`)**:
   - `internal/book`: `Scene` przechowuje `X, Y`, a `Book` trzyma `EditorPanX`, `EditorPanY`, `EditorZoom`, `HasEditorView`, `EditorWirePins` oraz stałe pikseli (`NodeCardWidth`, `NodeMinLogicalHeight`).
   - `internal/views/studio_view.go` (`buildStudioGraphPayload`) skleja topologię książki (`.playtxt`) i geometrię układu (`.editor.cfg`) w jeden monolityczny string `studioGraphPayload` i wstrzykuje go zarówno do `studioScript`, jak i do ukrytego `<pre id="studio-graph-data">` w DOM (`studio_view.tpl`).
2. **Brak czystego podziału `StudioController` <-> `NodeCanvas`**:
   - Logika canvasa jest rozrzucona po 8 globalnych obiektach (`CanvasModel`, `CanvasLayout`, `CanvasHittest`, `CanvasDrag`, `CanvasInput`, `CanvasOverlays`, `EditorWires`, `EditorWebgl` / `EditorWebglScene`), z których część rejestruje się w `Dispatcher` jako pseudo-kontrolery bez pary w `internal/views/`, a `StudioController` sięga bezpośrednio do wewnętrznych pól `CanvasModel.editorCanvasState` i `CanvasModel.studioGraphState`.
   - Stałe w `CanvasModel` używają zabronionego `SCREAMING_SNAKE_CASE` (`EDITOR_GRID_SIZE`, `NODE_CARD_WIDTH`, `FONT_STYLES`).
3. **Synchroniczny spam sieciowy zamiast Lazy Sync**:
   - `CanvasDrag` (`saveBatchNodePositions`, `saveWirePins`) i `CanvasInput` wysyłają `POST /api/action` (`SaveEditorState`) natychmiast przy każdym puszczeniu kursora/rolki lub końcu przeciągania.
   - Każda edycja w inspektorze (`EditorContextController`) odsyła pełny `studioGraphPayload` z pozycjami `x, y` z serwera, co grozi nadpisaniem lokalnego układu podczas pracy.
4. **Brak deklaratywnego szablonu wyglądu węzła (`NodeTemplate`)**:
   - Wymiary kart, offsety portów, promienie, fonty MSDF i kolory są zahardkodowane wewnątrz pętli budujących bufory WebGL (`EditorWebglScene`) oraz obliczeń metryk (`CanvasModel.recomputeNodeMetrics`).

---

## 2. Rozwiązanie Docelowe (`Solutions`)

### 2.1. Rozdział warstw danych: `bookGraph` (Topologia) vs `canvasLayout` (Geometria)
- **Topologia książki (`bookGraph`)**: Czysta struktura domenowa z `.playtxt` (`chapter`, `scene`, `choice`), bez pikseli (`x, y`, `w, h`, `pan`, `zoom`). Przychodzi w ramce FDP `StudioController` (`target=none` przy aktualizacjach lub `#workspace` przy wejściu).
- **Geometria układu (`canvasLayout` + `layoutHash`)**: Kamera (`panX`, `panY`, `zoom`), pozycje węzłów (`pos | id:<scene> | x:<int> | y:<int>`) oraz piny kabli (`pin | key:<wire> | pts:<x,y;...>`). Przychodzi **wyłącznie przy otwarciu projektu (`OpenStudioProject` / boot) lub gdy zmieni się `layoutHash`** (np. po `Undo`/`Redo` rewizji z historii).

#### Przykład ramki FDP przy otwarciu projektu / resecie widoku:
```text
controller=StudioController
target=#workspace
bookId=solaris
startSceneId=intro
selectedSceneId=intro
layoutHash=8f3a12c0
_body=html

<section class="view-panel" id="studio-view">...</section>
---
controller=StudioController
target=none
layoutHash=8f3a12c0
_body=bookGraph

chapter | id:prolog | name:Prolog | root:1 | start:intro
scene | id:intro | name:Przebudzenie | chapter:prolog | start:1 | entry:1
choice | scene:intro | idx:0 | label:Otwórz drzwi | target:korytarz
choice | scene:intro | idx:1 | label:Zostań w kajucie | target:kajuta
scene | id:korytarz | name:Pusty korytarz | chapter:prolog | next:END
---
controller=StudioController
target=none
panX=120
panY=80
zoom=1
hasView=true
_body=canvasLayout

pos | id:intro | x:120 | y:96
pos | id:korytarz | x:384 | y:96
pin | key:intro:0->korytarz | pts:336,130;336,160
```

#### Przykład ramki FDP przy edycji w inspektorze (np. zmiana tytułu sceny lub dodanie wyboru):
```text
controller=EditorContextController
target=#editor-context-view
sceneId=intro
_body=html

<aside class="editor-context-panel" id="editor-context-view">...</aside>
---
controller=StudioController
target=none
selectedSceneId=intro
layoutHash=8f3a12c0
_body=bookGraph

chapter | id:prolog | name:Prolog | root:1 | start:intro
scene | id:intro | name:Przebudzenie (Zmienione) | chapter:prolog | start:1 | entry:1
choice | scene:intro | idx:0 | label:Otwórz drzwi | target:korytarz
```
*(Brak ramki `canvasLayout` — `NodeCanvas` nakłada nową topologię na istniejące w pamięci `x, y`, `panX`, `panY`, `zoom` i przebudowuje tylko kable i batche WebGL).*

---

### 2.2. Autonomiczny `NodeCanvas` ze swoim stanem i Lazy Sync (`Idle / Interval / Flush`)
- `NodeCanvas` jest właścicielem stanu wizualnego w pamięci przeglądarki:
  - `camera`: `{ panX, panY, zoom, initialized, bookId }`
  - `layout`: `{ positions: Map<id, {x, y}>, wirePins: Map<key, [{x, y}]>, layoutHash }`
  - `dirty`: `{ camera: false, positions: Set<id>, wirePins: Set<key>, timer: null, lastSyncAt: 0 }`
- **Reguły synchronizacji (`SaveEditorState` -> `target=none`, HTTP `204`)**:
  1. **Zero requestów podczas interakcji**: Podczas `pan`, `zoom`, `node-drag`, `chapter-drag`, `wire-pin-drag` mutowany jest wyłącznie stan lokalny `NodeCanvas` (120fps WebGL).
  2. **Idle Debounce (`1200ms` po zakończeniu gestu)**: Po `pointerup` / `wheel` timer bezczynności wysyła skumulowany batch `SaveEditorState` tylko wtedy, gdy `activeGesture === null`.
  3. **Max Interval (`10s`) / Visibility Flush**: Przy ciągłej pracy lub przed przełączeniem widoku (`SwitchView`, `OpenBook`, `Undo`, `Redo`) `StudioController.flushCanvasState()` wymusza natychmiastowy zapis zaległych zmian.

---

### 2.3. Deklaratywny Szablon Wyglądu (`NodeCanvas.template`)
Wyizolowany obiekt konfiguracyjny `template` w `NodeCanvas` (zero magic numbers w rendererze i hit-teście, 100% `camelCase`):
- `grid`: `{ size: 24, wireStep: 6 }`
- `card`: `{ width: 192, halfWidth: 96, headerHeight: 34, choiceRowHeight: 24, footerHeight: 38, baseHeight: 72, buttonSize: 24, pad: 8 }`
- `chapter`: `{ minWidth: 240, minHeight: 120, padX: 36, padTop: 52, padBottom: 44, gap: 24, nodeGap: 48 }`
- `ports`: `{ nodeRadius: 6.5, chapterRadius: 3.5, chapterPitch: 37 }`
- `wires`: `{ pad: 14, tightPad: 10, collectorOffset: 12, bboxPad: 18, dashDuty: 0.54 }`
- `typography`: `{ emPx: 23.0, pixelRange: 3.0, styles: { title, choice, chapter, badge, button } }`

---

### 2.4. Publiczne API Helperów `NodeCanvas` (dla `StudioController`)
`StudioController` jest jedynym kontrolerem FDP rozmawiającym z `Dispatcher` w imieniu widoku Studio i steruje `NodeCanvas` wyłącznie przez jego metody:
- **Cykl życia i dane**:
  - `NodeCanvas.mount(canvasEl, viewportEl)`
  - `NodeCanvas.setLayout(layoutPayload, cameraMeta)` — aplikuje pozycje z `.editor.cfg` i kamerę.
  - `NodeCanvas.setGraph(bookGraphPayload)` — aktualizuje topologię węzłów/portów z zachowaniem lokalnych `x, y`, przelicza ramki rozdziałów, trasuje kable (`routeWires()`) i odświeża WebGL.
  - `NodeCanvas.renameNode(oldId, newId)` — przepisuje klucz w `positions`, `selectedNodeIds` i `dirty.positions` bez utraty współrzędnych.
- **Kamera i układ (100% lokalne, oznaczające `dirty`)**:
  - `NodeCanvas.zoomIn()`, `NodeCanvas.zoomOut()`, `NodeCanvas.zoomAt(targetZoom, pivotX, pivotY)`
  - `NodeCanvas.resetView(forceCenter)`
  - `NodeCanvas.fitAllNodes()`
  - `NodeCanvas.autoLayout()` — wylicza układ kolumnowy/warstwowy na siatce `24px`, aktualizuje `positions`, oznacza wszystkie węzły jako `dirty` i planuje sync w `idle`.
- **Selekcja i wyszukiwanie**:
  - `NodeCanvas.selectNode(sceneId, focusCamera)`
  - `NodeCanvas.setSearchFilter(query, matchedIds)`
- **Synchronizacja**:
  - `NodeCanvas.scheduleSync()` / `NodeCanvas.flushSync()` — zwraca payload zmienionych pozycji/pinów/kamery dla `StudioController`.

---

## 3. Plan Zmian Plik po Pliku (`File-by-File Refactor Plan`)

### Krok 1: Backend — Rozdzielenie `bookGraph` i `canvasLayout` w `internal/views/studio_view.go` oraz `internal/actions/studio_actions.go`
1. **`internal/views/studio_view.go`**:
   - W `studioTemplate`: usunąć pole `StudioGraphPayload string` (i usunąć `<pre id="studio-graph-data">` z `app/templates/studio_view.tpl` — stan nie może podróżować przez ukryte tagi DOM zgodnie z `ARCH.md §4.3`).
   - W `studioScript`:
     - Zamienić `studioGraphPayload string` na:
       - `bookGraph string` (czysta topologia: `chapter | ...`, `scene | ...`, `choice | ...`)
       - `canvasLayout string` (wypełniane tylko przy `s.Overlays.ResetCanvas` / pierwszym załadowaniu książki / zmianie rewizji historii: `pos | id:... | x:... | y:...`, `pin | key:... | pts:...`)
       - `layoutHash string` (hash FNV-1a z pozycji i pinów w `.editor.cfg`)
       - `panX int`, `panY int`, `zoom float64`, `hasView bool`
   - Rozbić `buildStudioGraphPayload` na dwie czyste funkcje:
     - `buildBookGraphPayload(b *book.Book, orderedScenes []*book.Scene) string`
     - `buildCanvasLayoutPayload(b *book.Book) (payload string, hash string)`
2. **`internal/actions/studio_actions.go`**:
   - `HandleSaveEditorState`: zapisuje `pan_x`, `pan_y`, `zoom`, `positions` i `wire_pins` do `.editor.cfg` i zwraca puste `204 No Content` (`target=none`, zero HTML, zero re-renderu `studio_view`).

### Krok 2: Szablon `app/templates/studio_view.tpl`
1. Usunąć `pre.hidden#studio-graph-data {StudioGraphPayload}` z `app/templates/studio_view.tpl`.
2. Wszystkie dane startowe i aktualizacje płyną wyłącznie przez ramki FDP (`studioScript` -> `StudioController.data`).

### Krok 3: Konsolidacja i wyczyszczenie `NodeCanvas` (`app/js/canvas_*.js`)
1. **`app/js/canvas_model.js`**:
   - Usunąć rejestrację `Dispatcher.register("CanvasModel", CanvasModel)` (to moduł silnika canvasa, a nie kontroler widoku `.tpl`).
   - Zamienić wszystkie stałe `SCREAMING_SNAKE_CASE` (`EDITOR_GRID_SIZE`, `NODE_CARD_WIDTH`, `FONT_STYLES` itd.) na ustrukturyzowany obiekt szablonu `CanvasModel.template` z kluczami `camelCase` (`grid`, `card`, `chapter`, `ports`, `wires`, `typography`).
   - Rozdzielić parser na:
     - `applyCanvasLayout(rawLayout, cameraMeta)` — aktualizuje `positions` i `wirePins` oraz ustawia kamerę.
     - `applyBookGraph(rawGraph)` — buduje/aktualizuje `chapters`, `nodes` i `ports` używając współrzędnych z `positions` (a dla zupełnie nowych węzłów wyliczając domyślne wolne miejsce na siatce `24px`), po czym woła `CanvasLayout.updateChapterGroupFrames()` i `EditorWires.route()`.
   - Dodać kolejkę `dirty` + `scheduleIdleSync()` / `flushSync()`:
     - Debounce `1200ms` w stanie spoczynku (`activeCanvasGesture === null`).
     - Wywołuje callback `StudioController.syncCanvasState(payload)` wysyłający cichy request FDP (`SaveEditorState`).
2. **`app/js/canvas_drag.js` & `app/js/canvas_input.js`**:
   - Zamiast bezpośrednich wywołań `Dispatcher.post("SaveEditorState", ...)` przy każdym ruchu kamery lub końcu draga węzła/pina, wywoływać `CanvasModel.markDirtyPositions(items)`, `CanvasModel.markDirtyWirePins(wireKey, pins)` oraz `CanvasModel.markDirtyCamera()`.
   - Zachować natychmiastowe akcje domenowe zmieniające strukturę książki (`SelectStudioScene`, `ConnectChoice`, `ConnectChapter`, `ReorderChoice`), poprzedzając je `CanvasModel.flushSync()`.

### Krok 4: `StudioController` (`app/js/controllers/studio_controller.js`)
1. **`StudioController.data`**:
   - Zaktualizować pola 1:1 ze `studioScript`: `bookGraph: ""`, `canvasLayout: ""`, `layoutHash: ""`, `panX: 0`, `panY: 0`, `zoom: 1`, `hasView: false`.
2. **`StudioController.initCanvas` & `StudioController.onHydrate`**:
   - W `initCanvas`: zamontować canvas WebGL, zaaplikować `this.data.canvasLayout` (jeśli obecny) i `this.data.bookGraph`, wytrasować kable i wyrenderować scenę.
   - W `onHydrate`:
     - Jeśli przyszedł `scenePrevId` != `sceneId`, wywołać `CanvasModel.renameNode(oldSceneId, newSceneId)`.
     - Jeśli przyszedł `canvasLayout` (lub zmienił się `layoutHash` przy czystym stanie lokalnym), wywołać `CanvasModel.applyCanvasLayout(...)`.
     - Jeśli przyszedł `bookGraph`, wywołać `CanvasModel.applyBookGraph(this.data.bookGraph)` (bez resetowania kamery i lokalnych pozycji `x, y`).
3. **Helpery kamery i `syncCanvasState`**:
   - Przenieść matematykę `zoomAtPoint`, `resetToFirstNode`, `fitAllNodes` do helperów `CanvasModel`, zostawiając w `StudioController` czyste handlery zdarzeń z `bindings()` delegujące do `CanvasModel` i oznaczające kamerę jako `dirty`.

---

## 4. Weryfikacja (`Verification Gates`)
1. `go test ./internal/audit/...` — weryfikacja kontraktu `1:1 ScriptData <-> StudioController.data`, braku `SCREAMING_SNAKE_CASE` w JS, braku magic strings w metodach i czystości `.tpl`.
2. `go test ./...` — pełna weryfikacja testów jednostkowych, integracyjnych i `TestClientJSRuntimeAndInteractiveResponsiveness` w Node VM.
3. `./build.sh --macos` — przebudowa bundla produkcyjnego i weryfikacja kompilacji.
