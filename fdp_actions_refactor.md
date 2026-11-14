# Plan naprawczy: Migracja do `FDP` (`fdp.go` / `fdp.js`), generyczny `dispatcher.go` (~90 LOC) i odchudzenie `actions` ↔ `state` / `book` ↔ `views` zgodnie z `@ARCH.md`

## 1. Co tam jest i jakie są problemy

1. **Niedokończona migracja `WFP` -> `FDP` (trzy równoległe mechanizmy transportu niezgodne z `ARCH.md §1, §3.3, §3.4`)**:
   - Nadal istnieją `internal/actions/wfp.go` (`EncodeWFPForm`, `DecodeWFPFormBytes`), `wfpBufPool` / `AcquireWFPBuffer` / `ReleaseWFPBuffer` / `isWFPRequestPayload` w `internal/actions/actions_dispatch.go` (`L47–121`) oraz `app/js/wfp.js` (`Wfp`).
   - Obok nich istnieje `internal/tpl/tpl_data_header.go` (`EncodeControllerDataHeader` piszący do nagłówka HTTP `X-Data` z separatorem `;`).
   - Obok `X-Data` `actions_dispatch.go` (`writeActionResponseHeaders`, `L741–794`) i `actions_editor_history.go` (`WriteEditorHistoryHeaders`, `L53–118`) wystawiają ~20 ręcznych nagłówków HTTP `X-*` (`X-Theme`, `X-Lang`, `X-Imageless`, `X-Imageless-Locked`, `X-UI-Scale`, `X-Zen-Mode`, `X-Scroll-Sensitivity`, `X-Device-Mode`, `X-Scene-ID`, `X-Scene-Prev-ID`, `X-Scene-Explicit-ID`, `X-Scene-Ref`, `X-Scene-Title`, `X-Scene-Chapter`, `X-Start-Scene-ID`, `X-History-Preview`, `X-History-Count`, `X-Can-Undo`, `X-Undo-Title`, `X-Can-Redo`, `X-Redo-Title`, `X-Toast`, `X-Native-Cmd`, `X-Voice-Peak`, `X-Modal-ID`, `X-Target`, `X-Keep-Scroll`, `X-Reset-Canvas`), które `AppShellController.syncHeaders` (`app/js/app_shell_controller.js:257–382`) parsuje ręcznie z `resp.headers`.
2. **Rozrost i hardkodowanie w `internal/actions/actions_dispatch.go` (794 LOC, 67 magic stringów, 7 ostrzeżeń `ARCH_*`)**:
   - **L28–37**: Martwy struct `Router` oraz zduplikowane `langOf(s)` (istnieje już `views.LangOf(s)` w `internal/views/view_app_shell.go:21`).
   - **L47–66**: Zduplikowany `wfpBufPool` (w `internal/tpl/tpl_render.go` istnieje już `tpl.AcquireBuffer()` / `tpl.ReleaseBuffer()`).
   - **L123–163, L374–472 (`actionTargetKind` i `resolveActionView`)**: 22 warianty enumu i 7 stałych `modalID*`, w tym 7 identycznych targetów modalnych (`targetModalQR`, `targetModalNewBook`, `targetModalExport`, `targetModalAddBook`, `targetModalStudioProjects`, `targetModalConfirm`, `targetTextEditor`), mimo że dla `#modals-root` `HandleAction` (`L695–712`) zawsze renderuje `views.ViewActiveModalChunks(s)`.
   - **L330–350**: 4 handlery (`ToggleStudioSearch`, `CloseStudioSearch`, `FilterStudioSearch`, `ToggleBookWarnings`) zahardkodowane jako anonimowe domknięcia wewnątrz mapy `ActionSpecs` (z magic stringiem `"q"` w `L344`) zamiast w `internal/actions/actions_editor.go`.
   - **L362–372**: 3 zakazane funkcje pass-through (`isStandaloneForbiddenAction`, `isKeepScrollAction`, `isResetCanvasAction` — `ARCH_RAVIOLI_INDIRECTION`).
   - **L402, L407, L457, L644, L650, L670**: Zahardkodowane gałęzie `if act == ...` (`AddBookVariable`, `ToggleSceneVoiceRecorder`, `SelectSceneVoiceSection`, `TrimSceneVoice`, `ConfirmModalAccept`, `SaveAudioTime`, `StartWindowDrag`, `AddChoice`) w `resolveActionView` i `HandleAction`.
   - **L458, L460, L466**: Podwójne opakowywanie `tpl.TemplateChunk{Template: "text_editor" | "app_shell" | "modal_qr", Data: views.View*(s)}` z zahardkodowanymi nazwami szablonów oraz ręczne kopiowanie `qrChunk.ScriptData = deckChunk.ScriptData` (`L465`).
   - **L521–550 (`isRemoteLANAllowedAction`)**: 30-liniowy zahardkodowany `switch act` z 21 nazwami akcji zamiast pola `LANAllowed bool` w `ActionSpec`.
   - **L565–597, L784–793**: Sprzętowa obsługa nagrywania głosu (`osbridge.*NativeAudioCapture*`, `voiceCaptureCtxKey`) zahardkodowana w `HandleAction` i zduplikowana w `writeActionResponseHeaders`, zamiast wywołania `s.WithUnlocked(...)` wewnątrz handlerów w `actions_editor_scene.go`.
