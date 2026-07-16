//go:build windows && !nogui

package appshell

import "unsafe"

// attention plays the system notification sound and flashes the window's
// taskbar button orange until it comes to the foreground - the
// "something happened while you weren't looking" combo.
func attention(hwnd uintptr) {
	const (
		sndAsync     = 0x0001
		sndNoDefault = 0x0002
		sndAlias     = 0x00010000
	)
	procPlaySoundW.Call(
		uintptr(unsafe.Pointer(utf16Ptr("SystemNotification"))),
		0, sndAlias|sndAsync|sndNoDefault)
	if hwnd == 0 {
		return
	}
	const (
		flashwAll       = 0x00000003 // caption + taskbar button
		flashwTimerNoFG = 0x0000000C // keep flashing until foregrounded
	)
	info := struct {
		CbSize    uint32
		Hwnd      uintptr
		DwFlags   uint32
		UCount    uint32
		DwTimeout uint32
	}{Hwnd: hwnd, DwFlags: flashwAll | flashwTimerNoFG}
	info.CbSize = uint32(unsafe.Sizeof(info))
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&info)))
}

// AttentionMainWindow plays the attention sound/flash on the main window
// from Go code (the frontend can also trigger it via the Attention
// binding).
func AttentionMainWindow() {
	attention(mainWindowHwnd.Load())
}
