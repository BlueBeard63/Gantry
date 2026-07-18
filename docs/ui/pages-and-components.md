# Pages and components

The pairing system in full: how a .tsx file and the .go file next to it talk, how routing works, and how data flows both ways.

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

If a key is wrong or unregistered, nothing crashes: send() logs "ui: no handler for pages/index.foo" on the Go side, and Custom() renders a visible [unknown component: x] placeholder.

## Pages

A page is routable. The Go half:

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

- Route derivation: pages/index -> "/", pages/anything -> "/anything", and pairs nest to any depth: pages/account/settings -> "/account/settings". An "index" leaf maps to its parent (pages/account/index -> "/account"), and a folder can both BE a page and CONTAIN pages:

```
pages/
  account/
    settings/
      settings.tsx, settings.go, settings.css   -> /account/settings
      profile/
        profile.tsx, profile.go                 -> /account/settings/profile
```

The key is always the folder path ("pages/account/settings/profile"), the css scope class follows it (gantry-pages-account-settings-profile), and registration stays automatic - gantry gen walks the whole tree. Components and layouts nest the same way (a nested layout's name is its path, e.g. layout = "admin/main").
- Model gives the page a Go state machine rendered with <TeaView /> - see [The Tea model](tea.md). A page can have a Model, handlers, both, or neither (a purely static page needs no .go at all... though the file must exist if main.go registers it).
- The tsx half's default export is the page component. Two optional named exports tune it: `export const chrome = false` hides the TitleBar on this page (widgets, popups), `export const route = "/x"` overrides the route (keep it matching the Go side if both are set).

Routing is plain pathname switching, no dependency: navigate("/settings") pushes history and re-renders; back/forward work. With a single page (only pages/index) the router disappears entirely.

- Dynamic segments: a folder named `[id]` matches one path segment and a `[...slug]` folder matches the rest, so one page serves many URLs (pages/examples/page1/[id] -> /examples/page1/1, /2, ...). Read the captured value with `useParams()` in the tsx and `ui.ParamsMsg` / `App.Param` in the Go half. See [Dynamic routes & pagination](../advanced/pagination.md).

## Layouts and navigation

Shared chrome that should wrap pages - navbars, sidebars, status bars - lives in the layouts/ directory, following the same folder convention as everything else: layouts/main/main.tsx (+ optional main.css, auto-imported; even an optional main.go, paired under the key "layouts/main"):

```tsx
// layouts/main/main.tsx
import { Link } from "gantry-web";
import type { ReactNode } from "react";

export default function Main({ children }: { children?: ReactNode }) {
  return (
    <div className="layout-main">
      <nav className="app-nav">
        <Link to="/">Home</Link>
        <Link to="/settings">Settings</Link>
      </nav>
      <main className="app-main">{children}</main>
    </div>
  );
}
```

Layouts are addressed by their short name ("main") and pages choose theirs with an optional export:

```tsx
export const layout = "compact";           // this page uses layouts/compact
export const layout = ["main", "compact"]; // nested: <Main><Compact><Page/>
export const layout = false;               // no layout at all
```

Defaults: pages use "main" when it exists; chromeless pages (export const chrome = false - widgets, popups) skip layouts unless they opt in (a name, or true for "main"). An unknown layout name logs a console warning and is skipped, so a typo cannot blank a page.

The full story - nesting rules, Link/useRoute/isActive, styling, and layouts with a Go half - is on the [Layouts](layouts.md) page.

Link is navigate() in anchor form, and it knows when it is the current page:

- data-active="true"/"false" on the rendered <a> - style with `.app-nav a[data-active="true"] { ... }` in css, or `data-[active=true]:bg-...` variants if you use Tailwind
- aria-current="page" while active
- activeClassName - an extra class applied only while active
- matchPrefix - also count child routes as active ("/docs" stays lit on "/docs/intro")

```tsx
<Link to="/settings" activeClassName="lit" matchPrefix>Settings</Link>
```

For fully custom nav elements, useRoute() returns the current pathname (re-rendering on navigation) and isActive(path, to, exact) does the matching:

```tsx
const path = useRoute();
<button data-active={isActive(path, "/stats")} onClick={() => navigate("/stats")}>
```

## Components

A component is a reusable pair. Two ways to use one:

1. Import it like any React component - normal composition, typed props, the usual:

```tsx
import Example from "../../components/example/example";
<Example />
```

2. Render it from Go, in a Tea View:

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

Handlers run on the websocket read loop - do quick work inline, spawn a goroutine for anything slow. When the tsx needs an ANSWER back (not just fire-and-forget), use Calls and usePaired().call - and for app-wide functionality (auth, settings) and state that lives in Go, see [Calls, services and shared state](calls-and-state.md).

## Data flow: Go -> tsx

app.Push sends a named event to a pair's tsx side:

```go
app.Push("components/example", "state", currentStats)
```

The tsx receives it two ways:

- state: pushes named "state" land in usePaired().state automatically - the one-liner for mirroring Go data into a component.
- on: subscribe to any event name:

```tsx
useEffect(() => on("progress", (p) => setProgress(p as number)), [on]);
```

Push goes to the connected window. There is exactly one connected client at a time (the app window; a reload or dev restart replaces it), so Push is fire-and-forget - a push with no client is silently dropped.

## Registration is automatic

Adding a new pair is two steps:

1. Make the folder and files: pages/stats/stats.go + stats.tsx (+ stats.css if you want styles).
2. In stats.go: package stats, export `var Page = ui.Page{...}` (or `var Component = ui.Component{...}`).

That is it. The frontend discovers the tsx by its location (the Vite plugin), and the Go side is generated: gantry dev/build scan pages/, components/ and layouts/ for the exported Page/Component vars and regenerate gantry_registry.go, which main.go consumes as `ui.NewApp(gantryPairs()...)`. Run `gantry gen` to regenerate it by hand (e.g. before a plain `go build`). Never edit the generated file.

During gantry dev, adding a folder hot-reloads the frontend; the Go side needs a dev restart (Go compiles).

## When to use which style

- Paired handlers (plain style): the UI is React through and through, Go is the backend. Familiar if you come from web dev; state lives in the browser.
- Tea Model: the UI logic and state live in Go, React renders it. One language for the whole feature, state survives frontend reloads, and everything is trivially testable Go. See [The Tea model](tea.md).

Mix freely - per page, even both on one page.