3. **Logika domenowa i mutacje stanu upchane w handlerach HTTP (`internal/actions/actions_editor_scene.go` — 1796 LOC)**:
   - `hasSceneTextModalChanges` (`L76–215`) i `applySceneFormFields` (`L217–626`) dwukrotnie parsują i sanityzują dokładnie te same pola formularza (`text`, `name`, `explicit_id`, `slugline`, `synopsis`, `scene_effects`, `chapter`, `explicit_image`, `explicit_voice`, `choice_*`).
   - Operacje czysto domenowe na modelu `book.Book` (przepinanie ID/referencji sceny po zmianie nazwy, materializacja wyborów `Dalej`/`END`, usuwanie scen i aktualizacja linków, dodawanie/usuwanie/zmiana kolejności/rozgałęzianie wyborów) siedzą w handlerach HTTP w `actions_editor_scene.go` zamiast w `internal/book/mutator.go`.
4. **Zduplikowana logika budowania widoków edytora (`internal/views/`)**:
   - `view_editor_context_panel.go` (`editorContextPanelTemplate` — 49 pól) i `view_text_editor.go` (`textEditorTemplate` — 76 pól) powielają 1:1 przygotowanie tych samych danych sceny: opcji rozdziału, mediów/sekcji głosu/trimu, sugestii zmiennych/enumów/celów, walidacji `Effects` oraz reguł wyborów (`Choices`).
   - `view_studio.go` (`studioTemplate`) kopiuje pola z powrotem z `ctxChunk` i `histChunk` (`ctxData.IsHistoryPreview = histData.IsHistoryPreview` itd.), zamiast delegować pod-widoki do autonomicznych `TemplateChunk`.

---

## 2. Rozwiązania

1. **Jeden płaski kodek `FDP` (`internal/tpl/fdp.go` + `app/js/fdp.js`) zgodnie z `ARCH.md §3.3–3.4`**:
   - Całkowite usunięcie `internal/actions/wfp.go`, `internal/tpl/tpl_data_header.go`, `wfpBufPool` z `actions_dispatch.go` oraz `app/js/wfp.js`.
   - Żądania i odpowiedzi `POST /api/action` (oraz mostek WASM) używają wyłącznie pojedynczej ramki `FDP` (`Content-Type: text/x-fdp;charset=UTF-8`) opartej o `tpl.AcquireBuffer()` / `tpl.ReleaseBuffer()`:
     - Nagłówek przed `\n\n`: płaskie linie `key=val` (`target`, `keepScroll`, `resetCanvas`, `toast`, `nativeCmd`, `voicePeak`, `saveHex` + skalarne pola zebrane z nie-nilowych `chunk.ScriptData` w drzewie `TemplateChunk` + `_body=html`).
     - Ciało po `\n\n`: czysty wyrenderowany HTML z `chunk.TemplateData`.
   - Całkowite usunięcie `writeActionResponseHeaders`, `WriteEditorHistoryHeaders`, `contextBookForHeaders` oraz `AppShellController.syncHeaders` — stan powłoki (`appShellScript`), edytora (`studioScript`, `editorHistoryScript`) i odtwarzacza (`playerScript`, `audioDeckScript`) zasila `ctrl.data` wyłącznie przez `Dispatcher.hydrate(decoded)` (`if (key in ctrl.data)`).
