# Plan refaktoryzacji: Uniwersalny `Dispatcher`, moduły obiektowe 1:1, dynamiczny bundler i redukcja `editor_*.js` o ~90%

## 1. Co jest obecnie i jakie są problemy (What is there & Problems)

1. **Luźna zupa globalnych funkcji i zmiennych we wszystkich plikach `app/js/*.js`**:
   - Wszystkie 20 plików w `app/js/*.js` deklaruje dziesiątki wolnostojących funkcji (`function encodeWFP`, `function qs`, `function routeStudioWires`, `function renderStudioCanvas`, `function syncVoiceStripDOM` itd.) oraz globalnych `let`/`const` wrzuconych do wspólnego scope'u.
   - Patrząc na wywołanie typu `encodeWFP(...)` lub `applyHeaders(...)`, nie wiadomo z którego pliku pochodzi dana funkcja ani kto jest właścicielem stanu.
2. **Zahardkodowane listy plików i sklejanie HTML w `internal/tpl/bundle.go`**:
   - Funkcje `CSSModuleFiles` (L228–249), `CSSPlayerModuleFiles` (L252–266), `JSModuleFiles` (L269–298) i `JSPlayerModuleFiles` (L301–312) trzymają zahardkodowane tablice nazw plików `.tss` i `.js` bezpośrednio w ciałach funkcji (łamie regułę zakazu magic-stringów w body funkcji i wymusza ręczną edycję Go przy każdym dodaniu/usunięciu pliku).
   - `RenderWasmSplashPageHTML` (L354–400) ręcznie skleja dokument HTML przez `strings.Builder` i `fmt.Fprintf` w kodzie Go zamiast użyć szablonu `.tpl`.
3. **Rozproszone listenery i ręczne wyjątki w `app/js/core.js` oraz `app/js/editor_*.js`**:
   - Każdy plik (`core.js`, `editor_context_panel.js`, `editor_history_panel.js`, `editor_view.js`, `editor_rules.js`, `editor_voice.js`) rejestruje własne `document.addEventListener("click" | "input" | "change" | "keydown" | "pointerdown" | "pointermove" | "pointerup" | "focusin" | "focusout" | "scroll")`.
   - W `core.js` (L1556–1787) i `editor_view.js` (L361–523) istnieją dziesiątki ręcznych gałęzi `e.target.closest("[data-...]")` oraz zahardkodowane listy wyjątków (`isModalOnlyAction`, `ACTIONS_CLEAR_SELECTION`, `ACTIONS_FLUSH_INSPECTOR`, `ACTIONS_KEEP_MODAL_SCROLL`).
   - W `core.js` (L852–1136) wymiana targetu ma ponad 280 linii ręcznych wyjątków dla `#modals-root` i `#editor-context-panel`.
4. **Ręczne łatanie DOM, `<form>`-walking i timery w `app/js/editor_context_panel.js` (734 linie)**:
   - Funkcje `syncBookTitleDOM`, `syncChapterGroupRenameDOM`, `syncSceneTitleAcrossForms`, `syncSceneChoiceLabelAcrossForms`, `getMatchingContextForm`, `formToParams` szukają ukrytych inputów (`qs('input[name="scene_id"]')`) i ręcznie mutują DOM zamiast zdać się na odpowiedź `POST /api/action` i podmianę `X-Target`.
   - Mapy timerów (`liveSaveTimers`, `chapterLiveSaveTimers`, `bookTitleSaveTimer`, `dirtyInspectorForms`) dublują stan backendu po stronie klienta.
5. **Brak modelu 1:1:1 (`.tpl` ↔ `.js` ↔ `.go`)**:
   - Parametry są rozrzucone po `<input type="hidden">`, `data-context-scene-id`, `data-old-title`, `data-old-chapter`, `data-choice-idx` zamiast atrybutów `param-*` i bloków `event:` bezpośrednio na elemencie.
   - Brak automatycznego wyznaczania `drop_target` w dispatcherze — `editor_history_panel.js` (`.history-row-grip`, `.history-rail-node` → `.history-lane-cell`, `.history-graph-row`) i `editor_voice.js` (`.voice-rec-trim-handle`, `.voice-rec-waveform-wrap`) ręcznie zarządzają cyklem `pointerdown` / `pointermove` / `pointerup` oraz `document.elementFromPoint`.
