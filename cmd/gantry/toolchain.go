package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// ndkVersion is the NDK the docs tell people to install; detection
// accepts any version present.
const ndkVersion = "27.2.12479018"

// androidTools is everything an APK build needs from the machine.
type androidTools struct {
	SDK      string // Android SDK root
	NDK      string // NDK root (holds toolchains/llvm/...)
	JavaHome string // JDK 17+ home for Gradle
}

// findAndroidTools locates the Android SDK, NDK and a JDK 17+. Instead
// of an error it returns the list of missing pieces, each with a fix
// hint - the caller warns and skips the android target, never fails.
func findAndroidTools() (androidTools, []string) {
	var t androidTools
	var missing []string

	t.SDK = firstDir(os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT"))
	if t.SDK == "" {
		t.SDK = firstDir(sdkGuesses()...)
	}
	if t.SDK == "" {
		missing = append(missing, "Android SDK (fix: install Android Studio or the cmdline-tools, or set ANDROID_HOME)")
	}

	if ndk := os.Getenv("ANDROID_NDK_HOME"); dirExists(ndk) {
		t.NDK = ndk
	} else if t.SDK != "" {
		t.NDK = newestNDK(filepath.Join(t.SDK, "ndk"))
	}
	if t.NDK == "" {
		missing = append(missing, fmt.Sprintf("Android NDK (fix: sdkmanager %q, or set ANDROID_NDK_HOME)", "ndk;"+ndkVersion))
	}

	t.JavaHome = findJDK()
	if t.JavaHome == "" {
		missing = append(missing, "JDK 17+ (fix: install a JDK 17+ and set JAVA_HOME or put java on PATH)")
	}

	return t, missing
}

// clang returns the NDK's per-target C compiler wrapper for a Go arch,
// e.g. .../bin/aarch64-linux-android26-clang(.cmd on Windows).
func (t androidTools) clang(goarch string, api int) string {
	triple := map[string]string{"arm64": "aarch64-linux-android", "amd64": "x86_64-linux-android"}[goarch]
	host := map[string]string{"windows": "windows-x86_64", "darwin": "darwin-x86_64", "linux": "linux-x86_64"}[runtime.GOOS]
	name := fmt.Sprintf("%s%d-clang", triple, api)
	if runtime.GOOS == "windows" {
		name += ".cmd"
	}
	return filepath.Join(t.NDK, "toolchains", "llvm", "prebuilt", host, "bin", name)
}

// sdkGuesses is where Android Studio installs the SDK per OS.
func sdkGuesses() []string {
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return []string{filepath.Join(base, "Android", "Sdk")}
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return []string{filepath.Join(home, "Library", "Android", "sdk")}
		}
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return []string{filepath.Join(home, "Android", "Sdk")}
		}
	}
	return nil
}

// newestNDK picks the highest-versioned install under <sdk>/ndk that
// looks real (has source.properties).
func newestNDK(ndkRoot string) string {
	entries, err := os.ReadDir(ndkRoot)
	if err != nil {
		return ""
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && fileExists(filepath.Join(ndkRoot, e.Name(), "source.properties")) {
			versions = append(versions, e.Name())
		}
	}
	if len(versions) == 0 {
		return ""
	}
	sort.Slice(versions, func(i, j int) bool { return versionLess(versions[i], versions[j]) })
	return filepath.Join(ndkRoot, versions[len(versions)-1])
}

// versionLess compares dotted numeric versions of any length
// ("26.1.10909125" < "27.2.12479018"); non-numeric parts compare as 0.
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var na, nb int
		if i < len(pa) {
			na, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			nb, _ = strconv.Atoi(pb[i])
		}
		if na != nb {
			return na < nb
		}
	}
	return false
}

// findJDK returns a JDK home of major version 17+: JAVA_HOME first,
// then the home derived from java on PATH. A home whose version can't
// be read (no release file) is trusted - Gradle gives the real error.
func findJDK() string {
	if home := os.Getenv("JAVA_HOME"); dirExists(home) {
		if v := javaMajorVersion(home); v == 0 || v >= 17 {
			return home
		}
		return ""
	}
	java, err := exec.LookPath("java")
	if err != nil {
		return ""
	}
	home := filepath.Dir(filepath.Dir(java))
	if v := javaMajorVersion(home); v == 0 || v >= 17 {
		return home
	}
	return ""
}

// javaMajorVersion reads the JDK's <home>/release file and parses
// JAVA_VERSION="21.0.1" down to its major number; 0 = unknown.
func javaMajorVersion(home string) int {
	data, err := os.ReadFile(filepath.Join(home, "release"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		val, ok := strings.CutPrefix(strings.TrimSpace(line), "JAVA_VERSION=")
		if !ok {
			continue
		}
		val = strings.Trim(val, `"`)
		// Old JDKs report "1.8.0_392"-style versions: the major is the
		// second field there, the first everywhere else.
		parts := strings.SplitN(val, ".", 3)
		if len(parts) > 1 && parts[0] == "1" {
			parts[0] = parts[1]
		}
		major, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0
		}
		return major
	}
	return 0
}

// firstDir returns the first argument that is an existing directory.
func firstDir(candidates ...string) string {
	for _, c := range candidates {
		if dirExists(c) {
			return c
		}
	}
	return ""
}

func dirExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
