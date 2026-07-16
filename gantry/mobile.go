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
)

// registeredWidgets holds the app's home-screen widgets. The concrete
// widget type arrives with the widget package; any keeps the generated
// registry compiling until then.
var registeredWidgets []any

// RegisterWidgets records the app's home-screen widgets - called by
// the generated gantry_widgets.go, one widget per widgets/<name> dir.
func RegisterWidgets(ws ...any) {
	registeredWidgets = append(registeredWidgets, ws...)
}

// emitWidgets writes the versioned widget envelope: the Android shell
// runs the binary with --emit-widgets to refresh home-screen widgets
// without booting the whole app. Stub until the widget package lands -
// an empty envelope with the versioned shape.
func emitWidgets(w io.Writer) error {
	env := struct {
		Version int   `json:"version"`
		Widgets []any `json:"widgets"`
	}{Version: 1, Widgets: []any{}}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
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
