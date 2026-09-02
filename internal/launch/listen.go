// Package launch holds the small pure-Go pieces of app startup - opening
// the default browser and binding the app's loopback port - so the CLI
// can use them without importing appshell (whose Linux build needs the
// GTK/WebKitGTK dev stack via cgo).
package launch

import (
	"fmt"
	"net"
)

// Listen binds the app's local loopback port, doubling as the
// single-instance guard: gantry apps serve their frontend from a fixed
// local port, so if the bind fails another instance already holds it.
// Pass the listener to http.Server.Serve.
func Listen(port int) (net.Listener, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("appshell: listening on %s (already running?): %w", addr, err)
	}
	return ln, nil
}
