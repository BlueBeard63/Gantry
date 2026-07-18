# Modes

Every Gantry app runs in one of two modes: `development` or `production`. The mode is how you gate things that should differ between working on the app and shipping it - debug pages, verbose error detail, mock endpoints, experimental features. It rides the same `gantry` service call as [args](args.md), so `useEnv()` returns both together.

## How the mode is decided

The runtime resolves the mode in a fixed order:

1. An explicit `GANTRY_MODE` environment variable wins (`development` or `production`). `gantry dev` sets `GANTRY_MODE=development` on the app process and on Vite; helper windows inherit it.
2. Otherwise, a `--dev-url` flag means development. This covers a plain `go run . --dev-url ...` launched without the CLI.
3. Otherwise, production. A built binary defaults to production with nothing set or passed.

Because the environment variable is checked first, you can also force a shipped production binary into development mode for debugging: `set GANTRY_MODE=development && myapp.exe`.

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

## Runtime gating vs build-time gating

`useMode()` is a **runtime** check: the code for both branches ships in the bundle, and the choice happens on the running client. That is what you want for anything you might toggle without a rebuild.

For dev-only code that should not exist in a production build at all, use Vite's **build-time** constants instead. `import.meta.env.DEV` and `import.meta.env.PROD` already align exactly with gantry's modes (`gantry dev` runs `vite dev`, `gantry build` runs `vite build`), and Vite tree-shakes the dead branch out of the production bundle:

```tsx
if (import.meta.env.DEV) {
  // stripped from production bundles entirely
}
```

The built-in error UI also keys off the mode: full stacks and action trails in development, a friendly minimal card in production - see [Errors](errors.md).
