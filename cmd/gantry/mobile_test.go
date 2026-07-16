package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAndroidPermissions(t *testing.T) {
	got, err := androidPermissions([]string{"camera", "location", "vibrate"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"android.permission.INTERNET",
		"android.permission.CAMERA",
		"android.permission.ACCESS_FINE_LOCATION",
		"android.permission.ACCESS_COARSE_LOCATION",
		"android.permission.VIBRATE",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAndroidPermissionsAlwaysInternet(t *testing.T) {
	got, err := androidPermissions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"android.permission.INTERNET"}) {
		t.Errorf("empty list should still emit INTERNET, got %v", got)
	}
}

func TestAndroidPermissionsUnknown(t *testing.T) {
	_, err := androidPermissions([]string{"camera", "telepathy"})
	if err == nil {
		t.Fatal("want error for unknown permission")
	}
	for _, frag := range []string{`"telepathy"`, "camera", "notifications", "foreground-service"} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("error should mention %s: %v", frag, err)
		}
	}
}

func TestAndroidRuntimePermissions(t *testing.T) {
	got, err := androidRuntimePermissions([]string{"camera", "vibrate", "network-state", "notifications"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"android.permission.CAMERA", "android.permission.POST_NOTIFICATIONS"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (normal permissions must not prompt)", got, want)
	}
}

func TestVersionCode(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    int
		wantErr bool
	}{
		{"0.1.0", 100, false},
		{"1.2.3", 10203, false},
		{"12.34.56", 123456, false},
		{"1.2.3-beta.1", 10203, false},
		{"not-a-version", 0, true},
		{"1.2", 0, true},
	} {
		got, err := appConfig{Version: tc.version}.versionCode()
		if tc.wantErr != (err != nil) {
			t.Errorf("%q: err = %v, wantErr %v", tc.version, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %d, want %d", tc.version, got, tc.want)
		}
	}
}

func TestResolveTargetsMobile(t *testing.T) {
	targets, err := resolveTargets("windows/amd64,android,ios,android/amd64", appConfig{})
	if err != nil {
		t.Fatal(err)
	}
	want := []buildTarget{
		{OS: "windows", Arch: "amd64"},
		{OS: "android", Arch: "arm64"}, // bare android defaults to arm64
		{OS: "ios", Arch: "arm64"},
		{OS: "android", Arch: "amd64"},
	}
	if !reflect.DeepEqual(targets, want) {
		t.Errorf("got %v, want %v", targets, want)
	}

	for _, bad := range []string{"android/386", "ios/amd64", "beos/amd64", "android/"} {
		if _, err := resolveTargets(bad, appConfig{}); err == nil {
			t.Errorf("target %q should be rejected", bad)
		}
	}
}

// fakeSDK builds an SDK tree with the given NDK versions and returns
// its root.
func fakeSDK(t *testing.T, ndkVersions ...string) string {
	t.Helper()
	sdk := t.TempDir()
	for _, v := range ndkVersions {
		dir := filepath.Join(sdk, "ndk", v)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "source.properties"), []byte("Pkg.Revision = "+v+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return sdk
}

// fakeJDK builds a JDK home whose release file reports the version.
func fakeJDK(t *testing.T, version string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "release"), []byte("IMPLEMENTOR=\"Fake\"\nJAVA_VERSION=\""+version+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestFindAndroidTools(t *testing.T) {
	sdk := fakeSDK(t, "26.1.10909125", "27.2.12479018", "9.0.0")
	t.Setenv("ANDROID_HOME", sdk)
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("ANDROID_NDK_HOME", "")
	t.Setenv("JAVA_HOME", fakeJDK(t, "21.0.1"))

	tools, missing := findAndroidTools()
	if len(missing) != 0 {
		t.Fatalf("nothing should be missing, got %v", missing)
	}
	if tools.SDK != sdk {
		t.Errorf("SDK = %q, want %q", tools.SDK, sdk)
	}
	if want := filepath.Join(sdk, "ndk", "27.2.12479018"); tools.NDK != want {
		t.Errorf("NDK = %q, want %q (highest numeric version)", tools.NDK, want)
	}
}

func TestFindAndroidToolsMissingNDK(t *testing.T) {
	t.Setenv("ANDROID_HOME", fakeSDK(t)) // SDK exists, no ndk/ installs
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("ANDROID_NDK_HOME", "")
	t.Setenv("JAVA_HOME", fakeJDK(t, "17.0.2"))

	_, missing := findAndroidTools()
	if len(missing) != 1 || !strings.Contains(missing[0], "NDK") || !strings.Contains(missing[0], "sdkmanager") {
		t.Errorf("want one NDK-missing entry with the sdkmanager fix, got %v", missing)
	}
}

func TestFindAndroidToolsNDKHomeOverride(t *testing.T) {
	ndk := t.TempDir()
	t.Setenv("ANDROID_HOME", fakeSDK(t, "26.1.10909125"))
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("ANDROID_NDK_HOME", ndk)
	t.Setenv("JAVA_HOME", fakeJDK(t, "21.0.1"))

	tools, missing := findAndroidTools()
	if len(missing) != 0 {
		t.Fatalf("nothing should be missing, got %v", missing)
	}
	if tools.NDK != ndk {
		t.Errorf("ANDROID_NDK_HOME should win, got %q", tools.NDK)
	}
}

func TestFindAndroidToolsOldJDK(t *testing.T) {
	t.Setenv("ANDROID_HOME", fakeSDK(t, "27.2.12479018"))
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("ANDROID_NDK_HOME", "")
	t.Setenv("JAVA_HOME", fakeJDK(t, "1.8.0_392"))

	_, missing := findAndroidTools()
	if len(missing) != 1 || !strings.Contains(missing[0], "JDK 17+") {
		t.Errorf("a JDK 8 home should report the JDK as missing, got %v", missing)
	}
}

func TestJavaMajorVersion(t *testing.T) {
	for version, want := range map[string]int{"21.0.1": 21, "17": 17, "1.8.0_392": 8} {
		if got := javaMajorVersion(fakeJDK(t, version)); got != want {
			t.Errorf("%q: got %d, want %d", version, got, want)
		}
	}
	if got := javaMajorVersion(t.TempDir()); got != 0 {
		t.Errorf("home without release file: got %d, want 0", got)
	}
}

func TestNDKClangPath(t *testing.T) {
	tools := androidTools{NDK: filepath.Join("sdk", "ndk", "27.2.12479018")}
	p := tools.clang("arm64", 26)
	if !strings.Contains(p, "aarch64-linux-android26-clang") {
		t.Errorf("clang path = %q", p)
	}
	if !strings.Contains(p, filepath.Join("toolchains", "llvm", "prebuilt")) {
		t.Errorf("clang path should live under toolchains/llvm/prebuilt: %q", p)
	}
}

// synthConfig is a ready-to-synth appConfig with the findApp defaults
// already applied (writeAndroidSynth assumes them).
func synthConfig() appConfig {
	return appConfig{
		Name:    "demo",
		Title:   "Demo",
		Version: "1.2.3",
		Mobile: &mobileConfig{
			ID:          "ec.morrison.demo",
			Permissions: []string{"camera", "vibrate"},
			Android:     &androidConfig{MinSdk: 26, TargetSdk: 35},
		},
	}
}

func TestWriteAndroidSynth(t *testing.T) {
	appDir := t.TempDir()
	dir, err := writeAndroidSynth(appDir, synthConfig())
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(appDir, ".gantry", "android"); dir != want {
		t.Fatalf("synth dir = %q, want %q", dir, want)
	}

	checks := map[string][]string{
		"settings.gradle.kts": {`rootProject.name = "demo"`, `include(":app")`},
		"build.gradle.kts":    {`id("com.android.application") version`},
		"app/build.gradle.kts": {
			`applicationId = "ec.morrison.demo"`,
			"versionCode = 10203",
			`versionName = "1.2.3"`,
			"minSdk = 26",
			"targetSdk = 35",
			"useLegacyPackaging = true",
			`signingConfig = signingConfigs.getByName("debug")`,
		},
		"app/src/main/AndroidManifest.xml": {
			`android:name="android.permission.INTERNET"`,
			`android:name="android.permission.CAMERA"`,
			`android:name="android.permission.VIBRATE"`,
			`android:usesCleartextTraffic="true"`,
		},
		"app/src/main/java/ec/morrison/demo/MainActivity.kt": {
			"package ec.morrison.demo",
			`"android.permission.CAMERA",`, // runtime prompt list: camera yes...
		},
		"app/src/main/java/ec/morrison/demo/GoBackend.kt":  {"libgantryapp.so", "GANTRY_READY"},
		"app/src/main/java/ec/morrison/demo/GantryApp.kt":  {"class GantryApp"},
		"app/src/main/res/values/strings.xml":              {"<string name=\"app_name\">Demo</string>"},
		"gradlew.bat":                                      nil,
		"gradle/wrapper/gradle-wrapper.jar":                nil,
		"gradle/wrapper/gradle-wrapper.properties":         {"gradle-8.11.1-bin.zip"},
	}
	for rel, frags := range checks {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("missing synth file %s: %v", rel, err)
			continue
		}
		for _, frag := range frags {
			if !strings.Contains(string(data), frag) {
				t.Errorf("%s should contain %q", rel, frag)
			}
		}
	}

	// ...but VIBRATE is a normal permission: declared, never prompted.
	main, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash("app/src/main/java/ec/morrison/demo/MainActivity.kt")))
	if strings.Contains(string(main), `"android.permission.VIBRATE",`) {
		t.Error("MainActivity should not runtime-prompt for VIBRATE")
	}
}

