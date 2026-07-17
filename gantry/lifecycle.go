package gantry

import "sync/atomic"

// closeToTray is read each time the main window closes (via the
// shell's KeepRunning hook), so flipping it takes effect on the very
// next close - no restart.
var closeToTray atomic.Bool

func init() { closeToTray.Store(true) }

// SetCloseToTray switches what closing the window does in a tray app:
// true (the default) keeps the app running in the tray, false makes
// the next close quit the whole app, tray included. Without a tray
// (Config.Tray / --tray) it has no effect - closing already exits, and
// a tray cannot be created at runtime. Safe to call from anywhere,
// e.g. a settings page's paired handler.
func SetCloseToTray(v bool) { closeToTray.Store(v) }

// CloseToTray reports the current close-to-tray setting.
func CloseToTray() bool { return closeToTray.Load() }
