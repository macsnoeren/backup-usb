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
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestList(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	testutil.WriteFile(t, w.srcDir, "Data/projects/2026/alpha/plan.txt", "plan")
	testutil.WriteFile(t, w.srcDir, "Data/projects/2026/beta/Plan B.txt", "plan b")
	bg := w.keysInit()
	w.mustRun(0, "run")

	out, _ := w.mustRun(0, "list")
	if !strings.Contains(out, "2026-W47") || !strings.Contains(out, "Books, Data") || !strings.Contains(out, "encrypted") {
		t.Fatalf("periods:\n%s", out)
	}
	// Files of an encrypted backup need a key.
	w.mustRun(64, "list", "--tag", "Data", "--identity", "/nonexistent")
	out, _ = w.mustRun(0, "list", "--tag", "Data", "--week", "2026-47", "--identity", bg)
	if !strings.Contains(out, "projects/") || !strings.Contains(out, "report.docx") || strings.Contains(out, "alpha") {
		t.Fatalf("top level:\n%s", out)
	}
	out, _ = w.mustRun(0, "list", "--tag", "Data", "--path", "projects/2026", "--identity", bg)
	if !strings.Contains(out, "projects/2026/alpha/") || !strings.Contains(out, "projects/2026/beta/") || strings.Contains(out, "plan.txt") {
		t.Fatalf("nested:\n%s", out)
	}
	out, _ = w.mustRun(0, "list", "--tag", "Data", "--search", "PLAN", "--json", "--identity", bg)
	var res app.ListResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 2 || !res.FromIndex || res.Entries[0].Path != "projects/2026/alpha/plan.txt" {
		t.Fatalf("search: %+v", res)
	}
	// With the vault password on stdin.
	w.stdin = testPassword + "\n"
	w.mustRun(0, "list", "--tag", "Books", "--password-stdin", "--vault", w.vaultPath())
}
