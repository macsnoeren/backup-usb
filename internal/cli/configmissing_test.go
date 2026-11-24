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

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// A source folder that does not exist (the default C:\Data on a fresh
// installation, or a folder that was moved) must not block other changes:
// the operator could not save anything, not even a new source.
func TestMissingSourceDoesNotBlockSettings(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	for _, d := range []string{"Data", "Books"} {
		if err := os.RemoveAll(filepath.Join(w.srcDir, d)); err != nil {
			t.Fatal(err)
		}
	}
	docs := filepath.Join(w.srcDir, "Docs")
	if err := os.Mkdir(docs, 0o700); err != nil {
		t.Fatal(err)
	}
	do := func(code int, args ...string) {
		t.Helper()
		w.stdin = testPassword + "\n"
		w.mustRun(code, append(args, "--vault", w.vaultPath(), "--password-stdin")...)
	}
	do(0, "config", "set", "language", "nl")
	do(0, "config", "add-source", "--tag", "Docs", "--path", docs)
	do(0, "config", "remove-source", "Data")
	// What is added or changed itself is still checked.
	do(64, "config", "add-source", "--tag", "Gone", "--path", filepath.Join(w.srcDir, "gone"))
	do(64, "config", "set", "sources", `[{"tag":"Gone","path":"`+filepath.ToSlash(filepath.Join(w.srcDir, "gone"))+`"}]`)
	do(64, "config", "set", "keepPerDisk", "-1")
}
