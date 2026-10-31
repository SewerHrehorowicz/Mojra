package tpl

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	cssPropKeyRe            = regexp.MustCompile(`^(?:\$|--)?-?[a-zA-Z_][a-zA-Z0-9_-]*$`)
	cssSwitchTargetRe       = regexp.MustCompile(`^switch\(\s*([a-zA-Z_][a-zA-Z0-9_-]*)\s*\)$`)
	cssInlineSwitchTargetRe = regexp.MustCompile(`^switch\(\s*([a-zA-Z_][a-zA-Z0-9_-]*)(?:\s*[,:\s]\s*(.+?))?\s*\)(?:\s*:?\s*(.+))?$`)
	cssSwitchDefCaseLineRe  = regexp.MustCompile(`^([a-zA-Z0-9_-]+)(?::\s*|\s+)(.+)$`)
	cssPropSwitchCaseLineRe = regexp.MustCompile(`^([a-zA-Z0-9_-]+(?:\s*,\s*[a-zA-Z0-9_-]+)*)(?:\s*:\s*|\s+)(.+)$`)
	cssCaseKeyIdentRe       = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// CSSSwitchDef holds a named CSS switch mapping case names to CSS selectors.
type CSSSwitchDef struct {
	Name  string
	Order []string
	Cases map[string]string
}

type cssNodeKind int

const (
	cssKindRoot cssNodeKind = iota
	cssKindRule
	cssKindSwitchDef
	cssKindPropSwitch
	cssKindBlockSwitch
	cssKindSwitchCase
	cssKindCond
)

type cssDecl struct {
	Key  string
	Val  string
	Line int
}

type cssSwitchCaseVal struct {
	CaseKeys []string
	Val      string
	Line     int
}

type cssNode struct {
	Kind       cssNodeKind
	Selector   string
	PropKey    string
	SwitchName string
	CaseKeys   []string
	Cond       string
	Decls      []cssDecl
	PropCases  []cssSwitchCaseVal
	Children   []*cssNode
	Indent     int
	Line       int
	File       string
}

// DefaultCSSSwitches returns a new switch registry pre-populated with default theme and preview switches.
func DefaultCSSSwitches() map[string]*CSSSwitchDef {
	return map[string]*CSSSwitchDef{
		"theme": {
			Name:  "theme",
			Order: []string{"dark", "light"},
			Cases: map[string]string{
				"dark":  `:root, body[data-theme="dark"]`,
				"light": `body[data-theme="light"]`,
			},
		},
		"preview": {
			Name:  "preview",
			Order: []string{"full", "portrait", "landscape"},
			Cases: map[string]string{
				"full":      `.device-stage[data-preview="full"]`,
				"portrait":  `.device-stage[data-preview="portrait"]`,
				"landscape": `.device-stage[data-preview="landscape"]`,
			},
		},
	}
}

// TranspileCSSFilesToString reads, transpiles, and bundles multiple CSS template files sharing switch definitions.
func TranspileCSSFilesToString(cssFiles []string, stripDev bool, ctx *RenderContext) (string, error) {
	switches := DefaultCSSSwitches()
	var sb strings.Builder
	for _, path := range cssFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		src := string(raw)
		if stripDev {
			src = StripDevOnlyBlocks(src)
		}
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".tss"), ".css")
		rendered, err := transpileCSSWithSwitches(src, name, ctx, switches, stripDev)
		if err != nil {
			return "", err
		}
		rendered = strings.TrimSpace(rendered)
		if rendered == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(rendered)
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// TranspileCSSString transpiles a single CSS template string into standard CSS.
func TranspileCSSString(source, name string, ctx *RenderContext) (string, error) {
	switches := DefaultCSSSwitches()
	return transpileCSSWithSwitches(source, name, ctx, switches, false)
}

type cssPreprocessedLine struct {
	Indent  int
	Content string
	LineNum int
}

var inlineBlockCommentRe = regexp.MustCompile(`/\*.*?\*/`)

