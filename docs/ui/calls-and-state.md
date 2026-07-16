# Calls, services and shared state

Three ways React talks to Go beyond fire-and-forget events: awaited calls on a pair, app-level services (the machinery behind hooks like useAuth), and useGoState - a useState whose value lives in Go and syncs both ways instantly.

## Awaited calls on a pair

Handlers (ui.Handlers) are one-way. When the tsx needs an ANSWER, give the pair Calls:

```go
// pages/files/files.go
var Page = ui.Page{
    Key: "pages/files",
    Call: ui.Calls{
        "list": func(p json.RawMessage) (any, error) {
            entries, err := os.ReadDir(somewhere)
            if err != nil {
                return nil, err // rejects the promise with this message
            }
            return names(entries), nil // resolves the promise
        },
    },
}
```

```tsx
// pages/files/files.tsx
const { call } = usePaired();
const files = await call<string[]>("list");
```

Calls run on their own goroutine (slow work is fine), errors reject the promise, and an unanswered call times out after 30 seconds instead of hanging forever.

## Services: app-level call groups

Some functionality belongs to the whole app, not one page - auth, settings, file dialogs. Register it as a service in main.go:

```go
app.Service("auth", ui.Calls{
    "me": func(json.RawMessage) (any, error) {
        return currentUser(), nil
    },
    "login": func(p json.RawMessage) (any, error) {
        var body struct{ User, Pass string }
        if err := json.Unmarshal(p, &body); err != nil {
            return nil, err
        }
        return doLogin(body.User, body.Pass)
    },
})
```

Reach it from any component with useService, or non-hook code with service():

```tsx
const auth = useService("auth");
await auth.call("login", { user, pass });
```

For read paths, useCall wraps the fetch-shaped boilerplate - it runs on mount, tracks loading/error, and re-runs when inputs change:

```tsx
const { data: me, loading, error, reload } = useCall<User>("auth", "me");
```

And that is everything a custom app hook needs:

```tsx
// hooks/useAuth.ts - your own hook, three lines of glue
export function useAuth() {
  const auth = useService("auth");
  const me = useCall<User>("auth", "me");
  return {
    ...me, // data, loading, error, reload
    login: (user: string, pass: string) =>
      auth.call("login", { user, pass }).then(me.reload),
  };
}
```

## useGoState: state that lives in Go

For values both sides read and write - volume, active session, feature toggles - declare a shared state variable in Go:

```go
volume := ui.NewState(app, "volume", 0.5)

volume.Set(0.8)         // every React subscriber re-renders instantly
v := volume.Get()       // current value
volume.OnChange(func(v float64) {
    applyVolume(v)      // React wrote it
})
```

React uses it exactly like useState:

```tsx
const [volume, setVolume] = useGoState("volume", 0.5);
<input type="range" value={volume} onChange={(e) => setVolume(Number(e.target.value))} />
```

Semantics worth knowing:

- Every component using the same key shares one value; a set anywhere re-renders them all.
- Frontend sets apply locally at once (no round-trip lag) and write through to Go; Go's OnChange observers fire.
- Go-side Set() pushes to the frontend immediately - "instant updates" from timers, watchers, background work.
- A fresh or reconnected client receives every declared state before anything else, so useGoState is correct from the first render (the fallback argument only covers the moment before the socket connects).
- Declare states at startup with ui.NewState BEFORE the window opens; writes to undeclared keys are logged and dropped.

## Picking between them

- One-way notification ("this happened"): usePaired().send + Handlers.
- Question with an answer ("give me X", "do this and tell me how it went"): call / useService / useCall + Calls.
- A live value both sides own: useGoState + ui.NewState.
- Continuous Go-driven UI: a Tea Model ([The Tea model](tea.md)).
