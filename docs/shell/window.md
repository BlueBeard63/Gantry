# The main window

The main window is a frameless native window whose chrome (title bar, buttons) is drawn by your React frontend, while movement, resizing and the buttons drive the real window natively. `appshell.RunWindow(opts)` opens it and blocks the main goroutine until it closes; most apps go through `appshell.App`, which calls RunWindow for you (see [Close behavior and app lifecycle](close-and-lifecycle.md)).

Every knob lives on one struct, `appshell.WindowOptions`. Its `normalize()` method fills in defaults, and the zero value of each field IS its default, so a minimal app sets only the three required-ish fields and takes everything else as-is:

```go
appshell.WindowOptions{
    AppName: "myapp",                    // required - RunWindow errors without it
    Title:   "My App",
    URL:     "http://127.0.0.1:8330",
}
```

The rest of this page walks every field of `WindowOptions` in turn - name, type, default, and what it does - then covers the JS bridge, the custom-chrome contract, and Linux at the end.

## Identity and content

### AppName `string` (required, no default)

Namespaces the WebView2 user-data folder (`%LocalAppData%\<AppName>\webview-<role>`) and the per-app logs. Two different apps must never share one folder, so there is no default: `normalize()` returns `errors.New("appshell: WindowOptions.AppName is required")` when it is empty, and RunWindow returns that error before opening anything.

### Title `string` (default `""`)

The native window title - what the taskbar button and Alt+Tab show. The frameless window draws no title bar on screen, so the visible title is whatever your [TitleBar](../ui/titlebar.md) renders; `Title` is still worth setting for the taskbar and because `appshell.App` builds its tray "Open <Title>" item from it.

### URL `string` (default `""`)

What the webview navigates to: your app's local server (`http://127.0.0.1:<port>`) in production, or the Vite dev server during `gantry dev`.

## Size and position

### Width `int` (default 1024), Height `int` (default 768)

Initial client size in device pixels. Any value `<= 0` is replaced by the default in `normalize()`. Saved geometry, if you set `Geometry`, overrides these on load.

```go
Width: 900, Height: 600,
```

### MinWidth, MinHeight, MaxWidth, MaxHeight `int` (default 0 = no limit)

Hard size limits enforced by the native resize loop itself (the window physically stops at them - no CSS involved). They are passed straight into the window subclass as `minWidth`/`minHeight`/`maxWidth`/`maxHeight`; `0` on any of them means "no limit in that direction".

```go
MinWidth: 480, MinHeight: 360,   // never smaller than this
```

### X, Y `int` (default 0, 0 = centered)

Initial top-left in virtual-desktop pixels. The window is centered on the primary monitor unless you give an explicit position: internally `explicitPos = haveGeometry || X != 0 || Y != 0`, and only then does RunWindow move the window with `SetWindowPos`. So leaving both at zero centers; setting either one places.

### Geometry `appshell.GeometryStore` (default nil)

Persists the window's position, size and maximized state between runs. `nil` means the window always opens at the configured size/position. Hand it `appshell.FileGeometry(path)` and it saves a small JSON file on every close, resize and hide, restoring on the next launch. Saved rectangles are clamped to a currently-attached monitor (`clampToVisible`), so unplugging the second screen never strands the window off-screen; a load that fails or points off all monitors just falls back to the configured geometry.

```go
Geometry: appshell.FileGeometry(
    filepath.Join(configDir, "myapp", "geometry.json")),
```

`GeometryStore` is a two-method interface (`Load() (Rect, bool, bool)` / `Save(Rect, bool)`) if you want to store the rect somewhere other than a file.

## Frame and window buttons

### Framed `bool` (default false)

`false` (the default) is frameless: RunWindow strips `WS_CAPTION` and your frontend draws the chrome. Set `true` to keep the ordinary OS title bar and frame and skip the custom chrome entirely - handy for a quick tool window where you do not want to build a TitleBar. When framed, the `ResizeEdge` binding is not added (the OS frame handles edges) and `Caps().frameless` reports `false`.

### DisableMinimize `bool` (default false)

