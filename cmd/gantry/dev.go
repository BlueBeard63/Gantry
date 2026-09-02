package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"

	"github.com/BlueBeard63/Gantry/gerr"
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
	if err := writeResources(appDir); err != nil {
		return err
	}
	if err := writeArgsRegistry(appDir, cfg); err != nil {
		return err
	}

	// The mode and every declared arg travel as environment variables;
	// helper windows (--shellrole children) inherit them from the app.
	childEnv := append(os.Environ(), "GANTRY_MODE=development")
	childEnv = append(childEnv, argEnv(cfg, argValues)...)

	// Ctrl+C tears every child down.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Vite and the Go app run in SEPARATE child groups so a .go edit can
	// restart the app (kill its whole go-run -> exe tree) without
	// disturbing vite's HMR. Each group kills whole trees, not just the
	// npx.cmd / go run wrappers (grandchildren: node, the app exe).
	viteGroup := newChildGroup()

	vite := exec.Command(npx(), "vite", "dev", "--port", strconv.Itoa(*vitePort), "--strictPort")
	vite.Dir = synthDir
	vite.Env = childEnv
	vite.Stdout = os.Stdout
	vite.Stderr = os.Stderr
	viteGroup.setup(vite)
	if err := vite.Start(); err != nil {
		return gerr.Wrap("dev.vite-start", err, "starting vite").
			WithHint("is node installed and npm install done?")
	}
	viteGroup.add(vite)
	viteDone := make(chan error, 1)
	go func() { viteDone <- vite.Wait() }()

	devURL := fmt.Sprintf("http://localhost:%d", *vitePort)
	// Everything after "--" goes to the app itself, e.g.
	// gantry dev -- --no-tray
	extraArgs := fs.Args()

	// startApp launches "go run . --dev-url ..." in a fresh child group,
	// plus a goroutine reporting its exit on the returned channel. A
	// restart is: kill the old group, drain its exit (which frees the
	// port - appshell.Listen is the single-instance guard), start again.
	startApp := func() (*childGroup, chan error, error) {
		g := newChildGroup()
		appArgs := append([]string{"run", "-ldflags", versionLdflag(cfg), ".",
			"--dev-url", devURL, "--port", strconv.Itoa(cfg.Port)}, extraArgs...)
		cmd := exec.Command("go", appArgs...)
		cmd.Dir = appDir
		cmd.Env = childEnv
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		g.setup(cmd)
		if err := cmd.Start(); err != nil {
			return nil, nil, err
		}
		g.add(cmd)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		return g, done, nil
	}

	appGroup, appDone, err := startApp()
	if err != nil {
		viteGroup.kill()
		return gerr.Wrap("dev.go-start", err, "starting go run").
			WithHint("is Go installed and on PATH?")
	}
	appAlive := true

	// Watch .go files and resources/ and signal on save. On change the Go
	// app rebuilds+restarts; the frontend re-renders on its own when the
	// websocket reconnects to the fresh server (web/src/socket.ts
	// re-announces the active page on every reconnect). vite HMR keeps
	// handling .tsx/.css - this only covers the Go half.
	changed := make(chan struct{}, 1)
	watchDone := make(chan struct{})
	defer close(watchDone)
	go watchGoSources(appDir, changed, watchDone)

	info("watching .go files - save to rebuild the Go app")

	for {
		select {
		case <-stop:
			appGroup.kill()
			viteGroup.kill()
			return nil
		case exitErr := <-appDone:
			// A clean exit (code 0) is a real quit - the window closed.
			// A non-zero exit is a crash or a failed rebuild: keep the dev
			// server up (like vite survives a TS error) and wait for the
			// next save to retry, instead of tearing everything down.
			if exitErr == nil {
				viteGroup.kill()
				return nil
			}
			appAlive = false
			warn("app exited: %v - fix and save to reload", exitErr)
		case <-viteDone: // vite died: nothing left to serve the frontend
			if appAlive {
				appGroup.kill()
			}
			return nil
		case <-changed:
			step("go change - rebuilding the app")
			// A new page/component or resource may have appeared: refresh
			// the Go-side generated files (writeSynth is skipped - the
			// vite root is unaffected by .go edits and rewriting it would
			// perturb the running vite server).
			regenForReload(appDir, cfg)
			if appAlive {
				appGroup.kill()
				<-appDone // drain the killed instance's exit
			}
			// go run's own build output (and, on success, the app's
			// "serving on ..." line) reports the outcome; a failed build
			// lands in the <-appDone case as a non-zero exit.
			g, done, err := startApp()
			if err != nil {
				appAlive = false
				warn("restart failed: %v - save to retry", err)
				continue
			}
			appGroup, appDone, appAlive = g, done, true
		}
	}
}

func npx() string {
	if p, err := exec.LookPath("npx.cmd"); err == nil {
		return p
	}
	return "npx"
}
