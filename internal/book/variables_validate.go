package book

import (
	"fmt"
	"strconv"
	"strings"
)

// EncodeBookVariablesSpec encodes the book's variable definitions as "name:kind:opt1,opt2;..." for client-side live validation.
func EncodeBookVariablesSpec(b *Book) string {
	if b == nil || len(b.Variables) == 0 {
		return ""
	}
	ResolveBookVariableTypes(b)
	var parts []string
	for _, v := range b.Variables {
		name := SanitizeVarName(v.Name)
		if name == "" {
			continue
		}
		kind := v.Kind
		if kind == "" {
			kind = VarKindBool
		}
		opts := strings.Join(v.Options, ",")
		parts = append(parts, name+":"+kind+":"+opts)
	}
	return strings.Join(parts, ";")
}

// ValidateConditionExpr checks whether a choice or block condition expression only references declared variables with valid operators and values.
func ValidateConditionExpr(expr string, b *Book) bool {
	return ExplainConditionExprError(expr, b) == ""
}

// ExplainConditionExprError returns a human-readable Polish syntax/type error explanation for an invalid condition expression, or "" when valid.
func ExplainConditionExprError(expr string, b *Book) string {
	clean := CleanConditionExpr(expr)
	clean = strings.TrimSpace(strings.TrimPrefix(clean, "?"))
	if clean == "" {
		return ""
	}
	if b == nil {
		return "Brak zadeklarowanych zmiennych w książce"
	}
	depth := 0
	for i := 0; i < len(clean); i++ {
		switch clean[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return "Błąd składni: niedomknięty nawias w warunku"
			}
		}
	}
	if depth != 0 {
		return "Błąd składni: niedomknięty nawias w warunku"
	}
	return explainCondOr(clean, b)
}

func explainCondOr(s string, b *Book) string {
	s = strings.TrimSpace(trimOuterParens(s))
	if s == "" {
		return "Błąd składni: puste wyrażenie warunkowe"
	}
	if parts := splitTopLevelLogical(s, "||"); len(parts) > 1 {
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				return "Błąd składni: brak wyrażenia po operatorze '||'"
			}
			if errMsg := explainCondAnd(p, b); errMsg != "" {
				return errMsg
			}
		}
		return ""
	}
	return explainCondAnd(s, b)
}

func explainCondAnd(s string, b *Book) string {
	s = strings.TrimSpace(trimOuterParens(s))
	if s == "" {
		return "Błąd składni: puste wyrażenie warunkowe"
	}
	if parts := splitTopLevelLogical(s, "&&"); len(parts) > 1 {
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				return "Błąd składni: brak wyrażenia po operatorze '&&'"
			}
			if errMsg := explainCondAtom(p, b); errMsg != "" {
				return errMsg
			}
		}
		return ""
	}
	return explainCondAtom(s, b)
}

func explainCondAtom(s string, b *Book) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Błąd składni: puste wyrażenie warunkowe"
	}
	if unwrapped := trimOuterParens(s); unwrapped != s {
		return explainCondOr(unwrapped, b)
	}
	if strings.HasPrefix(s, "?") {
		return explainCondAtom(s[1:], b)
	}
	if strings.HasPrefix(s, "!") && !strings.HasPrefix(s, "!=") {
		return explainCondAtom(s[1:], b)
	}
	for _, mutOp := range []string{"++", "--", "+=", "-="} {
		if strings.Contains(s, mutOp) {
			return fmt.Sprintf("Błąd w if: operator '%s' służy do zmiany wartości w polu set:, a w if: podaj porównanie", mutOp)
		}
	}
	if strings.ContainsAny(s, ",;") {
		return "Błąd składni w if: użyj '&&' (oraz) lub '||' (lub) zamiast przecinka/średnika"
	}
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<", "="} {
		if idx := strings.Index(s, op); idx >= 0 {
			lhs := strings.TrimSpace(s[:idx])
			rhs := strings.Trim(strings.TrimSpace(s[idx+len(op):]), `"'`)
			if lhs == "" {
				return fmt.Sprintf("Błąd składni: brak nazwy zmiennej przed operatorem '%s'", op)
			}
			if rhs == "" {
				return fmt.Sprintf("Błąd składni: brak wartości po operatorze '%s' dla zmiennej '%s'", op, lhs)
			}
			if strings.ContainsAny(lhs, " \t=!<>+-()") {
				return fmt.Sprintf("Błąd składni: nieprawidłowa nazwa zmiennej '%s'", lhs)
			}
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", lhs)
			}
			return explainVarValueMatchError(vDef, op, rhs, b)
		}
	}
	if strings.ContainsAny(s, " \t=!<>+-(),:|&") {
		return fmt.Sprintf("Błąd składni w warunku: '%s'", s)
	}
	if b.FindVariable(s) == nil {
		return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", s)
	}
	return ""
}

