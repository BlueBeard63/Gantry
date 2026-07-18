# Without the CLI

`gantry dev` and `gantry build` synthesize a Vite root in `.gantry/` so apps need no web-tooling files of their own. If an app outgrows that - custom Vite plugins, Tailwind beyond the built-in flag, a different bundler layout - you can own the build yourself. Everything the CLI does is plain, documented wiring; this page shows every layer so you can take over any one of them. See [Architecture](architecture.md) for how these pieces run at runtime.

## What the synthesis contains

`writeSynth` (`cmd/gantry/synth.go`) regenerates three files under `.gantry/` on **every** dev/build run - they are never hand-edited, and editing them is pointless. The app's own files are `main.go`, `pages/`, `components/`, `layouts/`, `index.css` and `package.json`.

### index.html

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Myapp</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/main.tsx"></script>
  </body>
</html>
```

### main.tsx

```tsx
import "gantry-web/styles.css";
import { createApp } from "gantry-web";
import * as app from "virtual:gantry-app";

createApp(app, { title: "Myapp" });
```

Two ordering facts are load-bearing. `styles.css` is imported first so the app's `index.css` can override the `--gantry-*` variables. And `virtual:gantry-app` (the route table and component registry the plugin generates) is imported HERE rather than inside gantry-web - only app code passes through the plugin that resolves the virtual id, which is what lets Vite prebundle `gantry-web` as a single module.

### vite.config.ts

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { gantry } from "gantry-web/vite";

export default defineConfig({
  plugins: [react(), gantry({ appRoot: ".." })],
  build: { outDir: "../webdist", emptyOutDir: true },
});
```

The output dir is `../webdist` - the directory `embed.go` bakes into the exe with `//go:embed all:webdist`. (Release binaries land in `dist/<os>/<arch>/`; do not confuse the two.) When `gantry.json` sets `"tailwind": true`, the synthesizer additionally injects `@tailwindcss/vite` into the plugin list - so you only need to own this file for Tailwind if you want a config beyond that flag.

## Owning the frontend build

Copy those three files into a folder you control (`web/`, or the app root), point **appRoot** at the folder holding `pages/` and `components/`, and add whatever you need - more plugins, a custom Tailwind config, env handling. From then on, run Vite yourself:

```
npx vite dev            (with: go run . --dev-url http://localhost:5173)
npx vite build          (then: go build .)
```

You lose `gantry dev`/`build`: they only ever write `.gantry/`, so they would not clobber your files, but running them would build from the synthesized root instead of yours. `gantry new`, `gantry add` and `gantry docs` keep working. Note that `--dev-url` is also the mode signal when the CLI is out of the loop - a plain `go run . --dev-url ...` runs in development mode (see [Modes](modes.md)).

## What the gantry() plugin does for you

Keep the plugin in your config unless you mean to re-implement it; it is the convention layer that makes the folder structure work:

- generates `virtual:gantry-app` - the route table from `pages/` and the component registry from `components/`, keyed by folder path.
- auto-imports the root `index.css` and every colocated same-name css.
- injects the pairing key into bare `usePaired()` calls, derived from the file's folder path.
- runs a dev server that proxies `/api` and `/gantry/ws` to the Go port (read from `gantry.json`, or pass `gantry({ goPort: 9000 })`).
- watches `pages/` and `components/` so new pairs appear without a restart.

Options: **appRoot** (the folder containing `pages/`) and **goPort** (the proxy target).

## Skipping createApp

`createApp` is also optional - it is ordinary React bootstrapping (mount, error handlers, the socket connect, the env fetch). Wire it manually to control mounting, add providers, or use your own router:

```tsx
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { TitleBar, installZoomGuard } from "gantry-web";
import "gantry-web/styles.css";
import Index from "../pages/index/index";

installZoomGuard();
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <TitleBar title="Myapp" />
    <Index />
  </StrictMode>,
);
```

You give up the generated router, the automatic component registry, and the built-in error UI wiring `createApp` installs (`errors` option; see [Errors](errors.md)); the **bridge**, **TitleBar**, **usePaired** (pass keys explicitly now) and **TeaView** all work exactly the same, since they only depend on the websocket.

## Go without gantry.Run

`gantry.Run` (`gantry/gantry.go`) is the one-call bootstrap: it parses flags, dispatches `--shellrole` helper windows, wires the error pipeline, registers the built-in `gantry` service (`appInfo`/`env`/`errors`/`clearErrors`/`reportError`/`breadcrumb`), mounts the frontend routes, calls `appshell.Listen`, and runs the shell. Its **Window** / **Setup** / **Roles** / **Errors** hooks cover most customization without leaving it. When an app truly outgrows it, copy the body of `run()` into your `main.go` and own the loop - it is ordinary wiring around public APIs:

```go
app := ui.NewApp(gantryPairs()...)          // pages + components
mux := http.NewServeMux()
mux.Handle("/gantry/ws", app.Handler())     // the websocket
// ... register your services, state, /api routes on app and mux ...
mux.Handle("/", appshell.ServeSPA(dist()))  // SPA catch-all, last
ln, _ := appshell.Listen(port)              // also the single-instance guard
go http.Serve(ln, mux)
shell := &appshell.App{Window: appshell.WindowOptions{ /* ... */ }}
shell.Run(ctx, cancel)                       // MUST be the main goroutine
```

One level further down, `appshell.App.Run` is ~90 lines of glue over **RunWindow**, **tray.Run** and **CloseMainWindow** (`appshell/app.go`). The one hard rule at every level: **RunWindow** must run on the **main goroutine** (the webview message loop requires it), and everything else on other goroutines. If you skip `gantry.Run` you also skip its crash-log setup and the built-in `gantry` service, so `useEnv`/`useMode`/`useAppInfo` and the [error pipeline](errors.md) go dark until you register that service yourself.

The generated `gantry_registry.go` (scanned from `pages/`, `components/` and `layouts/`) keeps working at any level - or drop it and pass **Page** and **Component** values to `ui.NewApp` by hand.
