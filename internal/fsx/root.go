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

package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// OpenDiskRoot opens the folder rel (slash or backslash separated) on a
// mounted disk as an os.Root, creating it if create is set (otherwise a
// missing folder gives fs.ErrNotExist). The mount point itself
// is trusted; every component below it must be a real folder, never a
// symlink or junction. All later access through the root stays inside it,
// so links planted on the disk cannot lead to files on the computer.
func OpenDiskRoot(mount, rel string, create bool) (*os.Root, error) {
	base, err := os.OpenRoot(mount)
	if err != nil {
		return nil, fmt.Errorf("open disk: %w", err)
	}
	defer func() { _ = base.Close() }()
	rel = path.Clean(strings.ReplaceAll(rel, `\`, "/"))
	if rel == "." || rel == "" {
		return nil, errors.New("empty backup folder")
	}
	cur := ""
	for _, comp := range strings.Split(rel, "/") {
		cur = path.Join(cur, comp)
		st, err := base.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) && !create {
			return nil, err
		}
		if errors.Is(err, fs.ErrNotExist) {
			if err := base.Mkdir(cur, 0o750); err != nil && !errors.Is(err, fs.ErrExist) {
				return nil, fmt.Errorf("create %s: %w", cur, err)
			}
			st, err = base.Lstat(cur)
		}
		if err != nil {
			return nil, fmt.Errorf("check %s: %w", cur, err)
		}
		if st.Mode().Type() != fs.ModeDir {
			return nil, fmt.Errorf("%s on the disk is not a plain folder", cur)
		}
	}
	r, err := base.OpenRoot(rel)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", rel, err)
	}
	return r, nil
}

// CreateExclusive removes an old file at rel and creates a new one with
// O_EXCL, so an existing symlink is never followed.
func CreateExclusive(root *os.Root, rel string, perm os.FileMode) (*os.File, error) {
	if err := root.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("remove old %s: %w", path.Base(rel), err)
	}
	f, err := root.OpenFile(rel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", path.Base(rel), err)
	}
	return f, nil
}

// RootWriteFileAtomic writes data to rel+".part" inside root, syncs it and
// renames it to rel.
func RootWriteFileAtomic(root *os.Root, rel string, data []byte, perm os.FileMode) error {
	part := rel + PartSuffix
	f, err := CreateExclusive(root, part, perm)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = root.Remove(part)
		return fmt.Errorf("write %s: %w", path.Base(part), err)
	}
	if err := RootRename(root, part, rel); err != nil {
		_ = root.Remove(part)
		return err
	}
	return nil
}

// RootRename renames inside root and syncs the target folder.
func RootRename(root *os.Root, from, to string) error {
	if err := root.Rename(from, to); err != nil {
		return fmt.Errorf("rename %s: %w", path.Base(from), err)
	}
	return RootSyncDir(root, path.Dir(to))
}

// RootSyncDir syncs a folder inside root where the platform supports it.
func RootSyncDir(root *os.Root, dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := root.Open(filepath.FromSlash(dir))
	if err != nil {
		return fmt.Errorf("open directory: %w", err)
	}
	serr := d.Sync()
	cerr := d.Close()
	if serr != nil && !errors.Is(serr, os.ErrInvalid) {
		return fmt.Errorf("sync directory: %w", serr)
	}
	return cerr
}
