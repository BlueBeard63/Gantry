package gantry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"

	"github.com/B-Commissions/Gantry/ui"
)

// ErrorInfo is one captured error: kind, code, message, stack, the
// page the user was on and the action trail that led there. See the
// ui package for the field docs.
type ErrorInfo = ui.ErrorInfo

// ErrorAction is an OnError hook's verdict.
type ErrorAction int

const (
	// ErrorShow lets the default handling run: the error is pushed to
	// the frontend, which shows the built-in (or custom) error UI.
	ErrorShow ErrorAction = iota
	// ErrorSuppress keeps the error away from the frontend - the app
	// handled it itself (set its own service flag, held the trace).
	// It stays in RecentErrors either way.
	ErrorSuppress
)

// ErrorOptions is Config.Errors: how the app wants crashes and errors
// handled. The zero value is the default-on pipeline: capture, record,
// push to the frontend, show the built-in error UI.
type ErrorOptions struct {
	// Disable turns the pipeline off entirely: no recording, no pushes.
	// Recovered panics still log to the terminal.
	Disable bool
	// OmitStacks strips stack traces before errors reach the frontend
	// (they only ever travel over the local-only websocket, but
	// paranoid production apps can opt out).
	OmitStacks bool
	// OnError intercepts every captured error before the default
	// handling. Return ErrorSuppress to keep it away from the frontend
	// and handle it yourself; return ErrorShow (or leave OnError nil)
	// for the default.
	OnError func(ErrorInfo) ErrorAction
}

// activeApp is the running ui.App, for Go() and other package-level
// helpers that report into the error pipeline.
var activeApp atomic.Pointer[ui.App]

// wireErrors installs Config.Errors onto the app.
func wireErrors(app *ui.App, opts ErrorOptions) {
	activeApp.Store(app)
	if opts.Disable {
		app.DisableErrorCapture()
		return
	}
	app.SetErrorHook(func(e *ui.ErrorInfo) bool {
		if opts.OmitStacks {
			e.Stack = ""
		}
		if opts.OnError != nil && opts.OnError(*e) == ErrorSuppress {
			return false
		}
		return true
	})
}

// Go runs fn on its own goroutine with panic capture: a panic is
// recovered into the error pipeline (kind "goroutine-panic") instead
// of killing the process. Use it for background work; a panic in a
// plain `go func()` still crashes the app (and is recovered into
// crash.log for the next launch).
func Go(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				app := activeApp.Load()
				if app == nil {
					panic(r) // no app to report to: crash as plain go would
				}
				app.ReportError(ui.ErrorInfo{
					Kind:    "goroutine-panic",
					Code:    "panic.goroutine",
					Message: fmt.Sprint(r),
					Stack:   string(debug.Stack()),
				})
			}
		}()
		fn()
	}()
}

// crashLogPath is <user config dir>/<app>/crash.log, next to
// geometry.json.
func crashLogPath(name string) string {
	base, err := os.UserConfigDir()
	if err != nil {
		return "crash.log"
	}
	return filepath.Join(base, name, "crash.log")
}

// setupCrashLog points the Go runtime's fatal-panic output at
// crash.log (so an uncatchable crash leaves its trace even in
// windowed builds with no console) and returns the previous run's
// trace when one is waiting there.
func setupCrashLog(name string) (lastCrash string) {
	path := crashLogPath(name)
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		lastCrash = string(data)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return lastCrash
	}
	f, err := os.Create(path) // truncates: the old trace is consumed
	if err != nil {
		return lastCrash
	}
	// The file handle stays open for the process lifetime on purpose.
	_ = debug.SetCrashOutput(f, debug.CrashOptions{})
	return lastCrash
}

// reportLastCrash records a previous run's fatal trace. No live push:
// the frontend picks it up from call("gantry","errors") on connect.
func reportLastCrash(app *ui.App, trace string) {
	app.ReportError(ui.ErrorInfo{
		Kind:    "process-crash",
		Code:    "panic.fatal",
		Message: "the app crashed on a previous run",
		Stack:   trace,
	})
}

// recoverHandler wraps the app's HTTP mux: net/http normally swallows
// handler panics, this reports them into the error pipeline and
// answers a coded 500.
func recoverHandler(app *ui.App, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec) // the conventional "drop this connection" signal
			}
			app.ReportError(ui.ErrorInfo{
				Kind:    "http-panic",
				Code:    "panic.http",
				Source:  r.URL.Path,
				Message: fmt.Sprint(rec),
				Stack:   string(debug.Stack()),
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprint(rec),
				"code":  "panic.http",
			})
		}()
		next.ServeHTTP(w, r)
	})
}
