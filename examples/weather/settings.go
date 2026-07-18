// Settings persistence. The chosen place and unit are the app's only
// durable state, so they live in one small JSON file under the user's
// home dir and are reloaded on launch. On Android the Go server runs
// with HOME set to the app's private files dir (see docs/mobile/android.md),
// so this is a per-app writable path on the phone and the normal home
// dir on desktop - no "files" permission or external storage needed.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Location is a place the app shows weather for. It is the value of the
// shared "location" state (mirrored to React with useGoState) and half
// of the persisted settings.
type Location struct {
	Name    string  `json:"name"`
	Admin1  string  `json:"admin1"` // state / region, e.g. "California"
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

// defaultLocation is where a fresh install starts.
var defaultLocation = Location{
	Name:    "San Francisco",
	Admin1:  "California",
	Country: "United States",
	Lat:     37.7749,
	Lon:     -122.4194,
}

// settings is the whole persisted state: the place and the unit.
type settings struct {
	Location Location `json:"location"`
	Units    string   `json:"units"` // "celsius" | "fahrenheit"
}

// settingsPath is <home>/.lazy-weather/settings.json.
func settingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".lazy-weather", "settings.json")
}

// loadSettings reads the saved settings, falling back to the defaults on
// a first run or any read/parse error.
func loadSettings() settings {
	s := settings{Location: defaultLocation, Units: "celsius"}
	b, err := os.ReadFile(settingsPath())
	if err != nil {
		return s
	}
	// A partial or garbage file keeps whatever defaults it couldn't fill.
	_ = json.Unmarshal(b, &s)
	if s.Units != "fahrenheit" {
		s.Units = "celsius"
	}
	if s.Location.Name == "" {
		s.Location = defaultLocation
	}
	return s
}

// saveSettings writes the settings to disk, creating the directory on
// first use. Called from the state OnChange observers, so every user
// edit to the place or the unit is flushed immediately.
func saveSettings(loc Location, units string) {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	b, err := json.MarshalIndent(settings{Location: loc, Units: units}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o600)
}
