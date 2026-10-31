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

type AttrPair struct {
	Key         string
	Val         string
	Conditional bool
}

type Node struct {
	Tag         string
	IsComponent bool
	Classes     []string
	ID          string
	Attrs       []AttrPair
	Text        string
	Mapping     string
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
	attrKeyRe  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)
	validTagRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	validIDRe  = regexp.MustCompile(`^[a-zA-Z0-9_.{}-]+$`)
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
	"required": true,
	"readonly": true,
	"checked":  true,
	"disabled": true,
	"selected": true,
	"multiple": true,
	"download": true,
}

// ParseTemplateString parses .tpl source (including '->' data mappings, conditionals, elif/else, and loops)
// and enforces a strict compile-time check forbidding inline CSS, inline JS, or reorder arrow buttons.
func ParseTemplateString(source, name string) (*ParsedTemplate, error) {
	rawLines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var rootNodes []*Node
	var stack []*Node

	for idx, line := range rawLines {
		lineNum := idx + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		indent := countLeadingIndent(line)
		content := trimmed

		for len(stack) > 0 && stack[len(stack)-1].Indent >= indent {
			stack = stack[:len(stack)-1]
		}

		// Normalize Workflow-style control keywords (if:, elif:, else:, for:, raw:) to canonical mappings
		if norm, ok := normalizeControlKeywordLine(content); ok {
			content = norm
		}

		if isScalarMappingLine(content) {
			if len(stack) == 0 {
				return nil, fmt.Errorf(
					"tpl compile error in %s.tpl:%d: mapping line %q must be indented under an element",
					name, lineNum, content,
				)
			}
			parent := stack[len(stack)-1]
			parent.Mapping = strings.TrimSpace(content[2:])
			continue
		}

		if isAttrLine(content) {
			if len(stack) == 0 {
				return nil, fmt.Errorf(
					"tpl compile error in %s.tpl:%d: attribute or text line %q must be indented under an element",
					name, lineNum, content,
				)
			}
			parent := stack[len(stack)-1]
			attrContent := content
			lineMapping := ""
			if arrowIdx := findUnquotedArrow(attrContent); arrowIdx >= 0 {
				lineMapping = strings.TrimSpace(attrContent[arrowIdx+2:])
				attrContent = strings.TrimSpace(attrContent[:arrowIdx])
			}
			lineAttrs, lineClasses, lineText, err := parseAttrsClassesAndText(attrContent, name, lineNum)
			if err != nil {
				return nil, err
			}
			parent.Attrs = append(parent.Attrs, lineAttrs...)
			parent.Classes = append(parent.Classes, lineClasses...)
			if lineText != "" {
				if parent.Text == "" {
					parent.Text = lineText
				} else {
					parent.Text += " " + lineText
				}
			}
			if lineMapping != "" {
				parent.Mapping = lineMapping
			}
			continue
		}

		mapping := ""
		if arrowIdx := findUnquotedArrow(content); arrowIdx >= 0 {
			mapping = normalizeArrowMapping(strings.TrimSpace(content[arrowIdx+2:]))
			content = strings.TrimSpace(content[:arrowIdx])
		}

		selectorPart, rest := splitFirstToken(content)
		attrs, extraClasses, textContent, err := parseAttrsClassesAndText(rest, name, lineNum)
		if err != nil {
			return nil, err
		}

		tag := "div"
		isComp := false
		var classes []string
		idVal := ""

		if selectorPart != "" {
			parts := splitSelectorParts(selectorPart)
			if len(parts) > 0 && parts[0] != "" && !strings.HasPrefix(selectorPart, "@") {
				tag = strings.TrimSpace(parts[0])
			}
			for i := 1; i+1 < len(parts); i += 2 {
				sep := parts[i]
				val := strings.TrimSpace(parts[i+1])
				if val == "" {
					continue
				}
				switch sep {
				case "@":
					tag = val
					isComp = true
				case ".":
					classes = append(classes, val)
				case "#":
					idVal = val
				}
			}
		}
		classes = append(classes, extraClasses...)

		node := &Node{
			Tag:         tag,
			IsComponent: isComp,
			Classes:     classes,
			ID:          idVal,
			Attrs:       attrs,
			Text:        textContent,
			Mapping:     mapping,
			Indent:      indent,
			Line:        lineNum,
			File:        name,
		}

		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			if strings.EqualFold(parent.Tag, "script") {
				return nil, fmt.Errorf(
					"tpl compile error in %s.tpl:%d: inline JS inside <script> is strictly forbidden in .tpl files; put all JavaScript in app/js/*.js and reference it via script src: /app.js",
					name, lineNum,
				)
			}
			parent.Children = append(parent.Children, node)
		} else {
			rootNodes = append(rootNodes, node)
		}
		stack = append(stack, node)
	}

	if err := validateNodesRecursive(rootNodes); err != nil {
		return nil, err
	}

	return &ParsedTemplate{
		Name:  name,
		Nodes: rootNodes,
	}, nil
}