6. **Zbędny kod diagnostyczny `app/js/editor_perf.js` (168 linii) i `app/styles/editor_perf.tss`**:
   - Pasek performance na dole ekranu (`#editor-perf-overlay`), który owija `routeStudioWires` i `renderStudioCanvas` oraz wykonuje sztuczne pętle warmup.

---

## 2. Rozwiązanie docelowe (Solutions)

1. **Zasada 1 plik `.js` = 1 nazwany object literal (Zero luźnych funkcji i zmiennych w global scope, zero god-objectu w `Dispatcher`)**:
   - Każdy plik `app/js/<name>.js` definiuje dokładnie jeden niezależny moduł-obiekt odpowiadający 1:1 nazwie pliku:
     - `app/js/dispatcher.js` → `const Dispatcher = { ... }`
     - `app/js/wfp.js` → `const Wfp = { encode, decodeHeaderUTF8, parseWasmResponse }`
     - `app/js/core.js` → `const Core = { qs, qsa, tr, trf, applyHeaders, ... }`
     - `app/js/audio.js` → `const Audio = { ... }`
     - `app/js/player.js` → `const Player = { ... }`
     - `app/js/editor_view.js` → `const EditorView = { ... }`
     - `app/js/editor_context_panel.js` → `const EditorContextPanel = { ... }`
     - `app/js/editor_history_panel.js` → `const EditorHistoryPanel = { ... }`
     - `app/js/editor_rules.js` → `const EditorRules = { ... }`
     - `app/js/editor_voice.js` → `const EditorVoice = { ... }`
     - `app/js/modal_scene_text.js` → `const ModalSceneText = { ... }`
     - `app/js/msdf_metrics.js` → `const MsdfMetrics = { ... }`
     - `app/js/canvas_model.js` → `const CanvasModel = { ... }`
     - `app/js/canvas_layout.js` → `const CanvasLayout = { ... }`
     - `app/js/canvas_hittest.js` → `const CanvasHittest = { ... }`
     - `app/js/canvas_overlays.js` → `const CanvasOverlays = { ... }`
     - `app/js/canvas_drag.js` → `const CanvasDrag = { ... }`
     - `app/js/canvas_input.js` → `const CanvasInput = { ... }`
     - `app/js/editor_webgl.js` → `const EditorWebgl = { ... }`
     - `app/js/editor_webgl_scene.js` → `const EditorWebglScene = { ... }`
     - `app/js/editor_wires.js` → `const EditorWires = { ... }`
   - Każde wywołanie międzyplikowe wskazuje jawnie moduł docelowy: np. `Wfp.encode(...)`, `Core.qs(...)`, `Core.applyHeaders(resp)`, `EditorWires.route(...)`, `EditorWebglScene.render(...)`.
   - `Dispatcher` **nie zna** żadnych konkretnych paneli ani komponentów (zero `Dispatcher.elements.xyz`) — rozwiązuje moduł docelowy generycznie.
2. **Dynamiczne odkrywanie modułów `.tss`/`.js` i szablon `.tpl` dla WASM Splash (`internal/tpl/bundle.go`)**:
   - Całkowite usunięcie zahardkodowanych list plików z `CSSModuleFiles`, `CSSPlayerModuleFiles`, `JSModuleFiles`, `JSPlayerModuleFiles` — bundler dynamicznie skanuje katalogi `app/styles/*.tss` oraz `app/js/*.js` (`os.ReadDir` / `fs.ReadDir`).
   - Usunięcie `RenderWasmSplashPageHTML` z `internal/tpl/bundle.go` i przeniesienie całego markupu do `app/templates/wasm_splash.tpl` renderowanego standardowo przez silnik `.tpl`.
