package appshell

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// ServeSPA serves an embedded frontend build: real files (assets) are
// served as-is, and every other path falls back to index.html so
// client-side routes like /settings work on reload and in widget or
// popup windows.
func ServeSPA(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(dist, p); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
