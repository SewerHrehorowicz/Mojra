# Mojra & Playbook Studio — Roadmapa MVP

## 1. Architektura Produktowa: Rozdzielenie `Mojra.app` i `Playbook Studio`
- **`Mojra.app` (Czytnik i Odtwarzacz)**:
  - Lekka aplikacja kliencka zawierająca wyłącznie Bibliotekę (`#view-library`) oraz Odtwarzacz (`#view-player`).
  - Całkowite wycięcie na etapie kompilacji (`build tags` / `tpl.BuildPlayerOnlyApp`) płótna WebGL, inspektora węzłów, mutatorów dyskowych (`internal/book/mutator.go`) oraz eksporterów.
  - Otwieranie wyłącznie podpisanych pakietów `.playbook` / `.stbook` w trybie tylko do odczytu.
- **`Playbook Studio` (Środowisko Autorskie i Produkcyjne)**:
  - Pełne IDE dla twórców książek interaktywnych, słuchowisk i scenariuszy filmowych (WebGL 120fps, Git dla pisarzy, Raport Produkcyjny i Obsada, Zmienne i Warunki, Silnik Multi-Exportu).
  - Jedyne narzędzie posiadające parser źródeł `.playtxt` oraz klucz podpisujący kompilowane paczki.

---

## 2. Silnik Multi-Exportu i Zaszyfrowany Runtime (`STBC`) — Plan Implementacji

Specyfikacja wizualna interfejsu znajduje się w `designs/export-modal.png`.

### 2.1. Kontrakt Architektoniczny (`internal/book/export.go`)
Wspólny rejestr eksporterów wywoływany wsadowo (wiele celów w jednym przebiegu):

```go
type ExportOptions struct {
	Targets        []string // np. ["playbook", "web", "mp3", "mp4", "pdf", "fountain", "macos", "windows", "android", "linux"]
	OutputDir      string   // katalog docelowy zapamiętywany per projekt w settings.ini
	PathStrategy   string   // "primary" (Główna oś), "all" (Wszystkie gałęzie), "save" (Zapis gracza)
	AudioCodec     string   // "mp3", "ogg", "flac"
	VideoFormat    string   // "1080p", "4k", "shorts"
	ScriptStandard string   // "pl" (PISF A4), "eu" (A4), "us" (Letter)
}

type Exporter interface {
	ID() string
	Label(b *Book) string
	Extension(opts ExportOptions) string
	Tier() string                    // "basic" lub "advanced"
	SupportsBook(b *Book) bool       // filtrowanie kontekstowe wg mode, narrated, imageless
	Export(b *Book, opts ExportOptions) (outPath string, err error)
}
```

### 2.2. UX Modalu Multi-Exportu (`#modal-export-project`)
Modal dzieli opcje na dwa poziomy, eliminując przeładowanie interfejsu:

1. **Widok Podstawowy (domyślny, multi-select na kwadratach `8×8px`)**:
   - **Książka z narracją i obrazami (`mode: book`, `narrated: true`, `imageless: false`)**:
     - `Playbook (.playbook)`
     - `Aplikacja Web (.html + .wasm)`
     - `Audiobook (.mp3)`
     - `Wideo YouTube (.mp4)`
   - **Książka bez obrazów (`imageless: true`)**: automatyczne ukrycie `Wideo YouTube (.mp4)`.
   - **Książka bez narracji (`narrated: false`)**: automatyczne ukrycie `Audiobook (.mp3)` i `Wideo YouTube (.mp4)`.
   - **Scenariusz (`mode: screenplay`)**:
     - `Scenariusz PDF (.pdf)` (domyślnie zaznaczony)
     - `Playbook (.playbook)`
     - `Aplikacja Web (.html + .wasm)`
   - Wiersz wyboru katalogu `folder:` oraz belka rozwijająca `ZAAWANSOWANE`.

