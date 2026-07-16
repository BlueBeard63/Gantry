// Package tray puts an app in the system tray: an icon with a tooltip, a
// left-click action, and a right-click menu of caller-defined actions
// (plain, checkable, disabled, separators, submenus). Tray-only apps and
// minimize-to-tray apps both build on this.
package tray

import "runtime"

// MenuItem describes one entry in the tray menu.
type MenuItem struct {
	Label   string
	Tooltip string
	// OnClick runs when the item is clicked. For checkable items the
	// check state has already been toggled when OnClick runs; read it
	// via item.Checked().
	OnClick func(item *Item)
	// Separator renders a divider line; all other fields are ignored.
	Separator bool
	// Checkable items show a checkmark and toggle on click.
	Checkable bool
	// Checked is the initial check state of a checkable item.
	Checked bool
	// Disabled items are visible but greyed out.
	Disabled bool
	// Children turns the item into a submenu.
	Children []MenuItem
}

// Options configures the tray icon.
type Options struct {
	// Icon is the tray icon as ICO bytes - what Windows trays want
	// (e.g. appicon.ICO(...) or an .ico file's contents).
	Icon []byte
	// IconPNG is the tray icon as PNG bytes - what Linux (and Mac)
	// trays want. Leave empty on Windows-only apps.
	IconPNG []byte
	Title   string
	Tooltip string
	// OnTapped runs on tray icon left-click (the menu stays on
	// right-click).
	OnTapped func()
	Menu     []MenuItem
	// OnReady runs once the tray is set up, with a handle for runtime
	// changes (swap icon, tooltip).
	OnReady func(h *Handle)
	// OnExit runs when the tray loop tears down (after Quit).
	OnExit func()
}

// platformIcon picks the icon format the current OS's tray expects.
func (o *Options) platformIcon() []byte {
	if runtime.GOOS == "windows" {
		if len(o.Icon) > 0 {
			return o.Icon
		}
		return o.IconPNG
	}
	if len(o.IconPNG) > 0 {
		return o.IconPNG
	}
	return o.Icon
}
