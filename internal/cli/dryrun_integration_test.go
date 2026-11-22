//go:build integration

package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestDryRunWritesNothing(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd", "BACKUP2": "even"})
	w.set("keepPerDisk", 3)
	w.oldWeeks("BACKUP1", 4)
	testutil.WriteFile(t, w.disks.Disk("BACKUP1").Mount, "Backup/2026/46/Data.zip.part", "left over")
	before := testutil.Tree(t, w.disks.Disk("BACKUP1").Mount)
	before2 := testutil.Tree(t, w.disks.Disk("BACKUP2").Mount)

	out, _ := w.mustRun(0, "run", "--dry-run")
	for _, want := range []string{"BACKUP1", "2026/47", "Rotation would delete: 2026/43, 2026/44", "nothing was written"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	out, _ = w.mustRun(0, "run", "--dry-run", "--json")
	var rep app.RunReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Status != app.StatusDryRun || rep.DryRun == nil || len(rep.DryRun.WouldRotate) != 2 || rep.DryRun.RequiredBytes == 0 {
		t.Fatalf("dry run report %+v", rep.DryRun)
	}
	if after := testutil.Tree(t, w.disks.Disk("BACKUP1").Mount); !reflect.DeepEqual(before, after) {
		t.Fatalf("BACKUP1 changed by a dry run")
	}
	if after := testutil.Tree(t, w.disks.Disk("BACKUP2").Mount); !reflect.DeepEqual(before2, after) {
		t.Fatalf("BACKUP2 changed by a dry run")
	}

	// A brand-new disk without a backup folder stays untouched too.
	w.disks.Remove("BACKUP1")
	w.clock.Advance(7 * 24 * 3600e9)
	before2 = testutil.Tree(t, w.disks.Disk("BACKUP2").Mount)
	w.mustRun(0, "run", "--dry-run")
	if after := testutil.Tree(t, w.disks.Disk("BACKUP2").Mount); !reflect.DeepEqual(before2, after) {
		t.Fatalf("BACKUP2 changed by a dry run")
	}
}
