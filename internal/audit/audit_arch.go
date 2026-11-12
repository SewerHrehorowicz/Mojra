package audit

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	reJSStrMethodCall = regexp.MustCompile(`\.(?:indexOf|lastIndexOf|includes|startsWith|endsWith|replace|replaceAll|split|slice|substring|trim)\s*\(`)
	reJSStrLiteral    = regexp.MustCompile(`(?:"[^"\n]{1,}"|'[^'\n]{1,}')`)
	reJSPassThrough   = regexp.MustCompile(`^\s*return\s+([a-zA-Z_$][a-zA-Z0-9_$.]*)\s*\(([^()]*)\)\s*;?\s*$`)
	reJSIdentToken    = regexp.MustCompile(`\b([a-zA-Z_$][a-zA-Z0-9_$]*)\b`)
	reJSCallToken     = regexp.MustCompile(`\b([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(`)
	reTplDefHeader    = regexp.MustCompile(`^(?:\{\s*def\s+([a-zA-Z0-9_]+)(.*?)\}|@component\s+([a-zA-Z0-9_]+)(.*?))\s*$`)
	reCSSRuleBlock    = regexp.MustCompile(`(?s)([^{}/]+)\{([^{}]*)\}`)
)

var archStringFuncs = map[string]bool{
	"Contains":      true,
	"ContainsAny":   true,
	"ContainsRune":  true,
	"HasPrefix":     true,
	"HasSuffix":     true,
	"Replace":       true,
	"ReplaceAll":    true,
	"Split":         true,
	"SplitN":        true,
	"SplitAfter":    true,
	"Fields":        true,
	"Trim":          true,
	"TrimSpace":     true,
	"TrimPrefix":    true,
	"TrimSuffix":    true,
	"TrimLeft":      true,
	"TrimRight":     true,
	"Index":         true,
	"IndexByte":     true,
	"IndexAny":      true,
	"LastIndex":     true,
	"LastIndexByte": true,
	"Cut":           true,
	"CutPrefix":     true,
	"CutSuffix":     true,
}

var stdInterfaceMethods = map[string]bool{
	"Error":     true,
	"String":    true,
	"ServeHTTP": true,
	"Handler":   true,
	"Read":      true,
	"Write":     true,
	"Close":     true,
	"Len":       true,
	"Less":      true,
	"Swap":      true,
}

type jsFuncInfo struct {
	name      string
	params    []string
	file      string
	startLine int
	body      string
}

// RunArch executes the deterministic architectural smell audit across Go, JS, TPL, and TSS/CSS files.
// Findings are scored and sorted descending from most suspicious to least.
func RunArch(rootDir string) ([]Finding, error) {
	idx, err := loadWorkspaceIndex(rootDir)
	if err != nil {
		return nil, err
	}

	jsFuncs, combinedJS := collectArchJSFuncs(idx)

	var findings []Finding
	findings = append(findings, analyzeArchStringHacks(idx, jsFuncs)...)
	findings = append(findings, analyzeArchRavioli(idx, jsFuncs, combinedJS)...)
	findings = append(findings, analyzeArchInterfaces(idx)...)
	findings = append(findings, analyzeArchStructCohesion(idx)...)
	findings = append(findings, analyzeArchSignatures(idx, jsFuncs)...)
	findings = append(findings, analyzeArchMonolithAndCSS(idx)...)

	for i := range findings {
		normalizeFinding(&findings[i])
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Score != findings[j].Score {
			return findings[i].Score > findings[j].Score
		}
		if findings[i].Category != findings[j].Category {
			return findings[i].Category < findings[j].Category
		}
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Symbol < findings[j].Symbol
	})

	return findings, nil
}

func isGeneratedOrWindowFile(relPath string) bool {
	return strings.HasPrefix(relPath, "internal/gui/window_") ||
		strings.HasPrefix(relPath, "internal/osbridge/window_") ||
		strings.HasPrefix(relPath, "internal/audit/")
}

func archSeverity(score int) string {
	if score >= 80 {
		return "ERROR"
	}
	if score >= 45 {
		return "WARNING"
	}
	return "INFO"
}

var reJSFuncDeclArch = regexp.MustCompile(`(?m)^\s*(?:async\s+)?function\s+([a-zA-Z_$][a-zA-Z0-9_$]*)\s*\(([^)]*)\)\s*\{`)

func collectArchJSFuncs(idx *workspaceIndex) ([]jsFuncInfo, string) {
	var jsFuncs []jsFuncInfo
	var allJS strings.Builder
	for _, jf := range idx.jsFiles {
		allJS.WriteString(jf.src)
		allJS.WriteByte('\n')
		for _, loc := range reJSFuncDeclArch.FindAllStringSubmatchIndex(jf.src, -1) {
			name := jf.src[loc[2]:loc[3]]
			rawParams := jf.src[loc[4]:loc[5]]
			startLine := 1 + strings.Count(jf.src[:loc[0]], "\n")
			openBraceIdx := loc[1] - 1
			closeBraceIdx := findMatchingJSBrace(jf.src, openBraceIdx)
			body := ""
			if closeBraceIdx > openBraceIdx {
				body = jf.src[openBraceIdx+1 : closeBraceIdx]
			}
			var params []string
			for _, p := range strings.Split(rawParams, ",") {
				p = strings.TrimSpace(p)
				if eq := strings.Index(p, "="); eq > 0 {
					p = strings.TrimSpace(p[:eq])
				}
				if p != "" {
					params = append(params, p)
				}
			}
			jsFuncs = append(jsFuncs, jsFuncInfo{
				name:      name,
				params:    params,
				file:      jf.relPath,
				startLine: startLine,
				body:      body,
			})
		}
	}
	return jsFuncs, allJS.String()
}

