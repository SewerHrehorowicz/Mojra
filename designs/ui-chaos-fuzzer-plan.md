# Plan Wdrożenia: Statyczny Crawler UI, Chaos Engineering i Fuzzer Protokołu WFP (`internal/gui/server_chaos_fuzz_test.go`)

## 1. Cel i Założenia Architektoniczne
- **Pełne pokrycie interakcji UI**: Automatyczny obchód wszystkich widoków (`#view-library`, `#view-player`, `#view-editor`), paneli bocznych (`320px`) oraz modali (`#modals-root`, w tym `#modal-scene-text`) z symulacją każdego kliknięcia, zmiany pól formularzy, `.custom-select` i gestów WebGL canvasu.
- **Deterministyczna inżynieria chaosu (PRNG `seed`)**: Losowe oraz złośliwe sekwencje akcji `POST /api/action` (WFP), wywołania na nieaktualnym DOM (stale state), równoległe żądania (`go test -race`) i uszkodzone ładunki — w 100% odtwarzalne z pojedynczego numeru `seed`.
- **Zero JSON / Zero Zewnętrznych Zależności**: Cały harness działa w oparciu o `go test`, wbudowany `httptest.Server` oraz istniejący mostek Node VM (`internal/gui/server_js_runtime_test.go`), a logi śladu zapisywane są w czystym formacie tekstowym (`key: value` / wiersze zdarzeń).
- **100% izolacja od systemu gospodarza**: Żadna operacja testowa nie ma prawa dotknąć plików repozytorium, `~/Library/Application Support/Mojra`, okien natywnych ani sieci zewnętrznej.

---

## 2. Etap 1 — Piaskownica Bezpieczeństwa (`internal/gui/server_chaos_fuzz_test.go`)

### 2.1. Izolacja Systemu Plików i Środowiska
- Wszystkie katalogi robocze serwera tworzone są wyłącznie wewnątrz `root := t.TempDir()`:
  - `filepath.Join(root, "books")` — kopia testowych książek oraz książki generowane przez fuzzer,
  - `filepath.Join(root, "saves")` — `settings.ini`, pliki `.sav` (`STSV\x01`), blokada `.mojra.pid`,
  - `filepath.Join(root, "history")` — migawki i gałęzie historii wersji,
  - `filepath.Join(root, "exports")` — paczki `.playbook` / `.stbook`, PDF, CSV, Web.
- Wymuszenie `t.Setenv("ST_NO_WINDOW", "1")` przed inicjalizacją `NewServer`:
  - Blokuje wywołania natywnego okna Cocoa/`WKWebView`,
  - Blokuje natywne okna dialogowe wyboru plików/katalogów (`Browse*`),
  - Blokuje inicjalizację fizycznego mikrofonu (`StartSceneVoiceCapture`).
- Strażnik integralności (Post-Test FS Guard):
  - Przed i po teście weryfikowane jest, że `git status --porcelain` w repozytorium oraz mtime plików w `book.DefaultAppDataDir()` pozostały nietknięte.

---

## 3. Etap 2 — Deterministyczny Crawler Wszystkich Interakcji UI (Node VM + `httptest.Server`)

### 3.1. Ekstraktor Elementów Interaktywnych w DOM
Rozszerzenie środowiska z `TestClientJSRuntimeAndInteractiveResponsiveness` o funkcję skanującą aktualne drzewo DOM po każdej odpowiedzi z `/api/action` i budującą listę dostępnych interakcji:
1. **Przyciski i przełączniki akcji**: wszystkie elementy z `[data-action]` oraz `[data-backdrop-action]` wraz z ich atrybutami `data-*`.
2. **Formularze i pola edycji**: `<input>`, `<textarea>` (z obsługą `data-auto-save`, `Enter`, `blur`, `change`) oraz komponenty `.custom-select` (otwarcie triggera -> wybór opcji `.custom-select-option`).
3. **Interakcje na płótnie Node Studio (`#timeline-canvas`)**:
   - Panning i zoom viewportu (`SaveEditorState`),
   - Przeciąganie węzłów na siatce `24px` (`data-node-drag-grip`),
   - Łączenie portów wyjściowych i wejściowych (`ConnectChoice`, `ConnectChapter`),
   - Zaznaczanie pojedyncze i wielokrotne (marquee selection -> `DeleteScenes`, `AssignChapterToScenes`).

