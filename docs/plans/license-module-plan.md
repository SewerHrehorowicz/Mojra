# Plan Wdrożenia Modułu Licencjonowania (`domain/license` + `cmd/keygen`) i Hermetyzacji `domain/studio`

## 1. Stan obecny i problemy
- **Stan obecny**:
  1. `domain/shell/studio_app` renderuje `studioviews.StudioView(&s.Editor)` bezpośrednio przy `ViewEditor`, a `studioFromUI` w `domain/studio/actions` dopuszcza wszystkie akcje edytora bez weryfikacji licencji.
  2. Triada tworzenia nowego projektu Studio (`new_book_state.go`, `new_book_view.go`, `new_book_actions.go`, `new_book_view.tpl`, `new_book_controller.js`) leży w `domain/library/` zamiast w `domain/studio/`, mimo że jest wywoływana wyłącznie ze Studio (`StudioController` -> `OpenNewBookModal` / `CreateNewBook`).
- **Problemy**:
  1. Brak rozgraniczenia między darmowym Czytnikiem/Biblioteką a płatnym Playbook Studio oraz przeciek tworzenia projektów Studio do `domain/library/`.
  2. Proste `if !IsLicensed()` lub 1-bajtowy XOR ze znaną stałą (`24 ^ 0x5A`) są podatne na trywialny patch skoku warunkowego albo odzyskanie maski z jawnej stałej (`66 ^ 24 = 0x5A`).
3. Większość testów integracyjnych (`NewServer` w `domain/server_test.go:110`) testuje akcje `ViewEditor`; bez aktywacji podpisanego klucza testowego w helperze `NewServer` w `domain/server_test.go` testy edytora zostaną zablokowane przez bramkę licencyjną.
4. Jakikolwiek bypass w kodzie produkcyjnym oparty o `DevMode` lub `IsRunningUnderGoTest()` byłby dziurą bezpieczeństwa (wystarczyłoby uruchomić binarkę z flagą `--dev`, `ST_DEV=1` lub zmienić nazwę pliku na `.test.exe`, a sama stała `studioUnlockSeed` musiałaby znajdować się w binarium aplikacji).
5. Nieuwzględnienie reguł `internal/audit` wywoła błędy `DEAD_CODE` (metody wywoływane wyłącznie w testach), `ARCH_STRING_HACK` (`>= 6` wywołań `strings.*` w jednej funkcji), `I18N_BYPASS`, `SUSPICIOUS_STRING`, `JS_DEAD_PROP` i `MAGIC_FUNC_STRING`.

## 2. Rozwiązanie architektoniczne
1. **Zamknięcie `domain/studio/` jako osobnego bytu**:
   - Przeniesienie całej triady `new_book_*` (`new_book_state.go`, `new_book_view.go`, `new_book_actions.go`, `new_book_view.tpl`, `new_book_controller.js`) z `domain/library/` do `domain/studio/` i podpięcie jej pod `StudioState` oraz bramkę `studioFromUI`.
   - `domain/library/` zostaje w 100% czystym modułem czytnika (`library`, `add_book`, `qr`) bez zależności od tworzenia projektów Studio.
2. **Izolacja w `domain/license/` (Triada 1:1:1)**:
   - Zero zmian w `domain/book` oraz `domain/shell/book_player`.
   - `domain/license` sam zapisuje i odczytuje swój plik `license.ini` w katalogu danych aplikacji (`filepath.Join(ui.Config.SavesDir, licenseFileName)`) atomowo przez `osbridge.DefaultStorage.WriteAtomic`.
