# Plan Naprawczy Duplikacji, Hardkodowania i Odporności Testów (`ARCH.md` & `AGENTS.md`)

Plan wykonawczy krok po kroku przygotowany na podstawie `duplication_and_hardcoding_audit.md` i zweryfikowany pod kątem wszystkich bramek w `internal/audit/` (`JS_BINDINGS_LOGIC`, `JS_CONTROLLER_CONTRACT`, `JS_CONTROLLER_DATA_MAP`, `TPL_ARCH`, `ARCH_SIGNATURE_BLOAT`, `MAGIC_FUNC_STRING`, `TestClientServerActionAndOverlayContracts`).

---

## ZADANIE 1 (P0): Naprawa skanera audytu statycznego (`internal/audit/audit.go`)

### Co tam jest i jakie są problemy
* W `internal/audit/audit.go:107` funkcja `loadWorkspaceIndex` posiada zahardkodowaną listę katalogów sprzed wydzielenia `engine/` i `domain/`:
  `walkDirs := []string{"internal", "cmd", "app", "www", "apps", "common"}`
* Mimo że `isCoreProdGoPath` (`internal/audit/audit.go:351–355`) sprawdza prefiksy `"engine/"` oraz `"domain/"`, `filepath.WalkDir` nigdy nie wchodzi do tych katalogów.

### Rozwiązanie
* Dodać `"engine"` i `"domain"` do `walkDirs` w `loadWorkspaceIndex` i usunąć nieistniejące katalogi (`"apps"`, `"common"`).

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`internal/audit/audit.go`**:
   * **`loadWorkspaceIndex(rootDir string)` (linia `107`)**:
     * Zmień `walkDirs := []string{"internal", "cmd", "app", "www", "apps", "common"}` na:
       `walkDirs := []string{"engine", "domain", "internal", "cmd", "app", "www"}`

---

## ZADANIE 2 (P0): Eliminacja duplikacji w `cmd/wasmplayer/main.go` i ujednolicenie z `domain/`

### Co tam jest i jakie są problemy
* `cmd/wasmplayer/main.go` (`843` linie) duplikuje struktury `*Template`/`*Script`, konstruktory widoków (`PlayerView`, `SceneView`, `AudioDeckView`, `ConfirmView`) oraz akcje odtwarzacza z `domain/`.
* Pliki `domain/views/player_view.go`, `domain/views/scene_view.go`, `domain/views/audio_deck_view.go` mają zbędny tag `//go:build !(js && wasm)`.
* **Pułapka kompilacji WASM (`GOOS=js GOARCH=wasm` & `server_js_runtime_test.go:2088–2114`)**:
  1. `go list -deps ./cmd/wasmplayer` pod `js && wasm` **nie może** wciągać `net/http`, `compress/gzip`, `archive/zip`, `image/png`, `encoding/base64`, `encoding/xml`, `os/exec`.
  2. Jeśli `cmd/wasmplayer` zaimportuje `mojra/domain/views` oraz `mojra/domain/state`, wszystkie widoki nieużywane w WASM (`library_view.go`, `topbar_view.go`, `qr_view.go`, `add_book_view.go`, `new_book_view.go`, `export_view.go`, `studio_projects_view.go`, `editor_help_view.go`, `node_canvas_view.go` oraz desktopowa część `app_shell_view.go`) muszą mieć `//go:build !(js && wasm)`.
  3. `domain/state/studio_state.go` odwołuje się do `*book.BookHistory` z `domain/book/history.go` (który ma `//go:build !(js && wasm)` i importuje zakazane w WASM `compress/gzip`), więc `studio_state.go` również wymaga `//go:build !(js && wasm)` (z lekkim stubem `EditorState` dla `js && wasm` lub wydzieleniem definicji `BookHistory` bez `compress/gzip`).
  4. `domain/server_js_runtime_test.go:2066–2085` skanuje tekst `cmd/wasmplayer/main.go` w poszukiwaniu `case "<Action>":` — po przeniesieniu switcha do `domain/actions/player_standalone.go` test ten musi sprawdzać wspólny plik.

