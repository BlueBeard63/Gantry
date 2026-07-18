# State, pushes & restarts

Beyond rendering and calling, an app under test has shared state you can read and write, server pushes you can await, and a process you can restart to exercise crash recovery. This page covers all three, plus the generic polling wait for anything without a dedicated verb, and the launch options and capability gates that shape how these behave. Firing events and awaiting calls is [Events & calls](events-and-calls.md); reading renders and querying the tree is [Pages & the tree](pages-and-tree.md). Asserting on errors a crash produces is [Errors & artifacts](errors-and-artifacts.md); starting from scratch is [Setup](setup.md).

## Reading and writing shared state

Shared state (the Go side of `useGoState`) reads and writes through three verbs:

```go
if got := app.State("volume").Float(); got != 0.5 { // waits for the connect snapshot
	t.Errorf("volume = %v, want 0.5", got)
}
app.SetState("volume", 0.25) // a frontend-style write
all := app.States()          // map[string]json.RawMessage of every value seen
```

`State(key)` waits for the value to exist - declared shared state arrives in the snapshot the server sends on connect - and returns a `Value` with typed accessors (below). Because it *waits*, a `State` read right after `Launch`/`Ready` is safe; it will not race the snapshot. `SetState(key, value)` writes like the frontend's `useGoState` setter. The server does not echo a writer's own state back to it, so the driver mirrors the write locally: a subsequent `State(key)` read reflects exactly what the frontend would see after its own write. `States()` returns a snapshot map of every state value observed so far, as raw wire JSON - useful for dumping everything at once or asserting a key's absence.

### Typed state values

`State` returns a `Value`, a thin wrapper over the raw JSON with typed accessors that fail the test (naming the key and the raw bytes) on a type mismatch:

```go
v := app.State("settings")
v.Float()        // float64
v.Int()          // int
v.Bool()         // bool
v.String()       // string
v.Raw()          // json.RawMessage, the exact wire bytes
var s Settings
v.Decode(&s)     // json.Unmarshal into any shape
```

`Float`/`Int`/`Bool`/`String` are conveniences for scalar state; `Decode(&out)` handles structs, maps and slices; `Raw()` gives you the untouched bytes when you want to compare or log the wire form. A mismatch - calling `.Bool()` on a number, say - fails the test with the key and the raw value, so a wrong accessor is an immediate, legible failure rather than a zero value.

## Awaiting pushes

A paired component can push to the frontend; `WaitPush` awaits the next one:

```go
p := app.WaitPush("pages/settings", "saved") // next unconsumed push for this key/name
var saved struct {
	At string `json:"at"`
}
json.Unmarshal(p, &saved)
```

`WaitPush(key, name)` returns the payload of the next unconsumed paired push for that key and name, and advances a per-key/name cursor. Two consequences matter: a push that landed *just before* the call is still delivered (you cannot miss one by calling a beat late), and consecutive `WaitPush` calls step through a stream of pushes one at a time rather than all returning the first. The cursor is per key/name, so waits for different pushes do not interfere.

## Generic waits

When nothing built in fits - a file appearing on disk, a device notification posting, an external side effect settling - `WaitFor` polls your own condition:

```go
app.WaitFor("the export file", func() bool {
	_, err := os.Stat(exportPath)
	return err == nil
})
```

`WaitFor(what, cond)` calls `cond` every 25ms until it returns true or the deadline passes, then fails with `what` and the last protocol frames. It is the escape hatch behind the specific waits; reach for it when the thing you are waiting on is not a render, state value, push, call, or error. It pairs naturally with `app.URL()` and `app.Port()`, which expose this launch's base URL and websocket port for probing the app's HTTP surface directly from a test.

