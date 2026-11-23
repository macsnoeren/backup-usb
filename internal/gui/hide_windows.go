package gui

import (
	"os/exec"
	"syscall"
)

// hideWindow starts the CLI without a console window.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
