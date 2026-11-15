# Docelowa Architektura `engine/` + `domain/` (Jednolity Układ Modułowy bez `app/`)

## 1. Główne założenia podziału odpowiedzialności (SoC)

Cały system **Mojra** składa się z dwóch warstw najwyższego poziomu: **`engine/`** oraz **`domain/`**, w których każdy moduł ma ten sam płaski, symetryczny podział katalogów (`state/`, `views/`, `actions/`, `templates/`, `styles/`, `controllers/` oraz opcjonalnie `scripts/` dla pomocniczych modułów JS):

1. **`engine/` (Wymienny silnik aplikacyjny — zero szablonów `.tpl`, zero stylów `.tss`, zero pojęcia książki, sceny czy Mojry)**:
   * **`engine/osbridge`**: Jedyna warstwa dotykająca systemu operacyjnego, systemu plików (`os.*`), procesów (`os/exec`), bezpiecznego pobierania zasobów z sieci z ochroną SSRF (`FetchURLBytes`), surowych archiwów (`archive/zip`), `window.localStorage` / `URL.createObjectURL` (w WASM), okien natywnych i audio capture. Wystawia czysto bajtowy, bezdomenowy interfejs `Storage` (`ReadBytes`, `WriteAtomic`, `WriteAt`, `ReadArchive`, `CreateBlobURL`, `FetchURLBytes`) z dwiema implementacjami: `storage_fs.go` (system plików OS: macOS, Windows, Linux, Android) oraz `storage_wasm.go` (pamięć + `localStorage`).
   * **`engine/transport`**: Jednolity kontrakt `ActionRequest -> []FDPFrame` (wieloramkowy protokół `FDP` `text/x-fdp;charset=UTF-8`: `controller=<Name>Controller`, `target=...`, płaskie `key=val` ze `ScriptData` + `_body=html\n\n` z `TemplateData`; zero wycofanego `WFP` i zero nagłówków `X-*`). Posiada dwa adaptery: `http_server.go` oraz `wasm_bridge.go`.
   * **`engine/templates`**: Bezdomenowy kompilator i renderer `.tpl` / `.tss` (z wstrzykiwanym z zewnątrz `ProseFormatter func(string) string` dla `{prose ...}`) oraz uniwersalny bundler (`bundle.go`) składający aplikację z `engine.FS` (`fdp.js`, `dispatcher.js`, wybrany adapter `transport_http.js` lub `transport_wasm.js`) oraz listy przekazanych `fs.FS` modułów odczytanej z `modules.cfg` danej powłoki. Zero wiedzy o składni `.playtxt` (`if:`, `set:`, `{{...}}`) i zero `SanitizePlainText`.
   * **`engine/scripts/`**: Wyłącznie kliencki mostek transportowy i dyspozytor (`fdp.js`, `dispatcher.js`, `transport_http.js`, `transport_wasm.js`). Zero szablonów i zero stylów w `engine/`.

