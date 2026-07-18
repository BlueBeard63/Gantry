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

- [Window options](shell/window-options.md) - every WindowOptions field explained
- [Frame and window chrome](shell/window-chrome.md) - frameless mode, title-bar buttons, hit-test metrics
- [Close behavior and app lifecycle](shell/close-and-lifecycle.md) - OnCloseRequest, tray life
- [The system tray](shell/tray.md) - tray apps and menu actions
- [Widgets](shell/widgets.md) - small always-on-top helper windows
- [Notifications](shell/notifications.md) - popups that cannot be missed
- [Monitors and icons](shell/monitors-and-icons.md) - placement and iconography

The UI layer (pages and components)

- [Pairs](ui/pairs.md) - the paired-file model: keys, usePaired, data flow both ways, registration
- [Pages](ui/pages.md) - the routable pair: Route/Model/On/Call and the tsx chrome/route/layout exports
- [Components](ui/components.md) - the reusable pair: importing it vs rendering it from Go
- [Routing basics](ui/routing.md) - route derivation, navigate/Link/useRoute/isActive, ExternalLink
- [Dynamic routes](ui/dynamic-routes.md) - [id]/[...slug] folders, useParams, the Go half
- [Awaited Go calls](ui/calls.md) - Call/CallResult, useCall, resolution order, the call timeout
- [Services and hooks](ui/services.md) - Service/useService, useAuth-style hooks, the built-in gantry service
- [Await and Skeleton](ui/await.md) - the Await and Skeleton components and their props
- [State](ui/state.md) - useGoState, shared values both sides own
- [Layouts](ui/layouts.md) - shared navbars/sidebars around pages
- [The Tea model](ui/tea-model.md) - Model, Update, View and the update loop in Go
- [The node tree](ui/tea-nodes.md) - the node builders (Column/Button/Input...) and modifiers
- [Commands and messages](ui/tea-commands.md) - Msg/Cmd, Batch/Tick, App.Send, ParamsMsg
- [Custom components](ui/custom-components.md) - rendering your React from Go
- [Styling](ui/styling.md) - colocated css and theme variables
- [HTTP endpoints](ui/http-endpoints.md) - serving your own routes on the app server
- [Resources](ui/resources.md) - embedded images, fonts and data, shared by both planes
- [The TitleBar](ui/titlebar.md) - configuring the window chrome

Mobile (the same app on a phone)

- [Android builds](mobile/android.md) - APKs, the toolchain, permissions, the phone dev loop
- [Home-screen widgets](mobile/widgets.md) - declarative Glance widgets from paired Go files
- [Notifications](mobile/notifications.md) - system notifications with actions from Go
- [iOS](mobile/ios.md) - the experimental Xcode scaffold

Testing (end-to-end tests against the real app)

- [Setup](testing/setup.md) - the gantrytest driver, your first test, the gantry test command
- [Pages and the tree](testing/pages-and-tree.md) - renders, tree queries and matchers
- [Events and calls](testing/events-and-calls.md) - firing events, awaiting Go calls
- [State, pushes and restarts](testing/state-and-restarts.md) - state, pushes/waits, restarts, launch options
- [The DOM plane](testing/dom.md) - element driving over CDP: real clicks and typing, screenshots, screencasts
- [Errors and artifacts](testing/errors-and-artifacts.md) - error assertions, traces, per-test recordings
- [Widget snapshots](testing/widgets.md) - host-side widget tests and golden files
- [Mobile testing](testing/mobile.md) - where the device story is going
- [CI](testing/ci.md) - running the suite in a pipeline

The CLI

- [Project and build](cli/project.md) - new, build, install, add, update, upgrade
- [Developing and generating](cli/develop.md) - dev and gen
- [The test command](cli/test-command.md) - gantry test and its flags
- [Mobile and docs](cli/mobile-and-docs.md) - mobile dev, docs, version

Advanced

- [Architecture](advanced/architecture.md) - processes, transport, embedding
- [App args](advanced/args.md) - declared args, gantry dev flags, env vars
- [Modes](advanced/modes.md) - development vs production gating
- [Errors and crash handling](advanced/errors.md) - the error pipeline, interception, gerr codes
- [The wire protocol](advanced/protocol.md) - what travels over the websocket
- [Win32 notes](advanced/win32-notes.md) - hard-won Windows lessons
- [Without the CLI](advanced/without-the-cli.md) - manual build wiring
