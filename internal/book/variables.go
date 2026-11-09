package book

import (
	"strconv"
	"strings"
	"unicode"
)

const (
	VarKindBool = "bool"
	VarKindInt  = "int"
	VarKindEnum = "enum"
)

// SanitizeVarName normalizes a variable or enum identifier (lowercase ASCII/Unicode letters, digits, underscores).
func SanitizeVarName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "$")
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			sb.WriteRune(unicode.ToLower(r))
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(unicode.ToLower(r))
		} else if (r == ' ' || r == '-') && sb.Len() > 0 {
			last := sb.String()
			if !strings.HasSuffix(last, "_") {
				sb.WriteByte('_')
			}
		}
	}
	return strings.Trim(sb.String(), "_")
}

// SanitizeEnumOption normalizes a single enum state token.
func SanitizeEnumOption(raw string) string {
	s := strings.TrimSpace(SanitizePlainText(raw))
	s = strings.Trim(s, `"'`)
	return SanitizeVarName(s)
}

// SanitizeRuleExpr sanitizes a condition or effect expression while preserving comparison operators (<, >, <=, >=).
func SanitizeRuleExpr(raw string) string {
	s := strings.ReplaceAll(raw, "\x00", "")
	s = htmlTagStripRe.ReplaceAllString(s, "")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\r' || r == '\n' {
			b.WriteByte(' ')
		} else if r == '\t' || r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// StripOuterDoubleBraces removes optional outer {{ and }} around an expression.
func StripOuterDoubleBraces(raw string) string {
	s := SanitizeRuleExpr(raw)
	if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") && len(s) >= 4 {
		inner := strings.TrimSpace(s[2 : len(s)-2])
		if !strings.Contains(inner, "}}") {
			return inner
		}
	}
	return s
}

// CleanConditionExpr normalizes a condition expression by stripping optional {{...}} or leading "if:" prefix.
func CleanConditionExpr(raw string) string {
	s := StripOuterDoubleBraces(raw)
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "if:") {
		s = strings.TrimSpace(s[3:])
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), ";")
	return strings.TrimSpace(s)
}

// CleanEffectsExpr normalizes an effects expression to semicolon-separated clauses without {{...}} or "set:" prefix.
func CleanEffectsExpr(raw string) string {
	s := StripOuterDoubleBraces(raw)
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "set:") {
		s = strings.TrimSpace(s[4:])
	}
	clauses := splitEffectClauses(s)
	var clean []string
	for _, c := range clauses {
		t := strings.TrimSpace(c)
		if t != "" {
			clean = append(clean, t)
		}
	}
	return strings.Join(clean, "; ")
}

// ParseEnumDeclaration parses an "enum: name = val1, val2, ..." value string (after "enum:").
func ParseEnumDeclaration(val string) (EnumDef, bool) {
	eqIdx := strings.Index(val, "=")
	if eqIdx <= 0 {
		return EnumDef{}, false
	}
	name := SanitizeVarName(val[:eqIdx])
	if name == "" {
		return EnumDef{}, false
	}
	rawOpts := strings.Split(val[eqIdx+1:], ",")
	var opts []string
	seen := make(map[string]bool)
	for _, ro := range rawOpts {
		opt := SanitizeEnumOption(ro)
		if opt != "" && !seen[opt] {
			seen[opt] = true
			opts = append(opts, opt)
		}
	}
	if len(opts) == 0 {
		return EnumDef{}, false
	}
	return EnumDef{
		Name:   name,
		Values: opts,
	}, true
}

