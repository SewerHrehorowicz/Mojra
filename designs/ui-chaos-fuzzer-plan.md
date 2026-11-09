# Plan Inżynierii Chaosu UI: Fuzzer Znak-po-Znaku, Szybkie Kliki, Mieszanie Węzłów i Wyrocznie Spójności `.playtxt` / `.playbook` (`internal/gui/server_chaos_fuzz_test.go`)

## 1. Cel: Złapać Błędy, Które Autor Trafia w 3 Minuty Pisania
- **Pisanie znak po znaku (Keystroke-by-Keystroke Input Monkey)**: Żadnego sztucznego podmieniania całych stringów w jednym wywołaniu HTTP. Fuzzer wpisuje teksty **litera po literze** (`keydown` → `input` → timer debounce `AutoSaveScene` → kolejna litera → `blur` / `Enter`), wywołując wyścigi między żądaniami `AutoSaveScene` w locie a zmianą `X-Scene-ID` na serwerze.
- **Szybkie kliki w dowolne miejsca (Rapid-Fire Click & Gesture Monkey)**: Serie szybkich kliknięć bez czekania na zakończenie poprzednich animacji/żądań — po węzłach WebGL (`#studio-canvas`), portach, tytułach inline (`#canvas-node-title-input`), modalach (`#modal-scene-text`), `.custom-select` i pasku górnym (`#topbar`).
- **Agresywne mieszanie węzłów i rozdziałów**: Tworzenie wielu nowych scen pod rząd (`s1`, `s2`, `s3`...), zmiana ich tytułów i `explicit_id` w losowej kolejności, przeciąganie na siatce `24px`, przepinanie przewodów (`ConnectChoice`, `ConnectChapter`, `END`), zaznaczanie grupowe (marquee), usuwanie scen w trakcie pisania oraz `UndoEditor` / `RedoEditor` w środku edycji.
- **Dziwne katalogi i nazwy książek**: Tworzenie i przenoszenie projektów w ścieżkach ze spacjami, polskimi znakami diakrytycznymi, myślnikami, kropkami oraz głębokim zagnieżdżeniem (`Nowa Książka — Śluza #1 (Kopia)/podkatalog z spacjami/`).
- **Zero tolerancji dla uszkodzenia `.playtxt` / `.playbook` i zablokowanego UI**: Po każdej serii operacji twarde wyrocznie weryfikują, czy każda scena ma unikalne ID, czy każdy węzeł na płótnie da się kliknąć i wyedytować bez nadpisywania innej sceny oraz czy pliki `.playtxt` i paczka `.playbook` przechodzą bezstratny round-trip.

---

## 2. Etap 1 — Piaskownica Bezpieczeństwa (`newIsolatedChaosServer`)
- Wszystkie operacje dyskowe zamknięte wyłącznie w `root := t.TempDir()` (`books/`, `saves/`, `history/`, `exports/`, `projects/`).
- Wymuszone `ST_NO_WINDOW=1` oraz `MOJRA_DATA_DIR=root` — zero okien natywnych, zero mikrofonu sprzętowego, zero dostępu do `~/Library/Application Support/Mojra` ani plików repozytorium.
- Strażnik integralności (`verifySandboxHostIsolation`) przed i po teście porównuje snapshot `os.UserConfigDir()/Mojra` oraz sprawdza brak plików runtime w repozytorium.

---

## 3. Etap 2 — Symulator Pisania Litera po Literze i Szybkich Klików w Node VM

### 3.1. Keystroke-by-Keystroke Input Monkey (`typeLetterByLetterInVM`)
Dla każdego pola tekstowego w UI (`#canvas-node-title-input`, `#canvas-choice-input`, `#canvas-chapter-input`, `#form-scene-text input[name="name"]`, `input[name="explicit_id"]`, `textarea[name="text"]`, `input[name="chapter"]`, `input[name="choice_label_*"]`, `input[name="choice_target_*"]`, `input[name="choice_cond_*"]`, `input[name="choice_effects_*"]`, `#new-book-title`, `#book-settings-title-input`, pola zmiennych i breakdownu):
1. Ustawia fokus (`focus`) na polu.
2. W pętli po kolejnych znakach docelowego tekstu (`"Z"`, `"Za"`, `"Zat"`, ..., `"Zatopiona śluza"`):
   - Aktualizuje `el.value`,
   - Wysyła zdarzenia `keydown`, `input`, `keyup`,
   - Co kilka znaków wyzwala `AutoSaveScene` **bez czekania** na odpowiedź poprzedniego żądania (symulacja szybkiego pisania, gdy odpowiedź z nowym `X-Scene-ID` wraca w trakcie wpisywania kolejnych liter!).
