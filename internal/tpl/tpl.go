package tpl

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type NodeKind int

const (
	NodeElement NodeKind = iota
	NodeDoctype
	NodeDef
	NodeComponent
	NodeSlot
	NodeIf
	NodeElif
	NodeElse
	NodeFor
	NodeWith
	NodeText
	NodeProse
	NodeSVG
)

type CondClass struct {
	Name string
	Cond string
}

type AttrPair struct {
	Key         string
	Val         string
	Conditional bool
}

type Node struct {
	Kind        NodeKind
	Tag         string
	IsComponent bool
	Classes     []string
	CondClasses []CondClass
	ID          string
	Attrs       []AttrPair
	Text        string
	Expr        string
	ContextExpr string
	LoopIdx     string
	LoopVal     string
	RawSVG      string
	Children    []*Node
	Indent      int
	Line        int
	File        string
}

type ParsedTemplate struct {
	Name  string
	Nodes []*Node
}

type RenderContext struct {
	Vars      map[string]any
	parent    *RenderContext
	localKey  string
	localVal  any
	hasLocal  bool
	slotNodes []*Node
	slotCtx   *RenderContext
}

var renderBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 64*1024))
	},
}

// AcquireBuffer returns a pooled 64KB+ bytes.Buffer ready for template or stylesheet rendering.
func AcquireBuffer() *bytes.Buffer {
	b := renderBufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

// ReleaseBuffer resets and returns a buffer to the pool if it has not grown past 2MB.
func ReleaseBuffer(b *bytes.Buffer) {
	if b == nil || b.Cap() > 2*1024*1024 {
		return
	}
	b.Reset()
	renderBufPool.Put(b)
}

func NewContext(vars map[string]any) *RenderContext {
	if vars == nil {
		vars = make(map[string]any)
	}
	return &RenderContext{Vars: vars}
}

// WithVar returns a lightweight child RenderContext binding key=val without copying parent maps.
func (c *RenderContext) WithVar(key string, val any) *RenderContext {
	if c == nil {
		return &RenderContext{
			localKey: key,
			localVal: val,
			hasLocal: true,
		}
	}
	return &RenderContext{
		parent:    c,
		localKey:  key,
		localVal:  val,
		hasLocal:  true,
		slotNodes: c.slotNodes,
		slotCtx:   c.slotCtx,
	}
}

var (
	attrPairRe = regexp.MustCompile(`([a-zA-Z_:][a-zA-Z0-9_.:-]*)(?:\s*=\s*"([^"]*)")?`)
	attrKeyRe  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.:-]*$`)
	validTagRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	validIDRe  = regexp.MustCompile(`^[a-zA-Z0-9_.{}$-]+$`)
	svgEventRe = regexp.MustCompile(`(?i)\bon[a-z0-9_]+\s*=`)
)

var forbiddenTags = map[string]bool{
	"style":         true,
	"iframe":        true,
	"frame":         true,
	"frameset":      true,
	"object":        true,
	"embed":         true,
	"applet":        true,
	"base":          true,
	"foreignobject": true,
}

var selfClosingTags = map[string]bool{
	"img":   true,
	"br":    true,
	"hr":    true,
	"input": true,
	"meta":  true,
	"link":  true,
}

var voidWithoutSlashTags = map[string]bool{
	"meta": true,
	"link": true,
}

var boolAttrs = map[string]bool{
	"autofocus": true,
	"required":  true,
	"readonly":  true,
	"checked":   true,
	"disabled":  true,
	"selected":  true,
	"multiple":  true,
	"download":  true,
	"open":      true,
	"controls":  true,
}

func isTPLComment(trimmed string) bool {
	return trimmed == "" || strings.HasPrefix(trimmed, "//") || trimmed == "#" || strings.HasPrefix(trimmed, "# ")
}

// ParseTemplateString parses Workflow-format .tpl source and enforces compile-time security gates.
func ParseTemplateString(source, name string) (*ParsedTemplate, error) {
	rawLines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var rootNodes []*Node
	var stack []*Node

	appendNode := func(n *Node) error {
		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			if parent.Kind == NodeElement && strings.EqualFold(parent.Tag, "script") {
				return fmt.Errorf(
					"tpl compile error in %s.tpl:%d: inline JS inside <script> is strictly forbidden in .tpl files; put all JavaScript in app/js/*.js and reference it via script src: /app.js",
					name, n.Line,
				)
			}
			parent.Children = append(parent.Children, n)
		} else {
			rootNodes = append(rootNodes, n)
		}
		stack = append(stack, n)
		return nil
	}

	for idx, rawLine := range rawLines {
		lineNum := idx + 1
		trimmed := strings.TrimSpace(rawLine)
		if isTPLComment(trimmed) {
			continue
		}

		if strings.HasPrefix(trimmed, ". ") || strings.HasPrefix(trimmed, ".\t") {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: '. ' property continuation syntax is not supported; use indented 'key: val'", name, lineNum)
		}
		if findUnquotedArrow(trimmed) >= 0 {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: '->' syntax is not supported; use '{...}' blocks or inline '{expr}'", name, lineNum)
		}
		if containsUnquotedQuestionEquals(trimmed) {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: '?=' syntax is not supported; use '?:' conditional syntax", name, lineNum)
		}

		indent := countLeadingIndent(rawLine)
		for len(stack) > 0 && stack[len(stack)-1].Indent >= indent {
			stack = stack[:len(stack)-1]
		}

		if len(stack) > 0 && (stack[len(stack)-1].Kind == NodeElement || stack[len(stack)-1].Kind == NodeComponent) && isAttrOrCondClassContinuation(trimmed) {
			parent := stack[len(stack)-1]
			if err := parseTokensIntoNode(parent, trimmed, name, lineNum); err != nil {
				return nil, err
			}
			continue
		}

		switch {
		case strings.EqualFold(trimmed, "doctype html"):
			if err := appendNode(&Node{Kind: NodeDoctype, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case isBraceWrapped(trimmed):
			n, err := parseBraceLine(trimmed, indent, name, lineNum)
			if err != nil {
				return nil, err
			}
			if err := appendNode(n); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, "def:"):
			defName := strings.TrimSpace(trimmed[4:])
			if defName == "" {
				return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty def:", name, lineNum)
			}
			if err := appendNode(&Node{Kind: NodeDef, Tag: defName, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, "if:"):
			expr := unwrapOuterBraces(strings.TrimSpace(trimmed[3:]))
			if expr == "" {
				return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty if:", name, lineNum)
			}
			if err := appendNode(&Node{Kind: NodeIf, Expr: expr, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, "elif:"):
			expr := unwrapOuterBraces(strings.TrimSpace(trimmed[5:]))
			if expr == "" {
				return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty elif:", name, lineNum)
			}
			if err := appendNode(&Node{Kind: NodeElif, Expr: expr, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case trimmed == "else:":
			if err := appendNode(&Node{Kind: NodeElse, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, "with:"):
			expr := unwrapOuterBraces(strings.TrimSpace(trimmed[5:]))
			if expr == "" {
				return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty with:", name, lineNum)
			}
			if err := appendNode(&Node{Kind: NodeWith, Expr: expr, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, "for:"):
			n, err := parseForSpec(strings.TrimSpace(trimmed[4:]), indent, name, lineNum)
			if err != nil {
				return nil, err
			}
			if err := appendNode(n); err != nil {
				return nil, err
			}

		case trimmed == "@slot":
			if err := appendNode(&Node{Kind: NodeSlot, Tag: "slot", IsComponent: true, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, "@"):
			compNode, err := parseComponentLine(trimmed, indent, name, lineNum)
			if err != nil {
				return nil, err
			}
			if err := appendNode(compNode); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, "svg:"):
			rest := strings.TrimSpace(trimmed[4:])
			kindTok, nextPos := nextTPLToken(rest, 0)
			actTok, _ := nextTPLToken(rest, nextPos)
			if err := appendNode(&Node{Kind: NodeSVG, Tag: "svg", Expr: kindTok, ContextExpr: actTok, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, "raw:"):
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: raw: is strictly forbidden for security; use {prose ...}", name, lineNum)

		case strings.HasPrefix(trimmed, "prose:"):
			expr := unwrapOuterBraces(strings.TrimSpace(trimmed[6:]))
			if expr == "" {
				return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty prose:", name, lineNum)
			}
			if err := appendNode(&Node{Kind: NodeProse, Expr: expr, Indent: indent, Line: lineNum, File: name}); err != nil {
				return nil, err
			}

		case strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, "$") || strings.HasPrefix(trimmed, "("):
			if len(stack) == 0 {
				return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: text or expression line %q must be indented under an element", name, lineNum, trimmed)
			}
			if strings.HasPrefix(trimmed, `"`) {
				unq, nextIdx, err := readQuotedString(trimmed, 0, name, lineNum)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(trimmed[nextIdx:]) != "" {
					return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: unexpected trailing tokens after string literal", name, lineNum)
				}
				if err := appendNode(&Node{Kind: NodeText, Text: `"` + unq + `"`, Indent: indent, Line: lineNum, File: name}); err != nil {
					return nil, err
				}
			} else {
				if err := appendNode(&Node{Kind: NodeText, Expr: trimmed, Indent: indent, Line: lineNum, File: name}); err != nil {
					return nil, err
				}
			}

		default:
			if isAttrOrCondClassContinuation(trimmed) && len(stack) == 0 {
				return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: attribute line %q must be indented under an element", name, lineNum, trimmed)
			}
			elNode, err := parseElementLine(trimmed, indent, name, lineNum)
			if err != nil {
				return nil, err
			}
			if err := appendNode(elNode); err != nil {
				return nil, err
			}
		}
	}

	if err := validateNodesRecursive(rootNodes); err != nil {
		return nil, err
	}

	return &ParsedTemplate{
		Name:  name,
		Nodes: rootNodes,
	}, nil
}

