# Installation

This page gets your machine ready to build Gantry apps. If you already
have Go and Node installed you only need the CLI step at the bottom.

## What you need and why

Gantry apps are two halves working together:

- Go compiles your app into a single .exe. The Go half owns the native
  window, the tray icon, and your app logic.
- Node (with npm) builds the frontend. The React half is what you see
  inside the window. Node is only needed while developing - the built
  exe does not need Node on the machine it runs on.
- WebView2 renders the frontend inside the native window. Windows 10
  and 11 ship it with the OS, so normally there is nothing to install.

## Install Go

Download Go from https://go.dev/dl/ and run the installer. Gantry needs
Go 1.25 or newer. To check what you have, open a terminal (press the
Windows key, type "terminal", press enter) and run:

```
go version
```

If it prints something like "go version go1.25.0 windows/amd64" you are
set.

## Install Node

Download the LTS installer from https://nodejs.org and run it. Check it
with:

```
node --version
npm --version
```

Any recent LTS (v20 or newer) is fine.

## Install the gantry CLI

The CLI is the tool you will actually type: it scaffolds apps, runs
them with live reload, and builds the final exe.

From a local checkout of the Gantry repo:

```
git clone https://github.com/B-Commissions/Gantry
cd Gantry
go install ./cmd/gantry
```

go install puts gantry.exe into your Go bin folder (usually
%USERPROFILE%\go\bin). If the terminal cannot find the gantry command
afterwards, add that folder to your PATH.

Check it works:

```
gantry help
```

## How apps find Gantry

A Gantry app depends on the framework's Go module and its npm package.
gantry new wires both up for you, pointing at your local checkout:

- go.mod gets a replace directive pointing at the checkout folder
- package.json gets "gantry-web": "file:<checkout>/web"

gantry new finds the checkout from the GANTRY_DIR environment variable,
by walking up from your current folder, or by asking. Setting the env
var once saves the prompt:

```
setx GANTRY_DIR "D:\New Source\B_Commissions\Gantry"
```

If the module is reachable on GitHub instead (private repos need
GOPRIVATE=github.com/B-Commissions), scaffold with gantry new --no-replace.

Next: [Your first app](first-app.md), or if Go or React are new to you,
read the [Go primer](go-primer.md) and [TSX primer](tsx-primer.md) first.
