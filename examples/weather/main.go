// Lazy Weather - a mobile Gantry app.
//
// The screens are plain React (pages/*.tsx); all the logic lives in the
// Go "weather" service (weather.go), which fetches Open-Meteo and frames
// today's forecast against yesterday. gantry dev runs it with live
// reload in a phone-sized window; gantry mobile dev android runs it on a
// plugged-in device.
package main

import (
	"github.com/BlueBeard63/Gantry/gantry"
)

func main() {
	gantry.Run(gantry.Config{
		Name:  "weather",
		Title: "Lazy Weather",
		Port:  8331,
		Dist:  dist(),
		Pairs: gantryPairs(),
		// Setup registers the shared location/units state and the
		// "weather" service that talks to Open-Meteo (see weather.go).
		Setup: registerWeather,
	})
}
