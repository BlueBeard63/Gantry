package gantrytest

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// androidBackend is the tier-M1 process-control plane: the same
// launchSpec contract as the desktop backend, implemented with adb
// against one device or emulator. The app under test is the installed
// debug APK - the same Go server packed as libgantryapp.so, launched
// by the Kotlin shell - and the shell honors the runner's intent
// extras (a fixed port, a runner-chosen token, and the test's declared
// environment), so the whole protocol plane works over one adb forward.
//
// `gantry test --device android` builds and installs that APK and sets
// the environment this backend reads; a bare `go test` run derives
// what it can (adb on PATH, the sole connected device, the app id from
// gantry.json) against an already-installed debug APK.
type androidBackend struct {
	adb        string
	serial     string
	appID      string
	appName    string
	allowClear bool

	// pm clear runs before the first launch of an App only: a Restart
	// must keep the app's data - that is the point of the crash.log
	// scenario.
	cleared bool
}

// devicePortSeq rotates the fixed server ports handed to launches so a
// just-stopped instance's port is not immediately rebound.
var devicePortSeq atomic.Int64

func nextDevicePort() int {
	return 18330 + int(devicePortSeq.Add(1)-1)%200
}

var readyLogRe = regexp.MustCompile(`GANTRY_READY port=(\d+)`)