3. **Deklaratywna składnia `param-*` i bloków `event:` w `.tpl` (`internal/tpl/tpl.go`)**:
   - Przykład rzeczywistego węzła na szynie historii w `app/templates/editor_history_panel.tpl`:
     ```tpl
     button.history-rail-node .is-current-node?: {row.is_current}
       type: button
       param-revision: "{row.id}"
       param-branch: "{row.branch}"
       param-label: "{row.label}"
       param-is-current: "{row.is_current}"
       event: pointerdown
         action: startNodeDrag
         kind: EditorHistoryPanel
       event: pointermove
         action: moveNodeDrag
         kind: EditorHistoryPanel
       event: pointerup
         action: endNodeDrag
         kind: EditorHistoryPanel
       event: click
         action: PreviewHistoryRevision
         kind: backend
     ```
   - Kompilator `.tpl` tłumaczy wcięty blok `event: <evt>` pod elementem na atrybuty `event-<evt>-action`, `event-<evt>-kind`, `event-<evt>-target` oraz `event-<evt>-param-<name>`.
   - Pojedyncze `action:` na elemencie (bez bloku `event:`) pozostaje skrótem dla domyślnego zdarzenia elementu (`click` dla przycisków, `change` dla inputów, `submit` dla formularzy) z domyślnym `kind: backend`.
4. **Jeden uniwersalny `Dispatcher` (object literal w `app/js/dispatcher.js`, ~100 linii, zero zahardkodowanych nazw komponentów)**:
   - Bindowanie wszystkich zdarzeń w jednej pętli z `capture: true`.
   - `resolve(e)` znajduje najbliższy element z `[event-<e.type>-action]` lub domyślnym `[action]` i odczytuje jego `action`, `kind` (domyślnie `"backend"`, lub nazwa modułu JS np. `"EditorHistoryPanel"`, `"EditorVoice"`, `"EditorRules"`, `"EditorView"`) oraz `target`.
   - `collectParams(hit, e)` zbiera bezpośrednio z elementu:
     - `el.name` → `params[el.name] = el.value`
     - wspólne atrybuty `param-*` → `params[name]`
     - nadpisania dla danego eventu `event-<e.type>-param-*` → `params[name]`
     - współrzędne kursora (`client_x`, `client_y`)
     - na `pointermove` / `pointerup`: automatyczny `params.drop_target` z elementu pod `document.elementFromPoint(e.clientX, e.clientY)` posiadającego `param-*` (np. `.history-lane-cell` z `param-branch` lub `.history-graph-row` z `param-revision` i `param-branch`).
   - Gdy `kind === "backend"` → `Dispatcher.post(action, params, target)` wysyła WFP `POST /api/action` przez `Wfp.encode`, aplikuje nagłówki przez `Core.applyHeaders(resp)` i podmienia `Core.qs(target || resp.headers.get("X-Target")).innerHTML = await resp.text()`.
   - Gdy `kind !== "backend"` → wywołuje metodę bezpośrednio na module ` modules[kind][action](params, el, e)` (gdzie każdy plik `.js` sam rejestruje swój moduł przez `Dispatcher.register("EditorHistoryPanel", EditorHistoryPanel)` lub wprost z mapy modułów, bez hardkodowania listy komponentów w `dispatcher.js`).
5. **Eliminacja 90% kodu z `editor_*.js`**:
   - Całkowite usunięcie `app/js/editor_perf.js` i `app/styles/editor_perf.tss`.
   - Usunięcie wszystkich ręcznych synchronizatorów DOM, `<form>`-walkerów i timerów z `editor_context_panel.js`.
   - Przeniesienie wszystkich prostych akcji UI (otwieranie/zamykanie paneli, wyszukiwarki, sekcji ustawień, zapisywanie pól formularza na `change`/`blur`) na `kind: backend` obsługiwane w 100% przez `Dispatcher.post`.

---

## 3. Zmiany plik po pliku i metoda po metodzie (File-by-File & Method-by-Method Plan)