### Rozwiązanie
* Usunąć `//go:build !(js && wasm)` z `domain/views/player_view.go`, `domain/views/scene_view.go`, `domain/views/audio_deck_view.go` i oznaczyć widoki czysto desktopowe/studio tagiem `//go:build !(js && wasm)`.
* Wydzielić czystą domenową obsługę akcji odtwarzacza i potwierdzeń bez zależności od `net/http` (`domain/actions/player_standalone.go`) tak, aby `cmd/wasmplayer/main.go` operował na `appState *state.AppState` (`StandaloneMode: true`) i delegował renderowanie do `views.PlayerView(appState)`, `views.ConfirmView(appState)` oraz `views.AppShellScriptView(appState)`.

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`domain/views/player_view.go`, `domain/views/scene_view.go`, `domain/views/audio_deck_view.go`**:
   * **Linia `1`**: Usuń `//go:build !(js && wasm)`.
   * W `PlayerView(appState *state.AppState)` aktualizuj `scData.saveHex` przez `appState.Player.SyncSaveHex()`.
2. **`domain/state/player_state.go`**:
   * Dodaj metodę `(player *PlayerState) SyncSaveHex()`: kodującą `book.EncodeSaveStateBytes(player.Save)` do `player.SaveHex`.
3. **`domain/actions/player_standalone.go` (nowy plik wspólny bez `//go:build !(js && wasm)`)**:
   * Przenieś czystą logikę przejść i akcji odtwarzacza (`ExecutePlayerChoiceTransition`, `DispatchPlayerStandaloneAction(appState, actionName, form)`), współdzieloną przez HTTP handlery w `player_actions.go` / `confirm_actions.go` oraz `wasmAction` w `cmd/wasmplayer/main.go`.
4. **`cmd/wasmplayer/main.go`**:
   * **Usuń linie `22–131` oraz `468–794`** (wszystkie zduplikowane struktury `*Template`/`*Script`, `buildPlayerChunk`, `buildWasmSceneChunk`, `buildConfirmChunk`, `executeChoiceTransition`).
   * W `wasmBoot` i `wasmAction` używaj bezpośrednio `state.AppState`, `actions.DispatchPlayerStandaloneAction`, `views.PlayerView` oraz `views.ConfirmView`.

---

## ZADANIE 3 (P1): Unifikacja `EditorContextView` i `TextEditorView` (`domain/views/` & `app/templates/`)

### Co tam jest i jakie są problemy
* `domain/views/text_editor_view.go:204–275` wywołuje `nv := buildEditorNodeView(s, modalSc, true)`, a następnie przepisuje 29 pól z `nv` do płaskich pól `SceneTextModal*` w `textEditorTemplate`, podczas gdy `editorContextTemplate` wystawia `SelectedNode *book.EditorNodeView`.
* W efekcie `app/templates/editor_context_view.tpl` oraz `app/templates/text_editor_view.tpl` duplikują markup dla `.choice-triple-stack`, `.syn-autocomplete-dropdown` oraz `.slugline-helper-pills`.
* **Pułapka testowa (`TestClientServerActionAndOverlayContracts`)**: `app/templates/scene_choice_arrow.tpl` używa zmiennej `{ch.*}` (`{ch.Label}`, `{ch.IsBranch}`, `{ch.IsEditable}`, `{ch.IsEnding}`). Zgodnie z `server_js_runtime_test.go:130`, pętla wyborów w obu szablonach musi używać nazwy zmiennej `ch` (`{for ch in SelectedNode.InspectorPorts}`), aby uniknąć błędu `out-of-scope {for} loop variable`.

### Rozwiązanie
* W `textEditorTemplate` (`domain/views/text_editor_view.go`) zastąpić 29 zduplikowanych pól `SceneTextModal*` bezpośrednim polem `SelectedNode *book.EditorNodeView` oraz wspólnym `BookNarrated bool`.
* Ujednolicić budowanie `SelectedNode.InspectorPorts` (wraz z walidacją `exit:`, `if:`, `set:` oraz podglądem `PreviewMode`) w `domain/views/editor_context_view.go`.
* Wydzielić wspólne szablony cząstkowe w `app/templates/`:
  * `app/templates/choice_triple_stack.tpl` (operujący na zmiennej pętli `ch`)
  * `app/templates/syn_autocomplete_dropdown.tpl`
  * `app/templates/slugline_helper_pills.tpl`

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`domain/views/editor_context_view.go`**:
   * **`buildEditorNodeView` (linie `403–428`)**: Wydziel budowanie wyborów do `buildSceneInspectorChoices(appState *state.AppState, targetScene *book.Scene, previewMode state.ScenePreviewMode, testVars map[string]string) []book.Choice`.
