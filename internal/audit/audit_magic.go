package audit

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"sort"
	"strconv"
	"strings"
)

// RunMagic executes the zero-tolerance magic string audit across all method and function bodies.
// Every non-empty string literal inside a Go FuncDecl/FuncLit body or JS function body is flagged,
// classified, scored, and sorted descending by score.
func RunMagic(rootDir string) ([]Finding, error) {
	idx, err := loadWorkspaceIndex(rootDir)
	if err != nil {
		return nil, err
	}

	jsFuncs, _ := collectArchJSFuncs(idx)
	findings := analyzeMagicFuncStrings(idx, jsFuncs)

	for i := range findings {
		normalizeFinding(&findings[i])
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Score != findings[j].Score {
			return findings[i].Score > findings[j].Score
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

func analyzeMagicFuncStrings(idx *workspaceIndex, jsFuncs []jsFuncInfo) []Finding {
	var findings []Finding

	for _, gf := range idx.goFiles {
		if gf.isTest || isGeneratedOrWindowFile(gf.relPath) {
			continue
		}

		for _, decl := range gf.file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil || d.Name == nil {
					continue
				}
				sym := funcSymbolName(d)
				findings = collectGoBodyMagicStrings(idx.fset, gf.relPath, sym, d.Body, findings)
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, val := range vs.Values {
						ast.Inspect(val, func(n ast.Node) bool {
							fl, ok := n.(*ast.FuncLit)
							if !ok || fl.Body == nil {
								return true
							}
							sym := "<anon>"
							if i < len(vs.Names) && vs.Names[i] != nil {
								sym = vs.Names[i].Name
							}
							findings = collectGoBodyMagicStrings(idx.fset, gf.relPath, sym, fl.Body, findings)
							return false
						})
					}
				}
			}
		}
	}

	for _, fn := range jsFuncs {
		if fn.name == "bindings" {
			continue
		}
		for _, loc := range reJSStrLiteral.FindAllStringIndex(fn.body, -1) {
			raw := fn.body[loc[0]:loc[1]]
			if len(raw) <= 2 {
				continue
			}
			val := raw[1 : len(raw)-1]
			if isBenignDelimiterOrSingleChar(val) {
				continue
			}
			line := fn.startLine + strings.Count(fn.body[:loc[0]], "\n")
			cat, sev, score := classifyMagicString(val, false, false)
			findings = append(findings, Finding{
				Category: cat,
				Severity: sev,
				File:     fn.file,
				Line:     line,
				FuncName: fn.name,
				Symbol:   fn.name,
				Message:  fmt.Sprintf("Magic string literal %q inside JS function %s(); move to top-level constant dictionary", truncateStr(val, 60), fn.name),
				Score:    score,
			})
		}
	}

	return findings
}

func collectGoBodyMagicStrings(fset *token.FileSet, relPath, funcSym string, body *ast.BlockStmt, out []Finding) []Finding {
	branchLits := make(map[*ast.BasicLit]bool)
	proseLits := make(map[*ast.BasicLit]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CaseClause:
			for _, expr := range x.List {
				markBasicStrings(expr, branchLits)
			}
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ || x.Op == token.LSS || x.Op == token.GTR || x.Op == token.LEQ || x.Op == token.GEQ {
				markBasicStrings(x.X, branchLits)
				markBasicStrings(x.Y, branchLits)
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok {
					switch pkg.Name {
					case "strings", "regexp", "filepath", "path":
						for _, arg := range x.Args {
							markBasicStrings(arg, branchLits)
						}
					case "fmt", "errors", "log":
						for _, arg := range x.Args {
							markBasicStrings(arg, proseLits)
						}
					}
				}
			}
		}
		return true
	})

	ast.Inspect(body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val, err := strconv.Unquote(lit.Value)
		if err != nil || isBenignDelimiterOrSingleChar(val) {
			return true
		}
		pos := fset.Position(lit.Pos())
		cat, sev, score := classifyMagicString(val, branchLits[lit], proseLits[lit])
		out = append(out, Finding{
			Category: cat,
			Severity: sev,
			File:     relPath,
			Line:     pos.Line,
			FuncName: funcSym,
			Symbol:   funcSym,
			Message:  fmt.Sprintf("Magic string literal %q (%d chars) in %s(); extract to package const/table or .tpl", truncateStr(val, 64), len(val), funcSym),
			Score:    score,
		})
		return true
	})

	return out
}

func markBasicStrings(expr ast.Expr, dest map[*ast.BasicLit]bool) {
	if expr == nil {
		return
	}
	ast.Inspect(expr, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			dest[lit] = true
		}
		return true
	})
}

