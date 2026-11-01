package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"storyteller"
	"storyteller/internal/book"
	"storyteller/internal/gui"
	"storyteller/internal/tpl"
)

// productionBuild is set to "true" via -ldflags="-X main.productionBuild=true" during production builds.
var productionBuild string

func isGoRunBinary() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(filepath.ToSlash(exe), "/go-build")
}

func main() {
	cmd := "run"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = strings.ToLower(strings.TrimSpace(args[0]))
		args = args[1:]
	}

	switch cmd {
	case "run", "start", "gui":
		runCmd(args, false)
	case "dev":
		runCmd(args, true)
	case "build":
		buildCmd(args)
	case "bundle", "bundle-all":
		buildCmd(append([]string{"--all"}, args...))
	case "bundle-app", "bundle-macos":
		bundleMacOSCmd(args)
	case "bundle-windows":
		bundleWindowsCmd(args)
	case "bundle-linux":
		bundleLinuxCmd(args)
	case "bundle-android":
		bundleAndroidCmd(args)
	case "pack":
		packCmd(args)
	case "add", "import":
		addCmd(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Nieznana komenda: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`StoryTeller — Interaktywny Odtwarzacz Audiobooków & Studio

Użycie / Usage:
  storyteller [run] [--port 8484] [--headless]
      Uruchamia samodzielną aplikację okienkową (natywne okno systemowe).

  storyteller build [opcje] [platformy...]
      Kompiluje szablony .tpl -> app/index.html, łączy CSS i JS oraz buduje
      pliki wykonywalne i paczki dystrybucyjne dla wybranych lub wszystkich platform.
      Uruchom 'storyteller build --help' lub './build.sh --help' po pełną listę flag.

  storyteller bundle-all
      Buduje binaria oraz paczki dystrybucyjne dla wszystkich platform (macOS, Windows, Linux).

  storyteller bundle-app | bundle-macos
      Tworzy gotową paczkę aplikacji natywnej (dist/StoryTeller.app oraz dist/StoryTeller-macos.zip)
      wraz z binariami dla macOS (arm64/amd64) i przykładową książką.

  storyteller bundle-windows
      Buduje samodzielną paczkę wykonywalną dla systemu Windows (dist/StoryTeller-windows/ + .zip).

  storyteller bundle-linux
      Buduje samodzielną paczkę wykonywalną dla systemu Linux (dist/StoryTeller-linux/ + .zip).

  storyteller pack <id-książki>
      Pakuje folder książki z katalogu books/<id> do archiwum dist/<id>.stbook.

  storyteller add <ścieżka-na-dysku | url>
      Dodaje książkę do biblioteki z dysku (bez kopiowania plików) lub pobiera z adresu URL (.stbook / .zip).`)
}

func printBuildUsage() {
	fmt.Println(`StoryTeller — Wieloplatformowy System Budowania (Multi-Platform Builder)

Użycie / Usage:
  ./build.sh [opcje] [platformy...]
  build.bat [opcje] [platformy...]
  go run ./cmd/storyteller build [opcje] [platformy...]

Platformy (Platforms):
  -a, --all                  Buduj dla wszystkich platform: macOS, Windows, Linux (domyślne)
  -m, --macos, --darwin      Buduj dla systemu macOS (.app, .zip oraz binaria arm64/amd64)
  -w, --windows, --win       Buduj dla systemu Windows (.exe z ikoną, .zip, amd64/arm64)
  -l, --linux                Buduj dla systemu Linux (binarium, .desktop, ikona .png, .zip)
      --local, --host        Buduj wyłącznie pojedyncze binarium dla bieżącego systemu i architektury
  -p, --platform <lista>     Lista platform lub celów os/arch oddzielona przecinkami
                             (np. macos,windows lub darwin/arm64,windows/amd64,linux/amd64)

Architektura i wyjście (Architecture & Output):
      --arch <arch>          Docelowa architektura: amd64 (x64), arm64 lub all (domyślnie: all)
  -o, --output <ścieżka>     Ścieżka wynikowego pliku wykonywalnego (dla pojedynczego binarium)
      --binaries-only        Buduj wyłącznie pliki wykonywalne (pomiń paczki .app / .zip)
      --no-zip               Alias dla --binaries-only
      --assets-only          Kompiluj tylko szablony .tpl, CSS/JS oraz przykładową książkę
  -y, --yes                  Automatycznie zaakceptuj instalację brakujących zależności w build.sh / build.bat
  -h, --help, /?             Wyświetl tę pomoc i zakończ (Show this help message)

Argumenty pozycyjne (Positional arguments):
  Możesz również podać nazwy platform bezpośrednio jako argumenty:
  all, macos, darwin, windows, win, linux, darwin/arm64, darwin/amd64,
  windows/amd64, windows/arm64, linux/amd64, linux/arm64

Przykłady (Examples):
  ./build.sh                                    # Buduje wszystko (macOS, Windows, Linux — amd64 + arm64)
  build.bat                                     # Buduje wszystko na Windowsie
  ./build.sh --macos                            # Tylko macOS (.app + binaria + .zip)
  build.bat --windows --linux                   # Windows oraz Linux
  ./build.sh windows linux                      # Windows oraz Linux (argumenty pozycyjne)
  ./build.sh -p windows/amd64,linux/amd64       # Wybrane kombinacje system/architektura
  ./build.sh --all --arch amd64                 # Wszystkie platformy tylko dla x86_64 (amd64)
  ./build.sh --binaries-only --linux            # Same pliki binarne dla Linuxa bez archiwum ZIP
  ./build.sh --help                             # Wyświetla pomoc`)
}

func isRepoDir(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil && !info.IsDir()
}

func copyDirRecursivePreserve(srcDir, dstDir string) {
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return
	}
	_ = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil || rel == "." {
			return nil
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			_ = os.MkdirAll(target, 0755)
			return nil
		}
		srcInfo, err := d.Info()
		if err != nil {
			return nil
		}
		if dstInfo, err := os.Stat(target); err == nil && !dstInfo.IsDir() {
			if !srcInfo.ModTime().After(dstInfo.ModTime()) {
				return nil
			}
		}
		_ = copyFileWithMode(path, target, srcInfo.Mode().Perm())
		return nil
	})
}

func preserveDistRuntimeState(distDir string) {
	distBooks := filepath.Join(distDir, "books")
	distSaves := filepath.Join(distDir, "saves")
	_ = os.MkdirAll(distBooks, 0755)
	_ = os.MkdirAll(distSaves, 0755)
	for _, candidate := range []string{
		filepath.Join(distDir, "StoryTeller.app", "Contents", "Resources"),
		filepath.Join(distDir, "StoryTeller-windows"),
		filepath.Join(distDir, "StoryTeller-linux"),
	} {
		copyDirRecursivePreserve(filepath.Join(candidate, "books"), distBooks)
		copyDirRecursivePreserve(filepath.Join(candidate, "saves"), distSaves)
	}
	repoDir := filepath.Clean(filepath.Join(distDir, ".."))
	if isRepoDir(repoDir) {
		repoBooks := filepath.Join(repoDir, "books")
		repoSaves := filepath.Join(repoDir, "saves")
		copyDirRecursivePreserve(repoSaves, distSaves)
		copyDirRecursivePreserve(distSaves, repoSaves)
		if entries, err := os.ReadDir(distBooks); err == nil {
			for _, e := range entries {
				if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "nie-boj-sie-policji-rzecznej" {
					continue
				}
				srcBookDir := filepath.Join(distBooks, e.Name())
				if book.HasBookFilesInDir(srcBookDir) {
					copyDirRecursivePreserve(srcBookDir, filepath.Join(repoBooks, e.Name()))
				}
			}
		}
	}
}

func resolveDistRuntimeDir(repoDir string) string {
	distDir := filepath.Join(repoDir, "dist")
	preserveDistRuntimeState(distDir)
	distBooks := filepath.Join(distDir, "books")
	if info, err := os.Stat(distBooks); err != nil || !info.IsDir() {
		_ = packageBooksForDist(filepath.Join(repoDir, "books"), distBooks)
	} else if entries, err := os.ReadDir(distBooks); err == nil && len(entries) == 0 {
		_ = packageBooksForDist(filepath.Join(repoDir, "books"), distBooks)
	}
	return distDir
}

func resolveRepoRootDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	if isRepoDir(cwd) {
		return cwd
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if isRepoDir(exeDir) {
			return exeDir
		}
		parentDir := filepath.Clean(filepath.Join(exeDir, ".."))
		if isRepoDir(parentDir) {
			return parentDir
		}
	}
	return cwd
}

func resolveRootDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	if !isGoRunBinary() {
		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			bundleRepoCandidate := filepath.Clean(filepath.Join(exeDir, "..", "..", "..", ".."))
			if isRepoDir(bundleRepoCandidate) {
				return resolveDistRuntimeDir(bundleRepoCandidate)
			}
			resDir := filepath.Clean(filepath.Join(exeDir, "..", "Resources"))
			if info, err := os.Stat(filepath.Join(resDir, "books")); err == nil && info.IsDir() {
				return resDir
			}
			if isRepoDir(exeDir) {
				return resolveDistRuntimeDir(exeDir)
			}
			parentDir := filepath.Clean(filepath.Join(exeDir, ".."))
			if isRepoDir(parentDir) {
				return resolveDistRuntimeDir(parentDir)
			}
			if info, err := os.Stat(filepath.Join(exeDir, "books")); err == nil && info.IsDir() {
				return exeDir
			}
			if info, err := os.Stat(filepath.Join(parentDir, "books")); err == nil && info.IsDir() {
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
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	port := fs.Int("port", 8484, "Port lokalny i sieciowy LAN")
	headless := fs.Bool("headless", false, "Uruchom bez otwierania natywnego okna (tryb testowy/serwerowy)")
	devFlag := fs.Bool("dev", false, "Uruchom w trybie deweloperskim (z paskiem podglądu urządzeń)")
	prodFlag := fs.Bool("prod", false, "Wymuś tryb produkcyjny (bez narzędzi deweloperskich)")
	_ = fs.Parse(args)

	devMode := false
	if !*prodFlag {
		if forceDev || *devFlag || os.Getenv("ST_DEV") == "1" || (productionBuild != "true" && isGoRunBinary()) {
			devMode = true
		}
	}

	rootDir := resolveRootDir()
	appDir := filepath.Join(rootDir, "app")
	if info, err := os.Stat(filepath.Join(appDir, "templates")); err == nil && info.IsDir() {
		if err := tpl.BuildApp(appDir); err != nil {
			fmt.Fprintf(os.Stderr, "Błąd kompilacji szablonów .tpl: %v\n", err)
			os.Exit(1)
		}
	}

	if err := gui.LaunchWithOptions(rootDir, storyteller.AppFS, *port, *headless, devMode); err != nil {
		fmt.Fprintf(os.Stderr, "Błąd uruchomienia StoryTeller: %v\n", err)
		os.Exit(1)
	}
}

type platformTarget struct {
	OS   string // "darwin", "windows", "linux"
	Arch string // "amd64", "arm64"
}

type buildOptions struct {
	ShowHelp     bool
	AssetsOnly   bool
	BinariesOnly bool
	OutPath      string
	Platforms    []string         // ordered unique OS list: "darwin", "windows", "linux"
	Targets      []platformTarget // ordered unique (OS, Arch) pairs
	SingleLocal  bool             // true when no platform args were provided
}

func normalizePlatformName(raw string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "all", "*":
		return []string{"darwin", "windows", "linux"}, nil
	case "macos", "darwin", "mac", "osx":
		return []string{"darwin"}, nil
	case "windows", "win", "win64", "win32":
		return []string{"windows"}, nil
	case "linux", "gnu/linux":
		return []string{"linux"}, nil
	case "android", "apk":
		return []string{"android"}, nil
	default:
		return nil, fmt.Errorf("nieznana platforma %q (dostępne: all, macos/darwin, windows, linux, android)", raw)
	}
}

func normalizeArchName(raw string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "all", "*":
		return []string{"amd64", "arm64"}, nil
	case "amd64", "x86_64", "x64":
		return []string{"amd64"}, nil
	case "arm64", "aarch64":
		return []string{"arm64"}, nil
	default:
		return nil, fmt.Errorf("nieznana architektura %q (dostępne: all, amd64, arm64)", raw)
	}
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

		// Handle --key=value syntax
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
					return "", fmt.Errorf("flaga %s wymaga wartości", flagName)
				}
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("flaga %s wymaga wartości", flagName)
			}
			i++
			return args[i], nil
		}

		switch key {
		case "-h", "--help":
			opts.ShowHelp = true
		case "--local", "-local", "--host", "-host":
			opts.SingleLocal = true
		case "-a", "--all":
			rawPlatformSpecs = append(rawPlatformSpecs, "all")
		case "-m", "--macos", "--darwin", "--mac", "--osx":
			rawPlatformSpecs = append(rawPlatformSpecs, "darwin")
		case "-w", "--windows", "--win":
			rawPlatformSpecs = append(rawPlatformSpecs, "windows")
		case "-l", "--linux":
			rawPlatformSpecs = append(rawPlatformSpecs, "linux")
		case "--android", "-android", "--apk":
			rawPlatformSpecs = append(rawPlatformSpecs, "android")
			opts.BinariesOnly = false
		case "-p", "--platform", "--platforms", "-platform", "-platforms":
			v, err := nextVal(key)
			if err != nil {
				return opts, err
			}
			for _, part := range strings.Split(v, ",") {
				if p := strings.TrimSpace(part); p != "" {
					rawPlatformSpecs = append(rawPlatformSpecs, p)
				}
			}
		case "--arch", "-arch", "--architecture":
			v, err := nextVal(key)
			if err != nil {
				return opts, err
			}
			for _, part := range strings.Split(v, ",") {
				if a := strings.TrimSpace(part); a != "" {
					rawArchSpecs = append(rawArchSpecs, a)
				}
			}
		case "-o", "--output", "-output":
			v, err := nextVal(key)
			if err != nil {
				return opts, err
			}
			opts.OutPath = v
		case "--assets-only", "-assets-only":
			opts.AssetsOnly = true
		case "--binaries-only", "-binaries-only", "--no-zip", "-no-zip", "--no-bundle", "-no-bundle":
			opts.BinariesOnly = true
		case "--no-pause", "-no-pause", "-y", "--yes", "-yes":
			// Ignored flags used by build wrapper scripts (build.bat / build.sh)
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("nieznana flaga: %s (użyj --help aby wyświetlić dostępne opcje)", arg)
			}
			if strings.EqualFold(arg, "help") || arg == "/?" || strings.EqualFold(arg, "/h") || strings.EqualFold(arg, "/help") {
				opts.ShowHelp = true
				continue
			}
			for _, part := range strings.Split(arg, ",") {
				if p := strings.TrimSpace(part); p != "" {
					rawPlatformSpecs = append(rawPlatformSpecs, p)
				}
			}
		}
	}

	if opts.ShowHelp || opts.AssetsOnly || opts.SingleLocal {
		return opts, nil
	}

	// Resolve global arch filter (default: amd64 + arm64)
	defaultArchs := []string{"amd64", "arm64"}
	var selectedArchs []string
	seenArch := map[string]bool{}
	if len(rawArchSpecs) > 0 {
		for _, spec := range rawArchSpecs {
			archs, err := normalizeArchName(spec)
			if err != nil {
				return opts, err
			}
			for _, a := range archs {
				if !seenArch[a] {
					seenArch[a] = true
					selectedArchs = append(selectedArchs, a)
				}
			}
		}
	} else {
		selectedArchs = defaultArchs
	}

	if len(rawPlatformSpecs) == 0 {
		if opts.OutPath != "" && len(rawArchSpecs) == 0 {
			opts.SingleLocal = true
			return opts, nil
		}
		// Default to building all platforms when no platform argument was provided
		rawPlatformSpecs = []string{"all"}
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
			osPart := spec[:slashIdx]
			archPart := spec[slashIdx+1:]
			osList, err := normalizePlatformName(osPart)
			if err != nil {
				return opts, err
			}
			archList, err := normalizeArchName(archPart)
			if err != nil {
				return opts, err
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
			return opts, err
		}
		for _, o := range osList {
			// On macOS, list arm64 first then amd64 if both are selected
			archsForOS := selectedArchs
			if o == "darwin" && len(selectedArchs) == 2 && selectedArchs[0] == "amd64" && selectedArchs[1] == "arm64" && len(rawArchSpecs) == 0 {
				archsForOS = []string{"arm64", "amd64"}
			}
			for _, a := range archsForOS {
				addTarget(o, a)
			}
		}
	}

	return opts, nil
}

func prepareBuildAssets(rootDir string) error {
	appDir := filepath.Join(rootDir, "app")
	booksDir := filepath.Join(rootDir, "books")

	if err := writeWindowsAppIconICO(filepath.Join(appDir, "icons", "favicon.ico")); err != nil {
		return fmt.Errorf("błąd generowania favicon.ico: %w", err)
	}
	if png256, err := encodeStoryTellerIconPNG(256); err == nil {
		_ = os.WriteFile(filepath.Join(appDir, "icons", "icon.png"), png256, 0644)
	}
	if err := tpl.BuildApp(appDir); err != nil {
		return fmt.Errorf("błąd kompilacji .tpl/CSS/JS: %w", err)
	}
	_ = os.Remove(filepath.Join(booksDir, "nie-boj-sie-policji-rzecznej.stbook"))
	fmt.Println("✓ Skompilowano app/templates/*.tpl -> build/app/index.html")
	fmt.Println("✓ Połączono arkusze CSS -> build/app/style.css")
	fmt.Println("✓ Połączono moduły JS -> build/app/app.js")
	fmt.Println("✓ Zweryfikowano przykładową książkę -> books/nie-boj-sie-policji-rzecznej/")
	return nil
}

