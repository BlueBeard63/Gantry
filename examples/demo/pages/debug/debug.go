// The Go half of the debug page: exercises the arg harness, loading
// states (a deliberately slow call) and the crash pipeline (calls and
// events that panic on purpose).
package debug

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/B-Commissions/Gantry/gantry"
	"github.com/B-Commissions/Gantry/ui"
)

var Page = ui.Page{
	Key: "pages/debug",
	On: ui.Handlers{
		// A paired event handler that panics: shows up as an
		// event-panic notice with the action trail.
		"eventBoom": func(json.RawMessage) {
			panic("eventBoom: deliberate panic from a paired event handler")
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
