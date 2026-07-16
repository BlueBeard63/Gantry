package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// cmdlineToolsVersion pins the Android command-line tools bootstrap
// download (dl.google.com/android/repository/commandlinetools-<os>-<v>_latest.zip).
const cmdlineToolsVersion = "13114758"

// cmdMobile dispatches the mobile command group:
//
//	gantry mobile dev android    build + run on a plugged-in device, logcat in stdout
//	gantry mobile dev ios        (experimental; needs a mac with Xcode)
func cmdMobile(args []string) error {
	if len(args) >= 1 && args[0] == "dev" {
		platform := ""
		if len(args) >= 2 {
			platform = args[1]
		}
		switch platform {
		case "android":
			return mobileDevAndroid()
		case "ios":
			return mobileDevIOS()
		}
		return fmt.Errorf("usage: gantry mobile dev <android|ios>")
	}
	return fmt.Errorf("usage: gantry mobile dev <android|ios>")
}

// mobileDevAndroid is the phone equivalent of gantry dev: check the
// toolchain (offering to install the missing SDK pieces), find the
// USB-connected device, build the APK for that device's ABI, install,
// launch and stream the app's logcat until Ctrl+C. adb-over-wifi is
// deliberately unsupported: the cable is what makes logcat reliable.
func mobileDevAndroid() error {
	appDir, cfg, err := findApp()
	if err != nil {
		return err
	}
	if cfg.Mobile == nil || cfg.Mobile.ID == "" {
		return fmt.Errorf(`gantry mobile dev needs a "mobile" section with an "id" in gantry.json, e.g. "mobile": {"id": "com.example.%s"}`, cfg.Name)
	}

	tools, err := ensureAndroidTools()
	if err != nil {
		return err
	}
	adb, err := findADB(tools)
	if err != nil {
		return err
	}

	serial, err := pickUSBDevice(adb)
	if err != nil {
		return err
	}
	abi, err := deviceABI(adb, serial)
	if err != nil {
		return err
	}
	arch := map[string]string{"arm64-v8a": "arm64", "x86_64": "amd64"}[abi]
	if arch == "" {
		return fmt.Errorf("device %s reports unsupported ABI %q (arm64-v8a and x86_64 builds are supported)", serial, abi)
	}
	info("device %s (%s)", serial, abi)

	if err := prepareApp(appDir, cfg); err != nil {
		return err
	}
	ok, err := buildAndroid(appDir, cfg, []string{arch})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("android build was skipped - fix the warnings above and rerun")
	}

	apk := filepath.Join(appDir, "dist", "android", fmt.Sprintf("%s-%s.apk", cfg.Name, cfg.Version))
	step("installing on %s", serial)
	if err := runADB(adb, "-s", serial, "install", "-r", apk); err != nil {
		return err
	}
	step("launching %s", cfg.Mobile.ID)
	if err := runADB(adb, "-s", serial, "shell", "am", "start", "-n", cfg.Mobile.ID+"/.MainActivity"); err != nil {
		return err
	}

	// Old runs would flood the tail: start the stream at "now".
	_ = runADBQuiet(adb, "-s", serial, "logcat", "-c")
	step("streaming logcat (gantry-go) - Ctrl+C to stop")
	cat := exec.Command(adb, "-s", serial, "logcat", "-v", "time", "-s", "gantry-go")
	cat.Stdout = os.Stdout
	cat.Stderr = os.Stderr
	return cat.Run()
}

// mobileDevIOS checks the ground the ios scaffold needs, generates it
// and hands off to Xcode - running on a device stays manual while the
// ios target is an experimental scaffold.
func mobileDevIOS() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("gantry mobile dev ios needs a mac with Xcode (this machine is %s)", runtime.GOOS)
	}
	if !haveXcodebuild() {
		return fmt.Errorf("xcodebuild not found - install Xcode from the App Store, then run xcode-select --install")
	}
	appDir, cfg, err := findApp()
	if err != nil {
		return err
	}
	if _, err := buildIOS(appDir, cfg, nil); err != nil {
		return err
	}
	info("run it from Xcode: cd %s && xcodegen generate && xed .", filepath.Join(appDir, ".gantry", "ios"))
	return nil
}

