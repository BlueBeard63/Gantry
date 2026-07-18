// The weather service: the app's whole backend. It owns the shared
// location/units state, persists it, and answers two calls that reach
// Open-Meteo (https://open-meteo.com, free, no API key):
//
//	search   - place lookup for the Add Location screen
//	forecast - the computed "lazy weather" view for the Home screen
//
// The "lazy" idea is that a forecast is framed against yesterday: each
// time-of-day row carries the delta and trend versus the same hour a day
// ago, and the headline says whether today is WARMER, COOLER, or ABOUT
// THE SAME. That comparison is all done here in Go; the React side just
// renders the result.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/B-Commissions/Gantry/ui"
)

// httpClient bounds every Open-Meteo request; a call still times out at
// 30s on the frontend, but this fails faster on a dead network.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// registerWeather wires the server-side pieces and is passed to
// gantry.Config.Setup. It declares the shared state, restores it from
// disk, saves it back whenever the frontend changes it, and registers
// the "weather" service.
func registerWeather(app *ui.App, _ *http.ServeMux) {
	saved := loadSettings()
	loc := ui.NewState(app, "location", saved.Location)
	units := ui.NewState(app, "units", saved.Units)

	// OnChange fires only when React writes the value, so every user edit
	// to the place or the unit is persisted and restored next launch.
	loc.OnChange(func(l Location) { saveSettings(l, units.Get()) })
	units.OnChange(func(u string) { saveSettings(loc.Get(), u) })

	app.Service("weather", ui.Calls{
		"search":   searchLocations,
		"forecast": buildForecast,
	})
}

// --- search ----------------------------------------------------------------

// searchLocations turns a query into place matches via Open-Meteo's
// geocoding API. Queries shorter than two characters return nothing.
func searchLocations(payload json.RawMessage) (any, error) {
	var q struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &q); err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(q.Name)) < 2 {
		return []Location{}, nil
	}
	u := "https://geocoding-api.open-meteo.com/v1/search?count=8&language=en&format=json&name=" + url.QueryEscape(q.Name)
	var body struct {
		Results []struct {
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := getJSON(u, &body); err != nil {
		return nil, err
	}
	out := make([]Location, 0, len(body.Results))
	for _, r := range body.Results {
		out = append(out, Location{
			Name:    r.Name,
			Admin1:  r.Admin1,
			Country: r.Country,
			Lat:     r.Latitude,
			Lon:     r.Longitude,
		})
	}
	return out, nil
}

// --- forecast --------------------------------------------------------------

// Forecast is the computed view the Home screen renders.
type Forecast struct {
	Current  Current `json:"current"`
	Compare  string  `json:"compare"`  // WARMER | COOLER | ABOUT THE SAME
	Detail   string  `json:"detail"`   // e.g. "It will be partly cloudy."
	Rows     []Row   `json:"rows"`     // Morning, Noon, Evening, Night
	UnitSign string  `json:"unitSign"` // "C" | "F"
}

// Current is the temperature and icon right now.
type Current struct {
	Temp int    `json:"temp"`
	Icon string `json:"icon"` // sun | cloud | rain
}

// Row is one time-of-day line comparing today to yesterday.
type Row struct {
	Label string `json:"label"` // Morning | Noon | Evening | Night
	Temp  int    `json:"temp"`
	Icon  string `json:"icon"`  // sun | cloud | rain
	Delta int    `json:"delta"` // signed degrees, today minus yesterday
	Trend string `json:"trend"` // up | down | same
}

// slots are the four times of day the forecast reports, by local hour.
var slots = []struct {
	Label string
	Hour  int
}{
	{"Morning", 8},
	{"Noon", 12},
	{"Evening", 18},
	{"Night", 22},
}

func buildForecast(payload json.RawMessage) (any, error) {
	var q struct {
		Lat   float64 `json:"lat"`
		Lon   float64 `json:"lon"`
		Units string  `json:"units"`
	}
	if err := json.Unmarshal(payload, &q); err != nil {
		return nil, err
	}
	unit := "celsius"
	if q.Units == "fahrenheit" {
		unit = "fahrenheit"
	}

	// past_days=1 pulls yesterday's hourly series alongside today's so
	// the deltas can be computed; timezone=auto keys the hours to the
	// location's local clock, which is what the slot hours mean.
	u := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%g&longitude=%g"+
			"&current=temperature_2m,weather_code"+
			"&hourly=temperature_2m,weather_code"+
			"&timezone=auto&past_days=1&forecast_days=1&temperature_unit=%s",
		q.Lat, q.Lon, unit)

	var body struct {
		Current struct {
			Time        string  `json:"time"`
			Temperature float64 `json:"temperature_2m"`
			WeatherCode int     `json:"weather_code"`
		} `json:"current"`
		Hourly struct {
			Time        []string  `json:"time"`
			Temperature []float64 `json:"temperature_2m"`
			WeatherCode []int     `json:"weather_code"`
		} `json:"hourly"`
	}
	if err := getJSON(u, &body); err != nil {
		return nil, err
	}

	// Index the hourly series by timestamp for O(1) slot lookup.
	idx := make(map[string]int, len(body.Hourly.Time))
	for i, t := range body.Hourly.Time {
		idx[t] = i
	}

	today, err := time.Parse("2006-01-02T15:04", body.Current.Time)
	if err != nil && len(body.Hourly.Time) > 0 {
		// Fall back to the last hourly entry's day if current.time is odd.
		today, _ = time.Parse("2006-01-02T15:04", body.Hourly.Time[len(body.Hourly.Time)-1])
	}
	todayDate := today.Format("2006-01-02")
	yesterdayDate := today.AddDate(0, 0, -1).Format("2006-01-02")

	rows := make([]Row, 0, len(slots))
	var sumToday, sumYest float64
	var n int
	for _, s := range slots {
		ti, ok := idx[fmt.Sprintf("%sT%02d:00", todayDate, s.Hour)]
		if !ok {
			continue
		}
		tTemp := body.Hourly.Temperature[ti]
		row := Row{
			Label: s.Label,
			Temp:  int(math.Round(tTemp)),
			Icon:  iconForCode(body.Hourly.WeatherCode[ti]),
			Trend: "same",
		}
		if yi, ok := idx[fmt.Sprintf("%sT%02d:00", yesterdayDate, s.Hour)]; ok {
			yTemp := body.Hourly.Temperature[yi]
			row.Delta = int(math.Round(tTemp - yTemp))
			row.Trend = trendFor(row.Delta)
			sumToday += tTemp
			sumYest += yTemp
			n++
		}
		rows = append(rows, row)
	}

	return Forecast{
		Current:  Current{Temp: int(math.Round(body.Current.Temperature)), Icon: iconForCode(body.Current.WeatherCode)},
		Compare:  compareWord(sumToday, sumYest, n),
		Detail:   detailForCode(body.Current.WeatherCode),
		Rows:     rows,
		UnitSign: map[string]string{"celsius": "C", "fahrenheit": "F"}[unit],
	}, nil
}

