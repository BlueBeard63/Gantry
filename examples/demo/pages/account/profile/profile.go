// A nested page: pairs work at any depth, and the folder path is the
// key and the route (pages/account/profile -> /account/profile).
package profile

import (
	"encoding/json"
	"log"

	"github.com/B-Commissions/Gantry/ui"
)

var Page = ui.Page{
	Key: "pages/account/profile",
	On: ui.Handlers{
		"rename": func(p json.RawMessage) {
			log.Printf("profile: rename %s", p)
		},
	},
}
