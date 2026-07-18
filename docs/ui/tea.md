# The Tea model

A page can keep its whole UI - state, logic, and structure - in Go, with React as the renderer. The shape comes from Elm by way of Bubble Tea: a `Model` with three methods, driven by one serialized loop. This page covers that loop, every builder that makes a `View` tree, and every way work re-enters the loop.

## The Model interface

```go
// ui/program.go
type Model interface {
    Init() Cmd                     // first command to run when the page activates (or nil)
    Update(msg Msg) (Model, Cmd)   // fold one event into a new model, optionally scheduling more work
    View() Node                    // render the current state as a tree
}
```

One loop drives it: something happens (a click, a timer, your server) -> it becomes a `Msg` -> `Update` folds it into a new `Model` -> `View` renders that model -> the tree travels to React over the websocket. You never mutate state outside `Update`, and `Update` runs on a single goroutine per page, so there are no data races to reason about. `Msg` is `any` and `Cmd` is `func() Msg` - both plain Go types, so you define your own.

## Registering the page

The `.go` half of a `pages/<name>/` folder exports a `ui.Page`. Setting its `Model` field turns the page into a Tea page; `Model` is a factory (`func() Model`) called **once, lazily, when the page first becomes active**, so per-page setup happens on activation, not at startup:

```go
// ui/app.go
type Page struct {
    Key   string           // "pages/stats" - must match the paired .tsx folder
    Route string           // optional; empty derives it from Key ("pages/index" -> "/")
    Model func() Model      // set this for a Tea page
    On    Handlers         // paired events from the tsx (usePaired().send)
    Call  Calls            // paired calls the tsx awaits (usePaired().call)
}
```

## A complete page

```go
package stats

import (
    "time"

    "github.com/B-Commissions/Gantry/ui"
)

var Page = ui.Page{
    Key:   "pages/stats",
    Model: func() ui.Model { return model{} },
}

type model struct {
    seconds int
    running bool
}

type startStop struct{}
type tick time.Time

func tickCmd() ui.Cmd {
    return ui.Tick(time.Second, func(t time.Time) ui.Msg { return tick(t) })
}

func (m model) Init() ui.Cmd { return nil }

func (m model) Update(msg ui.Msg) (ui.Model, ui.Cmd) {
    switch msg.(type) {
    case startStop:
        m.running = !m.running
        if m.running {
            return m, tickCmd()
        }
    case tick:
        if m.running {
            m.seconds++
            return m, tickCmd() // re-arm the timer
        }
    }
    return m, nil
}

func (m model) View() ui.Node {
    label := "Start"
    if m.running {
        label = "Stop"
    }
    return ui.Column(
        ui.Heading("Session"),
        ui.Textf("%02d:%02d", m.seconds/60, m.seconds%60),
        ui.Button(label, startStop{}),
    ).WithProps("pad", 16, "gap", 12)
}
```

The `.tsx` half is one line of hosting - drop a `TeaView` where the tree goes. It does not have to stay one line; wrap `TeaView` in any React layout you like, since it only renders the current page's Go-built tree:

```tsx
import { TeaView } from "gantry-web/tea";
export default function Stats() { return <TeaView />; }
```

`TeaView` subscribes to render frames for the active page and renders `null` until the first tree arrives - so pages without a `Model` simply never mount one.

## Messages

A `Msg` is any Go value, and `Update` switches on its concrete type. Define one type per thing-that-can-happen - empty structs for plain events, single-field or named types to carry data:

```go
type saveClicked struct{}       // a plain event
type nameTyped string           // an event carrying the new value
type rowPicked int              // an event carrying an index
type loaded struct{ data []byte } // an event carrying a result
```

Models ignore message types they do not switch on, which is what makes external broadcast (below) safe - an unrelated `Msg` falls through to the trailing `return m, nil`.

## Commands

A `Cmd` is `func() Msg`: work that runs off the update loop, on its own goroutine, and feeds its result back in as a `Msg`. This is the pattern that replaces hand-rolled goroutines and mutexes - the loop stays single-threaded and every result re-enters through `Update`.

- Return `nil` for "nothing to do" - the common case, and the second return value of most `Update` branches.
- `ui.Tick(d, fn)` waits `d`, then turns `fn(now)` into a `Msg`. Re-issue it from `Update` for a repeating timer (the example above re-arms on every `tick`).
- `ui.Batch(cmds...)` runs several commands concurrently; `nil` entries are dropped, and an all-`nil` batch collapses to `nil`.
- Any `func() ui.Msg` of your own - fetch something, read a file, run a query:

```go
func loadCmd(path string) ui.Cmd {
    return func() ui.Msg {
        data, err := os.ReadFile(path)
        if err != nil {
            return loadFailed{err}
        }
        return loaded{data}
    }
}
```

