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

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestRecognition(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	w.enroll("BACKUP1", "odd")

	// A copied ID file on another disk: unknown disk, exit 6, nothing
	// written.
	data, _ := os.ReadFile(w.diskFile("BACKUP1", "Backup/disk-id.json"))
	testutil.WriteFile(t, w.disks.Disk("BACKUP2").Mount, "Backup/disk-id.json", string(data))
	w.disks.Remove("BACKUP1")
	w.mustRun(6, "run")
	// Even with the serial numbers in the copy edited to match the new
	// disk: the signature does not fit any more.
	b2 := w.disks.Disk("BACKUP2")
	edited := strings.NewReplacer(w.diskSerial(data, "volumeSerial"), b2.VolumeSerial).Replace(string(data))
	testutil.WriteFile(t, b2.Mount, "Backup/disk-id.json", edited)
	w.mustRun(6, "run")
	if exists(w.diskFile("BACKUP2", "Backup/2026")) {
		t.Fatal("written to an unknown disk")
	}

	// An ordinary stick without an ID file is ignored silently: no disk.
	os.Remove(w.diskFile("BACKUP2", "Backup/disk-id.json"))
	code, out, errOut := w.run("run", "--quiet-skip")
	if code != 2 || out != "" || errOut != "" {
		t.Fatalf("plain stick: %d %q %q", code, out, errOut)
	}
}

func TestFormattedDisk(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	w.enroll("BACKUP1", "odd")
	// Formatting: new volume serial, ID file gone. The disk is unknown
	// (silently, like any stick) until it is enrolled again.
	w.disks.Update("BACKUP1", func(d *testutil.FakeDisk) { d.VolumeSerial = "1234-5678" })
	os.RemoveAll(w.diskFile("BACKUP1", "Backup"))
	w.mustRun(2, "run")
	w.stdin = testPassword + "\n"
	w.mustRun(0, "enroll", w.disks.Disk("BACKUP1").Mount, "--name", "BACKUP1", "--weeks", "odd", "--force", "--password-stdin", "--vault", w.vaultPath())
	w.mustRun(0, "run")
}

func TestRequireHardwareSerial(t *testing.T) {
	w := newWorld(t)
	w.disks.Update("BACKUP1", func(d *testutil.FakeDisk) { d.HardwareSerial = "" })
	w.keysInit()
	w.enroll("BACKUP1", "odd")
	w.mustRun(0, "run", "--dry-run")
	w.set("requireHardwareSerial", true)
	w.mustRun(6, "run")
	// A changed hardware serial on a disk that had one: unknown.
	w.set("requireHardwareSerial", false)
	w.enroll("BACKUP2", "even")
	w.clock.Advance(7 * 24 * 3600e9)
	w.disks.Remove("BACKUP1")
	w.mustRun(0, "run", "--dry-run")
	w.disks.Update("BACKUP2", func(d *testutil.FakeDisk) { d.HardwareSerial = "OTHERDEVICE" })
	w.mustRun(6, "run")
}
