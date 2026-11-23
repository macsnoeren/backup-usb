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

package cli

import (
	"testing"
	"time"
)

func TestFakesOnlyInTestBuild(t *testing.T) {
	env := &Env{Now: time.Now, Getenv: func(k string) string {
		return map[string]string{"USB_BACKUP_FAKE_DISKS": "/nonexistent.json", "USB_BACKUP_FAKE_NOW": "2026-12-31T12:00:00Z"}[k]
	}}
	if err := applyFakes(env); err != nil {
		t.Fatal(err)
	}
	if FakesEnabled {
		if env.Finder == nil || env.Now().Year() != 2026 || env.Now().Month() != 12 {
			t.Fatal("fakes not applied in a fakes build")
		}
	} else if env.Finder != nil {
		t.Fatal("fakes applied in a production build")
	}
}
