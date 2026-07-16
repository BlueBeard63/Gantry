//go:build nogui || (!windows && !linux)

package appshell

import "errors"

// RunWindow is unavailable in nogui builds; callers fall back to the
// browser (App.Run does this automatically).
func RunWindow(WindowOptions) error {
	return errors.New("appshell: built without native window support (nogui)")
}

// CloseMainWindow is a no-op without a native window host.
func CloseMainWindow() {}

// ShowMainWindow always reports no window to reveal.
func ShowMainWindow() bool { return false }

// AttentionMainWindow is a no-op without a native window host.
func AttentionMainWindow() {}
