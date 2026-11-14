# Plan naprawczy architektury zgodnie z `@ARCH.md` i `@AGENTS.md` (`arch_remediation_plan.md`)

## 1. Co tam jest i jakie są problemy (Diagnoza stanu obecnego)

Po wdrożeniu kodeka `FDP` (`internal/tpl/fdp.go` + `app/js/fdp.js`), 19 kontrolerów w `app/js/controllers/*_controller.js` oraz wydzieleniu `actions_editor_voice.go` i `mutator_scene.go`, statyczny audyt zwraca `0` błędów (`MOJRA STATIC CODE AUDIT: 0`), a wszystkie kontrolery w `app/js/` mają `0` magic stringów. Do pełnej zgodności z `@ARCH.md` i `@AGENTS.md` pozostało **6 konkretnych obszarów długu architektonicznego i duplikacji**:

1. **Martwy kod atrybutowy z ery `WFP` w `app/js/dispatcher.js` (naruszenie `ARCH.md §4.1` i `§5`)**:
   - Szablony `app/templates/*.tpl` nie używają już `action:`, `kind:`, `target:`, `event-*` ani `param-*`, ale `Dispatcher` nadal utrzymuje martwe ścieżki dla atrybutów HTML:
     - `Dispatcher.modules.backend` (`Proxy`, `L74–78`) i wywołanie `this.modules[hit.kind][hit.action]` (`L384–386`).
     - Gałęzie `event-<type>-action` i `[action]` w `Dispatcher.resolve(e)` (`L203–227`, `L251–265`).
     - Pętle `param-*` i `event-*-param-*` w `Dispatcher.collectParams(hit, e)` (`L284–300`, `L312–314`).
   - Dodatkowo `Dispatcher.applyResponse` (`L349`) przekazuje `this.swapTarget(t, decoded.html || "")` bez flagi `skipInit` (`decoded.keepScroll === "true"`), przez co `swapTarget` zawsze uruchamia `init` nawet przy `keepScroll=true` (niezgodnie z `ARCH.md §5 pkt 5` i `§6 pkt 3`).
   - `Dispatcher.action(actionName, mapFn)` (`L107–113`) nadal przyjmuje wycofany argument `mapFn` zakazany przez `ARCH.md §5 pkt 2 (JS_BINDINGS_LOGIC)`.

2. **Zduplikowane parsowanie formularza sceny i logika domenowa w `internal/actions/actions_editor_scene.go` (1125 LOC > 1000 LOC, ostrzeżenia `ARCH_MONOLITH_AND_NESTING` i `ARCH_STRING_HACK`)**:
   - `hasSceneTextModalChanges` (`L186–306`) oraz `applySceneFormFields` + `applySceneFormChoices` (`L308–593`) nadal **dwukrotnie** parsują, sanityzują i porównują te same pola formularza (`text`, `name`, `explicit_id`, `slugline`, `synopsis`, `scene_effects`, `chapter`, `explicit_image`, `explicit_voice`, `choice_label_*`, `choice_target_*`, `choice_cond_*`, `choice_effects_*`).
   - Funkcje czysto domenowe operujące na `*book.Book` i `*book.Scene` (`splitInlineChoiceLabelAndTarget` `L111–119`, `resolveEditedChoiceTarget` `L466–502`, `applySluglineToken` `L1033–1063`, `insertScreenplaySnippet` `L1075–1123`) leżą w warstwie HTTP (`internal/actions/`) zamiast w `internal/book/mutator_scene.go`.

