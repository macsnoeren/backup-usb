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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/week"
)

func TestMissingFileGivesDefaults(t *testing.T) {
	c, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := Defaults()
	if c.KeepPerDisk != 25 || c.FolderPattern != `{year}\{week}` || c.Schedule != week.ISOWeek ||
		c.Rotation != week.InTurn || !c.VerifyAfterBackup || c.CompressionLevel != 6 || len(c.StoreExtensions) != len(d.StoreExtensions) {
		t.Fatalf("defaults: %+v", c)
	}
	if runtime.GOOS == "windows" && (len(c.Sources) != 1 || c.Sources[0].Path != `C:\Data`) {
		t.Fatalf("default source: %+v", c.Sources)
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, dir, FileName, `{"keepPerDisk": 10, "sources": [{"tag":"Docs","path":"/srv/docs"}], "verifyAfterBackup": false}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.KeepPerDisk != 10 || c.VerifyAfterBackup || len(c.Sources) != 1 || c.Sources[0].Tag != "Docs" || c.LogKeep != 5 {
		t.Fatalf("loaded: %+v", c)
	}
}

func TestUnknownKey(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, dir, FileName, `{"keepPerDsk": 10}`)
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), `unknown setting "keepPerDsk"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestBadJSONAndSize(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, dir, FileName, `{"keepPerDisk": "x"}`)
	if _, err := Load(dir); err == nil {
		t.Fatal("type error accepted")
	}
	testutil.WriteFile(t, dir, FileName, `{} {}`)
	if _, err := Load(dir); err == nil {
		t.Fatal("trailing data accepted")
	}
	testutil.WriteFile(t, dir, FileName, `{"exclude":["`+strings.Repeat("a", MaxFileSize)+`"]}`)
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "larger") {
		t.Fatalf("huge file: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	c := Defaults()
	c.Sources = []Source{{Tag: "A", Path: "/a", Exclude: []string{"*.bak"}}}
	data, err := c.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := back.Marshal()
	if string(again) != string(data) {
		t.Fatalf("round trip differs:\n%s\n%s", data, again)
	}
}

func TestDirs(t *testing.T) {
	if Dir() == "" || DefaultVaultPath() == "" {
		t.Fatal("empty dir")
	}
	if !strings.HasSuffix(DefaultVaultPath(), VaultFileName) {
		t.Fatal(DefaultVaultPath())
	}
	if runtime.GOOS == "linux" && os.Geteuid() != 0 && !strings.HasSuffix(Dir(), filepath.Join(".config", "usb-backup")) && os.Getenv("XDG_CONFIG_HOME") == "" {
		t.Fatal(Dir())
	}
	t.Setenv("USBB_TEST", "x")
	if got := expandEnv(`%USBB_TEST%\v`); got != `x\v` {
		t.Fatal(got)
	}
	c := Defaults()
	if c.DisksPath("/etc/u") != filepath.Join("/etc/u", DefaultDisksFile) || c.LogDir("/d") != "/d" {
		t.Fatal("resolve")
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"keepPerDisk": 10}`))
	f.Add([]byte(`{"sources":[{"tag":"a","path":"b"}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := Parse(data)
		if err == nil {
			_ = c.Validate(ValidateOptions{SkipPathCheck: true})
		}
	})
}