2. **Widok Zaawansowany (rozwijany na żądanie)**:
   - **Aplikacje natywne (`aplikacje:`)**: wybór targetów `macOS (.app)`, `Windows (.exe)`, `Android (.apk)`, `Linux (x64)`.
   - **Druk książki (`druk:`, dla `mode: book`)**: `PDF z głównej ścieżki (.pdf)` — skład literacki głównej osi fabularnej do druku/e-booka.
   - **Formaty scenariuszowe (`scenariusz:` i `norma:`, dla `mode: screenplay`)**: `Scenariusz Fountain (.fountain)` oraz wybór normy `Polska — PISF`, `Europa (A4)`, `USA (Letter)`.
   - **Linearyzacja fabuły (`ścieżka:`)**: wybór przebiegu dla formatów liniowych (Audiobook, Wideo YouTube, PDF książki): `Główna oś`, `Wszystkie gałęzie`, `Zapis gracza`.
   - **Parametry kodeków (`audio:`, `wideo:`)**: wybór `MP3` / `OGG Vorbis` / `FLAC` oraz `1080p 16:9` / `4K 16:9` / `Shorts 9:16`.

### 2.3. Zabezpieczenie Runtime'u i Ochrona przed Inżynierią Wsteczną (`STBC`)
Wyeksportowane aplikacje (`Web WASM`, `macOS`, `Windows`, `Linux`, `Android`) nie mogą służyć jako darmowy silnik do uruchamiania obcych plików z pominięciem Studia:

1. **Brak parsera `.playtxt` i brak loadera `.stbook` w wyeksportowanej grze**:
   - Szablon wykonawczy (`player-runtime`) ma wycięty kod `ParsePlaytxt`, `LoadBookFromDir` oraz `zip.Reader`.
2. **Kompilacja do Bytecode'u (`STBC` — StoryTeller Bytecode)**:
   - Studio kompiluje strukturę `Book` (graf scen, już przetranspilowane bloki HTML prozy, warunki zmiennych AST i indeksy mediów) do zwartego strumienia binarnego.
3. **Polimorfizm per-build (losowa tablica opcodów)**:
   - Podczas każdego eksportu Studio generuje kryptograficznie losową permutację opcodów instrukcji warunkowych i rekordów scen oraz jednorazowy klucz `AES-256-GCM`.
4. **Bindowanie i podpis (`crypto/ed25519`)**:
   - Zaszyfrowany blok `STBC` wraz z mediami jest doklejany bezpośrednio na koniec szablonu binarki natywnej lub wtapiany w sekcję danych `.wasm` ze stopką `[offset][klucz_z_maską][podpis_ed25519][magic: STRT]`.

### 2.4. Etapy Wdrożenia Eksporterów (Fazy 1–5)
- **Faza 1 — Rejestr `Exporter` i przebudowa `#modal-export-project`**:
  - Zastąpienie pojedynczego dropdownu w `app/templates/modals.tpl` listą wielokrotnego wyboru z sekcją `Zaawansowane`.
  - Obsługa wsadowego wykonywania wybranych eksporterów w `exportProjectAction` z raportowaniem postępu.
- **Faza 2 — Kompilator `STBC` + Szyfrowanie `AES-256-GCM` (`internal/book/stbc.go`)**:
  - Serializer i deserializer drzewa `Book` do postaci binarnej z losowaniem tablicy opcodów i testami round-trip.
- **Faza 3 — Eksport Web (`GOOS=js GOARCH=wasm`) i Standalone Desktop**:
  - Budowa prekompilowanego szablonu `.wasm` oraz binarek bazowych (`darwin`, `windows`, `linux`) z `tpl.BuildPlayerOnlyApp`.
  - Pakowacz doklejający zaszyfrowany payload `STBC` bez wymogu posiadania kompilatora Go na maszynie autora.
- **Faza 4 — Eksporter PDF z głównej ścieżki dla Książki (`internal/book/book_pdf.go`)**:
  - Zastosowanie silnika PDF 1.4 (`screenplay_pdf.go` z obsługą polskich znaków UTF-8 / ToUnicode CMap) do składu rozdziałów i prozy wzdłuż wybranej ścieżki (`Główna oś` / `Zapis gracza`).
- **Faza 5 — Eksporter Audiobooka (`.mp3`/`.ogg`/`.flac`) i Wideo YouTube (`.mp4`)**:
  - Linearyzator ścieżki scen łączący pliki lektorskie `voice/` z pauzami międzyscenowymi i znacznikami rozdziałów.
  - Generator strumienia wideo łączący ilustracje scen (`image:` / okładkę) ze ścieżką audio w kontener `.mp4` (`1080p` / `4K` / `9:16`).

