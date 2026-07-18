// Package gantrytest is Gantry's end-to-end test driver: it launches a
// real app under test (the real Go process, real websocket), speaks
// the wire protocol like the frontend would, and gives tests the
// protocol-plane verbs - mount pages, fire events, await calls, read
// renders/pushes/shared state, and assert on the error pipeline.
//
// Test files are plain Go tests in the app repo under tests/, run by
// `gantry test` (which prebuilds the app and frontend) or a bare
// `go test ./tests/...`:
//
//	func TestCounter(t *testing.T) {
//		app := gantrytest.Launch(t)
//		app.Ready("pages/index")
//		btn := app.Tree().Find("button", gantrytest.Text("count is"))
//		app.Click(btn)
//		app.WaitTree("count advanced", func(n *gantrytest.Node) bool {
//			return strings.Contains(n.Text(), "count is 1")
//		})
//		app.ExpectNoErrors()
//	}
//
// Everything here is headless and cross-platform (tier 1 of the
// testing plan): a Tea-style app plus every service/state/error
// behavior tests end to end with no browser tooling. The DOM plane
// (element handles over CDP) is tier 2 and layers onto the same App.
package gantrytest

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/B-Commissions/Gantry/gantrytest/report"
)

// launchConfig is the resolved Launch options.
type launchConfig struct {
	appDir       string
	binPath      string
	mode         string
	headed       bool
	dom          bool
	record       bool
	keep         bool
	timeout      time.Duration
	args         map[string]any
	env          []string
	artifactRoot string
	grant        []string // permissions to pm-grant before launch (device only)
	revoke       []string // permissions to pm-revoke before launch (device only)
}

// Option configures Launch.
type Option func(*launchConfig)

// WithArgs sets the app's declared args (gantry.json "args") for this
// instance; they travel as the same environment variables gantry dev
// uses. Unknown names fail the test.
func WithArgs(args map[string]any) Option {
	return func(c *launchConfig) { c.args = args }
}

// WithMode runs the app in "development" or "production" mode. Tests
// default to development so error detail is full; override per test to
// assert production behavior.
func WithMode(mode string) Option {
	return func(c *launchConfig) { c.mode = mode }
}

// WithHeaded opens the real window instead of running headless. On
// Windows a headed launch also gets the DOM plane, like WithDOM.
func WithHeaded() Option {
	return func(c *launchConfig) { c.headed = true }
}

// WithDOM enables the DOM plane: the app opens a real webview - parked
// off-screen unless headed - and the driver attaches over CDP. Unlocks
// App.Page, Find, element actions and screenshots. On desktop this is
// WebView2 on Windows (validated) or WebKitGTK on Linux (experimental -
// inspector-based and WebKit-version-dependent). On a device target
// (tier M2) it attaches to the Android WebView's devtools socket, and
// clicks map to real touch events.
func WithDOM() Option {
	return func(c *launchConfig) { c.dom = true }
}

// WithRecording records a screencast of the whole test into the
// artifact directory as screencast.avi (kept even on pass). Needs the
// DOM plane; `gantry test --record` turns it on suite-wide. No-op on a
// launch without a webview.
func WithRecording() Option {
	return func(c *launchConfig) { c.record = true }
}

// WithEnv adds one environment variable to the app process.
func WithEnv(key, value string) Option {
	return func(c *launchConfig) { c.env = append(c.env, key+"="+value) }
}

// WithGrantedPermissions pre-grants Android runtime permissions before
// the app launches, so a test can assert a granted flow without driving
// the system permission prompt. Device-only (a no-op on desktop). Names
// are full Android permission strings, e.g.
// "android.permission.POST_NOTIFICATIONS".
func WithGrantedPermissions(perms ...string) Option {
	return func(c *launchConfig) { c.grant = append(c.grant, perms...) }
}

// WithDeniedPermissions pre-revokes Android runtime permissions before
// the app launches, so a test can assert a denied flow. Device-only.
func WithDeniedPermissions(perms ...string) Option {
	return func(c *launchConfig) { c.revoke = append(c.revoke, perms...) }
}

// WithBinary launches a prebuilt binary instead of building the app.
func WithBinary(path string) Option {
	return func(c *launchConfig) { c.binPath = path }
}

