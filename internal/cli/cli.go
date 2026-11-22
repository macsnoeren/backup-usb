// Package cli implements the usb-backup-cli commands, flags and exit codes.
package cli

import (
	"fmt"
	"io"
)

// Version is the program version, set by main.
var Version = "dev"

// Run executes the command line in args (args[0] is the program name) and
// returns the process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = stdin
	if len(args) < 2 {
		fmt.Fprintln(stdout, "usb-backup-cli", Version)
		return 0
	}
	fmt.Fprintf(stderr, "unknown command %q\n", args[1])
	return 64
}
