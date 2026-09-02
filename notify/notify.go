// Package notify shows notification popups: small frameless
// always-on-top windows that never steal focus and that Windows Do Not
// Disturb cannot suppress (they are app-drawn, not shell toasts). Each
// popup runs as its own child process so a webview crash can never take
// the host app down, and dismissing one is just a process kill.
package notify

import (
	"strconv"

	"github.com/BlueBeard63/Gantry/appshell"
)

// Notifier spawns popup windows by re-invoking the app's exe. The
// child process must recognize the spawn args and call
// appshell.RunPopup - gantry new generates that dispatch:
//
//	case "--shellrole popup": appshell.RunPopup(...) with --url/--monitor/--position
type Notifier struct {
	// Proc manages the popup child process (share one ProcManager with
	// the app's widgets).
	Proc *appshell.ProcManager
	// BaseURL prefixes Show's path, e.g. "http://127.0.0.1:8330".
	BaseURL string
	// Placement returns the monitor index and "top"/"bottom" position,
	// evaluated at each Show so settings changes apply live. nil =
	// primary monitor, bottom.
	Placement func() (monitor int, position string)
	// Args builds the child's argv given the popup URL, monitor and
	// position. nil = the standard gantry scaffold flags:
	// --shellrole popup --url U --monitor N --position P
	Args func(url string, monitor int, position string) []string
}

// Show replaces any visible popup with one rendering BaseURL+path.
func (n *Notifier) Show(path string) {
	monitor, position := -1, "bottom"
	if n.Placement != nil {
		monitor, position = n.Placement()
	}
	url := n.BaseURL + path
	args := n.Args
	if args == nil {
		args = func(url string, monitor int, position string) []string {
			return []string{
				"--shellrole", "popup",
				"--url", url,
				"--monitor", strconv.Itoa(monitor),
				"--position", position,
			}
		}
	}
	n.Proc.Replace("popup", args(url, monitor, position)...)
}

// Close dismisses the current popup, if any.
func (n *Notifier) Close() {
	n.Proc.Kill("popup")
}

// Attention plays the system notification sound and flashes the main
// window's taskbar button - for moments that deserve attention without a
// popup.
func Attention() {
	appshell.AttentionMainWindow()
}
