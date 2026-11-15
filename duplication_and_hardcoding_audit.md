# Audyt Duplikacji Logiki i Hardkodowania (`ARCH.md` & `AGENTS.md`)

Kompleksowy raport z audytu całego kodu aplikacji (`domain/`, `engine/`, `cmd/`, `internal/audit/`, `app/js/`, `app/templates/`, `app/styles/`, `www/`) pod kątem **zduplikowanej logiki**, **hardkodowania** (magic strings, sztywnych mapowań, duplikowanych struktur danych i widoków) oraz **zahardkodowanych selektorów w testach**.

---

## CZĘŚĆ I — ZDUPLIKOWANA LOGIKA (Najpoważniejsze problemy architektoniczne)

### 1. Całkowita duplikacja silnika widoków i akcji w `cmd/wasmplayer/main.go` vs `domain/views` + `domain/actions` + `domain/state`
* **Lokalizacja**:
  * `cmd/wasmplayer/main.go:1–843`
  * vs `domain/views/player_view.go`, `domain/views/scene_view.go`, `domain/views/audio_deck_view.go`, `domain/views/confirm_view.go`, `domain/actions/player_actions.go`, `domain/actions/confirm_actions.go`, `domain/state/player_state.go`
* **Problem**:
  `cmd/wasmplayer/main.go` (843 linie) to skopiowany monolit zawierający własne kopie struktur szablonów, skryptów, widoków i akcji:
  1. **Zduplikowane struktury `TemplateData` / `ScriptData`**: `playerTemplate`, `playerScript`, `sceneChoiceItem`, `screenplayBlockItem`, `sceneTemplate`, `sceneScript`, `audioDeckTemplate`, `audioDeckScript`, `confirmTemplate`, `appShellScript` (`cmd/wasmplayer/main.go:22–117`) — identyczne z tymi w `domain/views/*.go`.
  2. **Zduplikowane konstruktory widoków**: `buildConfirmChunk`, `buildPlayerChunk`, `buildWasmSceneChunk`, `buildWasmScreenplayBlocks`, `findSceneByID` (`cmd/wasmplayer/main.go:545–794`) — kopiują logikę `ConfirmView`, `PlayerView`, `SceneView`, `AudioDeckView` z `domain/views/` i `domain/book/`.
  3. **Zduplikowany dispatcher akcji (`wasmAction`, linie `235–525`)**: ręczny `switch action` obsługujący `SaveAudioTime`, `ChooseBranch`, `AutoAdvanceImplicit`, `AudioEnded`, `SetAutoChoiceDelay`, `CycleAudioSpeed`, `ToggleAutoChoice`, `PromptRewindStep`, `PromptResetProgress`, `ConfirmModalClose`, `ConfirmModalAccept`, `RevealHistory`, `FocusHistoryStep`, `FocusPresent` oraz `executeChoiceTransition` — dokładna kopia logiki z `domain/actions/player_actions.go` i `confirm_actions.go`.
* **Rozwiązanie**:
  Usunąć `//go:build !(js && wasm)` z widoków i czystych akcji odtwarzacza w `domain/` i wywoływać bezpośrednio `domain/state`, `domain/views` oraz `domain/actions` z `cmd/wasmplayer/main.go`.

---

### 2. Duplikacja logiki i modeli pomiędzy `EditorContextView` (prawy inspektor) a `TextEditorView` (pełny edytor sceny)
* **Lokalizacja**:
  * Go Views: `domain/views/editor_context_view.go:37–829` vs `domain/views/text_editor_view.go:40–519`
  * Templates: `app/templates/editor_context_view.tpl:105–362` vs `app/templates/text_editor_view.tpl:231–518`
  * JS Controllers: `app/js/controllers/editor_context_controller.js` vs `app/js/controllers/text_editor_controller.js`
