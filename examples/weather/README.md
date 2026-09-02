# Lazy Weather

A **mobile** weather app built with [Gantry](https://github.com/BlueBeard63/Gantry). Instead of raw numbers it frames today against yesterday - "Today's weather is ABOUT THE SAME as yesterday" - with a per-time-of-day delta and trend arrow on each forecast row. Data comes from [Open-Meteo](https://open-meteo.com) (free, no API key).

It's the example for Gantry's **server-side** surface: a Go service that fetches an external API, shared state synced to React, `Await`/`Skeleton` loading, and on-device persistence.

## How it's built

- **All the logic is in Go** (`weather.go`). A `weather` service registered in `Config.Setup` exposes two calls:
  - `search(name)` - Open-Meteo geocoding, for the Add Location screen.
  - `forecast(lat, lon, units)` - fetches today + yesterday (`past_days=1`) and computes the "lazy" view (WMO code → sun/cloud/rain icon, today-minus-yesterday deltas and trends, the WARMER/COOLER/ABOUT-THE-SAME headline).
- **The screens are plain React** (`pages/*.tsx`, tsx-only - no Go half). They read the forecast with `useCall` wrapped in `<Await fallback={<Skeleton/>}>`, and share the chosen place + unit through `useGoState` ⇄ `ui.NewState`.
- **Settings persist.** `location` and `units` are saved to `~/.lazy-weather/settings.json` on every change (via the state `OnChange` observers) and restored on launch. On Android that path is the app's private files dir, so no storage permission is needed (`settings.go`).
- **Icons** are inline SVG (`lib/icons.tsx`); the black/monospace look is plain CSS scoped per page.

## Screens

- `pages/index` - Home: location + settings gear, the weather statement, the Morning/Noon/Evening/Night forecast.
- `pages/settings` - place, country, °C/°F unit pills, Save.
- `pages/search` - live geocoding search and result list (this build has no GPS; "Use current location" falls back to the default city).

## Run it

- `gantry dev` - live reload in a phone-sized native window (logs in this terminal).
- `gantry mobile dev android` - build and run on a plugged-in, **unlocked** Android device (needs the `mobile.id` in `gantry.json`).
- `go test -run TestBuildForecast|TestSearch` - live smoke tests of the weather service against Open-Meteo (skip under `-short` and when offline).
