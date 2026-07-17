# Gantry

A Go framework for building native desktop apps with React frontends. Extracted from the Timekeep app shell: frameless WebView2 windows with custom title bars, window movement and sizing, tray apps, always-on-top widgets, notification popups, and an optional Bubble-Tea-style UI layer where app logic lives in Go and renders as real React components. The same app also builds for Android - an installable APK where the Go server runs on the phone, with home-screen widgets and system notifications driven from Go.

Apps follow a paired-file convention: every page and component is a folder with a `.go`, `.tsx`, and optional `.css` file side by side. No web/ directory - the `gantry` CLI synthesizes the build tooling.

```
myapp/
  main.go
  index.css                    # app-wide theme (CSS variables)
  pages/index/index.go,.tsx,.css
  components/gauge/gauge.go,.tsx,.css
```

## Quick start

```
go install github.com/B-Commissions/Gantry/cmd/gantry@latest
gantry new myapp
cd myapp
gantry dev      # live-reload development in a native window
gantry build    # single .exe with embedded frontend
```

Apps declare custom launch args in gantry.json (`gantry dev --mock-data` validates them and hands them to the app as env vars, in development and production - see [App args](docs/advanced/args.md)), run in a `development` or `production` [mode](docs/advanced/modes.md) for gating pages and features, and get [crash handling](docs/advanced/errors.md) on by default: render crashes show an error screen instead of a white window, Go panics surface in the frontend with the page and the trail of actions that led there, and every error is interceptable from app code.

## Mobile

`gantry build --targets android` packs the same app into an APK: the Go server cross-compiles for the phone and runs on-device behind a full-screen WebView shell - no gomobile, no Java in your project. Friendly permission names in gantry.json, home-screen widgets from paired `widgets/<name>/<name>.go` files, and system notifications posted straight from Go. `gantry mobile dev android` is the phone dev loop: plug in over USB and it builds, installs, launches and streams logcat. A missing toolchain skips the target with a fix hint, never fails the build. iOS ships as an experimental Xcode scaffold. See the [Mobile docs](docs/mobile/android.md).

## Documentation

Full docs live in [docs/](docs/README.md), or run `gantry docs` for the offline docs browser in your terminal.

## Packages

- `appshell` - frameless main window, widgets, popups, lifecycle, single instance
- `tray` - system tray icon with menus (checkable items, submenus)
- `notify` - out-of-process notification popups + attention flash
- `monitors` - multi-monitor enumeration and placement
- `appicon` - runtime-drawn fallback icons (PNG/ICO)
- `ui` - the Tea-style Go-driven UI layer (pages, components, Model/Update/View)
- `gerr` - coded errors ("config.unknown-arg", "panic.call") used across the CLI and runtime
- `widget` - declarative layout trees for Android home-screen widgets
- `notification` - system notifications with actions (Android)
- `web/` - the gantry-web npm package (TitleBar, bridge, Tea runtime, Vite plugin)
- `cmd/gantry` - the CLI (new, dev, build, add, update, upgrade, docs, mobile)
