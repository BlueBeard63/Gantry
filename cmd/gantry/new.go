package main

import (
	"bufio"
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
	Name      string // module + exe name, e.g. "myapp"
	Title     string // window title, e.g. "My App"
	Port      int
	Tray      bool
	Multi     bool
	Tea       bool
	BtnMin    bool
	BtnMax    bool
	BtnClose  bool
	GantryDir string // forward-slash path to the local Gantry checkout ("" = none, use GOPRIVATE)
}

func (s scaffold) GantryWeb() string { return s.GantryDir + "/web" }

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
		Name:  strings.ToLower(name),
		Title: title(name),
		Port:  *port,
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
			fmt.Printf("gantry: using local checkout %s (pass --no-replace for the published module)\n", g)
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

	fmt.Printf("gantry: created %s\n", appDir)

	fmt.Println("gantry: go mod tidy")
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = appDir
	tidy.Stdout = os.Stdout
	tidy.Stderr = os.Stderr
	if err := tidy.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	if !*noInstall {
		fmt.Println("gantry: npm install")
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
Done. Next:

  cd %s
  gantry dev      (live reload in a native window)
  gantry build    (single exe)
`, name)
	return nil
}

// render writes every scaffold file. Template paths mirror the app
// layout; a leading _ segment marks conditional trees.
func render(appDir string, s scaffold) error {
	type file struct {
		tmpl string // path under templates/
		out  string // path under appDir
		when bool
	}
	pageStyle := "plain"
	if s.Tea {
		pageStyle = "tea"
	}
	files := []file{
		{"go.mod.tmpl", "go.mod", true},
		{"main.go.tmpl", "main.go", true},
		{"embed.go.tmpl", "embed.go", true},
		{"package.json.tmpl", "package.json", true},
		{"tsconfig.json.tmpl", "tsconfig.json", true},
		{"gitignore.tmpl", ".gitignore", true},
		{"README.md.tmpl", "README.md", true},
		{"index.css.tmpl", "index.css", true},
		{"dist-placeholder.html.tmpl", "webdist/index.html", true},
		{"vscode-settings.json.tmpl", ".vscode/settings.json", true},
		{"vscode-extensions.json.tmpl", ".vscode/extensions.json", true},
		{"pages/index-" + pageStyle + ".go.tmpl", "pages/index/index.go", true},
		{"pages/index-" + pageStyle + ".tsx.tmpl", "pages/index/index.tsx", true},
		{"pages/index.css.tmpl", "pages/index/index.css", true},
		{"layouts/main.tsx.tmpl", "layouts/main/main.tsx", s.Multi},
		{"layouts/main.css.tmpl", "layouts/main/main.css", s.Multi},
		{"pages/settings.go.tmpl", "pages/settings/settings.go", s.Multi},
		{"pages/settings.tsx.tmpl", "pages/settings/settings.tsx", s.Multi},
		{"pages/settings.css.tmpl", "pages/settings/settings.css", s.Multi},
		{"components/example.go.tmpl", "components/example/example.go", s.Multi},
		{"components/example.tsx.tmpl", "components/example/example.tsx", s.Multi},
		{"components/example.css.tmpl", "components/example/example.css", s.Multi},
	}
	for _, f := range files {
		if !f.when {
			continue
		}
		src, err := templates.ReadFile("templates/" + f.tmpl)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", f.tmpl, err)
		}
		// [[ ]] delimiters: tsx legitimately contains {{ (JSX style
		// objects), so the Go defaults would collide.
		t, err := template.New(f.tmpl).Delims("[[", "]]").Parse(string(src))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", f.tmpl, err)
		}
		out := filepath.Join(appDir, f.out)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		w, err := os.Create(out)
		if err != nil {
			return err
		}
		if err := t.Execute(w, s); err != nil {
			w.Close()
			return fmt.Errorf("rendering %s: %w", f.tmpl, err)
		}
		w.Close()
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
		fmt.Fprintf(os.Stderr, "gantry: --gantry-dir %s is not a Gantry checkout; using the published module\n", flagVal)
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
	fmt.Printf("%s [%s] ", q, hint)
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
