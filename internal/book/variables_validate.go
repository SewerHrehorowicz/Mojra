//go:build !(js && wasm)

package book

import (
	"fmt"
	"strconv"
	"strings"

	"mojra/internal/tpl"
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

type valExtraKey uint8

const (
	valErrEmptyCond valExtraKey = iota
	valErrMissingAfterOr
	valErrMissingAfterAnd
	valErrInvalidVarName
	valErrNoNumericVal
	valExtraKeyCount
)

var valExtraDicts = [tpl.LangCount][valExtraKeyCount]string{
	tpl.LangPL: {
		valErrEmptyCond:       "Błąd składni: puste wyrażenie warunkowe",
		valErrMissingAfterOr:  "Błąd składni: brak wyrażenia po operatorze '||'",
		valErrMissingAfterAnd: "Błąd składni: brak wyrażenia po operatorze '&&'",
		valErrInvalidVarName:  "Błąd składni: nieprawidłowa nazwa zmiennej '%s'",
		valErrNoNumericVal:    "Błąd typu: zmienna '%s' (%s) nie przyjmuje wartości liczbowej",
	},
	tpl.LangEN: {
		valErrEmptyCond:       "Syntax error: empty condition expression",
		valErrMissingAfterOr:  "Syntax error: missing expression after '||'",
		valErrMissingAfterAnd: "Syntax error: missing expression after '&&'",
		valErrInvalidVarName:  "Syntax error: invalid variable name '%s'",
		valErrNoNumericVal:    "Type error: variable '%s' (%s) does not accept a numeric value",
	},
}

func valExtraMsg(l tpl.Lang, k valExtraKey) string {
	if l < tpl.LangCount && valExtraDicts[l][k] != "" {
		return valExtraDicts[l][k]
	}
	if valExtraDicts[tpl.LangEN][k] != "" {
		return valExtraDicts[tpl.LangEN][k]
	}
	return valExtraDicts[tpl.LangPL][k]
}

// ExplainConditionExprErrorLang returns a localized syntax/type error explanation for an invalid condition expression, or "" when valid.
func ExplainConditionExprErrorLang(expr string, b *Book, lang string) string {
	l := tpl.ParseLang(lang)
	clean := CleanConditionExpr(expr)
	clean = strings.TrimSpace(strings.TrimPrefix(clean, "?"))
	if clean == "" {
		return ""
	}
	if b == nil {
		return tpl.LookupJSI18n(l, "err_no_vars")
	}
	depth := 0
	for i := 0; i < len(clean); i++ {
		switch clean[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return tpl.LookupJSI18n(l, "err_unclosed_paren")
			}
		}
	}
	if depth != 0 {
		return tpl.LookupJSI18n(l, "err_unclosed_paren")
	}
	return explainCondOr(clean, b, l)
}

func explainCondOr(s string, b *Book, l tpl.Lang) string {
	s = strings.TrimSpace(trimOuterParens(s))
	if s == "" {
		return valExtraMsg(l, valErrEmptyCond)
	}
	if parts := splitTopLevelLogical(s, "||"); len(parts) > 1 {
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				return valExtraMsg(l, valErrMissingAfterOr)
			}
			if errMsg := explainCondAnd(p, b, l); errMsg != "" {
				return errMsg
			}
		}
		return ""
	}
	return explainCondAnd(s, b, l)
}

func explainCondAnd(s string, b *Book, l tpl.Lang) string {
	s = strings.TrimSpace(trimOuterParens(s))
	if s == "" {
		return valExtraMsg(l, valErrEmptyCond)
	}
	if parts := splitTopLevelLogical(s, "&&"); len(parts) > 1 {
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				return valExtraMsg(l, valErrMissingAfterAnd)
			}
			if errMsg := explainCondAtom(p, b, l); errMsg != "" {
				return errMsg
			}
		}
		return ""
	}
	return explainCondAtom(s, b, l)
}