func findMatchingJSBrace(s string, openIdx int) int {
	if openIdx < 0 || openIdx >= len(s) || s[openIdx] != '{' {
		return -1
	}
	depth := 0
	inStr := byte(0)
	escaped := false
	for i := openIdx; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inStr != 0 {
			if ch == '\\' {
				escaped = true
			} else if ch == inStr {
				inStr = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			inStr = ch
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// 1. ARCH_STRING_HACK (Go & JS)
// ---------------------------------------------------------------------------

func analyzeArchStringHacks(idx *workspaceIndex, jsFuncs []jsFuncInfo) []Finding {
	var findings []Finding

	for _, gf := range idx.goFiles {
		if gf.isTest || isGeneratedOrWindowFile(gf.relPath) {
			continue
		}
		for _, d := range gf.file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name == nil {
				continue
			}

			excludedLits := make(map[*ast.BasicLit]bool)
			stringsCalls := 0
			stringSwitches := 0
			inlineLiterals := 0

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.KeyValueExpr:
					if lit, ok := x.Key.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						excludedLits[lit] = true
					}
				case *ast.CallExpr:
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
						if pkg, ok := sel.X.(*ast.Ident); ok {
							switch pkg.Name {
							case "strings":
								if archStringFuncs[sel.Sel.Name] {
									stringsCalls++
								}
							case "fmt", "log", "errors":
								for _, arg := range x.Args {
									if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
										excludedLits[lit] = true
									}
								}
							}
						}
					}
				case *ast.SwitchStmt:
					hasStrCase := false
					for _, stmt := range x.Body.List {
						cc, ok := stmt.(*ast.CaseClause)
						if !ok {
							continue
						}
						for _, expr := range cc.List {
							if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
								hasStrCase = true
							}
						}
					}
					if hasStrCase {
						stringSwitches++
					}
				case *ast.BasicLit:
					if x.Kind == token.STRING && !excludedLits[x] {
						val, err := strconv.Unquote(x.Value)
						if err == nil && len(strings.TrimSpace(val)) > 0 {
							inlineLiterals++
						}
					}
				}
				return true
			})

			if stringsCalls >= 6 || (inlineLiterals >= 15 && stringsCalls >= 3) || (stringsCalls >= 4 && stringSwitches >= 1 && inlineLiterals >= 10) {
				score := stringsCalls*8 + inlineLiterals*2 + stringSwitches*10
				sym := funcSymbolName(fn)
				pos := idx.fset.Position(fn.Pos())
				findings = append(findings, Finding{
					Category: "ARCH_STRING_HACK",
					Severity: archSeverity(score),
					File:     gf.relPath,
					Line:     pos.Line,
					FuncName: sym,
					Symbol:   sym,
					Message:  fmt.Sprintf("Ad-hoc string manipulation in %s (%d strings.* calls, %d inline string literals, %d string switches); prefer typed tokens or table-driven dispatch", sym, stringsCalls, inlineLiterals, stringSwitches),
					Score:    score,
				})
			}
		}
	}

	for _, fn := range jsFuncs {
		strCalls := len(reJSStrMethodCall.FindAllStringIndex(fn.body, -1))
		strLits := len(reJSStrLiteral.FindAllStringIndex(fn.body, -1))
		if strCalls >= 6 || strLits >= 12 {
			score := strCalls*8 + strLits*3
			findings = append(findings, Finding{
				Category: "ARCH_STRING_HACK",
				Severity: archSeverity(score),
				File:     fn.file,
				Line:     fn.startLine,
				FuncName: fn.name,
				Symbol:   fn.name,
				Message:  fmt.Sprintf("Dense JS string hacking in %s() (%d string method calls, %d string literals)", fn.name, strCalls, strLits),
				Score:    score,
			})
		}
	}

	return findings
}

func funcSymbolName(fn *ast.FuncDecl) string {
	sym := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		if rt := bareExprTypeName(fn.Recv.List[0].Type); rt != "" {
			sym = rt + "." + sym
		}
	}
	return sym
}

func bareExprTypeName(expr ast.Expr) string {
	for expr != nil {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
		case *ast.Ident:
			return t.Name
		case *ast.IndexExpr:
			expr = t.X
		default:
			return ""
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// 2. ARCH_RAVIOLI_INDIRECTION (Go, JS & TPL)
// ---------------------------------------------------------------------------

type archGoFuncMeta struct {
	name      string
	recvType  string
	sym       string
	file      string
	line      int
	loc       int
	exported  bool
	selfCalls int
	fn        *ast.FuncDecl
}

func analyzeArchRavioli(idx *workspaceIndex, jsFuncs []jsFuncInfo, combinedJS string) []Finding {
	var findings []Finding
	var funcs []archGoFuncMeta

	directCalls := make(map[string]int)
	valueRefs := make(map[string]int)
	goDeferRefs := make(map[string]int)

	for _, gf := range idx.goFiles {
		f := gf.file
		if !gf.isTest && !isGeneratedOrWindowFile(gf.relPath) {
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Name == nil || fn.Body == nil {
					continue
				}
				name := fn.Name.Name
				if name == "init" || name == "main" || stdInterfaceMethods[name] {
					continue
				}
				startLine := idx.fset.Position(fn.Pos()).Line
				endLine := idx.fset.Position(fn.End()).Line
				loc := endLine - startLine + 1
				if loc < 1 {
					loc = 1
				}
				recv := ""
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					recv = bareExprTypeName(fn.Recv.List[0].Type)
				}
				selfCalls := countDirectSelfCalls(fn.Body, name)
				funcs = append(funcs, archGoFuncMeta{
					name:      name,
					recvType:  recv,
					sym:       funcSymbolName(fn),
					file:      gf.relPath,
					line:      startLine,
					loc:       loc,
					exported:  ast.IsExported(name),
					selfCalls: selfCalls,
					fn:        fn,
				})
			}
		}

		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.GoStmt:
				if name := calledFuncIdent(x.Call); name != "" {
					goDeferRefs[name]++
				}
			case *ast.DeferStmt:
				if name := calledFuncIdent(x.Call); name != "" {
					goDeferRefs[name]++
				}
			case *ast.CallExpr:
				if name := calledFuncIdent(x); name != "" {
					directCalls[name]++
				}
				for _, arg := range x.Args {
					switch a := arg.(type) {
					case *ast.Ident:
						valueRefs[a.Name]++
					case *ast.SelectorExpr:
						valueRefs[a.Sel.Name]++
					}
				}
			case *ast.KeyValueExpr:
				switch v := x.Value.(type) {
				case *ast.Ident:
					valueRefs[v.Name]++
				case *ast.SelectorExpr:
					valueRefs[v.Sel.Name]++
				}
			case *ast.AssignStmt:
				for _, rhs := range x.Rhs {
					switch r := rhs.(type) {
					case *ast.Ident:
						valueRefs[r.Name]++
					case *ast.SelectorExpr:
						valueRefs[r.Sel.Name]++
					}
				}
			}
			return true
		})
	}

	for _, m := range funcs {
		extCalls := directCalls[m.name] - m.selfCalls
		isPassThrough, targetCall := isTrivialPassThroughGo(m.fn)
		isSingleCallerMicro := !m.exported && m.loc <= 12 && extCalls == 1 && m.selfCalls == 0 && valueRefs[m.name] == 0 && goDeferRefs[m.name] == 0

		if isPassThrough && isSingleCallerMicro {
			score := 42
			findings = append(findings, Finding{
				Category: "ARCH_RAVIOLI_INDIRECTION",
				Severity: archSeverity(score),
				File:     m.file,
				Line:     m.line,
				FuncName: m.sym,
				Symbol:   m.sym,
				Message:  fmt.Sprintf("Single-caller pass-through micro-function %s (%d LOC, fanIn=1) simply delegates to %s(); inline into caller", m.sym, m.loc, targetCall),
				Score:    score,
			})
		} else if isPassThrough {
			score := 34
			findings = append(findings, Finding{
				Category: "ARCH_RAVIOLI_INDIRECTION",
				Severity: archSeverity(score),
				File:     m.file,
				Line:     m.line,
				FuncName: m.sym,
				Symbol:   m.sym,
				Message:  fmt.Sprintf("Trivial 1-statement pass-through wrapper %s (%d LOC) directly delegates to %s() without transforming data", m.sym, m.loc, targetCall),
				Score:    score,
			})
		} else if isSingleCallerMicro {
			score := 24 + (13-m.loc)*2
			findings = append(findings, Finding{
				Category: "ARCH_RAVIOLI_INDIRECTION",
				Severity: archSeverity(score),
				File:     m.file,
				Line:     m.line,
				FuncName: m.sym,
				Symbol:   m.sym,
				Message:  fmt.Sprintf("Unexported micro-function %s (%d LOC, fanIn=1) is called from only 1 site; consider inlining for linear flow", m.sym, m.loc),
				Score:    score,
			})
		}
	}

	jsIdentCounts := make(map[string]int)
	for _, m := range reJSIdentToken.FindAllStringSubmatch(combinedJS, -1) {
		jsIdentCounts[m[1]]++
	}
	jsCallCounts := make(map[string]int)
	for _, m := range reJSCallToken.FindAllStringSubmatch(combinedJS, -1) {
		jsCallCounts[m[1]]++
	}

	for _, fn := range jsFuncs {
		loc := 1 + strings.Count(strings.TrimSpace(fn.body), "\n")
		callCount := jsCallCounts[fn.name] - 1
		allRefCount := jsIdentCounts[fn.name] - 1

		trimmedBody := strings.TrimSpace(fn.body)
		ptMatch := reJSPassThrough.FindStringSubmatch(trimmedBody)
		isJSPT := len(ptMatch) >= 2 && !strings.Contains(trimmedBody, "\n")

		if isJSPT {
			score := 32
			findings = append(findings, Finding{
				Category: "ARCH_RAVIOLI_INDIRECTION",
				Severity: archSeverity(score),
				File:     fn.file,
				Line:     fn.startLine,
				FuncName: fn.name,
				Symbol:   fn.name,
				Message:  fmt.Sprintf("JS pass-through function %s() directly delegates to %s() in 1 statement", fn.name, ptMatch[1]),
				Score:    score,
			})
		} else if loc <= 10 && callCount == 1 && allRefCount == 1 {
			score := 26
			findings = append(findings, Finding{
				Category: "ARCH_RAVIOLI_INDIRECTION",
				Severity: archSeverity(score),
				File:     fn.file,
				Line:     fn.startLine,
				FuncName: fn.name,
				Symbol:   fn.name,
				Message:  fmt.Sprintf("JS micro-helper %s() (%d lines, fanIn=1) is called once; inline at call site", fn.name, loc),
				Score:    score,
			})
		}
	}

	findings = append(findings, analyzeArchDeepEmbedding(idx)...)
	findings = append(findings, analyzeArchTplMicroDefs(idx)...)

	return findings
}

