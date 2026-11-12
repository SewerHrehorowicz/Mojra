//go:build !(js && wasm)

package tpl

import (
	"fmt"
	"strings"
)

type cssSwitchedBucket struct {
	Selector string
	Decls    []string
}

type cssEmitTask struct {
	node            *cssNode
	currentSelector string
	closeAtSelector string
	startOutIdx     int
}

func evalCSSCondNode(node *cssNode, ctx *RenderContext, stripDev bool) bool {
	cond := node.Cond
	invert := false
	if rest, ok := strings.CutPrefix(cond, tssKwNotSpace); ok {
		invert = true
		cond = strings.TrimSpace(rest)
	}
	if cond == cssDevModeKey && stripDev {
		return invert
	}
	if ctx != nil {
		val, ok := resolvePath(ctx, cond)
		truthy := ok && isTruthy(val)
		if invert {
			truthy = !truthy
		}
		return truthy
	}
	return true
}

func resolveSwitchCaseTargetSelector(swDef *CSSSwitchDef, ch *cssNode, line int, caseKeys []string, currentSelector string) (string, error) {
	branchSelectors := make([]string, 0, len(caseKeys))
	for _, ck := range caseKeys {
		bs, exists := swDef.Cases[ck]
		if !exists {
			return "", fmt.Errorf(errCSSUnknownSwitchCaseFmt, ch.File, line, ck, ch.SwitchName)
		}
		branchSelectors = append(branchSelectors, bs)
	}
	combinedBranch := strings.Join(branchSelectors, ", ")
	return combineSwitchAndRuleSelector(combinedBranch, currentSelector), nil
}