2. **Odchudzony, generyczny `internal/actions/dispatcher.go` (~100 LOC, zero `if act == ...`, zero magic stringów) + podział akcji według domen stanu**:
   - Akcje są sparowane z mutacjami stanu (`state` / `book`), a nie z widokami.
   - `state.AppState` śledzi brudny cel DOM (`DirtyTarget`: `#app`, `#workspace`, `#modals-root`, `#editor-context-panel`, `none`) lub `ActionSpec` deklaruje 1 z 5 rzeczywistych celów DOM (`targetApp`, `targetWorkspace`, `targetModals`, `targetInspector`, `targetNone`), a eskalacja stanu (np. otwarcie `ModalConfirm`, otwarty `SceneText`, otwarty `Modals.QR`) działa automatycznie na bazie stanu `s` bez ani jednego `if act == ...`.
   - Podział `internal/actions/` na spójne pliki domenowe (`< 250 LOC` każdy):
     - `internal/actions/dispatcher.go` (~100 LOC — zastępuje `actions_dispatch.go`)
     - `internal/actions/action.go` (stałe nazw akcji)
     - `internal/actions/actions_app_shell.go` (`switchView`, `setDeviceMode`, `toggleFullscreen`, `startWindowDrag`, `quitApp`)
     - `internal/actions/actions_topbar.go` (`toggleTheme`, `setLanguage`, `toggleImageless`, `toggleUIScale`, `toggleZenMode`)
     - `internal/actions/actions_player.go` (akcje odtwarzacza i `audio_deck`)
     - `internal/actions/actions_library.go` (biblioteka i import z dysku/URL)
     - `internal/actions/actions_editor.go` (ustawienia książki, viewport kanwy, wyszukiwarka i ostrzeżenia)
     - `internal/actions/actions_editor_chapter.go` (rozdziały i połączenia rozdziałów)
     - `internal/actions/actions_editor_scene.go` (sceny i wybory — cienkie handlery delegujące do `internal/book/mutator.go`)
     - `internal/actions/actions_editor_voice.go` (nagrywanie, sekcje i przycinanie głosu — wydzielone z `actions_editor_scene.go`)
     - `internal/actions/actions_editor_variables.go` (zmienne i enumy)
     - `internal/actions/actions_editor_breakdown.go` (postacie i lokacje scenariusza)
     - `internal/actions/actions_editor_history.go` (historia wersji, gałęzie, undo/redo)
     - `internal/actions/actions_text_editor.go` (pełnoekranowy edytor tekstu sceny)
     - `internal/actions/actions_modal_*.go` (modale: `confirm`, `qr`, `new_book`, `export`, `add_book`, `studio_projects`)
3. **Przeniesienie operacji na grafie i formularzu sceny do `internal/book/mutator.go`**:
   - Połączenie `hasSceneTextModalChanges` i `applySceneFormFields` w jedną funkcję `applySceneFormFields(s, target, r) (changed bool)`.
   - Przeniesienie mutacji modelu (`MaterializeChoicesForEdit`, `RenameAndRebindScene`, `DeleteScenesByIDs`, `AddChoice`, `RemoveChoice`, `ReorderChoice`, `BranchChoiceToNewScene`) z `actions_editor_scene.go` do `internal/book/mutator.go`.
