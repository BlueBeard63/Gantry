# Testing: mobile

> **Status: the protocol plane on device is shipped** (tier M1 of the testing plan). `gantry test --device android` runs the same test files against a plugged-in phone or an emulator - the whole protocol plane (`Ready`, `Call`, `WaitError`, tree queries, state, the crash story) works unchanged. Element-level driving over the device's webview (the DOM plane) is the next mobile tier; see the phasing note at the bottom.

Mobile is a first-class target, not an afterthought, because of a fact about Gantry's architecture: the APK is not a different app. It is the same Go server (packed as a shared library, launched by the Kotlin shell) serving the same frontend to an Android WebView. So the test driver's planes transfer rather than being rebuilt - the same test files run on desktop and device wherever the surfaces overlap.

## Running on a device

```
gantry test --device android          # sole connected device or emulator
gantry test --device android:SERIAL   # pin one
```

The runner builds the **debug APK** for the device's ABI (reusing the `gantry build` android pipeline), installs it with `adb install -r`, and runs the suite through an adb-backed backend. Parallelism drops to one - each test is a full app instance and the device runs one at a time.

Per launch, the backend:

- wipes the app's data with `pm clear` for a hermetic start (guarded - see below), or just `force-stop`s the previous instance;
- hands the debug shell a runner-chosen port, token and the test's environment (mode, `WithArgs`, `WithEnv`) - written into the app sandbox through `run-as` so the very first server spawn uses them, and repeated as intent extras as a fallback - so the server binds a port the runner knows and authenticates with a token the runner chose (release builds ignore all of it);
- streams logcat (the `gantry-go` tag) into the test's `app.log` and waits for the server's ready announcement;
- `adb forward`s a local port to the on-device server and dials the websocket through it, as an [observer](dom.md) - the device's own WebView is the real client, and the driver rides alongside it.

Everything above the backend is the same driver: a test that mounts pages, fires events and asserts on calls/state/errors runs identically on both targets. `DesktopOnly(t)` / `MobileOnly(t)` fence the target-specific ones, and `app.Supports(...)` gates capability differences inside shared tests.

```go
func TestSaveNotifies(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t) // adb backend when --device android is set
	app.Ready("pages/settings")
	// ... same driver API as desktop ...
}
```

## Hermetic device runs and the crash story

`pm clear` before each test instance is what stands in for the desktop backend's per-test temp config dir. It is destructive on someone's actual phone, so the runner only does it on an emulator or with `--allow-device-data`; without either the suite still runs, just with app data persisting between tests. A `Restart()` within a test never clears - the crash.log scenario depends on relaunching over the same data.

The crash story works on device: the debug shell suspends its usual supervisor restart for a test-configured server, so a fatal panic leaves the process dead for `WaitExit()` to observe, `Restart()` relaunches it over the same data, and the relaunched server reports the previous run's trace as `panic.fatal`. The driver pulls the on-device `crash.log` out through `run-as` (debug builds allow it) into the test's artifacts.

Artifacts follow the [desktop layout](errors-and-artifacts.md): `app.log` is the device-side logcat, `trace.jsonl` records the protocol frames and driver actions as always, `crash.log` is fetched through `run-as`, and a failing test gets an automatic `failure.png` captured with `screencap` - no DOM plane required.

## What does not transfer yet

- **The DOM plane** (tier M2): `WithDOM()` skips on a device target. Android WebView speaks the same devtools dialect as WebView2, so DOM-plane tests will reuse the desktop CDP client wholesale, with `Click()` mapping to real touch events.
- **Mobile-specific helpers** (tier M2): `app.Back()`, `app.Rotate()`, `app.Notifications()`, permission pre-grants.
- **Screencasts on device** (tier 3): `--record` is a no-op against a device; `screenrecord` lands with the recording polish tier.
- **iOS**: waits for its scaffold to graduate - same protocol plane over a forwarded port, with WKWebView's inspector protocol behind the same driver seam.

## Running without gantry test

`gantry test --device` is the paved road: it builds and installs the debug APK and hands the driver its environment (`GANTRY_TEST_DEVICE`, `GANTRY_TEST_ADB`, `GANTRY_TEST_SERIAL`, `GANTRY_TEST_APP_ID`, `GANTRY_TEST_ALLOW_CLEAR`). A bare `GANTRY_TEST_DEVICE=android go test ./tests/...` also works against an already-installed debug APK: the driver finds adb on PATH (or under `ANDROID_HOME`), picks the sole connected device, and reads the application id from gantry.json's `mobile.id`.

## Widgets

Widget snapshot tests need no device at all - they are host-side and covered in [widget snapshots](widgets.md). On-device Glance rendering is screenshot-only territory, scoped to manual/nightly review.

## Phasing

The build order (from the framework's testing plan): tier 1 protocol driver on desktop (shipped), tier M1 the adb backend running the tier-1 suite on device (shipped), tier 2 the DOM plane on Windows (shipped), tier M2 the DOM plane plus mobile-specific helpers on Android, tier 3 recordings and polish. iOS follows once its scaffold graduates.