---

## 3. Pozostałe Funkcjonalności Wymagane do MVP

### 3.1. Książka Interaktywna i Słuchowisko (`mode: book`)
- **Nagrywanie lektorskie i przycinanie audio bezpośrednio w aplikacji (`designs/audio-recorder.png`)**:
  - Ikona mikrofonu (`mic.svg` ze wskaźnikiem 4 narożników w stanie aktywnym) obok `+` w wierszu `voice:` w inspektorze (`#editor-context-panel`, `320px`) oraz w nagłówku edytora treści sceny (`#modal-scene-text`, z widocznym `#topbar` i tekstem prozy poniżej do czytania podczas nagrania).
  - Jeden minimalistyczny wiersz pod `voice:` (bez ramek, dodatkowych linii podziału, etykiet `START`/`KONIEC` i przycisku `ZAPISZ`): przycisk `play`/`stop`, waveform z cienkich szarych pionowych linii `1px` (`#7a7a7a` obszar aktywny, `#292929` odcięte końcówki), pojedyncza biała pionowa linia `1px` (`playhead`) do klikania i przewijania odsłuchu, czas trwania oraz `trash.svg`.
  - Przycinanie początku i końca za pomocą markerów narożnikowych `⎡ ⎣` (lewy) i `⎤ ⎦` (prawy) identycznych z zaznaczeniem aktywnej ikony, z automatycznym zapisem wycinka do `voice/<scene_id>.flac` po zakończeniu nagrania lub puszczeniu markera.
- **Zmienne i warunki w narracji (`var:`, `enum:`, `{{...}}`)**:
  - Pełna ewaluacja warunków widoczności wyborów, modyfikacji stanów i interpolacji tekstu w odtwarzaczu oraz podglądzie Studia.
- **Spis treści (TOC) i zakładki czytnika**:
  - Szybki skok do odwiedzonych rozdziałów i zapisanych momentów w `#view-player`.
- **Pełnotekstowe wyszukiwanie w projekcie**:
  - Błyskawiczny skok do sceny po frazie dialogu, opisie lub identyfikatorze w Playbook Studio.

### 3.2. Scenariusz Filmowy, Serialowy i Audio (`mode: screenplay`)
- **Importer `.fountain` oraz Importer/Eksporter `.fdx` (Final Draft XML)**:
  - Bezstratne wczytywanie istniejących scenariuszy z rynku do grafu węzłów Mojry oraz eksport `.fdx` dla kierowników produkcji (Movie Magic Scheduling, Yamdu).
- **Eksport Drabinki / Treatmentu PDF pod wnioski PISF**:
  - Jednoklikowy wydruk streszczeń scen (`synopsis`) i struktury aktów do czystego dokumentu A4.
- **Eksport Tabeli Nagraniowej VO (`.csv`) i Indywidualnych Skryptów Aktorskich PDF**:
  - Wyciąg kwestii z podziałem na postacie, sceny i pliki audio pod studia lektorskie (Audioteka, Empik Go, dubbing gamedev).
- **Tryb Rewizji Produkcyjnych (Shooting Script)**:
  - Blokowanie numeracji scen (`14A`, sceny pominięte) oraz automatyczne oznaczanie zmienionych linijek gwiazdkami `*` na podstawie historii wersji (`history.go`).
- **Rozszerzona strona tytułowa scenariusza**:
  - Pola daty draftu, danych kontaktowych i praw autorskich w `book.playtxt` oraz na pierwszej stronie eksportu PDF.

### 3.3. Dystrybucja i Strona Pokazowa WWW
- **Interaktywny Showcase WASM na stronie głównej (`designs/mojra-www.png`)**:
  - Uruchamianie demonstracyjnego `.playbook` bezpośrednio po kliknięciu ikony Playbook na stronie WWW w oparciu o wyeksportowany runtime WebAssembly.
- **Globalna ikona i powiązanie plików `.playbook` / `.stbook` w systemie OS**:
  - Rejestracja typu dokumentu i ikony otwartej księgi ze znakiem Play w macOS (`Info.plist`), Windows (rejestr / zasoby COFF) oraz Linux (`.desktop` / MIME).
