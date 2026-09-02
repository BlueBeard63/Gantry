// End-to-end protocol-plane tests for the demo app, driven by the
// gantrytest package (tier 1 of the testing plan). These cover what
// wstest/main.go used to smoke-test by hand: mounting pages, Tea
// clicks and re-renders, paired events, the call/reply cycle, shared
// state, and the error pipeline - against the real app process.
//
// Run with `gantry test` (prebuilds the frontend and app), or a bare
// `go test ./tests/...` after a gantry dev/build has filled webdist/.
package tests

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/BlueBeard63/Gantry/gantrytest"
)

func TestCounterClicks(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	tree := app.WaitTree("the counter render", func(n *gantrytest.Node) bool {
		return strings.Contains(n.Text(), "count is 0")
	})

	// Two clicks, each stepping the label; renders may coalesce, so
	// wait on the tree content rather than counting frames.
	for _, want := range []string{"count is 1", "count is 2"} {
		app.Click(tree.Find("button", gantrytest.Text("count is")))
		tree = app.WaitTree(want, func(n *gantrytest.Node) bool {
			return strings.Contains(n.Text(), want)
		})
	}

	app.ExpectNoErrors()
}

func TestPairedEventsSurviveUnknownKeys(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	// A real paired event and a bogus one: the server logs the miss
	// and must not drop the connection.
	app.SendEvent("components/example", "ping", 123)
	app.SendEvent("components/nope", "x", nil)

	// The connection still answers calls afterwards.
	var env struct {
		Mode string         `json:"mode"`
		Args map[string]any `json:"args"`
	}
	if err := json.Unmarshal(app.Call("gantry", "env", nil), &env); err != nil {
		t.Fatalf("decoding env reply: %v", err)
	}
	if env.Mode != "development" {
		t.Errorf("mode = %q, want development (the test default)", env.Mode)
	}
	app.ExpectNoErrors()
}

func TestCallPanicPipeline(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	// A panicking call handler rejects the call with the gerr code...
	cerr := app.CallFail("pages/debug", "callBoom", nil)
	if cerr.Code != "panic.call" {
		t.Errorf("reply code = %q, want panic.call", cerr.Code)
	}

	// ...and pushes an error frame carrying the page and the trail.
	e := app.WaitError("panic.call")
	if e.Kind != "call-panic" {
		t.Errorf("error kind = %q, want call-panic", e.Kind)
	}
	if e.Page != "pages/index" {
		t.Errorf("error page = %q, want pages/index", e.Page)
	}
	if len(e.Trail) == 0 {
		t.Error("expected a non-empty breadcrumb trail on the error")
	}

	// The connection survived the panic; a normal call still answers.
	app.Call("gantry", "env", nil)
	app.ExpectNoErrors() // the panic.call was claimed by WaitError above
}

func TestEventPanicPipeline(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	app.SendEvent("pages/debug", "eventBoom", nil)
	e := app.WaitError("panic.event")
	if e.Kind != "event-panic" {
		t.Errorf("error kind = %q, want event-panic", e.Kind)
	}
	if len(e.Trail) == 0 {
		t.Error("expected a non-empty breadcrumb trail on the error")
	}
}

// teaTicks reads the debug page's Tea counter out of the newest render.
func teaTicks(t *testing.T, n *gantrytest.Node) int {
	t.Helper()
	txt := n.Text()
	i := strings.Index(txt, "tea ticks: ")
	if i < 0 {
		t.Fatalf("no tea ticks counter in the render: %q", txt)
	}
	var ticks int
	if _, err := fmt.Sscanf(txt[i:], "tea ticks: %d", &ticks); err != nil {
		t.Fatalf("parsing the tea ticks counter from %q: %v", txt[i:], err)
	}
	return ticks
}

// readyDebug mounts the Tea half of the debug page and returns its
// first render.
func readyDebug(t *testing.T, app *gantrytest.App) *gantrytest.Node {
	t.Helper()
	app.Ready("pages/debug")
	return app.WaitTree("the debug page's first render", func(n *gantrytest.Node) bool {
		return n.Find("button", gantrytest.Text("Tea tick")) != nil
	})
}

func TestUpdatePanicKeepsLastGoodModel(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	tree := readyDebug(t, app)

	// One benign tick, so there is a good state to preserve.
	app.Click(tree.Find("button", gantrytest.Text("Tea tick")))
	tree = app.WaitTree("the tick to land", func(n *gantrytest.Node) bool {
		return teaTicks(t, n) == 1
	})

	// The Update panic discards its model...
	app.Click(tree.Find("button", gantrytest.Text("Tea update panic")))
	e := app.WaitError("panic.update")
	if e.Kind != "tea-update-panic" {
		t.Errorf("error kind = %q, want tea-update-panic", e.Kind)
	}

	// ...so the next tick steps 1 -> 2, not 2 -> 3: the panicking
	// Update's increment never took.
	app.Click(app.Tree().Find("button", gantrytest.Text("Tea tick")))
	app.WaitTree("the tick after the update panic", func(n *gantrytest.Node) bool {
		return teaTicks(t, n) == 2
	})
	app.ExpectNoErrors() // the panic.update was claimed above
}

