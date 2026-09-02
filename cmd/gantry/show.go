package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/BlueBeard63/Gantry/gerr"
)

// showReport opens the most recent gantry_test_report.html in the
// browser (gantry test --show). Because the report is a self-contained
// file, this is just "open the file" - no server. An optional test name
// deep-links straight to that test's detail via the URL hash.
func showReport(artifactRoot, testName string) error {
	path := filepath.Join(artifactRoot, "gantry_test_report.html")
	if _, err := os.Stat(path); err != nil {
		return gerr.New("test.no-report", "no report at %s", path).
			WithHint("run `gantry test` first; it writes the report at the end of every run")
	}
	target := fileURL(path)
	if testName != "" {
		target += "#test=" + url.PathEscape(testName)
	}
	step("opening %s", path)
	return browserOpen(target)
}

// openInBrowser is the best-effort auto-open after a run (gantry test
// --open); a failure is a warning, never fatal.
func openInBrowser(path string) {
	if err := browserOpen(fileURL(path)); err != nil {
		warn("could not open the report: %v", err)
	}
}

// fileURL turns a local path into a file:// URL the browser will load.
func fileURL(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.ToSlash(abs)
	if len(abs) > 0 && abs[0] != '/' {
		abs = "/" + abs // Windows: /C:/...
	}
	return "file://" + abs
}

// browserOpen launches the platform's default browser on a URL.
func browserOpen(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launching the browser: %w", err)
	}
	return nil
}
