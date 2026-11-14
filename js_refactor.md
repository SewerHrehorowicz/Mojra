# Plan: Synchronizacja Widoków MVC, Eliminacja `modal_` z Nazewnictwa i Backendu, Ujednolicenie Konwencji (`<Name>View`, `<name>_view`, `<name>_controller`, `<name>_state`, `<name>_actions`), Multi-Frame FDP z `---` & Zero Hardkodowanych Map

## 1. Co jest obecnie i jakie są problemy

1. **Wyciek sposobu prezentacji (`modal_`, `_panel`) do nazw plików, konstruktorów, struktur i backendu Go**:
   - Backend Go (`internal/state/`, `internal/actions/`, `internal/views/`, `internal/tpl/fdp.go`) wie o tym, czy dany widok jest aktualnie ostylowany jako modal czy panel boczny (`view_modal_export.go`, `state_modal_new_book.go`, `actions_modal_confirm.go`, `ModalConfirm`, `ModalExport`, `ModalNewBook`, `ModalID` w `FDPFrameMeta`, `#modal-scene-text` dla `text_editor`).
   - Sposób prezentacji (modal, panel boczny, pełny widok, split-screen) to wyłącznie kwestia CSS/szablonu — każdy moduł UI jest po prostu niezależnym widokiem `<Name>View` (`<name>_view`).

2. **Niespójne nazewnictwo konstruktorów (`View<Name>` vs `<Name>Controller`), plików i selektorów DOM wymuszające ręczną mapę `templateNameByStructType`**:
   - W JS sufiks roli jest na końcu (`<Name>Controller`, `<name>_controller.js`), a w Go prefiks jest na początku (`View<Name>`, `view_<name>.go`, `state_<name>.go`, `actions_<name>.go`).
   - W `app/templates/` panuje mieszanka sufiksów (`_view.tpl`, `_panel.tpl`, brak sufiksu, a `confirm` siedzi wewnątrz `player_view.tpl` jako `{def confirm_modal}`), przez co w `internal/tpl/tpl_render_struct.go` istnieje ręcznie hardkodowana mapa `templateNameByStructType`.
   - Identyfikatory DOM nie pasują do nazw widoków (`#modal-scene-text`, `#view-editor`, `#view-library`, `#view-player`, `#lan-qr-modal`, `#editor-help-modal`, `#modal-export-project`, `#modal-studio-project`).

3. **Pojedyncza ramka FDP i brak izolacji kontrolera (`internal/tpl/fdp.go`, `app/js/fdp.js`, `app/js/dispatcher.js`)**:
   - `WriteFDPResponse` i `Fdp.decode` obsługują tylko jedną ramkę (`target` + `key=val` + `\n\n` + `html`).
   - `Dispatcher.hydrate` i `notifyHydrated` rozsyłają klucze i wywołują `onHydrate` po wszystkich kontrolerach naraz zamiast po konkretnym kontrolerze wynikającym z konwencji `<name>Script` -> `<Name>Controller`.

4. **Brak odświeżania węzłów na płótnie WebGL po edycji w edytorze tekstowym lub inspektorze (`AutoSaveScene`, `SaveSceneTextModal`, `SaveScene`)**:
   - Po zmianie tytułu wyboru lub tytułu sceny w `text_editor_view` (`AutoSaveScene`), serwer nie wysyła zaktualizowanego stanu do `StudioController` (`studioGraphPayload`), `#editor-context-view` ani `TextEditorController` (`sceneId`).

5. **Brak przełączania zwiniętego edytora tekstowego (`#btn-expand-scene-text`) po zmianie węzła (`SelectStudioScene`)**:
   - `StudioController` nie posiada `onHydrate`, więc `#btn-expand-scene-text` zachowuje stare `data-scene` i stary tytuł `#scene-text-expand-title`, przez co ponowne rozwinięcie otwiera stary węzeł i rozdział.

6. **Zbędne uderzanie do backendu podczas przesuwania i zoomowania canvasu (`CanvasDrag.scheduleViewportSave`)**:
   - `CanvasDrag.scheduleViewportSave()` wysyła `POST /api/action` (`SaveEditorState`) przy każdym panowaniu i zoomowaniu kamery.

---

## 2. Rozwiązania

