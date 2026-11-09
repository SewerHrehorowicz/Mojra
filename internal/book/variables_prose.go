package book

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
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

// EvaluateSceneProseText evaluates dynamic blocks (if:/elif:/else:/endif:) and inline variables ({{var}})
// on a scene's plain text without converting to HTML.
func EvaluateSceneProseText(sc *Scene, state map[string]string, b *Book) string {
	if sc == nil {
		return ""
	}
	if !HasDynamicProse(sc.Text) {
		return sc.Text
	}
	return EvaluateProseText(sc.Text, state, b)
}

type proseBlockFrame struct {
	branchTaken  bool
	branchActive bool
}

func isProseBlockStackActive(stack []proseBlockFrame) bool {
	for _, f := range stack {
		if !f.branchActive {
			return false
		}
	}
	return true
}

func stepProseBlockStack(stack *[]proseBlockFrame, kind, condExpr string, state map[string]string, b *Book) {
	s := *stack
	switch kind {
	case "endif":
		if len(s) > 0 {
			*stack = s[:len(s)-1]
		}
	case "else":
		if len(s) > 0 {
			top := &(*stack)[len(s)-1]
			top.branchActive = !top.branchTaken
			top.branchTaken = true
		}
	case "elif":
		if len(s) > 0 {
			top := &(*stack)[len(s)-1]
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
		*stack = append(s, proseBlockFrame{branchTaken: met, branchActive: met})
	}
}

// EvaluateProseText evaluates multi-paragraph if:/elif:/else:/endif: blocks and inline {{...}} tokens,
// returning plain text ready for RenderSafeProseHTML.
func EvaluateProseText(raw string, state map[string]string, b *Book) string {
	if !HasDynamicProse(raw) {
		return raw
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var stack []proseBlockFrame

	var outLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if kind, condExpr, ok := parseProseBlockDirective(trimmed, state, b); ok {
			stepProseBlockStack(&stack, kind, condExpr, state, b)
			continue
		}
		if _, _, _, isVoice := parseInlineVoiceSectionLine(trimmed); isVoice {
			continue
		}
		if !isProseBlockStackActive(stack) {
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
	case "elif":
		if val != "" {
			return "elif", val, true
		}
	case "if":
		if val != "" {
			return "if", val, true
		}
	case "set":
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

	var stack []proseBlockFrame

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if kind, condExpr, ok := parseProseBlockDirective(trimmed, state, b); ok {
			stepProseBlockStack(&stack, kind, condExpr, state, b)
			appendSeg(line, true)
		} else if trimmed == "---" || trimmed == "***" || !isProseBlockStackActive(stack) {
			appendSeg(line, true)
		} else {
			rest := line
			for maxTok := len(line) + 1; maxTok > 0; maxTok-- {
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

func evaluateInlineProseTokens(line string, state map[string]string, b *Book) string {
	var sb strings.Builder
	rest := line
	for maxTok := len(line) + 1; maxTok > 0; maxTok-- {
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
	var stack []proseBlockFrame
	blockState := func() (inBlock bool, active bool) {
		if len(stack) == 0 {
			return false, true
		}
		return true, isProseBlockStackActive(stack)
	}

	var out []map[string]any
	appendSyntaxLine := func(text string, inB, act bool) {
		out = append(out, map[string]any{
			"is_divider":  false,
			"is_active":   isHighlight && inB && act,
			"is_inactive": isHighlight && inB && !act,
			"segments": []map[string]any{
				{"text": text, "is_syntax": true},
			},
		})
	}
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
			if kind == "endif" {
				inB, act := blockState()
				stepProseBlockStack(&stack, kind, condExpr, state, b)
				appendSyntaxLine(trimmed, inB, act)
			} else {
				stepProseBlockStack(&stack, kind, condExpr, state, b)
				inB, act := blockState()
				appendSyntaxLine(trimmed, inB, act)
			}
			continue
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
	for maxTok := len(line) + 1; maxTok > 0; maxTok-- {
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

// HasConditionalVoiceDirectives reports whether raw prose text contains any if:/elif:/else:/endif: block directives.
func HasConditionalVoiceDirectives(raw string) bool {
	if raw == "" {
		return false
	}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if kind, _, ok := parseProseBlockDirective(strings.TrimSpace(line), nil, nil); ok {
			switch kind {
			case "if", "elif", "else", "endif":
				return true
			}
		}
	}
	return false
}

// SceneVoiceBaseStem returns the deterministic ASCII/underscore filename stem for a scene's voiceover files.
func SceneVoiceBaseStem(sc *Scene) string {
	if sc == nil {
		return "scena"
	}
	for _, candidate := range []string{sc.ExplicitID, sc.Name, sc.ID} {
		if stem := SanitizeChapterID(candidate); stem != "" {
			return stem
		}
	}
	return "scena"
}

func parseVoiceTrimHeader(val string) (float64, float64, bool) {
	clean := strings.TrimSpace(val)
	clean = strings.TrimPrefix(clean, "[")
	clean = strings.TrimSuffix(clean, "]")
	clean = strings.ReplaceAll(clean, "..", ",")
	clean = strings.ReplaceAll(clean, "-", ",")
	parts := strings.Split(clean, ",")
	if len(parts) != 2 {
		parts = strings.Fields(clean)
	}
	if len(parts) != 2 {
		return 0, 1, false
	}
	s, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	e, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 1, false
	}
	if s < 0 {
		s = 0
	}
	if e > 1 {
		e = 1
	}
	if e <= s+0.001 {
		return 0, 1, false
	}
	return s, e, true
}

// ParseVoiceHeaderValue parses a voice header value such as "voice/scene__02_if.flac [0.0420..0.9610]"
// into its relative file path and optional [trimStart, trimEnd] ratios.
func ParseVoiceHeaderValue(val string) (string, float64, float64) {
	trimmed := strings.TrimSpace(val)
	if strings.HasSuffix(trimmed, "]") {
		if openIdx := strings.LastIndexByte(trimmed, '['); openIdx > 0 {
			pathPart := strings.TrimSpace(trimmed[:openIdx])
			bracketPart := strings.TrimSpace(trimmed[openIdx+1 : len(trimmed)-1])
			if ts, te, ok := parseVoiceTrimHeader(bracketPart); ok {
				return pathPart, ts, te
			}
		}
	}
	return trimmed, 0, 0
}

func parseInlineVoiceSectionLine(trimmed string) (string, float64, float64, bool) {
	if trimmed == "" {
		return "", 0, 0, false
	}
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return "", 0, 0, false
	}
	key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
	val := strings.TrimSpace(trimmed[idx+1:])
	switch key {
	case "voice":
		vPath, tStart, tEnd := ParseVoiceHeaderValue(val)
		clean := SanitizeRelativeMediaPath(vPath)
		if clean == "" {
			return "", 0, 0, false
		}
		return clean, tStart, tEnd, true
	default:
		return "", 0, 0, false
	}
}

type rawParsedVoiceSection struct {
	kind      string
	cond      string
	lines     []string
	voicePath string
	trimStart float64
	trimEnd   float64
	hasVoice  bool
}

func splitRawVoiceSections(raw string) ([]rawParsedVoiceSection, []string) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var rawSecs []rawParsedVoiceSection
	var cleanedProseLines []string

	curKind := "static"
	curCond := ""
	var curLines []string
	var curVoicePath string
	var curTrimStart, curTrimEnd float64
	var curHasVoice bool

	flush := func(force bool) {
		joined := strings.TrimSpace(strings.Join(curLines, "\n"))
		if curKind == "static" && joined == "" && !curHasVoice && !force {
			curLines = nil
			return
		}
		rawSecs = append(rawSecs, rawParsedVoiceSection{
			kind:      curKind,
			cond:      curCond,
			lines:     append([]string(nil), curLines...),
			voicePath: curVoicePath,
			trimStart: curTrimStart,
			trimEnd:   curTrimEnd,
			hasVoice:  curHasVoice,
		})
		curLines = nil
		curVoicePath = ""
		curTrimStart = 0
		curTrimEnd = 0
		curHasVoice = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if kind, condExpr, ok := parseProseBlockDirective(trimmed, nil, nil); ok {
			switch kind {
			case "if":
				flush(false)
				curKind = "if"
				curCond = condExpr
				cleanedProseLines = append(cleanedProseLines, line)
				continue
			case "elif":
				flush(true)
				curKind = "elif"
				curCond = condExpr
				cleanedProseLines = append(cleanedProseLines, line)
				continue
			case "else":
				flush(true)
				curKind = "else"
				curCond = ""
				cleanedProseLines = append(cleanedProseLines, line)
				continue
			case "endif":
				flush(true)
				curKind = "static"
				curCond = ""
				cleanedProseLines = append(cleanedProseLines, line)
				continue
			case "set":
				cleanedProseLines = append(cleanedProseLines, line)
				continue
			}
		}
		if vPath, tStart, tEnd, isVoice := parseInlineVoiceSectionLine(trimmed); isVoice {
			curVoicePath = vPath
			curTrimStart = tStart
			curTrimEnd = tEnd
			curHasVoice = true
			continue
		}
		curLines = append(curLines, line)
		cleanedProseLines = append(cleanedProseLines, line)
	}
	flush(false)
	return rawSecs, cleanedProseLines
}

func formatVoiceSectionKeyAndLabels(idx, total int, kind, cond string) (key, label, shortLabel, headerTitle string) {
	switch kind {
	case "if":
		condUpper := strings.ToUpper(strings.TrimSpace(cond))
		key = fmt.Sprintf("%02d_if", idx)
		label = fmt.Sprintf("%02d · IF: %s", idx, condUpper)
		shortLabel = fmt.Sprintf("%02d · IF", idx)
		headerTitle = fmt.Sprintf("SEKCJA %02d / %02d · IF: %s", idx, total, condUpper)
	case "elif":
		condUpper := strings.ToUpper(strings.TrimSpace(cond))
		key = fmt.Sprintf("%02d_elif", idx)
		label = fmt.Sprintf("%02d · ELIF: %s", idx, condUpper)
		shortLabel = fmt.Sprintf("%02d · ELIF", idx)
		headerTitle = fmt.Sprintf("SEKCJA %02d / %02d · ELIF: %s", idx, total, condUpper)
	case "else":
		key = fmt.Sprintf("%02d_else", idx)
		label = fmt.Sprintf("%02d · ELSE", idx)
		shortLabel = fmt.Sprintf("%02d · ELSE", idx)
		headerTitle = fmt.Sprintf("SEKCJA %02d / %02d · ELSE", idx, total)
	default:
		suffix := "TEKST"
		if idx == 1 {
			suffix = "WSTĘP"
		} else if idx == total {
			suffix = "FINAŁ"
		}
		key = fmt.Sprintf("%02d", idx)
		label = fmt.Sprintf("%02d · %s", idx, suffix)
		shortLabel = fmt.Sprintf("%02d", idx)
		headerTitle = fmt.Sprintf("SEKCJA %02d / %02d · %s", idx, total, suffix)
	}
	return
}

func formatDurationSecondsLabel(durSec float64) string {
	if durSec <= 0 {
		return "0:00"
	}
	secs := int(math.Round(durSec))
	if secs < 0 {
		secs = 0
	}
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

func discoverBookAudioFilesForSections(b *Book) []string {
	if b == nil || !b.Narrated {
		return nil
	}
	if b.IsPacked {
		if len(b.AudioFiles) > 0 {
			return b.AudioFiles
		}
		if len(b.PackedMedia) > 0 {
			out := make([]string, 0, len(b.PackedMedia))
			for k := range b.PackedMedia {
				if isAudioFilename(k) {
					out = append(out, k)
				}
			}
			return out
		}
		return nil
	}
	seen := make(map[string]bool, len(b.AudioFiles)+8)
	var out []string
	add := func(rel string) {
		clean := SanitizeRelativeMediaPath(rel)
		if clean == "" || !isAudioFilename(clean) {
			return
		}
		lower := strings.ToLower(clean)
		if seen[lower] {
			return
		}
		seen[lower] = true
		out = append(out, clean)
	}
	for _, f := range b.AudioFiles {
		add(f)
	}
	if strings.TrimSpace(b.RootPath) != "" {
		voiceDir := filepath.Join(b.RootPath, "voice")
		if entries, err := os.ReadDir(voiceDir); err == nil {
			for _, e := range entries {
				if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				add(filepath.ToSlash(filepath.Join("voice", e.Name())))
			}
		}
	}
	b.AudioFiles = out
	return out
}

// NormalizeSceneInlineSectionVoices extracts any inline "voice:" lines inside conditional blocks into sc.SectionVoices and strips them from sc.Text.
func NormalizeSceneInlineSectionVoices(sc *Scene) {
	if sc == nil || !HasConditionalVoiceDirectives(sc.Text) {
		return
	}
	rawSecs, cleanedLines := splitRawVoiceSections(sc.Text)
	total := len(rawSecs)
	if sc.ExplicitVoice != "" || sc.HasVoiceTrim() {
		firstKey := "01"
		if total > 0 {
			firstKey, _, _, _ = formatVoiceSectionKeyAndLabels(1, total, rawSecs[0].kind, rawSecs[0].cond)
		}
		sc.SetSectionVoiceMeta(firstKey, sc.ExplicitVoice, sc.VoiceTrimStart, sc.VoiceTrimEnd)
		sc.ExplicitVoice = ""
		sc.VoiceTrimStart = 0
		sc.VoiceTrimEnd = 0
	}
	hasInlineVoice := false
	for i, rs := range rawSecs {
		if rs.hasVoice {
			hasInlineVoice = true
			key, _, _, _ := formatVoiceSectionKeyAndLabels(i+1, total, rs.kind, rs.cond)
			sc.SetSectionVoiceMeta(key, rs.voicePath, rs.trimStart, rs.trimEnd)
		}
	}
	if hasInlineVoice {
		sc.Text = strings.Join(cleanedLines, "\n")
	}
}

// ExtractSceneVoiceSections splits a scene's prose AST into ordered static and conditional voiceover sections.
// If the scene has no conditional directives (if:/elif:/else:/endif:), it returns a single section for the entire scene.
func ExtractSceneVoiceSections(sc *Scene, b *Book) []VoiceSection {
	if sc == nil {
		return nil
	}
	NormalizeSceneInlineSectionVoices(sc)
	baseStem := SceneVoiceBaseStem(sc)
	audioCandidates := discoverBookAudioFilesForSections(b)

	if !HasConditionalVoiceDirectives(sc.Text) {
		targetFile := filepath.ToSlash(filepath.Join("voice", baseStem+".flac"))
		expVoice := sc.ExplicitVoice
		ts, te := sc.NormalizedVoiceTrim()
		if sc.SectionVoices != nil {
			if meta, ok := sc.SectionVoices["01"]; ok {
				if expVoice == "" && meta.ExplicitVoice != "" {
					expVoice = meta.ExplicitVoice
				}
				if !sc.HasVoiceTrim() && HasVoiceTrimValues(meta.TrimStart, meta.TrimEnd) {
					ts, te = NormalizedVoiceTrimValues(meta.TrimStart, meta.TrimEnd)
				}
			}
		}
		audioFile := sc.AudioFile
		audioURL := sc.AudioURL
		var durSec float64
		durLbl := "—"
		hasAudio := false
		if b != nil && b.Narrated {
			if audioFile == "" {
				if expVoice != "" {
					if matched, ok := findMediaFile(audioCandidates, expVoice, "", ""); ok {
						audioFile = matched
					}
				} else if matched, ok := findMediaFile(audioCandidates, targetFile, sc.ID, sc.Name); ok {
					audioFile = matched
				}
			}
			if audioFile != "" {
				rawDur, stamp := ReadAudioDurationSecondsAndStamp(b, audioFile)
				if audioURL == "" {
					audioURL = encodeMediaURL(b.ID, audioFile)
				}
				if stamp > 0 && !strings.Contains(audioURL, "?v=") {
					audioURL = fmt.Sprintf("%s?v=%d", audioURL, stamp)
				}
				durSec = rawDur * (te - ts)
				durLbl = formatDurationSecondsLabel(durSec)
				hasAudio = true
			}
		}
		shownFile := targetFile
		if audioFile != "" {
			shownFile = audioFile
		} else if expVoice != "" {
			shownFile = expVoice
		}
		sec := VoiceSection{
			Index:         1,
			Key:           "01",
			Kind:          "static",
			Condition:     "",
			Label:         "01 · WSTĘP",
			ShortLabel:    "01",
			HeaderTitle:   "SEKCJA 01 / 01 · WSTĘP",
			Text:          sc.Text,
			ExplicitVoice: expVoice,
			TargetFile:    shownFile,
			AudioFile:     audioFile,
			AudioURL:      audioURL,
			TrimStart:     ts,
			TrimEnd:       te,
			DurationSec:   durSec,
			DurationLabel: durLbl,
			HasAudio:      hasAudio,
			HasTrim:        hasAudio && HasVoiceTrimValues(ts, te),
		}
		sc.VoiceSections = []VoiceSection{sec}
		return sc.VoiceSections
	}

	rawSecs, _ := splitRawVoiceSections(sc.Text)
	total := len(rawSecs)
	sections := make([]VoiceSection, 0, total)
	idStem := SanitizeChapterID(sc.ID)
	nameStem := SanitizeChapterID(sc.Name)

	for i, rs := range rawSecs {
		idx := i + 1
		key, label, shortLabel, headerTitle := formatVoiceSectionKeyAndLabels(idx, total, rs.kind, rs.cond)
		targetFile := fmt.Sprintf("voice/%s__%s.flac", baseStem, key)

		var expVoice string
		ts, te := 0.0, 1.0
		if sc.SectionVoices != nil {
			if meta, ok := sc.SectionVoices[strings.ToLower(key)]; ok {
				expVoice = meta.ExplicitVoice
				ts, te = NormalizedVoiceTrimValues(meta.TrimStart, meta.TrimEnd)
			}
		}
		if idx == 1 && expVoice == "" && sc.ExplicitVoice != "" {
			// If top-level ExplicitVoice points to section 01 (or was set on the scene), associate with section 01
			lowerExp := strings.ToLower(sc.ExplicitVoice)
			if strings.Contains(lowerExp, "__01.") || !strings.Contains(lowerExp, "__") {
				expVoice = sc.ExplicitVoice
				if !HasVoiceTrimValues(ts, te) && sc.HasVoiceTrim() {
					ts, te = sc.NormalizedVoiceTrim()
				}
			}
		}

		var audioFile, audioURL string
		var durSec float64
		durLbl := "—"
		hasAudio := false

		if b != nil && b.Narrated {
			if expVoice != "" {
				if matched, ok := findMediaFile(audioCandidates, expVoice, "", ""); ok {
					audioFile = matched
				}
			}
			if audioFile == "" {
				idSecKey := ""
				if idStem != "" {
					idSecKey = idStem + "__" + key
				}
				nameSecKey := ""
				if nameStem != "" {
					nameSecKey = nameStem + "__" + key
				}
				if matched, ok := findMediaFile(audioCandidates, targetFile, idSecKey, nameSecKey); ok {
					audioFile = matched
				} else if idSecKey != "" {
					if matched, ok := findMediaFile(audioCandidates, "", idSecKey, nameSecKey); ok {
						audioFile = matched
					}
				}
			}
			if audioFile != "" {
				rawDur, stamp := ReadAudioDurationSecondsAndStamp(b, audioFile)
				audioURL = encodeMediaURL(b.ID, audioFile)
				if stamp > 0 {
					audioURL = fmt.Sprintf("%s?v=%d", audioURL, stamp)
				}
				durSec = rawDur * (te - ts)
				durLbl = formatDurationSecondsLabel(durSec)
				hasAudio = true
			}
		}

		shownTarget := targetFile
		if audioFile != "" {
			shownTarget = audioFile
		} else if expVoice != "" {
			shownTarget = expVoice
		}

		sections = append(sections, VoiceSection{
			Index:         idx,
			Key:           key,
			Kind:          rs.kind,
			Condition:     rs.cond,
			Label:         label,
			ShortLabel:    shortLabel,
			HeaderTitle:   headerTitle,
			Text:          strings.TrimSpace(strings.Join(rs.lines, "\n")),
			ExplicitVoice: expVoice,
			TargetFile:    shownTarget,
			AudioFile:     audioFile,
			AudioURL:      audioURL,
			TrimStart:     ts,
			TrimEnd:       te,
			DurationSec:   durSec,
			DurationLabel: durLbl,
			HasAudio:      hasAudio,
			HasTrim:       hasAudio && HasVoiceTrimValues(ts, te),
		})
	}

	sc.VoiceSections = sections
	return sections
}

func appendSceneVoiceSectionWarnings(b *Book, sc *Scene) {
	if b == nil || sc == nil || !b.Narrated {
		return
	}
	scPrefix1 := fmt.Sprintf("Scena „%s”: brak nagrania audio dla sekcji", sc.DisplayName())
	scPrefix2 := fmt.Sprintf("Scena „%s” (sekcja ", sc.DisplayName())
	filterWarns := func(warns []string) []string {
		if len(warns) == 0 {
			return warns
		}
		out := warns[:0]
		for _, w := range warns {
			if strings.HasPrefix(w, scPrefix1) || strings.HasPrefix(w, scPrefix2) {
				continue
			}
			out = append(out, w)
		}
		return out
	}
	sc.Warnings = filterWarns(sc.Warnings)
	b.Warnings = filterWarns(b.Warnings)

	for _, sec := range sc.VoiceSections {
		if sec.HasAudio {
			continue
		}
		var warn string
		if sec.ExplicitVoice != "" {
			warn = fmt.Sprintf("Scena „%s” (sekcja %s): nie znaleziono pliku voice: „%s”", sc.DisplayName(), sec.Label, sec.ExplicitVoice)
		} else {
			warn = fmt.Sprintf("Scena „%s”: brak nagrania audio dla sekcji „%s” (oczekiwano pliku „%s”)", sc.DisplayName(), sec.Label, sec.TargetFile)
		}
		sc.Warnings = append(sc.Warnings, warn)
		b.Warnings = append(b.Warnings, warn)
	}
}

// ReplaceSceneVoiceSectionText replaces the prose text of a specific section key inside fullText while keeping all other sections and directives intact.
func ReplaceSceneVoiceSectionText(fullText, sectionKey, newSectionText string) string {
	cleanKey := strings.ToLower(strings.TrimSpace(sectionKey))
	if cleanKey == "" || !HasConditionalVoiceDirectives(fullText) {
		return SanitizePlainText(newSectionText)
	}
	rawSecs, _ := splitRawVoiceSections(fullText)
	total := len(rawSecs)
	targetIdx := -1
	for i, rs := range rawSecs {
		key, _, _, _ := formatVoiceSectionKeyAndLabels(i+1, total, rs.kind, rs.cond)
		if strings.EqualFold(key, cleanKey) {
			targetIdx = i
			existingText := strings.TrimSpace(strings.Join(rs.lines, "\n"))
			if strings.TrimSpace(SanitizePlainText(newSectionText)) == existingText {
				return fullText
			}
			break
		}
	}
	if targetIdx < 0 {
		return fullText
	}

	normalized := strings.ReplaceAll(fullText, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var outLines []string
	curKind := "static"
	curCond := ""
	var curLines []string
	secCounter := 0

	emitReplacedOrOriginal := func(force bool) {
		joined := strings.TrimSpace(strings.Join(curLines, "\n"))
		if curKind == "static" && joined == "" && !force {
			outLines = append(outLines, curLines...)
			curLines = nil
			return
		}
		if secCounter == targetIdx {
			cleanReplacement := strings.TrimSpace(SanitizePlainText(newSectionText))
			if cleanReplacement != "" {
				outLines = append(outLines, strings.Split(cleanReplacement, "\n")...)
			}
			if curKind == "static" && secCounter == 0 && len(curLines) > 0 && strings.TrimSpace(curLines[len(curLines)-1]) == "" {
				outLines = append(outLines, "")
			}
		} else {
			outLines = append(outLines, curLines...)
		}
		secCounter++
		curLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if kind, condExpr, ok := parseProseBlockDirective(trimmed, nil, nil); ok {
			switch kind {
			case "if":
				emitReplacedOrOriginal(false)
				curKind = "if"
				curCond = condExpr
				outLines = append(outLines, line)
				continue
			case "elif":
				emitReplacedOrOriginal(true)
				curKind = "elif"
				curCond = condExpr
				outLines = append(outLines, line)
				continue
			case "else":
				emitReplacedOrOriginal(true)
				curKind = "else"
				curCond = ""
				outLines = append(outLines, line)
				continue
			case "endif":
				emitReplacedOrOriginal(true)
				curKind = "static"
				curCond = ""
				outLines = append(outLines, line)
				continue
			case "set":
				outLines = append(outLines, line)
				continue
			}
		}
		_ = curCond
		curLines = append(curLines, line)
	}
	emitReplacedOrOriginal(false)
	return strings.Join(outLines, "\n")
}

// SerializeSceneProseWithSectionVoices returns sc.Text with optional "voice: <path> [start..end]" lines
// injected at the start of any conditional/static section that has a custom file or non-zero trim.
func SerializeSceneProseWithSectionVoices(sc *Scene, b *Book) string {
	if sc == nil {
		return ""
	}
	if b != nil && !b.Narrated {
		return sc.Text
	}
	if !HasConditionalVoiceDirectives(sc.Text) || len(sc.SectionVoices) == 0 {
		return sc.Text
	}
	baseStem := SceneVoiceBaseStem(sc)
	rawSecs, _ := splitRawVoiceSections(sc.Text)
	total := len(rawSecs)

	secVoiceHeaderByIdx := make(map[int]string)
	for i, rs := range rawSecs {
		idx := i + 1
		key, _, _, _ := formatVoiceSectionKeyAndLabels(idx, total, rs.kind, rs.cond)
		meta, ok := sc.SectionVoices[strings.ToLower(key)]
		if !ok {
			continue
		}
		defaultTarget := fmt.Sprintf("voice/%s__%s.flac", baseStem, key)
		hasTrim := HasVoiceTrimValues(meta.TrimStart, meta.TrimEnd)
		hasCustomFile := meta.ExplicitVoice != "" && !strings.EqualFold(meta.ExplicitVoice, defaultTarget)
		if !hasTrim && !hasCustomFile {
			continue
		}
		voicePath := meta.ExplicitVoice
		if voicePath == "" {
			voicePath = defaultTarget
		}
		if hasTrim {
			ts, te := NormalizedVoiceTrimValues(meta.TrimStart, meta.TrimEnd)
			secVoiceHeaderByIdx[i] = fmt.Sprintf("voice: %s [%.4f..%.4f]", voicePath, ts, te)
		} else {
			secVoiceHeaderByIdx[i] = fmt.Sprintf("voice: %s", voicePath)
		}
	}
	if len(secVoiceHeaderByIdx) == 0 {
		return sc.Text
	}

	normalized := strings.ReplaceAll(sc.Text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	var outLines []string
	curKind := "static"
	var curLines []string
	secCounter := 0

	flushChunk := func(force bool) {
		joined := strings.TrimSpace(strings.Join(curLines, "\n"))
		if curKind == "static" && joined == "" && !force {
			outLines = append(outLines, curLines...)
			curLines = nil
			return
		}
		if hdr, ok := secVoiceHeaderByIdx[secCounter]; ok {
			// Insert hdr before the first non-empty prose line of this chunk
			inserted := false
			for _, ln := range curLines {
				if !inserted && strings.TrimSpace(ln) != "" {
					outLines = append(outLines, hdr)
					inserted = true
				}
				outLines = append(outLines, ln)
			}
			if !inserted {
				outLines = append(outLines, hdr)
			}
		} else {
			outLines = append(outLines, curLines...)
		}
		secCounter++
		curLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if kind, _, ok := parseProseBlockDirective(trimmed, nil, nil); ok {
			switch kind {
			case "if":
				flushChunk(false)
				curKind = "if"
				outLines = append(outLines, line)
				continue
			case "elif":
				flushChunk(true)
				curKind = "elif"
				outLines = append(outLines, line)
				continue
			case "else":
				flushChunk(true)
				curKind = "else"
				outLines = append(outLines, line)
				continue
			case "endif":
				flushChunk(true)
				curKind = "static"
				outLines = append(outLines, line)
				continue
			}
		}
		if _, _, _, isVoice := parseInlineVoiceSectionLine(trimmed); isVoice {
			continue
		}
		curLines = append(curLines, line)
	}
	flushChunk(false)
	return strings.Join(outLines, "\n")
}

// EvaluateSceneAudioQueue returns the ordered list of active audio segments for sc given the player's current variable state.
func EvaluateSceneAudioQueue(sc *Scene, state map[string]string, b *Book) []VoiceQueueItem {
	if sc == nil || (b != nil && !b.Narrated) {
		return nil
	}
	sections := ExtractSceneVoiceSections(sc, b)
	if len(sections) == 0 {
		return nil
	}
	if len(sections) == 1 && !HasConditionalVoiceDirectives(sc.Text) {
		sec := sections[0]
		if !sec.HasAudio || sec.AudioURL == "" {
			return nil
		}
		return []VoiceQueueItem{
			{
				Key:         sec.Key,
				Label:       sec.Label,
				AudioFile:   sec.AudioFile,
				AudioURL:    sec.AudioURL,
				TrimStart:   sec.TrimStart,
				TrimEnd:     sec.TrimEnd,
				DurationSec: sec.DurationSec,
			},
		}
	}

	var queue []VoiceQueueItem
	branchTaken := false
	inCondBlock := false
	branchActive := true

	for _, sec := range sections {
		switch sec.Kind {
		case "if":
			inCondBlock = true
			met := EvaluateCondition(sec.Condition, state, b)
			branchActive = met
			branchTaken = met
		case "elif":
			inCondBlock = true
			if branchTaken {
				branchActive = false
			} else {
				met := EvaluateCondition(sec.Condition, state, b)
				branchActive = met
				if met {
					branchTaken = true
				}
			}
		case "else":
			inCondBlock = true
			branchActive = !branchTaken
			branchTaken = true
		default: // "static"
			inCondBlock = false
			branchTaken = false
			branchActive = true
		}
		_ = inCondBlock
		if !branchActive {
			continue
		}
		if !sec.HasAudio || sec.AudioURL == "" {
			continue
		}
		queue = append(queue, VoiceQueueItem{
			Key:         sec.Key,
			Label:       sec.Label,
			AudioFile:   sec.AudioFile,
			AudioURL:    sec.AudioURL,
			TrimStart:   sec.TrimStart,
			TrimEnd:     sec.TrimEnd,
			DurationSec: sec.DurationSec,
		})
	}
	return queue
}

// EncodeSceneAudioQueue encodes a slice of VoiceQueueItems into "url1|ts1|te1|dur1;url2|ts2|te2|dur2" and returns total duration.
func EncodeSceneAudioQueue(items []VoiceQueueItem) (queueSpec string, totalDurSec float64, totalDurLabel string) {
	if len(items) == 0 {
		return "", 0, "0:00"
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if it.AudioURL == "" {
			continue
		}
		ts, te := NormalizedVoiceTrimValues(it.TrimStart, it.TrimEnd)
		dur := it.DurationSec
		if dur < 0 {
			dur = 0
		}
		totalDurSec += dur
		parts = append(parts, fmt.Sprintf("%s|%.4f|%.4f|%.3f", it.AudioURL, ts, te, dur))
	}
	return strings.Join(parts, ";"), totalDurSec, formatDurationSecondsLabel(totalDurSec)
}
