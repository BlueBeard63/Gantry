# Home-screen widgets

Android home-screen widgets are declared once in `gantry.json` and drawn by a small Go render function - no Kotlin, no XML. Each widget pairs a `mobile.widgets` entry (launcher metadata) with a `widgets/<name>/<name>.go` file (content):

```json
"mobile": {
  "id": "ec.morrison.myapp",
  "widgets": [
    { "name": "status", "label": "My Status", "minSize": "3x1", "maxSize": "5x2", "refreshMinutes": 30 }
  ]
}
```

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

Like pages, widgets register themselves: `gantry dev/build/gen` scans `widgets/` and generates `gantry_widgets.go` - `main.go` never changes. A `gantry.json` entry without its Go pair is a build error; a Go pair without an entry works with defaults (label from the name, `2x1`, resizable, 30 minutes).

## The node tree

Render functions return a small declarative tree; the generated Kotlin shell draws it with Glance, so every gantry widget shares one look (dark rounded card, consistent type):

| Builder | What it is |
| --- | --- |
| `Column(...)` / `Row(...)` | vertical / horizontal stack |
| `Text(s)` / `Textf(f, ...)` | a text line |
| `Progress(v)` | horizontal bar, `v` clamped 0..1 |
| `Spacer()` | flexible whitespace - pushes siblings apart |
| `Divider()` | thin horizontal rule |

Chainable props on any node: `.Size(sp)`, `.Color("#rrggbb")`, `.Bold()`, `.Dim()`, `.MaxLines(n)`.

Widgets have no handlers in v1 - tapping a widget opens the app.

## Render functions are on their own

Widget refresh runs your binary with `--emit-widgets`: it renders every widget to JSON on stdout and exits **without starting the app**. No server, no pages, no live state. Read persisted state instead - files you write under `$HOME` (the app's private files dir on Android) are there for both the app and the widget refresh:

```go
var Widget = widget.New("count", func() widget.Node {
	data, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), "count.json")) // written by the app
	...
})
```

## When widgets refresh

- **App open**: every time the on-device server starts (or restarts), the shell pulls `/gantry/widgets.json` from it - widgets are hot right after you use the app.
- **Background**: a WorkManager job runs `--emit-widgets` on the smallest configured `refreshMinutes` across your widgets (Android floors this at 15 minutes; smaller values are raised, not errors).
- The widget also re-reads its cached JSON whenever the launcher re-composes it (resize, reboot).

## Launcher metadata

`minSize`/`maxSize` are launcher grid cells as `CxR` (converted to dp with Android's `70*cells - 30` rule), `resize` is `none`, `horizontal`, `vertical` or `horizontal|vertical`. Names must be `[a-z][a-z0-9_]*` - they become Kotlin classes and Android resource names.

Try it on the demo: `cd examples/demo && gantry mobile dev android`, then long-press the launcher > Widgets > Demo and place "Demo Status".
