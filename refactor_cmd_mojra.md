# Plan refaktoryzacji `cmd/mojra/main.go` (`refactor_cmd_mojra.md`)

## 1. Co tam jest i jakie są problemy

1. **Proceduralne rysowanie ikon w kodzie Go (`L2007–2460`, ~450 LOC) zamiast użycia gotowych plików z `app/icons/` (`AGENTS.md §0, §4`)**:
   - `sdCapsule`, `blendRGBA`, `renderIcon`, `encodeIconPNG`, `renderAndroidIcon`, `encodeAndroidIconPNG`, `encodeIconDIB`, `buildIconFrames`, `writeWindowsAppIconICO`, `writeLinuxAppIconPNG` rysują w kodzie Go wektory/SDF ikony, mimo że w repozytorium istnieją już gotowe źródła: `app/icons/mojra.svg`, `app/icons/icon.png` oraz `app/icons/favicon.ico`.
   - `prepareBuildAssets` (`L538–545`) przy każdym buildzie nadpisuje `app/icons/favicon.ico` i `app/icons/icon.png`.
   - Generuje to ostrzeżenia `ARCH_SIGNATURE_BLOAT` (`sdCapsule`, `renderAndroidIcon`, `encodeAndroidIconPNG`) i `ARCH_RAVIOLI_INDIRECTION` (`buildAndroidAdaptiveIconXML`, `writeLinuxAppIconPNG`).
2. **Monolit (`ARCH_MONOLITH_AND_NESTING`, 105 pkt, 2892 LOC > 1000 LOC) i brak separacji odpowiedzialności (`AGENTS.md §0`)**:
   - Jeden plik miesza router CLI/runtime (`main`, `runCmd`, `packCmd`, `addCmd`, `auditCmd`), system budowania (`parseBuildArgs`, `buildCmd`, `buildPlatformBinary`), bundlery desktopowe (`packageMacOSBundle`, `packageWindowsBundle`, `packageLinuxBundle`, COFF `.syso`) i bundler Android APK (`packageAndroidBundle` + 400 linii kodu Java/XML).
