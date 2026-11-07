package audit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Finding represents a single static analysis issue discovered in the codebase.
type Finding struct {
	File     string
	Line     int
	FuncName string
	Category string
	Message  string
}

func (f Finding) String() string {
	fn := f.FuncName
	if fn == "" {
		fn = "<pkg>"
	}
	return fmt.Sprintf("%s:%d | %s | %s | %s", f.File, f.Line, fn, f.Category, f.Message)
}

type parsedGoFile struct {
	relPath string
	absPath string
	pkgName string
	isTest  bool
	file    *ast.File
	src     []byte
}

// Run executes all static analysis passes on the repository rooted at rootDir.
func Run(rootDir string) ([]Finding, error) {
	fset := token.NewFileSet()
	var goFiles []*parsedGoFile

	walkDirs := []string{"internal", "cmd"}
	for _, sub := range walkDirs {
		base := filepath.Join(rootDir, sub)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			raw, rErr := os.ReadFile(path)
			if rErr != nil {
				return rErr
			}
			pf, pErr := parser.ParseFile(fset, path, raw, parser.ParseComments)
			if pErr != nil {
				return nil
			}
			rel, _ := filepath.Rel(rootDir, path)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/audit/") {
				return nil
			}
			goFiles = append(goFiles, &parsedGoFile{
				relPath: rel,
				absPath: path,
				pkgName: pf.Name.Name,
				isTest:  strings.HasSuffix(rel, "_test.go"),
				file:    pf,
				src:     raw,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	var findings []Finding
	findings = append(findings, checkDeadGoFuncsAndStructs(fset, goFiles)...)
	findings = append(findings, checkJSGhostAndUnusedFuncs(rootDir)...)
	findings = append(findings, checkJSDeepAnalysis(rootDir)...)
	findings = append(findings, checkUnusedActionsAndTplVars(rootDir, fset, goFiles)...)
	findings = append(findings, checkHardcodedGeometryAndDOM(fset, goFiles)...)
	findings = append(findings, checkSuspiciousStrings(fset, goFiles)...)
	findings = append(findings, checkASTDuplicatesAndAntiPatterns(fset, goFiles)...)
	findings = append(findings, checkSecurityLoopsAndReliability(rootDir, fset, goFiles)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Category != findings[j].Category {
			return findings[i].Category < findings[j].Category
		}
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})

	return findings, nil
}

// PrintReport writes a formatted audit report with category counts to w.
func PrintReport(w io.Writer, findings []Finding, filterCategory string) int {
	return PrintReportWithLang(w, findings, filterCategory, "")
}

// PrintReportWithLang writes a formatted audit report filtered by optional category and language ("js", "go", "tpl").
func PrintReportWithLang(w io.Writer, findings []Finding, filterCategory, filterLang string) int {
	filter := strings.ToUpper(strings.TrimSpace(filterCategory))
	lang := strings.ToLower(strings.TrimSpace(filterLang))
	counts := make(map[string]int)
	var filtered []Finding
	for _, f := range findings {
		if lang != "" && !matchesLanguageFilter(f.File, lang) {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToUpper(f.Category), filter) {
			continue
		}
		filtered = append(filtered, f)
		counts[f.Category]++
	}

	fmt.Fprintf(w, "================================================================================\n")
	fmt.Fprintf(w, " MOJRA STATIC CODE AUDIT (%d wykrytych miejsc)\n", len(filtered))
	fmt.Fprintf(w, " Format: plik:linia | funkcja | KATEGORIA | opis\n")
	fmt.Fprintf(w, "================================================================================\n\n")

	for _, f := range filtered {
		fmt.Fprintln(w, f.String())
	}

	if len(counts) > 0 {
		fmt.Fprintf(w, "\n--------------------------------------------------------------------------------\n")
		fmt.Fprintf(w, " PODSUMOWANIE KATEGORII:\n")
		var cats []string
		for k := range counts {
			cats = append(cats, k)
		}
		sort.Strings(cats)
		for _, c := range cats {
			fmt.Fprintf(w, "  %-22s : %d\n", c, counts[c])
		}
		fmt.Fprintf(w, "--------------------------------------------------------------------------------\n")
	}
	return len(filtered)
}

func matchesLanguageFilter(file, lang string) bool {
	switch lang {
	case "js", "javascript":
		return strings.HasSuffix(file, ".js")
	case "go", "golang":
		return strings.HasSuffix(file, ".go")
	case "tpl", "html", "tss", "css":
		return strings.HasSuffix(file, ".tpl") || strings.HasSuffix(file, ".tss") || strings.HasSuffix(file, ".css")
	default:
		return true
	}
}

func enclosingFuncName(file *ast.File, pos token.Pos) string {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if pos >= fn.Pos() && pos <= fn.End() {
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				recvType := exprTypeName(fn.Recv.List[0].Type)
				if recvType != "" {
					return "(" + recvType + ")." + fn.Name.Name
				}
			}
			return fn.Name.Name
		}
	}
	return "<pkg>"
}

func exprTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprTypeName(t.X)
	default:
		return ""
	}
}

// 1. DEAD_CODE: Unused Go functions/methods & struct fields
func checkDeadGoFuncsAndStructs(fset *token.FileSet, files []*parsedGoFile) []Finding {
	type funcDef struct {
		file     string
		line     int
		funcName string
		bareName string
		pkg      string
	}

	var defs []funcDef
	prodIdents := make(map[string]int)
	testIdents := make(map[string]int)

	skipNames := map[string]bool{
		"main": true, "init": true, "String": true, "Error": true,
		"ServeHTTP": true, "Handler": true,
	}

	for _, pf := range files {
		if !pf.isTest && strings.HasPrefix(pf.relPath, "internal/") {
			for _, decl := range pf.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name == nil {
					continue
				}
				name := fn.Name.Name
				if skipNames[name] {
					continue
				}
				pos := fset.Position(fn.Pos())
				fullName := name
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					fullName = "(" + exprTypeName(fn.Recv.List[0].Type) + ")." + name
				}
				defs = append(defs, funcDef{
					file:     pf.relPath,
					line:     pos.Line,
					funcName: fullName,
					bareName: name,
					pkg:      pf.pkgName,
				})
			}
		}

		ast.Inspect(pf.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				// Do not count the function's own declaration identifier as a call site
				if x.Recv != nil {
					ast.Inspect(x.Recv, func(rn ast.Node) bool {
						if id, ok := rn.(*ast.Ident); ok {
							if pf.isTest {
								testIdents[id.Name]++
							} else {
								prodIdents[id.Name]++
							}
						}
						return true
					})
				}
				if x.Type != nil {
					ast.Inspect(x.Type, func(tn ast.Node) bool {
						if id, ok := tn.(*ast.Ident); ok {
							if pf.isTest {
								testIdents[id.Name]++
							} else {
								prodIdents[id.Name]++
							}
						}
						return true
					})
				}
				if x.Body != nil {
					ast.Inspect(x.Body, func(bn ast.Node) bool {
						if id, ok := bn.(*ast.Ident); ok {
							if pf.isTest {
								testIdents[id.Name]++
							} else {
								prodIdents[id.Name]++
							}
						}
						return true
					})
				}
				return false
			case *ast.Ident:
				if pf.isTest {
					testIdents[x.Name]++
				} else {
					prodIdents[x.Name]++
				}
			}
			return true
		})
	}

	var out []Finding
	for _, d := range defs {
		prodCount := prodIdents[d.bareName]
		testCount := testIdents[d.bareName]
		if prodCount == 0 && testCount == 0 {
			out = append(out, Finding{
				File:     d.file,
				Line:     d.line,
				FuncName: d.funcName,
				Category: "DEAD_CODE",
				Message:  fmt.Sprintf("funkcja/metoda %q nie jest nigdzie wywoływana", d.funcName),
			})
		} else if prodCount == 0 && testCount > 0 {
			out = append(out, Finding{
				File:     d.file,
				Line:     d.line,
				FuncName: d.funcName,
				Category: "DEAD_CODE",
				Message:  fmt.Sprintf("funkcja/metoda %q wywoływana wyłącznie w testach (%d×), brak użycia w kodzie produkcyjnym", d.funcName, testCount),
			})
		}
	}

	// Also check constant boolean methods like IsMultiFileBook() returning true
	for _, pf := range files {
		if pf.isTest {
			continue
		}
		for _, decl := range pf.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || len(fn.Body.List) != 1 {
				continue
			}
			ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			if id, ok := ret.Results[0].(*ast.Ident); ok && (id.Name == "true" || id.Name == "false") {
				if fn.Type.Params != nil && len(fn.Type.Params.List) == 0 {
					pos := fset.Position(fn.Pos())
					out = append(out, Finding{
						File:     pf.relPath,
						Line:     pos.Line,
						FuncName: enclosingFuncName(pf.file, fn.Pos()),
						Category: "DEAD_CODE",
						Message:  fmt.Sprintf("metoda-wydmuszka zawsze zwraca stałą %q (martwa flaga architektoniczna)", id.Name),
					})
				}
			}
		}
	}

	return out
}

