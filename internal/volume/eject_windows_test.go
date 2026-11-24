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
	"errors"
	"testing"
)

// Looks through the real disks without removing anything.
func TestDiskInstanceUnknown(t *testing.T) {
	if inst, err := diskInstance(0xfffffff0_fffffff0); !errors.Is(err, errNotUSB) || inst != 0 {
		t.Fatalf("%d %v", inst, err)
	}
}

func TestEjectRefusesFixedDisk(t *testing.T) {
	if err := (windowsFinder{}).Eject(context.Background(), Volume{Device: "C:", Mount: `C:\`}); err == nil {
		t.Fatal("a fixed disk was ejected")
	}
}