2. **`domain/` (Pionowe moduły domenowe Mojry — każdy z własnym `state/`, `views/`, `actions/`, `templates/`, `ui_components/`, `styles/`, `controllers/` i jednokierunkowym, bezcyklicznym grafem zależności `common -> {player, library, studio} -> shell/<variant> -> cmd/mojra`)**:
   * **Zero pojęć prezentacyjnych typu „modal” czy „overlay” w domenie**: Model, stan (`state`), akcje (`actions`) i widoki (`views`) operują wyłącznie na pojęciu widoku (`ConfirmView`, `AddBookView`, `NewBookView`, `QRView`, `TextEditorView`, `ExportView`, `TextEditorState`). Sposób wyświetlenia widoku zależy wyłącznie od arkusza `.tss` i szablonu `.tpl`, nigdy od modelu domenowego.
   * **Czysty kontrakt 1:1 w `views/`, `templates/`, `controllers/` oraz wydzielone `ui_components/`**:
     * W `templates/` każdego modułu znajdują się **wyłącznie** główne szablony widoków `<name>_view.tpl` odpowiadające 1:1 plikom `views/<name>_view.go` oraz `controllers/<name>_controller.js`.
     * Współdzielone fragmenty i komponenty `.tpl` niebędące samodzielnymi widokami (brak własnego `*_view.go` i `*_controller.js`) mieszkają w płaskim podkatalogu `ui_components/*.tpl` danego modułu (analogicznie jak `scripts/` towarzyszy `controllers/`).
   * **Ścisła izolacja modułów (brak zależności zwrotnych)**:
     * `domain/common/actions/confirm_actions.go` nigdy nie importuje `player`, `library` ani `studio` — wykonanie potwierdzonej akcji odbywa się przez zarejestrowany handler/callback w `ConfirmState`.
     * `domain/player` nigdy nie importuje `studio` ani `library` — tryb podglądu historii jest czystą flagą w `PlayerState`, a przełączenie widoku z powrotem do edytora należy do `shell/studio_app`.
   * **`domain/book/model` & `domain/book/actions`**: Czyste struktury danych książki oraz operacje wyłącznie na `[]byte` i `osbridge.Storage` (parser `.playtxt`/`STBC`, warunki `if:/set:`, kodek `.sav`, walidacja mediów, `SanitizePlainText`, serializacja, mutacje, historia, paczkowanie i pobieranie przez `Storage.FetchURLBytes`). Zero bezpośrednich wywołań `os.*`, `os/exec` i `net/http` w `domain/`.
   * **`domain/common` (Wspólny fundament domeny Mojry dla obu powłok i wszystkich modułów)**:
     * `state/`: `ui_state.go` (`Settings`, `RuntimeConfig`, `Toast`, `NativeCmd`) oraz `confirm_state.go` (`ConfirmState` z bezdomenowym callbackiem akceptacji używany przez `player`, `library` i `studio`).
     * `views/`: `ui_view.go` (`UIView` + `uiScript` oraz wspólne helpery i18n `LangOf`, `Tr`, `Trf`, `BoolAttr`) i `confirm_view.go` (`ConfirmView` + `confirmScript`).
     * `actions/`: `ui_actions.go` (`ToggleTheme`, `ToggleUIScale`, `ToggleImageless`, `SetLanguage`, `SetScrollSensitivity`, `SetDeviceMode`, `ToggleFullscreen`, `StartWindowDrag`, `QuitApp`) i `confirm_actions.go` (`ConfirmAccept`, `ConfirmCancel`).
     * `templates/`: `ui_view.tpl` (globalne kontenery `#toast-banner`, `#ui-tooltip` i opcjonalny pasek dev preview — 1:1 z `ui_view.go` i `ui_controller.js`) oraz `confirm_view.tpl` (1:1 z `confirm_view.go` i `confirm_controller.js`).
     * `ui_components/`: `components.tpl` (prymitywy formularzy `@component`: input, toggle, radio, `.custom-select`) oraz `scene_choice_arrow.tpl` (strzałka/rozgałęzienie wyboru współdzielona między `player` a `studio`).
     * `styles/`: `colors.tss` (tokeny `light-dark()`), `base.tss` (reset, typografia, `.app-root`, `.workspace`, `.modals-root`, `.toast`, `.ui-tooltip`, 4-narożnikowy wskaźnik aktywności), `forms.tss` (minimalistyczny system formularzy oraz kontenery okien dialogowych).
     * `controllers/`: `ui_controller.js` (motyw, skala UI, tryb bez obrazów, zen, tooltipy `[data-hint]`, toasty, skróty `Escape`/`F12`) oraz `confirm_controller.js`.
     * `icons/` i `fonts/`: wszystkie ikony SVG (`/icons/*.svg`) i fonty TTF (`/fonts/*.ttf`).
   * **`domain/player`**: Autonomiczny czytnik **jednej otwartej książki** (`100%` współdzielony między obiema powłokami: `studio_app` oraz `book_player`, zero wiedzy o `studio`, `library` czy `wasm`).
   * **`domain/library`**: Katalog wielu książek, dodawanie książki, tworzenie nowej książki, widok QR.
   * **`domain/studio`**: Playbook Studio — płótno węzłowe WebGL, inspektor, edytor tekstu sceny, zmienne, breakdown, historia, pomoc, eksport, z własnym `ui_components/` (`choice_triple_stack.tpl`, `syn_autocomplete_dropdown.tpl`, `slugline_helper_pills.tpl`, `editor_book_settings_form.tpl`).
   * **`domain/shell` (`studio_app` & `book_player` — dwie powłoki aplikacyjne niezależne od platformy docelowej)**:
     * Wspólny kod UI (`Settings`: motyw, skala UI, tryb bez obrazów, toasty — `ui_controller.js`, `ui_view.tpl`, `ui_state.go`, `ui_view.go`, `ui_actions.go`) istnieje **tylko raz** w `domain/common/`.
     * **`domain/shell/studio_app` (Player + Library + Studio)**: Pełna aplikacja autorska i biblioteka (`modules.cfg`: `common, player, library, studio`), zawierająca układ `#topbar + #workspace` (`shell_view.go` <-> `shell_view.tpl` <-> `shell_controller.js` oraz `topbar_view.go` <-> `topbar_view.tpl` <-> `topbar_controller.js`) oraz prekompilowane szablony runtime'ów drugiej powłoki (`book_player.wasm`, docelowo `book_player.exe` / `book_player.app`), do których akcja `Export` w skompilowanym Studio wstrzykuje kontener `STBC` z książką (bez udziału kompilatora Go u użytkownika).
     * **`domain/shell/book_player` (Samodzielny Player jednej książki)**: Lekka aplikacja pojedynczej książki (`modules.cfg`: `common, player`, ekran startowy okładki + `#player-view` bez topbaru: `shell_view.go` <-> `shell_view.tpl` <-> `shell_controller.js`). Ta sama powłoka buduje się zarówno do Web/WASM (`GOOS=js GOARCH=wasm -tags book_player`), jak i do natywnego pliku wykonywalnego książki (`-tags book_player` dla `.exe`, `.app`, Linux) wypuszczanego ze Studio — różnica między `.wasm` a `.exe` leży wyłącznie w adapterach `engine/osbridge` i `engine/transport`.

