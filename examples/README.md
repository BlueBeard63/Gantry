# Examples

Each folder is a complete Gantry app, generated with the CLI and then
extended. Build any of them with `gantry build` inside its folder (or
run live with `gantry dev`).

## hello

The smallest possible app: one page (plain React style), no tray,
minimize/close buttons only. Exactly what `gantry new hello --single
--plain --no-tray --buttons=minimize,close` produces - a good diff base
for "what did the scaffold give me".

## demo

The showcase - multi-page, Tea-style, tray, all three window buttons,
then extended by hand to demonstrate:

- app.tsx: top bar customization - title on the left, back/forward
  buttons in the left slot (with the matching CaptionLeftReserve in
  main.go)
- layouts/main: a navbar with active-aware Links
- pages/index: a Tea Model page (the counter lives in Go)
- pages/settings: plain React + usePaired events + a useGoState
  slider bound to a Go-side ui.NewState (watch the dev terminal while
  dragging)
- components/example: a paired component used from another page
- main.go: gantry.Run with Window and Setup hooks
- wstest/: a wire-protocol smoke test (`go run ./wstest` against
  `demo.exe --no-open --port 8331`)

## weather

Lazy Weather - a **mobile** app that fetches [Open-Meteo](https://open-meteo.com)
and frames today's forecast against yesterday. The example for Gantry's
server-side surface, extended by hand to demonstrate:

- weather.go: a `weather` service (app.Service in Setup) that calls an
  external API and does the "today vs yesterday" computation in Go
- pages/index, settings, search: plain React screens (tsx-only) using
  `useCall` + `Await`/`Skeleton` for loading and `useGoState` for the
  shared location + unit
- settings.go: on-device persistence (`~/.lazy-weather/settings.json`),
  saved from the state OnChange observers
- gantry.json: a `mobile` block - run with `gantry mobile dev android`