// 2. GHOST_CALL & DEAD_CODE in JS
func checkJSGhostAndUnusedFuncs(rootDir string) []Finding {
	jsDir := filepath.Join(rootDir, "app", "js")
	entries, err := os.ReadDir(jsDir)
	if err != nil {
		return nil
	}

	type jsFuncDef struct {
		file string
		line int
		name string
	}
	definedFuncs := make(map[string]jsFuncDef)
	var allJSFiles []struct {
		rel   string
		lines []string
		full  string
	}

	funcDeclRe := regexp.MustCompile(`^\s*(?:async\s+)?function\s+([A-Za-z0-9_]+)\s*\(`)
	typeofFnRe := regexp.MustCompile(`typeof\s+([A-Za-z0-9_]+)\s*===\s*"function"`)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		fullPath := filepath.Join(jsDir, e.Name())
		raw, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		rel := "app/js/" + e.Name()
		src := string(raw)
		lines := strings.Split(src, "\n")
		allJSFiles = append(allJSFiles, struct {
			rel   string
			lines []string
			full  string
		}{rel: rel, lines: lines, full: src})

		for idx, ln := range lines {
			if m := funcDeclRe.FindStringSubmatch(ln); m != nil {
				definedFuncs[m[1]] = jsFuncDef{file: rel, line: idx + 1, name: m[1]}
			}
		}
	}

	var out []Finding
	browserGlobals := map[string]bool{
		"ResizeObserver":        true,
		"MutationObserver":      true,
		"IntersectionObserver":  true,
		"PerformanceObserver":   true,
		"requestAnimationFrame": true,
		"cancelAnimationFrame":  true,
		"AudioContext":          true,
		"webkitAudioContext":    true,
		"structuredClone":       true,
	}
	for _, jf := range allJSFiles {
		curFunc := "<global>"
		for idx, ln := range jf.lines {
			if m := funcDeclRe.FindStringSubmatch(ln); m != nil {
				curFunc = m[1]
			}
			for _, m := range typeofFnRe.FindAllStringSubmatch(ln, -1) {
				fnName := m[1]
				if browserGlobals[fnName] {
					continue
				}
				if _, ok := definedFuncs[fnName]; !ok {
					out = append(out, Finding{
						File:     jf.rel,
						Line:     idx + 1,
						FuncName: curFunc,
						Category: "GHOST_CALL",
						Message:  fmt.Sprintf("wywołanie nieistniejącej funkcji %q zamaskowane przez typeof === \"function\"", fnName),
					})
				}
			}
		}
	}

	// Check unused JS functions
	for name, def := range definedFuncs {
		wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
		totalMatches := 0
		for _, jf := range allJSFiles {
			matches := wordRe.FindAllStringIndex(jf.full, -1)
			totalMatches += len(matches)
		}
		if totalMatches <= 1 {
			out = append(out, Finding{
				File:     def.file,
				Line:     def.line,
				FuncName: name,
				Category: "DEAD_CODE",
				Message:  fmt.Sprintf("funkcja JS %q zadeklarowana, ale nigdy niewywołana", name),
			})
		}
	}

	return out
}

