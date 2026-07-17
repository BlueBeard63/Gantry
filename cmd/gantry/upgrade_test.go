package main

import (
	"strings"
	"testing"
)

func TestUnifiedDiff(t *testing.T) {
	have := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\n")
	want := []byte("line1\nline2\nline3\nline4\nCHANGED\nline6\nline7\nline8\nline9\n")
	d := unifiedDiff("f.txt", have, want, 120)
	for _, needle := range []string{"- line5", "+ CHANGED", "  line4", "  line6"} {
		if !strings.Contains(d, needle) {
			t.Errorf("diff missing %q:\n%s", needle, d)
		}
	}
	if strings.Contains(d, "line1") || strings.Contains(d, "line9") {
		t.Errorf("diff should elide lines outside context:\n%s", d)
	}
	if !strings.Contains(d, "...") {
		t.Errorf("elision marker missing:\n%s", d)
	}
}

func TestUnifiedDiffCRLF(t *testing.T) {
	// CRLF on disk vs LF template must not spew a whole-file diff.
	have := []byte("a\r\nb\r\nX\r\n")
	want := []byte("a\nb\nc\n")
	d := unifiedDiff("f.txt", have, want, 120)
	if !strings.Contains(d, "- X") || !strings.Contains(d, "+ c") {
		t.Errorf("unexpected diff:\n%s", d)
	}
	if strings.Contains(d, "- a") {
		t.Errorf("CRLF-only difference reported as change:\n%s", d)
	}
}

func TestUnifiedDiffTruncation(t *testing.T) {
	var a, b strings.Builder
	for i := 0; i < 500; i++ {
		a.WriteString("old\n")
		b.WriteString("new\n")
	}
	d := unifiedDiff("f.txt", []byte(a.String()), []byte(b.String()), 10)
	if !strings.Contains(d, "truncated") {
		t.Errorf("large diff not truncated:\n%.500s", d)
	}
}

func TestNormalizeEOL(t *testing.T) {
	if normalizeEOL([]byte("a\r\nb\n")) != "a\nb\n" {
		t.Error("normalizeEOL did not fold CRLF")
	}
}

func TestScaffoldFromConfig(t *testing.T) {
	// Round-trip: the mapping cmdNew writes into gantry.json must
	// reconstruct the same scaffold flags.
	var cfg appConfig
	cfg.Name, cfg.Title, cfg.Port = "myapp", "My App", 9001
	cfg.Mode, cfg.Style, cfg.Tray = "multi", "tea", true
	cfg.Buttons.Minimize, cfg.Buttons.Close = true, true

	s := scaffoldFromConfig(cfg)
	if !s.Multi || !s.Tea || !s.Tray || !s.BtnMin || s.BtnMax || !s.BtnClose {
		t.Errorf("flags mismatch: %+v", s)
	}
	if s.Name != "myapp" || s.Title != "My App" || s.Port != 9001 {
		t.Errorf("identity mismatch: %+v", s)
	}

	// Single/plain apps must not be offered the multi-only files.
	cfg.Mode, cfg.Style = "single", "plain"
	s = scaffoldFromConfig(cfg)
	for _, f := range scaffoldFiles(s) {
		if strings.HasPrefix(f.out, "pages/settings/") && f.when {
			t.Error("single-mode scaffold offers pages/settings")
		}
		if f.out == "pages/index/index.go" && !strings.Contains(f.tmpl, "plain") {
			t.Errorf("plain style renders %s", f.tmpl)
		}
	}
}

func TestWebDepPin(t *testing.T) {
	pkg := []byte(`{
  "dependencies": {
    "gantry-web": "latest",
    "recharts": "^2.0.0"
  }
}`)
	m := webDepRe.FindSubmatch(pkg)
	if m == nil {
		t.Fatal("gantry-web dependency not matched")
	}
	next := string(webDepRe.ReplaceAll(pkg, []byte("${1}\"0.3.4\"")))
	if !strings.Contains(next, `"gantry-web": "0.3.4"`) {
		t.Errorf("pin not applied:\n%s", next)
	}
	if !strings.Contains(next, `"recharts": "^2.0.0"`) {
		t.Errorf("other dependencies disturbed:\n%s", next)
	}

	link := []byte(`{"dependencies": {"gantry-web": "file:D:/x/web"}}`)
	if lm := webDepRe.FindSubmatch(link); lm == nil || !strings.HasPrefix(string(lm[2]), "file:") {
		t.Error("file: link not detected")
	}
}

func TestScaffoldGantryWebDep(t *testing.T) {
	s := scaffold{WebVersion: "0.3.4"}
	if s.GantryWebDep() != "0.3.4" {
		t.Errorf("pinned dep = %q", s.GantryWebDep())
	}
	s.GantryDir = "D:/checkout"
	if s.GantryWebDep() != "file:D:/checkout/web" {
		t.Errorf("linked dep = %q", s.GantryWebDep())
	}
	if (scaffold{}).GantryWebDep() != "latest" {
		t.Error("fallback dep should be latest")
	}
}
