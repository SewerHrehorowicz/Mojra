//go:build !(js && wasm)

package tpl

import "strings"

func trimCSSCodeLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") {
		return ""
	}
	return trimmed
}

func isCSSControlPrefix(s string) bool {
	return strings.HasPrefix(s, "@") || strings.HasPrefix(s, tssKwSwitchSpace) || strings.HasPrefix(s, tssArrowToken)
}

func preprocessCSSLines(source string) []cssPreprocessedLine {
	cleaned := replaceBlockCommentsPreservingNewlines(strings.ReplaceAll(source, "\r\n", "\n"))
	rawLines := strings.Split(cleaned, "\n")

	var out []cssPreprocessedLine
	i := 0
	for i < len(rawLines) {
		line := rawLines[i]
		lineNum := i + 1
		trimmed := trimCSSCodeLine(line)
		if trimmed == "" {
			i++
			continue
		}
		indent := countLeadingIndent(line)

		acc := trimmed
		for mergeStep := 0; mergeStep < len(rawLines); mergeStep++ {
			openParens := countUnquotedParens(acc)
			isSelectorComma := strings.HasSuffix(acc, ",") && openParens == 0 && !isCSSPropertyAssignment(acc) && findUnquotedArrow(acc) < 0 && !isBlockSwitchOrDefLine(acc)
			if openParens <= 0 && !isSelectorComma {
				break
			}
			if i+1 >= len(rawLines) {
				break
			}
			nextIdx := i + 1
			var nextTrimmed string
			for nextIdx < len(rawLines) {
				nextTrimmed = trimCSSCodeLine(rawLines[nextIdx])
				if nextTrimmed == "" {
					nextIdx++
					continue
				}
				break
			}
			if nextIdx >= len(rawLines) {
				break
			}
			if isSelectorComma && countLeadingIndent(rawLines[nextIdx]) != indent {
				break
			}
			acc += " " + nextTrimmed
			i = nextIdx
		}

		out = append(out, cssPreprocessedLine{
			Indent:  indent,
			Content: acc,
			LineNum: lineNum,
		})
		i++
	}
	return collapseMultilinePropertyBlocks(out)
}

func collapseMultilinePropertyBlocks(lines []cssPreprocessedLine) []cssPreprocessedLine {
	type blockCtx struct {
		indent       int
		isSwitchBody bool
	}
	var stack []blockCtx
	var out []cssPreprocessedLine

	i := 0
	for i < len(lines) {
		pl := lines[i]
		for len(stack) > 0 && stack[len(stack)-1].indent >= pl.Indent {
			stack = stack[:len(stack)-1]
		}
		inSwitchBody := len(stack) > 0 && stack[len(stack)-1].isSwitchBody
		hasChildren := i+1 < len(lines) && lines[i+1].Indent > pl.Indent

		if !hasChildren {
			out = append(out, pl)
			i++
			continue
		}

		endIdx := i + 1
		for endIdx < len(lines) && lines[endIdx].Indent > pl.Indent {
			endIdx++
		}
		subLines := lines[i+1 : endIdx]

		if isBlockSwitchOrDefLine(pl.Content) {
			out = append(out, pl)
			stack = append(stack, blockCtx{indent: pl.Indent, isSwitchBody: true})
			i++
			continue
		}

		if inSwitchBody {
			out = append(out, pl)
			stack = append(stack, blockCtx{indent: pl.Indent, isSwitchBody: false})
			i++
			continue
		}

		if propKey, isPropSw := parsePropertySwitchHeaderKey(pl.Content); isPropSw {
			out = append(out, pl)
			j := 0
			for j < len(subLines) {
				caseLine := subLines[j]
				caseEnd := j + 1
				for caseEnd < len(subLines) && subLines[caseEnd].Indent > caseLine.Indent {
					caseEnd++
				}
				if caseEnd > j+1 {
					caseKeySpec, inlineVal := splitSwitchCaseMultilineHeader(caseLine.Content)
					collapsedVal := evalMultilinePropertyValue(propKey, inlineVal, subLines[j+1:caseEnd])
					out = append(out, cssPreprocessedLine{
						Indent:  caseLine.Indent,
						Content: caseKeySpec + ": " + collapsedVal,
						LineNum: caseLine.LineNum,
					})
				} else {
					out = append(out, caseLine)
				}
				j = caseEnd
			}
			i = endIdx
			continue
		}

		if propKey, inlineVal, ok := matchMultilinePropertyHeader(pl.Content, pl.Indent, subLines[0].Content); ok {
			collapsedVal := evalMultilinePropertyValue(propKey, inlineVal, subLines)
			out = append(out, cssPreprocessedLine{
				Indent:  pl.Indent,
				Content: propKey + ": " + collapsedVal,
				LineNum: pl.LineNum,
			})
			i = endIdx
			continue
		}

		out = append(out, pl)
		stack = append(stack, blockCtx{indent: pl.Indent, isSwitchBody: false})
		i++
	}
	return out
}

