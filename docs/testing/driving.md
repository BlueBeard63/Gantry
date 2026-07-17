# Testing: driving the app

Everything on this page is the protocol plane: the driver speaking `/gantry/ws` the way the frontend does. It works headless on every platform. Element-level driving of the real webview is [the DOM plane](dom.md), which layers onto the same `App`.

## Launch options

```go
app := gantrytest.Launch(t,
	gantrytest.WithArgs(map[string]any{"mock-data": true}), // declared args, same env vars gantry dev uses
	gantrytest.WithMode("production"),                      // default: development
	gantrytest.WithHeaded(),                                // real window instead of headless
	gantrytest.WithDOM(),                                   // the DOM plane: real webview + CDP, off-screen (see dom.md)
	gantrytest.WithRecording(),                             // record screencast.avi for this test (needs the DOM plane)
	gantrytest.WithEnv("MY_FLAG", "1"),                     // extra environment for the app process
	gantrytest.WithTimeout(20*time.Second),                 // default deadline for every waiting API (default 10s)
	gantrytest.WithBinary("dist/windows/amd64/myapp.exe"),  // launch a prebuilt binary
	gantrytest.KeepArtifacts(),                             // keep this test's artifacts even on pass
)
```

`WithArgs` values are checked against the app's declared args in `gantry.json` - a typoed name fails the test instead of silently doing nothing.

## Pages and renders

`app.Ready("pages/index")` announces a page mount. For a Tea page the server starts (or re-attaches) the Model and answers with a full render; the driver keeps every render it sees.

```go
tree := app.Tree()                        // newest render (waits for at least one)
tree  = app.NextRender()                  // the next render this test has not consumed
tree  = app.WaitTree("row appeared", func(n *gantrytest.Node) bool {
	return n.Find("row", gantrytest.Key("user-42")) != nil
})
```

Renders are whole-tree and coalesce under bursts - the newest state wins and intermediate frames may never exist. So never count renders; wait on tree content with `WaitTree`.

## Querying trees

A render deserializes into a walkable `Node` tree mirroring the serialized `View()`: `Type`, `Key`, `Props`, `Handlers`, `Children`.

```go
btn := tree.Find("button", gantrytest.Text("Save"))   // first match, depth-first; nil if none
rows := tree.FindAll("row")                           // every match
name := tree.Find("", gantrytest.Key("title")).Text() // "" type matches anything; Text() is the subtree's text
id := btn.Handler("click")                            // the handler id for an event
```

Matchers: `Text(substr)` (text-bearing props: text, label, title, placeholder), `Key(key)`, `Prop(name, want)`, `HasHandler(event)`.

## Firing events

```go
app.Click(btn)                                  // sugar for TeaEvent(btn.Handler("click"), nil)
app.TeaEvent(tree.Find("input").Handler("change"), "Jack") // Tea handler with a payload
app.SendEvent("pages/settings", "save", map[string]any{"name": "Jack"}) // paired event (ui.Handlers)
```

Handler ids are generation-scoped: each render assigns fresh ones and the server keeps one previous generation, so a click racing a re-render still resolves. Re-query the tree after waiting on a render rather than caching handles for long.

## Calls

```go
raw := app.Call("auth", "login", map[string]any{"user": "jack"}) // fails the test if the call rejects
var reply struct{ Name string `json:"name"` }
json.Unmarshal(raw, &reply)

cerr := app.CallFail("pages/debug", "callBoom", nil) // must reject; returns the typed error
if cerr.Code != "panic.call" { t.Errorf(...) }
```

`Call`/`CallFail` target services and pair keys alike, exactly like `useService`/`usePaired().call`.

## Shared state

```go
if got := app.State("volume").Float(); got != 0.5 { t.Errorf(...) } // waits for the connect snapshot
app.SetState("volume", 0.25)      // a frontend-style write (not echoed back; the driver mirrors it)
all := app.States()               // every state value seen so far
```

`Value` has typed accessors - `Float`, `Int`, `Bool`, `String`, `Decode(&v)`, `Raw` - and a type mismatch fails the test with the key and raw value.

## Pushes and generic waits

```go
p := app.WaitPush("pages/settings", "saved")   // next unconsumed push, including one that already landed
app.WaitFor("the export file", func() bool {   // generic polling wait
	_, err := os.Stat(exportPath)
	return err == nil
})
```

Every waiting API respects the test's deadline and, on timeout, fails with the last 20 protocol frames attached - the "what was actually happening" you would otherwise re-run to see.

## Restarts

```go
app.Restart() // hard-kill the tree, relaunch with the same binary, config dir and env, reconnect
```

The config dir survives the restart, so this is the crash-recovery scenario: after a fatal crash, the relaunched app reports the previous run's trace and `app.WaitError("panic.fatal")` asserts it (see [errors](errors-and-artifacts.md)).

## Targets and capabilities

Shared suites gate on the target instead of forking:

```go
gantrytest.DesktopOnly(t)  // skip unless desktop
gantrytest.MobileOnly(t)   // skip unless a device target
if app.Supports(gantrytest.Hover) { ... } // capability check inside a shared test
```

A `WithDOM()`/headed launch reports `DOM` and `Hover`; a protocol-only launch reports every capability false. `Touch` and `Notifications` arrive with the device backends, without changing test code.
