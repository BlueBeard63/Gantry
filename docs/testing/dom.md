# Testing: the DOM plane

Everything on the [driving page](driving.md) works headless against the wire protocol. The DOM plane adds the other half: what the user actually sees. The driver attaches to the real webview over its devtools protocol (CDP) and drives it like a user would - element queries, real mouse clicks, real key events, screenshots, screencasts - while the protocol plane keeps watching the same app from the Go side. A single test can click a button in the DOM and assert the push it caused in Go, which is the point of the whole system.

## Enabling it

```go
app := gantrytest.Launch(t, gantrytest.WithDOM())
```

`WithDOM()` makes the launch open a real window - parked far off-screen (`GANTRY_WINDOW_OFFSCREEN=1`) so it never flashes at you or steals focus - with a per-instance remote-debugging port the driver attaches to. Headed runs (`gantry test --headed` / `WithHeaded()`) get the DOM plane automatically, window visible. A `WithDOM()` launch on a platform without a CDP-speaking webview skips cleanly (`t.Skip`), so shared suites stay runnable everywhere - see [Platforms](#platforms). Any DOM-plane call on a launch that did not get the plane fails the test with the fix ("needs the DOM plane - Launch with gantrytest.WithDOM()").

With a webview attached, the driver's protocol connection rides along as an *observer* (`?observer=1`): the webview stays the app's real client, and the driver sees every render, push, state change and error frame beside it. All the protocol-plane APIs - `WaitPush`, `State`, `SetState`, `WaitError`, `Tree`, `ExpectNoErrors` - keep working, now describing the app the webview is driving. Prefer `Page()` over `Ready()` in DOM tests: the webview announces its own mounts, so you navigate rather than mount by hand.

## Pages

```go
page := app.Page("/settings") // navigate + wait for the page container to mount
```

`Page` first waits for the frontend to actually be loaded in the webview (a cold WebView start plus the initial bundle load), then navigates client-side by pushing history state and dispatching the runtime's `gantry:navigate` event - the same path a `Link` click takes - and waits for the runtime's `.gantry-page` container to appear at that route. The returned `Page` knows its route and its key (`page.Key` - `"pages/settings"`, derived from the `gantry-pages-settings` class the runtime stamps), and scopes every `Find` to that container. If the app falls back to a different route (one it does not serve), the wait explains that rather than hanging silently.

## Finding elements

```go
name   := page.Find("input")                             // first match, auto-waiting
save   := page.Find("button", gantrytest.Text("Save"))   // narrowed by rendered text
banner := app.Find(".gantry-error-banner")               // document-wide: chrome-level elements
row    := page.Find(`[data-testid="user-row"]`)          // the convention for app-authored hooks
```

`Find` takes any CSS selector, plus `Text(substr)` filters on the element's rendered text (`innerText`, falling back to `textContent`) - the same `Text` matcher tree queries use, and the *only* matcher accepted here (a structural matcher like `Key` fails the test, since those are for tree queries). It auto-waits - up to the launch timeout - for a match to exist, so "find" doubles as "wait until it appears". Selectors are scoped: `page.Find` looks inside the page container only, `app.Find` searches the whole document for window-chrome things like the error banner. For elements without a natural selector, give them a `data-testid` attribute in your tsx and select on that - it survives styling and copy changes.

## Element actions and state

```go
save.Click()          // real mouse move + press + release at the element's center
name.Type("Jack")     // focus, then real per-character key events
name.Fill("Jacques")  // select-all + replace in one edit; Fill("") clears
name.Value()          // current input value ("" for elements without one)
save.Text()           // rendered text
save.Attr("disabled") // attribute value, "" when absent
save.Visible()        // a snapshot, no waiting - usable for absence assertions
save.Screenshot("save-button") // just this element, clipped, into the artifact dir
```

Actions re-resolve the element fresh each time, so a re-render between your `Find` and your `Click` never leaves you holding a stale handle. `Click` dispatches a real `Input.dispatchMouseEvent` move/press/release at the element's center; `Type` sends per-character `Input.dispatchKeyEvent` events so a React-controlled input sees genuine `input` events, not a synthetic `.value` poke; `Fill` uses `Input.insertText` after a select-all (and `Backspace` for the empty case). `Visible()` is the one action that does *not* wait - it returns a snapshot, which is what lets you assert an element is *absent*.

## Both planes in one test

```go
func TestSettingsSave(t *testing.T) {
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	page := app.Page("/settings")

	page.Find("input").Type("Jack")               // DOM: the user types
	page.Find("button", gantrytest.Text("Save")).Click()

	app.WaitPush("pages/settings", "saved")       // protocol: Go confirmed
	app.SetState("volume", 0.25)                  // protocol: Go-side write...
	page.Find("label", gantrytest.Text("25%"))    // ...DOM: the UI re-rendered live

	app.ExpectNoErrors()
}
```

Clicking a real range slider drags its value: `page.Find("input[type=range]").Click()` lands the mouse at the track's center and the value round-trips into shared state, so `app.State("volume").Float()` reads ~0.5 afterward.

## Screenshots and screencasts

```go
app.Screenshot("after-save") // whole window -> test-results/<TestName>/after-save.png
```

On any failure, a DOM-plane test automatically captures `failure.png` - what was on screen when it died - before tearing the app down. The webview's console output and uncaught JS exceptions stream into `console.log` alongside it (a `console.error` or an uncaught `throw` in your frontend lands there). `gantry test --record` (or `WithRecording()` per launch) records the whole test into `screencast.avi`, MJPEG-in-AVI so it plays everywhere with no encoder. See [errors and artifacts](errors-and-artifacts.md#artifacts) for the full artifact list.

## Notes

### Auto-waiting and real input

Actions auto-wait through an `actionable` gate: before clicking or typing, the driver waits for the element to be attached, visible, scrolled into view (`scrollIntoView({block:"center"})`) and *position-stable* - the same bounding rect across two consecutive resolves - so async content shifting the layout mid-test does not swallow a click at the old position. Every wait honors the launch timeout and fails with what it was stuck on ("first not visible", "waiting for a stable position") plus the last protocol frames.

### Screencast pacing

Frames come from the browser's screencast on every visual change, plus one forced frame after each driver action (an off-screen window's compositor can idle and starve the stream, so each action pins its resulting state). Byte-identical neighboring frames are merged, then the timeline is paced for humans rather than real time - a test can blow through its whole UI in under a second, so each distinct state stays on screen at least half a second and idle stretches are compressed to at most two, keeping the order exact and the result watchable. Recorded runs keep their artifacts even on pass (a screencast that vanishes would be pointless).