* **Problem**:
  1. **Zduplikowane pola w `textEditorTemplate`**: `TextEditorView` (`text_editor_view.go:204`) wywołuje `nv := buildEditorNodeView(s, modalSc, true)`, ale w liniach `222–275` przepisuje 29 pól z `nv` do płaskich pól `SceneTextModal*`, podczas gdy `editorContextTemplate` wystawia bezpośrednio `SelectedNode *book.EditorNodeView`.
  2. **Zduplikowane budowanie i walidacja wyborów**: `buildEditorNodeView` (`editor_context_view.go:403–428`) vs `TextEditorView` (`text_editor_view.go:404–447`).
   3. **Zduplikowane bloki szablonów `.tpl` (ponad 240 linii identycznego markupu)**:
      * Stos edycji wyborów `.choice-triple-stack` (`exit:`, `text:`, `if:`, `set:` + ukryte inputy) (`editor_context_view.tpl:256–324` vs `text_editor_view.tpl:407–475`).
      * Pasek nagrywarki głosu `.voice-rec-strip` (`editor_context_view.tpl:183–227` vs `text_editor_view.tpl:242–286`).
      * Dropdown autouzupełniania `.syn-autocomplete-dropdown` (`editor_context_view.tpl:330–358` vs `text_editor_view.tpl:491–518`).
      * Pigułki slugline `.slugline-helper-pills` (`editor_context_view.tpl:105–115` vs `text_editor_view.tpl:307–317`).
  4. **Zduplikowane metody i przekierowania w JS (`TextEditorController` <-> `EditorContextController`)**:
     * `autoResizeMeta` w `text_editor_controller.js:158–163` vs `autoResize` w `editor_context_controller.js:211–216`.
     * `selectNodeChapter`/`selectNodeImage`/`selectNodeVoice` (`editor_context_controller.js:177–187`) vs `selectModalChapter`/`selectModalImage`/`selectModalVoice` (`text_editor_controller.js:219–229`).
     * Metody pass-through w `TextEditorController` (`initTextarea`, `onRuleInput`, `onRuleKeydown`, `onRuleBlur`, `chooseAutocompleteItem`).
* **Rozwiązanie**:
  Wystawić `SelectedNode *book.EditorNodeView` bezpośrednio w `textEditorTemplate`, ujednolicić budowanie `InspectorPorts` (z zachowaniem zmiennej pętli `{for ch in SelectedNode.InspectorPorts}` wymaganej przez `@scene_choice_arrow`), wydzielić wspólne szablony cząstkowe i przenieść obsługę wspólnych kontrolek formularza sceny do `EditorContextController`.

---

### 3. Potrójna duplikacja zapisu formularza sceny w `domain/actions/` (`saveSceneTextModal` vs `flushSceneTextChanges` vs `saveSceneFromFormWithMode`)
* **Lokalizacja**:
  * `domain/actions/text_editor_actions.go:87–111` (`flushSceneTextChanges`)
  * `domain/actions/text_editor_actions.go:157–177` (`saveSceneTextModal`)
  * `domain/actions/studio_scene_actions.go:775–833` (`saveSceneFromForm`, `saveSceneFromFormWithMode`)
* **Problem**:
  Trzy ścieżki powtarzają wyszukiwanie sceny, `EnsureBaseline`, `applySceneFormFields`, śledzenie `LastRenamedFromSceneID`, zapis na dysk + `reloadActiveBook`, aktualizację `SelectedSceneID` / `SceneText.SyncSceneID` oraz `RecordStep`. Ponadto `saveSceneFromForm` (`775–777`) jest 1-liniowym pass-through.
* **Rozwiązanie**:
  Zunifikować mutację i zapis formularza sceny w jednej funkcji `commitSceneForm(appState, request, mode)` w `domain/actions/studio_scene_actions.go`.

---

### 4. Duplikacja rozstrzygania wyjść ze sceny i wyszukiwania domyślnej sceny w `domain/`
* **Lokalizacja**:
  1. Rozstrzyganie wyborów sceny (`EvaluateCondition`, `IsBranching`, `IsTerminal`):
     * `domain/views/scene_view.go:69–88`
     * `domain/views/audio_deck_view.go:52`
     * `domain/actions/player_actions.go:80–110`
     * `domain/views/text_editor_view.go:404–447`
  2. Wyszukiwanie poprawnej sceny z fallbackiem do `StartSceneID` i `Scenes[0]`:
     * `domain/state/studio_state.go:112–116`, `187–192`
     * `domain/state/player_state.go:45–50`, `76–81`
     * `domain/views/studio_view.go:216–220`
     * `domain/views/editor_context_view.go:605–613`, `819–823`
     * `domain/actions/studio_scene_actions.go:1014–1017`
* **Rozwiązanie**:
  Dodać kanoniczne metody `(targetBook *Book) ResolveValidSceneID(preferredID string) string` oraz `(targetBook *Book) EvaluateSceneChoices(targetScene *Scene, variables map[string]string, defaultNextLabel string) SceneChoicesEvaluation` (zwracającą strukturę zamiast `>= 4` wartości, zgodnie z `ARCH_SIGNATURE_BLOAT`).

---