// WithAppDir overrides app-root discovery (default: GANTRY_TEST_APP_DIR,
// else walk up from the test's working directory to gantry.json).
func WithAppDir(dir string) Option {
	return func(c *launchConfig) { c.appDir = dir }
}

// WithTimeout sets the default deadline for every waiting API on this
// app (default 10s), still capped by the test's own deadline.
func WithTimeout(d time.Duration) Option {
	return func(c *launchConfig) { c.timeout = d }
}

// KeepArtifacts keeps this test's artifacts even when it passes.
func KeepArtifacts() Option {
	return func(c *launchConfig) { c.keep = true }
}

// App is one running app instance under test, owning the process, the
// protocol connection, and the artifact directory. Launch cleans all
// of it up via t.Cleanup.
type App struct {
	t      testing.TB
	cfg    launchConfig
	appCfg appConfig
	appDir string

	be       backend
	onDevice bool
	spec     launchSpec
	proc     proc
	client   *client
	art      *artifacts

	// DOM plane (WithDOM / headed on Windows): the webview's devtools
	// port, the CDP connection attached to it, and the screencast
	// recorder when this run records.
	debugPort int
	cdp       domClient
	rec       *recorder

	// Report data layer (see result.go): which surface this launch drove,
	// its worker index and source location, when it started, the 1-based
	// retry attempt, and a driver-raised failure captured structurally.
	plane   report.Plane
	worker  int
	srcFile string
	srcLine int
	started time.Time
	attempt int
	failure *report.Failure
}

