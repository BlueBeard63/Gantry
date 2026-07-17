# Testing: mobile

> **Status: designed, not yet shipped.** Tier 1 (the protocol driver and `gantry test` on desktop) is what exists today. The device backend described here is the next tier; this page documents where the system is going so suites can be written to transfer. See the phasing note at the bottom.

Mobile is a first-class target, not an afterthought, because of a fact about Gantry's architecture: the APK is not a different app. It is the same Go server (packed as a shared library, launched by the Kotlin shell with `--port 0 --no-open --announce-ready`) serving the same frontend to an Android WebView. So the test driver's planes transfer rather than being rebuilt - the same test files run on desktop and device wherever the surfaces overlap.

## What it will look like

```
gantry test --device android          # sole connected device or emulator
gantry test --device android:SERIAL   # pin one
```

The runner builds the APK first (reusing the `gantry build` android pipeline), installs it, and runs the suite against the phone through an adb-backed backend: `adb forward` makes the on-device server reachable from the host, and the whole protocol plane - `Ready`, `Call`, `WaitError`, tree queries, error assertions - works against the phone unchanged. Parallelism drops to one per device.

Tests declare what they need and skip cleanly when the target cannot provide it - these helpers exist today so shared suites are written correctly from the start:

```go
func TestSaveNotifies(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t) // adb backend when --device android is set
	app.Ready("pages/settings")
	// ... same driver API as desktop ...
}
```

The default posture is shared: a test that mounts pages, fires events and asserts on calls/state/errors runs identically on both targets. `DesktopOnly(t)` / `MobileOnly(t)` fence the target-specific ones, and `app.Supports(...)` gates capability differences (hover exists on desktop, touch on device) inside shared tests.

## Hermetic device runs

Each device suite gets: `pm clear` for a clean app state (guarded - the runner refuses to wipe data on a physical device unless explicitly allowed), device screenshots and screen recordings into the same `test-results/` layout, logcat collected as the device-side `app.log`, and `force-stop` teardown.

## Mobile-specific surfaces (the tier after)

- **Native input and lifecycle**: `app.Back()`, `app.Home()` + relaunch, `app.Rotate()` (the webview must survive a configuration change without dropping the socket), and process-death recovery on device.
- **Notifications**: `app.Notifications()` asserting on posted system notifications and their actions.
- **Permissions**: pre-granting or revoking permissions before launch to assert both flows.
- **Element-level driving on device**: Android WebView speaks the same devtools dialect as WebView2, so DOM-plane tests reuse the desktop CDP client wholesale, with `Click()` mapping to real touch events.

## Widgets

Widget snapshot tests need no device at all - they are host-side and covered in [widget snapshots](widgets.md). On-device Glance rendering is screenshot-only territory, scoped to manual/nightly review.

## Phasing

The build order (from the framework's testing plan): tier 1 protocol driver on desktop (shipped), tier M1 the adb backend running the tier-1 suite on device, tier 2 the DOM plane on Windows, tier M2 the DOM plane plus mobile-specific helpers on Android, tier 3 recordings and polish. iOS follows once its scaffold graduates - same protocol plane over a forwarded port, with WKWebView's inspector protocol behind the same driver seam.