func normalizeControlKeywordLine(content string) (string, bool) {
	switch {
	case strings.HasPrefix(content, "if:"):
		return "-> if " + strings.TrimSpace(content[3:]), true
	case strings.HasPrefix(content, "elif:"):
		return "-> elif " + strings.TrimSpace(content[5:]), true
	case content == "else:":
		return "-> else", true
	case strings.HasPrefix(content, "for:"):
		spec := strings.TrimSpace(content[4:])
		if inIdx := strings.Index(spec, " in "); inIdx > 0 {
			varsPart := strings.TrimSpace(spec[:inIdx])
			listPart := strings.TrimSpace(spec[inIdx+4:])
			return "-> " + listPart + " as " + varsPart, true
		}
		return "-> " + spec, true
	case strings.HasPrefix(content, "raw:"):
		return "-> html:" + strings.TrimSpace(content[4:]), true
	default:
		return content, false
	}
}

func normalizeArrowMapping(mapping string) string {
	if strings.HasPrefix(mapping, "else if ") {
		return "elif " + strings.TrimSpace(mapping[8:])
	}
	if mapping == "else:" {
		return "else"
	}
	return mapping
}

func isElseOrElifMapping(mapping string) bool {
	return mapping == "else" || strings.HasPrefix(mapping, "elif ")
}

// findUnquotedArrow finds "->" outside of double-quoted strings and parentheses.
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

func validateNodesRecursive(nodes []*Node) error {
	for i, n := range nodes {
		mapping := strings.TrimSpace(n.Mapping)
		if isElseOrElifMapping(mapping) {
			if i == 0 {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: %q must immediately follow an 'if' or 'elif' block", n.File, n.Line, mapping)
			}
			prevMap := strings.TrimSpace(nodes[i-1].Mapping)
			if !strings.HasPrefix(prevMap, "if ") && !strings.HasPrefix(prevMap, "elif ") {
				return fmt.Errorf("tpl compile error in %s.tpl:%d: %q must immediately follow an 'if' or 'elif' block", n.File, n.Line, mapping)
			}
		}
		if err := validateNoInlineCSSOrJS(n); err != nil {
			return err
		}
		if err := validateNodesRecursive(n.Children); err != nil {
			return err
		}
	}
	return nil
}

func splitFirstToken(s string) (string, string) {
	inQuotes := false
	parenDepth := 0
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
		if parenDepth == 0 && (ch == ' ' || ch == '\t') {
			return s[:i], strings.TrimSpace(s[i+1:])
		}
	}
	return s, ""
}