// Launch builds (or reuses) the app under test, starts it headless on
// an ephemeral port with a hermetic per-test config dir, connects to
// the wire protocol, and registers cleanup: the process tree is
// killed, crash.log is collected, and artifacts are kept on failure.
func Launch(t testing.TB, opts ...Option) *App {
	t.Helper()

	cfg := launchConfig{
		mode:         envDefault("GANTRY_TEST_MODE", "development"),
		headed:       os.Getenv("GANTRY_TEST_HEADED") == "1",
		record:       os.Getenv("GANTRY_TEST_RECORD") == "1",
		keep:         os.Getenv("GANTRY_TEST_KEEP_ARTIFACTS") == "1",
		timeout:      10 * time.Second,
		artifactRoot: os.Getenv("GANTRY_TEST_ARTIFACTS"),
	}
	for _, o := range opts {
		o(&cfg)
	}

	// A device target launches the installed APK through the adb backend
	// instead of a local process; both planes transfer - the protocol
	// plane since tier M1, the DOM plane (over the device WebView's
	// devtools socket) since tier M2.
	onDevice := Target() != "desktop"
	if onDevice && Target() != "android" {
		t.Skipf("gantrytest: unsupported device target %q (android is what tier M1 ships; ios comes later)", Target())
	}

	// The desktop DOM plane needs a CDP-speaking webview: WebView2. Linux
	// runs WebKitGTK (its own inspector protocol) - that lands later; the
	// protocol plane stays fully cross-platform. The device DOM plane is
	// adb-driven, so the host OS does not matter there.
	if cfg.dom && !onDevice && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skipf("gantrytest: the desktop DOM plane needs WebView2 (Windows) or WebKitGTK (Linux); host is %s", runtime.GOOS)
	}
	domHost := runtime.GOOS == "windows" || runtime.GOOS == "linux"
	dom := cfg.dom || (cfg.headed && !onDevice && domHost)
	// Recording rides the DOM plane's screencast on every backend: desktop
	// WebView2, and - since tier 3 - the device WebView over its forwarded
	// devtools socket (the same CDP Page.startScreencast path).
	record := cfg.record && dom

	appDir := cfg.appDir
	if appDir == "" {
		appDir = os.Getenv("GANTRY_TEST_APP_DIR")
	}
	if appDir == "" {
		var err error
		appDir, err = findAppDir(".")
		if err != nil {
			t.Fatalf("gantrytest: %v", err)
		}
	}
	appCfg, err := readAppConfig(appDir)
	if err != nil {
		t.Fatalf("gantrytest: %v", err)
	}

	// The device runs the installed APK; only desktop targets need the
	// host binary.
	bin := cfg.binPath
	if bin == "" && !onDevice {
		bin = testBinary(t, appDir, appCfg)
	}

	artRoot := cfg.artifactRoot
	if artRoot == "" {
		artRoot = fmt.Sprintf("%s%ctest-results", appDir, os.PathSeparator)
	}
	cfg.artifactRoot = artRoot // the report layer reads it back for the run-records dir
	// Under `gantry test --retries`, each attempt gets its own artifact
	// subdir (test-results/<name>/attempt-k); a normal run stays flat.
	attempt := attemptFromEnv()
	// A recorded run keeps its artifacts - a screencast that vanishes on
	// pass would be pointless.
	art := newArtifacts(t, artRoot, cfg.keep || record, attempt)

	// The test's env deltas, kept separate from the full process env:
	// device backends can only inject the deltas (the shell owns the
	// rest of the server's environment).
	args := argEnv(t, appCfg, cfg.args)
	overrides := append([]string{"GANTRY_MODE=" + cfg.mode}, args...)
	overrides = append(overrides, cfg.env...)

	var be backend = localBackend{}
	var env []string
	var configDir string
	var debugPort int
	if onDevice {
		be = androidBackendFromEnv(t, appCfg)
	} else {
		configDir = tempConfigDir(t)
		env = append(os.Environ(), "GANTRY_MODE="+cfg.mode)
		env = append(env, args...)
		env = append(env, configDirEnv(configDir)...)
		if dom {
			// A per-instance remote-inspector port, passed the webview's
			// own official way: WebView2 reads a Chromium flag from an env
			// var; WebKitGTK reads WEBKIT_INSPECTOR_SERVER=host:port
			// natively. The window opens for real but parks off-screen
			// unless the test is headed (Linux CI drives it under xvfb).
			debugPort = freeTCPPort(t)
			if runtime.GOOS == "linux" {
				env = append(env, "WEBKIT_INSPECTOR_SERVER=127.0.0.1:"+strconv.Itoa(debugPort))
			} else {
				env = append(env, "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port="+strconv.Itoa(debugPort))
			}
			if !cfg.headed {
				env = append(env, "GANTRY_WINDOW_OFFSCREEN=1")
			}
		}
		env = append(env, cfg.env...)
	}

	srcFile, srcLine := testSourceLoc(appDir)
	a := &App{
		t: t, cfg: cfg, appCfg: appCfg, appDir: appDir,
		be:        be,
		onDevice:  onDevice,
		art:       art,
		debugPort: debugPort,
		plane:     planeFor(onDevice, dom),
		worker:    int(workerSeq.Add(1)),
		srcFile:   srcFile,
		srcLine:   srcLine,
		started:   time.Now(),
		attempt:   attempt,
		spec: launchSpec{
			bin: bin, appDir: appDir, appName: appCfg.Name,
			configDir: configDir, env: env, overrides: overrides,
			window: cfg.headed || dom,
			appLog: art.appLog, timeout: 60 * time.Second,
			grant: cfg.grant, revoke: cfg.revoke,
		},
	}
	t.Cleanup(a.teardown)

	if onDevice {
		art.trace.action("launch %s on %s (mode=%s)", appCfg.Name, Target(), cfg.mode)
	} else {
		art.trace.action("launch %s (mode=%s headed=%v dom=%v)", bin, cfg.mode, cfg.headed, dom)
	}
	a.proc, err = a.be.launch(a.spec)
	if err != nil {
		t.Fatalf("gantrytest: launching the app: %v", err)
	}
	art.trace.action("app ready on port %d", a.proc.port())

	// With a webview attached the driver connects as an observer, so it
	// never displaces the frontend's connection (the server allows one
	// real client and any number of observers). On a device the app's
	// own WebView is always attached, so the driver always observes.
	a.client, err = dialClient(t, art.trace, a.proc.port(), cfg.timeout, dom || onDevice, a.proc.token())
	if err != nil {
		t.Fatalf("gantrytest: %v", err)
	}
	if dom {
		if record {
			a.rec = &recorder{}
		}
		a.attachDOM()
	}
	return a
}

// domForwarder is a device proc that can expose its webview's devtools
// socket on a host port (adb forward); desktop resolves its debug port
// up front via the WebView2 launch env instead.
type domForwarder interface {
	forwardDevtools() (int, error)
}

