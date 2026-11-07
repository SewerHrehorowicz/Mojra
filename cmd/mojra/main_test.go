package main

import (
	"bytes"
	"debug/pe"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAppIconsAndWindowsResourceSYSO(t *testing.T) {
	tmp := t.TempDir()

	icnsPath := filepath.Join(tmp, "AppIcon.icns")
	if err := writeMacOSAppIconICNS(icnsPath); err != nil {
		t.Fatalf("writeMacOSAppIconICNS failed: %v", err)
	}

	icoPath := filepath.Join(tmp, "Mojra.ico")
	if err := writeWindowsAppIconICO(icoPath); err != nil {
		t.Fatalf("writeWindowsAppIconICO failed: %v", err)
	}

	pngBytes, err := encodeIconPNG(256)
	if err != nil {
		t.Fatalf("encodeIconPNG failed: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode failed: %v", err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("unexpected icon dimensions: %v", img.Bounds())
	}

	// Verify key features of the Mojra "M" loom icon (512px render):
	// 1. Center destiny node dot at (256, 256)
	// 2. Top-left & bottom-left solid nodes at (141, 135) and (141, 377)
	// 3. Bottom-right solid node at (371, 377)
	// 4. Center lower vertical stem at (256, 330)
	icon512 := renderIcon(512)
	for _, pt := range []struct {
		name string
		x, y int
	}{
		{"center destiny node", 256, 256},
		{"top-left node", 141, 135},
		{"bottom-left node", 141, 377},
		{"bottom-right node", 371, 377},
		{"center lower stem", 256, 330},
	} {
		c := icon512.RGBAAt(pt.x, pt.y)
		if c.R < 180 || c.G < 180 || c.B < 180 {
			t.Fatalf("expected bright foreground glyph at %s (%d,%d), got RGBA=%v", pt.name, pt.x, pt.y, c)
		}
	}

	sysoPath := filepath.Join(tmp, "rsrc_windows_amd64.syso")
	if err := writeWindowsResourceSYSO(sysoPath); err != nil {
		t.Fatalf("writeWindowsResourceSYSO failed: %v", err)
	}

	peFile, err := pe.Open(sysoPath)
	if err != nil {
		t.Fatalf("pe.Open(.syso) failed: %v", err)
	}
	defer peFile.Close()

	rsrcSec := peFile.Section(".rsrc")
	if rsrcSec == nil {
		t.Fatalf("expected .rsrc section in generated .syso")
	}
	if len(rsrcSec.Relocs) != 8 {
		t.Fatalf("expected 8 relocations in .rsrc (6 RT_ICON + 1 RT_GROUP_ICON + 1 RT_MANIFEST), got %d", len(rsrcSec.Relocs))
	}

	sysoArm64Path := filepath.Join(tmp, "rsrc_windows_arm64.syso")
	if err := writeWindowsResourceSYSOForArch(sysoArm64Path, "arm64"); err != nil {
		t.Fatalf("writeWindowsResourceSYSOForArch(arm64) failed: %v", err)
	}
	peArm64, err := pe.Open(sysoArm64Path)
	if err != nil {
		t.Fatalf("pe.Open(arm64 .syso) failed: %v", err)
	}
	defer peArm64.Close()
	if peArm64.Machine != 0xAA64 {
		t.Fatalf("expected ARM64 machine 0xAA64, got 0x%x", peArm64.Machine)
	}

	linuxIconPath := filepath.Join(tmp, "Mojra.png")
	if err := writeLinuxAppIconPNG(linuxIconPath); err != nil {
		t.Fatalf("writeLinuxAppIconPNG failed: %v", err)
	}
}

func TestParseBuildArgs(t *testing.T) {
	optsDefault, err := parseBuildArgs(nil)
	if err != nil {
		t.Fatalf("parseBuildArgs(nil) failed: %v", err)
	}
	if len(optsDefault.Platforms) != 3 || len(optsDefault.Targets) != 6 {
		t.Fatalf("expected default (no args) to build all 3 platforms (6 targets), got platforms=%v targets=%v", optsDefault.Platforms, optsDefault.Targets)
	}

	optsAll, err := parseBuildArgs([]string{"--all"})
	if err != nil {
		t.Fatalf("parseBuildArgs(--all) failed: %v", err)
	}
	if len(optsAll.Platforms) != 3 || len(optsAll.Targets) != 6 {
		t.Fatalf("expected 3 platforms and 6 targets for --all, got platforms=%v targets=%v", optsAll.Platforms, optsAll.Targets)
	}

	optsWinLinux, err := parseBuildArgs([]string{"--windows", "--linux", "--arch", "amd64"})
	if err != nil {
		t.Fatalf("parseBuildArgs(--windows --linux --arch amd64) failed: %v", err)
	}
	if len(optsWinLinux.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %v", optsWinLinux.Targets)
	}
	if optsWinLinux.Targets[0] != (platformTarget{OS: "windows", Arch: "amd64"}) ||
		optsWinLinux.Targets[1] != (platformTarget{OS: "linux", Arch: "amd64"}) {
		t.Fatalf("unexpected targets: %v", optsWinLinux.Targets)
	}

	optsExplicit, err := parseBuildArgs([]string{"-p", "darwin/arm64,windows/amd64,linux/arm64", "--binaries-only"})
	if err != nil {
		t.Fatalf("parseBuildArgs(-p ...) failed: %v", err)
	}
	if !optsExplicit.BinariesOnly || len(optsExplicit.Targets) != 3 {
		t.Fatalf("expected BinariesOnly=true and 3 targets, got %+v", optsExplicit)
	}

	for _, helpArg := range []string{"--help", "-h", "help", "/?"} {
		optsHelp, err := parseBuildArgs([]string{helpArg})
		if err != nil || !optsHelp.ShowHelp {
			t.Fatalf("expected ShowHelp=true for %q, err=%v", helpArg, err)
		}
	}

	optsYes, err := parseBuildArgs([]string{"-y", "--yes", "--macos"})
	if err != nil {
		t.Fatalf("parseBuildArgs(-y --yes --macos) failed: %v", err)
	}
	if len(optsYes.Platforms) != 1 || optsYes.Platforms[0] != "darwin" {
		t.Fatalf("unexpected platforms for -y --yes --macos: %v", optsYes.Platforms)
	}

	optsPreview, err := parseBuildArgs([]string{"--macos", "--preview"})
	if err != nil {
		t.Fatalf("parseBuildArgs(--macos --preview) failed: %v", err)
	}
	if !optsPreview.Preview || len(optsPreview.Platforms) != 1 || optsPreview.Platforms[0] != "darwin" {
		t.Fatalf("expected Preview=true and darwin platform for --macos --preview, got %+v", optsPreview)
	}

	if _, err := parseBuildArgs([]string{"--platform", "solaris"}); err == nil {
		t.Fatalf("expected error for unsupported platform")
	}
}

func TestBuildScriptsPromptAndInstallDependencies(t *testing.T) {
	rootDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}

	batBytes, err := os.ReadFile(filepath.Join(rootDir, "build.bat"))
	if err != nil {
		t.Fatalf("failed to read build.bat: %v", err)
	}
	batContent := string(batBytes)
	for _, required := range []string{
		"set /p \"USER_CONFIRM=",
		"winget install --id GoLang.Go",
		"https://go.dev/VERSION?m=text",
		"go mod download",
	} {
		if !strings.Contains(batContent, required) {
			t.Fatalf("expected build.bat to contain %q", required)
		}
	}

	shPath := filepath.Join(rootDir, "build.sh")
	shBytes, err := os.ReadFile(shPath)
	if err != nil {
		t.Fatalf("failed to read build.sh: %v", err)
	}
	shContent := string(shBytes)
	for _, required := range []string{
		"read -r USER_CONFIRM",
		"https://go.dev/VERSION?m=text",
		"go mod download",
	} {
		if !strings.Contains(shContent, required) {
			t.Fatalf("expected build.sh to contain %q", required)
		}
	}

	if runtime.GOOS != "windows" {
		cmd := exec.Command("/bin/bash", shPath)
		cmd.Dir = rootDir
		cmd.Env = []string{
			"HOME=" + t.TempDir(),
			"PATH=/usr/bin:/bin",
			"ST_DISABLE_SYSTEM_GO_FALLBACK=1",
		}
		cmd.Stdin = strings.NewReader("n\n")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("expected build.sh to exit with non-zero status when go is missing and user declines installation")
		}
		outStr := string(out)
		if !strings.Contains(outStr, "Czy wyrażasz zgodę na automatyczną instalację Go") {
			t.Fatalf("expected user confirmation prompt in build.sh output, got: %s", outStr)
		}
		if !strings.Contains(outStr, "[ANULOWANO]") {
			t.Fatalf("expected cancellation notice in build.sh output, got: %s", outStr)
		}

		// Verify user agreement ("t") triggers dependency installation and proceeds
		if realGo, lookErr := exec.LookPath("go"); lookErr == nil && realGo != "" {
			fakeHome := t.TempDir()
			fakeBin := t.TempDir()
			mockInstaller := "#!/bin/sh\nmkdir -p \"$HOME/.local/go/bin\"\nln -sf \"" + realGo + "\" \"$HOME/.local/go/bin/go\"\nexit 0\n"
			for _, tool := range []string{"brew", "apt-get", "dnf", "pacman", "zypper", "apk"} {
				toolPath := filepath.Join(fakeBin, tool)
				if err := os.WriteFile(toolPath, []byte(mockInstaller), 0755); err != nil {
					t.Fatalf("failed to write mock %s: %v", tool, err)
				}
			}

			goCacheOut, _ := exec.Command(realGo, "env", "GOCACHE").Output()
			goPathOut, _ := exec.Command(realGo, "env", "GOPATH").Output()
			cmdAgree := exec.Command("/bin/bash", shPath, "--help")
			cmdAgree.Dir = rootDir
			cmdAgree.Env = append(os.Environ(),
				"HOME="+fakeHome,
				"GOCACHE="+strings.TrimSpace(string(goCacheOut)),
				"GOPATH="+strings.TrimSpace(string(goPathOut)),
				"PATH="+fakeBin+":/usr/bin:/bin",
				"ST_DISABLE_SYSTEM_GO_FALLBACK=1",
			)
			cmdAgree.Stdin = strings.NewReader("t\n")
			outAgree, errAgree := cmdAgree.CombinedOutput()
			if errAgree != nil {
				t.Fatalf("expected build.sh to succeed after user agrees to install Go, err=%v, output:\n%s", errAgree, string(outAgree))
			}
			outAgreeStr := string(outAgree)
			if !strings.Contains(outAgreeStr, "[INSTALACJA]") || !strings.Contains(outAgreeStr, "Wieloplatformowy System Budowania") {
				t.Fatalf("expected installation log and build help output, got:\n%s", outAgreeStr)
			}
		}
	}
}