2. **`domain/views/text_editor_view.go`**:
   * **`textEditorTemplate` (linie `40–116`)**: Dodaj `SelectedNode *book.EditorNodeView` oraz `BookNarrated bool`; usuń 29 redundantnych pól kopiowanych z `nv`.
   * **`TextEditorView` (linie `204–275, 404–447`)**: Przypisz `v.SelectedNode = nv`, `v.BookNarrated = bookNarrated` oraz `nv.InspectorPorts = buildSceneInspectorChoices(s, modalSc, previewMode, testState)`.
3. **`app/templates/choice_triple_stack.tpl` (nowy wspólny szablon cząstkowy)**:
   * Przenieś blok `.choice-triple-stack` z `editor_context_view.tpl:256–324` i `text_editor_view.tpl:407–475`, używając zmiennej pętli `ch` (`{for ch in SelectedNode.InspectorPorts}` w obu widokach).
4. **`app/templates/syn_autocomplete_dropdown.tpl` i `app/templates/slugline_helper_pills.tpl`**:
   * Przenieś zduplikowane bloki `.syn-autocomplete-dropdown` (`editor_context_view.tpl:330–358` / `text_editor_view.tpl:491–518`) oraz `.slugline-helper-pills` (`editor_context_view.tpl:105–115` / `text_editor_view.tpl:307–317`).

---

## ZADANIE 4 (P1): Unifikacja zapisu formularza sceny i usuwania scen w `domain/actions/`

### Co tam jest i jakie są problemy
1. **Potrójna ścieżka zapisu sceny**: `flushSceneTextChanges` (`text_editor_actions.go:87–111`), `saveSceneTextModal` (`text_editor_actions.go:157–177`) oraz `saveSceneFromFormWithMode` (`studio_scene_actions.go:779–833`), plus 1-liniowy pass-through `saveSceneFromForm` (`775–777`).
2. **Podwójna ścieżka usuwania scen**: `deleteSceneByID` vs `deleteScenesByIDs` (`studio_scene_actions.go:973–1009`) oraz `pendingDeleteScene` vs `pendingDeleteScenes` (`confirm_actions.go:25–26, 138–141, 201–280`).
3. **Ograniczenie `ARCH_SIGNATURE_BLOAT` (`internal/audit/audit_arch.go:1270`)**: Funkcja w Go nie może mieć `>= 6` parametrów, `>= 4` wartości zwracanych ani `>= 2` parametrów `bool`.

### Rozwiązanie
* Zdefiniować w `domain/actions/studio_scene_actions.go` strukturę opcji `sceneFormSaveSpec` (zamiast wielu flag `bool`) i jedną funkcję `commitSceneForm(appState *state.AppState, request *http.Request, spec sceneFormSaveSpec) (saved bool, toastMsg string)`.
* Usunąć `saveSceneFromForm`, `deleteSceneByID` oraz `pendingDeleteScene`, obsługując 1..N scen przez `deleteScenesByIDs`.

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`domain/actions/studio_scene_actions.go`**:
   * **Linie `775–833`**: Usuń pass-through `saveSceneFromForm` i zastąp `saveSceneFromFormWithMode` funkcją `commitSceneForm(appState *state.AppState, request *http.Request, spec sceneFormSaveSpec) (bool, string)`.
   * **Linie `948–1009`**: Usuń `deleteSceneByID` i `deleteSceneFromForm`; w `deleteScenesByIDs` obsługuj 1..N identyfikatorów, a w `registerActions` zarejestruj `"DeleteScene": deleteScenesFromForm`.
2. **`domain/actions/text_editor_actions.go`**:
   * **`flushSceneTextChanges` (`87–111`)** i **`saveSceneTextModal` (`157–177`)**: Wywołuj wspólne `commitSceneForm`.
