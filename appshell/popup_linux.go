//go:build linux && !android && !nogui

package appshell

import (
	webview "github.com/BlueBeard63/Gantry/internal/webview"
)

// RunPopup opens a frameless, always-on-top notification window
// (WebKitGTK) at the top or bottom of the chosen monitor and blocks
// until it closes. It refuses focus, so whatever the user was typing
// in keeps the keyboard. Falls back to the default browser when
// WebKitGTK is unavailable.
func RunPopup(opts PopupOptions) error {
	if err := opts.normalize(); err != nil {
		return err
	}

	linuxEnvFixes()
	w := webview.New(false)
	if w == nil {
		return OpenInBrowser(opts.URL)
	}
	defer w.Destroy()
	w.SetTitle(opts.AppName)
	w.SetSize(opts.Width, opts.Height, webview.HintNone)

	win := w.Window()
	gtkSetDecorated(win, false)
	gtkSetKeepAbove(win, true)
	gtkSetSkipTaskbar(win, true)
	gtkSetAcceptFocus(win, false)

	prefix := opts.BindingPrefix
	_ = w.Bind(prefix+"Close", func() { w.Dispatch(func() { gtkClose(win) }) })
	_ = w.Bind(prefix+"Visible", func(show bool) {
		w.Dispatch(func() {
			if show {
				gtkShow(win)
			} else {
				gtkHide(win)
			}
		})
	})
	_ = w.Bind(prefix+"OpenExternal", func(url string) { _ = OpenInBrowser(url) })
	for name, fn := range opts.ExtraBindings {
		_ = w.Bind(name, fn)
	}

	gtkWatchWebProcess(win)
	x, y := popupPos(opts)
	gtkMove(win, x, y)

	w.Navigate(opts.URL)
	w.Run()
	return nil
}

// FindWindowVisible has no cheap cross-process equivalent on Linux;
// popups simply use their default position.
func FindWindowVisible(string) bool { return false }
