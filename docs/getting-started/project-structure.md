# Project structure

A Gantry app is a Go module with React files living inside it. There is
no web/ directory and no build config to maintain - the CLI synthesizes
those.

```
myapp/
  main.go            the entrypoint: a dozen lines calling gantry.Run
  gantry_registry.go generated page/component registrations - never edit
  app.tsx            optional: TitleBar and app options (see the TitleBar docs)
  embed.go           embeds dist/ into the exe
  go.mod             Go module + dependency on Gantry
  package.json       npm dependencies (react, gantry-web, ...)
  tsconfig.json      makes the editor understand the .tsx files
  gantry.json        app settings + build targets the gantry CLI reads
  index.css          app-wide styles and the theme variables
  icons/             default icon files (icon.ico, icon.png) - swap for your art
  layouts/
    main/
      main.tsx       optional: shared chrome (navbar etc.); pages pick
      main.css       layouts by name, nest them, or opt out
  pages/
    index/
      index.go       page logic (Go)
      index.tsx      page look (React)
      index.css      page styles (optional)
  components/
    example/
      example.go     component logic
      example.tsx    component look
      example.css    component styles (optional)
  .vscode/           editor settings (excludes, recommendations)
  .gantry/           synthesized build root - gitignored, regenerated
  webdist/           built frontend (embedded into the exe) - gitignored
  dist/              release artifacts per os/arch - gitignored
```

## The paired-file convention

Every page and component is a folder holding up to three same-named
files:

- <name>.go - the logic half. A normal Go package exporting a ui.Page
  or ui.Component value.
- <name>.tsx - the look half. A normal React component as the default
  export.
- <name>.css - optional styles, automatically imported. No import
  statement needed.

The two halves find each other by key: the folder path relative to the
app root ("pages/index", "components/example"). The Go side writes it
in the Page/Component value; the tsx side never does - the Vite plugin
fills usePaired() in from the file's location.

Go tooling completely ignores .tsx and .css files, so each folder is
also just a normal Go package. Nothing about the pairing is magic at
runtime: it is one websocket carrying keyed messages.

## Pages vs components

- Pages are routable: pages/index serves "/", pages/settings serves
  "/settings" (override with Route in Go or export const route in tsx).
  With only pages/index the app is single-page and no router runs at
  all - add a second folder and routing switches on automatically.
- Components are reusable pieces. Import them like any React component
  (import Example from "../../components/example/example"), or render
  them from a Tea View with ui.Custom("components/example", nil).

Widget and popup windows are pages too - a widget is just a small
native window pointed at one of your routes. See
[Widgets](../shell/widgets.md).

## Registration

Automatic. gantry dev/build (or gantry gen) scan the pages/,
components/ and layouts/ folders for exported Page/Component vars and
regenerate gantry_registry.go; main.go just calls
`gantry.Run(gantry.Config{... Pairs: gantryPairs() ...})`. Add a pair,
run gantry dev, done. If a key ever misses (typo in the Key string),
the tsx side's send() logs "no handler for ..." in the Go terminal.

## gantry.json

Written by gantry new so dev/build do not re-ask:

```json
{
  "name": "myapp",
  "title": "Myapp",
  "port": 8330,
  "mode": "single",
  "style": "tea",
  "tray": true,
  "buttons": { "minimize": true, "maximize": false, "close": true }
}
```

name is the exe/module name, port is the local server the app binds
(also the single-instance guard), and the rest records your scaffold
choices. Note that changing buttons here does NOT reconfigure a built
app - the real switches live in main.go's WindowOptions; this file just
remembers what the scaffold generated.

## The synthesized .gantry/ folder

gantry dev and gantry build regenerate .gantry/ (index.html, main.tsx,
vite.config.ts) every run. Never edit those files - your changes would
be overwritten. Everything you would want to change lives in your own
files: theme in index.css, page layout in your tsx, window behavior in
main.go. If you outgrow the synthesis entirely, see
[Without the CLI](../advanced/without-the-cli.md).