3. Na końcu wysyła `change` oraz `blur` lub `keydown(Enter)`.
4. W trakcie wpisywania testuje również:
   - Kasowanie znaków (`Backspace` do pustego pola i ponowne wpisywanie),
   - Wklejanie znaków specjalnych (`->`, `\->`, `:`, `;`, `"`, `'`, `Polskie znaki: ąęćłńóśźż ĄĘĆŁŃÓŚŹŻ`),
   - Próby wpisania **identycznego tytułu lub identycznego `explicit_id`** jak w innej istniejącej scenie.

### 3.2. Rapid-Fire Click & Canvas Node Mixer (`mixCanvasNodesAndClickRapidly`)
1. **Wielokrotne dodawanie nowych scen bez edycji**:
   - Kliknięcie `+ Scena` (`CreateSceneAt` / `InsertScene` / `InsertSceneBefore` / `BranchChoiceScene`) 3–6 razy pod rząd, tworząc grupę węzłów z automatycznymi identyfikatorami (`s1`, `s2`, `s3`, `s4`).
2. **Edycja pierwszej nowej sceny (`s1`) znak po znaku przy obecności `s2`, `s3`**:
   - Weryfikacja, że po zmianie tytułu/ID pierwszej sceny (`s1` → `zatopiona_sluza`) żaden in-flight ani kolejny `AutoSaveScene` nie trafi w dawną nazwę `s1` (która mogłaby zostać przydzielona kolejnej scenie) i **nigdy nie nadpisze tytułu ani treści żadnej innej sceny**.
3. **Szybkie klikanie po płótnie WebGL (`#studio-canvas`)**:
   - Klikanie w nagłówek węzła (edycja tytułu inline), w środek karty (zaznaczenie), w ikonę edycji treści (`OpenSceneTextModal`), w przycisk `+` wyboru (`AddChoice`), w etykietę wyboru (`#canvas-choice-input`), w kwadrat `8x8` ścieżki głównej (`SelectPrimaryBranchScene`) oraz w nagłówek ramki rozdziału.
   - Przeciąganie węzłów (`mousedown` → seria `mousemove` → `mouseup` → `SaveEditorState`) na te same lub sąsiednie współrzędne siatki `24px`.
   - Przeciąganie przewodów z portów wyboru do innych scen, innych rozdziałów oraz w pustą przestrzeń (`END`).

### 3.3. Generator Dziwnych Katalogów i Nazw Projektów
- Tworzenie książek (`CreateNewBook`) oraz przenoszenie (`BrowseBookSaveFolder`) i eksport (`ExportProject`) do katalogów o trudnych nazwach wewnątrz `t.TempDir()`:
  - `Katalog ze spacjami i polskimi znakami — Żółć 2026`,
  - `projekt.z.kropkami-i_podkresleniami (kopia #2)`,
  - Głęboko zagnieżdżone drzewo `poziom_1/poziom 2/poziom_3 — śluza/`.

---

## 4. Etap 3 — Wyrocznie Spójności Formatu i Osiągalności UI (Oracles)

Po **każdym** kroku i po **każdej** serii szybkich kliknięć/wpisywania znaków uruchamiane są cztery twarde wyrocznie:

### 4.1. Wyrocznia 1: Zero Zduplikowanych ID (`VerifyUniqueSceneAndChapterIDs`)
- W pamięci (`s.ActiveBook.Scenes`), w wyrenderowanym `#studio-graph-data` (oraz `studioGraphState.nodes` w JS) i po wczytaniu z dysku (`book.LoadBookFromDir`):
  1. Każda scena ma niepusty `sc.ID` (`SanitizeSceneID(sc.ID) != ""`).
  2. Żadne dwie sceny nie mają identycznego `sc.ID` (case-insensitive) ani identycznego niepustego `sc.ExplicitID`.
  3. Nawet jeśli zewnętrzny plik `.playtxt` zostanie ręcznie zapisany z dwoma takimi samymi nagłówkami `scene: ten_sam_id`, `FinalizeBook` emituje ostrzeżenie `Duplikat id sceny...` **oraz automatycznie nadaje drugiej scenie unikalny sufiks (`ten_sam_id_2`)**, dzięki czemu Node Studio nigdy się nie blokuje i obie sceny są w pełni edytowalne w UI.