---

## 2. Drzewo katalogów i plików

```text
engine/                                   # Wymienny silnik (zero zależności od domain/, zero .tpl, zero .tss)
  embed.go                                # //go:embed scripts/*.js -> FS
  osbridge/                               # Bezdomenowa abstrakcja systemu operacyjnego, plików i pamięci
    storage.go                            # Czysto bajtowe interfejsy ByteStorage, BytePatcher, ArchiveReader, BlobStore
    storage_fs.go                         # System plików OS (macOS/Windows/Linux/Android): ReadFile, zapis .tmp + rename, WriteAt, zip, UserConfigDir
    storage_wasm.go                       # Przeglądarka WASM: mapa []byte w pamięci, surowy KV w window.localStorage, Blob URL
    launcher.go                           # Uruchamianie okna natywnego lub przeglądarki
    window_darwin.go                      # Natywne okno macOS (Cocoa + WKWebView)
    window_windows.go                     # Natywne okno Windows (Win32 + WebView)
    window_linux.go                       # Natywne okno Linux (Chromium/Edge --app)
    window_other.go                       # Fallback dla pozostałych platform
    audio_capture_*.go                    # Nagrywanie strumienia PCM/audio z mikrofonu
  templates/                              # Bezdomenowy kompilator .tpl/.tss i uniwersalny bundler
    tpl.go                                # Kompilator i renderer szablonów .tpl (zero logiki .playtxt i zero SanitizePlainText)
    tss.go                                # Kompilator arkuszy stylów .tss
    bundle.go                             # Uniwersalny bundler składający aplikację z listy przekazanych fs.FS modułów (punkt wejścia: shell_view.tpl)
  transport/                              # Protokół komunikacji UI <-> Go (FDP — zero WFP i zero X-*)
    fdp.go                                # Dekoder/enkoder wieloramkowego text/x-fdp (pooled sync.Pool)
    contract.go                           # ActionFrame, FDPFrame (controller, target, ScriptData, HTMLBody)
    http_server.go                        # Adapter HTTP (POST /api/action + CSRF/SameOrigin)
    wasm_bridge.go                        # Adapter syscall/js dla WASM (ten sam kontrakt ActionFrame -> FDPFrame)
  scripts/
    fdp.js                                # Kodek wieloramkowego protokołu FDP po stronie przeglądarki
    dispatcher.js                         # Uniwersalny runtime kontrolerów, bindowanie zdarzeń, hydratacja ctrl.data i podmiana targetu
    transport_http.js                     # Mostek fetch("POST /api/action")
    transport_wasm.js                     # Mostek __wasmAction() + surowy zapis/odczyt localStorage
  qr.go                                   # Generator kodów QR (SVG/macierz)
  i18n.go                                 # Bezdomenowy mechanizm ładowania plików .cfg i interpolacji T/Tf/Plural
  telemetry.go                            # Metryki wydajności i logowanie

domain/                                   # Logika biznesowa i pionowe moduły UI Mojry (zero net/http, zero os.*)
  book/
    model/                                # Czyste struktury danych i tokeny (100% WASM-safe)
      book.go                             # Book, Chapter, Scene, Choice, ChoiceCondition, ChoiceMutation
      variables.go                        # VariableDef, VariableType, EnumDef
      save.go                             # SaveState, VisitedScene, AudioProgress
      layout.go                           # EditorLayout, NodePos, stała siatki EditorGridSize
      tokens.go                           # Stałe kluczy .playtxt, operatorów, nagłówka STSV\x01
    actions/                              # Operacje na modelu książki (przez osbridge.Storage / []byte)
      parser.go                           # Parser .playtxt ([]byte -> *model.Book) + BFS ordering (graph.go)
      loader.go                           # Ładowanie książek i biblioteki przez osbridge.Storage
      stbc.go                             # Dekoder/enkoder binarnego kontenera STBC
      conditions.go                       # Ewaluacja warunków if: i mutacji set: na SaveState (variables_prose.go, variables_validate.go)
      save_codec.go                       # Dekodowanie/kodowanie binarnego STSV\x01 ([]byte <-> SaveState)
      media_verify.go                     # Walidacja magic-bytes obrazów (.png/.jpg/.webp) i audio (.ogg/.flac/.mp3, audio_flac.go)
      sanitize.go                         # SanitizePlainText (.playtxt), SanitizeSceneID, RenderSafeProseHTML
      serializer.go                       # Serializacja *model.Book -> .playtxt []byte oraz .editor.cfg []byte
      mutator.go                          # Operacje edycyjne na grafie (dodaj/usuń/zmień scenę, wybór, rozdział — mutator_scene.go)
      history.go                          # Zarządzanie migawkami historii zmian i gałęziami (history_ops.go, history_branches.go)
      packager.go                         # Budowanie archiwum .playbook oraz wstrzykiwanie STBC do prekompilowanych runtime'ów book_player (.wasm / .exe / .app)
      exporters.go                        # Eksportery PDF (book_pdf.go), Screenplay (screenplay_*.go), VO (vo_export.go) i atlasu MSDF (studio_atlas.go)
      beta_license.go                     # Weryfikacja licencji beta

  i18n/                                   # Słowniki tłumaczeń domeny (pl.cfg, en.cfg) i stałe kluczy TrKey (keys.go)

  common/                                 # Wspólny fundament domeny (Confirm, globalny stan UI: motyw/skala/toasty, zasoby i system designu)
    embed.go                              # //go:embed templates/*.tpl ui_components/*.tpl styles/*.tss controllers/*.js icons/* fonts/* -> FS
    state/
      ui_state.go                         # Wspólny stan UI (Settings, RuntimeConfig, Toast, NativeCmd — używany w obu powłokach)
      confirm_state.go                    # ConfirmState (wspólny dla Player, Library i Studio)
    views/
      ui_view.go                          # Wspólny UIScriptView + uiScript oraz helpery i18n (LangOf, Tr, Trf, BoolAttr)
      confirm_view.go                     # ConfirmView + confirmScript
    actions/
      ui_actions.go                       # ToggleTheme, ToggleUIScale, ToggleImageless, SetLanguage, SetScrollSensitivity, SetDeviceMode, ToggleFullscreen, QuitApp
      confirm_actions.go                  # ConfirmModalAccept, CloseConfirmModal
    templates/
      confirm_view.tpl                    # Wyłącznie główny widok potwierdzenia (1:1 z confirm_view.go i confirm_controller.js)
    ui_components/
      components.tpl                      # @component dla minimalistycznych kontrolek formularzy (input, toggle, radio, custom-select)
      scene_choice_arrow.tpl              # Strzałka/ikona rozgałęzienia wyboru współdzielona między Player a Studio
    styles/
      colors.tss                          # Tokeny kolorów light-dark()
      base.tss                            # Reset, typografia, .app-root, .workspace, .modals-root, .toast, .ui-tooltip, 4-narożnikowy wskaźnik aktywności
      forms.tss                           # Bazowe style kontrolek formularzy oraz kontenery okien dialogowych
    controllers/
      ui_controller.js                    # Kontroler motywu, skali UI, trybu bez obrazów, zen, tooltipów [data-hint], toastów i skrótów globalnych (1x dla obu powłok)
      confirm_controller.js               # Kontroler widoku potwierdzenia
    icons/                                # Ikony SVG (/icons/*.svg)
    fonts/                                # Fonty TTF (/fonts/*.ttf)

  player/                                 # Czytnik JEDNEJ książki (100% wspólny dla studio_app oraz book_player)
    embed.go                              # //go:embed templates/*.tpl styles/*.tss controllers/*.js -> FS
    state/
      player_state.go                     # PlayerState (Book, Save, AudioSpeed, AutoChoice, ZenMode, FocusedHistoryIdx)
    views/
      player_view.go                      # PlayerView + playerScript
      scene_view.go                       # SceneView + sceneScript
      audio_deck_view.go                  # AudioDeckView + audioDeckScript
    actions/
      player_actions.go                   # ChooseOption, RestartBook, SetAudioTime, ToggleAutoChoice, RewindToScene
    templates/
      player_view.tpl
      scene_view.tpl
      audio_deck_view.tpl
    styles/
      player.tss
      audio_deck.tss
    controllers/
      player_controller.js
      scene_controller.js
      audio_deck_controller.js

  library/                                # Katalog wielu książek
    embed.go                              # //go:embed templates/*.tpl styles/*.tss controllers/*.js -> FS
    state/
      library_state.go                    # LibraryState (Books []*model.Book, ExternalPaths, ActiveFilter)
      new_book_state.go                   # NewBookState
    views/
      library_view.go                     # LibraryView + libraryScript
      add_book_view.go                    # AddBookView + addBookScript
      new_book_view.go                    # NewBookView + newBookScript
      qr_view.go                          # QRView + qrScript
    actions/
      library_actions.go
      add_book_actions.go
      new_book_actions.go
      qr_actions.go
    templates/
      library_view.tpl
      add_book_view.tpl
      new_book_view.tpl
      qr_view.tpl
    styles/
      library.tss
    controllers/
      library_controller.js
      add_book_controller.js
      new_book_controller.js
      qr_controller.js

  studio/                                 # Playbook Studio — edytor
    embed.go                              # //go:embed templates/*.tpl ui_components/*.tpl styles/*.tss controllers/*.js scripts/** studio-atlas.png -> FS
    state/
      studio_state.go                     # StudioState (SelectedScenes, Pan/Zoom, UndoStack, PreviewVars)
      text_editor_state.go                # TextEditorState
      export_state.go                     # ExportState
    views/
      studio_view.go
      studio_projects_view.go
      node_canvas_view.go
      editor_context_view.go
      text_editor_view.go
      editor_breakdown_view.go
      editor_variables_view.go
      editor_history_view.go
      editor_help_view.go
      export_view.go
    actions/
      studio_actions.go
      studio_chapter_actions.go
      studio_projects_actions.go
      studio_scene_actions.go
      studio_voice_actions.go
      text_editor_actions.go
      editor_variables_actions.go
      editor_breakdown_actions.go
      editor_history_actions.go
      export_actions.go
    templates/                            # Wyłącznie główne widoki *_view.tpl (1:1 z views/*_view.go i controllers/*_controller.js)
      studio_view.tpl                     # Główny widok studia
      studio_projects_view.tpl            # Widok projektów w Studio
      node_canvas_view.tpl                # Element <canvas#studio-canvas> (WebGL), inputy edycji inline i menu kontekstowe
      editor_context_view.tpl
      text_editor_view.tpl
      editor_breakdown_view.tpl
      editor_variables_view.tpl
      editor_history_view.tpl
      editor_help_view.tpl
      export_view.tpl
    ui_components/                        # Współdzielone komponenty cząstkowe .tpl wewnątrz modułu Studio
      choice_triple_stack.tpl             # Wiersze exit:/text:/if:/set: współdzielone między editor_context_view i text_editor_view
      syn_autocomplete_dropdown.tpl       # Podpowiedzi zmiennych/scen współdzielone między editor_context_view i text_editor_view
      slugline_helper_pills.tpl           # Pigułki INT./EXT. współdzielone między editor_context_view i text_editor_view
      editor_book_settings_form.tpl       # Formularz ustawień książki osadzany w editor_context_view
    styles/
      studio.tss                          # Style widoku Studio (pasek narzędzi, wyszukiwarka, pasek statusu, kontener <canvas>) — węzły i porty rysuje w 100% WebGL
      editor_inspector.tss
      editor_panels.tss
      editor_history.tss
    controllers/
      studio_controller.js
      studio_projects_controller.js
      node_canvas_controller.js
      editor_context_controller.js
      text_editor_controller.js
      editor_breakdown_controller.js
      editor_variables_controller.js
      editor_history_controller.js
      editor_help_controller.js
      export_controller.js
    scripts/                              # Pomocnicze moduły JS (WebGL, audio capture/waveform) ładowane przed controllers/
      msdf_metrics.js
      editor_voice.js
      editor_voice_waveform.js
      node_canvas/
        node_canvas_*.js

  shell/                                  # Dwie powłoki aplikacyjne (każda z własnym modules.cfg)
    studio_app/                           # Pełna aplikacja natywna Mojra: Library + Player + Studio (nigdy nie eksportowana do WASM)
      modules.cfg                         # common, player, library, studio
      embed.go                            # //go:embed modules.cfg templates/*.tpl styles/*.tss controllers/*.js -> FS
      state/
        shell_state.go                    # Rozszerza wspólny stan o ActiveTab, Library, Player, Studio
      views/
        shell_view.go                     # Renderuje układ z Topbarem i aktywnym widokiem
        topbar_view.go                    # TopbarView + topbarScript (tylko w studio_app)
      actions/
        shell_actions.go                  # Przełączanie widoków (SwitchView)
        topbar_actions.go                 # Akcje paska górnego
      templates/
        shell_view.tpl                    # Układ z #topbar + #workspace
        topbar_view.tpl                   # Nawigacja (Biblioteka, Tytuł książki, Studio) + akcje globalne
      styles/
        topbar.tss                        # Style górnego paska
      controllers/
        topbar_controller.js              # Kontroler paska górnego
    book_player/                          # Samodzielny czytnik JEDNEJ książki eksportowanej ze Studio (do .wasm lub .exe/.app)
      modules.cfg                         # common, player
      embed.go                            # //go:embed modules.cfg templates/*.tpl styles/*.tss -> FS
      state/
        shell_state.go                    # Stan pojedynczej książki (Book, Save, Settings)
      views/
        shell_view.go                     # Renderuje ekran startowy książki oraz #player-view
      actions/
        shell_actions.go                  # Inicjalizacja książki z kontenera STBC i dyspozycja akcji czytnika
      templates/
        shell_view.tpl                    # Układ bez topbaru (ekran startowy okładki + #player-view)
      styles/
        book_player.tss                   # Style ekranu startowego czytnika pojedynczej książki

cmd/
  mojra/
    main.go                               # Punkt wejścia aplikacji Mojra (studio_app) oraz kompilator bazowych runtime'ów książki (book_player) wg modules.cfg
```

