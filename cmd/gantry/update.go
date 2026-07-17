package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// cmdUpdate updates the gantry CLI itself to the newest release tag:
// gantry update = go install <module>/cmd/gantry@<latest>, plus the
// Windows dance (a running exe cannot be overwritten, but it can be
// renamed aside).
func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	force := fs.Bool("force", false, "reinstall even when already up to date")
	if err := fs.Parse(args); err != nil {
		return err
	}

	current := taggedVersion()
	if current == "" {
		info("this gantry is a local checkout build (%s)", fullVersion())
		info("update it from the checkout instead: git pull && go install ./cmd/gantry")
		return nil
	}
	latest := latestVersionFresh()
	if latest == "" {
		return fmt.Errorf("could not reach the Go module proxy to look up the latest version")
	}
	if !semverLess(current, latest) && !*force {
		success("already up to date (%s)", current)
		return nil
	}

	// Rename the running exe aside so go install can write a fresh
	// gantry.exe in its place. Assignment only when the rename worked -
	// when gantry runs from somewhere other than the install target the
	// rename is harmless either way. The leftover gantry-old.exe cannot
	// be deleted while it runs; the next update removes it.
	var exePath, renamed string
	if runtime.GOOS == "windows" {
		if exe, err := os.Executable(); err == nil {
			old := filepath.Join(filepath.Dir(exe), "gantry-old.exe")
			_ = os.Remove(old)
			if err := os.Rename(exe, old); err == nil {
				exePath, renamed = exe, old
			}
		}
	}

	step("go install %s/cmd/gantry@%s", modulePath, latest)
	cmd := exec.Command("go", "install", modulePath+"/cmd/gantry@"+latest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if renamed != "" {
			_ = os.Rename(renamed, exePath) // roll back so gantry keeps working
		}
		return fmt.Errorf("go install failed: %w", err)
	}

	success("updated %s -> %s", current, latest)
	info("run gantry upgrade inside your apps to pull the matching template and package changes")
	return nil
}
