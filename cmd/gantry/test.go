package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/B-Commissions/Gantry/gerr"
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

	env := append(os.Environ(),
		"GANTRY_TEST_APP_DIR="+appDir,
		"GANTRY_TEST_BIN="+binPath,
		"GANTRY_TEST_ARTIFACTS="+filepath.Join(appDir, "test-results"),
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

	goArgs := []string{"test", "./tests/...",
		"-count=1", // e2e runs are never cacheable
		"-parallel", strconv.Itoa(*par),
		"-timeout", timeout.String(),
	}
	if *verbose {
		goArgs = append(goArgs, "-v")
	}
	if pattern != "" {
		goArgs = append(goArgs, "-run", pattern)
	}

	step("running tests (go test ./tests/...)")
	test := exec.Command("go", goArgs...)
	test.Dir = appDir
	test.Env = env
	test.Stdout = os.Stdout
	test.Stderr = os.Stderr
	if err := test.Run(); err != nil {
		return gerr.New("test.failed", "tests failed").
			WithHint("failing tests keep their artifacts in %s", filepath.Join(appDir, "test-results"))
	}
	success("tests passed")
	return nil
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