func TestPackageBooksForDistPacksOnlyStbook(t *testing.T) {
	srcBooks := filepath.Join(t.TempDir(), "books")
	dstBooks := filepath.Join(t.TempDir(), "dist", "books")

	sampleSrc := filepath.Join(srcBooks, "nie-boj-sie-policji-rzecznej")
	if err := os.MkdirAll(sampleSrc, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sampleSrc, "book.playtxt"), []byte("title: Nie bój się policji rzecznej\nstart: s1\n"), 0644); err != nil {
		t.Fatalf("WriteFile book.playtxt failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sampleSrc, "story.playtxt"), []byte("scene: Scena 1\nid: s1\nend: true\nTreść.\n"), 0644); err != nil {
		t.Fatalf("WriteFile story.playtxt failed: %v", err)
	}

	if err := packageBooksForDist(srcBooks, dstBooks); err != nil {
		t.Fatalf("packageBooksForDist failed: %v", err)
	}

	// Source directory must NOT have a .stbook file created inside it
	if _, err := os.Stat(filepath.Join(srcBooks, "nie-boj-sie-policji-rzecznej.stbook")); !os.IsNotExist(err) {
		t.Fatalf("expected no .stbook in source books/ directory")
	}

	// Destination dist/books must have the packed .stbook and NOT the unpacked folder
	if _, err := os.Stat(filepath.Join(dstBooks, "nie-boj-sie-policji-rzecznej.stbook")); err != nil {
		t.Fatalf("expected nie-boj-sie-policji-rzecznej.stbook in dist books/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstBooks, "nie-boj-sie-policji-rzecznej")); !os.IsNotExist(err) {
		t.Fatalf("expected no unpacked source folder in dist books/")
	}
}

func TestResolveDistRuntimeDirNeverReturnsRepoRoot(t *testing.T) {
	fakeRepo := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeRepo, "go.mod"), []byte("module mojra\n"), 0644); err != nil {
		t.Fatalf("WriteFile go.mod failed: %v", err)
	}
	sampleSrc := filepath.Join(fakeRepo, "books", "nie-boj-sie-policji-rzecznej")
	if err := os.MkdirAll(sampleSrc, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sampleSrc, "book.playtxt"), []byte("title: Test\nstart: s1\n"), 0644); err != nil {
		t.Fatalf("WriteFile book.playtxt failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sampleSrc, "story.playtxt"), []byte("scene: Scena 1\nid: s1\nend: true\nTreść.\n"), 0644); err != nil {
		t.Fatalf("WriteFile story.playtxt failed: %v", err)
	}

	resolved := resolveDistRuntimeDir(fakeRepo)
	if filepath.Clean(resolved) == filepath.Clean(fakeRepo) {
		t.Fatalf("expected resolveDistRuntimeDir to never return repo root %q", fakeRepo)
	}
	if _, err := os.Stat(filepath.Join(resolved, "books", "nie-boj-sie-policji-rzecznej.stbook")); err != nil {
		t.Fatalf("expected packed .stbook in resolved runtime books dir: %v", err)
	}
}



