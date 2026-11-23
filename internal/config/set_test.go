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

import "testing"

func TestSet(t *testing.T) {
	c := Defaults()
	cases := []struct{ key, value, want string }{
		{"keepPerDisk", "10", "10"},
		{"verifyAfterBackup", "false", "false"},
		{"folderPattern", `{year}-{week}`, `"{year}-{week}"`},
		{"schedule", "day", `"day"`},
		{"exclude", "*.bak, *.tmp", `["*.bak","*.tmp"]`},
		{"storeExtensions", `["jpg"]`, `["jpg"]`},
		{"sources", `[{"tag":"A","path":"/a"}]`, `[{"tag":"A","path":"/a"}]`},
	}
	for _, tc := range cases {
		_, got, err := c.Set(tc.key, tc.value)
		if err != nil || got != tc.want {
			t.Errorf("%s=%s: %q %v", tc.key, tc.value, got, err)
		}
	}
	for _, bad := range [][2]string{{"nope", "1"}, {"keepPerDisk", "x"}, {"ejectAfter", "maybe"}, {"sources", "a,b"}} {
		if _, _, err := c.Set(bad[0], bad[1]); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	if len(Keys()) < 30 {
		t.Fatal("keys")
	}
}
