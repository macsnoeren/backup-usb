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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOneSourceFails(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs an unreadable file")
	}
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	old := w.oldWeeks("BACKUP1", 30) // more than keepPerDisk
	w.set("lockedFiles", "fail")
	locked := filepath.Join(w.srcDir, "Books", "2026", "q1.xlsx")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })

	w.mustRun(11, "run")
	week := "Backup/2026/47/"
	if !exists(w.diskFile("BACKUP1", week+"Data.zip")) {
		t.Fatal("the good zip was not kept")
	}
	if exists(w.diskFile("BACKUP1", week+"Books.zip")) || exists(w.diskFile("BACKUP1", week+"Books.zip.part")) {
		t.Fatal("failed zip left behind")
	}
	for _, rel := range old {
		if !exists(w.diskFile("BACKUP1", "Backup/"+rel)) {
			t.Fatalf("%s was rotated after a failed week", rel)
		}
	}
	report, err := os.ReadFile(w.diskFile("BACKUP1", week+"report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "FAILED") || !strings.Contains(string(report), "Old backups were not deleted") {
		t.Fatalf("report:\n%s", report)
	}
	if exists(w.diskFile("BACKUP1", "Backup/last-backup.txt")) {
		t.Fatal("a failed week counts as a backup for the duplicate guard")
	}

	// With the file readable again, the week succeeds and rotation runs.
	_ = os.Chmod(locked, 0o644)
	w.mustRun(0, "run")
	if exists(w.diskFile("BACKUP1", "Backup/"+old[len(old)-1])) {
		t.Fatal("rotation did not run after a good week")
	}
}

func TestLockedFileSkipped(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs an unreadable file")
	}
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	locked := filepath.Join(w.srcDir, "Data", "notes", "todo.txt")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	out, _ := w.mustRun(1, "run")
	if !strings.Contains(out, "notes/todo.txt") || !strings.Contains(out, "OK with warnings") {
		t.Fatalf("summary:\n%s", out)
	}
}
