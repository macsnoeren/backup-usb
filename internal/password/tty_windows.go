package password

import (
	"os"
	"syscall"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const enableEchoInput = 0x0004

// openTTYNoEcho opens the console input with echo switched off.
func openTTYNoEcho() (*os.File, func(), error) {
	name, _ := syscall.UTF16PtrFromString("CONIN$")
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, nil, ErrNoTTY
	}
	var mode uint32
	if err := syscall.GetConsoleMode(h, &mode); err != nil {
		_ = syscall.CloseHandle(h)
		return nil, nil, ErrNoTTY
	}
	if r, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(mode&^enableEchoInput)); r == 0 {
		_ = syscall.CloseHandle(h)
		return nil, nil, ErrNoTTY
	}
	f := os.NewFile(uintptr(h), "CONIN$")
	restore := func() { _, _, _ = procSetConsoleMode.Call(uintptr(h), uintptr(mode)) }
	return f, restore, nil
}
