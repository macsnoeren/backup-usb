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
	"context"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/volume"
)

// countingFinder records mounts and unmounts.
type countingFinder struct {
	volume.Finder
	mounted, unmounted int
}

func (f *countingFinder) Mount(ctx context.Context, v volume.Volume) (volume.Volume, func() error, error) {
	m, _, err := f.Finder.Mount(ctx, v)
	f.mounted++
	return m, func() error { f.unmounted++; return nil }, err
}

func TestMountsAreUndone(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	w.disks.Update("BACKUP1", func(d *testutil.FakeDisk) { d.Unmounted = true })
	cf := &countingFinder{Finder: w.disks.Finder()}
	env := w.env()
	env.Finder = cf
	if code, out, errOut := run(t, env, "run"); code != 0 {
		t.Fatal(out, errOut)
	}
	if cf.mounted == 0 || cf.mounted != cf.unmounted {
		t.Fatalf("mounted %d, unmounted %d", cf.mounted, cf.unmounted)
	}
}
