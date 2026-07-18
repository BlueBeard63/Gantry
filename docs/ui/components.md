# Components

A component is a reusable [pair](pairs.md): a .tsx/.go folder that is not a route, meant to be dropped into pages (or other components) instead of navigated to. It shares the whole [pairing model](pairs.md) with pages - keys, `usePaired`, pushes, automatic registration - and differs only in that its Go half is a `ui.Component` and it has no route.

```go
// components/example/example.go
var Component = ui.Component{
    Key: "components/example",
    On:  ui.Handlers{ /* events from the component */ },
}
```

## Two ways to use one

**1. Import it like any React component** - normal composition, typed props, the usual:

```tsx
import Example from "../../components/example/example";
<Example />
```

Its `usePaired()` channel still reaches the Go handlers keyed to "components/example", so an imported component keeps its own line to Go.

**2. Render it from Go**, in a Tea View, with `ui.Custom`:

```go
ui.Custom("components/example", map[string]any{"label": "hi"})
```

This is a different, richer contract - the Go side owns the props and receives the component's events as Tea messages. It has its own page: see [Custom components](custom-components.md) for `ui.Custom`, `TeaComponentProps`, `emit`/`OnEvent`, Go-side children, and dual-use components that work both ways. This page does not repeat it.

Either way, the component's Go handlers, pushes and shared state work exactly as described in [Pairs](pairs.md).

## Registration

Nothing to wire up. Drop a components/ folder with a .tsx and a .go exporting `var Component = ui.Component{...}`, and gantry dev/build pick it up when they regenerate the registry - the same automatic registration every pair gets (see [Pairs](pairs.md)). Components nest to any depth and key by folder path just like pages.
