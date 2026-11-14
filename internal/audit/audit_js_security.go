package audit

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type jsFileData struct {
	rel   string
	src   string
	lines []string
}

type jsFuncBlock struct {
	file      string
	name      string
	startLine int
	endLine   int
	params    []string
	bodyLines []string
	bodyText  string
}

func loadAllJSFiles(rootDir string) []jsFileData {
	jsDir := filepath.Join(rootDir, "app", "js")
	if _, err := os.Stat(jsDir); err != nil {
		return nil
	}
	var out []jsFileData
	readJSDir := func(dir, relPrefix string) {
		dirEntries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range dirEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			src := string(raw)
			out = append(out, jsFileData{
				rel:   relPrefix + e.Name(),
				src:   src,
				lines: strings.Split(src, "\n"),
			})
		}
	}
	readJSDir(jsDir, "app/js/")
	readJSDir(filepath.Join(jsDir, "controllers"), "app/js/controllers/")
	return out
}

func extractJSFunctions(files []jsFileData) []jsFuncBlock {
	funcHeaderRe := regexp.MustCompile(`^\s*(?:async\s+)?function\s+([A-Za-z0-9_]+)\s*\(([^)]*)\)\s*\{`)
	var funcs []jsFuncBlock

	for _, jf := range files {
		for i := 0; i < len(jf.lines); i++ {
			m := funcHeaderRe.FindStringSubmatch(jf.lines[i])
			if m == nil {
				continue
			}
			fnName := m[1]
			rawParams := strings.TrimSpace(m[2])
			var params []string
			if rawParams != "" {
				for _, p := range strings.Split(rawParams, ",") {
					p = strings.TrimSpace(p)
					if eqIdx := strings.IndexByte(p, '='); eqIdx > 0 {
						p = strings.TrimSpace(p[:eqIdx])
					}
					if p != "" && !strings.HasPrefix(p, "_") && !strings.ContainsAny(p, "{}[]().") {
						params = append(params, p)
					}
				}
			}

			startLine := i + 1
			braceDepth := 0
			endIdx := i
			for j := i; j < len(jf.lines); j++ {
				ln := stripJSLineCommentAndStrings(jf.lines[j])
				for _, ch := range ln {
					if ch == '{' {
						braceDepth++
					} else if ch == '}' {
						braceDepth--
					}
				}
				if braceDepth <= 0 && j >= i {
					endIdx = j
					break
				}
			}
			var bodyLines []string
			if endIdx > i+1 {
				bodyLines = jf.lines[i+1 : endIdx]
			}
			funcs = append(funcs, jsFuncBlock{
				file:      jf.rel,
				name:      fnName,
				startLine: startLine,
				endLine:   endIdx + 1,
				params:    params,
				bodyLines: bodyLines,
				bodyText:  strings.Join(bodyLines, "\n"),
			})
		}
	}
	return funcs
}

