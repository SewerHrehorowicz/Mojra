//go:build !(js && wasm)

package tpl

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mojra/internal/i18n"
)

const (
	appDirDefault     = "app"
	stylesDirName     = "styles"
	jsDirName         = "js"
	controllersDir    = "controllers"
	templatesDirName  = "templates"
	buildDirName      = "build"
	jsExt             = ".js"
	indexHTMLFileName = "index.html"
	styleCSSFileName     = "style.css"
	appJSFileName        = "app.js"
	colorsTSSFileName    = "colors.tss"
	fdpJSFileName        = "fdp.js"
	dispatcherJSFileName = "dispatcher.js"
	studioAtlasFile      = "studio-atlas.png"
	studioUIFile      = "studio-ui.png"
	devModeExpr       = "dev_mode"
	notDevModeExpr    = "not dev_mode"
	bangDevModeExpr   = "!dev_mode"
	errNilFS          = "nil fs"
	errReadFmt        = "read %s: %w"
	errMkdirFmt       = "mkdir %s: %w"
	errTplTranspile   = "tpl transpile error: %w"
	errCSSBundle      = "css bundle error: %w"
	errJSBundle       = "js bundle error: %w"
	errWriteAtlas     = "write studio-atlas.png: %w"
	errWriteUIAtlas   = "write studio-ui.png: %w"
)

var nonPlayerModulePrefixes = []string{
	"editor_",
	"studio_",
	"canvas_",
	"msdf_",
	"text_editor",
	"library",
	"modal_add_book",
	"modal_editor_help",
	"modal_export",
	"modal_new_book",
	"modal_studio_projects",
}

var devOnlyBlockRe = regexp.MustCompile(`(?s)/\*\s*@dev-only-start\s*\*/.*?/\*\s*@dev-only-end\s*\*/[ \t]*\r?\n?|(?s)//\s*@dev-only-start\b.*?//\s*@dev-only-end\b[ \t]*\r?\n?`)

// StripDevOnlyBlocks removes all /* @dev-only-start */ ... /* @dev-only-end */ sections from CSS or JS source.
func StripDevOnlyBlocks(src string) string {
	return devOnlyBlockRe.ReplaceAllString(src, "")
}

// StripDevNodesFromTemplates removes "if dev_mode" nodes and unwraps "if not dev_mode" control nodes in-place.
func StripDevNodesFromTemplates(templates map[string]*ParsedTemplate) {
	for _, t := range templates {
		if t != nil {
			t.Nodes = stripDevNodesSlice(t.Nodes)
		}
	}
}

func stripDevNodesSlice(rootNodes []*Node) []*Node {
	out := filterDevNodesLevel(rootNodes)
	stack := make([]*[]*Node, 0, 32)
	for i := range out {
		if len(out[i].Children) > 0 {
			stack = append(stack, &out[i].Children)
		}
	}
	for step := 0; step < 8192 && len(stack) > 0; step++ {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		filtered := filterDevNodesLevel(*top)
		*top = filtered
		for i := range filtered {
			if len(filtered[i].Children) > 0 {
				stack = append(stack, &filtered[i].Children)
			}
		}
	}
	return out
}

func filterDevNodesLevel(nodes []*Node) []*Node {
	cur := nodes
	for pass := 0; pass < 8; pass++ {
		var out []*Node
		unwrapped := false
		for i := 0; i < len(cur); i++ {
			n := cur[i]
			expr := strings.TrimSpace(n.Expr)
			if n.Kind == NodeIf && strings.EqualFold(expr, devModeExpr) {
				if i+1 < len(cur) {
					next := cur[i+1]
					if next.Kind == NodeElse {
						i++
						out = append(out, next.Children...)
						unwrapped = true
					} else if next.Kind == NodeElif {
						next.Kind = NodeIf
					}
				}
				continue
			}
			if n.Kind == NodeIf && (strings.EqualFold(expr, notDevModeExpr) || strings.EqualFold(expr, bangDevModeExpr)) {
				out = append(out, n.Children...)
				unwrapped = true
				for i+1 < len(cur) && (cur[i+1].Kind == NodeElif || cur[i+1].Kind == NodeElse) {
					i++
				}
				continue
			}
			out = append(out, n)
		}
		cur = out
		if !unwrapped {
			break
		}
	}
	return cur
}

// BundleFilesToString concatenates the given source files and optionally strips dev-only blocks.
// If all files are .tss (or .css) stylesheets, they are transpiled through the TSS/CSS template engine.
func BundleFilesToString(files []string, stripDev bool) (string, error) {
	allCSS := len(files) > 0
	for _, f := range files {
		lower := strings.ToLower(f)
		if !strings.HasSuffix(lower, tssExt) && !strings.HasSuffix(lower, cssExt) {
			allCSS = false
			break
		}
	}
	if allCSS {
		return TranspileCSSFilesToString(files, stripDev, nil)
	}

	var sb strings.Builder
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), jsExt) {
			sb.WriteString(i18n.BuildJSI18nHeader())
			break
		}
	}
	for i, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf(errReadFmt, path, err)
		}
		appendBundledJSChunk(&sb, i, raw, stripDev)
	}
	return sb.String(), nil
}

