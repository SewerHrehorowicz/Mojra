package tpl

import (
	"encoding/base64"
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
			if n.Kind == NodeIf && expr == "dev_mode" {
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
			if n.Kind == NodeIf && (expr == "not dev_mode" || expr == "!dev_mode") {
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
		if strings.HasSuffix(filepath.ToSlash(f), "js/core.js") {
			sb.WriteString(BuildJSI18nHeader())
			break
		}
	}
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
		return "", fmt.Errorf("nil fs")
	}
	switches := DefaultCSSSwitches()
	var sb strings.Builder
	for _, path := range paths {
		slashPath := filepath.ToSlash(path)
		raw, err := fs.ReadFile(fsys, slashPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", slashPath, err)
		}
		if err := appendTranspiledCSSChunk(&sb, raw, slashPath, nil, switches, stripDev); err != nil {
			return "", err
		}
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// BundleCSSFromFS transpiles and bundles embedded app/styles/*.tss modules from fsys.
func BundleCSSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	return bundleCSSPathsFromFS(fsys, CSSModuleFiles("app"), stripDev)
}

func bundleJSPathsFromFS(fsys fs.FS, paths []string, stripDev bool, prependMSDF bool) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf("nil fs")
	}
	var sb strings.Builder
	sb.WriteString(BuildJSI18nHeader())
	if prependMSDF {
		if _, metricsJS, err := BuildStudioMSDFAtlasFromFS(fsys); err == nil && metricsJS != "" {
			sb.WriteString(metricsJS)
		}
	}
	for i, path := range paths {
		slashPath := filepath.ToSlash(path)
		raw, err := fs.ReadFile(fsys, slashPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", slashPath, err)
		}
		appendBundledJSChunk(&sb, i, raw, stripDev)
	}
	return sb.String(), nil
}

// BundleJSFromFS concatenates embedded app/js/*.js modules from fsys and prepends MSDF glyph metrics.
func BundleJSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	return bundleJSPathsFromFS(fsys, JSModuleFiles("app"), stripDev, true)
}

// BundlePlayerCSSFromFS transpiles and bundles player-only app/styles/*.tss modules from fsys.
func BundlePlayerCSSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	return bundleCSSPathsFromFS(fsys, CSSPlayerModuleFiles("app"), stripDev)
}

// BundlePlayerJSFromFS concatenates player-only app/js/*.js modules from fsys.
func BundlePlayerJSFromFS(fsys fs.FS, stripDev bool) (string, error) {
	return bundleJSPathsFromFS(fsys, JSPlayerModuleFiles("app"), stripDev, false)
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
		"editor_panels.tss",
		"editor_history.tss",
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
		"editor_webgl_scene.js",
		"editor_canvas.js",
		"editor_wires.js",
		"editor_inspector.js",
		"editor_voice.js",
		"editor_rules.js",
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
	if uiPngBytes, err := BuildStudioUIAtlas(); err == nil && len(uiPngBytes) > 0 {
		if err := os.WriteFile(filepath.Join(outDir, "studio-ui.png"), uiPngBytes, 0644); err != nil {
			return fmt.Errorf("write studio-ui.png: %w", err)
		}
	}
	return nil
}