3. **Rekordowa liczba magic stringów w ciałach funkcji (`audit_strings.log` #1 w repozytorium: 653 stringi, 23 958 pkt; `AGENTS.md §0`)**:
   - Wieloliniowe bloki tekstu i kodu zaszyte wewnątrz funkcji (`printUsage`, `printBuildUsage`, `Info.plist`, entitlements plist, skrypty `.bat`/`.sh`/`.desktop`, `AndroidManifest.xml`, `MainActivity.java`, Windows manifest XML).
   - Setki literałów flag, ścieżek, nazw plików i komunikatów w `parseBuildArgs` (70), `packageAndroidBundle` (87), `buildPlatformBinary` (69) i `auditCmd` (54).
4. **Manipulacje stringami (`ARCH_STRING_HACK`, 570 pkt)** w `parseBuildArgs` (254 pkt), `packageAndroidBundle` (202 pkt) i `auditCmd` (114 pkt).
5. **Funkcje pass-through (`ARCH_RAVIOLI_INDIRECTION`, zakazane w `AGENTS.md §0`)**:
   - `bundleMacOSCmd`, `bundleWindowsCmd`, `bundleLinuxCmd`, `bundleAndroidCmd`, `writeWindowsResourceSYSO`, `writeLinuxAppIconPNG`, `buildAndroidManifestXML`, `buildAndroidMainActivityJava`, `buildAndroidAdaptiveIconXML`.

---

## 2. Rozwiązania

1. **Całkowite usunięcie proceduralnego rysowania ikon w Go (~450 LOC) i użycie gotowych zasobów z `app/icons/`**:
   - Usunąć: `sdCapsule`, `blendRGBA`, `renderIcon`, `encodeIconPNG`, `renderAndroidIcon`, `encodeAndroidIconPNG`, `buildAndroidAdaptiveIconXML`, `encodeIconDIB`, `buildIconFrames`, `writeWindowsAppIconICO`, `writeLinuxAppIconPNG`.
   - W `prepareBuildAssets` usunąć nadpisywanie `app/icons/favicon.ico` i `app/icons/icon.png`.
   - Linux (`packageLinuxBundle`): kopiować `app/icons/icon.png` -> `dist/Mojra-linux/Mojra.png`.
   - Windows (`packageWindowsBundle`): kopiować `app/icons/favicon.ico` -> `dist/Mojra-windows/Mojra.ico`.
   - Windows PE `.syso` (`writeWindowsResourceSYSOForArch`): parsować ramki bezpośrednio z gotowego pliku `app/icons/favicon.ico` (nagłówek ICO 6B + wpisy 16B) i osadzać je w sekcji `.rsrc` wraz z `RT_GROUP_ICON` i `RT_MANIFEST`.
   - macOS (`writeMacOSAppIconICNS`): opakować bajty gotowego `app/icons/icon.png` w kontener ICNS (`icns` + `ic08`/`ic09`).
   - Android (`writeAndroidLauncherResources`): kopiować `app/icons/icon.png` do `mipmap-xxxhdpi/ic_launcher.png` oraz `ic_launcher_round.png`.
2. **Podział `cmd/mojra/` na 4 spójne pliki (`< 550 LOC` każdy)**:
   - `cmd/mojra/main.go` — stałe CLI, dispatch `main()`, `runCmd`, `packCmd`, `addCmd`, `auditCmd`, rozwiązywanie ścieżek.
   - `cmd/mojra/build.go` — tablicowy parser `parseBuildArgs`, `buildCmd`, `buildPlatformBinary`, `prepareBuildAssets`, `packageBooksForDist`.
   - `cmd/mojra/bundle_desktop.go` — `packageMacOSBundle`, `packageWindowsBundle`, `packageLinuxBundle`, `writeMacOSAppIconICNS`, `writeWindowsResourceSYSOForArch`, `adhocSignMacOSPath`, `zipDirRecursive`, `copyFileWithMode`.
   - `cmd/mojra/bundle_android.go` — `resolveAndroidToolchain`, `findAndroidNDKClang`, `packageAndroidBundle`, `appendAndroidNativeAndDexToAPK`, `writeAndroidLauncherResources`.
3. **Eliminacja magic stringów, `ARCH_STRING_HACK` i pass-through**:
   - Przeniesienie wszystkich literałów i szablonów tekstowych do `const (...)` i tablic strukturalnych `var (...)` na górze plików.
   - Zastąpienie switchy po stringach tablicami dyspozycyjnymi (`buildFlagTable`, `platformAliases`, `archAliases`, `auditLangAliases`).
   - Usunięcie wszystkich 1-linijkowych wrapperów pass-through.

---

## 3. Zmiany plik po pliku, metoda po metodzie

### Plik 1: `cmd/mojra/main.go` (ok. 350 LOC)
- **Stałe i słowniki na górze pliku**:
  - `usageText` (z `printUsage` L98–132), stałe komend, flag, zmiennych środowiskowych i komunikatów.
  - Mapa `bundleSubcommandFlags` (`bundle-all` -> `--all`, `bundle-macos` -> `--macos`, `bundle-windows` -> `--windows`, `bundle-linux` -> `--linux`, `bundle-android` -> `--android`) zastępująca usunięte funkcje `bundle*Cmd`.
- **`main()` (L57–95)**: dispatch komend ze słownika bez literałów w ciele; usunięcie `printUsage()`.
- **`isGoRunBinary`, `isRepoDir`, `resolveDistRuntimeDir`, `resolveRepoRootDir`, `resolveRootDir` (L49–252)**: użycie stałych ścieżek.
- **`runCmd`, `packCmd`, `addCmd`, `auditCmd`**: zastąpienie literałów stałymi; w `auditCmd` tablicowe mapowanie filtrów języka/ważności i pętla po 3 trybach audytu (`static`, `arch`, `magic`).

### Plik 2: `cmd/mojra/build.go` (nowy plik, przeniesione L135–179, L286–880; ok. 380 LOC)
- **Stałe i struktury na górze pliku**:
  - `buildUsageText` (z `printBuildUsage` L136–178), `platformTarget`, `buildBinaryMode{Preview, Telemetry bool}`, `buildOptions`.
  - Słowniki `platformAliases`, `archAliases`, `platformBinFormat`, `buildFlagTable`, `staleDistEntries`.
- **`normalizePlatformName` i `normalizeArchName`**: odczyt ze słowników `platformAliases` i `archAliases`.
- **`parseBuildArgs(args []string) (buildOptions, error)`**: tablicowy dispatch flag przez `buildFlagTable` + pomocnik `expandBuildTargets`.
- **`prepareBuildAssets(rootDir string) error` (L534–568)**:
  - Usunięcie generowania `favicon.ico` i `icon.png` (L538–545); pozostaje wyłącznie weryfikacja obecności `app/icons/icon.png` i `app/icons/favicon.ico`, `tpl.BuildApp(appDir)` oraz weryfikacja przykładowej książki.
- **`packageBooksForDist`, `platformBinaryFilename`, `buildPlatformBinary`, `buildCmd`**:
  - Zmiana sygnatury `buildPlatformBinary(rootDir string, target platformTarget, outPath string, mode buildBinaryMode) error` (usunięcie wariadycznego `preview ...bool`).

### Plik 3: `cmd/mojra/bundle_desktop.go` (nowy plik, przeniesione L882–1154, L2353–2709; ok. 340 LOC)
- **Stałe na górze pliku**:
  - `macOSEntitlementsPlist`, `macOSInfoPlist`, `windowsLauncherBat`, `linuxLauncherSh`, `linuxDesktopEntry`, `windowsAppManifestXML`, stałe binarne ICNS/ICO/COFF.
- **Całkowicie usunięte funkcje**:
  - `sdCapsule`, `blendRGBA`, `renderIcon`, `encodeIconPNG`, `encodeIconDIB`, `buildIconFrames`, `writeWindowsAppIconICO`, `writeLinuxAppIconPNG`, `writeWindowsResourceSYSO`, `bundleMacOSCmd`, `bundleWindowsCmd`, `bundleLinuxCmd`, `bundleAndroidCmd`.
- **`writeMacOSAppIconICNS(srcPNGPath, outPath string) error`**:
  - Wczytuje gotowy `app/icons/icon.png` i zapisuje nagłówek ICNS + dane PNG do `AppIcon.icns`.
- **`parseICOFrames(icoBytes []byte) ([]winIconFrame, error)` + `writeWindowsResourceSYSOForArch(icoPath, outPath, goarch string) error`**:
  - Odczytuje ramki bezpośrednio z gotowego pliku `app/icons/favicon.ico` i buduje obiekt COFF `.syso` z sekcją `.rsrc`.
- **`packageMacOSBundle`, `packageWindowsBundle`, `packageLinuxBundle`**:
  - `packageWindowsBundle` kopiuje `app/icons/favicon.ico` -> `dist/Mojra-windows/Mojra.ico`.
  - `packageLinuxBundle` kopiuje `app/icons/icon.png` -> `dist/Mojra-linux/Mojra.png`.

### Plik 4: `cmd/mojra/bundle_android.go` (nowy plik, przeniesione L1172–2005, L2301–2351; ok. 420 LOC)
- **Stałe na górze pliku**:
  - `androidManifestXML`, `androidMainActivityJava`, stałe ścieżek i argumentów narzędzi SDK/NDK/JDK.
- **Całkowicie usunięte funkcje**:
  - `renderAndroidIcon`, `encodeAndroidIconPNG`, `buildAndroidAdaptiveIconXML`, `buildAndroidManifestXML`, `buildAndroidMainActivityJava`.
- **`writeAndroidLauncherResources(srcIconPNG, resRootDir string) error`**:
  - Kopiuje gotowy `app/icons/icon.png` do `mipmap-xxxhdpi/ic_launcher.png` i `mipmap-xxxhdpi/ic_launcher_round.png` bez jakiegokolwiek rysowania w kodzie.
- **`candidateUnityAndroidPlayerDirs`, `candidateAndroidSDKDirs`, `findAndroidNDKClang`, `resolveAndroidToolBinary`, `resolveAndroidToolchain`, `packageAndroidBundle`, `appendAndroidNativeAndDexToAPK`**:
  - Wszystkie literały wyodrębnione do stałych i tablic na górze pliku.

### Plik 5: `cmd/mojra/main_test.go`
- **`TestAppIconsAndWindowsResourceSYSO` (L16–99)**:
  - Zamiast wywoływać usunięte `renderIcon` / `writeWindowsAppIconICO`, weryfikuje istniejące zasoby `app/icons/icon.png` i `app/icons/favicon.ico` oraz testuje `writeMacOSAppIconICNS` i `writeWindowsResourceSYSOForArch` na bazie tych plików.
- **`TestAndroidToolchainAndAPKPackagingInvariants` (L319–482)**:
  - Odwołuje się do stałych `androidManifestXML` i `androidMainActivityJava` oraz przekazuje ścieżkę `app/icons/icon.png` do `writeAndroidLauncherResources`.

### Plik 6: `docs/project.graph`
- Aktualizacja grafu projektu po podziale i usunięciu generatorów ikon.