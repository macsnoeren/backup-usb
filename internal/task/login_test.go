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

package task

import "testing"

func TestLoginCommand(t *testing.T) {
	for _, tc := range []struct {
		spec Spec
		want string
	}{
		{Spec{Trigger: Login, Command: exe, Args: []string{"--tray"}}, `"` + exe + `" --tray`},
		{Spec{Trigger: Login, Command: `C:\ub\usb-backup.exe`}, `"C:\ub\usb-backup.exe"`},
		{Spec{Trigger: USB, Command: exe}, ""},
		{Spec{Trigger: Login}, ""},
		{Spec{Trigger: Login, Command: `C:\a" b.exe`}, ""},
		{Spec{Trigger: Login, Command: "C:\\a\n.exe"}, ""},
	} {
		got, err := loginCommand(tc.spec)
		if got != tc.want || (err == nil) != (tc.want != "") {
			t.Errorf("%+v: %q %v", tc.spec, got, err)
		}
	}
}

func TestLoginOnlyOnWindows(t *testing.T) {
	if _, err := SystemdFiles(Spec{Trigger: Login, Command: "/usr/bin/x", VolumeUUIDs: []string{"A41C-77E2"}}); err == nil {
		t.Fatal("systemd accepted a login trigger")
	}
}

func TestUnderFolder(t *testing.T) {
	const pf = `C:\Program Files`
	for p, want := range map[string]bool{
		`C:\Program Files\USB Backup\usb-backup.exe`: true,
		`c:\PROGRAM FILES\USB Backup\usb-backup.exe`: true,
		`C:/Program Files/USB Backup/usb-backup.exe`: true,
		`C:\Program Files (x86)\x.exe`:               false,
		`C:\Program Files`:                           false,
		`C:\Program Files\..\Users\a\x.exe`:          false,
		`C:\Users\a\Program Files\x.exe`:             false,
		`\\server\C:\Program Files\x.exe`:            false,
		`C:\USB-Backup\usb-backup.exe`:               false,
	} {
		if got := underFolder(p, pf); got != want {
			t.Errorf("%s: %v", p, got)
		}
	}
	if underFolder(`C:\x.exe`, "") || !underFolder(`C:\Program Files\x.exe`, pf+`\`) {
		t.Error("folder")
	}
}