func preprocessCSSLines(source string) []cssPreprocessedLine {
	// Remove multi-line /* ... */ comments while preserving line count
	cleaned := replaceBlockCommentsPreservingNewlines(strings.ReplaceAll(source, "\r\n", "\n"))
	rawLines := strings.Split(cleaned, "\n")

	var out []cssPreprocessedLine
	i := 0
	for i < len(rawLines) {
		line := rawLines[i]
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			i++
			continue
		}
		indent := countLeadingIndent(line)

		// Join multi-line parenthesized values or comma-Lists
		acc := trimmed
		for {
			openParens := countUnquotedParens(acc)
			isSelectorComma := strings.HasSuffix(acc, ",") && openParens == 0 && !isCSSPropertyAssignment(acc) && findUnquotedArrow(acc) < 0 && !strings.HasPrefix(acc, "switch ") && !strings.HasPrefix(acc, "@switch ")
			if openParens <= 0 && !isSelectorComma {
				break
			}
			if i+1 >= len(rawLines) {
				break
			}
			nextIdx := i + 1
			for nextIdx < len(rawLines) {
				nt := strings.TrimSpace(rawLines[nextIdx])
				if nt == "" || strings.HasPrefix(nt, "//") {
					nextIdx++
					continue
				}
				break
			}
			if nextIdx >= len(rawLines) {
				break
			}
			nextTrimmed := strings.TrimSpace(rawLines[nextIdx])
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
	return out
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

func isCSSPropertyAssignment(content string) bool {
	if strings.HasPrefix(content, "->") || strings.HasPrefix(content, "switch ") || strings.HasPrefix(content, "@") {
		return false
	}
	colonIdx := findFirstUnquotedColon(content)
	if colonIdx <= 0 {
		return false
	}
	key := strings.TrimSpace(content[:colonIdx])
	if !cssPropKeyRe.MatchString(key) {
		return false
	}
	if strings.HasPrefix(key, "$") || strings.HasPrefix(key, "--") {
		return true
	}
	if colonIdx+1 < len(content) && (content[colonIdx+1] == ' ' || content[colonIdx+1] == '\t') {
		return true
	}
	return false
}

func findFirstUnquotedColon(s string) int {
	inQuote := byte(0)
	bracketDepth := 0
	parenDepth := 0
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
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case ':':
			if bracketDepth == 0 && parenDepth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseSwitchTargetName(expr string) (string, bool) {
	trimmed := strings.TrimSpace(expr)
	if m := cssSwitchTargetRe.FindStringSubmatch(trimmed); len(m) == 2 {
		return m[1], true
	}
	if strings.HasPrefix(trimmed, "switch ") {
		name := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(trimmed, "switch ")), ":")
		if cssCaseKeyIdentRe.MatchString(name) {
			return name, true
		}
	}
	return "", false
}

func parseInlineSwitchTarget(expr string) (string, string, bool) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(expr), ";")
	if !strings.HasPrefix(trimmed, "switch(") {
		return "", "", false
	}
	depth := 0
	inQuote := byte(0)
	closeIdx := -1
	for i := 6; i < len(trimmed); i++ {
		ch := trimmed[i]
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
				closeIdx = i
				break
			}
		}
	}
	if closeIdx < 0 {
		return "", "", false
	}
	inside := strings.TrimSpace(trimmed[7:closeIdx])
	after := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(trimmed[closeIdx+1:]), ":"))
	if inside == "" || !isCSSIdentStart(inside[0]) {
		return "", "", false
	}
	j := 0
	for j < len(inside) && isCSSIdentChar(inside[j]) {
		j++
	}
	swName := inside[:j]
	insideRest := strings.TrimSpace(inside[j:])
	if strings.HasPrefix(insideRest, ",") || strings.HasPrefix(insideRest, ":") {
		insideRest = strings.TrimSpace(insideRest[1:])
	}
	if insideRest != "" && after != "" {
		return "", "", false
	}
	inlineExpr := insideRest
	if inlineExpr == "" {
		inlineExpr = after
	}
	return swName, inlineExpr, true
}

func splitCSSPropertyHeader(content string) (key string, val string, hasColon bool, ok bool) {
	if strings.HasPrefix(content, "->") || strings.HasPrefix(content, "switch ") || strings.HasPrefix(content, "@") {
		return "", "", false, false
	}
	if isCSSPropertyAssignment(content) {
		colonIdx := findFirstUnquotedColon(content)
		key = strings.TrimSpace(content[:colonIdx])
		val = strings.TrimSuffix(strings.TrimSpace(content[colonIdx+1:]), ";")
		return key, val, true, true
	}
	spaceIdx := strings.IndexAny(content, " \t")
	if spaceIdx <= 0 {
		return "", "", false, false
	}
	key = strings.TrimSpace(content[:spaceIdx])
	val = strings.TrimSuffix(strings.TrimSpace(content[spaceIdx+1:]), ";")
	if !cssPropKeyRe.MatchString(key) || val == "" {
		return "", "", false, false
	}
	return key, val, false, true
}

