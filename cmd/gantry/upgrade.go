package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var webDepRe = regexp.MustCompile(`("gantry-web"\s*:\s*)"([^"]*)"`)

// cmdUpgrade brings the current app up to the running CLI's version:
// pins + installs the matching packages, offers re-rendered scaffold
// files with a per-file diff (user-edited files default to keep,
// tooling files to overwrite), regenerates the derived files, and
// records the framework version in gantry.json.
func cmdUpgrade(args []string) error {
	fs := flag.NewFlagSet("upgrade", flag.ExitOnError)
	yes := fs.Bool("yes", false, "apply every file's default action without prompting")
	dry := fs.Bool("dry-run", false, "report what would change without writing anything")
	force := fs.Bool("force", false, "run even when the app is already at this version")
	if err := fs.Parse(args); err != nil {
		return err
	}

	appDir, cfg, err := findApp()
	if err != nil {
		return err
	}

	// Target: the running CLI's tag - its embedded templates ARE the
	// upgrade. A dev build still upgrades (its templates are newest of
	// all); packages then pin to the newest known tag.
	target := taggedVersion()
	if target == "" {
		target = latestVersionFresh()
		if target == "" {
			return fmt.Errorf("gantry is a dev build and the module proxy is unreachable - cannot pick a package version")
		}
		warn("gantry is a local checkout build; templates come from it, packages pin to %s", target)
	} else if latest := latestVersionFresh(); latest != "" && semverLess(target, latest) {
		warn("gantry %s is available (you have %s) - consider gantry update first", latest, target)
	}
	targetBare := strings.TrimPrefix(target, "v")

	from := cfg.Gantry
	fromLabel := from
	if fromLabel == "" {
		fromLabel = "unrecorded"
	}
	step("upgrading %s: gantry %s -> %s", cfg.Name, fromLabel, targetBare)
	if v := installedWebVersion(appDir); v != "" {
		info("installed gantry-web: %s", v)
	}
	if !*force && from == targetBare {
		success("already at %s (use --force to re-apply)", targetBare)
		return nil
	}

	// --- dependencies -------------------------------------------------
	if err := upgradeGoDep(appDir, target, *dry); err != nil {
		return err
	}
	if err := upgradeWebDep(appDir, targetBare, *dry); err != nil {
		return err
	}

	// --- scaffold templates -------------------------------------------
	s := scaffoldFromConfig(cfg)
	s.WebVersion = targetBare
	in := bufio.NewReader(os.Stdin)
	var created, updated, kept, same int
	for _, f := range scaffoldFiles(s) {
		// go.mod and package.json carry user dependencies and were
		// handled surgically above - never re-render them wholesale.
		// webdist/index.html is build OUTPUT once gantry build ran;
		// re-placing the placeholder would break a built app.
		if !f.when || f.out == "go.mod" || f.out == "package.json" || f.out == "webdist/index.html" {
			continue
		}
		want, err := renderBytes(f.tmpl, s)
		if err != nil {
			return err
		}
		path := filepath.Join(appDir, f.out)
		have, err := os.ReadFile(path)
		if err != nil { // missing: scaffold gained a file (or it was deleted)
			if *dry {
				info("would create %s", f.out)
				created++
				continue
			}
			if *yes || askYesNo(in, fmt.Sprintf("Create missing %s?", f.out), true) {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(path, want, 0o644); err != nil {
					return err
				}
				created++
			} else {
				kept++
			}
			continue
		}
		if normalizeEOL(have) == normalizeEOL(want) {
			same++
			continue
		}
		def := !f.user // tooling files default to overwrite, user files to keep
		if *dry {
			info("would %s %s (template differs)", map[bool]string{true: "overwrite", false: "keep"}[def], f.out)
			continue
		}
		apply := def
		if !*yes {
			fmt.Print(unifiedDiff(f.out, have, want, 120))
			hint := "template version"
			if f.user {
				hint = "your file may carry local edits"
			}
			apply = askYesNo(in, fmt.Sprintf("Overwrite %s? (%s)", f.out, hint), def)
		}
		if apply {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				return err
			}
			updated++
		} else {
			kept++
		}
	}

	// --- derived files + config ---------------------------------------
	if *dry {
		info("would regenerate .gantry/, gantry_registry.go, gantry_widgets.go, gantry_icons.go")
		info("would record gantry %s in gantry.json", targetBare)
	} else {
		step("regenerating derived files")
		if _, err := writeSynth(appDir, cfg); err != nil {
			return err
		}
		if err := writeRegistry(appDir); err != nil {
			return err
		}
		if err := writeWidgetRegistry(appDir, cfg); err != nil {
			return err
		}
		if err := writeIcons(appDir, cfg); err != nil {
			return err
		}
		cfg.Gantry = targetBare
		if err := writeConfig(appDir, cfg); err != nil {
			return err
		}
	}

	printUpgradeNotes(from, targetBare)
	if *dry {
		success("dry run complete - nothing written")
	} else {
		success("upgraded to %s (%d updated, %d created, %d kept, %d already current)",
			targetBare, updated, created, kept, same)
		info("run gantry dev to verify the app")
	}
	return nil
}

