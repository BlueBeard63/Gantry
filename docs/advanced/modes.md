# Modes

Every Gantry app runs in one of two modes: `development` or `production`. The mode is how you gate things that should differ between working on the app and shipping it - debug pages, verbose error detail, mock endpoints, experimental features. It rides the same `gantry` service call as [args](args.md), so `useEnv()` returns both together. The resolution lives in `gantry/mode.go`; the constants are `gantry.ModeDevelopment` and `gantry.ModeProduction`.

## How the mode is decided

`gantry.Mode()` resolves in a fixed order:

1. An explicit `GANTRY_MODE` environment variable wins, but only when it is exactly `development` or `production` (any other value falls through). `gantry dev` sets `GANTRY_MODE=development` on the app process and on Vite; helper windows inherit it.
2. Otherwise, if `gantry.Run` parsed a `--dev-url` flag this launch, the mode is development. This covers a plain `go run . --dev-url ...` started without the CLI (`Run` flips an internal `devURLSeen` flag when it sees the flag).
3. Otherwise, production. A built binary with nothing set or passed defaults to production.

Because the environment variable is checked first, you can also force a shipped production binary into development mode for debugging: `set GANTRY_MODE=development && myapp.exe`.

## Reading the mode

Go (`gantry/mode.go`):

```go
if gantry.IsDev() {                  // shorthand for Mode() == "development"
    app.Service("debug", debugCalls) // dev-only service
}
gantry.Mode() // "development" | "production"
```

React (`web/src/env.ts`):

```tsx
import { useMode } from "gantry-web";

const mode = useMode(); // null until fetched (call("gantry","env")), then "development" | "production"
if (mode === "development") {
  // show the debug panel
}
```

`useMode()` reads the same cached `{ mode, args }` payload as `useEnv()`/`useArg()`; it is `null` on the first render until the one-time fetch resolves.

## Runtime gating vs build-time gating

`useMode()` is a **runtime** check: the code for both branches ships in the bundle, and the choice happens on the running client (reading a value the Go server sent). That is what you want for anything you might toggle without a rebuild - including forcing a shipped binary into dev mode with `GANTRY_MODE`.

For dev-only code that should not exist in a production build at all, use Vite's **build-time** constants instead. `import.meta.env.DEV` and `import.meta.env.PROD` align exactly with gantry's modes (`gantry dev` runs `vite dev`, `gantry build` runs `vite build`), and Vite tree-shakes the dead branch out of the production bundle:

```tsx
if (import.meta.env.DEV) {
  // stripped from production bundles entirely
}
```

The trade-off: build-time gating cannot be flipped at runtime (a `GANTRY_MODE=development` override on a production binary will not resurrect a branch Vite already stripped), while runtime gating keeps both branches in the bundle. Use build-time for code you never want shipped (fixtures, dev-only dependencies); runtime for behavior you may want to toggle in the field.

The built-in error UI also keys off the mode: full stacks and action trails in development, a friendly minimal card in production, and Go-panic notice banners default to development-only (`showGoErrors`) - see [Errors](errors.md).
