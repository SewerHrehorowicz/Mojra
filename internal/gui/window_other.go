//go:build (!darwin || !cgo) && !windows && !linux && !(js && wasm)

package gui

// On non-darwin or non-cgo builds (such as headless test runs), no external browser
// executable is ever spawned. Windows native builds use Win32 + WebView2.

func RunNativeOSWindow(url string, onClosed func()) bool {
	return false
}

func ToggleNativeWindowFullscreen() {}

func StartNativeWindowDrag() {}

func PickNativeBookPath(chooseDir bool) string {
	return ""
}

func PickNativeMediaPaths(kind string) []string {
	return nil
}

func TerminateNativeWindow() {}
