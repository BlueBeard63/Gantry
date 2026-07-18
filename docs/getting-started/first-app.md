# Your first app

A hands-on lesson: from nothing to a running desktop app in about two minutes, then a short tour of the loop you just ran. Follow it top to bottom - each step ends with something concrete you should see. It assumes you have finished [Installation](installation.md). The whole loop is three commands: `gantry new`, `gantry dev`, `gantry build`.

## Step 1: Scaffold the app

Open a terminal in the folder where you keep projects and run:

```
gantry new myapp
```

The name becomes your Go module name and the exe name, so it must be a single path-safe word (no spaces or slashes). The CLI then asks seven questions; press Enter to take the default shown in capitals. Each one also has a flag if you would rather script the whole thing non-interactively:

| Prompt | Default | Flag to skip it | What it sets |
| --- | --- | --- | --- |
| Minimize button? | Y | `--buttons` | whether the title bar shows a minimize button |
| Maximize button? | N | `--buttons` | whether the window can maximize at all (not just cosmetic) |
| Close button? | Y | `--buttons` | whether the title bar shows a close button |
| System tray (close keeps the app running)? | Y | `--tray` / `--no-tray` | closing the window leaves the app in the tray (like Discord) vs. quits |
| Multiple pages (adds pages/settings + an example component)? | N | `--multi` / `--single` | single page, or a second page + example component to show navigation |
| Tea-style pages (UI logic in Go)? | Y | `--tea` / `--plain` | page logic in Go (Model/Update/View) vs. plain React with a channel to Go |
| Tailwind CSS (utility classes + theme tokens)? | N | `--tailwind` / `--no-tailwind` | Tailwind v4 wiring vs. a plain CSS theme |

`--buttons` takes a comma list, e.g. `--buttons minimize,close`. Other useful flags: `--port` (the local server port, default `8330`), `--dir` (where to create the folder), and `--no-install` (skip the automatic `npm install`). To reproduce this lesson's app in one line: `gantry new myapp --single --tea --no-tailwind --tray`.

After writing the files, the CLI runs **`go mod tidy`** (resolves the Go dependency on the Gantry module) and **`npm install`** (pulls `gantry-web`, React, and Vite). The first run downloads packages, so give it a moment. When it finishes you have a `myapp/` folder and the CLI prints the next two commands. What each generated file is for is covered in [Project structure](project-structure.md).

## Step 2: Run it with live reload

```
cd myapp
gantry dev
```

Two processes start. Vite serves the frontend with hot-module reload on port **5173**, and your Go app launches as `go run . --dev-url http://localhost:5173 --port 8330`, so its native window loads the live frontend from Vite instead of an embedded bundle. Within a few seconds a native window opens showing a counter demo. The terminal prints `watching .go files - save to rebuild the Go app`.

Now try both halves of the live loop:

- **Edit the frontend.** Open `pages/index/index.tsx`, change some text, and save. The window updates instantly through Vite's HMR - no restart, no flicker.
- **Edit the Go half.** Open `pages/index/index.go`, change something, and save. `gantry dev` is watching your `.go` files (and `resources/`); on save it prints `go change - rebuilding the app`, rebuilds and restarts the Go app, and the window reconnects on its own. A **build error does not tear everything down** - the dev server stays up, prints the error, and waits for your next save to retry, so a typo is a quick fix rather than a full restart.

The window is **frameless**: the top bar with the title and window buttons is drawn by React (gantry-web's `TitleBar`), while dragging, resizing, and the buttons drive the real native window through the bridge. Press **Ctrl+C** in the terminal to stop both processes.

## Step 3: Build the exe

```
gantry build
```

This runs Vite once to build the frontend (embedded into the binary via `//go:embed`), then compiles a single self-contained exe for your current machine into `dist/<os>/<arch>/` - on 64-bit Windows that is `dist/windows/amd64/myapp.exe`, with your `icons/icon.ico` baked in as the exe icon and the console hidden (`-H windowsgui`). Copy that one file to another Windows 10/11 machine and it runs - WebView2 is already there, and nothing else needs installing.

Two flags worth knowing now: `gantry build --console` keeps the console window on Windows so you can watch `log` output while debugging, and `gantry build --targets windows/amd64,linux/amd64` overrides the target list (Linux builds must run on a Linux machine; see [Installation](installation.md)). Extra targets and installers can also live permanently in `gantry.json` - the [project & build commands](../cli/project.md) have the full build story.

That is the whole loop. Everything below explains what you just built.

## What the counter demo shows

The index page looks like Vite's starter, but the interesting part is **where the count lives**, and that depends on the style you picked:

- **Tea style** (the default): the button and the count come from `pages/index/index.go`. Its `Update` handles an `incMsg`, its `View` returns the button, and the `.tsx` just hosts it with `<TeaView />`. Change `View`, save, and the UI changes - you are writing UI in Go.
- **Plain style**: the count is React `useState`, and the button also calls `send("buttonPress", n)`, which lands in the `buttonPress` handler in `index.go` and logs to the Go terminal. That is the paired-file channel - watch the terminal as you click.

Either way, the two files are a **pair**: same folder, same base name, one the look and one the logic. That convention is the heart of a Gantry app - see [Pairs](../ui/pairs.md) and [Project structure](project-structure.md).

## The main.go you got

Open `main.go` - it is about a dozen lines. It calls `gantry.Run` with a `gantry.Config`: the app's `Name` and `Title`, the `Port`, `Dist: dist()` (the embedded frontend from `embed.go`), and `Pairs: gantryPairs()` - a generated function that registers every page and component automatically, so adding a folder and running `gantry dev` is all it takes to register a new page. If you chose options that change the window (a maximize button, no minimize, no close), you will also see a `Window:` hook setting them on the shell. Services, shared state, and your own HTTP routes hang off a `Setup:` hook (shown commented out in the generated file) when your app grows into them. The fully expanded wiring, for when an app outgrows `Run`, is in [Without the CLI](../advanced/without-the-cli.md).

## Where to go next

- [Project structure](project-structure.md) - what every generated file and folder is for
- [Window options](../shell/window-options.md) - sizing, geometry and every `WindowOptions` field; [Frame & window chrome](../shell/window-chrome.md) for the frameless frame and title-bar buttons
- [Pairs](../ui/pairs.md) - the pairing system behind pages and components
- [The Tea model](../ui/tea-model.md) - writing page UI in Go