// RenderWasmSplashPageHTML renders the standalone Web/WASM splash page HTML document.
func RenderWasmSplashPageHTML(title, author, description, coverSrc, bookID, imagelessAttr, wasmSrc, playSVG string) string {
	if strings.TrimSpace(playSVG) == "" {
		playSVG = `<svg class="wasm-splash-play-icon" viewBox="0 0 24 24" fill="currentColor"><path fill-rule="evenodd" clip-rule="evenodd" d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm-2.5 5.5 7.5 4.5-7.5 4.5v-9z"/></svg>`
	} else if !strings.Contains(playSVG, "wasm-splash-play-icon") {
		playSVG = strings.Replace(playSVG, "<svg", `<svg class="wasm-splash-play-icon"`, 1)
	}
	escTitle := escapeHTMLText(title)
	escAuthor := escapeHTMLText(author)
	escDesc := escapeHTMLText(description)
	escCover := escapeHTMLText(coverSrc)
	escWasm := escapeHTMLText(wasmSrc)
	escBookID := escapeHTMLText(bookID)

	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html lang=\"pl\">\n<head>\n")
	sb.WriteString("  <meta charset=\"UTF-8\">\n")
	sb.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no, viewport-fit=cover\">\n")
	fmt.Fprintf(&sb, "  <title>%s</title>\n", escTitle)
	sb.WriteString("  <link rel=\"icon\" type=\"image/png\" sizes=\"256x256\" href=\"icon.png\">\n")
	sb.WriteString("  <link rel=\"icon\" type=\"image/x-icon\" href=\"favicon.ico\">\n")
	sb.WriteString("  <link rel=\"stylesheet\" href=\"style.css\">\n")
	sb.WriteString("</head>\n")
	fmt.Fprintf(&sb, "<body data-theme=\"dark\" data-imageless=\"%s\" data-imageless-locked=\"%s\" data-ui-scale=\"small\">\n", imagelessAttr, imagelessAttr)
	fmt.Fprintf(&sb, "  <div class=\"app-root wasm-player-root\" id=\"app\" data-wasm-src=\"%s\" data-book-id=\"%s\">\n", escWasm, escBookID)
	sb.WriteString("    <div class=\"wasm-splash\" id=\"wasm-splash\">\n")
	if escCover != "" {
		fmt.Fprintf(&sb, "      <img class=\"wasm-splash-bg\" src=\"%s\" alt=\"\" aria-hidden=\"true\">\n", escCover)
	}
	sb.WriteString("      <div class=\"wasm-splash-content\">\n")
	fmt.Fprintf(&sb, "        <h1 class=\"wasm-splash-title\">%s</h1>\n", escTitle)
	if author != "" {
		fmt.Fprintf(&sb, "        <p class=\"wasm-splash-author\">%s</p>\n", escAuthor)
	}
	if description != "" {
		sb.WriteString("        <div class=\"wasm-splash-divider\" aria-hidden=\"true\"></div>\n")
		fmt.Fprintf(&sb, "        <p class=\"wasm-splash-desc\">%s</p>\n", escDesc)
	}
	fmt.Fprintf(&sb, "        <button class=\"wasm-splash-play-btn\" id=\"btn-wasm-play\" type=\"button\" data-wasm-play=\"true\" data-wasm-src=\"%s\" data-book-id=\"%s\" aria-label=\"Odtwórz\">\n", escWasm, escBookID)
	sb.WriteString("          " + playSVG + "\n")
	sb.WriteString("        </button>\n")
	sb.WriteString("      </div>\n")
	sb.WriteString("    </div>\n")
	sb.WriteString("  </div>\n")
	sb.WriteString("  <script src=\"app.js\"></script>\n")
	sb.WriteString("</body>\n</html>\n")
	return sb.String()
}

