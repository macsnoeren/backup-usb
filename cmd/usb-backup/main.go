// Command usb-backup is the Windows GUI. It runs usb-backup-cli as a child
// process and shows its progress.
package main

import (
	"os"

	"github.com/macsnoeren/backup-usb/internal/gui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	gui.Version = version
	os.Exit(gui.Main(os.Args))
}
