# Plan Wdrożenia: Sekcyjny Recorder i Kolejkowe Odtwarzanie Voiceoveru dla Tekstu Warunkowego

## 1. Cel i Założenia Architektoniczne
- **Zero ręcznego cięcia i nazywania plików**: Scena zawierająca bloki warunkowe (`if:` / `elif:` / `else:` / `endif:`) jest automatycznie rozbijana przez parser AST na uporządkowane sekcje głosowe (`VOSection`).
- **Izolacja tekstu podczas nagrywania**: Po wybraniu sekcji w `.voice-rec-strip` lektor widzi poniżej wyłącznie prozę tej jednej sekcji, co eliminuje pomyłki przy czytaniu sąsiednich gałęzi warunkowych.
- **Automatyczne nazywanie i linkowanie `.flac`**: Każda sekcja otrzymuje deterministyczną ścieżkę `voice/<scene_id>__<sec_key>.flac` (np. `voice/komora_topielcow__01.flac`, `voice/komora_topielcow__02_if.flac`, `voice/komora_topielcow__03_else.flac`, `voice/komora_topielcow__04.flac`) oraz własny niedestrukcyjny zakres przycięcia `[trim_start, trim_end]`.
- **Odtwarzanie jeden po drugim bez sklejania plików**: Czytnik (`#view-player`) nie dekoduje ani nie skleja plików `.flac` w RAM — otrzymuje z backendu uporządkowaną kolejkę aktywnych sekcji dla bieżącego stanu zmiennych gracza i odtwarza je sekwencyjnie z preloadingiem następnego segmentu.

---

## 2. Etap 1 — Parser Sekcji Głosowych i Model Danych (`internal/book/`)

### 2.1. Struktury w `internal/book/model.go` i `internal/book/variables_prose.go`
- Dodanie struktury `VoiceSection`:
  - `Index int` (`1..N`),
  - `Key string` (deterministyczny sufiks: `01`, `02_if`, `03_elif`, `04_else`, `05`),
  - `Kind string` (`"static"`, `"if"`, `"elif"`, `"else"`),
  - `Condition string` (wyrażenie warunkowe dla `if` / `elif`),
  - `Label string` (etykieta w pasku sekcji: np. `01 · WSTĘP`, `02 · IF: SUMIENIE > 0`, `03 · ELSE`, `04 · FINAŁ`),
  - `Text string` (czysty tekst prozy należący wyłącznie do tej sekcji),
  - `AudioFile string` / `AudioURL string` / `TrimStart float64` / `TrimEnd float64` / `DurationSec float64`.
- Funkcja `ExtractSceneVoiceSections(sc *Scene, b *Book) []VoiceSection`:
  - Jeśli scena nie posiada dyrektyw `if:` / `elif:` / `else:` / `endif:`, zwraca pojedynczą sekcję odpowiadającą całemu `sc.Text` (`voice/<scene_id>.flac`) — zachowując 100% kompatybilności wstecznej.
  - Jeśli scena posiada bloki warunkowe, dzieli linie `sc.Text` na kolejne odcinki statyczne i gałęzie warunkowe.

### 2.2. Serializacja w `.playtxt` i Bytecode `STBC` (`internal/book/parser.go`, `serializer.go`, `stbc.go`)
- Domyślne auto-linkowanie po konwencji nazewniczej: obecność pliku `voice/<scene_id>__<sec_key>.flac` w katalogu `voice/` automatycznie wiąże go z sekcją `<sec_key>` podczas `FinalizeBook(b)`.
- Opcjonalne wpisy `voice:` wewnątrz bloków `if:` / `elif:` / `else:` w `.playtxt` (gdy autor wybierze niestandardowy plik lub posiada niezerowy `trim: [start..end]` na sekcji):
  ```txt
  if: sumienie > 0
  voice: voice/komora_topielcow__02_if.flac [0.0420..0.9610]
  Krzysiowi zadrżała dłoń na żelaznym pręcie...
  else:
  voice: voice/komora_topielcow__03_else.flac
  Cofnął stopę i uniósł ciężką żeliwną klapę...
  endif:
  ```
