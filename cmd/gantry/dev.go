package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"

	"github.com/B-Commissions/Gantry/gerr"
)

// cmdDev runs the app with live reload: vite dev serves the frontend
// with HMR, the Go app runs with --dev-url so its native window loads
// from vite, and vite proxies /api and /gantry/ws back to the Go port.
func cmdDev(args []string) error {
	// Config loads before flag parsing: the app's declared args
	// (gantry.json "args") register as real flags so gantry dev
	// validates them and --help lists them.
	appDir, cfg, err := findApp()
	if err != nil {
		return err
	}
	if err := validateArgSpecs(cfg); err != nil {
		return fmt.Errorf("gantry.json: %w", err)
	}

	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	vitePort := fs.Int("vite-port", 5173, "vite dev server port")
	argValues := registerArgFlags(fs, cfg)
	fs.Usage = devUsage(cfg)
	if err := fs.Parse(args); err != nil {
		return err
	}

	synthDir, err := writeSynth(appDir, cfg)
	if err != nil {
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
	if err := writeArgsRegistry(appDir, cfg); err != nil {
		return err
	}

	// The mode and every declared arg travel as environment variables;
	// helper windows (--shellrole children) inherit them from the app.
	childEnv := append(os.Environ(), "GANTRY_MODE=development")
	childEnv = append(childEnv, argEnv(cfg, argValues)...)

	// Ctrl+C tears both children down.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Both children spawn real work in grandchildren (npx.cmd -> node,
	// go run -> the app exe); the group kills whole trees, not just the
	// wrappers.
	group := newChildGroup()

	vite := exec.Command(npx(), "vite", "dev", "--port", strconv.Itoa(*vitePort), "--strictPort")
	vite.Dir = synthDir
	vite.Env = childEnv
	vite.Stdout = os.Stdout
	vite.Stderr = os.Stderr
	group.setup(vite)
	if err := vite.Start(); err != nil {
		return gerr.Wrap("dev.vite-start", err, "starting vite").
			WithHint("is node installed and npm install done?")
	}
	group.add(vite)

	devURL := fmt.Sprintf("http://localhost:%d", *vitePort)
	// Everything after "--" goes to the app itself, e.g.
	// gantry dev -- --no-tray
	appArgs := append([]string{"run", "-ldflags", versionLdflag(cfg), ".",
		"--dev-url", devURL, "--port", strconv.Itoa(cfg.Port)}, fs.Args()...)
	app := exec.Command("go", appArgs...)
	app.Dir = appDir
	app.Env = childEnv
	app.Stdout = os.Stdout
	app.Stderr = os.Stderr
	group.setup(app)
	if err := app.Start(); err != nil {
		group.kill()
		return gerr.Wrap("dev.go-start", err, "starting go run").
			WithHint("is Go installed and on PATH?")
	}
	group.add(app)

	appDone := make(chan error, 1)
	go func() { appDone <- app.Wait() }()
	viteDone := make(chan error, 1)
	go func() { viteDone <- vite.Wait() }()

	select {
	case <-stop:
	case <-appDone: // window closed / app quit
	case <-viteDone: // vite died
	}
	group.kill()
	return nil
}

func npx() string {
	if p, err := exec.LookPath("npx.cmd"); err == nil {
		return p
	}
	return "npx"
}
