package book

import (
	"fmt"
	"strings"
)

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
