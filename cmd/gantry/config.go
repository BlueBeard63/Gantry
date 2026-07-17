package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/B-Commissions/Gantry/gerr"
)

// schemaURL points editors at the gantry.json schema for validation
// and hover docs (fetched from GitHub by VS Code's JSON service).
const schemaURL = "https://raw.githubusercontent.com/B-Commissions/Gantry/main/gantry.schema.json"

// appConfig is gantry.json at the app root - written by gantry new,
// edited by the developer, read by dev/build.
type appConfig struct {
	Schema  string `json:"$schema,omitempty"`
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version,omitempty"` // shown in installers; default 0.1.0
	// Gantry is the framework version this app was scaffolded with (or
	// last brought to by gantry upgrade) - the baseline upgrade reports
	// against. Empty in apps scaffolded before it was recorded.
	Gantry string `json:"gantry,omitempty"`
	Port    int    `json:"port"`
	Mode    string `json:"mode"`  // "single" | "multi"
	Style   string `json:"style"` // "tea" | "plain"
	Tray    bool   `json:"tray"`
	// Tailwind adds @tailwindcss/vite to the synthesized vite config;
	// set by gantry new --tailwind or gantry install --tailwind.
	Tailwind bool `json:"tailwind,omitempty"`
	Buttons struct {
		Minimize bool `json:"minimize"`
		Maximize bool `json:"maximize"`
		Close    bool `json:"close"`
	} `json:"buttons"`
	// Icons is a directory (relative to the app root) holding default
	// icon files: icon.ico (Windows exe + tray) and icon.png (window,
	// Linux tray). Baked into the exe at build time; code-level Icon
	// settings override them.
	Icons string `json:"icons,omitempty"`
	// Args declares the app's custom arguments: gantry dev validates
	// them on its command line and hands them to the app as environment
	// variables; production binaries read the same variables directly.
	Args map[string]argSpec `json:"args,omitempty"`
	// Build configures gantry build.
	Build struct {
		// Targets like "windows/amd64", "linux/arm64", "mac/arm64",
		// "android" (bare = arm64). Empty = the current machine only.
		Targets []string `json:"targets,omitempty"`
		// Console keeps the console window on Windows builds (debug).
		Console bool `json:"console,omitempty"`
		// Installer also produces per-OS install artifacts: a Setup.exe
		// via Inno Setup on Windows, a .tar.gz on Linux, a .zip on Mac.
		Installer bool `json:"installer,omitempty"`
	} `json:"build,omitempty"`
	// Mobile is required by the android and ios build targets.
	Mobile *mobileConfig `json:"mobile,omitempty"`
}

// argSpec declares one custom app argument in gantry.json's "args"
// map, keyed by the flag name (lowercase-kebab). The value reaches the
// app as an environment variable: the explicit "env" name, or
// GANTRY_ARG_<UPPER_SNAKE> derived from the flag name.
type argSpec struct {
	Type        string          `json:"type,omitempty"` // "string" (default) | "bool" | "int"
	Default     json.RawMessage `json:"default,omitempty"`
	Description string          `json:"description,omitempty"`
	Env         string          `json:"env,omitempty"`
}

// mobileConfig is gantry.json's "mobile" section: app identity,
// friendly permissions, home-screen widgets and per-OS options.
type mobileConfig struct {
	// ID is the reverse-DNS application id, e.g. "ec.morrison.demo".
	ID string `json:"id"`
	// Permissions holds friendly names ("camera", "notifications", ...)
	// mapped to real platform permissions by permissions.go.
	Permissions []string       `json:"permissions,omitempty"`
	Widgets     []mobileWidget `json:"widgets,omitempty"`
	Android     *androidConfig `json:"android,omitempty"`
	IOS         *iosConfig     `json:"ios,omitempty"`
}

// mobileWidget is launcher metadata for one home-screen widget; its
// content comes from the paired widgets/<name>/<name>.go.
type mobileWidget struct {
	Name           string `json:"name"`
	Label          string `json:"label,omitempty"`
	MinSize        string `json:"minSize,omitempty"` // grid cells, "CxR"
	MaxSize        string `json:"maxSize,omitempty"`
	Resize         string `json:"resize,omitempty"` // "none", "horizontal", "vertical" or "horizontal|vertical"
	RefreshMinutes int    `json:"refreshMinutes,omitempty"`
}

type androidConfig struct {
	MinSdk    int             `json:"minSdk,omitempty"`
	TargetSdk int             `json:"targetSdk,omitempty"`
	Keystore  *keystoreConfig `json:"keystore,omitempty"`
}

// keystoreConfig points at a release signing key; without one the APK
// is signed with a debug key (installable, not store-ready).
type keystoreConfig struct {
	File        string `json:"file"`
	Alias       string `json:"alias"`
	PasswordEnv string `json:"passwordEnv"`
}

type iosConfig struct {
	BundleID string `json:"bundleId,omitempty"`
}

// versionCode derives Android's integer version code from the semantic
// version: major*10000 + minor*100 + patch, so every release bump
// yields a strictly larger code.
func (c appConfig) versionCode() (int, error) {
	p, ok := semverParts(c.Version)
	if !ok {
		return 0, fmt.Errorf("version %q is not X.Y.Z semver (needed to derive the Android versionCode)", c.Version)
	}
	return p[0]*10000 + p[1]*100 + p[2], nil
}

// findApp walks up from the working directory to the nearest gantry.json.
func findApp() (dir string, cfg appConfig, err error) {
	dir, err = os.Getwd()
	if err != nil {
		return "", cfg, err
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(dir, "gantry.json"))
		if readErr == nil {
			if err := json.Unmarshal(data, &cfg); err != nil {
				return "", cfg, gerr.Wrap("config.parse", err, "parsing %s", filepath.Join(dir, "gantry.json"))
			}
			if cfg.Port == 0 {
				cfg.Port = 8330
			}
			if cfg.Version == "" {
				cfg.Version = "0.1.0"
			}
			if cfg.Mobile != nil {
				if cfg.Mobile.Android == nil {
					cfg.Mobile.Android = &androidConfig{}
				}
				if cfg.Mobile.Android.MinSdk == 0 {
					cfg.Mobile.Android.MinSdk = 26
				}
				if cfg.Mobile.Android.TargetSdk == 0 {
					cfg.Mobile.Android.TargetSdk = 35
				}
			}
			return dir, cfg, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", cfg, gerr.New("config.not-found", "no gantry.json found in this directory or any parent").
				WithHint("run inside a gantry app, or create one with gantry new")
		}
		dir = parent
	}
}

func writeConfig(dir string, cfg appConfig) error {
	if cfg.Schema == "" {
		cfg.Schema = schemaURL
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "gantry.json"), append(data, '\n'), 0o644)
}
