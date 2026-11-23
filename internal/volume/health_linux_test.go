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

package volume

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestHealthWithFakeSmartctl puts a fake smartctl in place (the finder only
// uses programs from fixed system folders; tests point it elsewhere).
func TestHealthWithFakeSmartctl(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "smartctl")
	old := smartctlPath
	t.Cleanup(func() { smartctlPath = old })
	f := linuxFinder{}
	v := Volume{Device: "/dev/sdz1"}
	for out, want := range map[string]Health{
		"SMART overall-health self-assessment test result: PASSED":  Healthy,
		"SMART overall-health self-assessment test result: FAILED!": Unhealthy,
		"Unknown USB bridge": Unknown,
	} {
		os.WriteFile(fake, []byte("#!/bin/sh\necho '"+out+"'\n"), 0o755)
		smartctlPath = fake
		if got := f.Health(context.Background(), v); got != want {
			t.Errorf("%q: %s", out, got)
		}
	}
	smartctlPath = ""
	if f.Health(context.Background(), v) != Unknown {
		t.Fatal("without smartctl the health is unknown")
	}
	if f.Health(context.Background(), Volume{Device: "/dev/sda; rm -rf /"}) != Unknown {
		t.Fatal("bad device name accepted")
	}
}

func TestLinuxFinderRuns(t *testing.T) {
	if lsblkPath == "" {
		t.Skip("no lsblk")
	}
	vols, err := linuxFinder{}.Volumes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vols {
		if v.Mount == "/" {
			t.Fatal("system volume listed")
		}
	}
}
