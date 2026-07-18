# Architecture

How a Gantry app actually runs: the processes it spawns, the transports between them, and the build pipeline that turns source into a single binary. This page is the mental model; the [wire protocol](protocol.md), [Windows internals](win32-notes.md) and [manual wiring](without-the-cli.md) pages drill into the pieces.

## The process shape

```
myapp.exe (main process)
  |- local HTTP server on 127.0.0.1:<port>
  |    |- /            the embedded React build (or the Vite dev server in dev)
  |    |- /gantry/ws   the ui websocket (pages, components, Tea)
  |    |- /api/...     your own endpoints, if any
  |- native main window (WebView2) pointed at that server
  |- tray icon (optional)
  |- ProcManager children:
       |- myapp.exe --shellrole widget-x   (each widget: own process)
       |- myapp.exe --shellrole popup      (each popup: own process)
```

One binary plays every role. `main()` checks `--shellrole` first and, when asked, runs a single helper window instead of the whole app. Helper windows load their pages from the MAIN process's server, so all logic stays in one place - the children are pure renderers.

## Why serve HTTP at all?

The webview needs a URL, and serving the SPA locally buys three things at once: one code path for the native window, `--browser` mode and dev (only the URL changes); websockets that work naturally (the ui layer needs server push); and a fixed port that doubles as the single-instance guard. The server binds `127.0.0.1` only, so nothing is reachable from the network.

## Why child processes for widgets and popups?

Two reasons. Crash isolation: a renderer crash kills one widget, never the app, and teardown is just a process kill - no half-dead window states. And a WebView2 constraint: two webviews in different processes cannot share a browser-data folder, so giving each role its own process (and its own folder) sidesteps a whole family of startup failures. See [Win32 notes](win32-notes.md) for the folder-per-role detail.

## The two transports

A Gantry window talks to Go over two separate channels, each suited to a different job:

- **The bridge** (`window.gantry*` functions): tiny, window-scoped controls that Go injects into the page - close, minimize, drag, resize. It works with zero setup and is injected per window, but it is one-way: Go cannot push through it.
- **The websocket** (`/gantry/ws`): everything app-level - paired events, pushes, Tea renders, awaited calls. One connection per client, it survives reloads by reconnecting, and the server treats the newest connection as THE client.

Rule of thumb: window verbs go over the bridge, app data goes over the socket. The full frame-by-frame grammar is in the [protocol](protocol.md) page.

## The build pipeline

`gantry build` runs `vite build` (bundling every **page**, **component**, **colocated css** and **gantry-web** itself into `dist/`) and then `go build`, where `embed.go`'s `//go:embed all:dist` bakes `dist/` into the exe. The result is a single file: Go binary plus frontend, and no docs (those live in the CLI, not your app).

In dev the pipeline inverts: the frontend is served by Vite for HMR, the Go app runs beside it, and the native window loads the Vite URL with `/api` and `/gantry/ws` proxied back. Either way the app code is identical - only the URL the window loads changes. To own this pipeline yourself, see [Without the CLI](without-the-cli.md).

## Platforms

Windows (WebView2 + raw Win32) and Linux (WebKitGTK + GTK3) both get the full shell: frameless custom chrome, native drag, window buttons, min/max sizes, always-on-top, the close hook, geometry persistence, widgets, popups, monitors and the tray. The one visible difference is edge resizing - Windows re-implements the invisible native frame in the window's hit-test (see [Win32 notes](win32-notes.md)), while on Linux the frontend renders thin resize strips (gantry-web's ResizeFrame, added automatically) that hand off to the compositor's interactive resize.

Linux caveats: window positioning and saved geometry are best-effort under pure Wayland (the protocol does not let apps place themselves; X11 and XWayland honor it), corner rounding belongs to the compositor, and the tray needs a shell with a status-icon host (most desktops; not WSLg).

Mac is future work (no hardware to test against yet): apps still run there via the browser fallback, and OpenInBrowser already knows "open". `nogui` builds (`-tags nogui`) strip the native window on every platform - useful for CI and headless servers. The build-tag split lives inside appshell; app code does not branch.
