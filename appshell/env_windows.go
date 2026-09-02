//go:build windows

package appshell

import (
	"os"
	"strings"
	"sync"
)

var winEnvFixOnce sync.Once

// windowsEnvFixes applies environment tweaks the WebView2 loader reads,
// before the first webview environment is created in this process.
//
// Capture compatibility: WebView2 content is composited by DWM straight
// from the GPU (WS_EX_NOREDIRECTIONBITMAP), so legacy BitBlt capturers
// record black. Passing --disable-gpu-compositing makes Chromium render
// into the window surface, which BitBlt can read. go-webview2 exposes no
// environment-options parameter (it calls
// createCoreWebView2EnvironmentWithOptions with options=0), so the flag
// goes through the WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS environment
// variable, which the loader honors. Widget/popup child processes
// inherit the variable, so one Setenv here covers every window.
//
// GANTRY_CAPTURE_COMPAT=1 forces it on, =0 forces it off, unset defers
// to the app's WindowOptions.CaptureCompatible.
func windowsEnvFixes(captureCompat bool) {
	winEnvFixOnce.Do(func() {
		on := captureCompat
		switch strings.ToLower(os.Getenv("GANTRY_CAPTURE_COMPAT")) {
		case "1", "true", "yes", "on":
			on = true
		case "0", "false", "no", "off":
			on = false
		}
		if !on {
			return
		}
		const flag = "--disable-gpu-compositing"
		args := os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS")
		if strings.Contains(args, flag) {
			return
		}
		if args != "" {
			args += " "
		}
		_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", args+flag)
	})
}
