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

package week

import (
	"reflect"
	"testing"
	"time"
)

func TestChoose(t *testing.T) {
	two := []Slot{{Name: "BACKUP1", Weeks: Odd, Order: 0}, {Name: "BACKUP2", Weeks: Even, Order: 1}}
	three := []Slot{{Name: "A", Order: 0}, {Name: "B", Order: 1}, {Name: "C", Order: 2}}
	tests := []struct {
		name  string
		when  time.Time
		s     Schedule
		rot   Rotation
		disks []Slot
		want  []string
	}{
		{"week 47 odd", d(2026, 11, 22), ISOWeek, OddEven, two, []string{"BACKUP1"}},
		{"week 48 even", d(2026, 11, 23), ISOWeek, OddEven, two, []string{"BACKUP2"}},
		{"week 52 even", d(2026, 12, 21), ISOWeek, OddEven, two, []string{"BACKUP2"}},
		{"week 53 odd", d(2026, 12, 31), ISOWeek, OddEven, two, []string{"BACKUP1"}},
		{"week 1 after 53 also odd", d(2027, 1, 4), ISOWeek, OddEven, two, []string{"BACKUP1"}},
		{"week 2 even", d(2027, 1, 11), ISOWeek, OddEven, two, []string{"BACKUP2"}},
		{"day 22 even", d(2026, 11, 22), Day, OddEven, two, []string{"BACKUP2"}},
		{"month 11 odd", d(2026, 11, 22), Month, OddEven, two, []string{"BACKUP1"}},
		{"any", d(2026, 11, 22), ISOWeek, Any, two, []string{"BACKUP1", "BACKUP2"}},
		{"no disks", d(2026, 11, 22), ISOWeek, OddEven, nil, nil},
		{"only even registered", d(2026, 11, 22), ISOWeek, OddEven, two[1:], nil},
	}
	for _, tt := range tests {
		got, err := Choose(For(tt.when, tt.s), tt.rot, tt.disks)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}
	// Round robin cycles through all disks in order, also across a year end.
	per := For(d(2026, 12, 14), ISOWeek)
	var seq []string
	for i := 0; i < 6; i++ {
		got, err := Choose(per, RoundRobin, three)
		if err != nil || len(got) != 1 {
			t.Fatal(got, err)
		}
		seq = append(seq, got[0])
		per = For(per.End().Add(time.Hour), ISOWeek)
	}
	for i := 3; i < 6; i++ {
		if seq[i] != seq[i-3] || seq[i] == seq[i-1] {
			t.Fatalf("round robin sequence %v", seq)
		}
	}
	if _, err := Choose(per, "weird", two); err == nil {
		t.Fatal("unknown rotation accepted")
	}
}