func countDirectSelfCalls(body *ast.BlockStmt, fnName string) int {
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		if ce, ok := n.(*ast.CallExpr); ok {
			if calledFuncIdent(ce) == fnName {
				count++
			}
		}
		return true
	})
	return count
}

func calledFuncIdent(ce *ast.CallExpr) string {
	if ce == nil {
		return ""
	}
	switch fun := ce.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

func isTrivialPassThroughGo(fn *ast.FuncDecl) (bool, string) {
	if fn == nil || fn.Body == nil || len(fn.Body.List) != 1 {
		return false, ""
	}
	var call *ast.CallExpr
	switch stmt := fn.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(stmt.Results) == 1 {
			call, _ = stmt.Results[0].(*ast.CallExpr)
		}
	case *ast.ExprStmt:
		call, _ = stmt.X.(*ast.CallExpr)
	}
	if call == nil || len(call.Args) == 0 {
		return false, ""
	}
	target := calledFuncIdent(call)
	if target == "" {
		return false, ""
	}
	for _, arg := range call.Args {
		switch a := arg.(type) {
		case *ast.Ident:
			if a.Name == "true" || a.Name == "false" || a.Name == "nil" {
				return false, ""
			}
		case *ast.SelectorExpr:
			if _, ok := a.X.(*ast.Ident); !ok {
				return false, ""
			}
		default:
			return false, ""
		}
	}
	return true, target
}

type embedTypeNode struct {
	name     string
	file     string
	line     int
	embedded []string
}

func analyzeArchDeepEmbedding(idx *workspaceIndex) []Finding {
	var findings []Finding
	nodes := make(map[string]embedTypeNode)

	for _, gf := range idx.goFiles {
		if gf.isTest {
			continue
		}
		for _, d := range gf.file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil {
					continue
				}
				var emb []string
				switch t := ts.Type.(type) {
				case *ast.StructType:
					if t.Fields != nil {
						for _, f := range t.Fields.List {
							if len(f.Names) == 0 {
								if en := bareExprTypeName(f.Type); en != "" {
									emb = append(emb, en)
								}
							}
						}
					}
				case *ast.InterfaceType:
					if t.Methods != nil {
						for _, m := range t.Methods.List {
							if len(m.Names) == 0 {
								if en := bareExprTypeName(m.Type); en != "" {
									emb = append(emb, en)
								}
							}
						}
					}
				}
				if len(emb) > 0 {
					nodes[ts.Name.Name] = embedTypeNode{
						name:     ts.Name.Name,
						file:     gf.relPath,
						line:     idx.fset.Position(ts.Pos()).Line,
						embedded: emb,
					}
				}
			}
		}
	}

	var depthOf func(name string, seen map[string]bool) int
	depthOf = func(name string, seen map[string]bool) int {
		n, ok := nodes[name]
		if !ok || seen[name] {
			return 0
		}
		seen[name] = true
		maxSub := 0
		for _, child := range n.embedded {
			d := 1 + depthOf(child, seen)
			if d > maxSub {
				maxSub = d
			}
		}
		delete(seen, name)
		return maxSub
	}

	for _, n := range nodes {
		d := depthOf(n.name, make(map[string]bool))
		if d > 2 {
			score := d * 18
			findings = append(findings, Finding{
				Category: "ARCH_RAVIOLI_INDIRECTION",
				Severity: archSeverity(score),
				File:     n.file,
				Line:     n.line,
				FuncName: n.name,
				Symbol:   n.name,
				Message:  fmt.Sprintf("Type %s has deep embedding chain depth of %d (> 2); flatten nested composition", n.name, d),
				Score:    score,
			})
		}
	}

	return findings
}

type tplDefMeta struct {
	name      string
	file      string
	line      int
	lineCount int
	numParams int
}

var reTplComponentUseArch = regexp.MustCompile(`@([a-zA-Z0-9_]+)`)

