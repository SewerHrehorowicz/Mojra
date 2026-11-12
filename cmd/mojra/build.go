package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"mojra/internal/book"
	"mojra/internal/tpl"
)

const (
	osDarwin   = "darwin"
	osWindows  = "windows"
	osLinux    = "linux"
	osAndroid  = "android"
	archAMD64  = "amd64"
	archARM64  = "arm64"
	arch386    = "386"
	specAll    = "all"
	envGOOS    = "GOOS"
	envGOARCH  = "GOARCH"
	binMojra   = "mojra"
	binMojraEx = "mojra.exe"
	binWinCap  = "Mojra.exe"

	ldflagProdBase    = "-X main.productionBuild=true"
	ldflagPreviewMode = "-X main.productionBuild=true -X main.previewBuild=true"
	ldflagStripBase   = "-ldflags=-s -w "
	ldflagStripWinGUI = "-ldflags=-s -w -H=windowsgui "

	goToolCmd        = "go"
	goSubcmdBuild    = "build"
	goTagProdEmbed   = "-tags=prod_embed"
	goFlagTrimpath   = "-trimpath"
	goFlagOut        = "-o"
	goPkgCmdMojra    = "./cmd/mojra"
	envCGOEnabled0   = "CGO_ENABLED=0"
	envCGOEnabled1   = "CGO_ENABLED=1"
	envGOOSPrefix    = "GOOS="
	envGOARCHPrefix  = "GOARCH="
	envCCPrefix      = "CC="
	envAndroidLDFlag = "CGO_LDFLAGS=-Wl,-z,max-page-size=16384"

	sysoDirSubpath  = "cmd"
	sysoPkgSubpath  = "mojra"
	sysoGlobPattern = "rsrc_windows_*.syso"
	sysoFileFormat  = "rsrc_windows_%s.syso"
	legacyDistGlob  = "StoryTeller-*"

	defaultBinFormat = "Mojra-%s-%s"

	errUnknownPlatformFmt = "nieznana platforma %q (dostępne: all, macos/darwin, windows, linux, android)"
	errUnknownArchFmt     = "nieznana architektura %q (dostępne: all, amd64, arm64)"
	errFlagNeedsValueFmt  = "flaga %s wymaga wartości"
	errUnknownFlagFmt     = "nieznana flaga: %s (użyj --help aby wyświetlić dostępne opcje)"
	errMissingIconFmt     = "brak pliku ikony %s: %w"
	errBuildAssetsFmt     = "błąd kompilacji .tpl/CSS/JS: %w"
	errWinSYSOFmt         = "błąd generowania zasobów PE (.syso) dla windows/%s: %w"
	errCompileTargetFmt   = "błąd kompilacji %s/%s: %w"
	errBuildArgsFmt       = "Błąd argumentów: %v\n\n"
	errBuildLocalBinFmt   = "Błąd budowania pliku wykonywalnego: %v\n"
	errBundleMacOSFmt     = "Błąd tworzenia paczki macOS: %v\n"
	errBundleWindowsFmt   = "Błąd tworzenia paczki Windows: %v\n"
	errBundleLinuxFmt     = "Błąd tworzenia paczki Linux: %v\n"
	errBundleAndroidFmt   = "Błąd tworzenia paczki Android (.apk): %v\n"

	msgVerifiedSampleBook = "✓ Zweryfikowano przykładową książkę -> books/%s/\n"
	msgCompiledTemplates  = "✓ Skompilowano app/templates/*.tpl -> build/app/index.html"
	msgBundledCSS         = "✓ Połączono arkusze CSS -> build/app/style.css"
	msgBundledJS          = "✓ Połączono moduły JS -> build/app/app.js"
	msgBuildBannerLine    = "=================================================================="
	msgBuildBannerTitle   = "  Mojra — Wieloplatformowy System Budowania"
	msgBuiltSingleLocal   = "✓ Zbudowano samodzielną aplikację (%s/%s): %s\n"
	msgBuiltTargetBin     = "✓ Zbudowano binarium [%s/%s]: %s\n"
	msgBuildDoneFmt       = "  Budowanie zakończone pomyślnie! Artefakty: %s\n"

	buildUsageHeader = "Mojra — Wieloplatformowy System Budowania (Multi-Platform Builder)\n\nUżycie / Usage:\n  ./build.sh [opcje] [platformy...]\n  build.bat [opcje] [platformy...]\n  go run ./cmd/mojra build [opcje] [platformy...]"
	buildFlagRowFmt  = "  %-26s %s"
)