func androidBackendFromEnv(t testing.TB, appCfg appConfig) *androidBackend {
	t.Helper()
	b := &androidBackend{
		adb:        os.Getenv("GANTRY_TEST_ADB"),
		serial:     os.Getenv("GANTRY_TEST_SERIAL"),
		appID:      os.Getenv("GANTRY_TEST_APP_ID"),
		appName:    appCfg.Name,
		allowClear: os.Getenv("GANTRY_TEST_ALLOW_CLEAR") == "1",
	}
	if b.appID == "" && appCfg.Mobile != nil {
		b.appID = appCfg.Mobile.ID
	}
	if b.appID == "" {
		t.Fatalf(`gantrytest: no application id for the device target (gantry.json needs a "mobile" section with an "id")`)
	}
	if b.adb == "" {
		if p, err := exec.LookPath("adb"); err == nil {
			b.adb = p
		} else if home := os.Getenv("ANDROID_HOME"); home != "" {
			name := "adb"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			if p := filepath.Join(home, "platform-tools", name); isFile(p) {
				b.adb = p
			}
		}
	}
	if b.adb == "" {
		t.Fatalf("gantrytest: adb not found - run via `gantry test --device android`, or put adb on PATH / set ANDROID_HOME")
	}
	if b.serial == "" {
		serials := connectedDevices(t, b.adb)
		switch len(serials) {
		case 1:
			b.serial = serials[0]
		case 0:
			t.Fatalf("gantrytest: no device connected (adb devices lists none)")
		default:
			t.Fatalf("gantrytest: multiple devices connected (%s) - run via `gantry test --device android:SERIAL` or set GANTRY_TEST_SERIAL", strings.Join(serials, ", "))
		}
	}
	if !b.allowClear && strings.HasPrefix(b.serial, "emulator-") {
		b.allowClear = true
	}
	return b
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func connectedDevices(t testing.TB, adb string) []string {
	t.Helper()
	out, err := exec.Command(adb, "devices").Output()
	if err != nil {
		t.Fatalf("gantrytest: adb devices: %v", err)
	}
	var serials []string
	for _, line := range strings.Split(string(out), "\n")[1:] {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[1] == "device" {
			serials = append(serials, fields[0])
		}
	}
	return serials
}

// adbOut runs one adb command against the backend's device and returns
// its combined output, wrapping failures with the command and output.
func (b *androidBackend) adbOut(args ...string) (string, error) {
	full := append([]string{"-s", b.serial}, args...)
	out, err := exec.Command(b.adb, full...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("adb %s: %w\n%s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return string(out), nil
}

func (b *androidBackend) launch(spec launchSpec) (proc, error) {
	// Hermetic start: wipe the app's data once per test instance when
	// allowed (pm clear also kills the app); otherwise just make sure
	// it is stopped. Restarts land in the else branch and keep data.
	if b.allowClear && !b.cleared {
		if out, err := b.adbOut("shell", "pm", "clear", b.appID); err != nil {
			return nil, err
		} else if !strings.Contains(out, "Success") {
			return nil, fmt.Errorf("pm clear %s: %s (is the app installed? gantry test --device installs it)", b.appID, strings.TrimSpace(out))
		}
		b.cleared = true
	} else {
		if _, err := b.adbOut("shell", "am", "force-stop", b.appID); err != nil {
			return nil, err
		}
	}

	devicePort := nextDevicePort()
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	// The stream starts at "now": a previous run's lines must not
	// satisfy the ready wait or pollute this test's app.log.
	_, _ = b.adbOut("logcat", "-c")
	cat := exec.Command(b.adb, "-s", b.serial, "logcat", "-v", "time", "-s", "gantry-go")
	stdout, err := cat.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cat.Stderr = spec.appLog
	if err := cat.Start(); err != nil {
		return nil, fmt.Errorf("starting logcat: %w", err)
	}
	var catOnce sync.Once
	stopLogcat := func() {
		catOnce.Do(func() {
			if cat.Process != nil {
				_ = cat.Process.Kill()
			}
			_ = cat.Wait()
		})
	}

	// The logcat scanner runs for the instance lifetime: it mirrors the
	// device-side log into app.log and catches the ready announcement
	// for the port this launch chose.
	ready := make(chan struct{})
	var readyOnce sync.Once
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			fmt.Fprintln(spec.appLog, line)
			if m := readyLogRe.FindStringSubmatch(line); m != nil && m[1] == strconv.Itoa(devicePort) {
				readyOnce.Do(func() { close(ready) })
			}
		}
	}()

	// The config travels twice. A file written into the app sandbox
	// through run-as (debuggable builds allow it) reaches the shell at
	// Application.onCreate - before its first server spawn, so the
	// crash.log handoff is never consumed by a throwaway pre-config
	// server - and the intent extras repeat it as a fallback (the shell
	// treats a matching config as a no-op). Everything is base64ed:
	// port, token, then the env deltas (mode, declared args, WithEnv).
	cfgB64 := base64.StdEncoding.EncodeToString([]byte(
		strconv.Itoa(devicePort) + "\n" + token + "\n" + strings.Join(spec.overrides, "\n")))
	write := exec.Command(b.adb, "-s", b.serial, "shell",
		"run-as "+b.appID+" sh -c 'cat > files/.gantry-test.config'")
	write.Stdin = strings.NewReader(cfgB64)
	if out, err := write.CombinedOutput(); err != nil {
		stopLogcat()
		return nil, fmt.Errorf("writing the test config through run-as: %w\n%s", err, bytes.TrimSpace(out))
	}
	envB64 := base64.StdEncoding.EncodeToString([]byte(strings.Join(spec.overrides, "\n")))
	if _, err := b.adbOut("shell", "am", "start", "-n", b.appID+"/.MainActivity",
		"--ei", "gantry.test.port", strconv.Itoa(devicePort),
		"--es", "gantry.test.token", token,
		"--es", "gantry.test.env", envB64,
	); err != nil {
		stopLogcat()
		return nil, err
	}

	select {
	case <-ready:
	case <-time.After(spec.timeout):
		stopLogcat()
		_, _ = b.adbOut("shell", "am", "force-stop", b.appID)
		return nil, fmt.Errorf("timed out after %s waiting for GANTRY_READY port=%d in logcat (is the installed APK the debug build gantry test --device installs?)", spec.timeout, devicePort)
	}

	// adb forward tcp:0 allocates and prints the host-side port.
	fw, err := b.adbOut("forward", "tcp:0", "tcp:"+strconv.Itoa(devicePort))
	if err != nil {
		stopLogcat()
		return nil, err
	}
	localPort, err := strconv.Atoi(strings.TrimSpace(fw))
	if err != nil {
		stopLogcat()
		return nil, fmt.Errorf("parsing the adb forward port from %q: %v", fw, err)
	}

	p := &androidProc{b: b, localPort: localPort, devicePort: devicePort, tok: token, stopLogcat: stopLogcat, done: make(chan struct{})}
	go p.watchExit()
	return p, nil
}

// androidProc is one running app instance on the device: the forwarded
// local port, the runner-chosen token, and the logcat stream feeding
// app.log.
type androidProc struct {
	b          *androidBackend
	localPort  int
	devicePort int
	tok        string
	stopLogcat func()

	done     chan struct{}
	doneOnce sync.Once
	stopOnce sync.Once
}

func (p *androidProc) port() int               { return p.localPort }
func (p *androidProc) token() string           { return p.tok }
func (p *androidProc) exited() <-chan struct{} { return p.done }

// watchExit polls for the server process. A test-configured server
// stays dead once it exits (the debug shell suspends its supervisor
// restart), so a vanished pid is the fatal-crash signal WaitExit
// waits on. Two consecutive empty polls guard against one adb hiccup
// reading as an exit.
func (p *androidProc) watchExit() {
	misses := 0
	for {
		select {
		case <-p.done:
			return
		case <-time.After(300 * time.Millisecond):
		}
		out, _ := exec.Command(p.b.adb, "-s", p.b.serial, "shell", "pidof", "libgantryapp.so").Output()
		if len(bytes.TrimSpace(out)) == 0 {
			if misses++; misses >= 2 {
				p.doneOnce.Do(func() { close(p.done) })
				return
			}
		} else {
			misses = 0
		}
	}
}

func (p *androidProc) stop() {
	p.stopOnce.Do(func() {
		_, _ = p.b.adbOut("shell", "am", "force-stop", p.b.appID)
		_, _ = p.b.adbOut("forward", "--remove", "tcp:"+strconv.Itoa(p.localPort))
		// Remove the sandbox config so a manual app launch later does
		// not pick it up (the next test launch rewrites it).
		_, _ = p.b.adbOut("shell", "run-as", p.b.appID, "rm", "-f", "files/.gantry-test.config")
		p.doneOnce.Do(func() { close(p.done) })
		p.stopLogcat()
	})
}

// crashLogPath pulls the on-device crash.log out through run-as (the
// app runs with HOME=<filesDir>, so the runtime writes it to
// files/.config/<name>/crash.log inside the app sandbox; debuggable
// builds allow run-as) into a temp file for the artifact collector.
// Empty when there is none.
func (p *androidProc) crashLogPath() string {
	out, err := exec.Command(p.b.adb, "-s", p.b.serial, "exec-out", "run-as", p.b.appID,
		"cat", "files/.config/"+p.b.appName+"/crash.log").Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return ""
	}
	f, err := os.CreateTemp("", "gantrytest-crash-*.log")
	if err != nil {
		return ""
	}
	_, _ = f.Write(out)
	_ = f.Close()
	return f.Name()
}

// screenshot captures the device screen; screencap already emits PNG.
func (p *androidProc) screenshot() ([]byte, error) {
	out, err := exec.Command(p.b.adb, "-s", p.b.serial, "exec-out", "screencap", "-p").Output()
	if err != nil {
		return nil, fmt.Errorf("adb screencap: %w", err)
	}
	return out, nil
}
