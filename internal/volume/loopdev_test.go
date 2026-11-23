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

//go:build linux && loopdev

package volume

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoopDevice creates a loop device with a file system as a fake USB
// disk and checks that the finder sees it and can mount it. Runs in CI as
// root: go test -tags loopdev ./internal/volume/
func TestLoopDevice(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	for _, tool := range []string{"losetup", "mkfs.ext4"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("needs " + tool)
		}
	}
	img := filepath.Join(t.TempDir(), "disk.img")
	if err := os.WriteFile(img, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(img, 64<<20); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("mkfs.ext4", "-q", "-L", "BACKUPLOOP", img).CombinedOutput(); err != nil {
		t.Fatalf("mkfs: %v %s", err, out)
	}
	out, err := exec.Command("losetup", "--find", "--show", img).Output()
	if err != nil {
		t.Fatal(err)
	}
	dev := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("losetup", "-d", dev).Run() })
	_ = exec.Command("udevadm", "settle").Run()
	f := linuxFinder{}
	vols, err := f.Volumes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var v *Volume
	for i := range vols {
		if vols[i].Device == dev {
			v = &vols[i]
		}
	}
	if v == nil || v.Label != "BACKUPLOOP" || v.VolumeSerial == "" || v.FileSystem != "ext4" {
		t.Fatalf("loop device not found or incomplete: %+v", vols)
	}
	m, undo, err := f.Mount(context.Background(), *v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.Mount, "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if free, _, err := f.Space(context.Background(), m); err != nil || free == 0 {
		t.Fatal(free, err)
	}
	if err := undo(); err != nil {
		t.Fatal(err)
	}
}
