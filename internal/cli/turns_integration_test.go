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
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

const oneWeek = 7 * 24 * time.Hour

// logDir is where the turns are kept ("logs" in the settings folder on
// Windows).
func (w *world) logDir() string { return config.Defaults().LogDir(w.cfgDir) }

// plug leaves only the named disk connected.
func (w *world) plug(name string) {
	w.t.Helper()
	for _, n := range w.names {
		_, gone := w.removed[n]
		switch {
		case n == name && gone:
			w.addBack(n)
			delete(w.removed, n)
		case n != name && !gone:
			w.unplug(n)
		}
	}
}

// backup plugs a disk in and expects a good backup to it.
func (w *world) backup(name string) app.RunReport {
	w.t.Helper()
	w.plug(name)
	out, _ := w.mustRun(0, "run", "--json")
	var rep app.RunReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil || rep.Disk != name {
		w.t.Fatalf("%v %+v", err, rep)
	}
	return rep
}

// refused plugs a disk in and expects exit code 3 with text.
func (w *world) refused(name, text string) {
	w.t.Helper()
	w.plug(name)
	if _, errOut := w.mustRun(3, "run"); !strings.Contains(errOut, text) {
		w.t.Fatalf("stderr %q, want %q", errOut, text)
	}
}

func TestFourDisksInTurn(t *testing.T) {
	w := newWorldWith(t, "BACKUP1", "BACKUP2", "BACKUP3", "BACKUP4")
	w.keysInit()
	for _, n := range []string{"BACKUP1", "BACKUP2", "BACKUP3", "BACKUP4"} {
		w.enroll(n)
	}
	// Nothing is known yet: any disk may start, then the ones that were
	// not used yet.
	if rep := w.backup("BACKUP3"); !reflect.DeepEqual(rep.Next, []string{"BACKUP1", "BACKUP2", "BACKUP4"}) {
		t.Fatal(rep.Next)
	}
	w.clock.Advance(oneWeek)
	w.refused("BACKUP3", "2026-W48 is for BACKUP1 / BACKUP2 / BACKUP4, but BACKUP3 is connected")
	for _, n := range []string{"BACKUP1", "BACKUP2"} {
		w.backup(n)
		w.clock.Advance(oneWeek)
	}
	if rep := w.backup("BACKUP4"); !reflect.DeepEqual(rep.Next, []string{"BACKUP3"}) {
		t.Fatal(rep.Next)
	}
	// Later in the same week: refreshed on the same disk only.
	w.clock.Advance(24 * time.Hour)
	w.refused("BACKUP3", "the backup of 2026-W50 is on BACKUP4 already; next is BACKUP3")
	w.backup("BACKUP4")
	// From now on always the same order; weeks without a backup (also
	// week 53 and the new year) skip no disk.
	for i, n := range []string{"BACKUP3", "BACKUP1", "BACKUP2", "BACKUP4", "BACKUP3", "BACKUP1"} {
		w.clock.Advance(time.Duration(1+i%3) * oneWeek)
		w.backup(n)
	}
	// Forgetting a disk takes it out of the turn.
	w.clock.Advance(oneWeek)
	w.plug("BACKUP4")
	w.stdin = testPassword + "\n"
	w.mustRun(0, "forget", "BACKUP2", "--password-stdin", "--vault", w.vaultPath())
	w.refused("BACKUP3", "is for BACKUP4, but BACKUP3")
	w.backup("BACKUP4")
	// Registered again, it goes first.
	w.clock.Advance(oneWeek)
	w.plug("BACKUP2")
	w.enroll("BACKUP2")
	w.refused("BACKUP3", "is for BACKUP2, but BACKUP3")
	w.backup("BACKUP2")
}