- Walidacja `narrated: true`: ostrzeżenie parsera wskazuje dokładnie, której sekcji warunkowej brakuje nagrania `.flac`.
- Kompilator `STBC` (`internal/book/stbc.go`) pakuje wszystkie pliki sekcyjne `voice/<scene_id>__*.flac` do archiwum `.playbook` / `.stbook` oraz Web/WASM.

---

## 3. Etap 2 — Selektor Sekcji i Izolacja Tekstu w Recorderze (`internal/gui/`, `app/templates/`, `app/js/editor_voice.js`)

### 3.1. Stan Serwera i Akcje WFP (`internal/gui/server.go`, `actions_editor_scene.go`, `actions_dispatch.go`)
- Pole `SelectedVoiceSectionKey string` w `Server` (resetowane przy zmianie sceny na pierwszą sekcję `01`).
- Nowa akcja `SelectSceneVoiceSection` (`Target: targetActiveWorkspaceOrModalsWhenSceneTextOpen`):
  - Przełącza aktywną sekcję w dyktafonie, aktualizuje `data-voice-url`, `data-trim-start`, `data-trim-end` oraz przełącza wyświetlany tekst w `#modal-scene-text` na prozę wybranej sekcji.
- Rozszerzenie akcji `StopSceneVoiceCapture`, `SaveSceneVoiceRecording`, `TrimSceneVoice`, `DeleteSceneVoice`:
  - Odczytują parametr `voice_section` (np. `02_if`) i zapisują/przycinają/usuwają plik `voice/<scene_id>__<voice_section>.flac`.

### 3.2. Interfejs w `#modal-scene-text` i `#editor-context-panel` (`designs/audio-recorder.png`)
- Nad `.voice-rec-strip` (widoczny wyłącznie, gdy scena ma `> 1` sekcję i dyktafon jest otwarty) renderowany jest minimalistyczny pasek `.voice-rec-sections`:
  - Przyciski sekcji ze wskaźnikiem nagrania (`■` biały kwadrat `6×6px` gdy plik `.flac` istnieje, `□` pusty obrys gdy brak nagrania), nazwą sekcji (`02 · IF: SUMIENIE > 0`), czasem trwania (`0:09` lub `—`) i białym podkreśleniem `1px` pod aktywną sekcją.
  - Po prawej stronie paska wyświetlana jest docelowa ścieżka pliku (`voice/komora__02_if.flac`).
- **Izolacja tekstu nagrywanej sekcji**:
  - Gdy `.voice-rec-strip` jest otwarty w scenie wielosekcyjnej, obszar prozy pod dyktafonem wyświetla wyłącznie tekst aktywnie wybranej sekcji (`SEKCJA 02 / 04 · IF: SUMIENIE > 0`).
  - Zamknięcie dyktafonu (`ToggleSceneVoiceRecorder`) natychmiast przywraca pełny edytor tekstu całej sceny.

---

## 4. Etap 3 — Sekwencyjne Odtwarzanie w Czytniku bez Sklejania (`internal/gui/render_player.go`, `app/js/player.js`)

### 4.1. Ewaluacja Aktywnej Kolejki Audio w Go (`internal/book/variables_prose.go`, `internal/gui/render_player.go`)
- Funkcja `EvaluateSceneAudioQueue(sc *Scene, state map[string]string, b *Book) []VoiceQueueItem`:
  - Przechodzi przez `ExtractSceneVoiceSections(sc, b)` i wybiera wyłącznie sekcje aktywne dla bieżącego stanu zmiennych `state` (np. `[01, 02_if, 04]`).
  - Zwraca zakodowaną specyfikację kolejki `data-scene-audio-queue="url1|ts1|te1|dur1;url2|ts2|te2|dur2;..."` oraz łączny czas trwania wszystkich aktywnych segmentów (`∑ dur_i`, np. `0:29`).

