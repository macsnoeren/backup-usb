//go:build windows

// Package gui contains the Windows user interface.
package gui

import (
	"fmt"
	"os"
)

// Version is the program version, set by main.
var Version = "dev"

// Main reports that the GUI is only available on Windows.
func Main(_ []string) int {
	fmt.Fprintln(os.Stderr, "usb-backup: the graphical interface is only available on Windows; use usb-backup-cli")
	return 64
}
