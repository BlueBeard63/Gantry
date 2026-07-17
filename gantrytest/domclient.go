package gantrytest

import "encoding/json"

// domClient is the DOM-plane wire client, abstracting the browser
// dialect behind one interface so the element-action code (dom.go) and
// teardown are identical on every target. Two implementations satisfy it:
//
//   - cdp (cdp.go) speaks the Chrome DevTools Protocol - WebView2 on
//     Windows and Android WebView on device, both Chromium.
//   - webkit (webkit.go) speaks the WebKit remote inspector protocol -
//     WebKitGTK, the Linux desktop webview, which is not Chromium and
//     does not speak CDP.
//
// attachDOM picks the implementation by target. This is the "thin
// protocol-adapter seam" the DOM driver was built against from tier 2, so
// a third dialect (WKWebView's Safari Web Inspector, for iOS) slots in
// the same way later.
type domClient interface {
	// eval runs a JS expression in the page and returns its JSON value.
	eval(expr string) (json.RawMessage, error)
	// screenshot captures the page (whole page, or the clip rect when
	// given) as PNG bytes.
	screenshot(clip map[string]any) ([]byte, error)
	// mouseClick dispatches a real click at viewport coordinates.
	mouseClick(x, y float64) error
	// typeText sends per-character key events to the focused element.
	typeText(text string) error
	// insertText replaces the focused element's value in one edit.
	insertText(text string) error
	// pressKey presses a single named key (with its virtual keycode).
	pressKey(key string, vk int) error
	// startScreencast begins delivering JPEG frames to onFrame on every
	// visual change, for --record.
	startScreencast(onFrame func([]byte, float64)) error
	// jpegFrame grabs one JPEG still on demand (the forced frame after a
	// driver action, so an idle compositor does not starve the video).
	jpegFrame() ([]byte, error)
	// stopScreencast ends screencast delivery.
	stopScreencast()
	// close tears down the connection.
	close()
}

// compile-time assurance that cdp satisfies the seam.
var _ domClient = (*cdp)(nil)
