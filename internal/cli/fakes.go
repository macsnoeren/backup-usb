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

//go:build fakes

package cli

import (
	"fmt"
	"time"

	"github.com/macsnoeren/backup-usb/internal/volume"
)

// FakesEnabled reports whether this is a test build with fake disks and a
// fake clock.
const FakesEnabled = true

// applyFakes replaces the disk finder and the clock from the environment.
// Only compiled into test binaries (build tag "fakes").
func applyFakes(e *Env) error {
	if p := e.Getenv("USB_BACKUP_FAKE_DISKS"); p != "" {
		e.Finder = volume.NewFakeFinder(p)
	}
	if s := e.Getenv("USB_BACKUP_FAKE_NOW"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return fmt.Errorf("USB_BACKUP_FAKE_NOW: %w", err)
		}
		start := time.Now()
		e.Now = func() time.Time { return t.Add(time.Since(start)) }
	}
	if d := e.Getenv("USB_BACKUP_FAKE_CONFIG"); d != "" {
		e.ConfigDir = d
	}
	return nil
}