// BuildWWW compiles the standalone website from rootDir/www/{index.tpl,style.tss,app.js}
// using shared tokens/icons/fonts from rootDir/app into a self-contained static directory rootDir/dist/www/.
func BuildWWW(rootDir string) error {
	wwwDir := filepath.Join(rootDir, "www")
	if _, err := os.Stat(filepath.Join(wwwDir, "index.tpl")); os.IsNotExist(err) {
		return nil
	}
	appDir := filepath.Join(rootDir, "app")
	outDir := filepath.Join(rootDir, "dist", "www")
	if err := os.MkdirAll(filepath.Join(outDir, "fonts"), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	templates, err := LoadTemplatesFromDir(wwwDir, nil)
	if err != nil {
		return fmt.Errorf("www tpl load error: %w", err)
	}
	if _, ok := templates["index"]; !ok {
		return fmt.Errorf("missing index.tpl in %s", wwwDir)
	}
	financeCtx := LoadDeckFinanceContext(rootDir)
	for name, tmpl := range templates {
		htmlPath := filepath.Join(outDir, name+".html")
		htmlStr := TranspileStaticHTMLWithContext(tmpl, templates, financeCtx)
		relHTML := strings.ReplaceAll(htmlStr, `src="/app.js"`, `src="app.js"`)
		if err := os.WriteFile(htmlPath, []byte(relHTML), 0644); err != nil {
			return fmt.Errorf("write %s: %w", htmlPath, err)
		}
	}
	styleCSSPath := filepath.Join(outDir, "style.css")
	wwwCSSFiles := append([]string{}, CSSPlayerModuleFiles(appDir)...)
	wwwCSSFiles = append(wwwCSSFiles, filepath.Join(wwwDir, "style.tss"))
	if err := BundleCSSFiles(
		wwwCSSFiles,
		styleCSSPath,
	); err != nil {
		return fmt.Errorf("www css bundle error: %w", err)
	}
	if rawCSS, err := os.ReadFile(styleCSSPath); err == nil {
		relCSS := strings.ReplaceAll(string(rawCSS), `url("/fonts/`, `url("fonts/`)
		if err := os.WriteFile(styleCSSPath, []byte(relCSS), 0644); err != nil {
			return err
		}
	}
	var wwwJSFiles []string
	wasmExecPath := filepath.Join(rootDir, "build", "wasm", "wasm_exec.js")
	if _, err := os.Stat(wasmExecPath); err == nil {
		wwwJSFiles = append(wwwJSFiles, wasmExecPath)
	}
	wwwJSFiles = append(wwwJSFiles, JSPlayerModuleFiles(appDir)...)
	wwwJSFiles = append(wwwJSFiles, filepath.Join(wwwDir, "app.js"))
	if err := BundleJSFiles(
		wwwJSFiles,
		filepath.Join(outDir, "app.js"),
	); err != nil {
		return fmt.Errorf("www js bundle error: %w", err)
	}
	if wasmBytes, wErr := os.ReadFile(filepath.Join(outDir, "mojra.wasm")); wErr == nil && len(wasmBytes) > 0 {
		appJSPath := filepath.Join(outDir, "app.js")
		if rawJS, jErr := os.ReadFile(appJSPath); jErr == nil {
			jsStr := string(rawJS) + "\nwindow.__wasmBase64 = \"" + base64.StdEncoding.EncodeToString(wasmBytes) + "\";\n"
			_ = os.WriteFile(appJSPath, []byte(jsStr), 0644)
		}
	}

	if fontEntries, err := os.ReadDir(filepath.Join(appDir, "fonts")); err == nil {
		for _, fe := range fontEntries {
			if fe.IsDir() || !strings.HasSuffix(strings.ToLower(fe.Name()), ".ttf") {
				continue
			}
			if raw, rErr := os.ReadFile(filepath.Join(appDir, "fonts", fe.Name())); rErr == nil {
				if err := os.WriteFile(filepath.Join(outDir, "fonts", fe.Name()), raw, 0644); err != nil {
					return err
				}
			}
		}
	}
	for _, iconFile := range []string{"favicon.ico", "icon.png"} {
		if raw, rErr := os.ReadFile(filepath.Join(appDir, "icons", iconFile)); rErr == nil {
			if err := os.WriteFile(filepath.Join(outDir, iconFile), raw, 0644); err != nil {
				return err
			}
		}
	}
	if rawCover, cErr := os.ReadFile(filepath.Join(rootDir, "books", "nie-boj-sie-policji-rzecznej", "images", "cover.jpg")); cErr == nil && len(rawCover) > 0 {
		_ = os.WriteFile(filepath.Join(outDir, "cover.jpg"), rawCover, 0644)
	}
	return nil
}