### 5. Duplikacja usuwania pojedynczej sceny vs wielu scen (`DeleteScene` vs `DeleteScenes`)
* **Lokalizacja**:
  * `domain/actions/studio_scene_actions.go:948–1009` (`deleteSceneFromForm` vs `deleteScenesFromForm`, `deleteSceneByID` vs `deleteScenesByIDs`)
  * `domain/actions/confirm_actions.go:25–26, 49–50, 138–141, 201–280` (`pendingDeleteScene` vs `pendingDeleteScenes`, `promptDeleteScene` vs `promptDeleteScenesFromForm`)
* **Problem**:
  `delete_scene` to przypadek szczególny `delete_scenes` dla 1 elementu, ale utrzymywane są osobne funkcje, stałe i gałęzie `switch`.
* **Rozwiązanie**:
  Obsługiwać usuwanie 1..N scen jedną ścieżką (`deleteScenesByIDs` i `promptDeleteScenesFromForm`).

---

### 6. Duplikacja metod-przelotek (Pass-Through Functions — zakazanych w `AGENTS.md` §0 i `ARCH.md` §5.6)
* **Lokalizacja**:
  1. `app/js/controllers/studio_controller.js:187–201`:
     * `zoomIn() { NodeCanvasController.zoomIn(); }`
     * `zoomOut() { NodeCanvasController.zoomOut(); }`
     * `fitAllNodes() { NodeCanvasController.fitAllNodes(); }`
  2. `app/js/controllers/new_book_controller.js:47–55`:
     * `selectRadio(p, el) { if (!el) return; StudioController.selectRadio(p, el); }`
     * `toggleFormBtn(p, el) { if (!el || el.disabled) return; StudioController.toggleFormBtn(p, el); }`
  3. `app/js/controllers/text_editor_controller.js:146–148, 185–202`:
     * `initTextarea(_params, ta) { this.autoResize(ta); }`
     * `onRuleInput`, `onRuleKeydown`, `onRuleBlur`, `chooseAutocompleteItem` -> delegują do `EditorContextController`.
  4. `domain/actions/studio_scene_actions.go:775–777`:
     * `saveSceneFromForm(s, r)` -> 1-liniowy pass-through do `saveSceneFromFormWithMode(s, r, false)`.

---

### 7. Duplikacja logiki pływających inputów (`Chapter` / `NodeTitle` / `ChoiceLabel`) i hydratacji (`initCanvas` / `onHydrate`) w `NodeCanvasController`
* **Lokalizacja**:
  * `app/js/controllers/node_canvas_controller.js:401–584` (15 metod pływających inputów: 3 zestawy po 5 metod)
  * `app/js/controllers/node_canvas_controller.js:1234–1326` (`initCanvas` vs `onHydrate`)
  * `app/js/node_canvas/node_canvas_model.js:32–34` vs `app/js/node_canvas/node_canvas_helpers.js:64–66` (zduplikowane `choiceRowY(idx)`)
  * `app/js/controllers/studio_controller.js:5–6` oraz `domain/views/studio_view.go:72–73` (zduplikowane pole `isHistoryPreview` oraz `historyPreview`)

---

### 8. Duplikacja reguł CSS (`.tss`) dla prawych paneli inspektora
* **Lokalizacja**:
  * `app/styles/editor_inspector.tss:8–59`
* **Problem**:
  Trzy osobne bloki reguł (`.editor-context-panel`, `.editor-history-panel, .editor-help-panel`, `.editor-breakdown-panel, .editor-variables-panel`) powtarzają 11 identycznych właściwości układu `320px` (zgłaszane w `audit_arch.go` jako `Duplicate CSS/TSS property block`).
* **Uwaga architektoniczna (`TPL_ARCH`)**:
  `internal/audit/audit.go:17` wymusza `maxTSSCommaSelectors = 4`. Zamiast łączyć 5 selektorów po przecinku, wspólną bazę należy zdefiniować na `.editor-workspace-body > aside` (spójnie z liniami `60, 66, 78` w tym samym pliku!).

---

## CZĘŚĆ II — HARDKODOWANIE I ZŁAMANIA ZASAD `ARCH.md` / `AGENTS.md`

### 1. Krytyczna dziura w audycie statycznym (`internal/audit/audit.go:107`) — zahardkodowane stare katalogi
* **Lokalizacja**:
  * `internal/audit/audit.go:107`
* **Problem**:
  W `loadWorkspaceIndex` zahardkodowano `walkDirs := []string{"internal", "cmd", "app", "www", "apps", "common"}` — brakuje `"engine"` i `"domain"`. Przez to cały kod Go w `engine/` i `domain/` był pomijany przez `audit.Run`, `audit.RunArch` i `audit.RunMagic`.

