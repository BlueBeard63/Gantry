package appshell

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// ProcManager runs helper windows (widgets, popups) as child processes
// by re-invoking the app's own exe with role flags. Out-of-process
// windows mean a webview crash can never take the host app down, and
// closing one is just a process kill.
//
// The manager only spawns "exe + args" - the app owns its CLI. The
// usual pattern (which gantry new generates) is a --shellrole flag the
// app checks in main() before starting its server:
//
//	m := appshell.NewProcManager()
//	m.Show("timer", "--shellrole", "widget-timer", "--port", port)
type ProcManager struct {
	exe string

	mu    sync.Mutex
	procs map[string]*exec.Cmd
}

// NewProcManager creates a manager that re-invokes the current
// executable.
func NewProcManager() *ProcManager {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	return &ProcManager{exe: exe, procs: map[string]*exec.Cmd{}}
}

// Show starts the keyed child process if it is not already running.
func (m *ProcManager) Show(key string, args ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.procs[key] != nil {
		return
	}
	cmd := exec.Command(m.exe, args...)
	hideSpawnConsole(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("appshell: starting %s process: %v", key, err)
		return
	}
	m.procs[key] = cmd
	// Reap the process so it never zombies; forget it once gone.
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if m.procs[key] == cmd {
			delete(m.procs, key)
		}
		m.mu.Unlock()
	}()
}

// Toggle shows the keyed process, or kills it if it is already running
// (flyout style: one click opens, another dismisses).
func (m *ProcManager) Toggle(key string, args ...string) {
	m.mu.Lock()
	cmd := m.procs[key]
	delete(m.procs, key)
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		return
	}
	m.Show(key, args...)
}

// Replace kills any running keyed process and starts a fresh one
// (notification popups: a new one supersedes the old).
func (m *ProcManager) Replace(key string, args ...string) {
	m.mu.Lock()
	cmd := m.procs[key]
	delete(m.procs, key)
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	m.Show(key, args...)
}

// Kill stops the keyed process if it is running.
func (m *ProcManager) Kill(key string) {
	m.mu.Lock()
	cmd := m.procs[key]
	delete(m.procs, key)
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// Running reports whether the keyed process is alive.
func (m *ProcManager) Running(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.procs[key] != nil
}

// CloseAll kills every managed process (app shutdown).
func (m *ProcManager) CloseAll() {
	m.mu.Lock()
	procs := m.procs
	m.procs = map[string]*exec.Cmd{}
	m.mu.Unlock()
	for _, cmd := range procs {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

// RoleLog redirects the standard logger to
// %LocalAppData%\<appName>\<role>.log and returns a cleanup func.
// windowsgui builds have no console, so without this a crashing child
// role (widget, popup) is undiagnosable.
func RoleLog(appName, role string) func() {
	base, err := os.UserCacheDir()
	if err != nil {
		return func() {}
	}
	dir := filepath.Join(base, appName)
	_ = os.MkdirAll(dir, 0o700)
	f, err := os.OpenFile(filepath.Join(dir, role+".log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return func() {}
	}
	log.SetOutput(f)
	return func() { _ = f.Close() }
}