3. **`domain/actions/confirm_actions.go`**:
   * Usuń `pendingDeleteScene` (`linia 25, 138–139`); zunifikuj `promptDeleteScene` i `promptDeleteScenesFromForm` (`linie 201–280`) tak, aby zawsze ustawiały `PendingAction: pendingDeleteScenes`.

---

## ZADANIE 5 (P1): Kanoniczne metody domenowe w `domain/book/` (`ResolveValidSceneID` oraz `EvaluateSceneChoices`)

### Co tam jest i jakie są problemy
1. Wyszukiwanie poprawnej sceny z fallbackiem do `StartSceneID` i `Scenes[0]` jest powielone w 6 plikach (`studio_state.go`, `player_state.go`, `studio_view.go`, `editor_context_view.go`, `studio_scene_actions.go`).
2. Filtrowanie wyborów sceny (`EvaluateCondition`, `IsBranching`, `IsTerminal`) jest powielone w `scene_view.go`, `audio_deck_view.go` i `player_actions.go`.
3. **Ograniczenie `ARCH_SIGNATURE_BLOAT`**: Metoda nie może zwracać 4 osobnych wartości (`numResults >= 4`).

### Rozwiązanie
* Dodać w `domain/book/book.go`:
  1. `(targetBook *Book) ResolveValidSceneID(preferredID string) string`
  2. Strukturę `SceneChoicesEval` (`Visible []Choice`, `Unlocked []Choice`, `UnlockedNonEnding []Choice`, `IsTerminal bool`, `IsBranching bool`) oraz metodę `(targetBook *Book) EvaluateSceneChoices(targetScene *Scene, variables map[string]string, defaultNextLabel string) SceneChoicesEval`.

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`domain/book/book.go`**:
   * Dodaj `ResolveValidSceneID` oraz `EvaluateSceneChoices` zwracające `SceneChoicesEval`.
2. **`domain/state/studio_state.go`, `domain/state/player_state.go`, `domain/views/studio_view.go`, `domain/views/editor_context_view.go`, `domain/actions/studio_scene_actions.go`**:
   * Zastąp ręczne warunki `if b.FindScene(...) == nil` wywołaniem `b.ResolveValidSceneID(...)`.
3. **`domain/views/scene_view.go`, `domain/views/audio_deck_view.go`, `domain/actions/player_actions.go`**:
   * Zastąp ręczne pętle po `sc.Choices` wywołaniem `b.EvaluateSceneChoices(...)`.

---

## ZADANIE 6 (P1): Naprawa triady 1:1 dla `EditorHelpView` (`ARCH.md` §2) oraz deduplikacja `.tss` prawych paneli zgodna z `TPL_ARCH`

### Co tam jest i jakie są problemy
* W `app/templates/studio_view.tpl:76–98` panel pomocy jest zahardkodowany bezpośrednio w szablonie Studio jako `aside.editor-help-panel#editor-help-panel`, a jednocześnie `domain/views/editor_help_view.go` + `app/templates/editor_help_view.tpl` renderują modal `#editor-help-view` w `ActiveOverlayViews` (`app_shell_view.go:143–145`).
* Przycisk `.btn-close-editor-help` w `studio_view.tpl:79` nie pasuje do selektora `#editor-help-view .btn-close-editor-help` w `EditorHelpController` i nie zamyka panelu.
* W `app/styles/editor_inspector.tss:8–59` trzy reguły powtarzają 11 właściwości układu `320px`.
* **Ograniczenie `TPL_ARCH` (`internal/audit/audit.go:17`)**: Reguła `.tss` może mieć maksymalnie 4 selektory po przecinku (`maxTSSCommaSelectors = 4`). Nie wolno łączyć 5 klas paneli po przecinku!

