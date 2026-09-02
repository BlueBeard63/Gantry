//go:build linux && !android && !nogui

package appshell

import (
	"errors"

	webview "github.com/B-Commissions/Gantry/internal/webview"
)

// RunWidget opens a small always-on-top helper window (WebKitGTK) and
// blocks until it closes. Same contract as Windows; run it in a child
// process via ProcManager.
func RunWidget(opts WidgetOptions) error {
	if err := opts.normalize(); err != nil {
		return err
	}

	linuxEnvFixes()
	w := webview.New(false)
	if w == nil {
		return errors.New("appshell: webkit2gtk unavailable")
	}
	defer w.Destroy()
	w.SetTitle(opts.Title)
	w.SetSize(opts.Width, opts.Height, webview.HintNone)

	win := w.Window()
	gtkSetDecorated(win, false)
	gtkSetKeepAbove(win, true)
	gtkSetSkipTaskbar(win, true)
	if opts.NoActivate {
		gtkSetAcceptFocus(win, false)
	}
	if opts.CloseOnDeactivate {
		fn := func() { gtkClose(win) }
		focusOutHook.Store(&fn)
		gtkConnectFocusOut(win)
	}

	prefix := opts.BindingPrefix
	_ = w.Bind(prefix+"Close", func() { w.Dispatch(func() { gtkClose(win) }) })
	_ = w.Bind(prefix+"Drag", func() { w.Dispatch(func() { gtkBeginMoveDrag(win) }) })
	_ = w.Bind(prefix+"Visible", func(show bool) {
		w.Dispatch(func() {
			if show {
				gtkShow(win)
			} else {
				gtkHide(win)
			}
		})
	})
	_ = w.Bind(prefix+"Resize", func(width, height int) {
		if width <= 0 || height <= 0 {
			return
		}
		w.Dispatch(func() { gtkResize(win, width, height) })
	})
	_ = w.Bind(prefix+"OpenExternal", func(url string) { _ = OpenInBrowser(url) })
	for name, fn := range opts.ExtraBindings {
		_ = w.Bind(name, fn)
	}

	gtkWatchWebProcess(win)
	x, y := widgetPos(opts)
	gtkMove(win, x, y)
	if opts.StartHidden {
		gtkHide(win)
	}

	w.Navigate(opts.URL)
	w.Run()
	return nil
}
