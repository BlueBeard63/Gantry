// The Go half of the index page: handlers for events the paired
// index.tsx sends with usePaired().send(name, payload).
package index

import (
	"encoding/json"
	"log"

	"github.com/BlueBeard63/Gantry/ui"
)

var Page = ui.Page{
	Key: "pages/index",
	On: ui.Handlers{
		"buttonPress": func(p json.RawMessage) {
			var n int
			_ = json.Unmarshal(p, &n)
			log.Printf("index: button pressed %d times", n)
			// Push data the other way with the app's Push method:
			// app.Push("pages/index", "state", anything) in main.go, and
			// usePaired().state receives it in index.tsx.
		},
	},
}
