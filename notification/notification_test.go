package notification

import (
	"strings"
	"testing"
)

// asAndroid captures the control lines Post/Clear would write to the
// shell on-device.
func asAndroid(t *testing.T) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	oldOut, oldAndroid := out, android
	out, android = &buf, true
	t.Cleanup(func() { out, android = oldOut, oldAndroid })
	return &buf
}

// TestControlLines is the wire-format golden: the generated Kotlin
// shell parses exactly these verbs and keys.
func TestControlLines(t *testing.T) {
	buf := asAndroid(t)

	err := Post(Notification{
		ID:      "steep",
		Title:   "Tea's ready",
		Body:    "4 minutes are up.",
		Silent:  true,
		Actions: []Action{{ID: "again", Label: "Steep again"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	Clear("steep")
	ClearAll()

	want := `GANTRY_NOTIFY {"id":"steep","title":"Tea's ready","body":"4 minutes are up.","silent":true,"actions":[{"id":"again","label":"Steep again"}]}` + "\n" +
		`GANTRY_NOTIFY_CLEAR {"id":"steep"}` + "\n" +
		`GANTRY_NOTIFY_CLEAR {}` + "\n"
	if buf.String() != want {
		t.Errorf("wire format changed:\n got %q\nwant %q", buf.String(), want)
	}
}

func TestPostValidation(t *testing.T) {
	buf := asAndroid(t)
	if err := Post(Notification{Title: "no id"}); err == nil {
		t.Error("Post without ID should fail")
	}
	if err := Post(Notification{ID: "x"}); err == nil {
		t.Error("Post without Title should fail")
	}
	if buf.Len() != 0 {
		t.Errorf("invalid posts must not emit, got %q", buf.String())
	}
}

func TestDispatch(t *testing.T) {
	var gotN, gotA string
	OnAction(func(n, a string) { gotN, gotA = n, a })
	t.Cleanup(func() { OnAction(nil) })
	Dispatch("steep", "again")
	if gotN != "steep" || gotA != "again" {
		t.Errorf("Dispatch = %q/%q", gotN, gotA)
	}
}