// Every frequency with three disks; "insert" never uses a disk twice in a
// row, and each backup gets its own folder.
func TestFrequencies(t *testing.T) {
	for _, tc := range []struct {
		schedule string
		step     time.Duration
		folders  []string
	}{
		{"insert", 11 * time.Minute, []string{"2026/11-18_1000", "2026/11-18_1011", "2026/11-18_1022", "2026/11-18_1033"}},
		{"day", 24 * time.Hour, []string{"2026/11-18", "2026/11-19", "2026/11-20", "2026/11-21"}},
		{"isoWeek", oneWeek, []string{"2026/47", "2026/48", "2026/49", "2026/50"}},
		{"month", 30 * 24 * time.Hour, []string{"2026-11", "2026-12", "2027-01", "2027-02"}},
	} {
		t.Run(tc.schedule, func(t *testing.T) {
			w := newWorldWith(t, "BACKUP1", "BACKUP2", "BACKUP3")
			w.enrollUnsigned("BACKUP1", "BACKUP2", "BACKUP3")
			out, _ := w.mustRun(0, "config", "set", "schedule", tc.schedule)
			if strings.Contains(out, "folderPattern") != (tc.schedule != "isoWeek") {
				t.Fatal(out)
			}
			for i, n := range []string{"BACKUP1", "BACKUP2", "BACKUP3", "BACKUP1"} {
				if i == 2 {
					w.refused("BACKUP1", "BACKUP")
				}
				rep := w.backup(n)
				if rep.Folder != tc.folders[i] || !exists(w.diskFile(n, "Backup/"+rep.Folder+"/Data.zip")) {
					t.Fatalf("backup %d: %s", i, rep.Folder)
				}
				// The same disk again: the turn is over, or it was just done.
				w.clock.Advance(time.Minute)
				if code, _, _ := w.run("run"); code != 3 && code != 4 {
					t.Fatalf("same disk again: exit %d", code)
				}
				w.clock.Advance(tc.step - time.Minute)
			}
		})
	}
}

func TestInsertWithOneDisk(t *testing.T) {
	w := newWorldWith(t, "BACKUP1")
	w.enrollUnsigned("BACKUP1")
	w.mustRun(0, "config", "set", "schedule", "insert")
	w.mustRun(0, "config", "set", "keepPerDisk", "2")
	for i := 0; i < 4; i++ {
		w.backup("BACKUP1")
		w.clock.Advance(11 * time.Minute)
	}
	out, _ := w.mustRun(0, "list")
	if strings.Contains(out, "T10:00") || strings.Contains(out, "T10:11") || !strings.Contains(out, "2026-11-18T10:22") || !strings.Contains(out, "2026-11-18T10:33") {
		t.Fatal(out)
	}
	w.mustRun(0, "verify", "--week", "2026-11-18T10:22")
}

// Backups made before a change of frequency can still be listed and
// restored, and count for keepPerDisk.
func TestChangeOfFrequency(t *testing.T) {
	w := newWorldWith(t, "BACKUP1")
	w.enrollUnsigned("BACKUP1")
	w.backup("BACKUP1") // 2026/47
	w.clock.Advance(oneWeek)
	w.backup("BACKUP1") // 2026/48
	w.mustRun(0, "config", "set", "schedule", "month")
	w.clock.Advance(24 * time.Hour)
	testutil.WriteFile(t, w.srcDir, "Data/new.txt", "newest")
	w.backup("BACKUP1") // 2026-11, newer than week 48
	out, _ := w.mustRun(0, "list")
	for _, want := range []string{"2026-W47", "2026-W48", "2026-11"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%s missing:\n%s", want, out)
		}
	}
	to := t.TempDir()
	w.mustRun(0, "restore", "--tag", "Data", "--to", filepath.Join(to, "newest"))
	if !exists(filepath.Join(to, "newest", "new.txt")) && !exists(filepath.Join(to, "newest", "Data", "new.txt")) {
		t.Fatal("the newest backup was not used")
	}
	w.mustRun(0, "restore", "--tag", "Data", "--week", "2026-W47", "--to", filepath.Join(to, "old"))
	w.mustRun(0, "verify", "--all")
	out, _ = w.mustRun(0, "status")
	if !strings.Contains(out, "backups: 3") {
		t.Fatal(out)
	}
	w.mustRun(0, "config", "set", "keepPerDisk", "2")
	w.clock.Advance(10 * 24 * time.Hour)
	w.backup("BACKUP1") // 2026-12
	for rel, want := range map[string]bool{"2026/47": false, "2026/48": false, "2026-11": true, "2026-12": true} {
		if exists(w.diskFile("BACKUP1", "Backup/"+rel)) != want {
			t.Errorf("%s: want %v", rel, want)
		}
	}
}

// A pattern whose folders look like those of older backups is refused
// before anything is written.
func TestPatternClash(t *testing.T) {
	w := newWorldWith(t, "BACKUP1")
	w.enrollUnsigned("BACKUP1")
	w.backup("BACKUP1")
	w.clock.Advance(oneWeek)
	w.set("schedule", "month")
	w.set("folderPattern", `{year}\{month}`)
	before := testutil.Tree(t, w.disks.Disk("BACKUP1").Mount)
	for _, args := range [][]string{{"run"}, {"run", "--dry-run"}} {
		if _, errOut := w.mustRun(64, args...); !strings.Contains(errOut, "mixed up") {
			t.Fatal(errOut)
		}
	}
	if !reflect.DeepEqual(before, testutil.Tree(t, w.disks.Disk("BACKUP1").Mount)) {
		t.Fatal("the disk was changed")
	}
}