// ParseVariableDeclaration parses a "var: name = value" string (after "var:").
func ParseVariableDeclaration(val string, enums []EnumDef) (VariableDef, bool) {
	eqIdx := strings.Index(val, "=")
	if eqIdx <= 0 {
		name := SanitizeVarName(val)
		if name == "" {
			return VariableDef{}, false
		}
		return VariableDef{Name: name, Kind: VarKindBool, BoolVal: false}, true
	}
	name := SanitizeVarName(val[:eqIdx])
	if name == "" {
		return VariableDef{}, false
	}
	rhs := strings.TrimSpace(val[eqIdx+1:])
	if strings.Contains(rhs, ",") {
		parts := strings.Split(rhs, ",")
		var opts []string
		seen := make(map[string]bool)
		for _, p := range parts {
			opt := SanitizeEnumOption(p)
			if opt != "" && !seen[opt] {
				seen[opt] = true
				opts = append(opts, opt)
			}
		}
		if len(opts) > 0 {
			return VariableDef{
				Name:    name,
				Kind:    VarKindEnum,
				EnumVal: opts[0],
				Options: opts,
			}, true
		}
	}

	lowerRHS := strings.ToLower(strings.Trim(rhs, `"' `))
	switch lowerRHS {
	case "true", "tak", "yes", "on":
		return VariableDef{Name: name, Kind: VarKindBool, BoolVal: true}, true
	case "false", "nie", "no", "off":
		return VariableDef{Name: name, Kind: VarKindBool, BoolVal: false}, true
	}

	if iv, err := strconv.Atoi(lowerRHS); err == nil {
		return VariableDef{Name: name, Kind: VarKindInt, IntVal: iv}, true
	}

	optVal := SanitizeEnumOption(rhs)
	if optVal == "" {
		optVal = "neutralny"
	}
	for _, ed := range enums {
		for _, ev := range ed.Values {
			if strings.EqualFold(ev, optVal) {
				opts := make([]string, len(ed.Values))
				copy(opts, ed.Values)
				return VariableDef{
					Name:     name,
					Kind:     VarKindEnum,
					EnumName: ed.Name,
					EnumVal:  ev,
					Options:  opts,
				}, true
			}
		}
	}

	return VariableDef{
		Name:    name,
		Kind:    VarKindEnum,
		EnumVal: optVal,
		Options: []string{optVal},
	}, true
}

// ResolveBookVariableTypes links enum variables to declared Enums and normalizes options.
func ResolveBookVariableTypes(b *Book) {
	if b == nil {
		return
	}
	for i := range b.Variables {
		v := &b.Variables[i]
		v.Name = SanitizeVarName(v.Name)
		if v.Kind == VarKindEnum {
			if v.EnumName != "" {
				for _, ed := range b.Enums {
					if strings.EqualFold(ed.Name, v.EnumName) && len(ed.Values) > 0 {
						v.Options = append([]string(nil), ed.Values...)
						break
					}
				}
			} else if len(v.Options) <= 1 && v.EnumVal != "" {
				for _, ed := range b.Enums {
					for _, ev := range ed.Values {
						if strings.EqualFold(ev, v.EnumVal) {
							v.EnumName = ed.Name
							v.Options = append([]string(nil), ed.Values...)
							v.EnumVal = ev
							break
						}
					}
					if v.EnumName != "" {
						break
					}
				}
			}
			if len(v.Options) == 0 && v.EnumVal != "" {
				v.Options = []string{v.EnumVal}
			}
			if v.EnumVal == "" && len(v.Options) > 0 {
				v.EnumVal = v.Options[0]
			}
		}
	}
}

// DefaultValueString returns the canonical string representation of a VariableDef's starting value.
func (v VariableDef) DefaultValueString() string {
	switch v.Kind {
	case VarKindBool:
		if v.BoolVal {
			return "true"
		}
		return "false"
	case VarKindInt:
		return strconv.Itoa(v.IntVal)
	case VarKindEnum:
		if v.EnumVal != "" {
			return v.EnumVal
		}
		if len(v.Options) > 0 {
			return v.Options[0]
		}
		return "stan_1"
	default:
		return "false"
	}
}

// FindVariable looks up a variable definition by name (case-insensitive).
func (b *Book) FindVariable(name string) *VariableDef {
	if b == nil {
		return nil
	}
	clean := SanitizeVarName(name)
	if clean == "" {
		return nil
	}
	for i := range b.Variables {
		if strings.EqualFold(b.Variables[i].Name, clean) {
			return &b.Variables[i]
		}
	}
	return nil
}

// DefaultVariablesMap returns a fresh variable state map initialized to the book's defaults.
func DefaultVariablesMap(b *Book) map[string]string {
	out := make(map[string]string)
	if b == nil {
		return out
	}
	for _, v := range b.Variables {
		if v.Name != "" {
			out[v.Name] = v.DefaultValueString()
		}
	}
	return out
}

// CloneVariablesMap returns a shallow copy of a variable state map.
func CloneVariablesMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// EnsureSaveVariables merges book default variables with an existing save's variable map.
func EnsureSaveVariables(b *Book, current map[string]string) map[string]string {
	out := DefaultVariablesMap(b)
	for k, v := range current {
		clean := SanitizeVarName(k)
		if clean != "" {
			out[clean] = v
		}
	}
	return out
}

