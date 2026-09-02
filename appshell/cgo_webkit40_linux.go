//go:build linux && !android && !nogui && webkit2gtk40

package appshell

// Legacy pkg-config name for distros that never got webkit2gtk-4.1
// (Debian 11, Ubuntu 20.04). Selected with -tags webkit2gtk40.

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.0
*/
import "C"
