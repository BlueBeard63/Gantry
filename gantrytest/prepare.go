package gantrytest

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Environment contract with `gantry test` (all optional - the driver
// works under a plain `go test ./tests/...` too, deriving everything
// itself):
//
//	GANTRY_TEST_APP_DIR         the app root (default: walk up to gantry.json)
//	GANTRY_TEST_BIN             prebuilt app binary (default: go build once per run)
//	GANTRY_TEST_ARTIFACTS       artifact root (default: <app>/test-results)
//	GANTRY_TEST_MODE            default app mode (default: development)
//	GANTRY_TEST_HEADED          "1": launch with the real window
//	GANTRY_TEST_KEEP_ARTIFACTS  "1": keep passing tests' artifacts too
//	GANTRY_TEST_DEVICE          target device ("" = desktop; "android" or "android:SERIAL")
//	GANTRY_TEST_ADB             adb binary for the device target (default: PATH, then ANDROID_HOME)
//	GANTRY_TEST_SERIAL          device serial (default: the sole connected device)
//	GANTRY_TEST_APP_ID          installed application id (default: gantry.json mobile.id)
//	GANTRY_TEST_ALLOW_CLEAR     "1": pm clear before each test instance (hermetic device runs)
//	GANTRY_UPDATE_GOLDENS       "1": rewrite golden files instead of comparing

// appConfig is the slice of gantry.json the driver needs: identity for
// the binary and config dir, the version stamp, and the arg specs that
// map WithArgs names to environment variables.
type appConfig struct {
	Name    string             `json:"name"`
	Version string             `json:"version"`
	Args    map[string]argSpec `json:"args"`
	Mobile  *struct {
		ID string `json:"id"`
	} `json:"mobile"`
}

type argSpec struct {
	Type string `json:"type"`
	Env  string `json:"env"`
}

// findAppDir walks up from dir to the nearest gantry.json - tests run
// in tests/, so the app root is a parent.
func findAppDir(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "gantry.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no gantry.json found in %s or any parent (are the tests inside a gantry app?)", startDir)
		}
		dir = parent
	}
}

func readAppConfig(appDir string) (appConfig, error) {
	var cfg appConfig
	data, err := os.ReadFile(filepath.Join(appDir, "gantry.json"))
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", filepath.Join(appDir, "gantry.json"), err)
	}
	if cfg.Version == "" {
		cfg.Version = "0.1.0"
	}
	return cfg, nil
}

// argEnvName mirrors the CLI's mapping: the spec's explicit env name,
// or GANTRY_ARG_<UPPER_SNAKE> from the arg name.
func argEnvName(name string, spec argSpec) string {
	if spec.Env != "" {
		return spec.Env
	}
	return "GANTRY_ARG_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// argEnv renders WithArgs values as environment assignments, validated
// against the app's declared specs so a typoed arg name fails the test
// instead of silently doing nothing.
func argEnv(t testing.TB, cfg appConfig, args map[string]any) []string {
	t.Helper()
	out := make([]string, 0, len(args))
	for name, val := range args {
		spec, ok := cfg.Args[name]
		if !ok {
			t.Fatalf("gantrytest: WithArgs(%q): app declares no such arg in gantry.json", name)
		}
		var s string
		switch v := val.(type) {
		case string:
			s = v
		case bool:
			s = strconv.FormatBool(v)
		case int:
			s = strconv.Itoa(v)
		case float64:
			s = strconv.FormatFloat(v, 'f', -1, 64)
		default:
			t.Fatalf("gantrytest: WithArgs(%q): unsupported value type %T", name, val)
		}
		out = append(out, argEnvName(name, spec)+"="+s)
	}
	return out
}

// One binary per app dir per test-process run: `gantry test` prebuilds
// and passes GANTRY_TEST_BIN; a bare `go test` builds here once and
// every parallel test reuses it. The frontend must already be built
// (webdist/) - `gantry test` guarantees that; under bare `go test` the
// last dev/build's frontend is embedded.
var builds sync.Map // appDir -> *buildOnce

type buildOnce struct {
	once sync.Once
	path string
	err  error
}

// injectVersion is the ldflags -X target that stamps gantry.json's
// version into the runtime (what gantry build does).
const injectVersion = "github.com/BlueBeard63/Gantry/gantry.injectedVersion"

// testBinary returns the app binary to launch, building it if this is
// the first Launch of the run.
func testBinary(t testing.TB, appDir string, cfg appConfig) string {
	t.Helper()
	if p := os.Getenv("GANTRY_TEST_BIN"); p != "" {
		return p
	}
	v, _ := builds.LoadOrStore(appDir, &buildOnce{})
	b := v.(*buildOnce)
	b.once.Do(func() {
		b.path, b.err = buildApp(appDir, cfg)
	})
	if b.err != nil {
		t.Fatalf("gantrytest: building the app: %v", b.err)
	}
	return b.path
}

// testBinaryPath is where the app under test is built - shared between
// the driver's own build and `gantry test`'s prebuild.
func testBinaryPath(appDir, name string) string {
	exe := name
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	return filepath.Join(appDir, ".gantry", "test", exe)
}

func buildApp(appDir string, cfg appConfig) (string, error) {
	out := testBinaryPath(appDir, cfg.Name)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("go", "build", "-o", out,
		"-ldflags", "-X "+injectVersion+"="+cfg.Version, ".")
	cmd.Dir = appDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build in %s: %w\n%s", appDir, err, output)
	}
	return out, nil
}
