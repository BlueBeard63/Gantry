package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// cmdBuild produces a single exe: vite build embeds-ready dist/, then
// go build compiles it in.
func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	console := fs.Bool("console", false, "keep the console window (debug builds; default is windowsgui)")
	out := fs.String("o", "", "output exe path (default <name>.exe)")
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

	fmt.Println("gantry: building frontend (vite)")
	vite := exec.Command(npx(), "vite", "build")
	vite.Dir = synthDir
	vite.Stdout = os.Stdout
	vite.Stderr = os.Stderr
	if err := vite.Run(); err != nil {
		return fmt.Errorf("vite build failed: %w", err)
	}

	output := *out
	if output == "" {
		output = cfg.Name + ".exe"
	}
	fmt.Printf("gantry: building %s (go)\n", output)
	// Release exes are windowed apps: no console flashes up behind the
	// window. gantry dev keeps its terminal (that is where the logs
	// stream); use --console here when you need logs from a build.
	goArgs := []string{"build", "-o", output}
	if !*console {
		goArgs = append(goArgs, "-ldflags", "-H windowsgui")
	}
	goArgs = append(goArgs, ".")
	gob := exec.Command("go", goArgs...)
	gob.Dir = appDir
	gob.Stdout = os.Stdout
	gob.Stderr = os.Stderr
	if err := gob.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	fmt.Printf("gantry: done - %s\n", filepath.Join(appDir, output))
	return nil
}