A `Cmd` that returns a `nil` `Msg` schedules nothing further - use that when the work is a pure side effect. A panic inside a `Cmd` is recovered and routed to the error pipeline (code `panic.cmd`) rather than crashing the process.

## Init

`Init() Cmd` returns the first command, run once when the page's program starts (right after the `Model` factory is called). Return `nil` when there is nothing to kick off, or a load/subscribe command to prime the page:

```go
func (m model) Init() ui.Cmd { return loadCmd("data/config.json") }
```

There is no unconditional first render from `Init`: the very first `View` is delivered when the page activates and a client attaches, so `Init` and activation never race to paint two initial trees.

## External events

Anything outside the page - your server, a file watcher, another page - reaches every running page `Model` through the app. `App.Send` fans a `Msg` out to every active program's `Update`:

```go
app.Send(vaultLocked{})   // lands in every running page Model's Update
```

Because models ignore types they do not switch on, a broadcast is safe: only the pages that care react. For **dynamic routes**, the runtime also sends the page's `Model` a `ui.ParamsMsg{Params: ...}` on activation and whenever the concrete param changes for the same page key (`/item/1` -> `/item/2`); switch on it in `Update` to load the right data. See [Dynamic routes](dynamic-routes.md).

## Building Views

`View()` returns a `Node`. The builders in the `ui` package (`ui/node.go`) each produce one:

| Builder | Renders | Notes |
| --- | --- | --- |
| `Column(children...)` | vertical flex stack | `.gantry-tea-column` |
| `Row(children...)` | horizontal flex row | `.gantry-tea-row` |
| `Text(s)` | a text span | |
| `Textf(format, args...)` | formatted text | wraps `fmt.Sprintf` |
| `Heading(s)` | larger emphasized text | renders an `<h2>` |
| `Button(label, onClick)` | a button | `onClick` is a `Msg` sent on click |
| `Input(value, onChange)` | one-line text field | `onChange func(string) Msg`, per keystroke |
| `Checkbox(label, checked, onToggle)` | a checkbox | `onToggle func(bool) Msg` |
| `Select(value, options, onChange)` | a dropdown | `options []string`, `onChange func(string) Msg` |
| `Divider()` | a horizontal rule | |
| `Spacer()` | flexes to fill leftover space | `flex: 1` inside a Row/Column |
| `Progress(v)` | a bar filled to `v` (0..1) | clamped to the range on render |
| `Custom(name, props, children...)` | your own React component | see [Custom components](custom-components.md) |

Three modifiers chain onto any `Node` (each returns a copy - they never mutate the receiver, so they are safe to compose):

- `.WithKey("todo-3")` sets the reconciliation key - put it on list items so React keeps their identity when the list reorders or filters.
- `.WithProps("gap", 8, "pad", 16, "grow", true, "class", "hero")` adds alternating name/value pairs. The built-ins understand `gap` (px), `pad` (px), `grow` (bool -> `flexGrow: 1`), and `class` (a CSS class name); any other key is just passed through as a prop, which is how you feed a `Custom` component. See [styling Tea built-ins](styling.md).
- `.OnEvent("change", fn)` attaches a raw handler - the escape hatch behind `Button`, `Input`, and friends. `fn` is `func(payload json.RawMessage) Msg`; the payload is whatever the component emitted.

## Inputs are semi-controlled

Keystrokes echo into the field locally at once and stream to `Update`; your model's value only *forces* the field when it genuinely differs from what the field last sent - a validation rewrite, a reset, a value that changed for another reason. So typing never fights the round trip and never drops characters. (A plain `<input>` fully controlled over a websocket would jumble fast typing; the built-in `Input` avoids that.) You can add a `"placeholder"` via `WithProps("placeholder", "search...")` - the runtime reads it off the node's props.

## How rendering works (and why you can ignore it)

Every message that changes the model triggers a `View`, and the whole tree is serialized and sent to React, which reconciles it like any other render. The loop **coalesces**: a burst of rapid messages is drained before rendering, so ten quick clicks produce one render, not ten. Handler IDs are regenerated every render, but the *previous* generation is kept too, so an event that races an in-flight re-render still resolves instead of being dropped. Trees are small (this is a desktop app, not a million-row grid), so full-tree sends are cheap; if a page ever outgrows this, split it into [components](custom-components.md) rather than diffing by hand.

Panics are contained per stage and never kill the page loop: an `Update` panic (`panic.update`) leaves the model at its last good state, a `View` panic (`panic.view`) skips the frame, a `Cmd` panic (`panic.cmd`) is reported - all three flow into the [error pipeline](../advanced/errors.md).

## Notes

- **On the wire.** The exact render/event message shapes: [The protocol](../advanced/protocol.md).
- **Deep internals** (the program loop, handler generations, delivery to observers): [Architecture](../advanced/architecture.md).