type platformTarget struct {
	OS   string
	Arch string
}

type buildBinaryMode struct {
	Preview   bool
	Telemetry bool
}

type buildOptions struct {
	ShowHelp     bool
	AssetsOnly   bool
	BinariesOnly bool
	Preview      bool
	Telemetry    bool
	OutPath      string
	Platforms    []string
	Targets      []platformTarget
	SingleLocal  bool
}

type buildFlagKind uint8

const (
	flagKindNone buildFlagKind = iota
	flagKindHelp
	flagKindLocal
	flagKindPlatformConst
	flagKindAndroid
	flagKindPlatformList
	flagKindArchList
	flagKindOutput
	flagKindAssetsOnly
	flagKindPreview
	flagKindTelemetry
	flagKindBinariesOnly
	flagKindIgnore
)

type buildFlagDef struct {
	flags    []string
	argHint  string
	desc     string
	kind     buildFlagKind
	platform string
}

var (
	platformAliases = map[string][]string{
		specAll:     {osDarwin, osWindows, osLinux},
		"*":         {osDarwin, osWindows, osLinux},
		"macos":     {osDarwin},
		osDarwin:    {osDarwin},
		"mac":       {osDarwin},
		"osx":       {osDarwin},
		osWindows:   {osWindows},
		"win":       {osWindows},
		"win64":     {osWindows},
		"win32":     {osWindows},
		osLinux:     {osLinux},
		"gnu/linux": {osLinux},
		osAndroid:   {osAndroid},
		"apk":       {osAndroid},
	}

	archAliases = map[string][]string{
		specAll:   {archAMD64, archARM64},
		"*":       {archAMD64, archARM64},
		archAMD64: {archAMD64},
		"x86_64":  {archAMD64},
		"x64":     {archAMD64},
		archARM64: {archARM64},
		"aarch64": {archARM64},
	}

	platformBinFormat = map[string]string{
		osDarwin:  "Mojra-macos-%s",
		osWindows: "Mojra-windows-%s.exe",
		osLinux:   "Mojra-linux-%s",
		osAndroid: "Mojra-android-%s",
	}

	buildFlagDefs = []buildFlagDef{
		{
			flags:    []string{"-a", "--all"},
			desc:     "Buduj dla wszystkich platform: macOS, Windows, Linux (domyślne)",
			kind:     flagKindPlatformConst,
			platform: specAll,
		},
		{
			flags:    []string{"-m", "--macos", "--darwin", "--mac", "--osx"},
			desc:     "Buduj dla systemu macOS (.app, .zip oraz binaria arm64/amd64)",
			kind:     flagKindPlatformConst,
			platform: osDarwin,
		},
		{
			flags:    []string{"-w", "--windows", "--win"},
			desc:     "Buduj dla systemu Windows (.exe z ikoną, .zip, amd64/arm64)",
			kind:     flagKindPlatformConst,
			platform: osWindows,
		},
		{
			flags:    []string{"-l", "--linux"},
			desc:     "Buduj dla systemu Linux (binarium, .desktop, ikona .png, .zip)",
			kind:     flagKindPlatformConst,
			platform: osLinux,
		},
		{
			flags:    []string{"--android", "--apk", "-android"},
			desc:     "Buduj pakiet instalacyjny Android APK (dist/Mojra-android.apk, arm64/amd64)",
			kind:     flagKindAndroid,
			platform: osAndroid,
		},
		{
			flags: []string{"--local", "--host", "-local", "-host"},
			desc:  "Buduj wyłącznie pojedyncze binarium dla bieżącego systemu i architektury",
			kind:  flagKindLocal,
		},
		{
			flags:   []string{"-p", "--platform", "--platforms", "-platform", "-platforms"},
			argHint: "<lista>",
			desc:    "Lista platform lub celów os/arch oddzielona przecinkami",
			kind:    flagKindPlatformList,
		},
		{
			flags:   []string{"--arch", "-arch", "--architecture"},
			argHint: "<arch>",
			desc:    "Docelowa architektura: amd64 (x64), arm64 lub all (domyślnie: all)",
			kind:    flagKindArchList,
		},
		{
			flags:   []string{"-o", "--output", "-output"},
			argHint: "<ścieżka>",
			desc:    "Ścieżka wynikowego pliku wykonywalnego (dla pojedynczego binarium)",
			kind:    flagKindOutput,
		},
		{
			flags: []string{"--preview", "-preview", "--dev", "-dev"},
			desc:  "Buduj z aktywnym podglądem urządzeń (Telefon / Tablet / Desktop)",
			kind:  flagKindPreview,
		},
		{
			flags: []string{flagDashTelemetry, flagShortTelemetry},
			desc:  flagTelemetryDesc,
			kind:  flagKindTelemetry,
		},
		{
			flags: []string{"--binaries-only", "--no-zip", "--no-bundle", "-binaries-only", "-no-zip", "-no-bundle"},
			desc:  "Buduj wyłącznie pliki wykonywalne (pomiń paczki .app / .zip)",
			kind:  flagKindBinariesOnly,
		},
		{
			flags: []string{"--assets-only", "-assets-only"},
			desc:  "Kompiluj tylko szablony .tpl, CSS/JS oraz przykładową książkę",
			kind:  flagKindAssetsOnly,
		},
		{
			flags: []string{"-y", "--yes", "-yes", "--no-pause", "-no-pause"},
			desc:  "Automatycznie zaakceptuj instalację brakujących zależności w build.sh / build.bat",
			kind:  flagKindIgnore,
		},
		{
			flags: []string{"-h", "--help"},
			desc:  "Wyświetl tę pomoc i zakończ",
			kind:  flagKindHelp,
		},
	}

	positionalHelpArgs = map[string]bool{
		"help":  true,
		"/?":    true,
		"/h":    true,
		"/help": true,
	}

	positionalPreviewArgs = map[string]bool{
		"preview": true,
		"dev":     true,
	}

	staleDistEntries = []string{
		"Mojra.app",
		"Mojra-macos.zip",
		"Mojra-windows",
		"Mojra-windows.zip",
		"Mojra-linux",
		"Mojra-linux.zip",
		"Mojra-macos-arm64",
		"Mojra-macos-amd64",
		booksDirName,
		"saves",
	}
)

