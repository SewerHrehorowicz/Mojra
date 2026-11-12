# Plan: `cmd/deck/finance.go`

## Co tam jest i jakie są problemy (524 magic strings, #1 w `audit_strings.log`)
- `LoadDeckFinanceContext` (L50–488): używa `map[string]float64` zamiast structa, zawiera 327 literałów kluczy (`cfg`, `vars`), ścieżek i formatów wewnątrz ciała funkcji.
- `build36MonthSimulation` (L510–679): tablica 36 opisów `milestones` (L515–554), format `"M%02d"` i klucze map zadeklarowane wewnątrz funkcji.
- `writeCalculatedFinanceCSV` (L701–748) i `write36MonthSimulationCSV` (L750–853): 150+ literałów prozy tabelarycznej, nagłówków i nazw plików CSV bezpośrednio w wywołaniach metod.

## Rozwiązania (plik po pliku, metoda po metodzie)
- **`cmd/deck/finance_schema.go` (nowy — dane, stałe, struktury)**:
  - Zastąpić `cfg map[string]float64` typowanym structem `financeConfig` oraz pakietową tablicą deskryptorów pól (`key`, wskaźnik/offset, wartość domyślna).
  - Wyciągnąć na poziom pakietu stałe ścieżek (`finanse.cfg`, `kalkulator-finansowy.csv`, `symulacja-36-miesiecy.csv`), formatów, kluczy kontekstu `.tpl` oraz tablicę `deckMilestones = [36]string{...}`.
- **`cmd/deck/finance.go` (obliczenia i kontekst)**:
  - `LoadDeckFinanceContext` (L50): parsować `finanse.cfg` do `financeConfig`, wyliczać `deckYearCalc` bez `fmt.Sprintf("s_r%d_...", yr)` i budować `vars` ze stałych pakietowych.
  - `build36MonthSimulation` (L510): korzystać z pakietowego `deckMilestones` i stałych kluczy wiersza.
- **`cmd/deck/finance_csv.go` (nowy — wydzielenie eksportu CSV)**:
  - Przenieść `csvCell`, `writeCSVRow`, `writeCalculatedFinanceCSV` (L681–748) i `write36MonthSimulationCSV` (L750–853); zdefiniować nagłówki i szablony wierszy w strukturach pakietowych poza ciałem funkcji.