func stripJSLineCommentAndStrings(line string) string {
	var sb strings.Builder
	inStr := byte(0)
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inStr != 0 {
			escaped = true
			continue
		}
		if inStr != 0 {
			if c == inStr {
				inStr = 0
			}
			continue
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '/' {
			break
		}
		if c == '"' || c == '\'' || c == '`' {
			inStr = c
			continue
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

// 7. JS Deep Static Analysis: JS_DEAD_PROP, JS_UNUSED_PARAM, JS_DUPLICATE, JS_DOM_CONTRACT, JS_MAGIC_NUMBER
func checkJSDeepAnalysis(rootDir string) []Finding {
	jsFiles := loadAllJSFiles(rootDir)
	if len(jsFiles) == 0 {
		return nil
	}
	funcs := extractJSFunctions(jsFiles)
	var out []Finding

	var allJSSrc strings.Builder
	for _, jf := range jsFiles {
		allJSSrc.WriteString(jf.src)
		allJSSrc.WriteByte('\n')
	}
	fullJS := allJSSrc.String()

	// 7a. JS_UNUSED_PARAM
	for _, fn := range funcs {
		if len(fn.params) == 0 || strings.Contains(fn.bodyText, "arguments") {
			continue
		}
		for _, p := range fn.params {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(p) + `\b`)
			if !re.MatchString(fn.bodyText) {
				out = append(out, Finding{
					File:     fn.file,
					Line:     fn.startLine,
					FuncName: fn.name,
					Category: "JS_UNUSED_PARAM",
					Message:  fmt.Sprintf("parametr %q w funkcji %s() nigdy nie jest używany w ciele funkcji", p, fn.name),
				})
			}
		}
	}

	// 7b. JS_DEAD_PROP: properties assigned (obj.prop = ...) that are never read anywhere
	propWriteRe := regexp.MustCompile(`\.([A-Za-z_][A-Za-z0-9_]*)\s*=\s*[^=]`)
	domIgnoreProps := map[string]bool{
		"value": true, "disabled": true, "checked": true, "readOnly": true,
		"textContent": true, "innerHTML": true, "outerHTML": true, "className": true,
		"width": true, "height": true, "src": true, "href": true, "id": true, "title": true,
		"scrollTop": true, "scrollLeft": true, "currentTime": true, "playbackRate": true,
		"volume": true, "muted": true, "loop": true, "autoplay": true, "preload": true,
		"left": true, "top": true, "right": true, "bottom": true, "display": true,
		"opacity": true, "transform": true, "cursor": true, "pointerEvents": true,
		"heightPx": true, "minHeight": true, "maxHeight": true, "zIndex": true,
		"flexGrow": true, "fontSize": true, "letterSpacing": true,
		"fillStyle": true, "strokeStyle": true, "lineWidth": true, "buffer": true,
		"audio_time": true, "pcm_b64": true, "sample_rate": true,
		"onload": true, "onerror": true, "onended": true, "ontimeupdate": true,
		"onloadedmetadata": true, "onaudioprocess": true, "onstop": true, "ondataavailable": true,
		"__STUDIO_MSDF_DATA": true,
	}
	type propWriteLoc struct {
		file     string
		line     int
		funcName string
		prop     string
	}
	writtenProps := make(map[string]propWriteLoc)
	for _, jf := range jsFiles {
		curFunc := "<global>"
		funcDeclRe := regexp.MustCompile(`^\s*(?:async\s+)?function\s+([A-Za-z0-9_]+)\s*\(`)
		for idx, ln := range jf.lines {
			if fm := funcDeclRe.FindStringSubmatch(ln); fm != nil {
				curFunc = fm[1]
			}
			cleanLn := stripJSLineCommentAndStrings(ln)
			for _, m := range propWriteRe.FindAllStringSubmatch(cleanLn, -1) {
				prop := m[1]
				if domIgnoreProps[prop] || strings.HasPrefix(prop, "on") {
					continue
				}
				if _, exists := writtenProps[prop]; !exists {
					writtenProps[prop] = propWriteLoc{
						file:     jf.rel,
						line:     idx + 1,
						funcName: curFunc,
						prop:     prop,
					}
				}
			}
		}
	}

	for prop, loc := range writtenProps {
		// Count total occurrences of .prop or ["prop"] vs assignment occurrences .prop =
		accessRe := regexp.MustCompile(`(?:\.` + regexp.QuoteMeta(prop) + `\b|\[\s*"` + regexp.QuoteMeta(prop) + `"\s*\])`)
		assignOnlyRe := regexp.MustCompile(`\.` + regexp.QuoteMeta(prop) + `\s*=\s*[^=]`)
		totalAccess := len(accessRe.FindAllStringIndex(fullJS, -1))
		totalAssign := len(assignOnlyRe.FindAllStringIndex(fullJS, -1))
		if totalAccess > 0 && totalAccess <= totalAssign {
			out = append(out, Finding{
				File:     loc.file,
				Line:     loc.line,
				FuncName: loc.funcName,
				Category: "JS_DEAD_PROP",
				Message:  fmt.Sprintf("pole obiektu \".%s\" jest zapisywane (%d×), ale nigdy nie jest odczytywane", prop, totalAssign),
			})
		}
	}

	// 7c. JS_DOM_CONTRACT: qs("#id") / getElementById("id") vs app/templates/*.tpl & www/*.tpl
	var allTpl strings.Builder
	for _, tplDir := range []string{filepath.Join(rootDir, "app", "templates"), filepath.Join(rootDir, "www")} {
		if entries, err := os.ReadDir(tplDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".tpl") {
					if raw, err := os.ReadFile(filepath.Join(tplDir, e.Name())); err == nil {
						allTpl.Write(raw)
						allTpl.WriteByte('\n')
					}
				}
			}
		}
	}
	tplSrc := allTpl.String()

	qsIdRe := regexp.MustCompile(`(?:qs|getElementById)\(\s*"#?([A-Za-z0-9_-]+)"\s*\)`)
	seenMissingID := make(map[string]bool)
	for _, jf := range jsFiles {
		curFunc := "<global>"
		funcDeclRe := regexp.MustCompile(`^\s*(?:async\s+)?function\s+([A-Za-z0-9_]+)\s*\(`)
		for idx, ln := range jf.lines {
			if fm := funcDeclRe.FindStringSubmatch(ln); fm != nil {
				curFunc = fm[1]
			}
			for _, m := range qsIdRe.FindAllStringSubmatch(ln, -1) {
				idVal := m[1]
				if idVal == "app" || seenMissingID[idVal] {
					continue
				}
				if !strings.Contains(tplSrc, "#"+idVal) && !strings.Contains(tplSrc, `id: "`+idVal+`"`) && !strings.Contains(tplSrc, `id: `+idVal) {
					seenMissingID[idVal] = true
					out = append(out, Finding{
						File:     jf.rel,
						Line:     idx + 1,
						FuncName: curFunc,
						Category: "JS_DOM_CONTRACT",
						Message:  fmt.Sprintf("selektor #%s w JS nie występuje w żadnym szablonie app/templates/*.tpl", idVal),
					})
				}
			}
		}
	}

	// 7d. JS_DUPLICATE (only large functions >= 25 lines) & SPLIT_BRAIN (domain logic duplicated across JS modules or Go <-> JS)
	type jsFingerprint struct {
		fn     jsFuncBlock
		tokens []string
	}
	var fps []jsFingerprint
	jsTokenRe := regexp.MustCompile(`\b(if|else|for|while|switch|case|return|Math|pushQuadInstance|pushSolidQuad|pushBorderedBoxQuads|sendServerAction|classList|getAttribute|setAttribute)\b|(\+=|-=|\*=|===|!==|<=|>=|&&|\|\||<<|>>)`)
	for _, fn := range funcs {
		if len(fn.bodyLines) < 25 {
			continue
		}
		var toks []string
		for _, ln := range fn.bodyLines {
			clean := stripJSLineCommentAndStrings(ln)
			for _, m := range jsTokenRe.FindAllString(clean, -1) {
				toks = append(toks, m)
			}
		}
		if len(toks) >= 35 {
			fps = append(fps, jsFingerprint{fn: fn, tokens: toks})
		}
	}
	for i := 0; i < len(fps); i++ {
		for j := i + 1; j < len(fps); j++ {
			a, b := fps[i], fps[j]
			sim := tokenSequenceSimilarity(a.tokens, b.tokens)
			if sim >= 0.85 {
				out = append(out, Finding{
					File:     b.fn.file,
					Line:     b.fn.startLine,
					FuncName: b.fn.name,
					Category: "JS_DUPLICATE",
					Message:  fmt.Sprintf("duża bliźniacza funkcja JS (%.0f%% podobieństwa, %d linii) do %s:%d (%s)", sim*100, len(b.fn.bodyLines), a.fn.file, a.fn.startLine, a.fn.name),
				})
			}
		}
	}

	// 7e. Repeated multi-line blocks in JS (>= 25 non-trivial normalized lines repeated >= 2 times)
	out = append(out, detectRepeatedCodeBlocks(jsFiles, false)...)

	return out
}

type codeLineLoc struct {
	file     string
	line     int
	funcName string
	norm     string
	raw      string
}

