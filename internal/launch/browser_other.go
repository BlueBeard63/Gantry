//go:build !windows

package launch

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenInBrowser opens url in the OS default browser.
func OpenInBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return fmt.Errorf("launch: no browser opener for %s", runtime.GOOS)
	}
}