// packageBooksForDist packs unpacked source book directories from srcBooksDir into .stbook archives
// inside dstBooksDir so built bundles only ship read-only .stbook packages and never copy raw source folders.
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
		outZip := filepath.Join(dstBooksDir, bookID+".stbook")
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
		if !strings.HasSuffix(lower, ".stbook") && !strings.HasSuffix(lower, ".zip") {
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
	switch target.OS {
	case "darwin":
		return fmt.Sprintf("StoryTeller-macos-%s", target.Arch)
	case "windows":
		return fmt.Sprintf("StoryTeller-windows-%s.exe", target.Arch)
	case "linux":
		return fmt.Sprintf("StoryTeller-linux-%s", target.Arch)
	case "android":
		return fmt.Sprintf("StoryTeller-android-%s", target.Arch)
	default:
		return fmt.Sprintf("StoryTeller-%s-%s", target.OS, target.Arch)
	}
}

func buildPlatformBinary(rootDir string, target platformTarget, outPath string) error {
	_ = os.MkdirAll(filepath.Dir(outPath), 0755)

	switch target.OS {
	case "windows":
		// Clean up any stale .syso files before writing the target architecture .syso
		if matches, err := filepath.Glob(filepath.Join(rootDir, "cmd", "storyteller", "rsrc_windows_*.syso")); err == nil {
			for _, m := range matches {
				_ = os.Remove(m)
			}
		}
		sysoPath := filepath.Join(rootDir, "cmd", "storyteller", fmt.Sprintf("rsrc_windows_%s.syso", target.Arch))
		if err := writeWindowsResourceSYSOForArch(sysoPath, target.Arch); err != nil {
			return fmt.Errorf("błąd generowania zasobów PE (.syso) dla windows/%s: %w", target.Arch, err)
		}
		defer func() {
			_ = os.Remove(sysoPath)
		}()

		cmd := exec.Command("go", "build", "-tags=prod_embed", "-trimpath", "-ldflags=-H=windowsgui -X main.productionBuild=true", "-o", outPath, "./cmd/storyteller")
		cmd.Dir = rootDir
		cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+target.Arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("błąd kompilacji windows/%s: %w", target.Arch, err)
		}
		return nil

	case "darwin":
		// On macOS host, prefer CGO_ENABLED=1 so native Cocoa/WKWebView window support is compiled in.
		if runtime.GOOS == "darwin" {
			cmd := exec.Command("go", "build", "-tags=prod_embed", "-trimpath", "-ldflags=-X main.productionBuild=true", "-o", outPath, "./cmd/storyteller")
			cmd.Dir = rootDir
			cmd.Env = append(os.Environ(), "GOOS=darwin", "GOARCH="+target.Arch, "CGO_ENABLED=1")
			var stderr bytes.Buffer
			cmd.Stdout = os.Stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		cmd := exec.Command("go", "build", "-tags=prod_embed", "-trimpath", "-ldflags=-s -w -X main.productionBuild=true", "-o", outPath, "./cmd/storyteller")
		cmd.Dir = rootDir
		cmd.Env = append(os.Environ(), "GOOS=darwin", "GOARCH="+target.Arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("błąd kompilacji darwin/%s: %w", target.Arch, err)
		}
		return nil

	case "android":
		ndkClang, err := findAndroidNDKClang(target.Arch)
		if err != nil {
			return err
		}
		cmd := exec.Command("go", "build", "-tags=prod_embed", "-trimpath", "-ldflags=-s -w -X main.productionBuild=true", "-o", outPath, "./cmd/storyteller")
		cmd.Dir = rootDir
		cmd.Env = append(os.Environ(), "GOOS=android", "GOARCH="+target.Arch, "CGO_ENABLED=1", "CC="+ndkClang)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("błąd kompilacji android/%s: %w", target.Arch, err)
		}
		return nil

	default: // linux and others
		cmd := exec.Command("go", "build", "-tags=prod_embed", "-trimpath", "-ldflags=-s -w -X main.productionBuild=true", "-o", outPath, "./cmd/storyteller")
		cmd.Dir = rootDir
		cmd.Env = append(os.Environ(), "GOOS="+target.OS, "GOARCH="+target.Arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("błąd kompilacji %s/%s: %w", target.OS, target.Arch, err)
		}
		return nil
	}
}

func buildCmd(args []string) {
	opts, err := parseBuildArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Błąd argumentów: %v\n\n", err)
		printBuildUsage()
		os.Exit(1)
	}
	if opts.ShowHelp {
		printBuildUsage()
		return
	}

	rootDir := resolveRepoRootDir()
	fmt.Println("==================================================================")
	fmt.Println("  StoryTeller — Wieloplatformowy System Budowania")
	fmt.Println("==================================================================")

	if err := prepareBuildAssets(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if opts.AssetsOnly {
		return
	}

	// Single local binary build when invoked with --local / --host or -o <path>
	if opts.SingleLocal {
		targetOS := runtime.GOOS
		if envOS := os.Getenv("GOOS"); envOS != "" {
			targetOS = envOS
		}
		targetArch := runtime.GOARCH
		if envArch := os.Getenv("GOARCH"); envArch != "" {
			targetArch = envArch
		}
		targetBin := opts.OutPath
		if targetBin == "" {
			targetBin = "storyteller"
			if targetOS == "windows" {
				targetBin = "storyteller.exe"
			}
		}
		if !filepath.IsAbs(targetBin) {
			targetBin = filepath.Join(rootDir, targetBin)
		}
		if err := buildPlatformBinary(rootDir, platformTarget{OS: targetOS, Arch: targetArch}, targetBin); err != nil {
			fmt.Fprintf(os.Stderr, "Błąd budowania pliku wykonywalnego: %v\n", err)
			os.Exit(1)
		}
		_ = packageBooksForDist(filepath.Join(rootDir, "books"), filepath.Join(rootDir, "dist", "books"))
		fmt.Printf("✓ Zbudowano samodzielną aplikację (%s/%s): %s\n", targetOS, targetArch, targetBin)
		return
	}

	distDir := filepath.Join(rootDir, "dist")
	_ = os.MkdirAll(distDir, 0755)
	preserveDistRuntimeState(distDir)
	_ = packageBooksForDist(filepath.Join(rootDir, "books"), filepath.Join(distDir, "books"))
	for _, stale := range []string{
		"StoryTeller.app",
		"StoryTeller-macos.zip",
		"StoryTeller-windows",
		"StoryTeller-windows.zip",
		"StoryTeller-linux",
		"StoryTeller-linux.zip",
	} {
		_ = os.RemoveAll(filepath.Join(distDir, stale))
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
		if err := buildPlatformBinary(rootDir, target, outPath); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		if target.OS == "darwin" {
			adhocSignMacOSPath(outPath)
		}
		if builtByOS[target.OS] == nil {
			builtByOS[target.OS] = map[string]string{}
		}
		builtByOS[target.OS][target.Arch] = outPath
		fmt.Printf("✓ Zbudowano binarium [%s/%s]: %s\n", target.OS, target.Arch, outPath)
	}

	if hostMap := builtByOS[runtime.GOOS]; hostMap != nil {
		if hostBin := hostMap[runtime.GOARCH]; hostBin != "" {
			rootBinName := "storyteller"
			if runtime.GOOS == "windows" {
				rootBinName = "StoryTeller.exe"
			}
			rootBin := filepath.Join(rootDir, rootBinName)
			_ = copyFileWithMode(hostBin, rootBin, 0755)
			if runtime.GOOS == "darwin" {
				adhocSignMacOSPath(rootBin)
			}
		}
	}

	for _, p := range opts.Platforms {
		if p == "android" {
			opts.BinariesOnly = false
		}
	}
	if !opts.BinariesOnly {
		for _, osName := range opts.Platforms {
			archMap := builtByOS[osName]
			switch osName {
			case "darwin":
				if err := packageMacOSBundle(rootDir, archMap); err != nil {
					fmt.Fprintf(os.Stderr, "Błąd tworzenia paczki macOS: %v\n", err)
					os.Exit(1)
				}
			case "windows":
				if err := packageWindowsBundle(rootDir, archMap); err != nil {
					fmt.Fprintf(os.Stderr, "Błąd tworzenia paczki Windows: %v\n", err)
					os.Exit(1)
				}
			case "linux":
				if err := packageLinuxBundle(rootDir, archMap); err != nil {
					fmt.Fprintf(os.Stderr, "Błąd tworzenia paczki Linux: %v\n", err)
					os.Exit(1)
				}
			case "android":
				if err := packageAndroidBundle(rootDir, archMap); err != nil {
					fmt.Fprintf(os.Stderr, "Błąd tworzenia paczki Android (.apk): %v\n", err)
					os.Exit(1)
				}
			}
		}
	}

	fmt.Println("==================================================================")
	fmt.Printf("  Budowanie zakończone pomyślnie! Artefakty: %s\n", distDir)
	fmt.Println("==================================================================")
}