### 3.2. Graf Stanów UI i Raport Pokrycia (`ActionSpecs` Coverage)
- Crawler oblicza skrót stanu UI (`StateFingerprint = View + ActiveModal + ActiveRightPanel + SelectedSceneID + SelectedChapterID + HistoryPreviewRev`) i metodą BFS odwiedza każdy osiągalny stan.
- Na koniec przebiegu porównuje zbiór wywołanych akcji z mapą `actionSpecs` (`internal/gui/actions_dispatch.go`) i raportuje pokrycie 100% zarejestrowanych akcji.

---

## 4. Etap 3 — Silnik Chaosu i Fuzzingu Protokołu WFP

### 4.1. Cztery Wektory Chaosu
1. **DOM Monkey / Out-of-Order UI Monkey**:
   - Losowy wybór akcji z aktualnego DOM przy użyciu deterministycznego generatora `math/rand.New(rand.NewSource(seed))`.
   - **Stale DOM Replay**: Fuzzer zapamiętuje przyciski/formularze z poprzednich widoków (np. sprzed 5 kroków — usunięta scena, zamknięty modal, nieistniejąca gałąź historii) i strzela nimi w aktualny stan serwera.
2. **Mutator Ładunków WFP (`POST /api/action`)**:
   - Puste, gigantyczne (kilka MB), binarne i niepoprawne UTF-8 sekcje `_body`,
   - Zduplikowane klucze nagłówków WFP, brakujące znaki `\n\n`, ucięte ramki,
   - Ekstremalne wartości liczbowe (`NaN`, `Inf`, `-999999999`, `1e308`) w `pan_x`, `pan_y`, `zoom`, `positions`, `trim_start`, `trim_end`, `time`,
   - Próby Path Traversal (`../../etc/passwd`, `..\windows\system32`, nazwy z `\x00`) w `book_id`, `scene_id`, `chapter`, `voice`, `image`,
   - Wstrzyknięcia składni `.playtxt` (`->`, `\->`, `if:`, `endif:`, `chapter:`, wieloliniowe nagłówki) do pól tytułów, identyfikatorów i zmiennych.
3. **Chaos Współbieżności (`go test -race`)**:
   - Równoległe strzały z `N = 8` gorutyn w ten sam `httptest.Server`: jednoczesny `SaveAudioTime` (patchowanie in-place bajtów `[5..21]` w `.sav`), `AutoSaveScene`, `UndoEditor`/`RedoEditor`, `SaveHistoryVersion` oraz `SwitchView`.
4. **Fuzzing Walidacji Mediów i Archiwów**:
   - Wysyłanie podrobionych nagłówków magic-bytes (`.svg`, `.gif`, `.wav`, poliglotów HTML/JS udających `.png`/`.flac`) do akcji importu mediów i książek.

---

## 5. Etap 4 — Wyrocznie Inwariantów (Oracles) Po Każdym Kroku

Po **każdym** pojedynczym kliknięciu lub zmutowanym żądaniu fuzzer weryfikuje twarde reguły architektury Mojra:
1. **Brak awarii serwera i JS**:
   - Brak `panic` po stronie Go oraz brak nieobsłużonych wyjątków w Node VM,
   - Kod odpowiedzi HTTP musi należeć do dozwolonego zbioru (`200 OK`, `204 No Content`, `400 Bad Request`, `403 Forbidden`), nigdy `500 Internal Server Error`.