func lookupBuildFlagDef(key string) buildFlagDef {
	for _, def := range buildFlagDefs {
		for _, f := range def.flags {
			if f == key {
				return def
			}
		}
	}
	return buildFlagDef{}
}

func formatBuildUsage() string {
	lines := make([]string, 0, len(buildFlagDefs)+2)
	lines = append(lines, buildUsageHeader, "")
	for _, def := range buildFlagDefs {
		visible := def.flags
		if len(visible) > 2 {
			visible = visible[:2]
		}
		label := strings.Join(visible, ", ")
		if def.argHint != "" {
			label += " " + def.argHint
		}
		lines = append(lines, fmt.Sprintf(buildFlagRowFmt, label, def.desc))
	}
	return strings.Join(lines, "\n")
}

func normalizePlatformName(raw string) ([]string, error) {
	if list, ok := platformAliases[strings.ToLower(strings.TrimSpace(raw))]; ok {
		return list, nil
	}
	return nil, fmt.Errorf(errUnknownPlatformFmt, raw)
}

func normalizeArchName(raw string) ([]string, error) {
	if list, ok := archAliases[strings.ToLower(strings.TrimSpace(raw))]; ok {
		return list, nil
	}
	return nil, fmt.Errorf(errUnknownArchFmt, raw)
}