func parseInlineSwitchCases(inlineExpr string, swDef *CSSSwitchDef, name, swName string, lineNum int) ([]cssSwitchCaseVal, error) {
	var parts []string
	if strings.Contains(inlineExpr, "|") {
		parts = splitTopLevelPipes(inlineExpr)
	} else if colonIdx := findFirstUnquotedColon(inlineExpr); colonIdx > 0 {
		rawKeys := splitTopLevelCommas(inlineExpr[:colonIdx])
		allValidKeys := len(rawKeys) > 0
		for _, rk := range rawKeys {
			if !cssCaseKeyIdentRe.MatchString(rk) {
				allValidKeys = false
				break
			}
		}
		if allValidKeys {
			val := strings.TrimSuffix(strings.TrimSpace(inlineExpr[colonIdx+1:]), ";")
			return []cssSwitchCaseVal{{
				CaseKeys: rawKeys,
				Val:      val,
				Line:     lineNum,
			}}, nil
		}
		parts = splitTopLevelCommas(inlineExpr)
	} else {
		parts = splitTopLevelCommas(inlineExpr)
	}

	var out []cssSwitchCaseVal
	for idx, seg := range parts {
		if colonIdx := findFirstUnquotedColon(seg); colonIdx > 0 {
			rawKeys := splitTopLevelCommas(seg[:colonIdx])
			val := strings.TrimSuffix(strings.TrimSpace(seg[colonIdx+1:]), ";")
			out = append(out, cssSwitchCaseVal{
				CaseKeys: rawKeys,
				Val:      val,
				Line:     lineNum,
			})
			continue
		}
		val := strings.TrimSuffix(strings.TrimSpace(seg), ";")
		if val == "" || val == "_" {
			continue
		}
		if idx >= len(swDef.Order) {
			return nil, fmt.Errorf("css compile error in %s.css:%d: too many inline values (%d) for switch(%s) which has %d cases", name, lineNum, len(parts), swName, len(swDef.Order))
		}
		out = append(out, cssSwitchCaseVal{
			CaseKeys: []string{swDef.Order[idx]},
			Val:      val,
			Line:     lineNum,
		})
	}
	return out, nil
}

