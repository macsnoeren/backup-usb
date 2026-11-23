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

package disk

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/sign"
	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/volume"
)

func reg() (*Registry, volume.Volume, *IDFile) {
	v := volume.Volume{VolumeSerial: "A41C-77E2", PartitionID: "{6b1f}", HardwareSerial: "5758"}
	r := Registration{Name: "BACKUP1", Weeks: "odd", ID: "3f9c2b1e-7a4d-4e8b-9c21-5d6e7f8a9b0c", VolumeSerial: v.VolumeSerial, PartitionID: v.PartitionID, HardwareSerial: v.HardwareSerial}
	id := &IDFile{Version: 1, ID: r.ID, Name: r.Name, VolumeSerial: r.VolumeSerial, PartitionID: r.PartitionID, HardwareSerial: r.HardwareSerial}
	return &Registry{Version: 1, Disks: []Registration{r}}, v, id
}

func TestMatch(t *testing.T) {
	rg, v, id := reg()
	if r, err := Match(v, id, nil, rg, false, nil); err != nil || r.Name != "BACKUP1" {
		t.Fatal(r, err)
	}
	cases := map[string]func(v *volume.Volume, id *IDFile, rg *Registry){
		"other volume serial": func(v *volume.Volume, _ *IDFile, _ *Registry) { v.VolumeSerial = "0000-0000" },
		"other partition":     func(v *volume.Volume, _ *IDFile, _ *Registry) { v.PartitionID = "{x}" },
		"other hardware":      func(v *volume.Volume, _ *IDFile, _ *Registry) { v.HardwareSerial = "1" },
		"no hardware":         func(v *volume.Volume, _ *IDFile, _ *Registry) { v.HardwareSerial = "" },
		"unknown id":          func(_ *volume.Volume, id *IDFile, _ *Registry) { id.ID = "00000000-0000-0000-0000-000000000000" },
		"renamed":             func(_ *volume.Volume, id *IDFile, _ *Registry) { id.Name = "BACKUP2" },
		"id file serial":      func(_ *volume.Volume, id *IDFile, _ *Registry) { id.VolumeSerial = "X" },
		"require hw":          func(_ *volume.Volume, _ *IDFile, rg *Registry) { rg.Disks[0].HardwareSerial = "" },
	}
	for name, change := range cases {
		rg, v, id := reg()
		change(&v, id, rg)
		var m *Mismatch
		if _, err := Match(v, id, nil, rg, name == "require hw", nil); !errors.As(err, &m) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// Without a registered hardware serial and without requireHW, a stick
	// without serial is fine.
	rg, v, id = reg()
	rg.Disks[0].HardwareSerial, v.HardwareSerial, id.HardwareSerial = "", "", ""
	if _, err := Match(v, id, nil, rg, false, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryParse(t *testing.T) {
	rg, _, _ := reg()
	data, err := rg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseRegistry(data)
	if err != nil || len(back.Disks) != 1 {
		t.Fatal(back, err)
	}
	rg.Disks = append(rg.Disks, rg.Disks[0])
	dup, _ := json.Marshal(rg)
	for _, bad := range [][]byte{dup, []byte(`{"disks":[{"name":"../x","id":"3f9c2b1e-7a4d-4e8b-9c21-5d6e7f8a9b0c"}]}`), []byte(`{"extra":1}`), []byte(`{"disks":[{"name":"A","id":"nope"}]}`)} {
		if _, err := ParseRegistry(bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestReadIDFile(t *testing.T) {
	mount := t.TempDir()
	if _, _, err := ReadIDFile(mount); !errors.Is(err, ErrNoIDFile) {
		t.Fatal(err)
	}
	testutil.WriteFile(t, mount, "Backup/disk-id.json", `{"version":1,"id":"3f9c2b1e-7a4d-4e8b-9c21-5d6e7f8a9b0c","name":"BACKUP1","volumeSerial":"x","partitionId":"y","enrolled":"2026-11-22"}`)
	id, _, err := ReadIDFile(mount)
	if err != nil || id.Name != "BACKUP1" {
		t.Fatal(id, err)
	}
	testutil.WriteFile(t, mount, "Backup/disk-id.json", `{"id":"x"}`)
	if _, _, err := ReadIDFile(mount); err == nil || errors.Is(err, ErrNoIDFile) {
		t.Fatal(err)
	}
}

func FuzzParseIDFile(f *testing.F) {
	f.Add([]byte(`{"version":1,"id":"3f9c2b1e-7a4d-4e8b-9c21-5d6e7f8a9b0c","name":"B"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseIDFile(data)
		_, _ = ParseRegistry(data)
	})
}

func TestIDFileMustBeRegular(t *testing.T) {
	mount := t.TempDir()
	os.MkdirAll(filepath.Join(mount, "Backup"), 0o755)
	victim := filepath.Join(t.TempDir(), "host.json")
	os.WriteFile(victim, []byte(`{"secretfield":1}`), 0o600)
	if err := os.Symlink(victim, filepath.Join(mount, "Backup", "disk-id.json")); err == nil {
		if _, _, err := ReadIDFile(mount); err == nil || errors.Is(err, ErrNoIDFile) || strings.Contains(err.Error(), "secretfield") {
			t.Fatalf("symlinked ID file: %v", err)
		}
		os.Remove(filepath.Join(mount, "Backup", "disk-id.json"))
	}
	if err := mkfifo(filepath.Join(mount, "Backup", "disk-id.json")); err == nil {
		done := make(chan error, 1)
		go func() { _, _, err := ReadIDFile(mount); done <- err }()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("FIFO accepted")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a FIFO as ID file hangs the run")
		}
	}
}

func TestMatchSignature(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(testutil.TestSigningSeed(t, "signing"))
	pub := priv.Public().(ed25519.PublicKey)
	rg, v, id := reg()
	raw, _ := json.Marshal(id)
	signed, _ := sign.SignJSON(priv, sign.PurposeDiskID, raw)
	if _, err := Match(v, id, signed, rg, false, pub); err != nil {
		t.Fatal(err)
	}
	var m *Mismatch
	if _, err := Match(v, id, raw, rg, false, pub); !errors.As(err, &m) {
		t.Fatal("unsigned ID file accepted")
	}
	other := ed25519.NewKeyFromSeed(testutil.TestSigningSeed(t, "signing-other"))
	forged, _ := sign.SignJSON(other, sign.PurposeDiskID, raw)
	if _, err := Match(v, id, forged, rg, false, pub); !errors.As(err, &m) {
		t.Fatal("ID file signed by another key accepted")
	}
}