var (
	strLitNormRe = regexp.MustCompile(`"[^"\\]*(?:\\.[^"\\]*)*"|'[^'\\]*(?:\\.[^'\\]*)*'`)
	wsNormRe     = regexp.MustCompile(`\s+`)
)

func normalizeCodeLineForBlockMatch(line string) string {
	clean := stripJSLineCommentAndStrings(line)
	trimmed := strings.TrimSpace(clean)
	if len(trimmed) < 10 {
		return ""
	}
	switch trimmed {
	case "{", "}", "});", "};", "break;", "continue;", "return;", "return nil", "return false", "return true", "return \"\"", "} else {":
		return ""
	}
	// Replace string literals from original trimmed line with "$S" so structural clones with different field names match
	norm := strings.TrimSpace(line)
	if idx := strings.Index(norm, "//"); idx >= 0 {
		norm = strings.TrimSpace(norm[:idx])
	}
	norm = strLitNormRe.ReplaceAllString(norm, `"$S"`)
	norm = wsNormRe.ReplaceAllString(norm, " ")
	if len(norm) < 14 {
		return ""
	}
	return norm
}

func detectRepeatedCodeBlocks(jsFiles []jsFileData, isGo bool) []Finding {
	var stream []codeLineLoc
	funcDeclRe := regexp.MustCompile(`^\s*(?:async\s+)?function\s+([A-Za-z0-9_]+)\s*\(`)

	for _, jf := range jsFiles {
		curFunc := "<pkg>"
		for idx, ln := range jf.lines {
			trimmed := strings.TrimSpace(ln)
			if isGo {
				if strings.HasPrefix(trimmed, "func ") {
					curFunc = extractGoFuncHeaderName(trimmed)
				}
			} else {
				if fm := funcDeclRe.FindStringSubmatch(ln); fm != nil {
					curFunc = fm[1]
				}
			}
			norm := normalizeCodeLineForBlockMatch(ln)
			if norm == "" {
				continue
			}
			stream = append(stream, codeLineLoc{
				file:     jf.rel,
				line:     idx + 1,
				funcName: curFunc,
				norm:     norm,
				raw:      trimmed,
			})
		}
		// File boundary sentinel
		stream = append(stream, codeLineLoc{norm: "__FILE_BREAK__"})
	}

	const minWin = 25
	const minDuplicationScore = 400 // Require >= 25 non-trivial normalized lines repeated >= 2 times

	type candidateBlock struct {
		length  int
		indices []int
		score   int
	}

	// Build inverted index of 4-line seeds
	seedGroups := make(map[string][]int)
	for i := 0; i+minWin <= len(stream); i++ {
		valid := true
		var kb strings.Builder
		for k := 0; k < minWin; k++ {
			if stream[i+k].norm == "__FILE_BREAK__" {
				valid = false
				break
			}
			if k > 0 {
				kb.WriteByte('\n')
			}
			kb.WriteString(stream[i+k].norm)
		}
		if !valid {
			continue
		}
		key := kb.String()
		list := seedGroups[key]
		if len(list) > 0 && i-list[len(list)-1] < minWin {
			continue
		}
		seedGroups[key] = append(list, i)
	}

	var candidates []candidateBlock
	for _, seedIdxs := range seedGroups {
		if len(seedIdxs) < 2 {
			continue
		}
		// Find maximal extension length L for the full group (or subgroups of size >= 2)
		maxL := minWin
		for {
			nextNorm := ""
			allMatch := true
			for m, startIdx := range seedIdxs {
				pos := startIdx + maxL
				if pos >= len(stream) || stream[pos].norm == "__FILE_BREAK__" {
					allMatch = false
					break
				}
				if m == 0 {
					nextNorm = stream[pos].norm
				} else if stream[pos].norm != nextNorm {
					allMatch = false
					break
				}
			}
			if !allMatch {
				break
			}
			maxL++
		}
		n := len(seedIdxs)
		score := (maxL - 3) * (maxL - 3) * (n - 1)
		if score >= minDuplicationScore {
			candidates = append(candidates, candidateBlock{
				length:  maxL,
				indices: seedIdxs,
				score:   score,
			})
		}
	}

	// Sort by highest duplication score first so maximal blocks suppress sub-windows
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	var out []Finding
	coveredStreamIdx := make([]bool, len(stream))
	cat := "JS_DUPLICATE"
	if isGo {
		cat = "AST_DUPLICATE"
	}

	for _, cand := range candidates {
		firstIdx := cand.indices[0]
		alreadyCovered := false
		for k := 0; k < cand.length; k++ {
			if coveredStreamIdx[firstIdx+k] {
				alreadyCovered = true
				break
			}
		}
		if alreadyCovered {
			continue
		}
		for _, startIdx := range cand.indices {
			for k := 0; k < cand.length && startIdx+k < len(stream); k++ {
				coveredStreamIdx[startIdx+k] = true
			}
		}

		first := stream[firstIdx]
		count := len(cand.indices)
		var otherLocs []string
		for m := 1; m < count && len(otherLocs) < 4; m++ {
			loc := stream[cand.indices[m]]
			otherLocs = append(otherLocs, fmt.Sprintf("%s:%d", filepath.Base(loc.file), loc.line))
		}
		more := ""
		if count-1 > 4 {
			more = fmt.Sprintf(" i +%d miejsc", count-1-4)
		}
		out = append(out, Finding{
			File:     first.file,
			Line:     first.line,
			FuncName: first.funcName,
			Category: cat,
			Message:  fmt.Sprintf("blok %d linii powtórzony %d× [waga=%d] (także w: %s%s) — pocz.: %s", cand.length, count, cand.score, strings.Join(otherLocs, ", "), more, truncateStr(first.raw, 50)),
		})
	}

	return out
}

