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
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// Files in the backup folder of a disk.
const (
	LastBackupFile = "last-backup.txt"
	DiskStateFile  = "keys.json"
	ReportFile     = "report.txt"
	maxStateFile   = 4 << 20
)

// LastBackup is the content of last-backup.txt.
type LastBackup struct {
	Time   time.Time
	Folder string
}

// readLimited reads rel from root, at most max bytes. A missing file gives
// fs.ErrNotExist. Everything on the disk is untrusted.
func readLimited(root *os.Root, rel string, maxSize int64) ([]byte, error) {
	return fsx.ReadRegular(root, rel, maxSize)
}

// ReadLastBackup reads last-backup.txt; a missing or unreadable file gives
// nil (no recent backup).
func ReadLastBackup(root *os.Root) *LastBackup {
	data, err := readLimited(root, LastBackupFile, 4096)
	if err != nil {
		return nil
	}
	lb := &LastBackup{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), "=")
		switch k {
		case "time":
			t, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
			if err != nil {
				return nil
			}
			lb.Time = t
		case "folder":
			lb.Folder = strings.TrimSpace(v)
		}
	}
	if lb.Time.IsZero() {
		return nil
	}
	return lb
}

// WriteLastBackup writes last-backup.txt.
func WriteLastBackup(root *os.Root, lb LastBackup) error {
	data := fmt.Sprintf("time=%s\nfolder=%s\n", lb.Time.Format(time.RFC3339), lb.Folder)
	return fsx.RootWriteFileAtomic(root, LastBackupFile, []byte(data), 0o644)
}

// PatternRecord is a folder pattern that was used on this disk.
type PatternRecord struct {
	Pattern  string        `json:"pattern"`
	Schedule week.Schedule `json:"schedule"`
}

// FolderRecord describes one period folder.
type FolderRecord struct {
	Time time.Time `json:"time"`
	// Keys are the fingerprints of the public keys the backup was
	// encrypted for; empty if not encrypted.
	Keys []string `json:"keys,omitempty"`
}

// DiskState is keys.json in the backup folder: the folder patterns that
// were used on this disk (rotation only touches those) and, per folder, the
// keys that were used.
type DiskState struct {
	Version  int                     `json:"version"`
	Patterns []PatternRecord         `json:"patterns"`
	Folders  map[string]FolderRecord `json:"folders"`
}

// ReadDiskState reads keys.json; a missing file gives an empty state.
func ReadDiskState(root *os.Root) (*DiskState, error) {
	st := &DiskState{Version: 1, Folders: map[string]FolderRecord{}}
	data, err := readLimited(root, DiskStateFile, maxStateFile)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(st); err != nil {
		return nil, fmt.Errorf("%s: %w", DiskStateFile, err)
	}
	if st.Folders == nil {
		st.Folders = map[string]FolderRecord{}
	}
	return st, nil
}

// Write saves keys.json.
func (s *DiskState) Write(root *os.Root) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", DiskStateFile, err)
	}
	return fsx.RootWriteFileAtomic(root, DiskStateFile, append(data, '\n'), 0o644)
}

// AddPattern records a pattern if it is not there yet.
func (s *DiskState) AddPattern(p string, sch week.Schedule) {
	for _, r := range s.Patterns {
		if r.Pattern == p && r.Schedule == sch {
			return
		}
	}
	s.Patterns = append(s.Patterns, PatternRecord{Pattern: p, Schedule: sch})
}

// RotationPatterns returns the recorded patterns that are valid for the
// schedule. Invalid entries (the file is untrusted) are ignored.
func (s *DiskState) RotationPatterns(sch week.Schedule) []*week.Pattern {
	var out []*week.Pattern
	for _, r := range s.Patterns {
		if r.Schedule != sch {
			continue
		}
		if p, err := week.ParsePattern(r.Pattern, sch); err == nil {
			out = append(out, p)
		}
	}
	return out
}
