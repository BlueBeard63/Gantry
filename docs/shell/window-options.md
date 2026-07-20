# Window options

Every knob for the main window lives on one struct, `appshell.WindowOptions`, which you hand to `appshell.RunWindow(opts)` (most apps go through `appshell.App`, which calls RunWindow for you - see [Close behavior and app lifecycle](close-and-lifecycle.md)). This page is the exhaustive field reference: name, type, default, and behavior, for every field. The fields that shape the frameless frame and the title-bar buttons get a short entry here and their full treatment in [Frame & window chrome](window-chrome.md).

`WindowOptions.normalize()` fills in defaults, and the zero value of each field IS its default, so a minimal app sets only three fields and takes everything else as-is:

```go
appshell.WindowOptions{
    AppName: "myapp",                    // required - RunWindow errors without it
    Title:   "My App",
    URL:     "http://127.0.0.1:8330",
}
```

## Identity and content

### AppName `string` (required, no default)

Namespaces the WebView2 user-data folder (`%LocalAppData%\<AppName>\webview-<role>`) and the per-app logs. Two different apps must never share one folder, so there is no default: `normalize()` returns `errors.New("appshell: WindowOptions.AppName is required")` when it is empty, and RunWindow returns that error before opening anything.

### Title `string` (default `""`)

The native window title - what the taskbar button and Alt+Tab show. The frameless window draws no title bar on screen, so the visible title is whatever your [TitleBar](../ui/titlebar.md) renders; `Title` is still worth setting for the taskbar and because `appshell.App` builds its tray "Open <Title>" item from it.

### URL `string` (default `""`)

What the webview navigates to: your app's local server (`http://127.0.0.1:<port>`) in production, or the Vite dev server during `gantry dev`.

### DataDirRole `string` (default `"main"`)

Distinguishes the WebView2 user-data folders when one app opens several window kinds (main, widgets, popups) - each needs its own folder. The scaffold's default `"main"` is already correct for the main window; you only touch this if you hand-roll additional window roles.

## The JS bridge fields

### BindingPrefix `string` (default `"gantry"`)

