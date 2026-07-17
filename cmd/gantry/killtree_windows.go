//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

// childGroup ties dev's child processes to a Windows Job Object with
// kill-on-close: vite is started through npx.cmd and the app through
// "go run .", so a plain Process.Kill would only hit the wrapper and
// orphan the real node/app processes. Every process assigned to the
// job - and all its descendants - dies when the job handle closes,
// including when gantry itself crashes.
type childGroup struct {
	job  windows.Handle
	pids []int
}

func newChildGroup() *childGroup {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return &childGroup{}
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return &childGroup{}
	}
	return &childGroup{job: job}
}

// setup is called before cmd.Start; assignment happens in add on Windows.
func (g *childGroup) setup(cmd *exec.Cmd) {}

// add assigns a started command to the job; its future descendants
// inherit membership.
func (g *childGroup) add(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	g.pids = append(g.pids, cmd.Process.Pid)
	if g.job == 0 {
		return
	}
	h, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false,
		uint32(cmd.Process.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	_ = windows.AssignProcessToJobObject(g.job, h)
}

// kill terminates every process in the job. The taskkill sweep first
// catches grandchildren that spawned in the window between Start and
// add (job membership is not retroactive for pre-existing children).
func (g *childGroup) kill() {
	for _, pid := range g.pids {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
	if g.job != 0 {
		_ = windows.CloseHandle(g.job)
		g.job = 0
	}
}