func TestWriteAndroidSynthKeystore(t *testing.T) {
	appDir := t.TempDir()
	cfg := synthConfig()
	cfg.Mobile.Android.Keystore = &keystoreConfig{File: "release.jks", Alias: "app", PasswordEnv: "DEMO_KEY_PASS"}
	dir, err := writeAndroidSynth(appDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "app", "build.gradle.kts"))
	if err != nil {
		t.Fatal(err)
	}
	gradle := string(data)
	for _, frag := range []string{
		`file("` + filepath.ToSlash(filepath.Join(appDir, "release.jks")) + `")`,
		`keyAlias = "app"`,
		`System.getenv("DEMO_KEY_PASS")`,
		`signingConfig = signingConfigs.getByName("release")`,
	} {
		if !strings.Contains(gradle, frag) {
			t.Errorf("app/build.gradle.kts should contain %q", frag)
		}
	}
}

func TestWriteAndroidSynthOverlay(t *testing.T) {
	appDir := t.TempDir()
	overlay := filepath.Join(appDir, "mobile", "android", "app", "src", "main", "res", "values")
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "<resources><string name=\"app_name\">Custom</string></resources>\n"
	if err := os.WriteFile(filepath.Join(overlay, "strings.xml"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	dir, err := writeAndroidSynth(appDir, synthConfig())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "app", "src", "main", "res", "values", "strings.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Errorf("overlay should win over the synthesized strings.xml, got %q", data)
	}
}

// widgetAppDir builds a temp app with a widgets/<name> pair and go.mod.
func widgetAppDir(t *testing.T, names ...string) string {
	t.Helper()
	appDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(appDir, "go.mod"), []byte("module demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		dir := filepath.Join(appDir, "widgets", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		src := "package " + name + "\n\nimport \"github.com/B-Commissions/Gantry/widget\"\n\nvar Widget = widget.New(\"" + name + "\", nil)\n"
		if err := os.WriteFile(filepath.Join(dir, name+".go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return appDir
}

func TestCollectWidgetsDefaults(t *testing.T) {
	appDir := widgetAppDir(t, "status")
	// Go pair without a gantry.json entry: registered with defaults.
	widgets, err := collectWidgets(appDir, appConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(widgets) != 1 {
		t.Fatalf("got %d widgets, want 1", len(widgets))
	}
	w := widgets[0]
	if w.Name != "status" || w.Label != "Status" || w.MinSize != "2x1" || w.Resize != "horizontal|vertical" || w.RefreshMinutes != 30 {
		t.Errorf("defaults not applied: %+v", w)
	}
}

func TestCollectWidgetsEntryWithoutPair(t *testing.T) {
	appDir := widgetAppDir(t) // no pairs at all
	cfg := appConfig{Mobile: &mobileConfig{ID: "x.y", Widgets: []mobileWidget{{Name: "status"}}}}
	if _, err := collectWidgets(appDir, cfg); err == nil || !strings.Contains(err.Error(), "widgets/status/status.go") {
		t.Errorf("want pairing error naming the missing file, got %v", err)
	}
}

func TestCollectWidgetsRefreshFloor(t *testing.T) {
	appDir := widgetAppDir(t, "status")
	cfg := appConfig{Mobile: &mobileConfig{ID: "x.y", Widgets: []mobileWidget{{Name: "status", RefreshMinutes: 5}}}}
	widgets, err := collectWidgets(appDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if widgets[0].RefreshMinutes != 15 {
		t.Errorf("refresh should floor at 15, got %d", widgets[0].RefreshMinutes)
	}
}

func TestWriteWidgetRegistry(t *testing.T) {
	appDir := widgetAppDir(t, "status", "clock")
	if err := writeWidgetRegistry(appDir, appConfig{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(appDir, "gantry_widgets.go"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, frag := range []string{
		"Code generated by gantry gen",
		`w_clock "demo/widgets/clock"`,
		`w_status "demo/widgets/status"`,
		"gantry.RegisterWidgets(",
		"w_clock.Widget,",
		"w_status.Widget,",
	} {
		if !strings.Contains(got, frag) {
			t.Errorf("gantry_widgets.go should contain %q:\n%s", frag, got)
		}
	}

	// Removing every widget removes the generated file.
	if err := os.RemoveAll(filepath.Join(appDir, "widgets")); err != nil {
		t.Fatal(err)
	}
	if err := writeWidgetRegistry(appDir, appConfig{}); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(appDir, "gantry_widgets.go")) {
		t.Error("stale gantry_widgets.go should be removed when the last widget goes")
	}
}

func TestParseCells(t *testing.T) {
	if w, h, err := parseCells("3x1"); err != nil || w != 3 || h != 1 {
		t.Errorf("3x1 = %d,%d,%v", w, h, err)
	}
	for _, bad := range []string{"", "3", "x1", "3x0", "axb"} {
		if _, _, err := parseCells(bad); err == nil {
			t.Errorf("parseCells(%q) should fail", bad)
		}
	}
}

func TestWriteAndroidSynthWidgets(t *testing.T) {
	appDir := widgetAppDir(t, "status")
	cfg := synthConfig()
	cfg.Mobile.Widgets = []mobileWidget{{Name: "status", Label: "Demo Status", MinSize: "3x1", MaxSize: "5x2", RefreshMinutes: 45}}
	dir, err := writeAndroidSynth(appDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	checks := map[string][]string{
		"app/src/main/java/ec/morrison/demo/Widgets.kt": {
			"class StatusWidgetReceiver", "class StatusWidget", `WidgetStore.load(context, "status")`,
		},
		"app/src/main/java/ec/morrison/demo/WidgetRenderer.kt": {"GantryWidgetContent"},
		"app/src/main/java/ec/morrison/demo/WidgetWorker.kt":   {"PeriodicWorkRequestBuilder<WidgetWorker>(45, TimeUnit.MINUTES)"},
		"app/src/main/res/xml/widget_status_info.xml": {
			`android:minWidth="180dp"`, `android:minHeight="40dp"`,
			`android:targetCellWidth="3"`, `android:targetCellHeight="1"`,
			`android:maxResizeWidth="320dp"`, `android:maxResizeHeight="110dp"`,
			`android:resizeMode="horizontal|vertical"`,
		},
		"app/src/main/AndroidManifest.xml": {".StatusWidgetReceiver", "@xml/widget_status_info", `android:label="Demo Status"`},
		"app/build.gradle.kts":             {"glance-appwidget", "work-runtime-ktx", `id("org.jetbrains.kotlin.plugin.compose")`},
		"app/src/main/java/ec/morrison/demo/GantryApp.kt": {"WidgetWorker.schedule(this)"},
		"app/src/main/java/ec/morrison/demo/GoBackend.kt": {"WidgetRefresh.fromServer(context, newPort, token)"},
	}
	for rel, frags := range checks {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("missing synth file %s: %v", rel, err)
			continue
		}
		for _, frag := range frags {
			if !strings.Contains(string(data), frag) {
				t.Errorf("%s should contain %q", rel, frag)
			}
		}
	}

	// Dropping the widget cleans every generated widget file up.
	cfg.Mobile.Widgets = nil
	if err := os.RemoveAll(filepath.Join(appDir, "widgets")); err != nil {
		t.Fatal(err)
	}
	if _, err := writeAndroidSynth(appDir, cfg); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"app/src/main/java/ec/morrison/demo/Widgets.kt",
		"app/src/main/java/ec/morrison/demo/WidgetRenderer.kt",
		"app/src/main/java/ec/morrison/demo/WidgetWorker.kt",
		"app/src/main/res/xml/widget_status_info.xml",
	} {
		if fileExists(filepath.Join(dir, filepath.FromSlash(rel))) {
			t.Errorf("stale widget file %s should be removed", rel)
		}
	}
	manifest, _ := os.ReadFile(filepath.Join(dir, "app", "src", "main", "AndroidManifest.xml"))
	if strings.Contains(string(manifest), "Receiver") {
		t.Error("manifest should have no widget receivers left")
	}
	gradle, _ := os.ReadFile(filepath.Join(dir, "app", "build.gradle.kts"))
	if strings.Contains(string(gradle), "glance") {
		t.Error("gradle should have no glance dependency left")
	}
}
