# Command reference

The gantry CLI has ten commands. dev, build, add, install, upgrade, mobile and docs find the app by walking up from the current directory to the nearest gantry.json, so they work from anywhere inside the app tree. Progress output is coloured when the terminal supports it; piping to a file or setting `NO_COLOR` gives plain text.

## gantry new <name>

Scaffolds an app in `./<name>`. Interactive by default; every prompt has a flag for scripting:

```
gantry new myapp [flags]
```

- `--buttons=minimize,maximize,close` - which window buttons to enable (any comma subset; skips the three button prompts)
- `--tray / --no-tray` - include a system tray or not
- `--single / --multi` - one page, or index + settings + an example component
- `--tea / --plain` - page style: logic in Go (Tea) or plain React with paired handlers
- `--tailwind / --no-tailwind` - set up Tailwind v4: index.css becomes an `@theme` token file (utilities like `bg-surface`, `text-primary`) with the `--gantry-*` chrome variables bridged to the tokens, and the synthesized vite config gains `@tailwindcss/vite`
- `--port N` - the local server port (default **8330**); also the single-instance guard
- `--dir D` - parent directory for the app (default: **current directory**)
- `--gantry-dir D` - path to the local Gantry checkout; default comes from `$GANTRY_DIR`, then auto-detection
- `--no-replace` - do not add a local replace directive; depend on the published module instead (requires `GOPRIVATE` for private repos)
- `--no-install` - skip the npm install at the end

What it generates: `main.go` (**window options**, **tray**, **roles**, **registration**), `embed.go`, `go.mod`, `package.json`, `tsconfig.json`, `gantry.json`, `index.css`, `pages/index` (a Vite-style starter with the counter), `.vscode settings`, `.gitignore`, a placeholder `dist/`, and in multi mode pages/settings plus `components/example`.

## gantry dev

Runs the app with live reload:

1. regenerates the `.gantry/` build root
2. starts the Vite dev server (frontend, HMR)
3. runs `go run . --dev-url http://localhost:5173`, so the native window loads from Vite while `/api` and `/gantry/ws` proxy back to the Go port

Frontend edits apply instantly in the open window. Go edits need a dev restart (Ctrl+C, `gantry dev` again). Flags: `--vite-port N` (default **5173**), plus every arg the app declares in `gantry.json` (`gantry dev --mock-data --api-host=10.0.0.5` - validated, listed by `gantry dev --help`, and handed to the app as environment variables; see [App args](../advanced/args.md)). The app runs with `GANTRY_MODE=development` (see [Modes](../advanced/modes.md)). Everything after `--` goes to the app process unchanged (`gantry dev -- --no-tray`).

## gantry build

Builds every configured target into a per-OS release tree:

```
dist/
  windows/amd64/myapp.exe          (windowed app, icon embedded)
  windows/amd64/myapp-setup.exe    (with installers on + Inno Setup installed)
  linux/amd64/myapp                (+ myapp-linux-amd64.tar.gz)
  mac/arm64/myapp                  (+ myapp-mac-arm64.zip)
  android/myapp-0.1.0.apk          (with an android target - see the Mobile docs)
```

The pipeline: regenerate `.gantry/` and the generated Go files, one vite build into `webdist/` (the embedded frontend), then a go build per target. Targets come from `gantry.json` build.targets (see below) or the `--targets` flag; with neither, the current machine's `os/arch` builds.

Cross-compilation: windows and mac targets build from any machine (mac runs in browser-fallback mode, so it is pure Go). linux targets need a Linux machine - on Windows, run `gantry build` inside **WSL**; other targets are skipped there with a notice, never failed. `android` is its own world - toolchain requirements, the `mobile` config section, permissions and widgets live in the [Android builds](../mobile/android.md) page; a missing mobile toolchain skips that target with a fix hint while the rest of the run continues. `ios` generates an experimental Xcode scaffold - see [iOS](../mobile/ios.md).

Windows exes get `icons/icon.ico` embedded as the executable icon (**Explorer**, **taskbar**, **shortcuts**) automatically when the icons directory exists.

Installers (`build.installer` or `--installer`):

