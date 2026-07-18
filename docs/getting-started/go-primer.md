# A Go primer

Just enough Go to build Gantry apps, aimed at someone new to the language. This is background, not a tutorial - skim it, then learn the rest by reading the code `gantry new` generates. If you have written Go before, skip to [Your first app](first-app.md).

## What Go is

Go is a compiled language: you write `.go` files, run `go build`, and get one .exe with no runtime to install. It is strongly typed (every value has a type the compiler checks) and reads top to bottom without much ceremony.

## Packages and imports

Every `.go` file starts by naming its package. All files in one folder belong to the same package, and the folder is the package:

```go
package index
```

To use code from another package you import it by its path, then call it with the package name as a prefix:

```go
import "github.com/B-Commissions/Gantry/ui"

var Page = ui.Page{Key: "pages/index"}
```

Names that start with an Uppercase letter are exported (visible to other packages); lowercase names are private to the package. This is why your pages export "Page" with a capital P - main.go needs to see it.

## Variables and types

```go
var count int        // declare with a type; starts at 0
count = count + 1    // assign
name := "gantry"     // := declares AND assigns, type inferred (string)
```

Basic types you will meet: `int`, `float64`, `string`, `bool`. The "zero value" matters in Go: an `int` starts at `0`, a `string` at `""`, a `bool` at `false`, and Gantry leans on this - options structs treat the zero value of every field as the default.

## Structs

A struct groups fields into one value. This is how all Gantry options work:

```go
type model struct {
    count int
    input string
}

m := model{count: 3}   // literal; input gets its zero value ""
m.count = 4            // field access
```

## Functions and methods

```go
func add(a, b int) int {
    return a + b
}
```

A method is a function attached to a type. The extra part before the name is the receiver - the value the method runs on:

```go
func (m model) View() ui.Node {
    return ui.Textf("count: %d", m.count)
}
```

Receivers in Gantry pages are by value (a copy), which is why Update returns the new model: you change the copy and hand it back.

## Errors

Go functions return errors as ordinary values instead of throwing:

```go
ln, err := appshell.Listen(8330)
if err != nil {
    return err   // pass it up
}
```

The "if err != nil" pattern is everywhere. It looks repetitive; it is also why you always know exactly which call can fail.

## Slices and maps

```go
todos := []string{"a", "b"}      // slice: a growable list
todos = append(todos, "c")
first := todos[0]

byName := map[string]int{}       // map: key -> value
byName["gantry"] = 1
```

## switch on message types

Gantry's Tea pages receive messages as the interface type `ui.Msg` (which can hold any value) and switch on the concrete type:

```go
func (m model) Update(msg ui.Msg) (ui.Model, ui.Cmd) {
    switch v := msg.(type) {
    case incMsg:
        m.count++
    case inputMsg:
        m.input = string(v)
    }
    return m, nil
}
```

`switch v := msg.(type)` means: look at what is actually inside msg and give it to me as v with that type.

## Goroutines (you mostly will not need them)

`go someFunc()` runs a function concurrently. Gantry manages the concurrency you need (the **update loop**, **commands**, the **server**), so in app code you rarely write `go` yourself - return a `ui.Cmd` instead and the framework runs it on its own goroutine.

## go.mod

`go.mod` declares your module's name and dependencies. `gantry new` writes it; `go build` reads it. You edit it only when adding a new Go dependency, and even then `go get example.com/pkg` edits it for you.

That is genuinely most of what Gantry app code uses. Next: the [TSX primer](tsx-primer.md).
