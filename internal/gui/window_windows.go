//go:build windows

package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procUpdateWindow                  = user32.NewProc("UpdateWindow")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procSendMessageW                  = user32.NewProc("SendMessageW")
	procGetClientRect                 = user32.NewProc("GetClientRect")
	procGetWindowRect                 = user32.NewProc("GetWindowRect")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procGetWindowLongPtrW             = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW             = user32.NewProc("SetWindowLongPtrW")
	procLoadImageW                    = user32.NewProc("LoadImageW")
	procLoadCursorW                   = user32.NewProc("LoadCursorW")
	procReleaseCapture                = user32.NewProc("ReleaseCapture")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procIsZoomed                      = user32.NewProc("IsZoomed")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procGetSystemMetricsForDpi        = user32.NewProc("GetSystemMetricsForDpi")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procMonitorFromWindow             = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	procGetCursorPos                  = user32.NewProc("GetCursorPos")

	procGetModuleHandleW  = kernel32.NewProc("GetModuleHandleW")
	procCreateSolidBrush  = gdi32.NewProc("CreateSolidBrush")
	procCoInitializeEx    = ole32.NewProc("CoInitializeEx")
	procCoTaskMemFree     = ole32.NewProc("CoTaskMemFree")
	procDwmExtendFrame    = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	procDwmSetWindowAttr  = dwmapi.NewProc("DwmSetWindowAttribute")
	procRegOpenKeyExW     = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW  = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey       = advapi32.NewProc("RegCloseKey")
	procGetOpenFileNameW  = comdlg32.NewProc("GetOpenFileNameW")
	procSHBrowseForFolder = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromID   = shell32.NewProc("SHGetPathFromIDListW")

	winMu            sync.Mutex
	winHwnd          uintptr
	winController    uintptr
	winWebView       uintptr
	winIsFullscreen  bool
	winSavedStyle    uintptr
	winSavedRect     winRect
	winTargetURL     string
	winFallbackProc  *os.Process
	globalEnvHandler *comCallbackHandler
	globalCtlHandler *comCallbackHandler
	globalMsgHandler *comCallbackHandler
)

const (
	wmDestroy       = 0x0002
	wmSize          = 0x0005
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x0014
	wmGetMinMaxInfo = 0x0024
	wmNCCalcSize    = 0x0083
	wmNCHitTest     = 0x0084
	wmNCLButtonDown = 0x00A1
	wmSysCommand    = 0x0112
	wmAppDrag       = 0x8001
	wmAppFullscreen = 0x8002

	wsOverlappedWindow = 0x00CF0000
	wsClipChildren     = 0x02000000
	wsPopup            = 0x80000000
	wsVisible          = 0x10000000

	swShow        = 5
	swMaximize    = 3
	swRestore     = 9
	swpFrameChg   = 0x0020
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	swpNoMove     = 0x0002
	swpNoSize     = 0x0001

	htClient      = 1
	htCaption     = 2
	htLeft        = 10
	htRight       = 11
	htTop         = 12
	htTopLeft     = 13
	htTopRight    = 14
	htBottom      = 15
	htBottomLeft  = 16
	htBottomRight = 17

	smCxScreen       = 0
	smCyScreen       = 1
	smCxSizeFrame    = 32
	smCySizeFrame    = 33
	smCxPaddedBorder = 92

	imageIcon = 1
	idcArrow  = 32512
)

type winPoint struct {
	X, Y int32
}

type winRect struct {
	Left, Top, Right, Bottom int32
}

type ncCalcSizeParams struct {
	Rgrc  [3]winRect
	Lppos uintptr
}

type minMaxInfo struct {
	PtReserved     winPoint
	PtMaxSize      winPoint
	PtMaxPosition  winPoint
	PtMinTrackSize winPoint
	PtMaxTrackSize winPoint
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
}

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
}

type dwmMargins struct {
	CxLeftWidth    int32
	CxRightWidth   int32
	CyTopHeight    int32
	CyBottomHeight int32
}

type comHandlerVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Invoke         uintptr
}

type comCallbackHandler struct {
	vtbl *comHandlerVtbl
}

type winGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

