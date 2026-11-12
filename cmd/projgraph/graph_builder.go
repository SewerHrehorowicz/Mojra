package main

import (
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type symRefMeta struct {
	dirIdx  int
	fileIdx int
	symIdx  int
	id      string
	label   string
}

// BuildGraph scans the repository, resolves symbol dependencies, and returns a deterministic GraphView.
func BuildGraph(cfg Config) (GraphView, error) {
	fset := token.NewFileSet()
	dirMap := make(map[string][]FileView)
	var allRawEdges []rawEdge

	for _, rootSub := range defaultScanRoots {
		baseAbs := filepath.Join(cfg.RootDir, rootSub)
		_ = filepath.WalkDir(baseAbs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == skipDirGit || name == skipDirDist || name == skipDirBuild || name == skipDirLogs || name == skipDirNodeModules || name == skipDirVendor {
					return filepath.SkipDir
				}
				return nil
			}
			rel, relErr := filepath.Rel(cfg.RootDir, path)
			if relErr != nil {
				return nil
			}
			relSlash := filepath.ToSlash(rel)
			if !matchesFilter(relSlash, cfg.FilterDirs) {
				return nil
			}
			pkgDir := filepath.ToSlash(filepath.Dir(relSlash))
			ext := filepath.Ext(relSlash)

			switch ext {
			case extGo:
				if strings.HasSuffix(relSlash, testGoSuffix) {
					return nil
				}
				fv, edges, sErr := scanGoFile(fset, path, relSlash, pkgDir)
				if sErr == nil && len(fv.Symbols) > 0 {
					dirMap[pkgDir] = append(dirMap[pkgDir], fv)
					allRawEdges = append(allRawEdges, edges...)
				}
			case extTpl:
				fv, edges, sErr := scanTplFile(path, relSlash)
				if sErr == nil && len(fv.Symbols) > 0 {
					dirMap[pkgDir] = append(dirMap[pkgDir], fv)
					allRawEdges = append(allRawEdges, edges...)
				}
			case extJS:
				fv, edges, sErr := scanJSFile(path, relSlash)
				if sErr == nil && len(fv.Symbols) > 0 {
					dirMap[pkgDir] = append(dirMap[pkgDir], fv)
					allRawEdges = append(allRawEdges, edges...)
				}
			}
			return nil
		})
	}

	var dirPaths []string
	for p := range dirMap {
		dirPaths = append(dirPaths, p)
	}
	sort.Strings(dirPaths)

	var cleanFilters []string
	for _, f := range cfg.FilterDirs {
		trimmed := strings.Trim(f, "/ ")
		if trimmed != "" {
			cleanFilters = append(cleanFilters, trimmed)
		}
	}
	scopeLabel := defaultScopeAll
	if len(cleanFilters) > 0 {
		scopeLabel = strings.Join(cleanFilters, ", ")
	}

	view := GraphView{
		Scope:     scopeLabel,
		TotalDirs: len(dirPaths),
		Dirs:      make([]DirView, 0, len(dirPaths)),
	}

	for _, dp := range dirPaths {
		files := dirMap[dp]
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})
		view.TotalFiles += len(files)
		for i := range files {
			view.TotalSymbols += len(files[i].Symbols)
		}
		view.Dirs = append(view.Dirs, DirView{
			Path:  dp,
			Files: files,
		})
	}

	linkGraphEdges(&view, allRawEdges)
	return view, nil
}

func linkGraphEdges(view *GraphView, allRawEdges []rawEdge) {
	byID := make(map[string]symRefMeta, view.TotalSymbols)
	byPkgAndName := make(map[string]string, view.TotalSymbols)
	byPkgAndMethod := make(map[string]string, view.TotalSymbols)

	for dIdx := range view.Dirs {
		pkgDir := view.Dirs[dIdx].Path
		for fIdx := range view.Dirs[dIdx].Files {
			syms := view.Dirs[dIdx].Files[fIdx].Symbols
			for sIdx := range syms {
				sym := &syms[sIdx]
				meta := symRefMeta{
					dirIdx:  dIdx,
					fileIdx: fIdx,
					symIdx:  sIdx,
					id:      sym.ID,
					label:   pkgDir + "/" + sym.Name,
				}
				byID[sym.ID] = meta
				byPkgAndName[pkgDir+":"+sym.Name] = sym.ID
				if sym.Kind == kindMethod {
					if dotIdx := strings.IndexByte(sym.Name, '.'); dotIdx > 0 {
						mKey := pkgDir + ":." + sym.Name[dotIdx+1:]
						if _, exists := byPkgAndMethod[mKey]; !exists {
							byPkgAndMethod[mKey] = sym.ID
						}
					}
				}
			}
		}
	}

	seenEdge := make(map[string]bool, len(allRawEdges))
	for _, re := range allRawEdges {
		fromMeta, okFrom := byID[re.FromID]
		if !okFrom {
			continue
		}
		targetID, okTo := byPkgAndName[re.ToPkg+":"+re.ToName]
		if !okTo {
			targetID, okTo = byPkgAndMethod[re.ToPkg+":."+re.ToName]
		}
		if !okTo || targetID == re.FromID {
			continue
		}
		toMeta, okMeta := byID[targetID]
		if !okMeta {
			continue
		}

		dedupKey := re.FromID + "->" + targetID + ":" + re.Kind
		if seenEdge[dedupKey] {
			continue
		}
		seenEdge[dedupKey] = true

		fromSym := &view.Dirs[fromMeta.dirIdx].Files[fromMeta.fileIdx].Symbols[fromMeta.symIdx]
		toSym := &view.Dirs[toMeta.dirIdx].Files[toMeta.fileIdx].Symbols[toMeta.symIdx]

		fromSym.OutEdges = append(fromSym.OutEdges, EdgeView{
			Kind:        re.Kind,
			TargetID:    toMeta.id,
			TargetLabel: toMeta.label,
		})
		toSym.InEdges = append(toSym.InEdges, EdgeView{
			Kind:        re.Kind,
			TargetID:    fromMeta.id,
			TargetLabel: fromMeta.label,
		})
	}

	for dIdx := range view.Dirs {
		for fIdx := range view.Dirs[dIdx].Files {
			syms := view.Dirs[dIdx].Files[fIdx].Symbols
			for sIdx := range syms {
				sortEdgeSlice(syms[sIdx].OutEdges)
				sortEdgeSlice(syms[sIdx].InEdges)
			}
		}
	}
}

func sortEdgeSlice(edges []EdgeView) {
	if len(edges) <= 1 {
		return
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		if edges[i].TargetLabel != edges[j].TargetLabel {
			return edges[i].TargetLabel < edges[j].TargetLabel
		}
		return edges[i].TargetID < edges[j].TargetID
	})
}

func matchesFilter(relPath string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	matchedAny := false
	hasNonEmpty := false
	for _, rawFilter := range filters {
		f := strings.Trim(rawFilter, "/ ")
		if f == "" || f == "." {
			return true
		}
		hasNonEmpty = true
		if relPath == f || strings.HasPrefix(relPath, f+"/") {
			matchedAny = true
			break
		}
	}
	if !hasNonEmpty {
		return true
	}
	return matchedAny
}
