//go:build !(js && wasm)

package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mojra/engine/templates"
)

const (
	appDirDefault        = "app"
	stylesDirName        = "styles"
	jsDirName            = "js"
	controllersDir       = "controllers"
	templatesDirName     = "templates"
	buildDirName         = "build"
	tssExt               = ".tss"
	cssExt               = ".css"
	jsExt                = ".js"
	pngExt               = ".png"
	indexHTMLFileName    = "index.html"
	styleCSSFileName     = "style.css"
	appJSFileName        = "app.js"
	colorsTSSFileName    = "colors.tss"
	fdpJSFileName        = "fdp.js"
	dispatcherJSFileName = "dispatcher.js"
	devModeExpr          = "dev_mode"
	notDevModeExpr       = "not dev_mode"
	bangDevModeExpr      = "!dev_mode"
	errNilFS             = "nil fs"
	errReadFmt           = "read %s: %w"
	errMkdirFmt          = "mkdir %s: %w"
	errTplTranspile      = "tpl transpile error: %w"
	errCSSBundle         = "css bundle error: %w"
	errJSBundle          = "js bundle error: %w"
	errWriteAssetFmt     = "write %s: %w"
)

var (
	JSHeaderBuilder         func() string = BuildJSI18nHeader
	GeneratedAssetBuilders                = map[string]func() ([]byte, error){}
	NonPlayerModulePrefixes []string
)

// StripDevNodesFromTemplates removes "if dev_mode" nodes and unwraps "if not dev_mode" control nodes in-place.
func StripDevNodesFromTemplates(loadedTemplates map[string]*templates.ParsedTemplate) {
	for _, t := range loadedTemplates {
		if t != nil {
			t.Nodes = stripDevNodesSlice(t.Nodes)
		}
	}
}

func stripDevNodesSlice(rootNodes []*templates.Node) []*templates.Node {
	out := filterDevNodesLevel(rootNodes)
	stack := make([]*[]*templates.Node, 0, 32)
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

func filterDevNodesLevel(nodes []*templates.Node) []*templates.Node {
	cur := nodes
	for pass := 0; pass < 8; pass++ {
		var out []*templates.Node
		unwrapped := false
		for i := 0; i < len(cur); i++ {
			n := cur[i]
			expr := strings.TrimSpace(n.Expr)
			if n.Kind == templates.NodeIf && strings.EqualFold(expr, devModeExpr) {
				if i+1 < len(cur) {
					next := cur[i+1]
					if next.Kind == templates.NodeElse {
						i++
						out = append(out, next.Children...)
						unwrapped = true
					} else if next.Kind == templates.NodeElif {
						next.Kind = templates.NodeIf
					}
				}
				continue
			}
			if n.Kind == templates.NodeIf && (strings.EqualFold(expr, notDevModeExpr) || strings.EqualFold(expr, bangDevModeExpr)) {
				out = append(out, n.Children...)
				unwrapped = true
				for i+1 < len(cur) && (cur[i+1].Kind == templates.NodeElif || cur[i+1].Kind == templates.NodeElse) {
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
		return templates.TranspileCSSFilesToString(files, stripDev, nil)
	}

	var sb strings.Builder
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), jsExt) {
			if JSHeaderBuilder != nil {
				sb.WriteString(JSHeaderBuilder())
			}
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
		content = templates.StripDevOnlyBlocks(content)
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
	bundled, err := templates.TranspileCSSFilesToString(cssFiles, true, nil)
	if err != nil {
		return err
	}
	return os.WriteFile(cssOutPath, []byte(bundled), 0644)
}

func bundleCSSPathsFromFS(fsys fs.FS, paths []string, stripDev bool) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf(errNilFS)
	}
	switches := templates.DefaultCSSSwitches()
	var sb strings.Builder
	for _, path := range paths {
		slashPath := filepath.ToSlash(path)
		raw, err := fs.ReadFile(fsys, slashPath)
		if err != nil {
			return "", fmt.Errorf(errReadFmt, slashPath, err)
		}
		src := string(raw)
		if stripDev {
			src = templates.StripDevOnlyBlocks(src)
		}
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(slashPath), tssExt), cssExt)
		rendered, err := templates.TranspileCSSWithSwitches(src, name, nil, switches, stripDev)
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
	if JSHeaderBuilder != nil {
		sb.WriteString(JSHeaderBuilder())
	}
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
	for _, pfx := range NonPlayerModulePrefixes {
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
		if e.IsDir() {
			if e.Name() == controllersDir {
				continue
			}
			nestedDir := filepath.Join(dir, e.Name())
			nestedEntries, nErr := os.ReadDir(nestedDir)
			if nErr != nil {
				continue
			}
			for _, ne := range nestedEntries {
				if ne.IsDir() || !strings.HasSuffix(strings.ToLower(ne.Name()), ext) {
					continue
				}
				if playerOnly && (!isPlayerModuleName(e.Name()) || !isPlayerModuleName(ne.Name())) {
					continue
				}
				out = append(out, filepath.Join(nestedDir, ne.Name()))
			}
			continue
		}
		if isPriorityBundleFile(e.Name(), ext) || !strings.HasSuffix(strings.ToLower(e.Name()), ext) {
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
		out = append(out, templates.DiscoverComponentFiles(appDir, ext)...)
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
		if e.IsDir() {
			if e.Name() == controllersDir {
				continue
			}
			nestedDir := filepath.ToSlash(filepath.Join(dir, e.Name()))
			nestedEntries, nErr := fs.ReadDir(fsys, nestedDir)
			if nErr != nil {
				continue
			}
			for _, ne := range nestedEntries {
				if ne.IsDir() || !strings.HasSuffix(strings.ToLower(ne.Name()), ext) {
					continue
				}
				if playerOnly && (!isPlayerModuleName(e.Name()) || !isPlayerModuleName(ne.Name())) {
					continue
				}
				out = append(out, filepath.ToSlash(filepath.Join(nestedDir, ne.Name())))
			}
			continue
		}
		if isPriorityBundleFile(e.Name(), ext) || !strings.HasSuffix(strings.ToLower(e.Name()), ext) {
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
	if err := templates.TranspileDirectoryToHTML(
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
	if entries, err := os.ReadDir(appDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), pngExt) {
				continue
			}
			pngBytes, readErr := os.ReadFile(filepath.Join(appDir, entry.Name()))
			if readErr == nil && len(pngBytes) > 0 {
				if writeErr := os.WriteFile(filepath.Join(outDir, entry.Name()), pngBytes, 0644); writeErr != nil {
					return fmt.Errorf(errWriteAssetFmt, entry.Name(), writeErr)
				}
			}
		}
	}
	for assetName, buildFn := range GeneratedAssetBuilders {
		if buildFn == nil {
			continue
		}
		assetBytes, err := buildFn()
		if err == nil && len(assetBytes) > 0 {
			if writeErr := os.WriteFile(filepath.Join(outDir, assetName), assetBytes, 0644); writeErr != nil {
				return fmt.Errorf(errWriteAssetFmt, assetName, writeErr)
			}
		}
	}
	return nil
}
