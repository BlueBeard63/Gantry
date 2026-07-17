package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// cmdInstall adds an optional feature to an existing app. Today the
// only feature is --tailwind: install the packages, migrate index.css
// into the @theme token structure (carrying the app's own colors
// across when it has any), flip gantry.json's "tailwind" flag and
// regenerate the synthesized vite config. gantry new does the same for
// fresh apps; this is the retrofit path.
func cmdInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	tailwind := fs.Bool("tailwind", false, "set up Tailwind v4 in this app")
	yes := fs.Bool("yes", false, "apply without prompting")
	dry := fs.Bool("dry-run", false, "report what would change without writing anything")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*tailwind {
		return fmt.Errorf("nothing to install - available features: --tailwind")
	}
	return installTailwind(*yes, *dry)
}

func installTailwind(yes, dry bool) error {
	appDir, cfg, err := findApp()
	if err != nil {
		return err
	}
	if cfg.Tailwind {
		success("tailwind is already installed (gantry.json has \"tailwind\": true)")
		return nil
	}

	// index.css migration first, so a declined prompt aborts before
	// anything else changed.
	cssPath := filepath.Join(appDir, "index.css")
	orig, err := os.ReadFile(cssPath)
	if err != nil {
		return fmt.Errorf("reading index.css: %w", err)
	}
	s := scaffoldFromConfig(cfg)
	s.Tailwind = true
	fresh, err := renderBytes("index-tailwind.css.tmpl", s)
	if err != nil {
		return err
	}
	migrated, pristine := migrateIndexCSS(orig, fresh)
	if pristine {
		info("index.css is the stock scaffold - replacing it with the Tailwind theme template")
	} else {
		info("index.css has custom colors - pulling them into an @theme block")
	}
	if dry {
		fmt.Print(unifiedDiff("index.css", orig, migrated, 120))
		info("would install tailwindcss + @tailwindcss/vite, set \"tailwind\": true and regenerate .gantry/")
		success("dry run complete - nothing written")
		return nil
	}
	if !yes {
		fmt.Print(unifiedDiff("index.css", orig, migrated, 120))
		if !askYesNo(bufio.NewReader(os.Stdin), "Rewrite index.css like this? (a .bak backup is kept)", true) {
			return fmt.Errorf("aborted - index.css not changed, nothing installed")
		}
	}
	if err := os.WriteFile(cssPath+".bak", orig, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(cssPath, migrated, 0o644); err != nil {
		return err
	}

	step("npm install -D tailwindcss @tailwindcss/vite")
	npm := "npm"
	if p, err := exec.LookPath("npm.cmd"); err == nil {
		npm = p
	}
	cmd := exec.Command(npm, "install", "-D", "tailwindcss@^4", "@tailwindcss/vite@^4")
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install failed (run it manually in %s): %w", appDir, err)
	}

	cfg.Tailwind = true
	if err := writeConfig(appDir, cfg); err != nil {
		return err
	}
	if _, err := writeSynth(appDir, cfg); err != nil {
		return err
	}

	success("tailwind installed - utilities and @theme tokens are live")
	info("your old index.css is at index.css.bak; run gantry dev to verify")
	return nil
}

// scaffoldGantryDefaults are the --gantry-* values the plain scaffold
// ships; a var still holding its default is safe for the migration to
// rewrite into a token alias.
var scaffoldGantryDefaults = map[string]string{
	"--gantry-bg":             "#101012",
	"--gantry-fg":             "#e8e8ea",
	"--gantry-fg-dim":         "#9a9aa0",
	"--gantry-accent":         "#6ea8fe",
	"--gantry-border":         "#2a2a2e",
	"--gantry-titlebar-bg":    "transparent",
	"--gantry-btn-hover":      "rgba(255, 255, 255, 0.08)",
	"--gantry-close-hover":    "#c42b1c",
	"--gantry-control-bg":     "#1a1a1e",
	"--gantry-control-border": "#34343a",
	"--gantry-radius":         "6px",
}

