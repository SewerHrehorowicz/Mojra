//go:build !(js && wasm)

package gui

import (
	"bufio"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"mojra/internal/book"
)

var nativeWindowDevMode bool

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

// LaunchWithOptions starts the in-process Mojra engine and opens the Native OS Window (WKWebView on macOS / Edge WebView2 on Windows).
func LaunchWithOptions(rootDir string, embeddedFS fs.FS, port int, headless bool, devMode bool) error {
	nativeWindowDevMode = devMode
	StopPreviousInstance(rootDir)

	pidPath := pidFilePath(rootDir)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); err == nil {
		defer func() {
			_ = os.Remove(pidPath)
		}()
	}

	srv := NewServerWithOptions(rootDir, embeddedFS, devMode)
	bindHost := "127.0.0.1"
	if devMode && (runtime.GOOS != "windows" || os.Getenv("ST_LAN") == "1") {
		bindHost = ""
	}
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
	srv.Port = actualPort

	localURL := fmt.Sprintf("http://127.0.0.1:%d", actualPort)

	fmt.Printf("Mojra uruchomiona:\n")
	fmt.Printf("  • Okno lokalne: %s\n", localURL)
	if devMode {
		lanIP := DetectLANIP()
		lanURL := fmt.Sprintf("http://%s:%d", lanIP, actualPort)
		fmt.Printf("  • Podgląd mobilny (Wi-Fi QR): %s\n", lanURL)
	}

	httpSrv := &http.Server{
		Handler:           srv.Handler(),
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
				srv.TriggerShutdown()
				TerminateNativeWindow()
				return
			}
		}
	}()

	go func() {
		<-sigCh
		srv.TriggerShutdown()
		TerminateNativeWindow()
	}()

	if !headless {
		// On macOS, RunNativeOSWindow blocks on the main Cocoa runloop until the window is closed or Quit is clicked.
		if opened := RunNativeOSWindow(localURL, func() {
			srv.TriggerShutdown()
		}); opened {
			<-srv.ShutdownCh
			return nil
		}
	}

	<-srv.ShutdownCh
	return nil
}
