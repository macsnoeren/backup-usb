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

package rotate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/week"
)

// Folder is a period folder on a disk.
type Folder struct {
	// Rel is the slash path relative to the backup folder, e.g. "2026/47".
	Rel    string
	Period week.Period
	// When orders the folders: when the backup was made as far as known,
	// and always within Period.
	When time.Time
	// Shared is set when the folder is not certainly one backup: it
	// matches patterns of two schedules, or it contains another period
	// folder. Such a folder is never deleted.
	Shared bool
}

// Policy is the retention policy.
type Policy struct {
	// KeepPerDisk is the number of folders to keep, of all schedules
	// together; 0 keeps everything.
	KeepPerDisk int
	// KeepMinDays protects folders whose backup was made, as far as known,
	// less than this many days ago.
	KeepMinDays int
}

// Find returns the period folders under root that match one of the
// patterns exactly, oldest first. Only real directories count: files and
// symlinks are ignored, as is everything that does not match. made gives
// the time a backup was made, from the disk: it is only used within the
// period of the folder. It may be nil.
func Find(root *os.Root, patterns []*week.Pattern, made func(rel string) (time.Time, bool)) ([]Folder, error) {
	found := map[string]Folder{}
	for _, p := range patterns {
		if err := collect(root, ".", "", p.Depth(), func(rel string) {
			per, ok := p.Match(rel)
			if !ok {
				return
			}
			f, dup := found[rel]
			if !dup {
				found[rel] = Folder{Rel: rel, Period: per, When: per.Start}
				return
			}
			if f.Period.Schedule != per.Schedule {
				f.Shared = true
				found[rel] = f
			}
		}); err != nil {
			return nil, err
		}
	}
	for rel := range found {
		for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
			if f, ok := found[parent]; ok {
				f.Shared = true
				found[parent] = f
			}
		}
	}
	out := make([]Folder, 0, len(found))
	for _, f := range found {
		if made != nil && !f.Shared {
			if t, ok := made(f.Rel); ok {
				f.When = f.Period.Clamp(t)
			}
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].When.Equal(out[j].When) {
			return out[i].When.Before(out[j].When)
		}
		return out[i].Rel < out[j].Rel
	})
	return out, nil
}

// collect calls fn for every real directory exactly depth levels below dir.
func collect(root *os.Root, dir, rel string, depth int, fn func(rel string)) error {
	f, err := root.Open(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", dir, err)
	}
	entries, err := f.ReadDir(-1)
	cerr := f.Close()
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	if cerr != nil {
		return cerr
	}
	for _, e := range entries {
		if !e.IsDir() || e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		r := e.Name()
		if rel != "" {
			r = rel + "/" + e.Name()
		}
		if depth == 1 {
			fn(r)
			continue
		}
		if err := collect(root, path.Join(dir, e.Name()), r, depth-1, fn); err != nil {
			return err
		}
	}
	return nil
}

// NotFuture drops folders whose period starts after now. A folder dated
// in the future (a wrong clock, or planted on the disk) must neither count
// toward keepPerDisk nor be picked as the newest backup.
func NotFuture(folders []Folder, now time.Time) []Folder {
	out := folders[:0:0]
	for _, f := range folders {
		if !f.Period.Start.After(now) {
			out = append(out, f)
		}
	}
	return out
}

// Certain drops shared folders.
func Certain(folders []Folder) []Folder {
	out := folders[:0:0]
	for _, f := range folders {
		if !f.Shared {
			out = append(out, f)
		}
	}
	return out
}

// Plan returns the folders to delete: the oldest, until KeepPerDisk are
// left, never one younger than KeepMinDays and never protected ones (the
// folder that was just written). Folders dated in the future and shared
// folders are ignored.
func Plan(folders []Folder, pol Policy, now time.Time, protected ...string) []Folder {
	folders = Certain(NotFuture(folders, now))
	if pol.KeepPerDisk <= 0 || len(folders) <= pol.KeepPerDisk {
		return nil
	}
	prot := map[string]bool{}
	for _, p := range protected {
		prot[p] = true
	}
	excess := len(folders) - pol.KeepPerDisk
	var del []Folder
	for _, f := range folders { // oldest first
		if len(del) == excess {
			break
		}
		if prot[f.Rel] || TooYoung(f, pol, now) {
			continue
		}
		del = append(del, f)
	}
	return del
}

// TooYoung reports whether the folder is protected by KeepMinDays.
func TooYoung(f Folder, pol Policy, now time.Time) bool {
	age := now.Sub(f.Period.Start)
	if !f.When.IsZero() {
		age = now.Sub(f.When)
	}
	return pol.KeepMinDays > 0 && age < time.Duration(pol.KeepMinDays)*24*time.Hour
}

// Delete removes the folders and then their parents that became empty.
// Everything goes through root, so nothing outside it can be touched, and
// only paths that are known period folders are removed.
func Delete(root *os.Root, del []Folder) ([]string, error) {
	var removed []string
	for _, f := range del {
		if f.Rel == "" || f.Rel == "." || strings.Contains(f.Rel, "..") || strings.HasPrefix(f.Rel, "/") {
			return removed, fmt.Errorf("refusing to delete %q", f.Rel)
		}
		st, err := root.Lstat(f.Rel)
		if err != nil {
			return removed, fmt.Errorf("check %s: %w", f.Rel, err)
		}
		if !st.IsDir() {
			return removed, fmt.Errorf("refusing to delete %s: not a folder", f.Rel)
		}
		if err := root.RemoveAll(f.Rel); err != nil {
			return removed, fmt.Errorf("delete %s: %w", f.Rel, err)
		}
		removed = append(removed, f.Rel)
		// Remove parents that are now empty (for example a year folder).
		for parent := path.Dir(f.Rel); parent != "." && parent != "/"; parent = path.Dir(parent) {
			if err := root.Remove(parent); err != nil {
				break // not empty, or gone already
			}
		}
	}
	return removed, nil
}
