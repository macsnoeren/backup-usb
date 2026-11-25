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
	"path/filepath"
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

// TestCheckRelease checks which tags scripts/check-release.sh accepts.
func TestCheckRelease(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	const old = "## [1.2.0] - 2026-11-01\n\n- First.\n\n[1.2.0]: https://x\n"
	const ready = "# C\n\n## [Unreleased]\n\n## [1.3.0] - 2026-11-20\n\n### Added\n\n- New.\n\n" + old
	for _, tc := range []struct {
		name, tag, changelog string
		ok                   bool
	}{
		{"newest version", "v1.3.0", ready, true},
		{"headings only under Unreleased", "v1.3.0", strings.Replace(ready, "[Unreleased]\n", "[Unreleased]\n\n### Added\n", 1), true},
		{"changes left under Unreleased", "v1.3.0", strings.Replace(ready, "[Unreleased]\n", "[Unreleased]\n\n- Forgotten.\n", 1), false},
		{"older version", "v1.2.0", ready, false},
		{"not in the changelog", "v1.4.0", ready, false},
		{"no date", "v1.3.0", strings.Replace(ready, " - 2026-11-20", "", 1), false},
		{"empty section", "v1.3.0", "## [Unreleased]\n\n## [1.3.0] - 2026-11-20\n\n" + old, false},
		{"pre-release", "v1.4.0-rc1", strings.Replace(ready, "[Unreleased]\n", "[Unreleased]\n\n- Next.\n", 1), true},
		{"pre-release without changes", "v1.4.0-rc1", ready, false},
		{"no v", "1.3.0", ready, false},
		{"leading zero", "v1.03.0", ready, false},
		{"too short", "v1.3", ready, false},
		{"branch", "main", ready, false},
		{"empty", "", ready, false},
		{"shell", "v1.3.0;id", ready, false},
		{"substitution", "v1.3.0$(id)", ready, false},
		{"second line", "v1.3.0\nx", ready, false},
		{"option", "-v1.3.0", ready, false},
		{"pattern", "v1.3.*", ready, false},
	} {
		file := filepath.Join(t.TempDir(), "CHANGELOG.md")
		if err := os.WriteFile(file, []byte(tc.changelog), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(sh, "scripts/check-release.sh", tc.tag, file).CombinedOutput()
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v\n%s", tc.name, err, out)
		}
		if err != nil && len(out) == 0 {
			t.Errorf("%s: no explanation", tc.name)
		}
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