Every waiting verb on this page honors the test deadline (the launch timeout, capped by the Go test's own deadline) and attaches the last 20 protocol frames on timeout - see the failure format on [Errors & artifacts](errors-and-artifacts.md#artifacts).

## Restarts and crash recovery

The driver can hard-restart the app, which is how you test crash recovery:

```go
app.Restart()  // hard-kill the process tree, relaunch with the same binary/config-dir/env, reconnect
app.WaitExit() // block until the process dies on its own (the fatal-crash scenario)
```

`Restart()` kills the whole process tree and relaunches with the *same* binary, per-test config dir, and environment, then reconnects the protocol plane (and re-attaches the DOM plane if this launch had one). Because the config dir survives the restart, the relaunched app finds the previous run's `crash.log` and reports it - so a `Restart` after a crash is exactly the crash-recovery scenario, which `app.WaitError("panic.fatal")` then asserts. `WaitExit()` blocks until the process exits *by itself*, the fatal-crash case where an uncatchable panic kills the app. Pairing `WaitExit()` before a `Restart()` is the key ordering: it makes the relaunch read a complete `crash.log` rather than racing the still-dying process. The full walkthrough, with the error assertions, is on [Errors & artifacts](errors-and-artifacts.md#crash-recovery).

## Launch options

`Launch(t)` works with no options; each option below narrows or extends the instance. The ones that shape the behavior on this page are the run mode, declared args, wait timeout, and which binary to run:

```go
app := gantrytest.Launch(t,
	gantrytest.WithArgs(map[string]any{"mock-data": true}), // declared args -> the env vars gantry dev uses
	gantrytest.WithMode("production"),                      // default: development
	gantrytest.WithTimeout(20*time.Second),                 // deadline for every waiting API (default 10s)
	gantrytest.WithBinary("dist/windows/amd64/myapp.exe"),  // launch a prebuilt binary instead of building
	gantrytest.WithAppDir("../.."),                         // override app-root discovery
	gantrytest.WithEnv("MY_FLAG", "1"),                     // one extra env var for the app process
	gantrytest.KeepArtifacts(),                             // keep this test's artifacts even on pass
)
```

`WithArgs` values are validated against the app's declared args in `gantry.json` - a typoed name fails the test instead of silently doing nothing - and render to the same `GANTRY_ARG_*` (or the spec's explicit `env`) variables `gantry dev` uses, so an arg-driven code path is exercised for real; supported value types are `string`, `bool`, `int`, and `float64`. `WithMode` runs the app in `"development"` (the default, full error detail) or `"production"`. `WithTimeout` sets the default deadline every waiting verb here honors, still capped by the Go test's own deadline. `WithBinary` runs a prebuilt binary rather than building, and a `Restart` reuses that same binary. `WithAppDir` overrides where the driver discovers the app root. `WithEnv` injects one extra environment variable. `KeepArtifacts` keeps this test's artifact directory even when it passes. The DOM-plane options (`WithDOM`, `WithHeaded`, `WithRecording`) belong to [The DOM plane](dom.md#enabling-it), and the device-only permission options (`WithGrantedPermissions` / `WithDeniedPermissions`) to [mobile](mobile.md#mobile-specific-helpers).

## Targets and capabilities

Shared suites gate on the target and its capabilities instead of forking a separate test per platform:

```go
gantrytest.DesktopOnly(t)  // skip unless the target is desktop
gantrytest.MobileOnly(t)   // skip unless a device target
if app.Supports(gantrytest.Hover) {
	// ... a hover-only assertion
}
```

`Target()` reports `"desktop"` or `"android"` (from `GANTRY_TEST_DEVICE`), and `DesktopOnly`/`MobileOnly` skip a test that does not apply to the current target. `app.Supports(cap)` is the per-capability check for a shared test: a `WithDOM()` or headed launch reports `DOM` and `Hover`; a protocol-only launch reports every capability false; and `Touch` and `Notifications` arrive with the device backend without any change to the test. This lets one test body run everywhere and light up the extra assertions only where the surface supports them - see [The DOM plane](dom.md#capabilities) and [mobile](mobile.md).

---

See also [Pages & the tree](pages-and-tree.md), [Events & calls](events-and-calls.md), and [Errors & artifacts](errors-and-artifacts.md).