3. **Zahardkodowane warunki `if act == ...` w `internal/actions/dispatcher.go` (`L180–235`, `L311–321`, `L336`)**:
   - `resolveActionChunks` zawiera rozgałęzienia po nazwach akcji (`if act == ConfirmModalAccept`, `if act == ToggleImageless`, `if act == OpenSceneTextModal`, `if act == AddBookVariable`, `if act == AddChoice`, `if act == CloseSceneTextModal`), a `HandleAction` sprawdza `if act == SaveAudioTime || act == StartWindowDrag`.
   - Zgodnie z `ARCH.md §1 pkt 2` wybór celu DOM (`target`) i chunków widoku powinien wynikać deklaratywnie z pól `ActionSpec` oraz różnicy stanu `before` vs `s` (zero `if act == ...` w dispatcherze).
   - `isFDPRequestPayload` (`L71–72`) nadal utrzymuje fallback na usunięty nagłówek `text/x-wfp`.

4. **Duplikacja budowania danych sceny w `internal/views/` (~160 LOC między `view_editor_context_panel.go`, `view_text_editor.go` i `view_studio.go`)**:
   - `ViewTextEditor` (`internal/views/view_text_editor.go:184–238`) kopiuje ręcznie 28 pól z `nv := buildEditorNodeView(s, modalSc, true)` do płaskich pól `SceneTextModal*` w `textEditorTemplate`, zamiast osadzić `Node *book.EditorNodeView` bezpośrednio w `textEditorTemplate` (zgodnie z `ARCH.md §1 pkt 3`: *„reuse domain structs from `internal/book` directly”*).
   - `populateEditorBookAndChapterSettings` (`view_editor_context_panel.go:715–724`) i `ViewTextEditor` (`view_text_editor.go:229–238`) duplikują budowanie słowników zmiennych/celów (`BookVarsSpec`, `BookVarSuggestions`, `BookEnumSuggestions`, `BookTargetSuggestions`, `SyntaxHelpCondRows`).
   - `populateEditorCommonFields` (`view_editor_context_panel.go:86–108`) i `ViewStudio` (`view_studio.go:156–178`) duplikują wyliczanie stanu `IsHistoryPreview` / `HistoryPreviewID` / `HistoryPreviewLabel` oraz placeholderów trybu scenariuszowego.
   - `buildSceneMediaOptions` (`view_editor_context_panel.go:211`, 6 wartości zwracanych — `ARCH_SIGNATURE_BLOAT`) zwraca krotkę 6 skalarów/slice'ów zamiast spójnej struktury domenowej `book.SceneMediaOptions`.

5. **Sprzężenie zwrotne między `TextEditorController` a `EditorContextPanelController` oraz nadmiarowe pliki `app/js/editor_voice.js` (843 LOC) i `app/js/editor_webgl.js` (773 LOC)**:
   - `TextEditorController` (`app/js/controllers/text_editor_controller.js:158–178`) wywołuje bezpośrednio metody `EditorContextPanelController.onRuleInput/onRuleKeydown/onRuleBlur/applyAutocompleteItem`, przekazując własny element DOM `#modal-syn-autocomplete-dropdown`.
   - `app/js/editor_voice.js` (843 LOC > 600 LOC) i `app/js/editor_webgl.js` (773 LOC > 600 LOC) przekraczają limit 600 LOC (`ARCH_MONOLITH_AND_NESTING`) i nie są jeszcze w pełni zintegrowane z konwencją `controllers/`.
   - `www/app.js` posiada 17 magic stringów w `frame()`, `openWasmStage()`, `closeWasmStage()`, `openBetaModal()`, `closeBetaModal()` oraz jednowierszowy wrapper `buildStoryGraph()`.

6. **Higiena repozytorium i stałe w środku plików (`AGENTS.md §0, §6` oraz `ARCH_MONOLITH_AND_NESTING`)**:
   - W katalogu głównym repozytorium leży 11 tymczasowych plików roboczych `*.md` z poprzednich etapów (`actions_modal_scene_text_refactor.md`, `deck_refactor.md`, `declarative_controller_architecture.md`, `editor_gestures_refactor.md`, `fdp_actions_refactor.md`, `js_dispatcher_refactor.md`, `js_refactor.md`, `plan_gnerator.md`, `refactor_actions_modal_confirm.md`, `refactor_cmd_mojra.md`, `runtime_icon_removal.md`, `set_device_mode_refactor.md`, `tss_refactor.md`, `voice_rerecord_fix.md`).
   - 13 plików Go zawiera deklaracje `const` w środku pliku zamiast na górze tuż po `import` (`internal/actions/actions_editor.go:507`, `internal/actions/actions_modal_new_book.go:35`, `internal/views/view_app_shell.go:49`, `internal/tpl/tpl_render.go:206` itd.).