- **windows**: an Inno Setup script is generated next to the exe and compiled to `<name>-setup.exe` when **Inno Setup 6** is installed (https://jrsoftware.org/isinfo.php); without it the `.iss` is left ready to compile.
- **linux**: a `.tar.gz` of the binary.
- **mac**: a `.zip` of the binary.

Flags:

- `--targets windows/amd64,linux/arm64` - override `gantry.json`
- `--installer` - produce install artifacts (overrides `gantry.json`)
- `--console` - keep the console window on Windows builds, for when you need main-process logs from a built exe (child roles always log to `%LocalAppData%\<app>\<role>.log` regardless)

## gantry mobile dev <android|ios>

The phone equivalent of `gantry dev`. For android: checks the toolchain (offering to install the missing SDK pieces), finds the USB-connected phone, builds the APK for its ABI, installs, launches and streams the app's logcat until Ctrl+C. For ios: checks for a Mac with Xcode, generates the experimental scaffold and prints the Xcode hand-off. Details live in the Mobile docs: [Android builds](../mobile/android.md), [iOS](../mobile/ios.md).

## gantry gen

Regenerates `gantry_registry.go` - the file that auto-registers every `pages/`, `components/` and `layouts/` Go half (their exported Page and Component vars) so `main.go` never lists them. dev and build run it automatically; call it by hand before a plain go build after adding or removing pairs.

## gantry test [pattern]

Runs the app's end-to-end tests: Go tests under `tests/` using the `gantrytest` driver, against the real app process. Prepares the app like a build (registries, one vite build), prebuilds the binary once for the suite, then wraps `go test ./tests/...`. `pattern` filters test names; `--headed`, `--record`, `--retries N`, `--mode production`, `--device android`, `-p N`, `--keep-artifacts`, `--update` and `-v` cover the rest. Every run writes a self-contained `gantry_test_report.html` you open with `gantry test --show`. The [Testing docs](../testing/setup.md) walk through all of it, and [the report](../testing/report.md) covers the viewer and retries.

## gantry add <pkg...>

`npm install`, aimed at the app root regardless of where you run it. Frontend dependencies always belong to the app, never to the framework:

```
gantry add recharts
gantry add -D @types/node
```

(Anything after add is passed to npm install verbatim.)

## gantry install --tailwind

Retrofits an optional feature into an existing app - today that means Tailwind v4:

```
gantry install --tailwind [--yes] [--dry-run]
```

What it does:

- migrates `index.css` into the Tailwind structure and shows you the diff before writing (a `index.css.bak` backup is kept). A stock scaffold file is replaced with the standard `@theme` token template; a file with your own colors keeps everything and gains an `@theme` block exposing each custom token as a utility (`--bg-base` becomes `bg-base` via `--color-base`), and any `--gantry-*` variable still holding its scaffold default is re-pointed at your matching token (`--gantry-bg: var(--bg-base)`) so the chrome follows your palette
- installs `tailwindcss` + `@tailwindcss/vite` as devDependencies
- sets `"tailwind": true` in gantry.json and regenerates `.gantry/vite.config.ts` with the plugin - no need to own the vite config

Flags: `--yes` skips the diff confirmation, `--dry-run` prints the diff and steps without writing anything.

## gantry update

Updates the gantry CLI itself to the newest release - the built-in replacement for re-running the `go install ...@latest` line. It looks up the latest tag on the Go module proxy, reinstalls when you're behind, and on Windows renames the running exe aside first (a running binary can't be overwritten; the leftover `gantry-old.exe` is cleaned up by the next update).

```
gantry update
gantry: go install github.com/B-Commissions/Gantry/cmd/gantry@v0.3.4
gantry: updated v0.3.3 -> v0.3.4
gantry: run gantry upgrade inside your apps to pull the matching template and package changes
```

- `--force` reinstalls even when already up to date
- a CLI built from a local checkout is never auto-updated; update it from the checkout with `go install ./cmd/gantry`

The daily update check that dev/build/new print points here when a newer release exists. `GANTRY_NO_UPDATE_CHECK=1` silences the check; `gantry update` itself always asks the proxy.

## gantry upgrade

