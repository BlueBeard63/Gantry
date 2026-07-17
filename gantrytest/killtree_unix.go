//go:build !windows

package gantrytest

import (
	"os/exec"
	"syscall"
)

// childGroup kills an app-under-test's process tree: the app spawns
// helper windows (--shellrole children), so a plain Process.Kill would
// orphan them. Each launched app becomes its own process group leader;
// killing the negative pgid takes the whole tree down. Same machinery
// as the CLI's dev child management.
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
