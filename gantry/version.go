package gantry

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// injectedVersion is stamped by gantry dev/build via
//
//	-ldflags "-X github.com/BlueBeard63/Gantry/gantry.injectedVersion=<v>"
//
// from gantry.json's "version" field - the file stays the single
// source of truth and the exe carries the value everywhere.
var injectedVersion string

// Version returns the app's own version (gantry.json's "version"):
// the value gantry dev/build stamped into the binary, else gantry.json
// found by walking up from the working directory (covers a plain
// `go run .` during development), else "0.0.0". Show it in the UI via
// the built-in "gantry" service / useAppInfo() on the React side.
func Version() string {
	if injectedVersion != "" {
		return injectedVersion
	}
	if v := versionFromDisk(); v != "" {
		return v
	}
	return "0.0.0"
}

func versionFromDisk() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "gantry.json")); err == nil {
			var cfg struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(data, &cfg) == nil {
				return cfg.Version
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
