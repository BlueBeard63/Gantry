package main

import (
	"errors"
	"os"
	"os/exec"
)

// cmdAdd installs npm packages into the app (a thin npm install alias
// that always runs at the app root, wherever you are in the tree).
// Frontend dependencies belong to each app, never to Gantry itself -
// icon packs, chart libraries, whatever the app needs.
func cmdAdd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: gantry add <package...>")
	}
	appDir, _, err := findApp()
	if err != nil {
		return err
	}
	npm := "npm"
	if p, err := exec.LookPath("npm.cmd"); err == nil {
		npm = p
	}
	cmd := exec.Command(npm, append([]string{"install"}, args...)...)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
