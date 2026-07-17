package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
)

// cmdDev runs the app with live reload: vite dev serves the frontend
// with HMR, the Go app runs with --dev-url so its native window loads
// from vite, and vite proxies /api and /gantry/ws back to the Go port.
func cmdDev(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	vitePort := fs.Int("vite-port", 5173, "vite dev server port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	appDir, cfg, err := findApp()
	if err != nil {
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

	// Ctrl+C tears both children down.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Both children spawn real work in grandchildren (npx.cmd -> node,
	// go run -> the app exe); the group kills whole trees, not just the
	// wrappers.
	group := newChildGroup()

	vite := exec.Command(npx(), "vite", "dev", "--port", strconv.Itoa(*vitePort), "--strictPort")
	vite.Dir = synthDir
	vite.Stdout = os.Stdout
	vite.Stderr = os.Stderr
	group.setup(vite)
	if err := vite.Start(); err != nil {
		return fmt.Errorf("starting vite (is node installed and npm install done?): %w", err)
	}
	group.add(vite)

	devURL := fmt.Sprintf("http://localhost:%d", *vitePort)
	// Everything after "--" goes to the app itself, e.g.
	// gantry dev -- --no-tray
	appArgs := append([]string{"run", ".", "--dev-url", devURL, "--port", strconv.Itoa(cfg.Port)}, fs.Args()...)
	app := exec.Command("go", appArgs...)
	app.Dir = appDir
	app.Stdout = os.Stdout
	app.Stderr = os.Stderr
	group.setup(app)
	if err := app.Start(); err != nil {
		group.kill()
		return fmt.Errorf("starting go run: %w", err)
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
