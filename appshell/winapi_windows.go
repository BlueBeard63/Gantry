//go:build windows

package appshell

import (
	"os"
	"path/filepath"
	"syscall"
)

// All Win32 handles and constants used by the shell live here so each
// window file reads as logic, not plumbing.

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procShowWindow               = user32.NewProc("ShowWindow")
	procIsZoomed                 = user32.NewProc("IsZoomed")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW        = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procCallWindowProcW          = user32.NewProc("CallWindowProcW")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procReleaseCapture           = user32.NewProc("ReleaseCapture")
	procFlashWindowEx            = user32.NewProc("FlashWindowEx")
	procFindWindowW              = user32.NewProc("FindWindowW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procMonitorFromWindow        = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")

	shell32            = syscall.NewLazyDLL("shell32.dll")
	procExtractIconExW = shell32.NewProc("ExtractIconExW")
	procShellExecuteW  = shell32.NewProc("ShellExecuteW")

	winmm          = syscall.NewLazyDLL("winmm.dll")
	procPlaySoundW = winmm.NewProc("PlaySoundW")

	dwmapi                    = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

// GWL/GWLP indices are negative and MUST be sign-extended to the full
// uintptr width on 64-bit - passing the 32-bit two's-complement value
// (e.g. 0xFFFFFFF0) makes Set/GetWindowLongPtrW fail silently, which
// historically meant the caption was never removed (white bar) and the
// subclass never installed (no dragging).
var (
	gwlStyle    = ^uintptr(15) // GWL_STYLE    (-16)
	gwlpWndProc = ^uintptr(3)  // GWLP_WNDPROC (-4)
	gwlExStyle  = ^uintptr(19) // GWL_EXSTYLE  (-20)
	hwndTopmost = ^uintptr(0)  // (HWND)-1
	// hwndNotopmost is (HWND)-2.
	hwndNotopmost = ^uintptr(1)
)

const (
	swHide     = 0
	swMinimize = 6
	swShowNA   = 8
	swMaximize = 3
	swRestore  = 9

	wmClose          = 0x0010
	wmActivate       = 0x0006
	wmGetMinMaxInfo  = 0x0024
	wmSetIcon        = 0x0080
	wmNCCalcSize     = 0x0083
	wmNCHitTest      = 0x0084
	wmNCLButtonDown  = 0x00A1

	htClient  = 1
	htCaption = 2

	iconSmall = 0
	iconBig   = 1

	wsCaption     = 0x00C00000
	wsMaximizeBox = 0x00010000
	wsMinimizeBox = 0x00020000
	wsPopup       = 0x80000000
	wsVisible     = 0x10000000

	// WS_EX_NOACTIVATE: the window never steals focus - its buttons stay
	// clickable but whatever you were typing in keeps the keyboard.
	wsExNoActivate = 0x08000000
	wsExTopmost    = 0x00000008

	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
	swpFrameChange = 0x0020
	swpShowWindow  = 0x0040

	// DWMWA_WINDOW_CORNER_PREFERENCE / DWMWCP_ROUND: Win11 rounded corners.
	dwmwaCornerPref = 33
	dwmwcpRound     = 2

	monitorDefaultToNearest = 2
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

type winPoint struct {
	X, Y int32
}

// minMaxInfo is the WM_GETMINMAXINFO payload.
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

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

// webviewDataPath gives each app AND each window role its own WebView2
// user-data folder under %LocalAppData%: sharing one folder across
// processes makes the second environment fail to initialize, and sharing
// across apps would mix cookies and caches.
func webviewDataPath(appName, role string) string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(base, appName, "webview-"+role)
	_ = os.MkdirAll(p, 0o700)
	return p
}

// closeWindow asks a window to close the same way the native close
// button does - asynchronously via WM_CLOSE. Calling webview Terminate()
// from inside a JS binding crashes the app (teardown mid-dispatch), so
// every close in this package goes through here.
func closeWindow(hwnd uintptr) {
	procPostMessageW.Call(hwnd, wmClose, 0, 0)
}

// dragWindow hands the window to the native move loop. The WebView2
// child window covers the whole client area, so the parent's
// WM_NCHITTEST caption band never sees the mouse - the web top bar calls
// this on mousedown instead: releasing Chromium's capture and posting a
// caption-drag starts the native move. PostMessage, not SendMessage -
// entering the modal move loop from inside a JS-dispatch stack is the
// Terminate()-style crash again.
func dragWindow(hwnd uintptr) {
	procReleaseCapture.Call()
	procPostMessageW.Call(hwnd, wmNCLButtonDown, htCaption, 0)
}

// setVisible shows (without activating) or hides a window. Hiding MUST
// go through ShowWindow: clearing WS_VISIBLE with a raw style write
// hides the window logically without telling the DWM, which keeps
// compositing its last (unpainted, white) frame as an untouchable ghost.
func setVisible(hwnd uintptr, show bool) {
	if show {
		procShowWindow.Call(hwnd, swShowNA)
	} else {
		procShowWindow.Call(hwnd, swHide)
	}
}
