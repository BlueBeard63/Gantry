// DOM-plane tests for the demo app (tier 2 of the testing plan): the
// driver attaches to the real WebView2 window over CDP and drives what
// the user actually sees - real clicks, real typing - while the
// protocol plane watches the same app from the Go side. WithDOM skips
// these tests off-Windows, so the suite stays runnable everywhere.
package tests

import (
	"strings"
	"testing"

	"github.com/BlueBeard63/Gantry/gantrytest"
)

func TestDOMCounterClicks(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	page := app.Page("/")

	// The Tea counter, driven through the real UI: the DOM click goes
	// webview -> websocket -> Tea Update -> render -> DOM.
	btn := page.Find("button", gantrytest.Text("count is"))
	if got := btn.Text(); !strings.Contains(got, "count is 0") {
		t.Errorf("counter starts at %q, want count is 0", got)
	}
	btn.Click()
	page.Find("button", gantrytest.Text("count is 1")) // auto-waits

	// The other plane saw the same click: the newest render's tree.
	app.WaitTree("the protocol plane to see count 1", func(n *gantrytest.Node) bool {
		return strings.Contains(n.Text(), "count is 1")
	})
	app.ExpectNoErrors()
}

func TestDOMSettingsTypingAndSharedState(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	page := app.Page("/settings")

	// Real per-character key events into the React-controlled input.
	name := page.Find("input")
	name.Type("Jack")
	if got := name.Value(); got != "Jack" {
		t.Errorf("input value = %q after typing, want Jack", got)
	}

	// Go -> DOM: a protocol-plane state write re-renders the page live.
	app.SetState("volume", 0.25)
	page.Find("label", gantrytest.Text("25%"))

	// DOM -> Go: the volume slider's value round-trips into shared
	// state when clicked (a real mouse click on the range track).
	page.Find("input[type=range]").Click() // center of the track = 0.5
	if got := app.State("volume").Float(); got < 0.4 || got > 0.6 {
		t.Errorf("volume after clicking the slider center = %v, want ~0.5", got)
	}

	page.Find("button", gantrytest.Text("Save")).Click()
	app.ExpectNoErrors()
}

func TestDOMErrorBannerAndScreenshot(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	app.Page("/debug").Find("button", gantrytest.Text("Go call panic")).Click()

	// The protocol plane delivers the typed error...
	e := app.WaitError("panic.call")
	if e.Page != "pages/debug" {
		t.Errorf("error page = %q, want pages/debug", e.Page)
	}
	if len(e.Trail) == 0 {
		t.Error("expected a breadcrumb trail on the error")
	}

	// ...and the UI actually showed it (chrome-level, so App.Find).
	app.Find(".gantry-error-banner")
	app.Screenshot("banner")
	app.ExpectNoErrors() // the panic.call was claimed by WaitError
}

func TestDOMJSErrorReachesGo(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	app.Page("/debug").Find("button", gantrytest.Text("JS error")).Click()

	// An uncaught frontend error crosses to the Go side of the
	// pipeline (call("gantry","reportError")) and is queryable there.
	e := app.WaitError("js.error")
	if e.Kind != "js-error" {
		t.Errorf("error kind = %q, want js-error", e.Kind)
	}
	app.Find(".gantry-error-banner")
	app.ExpectNoErrors()
}

func TestDOMCapabilities(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	if !app.Supports(gantrytest.DOM) {
		t.Error("a DOM-plane launch must report the DOM capability")
	}
	if gantrytest.Target() == "desktop" {
		// Desktop drives via mouse: hover, no touch.
		if !app.Supports(gantrytest.Hover) {
			t.Error("a desktop DOM launch must report Hover")
		}
		if app.Supports(gantrytest.Touch) {
			t.Error("a desktop launch must not report Touch")
		}
	} else {
		// A phone drives via real taps: touch, no hover.
		if !app.Supports(gantrytest.Touch) {
			t.Error("a device DOM launch must report Touch")
		}
		if app.Supports(gantrytest.Hover) {
			t.Error("a device launch must not report Hover")
		}
	}
}
