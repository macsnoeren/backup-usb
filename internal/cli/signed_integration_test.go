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
)

func TestSignedConfig(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	w.keysInit() // signs the existing settings and registrations
	if !exists(filepath.Join(w.cfgDir, "usb-backup.json.sig")) || !exists(filepath.Join(w.cfgDir, "usb-backup.disks.json.sig")) {
		t.Fatal("keys init did not sign the existing files")
	}
	w.mustRun(0, "run", "--dry-run")

	// A manual edit of the settings: exit 7, nothing done.
	p := filepath.Join(w.cfgDir, "usb-backup.json")
	data, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(data), "{", `{"keepPerDisk": 1,`, 1)), 0o644)
	_, errOut := w.mustRun(7, "run")
	if !strings.Contains(errOut, "changed without the operator") {
		t.Fatalf("stderr %q", errOut)
	}
	if exists(w.diskFile("BACKUP1", "Backup/2026")) {
		t.Fatal("run continued with tampered settings")
	}
	os.WriteFile(p, data, 0o644) // restore

	// Same for the registration list, and for a removed signature.
	rp := filepath.Join(w.cfgDir, "usb-backup.disks.json")
	reg, _ := os.ReadFile(rp)
	os.WriteFile(rp, []byte(strings.Replace(string(reg), `"BACKUP1"`, `"BACKUP9"`, 1)), 0o644)
	w.mustRun(7, "run")
	os.WriteFile(rp, reg, 0o644)
	sig, _ := os.ReadFile(p + ".sig")
	os.Remove(p + ".sig")
	w.mustRun(7, "run")
	os.WriteFile(p+".sig", sig, 0o644)

	// config set with the password signs the change.
	w.stdin = testPassword + "\n"
	out, _ := w.mustRun(0, "config", "set", "keepPerDisk", "12", "--password-stdin", "--vault", w.vaultPath())
	if !strings.Contains(out, "keepPerDisk changed from 25 to 12") {
		t.Fatalf("%s", out)
	}
	got, _ := w.mustRun(0, "config", "show", "keepPerDisk")
	if strings.TrimSpace(got) != "12" {
		t.Fatalf("keepPerDisk = %q", got)
	}
	w.mustRun(0, "run")
	// Wrong password: 13, nothing changed. Invalid value: 64.
	w.stdin = "wrong password!!\n"
	w.mustRun(13, "config", "set", "keepPerDisk", "3", "--password-stdin", "--vault", w.vaultPath())
	w.stdin = testPassword + "\n"
	w.mustRun(64, "config", "set", "keepPerDisk", "-3", "--password-stdin", "--vault", w.vaultPath())
	if got, _ := w.mustRun(0, "config", "show", "keepPerDisk"); strings.TrimSpace(got) != "12" {
		t.Fatal("changed despite errors")
	}
	log, _ := os.ReadFile(filepath.Join(w.cfgDir, "usb-backup.log"))
	if !strings.Contains(string(log), `setting changed key=keepPerDisk old=25 new=12`) || strings.Contains(string(log), testPassword) {
		t.Fatalf("log:\n%s", log)
	}
}