---

## 3. Dlaczego płaski układ wewnątrz modułu (`state/`, `views/`, `actions/`, `templates/`, `styles/`, `controllers/`) jest lepszy niż `code/`

1. **Krótkie, idiomatyczne ścieżki importów Go**:
   * Importujesz bezpośrednio `"mojra/domain/player/views"`, `"mojra/domain/player/actions"`, `"mojra/domain/player/state"` oraz `"mojra/engine/osbridge"`, `"mojra/engine/transport"`, `"mojra/engine/templates"` — bez sztucznego segmentu `/code/` w każdym imporcie.
2. **Bezpośrednia symetria 1:1 na tym samym poziomie zagnieżdżenia**:
   * W każdym module (`common`, `player`, `library`, `studio`, `shell`) katalogi `state/`, `views/`, `actions/`, `templates/`, `styles/`, `controllers/` leżą bezpośrednio obok siebie:
     * `views/<name>_view.go` <-> `templates/<name>_view.tpl` <-> `controllers/<name>_controller.js`.
   * Katalogi pomocnicze `ui_components/` (współdzielone fragmenty `.tpl` bez własnego `_view.go`) oraz `scripts/` (biblioteki JS bez własnego `_view.go`) są ładowane przed głównymi widokami/kontrolerami: najpierw `ui_components/*.tpl` potem `templates/*_view.tpl`, oraz najpierw `scripts/**` potem `controllers/*.js`.
