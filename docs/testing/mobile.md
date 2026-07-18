# Testing: mobile

> **Status: both planes work on Android device.** `gantry test --device android` runs the same test files against a plugged-in phone or an emulator - the protocol plane (`Ready`, `Call`, `WaitError`, tree queries, state, the crash story) and the DOM plane (`Page`, `Find`, `Click`, screenshots) both work unchanged. Mobile-specific helpers - `Back`, `Rotate`, `Notifications`, permission pre-grants - layer on top. **iOS is not there yet** (see [What does not transfer yet](#what-does-not-transfer-yet)).

Mobile is a first-class target, not an afterthought, because of a fact about Gantry's architecture: the APK is not a different app. It is the same Go server (packed as `libgantryapp.so`, launched by the Kotlin shell) serving the same frontend to an Android WebView. So the test driver's planes transfer rather than being rebuilt - the same test files run on desktop and device wherever the surfaces overlap.

## Running on a device

```
gantry test --device android          # sole connected device or emulator
gantry test --device android:SERIAL   # pin one
```

The target needs a `"mobile"` section with an `"id"` in `gantry.json` (e.g. `"mobile": {"id": "com.example.myapp"}`), because that id is what gets installed and driven. `Target()` returns `"android"` under this flag, and `gantrytest.MobileOnly(t)` / `DesktopOnly(t)` fence the target-specific tests; `app.Supports(...)` gates capability differences inside shared tests. Everything above the backend is the same driver - a test that mounts pages, fires events and asserts on calls/state/errors runs identically on both targets:

```go
func TestSaveNotifies(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t) // the adb backend when --device android is set
	app.Ready("pages/settings")
	// ... same driver API as desktop ...
}
```

> **The device must be unlocked.** The DOM plane drives and captures the real WebView, and a locked or asleep screen puts it behind the keyguard - Chromium throttles the hidden page, so CDP stops replying, taps land nowhere and screenshots come back black. The runner wakes the screen and holds it on (`svc power stayon` + `input keyevent KEYCODE_WAKEUP` + `wm dismiss-keyguard`), but a *secure* lock (PIN/pattern/biometric) can only be cleared by you - unlock the phone, or set it to a swipe/no lock for testing. An emulator is always unlocked.

## Mobile-specific helpers

Layered on the same driver, device-only; each fails on desktop with `MobileOnly` guidance ("run against a device (gantry test --device android)").

- **`app.Back()`** presses the system back button (`KEYCODE_BACK`), exercising the shell's WebView history / exit behavior.
- **`app.Rotate()`** forces a configuration change (portrait ↔ landscape via `settings put system user_rotation`). The activity and WebView are recreated while the Go backend - owned by the `Application` - keeps running, so the protocol connection survives; the DOM plane is re-attached automatically (the process, hence the forwarded devtools socket, is unchanged). The point of the test is that the socket is not dropped - assert with `app.ExpectNoErrors()` and a DOM query after the call.
- **`app.Notifications()`** reads the notifications the app has currently posted (via `dumpsys notification`). It returns a `*NotificationSet`: `.Find("Saved")` matches the first notification whose title or text contains the substring (returning a `*Notification` with `Title`, `Text`, `Actions`, or nil), and `.All()` returns them all. Android only surfaces these when the app holds `POST_NOTIFICATIONS`.
- **`app.TapAction(notificationID, actionID)`** relays a notification action-button tap to the app's action receiver via a broadcast - the same path a real tap takes.
- **Permission pre-grants**: `gantrytest.WithGrantedPermissions(...)` / `WithDeniedPermissions(...)` set an Android runtime permission *before* the app launches, so a test can assert granted and denied flows without driving the system prompt; `app.Grant(perm)` / `app.Revoke(perm)` do it mid-test. Names are full Android permission strings.

```go
func TestSaveNotifies(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t,
		gantrytest.WithDOM(),
		gantrytest.WithGrantedPermissions("android.permission.POST_NOTIFICATIONS"),
	)
	app.Page("/debug").Find("button", gantrytest.Text("Notify")).Click()
	app.WaitFor("the Saved notification", func() bool {
		return app.Notifications().Find("Saved") != nil
	})
	app.Rotate()         // configuration change...
	app.ExpectNoErrors() // ...must not drop the socket or error
}
```

## Notes

### What the runner does

`gantry test --device android` resolves adb and the device (the sole connected one, or the pinned serial), reads the device ABI and maps it to a build arch (`arm64-v8a`→arm64, `x86_64`→amd64), then builds the **debug APK** for that arch (reusing the `gantry build` android pipeline) and installs it as **`<mobile.id>.test`**. The debug variant carries an `applicationIdSuffix` of `.test`, so the test app lives beside a real install of the app and never touches its data; a stale install from another machine is uninstalled and replaced on a signature clash. Parallelism is forced to one - each test is a full app instance and the device runs one at a time. When the suite finishes the runner uninstalls the test app again, leaving the device as it found it.

Per launch, the adb backend wipes the app's data with `pm clear` for a hermetic start (guarded - see below) or just `force-stop`s the previous instance; it hands the shell a runner-chosen fixed port, a random token and the test's environment (mode, `WithArgs`, `WithEnv`) - written into the app sandbox through `run-as` so the very first server spawn uses them, and repeated as intent extras as a fallback (release builds ignore all of it); streams logcat (the `gantry-go` tag) into the test's `app.log` and waits for the server's `GANTRY_READY` announcement; and `adb forward`s a local host port to the on-device server, dialing the websocket through it as an [observer](dom.md) since the device's own WebView is the real client.

### Hermetic device runs and the crash story

`pm clear` before each test instance is what stands in for the desktop backend's per-test temp config dir. It is destructive on someone's actual phone, so the runner only does it on an emulator or with `--allow-device-data`; without either the suite still runs, just with app data persisting between tests (a warning says so). A `Restart()` within a test never clears - the `crash.log` scenario depends on relaunching over the same data.

The crash story works on device: the debug shell suspends its usual supervisor restart for a test-configured server, so a fatal panic leaves the process dead for `WaitExit()` to observe (the driver polls `pidof libgantryapp.so` through `run-as`, so a real install's server cannot mask the test server's death), `Restart()` relaunches it over the same data, and the relaunched server reports the previous run's trace as `panic.fatal`. The driver pulls the on-device `crash.log` out through `run-as` (debug builds allow it) into the test's artifacts. Artifacts follow the [desktop layout](errors-and-artifacts.md#artifacts): `app.log` is the device-side logcat, `trace.jsonl` records the protocol frames and driver actions as always, `crash.log` is fetched through `run-as`, and a failing test gets an automatic `failure.png` captured with `screencap` - no DOM plane required.

### The DOM plane on device

`WithDOM()` works against a phone. Android WebView speaks the same devtools dialect as WebView2, so the DOM-plane tests reuse the desktop CDP client wholesale - the shell exposes the WebView's devtools socket in debug builds (`setWebContentsDebuggingEnabled`, gated on a debuggable build), the runner `adb forward`s that `localabstract:webview_devtools_remote_<pid>` socket to a host port, and attaches over CDP exactly as on desktop. So `app.Page("/settings").Find("button", Text("Save")).Click()` is the same code path on both targets.

Input is real, but the mechanism differs: `Click()` is a genuine screen tap and `Type()` genuine key input, sent through adb's native `input` rather than CDP. Android WebView's CDP synthetic input is unreliable (`dispatchTouchEvent` hangs, `dispatchMouseEvent` lands in the wrong coordinate space, `dispatchKeyEvent` never reaches the field), so the driver reads each element's viewport rect and its device-pixel ratio over CDP, translates the center point to a device-screen coordinate (accounting for the status-bar inset the shell applies), and taps it for real. `app.Supports(gantrytest.Touch)` is true on device; `Hover` is false (a phone has no hover). `--record` works too: the screencast is captured over the device WebView's devtools socket (the same CDP `Page.startScreencast` path as desktop) into `screencast.avi`, and plays in the [report](report.md) like any other. The counter test is byte-identical to its desktop twin:

```go
func TestMobileDOMClick(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	page := app.Page("/")
	page.Find("button", gantrytest.Text("count is")).Click()
	page.Find("button", gantrytest.Text("count is 1"))
	app.ExpectNoErrors()
}
```

### Running without gantry test

`gantry test --device` is the paved road: it builds and installs the debug APK and hands the driver its environment (`GANTRY_TEST_DEVICE`, `GANTRY_TEST_ADB`, `GANTRY_TEST_SERIAL`, `GANTRY_TEST_APP_ID`, `GANTRY_TEST_ACTIVITY`, `GANTRY_TEST_ALLOW_CLEAR`), then uninstalls the test app when the suite is done. A bare `GANTRY_TEST_DEVICE=android go test ./tests/...` also works against an already-installed debug APK (which stays installed - only `gantry test` cleans up): the driver finds adb on `PATH` (or under `ANDROID_HOME/platform-tools`), picks the sole connected device, and derives the test application id (`<mobile.id>.test`) from `gantry.json`.

### What does not transfer yet

**iOS** waits for its scaffold to graduate - same protocol plane over a forwarded port, with WKWebView's inspector protocol behind the same driver seam; a `gantry test --device` for a non-android target skips with a clear message today. **Widget snapshot** tests need no device at all - they are host-side and covered in [widget snapshots](widgets.md); on-device Glance rendering is screenshot-only territory, scoped to manual/nightly review.

Next: [CI](ci.md).
