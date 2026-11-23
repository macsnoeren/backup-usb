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

// Package backupusb_test checks the license notices of the repository.
package backupusb_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Defines -update, so "go test ./... -update" works for this package too.
	_ "github.com/macsnoeren/backup-usb/internal/testutil"
)

const spdx = "SPDX-License-Identifier: GPL-3.0-or-later"

// needsHeader reports whether a file is source or packaging that must carry
// the license notice. Data (testdata, golden files, keys) does not.
func needsHeader(path string) bool {
	switch filepath.Base(path) {
	case "Makefile":
		return true
	}
	switch filepath.Ext(path) {
	case ".go", ".sh", ".nsi", ".yml", ".yaml", ".service", ".timer", ".1", ".example":
		return true
	}
	return false
}

func TestSourceFilesHaveLicenseHeader(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "dist", "build", "testdata", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !needsHeader(path) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.SplitN(string(b), "\n", 4)
		if len(lines) > 3 {
			lines = lines[:3]
		}
		head := strings.Join(lines, "\n")
		if !strings.Contains(head, spdx) || !strings.Contains(string(b), "Copyright (C) 2026 Maurice Snoeren") {
			t.Errorf("%s: no GPL license header in the first lines", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestThirdPartyNotices checks that THIRD-PARTY-NOTICES.txt names every
// required module at the version in go.mod ("make notices" regenerates it).
func TestThirdPartyNotices(t *testing.T) {
	gomod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	notices, err := os.ReadFile("THIRD-PARTY-NOTICES.txt")
	if err != nil {
		t.Fatal(err)
	}
	inRequire := false
	for _, line := range strings.Split(string(gomod), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "require (":
			inRequire = true
			continue
		case line == ")":
			inRequire = false
			continue
		case strings.HasPrefix(line, "require "):
			line = strings.TrimPrefix(line, "require ")
		case !inRequire:
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		if want := f[0] + "@" + f[1]; !strings.Contains(string(notices), "\n"+want+"\n") {
			t.Errorf("THIRD-PARTY-NOTICES.txt lacks %s; run make notices", want)
		}
	}
}
