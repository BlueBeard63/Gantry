//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// childGroup kills dev's child process trees: vite is started through
// npx and the app through "go run .", so a plain Process.Kill would
// only hit the wrapper and orphan the real node/app processes. Each
// child becomes its own process group leader; killing the negative
// pgid takes the whole tree down.
type childGroup struct {
	pgids []int
}

func newChildGroup() *childGroup { return &childGroup{} }

// setup makes the command start in its own process group.
func (g *childGroup) setup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// add records a started command's process group.
func (g *childGroup) add(cmd *exec.Cmd) {
	if cmd.Process != nil {
		g.pgids = append(g.pgids, cmd.Process.Pid)
	}
}

// kill terminates every recorded process group.
func (g *childGroup) kill() {
	for _, pgid := range g.pgids {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
