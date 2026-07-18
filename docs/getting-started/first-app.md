# Your first app

This is a hands-on lesson: from nothing to a running desktop app in about two minutes, then a short tour of what you just made. Follow it top to bottom - each step ends with something concrete you should see. It assumes you have finished [Installation](installation.md).

## Step 1: Scaffold the app

Open a terminal in the folder where you keep projects and run:

```
gantry new myapp
```

The CLI asks a few questions. Press enter to accept the default (shown in capitals) for each - every question also has a flag if you would rather script it, listed in the [command reference](../cli/commands.md):

- Minimize / Maximize / Close buttons - which window buttons the top bar shows. These configure the native window itself, not just the looks: no maximize button means the window really cannot maximize.
- System tray - yes means closing the window leaves the app running in the tray (like Discord or Steam); the tray menu reopens or quits it.
- Multiple pages - no gives you a single page; yes adds a settings page and an example component so you can see navigation and composition.
- Tea-style pages - yes puts the page logic in Go (the framework's favorite trick); no gives you plain React with a channel to Go.
- Tailwind CSS - no gives you a plain CSS theme; yes wires up Tailwind v4 with theme tokens. Say no for this lesson.

Then it runs `go mod tidy` (Go dependencies) and `npm install` (frontend dependencies), which takes a moment the first time.

When it finishes you have a `myapp/` folder and the CLI prints the next two commands to run.

## Step 2: Run it with live reload

```
cd myapp
gantry dev
```

Two things start: the Vite dev server (frontend, with hot reload) and your Go app, whose native window loads from Vite. Within a few seconds a native window opens showing a counter demo.

Now try the live reload:

- Edit `pages/index/index.tsx`, change some text, and save. The window updates instantly - no restart.
- Edit a `.go` file (say `pages/index/index.go`) and save. `gantry dev` is watching your `.go` files: it rebuilds and restarts the Go app for you, and the window reconnects on its own. (Watch the terminal - it prints "go change - rebuilding the app".) A build error keeps the dev server up and waits for your next save, so a typo will not tear everything down.

The window that opens is frameless: the top bar with the title and window buttons is drawn by React (gantry-web's `TitleBar`), while drag, resize, and the buttons drive the real native window through the bridge.

Press Ctrl+C in the terminal to stop.

## Step 3: Build the exe

```
gantry build
```

This builds the frontend (embedded into the binary) and compiles the app for your machine into `dist/windows/amd64/myapp.exe` (or your own `os/arch`). Copy that one file to another machine and it runs - WebView2 ships with Windows 10 and 11. The exe is a windowed app with your `icons/icon.ico` baked in.

That is the whole loop: `gantry new`, `gantry dev`, `gantry build`. Everything below explains what you just built.

## What the counter demo shows

The index page looks like Vite's starter, but the interesting part is where the count lives:

- Tea style: the button and count come from `index.go`. Update handles `incMsg`, View returns the button, and React just hosts it with `<TeaView />`. Change View, save, and the UI changes - you are writing UI in Go.
- Plain style: the count is React state, and the button also calls `send("buttonPress", n)`, which lands in the handler in `index.go` - watch the terminal log. That is the paired-file channel.

## The main.go you got

Open it - it is about a dozen lines: `gantry.Run` with the app's name, title, port, the embedded frontend (`dist()`), and `gantryPairs()` (a generated function registering every page and component automatically - add a folder, run `gantry dev`, it is registered). Services, shared state, and API routes hang off the Setup hook when you need them; window tweaks off the Window hook. The full expanded wiring, for when an app outgrows Run, is documented in [Without the CLI](../advanced/without-the-cli.md).

## Notes

- `gantry build --console` keeps the console window on Windows so you can see main-process logs while debugging.
- Extra build targets and installers live in `gantry.json` - the [command reference](../cli/commands.md) has the full build story.

## Where to go next

- [Project structure](project-structure.md) - what every file is for
- [The main window](../shell/window.md) - sizing, buttons, geometry
- [Pages and components](../ui/pages-and-components.md) - the pairing system
