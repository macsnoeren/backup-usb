//go:build integration

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestStatus(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd", "BACKUP2": "even"})
	bg := w.keysInit()
	w.disks.Update("BACKUP2", func(d *testutil.FakeDisk) { d.Health = "Warning" })
	out, _ := w.mustRun(0, "status")
	for _, want := range []string{"2026-W47: BACKUP1 is due", "BACKUP1", "last backup never (due now)", "health Warning", "never been tested"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	w.mustRun(0, "run")
	w.mustRun(0, "verify", "--identity", bg)
	out, _ = w.mustRun(0, "status", "--json")
	var rep app.StatusReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Disks) != 2 || rep.BreakglassTested == nil || !rep.Encrypted || rep.Disks[0].Folders != 1 || rep.Disks[0].LastBackup == nil || !rep.Disks[0].Due || rep.Disks[1].Health != "Warning" {
		t.Fatalf("%+v", rep)
	}
	out, _ = w.mustRun(0, "status")
	if !strings.Contains(out, "Break-glass key last tested") {
		t.Fatal(out)
	}
}
