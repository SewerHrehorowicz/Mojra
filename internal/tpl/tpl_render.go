package tpl

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	proseInlineBoldRe   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	proseInlineItalicRe = regexp.MustCompile(`\*([^*]+)\*`)
	proseHTMLTagStripRe = regexp.MustCompile(`<[a-zA-Z/!?][^>]*>`)
)

// SanitizePlainText strips any HTML tags, control characters, and dangerous payloads
// so prose text and metadata are guaranteed to be pure plain text without executable code.
func SanitizePlainText(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	s = proseHTMLTagStripRe.ReplaceAllString(s, "")
	var b strings.Builder
	b.Grow(len(s))
	runes := []rune(s)
	inDoubleBraces := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '{' && i+1 < len(runes) && runes[i+1] == '{' {
			inDoubleBraces++
			b.WriteRune('{')
			b.WriteRune('{')
			i++
			continue
		}
		if r == '}' && i+1 < len(runes) && runes[i+1] == '}' && inDoubleBraces > 0 {
			inDoubleBraces--
			b.WriteRune('}')
			b.WriteRune('}')
			i++
			continue
		}
		if r == '\n' {
			inDoubleBraces = 0
		}
		if r == '<' || r == '>' {
			if inDoubleBraces > 0 {
				b.WriteRune(r)
				continue
			}
			if r == '>' && b.Len() > 0 && strings.HasSuffix(b.String(), "-") {
				b.WriteRune(r)
			} else if r == '>' {
				cur := b.String()
				lastNL := strings.LastIndexByte(cur, '\n')
				linePrefix := cur
				if lastNL >= 0 {
					linePrefix = cur[lastNL+1:]
				}
				if strings.TrimSpace(linePrefix) == "" {
					b.WriteRune(r)
				}
			}
			continue
		}
		if r == '\n' || r == '\t' || r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// RenderSafeProseHTML converts plain text into safe HTML paragraphs (`<p>`, `<strong>`, `<em>`, `<blockquote>`, `<hr>`)
// after escaping all HTML special characters.
func RenderSafeProseHTML(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var blocks []string
	var para []string

	flushPara := func() {
		if len(para) == 0 {
			return
		}
		joined := strings.Join(para, "<br />")
		blocks = append(blocks, "<p>"+joined+"</p>")
		para = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushPara()
			continue
		}
		if trimmed == "---" || trimmed == "***" {
			flushPara()
			blocks = append(blocks, `<hr class="scene-divider" />`)
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			flushPara()
			q := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			blocks = append(blocks, "<blockquote>"+formatProseInlineSafe(q)+"</blockquote>")
			continue
		}
		para = append(para, formatProseInlineSafe(trimmed))
	}
	flushPara()

	if len(blocks) == 0 {
		return "<p></p>"
	}
	return strings.Join(blocks, "\n")
}

func formatProseInlineSafe(s string) string {
	clean := SanitizePlainText(s)
	var eb bytes.Buffer
	writeEscapedHTML(&eb, clean)
	escaped := eb.String()
	escaped = proseInlineBoldRe.ReplaceAllString(escaped, "<strong>$1</strong>")
	escaped = proseInlineItalicRe.ReplaceAllString(escaped, "<em>$1</em>")
	return escaped
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

		case NodeProse:
			proseHTML := evalProseNodeHTML(n, ctx)
			if proseHTML != "" {
				writeLineIndent(buf, indent)
				buf.WriteString(proseHTML)
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

func evalProseNodeHTML(n *Node, ctx *RenderContext) string {
	expr := strings.TrimSpace(n.Expr)
	var plain string
	if strings.HasPrefix(expr, `"`) && strings.HasSuffix(expr, `"`) && len(expr) >= 2 {
		plain = interpolateString(expr[1:len(expr)-1], ctx)
	} else if v, ok := resolvePath(ctx, expr); ok {
		plain = valToString(v)
	} else {
		plain = evaluateValueExpr(expr, ctx)
	}
	return RenderSafeProseHTML(plain)
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
		writeEscapedHTML(buf, inlineText)
		buf.WriteString("</")
		buf.WriteString(n.Tag)
		buf.WriteByte('>')
		return
	}

	if inlineText == "" && len(n.Children) == 1 && (n.Children[0].Kind == NodeText || n.Children[0].Kind == NodeProse) {
		ch := n.Children[0]
		if ch.Kind == NodeProse {
			buf.WriteString(evalProseNodeHTML(ch, ctx))
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
		writeEscapedHTML(buf, inlineText)
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
		writeEscapedHTML(buf, inlineText)
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

func escapeHTMLText(s string) string {
	if !hasHTMLSpecialChars(s) {
		return s
	}
	var buf bytes.Buffer
	buf.Grow(len(s) + 8)
	writeEscapedHTML(&buf, s)
	return buf.String()
}
