package gantrytest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// webkit is the DOM-plane connection for WebKitGTK - the Linux desktop
// webview, which is not Chromium and speaks the WebKit remote inspector
// protocol, not CDP. It satisfies the same domClient seam as cdp, so the
// element-action code in dom.go is identical on Linux; only the wire
// dialect differs.
//
// Two things differ from CDP by necessity, both mirroring choices the
// project already made elsewhere:
//
//   - Input is synthesized in the page via Runtime.evaluate (dispatching
//     real MouseEvent/KeyboardEvent/input events), because the WebKit
//     inspector has no dependable synthetic-input domain - the same
//     reason the Android backend drives native taps instead of CDP input.
//   - Screencasts are assembled from periodic Page.snapshotRect stills
//     (WebKit has no startScreencast), each re-encoded to JPEG so the
//     MJPEG recorder pipeline (record.go) is unchanged.
//
// VALIDATION NOTE: WebKitGTK's remote-inspector discovery - the HTTP
// listing served on WEBKIT_INSPECTOR_SERVER and the per-target websocket
// path - is version-dependent and not formally documented. discoverWebKit
// below parses the listing for a target websocket, with a documented
// fallback. This client is written against the seam and gated to Linux;
// run the demo DOM suite on a WebKitGTK host (under xvfb) to confirm the
// discovery and domain method names for your WebKit version before
// relying on it in CI. Windows (WebView2/CDP) and device paths are
// untouched by this file.
type webkit struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	tr     *trace

	mu      sync.Mutex
	nextID  int
	pending map[int]chan cdpReply
	console *os.File
	closed  bool
	readErr error

	castStop chan struct{}
}

// attachWebKit discovers the app's inspector target, dials it, and
// enables the domains the driver reads.
func attachWebKit(tr *trace, inspectorPort, appPort int, consolePath string, timeout time.Duration) (*webkit, error) {
	deadline := time.Now().Add(timeout)
	base := fmt.Sprintf("http://127.0.0.1:%d", inspectorPort)

	var wsURL string
	for wsURL == "" {
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("timed out after %s waiting for the WebKitGTK inspector on port %d (is WebKitGTK installed, WEBKIT_INSPECTOR_SERVER honoured, and did the window open?)", timeout, inspectorPort)
		}
		if u, err := discoverWebKit(base); err == nil {
			wsURL = u
		} else {
			time.Sleep(200 * time.Millisecond)
		}
	}
	tr.action("webkit: attaching to %s", wsURL)

	ctx, cancel := context.WithCancel(context.Background())
	dialCtx, dialDone := context.WithTimeout(ctx, 15*time.Second)
	defer dialDone()
	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("dialing the WebKit inspector websocket %s: %w", wsURL, err)
	}
	conn.SetReadLimit(64 << 20) // snapshots come back as one base64 frame

	console, err := os.Create(consolePath)
	if err != nil {
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, err
	}

	d := &webkit{conn: conn, ctx: ctx, cancel: cancel, tr: tr, pending: map[int]chan cdpReply{}, console: console}
	go d.readLoop()

	// Best-effort domain enables; WebKit tolerates unknown/duplicate
	// enables differently across versions, so a failure here is logged,
	// not fatal - eval and snapshots work without them.
	for _, method := range []string{"Runtime.enable", "Console.enable", "Page.enable"} {
		if _, err := d.call(method, nil, 10*time.Second); err != nil {
			tr.action("webkit: %s not available (%v)", method, err)
		}
	}
	return d, nil
}

var webkitWSRe = regexp.MustCompile(`ws://[^"'\s<>]+`)
var webkitSocketRe = regexp.MustCompile(`(?:href|src)="([^"]*socket[^"]*)"`)

