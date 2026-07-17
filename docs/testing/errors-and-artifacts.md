# Testing: errors and artifacts

The [error pipeline](../advanced/errors.md) is itself a test surface: a test can assert "this action produces error code X", or "nothing landed in the error pipeline during this test", and every run leaves artifacts that explain a failure without re-running it.

## Asserting an error fires

```go
func TestCrashPipeline(t *testing.T) {
	app := gantrytest.Launch(t)
	app.Ready("pages/index")

	cerr := app.CallFail("pages/debug", "callBoom", nil) // the reply carries the gerr code...
	if cerr.Code != "panic.call" { t.Errorf(...) }

	e := app.WaitError("panic.call")                     // ...and the captured error frame, typed
	if e.Kind != "call-panic" { t.Errorf(...) }
	if e.Page != "pages/index" { t.Errorf(...) }         // the page the user was on
	if len(e.Trail) == 0 { t.Error("expected a breadcrumb trail") }
}
```

`WaitError(code)` watches both delivery channels: live `{"t":"error"}` frames on the connection, and the `call("gantry","errors")` ring buffer - which also carries errors captured while disconnected and a previous run's crash. Whatever it returns is an `ErrorInfo` (`ui.ErrorInfo`): kind, code, source, message, stack, page, trail.

## Asserting no errors fired

```go
app.ExpectNoErrors()
```

This is the closing assertion for happy-path tests: any captured error the test did not explicitly claim with `WaitError` fails it, listed with code, kind, message and page. `WaitError` consumes what it returns, so "assert this one specific error, then no others" composes naturally.

`Errors()` returns the raw error frames seen on this connection, and `RecentErrors()` fetches the ring buffer, when you need to inspect rather than assert.

## Crash recovery

An uncatchable crash (a panic on a plain goroutine) leaves its trace in crash.log; the next launch reports it as a `panic.fatal` / `process-crash` error. The driver's per-test config dir makes this assertable:

```go
app.Restart()                        // hard kill + relaunch with the same config dir
e := app.WaitError("panic.fatal")    // the previous run's trace entered the pipeline
```

## Artifacts

Each test writes to `test-results/<TestName>/`:

| File | What |
| --- | --- |
| `app.log` | the app process's stdout and stderr |
| `trace.jsonl` | every protocol frame in and out plus every driver action, timestamped - the test-side mirror of the app's breadcrumb trail |
| `crash.log` | the runtime's fatal-panic trace, when the process died that way |
| `failure.png` | what was on screen when the test failed - automatic on [DOM-plane](dom.md) tests |
| `console.log` | the webview's console output and uncaught JS exceptions (DOM plane) |
| `<name>.png` | explicit `app.Screenshot("name")` / `el.Screenshot("name")` captures (DOM plane) |
| `screencast.avi` | the whole test as video, with `gantry test --record` / `WithRecording()` (DOM plane) |

Passing tests keep nothing unless `gantry test --keep-artifacts` (or `gantrytest.KeepArtifacts()` per launch); recorded tests keep theirs regardless, since a screencast that vanishes on pass would be pointless. Failing tests always keep their directory, and the failure output names it.

Under [`gantry test --retries`](report.md), each attempt gets its own subdirectory - `test-results/<TestName>/attempt-k/` - so a flaky test keeps the artifacts of every try. A normal run (no `--retries`) uses the flat `test-results/<TestName>/` shown above. Either way, the whole run is also rolled up into a self-contained `gantry_test_report.html` - see [the report](report.md).

Every timed-out wait also embeds the last 20 protocol frames straight into the test failure message, so the common case needs no artifact spelunking at all:

```
--- FAIL: TestSettingsSave (10.19s)
    settings_test.go:24: gantrytest: timed out waiting for push pages/settings.saved
        last protocol frames:
          {"t":"state","key":"volume","p":0.5}
          {"t":"render","seq":2,"tree":{...}}
    artifacts.go:66: gantrytest: artifacts in .../test-results/TestSettingsSave
```

