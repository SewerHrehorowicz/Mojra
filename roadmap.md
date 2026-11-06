# Mojra & Playbook Studio — Roadmapa (Closed Beta MVP vs v1.0)

## 0. Co Już Działa w Kodzie (Zrealizowane Fundamenty)
- **Silnik Węzłów WebGL 120fps (`app/js/editor_webgl.js`)**: Płynne płótno na siatce `24px`, łączenie portów, przeciąganie scen i rozdziałów.
- **Git dla Pisarzy (`internal/book/history.go`)**: Tworzenie i przełączanie wariantów fabuły na osi czasu z natychmiastowym wyliczaniem różnic produkcyjnych (sceny, strony `1/8`, czas).
- **Raport Produkcyjny i Obsada na Żywo (`internal/book/screenplay.go`)**: Objętość w `1/8` strony, czas ekranowy/audio, dni zdjęciowe/nagraniowe, proporcje `WN./PL.` i `DZIEŃ/NOC`, balans aktów oraz statystyki kwestii i słów dla każdego aktora z globalną zmianą nazw.
- **Zmienne i Warunki (`internal/book/variables.go`)**: Typy `bool`, `int`, `enum`, warunki widoczności wyborów, modyfikacje stanów i interpolacja `{{...}}` w tekście.
- **Polski Standard Scenariuszowy PDF (`internal/book/screenplay_pdf.go`) i Fountain (`.fountain`)**: Eksport A4 Courier 12pt zgodny z PISF (`WN./PL.`, numeracja scen, `(WIĘCEJ)` / `(CIĄG DALSZY)`, strona tytułowa + automatyczna 2. strona `LISTA POSTACI / ZESTAWIENIE LOKACJI`).
- **Czytnik i Odtwarzacz (`#view-library`, `#view-player`)**: Obsługa pakietów `.playbook` / `.stbook`, tryb bez obrazów (`imageless`), regulacja tempa audio i `Auto-Choice` (słuchanie bez dotykania ekranu).
- **Strona Pokazowa WWW (`www/` -> `dist/www/`)**: Kompilowana z `.tpl`/`.tss` statyczna strona z animowanym grafem opowieści w tle, ikoną `.playbook` i formularzem zapisów na zamkniętą betę.

---

## 1. ETAP 1: MVP do Testów w Zamkniętej Grupie (Closed Beta — Priorytet P0)

Cel: Dać pierwszym wybranym twórcom (scenarzystom, autorom słuchowisk/gier i Tobie) kompletne narzędzie do codziennej pracy oraz wczytywania własnych tekstów.

### 1.1. Nagrywanie Lektorskie i Przycinanie Audio w Aplikacji (`designs/audio-recorder.png`)
- **Nagrywanie bezpośrednio w inspektorze i edytorze tekstu**:
  - Ikona mikrofonu (`mic.svg` ze wskaźnikiem 4 narożników w stanie aktywnym) obok `+` w wierszu `voice:` w `#editor-context-panel` (`320px`) oraz w nagłówku pełnoekranowego edytora treści sceny (`#modal-scene-text`, z widocznym `#topbar` i tekstem sceny poniżej do czytania podczas nagrania).
- **Minimalistyczny wiersz edycji i przycinania (Trim)**:
  - Jeden wiersz pod `voice:` (bez ramek, dodatkowych linii podziału, etykiet `START`/`KONIEC` i przycisku `ZAPISZ`): przycisk `play`/`stop`, waveform z cienkich szarych pionowych linii `1px` (`#7a7a7a` obszar aktywny, `#292929` odcięte końcówki), pojedyncza biała pionowa linia `1px` (`playhead`) do przewijania odsłuchu, czas trwania oraz `trash.svg`.
  - Przycinanie początku i końca za pomocą markerów narożnikowych `⎡ ⎣` (lewy) i `⎤ ⎦` (prawy) z automatycznym zapisem wycinka do `voice/<scene_id>.flac` (lub `.ogg`) po zakończeniu nagrania lub puszczeniu markera.

### 1.2. Wyszukiwarka Pełnotekstowa w Projekcie (`Cmd+F` / `Ctrl+F` w Studio)
- Błyskawiczne wyszukiwanie po frazie dialogu, prozie, nazwie postaci, lokacji lub ID sceny.
- Skok kamery płótna WebGL bezpośrednio do wybranego węzła sceny lub otwarcie jej w edytorze tekstu.

