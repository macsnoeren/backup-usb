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

package app

import (
	"context"

	"github.com/macsnoeren/backup-usb/internal/config"
)

// Eject safely removes the connected backup disk (or the one matching
// target).
func Eject(ctx context.Context, deps *Deps, cfg *config.Config, target string) (string, error) {
	// Never dismount a disk in the middle of a run or rewrap.
	release, err := deps.AcquireLock()
	if err != nil {
		return "", err
	}
	defer release()
	disks, err := connectedDisks(ctx, deps, cfg, target)
	if err != nil {
		return "", err
	}
	if len(disks) != 1 {
		return "", Errorf(KindUsage, nil, "err.oneDisk")
	}
	c := disks[0]
	if err := deps.Finder.Eject(ctx, c.Vol); err != nil {
		return "", Errorf(KindIO, err, "err.eject", c.Reg.Name)
	}
	deps.Log.Info("disk ejected", "disk", c.Reg.Name)
	return c.Reg.Name, nil
}
