//go:build windows && !nogui

package appshell

import (
	"reflect"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/pkg/edge"
)

// disableStatusBar turns off the WebView2 status bar - the little
// bottom-corner bubble that previews link URLs on hover. Fine in a
// browser, jarring in a desktop app.
//
// The go-webview2 wrapper does not expose its settings object, so this
// reaches the embedded *edge.Chromium through reflection. Deliberately
// best-effort: if a library update moves the field, the app keeps
// working and only the bubble comes back.
func disableStatusBar(w webview2.WebView) {
	defer func() { _ = recover() }()
	v := reflect.ValueOf(w)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	f := v.Elem().FieldByName("browser")
	if !f.IsValid() || !f.CanAddr() {
		return
	}
	f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	chromium, ok := f.Interface().(*edge.Chromium)
	if !ok || chromium == nil {
		return
	}
	settings, err := chromium.GetSettings()
	if err != nil || settings == nil {
		return
	}
	_ = settings.PutIsStatusBarEnabled(false)
}