// ParseChoiceRuleTokens extracts optional leading {{condition}} and trailing {{effects}} around a choice label.
func ParseChoiceRuleTokens(rawLeft string) (label string, cond string, effects string, showLocked bool) {
	s := strings.TrimSpace(rawLeft)
	if strings.HasPrefix(s, "{{") {
		if closeIdx := strings.Index(s, "}}"); closeIdx > 2 {
			after := strings.TrimSpace(s[closeIdx+2:])
			if after != "" {
				rawCond := strings.TrimSpace(s[2:closeIdx])
				if strings.HasPrefix(rawCond, "?") {
					showLocked = true
					if len(rawCond) > 1 && unicode.IsSpace(rune(rawCond[1])) {
						rawCond = strings.TrimSpace(rawCond[1:])
					}
				}
				cond = rawCond
				s = after
			}
		}
	}
	if strings.HasSuffix(s, "}}") {
		if openIdx := strings.LastIndex(s, "{{"); openIdx > 0 {
			before := strings.TrimSpace(s[:openIdx])
			rawEff := strings.TrimSpace(s[openIdx+2 : len(s)-2])
			if before != "" && rawEff != "" {
				effects = rawEff
				s = before
			}
		}
	}
	return strings.TrimSpace(s), cond, effects, showLocked
}

// splitEffectClauses splits an effects expression by ';', ',', or whitespace between complete clauses.
func splitEffectClauses(clean string) []string {
	rawChunks := strings.FieldsFunc(clean, func(r rune) bool {
		return r == ';' || r == ','
	})
	var out []string
	for _, chunk := range rawChunks {
		c := strings.TrimSpace(chunk)
		if c == "" {
			continue
		}
		out = append(out, splitSpaceEffectChunk(c)...)
	}
	return out
}

