# Plan Rozszerzenia Audytu Statycznego Mojra (Port z Workflow: Kontrakty, Architektura `ARCH_*` i Bezwzględny Audyt Magic Stringów `MAGIC_*`)

## 1. Cel i Wydajność (P0 Performance Gate)
- **Jednokrotne indeksowanie przestrzeni roboczej (`workspaceIndex`)**: Wszystkie przebiegi (`Run`, `RunArch`, `RunMagic`) współdzielą pojedynczy odczyt i parsowanie plików `.go` (`go/ast`), `.tpl`, `.tss`, `.js` oraz `.svg` z katalogów `internal/`, `cmd/`, `app/` i `www/` bez wielokrotnego czytania dysku w każdej regule.
- **Zero wpływu na ścieżkę wykonawczą aplikacji**: Analizator działa wyłącznie w `internal/audit/` podczas `go test ./internal/audit/...` oraz komendy CLI `mojra audit`, wykonując pełny przebieg w `< 50 ms` (weryfikowane przez `BenchmarkAuditRun`, `BenchmarkArchAuditRun`, `BenchmarkMagicAuditRun`).
- **Ujednolicony model znaleziska (`Finding`)**:
  - Rozszerzenie struktury `Finding` o pola `Severity` (`"ERROR"`, `"WARNING"`, `"INFO"`), `Symbol` (nazwa funkcji, struktury, selektora lub komponentu) oraz `Score` (waga punktowa długu architektonicznego / magic stringów).
- **Trzy dedykowane logi w `logs/`**:
  - `logs/audit_static.log` (`--static`): twarda bramka analizy statycznej i kontraktów (zero tolerancji dla błędów).
  - `logs/audit_arch.log` (`--arch`): doradczy audyt architektury i długu technicznego posortowany wg `Score`.
  - `logs/audit_strings.log` (`--magic` / `--strings`): doradczy audyt magic stringów w ciałach funkcji/metod posortowany wg `Score`.

---

## 2. Etap 1 — Rozszerzenie Audytu Podstawowego i Kontraktów (`internal/audit/audit.go`, `audit_js_security.go`)

### 2.1. `TPL_DUPLICATE` — Wykrywanie Klonów Poddrzew `.tpl` pod `@component`
- Normalizacja wcięć i kształtu drzewa w `app/templates/*.tpl` oraz `www/*.tpl`:
  - **Klony dokładne (`Exact .tpl subtree`)**: poddrzewa `>= 12` linii powtórzone `>= 2×` (`ERROR`).
  - **Klony strukturalne (`Structural .tpl subtree`)**: poddrzewa o identycznej strukturze tagów/klas po znormalizowaniu literałów `"$"` i wyrażeń `{$}` (`>= 20` linii `>= 2×` lub `>= 10` linii `>= 4×`), wskazujące miejsca do wydzielenia `@component` w `app/templates/components.tpl`.

### 2.2. Statyczna Analiza Arkuszy `.tss` (`app/styles/*.tss`, `www/style.tss`)
- **Martwe klasy CSS (`DEAD_CODE`)**: Wykrywanie selektorów `.klasa` zdefiniowanych w `.tss`, które nigdy nie występują w `.tpl`, `.js`, `.go` ani `.svg` (z uwzględnieniem klas warunkowych `.active?: { expr }` oraz dynamicznych prefiksów).
- **Duplikaty selektorów (`TPL_ARCH`)**: Wykrywanie powtórzonych identycznych selektorów najwyższego poziomu w tym samym pliku `.tss`.
- **Strażnik bezpieczeństwa i zasad UI (`SECURITY_XSS_INJECTION` / `TPL_ARCH`)**: Blokada `expression(`, `javascript:`, `<script` oraz niedozwolonych konstrukcji w `.tss`.

### 2.3. Dwukierunkowy Kontrakt Klient ↔ Szablon ↔ Serwer (`ACTION_CONTRACT`, `FIELD_CONTRACT`)
- **`ACTION_CONTRACT`**:
  - Każda akcja użyta w `.tpl` (`action:`, `data-backdrop-action:`, parametry akcji przekazywane do `@component`) oraz w `.js` (`sendServerAction`, `postServerActionSilent`) musi istnieć w `actionSpecs` (`internal/gui/actions_dispatch.go`).
  - Każda akcja w `actionSpecs` musi mieć wywołanie po stronie klienta lub w przepływie potwierdzenia (`ConfirmPendingAction`).
