# App args

Apps often need switches that change behavior per launch without a rebuild: pointing at a staging API, serving mock data, tuning a poll interval. Gantry's arg harness turns those into first-class declarations. You declare each arg once in `gantry.json`; `gantry dev` validates it on the command line and lists it in `--help`; and the value reaches the app as an environment variable that both the Go side and the React side can read - in development and in a shipped production exe. The mechanism spans three files: `cmd/gantry/args.go` (validation, flag registration, env rendering), the generated `gantry_args.go` (the spec baked into the binary), and `gantry/args.go` (runtime resolution).

## Declaring args

Args live in `gantry.json` under `args`, keyed by flag name (lowercase kebab-case):

```json
{
  "name": "myapp",
  "args": {
    "mock-data": {
      "type": "bool",
      "default": false,
      "description": "serve fake data instead of hitting the real API"
    },
    "api-host": {
      "type": "string",
      "default": "localhost",
      "description": "backend host the app talks to"
    },
    "poll-seconds": {
      "type": "int",
      "default": 30,
      "env": "MYAPP_POLL"
    }
  }
}
```

Each spec takes:

- `type` - `string` (the default when omitted), `bool` or `int`.
- `default` - the value when the flag and env var are both absent; it must JSON-parse to the declared type. Omit it and you get the type's zero value (`""`, `false`, `0`).
- `description` - one line, shown by `gantry dev --help` (defaults to "app arg (gantry.json)" when absent).
- `env` - an explicit environment variable name. Without it the name derives as `GANTRY_ARG_<UPPER_SNAKE>` (`api-host` -> `GANTRY_ARG_API_HOST`).

Arg names must match `^[a-z][a-z0-9-]*$`, an explicit env name must match `^[A-Z][A-Z0-9_]*$`, and a name must not shadow gantry's own flags. The reserved set is exactly: `vite-port`, `port`, `dev-url`, `browser`, `no-open`, `tray`, `no-tray`, `token`, `announce-ready`, `emit-widgets`, `shellrole`, `url`, `monitor`, `position`, `help`, `h`. A malformed name, reserved name, unknown type, mistyped default or bad env name is rejected at validation time with a `config.bad-arg-spec` error naming the exact key (e.g. `args.poll-seconds: default "x" is not an int`) - see [Errors](errors.md).

## Passing args in development

`gantry dev` loads `gantry.json`, validates the arg specs, then registers every declared arg as a real stdlib `flag` on its FlagSet, so `--name=value` parsing, unknown-flag errors and `--help` come for free (the only built-in dev flag is `--vite-port`):

```
gantry dev --mock-data --api-host=10.0.0.5
gantry dev --help          lists every declared arg with type, default and env var
gantry dev --bogus         errors: flag provided but not defined
```

One stdlib-flag quirk to know: bool flags are `--mock-data` or `--mock-data=false`, never `--mock-data false` (the space-separated form only works for string and int args). Everything after `--` is passed to the spawned app process raw (`gantry dev -- --no-tray`).

The CLI resolves each arg (command line beats declared default), renders them as `NAME=value` assignments, and sets those environment variables on the spawned `go run` process alongside `GANTRY_MODE=development` (see [Modes](modes.md)). Helper windows - widgets and popups run as `--shellrole` child processes (see [Architecture](architecture.md)) - inherit the environment automatically, so args are consistent across every window of the app.

## Args in production

A built exe carries its arg spec inside it. `gantry dev`, `gantry build` and `gantry gen` regenerate `gantry_args.go` at the app root (the same convention as `gantry_registry.go`), whose `init()` registers the declarations with the runtime via `gantry.SetArgSpecs`. At launch the runtime resolves each arg from its environment variable, falling back to the declared default - so a plain double-click launch gets all defaults, and a shortcut, script or service definition can override any of them:

```
set GANTRY_ARG_API_HOST=api.internal && myapp.exe
```

An invalid value in the environment (letters in an int arg) never crashes a shipped app: `ArgSpec.resolve` logs a warning (`gantry: arg ...: ... is not an int, using default ...`) and the default applies. Note the usual generated-file caveat - editing `gantry.json` without rerunning dev/build regenerates nothing, so run `gantry gen` if you need `gantry_args.go` refreshed by hand.

## Reading args in Go

```go
import "github.com/BlueBeard63/Gantry/gantry"

gantry.Arg("api-host")      // "10.0.0.5" - any arg formatted as a string
gantry.ArgBool("mock-data") // true
gantry.ArgInt("poll-seconds")
gantry.Args()               // map[string]any of every declared arg, resolved
```

Undeclared names return zero values (`""`, `false`, `0`); `ArgBool`/`ArgInt` also return zero for a name declared as a different type. `Args()` returns declared args only - never the raw process environment - so nothing sensitive leaks through it. It is exactly what the built-in `gantry` service's `env` call hands the frontend.

## Reading args in React

The built-in `gantry` service serves the resolved args (and the mode) to the frontend over the websocket (`web/src/env.ts`):

```tsx
import { useEnv, useArg, useMode } from "gantry-web";

const env = useEnv();                    // { mode, args } - null until the first fetch
const host = useArg<string>("api-host"); // one value - undefined until fetched (and for undeclared names)
const mode = useMode();                  // "development" | "production" | null until fetched

if (env?.args["mock-data"]) {
  // render the mock banner, skip the real fetch, ...
}
```

Values are fetched once (`call("gantry","env")`) and cached module-wide - they cannot change while the app runs, so the cache never invalidates. Gating a whole page or feature on an arg is just an `if` in the page component; combine with `useMode()` to build dev-only pages (see [Modes](modes.md)).

## Environment variables at a glance

The variables that cross the process boundary for app configuration:

- `GANTRY_ARG_<UPPER_SNAKE>` (or your explicit `env` name) - one declared arg's value, read by `ArgSpec.resolve` against the baked-in spec.
- `GANTRY_MODE` - `development` or `production`, the mode override checked first (see [Modes](modes.md)).

Both are set by `gantry dev` on the app process and inherited by helper windows. A shipped binary reads whatever the launching shell, shortcut or service set - or nothing, giving defaults and production mode.