### 1. `internal/tpl/bundle.go`, `internal/tpl/tpl.go` & `app/templates/wasm_splash.tpl`
- **`internal/tpl/bundle.go`**:
  - `CSSModuleFiles` (L228–249) & `CSSPlayerModuleFiles` (L252–266): usunięcie zahardkodowanych tablic `names := []string{...}`; zastąpienie dynamicznym skanowaniem `app/styles/*.tss`.
  - `JSModuleFiles` (L269–298) & `JSPlayerModuleFiles` (L301–312): usunięcie zahardkodowanych tablic `names := []string{...}`; zastąpienie dynamicznym skanowaniem `app/js/*.js`.
  - `RenderWasmSplashPageHTML` (L354–400): **USUŃ** ręczne sklejanie HTML w Go; zastąp renderowaniem szablonu `wasm_splash` przez `RenderTemplateTo`.
- **`app/templates/wasm_splash.tpl` (NOWY SZABLON zastępujący `RenderWasmSplashPageHTML`)**:
  - Pełny dokument `.tpl` ze strukturą `#app.app-root.wasm-player-root`, `#wasm-splash` i przyciskiem `button#btn-wasm-play` (`action: bootWasm`, `kind: Player`, `param-wasm-src: "{wasm_src}"`, `param-book-id: "{book_id}"`).
- **`internal/book/web_exporter.go`**:
  - L189: wywołanie renderowania szablonu `wasm_splash.tpl` zamiast `tpl.RenderWasmSplashPageHTML`.
- **`internal/tpl/tpl.go`**:
  - `parseElementLine` / `appendNode` (L215–295, L371–383): obsługa węzła `event: <name>` wciętego pod elementem; przepisuje `action`, `kind`, `target`, `param-*` na atrybuty rodzica `event-<name>-*`.
  - `validateNodesRecursive` (L885–960): dopuszczenie atrybutów `kind`, `target`, `param-*`, `event-*` oraz weryfikacja w teście architektury, że każdy plik `app/js/*.js` definiuje wyłącznie jeden object literal bez wolnostojących `function ...()`.

### 2. `app/js/dispatcher.js` (`const Dispatcher = { ... }`)
- **`Dispatcher` (L1–107)**:
  - `attr`: `{ action: "action", kind: "kind", target: "target", event: "event-", param: "param-" }`
  - `defaultEvents`: `{ click: 1, change: 1, submit: 1 }`
  - `fieldTags`: `{ INPUT: 1, TEXTAREA: 1, SELECT: 1 }`
  - `preventEvents`: `{ click: 1, submit: 1 }`
  - `events`: `["click", "input", "change", "submit", "focus", "blur", "keydown", "keyup", "scroll", "wheel", "pointerdown", "pointermove", "pointerup", "timeupdate", "ended"]`
  - `modules`:
    - `backend`: `new Proxy({}, { get: (_, action) => (params, el, e, hit) => Dispatcher.post(action, params, hit.target) })`
    - pozostałe moduły rejestrują się same przez `Dispatcher.register(name, mod)` (zero zahardkodowanych nazw modułów w `dispatcher.js`).
  - `resolve(e)`: sprawdza `t.closest("[event-" + e.type + "-action]")`, a następnie domyślne `t.closest("[action]")`; zwraca `{ el, action, kind, target, eventPrefix }`.
  - `collectParams(hit, e)`: czyta `el.name`/`el.value`, `param-*` oraz `<eventPrefix>param-*`; na `pointermove`/`pointerup` wyznacza `params.drop_target` przez `document.elementFromPoint(e.clientX, e.clientY)` (zbiera `param-*` z elementu pod kursorem).
  - `post(action, params, target)`: wysyła WFP `POST /api/action` przez `Wfp.encode(Object.assign({ action }, params))`, woła `Core.applyHeaders(resp)` i ustawia `Core.qs(target || resp.headers.get("X-Target")).innerHTML = await resp.text()`.
  - `handle(e)` & `bind()`: pojedyncza rejestracja zdarzeń w `capture: true`.

