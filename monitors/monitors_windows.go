//go:build windows

package monitors

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	enumMu                  sync.Mutex
	enumResult              []Monitor
	monitorEnumCallbackOnce sync.Once
	monitorEnumCallbackAddr uintptr
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfoEx struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
	SzDevice  [32]uint16
}

const monitorinfofPrimary = 1

// All enumerates displays via EnumDisplayMonitors.
func All() []Monitor {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumResult = nil

	// syscall.NewCallback allocations are permanent, so the callback is
	// created exactly once and reused for every enumeration.
	monitorEnumCallbackOnce.Do(func() {
		monitorEnumCallbackAddr = syscall.NewCallback(func(hMonitor, hdc, lprc, lparam uintptr) uintptr {
			var info monitorInfoEx
			info.CbSize = uint32(unsafe.Sizeof(info))
			ret, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&info)))
			if ret != 0 {
				m := Monitor{
					Index:      len(enumResult),
					Name:       syscall.UTF16ToString(info.SzDevice[:]),
					X:          int(info.RcMonitor.Left),
					Y:          int(info.RcMonitor.Top),
					Width:      int(info.RcMonitor.Right - info.RcMonitor.Left),
					Height:     int(info.RcMonitor.Bottom - info.RcMonitor.Top),
					Primary:    info.DwFlags&monitorinfofPrimary != 0,
					WorkX:      int(info.RcWork.Left),
					WorkY:      int(info.RcWork.Top),
					WorkWidth:  int(info.RcWork.Right - info.RcWork.Left),
					WorkHeight: int(info.RcWork.Bottom - info.RcWork.Top),
				}
				if m.Name == "" {
					m.Name = fmt.Sprintf("Display %d", m.Index+1)
				}
				enumResult = append(enumResult, m)
			}
			return 1 // continue enumeration
		})
	})

	procEnumDisplayMonitors.Call(0, 0, monitorEnumCallbackAddr, 0)
	return append([]Monitor(nil), enumResult...)
}