### Rozwiązanie
* Osadzić `EditorHelpView` w `studioTemplate` jako `@HelpPanel` (`aside.editor-help-panel#editor-help-view`) i usunąć go z `ActiveOverlayViews` w `app_shell_view.go`.
* W `app/styles/editor_inspector.tss` zdefiniować wspólne właściwości układu `320px` na pojedynczym selektorze `.editor-workspace-body > aside` (spójnie z liniami `60, 66, 78` w tym samym pliku!).

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`domain/views/editor_help_view.go` i `app/templates/editor_help_view.tpl`**:
   * Zmień `ShowEditorHelpModal` na `ShowEditorHelpPanel` i renderuj `aside.editor-help-panel#editor-help-view .hidden?: {!ShowEditorHelpPanel}`.
2. **`domain/views/studio_view.go` i `app/templates/studio_view.tpl`**:
   * Dodaj `HelpPanel templates.TemplateChunk` do `studioTemplate`, przypisz `HelpPanel: EditorHelpView(s)` i zastąp linie `76–98` w `studio_view.tpl` wywołaniem `@HelpPanel`.
3. **`domain/views/app_shell_view.go`**:
   * **Linie `143–145`**: Usuń dodawanie `EditorHelpView(s)` do `modals`.
4. **`app/js/controllers/editor_help_controller.js`**:
   * Pozostaw w `bindings()` wyłącznie `"#editor-help-view .btn-close-editor-help": { click: Dispatcher.action("CloseEditorHelpModal") }` i usuń `onBackdrop`.
5. **`app/styles/editor_inspector.tss`**:
   * **Linie `8–59`**: Przenieś wspólne 11 właściwości (`order 3`, `width 320px`, `min-width 320px`, `max-width 320px`, `height 100%`, `background --bg-obsidian`, `border-left 1px solid --border-subtle`, `display flex`, `flex-direction column`, `flex-shrink 0`, `z-index 35`) do reguły `.editor-workspace-body > aside` (1 selektor — zgodny z `maxTSSCommaSelectors = 4`), pozostawiając w klasach paneli jedynie różnice `overflow`.

---

## ZADANIE 7 (P1): Eliminacja duplikacji i Pass-Through Functions w JS (`app/js/`) w zgodzie z `JS_BINDINGS_LOGIC`

### Co tam jest i jakie są problemy
1. **Ograniczenie `JS_BINDINGS_LOGIC` (`ARCH.md` §5.2 & `audit_js_security.go:970–994`)**:
   * W `bindings()` dozwolone są **wyłącznie** `Dispatcher.action("ActionName")` lub `this.<methodName>`.
   * Krótkie 4-liniowe metody `onBackdrop` w kontrolerach modali są zgodne z kontraktem i **nie są** zgłaszane przez `audit_js_security.go` (`minWin = 25`), natomiast audyt zgłasza **Pass-Through Functions** (`ARCH_RAVIOLI_INDIRECTION`).
2. **Zakazane funkcje pass-through (`AGENTS.md` §0 & `ARCH_RAVIOLI_INDIRECTION`)**:
   * `StudioController` (`studio_controller.js:187–201`): `zoomIn`, `zoomOut`, `fitAllNodes`.
   * `NewBookController` (`new_book_controller.js:47–55`): `selectRadio`, `toggleFormBtn`.
   * `TextEditorController` (`text_editor_controller.js:146–148, 185–202`): `initTextarea`, `onRuleInput`, `onRuleKeydown`, `onRuleBlur`, `chooseAutocompleteItem`.
   * `TopbarController` / `LibraryController` / `PlayerController`: `switchToStudio`, `switchToLibrary`, `backToEditor`.
3. **15 zduplikowanych metod pływających inputów oraz duplikacja `initCanvas` / `onHydrate` w `NodeCanvasController` (`node_canvas_controller.js:401–584, 1234–1326`)**.
4. **Duplikacja `choiceRowY(idx)` (`node_canvas_model.js:32–34`) oraz pola `historyPreview` (`studio_controller.js:5–6`, `studio_view.go:72–73`)**.
5. **Duplikacja metod zmiany nazwy w `EditorBreakdownController` (`editor_breakdown_controller.js:40–58`)**.

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`app/js/controllers/studio_controller.js` i `app/js/controllers/node_canvas_controller.js`**:
   * Usuń zduplikowane pole `historyPreview` ze `StudioController.data` i `studioScript` (`studio_view.go`), zostawiając `isHistoryPreview`.
   * Przenieś bindingi `[data-canvas-zoom="out"]`, `[data-canvas-zoom="in"]`, `[data-canvas-zoom="fit"]`, `#btn-zoom-reset` do `NodeCanvasController.bindings()` i usuń metody pass-through `zoomIn`, `zoomOut`, `fitAllNodes`, `resetToFirstNode` ze `StudioController`.
