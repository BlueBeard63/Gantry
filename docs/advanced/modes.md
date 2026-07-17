# Modes

Every Gantry app runs in one of two modes: `development` or `production`. The mode is how you gate things that should differ between working on the app and shipping it - debug pages, verbose error detail, mock endpoints, experimental features.

## How the mode is decided

- `gantry dev` sets `GANTRY_MODE=development` on the app process (and on Vite). Helper windows inherit it.
- A built binary defaults to `production` - nothing needs to be set or passed.
- The full resolution order is: an explicit `GANTRY_MODE` environment variable wins, else a `--dev-url` flag means development (this covers a plain `go run . --dev-url ...` without the CLI), else production.

That last point also means you can force a production binary into development mode for debugging: `set GANTRY_MODE=development && myapp.exe`.

## Reading the mode

Go:

```go
if gantry.IsDev() {
    app.Service("debug", debugCalls) // dev-only service
}
gantry.Mode() // "development" | "production"
```

React:

```tsx
import { useMode } from "gantry-web";

const mode = useMode(); // null until fetched, then "development" | "production"
if (mode === "development") {
  // show the debug panel
}
```

The mode rides the same `gantry` service call as the [args](args.md) - `useEnv()` returns both together.

## Build-time gating

`useMode()` is a runtime check: the code for both branches ships in the bundle. For dev-only code that should not exist in a production build at all, use Vite's constants - `import.meta.env.DEV` and `import.meta.env.PROD` already align exactly with gantry's modes (`gantry dev` runs `vite dev`, `gantry build` runs `vite build`), and Vite tree-shakes the dead branch out of the production bundle:

```tsx
if (import.meta.env.DEV) {
  // stripped from production bundles entirely
}
```

The built-in error UI also keys off the mode: full stacks and action trails in development, a friendly minimal card in production - see [Errors](errors.md).
