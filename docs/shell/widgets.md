# Widgets

A widget is a small always-on-top helper window: a floating timer, a peek card, a mini toolbar. It renders one of your app's pages in its own frameless native window, and it runs as its own process so a renderer crash can never take your app down. `appshell.RunWidget(opts)` opens one and blocks until it closes.

## The pieces

1. **A page to render.** Widgets are just routes. Make a `pages/widget-timer/` pair like any other page and export `export const chrome = false;` from its tsx module so it does not get a TitleBar.
2. **A `--shellrole` branch in `main.go`** that runs the widget window instead of the app:

```go
case "widget-timer":
    defer appshell.RoleLog("myapp", "widget-timer")()
    if err := appshell.RunWidget(appshell.WidgetOptions{
        AppName:    "myapp",
        Title:      "Myapp Timer", // must be unique per widget kind
        URL:        "http://127.0.0.1:" + strconv.Itoa(*port) + "/widget-timer",
        Width:      300,
        Height:     44,
        Placement:  appshell.PlaceTopCenter,
        NoActivate: true,
    }); err != nil {
        log.Fatalf("widget: %v", err)
    }
    return
```

3. **A `ProcManager` in the app** that spawns it (see below).

## WidgetOptions fields

- `AppName string` (required) - same rules as `WindowOptions.AppName`: it namespaces the WebView2 data folder and logs.
- `Title string` (required) - identifies the widget window AND enforces the singleton: `RunWidget` calls `FindWindow` on the title and closes any predecessor before opening, so a widget can never double up (even against an orphan left by a killed app). Keep titles unique per widget kind.
- `URL string` - the page to load.
- `Width, Height int` (required) - a widget has no sensible default size, so `normalize()` errors if either is `<= 0`.
- `Placement Placement` (default `PlaceTopCenter`, the zero value) - a corner or center of the monitor work area: `PlaceTopCenter`, `PlaceTopLeft`, `PlaceTopRight`, `PlaceBottomLeft`, `PlaceBottomCenter`, `PlaceBottomRight`, `PlaceCenter`, or `PlaceCustom` (use `X`/`Y` directly).
- `Margin int` (default 16) - the gap in pixels from the work-area edge for every non-custom placement.
- `X, Y int` (default 0) - the exact top-left in virtual-desktop pixels, used only when `Placement` is `PlaceCustom`.
- `Monitor int` (default 0) - which display to place on; `-1` (or an index out of range) is the primary. See [Monitors and icons](monitors-and-icons.md).
- `NoActivate bool` (default false) - makes the widget glanceable: it takes `WS_EX_NOACTIVATE` and never steals keyboard focus. Turn it on for things like a floating timer - clicks still work, but typing elsewhere is never interrupted.
- `CloseOnDeactivate bool` (default false) - the widget dismisses itself the moment it loses activation, like a shell flyout: click anywhere else and it is gone. This one needs activation to receive the deactivate, so do **not** combine it with `NoActivate`.
- `StartHidden bool` (default false) - create the window hidden (a real `ShowWindow(SW_HIDE)`); the page reveals itself with `shell.setVisible(true)` when it has something to show.
- `SquareCorners bool` (default false) - opt out of the Windows 11 rounded corners (widgets are rounded by default).
- `BindingPrefix string` (default `"gantry"`) - as on the [main window](window-options.md#bindingprefix-string-default-gantry).
- `ExtraBindings map[string]any` (default nil) - extra JS functions, as on the main window.
- `Icon appshell.IconSource` (default zero value) - the window/taskbar icon.
- `DataDirRole string` (default `"widget-" + sanitized Title`) - the WebView2 data-folder suffix. The default is derived from the title, so distinct widget titles automatically get distinct folders; you rarely set this.

Widgets are always topmost - they are pinned `HWND_TOPMOST` at open and there is no binding to unpin them at runtime.

## Spawning widgets: ProcManager

Widgets run out of process, spawned by an `appshell.ProcManager` that re-invokes your own exe with the role flags:

```go
widgets := appshell.NewProcManager()
widgets.Show("timer", "--shellrole", "widget-timer", "--port", strconv.Itoa(port))
defer widgets.CloseAll()
```

The manager keys each child by a string you choose. Its methods:

- `Show(key, args...)` - start the child if it is not already running (no-op if it is).
- `Toggle(key, args...)` - kill the child if running, else `Show` it. This is the classic tray-left-click / menu-item pattern: one click opens, another dismisses.
- `Replace(key, args...)` - kill any running child and start a fresh one. Used for popups, where a new one supersedes the old (see [Notifications](notifications.md)).
- `Kill(key)` - stop the keyed child if running.
- `Running(key) bool` - whether the keyed child is alive.
- `CloseAll()` - kill every managed child (app shutdown; the `defer` above).

Each child is reaped automatically (`cmd.Wait` in a goroutine) so it never zombies, and forgotten once gone.

## The widget bridge

Inside a widget page, `useShell()` gives you exactly the bindings `RunWidget` registers - `Close`, `Drag`, `OpenExternal`, `Visible` and `Resize`:

- `shell.drag()` - start a native window move on mousedown. Widgets have no caption band, so put a grip element somewhere and call it there.
- `shell.close()` - close the widget window.
- `shell.setVisible(show)` - show or hide the native window from the page (the `Visible` binding). A timer widget shows itself only while something is running; pairs with `StartHidden`.
- `shell.resize(w, h)` - resize in place. It uses `SWP_NOMOVE`, so the widget keeps whatever position the user dragged it to (flyouts grow downward from where they are).
- `shell.openExternal(url)` - open a URL in the default browser.

```tsx
export const chrome = false;

export default function WidgetTimer() {
  const shell = useShell();
  const [open, setOpen] = useState(false);
  return (
    <div className="timer">
      <span className="grip" onMouseDown={(e) => {
        if (e.button === 0) { e.preventDefault(); shell.drag(); }
      }}>::</span>
      <span>00:25:13</span>
      <button onClick={() => {
        shell.resize(300, open ? 44 : 84);
        setOpen(!open);
      }}>{open ? "^" : ">"}</button>
    </div>
  );
}
```

The page talks to your app the normal way (`usePaired`); the widget process serves no logic of its own, so its events travel to the main process over the websocket.

---

## Advanced: why a separate process

The webview renderer is a big piece of software; if it ever crashes, only the widget process dies - your scheduler, server and main window never notice. It also sidesteps a WebView2 rule: each process needs its own browser-data folder, which `WidgetOptions` derives from the `Title` automatically. `appshell.RoleLog("myapp", "widget-timer")` writes the child's log to `%LocalAppData%\<AppName>\<role>.log`, because a `windowsgui` child has no console to print to - defer it at the top of the `--shellrole` branch.