### Platforms

The element API is identical on every target; only the wire dialect underneath differs.

- **Windows** - WebView2, which is Chromium and speaks CDP. This is the validated desktop path. WebView2 reads the debug port from `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`.
- **Linux** - WebKitGTK, driven over its own remote inspector protocol (a second dialect behind the same driver seam, enabled with `WEBKIT_INSPECTOR_SERVER`; run it under `xvfb` on a headless box). This path is **experimental and unvalidated**: WebKitGTK's inspector discovery and domain method names are version-dependent and not formally documented, and input there is synthesized as DOM events via `Runtime.evaluate` rather than through a native input domain. Confirm it against your WebKit version before relying on it in CI.
- **macOS** - a desktop `WithDOM()` launch skips (no CDP webview), so shared suites still run; the protocol plane is unaffected.
- **Android device** - the phone's WebView speaks the same CDP dialect as WebView2, so `WithDOM()` works under `gantry test --device android`, with clicks and typing mapped to real touch/key input. See [mobile](mobile.md#the-dom-plane-on-device).

### Capabilities

```go
if app.Supports(gantrytest.DOM) { ... }   // true whenever the CDP plane is attached
if app.Supports(gantrytest.Hover) { ... } // desktop DOM only; a phone reports false
```

Shared suites gate on capabilities instead of forking per target - a protocol-only launch reports false for both, a desktop DOM launch reports `DOM` and `Hover`, and a device launch reports `DOM`, `Touch` and `Notifications` (and not `Hover`, since a phone has no hover) without changing test code.

Next: [errors and artifacts](errors-and-artifacts.md).