func collectTplDefs(idx *workspaceIndex) ([]tplDefMeta, map[string]int) {
	var defs []tplDefMeta
	uses := make(map[string]int)

	for _, tf := range idx.tplFiles {
		for _, m := range reTplComponentUseArch.FindAllStringSubmatch(tf.src, -1) {
			if m[1] != "component" && m[1] != "slot" && m[1] != "include" {
				uses[m[1]]++
			}
		}
		for i := 0; i < len(tf.lines); i++ {
			raw := tf.lines[i]
			trimmed := strings.TrimSpace(raw)
			m := reTplDefHeader.FindStringSubmatch(trimmed)
			if len(m) == 0 {
				continue
			}
			defName := m[1]
			paramStr := strings.TrimSpace(m[2])
			if defName == "" {
				defName = m[3]
				paramStr = strings.TrimSpace(m[4])
			}
			numParams := 0
			if paramStr != "" {
				for _, tok := range strings.Fields(paramStr) {
					if tok != "" {
						numParams++
					}
				}
			}
			baseIndent := len(raw) - len(strings.TrimLeft(raw, " \t"))
			end := i + 1
			nonEmpty := 1
			for end < len(tf.lines) {
				tLine := strings.TrimSpace(tf.lines[end])
				if tLine == "" || strings.HasPrefix(tLine, "//") {
					end++
					continue
				}
				ind := len(tf.lines[end]) - len(strings.TrimLeft(tf.lines[end], " \t"))
				if ind <= baseIndent {
					break
				}
				nonEmpty++
				end++
			}
			defs = append(defs, tplDefMeta{
				name:      defName,
				file:      tf.relPath,
				line:      i + 1,
				lineCount: nonEmpty,
				numParams: numParams,
			})
		}
	}
	return defs, uses
}

func analyzeArchTplMicroDefs(idx *workspaceIndex) []Finding {
	var findings []Finding
	defs, uses := collectTplDefs(idx)
	for _, d := range defs {
		count := uses[d.name]
		if d.lineCount <= 6 && count <= 1 {
			score := 24
			sym := "{def " + d.name + "}"
			findings = append(findings, Finding{
				Category: "ARCH_RAVIOLI_INDIRECTION",
				Severity: archSeverity(score),
				File:     d.file,
				Line:     d.line,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Template micro-def %s (%d lines, invoked %dx) adds indirection for a single call site; inline directly", sym, d.lineCount, count),
				Score:    score,
			})
		}
	}
	return findings
}

// ---------------------------------------------------------------------------
// 3. ARCH_SINGLE_IMPL_IFACE (Go)
// ---------------------------------------------------------------------------

type archInterfaceMeta struct {
	name     string
	file     string
	line     int
	methods  []string
	embedded []string
}

func analyzeArchInterfaces(idx *workspaceIndex) []Finding {
	var findings []Finding
	var ifaces []archInterfaceMeta
	ifaceMap := make(map[string]archInterfaceMeta)
	concreteTypes := make(map[string]bool)
	typeMethods := make(map[string]map[string]bool)
	structEmbedded := make(map[string][]string)

	for _, gf := range idx.goFiles {
		if gf.isTest {
			continue
		}
		for _, d := range gf.file.Decls {
			switch decl := d.(type) {
			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				}
				for _, spec := range decl.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.Name == nil {
						continue
					}
					tName := ts.Name.Name
					switch st := ts.Type.(type) {
					case *ast.InterfaceType:
						if st.Methods == nil || len(st.Methods.List) == 0 {
							continue
						}
						var methods []string
						var emb []string
						for _, m := range st.Methods.List {
							if len(m.Names) > 0 {
								for _, id := range m.Names {
									methods = append(methods, id.Name)
								}
							} else if en := bareExprTypeName(m.Type); en != "" {
								emb = append(emb, en)
							}
						}
						meta := archInterfaceMeta{
							name:     tName,
							file:     gf.relPath,
							line:     idx.fset.Position(ts.Pos()).Line,
							methods:  methods,
							embedded: emb,
						}
						ifaces = append(ifaces, meta)
						ifaceMap[tName] = meta
					case *ast.StructType:
						concreteTypes[tName] = true
						if st.Fields != nil {
							for _, f := range st.Fields.List {
								if len(f.Names) == 0 {
									if en := bareExprTypeName(f.Type); en != "" {
										structEmbedded[tName] = append(structEmbedded[tName], en)
									}
								}
							}
						}
					default:
						concreteTypes[tName] = true
					}
				}
			case *ast.FuncDecl:
				if decl.Recv != nil && len(decl.Recv.List) > 0 && decl.Name != nil {
					rt := bareExprTypeName(decl.Recv.List[0].Type)
					if rt != "" {
						concreteTypes[rt] = true
						if typeMethods[rt] == nil {
							typeMethods[rt] = make(map[string]bool)
						}
						typeMethods[rt][decl.Name.Name] = true
					}
				}
			}
		}
	}

	var resolveIfaceMethods func(name string, seen map[string]bool) []string
	resolveIfaceMethods = func(name string, seen map[string]bool) []string {
		meta, ok := ifaceMap[name]
		if !ok || seen[name] {
			return nil
		}
		seen[name] = true
		out := append([]string(nil), meta.methods...)
		for _, e := range meta.embedded {
			out = append(out, resolveIfaceMethods(e, seen)...)
		}
		return out
	}

	var hasMethod func(cType, mName string, seen map[string]bool) bool
	hasMethod = func(cType, mName string, seen map[string]bool) bool {
		if seen[cType] {
			return false
		}
		seen[cType] = true
		if typeMethods[cType][mName] {
			return true
		}
		for _, emb := range structEmbedded[cType] {
			if hasMethod(emb, mName, seen) {
				return true
			}
		}
		return false
	}

	for _, iface := range ifaces {
		reqMethods := resolveIfaceMethods(iface.name, make(map[string]bool))
		if len(reqMethods) == 0 {
			continue
		}
		var impls []string
		for cType := range concreteTypes {
			if _, isIface := ifaceMap[cType]; isIface {
				continue
			}
			allMatch := true
			for _, mName := range reqMethods {
				if !hasMethod(cType, mName, make(map[string]bool)) {
					allMatch = false
					break
				}
			}
			if allMatch {
				impls = append(impls, cType)
			}
		}
		sort.Strings(impls)
		if len(impls) == 0 {
			score := 45
			findings = append(findings, Finding{
				Category: "ARCH_SINGLE_IMPL_IFACE",
				Severity: archSeverity(score),
				File:     iface.file,
				Line:     iface.line,
				FuncName: iface.name,
				Symbol:   iface.name,
				Message:  fmt.Sprintf("Interface %s (%d methods) has 0 concrete struct implementations in the repository; remove unused abstraction", iface.name, len(reqMethods)),
				Score:    score,
			})
		} else if len(impls) == 1 {
			score := 35
			findings = append(findings, Finding{
				Category: "ARCH_SINGLE_IMPL_IFACE",
				Severity: archSeverity(score),
				File:     iface.file,
				Line:     iface.line,
				FuncName: iface.name,
				Symbol:   iface.name,
				Message:  fmt.Sprintf("Interface %s (%d methods) has only 1 concrete implementation (%s); prefer concrete struct over premature vtable indirection", iface.name, len(reqMethods), impls[0]),
				Score:    score,
			})
		}
	}

	return findings
}

// ---------------------------------------------------------------------------
// 4. ARCH_LCOM_STRUCT & 5. ARCH_OBJECT_SOUP (Go)
// ---------------------------------------------------------------------------