- **`FIELD_CONTRACT`**:
  - Każde pole formularza (`input`, `textarea`, `.custom-select` z atrybutem `name:`) w `app/templates/*.tpl` musi być odczytywane w handlerach Go (`r.FormValue`, dekoder WFP) lub obsługiwane w JS.

### 2.4. Ścisła Separacja Warstw i Higiena Struktur Go (`TPL_ARCH`, `SUSPICIOUS_STRING`, `DEAD_CODE`)
- **Zakaz inline `.tpl`, CSS i JS w Go (`TPL_ARCH`)**: Wykrywanie literałów stringowych w Go zawierających kod szablonów (`@component`, `{for ...}`), reguły/atrybuty CSS (`style=".."`), kod JS (`document.querySelector`, `addEventListener`, `<script>`) poza kompilatorem `internal/tpl/`.
- **`oversized-string-literal` (`SUSPICIOUS_STRING`)**: Flaga dla literałów w Go przekraczających `200` znaków lub zawierających `>= 3` znaków `\n` (wymóg przeniesienia do `.tpl` lub dedykowanych stałych).
- **Martwe tagi serializacji (`DEAD_CODE`)**: Wykrywanie tagów `json:"..."`, `yaml:"..."` lub `xml:"..."` w strukturach Go (zgodnie z zasadą Zero JSON / Zero YAML).

---

## 3. Etap 2 — Deterministyczny Audyt Architektury i Długu Technicznego (`internal/audit/audit_arch.go` — `RunArch`)

Każde znalezisko otrzymuje deterministyczny wynik `Score`, severity (`ERROR` dla `Score >= 80`, `WARN` dla `Score >= 45`, `INFO` dla `< 45`), a raport otwiera tabela **TOP 10 MOST SUSPICIOUS FILES (BY CUMULATIVE DEBT SCORE)**.

### 3.1. `ARCH_STRING_HACK` (Go & JS)
- Wykrywanie funkcji wykonujących gęste operacje na surowych stringach zamiast typowanych tokenów lub tablic:
  - **Go**: `>= 6` wywołań `strings.*` (`Contains`, `Split`, `Trim*`, `Replace*`, `Cut*`, `Index*`) lub kombinacja `strings.*` + `switch` po stringach + `>= 10` literałów (`Score = stringsCalls*8 + inlineLiterals*2 + stringSwitches*10`).
  - **JS**: `>= 6` wywołań metod stringowych (`.indexOf`, `.includes`, `.startsWith`, `.replace`, `.split`, `.slice`, `.trim`) lub `>= 12` literałów stringowych w jednej funkcji (`Score = strCalls*8 + strLits*3`).

### 3.2. `ARCH_RAVIOLI_INDIRECTION` (Go, JS & TPL)
- **Go**:
  - 1-linijkowe funkcje/metody pass-through, które wyłącznie delegują argumenty `1:1` do innej funkcji bez transformacji danych (`Score = 34..42`).
  - Nieeksportowane mikro-funkcje (`<= 12` LOC, `fanIn = 1`, brak referencji w `go`/`defer`/wartościach funkcyjnych) wywoływane tylko z jednego miejsca (`Score = 24 + (13-LOC)*2`).
  - Głębokie łańcuchy osadzania struktur/interfejsów (`embedding depth > 2`, `Score = depth * 18`).
- **JS**: Funkcje pass-through (`return fn(...)` w 1 instrukcji, `Score = 32`) oraz mikro-helpery (`<= 10` linii, `fanIn = 1`, `Score = 26`).
- **TPL**: Mikro-komponenty `@component` (`<= 6` linii) użyte `<= 1` raz w całym projekcie (`Score = 24`).

### 3.3. `ARCH_SINGLE_IMPL_IFACE` (Go)
- Analiza wszystkich deklaracji `type ... interface` względem wszystkich typów konkretnych w repozytorium:
  - Interfejsy mające `0` konkretnych implementacji (`Score = 45`).
  - Interfejsy mające dokładnie `1` konkretną implementację (`Score = 35`, przedwczesna abstrakcja vtable).

### 3.4. `ARCH_LCOM_STRUCT` i `ARCH_OBJECT_SOUP` (Go)
- **`ARCH_LCOM_STRUCT` (Lack of Cohesion in Methods — LCOM4)**:
  - Dla struktur posiadających `>= 4` pola i `>= 4` metody operujące na polach odbiornika (czyste DTO/ViewModele bez metod są pomijane) budowany jest graf współdzielenia pól między metodami.
  - Flaga gdy liczba spójnych składowych `LCOM4 >= 2` lub odsetek rozłącznych par metod `disjointPct >= 70%` (`Score = lcom4*20 + disjointPct/2`).