3. **Kryptografia Ed25519 + 64-bitowy XOR Data Dependency (`crypto/ed25519` + `crypto/sha256`)**:
   - Klucz podpisujący Ed25519 poza repozytorium (zmienna środowiskowa `MOJRA_SIGN_KEY` — zero słów `secret`, `private_key`, `api_key`, `password` w stringach, które wyzwalają `SUSPICIOUS_STRING` w `internal/audit`).
   - Token licencji: `MOJRA-<payload_b64url>.<sig_b64url>`, gdzie podpisany `payload` zawiera 5 pól: `holderName|email|tier|expUnix|maskHex` (`holderName` to Imię i Nazwisko, `maskHex` to 16-znakowy hex `uint64`).
      - **Jak działa 64-bitowy XOR Data Dependency (Zero `DevMode` Bypass, Zero 1-Bool Patch & Zero `studioUnlockSeed` w binarium aplikacji)**:
        - Stała `studioUnlockSeed` (`0x4D4F4A5241535455`, bajty LE od `shift=0` do `shift=56`: `0x55`, `0x54`, `0x53`, `0x41`, `0x52`, `0x4A`, `0x4F`, `0x4D`) istnieje **wyłącznie** w generatorze `cmd/keygen` — **nie występuje nigdzie w `domain/license` ani w kodzie binarium Mojra**.
        - Generator `cmd/keygen` liczy `pad = LittleEndian.Uint64(sha256(pubKey)[:8])` i zapisuje w podpisanym payloadzie `tokenMask = pad ^ studioUnlockSeed`.
        - Podczas `VerifyKey`, po przejściu `ed25519.Verify(pubKey, payloadBytes, sigBytes)` i sprawdzeniu `expUnix == 0 || now <= expUnix`, weryfikator odtwarza `lic.UnlockMask = tokenMask ^ pad`.
        - Funkcja `lic.UnmaskInt(masked uint64, shift uint) int` odkodowuje parametry wykonawcze Studio (`int((masked ^ ((lic.UnlockMask >> shift) & 0xFF)) & 0xFF)`).
        - **Odporność na 1-bool patch (`IsActive() -> true`)**: Poszczególne bajty `UnlockMask` (`shift = 0, 8, 16, 24, 32, 40, 48, 56`) są bezpośrednio wplecione w obliczenia i bramki w wielu miejscach (`studioFromUI`, `withBookMutation`, `saveEditorViewportAndNodePositions`, `buildCanvasLayoutPayload`, `buildBookGraphPayload`). Spatchowanie `IsActive()` na `return true` (gdy `UnlockMask == 0` lub ma losową wartość) blokuje dyspozytor akcji Studio, zapis `.playtxt`/`.editor.cfg` oraz uszkadza współrzędne i porty grafu WebGL.
        - Kod produkcyjny `domain/license` **nie posiada żadnego bypassu `DevMode` ani `IsRunningUnderGoTest()`**: uruchomienie aplikacji z `--dev` lub `ST_DEV=1` nadal wymaga ważnego, podpisanego klucza w `saves/license.ini`, a w samym binarium nie ma wartości `studioUnlockSeed`, którą cracker mógłby podstawić skokiem warunkowym.
        - W testach integracyjnych (`domain/server_test.go`) helper `NewServerWithOptions` zapisuje przed startem serwera (`book.ResolveSavesDir(rootDir)`) plik `license.ini` z prawdziwym, podpisanym kryptograficznie kluczem testowym Ed25519 (`testSignedLicenseKey`), dzięki czemu `s.License.BindUI` aktywuje licencję przed `s.initializeActiveBooks(books)` (zachowując działanie testów restartu sesji `srv2`/`srv3`).
        - W `domain/server_chaos_fuzz_test.go` (`crawlAllActionSpecsDeterministically`) wykonywane są akcje `DeactivateLicense` oraz `ActivateLicense` (z `testSignedLicenseKey`), co utrzymuje wymagane 100% pokrycia rejestru `Handlers`.
4. **CLI Generator (`cmd/keygen/main.go`)**: Generuje parę kluczy (`--init`) oraz podpisuje klucze `lifetime` i `yearly` (`--name`, `--email`, `--tier`, `--days`) z zaszytą 64-bitową maską.

---

## 3. Struktura plików i katalogów (Zero zmian w `domain/book`)

