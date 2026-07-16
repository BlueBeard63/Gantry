package main

import (
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"golang.org/x/image/draw"

	"github.com/B-Commissions/Gantry/appicon"
)

// androidABI maps Go arch names onto Android's jniLibs ABI directories.
var androidABI = map[string]string{"arm64": "arm64-v8a", "amd64": "x86_64"}

// buildAndroid packs the Go server and the built frontend into one
// multi-ABI APK. It validates the mobile config hard (misconfiguration
// is the developer's to fix) but treats a missing toolchain as a
// coloured warning + skip, so desktop targets in the same run still
// build. Returns whether the APK was actually produced.
func buildAndroid(appDir string, cfg appConfig, arches []string) (bool, error) {
	if cfg.Mobile == nil || cfg.Mobile.ID == "" {
		return false, fmt.Errorf(`the android target needs a "mobile" section with an "id" in gantry.json, e.g. "mobile": {"id": "com.example.%s"}`, cfg.Name)
	}
	if _, err := androidPermissions(cfg.Mobile.Permissions); err != nil {
		return false, err
	}
	if _, err := cfg.versionCode(); err != nil {
		return false, err
	}

	tools, missing := findAndroidTools()
	if len(missing) > 0 {
		for _, m := range missing {
			warn("skipping android: missing %s", m)
		}
		return false, nil
	}

	synthDir, err := writeAndroidSynth(appDir, cfg)
	if err != nil {
		return false, err
	}

	// Cross-compile the Go server (frontend embedded via webdist) into
	// jniLibs, one shared-library-named executable per ABI.
	for _, arch := range arches {
		abi, ok := androidABI[arch]
		if !ok {
			return false, fmt.Errorf("unknown android arch %q", arch)
		}
		out := filepath.Join(synthDir, "app", "src", "main", "jniLibs", abi, "libgantryapp.so")
		step("building android/%s (go)", arch)
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", out, ".")
		cmd.Dir = appDir
		cmd.Env = append(os.Environ(),
			"GOOS=android",
			"GOARCH="+arch,
			"CGO_ENABLED=1",
			"CC="+tools.clang(arch, cfg.Mobile.Android.MinSdk),
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("go build android/%s failed: %w", arch, err)
		}
	}

	if err := writeAndroidIcons(appDir, cfg, synthDir); err != nil {
		return false, err
	}

	// local.properties points Gradle at this machine's SDK; regenerated
	// every build like the rest of the synth dir.
	lp := "# Synthesized by gantry build - regenerated every run.\nsdk.dir=" + filepath.ToSlash(tools.SDK) + "\n"
	if err := os.WriteFile(filepath.Join(synthDir, "local.properties"), []byte(lp), 0o644); err != nil {
		return false, err
	}

	step("assembling APK (gradle - the first run downloads Gradle itself)")
	gradlew := filepath.Join(synthDir, "gradlew")
	if runtime.GOOS == "windows" {
		gradlew += ".bat"
	}
	gradle := exec.Command(gradlew, ":app:assembleRelease", "--no-daemon")
	gradle.Dir = synthDir
	gradle.Env = append(os.Environ(), "JAVA_HOME="+tools.JavaHome, "ANDROID_HOME="+tools.SDK)
	gradle.Stdout = os.Stdout
	gradle.Stderr = os.Stderr
	if err := gradle.Run(); err != nil {
		return false, fmt.Errorf("gradle build failed: %w", err)
	}

	apk := filepath.Join(synthDir, "app", "build", "outputs", "apk", "release", "app-release.apk")
	dest := filepath.Join(appDir, "dist", "android", fmt.Sprintf("%s-%s.apk", cfg.Name, cfg.Version))
	if err := copyFile(apk, dest); err != nil {
		return false, fmt.Errorf("copying APK to dist: %w", err)
	}
	success("android APK - %s", dest)
	return true, nil
}

// writeAndroidIcons writes the launcher icon at every mipmap density,
// scaled from icons/icon.png, or drawn from the placeholder glyph when
// the app has no icon file (same fallback as the desktop surfaces).
func writeAndroidIcons(appDir string, cfg appConfig, synthDir string) error {
	densities := []struct {
		dir string
		px  int
	}{
		{"mipmap-mdpi", 48},
		{"mipmap-hdpi", 72},
		{"mipmap-xhdpi", 96},
		{"mipmap-xxhdpi", 144},
		{"mipmap-xxxhdpi", 192},
	}

	iconDir := cfg.Icons
	if iconDir == "" {
		iconDir = "icons"
	}
	var src image.Image
	if pngPath := filepath.Join(appDir, iconDir, "icon.png"); fileExists(pngPath) {
		f, err := os.Open(pngPath)
		if err != nil {
			return err
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("reading %s: %w", pngPath, err)
		}
		src = img
	}

	for _, d := range densities {
		var icon image.Image
		if src != nil {
			scaled := image.NewNRGBA(image.Rect(0, 0, d.px, d.px))
			draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), draw.Src, nil)
			icon = scaled
		} else {
			icon = appicon.Render(d.px, appicon.DefaultPalette())
		}
		out := filepath.Join(synthDir, "app", "src", "main", "res", d.dir, "ic_launcher.png")
		if err := writeFileMkdir(out, appicon.PNG(icon), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies src over dst, creating dst's parent directories.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
