package widget

import (
	"encoding/json"
	"testing"
)

// TestNodeJSON is the wire-format golden: the Kotlin renderer parses
// exactly these keys, so a marshalling change here is a breaking
// change there.
func TestNodeJSON(t *testing.T) {
	root := Column(
		Row(
			Text("Demo").Bold().Size(14),
			Spacer(),
			Text("later").Dim().MaxLines(1),
		),
		Divider(),
		Progress(0.25).Color("#4ade80"),
	)
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"column","children":[` +
		`{"type":"row","children":[` +
		`{"type":"text","text":"Demo","size":14,"bold":true},` +
		`{"type":"spacer"},` +
		`{"type":"text","text":"later","dim":true,"maxLines":1}]},` +
		`{"type":"divider"},` +
		`{"type":"progress","value":0.25,"color":"#4ade80"}]}`
	if string(data) != want {
		t.Errorf("wire format changed:\n got %s\nwant %s", data, want)
	}
}

func TestProgressClamped(t *testing.T) {
	if v := Progress(1.7).Value; v != 1 {
		t.Errorf("Progress(1.7) = %v, want clamped 1", v)
	}
	if v := Progress(-3).Value; v != 0 {
		t.Errorf("Progress(-3) = %v, want clamped 0", v)
	}
}

func TestTextf(t *testing.T) {
	n := Textf("%d%% done", 40)
	if n.Text != "40% done" || n.Type != "text" {
		t.Errorf("Textf = %+v", n)
	}
}
