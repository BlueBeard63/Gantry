// Demo - a Gantry desktop app.
//
// gantry dev runs it with live reload; gantry build makes the exe.
// Pages and components register themselves (gantry_registry.go is
// generated from the pages/, components/ and layouts/ folders).
package main

import (
	"log"
	"net/http"

	"github.com/B-Commissions/Gantry/appshell"
	"github.com/B-Commissions/Gantry/gantry"
	"github.com/B-Commissions/Gantry/ui"
)

func main() {
	gantry.Run(gantry.Config{
		Name:  "demo",
		Title: "Demo",
		Port:  8330,
		Dist:  dist(),
		Pairs: gantryPairs(),
		Tray:  true,
		Window: func(w *appshell.WindowOptions) {
			w.EnableMaximize = true
			// app.tsx puts back/forward buttons in the left slot with
			// leftReserve: 100 - the native clickable zone must match.
			w.CaptionLeftReserve = 100
		},
		Setup: func(app *ui.App, mux *http.ServeMux) {
			// A value shared with React in real time: the settings page
			// binds a slider to it with useGoState("volume", ...).
			volume := ui.NewState(app, "volume", 0.5)
			volume.OnChange(func(v float64) {
				log.Printf("demo: volume set to %.2f from the frontend", v)
			})
		},
	})
}