Removes minimize: RunWindow clears `WS_MINIMIZEBOX` from the window style, does not bind `<prefix>Minimize`, and `Caps().minimize` returns `false` so the TitleBar hides its minimize button automatically.

### EnableMaximize `bool` (default false)

Maximize is OFF by default - a deliberate Gantry choice, since many tool windows should resize but never fill the screen (the same default Timekeep ships). Setting it `true` binds `<prefix>Maximize`, `<prefix>Restore` and `<prefix>IsMaximized`, keeps the native maximize box, and makes `Caps().maximize` report `true` so the TitleBar shows the button. When it is `false`, RunWindow also clears `WS_MAXIMIZEBOX`, which additionally stops a caption double-click from maximizing.

### DisableClose `bool` (default false)

The window ignores close requests entirely (the subclass drops `WM_CLOSE`); `Caps().close` reports `false` so the TitleBar hides its close button. Only `appshell.CloseMainWindow()` - which tray Quit uses - can still close it. To intercept close rather than forbid it, use `OnCloseRequest` instead.

### OnCloseRequest `func() appshell.CloseAction` (default nil)

Runs on every close request (the X button, Alt+F4, any `WM_CLOSE`) and returns what should happen: `CloseAllow`, `CloseCancel`, or `CloseHide`. `nil` means allow. This is the heart of "close to tray" and "you have unsaved work" flows and gets its own full treatment in [Close behavior and app lifecycle](close-and-lifecycle.md). One rule: it runs on the window thread, so never block in it.

### AlwaysOnTop `bool` (default false)

`true` pins the window topmost at open (`SetWindowPos(HWND_TOPMOST)`) and makes `Caps().alwaysOnTop` report `true`. Note that the `<prefix>SetAlwaysOnTop` binding is always available regardless of this field - the frontend can pin or unpin at runtime either way; `AlwaysOnTop` only sets the starting state and the reported capability.

### Corners `string` (default `""`)

The Windows 11 corner style, one of `""` (system default), `"round"`, `"small"` (small radius) or `"square"`. Applied via `DwmSetWindowAttribute`. Ignored on Windows 10 and on Linux/Mac, where the compositor owns window corners.

## Icon and focus

### Icon `appshell.IconSource` (default zero value)

Sets the taskbar/Alt-Tab icon. `IconSource` has two fields: `PNG []byte` (raw PNG bytes drawn at runtime) and `SkipExe bool`. The zero value tries the exe's embedded icon resource first (`ExtractIconExW` on the running exe) and falls back to nothing; set `SkipExe: true` to skip the exe and go straight to your PNG. The [appicon](monitors-and-icons.md) package draws a decent placeholder if you have no art yet:

```go
Icon: appshell.IconSource{PNG: appicon.PNG(appicon.Render(32, appicon.DefaultPalette()))},
```

### AutoFocus `bool` (default false)

Focuses the webview when the window activates. Set `true` for the main window so typing lands in the page immediately.

### Debug `bool` (default false)

Enables the WebView2 devtools (right-click → Inspect, F12). Leave off in release builds.

## The JS bridge fields

### BindingPrefix `string` (default `"gantry"`)

Names every bound JS function - `<prefix>Close`, `<prefix>Minimize`, and so on. Change it only to dodge a name collision, and then pass the same prefix to `getShell()`/`useShell()` and the TitleBar on the frontend so the two halves still line up.

### ExtraBindings `map[string]any` (default nil)

Your own JS functions bound on the window (name → Go func), passed straight to WebView2's `Bind`. Use them for things only the native window can do (a native file picker, say). For ordinary app logic prefer paired events, which also work in browser mode.

```go
ExtraBindings: map[string]any{
    "myappPickFile": func() string { return pickFileNative() },
},
```

### DataDirRole `string` (default `"main"`)

Distinguishes the WebView2 user-data folders when one app opens several window kinds (main, widgets, popups) - each needs its own folder. The scaffold's default `"main"` is already correct for the main window; you only touch this if you hand-roll additional window roles.

## The custom-chrome hit-test metrics

