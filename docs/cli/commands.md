# Command reference

The gantry CLI has eleven commands plus `--version`. `dev`, `build`, `add`, `install`, `gen`, `test`, `upgrade` and `mobile` find the app by walking up from the current directory to the nearest `gantry.json`, so they run from anywhere inside the app tree; `new` creates a fresh tree, and `update`, `docs` and `--version` need no app at all. Every command parses its flags with Go's standard `flag` package, so a flag accepts both `-flag` and `--flag`, and a value can be written `--port 9000` or `--port=9000`; boolean flags take no value (`--tray`, not `--tray=true`). Progress output is coloured when the terminal supports it; piping to a file or setting `NO_COLOR` gives plain text. Running `gantry` with no command (or `gantry help`/`-h`/`--help`) prints the one-line usage list.

## gantry new

Scaffolds a new app into `./<name>` (or `<dir>/<name>`). The name becomes the Go module path and the exe name, so it must be a single path-safe word (no spaces or slashes); it is lowercased for the module and title-cased for the window title. The command is interactive by default - every choice has a matching flag, and passing that flag skips its prompt, so a fully-flagged invocation is non-interactive and scriptable. The name can come before or after the flags (`gantry new myapp --multi` or `gantry new --multi myapp`).

```
gantry new <name> [flags]
```

- `--dir D` (string, default: current working directory) - parent directory the app folder is created under.
- `--buttons LIST` (string, default: "") - comma list of titlebar buttons from `minimize,maximize,close`; any subset. Setting it skips all three button prompts (a name absent from the list means that button is off).
- `--tray` / `--no-tray` (bool) - include a system tray, or not. Either flag skips the tray prompt; the tray keeps the app running after the window closes.
- `--single` / `--multi` (bool) - one page, or index + settings + an example component with a shared layout. Either flag skips the pages prompt.
- `--tea` / `--plain` (bool) - page style: UI logic in Go (Tea-style Model/Update/View) or plain React pages with paired Go handlers. Either flag skips the style prompt.
- `--tailwind` / `--no-tailwind` (bool) - set up Tailwind v4 (an `@theme` token `index.css` plus `@tailwindcss/vite` in the synthesized vite config) or a plain-CSS theme. Either flag skips the Tailwind prompt.
- `--port N` (int, default: **8330**) - the local server port, which is also the single-instance guard; written to `gantry.json`.
- `--gantry-dir D` (string, default: `$GANTRY_DIR`, then a silent walk up from the working directory) - path to a local Gantry checkout to depend on (the framework-development workflow). When one is found, `go.mod` and `package.json` link the checkout instead of the published module/npm package.
- `--no-replace` (bool, default: false) - force the published module and npm package even when a local checkout is detected. Private repos then need `GOPRIVATE` set.
- `--no-install` (bool, default: false) - skip the final `npm install`.

### What it generates

`main.go` (window options, tray, roles, registration), `embed.go`, `go.mod`, `package.json`, `tsconfig.json`, `gantry.json`, `index.css` (plain or Tailwind template), `pages/index/` (a starter page with the counter), `pages/index/index.css`, `tests/smoke_test.go`, `.vscode/settings.json` + `.vscode/extensions.json`, `.gitignore`, `README.md`, a placeholder `webdist/index.html`, and real `icons/icon.png` + `icons/icon.ico` seeded from the placeholder glyph. In `--multi` mode it also writes `layouts/main/`, `pages/settings/` and `components/example/`. It then generates `gantry_registry.go`, runs `go mod tidy`, and (unless `--no-install`) `npm install`. Swap the icon files for your own art and every surface (exe, window, tray, installer) follows on the next build.

## gantry dev

Runs the app in a native window with live reload. In order it: regenerates the `.gantry/` vite build root and the generated Go files; starts the Vite dev server (frontend HMR); and runs `go run . --dev-url http://localhost:<vite-port> --port <port>`, so the native window loads from Vite while `/api` and `/gantry/ws` proxy back to the Go port. The app runs with `GANTRY_MODE=development` (see [Modes](../advanced/modes.md)).

Frontend edits (`.tsx`, `.css`) apply instantly through Vite HMR. Go edits are live too: gantry watches your `.go` files and the `resources/` directory and, on save, regenerates the derived Go files, then rebuilds and restarts the Go app; the frontend re-renders on its own when the websocket reconnects to the fresh server. Vite and the Go app run in separate process groups, so a `.go` save restarts only the Go half and leaves Vite's HMR untouched. A failed rebuild or a crash (non-zero exit) leaves the dev server up and waits for the next save to retry rather than tearing everything down; a clean window close (exit 0) ends the session, and Ctrl+C tears every child down.

