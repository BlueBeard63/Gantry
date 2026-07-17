package gantrytest

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// WidgetSnapshot runs the app binary on the host with --emit-widgets -
// no device, no server, the PR-gate tier of widget testing - and
// returns the named widget's rendered tree. Assert on it directly, or
// golden-file it with Golden.
func WidgetSnapshot(t testing.TB, name string, opts ...Option) json.RawMessage {
	t.Helper()

	cfg := launchConfig{mode: envDefault("GANTRY_TEST_MODE", "development")}
	for _, o := range opts {
		o(&cfg)
	}
	appDir := cfg.appDir
	if appDir == "" {
		appDir = os.Getenv("GANTRY_TEST_APP_DIR")
	}
	if appDir == "" {
		var err error
		appDir, err = findAppDir(".")
		if err != nil {
			t.Fatalf("gantrytest: %v", err)
		}
	}
	appCfg, err := readAppConfig(appDir)
	if err != nil {
		t.Fatalf("gantrytest: %v", err)
	}
	bin := cfg.binPath
	if bin == "" {
		bin = testBinary(t, appDir, appCfg)
	}

	cmd := exec.Command(bin, "--emit-widgets")
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(), "GANTRY_MODE="+cfg.mode)
	cmd.Env = append(cmd.Env, argEnv(t, appCfg, cfg.args)...)
	cmd.Env = append(cmd.Env, cfg.env...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("gantrytest: %s --emit-widgets: %v", bin, err)
	}

	var envelope struct {
		Version int `json:"version"`
		Widgets []struct {
			Name string          `json:"name"`
			Root json.RawMessage `json:"root"`
		} `json:"widgets"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("gantrytest: parsing --emit-widgets output: %v\n%s", err, out)
	}
	for _, w := range envelope.Widgets {
		if w.Name == name {
			return w.Root
		}
	}
	var have []string
	for _, w := range envelope.Widgets {
		have = append(have, w.Name)
	}
	t.Fatalf("gantrytest: no widget %q in the snapshot (have %v)", name, have)
	return nil
}

// Golden compares got against testdata/<name>.golden.json next to the
// test files. Run with GANTRY_UPDATE_GOLDENS=1 (or gantry test
// --update) to rewrite the file instead. Only golden stable widgets -
// a widget that renders the clock will never match.
func Golden(t testing.TB, name string, got json.RawMessage) {
	t.Helper()
	want := normalizeJSON(t, got)
	path := filepath.Join("testdata", name+".golden.json")

	if os.Getenv("GANTRY_UPDATE_GOLDENS") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("gantrytest: %v", err)
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatalf("gantrytest: writing %s: %v", path, err)
		}
		t.Logf("gantrytest: updated %s", path)
		return
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("gantrytest: reading %s: %v (run gantry test --update to create it)", path, err)
	}
	if !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(want)) {
		t.Fatalf("gantrytest: %s does not match the golden file %s\ngot:\n%s\nwant:\n%s\n(run gantry test --update to accept the change)",
			name, path, want, bytes.TrimSpace(existing))
	}
}

func normalizeJSON(t testing.TB, raw json.RawMessage) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("gantrytest: normalizing JSON: %v", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("gantrytest: normalizing JSON: %v", err)
	}
	return append(out, '\n')
}