func gwlStyleIndex() uintptr {
	idx := -16
	return uintptr(idx)
}

func getWindowDPI(hwnd uintptr) uint32 {
	if procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(hwnd); d > 0 {
			return uint32(d)
		}
	}
	return 96
}

func getMetricForDPI(metric int, dpi uint32) int32 {
	if procGetSystemMetricsForDpi.Find() == nil {
		v, _, _ := procGetSystemMetricsForDpi.Call(uintptr(metric), uintptr(dpi))
		return int32(v)
	}
	v, _, _ := procGetSystemMetrics.Call(uintptr(metric))
	return int32(v)
}

func syncWebViewBounds(hwnd uintptr) {
	winMu.Lock()
	ctl := winController
	winMu.Unlock()
	if ctl == 0 || hwnd == 0 {
		return
	}
	var rc winRect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	vtbl := *(*[30]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(ctl))))
	if runtime.GOARCH == "arm64" {
		lo := uint64(uint32(rc.Left)) | (uint64(uint32(rc.Top)) << 32)
		hi := uint64(uint32(rc.Right)) | (uint64(uint32(rc.Bottom)) << 32)
		_, _, _ = syscall.SyscallN(vtbl[6], ctl, uintptr(lo), uintptr(hi))
	} else {
		_, _, _ = syscall.SyscallN(vtbl[6], ctl, uintptr(unsafe.Pointer(&rc)))
	}
	_, _, _ = syscall.SyscallN(vtbl[4], ctl, 1) // put_IsVisible(TRUE)
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmNCCalcSize:
		if wParam != 0 {
			winMu.Lock()
			fs := winIsFullscreen
			winMu.Unlock()
			if fs {
				return 0
			}
			params := (*ncCalcSizeParams)(unsafe.Pointer(lParam))
			origTop := params.Rgrc[0].Top
			ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
			if z, _, _ := procIsZoomed.Call(hwnd); z != 0 {
				dpi := getWindowDPI(hwnd)
				frameY := getMetricForDPI(smCySizeFrame, dpi) + getMetricForDPI(smCxPaddedBorder, dpi)
				params.Rgrc[0].Top = origTop + frameY
			} else {
				params.Rgrc[0].Top = origTop
			}
			return ret
		}

	case wmNCHitTest:
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		if ret != htClient {
			return ret
		}
		if z, _, _ := procIsZoomed.Call(hwnd); z != 0 {
			return htClient
		}
		winMu.Lock()
		fs := winIsFullscreen
		winMu.Unlock()
		if fs {
			return htClient
		}
		var wr winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
		var pt winPoint
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		dpi := getWindowDPI(hwnd)
		border := int32((6*dpi + 48) / 96)
		if pt.Y >= wr.Top && pt.Y < wr.Top+border {
			if pt.X < wr.Left+border*2 {
				return htTopLeft
			}
			if pt.X >= wr.Right-border*2 {
				return htTopRight
			}
			return htTop
		}
		return htClient

	case wmGetMinMaxInfo:
		mmi := (*minMaxInfo)(unsafe.Pointer(lParam))
		mmi.PtMinTrackSize.X = 860
		mmi.PtMinTrackSize.Y = 560
		return 0

	case wmEraseBkgnd:
		return 1

	case wmSize:
		syncWebViewBounds(hwnd)
		return 0

	case wmAppDrag:
		procReleaseCapture.Call()
		procSendMessageW.Call(hwnd, wmNCLButtonDown, htCaption, 0)
		return 0

	case wmAppFullscreen:
		toggleHostFullscreenOnUIThread(hwnd)
		return 0

	case wmClose:
		winMu.Lock()
		ctl := winController
		winController = 0
		winWebView = 0
		winMu.Unlock()
		if ctl != 0 {
			vtbl := *(*[30]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(ctl))))
			_, _, _ = syscall.SyscallN(vtbl[24], ctl) // Close()
			_, _, _ = syscall.SyscallN(vtbl[2], ctl)  // Release()
		}
		procDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		winMu.Lock()
		winHwnd = 0
		winMu.Unlock()
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