// 3. UNUSED_TPL_VAR & UNUSED_ACTION (Go <-> .tpl / .js contract)
func checkUnusedActionsAndTplVars(rootDir string, fset *token.FileSet, goFiles []*parsedGoFile) []Finding {
	var allTplContent strings.Builder
	tplDir := filepath.Join(rootDir, "app", "templates")
	if entries, err := os.ReadDir(tplDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".tpl") {
				if raw, err := os.ReadFile(filepath.Join(tplDir, e.Name())); err == nil {
					allTplContent.Write(raw)
					allTplContent.WriteByte('\n')
				}
			}
		}
	}
	tplText := allTplContent.String()

	var allJSContent strings.Builder
	jsDir := filepath.Join(rootDir, "app", "js")
	if entries, err := os.ReadDir(jsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
				if raw, err := os.ReadFile(filepath.Join(jsDir, e.Name())); err == nil {
					allJSContent.Write(raw)
					allJSContent.WriteByte('\n')
				}
			}
		}
	}
	jsText := allJSContent.String()

	var out []Finding

	// Check actionSpecs entries vs .tpl and .js usage
	for _, pf := range goFiles {
		if pf.relPath != "internal/gui/actions_dispatch.go" {
			continue
		}
		ast.Inspect(pf.file, func(n ast.Node) bool {
			vs, ok := n.(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 || vs.Names[0].Name != "actionSpecs" || len(vs.Values) == 0 {
				return true
			}
			cl, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				lit, ok := kv.Key.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				actName, _ := strconv.Unquote(lit.Value)
				if !strings.Contains(tplText, actName) && !strings.Contains(jsText, actName) {
					// Check if referenced in ConfirmPendingAction in Go
					usedInConfirm := false
					for _, gf := range goFiles {
						if gf.isTest || gf.relPath == pf.relPath {
							continue
						}
						if strings.Contains(string(gf.src), `"`+actName+`"`) {
							usedInConfirm = true
							break
						}
					}
					if !usedInConfirm {
						pos := fset.Position(lit.Pos())
						out = append(out, Finding{
							File:     pf.relPath,
							Line:     pos.Line,
							FuncName: "actionSpecs",
							Category: "DEAD_CODE",
							Message:  fmt.Sprintf("akcja serwera %q w actionSpecs nie występuje w żadnym .tpl ani .js", actName),
						})
					}
				}
			}
			return false
		})
	}

	// Check vars["key"] in internal/gui/render*.go vs .tpl usage
	varsIndexRe := regexp.MustCompile(`\bvars\["([A-Za-z0-9_]+)"\]\s*=`)
	for _, pf := range goFiles {
		if pf.isTest || !strings.HasPrefix(pf.relPath, "internal/gui/render") {
			continue
		}
		lines := strings.Split(string(pf.src), "\n")
		seenKeyInFile := make(map[string]bool)
		for idx, ln := range lines {
			for _, m := range varsIndexRe.FindAllStringSubmatch(ln, -1) {
				key := m[1]
				if seenKeyInFile[key] {
					continue
				}
				seenKeyInFile[key] = true
				// Check if key is referenced anywhere in app/templates/*.tpl or other Go files (e.g. buildStudioGraphPayload)
				if !strings.Contains(tplText, key) {
					pos := pf.file.Pos()
					// Find approximate AST pos for enclosing function name
					lineOff := 0
					for i := 0; i < idx; i++ {
						lineOff += len(lines[i]) + 1
					}
					fnName := enclosingFuncName(pf.file, token.Pos(int(pos)+lineOff))
					out = append(out, Finding{
						File:     pf.relPath,
						Line:     idx + 1,
						FuncName: fnName,
						Category: "UNUSED_TPL_VAR",
						Message:  fmt.Sprintf("klucz kontekstu vars[%q] jest ustawiany w Go, ale żaden szablon .tpl go nie używa", key),
					})
				}
			}
		}

		// Also check map[string]any{"prop": ...} keys in render*.go that are never read via .prop in .tpl or Go
		ast.Inspect(pf.file, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			mt, ok := cl.Type.(*ast.MapType)
			if !ok {
				return true
			}
			if kIdent, ok := mt.Key.(*ast.Ident); !ok || kIdent.Name != "string" {
				return true
			}
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				lit, ok := kv.Key.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				propName, err := strconv.Unquote(lit.Value)
				if err != nil || propName == "" || seenKeyInFile["."+propName] {
					continue
				}
				// Check if referenced as .propName in .tpl or as ["propName"] in Go
				if !strings.Contains(tplText, "."+propName) && !strings.Contains(string(pf.src), `["`+propName+`"]`) {
					seenKeyInFile["."+propName] = true
					pos := fset.Position(lit.Pos())
					out = append(out, Finding{
						File:     pf.relPath,
						Line:     pos.Line,
						FuncName: enclosingFuncName(pf.file, lit.Pos()),
						Category: "UNUSED_TPL_VAR",
						Message:  fmt.Sprintf("pole obiektu map[string]any[%q] nie jest odczytywane w żadnym .tpl (.%s) ani w Go", propName, propName),
					})
				}
			}
			return true
		})
	}

	return out
}

