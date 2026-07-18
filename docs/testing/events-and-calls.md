# Events & calls

Finding a node is half a test; the other half is acting on it and reading a Go result back. This page covers firing events into the app - Tea handlers, clicks, and paired `ui.Handlers` events - and awaiting Go calls the way `useService` and `usePaired().call` do on the frontend, including asserting that a call *rejects*. Getting the nodes to fire into is [Pages & the tree](pages-and-tree.md); reading shared state, pushes and restarts is [State, pushes & restarts](state-and-restarts.md). Firing element-level input into the real webview instead of protocol events is [The DOM plane](dom.md); asserting on the errors a call raises is [Errors & artifacts](errors-and-artifacts.md).

## Firing events

Three verbs cover the three ways the frontend sends an event, from most to least sugared:

```go
app.Click(btn)                                              // sugar for a "click" Tea handler
app.TeaEvent(tree.Find("input").Handler("change"), "Jack")  // any Tea handler, by id, with a payload
app.SendEvent("pages/settings", "save", map[string]any{"name": "Jack"}) // a paired ui.Handlers event
```

`Click(n)` reads the node's `"click"` handler id and fires it with a nil payload - the common case, and the one that reads cleanest. `TeaEvent(handlerID, payload)` fires *any* Tea handler by the id you pulled off a node with `Node.Handler(event)`, so it is how you drive `change`, `submit`, `input`, or any non-click event; a `nil` payload sends none. `SendEvent(key, name, payload)` fires a *paired* event, where `key` names the page/component pair and `name` is the handler declared in its `ui.Handlers` - the mechanism paired components use instead of per-node Tea handlers.

### Mistyped queries fail loudly

Both click paths refuse to send a no-op, so a query that matched nothing surfaces at the fire, not three assertions later:

```go
app.Click(nil)                 // fails: "Click(nil) - the Find that produced this node matched nothing"
app.TeaEvent("", "Jack")       // fails: empty handler id (did Find miss, or the node lack that handler?)
```

`Click(nil)` fails because `Find` returned `nil`; `Click` on a node with no click handler fails naming the node's key and type; `TeaEvent("")` fails because the handler id is empty (typically `Node.Handler("change")` on a node that has no change handler). The upshot: you never chase a silent no-op - a bad selector fails the test with a message that points straight at the query.

## Calls

A call awaits a service or paired reply, exactly like `useService(...)` or `usePaired().call(...)` on the frontend:

```go
raw := app.Call("auth", "login", map[string]any{"user": "jack"}) // fails the test if it rejects
var reply struct {
	Name string `json:"name"`
}
json.Unmarshal(raw, &reply)
```

`Call(key, name, payload)` targets services and pair keys alike - `key` is the service name or the page/component pair, `name` the method. It returns the raw reply payload as `json.RawMessage` for you to `json.Unmarshal` into whatever shape you expect, and it fails the test outright if the call rejects, so a happy-path call needs no error handling of its own.

### Asserting a call rejects

`CallFail` is the inverse: it *requires* a rejection and hands you the typed error, which is how you assert "this action produces error code X":

```go
cerr := app.CallFail("pages/debug", "callBoom", nil) // fails the test if the call SUCCEEDS
if cerr.Code != "panic.call" {
	t.Errorf("code = %q, want panic.call", cerr.Code)
}
```

`CallFail` returns a `*CallError{Code, Message}`: `Code` is the wire `gerr` code (`"panic.call"`, `"auth.expired"`, and so on) and `Message` its human text. If the call unexpectedly *succeeds*, `CallFail` fails the test and reports the payload it got back instead. This is the call-side twin of the error assertions on [Errors & artifacts](errors-and-artifacts.md#asserting-an-error-fires) - use `CallFail` when the failure is the call's own rejection, and `WaitError` when the failure surfaces through the error pipeline.

### Built-in service probes

The `gantry` service answers a few calls that make useful, dependency-free probes of the running app:

```go
env  := app.Call("gantry", "env", nil)      // the run mode and declared args
info := app.Call("gantry", "appInfo", nil)  // the app name / title / version stamp
errs := app.Call("gantry", "errors", nil)   // the error ring buffer (raw)
```

`gantry.env` returns the mode and args the app launched with - handy for confirming a `WithMode` / `WithArgs` launch took effect (see [State, pushes & restarts](state-and-restarts.md#launch-options)). `gantry.appInfo` returns the identity stamp. `gantry.errors` returns the raw error ring buffer; you rarely call it directly, because [`app.RecentErrors()`](errors-and-artifacts.md) wraps it into decoded `ErrorInfo` values and folds it into the error assertions.

## Notes

### Handler generations

Handler ids are generation-scoped: each render assigns fresh ids and the server keeps exactly one previous generation, so a click that races an in-flight re-render still resolves against the generation it was read from. The practical rule follows directly - re-query the tree after waiting on a render rather than caching a `Node` and firing its handler many renders later, because an id from two generations back is gone. The "act, wait for the consequence, reuse the returned tree" loop on [Pages & the tree](pages-and-tree.md#why-you-wait-on-content-never-on-counts) keeps you on a live generation for free.
