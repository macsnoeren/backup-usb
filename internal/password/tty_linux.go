package password

import (
	"os"
	"syscall"
	"unsafe"
)

// openTTYNoEcho opens the controlling terminal with echo switched off.
func openTTYNoEcho() (*os.File, func(), error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, ErrNoTTY
	}
	fd := tty.Fd()
	var old syscall.Termios
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&old))); e != 0 { //nolint:gosec // ioctl
		_ = tty.Close()
		return nil, nil, ErrNoTTY
	}
	noEcho := old
	noEcho.Lflag &^= syscall.ECHO
	noEcho.Lflag |= syscall.ICANON | syscall.ISIG
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&noEcho))); e != 0 { //nolint:gosec // ioctl
		_ = tty.Close()
		return nil, nil, ErrNoTTY
	}
	restore := func() {
		_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&old))) //nolint:gosec // ioctl
	}
	return tty, restore, nil
}
