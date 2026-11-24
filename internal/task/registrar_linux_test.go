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

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdInstallUninstall(t *testing.T) {
	var calls []string
	s := &Systemd{UnitDir: t.TempDir(), RulesDir: t.TempDir(), Run: func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}}
	if s.Installed(USB) {
		t.Fatal("installed before installing")
	}
	if err := s.Install(Spec{Trigger: USB, Command: "/usr/bin/usb-backup-cli", VolumeUUIDs: []string{"A41C-77E2"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ";") != "systemctl daemon-reload;udevadm control --reload" || !s.Installed(USB) {
		t.Fatal(calls)
	}
	// Tasks of older versions are removed, never made.
	if err := s.Install(Spec{Trigger: OldDaily, Command: "/usr/bin/usb-backup-cli"}); err == nil {
		t.Fatal("daily task made")
	}
	for _, f := range []string{"usb-backup-daily.timer", "usb-backup-daily.service"} {
		if err := os.WriteFile(filepath.Join(s.UnitDir, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	calls = nil
	if _, err := os.Stat(filepath.Join(s.RulesDir, UdevRule)); err != nil {
		t.Fatal(err)
	}
	for _, tr := range Triggers {
		if err := s.Uninstall(tr); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(strings.Join(calls, ";"), "systemctl disable --now usb-backup-daily.timer") || strings.Contains(strings.Join(calls, ";"), "weekly") {
		t.Fatal(calls)
	}
	for _, d := range []string{s.UnitDir, s.RulesDir} {
		if e, _ := os.ReadDir(d); len(e) != 0 {
			t.Fatalf("left in %s: %v", d, e)
		}
	}
}
