# Components

A component is a reusable [pair](pairs.md): a `.tsx`/`.go` folder that is not a route, meant to be dropped into pages (or other components) instead of navigated to. It shares the whole [pairing model](pairs.md) with pages - keys, `usePaired`, pushes, automatic registration - and differs only in that its Go half is a `ui.Component` and it has no route.

## The Go half: ui.Component

```go
// components/example/example.go
var Component = ui.Component{
    Key: "components/example",
    On: ui.Handlers{ /* events from the component */ },
    Call: ui.Calls{ /* awaited requests from the component */ },
}
```

`ui.Component` is a trimmed `ui.Page`: it has `Key`, `On` and `Call`, but **no `Route`** (a component is never navigated to) and **no `Model`** (a component driven from Go is done through `ui.Custom`, below, not a page-style Model). `On` and `Call` behave exactly as they do on a page - see [Pairs](pairs.md) for `On` and [Awaited Go calls](calls.md) for `Call`.

## Two ways to use one

**1. Import it like any React component** - normal composition, typed props, the usual:

```tsx
import Example from "../../components/example/example";
<Example />
```

Its `usePaired()` channel still reaches the Go handlers keyed to `"components/example"`, because the Vite plugin injected that key from the file's folder. So an imported component keeps its own private line to Go no matter where it is rendered.

**2. Render it from Go**, inside a Tea `View`, with `ui.Custom`:

```go
ui.Custom("components/example", map[string]any{"label": "hi"})
```

This is a different, richer contract - the Go side owns the props and receives the component's events as Tea messages. It has its own page: see [Custom components](custom-components.md) for `ui.Custom`, `TeaComponentProps`, `emit`/`OnEvent`, Go-side children, and dual-use components that work both ways. This page does not repeat it.

Either way, the component's Go handlers, calls, pushes and shared state work exactly as described in [Pairs](pairs.md).

## Registration

Nothing to wire up. Drop a `components/` folder with a `.tsx` and a `.go` exporting `var Component = ui.Component{...}`, and `gantry dev`/`build` pick it up when they regenerate the registry - the same automatic registration every pair gets (see [Pairs](pairs.md)). A tsx-only component (no `.go`, imported and used purely on the React side) needs no registration at all. Components nest to any depth and key by folder path just like pages.
