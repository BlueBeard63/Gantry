# Custom components

The [Tea built-ins](tea.md) cover forms and layout; everything else - charts, canvases, maps, media, an npm widget - is a real React component that Go composes into the tree with `ui.Custom`. Go decides *where* it goes and *what data* it gets; React owns *how* it draws.

## Rendering your React from Go

`Custom` is a `Node` builder like `Column` or `Button`; its first argument is the registered name, the second is a props map (JSON-serializable values), and any trailing nodes become the component's children:

```go
// ui/node.go: func Custom(component string, props map[string]any, children ...Node) Node
ui.Custom("components/gauge", map[string]any{
    "value": 0.72,
    "label": "Disk",
})
```

The name resolves against the component registry, in this order:

1. **Every paired `components/` folder registers automatically under its key.** `components/gauge/gauge.tsx`'s default export is registered as `"components/gauge"`, so `ui.Custom("components/gauge", ...)` renders it. This is the usual path.
2. **`createApp` can register more by hand** via the `components` option - components that are *not* paired with any Go file. The usual home for this is the app-root `app.tsx`:

```tsx
// app.tsx
import FancyChart from "./widgets/FancyChart";
import type { CreateAppOptions } from "gantry-web";

export default {
  title: "Myapp",
  components: { FancyChart },
} satisfies CreateAppOptions;
// Go side: ui.Custom("FancyChart", props)
```

The explicit `components` map is merged *over* the paired registry, so an entry there wins on a name clash. An unresolved name renders a visible `[unknown component: <name>]` box (`.gantry-tea-unknown`) rather than failing silently, so a typo is obvious on screen.

## What a Tea-rendered component receives

Components rendered via `Custom` get `TeaComponentProps` (from `gantry-web/tea`):

```tsx
import type { TeaComponentProps } from "gantry-web/tea";

export default function Gauge({ node, emit, children }: TeaComponentProps) {
  const value = Number(node.props?.value ?? 0);
  const label = String(node.props?.label ?? "");
  return (
    <div className="gauge" onClick={() => emit("click", value)}>
      <svg /* draw the arc from value */ />
      <span>{label}</span>
      {children}
    </div>
  );
}
```

The three props, exactly:

- **`node`** is the wire node. `node.props` is whatever the Go side put in the map - values arrive as decoded JSON, so numbers are `number`, nested maps are objects, arrays are arrays. `node.handlers` is a map of event name -> handler id (you rarely touch it; `emit` uses it). Read props defensively (`?? default`) because a prop the Go side omitted is `undefined`.
- **`emit(event, payload?)`** fires the Go-side handler attached to `node` under that event name. It is a no-op unless the Go side attached one with `.OnEvent(event, ...)`, so `emit` and `OnEvent` are two halves of one contract - the string names must match:

```go
ui.Custom("components/gauge", props).
    OnEvent("click", func(p json.RawMessage) ui.Msg {
        var v float64
        _ = json.Unmarshal(p, &v)  // p is the payload emit() sent
        return gaugeClicked(v)
    })
```

- **`children`** is the already-rendered Go-side child nodes; render them wherever they belong in your markup. Children can be built-ins or more `Custom` nodes - the tree nests freely:

```go
ui.Custom("components/panel", nil,
    ui.Heading("Details"),
    ui.Text("inside the panel"),
)
```

## When to reach for Custom

- Anything visual the built-ins cannot express: canvas, SVG, an image grid, an embedded map, media playback.
- Anything with heavy *local* interactivity - drag and drop, hover states, animations, a text editor. Let React own the micro-interactions and `emit` only the decisions back to Go (the final position, the committed value), keeping the round trip off the hot path.
- Wrapping an npm component library: install it with `gantry add`, wrap it in a paired `components/` folder, and compose it from Go like any other `Custom` node.

Built-ins and `Custom` nodes mix in one tree - a `Column` of `Text` next to a `Custom` chart is the ordinary shape of a real page, and `Custom` is just another entry in the [builder table](tea.md#building-views).

## Advanced: dual-use components

A paired component can serve both worlds at once - rendered from Go via `Custom` *and* imported directly into other `.tsx` where it talks to its Go half through [`usePaired`](pairs.md). Make the Tea props optional and fall back to the paired channel when they are absent:

```tsx
import { usePaired } from "gantry-web";
import type { TeaComponentProps } from "gantry-web/tea";

export default function Gauge(props: Partial<TeaComponentProps>) {
  const { send } = usePaired();
  const value = Number(props.node?.props?.value ?? 0);
  // emit through the Tea handler when Custom-rendered, else the paired channel
  const report = props.emit ?? ((e: string, p?: unknown) => send(e, p));
  // ...draw, and call report(...) on interaction
}
```

Most components are only ever used one way, so you will usually know which path applies and can skip the optionality. See [Pairs](pairs.md) for the direct-import channel and [Styling](styling.md) for scoping a component's CSS.
