# Mojra & Playbook Studio — Plan Domknięcia MVP (Closed Beta) vs v1.0

## 0. Co Już Działa w Kodzie (Zrealizowane Fundamenty)
- **Silnik Węzłów WebGL 120fps (`app/js/editor_webgl.js`)**: Płynne płótno na siatce `24px`, łączenie portów, przeciąganie scen i rozdziałów.
- **Git dla Pisarzy (`internal/book/history.go`)**: Tworzenie i przełączanie wariantów fabuły na osi czasu z natychmiastowym wyliczaniem różnic produkcyjnych (sceny, strony `1/8`, czas).
- **Raport Produkcyjny i Obsada na Żywo (`internal/book/screenplay.go`)**: Objętość w `1/8` strony, czas ekranowy/audio, dni zdjęciowe/nagraniowe, proporcje `WN./PL.` i `DZIEŃ/NOC`, balans aktów oraz statystyki kwestii i słów dla każdego aktora z globalną zmianą nazw.
- **Zmienne i Warunki (`internal/book/variables.go`)**: Typy `bool`, `int`, `enum`, warunki widoczności wyborów, modyfikacje stanów i interpolacja `{{...}}` w tekście.
- **Nagrywanie Lektorskie i Przycinanie Audio (`app/js/editor_voice.js`, `internal/book/audio_flac.go`)**: Dyktafon w inspektorze (`320px`) i edytorze tekstu (`#modal-scene-text`), waveform z narożnikami przycinania (`TrimSceneVoice`) i zapisem bezstratnym `FLAC`.
- **Modal Multi-Exportu, Bytecode `STBC` i Eksport Web/WASM (`internal/book/stbc.go`, `internal/book/web_exporter.go`, `cmd/wasmplayer/main.go`)**: Eksport `.playbook`, samodzielnej paczki Web/WASM, `.pdf` (PISF/US), `.fountain` oraz `.playtxt`.
- **Polski Standard Scenariuszowy PDF (`internal/book/screenplay_pdf.go`) i Fountain (`.fountain`)**: Eksport A4 Courier 12pt zgodny z PISF (`WN./PL.`, numeracja scen, `(WIĘCEJ)` / `(CIĄG DALSZY)`, strona tytułowa + automatyczna 2. strona `LISTA POSTACI / ZESTAWIENIE LOKACJI`).
- **Czytnik i Odtwarzacz (`#view-library`, `#view-player`)**: Obsługa pakietów `.playbook` / `.stbook`, tryb bez obrazów (`imageless`), regulacja tempa audio i `Auto-Choice` (słuchanie bez dotykania ekranu).
- **Strona Pokazowa WWW z Odtwarzaczem WASM (`www/` -> `dist/www/`)**: Kompilowana z `.tpl`/`.tss` statyczna strona z zaszytym `mojra.wasm` (`STBC`), animowanym grafem opowieści w tle, ikoną `.playbook` i formularzem zapisów na zamkniętą betę.
- **Lokalizacja PL/EN oparta na `.cfg` i Audyt Statyczny 0 Błędów (`internal/tpl/i18n/*.cfg`, `internal/gui/i18n.go`, `internal/audit/`)**: Wszystkie słowniki UI i JS przeniesione do plików `pl.cfg` / `en.cfg` (`//go:embed`), pełna unifikacja nazewnictwa `Mojra` (`cmd/mojra`, `module mojra`) oraz wynik `0 wykrytych miejsc` w `./build.sh --audit --strict`.

---

## 1. ETAP 1: Lista Zadań do Domknięcia MVP (Closed Beta — Priorytet P0)

Cel: Zamknąć bieżący WIP i dać pierwszym testerom (scenarzystom, autorom słuchowisk/gier) kompletne narzędzie gotowe do wczytywania własnych tekstów, szybkiej nawigacji i eksportów produkcyjnych.