3. **Brak pojęć prezentacyjnych (`modal`, `overlay`) oraz dwie powłoki (`studio_app` vs `book_player`)**:
   * Pełna aplikacja `domain/shell/studio_app` (`Library + Player + Studio + Topbar`) jest zawsze aplikacją natywną (nigdy nie jest eksportowana do WASM).
   * Do `.wasm` (oraz docelowo do samodzielnego `.exe` / `.app`) eksportowana jest wyłącznie pojedyncza książka w powłoce `domain/shell/book_player` (`Player + ekran okładki`), poprzez wstrzyknięcie kontenera `STBC` z książką do prekompilowanego szablonu binarki `book_player` zaszytego w `studio_app`.

---

## 4. Całkowite wycofanie wszelkich resztek `WFP` (Zero Śladu w Kodzie, Testach i Narzędziach)

1. **`engine/transport/http_server.go` (dawniej `engine/dispatcher.go`)**:
   * Usunąć stałą `wfpMimePrefix = "text/x-wfp"` (L23) oraz warunek `|| strings.HasPrefix(lowerCT, wfpMimePrefix)` z `isFDPRequestPayload` (L38). Jedynym akceptowanym typem MIME protokołu jest `text/x-fdp`.
2. **`engine/scripts/dispatcher.js` (dawniej `app/js/dispatcher.js`)**:
   * Usunąć martwe pozostałości atrybutowego routingu z ery `WFP`: `attrs: { action, kind, target, eventPrefix, paramPrefix }`, `selectors: { action: "[action]" }`, obiekt `modules.backend` (`new Proxy`), gałęzie `[action]` / `[event-*-action]` w `resolve()` (L200–225, L252–265), `param-*` w `collectParams()` (L286–301, L313–315) oraz wywołanie `this.modules[hit.kind][hit.action]` w `handle()` (L401–403).
