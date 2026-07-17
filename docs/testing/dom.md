# Testing: the DOM plane

Everything on the [driving page](driving.md) works headless against the wire protocol. The DOM plane adds the other half: what the user actually sees. The driver attaches to the real webview over its devtools protocol (CDP) and drives it like a user would - element queries, real mouse clicks, real key events, screenshots, screencasts - while the protocol plane keeps watching the same app from the Go side. A single test can click a button in the DOM and assert the push it caused in Go, which is the point of the whole system.

The DOM plane is Windows-only for now (WebView2 is Chromium and speaks CDP; Linux's WebKitGTK does not). A `WithDOM()` launch on another OS skips the test, so shared suites stay runnable everywhere. The same CDP client is the one the Android tier reuses - Android WebView speaks the same dialect.

## Enabling it

```go
app := gantrytest.Launch(t, gantrytest.WithDOM())
```

`WithDOM()` makes the launch open a real window - parked far off-screen so it never flashes at you or steals focus - with a per-instance remote-debugging port the driver attaches to. Headed runs (`gantry test --headed` / `WithHeaded()`) get the DOM plane automatically, window visible.

With a webview attached, the driver's protocol connection rides along as an *observer*: the webview stays the app's real client, and the driver sees every render, push, state change and error frame beside it. All the protocol-plane APIs - `WaitPush`, `State`, `SetState`, `WaitError`, `Tree`, `ExpectNoErrors` - keep working, now describing the app the webview is driving. (Prefer `Page()` over `Ready()` in DOM tests: the webview announces its own mounts.)

## Pages

```go
page := app.Page("/settings") // navigate + wait for the page container to mount
```

`Page` navigates client-side (the same history-API path a `Link` click takes) and waits for the runtime's `.gantry-page` container to appear at that route. The returned `Page` knows its key (`page.Key` - "pages/settings") and scopes every `Find` to the page's container, via the `gantry-pages-settings` class the runtime already stamps.

## Finding elements

```go
name := page.Find("input")                                // first match, auto-waiting
save := page.Find("button", gantrytest.Text("Save"))      // narrowed by rendered text
banner := app.Find(".gantry-error-banner")                // document-wide: chrome-level elements
row := page.Find(`[data-testid="user-row"]`)              // the convention for app-authored hooks
```

`Find` takes any CSS selector, plus `Text(substr)` filters on the element's rendered text (the same matcher tree queries use). It waits - up to the launch timeout - for a match to exist, so "find" doubles as "wait until it appears". Selectors are scoped: `page.Find` looks inside the page container only, `app.Find` searches the whole document for window-chrome things like error banners.

For elements without a natural selector, give them a `data-testid` attribute in your tsx and select on that - it survives styling and copy changes.

## Element actions and state

```go
save.Click()          // real mouse move + press + release at the element's center
name.Type("Jack")     // focus, then real per-character key events
name.Fill("Jacques")  // select-all + replace in one edit; Fill("") clears
name.Value()          // current input value
save.Text()           // rendered text
save.Attr("disabled") // attribute value, "" when absent
save.Visible()        // a snapshot, no waiting - usable for absence assertions
save.Screenshot("save-button") // just this element, into the artifact dir
```

Actions auto-wait: before clicking or typing, the driver waits for the element to be attached, visible, scrolled into view and *position-stable* (the same rectangle across two consecutive checks), so async content shifting the layout mid-test does not swallow clicks. Every wait honors the launch timeout and fails with what it was stuck on plus the last protocol frames.

Input is real: `Click` dispatches mouse events through the browser's input pipeline (bubbling, focus, `:active`, everything), and `Type` sends per-character key events - a React-controlled input sees genuine `input` events, not synthetic `.value` pokes. Clicking a range slider at its center really drags the value to the middle.

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

## Screenshots and screencasts

```go
app.Screenshot("after-save") // whole window -> test-results/<TestName>/after-save.png
```

On any failure, a DOM-plane test automatically captures `failure.png` - what was on screen when it died - before tearing the app down. The webview's console output and uncaught JS exceptions stream into `console.log` alongside it.

`gantry test --record` (or `WithRecording()` per launch) records the whole test into `screencast.avi` (MJPEG - plays everywhere, no encoder needed). Frames come from the browser's screencast on every visual change, plus one forced frame after each driver action, so the video tracks exactly what the test did; recorded runs keep their artifacts even on pass. The video is paced for humans rather than real time - a test can blow through its whole UI in under a second, so each distinct state stays on screen for at least half a second and idle stretches are compressed, keeping the order exact and the result actually watchable. See [errors and artifacts](errors-and-artifacts.md) for the full artifact list.

## Capabilities

```go
if app.Supports(gantrytest.DOM) { ... }   // true on a WithDOM/headed launch
if app.Supports(gantrytest.Hover) { ... } // mouse targets only; mobile reports false
```

Shared suites gate on capabilities instead of forking per target - a protocol-only launch reports false for both, and the device tiers will flip `Touch` and `Notifications` on without changing test code.
