// Package tray puts an app in the system tray: an icon with a tooltip, a
// left-click action, and a right-click menu of caller-defined actions
// (plain, checkable, disabled, separators, submenus). Tray-only apps and
// minimize-to-tray apps both build on this.
package tray

import (
	"runtime"

	"fyne.io/systray"
)

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

// Item is a live handle to one menu entry, usable from OnClick handlers
// and OnReady to change the menu at runtime.
type Item struct {
	mi        *systray.MenuItem
	checkable bool
}

// Checked reports whether a checkable item is currently checked.
func (i *Item) Checked() bool { return i.mi.Checked() }

// SetChecked checks or unchecks a checkable item.
func (i *Item) SetChecked(v bool) {
	if v {
		i.mi.Check()
	} else {
		i.mi.Uncheck()
	}
}

// SetDisabled greys the item out (true) or re-enables it (false).
func (i *Item) SetDisabled(v bool) {
	if v {
		i.mi.Disable()
	} else {
		i.mi.Enable()
	}
}

// SetLabel changes the item's text.
func (i *Item) SetLabel(s string) { i.mi.SetTitle(s) }

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

// Handle changes tray properties at runtime.
type Handle struct{}

// SetIcon swaps the tray icon (ICO bytes).
func (h *Handle) SetIcon(ico []byte) { systray.SetIcon(ico) }

// SetTooltip changes the hover tooltip.
func (h *Handle) SetTooltip(s string) { systray.SetTooltip(s) }

// SetTitle changes the tray title.
func (h *Handle) SetTitle(s string) { systray.SetTitle(s) }

// Quit tears the tray down, which unblocks Run and fires OnExit.
func Quit() { systray.Quit() }

// Run starts the tray icon loop. It blocks until Quit is called; on
// Windows run it on its own goroutine.
func Run(o Options) {
	systray.Run(func() {
		if icon := o.platformIcon(); len(icon) > 0 {
			systray.SetIcon(icon)
		}
		if o.Title != "" {
			systray.SetTitle(o.Title)
		}
		if o.Tooltip != "" {
			systray.SetTooltip(o.Tooltip)
		}
		if o.OnTapped != nil {
			systray.SetOnTapped(o.OnTapped)
		}
		addItems(nil, o.Menu)
		if o.OnReady != nil {
			o.OnReady(&Handle{})
		}
	}, func() {
		if o.OnExit != nil {
			o.OnExit()
		}
	})
}

// addItems registers menu entries under parent (nil = top level) and
// wires their click loops.
func addItems(parent *systray.MenuItem, items []MenuItem) {
	for _, def := range items {
		if def.Separator {
			if parent == nil {
				systray.AddSeparator()
			}
			// systray has no submenu separators; skip inside submenus.
			continue
		}
		var mi *systray.MenuItem
		switch {
		case parent != nil && def.Checkable:
			mi = parent.AddSubMenuItemCheckbox(def.Label, def.Tooltip, def.Checked)
		case parent != nil:
			mi = parent.AddSubMenuItem(def.Label, def.Tooltip)
		case def.Checkable:
			mi = systray.AddMenuItemCheckbox(def.Label, def.Tooltip, def.Checked)
		default:
			mi = systray.AddMenuItem(def.Label, def.Tooltip)
		}
		if def.Disabled {
			mi.Disable()
		}
		if len(def.Children) > 0 {
			addItems(mi, def.Children)
			continue // parents of submenus do not fire clicks
		}
		item := &Item{mi: mi, checkable: def.Checkable}
		onClick := def.OnClick
		go func() {
			for range mi.ClickedCh {
				if item.checkable {
					item.SetChecked(!item.Checked())
				}
				if onClick != nil {
					onClick(item)
				}
			}
		}()
	}
}