1. **Jednolita Konwencja Nazewnictwa z sufiksem roli na końcu (zero `modal_`, zero `_panel`, zero hardkodowanych map)**:
   - Każdy z 19 widoków ma czysty domenowy rdzeń `<name>` (`snake_case` / `camelCase` / `PascalCase` / `kebab-case`):
     - Widok Go: `internal/views/<name>_view.go` -> konstruktor **`<PascalName>View`** (`AppShellView`, `StudioView`, `SceneView`, `TextEditorView`, `EditorContextView` itd.) oraz struktury `<camelName>Template` i `<camelName>Script`
     - Szablon TPL: `app/templates/<name>_view.tpl`
     - Kontroler JS: `app/js/controllers/<name>_controller.js` (`<PascalName>Controller`)
     - Stan Go: `internal/state/<name>_state.go` (`<PascalName>State`: `s.Confirm`, `s.Export`, `s.NewBook`)
     - Akcje Go: `internal/actions/<name>_actions.go`
     - Root DOM ID: `#<kebab-name>-view` (`#text-editor-view`, `#studio-view`, `#library-view`, `#player-view`, `#editor-context-view`, `#editor-breakdown-view`, `#editor-variables-view`, `#editor-history-view`, `#confirm-view`, `#export-view`, `#new-book-view`, `#add-book-view`, `#editor-help-view`, `#qr-view`, `#studio-projects-view`).
   - **Usunięcie `templateNameByStructType` z `internal/tpl/tpl_render_struct.go`**:
     - `chunkTemplateName(v any)` wylicza nazwę szablonu czysto po konwencji z nazwy typu `*<camelName>Template`: ucięcie `"Template"`, `camelCase -> snake_case` i dodanie `"_view"`.
   - **Wyliczanie nazwy kontrolera czysto po konwencji w `internal/tpl/fdp.go`**:
     - `scriptControllerName(script any)` wylicza nazwę kontrolera z typu `*<camelName>Script`: ucięcie `"Script"`, wielka pierwsza litera i dodanie `"Controller"`.

2. **Multi-Frame FDP rozdzielany `\n---\n` (1 ramka = 1 widok/kontroler, zero kompatybilności wstecznej)**:
   - Odpowiedź z `POST /api/action` składa się z 1..N niezależnych ramek FDP rozdzielonych linią `---` (`\n---\n`):
     ```text
     controller=EditorContextController
     target=#editor-context-view
     selectedSceneId=scena_2
     bookVarsSpec=...
     _body=html

     <aside id="editor-context-view">...</aside>
     ---
     controller=TextEditorController
     target=#text-editor-view
     sceneId=scena_2
     isScreenplay=false
     _body=html

     <div id="text-editor-view">...</div>
     ---
     controller=StudioController
     target=none
     selectedSceneId=scena_2
     sceneTitle=Rozdzia%C5%82%202
     studioGraphPayload=viewport | pan_x:0...
     ```
   - `Dispatcher.applyFrame(frame, action, params)` hydratuje **wyłącznie** `this.controllers[frame.controller]`, podmienia `frame.target` (jeśli `!= "none"`) i wywołuje `onHydrate` wyłącznie na tym kontrolerze. Kolejność ramek nie ma znaczenia.

3. **Zamknięty stan `StudioController` + synchronizacja `TextEditorController` + zero requestów przy pan/zoom**:
   - `StudioController.onHydrate` aktualizuje przemianowane ID węzła w `CanvasModel`, `#btn-expand-scene-text` (`data-scene`), `#scene-text-expand-title` (`textContent`), `CanvasModel.selectedNodeIds` oraz `#studio-graph-data` (`parseStudioGraphPayload()`, `EditorWires.route()`, `applyTransform()`).
   - `TextEditorController` i `EditorContextController` obsługują live-sync 120fps na węźle WebGL podczas pisania (`input` na tytule sceny i tytule wyboru), a `TextEditorController.onHydrate` aktualizuje `input[name="scene_id"].value` po `AutoSaveScene`.
   - Usunięcie `CanvasDrag.scheduleViewportSave()` oraz `CanvasModel.viewportSaveTimer`.

---

## 3. Pełna lista plików, konstruktorów, struktur i identyfikatorów DOM do zsynchronizowania wg konwencji

### 3.1 Triady Widok – Szablon – Kontroler – DOM ID (19 widoków, sufiks `*View` na końcu, zero `modal_`)