func isBlockSwitchOrDefLine(content string) bool {
	if strings.HasPrefix(content, tssKwAtSwitchSpace) || strings.HasPrefix(content, tssKwAtSwitchParen) || strings.HasPrefix(content, tssKwSwitchSpace) {
		return true
	}
	if lhs, rhs, hasArrow := splitArrowSides(content); hasArrow && lhs == "" {
		if _, ok := parseSwitchTargetName(rhs); ok {
			return true
		}
	}
	return false
}

func parsePropertySwitchHeaderKey(content string) (string, bool) {
	if lhs, rhs, hasArrow := splitArrowSides(content); hasArrow {
		if lhs != "" && cssPropKeyRe.MatchString(lhs) {
			if _, inlineExpr, ok := parseInlineSwitchTarget(rhs); ok && inlineExpr == "" {
				return lhs, true
			}
		}
		return "", false
	}
	if hdr, ok := parseCSSPropHeader(content); ok {
		if _, inlineExpr, isSw := parseInlineSwitchTarget(hdr.Val); isSw && inlineExpr == "" {
			return hdr.Key, true
		}
	}
	return "", false
}

func parsePropSwitchCaseLine(content string) ([]string, string, string, bool) {
	trimmed := strings.TrimSpace(content)
	if colonIdx := findFirstUnquotedColon(trimmed); colonIdx > 0 {
		candidateKeys := splitTopLevelCommas(trimmed[:colonIdx])
		valid := len(candidateKeys) > 0
		for _, ck := range candidateKeys {
			if !cssCaseKeyIdentRe.MatchString(ck) {
				valid = false
				break
			}
		}
		if valid {
			return candidateKeys, strings.TrimSpace(trimmed[:colonIdx]), trimCSSTrailingToken(trimmed[colonIdx+1:], ";"), true
		}
	}
	if m := cssPropSwitchCaseLineRe.FindStringSubmatch(trimmed); len(m) == 3 {
		keySpec := strings.TrimSpace(m[1])
		return splitTopLevelCommas(keySpec), keySpec, trimCSSTrailingToken(m[2], ";"), true
	}
	return nil, "", "", false
}

func splitSwitchCaseMultilineHeader(content string) (caseKeySpec string, inlineVal string) {
	trimmed := strings.TrimSpace(content)
	if strings.HasSuffix(trimmed, ":") {
		return trimCSSTrailingToken(trimmed, ":"), ""
	}
	if _, keySpec, val, ok := parsePropSwitchCaseLine(trimmed); ok {
		return keySpec, val
	}
	return trimmed, ""
}

func matchMultilinePropertyHeader(content string, indent int, firstChild string) (propKey string, inlineVal string, ok bool) {
	if isCSSControlPrefix(content) || isCSSControlPrefix(firstChild) {
		return "", "", false
	}

	trimmed := strings.TrimSpace(content)
	hasExplicitColon := strings.HasSuffix(trimmed, ":")
	bareKey := trimCSSTrailingToken(trimmed, ":")
	if cssPropKeyRe.MatchString(bareKey) && cssPseudoShorthands[bareKey] == "" {
		if isCustomCSSPropKey(bareKey) || (indent > 0 && (multilineCSSProperties[bareKey] || hasExplicitColon)) {
			return bareKey, "", true
		}
	}

	if hdr, isProp := parseCSSPropHeader(trimmed); isProp && cssPseudoShorthands[hdr.Key] == "" {
		if _, _, isSw := parseInlineSwitchTarget(hdr.Val); isSw {
			return "", "", false
		}
		if isCustomCSSPropKey(hdr.Key) || (indent > 0 && (multilineCSSProperties[hdr.Key] || hdr.HasColon)) {
			val := strings.TrimSpace(hdr.Val)
			openIdx := strings.IndexByte(val, '(')
			fnName := val
			if openIdx > 0 {
				fnName = strings.TrimSpace(val[:openIdx])
			}
			if cssNestableFunctionNames[val] || (openIdx > 0 && cssNestableFunctionNames[fnName]) {
				return hdr.Key, hdr.Val, true
			}
		}
	}

	return "", "", false
}

func evalMultilinePropertyValue(propKey, headerVal string, sub []cssPreprocessedLine) string {
	items := evalMultilineSubItems(propKey, sub)
	if headerVal != "" {
		return spliceCSSFunctionArgs(headerVal, items)
	}
	return joinCSSPropertyItems(propKey, items)
}

