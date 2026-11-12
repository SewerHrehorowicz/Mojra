//go:build !(js && wasm)

package tpl

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type cssParser struct {
	name     string
	switches map[string]*CSSSwitchDef
	stack    []*cssNode
}

// DefaultCSSSwitches returns a new switch registry pre-populated with default theme and preview switches.
func DefaultCSSSwitches() map[string]*CSSSwitchDef {
	out := make(map[string]*CSSSwitchDef, len(defaultCSSSwitchSpecs))
	for _, spec := range defaultCSSSwitchSpecs {
		order := make([]string, 0, len(spec.Cases))
		cases := make(map[string]string, len(spec.Cases))
		for _, c := range spec.Cases {
			order = append(order, c.Key)
			cases[c.Key] = c.Selector
		}
		out[spec.Name] = &CSSSwitchDef{
			Name:  spec.Name,
			Order: order,
			Cases: cases,
		}
	}
	return out
}

// TranspileCSSFilesToString reads, transpiles, and bundles multiple CSS template files sharing switch definitions.
func TranspileCSSFilesToString(cssFiles []string, stripDev bool, ctx *RenderContext) (string, error) {
	switches := DefaultCSSSwitches()
	var sb strings.Builder
	for _, path := range cssFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf(errReadCSSFileFmt, path, err)
		}
		src := string(raw)
		if stripDev {
			src = StripDevOnlyBlocks(src)
		}
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), tssExt), cssExt)
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