func transpileCSSWithSwitches(source, name string, ctx *RenderContext, switches map[string]*CSSSwitchDef, stripDev bool) (string, error) {
	lines := preprocessCSSLines(source)
	root := &cssNode{Kind: cssKindRoot, Indent: -1, File: name}
	stack := []*cssNode{root}

	for lineIdx, pl := range lines {
		indent := pl.Indent
		content := pl.Content
		lineNum := pl.LineNum
		hasChildren := lineIdx+1 < len(lines) && lines[lineIdx+1].Indent > indent

		for len(stack) > 1 && stack[len(stack)-1].Indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]

		// 1. Direct children of `switch <name>` / `@switch <name>` definition
		if parent.Kind == cssKindSwitchDef {
			m := cssSwitchDefCaseLineRe.FindStringSubmatch(content)
			if len(m) != 3 {
				return "", fmt.Errorf("css compile error in %s.css:%d: switch case definition %q must be 'case selector' or 'case: selector'", name, lineNum, content)
			}
			caseKey := strings.TrimSpace(m[1])
			selVal := strings.TrimSpace(m[2])
			def := switches[parent.SwitchName]
			if _, exists := def.Cases[caseKey]; !exists {
				def.Order = append(def.Order, caseKey)
			}
			def.Cases[caseKey] = selVal
			continue
		}

		// 2. Direct children of `prop switch(name)` / `prop -> switch(name)`
		if parent.Kind == cssKindPropSwitch {
			var rawKeys []string
			var val string
			if colonIdx := findFirstUnquotedColon(content); colonIdx > 0 {
				candidateKeys := splitTopLevelCommas(content[:colonIdx])
				validKeys := len(candidateKeys) > 0
				for _, ck := range candidateKeys {
					if !cssCaseKeyIdentRe.MatchString(ck) {
						validKeys = false
						break
					}
				}
				if validKeys {
					rawKeys = candidateKeys
					val = strings.TrimSuffix(strings.TrimSpace(content[colonIdx+1:]), ";")
				}
			}
			if len(rawKeys) == 0 {
				m := cssPropSwitchCaseLineRe.FindStringSubmatch(content)
				if len(m) != 3 {
					return "", fmt.Errorf("css compile error in %s.css:%d: property switch case %q must be 'case value' or 'case: value'", name, lineNum, content)
				}
				rawKeys = splitTopLevelCommas(m[1])
				val = strings.TrimSuffix(strings.TrimSpace(m[2]), ";")
			}
			parent.PropCases = append(parent.PropCases, cssSwitchCaseVal{
				CaseKeys: rawKeys,
				Val:      val,
				Line:     lineNum,
			})
			continue
		}

		// 3. Direct children of block-level `@switch <name>` / `-> switch(name)`
		if parent.Kind == cssKindBlockSwitch {
			caseSpec := strings.TrimSuffix(strings.TrimSpace(content), ":")
			caseKeys := splitTopLevelCommas(caseSpec)
			caseNode := &cssNode{
				Kind:     cssKindSwitchCase,
				CaseKeys: caseKeys,
				Indent:   indent,
				Line:     lineNum,
				File:     name,
			}
			parent.Children = append(parent.Children, caseNode)
			stack = append(stack, caseNode)
			continue
		}

		// 4. `@if <cond>` conditional block
		if strings.HasPrefix(content, "@if ") {
			condNode := &cssNode{
				Kind:   cssKindCond,
				Cond:   strings.TrimSpace(content[4:]),
				Indent: indent,
				Line:   lineNum,
				File:   name,
			}
			parent.Children = append(parent.Children, condNode)
			stack = append(stack, condNode)
			continue
		}

		// 5. `@switch` or `switch` block / definition
		if strings.HasPrefix(content, "@switch ") || strings.HasPrefix(content, "@switch(") || strings.HasPrefix(content, "switch ") {
			isAtSwitch := strings.HasPrefix(content, "@")
			rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(content, "@"), "switch"))
			if strings.HasPrefix(rest, "(") && strings.HasSuffix(rest, ")") {
				rest = strings.TrimSpace(rest[1 : len(rest)-1])
			}

			// Determine if this is a block switch vs switch definition
			childHasGrandchildren := hasChildren && lineIdx+2 < len(lines) && lines[lineIdx+2].Indent > lines[lineIdx+1].Indent
			if isAtSwitch && (parent.Kind != cssKindRoot || childHasGrandchildren) {
				swName := strings.TrimSuffix(rest, ":")
				swNode := &cssNode{
					Kind:       cssKindBlockSwitch,
					SwitchName: swName,
					Indent:     indent,
					Line:       lineNum,
					File:       name,
				}
				parent.Children = append(parent.Children, swNode)
				stack = append(stack, swNode)
				continue
			}

			if colonIdx := findFirstUnquotedColon(rest); colonIdx > 0 {
				swName := strings.TrimSpace(rest[:colonIdx])
				branches := strings.Split(rest[colonIdx+1:], "|")
				def := &CSSSwitchDef{
					Name:  swName,
					Cases: make(map[string]string),
				}
				for idx, br := range branches {
					br = strings.TrimSpace(br)
					if br == "" {
						continue
					}
					key := strconv.Itoa(idx)
					def.Order = append(def.Order, key)
					def.Cases[key] = br
				}
				switches[swName] = def
				continue
			}
			swName := strings.TrimSuffix(rest, ":")
			switches[swName] = &CSSSwitchDef{
				Name:  swName,
				Cases: make(map[string]string),
			}
			swNode := &cssNode{
				Kind:       cssKindSwitchDef,
				SwitchName: swName,
				Indent:     indent,
				Line:       lineNum,
				File:       name,
			}
			stack = append(stack, swNode)
			continue
		}

		// 6. Control-flow lines containing `->`
		if arrowIdx := findUnquotedArrow(content); arrowIdx >= 0 {
			lhs := strings.TrimSpace(content[:arrowIdx])
			rhs := strings.TrimSpace(content[arrowIdx+2:])

			if lhs == "" {
				// Block-level control structure: `-> switch(name)` or `-> if ...`
				if swName, ok := parseSwitchTargetName(rhs); ok {
					swNode := &cssNode{
						Kind:       cssKindBlockSwitch,
						SwitchName: swName,
						Indent:     indent,
						Line:       lineNum,
						File:       name,
					}
					parent.Children = append(parent.Children, swNode)
					stack = append(stack, swNode)
					continue
				}
				if strings.HasPrefix(rhs, "if ") {
					condNode := &cssNode{
						Kind:   cssKindCond,
						Cond:   strings.TrimSpace(rhs[3:]),
						Indent: indent,
						Line:   lineNum,
						File:   name,
					}
					parent.Children = append(parent.Children, condNode)
					stack = append(stack, condNode)
					continue
				}
				return "", fmt.Errorf("css compile error in %s.css:%d: unsupported control flow %q", name, lineNum, content)
			}

			// Property-level switch: `prop -> switch(name)` or `$var -> switch(name) val1 | val2`
			if swName, inlineExpr, ok := parseInlineSwitchTarget(rhs); ok {
				if !cssPropKeyRe.MatchString(lhs) {
					return "", fmt.Errorf("css compile error in %s.css:%d: invalid property name %q before -> switch(%s)", name, lineNum, lhs, swName)
				}
				psNode := &cssNode{
					Kind:       cssKindPropSwitch,
					PropKey:    lhs,
					SwitchName: swName,
					Indent:     indent,
					Line:       lineNum,
					File:       name,
				}
				if inlineExpr != "" {
					swDef, exists := switches[swName]
					if !exists {
						return "", fmt.Errorf("css compile error in %s.css:%d: unknown switch %q", name, lineNum, swName)
					}
					cases, err := parseInlineSwitchCases(inlineExpr, swDef, name, swName, lineNum)
					if err != nil {
						return "", err
					}
					psNode.PropCases = append(psNode.PropCases, cases...)
					parent.Children = append(parent.Children, psNode)
					continue
				}
				parent.Children = append(parent.Children, psNode)
				stack = append(stack, psNode)
				continue
			}

			return "", fmt.Errorf("css compile error in %s.css:%d: '->' in CSS is reserved for control structures like '-> switch(...)'; use '%s: {%s}' or '%s: $%s' instead", name, lineNum, lhs, rhs, lhs, rhs)
		}

		// 7. Bare quoted string inside ::before / ::after (sets `content: "..."`)
		if strings.HasPrefix(content, `"`) && strings.HasSuffix(content, `"`) && len(content) >= 2 {
			if parent.Kind == cssKindRoot {
				return "", fmt.Errorf("css compile error in %s.css:%d: bare string %s must be inside a pseudo-element block", name, lineNum, content)
			}
			parent.Decls = append(parent.Decls, cssDecl{
				Key:  "content",
				Val:  content,
				Line: lineNum,
			})
			continue
		}

		// 8. Property declaration (`key: value`, `key value`, or `key switch(...)`)
		if key, val, hasColon, ok := splitCSSPropertyHeader(content); ok {
			if swName, inlineExpr, isSw := parseInlineSwitchTarget(val); isSw {
				psNode := &cssNode{
					Kind:       cssKindPropSwitch,
					PropKey:    key,
					SwitchName: swName,
					Indent:     indent,
					Line:       lineNum,
					File:       name,
				}
				if inlineExpr != "" {
					swDef, exists := switches[swName]
					if !exists {
						return "", fmt.Errorf("css compile error in %s.css:%d: unknown switch %q", name, lineNum, swName)
					}
					cases, err := parseInlineSwitchCases(inlineExpr, swDef, name, swName, lineNum)
					if err != nil {
						return "", err
					}
					psNode.PropCases = append(psNode.PropCases, cases...)
					parent.Children = append(parent.Children, psNode)
					continue
				}
				parent.Children = append(parent.Children, psNode)
				stack = append(stack, psNode)
				continue
			}

			if hasColon || !hasChildren {
				if parent.Kind == cssKindRoot && !strings.HasPrefix(key, "$") && !strings.HasPrefix(key, "--") {
					return "", fmt.Errorf("css compile error in %s.css:%d: property %q must be indented under a selector", name, lineNum, content)
				}

				if hasColon && (key == "before" || key == "after") {
					pseudoNode := &cssNode{
						Kind:     cssKindRule,
						Selector: "::" + key,
						Decls: []cssDecl{
							{Key: "content", Val: val, Line: lineNum},
						},
						Indent: indent,
						Line:   lineNum,
						File:   name,
					}
					parent.Children = append(parent.Children, pseudoNode)
					stack = append(stack, pseudoNode)
					continue
				}

				parent.Decls = append(parent.Decls, cssDecl{
					Key:  key,
					Val:  val,
					Line: lineNum,
				})
				continue
			}
		}

		// 8. Selector / @media block header
		sel := strings.TrimSpace(content)
		if sel == "before" || sel == "after" {
			sel = "::" + sel
		}
		ruleNode := &cssNode{
			Kind:     cssKindRule,
			Selector: sel,
			Indent:   indent,
			Line:     lineNum,
			File:     name,
		}
		parent.Children = append(parent.Children, ruleNode)
		stack = append(stack, ruleNode)
	}

	blocks, err := emitCSSNode(root, "", switches, ctx, stripDev)
	if err != nil {
		return "", err
	}
	return strings.Join(blocks, "\n\n"), nil
}

