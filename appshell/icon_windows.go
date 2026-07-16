//go:build windows

package appshell

import (
	"os"
	"syscall"
	"unsafe"
)

// applyCorners sets the Win11 corner preference ("" leaves the system
// default). Harmless no-op on Win10 (the attribute is unknown there).
func applyCorners(hwnd uintptr, corners string) {
	var pref int32
	switch corners {
	case "":
		return
	case "round":
		pref = dwmwcpRound
	case "small":
		pref = dwmwcpRoundSm
	case "square":
		pref = dwmwcpSquare
	default:
		pref = dwmwcpDefault
	}
	procDwmSetWindowAttribute.Call(hwnd, dwmwaCornerPref,
		uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
}

// applyIcon stamps an icon onto a window (taskbar, alt-tab). Release
// builds usually carry a multi-size icon resource in the exe (e.g. via
// the rsrc tool) - extracting that gives crisp small/big variants; dev
// builds without the resource fall back to the caller-supplied runtime
// PNG (see the appicon package).
func applyIcon(hwnd uintptr, src IconSource) {
	if !src.SkipExe {
		if exe, err := os.Executable(); err == nil {
			if p, err := syscall.UTF16PtrFromString(exe); err == nil {
				var big, small uintptr
				n, _, _ := procExtractIconExW.Call(
					uintptr(unsafe.Pointer(p)), 0,
					uintptr(unsafe.Pointer(&big)), uintptr(unsafe.Pointer(&small)), 1,
				)
				if n != 0 && n != ^uintptr(0) && (big != 0 || small != 0) {
					if small != 0 {
						procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, small)
					}
					if big != 0 {
						procSendMessageW.Call(hwnd, wmSetIcon, iconBig, big)
					}
					return
				}
			}
		}
	}
	if len(src.PNG) == 0 {
		return
	}
	hicon, _, _ := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&src.PNG[0])), uintptr(len(src.PNG)),
		1, 0x00030000, 0, 0, 0,
	)
	if hicon != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, hicon)
		procSendMessageW.Call(hwnd, wmSetIcon, iconBig, hicon)
	}
}