// 8. SECURITY_XSS_INJECTION, UNBOUNDED_LOOP_RECURSION, RACE_AND_LOCK_HAZARD, HOT_PATH_ALLOC, SWALLOWED_ERROR
func checkSecurityLoopsAndReliability(rootDir string, fset *token.FileSet, goFiles []*parsedGoFile) []Finding {
	var out []Finding
	jsFiles := loadAllJSFiles(rootDir)

	// 8a. SECURITY_XSS_INJECTION in JS (innerHTML / outerHTML / insertAdjacentHTML outside HTML-Over-The-Wire transport)
	innerHTMLRe := regexp.MustCompile(`\.(innerHTML|outerHTML)\s*=|insertAdjacentHTML\s*\(`)
	emptyInnerHTMLRe := regexp.MustCompile(`\.(?:innerHTML|outerHTML)\s*=\s*(?:""|'')\s*;?$`)
	hotwTransportLineRe := regexp.MustCompile(`^(?:modalsRoot\.innerHTML\s*=\s*html;|existingModal\.outerHTML\s*=\s*html;|modalsRoot\.insertAdjacentHTML\("beforeend",\s*html\);|existingPanel\.outerHTML\s*=\s*html;|editorBody\.insertAdjacentHTML\("afterbegin",\s*html\);|targetEl\.innerHTML\s*=\s*html;|mountEl\.innerHTML\s*=\s*bootParsed\.(?:html|body);|if\s*\(el\)\s*el\.innerHTML\s*=\s*await\s*resp\.text\(\);)$`)
	for _, jf := range jsFiles {
		curFunc := "<global>"
		funcDeclRe := regexp.MustCompile(`^\s*(?:async\s+)?(?:function\s+)?([A-Za-z0-9_]+)\s*\(`)
		for idx, ln := range jf.lines {
			if fm := funcDeclRe.FindStringSubmatch(ln); fm != nil && fm[1] != "if" && fm[1] != "for" && fm[1] != "while" && fm[1] != "switch" && fm[1] != "catch" {
				curFunc = fm[1]
			}
			trimmed := strings.TrimSpace(ln)
			clean := strings.TrimSpace(stripJSLineCommentAndStrings(ln))
			if innerHTMLRe.MatchString(clean) && !emptyInnerHTMLRe.MatchString(trimmed) {
				if ((jf.rel == "app/js/core.js" && (curFunc == "sendServerAction" || curFunc == "swapTarget" || curFunc == "bootWasmPlayer")) ||
					(jf.rel == "app/js/dispatcher.js" && (curFunc == "post" || curFunc == "swapTarget"))) && hotwTransportLineRe.MatchString(trimmed) {
					continue
				}
				out = append(out, Finding{
					File:     jf.rel,
					Line:     idx + 1,
					FuncName: curFunc,
					Category: "SECURITY_XSS_INJECTION",
					Message:  fmt.Sprintf("bezpośredni zapis surowego HTML do DOM (%s)", trimmed),
				})
			}
		}
	}

	// 8b. SECURITY_XSS_INJECTION in .tpl ({raw ...} is strictly forbidden; templates must use {prose ...})
	tplDir := filepath.Join(rootDir, "app", "templates")
	rawDirectiveRe := regexp.MustCompile(`\{raw\s+([A-Za-z0-9_.]+)\}`)
	if entries, err := os.ReadDir(tplDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".tpl") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(tplDir, e.Name()))
			if err != nil {
				continue
			}
			rel := "app/templates/" + e.Name()
			for idx, ln := range strings.Split(string(raw), "\n") {
				for _, m := range rawDirectiveRe.FindAllStringSubmatch(ln, -1) {
					out = append(out, Finding{
						File:     rel,
						Line:     idx + 1,
						FuncName: "<template>",
						Category: "SECURITY_XSS_INJECTION",
						Message:  fmt.Sprintf("zakazana dyrektywa {raw %s} — użyj {prose ...}", m[1]),
					})
				}
			}
		}
	}

	// 8c. HOT_PATH_ALLOC & UNBOUNDED_LOOP_RECURSION in JS (infinite loops & direct recursion, no nesting depth limit)
	domQueryInRenderRe := regexp.MustCompile(`\b(qs|qsa|querySelector|querySelectorAll|getBoundingClientRect|getComputedStyle)\s*\(`)
	jsInfiniteLoopRe := regexp.MustCompile(`\b(?:while\s*\(\s*true\s*\)|for\s*\(\s*;\s*;\s*\))`)
	for _, fn := range extractJSFunctions(jsFiles) {
		isRenderHotPath := strings.HasPrefix(fn.name, "renderStudio") || strings.HasPrefix(fn.name, "push") || strings.HasPrefix(fn.name, "draw")
		selfCallRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(fn.name) + `\s*\(`)
		reportedRecursion := false
		for offset, ln := range fn.bodyLines {
			lineNo := fn.startLine + 1 + offset
			clean := stripJSLineCommentAndStrings(ln)
			if isRenderHotPath && domQueryInRenderRe.MatchString(clean) {
				out = append(out, Finding{
					File:     fn.file,
					Line:     lineNo,
					FuncName: fn.name,
					Category: "HOT_PATH_ALLOC",
					Message:  fmt.Sprintf("odpytywanie DOM / layoutu wewnątrz gorącej ścieżki renderowania klatki: %s", strings.TrimSpace(ln)),
				})
			}
			if jsInfiniteLoopRe.MatchString(clean) {
				out = append(out, Finding{
					File:     fn.file,
					Line:     lineNo,
					FuncName: fn.name,
					Category: "UNBOUNDED_LOOP_RECURSION",
					Message:  "pętla nieskończona while(true)/for(;;) w JS — ryzyko zawieszenia wątku UI",
				})
			}
			if !reportedRecursion && selfCallRe.MatchString(clean) {
				reportedRecursion = true
				out = append(out, Finding{
					File:     fn.file,
					Line:     lineNo,
					FuncName: fn.name,
					Category: "UNBOUNDED_LOOP_RECURSION",
					Message:  fmt.Sprintf("rekurencja bezpośrednia w funkcji JS %s() (NASA Power of 10 Rule 1)", fn.name),
				})
			}
		}
	}

	// 8d. Go AST passes: UNBOUNDED_LOOP_RECURSION, RACE_AND_LOCK_HAZARD, HOT_PATH_ALLOC, SWALLOWED_ERROR
	for _, pf := range goFiles {
		if pf.isTest {
			continue
		}
		lines := strings.Split(string(pf.src), "\n")
		isNativeWindowLoopFile := strings.HasPrefix(pf.relPath, "internal/gui/window_") ||
			strings.HasPrefix(pf.relPath, "internal/osbridge/window_")

		for _, decl := range pf.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			fnName := enclosingFuncName(pf.file, fn.Pos())
			bareFnName := ""
			if fn.Name != nil {
				bareFnName = fn.Name.Name
			}
			hasRecv := fn.Recv != nil && len(fn.Recv.List) > 0
			reportedFnRecursion := false

			// Inspect statements inside function body
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.ForStmt:
					if !isNativeWindowLoopFile && x.Cond == nil && x.Init == nil && x.Post == nil {
						pos := fset.Position(x.Pos())
						out = append(out, Finding{
							File:     pf.relPath,
							Line:     pos.Line,
							FuncName: fnName,
							Category: "UNBOUNDED_LOOP_RECURSION",
							Message:  "pętla bezwarunkowa `for { ... }` bez licznika maksymalnej liczby iteracji",
						})
					}

				case *ast.CallExpr:
					pos := fset.Position(x.Pos())
					callName := exprCallName(x.Fun)

					// UNBOUNDED_LOOP_RECURSION: direct function/method recursion (NASA Power of 10 Rule 1)
					if !reportedFnRecursion && bareFnName != "" {
						isSelfCall := false
						if !hasRecv && callName == bareFnName {
							isSelfCall = true
						} else if hasRecv && strings.HasSuffix(callName, "."+bareFnName) {
							isSelfCall = true
						}
						if isSelfCall {
							reportedFnRecursion = true
							out = append(out, Finding{
								File:     pf.relPath,
								Line:     pos.Line,
								FuncName: fnName,
								Category: "UNBOUNDED_LOOP_RECURSION",
								Message:  fmt.Sprintf("rekurencja bezpośrednia w %s (NASA Power of 10 Rule 1)", fnName),
							})
						}
					}

					// HOT_PATH_ALLOC: regexp.MustCompile or regexp.Compile inside a function body
					if callName == "regexp.MustCompile" || callName == "regexp.Compile" {
						out = append(out, Finding{
							File:     pf.relPath,
							Line:     pos.Line,
							FuncName: fnName,
							Category: "HOT_PATH_ALLOC",
							Message:  fmt.Sprintf("%s wywoływane wewnątrz funkcji zamiast raz na poziomie pakietu (var)", callName),
						})
					}

					// HOT_PATH_ALLOC: Disk I/O or heavy analysis inside render_*.go / view_*.go
					if strings.HasPrefix(pf.relPath, "internal/gui/render") || strings.HasPrefix(pf.relPath, "internal/views/view_") {
						if callName == "os.Stat" || callName == "os.ReadFile" || callName == "os.ReadDir" || callName == "book.AnalyzeScreenplay" || callName == "book.CompareBookBranches" {
							out = append(out, Finding{
								File:     pf.relPath,
								Line:     pos.Line,
								FuncName: fnName,
								Category: "HOT_PATH_ALLOC",
								Message:  fmt.Sprintf("kosztowne wywołanie %s na ścieżce renderowania widoku HTML", callName),
							})
						}
					}

					// RACE_AND_LOCK_HAZARD: manual s.mu.Unlock() mid-function
					if strings.HasSuffix(callName, ".mu.Unlock") {
						lineStr := ""
						if pos.Line-1 >= 0 && pos.Line-1 < len(lines) {
							lineStr = strings.TrimSpace(lines[pos.Line-1])
						}
						if !strings.HasPrefix(lineStr, "defer ") {
							out = append(out, Finding{
								File:     pf.relPath,
								Line:     pos.Line,
								FuncName: fnName,
								Category: "RACE_AND_LOCK_HAZARD",
								Message:  fmt.Sprintf("ręczne %s() bez defer w środku handlera — ryzyko wyścigu lub paniki przed ponownym Lock()", callName),
							})
						}
					}

				case *ast.AssignStmt:
					// SWALLOWED_ERROR: `_ = ...` on critical persistence / IO functions
					if len(x.Lhs) == 1 && len(x.Rhs) == 1 {
						if id, ok := x.Lhs[0].(*ast.Ident); ok && id.Name == "_" {
							if ce, ok := x.Rhs[0].(*ast.CallExpr); ok {
								callName := exprCallName(ce.Fun)
								if isCriticalPersistenceCall(callName) {
									pos := fset.Position(x.Pos())
									out = append(out, Finding{
										File:     pf.relPath,
										Line:     pos.Line,
										FuncName: fnName,
										Category: "SWALLOWED_ERROR",
										Message:  fmt.Sprintf("zignorowany błąd zapisu/IO `_ = %s(...)` — możliwa cicha utrata danych", callName),
									})
								}
							}
						}
					}
				}
				return true
			})
		}
	}

	return out
}

