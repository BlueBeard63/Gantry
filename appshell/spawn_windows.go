//go:build windows

package appshell

import (
	"os/exec"
	"syscall"
)

// hideSpawnConsole stops a child process from flashing a console window
// when the parent binary was built without -H windowsgui (dev builds):
// the child renders its own WebView2 window, the console is pure noise.
// CREATE_NO_WINDOW only - HideWindow (STARTUPINFO SW_HIDE) must NOT be
// set: it overrides the child GUI window's first ShowWindow and the
// WebView2 content ends up as an unpainted white rectangle.
func hideSpawnConsole(cmd *exec.Cmd) {
	const createNoWindow = 0x08000000
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
