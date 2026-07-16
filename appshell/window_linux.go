//go:build linux && !nogui

package appshell

import (
	"errors"

	webview "github.com/webview/webview_go"
)

// RunWindow opens the main window as a plain WebKitGTK window (title and
// size honored; the custom-chrome features are Windows-only for now).
// Blocks until the user closes the window.
func RunWindow(opts WindowOptions) error {
	if err := opts.normalize(); err != nil {
		return err
	}
	w := webview.New(opts.Debug)
	if w == nil {
		return errors.New("appshell: webkit2gtk unavailable")
	}
	defer w.Destroy()
	w.SetTitle(opts.Title)
	w.SetSize(opts.Width, opts.Height, webview.HintNone)
	w.Navigate(opts.URL)
	w.Run()
	return nil
}

// CloseMainWindow is a no-op on Linux (the WebKitGTK window closes
// itself).
func CloseMainWindow() {}

// ShowMainWindow always reports no window to reveal on Linux.
func ShowMainWindow() bool { return false }

// AttentionMainWindow is a no-op on Linux.
func AttentionMainWindow() {}