func emitCSSNode(rootNode *cssNode, rootSelector string, switches map[string]*CSSSwitchDef, ctx *RenderContext, stripDev bool) ([]string, error) {
	var out []string
	stack := []cssEmitTask{{node: rootNode, currentSelector: rootSelector}}

	for step := 0; step < 32768 && len(stack) > 0; step++ {
		task := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if task.closeAtSelector != "" {
			chBlocks := out[task.startOutIdx:]
			out = out[:task.startOutIdx]
			if len(chBlocks) > 0 {
				innerBlocks := make([]string, len(chBlocks))
				for i, b := range chBlocks {
					innerBlocks[i] = "  " + strings.ReplaceAll(b, "\n", "\n  ")
				}
				out = append(out, fmt.Sprintf(cssBlockFmt, task.closeAtSelector, strings.Join(innerBlocks, "\n\n")))
			}
			continue
		}

		node := task.node
		currentSelector := task.currentSelector
		if node == nil {
			continue
		}

		if node.Kind == cssKindCond && !evalCSSCondNode(node, ctx, stripDev) {
			continue
		}

		if node.Kind == cssKindRule && len(node.Selector) > 0 && node.Selector[0] == '@' {
			if len(node.Decls) > 0 && len(node.Children) == 0 && currentSelector == "" {
				decls := formatCSSDecls(node.Decls, ctx, "  ")
				out = append(out, fmt.Sprintf(cssBlockFmt, node.Selector, strings.Join(decls, "\n")))
				continue
			}
			bodyNode := *node
			bodyNode.Kind = cssKindRoot
			bodyNode.Selector = ""
			stack = append(stack,
				cssEmitTask{closeAtSelector: node.Selector, startOutIdx: len(out)},
				cssEmitTask{node: &bodyNode, currentSelector: currentSelector},
			)
			continue
		}

		var baseDecls []string
		if len(node.Decls) > 0 {
			baseDecls = formatCSSDecls(node.Decls, ctx, "  ")
		}

		var switchedOrder []string
		switchedBuckets := make(map[string]*cssSwitchedBucket)
		var beforeAfterBuckets []*cssSwitchedBucket

		addSwitchedDecl := func(sel, formattedDecl string) {
			b, ok := switchedBuckets[sel]
			if !ok {
				b = &cssSwitchedBucket{Selector: sel}
				switchedBuckets[sel] = b
				switchedOrder = append(switchedOrder, sel)
			}
			b.Decls = append(b.Decls, formattedDecl)
		}

		for _, ch := range node.Children {
			if ch.Kind == cssKindPropSwitch {
				swDef, ok := switches[ch.SwitchName]
				if !ok {
					return nil, fmt.Errorf(errCSSUnknownSwitchFmt, ch.File, ch.Line, ch.SwitchName)
				}
				propName := normalizeCSSPropKey(ch.PropKey)
				for _, pc := range ch.PropCases {
					targetSel, err := resolveSwitchCaseTargetSelector(swDef, ch, pc.Line, pc.CaseKeys, currentSelector)
					if err != nil {
						return nil, err
					}
					val := transformCSSValue(pc.Val, ctx)

					if pseudo := cssPseudoShorthands[ch.PropKey]; pseudo != "" {
						pseudoSel := combineNestedSelectors(targetSel, pseudo)
						beforeAfterBuckets = append(beforeAfterBuckets, &cssSwitchedBucket{
							Selector: pseudoSel,
							Decls:    []string{fmt.Sprintf(cssContentDeclFmt, val)},
						})
					} else {
						addSwitchedDecl(targetSel, fmt.Sprintf(cssIndentedDeclFmt, propName, val))
					}
				}
			}
		}

		if len(baseDecls) > 0 {
			if currentSelector == "" {
				mergedIntoRoot := false
				for _, sel := range switchedOrder {
					if strings.Contains(sel, cssRootSelector) {
						switchedBuckets[sel].Decls = append(switchedBuckets[sel].Decls, baseDecls...)
						mergedIntoRoot = true
						break
					}
				}
				if !mergedIntoRoot {
					out = append(out, formatCSSBlock(cssRootSelector, baseDecls))
				}
			} else {
				out = append(out, formatCSSBlock(currentSelector, baseDecls))
			}
		}

		for _, sel := range switchedOrder {
			bucket := switchedBuckets[sel]
			if len(bucket.Decls) > 0 {
				out = append(out, formatCSSBlock(bucket.Selector, bucket.Decls))
			}
		}
		for _, ba := range beforeAfterBuckets {
			out = append(out, formatCSSBlock(ba.Selector, ba.Decls))
		}

		var childTasks []cssEmitTask
		for _, ch := range node.Children {
			switch ch.Kind {
			case cssKindPropSwitch:
				continue
			case cssKindRule:
				if len(ch.Selector) > 0 && ch.Selector[0] == '@' {
					childTasks = append(childTasks, cssEmitTask{node: ch, currentSelector: currentSelector})
					continue
				}
				nextSel := combineNestedSelectors(currentSelector, ch.Selector)
				childTasks = append(childTasks, cssEmitTask{node: ch, currentSelector: nextSel})

			case cssKindBlockSwitch:
				swDef, ok := switches[ch.SwitchName]
				if !ok {
					return nil, fmt.Errorf(errCSSUnknownSwitchFmt, ch.File, ch.Line, ch.SwitchName)
				}
				for _, caseNode := range ch.Children {
					targetSel, err := resolveSwitchCaseTargetSelector(swDef, ch, caseNode.Line, caseNode.CaseKeys, currentSelector)
					if err != nil {
						return nil, err
					}
					childTasks = append(childTasks, cssEmitTask{node: caseNode, currentSelector: targetSel})
				}

			case cssKindCond:
				childTasks = append(childTasks, cssEmitTask{node: ch, currentSelector: currentSelector})
			}
		}
		for i := len(childTasks) - 1; i >= 0; i-- {
			stack = append(stack, childTasks[i])
		}
	}

	return out, nil
}

func formatCSSBlock(selector string, decls []string) string {
	parts := splitTopLevelCommas(selector)
	formattedSel := strings.Join(parts, ",\n")
	return fmt.Sprintf(cssBlockFmt, formattedSel, strings.Join(decls, "\n"))
}

func formatCSSDecls(decls []cssDecl, ctx *RenderContext, indent string) []string {
	out := make([]string, 0, len(decls))
	for _, d := range decls {
		key := normalizeCSSPropKey(d.Key)
		val := transformCSSValue(d.Val, ctx)
		out = append(out, fmt.Sprintf(cssDeclFmt, indent, key, val))
	}
	return out
}

