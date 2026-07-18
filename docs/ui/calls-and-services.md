# Calls and services

[Paired handlers](pairs.md) are fire-and-forget. This page covers the ways React asks Go a question and gets an answer back: **awaited calls** on a pair (ask this one pair), **services** (app-wide call groups, the machinery behind hooks like `useAuth`), and the read-path helpers - `useCall`, `Await`, `Skeleton`. For a live value both sides read and write, see [State](state.md).

## Awaited calls on a pair

When the tsx needs an ANSWER, give the pair `Call` alongside (or instead of) `On`:

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

`ui.Calls` is `map[string]func(json.RawMessage) (any, error)`. Each call runs on **its own goroutine** off the read loop, so slow work is fine. The returned value must be JSON-serializable and resolves the promise; a returned error rejects it (and is treated as control flow, not a crash - it never hits the [error pipeline](../advanced/errors.md)). A panic in a call is recovered, reported as `panic.call`, and rejects the promise. An unanswered call times out after **30 seconds** on the frontend instead of hanging forever. Go resolves a call by looking at pair keys first (a page's or component's `Call`) and then services, so a pair's own `Call` always answers before an identically named service.

## Services: app-level call groups

Some functionality belongs to the whole app, not one page - auth, settings, file dialogs. Register it as a service in `main.go`:

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

A service is just a named `ui.Calls` group not tied to any pair's folder. Reach it from any component with `useService`, or from non-hook code with `service()`:

```tsx
const auth = useService("auth");
await auth.call("login", { user, pass });
```

Both `useService(name)` and `service(name)` return a handle with a single method, `call<T>(name, payload?)`, that awaits and resolves with `T`. `useService` is the memoized hook form; `service` is a plain function usable anywhere (event handlers, module scope).

## useCall: the read-path helper

For read paths, `useCall` wraps the fetch-shaped boilerplate - it runs the call on mount, tracks loading/error, and **re-runs whenever its key, name, or payload change**:

```tsx
const { data: me, loading, error, code, reload } = useCall<User>("auth", "me");
```

The result is `{ data, error, code, loading, reload }`: `data` is `T | undefined`, `error` is the failure message (or `null`), `code` is the [gerr code](../advanced/errors.md) of a failed call (or `null`), `loading` is `true` until the first response, and `reload()` re-runs the call. It works against a pair key or a service name identically (both are just `key`, `name`, `payload`).

That is everything a custom app hook needs:

```tsx
// hooks/useAuth.ts - your own hook, a few lines of glue
export function useAuth() {
  const auth = useService("auth");
  const me = useCall<User>("auth", "me");
  return {
    ...me, // data, loading, error, code, reload
    login: (user: string, pass: string) =>
      auth.call("login", { user, pass }).then(me.reload),
  };
}
```

## Loading states: Await and Skeleton

While a Go call is in flight you usually want a placeholder, not a blank area. `Await` is the declarative wrapper around a `useCall` result: it shows your fallback while loading, an error card with a Retry button on failure, and your content once data lands:

```tsx
import { Await, Skeleton, useCall } from "gantry-web";

function Users() {
  const users = useCall<User[]>("api", "listUsers");
  return (
    <Await call={users} fallback={<Skeleton lines={4} />}>
      {(list) => (
        <ul>
          {list.map((u) => <li key={u.id}>{u.name}</li>)}
        </ul>
      )}
    </Await>
  );
}
```

`Await`'s props: `call` (any `CallResult`-shaped object), `fallback` (JSX shown while loading - defaults to `<Skeleton lines={3}/>`), `renderError` (`(error, code, reload) => ReactNode` to replace the default error card), and `children` (`(data) => ReactNode`, called once data is present).

`Skeleton` renders shimmering placeholder blocks sized like the content they stand in for: `<Skeleton lines={4}/>` renders text lines (the last shorter), `<Skeleton width={240} height={120}/>` a sized block, `<Skeleton circle/>` an avatar. It respects `prefers-reduced-motion`. Nothing about `Await` is mandatory - `loading` from `useCall` is right there for hand-rolled arrangements.

## Picking between the data paths

- One-way notification ("this happened"): `usePaired().send` + `ui.Handlers` ([Pairs](pairs.md)).
- Question with an answer ("give me X", "do this and tell me how it went"): `call` / `useService` / `useCall` + `ui.Calls`.
- A live value both sides own: `useGoState` + `ui.NewState` ([State](state.md)).
- Continuous Go-driven UI: a Tea Model ([The Tea model](tea.md)).

## Notes

- **Error codes.** `code` on a `useCall` result (and the second arg to `renderError`) is the [gerr code](../advanced/errors.md) of a failed call (`auth.expired`, `panic.call`), carried on the `GantryCallError` a rejected `call` throws, so error handling can switch on identity instead of message text.
- **The built-in "gantry" service.** Every app gets one service for free - `"gantry"`. Its `appInfo` call returns the app's identity (`name`, `title`, and the `version` from `gantry.json`, which Go can also read as `gantry.Version()`), and its `errors` call returns any crash recorded on the previous run. The React side has a dedicated hook so an About page or version tag is one line: `const info = useAppInfo();` returns `{ name, title, version }` or `null` until loaded. `fetchAppInfo()` is the non-hook variant; both cache after the first round-trip. Registering your own `"gantry"` service overrides the built-in.
- **Your own HTTP endpoints.** Calls and services ride the websocket. When you need a plain HTTP endpoint instead - a file download, a webhook target, something a non-Gantry client hits - register it on the `*http.ServeMux` in `Config.Setup`. See [Serving HTTP routes](http-endpoints.md).