func exprCallName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		lhs := exprCallName(x.X)
		if lhs != "" {
			return lhs + "." + x.Sel.Name
		}
		return x.Sel.Name
	default:
		return ""
	}
}

func isCriticalPersistenceCall(callName string) bool {
	critical := []string{
		"os.WriteFile", "os.Rename",
		"book.SaveBookMetadataToDisk", "book.SaveBookScenesToDisk", "book.SaveBookEditorStateToDisk",
		"book.SaveAppSettings", "book.WriteSaveState", "book.UpdateSaveAudioTimeInPlace",
		"book.RecordBookHistoryStep", "book.EnsureBookHistoryBaseline",
		"tpl.RenderTemplateTo",
		"s.saveAppSettings", "s.commitScenesMutation",
	}
	for _, c := range critical {
		if callName == c {
			return true
		}
	}
	return false
}

var (
	reCtrlConstDecl      = regexp.MustCompile(`(?m)^\s*const\s+([A-Za-z0-9_]+)\s*=\s*\{`)
	reCtrlRegisterCall   = regexp.MustCompile(`Dispatcher\.register\(\s*(?:"([A-Za-z0-9_]+)"|'([A-Za-z0-9_]+)'|[A-Za-z0-9_.]+\.moduleName)\s*,\s*([A-Za-z0-9_]+)\s*\)`)
	reCtrlMethodDecl     = regexp.MustCompile(`(?m)^\s*(?:async\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\s*\([^)]*\)\s*\{`)
	reBindingsHeader     = regexp.MustCompile(`(?m)^\s*bindings\s*\(\s*\)\s*\{`)
	reDataObjHeader      = regexp.MustCompile(`(?m)^\s*data\s*:\s*\{`)
	reSettingsObjHeader  = regexp.MustCompile(`(?m)^\s*settings\s*:\s*\{`)
	reCamelCaseIdent     = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	reNumberLiteral      = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	reBindingEventPair   = regexp.MustCompile(`([a-zA-Z]+)\s*:\s*([^,\n}]+)`)
	reDispatcherAction   = regexp.MustCompile(`^Dispatcher\.action\(\s*["']([A-Za-z0-9_]+)["']\s*\)$`)
	reDispatcherActAny   = regexp.MustCompile(`Dispatcher\.action\(([^)]*)\)`)
	reLocalThisMethod    = regexp.MustCompile(`^this\.([A-Za-z_$][A-Za-z0-9_$]*)$`)
	reDataKeyValLine     = regexp.MustCompile(`^\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*:\s*(.+?)\s*,?\s*$`)
	reThisDataField      = regexp.MustCompile(`\bthis\.data\.([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	reNamedCtrlDataField = regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*Controller)\.data\.([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	reTopLevelPropKey    = regexp.MustCompile(`^\s{2}([A-Za-z_$][A-Za-z0-9_$]*)\s*:`)
)

type parsedJSController struct {
	name       string
	file       string
	line       int
	methods    map[string]bool
	dataFields map[string]string // fieldName -> "bool" | "number" | "string" | "other"
	dataLines  map[string]int
}

func classifyJSDataLiteral(raw string) string {
	val := strings.TrimSpace(raw)
	if idx := strings.Index(val, "//"); idx >= 0 {
		val = strings.TrimSpace(val[:idx])
	}
	val = strings.TrimSuffix(val, ",")
	val = strings.TrimSpace(val)
	if val == "true" || val == "false" {
		return "bool"
	}
	if reNumberLiteral.MatchString(val) {
		return "number"
	}
	if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
		(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`)) {
		return "string"
	}
	return "other"
}

