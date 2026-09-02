// The Go half of the example component. Components pair exactly like
// pages: the .tsx next door reaches these handlers with
// usePaired().send, and Go pushes data back with app.Push(key, ...).
package example

import (
	"encoding/json"
	"log"

	"github.com/BlueBeard63/Gantry/ui"
)

var Component = ui.Component{
	Key: "components/example",
	On: ui.Handlers{
		"ping": func(p json.RawMessage) {
			log.Printf("example component: ping %s", p)
		},
	},
}