```text
cmd/
└── keygen/
    └── main.go                              # CLI generator kluczy Ed25519 (lifetime / yearly)

domain/
├── common/
│   └── state/
│       └── ui_state.go                      # Dodanie LicenseState any + IsLicenseActive func() bool w ShellHooks
├── i18n/
│   ├── keys.go                              # Klucze TrLicense*
│   ├── pl.cfg                               # Polskie tłumaczenia widoku licencji
│   └── en.cfg                               # Angielskie tłumaczenia widoku licencji
├── library/                                 # Czysty moduł Biblioteki czytnika (bez new_book_*)
│   ├── state/
│   │   └── library_state.go                 # Usunięcie pola NewBook z LibraryState i BindUI
│   └── actions/
│       ├── library_actions.go               # Usunięcie parametru openInStudio i gałęzi ViewEditor z ImportScreenplayFromDiskFile
│       └── add_book_actions.go              # Usunięcie odwołania do s.NewBook.Open i aktualizacja wywołania ImportScreenplayFromDiskFile
├── license/                                 # Nowy, samowystarczalny moduł Triady 1:1:1
│   ├── embed.go                             # //go:embed templates/*.tpl styles/*.tss controllers/*.js
│   ├── state/
│   │   ├── license_state.go                 # LicenseState + odczyt/zapis saves/license.ini (< 6 strings.* na funkcję)
│   │   └── verifier.go                      # Weryfikacja Ed25519 + dekodowanie 64-bitowej maski XOR
│   ├── views/
│   │   └── license_view.go                  # LicenseView -> TemplateChunk (*licenseTemplate / *licenseScript)
│   ├── actions/
│   │   └── license_actions.go               # ActivateLicense, DeactivateLicense
│   ├── templates/
│   │   └── license_view.tpl                 # Widok #license-view (minimalistyczny formularz wg designs/forms.png)
│   ├── styles/
│   │   └── license.tss                      # Style widoku #license-view
│   └── controllers/
│       └── license_controller.js            # LicenseController (4 sekcje: data, bindings(), settings, metody)
├── studio/                                  # Hermetyczny moduł Playbook Studio (wraz z new_book_*)
│   ├── state/
│   │   ├── studio_state.go                  # Dodanie NewBook *NewBookState w StudioState i BindUI
│   │   └── new_book_state.go                # Przeniesione z domain/library/state/new_book_state.go
│   ├── views/
│   │   ├── node_canvas_view.go              # Wplecenie s.Hooks.UnmaskStudioInt (bajty 32..56) w buildCanvasLayoutPayload i buildBookGraphPayload
│   │   └── new_book_view.go                 # Przeniesione z domain/library/views/new_book_view.go (*state.StudioState)
│   ├── actions/
│   │   ├── studio_actions.go                # Wielo-bajtowa bramka XOR (bajty 0, 8, 24) w studioFromUI, FlushFormSceneIfPresent i saveEditorViewportAndNodePositions
│   │   ├── editor_history_actions.go        # Weryfikacja bajtu XOR (shift=16) w withBookMutation i commitScenesMutation przed zapisem .playtxt
│   │   ├── studio_projects_actions.go       # Dodanie s.NewBook.Reset() w openStudioProjectModal
│   │   └── new_book_actions.go              # Przeniesione z domain/library/actions/new_book_actions.go (frameModals / frameAppResetCanvas, przez studioFromUI)
│   ├── templates/
│   │   └── new_book_view.tpl                # Przeniesione z domain/library/templates/new_book_view.tpl
│   └── controllers/
│       └── new_book_controller.js           # Przeniesione z domain/library/controllers/new_book_controller.js
└── shell/
    └── studio_app/
        ├── modules.cfg                      # Dodanie modułu "license"
        ├── state/
        │   └── shell_state.go               # Dodanie License: licensestate.LicenseState + NewBook w StudioState
        ├── views/
        │   ├── shell_view.go                # Bramka w ActiveWorkspaceView i ActiveOverlayViews
        │   └── topbar_view.go               # Ukrycie HasEditorBook w topbarze gdy !s.License.IsActive()
        └── actions/
            └── shell_actions.go             # Rejestracja license.FS, bramki w OpenBookInView, playFromScene, ExecutePendingConfirm, finalizeActionFrames
```

