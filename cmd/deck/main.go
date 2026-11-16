package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "mojra/domain/book/actions"
	"mojra/domain/common"
	bookplayeractions "mojra/domain/shell/book_player/actions"
	"mojra/engine/templates"
)

const (
	goModFile          = "go.mod"
	flagOutName        = "o"
	flagOutUsage       = "Ścieżka wyjściowa pliku PDF"
	dirWWW             = "www"
	dirApp             = "domain/common"
	dirDist            = "dist"
	dirDocs            = "docs/business"
	dirFonts           = "fonts"
	extHTML            = ".html"
	extTTF             = ".ttf"
	styleTSSFile       = "style.tss"
	styleCSSFile       = "style.css"
	srcAbsAppJS        = `src="/app.js"`
	srcRelAppJS        = `src="app.js"`
	urlAbsFonts        = `url("/fonts/`
	urlRelFonts        = `url("fonts/`
	deckHTMLFile       = "deck.html"
	planHTMLFile       = "plan.html"
	przegladHTMLFile   = "przeglad.html"
	deckPDFFile        = "mojra-deck-finansowy.pdf"
	planPDFFile        = "mojra-plan.pdf"
	przegladPDFFile    = "mojra-przeglad-zalozyciela.pdf"
	osDarwin           = "darwin"
	osWindows          = "windows"
	envProgramFiles    = "ProgramFiles"
	envProgramFilesX86 = "ProgramFiles(x86)"
	envLocalAppData    = "LocalAppData"
	winVendorGoogle    = "Google"
	winAppChrome       = "Chrome"
	winVendorMS        = "Microsoft"
	winAppEdge         = "Edge"
	winVendorBrave     = "BraveSoftware"
	winAppBrave        = "Brave-Browser"
	winDirApp          = "Application"
	winBinChrome       = "chrome.exe"
	winBinEdge         = "msedge.exe"
	winBinBrave        = "brave.exe"
	tmpProfilePattern  = "mojra-deck-pdf-*"
	fileURLPrefix      = "file://"
	argHeadless        = "--headless=new"
	argDisableGPU      = "--disable-gpu"
	argDisableExt      = "--disable-extensions"
	argDisableNet      = "--disable-background-networking"
	argNoFirstRun      = "--no-first-run"
	argNoDefBrowser    = "--no-default-browser-check"
	argUserDataPrefix  = "--user-data-dir="
	argNoPDFHeader     = "--no-pdf-header-footer"
	argNoPrintHeader   = "--print-to-pdf-no-header"
	argPrintPDFPrefix  = "--print-to-pdf="
	errMkdirFmt        = "mkdir %s: %w"
	errTplLoadFmt      = "deck tpl load error: %w"
	errWriteFmt        = "write %s: %w"
	errCSSBundleFmt    = "deck css bundle error: %w"
	errMissingHTMLFmt  = "brak skompilowanego pliku %s: %w"
	errPDFRenderFmt    = "błąd generowania PDF (%v): %s"
	errNoBrowser       = "nie znaleziono przeglądarki Chromium/Chrome/Edge/Brave do wygenerowania PDF"
	errDeckBuildFmt    = "błąd kompilacji szablonu decku: %v\n"
	errDeckPDFFmt      = "błąd generowania decku PDF: %v\n"
	msgGeneratedPDFFmt = "✓ Wygenerowano deck finansowy PDF (jasny motyw) -> %s\n"
)

var (
	deckTemplateNames = []string{"deck", "plan", "przeglad"}
	darwinBrowserBins = []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
	}
	linuxBrowserBins = []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"brave-browser",
		"microsoft-edge",
	}
)

func main() {
	var outPath string
	flag.StringVar(&outPath, flagOutName, "", flagOutUsage)
	flag.Parse()

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

	if err := buildDeckHTML(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, errDeckBuildFmt, err)
		os.Exit(1)
	}
	pdfPath, err := BuildDeckPDF(rootDir, outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, errDeckPDFFmt, err)
		os.Exit(1)
	}
	fmt.Printf(msgGeneratedPDFFmt, pdfPath)
}

