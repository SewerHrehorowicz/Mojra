package tpl

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	NodeRaw
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
			expr := unwrapOuterBraces(strings.TrimSpace(trimmed[4:]))
			if expr == "" {
				return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty raw:", name, lineNum)
			}
			if err := appendNode(&Node{Kind: NodeRaw, Expr: expr, Indent: indent, Line: lineNum, File: name}); err != nil {
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

func isBraceWrapped(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
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
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 && i < len(s)-1 {
				return false
			}
		}
	}
	return depth == 0
}

func isParenWrapped(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
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
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 && i < len(s)-1 {
				return false
			}
		}
	}
	return depth == 0
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
		rest := strings.TrimSpace(inner[3:])
		if rest == "" {
			return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: empty {raw}", fileName, lineNum)
		}
		return &Node{Kind: NodeRaw, Expr: rest, Indent: indent, Line: lineNum, File: fileName}, nil

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
	for _, kw := range []string{"def:", "if:", "elif:", "else:", "for:", "with:", "raw:", "svg:", "doctype "} {
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

func parseComponentLine(trimmed string, indent int, fileName string, lineNum int) (*Node, error) {
	firstTok, nextIdx := nextTPLToken(trimmed, 0)
	rawSel := strings.TrimPrefix(firstTok, "@")
	if rawSel == "" {
		return nil, fmt.Errorf("tpl compile error in %s.tpl:%d: missing component name after @", fileName, lineNum)
	}
	compName, id, classes, condClasses, err := parseSelectorToken(rawSel, fileName, lineNum)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Kind:        NodeComponent,
		Tag:         compName,
		IsComponent: true,
		ID:          id,
		Classes:     classes,
		CondClasses: condClasses,
		Indent:      indent,
		Line:        lineNum,
		File:        fileName,
	}
	rest := strings.TrimSpace(trimmed[nextIdx:])
	if rest != "" {
		if err := parseTokensIntoNode(n, rest, fileName, lineNum); err != nil {
			return nil, err
		}
	}
	return n, nil
}

func parseElementLine(trimmed string, indent int, fileName string, lineNum int) (*Node, error) {
	selTok, nextIdx := nextTPLToken(trimmed, 0)
	tag, id, classes, condClasses, err := parseSelectorToken(selTok, fileName, lineNum)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Kind:        NodeElement,
		Tag:         tag,
		ID:          id,
		Classes:     classes,
		CondClasses: condClasses,
		Indent:      indent,
		Line:        lineNum,
		File:        fileName,
	}
	rest := strings.TrimSpace(trimmed[nextIdx:])
	if rest != "" {
		if err := parseTokensIntoNode(n, rest, fileName, lineNum); err != nil {
			return nil, err
		}
	}
	return n, nil
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

func validateNodesRecursive(nodes []*Node) error {
	for i, n := range nodes {
		if n.Kind == NodeElif || n.Kind == NodeElse {
			kw := "else"
			if n.Kind == NodeElif {
				kw = "elif " + n.Expr
			}
			if i == 0 {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: %q must immediately follow an 'if' or 'elif' block", n.File, n.Line, kw)
			}
			prevKind := nodes[i-1].Kind
			if prevKind != NodeIf && prevKind != NodeElif && !(n.Kind == NodeElse && prevKind == NodeWith) {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: %q must immediately follow an 'if' or 'elif' block", n.File, n.Line, kw)
			}
		}
		if n.Kind == NodeElement || n.Kind == NodeComponent {
			if err := validateNoInlineCSSOrJS(n); err != nil {
				return err
			}
		}
		if err := validateNodesRecursive(n.Children); err != nil {
			return err
		}
	}
	return nil
}

