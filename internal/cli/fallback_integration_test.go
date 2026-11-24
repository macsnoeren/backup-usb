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
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestRestoreWithoutIndex(t *testing.T) {
	for _, damage := range []string{"remove", "corrupt"} {
		w := newWorld(t)
		w.enrollUnsigned("BACKUP1")
		bg := w.keysInit()
		w.mustRun(0, "run")
		ix := w.diskFile("BACKUP1", "Backup/2026/47/Data.index.age")
		if damage == "remove" {
			os.Remove(ix)
		} else {
			data, _ := os.ReadFile(ix)
			data[len(data)-10] ^= 1
			os.WriteFile(ix, data, 0o600)
		}
		to := t.TempDir()
		_, errOut := w.mustRun(0, "restore", "--tag", "Data", "--to", to, "--identity", bg)
		if !strings.Contains(errOut, "Index missing or damaged") {
			t.Fatalf("%s: no warning: %q", damage, errOut)
		}
		tree := testutil.Tree(t, to)
		if tree["notes/todo.txt"] != "buy milk" || tree["report.docx"] == "" {
			t.Fatalf("%s: tree %v", damage, tree)
		}
		out, errOut := w.mustRun(0, "list", "--tag", "Data", "--search", "todo", "--identity", bg)
		if !strings.Contains(out, "notes/todo.txt") || !strings.Contains(errOut, "Index missing") {
			t.Fatalf("%s: list %q %q", damage, out, errOut)
		}
	}
}

// A plaintext index planted next to an encrypted archive is never used.
func TestPlantedPlaintextIndexIgnored(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	bg := w.keysInit()
	w.mustRun(0, "run")
	os.Remove(w.diskFile("BACKUP1", "Backup/2026/47/Data.index.age"))
	forged := &archive.Index{Version: 1, Tag: "Data", Entries: []archive.Entry{{Path: "evil.txt", Size: 4, DataOffset: 0, CompressedSize: 4}}}
	f, _ := os.Create(w.diskFile("BACKUP1", "Backup/2026/47/Data.index.gz"))
	forged.Encode(f)
	f.Close()
	out, errOut := w.mustRun(0, "list", "--tag", "Data", "--identity", bg)
	if strings.Contains(out, "evil.txt") || !strings.Contains(errOut, "Index missing") {
		t.Fatalf("planted index used:\n%s\n%s", out, errOut)
	}
}
