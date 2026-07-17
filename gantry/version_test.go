package gantry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionInjected(t *testing.T) {
	old := injectedVersion
	defer func() { injectedVersion = old }()
	injectedVersion = "1.2.3"
	if Version() != "1.2.3" {
		t.Errorf("Version() = %q, want the injected 1.2.3", Version())
	}
}

func TestVersionFromDisk(t *testing.T) {
	old := injectedVersion
	defer func() { injectedVersion = old }()
	injectedVersion = ""

	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gantry.json"), []byte(`{"version": "4.5.6"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	// Walks up from a nested working directory to the gantry.json.
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	if v := Version(); v != "4.5.6" {
		t.Errorf("Version() = %q, want 4.5.6 from gantry.json", v)
	}
}

func TestVersionDefault(t *testing.T) {
	old := injectedVersion
	defer func() { injectedVersion = old }()
	injectedVersion = ""

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if v := Version(); v != "0.0.0" {
		t.Errorf("Version() = %q, want the 0.0.0 fallback", v)
	}
}

func TestSetCloseToTray(t *testing.T) {
	defer closeToTray.Store(true)
	if !CloseToTray() {
		t.Error("close-to-tray should default to true")
	}
	SetCloseToTray(false)
	if CloseToTray() {
		t.Error("SetCloseToTray(false) not reflected")
	}
}
