//go:build windows && !nogui

package appshell

import (
	"errors"
	"sync/atomic"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

// mainWindowHwnd tracks the main UI window while it is open, so tray
// Quit can close it and Go code can flash/show it (RunWindow blocks the
// main goroutine - a shutdown select can't run until the window goes
// away).
var (
	mainWindowHwnd atomic.Uintptr
	forceClose     atomic.Bool
)

var (
	procIsIconic            = user32.NewProc("IsIconic")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

// CloseMainWindow closes the main window unconditionally, bypassing
// DisableClose and OnCloseRequest - the "tray Quit means quit" path.
func CloseMainWindow() {
	if h := mainWindowHwnd.Load(); h != 0 {
		forceClose.Store(true)
		closeWindow(h)
	}
}

// ShowMainWindow reveals and foregrounds the main window if it exists
// (hidden by a CloseHide policy, or minimized). Returns false when no
// window is alive - the caller should open a fresh one.
func ShowMainWindow() bool {
	h := mainWindowHwnd.Load()
	if h == 0 {
		return false
	}
	const swShow = 5
	if iconic, _, _ := procIsIconic.Call(h); iconic != 0 {
		procShowWindow.Call(h, swRestore)
	} else {
		procShowWindow.Call(h, swShow)
	}
	procSetForegroundWindow.Call(h)
	return true
}

// RunWindow opens the app's main native window and blocks until it
// closes. It MUST run on the main goroutine (the webview message loop
// requires it); run everything else on other goroutines.
//
// By default the window is frameless: the web frontend draws its own
// chrome (gantry-web's TitleBar) and drives the window through the
// bound <prefix>* functions, while this side re-implements edge
// resizing and the draggable caption band natively.
func RunWindow(opts WindowOptions) error {
	if err := opts.normalize(); err != nil {
		return err
	}

	// Saved geometry (if any) wins over the configured size/position.
	var (
		haveGeometry   bool
		startMaximized bool
		rect           Rect
	)
	if opts.Geometry != nil {
		if r, max, ok := opts.Geometry.Load(); ok {
			rect, startMaximized, haveGeometry = r, max, true
			opts.Width, opts.Height = r.Width, r.Height
		}
	}
	explicitPos := haveGeometry || opts.X != 0 || opts.Y != 0
	if haveGeometry {
		opts.X, opts.Y = rect.X, rect.Y
	}

	windowsEnvFixes(opts.CaptureCompatible)
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     opts.Debug,
		AutoFocus: opts.AutoFocus,
		DataPath:  webviewDataPath(opts.AppName, opts.DataDirRole),
		WindowOptions: webview2.WindowOptions{
			Title:  opts.Title,
			Width:  uint(opts.Width),
			Height: uint(opts.Height),
			Center: !explicitPos,
		},
	})
	if w == nil {
		return errors.New("appshell: WebView2 runtime unavailable")
	}
	defer w.Destroy()

	hwnd := uintptr(w.Window())
	forceClose.Store(false)
	mainWindowHwnd.Store(hwnd)
	defer mainWindowHwnd.Store(0)
	disableStatusBar(w)

	prefix := opts.BindingPrefix
	_ = w.Bind(prefix+"Close", func() { closeWindow(hwnd) })
	_ = w.Bind(prefix+"Drag", func() { dragWindow(hwnd) })
	_ = w.Bind(prefix+"Attention", func() { attention(hwnd) })
	// External links open in the user's default browser, never inside
	// the app window (gantry-web's ExternalLink uses this).
	_ = w.Bind(prefix+"OpenExternal", func(url string) { _ = OpenInBrowser(url) })
	// ShowWindowAsync, never ShowWindow: these run inside a JS-binding
	// dispatch, and a synchronous show-state change re-enters the
	// WndProc (WM_SIZE/WM_NCCALCSIZE cascade) mid-dispatch - the same
	// crash family as Terminate-in-binding, seen as intermittent
	// crashes when window buttons are hit during rapid UI activity.
	if !opts.DisableMinimize {
		_ = w.Bind(prefix+"Minimize", func() { procShowWindowAsync.Call(hwnd, swMinimize) })
	}
	if opts.EnableMaximize {
		_ = w.Bind(prefix+"Maximize", func() { procShowWindowAsync.Call(hwnd, swMaximize) })
		_ = w.Bind(prefix+"Restore", func() { procShowWindowAsync.Call(hwnd, swRestore) })
		_ = w.Bind(prefix+"IsMaximized", func() bool {
			z, _, _ := procIsZoomed.Call(hwnd)
			return z != 0
		})
	}
	if !opts.Framed {
		// The frontend's ResizeFrame draws the edge strips (and their
		// resize cursors) and calls this on mousedown - the WebView2 child
		// covers the client area, so the parent's WM_NCHITTEST resize
		// margin never sees the hover.
		_ = w.Bind(prefix+"ResizeEdge", func(edge string) { resizeWindow(hwnd, edge) })
	}
	_ = w.Bind(prefix+"SetAlwaysOnTop", func(on bool) {
		z := hwndNotopmost
		if on {
			z = hwndTopmost
		}
		procSetWindowPos.Call(hwnd, z, 0, 0, 0, 0, swpNoMove|swpNoSize)
	})
	// Caps tells the frontend which window buttons this window actually
	// supports, so the TitleBar renders exactly those - no manual
	// mirroring of Go options into React props.
	_ = w.Bind(prefix+"Caps", func() map[string]any {
		return map[string]any{
			"minimize":    !opts.DisableMinimize,
			"maximize":    opts.EnableMaximize,
			"close":       !opts.DisableClose,
			"alwaysOnTop": opts.AlwaysOnTop,
			"platform":    "windows",
			"frameless":   !opts.Framed,
		}
	})
	for name, fn := range opts.ExtraBindings {
		_ = w.Bind(name, fn)
	}

	w.Navigate(opts.URL)

	// Style pass: frameless clears the caption; the maximize/minimize
	// boxes go by capability (clearing the maximize box also stops a
	// caption-band double-click from maximizing).
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	if !opts.Framed {
		style = style &^ uintptr(wsCaption)
	}
	if !opts.EnableMaximize {
		style = style &^ uintptr(wsMaximizeBox)
	}
	if opts.DisableMinimize {
		style = style &^ uintptr(wsMinimizeBox)
	}
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, style)

	saveGeometry := func() {}
	if opts.Geometry != nil {
		store := opts.Geometry
		saveGeometry = func() {
			var r winRect
			procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			z, _, _ := procIsZoomed.Call(hwnd)
			store.Save(Rect{
				X: int(r.Left), Y: int(r.Top),
				Width: int(r.Right - r.Left), Height: int(r.Bottom - r.Top),
			}, z != 0)
		}
	}
	subclassWindow(hwnd, subclassConfig{
		frameless:           !opts.Framed,
		captionHeight:       opts.CaptionHeight,
		captionLeftReserve:  opts.CaptionLeftReserve,
		captionRightReserve: opts.CaptionRightReserve,
		resizeMargin:        opts.ResizeMargin,
		minWidth:            opts.MinWidth,
		minHeight:           opts.MinHeight,
		maxWidth:            opts.MaxWidth,
		maxHeight:           opts.MaxHeight,
		disableClose:        opts.DisableClose,
		onCloseRequest:      opts.OnCloseRequest,
		forceClose:          &forceClose,
		onClosing:           saveGeometry,
		onResize:            saveGeometry,
	})
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpFrameChange|swpNoMove|swpNoSize|swpNoZOrder)

	if explicitPos {
		procSetWindowPos.Call(hwnd, 0,
			uintptr(opts.X), uintptr(opts.Y),
			uintptr(opts.Width), uintptr(opts.Height), swpNoZOrder)
	}
	applyIcon(hwnd, opts.Icon)
	applyCorners(hwnd, opts.Corners)
	if opts.AlwaysOnTop {
		procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize)
	}
	if startMaximized && opts.EnableMaximize {
		procShowWindow.Call(hwnd, swMaximize)
	}

	w.Run()
	return nil
}