// scaffoldFromConfig reconstructs the template data gantry new used,
// from the choices it recorded in gantry.json. GantryDir stays empty:
// upgrade never re-renders go.mod/package.json, the only templates
// that use it.
func scaffoldFromConfig(cfg appConfig) scaffold {
	return scaffold{
		Name:     cfg.Name,
		Title:    cfg.Title,
		Port:     cfg.Port,
		Tray:     cfg.Tray,
		Multi:    cfg.Mode == "multi",
		Tea:      cfg.Style == "tea",
		BtnMin:   cfg.Buttons.Minimize,
		BtnMax:   cfg.Buttons.Maximize,
		BtnClose: cfg.Buttons.Close,
	}
}

// upgradeGoDep moves the app's Gantry module requirement to the target
// tag, unless the app uses a local checkout via replace.
func upgradeGoDep(appDir, target string, dry bool) error {
	goMod, err := os.ReadFile(filepath.Join(appDir, "go.mod"))
	if err != nil {
		return fmt.Errorf("reading go.mod: %w", err)
	}
	if strings.Contains(string(goMod), modulePath+" =>") {
		info("go.mod replaces %s with a local checkout - skipping go get", modulePath)
		return nil
	}
	if dry {
		info("would run: go get %s@%s && go mod tidy", modulePath, target)
		return nil
	}
	step("go get %s@%s", modulePath, target)
	for _, argv := range [][]string{
		{"go", "get", modulePath + "@" + target},
		{"go", "mod", "tidy"},
	} {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = appDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", strings.Join(argv, " "), err)
		}
	}
	return nil
}

// upgradeWebDep pins gantry-web in package.json to the target version
// and installs it, unless the app links the package from a checkout.
// The pin is a targeted edit - package.json carries the user's own
// dependencies, so it is never re-rendered from the template.
func upgradeWebDep(appDir, targetBare string, dry bool) error {
	pkgPath := filepath.Join(appDir, "package.json")
	pkg, err := os.ReadFile(pkgPath)
	if err != nil {
		return fmt.Errorf("reading package.json: %w", err)
	}
	m := webDepRe.FindSubmatch(pkg)
	if m == nil {
		warn("package.json has no gantry-web dependency - skipping npm")
		return nil
	}
	if strings.HasPrefix(string(m[2]), "file:") {
		info("gantry-web is a file: link - skipping npm")
		return nil
	}
	if dry {
		info("would pin gantry-web to %s and run npm install", targetBare)
		return nil
	}
	if string(m[2]) != targetBare {
		next := webDepRe.ReplaceAll(pkg, []byte("${1}\""+targetBare+"\""))
		if err := os.WriteFile(pkgPath, next, 0o644); err != nil {
			return err
		}
	}
	step("npm install (gantry-web %s)", targetBare)
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
	return nil
}

// installedWebVersion reads the version of the gantry-web actually in
// node_modules, "" when absent.
func installedWebVersion(appDir string) string {
	data, err := os.ReadFile(filepath.Join(appDir, "node_modules", "gantry-web", "package.json"))
	if err != nil {
		return ""
	}
	var p struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &p) != nil {
		return ""
	}
	return p.Version
}