func isCSSPropertyAssignment(content string) bool {
	if isCSSControlPrefix(content) {
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
	if isCustomCSSPropKey(key) {
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
	if rest, ok := strings.CutPrefix(trimmed, tssKwSwitchSpace); ok {
		name := trimCSSTrailingToken(rest, ":")
		if cssCaseKeyIdentRe.MatchString(name) {
			return name, true
		}
	}
	return "", false
}

func parseInlineSwitchTarget(expr string) (string, string, bool) {
	trimmed := trimCSSTrailingToken(expr, ";")
	if !strings.HasPrefix(trimmed, tssKwSwitchParen) {
		return "", "", false
	}
	closeIdx := findMatchingCloseParen(trimmed, len(tssKwSwitchParen)-1)
	if closeIdx < 0 {
		return "", "", false
	}
	inside := strings.TrimSpace(trimmed[len(tssKwSwitchParen):closeIdx])
	after := trimCSSLeadingToken(trimmed[closeIdx+1:], ":")
	if inside == "" || !isCSSIdentStart(inside[0]) {
		return "", "", false
	}
	j := 0
	for j < len(inside) && isCSSIdentChar(inside[j]) {
		j++
	}
	swName := inside[:j]
	insideRest := strings.TrimSpace(inside[j:])
	if len(insideRest) > 0 && (insideRest[0] == ',' || insideRest[0] == ':') {
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

func parseCSSPropHeader(content string) (cssPropHeader, bool) {
	if isCSSControlPrefix(content) {
		return cssPropHeader{}, false
	}
	if isCSSPropertyAssignment(content) {
		colonIdx := findFirstUnquotedColon(content)
		return cssPropHeader{
			Key:      strings.TrimSpace(content[:colonIdx]),
			Val:      trimCSSTrailingToken(content[colonIdx+1:], ";"),
			HasColon: true,
		}, true
	}
	spaceIdx := strings.IndexAny(content, " \t")
	if spaceIdx <= 0 {
		return cssPropHeader{}, false
	}
	key := strings.TrimSpace(content[:spaceIdx])
	val := trimCSSTrailingToken(content[spaceIdx+1:], ";")
	if !cssPropKeyRe.MatchString(key) || val == "" {
		return cssPropHeader{}, false
	}
	return cssPropHeader{Key: key, Val: val, HasColon: false}, true
}

func parseInlineSwitchCases(inlineExpr string, swDef *CSSSwitchDef, name, swName string, lineNum int) ([]cssSwitchCaseVal, error) {
	var parts []string
	if strings.Contains(inlineExpr, "|") {
		parts = splitTopLevelDelimiter(inlineExpr, '|', false)
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
			val := trimCSSTrailingToken(inlineExpr[colonIdx+1:], ";")
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
			val := trimCSSTrailingToken(seg[colonIdx+1:], ";")
			out = append(out, cssSwitchCaseVal{
				CaseKeys: rawKeys,
				Val:      val,
				Line:     lineNum,
			})
			continue
		}
		val := trimCSSTrailingToken(seg, ";")
		if val == "" || val == "_" {
			continue
		}
		if idx >= len(swDef.Order) {
			return nil, fmt.Errorf(errCSSInlineSwitchCountFmt, name, lineNum, len(parts), swName, len(swDef.Order))
		}
		out = append(out, cssSwitchCaseVal{
			CaseKeys: []string{swDef.Order[idx]},
			Val:      val,
			Line:     lineNum,
		})
	}
	return out, nil
}

func (p *cssParser) parseSwitchDefChild(parent *cssNode, content string, lineNum int) error {
	m := cssSwitchDefCaseLineRe.FindStringSubmatch(content)
	if len(m) != 3 {
		return fmt.Errorf(errCSSSwitchDefCaseFmt, p.name, lineNum, content)
	}
	caseKey := strings.TrimSpace(m[1])
	selVal := strings.TrimSpace(m[2])
	def := p.switches[parent.SwitchName]
	if _, exists := def.Cases[caseKey]; !exists {
		def.Order = append(def.Order, caseKey)
	}
	def.Cases[caseKey] = selVal
	return nil
}

func (p *cssParser) parseSwitchStatement(parent *cssNode, content string, indent, lineNum int, childHasGrandchildren bool) {
	isAtSwitch := len(content) > 0 && content[0] == '@'
	rest := trimCSSLeadingToken(strings.TrimPrefix(content, "@"), tssKwSwitch)
	if len(rest) >= 2 && rest[0] == '(' && rest[len(rest)-1] == ')' {
		rest = strings.TrimSpace(rest[1 : len(rest)-1])
	}

	if isAtSwitch && (parent.Kind != cssKindRoot || childHasGrandchildren) {
		swName := trimCSSTrailingToken(rest, ":")
		swNode := &cssNode{
			Kind:       cssKindBlockSwitch,
			SwitchName: swName,
			Indent:     indent,
			Line:       lineNum,
			File:       p.name,
		}
		parent.Children = append(parent.Children, swNode)
		p.stack = append(p.stack, swNode)
		return
	}

	if colonIdx := findFirstUnquotedColon(rest); colonIdx > 0 {
		swName := strings.TrimSpace(rest[:colonIdx])
		branches := splitTopLevelDelimiter(rest[colonIdx+1:], '|', false)
		def := &CSSSwitchDef{
			Name:  swName,
			Cases: make(map[string]string),
		}
		for idx, br := range branches {
			if br == "" {
				continue
			}
			key := strconv.Itoa(idx)
			def.Order = append(def.Order, key)
			def.Cases[key] = br
		}
		p.switches[swName] = def
		return
	}
	swName := trimCSSTrailingToken(rest, ":")
	p.switches[swName] = &CSSSwitchDef{
		Name:  swName,
		Cases: make(map[string]string),
	}
	swNode := &cssNode{
		Kind:       cssKindSwitchDef,
		SwitchName: swName,
		Indent:     indent,
		Line:       lineNum,
		File:       p.name,
	}
	p.stack = append(p.stack, swNode)
}

func (p *cssParser) parseArrowControlFlow(parent *cssNode, pl cssPreprocessedLine, lhs, rhs string) error {
	if lhs == "" {
		if swName, ok := parseSwitchTargetName(rhs); ok {
			swNode := &cssNode{
				Kind:       cssKindBlockSwitch,
				SwitchName: swName,
				Indent:     pl.Indent,
				Line:       pl.LineNum,
				File:       p.name,
			}
			parent.Children = append(parent.Children, swNode)
			p.stack = append(p.stack, swNode)
			return nil
		}
		if condExpr, ok := strings.CutPrefix(rhs, tssKwIfSpace); ok {
			condNode := &cssNode{
				Kind:   cssKindCond,
				Cond:   strings.TrimSpace(condExpr),
				Indent: pl.Indent,
				Line:   pl.LineNum,
				File:   p.name,
			}
			parent.Children = append(parent.Children, condNode)
			p.stack = append(p.stack, condNode)
			return nil
		}
		return fmt.Errorf(errCSSUnsupportedFlowFmt, p.name, pl.LineNum, pl.Content)
	}

	if swName, inlineExpr, ok := parseInlineSwitchTarget(rhs); ok {
		if !cssPropKeyRe.MatchString(lhs) {
			return fmt.Errorf(errCSSInvalidPropSwitchFmt, p.name, pl.LineNum, lhs, swName)
		}
		return p.addPropSwitch(parent, pl, lhs, swName, inlineExpr)
	}

	return fmt.Errorf(errCSSReservedArrowFmt, p.name, pl.LineNum, lhs, rhs, lhs, rhs)
}

func (p *cssParser) addPropSwitch(parent *cssNode, pl cssPreprocessedLine, propKey, swName, inlineExpr string) error {
	psNode := &cssNode{
		Kind:       cssKindPropSwitch,
		PropKey:    propKey,
		SwitchName: swName,
		Indent:     pl.Indent,
		Line:       pl.LineNum,
		File:       p.name,
	}
	if inlineExpr != "" {
		swDef, exists := p.switches[swName]
		if !exists {
			return fmt.Errorf(errCSSUnknownSwitchFmt, p.name, pl.LineNum, swName)
		}
		cases, err := parseInlineSwitchCases(inlineExpr, swDef, p.name, swName, pl.LineNum)
		if err != nil {
			return err
		}
		psNode.PropCases = append(psNode.PropCases, cases...)
		parent.Children = append(parent.Children, psNode)
		return nil
	}
	parent.Children = append(parent.Children, psNode)
	p.stack = append(p.stack, psNode)
	return nil
}

func transpileCSSWithSwitches(source, name string, ctx *RenderContext, switches map[string]*CSSSwitchDef, stripDev bool) (string, error) {
	lines := preprocessCSSLines(source)
	root := &cssNode{Kind: cssKindRoot, Indent: -1, File: name}
	p := &cssParser{
		name:     name,
		switches: switches,
		stack:    []*cssNode{root},
	}

	for lineIdx, pl := range lines {
		indent := pl.Indent
		content := pl.Content
		lineNum := pl.LineNum
		hasChildren := lineIdx+1 < len(lines) && lines[lineIdx+1].Indent > indent

		for len(p.stack) > 1 && p.stack[len(p.stack)-1].Indent >= indent {
			p.stack = p.stack[:len(p.stack)-1]
		}
		parent := p.stack[len(p.stack)-1]

		if parent.Kind == cssKindSwitchDef {
			if err := p.parseSwitchDefChild(parent, content, lineNum); err != nil {
				return "", err
			}
			continue
		}

		if parent.Kind == cssKindPropSwitch {
			rawKeys, _, val, ok := parsePropSwitchCaseLine(content)
			if !ok {
				return "", fmt.Errorf(errCSSPropSwitchCaseFmt, p.name, lineNum, content)
			}
			parent.PropCases = append(parent.PropCases, cssSwitchCaseVal{
				CaseKeys: rawKeys,
				Val:      val,
				Line:     lineNum,
			})
			continue
		}

		if parent.Kind == cssKindBlockSwitch {
			caseSpec := trimCSSTrailingToken(content, ":")
			caseNode := &cssNode{
				Kind:     cssKindSwitchCase,
				CaseKeys: splitTopLevelCommas(caseSpec),
				Indent:   indent,
				Line:     lineNum,
				File:     name,
			}
			parent.Children = append(parent.Children, caseNode)
			p.stack = append(p.stack, caseNode)
			continue
		}

		if condExpr, ok := strings.CutPrefix(content, tssKwAtIfSpace); ok {
			condNode := &cssNode{
				Kind:   cssKindCond,
				Cond:   strings.TrimSpace(condExpr),
				Indent: indent,
				Line:   lineNum,
				File:   name,
			}
			parent.Children = append(parent.Children, condNode)
			p.stack = append(p.stack, condNode)
			continue
		}

		if lhs, rhs, hasArrow := splitArrowSides(content); hasArrow {
			if err := p.parseArrowControlFlow(parent, pl, lhs, rhs); err != nil {
				return "", err
			}
			continue
		}

		if isBlockSwitchOrDefLine(content) {
			childHasGrandchildren := hasChildren && lineIdx+2 < len(lines) && lines[lineIdx+2].Indent > lines[lineIdx+1].Indent
			p.parseSwitchStatement(parent, content, indent, lineNum, childHasGrandchildren)
			continue
		}

		if len(content) >= 2 && content[0] == '"' && content[len(content)-1] == '"' {
			if parent.Kind == cssKindRoot {
				return "", fmt.Errorf(errCSSBareStringRootFmt, name, lineNum, content)
			}
			parent.Decls = append(parent.Decls, cssDecl{
				Key:  cssContentProp,
				Val:  content,
				Line: lineNum,
			})
			continue
		}

		if hdr, ok := parseCSSPropHeader(content); ok {
			if swName, inlineExpr, isSw := parseInlineSwitchTarget(hdr.Val); isSw {
				if err := p.addPropSwitch(parent, pl, hdr.Key, swName, inlineExpr); err != nil {
					return "", err
				}
				continue
			}

			if hdr.HasColon || !hasChildren {
				if parent.Kind == cssKindRoot && !isCustomCSSPropKey(hdr.Key) {
					return "", fmt.Errorf(errCSSUnindentedPropFmt, name, lineNum, content)
				}

				if hdr.HasColon && cssPseudoShorthands[hdr.Key] != "" {
					pseudoNode := &cssNode{
						Kind:     cssKindRule,
						Selector: cssPseudoShorthands[hdr.Key],
						Decls: []cssDecl{
							{Key: cssContentProp, Val: hdr.Val, Line: lineNum},
						},
						Indent: indent,
						Line:   lineNum,
						File:   name,
					}
					parent.Children = append(parent.Children, pseudoNode)
					p.stack = append(p.stack, pseudoNode)
					continue
				}

				parent.Decls = append(parent.Decls, cssDecl{
					Key:  hdr.Key,
					Val:  hdr.Val,
					Line: lineNum,
				})
				continue
			}
		}

		sel := strings.TrimSpace(content)
		if pseudoSel := cssPseudoShorthands[sel]; pseudoSel != "" {
			sel = pseudoSel
		}
		ruleNode := &cssNode{
			Kind:     cssKindRule,
			Selector: sel,
			Indent:   indent,
			Line:     lineNum,
			File:     name,
		}
		parent.Children = append(parent.Children, ruleNode)
		p.stack = append(p.stack, ruleNode)
	}

	blocks, err := emitCSSNode(root, "", switches, ctx, stripDev)
	if err != nil {
		return "", err
	}
	return strings.Join(blocks, "\n\n"), nil
}