func isScalarMappingLine(content string) bool {
	if !strings.HasPrefix(content, "->") {
		return false
	}
	mapping := normalizeArrowMapping(strings.TrimSpace(content[2:]))
	if mapping == "" {
		return false
	}
	if strings.HasPrefix(mapping, "if ") || strings.HasPrefix(mapping, "elif ") || mapping == "else" || strings.Contains(mapping, " as ") {
		return false
	}
	return true
}

func isControlKeywordName(k string) bool {
	switch k {
	case "if", "elif", "else", "for", "with", "raw", "def":
		return true
	default:
		return false
	}
}

func isAttrLine(content string) bool {
	if strings.HasPrefix(content, "->") {
		return false
	}
	if strings.HasPrefix(content, `"`) || strings.HasPrefix(content, "[") {
		return true
	}
	first, _ := splitFirstToken(content)
	if strings.HasPrefix(first, ".") || strings.HasPrefix(first, "#") || strings.HasPrefix(first, "@") {
		return false
	}
	if strings.HasSuffix(first, "?:") {
		key := strings.TrimSuffix(first, "?:")
		return attrKeyRe.MatchString(key) && !isControlKeywordName(key)
	}
	if strings.HasSuffix(first, ":") {
		key := strings.TrimSuffix(first, ":")
		return attrKeyRe.MatchString(key) && !isControlKeywordName(key)
	}
	if qIdx := strings.Index(first, "?="); qIdx > 0 {
		key := first[:qIdx]
		return attrKeyRe.MatchString(key) && !isControlKeywordName(key)
	}
	if eqIdx := strings.IndexByte(first, '='); eqIdx > 0 {
		key := first[:eqIdx]
		return attrKeyRe.MatchString(key) && !isControlKeywordName(key)
	}
	return false
}

func parseAttrsAndText(rest, file string, line int) ([]AttrPair, string, error) {
	attrs, _, text, err := parseAttrsClassesAndText(rest, file, line)
	return attrs, text, err
}