// Settings and registrations of the odd and even weeks still work.
func TestOldOddEvenSettings(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	w.enroll("BACKUP1")
	w.enroll("BACKUP2")
	w.set("rotation", "oddEven")
	p := filepath.Join(w.cfgDir, config.DefaultDisksFile)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	old := strings.Replace(strings.Replace(string(data), `"name": "BACKUP1",`, `"name": "BACKUP1", "weeks": "even",`, 1), `"name": "BACKUP2",`, `"name": "BACKUP2", "weeks": "even",`, 1)
	testutil.WriteFile(t, w.cfgDir, config.DefaultDisksFile, old)
	w.resign()
	w.backup("BACKUP1")
	w.clock.Advance(oneWeek)
	w.refused("BACKUP1", "is for BACKUP2")
	w.backup("BACKUP2")
	if out, _ := w.mustRun(0, "config", "show", "rotation"); !strings.Contains(out, "inTurn") {
		t.Fatal(out)
	}
	w.stdin = testPassword + "\n"
	w.mustRun(0, "forget", "BACKUP1", "--password-stdin", "--vault", w.vaultPath())
	if data, _ = os.ReadFile(p); strings.Contains(string(data), "weeks") {
		t.Fatal(string(data))
	}
	// They cannot be chosen any more.
	w.mustRun(64, "config", "set", "rotation", "oddEven", "--password-stdin", "--vault", w.vaultPath())
}

// What does not use up the turn: dry runs, failed runs and runs of one
// source.
func TestTurnNotUsedUp(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	turns := filepath.Join(w.logDir(), app.TurnsFile)
	w.plug("BACKUP1")
	w.mustRun(0, "run", "--dry-run")
	w.mustRun(0, "run", "--tag", "Books")
	w.clock.Advance(time.Hour)
	if err := os.Rename(filepath.Join(w.srcDir, "Books"), filepath.Join(w.srcDir, "Gone")); err != nil {
		t.Fatal(err)
	}
	w.mustRun(11, "run", "--force")
	if exists(turns) {
		t.Fatal("the turn was used up")
	}
	if err := os.Rename(filepath.Join(w.srcDir, "Gone"), filepath.Join(w.srcDir, "Books")); err != nil {
		t.Fatal(err)
	}
	w.mustRun(0, "run", "--force")
	if !exists(turns) {
		t.Fatal("the turn was not saved")
	}
}

// A damaged or missing turns file never costs a backup.
func TestDamagedTurns(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	w.backup("BACKUP1")
	for _, bad := range []string{"{", `{"version":1,"disks":{},"x":1}`, "", strings.Repeat("x", 300<<10), `{"version":1,"disks":null}`} {
		w.clock.Advance(oneWeek)
		testutil.WriteFile(t, w.logDir(), app.TurnsFile, bad)
		w.backup("BACKUP1")
	}
	// A planted huge number does not overflow.
	w.clock.Advance(oneWeek)
	data, err := os.ReadFile(filepath.Join(w.logDir(), app.TurnsFile))
	if err != nil {
		t.Fatal(err)
	}
	testutil.WriteFile(t, w.logDir(), app.TurnsFile, strings.Replace(string(data), `"seq": 1`, `"seq": 18446744073709551615`, 1))
	w.backup("BACKUP2")
	w.clock.Advance(oneWeek)
	w.backup("BACKUP1")
	// It cannot be saved: the backup is fine, with a warning.
	w.clock.Advance(oneWeek)
	if err := os.Remove(filepath.Join(w.logDir(), app.TurnsFile)); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFile(t, w.logDir(), app.TurnsFile+"/x", "a folder in the way")
	w.plug("BACKUP2")
	out, _ := w.mustRun(1, "run")
	if !strings.Contains(out, "whose turn it is could not be saved") {
		t.Fatal(out)
	}
}

func TestDisksShowsTurn(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	w.backup("BACKUP1")
	w.clock.Advance(oneWeek)
	out, _ := w.mustRun(0, "disks")
	if !strings.Contains(out, "last backup 2026-11-18 10:00") || !strings.Contains(out, "last backup never (due now)") {
		t.Fatal(out)
	}
}
