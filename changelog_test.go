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

package backupusb_test

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	versionHeading = regexp.MustCompile(`^## \[(\d+)\.(\d+)\.(\d+)\] - (\d{4}-\d{2}-\d{2})$`)
	linkRef        = regexp.MustCompile(`^\[([^\]]+)\]: https://`)
)

// TestChangelog checks CHANGELOG.md: an Unreleased section first, then
// versions with a valid date, newest first, each with a link at the bottom,
// and scripts/release-notes.sh can extract every one of them.
func TestChangelog(t *testing.T) {
	b, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	links := map[string]bool{}
	var prev [3]int
	sawUnreleased := false
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case line == "## [Unreleased]":
			if len(versions) > 0 {
				t.Error("## [Unreleased] must come before the versions")
			}
			sawUnreleased = true
		case strings.HasPrefix(line, "## ["):
			m := versionHeading.FindStringSubmatch(line)
			if m == nil {
				t.Errorf("heading %q is not ## [X.Y.Z] - YYYY-MM-DD", line)
				continue
			}
			if _, err := time.Parse("2006-01-02", m[4]); err != nil {
				t.Errorf("%q: invalid date", line)
			}
			var v [3]int
			for i := range v {
				v[i], _ = strconv.Atoi(m[i+1])
			}
			if len(versions) > 0 && !less(v, prev) {
				t.Errorf("version %s is not lower than the one above it", line)
			}
			prev = v
			versions = append(versions, m[1]+"."+m[2]+"."+m[3])
		default:
			if m := linkRef.FindStringSubmatch(line); m != nil {
				links[m[1]] = true
			}
		}
	}
	if !sawUnreleased {
		t.Error("no ## [Unreleased] section")
	}
	if len(versions) == 0 {
		t.Fatal("no versions in CHANGELOG.md")
	}
	for _, v := range append([]string{"Unreleased"}, versions...) {
		if !links[v] {
			t.Errorf("no link reference [%s]: at the bottom", v)
		}
	}

	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh: release-notes.sh not tested")
	}
	for _, v := range versions {
		out, err := exec.Command(sh, "scripts/release-notes.sh", "v"+v).CombinedOutput()
		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			t.Errorf("release-notes.sh v%s: %v\n%s", v, err, out)
		}
	}
	if out, err := exec.Command(sh, "scripts/release-notes.sh", "v999.0.0").CombinedOutput(); err == nil {
		t.Errorf("release-notes.sh accepted a version without a section:\n%s", out)
	}
}

func less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
