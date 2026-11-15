# Plan ujednolicenia obsługi zdarzeń `NodeCanvasController` z `Dispatcher` (`node_canvas_dispatcher_bindings.md`)

## 1. Co tam jest i jakie są problemy (What is there & Problems)

1. **Równoległa rejestracja zdarzeń w `NodeCanvasController.bindEvents()` omijająca `Dispatcher` (`app/js/controllers/node_canvas_controller.js:30, 35–37, 929–1172`)**:
   - `NodeCanvasController` używa `"body": { init: this.bindEvents }` oraz flagi `this.data.eventsBound`, aby ręcznie wywołać 8 razy `document.addEventListener` (`keydown`, `keyup`, `wheel`, `mousedown`, `mousemove`, `mouseup`, `contextmenu`, `dblclick`).
   - Narusza to architekturę `ARCH.md §5` i `AGENTS.md §0` (zakaz duplikacji mechanizmów routingu zdarzeń; wszystkie bindowania DOM mają być deklaratywne w `bindings()`).
2. **Brak `mouseup`, `contextmenu` i `dblclick` oraz opcji `{ passive: false }` dla `wheel` w `Dispatcher` (`app/js/dispatcher.js:15–21, 392–395`)**:
   - `Dispatcher.events` nie nasłuchuje `mouseup`, `contextmenu` ani `dblclick`.
   - `Dispatcher.bind()` rejestruje `wheel` bez `{ capture: true, passive: false }`, co w przeglądarkach blokuje `e.preventDefault()` przy zoomowaniu kółkiem.
3. **Gubienie współdzielonych bindowań na tym samym elemencie (`"body"`) w `Dispatcher.resolve(e)` (`app/js/dispatcher.js:226–246, 381–386`)**:
   - Warunek `bestEl !== matchedEl` w `Dispatcher.resolve(e)` zachowuje wyłącznie pierwszy kontroler dopasowany do danego elementu DOM (np. `AppShellController` na `"body"`), ignorując kolejne kontrolery bindujące to samo zdarzenie na `"body"` (`NodeCanvasController`, `StudioController`).
4. **Zduplikowany skrót `Ctrl/Cmd+Z` / `Ctrl/Cmd+Y` (`UndoEditor` / `RedoEditor`) w `AppShellController.onKeydown` (`app/js/controllers/app_shell_controller.js:100–101, 109–112, 244–258`)**:
   - `AppShellController.onKeydown` duplikuje wysyłanie `UndoEditor` / `RedoEditor` z `NodeCanvasController` bez wywołania `flushSync()` i bez sprawdzenia obecności `#timeline-scroll-viewport`.

---

## 2. Rozwiązania (Solutions)

1. **Rozszerzenie `Dispatcher` o brakujące zdarzenia i wielokrotne handlery na tym samym elemencie (`bestEntries`)**:
   - Dodanie `"dblclick"`, `"contextmenu"`, `"mouseup"` do `Dispatcher.events` oraz `wheel: "wheel"` do `Dispatcher.settings.evNames`.
   - Przekazanie `{ capture: true, passive: false }` dla `wheel` w `Dispatcher.bind()`.
   - Zbieranie w `Dispatcher.resolve(e)` wszystkich wpisów `bestEntries` dopasowanych do tego samego najgłębszego elementu `bestEl` (`matchedEl === bestEl`) i wywoływanie ich w pętli w `Dispatcher.handle(e)`.
2. **Przeniesienie wszystkich 8 listenerów z `bindEvents()` do `NodeCanvasController.bindings()`**:
   - Usunięcie `eventsBound` z `NodeCanvasController.data`, usunięcie `bindEvents()` oraz stałych `evKeyDown`..`evDblClick` z `settings.values`.
   - Zadeklarowanie w `NodeCanvasController.bindings()`:
     - na `"body"`: `keydown: this.onKeydown`, `keyup: this.onKeyup`, `mousedown: this.onMousedown`, `mousemove: this.onMousemove`, `mouseup: this.finishGesture`,
     - na `"#timeline-scroll-viewport"`: `wheel: this.onWheel`, `contextmenu: this.onContextMenu`, `dblclick: this.onDblclick`.
   - Rozbicie monolitycznego `bindEvents()` na nazwane metody kontrolera: `onKeydown`, `onKeyup`, `onWheel`, `onMousedown`, `onMousemove`, `onContextMenu`, `onDblclick` oraz bezpośrednie podpięcie `finishGesture` pod `mouseup`.
