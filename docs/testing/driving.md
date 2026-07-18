# Testing: driving the app

This page is the protocol plane: the driver speaking `/gantry/ws` the way the frontend does - mounting pages, reading renders, firing events, awaiting calls, reading shared state, restarting the process. It works headless on every platform, and it is enough to test a Tea-style app end to end. Element-level driving of the real webview is [the DOM plane](dom.md), which layers onto the same `App`; error assertions are on [errors and artifacts](errors-and-artifacts.md).

## Pages and renders

`app.Ready("pages/index")` announces a page mount, like the frontend does when a page component mounts: the server makes it the active page, and a Tea page responds with a full render. The driver keeps every render frame it receives, and three verbs read them.

```go
tree := app.Tree()          // waits for >=1 render, returns the newest tree
tree  = app.NextRender()    // the next render this test has not consumed yet
tree  = app.WaitTree("row appeared", func(n *gantrytest.Node) bool {
	return n.Find("row", gantrytest.Key("user-42")) != nil
})
```

Renders are whole-tree and coalesce under bursts - the newest state wins and intermediate frames may never exist. So never count renders; wait on tree *content* with `WaitTree`, which re-evaluates the predicate against the newest tree as each render lands. This is exactly why the counter test loops on labels rather than clicks:

```go
for _, want := range []string{"count is 1", "count is 2"} {
	app.Click(tree.Find("button", gantrytest.Text("count is")))
	tree = app.WaitTree(want, func(n *gantrytest.Node) bool {
		return strings.Contains(n.Text(), want)
	})
}
```

## Querying trees

A render deserializes into a walkable `Node` tree mirroring the serialized `View()`: `Type`, `Key`, `Props map[string]any`, `Handlers map[string]string`, `Children`.

```go
btn  := tree.Find("button", gantrytest.Text("Save"))   // first match, depth-first; nil if none
rows := tree.FindAll("row")                             // every match, depth-first
name := tree.Find("", gantrytest.Key("title")).Text()  // "" type matches anything
id   := btn.Handler("click")                            // the handler id for an event ("" if none)
```

`Find(typ, matches...)` walks depth-first (including the receiver) and returns the first node whose type equals `typ` (an empty `typ` matches any type) and that satisfies every matcher; `FindAll` returns them all. The matchers:

- `Text(substr)` - the node's own text-bearing props (`text`, `label`, `title`, `placeholder`) contain `substr`. (On the DOM plane this same matcher filters on an element's rendered text - see [the DOM plane](dom.md#finding-elements).)
- `Key(key)` - exact key match.
- `Prop(name, want)` - the prop equals `want`, compared with JSON round-trip semantics (numbers arrive as `float64`, so compare against `float64(3)`, not `3`).
- `HasHandler(event)` - the node carries a handler for that event.
- `MatchFunc(func(*Node) bool)` - any custom predicate.

`Node.Text()` returns the concatenated text of the whole subtree (space-separated), which is what the `strings.Contains(n.Text(), ...)` idiom reads; `Node.Handler(event)` returns the handler id for a single node.

## Firing events

```go
app.Click(btn)                                         // sugar for TeaEvent(btn.Handler("click"), nil)
app.TeaEvent(tree.Find("input").Handler("change"), "Jack") // a Tea handler with a payload
app.SendEvent("pages/settings", "save", map[string]any{"name": "Jack"}) // a paired event (ui.Handlers)
```

`Click(nil)` and firing an empty handler id both fail the test with a clear message ("the Find that produced this node matched nothing" / "node has no click handler"), so a mistyped query surfaces immediately rather than sending a no-op. `TeaEvent` targets a handler by its id from the current render; `SendEvent(key, name, payload)` fires a paired event where `key` names the page/component pair and `name` the handler in its `ui.Handlers`.

## Calls

```go
raw := app.Call("auth", "login", map[string]any{"user": "jack"}) // fails the test if the call rejects
var reply struct{ Name string `json:"name"` }
json.Unmarshal(raw, &reply)

cerr := app.CallFail("pages/debug", "callBoom", nil) // must reject; returns the typed *CallError
if cerr.Code != "panic.call" { t.Errorf("code = %q", cerr.Code) }
```

`Call` / `CallFail` target services and pair keys alike, exactly like `useService` / `usePaired().call` on the frontend. `Call` returns the raw reply payload and fails the test on rejection; `CallFail` is its inverse - it requires a rejection and returns a `*CallError{Code, Message}` so you can assert "this action produces error code X". Built-in service calls are useful probes: `app.Call("gantry", "env", nil)` returns the mode and args, `app.Call("gantry", "appInfo", nil)` the name/title/version stamp, and `app.Call("gantry", "errors", nil)` the error ring buffer (wrapped by [`RecentErrors`](errors-and-artifacts.md)).

## Shared state

```go
if got := app.State("volume").Float(); got != 0.5 { t.Errorf("volume = %v", got) } // waits for the connect snapshot
app.SetState("volume", 0.25)      // a frontend-style write (not echoed back; the driver mirrors it locally)
all := app.States()               // map[string]json.RawMessage of every state value seen so far
```

`State(key)` waits for the value to exist - declared shared state arrives in the connect snapshot - and returns a `Value` with typed accessors: `Float()`, `Int()`, `Bool()`, `String()`, `Decode(&v)`, and `Raw()` for the wire JSON. A type mismatch fails the test with the key and the raw value. `SetState` writes like the frontend's `useGoState` setter; because the server does not echo a writer's own state back, the driver mirrors it locally so a subsequent `State` read reflects what the frontend would see.

## Pushes and generic waits

```go
p := app.WaitPush("pages/settings", "saved")   // next unconsumed push, including one that already landed
app.WaitFor("the export file", func() bool {    // generic polling wait (25ms interval)
	_, err := os.Stat(exportPath)
	return err == nil
})
```

`WaitPush(key, name)` returns the next unconsumed paired push and advances a per-key/name cursor, so consecutive waits step through a stream and one that landed just before the call is still delivered. `WaitFor` is the escape hatch for anything else - a file appearing, a device notification posting - polling your condition against the test deadline.

Every waiting API respects the test's deadline (the launch timeout, capped by the Go test's own deadline) and, on timeout, fails with the last 20 protocol frames attached - the "what was actually happening" you would otherwise re-run to see. See [errors and artifacts](errors-and-artifacts.md#artifacts) for the failure format.

## Launch options

`Launch(t)` works with no options; each option below narrows or extends the instance.

```go
app := gantrytest.Launch(t,
	gantrytest.WithArgs(map[string]any{"mock-data": true}), // declared args, same env vars gantry dev uses
	gantrytest.WithMode("production"),                      // default: development
	gantrytest.WithHeaded(),                                // real window instead of headless
	gantrytest.WithDOM(),                                   // the DOM plane: real webview + CDP, off-screen (see dom.md)
	gantrytest.WithRecording(),                             // record screencast.avi for this test (needs the DOM plane)
	gantrytest.WithEnv("MY_FLAG", "1"),                     // one extra environment variable for the app process
	gantrytest.WithTimeout(20*time.Second),                 // default deadline for every waiting API (default 10s)
	gantrytest.WithBinary("dist/windows/amd64/myapp.exe"),  // launch a prebuilt binary instead of building
	gantrytest.WithAppDir("../.."),                         // override app-root discovery
	gantrytest.KeepArtifacts(),                             // keep this test's artifacts even on pass
)
```

`WithArgs` values are validated against the app's declared args in `gantry.json` - a typoed name fails the test instead of silently doing nothing - and are rendered to the same `GANTRY_ARG_*` (or the spec's explicit `env`) variables `gantry dev` uses, so an arg-driven code path is exercised for real. Supported value types are `string`, `bool`, `int`, and `float64`. (`WithGrantedPermissions` / `WithDeniedPermissions` are device-only; see [mobile](mobile.md#mobile-specific-helpers).)

## Notes

### Handler generations

Handler ids are generation-scoped: each render assigns fresh ones and the server keeps one previous generation, so a click racing a re-render still resolves. Re-query the tree after waiting on a render rather than caching a `Node` and firing its handler much later.

### Restarts and crash recovery

```go
app.Restart() // hard-kill the process tree, relaunch with the same binary, config dir and env, reconnect
app.WaitExit() // block until the process dies on its own (the fatal-crash scenario)
```

The per-test config dir survives a `Restart`, so this is the crash-recovery scenario: after a fatal crash the relaunched app finds the previous run's `crash.log` and reports it, and `app.WaitError("panic.fatal")` asserts it. `WaitExit()` blocks until the process exits by itself, so pairing it before a `Restart` means the relaunch reads a complete `crash.log` rather than racing the dying process. The full walkthrough is on [errors and artifacts](errors-and-artifacts.md#crash-recovery).

### Targets and capabilities

Shared suites gate on the target instead of forking:

```go
gantrytest.DesktopOnly(t)  // skip unless the target is desktop
gantrytest.MobileOnly(t)   // skip unless a device target
if app.Supports(gantrytest.Hover) { ... } // capability check inside a shared test
```

`Target()` reports `"desktop"` or `"android"` (from `GANTRY_TEST_DEVICE`). A `WithDOM()` / headed launch reports the `DOM` and `Hover` capabilities; a protocol-only launch reports every capability false. `Touch` and `Notifications` arrive with the device backend, without changing test code. See [mobile](mobile.md).

Next: [the DOM plane](dom.md).