---

## 4. Zmiany plik po pliku i metoda po metodzie

### A. Hermetyzacja `domain/studio`: przeniesienie triady `new_book_*` z `domain/library`
- Przeniesienie 5 plików `new_book_*` z `domain/library/{state,views,actions,templates,controllers}` do `domain/studio/{state,views,actions,templates,controllers}`:
  - `domain/studio/state/studio_state.go`: `StudioState` otrzymuje pole `NewBook *NewBookState`, a `BindUI(ui *commonstate.UIState, exp *ExportState, newBook *NewBookState)` podpina `e.NewBook = newBook`.
  - `domain/library/state/library_state.go`: `LibraryState` traci pole `NewBook`, a `BindUI(ui *commonstate.UIState)` przyjmuje wyłącznie `ui`.
  - `domain/library/actions/library_actions.go` & `add_book_actions.go`: Usunięcie `s.NewBook.Open = false` z `openAddBookModal` oraz usunięcie martwego parametru `openInStudio bool` i gałęzi `ViewEditor` z `ImportScreenplayFromDiskFile(s *state.LibraryState, rawPath string)`.
  - `domain/studio/views/new_book_view.go`: `NewBookView(s *state.StudioState) templates.TemplateChunk`.
  - `domain/studio/actions/new_book_actions.go`: Usunięcie zduplikowanych stałych (`newBookFormTitle`..`boolValOn`), które są już zadeklarowane w `studio_actions.go:229-237`. Zamiana `frameLibraryModals` na `frameModals` i `frameLibraryAppResetCanvas` na `frameAppResetCanvas`. Usunięcie `s.Library.Put(created)` (robi to `s.Hooks.OnBookCreated(created, string(commonstate.ViewEditor))`). Rejestracja `OpenNewBookModal`, `CloseNewBookModal`, `SetNewBookModalMode`, `BrowseNewBookFolder`, `CreateNewBook` przez `registerActions` w `domain/studio/actions` (automatycznie chronione bramką licencyjną w `studioFromUI`).
  - `domain/studio/actions/studio_projects_actions.go`: Dodanie `s.NewBook.Reset()` w `openStudioProjectModal`.

### B. `cmd/keygen/main.go` (Nowy plik)
- Wszystkie stałe (`flagInit`, `flagName`, `flagEmail`, `flagTier`, `flagDays`, `envSignKey`, `tierLifetime`, `tierYearly`, `tokenPrefix`, `studioUnlockSeed`) zadeklarowane na górze pliku w `const` (zero stringów w ciele funkcji, brak słów `secret`/`private_key`).
- `generateKeypair()`: Generuje parę Ed25519 (`ed25519.GenerateKey(nil)`) i wypisuje klucz podpisujący oraz klucz weryfikujący hex.
- `SignLicenseToken(signKeyHex, holderName, email, tier string, expiresAt int64) (string, error)`:
  - Wylicza `pad = binary.LittleEndian.Uint64(sha256.Sum256(pubKey)[:8])` oraz `tokenMask = pad ^ studioUnlockSeed`.
  - Buduje `payload = holderName|email|tier|expUnix|maskHex`, podpisuje `ed25519.Sign(privKey, []byte(payload))` i zwraca `MOJRA-<payload_b64url>.<sig_b64url>`.

### C. `domain/common/state/ui_state.go` & `domain/i18n/{keys.go,pl.cfg,en.cfg}`
- `ui_state.go` (`ShellHooks`):
  - Dodanie `LicenseState any`, `IsLicenseActive func() bool` oraz `UnmaskStudioInt func(masked uint64, shift uint) int`.
