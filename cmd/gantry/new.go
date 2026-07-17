package main

import (
	"bufio"
	"bytes"
	"embed"
	"flag"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/B-Commissions/Gantry/appicon"
)

//go:embed templates
var templates embed.FS

// scaffold is the data every template renders with.
type scaffold struct {
	Name       string // module + exe name, e.g. "myapp"
	Title      string // window title, e.g. "My App"
	Port       int
	Tray       bool
	Multi      bool
	Tea        bool
	BtnMin     bool
	BtnMax     bool
	BtnClose   bool
	GantryDir  string // forward-slash path to the local Gantry checkout ("" = none, use GOPRIVATE)
	WebVersion string // exact gantry-web npm version to pin ("" = fall back to "latest")
}

func (s scaffold) GantryWeb() string { return s.GantryDir + "/web" }

// GantryWebDep is package.json's gantry-web value: a file: link in
// framework-development mode, otherwise an exact version pinned to the
// CLI - the CLI's synthesized entry and the package must move in
// lockstep, so a floating "latest" is only the last resort (dev CLI
// builds with the proxy unreachable).
func (s scaffold) GantryWebDep() string {
	if s.GantryDir != "" {
		return "file:" + s.GantryWeb()
	}
	if s.WebVersion != "" {
		return s.WebVersion
	}
	return "latest"
}

// webPinVersion is the npm version new/upgrade pin gantry-web to: the
// CLI's own release tag, or the newest known tag for dev CLI builds.
// Bare (no "v"), as npm versions are written.
func webPinVersion() string {
	v := taggedVersion()
	if v == "" {
		v = latestVersionFresh()
	}
	return strings.TrimPrefix(v, "v")
}

func cmdNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	dir := fs.String("dir", "", "parent directory for the app (default: current directory)")
	buttons := fs.String("buttons", "", "comma list of topbar buttons: minimize,maximize,close (skips the prompt)")
	trayYes := fs.Bool("tray", false, "include a system tray (skips the prompt)")
	trayNo := fs.Bool("no-tray", false, "no system tray (skips the prompt)")
	single := fs.Bool("single", false, "single-page app (skips the prompt)")
	multi := fs.Bool("multi", false, "multi-page app (skips the prompt)")
	tea := fs.Bool("tea", false, "Tea-style page (Go Model/Update/View; skips the prompt)")
	plain := fs.Bool("plain", false, "plain React page with paired handlers (skips the prompt)")
	port := fs.Int("port", 8330, "local server port")
	gantryDir := fs.String("gantry-dir", "", "path to the local Gantry checkout (default: $GANTRY_DIR or auto-detect)")
	noReplace := fs.Bool("no-replace", false, "force the published module even when a local Gantry checkout is detected")
	noInstall := fs.Bool("no-install", false, "skip npm install")
	// Accept "gantry new myapp --flags" as well as "gantry new --flags
	// myapp": the flag package stops at the first non-flag argument, so
	// pull a leading name out before parsing.
	var name string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name = args[0]
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" {
		return fmt.Errorf("usage: gantry new <name>")
	}
	if strings.ContainsAny(name, " \\/") {
		return fmt.Errorf("app name %q must be a single path-safe word (it becomes the Go module and exe name)", name)
	}

	in := bufio.NewReader(os.Stdin)
	s := scaffold{
		Name:       strings.ToLower(name),
		Title:      title(name),
		Port:       *port,
		WebVersion: webPinVersion(),
	}

	// Buttons.
	if *buttons != "" {
		s.BtnMin = strings.Contains(*buttons, "minimize")
		s.BtnMax = strings.Contains(*buttons, "maximize")
		s.BtnClose = strings.Contains(*buttons, "close")
	} else {
		s.BtnMin = askYesNo(in, "Minimize button?", true)
		s.BtnMax = askYesNo(in, "Maximize button?", false)
		s.BtnClose = askYesNo(in, "Close button?", true)
	}

	// Tray.
	switch {
	case *trayYes:
		s.Tray = true
	case *trayNo:
		s.Tray = false
	default:
		s.Tray = askYesNo(in, "System tray (close keeps the app running)?", true)
	}

	// Pages.
	switch {
	case *single:
		s.Multi = false
	case *multi:
		s.Multi = true
	default:
		s.Multi = askYesNo(in, "Multiple pages (adds pages/settings + an example component)?", false)
	}

	// Page style.
	switch {
	case *tea:
		s.Tea = true
	case *plain:
		s.Tea = false
	default:
		s.Tea = askYesNo(in, "Tea-style pages (UI logic in Go)?", true)
	}

	// Default: depend on the PUBLISHED module and npm package (latest).
	// A local checkout is used only when explicitly given (--gantry-dir
	// or GANTRY_DIR) or silently detected by walking up from here -
	// the framework-development workflow.
	if !*noReplace {
		if g := resolveGantryDir(*gantryDir); g != "" {
			s.GantryDir = filepath.ToSlash(g)
			info("using local checkout %s (pass --no-replace for the published module)", g)
		}
	}

	parent := *dir
	if parent == "" {
		parent, _ = os.Getwd()
	}
	appDir := filepath.Join(parent, name)
	if _, err := os.Stat(appDir); err == nil {
		return fmt.Errorf("%s already exists", appDir)
	}

	if err := render(appDir, s); err != nil {
		return err
	}
	if err := writeRegistry(appDir); err != nil {
		return err
	}

	cfg := appConfig{Name: s.Name, Title: s.Title, Version: "0.1.0", Port: s.Port, Tray: s.Tray}
	cfg.Gantry = strings.TrimPrefix(taggedVersion(), "v")
	cfg.Mode = map[bool]string{true: "multi", false: "single"}[s.Multi]
	cfg.Style = map[bool]string{true: "tea", false: "plain"}[s.Tea]
	cfg.Buttons.Minimize = s.BtnMin
	cfg.Buttons.Maximize = s.BtnMax
	cfg.Buttons.Close = s.BtnClose
	cfg.Icons = "icons"
	if err := writeConfig(appDir, cfg); err != nil {
		return err
	}

	// Real icon files, drawn from the placeholder glyph: replace them
	// with your art and every surface (exe, window, tray, installer)
	// follows. gantry_icons.go embeds them into the binary.
	iconDir := filepath.Join(appDir, "icons")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return err
	}
	glyph := appicon.Render(256, appicon.DefaultPalette())
	if err := os.WriteFile(filepath.Join(iconDir, "icon.png"), appicon.PNG(glyph), 0o644); err != nil {
		return err
	}
	ico := appicon.MultiICO(func(sz int) *image.NRGBA {
		return appicon.Render(sz, appicon.DefaultPalette())
	})
	if err := os.WriteFile(filepath.Join(iconDir, "icon.ico"), ico, 0o644); err != nil {
		return err
	}
	if err := writeIcons(appDir, cfg); err != nil {
		return err
	}

	success("created %s", appDir)

	step("go mod tidy")
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = appDir
	tidy.Stdout = os.Stdout
	tidy.Stderr = os.Stderr
	if err := tidy.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	if !*noInstall {
		step("npm install")
		npm := "npm"
		if p, err := exec.LookPath("npm.cmd"); err == nil {
			npm = p
		}
		cmd := exec.Command(npm, "install")
		cmd.Dir = appDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("npm install failed (run it manually in %s): %w", appDir, err)
		}
	}

	fmt.Printf(`
%s

  cd %s
  gantry dev      (live reload in a native window)
  gantry build    (single exe)
`, successStyle.Render("Done. Next:"), name)
	return nil
}

// scaffoldFile is one entry of the scaffold table. user marks files an
// app developer normally edits after scaffolding - gantry upgrade
// defaults those to "keep" and everything else to "overwrite".
type scaffoldFile struct {
	tmpl string // path under templates/
	out  string // path under appDir
	when bool
	user bool
}