---

## 2. Rozwiązania (Architektura docelowa wg `@ARCH.md`)

1. **Czysty `Dispatcher` w `app/js/dispatcher.js` (~210 LOC zamiast 398 LOC)**:
   - Usunięcie martwej obsługi atrybutów `action`, `kind`, `target`, `event-*`, `param-*` i `modules.backend`.
   - `Dispatcher.action(actionName)` przyjmuje wyłącznie nazwę akcji (1 parametr), zgodnie z `ARCH.md §5 pkt 2`.
   - `Dispatcher.swapTarget(targetSel, html, skipInit)` przekazuje `skipInit` (`decoded.keepScroll === true` zhydratowane lub z ramki `FDP`) do `this.mountAll(targetEl, skipInit)`.
2. **Jednoprzebiegowy mutator formularza sceny i odchudzenie `internal/actions/actions_editor_scene.go` (< 550 LOC)**:
   - Połączenie `hasSceneTextModalChanges` i `applySceneFormFields` w jedną funkcję `applySceneFormFields(s *state.AppState, target *book.Scene, r *http.Request) (changed bool)`.
   - Przeniesienie `SplitInlineChoiceLabelAndTarget`, `ResolveEditedChoiceTarget`, `ApplySceneChoiceUpdates`, `ApplySceneSluglineToken`, `InsertSceneScreenplayBlock` do `internal/book/mutator_scene.go`.
3. **Deklaratywny `internal/actions/dispatcher.go` bez gałęzi `if act == ...`**:
   - Rozszerzenie `ActionSpec` o flagi zachowania (`ImmediateNoContent bool`, `RefreshPlayerOnImageless bool`, `EscalateAppWhenFormScene bool`), co pozwala usunąć wszystkie `if act == ...` z `resolveActionChunks` i `HandleAction` oraz usunąć martwy `wfpMimePrefix`.
4. **Deduplikacja `internal/views/` z użyciem struktur domenowych `internal/book`**:
   - Osadzenie `Node *book.EditorNodeView` w `textEditorTemplate` (`internal/views/view_text_editor.go` + `app/templates/text_editor.tpl`), co eliminuje 28 zduplikowanych pól `SceneTextModal*`.
   - Zastąpienie 6-wartościowej sygnatury `buildSceneMediaOptions` strukturą `book.SceneMediaOptions`.
   - Wydzielenie wspólnego wypełniania sugestii zmiennych (`populateBookVariableSuggestions`) i stanu podglądu historii (` resolveHistoryPreviewBanner`).
5. **Modularyzacja `app/js/editor_voice.js`, `app/js/editor_webgl.js` i oczyszczenie `www/app.js`**:
   - Podział `app/js/editor_voice.js` (843 LOC) na `app/js/editor_voice.js` (< 480 LOC) i `app/js/editor_voice_trim.js` (~360 LOC).
   - Podział `app/js/editor_webgl.js` (773 LOC) na `app/js/editor_webgl.js` (< 450 LOC) i `app/js/editor_webgl_shaders.js` (~320 LOC).
   - Przeniesienie 17 magic stringów z funkcji w `www/app.js` do obiektu `settings` na górze pliku i inlinowanie `buildStoryGraph()`.
6. **Porządki w stałych Go i usunięcie tymczasowych plików `*_refactor.md`**:
   - Przeniesienie wszystkich śródplikowych deklaracji `const` na górę odpowiednich plików `.go` zaraz po `import`.
   - Usunięcie roboczych plików `*_refactor.md` po zakończeniu wdrożenia i aktualizacja `docs/project.graph`.

