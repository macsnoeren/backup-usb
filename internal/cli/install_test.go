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
	"path/filepath"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/task"
	"github.com/macsnoeren/backup-usb/internal/testutil"
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

func (f *fakeTasks) Installed(t task.Trigger) bool {
	for _, s := range f.installed {
		if s.Trigger == t {
			return true
		}
	}
	return false
}

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

// Starting the window at login needs the window and trayIcon, and changes
// nothing about starting on insert.
func TestInstallLogin(t *testing.T) {
	w := newWorld(t)
	ft := &fakeTasks{}
	dir := t.TempDir()
	env := w.env()
	env.Tasks, env.Executable = ft, filepath.Join(dir, "usb-backup-cli")
	if code, _, errOut := run(t, env, "install", "--trigger", "login"); code != 64 || !strings.Contains(errOut, "trayIcon") {
		t.Fatalf("without trayIcon: %d %s", code, errOut)
	}
	w.cfgData["trayIcon"] = true
	w.writeConfig()
	if code, _, errOut := run(t, env, "install", "--trigger", "login"); code != 64 || !strings.Contains(errOut, "usb-backup.exe") {
		t.Fatalf("without the window: %d %s", code, errOut)
	}
	if len(ft.installed) != 0 {
		t.Fatalf("%+v", ft.installed)
	}
	testutil.WriteFile(t, dir, "usb-backup.exe", "MZ")
	if code, _, errOut := run(t, env, "install", "--trigger", "login"); code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if len(ft.installed) != 1 || ft.installed[0].Trigger != task.Login || ft.installed[0].Command != filepath.Join(dir, "usb-backup.exe") || strings.Join(ft.installed[0].Args, " ") != "--tray" {
		t.Fatalf("%+v", ft.installed)
	}
	_, out, _ := run(t, env, "install", "--check", "--json")
	if !strings.Contains(out, `"usb": false`) || !strings.Contains(out, `"login": true`) {
		t.Fatal(out)
	}
	if code, _, _ := run(t, env, "uninstall", "--trigger", "all"); code != 0 || len(ft.removed) != 4 {
		t.Fatalf("%v", ft.removed)
	}
	ft.removed = nil
	if code, _, _ := run(t, env, "uninstall", "--trigger", "login"); code != 0 || len(ft.removed) != 1 || ft.removed[0] != task.Login {
		t.Fatalf("%v", ft.removed)
	}
}
