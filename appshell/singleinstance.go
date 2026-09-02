package appshell

import (
	"net"

	"github.com/B-Commissions/Gantry/internal/launch"
)

// Listen binds the app's local loopback port, doubling as the
// single-instance guard: gantry apps serve their frontend from a fixed
// local port, so if the bind fails another instance already holds it.
// Pass the listener to http.Server.Serve.
func Listen(port int) (net.Listener, error) {
	return launch.Listen(port)
}