// trendFor turns a signed delta into an arrow direction. A degree either
// way counts as a change; anything smaller is "same".
func trendFor(delta int) string {
	switch {
	case delta >= 1:
		return "up"
	case delta <= -1:
		return "down"
	default:
		return "same"
	}
}

// compareWord is the headline: how today's slots average against
// yesterday's. Within 2 degrees reads as "about the same".
func compareWord(sumToday, sumYest float64, n int) string {
	if n == 0 {
		return "ABOUT THE SAME"
	}
	switch diff := (sumToday - sumYest) / float64(n); {
	case diff >= 2:
		return "WARMER"
	case diff <= -2:
		return "COOLER"
	default:
		return "ABOUT THE SAME"
	}
}

// --- WMO weather codes -----------------------------------------------------

// iconForCode maps a WMO weather code to one of the three icons the UI
// draws (sun / cloud / rain). Codes: 0-1 clear, 2-3/45-48 cloud/fog,
// 51+ drizzle, rain, snow, showers and thunder.
func iconForCode(code int) string {
	switch {
	case code <= 1:
		return "sun"
	case code == 2 || code == 3 || code == 45 || code == 48:
		return "cloud"
	default:
		return "rain"
	}
}

// detailForCode is the one-line sentence under the weather statement.
func detailForCode(code int) string {
	switch code {
	case 0:
		return "The sky will be clear."
	case 1:
		return "It will be mainly clear."
	case 2:
		return "It will be partly cloudy."
	case 3:
		return "It will be overcast."
	case 45, 48:
		return "Expect fog."
	case 51, 53, 55, 56, 57:
		return "Expect light drizzle."
	case 61, 63, 65, 66, 67:
		return "Expect rain."
	case 71, 73, 75, 77:
		return "Expect snow."
	case 80, 81, 82:
		return "Expect rain showers."
	case 85, 86:
		return "Expect snow showers."
	case 95, 96, 99:
		return "Expect a thunderstorm."
	default:
		return ""
	}
}

// getJSON fetches url and decodes the JSON body into out. A non-2xx
// status becomes an error so the frontend's Await shows a retry card.
func getJSON(u string, out any) error {
	resp, err := httpClient.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("open-meteo %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