3. **`internal/audit/audit_js_security.go` & `internal/audit/audit.go`**:
   * W `audit_js_security.go` (L566) usunąć martwy wyjątek dla nieistniejącego `app/js/core.js` (`sendServerAction`, `bootWasmPlayer`).
   * W `audit.go` dodać twardą regułę statyczną (`RETIRED_WFP_PROTOCOL` o wadze `ERROR`), która blokuje pojawienie się w kodzie Go/JS/TPL literałów i symboli `text/x-wfp`, `wfp.go`, `wfp.js`, `EncodeWFP`, `DecodeWFP`, `X-Target`, `X-Data` oraz `app/js/core.js`.
4. **`engine/templates/tpl_validate.go`**:
   * Zaktualizować komunikat błędu kompilacji `.tpl` (L183), zastępując odwołanie do wycofanych `app/js/core.js and app/js/events.js` wskazaniem na `bindings()` w `controllers/<name>_controller.js`.
5. **`domain/server_editor_test.go`, `domain/server_chaos_fuzz_test.go` oraz `build.sh`**:
   * Przemianować `TestWFPProtocolAndZeroRepoSavesOrAppState` -> `TestFDPProtocolAndZeroRepoSavesOrAppState`, `BenchmarkParseActionRequestWFP` -> `BenchmarkParseActionRequestFDP`, `postWFP` -> `postFDP`, `postChaosWFP` -> `postChaosFDP`, `generateMutatedWFPRequest` -> `generateMutatedFDPRequest` oraz tagi `"wfp-*"` -> `"fdp-*"`.
   * Zaktualizować komunikaty w `build.sh` (L215, L230–231) z `WFP` / `X-Target` na `FDP`.

