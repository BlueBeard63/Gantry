package ui

import (
	"time"
)

// ErrorInfo is one captured error - a recovered Go panic, a JS error
// reported by the frontend, or a crash trace recovered from the last
// run. Every capture carries the page the user was on and the trail of
// actions that led there, so an error report reads as a story, not a
// bare stack.
type ErrorInfo struct {
	// Kind names the capture site: "event-panic" | "call-panic" |
	// "cmd-panic" | "tea-update-panic" | "tea-view-panic" |
	// "goroutine-panic" | "http-panic" | "process-crash" | "js-error" |
	// "js-rejection" | "react-render".
	Kind string `json:"kind"`
	// Code is the gerr code ("panic.call", "js.error", ...).
	Code string `json:"code"`
	// Source locates the failure: "pages/index.increment", a URL path,
	// a component name.
	Source  string    `json:"source,omitempty"`
	Message string    `json:"message"`
	Stack   string    `json:"stack,omitempty"`
	Time    time.Time `json:"time"`
	// Page is the active page key when the error fired.
	Page string `json:"page,omitempty"`
	// Trail is the recent user actions leading up to the error, oldest
	// first.
	Trail []Crumb `json:"trail,omitempty"`
}

// Crumb is one breadcrumb in an error's trail. The server records them
// automatically from the websocket traffic (page mounts, events, calls,
// state writes); App.Breadcrumb adds app-specific ones.
type Crumb struct {
	Time time.Time `json:"time"`
	// Type is "navigate" | "event" | "call" | "state" | "custom".
	Type string `json:"type"`
	// Detail is the human line: "pages/settings mounted",
	// "pages/index.increment", "auth.login -> err: bad password".
	Detail string `json:"detail"`
	// OK is false when the action itself failed (a call returned an
	// error or panicked).
	OK bool `json:"ok"`
}

const (
	maxErrors = 20
	maxCrumbs = 50
)

// SetErrorHook installs the app-level interceptor ReportError runs
// before pushing an error to the frontend. The hook may mutate the
// error (strip the stack, rewrite the message) and returns false to
// suppress the push - the error stays recorded either way. Normally
// wired by gantry.Config.Errors, not called directly.
func (a *App) SetErrorHook(fn func(*ErrorInfo) bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errHook = fn
}

// DisableErrorCapture turns the error pipeline off entirely: no
// recording, no pushes. Recovered panics still log to the terminal.
func (a *App) DisableErrorCapture() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errDisabled = true
}

// ReportError records an error, stamps the page and breadcrumb trail,
// runs the app's error hook, and pushes an {"t":"error"} frame to the
// connected client. Recovered panics inside the framework flow through
// here; apps can call it to escalate their own errors into the same
// pipeline.
func (a *App) ReportError(e ErrorInfo) {
	a.reportError(e, true)
}

// ReportErrorFromClient is ReportError for JS-side errors: the
// frontend already displayed them, so record + hook only, no echo push.
func (a *App) ReportErrorFromClient(e ErrorInfo) {
	a.reportError(e, false)
}

func (a *App) reportError(e ErrorInfo, push bool) {
	a.mu.Lock()
	if a.errDisabled {
		a.mu.Unlock()
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if e.Page == "" {
		e.Page = a.activePage
	}
	if e.Trail == nil {
		e.Trail = append([]Crumb(nil), a.crumbs...)
	}
	hook := a.errHook
	a.mu.Unlock()

	if hook != nil && !hook(&e) {
		push = false
	}

	a.mu.Lock()
	a.errors = append(a.errors, e)
	if len(a.errors) > maxErrors {
		a.errors = a.errors[len(a.errors)-maxErrors:]
	}
	a.mu.Unlock()

	if push {
		for _, c := range a.allClients() {
			c.write(struct {
				T string    `json:"t"`
				P ErrorInfo `json:"p"`
			}{T: "error", P: e})
		}
	}
}

// RecentErrors returns the captured errors, oldest first. The built-in
// "gantry" service serves them to the frontend (call("gantry","errors")).
func (a *App) RecentErrors() []ErrorInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]ErrorInfo(nil), a.errors...)
}

// ClearErrors drops the captured errors.
func (a *App) ClearErrors() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errors = nil
}

// Breadcrumb records an app-specific action in the error trail
// ("sync started", "importing backup"), alongside the automatic
// navigate/event/call crumbs.
func (a *App) Breadcrumb(detail string) {
	a.recordCrumb("custom", detail, true)
}

func (a *App) recordCrumb(typ, detail string, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.errDisabled {
		return
	}
	a.crumbs = append(a.crumbs, Crumb{Time: time.Now(), Type: typ, Detail: detail, OK: ok})
	if len(a.crumbs) > maxCrumbs {
		a.crumbs = a.crumbs[len(a.crumbs)-maxCrumbs:]
	}
}