func splitCommaNonEmpty(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseBuildArgs(args []string) (buildOptions, error) {
	opts := buildOptions{}
	var rawPlatformSpecs []string
	var rawArchSpecs []string

	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}

		var key, val string
		hasEq := false
		if strings.HasPrefix(arg, "-") {
			if eqIdx := strings.IndexByte(arg, '='); eqIdx >= 0 {
				key = arg[:eqIdx]
				val = arg[eqIdx+1:]
				hasEq = true
			} else {
				key = arg
			}
		}

		nextVal := func(flagName string) (string, error) {
			if hasEq {
				if strings.TrimSpace(val) == "" {
					return "", fmt.Errorf(errFlagNeedsValueFmt, flagName)
				}
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf(errFlagNeedsValueFmt, flagName)
			}
			i++
			return args[i], nil
		}

		spec := lookupBuildFlagDef(key)
		switch spec.kind {
		case flagKindHelp:
			opts.ShowHelp = true
		case flagKindLocal:
			opts.SingleLocal = true
		case flagKindPlatformConst:
			rawPlatformSpecs = append(rawPlatformSpecs, spec.platform)
		case flagKindAndroid:
			rawPlatformSpecs = append(rawPlatformSpecs, spec.platform)
			opts.BinariesOnly = false
		case flagKindPlatformList:
			v, err := nextVal(key)
			if err != nil {
				return opts, err
			}
			rawPlatformSpecs = append(rawPlatformSpecs, splitCommaNonEmpty(v)...)
		case flagKindArchList:
			v, err := nextVal(key)
			if err != nil {
				return opts, err
			}
			rawArchSpecs = append(rawArchSpecs, splitCommaNonEmpty(v)...)
		case flagKindOutput:
			v, err := nextVal(key)
			if err != nil {
				return opts, err
			}
			opts.OutPath = v
		case flagKindAssetsOnly:
			opts.AssetsOnly = true
		case flagKindPreview:
			opts.Preview = true
		case flagKindTelemetry:
			opts.Telemetry = true
		case flagKindBinariesOnly:
			opts.BinariesOnly = true
		case flagKindIgnore:
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf(errUnknownFlagFmt, arg)
			}
			lower := strings.ToLower(arg)
			if positionalHelpArgs[lower] {
				opts.ShowHelp = true
				continue
			}
			if positionalPreviewArgs[lower] {
				opts.Preview = true
				continue
			}
			if lower == argTelemetry {
				opts.Telemetry = true
				continue
			}
			rawPlatformSpecs = append(rawPlatformSpecs, splitCommaNonEmpty(arg)...)
		}
	}

	if opts.ShowHelp || opts.AssetsOnly || opts.SingleLocal {
		return opts, nil
	}
	err := expandBuildTargets(&opts, rawPlatformSpecs, rawArchSpecs)
	return opts, err
}

func expandBuildTargets(opts *buildOptions, rawPlatformSpecs, rawArchSpecs []string) error {
	selectedArchs := []string{archAMD64, archARM64}
	if len(rawArchSpecs) > 0 {
		selectedArchs = nil
		seenArch := map[string]bool{}
		for _, spec := range rawArchSpecs {
			archs, err := normalizeArchName(spec)
			if err != nil {
				return err
			}
			for _, a := range archs {
				if !seenArch[a] {
					seenArch[a] = true
					selectedArchs = append(selectedArchs, a)
				}
			}
		}
	}

	if len(rawPlatformSpecs) == 0 {
		if opts.OutPath != "" && len(rawArchSpecs) == 0 {
			opts.SingleLocal = true
			return nil
		}
		rawPlatformSpecs = []string{specAll}
	}

	seenPlatform := map[string]bool{}
	seenTarget := map[string]bool{}
	addTarget := func(osName, archName string) {
		if !seenPlatform[osName] {
			seenPlatform[osName] = true
			opts.Platforms = append(opts.Platforms, osName)
		}
		key := osName + "/" + archName
		if !seenTarget[key] {
			seenTarget[key] = true
			opts.Targets = append(opts.Targets, platformTarget{OS: osName, Arch: archName})
		}
	}

	for _, spec := range rawPlatformSpecs {
		if slashIdx := strings.IndexByte(spec, '/'); slashIdx >= 0 {
			osList, err := normalizePlatformName(spec[:slashIdx])
			if err != nil {
				return err
			}
			archList, err := normalizeArchName(spec[slashIdx+1:])
			if err != nil {
				return err
			}
			for _, o := range osList {
				for _, a := range archList {
					addTarget(o, a)
				}
			}
			continue
		}

		osList, err := normalizePlatformName(spec)
		if err != nil {
			return err
		}
		for _, o := range osList {
			archsForOS := selectedArchs
			if o == osDarwin && len(selectedArchs) == 2 && selectedArchs[0] == archAMD64 && selectedArchs[1] == archARM64 && len(rawArchSpecs) == 0 {
				archsForOS = []string{archARM64, archAMD64}
			}
			for _, a := range archsForOS {
				addTarget(o, a)
			}
		}
	}
	return nil
}

