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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

// stableSources gives every source file and folder fixed times and modes,
// so the zips and their hashes are the same on every run.
func stableSources(t *testing.T, dir string) {
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if info.IsDir() {
			mode = 0o755
		}
		if err := os.Chmod(p, mode); err != nil {
			return err
		}
		return os.Chtimes(p, stamp, stamp)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReportGoldenFiles(t *testing.T) {
	w := newWorld(t)
	w.clock.Set(time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC))
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	stableSources(t, w.srcDir)
	out, _ := w.mustRun(0, "run", "--json")
	mount := w.disks.Disk("BACKUP1").Mount
	norm := strings.NewReplacer(mount, "<mount>", strings.ReplaceAll(mount, `\`, `\\`), "<mount>")
	// Hashes and zip sizes depend on the owner and group IDs (and file
	// attributes on Windows) of the machine; they are checked elsewhere.
	testutil.Golden(t, "run-week47.json", []byte(stable(norm.Replace(out))))
	report, err := os.ReadFile(w.diskFile("BACKUP1", "Backup/2026/47/report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Golden(t, "run-week47-report.txt", []byte(stable(string(report))))
}

var (
	hexRe     = regexp.MustCompile(`[0-9a-f]{64}`)
	zipJSONRe = regexp.MustCompile(`"zipBytes": \d+`)
	zipTextRe = regexp.MustCompile(`Zip size:( +)[0-9.]+ [KMG]?i?B`)
	freeRe    = regexp.MustCompile(`"freeBytes": \d+`)
)

func stable(s string) string {
	s = hexRe.ReplaceAllString(s, "<sha256>")
	s = zipJSONRe.ReplaceAllString(s, `"zipBytes": 0`)
	s = zipTextRe.ReplaceAllString(s, "Zip size:$1<size>")
	return freeRe.ReplaceAllString(s, `"freeBytes": 0`)
}
