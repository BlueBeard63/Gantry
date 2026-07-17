package gantry

import (
	"os"
	"sync/atomic"
)

// The two app modes. gantry dev runs the app in development; a built
// binary defaults to production. Apps gate pages, features and error
// detail on them via IsDev / Mode (Go) and useMode / useEnv (React).
const (
	ModeDevelopment = "development"
	ModeProduction  = "production"
)

// devURLSeen flips when Run parses a --dev-url flag - the fallback
// signal for development when GANTRY_MODE is absent (covers a plain
// `go run . --dev-url ...` without the CLI).
var devURLSeen atomic.Bool

func setDevURLSeen() { devURLSeen.Store(true) }

// Mode returns the app's mode: the GANTRY_MODE environment variable
// (gantry dev sets development), else development when Run saw a
// --dev-url flag, else production. A shipped binary needs nothing set
// to be production.
func Mode() string {
	switch os.Getenv("GANTRY_MODE") {
	case ModeDevelopment:
		return ModeDevelopment
	case ModeProduction:
		return ModeProduction
	}
	if devURLSeen.Load() {
		return ModeDevelopment
	}
	return ModeProduction
}

// IsDev reports whether the app runs in development mode.
func IsDev() bool { return Mode() == ModeDevelopment }