// ensureAndroidTools is findAndroidTools plus interactive fixes: when
// the SDK, its command-line tools or the NDK are missing it offers to
// install them (a JDK is the one thing it won't install for you).
func ensureAndroidTools() (androidTools, error) {
	tools, missing := findAndroidTools()

	if tools.JavaHome == "" {
		return tools, fmt.Errorf("JDK 17+ not found - install one (e.g. https://adoptium.net) and set JAVA_HOME or put java on PATH")
	}

	in := bufio.NewReader(os.Stdin)
	if tools.SDK == "" {
		sdkDir := defaultSDKDir()
		if sdkDir == "" {
			return tools, fmt.Errorf("no default Android SDK location for this OS - set ANDROID_HOME")
		}
		if !askYesNo(in, fmt.Sprintf("Android SDK not found. Download the command-line tools into %s (~150 MB)?", sdkDir), true) {
			return tools, fmt.Errorf("android SDK required - install Android Studio or rerun and answer yes")
		}
		if err := bootstrapCmdlineTools(sdkDir, tools.JavaHome); err != nil {
			return tools, err
		}
	} else if sdkmanagerPath(tools.SDK) == "" {
		if !askYesNo(in, fmt.Sprintf("SDK at %s has no command-line tools (needed to install packages). Download them (~150 MB)?", tools.SDK), true) {
			return tools, fmt.Errorf("command-line tools required - install them via Android Studio's SDK Manager or rerun and answer yes")
		}
		if err := bootstrapCmdlineTools(tools.SDK, tools.JavaHome); err != nil {
			return tools, err
		}
	}
	tools, missing = findAndroidTools()

	// adb lives in platform-tools; the NDK compiles the Go server.
	wanted := map[string]string{}
	if !fileExists(adbPath(tools.SDK)) {
		wanted["platform-tools"] = "platform-tools (adb)"
	}
	if tools.NDK == "" {
		wanted["ndk;"+ndkVersion] = "NDK " + ndkVersion
	}
	for pkg, label := range wanted {
		if !askYesNo(in, fmt.Sprintf("%s is missing. Install it with sdkmanager?", label), true) {
			return tools, fmt.Errorf("%s required - rerun and answer yes, or install it via Android Studio", label)
		}
		if err := sdkmanagerInstall(tools, pkg); err != nil {
			return tools, err
		}
	}

	tools, missing = findAndroidTools()
	if len(missing) > 0 {
		return tools, fmt.Errorf("still missing after install: %s", strings.Join(missing, "; "))
	}
	return tools, nil
}

func defaultSDKDir() string {
	guesses := sdkGuesses()
	if len(guesses) == 0 {
		return ""
	}
	return guesses[0]
}

func adbPath(sdk string) string {
	name := "adb"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(sdk, "platform-tools", name)
}