func toggleHostFullscreenOnUIThread(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	styleIdx := gwlStyleIndex()
	winMu.Lock()
	currentlyFS := winIsFullscreen
	winMu.Unlock()

	if !currentlyFS {
		style, _, _ := procGetWindowLongPtrW.Call(hwnd, styleIdx)
		var wr winRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))

		hMon, _, _ := procMonitorFromWindow.Call(hwnd, 2)
		var mi monitorInfo
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		if hMon != 0 {
			procGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
		}
		winMu.Lock()
		winSavedStyle = style
		winSavedRect = wr
		winIsFullscreen = true
		winMu.Unlock()

		procSetWindowLongPtrW.Call(hwnd, styleIdx, (style&^wsOverlappedWindow)|wsPopup|wsVisible)
		mw := mi.RcMonitor.Right - mi.RcMonitor.Left
		mh := mi.RcMonitor.Bottom - mi.RcMonitor.Top
		if mw > 0 && mh > 0 {
			procSetWindowPos.Call(hwnd, 0, uintptr(mi.RcMonitor.Left), uintptr(mi.RcMonitor.Top), uintptr(mw), uintptr(mh), swpNoZOrder|swpFrameChg)
		}
	} else {
		winMu.Lock()
		savedStyle := winSavedStyle
		savedRect := winSavedRect
		winIsFullscreen = false
		winMu.Unlock()

		if savedStyle == 0 {
			savedStyle = wsOverlappedWindow | wsClipChildren | wsVisible
		}
		procSetWindowLongPtrW.Call(hwnd, styleIdx, savedStyle|wsVisible)
		w := savedRect.Right - savedRect.Left
		h := savedRect.Bottom - savedRect.Top
		if w > 100 && h > 100 {
			procSetWindowPos.Call(hwnd, 0, uintptr(savedRect.Left), uintptr(savedRect.Top), uintptr(w), uintptr(h), swpNoZOrder|swpFrameChg)
		} else {
			procShowWindow.Call(hwnd, swRestore)
		}
	}
	syncWebViewBounds(hwnd)
}

func StartNativeWindowDrag() {
	winMu.Lock()
	hwnd := winHwnd
	winMu.Unlock()
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmAppDrag, 0, 0)
	}
}

func ToggleNativeWindowFullscreen() {
	winMu.Lock()
	hwnd := winHwnd
	winMu.Unlock()
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmAppFullscreen, 0, 0)
	}
}

func TerminateNativeWindow() {
	winMu.Lock()
	hwnd := winHwnd
	p := winFallbackProc
	winFallbackProc = nil
	winMu.Unlock()
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
	if p != nil {
		_ = p.Kill()
	}
}

func queryRegString(root uintptr, subKey, valueName string) string {
	subKeyUTF16, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return ""
	}
	var hKey uintptr
	r, _, _ := procRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(subKeyUTF16)), 0, 0x20019, uintptr(unsafe.Pointer(&hKey)))
	if r != 0 || hKey == 0 {
		return ""
	}
	defer procRegCloseKey.Call(hKey)

	valNameUTF16, _ := syscall.UTF16PtrFromString(valueName)
	var dataType uint32
	var buf [512]uint16
	bufBytes := uint32(len(buf) * 2)
	r, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valNameUTF16)),
		0,
		uintptr(unsafe.Pointer(&dataType)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufBytes)),
	)
	if r != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:])
}