func copyFileWithMode(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Remove the destination file first (or write via temp + rename) so macOS kernel
	// allocates a fresh inode and does not kill the binary due to cached vnode code-signing hashes.
	_ = os.Remove(dst)
	tmpDst := dst + ".tmp"
	_ = os.Remove(tmpDst)
	out, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpDst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpDst)
		return err
	}
	return os.Rename(tmpDst, dst)
}

func adhocSignMacOSPath(targetPath string) {
	if runtime.GOOS != "darwin" {
		return
	}
	if codesignPath, err := exec.LookPath("codesign"); err == nil && codesignPath != "" {
		_ = exec.Command(codesignPath, "--force", "--sign", "-", targetPath).Run()
	}
}

func packageMacOSBundle(rootDir string, archBins map[string]string) error {
	bundleRoot := filepath.Join(rootDir, "dist", "StoryTeller.app")
	contentsDir := filepath.Join(bundleRoot, "Contents")
	macOSDir := filepath.Join(contentsDir, "MacOS")
	resourcesDir := filepath.Join(contentsDir, "Resources")

	_ = os.RemoveAll(bundleRoot)
	_ = os.MkdirAll(macOSDir, 0755)
	_ = os.MkdirAll(resourcesDir, 0755)

	binPath := filepath.Join(macOSDir, "storyteller")
	arm64Bin := archBins["arm64"]
	amd64Bin := archBins["amd64"]

	createdUniversal := false
	if arm64Bin != "" && amd64Bin != "" && runtime.GOOS == "darwin" {
		if lipoPath, err := exec.LookPath("lipo"); err == nil && lipoPath != "" {
			cmd := exec.Command(lipoPath, "-create", "-output", binPath, arm64Bin, amd64Bin)
			if err := cmd.Run(); err == nil {
				_ = os.Chmod(binPath, 0755)
				createdUniversal = true
			}
		}
	}

	if !createdUniversal {
		chosen := archBins[runtime.GOARCH]
		if chosen == "" {
			if arm64Bin != "" {
				chosen = arm64Bin
			} else {
				chosen = amd64Bin
			}
		}
		if chosen == "" {
			if err := buildPlatformBinary(rootDir, platformTarget{OS: "darwin", Arch: "arm64"}, binPath); err != nil {
				return err
			}
		} else {
			if err := copyFileWithMode(chosen, binPath, 0755); err != nil {
				return err
			}
		}
	}

	// Keep ./storyteller in the project root updated when building on macOS
	if runtime.GOOS == "darwin" {
		hostBin := archBins[runtime.GOARCH]
		if hostBin == "" {
			hostBin = binPath
		}
		rootBin := filepath.Join(rootDir, "storyteller")
		_ = copyFileWithMode(hostBin, rootBin, 0755)
		adhocSignMacOSPath(rootBin)
	}

	infoPlist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>StoryTeller</string>
	<key>CFBundleDisplayName</key>
	<string>StoryTeller</string>
	<key>CFBundleIdentifier</key>
	<string>pl.storyteller.app</string>
	<key>CFBundleVersion</key>
	<string>1.0.0</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleExecutable</key>
	<string>storyteller</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(contentsDir, "Info.plist"), []byte(infoPlist), 0644); err != nil {
		return err
	}
	if err := writeMacOSAppIconICNS(filepath.Join(resourcesDir, "AppIcon.icns")); err != nil {
		return err
	}
	if err := packageBooksForDist(filepath.Join(rootDir, "books"), filepath.Join(resourcesDir, "books")); err != nil {
		return err
	}
	adhocSignMacOSPath(binPath)
	adhocSignMacOSPath(bundleRoot)

	zipPath := filepath.Join(rootDir, "dist", "StoryTeller-macos.zip")
	if err := zipDirRecursive(bundleRoot, zipPath, "StoryTeller.app"); err != nil {
		return err
	}

	fmt.Printf("✓ Utworzono natywną aplikację macOS: %s\n", bundleRoot)
	fmt.Printf("✓ Utworzono archiwum dystrybucyjne macOS: %s\n", zipPath)
	return nil
}

func packageWindowsBundle(rootDir string, archBins map[string]string) error {
	booksDir := filepath.Join(rootDir, "books")
	outDir := filepath.Join(rootDir, "dist", "StoryTeller-windows")
	_ = os.RemoveAll(outDir)
	_ = os.MkdirAll(outDir, 0755)
	_ = os.MkdirAll(filepath.Join(outDir, "saves"), 0755)

	icoPath := filepath.Join(outDir, "StoryTeller.ico")
	if err := writeWindowsAppIconICO(icoPath); err != nil {
		return fmt.Errorf("błąd generowania ikony Windows (.ico): %w", err)
	}

	exePath := filepath.Join(outDir, "StoryTeller.exe")
	primaryBin := archBins["amd64"]
	if primaryBin == "" {
		primaryBin = archBins["arm64"]
	}
	if primaryBin != "" {
		if err := copyFileWithMode(primaryBin, exePath, 0755); err != nil {
			return err
		}
	} else {
		if err := buildPlatformBinary(rootDir, platformTarget{OS: "windows", Arch: "amd64"}, exePath); err != nil {
			return err
		}
	}

	if arm64Bin := archBins["arm64"]; arm64Bin != "" && archBins["amd64"] != "" {
		_ = copyFileWithMode(arm64Bin, filepath.Join(outDir, "StoryTeller-arm64.exe"), 0755)
	}

	if runtime.GOOS == "windows" {
		_ = copyFileWithMode(exePath, filepath.Join(rootDir, "StoryTeller.exe"), 0755)
		_ = copyFileWithMode(icoPath, filepath.Join(rootDir, "StoryTeller.ico"), 0644)
	}

	if err := packageBooksForDist(booksDir, filepath.Join(outDir, "books")); err != nil {
		return err
	}

	launcherBat := "@echo off\r\ncd /d \"%~dp0\"\r\nstart \"\" \"%~dp0StoryTeller.exe\"\r\n"
	if err := os.WriteFile(filepath.Join(outDir, "Uruchom StoryTeller.bat"), []byte(launcherBat), 0644); err != nil {
		return err
	}

	zipPath := filepath.Join(rootDir, "dist", "StoryTeller-windows.zip")
	if err := zipDirRecursive(outDir, zipPath, "StoryTeller-windows"); err != nil {
		return fmt.Errorf("błąd tworzenia archiwum ZIP dla Windows: %w", err)
	}

	fmt.Printf("✓ Utworzono aplikację Windows z ikoną: %s\n", exePath)
	fmt.Printf("✓ Utworzono ikonę aplikacji Windows: %s\n", icoPath)
	fmt.Printf("✓ Utworzono archiwum dystrybucyjne Windows: %s\n", zipPath)
	return nil
}

func writeLinuxAppIconPNG(outPath string) error {
	pngData, err := encodeStoryTellerIconPNG(512)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, pngData, 0644)
}