| # | Rdzeń `<name>` | Widok Go (`internal/views/`) | Konstruktor Go (`*View`) | Struktury Go (`*Template` / `*Script`) | Szablon (`app/templates/`) | Kontroler JS (`app/js/controllers/`) | Root DOM ID |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | `app_shell` | `view_app_shell.go` -> **`app_shell_view.go`** | `ViewAppShell` -> **`AppShellView`** | `appShellTemplate` / `appShellScript` | `app_shell.tpl` -> **`app_shell_view.tpl`** | `app_shell_controller.js` (`AppShellController`) | `#app` |
| 2 | `topbar` | `view_topbar.go` -> **`topbar_view.go`** | `ViewTopbar` -> **`TopbarView`** | `topbarTemplate` / `nil` | `topbar.tpl` -> **`topbar_view.tpl`** | `topbar_controller.js` (`TopbarController`) | `#topbar` |
| 3 | `library` | `view_library.go` -> **`library_view.go`** | `ViewLibrary` -> **`LibraryView`** | `libraryTemplate` / `libraryScript` | `library_view.tpl` | `library_controller.js` (`LibraryController`) | `#view-library` -> **`#library-view`** |
| 4 | `player` | `view_player.go` -> **`player_view.go`** | `ViewPlayer` -> **`PlayerView`** | `playerTemplate` / `playerScript` | `player_view.tpl` (wydzielić `confirm`) | `player_controller.js` (`PlayerController`) | `#view-player` -> **`#player-view`** |
| 5 | `scene` | `view_scene.go` -> **`scene_view.go`** | `ViewScene` -> **`SceneView`** | `sceneTemplate` / `sceneScript` | `scene_view.tpl` | `scene_controller.js` (`SceneController`) | `.scene-card` |
| 6 | `audio_deck` | `view_audio_deck.go` -> **`audio_deck_view.go`** | `ViewAudioDeck` -> **`AudioDeckView`** | `audioDeckTemplate` / `audioDeckScript` | `audio_deck.tpl` -> **`audio_deck_view.tpl`** | `audio_deck_controller.js` (`AudioDeckController`) | `#audio-deck` |
| 7 | `studio` | `view_studio.go` -> **`studio_view.go`** | `ViewStudio` -> **`StudioView`** | `studioTemplate` / `studioScript` | `studio_view.tpl` | `studio_controller.js` (`StudioController`) | `#view-editor` -> **`#studio-view`** |
| 8 | `text_editor` | `view_text_editor.go` -> **`text_editor_view.go`** | `ViewTextEditor` -> **`TextEditorView`** | `textEditorTemplate` / `textEditorScript` | `text_editor.tpl` -> **`text_editor_view.tpl`** | `text_editor_controller.js` (`TextEditorController`) | `#modal-scene-text` -> **`#text-editor-view`** |
| 9 | `editor_context` | `view_editor_context_panel.go` -> **`editor_context_view.go`** | `ViewEditorContextPanel` -> **`EditorContextView`** | `editorContextTemplate` / `editorContextScript` | `editor_context_panel.tpl` -> **`editor_context_view.tpl`** | `editor_context_panel_controller.js` -> **`editor_context_controller.js`** (`EditorContextController`) | `#editor-context-panel` -> **`#editor-context-view`** |
| 10 | `editor_breakdown` | `view_editor_breakdown.go` -> **`editor_breakdown_view.go`** | `ViewEditorBreakdown` -> **`EditorBreakdownView`** | `editorBreakdownTemplate` / `nil` | `editor_breakdown_panel.tpl` -> **`editor_breakdown_view.tpl`** | `editor_breakdown_controller.js` (`EditorBreakdownController`) | `#editor-breakdown-panel` -> **`#editor-breakdown-view`** |
| 11 | `editor_variables` | `view_editor_variables.go` -> **`editor_variables_view.go`** | `ViewEditorVariables` -> **`EditorVariablesView`** | `editorVariablesTemplate` / `nil` | `editor_variables_panel.tpl` -> **`editor_variables_view.tpl`** | `editor_variables_controller.js` (`EditorVariablesController`) | `#editor-variables-panel` -> **`#editor-variables-view`** |
| 12 | `editor_history` | `view_editor_history.go` -> **`editor_history_view.go`** | `ViewEditorHistory` -> **`EditorHistoryView`** | `editorHistoryTemplate` / `nil` | `editor_history_panel.tpl` -> **`editor_history_view.tpl`** | `editor_history_controller.js` (`EditorHistoryController`) | `#editor-history-panel` -> **`#editor-history-view`** |
| 13 | `add_book` | `view_modal_add_book.go` -> **`add_book_view.go`** | `ViewModalAddBook` -> **`AddBookView`** | `addBookTemplate` / `nil` | `modal_add_book.tpl` -> **`add_book_view.tpl`** | `modal_add_book_controller.js` -> **`add_book_controller.js`** (`AddBookController`) | `#modal-add-book` -> **`#add-book-view`** |
| 14 | `confirm` | `view_modal_confirm.go` -> **`confirm_view.go`** | `ViewModalConfirm` -> **`ConfirmView`** | `confirmTemplate` / `nil` | `{def confirm_modal}` -> **`confirm_view.tpl`** | `modal_confirm_controller.js` -> **`confirm_controller.js`** (`ConfirmController`) | `#modal-confirm` -> **`#confirm-view`** |
| 15 | `editor_help` | `view_modal_editor_help.go` -> **`editor_help_view.go`** | `ViewModalEditorHelp` -> **`EditorHelpView`** | `editorHelpTemplate` / `nil` | `modal_editor_help.tpl` -> **`editor_help_view.tpl`** | `modal_editor_help_controller.js` -> **`editor_help_controller.js`** (`EditorHelpController`) | `#editor-help-modal` -> **`#editor-help-view`** |
| 16 | `export` | `view_modal_export.go` -> **`export_view.go`** | `ViewModalExport` -> **`ExportView`** | `exportTemplate` / `nil` | `modal_export.tpl` -> **`export_view.tpl`** | `modal_export_controller.js` -> **`export_controller.js`** (`ExportController`) | `#modal-export-project` -> **`#export-view`** |
| 17 | `new_book` | `view_modal_new_book.go` -> **`new_book_view.go`** | `ViewModalNewBook` -> **`NewBookView`** | `newBookTemplate` / `nil` | `modal_new_book.tpl` -> **`new_book_view.tpl`** | `modal_new_book_controller.js` -> **`new_book_controller.js`** (`NewBookController`) | `#modal-new-book` -> **`#new-book-view`** |
| 18 | `qr` | `view_modal_qr.go` -> **`qr_view.go`** | `ViewModalQR` -> **`QRView`** | `qrTemplate` / `qrScript` | `modal_qr.tpl` -> **`qr_view.tpl`** | `modal_qr_controller.js` -> **`qr_controller.js`** (`QRController`) | `#lan-qr-modal` -> **`#qr-view`** |
| 19 | `studio_projects` | `view_modal_studio_projects.go` -> **`studio_projects_view.go`** | `ViewModalStudioProjects` -> **`StudioProjectsView`** | `studioProjectsTemplate` / `nil` | `modal_studio_projects.tpl` -> **`studio_projects_view.tpl`** | `modal_studio_projects_controller.js` -> **`studio_projects_controller.js`** (`StudioProjectsController`) | `#modal-studio-project` -> **`#studio-projects-view`** |