4. **Eliminacja duplikacji w `internal/views/`**:
   - Wydzielenie wspólnego konstruktora danych inspektora sceny (`buildSceneInspectorData` w `internal/views/view_editor_scene_inspector.go`) używanego przez `ViewEditorContextPanel` i `ViewTextEditor`.
   - Usunięcie ręcznego kopiowania pól między `studioTemplate`, `editorContextPanelTemplate` i `editorHistoryTemplate` w `view_studio.go`.

---

## 3. Zmiany plik po pliku, metoda po metodzie

### Etap 1: Kodek `FDP` (`internal/tpl/fdp.go` & `app/js/fdp.js`) — usunięcie `WFP` i `X-*`

#### 1.1. `internal/actions/wfp.go` & `internal/tpl/tpl_data_header.go` -> **USUNĄĆ**, zastąpić przez `internal/tpl/fdp.go`
- **`const (...)` na górze `internal/tpl/fdp.go`**:
  - `ContentTypeFDP = "text/x-fdp;charset=UTF-8"`, `fdpKeyTarget = "target"`, `fdpKeyKeepScroll = "keepScroll"`, `fdpKeyResetCanvas = "resetCanvas"`, `fdpKeyToast = "toast"`, `fdpKeyNativeCmd = "nativeCmd"`, `fdpKeyVoicePeak = "voicePeak"`, `fdpKeySaveHex = "saveHex"`, `fdpKeyBody = "_body"`, `fdpBodyHTML = "html"`.
- **`type FDPFrameMeta struct`**:
  - Pola: `Target string`, `KeepScroll bool`, `ResetCanvas bool`, `Toast string`, `NativeCmd string`, `VoicePeak string`, `SaveHex string`.
- **`EscapeFDPLineVal(s string) string` & `UnescapeFDPLineVal(s string) string`**:
  - Escapowanie `\n`, `\r`, `\` w liniach nagłówka `FDP`.
- **`EncodeFDPForm(vals url.Values) []byte` & `DecodeFDPBytes(raw []byte, dst url.Values) bool`**:
  - Kodowanie i dekodowanie płaskich ramek żądań `key=val\n...\n_body=<key>\n\n<raw UTF-8>`.
- **`WriteFDPResponse(w io.Writer, meta FDPFrameMeta, htmlBody []byte, chunks ...TemplateChunk) error`**:
  - Zapisuje linie z `meta`, przechodzi BFS po `chunks` (oraz zagnieżdżonych `TemplateChunk` / `[]TemplateChunk` w `TemplateData`), emituje 1:1 skalarne pola z każdego nie-nilowego `ScriptData`, a gdy `len(htmlBody) > 0` — zapisuje `_body=html\n\n` oraz `htmlBody`.

#### 1.2. `app/js/wfp.js` -> **ZMIGROWAĆ DO `app/js/fdp.js` (`const Fdp = { encode, decode }`)**
- **`Fdp.encode(paramsObj)`**:
  - Buduje płaską ramkę `key=val\n...` z opcjonalnym `_body=<key>\n\n<val>` dla wartości wieloliniowych.
- **`Fdp.decode(rawStr)`**:
  - Rozdziela nagłówek od ciała na pierwszym `\n\n` (lub `\r\n\r\n`), dekoduje linie `key=val` do płaskiego obiektu `decoded` i przypisuje ciało pod `decoded[bodyKey]` (`decoded.html`).

#### 1.3. `app/js/dispatcher.js` & `app/js/app_shell_controller.js`
- **`app/js/dispatcher.js`**:
  - W `settings.http`: `fdpMime: "text/x-fdp;charset=UTF-8"`; usunięcie `settings.headers` (`X-Data`, `X-Target`).
  - `hydrate(decoded, action, params)`: iteruje po kluczach `decoded` (pomijając `html` i `_body`), aktualizuje `this.assignDataValue(ctrl.data, key, decoded[key])` dla każdego zarejestrowanego kontrolera posiadającego `key in ctrl.data`, a następnie wywołuje opcjonalny hook `ctrl.onHydrate(decoded, action, params)`.
  - `swapTarget(targetSel, html, skipInit)`: podmienia `targetEl.innerHTML = html` i woła `this.mountAll(targetEl, skipInit)` (gdzie `skipInit === true` pomija ponowne wywołanie `init` przy `keepScroll`).
  - `applyResponse(resp, action, params, targetOverride)`: pobiera tekst ramki (`typeof resp === "string" ? resp : await resp.text()`), dekoduje przez `const decoded = Fdp.decode(raw)`, wywołuje `this.hydrate(decoded, action, params)` i gdy `decoded.target && decoded.target !== "none"` — woła `this.swapTarget(targetOverride || decoded.target, decoded.html || "", decoded.keepScroll === "true")`.
  - `post(action, params, target)`: wysyła `Fdp.encode(Object.assign({ action: action }, params))` z `Content-Type: text/x-fdp;charset=UTF-8`.
- **`app/js/app_shell_controller.js`**:
  - Usunąć metodę `syncHeaders(resp, _action, params)` (`L257–382`) oraz cały słownik `settings.headers` (`L65–83`).
  - Dodać w `data`: `keepScroll: false`, `resetCanvas: false`, `voicePeak: ""`, `saveHex: ""`, `sceneId: ""`.
  - Dodać metodę `onHydrate(decoded, _action, params)` aplikującą zhydratowane `this.data` (`theme`, `lang`, `imageless`, `imagelessLocked`, `uiScale`, `zenMode`, `deviceMode`, `toast`, `nativeCmd`, `resetCanvas`, `voicePeak`, `saveHex`, `sceneId`) na `document.body` i komponenty.

#### 1.4. `internal/views/view_app_shell.go`, `view_studio.go`, `view_editor_history.go`, `view_modal_qr.go`
- **`internal/views/view_app_shell.go`**:
  - Rozszerzyć `appShellScript` o `lang string` (`book.NormalizeLanguage(s.Settings.Language)`).
  - Dodać `ViewAppShellScript(s *state.AppState) tpl.TemplateChunk` zwracający `TemplateChunk{ScriptData: &appShellScript{...}}`, dołączany automatycznie przez dispatcher przy częściowych wymianach DOM (`#workspace`, `#modals-root`, `#editor-context-panel`, `none`).