func prepareBuildAssets(rootDir string) error {
	appDir := filepath.Join(rootDir, appDirName)
	booksDir := filepath.Join(rootDir, booksDirName)

	for _, iconFile := range []string{iconPNGFile, faviconICOFile} {
		iconPath := filepath.Join(appDir, iconsDirName, iconFile)
		if info, err := os.Stat(iconPath); err != nil || info.IsDir() || info.Size() == 0 {
			return fmt.Errorf(errMissingIconFmt, iconPath, err)
		}
	}
	if err := tpl.BuildApp(appDir); err != nil {
		return fmt.Errorf(errBuildAssetsFmt, err)
	}
	if entries, rErr := os.ReadDir(booksDir); rErr == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			sampleID := e.Name()
			sampleBookDir := filepath.Join(booksDir, sampleID)
			if !book.HasBookFilesInDir(sampleBookDir) {
				continue
			}
			_ = os.Remove(filepath.Join(booksDir, sampleID+playbookExt))
			fmt.Printf(msgVerifiedSampleBook, sampleID)
			break
		}
	}
	fmt.Println(msgCompiledTemplates)
	fmt.Println(msgBundledCSS)
	fmt.Println(msgBundledJS)
	return nil
}

func packageBooksForDist(srcBooksDir, dstBooksDir string) error {
	if err := os.MkdirAll(dstBooksDir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcBooksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	packedIDs := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dirPath := filepath.Join(srcBooksDir, e.Name())
		if !book.HasBookFilesInDir(dirPath) {
			continue
		}
		bookID := book.SanitizeBookID(e.Name())
		_ = os.RemoveAll(filepath.Join(dstBooksDir, e.Name()))
		outZip := filepath.Join(dstBooksDir, bookID+playbookExt)
		if err := book.PackBookToZip(dirPath, outZip); err != nil {
			return err
		}
		packedIDs[bookID] = true
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		lower := strings.ToLower(e.Name())
		if !strings.HasSuffix(lower, playbookExt) && !strings.HasSuffix(lower, zipExt) {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		bookID := book.SanitizeBookID(stem)
		if packedIDs[bookID] {
			continue
		}
		if err := copyFileWithMode(filepath.Join(srcBooksDir, e.Name()), filepath.Join(dstBooksDir, e.Name()), 0644); err != nil {
			return err
		}
	}
	return nil
}

func platformBinaryFilename(target platformTarget) string {
	if format, ok := platformBinFormat[target.OS]; ok {
		return fmt.Sprintf(format, target.Arch)
	}
	return fmt.Sprintf(defaultBinFormat, target.OS, target.Arch)
}

func buildPlatformBinary(rootDir string, target platformTarget, outPath string, mode buildBinaryMode) error {
	_ = os.MkdirAll(filepath.Dir(outPath), 0755)
	modeFlags := ldflagProdBase
	if mode.Preview {
		modeFlags = ldflagPreviewMode
	}
	if mode.Telemetry {
		modeFlags += ldflagTelemetryEnable
	}

	switch target.OS {
	case osWindows:
		if matches, err := filepath.Glob(filepath.Join(rootDir, sysoDirSubpath, sysoPkgSubpath, sysoGlobPattern)); err == nil {
			for _, m := range matches {
				_ = os.Remove(m)
			}
		}
		icoPath := filepath.Join(rootDir, appDirName, iconsDirName, faviconICOFile)
		sysoPath := filepath.Join(rootDir, sysoDirSubpath, sysoPkgSubpath, fmt.Sprintf(sysoFileFormat, target.Arch))
		if err := writeWindowsResourceSYSOForArch(icoPath, sysoPath, target.Arch); err != nil {
			return fmt.Errorf(errWinSYSOFmt, target.Arch, err)
		}
		defer func() {
			_ = os.Remove(sysoPath)
		}()

		cmd := exec.Command(goToolCmd, goSubcmdBuild, goTagProdEmbed, goFlagTrimpath, ldflagStripWinGUI+modeFlags, goFlagOut, outPath, goPkgCmdMojra)
		cmd.Dir = rootDir
		cmd.Env = append(os.Environ(), envGOOSPrefix+osWindows, envGOARCHPrefix+target.Arch, envCGOEnabled0)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(errCompileTargetFmt, osWindows, target.Arch, err)
		}
		return nil

	case osDarwin:
		if runtime.GOOS == osDarwin {
			cmd := exec.Command(goToolCmd, goSubcmdBuild, goTagProdEmbed, goFlagTrimpath, ldflagStripBase+modeFlags, goFlagOut, outPath, goPkgCmdMojra)
			cmd.Dir = rootDir
			cmd.Env = append(os.Environ(), envGOOSPrefix+osDarwin, envGOARCHPrefix+target.Arch, envCGOEnabled1)
			var stderr bytes.Buffer
			cmd.Stdout = os.Stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		cmd := exec.Command(goToolCmd, goSubcmdBuild, goTagProdEmbed, goFlagTrimpath, ldflagStripBase+modeFlags, goFlagOut, outPath, goPkgCmdMojra)
		cmd.Dir = rootDir
		cmd.Env = append(os.Environ(), envGOOSPrefix+osDarwin, envGOARCHPrefix+target.Arch, envCGOEnabled0)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(errCompileTargetFmt, osDarwin, target.Arch, err)
		}
		return nil

	case osAndroid:
		ndkClang, err := findAndroidNDKClang(target.Arch)
		if err != nil {
			return err
		}
		cmd := exec.Command(goToolCmd, goSubcmdBuild, goTagProdEmbed, goFlagTrimpath, ldflagStripBase+modeFlags, goFlagOut, outPath, goPkgCmdMojra)
		cmd.Dir = rootDir
		cmd.Env = append(os.Environ(),
			envGOOSPrefix+osAndroid,
			envGOARCHPrefix+target.Arch,
			envCGOEnabled1,
			envCCPrefix+ndkClang,
			envAndroidLDFlag,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(errCompileTargetFmt, osAndroid, target.Arch, err)
		}
		return nil

	default:
		cmd := exec.Command(goToolCmd, goSubcmdBuild, goTagProdEmbed, goFlagTrimpath, ldflagStripBase+modeFlags, goFlagOut, outPath, goPkgCmdMojra)
		cmd.Dir = rootDir
		cmd.Env = append(os.Environ(), envGOOSPrefix+target.OS, envGOARCHPrefix+target.Arch, envCGOEnabled0)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(errCompileTargetFmt, target.OS, target.Arch, err)
		}
		return nil
	}
}

