//go:build windows

package appshell

import (
	"sync/atomic"
	"syscall"
	"unsafe"
)

// subclassConfig drives the main window's WndProc subclass. One subclass
// handles the frameless chrome (frame removal, edge resize, drag band),
// size limits, and the close hook, so only one permanent
// syscall.NewCallback is ever allocated per window role.
type subclassConfig struct {
	frameless bool

	captionHeight       int
	captionLeftReserve  int
	captionRightReserve int
	resizeMargin        int

	minWidth, minHeight int
	maxWidth, maxHeight int

	disableClose bool
	// onCloseRequest is WindowOptions.OnCloseRequest; nil = allow.
	onCloseRequest func() CloseAction
	// forceClose is set by CloseMainWindow to bypass the hook.
	forceClose *atomic.Bool
	// onClosing runs (once) when a close is actually proceeding - the
	// geometry snapshot hook. Also runs on CloseHide, so a hidden
	// window's position survives an app exit.
	onClosing func()
}

// subclassWindow installs the WndProc subclass.
//
//   - WM_NCCALCSIZE (frameless): client area = whole window, killing the
//     white native frame strip.
//   - WM_NCHITTEST (frameless): re-implements 8-direction edge resizing
//     plus a draggable top caption band, so the window behaves exactly
//     like it still had a title bar.
//   - WM_GETMINMAXINFO: enforces min/max sizes while resizing and clamps
//     a maximized window to the monitor WORK AREA - a frameless window
//     otherwise maximizes over the taskbar.
//   - WM_CLOSE: runs the close hook (allow/cancel/hide) and the geometry
//     snapshot.
func subclassWindow(hwnd uintptr, cfg subclassConfig) {
	var origProc uintptr
	closed := false
	proc := syscall.NewCallback(func(h, msg, wparam, lparam uintptr) uintptr {
		switch msg {
		case wmNCCalcSize:
			if cfg.frameless && wparam != 0 {
				return 0 // client area = whole window: no native frame
			}

		case wmNCHitTest:
			if !cfg.frameless {
				break
			}
			x := int32(int16(lparam & 0xFFFF))
			y := int32(int16((lparam >> 16) & 0xFFFF))
			var r winRect
			procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&r)))
			zoomed, _, _ := procIsZoomed.Call(h)
			if zoomed == 0 {
				m := int32(cfg.resizeMargin)
				left := x < r.Left+m
				right := x >= r.Right-m
				top := y < r.Top+m
				bottom := y >= r.Bottom-m
				switch {
				case top && left:
					return 13 // HTTOPLEFT
				case top && right:
					return 14 // HTTOPRIGHT
				case bottom && left:
					return 16 // HTBOTTOMLEFT
				case bottom && right:
					return 17 // HTBOTTOMRIGHT
				case left:
					return 10 // HTLEFT
				case right:
					return 11 // HTRIGHT
				case top:
					return 12 // HTTOP
				case bottom:
					return 15 // HTBOTTOM
				}
			}
			// Empty middle of the top band drags the window. The
			// reserves keep the frontend's buttons clickable; the
			// centered title is pointer-transparent so it drags too.
			if y < r.Top+int32(cfg.captionHeight) &&
				x > r.Left+int32(cfg.captionLeftReserve) &&
				x < r.Right-int32(cfg.captionRightReserve) {
				return htCaption
			}
			return htClient

		case wmGetMinMaxInfo:
			// lparam IS a *MINMAXINFO for this message; reinterpret via
			// &lparam so go vet does not flag a uintptr->pointer cast
			// (the pointee is owned by the OS for the call's duration).
			mmi := *(**minMaxInfo)(unsafe.Pointer(&lparam))
			// Clamp maximize to the work area first (the default fills
			// the whole monitor, covering the taskbar, because a
			// frameless window has no frame overhang to hide behind).
			hmon, _, _ := procMonitorFromWindow.Call(h, monitorDefaultToNearest)
			if hmon != 0 {
				var mi monitorInfo
				mi.CbSize = uint32(unsafe.Sizeof(mi))
				if ok, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); ok != 0 {
					mmi.PtMaxPosition = winPoint{
						X: mi.RcWork.Left - mi.RcMonitor.Left,
						Y: mi.RcWork.Top - mi.RcMonitor.Top,
					}
					mmi.PtMaxSize = winPoint{
						X: mi.RcWork.Right - mi.RcWork.Left,
						Y: mi.RcWork.Bottom - mi.RcWork.Top,
					}
				}
			}
			if cfg.minWidth > 0 {
				mmi.PtMinTrackSize.X = int32(cfg.minWidth)
			}
			if cfg.minHeight > 0 {
				mmi.PtMinTrackSize.Y = int32(cfg.minHeight)
			}
			if cfg.maxWidth > 0 {
				mmi.PtMaxTrackSize.X = int32(cfg.maxWidth)
			}
			if cfg.maxHeight > 0 {
				mmi.PtMaxTrackSize.Y = int32(cfg.maxHeight)
			}
			return 0

		case wmClose:
			forced := cfg.forceClose != nil && cfg.forceClose.Load()
			if !forced {
				if cfg.disableClose {
					return 0
				}
				action := CloseAllow
				if cfg.onCloseRequest != nil {
					action = cfg.onCloseRequest()
				}
				switch action {
				case CloseCancel:
					return 0
				case CloseHide:
					if cfg.onClosing != nil && !closed {
						closed = true
						cfg.onClosing()
					}
					closed = false // geometry saved; allow a later real close to save again
					procShowWindow.Call(h, swHide)
					return 0
				}
			}
			if cfg.onClosing != nil && !closed {
				closed = true
				cfg.onClosing()
			}
			// fall through to the original proc, which destroys the window
		}
		ret, _, _ := procCallWindowProcW.Call(origProc, h, msg, wparam, lparam)
		return ret
	})
	origProc, _, _ = procSetWindowLongPtrW.Call(hwnd, gwlpWndProc, proc)
	_ = origProc
}
