# Command reference

The gantry CLI has five commands. dev, build, add and docs find the app
by walking up from the current directory to the nearest gantry.json, so
they work from anywhere inside the app tree.

## gantry new <name>

Scaffolds an app in ./<name>. Interactive by default; every prompt has
a flag for scripting:

```
gantry new myapp [flags]
```

- --buttons=minimize,maximize,close - which window buttons to enable
  (any comma subset; skips the three button prompts)
- --tray / --no-tray - include a system tray or not
- --single / --multi - one page, or index + settings + an example
  component
- --tea / --plain - page style: logic in Go (Tea) or plain React with
  paired handlers
- --port N - the local server port (default 8330); also the
  single-instance guard
- --dir D - parent directory for the app (default: current directory)
- --gantry-dir D - path to the local Gantry checkout; default comes
  from $GANTRY_DIR, then auto-detection
- --no-replace - do not add a local replace directive; depend on the
  published module instead (requires GOPRIVATE for private repos)
- --no-install - skip the npm install at the end

What it generates: main.go (window options, tray, roles, registration),
embed.go, go.mod, package.json, tsconfig.json, gantry.json, index.css,
pages/index (a Vite-style starter with the counter), .vscode settings,
.gitignore, a placeholder dist/, and in multi mode pages/settings plus
components/example.

## gantry dev

Runs the app with live reload:

1. regenerates the .gantry/ build root
2. starts the Vite dev server (frontend, HMR)
3. runs go run . --dev-url http://localhost:5173, so the native window
   loads from Vite while /api and /gantry/ws proxy back to the Go port

Frontend edits apply instantly in the open window. Go edits need a dev
restart (Ctrl+C, gantry dev again). Flags: --vite-port N (default 5173).

## gantry build

Produces the distributable exe:

1. regenerates .gantry/
2. vite build -> dist/
3. go build with dist/ embedded

The exe is a windowed app by default - launching it shows no console,
just the window (gantry dev is where logs stream during development).

Flags:

- --console - keep the console window, for when you need main-process
  logs from a built exe (child roles always log to
  %LocalAppData%\<app>\<role>.log regardless).
- -o path - output name (default <name>.exe)

## gantry gen

Regenerates gantry_registry.go - the file that auto-registers every
pages/, components/ and layouts/ Go half (their exported Page and
Component vars) so main.go never lists them. dev and build run it
automatically; call it by hand before a plain go build after adding or
removing pairs.

## gantry add <pkg...>

npm install, aimed at the app root regardless of where you run it.
Frontend dependencies always belong to the app, never to the framework:

```
gantry add recharts
gantry add -D @types/node
```

(Anything after add is passed to npm install verbatim.)

## gantry docs [topic]

The documentation browser - these very pages, embedded in the CLI,
readable offline in the terminal:

```
gantry docs             open the browser at the index
gantry docs window      jump to the best match for "window"
gantry docs --print tea print a page as plain markdown to stdout
```

Inside the browser:

- left pane: search box and the category tree; right pane: the page
- tab switches focus, arrows move, enter opens
- / starts a search across titles and content; esc cancels
- f lists the current page's links: pick one and enter follows it -
  internal links navigate here, external links open in your browser
  (or land on the clipboard if no browser can open)
- b / n go back / forward through your history
- q quits

## gantry.json

The file that makes a folder an app in the CLI's eyes:

```json
{
  "name": "myapp",           // exe and module name
  "title": "Myapp",          // window title
  "port": 8330,              // local server + single-instance port
  "mode": "single",          // or "multi" - informational
  "style": "tea",            // or "plain" - informational
  "tray": true,              // informational
  "buttons": {               // informational
    "minimize": true, "maximize": false, "close": true
  }
}
```

name, title and port feed dev/build (the synthesized index.html title,
the proxy target). The rest records scaffold choices - the live
switches are in your main.go.