func appendBundledJSChunk(sb *strings.Builder, idx int, raw []byte, stripDev bool) {
	content := string(raw)
	if stripDev {
		content = StripDevOnlyBlocks(content)
	}
	if idx > 0 && len(content) > 0 && sb.Len() > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(content)
}

// BundleJSFiles concatenates the handwritten JS modules in order into a single production JS output file.
func BundleJSFiles(jsFiles []string, jsOutPath string) error {
	bundled, err := BundleFilesToString(jsFiles, true)
	if err != nil {
		return err
	}
	return os.WriteFile(jsOutPath, []byte(bundled), 0644)
}

// BundleCSSFiles transpiles and bundles the CSS template modules in order into a single production CSS output file.
func BundleCSSFiles(cssFiles []string, cssOutPath string) error {
	bundled, err := TranspileCSSFilesToString(cssFiles, true, nil)
	if err != nil {
		return err
	}
	return os.WriteFile(cssOutPath, []byte(bundled), 0644)
}

func bundleCSSPathsFromFS(fsys fs.FS, paths []string, stripDev bool) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf(errNilFS)
	}
	switches := DefaultCSSSwitches()
	var sb strings.Builder
	for _, path := range paths {
		slashPath := filepath.ToSlash(path)
		raw, err := fs.ReadFile(fsys, slashPath)
		if err != nil {
			return "", fmt.Errorf(errReadFmt, slashPath, err)
		}
		src := string(raw)
		if stripDev {
			src = StripDevOnlyBlocks(src)
		}
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(slashPath), tssExt), cssExt)
		rendered, err := transpileCSSWithSwitches(src, name, nil, switches, stripDev)
		if err != nil {
			return "", err
		}
		rendered = strings.TrimSpace(rendered)
		if rendered == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(rendered)
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// BundleCSSFromFS transpiles and bundles embedded app/styles/*.tss modules from fsys.
func BundleCSSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	return bundleCSSPathsFromFS(fsys, discoverFSModuleFiles(fsys, stylesDirName, tssExt, false), stripDev)
}

func bundleJSPathsFromFS(fsys fs.FS, paths []string, stripDev bool) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf(errNilFS)
	}
	var sb strings.Builder
	sb.WriteString(i18n.BuildJSI18nHeader())
	for i, path := range paths {
		slashPath := filepath.ToSlash(path)
		raw, err := fs.ReadFile(fsys, slashPath)
		if err != nil {
			return "", fmt.Errorf(errReadFmt, slashPath, err)
		}
		appendBundledJSChunk(&sb, i, raw, stripDev)
	}
	return sb.String(), nil
}

// BundleJSFromFS concatenates embedded app/js/*.js modules from fsys.
func BundleJSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	return bundleJSPathsFromFS(fsys, discoverFSModuleFiles(fsys, jsDirName, jsExt, false), stripDev)
}

// BundlePlayerCSSFromFS transpiles and bundles player-only app/styles/*.tss modules from fsys.
func BundlePlayerCSSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	return bundleCSSPathsFromFS(fsys, discoverFSModuleFiles(fsys, stylesDirName, tssExt, true), stripDev)
}

// BundlePlayerJSFromFS concatenates player-only app/js/*.js modules from fsys.
func BundlePlayerJSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	return bundleJSPathsFromFS(fsys, discoverFSModuleFiles(fsys, jsDirName, jsExt, true), stripDev)
}

func isPlayerModuleName(name string) bool {
	lower := strings.ToLower(name)
	for _, pfx := range nonPlayerModulePrefixes {
		if strings.HasPrefix(lower, pfx) {
			return false
		}
	}
	return true
}

func isPriorityBundleFile(name, ext string) bool {
	return (name == fdpJSFileName || name == dispatcherJSFileName || name == colorsTSSFileName) && strings.HasSuffix(name, ext)
}

