# Monitors and icons

Two small packages the shell leans on: monitors for multi-display placement, appicon for apps that have no artwork yet.

## monitors

```go
import "github.com/B-Commissions/Gantry/monitors"

all := monitors.All()          // every attached display
m := monitors.Pick(all, index) // by index; -1 = primary; always returns something
```

Each Monitor carries two rectangles in virtual-desktop pixels:

- X, Y, Width, Height - the full display.
- WorkX, WorkY, WorkWidth, WorkHeight - the work area, which excludes the taskbar. Widgets, popups and maximized windows all place against the work area so they never sit under or over the taskbar.

Coordinates can be negative: a monitor left of the primary starts at a negative X. Pick never fails - with a bad index it returns the primary, and with no data at all a sane 1920x1080 stand-in, so placement code needs no error handling.

Most of the time you never call this package: `WidgetOptions.Monitor`, `PopupOptions.Monitor` and geometry-restore clamping all use it for you. Reach for it directly when building your own placement - a settings page listing displays by name, for instance (Monitor.Name and Primary are there for exactly that).

## appicon

Windows wants icons in several places (taskbar, alt-tab, tray, exe) and two formats (PNG-ish HICONs at runtime, ICO containers for the tray/exe). appicon draws a clean placeholder glyph at any size and packs the containers, so a young app looks intentional without any asset work:

```go
import "github.com/B-Commissions/Gantry/appicon"

img := appicon.Render(32, appicon.DefaultPalette()) // draw the glyph
png := appicon.PNG(img)                             // window icon bytes
ico := appicon.ICO(img)                             // tray icon bytes
```

Recolor it to your app in one line:

```go
p := appicon.Palette{
    BG: color.NRGBA{R: 16, G: 16, B: 18, A: 255},
    FG: color.NRGBA{R: 110, G: 168, B: 254, A: 255},
}
```

When you have real artwork, skip the package: put PNG bytes in `appshell.IconSource{PNG: ...}` and ICO bytes in tray.Options.Icon.

---

## Advanced: exe icons (release builds)

The window icon defaults to whatever icon resource is embedded in the exe, falling back to your IconSource PNG. To embed one (so Explorer and shortcuts show it too), generate a multi-resolution ICO with appicon.MultiICO and compile it in with the rsrc tool:

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

The .syso file sitting in the package folder is picked up by go build automatically.
