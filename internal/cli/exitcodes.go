package cli

import "github.com/macsnoeren/backup-usb/internal/app"

// Exit codes, as in the table in the design. Only this package knows them.
const (
	ExitOK          = 0
	ExitWarning     = 1
	ExitNoDisk      = 2
	ExitWrongDisk   = 3
	ExitSkipped     = 4
	ExitBusy        = 5
	ExitUnknownDisk = 6
	ExitTampered    = 7
	ExitDiskCheck   = 10
	ExitIO          = 11
	ExitVerify      = 12
	ExitDecrypt     = 13
	ExitUsage       = 64
)

var kindToExit = map[app.Kind]int{
	app.KindNone:        ExitOK,
	app.KindWarning:     ExitWarning,
	app.KindNoDisk:      ExitNoDisk,
	app.KindWrongDisk:   ExitWrongDisk,
	app.KindSkipped:     ExitSkipped,
	app.KindBusy:        ExitBusy,
	app.KindUnknownDisk: ExitUnknownDisk,
	app.KindTampered:    ExitTampered,
	app.KindDiskCheck:   ExitDiskCheck,
	app.KindIO:          ExitIO,
	app.KindVerify:      ExitVerify,
	app.KindDecrypt:     ExitDecrypt,
	app.KindUsage:       ExitUsage,
}

// ExitCode returns the exit code for err.
func ExitCode(err error) int {
	if c, ok := kindToExit[app.KindOf(err)]; ok {
		return c
	}
	return ExitIO
}
