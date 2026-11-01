package tpl

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

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

func stripDevNodesSlice(nodes []*Node) []*Node {
	var out []*Node
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]
		expr := strings.TrimSpace(n.Expr)
		if n.Kind == NodeIf && expr == "dev_mode" {
			if i+1 < len(nodes) {
				next := nodes[i+1]
				if next.Kind == NodeElse {
					i++
					next.Children = stripDevNodesSlice(next.Children)
					out = append(out, next.Children...)
				} else if next.Kind == NodeElif {
					next.Kind = NodeIf
				}
			}
			continue
		}
		n.Children = stripDevNodesSlice(n.Children)
		if n.Kind == NodeIf && (expr == "not dev_mode" || expr == "!dev_mode") {
			out = append(out, n.Children...)
			for i+1 < len(nodes) && (nodes[i+1].Kind == NodeElif || nodes[i+1].Kind == NodeElse) {
				i++
			}
			continue
		}
		out = append(out, n)
	}
	return out
}

// BundleFilesToString concatenates the given source files and optionally strips dev-only blocks.
// If all files are .tss (or .css) stylesheets, they are transpiled through the TSS/CSS template engine.
func BundleFilesToString(files []string, stripDev bool) (string, error) {
	allCSS := len(files) > 0
	for _, f := range files {
		lower := strings.ToLower(f)
		if !strings.HasSuffix(lower, ".tss") && !strings.HasSuffix(lower, ".css") {
			allCSS = false
			break
		}
	}
	if allCSS {
		return TranspileCSSFilesToString(files, stripDev, nil)
	}

	var sb strings.Builder
	for _, f := range files {
		if strings.HasSuffix(filepath.ToSlash(f), "js/editor_canvas.js") {
			appDir := filepath.Dir(filepath.Dir(f))
			if _, metricsJS, err := BuildStudioMSDFAtlas(appDir); err == nil && metricsJS != "" {
				sb.WriteString(metricsJS)
			}
			break
		}
	}
	for i, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		content := string(raw)
		if stripDev {
			content = StripDevOnlyBlocks(content)
		}
		if i > 0 && len(content) > 0 && sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(content)
	}
	return sb.String(), nil
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

// BundleCSSFromFS transpiles and bundles embedded app/styles/*.tss modules from fsys.
func BundleCSSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf("nil fs")
	}
	switches := DefaultCSSSwitches()
	var sb strings.Builder
	for _, path := range CSSModuleFiles("app") {
		slashPath := filepath.ToSlash(path)
		raw, err := fs.ReadFile(fsys, slashPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", slashPath, err)
		}
		src := string(raw)
		if stripDev {
			src = StripDevOnlyBlocks(src)
		}
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(slashPath), ".tss"), ".css")
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

// BundleJSFromFS concatenates embedded app/js/*.js modules from fsys and prepends MSDF glyph metrics.
func BundleJSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf("nil fs")
	}
	var sb strings.Builder
	if _, metricsJS, err := BuildStudioMSDFAtlasFromFS(fsys); err == nil && metricsJS != "" {
		sb.WriteString(metricsJS)
	}
	for i, path := range JSModuleFiles("app") {
		slashPath := filepath.ToSlash(path)
		raw, err := fs.ReadFile(fsys, slashPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", slashPath, err)
		}
		content := string(raw)
		if stripDev {
			content = StripDevOnlyBlocks(content)
		}
		if i > 0 && len(content) > 0 && sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(content)
	}
	return sb.String(), nil
}

// discoverComponentFiles returns sorted paths matching appDir/components/*/*<ext>.
func discoverComponentFiles(appDir, ext string) []string {
	compRoot := filepath.Join(appDir, "components")
	entries, err := os.ReadDir(compRoot)
	if err != nil {
		return nil
	}
	var found []string
	for _, d := range entries {
		if !d.IsDir() {
			continue
		}
		subDir := filepath.Join(compRoot, d.Name())
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, f := range subEntries {
			if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ext) {
				found = append(found, filepath.Join(subDir, f.Name()))
			}
		}
	}
	sort.Strings(found)
	return found
}

// CSSModuleFiles returns the ordered list of TSS stylesheet template files in appDir/styles (plus any appDir/components/*/*.tss).
func CSSModuleFiles(appDir string) []string {
	names := []string{
		"colors.tss",
		"base.tss",
		"app.tss",
		"topbar.tss",
		"library.tss",
		"player.tss",
		"editor_canvas.tss",
		"editor_inspector.tss",
		"editor_perf.tss",
		"modals.tss",
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, filepath.Join(appDir, "styles", n))
	}
	out = append(out, discoverComponentFiles(appDir, ".tss")...)
	return out
}

