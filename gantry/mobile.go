package gantry

// Mobile-shell plumbing. The Android shell runs the app's Go server as
// a child process, so the runtime grows three loopback affordances:
// a ready handshake on stdout (--announce-ready), a shared-token guard
// so only the shell's WebView can talk to the server (--token), and a
// widget snapshot channel (--emit-widgets + RegisterWidgets).

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"

	"github.com/B-Commissions/Gantry/notification"
	"github.com/B-Commissions/Gantry/widget"
)

// registeredWidgets holds the app's home-screen widgets.
var registeredWidgets []widget.Widget

// RegisterWidgets records the app's home-screen widgets - called by
// the generated gantry_widgets.go, one widget per widgets/<name> dir.
func RegisterWidgets(ws ...widget.Widget) {
	registeredWidgets = append(registeredWidgets, ws...)
}

// widgetSnapshot is one rendered widget in the envelope.
type widgetSnapshot struct {
	Name string      `json:"name"`
	Root widget.Node `json:"root"`
}

// widgetEnvelope renders every registered widget into the versioned
// shape shared by --emit-widgets and /gantry/widgets.json. The Kotlin
// shell splits it into per-widget files and hands each root to the
// Glance renderer.
func widgetEnvelope() any {
	snaps := make([]widgetSnapshot, 0, len(registeredWidgets))
	for _, w := range registeredWidgets {
		snaps = append(snaps, widgetSnapshot{Name: w.Name, Root: w.Render()})
	}
	return struct {
		Version int              `json:"version"`
		Widgets []widgetSnapshot `json:"widgets"`
	}{Version: 1, Widgets: snaps}
}

// emitWidgets writes the widget envelope: the Android shell runs the
// binary with --emit-widgets to refresh home-screen widgets in the
// background without booting the whole app.
func emitWidgets(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(widgetEnvelope())
}

// widgetsHandler serves the same envelope from the running server, so
// the shell can refresh widgets for free whenever the app is open.
func widgetsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(widgetEnvelope())
	})
}

// notifyActionHandler receives notification button taps from the
// shell (which Android delivers even when the tap has to start the app
// process first) and routes them to the app's OnAction handler.
func notifyActionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		notification.Dispatch(q.Get("id"), q.Get("action"))
		w.WriteHeader(http.StatusNoContent)
	})
}

// tokenHandler guards every route with a shared secret when token is
// non-empty: a request must carry the gantry_token cookie, or the
// ?gantry_token= query which sets that cookie (how the shell's first
// page load authenticates). Anything else on the loopback port - other
// apps on the device - gets 403. Empty token = no guard (desktop).
func tokenHandler(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	ok := func(v string) bool { return subtle.ConstantTimeCompare([]byte(v), []byte(token)) == 1 }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("gantry_token"); err == nil && ok(c.Value) {
			next.ServeHTTP(w, r)
			return
		}
		if ok(r.URL.Query().Get("gantry_token")) {
			http.SetCookie(w, &http.Cookie{
				Name:     "gantry_token",
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}
