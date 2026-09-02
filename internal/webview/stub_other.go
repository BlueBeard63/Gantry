//go:build !linux || android || nogui

// Package webview is a Linux-only fork of github.com/webview/webview_go
// at commit 6173450, patched to link webkit2gtk-4.1 by default (see
// webview.go). This stub only exists so the package compiles on every
// platform; nothing imports it outside linux && !nogui builds.
package webview