### Krok 1.1. Domknięcie Bieżącego WIP (`i18n` PL/EN, Odizolowane Tryby Reader/Studio i Higiena `#topbar`) *(Zrealizowane)*
- **Odizolowane flowy `Mojra` (Reader) vs `Playbook Studio` (`designs/app-modes-navigation.png`, `app/templates/topbar.tpl`)** *(Zrealizowane)*:
  - Sygnet `M` (`mojra.svg`) dla Readera oraz sygnet ołówka i nici (`studio.svg`) dla Studio w lewym górnym rogu (zawsze bez ramki/narożników) — kliknięcie wraca do `Home` danego trybu.
  - Wspólny styl bazowy `.book-card` w Bibliotece i Studio Home, odróżniony miniaturą (okładka + `playbook.svg` + pasek postępu w Bibliotece vs siatka `24px` + `studio.svg` + metryki scen/rozdziałów i ścieżka pliku w Studio Home).
  - Przełącznik między trybem Readera a Studio umieszczony wyłącznie na dole ekranów `Home` pod listą kafelków (nigdy w `#topbar`).
- **Odchudzenie `#topbar` i konsolidacja Opcji (`settings.svg`)** *(Zrealizowane)*:
  - Usunięcie z `#topbar` przycisków `ToggleImageless`, `ToggleUIScale`, `ToggleLanguage` (zaszyte w modalu Opcji z ikonami `theme.svg`, `sun.svg`, `image.svg`, `image-off.svg`) oraz globalnego `PromptResetProgress` (zostaje cofanie przy scenach).
  - W trakcie czytania książki (`#view-player`) brak ikony Opcji — widoczne wyłącznie `Zen`, działający `Fullscreen` (ukryty na Androidzie) oraz `✕` (na desktopie).
- **Kompletna lokalizacja UI (`internal/tpl/i18n/pl.cfg`, `en.cfg`, `internal/gui/i18n.go`, `app/templates/*.tpl`)** *(Zrealizowane)*:
  - Przeniesienie wszystkich kluczy tłumaczeń (`pl` / `en`) dla Go/szablonów i JS do plików `.cfg` ładowanych do tablic zero-alloc przy starcie.
- **Porządek w drzewie roboczym** *(Zrealizowane)*:
  - Usunięcie `audit.log` i zduplikowanych plików słownikowych `.go`, ujednolicenie modułu do `mojra` (`cmd/mojra`), pełny przebieg `./build.sh --audit --strict`, `go test ./...` oraz `./build.sh --macos`.

### Krok 1.2. Wyszukiwarka Pełnotekstowa w Studio (`Cmd+F` / `Ctrl+F`)
- **Indeksowanie i filtrowanie na żywo (`internal/gui/render_editor.go`, `app/js/editor_canvas.js`)**:
  - Błyskawiczne wyszukiwanie po frazie dialogu, prozie, nagłówku `slugline`, nazwie postaci, lokacji lub `id:` sceny.
- **Nawigacja z wyników**:
  - Kliknięcie lub `Enter` na wyniku centruje kamerę płótna WebGL na węźle sceny (`SelectStudioScene`) a `Shift+Enter` otwiera ją bezpośrednio w `#modal-scene-text`.

### Krok 1.3. Importer `.fountain` oraz Importer/Eksporter `.fdx` (Final Draft XML)
- **Importer `.fountain` i `.fdx` (`internal/book/screenplay_fountain.go`, `internal/book/screenplay_fdx.go`)**:
  - Automatyczne rozbicie wczytanego scenariusza po nagłówkach scen (`WN.` / `PL.` / `INT.` / `EXT.`) na gotowy graf węzłów w Playbook Studio z automatycznym układem na siatce `24px`.
  - Obsługa wczytywania plików `.fountain` i `.fdx` bezpośrednio w modalu dodawania/otwierania projektu (`AddBookFromDisk` / `OpenStudioProjectFromDisk`).
- **Eksporter `.fdx` w `#modal-export-project`**:
  - Zapis wybranej ścieżki scenariusza do formatu Final Draft XML (`.fdx` — bez użycia JSON) pod narzędzia kierowników produkcji (Movie Magic Scheduling, Yamdu).

### Krok 1.4. Domknięcie Brakujących Eksportów PDF i Produkcyjnych VO
- **Eksport Drabinki / Treatmentu PDF pod wnioski PISF (`internal/book/screenplay_pdf.go`)**:
  - Jednoklikowy wydruk streszczeń scen (`synopsis`) i podziału na akty do czystego dokumentu A4.
- **Eksport PDF dla Książki (`mode: book`, `internal/book/book_pdf.go`)**:
  - Skład literacki rozdziałów i prozy wzdłuż głównej osi fabularnej na bazie istniejącego silnika PDF 1.4.
