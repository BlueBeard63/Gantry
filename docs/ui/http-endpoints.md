# Serving your own HTTP routes

Most Go<->React traffic goes over the websocket - [paired events](pairs.md), [calls](calls.md) and [services](services.md), [Tea](tea-model.md) renders. Sometimes a plain HTTP endpoint is the right tool instead: a file download, an image the browser loads with a normal `<img src>`, a webhook a non-Gantry client posts to, a `fetch` that returns a blob, an SSE stream. Gantry hands you the app's own `*http.ServeMux` so you register those routes on the same local server.

## The Setup hook

`Config.Setup` runs once, before the server starts serving. Its signature gives you both the app and the mux:

```go
// gantry.Config
Setup func(app *ui.App, mux *http.ServeMux)
```

```go
gantry.Run(gantry.Config{
    // ...
    Setup: func(app *ui.App, mux *http.ServeMux) {
        mux.HandleFunc("/api/report.csv", func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "text/csv")
            _, _ = w.Write(buildCSV())
        })
    },
})
```

`Setup` is the single place all three server-side registrations happen: [services](services.md) (`app.Service`), [shared state](state.md) (`ui.NewState`), and HTTP routes on `mux`. It runs on the goroutine that builds the server, so keep it quick and non-blocking - spin up any long-lived workers as goroutines.

Handlers you register are wrapped by the same middleware as the built-in routes: a **recover** layer (a panic in your handler is caught and fed to the [error pipeline](../advanced/errors.md) instead of crashing the process) and, when the app runs with a `--token`, a **token** gate (the mobile shell sets one; on the desktop there is none). You get those for free - write an ordinary `http.Handler`.

## Fetching it from the frontend

The endpoint lives on the same origin as the app, so the frontend hits it with a relative URL - no host, no port to thread through:

```tsx
const res = await fetch("/api/report.csv");
const text = await res.text();
```

In `gantry dev` the frontend is served by Vite while your Go app runs beside it; the Vite plugin **proxies `/api` and `/gantry/ws`** back to the Go server, so the same relative fetch works in dev and in the built app with no code change. This is why an `/api` prefix (or another path you know the proxy forwards) matters for dev: put your routes under it. (Resources are the exception - `/resources/` is served by Vite directly off disk in dev, not proxied; see [Resources](resources.md).)

## What is already claimed

The mux has a few prefixes registered before and after your `Setup`, so pick paths that avoid collisions. Registered **before** `Setup`:

- `/gantry/ws` - the websocket endpoint (`App.Handler`).
- `/gantry/widgets.json` and `/gantry/notify/action` - internal widget/notification endpoints.
- `/resources/` - the embedded [resources](resources.md) tree, mounted only when the app has a `resources/` directory.

Registered **after** `Setup`:

- `/` - the SPA catch-all, serving `index.html` for any otherwise-unmatched path. Because Go's `ServeMux` matches the longest registered prefix, a specific route you add (like `/api/...`) always wins over this catch-all - which is exactly why a distinct prefix keeps your endpoints from being shadowed by, or shadowing, the single-page app.

The server binds `127.0.0.1` only (via `appshell.Listen`, which doubles as the single-instance guard - see [Architecture](../advanced/architecture.md)), so these routes are reachable from the local machine, never the network.

## Reaching app state from a handler

A handler closes over whatever you give it - and `Setup` hands you the `*ui.App`, so a route can read app state, push to a page, or broadcast a Tea message:

```go
Setup: func(app *ui.App, mux *http.ServeMux) {
    mux.HandleFunc("/api/hooks/sync", func(w http.ResponseWriter, r *http.Request) {
        app.Send(syncRequested{})          // into every running Tea Model's Update
        app.Push("pages/status", "state", latestStatus()) // to a paired tsx
        w.WriteHeader(http.StatusNoContent)
    })
},
```

`App.Send` and `App.Push` are safe to call from the handler's goroutine; see [Commands & messages](tea-commands.md#appsend-reaching-every-page) and [Pairs](pairs.md).