### 4.2. Odtwarzacz Kolejkowy z Preloadingiem (`app/js/player.js`)
- `player.js` parsuje `data-scene-audio-queue` na listę segmentów `[{url, trimStart, trimEnd, durationSec}, ...]`.
- Podczas odtwarzania segmentu `i` w `#story-audio`, pomocniczy bufor `nextAudioPreload` w tle buforuje plik segmentu `i+1` ustawiony na jego `trimStart`.
- W momencie osiągnięcia `trimEnd` (lub `ended`) segmentu `i`, odtwarzacz natychmiast przełącza źródło na segment `i+1` bez przerwy i po zakończeniu ostatniego segmentu wyzwala `Auto-Choice` (jeśli aktywne).
- Suwak `#audio-seek-slider` i licznik `#audio-time-current / #audio-time-total` operują na łącznym czasie całej kolejki (`0..totalDuration`), mapując pozycję suwaka w czasie `O(K)` na właściwy segment i jego lokalny offset czasowy.

---

## 5. Etap 4 — Eksport Tabeli VO i Testy Automatyczne
- **`internal/book/vo_export.go`**: `CollectProjectVOCues` wypisuje osobny wiersz CSV / kwestię w skrypcie PDF dla każdej sekcji warunkowej wraz z jej docelową nazwą pliku `voice/<scene_id>__<sec_key>.flac`.
- **Testy (`internal/book/variables_test.go`, `internal/gui/server_editor_test.go`, `server_js_runtime_test.go`)**:
  - Test podziału sceny z `if:/elif:/else:` na sekcje i deterministycznego nazywania plików `.flac`.
  - Test nagrywania, przycinania i usuwania audio dla wybranej sekcji warunkowej oraz izolacji tekstu w `#modal-scene-text`.
  - Test kolejki `data-scene-audio-queue` w `player.js` (automatyczne przejście między segmentami `01 -> 02_if -> 04` oraz przewijanie suwakiem po łącznym czasie).

---

## 6. Plan Wdrożenia Zmian `#topbar` oraz Odizolowanych Trybów `Reader` vs `Playbook Studio` (`designs/app-modes-navigation.png`)

### 6.1. Dwa Odizolowane Flowy: `Mojra` (Reader) vs `Playbook Studio`
1. **Tryb `Reader` (`Mojra`)**:
   - **Sygnet w lewym górnym rogu `#topbar`**: ikona `mojra.svg` (`M`) — **nigdy nie jest w ramce ani w narożnikach `[ ]`**.
   - **Reader Home (`#view-library`)**:
     - Wyświetla sygnet `M` + napis `Mojra` po lewej (brak przycisku `<- Biblioteka`, gdy użytkownik już jest w Bibliotece).
     - Po prawej stronie `#topbar`: wyłącznie `Opcje` (`settings.svg`), `Fullscreen` (`fullscreen.svg` — tylko na desktopie) i `Zamknij` (`close.svg` — tylko na desktopie).
   - **Czytanie książki (`#view-player`)**:
     - Kliknięcie w sygnet `M` (`mojra.svg`) w lewym górnym rogu wraca do `Reader Home` (`#view-library`). Obok sygnetu wyświetla się tytuł czytanej książki.
     - Po prawej stronie `#topbar` podczas czytania: **wyłącznie** `Zen` (`zen.svg`), działający `Fullscreen` (`fullscreen.svg` / `windowed.svg` — tylko na desktopie) oraz `Zamknij` (`close.svg` — tylko na desktopie).
     - **Android / Mobile**: brak przycisku `Fullscreen` i brak przycisku `Zamknij` (zostaje wyłącznie `Zen`).
     - **Brak w `#topbar` podczas czytania**: ikony `Opcje` (`settings.svg`), globalnego przycisku resetu (`PromptResetProgress` — wystarcza cofanie przy poszczególnych scenach na osi historii) oraz przełącznika do Studio.