// scaffoldFiles is the full scaffold table for one app configuration.
// Template paths mirror the app layout.
func scaffoldFiles(s scaffold) []scaffoldFile {
	pageStyle := "plain"
	if s.Tea {
		pageStyle = "tea"
	}
	return []scaffoldFile{
		{"go.mod.tmpl", "go.mod", true, false},
		{"main.go.tmpl", "main.go", true, true},
		{"embed.go.tmpl", "embed.go", true, false},
		{"package.json.tmpl", "package.json", true, false},
		{"tsconfig.json.tmpl", "tsconfig.json", true, false},
		{"gitignore.tmpl", ".gitignore", true, false},
		{"README.md.tmpl", "README.md", true, true},
		{"index.css.tmpl", "index.css", true, true},
		{"dist-placeholder.html.tmpl", "webdist/index.html", true, false},
		{"vscode-settings.json.tmpl", ".vscode/settings.json", true, false},
		{"vscode-extensions.json.tmpl", ".vscode/extensions.json", true, false},
		{"pages/index-" + pageStyle + ".go.tmpl", "pages/index/index.go", true, true},
		{"pages/index-" + pageStyle + ".tsx.tmpl", "pages/index/index.tsx", true, true},
		{"pages/index.css.tmpl", "pages/index/index.css", true, true},
		{"layouts/main.tsx.tmpl", "layouts/main/main.tsx", s.Multi, true},
		{"layouts/main.css.tmpl", "layouts/main/main.css", s.Multi, true},
		{"pages/settings.go.tmpl", "pages/settings/settings.go", s.Multi, true},
		{"pages/settings.tsx.tmpl", "pages/settings/settings.tsx", s.Multi, true},
		{"pages/settings.css.tmpl", "pages/settings/settings.css", s.Multi, true},
		{"components/example.go.tmpl", "components/example/example.go", s.Multi, true},
		{"components/example.tsx.tmpl", "components/example/example.tsx", s.Multi, true},
		{"components/example.css.tmpl", "components/example/example.css", s.Multi, true},
	}
}

// renderBytes renders one embedded template with the scaffold data.
func renderBytes(tmpl string, s scaffold) ([]byte, error) {
	src, err := templates.ReadFile("templates/" + tmpl)
	if err != nil {
		return nil, fmt.Errorf("reading template %s: %w", tmpl, err)
	}
	// [[ ]] delimiters: tsx legitimately contains {{ (JSX style
	// objects), so the Go defaults would collide.
	t, err := template.New(tmpl).Delims("[[", "]]").Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parsing template %s: %w", tmpl, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, s); err != nil {
		return nil, fmt.Errorf("rendering %s: %w", tmpl, err)
	}
	return buf.Bytes(), nil
}

// render writes every scaffold file.
func render(appDir string, s scaffold) error {
	for _, f := range scaffoldFiles(s) {
		if !f.when {
			continue
		}
		data, err := renderBytes(f.tmpl, s)
		if err != nil {
			return err
		}
		out := filepath.Join(appDir, f.out)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// resolveGantryDir finds a local Gantry checkout: flag, env, then a
// silent walk up from the working directory. Empty means none - the
// scaffold then depends on the published module (the normal case).
func resolveGantryDir(flagVal string) string {
	if flagVal != "" {
		if dir, err := verifyGantryDir(flagVal); err == nil {
			return dir
		}
		warn("--gantry-dir %s is not a Gantry checkout; using the published module", flagVal)
		return ""
	}
	if env := os.Getenv("GANTRY_DIR"); env != "" {
		if dir, err := verifyGantryDir(env); err == nil {
			return dir
		}
		return ""
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			if abs, err := verifyGantryDir(dir); err == nil {
				return abs
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

func verifyGantryDir(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil || !strings.Contains(string(data), "module github.com/B-Commissions/Gantry") {
		return "", fmt.Errorf("%s is not a Gantry checkout", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func askYesNo(in *bufio.Reader, q string, def bool) bool {
	hint := "Y/n"
	if !def {
		hint = "y/N"
	}
	fmt.Print(promptStr(q, hint))
	line, _ := in.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def
	}
	return line == "y" || line == "yes"
}

func title(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
