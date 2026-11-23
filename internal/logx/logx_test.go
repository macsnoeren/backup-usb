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

package logx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestRotation(t *testing.T) {
	dir := t.TempDir()
	clock := testutil.NewClock(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC))
	l, err := New(Options{Dir: dir, MaxBytes: 200, Keep: 2, Level: LevelInfo, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		l.Info("backup step", "n", i, "disk", "BACKUP 1")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "usb-backup.log*"))
	if len(files) != 3 {
		t.Fatalf("files: %v", files)
	}
	for _, f := range files {
		st, _ := os.Stat(f)
		if st.Size() > 200 {
			t.Errorf("%s is %d bytes", f, st.Size())
		}
	}
	cur, _ := os.ReadFile(filepath.Join(dir, "usb-backup.log"))
	old, _ := os.ReadFile(filepath.Join(dir, "usb-backup.log.2"))
	if !strings.Contains(string(cur), "n=39") || strings.Contains(string(old), "n=39") {
		t.Fatalf("newest line in wrong file:\n%s\n--\n%s", cur, old)
	}
	if !strings.HasPrefix(string(cur), "2026-11-22T10:00:00Z INFO backup step") || !strings.Contains(string(cur), `disk="BACKUP 1"`) {
		t.Fatalf("format: %s", cur)
	}
}

func TestLevelsAndNil(t *testing.T) {
	dir := t.TempDir()
	l, _ := New(Options{Dir: dir, Level: LevelWarn})
	l.Info("hidden")
	l.Debug("hidden")
	l.Warn("shown\ninjected line")
	l.Error("shown too")
	data, _ := os.ReadFile(l.Path())
	s := string(data)
	if strings.Contains(s, "hidden") || strings.Count(s, "\n") != 2 || !strings.Contains(s, `shown\ninjected`) {
		t.Fatalf("log: %q", s)
	}
	var nl *Logger
	nl.Info("nothing") // must not panic
	if ParseLevel("DEBUG") != LevelDebug || ParseLevel("x") != LevelInfo {
		t.Fatal("ParseLevel")
	}
}
