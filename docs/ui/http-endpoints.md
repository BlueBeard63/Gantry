# Serving your own HTTP routes

Most Go<->React traffic goes over the websocket - [paired events](pairs.md), [calls and services](calls-and-services.md), [Tea](tea.md) renders. Sometimes you want a plain HTTP endpoint instead: a file download, an image the browser loads with a normal `<img src>`, a webhook a non-Gantry client posts to, or a `fetch` that returns a blob. Gantry hands you the app's own `*http.ServeMux` so you can register those routes on the same local server.

## The Setup hook

`Config.Setup` runs once before the server starts serving. Its signature gives you the app and the mux:

```go
Setup: func(app *ui.App, mux *http.ServeMux) {
    mux.HandleFunc("/api/report.csv", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/csv")
        w.Write(buildCSV())
    })
},
```

Setup is also where you register [services](calls-and-services.md) (`app.Service`) and [shared state](state.md) (`ui.NewState`); adding HTTP routes is the third thing it is for.

## Fetching it from the frontend

The endpoint lives on the same origin as the app, so the frontend hits it with a relative URL - no host, no port to thread through:

```tsx
const res = await fetch("/api/report.csv");
const text = await res.text();
```

In `gantry dev` the frontend is served by Vite while your Go app runs beside it; the CLI proxies `/api` (and `/gantry/ws`) back to the Go server, so the same relative fetch works in dev and in the built app with no code change. Use an `/api` prefix (or another distinct path) for your routes so they fall inside that proxy.

## What is already claimed

The mux already has a few prefixes registered before your Setup runs, so pick paths that avoid them:

- `/gantry/` - the websocket and internal endpoints (`/gantry/ws`, widgets, notifications).
- `/resources/` - the embedded [resources](resources.md) tree, when the app has one.
- `/` - the SPA catch-all (`index.html` for any unmatched path), registered *after* Setup, so a specific route you add always wins over it. This is why a distinct prefix like `/api/` matters: it keeps your endpoints from being shadowed by, or shadowing, the single-page app.

The server binds `127.0.0.1` only (see [Architecture](../advanced/architecture.md)), so these routes are reachable from the machine, never the network.
