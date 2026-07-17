# App args

Apps often need switches that change behavior per launch without a rebuild: pointing at a staging API, serving mock data, tuning a poll interval. Gantry's arg harness turns those into first-class declarations: you declare each arg once in `gantry.json`, `gantry dev` validates it on the command line and lists it in `--help`, and the value reaches the app as an environment variable that both the Go side and the React side can read - in development and in a shipped production exe.

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

- `type` - `string` (the default), `bool` or `int`
- `default` - the value when the flag/env var is absent; must match the type (zero value when omitted: `""`, `false`, `0`)
- `description` - one line, shown by `gantry dev --help`
- `env` - an explicit environment variable name; without it the name derives as `GANTRY_ARG_<UPPER_SNAKE>` (`api-host` becomes `GANTRY_ARG_API_HOST`)

Arg names must not shadow gantry's own flags (`port`, `dev-url`, `tray`, ...) - the CLI rejects those with a `config.bad-arg-spec` error.

## Passing args in development

`gantry dev` registers every declared arg as a real flag, so validation, `--name=value` parsing and help come for free:

```
gantry dev --mock-data --api-host=10.0.0.5
gantry dev --help          lists every declared arg with type, default and env var
gantry dev --bogus         errors: flag provided but not defined
```

One stdlib-flag quirk to know: bool flags are `--mock-data` or `--mock-data=false`, never `--mock-data false` (the space form only works for string and int args). Everything after `--` still goes to the app process raw (`gantry dev -- --no-tray`), exactly as before.

The CLI resolves each arg (command line > declared default), sets the environment variables on the spawned app process, and also sets `GANTRY_MODE=development` (see [Modes](modes.md)). Helper windows - widgets and popups run as `--shellrole` child processes - inherit the environment automatically, so args are consistent across every window of the app.

## Args in production

A built exe carries its arg spec inside it: `gantry dev`, `gantry build` and `gantry gen` regenerate `gantry_args.go` at the app root (the same convention as `gantry_registry.go`), which registers the declarations with the runtime. At launch the runtime resolves each arg from its environment variable, falling back to the declared default - so a plain double-click launch gets all defaults, and a shortcut, script or service definition can override any of them:

```
set GANTRY_ARG_API_HOST=api.internal && myapp.exe
```

An invalid value in the environment (letters in an int arg) never crashes a shipped app: it logs a warning and the default applies. Note the usual generated-file caveat: editing `gantry.json` without rerunning dev/build regenerates nothing - run `gantry gen` if you need `gantry_args.go` refreshed by hand.

## Reading args in Go

```go
import "github.com/B-Commissions/Gantry/gantry"

gantry.Arg("api-host")      // "10.0.0.5" - any arg as a string
gantry.ArgBool("mock-data") // true
gantry.ArgInt("poll-seconds")
gantry.Args()               // map[string]any of every declared arg, resolved
```

Undeclared names return zero values. `Args()` returns declared args only - never the raw process environment - so nothing sensitive leaks through it.

## Reading args in React

The built-in `gantry` service serves the resolved args (and the mode) to the frontend:

```tsx
import { useEnv, useArg, useMode } from "gantry-web";

const env = useEnv();                    // { mode, args } - null until the first fetch
const host = useArg<string>("api-host"); // one value
const mode = useMode();                  // "development" | "production"

if (env?.args["mock-data"]) {
  // render the mock banner, skip the real fetch, ...
}
```

Values are fetched once over the websocket and cached - they cannot change while the app runs. Gating whole pages or features on an arg is just an `if` in the page component; combine with `useMode()` to build dev-only pages (see [Modes](modes.md)).