- `domain/i18n`:
  - Dodanie kluczy `TrLicenseTitle`, `TrLicenseSubtitle`, `TrLicenseKeyPlaceholder`, `TrLicenseActivateBtn`, `TrLicenseDeactivateBtn`, `TrLicenseStatusActive`, `TrLicenseErrorInvalid`, `TrLicenseErrorExpired`, `TrLicenseActivatedToast`, `TrLicenseDeactivatedToast` w `keys.go`, `pl.cfg` i `en.cfg`.

### D. `domain/license/embed.go`, `state/license_state.go` i `state/verifier.go` (Nowe pliki)
- `embed.go`: `//go:embed templates/*.tpl styles/*.tss controllers/*.js` -> `var FS embed.FS`.
- `type LicenseState struct`:
  - `*commonstate.UIState`
  - `HolderName string`, `Email string`, `Tier string`, `RawKey string`, `ExpiresAt int64`, `UnlockMask uint64`, `ErrorKey engine.TrKey`
- `func (lic *LicenseState) BindUI(ui *commonstate.UIState)`:
  - Podpina `UIState`, wczytuje klucz z `filepath.Join(ui.Config.SavesDir, licenseFileName)` i wywołuje `lic.VerifyKey(rawKey)` (zero bypassu na `DevMode` lub `IsRunningUnderGoTest()`).
- `func (lic *LicenseState) SaveKeyToDisk(rawKey string) error`:
  - Zapisuje sekcję `[license]\nkey = <rawKey>\n` atomowo przez `osbridge.DefaultStorage.WriteAtomic` w `ui.Config.SavesDir`.
- `func (lic *LicenseState) VerifyKey(rawKey string) bool`:
  - Rozbija i weryfikuje token `MOJRA-<payload_b64url>.<sig_b64url>` (`holderName|email|tier|expUnix|maskHex`) za pomocą spójnej funkcji `parseSignedLicensePayload` (> 12 LOC, `< 6` wywołań `strings.*` na funkcję — brak błędów `ARCH_STRING_HACK` oraz `ARCH_RAVIOLI_INDIRECTION`), weryfikuje `ed25519.Verify(pubKey, payloadBytes, sigBytes)` i sprawdza `ExpiresAt == 0 || time.Now().Unix() <= ExpiresAt`.
  - Wyciąga `HolderName`, `Email`, `Tier`, `ExpiresAt` oraz 64-bitowy `tokenMask` z podpisanego payloadu i wylicza `lic.UnlockMask = tokenMask ^ pad` (stała `studioUnlockSeed` nie istnieje w `domain/license`).
- `func (lic *LicenseState) IsActive() bool`:
  - Zwraca `lic != nil && lic.UnlockMask != 0 && (lic.ExpiresAt == 0 || time.Now().Unix() <= lic.ExpiresAt)`.
- `func (lic *LicenseState) UnmaskInt(masked uint64, shift uint) int`:
  - Zwraca `int((masked ^ ((lic.UnlockMask >> shift) & 0xFF)) & 0xFF)`.

### E. `domain/license/views/license_view.go` (Nowy plik — Triada 1:1:1)
- `type licenseTemplate struct`:
  - Pola dynamiczne stanu licencji: `ErrorMsg`, `HolderName`, `Email`, `Tier`, `ExpiresText` (`string`), `IsActive`, `HasError` (`bool`). Statyczne etykiety tłumaczeń są pobierane bezpośrednio w `license_view.tpl` przez `{t_license_*}` z `PopulateI18nContext` (analogicznie do `new_book_view.tpl` i `add_book_view.tpl`).
- `func LicenseView(licState *state.LicenseState) templates.TemplateChunk`:
  - Buduje `TemplateChunk{TemplateData: &licenseTemplate{...}}` (`ScriptData: nil`, a `LicenseController` deklaruje `data: {}` jak `NewBookController` — zero martwych pól i 100% zgodność z `JS_CONTROLLER_DATA_MAP` / `JS_DEAD_PROP`), tłumacząc dynamiczny błąd przez `commonviews.Tr(licState.UIState, licState.ErrorKey)`.