func discoverDirModuleFiles(appDir, subDir, ext string, playerOnly bool) []string {
	dir := filepath.Join(appDir, subDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && isPriorityBundleFile(e.Name(), ext) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	for _, e := range entries {
		if e.IsDir() || isPriorityBundleFile(e.Name(), ext) || !strings.HasSuffix(strings.ToLower(e.Name()), ext) {
			continue
		}
		if playerOnly && !isPlayerModuleName(e.Name()) {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	if subDir == jsDirName {
		ctrlDir := filepath.Join(dir, controllersDir)
		if ctrlEntries, err := os.ReadDir(ctrlDir); err == nil {
			for _, ce := range ctrlEntries {
				if ce.IsDir() || !strings.HasSuffix(strings.ToLower(ce.Name()), ext) {
					continue
				}
				if playerOnly && !isPlayerModuleName(ce.Name()) {
					continue
				}
				out = append(out, filepath.Join(ctrlDir, ce.Name()))
			}
		}
	}
	if !playerOnly {
		out = append(out, discoverComponentFiles(appDir, ext)...)
	}
	return out
}

func discoverFSModuleFiles(fsys fs.FS, subDir, ext string, playerOnly bool) []string {
	if fsys == nil {
		return nil
	}
	dir := filepath.ToSlash(filepath.Join(appDirDefault, subDir))
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && isPriorityBundleFile(e.Name(), ext) {
			out = append(out, filepath.ToSlash(filepath.Join(dir, e.Name())))
		}
	}
	for _, e := range entries {
		if e.IsDir() || isPriorityBundleFile(e.Name(), ext) || !strings.HasSuffix(strings.ToLower(e.Name()), ext) {
			continue
		}
		if playerOnly && !isPlayerModuleName(e.Name()) {
			continue
		}
		out = append(out, filepath.ToSlash(filepath.Join(dir, e.Name())))
	}
	if subDir == jsDirName {
		ctrlDir := filepath.ToSlash(filepath.Join(dir, controllersDir))
		if ctrlEntries, err := fs.ReadDir(fsys, ctrlDir); err == nil {
			for _, ce := range ctrlEntries {
				if ce.IsDir() || !strings.HasSuffix(strings.ToLower(ce.Name()), ext) {
					continue
				}
				if playerOnly && !isPlayerModuleName(ce.Name()) {
					continue
				}
				out = append(out, filepath.ToSlash(filepath.Join(ctrlDir, ce.Name())))
			}
		}
	}
	return out
}

// CSSModuleFiles returns the dynamically discovered list of TSS stylesheet template files in appDir/styles (plus any appDir/components/*/*.tss).
func CSSModuleFiles(appDir string) []string {
	return discoverDirModuleFiles(appDir, stylesDirName, tssExt, false)
}

// CSSPlayerModuleFiles returns the dynamically discovered list of TSS stylesheet template files needed for standalone player mode.
func CSSPlayerModuleFiles(appDir string) []string {
	return discoverDirModuleFiles(appDir, stylesDirName, tssExt, true)
}

// JSModuleFiles returns the dynamically discovered list of JS source files in appDir/js (plus any appDir/components/*/*.js).
func JSModuleFiles(appDir string) []string {
	return discoverDirModuleFiles(appDir, jsDirName, jsExt, false)
}

// JSPlayerModuleFiles returns the dynamically discovered list of JS source files needed for standalone player mode.
func JSPlayerModuleFiles(appDir string) []string {
	return discoverDirModuleFiles(appDir, jsDirName, jsExt, true)
}

// BuildApp transpiles all .tpl files in appDir/templates to build/app/index.html,
// bundles CSS modules from appDir/styles/*.tss into build/app/style.css,
// and bundles JS modules from appDir/js/*.js into build/app/app.js.
func BuildApp(appDir string) error {
	outDir := filepath.Join(filepath.Dir(filepath.Clean(appDir)), buildDirName, appDirDefault)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf(errMkdirFmt, outDir, err)
	}
	if err := TranspileDirectoryToHTML(
		filepath.Join(appDir, templatesDirName),
		filepath.Join(outDir, indexHTMLFileName),
	); err != nil {
		return fmt.Errorf(errTplTranspile, err)
	}
	if err := BundleCSSFiles(
		CSSModuleFiles(appDir),
		filepath.Join(outDir, styleCSSFileName),
	); err != nil {
		return fmt.Errorf(errCSSBundle, err)
	}
	if err := BundleJSFiles(
		JSModuleFiles(appDir),
		filepath.Join(outDir, appJSFileName),
	); err != nil {
		return fmt.Errorf(errJSBundle, err)
	}
	if pngBytes, err := os.ReadFile(filepath.Join(appDir, studioAtlasFile)); err == nil && len(pngBytes) > 0 {
		if err := os.WriteFile(filepath.Join(outDir, studioAtlasFile), pngBytes, 0644); err != nil {
			return fmt.Errorf(errWriteAtlas, err)
		}
	}
	if uiPngBytes, err := BuildStudioUIAtlas(); err == nil && len(uiPngBytes) > 0 {
		if err := os.WriteFile(filepath.Join(outDir, studioUIFile), uiPngBytes, 0644); err != nil {
			return fmt.Errorf(errWriteUIAtlas, err)
		}
	}
	return nil
}