Brings the current app up to the CLI's version - run it after `gantry update` (or after pulling a new framework release). A release moves the CLI, the Go module and the gantry-web npm package in lockstep, and the synthesized frontend entry must match the installed package, so apps should follow in one step rather than piecemeal:

```
gantry upgrade [--yes] [--dry-run] [--force]
```

What it does, in order:

- bumps the `github.com/B-Commissions/Gantry` requirement (`go get` + `go mod tidy`) - skipped when go.mod `replace`s it with a local checkout
- pins `gantry-web` in package.json to the exact matching version and runs `npm install` - skipped for `file:` links; the pin is a targeted edit, your other dependencies are untouched
- re-renders the scaffold templates with the choices recorded in gantry.json and compares each against your file: identical files are skipped, missing ones offered for creation, and changed ones shown as a diff with a per-file prompt. Tooling files (tsconfig.json, .vscode/, .gitignore, embed.go) default to **overwrite**; files you own (main.go, pages/, layouts/, components/, index.css, README) default to **keep**; go.mod, package.json and webdist/ are never re-rendered
- regenerates the derived files (`.gantry/`, `gantry_registry.go`, `gantry_widgets.go`, `gantry_icons.go`)
- records the framework version in gantry.json's `gantry` field and prints any release notes that apply to the versions you crossed

Flags: `--yes` applies every file's default without prompting, `--dry-run` reports what would change without writing anything, `--force` re-applies even when the app is already at the CLI's version.

## gantry docs [topic]

The documentation browser - these very pages, embedded in the CLI, readable offline in the terminal:

```
gantry docs             open the browser at the index
gantry docs window      jump to the best match for "window"
gantry docs --print tea print a page as plain markdown to stdout
```

Inside the browser:

- left pane: search box and the category tree; right pane: the page
- tab switches focus, arrows move, enter opens
- / starts a search across titles and content; esc cancels
- f lists the current page's links: pick one and enter follows it - internal links navigate here, external links open in your browser (or land on the clipboard if no browser can open)
- b / n go back / forward through your history
- q quits

## gantry --version

Prints the installed CLI version (also `gantry version` or `-v`):

```
gantry --version
Version: v0.3.1
```

Installed with `go install github.com/B-Commissions/Gantry/cmd/gantry@latest` this reports the module tag - the quick way to confirm an update actually took. Built from a local checkout it reports the version stamped from git (a pseudo-version, or `(devel)` plus the commit when no tag info is available).

## gantry.json

The file that makes a folder an app in the CLI's eyes. It carries a `$schema` reference (added by `gantry new`) so editors validate it as you type - unknown keys are flagged and every field shows its docs on hover:

```json
{
  "$schema": "https://raw.githubusercontent.com/B-Commissions/Gantry/main/gantry.schema.json",
  "name": "myapp",           // exe and module name
  "title": "Myapp",          // window title
  "version": "0.1.0",        // shown by installers; stamped into the exe by dev/build - read it as gantry.Version() or useAppInfo()
  "port": 8330,              // local server + single-instance port
  "mode": "single",          // or "multi" - informational
  "style": "tea",            // or "plain" - informational
  "tray": true,              // informational (runtime: --tray/--no-tray)
  "buttons": {               // informational
    "minimize": true, "maximize": false, "close": true
  },
  "icons": "icons",          // directory with icon.ico + icon.png defaults
  "build": {
    "targets": ["windows/amd64", "linux/amd64", "mac/arm64"],
    "console": false,        // keep the console on Windows builds
    "installer": true        // produce Setup.exe / tar.gz / zip
  },
  "mobile": { ... }          // android/ios identity, permissions, widgets - see the Mobile docs
}
```

**name**, **title**, **port**, **version**, **icons** and **build** feed `dev/build`; **mode**, **style**, **tray** and **buttons** record scaffold choices - those live switches are in your `main.go` (and the tray can be flipped at RUN time with the app's own `--tray/--no-tray` flags, no rebuild: `gantry dev -- --no-tray`, or `myapp.exe --no-tray`).

The icons directory holds the app's default iconography: `icon.ico` (**Windows exe + tray**) and `icon.png` (**window, Linux tray**). `gantry new` seeds it with the placeholder glyph - swap the files for your art and every surface follows on the next build. Code-level Icon settings override them.
