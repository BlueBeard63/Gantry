package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Wire protocol (one websocket, JSON text frames).
//
// client -> server:
//
//	{"t":"ready","page":"pages/index"}          page mounted; start/attach its program
//	{"t":"event","h":"h42","p":<json>}          Tea handler event (id from the render tree)
//	{"t":"event","key":"pages/index","name":"buttonPress","p":<json>}   paired event
//
// server -> client:
//
//	{"t":"render","seq":1,"tree":{...}}          full tree for the active page
//	{"t":"push","key":"components/gauge","name":"state","p":<json>}     paired push
type clientMsg struct {
	T    string          `json:"t"`
	Page string          `json:"page,omitempty"`
	H    string          `json:"h,omitempty"`
	Key  string          `json:"key,omitempty"`
	Name string          `json:"name,omitempty"`
	ID   string          `json:"id,omitempty"`
	P    json.RawMessage `json:"p,omitempty"`
}

type replyMsg struct {
	T   string `json:"t"`
	ID  string `json:"id"`
	OK  bool   `json:"ok"`
	P   any    `json:"p,omitempty"`
	Err string `json:"err,omitempty"`
}

type renderMsg struct {
	T    string   `json:"t"`
	Seq  uint64   `json:"seq"`
	Tree wireNode `json:"tree"`
}

type pushMsg struct {
	T    string `json:"t"`
	Key  string `json:"key"`
	Name string `json:"name"`
	P    any    `json:"p,omitempty"`
}

// conn is one connected client. A desktop app has exactly one; a new
// connection (webview reload, React StrictMode remount, dev HMR)
// replaces the old.
type conn struct {
	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	seq    uint64
	page   string // active page key
	closed bool
}

func (c *conn) write(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, data); err != nil {
		c.closed = true
	}
}

func (c *conn) sendRender(tree wireNode) {
	c.mu.Lock()
	c.seq++
	seq := c.seq
	c.mu.Unlock()
	c.write(renderMsg{T: "render", Seq: seq, Tree: tree})
}

func (c *conn) push(key, event string, payload any) {
	c.write(pushMsg{T: "push", Key: key, Name: event, P: payload})
}

// Handler returns the websocket endpoint. Mount it where the frontend
// expects it: mux.Handle("/gantry/ws", app.Handler()).
func (a *App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// Local-only server; the origin is the app's own page or the
			// Vite dev server, so skip origin checks.
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		c := &conn{ws: ws, ctx: ctx, cancel: cancel}

		// This client is now THE client; detach and close any predecessor.
		a.mu.Lock()
		old := a.conn
		a.conn = c
		a.mu.Unlock()
		if old != nil {
			old.detach(a)
			old.cancel()
			_ = old.ws.Close(websocket.StatusNormalClosure, "replaced")
		}

		// A fresh client starts from the current shared state.
		for _, msg := range a.snapshotStates() {
			c.write(msg)
		}

		// Keepalive pings.
		go func() {
			t := time.NewTicker(30 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					pctx, pcancel := context.WithTimeout(ctx, 5*time.Second)
					_ = ws.Ping(pctx)
					pcancel()
				}
			}
		}()

		a.readLoop(c)

		c.detach(a)
		cancel()
		a.mu.Lock()
		if a.conn == c {
			a.conn = nil
		}
		a.mu.Unlock()
		_ = ws.Close(websocket.StatusNormalClosure, "")
	})
}

// detach stops render delivery to this conn's active page.
func (c *conn) detach(a *App) {
	c.mu.Lock()
	page := c.page
	c.page = ""
	c.mu.Unlock()
	if page == "" {
		return
	}
	a.mu.Lock()
	prog := a.programs[page]
	a.mu.Unlock()
	if prog != nil {
		prog.setDeliver(nil)
	}
}

func (a *App) readLoop(c *conn) {
	for {
		_, data, err := c.ws.Read(c.ctx)
		if err != nil {
			return
		}
		var msg clientMsg
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.T {
		case "ready":
			// The client mounted a page: move render delivery to it.
			c.detach(a)
			c.mu.Lock()
			c.page = msg.Page
			c.mu.Unlock()
			if prog := a.program(msg.Page); prog != nil {
				prog.setDeliver(c.sendRender)
			}

		case "event":
			switch {
			case msg.H != "":
				// Tea handler event for the conn's active page.
				c.mu.Lock()
				page := c.page
				c.mu.Unlock()
				a.mu.Lock()
				prog := a.programs[page]
				a.mu.Unlock()
				if prog != nil {
					prog.handleEvent(msg.H, msg.P)
				}
			case msg.Key != "":
				// Paired event for a page or component.
				if fn := a.pairedHandler(msg.Key, msg.Name); fn != nil {
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("ui: %s.%s handler panicked: %v", msg.Key, msg.Name, r)
							}
						}()
						fn(msg.P)
					}()
				} else {
					log.Printf("ui: no handler for %s.%s (check the Key in your Page/Component and that main.go registers it)", msg.Key, msg.Name)
				}
			}

		case "call":
			// Awaited request: run off the read loop (calls may be
			// slow) and reply with the result or the error.
			fn := a.callHandler(msg.Key, msg.Name)
			id := msg.ID
			if fn == nil {
				c.write(replyMsg{T: "reply", ID: id, OK: false,
					Err: fmt.Sprintf("no call %q on %q", msg.Name, msg.Key)})
				continue
			}
			payload := msg.P
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("ui: call %s.%s panicked: %v", msg.Key, msg.Name, r)
						c.write(replyMsg{T: "reply", ID: id, OK: false, Err: "internal error"})
					}
				}()
				result, err := fn(payload)
				if err != nil {
					c.write(replyMsg{T: "reply", ID: id, OK: false, Err: err.Error()})
					return
				}
				c.write(replyMsg{T: "reply", ID: id, OK: true, P: result})
			}()

		case "setstate":
			a.applyFrontendState(msg.Key, msg.P)
		}
	}
}
