package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mojra"
	"mojra/internal/audit"
	"mojra/internal/book"
	"mojra/internal/logging"
	"mojra/internal/osbridge"
	"mojra/internal/server"
)

const (
	flagTelemetryName     = "telemetry"
	flagTelemetryDesc     = "Zapisuj pomiary czasu startu i przełączania widoków do saves/telemetry.log"
	flagDashTelemetry     = "--telemetry"
	flagShortTelemetry    = "-telemetry"
	argTelemetry          = "telemetry"
	ldflagTelemetryEnable = " -X mojra/internal/logging.buildEnabled=true"

	cmdDefault     = "run"
	cmdBuild       = "build"
	cmdPack        = "pack"
	cmdAdd         = "add"
	cmdImport      = "import"
	cmdAudit       = "audit"
	cmdLint        = "lint"
	goBuildMarker  = "/go-build"
	goModFileName  = "go.mod"
	booksDirName   = "books"
	distDirName    = "dist"
	appDirName     = "app"
	iconsDirName   = "icons"
	iconPNGFile    = "icon.png"
	faviconICOFile = "favicon.ico"
	resourcesDir   = "Resources"
	parentDirRel   = ".."
	currentDirRel  = "."
	playbookExt    = ".playbook"
	zipExt         = ".zip"
	httpPrefix     = "http://"
	httpsPrefix    = "https://"
	boolTrueStr    = "true"
	envOneStr      = "1"
	envSTDev       = "ST_DEV"
	envSTPreview   = "ST_PREVIEW"
	envBetaKey     = "MOJRA_BETA_KEY"

	flagRunPortName     = "port"
	flagRunPortDesc     = "Port lokalny i sieciowy LAN"
	flagRunHeadlessName = "headless"
	flagRunHeadlessDesc = "Uruchom bez otwierania natywnego okna (tryb testowy/serwerowy)"
	flagRunDevName      = "dev"
	flagRunDevDesc      = "Uruchom w trybie deweloperskim (z paskiem podglądu urządzeń)"
	flagRunPreviewName  = "preview"
	flagRunPreviewDesc  = "Uruchom z paskiem podglądu urządzeń (Telefon / Tablet / Desktop)"
	flagRunProdName     = "prod"
	flagRunProdDesc     = "Wymuś tryb produkcyjny (bez narzędzi deweloperskich)"

	flagAuditStaticName   = "static"
	flagAuditStaticDesc   = "Uruchom wyłącznie twardą analizę statyczną (-> logs/audit_static.log)"
	flagAuditArchName     = "arch"
	flagAuditArchDesc     = "Uruchom doradczy audyt architektury i długu technicznego (-> logs/audit_arch.log)"
	flagAuditMagicName    = "magic"
	flagAuditMagicDesc    = "Uruchom doradczy audyt magic stringów w metodach (-> logs/audit_strings.log)"
	flagAuditStringsName  = "strings"
	flagAuditStringsDesc  = "Alias dla --magic (-> logs/audit_strings.log)"
	flagAuditCategoryName = "category"
	flagAuditCategoryDesc = "Filtruj po kategorii (np. DEAD_CODE, ARCH_*, MAGIC_*)"
	flagAuditSeverityName = "severity"
	flagAuditSeverityDesc = "Filtruj po ważności: error, warning, info"
	flagAuditLangName     = "lang"
	flagAuditLangDesc     = "Filtruj po języku: js, go, tpl, tss"
	flagAuditJSName       = "js"
	flagAuditJSDesc       = "Skanuj i wyświetl wyłącznie wyniki dla plików JavaScript (app/js/*.js)"
	flagAuditGoName       = "go"
	flagAuditGoDesc       = "Skanuj i wyświetl wyłącznie wyniki dla plików Go (*.go)"
	flagAuditTplName      = "tpl"
	flagAuditTplDesc      = "Skanuj i wyświetl wyłącznie wyniki dla szablonów (*.tpl / *.tss)"
	flagAuditTSSName      = "tss"
	flagAuditTSSDesc      = "Skanuj i wyświetl wyłącznie wyniki dla arkuszy stylów (*.tss / *.css)"
	flagAuditOutName      = "out"
	flagAuditOutDesc      = "Opcjonalna ścieżka zapisu raportu do pliku"
	flagAuditStrictName   = "strict"
	flagAuditStrictDesc   = "Zwróć kod wyjścia 1 jeśli wykryto problemy w analizie statycznej"

	langJS  = "js"
	langGo  = "go"
	langTpl = "tpl"
	langTSS = "tss"

	auditStaticLogFile  = "audit_static.log"
	auditArchLogFile    = "audit_arch.log"
	auditStringsLogFile = "audit_strings.log"

	usageHeader  = "Mojra — Interaktywny Czytnik & Playbook Studio\n\nUżycie / Usage:"
	usageLineFmt = "  mojra %s\n      %s"
	pipeSep      = " | "

	msgUnknownCommand   = "Nieznana komenda: %s\n\n"
	msgBetaAuthErr      = "Błąd autoryzacji buildu Closed Beta: %v\n"
	msgLaunchErr        = "Błąd uruchomienia Mojra: %v\n"
	msgPackMissingArg   = "Podaj identyfikator książki, np.: mojra pack <book-id>"
	msgPackErr          = "Błąd pakowania .playbook: %v\n"
	msgPackSaved        = "✓ Spakowano książkę do archiwum: %s\n"
	msgAddMissingArg    = "Podaj ścieżkę na dysku lub adres URL, np.: mojra add /ścieżka/do/książki lub mojra add https://example.com/ksiazka.playbook"
	msgAddDownloadErr   = "Błąd pobierania książki z URL (%s): %v\n"
	msgSaveSettingsErr  = "Błąd zapisu ustawień: %v\n"
	msgAddDownloaded    = "✓ Pobrano i dodano książkę do biblioteki: %s (%s)\n"
	msgAddDiskErr       = "Błąd dodawania książki z dysku (%s): %v\n"
	msgAddDiskSuccess   = "✓ Dodano książkę z dysku (bez kopiowania): %s -> %s\n"
	msgAuditStaticErr   = "Błąd audytu statycznego: %v\n"
	msgAuditStaticSaved = "\n✓ Zapisano raport statyczny: %s\n"
	msgAuditArchErr     = "Błąd audytu architektury: %v\n"
	msgAuditArchSaved   = "✓ Zapisano raport architektury: %s\n"
	msgAuditMagicErr    = "Błąd audytu magic stringów: %v\n"
	msgAuditMagicSaved  = "✓ Zapisano raport magic stringów: %s\n"
)

