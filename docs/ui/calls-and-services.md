# Calls and services

[Paired handlers](pairs.md) are fire-and-forget. This page covers the two ways React asks Go a question and gets an answer: **awaited calls** on a pair (ask this one pair and get a result), and **services** (app-wide call groups, the machinery behind hooks like useAuth). For a live value both sides read and write, see [State](state.md).

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

Calls run on their own goroutine (slow work is fine), a returned error rejects the promise, and an unanswered call times out after 30 seconds instead of hanging forever.

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

Reach it from any component with `useService`, or from non-hook code with `service()`:

```tsx
const auth = useService("auth");
await auth.call("login", { user, pass });
```

For read paths, `useCall` wraps the fetch-shaped boilerplate - it runs on mount, tracks loading/error, and re-runs when its inputs change:

```tsx
const { data: me, loading, error, code, reload } = useCall<User>("auth", "me");
```

That is everything a custom app hook needs:

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

The fallback is any JSX - a spinner, custom skeleton markup shaped like your real layout, or the built-in `Skeleton`: `<Skeleton lines={4}/>` renders shimmering text lines, `<Skeleton width={240} height={120}/>` a sized block, `<Skeleton circle/>` an avatar (it respects prefers-reduced-motion). Nothing about `Await` is mandatory - `loading` from `useCall` is right there for hand-rolled arrangements, and you can replace the error card with `renderError`.

## Picking between the data paths

- One-way notification ("this happened"): usePaired().send + `Handlers` ([Pairs](pairs.md)).
- Question with an answer ("give me X", "do this and tell me how it went"): call / useService / useCall + `Calls`.
- A live value both sides own: useGoState + ui.NewState ([State](state.md)).
- Continuous Go-driven UI: a Tea Model ([The Tea model](tea.md)).

## Notes

- **Error codes.** `code` on a `useCall` result (and the second arg to `renderError`) is the [gerr code](../advanced/errors.md) of a failed call ("auth.expired", "panic.call"), so error handling can switch on identity instead of message text.
- **The built-in "gantry" service.** Every app gets one service for free - `"gantry"`, whose `appInfo` call returns the app's identity (`name`, `title`, and the `version` from gantry.json, which Go can also read as `gantry.Version()`). The React side has a dedicated hook so an About page or version tag is one line: `const info = useAppInfo();` returns `{ name, title, version }` or null until loaded. `fetchAppInfo()` is the non-hook variant; both cache after the first round-trip. Registering your own `"gantry"` service overrides the built-in.
- **Your own HTTP endpoints.** Calls and services ride the websocket. When you need a plain HTTP endpoint instead - a file download, a webhook target, something a non-Gantry client hits - register it on the `*http.ServeMux` in `Config.Setup`. See [Serving HTTP routes](http-endpoints.md).
