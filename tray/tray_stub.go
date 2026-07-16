//go:build android || (darwin && !cgo) || (!windows && !linux && !darwin)

package tray

import "log"

// The Mac tray needs cgo (systray's darwin backend is Objective-C), so
// cross-compiled CGO_ENABLED=0 mac builds get this stub: the app runs
// (browser fallback), just without a tray icon.

// Item is a no-op handle in tray-less builds.
type Item struct{}

func (i *Item) Checked() bool      { return false }
func (i *Item) SetChecked(bool)    {}
func (i *Item) SetDisabled(bool)   {}
func (i *Item) SetLabel(string)    {}

// Handle is a no-op in tray-less builds.
type Handle struct{}

func (h *Handle) SetIcon([]byte)     {}
func (h *Handle) SetTooltip(string)  {}
func (h *Handle) SetTitle(string)    {}

// Quit is a no-op in tray-less builds.
func Quit() {}

// Run logs and returns immediately; there is no tray in this build.
func Run(o Options) {
	log.Print("tray: not available in this build (mac needs a cgo build)")
	if o.OnExit != nil {
		// Never auto-fires: a missing tray must not quit the app.
		_ = o.OnExit
	}
}