---

## 3. Zmiany plik po pliku, metoda po metodzie, linia po linii

### Etap 1: Oczyszczenie `app/js/dispatcher.js` z pozostałości `WFP` i naprawa `keepScroll` (`skipInit`)

#### `app/js/dispatcher.js`
- **`L17–68` (`settings`)**:
  - Usunąć nieużywane klucze atrybutowe: `paramPrefixLen`, `attrs.action`, `attrs.kind`, `attrs.target`, `attrs.eventPrefix`, `attrs.paramPrefix`, `selectors.action`, `vals.backend`.
  - Dodać `vals.keepScrollKey: "keepScroll"`.
- **`L74–78` (`modules`)**:
  - Usunąć `modules: { backend: new Proxy(...) }` (zostaje wyłącznie mapa `controllers: {}`).
- **`L107–113` (`action(actionName)`)**:
  - Zmienić sygnaturę z `action(actionName, mapFn)` na czyste:
    ```js
    action(actionName) {
      return function (params, _el, e) {
        if (e && e.preventDefault) e.preventDefault();
        Dispatcher.post(actionName, params);
      };
    },
    ```
- **`L180–265` (`resolve(e)`)**:
  - Usunąć gałęzie `pPrefix + a.action` (`L203–214`), `evEl = t.closest("[" + evAttr + "]")` (`L217–227`) oraz fallback `t.closest(s.selectors.action)` (`L251–264`).
  - Zostawić wyłącznie obsługę `this.activePointerEl` dla `pointermove`/`pointerup` (`L186–202`) oraz dopasowanie najgłębszego selektora z `this.bindingsByEvent[e.type]` (`L229–250`).
- **`L267–325` (`collectParams(hit, e)`)**:
  - Usunąć gałęzie `a.name.startsWith(s.attrs.paramPrefix)` (`L285–286`, `L312–313`) oraz pętlę `hit.eventPrefix` (`L292–300`). Zostawić zbieranie pól `formEl.elements`, `el.name`, atrybutów `data-*` (z pominięciem `data-hint`), współrzędnych kursora i `dropZone`.
- **`L327–353` (`swapTarget` i `applyResponse`)**:
  - Zmienić `swapTarget(targetSel, html, skipInit)`: w `L336` wywołać `this.mountAll(targetEl, skipInit)`.
  - W `applyResponse` (`L348–350`):
    ```js
    const skipInit = decoded[s.vals.keepScrollKey] === s.vals.trueStr;
    if (t && t !== s.vals.none) {
      this.swapTarget(t, decoded.html || "", skipInit);
    }
    ```
- **`L365–387` (`handle(e)`)**:
  - Usunąć martwy fallback `const mod = this.modules[hit.kind]` (`L384–386`), ponieważ każdy trafiony element posiada `hit.handler` z `bindings()`.

---

### Etap 2: Odchudzenie `internal/actions/dispatcher.go` (eliminacja `if act == ...` i `wfpMimePrefix`)

#### 2.1. `internal/actions/specs.go`
- **`type ActionSpec struct`**:
  - Dodać deklaratywne flagi eliminujące sprawdzanie nazw akcji w dispatcherze:
    - `ImmediateNoContent bool` (ustawione na `true` dla `SaveAudioTime` i `StartWindowDrag`).
    - `RefreshPlayerOnImageless bool` (ustawione na `true` dla `ToggleImageless`).
    - `EscalateAppOnFormScene bool` (ustawione na `true` dla `OpenSceneTextModal`).
    - `CloseSceneTextTarget bool` (ustawione na `true` dla `CloseSceneTextModal`).