type cliCommandDef struct {
	names          []string
	argHint        string
	desc           string
	bundlePlatform string
	isRun          bool
	forceDev       bool
}

var (
	// productionBuild is set to "true" via -ldflags="-X main.productionBuild=true" during production builds.
	productionBuild string

	// previewBuild is set to "true" via -ldflags="-X main.previewBuild=true" during preview builds (--preview).
	previewBuild string

	// betaExpiryRFC3339 and betaPubKeyHex can be injected via -ldflags during Closed Beta builds.
	betaExpiryRFC3339 string
	betaPubKeyHex     string

	cliCommandDefs = []cliCommandDef{
		{
			names:   []string{cmdDefault, "start", "gui"},
			argHint: "[--port 8484] [--headless]",
			desc:    "Uruchamia samodzielną aplikację okienkową (natywne okno systemowe).",
			isRun:   true,
		},
		{
			names:    []string{flagRunDevName, flagRunPreviewName},
			isRun:    true,
			forceDev: true,
		},
		{
			names:   []string{cmdBuild},
			argHint: "[opcje] [platformy...]",
			desc:    "Kompiluje szablony .tpl -> app/index.html, łączy CSS i JS oraz buduje pliki wykonywalne i paczki dystrybucyjne.",
		},
		{
			names:          []string{"bundle-all", "bundle"},
			desc:           "Buduje binaria oraz paczki dystrybucyjne dla wszystkich platform (macOS, Windows, Linux).",
			bundlePlatform: specAll,
		},
		{
			names:          []string{"bundle-app", "bundle-macos"},
			desc:           "Tworzy gotową paczkę aplikacji natywnej (dist/Mojra.app oraz dist/Mojra-macos.zip).",
			bundlePlatform: osDarwin,
		},
		{
			names:          []string{"bundle-windows"},
			desc:           "Buduje samodzielną paczkę wykonywalną dla systemu Windows (dist/Mojra-windows/ + .zip).",
			bundlePlatform: osWindows,
		},
		{
			names:          []string{"bundle-linux"},
			desc:           "Buduje samodzielną paczkę wykonywalną dla systemu Linux (dist/Mojra-linux/ + .zip).",
			bundlePlatform: osLinux,
		},
		{
			names:          []string{"bundle-android"},
			desc:           "Buduje podpisany pakiet instalacyjny Android APK (dist/Mojra-android.apk) z binariami arm64/amd64.",
			bundlePlatform: osAndroid,
		},
		{
			names:   []string{cmdPack},
			argHint: "<id-książki>",
			desc:    "Pakuje folder książki z katalogu books/<id> do archiwum dist/<id>.playbook.",
		},
		{
			names:   []string{cmdAdd, cmdImport},
			argHint: "<ścieżka-na-dysku | url>",
			desc:    "Dodaje książkę do biblioteki z dysku (bez kopiowania plików) lub pobiera z adresu URL (.playbook / .zip).",
		},
		{
			names:   []string{cmdAudit, cmdLint},
			argHint: "[--js | --go | --tpl | --lang <język>] [--category <kategoria>] [--out <plik.log>] [--strict]",
			desc:    "Uruchamia statyczny audyt kodu Go, szablonów .tpl i skryptów .js.",
		},
	}

	helpSubcommands = map[string]bool{
		"help":   true,
		"-h":     true,
		"--help": true,
	}

	auditLangAliases = map[string]string{
		"js":         langJS,
		"javascript": langJS,
		"go":         langGo,
		"golang":     langGo,
		"tpl":        langTpl,
		"tss":        langTSS,
		"css":        langTSS,
	}

	auditSevAliases = map[string]bool{
		"error":   true,
		"warning": true,
		"warn":    true,
		"info":    true,
	}
)

