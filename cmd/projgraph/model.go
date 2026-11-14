package main

const (
	flagDirName     = "dir"
	flagOutTxtName  = "out-txt"
	flagOutHTMLName = "out-html"

	flagDirDesc     = "comma-separated relative directories to scan (default: entire workspace)"
	flagOutTxtDesc  = "output path for plain-text AI project graph"
	flagOutHTMLDesc = "output path for self-contained HTML project graph"

	defaultOutTxtRel  = "docs/project.graph"
	defaultOutHTMLRel = "docs/project_graph.html"
	defaultTplDirRel  = "cmd/projgraph"
	defaultScopeAll   = "workspace"

	tplNameGraph     = "graph"
	tssFileNameGraph = "graph.tss"
	cssHrefPlaceholder = `<link rel="stylesheet" href="graph.css">`
	styleOpenTag       = "<style>\n"
	styleCloseTag      = "</style>"

	extGo  = ".go"
	extTpl = ".tpl"
	extJS  = ".js"
	testGoSuffix = "_test.go"
	goModFile    = "go.mod"
	modulePrefix = "mojra/"
	moduleRoot   = "mojra"

	dirInternal = "internal"
	dirCmd      = "cmd"
	dirApp      = "app"
	dirWWW      = "www"

	skipDirGit         = ".git"
	skipDirDist        = "dist"
	skipDirBuild       = "build"
	skipDirLogs        = "logs"
	skipDirNodeModules = "node_modules"
	skipDirVendor      = "vendor"

	kindStruct    = "struct"
	kindInterface = "interface"
	kindType      = "type"
	kindFunc      = "func"
	kindMethod    = "method"
	kindConst     = "const"
	kindVar       = "var"
	kindComponent = "component"
	kindTemplate  = "template"
	kindAction    = "action"
	kindJSFunc    = "js_func"

	edgeCalls      = "calls"
	edgeUsesType   = "uses_type"
	edgeEmbeds     = "embeds"
	edgeRenders    = "renders"
	edgeDispatches = "dispatches"
	edgeIncludes   = "includes"

	pkgUIActions   = "actions"
	pkgUITemplates = "templates"
	pkgUIJS        = "js"

	hdrPrefix      = "# MOJRA PROJECT GRAPH"
	hdrScope       = "# scope: "
	hdrStatsFmt    = "# dirs: %d | files: %d | symbols: %d\n\n"
	lineDirFmt     = "dir %s\n"
	lineFileFmt    = "  file %s\n"
	lineSymFmt     = "    %s %s [%s] #%s\n"
	lineFieldFmt   = "      . %s\n"
	lineOutEdgeFmt = "      -> %s: %s (#%s)\n"
	lineInEdgeFmt  = "      <- %s: %s (#%s)\n"
	lineRangeFmt   = "L%d-L%d"
	lineSingleFmt  = "L%d"

	errBuildGraphFmt = "projgraph build error: %v\n"
	errWriteTxtFmt   = "projgraph write text error: %v\n"
	errWriteHTMLFmt  = "projgraph write html error: %v\n"

	identTrue    = "true"
	identFalse   = "false"
	identNil     = "nil"
	identIota    = "iota"
	actionSpecsVar  = "ActionSpecs"
	actionSpecsPriv = "actionSpecs"
	tplComponentKw  = "component"
	tplSlotKw       = "slot"
	tplIncludeKw    = "include"
)

var (
	defaultScanRoots = []string{dirInternal, dirCmd, dirApp, dirWWW}

	goBuiltinTypes = map[string]bool{
		"any":        true,
		"bool":       true,
		"byte":       true,
		"comparable": true,
		"complex64":  true,
		"complex128": true,
		"error":      true,
		"float32":    true,
		"float64":    true,
		"int":        true,
		"int8":       true,
		"int16":      true,
		"int32":      true,
		"int64":      true,
		"rune":       true,
		"string":     true,
		"uint":       true,
		"uint8":      true,
		"uint16":     true,
		"uint32":     true,
		"uint64":     true,
		"uintptr":    true,
	}
)

// Config holds CLI configuration for building and rendering the project graph.
type Config struct {
	RootDir     string
	FilterDirs  []string
	OutTxtPath  string
	OutHTMLPath string
}

// GraphView represents the root view model passed to graph.tpl and WriteTextGraph.
type GraphView struct {
	Scope        string
	TotalDirs    int
	TotalFiles   int
	TotalSymbols int
	Dirs         []DirView
}

// DirView groups scanned files under a single relative directory path.
type DirView struct {
	Path  string
	Files []FileView
}

// FileView holds all symbols discovered inside a single source file.
type FileView struct {
	RelPath string
	Name    string
	Symbols []SymbolView
}

// SymbolView describes a declared type, function, method, template, component, or action.
type SymbolView struct {
	ID        string
	Kind      string
	Name      string
	LineRange string
	Fields    []string
	OutEdges  []EdgeView
	InEdges   []EdgeView
}

// EdgeView represents a resolved directional dependency between two symbols.
type EdgeView struct {
	Kind        string
	TargetID    string
	TargetLabel string
}

// rawEdge represents an unresolved reference emitted by a file scanner before global linking.
type rawEdge struct {
	FromID string
	ToPkg  string
	ToName string
	Kind   string
}