// 4. DOM_ID_IN_GO: Raw DOM IDs or HTML/SVG markup generated in Go outside tpl engine
func checkHardcodedGeometryAndDOM(fset *token.FileSet, goFiles []*parsedGoFile) []Finding {
	var out []Finding

	domSelectorRe := regexp.MustCompile(`^#[a-z][a-z0-9_-]+$`)
	htmlTagRe := regexp.MustCompile(`<(?:div|span|svg|button|input|p|hr|ul|li|form|section|header|footer)\b`)

	for _, pf := range goFiles {
		if pf.isTest || !strings.HasPrefix(pf.relPath, "internal/") {
			continue
		}

		ast.Inspect(pf.file, func(n ast.Node) bool {
			if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.CONST {
				return false
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			pos := fset.Position(lit.Pos())
			fnName := enclosingFuncName(pf.file, lit.Pos())

			val, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if domSelectorRe.MatchString(val) {
				out = append(out, Finding{
					File:     pf.relPath,
					Line:     pos.Line,
					FuncName: fnName,
					Category: "DOM_ID_IN_GO",
					Message:  fmt.Sprintf("selektor DOM %q zaszyty bezpośrednio w kodzie Go", val),
				})
			} else if htmlTagRe.MatchString(val) && !strings.HasPrefix(pf.relPath, "internal/tpl/") && pf.relPath != "internal/book/sanitize.go" {
				out = append(out, Finding{
					File:     pf.relPath,
					Line:     pos.Line,
					FuncName: fnName,
					Category: "DOM_ID_IN_GO",
					Message:  fmt.Sprintf("surowy znacznik HTML/SVG %q generowany w kodzie Go poza silnikiem szablonów", truncateStr(val, 60)),
				})
			}
			return true
		})
	}

	return out
}

// 5. SUSPICIOUS_STRING: Hardcoded OS paths, IPs, secrets, and UI tutorial copy
func checkSuspiciousStrings(fset *token.FileSet, goFiles []*parsedGoFile) []Finding {
	var out []Finding

	absPathRe := regexp.MustCompile(`(?i)(^/Users/|^/home/|^/var/folders/|^[A-Z]:\\)`)
	ipAddrRe := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	secretKeyRe := regexp.MustCompile(`(?i)(api[_-]?key|secret|password|passwd|private[_-]?key|bearer\s+[a-z0-9_\-\.]{12,}|sk-[a-z0-9]{16,}|ghp_[a-z0-9]{16,})`)
	uiExplainRe := regexp.MustCompile(`(?i)(kliknij,?\s+aby|kliknij\s+przycisk|przeciągnij,?\s+aby|przeciągnij\s+przewód|kliknij\s+kwadrat)`)

	for _, pf := range goFiles {
		if pf.isTest {
			continue
		}
		ast.Inspect(pf.file, func(n ast.Node) bool {
			if _, ok := n.(*ast.ImportSpec); ok {
				return false
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, err := strconv.Unquote(lit.Value)
			if err != nil || val == "" {
				return true
			}
			pos := fset.Position(lit.Pos())
			fnName := enclosingFuncName(pf.file, lit.Pos())

			if absPathRe.MatchString(val) {
				out = append(out, Finding{
					File:     pf.relPath,
					Line:     pos.Line,
					FuncName: fnName,
					Category: "SUSPICIOUS_STRING",
					Message:  fmt.Sprintf("ścieżka bezwzględna systemu plików w stringu: %q", val),
				})
			}

			if m := ipAddrRe.FindString(val); m != "" && m != "127.0.0.1" && m != "1.0.0.0" {
				out = append(out, Finding{
					File:     pf.relPath,
					Line:     pos.Line,
					FuncName: fnName,
					Category: "SUSPICIOUS_STRING",
					Message:  fmt.Sprintf("zahardkodowany adres IP (%s) w kodzie produkcyjnym: %q", m, truncateStr(val, 60)),
				})
			}

			if secretKeyRe.MatchString(val) {
				out = append(out, Finding{
					File:     pf.relPath,
					Line:     pos.Line,
					FuncName: fnName,
					Category: "SUSPICIOUS_STRING",
					Message:  fmt.Sprintf("podejrzany wzorzec hasła/klucza/sekretu: %q", truncateStr(val, 50)),
				})
			}

			if uiExplainRe.MatchString(val) {
				out = append(out, Finding{
					File:     pf.relPath,
					Line:     pos.Line,
					FuncName: fnName,
					Category: "SUSPICIOUS_STRING",
					Message:  fmt.Sprintf("instrukcja obsługi UI zaszyta w stringu Go (naruszenie zasady 'Never Explain the UI'): %q", truncateStr(val, 70)),
				})
			}

			return true
		})
	}

	return out
}

// 6. AST_DUPLICATE in Go
func checkASTDuplicatesAndAntiPatterns(fset *token.FileSet, goFiles []*parsedGoFile) []Finding {
	var out []Finding

	// 6a. Detect structural AST clones across large functions (>= 25 statements, high normalized AST similarity)
	type funcFingerprint struct {
		file     string
		line     int
		funcName string
		stmtCnt  int
		tokens   []string
	}
	var fps []funcFingerprint

	for _, pf := range goFiles {
		if pf.isTest || !strings.HasPrefix(pf.relPath, "internal/") {
			continue
		}
		for _, decl := range pf.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || len(fn.Body.List) < 12 {
				continue
			}
			var toks []string
			stmtCount := 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if n == nil {
					return false
				}
				if _, isStmt := n.(ast.Stmt); isStmt {
					stmtCount++
				}
				toks = append(toks, astNodeToken(n))
				return true
			})
			if stmtCount < 25 || len(toks) < 90 {
				continue
			}
			pos := fset.Position(fn.Pos())
			fps = append(fps, funcFingerprint{
				file:     pf.relPath,
				line:     pos.Line,
				funcName: enclosingFuncName(pf.file, fn.Pos()),
				stmtCnt:  stmtCount,
				tokens:   toks,
			})
		}
	}

	reportedPair := make(map[string]bool)
	for i := 0; i < len(fps); i++ {
		for j := i + 1; j < len(fps); j++ {
			a, b := fps[i], fps[j]
			sim := tokenSequenceSimilarity(a.tokens, b.tokens)
			if sim >= 0.88 {
				pairKey := a.file + ":" + a.funcName + "<->" + b.file + ":" + b.funcName
				if reportedPair[pairKey] {
					continue
				}
				reportedPair[pairKey] = true
				out = append(out, Finding{
					File:     b.file,
					Line:     b.line,
					FuncName: b.funcName,
					Category: "AST_DUPLICATE",
					Message:  fmt.Sprintf("wysokie podobieństwo drzewa AST (%.0f%%) do %s:%d (%s)", sim*100, a.file, a.line, a.funcName),
				})
			}
		}
	}

	// 6c. Repeated >=4 line blocks repeated >=3 times across Go files
	var goData []jsFileData
	for _, pf := range goFiles {
		if pf.isTest || !strings.HasPrefix(pf.relPath, "internal/") {
			continue
		}
		goData = append(goData, jsFileData{
			rel:   pf.relPath,
			src:   string(pf.src),
			lines: strings.Split(string(pf.src), "\n"),
		})
	}
	out = append(out, detectRepeatedCodeBlocks(goData, true)...)

	return out
}

