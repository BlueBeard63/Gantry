//go:build !windows

package appshell

import "os/exec"

func hideSpawnConsole(*exec.Cmd) {}
