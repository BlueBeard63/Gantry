package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// androidScaffold is the data every android template renders with.
type androidScaffold struct {
	Name               string
	Title              string
	ID                 string // reverse-DNS application id
	Version            string
	VersionCode        int
	MinSdk             int
	TargetSdk          int
	Permissions        []string        // manifest uses-permission names
	RuntimePermissions []string        // subset the shell prompts for at launch
	Keystore           *keystoreConfig // File resolved to an absolute forward-slash path; nil = debug signing
	Widgets            []androidWidget
	MinRefreshMinutes  int // smallest widget refresh period - the worker's cadence
}

// androidWidget is one home-screen widget with launcher metadata
// translated to Android's units (grid cells and their dp equivalent,
// dp = 70*cells - 30).
type androidWidget struct {
	Name           string
	ClassName      string // Kotlin class, e.g. "StatusWidget"
	Label          string
	TargetW        int // launcher grid cells
	TargetH        int
	MinWidthDp     int
	MinHeightDp    int
	MaxWidthDp     int // 0 = no max (template omits the attrs)
	MaxHeightDp    int
	Resize         string
	RefreshMinutes int
}

// androidWidgets translates the effective widget list for the
// templates; the second result is the smallest refresh period.
func androidWidgets(appDir string, cfg appConfig) ([]androidWidget, int, error) {
	widgets, err := collectWidgets(appDir, cfg)
	if err != nil {
		return nil, 0, err
	}
	out := make([]androidWidget, 0, len(widgets))
	minRefresh := 0
	for _, w := range widgets {
		tw, th, err := parseCells(w.MinSize)
		if err != nil {
			return nil, 0, fmt.Errorf("widget %q minSize: %w", w.Name, err)
		}
		aw := androidWidget{
			Name:           w.Name,
			ClassName:      strings.ReplaceAll(title(w.Name), " ", "") + "Widget",
			Label:          w.Label,
			TargetW:        tw,
			TargetH:        th,
			MinWidthDp:     70*tw - 30,
			MinHeightDp:    70*th - 30,
			Resize:         w.Resize,
			RefreshMinutes: w.RefreshMinutes,
		}
		if w.MaxSize != "" {
			mw, mh, err := parseCells(w.MaxSize)
			if err != nil {
				return nil, 0, fmt.Errorf("widget %q maxSize: %w", w.Name, err)
			}
			aw.MaxWidthDp, aw.MaxHeightDp = 70*mw-30, 70*mh-30
		}
		if minRefresh == 0 || w.RefreshMinutes < minRefresh {
			minRefresh = w.RefreshMinutes
		}
		out = append(out, aw)
	}
	return out, minRefresh, nil
}

// parseCells parses a "CxR" launcher grid size.
func parseCells(s string) (w, h int, err error) {
	c, r, ok := strings.Cut(s, "x")
	if ok {
		w, err = strconv.Atoi(c)
		if err == nil {
			h, err = strconv.Atoi(r)
		}
	}
	if !ok || err != nil || w < 1 || h < 1 {
		return 0, 0, fmt.Errorf("bad size %q (want CxR grid cells, e.g. 3x1)", s)
	}
	return w, h, nil
}

