package main

import (
	"strings"
	"testing"
)

const pristineIndexCSS = `/* App-wide styles. */

:root {
  --gantry-bg: #101012;
  --gantry-fg: #e8e8ea;
  --gantry-fg-dim: #9a9aa0;
  --gantry-accent: #6ea8fe;
  --gantry-border: #2a2a2e;
  --gantry-titlebar-bg: transparent;
  --gantry-btn-hover: rgba(255, 255, 255, 0.08);
  --gantry-close-hover: #c42b1c;
  --gantry-control-bg: #1a1a1e;
  --gantry-control-border: #34343a;
  --gantry-radius: 6px;
}

* {
  box-sizing: border-box;
}
`

func TestMigrateIndexCSSPristine(t *testing.T) {
	fresh := []byte("@import \"tailwindcss\";\n/* fresh */\n")
	out, pristine := migrateIndexCSS([]byte(pristineIndexCSS), fresh)
	if !pristine {
		t.Fatal("stock scaffold not detected as pristine")
	}
	if string(out) != string(fresh) {
		t.Error("pristine file should be replaced with the rendered template")
	}
}

func TestMigrateIndexCSSCustomTokens(t *testing.T) {
	custom := `:root {
  --gantry-bg: #101012;
  --gantry-accent: #6ea8fe;
  --gantry-close-hover: #c42b1c;
  --gantry-control-bg: #123456;

  --bg-base: #0E0F13FF;
  --text-primary: #F3F5FAFF;
  --accent: #E7E9EFFF;
  --danger-hover: #454B5CFF;
  --accent-pill: #E7E9EF29;
}

.my-rule { color: red; }
`
	out, pristine := migrateIndexCSS([]byte(custom), []byte("FRESH"))
	if pristine {
		t.Fatal("customized file detected as pristine")
	}
	got := string(out)

	// @theme exposes the tokens with mapped names.
	for _, needle := range []string{
		`@import "tailwindcss";`,
		"--color-base: var(--bg-base);",
		"--color-primary: var(--text-primary);",
		"--color-accent: var(--accent);",
		"--color-danger-hover: var(--danger-hover);",
		"--color-accent-pill: var(--accent-pill);",
	} {
		if !strings.Contains(got, needle) {
			t.Errorf("missing %q in:\n%s", needle, got)
		}
	}
	// Default gantry values with a matching token get re-pointed.
	for _, needle := range []string{
		"--gantry-bg: var(--bg-base);",
		"--gantry-accent: var(--accent);",
		"--gantry-close-hover: var(--danger-hover);",
	} {
		if !strings.Contains(got, needle) {
			t.Errorf("gantry bridge missing %q in:\n%s", needle, got)
		}
	}
	// A hand-customized gantry value must be left alone.
	if !strings.Contains(got, "--gantry-control-bg: #123456;") {
		t.Errorf("customized gantry value was rewritten:\n%s", got)
	}
	// Everything else survives.
	if !strings.Contains(got, ".my-rule { color: red; }") {
		t.Errorf("user rule lost:\n%s", got)
	}
}

func TestMigrateIndexCSSNameCollision(t *testing.T) {
	custom := `:root {
  --bg-hover: #111111;
  --hover: #222222;
}
`
	out, _ := migrateIndexCSS([]byte(custom), nil)
	got := string(out)
	// --hover claims the short name; --bg-hover must keep its full name.
	if !strings.Contains(got, "--color-hover: var(--hover);") ||
		!strings.Contains(got, "--color-bg-hover: var(--bg-hover);") {
		t.Errorf("collision handling wrong:\n%s", got)
	}
}

func TestScaffoldTailwindVariant(t *testing.T) {
	s := scaffold{Tailwind: true}
	var found bool
	for _, f := range scaffoldFiles(s) {
		if f.out == "index.css" {
			found = true
			if f.tmpl != "index-tailwind.css.tmpl" {
				t.Errorf("tailwind scaffold picks %s", f.tmpl)
			}
		}
	}
	if !found {
		t.Fatal("no index.css entry")
	}
	for _, f := range scaffoldFiles(scaffold{}) {
		if f.out == "index.css" && f.tmpl != "index-plain.css.tmpl" {
			t.Errorf("plain scaffold picks %s", f.tmpl)
		}
	}

	// The tailwind template renders with both @theme and the bridge.
	data, err := renderBytes("index-tailwind.css.tmpl", s)
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)
	if !strings.Contains(css, "@theme") || !strings.Contains(css, "--gantry-bg: var(--color-base);") {
		t.Errorf("tailwind template missing @theme or bridge:\n%.400s", css)
	}

	// scaffoldFromConfig round-trips the flag.
	var cfg appConfig
	cfg.Tailwind = true
	if !scaffoldFromConfig(cfg).Tailwind {
		t.Error("scaffoldFromConfig drops Tailwind")
	}
}