func normalizeCSSPropKey(key string) string {
	if strings.HasPrefix(key, tssDollarPrefix) {
		return cssVarPrefix + key[1:]
	}
	return key
}

func transformCSSValue(val string, ctx *RenderContext) string {
	if strings.Contains(val, "{") {
		val = interpolateCSSGoVars(val, ctx)
	}
	if strings.Contains(val, tssDollarPrefix) || strings.Contains(val, cssVarPrefix) {
		val = replaceCSSDollarVars(val)
	}
	if strings.Contains(val, "#") {
		val = expandShortHexColors(val)
	}
	return val
}

func stepCSSQuoteChar(s string, i int, inQuote *byte, b *strings.Builder) (int, bool) {
	ch := s[i]
	if *inQuote != 0 {
		if ch == '\\' && i+1 < len(s) {
			b.WriteByte(s[i])
			b.WriteByte(s[i+1])
			return i + 2, true
		}
		if ch == *inQuote {
			*inQuote = 0
		}
		b.WriteByte(ch)
		return i + 1, true
	}
	if ch == '"' || ch == '\'' {
		*inQuote = ch
		b.WriteByte(ch)
		return i + 1, true
	}
	return i, false
}

func expandShortHexColors(s string) string {
	var b strings.Builder
	inQuote := byte(0)
	i := 0
	for i < len(s) {
		if nextI, handled := stepCSSQuoteChar(s, i, &inQuote, &b); handled {
			i = nextI
			continue
		}
		ch := s[i]
		if ch == '#' && (i == 0 || !isCSSIdentChar(s[i-1])) {
			j := i + 1
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || (s[j] >= 'a' && s[j] <= 'f') || (s[j] >= 'A' && s[j] <= 'F')) {
				j++
			}
			hexLen := j - (i + 1)
			if (hexLen == 1 || hexLen == 2) && (j == len(s) || !isCSSIdentChar(s[j])) {
				unit := s[i+1 : j]
				b.WriteByte('#')
				if hexLen == 1 {
					b.WriteString(strings.Repeat(unit, 6))
				} else {
					b.WriteString(strings.Repeat(unit, 3))
				}
				i = j
				continue
			}
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func interpolateCSSGoVars(s string, ctx *RenderContext) string {
	var b strings.Builder
	inQuote := byte(0)
	i := 0
	for i < len(s) {
		ch := s[i]
		if inQuote != 0 && ch == '{' {
			end := strings.IndexByte(s[i:], '}')
			if end > 1 {
				key := strings.TrimSpace(s[i+1 : i+end])
				if ctx != nil {
					if v, ok := resolvePath(ctx, key); ok {
						escaped := strings.ReplaceAll(valToString(v), string(inQuote), `\`+string(inQuote))
						b.WriteString(escaped)
					}
				} else {
					b.WriteString(s[i : i+end+1])
				}
				i += end + 1
				continue
			}
		}
		if nextI, handled := stepCSSQuoteChar(s, i, &inQuote, &b); handled {
			i = nextI
			continue
		}

		if ch == '{' {
			end := strings.IndexByte(s[i:], '}')
			if end > 1 {
				key := strings.TrimSpace(s[i+1 : i+end])
				if ctx != nil {
					if v, ok := resolvePath(ctx, key); ok {
						b.WriteString(valToString(v))
						i += end + 1
						continue
					}
				}
				b.WriteString(cssVarOpenPrefix)
				b.WriteString(key)
				b.WriteByte(')')
				i += end + 1
				continue
			}
		}

		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func consumeCSSVarRef(s string, start int, b *strings.Builder) int {
	j := start
	for j < len(s) && isCSSIdentChar(s[j]) {
		j++
	}
	varName := s[start:j]
	if j < len(s) && s[j] == '(' {
		depth := 1
		k := j + 1
		for k < len(s) && depth > 0 {
			if s[k] == '(' {
				depth++
			} else if s[k] == ')' {
				depth--
			}
			k++
		}
		if depth == 0 {
			fallback := replaceCSSDollarVars(strings.TrimSpace(s[j+1 : k-1]))
			b.WriteString(cssVarOpenPrefix)
			b.WriteString(varName)
			b.WriteString(", ")
			b.WriteString(fallback)
			b.WriteByte(')')
			return k
		}
	}
	b.WriteString(cssVarOpenPrefix)
	b.WriteString(varName)
	b.WriteByte(')')
	return j
}

func replaceCSSDollarVars(s string) string {
	var b strings.Builder
	inQuote := byte(0)
	i := 0
	for i < len(s) {
		if nextI, handled := stepCSSQuoteChar(s, i, &inQuote, &b); handled {
			i = nextI
			continue
		}
		ch := s[i]
		if ch == '$' && i+1 < len(s) && isCSSIdentStart(s[i+1]) {
			i = consumeCSSVarRef(s, i+1, &b)
			continue
		}
		if ch == '-' && i+2 < len(s) && s[i+1] == '-' && isCSSIdentStart(s[i+2]) && (i == 0 || !isCSSIdentChar(s[i-1])) {
			prev := i - 1
			for prev >= 0 && (s[prev] == ' ' || s[prev] == '\t') {
				prev--
			}
			alreadyInVar := prev >= 3 && s[prev-3:prev+1] == cssVarCallPrefix && (prev-3 == 0 || !isCSSIdentChar(s[prev-4]))
			if !alreadyInVar {
				i = consumeCSSVarRef(s, i+2, &b)
				continue
			}
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func isCSSIdentStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b == '-'
}

func isCSSIdentChar(b byte) bool {
	return isCSSIdentStart(b) || (b >= '0' && b <= '9')
}

func splitTopLevelDelimiter(s string, sep byte, skipEmpty bool) []string {
	var parts []string
	last := 0
	inQuote := byte(0)
	parenDepth := 0
	bracketDepth := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inQuote = ch
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		default:
			if ch == sep && parenDepth == 0 && bracketDepth == 0 {
				trimmed := strings.TrimSpace(s[last:i])
				if !skipEmpty || trimmed != "" {
					parts = append(parts, trimmed)
				}
				last = i + 1
			}
		}
	}
	tail := strings.TrimSpace(s[last:])
	if !skipEmpty || tail != "" {
		parts = append(parts, tail)
	}
	return parts
}

func splitTopLevelCommas(s string) []string {
	return splitTopLevelDelimiter(s, ',', true)
}

func combineNestedSelectors(parentSel, childSel string) string {
	if parentSel == "" {
		return childSel
	}
	if childSel == "" {
		return parentSel
	}
	parents := splitTopLevelCommas(parentSel)
	children := splitTopLevelCommas(childSel)
	var out []string
	seen := make(map[string]bool)
	for _, p := range parents {
		for _, c := range children {
			var combined string
			if strings.Contains(c, "&") {
				combined = strings.ReplaceAll(c, "&", p)
			} else if strings.HasPrefix(c, ":") || strings.HasPrefix(c, "[") {
				combined = p + c
			} else {
				combined = p + " " + c
			}
			if !seen[combined] {
				seen[combined] = true
				out = append(out, combined)
			}
		}
	}
	return strings.Join(out, ", ")
}

func combineSwitchAndRuleSelector(branchSel, ruleSel string) string {
	if ruleSel == "" {
		return branchSel
	}
	if branchSel == "" {
		return ruleSel
	}
	branches := splitTopLevelCommas(branchSel)
	rules := splitTopLevelCommas(ruleSel)
	var out []string
	seen := make(map[string]bool)
	for _, b := range branches {
		bBase := b
		for i := 0; i < len(b); i++ {
			if b[i] == '[' || b[i] == ':' || b[i] == ' ' {
				bBase = b[:i]
				break
			}
		}
		for _, r := range rules {
			var combined string
			if strings.HasPrefix(r, "&") {
				combined = b + r[1:]
			} else if bBase != "" && r == bBase {
				combined = b
			} else if bBase != "" && strings.HasPrefix(r, bBase) && len(r) > len(bBase) && (r[len(bBase)] == ' ' || r[len(bBase)] == ':' || r[len(bBase)] == '.' || r[len(bBase)] == '#') {
				combined = b + r[len(bBase):]
			} else {
				combined = b + " " + r
			}
			if !seen[combined] {
				seen[combined] = true
				out = append(out, combined)
			}
		}
	}
	return strings.Join(out, ", ")
}