// gantryTokenBridge says which semantic token (by its name in the
// user's palette) drives which --gantry-* variable. Matches the wiring
// in templates/index-tailwind.css.tmpl.
var gantryTokenBridge = map[string]string{
	"--gantry-bg":             "bg-base",
	"--gantry-fg":             "text-primary",
	"--gantry-fg-dim":         "text-secondary",
	"--gantry-accent":         "accent",
	"--gantry-border":         "border-subtle",
	"--gantry-btn-hover":      "bg-hover",
	"--gantry-close-hover":    "danger-hover",
	"--gantry-control-bg":     "bg-raised",
	"--gantry-control-border": "border-default",
}

var (
	rootBlockRe  = regexp.MustCompile(`(?s):root\s*\{(.*?)\}`)
	customPropRe = regexp.MustCompile(`(--[\w-]+)\s*:\s*([^;]+);`)
)

// migrateIndexCSS turns an existing index.css into the Tailwind v4
// structure. A pristine scaffold file is replaced wholesale with the
// rendered template (pristine=true). A customized file keeps all of
// its content: the custom :root tokens are additionally exposed as
// @theme colors (utilities), and --gantry-* vars still holding their
// scaffold default are re-pointed at the matching token when the app
// defines one.
func migrateIndexCSS(orig, freshTailwind []byte) (out []byte, pristine bool) {
	src := normalizeEOL(orig)

	type prop struct{ name, value string }
	var custom []prop
	gantry := map[string]string{}
	if m := rootBlockRe.FindStringSubmatch(src); m != nil {
		for _, p := range customPropRe.FindAllStringSubmatch(m[1], -1) {
			name, value := p[1], strings.TrimSpace(p[2])
			if strings.HasPrefix(name, "--gantry-") {
				gantry[name] = value
			} else {
				custom = append(custom, prop{name, value})
			}
		}
	}

	allDefault := true
	for name, value := range gantry {
		if def, ok := scaffoldGantryDefaults[name]; !ok || def != value {
			allDefault = false
			break
		}
	}
	if len(custom) == 0 && allDefault {
		return freshTailwind, true
	}

	// @theme entries: bg-base -> --color-base, text-primary ->
	// --color-primary; other names pass through as --color-<name>.
	// A stripped name that collides with another token keeps its full
	// name.
	seen := map[string]bool{}
	for _, p := range custom {
		seen[strings.TrimPrefix(p.name, "--")] = true
	}
	themeName := func(name string) string {
		n := strings.TrimPrefix(name, "--")
		for _, prefix := range []string{"bg-", "text-"} {
			if short := strings.TrimPrefix(n, prefix); short != n && !seen[short] {
				return short
			}
		}
		return n
	}

	var theme strings.Builder
	theme.WriteString("@import \"tailwindcss\";\n@source \"./pages\";\n@source \"./components\";\n@source \"./layouts\";\n@source \"./widgets\";\n\n")
	theme.WriteString("/* The app's palette below, exposed as Tailwind utilities\n   (bg-base -> bg-base/--color-base, and so on). */\n@theme {\n")
	for _, p := range custom {
		fmt.Fprintf(&theme, "  --color-%s: var(%s);\n", themeName(p.name), p.name)
	}
	theme.WriteString("}\n\n")

	// Re-point still-default --gantry-* vars at the app's tokens.
	body := src
	for gv, token := range gantryTokenBridge {
		value, ok := gantry[gv]
		if !ok || value != scaffoldGantryDefaults[gv] || !seen[token] {
			continue
		}
		re := regexp.MustCompile(regexp.QuoteMeta(gv) + `\s*:\s*` + regexp.QuoteMeta(value) + `\s*;`)
		body = re.ReplaceAllString(body, gv+": var(--"+token+");")
	}

	return []byte(theme.String() + body), false
}
