// Command usb-backup-cli does all the work: backup, verify, browse, restore,
// disks, keys and scheduling.
package main

import (
	"os"

	"github.com/macsnoeren/backup-usb/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cli.Version = version
	os.Exit(cli.Run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}