func explainCondAtom(s string, b *Book, l tpl.Lang) string {
	for step := 0; step < 32; step++ {
		s = strings.TrimSpace(s)
		if s == "" {
			return valExtraMsg(l, valErrEmptyCond)
		}
		if unwrapped := trimOuterParens(s); unwrapped != s {
			return explainCondOr(unwrapped, b, l)
		}
		if strings.HasPrefix(s, "?") {
			s = s[1:]
			continue
		}
		if strings.HasPrefix(s, "!") && !strings.HasPrefix(s, "!=") {
			s = s[1:]
			continue
		}
		break
	}
	if s == "" {
		return valExtraMsg(l, valErrEmptyCond)
	}
	for _, mutOp := range []string{"++", "--", "+=", "-="} {
		if strings.Contains(s, mutOp) {
			return fmt.Sprintf(tpl.LookupJSI18n(l, "err_mut_op_in_if"), mutOp)
		}
	}
	if strings.ContainsAny(s, ",;") {
		return tpl.LookupJSI18n(l, "err_comma_in_if")
	}
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<", "="} {
		if idx := strings.Index(s, op); idx >= 0 {
			lhs := strings.TrimSpace(s[:idx])
			rhs := strings.Trim(strings.TrimSpace(s[idx+len(op):]), `"'`)
			if lhs == "" {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_missing_var_before_op"), op)
			}
			if rhs == "" {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_missing_val_after_op"), op, lhs)
			}
			if strings.ContainsAny(lhs, " \t=!<>+-()") {
				return fmt.Sprintf(valExtraMsg(l, valErrInvalidVarName), lhs)
			}
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_unknown_var"), lhs)
			}
			return explainVarValueMatchError(vDef, op, rhs, b, l)
		}
	}
	if strings.ContainsAny(s, " \t=!<>+-(),:|&") {
		return fmt.Sprintf(tpl.LookupJSI18n(l, "err_cond_syntax"), s)
	}
	if b.FindVariable(s) == nil {
		return fmt.Sprintf(tpl.LookupJSI18n(l, "err_unknown_var"), s)
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

func explainVarValueMatchError(vDef *VariableDef, op string, rhs string, b *Book, l tpl.Lang) string {
	switch vDef.Kind {
	case VarKindInt:
		if _, err := strconv.Atoi(rhs); err == nil {
			return ""
		}
		if rhsVar := b.FindVariable(rhs); rhsVar != nil && rhsVar.Kind == VarKindInt {
			return ""
		}
		return fmt.Sprintf(tpl.LookupJSI18n(l, "err_int_type"), vDef.Name, rhs)
	case VarKindEnum:
		if op != "==" && op != "!=" && op != "=" {
			return fmt.Sprintf(tpl.LookupJSI18n(l, "err_enum_op"), vDef.Name)
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
		return fmt.Sprintf(tpl.LookupJSI18n(l, "err_enum_val"), rhs, vDef.Name, strings.Join(vDef.Options, ", "))
	default:
		if op != "==" && op != "!=" && op != "=" {
			return fmt.Sprintf(tpl.LookupJSI18n(l, "err_bool_op"), vDef.Name)
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
		return fmt.Sprintf(tpl.LookupJSI18n(l, "err_bool_type"), vDef.Name)
	}
}

// ExplainEffectsExprErrorLang returns a localized syntax/type error explanation for an invalid effects expression, or "" when valid.
func ExplainEffectsExprErrorLang(expr string, b *Book, lang string) string {
	l := tpl.ParseLang(lang)
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
		return tpl.LookupJSI18n(l, "err_no_vars")
	}
	clauses := splitEffectClauses(clean)
	if len(clauses) == 0 {
		return tpl.LookupJSI18n(l, "err_set_syntax_empty")
	}
	for _, rawClause := range clauses {
		c := strings.TrimSpace(rawClause)
		if c == "" {
			return tpl.LookupJSI18n(l, "err_set_empty_after_comma")
		}
		if (strings.HasSuffix(c, "++") || strings.HasSuffix(c, "--")) && !strings.Contains(c, "=") {
			op := c[len(c)-2:]
			varName := strings.TrimSpace(c[:len(c)-2])
			vDef := b.FindVariable(varName)
			if vDef == nil {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_unknown_var"), varName)
			}
			if vDef.Kind != VarKindInt {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_op_requires_int"), op, vDef.Name, vDef.Kind)
			}
			continue
		}
		if strings.HasPrefix(c, "!") && !strings.Contains(c, "=") {
			varName := strings.TrimSpace(c[1:])
			vDef := b.FindVariable(varName)
			if vDef == nil {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_unknown_var"), varName)
			}
			if vDef.Kind != VarKindBool {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_neg_requires_bool"), vDef.Name, vDef.Kind)
			}
			continue
		}
		if (strings.HasPrefix(c, "+") || strings.HasPrefix(c, "-")) && !strings.Contains(c, "=") {
			rest := strings.Fields(strings.TrimSpace(c[1:]))
			if len(rest) == 0 || len(rest) > 2 {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_set_clause_syntax"), c)
			}
			vDef := b.FindVariable(rest[0])
			if vDef == nil {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_unknown_var"), rest[0])
			}
			if vDef.Kind != VarKindBool && vDef.Kind != VarKindInt {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_enum_needs_assign"), vDef.Name)
			}
			if len(rest) == 2 {
				if vDef.Kind != VarKindInt {
					return fmt.Sprintf(valExtraMsg(l, valErrNoNumericVal), vDef.Name, vDef.Kind)
				}
				if _, err := strconv.Atoi(rest[1]); err != nil {
					return fmt.Sprintf(tpl.LookupJSI18n(l, "err_int_type"), vDef.Name, rest[1])
				}
			}
			continue
		}
		if strings.Contains(c, "==") || strings.Contains(c, "!=") || strings.Contains(c, ">=") || strings.Contains(c, "<=") || strings.Contains(c, ">") || strings.Contains(c, "<") || strings.Contains(c, "&&") || strings.Contains(c, "||") {
			return tpl.LookupJSI18n(l, "err_cond_op_in_set")
		}
		if op, idx := findCompoundAssignOp(c); idx > 0 {
			lhs := strings.TrimSpace(c[:idx])
			rhs := strings.TrimSpace(c[idx+len(op):])
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_unknown_var"), lhs)
			}
			if vDef.Kind != VarKindInt {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_op_requires_int"), op, vDef.Name, vDef.Kind)
			}
			if rhs == "" {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_missing_val_after_compound"), op, vDef.Name)
			}
			if _, err := strconv.Atoi(rhs); err != nil {
				if rhsVar := b.FindVariable(rhs); rhsVar == nil || rhsVar.Kind != VarKindInt {
					return fmt.Sprintf(tpl.LookupJSI18n(l, "err_int_after_compound"), vDef.Name, op, rhs)
				}
			}
			continue
		}
		if idx := strings.Index(c, "="); idx > 0 {
			lhs := strings.TrimSpace(c[:idx])
			rhs := strings.Trim(strings.TrimSpace(c[idx+1:]), `"'`)
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_unknown_var"), lhs)
			}
			if rhs == "" {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_missing_val_after_eq"), vDef.Name)
			}
			if errMsg := explainVarValueMatchError(vDef, "=", rhs, b, l); errMsg != "" {
				return errMsg
			}
			continue
		}
		if !strings.ContainsAny(c, " \t+-*/=<>!&|,;:") {
			vDef := b.FindVariable(c)
			if vDef == nil {
				return fmt.Sprintf(tpl.LookupJSI18n(l, "err_unknown_var"), c)
			}
			if vDef.Kind == VarKindBool {
				continue
			}
			return fmt.Sprintf(tpl.LookupJSI18n(l, "err_bare_var_needs_assign"), vDef.Name, vDef.Kind, vDef.Name)
		}
		return fmt.Sprintf(tpl.LookupJSI18n(l, "err_set_clause_syntax"), c)
	}
	return ""
}