type archStructInfo struct {
	name     string
	pkgDir   string
	file     string
	line     int
	fields   map[string]bool
	embedded []string
	methods  []archMethodFieldAccess
}

type archMethodFieldAccess struct {
	name   string
	fields map[string]bool
}

func analyzeArchStructCohesion(idx *workspaceIndex) []Finding {
	var findings []Finding
	structs := make(map[string]*archStructInfo)
	var structOrder []string

	for _, gf := range idx.goFiles {
		if gf.isTest || isGeneratedOrWindowFile(gf.relPath) {
			continue
		}
		pkgDir := filepath.Dir(gf.relPath)
		for _, d := range gf.file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := make(map[string]bool)
				var embedded []string
				if st.Fields != nil {
					for _, f := range st.Fields.List {
						if len(f.Names) == 0 {
							if en := bareExprTypeName(f.Type); en != "" {
								fields[en] = true
								embedded = append(embedded, en)
							}
						} else {
							for _, id := range f.Names {
								fields[id.Name] = true
							}
						}
					}
				}
				key := pkgDir + ":" + ts.Name.Name
				if _, exists := structs[key]; !exists {
					structOrder = append(structOrder, key)
					structs[key] = &archStructInfo{
						name:     ts.Name.Name,
						pkgDir:   pkgDir,
						file:     gf.relPath,
						line:     idx.fset.Position(ts.Pos()).Line,
						fields:   fields,
						embedded: embedded,
					}
				}
			}
		}
	}

	var expandEmbeddedFields func(st *archStructInfo, seen map[string]bool)
	expandEmbeddedFields = func(st *archStructInfo, seen map[string]bool) {
		key := st.pkgDir + ":" + st.name
		if seen[key] {
			return
		}
		seen[key] = true
		for _, embName := range st.embedded {
			if embSt, ok := structs[st.pkgDir+":"+embName]; ok {
				expandEmbeddedFields(embSt, seen)
				delete(st.fields, embName)
				for f := range embSt.fields {
					st.fields[f] = true
				}
			}
		}
	}
	for _, key := range structOrder {
		expandEmbeddedFields(structs[key], make(map[string]bool))
	}

	for _, gf := range idx.goFiles {
		if gf.isTest || isGeneratedOrWindowFile(gf.relPath) {
			continue
		}
		pkgDir := filepath.Dir(gf.relPath)
		for _, d := range gf.file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Body == nil || fn.Name == nil {
				continue
			}
			rt := bareExprTypeName(fn.Recv.List[0].Type)
			st, ok := structs[pkgDir+":"+rt]
			if !ok {
				continue
			}
			recvVar := ""
			if len(fn.Recv.List[0].Names) > 0 && fn.Recv.List[0].Names[0] != nil {
				recvVar = fn.Recv.List[0].Names[0].Name
			}
			accessed := make(map[string]bool)
			if recvVar != "" {
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == recvVar {
						if st.fields[sel.Sel.Name] {
							accessed[sel.Sel.Name] = true
						}
					}
					return true
				})
			}
			st.methods = append(st.methods, archMethodFieldAccess{
				name:   fn.Name.Name,
				fields: accessed,
			})
		}
	}

	for _, key := range structOrder {
		st := structs[key]
		if len(st.fields) < 4 || len(st.methods) < 4 {
			continue
		}
		var stateful []archMethodFieldAccess
		for _, m := range st.methods {
			if len(m.fields) > 0 {
				stateful = append(stateful, m)
			}
		}
		if len(stateful) < 4 {
			continue
		}
		lcom4, disjointPct := computeLCOM4AndDisjoint(stateful)
		if lcom4 >= 2 || disjointPct >= 70 {
			score := lcom4*20 + disjointPct/2
			findings = append(findings, Finding{
				Category: "ARCH_LCOM_STRUCT",
				Severity: archSeverity(score),
				File:     st.file,
				Line:     st.line,
				FuncName: st.name,
				Symbol:   st.name,
				Message:  fmt.Sprintf("Low struct cohesion in %s (%d fields, %d methods, LCOM4=%d disconnected components, %d%% disjoint method pairs); split orthogonal state clusters", st.name, len(st.fields), len(st.methods), lcom4, disjointPct),
				Score:    score,
			})
		}
	}

	byFile := make(map[string][]*archStructInfo)
	var fileOrder []string
	for _, key := range structOrder {
		st := structs[key]
		if len(st.fields) <= 3 && len(st.methods) >= 1 && len(st.methods) <= 2 {
			if _, seen := byFile[st.file]; !seen {
				fileOrder = append(fileOrder, st.file)
			}
			byFile[st.file] = append(byFile[st.file], st)
		}
	}
	for _, file := range fileOrder {
		list := byFile[file]
		if len(list) >= 3 {
			var names []string
			for _, s := range list {
				names = append(names, fmt.Sprintf("%s(%df/%dm)", s.name, len(s.fields), len(s.methods)))
			}
			score := len(list) * 15
			sym := filepath.Base(file)
			findings = append(findings, Finding{
				Category: "ARCH_OBJECT_SOUP",
				Severity: archSeverity(score),
				File:     file,
				Line:     list[0].line,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Object soup in %s: %d behavioral micro-structs (%s) fragment logic into tiny objects instead of cohesive structs + procedures", file, len(list), strings.Join(names, ", ")),
				Score:    score,
			})
		}
	}

	return findings
}

func computeLCOM4AndDisjoint(methods []archMethodFieldAccess) (int, int) {
	n := len(methods)
	if n == 0 {
		return 0, 0
	}
	adj := make([][]int, n)
	totalPairs := 0
	disjointPairs := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			totalPairs++
			shared := false
			for f := range methods[i].fields {
				if methods[j].fields[f] {
					shared = true
					break
				}
			}
			if shared {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			} else {
				disjointPairs++
			}
		}
	}
	visited := make([]bool, n)
	components := 0
	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		components++
		queue := []int{i}
		visited[i] = true
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			for _, nb := range adj[curr] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}
	}
	disjointPct := 0
	if totalPairs > 0 {
		disjointPct = (disjointPairs * 100) / totalPairs
	}
	return components, disjointPct
}

// ---------------------------------------------------------------------------
// 6. ARCH_SIGNATURE_BLOAT (Go, JS & TPL)
// ---------------------------------------------------------------------------

type stemVariantGroup struct {
	pkgDir string
	stem   string
	file   string
	line   int
	names  []string
}

