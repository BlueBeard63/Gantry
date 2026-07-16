package main

import "fmt"

// buildAndroid packs the Go server and the built frontend into one
// multi-ABI APK. It validates the mobile config hard (misconfiguration
// is the developer's to fix) but treats a missing toolchain as a
// coloured warning + skip, so desktop targets in the same run still
// build. Returns whether the APK was actually produced.
//
// APK assembly (the Gradle synth + cross-compile) lands in the next
// phase; this phase ships the config, target and toolchain plumbing.
func buildAndroid(appDir string, cfg appConfig, arches []string) (bool, error) {
	if cfg.Mobile == nil || cfg.Mobile.ID == "" {
		return false, fmt.Errorf(`the android target needs a "mobile" section with an "id" in gantry.json, e.g. "mobile": {"id": "com.example.%s"}`, cfg.Name)
	}
	if _, err := androidPermissions(cfg.Mobile.Permissions); err != nil {
		return false, err
	}
	if _, err := cfg.versionCode(); err != nil {
		return false, err
	}

	_, missing := findAndroidTools()
	if len(missing) > 0 {
		for _, m := range missing {
			warn("skipping android: missing %s", m)
		}
		return false, nil
	}

	warn("skipping android: APK assembly is not implemented yet (toolchain and config check out)")
	return false, nil
}
