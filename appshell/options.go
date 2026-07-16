// Package appshell hosts an app's native windows: a frameless main
// window whose chrome is drawn by the web frontend, always-on-top helper
// widgets, notification popups, and the lifecycle glue (minimize to
// tray, single instance, child processes). The web side talks to it
// through JS functions bound as window.<prefix>Close, <prefix>Minimize,
// <prefix>Drag and so on; the gantry-web npm package wraps them.
package appshell

import "errors"

// CloseAction is what OnCloseRequest returns to decide the fate of a
// close request (the X button, Alt+F4, a WM_CLOSE from anywhere).
type CloseAction int

const (
	// CloseAllow proceeds with the default close: the window is
	// destroyed and RunWindow returns.
	CloseAllow CloseAction = iota
	// CloseCancel swallows the request; the window stays.
	CloseCancel
	// CloseHide hides the window instead; the app keeps running. Show it
	// again with ShowMainWindow, or close for real with CloseMainWindow.
	CloseHide
)

// IconSource selects the window/taskbar icon. The zero value tries the
// exe's embedded icon resource and falls back to nothing.
type IconSource struct {
	// PNG is raw PNG bytes drawn at runtime (used when the exe carries
	// no icon resource, e.g. dev builds). appicon.PNG(appicon.Render(32,
	// palette)) produces a usable one.
	PNG []byte
	// SkipExe skips trying ExtractIconExW on the running exe first.
	SkipExe bool
}

// WindowOptions configures the main window. The zero value of every
// field is the default, so a minimal app only sets AppName, Title and
// URL.
type WindowOptions struct {
	// AppName namespaces the WebView2 user-data folder
	// (%LocalAppData%\<AppName>\webview-<role>) and per-app logs. It is
	// REQUIRED: two apps must never share a data folder, so there is no
	// default.
	AppName string
	Title   string
	URL     string

	Width  int // default 1024
	Height int // default 768
	// Min/Max sizes are enforced natively while resizing (0 = no limit).
	MinWidth, MinHeight int
	MaxWidth, MaxHeight int
	// X, Y position the window; when both are zero the window opens
	// centered on the primary monitor.
	X, Y int

	// Framed keeps the normal OS title bar and frame instead of the
	// frameless custom-chrome look. Default false = frameless.
	Framed bool
	// DisableMinimize removes the minimize capability (strips
	// WS_MINIMIZEBOX and omits the Minimize binding).
	DisableMinimize bool
	// EnableMaximize allows maximize/restore (adds bindings and, when
	// Framed, keeps the native maximize box). Default false: like
	// Timekeep, the window resizes but never maximizes.
	EnableMaximize bool
	// DisableClose makes the window ignore close requests entirely;
	// only CloseMainWindow (e.g. tray Quit) can close it.
	DisableClose bool
	// OnCloseRequest intercepts close requests. Return CloseAllow (or
	// leave nil) for the default, CloseCancel to keep the window, or
	// CloseHide to hide it and keep the app running - the natural place
	// to hand off to a tray icon or fire a notification. Runs on the
	// window thread: do not block; spawn goroutines for real work.
	// CloseMainWindow bypasses this hook.
	OnCloseRequest func() CloseAction
	// AlwaysOnTop pins the window above normal windows and adds the
	// SetAlwaysOnTop binding.
	AlwaysOnTop bool
	// Corners picks the Win11 corner style: "" (system default),
	// "round", "small" (small radius), or "square". Ignored on other
	// platforms (the compositor owns corners there) and on Win10.
	Corners string

	// Custom-chrome hit-test metrics (device pixels). These MUST match
	// the TitleBar the frontend renders - they are two halves of one
	// hit-test. Defaults match gantry-web's TitleBar defaults.
	CaptionHeight       int // draggable top band; default 40
	CaptionLeftReserve  int // non-drag zone on the left; default 8
	CaptionRightReserve int // non-drag zone for the window buttons; default 150
	ResizeMargin        int // edge resize hit zone; default 6

	// BindingPrefix names the JS bridge functions: <prefix>Close,
	// <prefix>Minimize, ... Default "gantry". Must match the prefix the
	// frontend passes to getShell()/useShell().
	BindingPrefix string
	// ExtraBindings are additional JS functions bound on the window
	// (name -> Go func), passed straight to webview Bind.
	ExtraBindings map[string]any

	Icon IconSource
	// Geometry persists window position/size between runs. nil = the
	// window always opens at the configured size and position.
	Geometry GeometryStore

	// Debug enables webview devtools.
	Debug bool
	// AutoFocus focuses the webview when the window gains focus.
	AutoFocus bool
	// DataDirRole distinguishes WebView2 data folders when one app runs
	// several window roles (main, widgets, popups). Default "main".
	DataDirRole string
}