// discoverWebKit finds a target websocket URL from the inspector's HTTP
// listing. WebKitGTK does not serve CDP's /json/list; it serves an HTML
// page. We first take any explicit ws:// URL in the page, then fall back
// to a "socket" link resolved against the base. This is the
// version-dependent seam called out in the type doc.
func discoverWebKit(base string) (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(base + "/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	page := string(body)
	if m := webkitWSRe.FindString(page); m != "" {
		return m, nil
	}
	if m := webkitSocketRe.FindStringSubmatch(page); len(m) == 2 {
		path := m[1]
		wsBase := "ws://" + strings.TrimPrefix(base, "http://")
		if strings.HasPrefix(path, "/") {
			return wsBase + path, nil
		}
		return wsBase + "/" + path, nil
	}
	return "", fmt.Errorf("no inspector target in the WebKit listing yet")
}

func (d *webkit) close() {
	d.stopScreencast()
	d.cancel()
	_ = d.conn.Close(websocket.StatusNormalClosure, "test done")
	d.mu.Lock()
	if d.console != nil {
		_ = d.console.Close()
		d.console = nil
	}
	d.mu.Unlock()
}

// call sends one inspector command and waits for its reply. The JSON-RPC
// envelope is the same shape CDP uses (id/method/params -> id/result|error).
func (d *webkit) call(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	d.mu.Lock()
	if d.closed {
		err := d.readErr
		d.mu.Unlock()
		return nil, fmt.Errorf("inspector connection closed (%v)", err)
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
		return nil, fmt.Errorf("webkit %s: %w", method, err)
	}
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("webkit %s: %w", method, r.err)
		}
		return r.result, nil
	case <-time.After(timeout):
		d.mu.Lock()
		delete(d.pending, id)
		d.mu.Unlock()
		return nil, fmt.Errorf("webkit %s: no reply after %s", method, timeout)
	case <-d.ctx.Done():
		return nil, fmt.Errorf("webkit %s: connection closed", method)
	}
}

func (d *webkit) readLoop() {
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
		if msg.Method == "Console.messageAdded" {
			d.logConsole(msg.Params)
		}
	}
}

func (d *webkit) logConsole(params json.RawMessage) {
	var p struct {
		Message struct {
			Text  string `json:"text"`
			Level string `json:"level"`
		} `json:"message"`
	}
	if json.Unmarshal(params, &p) != nil || p.Message.Text == "" {
		return
	}
	d.mu.Lock()
	if d.console != nil {
		fmt.Fprintf(d.console, "[%s] %s\n", p.Message.Level, p.Message.Text)
	}
	d.mu.Unlock()
}

// eval runs a JS expression via Runtime.evaluate and returns its JSON
// value - the same contract cdp.eval provides.
func (d *webkit) eval(expr string) (json.RawMessage, error) {
	res, err := d.call("Runtime.evaluate", map[string]any{
		"expression":    expr,
		"returnByValue": true,
	}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	var out struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		WasThrown bool `json:"wasThrown"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	if out.WasThrown {
		return nil, fmt.Errorf("evaluating in the page threw")
	}
	return out.Result.Value, nil
}

// snapshotPNG captures the current viewport as PNG bytes via
// Page.snapshotRect (WebKit returns a data:image/png;base64 dataURL).
func (d *webkit) snapshotPNG(clip map[string]any) ([]byte, error) {
	rect := clip
	if rect == nil {
		// Whole viewport: ask the page for its size.
		w, h := d.viewportSize()
		rect = map[string]any{"x": 0, "y": 0, "width": w, "height": h, "coordinateSystem": "Viewport"}
	} else if _, ok := rect["coordinateSystem"]; !ok {
		rect["coordinateSystem"] = "Viewport"
	}
	res, err := d.call("Page.snapshotRect", rect, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var out struct {
		DataURL string `json:"dataURL"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	i := strings.IndexByte(out.DataURL, ',')
	if i < 0 {
		return nil, fmt.Errorf("webkit snapshot: unexpected dataURL")
	}
	return base64.StdEncoding.DecodeString(out.DataURL[i+1:])
}

func (d *webkit) viewportSize() (int, int) {
	w, h := 1280, 800
	if raw, err := d.eval("[window.innerWidth, window.innerHeight]"); err == nil {
		var wh []int
		if json.Unmarshal(raw, &wh) == nil && len(wh) == 2 && wh[0] > 0 && wh[1] > 0 {
			w, h = wh[0], wh[1]
		}
	}
	return w, h
}

// screenshot returns PNG bytes (failure.png / App.Screenshot).
func (d *webkit) screenshot(clip map[string]any) ([]byte, error) { return d.snapshotPNG(clip) }

// jpegFrame returns one JPEG still: WebKit snapshots are PNG, so re-encode
// for the MJPEG recorder, which expects JPEG.
func (d *webkit) jpegFrame() ([]byte, error) {
	pngBytes, err := d.snapshotPNG(nil)
	if err != nil {
		return nil, err
	}
	return pngToJPEG(pngBytes)
}

// startScreencast polls snapshots on a fixed cadence (WebKit has no
// screencast domain), feeding JPEG frames to the recorder with an elapsed
// timestamp. The forced frame after each action (jpegFrame) still fills
// gaps between polls exactly as it does on CDP.
func (d *webkit) startScreencast(onFrame func([]byte, float64)) error {
	d.mu.Lock()
	if d.castStop != nil {
		d.mu.Unlock()
		return nil
	}
	stop := make(chan struct{})
	d.castStop = stop
	d.mu.Unlock()

	start := time.Now()
	go func() {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-d.ctx.Done():
				return
			case <-ticker.C:
				if frame, err := d.jpegFrame(); err == nil {
					onFrame(frame, time.Since(start).Seconds())
				}
			}
		}
	}()
	return nil
}