func astNodeToken(n ast.Node) string {
	switch x := n.(type) {
	case *ast.IfStmt:
		return "IF"
	case *ast.ForStmt:
		return "FOR"
	case *ast.RangeStmt:
		return "RANGE"
	case *ast.SwitchStmt:
		return "SWITCH"
	case *ast.CaseClause:
		return "CASE"
	case *ast.ReturnStmt:
		return "RET"
	case *ast.AssignStmt:
		return "ASSIGN:" + x.Tok.String()
	case *ast.BinaryExpr:
		return "BIN:" + x.Op.String()
	case *ast.UnaryExpr:
		return "UN:" + x.Op.String()
	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			return "CALL:." + sel.Sel.Name
		}
		if id, ok := x.Fun.(*ast.Ident); ok {
			return "CALL:" + id.Name
		}
		return "CALL"
	case *ast.IndexExpr:
		return "INDEX"
	case *ast.CompositeLit:
		return "COMPLIT"
	default:
		return ""
	}
}

func tokenSequenceSimilarity(a, b []string) float64 {
	var fa, fb []string
	for _, t := range a {
		if t != "" {
			fa = append(fa, t)
		}
	}
	for _, t := range b {
		if t != "" {
			fb = append(fb, t)
		}
	}
	if len(fa) == 0 || len(fb) == 0 {
		return 0
	}
	minLen, maxLen := len(fa), len(fb)
	if minLen > maxLen {
		minLen, maxLen = maxLen, minLen
	}
	if float64(minLen)/float64(maxLen) < 0.75 {
		return 0
	}

	// Multiset intersection + bigram overlap for fast O(N) structural similarity
	countsA := make(map[string]int, len(fa))
	for _, t := range fa {
		countsA[t]++
	}
	common := 0
	for _, t := range fb {
		if countsA[t] > 0 {
			countsA[t]--
			common++
		}
	}
	unigramSim := float64(2*common) / float64(len(fa)+len(fb))
	if unigramSim < 0.80 {
		return unigramSim
	}

	bigramsA := make(map[string]int, len(fa))
	for i := 0; i+1 < len(fa); i++ {
		bigramsA[fa[i]+"|"+fa[i+1]]++
	}
	biCommon := 0
	for i := 0; i+1 < len(fb); i++ {
		k := fb[i] + "|" + fb[i+1]
		if bigramsA[k] > 0 {
			bigramsA[k]--
			biCommon++
		}
	}
	bigramSim := float64(2*biCommon) / float64(len(fa)+len(fb)-2)
	return bigramSim
}

func extractGoFuncHeaderName(line string) string {
	s := strings.TrimPrefix(strings.TrimSpace(line), "func ")
	if strings.HasPrefix(s, "(") {
		if closeIdx := strings.IndexByte(s, ')'); closeIdx > 0 {
			recv := strings.TrimSpace(s[1:closeIdx])
			fields := strings.Fields(recv)
			recvType := fields[len(fields)-1]
			rest := strings.TrimSpace(s[closeIdx+1:])
			if openParen := strings.IndexByte(rest, '('); openParen > 0 {
				return "(" + recvType + ")." + strings.TrimSpace(rest[:openParen])
			}
		}
	}
	if openParen := strings.IndexByte(s, '('); openParen > 0 {
		return strings.TrimSpace(s[:openParen])
	}
	return s
}

func truncateStr(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