Because the frontend draws the chrome over a frameless window, four metrics (all device pixels) shape the invisible native hit-test that makes the drawn chrome behave like a real title bar. Their defaults match gantry-web's TitleBar defaults, so normally you touch neither side. If you change one here, change its twin on the [TitleBar](../ui/titlebar.md) - they are two halves of one hit-test, and a mismatch shows up as "the close button drags the window" or "this strip of page cannot be clicked".

### CaptionHeight `int` (default 40)

The height of the top band that drags the window.

### CaptionLeftReserve `int` (default 8)

A dead zone on the left of the caption that stays clickable (does not drag).

### CaptionRightReserve `int` (default 150)

A dead zone on the right where the window buttons live, so clicking them never starts a drag.

### ResizeMargin `int` (default 6)

The edge thickness that shows resize cursors and starts a native resize, in all eight directions.

---

## The bridge in practice

RunWindow binds these JS functions on the window (prefix `gantry` by default). Some are conditional:

- `gantryClose`, `gantryDrag`, `gantryAttention`, `gantryOpenExternal`, `gantrySetAlwaysOnTop`, `gantryCaps` - always bound.
- `gantryMinimize` - bound unless `DisableMinimize`.
- `gantryMaximize`, `gantryRestore`, `gantryIsMaximized` - bound only when `EnableMaximize`.
- `gantryResizeEdge` - bound only when frameless (`!Framed`); it starts an interactive edge resize on Linux and is effectively unused on Windows, where the native hit-test handles edges.
- `gantryOpenExternal` opens a URL in the default browser (gantry-web's `ExternalLink` uses it so external links never navigate the app window).

You will not call these raw - gantry-web's `getShell()`/`useShell()` wraps them with feature detection, so the same frontend also runs in a plain browser tab where they simply do not exist. Each is safe to call anywhere: outside a native window it just does nothing, and `shell.available` tells you which world you are in. The `useShell()` surface:

- `shell.close()`, `shell.minimize()`, `shell.maximize()`, `shell.restore()` - window verbs.
- `shell.isMaximized()` - a `Promise<boolean>` for the current maximized state.
- `shell.drag()` - start the native window-move loop (call on mousedown from a custom caption).
- `shell.attention()` - system notification sound plus taskbar flash (see [Notifications](notifications.md)).
- `shell.caps()` - a `Promise<ShellCaps>` reporting `{minimize, maximize, close, alwaysOnTop, platform, frameless}` - which buttons the Go side enabled, so the TitleBar renders exactly those (all false in a browser).
- `shell.setAlwaysOnTop(on)` - pin or unpin the window above others.
- `shell.openExternal(url)` - open a URL in the user's default browser, never in the app window.
- `shell.resizeEdge(edge)` - start an interactive resize from an edge (`"n"`, `"se"`, ...) on Linux frameless windows.

`shell.setVisible(show)` and `shell.resize(w, h)` exist in the useShell surface too, but their native bindings live on [widgets](widgets.md) and [popups](notifications.md), not the main window - the main window has no `Visible`/`Resize` binding, so calling them there is a no-op. RunWindow also disables the WebView2 status bar (the bottom-corner URL-preview bubble), a browser artifact that looks wrong in a desktop app.

## Linux

The same `WindowOptions` drive a GTK window on Linux: frameless (undecorated), native drag, minimize/maximize/close, min/max sizes, always-on-top, the close hook and geometry persistence all work. Edge resizing comes from the frontend there - gantry-web renders invisible resize strips (ResizeFrame, added automatically on frameless Linux windows) that start the compositor's interactive resize via the `ResizeEdge` binding. Positioning and saved geometry are best-effort under pure Wayland (X11/XWayland honor them), and `Corners` is Windows-only.

## Related pages

- [Close behavior and app lifecycle](close-and-lifecycle.md) - `OnCloseRequest` in full, and `appshell.App`.
- [The TitleBar](../ui/titlebar.md) - the frontend half of the chrome contract.
- [Monitors and icons](monitors-and-icons.md) - the `appicon` and `monitors` packages the fields above lean on.
- [Win32 notes](../advanced/win32-notes.md) - what happens underneath.