func TestCmdPanicIsRecovered(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	tree := readyDebug(t, app)

	app.Click(tree.Find("button", gantrytest.Text("Tea cmd panic")))
	e := app.WaitError("panic.cmd")
	if e.Kind != "cmd-panic" {
		t.Errorf("error kind = %q, want cmd-panic", e.Kind)
	}

	// The command goroutine died, but the Tea loop did not: the page
	// still folds messages and renders.
	app.Click(app.Tree().Find("button", gantrytest.Text("Tea tick")))
	app.WaitTree("a tick after the cmd panic", func(n *gantrytest.Node) bool {
		return teaTicks(t, n) == 2 // the cmd panic's own click ticked first
	})
	app.ExpectNoErrors()
}

func TestViewPanicKeepsLastGoodRender(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	tree := readyDebug(t, app)

	app.Click(tree.Find("button", gantrytest.Text("Tea view panic")))
	e := app.WaitError("panic.view")
	if e.Kind != "tea-view-panic" {
		t.Errorf("error kind = %q, want tea-view-panic", e.Kind)
	}

	// Every render panics from here, so no new tree is delivered - the
	// last good one is still what the frontend holds.
	if got := teaTicks(t, app.Tree()); got != 0 {
		t.Errorf("tea ticks = %d, want the last good render's 0 (a broken View must not deliver a tree)", got)
	}
	app.ExpectNoErrors()
}

func TestGoroutinePanicIsRecovered(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	// gantry.Go captures the panic instead of killing the app.
	app.SendEvent("pages/debug", "goroutineBoom", nil)
	e := app.WaitError("panic.goroutine")
	if e.Kind != "goroutine-panic" {
		t.Errorf("error kind = %q, want goroutine-panic", e.Kind)
	}
	if e.Stack == "" {
		t.Error("expected a stack on the goroutine panic")
	}

	// The app is still alive and answering.
	app.Call("gantry", "env", nil)
	app.ExpectNoErrors()
}

func TestProcessCrashReportedOnNextLaunch(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	// A panic on a plain goroutine is uncatchable: the process dies and
	// the runtime's crash output leaves the trace in crash.log.
	app.SendEvent("pages/debug", "fatalBoom", nil)
	app.WaitExit()

	// The next launch (same config dir, so the same crash.log) reports
	// the waiting trace through the normal error pipeline.
	app.Restart()
	e := app.WaitError("panic.fatal")
	if e.Kind != "process-crash" {
		t.Errorf("error kind = %q, want process-crash", e.Kind)
	}
	if !strings.Contains(e.Stack, "fatalBoom") {
		t.Errorf("crash stack does not mention the panic:\n%s", e.Stack)
	}
	if len(e.Trail) != 0 {
		t.Errorf("trail = %v, want empty (the process died before it could be snapshotted)", e.Trail)
	}
	app.ExpectNoErrors()
}

func TestDeclaredArgsReachTheApp(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t, gantrytest.WithArgs(map[string]any{
		"mock-data": true,
		"api-host":  "example.test",
	}))
	app.Ready("pages/index")

	var users []string
	if err := json.Unmarshal(app.Call("pages/debug", "slowUsers", nil), &users); err != nil {
		t.Fatalf("decoding slowUsers reply: %v", err)
	}
	if len(users) == 0 || !strings.HasPrefix(users[0], "mock-") {
		t.Errorf("users = %v, want the mock-data set", users)
	}
	app.ExpectNoErrors()
}

func TestProductionMode(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t, gantrytest.WithMode("production"))
	app.Ready("pages/index")

	var env struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(app.Call("gantry", "env", nil), &env); err != nil {
		t.Fatalf("decoding env reply: %v", err)
	}
	if env.Mode != "production" {
		t.Errorf("mode = %q, want production", env.Mode)
	}
	app.ExpectNoErrors()
}

func TestSharedStateSnapshotAndWrite(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	// The declared state arrives in the connect snapshot.
	if got := app.State("volume").Float(); got != 0.5 {
		t.Errorf("volume = %v, want the declared default 0.5", got)
	}

	// A frontend-style write is accepted (OnChange logs on the Go
	// side) and must not produce errors.
	app.SetState("volume", 0.25)
	if got := app.State("volume").Float(); got != 0.25 {
		t.Errorf("volume after SetState = %v, want 0.25", got)
	}
	app.ExpectNoErrors()
}

func TestAppInfoVersionStamp(t *testing.T) {
	t.Parallel()
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	var info struct {
		Name    string `json:"name"`
		Title   string `json:"title"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(app.Call("gantry", "appInfo", nil), &info); err != nil {
		t.Fatalf("decoding appInfo reply: %v", err)
	}
	if info.Name != "demo" || info.Title != "Demo" {
		t.Errorf("appInfo = %+v, want name demo / title Demo", info)
	}
	if info.Version != "0.1.0" {
		t.Errorf("version = %q, want the gantry.json stamp 0.1.0", info.Version)
	}
}