### 3.2 Pliki Stanu (`internal/state/`) — sufiks `_state.go` na końcu, zero `modal_`

| Obecny plik (`internal/state/`) | Docelowy plik (`internal/state/`) | Zmiana w `AppState` |
| :--- | :--- | :--- |
| `state_app_shell.go` | **`app_shell_state.go`** | `s.Modals` -> **`s.Overlays`** (`OverlayFlags`) |
| `state_library.go` | **`library_state.go`** | `s.Library` |
| `state_player.go` | **`player_state.go`** | `s.Player` |
| `state_editor.go` | **`studio_state.go`** | `s.Editor` |
| `state_text_editor.go` | **`text_editor_state.go`** | `s.Editor.SceneText` |
| `state_modal_confirm.go` | **`confirm_state.go`** | `s.ModalConfirm` -> **`s.Confirm`** (`ConfirmState`) |
| `state_modal_export.go` | **`export_state.go`** | `s.ModalExport` -> **`s.Export`** (`ExportState`) |
| `state_modal_new_book.go` | **`new_book_state.go`** | `s.ModalNewBook` -> **`s.NewBook`** (`NewBookState`) |

### 3.3 Pliki Akcji (`internal/actions/`) — sufiks `_actions.go` na końcu, zero `modal_`

| Obecny plik (`internal/actions/`) | Docelowy plik (`internal/actions/`) |
| :--- | :--- |
| `actions_app_shell.go` | **`app_shell_actions.go`** |
| `actions_topbar.go` | **`topbar_actions.go`** |
| `actions_library.go` | **`library_actions.go`** |
| `actions_player.go` | **`player_actions.go`** |
| `actions_editor.go` | **`studio_actions.go`** |
| `actions_editor_scene.go` | **`studio_scene_actions.go`** |
| `actions_editor_chapter.go` | **`studio_chapter_actions.go`** |
| `actions_editor_voice.go` | **`studio_voice_actions.go`** |
| `actions_text_editor.go` | **`text_editor_actions.go`** |
| `actions_editor_breakdown.go` | **`editor_breakdown_actions.go`** |
| `actions_editor_variables.go` | **`editor_variables_actions.go`** |
| `actions_editor_history.go` | **`editor_history_actions.go`** |
| `actions_modal_add_book.go` | **`add_book_actions.go`** |
| `actions_modal_confirm.go` | **`confirm_actions.go`** |
| `actions_modal_export.go` | **`export_actions.go`** |
| `actions_modal_new_book.go` | **`new_book_actions.go`** |
| `actions_modal_qr.go` | **`qr_actions.go`** |
| `actions_modal_studio_projects.go` | **`studio_projects_actions.go`** |