func formatCLIUsage() string {
	lines := make([]string, 0, len(cliCommandDefs)+1)
	lines = append(lines, usageHeader)
	for _, def := range cliCommandDefs {
		if def.desc == "" {
			continue
		}
		synopsis := strings.Join(def.names, pipeSep)
		if def.argHint != "" {
			synopsis += " " + def.argHint
		}
		lines = append(lines, fmt.Sprintf(usageLineFmt, synopsis, def.desc))
	}
	return strings.Join(lines, "\n\n")
}

func findCLICommandDef(cmd string) (cliCommandDef, bool) {
	for _, def := range cliCommandDefs {
		for _, name := range def.names {
			if name == cmd {
				return def, true
			}
		}
	}
	return cliCommandDef{}, false
}

func isGoRunBinary() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(filepath.ToSlash(exe), goBuildMarker)
}

func main() {
	cmd := cmdDefault
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = strings.ToLower(strings.TrimSpace(args[0]))
		args = args[1:]
	}

	if helpSubcommands[cmd] {
		fmt.Println(formatCLIUsage())
		return
	}
	if def, ok := findCLICommandDef(cmd); ok {
		if def.isRun {
			runCmd(args, def.forceDev)
			return
		}
		if def.bundlePlatform != "" {
			buildCmd(append([]string{def.bundlePlatform}, args...))
			return
		}
		switch def.names[0] {
		case cmdBuild:
			buildCmd(args)
		case cmdPack:
			packCmd(args)
		case cmdAdd:
			addCmd(args)
		case cmdAudit:
			auditCmd(args)
		}
		return
	}

	fmt.Fprintf(os.Stderr, msgUnknownCommand, cmd)
	fmt.Println(formatCLIUsage())
	os.Exit(1)
}

func isRepoDir(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, goModFileName))
	return err == nil && !info.IsDir()
}

func resolveDistRuntimeDir(_ string) string {
	runtimeDir := book.DefaultAppDataDir()
	runtimeBooks := filepath.Join(runtimeDir, booksDirName)
	_ = os.MkdirAll(runtimeBooks, 0755)
	return runtimeDir
}

func resolveRepoRootDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = currentDirRel
	}
	if isRepoDir(cwd) {
		return cwd
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if isRepoDir(exeDir) {
			return exeDir
		}
		parentDir := filepath.Clean(filepath.Join(exeDir, parentDirRel))
		if isRepoDir(parentDir) {
			return parentDir
		}
	}
	return cwd
}

func resolveRootDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = currentDirRel
	}
	if !isGoRunBinary() {
		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			bundleRepoCandidate := filepath.Clean(filepath.Join(exeDir, parentDirRel, parentDirRel, parentDirRel, parentDirRel))
			if isRepoDir(bundleRepoCandidate) {
				return resolveDistRuntimeDir(bundleRepoCandidate)
			}
			resDir := filepath.Clean(filepath.Join(exeDir, parentDirRel, resourcesDir))
			if info, err := os.Stat(filepath.Join(resDir, booksDirName)); err == nil && info.IsDir() {
				return resDir
			}
			if isRepoDir(exeDir) {
				return resolveDistRuntimeDir(exeDir)
			}
			parentDir := filepath.Clean(filepath.Join(exeDir, parentDirRel))
			if isRepoDir(parentDir) {
				return resolveDistRuntimeDir(parentDir)
			}
			if info, err := os.Stat(filepath.Join(exeDir, booksDirName)); err == nil && info.IsDir() {
				return exeDir
			}
			if info, err := os.Stat(filepath.Join(parentDir, booksDirName)); err == nil && info.IsDir() {
				return parentDir
			}
		}
		if isRepoDir(cwd) {
			return resolveDistRuntimeDir(cwd)
		}
	}
	return cwd
}