---

### 2. Złamanie zasady 1:1 Triad (`ARCH.md` §2) dla `EditorHelpView`
* **Lokalizacja**:
  * `domain/views/editor_help_view.go` + `app/templates/editor_help_view.tpl` + `app/js/controllers/editor_help_controller.js`
  * vs `app/templates/studio_view.tpl:76–98` (`aside.editor-help-panel#editor-help-panel`) i `domain/views/app_shell_view.go:143–145`
* **Problem**:
  W `studio_view.tpl:76–98` prawy panel pomocy jest zahardkodowany bezpośrednio w `studio_view.tpl`, a równolegle `EditorHelpView` jest dodawany do `ActiveOverlayViews` jako modal `#editor-help-view`. Ponadto `EditorHelpController` binduje `#editor-help-view .btn-close-editor-help`, więc przycisk zamknięcia w `#editor-help-panel` nie działa.

---

### 3. Złamanie `ARCH.md` §4.3 — State-Shuttling `data-*` w `.tpl`
* **Lokalizacja**:
  1. `app/templates/player_view.tpl:25`: `data-prefetch-urls: "{PrefetchMediaURLs}"` (powinno podróżować przez `playerScript.prefetchUrls` w `FDP`, a nie jako atrybut DOM na `#active-scene-stage`).
  2. `app/templates/editor_context_view.tpl:4–5` i `app/templates/text_editor_view.tpl:27–28`: `data-context-scene-id`, `data-book-vars`, `data-test-vars` (już obecne w `ScriptData`).
  3. `app/templates/studio_view.tpl:2, 122–129`: `data-history-preview` oraz 8 nieużywanych atrybutów `data-search-*` (wyszukiwanie działa po stronie Go przez `FilterStudioSearch`).
  4. `app/templates/editor_context_view.tpl:185–191` i `app/templates/text_editor_view.tpl:244–250`: przekazywanie stanu nagrywarki (`data-voice-open`, `data-voice-url`, `data-voice-file`, `data-trim-start`, `data-trim-end`) przez atrybuty DOM zamiast przez `ScriptData` (`editorContextScript` / `textEditorScript`).

---

### 4. Hardkodowane teksty i glify w `.tpl` oraz magic strings w `www/app.js`
* `app/templates/editor_context_view.tpl:84` i `text_editor_view.tpl:174`: zahardkodowany polski placeholder `"odkryta_prawda = true; sumienie++"` zamiast `{t_scene_effects_placeholder}`.
* `app/templates/editor_context_view.tpl:596` i `text_editor_view.tpl:47`: tekstowe znaki `"✕"` i `"↑"` zamiast ikon SVG (`app/icons/close.svg`, `@scene_choice_arrow`).
* `www/app.js:1–446`: zahardkodowane literały stringowe w ciałach funkcji (selektory, klasy, kolory, teksty UI).

---

### 5. Kruche testy oparte na zahardkodowanych ID DOM, atrybutach i selektorach CSS (`domain/*_test.go`)
* **Lokalizacja**:
  * `domain/server_test.go:534, 1310, 1727`
  * `domain/server_editor_test.go:494–518, 1578–1599, 1757, 1775`
  * `domain/server_js_runtime_test.go:1401–1735, 2066–2085`
* **Problem**:
  Testy w wielu miejscach sprawdzają surowe podciągi HTML/CSS/Go ze starymi lub zahardkodowanymi identyfikatorami (`id="editor-help-panel"`, `data-prefetch-urls="..."`, `data-trim-end="..."`, `".editor-history-panel,\n.editor-help-panel {"`, `case "SaveAudioTime":` w `cmd/wasmplayer/main.go`), przez co każda zmiana nazwy ID, konsolidacja CSS lub deduplikacja WASM zgodnie z `ARCH.md` psuje testy mimo poprawnego działania funkcji.
* **Rozwiązanie**:
  Oprzeć testy na **kontraktach `ARCH.md`**:
  1. Weryfikacji stanu domenowego (`srv.State`) oraz zdekodowanych ramek `FDP` (`frame.Get("controller")`, pól `ScriptData`, docelowego `target`).
  2. Weryfikacji semantycznych reguł CSS przez parser reguł zamiast porównywania surowych nagłówków selektorów.
  3. W testach runtime JS (`server_js_runtime_test.go`) — rozwiązywaniu elementów DOM poprzez selektory zadeklarowane w `<Controller>.bindings()` zamiast duplikowania zahardkodowanych `#id` w kodzie testu.
