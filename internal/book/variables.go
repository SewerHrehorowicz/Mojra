package book

import (
	"fmt"
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
	if strings.HasPrefix(lower, "if?:") {
		s = "? " + strings.TrimSpace(s[4:])
	} else if strings.HasPrefix(lower, "if:") {
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
	} else if strings.HasPrefix(lower, "then:") {
		s = strings.TrimSpace(s[5:])
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

// IsStandaloneSceneEffectLine reports whether a trimmed line at the top of a scene is a {{ ... }} effect statement
// (e.g. "{{+odwiedzono_sluze, zaufanie += 1}}") rather than a conditional prose block opener ("{{latarka}}").
func IsStandaloneSceneEffectLine(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") || len(trimmed) < 5 {
		return "", false
	}
	inner := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
	if inner == "" || inner == "/" || inner == ":" || strings.Contains(inner, "{{") || strings.Contains(inner, "}}") {
		return "", false
	}
	if strings.Contains(inner, ":") || strings.Contains(inner, "&&") || strings.Contains(inner, "||") {
		return "", false
	}
	clauses := strings.Split(inner, ",")
	for _, rawClause := range clauses {
		c := strings.TrimSpace(rawClause)
		if c == "" {
			return "", false
		}
		if strings.HasPrefix(c, "+") || strings.HasPrefix(c, "-") || (strings.HasPrefix(c, "!") && !strings.HasPrefix(c, "!=") && !strings.HasPrefix(c, "!?")) {
			rest := SanitizeVarName(c[1:])
			if rest == "" {
				return "", false
			}
			continue
		}
		if strings.Contains(c, "==") || strings.Contains(c, "!=") || strings.Contains(c, ">=") || strings.Contains(c, "<=") || strings.Contains(c, ">") || strings.Contains(c, "<") {
			return "", false
		}
		if strings.Contains(c, "+=") || strings.Contains(c, "-=") || strings.Contains(c, "=") {
			continue
		}
		return "", false
	}
	return inner, true
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

// FormatChoiceLeftWithRules formats the left side of a choice line ("{{cond}} Label {{effects}}") for story.txt.
func FormatChoiceLeftWithRules(ch Choice) string {
	lbl := strings.TrimSpace(ch.Label)
	if lbl == "" {
		lbl = "Dalej"
	}
	var sb strings.Builder
	cond := StripOuterDoubleBraces(ch.Condition)
	if cond != "" {
		sb.WriteString("{{")
		if ch.ShowWhenLocked && !strings.HasPrefix(cond, "?") {
			sb.WriteString("? ")
		}
		sb.WriteString(cond)
		sb.WriteString("}} ")
	}
	sb.WriteString(lbl)
	eff := StripOuterDoubleBraces(ch.Effects)
	if eff != "" {
		sb.WriteString(" {{")
		sb.WriteString(eff)
		sb.WriteString("}}")
	}
	return sb.String()
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
		if strings.HasPrefix(c, "+") && !strings.Contains(c, "=") {
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
					state[varName] = strconv.Itoa(cur + delta)
					continue
				}
			}
			if vDef != nil && vDef.Kind == VarKindInt {
				cur, _ := strconv.Atoi(state[varName])
				state[varName] = strconv.Itoa(cur + 1)
			} else if curVal, ok := state[varName]; ok {
				if curInt, err := strconv.Atoi(curVal); err == nil {
					state[varName] = strconv.Itoa(curInt + 1)
				} else {
					state[varName] = "true"
				}
			} else {
				state[varName] = "true"
			}
			continue
		}
		if strings.HasPrefix(c, "-") && !strings.Contains(c, "=") {
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
					state[varName] = strconv.Itoa(cur - delta)
					continue
				}
			}
			if vDef != nil && vDef.Kind == VarKindInt {
				cur, _ := strconv.Atoi(state[varName])
				state[varName] = strconv.Itoa(cur - 1)
			} else if curVal, ok := state[varName]; ok {
				if curInt, err := strconv.Atoi(curVal); err == nil {
					state[varName] = strconv.Itoa(curInt - 1)
				} else {
					state[varName] = "false"
				}
			} else {
				state[varName] = "false"
			}
			continue
		}
		if idx := strings.Index(c, "+="); idx > 0 {
			varName := SanitizeVarName(c[:idx])
			rhs := strings.TrimSpace(c[idx+2:])
			if varName != "" {
				delta, _ := strconv.Atoi(resolveOperandString(rhs, state, b))
				cur, _ := strconv.Atoi(resolveOperandString(varName, state, b))
				state[varName] = strconv.Itoa(cur + delta)
			}
			continue
		}
		if idx := strings.Index(c, "-="); idx > 0 {
			varName := SanitizeVarName(c[:idx])
			rhs := strings.TrimSpace(c[idx+2:])
			if varName != "" {
				delta, _ := strconv.Atoi(resolveOperandString(rhs, state, b))
				cur, _ := strconv.Atoi(resolveOperandString(varName, state, b))
				state[varName] = strconv.Itoa(cur - delta)
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
	for {
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
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if unwrapped := trimOuterParens(s); unwrapped != s {
		return evalLogicalOr(unwrapped, state, b)
	}
	if strings.HasPrefix(s, "?") {
		return evalLogicalAtom(s[1:], state, b)
	}
	if strings.HasPrefix(s, "!") && !strings.HasPrefix(s, "!=") {
		return !evalLogicalAtom(s[1:], state, b)
	}
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

// HasDynamicProse reports whether raw prose text contains any conditional block directives (if:/elif:/else:/endif:) or inline tokens ({{...}}).
func HasDynamicProse(raw string) bool {
	if strings.Contains(raw, "{{") {
		return true
	}
	lowerRaw := strings.ToLower(raw)
	for _, line := range strings.Split(lowerRaw, "\n") {
		if isProseSyntaxDirective(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}

// RenderSceneProseHTML is the single canonical prose evaluation + HTML rendering pipeline
// shared by Player mode (#view-player) and Editor output preview (#modal-scene-text).
func RenderSceneProseHTML(sc *Scene, state map[string]string, b *Book) string {
	if sc == nil {
		return "<p></p>"
	}
	if !HasDynamicProse(sc.Text) {
		return sc.EnsureHTML()
	}
	return RenderSafeProseHTML(EvaluateProseText(sc.Text, state, b))
}

// EvaluateProseText evaluates multi-paragraph if:/elif:/else:/endif: blocks and inline {{...}} tokens,
// returning plain text ready for RenderSafeProseHTML.
func EvaluateProseText(raw string, state map[string]string, b *Book) string {
	if !HasDynamicProse(raw) {
		return raw
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	type blockFrame struct {
		branchTaken  bool
		branchActive bool
	}
	var stack []blockFrame
	isCurrentlyActive := func() bool {
		for _, f := range stack {
			if !f.branchActive {
				return false
			}
		}
		return true
	}

	var outLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if kind, condExpr, ok := parseProseBlockDirective(trimmed, state, b); ok {
			switch kind {
			case "endif":
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				continue
			case "else":
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					top.branchActive = !top.branchTaken
					top.branchTaken = true
				}
				continue
			case "elif":
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					if top.branchTaken {
						top.branchActive = false
					} else {
						met := EvaluateCondition(condExpr, state, b)
						top.branchActive = met
						if met {
							top.branchTaken = true
						}
					}
				}
				continue
			case "if":
				met := EvaluateCondition(condExpr, state, b)
				stack = append(stack, blockFrame{branchTaken: met, branchActive: met})
				continue
			case "set":
				continue
			}
		}
		if !isCurrentlyActive() {
			continue
		}
		if strings.Contains(line, "{{") {
			line = evaluateInlineProseTokens(line, state, b)
		}
		outLines = append(outLines, line)
	}
	return strings.Join(outLines, "\n")
}

func parseProseBlockDirective(trimmed string, _ map[string]string, _ *Book) (string, string, bool) {
	if strings.EqualFold(trimmed, ":end") {
		return "endif", "", true
	}
	colonIdx := strings.IndexByte(trimmed, ':')
	if colonIdx <= 0 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
	val := strings.TrimSpace(trimmed[colonIdx+1:])
	switch key {
	case "else":
		if val == "" {
			return "else", "", true
		}
	case "endif":
		if val == "" {
			return "endif", "", true
		}
	case "elif", "else if":
		if val != "" {
			return "elif", val, true
		}
	case "if", "if?", "gdy", "warunek":
		if val != "" {
			return "if", val, true
		}
	case "set", "then":
		return "set", val, true
	}
	return "", "", false
}

func isProseSyntaxDirective(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	_, _, ok := parseProseBlockDirective(trimmed, nil, nil)
	return ok
}

func isProseSyntaxLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if trimmed == "---" || trimmed == "***" {
		return true
	}
	return isProseSyntaxDirective(trimmed)
}

// EncodeVariablesStateSpec encodes current variable state values as "name=val;..." for client-side live condition evaluation.
func EncodeVariablesStateSpec(state map[string]string, b *Book) string {
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
		val := resolveOperandString(name, state, b)
		parts = append(parts, name+"="+val)
	}
	return strings.Join(parts, ";")
}

// BuildSceneProseHighlightSegments splits raw scene prose text into contiguous segments
// with "text" and "is_syntax" (true for if:/elif:/else:/endif:/set: directive lines, ---/*** dividers, inline {{...}} tokens,
// and prose inside conditional branches that do not satisfy state), preserving every character and newline 1:1.
func BuildSceneProseHighlightSegments(raw string, state map[string]string, b *Book) []map[string]any {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if normalized == "" {
		return []map[string]any{
			{"text": "", "is_syntax": false},
		}
	}
	lines := strings.Split(normalized, "\n")
	type segItem struct {
		text string
		syn  bool
	}
	var segs []segItem
	appendSeg := func(txt string, syn bool) {
		if txt == "" {
			return
		}
		if len(segs) > 0 && segs[len(segs)-1].syn == syn {
			segs[len(segs)-1].text += txt
			return
		}
		segs = append(segs, segItem{text: txt, syn: syn})
	}

	type blockFrame struct {
		branchTaken  bool
		branchActive bool
	}
	var stack []blockFrame
	isCurrentlyActive := func() bool {
		for _, f := range stack {
			if !f.branchActive {
				return false
			}
		}
		return true
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if kind, condExpr, ok := parseProseBlockDirective(trimmed, state, b); ok {
			switch kind {
			case "endif":
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			case "else":
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					top.branchActive = !top.branchTaken
					top.branchTaken = true
				}
			case "elif":
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					if top.branchTaken {
						top.branchActive = false
					} else {
						met := EvaluateCondition(condExpr, state, b)
						top.branchActive = met
						if met {
							top.branchTaken = true
						}
					}
				}
			case "if":
				met := EvaluateCondition(condExpr, state, b)
				stack = append(stack, blockFrame{branchTaken: met, branchActive: met})
			}
			appendSeg(line, true)
		} else if trimmed == "---" || trimmed == "***" || !isCurrentlyActive() {
			appendSeg(line, true)
		} else {
			rest := line
			for {
				openIdx := strings.Index(rest, "{{")
				if openIdx < 0 {
					appendSeg(rest, false)
					break
				}
				appendSeg(rest[:openIdx], false)
				afterOpen := rest[openIdx+2:]
				closeIdx := strings.Index(afterOpen, "}}")
				if closeIdx < 0 {
					appendSeg(rest[openIdx:], false)
					break
				}
				appendSeg(rest[openIdx:openIdx+2+closeIdx+2], true)
				rest = afterOpen[closeIdx+2:]
			}
		}
		if i < len(lines)-1 {
			lastSyn := len(segs) > 0 && segs[len(segs)-1].syn
			appendSeg("\n", lastSyn)
		}
	}
	if strings.HasSuffix(normalized, "\n") {
		appendSeg("\n", false)
	}
	if len(segs) == 0 {
		return []map[string]any{
			{"text": "", "is_syntax": false},
		}
	}
	out := make([]map[string]any, len(segs))
	for i, s := range segs {
		out[i] = map[string]any{
			"text":      s.text,
			"is_syntax": s.syn,
		}
	}
	return out
}