2. **`app/js/controllers/new_book_controller.js` i `app/js/controllers/studio_controller.js`**:
   * W `StudioController.bindings()` rozszerz selektory radio i toggle na `.form-radio-option[data-radio-option]` i `.form-toggle-btn[data-toggle-btn]`. Usuń metody pass-through `selectRadio` i `toggleFormBtn` z `NewBookController`.
3. **`app/js/controllers/topbar_controller.js`, `library_controller.js`, `player_controller.js` + `.tpl`**:
   * Dodaj `data-view: "studio"` / `data-view: "library"` na przyciskach w `topbar_view.tpl`, `library_view.tpl`, `player_view.tpl`, użyj `Dispatcher.action("SwitchView")` w `bindings()` i usuń `switchToStudio`, `switchToLibrary`, `backToEditor`.
4. **`app/js/controllers/text_editor_controller.js` i `app/js/controllers/editor_context_controller.js`**:
   * Usuń `initTextarea`; w `bindings()` dla `#modal-scene-textarea` ustaw `init: this.autoResize` i połącz `autoResize` z `autoResizeMeta`.
   * Przenieś obsługę `.syn-autocomplete-dropdown` oraz inputów reguł (`.syn-inner-input, .choice-exit-input, .choice-rule-input`) do `EditorContextController.bindings()` i usuń metody pass-through `onRuleInput`, `onRuleKeydown`, `onRuleBlur`, `chooseAutocompleteItem` z `TextEditorController`.
5. **`app/js/controllers/node_canvas_controller.js` i `app/js/node_canvas/node_canvas_model.js`**:
   * Zastąp 15 metod pływających inputów (`linie 401–584`) 5 metodami opartymi o słownik `this.settings.floatingInputs` (`openFloatingEditor`, `onFloatingEditorInput`, `commitFloatingEditor`, `hideFloatingEditor`, `onFloatingEditorKeydown`).
   * Wydziel wspólną metodę `syncCanvasGraphAndCamera(forceReset)` dla `initCanvas` i `onHydrate` (`linie 1234–1326`).
   * Usuń duplikat `choiceRowY` z `node_canvas_model.js:32–34` (używaj `NodeCanvasHelpers.choiceRowY`).
6. **`app/js/controllers/editor_breakdown_controller.js` i `app/templates/editor_breakdown_view.tpl`**:
   * Dodaj `data-old-value` na `<form>` w `editor_breakdown_view.tpl` i połącz `submitCharRename` + `renameCharacter` oraz `submitLocRename` + `renameLocation`.

---

## ZADANIE 8 (P2): Usunięcie state-shuttling `data-*` z `.tpl` i przeniesienie `prefetchUrls` oraz stanu głosu do `ScriptData` (`ARCH.md` §3 & §4)

### Co tam jest i jakie są problemy
1. `player_view.tpl:25` umieszcza `data-prefetch-urls: "{PrefetchMediaURLs}"` w DOM — wbrew `ARCH.md` §4.3 (`No State-Shuttling data-* Attributes`).
2. `editor_context_view.tpl:4–5`, `text_editor_view.tpl:27–28` oraz `studio_view.tpl:2, 122–129` umieszczają w DOM nieużywane atrybuty `data-context-scene-id`, `data-book-vars`, `data-test-vars`, `data-history-preview`, `data-search-*`.
3. `.voice-rec-strip` (`editor_context_view.tpl:185–191` i `text_editor_view.tpl:244–250`) przekazuje stan nagrywarki (`data-voice-open`, `data-voice-url`, `data-voice-file`, `data-trim-start`, `data-trim-end`) przez atrybuty DOM zamiast przez `FDP` (`ScriptData`).
4. **Uwaga architektoniczna (`1:1 Triad`)**: `app/js/editor_voice.js` jest modułem pomocniczym audio (jak `editor_voice_waveform.js`), a nie osobnym widokiem `.tpl`, dlatego pozostaje w `app/js/editor_voice.js` (nie w `app/js/controllers/`), natomiast dane stanu głosu otrzymuje z `EditorContextController.data` / `TextEditorController.data` (hydratowanych przez `editorContextScript` / `textEditorScript`).

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`domain/views/player_view.go`, `app/templates/player_view.tpl`, `app/js/controllers/player_controller.js`**:
   * Przenieś `prefetchUrls string` z `playerTemplate.PrefetchMediaURLs` (i `data-prefetch-urls` w `player_view.tpl:25`) do `playerScript.prefetchUrls` oraz `PlayerController.data.prefetchUrls: ""`.