func isBoolLiteralString(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "false", "tak", "nie", "yes", "no", "on", "off":
		return true
	default:
		return false
	}
}

func validateVarValueMatch(vDef *VariableDef, op string, rhs string, b *Book) bool {
	return explainVarValueMatchError(vDef, op, rhs, b) == ""
}

func explainVarValueMatchError(vDef *VariableDef, op string, rhs string, b *Book) string {
	switch vDef.Kind {
	case VarKindInt:
		if _, err := strconv.Atoi(rhs); err == nil {
			return ""
		}
		if rhsVar := b.FindVariable(rhs); rhsVar != nil && rhsVar.Kind == VarKindInt {
			return ""
		}
		return fmt.Sprintf("Błąd typu: zmienna '%s' (int) wymaga liczby całkowitej, otrzymano '%s'", vDef.Name, rhs)
	case VarKindEnum:
		if op != "==" && op != "!=" && op != "=" {
			return fmt.Sprintf("Błąd operatora: zmienna '%s' (enum) obsługuje tylko '=' / '==' lub '!='", vDef.Name)
		}
		opt := SanitizeEnumOption(rhs)
		for _, allowed := range vDef.Options {
			if strings.EqualFold(allowed, opt) {
				return ""
			}
		}
		if rhsVar := b.FindVariable(rhs); rhsVar != nil && rhsVar.Kind == VarKindEnum {
			return ""
		}
		return fmt.Sprintf("Nieznana wartość '%s' dla zmiennej '%s' (dozwolone: %s)", rhs, vDef.Name, strings.Join(vDef.Options, ", "))
	default:
		if op != "==" && op != "!=" && op != "=" {
			return fmt.Sprintf("Błąd operatora: zmienna '%s' (bool) obsługuje tylko '=' / '==' lub '!='", vDef.Name)
		}
		if isBoolLiteralString(rhs) {
			return ""
		}
		if strings.HasPrefix(rhs, "!") {
			if rhsVar := b.FindVariable(strings.TrimSpace(rhs[1:])); rhsVar != nil && rhsVar.Kind == VarKindBool {
				return ""
			}
		}
		if rhsVar := b.FindVariable(rhs); rhsVar != nil && rhsVar.Kind == VarKindBool {
			return ""
		}
		return fmt.Sprintf("Błąd typu: zmienna '%s' (bool) wymaga wartości true/false lub samej nazwy zmiennej", vDef.Name)
	}
}

// ValidateEffectsExpr checks whether a scene or choice effects expression only mutates declared variables with valid C/C#-style syntax.
func ValidateEffectsExpr(expr string, b *Book) bool {
	return ExplainEffectsExprError(expr, b) == ""
}

