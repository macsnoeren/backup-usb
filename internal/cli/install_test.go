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
	"errors"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/task"
)

type fakeTasks struct {
	installed []task.Spec
	removed   []task.Trigger
	fail      bool
}

func (f *fakeTasks) Install(s task.Spec) error {
	if f.fail {
		return errors.New("access denied")
	}
	f.installed = append(f.installed, s)
	return nil
}

func (f *fakeTasks) Uninstall(t task.Trigger) error {
	f.removed = append(f.removed, t)
	return nil
}

func (f *fakeTasks) Installed(task.Trigger) bool { return len(f.installed) > 0 }

func TestInstall(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	ft := &fakeTasks{}
	env := w.env()
	env.Tasks, env.Executable = ft, "/usr/bin/usb-backup-cli"
	if _, out, _ := run(t, env, "install", "--check", "--json"); !strings.Contains(out, `"usb": false`) {
		t.Fatal(out)
	}
	run(t, env, "install")
	if _, out, _ := run(t, env, "install", "--check", "--json"); !strings.Contains(out, `"usb": true`) {
		t.Fatal(out)
	}
	// Backups at a time of day are gone.
	for _, args := range [][]string{{"install", "--trigger", "daily"}, {"install", "--trigger", "weekly"}, {"install", "--at", "02:00"}} {
		if code, _, _ := run(t, env, args...); code != 64 {
			t.Fatalf("%v accepted", args)
		}
	}
	if len(ft.installed) != 1 {
		t.Fatalf("%+v", ft.installed)
	}
	u := ft.installed[0]
	if u.Trigger != task.USB || strings.Join(u.Args, " ") != "run --quiet-skip --wait 30" || len(u.VolumeUUIDs) != 1 || u.VolumeUUIDs[0] != w.disks.Disk("BACKUP1").VolumeSerial {
		t.Fatalf("usb %+v", u)
	}
	if code, out, _ := run(t, env, "uninstall", "--trigger", "all"); code != 0 || len(ft.removed) != 3 || !strings.Contains(out, "removed") {
		t.Fatalf("uninstall %d %v", code, ft.removed)
	}
	ft.fail = true
	if code, _, _ := run(t, env, "install"); code != 11 {
		t.Fatal("failure not reported")
	}
}