### 4.2. Wyrocznia 2: 100% Osiągalność Węzłów w UI i Izolacja Edycji (`VerifyEveryNodeSelectableAndIsolated`)
- **Spójność DOM/WebGL z Backendem**: Liczba węzłów w `studioGraphState.nodes` w JS musi być 1:1 równa `len(s.ActiveBook.Scenes)`, a każdy `sc.ID` musi być kluczem w `studioGraphState.nodeById`.
- **Test Izolacji Edycji (Anti-Overwrite Guard)**:
  - Przed edycją sceny `targetID` wyrocznia zapisuje mapę kontrolną wszystkich pozostałych scen: `snapshot[other.ID] = {Name: other.Name, Text: other.Text, Choices: other.Choices}`.
  - Po serii wpisów znak-po-znaku i `AutoSaveScene` dla `targetID`, wyrocznia sprawdza, że:
    1. Scena docelowa ma dokładnie nowo wpisany tytuł i tekst,
    2. **Żadna inna scena (`other.ID != targetID`) nie zmieniła swojego tytułu, tekstu ani wyborów** (natychmiast wykrywa błąd nadpisania obu scen!).
- **Osiągalność Kliknięciem na Płótnie**:
  - Dla każdego węzła `n` w `studioGraphState.nodes` wyliczenie punktu ekranowego `(screenX, screenY)` i kliknięcie w `#studio-canvas` musi zaznaczyć ten konkretny węzeł (`selectedNodeIds.has(n.id)`) i otworzyć w `#modal-scene-text` treść należącą do `n.id`, nigdy do innej sceny.

### 4.3. Wyrocznia 3: Spójność Formatu `.playtxt` i Archiwum `.playbook` (`VerifyPlaytxtAndPlaybookRoundTrip`)
Po seriach mutacji wykonywany jest pełny test integralności formatu:
1. **Walidacja surowych plików `.playtxt` na dysku**:
   - Każdy plik `.playtxt` w katalogu książki jest poprawnym UTF-8 bez bajtów `\x00`, bez surowych tagów HTML (`<script>`, `<style>`) i bez osieroconych separatorów.
   - Liczba bloków `scene:` we wszystkich plikach `.playtxt` zgadza się z `len(s.ActiveBook.Scenes)`.
   - Przy `narrated: false` żaden plik `.playtxt` nie zawiera nagłówka `voice:`.
   - Pojedyncze wyjście ze sceny jest zawsze znormalizowane do niejawnego `Dalej`.
2. **Bezstratny Round-Trip `.playtxt` (`LoadBookFromDir`)**:
   - Wczytanie książki na świeżo przez `reloaded, err := book.LoadBookFromDir(activeRoot)` musi zakończyć się sukcesem (`err == nil`), a lista scen `(ID, ExplicitID, Name, Text, Chapter, ExplicitEnd, len(Choices))` w `reloaded` musi być identyczna jak w `s.ActiveBook`.
3. **Bezstratny Round-Trip `.playbook` (`PackBookToZip` → `LoadBookFromZip` → `UnpackBookZip`)**:
   - Spakowanie książki do tymczasowego archiwum `test.playbook` (`book.PackBookToZip`), walidacja `book.ValidateBookZip`, wczytanie `packed, err := book.LoadBookFromZip` oraz rozpakowanie do czystego katalogu (`book.UnpackBookZip` + `book.LoadBookFromDir`) musi dać identyczny graf scen, rozdziałów, zmiennych i `StartSceneID`.

### 4.4. Wyrocznia 4: Nienaruszalność `#topbar` i Kontraktu WFP
- `header.topbar#topbar` zawsze widoczny (`grid-row: 1; z-index: 200`), żaden `.modal-backdrop` nie ma `position: fixed`.
- Odpowiedzi `/api/action` zwracają wyłącznie dozwolone statusy HTTP (`200`, `204`, `400`, `403`, `405` — nigdy `500`) oraz poprawne nagłówki `X-Target`, `X-Scene-ID`, `X-Scene-Explicit-ID`, `X-Scene-Title`.

---

## 5. Uruchamianie i Generowanie Raportu

```sh
# Szybki przebieg (pisanie znak po znaku + mieszanie węzłów + 100% actionSpecs + 300 kroków chaosu + raport):
./build.sh --chaos

# Pomoc i lista opcji:
./build.sh --chaos --help

# Głęboki przebieg (2000 kroków + detektor wyścigów pamięci Go -race):
./build.sh --chaos --steps 2000 --race

# Odtworzenie konkretnego przebiegu z ziarna PRNG:
./build.sh --chaos --seed 1731051025 --steps 500
```

Raport zapisywany jest w czystym formacie tekstowym (Zero JSON) do `build/ui-chaos-trace.log` wraz z podsumowaniem wyroczni i gotową komendą `reproducer:`.
