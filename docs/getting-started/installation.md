# Installation

This page gets your machine ready to build Gantry apps. The happy path is three installs - Go, Node, and the gantry CLI - and on Windows there is nothing else to set up because WebView2 already ships with the OS. If you already have Go and Node, skip straight to [Install the gantry CLI](#install-the-gantry-cli). Once this page is done, go build [Your first app](first-app.md).

## What you need and why

A Gantry app is two halves that ship as one .exe, so you install one toolchain for each half plus the CLI that ties them together:

- **Go** compiles your whole app - logic, native window, tray, server, and the embedded frontend - into a single .exe with no runtime to install on the target machine. Gantry's own module targets **Go 1.25.0**, and `gantry new` writes `go 1.25.0` into your app's `go.mod`, so you need **Go 1.25 or newer**.
- **Node (with npm)** runs Vite, which compiles your `.tsx`/`.css` into the JavaScript bundle Go embeds. Node is a **build-time** tool only - the finished exe contains the built frontend and does not need Node (or npm, or Vite) on the machine it runs on.
- **WebView2** is the renderer that draws the frontend inside the native window. Windows 10 and 11 ship the runtime with the OS, so on Windows there is normally nothing to install. (Linux uses WebKitGTK instead - see [Linux prerequisites](#linux-prerequisites); macOS is browser-fallback only for now.)

## Install Go

Download Go from https://go.dev/dl/ and run the installer. Open a fresh terminal (press the Windows key, type "terminal", press Enter) and confirm the version:

```
go version
```

You want `go version go1.25.x windows/amd64` or higher. If the number is below 1.25, download the current release - `gantry new` sets `go 1.25.0` in the generated `go.mod` and `go build` refuses an older toolchain.

## Install Node

Download the **LTS** installer from https://nodejs.org and run it. It includes npm and npx, both of which the CLI shells out to. Check them:

```
node --version
npm --version
```

Any current LTS (v20 or newer) is fine. On Windows the CLI looks for `npm.cmd` and `npx.cmd` specifically, which the official installer puts on your `PATH` - so a normal install just works.

## Install the gantry CLI

The CLI is the one command you actually type: it scaffolds apps (`gantry new`), runs them with live reload (`gantry dev`), and builds the final exe (`gantry build`). Install it straight from the module with `go install` - the CLI is pure Go, so this works on every platform with nothing but the Go toolchain (on Linux the GTK/WebKitGTK packages below are only needed later, when you build an app):

```
go install github.com/BlueBeard63/Gantry/cmd/gantry@latest
```

`go install` compiles `gantry.exe` and drops it in your Go bin directory - `%USERPROFILE%\go\bin` unless you have set `GOBIN`. Run `go env GOPATH GOBIN` if you are unsure where that is. That directory needs to be on your `PATH`; the Go installer usually adds it, but if the next command is "not recognized", add the bin folder to `PATH` and open a new terminal. Verify:

```
gantry --version
gantry help
```

`gantry help` prints the full command list (`new`, `dev`, `build`, `add`, `install`, `gen`, `test`, `update`, `upgrade`, `mobile`, `docs`). That is everything you need. Continue to [Your first app](first-app.md) - or if Go or React are new to you, read the [Go primer](go-primer.md) and [TSX primer](tsx-primer.md) first.

## Keeping the CLI current

Before `new`, `dev`, `build`, and `test`, the CLI does a once-a-day check against the Go module proxy (capped at 1.5 seconds) and prints a one-line notice when a newer Gantry release exists. When you see it, `gantry update` reinstalls the CLI itself (the same `go install ... @latest`), and inside an app `gantry upgrade` brings that app's packages and regenerated scaffold files up to the CLI's version. The two halves of a release move together - the CLI pins the exact matching `gantry-web` npm version into `package.json` - so upgrade the CLI and the app together. Set `GANTRY_NO_UPDATE_CHECK=1` to silence the daily check; a local `(devel)` build (installed from a checkout) never checks. The `update` and `upgrade` commands are documented in [Project & build](../cli/project.md).

## Linux prerequisites

Installing the CLI itself (`go install` above) is pure Go and needs none of this - the packages below are only required once you build or run an app, because building an app links against GTK and WebKitGTK (the resulting exe then runs on machines that only have the ordinary runtime libraries). Gantry links `webkit2gtk-4.1`, the build every current distro ships - no pkg-config symlinks needed.

Debian / Ubuntu:

```
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config
```

Fedora:

```
sudo dnf install gtk3-devel webkit2gtk4.1-devel gcc gcc-c++ pkgconf-pkg-config
```

Do not install or alias the GTK4-based `webkitgtk-6.0` packages - Gantry's shell is GTK3 and cannot link them. On old distros that only ship `webkit2gtk-4.0` (Debian 11, Ubuntu 20.04), build with the legacy name selected: `GOFLAGS=-tags=webkit2gtk40 gantry dev` (or pass `-tags webkit2gtk40` to `go build`). And a `-tags nogui` build needs no GTK packages at all - it strips the native window on every platform, see [Architecture](../advanced/architecture.md).

Under WSLg specifically, Gantry disables WebKit's DMA-BUF renderer for you (it cannot drive WSL's software GL and produces a white, input-dead window); real Linux machines keep the GPU path. Set `WEBKIT_DISABLE_DMABUF_RENDERER` yourself to override in either direction. Note that Windows cannot cross-compile Linux builds - `gantry build` skips a `linux/*` target with a hint to run the build on a Linux machine (WSL counts).

## Building from a local checkout (advanced)

You do not need any of this for your first app - it is here for when you develop Gantry itself or run against the private repo before it is public.

To install the CLI from a clone instead of the module proxy:

```
git clone https://github.com/BlueBeard63/Gantry
cd Gantry
go install ./cmd/gantry
```

An app scaffolded from inside a checkout (or with `--gantry-dir`, or with `GANTRY_DIR` set) points its `go.mod` `replace` directive and its `package.json` `gantry-web` entry at that checkout, so every edit to the framework shows up in the app immediately; pass `gantry new --no-replace` to force the published module even then. If the repo is still private, set `GOPRIVATE=github.com/BlueBeard63` so the Go tool skips the public proxy and checksum database for it. How apps depend on the two halves is covered in [Project structure](project-structure.md).
