package main

import (
	"fmt"
	"sort"
	"strings"
)

// permission maps one friendly gantry.json name onto real platform
// permissions.
type permission struct {
	// Android manifest permission names (uses-permission entries).
	Android []string
	// Runtime marks Android "dangerous" permissions the shell must
	// also request with a runtime prompt, not just declare.
	Runtime bool
	// IOS holds Info.plist usage-description entries the ios scaffold
	// emits; empty = nothing to declare on iOS (either not needed, or
	// requested at runtime like notifications).
	IOS []iosUsage
}

// iosUsage is one Info.plist usage-description entry. Text is a
// sentence with a %s slot for the app title.
type iosUsage struct {
	Key  string
	Text string
}

// permissionTable is the whole friendly-name vocabulary. INTERNET is
// not listed: every gantry app talks to its own local server, so it is
// always emitted.
var permissionTable = map[string]permission{
	"camera": {Android: []string{"android.permission.CAMERA"}, Runtime: true,
		IOS: []iosUsage{{"NSCameraUsageDescription", "%s uses the camera."}}},
	"microphone": {Android: []string{"android.permission.RECORD_AUDIO"}, Runtime: true,
		IOS: []iosUsage{{"NSMicrophoneUsageDescription", "%s uses the microphone."}}},
	"location": {Android: []string{"android.permission.ACCESS_FINE_LOCATION", "android.permission.ACCESS_COARSE_LOCATION"}, Runtime: true,
		IOS: []iosUsage{{"NSLocationWhenInUseUsageDescription", "%s uses your location while it is open."}}},
	"location-background": {Android: []string{"android.permission.ACCESS_BACKGROUND_LOCATION"}, Runtime: true,
		IOS: []iosUsage{{"NSLocationAlwaysAndWhenInUseUsageDescription", "%s uses your location, including in the background."}}},
	"notifications": {Android: []string{"android.permission.POST_NOTIFICATIONS"}, Runtime: true},
	"files":         {Android: []string{"android.permission.READ_EXTERNAL_STORAGE", "android.permission.WRITE_EXTERNAL_STORAGE"}, Runtime: true},
	"bluetooth": {Android: []string{"android.permission.BLUETOOTH_CONNECT", "android.permission.BLUETOOTH_SCAN"}, Runtime: true,
		IOS: []iosUsage{{"NSBluetoothAlwaysUsageDescription", "%s connects to Bluetooth devices."}}},
	"contacts": {Android: []string{"android.permission.READ_CONTACTS"}, Runtime: true,
		IOS: []iosUsage{{"NSContactsUsageDescription", "%s reads your contacts."}}},
	"vibrate":            {Android: []string{"android.permission.VIBRATE"}},
	"boot":               {Android: []string{"android.permission.RECEIVE_BOOT_COMPLETED"}},
	"network-state":      {Android: []string{"android.permission.ACCESS_NETWORK_STATE"}},
	"foreground-service": {Android: []string{"android.permission.FOREGROUND_SERVICE"}},
}

// androidPermissions expands friendly names into the manifest's
// uses-permission list: INTERNET first, then the mapped permissions in
// declaration order, deduplicated. An unknown name is a build error
// naming the whole valid vocabulary.
func androidPermissions(friendly []string) ([]string, error) {
	out := []string{"android.permission.INTERNET"}
	seen := map[string]bool{out[0]: true}
	for _, name := range friendly {
		p, ok := permissionTable[name]
		if !ok {
			return nil, fmt.Errorf("unknown mobile permission %q (valid: %s)", name, strings.Join(permissionNames(), ", "))
		}
		for _, a := range p.Android {
			if !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
	}
	return out, nil
}

// androidRuntimePermissions is the subset of androidPermissions the
// Kotlin shell requests with a runtime prompt on first launch.
func androidRuntimePermissions(friendly []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, name := range friendly {
		p, ok := permissionTable[name]
		if !ok {
			return nil, fmt.Errorf("unknown mobile permission %q (valid: %s)", name, strings.Join(permissionNames(), ", "))
		}
		if !p.Runtime {
			continue
		}
		for _, a := range p.Android {
			if !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
	}
	return out, nil
}

// iosUsageStrings expands friendly names into the Info.plist
// usage-description entries the ios scaffold declares, in declaration
// order, deduplicated by key, with the app title filled in. Names
// without an iOS mapping (notifications, vibrate, ...) contribute
// nothing - they are not an error, the same gantry.json builds both
// platforms.
func iosUsageStrings(friendly []string, title string) ([]iosUsage, error) {
	var out []iosUsage
	seen := map[string]bool{}
	for _, name := range friendly {
		p, ok := permissionTable[name]
		if !ok {
			return nil, fmt.Errorf("unknown mobile permission %q (valid: %s)", name, strings.Join(permissionNames(), ", "))
		}
		for _, u := range p.IOS {
			if !seen[u.Key] {
				seen[u.Key] = true
				out = append(out, iosUsage{Key: u.Key, Text: fmt.Sprintf(u.Text, title)})
			}
		}
	}
	return out, nil
}

func permissionNames() []string {
	names := make([]string, 0, len(permissionTable))
	for n := range permissionTable {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
