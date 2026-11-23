// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

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

// When deleting every allowed folder still would not free enough space,
// nothing is deleted.
func TestFreeSpaceDeleteOldestRefusesHopelessCase(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	old := w.oldWeeks("BACKUP1", 3)
	for _, rel := range old {
		testutil.WriteFile(t, w.disks.Disk("BACKUP1").Mount, "Backup/"+rel+"/big.bin", strings.Repeat("x", 10<<20))
	}
	// 2 MB free; deleting the two allowed folders frees 20 MB, far below
	// the 100 MB margin.
	w.disks.Update("BACKUP1", func(d *testutil.FakeDisk) { d.Capacity = 3*10<<20 + 3<<20 })
	w.set("freeSpaceAction", "deleteOldest")
	w.mustRun(10, "run")
	for _, rel := range old {
		if !exists(w.diskFile("BACKUP1", "Backup/"+rel)) {
			t.Fatalf("%s deleted although it could not help", rel)
		}
	}
}