func containsUnquotedQuestionEquals(s string) bool {
	inQuotes := false
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '"' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes && s[i] == '?' && s[i+1] == '=' {
			return true
		}
	}
	return false
}

// findUnquotedArrow finds "->" outside of double-quoted strings and parentheses (also used by tss.go).
func findUnquotedArrow(s string) int {
	inQuotes := false
	parenDepth := 0
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '"' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes {
			if s[i] == '(' {
				parenDepth++
				continue
			}
			if s[i] == ')' && parenDepth > 0 {
				parenDepth--
				continue
			}
			if parenDepth == 0 && s[i] == '-' && s[i+1] == '>' {
				if i == 0 || s[i-1] == ' ' || s[i-1] == '\t' {
					return i
				}
			}
		}
	}
	return -1
}

func isBalancedWrapped(s string, openCh, closeCh byte) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != openCh || s[len(s)-1] != closeCh {
		return false
	}
	depth := 0
	inQuotes := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuotes {
			if ch == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if ch == '"' {
				inQuotes = false
			}
			continue
		}
		if ch == '"' {
			inQuotes = true
			continue
		}
		if ch == openCh {
			depth++
		} else if ch == closeCh {
			depth--
			if depth == 0 && i < len(s)-1 {
				return false
			}
		}
	}
	return depth == 0
}

