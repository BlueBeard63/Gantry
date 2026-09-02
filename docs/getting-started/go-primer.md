# A Go primer

Just enough Go to build Gantry apps, aimed at someone new to the language. This is background, not a full tutorial - skim it, then learn the rest by reading the code `gantry new` generates. Every snippet here is real Gantry app code, not generic Go. If you have written Go before, skip to [Your first app](first-app.md).

## What Go is

Go is a compiled language: you write `.go` files, the compiler checks every type, and `go build` produces one .exe with no separate runtime to install. Gantry apps are ordinary Go modules, so the whole toolchain applies - `go build`, `go run`, `go mod tidy` - and the gantry CLI mostly just drives those commands for you. You will rarely run them by hand.

## Packages and imports

Every `.go` file starts by declaring its package. All files in one folder share a package, and each pair folder is its own package - that is why `pages/index/index.go` begins `package index`:

```go
package index
```

To use code from another package, import it by its module path and call through the package name:

```go
import "github.com/BlueBeard63/Gantry/ui"

var Page = ui.Page{Key: "pages/index"}
```

Names that start with an **Uppercase** letter are exported (visible to other packages); lowercase names are private to the package. This rule is load-bearing in Gantry: your page must export a variable named `Page` (or a component `Component`) with a capital letter, because the generated registry in another package needs to see it. `model`, `incMsg`, and helper functions stay lowercase and private.

## Variables, types, and zero values

```go
var count int        // declared with a type; starts at its zero value, 0
count = count + 1    // assignment
name := "gantry"     // := declares AND assigns, type inferred as string
```

The basic types you meet are `int`, `float64`, `string`, and `bool`. The **zero value** matters a lot in Go and Gantry leans on it everywhere: an `int` starts at `0`, a `string` at `""`, a `bool` at `false`, a pointer or function at `nil`. Options structs treat the zero value of every field as "the default", which is why you only set the fields you care about.

## Structs

A struct groups named fields into one value. Gantry's configuration and your page state are both structs:

```go
// The page's private state - just an int here.
type model struct {
    count int
}

m := model{count: 3}   // struct literal; unset fields take their zero value
m.count++              // field access and increment
```

You write a struct literal with named fields (`ui.Page{Key: "pages/index"}`); fields you omit keep their zero value, so a Page with no `Route` gets its default route.

## Functions and methods

A plain function:

```go
func add(a, b int) int {
    return a + b
}
```

A **method** is a function attached to a type. The part in parentheses before the name is the **receiver** - the value the method runs on. A Tea page's `Update` and `View` are methods on your `model`:

```go
func (m model) View() ui.Node {
    return ui.Textf("count is %d", m.count)
}
```

These receivers are **by value** - `m` is a copy. That is exactly why `Update` returns a new model instead of mutating in place: you change your copy and hand it back, and the framework keeps what you return. (See [The Tea model](../ui/tea-model.md).)

## Interfaces: the Model

An interface is a set of methods a type must have. Gantry's `ui.Model` is the one you will implement most:

```go
type Model interface {
    Init() Cmd
    Update(msg Msg) (Model, Cmd)
    View() Node
}
```

Any struct with those three methods **is** a `ui.Model` - there is no `implements` keyword, the compiler just checks the methods line up. The generated tea page provides all three on its `model` struct, and that is what makes it a page whose UI logic lives in Go.

## Messages, Cmd, and the type switch

A Tea page reacts to messages. `ui.Msg` is an alias for `any` (it can hold a value of any type), and you tell them apart with a **type switch** - a `switch` that branches on the concrete type inside the value. This is the exact shape the scaffold generates:

```go
type incMsg struct{}   // an empty struct used purely as a signal

func (m model) Update(msg ui.Msg) (ui.Model, ui.Cmd) {
    switch msg.(type) {
    case incMsg:
        m.count++
    }
    return m, nil
}
```

`switch msg.(type)` means "look at what is actually inside msg". Bind it with `switch v := msg.(type)` when a case carries data you need (`v` then has that case's type). The second return value is a `ui.Cmd` - `func() Msg`, a piece of work the framework runs on its own goroutine and feeds back into `Update`. Return `nil` for "nothing to do", which is the common case.

## Building UI in Go: ui.Node

A Tea page's `View()` returns a `ui.Node` - a tree the frontend renders as real React. You build it with plain function calls, no markup:

```go
func (m model) View() ui.Node {
    return ui.Column(
        ui.Button(fmt.Sprintf("count is %d", m.count), incMsg{}),
    ).WithProps("class", "counter")
}
```

`ui.Button(label, onClick)` takes the message to send when clicked - here `incMsg{}`, which lands back in `Update`. Other builders include `ui.Row`, `ui.Text`/`ui.Textf`, `ui.Heading`, `ui.Input`, `ui.Checkbox`, and `ui.Custom` (to drop a React component into the tree). `.WithProps("class", "counter")` attaches a CSS class the paired `index.css` can style. The full catalog is in [The node tree](../ui/tea-nodes.md).

## Errors are values

Go functions return errors as an ordinary last return value instead of throwing. You check them right where they happen:

```go
ln, err := appshell.Listen(cfg.Port)
if err != nil {
    return err   // pass it up the chain
}
```

The `if err != nil` pattern is everywhere in Go. It looks repetitive, and it is also why you always know exactly which call can fail. In most Gantry app code the framework handles the fallible parts (starting the server, opening the window) for you, so you mainly meet this pattern in your own handlers.

## JSON payloads from the frontend

A **plain** (non-Tea) page or component handles named events from its `.tsx`. The payload arrives as `json.RawMessage` - raw bytes you decode into the type you expect:

```go
var Page = ui.Page{
    Key: "pages/index",
    On: ui.Handlers{
        "buttonPress": func(p json.RawMessage) {
            var n int
            _ = json.Unmarshal(p, &n)   // decode the bytes into n
            log.Printf("button pressed %d times", n)
        },
    },
}
```

`json.Unmarshal(bytes, &target)` fills `target` (passed by pointer with `&`) from the JSON. This is the entire Go side of a plain page - a map of event names to functions. See [Pairs](../ui/pairs.md).

## Slices and maps

```go
todos := []string{"a", "b"}   // slice: a growable list
todos = append(todos, "c")    // append returns the (possibly new) slice
first := todos[0]

byName := map[string]int{}     // map: key -> value
byName["gantry"] = 1
```

`ui.Handlers` above is just a `map[string]func(...)`, and a page's `View` often builds a `ui.Column` from a slice of items.

## Goroutines (you mostly will not write them)

`go someFunc()` runs a function concurrently on a new goroutine. Gantry already manages the concurrency you need - the update loop, commands, and the server each run on their own goroutines - so in app code you almost never write `go` yourself. When a page needs background work (a timer, a fetch), return a `ui.Cmd` from `Update` and let the framework run it.

## go.mod and dependencies

`go.mod` declares your module's name and its dependencies; `gantry new` writes it and `go build` reads it. You edit it only to add a new Go library, and even then `go get example.com/pkg` (or `go mod tidy`, which the CLI runs after scaffolding) edits it for you. To add an npm package instead, use `gantry add <pkg>`.

That is genuinely most of what Gantry app code uses. Next: the [TSX primer](tsx-primer.md), then [Your first app](first-app.md).
