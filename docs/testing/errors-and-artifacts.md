# Testing: errors and artifacts

The [error pipeline](../advanced/errors.md) is itself a test surface: a test can assert "this action produces error code X", or "nothing landed in the error pipeline during this test". And every run leaves artifacts that explain a failure without re-running it. This page is the reference for both.

## Asserting an error fires

```go
func TestCrashPipeline(t *testing.T) {
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	cerr := app.CallFail("pages/debug", "callBoom", nil) // the reply carries the gerr code...
	if cerr.Code != "panic.call" { t.Errorf("code = %q", cerr.Code) }

	e := app.WaitError("panic.call")                     // ...and the captured error frame, typed
	if e.Kind != "call-panic" { t.Errorf("kind = %q", e.Kind) }
	if e.Page != "pages/index" { t.Errorf("page = %q", e.Page) } // the page the user was on
	if len(e.Trail) == 0 { t.Error("expected a breadcrumb trail") }
}
```

`WaitError(code)` watches both delivery channels: live `{"t":"error"}` frames on the connection, and the `call("gantry","errors")` ring buffer - which also carries errors captured while disconnected and a previous run's crash (the [crash-recovery](#crash-recovery) case below). It consumes what it returns, keyed on a signature of code+time+message, so the same error is never returned twice and `ExpectNoErrors` will not trip on one you already claimed. Whatever it returns is an `ErrorInfo` (an alias of `ui.ErrorInfo`), carrying `Kind`, `Code`, `Source`, `Message`, `Stack`, `Time`, `Page` (the page the user was on) and `Trail` (the breadcrumb trail leading to the error).

The kinds and codes are real and stable, so you can assert on them precisely. A few from the demo suite: a panicking call handler is `panic.call` / `call-panic`; a paired-event handler panic is `panic.event` / `event-panic`; a Tea `Update` panic is `panic.update` / `tea-update-panic` (and the last good model survives); a `Cmd` panic is `panic.cmd` / `cmd-panic`; a `View` panic is `panic.view` / `tea-view-panic`; a `gantry.Go` goroutine panic is `panic.goroutine` / `goroutine-panic`; an uncaught frontend error crossing to Go is `js.error` / `js-error`; and an uncatchable fatal crash is `panic.fatal` / `process-crash`.

## Asserting no errors fired

```go
app.ExpectNoErrors()
```

This is the closing assertion for happy-path tests: any captured error the test did not explicitly claim with `WaitError` fails it, listed with code, kind, message and page. Because `WaitError` consumes the error it returns, "assert this one specific error, then no others" composes naturally - claim the `panic.call` you expected, then `ExpectNoErrors()` for everything else. When you need to inspect rather than assert, `Errors()` returns the live error frames seen on this connection and `RecentErrors()` fetches the ring buffer (what the frontend error UI would show, including errors captured while disconnected).

## Artifacts

Each test writes to `test-results/<TestName>/` (the test name is sanitized for the filesystem):

| File | What | When |
| --- | --- | --- |
| `app.log` | the app process's stdout and stderr (on device, the `gantry-go` logcat) | always |
| `trace.jsonl` | every protocol frame in and out plus every driver action, timestamped | always |
| `crash.log` | the runtime's fatal-panic trace, copied out when the process died that way | on a fatal crash |
| `failure.png` | what was on screen when the test failed | automatic on failure, DOM plane (or any device launch) |
| `console.log` | the webview's console output and uncaught JS exceptions | DOM plane |
| `<name>.png` | explicit `app.Screenshot("name")` / `el.Screenshot("name")` captures | DOM plane |
| `screencast.avi` | the whole test as MJPEG video | with `--record` / `WithRecording()`, DOM plane |

Passing tests keep nothing unless `gantry test --keep-artifacts` (or `gantrytest.KeepArtifacts()` per launch); recorded tests keep theirs regardless, since a screencast that vanishes on pass would be pointless. Failing tests always keep their directory, and the failure output names it. `trace.jsonl` is one JSON object per line - `{"time","dir":"send|recv|action","frame"|"msg"}` - the test-side mirror of the app's breadcrumb trail, and the source the report's Trace tab reads. The whole run is also rolled up into a self-contained `gantry_test_report.html` - see [the report](report.md).

Every timed-out wait also embeds the last 20 protocol frames straight into the test failure message, so the common case needs no artifact spelunking at all:

```
--- FAIL: TestSettingsSave (10.19s)
    settings_test.go:24: gantrytest: timed out waiting for push pages/settings.saved
        last protocol frames:
          {"t":"state","key":"volume","p":0.5}
          {"t":"render","seq":2,"tree":{...}}
    artifacts.go:66: gantrytest: artifacts in .../test-results/TestSettingsSave
```

## Notes

### Crash recovery

An uncatchable crash - a panic on a plain goroutine, not one under `gantry.Go` - leaves its trace in `crash.log`; the next launch reports it as a `panic.fatal` / `process-crash` error. The driver's per-test config dir makes this assertable, because the relaunch reuses the same dir and so finds the same `crash.log`:

```go
app.Ready("pages/index")
app.SendEvent("pages/debug", "fatalBoom", nil) // kills the process
app.WaitExit()                                 // wait for it to actually die (a complete crash.log)
app.Restart()                                  // hard kill + relaunch over the same config dir
e := app.WaitError("panic.fatal")              // the previous run's trace entered the pipeline
if e.Kind != "process-crash" { t.Errorf("kind = %q", e.Kind) }
// e.Stack mentions the panic site; e.Trail is empty (the process died before a snapshot)
```

`WaitExit()` before `Restart()` matters: it blocks until the process exits on its own, so the relaunch reads a complete `crash.log` rather than racing the dying process. See [driving the app](driving.md#restarts-and-crash-recovery).

### Artifacts under retries

Under [`gantry test --retries`](report.md), each attempt gets its own subdirectory - `test-results/<TestName>/attempt-k/` - so a flaky test keeps the artifacts of every try, and the report shows them per attempt. A normal run (no `--retries`) uses the flat `test-results/<TestName>/` shown above.

Next: [widget snapshots](widgets.md).
