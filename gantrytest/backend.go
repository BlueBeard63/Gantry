package gantrytest

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

// backend is the process-control plane - the only tier that varies by
// target, so it sits behind this seam. Tier 1 ships the local desktop
// backend below; tier M1 adds the adb-backed one in android.go, so the
// same tests run against a phone or emulator.
type backend interface {
	// launch starts the app under test and returns once the server has
	// announced its listening port.
	launch(spec launchSpec) (proc, error)
}

// proc is one running app instance.
type proc interface {
	// port is where the app's HTTP/websocket server is reachable from
	// the host (on a device backend, the adb-forwarded local port).
	port() int
	// token authenticates driver requests (gantry_token); empty when
	// the server runs unguarded (desktop).
	token() string
	// stop kills the whole process tree, hard. Safe to call twice.
	stop()
	// crashLogPath is where the runtime's crash.log lives for this
	// instance (may not exist; empty when there is none to fetch).
	crashLogPath() string
	// exited closes when the process has exited on its own.
	exited() <-chan struct{}
	// screenshot captures the target's screen at the process-control
	// level (device backends); errors where only the DOM plane can.
	screenshot() ([]byte, error)
}

// launchSpec is everything a backend needs to start one hermetic app
// instance.
type launchSpec struct {
	bin       string
	appDir    string
	appName   string
	configDir string   // per-test stand-in for the user config dir
	env       []string // full environment (os.Environ() + overrides); desktop only
	overrides []string // the test's env deltas alone (mode, args, WithEnv) - what device backends inject
	window    bool     // open the real window (headed or DOM-plane runs)
	appLog    *syncWriter
	timeout   time.Duration // how long to wait for GANTRY_READY
}

// localBackend runs the app as a local process, the way gantry dev
// launches children: --port 0 for a parallel-safe ephemeral port,
// --announce-ready to learn it from stdout, --no-open unless headed,
// and the kill-tree machinery so no orphaned webview or helper
// processes survive a failed run.
type localBackend struct{}

var readyRe = regexp.MustCompile(`^GANTRY_READY port=(\d+)$`)

func (localBackend) launch(spec launchSpec) (proc, error) {
	args := []string{"--port", "0", "--announce-ready"}
	if !spec.window {
		args = append(args, "--no-open")
	}
	cmd := exec.Command(spec.bin, args...)
	cmd.Dir = spec.appDir
	cmd.Env = spec.env

	group := newChildGroup()
	group.setup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = spec.appLog

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", spec.bin, err)
	}
	group.add(cmd)

	p := &localProc{spec: spec, group: group, done: make(chan struct{})}

	// The stdout scanner runs for the process lifetime: it mirrors
	// every line into app.log and catches the ready announcement.
	portCh := make(chan int, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			fmt.Fprintln(spec.appLog, line)
			if m := readyRe.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				select {
				case portCh <- n:
				default:
				}
			}
		}
	}()
	go func() {
		_ = cmd.Wait()
		close(p.done)
	}()

	select {
	case port := <-portCh:
		p.listenPort = port
		return p, nil
	case <-p.done:
		p.stop()
		return nil, fmt.Errorf("app exited before announcing readiness (see app.log)")
	case <-time.After(spec.timeout):
		p.stop()
		return nil, fmt.Errorf("timed out after %s waiting for GANTRY_READY on stdout", spec.timeout)
	}
}

type localProc struct {
	spec       launchSpec
	group      *childGroup
	listenPort int
	done       chan struct{}
}

func (p *localProc) port() int               { return p.listenPort }
func (p *localProc) token() string           { return "" }
func (p *localProc) exited() <-chan struct{} { return p.done }

func (p *localProc) screenshot() ([]byte, error) {
	return nil, errors.New("process-level screenshots need a device backend (desktop captures through the DOM plane)")
}

func (p *localProc) stop() {
	p.group.kill()
	// Give the tree a moment to actually die so temp dirs unlock.
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
	}
}

// crashLogPath mirrors the runtime's crashLogPath under the redirected
// config dir (the backend points os.UserConfigDir at spec.configDir
// via APPDATA / XDG_CONFIG_HOME / HOME).
func (p *localProc) crashLogPath() string {
	base := p.spec.configDir
	if runtime.GOOS == "darwin" {
		base = filepath.Join(base, "Library", "Application Support")
	}
	return filepath.Join(base, p.spec.appName, "crash.log")
}

// configDirEnv redirects the app's os.UserConfigDir - where
// geometry.json and crash.log live - into the per-test dir, so tests
// are hermetic and crash.log assertions are per-test. On Windows the
// cache dir (LOCALAPPDATA) is redirected too: that is where the
// WebView2 user-data folder lives, and two app processes must never
// share one - without this, parallel DOM-plane tests would collide.
func configDirEnv(configDir string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"APPDATA=" + configDir, "LOCALAPPDATA=" + configDir}
	case "darwin":
		return []string{"HOME=" + configDir}
	default:
		return []string{"XDG_CONFIG_HOME=" + configDir, "XDG_CACHE_HOME=" + configDir}
	}
}
