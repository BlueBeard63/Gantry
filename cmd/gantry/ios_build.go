package main

import "fmt"

// buildIOS generates the experimental iOS scaffold. It needs a Mac
// with Xcode; everywhere else it warns and skips so the rest of the
// build proceeds. Returns whether anything was produced.
//
// The scaffold itself lands in a later phase; this phase ships the
// target plumbing.
func buildIOS(appDir string, cfg appConfig, arches []string) (bool, error) {
	if cfg.Mobile == nil || cfg.Mobile.ID == "" {
		return false, fmt.Errorf(`the ios target needs a "mobile" section with an "id" in gantry.json, e.g. "mobile": {"id": "com.example.%s"}`, cfg.Name)
	}
	warn("skipping ios: the iOS scaffold is not implemented yet (and needs a Mac with Xcode)")
	return false, nil
}
