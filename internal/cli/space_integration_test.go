//go:build integration

package cli

import (
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

// fillDisk gives BACKUP1 old week folders of 30 MB each and a capacity that
// leaves too little room for a new backup (100 MB margin).
func fillDisk(t *testing.T, w *world) []string {
	old := w.oldWeeks("BACKUP1", 5)
	for _, rel := range old {
		testutil.WriteFile(t, w.disks.Disk("BACKUP1").Mount, "Backup/"+rel+"/big.bin", strings.Repeat("x", 30<<20))
	}
	w.disks.Update("BACKUP1", func(d *testutil.FakeDisk) { d.Capacity = 5*30<<20 + 2*(1<<20) + 60<<20 })
	return old
}

func TestFreeSpaceStop(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	old := fillDisk(t, w)
	_, errOut := w.mustRun(10, "run")
	if !strings.Contains(errOut, "not enough free space") || !strings.Contains(errOut, "nothing was deleted") {
		t.Fatalf("stderr %q", errOut)
	}
	for _, rel := range old {
		if !exists(w.diskFile("BACKUP1", "Backup/"+rel)) {
			t.Fatalf("%s deleted with freeSpaceAction stop", rel)
		}
	}
}

func TestFreeSpaceDeleteOldest(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	old := fillDisk(t, w) // old[0] is the newest, old[4] the oldest
	w.set("freeSpaceAction", "deleteOldest")
	w.mustRun(0, "run")
	if exists(w.diskFile("BACKUP1", "Backup/"+old[4])) || exists(w.diskFile("BACKUP1", "Backup/"+old[3])) {
		t.Fatal("oldest folders not deleted")
	}
	if !exists(w.diskFile("BACKUP1", "Backup/"+old[0])) || !exists(w.diskFile("BACKUP1", "Backup/"+old[1])) {
		t.Fatal("deleted more than needed")
	}
	if !exists(w.diskFile("BACKUP1", "Backup/2026/47/Data.zip")) {
		t.Fatal("no backup made")
	}
}

func TestFreeSpaceDeleteOldestRespectsMinDays(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	old := fillDisk(t, w)
	w.set("freeSpaceAction", "deleteOldest")
	w.set("keepMinDays", 365)
	w.mustRun(10, "run")
	for _, rel := range old {
		if !exists(w.diskFile("BACKUP1", "Backup/"+rel)) {
			t.Fatalf("%s deleted although younger than keepMinDays", rel)
		}
	}
}
