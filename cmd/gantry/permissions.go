package main

import (
	"fmt"
	"sort"
	"strings"
)

// permission maps one friendly gantry.json name onto real platform
// permissions. The iOS column (Info.plist usage-description keys)
// grows here when the ios target learns to use it.
type permission struct {
	// Android manifest permission names (uses-permission entries).
	Android []string
	// Runtime marks Android "dangerous" permissions the shell must
	// also request with a runtime prompt, not just declare.
	Runtime bool
}

// permissionTable is the whole friendly-name vocabulary. INTERNET is
// not listed: every gantry app talks to its own local server, so it is
// always emitted.
var permissionTable = map[string]permission{
	"camera":              {Android: []string{"android.permission.CAMERA"}, Runtime: true},
	"microphone":          {Android: []string{"android.permission.RECORD_AUDIO"}, Runtime: true},
	"location":            {Android: []string{"android.permission.ACCESS_FINE_LOCATION", "android.permission.ACCESS_COARSE_LOCATION"}, Runtime: true},
	"location-background": {Android: []string{"android.permission.ACCESS_BACKGROUND_LOCATION"}, Runtime: true},
	"notifications":       {Android: []string{"android.permission.POST_NOTIFICATIONS"}, Runtime: true},
	"files":               {Android: []string{"android.permission.READ_EXTERNAL_STORAGE", "android.permission.WRITE_EXTERNAL_STORAGE"}, Runtime: true},
	"bluetooth":           {Android: []string{"android.permission.BLUETOOTH_CONNECT", "android.permission.BLUETOOTH_SCAN"}, Runtime: true},
	"contacts":            {Android: []string{"android.permission.READ_CONTACTS"}, Runtime: true},
	"vibrate":             {Android: []string{"android.permission.VIBRATE"}},
	"boot":                {Android: []string{"android.permission.RECEIVE_BOOT_COMPLETED"}},
	"network-state":       {Android: []string{"android.permission.ACCESS_NETWORK_STATE"}},
	"foreground-service":  {Android: []string{"android.permission.FOREGROUND_SERVICE"}},
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

func permissionNames() []string {
	names := make([]string, 0, len(permissionTable))
	for n := range permissionTable {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
