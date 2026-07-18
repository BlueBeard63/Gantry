# The node tree

`View()` returns a `Node` - one element of a Go-driven UI tree that the gantry-web runtime renders with real React components. This page is the catalog: every builder in `ui/node.go` with its exact signature, the three modifiers that chain onto any node, how the built-in `Input` stays typeable over a websocket, and how the runtime maps a handful of prop names onto CSS. For the loop that calls `View` see [The Tea model](tea-model.md); for the messages a node's handlers produce see [Commands & messages](tea-commands.md).

A `Node` is a plain struct - a `Type` string, an optional reconciliation `Key`, a `Props` map, `Children`, and an unexported handler table - so the builders below are just conveniences that fill it in. Nothing here mutates: a builder returns a fresh `Node`, and every modifier returns a copy.

## The node builders

Each builder produces one node. Signatures are exact, straight from `ui/node.go`:

| Builder | Signature | Renders |
| --- | --- | --- |
| `Column` | `Column(children ...Node) Node` | a vertical flex stack (`.gantry-tea-column`) |
| `Row` | `Row(children ...Node) Node` | a horizontal flex row (`.gantry-tea-row`) |
| `Text` | `Text(s string) Node` | a text span (`.gantry-tea-text`) |
| `Textf` | `Textf(format string, args ...any) Node` | formatted text - wraps `fmt.Sprintf` and calls `Text` |
| `Heading` | `Heading(s string) Node` | larger emphasized text, an `<h2>` (`.gantry-tea-heading`) |
| `Button` | `Button(label string, onClick Msg) Node` | a button; `onClick` is the `Msg` sent on click |
| `Input` | `Input(value string, onChange func(string) Msg) Node` | a one-line text field; `onChange` fires per keystroke |
| `Checkbox` | `Checkbox(label string, checked bool, onToggle func(bool) Msg) Node` | a checkbox; `onToggle` gets the new checked state |
| `Select` | `Select(value string, options []string, onChange func(string) Msg) Node` | a dropdown; `onChange` gets the selected option |
| `Divider` | `Divider() Node` | a horizontal rule, an `<hr>` (`.gantry-tea-divider`) |
| `Spacer` | `Spacer() Node` | a div that flexes to fill leftover space in a Row/Column |
| `Progress` | `Progress(v float64) Node` | a bar filled to `v` (0..1), clamped on render |
| `Custom` | `Custom(component string, props map[string]any, children ...Node) Node` | any React component the app registered |

A few details the table cannot carry:

- `Button`, `Input`, `Checkbox`, and `Select` are not special node types under the hood - each builds its node and attaches a handler with `OnEvent`. `Button` wraps `onClick` so a `click` event returns it verbatim; `Input`/`Select` unmarshal a JSON string and call your `onChange`; `Checkbox` unmarshals a JSON bool and calls your `onToggle`. That is why passing an `onChange func(string) Msg` returning `nil` for some inputs is fine - a `nil` `Msg` schedules nothing.
- `Custom(component, props, children...)` takes the registered component name as its `Type` (a paired folder's key, e.g. `Custom("components/gauge", ...)`, or a name registered through `createApp`). Children render into the component's `children` prop, and an unresolved name renders as `.gantry-tea-unknown`. See [Custom components](custom-components.md).
- `Progress` accepts any `float64`; the runtime clamps it to `0..1` at render time (`Math.min(1, Math.max(0, v))`), so an out-of-range value never overflows the bar.

```go
func (m model) View() ui.Node {
    return ui.Column(
        ui.Heading("Downloads"),
        ui.Input(m.query, func(s string) ui.Msg { return queryChanged(s) }),
        ui.Row(
            ui.Text("Progress"),
            ui.Spacer(),
            ui.Progress(m.done),
        ).WithProps("gap", 8),
        ui.Checkbox("Auto-retry", m.retry, func(b bool) ui.Msg { return retryToggled(b) }),
        ui.Divider(),
        ui.Button("Start", startClicked{}),
    ).WithProps("pad", 16, "gap", 12)
}
```

## The modifiers

Three methods chain onto any `Node`. Each returns a **copy** - they never mutate the receiver, so they are safe to compose and reuse:

- `WithKey(k string) Node` - sets the reconciliation key. Put it on list items so React keeps their identity when the list reorders, filters, or grows: `ui.Text(todo.title).WithKey(todo.id)`.
- `WithProps(kv ...any) Node` - adds alternating name/value pairs to the node's props. It copies the existing props first, then writes each pair whose key is a string, so later calls override earlier ones. The built-ins understand four style hints (below); any other key is passed straight through, which is how you feed props to a `Custom` component or set a `placeholder`.
- `OnEvent(event string, fn func(payload json.RawMessage) Msg) Node` - attaches a raw event handler, the escape hatch that `Button`, `Input`, `Checkbox`, and `Select` are built on. `fn` receives whatever JSON the component emitted and returns a `Msg` (or `nil`). Reach for it when a `Custom` component fires an event no built-in covers.

```go
ui.Custom("components/chart", map[string]any{"series": data}).
    OnEvent("pointSelected", func(p json.RawMessage) ui.Msg {
        var idx int
        _ = json.Unmarshal(p, &idx)
        return pointPicked(idx)
    })
```

## Semi-controlled inputs

A plain `<input>` fully controlled over a websocket would jumble fast typing - each keystroke would have to round-trip to Go and back before the character appeared. The built-in `Input` avoids that by being **semi-controlled**: keystrokes echo into the field locally at once *and* stream to `Update`, while the Go value only *forces* the field when it genuinely differs from what the field last sent. So a validation rewrite, a reset, or a value that changed for another reason overrides the field, but your own echo arriving late does not - typing never fights the round trip and never drops characters.

The mechanism (`TeaInput` in `Runtime.tsx`) keeps local state plus a `lastSent` ref: on change it sets local state, records `lastSent`, and emits; when a new server `value` arrives it re-syncs the field only if that value differs from `lastSent` (a real remote change), otherwise it just clears `lastSent`. You can add placeholder text with `WithProps("placeholder", "search...")` - the runtime reads `props.placeholder` (when it is a string) straight onto the `<input>`.

```go
ui.Input(m.name, func(s string) ui.Msg { return nameTyped(s) }).
    WithProps("placeholder", "Your name")
```

## Style-hint mapping

The runtime translates four prop names into inline styles or a class; everything else on a built-in node is ignored for styling and simply passed as a prop. From `styleHints` in `Runtime.tsx`:

| Prop | Type checked | Effect |
| --- | --- | --- |
| `gap` | `number` | `style.gap = value` (px) |
| `pad` | `number` | `style.padding = value` (px) |
| `grow` | `=== true` | `style.flexGrow = 1` |
| `class` | `string` | appended to the node's base class name |

So `WithProps("gap", 8, "pad", 16, "grow", true, "class", "hero")` lays out a stack with spacing and padding, lets it grow to fill its parent, and tags it with a CSS hook. The value types matter: `gap`/`pad` must be numbers and `grow` must be the boolean `true`, or the hint is skipped. Every built-in also carries a stable base class (`.gantry-tea-column`, `.gantry-tea-button`, and so on; the `progress` bar has an inner `.gantry-tea-progress-fill`) that your CSS can target directly - see [styling Tea built-ins](styling.md).

## Notes

- **The Tea model** - the loop that calls `View`, coalescing, and per-stage panics: [The Tea model](tea-model.md).
- **Commands & messages** - the `Msg` values your handlers return and how their IDs are generated: [Commands & messages](tea-commands.md).
- **Custom components** - registering and composing your own React nodes: [Custom components](custom-components.md).
- **Styling** - the class hooks and the `"class"` prop: [Styling Tea built-ins](styling.md).