func isStandaloneBlockDirective(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") || len(trimmed) < 3 {
		return false
	}
	inner := trimmed[2 : len(trimmed)-2]
	return !strings.Contains(inner, "{{") && !strings.Contains(inner, "}}")
}

func isPureVariableLookup(inner string, state map[string]string, b *Book) bool {
	clean := strings.TrimSpace(inner)
	if clean == "" || strings.ContainsAny(clean, " !&|=<>+-()") {
		return false
	}
	vDef := b.FindVariable(clean)
	if vDef != nil {
		// Standalone line "{{latarka}}" for a bool variable is a block condition opener,
		// whereas "{{naboje}}" or "{{frakcja}}" in inline prose is interpolation.
		return vDef.Kind != VarKindBool
	}
	return false
}

func evaluateInlineProseTokens(line string, state map[string]string, b *Book) string {
	var sb strings.Builder
	rest := line
	for {
		openIdx := strings.Index(rest, "{{")
		if openIdx < 0 {
			sb.WriteString(rest)
			break
		}
		sb.WriteString(rest[:openIdx])
		afterOpen := rest[openIdx+2:]
		closeIdx := strings.Index(afterOpen, "}}")
		if closeIdx < 0 {
			sb.WriteString(rest[openIdx:])
			break
		}
		inner := strings.TrimSpace(afterOpen[:closeIdx])
		rest = afterOpen[closeIdx+2:]

		if colonIdx := strings.Index(inner, ":"); colonIdx > 0 {
			condExpr := strings.TrimSpace(inner[:colonIdx])
			branchesRaw := inner[colonIdx+1:]
			var trueText, falseText string
			if pipeIdx := strings.Index(branchesRaw, "|"); pipeIdx >= 0 {
				trueText = strings.TrimSpace(branchesRaw[:pipeIdx])
				falseText = strings.TrimSpace(branchesRaw[pipeIdx+1:])
			} else {
				trueText = strings.TrimSpace(branchesRaw)
			}
			if EvaluateCondition(condExpr, state, b) {
				sb.WriteString(trueText)
			} else {
				sb.WriteString(falseText)
			}
			continue
		}

		// Variable interpolation {{varName}}
		id := SanitizeVarName(inner)
		if id != "" {
			val := resolveOperandString(id, state, b)
			if vDef := b.FindVariable(id); vDef != nil && vDef.Kind == VarKindBool {
				if val == "true" {
					val = "Tak"
				} else {
					val = "Nie"
				}
			}
			sb.WriteString(val)
		}
	}
	return sb.String()
}