### 3. `app/js/wfp.js` (Wydzielony z `core.js` moduł `const Wfp = { ... }`)
- Przeniesione z `app/js/core.js`:
  - `encodeWFP` → `Wfp.encode(paramsObj)`
  - `decodeHeaderUTF8` → `Wfp.decodeHeaderUTF8(rawVal)`
  - `parseWasmActionResponse` → `Wfp.parseWasmResponse(rawStr)`

### 4. `app/js/core.js` (`const Core = { ... }`)
- **Zamiana luźnych funkcji na metody `Core`**:
  - `qs` → `Core.qs(sel, root)`
  - `qsa` → `Core.qsa(sel, root)`
  - `tr` / `trf` → `Core.tr(key)` / `Core.trf(key, ...args)`
  - `showToast` → `Core.showToast(message)`
  - `applyServerResponseHeaders` → `Core.applyHeaders(resp)`
  - `hideUITooltip` / `positionAndShowUITooltip` / `setElementHint` → `Core.hideTooltip()` / `Core.showTooltip(el)` / `Core.setHint(el, text)`
- **USUŃ z `app/js/core.js`**:
  - `sendServerAction` (L754–1195) oraz `postServerActionSilent` — zastąpione przez `Dispatcher.post`.
  - `bindDeclarativeActionBridge` (L1430–1851) — cały blok `document.addEventListener(...)` wraz z ręcznymi wyjątkami zastąpiony przez `Dispatcher.bind()`.

### 5. `app/js/editor_perf.js` & `app/styles/editor_perf.tss` (CAŁKOWITE USUNIĘCIE)
- **Usuń pliki**: `app/js/editor_perf.js` (L1–168) oraz `app/styles/editor_perf.tss`.
- **`app/templates/editor_view.tpl`**: usuń `div#editor-perf-overlay.editor-perf-overlay.hidden` (L193–194).
- **`internal/server/server_test.go`**: usuń `.editor-perf-overlay` z listy overlayów w teście kontraktu.

### 6. `app/js/editor_context_panel.js` (`const EditorContextPanel = { ... }`, 734 linie → ~35 linii)
- **USUŃ całkowicie funkcje (zastąpione przez `param-*` na elementach w `app/templates/editor_context_panel.tpl` i `app/templates/editor_breakdown_panel.tpl` oraz `kind: backend` w `Dispatcher`)**:
  - `formToParams` (L11–24), `getMatchingContextForm` (L26–50)
  - `syncBookTitleDOM` (L113–125), `commitBookTitleInput` (L127–146), `scheduleBookTitleLiveSave` (L148–158) — `input#book-settings-title-input` wysyła `action: RenameBook` na `change`.
  - `syncChapterGroupRenameDOM` (L160–219), `commitChapterTitleInput` (L221–241), `scheduleChapterLiveSave` (L243–248) — input rozdziału wysyła `action: RenameChapter` z `param-old-chapter: "{chapter}"` na `change`.
  - `cancelPendingInspectorTimers` (L250–272), `flushPendingDirtyFormsInternal` (L274–300), `consumeDirtyInspectorFormIntoParams` (L302–304), `flushInspectorFormIfDirty` (L306–308)
  - `updateEditorContextPanelVisibility` (L310–322), `selectNodeForContextPanel` (L324–343), `selectChapterForContextPanel` (L345–380), `hideEditorContextPanel` (L382–402), `syncNodeSelectionDOM` (L404–486)
  - `scheduleNodeLiveSave` (L488–499), `scheduleBookSettingsLiveSave` (L515–526)
  - `triggerCanvasAddChoiceForNode` (L528–533), `triggerCanvasReorderChoice` (L535–539)
  - `commitBreakdownRenameInput` (L541–559) — `input.breakdown-name-input` w `editor_breakdown_panel.tpl` (L60, L92) dostaje wprost `action: RenameCharacter` (`param-old-name: "{ch.name}"`) oraz `action: RenameLocation` (`param-old-location: "{loc.name}"`), bez ukrytych `<input type="hidden">`.
  - `commitEditorField` (L561–609), `bindEditorFormEvents` (L611–734)
