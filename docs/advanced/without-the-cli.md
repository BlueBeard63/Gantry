# Without the CLI

`gantry dev` and `gantry build` synthesize a Vite root in `.gantry/` so apps need no web tooling files. If an app outgrows that - custom Vite plugins, Tailwind, a different bundler layout - you can own the build yourself. Everything the CLI does is plain, documented wiring.

## What the synthesis contains

Three files, regenerated every run:

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
import { createApp } from "gantry-web";
createApp({ title: "Myapp" });
```

### vite.config.ts

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { gantry } from "gantry-web/vite";

export default defineConfig({
  plugins: [react(), gantry({ appRoot: ".." })],
  build: { outDir: "../dist", emptyOutDir: true },
});
```

## Owning it

Copy those three files into a folder you control (`web/`, or the app root), adjust **appRoot** to point at the folder holding `pages/` and `components/`, and add whatever you need - more plugins, Tailwind, env handling. From then on run vite yourself:

```
npx vite dev            (with: go run . --dev-url http://localhost:5173)
npx vite build          (then: go build .)
```

You lose `gantry dev/build` (they would overwrite nothing - they write only `.gantry/` - but running them would build from the synthesized root, not yours). gantry new, add and docs keep working.

## What the gantry() plugin does for you

Keep it in your config unless you want to re-implement it; it is the convention layer:

- generates `virtual:gantry-app` - the route table from `pages/` and the component registry from `components/`, keyed by folder path
- auto-imports root `index.css` and every colocated same-name css
- injects the pairing key into bare `usePaired()` calls from the file's folder path
- dev server: proxies `/api` and `/gantry/ws` to the Go port (read from `gantry.json`, or pass `gantry({ goPort: 9000 }`))
- watches `pages/` and `components/` so new pairs appear without a restart

Options: **appRoot** (folder containing `pages/`), **goPort** (proxy target).

## Skipping createApp

**createApp** is also optional - it is ordinary React bootstrapping. Wire it manually to control mounting, add providers, or use your own router:

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

You give up the generated router and automatic component registry; the **bridge**, **TitleBar**, **usePaired** (pass keys explicitly) and **TeaView** all work the same.

## Go without gantry.Run

`gantry.Run` is the one-call bootstrap (**flags**, **roles**, **server**, **window**, **tray**); its **Window/Setup/Roles** hooks cover most customization without leaving it. When an app truly outgrows it, copy the body of `gantry/gantry.go` into your `main.go` and own the loop - it is ordinary wiring around public APIs: `appshell.Listen`, an `http.ServeMux` with `app.Handler()` and `appshell.ServeSPA`, then `appshell.App.Run`.

One level further down, `appshell.App.Run` itself is ~60 lines of glue over **RunWindow**, **tray.Run** and **CloseMainWindow**. The one hard rule at every level: **RunWindow** must run on the **main goroutine**, and everything else on other goroutines. Read `appshell/app.go` for the reference loop.

The generated `gantry_registry.go` keeps working at any level - or drop it and pass **pages/components** to `ui.NewApp` by hand.