func unquoteLiteral(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func validateNoInlineCSSOrJS(n *Node) error {
	if !validTagRe.MatchString(n.Tag) {
		return fmt.Errorf(
			"tpl compile error in %s.tpl:%d: invalid or unsafe tag/component name %q",
			n.File, n.Line, n.Tag,
		)
	}
	if n.ID != "" && !validIDRe.MatchString(n.ID) {
		return fmt.Errorf(
			"tpl compile error in %s.tpl:%d: invalid element id %q",
			n.File, n.Line, n.ID,
		)
	}
	for _, cls := range n.Classes {
		if !validIDRe.MatchString(cls) {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: invalid CSS class name %q",
				n.File, n.Line, cls,
			)
		}
	}
	for _, cc := range n.CondClasses {
		if cc.Name == "" || cc.Cond == "" || !validIDRe.MatchString(cc.Name) {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: invalid conditional CSS class %q",
				n.File, n.Line, cc.Name,
			)
		}
	}

	tagLower := strings.ToLower(n.Tag)
	if !n.IsComponent && tagLower == "button" {
		txt := strings.TrimSpace(unquoteLiteral(n.Text))
		if txt == "" && len(n.Children) == 1 && n.Children[0].Kind == NodeText {
			txt = strings.TrimSpace(unquoteLiteral(n.Children[0].Text))
		}
		if txt == "↑" || txt == "↓" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: up/down reorder arrow buttons (↑/↓) are strictly forbidden",
				n.File, n.Line,
			)
		}
	}
	if !n.IsComponent && tagLower == "style" {
		return fmt.Errorf(
			"tpl compile error in %s.tpl:%d: <style> tags are strictly forbidden in .tpl files; move all CSS rules to app/styles/*.css",
			n.File, n.Line,
		)
	}
	if !n.IsComponent && forbiddenTags[tagLower] {
		return fmt.Errorf(
			"tpl compile error in %s.tpl:%d: <%s> tags are strictly forbidden in .tpl files for security",
			n.File, n.Line, tagLower,
		)
	}

	if !n.IsComponent && tagLower == "script" {
		if n.Text != "" || n.Expr != "" || len(n.Children) > 0 {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline JS inside <script> is strictly forbidden in .tpl files; move all JavaScript to app/js/*.js",
				n.File, n.Line,
			)
		}
		hasSrc := false
		for _, a := range n.Attrs {
			if strings.EqualFold(a.Key, "src") {
				srcVal := strings.TrimSpace(unquoteLiteral(a.Val))
				if srcVal != "" {
					if !strings.HasPrefix(srcVal, "/") || strings.HasPrefix(srcVal, "//") || strings.Contains(srcVal, "..") {
						return fmt.Errorf(
							"tpl compile error in %s.tpl:%d: external or non-root script src %q is strictly forbidden",
							n.File, n.Line, a.Val,
						)
					}
					hasSrc = true
				}
			}
		}
		if !hasSrc {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: <script> without [src=\"...\"] is strictly forbidden in .tpl files; move all JavaScript to app/js/*.js",
				n.File, n.Line,
			)
		}
	}

	if !n.IsComponent && tagLower == "svg" {
		if n.Text != "" || n.Expr != "" || len(n.Children) > 0 {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline <svg> is strictly forbidden in .tpl files; move all SVG icons to app/icons/*.svg and reference them via svg src: /icons/...",
				n.File, n.Line,
			)
		}
		hasSrc := false
		for _, a := range n.Attrs {
			if strings.EqualFold(a.Key, "src") && strings.TrimSpace(unquoteLiteral(a.Val)) != "" {
				hasSrc = true
			}
		}
		if !hasSrc {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: <svg> without src: is strictly forbidden in .tpl files; move all SVG icons to app/icons/*.svg and reference them via svg src: /icons/...",
				n.File, n.Line,
			)
		}
	}

	for _, a := range n.Attrs {
		keyTrimmed := strings.TrimSpace(a.Key)
		if !attrKeyRe.MatchString(keyTrimmed) {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: invalid attribute name %q",
				n.File, n.Line, a.Key,
			)
		}
		keyLower := strings.ToLower(keyTrimmed)
		rawVal := unquoteLiteral(a.Val)
		valNormalized := normalizeAttrValueForSecurityCheck(rawVal)

		if keyLower == "style" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline CSS attribute [style=%q] is strictly forbidden in .tpl files; use semantic CSS classes in app/styles/*.css instead",
				n.File, n.Line, rawVal,
			)
		}
		if keyLower == "srcdoc" || keyLower == "http-equiv" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: unsafe attribute [%s=%q] is strictly forbidden in .tpl files",
				n.File, n.Line, a.Key, rawVal,
			)
		}
		if strings.HasPrefix(keyLower, "on") {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline JS event handler [%s=%q] is strictly forbidden in .tpl files; bind events in app/js/core.js and app/js/events.js instead",
				n.File, n.Line, a.Key, rawVal,
			)
		}
		if strings.HasPrefix(valNormalized, "javascript:") ||
			strings.HasPrefix(valNormalized, "vbscript:") ||
			strings.HasPrefix(valNormalized, "data:") {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: javascript:, vbscript:, or data: URL in [%s=%q] is strictly forbidden in .tpl files",
				n.File, n.Line, a.Key, rawVal,
			)
		}
	}

	return nil
}

func normalizeAttrValueForSecurityCheck(v string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(v) {
		if r <= 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	s := b.String()
	if strings.Contains(s, "&#") || strings.Contains(s, "&colon;") || strings.Contains(s, "&tab;") || strings.Contains(s, "&newline;") {
		return "javascript:obfuscated-entity"
	}
	return s
}

func validateSafeSVGContent(svgContent, file string, line int, src string) error {
	lower := strings.ToLower(svgContent)
	if !strings.HasPrefix(strings.TrimSpace(lower), "<svg") {
		return fmt.Errorf("tpl compile error in %s.tpl:%d: SVG file %q must start with <svg", file, line, src)
	}
	if strings.Contains(lower, "<script") ||
		strings.Contains(lower, "<foreignobject") ||
		strings.Contains(lower, "<iframe") ||
		strings.Contains(lower, "javascript:") ||
		strings.Contains(lower, "data:text/html") ||
		svgEventRe.MatchString(svgContent) {
		return fmt.Errorf("tpl compile error in %s.tpl:%d: unsafe executable content inside SVG %q is strictly forbidden", file, line, src)
	}
	return nil
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
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".tpl") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".tpl")
			raw, err := os.ReadFile(filepath.Join(templatesDir, e.Name()))
			if err != nil {
				return nil, err
			}
			parsed, err := ParseTemplateString(string(raw), name)
			if err != nil {
				return nil, err
			}
			if err := resolveExternalSVGs(templatesDir, embeddedFS, parsed.Nodes); err != nil {
				return nil, err
			}
			templates[name] = parsed
			registerDefsFromNodes(templates, parsed.Nodes)
		}
		for _, compTplPath := range discoverComponentFiles(filepath.Dir(templatesDir), ".tpl") {
			name := strings.TrimSuffix(filepath.Base(compTplPath), ".tpl")
			raw, err := os.ReadFile(compTplPath)
			if err != nil {
				return nil, err
			}
			parsed, err := ParseTemplateString(string(raw), name)
			if err != nil {
				return nil, err
			}
			if err := resolveExternalSVGs(templatesDir, embeddedFS, parsed.Nodes); err != nil {
				return nil, err
			}
			templates[name] = parsed
			registerDefsFromNodes(templates, parsed.Nodes)
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
			raw, err := fs.ReadFile(embeddedFS, "app/templates/"+e.Name())
			if err != nil {
				return nil, err
			}
			parsed, err := ParseTemplateString(string(raw), name)
			if err != nil {
				return nil, err
			}
			if err := resolveExternalSVGs(templatesDir, embeddedFS, parsed.Nodes); err != nil {
				return nil, err
			}
			templates[name] = parsed
			registerDefsFromNodes(templates, parsed.Nodes)
		}
		return templates, nil
	}

	return nil, err
}

func hasLeadingDoctype(nodes []*Node) bool {
	return len(nodes) > 0 && nodes[0].Kind == NodeDoctype
}

// RenderTemplateTo streams a named template directly into w using a pooled buffer.
func RenderTemplateTo(w io.Writer, tplName string, components map[string]*ParsedTemplate, ctx *RenderContext) error {
	tpl, ok := components[tplName]
	if !ok {
		return nil
	}
	if ctx == nil {
		ctx = NewContext(nil)
	}
	buf := AcquireBuffer()
	defer ReleaseBuffer(buf)

	if tplName == "index" && !hasLeadingDoctype(tpl.Nodes) {
		buf.WriteString("<!DOCTYPE html>")
	}
	renderNodesToBuffer(buf, tpl.Nodes, components, ctx, 0)
	if tplName == "index" {
		buf.WriteByte('\n')
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// RenderTemplate renders a named template (e.g. "index" or "app_shell") with the given RenderContext.
func RenderTemplate(tplName string, components map[string]*ParsedTemplate, ctx *RenderContext) string {
	tpl, ok := components[tplName]
	if !ok {
		return ""
	}
	if ctx == nil {
		ctx = NewContext(nil)
	}
	buf := AcquireBuffer()
	defer ReleaseBuffer(buf)

	if tplName == "index" && !hasLeadingDoctype(tpl.Nodes) {
		buf.WriteString("<!DOCTYPE html>")
	}
	renderNodesToBuffer(buf, tpl.Nodes, components, ctx, 0)
	if tplName == "index" {
		buf.WriteByte('\n')
	}
	return buf.String()
}

// TranspileStaticHTML transpiles a root .tpl document and its @components into plain HTML.
func TranspileStaticHTML(tpl *ParsedTemplate, components map[string]*ParsedTemplate) string {
	buf := AcquireBuffer()
	defer ReleaseBuffer(buf)

	if !hasLeadingDoctype(tpl.Nodes) {
		buf.WriteString("<!DOCTYPE html>")
	}
	renderNodesToBuffer(buf, tpl.Nodes, components, NewContext(nil), 0)
	buf.WriteByte('\n')
	return buf.String()
}

const indentSpaces = "                                                                "

func writeLineIndent(buf *bytes.Buffer, indent int) {
	if indent < 0 {
		return
	}
	if buf.Len() > 0 {
		buf.WriteByte('\n')
	}
	spaces := indent * 2
	for spaces > len(indentSpaces) {
		buf.WriteString(indentSpaces)
		spaces -= len(indentSpaces)
	}
	if spaces > 0 {
		buf.WriteString(indentSpaces[:spaces])
	}
}

func renderNodesToBuffer(buf *bytes.Buffer, nodes []*Node, components map[string]*ParsedTemplate, ctx *RenderContext, indent int) {
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]

		switch n.Kind {
		case NodeDoctype:
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
			buf.WriteString("<!DOCTYPE html>")

		case NodeDef:
			continue

		case NodeSlot:
			if ctx != nil && len(ctx.slotNodes) > 0 {
				slotScope := ctx.slotCtx
				if slotScope == nil {
					slotScope = ctx
				}
				renderNodesToBuffer(buf, ctx.slotNodes, components, slotScope, indent)
			}

		case NodeComponent:
			if n.Tag == "slot" {
				if ctx != nil && len(ctx.slotNodes) > 0 {
					slotScope := ctx.slotCtx
					if slotScope == nil {
						slotScope = ctx
					}
					renderNodesToBuffer(buf, ctx.slotNodes, components, slotScope, indent)
				}
				continue
			}
			comp, ok := components[n.Tag]
			if !ok {
				continue
			}
			compCtx := ctx
			if n.ContextExpr != "" && n.ContextExpr != "." && n.ContextExpr != "$" {
				if subVal, exists := resolvePath(ctx, n.ContextExpr); exists {
					compCtx = ctx.WithVar(".", subVal)
				}
			}
			if len(n.Attrs) > 0 || len(n.Classes) > 0 || len(n.CondClasses) > 0 || n.ID != "" || len(n.Children) > 0 {
				var propVars map[string]any
				if len(n.Attrs) > 0 || len(n.Classes) > 0 || len(n.CondClasses) > 0 || n.ID != "" {
					propVars = make(map[string]any, len(n.Attrs)+2)
					if n.ID != "" {
						propVars["id"] = interpolateString(n.ID, ctx)
					}
					if len(n.Classes) > 0 {
						propVars["class"] = interpolateString(strings.Join(n.Classes, " "), ctx)
					}
					for _, cc := range n.CondClasses {
						propVars[cc.Name] = evalCondition(ctx, cc.Cond)
					}
					for _, a := range n.Attrs {
						if a.Conditional && !evalCondition(ctx, unwrapOuterBraces(a.Val)) {
							continue
						}
						propVars[a.Key] = evaluateAttrOrPropAny(a.Val, ctx)
					}
				}
				compCtx = &RenderContext{
					Vars:      propVars,
					parent:    compCtx,
					slotNodes: n.Children,
					slotCtx:   ctx,
				}
				if len(n.Children) == 0 && ctx != nil {
					compCtx.slotNodes = ctx.slotNodes
					compCtx.slotCtx = ctx.slotCtx
				}
			}
			renderNodesToBuffer(buf, comp.Nodes, components, compCtx, indent)

		case NodeIf:
			chainEnd := i
			for chainEnd+1 < len(nodes) && (nodes[chainEnd+1].Kind == NodeElif || nodes[chainEnd+1].Kind == NodeElse) {
				chainEnd++
			}
			for bIdx := i; bIdx <= chainEnd; bIdx++ {
				branch := nodes[bIdx]
				matched := false
				if branch.Kind == NodeElse {
					matched = true
				} else {
					matched = evalCondition(ctx, branch.Expr)
				}
				if matched {
					renderNodesToBuffer(buf, branch.Children, components, ctx, indent)
					break
				}
			}
			i = chainEnd

		case NodeElif, NodeElse:
			continue

		case NodeWith:
			var elseNode *Node
			if i+1 < len(nodes) && nodes[i+1].Kind == NodeElse {
				elseNode = nodes[i+1]
				i++
			}
			val, ok := resolvePath(ctx, n.Expr)
			if ok && isTruthy(val) {
				withCtx := ctx.WithVar(".", val)
				renderNodesToBuffer(buf, n.Children, components, withCtx, indent)
			} else if elseNode != nil {
				renderNodesToBuffer(buf, elseNode.Children, components, ctx, indent)
			}

		case NodeFor:
			rawList, _ := resolvePath(ctx, strings.TrimSuffix(n.Expr, ".each()"))
			itemVar := n.LoopVal
			if itemVar == "" {
				itemVar = "."
			}
			idxVar := n.LoopIdx

			var itemCtx RenderContext
			if ctx != nil {
				itemCtx.parent = ctx
				itemCtx.slotNodes = ctx.slotNodes
				itemCtx.slotCtx = ctx.slotCtx
			}
			itemCtx.localKey = itemVar
			itemCtx.hasLocal = true

			var idxCtx RenderContext
			useIdx := idxVar != ""
			if useIdx {
				idxCtx.parent = &itemCtx
				idxCtx.localKey = idxVar
				idxCtx.hasLocal = true
				if ctx != nil {
					idxCtx.slotNodes = ctx.slotNodes
					idxCtx.slotCtx = ctx.slotCtx
				}
			}

			forEachListItem(rawList, func(idx int, item any) {
				itemCtx.localVal = item
				if useIdx {
					idxCtx.localVal = idx
					renderNodesToBuffer(buf, n.Children, components, &idxCtx, indent)
				} else {
					renderNodesToBuffer(buf, n.Children, components, &itemCtx, indent)
				}
			})

		case NodeText:
			txt := evalTextNodeString(n, ctx)
			if txt != "" {
				writeLineIndent(buf, indent)
				writeEscapedHTML(buf, txt)
			}

		case NodeRaw:
			rawStr := evalRawNodeString(n, ctx)
			if rawStr != "" {
				writeLineIndent(buf, indent)
				buf.WriteString(rawStr)
			}

		case NodeSVG:
			if n.RawSVG != "" {
				writeSVGNodeToBuffer(buf, n, ctx, indent)
			}

		case NodeElement:
			if n.RawSVG != "" {
				writeSVGNodeToBuffer(buf, n, ctx, indent)
				continue
			}
			renderSingleElementNode(buf, n, components, ctx, indent)
		}
	}
}

func evalTextNodeString(n *Node, ctx *RenderContext) string {
	if n.Expr != "" {
		return evaluateValueExpr(n.Expr, ctx)
	}
	if n.Text != "" {
		return evaluateAttrString(n.Text, ctx)
	}
	return ""
}

func evalRawNodeString(n *Node, ctx *RenderContext) string {
	expr := strings.TrimSpace(n.Expr)
	if strings.HasPrefix(expr, `"`) && strings.HasSuffix(expr, `"`) && len(expr) >= 2 {
		return interpolateString(expr[1:len(expr)-1], ctx)
	}
	if v, ok := resolvePath(ctx, expr); ok {
		return valToString(v)
	}
	return evaluateValueExpr(expr, ctx)
}

func renderSingleElementNode(buf *bytes.Buffer, n *Node, components map[string]*ParsedTemplate, ctx *RenderContext, indent int) {
	if n.Tag == "img" {
		for _, a := range n.Attrs {
			if strings.EqualFold(a.Key, "src") {
				resolvedSrc := strings.TrimSpace(evaluateAttrString(a.Val, ctx))
				if resolvedSrc == "" {
					return
				}
			}
		}
	}

	var inlineText string
	var inlineRaw bool
	if n.Expr != "" {
		inlineText = evaluateValueExpr(n.Expr, ctx)
	} else if n.Text != "" {
		inlineText = evaluateAttrString(n.Text, ctx)
	}

	writeLineIndent(buf, indent)
	buf.WriteByte('<')
	buf.WriteString(n.Tag)
	writeElementAttrs(buf, n, ctx)

	if selfClosingTags[n.Tag] {
		if voidWithoutSlashTags[n.Tag] {
			buf.WriteByte('>')
		} else {
			buf.WriteString(" />")
		}
		return
	}

	buf.WriteByte('>')

	if len(n.Children) == 0 {
		if inlineRaw {
			buf.WriteString(inlineText)
		} else {
			writeEscapedHTML(buf, inlineText)
		}
		buf.WriteString("</")
		buf.WriteString(n.Tag)
		buf.WriteByte('>')
		return
	}

	if inlineText == "" && len(n.Children) == 1 && (n.Children[0].Kind == NodeText || n.Children[0].Kind == NodeRaw) {
		ch := n.Children[0]
		if ch.Kind == NodeRaw {
			buf.WriteString(evalRawNodeString(ch, ctx))
		} else {
			writeEscapedHTML(buf, evalTextNodeString(ch, ctx))
		}
		buf.WriteString("</")
		buf.WriteString(n.Tag)
		buf.WriteByte('>')
		return
	}

	childIndent := indent + 1
	if indent < 0 || n.Tag == "p" || n.Tag == "span" || n.Tag == "pre" || n.Tag == "code" || n.Tag == "textarea" || n.ID == "modal-scene-prose-hl" {
		childIndent = -1
	}

	posAfterOpen := buf.Len()
	if inlineText != "" {
		writeLineIndent(buf, childIndent)
		if inlineRaw {
			buf.WriteString(inlineText)
		} else {
			writeEscapedHTML(buf, inlineText)
		}
	}
	posBeforeChildren := buf.Len()
	renderNodesToBuffer(buf, n.Children, components, ctx, childIndent)
	if buf.Len() > posBeforeChildren {
		if childIndent >= 0 {
			writeLineIndent(buf, indent)
		}
		buf.WriteString("</")
		buf.WriteString(n.Tag)
		buf.WriteByte('>')
		return
	}

	buf.Truncate(posAfterOpen)
	if inlineText != "" {
		if inlineRaw {
			buf.WriteString(inlineText)
		} else {
			writeEscapedHTML(buf, inlineText)
		}
	}
	buf.WriteString("</")
	buf.WriteString(n.Tag)
	buf.WriteByte('>')
}

func writeSVGNodeToBuffer(buf *bytes.Buffer, n *Node, ctx *RenderContext, indent int) {
	resolvedSVG := strings.TrimSpace(interpolateString(n.RawSVG, ctx))
	rem := resolvedSVG
	firstLine := true
	for len(rem) > 0 {
		var line string
		if nl := strings.IndexByte(rem, '\n'); nl >= 0 {
			line = strings.TrimSuffix(rem[:nl], "\r")
			rem = rem[nl+1:]
		} else {
			line = strings.TrimSuffix(rem, "\r")
			rem = ""
		}
		writeLineIndent(buf, indent)
		if firstLine && strings.HasPrefix(line, "<svg") && (len(n.Classes) > 0 || len(n.CondClasses) > 0 || n.ID != "") {
			buf.WriteString("<svg")
			writeResolvedClassAndID(buf, n.Classes, n.CondClasses, n.ID, ctx)
			buf.WriteString(line[4:])
		} else {
			buf.WriteString(line)
		}
		firstLine = false
	}
}

func resolveClassSegment(seg string, ctx *RenderContext) string {
	if isBraceWrapped(seg) {
		return evaluateValueExpr(unwrapOuterBraces(seg), ctx)
	}
	return interpolateString(seg, ctx)
}

func writeResolvedClassAndID(buf *bytes.Buffer, rawClasses []string, condClasses []CondClass, rawID string, ctx *RenderContext) {
	wroteClassAttr := false
	writeClassTokens := func(resolved string) {
		i := 0
		for i < len(resolved) {
			for i < len(resolved) && (resolved[i] == ' ' || resolved[i] == '\t') {
				i++
			}
			if i >= len(resolved) {
				break
			}
			start := i
			for i < len(resolved) && resolved[i] != ' ' && resolved[i] != '\t' {
				i++
			}
			tok := resolved[start:i]
			if tok != "" {
				if !wroteClassAttr {
					buf.WriteString(` class="`)
					wroteClassAttr = true
				} else {
					buf.WriteByte(' ')
				}
				writeEscapedHTML(buf, tok)
			}
		}
	}

	for _, c := range rawClasses {
		writeClassTokens(resolveClassSegment(c, ctx))
	}
	for _, cc := range condClasses {
		if !evalCondition(ctx, cc.Cond) {
			continue
		}
		writeClassTokens(resolveClassSegment(cc.Name, ctx))
	}
	if wroteClassAttr {
		buf.WriteByte('"')
	}
	if rawID != "" {
		buf.WriteString(` id="`)
		writeEscapedHTML(buf, resolveClassSegment(rawID, ctx))
		buf.WriteByte('"')
	}
}

func evaluateAttrOrPropAny(raw string, ctx *RenderContext) any {
	trimmed := strings.TrimSpace(raw)
	if isBraceWrapped(trimmed) {
		inner := unwrapOuterBraces(trimmed)
		if v, ok := resolvePath(ctx, inner); ok && !strings.ContainsAny(inner, " \t?:|!=") {
			return v
		}
		return evaluateValueExpr(inner, ctx)
	}
	if strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) && len(trimmed) >= 2 {
		return interpolateString(trimmed[1:len(trimmed)-1], ctx)
	}
	if isParenWrapped(trimmed) || strings.HasPrefix(trimmed, "$") {
		return evaluateValueExpr(trimmed, ctx)
	}
	if trimmed == "true" {
		return true
	}
	if trimmed == "false" {
		return false
	}
	if v, ok := resolvePath(ctx, trimmed); ok {
		return v
	}
	return interpolateString(trimmed, ctx)
}

func evaluateAttrString(raw string, ctx *RenderContext) string {
	trimmed := strings.TrimSpace(raw)
	if isBraceWrapped(trimmed) {
		return evaluateValueExpr(unwrapOuterBraces(trimmed), ctx)
	}
	if strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) && len(trimmed) >= 2 {
		return interpolateString(trimmed[1:len(trimmed)-1], ctx)
	}
	if isParenWrapped(trimmed) || strings.HasPrefix(trimmed, "$") {
		return evaluateValueExpr(trimmed, ctx)
	}
	return interpolateString(trimmed, ctx)
}

func writeElementAttrs(buf *bytes.Buffer, n *Node, ctx *RenderContext) {
	var extraClasses []string
	var extraCondClasses []CondClass
	var resolvedID = n.ID

	for _, a := range n.Attrs {
		if a.Key == "class" {
			if a.Conditional {
				extraCondClasses = append(extraCondClasses, CondClass{Name: a.Val, Cond: unwrapOuterBraces(a.Val)})
			} else {
				extraClasses = append(extraClasses, evaluateAttrString(a.Val, ctx))
			}
		} else if a.Key == "id" && resolvedID == "" {
			if !a.Conditional || evalCondition(ctx, unwrapOuterBraces(a.Val)) {
				resolvedID = evaluateAttrString(a.Val, ctx)
			}
		}
	}

	allClasses := n.Classes
	if len(extraClasses) > 0 {
		allClasses = append(append([]string(nil), extraClasses...), n.Classes...)
	}
	allCondClasses := n.CondClasses
	if len(extraCondClasses) > 0 {
		allCondClasses = append(append([]CondClass(nil), n.CondClasses...), extraCondClasses...)
	}

	writeResolvedClassAndID(buf, allClasses, allCondClasses, resolvedID, ctx)

	for _, a := range n.Attrs {
		if a.Key == "class" || a.Key == "id" {
			continue
		}
		if a.Conditional {
			condExpr := unwrapOuterBraces(a.Val)
			if !evalCondition(ctx, condExpr) {
				continue
			}
			if boolAttrs[a.Key] {
				buf.WriteByte(' ')
				buf.WriteString(a.Key)
				buf.WriteString(`="`)
				buf.WriteString(a.Key)
				buf.WriteByte('"')
				continue
			}
			val := evaluateValueExpr(condExpr, ctx)
			if val == "" {
				continue
			}
			buf.WriteByte(' ')
			buf.WriteString(a.Key)
			buf.WriteString(`="`)
			writeEscapedHTML(buf, val)
			buf.WriteByte('"')
			continue
		}

		val := evaluateAttrString(a.Val, ctx)
		if boolAttrs[a.Key] {
			if val == "false" || val == "0" || (val == "" && (strings.Contains(a.Val, "{") || strings.Contains(a.Val, "$"))) {
				continue
			}
			if val == "" || val == "true" || val == "1" || val == a.Key {
				buf.WriteByte(' ')
				buf.WriteString(a.Key)
				buf.WriteString(`="`)
				buf.WriteString(a.Key)
				buf.WriteByte('"')
				continue
			}
		}
		buf.WriteByte(' ')
		buf.WriteString(a.Key)
		buf.WriteString(`="`)
		writeEscapedHTML(buf, val)
		buf.WriteByte('"')
	}
}

func evaluateValueExpr(raw string, ctx *RenderContext) string {
	expr := strings.TrimSpace(raw)
	for isBraceWrapped(expr) || isParenWrapped(expr) {
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	if expr == "" {
		return ""
	}

	if pipeIdx := findTopLevelSinglePipe(expr); pipeIdx >= 0 {
		left := evaluateValueExpr(expr[:pipeIdx], ctx)
		if left != "" && left != "false" {
			return left
		}
		return evaluateValueExpr(expr[pipeIdx+1:], ctx)
	}

	if qIdx, colonIdx := findTopLevelTernary(expr); qIdx >= 0 && colonIdx > qIdx {
		cond := strings.TrimSpace(expr[:qIdx])
		thenExpr := strings.TrimSpace(expr[qIdx+1 : colonIdx])
		elseExpr := strings.TrimSpace(expr[colonIdx+1:])
		if evalCondition(ctx, cond) {
			return evaluateValueExpr(thenExpr, ctx)
		}
		return evaluateValueExpr(elseExpr, ctx)
	}

	if strings.HasPrefix(expr, "if ") {
		condTok, rest := splitFirstToken(strings.TrimSpace(expr[3:]))
		thenTok, elseRest := splitFirstToken(rest)
		elseTok, _ := splitFirstToken(elseRest)
		if evalCondition(ctx, condTok) {
			return evaluateValueExpr(thenTok, ctx)
		}
		return evaluateValueExpr(elseTok, ctx)
	}

	if strings.HasPrefix(expr, `"`) && strings.HasSuffix(expr, `"`) && len(expr) >= 2 {
		return interpolateString(expr[1:len(expr)-1], ctx)
	}

	if v, ok := resolvePath(ctx, expr); ok {
		return valToString(v)
	}

	if strings.Contains(expr, "{") || strings.Contains(expr, "$") {
		return interpolateString(expr, ctx)
	}
	return ""
}

func findTopLevelSinglePipe(s string) int {
	parenDepth, braceDepth := 0, 0
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
		switch ch {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '|':
			if parenDepth == 0 && braceDepth == 0 {
				if (i+1 < len(s) && s[i+1] == '|') || (i > 0 && s[i-1] == '|') {
					continue
				}
				return i
			}
		}
	}
	return -1
}

func findTopLevelTernary(s string) (int, int) {
	parenDepth, braceDepth := 0, 0
	inQuotes := false
	qIdx := -1
	ternaryDepth := 0
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
		switch ch {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '?':
			if parenDepth == 0 && braceDepth == 0 {
				if i+1 < len(s) && (s[i+1] == ':' || s[i+1] == '=') {
					continue
				}
				if qIdx < 0 {
					qIdx = i
				} else {
					ternaryDepth++
				}
			}
		case ':':
			if parenDepth == 0 && braceDepth == 0 && qIdx >= 0 {
				if ternaryDepth > 0 {
					ternaryDepth--
				} else {
					return qIdx, i
				}
			}
		}
	}
	return -1, -1
}

func splitFirstToken(s string) (string, string) {
	inQuotes := false
	parenDepth := 0
	braceDepth := 0
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
		if ch == '(' {
			parenDepth++
			continue
		}
		if ch == ')' && parenDepth > 0 {
			parenDepth--
			continue
		}
		if ch == '{' {
			braceDepth++
			continue
		}
		if ch == '}' && braceDepth > 0 {
			braceDepth--
			continue
		}
		if parenDepth == 0 && braceDepth == 0 && (ch == ' ' || ch == '\t') {
			return s[:i], strings.TrimSpace(s[i+1:])
		}
	}
	return s, ""
}

func evalCondition(ctx *RenderContext, cond string) bool {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return false
	}
	for isBraceWrapped(cond) || (strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") && isBalancedParens(cond[1:len(cond)-1])) {
		cond = strings.TrimSpace(cond[1 : len(cond)-1])
	}
	if strings.HasPrefix(cond, "not ") {
		return !evalCondition(ctx, cond[4:])
	}
	if strings.HasPrefix(cond, "!") {
		return !evalCondition(ctx, cond[1:])
	}
	if idx := findTopLevelCondOp(cond, "||"); idx >= 0 {
		return evalCondition(ctx, cond[:idx]) || evalCondition(ctx, cond[idx+2:])
	}
	if idx := findTopLevelCondOp(cond, " or "); idx >= 0 {
		return evalCondition(ctx, cond[:idx]) || evalCondition(ctx, cond[idx+4:])
	}
	if idx := findTopLevelCondOp(cond, "&&"); idx >= 0 {
		return evalCondition(ctx, cond[:idx]) && evalCondition(ctx, cond[idx+2:])
	}
	if idx := findTopLevelCondOp(cond, " and "); idx >= 0 {
		return evalCondition(ctx, cond[:idx]) && evalCondition(ctx, cond[idx+5:])
	}
	if strings.HasPrefix(cond, "eq ") || strings.HasPrefix(cond, "ne ") {
		isNe := strings.HasPrefix(cond, "ne ")
		lhs, rhs := splitFirstToken(strings.TrimSpace(cond[3:]))
		if lhs != "" && rhs != "" {
			eq := evalOperandString(ctx, lhs) == evalOperandString(ctx, rhs)
			if isNe {
				return !eq
			}
			return eq
		}
	}
	if idx := findTopLevelCondOp(cond, "=="); idx >= 0 {
		return evalOperandString(ctx, cond[:idx]) == evalOperandString(ctx, cond[idx+2:])
	}
	if idx := findTopLevelCondOp(cond, "!="); idx >= 0 {
		return evalOperandString(ctx, cond[:idx]) != evalOperandString(ctx, cond[idx+2:])
	}
	if cond == "true" || cond == "1" {
		return true
	}
	if cond == "false" || cond == "0" {
		return false
	}
	val, ok := resolvePath(ctx, cond)
	return ok && isTruthy(val)
}

func evalOperandString(ctx *RenderContext, expr string) string {
	expr = strings.TrimSpace(expr)
	if len(expr) >= 2 && ((expr[0] == '"' && expr[len(expr)-1] == '"') || (expr[0] == '\'' && expr[len(expr)-1] == '\'')) {
		return interpolateString(expr[1:len(expr)-1], ctx)
	}
	if v, ok := resolvePath(ctx, expr); ok {
		return valToString(v)
	}
	return expr
}

func isBalancedParens(s string) bool {
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
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func findTopLevelCondOp(s, op string) int {
	depth := 0
	braceDepth := 0
	inQuotes := false
	opLen := len(op)
	for i := 0; i <= len(s)-opLen; i++ {
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
		if ch == '(' {
			depth++
			continue
		}
		if ch == ')' && depth > 0 {
			depth--
			continue
		}
		if ch == '{' {
			braceDepth++
			continue
		}
		if ch == '}' && braceDepth > 0 {
			braceDepth--
			continue
		}
		if depth == 0 && braceDepth == 0 && s[i:i+opLen] == op {
			return i
		}
	}
	return -1
}

func isIdentTokenByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '.'
}

func interpolateString(s string, ctx *RenderContext) string {
	if ctx == nil || (!strings.Contains(s, "{") && !strings.Contains(s, "$")) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	last := 0
	i := 0
	for i < len(s) {
		if s[i] == '{' {
			closeOff := strings.IndexByte(s[i+1:], '}')
			if closeOff >= 0 {
				tokenStart := i
				if tokenStart > last && s[tokenStart-1] == '$' {
					tokenStart--
				}
				b.WriteString(s[last:tokenStart])
				key := strings.TrimSpace(s[i+1 : i+1+closeOff])
				if v, ok := resolvePath(ctx, key); ok {
					b.WriteString(valToString(v))
				}
				i = i + 1 + closeOff + 1
				last = i
				continue
			}
		}
		if s[i] == '$' && i+1 < len(s) && s[i+1] != '{' {
			j := i + 1
			if s[j] == '$' {
				j++
			}
			startIdent := j
			for j < len(s) && isIdentTokenByte(s[j]) {
				j++
			}
			if j > startIdent {
				b.WriteString(s[last:i])
				key := s[i:j]
				if v, ok := resolvePath(ctx, key); ok {
					b.WriteString(valToString(v))
				}
				i = j
				last = i
				continue
			}
		}
		i++
	}
	if last < len(s) {
		b.WriteString(s[last:])
	}
	return b.String()
}

func lookupFieldOnVal(curr any, part string) (any, bool) {
	switch typed := curr.(type) {
	case map[string]any:
		if v, exists := typed[part]; exists {
			return v, true
		}
		for k, v := range typed {
			if strings.EqualFold(k, part) {
				return v, true
			}
		}
	case map[string]string:
		if v, exists := typed[part]; exists {
			return v, true
		}
		for k, v := range typed {
			if strings.EqualFold(k, part) {
				return v, true
			}
		}
	}
	return nil, false
}

func lookupRootVar(ctx *RenderContext, key string) (any, bool) {
	for cur := ctx; cur != nil; cur = cur.parent {
		if cur.hasLocal {
			if cur.localKey == key {
				return cur.localVal, true
			}
			if cur.localKey == "." {
				if v, ok := lookupFieldOnVal(cur.localVal, key); ok {
					return v, true
				}
			}
		}
		if cur.Vars != nil {
			if v, ok := cur.Vars[key]; ok {
				return v, true
			}
			for k, v := range cur.Vars {
				if strings.EqualFold(k, key) {
					return v, true
				}
			}
		}
	}
	return nil, false
}

func resolvePath(ctx *RenderContext, path string) (any, bool) {
	if ctx == nil || path == "" {
		return nil, false
	}
	path = strings.TrimSpace(path)
	if isBraceWrapped(path) {
		path = unwrapOuterBraces(path)
	}
	if path == "." || path == "$" || path == "this" {
		for cur := ctx; cur != nil; cur = cur.parent {
			if cur.hasLocal && cur.localKey == "." {
				return cur.localVal, true
			}
		}
		return ctx.Vars, true
	}
	if path == "$$" || path == "root" || path == "$." {
		for ctx.parent != nil {
			ctx = ctx.parent
		}
		return ctx.Vars, true
	}
	if strings.HasPrefix(path, "$$.") {
		path = path[3:]
		for ctx.parent != nil {
			ctx = ctx.parent
		}
	} else if strings.HasPrefix(path, "$$") {
		path = path[2:]
		for ctx.parent != nil {
			ctx = ctx.parent
		}
	} else if strings.HasPrefix(path, "$.") {
		path = path[2:]
		for ctx.parent != nil {
			ctx = ctx.parent
		}
	} else if strings.HasPrefix(path, "root.") {
		path = path[5:]
		for ctx.parent != nil {
			ctx = ctx.parent
		}
	} else if strings.HasPrefix(path, "$") {
		path = path[1:]
	}
	if isBraceWrapped(path) {
		path = unwrapOuterBraces(path)
	}
	if path == "" {
		return nil, false
	}

	dotIdx := strings.IndexByte(path, '.')
	first := path
	rest := ""
	if dotIdx >= 0 {
		first = path[:dotIdx]
		rest = path[dotIdx+1:]
	}

	curr, ok := lookupRootVar(ctx, first)
	if !ok {
		return nil, false
	}

	for rest != "" {
		dotIdx = strings.IndexByte(rest, '.')
		part := rest
		if dotIdx >= 0 {
			part = rest[:dotIdx]
			rest = rest[dotIdx+1:]
		} else {
			rest = ""
		}
		nextVal, exists := lookupFieldOnVal(curr, part)
		if !exists {
			return nil, false
		}
		curr = nextVal
	}
	return curr, true
}

func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t != "" && t != "false" && t != "0"
	case int:
		return t != 0
	case float64:
		return t != 0
	case []map[string]any:
		return len(t) > 0
	case []any:
		return len(t) > 0
	case []string:
		return len(t) > 0
	}
	return true
}

func forEachListItem(v any, fn func(idx int, item any)) {
	if v == nil {
		return
	}
	switch t := v.(type) {
	case []map[string]any:
		for i := range t {
			fn(i, t[i])
		}
	case []any:
		for i := range t {
			fn(i, t[i])
		}
	case []string:
		for i := range t {
			fn(i, t[i])
		}
	}
}

func valToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// TranspileDirectoryToHTML reads all .tpl files in templatesDir, enforces the no-inline-CSS/JS rule,
// resolves @component includes, and writes the resulting plain HTML file to indexOutPath.
func TranspileDirectoryToHTML(templatesDir, indexOutPath string) error {
	templates, err := LoadTemplatesFromDir(templatesDir, nil)
	if err != nil {
		return err
	}
	idxTpl, ok := templates["index"]
	if !ok {
		return fmt.Errorf("missing index.tpl in %s", templatesDir)
	}

	htmlStr := TranspileStaticHTML(idxTpl, templates)
	return os.WriteFile(indexOutPath, []byte(htmlStr), 0644)
}

func resolveExternalSVGs(templatesDir string, embeddedFS fs.FS, nodes []*Node) error {
	appDir := ""
	if templatesDir != "" {
		appDir = filepath.Dir(templatesDir)
	}
	for _, n := range nodes {
		if (!n.IsComponent && strings.EqualFold(n.Tag, "svg")) || n.Kind == NodeSVG {
			var src string
			if n.Kind == NodeSVG {
				src = strings.TrimSpace(unquoteLiteral(n.Expr))
				if src != "" && !strings.HasPrefix(src, "/") && !strings.HasSuffix(src, ".svg") {
					src = "/icons/" + src + ".svg"
				}
			} else {
				for _, a := range n.Attrs {
					if strings.EqualFold(a.Key, "src") {
						src = strings.TrimSpace(unquoteLiteral(a.Val))
						break
					}
				}
			}
			rel := filepath.FromSlash(strings.TrimPrefix(src, "/"))
			if strings.Contains(rel, "..") {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: path traversal in SVG src %q is forbidden", n.File, n.Line, src)
			}
			var raw []byte
			var err error
			if templatesDir != "" {
				svgPath := filepath.Join(appDir, rel)
				raw, err = os.ReadFile(svgPath)
				if err != nil {
					for _, altPath := range []string{
						filepath.Join(templatesDir, rel),
						filepath.Join(appDir, "app", rel),
					} {
						if altRaw, altErr := os.ReadFile(altPath); altErr == nil {
							raw = altRaw
							err = nil
							break
						}
					}
				}
			} else {
				err = os.ErrNotExist
			}
			if err != nil && embeddedFS != nil {
				if embRaw, embErr := fs.ReadFile(embeddedFS, "app/"+strings.TrimPrefix(src, "/")); embErr == nil {
					raw = embRaw
					err = nil
				}
			}
			if err != nil {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: cannot read SVG file %q: %w", n.File, n.Line, src, err)
			}
			svgContent := strings.TrimSpace(string(raw))
			if err := validateSafeSVGContent(svgContent, n.File, n.Line, src); err != nil {
				return err
			}
			n.RawSVG = svgContent
		}
		if err := resolveExternalSVGs(templatesDir, embeddedFS, n.Children); err != nil {
			return err
		}
	}
	return nil
}

func hasHTMLSpecialChars(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>', '"', '\'':
			return true
		}
	}
	return false
}

func writeEscapedHTML(buf *bytes.Buffer, s string) {
	if s == "" {
		return
	}
	if !hasHTMLSpecialChars(s) {
		buf.WriteString(s)
		return
	}
	last := 0
	for i := 0; i < len(s); i++ {
		var esc string
		switch s[i] {
		case '&':
			esc = "&amp;"
		case '<':
			esc = "&lt;"
		case '>':
			esc = "&gt;"
		case '"':
			esc = "&quot;"
		case '\'':
			esc = "&#x27;"
		default:
			continue
		}
		if i > last {
			buf.WriteString(s[last:i])
		}
		buf.WriteString(esc)
		last = i + 1
	}
	if last < len(s) {
		buf.WriteString(s[last:])
	}
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
