# Architecture

How a Gantry app actually runs: the processes, the transports, and the build pipeline.

## The shape

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

One binary plays every role: `main()` checks `--shellrole` first and runs a single helper window instead of the app when asked. Helper windows load their pages from the MAIN process's server, so all logic stays in one place - the children are pure renderers.

## Why serve HTTP at all?

The webview needs a URL; serving the SPA locally means:

- one code path for the native window, `--browser` mode, and dev (only the URL changes)
- websockets work naturally (the ui layer needs server push)
- the fixed port doubles as the single-instance guard

The server binds `127.0.0.1` only - nothing is reachable from the network.

## Why child processes for widgets/popups?

Crash isolation (a renderer crash kills a widget, never the app), plus a WebView2 requirement: two webviews in different processes cannot share a browser-data folder, and giving each role its own folder sidesteps a family of startup failures. Teardown is also just a process kill - no half-dead window states.

## The two transports

- The bridge (`window.gantry*` functions): tiny, window-scoped controls bound by Go into the page - close, minimize, drag, resize. Injected per window; works with zero setup; cannot push from Go.
- The websocket (`/gantry/ws`): everything app-level - paired events, pushes, Tea renders. One connection per client, survives reloads by reconnecting, and the server treats the newest connection as THE client.

Rule of thumb: window verbs go over the bridge, app data goes over the socket.

## The build pipeline

`gantry build` runs `vite build` (bundling every **page**, **component**, **colocated css** and **gantry-web** itself into `dist/`) and then `go build`, where
`embed.go's` `//go:embed all:dist` bakes `dist/` into the **exe**. The result is a single file: Go binary + frontend + docs-free (docs live in the CLI, not your app).

In dev the pipeline inverts: the frontend is served by Vite (HMR), the Go app runs beside it, and the native window loads the Vite URL with `/api` and `/gantry/ws` proxied back.

## Platforms

Windows (WebView2 + raw Win32) and Linux (WebKitGTK + GTK3) both get the full shell: frameless custom chrome, native drag, window buttons, min/max sizes, always-on-top, the close hook, geometry persistence, widgets, popups, monitors and the tray. The one visible difference is edge resizing: Windows re-implements the invisible native frame in the window's hit-test, while on Linux the frontend renders thin resize strips (gantry-web's ResizeFrame, added automatically) that hand off to the compositor's interactive resize.

Linux caveats: window positioning and saved geometry are best-effort under pure Wayland (the protocol does not let apps place themselves; X11 and XWayland honor it), corner rounding belongs to the compositor, and the tray needs a shell with a status-icon host (most desktops; not WSLg).

Mac is future work (no hardware to test against yet): apps still run there via the browser fallback, and OpenInBrowser already knows "open". nogui builds (`-tags nogui`) strip the native window on every platform - useful for CI and headless servers. The build-tag split lives inside appshell; app code does not branch.