#### 2.2. `internal/actions/dispatcher.go`
- **`L49` i `L70–80`**:
  - Usunąć stałą `wfpMimePrefix = "text/x-wfp"` i warunek `strings.HasPrefix(..., wfpMimePrefix)`.
  - Zinlinować jednowierszowe wywołanie `isFDPRequestPayload` bezpośrednio w `ParseActionRequest` (`L95`), likwidując ostrzeżenie `ARCH_RAVIOLI_INDIRECTION` (`dispatcher.go:70`).
- **`L173–245` (`resolveActionChunks`)**:
  - Zastąpić warunki `if act == ToggleImageless`, `if act == OpenSceneTextModal`, `if act == CloseSceneTextModal`, `if act == AddBookVariable`, `if act == AddChoice` sprawdzeniem flag z `ActionSpec` i porównaniem stanu `before` vs `s`:
    - `before.sceneTextOpen != s.Editor.SceneText.IsOpen()` oraz `before.rightPanel != s.Editor.RightPanel`.
    - `spec.RefreshPlayerOnImageless && state.ActiveWorkspaceView(s) == state.ViewPlayer`.
    - `spec.EscalateAppOnFormScene && before.hasFormScene`.
    - `spec.CloseSceneTextTarget`.
- **`L317–321` (`HandleAction`)**:
  - Zastąpić `if act == SaveAudioTime || act == StartWindowDrag` przez `if spec.ImmediateNoContent`.

---

### Etap 3: Konsolidacja formularza sceny i przeniesienie logiki domenowej do `internal/book/mutator_scene.go`

#### 3.1. `internal/book/mutator_scene.go`
- Przenieść z `internal/actions/actions_editor_scene.go` i ustrukturyzować jako metody/funkcje domenowe:
  - `SplitInlineChoiceLabelAndTarget(raw string) (label, target string, hasArrow bool)` (z `actions_editor_scene.go:111–119`).
  - `ResolveEditedChoiceTarget(b *Book, target *Scene, tgt string) string` (z `actions_editor_scene.go:466–502`).
  - `ApplySceneSluglineToken(b *Book, target *Scene, newPrefix, newTime string)` (z `actions_editor_scene.go:1033–1063`).
  - `AppendScreenplayBlockSnippet(b *Book, target *Scene, kind, charName string) bool` (z `actions_editor_scene.go:1075–1123`).

#### 3.2. `internal/actions/actions_editor_scene.go`
- **`L186–593` (`hasSceneTextModalChanges`, `applySceneFormFields`, `applySceneFormChoices`)**:
  - Połączyć `hasSceneTextModalChanges` (`L186–306`) i `applySceneFormFields` (`L308–464`) w jedną funkcję `applySceneFormFields(s *state.AppState, target *book.Scene, r *http.Request) (changed bool)`, która w jednym przebiegu porównuje zsanityzowane wartości z `target`, zapisuje różnice i zwraca `changed`.
  - W `actions_text_editor.go` (przy `CloseSceneTextModal` / `SaveSceneTextModal`) użyć wyniku `changed := applySceneFormFields(s, curSc, r)` zamiast osobnego wywołania `hasSceneTextModalChanges` przed `saveSceneFromForm`.
  - Redukcja rozmiaru `internal/actions/actions_editor_scene.go` z `1125 LOC` do `~780 LOC` (poniżej progu 1000 LOC `ARCH_MONOLITH_AND_NESTING` i usunięcie 2 ostrzeżeń `ARCH_STRING_HACK`).

---

### Etap 4: Eliminacja duplikacji w `internal/views/` (`view_editor_context_panel.go`, `view_text_editor.go`, `view_studio.go`)

#### 4.1. `internal/book/model.go`
- Dodać strukturę `SceneMediaOptions` zastępującą 6-elementową krotkę zwracaną przez `buildSceneMediaOptions`:
  ```go
  type SceneMediaOptions struct {
      CurrentImage      string
      CurrentImageLabel string
      ImageOptions      []SelectOption
      CurrentVoice      string
      CurrentVoiceLabel string
      VoiceOptions      []SelectOption
  }
  ```

