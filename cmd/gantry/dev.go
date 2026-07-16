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

	// Ctrl+C tears both children down.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	vite := exec.Command(npx(), "vite", "dev", "--port", strconv.Itoa(*vitePort), "--strictPort")
	vite.Dir = synthDir
	vite.Stdout = os.Stdout
	vite.Stderr = os.Stderr
	if err := vite.Start(); err != nil {
		return fmt.Errorf("starting vite (is node installed and npm install done?): %w", err)
	}

	devURL := fmt.Sprintf("http://localhost:%d", *vitePort)
	app := exec.Command("go", "run", ".", "--dev-url", devURL, "--port", strconv.Itoa(cfg.Port))
	app.Dir = appDir
	app.Stdout = os.Stdout
	app.Stderr = os.Stderr
	if err := app.Start(); err != nil {
		_ = vite.Process.Kill()
		return fmt.Errorf("starting go run: %w", err)
	}

	appDone := make(chan error, 1)
	go func() { appDone <- app.Wait() }()
	viteDone := make(chan error, 1)
	go func() { viteDone <- vite.Wait() }()

	select {
	case <-stop:
	case <-appDone: // window closed / app quit
	case <-viteDone: // vite died
	}
	if app.Process != nil {
		_ = app.Process.Kill()
	}
	if vite.Process != nil {
		_ = vite.Process.Kill()
	}
	return nil
}

func npx() string {
	if p, err := exec.LookPath("npx.cmd"); err == nil {
		return p
	}
	return "npx"
}