- **`ARCH_OBJECT_SOUP`**:
  - Wykrywanie plików zawierających `>= 3` mikro-struktury behawioralne (`<= 3` pola oraz `1..2` metody), które rozbijają spójną logikę na drobne obiekty (`Score = count * 15`).

### 3.5. `ARCH_SIGNATURE_BLOAT` (Go, JS & TPL)
- **Go**:
  - Funkcje z `>= 6` parametrami, `>= 4` wartościami zwracanymi lub `>= 2` flagami `bool` (`Score = params*6 + results*8 + bools*15`).
  - Zbyt długie identyfikatory funkcji (`> 38` znaków, `Score = 20 + (len-38)*2`).
  - Proliferacja wariantów funkcji o wspólnym rdzeniu `Stem` + sufiksach `With` / `For` / `By` / `From` (`>= 3` warianty w jednym pakiecie, `Score = variants * 12`).
- **JS**: Funkcje przyjmujące `>= 5` parametrów lub o nazwie `> 38` znaków.
- **TPL**: Definicje `@component` wymagające `>= 7` propsów (`Score = params * 6`).

### 3.6. `ARCH_MONOLITH_AND_NESTING` (Go, JS, TPL & TSS)
- **Rozmiar plików**: Go `> 1000` LOC, JS `> 600` LOC, TSS `> 600` LOC, TPL `> 400` LOC.
- **Deklaracje `const` w środku pliku Go**: Wykrywanie bloków `const` zadeklarowanych poniżej zmiennych `var` lub funkcji `func` (z wyjątkiem bloków `iota` umieszczonych bezpośrednio pod swoją deklaracją `type`, `Severity = INFO`, `Score = 20`).
- **Głębokość zagnieżdżenia `.tpl`**: Wcięcia przekraczające `8` poziomów DOM (`Score = (depth-8)*12 + 20`).
- **Złożoność i duplikacja `.tss`**:
  - Selektory złożone z `>= 4` poziomów zagnieżdżenia (`Score = 20 + (levels-3)*6`).
  - Gęstość `!important` `>= 5` w jednym pliku `.tss` (z wyłączeniem globalnego resetu `base.tss`, `Score = count * 6`).
  - Zduplikowane bloki właściwości `.tss` (`>= 4` identyczne deklaracje `klucz: wartość` powtórzone w `>= 3` regułach, `Score = declCount * occs * 4`).

---

## 4. Etap 3 — Bezwzględny Audyt Magic Stringów w Metodach (`internal/audit/audit_magic.go` — `RunMagic`)

### 4.1. Zasada Zero Stringów w Ciele Funkcji / Metod
- Skaner odwiedza wyłącznie ciała funkcji i metod:
  - W Go: każde `ast.FuncDecl.Body` oraz każde `ast.FuncLit.Body` (w tym domknięcia przypisane do tablic/map `var`, np. `actionSpecs`).
  - W JS: ciała wszystkich funkcji w `app/js/*.js` oraz `www/app.js`.
- **Dozwolone wyjątki**:
  1. Deklaracje `const` i tablice/słowniki `var` na poziomie pakietu/modułu (poza ciałem funkcji).
  2. Puste stringi `""` oraz jednoliterowe/interpunkcyjne separatory bez znaków alfanumerycznych (`":"`, `","`, `"/"`, `"\n"`, `"->"`).

### 4.2. Klasyfikacja i Punktacja (`classifyMagicString`)
1. **`MAGIC_BRANCH_STRING` (`ERROR`, `Score = 40 + min(len, 120)/2`)**:
   - Literały stringowe użyte w warunkach `case "..."`, porównaniach `==`, `!=`, `<`, `<=`, `>`, `>=` oraz jako argumenty `strings.*`, `regexp.*`, `filepath.*`, `path.*`.
2. **`MAGIC_PROSE_STRING` (`ERROR`, `Score = 35 + min(len, 120)/2`)**:
   - Literały przekazywane do `fmt.*`, `errors.*`, `log.*`, literały o długości `>= 24` znaków lub zawierające `>= 3` słowa.
