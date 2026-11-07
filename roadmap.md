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
- **Strona Pokazowa WWW (`www/` -> `dist/www/`)**: Kompilowana z `.tpl`/`.tss` statyczna strona z animowanym grafem opowieści w tle, ikoną `.playbook` i formularzem zapisów na zamkniętą betę.

---

## 1. ETAP 1: Lista Zadań do Domknięcia MVP (Closed Beta — Priorytet P0)

Cel: Zamknąć bieżący WIP i dać pierwszym testerom (scenarzystom, autorom słuchowisk/gier) kompletne narzędzie gotowe do wczytywania własnych tekstów, szybkiej nawigacji i eksportów produkcyjnych.

### Krok 1.1. Domknięcie Bieżącego WIP (`i18n` PL/EN i Higiena Repo)
- **Kompletna lokalizacja UI (`internal/gui/i18n*.go`, `internal/tpl/i18n.go`, `app/templates/*.tpl`)**:
  - Dokończenie podpięcia kluczy tłumaczeń (`pl` / `en`) we wszystkich widokach, modalach, komunikatach toast i podpowiedziach.
  - Weryfikacja przełącznika języka (`ToggleLanguage` / `SetLanguage`) w testach integracyjnych (`internal/gui/server_test.go`, `server_js_runtime_test.go`).
- **Porządek w drzewie roboczym**:
  - Usunięcie tymczasowych plików (`audit.log`), pełny przebieg `go test ./...` i `./build.sh --macos` oraz spójny commit zmian.

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

### Krok 1.5. Interaktywna Pokazówka WASM na Stronie WWW i Paczka Closed Beta
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

