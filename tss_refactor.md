# Plan dokończenia podziału `internal/tpl/tss.go` (`tss_emit_split.md`)

## 1. Co tam jest i jakie są problemy (wg `logs/audit_arch.log`)

1. **`ARCH_MONOLITH_AND_NESTING` (`internal/tpl/tss.go:1`, 1085 LOC > 1000)**:
   - `internal/tpl/tss.go` nadal łączy dwie odrębne warstwy: parser drzewa AST (`L1–561`, ~550 LOC) oraz emiter CSS z transformacją wartości i selektorów (`L563–1085`, ~525 LOC).
2. **`ARCH_SIGNATURE_BLOAT` (3 funkcje z 6 parametrami)**:
   - `resolveSwitchCaseTargetSelector` (`L584`, 6 parametrów: `swDef, file, swName, line, caseKeys, currentSelector`),
   - `cssParser.parseArrowControlFlow` (`L351`, 6 parametrów: `parent, content, lhs, rhs, indent, lineNum`),
   - `cssParser.addPropSwitch` (`L390`, 6 parametrów: `parent, propKey, swName, inlineExpr, indent, lineNum`).
3. **`ARCH_RAVIOLI_INDIRECTION` (`internal/tpl/tss.go:284`)**:
   - `cssParser.parsePropSwitchChild` (`L284–295`, 12 LOC, `fanIn=1`) — wywoływana w jednym miejscu w `transpileCSSWithSwitches` (`L445`).

---

## 2. Rozwiązania

1. **Wydzielenie emitera i transformacji CSS do `internal/tpl/tss_emit.go` (~525 LOC)**:
   - Przenieść typy `cssSwitchedBucket` i `cssEmitTask` (`L19–29`) oraz funkcje `evalCSSCondNode`, `resolveSwitchCaseTargetSelector`, `emitCSSNode`, `formatCSSBlock`, `formatCSSDecls`, `normalizeCSSPropKey`, `transformCSSValue`, `stepCSSQuoteChar`, `expandShortHexColors`, `interpolateCSSGoVars`, `consumeCSSVarRef`, `replaceCSSDollarVars`, `isCSSIdentStart`, `isCSSIdentChar`, `splitTopLevelDelimiter`, `splitTopLevelCommas`, `combineNestedSelectors`, `combineSwitchAndRuleSelector` (`L563–1085`) z `tss.go` do `internal/tpl/tss_emit.go`.
   - `internal/tpl/tss.go` spadnie do `~530 LOC` (wyłącznie wejście bundlera i parser AST `cssParser`).
2. **Redukcja parametrów (`ARCH_SIGNATURE_BLOAT` -> `< 6` parametrów)**:
   - W `cssParser.parseArrowControlFlow` (`L351`) i `cssParser.addPropSwitch` (`L390`) przekazywać `pl cssPreprocessedLine` zamiast osobnych `(content, indent, lineNum)` / `(indent, lineNum)` (spadek z 6 do 4 i 5 parametrów).
   - W `resolveSwitchCaseTargetSelector` (`L584`) przekazywać węzeł `ch *cssNode` zamiast osobnych `(file, swName string)` (spadek z 6 do 5 parametrów: `(swDef *CSSSwitchDef, ch *cssNode, line int, caseKeys []string, currentSelector string)`).
3. **Usunięcie `ARCH_RAVIOLI_INDIRECTION`**:
   - Zinlinować `cssParser.parsePropSwitchChild` (`L284–295`) bezpośrednio w `if parent.Kind == cssKindPropSwitch` w `transpileCSSWithSwitches` (`L444–449`).

---

## 3. Zmiany plik po pliku, metoda po metodzie, linia po linii

### Plik 1 (nowy): `internal/tpl/tss_emit.go`
- Przenieść z `internal/tpl/tss.go`:
  - `type cssSwitchedBucket` i `type cssEmitTask` (`L19–29`),
  - `evalCSSCondNode` (`L563–582`),
  - `resolveSwitchCaseTargetSelector(swDef *CSSSwitchDef, ch *cssNode, line int, caseKeys []string, currentSelector string) (string, error)` (`L584–595`, 5 parametrów zamiast 6),
  - `emitCSSNode` (`L597–754`, zaktualizować 2 wywołania `resolveSwitchCaseTargetSelector` na `L671` i `L737`),
  - `formatCSSBlock`, `formatCSSDecls`, `normalizeCSSPropKey`, `transformCSSValue` (`L756–790`),
  - `stepCSSQuoteChar`, `expandShortHexColors`, `interpolateCSSGoVars`, `consumeCSSVarRef`, `replaceCSSDollarVars`, `isCSSIdentStart`, `isCSSIdentChar` (`L792–962`),
  - `splitTopLevelDelimiter`, `splitTopLevelCommas`, `combineNestedSelectors`, `combineSwitchAndRuleSelector` (`L964–1085`).

### Plik 2 (zrefaktoryzowany): `internal/tpl/tss.go`
- **Typy (`L19–29`)**: usunąć `cssSwitchedBucket` i `cssEmitTask` (przeniesione do `tss_emit.go`).
- **`cssParser.parsePropSwitchChild` (`L284–295`)**: usunąć metodę i zinlinować jej ciało w `transpileCSSWithSwitches` (`L444–449`).
- **`cssParser.parseArrowControlFlow` (`L351–388`)**: zmienić sygnaturę na `(p *cssParser) parseArrowControlFlow(parent *cssNode, pl cssPreprocessedLine, lhs, rhs string) error` (4 parametry).
- **`cssParser.addPropSwitch` (`L390–415`)**: zmienić sygnaturę na `(p *cssParser) addPropSwitch(parent *cssNode, pl cssPreprocessedLine, propKey, swName, inlineExpr string) error` (5 parametrów).
- **`transpileCSSWithSwitches` (`L417–561`)**: zaktualizować wywołania `p.parseArrowControlFlow` (`L479`) oraz `p.addPropSwitch` (`L505`).
- **Linie `L563–1085`**: usunąć z `tss.go` (przeniesione do `tss_emit.go`).

### Plik 3: `docs/project.graph`
- Zaktualizować graf projektu (`go run ./cmd/projgraph`).