func analyzeArchSignatures(idx *workspaceIndex, jsFuncs []jsFuncInfo) []Finding {
	var findings []Finding
	stemGroups := make(map[string]*stemVariantGroup)
	var stemOrder []string
	allPkgFuncs := make(map[string]map[string]bool)

	for _, gf := range idx.goFiles {
		if gf.isTest || isGeneratedOrWindowFile(gf.relPath) {
			continue
		}
		pkgDir := filepath.Dir(gf.relPath)
		if allPkgFuncs[pkgDir] == nil {
			allPkgFuncs[pkgDir] = make(map[string]bool)
		}

		for _, d := range gf.file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name == nil {
				continue
			}
			name := fn.Name.Name
			sym := funcSymbolName(fn)
			pos := idx.fset.Position(fn.Pos())
			allPkgFuncs[pkgDir][name] = true

			numParams, boolParams := countGoParamsAndBools(fn.Type.Params)
			numResults, _ := countGoParamsAndBools(fn.Type.Results)

			if numParams >= 6 || numResults >= 4 || boolParams >= 2 {
				score := numParams*6 + numResults*8 + boolParams*15
				findings = append(findings, Finding{
					Category: "ARCH_SIGNATURE_BLOAT",
					Severity: archSeverity(score),
					File:     gf.relPath,
					Line:     pos.Line,
					FuncName: sym,
					Symbol:   sym,
					Message:  fmt.Sprintf("Bloated signature on %s (%d params, %d returns, %d bool flags); group coupled parameters or eliminate boolean mode traps", sym, numParams, numResults, boolParams),
					Score:    score,
				})
			}

			if len(name) > 38 {
				score := 20 + (len(name)-38)*2
				findings = append(findings, Finding{
					Category: "ARCH_SIGNATURE_BLOAT",
					Severity: archSeverity(score),
					File:     gf.relPath,
					Line:     pos.Line,
					FuncName: sym,
					Symbol:   sym,
					Message:  fmt.Sprintf("Overlong function identifier %s (%d chars > 38); simplify name and move context to types/package", sym, len(name)),
					Score:    score,
				})
			}

			if stem := extractVariantStem(name); stem != "" {
				gKey := pkgDir + ":" + stem
				grp, exists := stemGroups[gKey]
				if !exists {
					stemOrder = append(stemOrder, gKey)
					grp = &stemVariantGroup{
						pkgDir: pkgDir,
						stem:   stem,
						file:   gf.relPath,
						line:   pos.Line,
					}
					stemGroups[gKey] = grp
				}
				grp.names = append(grp.names, name)
			}
		}
	}

	for _, gKey := range stemOrder {
		grp := stemGroups[gKey]
		names := append([]string(nil), grp.names...)
		if allPkgFuncs[grp.pkgDir][grp.stem] {
			names = append([]string{grp.stem}, names...)
		}
		if len(names) >= 3 {
			score := len(names) * 12
			sym := grp.stem + "*"
			findings = append(findings, Finding{
				Category: "ARCH_SIGNATURE_BLOAT",
				Severity: archSeverity(score),
				File:     grp.file,
				Line:     grp.line,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Function variant proliferation (%d variants: %s); consolidate into a single orthogonal procedure", len(names), strings.Join(names, ", ")),
				Score:    score,
			})
		}
	}

	for _, fn := range jsFuncs {
		if len(fn.params) >= 5 {
			score := len(fn.params) * 8
			findings = append(findings, Finding{
				Category: "ARCH_SIGNATURE_BLOAT",
				Severity: archSeverity(score),
				File:     fn.file,
				Line:     fn.startLine,
				FuncName: fn.name,
				Symbol:   fn.name,
				Message:  fmt.Sprintf("JS function %s() takes %d parameters (>= 5); pass a structured options/context value", fn.name, len(fn.params)),
				Score:    score,
			})
		}
		if len(fn.name) > 38 {
			score := 20 + (len(fn.name)-38)*2
			findings = append(findings, Finding{
				Category: "ARCH_SIGNATURE_BLOAT",
				Severity: archSeverity(score),
				File:     fn.file,
				Line:     fn.startLine,
				FuncName: fn.name,
				Symbol:   fn.name,
				Message:  fmt.Sprintf("Overlong JS function name %s() (%d chars > 38)", fn.name, len(fn.name)),
				Score:    score,
			})
		}
	}

	defs, _ := collectTplDefs(idx)
	for _, d := range defs {
		if d.numParams >= 7 {
			score := d.numParams * 6
			sym := "{def " + d.name + "}"
			findings = append(findings, Finding{
				Category: "ARCH_SIGNATURE_BLOAT",
				Severity: archSeverity(score),
				File:     d.file,
				Line:     d.line,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Template %s requires %d positional parameters (>= 7); bind a structured view-model instead", sym, d.numParams),
				Score:    score,
			})
		}
	}

	return findings
}

func countGoParamsAndBools(fl *ast.FieldList) (int, int) {
	if fl == nil {
		return 0, 0
	}
	total := 0
	bools := 0
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		total += n
		if id, ok := f.Type.(*ast.Ident); ok && id.Name == "bool" {
			bools += n
		}
	}
	return total, bools
}

func extractVariantStem(name string) string {
	for _, marker := range []string{"With", "For", "By", "From"} {
		idx := strings.Index(name, marker)
		if idx >= 3 && idx+len(marker) < len(name) {
			nextCh := name[idx+len(marker)]
			if nextCh >= 'A' && nextCh <= 'Z' {
				return name[:idx]
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// 7. ARCH_MONOLITH_AND_NESTING (Go, JS, TPL & TSS/CSS)
// ---------------------------------------------------------------------------

func analyzeArchMonolithAndCSS(idx *workspaceIndex) []Finding {
	var findings []Finding

	for _, gf := range idx.goFiles {
		if gf.isTest || isGeneratedOrWindowFile(gf.relPath) {
			continue
		}
		loc := len(gf.lines)
		if loc > 1000 {
			score := 30 + (loc-1000)/25
			sym := filepath.Base(gf.relPath)
			findings = append(findings, Finding{
				Category: "ARCH_MONOLITH_AND_NESTING",
				Severity: archSeverity(score),
				File:     gf.relPath,
				Line:     1,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Oversized Go file %s (%d LOC > 1000); split by cohesive subsystem responsibility", gf.relPath, loc),
				Score:    score,
			})
		}

		seenFunc := false
		seenVar := false
		prevWasType := false
		for _, decl := range gf.file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				seenFunc = true
				prevWasType = false
			case *ast.GenDecl:
				switch d.Tok {
				case token.CONST:
					if (seenFunc || seenVar) && !(prevWasType && constDeclUsesIota(d)) {
						score := 20
						sym := firstConstSymbol(d)
						line := idx.fset.Position(d.Pos()).Line
						findings = append(findings, Finding{
							Category: "ARCH_MONOLITH_AND_NESTING",
							Severity: "INFO",
							File:     gf.relPath,
							Line:     line,
							FuncName: sym,
							Symbol:   sym,
							Message:  fmt.Sprintf("Mid-file const declaration %q in %s:%d; move file-level constants to the top of the file right after imports", sym, gf.relPath, line),
							Score:    score,
						})
					}
					prevWasType = false
				case token.VAR:
					seenVar = true
					prevWasType = false
				case token.TYPE:
					prevWasType = true
				default:
					prevWasType = false
				}
			default:
				prevWasType = false
			}
		}
	}

	for _, jf := range idx.jsFiles {
		loc := len(jf.lines)
		if loc > 600 {
			score := 30 + (loc-600)/20
			sym := filepath.Base(jf.relPath)
			findings = append(findings, Finding{
				Category: "ARCH_MONOLITH_AND_NESTING",
				Severity: archSeverity(score),
				File:     jf.relPath,
				Line:     1,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Oversized JS file %s (%d LOC > 600)", jf.relPath, loc),
				Score:    score,
			})
		}
	}

	for _, tf := range idx.tplFiles {
		loc := len(tf.lines)
		if loc > 400 {
			score := 25 + (loc-400)/15
			sym := filepath.Base(tf.relPath)
			findings = append(findings, Finding{
				Category: "ARCH_MONOLITH_AND_NESTING",
				Severity: archSeverity(score),
				File:     tf.relPath,
				Line:     1,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Oversized template file %s (%d LOC > 400); extract focused sub-templates or components", tf.relPath, loc),
				Score:    score,
			})
		}

		maxDepth := 0
		maxDepthLine := 1
		for i, raw := range tf.lines {
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			indent := countIndentSpaces(raw)
			depth := indent / 2
			if depth > maxDepth {
				maxDepth = depth
				maxDepthLine = i + 1
			}
		}
		if maxDepth > 8 {
			score := (maxDepth-8)*12 + 20
			sym := fmt.Sprintf("depth:%d", maxDepth)
			findings = append(findings, Finding{
				Category: "ARCH_MONOLITH_AND_NESTING",
				Severity: archSeverity(score),
				File:     tf.relPath,
				Line:     maxDepthLine,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Deep template DOM nesting in %s reaches %d levels (> 8) at line %d; flatten layout hierarchy", tf.relPath, maxDepth, maxDepthLine),
				Score:    score,
			})
		}
	}

	findings = append(findings, analyzeArchCSSAndTSS(idx)...)
	return findings
}

func countIndentSpaces(raw string) int {
	n := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] == ' ' {
			n++
		} else if raw[i] == '\t' {
			n += 2
		} else {
			break
		}
	}
	return n
}

