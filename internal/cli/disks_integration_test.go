//go:build integration

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func TestDisksAndForget(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	w.enroll("BACKUP1", "odd")
	w.enroll("BACKUP2", "even")
	w.disks.Remove("BACKUP2")
	out, _ := w.mustRun(0, "disks")
	for _, want := range []string{"Registered disks:", "BACKUP1", "BACKUP2", "not connected", "registered as BACKUP1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	out, _ = w.mustRun(0, "disks", "--json")
	var rep app.DisksReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Registered) != 2 || !rep.Registered[0].Connected || rep.Registered[1].Connected || len(rep.Connected) != 1 || rep.Connected[0].State != app.DiskRegistered {
		t.Fatalf("%+v", rep)
	}

	w.stdin = "wrong password..\n"
	w.mustRun(13, "forget", "BACKUP1", "--password-stdin", "--vault", w.vaultPath())
	w.stdin = testPassword + "\n"
	w.mustRun(64, "forget", "NOPE", "--password-stdin", "--vault", w.vaultPath())
	w.stdin = testPassword + "\n"
	w.mustRun(0, "forget", "BACKUP1", "--password-stdin", "--vault", w.vaultPath())
	out, _ = w.mustRun(0, "disks", "--json")
	rep = app.DisksReport{}
	json.Unmarshal([]byte(out), &rep)
	if len(rep.Registered) != 1 || rep.Connected[0].State != app.DiskUnknown {
		t.Fatalf("after forget: %+v", rep)
	}
	// The forgotten disk is no longer used.
	w.mustRun(6, "run")
}
