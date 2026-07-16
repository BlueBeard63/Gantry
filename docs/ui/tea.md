# The Tea model

Pages can keep their whole UI - state, logic, and structure - in Go,
with React as the renderer. The shape comes from Elm via Bubble Tea:
a Model with three methods.

## The shape

```go
type Model interface {
    Init() Cmd                     // first command to run (or nil)
    Update(msg Msg) (Model, Cmd)   // fold an event into new state
    View() Node                    // render the state as a tree
}
```

One loop drives it: something happens (a click, a timer, your server) ->
it becomes a Msg -> Update returns the new model -> View renders it ->
the tree travels to React. You never mutate state outside Update, and
Update runs on a single goroutine, so there are no data races to think
about - ever.

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

The tsx half is one line of hosting:

```tsx
import { TeaView } from "gantry-web/tea";
export default function Stats() { return <TeaView />; }
```

(And it does not have to be just one line - wrap TeaView in any React
layout you like.)

## Messages

A Msg is any Go value; Update switches on the concrete type. Define one
type per thing-that-can-happen. Empty structs for plain events, carrier
types for events with data:

```go
type saveClicked struct{}
type nameTyped string
type rowPicked int
```

## Commands

A Cmd is work that runs off the update loop and feeds its result back
in as a Msg:

- Return nil for "nothing to do" (the common case).
- ui.Tick(d, fn) - wait d, then fn(now) becomes a Msg. Re-issue it
  from Update for a repeating timer (see the example above).
- ui.Batch(cmds...) - run several at once.
- Any func() Msg of your own - fetch something, read a file, compute:

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

Commands run on their own goroutines; their Msg lands back in Update on
the loop. This is the pattern instead of hand-rolled goroutines and
mutexes.

## External events

Anything outside the page (your server, a watcher, another page) sends
messages through the app:

```go
app.Send(vaultLocked{})   // lands in every running page Model's Update
```

Models ignore message types they do not switch on, so broadcast is
safe.

## Building Views

The builders in the ui package: Column, Row, Text, Textf, Heading,
Button, Input, Checkbox, Select, Divider, Spacer, Progress, and
Custom for your own React components (see
[Custom components](custom-components.md)).

Three modifiers:

- .WithKey("todo-3") - identity for list items, so React keeps them
  straight when the list reorders.
- .WithProps("gap", 8, "pad", 16, "grow", true, "class", "hero") -
  layout hints and a css class.
- .OnEvent("hover", fn) - raw event handler, the escape hatch behind
  Button and friends.

Inputs are semi-controlled on the React side: keystrokes echo locally
immediately and stream to Update, and your model's value only forces
the field when it genuinely differs (a validation rewrite, a reset) -
typing never fights the round trip.

## How rendering works (and why you can ignore it)

Every Update sends the whole View tree to React, which reconciles it
like any other render. Trees are small (this is a desktop app, not a
million-row table), renders coalesce under load, and event handlers
from the immediately previous render still resolve during the swap. If
a page ever outgrows this, restructure it into components - do not
diff by hand.

Details of the messages on the wire: [The protocol](../advanced/protocol.md).