type cssSwitchedBucket struct {
	Selector string
	Decls    []string
}

func emitCSSNode(node *cssNode, currentSelector string, switches map[string]*CSSSwitchDef, ctx *RenderContext, stripDev bool) ([]string, error) {
	var out []string

	// Evaluate conditional if this is a `-> if ...` node
	if node.Kind == cssKindCond {
		cond := node.Cond
		invert := false
		if strings.HasPrefix(cond, "not ") {
			invert = true
			cond = strings.TrimSpace(cond[4:])
		}
		if cond == "dev_mode" && stripDev {
			if !invert {
				return nil, nil
			}
		} else if ctx != nil {
			val, ok := resolvePath(ctx, cond)
			truthy := ok && isTruthy(val)
			if invert {
				truthy = !truthy
			}
			if !truthy {
				return nil, nil
			}
		}
	}

	// Handle `@media` / `@supports` / `@font-face` rules
	if node.Kind == cssKindRule && strings.HasPrefix(node.Selector, "@") {
		if len(node.Decls) > 0 && len(node.Children) == 0 && currentSelector == "" {
			decls := formatCSSDecls(node.Decls, ctx, "  ")
			out = append(out, fmt.Sprintf("%s {\n%s\n}", node.Selector, strings.Join(decls, "\n")))
			return out, nil
		}
		bodyNode := *node
		bodyNode.Kind = cssKindRoot
		bodyNode.Selector = ""
		chBlocks, err := emitCSSNode(&bodyNode, currentSelector, switches, ctx, stripDev)
		if err != nil {
			return nil, err
		}
		var innerBlocks []string
		for _, b := range chBlocks {
			indented := "  " + strings.ReplaceAll(b, "\n", "\n  ")
			innerBlocks = append(innerBlocks, indented)
		}
		if len(innerBlocks) > 0 {
			out = append(out, fmt.Sprintf("%s {\n%s\n}", node.Selector, strings.Join(innerBlocks, "\n\n")))
		}
		return out, nil
	}

	// Collect direct declarations and property-level switches on this node
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
				return nil, fmt.Errorf("css compile error in %s.css:%d: unknown switch %q", ch.File, ch.Line, ch.SwitchName)
			}
			propName := normalizeCSSPropKey(ch.PropKey)
			for _, pc := range ch.PropCases {
				var branchSelectors []string
				for _, ck := range pc.CaseKeys {
					bs, exists := swDef.Cases[ck]
					if !exists {
						return nil, fmt.Errorf("css compile error in %s.css:%d: unknown case %q in switch(%s)", ch.File, pc.Line, ck, ch.SwitchName)
					}
					branchSelectors = append(branchSelectors, bs)
				}
				combinedBranch := strings.Join(branchSelectors, ", ")
				targetSel := combineSwitchAndRuleSelector(combinedBranch, currentSelector)
				val := transformCSSValue(pc.Val, ctx)

				if ch.PropKey == "before" || ch.PropKey == "after" {
					pseudoSel := combineNestedSelectors(targetSel, "::"+ch.PropKey)
					beforeAfterBuckets = append(beforeAfterBuckets, &cssSwitchedBucket{
						Selector: pseudoSel,
						Decls:    []string{fmt.Sprintf("  content: %s;", val)},
					})
				} else {
					addSwitchedDecl(targetSel, fmt.Sprintf("  %s: %s;", propName, val))
				}
			}
		}
	}

	// Emit base declarations for this node
	if len(baseDecls) > 0 {
		if currentSelector == "" {
			// Top-level `$var` declarations: if there is a switched `:root` bucket, merge into it; otherwise emit `:root`
			mergedIntoRoot := false
			for _, sel := range switchedOrder {
				if strings.Contains(sel, ":root") {
					switchedBuckets[sel].Decls = append(switchedBuckets[sel].Decls, baseDecls...)
					mergedIntoRoot = true
					break
				}
			}
			if !mergedIntoRoot {
				out = append(out, formatCSSBlock(":root", baseDecls))
			}
		} else {
			out = append(out, formatCSSBlock(currentSelector, baseDecls))
		}
	}

	// Emit grouped property-level switch buckets in order
	for _, sel := range switchedOrder {
		bucket := switchedBuckets[sel]
		if len(bucket.Decls) > 0 {
			out = append(out, formatCSSBlock(bucket.Selector, bucket.Decls))
		}
	}
	for _, ba := range beforeAfterBuckets {
		out = append(out, formatCSSBlock(ba.Selector, ba.Decls))
	}

	// Process remaining children (nested rules, block-level switches, conditionals)
	for _, ch := range node.Children {
		switch ch.Kind {
		case cssKindPropSwitch:
			continue
		case cssKindRule:
			if strings.HasPrefix(ch.Selector, "@") {
				chBlocks, err := emitCSSNode(ch, currentSelector, switches, ctx, stripDev)
				if err != nil {
					return nil, err
				}
				out = append(out, chBlocks...)
				continue
			}
			nextSel := combineNestedSelectors(currentSelector, ch.Selector)
			chBlocks, err := emitCSSNode(ch, nextSel, switches, ctx, stripDev)
			if err != nil {
				return nil, err
			}
			out = append(out, chBlocks...)

		case cssKindBlockSwitch:
			swDef, ok := switches[ch.SwitchName]
			if !ok {
				return nil, fmt.Errorf("css compile error in %s.css:%d: unknown switch %q", ch.File, ch.Line, ch.SwitchName)
			}
			for _, caseNode := range ch.Children {
				var branchSelectors []string
				for _, ck := range caseNode.CaseKeys {
					bs, exists := swDef.Cases[ck]
					if !exists {
						return nil, fmt.Errorf("css compile error in %s.css:%d: unknown case %q in switch(%s)", ch.File, caseNode.Line, ck, ch.SwitchName)
					}
					branchSelectors = append(branchSelectors, bs)
				}
				combinedBranch := strings.Join(branchSelectors, ", ")
				targetSel := combineSwitchAndRuleSelector(combinedBranch, currentSelector)
				chBlocks, err := emitCSSNode(caseNode, targetSel, switches, ctx, stripDev)
				if err != nil {
					return nil, err
				}
				out = append(out, chBlocks...)
			}

		case cssKindCond:
			chBlocks, err := emitCSSNode(ch, currentSelector, switches, ctx, stripDev)
			if err != nil {
				return nil, err
			}
			out = append(out, chBlocks...)
		}
	}

	return out, nil
}