- **`internal/views/view_studio.go` & `internal/views/view_editor_history.go`**:
  - Rozszerzyć `studioScript` o pola zastępujące dawne `WriteEditorHistoryHeaders`: `sceneId`, `scenePrevId`, `sceneExplicitId`, `sceneRef`, `sceneTitle`, `sceneChapter`, `startSceneId`, `historyPreview`, `historyCount`, `canUndo`, `undoTitle`, `canRedo`, `redoTitle`.
- **`internal/views/view_modal_qr.go`**:
  - Dodać `modalQRScript{theme, uiScale, imageless, imagelessLocked, sensitivity, autoChoiceDelay}` i przypisać do `ScriptData` w `ViewModalQR(s)`.

---

### Etap 2: Generyczny `internal/actions/dispatcher.go` (~100 LOC, zastępuje `actions_dispatch.go`)

- **Zmiana nazwy pliku**: `internal/actions/actions_dispatch.go` (794 LOC) -> `internal/actions/dispatcher.go` (~100–140 LOC) + `internal/actions/specs.go` (deklaratywna tabela `ActionSpecs`).
- **Usunięte z dispatchera**:
  - `Router`, `langOf`, `wfpBufPool`, `AcquireWFPBuffer`, `ReleaseWFPBuffer`, `isWFPRequestPayload`, `isStandaloneForbiddenAction`, `isKeepScrollAction`, `isResetCanvasAction`, `isRemoteLANAllowedAction` (switch), `contextBookForHeaders`, `writeActionResponseHeaders`, `WriteEditorHistoryHeaders`, 7 stałych `modalID*`, 14 nadmiarowych wariantów `actionTargetKind`, 4 inline handlery wyszukiwarki/ostrzeżeń oraz blok `osbridge.*NativeAudioCapture*`.