func constDeclUsesIota(gd *ast.GenDecl) bool {
	found := false
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, v := range vs.Values {
			ast.Inspect(v, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == "iota" {
					found = true
					return false
				}
				return true
			})
			if found {
				return true
			}
		}
	}
	return false
}

func firstConstSymbol(gd *ast.GenDecl) string {
	for _, spec := range gd.Specs {
		if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Names) > 0 && vs.Names[0] != nil {
			return vs.Names[0].Name
		}
	}
	return "const"
}

type cssBlockOccurrence struct {
	file     string
	line     int
	selector string
}

func analyzeArchCSSAndTSS(idx *workspaceIndex) []Finding {
	var findings []Finding
	dupBlocks := make(map[string][]cssBlockOccurrence)
	dupDeclCounts := make(map[string]int)
	var dupOrder []string

	for _, cf := range idx.cssFiles {
		loc := len(cf.lines)
		if loc > 600 {
			score := 25 + (loc-600)/20
			sym := filepath.Base(cf.relPath)
			findings = append(findings, Finding{
				Category: "ARCH_MONOLITH_AND_NESTING",
				Severity: archSeverity(score),
				File:     cf.relPath,
				Line:     1,
				FuncName: sym,
				Symbol:   sym,
				Message:  fmt.Sprintf("Oversized stylesheet %s (%d LOC > 600)", cf.relPath, loc),
				Score:    score,
			})
		}

		importantCount := 0
		firstImportantLine := 1
		for i, line := range cf.lines {
			if strings.Contains(line, "!important") {
				if importantCount == 0 {
					firstImportantLine = i + 1
				}
				importantCount++
			}
		}
		if importantCount >= 5 {
			score := importantCount * 6
			findings = append(findings, Finding{
				Category: "ARCH_MONOLITH_AND_NESTING",
				Severity: archSeverity(score),
				File:     cf.relPath,
				Line:     firstImportantLine,
				FuncName: "!important",
				Symbol:   "!important",
				Message:  fmt.Sprintf("High !important override density in %s (%d occurrences >= 5); fix selector specificity hierarchy", cf.relPath, importantCount),
				Score:    score,
			})
		}

		rules := parseStylesheetRules(cf)
		for _, rule := range rules {
			if strings.HasPrefix(rule.selector, "@") || strings.Contains(rule.selector, "@") {
				continue
			}
			for _, selPart := range strings.Split(rule.selector, ",") {
				selPart = strings.TrimSpace(selPart)
				levels := countCSSCompoundLevels(selPart)
				if levels >= 4 {
					score := 20 + (levels-3)*6
					sym := truncateStr(selPart, 40)
					findings = append(findings, Finding{
						Category: "ARCH_MONOLITH_AND_NESTING",
						Severity: archSeverity(score),
						File:     cf.relPath,
						Line:     rule.line,
						FuncName: sym,
						Symbol:   sym,
						Message:  fmt.Sprintf("Over-specific selector %q has %d compound levels (>= 4); prefer flat utility/component classes", selPart, levels),
						Score:    score,
					})
				}
			}

			normKey, declCount := normalizeRuleDeclarations(rule.decls)
			if declCount >= 4 && normKey != "" {
				if _, exists := dupBlocks[normKey]; !exists {
					dupOrder = append(dupOrder, normKey)
					dupDeclCounts[normKey] = declCount
				}
				dupBlocks[normKey] = append(dupBlocks[normKey], cssBlockOccurrence{
					file:     cf.relPath,
					line:     rule.line,
					selector: truncateStr(strings.Join(strings.Fields(rule.selector), " "), 36),
				})
			}
		}
	}

	for _, key := range dupOrder {
		occs := dupBlocks[key]
		if len(occs) >= 3 {
			declCount := dupDeclCounts[key]
			score := declCount * len(occs) * 4
			first := occs[0]
			var others []string
			for _, o := range occs[1:] {
				others = append(others, fmt.Sprintf("%s (%s:%d)", o.selector, o.file, o.line))
			}
			findings = append(findings, Finding{
				Category: "ARCH_MONOLITH_AND_NESTING",
				Severity: archSeverity(score),
				File:     first.file,
				Line:     first.line,
				FuncName: first.selector,
				Symbol:   first.selector,
				Message:  fmt.Sprintf("Duplicate CSS/TSS property block (%d identical declarations) repeated across %d selectors (also in %s)", declCount, len(occs), strings.Join(others, ", ")),
				Score:    score,
			})
		}
	}

	return findings
}

type parsedStyleRule struct {
	selector string
	line     int
	decls    []string
}

