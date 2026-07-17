package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/B-Commissions/Gantry/gerr"
)

// prepareDeviceTarget readies a phone or emulator for `gantry test
// --device` (tier M1 of the testing plan): it resolves adb and the
// device, builds the debug APK for the device's ABI (debuggable, so
// the shell honors the runner's test config), installs it, and returns
// the environment the gantrytest driver's adb backend reads plus a
// cleanup that removes the test app again. The debug variant carries
// an applicationIdSuffix of ".test", so the test install lives beside
// a real install of the app and uninstalling it touches nothing else.
func prepareDeviceTarget(appDir string, cfg appConfig, device string, allowData bool) ([]string, func(), error) {
	platform, serial, _ := strings.Cut(device, ":")
	if platform != "android" {
		return nil, nil, gerr.New("test.bad-device", "unknown --device %q (android or android:SERIAL)", device)
	}
	if cfg.Mobile == nil || cfg.Mobile.ID == "" {
		return nil, nil, gerr.New("test.no-mobile", `--device needs a "mobile" section with an "id" in gantry.json, e.g. "mobile": {"id": "com.example.%s"}`, cfg.Name)
	}
	testAppID := cfg.Mobile.ID + ".test"

	tools, err := ensureAndroidTools()
	if err != nil {
		return nil, nil, err
	}
	adb, err := findADB(tools)
	if err != nil {
		return nil, nil, err
	}
	if serial == "" {
		serial, err = pickUSBDevice(adb)
		if err != nil {
			return nil, nil, err
		}
	} else if err := exec.Command(adb, "-s", serial, "get-state").Run(); err != nil {
		return nil, nil, fmt.Errorf("device %s is not connected (adb get-state failed)", serial)
	}
	abi, err := deviceABI(adb, serial)
	if err != nil {
		return nil, nil, err
	}
	arch := map[string]string{"arm64-v8a": "arm64", "x86_64": "amd64"}[abi]
	if arch == "" {
		return nil, nil, fmt.Errorf("device %s reports unsupported ABI %q (arm64-v8a and x86_64 builds are supported)", serial, abi)
	}
	info("device %s (%s)", serial, abi)

	// Hermetic runs wipe the test app's data (pm clear) before each
	// test instance. Even against the suffixed test id that is
	// destructive on someone's actual phone, so it needs an emulator or
	// explicit consent; without either the suite still runs, just
	// without the wipe.
	allowClear := allowData || isEmulator(adb, serial)
	if !allowClear {
		warn("physical device without --allow-device-data: skipping pm clear, test-app data persists between tests")
	}

	apk, ok, err := buildAndroidAPK(appDir, cfg, []string{arch}, true)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, gerr.New("test.no-toolchain", "the android toolchain is incomplete - fix the warnings above and rerun")
	}

	step("installing %s on %s", testAppID, serial)
	if err := runADB(adb, "-s", serial, "install", "-r", apk); err != nil {
		// The common failure is a signature clash with a leftover test
		// install from another machine: replace it wholesale.
		warn("install -r failed, retrying after uninstall")
		_ = runADBQuiet(adb, "-s", serial, "uninstall", testAppID)
		if err := runADB(adb, "-s", serial, "install", apk); err != nil {
			return nil, nil, err
		}
	}

	env := []string{
		"GANTRY_TEST_DEVICE=" + device,
		"GANTRY_TEST_ADB=" + adb,
		"GANTRY_TEST_SERIAL=" + serial,
		"GANTRY_TEST_APP_ID=" + testAppID,
		"GANTRY_TEST_ACTIVITY=" + cfg.Mobile.ID + ".MainActivity",
	}
	if allowClear {
		env = append(env, "GANTRY_TEST_ALLOW_CLEAR=1")
	}
	cleanup := func() {
		step("removing %s from %s", testAppID, serial)
		_ = runADBQuiet(adb, "-s", serial, "uninstall", testAppID)
	}
	return env, cleanup, nil
}

// isEmulator reports whether the serial is an AVD - the case where
// wiping app data needs no consent.
func isEmulator(adb, serial string) bool {
	if strings.HasPrefix(serial, "emulator-") {
		return true
	}
	out, err := exec.Command(adb, "-s", serial, "shell", "getprop", "ro.kernel.qemu").Output()
	return err == nil && strings.TrimSpace(string(out)) == "1"
}
