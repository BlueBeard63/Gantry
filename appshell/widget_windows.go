//go:build windows && !nogui

package appshell

import (
	"errors"
	"syscall"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

// closeOnDeactivate dismisses the window when it loses activation (a
// flyout: click anywhere else and it goes away). Deactivations in the
// first moments after creation are ignored - window setup can briefly
// bounce activation around.
func closeOnDeactivate(hwnd uintptr) {
	born := time.Now()
	var orig uintptr
	proc := syscall.NewCallback(func(h, msg, wparam, lparam uintptr) uintptr {
		if msg == wmActivate && wparam&0xFFFF == 0 && time.Since(born) > 500*time.Millisecond {
			procPostMessageW.Call(h, wmClose, 0, 0)
		}
		ret, _, _ := procCallWindowProcW.Call(orig, h, msg, wparam, lparam)
		return ret
	})
	orig, _, _ = procSetWindowLongPtrW.Call(hwnd, gwlpWndProc, proc)
	_ = orig
}

// RunWidget opens a small always-on-top helper window and blocks until
// it closes. Run it in a dedicated child process (see ProcManager and
// the --shellrole pattern) so a webview crash cannot take the host app
// down.
func RunWidget(opts WidgetOptions) error {
	if err := opts.normalize(); err != nil {
		return err
	}

	// Exactly one widget per Title, ever: close any predecessor (e.g. an
	// orphan left behind by a killed app) before opening ours. The
	// distinct title keeps this from ever touching the main window.
	if prev, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(opts.Title)))); prev != 0 {
		closeWindow(prev)
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: false,
		DataPath:  webviewDataPath(opts.AppName, opts.DataDirRole),
		WindowOptions: webview2.WindowOptions{
			Title:  opts.Title,
			Width:  uint(opts.Width),
			Height: uint(opts.Height),
		},
	})
	if w == nil {
		return errors.New("appshell: WebView2 runtime unavailable")
	}
	defer w.Destroy()

	hwnd := uintptr(w.Window())
	disableStatusBar(w)
	prefix := opts.BindingPrefix
	_ = w.Bind(prefix+"Close", func() { closeWindow(hwnd) })
	_ = w.Bind(prefix+"Drag", func() { dragWindow(hwnd) })
	_ = w.Bind(prefix+"OpenExternal", func(url string) { _ = OpenInBrowser(url) })
	// The page drives visibility (e.g. a timer widget only appears while
	// something is running).
	_ = w.Bind(prefix+"Visible", func(show bool) { setVisible(hwnd, show) })
	// Resize in place - SWP_NOMOVE keeps whatever position the user
	// dragged the widget to (flyouts grow downward from where they are).
	_ = w.Bind(prefix+"Resize", func(width, height int) {
		if width <= 0 || height <= 0 {
			return
		}
		procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0,
			uintptr(width), uintptr(height), swpNoMove|swpNoActivate)
	})
	for name, fn := range opts.ExtraBindings {
		_ = w.Bind(name, fn)
	}
	w.Navigate(opts.URL)

	x, y := widgetPos(opts)

	applyIcon(hwnd, opts.Icon)
	// NEVER clear WS_VISIBLE via a raw style write: that hides the
	// window logically without telling the DWM, which keeps compositing
	// its last (unpainted, white) frame as an untouchable ghost on
	// screen. Hiding happens with a real ShowWindow(SW_HIDE) below.
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, wsPopup|wsVisible)
	// OR into the existing extended style - REPLACING it strips
	// WS_EX_NOREDIRECTIONBITMAP and WebView2's composited content goes
	// white (invisible page, busy cursor).
	exStyle, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	exStyle |= wsExTopmost
	flags := uintptr(swpFrameChange | swpShowWindow)
	if opts.NoActivate {
		exStyle |= wsExNoActivate // glanceable: never steals the keyboard
		flags |= swpNoActivate
	}
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, exStyle)
	if opts.CloseOnDeactivate {
		closeOnDeactivate(hwnd)
	}
	if !opts.SquareCorners {
		pref := int32(dwmwcpRound)
		procDwmSetWindowAttribute.Call(hwnd, dwmwaCornerPref,
			uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
	}
	procSetWindowPos.Call(hwnd, hwndTopmost,
		uintptr(x), uintptr(y), uintptr(opts.Width), uintptr(opts.Height), flags)
	if opts.StartHidden {
		// A REAL hide transition (DWM-notified): the page re-shows it
		// via the Visible binding when it has something to show.
		procShowWindow.Call(hwnd, swHide)
	}

	w.Run()
	return nil
}