func packageLinuxBundle(rootDir string, archBins map[string]string) error {
	booksDir := filepath.Join(rootDir, "books")
	outDir := filepath.Join(rootDir, "dist", "StoryTeller-linux")
	_ = os.RemoveAll(outDir)
	_ = os.MkdirAll(outDir, 0755)
	_ = os.MkdirAll(filepath.Join(outDir, "saves"), 0755)

	iconPath := filepath.Join(outDir, "StoryTeller.png")
	if err := writeLinuxAppIconPNG(iconPath); err != nil {
		return fmt.Errorf("błąd generowania ikony Linux (.png): %w", err)
	}

	binPath := filepath.Join(outDir, "storyteller")
	primaryBin := archBins["amd64"]
	if primaryBin == "" {
		primaryBin = archBins["arm64"]
	}
	if primaryBin != "" {
		if err := copyFileWithMode(primaryBin, binPath, 0755); err != nil {
			return err
		}
	} else {
		if err := buildPlatformBinary(rootDir, platformTarget{OS: "linux", Arch: "amd64"}, binPath); err != nil {
			return err
		}
	}

	if arm64Bin := archBins["arm64"]; arm64Bin != "" && archBins["amd64"] != "" {
		_ = copyFileWithMode(arm64Bin, filepath.Join(outDir, "storyteller-arm64"), 0755)
	}

	if runtime.GOOS == "linux" {
		hostBin := archBins[runtime.GOARCH]
		if hostBin == "" {
			hostBin = binPath
		}
		_ = copyFileWithMode(hostBin, filepath.Join(rootDir, "storyteller"), 0755)
	}

	if err := packageBooksForDist(booksDir, filepath.Join(outDir, "books")); err != nil {
		return err
	}

	launcherSh := "#!/usr/bin/env bash\nset -euo pipefail\nDIR=\"$(cd \"$(dirname \"${BASH_SOURCE[0]}\")\" && pwd)\"\ncd \"$DIR\"\nexec \"$DIR/storyteller\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(outDir, "Uruchom StoryTeller.sh"), []byte(launcherSh), 0755); err != nil {
		return err
	}

	desktopEntry := `[Desktop Entry]
Type=Application
Name=StoryTeller
Comment=Interaktywny Odtwarzacz Audiobooków & Studio
Exec=./storyteller
Icon=StoryTeller
Terminal=false
Categories=AudioVideo;Audio;Player;
`
	if err := os.WriteFile(filepath.Join(outDir, "StoryTeller.desktop"), []byte(desktopEntry), 0755); err != nil {
		return err
	}

	zipPath := filepath.Join(rootDir, "dist", "StoryTeller-linux.zip")
	if err := zipDirRecursive(outDir, zipPath, "StoryTeller-linux"); err != nil {
		return fmt.Errorf("błąd tworzenia archiwum ZIP dla Linux: %w", err)
	}

	fmt.Printf("✓ Utworzono aplikację Linux: %s\n", binPath)
	fmt.Printf("✓ Utworzono ikonę aplikacji Linux: %s\n", iconPath)
	fmt.Printf("✓ Utworzono archiwum dystrybucyjne Linux: %s\n", zipPath)
	return nil
}

func bundleMacOSCmd(args []string) {
	buildCmd(append([]string{"--macos"}, args...))
}

func bundleWindowsCmd(args []string) {
	buildCmd(append([]string{"--windows"}, args...))
}

func bundleLinuxCmd(args []string) {
	buildCmd(append([]string{"--linux"}, args...))
}

func bundleAndroidCmd(args []string) {
	buildCmd(append([]string{"--android"}, args...))
}

func findAndroidNDKClang(arch string) (string, error) {
	prefix := "aarch64-linux-android24-clang"
	if arch == "amd64" {
		prefix = "x86_64-linux-android24-clang"
	}
	var candidates []string
	if ndkHome := os.Getenv("ANDROID_NDK_HOME"); ndkHome != "" {
		if m, err := filepath.Glob(filepath.Join(ndkHome, "toolchains", "llvm", "prebuilt", "*", "bin", prefix)); err == nil {
			candidates = append(candidates, m...)
		}
	}
	if m, err := filepath.Glob("/Applications/Unity/Hub/Editor/*/PlaybackEngines/AndroidPlayer/NDK/toolchains/llvm/prebuilt/*/bin/" + prefix); err == nil {
		candidates = append(candidates, m...)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if m, err := filepath.Glob(filepath.Join(home, "Library", "Android", "sdk", "ndk", "*", "toolchains", "llvm", "prebuilt", "*", "bin", prefix)); err == nil {
			candidates = append(candidates, m...)
		}
	}
	for i := len(candidates) - 1; i >= 0; i-- {
		if info, err := os.Stat(candidates[i]); err == nil && !info.IsDir() {
			return candidates[i], nil
		}
	}
	return "", fmt.Errorf("nie znaleziono kompilatora Android NDK (%s)", prefix)
}

func findUnityAndroidPlayerDir() string {
	matches, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/PlaybackEngines/AndroidPlayer")
	for i := len(matches) - 1; i >= 0; i-- {
		if info, err := os.Stat(filepath.Join(matches[i], "SDK")); err == nil && info.IsDir() {
			return matches[i]
		}
	}
	return ""
}