func isSignedIntToken(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

func splitSpaceEffectChunk(chunk string) []string {
	if strings.Contains(chunk, "==") || strings.Contains(chunk, "!=") ||
		strings.Contains(chunk, ">=") || strings.Contains(chunk, "<=") ||
		strings.Contains(chunk, ">") || strings.Contains(chunk, "<") ||
		strings.Contains(chunk, "&&") || strings.Contains(chunk, "||") {
		return []string{chunk}
	}
	var norm strings.Builder
	norm.Grow(len(chunk) + 8)
	for i := 0; i < len(chunk); i++ {
		ch := chunk[i]
		if (ch == '+' || ch == '-') && i+1 < len(chunk) && chunk[i+1] == '=' {
			norm.WriteString(" ")
			norm.WriteByte(ch)
			norm.WriteString("= ")
			i++
			continue
		}
		if ch == '=' {
			norm.WriteString(" = ")
			continue
		}
		norm.WriteByte(ch)
	}
	tokens := strings.Fields(norm.String())
	if len(tokens) <= 1 {
		return []string{chunk}
	}
	var clauses []string
	for i := 0; i < len(tokens); {
		tok := tokens[i]
		if i+1 < len(tokens) && (tokens[i+1] == "=" || tokens[i+1] == "+=" || tokens[i+1] == "-=") {
			op := tokens[i+1]
			if i+2 >= len(tokens) {
				clauses = append(clauses, tok+" "+op)
				break
			}
			clause := tok + " " + op + " " + tokens[i+2]
			i += 3
			for i < len(tokens) && isSignedIntToken(tokens[i]) {
				clause += " " + tokens[i]
				i++
			}
			clauses = append(clauses, clause)
			continue
		}
		if tok == "!" && i+1 < len(tokens) {
			clauses = append(clauses, "!"+tokens[i+1])
			i += 2
			continue
		}
		if (strings.HasPrefix(tok, "+") || strings.HasPrefix(tok, "-")) && !strings.HasSuffix(tok, "++") && !strings.HasSuffix(tok, "--") {
			clause := tok
			i++
			if (tok == "+" || tok == "-") && i < len(tokens) {
				clause += tokens[i]
				i++
			}
			if i < len(tokens) && isSignedIntToken(tokens[i]) {
				clause += " " + tokens[i]
				i++
			}
			clauses = append(clauses, clause)
			continue
		}
		clauses = append(clauses, tok)
		i++
	}
	return clauses
}

// ApplyEffects executes C/C#-style semicolon-separated variable mutations ("latarka = false; naboje -= 1; frakcja = rzeczni") on state.
func ApplyEffects(expr string, state map[string]string, b *Book) {
	if state == nil {
		return
	}
	clean := CleanEffectsExpr(expr)
	if clean == "" {
		return
	}
	clauses := splitEffectClauses(clean)
	for _, rawClause := range clauses {
		c := strings.TrimSpace(rawClause)
		if c == "" {
			continue
		}
		if strings.HasSuffix(c, "++") && !strings.Contains(c, "=") {
			varName := SanitizeVarName(strings.TrimSuffix(c, "++"))
			if varName != "" {
				cur, _ := strconv.Atoi(resolveOperandString(varName, state, b))
				state[varName] = strconv.Itoa(cur + 1)
			}
			continue
		}
		if strings.HasSuffix(c, "--") && !strings.Contains(c, "=") {
			varName := SanitizeVarName(strings.TrimSuffix(c, "--"))
			if varName != "" {
				cur, _ := strconv.Atoi(resolveOperandString(varName, state, b))
				state[varName] = strconv.Itoa(cur - 1)
			}
			continue
		}
		if strings.HasPrefix(c, "!") && !strings.Contains(c, "=") {
			varName := SanitizeVarName(c[1:])
			if varName != "" {
				if EvaluateCondition(varName, state, b) {
					state[varName] = "false"
				} else {
					state[varName] = "true"
				}
			}
			continue
		}
		if (strings.HasPrefix(c, "+") || strings.HasPrefix(c, "-")) && !strings.Contains(c, "=") {
			sign := 1
			boolLiteral := "true"
			if strings.HasPrefix(c, "-") {
				sign = -1
				boolLiteral = "false"
			}
			rest := strings.Fields(strings.TrimSpace(c[1:]))
			if len(rest) == 0 {
				continue
			}
			varName := SanitizeVarName(rest[0])
			if varName == "" {
				continue
			}
			vDef := b.FindVariable(varName)
			if len(rest) >= 2 {
				if delta, err := strconv.Atoi(rest[1]); err == nil {
					cur, _ := strconv.Atoi(state[varName])
					state[varName] = strconv.Itoa(cur + sign*delta)
					continue
				}
			}
			if vDef != nil && vDef.Kind == VarKindInt {
				cur, _ := strconv.Atoi(state[varName])
				state[varName] = strconv.Itoa(cur + sign)
			} else if curVal, ok := state[varName]; ok {
				if curInt, err := strconv.Atoi(curVal); err == nil {
					state[varName] = strconv.Itoa(curInt + sign)
				} else {
					state[varName] = boolLiteral
				}
			} else {
				state[varName] = boolLiteral
			}
			continue
		}
		if op, idx := findCompoundAssignOp(c); idx > 0 {
			varName := SanitizeVarName(c[:idx])
			rhs := strings.TrimSpace(c[idx+len(op):])
			if varName != "" {
				delta, _ := strconv.Atoi(resolveOperandString(rhs, state, b))
				if op == "-=" {
					delta = -delta
				}
				cur, _ := strconv.Atoi(resolveOperandString(varName, state, b))
				state[varName] = strconv.Itoa(cur + delta)
			}
			continue
		}
		if idx := strings.Index(c, "="); idx > 0 {
			varName := SanitizeVarName(c[:idx])
			rhs := strings.Trim(strings.TrimSpace(c[idx+1:]), `"'`)
			if varName == "" {
				continue
			}
			lowerRHS := strings.ToLower(rhs)
			switch lowerRHS {
			case "true", "tak", "yes", "on":
				state[varName] = "true"
			case "false", "nie", "no", "off":
				state[varName] = "false"
			default:
				if strings.HasPrefix(rhs, "!") {
					targetVar := SanitizeVarName(rhs[1:])
					if targetVar != "" {
						curBool := EvaluateCondition(targetVar, state, b)
						if curBool {
							state[varName] = "false"
						} else {
							state[varName] = "true"
						}
						continue
					}
				}
				if iv, err := strconv.Atoi(rhs); err == nil {
					state[varName] = strconv.Itoa(iv)
				} else {
					state[varName] = SanitizeEnumOption(rhs)
				}
			}
			continue
		}
		// Bare boolean variable name in effects ("latarka") sets it to true
		if varName := SanitizeVarName(c); varName != "" && !strings.ContainsAny(c, " \t+-*/=<>!&|,;:") {
			if vDef := b.FindVariable(varName); vDef == nil || vDef.Kind == VarKindBool {
				state[varName] = "true"
			}
		}
	}
}

func resolveOperandString(tok string, state map[string]string, b *Book) string {
	clean := strings.Trim(strings.TrimSpace(tok), `"'`)
	if clean == "" {
		return ""
	}
	lower := strings.ToLower(clean)
	switch lower {
	case "true", "tak", "yes", "on":
		return "true"
	case "false", "nie", "no", "off":
		return "false"
	}
	if _, err := strconv.Atoi(clean); err == nil {
		return clean
	}
	id := SanitizeVarName(clean)
	if state != nil {
		if v, ok := state[id]; ok {
			return v
		}
	}
	if vDef := b.FindVariable(id); vDef != nil {
		return vDef.DefaultValueString()
	}
	return id
}

// EvaluateCondition evaluates a boolean expression such as "latarka && (klucz_sluzy || zaufanie >= 2)".
func EvaluateCondition(expr string, state map[string]string, b *Book) bool {
	clean := CleanConditionExpr(expr)
	clean = strings.TrimSpace(strings.TrimPrefix(clean, "?"))
	if clean == "" {
		return true
	}
	return evalLogicalOr(clean, state, b)
}

func trimOuterParens(s string) string {
	for maxDepth := len(s); maxDepth > 0; maxDepth-- {
		s = strings.TrimSpace(s)
		if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
			return s
		}
		depth := 0
		matchedOuter := true
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 && i < len(s)-1 {
					matchedOuter = false
				}
			}
		}
		if !matchedOuter || depth != 0 {
			return s
		}
		s = s[1 : len(s)-1]
	}
	return s
}

