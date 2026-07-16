package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

// iosScaffold is the data every ios template renders with.
type iosScaffold struct {
	Name        string
	Title       string
	BundleID    string
	Version     string
	VersionCode int
	Usage       []iosUsage // Info.plist usage-description entries
}

// buildIOS generates the experimental iOS scaffold into .gantry/ios/:
// an XcodeGen project.yml, Info.plist with the app's permission usage
// strings, a WKWebView shell and the GantryShim seam where c-archive
// Go linking will attach. Turning the scaffold into a running app
// needs a Mac with Xcode - elsewhere the scaffold still generates, so
// it can be inspected or handed to a Mac. Returns whether anything was
// produced.
func buildIOS(appDir string, cfg appConfig, arches []string) (bool, error) {
	if cfg.Mobile == nil || cfg.Mobile.ID == "" {
		return false, fmt.Errorf(`the ios target needs a "mobile" section with an "id" in gantry.json, e.g. "mobile": {"id": "com.example.%s"}`, cfg.Name)
	}
	dir, err := writeIOSSynth(appDir, cfg)
	if err != nil {
		return false, err
	}
	warn("the ios target is an experimental scaffold: the shell builds, the Go server is not linked in yet")
	switch {
	case runtime.GOOS != "darwin":
		info("ios scaffold generated: %s (building it needs a Mac with Xcode - see its README.md)", dir)
	case !haveXcodebuild():
		info("ios scaffold generated: %s", dir)
		warn("xcodebuild not found - install Xcode from the App Store, then run xcode-select --install")
	default:
		info("ios scaffold generated: %s (next: cd there, xcodegen generate, xed . - see its README.md)", dir)
	}
	return true, nil
}

func haveXcodebuild() bool {
	_, err := exec.LookPath("xcodebuild")
	return err == nil
}

// writeIOSSynth (re)generates the hidden Xcode scaffold in
// .gantry/ios/ - the ios counterpart of writeAndroidSynth. Files are
// overwritten in place; an app-owned mobile/ios/ directory, when
// present, is copied on top for custom Swift or resources.
func writeIOSSynth(appDir string, cfg appConfig) (string, error) {
	dir := filepath.Join(appDir, ".gantry", "ios")

	usage, err := iosUsageStrings(cfg.Mobile.Permissions, cfg.Title)
	if err != nil {
		return "", err
	}
	code, err := cfg.versionCode()
	if err != nil {
		return "", err
	}
	bundleID := cfg.Mobile.ID
	if cfg.Mobile.IOS != nil && cfg.Mobile.IOS.BundleID != "" {
		bundleID = cfg.Mobile.IOS.BundleID
	}

	s := iosScaffold{
		Name:        cfg.Name,
		Title:       cfg.Title,
		BundleID:    bundleID,
		Version:     cfg.Version,
		VersionCode: code,
		Usage:       usage,
	}

	rendered := map[string]string{
		"project.yml.tmpl":       "project.yml",
		"Info.plist.tmpl":        "Info.plist",
		"README.md.tmpl":         "README.md",
		"AppDelegate.swift.tmpl": "Sources/AppDelegate.swift",
		"GantryShim.h.tmpl":      "Sources/GantryShim.h",
		"GantryShim.c.tmpl":      "Sources/GantryShim.c",
	}
	for tmpl, out := range rendered {
		src, err := templates.ReadFile("templates/ios/" + tmpl)
		if err != nil {
			return "", fmt.Errorf("reading template ios/%s: %w", tmpl, err)
		}
		// [[ ]] delimiters, same reason as the android templates.
		t, err := template.New(tmpl).Delims("[[", "]]").Parse(string(src))
		if err != nil {
			return "", fmt.Errorf("parsing template ios/%s: %w", tmpl, err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, s); err != nil {
			return "", fmt.Errorf("rendering ios/%s: %w", tmpl, err)
		}
		if err := writeFileMkdir(filepath.Join(dir, filepath.FromSlash(out)), buf.Bytes(), 0o644); err != nil {
			return "", err
		}
	}

	if overlay := filepath.Join(appDir, "mobile", "ios"); dirExists(overlay) {
		info("applying mobile/ios overlay")
		if err := copyTree(overlay, dir); err != nil {
			return "", fmt.Errorf("copying mobile/ios overlay: %w", err)
		}
	}
	return dir, nil
}