- **ZOSTAW w `const EditorContextPanel`**:
  - `autoResize(params, el)` (d. `autoResizeInspectorTextarea`, L501–509) — wywoływane z `textarea.node-meta-textarea` (`event: input`, `action: autoResize`, `kind: EditorContextPanel`).
  - `previewChoiceTarget(params, el)` (d. `syncChoiceTargetToCanvas`, L52–111) — wywoływane z `input.choice-exit-input` (`event: input`, `action: previewChoiceTarget`, `kind: EditorContextPanel`, `param-scene-id: "{selected_node.id}"`, `param-choice-idx: "{pt.idx}"`).

### 7. `app/js/editor_history_panel.js` (`const EditorHistoryPanel = { ... }`, 150 linii → ~45 linii)
- **Jak działa obecnie `editor_history_panel.tpl` + `editor_history_panel.js`**:
  - W `editor_history_panel.tpl` wiersze `.history-graph-row` (L35) zawierają szyny `.history-lane-cell` (L41), kropki `button.history-rail-node` (L70, L78) oraz uchwyt `span.history-row-grip` (L92) wewnątrz `.history-row-box` (L86).
  - Kliknięcie w `.history-rail-hit` wywołuje `SwitchHistoryBranch`, a kliknięcie w `.history-rail-node` / `.history-row-box` wywołuje `PreviewHistoryRevision` (lub `SwitchHistoryBranch` dla innej gałęzi).
  - Przeciągnięcie `.history-rail-node` lub `.history-row-grip` nad inną szynę `.history-lane-cell` (`param-branch`) lub inny wiersz `.history-graph-row` (`param-revision`, `param-branch`) pokazuje podgląd `#history-drop-preview-row`, a po upuszczeniu (`pointerup`):
    - dla bieżącego węzła (`is_current == "true"`): na inną gałąź → `SwitchHistoryBranch`, na inną rewizję tej samej gałęzi → `RestoreHistoryRevision`;
    - dla innego węzła: → `CopyHistoryRevisionToBranch` (`revision`, `target_branch`).
- **USUŃ z `editor_history_panel.js`**:
  - `bindHistoryGraphInteractions` (L25–150) — usunięcie wszystkich 5 ręcznych `document.addEventListener("scroll" | "click" | "pointerdown" | "pointermove" | "pointerup")` i ręcznego `document.elementFromPoint`.
- **ZOSTAW w `const EditorHistoryPanel`**:
  - `startNodeDrag(params, el, e)`: inicjuje przeciąganie na podstawie `params.revision`, `params.branch`, `params.label`, `params.is_current` z `.history-rail-node` / `.history-row-grip`.
  - `moveNodeDrag(params, el, e)`: podświetla `.history-lane-cell` i aktualizuje `#history-drop-preview-row` na podstawie `params.drop_target`.
  - `endNodeDrag(params, el, e)`: czyta `params.drop_target` (`branch`, `revision`) dostarczone przez `Dispatcher.collectParams` i wywołuje `Dispatcher.post("SwitchHistoryBranch" | "RestoreHistoryRevision" | "CopyHistoryRevisionToBranch", ...)`.
  - `updateScrollCue(params, el)`: wywoływane deklaratywnie na `#history-graph-scroll` (`event: scroll`, `action: updateScrollCue`, `kind: EditorHistoryPanel`).

### 8. `app/js/editor_view.js` (`const EditorView = { ... }`, 523 linie → ~140 linii)
- **USUŃ z `editor_view.js`**:
  - `handleEditorFormClick` (L361–523): ręczna obsługa `[data-form-toggle]`, `label.form-radio-option` oraz ręczne gałęzie `SaveBookSettings`, `SaveSceneTextModal`, `SaveScene`, `AssignChapterToScenes`, `BrowseAddBookMedia` — zastąpione deklaratywnym `action:` + `param-*` w `.tpl`.
  - Ręczne odpytywanie DOM o `modalSceneInput`, `contextForm`, `chapterForm`, `bookForm` w `initNodeEditorCanvas` (L318–343).