2. **Nienaruszalność Paska Górnego (`#topbar` Invariant)**:
   - `header.topbar#topbar` zawsze istnieje w DOM, ma `grid-row: 1`, `z-index: 200`, nie jest ukryty i żaden `.modal-backdrop` nie ma `position: fixed`.
3. **Kontrakt WFP i HTML-Over-The-Wire**:
   - Nagłówek `X-Target` zawsze należy do `{#app, #workspace, #modals-root, #editor-context-panel, none}`,
   - Zwrócony fragment HTML kompiluje się bez surowych znaczników `{...}` i bez niedozwolonych tagów (`<script>`, `<style>`, `style=`, `<svg>` inline).
4. **Spójność Danych na Dysku (`.playtxt` / `.sav` / `.editor.cfg` Round-Trip)**:
   - Po seriach mutacji książka na dysku w `t.TempDir()` musi przejść `book.LoadBookFromDir` bez uszkodzenia struktury pliku,
   - Pojedyncze przejścia zawsze normalizują się do niejawnego `Dalej`, a przy `narrated: false` żaden plik `.playtxt` nie zawiera klucza `voice:`.

---

## 6. Etap 5 — Tekstowy Log Śladu Wykonania (`ui-chaos-trace.log`)

Zgodnie z regułą Zero JSON, fuzzer zapisuje pełny, deterministyczny ślad każdego kroku do pliku tekstowego (domyślnie w `t.TempDir()` lub do ścieżki wskazanej przez zmienną środowiskową `MOJRA_CHAOS_LOG=build/ui-chaos-trace.log`):

```txt
seed: 1731051025
mode: chaos-monkey
steps: 2500

step: 0001 | view: library | modal: none | el: button.topbar-tab[data-action=SwitchView]
  req:  action=SwitchView view=editor
  res:  status=200 target=#app theme=dark scale=100 toast=""
  dom:  topbar=ok nodes=12 warnings=0 disk_check=ok (1.4ms)

step: 0002 | view: editor | modal: none | el: div.timeline-node-card[data-scene-id=sluza_1]
  req:  action=OpenSceneTextModal scene_id=sluza_1
  res:  status=200 target=#modals-root theme=dark scale=100 toast=""
  dom:  topbar=ok modal=#modal-scene-text disk_check=ok (0.9ms)
```

W przypadku wykrycia błędu log kończy się sekcją `reproducer:`, która wypisuje dokładną komendę:
`go test ./internal/gui -run TestUIChaosAndStaticCrawler -v -args -chaos-seed=1731051025 -chaos-steps=2500`
pozwalającą odtworzyć identyczną sekwencję krok po kroku.

---

## 7. Etap 6 — Harmonogram Implementacji i Komendy Uruchomieniowe

1. **Krok 1 (`internal/gui/server_chaos_fuzz_test.go`)**: Utworzenie piaskownicy `newIsolatedChaosServer(t)` z pełną izolacją `t.TempDir()`, `ST_NO_WINDOW=1` i loggerem tekstowym `ChaosTraceLogger`.
2. **Krok 2 (Crawler DOM w Node VM)**: Dodanie pętli introspekcji DOM i automatycznego obchodu wszystkich akcji z `actionSpecs` z zapisem logu każdego kliknięcia.
3. **Krok 3 (Generator Chaosu + Wyrocznie)**: Dodanie generatora losowych/Stale-DOM sekwencji, mutatora ramek WFP, testu współbieżności `-race` oraz weryfikatorów inwariantów (`#topbar`, parser `.playtxt`, nagłówki `X-*`).
4. **Krok 4 (Weryfikacja)**:
   - Szybki przebieg w standardowym `go test ./...` (np. pełny obchód statyczny + 300 kroków chaosu w `< 2s`),
   - Głęboki przebieg na żądanie: `MOJRA_CHAOS_LOG=build/ui-chaos.log MOJRA_CHAOS_STEPS=10000 go test -race -v ./internal/gui -run TestUIChaosAndStaticCrawler`.