func formatCSSBlock(selector string, decls []string) string {
	parts := splitTopLevelCommas(selector)
	formattedSel := strings.Join(parts, ",\n")
	return fmt.Sprintf("%s {\n%s\n}", formattedSel, strings.Join(decls, "\n"))
}

func formatCSSDecls(decls []cssDecl, ctx *RenderContext, indent string) []string {
	out := make([]string, 0, len(decls))
	for _, d := range decls {
		key := normalizeCSSPropKey(d.Key)
		val := transformCSSValue(d.Val, ctx)
		out = append(out, fmt.Sprintf("%s%s: %s;", indent, key, val))
	}
	return out
}

func normalizeCSSPropKey(key string) string {
	if strings.HasPrefix(key, "$") {
		return "--" + key[1:]
	}
	return key
}

func transformCSSValue(val string, ctx *RenderContext) string {
	// 1. Interpolate {go_var} tokens
	if strings.Contains(val, "{") {
		val = interpolateCSSGoVars(val, ctx)
	}
	// 2. Transform $css_var, --css_var, and $css_var(fallback) / --css_var(fallback) tokens outside quotes
	if strings.Contains(val, "$") || strings.Contains(val, "--") {
		val = replaceCSSDollarVars(val)
	}
	// 3. Expand 1-digit (#x -> #xxxxxx) and 2-digit (#xy -> #xyxyxy) hex color shorthands
	if strings.Contains(val, "#") {
		val = expandShortHexColors(val)
	}
	return val
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func expandShortHexColors(s string) string {
	var b strings.Builder
	inQuote := byte(0)
	i := 0
	for i < len(s) {
		ch := s[i]
		if inQuote != 0 {
			if ch == '\\' && i+1 < len(s) {
				b.WriteByte(s[i])
				b.WriteByte(s[i+1])
				i += 2
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == '"' || ch == '\'' {
			inQuote = ch
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == '#' && (i == 0 || !isCSSIdentChar(s[i-1])) {
			j := i + 1
			for j < len(s) && isHexDigit(s[j]) {
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
		if inQuote != 0 {
			if ch == '\\' && i+1 < len(s) {
				b.WriteByte(s[i])
				b.WriteByte(s[i+1])
				i += 2
				continue
			}
			if ch == inQuote {
				inQuote = 0
				b.WriteByte(ch)
				i++
				continue
			}
			if ch == '{' {
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
			b.WriteByte(ch)
			i++
			continue
		}

		if ch == '"' || ch == '\'' {
			inQuote = ch
			b.WriteByte(ch)
			i++
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
				b.WriteString("var(--" + key + ")")
				i += end + 1
				continue
			}
		}

		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func replaceCSSDollarVars(s string) string {
	var b strings.Builder
	inQuote := byte(0)
	i := 0
	for i < len(s) {
		ch := s[i]
		if inQuote != 0 {
			if ch == '\\' && i+1 < len(s) {
				b.WriteByte(s[i])
				b.WriteByte(s[i+1])
				i += 2
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == '"' || ch == '\'' {
			inQuote = ch
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == '$' && i+1 < len(s) && isCSSIdentStart(s[i+1]) {
			start := i + 1
			j := start
			for j < len(s) && isCSSIdentChar(s[j]) {
				j++
			}
			varName := s[start:j]
			if j < len(s) && s[j] == '(' {
				// Find matching ')' for fallback value: $var(fallback) -> var(--var, fallback)
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
					fallback := strings.TrimSpace(s[j+1 : k-1])
					fallback = replaceCSSDollarVars(fallback)
					b.WriteString("var(--" + varName + ", " + fallback + ")")
					i = k
					continue
				}
			}
			b.WriteString("var(--" + varName + ")")
			i = j
			continue
		}
		if ch == '-' && i+2 < len(s) && s[i+1] == '-' && isCSSIdentStart(s[i+2]) && (i == 0 || !isCSSIdentChar(s[i-1])) {
			prev := i - 1
			for prev >= 0 && (s[prev] == ' ' || s[prev] == '\t') {
				prev--
			}
			alreadyInVar := prev >= 3 && s[prev-3:prev+1] == "var(" && (prev-3 == 0 || !isCSSIdentChar(s[prev-4]))
			if !alreadyInVar {
				start := i + 2
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
						fallback := strings.TrimSpace(s[j+1 : k-1])
						fallback = replaceCSSDollarVars(fallback)
						b.WriteString("var(--" + varName + ", " + fallback + ")")
						i = k
						continue
					}
				}
				b.WriteString("var(--" + varName + ")")
				i = j
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

func splitTopLevelCommas(s string) []string {
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
		case ',':
			if parenDepth == 0 && bracketDepth == 0 {
				trimmed := strings.TrimSpace(s[last:i])
				if trimmed != "" {
					parts = append(parts, trimmed)
				}
				last = i + 1
			}
		}
	}
	if tail := strings.TrimSpace(s[last:]); tail != "" {
		parts = append(parts, tail)
	}
	return parts
}

func splitTopLevelPipes(s string) []string {
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
		case '|':
			if parenDepth == 0 && bracketDepth == 0 {
				parts = append(parts, strings.TrimSpace(s[last:i]))
				last = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[last:]))
	return parts
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
	var seen = make(map[string]bool)
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
		bBase := extractBaseSelectorPrefix(b)
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

func extractBaseSelectorPrefix(sel string) string {
	for i := 0; i < len(sel); i++ {
		if sel[i] == '[' || sel[i] == ':' || sel[i] == ' ' {
			return sel[:i]
		}
	}
	return sel
}
