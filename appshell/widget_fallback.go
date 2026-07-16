//go:build nogui || (!windows && !linux)

package appshell

import "errors"

// RunWidget is unavailable without a native webview host.
func RunWidget(WidgetOptions) error {
	return errors.New("appshell: widget windows are not supported on this build")
}
