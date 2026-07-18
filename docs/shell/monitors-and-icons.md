# Monitors and icons

Two small packages the shell leans on: `monitors` for multi-display placement, and `appicon` for apps that have no artwork yet. Both are things `WidgetOptions`, `PopupOptions`, geometry restore and the tray use for you - reach for them directly only when you build your own placement or your own icons.

## monitors

```go
import "github.com/B-Commissions/Gantry/monitors"

all := monitors.All()          // every attached display
m := monitors.Pick(all, index) // by index; -1 = primary; always returns something
```

### The Monitor type

Each `Monitor` carries two rectangles in virtual-desktop pixels (the primary monitor's top-left is `(0,0)`; monitors to its left or above start at negative origins):

- `Index int` - the display's index, matching what `WidgetOptions.Monitor` / `PopupOptions.Monitor` take.
- `Name string` - a human-readable display name, for listing monitors in a settings UI.
- `X, Y, Width, Height int` - the full display bounds.
- `Primary bool` - whether this is the primary display.
- `WorkX, WorkY, WorkWidth, WorkHeight int` - the **work area**, which excludes taskbars and docks. Widgets, popups and maximized windows all place against the work area so they never sit under or over the taskbar.

All fields carry JSON tags (`index`, `name`, `x`, `workX`, ...), so `monitors.All()` serializes straight to a settings page listing displays by name.

### All and Pick

- `monitors.All() []Monitor` - enumerates the attached displays.
- `monitors.Pick(monitors []Monitor, index int) Monitor` - returns the monitor with that index; `-1` explicitly requests the primary. It never fails: a bad index falls back to the primary, an empty slice falls back to a sane `1920x1080` stand-in (`WorkHeight` 1040). So placement code needs no error handling.

Most of the time you never call this package: `WidgetOptions.Monitor`, `PopupOptions.Monitor` and geometry-restore clamping all use it internally. Reach for it directly when building your own placement - a settings page listing displays, for instance - via `Monitor.Name` and `Monitor.Primary`.

## appicon

Windows wants icons in several places (taskbar, alt-tab, tray, exe) and two formats (PNG at runtime, ICO containers for the tray and exe). `appicon` draws a clean placeholder glyph - the Gantry mark (a portal frame with a hoist), the same shape as the scaffold's SVG logo - at any size and packs the containers, so a young app looks intentional without any asset work:

```go
import "github.com/B-Commissions/Gantry/appicon"

img := appicon.Render(32, appicon.DefaultPalette()) // draw the glyph -> *image.NRGBA
png := appicon.PNG(img)                             // window icon bytes ([]byte)
ico := appicon.ICO(img)                             // tray icon bytes ([]byte)
```

The functions and types:

- `appicon.Palette` - two colors, `BG color.NRGBA` (the disc background) and `FG color.NRGBA` (the ring and hands).
- `appicon.DefaultPalette() Palette` - the Gantry accent blue on a dark disc (the same colors as the default `--gantry-*` theme).
- `appicon.Render(sz int, p Palette) *image.NRGBA` - draw the glyph at any edge length in the given palette.
- `appicon.PNG(img image.Image) []byte` - encode an image as PNG bytes (window/taskbar icons; `IconSource.PNG` and tray `IconPNG` want this).
- `appicon.ICO(img *image.NRGBA) []byte` - wrap one image in a single-frame ICO container (Windows tray `Options.Icon` wants this).
- `appicon.MultiICO(render func(sz int) *image.NRGBA) []byte` - build a multi-resolution `.ico` for embedding into an exe (see below).

Recolor it to your app in one line:

```go
p := appicon.Palette{
    BG: color.NRGBA{R: 16, G: 16, B: 18, A: 255},
    FG: color.NRGBA{R: 110, G: 168, B: 254, A: 255},
}
```

When you have real artwork, skip the package: put PNG bytes in `appshell.IconSource{PNG: ...}` and ICO bytes in `tray.Options.Icon` (and `IconPNG` for Linux/Mac).

---

## Advanced: exe icons (release builds)

The window icon defaults to whatever icon resource is embedded in the exe, falling back to your `IconSource` PNG. To embed one (so Explorer and shortcuts show it too), build a multi-resolution ICO with `appicon.MultiICO` - it packs DIB frames at the classic shell sizes (16, 24, 32, 48, 64) and PNG frames at the large ones (128, 256) - and compile it in with the `rsrc` tool:

```go
ico := appicon.MultiICO(func(sz int) *image.NRGBA {
    return appicon.Render(sz, myPalette)
})
os.WriteFile("app.ico", ico, 0o644)
```

```
go run github.com/akavel/rsrc@latest -ico app.ico -o rsrc_windows_amd64.syso
go build .
```

The `.syso` file sitting in the package folder is picked up by `go build` automatically.
