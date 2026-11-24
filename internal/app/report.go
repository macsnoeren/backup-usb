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
	"time"

	"github.com/macsnoeren/backup-usb/internal/archive"
)

// Run statuses.
const (
	StatusOK      = "ok"
	StatusWarning = "warning"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
	StatusDryRun  = "dryRun"
	StatusEmpty   = "empty"
)

// SourceReport is the result for one source folder.
type SourceReport struct {
	Tag      string `json:"tag"`
	Status   string `json:"status"`
	Archive  string `json:"archive,omitempty"`
	Files    int    `json:"files"`
	Dirs     int    `json:"dirs"`
	Bytes    uint64 `json:"bytes"`
	ZipBytes int64  `json:"zipBytes"`
	SHA256   string `json:"sha256,omitempty"`
	Verified bool   `json:"verified"`
	// Skipped lists files that were not backed up. Names are left out when
	// the backup is encrypted; SkippedCount is always set.
	SkippedCount int               `json:"skippedCount"`
	Skipped      []archive.Skipped `json:"skipped,omitempty"`
	WarningCount int               `json:"warningCount"`
	Warnings     []archive.Skipped `json:"warnings,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// DryRunInfo is what --dry-run reports.
type DryRunInfo struct {
	ExpectedBytes uint64   `json:"expectedBytes"`
	RequiredBytes uint64   `json:"requiredBytes"`
	WouldRotate   []string `json:"wouldRotate"`
}

// RunReport is the summary of a run: rapport/report.txt and the --json
// output.
type RunReport struct {
	Status      string         `json:"status"`
	Disk        string         `json:"disk,omitempty"`
	Device      string         `json:"device,omitempty"`
	Mount       string         `json:"mount,omitempty"`
	Period      string         `json:"period,omitempty"`
	Folder      string         `json:"folder,omitempty"`
	Started     time.Time      `json:"started"`
	Finished    time.Time      `json:"finished"`
	Encrypted   bool           `json:"encrypted"`
	Keys        []string       `json:"keys,omitempty"`
	Health      string         `json:"health,omitempty"`
	FreeBytes   uint64         `json:"freeBytes,omitempty"`
	TotalBytes  uint64         `json:"totalBytes,omitempty"`
	FolderCount int            `json:"folderCount,omitempty"`
	Rotated     []string       `json:"rotated,omitempty"`
	Messages    []string       `json:"messages,omitempty"`
	Sources     []SourceReport `json:"sources"`
	DryRun      *DryRunInfo    `json:"dryRun,omitempty"`
	Error       string         `json:"error,omitempty"`
	// MessageKeys are the translation keys of Messages, for the GUI.
	MessageKeys []string `json:"messageKeys,omitempty"`
}
