# Testing: setup

Gantry ships an end-to-end testing system that drives the real app - the real Go process, the real websocket, and the real window - not a simulation of the wire protocol against mocked pages. Tests live in your app repo, are written as plain Go tests with the `gantrytest` driver, and run with one command:

```
gantry test
```

This page covers getting a first test running. The follow-on pages cover [driving the app](driving.md), [the DOM plane](dom.md), [error assertions and artifacts](errors-and-artifacts.md), [widget snapshots](widgets.md), [mobile testing](mobile.md) and [CI](ci.md).

## What a test can see

A test session talks to the app on two planes, and can assert on either or both:

- **The protocol plane** - what Go thinks. The driver dials `/gantry/ws` and speaks the [wire protocol](../advanced/protocol.md) exactly like the frontend does: it mounts pages, fires Tea and paired events, awaits calls, and observes renders, pushes, shared state and error frames. Everything on this plane is headless and cross-platform, and it is enough to test a Tea-style app end to end.
- **The DOM plane** - what the user sees. Element queries, real clicks and typing, screenshots and screencasts, driven over the webview's devtools protocol (CDP). Launch with `WithDOM()` to enable it; Windows-only for now (WebView2 speaks CDP, WebKitGTK does not), and `WithDOM()` tests skip cleanly elsewhere. See [the DOM plane](dom.md).

## Your first test

Create a `tests/` directory at the app root (next to `gantry.json`) and add a Go test file:

```go
// tests/smoke_test.go
package tests

import (
	"strings"
	"testing"

	"github.com/B-Commissions/Gantry/gantrytest"
)

func TestCounter(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)         // builds + starts the app, cleans up after
	app.Ready("pages/index")            // mount a page, like the frontend would

	tree := app.WaitTree("first render", func(n *gantrytest.Node) bool {
		return strings.Contains(n.Text(), "count is 0")
	})
	app.Click(tree.Find("button"))      // fire the button's Tea click handler
	app.WaitTree("count advanced", func(n *gantrytest.Node) bool {
		return strings.Contains(n.Text(), "count is 1")
	})

	app.ExpectNoErrors()                // nothing landed in the error pipeline
}
```

Then run:

```
gantry test
```

`gantry test` prepares the app exactly like a build (regenerates `.gantry/` and the registries, runs one vite build so the embedded frontend is current), builds the app binary once for the whole suite, and runs `go test ./tests/...` with the environment the driver expects.

## What Launch gives you

Every `gantrytest.Launch(t)`:

- starts the app binary with `--port 0`, so each test gets its own ephemeral port and parallel tests never collide with each other or a dev instance;
- runs headless (`--no-open`) unless `--headed` / `WithHeaded()` asks for the window;
- points the app's config dir (geometry.json, crash.log) at a per-test temp directory, so tests are hermetic and crash assertions are per-test;
- defaults the app to `development` mode so error detail is full - override per test with `WithMode("production")`;
- registers cleanup with `t.Cleanup`: the whole process tree is killed (no orphaned webview or helper processes), crash.log is collected, and artifacts are kept when the test failed.

Because tests are standard Go tests, everything from `go test` works: `t.Parallel()`, subtests, `-run` filters, timeouts, IDE integration.

## The gantry test command

```
gantry test [flags] [pattern]
```

| Flag | Meaning |
| --- | --- |
| `pattern` | test name filter, passed to `go test -run` |
| `--headed` | real window instead of headless (and the [DOM plane](dom.md) with it, on Windows) |
| `--record` | record `screencast.avi` for every DOM-plane test (implies keeping those artifacts) |
| `--mode production` | run apps in production mode |
| `--device android[:SERIAL]` | run the suite on a plugged-in phone or emulator instead of the desktop ([mobile](mobile.md)) |
| `--allow-device-data` | allow the hermetic `pm clear` (wipes the app's on-device data) on a physical device; emulators always allow it |
| `-p N` | parallelism (default NumCPU/2 - each parallel test is a full app process; forced to 1 with `--device`) |
| `--keep-artifacts` | keep passing tests' artifacts too |
| `--update` | rewrite golden files ([widget snapshots](widgets.md)) instead of comparing |
| `-v` | verbose go test output |
| `--timeout` | overall suite timeout (default 10m) |

A bare `go test ./tests/...` also works - the driver finds the app root by walking up to `gantry.json` and builds the binary itself on first Launch. The difference: `gantry test` refreshes the frontend first, a bare `go test` embeds whatever the last `gantry dev`/`gantry build` left in `webdist/`.

## Out of scope

Unit-testing React components in isolation (use vitest + testing-library directly - nothing Gantry-specific about it) and Go unit tests (plain `go test` on your own packages).
