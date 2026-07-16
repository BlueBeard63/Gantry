package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// appConfig is gantry.json at the app root - written by gantry new,
// edited by the developer, read by dev/build.
type appConfig struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version,omitempty"` // shown in installers; default 0.1.0
	Port    int    `json:"port"`
	Mode    string `json:"mode"`  // "single" | "multi"
	Style   string `json:"style"` // "tea" | "plain"
	Tray    bool   `json:"tray"`
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
	// Build configures gantry build.
	Build struct {
		// Targets like "windows/amd64", "linux/arm64", "mac/arm64".
		// Empty = the current machine only.
		Targets []string `json:"targets,omitempty"`
		// Console keeps the console window on Windows builds (debug).
		Console bool `json:"console,omitempty"`
		// Installer also produces per-OS install artifacts: a Setup.exe
		// via Inno Setup on Windows, a .tar.gz on Linux, a .zip on Mac.
		Installer bool `json:"installer,omitempty"`
	} `json:"build,omitempty"`
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
				return "", cfg, fmt.Errorf("parsing %s: %w", filepath.Join(dir, "gantry.json"), err)
			}
			if cfg.Port == 0 {
				cfg.Port = 8330
			}
			if cfg.Version == "" {
				cfg.Version = "0.1.0"
			}
			return dir, cfg, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", cfg, errors.New("no gantry.json found in this directory or any parent (run inside a gantry app, or create one with gantry new)")
		}
		dir = parent
	}
}

func writeConfig(dir string, cfg appConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "gantry.json"), append(data, '\n'), 0o644)
}
