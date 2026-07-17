package gantrytest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// cdp is the DOM-plane connection: a minimal Chrome DevTools Protocol
// client attached to the app's webview (WebView2 on desktop, Android
// WebView on device - both speak the same dialect). The runner points
// the browser at a per-instance --remote-debugging-port and this client
// does everything above it: element queries and JS evaluation via
// Runtime, real input via Input.dispatch*, screenshots via Page, and
// console/exception capture into the console.log artifact.
type cdp struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	tr     *trace

	mu      sync.Mutex
	nextID  int
	pending map[int]chan cdpReply
	console *os.File
	onFrame func(data []byte, t float64) // screencast frames (recorder)
	closed  bool
	readErr error
}

type cdpReply struct {
	result json.RawMessage
	err    error
}

// cdpTarget is one entry of the browser's /json/list.
type cdpTarget struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	WSURL  string `json:"webSocketDebuggerUrl"`
	Target string `json:"id"`
}

// attachCDP polls the browser's devtools endpoint until the app's page
// target exists (the webview needs a moment to start and navigate),
// dials its debugger websocket, and enables the domains the driver
// uses. consolePath receives every console message and uncaught JS
// exception for the test's artifacts.
func attachCDP(tr *trace, debugPort, appPort int, consolePath string, timeout time.Duration) (*cdp, error) {
	deadline := time.Now().Add(timeout)
	listURL := fmt.Sprintf("http://127.0.0.1:%d/json/list", debugPort)
	appHost := fmt.Sprintf("127.0.0.1:%d", appPort)

	var target *cdpTarget
	for target == nil {
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("timed out after %s waiting for the webview's devtools endpoint on port %d (is the WebView2 runtime installed, and did the window open?)", timeout, debugPort)
		}
		if t, err := findTarget(listURL, appHost); err == nil {
			target = t
		} else {
			time.Sleep(150 * time.Millisecond)
		}
	}
	tr.action("cdp: attaching to %s (%s)", target.URL, target.WSURL)

	ctx, cancel := context.WithCancel(context.Background())
	dialCtx, dialDone := context.WithTimeout(ctx, 15*time.Second)
	defer dialDone()
	conn, _, err := websocket.Dial(dialCtx, target.WSURL, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("dialing the devtools websocket %s: %w", target.WSURL, err)
	}
	conn.SetReadLimit(64 << 20) // screenshots come back base64 in one frame

	console, err := os.Create(consolePath)
	if err != nil {
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, err
	}

	d := &cdp{conn: conn, ctx: ctx, cancel: cancel, tr: tr,
		pending: map[int]chan cdpReply{}, console: console}
	go d.readLoop()

	for _, method := range []string{"Runtime.enable", "Page.enable"} {
		if _, err := d.call(method, nil, 10*time.Second); err != nil {
			d.close()
			return nil, fmt.Errorf("cdp %s: %w", method, err)
		}
	}
	return d, nil
}

// findTarget picks the page target whose URL is the app's origin.
func findTarget(listURL, appHost string) (*cdpTarget, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(listURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", listURL, err)
	}
	for i := range targets {
		t := &targets[i]
		if t.Type == "page" && strings.Contains(t.URL, appHost) && t.WSURL != "" {
			return t, nil
		}
	}
	return nil, fmt.Errorf("no page target for %s yet", appHost)
}

func (d *cdp) close() {
	d.cancel()
	_ = d.conn.Close(websocket.StatusNormalClosure, "test done")
	d.mu.Lock()
	if d.console != nil {
		_ = d.console.Close()
		d.console = nil
	}
	d.mu.Unlock()
}

// call sends one CDP command and waits for its reply.
func (d *cdp) call(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	d.mu.Lock()
	if d.closed {
		err := d.readErr
		d.mu.Unlock()
		return nil, fmt.Errorf("devtools connection closed (%v)", err)
	}
	d.nextID++
	id := d.nextID
	ch := make(chan cdpReply, 1)
	d.pending[id] = ch
	d.mu.Unlock()

	msg := struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{ID: id, Method: method, Params: params}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	wctx, wdone := context.WithTimeout(d.ctx, timeout)
	defer wdone()
	if err := d.conn.Write(wctx, websocket.MessageText, data); err != nil {
		return nil, fmt.Errorf("cdp %s: %w", method, err)
	}

	select {
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("cdp %s: %w", method, r.err)
		}
		return r.result, nil
	case <-time.After(timeout):
		d.mu.Lock()
		delete(d.pending, id)
		d.mu.Unlock()
		return nil, fmt.Errorf("cdp %s: no reply after %s", method, timeout)
	case <-d.ctx.Done():
		return nil, fmt.Errorf("cdp %s: connection closed", method)
	}
}

