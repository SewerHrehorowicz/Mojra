//go:build linux

package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	linuxMu   sync.Mutex
	linuxProc *os.Process
)

func resolveLinuxWebViewHost() string {
	candidates := []string{
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"microsoft-edge",
		"microsoft-edge-stable",
		"brave-browser",
	}
	for _, candidate := range candidates {
		if p, err := exec.LookPath(candidate); err == nil && p != "" {
			return p
		}
	}
	return ""
}

func RunNativeOSWindow(url string, onClosed func()) bool {
	if os.Getenv("ST_NO_WINDOW") == "1" {
		return false
	}
	if strings.HasSuffix(os.Args[0], ".test") {
		return false
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}

	if hostBin := resolveLinuxWebViewHost(); hostBin != "" {
		baseCacheDir, err := os.UserCacheDir()
		if err != nil || baseCacheDir == "" {
			baseCacheDir = os.TempDir()
		}
		profileDir := filepath.Join(baseCacheDir, "StoryTeller-WebView")
		_ = os.MkdirAll(profileDir, 0700)

		cmd := exec.Command(
			hostBin,
			"--app="+url,
			"--new-window",
			"--window-size=1440,900",
			"--user-data-dir="+profileDir,
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-features=Translate",
		)

		start := time.Now()
		if err := cmd.Start(); err == nil {
			linuxMu.Lock()
			linuxProc = cmd.Process
			linuxMu.Unlock()

			_ = cmd.Wait()

			linuxMu.Lock()
			linuxProc = nil
			linuxMu.Unlock()

			if time.Since(start) >= 1500*time.Millisecond && onClosed != nil {
				onClosed()
			}
			return true
		}
	}

	if xdgOpen, err := exec.LookPath("xdg-open"); err == nil && xdgOpen != "" {
		fallback := exec.Command(xdgOpen, url)
		if err := fallback.Start(); err == nil {
			return true
		}
	}
	return false
}

func ToggleNativeWindowFullscreen() {}

func StartNativeWindowDrag() {}

func PickNativeBookPath(chooseDir bool) string {
	if os.Getenv("ST_NO_WINDOW") == "1" || strings.HasSuffix(os.Args[0], ".test") {
		return ""
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return ""
	}
	if zenity, err := exec.LookPath("zenity"); err == nil && zenity != "" {
		args := []string{"--file-selection", "--title=Wybierz książkę StoryTeller"}
		if chooseDir {
			args = append(args, "--directory")
		}
		if out, err := exec.Command(zenity, args...).Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
		return ""
	}
	if kdialog, err := exec.LookPath("kdialog"); err == nil && kdialog != "" {
		args := []string{"--getopenfilename"}
		if chooseDir {
			args = []string{"--getexistingdirectory"}
		}
		if out, err := exec.Command(kdialog, args...).Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

func PickNativeMediaPaths(kind string) []string {
	if os.Getenv("ST_NO_WINDOW") == "1" || strings.HasSuffix(os.Args[0], ".test") {
		return nil
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil
	}
	normKind := strings.ToLower(strings.TrimSpace(kind))
	if zenity, err := exec.LookPath("zenity"); err == nil && zenity != "" {
		args := []string{"--file-selection", "--multiple", "--separator=\n", "--title=Wybierz pliki multimedialne"}
		switch normKind {
		case "image", "img":
			args = append(args, "--file-filter=Obrazy (*.png *.jpg *.jpeg *.webp) | *.png *.jpg *.jpeg *.webp *.PNG *.JPG *.JPEG *.WEBP")
		case "audio", "voice":
			args = append(args, "--file-filter=Audio (*.ogg *.flac *.mp3) | *.ogg *.flac *.mp3 *.OGG *.FLAC *.MP3")
		default:
			args = append(args, "--file-filter=Multimedia | *.png *.jpg *.jpeg *.webp *.ogg *.flac *.mp3 *.PNG *.JPG *.JPEG *.WEBP *.OGG *.FLAC *.MP3")
		}
		if out, err := exec.Command(zenity, args...).Output(); err == nil {
			var paths []string
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					paths = append(paths, trimmed)
				}
			}
			return paths
		}
		return nil
	}
	if kdialog, err := exec.LookPath("kdialog"); err == nil && kdialog != "" {
		filter := "*.png *.jpg *.jpeg *.webp *.ogg *.flac *.mp3"
		switch normKind {
		case "image", "img":
			filter = "*.png *.jpg *.jpeg *.webp"
		case "audio", "voice":
			filter = "*.ogg *.flac *.mp3"
		}
		args := []string{"--getopenfilename", ".", filter, "--multiple", "--separate-output"}
		if out, err := exec.Command(kdialog, args...).Output(); err == nil {
			var paths []string
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					paths = append(paths, trimmed)
				}
			}
			return paths
		}
	}
	return nil
}

func TerminateNativeWindow() {
	linuxMu.Lock()
	proc := linuxProc
	linuxMu.Unlock()
	if proc != nil {
		_ = proc.Kill()
	}
}