func (o *WindowOptions) normalize() error {
	if o.AppName == "" {
		return errors.New("appshell: WindowOptions.AppName is required")
	}
	if o.Width <= 0 {
		o.Width = 1024
	}
	if o.Height <= 0 {
		o.Height = 768
	}
	if o.CaptionHeight <= 0 {
		o.CaptionHeight = 40
	}
	if o.CaptionLeftReserve <= 0 {
		o.CaptionLeftReserve = 8
	}
	if o.CaptionRightReserve <= 0 {
		o.CaptionRightReserve = 150
	}
	if o.ResizeMargin <= 0 {
		o.ResizeMargin = 6
	}
	if o.BindingPrefix == "" {
		o.BindingPrefix = "gantry"
	}
	if o.DataDirRole == "" {
		o.DataDirRole = "main"
	}
	return nil
}

// Placement positions a widget window on its monitor's work area.
type Placement int

const (
	PlaceTopCenter Placement = iota
	PlaceTopLeft
	PlaceTopRight
	PlaceBottomLeft
	PlaceBottomCenter
	PlaceBottomRight
	PlaceCenter
	// PlaceCustom uses WidgetOptions.X and Y as virtual-desktop
	// coordinates directly.
	PlaceCustom
)

// WidgetOptions configures a small always-on-top helper window (a
// floating timer, a peek card, a mini toolbar). Widgets normally run as
// their own process (see ProcManager) so a webview crash cannot take the
// host app down.
type WidgetOptions struct {
	// AppName - same rules as WindowOptions.AppName.
	AppName string
	// Title identifies the widget window and MUST be unique per widget
	// kind: it enforces the one-instance rule (a new widget closes any
	// predecessor with the same title, e.g. an orphan left behind by a
	// killed app).
	Title string
	URL   string

	Width, Height int // required (no useful default for a widget)

	Placement Placement
	// Margin is the gap from the work-area edge; default 16.
	Margin int
	// X, Y are used when Placement is PlaceCustom.
	X, Y int
	// Monitor index; -1 or 0 out of range = primary.
	Monitor int

	// NoActivate makes the widget glanceable: it never steals keyboard
	// focus (WS_EX_NOACTIVATE), like a floating timer.
	NoActivate bool
	// CloseOnDeactivate dismisses the widget when it loses activation,
	// like a shell flyout: click anywhere else and it goes away.
	CloseOnDeactivate bool
	// StartHidden creates the widget hidden; the page shows it when
	// ready via the Visible(true) binding.
	StartHidden bool
	// SquareCorners disables the Win11 rounded-corner preference.
	SquareCorners bool

	BindingPrefix string
	ExtraBindings map[string]any
	Icon          IconSource
	DataDirRole   string // default derived from Title
}

func (o *WidgetOptions) normalize() error {
	if o.AppName == "" {
		return errors.New("appshell: WidgetOptions.AppName is required")
	}
	if o.Title == "" {
		return errors.New("appshell: WidgetOptions.Title is required (it enforces the widget singleton)")
	}
	if o.Width <= 0 || o.Height <= 0 {
		return errors.New("appshell: WidgetOptions.Width and Height are required")
	}
	if o.Margin <= 0 {
		o.Margin = 16
	}
	if o.BindingPrefix == "" {
		o.BindingPrefix = "gantry"
	}
	if o.DataDirRole == "" {
		o.DataDirRole = "widget-" + sanitizeRole(o.Title)
	}
	return nil
}

// PopupOptions configures a notification popup: frameless, always on
// top, never steals focus, placed at the top or bottom of a monitor.
// Because the window is app-drawn (not a shell toast), Windows Do Not
// Disturb / Focus Assist cannot suppress it. Popups run out-of-process
// (see notify.Notifier).
type PopupOptions struct {
	AppName string
	URL     string

	Width, Height int // required

	// Monitor index; -1 = primary.
	Monitor int
	// Position is "top" or "bottom" (default "bottom").
	Position string
	// Margin is the gap from the work-area edge; default 24.
	Margin int
	// AdjustPos optionally nudges the computed position, given the
	// chosen monitor and the default x, y (e.g. slide below a widget
	// that parks in the same corner).
	AdjustPos func(x, y int) (int, int)

	BindingPrefix string
	ExtraBindings map[string]any
	Icon          IconSource
	DataDirRole   string // default "popup"
}

func (o *PopupOptions) normalize() error {
	if o.AppName == "" {
		return errors.New("appshell: PopupOptions.AppName is required")
	}
	if o.Width <= 0 || o.Height <= 0 {
		return errors.New("appshell: PopupOptions.Width and Height are required")
	}
	if o.Position == "" {
		o.Position = "bottom"
	}
	if o.Margin <= 0 {
		o.Margin = 24
	}
	if o.BindingPrefix == "" {
		o.BindingPrefix = "gantry"
	}
	if o.DataDirRole == "" {
		o.DataDirRole = "popup"
	}
	return nil
}

// sanitizeRole turns a window title into a folder-name-safe role.
func sanitizeRole(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}
