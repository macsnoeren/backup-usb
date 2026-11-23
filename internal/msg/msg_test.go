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

package msg

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestEveryKeyHasEnglishAndMatchingVerbs(t *testing.T) {
	for _, k := range Keys() {
		e := table[k]
		if e[EN] == "" {
			t.Errorf("%s: no English text", k)
		}
		if e[NL] != "" && strings.Count(e[NL], "%") != strings.Count(e[EN], "%") {
			t.Errorf("%s: format verbs differ between nl and en", k)
		}
	}
}

func TestFallback(t *testing.T) {
	if got := T(EN, "no.such.key"); got != "no.such.key" {
		t.Fatalf("got %q", got)
	}
	if got := T(EN, "version", "1", "x", "linux", "amd64"); !strings.Contains(got, "version 1") {
		t.Fatalf("got %q", got)
	}
}

// TestUsedKeysExist scans the Go sources for message keys and checks that
// each one is in the table, so a lost entry fails the build.
var fileName = regexp.MustCompile(`\.(json|txt|zip|sha256|age|log|vault|recipients|pub|sig|key|part)$`)

func TestUsedKeysExist(t *testing.T) {
	root := testutil.RepoRoot()
	re := regexp.MustCompile(`"((?:err|msg|report|status|dry|state|gui|help|keys|disk|cfg|task|restore|list|verify|status)\.[A-Za-z0-9.]+)"`)
	err := filepath.Walk(filepath.Join(root, "internal"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || strings.Contains(p, string(filepath.Separator)+"msg"+string(filepath.Separator)) {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			if fileName.MatchString(m[1]) {
				continue // a file name such as keys.json, not a key
			}
			if !Has(m[1]) && !strings.HasSuffix(m[1], ".") {
				t.Errorf("%s: key %q is not in the translation table", filepath.Base(p), m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
