# Custom components

The Tea built-ins cover forms and layout; everything else - charts, canvases, rich views - is a real React component that Go composes with ui.Custom.

## Rendering your React from Go

```go
ui.Custom("components/gauge", map[string]any{
    "value": 0.72,
    "label": "Disk",
})
```

The name resolves against the component registry:

1. Every paired components/ folder registers automatically under its key ("components/gauge" -> the default export of gauge.tsx).
2. createApp can add more by hand - components that are not paired with any Go file:

```tsx
createApp({
  title: "Myapp",
  components: { FancyChart: FancyChart },
});
// Go side: ui.Custom("FancyChart", props)
```

An unresolved name renders a visible [unknown component: x] box rather than failing silently.

## What a Tea-rendered component receives

Components rendered via Custom get TeaComponentProps:

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

- node.props - whatever the Go side put in the map. Values arrive as JSON, so numbers are number, nested maps are objects.
- emit(event, payload) - fires a handler the Go side attached:

```go
ui.Custom("components/gauge", props).
    OnEvent("click", func(p json.RawMessage) ui.Msg {
        var v float64
        _ = json.Unmarshal(p, &v)
        return gaugeClicked(v)
    })
```

- children - Go-side child nodes, already rendered:

```go
ui.Custom("components/panel", nil,
    ui.Text("inside the panel"),
)
```

## Dual-use components

A paired component can serve both worlds: rendered from Go via Custom AND imported directly into other tsx. Make props optional and fall back to usePaired for its Go channel:

```tsx
export default function Gauge(props: Partial<TeaComponentProps>) {
  const { send } = usePaired();
  const value = Number(props.node?.props?.value ?? 0);
  // emit through the Tea handler when present, else the paired channel
  const report = props.emit ?? ((e: string, p?: unknown) => send(e, p));
  // ...
}
```

For most components you will know which way they are used and can skip the gymnastics.

## When to reach for Custom

- Anything visual the built-ins cannot express (canvas, SVG, media).
- Anything with heavy local interactivity (drag and drop, hover states, animations) - let React own the micro-interactions and emit only the decisions to Go.
- Wrapping an npm component library: install it with gantry add, wrap it in a paired component, compose it from Go.

The built-ins and Custom nodes nest freely - a Column of Text next to a Custom chart is the normal shape of a real page.