func packageAndroidBundle(rootDir string, archBins map[string]string) error {
	apDir := findUnityAndroidPlayerDir()
	if apDir == "" {
		return nil
	}
	sdkDir := filepath.Join(apDir, "SDK")
	jdkDir := filepath.Join(apDir, "OpenJDK")

	btMatches, _ := filepath.Glob(filepath.Join(sdkDir, "build-tools", "*"))
	platMatches, _ := filepath.Glob(filepath.Join(sdkDir, "platforms", "android-*"))
	if len(btMatches) == 0 || len(platMatches) == 0 {
		return nil
	}
	btDir := btMatches[len(btMatches)-1]
	androidJar := filepath.Join(platMatches[len(platMatches)-1], "android.jar")
	aapt := filepath.Join(btDir, "aapt")
	d8 := filepath.Join(btDir, "d8")
	zipalign := filepath.Join(btDir, "zipalign")
	apksigner := filepath.Join(btDir, "apksigner")
	javac := filepath.Join(jdkDir, "bin", "javac")
	keytool := filepath.Join(jdkDir, "bin", "keytool")

	workDir, err := os.MkdirTemp("", "storyteller-apk-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	srcDir := filepath.Join(workDir, "src", "pl", "storyteller", "app")
	classesDir := filepath.Join(workDir, "classes")
	resDir := filepath.Join(workDir, "res", "mipmap-xxxhdpi")
	assetsBooksDir := filepath.Join(workDir, "assets", "books")
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.MkdirAll(classesDir, 0755)
	_ = os.MkdirAll(resDir, 0755)
	_ = os.MkdirAll(assetsBooksDir, 0755)

	if pngIcon, err := encodeStoryTellerIconPNG(192); err == nil {
		_ = os.WriteFile(filepath.Join(resDir, "ic_launcher.png"), pngIcon, 0644)
	}
	if err := packageBooksForDist(filepath.Join(rootDir, "books"), assetsBooksDir); err != nil {
		return err
	}

	manifestXML := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="pl.storyteller.app"
    android:versionCode="1"
    android:versionName="1.0.0">
    <uses-sdk android:minSdkVersion="24" android:targetSdkVersion="35" />
    <uses-permission android:name="android.permission.INTERNET" />
    <application
        android:label="StoryTeller"
        android:icon="@mipmap/ic_launcher"
        android:usesCleartextTraffic="true"
        android:hardwareAccelerated="true"
        android:extractNativeLibs="true"
        android:theme="@android:style/Theme.NoTitleBar.Fullscreen">
        <activity
            android:name="pl.storyteller.app.MainActivity"
            android:exported="true"
            android:configChanges="orientation|screenSize|keyboardHidden">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`
	manifestPath := filepath.Join(workDir, "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, []byte(manifestXML), 0644); err != nil {
		return err
	}

	mainActivityJava := `package pl.storyteller.app;

import android.app.Activity;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.net.Socket;

public class MainActivity extends Activity {
    private Process serverProcess;
    private WebView webView;
    private static final int PORT = 18484;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        webView = new WebView(this);
        WebSettings ws = webView.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setDomStorageEnabled(true);
        ws.setMediaPlaybackRequiresUserGesture(false);
        webView.setWebViewClient(new WebViewClient());
        setContentView(webView);

        new Thread(new Runnable() {
            @Override
            public void run() {
                startEmbeddedServer();
            }
        }).start();
    }

    private void startEmbeddedServer() {
        try {
            File filesDir = getFilesDir();
            File booksDir = new File(filesDir, "books");
            if (!booksDir.exists()) {
                booksDir.mkdirs();
            }
            String[] assetBooks = getAssets().list("books");
            if (assetBooks != null) {
                for (String name : assetBooks) {
                    File dst = new File(booksDir, name);
                    if (!dst.exists()) {
                        try (InputStream in = getAssets().open("books/" + name);
                             FileOutputStream out = new FileOutputStream(dst)) {
                            byte[] buf = new byte[16384];
                            int n;
                            while ((n = in.read(buf)) > 0) {
                                out.write(buf, 0, n);
                            }
                        }
                    }
                }
            }
            String libDir = getApplicationInfo().nativeLibraryDir;
            File binFile = new File(libDir, "libstoryteller.so");
            ProcessBuilder pb = new ProcessBuilder(
                binFile.getAbsolutePath(), "run", "--headless", "--port", String.valueOf(PORT)
            );
            pb.directory(filesDir);
            pb.environment().put("ST_NO_WINDOW", "1");
            serverProcess = pb.start();

            for (int i = 0; i < 60; i++) {
                try (Socket s = new Socket("127.0.0.1", PORT)) {
                    break;
                } catch (Exception ignored) {
                    Thread.sleep(60);
                }
            }

            new Handler(Looper.getMainLooper()).post(new Runnable() {
                @Override
                public void run() {
                    if (webView != null) {
                        webView.loadUrl("http://127.0.0.1:" + PORT + "/");
                    }
                }
            });
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    @Override
    protected void onDestroy() {
        if (serverProcess != null) {
            serverProcess.destroy();
            serverProcess = null;
        }
        super.onDestroy();
    }
}
`
	javaPath := filepath.Join(srcDir, "MainActivity.java")
	if err := os.WriteFile(javaPath, []byte(mainActivityJava), 0644); err != nil {
		return err
	}

	envWithJava := append(os.Environ(), "JAVA_HOME="+jdkDir, "PATH="+filepath.Join(jdkDir, "bin")+":"+os.Getenv("PATH"))
	runTool := func(bin string, toolArgs ...string) error {
		c := exec.Command(bin, toolArgs...)
		c.Dir = workDir
		c.Env = envWithJava
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s failed: %v\n%s", filepath.Base(bin), err, string(out))
		}
		return nil
	}

	if err := runTool(aapt, "package", "-f", "-m", "-J", filepath.Join(workDir, "src"), "-M", manifestPath, "-S", filepath.Join(workDir, "res"), "-I", androidJar); err != nil {
		return err
	}
	rJavaPath := filepath.Join(srcDir, "R.java")
	javacArgs := []string{"-source", "1.8", "-target", "1.8", "-bootclasspath", androidJar, "-d", classesDir, javaPath}
	if _, err := os.Stat(rJavaPath); err == nil {
		javacArgs = append(javacArgs, rJavaPath)
	}
	if err := runTool(javac, javacArgs...); err != nil {
		return err
	}

	classFiles, _ := filepath.Glob(filepath.Join(classesDir, "pl", "storyteller", "app", "*.class"))
	d8Args := append([]string{"--lib", androidJar, "--output", workDir}, classFiles...)
	if err := runTool(d8, d8Args...); err != nil {
		return err
	}

	unalignedAPK := filepath.Join(workDir, "unaligned.apk")
	if err := runTool(aapt, "package", "-f", "-M", manifestPath, "-S", filepath.Join(workDir, "res"), "-A", filepath.Join(workDir, "assets"), "-I", androidJar, "-F", unalignedAPK); err != nil {
		return err
	}

	// Inject classes.dex and native lib/<abi>/libstoryteller.so into unaligned.apk
	if err := appendAndroidNativeAndDexToAPK(unalignedAPK, filepath.Join(workDir, "classes.dex"), archBins); err != nil {
		return err
	}

	alignedAPK := filepath.Join(workDir, "aligned.apk")
	if err := runTool(zipalign, "-f", "-p", "4", unalignedAPK, alignedAPK); err != nil {
		return err
	}

	ksPath := filepath.Join(workDir, "debug.keystore")
	if err := runTool(keytool, "-genkeypair", "-v", "-keystore", ksPath, "-storepass", "android", "-alias", "androiddebugkey", "-keypass", "android", "-keyalg", "RSA", "-keysize", "2048", "-validity", "10000", "-dname", "CN=Android Debug,O=Android,C=US"); err != nil {
		return err
	}

	outAPK := filepath.Join(rootDir, "dist", "StoryTeller-android.apk")
	if err := runTool(apksigner, "sign", "--ks", ksPath, "--ks-pass", "pass:android", "--key-pass", "pass:android", "--out", outAPK, alignedAPK); err != nil {
		return err
	}

	fmt.Printf("✓ Utworzono podpisany pakiet instalacyjny Android APK: %s\n", outAPK)
	return nil
}

func appendAndroidNativeAndDexToAPK(apkPath, dexPath string, archBins map[string]string) error {
	rawAPK, err := os.ReadFile(apkPath)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(rawAPK), int64(len(rawAPK)))
	if err != nil {
		return err
	}
	var outBuf bytes.Buffer
	zw := zip.NewWriter(&outBuf)

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		hdr := f.FileHeader
		w, err := zw.CreateHeader(&hdr)
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, _ = io.Copy(w, rc)
		_ = rc.Close()
	}

	if dexBytes, err := os.ReadFile(dexPath); err == nil {
		w, err := zw.Create("classes.dex")
		if err != nil {
			return err
		}
		_, _ = w.Write(dexBytes)
	}

	abiMap := map[string]string{
		"arm64": "arm64-v8a",
		"amd64": "x86_64",
	}
	for arch, binPath := range archBins {
		abi, ok := abiMap[arch]
		if !ok || binPath == "" {
			continue
		}
		binBytes, err := os.ReadFile(binPath)
		if err != nil {
			continue
		}
		hdr := &zip.FileHeader{
			Name:   "lib/" + abi + "/libstoryteller.so",
			Method: zip.Deflate,
		}
		hdr.SetMode(0755)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, _ = w.Write(binBytes)
	}

	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(apkPath, outBuf.Bytes(), 0644)
}

func sdCapsule(px, py, ax, ay, bx, by, r float64) float64 {
	pax, pay := px-ax, py-ay
	bax, bay := bx-ax, by-ay
	h := math.Max(0.0, math.Min(1.0, (pax*bax+pay*bay)/(bax*bax+bay*bay)))
	dx := pax - bax*h
	dy := pay - bay*h
	return math.Sqrt(dx*dx+dy*dy) - r
}

func blendRGBA(dst, src color.RGBA, alpha float64) color.RGBA {
	if alpha <= 0 {
		return dst
	}
	if alpha >= 1 {
		return src
	}
	inv := 1.0 - alpha
	return color.RGBA{
		R: uint8(math.Round(float64(dst.R)*inv + float64(src.R)*alpha)),
		G: uint8(math.Round(float64(dst.G)*inv + float64(src.G)*alpha)),
		B: uint8(math.Round(float64(dst.B)*inv + float64(src.B)*alpha)),
		A: uint8(math.Round(float64(dst.A)*inv + float64(src.A)*alpha)),
	}
}

// renderStoryTellerIcon renders an original StoryTeller audiobook icon:
// an open two-page book spread paired with studio headphones and a 5-bar audio equalizer.
// Uses 2x2 supersampling and pixel-aligned minimum stroke weights so small icons (16x16..64x64)
// and large icons (256x256..512x512) are both razor-sharp.
func renderStoryTellerIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scale := 512.0 / float64(size)
	aa := 0.45 * scale
	minHalf := math.Max(5.5, 0.72*scale)

	sqHalf := 216.0
	sqRad := 96.0
	if size <= 64 {
		sqHalf = 236.0
		sqRad = 96.0
	}

	smoothAlpha := func(dist float64) float64 {
		if dist >= aa {
			return 0.0
		}
		if dist <= -aa {
			return 1.0
		}
		return 0.5 - dist/(2.0*aa)
	}

	sampleAt := func(fx, fy float64) (float64, float64, float64, float64) {
		dxSq := math.Abs(fx-256.0) - (sqHalf - sqRad)
		dySq := math.Abs(fy-256.0) - (sqHalf - sqRad)
		qx := math.Max(dxSq, 0)
		qy := math.Max(dySq, 0)
		distSq := math.Sqrt(qx*qx+qy*qy) + math.Min(math.Max(dxSq, dySq), 0) - sqRad

		bgAlpha := smoothAlpha(distSq)
		if bgAlpha <= 0 {
			return 0, 0, 0, 0
		}

		tGrad := math.Max(0.0, math.Min(1.0, (fy-40.0)/432.0))
		bgCol := color.RGBA{
			R: uint8(24.0 - 10.0*tGrad),
			G: uint8(24.0 - 10.0*tGrad),
			B: uint8(30.0 - 12.0*tGrad),
			A: 255,
		}

		borderHalf := math.Max(3.0, 0.45*scale)
		borderDist := math.Abs(distSq+borderHalf) - borderHalf
		col := blendRGBA(bgCol, color.RGBA{R: 88, G: 88, B: 104, A: 255}, smoothAlpha(borderDist))

		ax := math.Abs(fx - 256.0)

		const bookHalfW = 96.0
		clampedX := math.Min(ax, bookHalfW)
		pageArch := -14.0 * math.Sin(math.Pi*clampedX/bookHalfW)
		yTop := 254.0 + pageArch
		yBot := 356.0 + pageArch
		yBot2 := 374.0 + pageArch

		if ax <= bookHalfW && fy >= yTop && fy <= yBot {
			col = blendRGBA(col, color.RGBA{R: 36, G: 36, B: 46, A: 255}, 0.85)
		}

		distBook := 1e9
		if ax <= bookHalfW+minHalf {
			distBook = math.Min(distBook, math.Abs(fy-yTop)-minHalf)
			distBook = math.Min(distBook, math.Abs(fy-yBot)-minHalf)
		}
		if size >= 32 && ax <= bookHalfW-4.0 {
			distBook = math.Min(distBook, math.Abs(fy-yBot2)-math.Max(4.0, 0.55*scale))
		}
		distBook = math.Min(distBook, sdCapsule(ax, fy, bookHalfW, 254.0, bookHalfW, 356.0, minHalf))
		distBook = math.Min(distBook, sdCapsule(fx, fy, 256.0, 254.0, 256.0, 360.0, minHalf))

		col = blendRGBA(col, color.RGBA{R: 248, G: 248, B: 252, A: 255}, smoothAlpha(distBook))

		distHeadphones := 1e9
		archHalf := math.Max(7.5, 0.78*scale)
		if fy <= 244.0 {
			dArc := math.Abs(math.Hypot(fx-256.0, fy-244.0)-138.0) - archHalf
			distHeadphones = math.Min(distHeadphones, dArc)
		}
		cupHalf := math.Max(14.0, 0.95*scale)
		dCup := sdCapsule(ax, fy, 138.0, 234.0, 138.0, 296.0, cupHalf)
		distHeadphones = math.Min(distHeadphones, dCup)

		col = blendRGBA(col, color.RGBA{R: 238, G: 238, B: 246, A: 255}, smoothAlpha(distHeadphones))

		eqHalf := math.Max(6.5, 0.70*scale)
		distEq := sdCapsule(fx, fy, 256.0, 150.0, 256.0, 216.0, eqHalf)
		distEq = math.Min(distEq, sdCapsule(ax, fy, 28.0, 168.0, 28.0, 216.0, eqHalf))
		if size >= 24 {
			distEq = math.Min(distEq, sdCapsule(ax, fy, 56.0, 186.0, 56.0, 216.0, eqHalf))
		}

		col = blendRGBA(col, color.RGBA{R: 212, G: 212, B: 226, A: 255}, smoothAlpha(distEq))
		return float64(col.R) * bgAlpha, float64(col.G) * bgAlpha, float64(col.B) * bgAlpha, bgAlpha
	}

	offsets := [2]float64{0.25, 0.75}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var rSum, gSum, bSum, aSum float64
			for _, oy := range offsets {
				for _, ox := range offsets {
					r, g, b, a := sampleAt((float64(x)+ox)*scale, (float64(y)+oy)*scale)
					rSum += r
					gSum += g
					bSum += b
					aSum += a
				}
			}
			aAvg := aSum / 4.0
			if aAvg <= 0 {
				continue
			}
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(math.Min(255, math.Round(rSum/(4.0*aAvg)))),
				G: uint8(math.Min(255, math.Round(gSum/(4.0*aAvg)))),
				B: uint8(math.Min(255, math.Round(bSum/(4.0*aAvg)))),
				A: uint8(math.Min(255, math.Round(255.0*aAvg))),
			})
		}
	}
	return img
}

func encodeStoryTellerIconPNG(size int) ([]byte, error) {
	img := renderStoryTellerIcon(size)
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}
	return pngBuf.Bytes(), nil
}

func writeMacOSAppIconICNS(outPath string) error {
	pngData, err := encodeStoryTellerIconPNG(512)
	if err != nil {
		return err
	}

	var icnsBuf bytes.Buffer
	icnsBuf.WriteString("icns")
	_ = binary.Write(&icnsBuf, binary.BigEndian, uint32(16+len(pngData)))
	icnsBuf.WriteString("ic09")
	_ = binary.Write(&icnsBuf, binary.BigEndian, uint32(8+len(pngData)))
	icnsBuf.Write(pngData)

	return os.WriteFile(outPath, icnsBuf.Bytes(), 0644)
}

func encodeStoryTellerIconDIB(size int) []byte {
	img := renderStoryTellerIcon(size)
	andRowBytes := ((size + 31) / 32) * 4
	xorSize := size * size * 4
	andSize := andRowBytes * size

	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))              // biSize
	_ = binary.Write(&buf, binary.LittleEndian, int32(size))             // biWidth
	_ = binary.Write(&buf, binary.LittleEndian, int32(size*2))           // biHeight (XOR + AND)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))               // biPlanes
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))              // biBitCount
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))               // biCompression (BI_RGB)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(xorSize+andSize)) // biSizeImage
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))                // biXPelsPerMeter
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))                // biYPelsPerMeter
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))               // biClrUsed
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))               // biClrImportant

	for y := size - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			c := img.RGBAAt(x, y)
			buf.WriteByte(c.B)
			buf.WriteByte(c.G)
			buf.WriteByte(c.R)
			buf.WriteByte(c.A)
		}
	}

	andRow := make([]byte, andRowBytes)
	for y := size - 1; y >= 0; y-- {
		for i := range andRow {
			andRow[i] = 0
		}
		for x := 0; x < size; x++ {
			if img.RGBAAt(x, y).A < 128 {
				andRow[x>>3] |= 0x80 >> (uint(x) & 7)
			}
		}
		buf.Write(andRow)
	}
	return buf.Bytes()
}

type winIconFrame struct {
	sizeByte byte
	data     []byte
}

func buildStoryTellerIconFrames() ([]winIconFrame, error) {
	png256, err := encodeStoryTellerIconPNG(256)
	if err != nil {
		return nil, err
	}
	return []winIconFrame{
		{sizeByte: 0, data: png256},
		{sizeByte: 64, data: encodeStoryTellerIconDIB(64)},
		{sizeByte: 48, data: encodeStoryTellerIconDIB(48)},
		{sizeByte: 32, data: encodeStoryTellerIconDIB(32)},
		{sizeByte: 24, data: encodeStoryTellerIconDIB(24)},
		{sizeByte: 16, data: encodeStoryTellerIconDIB(16)},
	}, nil
}

func writeWindowsAppIconICO(outPath string) error {
	frames, err := buildStoryTellerIconFrames()
	if err != nil {
		return err
	}

	var icoBuf bytes.Buffer
	_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(len(frames)))

	offset := uint32(6 + 16*len(frames))
	for _, fr := range frames {
		icoBuf.WriteByte(fr.sizeByte)
		icoBuf.WriteByte(fr.sizeByte)
		icoBuf.WriteByte(0)
		icoBuf.WriteByte(0)
		_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(1))
		_ = binary.Write(&icoBuf, binary.LittleEndian, uint16(32))
		_ = binary.Write(&icoBuf, binary.LittleEndian, uint32(len(fr.data)))
		_ = binary.Write(&icoBuf, binary.LittleEndian, offset)
		offset += uint32(len(fr.data))
	}
	for _, fr := range frames {
		icoBuf.Write(fr.data)
	}
	return os.WriteFile(outPath, icoBuf.Bytes(), 0644)
}

// writeWindowsResourceSYSO generates an amd64 COFF .syso object containing the .rsrc section
// with RT_ICON (3) and RT_GROUP_ICON (14) so Go's linker embeds the icon directly into StoryTeller.exe.
func writeWindowsResourceSYSO(outPath string) error {
	return writeWindowsResourceSYSOForArch(outPath, "amd64")
}

func writeWindowsResourceSYSOForArch(outPath, goarch string) error {
	var machine uint16
	var relocType uint16
	switch goarch {
	case "amd64", "":
		machine = 0x8664   // IMAGE_FILE_MACHINE_AMD64
		relocType = 0x0003 // IMAGE_REL_AMD64_ADDR32NB
	case "arm64":
		machine = 0xAA64   // IMAGE_FILE_MACHINE_ARM64
		relocType = 0x0002 // IMAGE_REL_ARM64_ADDR32NB
	case "386":
		machine = 0x014c   // IMAGE_FILE_MACHINE_I386
		relocType = 0x0007 // IMAGE_REL_I386_DIR32NB
	default:
		return fmt.Errorf("nieobsługiwana architektura PE .syso: %s", goarch)
	}

	frames, err := buildStoryTellerIconFrames()
	if err != nil {
		return err
	}
	nIcons := len(frames) // 4

	var grpBuf bytes.Buffer
	_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(nIcons))
	for i, fr := range frames {
		grpBuf.WriteByte(fr.sizeByte)
		grpBuf.WriteByte(fr.sizeByte)
		grpBuf.WriteByte(0)
		grpBuf.WriteByte(0)
		_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(1))
		_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(32))
		_ = binary.Write(&grpBuf, binary.LittleEndian, uint32(len(fr.data)))
		_ = binary.Write(&grpBuf, binary.LittleEndian, uint16(i+1))
	}
	grpData := grpBuf.Bytes()
	manifestData := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0" xmlns:asmv3="urn:schemas-microsoft-com:asm.v3">
  <assemblyIdentity type="win32" name="StoryTeller.App" version="1.0.0.0"/>
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
  <asmv3:application>
    <asmv3:windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2, PerMonitor</dpiAwareness>
    </asmv3:windowsSettings>
  </asmv3:application>
</assembly>`)

	// Directory offsets:
	// 0..40: Level 1 (Root: RT_ICON=3, RT_GROUP_ICON=14, RT_MANIFEST=24)
	// iconL2Off: Level 2 for RT_ICON (IDs 1..nIcons)
	// grpL2Off: Level 2 for RT_GROUP_ICON (ID 1)
	// manL2Off: Level 2 for RT_MANIFEST (ID 1)
	// iconL3Base: nIcons Level 3 directories (24B each) for RT_ICON
	// grpL3Off: Level 3 directory for RT_GROUP_ICON
	// manL3Off: Level 3 directory for RT_MANIFEST
	// dataEntriesOff: IMAGE_RESOURCE_DATA_ENTRY x (nIcons+2)
	iconL2Off := 40
	grpL2Off := iconL2Off + 16 + 8*nIcons
	manL2Off := grpL2Off + 24
	iconL3Base := manL2Off + 24
	grpL3Off := iconL3Base + 24*nIcons
	manL3Off := grpL3Off + 24
	dataEntriesOff := manL3Off + 24
	totalDataEntries := nIcons + 2
	payloadStart := dataEntriesOff + 16*totalDataEntries

	curOff := payloadStart
	frameOffsets := make([]int, nIcons)
	for i, fr := range frames {
		frameOffsets[i] = curOff
		curOff += (len(fr.data) + 7) &^ 7
	}
	grpDataOffset := curOff
	curOff += (len(grpData) + 7) &^ 7
	manDataOffset := curOff
	rsrcSize := manDataOffset + ((len(manifestData) + 7) &^ 7)

	rsrc := make([]byte, rsrcSize)
	putDir := func(off int, idEntries uint16) {
		binary.LittleEndian.PutUint16(rsrc[off+14:], idEntries)
	}
	putEntry := func(off int, id uint32, target uint32, isSubdir bool) {
		binary.LittleEndian.PutUint32(rsrc[off:], id)
		if isSubdir {
			target |= 0x80000000
		}
		binary.LittleEndian.PutUint32(rsrc[off+4:], target)
	}

	// Level 1 (RT_ICON=3, RT_GROUP_ICON=14, RT_MANIFEST=24)
	putDir(0, 3)
	putEntry(16, 3, uint32(iconL2Off), true)
	putEntry(24, 14, uint32(grpL2Off), true)
	putEntry(32, 24, uint32(manL2Off), true)

	// Level 2
	putDir(iconL2Off, uint16(nIcons))
	for i := 0; i < nIcons; i++ {
		putEntry(iconL2Off+16+8*i, uint32(i+1), uint32(iconL3Base+24*i), true)
	}
	putDir(grpL2Off, 1)
	putEntry(grpL2Off+16, 1, uint32(grpL3Off), true)
	putDir(manL2Off, 1)
	putEntry(manL2Off+16, 1, uint32(manL3Off), true)

	// Level 3 & Data Entries for RT_ICON
	for i, fr := range frames {
		l3 := iconL3Base + 24*i
		de := dataEntriesOff + 16*i
		putDir(l3, 1)
		putEntry(l3+16, 0x0409, uint32(de), false)
		binary.LittleEndian.PutUint32(rsrc[de:], uint32(frameOffsets[i]))
		binary.LittleEndian.PutUint32(rsrc[de+4:], uint32(len(fr.data)))
		copy(rsrc[frameOffsets[i]:], fr.data)
	}

	// Level 3 & Data Entry for RT_GROUP_ICON
	grpDE := dataEntriesOff + 16*nIcons
	putDir(grpL3Off, 1)
	putEntry(grpL3Off+16, 0x0409, uint32(grpDE), false)
	binary.LittleEndian.PutUint32(rsrc[grpDE:], uint32(grpDataOffset))
	binary.LittleEndian.PutUint32(rsrc[grpDE+4:], uint32(len(grpData)))
	copy(rsrc[grpDataOffset:], grpData)

	// Level 3 & Data Entry for RT_MANIFEST
	manDE := dataEntriesOff + 16*(nIcons+1)
	putDir(manL3Off, 1)
	putEntry(manL3Off+16, 0x0409, uint32(manDE), false)
	binary.LittleEndian.PutUint32(rsrc[manDE:], uint32(manDataOffset))
	binary.LittleEndian.PutUint32(rsrc[manDE+4:], uint32(len(manifestData)))
	copy(rsrc[manDataOffset:], manifestData)

	rawOffset := uint32(60)
	relocOffset := rawOffset + uint32(rsrcSize)
	symOffset := relocOffset + uint32(10*totalDataEntries)

	var coff bytes.Buffer
	_ = binary.Write(&coff, binary.LittleEndian, machine)
	_ = binary.Write(&coff, binary.LittleEndian, uint16(1))
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&coff, binary.LittleEndian, symOffset)
	_ = binary.Write(&coff, binary.LittleEndian, uint32(1))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(0))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(0))

	var secName [8]byte
	copy(secName[:], ".rsrc")
	coff.Write(secName[:])
	_ = binary.Write(&coff, binary.LittleEndian, uint32(rsrcSize))
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&coff, binary.LittleEndian, uint32(rsrcSize))
	_ = binary.Write(&coff, binary.LittleEndian, rawOffset)
	_ = binary.Write(&coff, binary.LittleEndian, relocOffset)
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(totalDataEntries))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(0))
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0x40000040))

	coff.Write(rsrc)

	for i := 0; i < totalDataEntries; i++ {
		de := uint32(dataEntriesOff + 16*i)
		_ = binary.Write(&coff, binary.LittleEndian, de)
		_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
		_ = binary.Write(&coff, binary.LittleEndian, relocType)
	}

	coff.Write(secName[:])
	_ = binary.Write(&coff, binary.LittleEndian, uint32(0))
	_ = binary.Write(&coff, binary.LittleEndian, int16(1))
	_ = binary.Write(&coff, binary.LittleEndian, uint16(0))
	coff.WriteByte(3)
	coff.WriteByte(0)

	_ = binary.Write(&coff, binary.LittleEndian, uint32(4))

	return os.WriteFile(outPath, coff.Bytes(), 0644)
}

