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

//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/disk"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

type machine struct {
	cfgDir string
	disks  *testutil.FakeDiskSet
	env    []string
}

func newMachine(t *testing.T) *machine {
	m := &machine{cfgDir: t.TempDir(), disks: testutil.FakeDisks(t, "BACKUP1", "BACKUP2")}
	src := t.TempDir()
	testutil.WriteFile(t, src, "a.txt", "hello")
	cfg := map[string]any{"sources": []map[string]string{{"tag": "Data", "path": src}}}
	data, _ := json.Marshal(cfg)
	testutil.WriteFile(t, m.cfgDir, config.FileName, string(data))
	m.env = []string{
		"USB_BACKUP_FAKE_DISKS=" + m.disks.File,
		"USB_BACKUP_FAKE_NOW=2026-11-18T10:00:00Z",
		"USB_BACKUP_FAKE_CONFIG=" + m.cfgDir,
	}
	return m
}

// enroll registers BACKUP1 (odd) without signatures.
func (m *machine) enroll(t *testing.T) {
	d := m.disks.Disk("BACKUP1")
	r := disk.Registration{Name: "BACKUP1", Weeks: "odd", ID: "3f9c2b1e-7a4d-4e8b-9c21-5d6e7f8a9b0c", VolumeSerial: d.VolumeSerial, PartitionID: d.PartitionID, HardwareSerial: d.HardwareSerial}
	reg := &disk.Registry{Disks: []disk.Registration{r}}
	data, _ := reg.Marshal()
	testutil.WriteFile(t, m.cfgDir, config.DefaultDisksFile, string(data))
	id, _ := json.Marshal(disk.IDFile{Version: 1, ID: r.ID, Name: r.Name, VolumeSerial: r.VolumeSerial, PartitionID: r.PartitionID, HardwareSerial: r.HardwareSerial})
	testutil.WriteFile(t, d.Mount, "Backup/disk-id.json", string(id))
}

func TestRunWithRealBinary(t *testing.T) {
	m := newMachine(t)
	if r := cli(t, m.env, "", "run", "--quiet-skip"); r.code != 2 || r.stdout != "" || r.stderr != "" {
		t.Fatalf("no disk: %+v", r)
	}
	m.enroll(t)
	r := cli(t, m.env, "", "run", "--json")
	if r.code != 0 {
		t.Fatalf("run: %+v", r)
	}
	var rep struct{ Status, Disk, Folder string }
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil || rep.Disk != "BACKUP1" || rep.Folder != "2026/47" {
		t.Fatalf("report %+v %v", rep, err)
	}
	if _, err := os.Stat(filepath.Join(m.disks.Disk("BACKUP1").Mount, "Backup", "2026", "47", "Data.zip")); err != nil {
		t.Fatal(err)
	}
	if r := cli(t, m.env, "", "run"); r.code != 4 {
		t.Fatalf("duplicate: %+v", r)
	}
}
