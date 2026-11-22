package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func TestExitCodes(t *testing.T) {
	want := map[app.Kind]int{
		app.KindNone: 0, app.KindWarning: 1, app.KindNoDisk: 2, app.KindWrongDisk: 3, app.KindSkipped: 4,
		app.KindBusy: 5, app.KindUnknownDisk: 6, app.KindTampered: 7, app.KindDiskCheck: 10, app.KindIO: 11,
		app.KindVerify: 12, app.KindDecrypt: 13, app.KindUsage: 64,
	}
	for k, code := range want {
		var err error
		if k != app.KindNone {
			err = fmt.Errorf("wrapped: %w", app.Errorf(k, nil, "x"))
		}
		if got := ExitCode(err); got != code {
			t.Errorf("kind %d: exit %d, want %d", k, got, code)
		}
	}
	if ExitCode(errors.New("plain")) != ExitIO {
		t.Fatal("plain error should be an I/O error")
	}
}
