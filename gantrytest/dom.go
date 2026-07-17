package gantrytest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// The DOM plane (tier 2 of the testing plan): element-level driving of
// the real webview over CDP. Launch with WithDOM() (or run headed on
// Windows) and the app opens a real window - off-screen unless headed -
// with a per-instance devtools port the driver attaches to. Everything
// here is "what the user actually sees": querySelector visibility and
// text, real mouse clicks and key events, screenshots.

// Page is one mounted app page in the webview: a navigation target plus
// the scope (the runtime's .gantry-page container) Find queries within.
type Page struct {
	app *App
	// Route is the path this page was opened at ("/settings").
	Route string
	// Key is the page key the container was stamped with
	// ("pages/settings").
	Key string

	sel string // ".gantry-page.gantry-pages-settings"
}

// Element is one auto-waiting element query: a selector (plus Text
// filters) scoped to a page container or the document. Actions
// re-resolve it fresh each time, so re-renders never leave a test
// holding a stale handle.
type Element struct {
	app   *App
	scope string   // container selector; "" = document
	sel   string   // CSS selector
	texts []string // rendered-text substring filters
	desc  string   // for failure messages
}

// requireDOM returns the CDP connection or fails the test with the fix.
func (a *App) requireDOM(what string) *cdp {
	a.t.Helper()
	if a.cdp == nil {
		a.t.Fatalf("gantrytest: %s needs the DOM plane - Launch with gantrytest.WithDOM() (or run headed on Windows)", what)
	}
	return a.cdp
}

// Page navigates the webview to the route (client-side, like a Link
// click) and waits until the page's container is mounted. The returned
// Page scopes Find to that container.
func (a *App) Page(route string) *Page {
	a.t.Helper()
	d := a.requireDOM("Page")
	clean := route
	if clean != "/" {
		clean = strings.TrimSuffix(clean, "/")
	}
	a.art.trace.action("page %s", clean)

	// The webview may still be booting: wait for the app document
	// before navigating into it.
	host := fmt.Sprintf("127.0.0.1:%d", a.proc.webPort())
	a.waitDOM("the app frontend to load", func() (bool, string) {
		raw, err := d.eval(fmt.Sprintf(`(() => location.host === %q && !!document.getElementById("root"))()`, host))
		if err != nil {
			return false, err.Error()
		}
		return string(raw) == "true", "frontend not mounted yet"
	})

	if _, err := d.eval(fmt.Sprintf(
		`(() => { history.pushState(null, "", %q); window.dispatchEvent(new Event("gantry:navigate")); })()`, clean)); err != nil {
		a.t.Fatalf("gantrytest: navigating to %s: %v", clean, err)
	}

	p := &Page{app: a, Route: clean}
	a.waitDOM(fmt.Sprintf("the page at %s to mount", clean), func() (bool, string) {
		raw, err := d.eval(`(() => {
			const el = document.querySelector(".gantry-page");
			return el ? {path: location.pathname, cls: el.className} : null;
		})()`)
		if err != nil {
			return false, err.Error()
		}
		var got struct {
			Path string `json:"path"`
			Cls  string `json:"cls"`
		}
		if string(raw) == "null" || json.Unmarshal(raw, &got) != nil {
			return false, "no .gantry-page container yet"
		}
		path := got.Path
		if path != "/" {
			path = strings.TrimSuffix(path, "/")
		}
		if path != clean {
			return false, fmt.Sprintf("the app is showing %s (a route the app does not serve falls back)", got.Path)
		}
		for _, cls := range strings.Fields(got.Cls) {
			if cls != "gantry-page" && strings.HasPrefix(cls, "gantry-") {
				p.sel = ".gantry-page." + cls
				p.Key = strings.ReplaceAll(strings.TrimPrefix(cls, "gantry-"), "-", "/")
			}
		}
		return p.sel != "", "container has no page class yet"
	})
	a.snapRecording()
	return p
}

// Find waits for the first element matching the CSS selector (and any
// Text filters) inside this page's container.
func (p *Page) Find(selector string, opts ...Match) *Element {
	p.app.t.Helper()
	return p.app.find(p.sel, selector, opts)
}

// Find waits for the first element matching the CSS selector anywhere
// in the document - chrome-level elements like the error banner; use
// Page.Find for page content.
func (a *App) Find(selector string, opts ...Match) *Element {
	a.t.Helper()
	return a.find("", selector, opts)
}

