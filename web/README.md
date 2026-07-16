# gantry-web

The frontend half of [Gantry](https://github.com/B-Commissions/Gantry),
a Go framework for building native desktop apps with React interfaces.
This package provides the window chrome, the bridge to the native
window, the Tea runtime, the router, and the Vite plugin that powers
Gantry's paired-file convention.

You normally do not install this by hand - `gantry new` scaffolds an
app with it wired up. See the
[Gantry documentation](https://github.com/B-Commissions/Gantry/tree/main/docs)
or run `gantry docs` for the full story.

## What it exports

From `gantry-web`:

- `TitleBar`, `DragStrip`, `ResizeFrame` - the custom window chrome
- `getShell` / `useShell` / `useShellCaps` - the typed native bridge
  (close, minimize, maximize, drag, attention, always-on-top), with
  feature detection so the same app runs in a plain browser tab
- `createApp` - boots a Gantry frontend (router, layouts, registry)
- `Link`, `ExternalLink`, `navigate`, `goBack`, `goForward`,
  `useRoute`, `isActive` - navigation with active-state styling
- `usePaired` - the channel between a .tsx file and the .go file next
  to it (send events, await calls, receive pushes)
- `useService`, `useCall`, `service` - awaited calls into Go services
- `useGoState` - useState whose value lives in Go, synced both ways
- `installZoomGuard` - suppress browser zoom gestures

From `gantry-web/tea`:

- `TeaView` - renders a page whose UI logic lives in Go
  (Model/Update/View)
- `TeaComponentProps`, `setRegistry` - custom component integration

From `gantry-web/vite`:

- `gantry()` - the Vite plugin: discovers pages/, components/ and
  layouts/ folders, auto-imports colocated css, injects pairing keys,
  and proxies to the Go server during development

## Peer dependencies

react and react-dom (18+), lucide-react for the chrome icons, and vite
(only when using the plugin). The package ships TypeScript source
directly; Vite consumes it as-is.
