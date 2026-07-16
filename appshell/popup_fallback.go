//go:build nogui || android || (!windows && !linux)

package appshell

// RunPopup without a native host opens the popup page in the default
// browser instead of a dedicated always-on-top window.
func RunPopup(opts PopupOptions) error {
	if err := opts.normalize(); err != nil {
		return err
	}
	return OpenInBrowser(opts.URL)
}

// FindWindowVisible always reports false without a native window host.
func FindWindowVisible(string) bool { return false }
