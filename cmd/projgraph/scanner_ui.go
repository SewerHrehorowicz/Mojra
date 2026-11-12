package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	dirInternalActions = "internal/actions"
)

var (
	reTplDefLine    = regexp.MustCompile(`^(?:\{\s*def\s+([a-zA-Z0-9_]+)|def:\s*([a-zA-Z0-9_]+)|@component\s+([a-zA-Z0-9_]+))`)
	reTplIncludeUse = regexp.MustCompile(`^\s*@([a-zA-Z0-9_]+)\b`)
	reTplActionAttr = regexp.MustCompile(`(?:^|\s)(?:action|data-backdrop-action):\s*"?([A-Za-z0-9_]+)"?`)
	reJSFuncHeader  = regexp.MustCompile(`(?m)^\s*(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(([^)]*)\)\s*\{`)
	reJSActionCall  = regexp.MustCompile(`\b(?:sendServerAction|postServerActionSilent)\s*\(\s*["']([A-Za-z0-9_]+)["']`)
	reJSIdentCall   = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
)

func scanTplFile(absPath, relPath string) (FileView, []rawEdge, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return FileView{}, nil, err
	}

	pkgDir := filepath.ToSlash(filepath.Dir(relPath))
	baseName := filepath.Base(relPath)
	tplName := strings.TrimSuffix(baseName, extTpl)
	rootID := makeSymbolID(pkgDir, tplName)

	lines := strings.Split(string(raw), "\n")
	lineCount := len(lines)
	if lineCount < 1 {
		lineCount = 1
	}

	fv := FileView{
		RelPath: relPath,
		Name:    baseName,
	}
	rootSym := SymbolView{
		ID:        rootID,
		Kind:      kindTemplate,
		Name:      tplName,
		LineRange: fmt.Sprintf(lineRangeFmt, 1, lineCount),
	}

	var compSyms []SymbolView
	var edges []rawEdge

	for idx, line := range lines {
		lineNo := idx + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if m := reTplDefLine.FindStringSubmatch(trimmed); len(m) > 0 {
			defName := m[1]
			if defName == "" {
				defName = m[2]
			}
			if defName == "" {
				defName = m[3]
			}
			if defName != "" {
				compSyms = append(compSyms, SymbolView{
					ID:        makeSymbolID(pkgDir, defName),
					Kind:      kindComponent,
					Name:      defName,
					LineRange: fmt.Sprintf(lineSingleFmt, lineNo),
				})
			}
			continue
		}

		if m := reTplIncludeUse.FindStringSubmatch(line); len(m) > 1 {
			incName := m[1]
			if incName != tplComponentKw && incName != tplSlotKw && incName != tplIncludeKw {
				edges = append(edges, rawEdge{
					FromID: rootID,
					ToPkg:  pkgDir,
					ToName: incName,
					Kind:   edgeIncludes,
				})
			}
		}

		for _, m := range reTplActionAttr.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 && m[1] != "" {
				edges = append(edges, rawEdge{
					FromID: rootID,
					ToPkg:  dirInternalActions,
					ToName: m[1],
					Kind:   edgeDispatches,
				})
			}
		}
	}

	fv.Symbols = append(fv.Symbols, rootSym)
	fv.Symbols = append(fv.Symbols, compSyms...)
	return fv, edges, nil
}

func scanJSFile(absPath, relPath string) (FileView, []rawEdge, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return FileView{}, nil, err
	}

	src := string(raw)
	pkgDir := filepath.ToSlash(filepath.Dir(relPath))
	fv := FileView{
		RelPath: relPath,
		Name:    filepath.Base(relPath),
	}
	var edges []rawEdge

	matches := reJSFuncHeader.FindAllStringSubmatchIndex(src, -1)
	for _, loc := range matches {
		fnName := src[loc[2]:loc[3]]
		startLine := 1 + strings.Count(src[:loc[0]], "\n")
		openIdx := loc[1] - 1
		closeIdx := findJSClosingBrace(src, openIdx)
		endLine := startLine
		body := ""
		if closeIdx > openIdx {
			body = src[openIdx+1 : closeIdx]
			endLine = startLine + strings.Count(src[loc[0]:closeIdx+1], "\n")
		}

		symID := makeSymbolID(pkgDir, fnName)
		fv.Symbols = append(fv.Symbols, SymbolView{
			ID:        symID,
			Kind:      kindJSFunc,
			Name:      fnName,
			LineRange: fmt.Sprintf(lineRangeFmt, startLine, endLine),
		})

		for _, actMatch := range reJSActionCall.FindAllStringSubmatch(body, -1) {
			if len(actMatch) > 1 && actMatch[1] != "" {
				edges = append(edges, rawEdge{
					FromID: symID,
					ToPkg:  dirInternalActions,
					ToName: actMatch[1],
					Kind:   edgeDispatches,
				})
			}
		}

		for _, callMatch := range reJSIdentCall.FindAllStringSubmatch(body, -1) {
			if len(callMatch) > 1 && callMatch[1] != "" && callMatch[1] != fnName {
				edges = append(edges, rawEdge{
					FromID: symID,
					ToPkg:  pkgDir,
					ToName: callMatch[1],
					Kind:   edgeCalls,
				})
			}
		}
	}

	return fv, edges, nil
}

func findJSClosingBrace(s string, openIdx int) int {
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