func splitTopLevelLogical(s, op string) []string {
	var parts []string
	depth := 0
	last := 0
	opLen := len(op)
	for i := 0; i <= len(s)-opLen; i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && s[i:i+opLen] == op {
				parts = append(parts, s[last:i])
				last = i + opLen
				i += opLen - 1
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	parts = append(parts, s[last:])
	return parts
}

func evalLogicalOr(s string, state map[string]string, b *Book) bool {
	s = trimOuterParens(s)
	if parts := splitTopLevelLogical(s, "||"); len(parts) > 1 {
		for _, p := range parts {
			if evalLogicalAnd(p, state, b) {
				return true
			}
		}
		return false
	}
	return evalLogicalAnd(s, state, b)
}

func evalLogicalAnd(s string, state map[string]string, b *Book) bool {
	s = trimOuterParens(s)
	if parts := splitTopLevelLogical(s, "&&"); len(parts) > 1 {
		for _, p := range parts {
			if !evalLogicalAtom(p, state, b) {
				return false
			}
		}
		return true
	}
	return evalLogicalAtom(s, state, b)
}

func evalLogicalAtom(s string, state map[string]string, b *Book) bool {
	invert := false
	for step := 0; step < 32; step++ {
		s = strings.TrimSpace(s)
		if s == "" {
			return !invert
		}
		if unwrapped := trimOuterParens(s); unwrapped != s {
			res := evalLogicalOr(unwrapped, state, b)
			if invert {
				return !res
			}
			return res
		}
		if strings.HasPrefix(s, "?") {
			s = s[1:]
			continue
		}
		if strings.HasPrefix(s, "!") && !strings.HasPrefix(s, "!=") {
			invert = !invert
			s = s[1:]
			continue
		}
		break
	}
	if s == "" {
		return !invert
	}
	res := evalLogicalComparisonOrVar(s, state, b)
	if invert {
		return !res
	}
	return res
}

func evalLogicalComparisonOrVar(s string, state map[string]string, b *Book) bool {
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<", "="} {
		if idx := strings.Index(s, op); idx > 0 {
			lhs := strings.TrimSpace(s[:idx])
			rhs := strings.TrimSpace(s[idx+len(op):])
			lVal := resolveOperandString(lhs, state, b)
			rVal := resolveOperandString(rhs, state, b)
			lInt, lErr := strconv.Atoi(lVal)
			rInt, rErr := strconv.Atoi(rVal)
			if lErr == nil && rErr == nil {
				switch op {
				case "==", "=":
					return lInt == rInt
				case "!=":
					return lInt != rInt
				case ">=":
					return lInt >= rInt
				case "<=":
					return lInt <= rInt
				case ">":
					return lInt > rInt
				case "<":
					return lInt < rInt
				}
			}
			switch op {
			case "==", "=":
				return strings.EqualFold(lVal, rVal)
			case "!=":
				return !strings.EqualFold(lVal, rVal)
			default:
				return false
			}
		}
	}

	val := resolveOperandString(s, state, b)
	lower := strings.ToLower(val)
	if lower == "true" || lower == "tak" || lower == "yes" {
		return true
	}
	if lower == "false" || lower == "nie" || lower == "no" || lower == "" {
		return false
	}
	if iv, err := strconv.Atoi(val); err == nil {
		return iv != 0
	}
	return true
}

func findCompoundAssignOp(clause string) (string, int) {
	if idx := strings.Index(clause, "+="); idx > 0 {
		return "+=", idx
	}
	if idx := strings.Index(clause, "-="); idx > 0 {
		return "-=", idx
	}
	return "", -1
}
