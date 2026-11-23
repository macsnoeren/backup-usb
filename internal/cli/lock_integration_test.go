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
	"path/filepath"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/lock"
)

func TestSecondInstance(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	release, err := lock.Acquire(filepath.Join(w.cfgDir, "test.lock"))
	if err != nil {
		t.Fatal(err)
	}
	w.mustRun(5, "run")
	w.mustRun(5, "verify")
	release()
	w.mustRun(0, "run")
}