func findEmbeddedBrowserWebViewDLL() string {
	archDir := "x64"
	switch runtime.GOARCH {
	case "arm64":
		archDir = "arm64"
	case "386":
		archDir = "x86"
	}

	const (
		hklm = uintptr(0x80000002)
		hkcu = uintptr(0x80000001)
		guid = `{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`
	)
	regPaths := []struct {
		root uintptr
		key  string
	}{
		{hklm, `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\` + guid},
		{hklm, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + guid},
		{hkcu, `Software\Microsoft\EdgeUpdate\Clients\` + guid},
	}
	for _, rp := range regPaths {
		if eb := strings.TrimSpace(queryRegString(rp.root, rp.key, "EBWebView")); eb != "" {
			candidate := filepath.Join(eb, "EBWebView", archDir, "EmbeddedBrowserWebView.dll")
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}

	roots := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "EdgeWebView", "Application"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "EdgeWebView", "Application"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "EdgeWebView", "Application"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "EdgeCore"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "EdgeCore"),
	}
	for _, base := range roots {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
		for _, name := range names {
			candidate := filepath.Join(base, name, "EBWebView", archDir, "EmbeddedBrowserWebView.dll")
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func comQueryInterface(this, riid, ppvObject uintptr) uintptr {
	if ppvObject != 0 {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
	}
	return 0
}

func comAddRefRelease(this uintptr) uintptr {
	return 1
}

func envCompletedInvoke(this, hr, envPtr uintptr) uintptr {
	if int32(hr) < 0 || envPtr == 0 {
		return 0
	}
	winMu.Lock()
	hwnd := winHwnd
	winMu.Unlock()
	if hwnd == 0 {
		return 0
	}
	vtbl := *(*[10]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(envPtr))))
	_, _, _ = syscall.SyscallN(vtbl[1], envPtr) // AddRef
	_, _, _ = syscall.SyscallN(vtbl[3], envPtr, hwnd, uintptr(unsafe.Pointer(globalCtlHandler)))
	return 0
}

func ctlCompletedInvoke(this, hr, ctlPtr uintptr) uintptr {
	if int32(hr) < 0 || ctlPtr == 0 {
		return 0
	}
	ctlVtbl := *(*[30]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(ctlPtr))))
	_, _, _ = syscall.SyscallN(ctlVtbl[1], ctlPtr) // AddRef

	var wvPtr uintptr
	_, _, _ = syscall.SyscallN(ctlVtbl[25], ctlPtr, uintptr(unsafe.Pointer(&wvPtr)))
	if wvPtr == 0 {
		return 0
	}
	wvVtbl := *(*[40]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(wvPtr))))
	_, _, _ = syscall.SyscallN(wvVtbl[1], wvPtr) // AddRef

	winMu.Lock()
	winController = ctlPtr
	winWebView = wvPtr
	hwnd := winHwnd
	targetURL := winTargetURL
	winMu.Unlock()

	// Enable non-client region support (-webkit-app-region: drag) via ICoreWebView2Settings9 if supported
	var settingsPtr uintptr
	_, _, _ = syscall.SyscallN(wvVtbl[3], wvPtr, uintptr(unsafe.Pointer(&settingsPtr)))
	if settingsPtr != 0 {
		sVtbl := *(*[20]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(settingsPtr))))
		// Disable default dialogs, status bar, and (outside dev mode) context menu & DevTools for native app feel
		_, _, _ = syscall.SyscallN(sVtbl[8], settingsPtr, 0)  // put_AreDefaultScriptDialogsEnabled(FALSE)
		_, _, _ = syscall.SyscallN(sVtbl[10], settingsPtr, 0) // put_IsStatusBarEnabled(FALSE)
		if !nativeWindowDevMode {
			_, _, _ = syscall.SyscallN(sVtbl[12], settingsPtr, 0) // put_AreDevToolsEnabled(FALSE)
			_, _, _ = syscall.SyscallN(sVtbl[14], settingsPtr, 0) // put_AreDefaultContextMenusEnabled(FALSE)
		}

		iidSettings9 := winGUID{
			Data1: 0x0528a73b,
			Data2: 0xe92d,
			Data3: 0x49f4,
			Data4: [8]byte{0x92, 0x7a, 0xe5, 0x47, 0xdd, 0xaf, 0xff, 0xc6},
		}
		var settings9Ptr uintptr
		qhr, _, _ := syscall.SyscallN(sVtbl[0], settingsPtr, uintptr(unsafe.Pointer(&iidSettings9)), uintptr(unsafe.Pointer(&settings9Ptr)))
		if int32(qhr) >= 0 && settings9Ptr != 0 {
			s9Vtbl := *(*[28]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(settings9Ptr))))
			_, _, _ = syscall.SyscallN(s9Vtbl[26], settings9Ptr, 1) // put_IsNonClientRegionSupportEnabled(TRUE)
			_, _, _ = syscall.SyscallN(s9Vtbl[2], settings9Ptr)
		}
		_, _, _ = syscall.SyscallN(sVtbl[2], settingsPtr)
	}

	// Register WebMessageReceived handler so window.chrome.webview.postMessage("quit") works directly
	var token int64
	_, _, _ = syscall.SyscallN(wvVtbl[34], wvPtr, uintptr(unsafe.Pointer(globalMsgHandler)), uintptr(unsafe.Pointer(&token)))

	// Inject window.__storytellerNativeHost = true before document creation
	initJS, _ := syscall.UTF16PtrFromString("window.__storytellerNativeHost=true;")
	_, _, _ = syscall.SyscallN(wvVtbl[29], wvPtr, uintptr(unsafe.Pointer(initJS)), 0)

	syncWebViewBounds(hwnd)
	urlUTF16, _ := syscall.UTF16PtrFromString(targetURL)
	_, _, _ = syscall.SyscallN(wvVtbl[5], wvPtr, uintptr(unsafe.Pointer(urlUTF16)))
	return 0
}

func msgReceivedInvoke(this, sender, argsPtr uintptr) uintptr {
	if argsPtr == 0 {
		return 0
	}
	argsVtbl := *(*[10]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(argsPtr))))
	var rawMsg *uint16
	hr, _, _ := syscall.SyscallN(argsVtbl[4], argsPtr, uintptr(unsafe.Pointer(&rawMsg))) // TryGetWebMessageAsString
	if int32(hr) >= 0 && rawMsg != nil {
		msg := syscall.UTF16ToString((*[256]uint16)(unsafe.Pointer(rawMsg))[:])
		procCoTaskMemFree.Call(uintptr(unsafe.Pointer(rawMsg)))
		switch msg {
		case "quit":
			TerminateNativeWindow()
		case "start-window-drag":
			StartNativeWindowDrag()
		}
	}
	return 0
}

func runNativeWin32WebView2Window(url, dllPath string, onWindowClosed func()) bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	} else if procSetProcessDPIAware.Find() == nil {
		procSetProcessDPIAware.Call()
	}
	procCoInitializeEx.Call(0, 0x2) // COINIT_APARTMENTTHREADED

	wvDLL, err := syscall.LoadDLL(dllPath)
	if err != nil {
		return false
	}
	procCreateEnv, err := wvDLL.FindProc("CreateWebViewEnvironmentWithOptionsInternal")
	if err != nil {
		return false
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	hIconBig, _, _ := procLoadImageW.Call(hInst, 1, imageIcon, 256, 256, 0)
	if hIconBig == 0 {
		hIconBig, _, _ = procLoadImageW.Call(hInst, 1, imageIcon, 48, 48, 0)
	}
	hIconSm, _, _ := procLoadImageW.Call(hInst, 1, imageIcon, 32, 32, 0)
	if hIconSm == 0 {
		hIconSm, _, _ = procLoadImageW.Call(hInst, 1, imageIcon, 16, 16, 0)
	}
	hCursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	hBrush, _, _ := procCreateSolidBrush.Call(0x00101214)

	className, _ := syscall.UTF16PtrFromString("StoryTellerNativeWindow")
	wcx := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInst,
		HIcon:         hIconBig,
		HCursor:       hCursor,
		HbrBackground: hBrush,
		LpszClassName: className,
		HIconSm:       hIconSm,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcx)))

	screenW, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCyScreen)
	winW := uintptr(1440)
	winH := uintptr(900)
	if screenW > 0 && winW > screenW*92/100 {
		winW = screenW * 92 / 100
	}
	if screenH > 0 && winH > screenH*90/100 {
		winH = screenH * 90 / 100
	}
	posX := uintptr(0)
	posY := uintptr(0)
	if screenW > winW {
		posX = (screenW - winW) / 2
	}
	if screenH > winH {
		posY = (screenH - winH) / 2
	}

	titleUTF16, _ := syscall.UTF16PtrFromString("Mojra")
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(titleUTF16)),
		wsOverlappedWindow|wsClipChildren,
		posX, posY, winW, winH,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return false
	}

	if procDwmSetWindowAttr.Find() == nil {
		dark := int32(1)
		procDwmSetWindowAttr.Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
		round := int32(2)
		procDwmSetWindowAttr.Call(hwnd, 33, uintptr(unsafe.Pointer(&round)), 4)
	}
	if procDwmExtendFrame.Find() == nil {
		margins := dwmMargins{CxLeftWidth: 0, CxRightWidth: 0, CyTopHeight: 1, CyBottomHeight: 0}
		procDwmExtendFrame.Call(hwnd, uintptr(unsafe.Pointer(&margins)))
	}

	winMu.Lock()
	winHwnd = hwnd
	winTargetURL = url
	winIsFullscreen = false
	winMu.Unlock()

	qiCB := syscall.NewCallback(comQueryInterface)
	arCB := syscall.NewCallback(comAddRefRelease)
	globalEnvHandler = &comCallbackHandler{
		vtbl: &comHandlerVtbl{
			QueryInterface: qiCB,
			AddRef:         arCB,
			Release:        arCB,
			Invoke:         syscall.NewCallback(envCompletedInvoke),
		},
	}
	globalCtlHandler = &comCallbackHandler{
		vtbl: &comHandlerVtbl{
			QueryInterface: qiCB,
			AddRef:         arCB,
			Release:        arCB,
			Invoke:         syscall.NewCallback(ctlCompletedInvoke),
		},
	}
	globalMsgHandler = &comCallbackHandler{
		vtbl: &comHandlerVtbl{
			QueryInterface: qiCB,
			AddRef:         arCB,
			Release:        arCB,
			Invoke:         syscall.NewCallback(msgReceivedInvoke),
		},
	}

	baseCacheDir, err := os.UserCacheDir()
	if err != nil || baseCacheDir == "" {
		baseCacheDir = os.TempDir()
	}
	profileDir := filepath.Join(baseCacheDir, "StoryTeller-WebView2")
	_ = os.MkdirAll(profileDir, 0700)
	profileUTF16, _ := syscall.UTF16PtrFromString(profileDir)

	hr, _, _ := procCreateEnv.Call(
		1,
		0,
		uintptr(unsafe.Pointer(profileUTF16)),
		0,
		uintptr(unsafe.Pointer(globalEnvHandler)),
	)
	if int32(hr) < 0 {
		procDestroyWindow.Call(hwnd)
		winMu.Lock()
		winHwnd = 0
		winMu.Unlock()
		return false
	}

	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChg)
	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)

	var m winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	if onWindowClosed != nil {
		onWindowClosed()
	}
	return true
}

func findWindowsChromiumHost() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	if p, err := exec.LookPath("msedge.exe"); err == nil {
		return p
	}
	if p, err := exec.LookPath("chrome.exe"); err == nil {
		return p
	}
	return ""
}

func RunNativeOSWindow(url string, onWindowClosed func()) bool {
	if dllPath := findEmbeddedBrowserWebViewDLL(); dllPath != "" {
		if runNativeWin32WebView2Window(url, dllPath, onWindowClosed) {
			return true
		}
	}

	if hostBin := findWindowsChromiumHost(); hostBin != "" {
		baseCacheDir, err := os.UserCacheDir()
		if err != nil || baseCacheDir == "" {
			baseCacheDir = os.TempDir()
		}
		profileDir := filepath.Join(baseCacheDir, "StoryTeller-AppHost")
		_ = os.MkdirAll(profileDir, 0700)

		cmd := exec.Command(
			hostBin,
			"--app="+url,
			"--new-window",
			"--window-size=1440,900",
			"--user-data-dir="+profileDir,
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-features=Translate,msEdgeSidebarV2",
		)
		if err := cmd.Start(); err == nil {
			winMu.Lock()
			winFallbackProc = cmd.Process
			winMu.Unlock()

			go func() {
				_ = cmd.Wait()
				if onWindowClosed != nil {
					onWindowClosed()
				}
			}()
			return true
		}
	}

	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err == nil {
		return true
	}
	return false
}

type openFileNameW struct {
	LStructSize       uint32
	HwndOwner         uintptr
	HInstance         uintptr
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         *uint16
	NMaxFile          uint32
	LpstrFileTitle    *uint16
	NMaxFileTitle     uint32
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}

type browseInfoW struct {
	HwndOwner      uintptr
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	IImage         int32
}

func makeUTF16Filter(parts ...string) *uint16 {
	var out []uint16
	for _, p := range parts {
		u, _ := syscall.UTF16FromString(p)
		out = append(out, u...)
	}
	out = append(out, 0)
	return &out[0]
}

// PickNativeBookPath opens a native Windows folder or file picker dialog.
func PickNativeBookPath(chooseDir bool) string {
	winMu.Lock()
	owner := winHwnd
	winMu.Unlock()

	if !chooseDir {
		var fileBuf [1024]uint16
		title, _ := syscall.UTF16PtrFromString("Wybierz książkę (.playbook / .zip)")
		ofn := openFileNameW{
			LStructSize: uint32(unsafe.Sizeof(openFileNameW{})),
			HwndOwner:   owner,
			LpstrFilter: makeUTF16Filter("Playbook (*.playbook;*.stbook;*.zip)", "*.playbook;*.stbook;*.zip", "Wszystkie pliki (*.*)", "*.*"),
			LpstrFile:   &fileBuf[0],
			NMaxFile:    uint32(len(fileBuf)),
			LpstrTitle:  title,
			Flags:       0x00081804, // OFN_EXPLORER | OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_HIDEREADONLY
		}
		r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
		if r == 0 {
			return ""
		}
		return syscall.UTF16ToString(fileBuf[:])
	}

	var dispBuf [260]uint16
	title, _ := syscall.UTF16PtrFromString("Wybierz folder książki (zawierający pliki .playtxt)")
	bi := browseInfoW{
		HwndOwner:      owner,
		PszDisplayName: &dispBuf[0],
		LpszTitle:      title,
		UlFlags:        0x00000041, // BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE
	}
	pidl, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(pidl)

	var pathBuf [520]uint16
	ok, _, _ := procSHGetPathFromID.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(pathBuf[:])
}

// PickNativeMediaPaths opens a native Windows multi-file picker dialog for images or audio.
func PickNativeMediaPaths(kind string) []string {
	winMu.Lock()
	owner := winHwnd
	winMu.Unlock()

	filter := makeUTF16Filter("Obrazy i nagrania (*.png;*.jpg;*.jpeg;*.webp;*.gif;*.ogg;*.flac;*.mp3)", "*.png;*.jpg;*.jpeg;*.webp;*.gif;*.ogg;*.flac;*.mp3")
	titleStr := "Wybierz obrazy lub nagrania audio z dysku"
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "image", "images":
		filter = makeUTF16Filter("Obrazy (*.png;*.jpg;*.jpeg;*.webp;*.gif)", "*.png;*.jpg;*.jpeg;*.webp;*.gif")
		titleStr = "Wybierz obrazy ilustracji z dysku"
	case "audio", "voice":
		filter = makeUTF16Filter("Nagrania audio (*.ogg;*.flac;*.mp3)", "*.ogg;*.flac;*.mp3")
		titleStr = "Wybierz nagrania lektorskie z dysku (.ogg / .flac / .mp3)"
	}

	fileBuf := make([]uint16, 16384)
	title, _ := syscall.UTF16PtrFromString(titleStr)
	ofn := openFileNameW{
		LStructSize: uint32(unsafe.Sizeof(openFileNameW{})),
		HwndOwner:   owner,
		LpstrFilter: filter,
		LpstrFile:   &fileBuf[0],
		NMaxFile:    uint32(len(fileBuf)),
		LpstrTitle:  title,
		Flags:       0x00081A04, // OFN_EXPLORER | OFN_ALLOWMULTISELECT | OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_HIDEREADONLY
	}
	r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return nil
	}

	var tokens []string
	start := 0
	for i := 0; i < len(fileBuf); i++ {
		if fileBuf[i] == 0 {
			if i == start {
				break
			}
			tokens = append(tokens, syscall.UTF16ToString(fileBuf[start:i]))
			start = i + 1
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	if len(tokens) == 1 {
		return tokens
	}
	dir := tokens[0]
	out := make([]string, 0, len(tokens)-1)
	for _, name := range tokens[1:] {
		out = append(out, filepath.Join(dir, name))
	}
	return out
}
