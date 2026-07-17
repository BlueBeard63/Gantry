# The system tray

The tray package puts your app in the notification area: an icon, a tooltip, a left-click action, and a right-click menu you define. With a tray, "close" can mean "keep running" - see [Close behavior](close-and-lifecycle.md).

## Basic tray

Through appshell.App (the usual way):

```go
shell := &appshell.App{
    Window: appshell.WindowOptions{ /* ... */ },
    Tray: &tray.Options{
        Icon:    appicon.ICO(appicon.Render(32, appicon.DefaultPalette())),
        Tooltip: "Myapp is running",
    },
}
```

App.Run adds "Open <Title>" at the top and "Quit" at the bottom for you. Everything you put in Menu appears between them.

## Menu items

```go
Tray: &tray.Options{
    Icon:    icoBytes,
    Tooltip: "Myapp",
    OnTapped: func() { /* tray icon left-click */ },
    Menu: []tray.MenuItem{
        {Label: "Show timer", OnClick: func(*tray.Item) {
            widgets.Toggle("timer", "--shellrole", "widget-timer")
        }},
        {Separator: true},
        {Label: "Compact mode", Checkable: true, Checked: false,
            OnClick: func(it *tray.Item) {
                setCompact(it.Checked()) // toggled before OnClick runs
            }},
        {Label: "Monitor", Children: []tray.MenuItem{
            {Label: "Primary", OnClick: func(*tray.Item) { pick(-1) }},
            {Label: "Second", OnClick: func(*tray.Item) { pick(1) }},
        }},
        {Label: "Not ready yet", Disabled: true},
    },
},
```

Item features:

- Separator: true renders a divider (top-level menus only).
- Checkable items show a checkmark and toggle automatically on click; read the new state inside OnClick with item.Checked(), or set it yourself with item.SetChecked(v).
- Disabled items are visible but greyed out; flip at runtime with item.SetDisabled(v).
- Children turns an item into a submenu (parents of submenus do not fire clicks themselves).
- item.SetLabel(s) renames an item live ("Pause" -> "Resume").

## Runtime changes

Options.OnReady hands you a Handle once the tray exists:

```go
Tray: &tray.Options{
    // ...
    OnReady: func(h *tray.Handle) {
        h.SetTooltip("Myapp - 3 tasks running")
        // keep h around; swap icons with h.SetIcon(busyIco) later
    },
},
```

## Toggling the tray without a rebuild

Whether *closing the window* keeps the app in the tray is also switchable from code while the app runs - `gantry.SetCloseToTray(false)` makes close quit outright - see [Switching close behavior at runtime](close-and-lifecycle.md#switching-close-behavior-at-runtime).

`gantry.Run` gives every app --tray and --no-tray runtime flags that override Config.Tray, so the tray is a launch decision, not a compile decision:

```
myapp.exe --no-tray        (closing the window exits)
gantry dev -- --tray       (dev run with the tray on)
```

## Tray-only apps

Set `TrayOnly` on `appshell.App` and skip the main window entirely: the app is its tray icon, menu actions, widgets and popups. Useful for background utilities.

```go
shell := &appshell.App{
    Window:   appshell.WindowOptions{AppName: "myutil", URL: url},
    TrayOnly: true,
    Tray:     &tray.Options{ /* ... */ },
}
```

(The Window options still matter for AppName and URL - widgets and the browser fallback use them.)

## Using tray directly

Without App.Run, call tray.Run(options) yourself - it blocks, so start it on its own goroutine, and call tray.Quit() to tear it down. Options.OnExit runs after teardown; that is where App.Run hooks "Quit means cancel the app context".

## Icons

Windows trays want ICO bytes (Options.Icon); Linux and Mac trays want PNG (Options.IconPNG). Set both - tray.Run picks the right one per platform - or use the appicon package for both formats at once; see [Monitors and icons](monitors-and-icons.md). gantry.Run sets both automatically.