2. **`domain/views/editor_context_view.go`, `domain/views/text_editor_view.go`, `app/templates/editor_context_view.tpl`, `app/templates/text_editor_view.tpl`**:
   * Dodaj pola stanu nagrywarki (`voiceOpen bool`, `voiceSection string`, `voiceUrl string`, `voiceTrimStart float64`, `voiceTrimEnd float64`) do `editorContextScript` i `textEditorScript` oraz odpowiadających im `EditorContextController.data` i `TextEditorController.data`.
   * Usuń `data-voice-open`, `data-voice-url`, `data-voice-file`, `data-trim-start`, `data-trim-end`, `data-context-scene-id`, `data-book-vars`, `data-test-vars` z `.tpl`.
   * Zastąp zahardkodowany placeholder `"odkryta_prawda = true; sumienie++"` kluczem `{t_scene_effects_placeholder}`, a znaki `"✕"` (`editor_context_view.tpl:596`) i `"↑"` (`text_editor_view.tpl:47`) ikonami SVG.
3. **`app/templates/studio_view.tpl`, `domain/views/studio_view.go`, `domain/book/models.go`**:
   * Usuń `data-history-preview` oraz 8 nieużywanych atrybutów `data-search-*` z `studio_view.tpl` i powiązane pola z `StudioSearchItem`.

---

## ZADANIE 9 (P2): Czyszczenie magicznych stringów w `domain/`, `engine/` i `www/app.js` (`AGENTS.md` §0 & `RunMagic`)

### Co tam jest i jakie są problemy
* Po włączeniu `"engine"` i `"domain"` w Zadaniu 1 skaner `RunMagic` (`internal/audit/audit_magic.go`) sprawdza literały stringowe wewnątrz ciał funkcji Go i JS (poza `bindings()`).
* W `www/app.js` oraz części funkcji w `domain/` i `engine/` znajdują się literały stringowe bezpośrednio w ciałach funkcji.

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`domain/views/*.go`, `domain/actions/*.go`, `domain/book/*.go`, `engine/*.go`**:
   * Wyciągnij wszystkie literały stringowe dłuższe niż 1 znak z ciał funkcji do bloków `const (...)` na początku odpowiednich plików (zaraz po `import`, zgodnie z `ARCH_MONOLITH_AND_NESTING` w `audit_arch.go:1458`).
   * Zastąp `" [⚠️ brak pliku]"` w `editor_context_view.go` kluczem słownika `i18n.TrMissingFileSuffix`.
2. **`www/app.js`**:
   * Wydziel słownik stałych `landingConfig` na początku pliku i zastąp wszystkie literały stringowe w ciałach funkcji odwołaniami do `landingConfig`.

---

## ZADANIE 10 (P1): Uodpornienie testów na zmiany ID DOM i struktury CSS (`domain/*_test.go`)

### Co tam jest i jakie są problemy
* Część testów w `domain/server_test.go`, `domain/server_editor_test.go` i `domain/server_js_runtime_test.go` testuje starą architekturę poprzez szukanie zahardkodowanych podciągów w surowym HTML/CSS (`id="editor-help-panel"`, `data-prefetch-urls="..."`, `".editor-history-panel,\n.editor-help-panel {"`).
* Przez to każda zmiana identyfikatora DOM lub konsolidacja reguł `.tss` wymaga ręcznego przepisywania testów.

