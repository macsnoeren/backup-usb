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
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func TestRestoreOriginalNeverOverwrites(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	w.mustRun(0, "run")
	todo := filepath.Join(w.srcDir, "Data", "notes", "todo.txt")
	os.WriteFile(todo, []byte("changed since the backup"), 0o644)
	os.Remove(filepath.Join(w.srcDir, "Data", "report.docx"))
	out, _ := w.mustRun(0, "restore", "--tag", "Data", "--original", "--json")
	var rep app.RestoreReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(todo); string(got) != "changed since the backup" {
		t.Fatal("existing file overwritten")
	}
	restored := filepath.Join(w.srcDir, "Data", "notes", "todo (restored 2026-11-18).txt")
	if got, _ := os.ReadFile(restored); string(got) != "buy milk" {
		t.Fatal("renamed copy missing")
	}
	if _, err := os.Stat(filepath.Join(w.srcDir, "Data", "report.docx")); err != nil {
		t.Fatal("missing file not restored to its place")
	}
	// Every file that still exists is restored under a new name (never
	// overwritten); report.docx was gone and is back under its own name.
	for _, r := range rep.Renamed {
		if r.Path == "report.docx" {
			t.Fatal("report.docx renamed although its place was free")
		}
	}
	// A second restore picks the next free name.
	w.mustRun(0, "restore", "--tag", "Data", "--original", "--file", "notes/todo.txt")
	if _, err := os.Stat(filepath.Join(w.srcDir, "Data", "notes", "todo (restored 2026-11-18 2).txt")); err != nil {
		t.Fatal("second copy name")
	}
}
