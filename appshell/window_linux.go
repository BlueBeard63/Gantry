//go:build linux && !android && !nogui

package appshell

import (
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"

	webview "github.com/B-Commissions/Gantry/internal/webview"
)

// The main window's live handles, so tray Quit and Go code can reach
// it from other goroutines (Dispatch marshals onto the GTK thread).
var (
	mainMu     sync.Mutex
	mainWV     webview.WebView
	mainWin    unsafe.Pointer
	forceClose atomic.Bool
)

func mainHandles() (webview.WebView, unsafe.Pointer) {
	mainMu.Lock()
	defer mainMu.Unlock()
	return mainWV, mainWin
}

// CloseMainWindow closes the main window unconditionally, bypassing
// DisableClose and OnCloseRequest - the "tray Quit means quit" path.
func CloseMainWindow() {
	wv, win := mainHandles()
	if wv == nil {
		return
	}
	forceClose.Store(true)
	wv.Dispatch(func() { gtkClose(win) })
}

// ShowMainWindow reveals and foregrounds the main window if it exists.
func ShowMainWindow() bool {
	wv, win := mainHandles()
	if wv == nil {
		return false
	}
	wv.Dispatch(func() {
		gtkShow(win)
		gtkPresent(win)
	})
	return true
}

// AttentionMainWindow sets the urgency hint - most Linux shells bounce
// or highlight the taskbar entry until the window is focused.
func AttentionMainWindow() {
	wv, win := mainHandles()
	if wv == nil {
		return
	}
	wv.Dispatch(func() { gtkSetUrgency(win, true) })
}

// RunWindow opens the app's main native window (WebKitGTK) and blocks
// until it closes. It MUST run on the main goroutine. Frameless mode,
// dragging, sizing limits, the close hook and geometry persistence all
// work like on Windows; edge resizing is driven by the frontend's
// ResizeFrame (the compositor has no invisible frame to hit-test, so
// gantry-web renders thin resize strips and calls the ResizeEdge
// binding).
func RunWindow(opts WindowOptions) error {
	if err := opts.normalize(); err != nil {
		return err
	}

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

	linuxEnvFixes()
	w := webview.New(opts.Debug)
	if w == nil {
		return errors.New("appshell: webkit2gtk unavailable")
	}
	defer w.Destroy()
	w.SetTitle(opts.Title)
	w.SetSize(opts.Width, opts.Height, webview.HintNone)

	win := w.Window()
	forceClose.Store(false)
	mainMu.Lock()
	mainWV, mainWin = w, win
	mainMu.Unlock()
	defer func() {
		mainMu.Lock()
		mainWV, mainWin = nil, nil
		mainMu.Unlock()
		deleteHook.Store(nil)
	}()

	if !opts.Framed {
		gtkSetDecorated(win, false)
	}
	gtkSetMinMax(win, opts.MinWidth, opts.MinHeight, opts.MaxWidth, opts.MaxHeight)

	saveGeometry := func() {}
	if opts.Geometry != nil {
		store := opts.Geometry
		saveGeometry = func() {
			store.Save(gtkGetGeometry(win), gtkIsMaximized(win))
		}
	}
	deleteHook.Store(&deleteConfig{
		disableClose: opts.DisableClose,
		onClose:      opts.OnCloseRequest,
		force:        &forceClose,
		onClosing:    saveGeometry,
	})
	gtkConnectDelete(win)

	prefix := opts.BindingPrefix
	_ = w.Bind(prefix+"Close", func() { w.Dispatch(func() { gtkClose(win) }) })
	_ = w.Bind(prefix+"Drag", func() { w.Dispatch(func() { gtkBeginMoveDrag(win) }) })
	_ = w.Bind(prefix+"Attention", func() { w.Dispatch(func() { gtkSetUrgency(win, true) }) })
	_ = w.Bind(prefix+"OpenExternal", func(url string) { _ = OpenInBrowser(url) })
	if !opts.DisableMinimize {
		_ = w.Bind(prefix+"Minimize", func() { w.Dispatch(func() { gtkIconify(win) }) })
	}
	if opts.EnableMaximize {
		_ = w.Bind(prefix+"Maximize", func() { w.Dispatch(func() { gtkMaximize(win) }) })
		_ = w.Bind(prefix+"Restore", func() { w.Dispatch(func() { gtkUnmaximize(win) }) })
		_ = w.Bind(prefix+"IsMaximized", func() bool { return gtkIsMaximized(win) })
	}
	if !opts.Framed {
		// The frontend's ResizeFrame drives edge resizing.
		_ = w.Bind(prefix+"ResizeEdge", func(edge string) {
			w.Dispatch(func() { gtkBeginResizeDrag(win, edge) })
		})
	}
	_ = w.Bind(prefix+"SetAlwaysOnTop", func(on bool) {
		w.Dispatch(func() { gtkSetKeepAbove(win, on) })
	})
	_ = w.Bind(prefix+"Caps", func() map[string]any {
		return map[string]any{
			"minimize":    !opts.DisableMinimize,
			"maximize":    opts.EnableMaximize,
			"close":       !opts.DisableClose,
			"alwaysOnTop": opts.AlwaysOnTop,
			"platform":    "linux",
			"frameless":   !opts.Framed,
		}
	})
	for name, fn := range opts.ExtraBindings {
		_ = w.Bind(name, fn)
	}

	gtkWatchWebProcess(win)
	if explicitPos {
		gtkMove(win, opts.X, opts.Y)
	} else {
		x, y := centerPos(opts.Width, opts.Height)
		gtkMove(win, x, y)
	}
	if opts.AlwaysOnTop {
		gtkSetKeepAbove(win, true)
	}
	if startMaximized && opts.EnableMaximize {
		gtkMaximize(win)
	}

	w.Navigate(opts.URL)
	w.Run()
	return nil
}
