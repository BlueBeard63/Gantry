// The Go half of the debug page: exercises the arg harness, loading
// states (a deliberately slow call) and the crash pipeline. Every
// handler and every Tea message below panics somewhere different on
// purpose, so each error kind in docs/advanced/errors.md has one place
// that produces it - watch them in the UI, or see tests/protocol_test.go
// where each one is asserted end to end.
package debug

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/B-Commissions/Gantry/gantry"
	"github.com/B-Commissions/Gantry/ui"
)

var Page = ui.Page{
	Key:   "pages/debug",
	Model: func() ui.Model { return crashModel{} },
	On: ui.Handlers{
		// A paired event handler that panics: shows up as an
		// event-panic notice with the action trail.
		"eventBoom": func(json.RawMessage) {
			panic("eventBoom: deliberate panic from a paired event handler")
		},
		// A panic on a gantry.Go goroutine: recovered into the error
		// pipeline as "panic.goroutine" - the app survives.
		"goroutineBoom": func(json.RawMessage) {
			gantry.Go(func() {
				panic("goroutineBoom: deliberate panic on a gantry.Go goroutine")
			})
		},
		// A panic on a plain goroutine: uncatchable, so it kills the
		// process. The trace lands in crash.log and the *next* launch
		// reports it as "panic.fatal". Fire-and-forget by design - a
		// call could never reply, the process is gone.
		"fatalBoom": func(json.RawMessage) {
			go func() {
				panic("fatalBoom: deliberate panic on a plain goroutine")
			}()
		},
	},
	Call: ui.Calls{
		// Slow on purpose so <Await fallback={<Skeleton/>}> is visible.
		"slowUsers": func(json.RawMessage) (any, error) {
			time.Sleep(1500 * time.Millisecond)
			if gantry.ArgBool("mock-data") {
				return []string{"mock-alice", "mock-bob", "mock-carol"}, nil
			}
			return []string{
				"alice@" + gantry.Arg("api-host"),
				"bob@" + gantry.Arg("api-host"),
			}, nil
		},
		// A call handler that panics: the awaiting promise rejects with
		// code "panic.call" and a notice banner appears.
		"callBoom": func(json.RawMessage) (any, error) {
			panic(fmt.Sprintf("callBoom: deliberate panic (mode=%s)", gantry.Mode()))
		},
	},
}

// crashModel is the Tea half of the debug page, rendered by the
// <TeaView /> in debug.tsx. Its ticks counter is the evidence for what
// each Tea-loop panic is supposed to preserve: an Update panic keeps
// the last good model, a View panic keeps the last good render.
type crashModel struct {
	ticks    int
	viewBoom bool
}

type (
	tickMsg       struct{} // benign: the control case
	updateBoomMsg struct{}
	cmdBoomMsg    struct{}
	viewBoomMsg   struct{}
)

func (m crashModel) Init() ui.Cmd { return nil }

func (m crashModel) Update(msg ui.Msg) (ui.Model, ui.Cmd) {
	m.ticks++
	switch msg.(type) {
	case updateBoomMsg:
		// The panic discards this model, ticks included: the page keeps
		// the state it had before the click.
		panic("updateBoom: deliberate panic from a Model's Update")
	case cmdBoomMsg:
		return m, func() ui.Msg {
			panic("cmdBoom: deliberate panic from a Tea command goroutine")
		}
	case viewBoomMsg:
		m.viewBoom = true
	}
	return m, nil
}

func (m crashModel) View() ui.Node {
	if m.viewBoom {
		// Every render from here on panics; the last good tree stays on
		// screen and a notice banner appears over it.
		panic("viewBoom: deliberate panic from a Model's View")
	}
	return ui.Column(
		ui.Textf("tea ticks: %d", m.ticks),
		ui.Button("Tea tick", tickMsg{}),
		ui.Button("Tea update panic", updateBoomMsg{}),
		ui.Button("Tea cmd panic", cmdBoomMsg{}),
		ui.Button("Tea view panic", viewBoomMsg{}),
	).WithProps("class", "tea-crash")
}