// BuildSceneProsePreviewParagraphs constructs template-ready paragraphs and segments for the 3 Scene Text Preview modes:
//   - "normal": shows all lines with subtle gray {{...}} syntax tokens
//   - "highlight": shows gray {{...}} tokens + highlights active conditional branches and dims inactive ones + shows =val on variables
//   - "output": shows only the evaluated output text (identical to Player view)
func BuildSceneProsePreviewParagraphs(raw string, mode string, state map[string]string, b *Book) []map[string]any {
	normMode := strings.ToLower(strings.TrimSpace(mode))
	if normMode == "output" {
		evaluated := EvaluateProseText(raw, state, b)
		lines := strings.Split(strings.ReplaceAll(evaluated, "\r\n", "\n"), "\n")
		var out []map[string]any
		var curLines []string
		flush := func() {
			if len(curLines) == 0 {
				return
			}
			out = append(out, map[string]any{
				"is_divider":  false,
				"is_quote":    false,
				"is_active":   false,
				"is_inactive": false,
				"segments": []map[string]any{
					{
						"text":        strings.Join(curLines, "\n"),
						"is_syntax":   false,
						"is_active":   false,
						"is_inactive": false,
						"is_var_val":  false,
					},
				},
			})
			curLines = nil
		}
		for _, ln := range lines {
			t := strings.TrimSpace(ln)
			if t == "" {
				flush()
				continue
			}
			if t == "---" || t == "***" {
				flush()
				out = append(out, map[string]any{"is_divider": true})
				continue
			}
			curLines = append(curLines, t)
		}
		flush()
		return out
	}

	isHighlight := normMode == "highlight"
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	type blockFrame struct {
		branchTaken  bool
		branchActive bool
	}
	var stack []blockFrame
	blockState := func() (inBlock bool, active bool) {
		if len(stack) == 0 {
			return false, true
		}
		for _, f := range stack {
			if !f.branchActive {
				return true, false
			}
		}
		return true, true
	}

	var out []map[string]any
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "---" || trimmed == "***" {
			out = append(out, map[string]any{"is_divider": true})
			continue
		}
		if kind, condExpr, ok := parseProseBlockDirective(trimmed, state, b); ok {
			switch kind {
			case "endif":
				inB, act := blockState()
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				out = append(out, map[string]any{
					"is_divider":  false,
					"is_active":   isHighlight && inB && act,
					"is_inactive": isHighlight && inB && !act,
					"segments": []map[string]any{
						{"text": trimmed, "is_syntax": true},
					},
				})
				continue
			case "else":
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					top.branchActive = !top.branchTaken
					top.branchTaken = true
				}
				inB, act := blockState()
				out = append(out, map[string]any{
					"is_divider":  false,
					"is_active":   isHighlight && inB && act,
					"is_inactive": isHighlight && inB && !act,
					"segments": []map[string]any{
						{"text": trimmed, "is_syntax": true},
					},
				})
				continue
			case "elif":
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					if top.branchTaken {
						top.branchActive = false
					} else {
						met := EvaluateCondition(condExpr, state, b)
						top.branchActive = met
						if met {
							top.branchTaken = true
						}
					}
				}
				inB, act := blockState()
				out = append(out, map[string]any{
					"is_divider":  false,
					"is_active":   isHighlight && inB && act,
					"is_inactive": isHighlight && inB && !act,
					"segments": []map[string]any{
						{"text": trimmed, "is_syntax": true},
					},
				})
				continue
			case "if":
				met := EvaluateCondition(condExpr, state, b)
				stack = append(stack, blockFrame{branchTaken: met, branchActive: met})
				inB, act := blockState()
				out = append(out, map[string]any{
					"is_divider":  false,
					"is_active":   isHighlight && inB && act,
					"is_inactive": isHighlight && inB && !act,
					"segments": []map[string]any{
						{"text": trimmed, "is_syntax": true},
					},
				})
				continue
			case "set":
				inB, act := blockState()
				out = append(out, map[string]any{
					"is_divider":  false,
					"is_active":   isHighlight && inB && act,
					"is_inactive": isHighlight && inB && !act,
					"segments": []map[string]any{
						{"text": trimmed, "is_syntax": true},
					},
				})
				continue
			}
		}

		inB, act := blockState()
		segs := buildLinePreviewSegments(trimmed, isHighlight, act, state, b)
		out = append(out, map[string]any{
			"is_divider":  false,
			"is_active":   isHighlight && inB && act,
			"is_inactive": isHighlight && inB && !act,
			"segments":    segs,
		})
	}
	return out
}