3. **Usunięcie duplikatu `UndoEditor` / `RedoEditor` z `AppShellController.onKeydown`**:
   - Pozostawienie obsługi `UndoEditor` / `RedoEditor` w `NodeCanvasController.onKeydown` (gdzie wykonywany jest `this.flushSync()` oraz weryfikacja `#timeline-scroll-viewport`).

---

## 3. Zmiany plik po pliku, metoda po metodzie, linia po linii

### 1. `app/js/dispatcher.js`
- **`L15–21` (`events`)**: Dodać `"dblclick"`, `"contextmenu"`, `"mouseup"`.
- **`L49–56` (`settings.evNames`)**: Dodać `wheel: "wheel"`.
- **`L225–246` (`resolve(e)`)**: Zamiast pojedynczego `bestEntry` gromadzić tablicę `bestEntries` dla najgłębszego `bestEl` (reset przy głębszym `bestEl.contains(matchedEl)`, `push` przy `matchedEl === bestEl`) i zwracać `entries: bestEntries` w obiekcie `hit`.
- **`L381–386` (`handle(e)`)**: Iterować po `hit.entries` i wywoływać `entry.handler.call(entry.ctrl, params, hit.el, e, hit)`.
- **`L392–395` (`bind()`)**: Dla `evt === this.settings.evNames.wheel` przekazywać `{ capture: true, passive: false }`.

### 2. `app/js/controllers/node_canvas_controller.js`
- **`L30` (`data.eventsBound`)**: Usunąć pole `eventsBound: false`.
- **`L33–40` (`bindings()`)**: Zastąpić `"body": { init: this.bindEvents }` deklaratywnymi bindowaniami na `"body"` (`keydown`, `keyup`, `mousedown`, `mousemove`, `mouseup`) oraz `"#timeline-scroll-viewport"` (`wheel`, `contextmenu`, `dblclick`).
- **`L69–85` (`settings.metrics`)**: Dodać stałe `wheelZoomCtrlSens`, `wheelZoomSens`, `ctxMenuOffsetX`, `ctxMenuOffsetY`, `ctxMenuMinEdgePx`, `ctxMenuMaxRightPadPx`, `ctxMenuMaxBottomPadPx`.
- **`L164–171` (`settings.values`)**: Usunąć `evKeyDown`, `evKeyUp`, `evWheel`, `evMouseDown`, `evMouseMove`, `evMouseUp`, `evContextMenu`, `evDblClick`.
- **`L841–843` (`finishGesture()`)**: Zmienić warunek początkowy na `if (!g || g.isTouch) return;`, aby metoda mogła być bezpośrednio handlerem `mouseup` w `bindings()`.
- **`L929–1172` (`bindEvents()` -> metody kontrolera)**: Zastąpić `bindEvents()` metodami `onKeydown(_params, _el, e)`, `onKeyup(_params, _el, e)`, `onWheel(_params, viewport, e)`, `onMousedown(_params, _el, e)`, `onMousemove(_params, _el, e)`, `onContextMenu(_params, vp, e)`, `onDblclick(_params, vp, e)`.

### 3. `app/js/controllers/app_shell_controller.js`
- **`L98–112` (`settings.keys`, `settings.historyActions`)**: Usunąć nieużywane `z`, `y` oraz `historyActions`.
- **`L244–258` (`onKeydown`)**: Usunąć zduplikowany blok wysyłający `UndoEditor` / `RedoEditor`.

### 4. `app/js/node_canvas/node_canvas_renderer.js`
- **`L1016–1021` (`render()`)**: Jeśli `studioGL.canvas` już istnieje i `canvas !== studioGL.canvas`, podmienić `canvas` na `studioGL.canvas` przed `initContext(canvas)`, aby `EditorContextController.initContextPanel()` wywołany przed `NodeCanvasController.initCanvas()` nie odłączał zachowanego elementu `<canvas>` od `document.body`.