// attachDOM connects the CDP client to the webview (and starts the
// screencast when this run records). The deadline is generous: the
// first attach includes a cold WebView start and the frontend's initial
// load. On a device the devtools socket is reached through a second adb
// forward resolved here; desktop uses the port set at launch.
func (a *App) attachDOM() {
	a.t.Helper()
	debugPort := a.debugPort
	if a.onDevice {
		fwd, ok := a.proc.(domForwarder)
		if !ok {
			a.t.Fatalf("gantrytest: the device backend cannot forward a devtools socket")
		}
		p, err := fwd.forwardDevtools()
		if err != nil {
			a.t.Fatalf("gantrytest: forwarding the device devtools socket: %v", err)
		}
		debugPort = p
	}
	var d domClient
	var err error
	if !a.onDevice && runtime.GOOS == "linux" {
		// Linux desktop runs WebKitGTK, which speaks the WebKit remote
		// inspector protocol, not CDP - a different dialect behind the same
		// seam (see webkit.go).
		d, err = attachWebKit(a.art.trace, debugPort, a.proc.webPort(), a.art.path("console.log"), 30*time.Second)
	} else {
		d, err = attachCDP(a.art.trace, debugPort, a.proc.webPort(), a.art.path("console.log"), 30*time.Second)
	}
	if err != nil {
		a.t.Fatalf("gantrytest: attaching the DOM plane: %v", err)
	}
	a.cdp = d
	if a.rec != nil {
		if err := d.startScreencast(a.rec.add); err != nil {
			a.t.Logf("gantrytest: starting the screencast: %v", err)
		}
	}
}

func (a *App) teardown() {
	if a.cdp != nil {
		// The automatic failure artifact: what was on screen when the
		// test died, captured while the app is still alive.
		if a.t.Failed() {
			if data, err := a.cdp.screenshot(nil); err == nil {
				a.art.savePNG(a.t, "failure", data)
			}
		}
		if a.rec != nil {
			a.cdp.stopScreencast()
		}
		a.cdp.close()
		a.cdp = nil
	}
	if a.rec != nil {
		written, err := a.rec.writeAVI(a.art.path("screencast.avi"), 12)
		switch {
		case err != nil:
			a.t.Logf("gantrytest: writing screencast.avi: %v", err)
		case written:
			a.t.Logf("gantrytest: screencast %s", a.art.path("screencast.avi"))
		}
		a.rec = nil
	}
	// Without a DOM plane the process-control plane may still have a
	// screen to shoot (device backends): the same automatic failure
	// artifact, captured before the app is stopped.
	if a.t.Failed() && a.cdp == nil && a.proc != nil {
		if data, err := a.proc.screenshot(); err == nil {
			a.art.savePNG(a.t, "failure", data)
		}
	}
	if a.client != nil {
		a.client.close()
	}
	if a.proc != nil {
		a.proc.stop()
		a.art.saveFile("crash.log", a.proc.crashLogPath())
	}
	// The report record is written before finalize deletes a passing,
	// non-kept artifact dir: it lives in the run-scoped dir, so the
	// overview still lists this test with its plane, timing and worker.
	a.writeRecord()
	a.art.finalize(a.t)
	if a.spec.configDir != "" {
		removeTempDir(a.t, a.spec.configDir)
	}
}

// tempConfigDir makes the per-test stand-in for the user config dir.
// Deliberately not t.TempDir: on Windows the webview's data folder
// lives in here, and its browser processes can hold file locks for a
// beat after the kill - t.TempDir's cleanup would fail the test over
// it. Removal is retried and best-effort instead.
func tempConfigDir(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gantrytest-")
	if err != nil {
		t.Fatalf("gantrytest: creating the config dir: %v", err)
	}
	return dir
}