- **Eksport Tabeli Nagraniowej VO (`.csv`) i Skryptów Aktorskich PDF (`internal/book/vo_export.go`)**:
  - Wyciąg kwestii z podziałem na postacie, ID scen, wskazówki aktorskie i status plików `voice/<scene_id>.flac` pod nagrania lektorskie.

### Krok 1.5. Sekcyjne Nagrywanie i Kolejkowe Odtwarzanie Voiceoveru dla Tekstu Warunkowego (`designs/conditional-voice-spec.md`, `designs/audio-recorder.png`)
- **Automatyczny podział sceny na sekcje VO (`internal/book/variables_prose.go`)**:
  - Rozbicie prozy sceny zawierającej bloki `if:` / `elif:` / `else:` / `endif:` na uporządkowane sekcje: bloki stałe (`01`, `04`) oraz gałęzie warunkowe (`02_if`, `03_else`).
- **Selektor sekcji w Recorderze i izolacja tekstu (`#modal-scene-text`, `#editor-context-panel`, `app/js/editor_voice.js`)**:
  - Pasek sekcji nad waveformem ze statusem nagrania (`■`/`□`), wyborem aktywnej sekcji i wyświetlaniem wyłącznie tekstu nagrywanej sekcji na czas nagrania.
  - Automatyczne nazywanie, zapis i linkowanie plików `voice/<scene_id>__<sec_id>.flac` z niezależnym `trim` dla każdej sekcji.
- **Sekwencyjne odtwarzanie w Czytniku bez sklejania plików (`internal/gui/render_player.go`, `app/js/player.js`)**:
  - Ewaluacja aktywnych sekcji dla bieżącego stanu zmiennych gracza i przekazanie kolejki plików (`data-scene-audio-queue`) odtwarzanych jeden po drugim z preloadingiem kolejnego segmentu i łącznym paskiem postępu na `#audio-deck`.

### Krok 1.6. Interaktywna Pokazówka WASM na Stronie WWW i Paczka Closed Beta
- **Osadzenie odtwarzacza WASM na `www/` (`build.sh`, `www/index.tpl`, `www/app.js`)**:
  - Podpięcie skompilowanego `cmd/wasmplayer` z pokazową, udźwiękowioną opowieścią `.playbook` uruchamianą po kliknięciu ikony książki na stronie głównej.
- **Zabezpieczenie buildu Closed Beta (`internal/gui/launcher.go`)**:
  - Offline znacznik wygaśnięcia buildu testowego / klucz podpisany `crypto/ed25519` dla paczek instalacyjnych rozsyłanych testerom.

---

## 2. ETAP 2: Pełne Wydanie Komercyjne v1.0 (Po Zamkniętej Becie)

Elementy ciężkie inżynieryjnie, które **nie blokują** startu zamkniętych testów:

### 2.1. Fizyczne Rozdzielenie Binariów `Mojra.app` (Czytnik) i `Playbook Studio`
- Wycięcie na etapie kompilacji (`build tags` / `tpl.BuildPlayerOnlyApp`) płótna WebGL, mutatorów i eksporterów z darmowego czytnika klienckiego `Mojra.app`.
- Rejestracja ikony dokumentu `.playbook` w systemach macOS (`Info.plist`), Windows (zasoby COFF) oraz Linux (MIME).

### 2.2. Samodzielne Aplikacje Desktop/Mobile per Książka (`macOS`, `Windows`, `Linux`, `Android`)
- Eksport pojedynczej książki z zaszyfrowanym strumieniem `STBC` jako gotowej binarki desktopowej lub mobilnej.
- Wybór strategii linearyzacji w zaawansowanych opcjach eksportu (`Główna oś` / `Wszystkie gałęzie` / `Zapis gracza`).

### 2.3. Eksporter Audiobooka Liniowego (`.mp3`/`.ogg`/`.flac`) i Wideo YouTube (`.mp4`)
- Automatyczne łączenie nagrań `voice/` wzdłuż wybranej ścieżki fabularnej z pauzami międzyscenowymi i rozdziałami oraz renderowanie strumienia wideo (`1080p` / `4K` / `Shorts 9:16`) z ilustracjami scen.

### 2.4. Dodatki Scenariuszowe i Czytelnicze v1.0
- **Tryb Rewizji Produkcyjnych (Shooting Script)**: Blokowanie numeracji scen (`14A`) i oznaczanie zmienionych linijek gwiazdkami `*` na podstawie `history.go`.
- **Spis treści (TOC) i zakładki czytnika** w `#view-player`.

