# Pages and components

A Gantry feature is a **pair**: a .tsx file and the .go file next to it, talking over one websocket. This page is the pairing system in full - how the two halves find each other, how pages route, and how data flows both ways.

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

## Pages

A page is a routable pair. The Go half:

```go
var Page = ui.Page{
    Key:   "pages/settings",
    Route: "/settings",       // optional; derived from Key when empty
    Model: func() ui.Model { return model{} }, // optional Tea model
    On: ui.Handlers{          // optional plain handlers
        "save": func(p json.RawMessage) { /* ... */ },
    },
}
```

Route derivation follows the folder: pages/index -> "/", pages/anything -> "/anything", nesting to any depth (pages/account/settings -> "/account/settings"), with an "index" leaf mapping to its parent (pages/account/index -> "/account"). A folder can both BE a page and CONTAIN pages:

```
pages/
  account/
    settings/
      settings.tsx, settings.go, settings.css   -> /account/settings
      profile/
        profile.tsx, profile.go                 -> /account/settings/profile
```

The key is always the folder path, and the css scope class follows it (gantry-pages-account-settings-profile - see [Styling](styling.md)). Components and layouts nest the same way.

A page needs a Model, handlers, both, or neither: a purely static page needs no .go logic at all (though the .go file must still exist if main.go registers it). Give it a `Model` to run the whole UI as a Go state machine - see [The Tea model](tea.md).

The tsx half's default export is the page component. Two optional named exports tune it:

- `export const chrome = false` hides the [TitleBar](titlebar.md) on this page (widgets, popups).
- `export const route = "/x"` overrides the route (keep it matching the Go side if both are set).

Routing is plain pathname switching with no dependency: navigate("/settings") pushes history and re-renders, and back/forward work. With a single page (only pages/index) the router disappears entirely.

## Components

A component is a reusable pair. Two ways to use one:

1. Import it like any React component - normal composition, typed props, the usual:

```tsx
import Example from "../../components/example/example";
<Example />
```

2. Render it from Go, in a Tea View, with `ui.Custom` - see [Custom components](custom-components.md):

```go
ui.Custom("components/example", map[string]any{"label": "hi"})
```

Either way its usePaired() channel reaches the same Go handlers.

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

Handlers run on the websocket read loop - do quick work inline, spawn a goroutine for anything slow. This channel is one-way, fire-and-forget. When the tsx needs an ANSWER back, or when the functionality is app-wide (auth, settings) or the state lives in Go, reach for [Calls, services and shared state](calls-and-state.md).

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

## Layouts and navigation

Shared chrome that wraps pages - navbars, sidebars, status bars - lives in the layouts/ directory and follows the same folder convention. A page picks its layout with `export const layout = "..."`, and `Link` / `useRoute` / `isActive` handle active-aware navigation. The full story is on the [Layouts](layouts.md) page.

## Two styles, mixed freely

- **Paired handlers (plain style)**: the UI is React through and through, Go is the backend. Familiar if you come from web dev; state lives in the browser.
- **Tea Model**: the UI logic and state live in Go, React renders it. One language for the whole feature, state survives frontend reloads, and everything is trivially testable Go. See [The Tea model](tea.md).

Mix them per page, or even both on one page.

## Notes

- **Nothing crashes on a bad key.** An unregistered handler logs "ui: no handler for pages/index.foo (check the Key ...)" on the Go side; an unresolved Custom() name renders a visible `[unknown component: x]` placeholder rather than failing silently.
- **Dynamic segments.** A `[id]` folder matches one path segment and a `[...slug]` folder matches the rest, so one page serves many URLs. Read the captured value with `useParams()` in the tsx and `ui.ParamsMsg` / `App.Param` in Go. See [Dynamic routes & pagination](../advanced/pagination.md).