---

## 5. Dostosowanie Bundlera, Audytu i Testów

1. **Uniwersalny Bundler (`engine/templates/bundle.go` & `tpl.go`)**:
   * Usunąć z `engine/bundle.go` wszelkie funkcje i zmienne specyficzne dla Mojry (`BundlePlayerCSSFromFS`, `BundlePlayerJSFromFS`, `CSSPlayerModuleFiles`, `JSPlayerModuleFiles`, `NonPlayerModulePrefixes`, `standaloneExcludedTemplates`) oraz globalne pliki `embed.go`, `embed_prod.go`, `embed_wasm.go`.
   * `engine/templates/bundle.go` czyta `modules.cfg` wybranej powłoki (`studio_app` lub `book_player`), przyjmuje uporządkowaną listę `[]fs.FS` modułów i skanuje w każdym z nich najpierw `ui_components/*.tpl`, potem `templates/*_view.tpl`, `styles/*.tss`, a dla JS najpierw `scripts/**/*.js`, następnie `controllers/*.js`.

2. **Audyt Statyczny (`internal/audit/audit.go`, `audit_js_security.go`, `audit_arch.go`)**:
   * **Odkrywanie plików przez `loadWorkspaceIndex` zamiast zahardkodowanych ścieżek `app/...`**:
     * `loadWorkspaceIndex` rekurencyjnie indeksuje `.go`, `.tpl`, `.tss`/`.css`, `.js` i `.svg` bezpośrednio z `engine/` oraz `domain/` (zastępując odwołania do `app/js`, `app/templates` i `app/styles` w `audit.go:511, 633, 647, 1191` oraz `audit_js_security.go:30, 261, 582`).
     * Rozpoznawać pliki widoków po ścieżce `*/views/*_view.go` (`audit.go:729, 886`, `audit_js_security.go:1182`), a pliki akcji po `*/actions/*_actions.go` (`audit.go:663`) — dzięki temu weryfikacja 1:1 między `<snake>_view.go`, `<snake>_view.tpl` i `<snake>_controller.js` działa w każdym module `domain/<module>/`.
     * Zaktualizować reguły ścieżkowe w `audit.go:842, 887, 900` (`domain/book/actions/{sanitize,conditions,exporters}.go`) oraz wyjątek warstwy transportowej w `audit_js_security.go:567` (`engine/scripts/{dispatcher,transport_http,transport_wasm}.js`).
   * **Interfejsy graniczne OS/WASM (`audit_arch.go` — `ARCH_SINGLE_IMPL_IFACE`)**:
     * `osbridge.Storage` posiada dwie konkretne implementacje (`FSStorage` w `storage_fs.go` oraz `WasmStorage` w `storage_wasm.go`).

3. **Testy Kontraktowe, Runtime JS i Izolacja Powłok (`domain/*_test.go`)**:
   * Pobierać szablony, style i skrypty z listy modułów `fs.FS` zadeklarowanej w `modules.cfg` danej powłoki zamiast zahardkodowanych ścieżek `app/templates/*.tpl`.
   * Weryfikować przez `go list -deps`, że drzewo zależności powłoki `domain/shell/book_player` (zarówno dla WASM, jak i natywnego czytnika książki) zawiera wyłącznie `engine/**`, `domain/common/**`, `domain/player/**`, `domain/book/**` i `domain/i18n`, a nigdy nie importuje `domain/studio/**`, `domain/library/**` ani `domain/shell/studio_app/**`.

4. **Generator Grafu Projektu (`cmd/projgraph/scanner_ui.go`)**:
   * Zastąpić stałą `dirInternalActions = "domain/actions"` wiązaniem krawędzi akcji do pakietu `domain/<module>/actions` rejestrującego dany handler.

---

## 6. Plan Wykonawczy Krok po Kroku (Mapowanie Plik po Pliku: Skąd -> Dokąd)

