//go:build !windows && (!linux || android || nogui)

package monitors

// All returns no monitors on platforms without an enumerator; Pick falls
// back to a sane default.
func All() []Monitor { return nil }