func buildCmd(args []string) {
	opts, err := parseBuildArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, errBuildArgsFmt, err)
		fmt.Println(formatBuildUsage())
		os.Exit(1)
	}
	if opts.ShowHelp {
		fmt.Println(formatBuildUsage())
		return
	}

	rootDir := resolveRepoRootDir()
	fmt.Println(msgBuildBannerLine)
	fmt.Println(msgBuildBannerTitle)
	fmt.Println(msgBuildBannerLine)

	if err := prepareBuildAssets(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if opts.AssetsOnly {
		return
	}

	mode := buildBinaryMode{Preview: opts.Preview, Telemetry: opts.Telemetry}
	if opts.SingleLocal {
		targetOS := runtime.GOOS
		if envOS := os.Getenv(envGOOS); envOS != "" {
			targetOS = envOS
		}
		targetArch := runtime.GOARCH
		if envArch := os.Getenv(envGOARCH); envArch != "" {
			targetArch = envArch
		}
		targetBin := opts.OutPath
		if targetBin == "" {
			targetBin = binMojra
			if targetOS == osWindows {
				targetBin = binMojraEx
			}
		}
		if !filepath.IsAbs(targetBin) {
			targetBin = filepath.Join(rootDir, targetBin)
		}
		if err := buildPlatformBinary(rootDir, platformTarget{OS: targetOS, Arch: targetArch}, targetBin, mode); err != nil {
			fmt.Fprintf(os.Stderr, errBuildLocalBinFmt, err)
			os.Exit(1)
		}
		fmt.Printf(msgBuiltSingleLocal, targetOS, targetArch, targetBin)
		return
	}

	distDir := filepath.Join(rootDir, distDirName)
	_ = os.MkdirAll(distDir, 0755)
	for _, stale := range staleDistEntries {
		_ = os.RemoveAll(filepath.Join(distDir, stale))
	}
	if legacyMatches, err := filepath.Glob(filepath.Join(distDir, legacyDistGlob)); err == nil {
		for _, m := range legacyMatches {
			_ = os.RemoveAll(m)
		}
	}

	builtByOS := map[string]map[string]string{}
	for _, target := range opts.Targets {
		outPath := filepath.Join(distDir, platformBinaryFilename(target))
		if len(opts.Targets) == 1 && opts.OutPath != "" {
			if filepath.IsAbs(opts.OutPath) {
				outPath = opts.OutPath
			} else {
				outPath = filepath.Join(rootDir, opts.OutPath)
			}
		}
		if err := buildPlatformBinary(rootDir, target, outPath, mode); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		if target.OS == osDarwin {
			adhocSignMacOSPath(outPath)
		}
		if builtByOS[target.OS] == nil {
			builtByOS[target.OS] = map[string]string{}
		}
		builtByOS[target.OS][target.Arch] = outPath
		fmt.Printf(msgBuiltTargetBin, target.OS, target.Arch, outPath)
	}

	if hostMap := builtByOS[runtime.GOOS]; hostMap != nil {
		if hostBin := hostMap[runtime.GOARCH]; hostBin != "" {
			rootBinName := binMojra
			if runtime.GOOS == osWindows {
				rootBinName = binWinCap
			}
			rootBin := filepath.Join(rootDir, rootBinName)
			_ = copyFileWithMode(hostBin, rootBin, 0755)
			if runtime.GOOS == osDarwin {
				adhocSignMacOSPath(rootBin)
			}
		}
	}

	for _, p := range opts.Platforms {
		if p == osAndroid {
			opts.BinariesOnly = false
		}
	}
	if !opts.BinariesOnly {
		for _, osName := range opts.Platforms {
			archMap := builtByOS[osName]
			switch osName {
			case osDarwin:
				if err := packageMacOSBundle(rootDir, archMap); err != nil {
					fmt.Fprintf(os.Stderr, errBundleMacOSFmt, err)
					os.Exit(1)
				}
			case osWindows:
				if err := packageWindowsBundle(rootDir, archMap); err != nil {
					fmt.Fprintf(os.Stderr, errBundleWindowsFmt, err)
					os.Exit(1)
				}
			case osLinux:
				if err := packageLinuxBundle(rootDir, archMap); err != nil {
					fmt.Fprintf(os.Stderr, errBundleLinuxFmt, err)
					os.Exit(1)
				}
			case osAndroid:
				if err := packageAndroidBundle(rootDir, archMap); err != nil {
					fmt.Fprintf(os.Stderr, errBundleAndroidFmt, err)
					os.Exit(1)
				}
			}
		}
	}

	fmt.Println(msgBuildBannerLine)
	fmt.Printf(msgBuildDoneFmt, distDir)
	fmt.Println(msgBuildBannerLine)
}
