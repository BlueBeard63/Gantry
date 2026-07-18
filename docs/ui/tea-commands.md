# Commands & messages

Everything that re-enters a Tea page's loop is either a `Msg` (an event to fold into the model) or a `Cmd` (work to run off the loop that produces a `Msg`). This page covers both types, the standard command constructors (`Batch`, `Tick`, and the first command from `Init`), the `App.Send` broadcast that lets the outside world reach every page, the `ParamsMsg` a dynamic route delivers, and how the runtime generates and retains the handler IDs that turn a click into a `Msg`. For the loop that consumes all of this see [The Tea model](tea-model.md); for the nodes whose handlers emit these messages see [The node tree](tea-nodes.md).

## Messages

```go
// ui/program.go
type Msg any
```

A `Msg` is any Go value, and `Update` switches on its concrete type. Define one type per thing-that-can-happen - empty structs for plain events, single-field or named types to carry data:

```go
type saveClicked struct{}          // a plain event
type nameTyped string              // an event carrying the new value
type rowPicked int                 // an event carrying an index
type loaded struct{ data []byte }  // an event carrying a result
```

Models ignore message types they do not switch on: an unrecognized `Msg` falls through to the trailing `return m, nil`. That is exactly what makes broadcast (below) safe - a `Msg` a page does not care about changes nothing.

## Commands

```go
// ui/program.go
type Cmd func() Msg
```

A `Cmd` is work that runs off the update loop, on its own goroutine, and feeds its result back in as a `Msg`. This is the pattern that replaces hand-rolled goroutines and mutexes - the loop stays single-threaded and every result re-enters through `Update`, so nothing outside `Update` touches the model.

- Return `nil` for "nothing to do" - the common case, and the second return value of most `Update` branches.
- A `Cmd` that returns a `nil` `Msg` schedules nothing further - use that when the work is a pure side effect (the loop simply drops a `nil` result).
- Any `func() ui.Msg` of your own is a command - fetch something, read a file, run a query:

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

A panic inside a command is recovered on its goroutine and routed to the error pipeline (code `panic.cmd`) rather than crashing the process - no result message comes back, but the loop is untouched. See [the panic codes](tea-model.md#panics-are-contained-per-stage).

## Init

`Init() Cmd` is the model's chance to return a first command, run once when the page's program starts (right after the `Model` factory produces the model). Return `nil` for nothing to kick off, or a load/subscribe command to prime the page:

```go
func (m model) Init() ui.Cmd { return loadCmd("data/config.json") }
```

`Init` schedules a command; it does not paint - the first render is driven by activation, not by `Init`. That timing lives with the loop on [The Tea model](tea-model.md#init-and-the-first-render).

## Batch and Tick

Two constructors in `ui/program.go` build commands from other work:

- `Batch(cmds ...Cmd) Cmd` runs several commands concurrently. It drops `nil` entries, and an all-`nil` (or empty) batch collapses to `nil`, so you can pass optional commands without guarding each one. Internally a batch is carried as a `batchMsg` that the loop fans out - applying a batch schedules its commands and renders nothing on its own.

```go
return m, ui.Batch(loadCmd("a.json"), loadCmd("b.json"))
```

- `Tick(d time.Duration, fn func(time.Time) Msg) Cmd` waits `d`, then turns `fn(now)` into a `Msg`. It fires **once** - re-issue it from `Update` for a repeating timer:

```go
func tickCmd() ui.Cmd {
    return ui.Tick(time.Second, func(t time.Time) ui.Msg { return tick(t) })
}

func (m model) Update(msg ui.Msg) (ui.Model, ui.Cmd) {
    switch msg.(type) {
    case tick:
        m.seconds++
        return m, tickCmd() // re-arm for the next second
    }
    return m, nil
}
```

## App.Send: reaching every page

Anything outside a page - your server, a file watcher, another page's handler - reaches every running page `Model` through the app. `App.Send` fans a `Msg` out to every active program's `Update`:

```go
// ui/app.go: Send feeds msg into every running page Model.
app.Send(vaultLocked{})   // lands in every running page Model's Update
```

`Send` snapshots the current programs under a lock and then calls `send` on each, so it is safe to call from any goroutine (an HTTP handler, a background worker). Because models ignore types they do not switch on, a broadcast is inherently safe: only the pages that actually handle `vaultLocked{}` react, everything else drops it. Note the reach is *every running program* - a page whose model was never built (never activated) is not in the set, and an inactive-but-built page still updates its model even though its render goes nowhere.

## ParamsMsg for dynamic routes

A dynamic page ([`[id]`, `[...slug]`](dynamic-routes.md)) does not read its route params from a global - the runtime **delivers** them as a message. `ui.ParamsMsg{Params: RouteParams}` arrives at the page's `Model` when the page becomes active and again whenever the concrete param changes for the same page key (`/item/1` -> `/item/2`, which reuses the one program rather than rebuilding it). Switch on it in `Update` to load the right data:

```go
func (m model) Update(msg ui.Msg) (ui.Model, ui.Cmd) {
    switch msg := msg.(type) {
    case ui.ParamsMsg:
        m.id = msg.Params.Get("id")   // "7" for /item/7
        return m, loadItem(m.id)
    case loaded:
        m.item = msg.item
        return m, nil
    }
    return m, nil
}
```

`RouteParams` is a `map[string][]string` (each captured segment normalized to a slice): `Params.Get(name)` joins with `/` (a `[id]` gives `"7"`, a `[...slug]` gives `"a/b/c"`), and `Params.List(name)` returns the raw segments - the natural read for a catch-all. The same values are also reachable off the `*App` through `App.Params()` / `App.Param(name)` for a Call/On handler that captured it. See [Dynamic routes](dynamic-routes.md).

## How handlers are generated and retained

A node's handler is a Go closure, but the client only ever sees an **ID**. On every render `serialize` walks the tree and, for each node with handlers, mints fresh IDs (`h1`, `h2`, ... from a per-program counter) into a new handler table, and the wire tree carries `{event: id}` maps instead of functions. When the client echoes an event back, `handleEvent` looks the ID up and feeds the produced `Msg` into `Update` (a `nil` `Msg` is dropped).

The IDs are regenerated every render, so they are only valid for the generation that produced them - but the loop keeps the **previous** generation too. `render` moves the current table to `prev` before building the new one, and `handleEvent` resolves an ID against the current table first, then `prev`. That is why an event that races an in-flight re-render still resolves instead of vanishing: it can be one generation stale and still find its closure. IDs older than one render (two or more generations back) are dropped silently. This retention is invisible to your model - you never see or manage a handler ID - but it is why fast clicking during rapid updates never loses an event.

## Notes

- **The Tea model** - the loop that runs commands and applies messages, coalescing, panic codes: [The Tea model](tea-model.md).
- **The node tree** - the builders whose handlers produce these messages: [The node tree](tea-nodes.md).
- **Dynamic routes** - where `ParamsMsg` comes from: [Dynamic routes](dynamic-routes.md).
- **On the wire** - the exact render/event message shapes: [The protocol](../advanced/protocol.md).
