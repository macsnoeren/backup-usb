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
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

// Paths longer than 260 characters must back up and restore (Windows needs
// the \\?\ prefix, which Go adds itself).
func TestLongPaths(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	var parts []string
	for i := 0; i < 8; i++ {
		parts = append(parts, strings.Repeat(string(rune('a'+i)), 40))
	}
	rel := strings.Join(parts, "/") + "/deep file.txt"
	testutil.WriteFile(t, w.srcDir, "Data/"+rel, "deep")
	if len(filepath.Join(w.srcDir, "Data", rel)) <= 260 {
		t.Fatal("path not long enough")
	}
	w.mustRun(0, "run")
	to := t.TempDir()
	w.mustRun(0, "restore", "--tag", "Data", "--file", rel, "--to", to)
	got, err := os.ReadFile(filepath.Join(to, filepath.FromSlash(rel)))
	if err != nil || string(got) != "deep" {
		t.Fatal(err)
	}
}

func TestPermissionsRestored(t *testing.T) {
	if isWindows() {
		t.Skip("permissions are a Unix thing")
	}
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	p := filepath.Join(w.srcDir, "Data", "notes", "todo.txt")
	os.Chmod(p, 0o600)
	w.mustRun(0, "run")
	to := t.TempDir()
	w.mustRun(0, "restore", "--tag", "Data", "--to", to)
	st, err := os.Stat(filepath.Join(to, "notes", "todo.txt"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v %v", st.Mode(), err)
	}
}