func stripJSCommentOnly(line string) string {
	inStr := byte(0)
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inStr != 0 {
			escaped = true
			continue
		}
		if inStr != 0 {
			if c == inStr {
				inStr = 0
			}
			continue
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i]
		}
		if c == '"' || c == '\'' || c == '`' {
			inStr = c
		}
	}
	return line
}

// 9. JS_BINDINGS_LOGIC, JS_CONTROLLER_CONTRACT, JS_CONTROLLER_DATA_MAP
func checkJSControllersAndDataContracts(rootDir string, fset *token.FileSet, goFiles []*parsedGoFile) []Finding {
	jsFiles := loadAllJSFiles(rootDir)
	if len(jsFiles) == 0 {
		return nil
	}

	var out []Finding
	controllersByName := make(map[string]*parsedJSController)
	controllersByBase := make(map[string]*parsedJSController)

	for _, jf := range jsFiles {
		methods := make(map[string]bool)
		for _, m := range reCtrlMethodDecl.FindAllStringSubmatch(jf.src, -1) {
			name := m[1]
			if name != "if" && name != "for" && name != "while" && name != "switch" && name != "catch" && name != "function" {
				methods[name] = true
			}
		}

		// 9a. Audit bindings() in every JS file for inline logic / arrow functions / non-local handlers
		if loc := reBindingsHeader.FindStringIndex(jf.src); loc != nil {
			openIdx := strings.IndexByte(jf.src[loc[0]:], '{') + loc[0]
			closeIdx := findMatchingJSBrace(jf.src, openIdx)
			startLine := 1 + strings.Count(jf.src[:loc[0]], "\n")
			if closeIdx > openIdx {
				bindingsBody := jf.src[openIdx+1 : closeIdx]
				bLines := strings.Split(bindingsBody, "\n")
				depth := 0
				for idx, rawLn := range bLines {
					lineNo := startLine + idx
					noComment := stripJSCommentOnly(rawLn)
					structureLn := stripJSLineCommentAndStrings(rawLn)

					if strings.Contains(structureLn, "=>") || strings.Contains(structureLn, "function") {
						out = append(out, Finding{
							File:     jf.rel,
							Line:     lineNo,
							FuncName: "bindings",
							Category: "JS_BINDINGS_LOGIC",
							Severity: "ERROR",
							Message:  "zakazana logika inline / funkcja strzałkowa wewnątrz bindings() — przekieruj zdarzenie do lokalnej metody kontrolera (this.<metoda>)",
						})
					}

					for _, actMatch := range reDispatcherActAny.FindAllStringSubmatch(noComment, -1) {
						inner := strings.TrimSpace(actMatch[1])
						if strings.Contains(inner, ",") {
							out = append(out, Finding{
								File:     jf.rel,
								Line:     lineNo,
								FuncName: "bindings",
								Category: "JS_BINDINGS_LOGIC",
								Severity: "ERROR",
								Message:  "zakazane mapowanie parametrów w Dispatcher.action(...) wewnątrz bindings() — użyj lokalnej metody kontrolera (this.<metoda>)",
							})
						}
					}

					prevDepth := depth
					for i := 0; i < len(structureLn); i++ {
						if structureLn[i] == '{' {
							depth++
						} else if structureLn[i] == '}' {
							depth--
						}
					}

					// Event handler pairs appear inside selector objects (depth >= 2 or single-line selector object)
					if prevDepth >= 2 || (prevDepth == 1 && strings.Contains(structureLn, "{") && strings.Contains(structureLn, "}")) {
						inspectSegment := noComment
						if prevDepth == 1 {
							firstBrace := strings.IndexByte(noComment, '{')
							lastBrace := strings.LastIndexByte(noComment, '}')
							if firstBrace >= 0 && lastBrace > firstBrace {
								inspectSegment = noComment[firstBrace+1 : lastBrace]
							}
						}
						for _, pm := range reBindingEventPair.FindAllStringSubmatch(inspectSegment, -1) {
							evName := strings.TrimSpace(pm[1])
							handlerExpr := strings.TrimSpace(pm[2])
							if reDispatcherAction.MatchString(handlerExpr) {
								continue
							}
							if tm := reLocalThisMethod.FindStringSubmatch(handlerExpr); tm != nil {
								methodName := tm[1]
								if !methods[methodName] {
									out = append(out, Finding{
										File:     jf.rel,
										Line:     lineNo,
										FuncName: "bindings",
										Category: "JS_BINDINGS_LOGIC",
										Severity: "ERROR",
										Message:  fmt.Sprintf("bindings() wskazuje na nieistniejącą lokalną metodę this.%s", methodName),
									})
								}
								continue
							}
							out = append(out, Finding{
								File:     jf.rel,
								Line:     lineNo,
								FuncName: "bindings",
								Category: "JS_BINDINGS_LOGIC",
								Severity: "ERROR",
								Message:  fmt.Sprintf("niedozwolony handler %q dla zdarzenia %q w bindings() — dozwolone wyłącznie this.<metoda> lub Dispatcher.action(\"...\")", handlerExpr, evName),
							})
						}
					}
				}
			}
		}

		// 9b. Audit *_controller.js structure and data contract
		baseName := filepath.Base(jf.rel)
		if !strings.HasSuffix(baseName, "_controller.js") {
			continue
		}

		cm := reCtrlConstDecl.FindStringSubmatchIndex(jf.src)
		if cm == nil {
			out = append(out, Finding{
				File:     jf.rel,
				Line:     1,
				FuncName: "<controller>",
				Category: "JS_CONTROLLER_CONTRACT",
				Severity: "ERROR",
				Message:  "plik *_controller.js musi deklarować obiekt const <Name>Controller = { ... }",
			})
			continue
		}
		ctrlName := jf.src[cm[2]:cm[3]]
		ctrlLine := 1 + strings.Count(jf.src[:cm[0]], "\n")

		regMatch := reCtrlRegisterCall.FindStringSubmatch(jf.src)
		if regMatch == nil || regMatch[3] != ctrlName || (regMatch[1] != "" && regMatch[1] != ctrlName) || (regMatch[2] != "" && regMatch[2] != ctrlName) {
			out = append(out, Finding{
				File:     jf.rel,
				Line:     ctrlLine,
				FuncName: ctrlName,
				Category: "JS_CONTROLLER_CONTRACT",
				Severity: "ERROR",
				Message:  fmt.Sprintf("kontroler %s musi wywoływać Dispatcher.register(%q, %s)", ctrlName, ctrlName, ctrlName),
			})
		}

		if !reBindingsHeader.MatchString(jf.src) {
			out = append(out, Finding{
				File:     jf.rel,
				Line:     ctrlLine,
				FuncName: ctrlName,
				Category: "JS_CONTROLLER_CONTRACT",
				Severity: "ERROR",
				Message:  fmt.Sprintf("kontroler %s nie deklaruje metody bindings()", ctrlName),
			})
		}

		if !reSettingsObjHeader.MatchString(jf.src) {
			out = append(out, Finding{
				File:     jf.rel,
				Line:     ctrlLine,
				FuncName: ctrlName,
				Category: "JS_CONTROLLER_CONTRACT",
				Severity: "ERROR",
				Message:  fmt.Sprintf("kontroler %s nie deklaruje obiektu settings: { ... }", ctrlName),
			})
		}

		// Top-level property check on controller object: only data: and settings: are allowed as properties
		for idx, ln := range jf.lines {
			if pm := reTopLevelPropKey.FindStringSubmatch(ln); pm != nil {
				propKey := pm[1]
				if propKey != "data" && propKey != "settings" {
					out = append(out, Finding{
						File:     jf.rel,
						Line:     idx + 1,
						FuncName: ctrlName,
						Category: "JS_CONTROLLER_CONTRACT",
						Severity: "ERROR",
						Message:  fmt.Sprintf("niedozwolone pole najwyższego poziomu %q w %s — dozwolone wyłącznie sekcje data, bindings(), settings oraz metody", propKey, ctrlName),
					})
				}
			}
		}

		dataFields := make(map[string]string)
		dataLines := make(map[string]int)
		dataLoc := reDataObjHeader.FindStringIndex(jf.src)
		if dataLoc == nil {
			out = append(out, Finding{
				File:     jf.rel,
				Line:     ctrlLine,
				FuncName: ctrlName,
				Category: "JS_CONTROLLER_CONTRACT",
				Severity: "ERROR",
				Message:  fmt.Sprintf("kontroler %s nie deklaruje obiektu data: { ... }", ctrlName),
			})
		} else {
			openIdx := strings.IndexByte(jf.src[dataLoc[0]:], '{') + dataLoc[0]
			closeIdx := findMatchingJSBrace(jf.src, openIdx)
			dataStartLine := 1 + strings.Count(jf.src[:dataLoc[0]], "\n")
			if closeIdx > openIdx {
				dataBody := jf.src[openIdx+1 : closeIdx]
				for offset, dLn := range strings.Split(dataBody, "\n") {
					lineNo := dataStartLine + offset
					cleanDLn := stripJSCommentOnly(dLn)
					if strings.TrimSpace(cleanDLn) == "" {
						continue
					}
					if km := reDataKeyValLine.FindStringSubmatch(cleanDLn); km != nil {
						fKey := km[1]
						fVal := km[2]
						dataFields[fKey] = classifyJSDataLiteral(fVal)
						dataLines[fKey] = lineNo
						if !reCamelCaseIdent.MatchString(fKey) {
							out = append(out, Finding{
								File:     jf.rel,
								Line:     lineNo,
								FuncName: ctrlName,
								Category: "JS_CONTROLLER_DATA_MAP",
								Severity: "ERROR",
								Message:  fmt.Sprintf("pole %s.data.%s musi być w notacji camelCase", ctrlName, fKey),
							})
						}
					}
				}
			}
		}

		// Check all this.data.<field> accesses inside this controller file
		for idx, ln := range jf.lines {
			clean := stripJSCommentOnly(ln)
			for _, dm := range reThisDataField.FindAllStringSubmatch(clean, -1) {
				field := dm[1]
				if _, ok := dataFields[field]; !ok {
					out = append(out, Finding{
						File:     jf.rel,
						Line:     idx + 1,
						FuncName: ctrlName,
						Category: "JS_CONTROLLER_DATA_MAP",
						Severity: "ERROR",
						Message:  fmt.Sprintf("odwołanie do niezadeklarowanego pola this.data.%s w %s (brak klucza w %s.data)", field, ctrlName, ctrlName),
					})
				}
			}
		}

		parsedCtrl := &parsedJSController{
			name:       ctrlName,
			file:       jf.rel,
			line:       ctrlLine,
			methods:    methods,
			dataFields: dataFields,
			dataLines:  dataLines,
		}
		controllersByName[ctrlName] = parsedCtrl
		controllersByBase[baseName] = parsedCtrl
	}

	// Check cross-controller <CtrlName>.data.<field> accesses across all JS files
	for _, jf := range jsFiles {
		for idx, ln := range jf.lines {
			clean := stripJSCommentOnly(ln)
			for _, cm := range reNamedCtrlDataField.FindAllStringSubmatch(clean, -1) {
				targetCtrlName := cm[1]
				field := cm[2]
				targetCtrl, ok := controllersByName[targetCtrlName]
				if !ok {
					out = append(out, Finding{
						File:     jf.rel,
						Line:     idx + 1,
						FuncName: targetCtrlName,
						Category: "JS_CONTROLLER_DATA_MAP",
						Severity: "ERROR",
						Message:  fmt.Sprintf("odwołanie do nieistniejącego kontrolera %s.data.%s", targetCtrlName, field),
					})
					continue
				}
				if _, exists := targetCtrl.dataFields[field]; !exists {
					out = append(out, Finding{
						File:     jf.rel,
						Line:     idx + 1,
						FuncName: targetCtrlName,
						Category: "JS_CONTROLLER_DATA_MAP",
						Severity: "ERROR",
						Message:  fmt.Sprintf("odwołanie do niezadeklarowanego pola %s.data.%s (brak klucza w %s.data)", targetCtrlName, field, targetCtrlName),
					})
				}
			}
		}
	}

	// 9c. Verify 1:1 mapping between Go view_*Script structs (ScriptData) and JS Controller.data
	for _, pf := range goFiles {
		if pf.isTest || !strings.HasPrefix(pf.relPath, "internal/views/view_") {
			continue
		}
		viewBase := filepath.Base(pf.relPath) // view_<snake>.go
		snakeName := strings.TrimSuffix(strings.TrimPrefix(viewBase, "view_"), ".go")
		expectedCtrlBase := snakeName + "_controller.js"
		ctrl := controllersByBase[expectedCtrlBase]

		for _, decl := range pf.file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil || !strings.HasSuffix(ts.Name.Name, "Script") {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				structName := ts.Name.Name
				structLine := fset.Position(ts.Pos()).Line

				if ctrl == nil {
					out = append(out, Finding{
						File:     pf.relPath,
						Line:     structLine,
						FuncName: structName,
						Category: "JS_CONTROLLER_DATA_MAP",
						Severity: "ERROR",
						Message:  fmt.Sprintf("struktura %s w %s nie ma odpowiadającego kontrolera JS %s", structName, pf.relPath, expectedCtrlBase),
					})
					continue
				}

				// Verify that ScriptData is actually populated with this struct in pf
				if !strings.Contains(string(pf.src), "ScriptData:") {
					out = append(out, Finding{
						File:     pf.relPath,
						Line:     structLine,
						FuncName: structName,
						Category: "JS_CONTROLLER_DATA_MAP",
						Severity: "ERROR",
						Message:  fmt.Sprintf("struktura %s w %s nie jest przypisana do pola ScriptData w TemplateChunk", structName, pf.relPath),
					})
				}

				for _, field := range st.Fields.List {
					goType := exprCallName(field.Type)
					expectedJSType := ""
					switch goType {
					case "bool":
						expectedJSType = "bool"
					case "int", "int8", "int16", "int32", "int64", "float32", "float64":
						expectedJSType = "number"
					case "string":
						expectedJSType = "string"
					}
					for _, ident := range field.Names {
						fName := ident.Name
						fLine := fset.Position(ident.Pos()).Line
						if !reCamelCaseIdent.MatchString(fName) {
							out = append(out, Finding{
								File:     pf.relPath,
								Line:     fLine,
								FuncName: structName,
								Category: "JS_CONTROLLER_DATA_MAP",
								Severity: "ERROR",
								Message:  fmt.Sprintf("pole %s.%s musi być w notacji camelCase dla bezpośredniego mapowania 1:1 do %s.data", structName, fName, ctrl.name),
							})
						}
						jsType, exists := ctrl.dataFields[fName]
						if !exists {
							out = append(out, Finding{
								File:     ctrl.file,
								Line:     ctrl.line,
								FuncName: ctrl.name,
								Category: "JS_CONTROLLER_DATA_MAP",
								Severity: "ERROR",
								Message:  fmt.Sprintf("pole %s.%s (%s:%d) nie jest zadeklarowane w %s.data — hydratacja FDP pominie to pole", structName, fName, pf.relPath, fLine, ctrl.name),
							})
							continue
						}
						if expectedJSType != "" && jsType != expectedJSType {
							out = append(out, Finding{
								File:     ctrl.file,
								Line:     ctrl.dataLines[fName],
								FuncName: ctrl.name,
								Category: "JS_CONTROLLER_DATA_MAP",
								Severity: "ERROR",
								Message:  fmt.Sprintf("niezgodny typ domyślny dla %s.data.%s: w Go %s.%s ma typ %s, a w JS %s (wymagany literał typu %s dla poprawnej hydratacji FDP)", ctrl.name, fName, structName, fName, goType, jsType, expectedJSType),
							})
						}
					}
				}
			}
		}
	}

	return out
}

