# Your first app

From nothing to a running desktop app in about two minutes, then a
tour of what you just made.

## Scaffold

Open a terminal where you keep projects and run:

```
gantry new myapp
```

The CLI asks a few questions (every one has a flag if you would rather
script it - see the [command reference](../cli/commands.md)):

- Minimize / Maximize / Close buttons - which window buttons the top
  bar shows. These configure the native window itself, not just the
  looks: no maximize button means the window really cannot maximize.
- System tray - yes means closing the window leaves the app running in
  the tray (like Discord or Steam); the tray menu reopens or quits it.
- Multiple pages - no gives you a single page; yes adds a settings
  page and an example component so you can see navigation and
  composition.
- Tea-style pages - yes puts the page logic in Go (the framework's
  favorite trick); no gives you plain React with a channel to Go.

Then it runs npm install, which takes a moment the first time.

## Run it

```
cd myapp
gantry dev
```

Two things start: the Vite dev server (frontend, with hot reload) and
your Go app, whose native window loads from Vite. Edit
pages/index/index.tsx, save, and the window updates instantly. Edit a
.go file and restart gantry dev to pick it up (Go is compiled).

The window that opens is frameless: the top bar with the title and
window buttons is drawn by React (gantry-web's TitleBar), while drag,
resize, and the buttons drive the real native window through the
bridge.

## Build the exe

```
gantry build
```

This builds the frontend into dist/, then compiles a single myapp.exe
with the frontend embedded inside. Copy that one file to another
Windows machine and it runs (WebView2 ships with Windows 10/11). The
exe is a windowed app - no console appears behind it; build with
`gantry build --console` when you want to see main-process logs.

## What the counter demo shows

The index page looks like Vite's starter, but the interesting part is
where the count lives:

- Tea style: the button and count come from index.go. Update handles
  incMsg, View returns the button. React just hosts it with <TeaView />.
  Change View, restart dev, and the UI changes - you are writing UI in
  Go.
- Plain style: the count is React state, and the button also calls
  send("buttonPress", n), which lands in the handler in index.go -
  watch the terminal log. That is the paired-file channel.

## Where to go next

- [Project structure](project-structure.md) - what every file is for
- [The main window](../shell/window.md) - sizing, buttons, geometry
- [Pages and components](../ui/pages-and-components.md) - the pairing system