Names every bound JS function - `<prefix>Close`, `<prefix>Minimize`, and so on. Change it only to dodge a name collision, and then pass the same prefix to `getShell()`/`useShell()` and the TitleBar on the frontend so the two halves still line up. The full list of what gets bound lives in [Frame & window chrome](window-chrome.md#the-bound-bridge-functions).

### ExtraBindings `map[string]any` (default nil)

Your own JS functions bound on the window (name → Go func), passed straight to WebView2's `Bind`. Use them for things only the native window can do (a native file picker, say). For ordinary app logic prefer paired events, which also work in browser mode.

```go
ExtraBindings: map[string]any{
    "myappPickFile": func() string { return pickFileNative() },
},
```

## Size and position

### Width `int` (default 1024), Height `int` (default 768)

Initial client size in device pixels. Any value `<= 0` is replaced by the default in `normalize()`. Saved geometry, if you set `Geometry`, overrides these on load.

```go
Width: 900, Height: 600,
```

### MinWidth, MinHeight, MaxWidth, MaxHeight `int` (default 0 = no limit)

Hard size limits enforced by the native resize loop itself (the window physically stops at them - no CSS involved). They are passed into the window subclass and applied in `WM_GETMINMAXINFO` as `PtMinTrackSize`/`PtMaxTrackSize`; `0` on any of them means "no limit in that direction".

```go
MinWidth: 480, MinHeight: 360,   // never smaller than this
```

### X, Y `int` (default 0, 0 = centered)

Initial top-left in virtual-desktop pixels. The window is centered on the primary monitor unless you give an explicit position: internally `explicitPos = haveGeometry || X != 0 || Y != 0`, and only then does RunWindow move the window with `SetWindowPos`. So leaving both at zero centers; setting either one places.

### Geometry `appshell.GeometryStore` (default nil)

Persists the window's position, size and maximized state between runs. `nil` means the window always opens at the configured size/position. Hand it `appshell.FileGeometry(path)` and it saves a small JSON file on every close, resize-drag end and hide, restoring on the next launch. Saved rectangles are clamped to a currently-attached monitor (`clampToVisible`), so unplugging the second screen never strands the window off-screen; a load that fails or points off all monitors just falls back to the configured geometry.

```go
Geometry: appshell.FileGeometry(
    filepath.Join(configDir, "myapp", "geometry.json")),
```

`GeometryStore` is a two-method interface (`Load() (Rect, bool, bool)` / `Save(Rect, bool)`) if you want to store the rect somewhere other than a file. Saved geometry wins over the configured `Width`/`Height`/`X`/`Y` on load; a maximized-at-close flag reopens maximized only when `EnableMaximize` is set.

## Focus and behavior

### AutoFocus `bool` (default false)

Focuses the webview when the window activates. Set `true` for the main window so typing lands in the page immediately.

### AlwaysOnTop `bool` (default false)

`true` pins the window topmost at open (`SetWindowPos(HWND_TOPMOST)`) and makes `Caps().alwaysOnTop` report `true`. Note that the `<prefix>SetAlwaysOnTop` binding is added unconditionally, regardless of this field - the frontend can pin or unpin at runtime either way; `AlwaysOnTop` only sets the starting state and the reported capability.

### Debug `bool` (default false)

Enables the WebView2 devtools (right-click → Inspect, F12). Leave off in release builds.

### OnCloseRequest `func() appshell.CloseAction` (default nil)

Runs on every close request (the X button, Alt+F4, any `WM_CLOSE`) and returns what should happen: `CloseAllow`, `CloseCancel`, or `CloseHide`. `nil` means allow. This is the heart of "close to tray" and "you have unsaved work" flows and gets its own full treatment in [Close behavior and app lifecycle](close-and-lifecycle.md). One rule: it runs on the window thread, so never block in it.

### Corners `string` (default `""`)

The Windows 11 corner style, one of `""` (system default), `"round"`, `"small"` (small radius) or `"square"`. Applied via `DwmSetWindowAttribute`. Ignored on Windows 10 and on Linux/Mac, where the compositor owns window corners.

## Icon

### Icon `appshell.IconSource` (default zero value)

Sets the taskbar/Alt-Tab icon. `IconSource` has two fields: `PNG []byte` (raw PNG bytes drawn at runtime) and `SkipExe bool`. The zero value tries the exe's embedded icon resource first (`ExtractIconExW` on the running exe) and falls back to nothing; set `SkipExe: true` to skip the exe and go straight to your PNG. The [appicon](monitors-and-icons.md) package draws a decent placeholder if you have no art yet:

```go
Icon: appshell.IconSource{PNG: appicon.PNG(appicon.Render(32, appicon.DefaultPalette()))},
```

## Frame and chrome fields

These fields configure the frameless frame, the title-bar buttons and the invisible native hit-test that makes your drawn chrome behave like a real title bar. They are listed here for completeness; each gets its full treatment in [Frame & window chrome](window-chrome.md).

| Field | Type | Default | Purpose |
| --- | --- | --- | --- |
| `Framed` | `bool` | `false` (frameless) | Keep the OS title bar and frame instead of custom chrome. |
| `DisableMinimize` | `bool` | `false` | Remove the minimize button/capability. |
| `EnableMaximize` | `bool` | `false` | Allow maximize/restore (off by default). |
| `DisableClose` | `bool` | `false` | Ignore close requests entirely. |
| `CaptionHeight` | `int` | `40` | Height of the draggable top band (device px). |
| `CaptionLeftReserve` | `int` | `8` | Non-drag clickable zone on the left. |
| `CaptionRightReserve` | `int` | `150` | Non-drag zone where the window buttons live. |
| `ResizeMargin` | `int` | `6` | Edge thickness that starts a native resize. |

## Screen recording and capture

Gantry's window is a WebView2 surface, which Windows composites directly through the GPU (DirectComposition) - the window carries `WS_EX_NOREDIRECTIONBITMAP` and has no GDI redirection bitmap for a legacy capturer to read. The practical consequence: **OBS's legacy "Window Capture (BitBlt)" and some "Display Capture" paths record a black rectangle** where the app should be. This is not a Gantry bug and not something the app can style around - it is how every WebView2/Chromium-composited window behaves.

Capture it with a method that reads composited windows instead:

- **OBS:** add a **Windows Graphics Capture (WGC)** source (Sources → `+` → *Windows Capture*, and pick the **Windows 10 (1903+)** / *Windows Graphics Capture* method), then select the Gantry window. WGC reads the composited surface correctly. If a source only offers the BitBlt method, or shows black, that is the legacy path - switch it to Windows Graphics Capture.
- The same applies to every Gantry window: the main window, [widgets](widgets.md) and popups/[notifications](notifications.md) are each their own WebView2 window (separate processes), so capture each the same way.

On the window name you see in a recorder's window list: the built binary is named after your app (`gantry build` emits `dist/<os>/<arch>/<name>.exe`, where `<name>` is the `name` in `gantry.json`), and `Title` above sets the window/taskbar caption. A recorder that shows a generic entry is listing the underlying WebView2 window (whose native window class is the generic `"webview"`), not a different executable.

> A future release may add an opt-in to render without GPU compositing (so legacy BitBlt capture works too); today, Windows Graphics Capture is the supported path.

## Related pages

- [Frame & window chrome](window-chrome.md) - the frameless frame, the button toggles, the hit-test metrics, the bridge functions and `useShell()`.
- [Close behavior and app lifecycle](close-and-lifecycle.md) - `OnCloseRequest` in full, and `appshell.App`.
- [The TitleBar](../ui/titlebar.md) - the frontend half of the chrome contract.
- [Monitors and icons](monitors-and-icons.md) - the `appicon` and `monitors` packages the fields above lean on.
- [Win32 notes](../advanced/win32-notes.md) - what happens underneath.
