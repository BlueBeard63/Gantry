# Installation

This page gets your machine ready to build Gantry apps. If you already have Go and Node installed you only need the CLI step at the bottom.

## What you need and why

Gantry apps are two halves working together:

- Go compiles your app into a single .exe. The Go half owns the native window, the tray icon, and your app logic.
- Node (with npm) builds the frontend. The React half is what you see inside the window. Node is only needed while developing - the built exe does not need Node on the machine it runs on.
- WebView2 renders the frontend inside the native window. Windows 10 and 11 ship it with the OS, so normally there is nothing to install.

## Install Go

Download Go from https://go.dev/dl/ and run the installer. Gantry needs Go 1.25 or newer. To check what you have, open a terminal (press the Windows key, type "terminal", press enter) and run:

```
go version
```

If it prints something like "go version go1.25.0 windows/amd64" you are set.

## Install Node

Download the LTS installer from https://nodejs.org and run it. Check it with:

```
node --version
npm --version
```

Any recent LTS (v20 or newer) is fine.

## Linux prerequisites

Building on Linux needs the GTK and WebKit development packages (the exe then runs on machines with the ordinary runtime libraries):

```
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config gcc
```

On distros that only ship `webkit2gtk-4.1` (Ubuntu 24.04+), alias the 4.0 pkg-config name the webview library asks for - the two are API-compatible for what it uses:

```
sudo ln -s /usr/lib/x86_64-linux-gnu/pkgconfig/webkit2gtk-4.1.pc \
           /usr/lib/x86_64-linux-gnu/pkgconfig/webkit2gtk-4.0.pc
```

Under WSLg specifically, Gantry automatically disables WebKit's DMA-BUF renderer (it cannot drive WSL's software GL and produces a white, input-dead window); real Linux machines keep the GPU path. Set WEBKIT_DISABLE_DMABUF_RENDERER yourself to override in either direction.

## Install the gantry CLI

The CLI is the tool you will actually type: it scaffolds apps, runs them with live reload, and builds the final exe.

Straight from the module (once the repo is public, or with GOPRIVATE set for private access):

```
go install github.com/B-Commissions/Gantry/cmd/gantry@latest
```

Or from a local checkout (the way to go when developing Gantry itself):

```
git clone https://github.com/B-Commissions/Gantry
cd Gantry
go install ./cmd/gantry
```

`go install` puts `gantry.exe` into your Go bin folder (usually `%USERPROFILE%\go\bin`). If the terminal cannot find the gantry command afterwards, add that folder to your `PATH`.

Check it works:

```
gantry help
```

## How apps find Gantry

A Gantry app depends on the framework's Go module and its npm package (gantry-web). gantry new wires both up in one of two modes:

Published mode (the default): go.mod depends on the module normally (`go mod tidy` resolves the latest tag) and `package.json` depends on the `gantry-web` package from the npm registry. Nothing to set up.

Local-checkout mode (for developing Gantry itself): `go.mod` gets a replace directive pointing at the checkout folder and `package.json` gets `"gantry-web": "file:<checkout>/web"` - every edit to the framework shows up in the app immediately. It activates when you pass `--gantry-dir`, set the `GANTRY_DIR` environment variable, or scaffoldfrom inside a checkout (detected silently); `--no-replace` forces published mode even then.

Private repos need `GOPRIVATE=github.com/B-Commissions` for the published mode's go side.

The CLI also checks for new Gantry releases once a day (a 1.5 second query to the Go module proxy before new/dev/build) and prints a one-line notice when you are behind. Set `GANTRY_NO_UPDATE_CHECK=1` to turn it off; local (devel) builds never check.

Next: [Your first app](first-app.md), or if Go or React are new to you, read the [Go primer](go-primer.md) and [TSX primer](tsx-primer.md) first.