// ExplainEffectsExprError returns a human-readable Polish syntax/type error explanation for an invalid effects expression, or "" when valid.
func ExplainEffectsExprError(expr string, b *Book) string {
	clean := StripOuterDoubleBraces(expr)
	lower := strings.ToLower(clean)
	if strings.HasPrefix(lower, "set:") {
		clean = strings.TrimSpace(clean[4:])
	} else if strings.HasPrefix(lower, "then:") {
		clean = strings.TrimSpace(clean[5:])
	}
	clean = strings.TrimSuffix(strings.TrimSpace(clean), ";")
	if clean == "" {
		return ""
	}
	if b == nil {
		return "Brak zadeklarowanych zmiennych w książce"
	}
	clauses := splitEffectClauses(clean)
	if len(clauses) == 0 {
		return "Błąd składni w polu set:"
	}
	for _, rawClause := range clauses {
		c := strings.TrimSpace(rawClause)
		if c == "" {
			return "Błąd składni w set: puste wyrażenie po przecinku"
		}
		if (strings.HasSuffix(c, "++") || strings.HasSuffix(c, "--")) && !strings.Contains(c, "=") {
			op := c[len(c)-2:]
			varName := strings.TrimSpace(c[:len(c)-2])
			vDef := b.FindVariable(varName)
			if vDef == nil {
				return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", varName)
			}
			if vDef.Kind != VarKindInt {
				return fmt.Sprintf("Błąd typu: operator '%s' wymaga zmiennej typu int (zmienna '%s' jest typu %s)", op, vDef.Name, vDef.Kind)
			}
			continue
		}
		if strings.HasPrefix(c, "!") && !strings.Contains(c, "=") {
			varName := strings.TrimSpace(c[1:])
			vDef := b.FindVariable(varName)
			if vDef == nil {
				return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", varName)
			}
			if vDef.Kind != VarKindBool {
				return fmt.Sprintf("Błąd typu: negacja '!%s' wymaga zmiennej typu bool (jest typu %s)", vDef.Name, vDef.Kind)
			}
			continue
		}
		if (strings.HasPrefix(c, "+") || strings.HasPrefix(c, "-")) && !strings.Contains(c, "=") {
			rest := strings.Fields(strings.TrimSpace(c[1:]))
			if len(rest) == 0 || len(rest) > 2 {
				return fmt.Sprintf("Błąd składni w set: '%s'", c)
			}
			vDef := b.FindVariable(rest[0])
			if vDef == nil {
				return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", rest[0])
			}
			if vDef.Kind != VarKindBool && vDef.Kind != VarKindInt {
				return fmt.Sprintf("Błąd typu: dla zmiennej '%s' (enum) użyj przypisania '='", vDef.Name)
			}
			if len(rest) == 2 {
				if vDef.Kind != VarKindInt {
					return fmt.Sprintf("Błąd typu: zmienna '%s' (%s) nie przyjmuje wartości liczbowej", vDef.Name, vDef.Kind)
				}
				if _, err := strconv.Atoi(rest[1]); err != nil {
					return fmt.Sprintf("Błąd typu: zmienna '%s' (int) wymaga liczby całkowitej, otrzymano '%s'", vDef.Name, rest[1])
				}
			}
			continue
		}
		if strings.Contains(c, "==") || strings.Contains(c, "!=") || strings.Contains(c, ">=") || strings.Contains(c, "<=") || strings.Contains(c, ">") || strings.Contains(c, "<") || strings.Contains(c, "&&") || strings.Contains(c, "||") {
			return "Błąd w set: użyj przypisania ('=', '+=', '-=', '++', '--'), a nie operatora warunkowego"
		}
		if idx := strings.Index(c, "+="); idx > 0 {
			lhs := strings.TrimSpace(c[:idx])
			rhs := strings.TrimSpace(c[idx+2:])
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", lhs)
			}
			if vDef.Kind != VarKindInt {
				return fmt.Sprintf("Błąd typu: operator '+=' wymaga zmiennej typu int (zmienna '%s' jest typu %s)", vDef.Name, vDef.Kind)
			}
			if rhs == "" {
				return fmt.Sprintf("Błąd składni: brak wartości po '+=' dla zmiennej '%s'", vDef.Name)
			}
			if _, err := strconv.Atoi(rhs); err != nil {
				if rhsVar := b.FindVariable(rhs); rhsVar == nil || rhsVar.Kind != VarKindInt {
					return fmt.Sprintf("Błąd typu: zmienna '%s' (int) wymaga liczby całkowitej po '+=', otrzymano '%s'", vDef.Name, rhs)
				}
			}
			continue
		}
		if idx := strings.Index(c, "-="); idx > 0 {
			lhs := strings.TrimSpace(c[:idx])
			rhs := strings.TrimSpace(c[idx+2:])
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", lhs)
			}
			if vDef.Kind != VarKindInt {
				return fmt.Sprintf("Błąd typu: operator '-=' wymaga zmiennej typu int (zmienna '%s' jest typu %s)", vDef.Name, vDef.Kind)
			}
			if rhs == "" {
				return fmt.Sprintf("Błąd składni: brak wartości po '-=' dla zmiennej '%s'", vDef.Name)
			}
			if _, err := strconv.Atoi(rhs); err != nil {
				if rhsVar := b.FindVariable(rhs); rhsVar == nil || rhsVar.Kind != VarKindInt {
					return fmt.Sprintf("Błąd typu: zmienna '%s' (int) wymaga liczby całkowitej po '-=', otrzymano '%s'", vDef.Name, rhs)
				}
			}
			continue
		}
		if idx := strings.Index(c, "="); idx > 0 {
			lhs := strings.TrimSpace(c[:idx])
			rhs := strings.Trim(strings.TrimSpace(c[idx+1:]), `"'`)
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", lhs)
			}
			if rhs == "" {
				return fmt.Sprintf("Błąd składni: brak przypisywanej wartości po '=' dla zmiennej '%s'", vDef.Name)
			}
			if errMsg := explainVarValueMatchError(vDef, "=", rhs, b); errMsg != "" {
				return errMsg
			}
			continue
		}
		if !strings.ContainsAny(c, " \t+-*/=<>!&|,;:") {
			vDef := b.FindVariable(c)
			if vDef == nil {
				return fmt.Sprintf("Nieznana zmienna '%s' — zadeklaruj ją w panelu Zmienne {}", c)
			}
			if vDef.Kind == VarKindBool {
				continue
			}
			return fmt.Sprintf("Błąd składni: dla zmiennej '%s' (%s) podaj przypisanie (np. %s = wartość)", vDef.Name, vDef.Kind, vDef.Name)
		}
		return fmt.Sprintf("Błąd składni w set: '%s'", c)
	}
	return ""
}
