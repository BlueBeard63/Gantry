//go:build windows && !nogui

package appshell

import (
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

// RunPopup opens a frameless, always-on-top notification window at the
// top or bottom of the chosen monitor and blocks until it closes. It
// never steals focus: buttons stay clickable but whatever the user was
// typing in keeps the keyboard. Run it in a dedicated child process (see
// notify.Notifier) so a webview crash cannot take the host app down.
//
// When the WebView2 runtime is unavailable the same URL opens in the
// default browser instead.
func RunPopup(opts PopupOptions) error {
	if err := opts.normalize(); err != nil {
		return err
	}

	// Capture compatibility rides in on the environment: the main process
	// sets WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS and this child inherited
	// it; the env-var override still applies for a standalone run.
	windowsEnvFixes(false)

	// The popup is normally a SEPARATE process from the main window: it
	// must use its own WebView2 user-data folder, or environment
	// creation fails while the main window holds the default one -
	// historically the popup then never appeared at all.
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: false,
		DataPath:  webviewDataPath(opts.AppName, opts.DataDirRole),
		WindowOptions: webview2.WindowOptions{
			Title:  opts.AppName,
			Width:  uint(opts.Width),
			Height: uint(opts.Height),
		},
	})
	if w == nil {
		return OpenInBrowser(opts.URL)
	}
	defer w.Destroy()

	hwnd := uintptr(w.Window())
	disableStatusBar(w)
	prefix := opts.BindingPrefix
	_ = w.Bind(prefix+"Close", func() { closeWindow(hwnd) })
	_ = w.Bind(prefix+"OpenExternal", func(url string) { _ = OpenInBrowser(url) })
	// The page can hide itself the instant it is answered/dismissed:
	// the popup process is then torn down by its owner, and destroying
	// a hidden window can never paint a flash frame.
	_ = w.Bind(prefix+"Visible", func(show bool) { setVisible(hwnd, show) })
	for name, fn := range opts.ExtraBindings {
		_ = w.Bind(name, fn)
	}
	w.Navigate(opts.URL)

	x, y := popupPos(opts)

	// Strip the frame, pin topmost WITHOUT taking focus, position on the
	// work area (above the taskbar).
	applyIcon(hwnd, opts.Icon)
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, wsPopup|wsVisible)
	// OR into the existing extended style - replacing it strips
	// WS_EX_NOREDIRECTIONBITMAP and the WebView2 content renders white.
	exStyle, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, exStyle|wsExTopmost|wsExNoActivate)
	pref := int32(dwmwcpRound)
	procDwmSetWindowAttribute.Call(hwnd, dwmwaCornerPref,
		uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
	procSetWindowPos.Call(hwnd, hwndTopmost,
		uintptr(x), uintptr(y), uintptr(opts.Width), uintptr(opts.Height),
		swpFrameChange|swpShowWindow|swpNoActivate)

	w.Run()
	return nil
}

// FindWindowVisible reports whether a window with the given title exists
// and is currently visible - useful in PopupOptions.AdjustPos to slide a
// popup below a widget that parks in the same spot.
func FindWindowVisible(title string) bool {
	h, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(title))))
	if h == 0 {
		return false
	}
	vis, _, _ := procIsWindowVisible.Call(h)
	return vis != 0
}
