// Package ui is Gantry's Go-driven UI layer. Apps are folders of paired
// files: pages/index/index.go + index.tsx, components/gauge/gauge.go +
// gauge.tsx. The .go half registers a Page or Component here; the .tsx
// half is a normal React component that talks to it through usePaired()
// (and, for pages with a Model, renders the Go-built tree via TeaView).
// Everything travels over one websocket that App.Handler serves.
package ui

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
)

// Handlers maps event names (what the tsx passes to send()) to Go
// functions. The payload is the JSON value sent from the tsx.
// Fire-and-forget: nothing goes back to the caller.
type Handlers map[string]func(payload json.RawMessage)

// Calls maps call names to Go functions the frontend can await:
// call()/useCall() on the tsx side resolve with the returned value or
// reject with the error. The result must be JSON-serializable.
type Calls map[string]func(payload json.RawMessage) (any, error)

// Page is the .go half of a pages/<name>/ folder.
type Page struct {
	// Key is the folder path relative to the app root: "pages/index".
	// It must match the paired .tsx (the Vite plugin derives the same
	// key from the file's location).
	Key string
	// Route is the URL path this page serves. Empty derives it from
	// Key: "pages/index" -> "/", "pages/settings" -> "/settings".
	Route string
	// Model, when set, gives the page a Tea-style state machine whose
	// View() tree the tsx renders with <TeaView />. Called once, lazily,
	// when the page first becomes active.
	Model func() Model
	// On handles events the paired tsx sends via usePaired().send.
	On Handlers
	// Call answers requests the paired tsx awaits via usePaired().call.
	Call Calls
}

// Component is the .go half of a components/<name>/ folder.
type Component struct {
	// Key is the folder path relative to the app root:
	// "components/gauge".
	Key string
	// On handles events the paired tsx sends via usePaired().send.
	On Handlers
	// Call answers requests the paired tsx awaits via usePaired().call.
	Call Calls
}

// route derives the URL path for a page key.
func (p Page) route() string {
	if p.Route != "" {
		return p.Route
	}
	name := strings.TrimPrefix(p.Key, "pages/")
	if name == "index" {
		return "/"
	}
	return "/" + name
}

// App holds every registered page and component and serves the
// websocket that connects them to their tsx halves.
type App struct {
	mu       sync.Mutex
	pages    map[string]*Page
	comps    map[string]*Component
	services map[string]Calls       // app-level callables (auth, files, ...)
	states   map[string]*stateEntry // shared state vars (ui.NewState)
	programs map[string]*program    // lazily created per Model page
	conn     *conn                  // the active client (a desktop app has one)
}

// NewApp registers pages and components (pass ui.Page and ui.Component
// values, the exported vars of your pages/ and components/ packages).
func NewApp(regs ...any) *App {
	a := &App{
		pages:    map[string]*Page{},
		comps:    map[string]*Component{},
		services: map[string]Calls{},
		states:   map[string]*stateEntry{},
		programs: map[string]*program{},
	}
	for _, r := range regs {
		switch v := r.(type) {
		case Page:
			p := v
			a.pages[p.Key] = &p
		case *Page:
			a.pages[v.Key] = v
		case Component:
			c := v
			a.comps[c.Key] = &c
		case *Component:
			a.comps[v.Key] = v
		default:
			log.Printf("ui: NewApp: ignoring unknown registration %T", r)
		}
	}
	return a
}

// Send feeds a message into every running page Model - the way external
// events (timers, file watchers, server state) reach Update.
func (a *App) Send(msg Msg) {
	a.mu.Lock()
	progs := make([]*program, 0, len(a.programs))
	for _, p := range a.programs {
		progs = append(progs, p)
	}
	a.mu.Unlock()
	for _, p := range progs {
		p.send(msg)
	}
}

// Push sends a named event to the paired tsx of a page or component
// (received via usePaired().on). No-op when no client is connected.
func (a *App) Push(key, event string, payload any) {
	a.mu.Lock()
	c := a.conn
	a.mu.Unlock()
	if c == nil {
		return
	}
	c.push(key, event, payload)
}

// program returns (creating if needed) the program for a Model page.
func (a *App) program(key string) *program {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p := a.programs[key]; p != nil {
		return p
	}
	page := a.pages[key]
	if page == nil || page.Model == nil {
		return nil
	}
	p := newProgram(key, page.Model())
	a.programs[key] = p
	return p
}

// Service registers an app-level group of callable functions under a
// bare name ("auth", "files"), reachable from the frontend with
// useService(name) / call(name, fn, payload). This is how app-wide
// hooks like useAuth() get their Go side:
//
//	app.Service("auth", ui.Calls{
//	    "me":    func(json.RawMessage) (any, error) { return currentUser, nil },
//	    "login": func(p json.RawMessage) (any, error) { ... },
//	})
func (a *App) Service(name string, calls Calls) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.services[name] = calls
}

// pairedHandler resolves a paired event to its registered handler.
func (a *App) pairedHandler(key, event string) func(json.RawMessage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p := a.pages[key]; p != nil {
		return p.On[event]
	}
	if c := a.comps[key]; c != nil {
		return c.On[event]
	}
	return nil
}

// callHandler resolves a call target: pair keys first, then services.
func (a *App) callHandler(key, name string) func(json.RawMessage) (any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p := a.pages[key]; p != nil && p.Call[name] != nil {
		return p.Call[name]
	}
	if c := a.comps[key]; c != nil && c.Call[name] != nil {
		return c.Call[name]
	}
	if s := a.services[key]; s != nil {
		return s[name]
	}
	return nil
}