// CSSPlayerModuleFiles returns the ordered list of TSS stylesheet template files needed for standalone player mode.
func CSSPlayerModuleFiles(appDir string) []string {
	names := []string{
		"colors.tss",
		"base.tss",
		"app.tss",
		"topbar.tss",
		"player.tss",
		"modals.tss",
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, filepath.Join(appDir, "styles", n))
	}
	return out
}

// JSModuleFiles returns the ordered list of JS source files in appDir/js (plus any appDir/components/*/*.js).
func JSModuleFiles(appDir string) []string {
	names := []string{
		"core.js",
		"audio.js",
		"player.js",
		"editor_webgl.js",
		"editor_canvas.js",
		"editor_wires.js",
		"editor_inspector.js",
		"editor_gestures.js",
		"editor_perf.js",
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, filepath.Join(appDir, "js", n))
	}
	out = append(out, discoverComponentFiles(appDir, ".js")...)
	return out
}

// JSPlayerModuleFiles returns the ordered list of JS source files needed for standalone player mode.
func JSPlayerModuleFiles(appDir string) []string {
	names := []string{
		"core.js",
		"audio.js",
		"player.js",
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, filepath.Join(appDir, "js", n))
	}
	return out
}

// BuildApp transpiles all .tpl files in appDir/templates to build/app/index.html,
// bundles CSS modules from appDir/styles/*.css into build/app/style.css,
// and bundles JS modules from appDir/js/*.js into build/app/app.js.
func BuildApp(appDir string) error {
	outDir := filepath.Join(filepath.Dir(filepath.Clean(appDir)), "build", "app")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	if err := TranspileDirectoryToHTML(
		filepath.Join(appDir, "templates"),
		filepath.Join(outDir, "index.html"),
	); err != nil {
		return fmt.Errorf("tpl transpile error: %w", err)
	}
	if err := BundleCSSFiles(
		CSSModuleFiles(appDir),
		filepath.Join(outDir, "style.css"),
	); err != nil {
		return fmt.Errorf("css bundle error: %w", err)
	}
	if err := BundleJSFiles(
		JSModuleFiles(appDir),
		filepath.Join(outDir, "app.js"),
	); err != nil {
		return fmt.Errorf("js bundle error: %w", err)
	}
	if pngBytes, _, err := BuildStudioMSDFAtlas(appDir); err == nil && len(pngBytes) > 0 {
		if err := os.WriteFile(filepath.Join(outDir, "studio-atlas.png"), pngBytes, 0644); err != nil {
			return fmt.Errorf("write studio-atlas.png: %w", err)
		}
	}
	return nil
}

// BuildPlayerOnlyApp builds a player-only bundle into outDir, omitting library and editor templates, CSS, and JS.
func BuildPlayerOnlyApp(appDir, outDir string) error {
	if outDir == "" {
		outDir = filepath.Join(filepath.Dir(filepath.Clean(appDir)), "build", "app")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	templates, err := LoadTemplatesFromDir(filepath.Join(appDir, "templates"), nil)
	if err != nil {
		return fmt.Errorf("tpl load error: %w", err)
	}
	delete(templates, "library_view")
	delete(templates, "editor_view")
	delete(templates, "editor_context_panel")
	delete(templates, "editor_history_panel")
	delete(templates, "editor_breakdown_panel")
	StripDevNodesFromTemplates(templates)
	idxTpl, ok := templates["index"]
	if !ok {
		return fmt.Errorf("missing index.tpl in %s", filepath.Join(appDir, "templates"))
	}
	htmlStr := TranspileStaticHTML(idxTpl, templates)
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(htmlStr), 0644); err != nil {
		return fmt.Errorf("write index.html: %w", err)
	}
	if err := BundleCSSFiles(
		CSSPlayerModuleFiles(appDir),
		filepath.Join(outDir, "style.css"),
	); err != nil {
		return fmt.Errorf("css bundle error: %w", err)
	}
	if err := BundleJSFiles(
		JSPlayerModuleFiles(appDir),
		filepath.Join(outDir, "app.js"),
	); err != nil {
		return fmt.Errorf("js bundle error: %w", err)
	}
	return nil
}
