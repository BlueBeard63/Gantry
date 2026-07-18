# Testing: widget snapshots

[Home-screen widgets](../mobile/widgets.md) render via `--emit-widgets`: the app binary prints every widget's tree as JSON and exits, no server, no device. That makes widget testing host-side and cheap - it runs the same host binary `Launch` builds, so it belongs in the PR gate, not on an emulator.

## Snapshot assertions

`gantrytest.WidgetSnapshot(t, name)` runs the app binary with `--emit-widgets`, parses the emitted envelope, and returns the named widget's rendered `root` tree as raw JSON. Unmarshal it into a `widget.Node` and assert on the structure.

```go
func TestStatusWidget(t *testing.T) {
	t.Parallel()
	raw := gantrytest.WidgetSnapshot(t, "status") // runs the binary once with --emit-widgets

	var root widget.Node
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("decoding widget root: %v", err)
	}
	if root.Type != "column" {
		t.Errorf("root type = %q, want column", root.Type)
	}
}
```

A `widget.Node` is the same tree the Kotlin renderer draws with Glance: `Type` (`column`, `row`, `text`, `progress`, `spacer`, `divider`), `Text`, `Value` (a progress fraction), `Children`, and the styling fields (`IsBold`, `ColorHex`, `SizeSp`, ...). A small recursive walk finds a node by type deep in the tree, which is the usual way to assert "this widget contains a progress bar and some text":

```go
func findWidgetNode(n widget.Node, typ string) *widget.Node {
	if n.Type == typ {
		return &n
	}
	for i := range n.Children {
		if found := findWidgetNode(n.Children[i], typ); found != nil {
			return found
		}
	}
	return nil
}
```

`WidgetSnapshot` names the widgets it found when the one you asked for is missing (`no widget "status" in the snapshot (have [...])`), so a renamed widget fails loudly. It takes the same options as `Launch` where they make sense - `WithArgs`, `WithMode`, `WithEnv`, `WithBinary`, `WithAppDir` - since widget render functions read the same declared-arg environment; the process-level options (`WithDOM`, `WithHeaded`, `WithRecording`) simply do not apply.

## Golden files

For widgets whose output is stable, compare against a golden file instead of hand-written assertions:

```go
gantrytest.Golden(t, "status", gantrytest.WidgetSnapshot(t, "status"))
```

`Golden` normalizes the JSON (unmarshal + re-marshal indented, so field ordering and whitespace never cause spurious diffs) and compares it byte-for-byte against `testdata/<name>.golden.json`, resolved relative to the test file - so it lives at `tests/testdata/status.golden.json`. A mismatch fails with both the got and want JSON and the fix. Create or refresh the file with `gantry test --update` (which sets `GANTRY_UPDATE_GOLDENS=1`; `Golden` then writes the normalized snapshot instead of comparing). If the golden file does not exist yet, the test fails pointing you at `--update` to create it.

Only golden *stable* output. The demo's `status` widget renders the current time, so it can never match a fixed golden - it uses the structural assertions above instead. For a time-dependent widget, assert on the node types and the props that do not move, or inject determinism through a declared arg (`WithArgs`) that pins the value the render function reads.

## Notes

Remember the constraint from the [widgets doc](../mobile/widgets.md): render functions run in a short-lived process without the app loop, so they must be self-contained - they read persisted files (under `$HOME`), never live app state. Snapshot tests enforce this for free, because `--emit-widgets` runs exactly that way: a render that reaches for the app loop simply has nothing there.

On-device Glance rendering - does the launcher actually draw the tree right - is screenshot territory, scoped to manual/nightly review, not this host-side gate. See [mobile testing](mobile.md#what-does-not-transfer-yet).

Next: [mobile testing](mobile.md).