Flags:

- `--vite-port N` (int, default: **5173**) - the Vite dev server port (started with `--strictPort`, so it fails rather than drifting). The `--dev-url` handed to the app is `http://localhost:<vite-port>`.

Beyond `--vite-port`, `gantry dev` registers every argument the app declares in `gantry.json`'s `args` block as a real flag, so they are validated, listed by `gantry dev --help`, and handed to the app as environment variables - `gantry dev --mock-data --api-host=10.0.0.5`. See [App args](../advanced/args.md). Everything after a bare `--` is passed to the app process unchanged, e.g. `gantry dev -- --no-tray`.

## gantry build

Builds the frontend once, then compiles every configured target into a per-OS/arch release tree under `dist/`:

```
dist/
  windows/amd64/myapp.exe          (windowed app, icon embedded)
  windows/amd64/myapp-setup.exe    (with --installer + Inno Setup installed)
  linux/amd64/myapp                (+ myapp-linux-amd64.tar.gz with --installer)
  mac/arm64/myapp                  (+ myapp-mac-arm64.zip with --installer)
  android/myapp-0.1.0.apk          (with an android target - see the Mobile docs)
```

The pipeline regenerates `.gantry/` and the generated Go files, runs one `vite build` into `webdist/` (the embedded frontend), then runs a `go build` per target. `gantry.json`'s `version` is stamped into the framework via ldflags so the app can read it at runtime (`gantry.Version()` / `useAppInfo()`). Windows exes get `icons/icon.ico` (or `icon.png`) embedded as the executable icon (Explorer, taskbar, shortcuts) automatically when the icons directory exists.

Targets come from `gantry.json`'s `build.targets`, or from `--targets`; with neither, only the current machine's `os/arch` is built. If nothing is built the command errors with "no targets were built".

Flags:

- `--targets LIST` (string, default: "") - comma list of `os/arch` targets, overriding `gantry.json`, e.g. `windows/amd64,linux/arm64`. OS names are `windows`, `linux`, `mac` (aliased from `darwin`), `android`, `ios`. `android` and `ios` may be written bare (both mean `arm64`); android also accepts `amd64`, ios is `arm64` only. A malformed entry errors with the exact bad target.
- `--installer` (bool, default: false; OR'd with `gantry.json` `build.installer`) - also produce install artifacts: an Inno Setup `Setup.exe` on Windows, a `.tar.gz` on Linux, a `.zip` on Mac.
- `--console` (bool, default: false; OR'd with `gantry.json` `build.console`) - keep the console window on Windows builds (without it, Windows targets link `-H windowsgui`). Useful for reading main-process logs from a built exe; child roles always log to `%LocalAppData%\<app>\<role>.log` regardless.

### Cross-compilation and installers

Windows and Mac targets build from any machine (Mac runs in browser-fallback mode, so it is pure Go with `CGO_ENABLED=0`). Linux targets need a Linux machine - on Windows, run `gantry build` inside WSL; non-Linux hosts skip Linux targets with a notice rather than failing. `android` is its own toolchain - see [Android builds](../mobile/android.md); a missing mobile toolchain skips that target with a fix hint while the rest of the run continues. `ios` generates an experimental Xcode scaffold - see [iOS](../mobile/ios.md). On Windows, `--installer` writes a generated `<name>.iss` next to the exe and compiles it to `<name>-setup.exe` when Inno Setup 6's `iscc` is on PATH (or in the standard install locations); without it the `.iss` is left ready to compile.

## gantry add

Installs npm packages into the app, wherever you run it from - a thin `npm install` alias aimed at the app root. Frontend dependencies belong to each app, never to the framework (icon packs, chart libraries, whatever the UI needs). Everything after `add` is passed to `npm install` verbatim, so npm's own flags work.

```
gantry add recharts
gantry add -D @types/node
```

It takes no gantry flags of its own; with no packages it prints its usage.

## gantry install

Retrofits an optional feature into an existing app. Today the only feature is Tailwind v4 - `gantry new --tailwind` does the same for fresh apps, and running it in an app that already has Tailwind is a no-op.

```
gantry install --tailwind [--yes] [--dry-run]
```

Flags:

- `--tailwind` (bool, default: false) - the feature to install. Without it the command errors ("nothing to install - available features: --tailwind").
- `--yes` (bool, default: false) - apply without the diff confirmation prompt.
- `--dry-run` (bool, default: false) - print the `index.css` diff and the planned steps without writing anything.

What `--tailwind` does: migrates `index.css` into the Tailwind structure and shows the diff before writing (an `index.css.bak` backup is kept), installs `tailwindcss@^4` + `@tailwindcss/vite@^4` as devDependencies, sets `"tailwind": true` in `gantry.json`, and regenerates `.gantry/vite.config.ts` with the plugin. A stock scaffold `index.css` is replaced with the standard `@theme` token template; a file with your own colors is kept in full and gains an `@theme` block exposing each custom token as a utility (`--bg-base` becomes `bg-base` via `--color-base`), and any `--gantry-*` chrome variable still holding its scaffold default is re-pointed at your matching token so the window chrome follows your palette.

## gantry gen

Regenerates the CLI's generated Go files by hand. `dev`, `build`, `test` and `upgrade` all do this automatically, so you only need `gen` before a plain `go build` (for example in CI or an IDE run configuration that does not go through gantry). It rewrites, dropping each file when its source is absent:

- `gantry_registry.go` - auto-registers every `pages/`, `components/` and `layouts/` Go half (their exported `Page`/`Component` vars) so `main.go` never lists them. Dynamic `[id]`/`[...slug]` pages are included via an importable mirror generated under `internal/gantrydyn` (Go cannot import bracket-named folders). See [Dynamic routes](../ui/dynamic-routes.md).
- `gantry_widgets.go` - the widget registry (dropped when there are no widgets).
- `gantry_icons.go` - embeds the app's default `icon.png`/`icon.ico` (dropped when no icons directory exists).
- `gantry_resources.go` - embeds a non-empty `resources/` directory (dropped when it is missing or empty - `//go:embed all:resources` will not compile with nothing to match). See [Resources](../ui/resources.md).
- `gantry_args.go` - bakes `gantry.json`'s `args` spec into the binary so a production exe resolves its args from environment variables (dropped when there is no `args` block).

It does not rebuild the `.gantry/` vite root. `gen` takes no flags.

## gantry test

Runs the app's end-to-end tests: Go tests under `tests/` using the `gantrytest` driver, against the real app process. It prepares the app exactly like a build (regenerated `.gantry/`, registries, one `vite build` so the served frontend is current), prebuilds the app binary once for the whole suite (shared with every test via `GANTRY_TEST_BIN`), then wraps `go test ./tests/...` (with `-count=1`, so runs are never cached). The optional `pattern` argument filters test names and is passed through to `go test -run`. Every run writes a self-contained `gantry_test_report.html` under `test-results/`. The [Testing docs](../testing/setup.md) walk through authoring tests, and [the report](../testing/report.md) covers the viewer, retries and flakiness.

```
gantry test
gantry test Counter        # only tests whose name matches "Counter"
gantry test --headed --record
gantry test --show         # just open the latest report
```

Flags:

- `--headed` (bool, default: false) - run apps with the real window instead of headless.
- `--record` (bool, default: false) - record a `screencast.avi` artifact for every DOM-plane test (implies keeping those artifacts).
- `--keep-artifacts` (bool, default: false) - keep passing tests' artifacts too (under `test-results/`); failing tests always keep theirs.
- `--mode M` (string, default: **development**) - app mode for the suite, `development` or `production`; any other value errors.
- `--device D` (string, default: "") - run the suite on a device instead of the desktop: `android` (the sole connected device/emulator) or `android:SERIAL` for a specific one. See Notes below and [Android builds](../mobile/android.md).
- `--allow-device-data` (bool, default: false) - allow the hermetic `pm clear` (which wipes the test app's on-device data) on a physical device; emulators always allow it. Without consent the suite still runs, just without the wipe.
- `-p N` (int, default: **NumCPU/2**, floored at 1) - test parallelism; each parallel test is a full app process. Forced to 1 for `--device` runs.
- `-v` (bool, default: false) - verbose `go test` output.
- `--update` (bool, default: false) - update golden files (widget snapshots) instead of comparing.
- `--retries N` (int, default: **0**) - re-run each failed test up to N times; a test that then passes is reported flaky rather than failed.
- `--timeout D` (duration, default: **10m**) - the overall `go test` timeout, e.g. `--timeout 5m`.
- `--show` (bool, default: false) - open the most recent `gantry_test_report.html` instead of running anything (a viewer shortcut; `pattern` then selects among reports).
- `--open` (bool, default: false) - open the report in the browser when the run finishes.

The command exits non-zero when any test fails, printing a one-line tally (passed / flaky / failed / skipped) and the report path.

## gantry update

Updates the gantry CLI itself to the newest release - the built-in replacement for re-running `go install github.com/B-Commissions/Gantry/cmd/gantry@latest`. It looks up the latest tag on the Go module proxy, reinstalls when you are behind, and on Windows renames the running exe aside first (a running binary cannot be overwritten; the leftover `gantry-old.exe` is cleaned up by the next update). A CLI built from a local checkout is never auto-updated - it tells you to `git pull && go install ./cmd/gantry` from the checkout instead.

```
gantry update
gantry: go install github.com/B-Commissions/Gantry/cmd/gantry@v0.4.0
gantry: updated v0.3.4 -> v0.4.0
gantry: run gantry upgrade inside your apps to pull the matching template and package changes
```

Flags:

- `--force` (bool, default: false) - reinstall even when already up to date.

## gantry upgrade

Brings the current app up to the running CLI's version - run it inside an app after `gantry update` (or after pulling a new framework release). A release moves the CLI, the Go module and the `gantry-web` npm package in lockstep, and the synthesized frontend entry must match the installed package, so apps should follow in one step rather than piecemeal.

```
gantry upgrade [--yes] [--dry-run] [--force]
```

Flags:

- `--yes` (bool, default: false) - apply every file's default action without prompting.
- `--dry-run` (bool, default: false) - report what would change without writing anything.
- `--force` (bool, default: false) - run even when the app is already recorded at this version.

What it does, in order:

- Bumps the `github.com/B-Commissions/Gantry` requirement (`go get` + `go mod tidy`) - skipped when `go.mod` `replace`s it with a local checkout.
- Pins `gantry-web` in `package.json` to the exact matching version and runs `npm install` - skipped for `file:` links; the pin is a targeted edit, your other dependencies untouched.
- Re-renders the scaffold templates with the choices recorded in `gantry.json` and compares each against your file: identical files are skipped, missing ones offered for creation, changed ones shown as a diff with a per-file prompt. Tooling files (`tsconfig.json`, `.vscode/`, `.gitignore`, `embed.go`) default to overwrite; files you own (`main.go`, `pages/`, `layouts/`, `components/`, `index.css`, `README.md`) default to keep. `go.mod`, `package.json` and `webdist/index.html` are never re-rendered (the first two were handled surgically above; the third is build output).
- Regenerates the derived files (`.gantry/`, `gantry_registry.go`, `gantry_widgets.go`, `gantry_icons.go`), records the framework version in `gantry.json`'s `gantry` field, and prints any release notes for the versions you crossed.

## gantry mobile dev

The phone equivalent of `gantry dev`; the `mobile` group has one subcommand today, `dev`, which takes the platform as its argument.

```
gantry mobile dev android
gantry mobile dev ios
```

For **android**: checks the toolchain (offering to install the missing SDK pieces - command-line tools, platform-tools/adb, the NDK; a JDK 17+ is the one thing it will not install for you), finds the single USB-connected device (Wi-Fi/network adb devices are rejected on purpose, emulators count as plugged in), builds the APK for that device's ABI, installs it, launches it, and streams the app's logcat (`gantry-go` tag) until Ctrl+C. It needs a `mobile` section with an `id` in `gantry.json`. For **ios**: requires a Mac with Xcode, generates the experimental scaffold under `.gantry/ios/` and prints the Xcode hand-off (`xcodegen generate && xed .`) - running on a device stays manual. Details live in the Mobile docs: [Android builds](../mobile/android.md), [iOS](../mobile/ios.md). `mobile dev` takes no flags; an unknown or missing platform prints its usage.

## gantry docs

The documentation browser - these very pages, embedded in the CLI and readable fully offline. By default it opens the web viewer in your browser; `-tui` keeps the terminal browser instead. An optional `topic` argument jumps to the best-matching page.

```
gantry docs                  open the web viewer at the index
gantry docs window           open at the best match for "window"
gantry docs -tui             browse in the terminal instead
gantry docs --print tea      print a page as plain markdown to stdout
```

Flags:

- `-tui` (bool, default: false) - browse in the terminal (a Bubble Tea viewer) instead of the web viewer.
- `--print` (bool, default: false) - print the selected page as plain markdown to stdout and exit (good for piping or grepping).
- `--frame` (bool, default: false) - render one frame at the terminal size to stdout and exit (a layout diagnostic for the terminal viewer).
- `--size WxH` (string, default: "") - force the `--frame` size, e.g. `--size 188x41`, for redirecting a frame to a file.

### The terminal browser (`-tui`)

The left pane holds a search box and the category tree, the right pane the page. `tab` switches focus; arrows (or `k`/`j`) move and `enter` opens; `/` starts a search across titles and content and `esc` cancels; `f` lists the current page's links and `enter` follows the pick (internal links navigate here, external links open in your browser, or land on the clipboard if no browser can open); `b`/`n` go back/forward through your history; `pgup`/`pgdown` (and space to page down) scroll the content; `q` (or Ctrl+C) quits. On narrow terminals the sidebar collapses and `tab` toggles between the two panes.

## gantry --version

Prints the installed CLI version. `gantry version` and `gantry -v` are equivalent.

```
gantry --version
Version: v0.4.0
```

Installed with `go install github.com/B-Commissions/Gantry/cmd/gantry@latest` this reports the module tag - the quick way to confirm an update took. Built from a local checkout it reports `(devel)` plus the commit revision when no tag info is available.

## gantry.json

The file that makes a folder an app in the CLI's eyes; the app-finding commands walk up to the nearest one. It carries a `$schema` reference (added by `gantry new`) so editors validate it as you type - unknown keys are flagged and each field shows its docs on hover. Ports default to 8330 and versions to 0.1.0 when omitted.

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/B-Commissions/Gantry/main/gantry.schema.json",
  "name": "myapp",           // exe and Go module name
  "title": "Myapp",          // window title
  "version": "0.1.0",        // shown by installers; stamped into the exe by dev/build (gantry.Version() / useAppInfo())
  "gantry": "0.4.0",         // framework version this app was scaffolded with / last upgraded to (the upgrade baseline)
  "port": 8330,              // local server + single-instance port
  "mode": "single",          // or "multi" - records the scaffold choice
  "style": "tea",            // or "plain" - records the scaffold choice
  "tray": true,              // records the scaffold choice (runtime toggle: --tray/--no-tray)
  "tailwind": true,          // adds @tailwindcss/vite to the synthesized vite config (set by new/install --tailwind)
  "buttons": {               // records the scaffold choice
    "minimize": true, "maximize": false, "close": true
  },
  "icons": "icons",          // directory holding icon.ico + icon.png
  "args": { },               // custom app arguments - see App args
  "build": {
    "targets": ["windows/amd64", "linux/amd64", "mac/arm64"],
    "console": false,        // keep the console on Windows builds
    "installer": true        // produce Setup.exe / tar.gz / zip
  },
  "mobile": { }              // android/ios identity, permissions, widgets - see the Mobile docs
}
```

`name`, `title`, `port`, `version`, `icons`, `tailwind`, `args` and `build` feed `dev`/`build`; `mode`, `style`, `tray` and `buttons` record the scaffold choices (the live switches are in `main.go`, and the tray can be flipped at run time with the app's own `--tray`/`--no-tray` flags without a rebuild: `gantry dev -- --no-tray`, or `myapp.exe --no-tray`). `gantry` is maintained by `gantry new`/`gantry upgrade` and is the baseline `upgrade` reports against. The `mobile` section is documented under the [Mobile docs](../mobile/android.md); the `build` targets and installer behaviour are covered under [gantry build](#gantry-build) above.

## Notes and advanced behaviour

**Update-check notice.** `new`, `dev`, `build`, `test` and `mobile` print a one-line "vX.Y.Z is available" notice when a newer tag exists. The check asks the module proxy at most once a day (cached), spends at most 1.5s on the network, stays silent on any failure, and never runs for local-checkout builds. `GANTRY_NO_UPDATE_CHECK=1` silences it; `gantry update` and `gantry upgrade` always ask the proxy directly (with a more generous timeout).

**`gantry test --device`.** A device run builds and installs a debug APK under `<mobile.id>.test` (an `applicationIdSuffix` of `.test`, so it sits beside any real install and uninstalling it touches nothing else), forces `-p` to 1 (one app instance per device), and needs a `mobile` section with an `id`. `android:SERIAL` targets a specific device; bare `android` uses the sole connected device/emulator. `--allow-device-data` permits the hermetic `pm clear` on a physical device (emulators always allow it); `--timeout` bounds the whole `go test` (default 10m); `--open` opens the report when the run finishes. The test app is uninstalled again when the suite ends.

**Declared app args as env vars.** Each `gantry.json` `args` entry (name in lowercase kebab-case matching `^[a-z][a-z0-9-]*$`, `type` `string`/`bool`/`int`, optional `default`, `description`, `env`) travels to the app as an environment variable - the explicit `env` name (UPPER_SNAKE), or `GANTRY_ARG_<UPPER_SNAKE>` derived from the flag name (`api-host` becomes `GANTRY_ARG_API_HOST`). `gantry dev` registers them as real flags (validation and `--help` for free); a production binary reads the same variables via the generated `gantry_args.go`. Arg names may not shadow gantry's own flags (`port`, `vite-port`, `dev-url`, `tray`, `no-tray`, and so on) - a clash is a config error. See [App args](../advanced/args.md).