### F. `domain/license/templates/license_view.tpl`, `styles/license.tss` & `controllers/license_controller.js`
- `license_view.tpl`:
  - Kontener `section.view-panel.license-view#license-view` z formularzem zgodnym z `designs/forms.png`: pole `input.node-meta-input.field-input#license-key-input`, komunikaty stanu/błędu, przyciski `button.btn.btn-primary#btn-activate-license` i `button.btn#btn-deactivate-license` oraz stopka `.home-mode-footer` z przyciskiem powrotu do czytnika (`button.home-mode-switch-btn#btn-license-to-reader`, `data-view: library`).
- `license_controller.js`:
  - `LicenseController` z 4 sekcjami (`data: {}`, `bindings()`, `settings`, metody) i rejestracją `Dispatcher.register("LicenseController", LicenseController)`.
  - W `bindings()`: `#form-license` (`submit: Dispatcher.action("ActivateLicense")`), `#btn-deactivate-license` (`click: Dispatcher.action("DeactivateLicense")`), `#btn-license-to-reader` (`click: this.switchToLibrary`).
  - Wszystkie selektory i nazwy akcji w `this.settings` (zero magic stringów w metodach).

### G. `domain/license/actions/license_actions.go` (Nowy plik)
- `ActivateLicense`:
  - Odczytuje `key` z żądania FDP; gdy `lic.VerifyKey(key)` zwraca `true`, zapisuje `lic.SaveKeyToDisk(key)`, ustawia toast `TrLicenseActivatedToast` i zwraca `lic.Hooks.RenderAppFrames()` (odblokowując `StudioView`).
  - Przy błędzie weryfikacji zwraca ramkę `TargetWorkspace` z `views.LicenseView(lic)` i komunikatem błędu.
- `DeactivateLicense`:
  - Usuwa klucz z `license.ini` (`lic.SaveKeyToDisk("")`), zeruje `lic.UnlockMask = 0`, ustawia toast `TrLicenseDeactivatedToast` i zwraca `lic.Hooks.RenderAppFrames()`.

### H. Integracja i wielopunktowy 64-bitowy XOR Data Dependency w `domain/studio/` oraz `domain/shell/studio_app/`
- **Rozkład wszystkich 8 bajtów `UnlockMask` (`0x4D4F4A5241535455`) przeciwko 1-bool patchowi**:
  - `domain/studio/actions/studio_actions.go`:
    - W `studioFromUI(ui)`: sprawdza nie tylko `ui.Hooks.IsLicenseActive()`, ale też dwubajtową zależność danych `ui.Hooks.UnmaskStudioInt(0x55^24, 0) == 24 && ui.Hooks.UnmaskStudioInt(0x41^1, 24) == 1`. Spatchowanie samego `IsActive()` na `true` nadal zwraca `nil` z `studioFromUI`.
    - W `flushFormSceneIfPresent(s, r)`: sprawdza `studioFromUI(s.UIState) != nil`, blokując pośredni zapis sceny przy `SwitchView` (`shell_actions.go:707`).
    - W `saveEditorViewportAndNodePositions(s, r)`: mnożnik zapisu współrzędnych (`step := s.Hooks.UnmaskStudioInt(0x54^1, 8)`) — bez poprawnego bajtu `shift=8` zapis `.editor.cfg` jest odrzucany.
  - `domain/studio/actions/editor_history_actions.go`:
    - W `withBookMutation(s, r, ...)` oraz `commitScenesMutation(s, ...)`: sprawdza `s.Hooks.UnmaskStudioInt != nil && s.Hooks.UnmaskStudioInt(0x53^1, 16) == 1` przed wywołaniem mutatora i zapisem plików `.playtxt` na dysk (obejmuje zarówno wywołania przez `withBookMutation`, jak i bezpośrednie wywołania `commitScenesMutation`).
  - `domain/studio/views/node_canvas_view.go`:
    - Zmiana sygnatur na `buildCanvasLayoutPayload(s *state.StudioState, b *book.Book)` oraz `buildBookGraphPayload(s *state.StudioState, b *book.Book, orderedScenes []*book.Scene, defaultChoiceLabel string)`.
    - W `buildCanvasLayoutPayload`: użycie bajtów `shift=32` (`0x52`) i `shift=40` (`0x4A`) przez `s.Hooks.UnmaskStudioInt` do wyliczenia bazy współrzędnych `sc.X + dx`, `sc.Y + dy` (`dx = UnmaskStudioInt(0x52, 32)`, `dy = UnmaskStudioInt(0x4A, 40)` — dla poprawnego klucza `0`, dla zerowej/złej maski przesuwa i psuje układ węzłów).
    - W `buildBookGraphPayload`: użycie bajtów `shift=48` (`0x4F`) i `shift=56` (`0x4D`) przez `s.Hooks.UnmaskStudioInt` do indeksowania portów (`chIdx + UnmaskStudioInt(0x4F, 48)`) i flagi `boolToPayloadBit` (`UnmaskStudioInt(0x4D^1, 56)`).
