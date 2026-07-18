# The main window

The main window is a frameless native window whose chrome (title bar, buttons) is drawn by your React frontend, while movement, resizing and the buttons drive the real window natively. appshell.RunWindow opens it; most apps go through appshell.App, which calls RunWindow for you (see [Close behavior and app lifecycle](close-and-lifecycle.md)).

Every option lives on appshell.WindowOptions. The zero value of each field is its default, so you only set what you change - a minimal app sets AppName, Title and URL.

```go
appshell.WindowOptions{
    AppName: "myapp",           // required
    Title:   "My App",
    URL:     "http://127.0.0.1:8330",
}
```

## Identity

- AppName (required) - namespaces the WebView2 browser-data folder (%LocalAppData%\<AppName>\webview-main) and role logs. Two different apps must never share one, so there is no default; RunWindow errors if it is empty.
- Title - the window title (taskbar, alt-tab). The frameless window shows no native title bar, so on screen the title is whatever your TitleBar renders.
- URL - what the webview loads: your local server, or the Vite dev server during gantry dev.

## Size and position

- Width, Height - initial size (default 1024x768).
- MinWidth, MinHeight, MaxWidth, MaxHeight - hard limits enforced by the native resize loop itself (the window physically stops at them, no CSS involved). 0 means no limit.
- X, Y - initial position in desktop pixels. Both zero (the default) centers the window on the primary monitor.
- Geometry - hand it appshell.FileGeometry(path) and the window remembers its position and size between runs. Saved rects are clamped to a currently attached monitor, so unplugging the second screen never strands the window somewhere invisible.

```go
Geometry: appshell.FileGeometry(
    filepath.Join(configDir, "myapp", "geometry.json")),
```

## Frame and buttons

- Framed - true keeps the ordinary OS title bar and skips the custom chrome entirely. Default false (frameless).
- DisableMinimize - removes minimize: the native minimize box goes away, the Minimize bridge function is not bound, and the TitleBar hides its minimize button automatically.
- EnableMaximize - maximize is OFF by default (a deliberate Gantry default: many tool windows should resize but never fullscreen). Turning it on adds Maximize/Restore/IsMaximized to the bridge, shows the TitleBar button, and lets the native window maximize - clamped to the monitor work area so it never covers the taskbar.
- DisableClose - the window ignores close requests entirely; only appshell.CloseMainWindow() (which tray Quit uses) can close it.
- OnCloseRequest - intercept close instead of disabling it; see [Close behavior](close-and-lifecycle.md).
- AlwaysOnTop - pins the window above normal windows, and binds SetAlwaysOnTop(bool) so the frontend can toggle it.
- Corners - Win11 corner style: "" (system default), "round", "small" or "square". On Linux/Mac the compositor owns window corners, and Win10 ignores the attribute.

The frontend never needs telling which buttons you picked: the bridge's Caps() function reports them and the TitleBar configures itself.

## Icon, focus, debugging

- Icon - appshell.IconSource{PNG: pngBytes} sets the taskbar icon at runtime; by default the exe's embedded icon resource is tried first (set IconSource.SkipExe to skip it). The [appicon](monitors-and-icons.md) package draws a decent placeholder if you have no art yet.
- AutoFocus - focus the webview when the window activates (set true for the main window).
- Debug - enables webview devtools.

---

## Advanced: the chrome contract

Because the frontend draws the chrome over a frameless window, four metrics shape the invisible native hit-test that makes the custom chrome feel real. Their defaults match gantry-web's TitleBar defaults, so normally you touch neither side:

- CaptionHeight (40) - the top band that drags the window.
- CaptionLeftReserve (8) - dead zone on the left that stays clickable.
- CaptionRightReserve (150) - dead zone on the right where the window buttons live.
- ResizeMargin (6) - the edge thickness for resize cursors, all 8 directions.

If you change one in Go, change its twin on the [TitleBar](../ui/titlebar.md) (its height / rightReserve props) - they are two halves of one hit-test. A mismatch shows up as "the close button drags the window" or "this strip of page cannot be clicked".

## Advanced: the bridge

RunWindow binds JS functions the frontend calls, named `<BindingPrefix><Name>` with prefix "gantry" by default:

- gantryClose, gantryMinimize, gantryDrag, gantryAttention
- gantryMaximize / gantryRestore / gantryIsMaximized (EnableMaximize)
- gantrySetAlwaysOnTop, gantryCaps
- gantryOpenExternal - open a URL in the default browser (gantry-web's `ExternalLink` component uses it, so external links never navigate the app window)
- gantryResizeEdge - start an interactive edge resize on Linux frameless windows (gantry-web's `ResizeFrame` strips call it; a no-op on Windows, where the native hit-test handles edges)

You will not call these raw - gantry-web's getShell()/useShell() wraps them with feature detection so the same frontend also runs in a plain browser tab (where they simply do not exist). Each is safe to call anywhere: outside a native window it just does nothing, and `shell.available` tells you which world you are in. The `useShell()` method surface:

- `shell.close()`, `shell.minimize()`, `shell.maximize()`, `shell.restore()` - window verbs.
- `shell.isMaximized()` - a Promise<boolean> for the current maximized state.
- `shell.drag()` - start the native window-move loop (call on mousedown, e.g. from a custom caption).
- `shell.attention()` - system notification sound plus taskbar flash.
- `shell.caps()` - a Promise<ShellCaps> reporting which buttons the Go side enabled (all false in a browser).
- `shell.setAlwaysOnTop(on)` - pin or unpin the window above others (bound when AlwaysOnTop is set).
- `shell.openExternal(url)` - open a URL in the user's default browser, never in the app window.
- `shell.setVisible(show)` and `shell.resize(w, h)` - show/hide and resize in place, used mainly by [widgets](widgets.md).
- `shell.resizeEdge(edge)` - start an interactive resize from an edge ("n", "se", ...) on Linux frameless windows.

The window also disables the WebView2 status bar (the bottom-corner URL-preview bubble), a browser artifact that looks wrong in a desktop app.

- BindingPrefix - change it only if you must avoid a name collision; then pass the same prefix to useShell/TitleBar.
- ExtraBindings - your own JS functions bound on the window: a map of name to Go func, e.g. "myappPickFile": func() string {...}. For app logic prefer paired events (they work in browser mode too); extra bindings are for things only the native window can do.
- DataDirRole - only matters when one app opens several window kinds; distinguishes their WebView2 data folders (default "main"). The scaffold's defaults are already correct.

## Advanced: Linux

The same WindowOptions drive a GTK window on Linux: frameless (undecorated), native drag, minimize/maximize/close, min/max sizes, always-on-top, the close hook and geometry persistence all work. Edge resizing comes from the frontend there - gantry-web renders invisible resize strips (ResizeFrame, added automatically on frameless Linux windows) that start the compositor's interactive resize via the ResizeEdge binding. Positioning and saved geometry are best-effort under pure Wayland (X11/XWayland honor them), and Corners is Windows-only (compositors own corners on Linux).

## Related pages

- [Close behavior and app lifecycle](close-and-lifecycle.md)
- [The TitleBar](../ui/titlebar.md)
- [Win32 notes](../advanced/win32-notes.md) for what happens underneath
