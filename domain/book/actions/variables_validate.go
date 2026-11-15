//go:build !(js && wasm)

package actions

import (
	"strconv"
	"strings"

	"mojra/domain/i18n"
	"mojra/engine"
)

const (
	logicOrOp            = "||"
	logicAndOp           = "&&"
	notEqualOp           = "!="
	equalOp              = "=="
	greaterEqualOp       = ">="
	lessEqualOp          = "<="
	incOp                = "++"
	decOp                = "--"
	addAssignOp          = "+="
	subAssignOp          = "-="
	condClauseSeparators = ",;"
	quoteTrimSet         = `"'`
	varInvalidLHSChars   = " \t=!<>+-()"
	condSyntaxChars      = " \t=!<>+-(),:|&"
	setClauseSyntaxChars = " \t+-*/=<>!&|,;:"
	enumJoinSep          = ", "
	setPrefixLower       = "set:"
	thenPrefixLower      = "then:"
	boolLitTrue          = "true"
	boolLitFalse         = "false"
	boolLitTak           = "tak"
	boolLitNie           = "nie"
	boolLitYes           = "yes"
	boolLitNo            = "no"
	boolLitOn            = "on"
	boolLitOff           = "off"
	targetKoniec         = "KONIEC"
	targetNext           = "NEXT"
	targetDalej          = "DALEJ"
)

var (
	mutOpsInCond = [...]string{incOp, decOp, addAssignOp, subAssignOp}
	condCompOps  = [...]string{equalOp, notEqualOp, greaterEqualOp, lessEqualOp, ">", "<", "="}
	setForbidOps = [...]string{equalOp, notEqualOp, greaterEqualOp, lessEqualOp, ">", "<", logicAndOp, logicOrOp}
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

// ExplainConditionExprErrorLang returns a localized syntax/type error explanation for an invalid condition expression, or "" when valid.
func ExplainConditionExprErrorLang(expr string, b *Book, lang string) string {
	l := engine.ParseLang(lang)
	clean := CleanConditionExpr(expr)
	clean = strings.TrimSpace(strings.TrimPrefix(clean, "?"))
	if clean == "" {
		return ""
	}
	if b == nil {
		return engine.T(l, i18n.TrErrNoVars)
	}
	depth := 0
	for i := 0; i < len(clean); i++ {
		switch clean[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return engine.T(l, i18n.TrErrUnclosedParen)
			}
		}
	}
	if depth != 0 {
		return engine.T(l, i18n.TrErrUnclosedParen)
	}
	return explainCondOr(clean, b, l)
}

func explainCondOr(s string, b *Book, l engine.Lang) string {
	s = strings.TrimSpace(trimOuterParens(s))
	if s == "" {
		return engine.T(l, i18n.TrErrEmptyCond)
	}
	if parts := splitTopLevelLogical(s, logicOrOp); len(parts) > 1 {
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				return engine.T(l, i18n.TrErrMissingAfterOr)
			}
			if errMsg := explainCondAnd(p, b, l); errMsg != "" {
				return errMsg
			}
		}
		return ""
	}
	return explainCondAnd(s, b, l)
}

func explainCondAnd(s string, b *Book, l engine.Lang) string {
	s = strings.TrimSpace(trimOuterParens(s))
	if s == "" {
		return engine.T(l, i18n.TrErrEmptyCond)
	}
	if parts := splitTopLevelLogical(s, logicAndOp); len(parts) > 1 {
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				return engine.T(l, i18n.TrErrMissingAfterAnd)
			}
			if errMsg := explainCondAtom(p, b, l); errMsg != "" {
				return errMsg
			}
		}
		return ""
	}
	return explainCondAtom(s, b, l)
}

func explainCondAtom(s string, b *Book, l engine.Lang) string {
	for step := 0; step < 32; step++ {
		s = strings.TrimSpace(s)
		if s == "" {
			return engine.T(l, i18n.TrErrEmptyCond)
		}
		if unwrapped := trimOuterParens(s); unwrapped != s {
			return explainCondOr(unwrapped, b, l)
		}
		if strings.HasPrefix(s, "?") {
			s = s[1:]
			continue
		}
		if strings.HasPrefix(s, "!") && !strings.HasPrefix(s, notEqualOp) {
			s = s[1:]
			continue
		}
		break
	}
	if s == "" {
		return engine.T(l, i18n.TrErrEmptyCond)
	}
	for _, mutOp := range mutOpsInCond {
		if strings.Contains(s, mutOp) {
			return engine.Tf(l, i18n.TrErrMutOpInIf, mutOp)
		}
	}
	if strings.ContainsAny(s, condClauseSeparators) {
		return engine.T(l, i18n.TrErrCommaInIf)
	}
	for _, op := range condCompOps {
		if idx := strings.Index(s, op); idx >= 0 {
			lhs := strings.TrimSpace(s[:idx])
			rhs := strings.Trim(strings.TrimSpace(s[idx+len(op):]), quoteTrimSet)
			if lhs == "" {
				return engine.Tf(l, i18n.TrErrMissingVarBeforeOp, op)
			}
			if rhs == "" {
				return engine.Tf(l, i18n.TrErrMissingValAfterOp, op, lhs)
			}
			if strings.ContainsAny(lhs, varInvalidLHSChars) {
				return engine.Tf(l, i18n.TrErrInvalidVarName, lhs)
			}
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return engine.Tf(l, i18n.TrErrUnknownVar, lhs)
			}
			return explainVarValueMatchError(vDef, op, rhs, b, l)
		}
	}
	if strings.ContainsAny(s, condSyntaxChars) {
		return engine.Tf(l, i18n.TrErrCondSyntax, s)
	}
	if b.FindVariable(s) == nil {
		return engine.Tf(l, i18n.TrErrUnknownVar, s)
	}
	return ""
}