func (d *webkit) stopScreencast() {
	d.mu.Lock()
	stop := d.castStop
	d.castStop = nil
	d.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

// ---- input via synthesized events ----
//
// The WebKit inspector has no reliable Input domain, so clicks and
// keystrokes are dispatched as genuine DOM events through Runtime.evaluate
// on the target element - React and plain listeners see real
// mousedown/mouseup/click and keydown/input events, which is what the
// driver's actions need.

func (d *webkit) mouseClick(x, y float64) error {
	expr := fmt.Sprintf(`(function(){
      var el=document.elementFromPoint(%[1]f,%[2]f); if(!el){return false;}
      ['mousedown','mouseup','click'].forEach(function(t){
        el.dispatchEvent(new MouseEvent(t,{bubbles:true,cancelable:true,clientX:%[1]f,clientY:%[2]f,button:0}));
      });
      if(typeof el.focus==='function'){el.focus();}
      return true;
    })()`, x, y)
	_, err := d.eval(expr)
	return err
}

func (d *webkit) typeText(text string) error {
	expr := fmt.Sprintf(`(function(){
      var el=document.activeElement; if(!el){return false;}
      var s=%s;
      for(var i=0;i<s.length;i++){
        var ch=s[i];
        el.dispatchEvent(new KeyboardEvent('keydown',{bubbles:true,key:ch}));
        if('value' in el){el.value=(el.value||'')+ch;}
        el.dispatchEvent(new InputEvent('input',{bubbles:true,data:ch}));
        el.dispatchEvent(new KeyboardEvent('keyup',{bubbles:true,key:ch}));
      }
      return true;
    })()`, jsString(text))
	_, err := d.eval(expr)
	return err
}

func (d *webkit) insertText(text string) error {
	expr := fmt.Sprintf(`(function(){
      var el=document.activeElement; if(!el){return false;}
      if('value' in el){el.value=%s;}
      el.dispatchEvent(new InputEvent('input',{bubbles:true}));
      return true;
    })()`, jsString(text))
	_, err := d.eval(expr)
	return err
}

func (d *webkit) pressKey(key string, vk int) error {
	// The driver uses pressKey mainly to clear a field (Backspace) before
	// insertText replaces the value; dispatch a real key event, and handle
	// Backspace/Delete by trimming so a subsequent insertText is clean.
	expr := fmt.Sprintf(`(function(){
      var el=document.activeElement; if(!el){return false;}
      var k=%s;
      el.dispatchEvent(new KeyboardEvent('keydown',{bubbles:true,key:k}));
      if((k==='Backspace'||k==='Delete')&&'value' in el){el.value='';el.dispatchEvent(new InputEvent('input',{bubbles:true}));}
      el.dispatchEvent(new KeyboardEvent('keyup',{bubbles:true,key:k}));
      return true;
    })()`, jsString(key))
	_, err := d.eval(expr)
	return err
}

// ---- helpers ----

func pngToJPEG(pngBytes []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// jsString renders a Go string as a JS string literal, safely escaped for
// embedding in an evaluated expression.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// compile-time assurance webkit satisfies the seam.
var _ domClient = (*webkit)(nil)
