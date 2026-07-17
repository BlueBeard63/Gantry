# Testing: widget snapshots

[Home-screen widgets](../mobile/widgets.md) render via `--emit-widgets`: the app binary prints every widget's tree as JSON and exits, no server, no device. That makes widget testing host-side and cheap - it belongs in the PR gate, not on an emulator.

## Snapshot assertions

```go
func TestStatusWidget(t *testing.T) {
	raw := gantrytest.WidgetSnapshot(t, "status") // runs the binary with --emit-widgets

	var root widget.Node
	json.Unmarshal(raw, &root)
	if root.Type != "column" { t.Errorf(...) }
}
```

`WidgetSnapshot` takes the same options as `Launch` where they make sense - `WithArgs`, `WithMode`, `WithEnv` - since widget render functions read the same declared-arg environment.

## Golden files

For widgets whose output is stable, compare against a golden file instead of hand-written assertions:

```go
gantrytest.Golden(t, "status", gantrytest.WidgetSnapshot(t, "status"))
```

The golden lives at `tests/testdata/status.golden.json` (normalized, indented JSON, so diffs read well). Create or refresh it with:

```
gantry test --update
```

Only golden stable output: a widget that renders the clock will never match. For time-dependent widgets, assert on structure (the node types and the props that do not move) like the snapshot example above, or inject determinism via a declared arg.

Remember the constraint from the widgets doc: render functions run in a short-lived process without the app loop, so they must be self-contained - persisted files, not live app state. Snapshot tests enforce this for free, because they run exactly that way.

On-device Glance rendering (does the launcher actually draw it right) is screenshot territory, scoped to manual/nightly review - see [mobile testing](mobile.md).