#### 4.2. `internal/views/view_editor_context_panel.go`
- **`L211–305` (`buildSceneMediaOptions`)**:
  - Zmienić sygnaturę z `(s *state.AppState, sc *book.Scene, buildMenus bool) (string, string, []book.SelectOption, string, string, []book.SelectOption)` na `(s *state.AppState, sc *book.Scene, buildMenus bool) book.SceneMediaOptions` (usuwa błąd `ARCH_SIGNATURE_BLOAT` o wadze 81 pkt).
- **`L715–724` -> wydzielić helper `buildSceneSyntaxSuggestions(s *state.AppState, sc *book.Scene)`**:
  - Zwraca spójną strukturę sugestii zmiennych, enumów, celów i wierszy pomocy składniowej współdzieloną przez `ViewEditorContextPanel` i `ViewTextEditor`.

#### 4.3. `internal/views/view_text_editor.go` & `app/templates/text_editor.tpl`
- **`internal/views/view_text_editor.go` (`L14–90` i `L184–241`)**:
  - W `textEditorTemplate` dodać pole `Node *book.EditorNodeView` i usunąć 24 zduplikowane pola przepisywane 1:1 z `nv` (`SceneTextModalRawName`, `SceneTextModalExplicitID`, `SceneTextModalSlugline`, `SceneTextModalSynopsis`, `SceneTextModalLengthLabel`, `SceneTextModalIsStart`, `SceneTextModalChapter`, `SceneTextModalChapterOptions`, `SceneTextModalHasVoiceSections`, `SceneTextModalVoiceSections`, `SceneTextModalActiveVoiceSectionKey`, `SceneTextModalActiveVoiceSectionFile`, `SceneTextModalCurrentImage`, `SceneTextModalCurrentImageLabel`, `SceneTextModalImageOptions`, `SceneTextModalCurrentVoice`, `SceneTextModalCurrentVoiceLabel`, `SceneTextModalVoiceURL`, `SceneTextModalVoiceDurationLabel`, `SceneTextModalVoiceTrimStart`, `SceneTextModalVoiceTrimEnd`, `SceneTextModalHasVoice`, `SceneTextModalHasVoiceTrim`, `SceneTextModalVoiceOptions`, `SceneTextModalEffects`, `SceneTextModalHasEffects`, `SceneTextModalEffectsInvalid`, `SceneTextModalEffectsErrorHint`).
  - W `app/templates/text_editor.tpl` odwoływać się bezpośrednio do `{Node.ExplicitID}`, `{Node.Slugline}`, `{Node.Synopsis}`, `{Node.ChapterOptions}`, `{Node.ImageOptions}`, `{Node.VoiceOptions}`, `{Node.Effects}` — identycznie jak w `editor_context_panel.tpl`.

---

### Etap 5: Modularyzacja `app/js/` i usunięcie magic stringów z `www/app.js`

#### 5.1. Podział `app/js/editor_voice.js` (843 LOC -> 2 pliki < 500 LOC) + rejestracja w `internal/tpl/bundle.go`
- **`app/js/editor_voice_trim.js` (nowy moduł ~360 LOC)**:
  - Przenieść logikę przycinania (trim handles, playhead scrub, formatowanie czasu i zapisu zakresu `TrimSceneVoice`) z `app/js/editor_voice.js`.
- **`app/js/editor_voice.js` (~480 LOC)**:
  - Zostawić obsługę nagrywania mikrofonu, pollingu `PollSceneVoiceCapture` i przełączania sekcji głosu.
- **`internal/tpl/bundle.go`**:
  - Dodać `editor_voice_trim.js` do `JSModuleFiles`.

