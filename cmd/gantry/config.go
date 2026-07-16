package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// appConfig is gantry.json at the app root - written by gantry new so
// dev/build/docs do not re-ask anything.
type appConfig struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Port  int    `json:"port"`
	Mode  string `json:"mode"`  // "single" | "multi"
	Style string `json:"style"` // "tea" | "plain"
	Tray  bool   `json:"tray"`
	Buttons struct {
		Minimize bool `json:"minimize"`
		Maximize bool `json:"maximize"`
		Close    bool `json:"close"`
	} `json:"buttons"`
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