func evalMultilineSubItems(propKey string, sub []cssPreprocessedLine) []string {
	if len(sub) == 0 {
		return nil
	}
	work := make([]cssPreprocessedLine, len(sub))
	for i := range sub {
		cleaned := trimCSSTrailingToken(trimCSSTrailingToken(sub[i].Content, ";"), ",")
		if propKey == cssTransitionProp && isCSSPropertyAssignment(cleaned) {
			if colonIdx := findFirstUnquotedColon(cleaned); colonIdx > 0 {
				cleaned = strings.TrimSpace(cleaned[:colonIdx]) + " " + strings.TrimSpace(cleaned[colonIdx+1:])
			}
		}
		work[i] = cssPreprocessedLine{
			Indent:  sub[i].Indent,
			Content: cleaned,
			LineNum: sub[i].LineNum,
		}
	}
	for pass := 0; pass < len(sub); pass++ {
		lastParent := -1
		for i := 0; i+1 < len(work); i++ {
			if work[i+1].Indent > work[i].Indent {
				lastParent = i
			}
		}
		if lastParent < 0 {
			break
		}
		childEnd := lastParent + 1
		for childEnd < len(work) && work[childEnd].Indent > work[lastParent].Indent {
			childEnd++
		}
		var nestedItems []string
		for k := lastParent + 1; k < childEnd; k++ {
			if work[k].Content != "" {
				nestedItems = append(nestedItems, work[k].Content)
			}
		}
		work[lastParent].Content = spliceCSSFunctionArgs(work[lastParent].Content, nestedItems)
		work = append(work[:lastParent+1], work[childEnd:]...)
	}
	var items []string
	for _, w := range work {
		if w.Content != "" {
			items = append(items, w.Content)
		}
	}
	return items
}

func spliceCSSFunctionArgs(header string, args []string) string {
	trimmed := strings.TrimSpace(header)
	openIdx := findFirstUnquotedOpenParen(trimmed)
	if openIdx < 0 {
		sep := ", "
		if cssSpaceSeparatedFuncs[trimmed] {
			sep = " "
		}
		return trimmed + "(" + strings.Join(args, sep) + ")"
	}
	closeIdx := findMatchingCloseParen(trimmed, openIdx)
	if closeIdx < 0 {
		closeIdx = len(trimmed)
	}
	fnName := strings.TrimSpace(trimmed[:openIdx])
	sep := ", "
	if cssSpaceSeparatedFuncs[fnName] {
		sep = " "
	}
	existing := trimCSSTrailingToken(trimmed[openIdx+1:closeIdx], ",")
	joined := strings.Join(args, sep)
	var inner string
	if existing != "" && joined != "" {
		inner = existing + sep + joined
	} else if existing != "" {
		inner = existing
	} else {
		inner = joined
	}
	if closeIdx >= len(trimmed) {
		return trimmed[:openIdx+1] + inner + ")"
	}
	return trimmed[:openIdx+1] + inner + trimmed[closeIdx:]
}

func findFirstUnquotedOpenParen(s string) int {
	inQuote := byte(0)
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
		if ch == '"' || ch == '\'' {
			inQuote = ch
			continue
		}
		if ch == '(' {
			return i
		}
	}
	return -1
}

func findMatchingCloseParen(s string, openIdx int) int {
	depth := 0
	inQuote := byte(0)
	for i := openIdx; i < len(s); i++ {
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
		if ch == '"' || ch == '\'' {
			inQuote = ch
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func joinCSSPropertyItems(propKey string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	if spaceJoinedCSSProperties[propKey] {
		return strings.Join(items, " ")
	}
	if propKey == cssBackgroundProp || propKey == cssMaskProp || propKey == cssWebkitMaskProp || isCustomCSSPropKey(propKey) {
		var layers []string
		for _, item := range items {
			if len(layers) > 0 && isCSSImageLayerExpr(layers[len(layers)-1]) && isCSSBackgroundLayerModifier(item) {
				layers[len(layers)-1] += " " + item
				continue
			}
			layers = append(layers, item)
		}
		return strings.Join(layers, ", ")
	}
	return strings.Join(items, ", ")
}

func isCSSImageLayerExpr(s string) bool {
	for _, tok := range cssImageLayerTokens {
		if strings.Contains(s, tok) {
			return true
		}
	}
	return false
}

func isCSSBackgroundLayerModifier(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || isCSSImageLayerExpr(trimmed) {
		return false
	}
	if strings.Contains(trimmed, "/") {
		return true
	}
	firstWord := trimmed
	if sp := strings.IndexAny(trimmed, " \t"); sp > 0 {
		firstWord = trimmed[:sp]
	}
	if (firstWord[0] >= '0' && firstWord[0] <= '9') || (len(firstWord) > 1 && firstWord[0] == '-' && firstWord[1] >= '0' && firstWord[1] <= '9') {
		return true
	}
	return cssBackgroundModifierKeywords[firstWord]
}

func replaceBlockCommentsPreservingNewlines(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	inQuotes := byte(0)
	for i < len(s) {
		if inQuotes != 0 {
			if s[i] == '\\' && i+1 < len(s) {
				b.WriteByte(s[i])
				b.WriteByte(s[i+1])
				i += 2
				continue
			}
			if s[i] == inQuotes {
				inQuotes = 0
			}
			b.WriteByte(s[i])
			i++
			continue
		}
		if s[i] == '"' || s[i] == '\'' {
			inQuotes = s[i]
			b.WriteByte(s[i])
			i++
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i < len(s) {
				if i+1 < len(s) && s[i] == '*' && s[i+1] == '/' {
					i += 2
					break
				}
				if s[i] == '\n' {
					b.WriteByte('\n')
				}
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func countUnquotedParens(s string) int {
	depth := 0
	inQuote := byte(0)
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
		if ch == '"' || ch == '\'' {
			inQuote = ch
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' && depth > 0 {
			depth--
		}
	}
	return depth
}