func isBraceWrapped(s string) bool {
	return isBalancedWrapped(s, '{', '}')
}

func isParenWrapped(s string) bool {
	return isBalancedWrapped(s, '(', ')')
}

func unwrapOuterBraces(s string) string {
	s = strings.TrimSpace(s)
	if isBraceWrapped(s) {
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

func hasBraceKeyword(inner, kw string) bool {
	if !strings.HasPrefix(inner, kw) {
		return false
	}
	if len(inner) == len(kw) {
		return true
	}
	next := inner[len(kw)]
	return next == ' ' || next == '\t' || next == '('
}

func parseForSpec(spec string, indent int, fileName string, lineNum int) (*Node, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty for block", fileName, lineNum)
	}
	n := &Node{Kind: NodeFor, Indent: indent, Line: lineNum, File: fileName}
	if inIdx := strings.Index(spec, " in "); inIdx > 0 {
		varsPart := strings.TrimSpace(spec[:inIdx])
		n.Expr = unwrapOuterBraces(strings.TrimSpace(spec[inIdx+4:]))
		if commaIdx := strings.IndexByte(varsPart, ','); commaIdx > 0 {
			n.LoopIdx = strings.TrimPrefix(strings.TrimSpace(varsPart[:commaIdx]), "$")
			n.LoopVal = strings.TrimPrefix(strings.TrimSpace(varsPart[commaIdx+1:]), "$")
		} else {
			n.LoopVal = strings.TrimPrefix(varsPart, "$")
		}
	} else {
		n.Expr = unwrapOuterBraces(spec)
	}
	return n, nil
}

func parseBraceLine(trimmed string, indent int, fileName string, lineNum int) (*Node, error) {
	inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if inner == "" {
		return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty {} block", fileName, lineNum)
	}
	switch {
	case inner == "else":
		return &Node{Kind: NodeElse, Indent: indent, Line: lineNum, File: fileName}, nil

	case inner == "slot":
		return &Node{Kind: NodeSlot, Tag: "slot", IsComponent: true, Indent: indent, Line: lineNum, File: fileName}, nil

	case hasBraceKeyword(inner, "def"):
		defName := strings.TrimSpace(inner[3:])
		if defName == "" {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty {def}", fileName, lineNum)
		}
		return &Node{Kind: NodeDef, Tag: defName, Indent: indent, Line: lineNum, File: fileName}, nil

	case hasBraceKeyword(inner, "if"):
		rest := strings.TrimSpace(inner[2:])
		if rest == "" {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty {if}", fileName, lineNum)
		}
		return &Node{Kind: NodeIf, Expr: rest, Indent: indent, Line: lineNum, File: fileName}, nil

	case hasBraceKeyword(inner, "elif"):
		rest := strings.TrimSpace(inner[4:])
		if rest == "" {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty {elif}", fileName, lineNum)
		}
		return &Node{Kind: NodeElif, Expr: rest, Indent: indent, Line: lineNum, File: fileName}, nil

	case strings.HasPrefix(inner, "else if ") || strings.HasPrefix(inner, "else if\t"):
		rest := strings.TrimSpace(inner[7:])
		if rest == "" {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty {else if}", fileName, lineNum)
		}
		return &Node{Kind: NodeElif, Expr: rest, Indent: indent, Line: lineNum, File: fileName}, nil

	case hasBraceKeyword(inner, "with"):
		rest := strings.TrimSpace(inner[4:])
		if rest == "" {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty {with}", fileName, lineNum)
		}
		return &Node{Kind: NodeWith, Expr: rest, Indent: indent, Line: lineNum, File: fileName}, nil

	case hasBraceKeyword(inner, "for"):
		return parseForSpec(strings.TrimSpace(inner[3:]), indent, fileName, lineNum)

	case hasBraceKeyword(inner, "raw"):
		return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: {raw} is strictly forbidden for security; use {prose ...}", fileName, lineNum)

	case hasBraceKeyword(inner, "prose"):
		rest := strings.TrimSpace(inner[5:])
		if rest == "" {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty {prose}", fileName, lineNum)
		}
		return &Node{Kind: NodeProse, Expr: rest, Indent: indent, Line: lineNum, File: fileName}, nil

	case hasBraceKeyword(inner, "svg"):
		rest := strings.TrimSpace(inner[3:])
		kindTok, nextPos := nextTPLToken(rest, 0)
		actTok, _ := nextTPLToken(rest, nextPos)
		return &Node{Kind: NodeSVG, Tag: "svg", Expr: kindTok, ContextExpr: actTok, Indent: indent, Line: lineNum, File: fileName}, nil

	default:
		return &Node{Kind: NodeText, Expr: inner, Indent: indent, Line: lineNum, File: fileName}, nil
	}
}

func isAttrOrCondClassContinuation(trimmed string) bool {
	if strings.HasPrefix(trimmed, "[") {
		return true
	}
	if strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, "$") || strings.HasPrefix(trimmed, "(") || strings.HasPrefix(trimmed, "{") {
		return false
	}
	for _, kw := range []string{"def:", "if:", "elif:", "else:", "for:", "with:", "raw:", "prose:", "svg:", "doctype "} {
		if strings.HasPrefix(trimmed, kw) || trimmed == "else:" {
			return false
		}
	}
	firstTok, _ := nextTPLToken(trimmed, 0)
	if strings.HasPrefix(firstTok, ".") {
		return strings.Contains(firstTok, "?:")
	}
	if strings.HasPrefix(firstTok, "#") {
		return false
	}
	return strings.HasSuffix(firstTok, ":") || strings.Contains(firstTok, "=") || strings.Contains(firstTok, ":")
}

