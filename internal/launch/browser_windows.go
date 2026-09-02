//go:build windows

package launch

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// OpenInBrowser opens url in the OS default browser (the --browser mode
// and the popup fallback path). Native ShellExecuteW: no intermediate
// shell, so query strings with & survive verbatim (cmd truncated them;
// rundll32 proved unreliable on some machines).
func OpenInBrowser(url string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := syscall.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	const swShowNormal = 1
	ret, _, _ := procShellExecuteW.Call(
		0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, swShowNormal,
	)
	if ret <= 32 { // per ShellExecute docs, <=32 means failure
		return fmt.Errorf("launch: opening browser failed (ShellExecute code %d)", ret)
	}
	return nil
}