#### 5.2. Podział `app/js/editor_webgl.js` (773 LOC -> 2 pliki < 460 LOC) + rejestracja w `internal/tpl/bundle.go`
- **`app/js/editor_webgl_shaders.js` (nowy moduł ~320 LOC)**:
  - Przenieść źródła shaderów GLSL (grid, rounded-rect SDF, bezier wires, MSDF text) oraz kompilację programów WebGL i budowanie atlasu MSDF z `app/js/editor_webgl.js`.
- **`app/js/editor_webgl.js` (~450 LOC)**:
  - Zostawić inicjalizację kontekstu WebGL, buforowanie instancji i pętlę rysowania klatek.
- **`internal/tpl/bundle.go`**:
  - Dodać `editor_webgl_shaders.js` przed `editor_webgl.js` w `JSModuleFiles`.

#### 5.3. `www/app.js` (usunięcie 17 magic stringów i 1 funkcji pass-through)
- **`L1–12`**:
  - Dodać słownik stałych `wwwCfg` (`classes: { betaOpen: "beta-open", wasmOpen: "wasm-open" }`, `ids: { storyAudio: "story-audio" }`, `fnType: "function"`, `colors: { bgBlack: "#000000", edgeDim: "rgba(255, 255, 255, 0.075)", edgeActive: "rgba(255, 255, 255, 0.62)", whiteAlphaPrefix: "rgba(255, 255, 255, " }`).
- **`L14–38` (`openWasmStage`, `closeWasmStage`, `openBetaModal`, `closeBetaModal`) i `L322–438` (`frame`)**:
  - Zastąpić wszystkie 17 literałów stringowych referencjami do `wwwCfg`.
- **`L116` (`buildStoryGraph`)**:
  - Zinlinować jednowierszową funkcję `buildStoryGraph()` w miejscu wywołania (usuwa `ARCH_RAVIOLI_INDIRECTION`).

---

### Etap 6: Porządki w deklaracjach `const` w Go, usunięcie plików tymczasowych `*.md` i weryfikacja

1. **Przeniesienie śródplikowych deklaracji `const` na górę pliku tuż po `import` (13 plików)**:
   - `internal/actions/actions_editor.go:507` (`formKeySearchQuery`)
   - `internal/actions/actions_modal_new_book.go:35` (`newBookFormTitle`)
   - `internal/book/history_ops.go:814` (`historyHoverSummaryFmt`)
   - `internal/book/layout.go:155` (`EditorGridSize`)
   - `internal/book/loader.go:429, 670` (`MaxZipEntries`, `maxBookDownloadBytes`)
   - `internal/book/media.go:81, 82` (`MaxSingleMediaBytes`, `maxMediaEntryBytes`)
   - `internal/book/model.go:881` (`DefaultAutoChoiceDelay`)
   - `internal/book/save.go:59` (`DefaultScrollSensitivity`)
   - `internal/book/web_exporter.go:166` (`wasmSplashTplName`)
   - `internal/tpl/tpl_render.go:206` (`indentSpaces`)
   - `internal/views/view_app_shell.go:49` (`themeLight`)
2. **Usunięcie nieaktualnych roboczych plików `*.md` z katalogu głównego (`AGENTS.md §6`)**:
   - Usunąć: `actions_modal_scene_text_refactor.md`, `deck_refactor.md`, `declarative_controller_architecture.md`, `editor_gestures_refactor.md`, `fdp_actions_refactor.md`, `js_dispatcher_refactor.md`, `js_refactor.md`, `plan_gnerator.md`, `refactor_actions_modal_confirm.md`, `refactor_cmd_mojra.md`, `runtime_icon_removal.md`, `set_device_mode_refactor.md`, `tss_refactor.md`, `voice_rerecord_fix.md`.
3. **Weryfikacja końcowa**:
   - `go run ./cmd/projgraph` (aktualizacja `docs/project.graph`)
   - `go run ./cmd/mojra audit --static` + `go run ./cmd/mojra audit --arch` + `go run ./cmd/mojra audit --magic --js`
   - `go test ./...`
   - `./build.sh --macos`
