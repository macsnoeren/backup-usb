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
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/verify"
)

func TestRunWeek(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")

	// The first disk starts, also when both disks are connected.
	out, _ := w.mustRun(0, "run")
	if !strings.Contains(out, "BACKUP1") || !strings.Contains(out, "2026-W47") {
		t.Fatalf("summary:\n%s", out)
	}
	for _, f := range []string{"Data.zip", "Data.zip.sha256", "Books.zip", "Books.zip.sha256", "report.txt"} {
		if !exists(w.diskFile("BACKUP1", "Backup/2026/47/"+f)) {
			t.Errorf("%s missing", f)
		}
	}
	for _, f := range []string{"Backup/last-backup.txt", "Backup/keys.json"} {
		if !exists(w.diskFile("BACKUP1", f)) {
			t.Errorf("%s missing", f)
		}
	}
	if exists(w.diskFile("BACKUP2", "Backup/2026")) {
		t.Error("BACKUP2 was written too")
	}
	if _, err := verify.File(t.Context(), w.diskFile("BACKUP1", "Backup/2026/47/Data.zip"), verify.Options{ExpectedFiles: 2}); err != nil {
		t.Fatal(err)
	}

	// A second trigger right after: skipped (4); --force goes ahead.
	w.clock.Advance(2 * time.Minute)
	w.mustRun(4, "run")
	w.mustRun(0, "run", "--force")
	// Within minIntervalHours in the same folder: skipped.
	w.clock.Advance(3 * time.Hour)
	w.mustRun(4, "run")

	// Next week it is the turn of BACKUP2.
	w.clock.Advance(7 * 24 * time.Hour)
	out, _ = w.mustRun(0, "run", "--json")
	var rep app.RunReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Disk != "BACKUP2" || rep.Folder != "2026/48" || rep.Status != app.StatusOK || len(rep.Sources) != 2 {
		t.Fatalf("report %+v", rep)
	}
}

func TestRunWrongAndNoDisk(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	w.unplug("BACKUP1")
	w.mustRun(0, "run") // week 47 on BACKUP2
	w.clock.Advance(7 * 24 * time.Hour)
	_, errOut := w.mustRun(3, "run")
	if !strings.Contains(errOut, "2026-W48 is for BACKUP1, but BACKUP2 is connected") {
		t.Fatalf("stderr %q", errOut)
	}
	if exists(w.diskFile("BACKUP2", "Backup/2026/48")) {
		t.Fatal("wrong disk was written")
	}
	// --any-disk and --target override the turn.
	w.mustRun(0, "run", "--any-disk")
	w.clock.Advance(7 * 24 * time.Hour)
	w.disks.Remove("BACKUP2")
	_, errOut = w.mustRun(2, "run")
	if !strings.Contains(errOut, "insert BACKUP1") {
		t.Fatalf("stderr %q", errOut)
	}
	// --quiet-skip: same code, no message.
	code, out, errOut := w.run("run", "--quiet-skip")
	if code != 2 || out != "" || errOut != "" {
		t.Fatalf("quiet skip: %d %q %q", code, out, errOut)
	}
}

func TestRunTargetAndWait(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	w.mustRun(0, "run", "--target", w.disks.Disk("BACKUP2").Mount)
	if !exists(w.diskFile("BACKUP2", "Backup/2026/47/Data.zip")) {
		t.Fatal("--target ignored")
	}
	// --wait polls; with no disk it gives up after the wait (fake clock).
	w.disks.Remove("BACKUP1")
	w.disks.Remove("BACKUP2")
	start := w.clock.Now()
	w.mustRun(2, "run", "--wait", "5")
	if w.clock.Now().Sub(start) < 5*time.Second {
		t.Fatal("did not wait")
	}
}

func TestRunUnknownDisk(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	// Copy the ID file of BACKUP1 to BACKUP2, then unplug BACKUP1.
	data, err := os.ReadFile(w.diskFile("BACKUP1", "Backup/disk-id.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(w.diskFile("BACKUP2", "Backup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(w.diskFile("BACKUP2", "Backup/disk-id.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	w.disks.Remove("BACKUP1")
	_, errOut := w.mustRun(6, "run")
	if !strings.Contains(errOut, "unknown disk") {
		t.Fatalf("stderr %q", errOut)
	}
	if exists(w.diskFile("BACKUP2", "Backup/2026")) {
		t.Fatal("unknown disk was written")
	}
}

func TestRunMissingSource(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	if err := os.RemoveAll(w.srcDir + "/Books"); err != nil {
		t.Fatal(err)
	}
	_, errOut := w.mustRun(11, "run")
	if !strings.Contains(errOut, "Books") {
		t.Fatalf("stderr %q", errOut)
	}
}
