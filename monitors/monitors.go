// Package monitors enumerates the displays attached to the machine so
// windows, widgets and popups can be placed on a specific monitor's work
// area (the part of the screen not covered by taskbars or docks).
package monitors

// Monitor describes one display. Coordinates are in virtual-desktop
// pixels: the primary monitor's top-left corner is (0,0) and secondary
// monitors can have negative origins.
type Monitor struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Primary bool   `json:"primary"`
	// Work area (excludes taskbars/docks) - always-on-top surfaces sit
	// just inside its edges.
	WorkX      int `json:"workX"`
	WorkY      int `json:"workY"`
	WorkWidth  int `json:"workWidth"`
	WorkHeight int `json:"workHeight"`
}

// Pick returns the monitor with the given index, falling back to the
// primary (index -1 requests the primary explicitly), then to a sane
// default when enumeration fails entirely.
func Pick(monitors []Monitor, index int) Monitor {
	if index >= 0 {
		for _, m := range monitors {
			if m.Index == index {
				return m
			}
		}
	}
	for _, m := range monitors {
		if m.Primary {
			return m
		}
	}
	if len(monitors) > 0 {
		return monitors[0]
	}
	return Monitor{Name: "default", Width: 1920, Height: 1080, WorkWidth: 1920, WorkHeight: 1040, Primary: true}
}