func (d *cdp) readLoop() {
	for {
		_, data, err := d.conn.Read(d.ctx)
		if err != nil {
			d.mu.Lock()
			d.closed = true
			d.readErr = err
			for id, ch := range d.pending {
				ch <- cdpReply{err: fmt.Errorf("connection closed: %w", err)}
				delete(d.pending, id)
			}
			d.mu.Unlock()
			return
		}
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if msg.ID != 0 {
			d.mu.Lock()
			ch := d.pending[msg.ID]
			delete(d.pending, msg.ID)
			d.mu.Unlock()
			if ch != nil {
				r := cdpReply{result: msg.Result}
				if msg.Error != nil {
					r.err = fmt.Errorf("%s (code %d)", msg.Error.Message, msg.Error.Code)
				}
				ch <- r
			}
			continue
		}
		switch msg.Method {
		case "Runtime.consoleAPICalled":
			d.logConsole(msg.Params)
		case "Runtime.exceptionThrown":
			d.logException(msg.Params)
		case "Page.screencastFrame":
			d.handleScreencastFrame(msg.Params)
		}
	}
}

// fire sends a command without waiting for its reply - for calls made
// from the read loop itself, where waiting would deadlock.
func (d *cdp) fire(method string, params any) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.nextID++
	id := d.nextID
	d.mu.Unlock()
	msg := struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{ID: id, Method: method, Params: params}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	wctx, wdone := context.WithTimeout(d.ctx, 10*time.Second)
	defer wdone()
	_ = d.conn.Write(wctx, websocket.MessageText, data)
}

// startScreencast asks the browser for a JPEG frame on every visual
// change, feeding them to onFrame (the recorder).
func (d *cdp) startScreencast(onFrame func([]byte, float64)) error {
	d.mu.Lock()
	d.onFrame = onFrame
	d.mu.Unlock()
	_, err := d.call("Page.startScreencast", map[string]any{
		"format": "jpeg", "quality": 70,
		"maxWidth": 1280, "maxHeight": 1280,
		"everyNthFrame": 1,
	}, 10*time.Second)
	return err
}

// jpegFrame captures one JPEG still - the recorder's forced frame
// after a driver action (an off-screen window's compositor can idle
// and starve the screencast; explicit captures always work).
func (d *cdp) jpegFrame() ([]byte, error) {
	res, err := d.call("Page.captureScreenshot",
		map[string]any{"format": "jpeg", "quality": 70}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out.Data)
}

func (d *cdp) stopScreencast() {
	d.mu.Lock()
	live := !d.closed
	d.onFrame = nil
	d.mu.Unlock()
	if live {
		_, _ = d.call("Page.stopScreencast", nil, 5*time.Second)
	}
}

