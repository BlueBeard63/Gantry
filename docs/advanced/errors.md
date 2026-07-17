# Errors and crash handling

Gantry ships with crash detection on by default. A React render crash shows an error screen instead of a white window; a Go panic in a handler surfaces in the frontend instead of vanishing into a log line; even a hard process crash leaves a trace that the next launch reports. Every captured error carries the page the user was on and a breadcrumb trail of the actions that led there, so a report reads as a story ("was on /settings, clicked save, auth.login failed, then the panic") rather than a bare stack. Everything is interceptable and replaceable per app, on both the Go and React side.

## What gets caught

| Kind | Code | What happened | Result |
| --- | --- | --- | --- |
| `react-render` | `react.render` | a page/component threw during render | full-screen error overlay (the window chrome keeps working - drag and close stay alive) |
| `js-error` | `js.error` | an uncaught JS exception | notice banner in development, console in production |
| `js-rejection` | `js.rejection` | an unhandled promise rejection | notice banner in development, console in production |
| `call-panic` | `panic.call` | a Go call handler panicked | the awaiting call rejects with the code + a notice banner |
| `event-panic` | `panic.event` | a paired event handler panicked | notice banner |
| `cmd-panic` | `panic.cmd` | a Tea command goroutine panicked | notice banner |
| `tea-update-panic` | `panic.update` | a Model's Update panicked | model keeps its last good state, notice banner |
| `tea-view-panic` | `panic.view` | a Model's View panicked | last good render stays up, notice banner |
| `http-panic` | `panic.http` | an /api handler panicked | coded 500 JSON response + notice banner |
| `goroutine-panic` | `panic.goroutine` | a `gantry.Go(fn)` goroutine panicked | app stays alive, notice banner |
| `process-crash` | `panic.fatal` | an uncatchable crash killed the process | trace lands in crash.log; the next launch reports it |

JS-side errors are also reported back to Go, so the `gantry dev` terminal, the error ring buffer and the app's `OnError` hook see every error from both sides through one pipeline.

## The pipeline

Capture site → stamp page + breadcrumb trail → ring buffer (last 20, `call("gantry","errors")`) → the app's `OnError` hook → an `{"t":"error"}` websocket frame → the frontend error UI (built-in or custom). The breadcrumb trail (last 50 actions) records itself automatically from the websocket traffic: page navigations, paired events, service calls with their ok/err outcome, and state writes. Add your own context lines with `app.Breadcrumb("sync started")` in Go or `addBreadcrumb("import chosen")` from React.

## The built-in error UI

Two severities. A **fatal** error (render crash) takes the content area over: in development you get the message, the JS and component stacks, the page, a "What led here" timeline and a Reload button; in production a friendly "Something went wrong" card with Reload. A **notice** (Go panics, rejections - the app is still alive) is a dismissible banner, shown in development and kept to the console in production. The window chrome sits outside the error boundary on purpose: a crashed page can always still be dragged and closed.

## Intercepting in Go

`Config.Errors` is the app's hook into the pipeline:

```go
gantry.Run(gantry.Config{
    // ...
    Errors: gantry.ErrorOptions{
        OnError: func(e gantry.ErrorInfo) gantry.ErrorAction {
            // e.Kind, e.Code, e.Message, e.Stack, e.Page, e.Trail
            saveToMyLog(e)
            if e.Kind == "call-panic" {
                myState.Set("showError", true) // drive your own UI instead
                return gantry.ErrorSuppress    // keep it away from the built-in screen
            }
            return gantry.ErrorShow
        },
        OmitStacks: false, // true strips stacks before they reach the frontend
        Disable:    false, // true turns the whole pipeline off
    },
})
```

Suppressed errors stay in the ring buffer, so `call("gantry","errors")` still returns them. For background work you start yourself, prefer `gantry.Go(fn)` over a bare `go func()` - it recovers a panic into the pipeline instead of letting it kill the process.

## Intercepting in React

`createApp` (usually via the root `app.tsx`) takes an `errors` option:

```tsx
export default {
  errors: {
    screen: MyErrorScreen,          // replace the built-in UI (receives ErrorScreenProps)
    onError: (e) => {               // intercept every error
      track(e);
      return false;                 // false suppresses the default UI
    },
    showGoErrors: "development",    // Go panic banners: true | false | "development" (default)
    reportToGo: true,               // send JS errors to the Go side (default)
    enabled: true,                  // false restores the old white-screen behavior
  },
} satisfies CreateAppOptions;
```

A custom screen gets `{ error, mode, variant, onDismiss }` - render whatever fits the app; `useGantryErrors()` exposes the store directly for fully custom arrangements.

## Uncatchable crashes

Go cannot recover a panic on a goroutine it did not wrap: a bare `go func()` that panics kills the process - no hook can run in that moment. What Gantry does instead: at startup it points the runtime's fatal-trace output at `<config dir>/<app>/crash.log` (so the trace survives even in windowed builds with no console), and on the next launch a waiting trace is reported through the normal pipeline as `process-crash` - your `OnError` hook runs, and the frontend shows a "crashed last run" notice with the full trace in development. The trail is empty for these (the process died before it could be snapshotted).

## Coded errors (gerr)

Framework and CLI errors carry stable, greppable codes - `config.unknown-arg`, `panic.call`, `dev.vite-start` - via the `gerr` package:

```go
import "github.com/B-Commissions/Gantry/gerr"

return nil, gerr.New("auth.expired", "session expired").WithHint("log in again")
```

An error a call handler returns is control flow, not a crash: it rejects the awaiting frontend call (never the error screen), and its code travels with it - `GantryCallError.code` on the rejection, `code` on `useCall` results - so the frontend can switch on `"auth.expired"` instead of string-matching messages. `gerr.CodeOf(err)` reads a code back out of any wrapped error chain.

### Code reference

| Code | Meaning |
| --- | --- |
| `config.not-found` | no gantry.json here or in any parent |
| `config.parse` | gantry.json is not valid JSON |
| `config.bad-arg-spec` | an `args` declaration is invalid (name, type, default or env) |
| `dev.vite-start` | the Vite dev server failed to start |
| `dev.go-start` | `go run` failed to start |
| `panic.call` / `panic.event` / `panic.cmd` / `panic.update` / `panic.view` | a recovered Go panic (see the kinds table) |
| `panic.http` / `panic.goroutine` / `panic.fatal` | HTTP handler panic, gantry.Go panic, process crash |
| `js.error` / `js.rejection` / `react.render` | frontend-side captures |

Apps are encouraged to mint their own codes in the same `category.name` shape for errors their handlers return.
