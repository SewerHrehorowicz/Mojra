package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

func scanGoFile(fset *token.FileSet, absPath, relPath, pkgPath string) (FileView, []rawEdge, error) {
	pf, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		return FileView{}, nil, err
	}

	fv := FileView{
		RelPath: relPath,
		Name:    filepath.Base(relPath),
	}
	imports := collectInternalImports(pf)
	var edges []rawEdge

	for _, decl := range pf.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			syms, genEdges := scanGoGenDecl(fset, d, pkgPath, imports)
			fv.Symbols = append(fv.Symbols, syms...)
			edges = append(edges, genEdges...)
		case *ast.FuncDecl:
			if d.Name == nil {
				continue
			}
			sym, fnEdges := scanGoFuncDecl(fset, d, pkgPath, imports)
			fv.Symbols = append(fv.Symbols, sym)
			edges = append(edges, fnEdges...)
		}
	}

	return fv, edges, nil
}

func collectInternalImports(pf *ast.File) map[string]string {
	imports := make(map[string]string, len(pf.Imports))
	for _, imp := range pf.Imports {
		if imp.Path == nil {
			continue
		}
		rawPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		var targetPkg string
		if rawPath == moduleRoot {
			targetPkg = "."
		} else if strings.HasPrefix(rawPath, modulePrefix) {
			targetPkg = strings.TrimPrefix(rawPath, modulePrefix)
		} else {
			continue
		}
		alias := filepath.Base(rawPath)
		if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		}
		imports[alias] = targetPkg
	}
	return imports
}

func scanGoGenDecl(fset *token.FileSet, d *ast.GenDecl, pkgPath string, imports map[string]string) ([]SymbolView, []rawEdge) {
	var syms []SymbolView
	var edges []rawEdge

	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name == nil {
				continue
			}
			sym, typeEdges := scanGoTypeSpec(fset, s, pkgPath, imports)
			syms = append(syms, sym)
			edges = append(edges, typeEdges...)

		case *ast.ValueSpec:
			valKind := kindVar
			if d.Tok == token.CONST {
				valKind = kindConst
			}
			for _, ident := range s.Names {
				if ident == nil || ident.Name == "_" {
					continue
				}
				symID := makeSymbolID(pkgPath, ident.Name)
				syms = append(syms, SymbolView{
					ID:        symID,
					Kind:      valKind,
					Name:      ident.Name,
					LineRange: formatNodeLines(fset, s.Pos(), s.End()),
				})
				if s.Type != nil {
					edges = appendTypeRefEdges(edges, symID, pkgPath, imports, extractTypeRefs(s.Type), edgeUsesType)
				}
				for _, valExpr := range s.Values {
					edges = scanASTBodyEdges(valExpr, symID, pkgPath, imports, edges)
				}
			}
		}
	}

	return syms, edges
}

func scanGoTypeSpec(fset *token.FileSet, ts *ast.TypeSpec, pkgPath string, imports map[string]string) (SymbolView, []rawEdge) {
	symName := ts.Name.Name
	symID := makeSymbolID(pkgPath, symName)
	kind := kindType
	var fields []string
	var edges []rawEdge

	switch st := ts.Type.(type) {
	case *ast.StructType:
		kind = kindStruct
		if st.Fields != nil {
			for _, f := range st.Fields.List {
				refs := extractTypeRefs(f.Type)
				typeLabel := strings.Join(refs, "/")
				if len(f.Names) == 0 {
					if typeLabel != "" {
						fields = append(fields, typeLabel)
					}
					edges = appendTypeRefEdges(edges, symID, pkgPath, imports, refs, edgeEmbeds)
				} else {
					for _, n := range f.Names {
						if n != nil {
							fields = append(fields, n.Name+": "+typeLabel)
						}
					}
					edges = appendTypeRefEdges(edges, symID, pkgPath, imports, refs, edgeUsesType)
				}
			}
		}
	case *ast.InterfaceType:
		kind = kindInterface
		if st.Methods != nil {
			for _, m := range st.Methods.List {
				refs := extractTypeRefs(m.Type)
				if len(m.Names) == 0 {
					edges = appendTypeRefEdges(edges, symID, pkgPath, imports, refs, edgeEmbeds)
				} else {
					for _, n := range m.Names {
						if n != nil {
							fields = append(fields, n.Name+"()")
						}
					}
					edges = appendTypeRefEdges(edges, symID, pkgPath, imports, refs, edgeUsesType)
				}
			}
		}
	default:
		edges = appendTypeRefEdges(edges, symID, pkgPath, imports, extractTypeRefs(ts.Type), edgeUsesType)
	}

	sym := SymbolView{
		ID:        symID,
		Kind:      kind,
		Name:      symName,
		LineRange: formatNodeLines(fset, ts.Pos(), ts.End()),
		Fields:    fields,
	}
	return sym, edges
}