func (d *cdp) handleScreencastFrame(params json.RawMessage) {
	var p struct {
		Data      string `json:"data"`
		SessionID int    `json:"sessionId"`
		Metadata  struct {
			Timestamp float64 `json:"timestamp"`
		} `json:"metadata"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	// Unacknowledged screencasts stall after a few frames.
	d.fire("Page.screencastFrameAck", map[string]any{"sessionId": p.SessionID})
	d.mu.Lock()
	cb := d.onFrame
	d.mu.Unlock()
	if cb == nil {
		return
	}
	data, err := base64.StdEncoding.DecodeString(p.Data)
	if err != nil {
		return
	}
	t := p.Metadata.Timestamp
	if t == 0 {
		t = float64(time.Now().UnixNano()) / 1e9
	}
	cb(data, t)
}

// remoteValue is the slice of Runtime.RemoteObject the console log
// needs.
type remoteValue struct {
	Type        string          `json:"type"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
}

func (v remoteValue) String() string {
	if len(v.Value) > 0 {
		return string(v.Value)
	}
	if v.Description != "" {
		return v.Description
	}
	return v.Type
}

func (d *cdp) logConsole(params json.RawMessage) {
	var p struct {
		Type string        `json:"type"`
		Args []remoteValue `json:"args"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	parts := make([]string, len(p.Args))
	for i, a := range p.Args {
		parts[i] = a.String()
	}
	d.writeConsole(fmt.Sprintf("[%s] %s", p.Type, strings.Join(parts, " ")))
}

func (d *cdp) logException(params json.RawMessage) {
	var p struct {
		ExceptionDetails struct {
			Text      string       `json:"text"`
			Exception *remoteValue `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	detail := p.ExceptionDetails.Text
	if p.ExceptionDetails.Exception != nil {
		detail += " " + p.ExceptionDetails.Exception.String()
	}
	d.writeConsole("[exception] " + detail)
}

func (d *cdp) writeConsole(line string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.console == nil {
		return
	}
	fmt.Fprintf(d.console, "%s %s\n", time.Now().Format(time.RFC3339Nano), line)
}

// eval runs a JS expression in the page and returns its JSON value.
// The expression is awaited, so async IIFEs work too.
func (d *cdp) eval(expr string) (json.RawMessage, error) {
	res, err := d.call("Runtime.evaluate", map[string]any{
		"expression":    expr,
		"returnByValue": true,
		"awaitPromise":  true,
	}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	var out struct {
		Result           remoteValue `json:"result"`
		ExceptionDetails *struct {
			Text      string       `json:"text"`
			Exception *remoteValue `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	if out.ExceptionDetails != nil {
		detail := out.ExceptionDetails.Text
		if out.ExceptionDetails.Exception != nil {
			detail += " " + out.ExceptionDetails.Exception.String()
		}
		return nil, fmt.Errorf("evaluating in the page: %s", detail)
	}
	return out.Result.Value, nil
}

// screenshot captures the page (or a clip of it) as PNG bytes.
func (d *cdp) screenshot(clip map[string]any) ([]byte, error) {
	params := map[string]any{"format": "png"}
	if clip != nil {
		params["clip"] = clip
	}
	res, err := d.call("Page.captureScreenshot", params, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out.Data)
}

// mouseClick dispatches a real move-press-release at viewport
// coordinates (CSS px).
func (d *cdp) mouseClick(x, y float64) error {
	steps := []map[string]any{
		{"type": "mouseMoved", "x": x, "y": y, "button": "none", "pointerType": "mouse"},
		{"type": "mousePressed", "x": x, "y": y, "button": "left", "buttons": 1, "clickCount": 1, "pointerType": "mouse"},
		{"type": "mouseReleased", "x": x, "y": y, "button": "left", "buttons": 0, "clickCount": 1, "pointerType": "mouse"},
	}
	for _, s := range steps {
		if _, err := d.call("Input.dispatchMouseEvent", s, 10*time.Second); err != nil {
			return err
		}
	}
	return nil
}

// typeText dispatches per-character key events into the focused
// element - real keydown/keyup with text, so input events fire the way
// user typing does.
func (d *cdp) typeText(text string) error {
	for _, r := range text {
		ch := string(r)
		down := map[string]any{"type": "keyDown", "text": ch, "key": ch, "unmodifiedText": ch}
		up := map[string]any{"type": "keyUp", "key": ch}
		if _, err := d.call("Input.dispatchKeyEvent", down, 10*time.Second); err != nil {
			return err
		}
		if _, err := d.call("Input.dispatchKeyEvent", up, 10*time.Second); err != nil {
			return err
		}
	}
	return nil
}

// insertText commits text into the focused element in one edit (the
// IME path) - what Fill uses after select-all.
func (d *cdp) insertText(text string) error {
	_, err := d.call("Input.insertText", map[string]any{"text": text}, 10*time.Second)
	return err
}

// pressKey sends one named key (e.g. "Backspace" with its Windows
// virtual key code) down and up.
func (d *cdp) pressKey(key string, vk int) error {
	down := map[string]any{"type": "rawKeyDown", "key": key, "windowsVirtualKeyCode": vk, "nativeVirtualKeyCode": vk}
	up := map[string]any{"type": "keyUp", "key": key, "windowsVirtualKeyCode": vk, "nativeVirtualKeyCode": vk}
	if _, err := d.call("Input.dispatchKeyEvent", down, 10*time.Second); err != nil {
		return err
	}
	_, err := d.call("Input.dispatchKeyEvent", up, 10*time.Second)
	return err
}