func zipDirRecursive(srcDir, outZipPath, rootPrefix string) error {
	_ = os.Remove(outZipPath)
	zf, err := os.Create(outZipPath)
	if err != nil {
		return err
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		zipName := filepath.ToSlash(filepath.Join(rootPrefix, rel))
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = zipName
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func packCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Podaj identyfikator książki, np.: storyteller pack nie-boj-sie-policji-rzecznej")
		os.Exit(1)
	}
	bookID := book.SanitizeBookID(args[0])
	rootDir := resolveRepoRootDir()
	srcDir := filepath.Join(rootDir, "books", bookID)
	distDir := filepath.Join(rootDir, "dist")
	_ = os.MkdirAll(distDir, 0755)
	outZip := filepath.Join(distDir, bookID+".stbook")

	if err := book.PackBookToZip(srcDir, outZip); err != nil {
		fmt.Fprintf(os.Stderr, "Błąd pakowania .stbook: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Spakowano książkę do archiwum: %s\n", outZip)
}

func addCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Podaj ścieżkę na dysku lub adres URL, np.: storyteller add /ścieżka/do/książki lub storyteller add https://example.com/ksiazka.stbook")
		os.Exit(1)
	}
	rootDir := resolveRootDir()
	booksDir := filepath.Join(rootDir, "books")
	savesDir := filepath.Join(rootDir, "saves")
	settings := book.LoadAppSettings(savesDir)

	for _, target := range args {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			b, err := book.DownloadBookFromURL(booksDir, trimmed)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Błąd pobierania książki z URL (%s): %v\n", trimmed, err)
				os.Exit(1)
			}
			settings.ActiveBookID = b.ID
			_ = book.SaveAppSettings(savesDir, settings)
			fmt.Printf("✓ Pobrano i dodano książkę do biblioteki: %s (%s)\n", b.Title, b.RootPath)
			continue
		}

		resolved, err := book.ResolveDiskBookPaths(trimmed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Błąd dodawania książki z dysku (%s): %v\n", trimmed, err)
			os.Exit(1)
		}
		for _, p := range resolved {
			if filepath.Clean(filepath.Dir(p)) != filepath.Clean(booksDir) {
				settings.AddExternalBook(p)
			}
			if b, err := book.LoadBookFromPath(p); err == nil && b != nil {
				settings.ActiveBookID = b.ID
				fmt.Printf("✓ Dodano książkę z dysku (bez kopiowania): %s -> %s\n", b.Title, p)
			}
		}
		_ = book.SaveAppSettings(savesDir, settings)
	}
}
