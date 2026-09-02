package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/BlueBeard63/Gantry/gantrytest/report"
	"github.com/BlueBeard63/Gantry/gerr"
)

// cmdTest runs the app's end-to-end tests: Go tests under tests/
// using the gantrytest driver. It is a thin wrapper over
// `go test ./tests/...` that first prepares the app (regenerated
// .gantry/, registries, one vite build so the served frontend is
// current), prebuilds the app binary once for the whole suite, and
// sets the environment the driver expects.
func cmdTest(args []string) error {
	appDir, cfg, err := findApp()
	if err != nil {
		return err
	}
	if err := validateArgSpecs(cfg); err != nil {
		return fmt.Errorf("gantry.json: %w", err)
	}

	fs := flag.NewFlagSet("test", flag.ExitOnError)
	headed := fs.Bool("headed", false, "run apps with the real window instead of headless")
	record := fs.Bool("record", false, "record a screencast.avi artifact for every DOM-plane test (implies keeping those artifacts)")
	keep := fs.Bool("keep-artifacts", false, "keep passing tests' artifacts too (test-results/)")
	mode := fs.String("mode", "development", "app mode for the suite: development or production")
	device := fs.String("device", "", `run the suite on a device instead of the desktop: "android" (sole connected device/emulator) or "android:SERIAL"`)
	allowDeviceData := fs.Bool("allow-device-data", false, "allow the hermetic pm clear (wipes the app's on-device data) on a physical device; emulators always allow it")
	par := fs.Int("p", defaultParallel(), "test parallelism (each parallel test is a full app process)")
	verbose := fs.Bool("v", false, "verbose go test output")
	update := fs.Bool("update", false, "update golden files (widget snapshots) instead of comparing")
	retries := fs.Int("retries", 0, "re-run each failed test up to N times; a test that then passes is reported flaky")
	show := fs.Bool("show", false, "open the most recent gantry_test_report.html instead of running tests")
	open := fs.Bool("open", false, "open gantry_test_report.html in the browser when the run finishes")
	timeout := fs.Duration("timeout", 10*time.Minute, "overall go test timeout")
	fs.Usage = func() {
		fmt.Println("Usage: gantry test [flags] [pattern]")
		fmt.Println("\nRuns the app's end-to-end tests (Go tests in tests/ using the gantrytest driver).")
		fmt.Println("pattern filters test names, passed to go test -run.")
		fmt.Println("\nFlags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	pattern := fs.Arg(0)

	artifactRoot := filepath.Join(appDir, "test-results")
	// --show is a viewer shortcut: open the last report, run nothing.
	if *show {
		return showReport(artifactRoot, pattern)
	}

	if *mode != "development" && *mode != "production" {
		return gerr.New("test.bad-mode", "unknown --mode %q (development or production)", *mode)
	}
	testsDir := filepath.Join(appDir, "tests")
	if fi, err := os.Stat(testsDir); err != nil || !fi.IsDir() {
		return gerr.New("test.no-tests", "no tests/ directory in %s", appDir).
			WithHint("create tests/<name>_test.go using the gantrytest package - see the testing docs")
	}

	// Prepare the app exactly like a build: synth, registries, one vite
	// build so the embedded frontend is current.
	if err := prepareApp(appDir, cfg); err != nil {
		return err
	}

	// Build the app once for the whole suite; every Launch reuses it
	// via GANTRY_TEST_BIN instead of racing its own build.
	exeName := cfg.Name
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	binPath := filepath.Join(appDir, ".gantry", "test", exeName)
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return err
	}
	step("building app binary (go)")
	build := exec.Command("go", "build", "-o", binPath, "-ldflags", versionLdflag(cfg), ".")
	build.Dir = appDir
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	// A device target (tier M1): build + install the debug APK (as
	// <id>.test, beside any real install) and hand the driver the adb
	// backend's environment. One app instance per device, so
	// parallelism drops to 1; the test app is uninstalled again when
	// the suite finishes.
	var deviceEnv []string
	if *device != "" {
		env, cleanup, err := prepareDeviceTarget(appDir, cfg, *device, *allowDeviceData)
		if err != nil {
			return err
		}
		defer cleanup()
		deviceEnv = env
		if *par != 1 {
			info("device target: parallelism forced to 1 (one app instance per device)")
			*par = 1
		}
	}

	runDir := filepath.Join(artifactRoot, ".gantry-run")
	_ = os.RemoveAll(runDir) // a fresh run starts with no stale records
	env := append(os.Environ(),
		"GANTRY_TEST_APP_DIR="+appDir,
		"GANTRY_TEST_BIN="+binPath,
		"GANTRY_TEST_ARTIFACTS="+artifactRoot,
		"GANTRY_TEST_RUN_DIR="+runDir,
		"GANTRY_TEST_MODE="+*mode,
	)
	if *headed {
		env = append(env, "GANTRY_TEST_HEADED=1")
	}
	if *record {
		env = append(env, "GANTRY_TEST_RECORD=1")
	}
	if *keep {
		env = append(env, "GANTRY_TEST_KEEP_ARTIFACTS=1")
	}
	if *update {
		env = append(env, "GANTRY_UPDATE_GOLDENS=1")
	}
	env = append(env, deviceEnv...)

	baseArgs := []string{"./tests/...",
		"-count=1", // e2e runs are never cacheable
		"-parallel", strconv.Itoa(*par),
		"-timeout", timeout.String(),
	}
	if pattern != "" {
		baseArgs = append(baseArgs, "-run", pattern)
	}

	started := time.Now()
	step("running tests (go test ./tests/...)")

	// Attempt 1 runs the whole suite. attempts[name] accumulates each
	// test's outcome per attempt (index 0 == attempt 1).
	attempts := map[string][]*attemptOutcome{}
	attemptEnv := func(k int) []string {
		if *retries == 0 {
			return env // flat artifact dirs, as a normal run has always had
		}
		return append(append([]string(nil), env...), "GANTRY_TEST_ATTEMPT="+strconv.Itoa(k))
	}
	outs, err := runGoTest(appDir, attemptEnv(1), baseArgs, *verbose)
	if err != nil {
		return fmt.Errorf("running go test: %w", err)
	}
	for name, o := range outs {
		attempts[name] = []*attemptOutcome{o}
	}

	// Retries: re-run each still-failing test on its own, up to N times or
	// until it passes. Running one test per invocation keeps its artifacts
	// isolated and the -run pattern unambiguous.
	failing := sortedFailedNames(outs)
	for k := 2; k <= *retries+1 && len(failing) > 0; k++ {
		info("retry %d/%d: %d failed test(s)", k-1, *retries, len(failing))
		var stillFailing []string
		for _, name := range failing {
			ra := append(append([]string(nil), baseArgs...), "-run", runPattern(name))
			// -run above narrows to this one test; drop any suite pattern.
			ra = stripDuplicateRun(ra)
			ro, rerr := runGoTest(appDir, attemptEnv(k), ra, *verbose)
			if rerr != nil {
				return fmt.Errorf("running go test (retry): %w", rerr)
			}
			o := ro[name]
			if o == nil {
				o = &attemptOutcome{Name: name, Status: report.StatusFail}
			}
			attempts[name] = append(attempts[name], o)
			if o.Status == report.StatusFail {
				stillFailing = append(stillFailing, name)
			}
		}
		failing = stillFailing
	}

	// Build the report and write the self-contained HTML.
	command := strings.TrimSpace("gantry test " + strings.Join(args, " "))
	target := "desktop"
	if *device != "" {
		target = *device
	}
	rep := buildRunReport(artifactRoot, runDir, command, started.Format(time.RFC3339), time.Since(started).Seconds(), target, attempts)
	reportPath, rerr := writeReport(rep, artifactRoot)
	if rerr != nil {
		warn("could not write the test report: %v", rerr)
	} else {
		info("report: %s", reportPath)
		if *open {
			openInBrowser(reportPath)
		}
	}
	printSummary(rep)

	if rep.Counts.Failed > 0 {
		return gerr.New("test.failed", "%d test(s) failed", rep.Counts.Failed).
			WithHint("open the report (gantry test --show) or the artifacts in %s", artifactRoot)
	}
	success("tests passed")
	return nil
}

// stripDuplicateRun keeps only the last -run flag, so a suite pattern
// plus a per-test retry pattern don't fight (the retry pattern wins).
func stripDuplicateRun(args []string) []string {
	last := -1
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-run" {
			last = i
		}
	}
	if last < 0 {
		return args
	}
	var out []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-run" && i != last {
			i++ // skip its value too
			continue
		}
		out = append(out, args[i])
	}
	return out
}

// printSummary prints the final one-line tally after a run.
func printSummary(rep report.RunReport) {
	c := rep.Counts
	parts := []string{fmt.Sprintf("%d passed", c.Passed)}
	if c.Flaky > 0 {
		parts = append(parts, fmt.Sprintf("%d flaky", c.Flaky))
	}
	if c.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", c.Failed))
	}
	if c.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", c.Skipped))
	}
	info("%s in %.1fs", strings.Join(parts, ", "), rep.Duration)
}

// defaultParallel is NumCPU/2, floored at 1 - each parallel test is a
// full app process, so hyperthread-count parallelism just thrashes.
func defaultParallel() int {
	n := runtime.NumCPU() / 2
	if n < 1 {
		n = 1
	}
	return n
}