- **ZOSTAW w `const EditorView`**:
  - `initCanvas()`, `fitAllNodes()`, `resetToFirstNode()`, `zoomAtPoint(params, el, e)`, `centerOnScene(params)`, `syncViewportVisibility()`, `toggleSelect(params, el)`, `selectOption(params, el)`.

### 9. `app/js/editor_rules.js` (`const EditorRules = { ... }`, 725 linii → ~220 linii)
- **USUŃ**:
  - `bindSynAutocompleteDropdown` (L607–725) — usunięcie 6 globalnych listenerów (`focusin`, `focusout`, `input`, `keydown`, `pointerover`, `pointerdown`).
  - W `explainClientExitTargetError` (L304–357): usunięcie `qs('input[name="scene_id"]', form)` itd. — walidator dostaje `params.scene_id`, `params.explicit_id`, `params.chapter`, `params.book_vars` z `param-*`.
- **ZOSTAW w `const EditorRules`**:
  - `onInput(params, el, e)`, `onKeydown(params, el, e)`, `chooseItem(params, el, e)`, `validate(params, el)` (wywoływane przez `kind: EditorRules`).

### 10. `app/js/editor_voice.js` (`const EditorVoice = { ... }`, 1244 linie → ~320 linii)
- **USUŃ**:
  - `bindVoiceRecorderEvents` (L1067–1244) — usunięcie globalnych listenerów `click`, `pointerdown`, `pointermove`, `pointerup`, `pointercancel`.
  - Ręczne odczyty `stripEl.getAttribute("data-*")` w `commitVoiceTrimAutoSave` (L992–1013) i `commitVoiceTrimDestructive` (L1015–1049) — wszystko przychodzi w `params` z `param-scene`, `param-voice-section`, `param-voice-url`, `param-voice-file`.
- **ZOSTAW w `const EditorVoice`**:
  - `state`, `initStrips()`, `toggleTransport(params, el)`, `startRecord(params, el)`, `cutTrim(params, el)`, `startTrimDrag(params, el, e)`, `moveTrimDrag(params, el, e)`, `endTrimDrag(params, el, e)` (wywoływane przez `kind: EditorVoice`).

### 11. Pozostałe moduły `app/js/*.js` (Opakowanie w object literals 1:1 z nazwą pliku)
- `app/js/audio.js` → `const Audio = { ... }`
- `app/js/player.js` → `const Player = { ... }`
- `app/js/modal_scene_text.js` → `const ModalSceneText = { ... }`
- `app/js/msdf_metrics.js` → `const MsdfMetrics = { ... }`
- `app/js/canvas_model.js` → `const CanvasModel = { ... }`
- `app/js/canvas_layout.js` → `const CanvasLayout = { ... }`
- `app/js/canvas_hittest.js` → `const CanvasHittest = { ... }`
- `app/js/canvas_overlays.js` → `const CanvasOverlays = { ... }`
- `app/js/canvas_drag.js` → `const CanvasDrag = { ... }`
- `app/js/canvas_input.js` → `const CanvasInput = { ... }`
- `app/js/editor_webgl.js` → `const EditorWebgl = { ... }`
- `app/js/editor_webgl_scene.js` → `const EditorWebglScene = { ... }`
- `app/js/editor_wires.js` → `const EditorWires = { ... }`

### 12. Szablony `app/templates/editor_*.tpl` & `app/templates/components.tpl`
- Zamiana wszystkich `data-*` przekazywanych do akcji oraz ukrytych `<input type="hidden" name="scene_id">` na jawne atrybuty `param-*` i bloki `event:` w:
  - `app/templates/editor_view.tpl`
  - `app/templates/editor_context_panel.tpl`
  - `app/templates/editor_history_panel.tpl`
  - `app/templates/editor_breakdown_panel.tpl`
  - `app/templates/editor_variables_panel.tpl`
  - `app/templates/components.tpl`