### 1.3. Importer `.fountain` oraz Importer/Eksporter `.fdx` (Final Draft XML)
- **Importer `.fountain` i `.fdx`**: Automatyczne rozbicie wczytanego scenariusza po nagłówkach scen (`WN.` / `PL.` / `INT.` / `EXT.`) na gotowy graf węzłów w Playbook Studio (żeby testerzy mogli w 5 sekund wrzucić swój własny 100-stronicowy scenariusz).
- **Eksporter `.fdx`**: Zapis głównej ścieżki scenariusza do formatu Final Draft XML dla kierowników produkcji (Movie Magic Scheduling, Yamdu).

### 1.4. Szybkie Domknięcie Eksportów Tekstowych PDF
- **Eksport Drabinki / Treatmentu PDF pod wnioski PISF (`mode: screenplay`)**: Jednoklikowy wydruk streszczeń scen (`synopsis`) i podziału na akty do czystego dokumentu A4.
- **Eksport PDF z głównej ścieżki dla Książki (`mode: book`, `internal/book/book_pdf.go`)**: Skład literacki rozdziałów i prozy wzdłuż wybranej ścieżki fabularnej na bazie istniejącego silnika PDF 1.4.
- **Eksport Tabeli Nagraniowej VO (`.csv`) i Skryptów Aktorskich PDF**: Wyciąg kwestii z podziałem na postacie, sceny i wskazówki aktorskie pod studia lektorskie i dubbingowe.

### 1.5. Interaktywna Pokazówka WASM na Stronie WWW (`dist/www/`)
- Skompilowanie lekkiego odtwarzacza (`GOOS=js GOARCH=wasm`) z jedną pokazową, udźwiękowioną opowieścią `.playbook`, uruchamianą po kliknięciu pulsującej ikony książki na stronie głównej.

### 1.6. Zabezpieczenie Buildu Zamkniętej Bety
- Prosty, podpisany offline klucz czasowy (`crypto/ed25519`) lub znacznik wygaśnięcia buildu testowego dla paczek rozsyłanych testerom.

---

## 2. ETAP 2: Pełne Wydanie Komercyjne v1.0 (Po Zamkniętej Becie)

Elementy ciężkie inżynieryjnie, które **nie blokują** startu zamkniętych testów:

### 2.1. Fizyczne Rozdzielenie Binariów `Mojra.app` (Czytnik) i `Playbook Studio`
- Wycięcie na etapie kompilacji (`build tags` / `tpl.BuildPlayerOnlyApp`) płótna WebGL, mutatorów i eksporterów z darmowego czytnika klienckiego `Mojra.app`.
- Rejestracja ikony dokumentu `.playbook` w systemach macOS (`Info.plist`), Windows (zasoby COFF) oraz Linux (MIME).

### 2.2. Przebudowany Modal Multi-Exportu (`#modal-export-project`, `designs/export-modal.png`)
- Wspólny rejestr `Exporter` (`internal/book/export.go`) obsługujący wsadowy eksport wielu formatów naraz (Widok Podstawowy + rozwijana belka `ZAAWANSOWANE` z wyborem strategii linearyzacji `Główna oś` / `Wszystkie gałęzie` / `Zapis gracza`).

### 2.3. Polimorficzny Bytecode `STBC` i Samodzielne Aplikacje (`macOS`, `Windows`, `Linux`, `Android`)
- Kompilator drzewa `Book` do zaszyfrowanego strumienia binarnego `STBC` (`AES-256-GCM` + losowa permutacja opcodów per-build + podpis `ed25519`).
- Eksport pojedynczej książki jako samodzielnej gry/aplikacji desktopowej lub mobilnej bez parsera `.playtxt`.

### 2.4. Eksporter Audiobooka Liniowego (`.mp3`/`.ogg`/`.flac`) i Wideo YouTube (`.mp4`)
- Automatyczne łączenie nagrań `voice/` wzdłuż wybranej ścieżki fabularnej z pauzami międzyscenowymi i rozdziałami oraz renderowanie strumienia wideo (`1080p` / `4K` / `Shorts 9:16`) z ilustracjami scen.

### 2.5. Dodatki Scenariuszowe i Czytelnicze v1.0
- **Tryb Rewizji Produkcyjnych (Shooting Script)**: Blokowanie numeracji scen (`14A`) i oznaczanie zmienionych linijek gwiazdkami `*` na podstawie `history.go`.
- **Spis treści (TOC) i zakładki czytnika** w `#view-player`.