// writeAndroidSynth (re)generates the hidden Gradle project in
// .gantry/android/ - the mobile counterpart of the Vite synth dir.
// Files are overwritten in place, never wiping the directory, so
// Gradle's build caches survive between runs. An app-owned
// mobile/android/ directory, when present, is copied on top for custom
// Kotlin or resources.
func writeAndroidSynth(appDir string, cfg appConfig) (string, error) {
	dir := filepath.Join(appDir, ".gantry", "android")

	perms, err := androidPermissions(cfg.Mobile.Permissions)
	if err != nil {
		return "", err
	}
	runtimePerms, err := androidRuntimePermissions(cfg.Mobile.Permissions)
	if err != nil {
		return "", err
	}
	code, err := cfg.versionCode()
	if err != nil {
		return "", err
	}

	s := androidScaffold{
		Name:               cfg.Name,
		Title:              cfg.Title,
		ID:                 cfg.Mobile.ID,
		Version:            cfg.Version,
		VersionCode:        code,
		MinSdk:             cfg.Mobile.Android.MinSdk,
		TargetSdk:          cfg.Mobile.Android.TargetSdk,
		Permissions:        perms,
		RuntimePermissions: runtimePerms,
	}
	if ks := cfg.Mobile.Android.Keystore; ks != nil {
		resolved := *ks
		if !filepath.IsAbs(resolved.File) {
			resolved.File = filepath.Join(appDir, resolved.File)
		}
		resolved.File = filepath.ToSlash(resolved.File)
		s.Keystore = &resolved
	}
	s.Widgets, s.MinRefreshMinutes, err = androidWidgets(appDir, cfg)
	if err != nil {
		return "", err
	}

	// Kotlin sources live under the id's package path so the generated
	// tree looks like any hand-written Android project.
	pkgDir := strings.ReplaceAll(s.ID, ".", "/")
	rendered := map[string]string{
		"settings.gradle.kts.tmpl":  "settings.gradle.kts",
		"build.gradle.kts.tmpl":     "build.gradle.kts",
		"gradle.properties.tmpl":    "gradle.properties",
		"app-build.gradle.kts.tmpl": "app/build.gradle.kts",
		"AndroidManifest.xml.tmpl":  "app/src/main/AndroidManifest.xml",
		"MainActivity.kt.tmpl":      "app/src/main/java/" + pkgDir + "/MainActivity.kt",
		"GoBackend.kt.tmpl":         "app/src/main/java/" + pkgDir + "/GoBackend.kt",
		"GantryApp.kt.tmpl":         "app/src/main/java/" + pkgDir + "/GantryApp.kt",
		"strings.xml.tmpl":          "app/src/main/res/values/strings.xml",
		"themes.xml.tmpl":           "app/src/main/res/values/themes.xml",
	}

	// Widget files regenerate from scratch: drop them all first so
	// removing a widget doesn't leave stale Kotlin/resources behind
	// (the synth otherwise never deletes - Gradle caches live here).
	for _, stale := range []string{
		"app/src/main/java/" + pkgDir + "/Widgets.kt",
		"app/src/main/java/" + pkgDir + "/WidgetRenderer.kt",
		"app/src/main/java/" + pkgDir + "/WidgetWorker.kt",
	} {
		_ = os.Remove(filepath.Join(dir, filepath.FromSlash(stale)))
	}
	if infos, err := filepath.Glob(filepath.Join(dir, "app", "src", "main", "res", "xml", "widget_*_info.xml")); err == nil {
		for _, p := range infos {
			_ = os.Remove(p)
		}
	}
	if len(s.Widgets) > 0 {
		rendered["Widgets.kt.tmpl"] = "app/src/main/java/" + pkgDir + "/Widgets.kt"
		rendered["WidgetRenderer.kt.tmpl"] = "app/src/main/java/" + pkgDir + "/WidgetRenderer.kt"
		rendered["WidgetWorker.kt.tmpl"] = "app/src/main/java/" + pkgDir + "/WidgetWorker.kt"
	}

	for tmpl, out := range rendered {
		src, err := templates.ReadFile("templates/android/" + tmpl)
		if err != nil {
			return "", fmt.Errorf("reading template android/%s: %w", tmpl, err)
		}
		// [[ ]] delimiters, same reason as the scaffold templates: the
		// file bodies legitimately contain ${...} and {{ }}-adjacent text.
		t, err := template.New(tmpl).Delims("[[", "]]").Parse(string(src))
		if err != nil {
			return "", fmt.Errorf("parsing template android/%s: %w", tmpl, err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, s); err != nil {
			return "", fmt.Errorf("rendering android/%s: %w", tmpl, err)
		}
		if err := writeFileMkdir(filepath.Join(dir, filepath.FromSlash(out)), buf.Bytes(), 0o644); err != nil {
			return "", err
		}
	}

	// Each widget's launcher metadata renders its own provider info xml.
	if len(s.Widgets) > 0 {
		src, err := templates.ReadFile("templates/android/widget_info.xml.tmpl")
		if err != nil {
			return "", fmt.Errorf("reading template android/widget_info.xml.tmpl: %w", err)
		}
		t, err := template.New("widget_info.xml.tmpl").Delims("[[", "]]").Parse(string(src))
		if err != nil {
			return "", fmt.Errorf("parsing template android/widget_info.xml.tmpl: %w", err)
		}
		for _, w := range s.Widgets {
			var buf bytes.Buffer
			if err := t.Execute(&buf, w); err != nil {
				return "", fmt.Errorf("rendering widget_%s_info.xml: %w", w.Name, err)
			}
			out := filepath.Join(dir, "app", "src", "main", "res", "xml", "widget_"+w.Name+"_info.xml")
			if err := writeFileMkdir(out, buf.Bytes(), 0o644); err != nil {
				return "", err
			}
		}
	}

	// The Gradle wrapper ships as raw assets (the jar is binary): the
	// only thing a machine needs besides the SDK/NDK is a JDK 17+.
	raw := map[string]struct {
		out  string
		mode os.FileMode
	}{
		"wrapper/gradlew":                   {"gradlew", 0o755},
		"wrapper/gradlew.bat":               {"gradlew.bat", 0o644},
		"wrapper/gradle-wrapper.jar":        {"gradle/wrapper/gradle-wrapper.jar", 0o644},
		"wrapper/gradle-wrapper.properties": {"gradle/wrapper/gradle-wrapper.properties", 0o644},
	}
	for src, dst := range raw {
		data, err := templates.ReadFile("templates/android/" + src)
		if err != nil {
			return "", fmt.Errorf("reading asset android/%s: %w", src, err)
		}
		if err := writeFileMkdir(filepath.Join(dir, filepath.FromSlash(dst.out)), data, dst.mode); err != nil {
			return "", err
		}
	}

	if overlay := filepath.Join(appDir, "mobile", "android"); dirExists(overlay) {
		info("applying mobile/android overlay")
		if err := copyTree(overlay, dir); err != nil {
			return "", fmt.Errorf("copying mobile/android overlay: %w", err)
		}
	}
	return dir, nil
}

// writeFileMkdir writes a file, creating its parent directories.
func writeFileMkdir(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}

// copyTree copies every file under src into dst, keeping relative
// paths and overwriting what is already there.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFileMkdir(filepath.Join(dst, rel), data, 0o644)
	})
}
