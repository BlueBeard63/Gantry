// Hello - a Gantry desktop app.
//
// gantry dev runs it with live reload; gantry build makes the exe.
// Pages and components register themselves (gantry_registry.go is
// generated from the pages/, components/ and layouts/ folders).
package main

import (
	"github.com/B-Commissions/Gantry/gantry"
)

func main() {
	gantry.Run(gantry.Config{
		Name:  "hello",
		Title: "Hello",
		Port:  8330,
		Dist:  dist(),
		Pairs: gantryPairs(),
		// Setup is where services, shared state and API routes go:
		// Setup: func(app *ui.App, mux *http.ServeMux) {
		//     app.Service("auth", ui.Calls{...})
		//     volume := ui.NewState(app, "volume", 0.5)
		//     mux.HandleFunc("/api/hello", ...)
		// },
	})
}
