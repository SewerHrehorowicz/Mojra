package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mojra/internal/book"
	"mojra/internal/tpl"
)

const (
	goModFile       = "go.mod"
	dirWWW          = "www"
	dirApp          = "app"
	dirDist         = "dist"
	dirFonts        = "fonts"
	dirIcons        = "icons"
	dirBuild        = "build"
	dirWasm         = "wasm"
	dirBooks        = "books"
	dirImages       = "images"
	demoBookID      = "nie-boj-sie-policji-rzecznej"
	tplIndexFile    = "index.tpl"
	tplIndexName    = "index"
	extHTML         = ".html"
	extTTF          = ".ttf"
	styleTSSFile    = "style.tss"
	styleCSSFile    = "style.css"
	appJSFile       = "app.js"
	wasmExecJSFile  = "wasm_exec.js"
	coverJPGFile    = "cover.jpg"
	srcAbsAppJS     = `src="/app.js"`
	srcRelAppJS     = `src="app.js"`
	urlAbsFonts     = `url("/fonts/`
	urlRelFonts     = `url("fonts/`
	errMkdirFmt     = "mkdir %s: %w"
	errTplLoadFmt   = "www tpl load error: %w"
	errMissingIndex = "missing index.tpl in %s"
	errWriteFmt     = "write %s: %w"
	errCSSBundleFmt = "www css bundle error: %w"
	errJSBundleFmt  = "www js bundle error: %w"
	errWasmBuildFmt = "błąd kompilacji mojra.wasm: %v\n"
	errSiteBuildFmt = "błąd kompilacji strony WWW: %v\n"
	errEmbedWasmFmt = "błąd osadzania STBC w dist/www/mojra.wasm: %v\n"
	msgBuiltWWW     = "✓ Zbudowano statyczną stronę WWW -> dist/www/"
)

var (
	wwwSiteTemplates = []string{"index", "inwestorzy"}
	wwwIconFiles     = []string{"favicon.ico", "icon.png"}
)

func main() {
	rootDir, err := os.Getwd()
	if err != nil {
		rootDir = "."
	}
	for step := 0; step < 6; step++ {
		if info, statErr := os.Stat(filepath.Join(rootDir, goModFile)); statErr == nil && !info.IsDir() {
			break
		}
		parent := filepath.Dir(rootDir)
		if parent == rootDir {
			break
		}
		rootDir = parent
	}

	baseWasm, _, err := book.BuildBaseWasmAssets(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, errWasmBuildFmt, err)
		os.Exit(1)
	}
	if err := buildSite(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, errSiteBuildFmt, err)
		os.Exit(1)
	}
	if err := book.EmbedDemoBookIntoWWW(rootDir, baseWasm); err != nil {
		fmt.Fprintf(os.Stderr, errEmbedWasmFmt, err)
		os.Exit(1)
	}
	fmt.Println(msgBuiltWWW)
}

func buildSite(rootDir string) error {
	wwwDir := filepath.Join(rootDir, dirWWW)
	if _, err := os.Stat(filepath.Join(wwwDir, tplIndexFile)); os.IsNotExist(err) {
		return nil
	}
	appDir := filepath.Join(rootDir, dirApp)
	outDir := filepath.Join(rootDir, dirDist, dirWWW)
	if err := os.MkdirAll(filepath.Join(outDir, dirFonts), 0755); err != nil {
		return fmt.Errorf(errMkdirFmt, outDir, err)
	}
	templates, err := tpl.LoadTemplatesFromDir(wwwDir, nil)
	if err != nil {
		return fmt.Errorf(errTplLoadFmt, err)
	}
	if _, ok := templates[tplIndexName]; !ok {
		return fmt.Errorf(errMissingIndex, wwwDir)
	}
	for _, name := range wwwSiteTemplates {
		tmpl, ok := templates[name]
		if !ok {
			continue
		}
		htmlPath := filepath.Join(outDir, name+extHTML)
		htmlStr := tpl.TranspileStaticHTML(tmpl, templates)
		relHTML := strings.ReplaceAll(htmlStr, srcAbsAppJS, srcRelAppJS)
		if err := os.WriteFile(htmlPath, []byte(relHTML), 0644); err != nil {
			return fmt.Errorf(errWriteFmt, htmlPath, err)
		}
	}
	styleCSSPath := filepath.Join(outDir, styleCSSFile)
	wwwCSSFiles := append([]string{}, tpl.CSSPlayerModuleFiles(appDir)...)
	wwwCSSFiles = append(wwwCSSFiles, filepath.Join(wwwDir, styleTSSFile))
	if err := tpl.BundleCSSFiles(wwwCSSFiles, styleCSSPath); err != nil {
		return fmt.Errorf(errCSSBundleFmt, err)
	}
	if rawCSS, err := os.ReadFile(styleCSSPath); err == nil {
		relCSS := strings.ReplaceAll(string(rawCSS), urlAbsFonts, urlRelFonts)
		if err := os.WriteFile(styleCSSPath, []byte(relCSS), 0644); err != nil {
			return err
		}
	}
	var wwwJSFiles []string
	wasmExecPath := filepath.Join(rootDir, dirBuild, dirWasm, wasmExecJSFile)
	if _, err := os.Stat(wasmExecPath); err == nil {
		wwwJSFiles = append(wwwJSFiles, wasmExecPath)
	}
	wwwJSFiles = append(wwwJSFiles, tpl.JSPlayerModuleFiles(appDir)...)
	wwwJSFiles = append(wwwJSFiles, filepath.Join(wwwDir, appJSFile))
	if err := tpl.BundleJSFiles(wwwJSFiles, filepath.Join(outDir, appJSFile)); err != nil {
		return fmt.Errorf(errJSBundleFmt, err)
	}

	if fontEntries, err := os.ReadDir(filepath.Join(appDir, dirFonts)); err == nil {
		for _, fe := range fontEntries {
			if fe.IsDir() || !strings.HasSuffix(strings.ToLower(fe.Name()), extTTF) {
				continue
			}
			if raw, rErr := os.ReadFile(filepath.Join(appDir, dirFonts, fe.Name())); rErr == nil {
				if err := os.WriteFile(filepath.Join(outDir, dirFonts, fe.Name()), raw, 0644); err != nil {
					return err
				}
			}
		}
	}
	for _, iconFile := range wwwIconFiles {
		if raw, rErr := os.ReadFile(filepath.Join(appDir, dirIcons, iconFile)); rErr == nil {
			if err := os.WriteFile(filepath.Join(outDir, iconFile), raw, 0644); err != nil {
				return err
			}
		}
	}
	if rawCover, cErr := os.ReadFile(filepath.Join(rootDir, dirBooks, demoBookID, dirImages, coverJPGFile)); cErr == nil && len(rawCover) > 0 {
		if err := os.WriteFile(filepath.Join(outDir, coverJPGFile), rawCover, 0644); err != nil {
			return err
		}
	}
	return nil
}