func removeTempDir(t testing.TB, dir string) {
	var err error
	for i := 0; i < 10; i++ {
		if err = os.RemoveAll(dir); err == nil {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Logf("gantrytest: could not remove the temp config dir %s: %v (still locked by a dying webview?)", dir, err)
}

// Restart kills the app hard and relaunches it with the same binary,
// config dir and environment - the crash.log / process-crash scenario:
// after a fatal crash, the relaunched app reports the previous run's
// trace as a "panic.fatal" error (WaitError("panic.fatal")).
func (a *App) Restart() {
	a.t.Helper()
	a.art.trace.action("restart: hard-killing the app")
	if a.cdp != nil {
		a.cdp.close()
		a.cdp = nil
	}
	a.client.close()
	a.proc.stop()
	// The relaunch truncates crash.log after reading it - keep a copy.
	a.art.saveFile("crash.log", a.proc.crashLogPath())

	var err error
	a.proc, err = a.be.launch(a.spec)
	if err != nil {
		a.t.Fatalf("gantrytest: relaunching the app: %v", err)
	}
	a.art.trace.action("app ready on port %d after restart", a.proc.port())
	a.client, err = dialClient(a.t, a.art.trace, a.proc.port(), a.cfg.timeout, a.spec.window || a.onDevice, a.proc.token())
	if err != nil {
		a.t.Fatalf("gantrytest: %v", err)
	}
	if a.spec.window {
		a.attachDOM()
	}
}

// WaitExit blocks until the app process exits on its own - the fatal
// crash scenario, where an uncatchable panic kills the app and the
// trace lands in crash.log. Pair it with Restart so the relaunch reads
// a complete crash.log rather than racing the dying process.
func (a *App) WaitExit() {
	a.t.Helper()
	a.art.trace.action("waiting for the app process to exit")
	select {
	case <-a.proc.exited():
		a.art.trace.action("app process exited")
	case <-time.After(a.cfg.timeout):
		a.t.Fatalf("gantrytest: timed out after %s waiting for the app process to exit", a.cfg.timeout)
	}
}

// Port is the app's HTTP/websocket port for this launch.
func (a *App) Port() int { return a.proc.port() }

// URL is the app's base URL for this launch.
func (a *App) URL() string { return fmt.Sprintf("http://127.0.0.1:%d", a.proc.port()) }

// ---- protocol plane ----

// Ready announces a page mount, like the frontend does when a page
// component mounts: the server makes it the active page, and a Tea
// page responds with a full render (see Tree / NextRender).
func (a *App) Ready(page string) {
	a.t.Helper()
	a.client.ready(page)
}

// NextRender waits for a render the test has not consumed yet and
// returns its tree.
func (a *App) NextRender() *Node {
	a.t.Helper()
	return a.parseTree(a.client.nextRender())
}

// Tree waits for at least one render and returns the newest tree.
func (a *App) Tree() *Node {
	a.t.Helper()
	return a.parseTree(a.client.latestTree("a render", nil))
}

// WaitTree waits until the newest render's tree satisfies pred -
// renders are whole-tree and may coalesce, so this is the reliable way
// to await a state change ("count advanced", "row appeared").
func (a *App) WaitTree(what string, pred func(*Node) bool) *Node {
	a.t.Helper()
	raw := a.client.latestTree(what, func(tree json.RawMessage) bool {
		n, err := parseTree(tree)
		if err != nil {
			return false
		}
		return pred(n)
	})
	return a.parseTree(raw)
}

func (a *App) parseTree(raw json.RawMessage) *Node {
	a.t.Helper()
	n, err := parseTree(raw)
	if err != nil {
		a.t.Fatalf("gantrytest: parsing render tree: %v", err)
	}
	return n
}

// TeaEvent fires a Tea handler by its id from the current render tree
// (Node.Handler). Payload nil sends none.
func (a *App) TeaEvent(handlerID string, payload any) {
	a.t.Helper()
	if handlerID == "" {
		a.t.Fatalf("gantrytest: TeaEvent with an empty handler id (did Find miss, or the node has no such handler?)")
	}
	a.client.teaEvent(handlerID, payload)
}

// Click fires the node's click handler - sugar for
// TeaEvent(n.Handler("click"), nil).
func (a *App) Click(n *Node) {
	a.t.Helper()
	if n == nil {
		a.t.Fatalf("gantrytest: Click(nil) - the Find that produced this node matched nothing")
	}
	h := n.Handler("click")
	if h == "" {
		a.t.Fatalf("gantrytest: node %q (%s) has no click handler", n.Key, n.Type)
	}
	a.client.teaEvent(h, nil)
}

// SendEvent fires a paired event: key names the page/component pair,
// name the handler in its ui.Handlers.
func (a *App) SendEvent(key, name string, payload any) {
	a.t.Helper()
	a.client.sendEvent(key, name, payload)
}

// Call awaits a service/paired call and fails the test if it rejects.
// Decode the reply with json.Unmarshal.
func (a *App) Call(key, name string, payload any) json.RawMessage {
	a.t.Helper()
	p, cerr := a.client.call(key, name, payload)
	if cerr != nil {
		a.t.Fatalf("gantrytest: call %s.%s rejected: %v", key, name, cerr)
	}
	return p
}

// CallFail awaits a call that must reject and returns the typed
// error, for asserting "this action produces error code X".
func (a *App) CallFail(key, name string, payload any) *CallError {
	a.t.Helper()
	p, cerr := a.client.call(key, name, payload)
	if cerr == nil {
		a.t.Fatalf("gantrytest: call %s.%s succeeded (%s) but the test expected it to fail", key, name, p)
	}
	return cerr
}

// SetState writes a shared state variable, like the frontend's
// useGoState setter.
func (a *App) SetState(key string, value any) {
	a.t.Helper()
	a.client.setState(key, value)
}

// State waits for the shared state variable to exist (the snapshot
// arrives on connect) and returns its current value.
func (a *App) State(key string) Value {
	a.t.Helper()
	var raw json.RawMessage
	a.client.wait("state "+key, func() bool {
		r, ok := a.client.states[key]
		if ok {
			raw = r
		}
		return ok
	})
	return Value{t: a.t, key: key, raw: raw}
}

// States returns a snapshot of every shared state value seen so far.
func (a *App) States() map[string]json.RawMessage {
	a.client.mu.Lock()
	defer a.client.mu.Unlock()
	out := make(map[string]json.RawMessage, len(a.client.states))
	for k, v := range a.client.states {
		out[k] = v
	}
	return out
}

// WaitPush waits for the next unconsumed paired push for key/name
// (including one that landed just before the call) and returns its
// payload.
func (a *App) WaitPush(key, name string) json.RawMessage {
	a.t.Helper()
	return a.client.waitPush(key, name)
}

// WaitFor polls cond until it is true or the deadline passes.
func (a *App) WaitFor(what string, cond func() bool) {
	a.t.Helper()
	deadline := a.client.deadline()
	for !cond() {
		if !time.Now().Before(deadline) {
			a.client.mu.Lock()
			dump := a.client.lastFramesLocked(20)
			a.client.mu.Unlock()
			a.t.Fatalf("gantrytest: timed out waiting for %s\nlast protocol frames:\n%s", what, dump)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// ---- the error pipeline as a test surface ----

// WaitError waits for a captured error with the gerr code, consuming
// it so ExpectNoErrors will not trip on it. It watches both channels:
// live {"t":"error"} frames and the call("gantry","errors") ring
// buffer (which also carries errors captured before this connection,
// like a previous run's crash after Restart).
func (a *App) WaitError(code string) ErrorInfo {
	a.t.Helper()
	deadline := a.client.deadline()
	for {
		if e, ok := a.client.takeErrorFrame(code); ok {
			return e
		}
		for _, e := range a.RecentErrors() {
			if e.Code == code && !a.client.isConsumed(e) {
				a.client.markConsumed(e)
				return e
			}
		}
		if !time.Now().Before(deadline) {
			a.client.mu.Lock()
			dump := a.client.lastFramesLocked(20)
			a.client.mu.Unlock()
			a.t.Fatalf("gantrytest: timed out waiting for error %q\nlast protocol frames:\n%s", code, dump)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Errors returns every live error frame captured on this connection.
func (a *App) Errors() []ErrorInfo {
	return a.client.snapshotErrors()
}

// RecentErrors fetches the app's error ring buffer - what the frontend
// error UI would show, including errors captured while disconnected.
func (a *App) RecentErrors() []ErrorInfo {
	a.t.Helper()
	p, cerr := a.client.call("gantry", "errors", nil)
	if cerr != nil {
		a.t.Fatalf("gantrytest: call gantry.errors rejected: %v", cerr)
	}
	var errs []ErrorInfo
	if len(p) > 0 && string(p) != "null" {
		if err := json.Unmarshal(p, &errs); err != nil {
			a.t.Fatalf("gantrytest: decoding gantry.errors reply: %v", err)
		}
	}
	return errs
}

// ExpectNoErrors fails the test when any error was captured that a
// WaitError did not already claim - the default closing assertion for
// happy-path tests.
func (a *App) ExpectNoErrors() {
	a.t.Helper()
	seen := map[string]bool{}
	var unexpected []ErrorInfo
	for _, e := range append(a.client.snapshotErrors(), a.RecentErrors()...) {
		sig := errSignature(e)
		if seen[sig] || a.client.isConsumed(e) {
			continue
		}
		seen[sig] = true
		unexpected = append(unexpected, e)
	}
	if len(unexpected) == 0 {
		return
	}
	var b strings.Builder
	for _, e := range unexpected {
		fmt.Fprintf(&b, "  [%s] %s: %s (page=%s)\n", e.Code, e.Kind, e.Message, e.Page)
	}
	a.t.Fatalf("gantrytest: %d unexpected error(s) captured during the test:\n%s", len(unexpected), strings.TrimRight(b.String(), "\n"))
}

// ---- shared state values ----

// Value is one shared state value with typed accessors; a type
// mismatch fails the test with the key and raw value.
type Value struct {
	t   testing.TB
	key string
	raw json.RawMessage
}

// Raw is the wire JSON.
func (v Value) Raw() json.RawMessage { return v.raw }

// Decode unmarshals the value into out.
func (v Value) Decode(out any) {
	v.t.Helper()
	if err := json.Unmarshal(v.raw, out); err != nil {
		v.t.Fatalf("gantrytest: state %q: decoding %s: %v", v.key, v.raw, err)
	}
}

// Float returns the value as a float64.
func (v Value) Float() float64 {
	v.t.Helper()
	var f float64
	v.Decode(&f)
	return f
}

// Int returns the value as an int.
func (v Value) Int() int {
	v.t.Helper()
	var i int
	v.Decode(&i)
	return i
}

// Bool returns the value as a bool.
func (v Value) Bool() bool {
	v.t.Helper()
	var b bool
	v.Decode(&b)
	return b
}

// String returns the value as a string.
func (v Value) String() string {
	v.t.Helper()
	var s string
	v.Decode(&s)
	return s
}

// ---- targets and capabilities ----

// Target reports what the suite is running against: "desktop", or
// "android" under `gantry test --device android` (tier M1).
func Target() string {
	if d := os.Getenv("GANTRY_TEST_DEVICE"); d != "" {
		if i := strings.IndexByte(d, ':'); i >= 0 {
			return d[:i]
		}
		return d
	}
	return "desktop"
}

// DesktopOnly skips the test unless it runs against a desktop target.
func DesktopOnly(t testing.TB) {
	t.Helper()
	if Target() != "desktop" {
		t.Skipf("desktop-only test (target %s)", Target())
	}
}

// MobileOnly skips the test unless it runs against a mobile target.
func MobileOnly(t testing.TB) {
	t.Helper()
	if Target() == "desktop" {
		t.Skip("mobile-only test (target desktop)")
	}
}

// Capability is a driver feature shared tests can gate on with
// App.Supports instead of forking per target.
type Capability string

const (
	// DOM is element-level driving over the webview's devtools
	// protocol (tier 2).
	DOM Capability = "dom"
	// Hover is mouse hover; device backends report false (a phone has no
	// hover).
	Hover Capability = "hover"
	// Touch is touch input. A device delivers real taps (native adb
	// input, since Android WebView's CDP touch dispatch hangs); desktop
	// is mouse and reports false.
	Touch Capability = "touch"
	// Notifications is system-notification inspection, available on a
	// device target.
	Notifications Capability = "notifications"
)

// Supports reports whether this launch provides the capability: DOM
// when the CDP plane is attached (WithDOM / headed on Windows / device);
// Hover on desktop only; Touch and Notifications on a device target.
func (a *App) Supports(c Capability) bool {
	switch c {
	case DOM:
		return a.cdp != nil
	case Hover:
		return a.cdp != nil && !a.onDevice // a phone has no hover
	case Touch:
		return a.onDevice
	case Notifications:
		return a.onDevice
	}
	return false
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// freeTCPPort reserves an ephemeral port and releases it for the
// webview's devtools endpoint to bind. The tiny reuse race is
// acceptable for a per-instance test port.
func freeTCPPort(t testing.TB) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("gantrytest: reserving a devtools port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}