- **5 rzeczywistych celów DOM (`actionTargetKind`)**:
  - `targetApp` (`#app` -> `views.ViewAppShell(s)`)
  - `targetWorkspace` (`#workspace` -> `views.ViewActiveWorkspaceChunk(s)`)
  - `targetModals` (`#modals-root` -> `views.ViewActiveModalChunks(s)`)
  - `targetInspector` (`#editor-context-panel` -> `views.ViewEditorContextPanel(s)`)
  - `targetNone` (`none` -> brak HTML, sama hydratacja `ScriptData`)
- **`resolveActionChunks(s *state.AppState, spec ActionSpec, before stateSnapshot) (string, []tpl.TemplateChunk)`**:
  - ~35 linii czystej logiki opartej na `spec.Target` i porównaniu `before` vs `s` (otwarcie `ModalConfirm`, otwarty `SceneText`, otwarty `Modals.QR`, załadowanie pierwszej książki) — zero `if act == ...`.
- **`HandleAction(s *state.AppState, w http.ResponseWriter, r *http.Request, ensureTemplates func() map[string]*tpl.ParsedTemplate)`**:
  1. Walidacja `POST` + `validateSameOriginAction(r)` + `ParseActionRequest(r)` (`tpl.DecodeFDPBytes`).
  2. `s.Mu.Lock()`, sprawdzenie `!isLoopbackRequest(r) && !spec.isLANAllowed(s)` oraz `s.Config.StandaloneMode && spec.StandaloneForbidden`.
  3. Zapis `audio_time`, opcjonalny `flushFormSceneIfPresent(s, r)`, wywołanie `toast := spec.Handler(s, r)`, `s.Editor.SyncToSettings(s.Settings)`.
  4. Wyznaczenie `targetSelector, chunks := resolveActionChunks(s, spec, before)`, wyrenderowanie `chunks` przez `tpl.RenderTemplateTo` do bufora `tpl.AcquireBuffer()` i zapis ramki `tpl.WriteFDPResponse(w, meta, htmlBuf.Bytes(), allChunks...)` z `Content-Type: text/x-fdp;charset=UTF-8`.

---

### Etap 3: Odchudzenie `actions_editor_scene.go` (1796 LOC -> ~350 LOC), wydzielenie `actions_editor_voice.go` i przeniesienie domeny do `internal/book/mutator.go`

#### 3.1. `internal/actions/actions_editor_voice.go` (nowy plik wydzielony z `actions_editor_scene.go:30–75, 1485–1796`)
- Przenieść: `activeVoiceSectionKeyForRequest`, `formVoiceSectionKeyForRequest`, `resolveSceneVoiceSectionKey`, `resolveVoiceActionTargetScene`, `toggleSceneVoiceRecorderAction`, `selectSceneVoiceSectionAction`, `startSceneVoiceCaptureAction`, `pollSceneVoiceCaptureAction`, `commitSceneVoiceFile`, `stopSceneVoiceCaptureAction`, `saveSceneVoiceRecordingAction`, `trimSceneVoiceAction`, `deleteSceneVoiceAction`.
- W `selectSceneVoiceSectionAction`, `startSceneVoiceCaptureAction`, `stopSceneVoiceCaptureAction` wywoływać `osbridge.*NativeAudioCapture*` bezpośrednio wewnątrz `s.WithUnlocked(func() { ... })`; usunąć `voiceCaptureCtxKey` i `voiceCaptureResult`.

