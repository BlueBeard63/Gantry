// Shared types mirroring the Go structs in weather.go / settings.go, plus
// the default location the UI falls back to before the first state sync.

export interface Location {
  name: string;
  admin1: string; // state / region
  country: string;
  lat: number;
  lon: number;
}

export type Units = "celsius" | "fahrenheit";

export type WxKind = "sun" | "cloud" | "rain";
export type Trend = "up" | "down" | "same";

export interface Row {
  label: string; // Morning | Noon | Evening | Night
  temp: number;
  icon: WxKind;
  delta: number; // signed degrees, today minus yesterday
  trend: Trend;
}

export interface Forecast {
  current: { temp: number; icon: WxKind };
  compare: string; // WARMER | COOLER | ABOUT THE SAME
  detail: string; // e.g. "It will be partly cloudy."
  rows: Row[];
  unitSign: string; // "C" | "F"
}

// Matches defaultLocation in settings.go - the value shown for the brief
// moment before useGoState receives the real state from Go.
export const DEFAULT_LOCATION: Location = {
  name: "San Francisco",
  admin1: "California",
  country: "United States",
  lat: 37.7749,
  lon: -122.4194,
};
