# Pairs: the paired-file model

A Gantry feature is a **pair**: a .tsx file and the .go file next to it, talking over one websocket. Pages and components are both pairs - they share this whole mechanism and differ only in what makes them a page or a component. This page is the pairing system itself: how the two halves find each other, how data flows both ways, and how a pair registers itself.

## Keys: how the halves find each other

Every pair is identified by its folder path relative to the app root: "pages/index", "pages/settings", "components/example". The Go half declares it; the tsx half gets it injected by the Vite plugin, so only one side ever writes the string:

```go
// pages/index/index.go
var Page = ui.Page{Key: "pages/index"}
```

```tsx
// pages/index/index.tsx
const { send, on, state } = usePaired(); // key filled in at build time
```

The key is always the folder path, and the css scope class follows it (a pair at pages/account/settings/profile scopes under gantry-pages-account-settings-profile - see [Styling](styling.md)). Pages, components and layouts all nest to any depth and key the same way.

## Data flow: tsx -> Go

usePaired().send fires a named handler with any JSON-serializable payload:

```tsx
send("save", { name: "jack" });
```

```go
On: ui.Handlers{
    "save": func(p json.RawMessage) {
        var body struct{ Name string `json:"name"` }
        _ = json.Unmarshal(p, &body)
        // ...
    },
},
```

Handlers run on the websocket read loop - do quick work inline, spawn a goroutine for anything slow. This channel is one-way, fire-and-forget. When the tsx needs an ANSWER back, or when the functionality is app-wide (auth, settings) or the state lives in Go, reach for [Calls and services](calls-and-services.md).

## Data flow: Go -> tsx

app.Push sends a named event to a pair's tsx side:

```go
app.Push("components/example", "state", currentStats)
```

The tsx receives it two ways:

- **state**: pushes named "state" land in usePaired().state automatically - the one-liner for mirroring Go data into a component.
- **on**: subscribe to any event name:

```tsx
useEffect(() => on("progress", (p) => setProgress(p as number)), [on]);
```

Push targets the connected window. A desktop app has exactly one connected client at a time (a reload or dev restart replaces it), so Push is fire-and-forget - a push with no client is silently dropped.

## Registration is automatic

Adding a new pair is two steps:

1. Make the folder and files: pages/stats/stats.go + stats.tsx (+ stats.css if you want styles).
2. In stats.go: package stats, export `var Page = ui.Page{...}` (or `var Component = ui.Component{...}`).

That is it. The frontend discovers the tsx by its location (the Vite plugin); the Go side is generated - gantry dev/build scan pages/, components/ and layouts/ for the exported Page/Component vars and regenerate gantry_registry.go, which main.go consumes as `ui.NewApp(gantryPairs()...)`. Run `gantry gen` to regenerate it by hand (e.g. before a plain `go build`); never edit the generated file. During gantry dev, adding a folder hot-reloads the frontend, but the Go side needs a dev restart (Go compiles).

## Two styles, mixed freely

- **Paired handlers (plain style)**: the UI is React through and through, Go is the backend. Familiar if you come from web dev; state lives in the browser.
- **Tea Model**: the UI logic and state live in Go, React renders it. One language for the whole feature, state survives frontend reloads, and everything is trivially testable Go. See [The Tea model](tea.md).

Mix them per pair, or even both on one pair.

## Notes

- **Nothing crashes on a bad key.** An unregistered handler logs "ui: no handler for pages/index.foo (check the Key ...)" on the Go side; an unresolved Custom() name renders a visible `[unknown component: x]` placeholder rather than failing silently.
- **Where pairs go from here.** A pair becomes a routable [page](pages.md) when its Go half is a `ui.Page`, a reusable [component](components.md) when it is a `ui.Component`, and shared chrome when it lives under [layouts/](layouts.md). The mechanism above is identical across all three.
