package appshell

import "github.com/BlueBeard63/Gantry/internal/launch"

// OpenInBrowser opens url in the OS default browser.
func OpenInBrowser(url string) error {
	return launch.OpenInBrowser(url)
}
