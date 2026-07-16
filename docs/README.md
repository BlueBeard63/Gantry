# Gantry documentation

Gantry is a Go framework for building native desktop apps whose interface is React. This documentation assumes nothing: if you have never written Go or React before, start at the top and read down.

Run `gantry docs` in a terminal to browse these pages offline with search, or read them on GitHub.

## Reading order

Getting started

- [Installation](getting-started/installation.md) - prerequisites and setting up the CLI
- [A Go primer](getting-started/go-primer.md) - just enough Go to build apps
- [A TSX primer](getting-started/tsx-primer.md) - just enough React to build pages
- [Your first app](getting-started/first-app.md) - gantry new to a running exe
- [Project structure](getting-started/project-structure.md) - the paired-file convention

The shell (windows and chrome)

- [The main window](shell/window.md) - every WindowOptions field explained
- [Close behavior and app lifecycle](shell/close-and-lifecycle.md) - OnCloseRequest, tray life
- [The system tray](shell/tray.md) - tray apps and menu actions
- [Widgets](shell/widgets.md) - small always-on-top helper windows
- [Notifications](shell/notifications.md) - popups that cannot be missed
- [Monitors and icons](shell/monitors-and-icons.md) - placement and iconography

The UI layer (pages and components)

- [Pages and components](ui/pages-and-components.md) - registration, routing, usePaired
- [Calls, services and shared state](ui/calls-and-state.md) - awaited Go calls, useAuth-style hooks, useGoState
- [Layouts](ui/layouts.md) - shared navbars/sidebars, Link and active state
- [The Tea model](ui/tea.md) - Model, Update, View in Go
- [Custom components](ui/custom-components.md) - rendering your React from Go
- [Styling](ui/styling.md) - colocated css and theme variables
- [The TitleBar](ui/titlebar.md) - configuring the window chrome

Mobile (the same app on a phone)

- [Android builds](mobile/android.md) - APKs, the toolchain, permissions, the phone dev loop
- [Home-screen widgets](mobile/widgets.md) - declarative Glance widgets from paired Go files
- [Notifications](mobile/notifications.md) - system notifications with actions from Go
- [iOS](mobile/ios.md) - the experimental Xcode scaffold

The CLI

- [Command reference](cli/commands.md) - new, dev, build, add, docs, mobile

Advanced

- [Architecture](advanced/architecture.md) - processes, transport, embedding
- [The wire protocol](advanced/protocol.md) - what travels over the websocket
- [Win32 notes](advanced/win32-notes.md) - hard-won Windows lessons
- [Without the CLI](advanced/without-the-cli.md) - manual build wiring
