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
	"time"
)

func TestQuietSkipAndWait(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	w.mustRun(0, "run")
	// 4 (recent) and 3 (wrong disk) are silent with --quiet-skip.
	for _, setup := range []func(){func() {}, func() { w.clock.Advance(time.Hour); w.disks.Remove("BACKUP1") }} {
		setup()
		code, out, errOut := w.run("run", "--quiet-skip")
		if (code != 4 && code != 3) || out != "" || errOut != "" {
			t.Fatalf("quiet skip: %d %q %q", code, out, errOut)
		}
	}
	// Real problems are never silent.
	w.set("encryption", "age")
	code, _, errOut := w.run("run", "--quiet-skip", "--any-disk")
	if code != 10 || !strings.Contains(errOut, "encryption is required") {
		t.Fatalf("real error hidden: %d %q", code, errOut)
	}
	// --wait: the CLI waits 0 seconds by default, the trigger passes
	// waitSeconds (30); a disk that shows up in time is used.
	w.set("encryption", "auto")
	w.disks.Remove("BACKUP2")
	start := w.clock.Now()
	w.mustRun(2, "run")
	if w.clock.Now().Sub(start) > time.Second {
		t.Fatal("waited without --wait")
	}
	if code, _, _ := w.run("run", "--wait", "-1"); code != 64 {
		t.Fatal("negative wait accepted")
	}
}
