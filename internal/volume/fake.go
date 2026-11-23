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
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FakeDisk is one fake disk in a fake disk file. Only used by tests and by
// binaries built with the "fakes" build tag.
type FakeDisk struct {
	Volume
	Health Health `json:"health"`
	// Unmounted simulates a volume that must be mounted first.
	Unmounted bool `json:"unmounted,omitempty"`
	// Ejected is set by Eject.
	Ejected bool `json:"ejected,omitempty"`
	// Capacity, if set, makes the free space Capacity minus the bytes
	// stored under Mount, so deleting files frees space.
	Capacity uint64 `json:"capacity,omitempty"`
}

// FakeFile is the content of the file named by USB_BACKUP_FAKE_DISKS.
type FakeFile struct {
	Disks []FakeDisk `json:"disks"`
}

// FakeFinder presents directories described in a JSON file as disks. The
// file is read on every call so tests can change it while a run is going on.
type FakeFinder struct {
	Path string
}

// NewFakeFinder returns a Finder reading disks from the JSON file at path.
func NewFakeFinder(path string) *FakeFinder { return &FakeFinder{Path: path} }

func (f *FakeFinder) load() (*FakeFile, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("read fake disks: %w", err)
	}
	var ff FakeFile
	if err := json.Unmarshal(data, &ff); err != nil {
		return nil, fmt.Errorf("parse fake disks: %w", err)
	}
	return &ff, nil
}

func (f *FakeFinder) find(v Volume) (*FakeFile, int, error) {
	ff, err := f.load()
	if err != nil {
		return nil, 0, err
	}
	for i := range ff.Disks {
		if ff.Disks[i].Device == v.Device {
			return ff, i, nil
		}
	}
	return nil, 0, fmt.Errorf("fake disk %q not found", v.Device)
}

// Volumes implements Finder.
func (f *FakeFinder) Volumes(context.Context) ([]Volume, error) {
	ff, err := f.load()
	if err != nil {
		return nil, err
	}
	out := make([]Volume, 0, len(ff.Disks))
	for _, d := range ff.Disks {
		if d.Ejected {
			continue
		}
		v := d.Volume
		if d.Unmounted {
			v.Mount = ""
		}
		out = append(out, v)
	}
	return out, nil
}

// Space implements Finder.
func (f *FakeFinder) Space(_ context.Context, v Volume) (uint64, uint64, error) {
	ff, i, err := f.find(v)
	if err != nil {
		return 0, 0, err
	}
	d := ff.Disks[i]
	if d.Capacity > 0 {
		used := du(d.Mount)
		if used >= d.Capacity {
			return 0, d.Capacity, nil
		}
		return d.Capacity - used, d.Capacity, nil
	}
	return d.FreeBytes, d.TotalBytes, nil
}

func du(dir string) uint64 {
	var n uint64
	_ = filepath.WalkDir(dir, func(_ string, de fs.DirEntry, err error) error {
		if err == nil && de.Type().IsRegular() {
			if info, err := de.Info(); err == nil {
				n += uint64(max(info.Size(), 0))
			}
		}
		return nil
	})
	return n
}

// Health implements Finder.
func (f *FakeFinder) Health(_ context.Context, v Volume) Health {
	ff, i, err := f.find(v)
	if err != nil || ff.Disks[i].Health == "" {
		return Unknown
	}
	return ff.Disks[i].Health
}

// Mount implements Finder.
func (f *FakeFinder) Mount(_ context.Context, v Volume) (Volume, func() error, error) {
	ff, i, err := f.find(v)
	if err != nil {
		return v, func() error { return nil }, err
	}
	v.Mount = ff.Disks[i].Mount
	return v, func() error { return nil }, nil
}

// Eject implements Finder.
func (f *FakeFinder) Eject(_ context.Context, v Volume) error {
	ff, i, err := f.find(v)
	if err != nil {
		return err
	}
	ff.Disks[i].Ejected = true
	data, err := json.MarshalIndent(ff, "", "  ")
	if err != nil {
		return fmt.Errorf("encode fake disks: %w", err)
	}
	if err := os.WriteFile(f.Path, data, 0o600); err != nil {
		return fmt.Errorf("write fake disks: %w", err)
	}
	return nil
}