#### 3.2. `internal/book/mutator.go` (przeniesienie operacji na modelu z `actions_editor_scene.go`)
- `MaterializeChoicesForEdit(b *Book, sc *Scene) []Choice` (z `materializeChoicesForEdit`, `L698–711`).
- `RenameAndRebindScene(b *Book, sc *Scene, newName, newExplicitID string) (oldID string, changed bool)` (z `L224–268` i `L517–614`).
- `DeleteScenesByIDs(b *Book, ids []string) (deleted []*Scene, err error)` (połączenie zduplikowanych `deleteSceneByID` i `deleteScenesByIDs`, `L1196–1384`).
- `AddChoiceToScene(b *Book, sc *Scene) int` (z `addChoiceToScene`, `L713–777`).
- `RemoveChoiceFromScene(b *Book, sc *Scene, choiceIdx int) (removedLabel string, ok bool)` (z `removeChoiceBySceneAndIndex`, `L787–827`).
- `ReorderSceneChoice(b *Book, sc *Scene, fromIdx, toIdx int) bool` (z `reorderChoiceOnScene`, `L829–905`).
- `BranchChoiceToNewScene(b *Book, sc *Scene, choiceIdx int) (*Scene, error)` (z `branchSceneFromChoice`, `L907–1025`).

#### 3.3. `internal/actions/actions_editor_scene.go` (~350 LOC)
- Zastąpienie zduplikowanej pary `hasSceneTextModalChanges` (`L76–215`) + `applySceneFormFields` (`L217–626`) jedną funkcją `applySceneFormFields(s *state.AppState, target *book.Scene, r *http.Request) (changed bool)`, która w jednym przebiegu porównuje zsanityzowane pola formularza z `target`, aplikuje zmiany i zwraca `changed`.
- Handlery `addChoiceToScene`, `removeChoiceFromScene`, `reorderChoiceOnScene`, `branchSceneFromChoice`, `insertScene`, `insertSceneBefore`, `createSceneAt`, `deleteSceneFromForm`, `deleteScenesFromForm`, `applySluglineTokenAction`, `insertScreenplayBlockAction` stają się krótkimi (10–20 linii) adapterami wywołującymi metody `book.*` i `commitScenesMutation`.

---

### Etap 4: Eliminacja duplikacji w `internal/views/` (`view_editor_context_panel.go` ↔ `view_text_editor.go` ↔ `view_studio.go`)

- **`internal/views/view_editor_scene_inspector.go` (nowy wspólny moduł dla `view_editor_context_panel.go` i `view_text_editor.go`)**:
  - `buildSceneInspectorData(s *state.AppState, sc *book.Scene) sceneInspectorData`: buduje w jednym miejscu opcje rozdziałów, media (`image`/`voice`/sekcje głosu/trim), walidację `Effects`, sugestie zmiennych (`BookVarsSpec`, `BookVarSuggestions`, `BookEnumSuggestions`, `BookTargetSuggestions`) oraz listę portów/wyborów (`buildChoiceRuleView`).
  - `ViewEditorContextPanel` i `ViewTextEditor` delegują budowanie danych sceny do `buildSceneInspectorData`, usuwając ~220 linii zduplikowanego kodu.
- **`internal/views/view_studio.go`**:
  - Usunięcie kopiowania pól z `ctxData`, `varsData`, `histData` do `studioTemplate`; `studioTemplate` trzyma wyłącznie własne pola kanwy/wyszukiwarki oraz osadzone chunki `ContextPanel`, `VariablesPanel`, `BreakdownPanel`, `HistoryPanel`.

---

### Etap 5: Aktualizacja testów, WASM i grafu projektu
- **`cmd/wasmplayer/main.go`**: zastąpienie `actions.EncodeWFPForm` / `actions.DecodeWFPFormBytes` przez `tpl.WriteFDPResponse` / `tpl.DecodeFDPBytes`.
- **`internal/server/server_test.go`, `server_editor_test.go`, `server_js_runtime_test.go`, `server_chaos_fuzz_test.go`**: odczyt metadanych z ramki `FDP` (`tpl.DecodeFDPResponse`) zamiast nagłówków HTTP `X-*` oraz zamiana `actions.EncodeWFPForm` na `tpl.EncodeFDPForm`.
- **`docs/project.graph`**: przebudowa grafu projektu po usunięciu `wfp.go`/`wfp.js`/`tpl_data_header.go` i wprowadzeniu `fdp.go`/`fdp.js`/`dispatcher.go`.