### Etap 1: Porządek w `engine/` (`osbridge/`, `transport/`, `templates/`, `scripts/`) i wycofanie resztek `WFP`
1. Dodanie bezdomenowych `storage.go`, `storage_fs.go`, `storage_wasm.go` w `engine/osbridge/`
2. `engine/fdp.go`, `engine/dispatcher.go`, `engine/server.go` -> `engine/transport/{fdp.go,contract.go,http_server.go,wasm_bridge.go}` (usunięcie `wfpMimePrefix`)
3. `engine/bundle.go` -> `engine/templates/bundle.go` (przeniesienie `SanitizePlainText`/`RenderSafeProseHTML` z `tpl_render.go` do `domain/book/actions/sanitize.go` z rejestracją `ProseFormatter` w `engine/templates` oraz usunięcie `BundlePlayer*` z `bundle.go`)
4. `app/js/{fdp.js,dispatcher.js}` -> `engine/scripts/{fdp.js,dispatcher.js,transport_http.js,transport_wasm.js}` + `engine/embed.go` (zero `.tpl` i zero `.tss` w `engine/`)

### Etap 2: Podział `domain/book` na `domain/book/model` oraz `domain/book/actions`
1. Struktury z `domain/book/{model,variables,save,layout}.go` -> `domain/book/model/{book,variables,save,layout,tokens}.go`
2. Operacje z `domain/book/*` + `SanitizePlainText`/proza z `tpl_render.go` -> `domain/book/actions/{parser,loader,stbc,conditions,save_codec,media_verify,sanitize,serializer,mutator,history,packager,exporters,beta_license}.go`

### Etap 3: Pionowe moduły `domain/{common,player,library,studio,shell}` (usunięcie `app/`, `cmd/wasmplayer` i głównych `embed*.go`)
1. **`domain/common`**: `domain/{state,views,actions}/confirm_*.go` + `ui_state.go`/`ui_view.go`/`ui_actions.go` + `app/templates/confirm_view.tpl` + `ui_components/{components,scene_choice_arrow}.tpl` + `app/js/controllers/{app_shell,confirm}_controller.js` (`ui_controller.js`, `confirm_controller.js`) + `app/styles/{colors,base,app,modals}.tss` (`colors.tss`, `base.tss`, `forms.tss`) + `app/icons/*` + `app/fonts/*` -> `domain/common/{state,views,actions,templates,ui_components,styles,controllers,icons,fonts}` + `domain/common/embed.go`
2. **`domain/player`**:
   * `domain/state/player_state.go` -> `domain/player/state/`
   * `domain/views/{player,scene,audio_deck}_view.go` -> `domain/player/views/`
   * `domain/actions/{player_actions,player_standalone}.go` -> `domain/player/actions/`
   * Zasoby czytnika z `app/{templates,styles,js/controllers}` -> `domain/player/{templates,styles,controllers}/` + `domain/player/embed.go`
3. **`domain/library`**:
   * `domain/state/{library,new_book}_state.go` -> `domain/library/state/`
   * `domain/views/{library,add_book,new_book,qr}_view.go` -> `domain/library/views/`
   * `domain/actions/{library,add_book,new_book,qr}_actions.go` -> `domain/library/actions/`
   * Zasoby biblioteki z `app/{templates,styles,js/controllers}` -> `domain/library/{templates,styles,controllers}/` + `domain/library/embed.go`
4. **`domain/studio`**:
   * `domain/state/{studio,text_editor,export}_state.go` -> `domain/studio/state/`
   * `domain/views/{studio*,node_canvas,editor_*,text_editor,export}_view.go` -> `domain/studio/views/`
   * `domain/actions/{studio*,text_editor*,editor_*,export*}_actions.go` -> `domain/studio/actions/`
   * Zasoby studia z `app/{templates,styles,js/controllers,js}` (`editor_canvas.tss` -> `studio.tss`, komponenty cząstkowe do `ui_components/`) + `app/studio-atlas.png` -> `domain/studio/{templates,ui_components,styles,controllers,scripts}/` + `domain/studio/embed.go`
5. **`domain/shell/studio_app` & `domain/shell/book_player` oraz `cmd/mojra`**:
   * Wspólna obsługa motywu, skali UI, trybu bez obrazów i toastów (`app_shell_controller.js` -> `ui_controller.js`, `ui_state.go`, `ui_view.go`, `ui_actions.go`) trafia **tylko raz** do `domain/common/`.
   * `studio_app`: własny `modules.cfg` (`common, player, library, studio`), układ `shell_view.tpl` (`#topbar + #workspace`), `topbar_*` i przełączanie zakładek.
   * `book_player`: własny `modules.cfg` (`common, player`), układ `shell_view.tpl` (`#player-view` + ekran startowy okładki bez topbaru) i `book_player.tss` (wspólna powłoka dla eksportów `.wasm` oraz `.exe`/`.app` ze Studio).
   * Usunięcie `cmd/wasmplayer/`, katalogu `app/` oraz `embed.go`, `embed_prod.go`, `embed_wasm.go` z katalogu głównego.

### Etap 4: Aktualizacja `internal/audit`, `cmd/projgraph` i `domain/*_test.go`
* Wykonanie wszystkich punktów z §4 i §5, uruchomienie `go test ./...`, audytu statycznego (`0` błędów) oraz budowania obu powłok przez `./build.sh --macos`.