func (a *App) find(scope, selector string, opts []Match) *Element {
	a.t.Helper()
	a.requireDOM("Find")
	e := &Element{app: a, scope: scope, sel: selector}
	for _, m := range opts {
		tm, ok := m.(textMatch)
		if !ok {
			a.t.Fatalf("gantrytest: only Text() narrows a DOM Find (structural matchers are for tree queries)")
		}
		e.texts = append(e.texts, string(tm))
	}
	e.desc = selector
	if scope != "" {
		e.desc = scope + " " + selector
	}
	if len(e.texts) > 0 {
		e.desc += fmt.Sprintf(" with text %q", strings.Join(e.texts, "\", \""))
	}
	// Auto-wait for attachment, so Find doubles as "wait until it
	// appears" and actions never race the mount.
	a.waitDOM(e.desc, func() (bool, string) {
		info, err := e.resolve(false, false, false)
		if err != nil {
			return false, err.Error()
		}
		return info.State == "ok", info.explain()
	})
	return e
}

// elemInfo is one resolution of an Element in the page.
type elemInfo struct {
	State   string  `json:"state"`
	Count   int     `json:"count"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	Visible bool    `json:"visible"`
	Text    string  `json:"text"`
	Value   *string `json:"value"`
	Dpr     float64 `json:"dpr"` // device-pixel ratio (device input coord translation)
}

func (i elemInfo) explain() string {
	switch i.State {
	case "noscope":
		return "the page container is gone"
	case "none":
		return "no matching element"
	case "ok":
		if !i.Visible {
			return fmt.Sprintf("%d match(es), first not visible", i.Count)
		}
		return fmt.Sprintf("%d match(es)", i.Count)
	}
	return i.State
}

// resolve runs the element query in the page, optionally scrolling the
// element into view, focusing it (caret at end), or selecting its
// content.
func (e *Element) resolve(scroll, focus, selectAll bool) (elemInfo, error) {
	scopeExpr := "document"
	if e.scope != "" {
		scopeExpr = fmt.Sprintf("document.querySelector(%q)", e.scope)
	}
	// Never nil: a nil slice marshals to JSON null, which for-of chokes on.
	texts, _ := json.Marshal(append([]string{}, e.texts...))
	expr := fmt.Sprintf(`(() => {
		const scope = %s;
		if (!scope) return {state: "noscope"};
		let els = Array.from(scope.querySelectorAll(%q));
		for (const t of %s) {
			els = els.filter(el => ((el.innerText !== undefined ? el.innerText : el.textContent) || "").includes(t));
		}
		if (!els.length) return {state: "none"};
		const el = els[0];
		if (%t) el.scrollIntoView({block: "center", inline: "center"});
		if (%t) {
			el.focus();
			if (el.setSelectionRange && typeof el.value === "string") {
				try { el.setSelectionRange(el.value.length, el.value.length); } catch {}
			}
		}
		if (%t && el.select) el.select();
		const r = el.getBoundingClientRect();
		const cs = getComputedStyle(el);
		return {
			state: "ok", count: els.length,
			x: r.x, y: r.y, w: r.width, h: r.height,
			visible: !!(el.getClientRects().length && cs.visibility !== "hidden" && cs.display !== "none"),
			text: (el.innerText !== undefined ? el.innerText : el.textContent) || "",
			value: ("value" in el) ? String(el.value) : null,
			dpr: window.devicePixelRatio,
		};
	})()`, scopeExpr, e.sel, texts, scroll, focus, selectAll)

	raw, err := e.app.cdp.eval(expr)
	if err != nil {
		return elemInfo{}, err
	}
	var info elemInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return elemInfo{}, fmt.Errorf("decoding element query result %s: %w", raw, err)
	}
	return info, nil
}

// actionable waits until the element is attached, visible, scrolled
// into view AND position-stable (the same rect across two resolves a
// beat apart), then returns its geometry. The stability check matters:
// async content above the target can land between resolving and
// clicking, shifting the element so the click hits its old position.
func (e *Element) actionable(verb string, focus, selectAll bool) elemInfo {
	e.app.t.Helper()
	var info elemInfo
	havePrev := false
	var prev elemInfo
	e.app.waitDOM(fmt.Sprintf("%s %s", verb, e.desc), func() (bool, string) {
		i, err := e.resolve(true, focus, selectAll)
		if err != nil {
			return false, err.Error()
		}
		info = i
		if i.State != "ok" || !i.Visible || i.W <= 0 || i.H <= 0 {
			havePrev = false
			return false, i.explain()
		}
		stable := havePrev && prev.X == i.X && prev.Y == i.Y && prev.W == i.W && prev.H == i.H
		prev, havePrev = i, true
		if !stable {
			return false, "waiting for a stable position"
		}
		return true, ""
	})
	return info
}

// Click activates the element's center: a CDP mouse click on desktop, a
// real screen tap on a device. Android WebView's CDP synthetic input is
// unreliable (dispatchTouchEvent hangs, dispatchMouseEvent lands in the
// wrong coordinate space, dispatchKeyEvent never reaches the input), so
// device actions go through adb's native input, with element coordinates
// translated from the viewport (CSS px) to the screen (device px).
func (e *Element) Click() {
	e.app.t.Helper()
	info := e.actionable("click", false, false)
	e.app.art.trace.action("click %s", e.desc)
	if e.app.onDevice {
		e.app.deviceTap(info)
	} else if err := e.app.cdp.mouseClick(info.X+info.W/2, info.Y+info.H/2); err != nil {
		e.app.t.Fatalf("gantrytest: clicking %s: %v", e.desc, err)
	}
	e.app.snapRecording()
}

// Type focuses the element and types the text: real per-character key
// events on desktop; on a device it taps the field (to establish the
// input connection) then sends the text with adb input.
func (e *Element) Type(text string) {
	e.app.t.Helper()
	info := e.actionable("type into", true, false)
	e.app.art.trace.action("type %q into %s", text, e.desc)
	if e.app.onDevice {
		e.app.deviceTap(info)
		if err := e.app.android().inputText(text); err != nil {
			e.app.t.Fatalf("gantrytest: typing into %s: %v", e.desc, err)
		}
	} else if err := e.app.cdp.typeText(text); err != nil {
		e.app.t.Fatalf("gantrytest: typing into %s: %v", e.desc, err)
	}
	e.app.snapRecording()
}

// Fill replaces the element's content: on desktop, select-all then commit
// in one edit; on a device, tap to focus, clear the existing value with
// backspaces, then type the new text (empty text just clears).
func (e *Element) Fill(text string) {
	e.app.t.Helper()
	info := e.actionable("fill", true, true)
	e.app.art.trace.action("fill %s with %q", e.desc, text)
	if e.app.onDevice {
		p := e.app.android()
		e.app.deviceTap(info)
		_ = p.keyEvent("KEYCODE_MOVE_END")
		existing := ""
		if info.Value != nil {
			existing = *info.Value
		}
		for range existing {
			_ = p.keyEvent("KEYCODE_DEL")
		}
		if text != "" {
			if err := p.inputText(text); err != nil {
				e.app.t.Fatalf("gantrytest: filling %s: %v", e.desc, err)
			}
		}
		e.app.snapRecording()
		return
	}
	var err error
	if text == "" {
		err = e.app.cdp.pressKey("Backspace", 8)
	} else {
		err = e.app.cdp.insertText(text)
	}
	if err != nil {
		e.app.t.Fatalf("gantrytest: filling %s: %v", e.desc, err)
	}
	e.app.snapRecording()
}

// deviceTap translates an element's viewport point (CSS px) into a
// device-screen point and taps it there. The WebView sits below the
// status bar (the shell insets it) and fills the width, so a CSS point
// maps to the screen as (inset.left + cssX*dpr, inset.top + cssY*dpr) -
// dpr converts CSS px to device px directly. Android WebView reports 0
// for innerWidth/clientWidth, so dpr (reliable) plus the system-bar
// insets are used rather than a viewport fraction.
func (a *App) deviceTap(info elemInfo) {
	a.t.Helper()
	p := a.android()
	dpr := info.Dpr
	if dpr <= 0 {
		a.t.Fatalf("gantrytest: could not read the device-pixel ratio for a tap")
	}
	left, top, err := p.contentInset()
	if err != nil {
		a.t.Fatalf("gantrytest: reading the device insets for a tap: %v", err)
	}
	cssX, cssY := info.X+info.W/2, info.Y+info.H/2
	dx := left + int(cssX*dpr)
	dy := top + int(cssY*dpr)
	if err := p.tap(dx, dy); err != nil {
		a.t.Fatalf("gantrytest: tapping at %d,%d: %v", dx, dy, err)
	}
}

// Text waits for the element and returns its rendered text.
func (e *Element) Text() string {
	e.app.t.Helper()
	return e.await("read text of").Text
}

// Value waits for the element and returns its current input value
// (empty for elements without one).
func (e *Element) Value() string {
	e.app.t.Helper()
	if v := e.await("read value of").Value; v != nil {
		return *v
	}
	return ""
}

// Attr waits for the element and returns the attribute's value ("" when
// absent).
func (e *Element) Attr(name string) string {
	e.app.t.Helper()
	var out string
	e.app.waitDOM(fmt.Sprintf("read attribute %q of %s", name, e.desc), func() (bool, string) {
		info, err := e.resolve(false, false, false)
		if err != nil {
			return false, err.Error()
		}
		if info.State != "ok" {
			return false, info.explain()
		}
		scopeExpr := "document"
		if e.scope != "" {
			scopeExpr = fmt.Sprintf("document.querySelector(%q)", e.scope)
		}
		raw, err := e.app.cdp.eval(fmt.Sprintf(
			`(() => { const s = %s; const el = s && s.querySelector(%q); return el ? el.getAttribute(%q) : null; })()`,
			scopeExpr, e.sel, name))
		if err != nil {
			return false, err.Error()
		}
		if string(raw) != "null" {
			_ = json.Unmarshal(raw, &out)
		}
		return true, ""
	})
	return out
}

// Visible reports whether the element currently exists and is visible -
// a snapshot, no waiting, so tests can assert absence too.
func (e *Element) Visible() bool {
	e.app.t.Helper()
	info, err := e.resolve(false, false, false)
	if err != nil {
		e.app.t.Fatalf("gantrytest: checking visibility of %s: %v", e.desc, err)
	}
	return info.State == "ok" && info.Visible
}

// await waits for attachment and returns the resolution.
func (e *Element) await(verb string) elemInfo {
	e.app.t.Helper()
	var info elemInfo
	e.app.waitDOM(fmt.Sprintf("%s %s", verb, e.desc), func() (bool, string) {
		i, err := e.resolve(false, false, false)
		if err != nil {
			return false, err.Error()
		}
		info = i
		return i.State == "ok", i.explain()
	})
	return info
}

// Screenshot captures just this element into the artifact directory as
// <name>.png.
func (e *Element) Screenshot(name string) {
	e.app.t.Helper()
	info := e.actionable("screenshot", false, false)
	data, err := e.app.cdp.screenshot(map[string]any{
		"x": info.X, "y": info.Y, "width": info.W, "height": info.H, "scale": 1,
	})
	if err != nil {
		e.app.t.Fatalf("gantrytest: capturing %s: %v", e.desc, err)
	}
	e.app.art.savePNG(e.app.t, name, data)
}

// Screenshot captures the whole page into the artifact directory as
// <name>.png - the explicit-artifact call; failures also capture
// failure.png automatically.
func (a *App) Screenshot(name string) {
	a.t.Helper()
	var data []byte
	var err error
	if a.cdp != nil {
		data, err = a.cdp.screenshot(nil)
	} else if data, err = a.proc.screenshot(); err != nil {
		// Device targets capture through adb; a headless desktop
		// launch has no screen to shoot - point at the DOM plane.
		a.requireDOM("Screenshot")
	}
	if err != nil {
		a.t.Fatalf("gantrytest: capturing a screenshot: %v", err)
	}
	a.art.trace.action("screenshot %s", name)
	a.art.savePNG(a.t, name, data)
}

// snapRecording pins one forced frame into the recording after a
// driver action: an off-screen window's compositor can go idle and
// starve the screencast, so each action captures its resulting state
// explicitly. No-op when not recording.
func (a *App) snapRecording() {
	if a.rec == nil || a.cdp == nil {
		return
	}
	if data, err := a.cdp.jpegFrame(); err == nil {
		a.rec.add(data, float64(time.Now().UnixNano())/1e9)
	}
}

// waitDOM polls cond until it reports done, failing with the last
// explanation and the recent protocol frames on timeout.
func (a *App) waitDOM(what string, cond func() (bool, string)) {
	a.t.Helper()
	deadline := a.client.deadline()
	explain := ""
	for {
		done, why := cond()
		if done {
			return
		}
		explain = why
		if !time.Now().Before(deadline) {
			a.client.mu.Lock()
			dump := a.client.lastFramesLocked(20)
			a.client.mu.Unlock()
			a.t.Fatalf("gantrytest: timed out waiting for %s (%s)\nlast protocol frames:\n%s", what, explain, dump)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