func nextTPLToken(s string, start int) (string, int) {
	n := len(s)
	i := start
	for i < n && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i >= n {
		return "", n
	}
	var b strings.Builder
	parenDepth := 0
	braceDepth := 0
	inQuotes := false
	for i < n {
		ch := s[i]
		if inQuotes {
			if ch == '\\' && i+1 < n {
				b.WriteByte(s[i])
				b.WriteByte(s[i+1])
				i += 2
				continue
			}
			if ch == '"' {
				inQuotes = false
			}
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == '"' {
			inQuotes = true
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == '(' {
			parenDepth++
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == ')' {
			if parenDepth > 0 {
				parenDepth--
			}
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == '{' {
			braceDepth++
			b.WriteByte(ch)
			i++
			continue
		}
		if ch == '}' {
			if braceDepth > 0 {
				braceDepth--
			}
			b.WriteByte(ch)
			i++
			continue
		}
		if parenDepth == 0 && braceDepth == 0 && (ch == ' ' || ch == '\t') {
			j := i
			for j < n && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			cur := b.String()
			if strings.HasSuffix(cur, "=") || (strings.HasPrefix(cur, ".") && strings.HasSuffix(cur, "?:")) {
				i = j
				continue
			}
			if j < n && (s[j] == '=' || (strings.HasPrefix(cur, ".") && strings.HasPrefix(s[j:], "?:"))) {
				if strings.HasPrefix(s[j:], "?:") {
					b.WriteString("?:")
					j += 2
				} else {
					b.WriteByte('=')
					j++
				}
				for j < n && (s[j] == ' ' || s[j] == '\t') {
					j++
				}
				i = j
				continue
			}
			if j < n && cur != "" && !strings.HasSuffix(cur, ":") && (strings.HasPrefix(s[j:], "?:") || s[j] == ':') {
				if strings.HasPrefix(s[j:], "?:") {
					b.WriteString("?:")
					i = j + 2
				} else {
					b.WriteByte(':')
					i = j + 1
				}
				break
			}
			break
		}
		b.WriteByte(ch)
		i++
	}
	return b.String(), i
}

func parseSelectorNodeLine(rawSel, rest string, kind NodeKind, isComp bool, indent int, fileName string, lineNum int) (*Node, error) {
	tag, id, classes, condClasses, err := parseSelectorToken(rawSel, fileName, lineNum)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Kind:        kind,
		Tag:         tag,
		IsComponent: isComp,
		ID:          id,
		Classes:     classes,
		CondClasses: condClasses,
		Indent:      indent,
		Line:        lineNum,
		File:        fileName,
	}
	if rest != "" {
		if err := parseTokensIntoNode(n, rest, fileName, lineNum); err != nil {
			return nil, err
		}
	}
	return n, nil
}

func parseComponentLine(trimmed string, indent int, fileName string, lineNum int) (*Node, error) {
	firstTok, nextIdx := nextTPLToken(trimmed, 0)
	rawSel := strings.TrimPrefix(firstTok, "@")
	if rawSel == "" {
		return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: missing component name after @", fileName, lineNum)
	}
	return parseSelectorNodeLine(rawSel, strings.TrimSpace(trimmed[nextIdx:]), NodeComponent, true, indent, fileName, lineNum)
}

func parseElementLine(trimmed string, indent int, fileName string, lineNum int) (*Node, error) {
	selTok, nextIdx := nextTPLToken(trimmed, 0)
	return parseSelectorNodeLine(selTok, strings.TrimSpace(trimmed[nextIdx:]), NodeElement, false, indent, fileName, lineNum)
}

func parseSelectorToken(sel string, fileName string, lineNum int) (tag string, id string, classes []string, condClasses []CondClass, err error) {
	tag = "div"
	i := 0
	n := len(sel)

	if i < n && sel[i] != '.' && sel[i] != '#' {
		start := i
		for i < n && sel[i] != '.' && sel[i] != '#' {
			i++
		}
		tag = sel[start:i]
	}

	for i < n {
		sep := sel[i]
		i++
		start := i
		inVar := false
		parenDepth := 0
		braceDepth := 0
		for i < n {
			ch := sel[i]
			if ch == '$' {
				inVar = true
			} else if ch == '(' {
				parenDepth++
			} else if ch == ')' && parenDepth > 0 {
				parenDepth--
			} else if ch == '{' {
				braceDepth++
			} else if ch == '}' && braceDepth > 0 {
				braceDepth--
			} else if parenDepth == 0 && braceDepth == 0 {
				if ch == '#' {
					break
				}
				if ch == '.' {
					if !inVar || i+1 >= n || sel[i+1] < 'A' || sel[i+1] > 'Z' {
						break
					}
				}
			}
			i++
		}
		seg := sel[start:i]
		if seg == "" {
			continue
		}
		if sep == '#' {
			id = seg
		} else if sep == '.' {
			if qIdx := strings.Index(seg, "?:"); qIdx >= 0 {
				clsName := strings.TrimSpace(seg[:qIdx])
				condExpr := unwrapOuterBraces(strings.TrimSpace(seg[qIdx+2:]))
				if clsName == "" || condExpr == "" {
					return "", "", nil, nil, fmt.Errorf("tpl compile error in %s.tpl:%d: invalid conditional CSS class %q", fileName, lineNum, seg)
				}
				condClasses = append(condClasses, CondClass{Name: clsName, Cond: condExpr})
			} else {
				classes = append(classes, seg)
			}
		}
	}
	return tag, id, classes, condClasses, nil
}

func formatAttrTokenValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`) && len(raw) >= 2 {
		return raw
	}
	if isBraceWrapped(raw) || isParenWrapped(raw) || strings.HasPrefix(raw, "$") {
		return raw
	}
	return raw
}

func parseTokensIntoNode(n *Node, rest string, fileName string, lineNum int) error {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil
	}

	if n.Kind == NodeComponent && len(n.Attrs) == 0 && n.ContextExpr == "" {
		firstArg, afterFirst := nextTPLToken(rest, 0)
		if strings.TrimSpace(rest[afterFirst:]) == "" {
			if isBraceWrapped(firstArg) {
				n.ContextExpr = unwrapOuterBraces(firstArg)
				return nil
			}
			if !strings.Contains(firstArg, "=") && !strings.Contains(firstArg, ":") && !strings.HasPrefix(firstArg, ".") && !strings.HasPrefix(firstArg, "#") && !strings.HasPrefix(firstArg, `"`) {
				n.ContextExpr = firstArg
				return nil
			}
		}
	}

	pos := 0
	for pos < len(rest) {
		for pos < len(rest) && (rest[pos] == ' ' || rest[pos] == '\t') {
			pos++
		}
		if pos >= len(rest) {
			break
		}

		if rest[pos] == '[' {
			end := strings.IndexByte(rest[pos:], ']')
			if end < 0 {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: unclosed '[' in attributes", fileName, lineNum)
			}
			rawBlock := rest[pos+1 : pos+end]
			for _, pm := range attrPairRe.FindAllStringSubmatch(rawBlock, -1) {
				n.Attrs = append(n.Attrs, AttrPair{Key: pm[1], Val: `"` + pm[2] + `"`})
			}
			pos += end + 1
			continue
		}

		if rest[pos] == '"' {
			unq, nextPos, err := readQuotedString(rest, pos, fileName, lineNum)
			if err != nil {
				return err
			}
			lit := `"` + unq + `"`
			if n.Kind == NodeComponent {
				n.Attrs = append(n.Attrs, AttrPair{Key: "label", Val: lit})
			} else if n.Text == "" {
				n.Text = lit
			} else {
				n.Text += " " + lit
			}
			pos = nextPos
			continue
		}

		if rest[pos] == '$' || rest[pos] == '(' || rest[pos] == '{' {
			tok, nextPos := nextTPLToken(rest, pos)
			n.Expr = unwrapOuterBraces(tok)
			pos = nextPos
			continue
		}

		tok, nextPos := nextTPLToken(rest, pos)
		pos = nextPos
		if tok == "" {
			break
		}

		if strings.HasPrefix(tok, ".") || strings.HasPrefix(tok, "#") {
			subSel := tok
			if strings.HasPrefix(subSel, ".") {
				subSel = "div" + subSel
			} else {
				subSel = "div" + subSel
			}
			_, subID, subClasses, subCondClasses, err := parseSelectorToken(subSel, fileName, lineNum)
			if err != nil {
				return err
			}
			if subID != "" {
				n.ID = subID
			}
			n.Classes = append(n.Classes, subClasses...)
			n.CondClasses = append(n.CondClasses, subCondClasses...)
			continue
		}

		if strings.HasSuffix(tok, ":") && len(tok) > 1 {
			cond := strings.HasSuffix(tok, "?:")
			key := strings.TrimSuffix(strings.TrimSuffix(tok, ":"), "?")
			valTok, nextValPos := nextTPLToken(rest, pos)
			if valTok == "" {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: missing value after %s", fileName, lineNum, tok)
			}
			if strings.HasPrefix(valTok, `"`) {
				unq, _, err := readQuotedString(valTok, 0, fileName, lineNum)
				if err != nil {
					return err
				}
				valTok = `"` + unq + `"`
			}
			pos = nextValPos
			n.Attrs = append(n.Attrs, AttrPair{Key: key, Val: formatAttrTokenValue(valTok), Conditional: cond})
			continue
		}

		if qIdx := strings.Index(tok, "?:"); qIdx > 0 {
			key := strings.TrimSpace(tok[:qIdx])
			valRaw := strings.TrimSpace(tok[qIdx+2:])
			if strings.HasPrefix(valRaw, `"`) {
				unq, _, err := readQuotedString(valRaw, 0, fileName, lineNum)
				if err != nil {
					return err
				}
				valRaw = `"` + unq + `"`
			}
			n.Attrs = append(n.Attrs, AttrPair{Key: key, Val: formatAttrTokenValue(valRaw), Conditional: true})
			continue
		}

		if eqIdx := strings.IndexByte(tok, '='); eqIdx > 0 {
			key := strings.TrimSpace(tok[:eqIdx])
			valRaw := strings.TrimSpace(tok[eqIdx+1:])
			if strings.HasPrefix(valRaw, `"`) {
				unq, _, err := readQuotedString(valRaw, 0, fileName, lineNum)
				if err != nil {
					return err
				}
				valRaw = `"` + unq + `"`
			}
			n.Attrs = append(n.Attrs, AttrPair{Key: key, Val: formatAttrTokenValue(valRaw), Conditional: false})
			continue
		}

		if colonIdx := strings.IndexByte(tok, ':'); colonIdx > 0 {
			key := strings.TrimSpace(tok[:colonIdx])
			valRaw := strings.TrimSpace(tok[colonIdx+1:])
			if strings.HasPrefix(valRaw, `"`) {
				unq, _, err := readQuotedString(valRaw, 0, fileName, lineNum)
				if err != nil {
					return err
				}
				valRaw = `"` + unq + `"`
			}
			n.Attrs = append(n.Attrs, AttrPair{Key: key, Val: formatAttrTokenValue(valRaw), Conditional: false})
			continue
		}

		return fmt.Errorf("tpl compile error in %s.tpl:%d: unexpected token %q", fileName, lineNum, tok)
	}
	return nil
}

