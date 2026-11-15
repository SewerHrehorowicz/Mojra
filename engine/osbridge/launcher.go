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

	"mojra/engine"
)

// AppIconPNG holds the optional PNG icon bytes injected by the application shell at startup.
var AppIconPNG []byte

const (
	envNoWindow       = "ST_NO_WINDOW"
	envSTLAN          = "ST_LAN"
	testBinSuffix     = ".test"
	testExeSuffix     = ".test.exe"
	testDirMarker     = "/_test/"
	goosDarwin        = "darwin"
	goosWindows       = "windows"
	goosLinux         = "linux"
	cmdOpen           = "open"
	flagReveal        = "-R"
	cmdExplorer       = "explorer.exe"
	flagSelect        = "/select,"
	cmdXdgOpen        = "xdg-open"
	loopbackIP        = "127.0.0.1"
	pidFileName       = ".mojra.pid"
	tcpNet            = "tcp"
	listenAddrFmt     = "%s:%d"
	listenFallbackFmt = "%s:0"
	localURLFmt       = "http://127.0.0.1:%d"
	lanURLFmt         = "http://%s:%d"
	msgStartedHeader  = "Mojra uruchomiona:\n"
	msgLocalWindowFmt = "  • Okno lokalne: %s\n"
	msgLANPreviewFmt  = "  • Podgląd mobilny (Wi-Fi QR): %s\n"
	cmdExit           = "exit"
	cmdQuit           = "quit"
	cmdStop           = "stop"
)

var nativeWindowDevMode bool

func IsRunningUnderGoTestBinary() bool {
	return os.Getenv(envNoWindow) == "1" ||
		strings.HasSuffix(os.Args[0], testBinSuffix) ||
		strings.HasSuffix(os.Args[0], testExeSuffix) ||
		strings.Contains(os.Args[0], testDirMarker)
}

func RevealExportedFileInOS(filePath string) {
	if IsRunningUnderGoTestBinary() || os.Getenv(envNoWindow) == "1" {
		return
	}
	cleanPath := filepath.Clean(strings.TrimSpace(filePath))
	if cleanPath == "" || cleanPath == "." {
		return
	}
	dirPath := filepath.Dir(cleanPath)
	go func() {
		switch runtime.GOOS {
		case goosDarwin:
			if err := exec.Command(cmdOpen, flagReveal, cleanPath).Start(); err != nil {
				_ = exec.Command(cmdOpen, dirPath).Start()
			}
		case goosWindows:
			if err := exec.Command(cmdExplorer, flagSelect, cleanPath).Start(); err != nil {
				_ = exec.Command(cmdExplorer, dirPath).Start()
			}
		case goosLinux:
			_ = exec.Command(cmdXdgOpen, dirPath).Start()
		}
	}()
}

func pidFilePath(savesDir string) string {
	_ = os.MkdirAll(savesDir, 0755)
	return filepath.Join(savesDir, pidFileName)
}

// StopPreviousInstance terminates any stale Mojra GUI process recorded in .mojra.pid.
func StopPreviousInstance(savesDir string) {
	pidPath := pidFilePath(savesDir)
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
func LaunchWithOptions(savesDir string, port int, headless bool, devMode bool, handler http.Handler, shutdownCh <-chan struct{}, triggerShutdown func(), setPort func(int)) error {
	nativeWindowDevMode = devMode
	tPID := time.Now()
	StopPreviousInstance(savesDir)
	engine.Active.MarkStep(engine.StageStopPrevPID, tPID, savesDir)

	pidPath := pidFilePath(savesDir)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); err == nil {
		defer func() {
			_ = os.Remove(pidPath)
		}()
	}

	bindHost := loopbackIP
	if devMode && (runtime.GOOS != goosWindows || os.Getenv(envSTLAN) == "1") {
		bindHost = ""
	}
	tListen := time.Now()
	listenAddr := fmt.Sprintf(listenAddrFmt, bindHost, port)
	ln, err := net.Listen(tcpNet, listenAddr)
	if err != nil {
		ln, err = net.Listen(tcpNet, fmt.Sprintf(listenFallbackFmt, bindHost))
		if err != nil {
			return err
		}
	}
	defer ln.Close()

	actualPort := ln.Addr().(*net.TCPAddr).Port
	if setPort != nil {
		setPort(actualPort)
	}

	localURL := fmt.Sprintf(localURLFmt, actualPort)
	engine.Active.MarkStep(engine.StageNetListen, tListen, localURL)

	fmt.Print(msgStartedHeader)
	fmt.Printf(msgLocalWindowFmt, localURL)
	if devMode {
		tLAN := time.Now()
		lanIP := DetectLANIP()
		lanURL := fmt.Sprintf(lanURLFmt, lanIP, actualPort)
		engine.Active.MarkStep(engine.StageDetectLAN, tLAN, lanURL)
		fmt.Printf(msgLANPreviewFmt, lanURL)
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
			if cmd == cmdExit || cmd == cmdQuit || cmd == "q" || cmd == cmdStop {
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

const loopbackIPv4 = "localhost"

// DetectLANIP returns the preferred non-loopback IPv4 LAN address of the current machine.
func DetectLANIP() string {
	addrs, addrErr := net.InterfaceAddrs()
	if addrErr == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return loopbackIPv4
}