func runCmd(args []string, forceDev bool) {
	t0 := time.Now()
	fs := flag.NewFlagSet(cmdDefault, flag.ExitOnError)
	port := fs.Int(flagRunPortName, 8484, flagRunPortDesc)
	headless := fs.Bool(flagRunHeadlessName, false, flagRunHeadlessDesc)
	devFlag := fs.Bool(flagRunDevName, false, flagRunDevDesc)
	previewFlag := fs.Bool(flagRunPreviewName, false, flagRunPreviewDesc)
	prodFlag := fs.Bool(flagRunProdName, false, flagRunProdDesc)
	telemetryFlag := fs.Bool(flagTelemetryName, false, flagTelemetryDesc)
	_ = fs.Parse(args)

	devMode := false
	if !*prodFlag {
		if forceDev || *devFlag || *previewFlag || os.Getenv(envSTDev) == envOneStr || os.Getenv(envSTPreview) == envOneStr || previewBuild == boolTrueStr || (productionBuild != boolTrueStr && isGoRunBinary()) {
			devMode = true
		}
	}

	rootDir := resolveRootDir()
	logging.Init(book.ResolveSavesDir(rootDir), t0, *telemetryFlag)
	logging.Active.MarkStep(logging.StageResolveRoot, t0, rootDir)
	if err := book.CheckBetaBuildExpiry(betaExpiryRFC3339, os.Getenv(envBetaKey), betaPubKeyHex, time.Time{}); err != nil {
		fmt.Fprintf(os.Stderr, msgBetaAuthErr, err)
		os.Exit(1)
	}
	srv := server.NewServerWithOptions(rootDir, mojra.AppFS, devMode)
	if err := osbridge.LaunchWithOptions(rootDir, *port, *headless, devMode, srv.Handler(), srv.State.Config.ShutdownCh, srv.State.Config.TriggerShutdown, func(p int) { srv.State.Config.Port = p }); err != nil {
		fmt.Fprintf(os.Stderr, msgLaunchErr, err)
		os.Exit(1)
	}
}

func packCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, msgPackMissingArg)
		os.Exit(1)
	}
	bookID := book.SanitizeBookID(args[0])
	rootDir := resolveRepoRootDir()
	srcDir := filepath.Join(rootDir, booksDirName, bookID)
	distDir := filepath.Join(rootDir, distDirName)
	_ = os.MkdirAll(distDir, 0755)
	outZip := filepath.Join(distDir, bookID+playbookExt)

	if err := book.PackBookToZip(srcDir, outZip); err != nil {
		fmt.Fprintf(os.Stderr, msgPackErr, err)
		os.Exit(1)
	}
	fmt.Printf(msgPackSaved, outZip)
}

func addCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, msgAddMissingArg)
		os.Exit(1)
	}
	rootDir := resolveRootDir()
	booksDir := book.DefaultProjectsDir()
	savesDir := book.ResolveSavesDir(rootDir)
	settings := book.LoadAppSettings(savesDir)

	for _, target := range args {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, httpPrefix) || strings.HasPrefix(lower, httpsPrefix) {
			b, err := book.DownloadBookFromURL(booksDir, trimmed)
			if err != nil {
				fmt.Fprintf(os.Stderr, msgAddDownloadErr, trimmed, err)
				os.Exit(1)
			}
			settings.AddExternalBook(b.RootPath)
			settings.ActiveBookID = b.ID
			if err := book.SaveAppSettings(savesDir, settings); err != nil {
				fmt.Fprintf(os.Stderr, msgSaveSettingsErr, err)
			}
			fmt.Printf(msgAddDownloaded, b.Title, b.RootPath)
			continue
		}

		resolved, err := book.ResolveDiskBookPaths(trimmed)
		if err != nil {
			fmt.Fprintf(os.Stderr, msgAddDiskErr, trimmed, err)
			os.Exit(1)
		}
		for _, p := range resolved {
			if filepath.Clean(filepath.Dir(p)) != filepath.Clean(booksDir) {
				settings.AddExternalBook(p)
			}
			if b, err := book.LoadBookFromPath(p); err == nil && b != nil {
				settings.ActiveBookID = b.ID
				fmt.Printf(msgAddDiskSuccess, b.Title, p)
			}
		}
		if err := book.SaveAppSettings(savesDir, settings); err != nil {
			fmt.Fprintf(os.Stderr, msgSaveSettingsErr, err)
		}
	}
}