3. **`MAGIC_FUNC_STRING` (`WARN` dla `< 8` znaków, `ERROR` dla `>= 8` znaków, `Score = 15 + min(len, 120)/3`)**:
   - Wszystkie pozostałe niepuste literały alfanumeryczne wewnątrz funkcji Go i JS.

### 4.3. Rankingi w Raporcie (`PrintMagicReportWithLang`)
- Podsumowanie liczby stringów i punktów w kategoriach `MAGIC_BRANCH_STRING`, `MAGIC_PROSE_STRING`, `MAGIC_FUNC_STRING`.
- **TOP 15 FILES BY IN-METHOD MAGIC STRINGS** (Rank, Score, Strings, File).
- **TOP 15 FUNCTIONS/METHODS BY MAGIC STRINGS** (Rank, Score, Strings, `File :: Function`).
- Lista wszystkich wystąpień posortowana malejąco według `Score`.

---

## 5. Etap 4 — Testy Syntetyczne, Benchmarki oraz Integracja z CLI i `build.sh`

### 5.1. Samotestowanie Reguł na Syntetycznym Repozytorium (`internal/audit/audit_test.go`)
1. **`TestSyntheticViolationsDetected`**: Tworzy w `t.TempDir()` mini-repozytorium z celowymi błędami (`DEAD_CODE`, `ACTION_CONTRACT`, `JS_DOM_CONTRACT`, `FIELD_CONTRACT`, `UNUSED_TPL_VAR`, `DOM_ID_IN_GO`, `TPL_ARCH`, `SUSPICIOUS_STRING`, `HOT_PATH_ALLOC`, `TPL_DUPLICATE`, `.unused-css-selector`, `inline-css-in-tpl`, `inline-js-in-tpl`, `inline-tpl-in-go`, `oversized-string-literal`) i weryfikuje `100%` wykrywalności.
2. **`TestArchitectureAuditSynthetic`**: Tworzy syntetyczne przypadki dla wszystkich 7 kategorii `ARCH_*` oraz weryfikuje brak fałszywych alarmów dla czystych struktur DTO (`0` metod w `ARCH_LCOM_STRUCT`) i bloków `iota` bezpośrednio pod `type`.
3. **`TestMagicStringAuditSynthetic`**: Weryfikuje dokładną liczbę wykrytych `MAGIC_BRANCH_STRING`, `MAGIC_PROSE_STRING` i `MAGIC_FUNC_STRING`, brak flagowania pakietowych `const`/`var` oraz obecność obu tabel Top 15 w raporcie.
4. **Benchmarki**: `BenchmarkAuditRun`, `BenchmarkArchAuditRun`, `BenchmarkMagicAuditRun`.

### 5.2. Rozszerzenie CLI (`cmd/mojra/main.go`) i `build.sh`
- Nowe flagi w `mojra audit` oraz `./build.sh --audit`:
  - `--static`: uruchamia wyłącznie analizę statyczną (`audit.Run`), zapisuje `logs/audit_static.log` i blokuje przy wykrytych problemach w trybie `--strict` / `ST_AUDIT_STRICT=1`.
  - `--arch` (lub `ST_AUDIT_ARCH=1`): uruchamia `audit.RunArch` i zapisuje raport do `logs/audit_arch.log` (tryb doradczy).
  - `--magic` / `--strings` (lub `ST_AUDIT_MAGIC=1`): uruchamia `audit.RunMagic` i zapisuje raport do `logs/audit_strings.log` (tryb doradczy).
  - Brak flag trybu (`mojra audit`): uruchamia wszystkie 3 audyty i zapisuje wszystkie 3 pliki w `logs/`.
  - `--tss` / `--css`, `--js`, `--go`, `--tpl`, `--category <KATEGORIA>`, `--severity <error|warning|info>`, `--strict` (lub `ST_AUDIT_STRICT=1`), `--out <ścieżka>`.

```sh
# Wszystkie 3 raporty (logs/audit_static.log, logs/audit_arch.log, logs/audit_strings.log):
go run ./cmd/mojra audit

# Wyłącznie twarda analiza statyczna:
go run ./cmd/mojra audit --static --strict

# Audyt architektury i długu technicznego (z rankingiem Top 10 plików):
go run ./cmd/mojra audit --arch

# Bezwzględny audyt magic stringów w metodach (z rankingami Top 15 plików i funkcji):
go run ./cmd/mojra audit --magic

# Pełna weryfikacja jednostkowa i syntetyczna:
go test -v ./internal/audit/...
```