func buildDeckHTML(rootDir string) error {
	wwwDir := filepath.Join(rootDir, dirWWW)
	appDir := filepath.Join(rootDir, dirApp)
	outDir := filepath.Join(rootDir, dirDist, dirWWW)
	if err := os.MkdirAll(filepath.Join(outDir, dirFonts), 0755); err != nil {
		return fmt.Errorf(errMkdirFmt, outDir, err)
	}
	loadedTemplates, err := templates.LoadTemplatesFromDir(wwwDir, common.FS)
	if err != nil {
		return fmt.Errorf(errTplLoadFmt, err)
	}
	financeCtx := LoadDeckFinanceContext(rootDir)
	for _, name := range deckTemplateNames {
		tmpl, ok := loadedTemplates[name]
		if !ok {
			continue
		}
		htmlPath := filepath.Join(outDir, name+extHTML)
		htmlStr := templates.TranspileStaticHTMLWithContext(tmpl, loadedTemplates, financeCtx)
		relHTML := strings.ReplaceAll(htmlStr, srcAbsAppJS, srcRelAppJS)
		if err := os.WriteFile(htmlPath, []byte(relHTML), 0644); err != nil {
			return fmt.Errorf(errWriteFmt, htmlPath, err)
		}
	}
	styleCSSPath := filepath.Join(outDir, styleCSSFile)
	baseCSS, err := templates.BundleModulesCSS(bookplayeractions.ModuleFSList(), true)
	if err != nil {
		return fmt.Errorf(errCSSBundleFmt, err)
	}
	wwwStyleRaw, err := os.ReadFile(filepath.Join(wwwDir, styleTSSFile))
	if err != nil {
		return fmt.Errorf(errCSSBundleFmt, err)
	}
	wwwStyleCSS, err := templates.TranspileCSSWithSwitches(string(wwwStyleRaw), styleTSSFile, nil, templates.DefaultCSSSwitches(), true)
	if err != nil {
		return fmt.Errorf(errCSSBundleFmt, err)
	}
	relCSS := strings.ReplaceAll(baseCSS+"\n\n"+wwwStyleCSS, urlAbsFonts, urlRelFonts)
	if err := os.WriteFile(styleCSSPath, []byte(relCSS), 0644); err != nil {
		return err
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
	return nil
}

// FindHeadlessPDFBrowser locates an installed Chromium-based browser for PDF rendering.
func FindHeadlessPDFBrowser() string {
	var candidates []string
	switch runtime.GOOS {
	case osDarwin:
		candidates = darwinBrowserBins
	case osWindows:
		for _, base := range []string{os.Getenv(envProgramFiles), os.Getenv(envProgramFilesX86), os.Getenv(envLocalAppData)} {
			if base == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(base, winVendorGoogle, winAppChrome, winDirApp, winBinChrome),
				filepath.Join(base, winVendorMS, winAppEdge, winDirApp, winBinEdge),
				filepath.Join(base, winVendorBrave, winAppBrave, winDirApp, winBinBrave),
			)
		}
	default:
		for _, bin := range linuxBrowserBins {
			if p, err := exec.LookPath(bin); err == nil && p != "" {
				return p
			}
		}
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func renderHTMLFileToPDF(browserBin, htmlPath, outPDFPath string) error {
	absHTML, err := filepath.Abs(htmlPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absHTML); err != nil {
		return fmt.Errorf(errMissingHTMLFmt, absHTML, err)
	}
	absOutPDF, err := filepath.Abs(outPDFPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absOutPDF), 0755); err != nil {
		return err
	}
	tmpProfileDir, err := os.MkdirTemp("", tmpProfilePattern)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpProfileDir)

	fileURL := fileURLPrefix + filepath.ToSlash(absHTML)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		browserBin,
		argHeadless,
		argDisableGPU,
		argDisableExt,
		argDisableNet,
		argNoFirstRun,
		argNoDefBrowser,
		argUserDataPrefix+tmpProfileDir,
		argNoPDFHeader,
		argNoPrintHeader,
		argPrintPDFPrefix+absOutPDF,
		fileURL,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(errPDFRenderFmt, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// BuildDeckPDF renders dist/www/deck.html, dist/www/plan.html, and dist/www/przeglad.html
// into docs/business/mojra-deck-finansowy.pdf, docs/business/mojra-plan.pdf, and docs/business/mojra-przeglad-zalozyciela.pdf.
func BuildDeckPDF(rootDir, customOutPath string) (string, error) {
	browserBin := FindHeadlessPDFBrowser()
	if browserBin == "" {
		return "", fmt.Errorf(errNoBrowser)
	}
	docsDir := filepath.Join(rootDir, dirDocs)
	_ = os.MkdirAll(docsDir, 0755)

	outPDF := filepath.Join(docsDir, deckPDFFile)
	if strings.TrimSpace(customOutPath) != "" {
		outPDF = customOutPath
	}
	absOutPDF, err := filepath.Abs(outPDF)
	if err != nil {
		return "", err
	}
	deckHTMLPath := filepath.Join(rootDir, dirDist, dirWWW, deckHTMLFile)
	if err := renderHTMLFileToPDF(browserBin, deckHTMLPath, absOutPDF); err != nil {
		return "", err
	}
	if raw, rErr := os.ReadFile(absOutPDF); rErr == nil {
		if err := os.WriteFile(filepath.Join(rootDir, dirDist, dirWWW, deckPDFFile), raw, 0644); err != nil {
			return "", err
		}
	}
	planHTMLPath := filepath.Join(rootDir, dirDist, dirWWW, planHTMLFile)
	if _, err := os.Stat(planHTMLPath); err == nil {
		docsPlanPDF := filepath.Join(docsDir, planPDFFile)
		if err := renderHTMLFileToPDF(browserBin, planHTMLPath, docsPlanPDF); err == nil {
			if raw, rErr := os.ReadFile(docsPlanPDF); rErr == nil {
				if err := os.WriteFile(filepath.Join(rootDir, dirDist, dirWWW, planPDFFile), raw, 0644); err != nil {
					return "", err
				}
			}
		}
	}
	przegladHTMLPath := filepath.Join(rootDir, dirDist, dirWWW, przegladHTMLFile)
	if _, err := os.Stat(przegladHTMLPath); err == nil {
		docsPrzegladPDF := filepath.Join(docsDir, przegladPDFFile)
		if err := renderHTMLFileToPDF(browserBin, przegladHTMLPath, docsPrzegladPDF); err == nil {
			if raw, rErr := os.ReadFile(docsPrzegladPDF); rErr == nil {
				if err := os.WriteFile(filepath.Join(rootDir, dirDist, dirWWW, przegladPDFFile), raw, 0644); err != nil {
					return "", err
				}
			}
		}
	}
	return absOutPDF, nil
}
