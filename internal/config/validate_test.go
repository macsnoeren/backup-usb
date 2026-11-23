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

package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func validConfig(t *testing.T) *Config {
	c := Defaults()
	src := t.TempDir()
	c.Sources = []Source{{Tag: "Data", Path: src}, {Tag: "Boek-houding_2", Path: src}}
	return c
}

func TestValidConfig(t *testing.T) {
	if err := validConfig(t).Validate(ValidateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestEachInvalidValue(t *testing.T) {
	file := testutil.WriteFile(t, t.TempDir(), "f.txt", "x")
	tests := []struct {
		name   string
		change func(c *Config)
		want   string
	}{
		{"bad tag", func(c *Config) { c.Sources[0].Tag = "Da ta" }, `sources[0].tag: "Da ta" is not a valid tag`},
		{"empty tag", func(c *Config) { c.Sources[0].Tag = "" }, `sources[0].tag`},
		{"slash tag", func(c *Config) { c.Sources[0].Tag = "../x" }, `sources[0].tag`},
		{"duplicate tag", func(c *Config) { c.Sources[1].Tag = "data" }, `sources[1].tag: tag "data" is used twice`},
		{"missing path", func(c *Config) { c.Sources[0].Path = filepath.Join(t.TempDir(), "nope") }, "does not exist"},
		{"file path", func(c *Config) { c.Sources[0].Path = file }, "is not a folder"},
		{"relative path", func(c *Config) { c.Sources[0].Path = "data" }, "is not an absolute path"},
		{"bad exclude", func(c *Config) { c.Exclude = []string{"[x"} }, `exclude: "[x" is not a valid pattern`},
		{"bad source exclude", func(c *Config) { c.Sources[0].Exclude = []string{""} }, `sources[0].exclude: empty pattern`},
		{"negative keep", func(c *Config) { c.KeepPerDisk = -1 }, "keepPerDisk: must be 0 or more"},
		{"negative minDays", func(c *Config) { c.KeepMinDays = -3 }, "keepMinDays"},
		{"negative interval", func(c *Config) { c.MinIntervalHours = -1 }, "minIntervalHours"},
		{"pattern", func(c *Config) { c.FolderPattern = "{year}" }, "folderPattern:"},
		{"pattern traversal", func(c *Config) { c.FolderPattern = `..\{year}\{week}` }, "folderPattern:"},
		{"schedule", func(c *Config) { c.Schedule = "weekly" }, `schedule: "weekly"`},
		{"rotation", func(c *Config) { c.Rotation = "random" }, `rotation: "random"`},
		{"freeSpaceAction", func(c *Config) { c.FreeSpaceAction = "panic" }, "freeSpaceAction"},
		{"compression", func(c *Config) { c.CompressionLevel = 10 }, "compressionLevel: must be between 0 and 9"},
		{"store ext", func(c *Config) { c.StoreExtensions = []string{"j/pg"} }, "storeExtensions"},
		{"lockedFiles", func(c *Config) { c.LockedFiles = "wait" }, "lockedFiles"},
		{"minFree", func(c *Config) { c.MinFreePercent = 99 }, "minFreePercent"},
		{"target abs", func(c *Config) { c.TargetFolder = `C:\Backup` }, "targetFolder"},
		{"target dotdot", func(c *Config) { c.TargetFolder = `Backup\..\..` }, "targetFolder"},
		{"target empty", func(c *Config) { c.TargetFolder = "" }, "targetFolder: must not be empty"},
		{"archive", func(c *Config) { c.ArchiveName = "data.zip" }, "archiveName"},
		{"archive slash", func(c *Config) { c.ArchiveName = "../{tag}.zip" }, "archiveName"},
		{"encryption", func(c *Config) { c.Encryption = "aes" }, "encryption"},
		{"notify", func(c *Config) { c.Notify = "sometimes" }, "notify"},
		{"language", func(c *Config) { c.Language = "de" }, "language"},
		{"webhook", func(c *Config) { c.WebhookURL = "ftp://x" }, "webhookUrl"},
		{"webhook creds", func(c *Config) { c.WebhookURL = "https://u:p@x" }, "webhookUrl"},
		{"logLevel", func(c *Config) { c.LogLevel = "trace" }, "logLevel"},
		{"logMax", func(c *Config) { c.LogMaxSizeMB = 0 }, "logMaxSizeMB"},
		{"wait", func(c *Config) { c.WaitSeconds = -1 }, "waitSeconds"},
		{"disksFile", func(c *Config) { c.DisksFile = "" }, "disksFile"},
		{"operatorTimeout", func(c *Config) { c.OperatorTimeoutMinutes = 0 }, "operatorTimeoutMinutes"},
	}
	for _, tt := range tests {
		c := validConfig(t)
		tt.change(c)
		err := c.Validate(ValidateOptions{})
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
			continue
		}
		var joined interface{ Unwrap() []error }
		if errors.As(err, &joined) && len(joined.Unwrap()) != 1 {
			t.Errorf("%s: expected exactly one error, got %v", tt.name, err)
		}
	}
}

func TestArchiveFile(t *testing.T) {
	if got := Defaults().ArchiveFile("Data"); got != "Data.zip" {
		t.Fatal(got)
	}
	c := Defaults()
	c.TargetFolder = `Back-ups\Office`
	if p := c.TargetParts(); len(p) != 2 || p[1] != "Office" {
		t.Fatal(p)
	}
}
