package secmem

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                      = syscall.NewLazyDLL("kernel32.dll")
	procVirtualLock               = kernel32.NewProc("VirtualLock")
	procVirtualUnlock             = kernel32.NewProc("VirtualUnlock")
	procSetErrorMode              = kernel32.NewProc("SetErrorMode")
	wer                           = syscall.NewLazyDLL("wer.dll")
	procWerAddExcludedApplication = wer.NewProc("WerAddExcludedApplication")
)

func lock(b []byte) error {
	r, _, err := procVirtualLock.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b))) //nolint:gosec // Win32 call
	if r == 0 {
		return err
	}
	return nil
}

func unlock(b []byte) error {
	r, _, err := procVirtualUnlock.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b))) //nolint:gosec // Win32 call
	if r == 0 {
		return err
	}
	return nil
}

const (
	semFailCriticalErrors = 0x0001
	semNoGPFaultErrorBox  = 0x0002
)

// ProtectProcess keeps secrets out of crash dumps: no Windows Error
// Reporting for this program (WerAddExcludedApplication) and no fault
// dialogs (SetErrorMode).
func ProtectProcess() error {
	_, _, _ = procSetErrorMode.Call(semFailCriticalErrors | semNoGPFaultErrorBox) // returns the old mode
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	name, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	if err := procWerAddExcludedApplication.Find(); err != nil {
		return err
	}
	hr, _, _ := procWerAddExcludedApplication.Call(uintptr(unsafe.Pointer(name)), 0) //nolint:gosec // Win32 call
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

func dumpable() (bool, error) { return false, nil }