func parseAttrsClassesAndText(rest, file string, line int) ([]AttrPair, []string, string, error) {
	var attrs []AttrPair
	var extraClasses []string
	var text string
	i := 0
	n := len(rest)

	for i < n {
		for i < n && (rest[i] == ' ' || rest[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}

		if rest[i] == '[' {
			end := strings.IndexByte(rest[i:], ']')
			if end < 0 {
				return nil, nil, "", fmt.Errorf("tpl compile error in %s.tpl:%d: unclosed '[' in attributes", file, line)
			}
			rawBlock := rest[i+1 : i+end]
			for _, pm := range attrPairRe.FindAllStringSubmatch(rawBlock, -1) {
				attrs = append(attrs, AttrPair{Key: pm[1], Val: pm[2]})
			}
			i += end + 1
			continue
		}

		if rest[i] == '"' {
			val, nextIdx, err := readQuotedString(rest, i, file, line)
			if err != nil {
				return nil, nil, "", err
			}
			if text == "" {
				text = val
			} else {
				text += " " + val
			}
			i = nextIdx
			continue
		}

		if rest[i] == '.' {
			tok, nextPos := readTokenWithParensAndQuotes(rest, i)
			if len(tok) > 1 {
				extraClasses = append(extraClasses, tok[1:])
				i = nextPos
				continue
			}
		}

		tokStart := i
		for i < n && rest[i] != ' ' && rest[i] != '\t' && rest[i] != ':' && rest[i] != '=' && !(rest[i] == '?' && i+1 < n && (rest[i+1] == ':' || rest[i+1] == '=')) {
			i++
		}
		key := rest[tokStart:i]
		isCondAttr := false
		hasSep := false
		if i+1 < n && rest[i] == '?' && (rest[i+1] == ':' || rest[i+1] == '=') {
			isCondAttr = true
			hasSep = true
			i += 2
		} else if i < n && (rest[i] == ':' || rest[i] == '=') {
			hasSep = true
			i++
		}

		if hasSep && attrKeyRe.MatchString(key) {
			for i < n && (rest[i] == ' ' || rest[i] == '\t') {
				i++
			}
			if i >= n {
				attrs = append(attrs, AttrPair{Key: key, Val: "", Conditional: isCondAttr})
				break
			}
			if rest[i] == '"' {
				val, nextIdx, err := readQuotedString(rest, i, file, line)
				if err != nil {
					return nil, nil, "", err
				}
				attrs = append(attrs, AttrPair{Key: key, Val: val, Conditional: isCondAttr})
				i = nextIdx
			} else if rest[i] == '(' {
				val, nextIdx := readTokenWithParensAndQuotes(rest, i)
				attrs = append(attrs, AttrPair{Key: key, Val: val, Conditional: isCondAttr})
				i = nextIdx
			} else {
				valStart := i
				for i < n && rest[i] != ' ' && rest[i] != '\t' {
					i++
				}
				attrs = append(attrs, AttrPair{Key: key, Val: rest[valStart:i], Conditional: isCondAttr})
			}
			continue
		}

		return nil, nil, "", fmt.Errorf("tpl compile error in %s.tpl:%d: unexpected token %q", file, line, rest[tokStart:])
	}

	return attrs, extraClasses, text, nil
}

func readTokenWithParensAndQuotes(s string, start int) (string, int) {
	n := len(s)
	i := start
	inQuotes := false
	parenDepth := 0
	for i < n {
		ch := s[i]
		if inQuotes {
			if ch == '\\' && i+1 < n {
				i += 2
				continue
			}
			if ch == '"' {
				inQuotes = false
			}
			i++
			continue
		}
		if ch == '"' {
			inQuotes = true
			i++
			continue
		}
		if ch == '(' {
			parenDepth++
			i++
			continue
		}
		if ch == ')' && parenDepth > 0 {
			parenDepth--
			i++
			continue
		}
		if parenDepth == 0 && (ch == ' ' || ch == '\t') {
			break
		}
		i++
	}
	return s[start:i], i
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
		clsName := cls
		if qIdx := strings.Index(cls, "?="); qIdx >= 0 {
			clsName = cls[:qIdx]
			cond := strings.TrimSpace(cls[qIdx+2:])
			if clsName == "" || cond == "" {
				return fmt.Errorf(
					"tpl compile error in %s.tpl:%d: invalid conditional CSS class %q",
					n.File, n.Line, cls,
				)
			}
		}
		if !validIDRe.MatchString(clsName) {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: invalid CSS class name %q",
				n.File, n.Line, cls,
			)
		}
	}

	tagLower := strings.ToLower(n.Tag)
	if !n.IsComponent && tagLower == "button" {
		txt := strings.TrimSpace(n.Text)
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
		if n.Text != "" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline JS inside <script> is strictly forbidden in .tpl files; move all JavaScript to app/js/*.js",
				n.File, n.Line,
			)
		}
		hasSrc := false
		for _, a := range n.Attrs {
			if strings.EqualFold(a.Key, "src") {
				srcVal := strings.TrimSpace(a.Val)
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
		if n.Text != "" || len(n.Children) > 0 {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline <svg> is strictly forbidden in .tpl files; move all SVG icons to app/icons/*.svg and reference them via svg src: /icons/...",
				n.File, n.Line,
			)
		}
		hasSrc := false
		for _, a := range n.Attrs {
			if strings.EqualFold(a.Key, "src") && strings.TrimSpace(a.Val) != "" {
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
		valNormalized := normalizeAttrValueForSecurityCheck(a.Val)

		if keyLower == "style" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline CSS attribute [style=%q] is strictly forbidden in .tpl files; use semantic CSS classes in app/styles/*.css instead",
				n.File, n.Line, a.Val,
			)
		}
		if keyLower == "srcdoc" || keyLower == "http-equiv" {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: unsafe attribute [%s=%q] is strictly forbidden in .tpl files",
				n.File, n.Line, a.Key, a.Val,
			)
		}
		if strings.HasPrefix(keyLower, "on") {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: inline JS event handler [%s=%q] is strictly forbidden in .tpl files; bind events in app/js/core.js and app/js/events.js instead",
				n.File, n.Line, a.Key, a.Val,
			)
		}
		if strings.HasPrefix(valNormalized, "javascript:") ||
			strings.HasPrefix(valNormalized, "vbscript:") ||
			strings.HasPrefix(valNormalized, "data:") {
			return fmt.Errorf(
				"tpl compile error in %s.tpl:%d: javascript:, vbscript:, or data: URL in [%s=%q] is strictly forbidden in .tpl files",
				n.File, n.Line, a.Key, a.Val,
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
		}
		return templates, nil
	}

	return nil, err
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

	if tplName == "index" {
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

	if tplName == "index" {
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

	buf.WriteString("<!DOCTYPE html>")
	renderNodesToBuffer(buf, tpl.Nodes, components, NewContext(nil), 0)
	buf.WriteByte('\n')
	return buf.String()
}

const indentSpaces = "                                                                "

func writeLineIndent(buf *bytes.Buffer, indent int) {
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

		if n.IsComponent {
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
			if len(n.Attrs) > 0 || len(n.Children) > 0 {
				var propVars map[string]any
				if len(n.Attrs) > 0 {
					propVars = make(map[string]any, len(n.Attrs))
					for _, a := range n.Attrs {
						if a.Conditional && !evalCondition(ctx, a.Val) {
							continue
						}
						if v, exists := resolvePath(ctx, a.Val); exists && !strings.Contains(a.Val, "{") {
							propVars[a.Key] = v
						} else {
							propVars[a.Key] = evaluateValueExpr(a.Val, ctx)
						}
					}
				}
				compCtx = &RenderContext{
					Vars:      propVars,
					parent:    ctx,
					slotNodes: n.Children,
					slotCtx:   ctx,
				}
				if len(n.Children) == 0 && ctx != nil {
					compCtx.slotNodes = ctx.slotNodes
					compCtx.slotCtx = ctx.slotCtx
				}
			}
			renderNodesToBuffer(buf, comp.Nodes, components, compCtx, indent)
			continue
		}

		if n.RawSVG != "" {
			writeSVGNodeToBuffer(buf, n, ctx, indent)
			continue
		}

		mapping := strings.TrimSpace(n.Mapping)

		// 1. Conditional chain: "-> if <cond>", followed by optional sibling "-> elif <cond>" / "-> else"
		if strings.HasPrefix(mapping, "if ") {
			chainEnd := i
			for chainEnd+1 < len(nodes) && isElseOrElifMapping(strings.TrimSpace(nodes[chainEnd+1].Mapping)) {
				chainEnd++
			}
			for bIdx := i; bIdx <= chainEnd; bIdx++ {
				branch := nodes[bIdx]
				bMap := strings.TrimSpace(branch.Mapping)
				matched := false
				if bMap == "else" {
					matched = true
				} else if strings.HasPrefix(bMap, "if ") {
					matched = evalCondition(ctx, bMap[3:])
				} else if strings.HasPrefix(bMap, "elif ") {
					matched = evalCondition(ctx, bMap[5:])
				}
				if matched {
					isInvisible := branch.Tag == "div" && len(branch.Classes) == 0 && branch.ID == "" && len(branch.Attrs) == 0 && branch.Text == ""
					if isInvisible {
						renderNodesToBuffer(buf, branch.Children, components, ctx, indent)
					} else {
						renderSingleElementNode(buf, branch, "", components, ctx, indent)
					}
					break
				}
			}
			i = chainEnd
			continue
		}

		if isElseOrElifMapping(mapping) {
			continue
		}

		// 2. Loop mapping: "-> list.each() as item" or "-> list as item" or "-> list as idx, item"
		if mapping != "" && strings.Contains(mapping, " as ") {
			isInvisibleControlNode := n.Tag == "div" && len(n.Classes) == 0 && n.ID == "" && len(n.Attrs) == 0 && n.Text == ""
			asIdx := strings.Index(mapping, " as ")
			listExpr := strings.TrimSpace(mapping[:asIdx])
			listExpr = strings.TrimSuffix(listExpr, ".each()")
			varsSpec := strings.TrimSpace(mapping[asIdx+4:])
			idxVar := ""
			itemVar := varsSpec
			if commaIdx := strings.IndexByte(varsSpec, ','); commaIdx > 0 {
				idxVar = strings.TrimPrefix(strings.TrimSpace(varsSpec[:commaIdx]), "$")
				itemVar = strings.TrimPrefix(strings.TrimSpace(varsSpec[commaIdx+1:]), "$")
			} else {
				itemVar = strings.TrimPrefix(itemVar, "$")
			}

			rawList, _ := resolvePath(ctx, listExpr)

			if !isInvisibleControlNode {
				writeLineIndent(buf, indent)
				buf.WriteByte('<')
				buf.WriteString(n.Tag)
				writeElementAttrs(buf, n, ctx, "")
				buf.WriteByte('>')
			}
			childIndent := indent
			if !isInvisibleControlNode {
				childIndent = indent + 1
			}
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
					renderNodesToBuffer(buf, n.Children, components, &idxCtx, childIndent)
				} else {
					renderNodesToBuffer(buf, n.Children, components, &itemCtx, childIndent)
				}
			})

			if !isInvisibleControlNode {
				writeLineIndent(buf, indent)
				buf.WriteString("</")
				buf.WriteString(n.Tag)
				buf.WriteByte('>')
			}
			continue
		}

		renderSingleElementNode(buf, n, mapping, components, ctx, indent)
	}
}

func renderSingleElementNode(buf *bytes.Buffer, n *Node, mapping string, components map[string]*ParsedTemplate, ctx *RenderContext, indent int) {
	mappedText := n.Text
	mappedSrc := ""
	isRawHTML := false
	resolvedFromCtx := false
	if mapping != "" {
		resolvedFromCtx = true
		if strings.HasPrefix(mapping, "html:") {
			rawKey := strings.TrimSpace(strings.TrimPrefix(mapping, "html:"))
			if v, ok := resolvePath(ctx, rawKey); ok {
				mappedText = valToString(v)
				isRawHTML = true
			}
		} else if strings.HasPrefix(mapping, `"`) && strings.HasSuffix(mapping, `"`) && len(mapping) >= 2 {
			mappedText = interpolateString(mapping[1:len(mapping)-1], ctx)
		} else if strings.HasPrefix(mapping, "(") && strings.HasSuffix(mapping, ")") {
			mappedText = evaluateValueExpr(mapping, ctx)
		} else if v, ok := resolvePath(ctx, mapping); ok {
			strVal := valToString(v)
			if n.Tag == "img" {
				if strings.TrimSpace(strVal) == "" {
					return
				}
				mappedSrc = strVal
			} else {
				mappedText = strVal
			}
		} else if n.Tag == "img" {
			return
		}
	}

	if strings.Contains(mappedText, "{") && !isRawHTML && !resolvedFromCtx {
		mappedText = interpolateString(mappedText, ctx)
	}

	writeLineIndent(buf, indent)
	buf.WriteByte('<')
	buf.WriteString(n.Tag)
	writeElementAttrs(buf, n, ctx, mappedSrc)

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
		if isRawHTML {
			buf.WriteString(mappedText)
		} else {
			writeEscapedHTML(buf, mappedText)
		}
		buf.WriteString("</")
		buf.WriteString(n.Tag)
		buf.WriteByte('>')
		return
	}

	posAfterOpen := buf.Len()
	if mappedText != "" {
		writeLineIndent(buf, indent+1)
		if isRawHTML {
			buf.WriteString(mappedText)
		} else {
			writeEscapedHTML(buf, mappedText)
		}
	}
	posBeforeChildren := buf.Len()
	renderNodesToBuffer(buf, n.Children, components, ctx, indent+1)
	if buf.Len() > posBeforeChildren {
		writeLineIndent(buf, indent)
		buf.WriteString("</")
		buf.WriteString(n.Tag)
		buf.WriteByte('>')
		return
	}

	buf.Truncate(posAfterOpen)
	if mappedText != "" {
		if isRawHTML {
			buf.WriteString(mappedText)
		} else {
			writeEscapedHTML(buf, mappedText)
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
		if firstLine && strings.HasPrefix(line, "<svg") && (len(n.Classes) > 0 || n.ID != "") {
			buf.WriteString("<svg")
			writeResolvedClassAndID(buf, n.Classes, n.ID, ctx)
			buf.WriteString(line[4:])
		} else {
			buf.WriteString(line)
		}
		firstLine = false
	}
}

func writeResolvedClassAndID(buf *bytes.Buffer, rawClasses []string, rawID string, ctx *RenderContext) {
	wroteClassAttr := false
	for _, c := range rawClasses {
		clsPart := c
		if qIdx := strings.Index(c, "?="); qIdx > 0 {
			cond := c[qIdx+2:]
			if !evalCondition(ctx, cond) {
				continue
			}
			clsPart = c[:qIdx]
		}
		resolved := interpolateString(clsPart, ctx)
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
	if wroteClassAttr {
		buf.WriteByte('"')
	}
	if rawID != "" {
		buf.WriteString(` id="`)
		writeEscapedHTML(buf, interpolateString(rawID, ctx))
		buf.WriteByte('"')
	}
}

func writeElementAttrs(buf *bytes.Buffer, n *Node, ctx *RenderContext, extraSrc string) {
	writeResolvedClassAndID(buf, n.Classes, n.ID, ctx)
	if extraSrc != "" {
		buf.WriteString(` src="`)
		writeEscapedHTML(buf, extraSrc)
		buf.WriteByte('"')
	}
	for _, a := range n.Attrs {
		if a.Conditional {
			if !evalCondition(ctx, a.Val) {
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
			val := evaluateValueExpr(a.Val, ctx)
			if v, ok := resolvePath(ctx, strings.TrimSpace(a.Val)); ok && !strings.Contains(a.Val, "{") {
				val = valToString(v)
			}
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

		val := evaluateValueExpr(a.Val, ctx)
		if boolAttrs[a.Key] {
			if val == "false" || val == "0" || (val == "" && strings.Contains(a.Val, "{")) {
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
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") && len(trimmed) > 2 {
		inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		if strings.HasPrefix(inner, "if ") {
			condTok, rest := splitFirstToken(strings.TrimSpace(inner[3:]))
			thenTok, elseRest := splitFirstToken(rest)
			elseTok, _ := splitFirstToken(elseRest)
			var chosen string
			if evalCondition(ctx, condTok) {
				chosen = thenTok
			} else {
				chosen = elseTok
			}
			chosen = strings.TrimSpace(chosen)
			if strings.HasPrefix(chosen, `"`) && strings.HasSuffix(chosen, `"`) && len(chosen) >= 2 {
				return interpolateString(chosen[1:len(chosen)-1], ctx)
			}
			if v, ok := resolvePath(ctx, chosen); ok {
				return valToString(v)
			}
			return interpolateString(chosen, ctx)
		}
	}
	return interpolateString(raw, ctx)
}

func evalCondition(ctx *RenderContext, cond string) bool {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return false
	}
	for strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") && isBalancedParens(cond[1:len(cond)-1]) {
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
		if depth == 0 && s[i:i+opLen] == op {
			return i
		}
	}
	return -1
}

func interpolateString(s string, ctx *RenderContext) string {
	openIdx := strings.IndexByte(s, '{')
	if openIdx < 0 || ctx == nil {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	last := 0
	i := openIdx
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
		i++
	}
	if last < len(s) {
		b.WriteString(s[last:])
	}
	return b.String()
}

func lookupRootVar(ctx *RenderContext, key string) (any, bool) {
	for cur := ctx; cur != nil; cur = cur.parent {
		if cur.hasLocal && cur.localKey == key {
			return cur.localVal, true
		}
		if cur.Vars != nil {
			if v, ok := cur.Vars[key]; ok {
				return v, true
			}
		}
	}
	return nil, false
}

func resolvePath(ctx *RenderContext, path string) (any, bool) {
	if ctx == nil || path == "" {
		return nil, false
	}
	if strings.HasPrefix(path, "$$.") {
		path = path[3:]
		for ctx.parent != nil {
			ctx = ctx.parent
		}
	} else if strings.HasPrefix(path, "$") {
		path = path[1:]
	}
	if strings.HasPrefix(path, "{") && strings.HasSuffix(path, "}") && len(path) >= 2 {
		path = strings.TrimSpace(path[1 : len(path)-1])
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
		switch typed := curr.(type) {
		case map[string]any:
			v, exists := typed[part]
			if !exists {
				return nil, false
			}
			curr = v
		case map[string]string:
			v, exists := typed[part]
			if !exists {
				return nil, false
			}
			curr = v
		default:
			return nil, false
		}
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
		if !n.IsComponent && strings.EqualFold(n.Tag, "svg") {
			var src string
			for _, a := range n.Attrs {
				if strings.EqualFold(a.Key, "src") {
					src = strings.TrimSpace(a.Val)
					break
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
					altPath := filepath.Join(templatesDir, rel)
					if altRaw, altErr := os.ReadFile(altPath); altErr == nil {
						raw = altRaw
						err = nil
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

func escapeStaticHTML(s string) string {
	if !hasHTMLSpecialChars(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
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
			b.WriteString(s[last:i])
		}
		b.WriteString(esc)
		last = i + 1
	}
	if last < len(s) {
		b.WriteString(s[last:])
	}
	return b.String()
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

func splitSelectorParts(content string) []string {
	var parts []string
	last := 0
	braceDepth := 0
	parenDepth := 0
	inCond := false
	condDots := 0

	for i := 0; i < len(content); i++ {
		ch := content[i]
		if ch == '{' {
			braceDepth++
			continue
		}
		if ch == '}' {
			if braceDepth > 0 {
				braceDepth--
			}
			continue
		}
		if ch == '(' {
			parenDepth++
			continue
		}
		if ch == ')' {
			if parenDepth > 0 {
				parenDepth--
			}
			continue
		}
		if braceDepth == 0 && parenDepth == 0 {
			if ch == '?' && i+1 < len(content) && content[i+1] == '=' {
				inCond = true
				condDots = 0
				i++
				continue
			}
			if ch == '#' || ch == '@' {
				inCond = false
				condDots = 0
				parts = append(parts, content[last:i])
				parts = append(parts, string(ch))
				last = i + 1
				continue
			}
			if ch == '.' {
				if inCond {
					nextSep := len(content)
					for j := i + 1; j < len(content); j++ {
						if content[j] == '.' || content[j] == '#' || content[j] == '@' {
							nextSep = j
							break
						}
					}
					nextSeg := content[i+1 : nextSep]
					if condDots == 0 && nextSeg != "" && !strings.Contains(nextSeg, "?=") && !strings.Contains(nextSeg, "-") && !strings.Contains(nextSeg, "{") {
						condDots++
						continue
					}
				}
				inCond = false
				condDots = 0
				parts = append(parts, content[last:i])
				parts = append(parts, ".")
				last = i + 1
			}
		}
	}
	parts = append(parts, content[last:])
	return parts
}