---

## 4. Zmiany w kodzie (krok po kroku)

### A. `internal/tpl/tpl_render_struct.go`, `internal/tpl/tpl_render.go` & `internal/tpl/fdp.go`
- **`internal/tpl/tpl_render_struct.go`**:
  - Usunąć mapę `templateNameByStructType`.
  - W `chunkTemplateName(v any) string`: odczytać `rt.Name()`, uciąć sufiks `"Template"`, zamienić `camelCase` na `snake_case` przez `pascalToSnake(...)` i dodać `"_view"`.
- **`internal/tpl/fdp.go`**:
  - Usunąć `ModalID` / `fdpKeyModalID` z `FDPFrameMeta`.
  - Dodać `scriptControllerName(script any) string`: odczytać `rt.Name()`, uciąć sufiks `"Script"`, zamienić pierwszą literę na wielką i dodać `"Controller"`.
  - Dodać `FDPFrame`, `WriteFDPFrames(w io.Writer, frames ...FDPFrame) error` oraz `DecodeFDPFrames(raw []byte)` z jawnym separatorem `\n---\n` i nagłówkiem `controller=<Name>Controller`.

### B. `internal/actions/dispatcher.go`
- Zwracać wieloramkowe odpowiedzi FDP (`WriteFDPFrames`):
  - `AutoSaveScene` (w `state.ViewEditor`): ramka `#editor-context-view` (`EditorContextView(s)`) + ramka skryptowa `TextEditorScriptView(s)` (`target=none`, jeśli edytor tekstu otwarty) + ramka skryptowa `StudioScriptView(s)` (`target=none`).
  - Zapis / nawigacja w `text_editor` oraz `SelectStudioScene` (gdy edytor tekstu otwarty): ramka `#editor-context-view` (`EditorContextView(s)`) + ramka `TextEditorView(s)` + ramka skryptowa `StudioScriptView(s)` (`target=none`).

### C. `app/js/fdp.js` & `app/js/dispatcher.js`
- **`Fdp.decode(rawStr)` (`app/js/fdp.js`)**:
  - Dzieli odpowiedź po `\n---\n` na tablicę niezależnych ramek.
- **`Dispatcher.hydrate(frame)` & `Dispatcher.notifyHydrated(frame, action, params)` (`app/js/dispatcher.js`)**:
  - Aktualizuje `ctrl.data` i wywołuje `ctrl.onHydrate(frame, action, params)` wyłącznie na kontrolerze `this.controllers[frame.controller]`.

### D. `app/js/controllers/studio_controller.js`, `text_editor_controller.js`, `editor_context_controller.js`, `app_shell_controller.js`, `canvas_drag.js` & `canvas_model.js`
- Przenieść aktualizację przemianowanego `sceneId` w `CanvasModel` z `AppShellController.onHydrate` do `StudioController.onHydrate`.
- Dodać `StudioController.onHydrate` synchronizujące `#btn-expand-scene-text`, `#scene-text-expand-title`, `CanvasModel.selectedNodeIds` oraz `#studio-graph-data` (`parseStudioGraphPayload()`, `EditorWires.route()`, `applyTransform()`).
- Dodać `TextEditorController.onHydrate` (aktualizacja `input[name="scene_id"]`) oraz `onSceneTitleInput` / `onChoiceLabelInput` w `TextEditorController` i `EditorContextController`.
- Usunąć `CanvasDrag.scheduleViewportSave()` i `CanvasModel.viewportSaveTimer`.

### E. `ARCH.md`, `AGENTS.md` & Testy (`internal/server/*_test.go`, `internal/audit/*`)
- Zaktualizować tabelę triad (`<Name>View`, `<name>_view.go`, `<name>_view.tpl`, `<name>_controller.js`) i specyfikację Multi-Frame FDP w `ARCH.md`.
- Zaktualizować selektory i nazwy plików w `app/styles/*.tss`, `internal/tpl/bundle.go`, `internal/server/server.go`, `internal/audit/audit_js_security.go` oraz testach.
