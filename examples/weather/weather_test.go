// Live smoke tests for the weather service. They hit the real Open-Meteo
// endpoints, so they skip under -short and skip (rather than fail) when
// the network is unavailable - a flaky connection should never break the
// build. Run just these with: go test -run TestSearch|TestForecast.
package main

import (
	"encoding/json"
	"errors"
	"net"
	"testing"
)

// skipIfOffline turns a network error into a skip so CI without egress
// doesn't fail on these live tests.
func skipIfOffline(t *testing.T, err error) {
	t.Helper()
	var netErr net.Error
	if errors.As(err, &netErr) || err != nil {
		t.Skipf("skipping live Open-Meteo test: %v", err)
	}
}

func TestSearchLocations(t *testing.T) {
	if testing.Short() {
		t.Skip("live network test")
	}
	out, err := searchLocations(json.RawMessage(`{"name":"San Francisco"}`))
	if err != nil {
		skipIfOffline(t, err)
	}
	locs, ok := out.([]Location)
	if !ok || len(locs) == 0 {
		t.Fatalf("expected at least one location, got %#v", out)
	}
	first := locs[0]
	if first.Name == "" || first.Country == "" || first.Lat == 0 {
		t.Fatalf("result missing fields: %#v", first)
	}
	t.Logf("top match: %s, %s, %s (%.4f, %.4f)", first.Name, first.Admin1, first.Country, first.Lat, first.Lon)
}

func TestSearchShortQuery(t *testing.T) {
	out, err := searchLocations(json.RawMessage(`{"name":"a"}`))
	if err != nil {
		t.Fatalf("short query should not error: %v", err)
	}
	if locs, ok := out.([]Location); !ok || len(locs) != 0 {
		t.Fatalf("short query should return no results, got %#v", out)
	}
}

func TestBuildForecast(t *testing.T) {
	if testing.Short() {
		t.Skip("live network test")
	}
	out, err := buildForecast(json.RawMessage(`{"lat":37.7749,"lon":-122.4194,"units":"celsius"}`))
	if err != nil {
		skipIfOffline(t, err)
	}
	fc, ok := out.(Forecast)
	if !ok {
		t.Fatalf("expected Forecast, got %#v", out)
	}
	if fc.UnitSign != "C" {
		t.Errorf("unitSign = %q, want C", fc.UnitSign)
	}
	if fc.Compare == "" || fc.Detail == "" {
		t.Errorf("empty statement: compare=%q detail=%q", fc.Compare, fc.Detail)
	}
	if len(fc.Rows) == 0 {
		t.Fatalf("no forecast rows")
	}
	wantLabels := map[string]bool{"Morning": true, "Noon": true, "Evening": true, "Night": true}
	for _, r := range fc.Rows {
		if !wantLabels[r.Label] {
			t.Errorf("unexpected row label %q", r.Label)
		}
		switch r.Icon {
		case "sun", "cloud", "rain":
		default:
			t.Errorf("row %s: bad icon %q", r.Label, r.Icon)
		}
		switch r.Trend {
		case "up", "down", "same":
		default:
			t.Errorf("row %s: bad trend %q", r.Label, r.Trend)
		}
	}
	t.Logf("SF now: %d°C %s - today is %s; %s (%d rows)",
		fc.Current.Temp, fc.Current.Icon, fc.Compare, fc.Detail, len(fc.Rows))
}

func TestBuildForecastFahrenheit(t *testing.T) {
	if testing.Short() {
		t.Skip("live network test")
	}
	out, err := buildForecast(json.RawMessage(`{"lat":37.7749,"lon":-122.4194,"units":"fahrenheit"}`))
	if err != nil {
		skipIfOffline(t, err)
	}
	if fc, ok := out.(Forecast); !ok || fc.UnitSign != "F" {
		t.Fatalf("expected fahrenheit forecast, got %#v", out)
	}
}
