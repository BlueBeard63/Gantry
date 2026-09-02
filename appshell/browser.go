package appshell

import "github.com/B-Commissions/Gantry/internal/launch"

// OpenInBrowser opens url in the OS default browser.
func OpenInBrowser(url string) error {
	return launch.OpenInBrowser(url)
}
