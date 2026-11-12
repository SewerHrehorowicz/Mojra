//go:build !(js && wasm)

package osbridge

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"mojra/internal/book"
	"mojra/internal/logging"
)

var nativeWindowDevMode bool

func IsRunningUnderGoTestBinary() bool {
	return os.Getenv("ST_NO_WINDOW") == "1" ||
		strings.HasSuffix(os.Args[0], ".test") ||
		strings.HasSuffix(os.Args[0], ".test.exe") ||
		strings.Contains(os.Args[0], "/_test/")
}

func RevealExportedFileInOS(filePath string) {
	if IsRunningUnderGoTestBinary() || os.Getenv("ST_NO_WINDOW") == "1" {
		return
	}
	cleanPath := filepath.Clean(strings.TrimSpace(filePath))
	if cleanPath == "" || cleanPath == "." {
		return
	}
	dirPath := filepath.Dir(cleanPath)
	go func() {
		switch runtime.GOOS {
		case "darwin":
			if err := exec.Command("open", "-R", cleanPath).Start(); err != nil {
				_ = exec.Command("open", dirPath).Start()
			}
		case "windows":
			if err := exec.Command("explorer.exe", "/select,", cleanPath).Start(); err != nil {
				_ = exec.Command("explorer.exe", dirPath).Start()
			}
		case "linux":
			_ = exec.Command("xdg-open", dirPath).Start()
		}
	}()
}

// DetectLANIP finds the preferred local Wi-Fi/LAN IPv4 address for mobile QR preview.
func DetectLANIP() string {
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
					continue
				}
				if ipv4 := ip.To4(); ipv4 != nil {
					return ipv4.String()
				}
			}
		}
	}
	return "127.0.0.1"
}

func pidFilePath(rootDir string) string {
	savesDir := book.ResolveSavesDir(rootDir)
	_ = os.MkdirAll(savesDir, 0755)
	return filepath.Join(savesDir, ".mojra.pid")
}

// StopPreviousInstance terminates any stale Mojra GUI process recorded in .mojra.pid.
func StopPreviousInstance(rootDir string) {
	pidPath := pidFilePath(rootDir)
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid != os.Getpid() {
		if proc, err := os.FindProcess(pid); err == nil {
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				_ = proc.Kill()
			}
		}
	}
	_ = os.Remove(pidPath)
}

// LaunchWithOptions starts the in-process Mojra HTTP server and opens the Native OS Window (WKWebView on macOS / Edge WebView2 on Windows).
func LaunchWithOptions(rootDir string, port int, headless bool, devMode bool, handler http.Handler, shutdownCh <-chan struct{}, triggerShutdown func(), setPort func(int)) error {
	nativeWindowDevMode = devMode
	tPID := time.Now()
	StopPreviousInstance(rootDir)
	logging.Active.MarkStep(logging.StageStopPrevPID, tPID, rootDir)

	pidPath := pidFilePath(rootDir)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); err == nil {
		defer func() {
			_ = os.Remove(pidPath)
		}()
	}

	bindHost := "127.0.0.1"
	if devMode && (runtime.GOOS != "windows" || os.Getenv("ST_LAN") == "1") {
		bindHost = ""
	}
	tListen := time.Now()
	listenAddr := fmt.Sprintf("%s:%d", bindHost, port)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		ln, err = net.Listen("tcp", fmt.Sprintf("%s:0", bindHost))
		if err != nil {
			return err
		}
	}
	defer ln.Close()

	actualPort := ln.Addr().(*net.TCPAddr).Port
	if setPort != nil {
		setPort(actualPort)
	}

	localURL := fmt.Sprintf("http://127.0.0.1:%d", actualPort)
	logging.Active.MarkStep(logging.StageNetListen, tListen, localURL)

	fmt.Printf("Mojra uruchomiona:\n")
	fmt.Printf("  • Okno lokalne: %s\n", localURL)
	if devMode {
		tLAN := time.Now()
		lanIP := DetectLANIP()
		lanURL := fmt.Sprintf("http://%s:%d", lanIP, actualPort)
		logging.Active.MarkStep(logging.StageDetectLAN, tLAN, lanURL)
		fmt.Printf("  • Podgląd mobilny (Wi-Fi QR): %s\n", lanURL)
	}

	httpSrv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		_ = httpSrv.Serve(ln)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			cmd := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if cmd == "exit" || cmd == "quit" || cmd == "q" || cmd == "stop" {
				if triggerShutdown != nil {
					triggerShutdown()
				}
				TerminateNativeWindow()
				return
			}
		}
	}()

	go func() {
		<-sigCh
		if triggerShutdown != nil {
			triggerShutdown()
		}
		TerminateNativeWindow()
	}()

	if !headless {
		// On macOS, RunNativeOSWindow blocks on the main Cocoa runloop until the window is closed or Quit is clicked.
		if opened := RunNativeOSWindow(localURL, func() {
			if triggerShutdown != nil {
				triggerShutdown()
			}
		}); opened {
			<-shutdownCh
			return nil
		}
	}

	<-shutdownCh
	return nil
}
