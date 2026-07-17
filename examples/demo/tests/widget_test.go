package tests

import (
	"encoding/json"
	"testing"

	"github.com/B-Commissions/Gantry/gantrytest"
	"github.com/B-Commissions/Gantry/widget"
)

// The status widget renders the current time, so its snapshot cannot
// be golden-filed byte-for-byte - assert on the stable structure
// instead. Widgets with stable output would use gantrytest.Golden.
func TestStatusWidgetSnapshot(t *testing.T) {
	t.Parallel()
	raw := gantrytest.WidgetSnapshot(t, "status")

	var root widget.Node
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("decoding widget root: %v", err)
	}
	if root.Type != "column" {
		t.Errorf("root type = %q, want column", root.Type)
	}
	if findWidgetNode(root, "progress") == nil {
		t.Error("expected a progress node in the status widget")
	}
	if findWidgetNode(root, "text") == nil {
		t.Error("expected text nodes in the status widget")
	}
}

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