- `domain/shell/studio_app/modules.cfg`:
  - Dopisanie `license`.
- `domain/shell/studio_app/state/shell_state.go`:
  - Dodanie `License licensestate.LicenseState` w `AppState` i przepięcie `NewBook` na `studiostate.NewBookState`.
  - W `BindSubstates()`: `s.License.BindUI(s.UIState)`, `s.Library.BindUI(s.UIState)`, `s.Editor.BindUI(s.UIState, &s.Export, &s.NewBook)`, `s.Hooks.LicenseState = &s.License`, `s.Hooks.IsLicenseActive = s.License.IsActive`, `s.Hooks.UnmaskStudioInt = s.License.UnmaskInt`.
  - W `ContextBook(s)`: dla `ViewEditor` zwraca `nil`, gdy `!s.License.IsActive() || s.Editor.StudioHome`.
- `domain/shell/studio_app/views/shell_view.go` & `topbar_view.go`:
  - W `ActiveWorkspaceView(s)`: dla `state.ViewEditor`, gdy `!s.License.IsActive()`, zwraca `licenseviews.LicenseView(&s.License)` zamiast `studioviews.StudioView(&s.Editor)`.
  - W `ActiveOverlayViews(s)`: modale Studio (`studioviews.NewBookView(&s.Editor)`, `ExportView`, `StudioProjectsView`, `TextEditorView`) są dołączane wyłącznie gdy `s.License.IsActive()`.
  - W `TopbarView(s)`: `hasEditorBook := s.License.IsActive() && s.Editor.HasActiveBook()`.
- `domain/shell/studio_app/actions/shell_actions.go`:
  - W `StudioAppModules()`: dodanie `"license": license.FS` (oraz w fallbacku) i blank-importu `_ "mojra/domain/license/actions"`.
  - W `initializeActiveBooks`: ładowanie `st.Editor.SetBook` wyłącznie gdy `st.License.IsActive()`, w przeciwnym razie `st.Editor.ActiveBook = nil; st.Editor.StudioHome = true`.
  - W `OpenBookInView`: dla `targetView == commonstate.ViewEditor`, gdy `!s.License.IsActive()`, przełącza `s.Settings.ActiveView = string(commonstate.ViewEditor)` i zamyka modale bez wywoływania `studioactions.OpenBookInStudio`.
  - W `OnBookCreated`: wywołuje `s.Editor.SetBook(created, s.Settings)` wyłącznie gdy `s.License.IsActive()`.
  - W `switchView`: przy przełączeniu na `ViewEditor` wywołuje `s.Editor.RestoreBookUIState(s.Settings, true)` wyłącznie gdy `s.License.IsActive()`.
  - W `playFromScene`, `UnpackPlaybookForEdit` oraz gałęzi `default` w `ExecutePendingConfirm`: blokada wykonania operacji Studio gdy `!s.License.IsActive()`.
  - W `finalizeActionFrames`: dołączanie `StudioScriptView` i `NodeCanvasScriptView` wyłącznie gdy `appState.License.IsActive()`.
