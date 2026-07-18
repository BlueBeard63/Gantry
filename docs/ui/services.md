# Services & hooks

A [call on a pair](calls.md) belongs to one page or component. Some functionality belongs to the whole app - auth, settings, file dialogs, a shared API client - and shouldn't be nailed to a folder. A **service** is a named group of [`ui.Calls`](calls.md) registered app-wide, reachable from any component. Services are the machinery behind hooks like `useAuth`: a few lines of glue over `useService` and `useCall`. This page covers registering a service on the Go side, reaching it from the frontend with `useService` / `service`, building your own hooks on top, and the one service every app gets for free - `gantry`.

## Registering a service (Go)

Register a service in `main.go` (or in `Config.Setup`) with `app.Service(name, calls)`:

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

A service is just a named `ui.Calls` group not tied to any pair's folder. Everything about the [call mechanism](calls.md) applies unchanged: each call runs on its own goroutine off the read loop, a returned value resolves the promise, a returned error rejects it (with its [gerr code](../advanced/errors.md)), a panic is recovered and reported as `panic.call`, and an unanswered call times out after 30s on the frontend. The only difference from a pair `Call` is scope and resolution order: a service is addressed by its bare name and is consulted *after* any page/component `Call` with the same key (see [Resolution order](calls.md#resolution-order-pair-before-service)).

`Config.Setup` is the single place all server-side registrations belong - services (`app.Service`), [shared state](state.md) (`ui.NewState`), and [HTTP routes](http-endpoints.md). It runs on the goroutine that builds the server, so keep registration quick.

## Reaching a service (frontend)

Two functions get you a handle to a service, both returning the same `Service` interface:

```ts
interface Service {
  call: <T = unknown>(name: string, payload?: unknown) => Promise<T>;
}
```

- `useService(name)` - the memoized **hook** form, for use inside a component.
- `service(name)` - a plain **function** usable anywhere: event handlers, module scope, non-hook code.

```tsx
const auth = useService("auth");
await auth.call("login", { user, pass });
```

The handle has a single method, `call<T>(name, payload?)`, that awaits and resolves with `T` (or rejects with a `GantryCallError` carrying the gerr `code`). `useService(name)` is just `useMemo(() => service(name), [name])` - the hook form exists so the handle is stable across renders, nothing more. Under the hood `auth.call("login", body)` is the same wire request as `useCall("auth", "login", body)` on first run; use `service`/`useService` for **actions** you fire imperatively (log in, save, delete) and `useCall` for **reads** you want tracked with loading/error state.

## Building a hook on top

A service plus `useCall` is everything a custom app hook needs. The idiomatic `useAuth` is a few lines of glue:

```tsx
// hooks/useAuth.ts - your own hook
export function useAuth() {
  const auth = useService("auth");
  const me = useCall<User>("auth", "me"); // read: data, loading, error, code, reload
  return {
    ...me,
    login: (user: string, pass: string) =>
      auth.call("login", { user, pass }).then(me.reload),
  };
}
```

The pattern generalizes: pick the reads you want as tracked `useCall` results, pick the actions you want as `service().call(...)` methods, and after an action that changes server state call the read's `reload()` (as `login` does above) so the UI re-fetches. Nothing about this is framework magic - it is `useService` + `useCall`, both documented on their own.

## The built-in "gantry" service

Every app gets one service for free, registered by `gantry.Run` before `Setup` (so an app can override it by registering its own `"gantry"` service). Two of its calls are the ones you'll reach for:

- **`appInfo`** returns the app's identity - `name`, `title`, and `version` - read from `gantry.json` and stamped into the binary by `gantry dev`/`gantry build`. The React side has a dedicated hook so an About page or version tag is one line:

  ```tsx
  const info = useAppInfo(); // { name, title, version } | null until loaded
  return <span>v{info?.version}</span>;
  ```

  `useAppInfo()` returns `AppInfo | null` (null until the first round-trip resolves). `fetchAppInfo()` is the non-hook variant returning `Promise<AppInfo>`. Both cache after the first fetch - the values cannot change while the app runs - so calling either repeatedly is free. On the Go side the same version is available as `gantry.Version()`.

  ```ts
  interface AppInfo {
    name: string;    // gantry.json "name" - the exe/module identity
    title: string;   // gantry.json "title" - the human-facing name
    version: string; // gantry.json "version"
  }
  ```

- **`errors`** returns everything the [error pipeline](../advanced/errors.md) has captured, including errors that fired while the frontend was disconnected and any crash recorded on the previous run. It is how the error UI shows a last-run crash on the very next launch. (The service also carries `env`, `clearErrors`, and `reportError` for the pipeline's internals.)

## Related

- [Awaited Go calls](calls.md) - the call mechanism, `CallResult` shape, resolution order, and timeout that services share.
- [Await & Skeleton](await.md) - the loading/error wrapper for the `useCall` reads your hooks expose.
- [Serving HTTP routes](http-endpoints.md) - for a plain HTTP endpoint when a websocket call is the wrong tool.