### Rozwiązanie — Jak pisać testy odporne na zmiany ID i selektorów:
1. **Asercje na kontraktach `FDP` i `state.AppState` zamiast surowych atrybutów HTML**:
   * Zamiast szukać `data-prefetch-urls="..."` w HTML, dekoduj ramki `FDP` (`frames := engine.DecodeFDPFrames(raw)`) i sprawdzaj pole kontraktu `ScriptData` (`meta.Get("prefetchUrls")`).
   * Zamiast szukać zahardkodowanego `id="editor-help-panel"`, sprawdzaj stan domenowy (`srv.State.Editor.RightPanel == state.PanelHelp`) oraz obecność przetłumaczonego tytułu sekcji (`engine.T(lang, i18n.TrHelpPanelTitle)`) w wyrenderowanym HTML.
2. **Semantyczny parser reguł CSS w testach zamiast dopasowywania surowych nagłówków selektorów**:
   * W `domain/server_test.go` dodaj helper `findCSSRuleProps(cssText, selectorToken string) string`, który wyszukuje dowolną regułę CSS obejmującą dany selektor (np. `.editor-workspace-body > aside` lub `.topbar`) i sprawdza właściwości (`width: 320px`, `grid-row: 1`), niezależnie od tego, czy reguły zostały połączone.
3. **Rozwiązywanie selektorów przez `<Controller>.bindings()` w `TestClientJSRuntimeAndInteractiveResponsiveness` (`domain/server_js_runtime_test.go`)**:
   * W środowisku VM Node (`server_js_runtime_test.go`) wszystkie kontrolery są zarejestrowane w `Dispatcher.controllers`.
   * Dodaj w harnessie JS funkcję pomocniczą `findBoundElement(doc, actionOrMethodName)`, która znajduje selektor bezpośrednio w `Dispatcher.controllers[...].bindings()` i zwraca dopasowany element DOM. Dzięki temu zmiana `#id` lub `.class` jednocześnie w `.tpl` i `_controller.js` nie psuje żadnego testu E2E!

### Zmiany plik po pliku, metoda po metodzie, linia po linii
1. **`domain/server_test.go`**:
   * **Linie `534, 1310`**: Zastąp sprawdzanie `id="editor-help-panel"` asercją na `srv.State.Editor.RightPanel == state.PanelHelp` oraz widocznością zawartości pomocy (`!strings.Contains(helpChunk, "editor-help-panel hidden")`).
   * **Linie `1727–1739`**: Zastąp pętlę po 3 zahardkodowanych nagłówkach CSS sprawdzeniem reguły `.editor-workspace-body > aside` (`width: 320px`, brak `width: 100%` poza `@media` portrait).
2. **`domain/server_editor_test.go`**:
   * **`TestPlayerSequentialTwoHopPrefetchAndFullAudioPreload` (linie `480–519`)**: Użyj `decodeFDPResponse(openBytes)` i sprawdzaj pole kontraktu `openMeta.Get("prefetchUrls")` zamiast atrybutu DOM `data-prefetch-urls`.
   * **Linie `1757, 1775`**: Zastąp sprawdzanie `data-trim-start` / `data-trim-end` / `data-voice-file` w HTML asercją na polach `ScriptData` (`voiceTrimStart`, `voiceTrimEnd`, `voiceUrl`) zdekodowanej ramki `FDP`.
3. **`domain/server_js_runtime_test.go`**:
   * **Linie `1541, 1557, 1566`**: Zamiast odczytywać `getAttribute("data-trim-end")` i `getAttribute("data-voice-file")` z DOM, sprawdzaj `ctx.EditorContextController.data.voiceTrimEnd` oraz `ctx.EditorContextController.data.voiceUrl`.
   * **Linie `2066–2085`**: Sprawdzaj obecność `case "<Action>":` w `domain/actions/player_standalone.go` zamiast w `cmd/wasmplayer/main.go`.
   * Dodaj w harnessie VM helper `queryByAction(doc, actionName)` / `queryByControllerBinding(doc, ctrlName, methodName)` korzystający z `Dispatcher.controllers` i zastąp sztywne selektory tam, gdzie test weryfikuje akcję kontrolera.
