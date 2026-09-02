//go:build linux && !android && !nogui && !webkit2gtk40

package webview

// WebKitGTK 4.1 is the libsoup3 build of the same GTK3 API - the only
// one modern distros ship (Fedora 40+, Ubuntu 24.04+). Old distros that
// only have 4.0 build with -tags webkit2gtk40 instead.

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
*/
import "C"
