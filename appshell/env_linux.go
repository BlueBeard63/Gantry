//go:build linux && !nogui

package appshell

import (
	"os"
	"strings"
	"sync"
)

var envFixOnce sync.Once

// linuxEnvFixes applies known-environment workarounds before the first
// webview is created.
//
// WSLg: WebKitGTK's DMA-BUF renderer cannot drive WSL's software GL
// stack - the window comes up as a white box that ignores input
// (libEGL/MESA errors in the log). Disabling it falls back to shared
// memory rendering, which works. Only applied under WSL so real Linux
// GPUs keep the fast path; set WEBKIT_DISABLE_DMABUF_RENDERER yourself
// to override either way.
func linuxEnvFixes() {
	envFixOnce.Do(func() {
		if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") != "" {
			return
		}
		if data, err := os.ReadFile("/proc/version"); err == nil &&
			strings.Contains(strings.ToLower(string(data)), "microsoft") {
			_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
		}
	})
}