func readQuotedString(s string, start int, file string, line int) (string, int, error) {
	var buf strings.Builder
	i := start + 1
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			buf.WriteByte(s[i+1])
			i += 2
			continue
		}
		if s[i] == '"' {
			return buf.String(), i + 1, nil
		}
		buf.WriteByte(s[i])
		i++
	}
	return "", i, fmt.Errorf("tpl compile error in %s.tpl:%d: unterminated quoted string", file, line)
}

func registerDefsFromNodes(templates map[string]*ParsedTemplate, nodes []*Node) {
	for _, n := range nodes {
		if n.Kind == NodeDef && n.Tag != "" {
			templates[n.Tag] = &ParsedTemplate{
				Name:  n.Tag,
				Nodes: n.Children,
			}
		}
	}
}

// LoadTemplatesFromDir parses all .tpl files in templatesDir (plus optional ../components/*/*.tpl) and resolves external SVGs.
func LoadTemplatesFromDir(templatesDir string, embeddedFS fs.FS) (map[string]*ParsedTemplate, error) {
	templates := make(map[string]*ParsedTemplate)

	entries, err := os.ReadDir(templatesDir)
	registerTplBytes := func(name string, raw []byte, readErr error) error {
		if readErr != nil {
			return readErr
		}
		parsed, err := ParseTemplateString(string(raw), name)
		if err != nil {
			return err
		}
		if err := resolveExternalSVGs(templatesDir, embeddedFS, parsed.Nodes); err != nil {
			return err
		}
		templates[name] = parsed
		registerDefsFromNodes(templates, parsed.Nodes)
		return nil
	}

	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".tpl") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".tpl")
			raw, readErr := os.ReadFile(filepath.Join(templatesDir, e.Name()))
			if err := registerTplBytes(name, raw, readErr); err != nil {
				return nil, err
			}
		}
		for _, compTplPath := range discoverComponentFiles(filepath.Dir(templatesDir), ".tpl") {
			name := strings.TrimSuffix(filepath.Base(compTplPath), ".tpl")
			raw, readErr := os.ReadFile(compTplPath)
			if err := registerTplBytes(name, raw, readErr); err != nil {
				return nil, err
			}
		}
		return templates, nil
	}

	if embeddedFS != nil {
		embEntries, embErr := fs.ReadDir(embeddedFS, "app/templates")
		if embErr != nil {
			return nil, err
		}
		for _, e := range embEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".tpl") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".tpl")
			raw, readErr := fs.ReadFile(embeddedFS, "app/templates/"+e.Name())
			if err := registerTplBytes(name, raw, readErr); err != nil {
				return nil, err
			}
		}
		return templates, nil
	}

	return nil, err
}

func countLeadingIndent(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			n++
		} else {
			break
		}
	}
	return n
}