func scanGoFuncDecl(fset *token.FileSet, fn *ast.FuncDecl, pkgPath string, imports map[string]string) (SymbolView, []rawEdge) {
	symName := fn.Name.Name
	kind := kindFunc
	var edges []rawEdge

	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recvRefs := extractTypeRefs(fn.Recv.List[0].Type)
		if len(recvRefs) > 0 {
			recvType := recvRefs[0]
			symName = recvType + "." + fn.Name.Name
			kind = kindMethod
		}
	}

	symID := makeSymbolID(pkgPath, symName)
	if kind == kindMethod {
		if dotIdx := strings.IndexByte(symName, '.'); dotIdx > 0 {
			edges = append(edges, rawEdge{
				FromID: symID,
				ToPkg:  pkgPath,
				ToName: symName[:dotIdx],
				Kind:   edgeUsesType,
			})
		}
	}

	if fn.Type != nil {
		if fn.Type.Params != nil {
			for _, p := range fn.Type.Params.List {
				edges = appendTypeRefEdges(edges, symID, pkgPath, imports, extractTypeRefs(p.Type), edgeUsesType)
			}
		}
		if fn.Type.Results != nil {
			for _, r := range fn.Type.Results.List {
				edges = appendTypeRefEdges(edges, symID, pkgPath, imports, extractTypeRefs(r.Type), edgeUsesType)
			}
		}
	}

	if fn.Body != nil {
		edges = scanASTBodyEdges(fn.Body, symID, pkgPath, imports, edges)
	}

	sym := SymbolView{
		ID:        symID,
		Kind:      kind,
		Name:      symName,
		LineRange: formatNodeLines(fset, fn.Pos(), fn.End()),
	}
	return sym, edges
}

func scanASTBodyEdges(root ast.Node, fromID, pkgPath string, imports map[string]string, edges []rawEdge) []rawEdge {
	ast.Inspect(root, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			switch fun := x.Fun.(type) {
			case *ast.Ident:
				name := fun.Name
				if !goBuiltinTypes[name] && name != identTrue && name != identFalse && name != identNil {
					edges = append(edges, rawEdge{
						FromID: fromID,
						ToPkg:  pkgPath,
						ToName: name,
						Kind:   edgeCalls,
					})
				}
			case *ast.SelectorExpr:
				if pkgIdent, ok := fun.X.(*ast.Ident); ok {
					if targetPkg, isImp := imports[pkgIdent.Name]; isImp {
						edges = append(edges, rawEdge{
							FromID: fromID,
							ToPkg:  targetPkg,
							ToName: fun.Sel.Name,
							Kind:   edgeCalls,
						})
						return true
					}
				}
				edges = append(edges, rawEdge{
					FromID: fromID,
					ToPkg:  pkgPath,
					ToName: fun.Sel.Name,
					Kind:   edgeCalls,
				})
			}
		case *ast.CompositeLit:
			if x.Type != nil {
				edges = appendTypeRefEdges(edges, fromID, pkgPath, imports, extractTypeRefs(x.Type), edgeUsesType)
			}
		}
		return true
	})
	return edges
}

func extractTypeRefs(expr ast.Expr) []string {
	if expr == nil {
		return nil
	}
	var refs []string
	seen := make(map[string]bool)
	ast.Inspect(expr, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.StructType, *ast.InterfaceType, *ast.FuncType:
			if n != expr {
				return false
			}
		case *ast.SelectorExpr:
			if pkgIdent, ok := x.X.(*ast.Ident); ok && x.Sel != nil {
				qual := pkgIdent.Name + "." + x.Sel.Name
				if !seen[qual] {
					seen[qual] = true
					refs = append(refs, qual)
				}
				return false
			}
		case *ast.Ident:
			name := x.Name
			if !goBuiltinTypes[name] && name != "_" && !seen[name] {
				seen[name] = true
				refs = append(refs, name)
			}
		}
		return true
	})
	return refs
}

func appendTypeRefEdges(edges []rawEdge, fromID, pkgPath string, imports map[string]string, refs []string, edgeKind string) []rawEdge {
	for _, ref := range refs {
		if dotIdx := strings.IndexByte(ref, '.'); dotIdx > 0 {
			alias := ref[:dotIdx]
			typeName := ref[dotIdx+1:]
			if targetPkg, ok := imports[alias]; ok {
				edges = append(edges, rawEdge{
					FromID: fromID,
					ToPkg:  targetPkg,
					ToName: typeName,
					Kind:   edgeKind,
				})
			}
			continue
		}
		if goBuiltinTypes[ref] {
			continue
		}
		edges = append(edges, rawEdge{
			FromID: fromID,
			ToPkg:  pkgPath,
			ToName: ref,
			Kind:   edgeKind,
		})
	}
	return edges
}

func formatNodeLines(fset *token.FileSet, start, end token.Pos) string {
	sLine := fset.Position(start).Line
	eLine := fset.Position(end).Line
	if eLine <= sLine {
		return fmt.Sprintf(lineSingleFmt, sLine)
	}
	return fmt.Sprintf(lineRangeFmt, sLine, eLine)
}

func makeSymbolID(pkgPath, symName string) string {
	var b strings.Builder
	b.Grow(len(pkgPath) + len(symName) + 2)
	for i := 0; i < len(pkgPath); i++ {
		c := pkgPath[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
		} else {
			b.WriteByte('-')
		}
	}
	b.WriteByte('-')
	for i := 0; i < len(symName); i++ {
		c := symName[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}