func buildLinePreviewSegments(line string, isHighlight bool, blockActive bool, state map[string]string, b *Book) []map[string]any {
	var segs []map[string]any
	appendSeg := func(txt string, syn, act, inact, varVal bool) {
		if txt == "" {
			return
		}
		segs = append(segs, map[string]any{
			"text":        txt,
			"is_syntax":   syn,
			"is_active":   act,
			"is_inactive": inact,
			"is_var_val":  varVal,
		})
	}

	rest := line
	for {
		openIdx := strings.Index(rest, "{{")
		if openIdx < 0 {
			appendSeg(rest, false, false, isHighlight && !blockActive, false)
			break
		}
		appendSeg(rest[:openIdx], false, false, isHighlight && !blockActive, false)
		afterOpen := rest[openIdx+2:]
		closeIdx := strings.Index(afterOpen, "}}")
		if closeIdx < 0 {
			appendSeg(rest[openIdx:], false, false, isHighlight && !blockActive, false)
			break
		}
		inner := afterOpen[:closeIdx]
		rest = afterOpen[closeIdx+2:]

		if !isHighlight {
			appendSeg("{{"+inner+"}}", true, false, false, false)
			continue
		}

		if colonIdx := strings.Index(inner, ":"); colonIdx > 0 {
			condExpr := strings.TrimSpace(inner[:colonIdx])
			branchesRaw := inner[colonIdx+1:]
			met := blockActive && EvaluateCondition(condExpr, state, b)
			appendSeg("{{"+condExpr+": ", true, false, false, false)
			if pipeIdx := strings.Index(branchesRaw, "|"); pipeIdx >= 0 {
				tTxt := branchesRaw[:pipeIdx]
				fTxt := branchesRaw[pipeIdx+1:]
				appendSeg(tTxt, false, met, !met, false)
				appendSeg("|", true, false, false, false)
				appendSeg(fTxt, false, blockActive && !met, !blockActive || met, false)
			} else {
				appendSeg(branchesRaw, false, met, !met, false)
			}
			appendSeg("}}", true, false, false, false)
			continue
		}

		id := SanitizeVarName(inner)
		val := resolveOperandString(id, state, b)
		if vDef := b.FindVariable(id); vDef != nil && vDef.Kind == VarKindBool {
			if val == "true" {
				val = "Tak"
			} else {
				val = "Nie"
			}
		}
		appendSeg("{{"+strings.TrimSpace(inner)+"}}", true, false, false, false)
		if id != "" {
			appendSeg(fmt.Sprintf("=%s", val), false, false, false, true)
		}
	}
	return segs
}

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