// findADB prefers the SDK's platform-tools copy, then PATH.
func findADB(tools androidTools) (string, error) {
	if p := adbPath(tools.SDK); fileExists(p) {
		return p, nil
	}
	if p, err := exec.LookPath("adb"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("adb not found - install platform-tools (sdkmanager \"platform-tools\")")
}

func sdkmanagerPath(sdk string) string {
	name := "sdkmanager"
	if runtime.GOOS == "windows" {
		name += ".bat"
	}
	p := filepath.Join(sdk, "cmdline-tools", "latest", "bin", name)
	if fileExists(p) {
		return p
	}
	return ""
}

// bootstrapCmdlineTools downloads Google's command-line tools zip into
// <sdk>/cmdline-tools/latest and accepts the SDK licenses, giving a
// bare machine a working sdkmanager.
func bootstrapCmdlineTools(sdk, javaHome string) error {
	osName := map[string]string{"windows": "win", "darwin": "mac", "linux": "linux"}[runtime.GOOS]
	url := fmt.Sprintf("https://dl.google.com/android/repository/commandlinetools-%s-%s_latest.zip", osName, cmdlineToolsVersion)

	step("downloading %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading command-line tools: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading command-line tools: %s", resp.Status)
	}
	tmp, err := os.CreateTemp("", "cmdline-tools-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// The zip roots everything at cmdline-tools/; sdkmanager derives the
	// SDK root from its own path, which only works from the
	// cmdline-tools/latest/ layout.
	step("extracting to %s", filepath.Join(sdk, "cmdline-tools", "latest"))
	zr, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		rel := strings.TrimPrefix(filepath.ToSlash(f.Name), "cmdline-tools/")
		if rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		dst := filepath.Join(sdk, "cmdline-tools", "latest", filepath.FromSlash(rel))
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		mode := f.Mode() & os.ModePerm
		if mode == 0 {
			mode = 0o644
		}
		if err := writeFileMkdir(dst, data, mode); err != nil {
			return err
		}
	}

	step("accepting SDK licenses")
	return sdkmanagerRun(androidTools{SDK: sdk, JavaHome: javaHome}, "--licenses")
}

func sdkmanagerInstall(tools androidTools, pkg string) error {
	step("sdkmanager %q", pkg)
	return sdkmanagerRun(tools, pkg)
}

// sdkmanagerRun runs sdkmanager with license prompts pre-answered.
func sdkmanagerRun(tools androidTools, args ...string) error {
	sm := sdkmanagerPath(tools.SDK)
	if sm == "" {
		return fmt.Errorf("sdkmanager not found under %s", tools.SDK)
	}
	cmd := exec.Command(sm, args...)
	cmd.Env = append(os.Environ(), "JAVA_HOME="+tools.JavaHome)
	cmd.Stdin = strings.NewReader(strings.Repeat("y\n", 100))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sdkmanager %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

// pickUSBDevice returns the serial of the one USB-connected device.
// Wifi/network adb devices are rejected on purpose; emulators count as
// plugged in (they are local, logcat is just as reliable).
func pickUSBDevice(adb string) (string, error) {
	out, err := exec.Command(adb, "devices", "-l").Output()
	if err != nil {
		return "", fmt.Errorf("adb devices failed: %w", err)
	}
	var usb, network, unauthorized []string
	for _, line := range strings.Split(string(out), "\n")[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		serial, state := fields[0], fields[1]
		switch {
		case state == "unauthorized":
			unauthorized = append(unauthorized, serial)
		case state != "device":
			continue
		case strings.Contains(line, "usb:") || strings.HasPrefix(serial, "emulator-"):
			usb = append(usb, serial)
		default:
			network = append(network, serial)
		}
	}
	switch {
	case len(usb) == 1:
		return usb[0], nil
	case len(usb) > 1:
		return "", fmt.Errorf("multiple devices connected (%s) - unplug the others or set ANDROID_SERIAL", strings.Join(usb, ", "))
	case len(unauthorized) > 0:
		return "", fmt.Errorf("device %s is unauthorized - accept the USB debugging prompt on the phone", unauthorized[0])
	case len(network) > 0:
		return "", fmt.Errorf("only wifi/network adb devices found (%s) - gantry mobile dev needs the phone plugged in over USB", strings.Join(network, ", "))
	default:
		return "", fmt.Errorf("no device found - plug the phone in over USB and enable USB debugging (Settings > Developer options)")
	}
}

func deviceABI(adb, serial string) (string, error) {
	out, err := exec.Command(adb, "-s", serial, "shell", "getprop", "ro.product.cpu.abi").Output()
	if err != nil {
		return "", fmt.Errorf("reading device ABI: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runADB(adb string, args ...string) error {
	cmd := exec.Command(adb, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("adb %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

func runADBQuiet(adb string, args ...string) error {
	return exec.Command(adb, args...).Run()
}
