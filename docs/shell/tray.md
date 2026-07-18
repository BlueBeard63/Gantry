# The system tray

The `tray` package puts your app in the notification area: an icon, a tooltip, a left-click action, and a right-click menu you define. With a tray, "close" can mean "keep running" - see [Close behavior](close-and-lifecycle.md). Everything here is one struct, `tray.Options`, plus the `MenuItem` list it carries.

## Basic tray

Through `appshell.App` (the usual way):

```go
shell := &appshell.App{
    Window: appshell.WindowOptions{ /* ... */ },
    Tray: &tray.Options{
        Icon:    appicon.ICO(appicon.Render(32, appicon.DefaultPalette())),
        Tooltip: "Myapp is running",
    },
}
```

`App.Run` adds "Open <Title>" at the top and "Quit" at the bottom for you (Quit closes the window and cancels the app context). Everything you put in `Menu` appears between them.

## tray.Options fields

- `Icon []byte` - the tray icon as ICO bytes, what Windows trays want (`appicon.ICO(...)` or an `.ico` file's contents).
- `IconPNG []byte` - the tray icon as PNG bytes, what Linux and Mac trays want. `tray.Run` picks the right one per platform via `platformIcon()` (ICO on Windows, PNG elsewhere, falling back to whichever is present), so on a cross-platform app set both. `gantry.Run` sets both automatically; the [appicon](monitors-and-icons.md) package produces both formats from one glyph.
- `Title string` - the tray title (shown by some Linux tray implementations; usually left empty on Windows).
- `Tooltip string` - the hover tooltip on the icon.
- `OnTapped func()` - runs on a left-click of the tray icon (the menu stays on right-click). `App.Run` does not set this; it is yours to use, e.g. left-click toggles a widget.
- `Menu []MenuItem` - the right-click menu, described below.
- `OnReady func(h *tray.Handle)` - runs once the tray is up, handing you a `Handle` for runtime changes (see [Advanced: runtime changes](#advanced-runtime-changes)).
- `OnExit func()` - runs when the tray loop tears down (after `Quit`). `App.Run` hooks this to close the main window and cancel `ctx`.

## Menu items

```go
Tray: &tray.Options{
    Icon:    icoBytes,
    Tooltip: "Myapp",
    OnTapped: func() { /* tray icon left-click */ },
    Menu: []tray.MenuItem{
        {Label: "Show timer", Tooltip: "Toggle the floating timer",
            OnClick: func(*tray.Item) {
                widgets.Toggle("timer", "--shellrole", "widget-timer")
            }},
        {Separator: true},
        {Label: "Compact mode", Checkable: true, Checked: false,
            OnClick: func(it *tray.Item) {
                setCompact(it.Checked()) // already toggled before OnClick runs
            }},
        {Label: "Monitor", Children: []tray.MenuItem{
            {Label: "Primary", OnClick: func(*tray.Item) { pick(-1) }},
            {Label: "Second", OnClick: func(*tray.Item) { pick(1) }},
        }},
        {Label: "Not ready yet", Disabled: true},
    },
},
```

Every `MenuItem` field:

- `Label string` - the item's text.
- `Tooltip string` - an optional hover tooltip on the item.
- `OnClick func(item *tray.Item)` - runs when the item is clicked and receives a live `Item` handle. For a checkable item the check state has **already** been toggled by the time `OnClick` runs, so read the new value with `item.Checked()`.
- `Separator bool` - renders a divider line; all other fields are ignored. Separators only work at the top level - `systray` has no submenu separators, so a `Separator` inside `Children` is silently skipped.
- `Checkable bool` - the item shows a checkmark and toggles automatically on each click. Works at the top level and inside submenus.
- `Checked bool` - the initial check state of a checkable item.
- `Disabled bool` - the item is visible but greyed out and unclickable.
- `Children []MenuItem` - turns the item into a submenu. A parent of a submenu does not fire its own `OnClick` (only its leaf children do).

## Changing items at runtime: the Item handle

`OnClick` (and `OnReady`, if you stash the handles) hand you a `*tray.Item`. Its methods:

- `item.Checked() bool` - the current check state.
- `item.SetChecked(v bool)` - check or uncheck a checkable item yourself.
- `item.SetDisabled(v bool)` - grey out (`true`) or re-enable (`false`).
- `item.SetLabel(s string)` - rename the item live, e.g. "Pause" → "Resume".

## Icons

Windows trays want ICO bytes (`Options.Icon`); Linux and Mac want PNG (`Options.IconPNG`). Set both - `tray.Run` picks the right one per platform - or use the [appicon](monitors-and-icons.md) package for both formats at once. `gantry.Run` sets both automatically.

---

## Advanced: runtime changes

`Options.OnReady` hands you a `*tray.Handle` once the tray exists:

```go
Tray: &tray.Options{
    // ...
    OnReady: func(h *tray.Handle) {
        h.SetTooltip("Myapp - 3 tasks running")
        // keep h around; swap icons with h.SetIcon(busyIco) later
    },
},
```

`Handle` carries three live setters: `SetIcon([]byte)` (ICO on Windows, PNG elsewhere), `SetTooltip(string)`, and `SetTitle(string)`.

## Advanced: toggling the tray without a rebuild

Whether *closing the window* keeps the app in the tray is switchable from code while the app runs - `gantry.SetCloseToTray(false)` makes close quit outright - see [Switching close behavior at runtime](close-and-lifecycle.md#advanced-switching-close-behavior-at-runtime).

Whether the tray *exists* is a launch decision, not a compile decision: `gantry.Run` gives every app `--tray` and `--no-tray` runtime flags that override `Config.Tray`.

```
myapp.exe --no-tray        (closing the window exits)
gantry dev -- --tray       (dev run with the tray on)
```

## Advanced: tray-only apps

Set `TrayOnly` on `appshell.App` and skip the main window entirely: the app is its tray icon, menu actions, widgets and popups. Useful for background utilities.

```go
shell := &appshell.App{
    Window:   appshell.WindowOptions{AppName: "myutil", URL: url},
    TrayOnly: true,
    Tray:     &tray.Options{ /* ... */ },
}
```

The `Window` options still matter for `AppName` and `URL` - widgets and the browser fallback use them. With `TrayOnly` set, `App.Run` skips the "Open <Title>" item it would otherwise prepend.

## Advanced: using tray directly

Without `App.Run`, call `tray.Run(options)` yourself - it blocks (it drives `systray.Run`), so start it on its own goroutine, and call `tray.Quit()` to tear it down. `Options.OnExit` runs after teardown; that is exactly where `App.Run` hooks "Quit means close the window and cancel the app context".