// ExplainExitTargetErrorLang returns a localized syntax/target error explanation for an invalid exit: target on sc, or "" when valid.
func ExplainExitTargetErrorLang(sc *Scene, rawTarget string, b *Book, lang string) string {
	l := tpl.ParseLang(lang)
	trimmed := strings.TrimSpace(rawTarget)
	if trimmed == "" {
		return tpl.LookupJSI18n(l, "err_exit_empty")
	}
	if strings.Contains(trimmed, "?") {
		return tpl.LookupJSI18n(l, "err_exit_question")
	}
	if strings.EqualFold(trimmed, "END") || strings.EqualFold(trimmed, "KONIEC") || strings.EqualFold(trimmed, "NEXT") || strings.EqualFold(trimmed, "DALEJ") {
		return ""
	}
	if b == nil {
		return ""
	}
	curCh := ""
	if sc != nil {
		curCh = strings.TrimSpace(sc.Chapter)
	}
	if curCh == "" {
		curCh = b.DefaultChapterName()
	}
	curChMeta := b.FindChapter(curCh)

	if chRef, isChTarget := ParseChapterTarget(trimmed); isChTarget {
		targetCh := b.FindChapter(chRef)
		if targetCh == nil {
			return fmt.Sprintf(tpl.LookupJSI18n(l, "err_exit_unknown_chapter"), chRef)
		}
		if strings.EqualFold(targetCh.Name, curCh) || strings.EqualFold(targetCh.ReferenceKey(), curCh) || strings.EqualFold(targetCh.ID, curCh) ||
			(curChMeta != nil && (strings.EqualFold(targetCh.ID, curChMeta.ID) || strings.EqualFold(targetCh.ReferenceKey(), curChMeta.ReferenceKey()))) {
			return tpl.LookupJSI18n(l, "err_exit_self_chapter")
		}
		return ""
	}

	if sc != nil {
		if strings.EqualFold(trimmed, sc.ID) ||
			(sc.ExplicitID != "" && strings.EqualFold(trimmed, sc.ExplicitID)) ||
			(sc.Name != "" && strings.EqualFold(trimmed, sc.Name)) {
			return tpl.LookupJSI18n(l, "err_exit_self_scene")
		}
	}

	normKey := NormalizeLookupKey(trimmed)
	for _, cand := range b.Scenes {
		if sc != nil && strings.EqualFold(cand.ID, sc.ID) {
			continue
		}
		candCh := strings.TrimSpace(cand.Chapter)
		if candCh == "" {
			candCh = b.DefaultChapterName()
		}
		if !strings.EqualFold(candCh, curCh) {
			continue
		}
		if strings.EqualFold(cand.ID, trimmed) ||
			(cand.ExplicitID != "" && strings.EqualFold(cand.ExplicitID, trimmed)) ||
			strings.EqualFold(cand.ReferenceKey(), trimmed) ||
			(cand.Name != "" && (strings.EqualFold(cand.Name, trimmed) || NormalizeLookupKey(cand.Name) == normKey)) {
			return ""
		}
	}

	if targetCh := b.FindChapter(trimmed); targetCh != nil {
		if strings.EqualFold(targetCh.Name, curCh) || strings.EqualFold(targetCh.ReferenceKey(), curCh) || strings.EqualFold(targetCh.ID, curCh) ||
			(curChMeta != nil && (strings.EqualFold(targetCh.ID, curChMeta.ID) || strings.EqualFold(targetCh.ReferenceKey(), curChMeta.ReferenceKey()))) {
			return tpl.LookupJSI18n(l, "err_exit_self_chapter")
		}
		return ""
	}

	return fmt.Sprintf(tpl.LookupJSI18n(l, "err_exit_unknown_target"), trimmed)
}