func saveAuditReportOutput(rootDir, defaultLogFile, outPath, savedFmt string, printToStdout, forceDefaultLog bool, data []byte) {
	if printToStdout {
		os.Stdout.Write(data)
	}
	if outPath == "" || forceDefaultLog {
		if p, err := audit.WriteAuditLog(rootDir, defaultLogFile, data); err == nil {
			fmt.Printf(savedFmt, p)
		}
	} else if err := os.WriteFile(outPath, data, 0644); err == nil {
		fmt.Printf(savedFmt, outPath)
	}
}

func auditCmd(args []string) {
	fs := flag.NewFlagSet(cmdAudit, flag.ExitOnError)
	staticMode := fs.Bool(flagAuditStaticName, false, flagAuditStaticDesc)
	archMode := fs.Bool(flagAuditArchName, false, flagAuditArchDesc)
	magicMode := fs.Bool(flagAuditMagicName, false, flagAuditMagicDesc)
	stringsMode := fs.Bool(flagAuditStringsName, false, flagAuditStringsDesc)
	category := fs.String(flagAuditCategoryName, "", flagAuditCategoryDesc)
	severity := fs.String(flagAuditSeverityName, "", flagAuditSeverityDesc)
	lang := fs.String(flagAuditLangName, "", flagAuditLangDesc)
	onlyJS := fs.Bool(flagAuditJSName, false, flagAuditJSDesc)
	onlyGo := fs.Bool(flagAuditGoName, false, flagAuditGoDesc)
	onlyTpl := fs.Bool(flagAuditTplName, false, flagAuditTplDesc)
	onlyTSS := fs.Bool(flagAuditTSSName, false, flagAuditTSSDesc)
	outPath := fs.String(flagAuditOutName, "", flagAuditOutDesc)
	strict := fs.Bool(flagAuditStrictName, false, flagAuditStrictDesc)
	_ = fs.Parse(args)

	filterLang := strings.ToLower(strings.TrimSpace(*lang))
	langFlags := []struct {
		enabled bool
		code    string
	}{
		{*onlyJS, langJS},
		{*onlyGo, langGo},
		{*onlyTSS, langTSS},
		{*onlyTpl, langTpl},
	}
	for _, lf := range langFlags {
		if lf.enabled {
			filterLang = lf.code
			break
		}
	}

	filter := *category
	filterSev := strings.TrimSpace(*severity)
	for _, posArg := range fs.Args() {
		lower := strings.ToLower(strings.TrimSpace(posArg))
		if filterLang == "" && auditLangAliases[lower] != "" {
			filterLang = lower
		} else if filterSev == "" && auditSevAliases[lower] {
			filterSev = lower
		} else if filter == "" {
			filter = posArg
		}
	}

	isMagic := *magicMode || *stringsMode
	runAll := !*staticMode && !*archMode && !isMagic
	rootDir := resolveRepoRootDir()
	staticCount := 0

	if *staticMode || runAll {
		findings, err := audit.Run(rootDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, msgAuditStaticErr, err)
			os.Exit(1)
		}
		var buf bytes.Buffer
		staticCount = audit.PrintReportWithOptions(&buf, findings, filter, filterLang, filterSev)
		saveAuditReportOutput(rootDir, auditStaticLogFile, *outPath, msgAuditStaticSaved, true, false, buf.Bytes())
	}

	if *archMode || runAll {
		archFindings, err := audit.RunArch(rootDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, msgAuditArchErr, err)
			os.Exit(1)
		}
		var buf bytes.Buffer
		audit.PrintArchReportWithLang(&buf, archFindings, filter, filterLang, filterSev)
		saveAuditReportOutput(rootDir, auditArchLogFile, *outPath, msgAuditArchSaved, *archMode, runAll, buf.Bytes())
	}

	if isMagic || runAll {
		magicFindings, err := audit.RunMagic(rootDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, msgAuditMagicErr, err)
			os.Exit(1)
		}
		var buf bytes.Buffer
		audit.PrintMagicReportWithLang(&buf, magicFindings, filter, filterLang, filterSev)
		saveAuditReportOutput(rootDir, auditStringsLogFile, *outPath, msgAuditMagicSaved, isMagic, runAll, buf.Bytes())
	}

	if (*staticMode || *strict) && staticCount > 0 {
		os.Exit(1)
	}
}