func parseStylesheetRules(cf parsedTextFile) []parsedStyleRule {
	if strings.Contains(cf.src, "{") {
		var out []parsedStyleRule
		for _, locIdx := range reCSSRuleBlock.FindAllStringSubmatchIndex(cf.src, -1) {
			rawSel := strings.TrimSpace(cf.src[locIdx[2]:locIdx[3]])
			rawBody := cf.src[locIdx[4]:locIdx[5]]
			ruleLine := 1 + strings.Count(cf.src[:locIdx[2]], "\n")
			var decls []string
			for _, part := range strings.Split(rawBody, ";") {
				line := strings.TrimSpace(part)
				if idx := strings.Index(line, "/*"); idx >= 0 {
					line = strings.TrimSpace(line[:idx])
				}
				if line != "" {
					decls = append(decls, line)
				}
			}
			out = append(out, parsedStyleRule{
				selector: rawSel,
				line:     ruleLine,
				decls:    decls,
			})
		}
		return out
	}

	// Indentation-based .tss format
	var out []parsedStyleRule
	for i := 0; i < len(cf.lines); i++ {
		raw := cf.lines[i]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		indent := countIndentSpaces(raw)
		// Check if next non-empty line is indented further (making this line a selector)
		nextIdx := i + 1
		for nextIdx < len(cf.lines) {
			nt := strings.TrimSpace(cf.lines[nextIdx])
			if nt != "" && !strings.HasPrefix(nt, "//") && !strings.HasPrefix(nt, "/*") {
				break
			}
			nextIdx++
		}
		if nextIdx >= len(cf.lines) || countIndentSpaces(cf.lines[nextIdx]) <= indent {
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			continue
		}
		rule := parsedStyleRule{
			selector: trimmed,
			line:     i + 1,
		}
		for j := nextIdx; j < len(cf.lines); j++ {
			jt := strings.TrimSpace(cf.lines[j])
			if jt == "" || strings.HasPrefix(jt, "//") || strings.HasPrefix(jt, "/*") {
				continue
			}
			jInd := countIndentSpaces(cf.lines[j])
			if jInd <= indent {
				break
			}
			rule.decls = append(rule.decls, jt)
		}
		out = append(out, rule)
	}
	return out
}

func countCSSCompoundLevels(selector string) int {
	cleaned := strings.NewReplacer(">", " ", "+", " ", "~", " ").Replace(selector)
	parts := strings.Fields(cleaned)
	count := 0
	for _, p := range parts {
		if p != "" {
			count++
		}
	}
	return count
}

func normalizeRuleDeclarations(rawDecls []string) (string, int) {
	var decls []string
	for _, raw := range rawDecls {
		line := strings.TrimSpace(raw)
		if idx := strings.Index(line, "/*"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}
		var k, v string
		if strings.Contains(line, ":") {
			kv := strings.SplitN(line, ":", 2)
			k = strings.ToLower(strings.TrimSpace(kv[0]))
			v = strings.Join(strings.Fields(strings.TrimSpace(kv[1])), " ")
		} else {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				k = strings.ToLower(fields[0])
				v = strings.Join(fields[1:], " ")
			}
		}
		if k != "" && v != "" {
			decls = append(decls, k+":"+v)
		}
	}
	if len(decls) < 4 {
		return "", len(decls)
	}
	sort.Strings(decls)
	return strings.Join(decls, ";"), len(decls)
}

// ---------------------------------------------------------------------------
// Architecture Report Formatting & Top 10 Leaderboard
// ---------------------------------------------------------------------------

// PrintArchReportWithLang writes the scored architectural audit summary,
// Top 10 Most Suspicious Files leaderboard, and ranked findings to w.
func PrintArchReportWithLang(w io.Writer, findings []Finding, filterCategory, filterLang, filterSeverity string) int {
	catCounts := make(map[string]int)
	catPoints := make(map[string]int)
	sevCounts := make(map[string]int)

	type fileScoreEntry struct {
		file  string
		score int
		count int
	}
	fileScores := make(map[string]*fileScoreEntry)

	var filtered []Finding
	for _, f := range findings {
		normalizeFinding(&f)
		if !MatchesFilters(f, filterCategory, filterLang, filterSeverity) {
			continue
		}
		catCounts[f.Category]++
		catPoints[f.Category] += f.Score
		sevCounts[f.Severity]++
		fe, ok := fileScores[f.File]
		if !ok {
			fe = &fileScoreEntry{file: f.File}
			fileScores[f.File] = fe
		}
		fe.score += f.Score
		fe.count++
		filtered = append(filtered, f)
	}

	var leaderboard []*fileScoreEntry
	for _, fe := range fileScores {
		leaderboard = append(leaderboard, fe)
	}
	sort.Slice(leaderboard, func(i, j int) bool {
		if leaderboard[i].score != leaderboard[j].score {
			return leaderboard[i].score > leaderboard[j].score
		}
		if leaderboard[i].count != leaderboard[j].count {
			return leaderboard[i].count > leaderboard[j].count
		}
		return leaderboard[i].file < leaderboard[j].file
	})

	fmt.Fprintln(w, "================================================================================")
	if filterLang != "" || filterSeverity != "" || filterCategory != "" {
		fmt.Fprintf(w, "  MOJRA ARCHITECTURE SMELL AUDIT REPORT (lang=%s severity=%s category=%s)\n", filterLang, filterSeverity, filterCategory)
	} else {
		fmt.Fprintln(w, "  MOJRA ARCHITECTURE SMELL AUDIT REPORT")
	}
	fmt.Fprintf(w, "  SEVERITY: ERROR=%d | WARNING=%d | INFO=%d\n", sevCounts["ERROR"], sevCounts["WARNING"], sevCounts["INFO"])
	fmt.Fprintln(w, "================================================================================")

	categories := []string{
		"ARCH_STRING_HACK",
		"ARCH_RAVIOLI_INDIRECTION",
		"ARCH_SINGLE_IMPL_IFACE",
		"ARCH_LCOM_STRUCT",
		"ARCH_OBJECT_SOUP",
		"ARCH_SIGNATURE_BLOAT",
		"ARCH_MONOLITH_AND_NESTING",
	}
	totalFindings := 0
	totalScore := 0
	for _, cat := range categories {
		c := catCounts[cat]
		pts := catPoints[cat]
		totalFindings += c
		totalScore += pts
		fmt.Fprintf(w, "  %-26s : %4d findings (%5d pts)\n", cat, c, pts)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	fmt.Fprintf(w, "  TOTAL                      : %4d findings (%5d pts)\n", totalFindings, totalScore)
	fmt.Fprintln(w, "================================================================================")

	if len(leaderboard) > 0 {
		fmt.Fprintln(w, "  TOP 10 MOST SUSPICIOUS FILES (BY CUMULATIVE DEBT SCORE)")
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
		fmt.Fprintln(w, "  Rank | Score | Findings | File")
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
		limit := 10
		if len(leaderboard) < limit {
			limit = len(leaderboard)
		}
		for i := 0; i < limit; i++ {
			fe := leaderboard[i]
			fmt.Fprintf(w, "  #%2d  | %5d | %8d | %s\n", i+1, fe.score, fe.count, fe.file)
		}
		fmt.Fprintln(w, "================================================================================")
	}

	for _, f := range filtered {
		fmt.Fprintf(w, "[SCORE: %3d] [%s] [%s] %s:%d (%s)\n    %s\n",
			f.Score, f.Category, f.Severity, f.File, f.Line, f.Symbol, f.Message)
	}
	return len(filtered)
}
