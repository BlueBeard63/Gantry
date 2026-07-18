# Home-screen widgets

Android home-screen widgets are declared once in `gantry.json` and drawn by a small Go render function - no Kotlin, no XML. The generated shell renders the tree with Jetpack Glance, so every gantry widget shares one look. This page is only about home-screen widgets; building and running the app itself is [Android builds](android.md).

## Add a widget

Each widget pairs a `mobile.widgets` entry (launcher metadata) with a `widgets/<name>/<name>.go` file (its content). Declare the entry:

```json
"mobile": {
  "id": "ec.morrison.myapp",
  "widgets": [
    { "name": "status", "label": "My Status", "minSize": "3x1", "maxSize": "5x2", "refreshMinutes": 30 }
  ]
}
```

Then write the render function - it returns a small declarative tree:

```go
// widgets/status/status.go
package status

import "github.com/B-Commissions/Gantry/widget"

var Widget = widget.New("status", func() widget.Node {
	return widget.Column(
		widget.Row(
			widget.Text("My App").Bold().Size(14),
			widget.Spacer(),
			widget.Text("just now").Dim().Size(11),
		),
		widget.Spacer(),
		widget.Progress(0.4).Color("#4ade80"),
	)
})
```

Like pages, widgets register themselves. `gantry dev`/`build`/`gen` scans `widgets/` for directories holding a `.go` file with an exported `var Widget = ...` and generates `gantry_widgets.go`, which `init()`-registers each one - `main.go` never changes. Naming rules: the entry `name`, the directory `widgets/<name>/`, and the widget passed to `widget.New(...)` must all match. A `gantry.json` entry without its Go pair is a build error (`widget "status" has no paired widgets/status/status.go`); a Go pair without an entry works with defaults.

Try it on the demo: `cd examples/demo && gantry mobile dev android`, then long-press the launcher > Widgets > Demo and place "Demo Status".

## The node tree

Render functions return a small declarative tree of `widget.Node` values. Builders:

| Builder | What it is |
| --- | --- |
| `Column(children ...Node)` | vertical stack |
| `Row(children ...Node)` | horizontal stack (children centred vertically) |
| `Text(s string)` | a text line |
| `Textf(format string, a ...any)` | `Text` with `Sprintf` formatting |
| `Progress(v float64)` | horizontal bar; `v` is clamped to 0..1 |
| `Spacer()` | flexible whitespace - inside a Column/Row it expands to push siblings apart; on its own it is a fixed 8dp gap |
| `Divider()` | a thin 1dp horizontal rule |

Every node has chainable props (each returns a new `Node`, so order does not matter):

| Prop | Effect |
| --- | --- |
| `.Size(sp int)` | text size in sp (default 13) |
| `.Color("#rrggbb")` | text colour, or the bar's fill colour on `Progress` |
| `.Bold()` | bold text |
| `.Dim()` | muted foreground colour |
| `.MaxLines(n int)` | truncate (ellipsize) after n lines |

The tree is serialized to JSON and handed to a single generic Glance renderer, so the palette is fixed in the shell, not per app: a rounded card (`#111111` background, 20dp corners, 12dp padding), `#EDEDED` foreground, `#8F8F8F` for dim text and the progress track. `.Color(...)` overrides the foreground on a `Text` or the fill on a `Progress`. Widgets have no handlers in v1 - the whole card is tappable and opens the app. A widget whose cached data is missing shows "Open the app to fill this widget" until the first refresh.

## Launcher metadata

The `mobile.widgets` entry is pure launcher metadata; defaults fill in anything you omit:

| Field | Default | Notes |
| --- | --- | --- |
| `name` | (required) | must match `[a-z][a-z0-9_]*` - it becomes a Kotlin class (`StatusWidget`) and Android resource names |
| `label` | title-cased `name` | the name the launcher's widget picker shows |
| `minSize` | `2x1` | launcher grid cells as `CxR` |
| `maxSize` | none | `CxR`; sets `maxResizeWidth/Height`, omit for no cap |
| `resize` | `horizontal\|vertical` | `none`, `horizontal`, `vertical`, or `horizontal\|vertical` |
| `refreshMinutes` | `30` | background refresh cadence; see below |

Grid cells convert to dp with Android's `70*cells - 30` rule, so `3x1` becomes `minWidth=180dp`, `minHeight=40dp`. Each widget renders its own `res/xml/widget_<name>_info.xml` provider and a `<ClassName>Receiver` in the manifest, so removing a widget from `gantry.json` cleanly drops its Kotlin and resources on the next build.

## Notes (advanced)

### Render functions run in isolation

Widget refresh runs your binary with `--emit-widgets`: it renders every registered widget to a JSON envelope on stdout and exits **without starting the app**. No server, no pages, no live state. Read persisted state instead - files you write under `$HOME` (the app's private files dir on Android, set for both the app process and the widget refresh) are the shared channel:

```go
var Widget = widget.New("count", func() widget.Node {
	data, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), "count.json")) // written by the app
	...
})
```

The envelope is versioned - `{"version": 1, "widgets": [{"name": "...", "root": {...}}]}` - and the exact same shape is served from the running app at `/gantry/widgets.json`, which is how the shell refreshes for free while the app is open.

### When widgets refresh

- **App open**: every time the on-device server starts (or restarts) and announces its port, the shell pulls `/gantry/widgets.json` from it - widgets are hot right after you use the app.
- **Background**: one WorkManager periodic job runs `--emit-widgets` on the *smallest* configured `refreshMinutes` across all your widgets. Android floors periodic work at 15 minutes, so `collectWidgets` raises anything below 15 to 15 (it is not an error) - a `refreshMinutes` of 5 effectively becomes 15.
- **Recompose**: the launcher re-reads each widget's cached JSON whenever it recomposes the widget (resize, reboot), with no process spawn.

Because the background path is a short-lived process, a slow or crashing render function just fails that refresh (the worker retries) - it never blocks the app.