func isBenignDelimiterOrSingleChar(val string) bool {
	if len(val) <= 1 {
		return true
	}
	for i := 0; i < len(val); i++ {
		c := val[i]
		if c == '%' && i+1 < len(val) {
			if end, ok := consumeFmtSpecifier(val, i); ok {
				i = end - 1
				continue
			}
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func consumeFmtSpecifier(val string, start int) (int, bool) {
	j := start + 1
	if j >= len(val) {
		return start, false
	}
	if val[j] == '%' {
		return j + 1, true
	}
	for j < len(val) {
		switch val[j] {
		case '+', '-', '#', ' ', '0', '.', '*', '[', ']':
			j++
		default:
			if val[j] >= '0' && val[j] <= '9' {
				j++
			} else {
				goto checkVerb
			}
		}
	}
checkVerb:
	if j >= len(val) {
		return start, false
	}
	switch val[j] {
	case 'v', 'T', 't', 'b', 'c', 'd', 'o', 'O', 'q', 'x', 'X', 'U', 'e', 'E', 'f', 'F', 'g', 'G', 's', 'p', 'w':
		return j + 1, true
	default:
		return start, false
	}
}

func classifyMagicString(val string, isBranch, isProseCall bool) (category, severity string, score int) {
	cappedLen := len(val)
	if cappedLen > 120 {
		cappedLen = 120
	}
	wordCount := strings.Count(strings.TrimSpace(val), " ") + 1

	switch {
	case isBranch:
		score = 40 + cappedLen/2
		return "MAGIC_BRANCH_STRING", "ERROR", score
	case isProseCall || len(val) >= 24 || wordCount >= 3:
		score = 35 + cappedLen/2
		return "MAGIC_PROSE_STRING", "ERROR", score
	default:
		score = 15 + cappedLen/3
		sev := "WARNING"
		if len(val) >= 8 {
			sev = "ERROR"
		}
		return "MAGIC_FUNC_STRING", sev, score
	}
}

// PrintMagicReportWithLang writes the magic-string audit summary, Top 15 Files leaderboard,
// Top 15 Functions/Methods leaderboard, and ranked findings to w.
func PrintMagicReportWithLang(w io.Writer, findings []Finding, filterCategory, filterLang, filterSeverity string) int {
	catCounts := make(map[string]int)
	catPoints := make(map[string]int)
	sevCounts := make(map[string]int)

	type rankEntry struct {
		key   string
		score int
		count int
	}
	fileRanks := make(map[string]*rankEntry)
	funcRanks := make(map[string]*rankEntry)

	var filtered []Finding
	for _, f := range findings {
		normalizeFinding(&f)
		if !MatchesFilters(f, filterCategory, filterLang, filterSeverity) {
			continue
		}
		catCounts[f.Category]++
		catPoints[f.Category] += f.Score
		sevCounts[f.Severity]++

		fe, ok := fileRanks[f.File]
		if !ok {
			fe = &rankEntry{key: f.File}
			fileRanks[f.File] = fe
		}
		fe.score += f.Score
		fe.count++

		fnKey := f.File + " :: " + f.Symbol
		fne, ok := funcRanks[fnKey]
		if !ok {
			fne = &rankEntry{key: fnKey}
			funcRanks[fnKey] = fne
		}
		fne.score += f.Score
		fne.count++

		filtered = append(filtered, f)
	}

	sortRanks := func(m map[string]*rankEntry) []*rankEntry {
		out := make([]*rankEntry, 0, len(m))
		for _, e := range m {
			out = append(out, e)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].score != out[j].score {
				return out[i].score > out[j].score
			}
			if out[i].count != out[j].count {
				return out[i].count > out[j].count
			}
			return out[i].key < out[j].key
		})
		return out
	}

	topFiles := sortRanks(fileRanks)
	topFuncs := sortRanks(funcRanks)

	fmt.Fprintln(w, "================================================================================")
	if filterLang != "" || filterSeverity != "" || filterCategory != "" {
		fmt.Fprintf(w, "  MOJRA MAGIC STRING AUDIT REPORT (lang=%s severity=%s category=%s)\n", filterLang, filterSeverity, filterCategory)
	} else {
		fmt.Fprintln(w, "  MOJRA MAGIC STRING AUDIT REPORT (ZERO STRINGS IN METHODS)")
	}
	fmt.Fprintf(w, "  SEVERITY: ERROR=%d | WARNING=%d | INFO=%d\n", sevCounts["ERROR"], sevCounts["WARNING"], sevCounts["INFO"])
	fmt.Fprintln(w, "================================================================================")

	categories := []string{
		"MAGIC_BRANCH_STRING",
		"MAGIC_PROSE_STRING",
		"MAGIC_FUNC_STRING",
	}
	totalFindings := 0
	totalScore := 0
	for _, cat := range categories {
		c := catCounts[cat]
		pts := catPoints[cat]
		totalFindings += c
		totalScore += pts
		fmt.Fprintf(w, "  %-26s : %5d strings (%7d pts)\n", cat, c, pts)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	fmt.Fprintf(w, "  TOTAL                      : %5d strings (%7d pts)\n", totalFindings, totalScore)
	fmt.Fprintln(w, "================================================================================")

	if len(topFiles) > 0 {
		fmt.Fprintln(w, "  TOP 15 FILES BY IN-METHOD MAGIC STRINGS")
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
		fmt.Fprintln(w, "  Rank |   Score | Strings | File")
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
		limit := 15
		if len(topFiles) < limit {
			limit = len(topFiles)
		}
		for i := 0; i < limit; i++ {
			fe := topFiles[i]
			fmt.Fprintf(w, "  #%2d  | %7d | %7d | %s\n", i+1, fe.score, fe.count, fe.key)
		}
		fmt.Fprintln(w, "================================================================================")
	}

	if len(topFuncs) > 0 {
		fmt.Fprintln(w, "  TOP 15 FUNCTIONS/METHODS BY MAGIC STRINGS")
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
		fmt.Fprintln(w, "  Rank |   Score | Strings | File :: Function")
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
		limit := 15
		if len(topFuncs) < limit {
			limit = len(topFuncs)
		}
		for i := 0; i < limit; i++ {
			fne := topFuncs[i]
			fmt.Fprintf(w, "  #%2d  | %7d | %7d | %s\n", i+1, fne.score, fne.count, fne.key)
		}
		fmt.Fprintln(w, "================================================================================")
	}

	for _, f := range filtered {
		fmt.Fprintf(w, "[SCORE: %3d] [%s] [%s] %s:%d (%s)\n    %s\n",
			f.Score, f.Category, f.Severity, f.File, f.Line, f.Symbol, f.Message)
	}
	return len(filtered)
}