2. **Tryb `Studio` (`Playbook Studio`)**:
   - **Sygnet w lewym górnym rogu `#topbar`**: ikona `studio.svg` (ołówek z nicią grafu) — **nigdy nie jest w ramce ani w narożnikach `[ ]`**.
   - **Studio Home (`#view-studio-home`)**:
     - Wyświetla sygnet `studio.svg` + napis `Playbook Studio` po lewej oraz akcje `+ Nowa książka`, `+ Nowy scenariusz`, `Otwórz projekt z dysku`.
     - Po prawej stronie `#topbar`: wyłącznie `Opcje` (`settings.svg`), `Fullscreen` (`fullscreen.svg`) i `Zamknij` (`close.svg`).
   - **Edytor projektu (`#view-editor`)**:
     - Kliknięcie w sygnet `studio.svg` w lewym górnym rogu wraca do `Studio Home`. Obok sygnetu wyświetla się tytuł edytowanego projektu.
     - Brak jakiegokolwiek przycisku przełączającego do Biblioteki/Readera z poziomu otwartego edytora.

### 6.2. Przełączanie Trybów Wyłącznie na Dole Ekranów `Home` (Nigdy w `#topbar`)
- Z `#topbar` całkowicie znikają środkowe zakładki nawigacyjne (`#tab-library`, `#tab-player`, `#tab-editor`).
- Przełączenie między środowiskiem czytnika a środowiskiem autorskim znajduje się **wyłącznie na dole widoku `Home` pod listą kafelków**:
  - W `Reader Home` (na dole pod książkami): dyskretny wiersz z ikoną `studio.svg` i akcją `Przełącz na środowisko autorskie Playbook Studio →`.
  - W `Studio Home` (na dole pod projektami): dyskretny wiersz z ikoną `mojra.svg` i akcją `Przełącz na czytnik i bibliotekę Mojra →`.

### 6.3. Wspólny Styl Bazowy Kafelków `.book-card` z Wyraźnym Rozróżnieniem Trybu
- Zarówno `Reader Home`, jak i `Studio Home` używają tego samego bazowego układu wiersza `.book-card` (`grid-template-columns: 120px 1fr`, `border-bottom: 1px solid --border-subtle`, brak zewnętrznych ramek wokół kart), ale różnią się zawartością:
  - **W Bibliotece (`Reader Home`)**:
    - Kwadrat `120×120px` z okładką książki i centralną ikoną `playbook.svg` (otwarta książka z trójkątem Play).
    - W wierszu metadanych: imię i nazwisko autora + pasek postępu czytania (`progress.book-progress-bar`), poniżej tytuł i opis fabularny.
  - **W Studio (`Studio Home`)**:
    - Kwadrat `120×120px` z tłem siatki węzłów `24px` i centralną ikoną `studio.svg` (ołówek z nicią grafu).
    - W wierszu metadanych: typ projektu (`KSIĄŻKA INTERAKTYWNA` / `SCENARIUSZ FILMOWY`), liczba scen, liczba rozdziałów/aktów, status narracji `.flac` / objętość w `1/8` strony oraz poniżej tytuł i ścieżka katalogu projektu (`books/<id>/book.playtxt`).

### 6.4. Konsolidacja Ustawień w Modalu Opcji (`settings.svg`)
- Z `#topbar` zostają usunięte pojedyncze przełączniki:
  - `ToggleImageless` (`#btn-toggle-imageless`),
  - `ToggleUIScale` (`#btn-toggle-ui-scale`),
  - `ToggleLanguage` (`#btn-toggle-lang`),
  - `ToggleTheme` (`#icon-theme-sun` / `#icon-theme-moon`),
  - `PromptResetProgress`.
- Wszystkie te ustawienia są obsługiwane wewnątrz modalu Opcji (`modals.tpl`) przy użyciu istniejących kontrolek i ikon z `app/icons/*.svg`:
  - `język:` — radio `PL` / `EN` (`8×8px` kwadraty `.form-radio-sq`),
  - `motyw:` — `.form-toggle-btn` z ikonami `theme.svg` / `sun.svg` i suwakiem `22×10px`,
  - `obrazy:` — `.form-toggle-btn` z ikonami `image.svg` / `image-off.svg` i suwakiem `22×10px`,
  - `skala ui:` — radio `Domyślny` / `Duży`,
  - `czułość przewijania:` — radio `Wysoka` / `Średnia` / `Niska` / `Wył.`,
  - `opóźnienie auto-wyboru:` — radio `1s` / `2s` / `3s` / `5s`.