func isBoolLiteralString(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case boolLitTrue, boolLitFalse, boolLitTak, boolLitNie, boolLitYes, boolLitNo, boolLitOn, boolLitOff:
		return true
	default:
		return false
	}
}

func explainVarValueMatchError(vDef *VariableDef, op string, rhs string, b *Book, l engine.Lang) string {
	switch vDef.Kind {
	case VarKindInt:
		if _, err := strconv.Atoi(rhs); err == nil {
			return ""
		}
		if rhsVar := b.FindVariable(rhs); rhsVar != nil && rhsVar.Kind == VarKindInt {
			return ""
		}
		return engine.Tf(l, i18n.TrErrIntType, vDef.Name, rhs)
	case VarKindEnum:
		if op != equalOp && op != notEqualOp && op != "=" {
			return engine.Tf(l, i18n.TrErrEnumOp, vDef.Name)
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
		return engine.Tf(l, i18n.TrErrEnumVal, rhs, vDef.Name, strings.Join(vDef.Options, enumJoinSep))
	default:
		if op != equalOp && op != notEqualOp && op != "=" {
			return engine.Tf(l, i18n.TrErrBoolOp, vDef.Name)
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
		return engine.Tf(l, i18n.TrErrBoolType, vDef.Name)
	}
}

// ExplainEffectsExprErrorLang returns a localized syntax/type error explanation for an invalid effects expression, or "" when valid.
func ExplainEffectsExprErrorLang(expr string, b *Book, lang string) string {
	l := engine.ParseLang(lang)
	clean := StripOuterDoubleBraces(expr)
	lower := strings.ToLower(clean)
	if strings.HasPrefix(lower, setPrefixLower) {
		clean = strings.TrimSpace(clean[4:])
	} else if strings.HasPrefix(lower, thenPrefixLower) {
		clean = strings.TrimSpace(clean[5:])
	}
	clean = strings.TrimSuffix(strings.TrimSpace(clean), ";")
	if clean == "" {
		return ""
	}
	if b == nil {
		return engine.T(l, i18n.TrErrNoVars)
	}
	clauses := splitEffectClauses(clean)
	if len(clauses) == 0 {
		return engine.T(l, i18n.TrErrSetSyntaxEmpty)
	}
	for _, rawClause := range clauses {
		c := strings.TrimSpace(rawClause)
		if c == "" {
			return engine.T(l, i18n.TrErrSetEmptyAfterComma)
		}
		if (strings.HasSuffix(c, incOp) || strings.HasSuffix(c, decOp)) && !strings.Contains(c, "=") {
			op := c[len(c)-2:]
			varName := strings.TrimSpace(c[:len(c)-2])
			vDef := b.FindVariable(varName)
			if vDef == nil {
				return engine.Tf(l, i18n.TrErrUnknownVar, varName)
			}
			if vDef.Kind != VarKindInt {
				return engine.Tf(l, i18n.TrErrOpRequiresInt, op, vDef.Name, vDef.Kind)
			}
			continue
		}
		if strings.HasPrefix(c, "!") && !strings.Contains(c, "=") {
			varName := strings.TrimSpace(c[1:])
			vDef := b.FindVariable(varName)
			if vDef == nil {
				return engine.Tf(l, i18n.TrErrUnknownVar, varName)
			}
			if vDef.Kind != VarKindBool {
				return engine.Tf(l, i18n.TrErrNegRequiresBool, vDef.Name, vDef.Kind)
			}
			continue
		}
		if (strings.HasPrefix(c, "+") || strings.HasPrefix(c, "-")) && !strings.Contains(c, "=") {
			rest := strings.Fields(strings.TrimSpace(c[1:]))
			if len(rest) == 0 || len(rest) > 2 {
				return engine.Tf(l, i18n.TrErrSetClauseSyntax, c)
			}
			vDef := b.FindVariable(rest[0])
			if vDef == nil {
				return engine.Tf(l, i18n.TrErrUnknownVar, rest[0])
			}
			if vDef.Kind != VarKindBool && vDef.Kind != VarKindInt {
				return engine.Tf(l, i18n.TrErrEnumNeedsAssign, vDef.Name)
			}
			if len(rest) == 2 {
				if vDef.Kind != VarKindInt {
					return engine.Tf(l, i18n.TrErrNoNumericVal, vDef.Name, vDef.Kind)
				}
				if _, err := strconv.Atoi(rest[1]); err != nil {
					return engine.Tf(l, i18n.TrErrIntType, vDef.Name, rest[1])
				}
			}
			continue
		}
		hasCondOp := false
		for _, cop := range setForbidOps {
			if strings.Contains(c, cop) {
				hasCondOp = true
				break
			}
		}
		if hasCondOp {
			return engine.T(l, i18n.TrErrCondOpInSet)
		}
		if op, idx := findCompoundAssignOp(c); idx > 0 {
			lhs := strings.TrimSpace(c[:idx])
			rhs := strings.TrimSpace(c[idx+len(op):])
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return engine.Tf(l, i18n.TrErrUnknownVar, lhs)
			}
			if vDef.Kind != VarKindInt {
				return engine.Tf(l, i18n.TrErrOpRequiresInt, op, vDef.Name, vDef.Kind)
			}
			if rhs == "" {
				return engine.Tf(l, i18n.TrErrMissingValAfterCompound, op, vDef.Name)
			}
			if _, err := strconv.Atoi(rhs); err != nil {
				if rhsVar := b.FindVariable(rhs); rhsVar == nil || rhsVar.Kind != VarKindInt {
					return engine.Tf(l, i18n.TrErrIntAfterCompound, vDef.Name, op, rhs)
				}
			}
			continue
		}
		if idx := strings.Index(c, "="); idx > 0 {
			lhs := strings.TrimSpace(c[:idx])
			rhs := strings.Trim(strings.TrimSpace(c[idx+1:]), quoteTrimSet)
			vDef := b.FindVariable(lhs)
			if vDef == nil {
				return engine.Tf(l, i18n.TrErrUnknownVar, lhs)
			}
			if rhs == "" {
				return engine.Tf(l, i18n.TrErrMissingValAfterEq, vDef.Name)
			}
			if errMsg := explainVarValueMatchError(vDef, "=", rhs, b, l); errMsg != "" {
				return errMsg
			}
			continue
		}
		if !strings.ContainsAny(c, setClauseSyntaxChars) {
			vDef := b.FindVariable(c)
			if vDef == nil {
				return engine.Tf(l, i18n.TrErrUnknownVar, c)
			}
			if vDef.Kind == VarKindBool {
				continue
			}
			return engine.Tf(l, i18n.TrErrBareVarNeedsAssign, vDef.Name, vDef.Kind, vDef.Name)
		}
		return engine.Tf(l, i18n.TrErrSetClauseSyntax, c)
	}
	return ""
}

// ExplainExitTargetErrorLang returns a localized syntax/target error explanation for an invalid exit: target on sc, or "" when valid.
func ExplainExitTargetErrorLang(sc *Scene, rawTarget string, b *Book, lang string) string {
	l := engine.ParseLang(lang)
	trimmed := strings.TrimSpace(rawTarget)
	if trimmed == "" {
		return engine.T(l, i18n.TrErrExitEmpty)
	}
	if strings.Contains(trimmed, "?") {
		return engine.T(l, i18n.TrErrExitQuestion)
	}
	if strings.EqualFold(trimmed, TargetEnd) || strings.EqualFold(trimmed, targetKoniec) || strings.EqualFold(trimmed, targetNext) || strings.EqualFold(trimmed, targetDalej) {
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
			return engine.Tf(l, i18n.TrErrExitUnknownChapter, chRef)
		}
		if strings.EqualFold(targetCh.Name, curCh) || strings.EqualFold(targetCh.ReferenceKey(), curCh) || strings.EqualFold(targetCh.ID, curCh) ||
			(curChMeta != nil && (strings.EqualFold(targetCh.ID, curChMeta.ID) || strings.EqualFold(targetCh.ReferenceKey(), curChMeta.ReferenceKey()))) {
			return engine.T(l, i18n.TrErrExitSelfChapter)
		}
		return ""
	}

	if sc != nil {
		if strings.EqualFold(trimmed, sc.ID) ||
			(sc.ExplicitID != "" && strings.EqualFold(trimmed, sc.ExplicitID)) ||
			(sc.Name != "" && strings.EqualFold(trimmed, sc.Name)) {
			return engine.T(l, i18n.TrErrExitSelfScene)
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
			return engine.T(l, i18n.TrErrExitSelfChapter)
		}
		return ""
	}

	return engine.Tf(l, i18n.TrErrExitUnknownTarget, trimmed)
}

